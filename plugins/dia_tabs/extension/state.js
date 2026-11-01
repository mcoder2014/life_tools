const isWeb = tab => !tab.incognito && /^https?:\/\//i.test(tab.url || '');

// Only this injected function reads page content. The observer sends a signal;
// the worker checks active/complete again before each subsequent title read.
export function readPageTitle() {
  const key = '__lifeToolsDiaTitleObserverV1';
  if (!globalThis[key] && document.head) {
    let timer;
    const observer = new MutationObserver(mutations => {
      const titleChanged = mutations.some(mutation => mutation.target.nodeName === 'TITLE'
        || mutation.target.parentNode?.nodeName === 'TITLE'
        || [...mutation.addedNodes, ...mutation.removedNodes].some(node => node.nodeName === 'TITLE'));
      if (!titleChanged || timer) return;
      timer = setTimeout(() => {
        timer = null;
        chrome.runtime.sendMessage({type: 'dia-tabs-title-changed'}).catch(() => {});
      }, 150);
    });
    observer.observe(document.head, {subtree: true, childList: true, characterData: true});
    globalThis[key] = observer;
  }
  return {title: document.title, url: location.href, readyState: document.readyState};
}

// Session state stays in one worker. Navigation invalidation is synchronous;
// every asynchronous result is checked against both navigation and sample IDs.
export function createTabService(api) {
  const records = new Map(), versions = new Map(), samples = new Map(), tasks = new Set(), initialRemovals = new Set();
  const pendingManual = new Set();
  let sessionId, initialized = false, writeTail = Promise.resolve(), syncPromise = null;
  const persist = () => {
    writeTail = writeTail.catch(() => {}).then(() => api.storage.session.set({
      diaTabsSession: sessionId, diaTabsRecords: Object.fromEntries(records),
    }));
    writeTail.catch(() => {});
  };
  const track = promise => {
    tasks.add(promise);
    promise.catch(() => {}).finally(() => tasks.delete(promise));
    return promise;
  };
  const invalidate = (id, url) => {
    versions.set(id, (versions.get(id) || 0) + 1);
    samples.set(id, (samples.get(id) || 0) + 1);
    if (url && /^https?:\/\//i.test(url)) records.set(id, {url, title: '', titleState: 'pending'});
    else records.delete(id);
    if (sessionId) persist();
  };
  const capture = async (tab, {allowBackground = false} = {}) => {
    if (!isWeb(tab)) return 'unsupported';
    if (!allowBackground && !tab.active) return 'inactive';
    if (tab.discarded) return 'discarded';
    if (tab.frozen) return 'frozen';
    if (tab.status !== 'complete') return tab.status === 'loading' ? 'loading' : tab.status === 'unloaded' ? 'unloaded' : 'unknown_status';
    const version = versions.get(tab.id) || 0, sample = (samples.get(tab.id) || 0) + 1;
    samples.set(tab.id, sample);
    // A queued manual sample must not inject into a tab discarded/frozen since
    // the batch snapshot. No activation, reload or other wake-up is attempted.
    if (allowBackground) {
      let current;
      try { current = await api.tabs.get(tab.id); } catch { return 'closed'; }
      if (current.discarded) return 'discarded';
      if (current.frozen) return 'frozen';
      if (!isWeb(current) || current.url !== tab.url || (versions.get(tab.id) || 0) !== version || samples.get(tab.id) !== sample) return 'stale';
      if (current.status !== 'complete') return current.status === 'loading' ? 'loading' : current.status === 'unloaded' ? 'unloaded' : 'unknown_status';
    }
    let page, timeout;
    try {
      if (allowBackground && pendingManual.size >= 4) return 'capacity';
      const execution = api.scripting.executeScript({target: {tabId: tab.id, frameIds: [0]}, func: readPageTitle});
      if (allowBackground) {
        pendingManual.add(execution);
        execution.then(() => pendingManual.delete(execution), () => pendingManual.delete(execution));
      }
      const results = allowBackground ? await Promise.race([execution, new Promise(resolve => {
        timeout = setTimeout(() => resolve(null), 5000);
      })]) : await execution;
      if (results === null) return 'timeout';
      page = results.find(result => result.frameId === 0)?.result;
    } catch { /* Restricted pages remain visible with an explicit unavailable state. */ }
    finally { clearTimeout(timeout); }
    let current;
    try { current = await api.tabs.get(tab.id); } catch { return 'closed'; }
    if (current.discarded) return 'discarded';
    if (current.frozen) return 'frozen';
    if (!isWeb(current) || current.url !== tab.url || (versions.get(tab.id) || 0) !== version || samples.get(tab.id) !== sample) return 'stale';
    if (!allowBackground && !current.active) return 'inactive';
    if (current.status !== 'complete') return current.status === 'loading' ? 'loading' : current.status === 'unloaded' ? 'unloaded' : 'unknown_status';
    if (page && (page.url !== tab.url || page.readyState !== 'complete')) return 'stale';
    records.set(tab.id, {url: tab.url, title: typeof page?.title === 'string' ? page.title : '',
      titleState: typeof page?.title === 'string' ? 'captured' : 'unavailable'});
    persist();
    return records.get(tab.id).titleState;
  };
  const ready = (async () => {
    const saved = await api.storage.session.get(['diaTabsSession', 'diaTabsRecords']);
    sessionId = typeof saved.diaTabsSession === 'string' && saved.diaTabsSession ? saved.diaTabsSession : crypto.randomUUID();
    for (const [key, record] of Object.entries(saved.diaTabsRecords || {})) {
      if (!versions.has(Number(key)) && record && ['captured','pending','unavailable'].includes(record.titleState)
        && typeof record.url === 'string' && typeof record.title === 'string') records.set(Number(key), record);
    }
    const tabs = (await api.tabs.query({})).filter(tab => isWeb(tab) && !initialRemovals.has(tab.id));
    const live = new Set(tabs.map(tab => tab.id));
    for (const id of records.keys()) if (!live.has(id)) records.delete(id);
    for (const tab of tabs) {
      if (records.get(tab.id)?.url !== tab.url) records.set(tab.id, {url: tab.url, title: '', titleState: 'pending'});
    }
    initialized = true;
    initialRemovals.clear();
    persist();
    await Promise.all(tabs.map(tab => capture(tab)));
  })();
  const refreshTab = id => track(ready.then(async () => {
    try { const tab = await api.tabs.get(id); await capture(tab); } catch { /* Tab closed. */ }
  }));
  api.tabs.onUpdated.addListener((id, change, tab) => {
    if (change.status === 'loading' || Object.hasOwn(change, 'url')) invalidate(id, tab.incognito ? undefined : change.url || tab.url);
    if (change.status === 'complete' || Object.hasOwn(change, 'title') || Object.hasOwn(change, 'url')) refreshTab(id);
  });
  api.tabs.onActivated.addListener(({tabId}) => { refreshTab(tabId); });
  api.tabs.onCreated.addListener(tab => { refreshTab(tab.id); });
  api.tabs.onRemoved.addListener(id => { if (!initialized) initialRemovals.add(id); invalidate(id); });
  api.tabs.onReplaced.addListener((added, removed) => {
    if (!initialized) initialRemovals.add(removed);
    invalidate(removed); refreshTab(added);
  });

  const list = async () => {
    await ready;
    const tabs = (await api.tabs.query({})).filter(isWeb).sort((a,b) => a.windowId-b.windowId || a.index-b.index);
    const live = new Set(tabs.map(tab => tab.id));
    for (const id of records.keys()) if (!live.has(id)) records.delete(id);
    await Promise.all(tabs.filter(tab => !records.has(tab.id) || records.get(tab.id).url !== tab.url || records.get(tab.id).titleState === 'pending').map(tab => capture(tab)));
    persist();
    return tabs.map(tab => {
      const record = records.get(tab.id);
      const valid = record?.url === tab.url;
      return {tabId: tab.id, windowId: tab.windowId, index: tab.index, url: tab.url,
        title: valid ? record.title : '', titleState: valid ? record.titleState : 'pending', active: tab.active,
        status: typeof tab.status === 'string' ? tab.status : 'unknown',
        discarded: typeof tab.discarded === 'boolean' ? tab.discarded : null,
        frozen: typeof tab.frozen === 'boolean' ? tab.frozen : null};
    });
  };
  const syncOpenTabs = () => {
    if (syncPromise) return syncPromise;
    syncPromise = (async () => {
      await ready;
      const tabs = (await api.tabs.query({})).filter(isWeb);
      const counts = {captured: 0, unavailable: 0, skipped: 0, skipReasons: {}};
      let next = 0;
      await Promise.all(Array.from({length: Math.min(4, tabs.length)}, async () => {
        while (next < tabs.length) {
          const result = await capture(tabs[next++], {allowBackground: true});
          if (result === 'captured' || result === 'unavailable') counts[result]++;
          else { counts.skipped++; counts.skipReasons[result] = (counts.skipReasons[result] || 0) + 1; }
        }
      }));
      return counts;
    })().finally(() => { syncPromise = null; });
    track(syncPromise);
    return syncPromise;
  };
  const handleRequest = async request => {
    const response = {v: 1, id: typeof request?.id === 'string' ? request.id : '', ok: false};
    const fail = (error, message) => ({...response, error, message});
    if (request?.v !== 1 || typeof request.id !== 'string' || !request.id || !['list','activate'].includes(request.op))
      return fail('BAD_REQUEST', '请求格式无效');
    try {
      await ready;
      if (request.op === 'list') return {...response, ok: true, sessionId, tabs: await list()};
      if (typeof request.sessionId !== 'string' || !Number.isInteger(request.tabId) || request.tabId < 0)
        return fail('BAD_REQUEST', '激活参数无效');
      if (request.sessionId !== sessionId) return fail('STALE_SESSION', '浏览器会话已变化，请刷新列表');
      let tab;
      try { tab = await api.tabs.get(request.tabId); } catch { return fail('TAB_CLOSED', '标签已关闭'); }
      if (!isWeb(tab)) return fail('TAB_CLOSED', '标签已关闭或不在支持范围');
      try { tab = await api.tabs.update(request.tabId, {active: true}); } catch {
        try { await api.tabs.get(request.tabId); } catch { return fail('TAB_CLOSED', '标签已关闭'); }
        return fail('INTERNAL_ERROR', '无法激活标签');
      }
      const window = await api.windows.get(tab.windowId);
      if (window.state === 'minimized') await api.windows.update(tab.windowId, {state: 'normal'});
      await api.windows.update(tab.windowId, {focused: true});
      return {...response, ok: true};
    } catch { return fail('INTERNAL_ERROR', '浏览器操作失败，请刷新后重试'); }
  };
  return {ready, list, syncOpenTabs, handleRequest, refreshTab, get sessionId() {return sessionId;}, async idle() {
    await ready;
    while (tasks.size) await Promise.allSettled([...tasks]);
    await writeTail;
  }};
}

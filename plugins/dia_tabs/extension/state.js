const isWeb = tab => !tab.incognito && /^https?:\/\//i.test(tab.url || '');

// Only this injected function reads page content. The observer sends a signal;
// the worker checks active/complete again before each subsequent title read.
export function readPageTitle() {
  const key = '__lifeToolsDiaTitleObserverV2';
  if (!globalThis[key] && document.head) {
    globalThis.__lifeToolsDiaTitleObserverV1?.disconnect();
    delete globalThis.__lifeToolsDiaTitleObserverV1;
    let timer, dirty = false;
    const observer = new MutationObserver(mutations => {
      const titleChanged = mutations.some(mutation => mutation.target.nodeName === 'TITLE'
        || mutation.target.parentNode?.nodeName === 'TITLE'
        || [...mutation.addedNodes, ...mutation.removedNodes].some(node => node.nodeName === 'TITLE'));
      if (!titleChanged) return;
      if (timer) { dirty = true; return; }
      chrome.runtime.sendMessage({type: 'dia-tabs-title-changed'}).catch(() => {});
      timer = setTimeout(() => {
        timer = null;
        if (dirty) {
          dirty = false;
          chrome.runtime.sendMessage({type: 'dia-tabs-title-changed'}).catch(() => {});
        }
      }, 150);
    });
    observer.observe(document.head, {subtree: true, childList: true, characterData: true});
    globalThis[key] = observer;
  }
  return {title: document.title, url: location.href, readyState: document.readyState, visibilityState: document.visibilityState};
}

// Session state stays in one worker. Navigation invalidation is synchronous;
// every asynchronous result is checked against both navigation and sample IDs.
export function createTabService(api) {
  const records = new Map(), versions = new Map(), samples = new Map(), tasks = new Set(), initialRemovals = new Set();
  const pendingReads = new Map(), initialUpdates = [];
  let sessionId, initialized = false, writeTail = Promise.resolve(), syncPromise = null, probeEpoch = 0;
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
  const invalidate = (id, url, preserveTitle = false) => {
    versions.set(id, (versions.get(id) || 0) + 1);
    samples.set(id, (samples.get(id) || 0) + 1);
    if (!preserveTitle) {
      if (url && /^https?:\/\//i.test(url)) records.set(id, {url, title: '', titleState: 'pending'});
      else records.delete(id);
    }
    if (sessionId) persist();
  };
  const updateRecord = (id, url, loading, suspended) => {
    const record = records.get(id), urlChanged = record?.url !== url;
    // Suspension rejects in-flight reads even after recovery, while retaining
    // the last DOM title for the same URL. A real reload still clears it.
    if (urlChanged || loading || suspended)
      invalidate(id, url, !urlChanged && suspended && record?.titleState === 'captured');
  };
  // Manual reads and foreground probes share actual execution slots. A timeout
  // ends the wait, but only the browser settling the execution releases its slot.
  const executeTitle = async (tab, bounded) => {
    let timeout;
    try {
      const execution = api.scripting.executeScript({target: {tabId: tab.id, frameIds: [0]}, func: readPageTitle});
      if (bounded) {
        pendingReads.set(tab.id, execution);
        execution.then(() => pendingReads.delete(tab.id), () => pendingReads.delete(tab.id));
      }
      const results = bounded ? await Promise.race([execution, new Promise(resolve => {
        timeout = setTimeout(() => resolve(null), 5000);
      })]) : await execution;
      return results === null ? {error: 'timeout'} : {page: results.find(result => result.frameId === 0)?.result};
    } catch { return {error: 'unavailable'}; }
    finally { clearTimeout(timeout); }
  };
  const capture = async (tab, {allowBackground = false} = {}) => {
    if (!isWeb(tab)) return 'unsupported';
    if (!allowBackground && !tab.active) return 'inactive';
    if (tab.discarded) return 'discarded';
    const foregroundProbe = !allowBackground && tab.frozen === true && tab.active;
    if (tab.frozen && !allowBackground && !foregroundProbe) return 'frozen';
    if (tab.status !== 'complete') return tab.status === 'loading' ? 'loading' : tab.status === 'unloaded' ? 'unloaded' : 'unknown_status';
    // Reserve one of the four bounded execution slots for automatic foreground probes.
    const bounded = allowBackground || foregroundProbe, readLimit = allowBackground ? 3 : 4;
    if (bounded && (pendingReads.has(tab.id) || pendingReads.size >= readLimit)) return 'capacity';
    const version = versions.get(tab.id) || 0, previousSample = samples.get(tab.id) || 0, epoch = probeEpoch;
    // Manual sync also attempts pages reported frozen, but never discarded
    // pages. Automatic frozen probes still require an active, focused page.
    // No activation or reload is requested.
    if (bounded) {
      let current;
      try { current = await api.tabs.get(tab.id); } catch { return 'closed'; }
      if (current.discarded) return 'discarded';
      if (current.frozen && !allowBackground && !foregroundProbe) return 'frozen';
      if (foregroundProbe) {
        if (!current.active) return 'inactive';
        try { if ((await api.windows.get(current.windowId)).focused !== true) return 'frozen'; }
        catch { return 'frozen'; }
      }
      if (!isWeb(current) || current.url !== tab.url || (versions.get(tab.id) || 0) !== version
        || (samples.get(tab.id) || 0) !== previousSample || (foregroundProbe && probeEpoch !== epoch)) return 'stale';
      if (current.status !== 'complete') return current.status === 'loading' ? 'loading' : current.status === 'unloaded' ? 'unloaded' : 'unknown_status';
      tab = current;
    }
    // Recheck after async gates and before changing sample IDs: duplicate list
    // requests must neither overbook slots nor invalidate the original probe.
    if (bounded && (pendingReads.has(tab.id) || pendingReads.size >= readLimit)) return 'capacity';
    const sample = (samples.get(tab.id) || 0) + 1;
    samples.set(tab.id, sample);
    const {page, error} = await executeTitle(tab, bounded);
    if (error === 'timeout') return 'timeout';
    let current;
    try { current = await api.tabs.get(tab.id); } catch { return 'closed'; }
    if (current.discarded) return 'discarded';
    if (current.frozen && !allowBackground && !foregroundProbe) return 'frozen';
    if (foregroundProbe) {
      if (!current.active) return 'inactive';
      try { if ((await api.windows.get(current.windowId)).focused !== true) return 'frozen'; }
      catch { return 'frozen'; }
    }
    if (!isWeb(current) || current.url !== tab.url || (versions.get(tab.id) || 0) !== version || samples.get(tab.id) !== sample) return 'stale';
    if (!allowBackground && !current.active) return 'inactive';
    if (current.status !== 'complete') return current.status === 'loading' ? 'loading' : current.status === 'unloaded' ? 'unloaded' : 'unknown_status';
    if (page && (page.url !== tab.url || page.readyState !== 'complete')) return 'stale';
    if (foregroundProbe && (probeEpoch !== epoch || page?.visibilityState !== 'visible' || typeof page?.title !== 'string')) return 'stale';
    const record = records.get(tab.id);
    if (allowBackground && typeof page?.title !== 'string' && record?.url === tab.url && record.titleState === 'captured') return 'unavailable';
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
    // Compare early events with restored records, in order; the query snapshot
    // may predate these events. Removed/replaced tabs must not be resurrected.
    for (const update of initialUpdates) if (live.has(update[0])) updateRecord(...update);
    initialUpdates.length = 0;
    initialized = true;
    initialRemovals.clear();
    persist();
    await Promise.all(tabs.filter(tab => !versions.has(tab.id)).map(tab => capture(tab)));
  })();
  const refreshTab = id => track(ready.then(async () => {
    try { const tab = await api.tabs.get(id); await capture(tab); } catch { /* Tab closed. */ }
  }));
  api.tabs.onUpdated.addListener((id, change, tab) => {
    if (change.status === 'loading' || Object.hasOwn(change, 'url')
      || change.frozen === true || change.discarded === true || change.status === 'unloaded') {
      const update = [id, tab.incognito ? undefined : change.url || tab.url, change.status === 'loading',
        !!(tab.frozen || tab.discarded || tab.status === 'unloaded'
          || change.frozen === true || change.discarded === true || change.status === 'unloaded')];
      if (initialized) updateRecord(...update);
      else initialUpdates.push(update);
    }
    if (change.status === 'complete' || Object.hasOwn(change, 'title') || Object.hasOwn(change, 'url')
      || change.frozen === false || change.discarded === false) refreshTab(id);
  });
  api.tabs.onActivated.addListener(({tabId}) => { probeEpoch++; refreshTab(tabId); });
  api.windows.onFocusChanged.addListener(windowId => {
    probeEpoch++;
    if (windowId < 0) return;
    track(ready.then(async () => {
      const tabs = await api.tabs.query({windowId, active: true});
      await Promise.all(tabs.map(tab => refreshTab(tab.id)));
    }).catch(() => {}));
  });
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
      await Promise.all(Array.from({length: Math.min(3, tabs.length)}, async () => {
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

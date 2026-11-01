import {updateSelection, retainSelection, selectedOutput} from './control-core.js';

const byId = id => document.getElementById(id);
let tabs = [], selected = new Set(), anchor = null, sessionId = '', loading = false;
let refreshQueued = false, syncing = false;

function visibleTabs() {
  const query = byId('search').value.trim().toLocaleLowerCase();
  return tabs.filter(tab => `${tab.title}\n${tab.url}`.toLocaleLowerCase().includes(query));
}

function notify(message, tone = 'info') {
  byId('notice').textContent = message;
  byId('notice').setAttribute('data-tone', tone);
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
    notify('已复制到剪贴板', 'success');
  } catch { notify('复制失败，请检查剪贴板权限后重试。', 'error'); }
}

async function activate(tabId) {
  const response = await chrome.runtime.sendMessage({v:1, id:crypto.randomUUID(), op:'activate', sessionId, tabId}).catch(() => null);
  if (!response?.ok) {
    notify(response?.message || '无法切换标签，请刷新列表。', 'error');
    if (response?.error === 'TAB_CLOSED' || response?.error === 'STALE_SESSION') await refresh();
  }
}

function render() {
  const shown = visibleTabs();
  const visibleIds = shown.map(tab => tab.tabId);
  const body = byId('tabs');
  body.replaceChildren();
  for (const tab of shown) {
    const row = document.createElement('tr');
    row.classList.toggle('selected', selected.has(tab.tabId));
    row.setAttribute('aria-selected', String(selected.has(tab.tabId)));
    row.addEventListener('click', event => {
      const result = updateSelection(selected, anchor, tab.tabId, visibleIds, event);
      selected = result.selected; anchor = result.anchor; render();
    });
    const checkCell = document.createElement('td');
    const check = document.createElement('input');
    check.type = 'checkbox'; check.checked = selected.has(tab.tabId); check.setAttribute('aria-label', `选择 ${tab.title || tab.url}`);
    check.addEventListener('click', event => {
      event.stopPropagation();
      const result = updateSelection(selected, anchor, tab.tabId, visibleIds, {...event, ctrlKey:true, shiftKey:event.shiftKey});
      selected = result.selected; anchor = result.anchor; render();
    });
    checkCell.append(check);
    const titleCell = document.createElement('td');
    const title = document.createElement('button');
    title.className = 'title-button'; title.type = 'button';
    title.textContent = tab.titleState === 'captured' ? (tab.title || '无标题') : tab.titleState === 'pending' ? '待访问' : '无法读取页面标题';
    title.title = title.textContent;
    title.classList.toggle('is-placeholder', tab.titleState !== 'captured' || !tab.title);
    const navigate = event => {
      event.preventDefault();
      if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
      event.stopPropagation(); activate(tab.tabId);
    };
    title.addEventListener('click', navigate); titleCell.append(title);
    const urlCell = document.createElement('td');
    const link = document.createElement('a');
    link.className = 'url'; link.href = tab.url; link.textContent = tab.url; link.title = tab.url;
    link.addEventListener('click', navigate);
    link.addEventListener('auxclick', event => { event.preventDefault(); event.stopPropagation(); });
    urlCell.append(link);
    const stateCell = document.createElement('td'); stateCell.className = 'state-cell';
    const primaryState = document.createElement('div'); primaryState.className = 'state-line';
    const recordState = document.createElement('span');
    const recordKind = tab.titleState === 'captured' ? 'captured' : tab.titleState === 'pending' ? 'pending' : 'unavailable';
    recordState.className = `state-badge ${recordKind}`;
    recordState.textContent = recordKind === 'captured' ? '已记录' : recordKind === 'pending' ? '待访问' : '无法读取';
    primaryState.append(recordState);
    if (tab.active) {
      const current = document.createElement('span'); current.className = 'state-badge current'; current.textContent = '当前标签';
      primaryState.append(current);
    }
    const secondaryState = document.createElement('div'); secondaryState.className = 'state-line state-secondary';
    const pageState = tab.discarded ? {label:'已释放', kind:'discarded'} : tab.frozen ? {label:'浏览器标记冻结', kind:'frozen'} : tab.status === 'loading' ? {label:'加载中', kind:'loading'} : tab.status === 'unloaded' ? {label:'尚未加载', kind:'unloaded'} : tab.status === 'complete' ? {label:'已加载', kind:'complete'} : {label:'状态未知', kind:'unknown'};
    const page = document.createElement('span'); page.className = `state-badge ${pageState.kind}`; page.textContent = pageState.label;
    const windowLabel = document.createElement('span'); windowLabel.className = 'window-label'; windowLabel.textContent = `窗口 ${tab.windowId}`;
    secondaryState.append(page, windowLabel); stateCell.append(primaryState, secondaryState);
    const copyCell = document.createElement('td');
    const copyButton = document.createElement('button'); copyButton.type = 'button'; copyButton.className = 'copy-button'; copyButton.textContent = '复制';
    copyButton.setAttribute('aria-label', `复制 ${tab.url}`);
    copyButton.addEventListener('click', event => { event.stopPropagation(); copy(tab.url); }); copyCell.append(copyButton);
    row.append(checkCell,titleCell,urlCell,stateCell,copyCell); body.append(row);
  }
  const selectedVisible = shown.filter(tab => selected.has(tab.tabId)).length;
  byId('count').textContent = `${shown.length} 个网页 / 共 ${tabs.length} 个 · 已选择 ${selectedVisible} 个可见标签`;
  byId('copy-urls').disabled = selectedVisible === 0; byId('copy-markdown').disabled = selectedVisible === 0;
  byId('select-all').checked = shown.length > 0 && selectedVisible === shown.length;
  byId('select-all').indeterminate = selectedVisible > 0 && selectedVisible < shown.length;
  byId('empty').hidden = shown.length !== 0;
}

async function bridge(reconnect = false) {
  const result = await chrome.runtime.sendMessage({type:reconnect ? 'dia-tabs-reconnect' : 'dia-tabs-bridge-status'}).catch(() => null);
  const state = result?.state === 'connected' ? 'connected' : result?.state === 'connecting' ? 'connecting' : 'disconnected';
  byId('bridge-status').textContent = state === 'connected' ? 'Alfred 连接就绪' : state === 'connecting' ? '正在准备 Alfred 连接…' : 'Alfred 连接未就绪';
  byId('bridge-status').setAttribute('data-state', state);
  const detail = state === 'disconnected' && result?.message ? result.message : '';
  byId('bridge-detail').textContent = detail;
  byId('bridge-detail').hidden = !detail;
}

async function refresh() {
  if (loading) { refreshQueued = true; return; }
  const preserveNotice = syncing;
  loading = true;
  byId('refresh').disabled = true;
  try {
    const response = await chrome.runtime.sendMessage({v:1,id:crypto.randomUUID(),op:'list'});
    if (!response?.ok) throw Error(response?.message || '列表读取失败');
    const changed = sessionId !== response.sessionId || JSON.stringify(tabs) !== JSON.stringify(response.tabs);
    if (sessionId && sessionId !== response.sessionId) {selected.clear(); anchor = null;}
    sessionId = response.sessionId; tabs = response.tabs;
    selected = retainSelection(selected, tabs);
    if (!tabs.some(tab => tab.tabId === anchor)) anchor = null;
    if (changed) render();
  } catch { if (!preserveNotice && !syncing) notify('无法读取标签列表，请重新加载扩展后刷新本页。', 'error'); }
  finally {
    loading = false; byId('refresh').disabled = false;
    if (refreshQueued) { refreshQueued = false; refresh(); }
  }
  await bridge();
}

async function syncOpenTabs() {
  if (syncing) return;
  syncing = true;
  byId('sync').disabled = true;
  byId('sync').setAttribute('aria-busy', 'true');
  byId('sync-label').textContent = '正在同步…';
  notify('正在读取已加载网页的原始标题…', 'progress');
  try {
    const result = await chrome.runtime.sendMessage({type: 'dia-tabs-sync'});
    if (!result?.ok) throw Error('sync failed');
    await refresh();
    const labels = {discarded: '已释放', frozen: '浏览器标记冻结', loading: '加载中', unloaded: '尚未加载', unknown_status: '状态未知', stale: '页面已变化', closed: '已关闭', timeout: '读取超时', capacity: '先前读取尚未结束'};
    const reasons = Object.entries(result.skipReasons || {}).map(([reason, count]) => `${labels[reason] || '其他'} ${count} 个`).join('，');
    notify(`同步完成：成功 ${result.captured} 个，无法读取 ${result.unavailable} 个，跳过 ${result.skipped} 个${reasons ? `（${reasons}）` : ''}。`, 'success');
  } catch { notify('同步失败，请稍后重试。', 'error'); }
  finally {
    syncing = false;
    byId('sync').disabled = false;
    byId('sync').setAttribute('aria-busy', 'false');
    byId('sync-label').textContent = '同步已打开标签';
  }
}

byId('search').addEventListener('input', render);
byId('refresh').addEventListener('click', refresh);
byId('sync').addEventListener('click', syncOpenTabs);
byId('reconnect').addEventListener('click', async () => { await bridge(true); setTimeout(() => bridge(), 500); });
byId('copy-urls').addEventListener('click', () => copy(selectedOutput(visibleTabs(), selected, 'url')));
byId('copy-markdown').addEventListener('click', () => copy(selectedOutput(visibleTabs(), selected, 'markdown')));
byId('select-all').addEventListener('change', event => {
  for (const tab of visibleTabs()) { if (event.target.checked) selected.add(tab.tabId); else selected.delete(tab.tabId); }
  render();
});
chrome.tabs.onCreated.addListener(() => refresh());
chrome.tabs.onRemoved.addListener(() => refresh());
chrome.tabs.onUpdated.addListener(() => refresh());
chrome.tabs.onActivated.addListener(() => refresh());
chrome.storage.onChanged.addListener((changes, area) => {
  const change = changes.diaTabsRecords;
  if (area === 'session' && change && JSON.stringify(change.oldValue) !== JSON.stringify(change.newValue)) refresh();
});
window.addEventListener('focus', refresh);
refresh();

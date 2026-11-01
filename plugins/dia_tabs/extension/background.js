import {createTabService} from './state.js';

const service = createTabService(chrome);
const hostName = 'com.mcoder2014.life_tools.dia_tabs';
let port = null;
let bridgeState = 'connecting';
let bridgeMessage = '';

function connectBridge() {
  if (port) return;
  bridgeState = 'connecting';
  bridgeMessage = '';
  try {
    const connection = chrome.runtime.connectNative(hostName);
    port = connection;
    bridgeState = 'connected';
    connection.onMessage.addListener(request => {
      service.handleRequest(request).then(response => {
        if (port === connection) {
          try { connection.postMessage(response); } catch { /* Disconnect handler schedules retry. */ }
        }
      });
    });
    connection.onDisconnect.addListener(() => {
      const error = chrome.runtime.lastError;
      if (port !== connection) return;
      port = null;
      bridgeState = 'disconnected';
      bridgeMessage = error?.message || '本机桥接已断开';
    });
  } catch {
    port = null;
    bridgeState = 'disconnected';
    bridgeMessage = '本机桥接不可用，请检查安装';
  }
}

chrome.runtime.onMessage.addListener((request, sender, reply) => {
  if (sender.id !== chrome.runtime.id) return false;
  if (request?.type === 'dia-tabs-title-changed' && sender.tab && sender.frameId === 0) {
    service.refreshTab(sender.tab.id);
    reply({ok: true});
    return false;
  }
  // Content scripts can only notify. List/activation/bridge controls belong to
  // the extension's own page; website input cannot choose a tab ID to activate.
  if (!sender.url?.startsWith(chrome.runtime.getURL(''))) return false;
  if (request?.type === 'dia-tabs-bridge-status' || request?.type === 'dia-tabs-reconnect') {
    if (request.type === 'dia-tabs-reconnect') connectBridge();
    reply({state: bridgeState, message: bridgeMessage});
    return false;
  }
  service.handleRequest(request).then(reply);
  return true;
});
chrome.alarms.onAlarm.addListener(alarm => {
  if (alarm.name === 'dia-tabs-reconnect') connectBridge();
});
chrome.action.onClicked.addListener(async () => {
  const url = chrome.runtime.getURL('control.html');
  const pages = await chrome.tabs.query({url});
  const existing = pages.find(tab => !tab.incognito);
  if (existing) {
    await chrome.tabs.update(existing.id, {active: true});
    const window = await chrome.windows.get(existing.windowId);
    if (window.state === 'minimized') await chrome.windows.update(existing.windowId, {state: 'normal'});
    await chrome.windows.update(existing.windowId, {focused: true});
  } else await chrome.tabs.create({url});
});
chrome.alarms.create('dia-tabs-reconnect', {periodInMinutes: 1});
connectBridge();
service.ready.catch(() => {});

import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const source = await readFile(new URL('../extension/state.js', import.meta.url), 'utf8').catch(() => '');
const {createTabService} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);

function event() {
  const listeners = [];
  return {addListener(fn) {listeners.push(fn);}, emit(...args) {for (const fn of listeners) fn(...args);}};
}
function fake(tabs, saved = {}) {
  const current = new Map(tabs.map(tab => [tab.id, {...tab}]));
  const calls = [];
  const storage = {...saved};
  const api = {
    tabs: {
      onUpdated:event(),onActivated:event(),onRemoved:event(),onReplaced:event(),onCreated:event(),
      async query() {return [...current.values()].map(tab => ({...tab}));},
      async get(id) {if (!current.has(id)) throw Error('closed'); return {...current.get(id)};},
      async update(id, changes) {const tab=await this.get(id); current.set(id,{...tab,...changes}); calls.push(['tab',id,changes]); return current.get(id);},
    },
    scripting: {async executeScript({target}) {calls.push(['script',target.tabId]);return [{frameId:0,result:{title:`DOM ${target.tabId}`,url:current.get(target.tabId)?.url,readyState:'complete'}}];}},
    storage:{session:{async get() {return structuredClone(storage);},async set(value) {Object.assign(storage,structuredClone(value));}}},
    windows:{async get(id) {return {id,state:'minimized'};},async update(id,value) {calls.push(['window',id,value]);}},
  };
  return {api,current,calls,storage};
}
const tab = (id, extra={}) => ({id,windowId:1,index:id,url:'https://example.com/',title:'USER RENAMED',active:true,status:'complete',incognito:false,...extra});

function service(api) {assert.equal(typeof createTabService,'function');return createTabService(api);}

test('only active complete HTTP(S) pages are sampled and tabs.title never supplies title', async () => {
  const f=fake([tab(1),tab(2,{active:false}),tab(3,{status:'loading'}),tab(4,{url:'chrome://settings/'}),tab(5,{incognito:true})]);
  const s=service(f.api); await s.ready;
  const rows=await s.list();
  assert.deepEqual(rows.map(row=>row.tabId),[1,2,3]);
  assert.equal(rows[0].title,'DOM 1');
  assert.equal(rows[1].titleState,'pending');
  assert.equal(rows[2].titleState,'pending');
  assert.ok(f.calls.filter(c=>c[0]==='script').every(c=>c[1]===1));
});

test('same-URL reload immediately clears title, then inactive tabs retain only current-page titles', async () => {
  const f=fake([tab(1)]); const s=service(f.api);await s.ready;
  f.current.set(1,tab(1,{active:false,status:'loading'}));
  f.api.tabs.onUpdated.emit(1,{status:'loading'},f.current.get(1));
  assert.equal((await s.list())[0].titleState,'pending');
  assert.equal((await s.list())[0].title,'');
  f.current.set(1,tab(1));f.api.tabs.onUpdated.emit(1,{status:'complete'},f.current.get(1));await s.idle();
  f.current.set(1,tab(1,{active:false}));
  assert.equal((await s.list())[0].title,'DOM 1');
});

test('a delayed sample cannot restore a title after same-URL navigation or closing', async () => {
  const f=fake([tab(1)]);const s=service(f.api);await s.ready;
  let release;
  f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
  f.api.tabs.onUpdated.emit(1,{title:'ignored'},f.current.get(1));
  await new Promise(resolve=>setImmediate(resolve));
  f.current.set(1,tab(1,{active:false,status:'loading'}));f.api.tabs.onUpdated.emit(1,{status:'loading'},f.current.get(1));
  release([{frameId:0,result:{title:'STALE',url:'https://example.com/',readyState:'complete'}}]);await s.idle();
  assert.equal((await s.list())[0].title,'');
  f.current.delete(1);f.api.tabs.onRemoved.emit(1);await s.idle();
  assert.deepEqual(await s.list(),[]);
});

test('worker restores valid session records and drops closed or URL-mismatched records', async () => {
  const saved={diaTabsSession:'session-before',diaTabsRecords:{1:{url:'https://example.com/',title:'Saved DOM',titleState:'captured'},2:{url:'https://old.example/',title:'OLD',titleState:'captured'},99:{url:'https://example.com/',title:'CLOSED',titleState:'captured'}}};
  const f=fake([tab(1,{active:false}),tab(2,{active:false})],saved);const s=service(f.api);await s.ready;
  assert.equal(s.sessionId,'session-before');
  const rows=await s.list();assert.equal(rows[0].title,'Saved DOM');assert.equal(rows[1].titleState,'pending');
  await s.idle();assert.equal(f.storage.diaTabsRecords[99],undefined);
});

test('same URL tabs activate by ID using current window, restoring minimized window', async () => {
  const f=fake([tab(1),tab(2,{windowId:7,active:false})]);const s=service(f.api);await s.ready;
  const request={v:1,id:'test',op:'activate',sessionId:s.sessionId,tabId:2};
  assert.deepEqual(await s.handleRequest(request),{v:1,id:'test',ok:true});
  assert.deepEqual(f.calls.filter(c=>c[0]==='tab'),[['tab',2,{active:true}]]);
  assert.deepEqual(f.calls.filter(c=>c[0]==='window'),[['window',7,{state:'normal'}],['window',7,{focused:true}]]);
  assert.equal((await s.handleRequest({...request,sessionId:'old'})).error,'STALE_SESSION');
  f.current.delete(2);assert.equal((await s.handleRequest(request)).error,'TAB_CLOSED');
  assert.equal((await s.handleRequest({v:1,id:'bad',op:'eval'})).error,'BAD_REQUEST');
});

test('script rejection is unavailable rather than tabs.title fallback', async () => {
  const f=fake([tab(1)]);f.api.scripting.executeScript=async()=>{throw Error('restricted');};
  const s=service(f.api);await s.ready;const row=(await s.list())[0];
  assert.equal(row.title,'');assert.equal(row.titleState,'unavailable');
});

test('title observer ignores style/meta churn and coalesces title mutations without starvation', async () => {
  const {runInNewContext} = await import('node:vm');
  const {readPageTitle} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  const callbacks=[],timers=[],messages=[];
  const context={document:{head:{},title:'DOM Title',readyState:'complete'},location:{href:'https://example.com/'},
    MutationObserver:class {constructor(callback) {callbacks.push(callback);}observe() {}},
    setTimeout(fn) {timers.push(fn);return timers.length;},clearTimeout() {},
    chrome:{runtime:{sendMessage(message) {messages.push(message);return Promise.resolve();}}},
  };
  runInNewContext(`(${readPageTitle.toString()})()`,context);
  callbacks[0]([{type:'childList',target:{nodeName:'STYLE'},addedNodes:[],removedNodes:[]}]);
  assert.equal(timers.length,0);
  const mutation={type:'characterData',target:{parentNode:{nodeName:'TITLE'}},addedNodes:[],removedNodes:[]};
  callbacks[0]([mutation]);callbacks[0]([mutation]);
  assert.equal(timers.length,1);
  timers[0]();assert.equal(messages.length,1);
});

test('private-window events never persist URL or title even if API exposes them', async () => {
  const f=fake([tab(1)]);const s=service(f.api);await s.ready;
  const privateTab=tab(7,{incognito:true,url:'https://example.org/private-demo'});
  f.current.set(7,privateTab);f.api.tabs.onUpdated.emit(7,{url:privateTab.url},privateTab);await s.idle();
  assert.equal(f.storage.diaTabsRecords[7],undefined);
  assert.equal((await s.list()).some(row=>row.tabId===7),false);
});

test('close during initial query cannot reinsert a stale restored record', async () => {
  const f=fake([tab(1,{active:false})]);let release;
  f.api.tabs.query=()=>new Promise(resolve=>{release=resolve;});
  const s=service(f.api);await new Promise(resolve=>setImmediate(resolve));
  f.current.delete(1);f.api.tabs.onRemoved.emit(1);
  release([tab(1,{active:false})]);await s.ready;await s.idle();
  assert.equal(f.storage.diaTabsRecords[1],undefined);
});

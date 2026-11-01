import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {runInNewContext} from 'node:vm';
const source = await readFile(new URL('../extension/state.js', import.meta.url), 'utf8').catch(() => '');
const {createTabService,readPageTitle} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);

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

function titleObserver(extra={}) {
  const callbacks=[],timers=[],delays=[],messages=[];
  const context={document:{head:{},title:'DOM Title',readyState:'complete'},location:{href:'https://example.com/'},
    MutationObserver:class {constructor(callback) {callbacks.push(callback);}observe() {}},
    setTimeout(fn,delay) {timers.push(fn);delays.push(delay);return timers.length;},clearTimeout() {},
    chrome:{runtime:{sendMessage(message) {messages.push(message);return Promise.resolve();}}},
    ...extra,
  };
  runInNewContext(`(${readPageTitle.toString()})()`,context);
  const notify=()=>callbacks[0]([{type:'characterData',target:{parentNode:{nodeName:'TITLE'}},addedNodes:[],removedNodes:[]}]);
  return {callbacks,timers,delays,messages,notify,context};
}

test('title observer immediately signals the first mutation without a title payload', () => {
  const f=titleObserver();f.notify();
  assert.deepEqual(structuredClone(f.messages),[{type:'dia-tabs-title-changed'}]);
  assert.equal(f.timers.length,1);assert.equal(f.delays[0],150);
  f.timers[0]();assert.equal(f.messages.length,1);
});

test('title observer coalesces later mutations and does not starve continuous changes', () => {
  const f=titleObserver();f.notify();f.notify();f.notify();
  assert.equal(f.messages.length,1);assert.equal(f.timers.length,1);
  f.timers[0]();assert.equal(f.messages.length,2);
  f.notify();f.notify();assert.equal(f.messages.length,3);assert.equal(f.timers.length,2);
  f.timers[1]();assert.equal(f.messages.length,4);
});

test('title observer ignores style and meta changes without notifications or timers', () => {
  const f=titleObserver();
  for(const nodeName of ['STYLE','META','LINK']) {
    f.callbacks[0]([{type:'childList',target:{nodeName},addedNodes:[],removedNodes:[]}]);
  }
  assert.equal(f.timers.length,0);assert.deepEqual(f.messages,[]);
});

test('title observer replaces V1 on existing pages and installs the new observer only once', () => {
  let disconnected=0;
  const f=titleObserver({__lifeToolsDiaTitleObserverV1:{disconnect() {disconnected++;}}});
  assert.equal(disconnected,1);assert.equal(f.callbacks.length,1);
  runInNewContext(`(${readPageTitle.toString()})()`,f.context);
  assert.equal(disconnected,1);assert.equal(f.callbacks.length,1);
  f.notify();assert.equal(f.messages.length,1);
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

test('explicit sync reads complete background pages without activating or waking skipped pages', async () => {
  const f=fake([tab(1,{active:false}),tab(2,{active:false}),tab(3,{active:false,status:'loading'}),
    tab(4,{active:false,discarded:true}),tab(5,{active:false,frozen:true}),tab(6,{incognito:true}),
    tab(7,{url:'chrome://settings/'}),tab(8,{active:false})]);
  const s=service(f.api);await s.ready;
  assert.deepEqual(f.calls,[]);
  f.api.tabs.reload=async()=>{throw Error('sync must not reload');};
  f.api.tabs.create=async()=>{throw Error('sync must not open tabs');};
  const execute=f.api.scripting.executeScript;
  f.api.scripting.executeScript=async options=>{if(options.target.tabId===8)throw Error('restricted');return execute(options);};
  assert.equal(typeof s.syncOpenTabs,'function');
  assert.deepEqual(await s.syncOpenTabs(),{captured:2,unavailable:1,skipped:3,skipReasons:{loading:1,discarded:1,frozen:1}});
  const rows=await s.list();
  assert.equal(rows.find(row=>row.tabId===1).title,'DOM 1');
  assert.equal(rows.find(row=>row.tabId===8).titleState,'unavailable');
  assert.deepEqual(f.calls.filter(call=>call[0]==='script').map(call=>call[1]).sort(),[1,2]);
  assert.ok(f.calls.every(call=>call[0]==='script'));
  f.api.tabs.onUpdated.emit(2,{title:'Renamed browser label'},f.current.get(2));await s.idle();
  assert.equal(f.calls.filter(call=>call[0]==='script').length,2);
  assert.equal((await s.handleRequest({v:1,id:'native-sync',op:'sync'})).error,'BAD_REQUEST');
});

test('sync shares one in-flight batch and never exceeds four simultaneous samples', async () => {
  const f=fake(Array.from({length:9},(_,index)=>tab(index+1,{active:false})));
  const s=service(f.api);await s.ready;
  const waiting=[];let running=0,maximum=0,total=0;
  f.api.scripting.executeScript=({target})=>new Promise(resolve=>{
    running++;total++;maximum=Math.max(maximum,running);
    waiting.push(()=>{running--;resolve([{frameId:0,result:{title:`Synced ${target.tabId}`,url:'https://example.com/',readyState:'complete'}}]);});
  });
  assert.equal(typeof s.syncOpenTabs,'function');
  const first=s.syncOpenTabs(),second=s.syncOpenTabs();assert.equal(first,second);
  await new Promise(resolve=>setImmediate(resolve));assert.equal(running,4);
  while(total<9 || waiting.length) {
    for(const release of waiting.splice(0)) release();
    await new Promise(resolve=>setImmediate(resolve));
  }
  assert.deepEqual(await first,{captured:9,unavailable:0,skipped:0,skipReasons:{}});
  assert.equal(maximum,4);assert.equal(total,9);
});

test('manual background results are rejected after navigation, close or a later automatic sample', async t => {
  for(const change of ['navigation','close','newer']) await t.test(change,async()=>{
    const f=fake([tab(1,{active:false})]);const s=service(f.api);await s.ready;let release;
    f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
    assert.equal(typeof s.syncOpenTabs,'function');const pending=s.syncOpenTabs();
    await new Promise(resolve=>setImmediate(resolve));
    if(change==='navigation') {
      f.current.set(1,tab(1,{active:false,status:'loading'}));f.api.tabs.onUpdated.emit(1,{status:'loading'},f.current.get(1));
    } else if(change==='close') {f.current.delete(1);f.api.tabs.onRemoved.emit(1);}
    else {
      f.api.scripting.executeScript=async()=>[{frameId:0,result:{title:'Newer DOM',url:'https://example.com/',readyState:'complete'}}];
      f.current.set(1,tab(1));f.api.tabs.onUpdated.emit(1,{title:'browser label'},f.current.get(1));
      await new Promise(resolve=>setImmediate(resolve));
    }
    release([{frameId:0,result:{title:'Stale manual title',url:'https://example.com/',readyState:'complete'}}]);
    assert.deepEqual(await pending,{captured:0,unavailable:0,skipped:1,skipReasons:{[change==='close'?'closed':'stale']:1}});await s.idle();
    assert.equal(f.storage.diaTabsRecords[1]?.title,change==='close'?undefined:change==='newer'?'Newer DOM':'');
  });
});

test('sync rechecks discarded/frozen before injection and rejects a page frozen during sampling', async () => {
  const f=fake([tab(1,{active:false}),tab(2,{active:false})]);const s=service(f.api);await s.ready;
  const query=f.api.tabs.query;
  f.api.tabs.query=async()=>{const snapshot=await query();f.current.set(1,tab(1,{active:false,discarded:true}));return snapshot;};
  let scripts=0;
  f.api.scripting.executeScript=async({target})=>{
    scripts++;f.current.set(target.tabId,tab(target.tabId,{active:false,frozen:true}));
    return [{frameId:0,result:{title:'Frozen result',url:'https://example.com/',readyState:'complete'}}];
  };
  assert.equal(typeof s.syncOpenTabs,'function');
  assert.deepEqual(await s.syncOpenTabs(),{captured:0,unavailable:0,skipped:2,skipReasons:{discarded:1,frozen:1}});assert.equal(scripts,1);
});

test('a stalled manual script times out as skipped and late execution cannot restore its result', async t => {
  const f=fake([tab(1,{active:false})]);const s=service(f.api);await s.ready;
  let release;f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
  t.mock.timers.enable({apis:['setTimeout']});
  assert.equal(typeof s.syncOpenTabs,'function');const pending=s.syncOpenTabs();
  await new Promise(resolve=>setImmediate(resolve));t.mock.timers.tick(5000);
  assert.deepEqual(await pending,{captured:0,unavailable:0,skipped:1,skipReasons:{timeout:1}});
  release([{frameId:0,result:{title:'Too late',url:'https://example.com/',readyState:'complete'}}]);
  await new Promise(resolve=>setImmediate(resolve));await s.idle();
  assert.equal(f.storage.diaTabsRecords[1]?.title,'');
});

test('timed-out scripts keep the four execution slots across repeated sync batches', async t => {
  const f=fake(Array.from({length:9},(_,index)=>tab(index+1,{active:false})));
  const s=service(f.api);await s.ready;const release=[];let calls=0;
  f.api.scripting.executeScript=()=>{calls++;return new Promise(resolve=>release.push(resolve));};
  t.mock.timers.enable({apis:['setTimeout']});
  const first=s.syncOpenTabs();await new Promise(resolve=>setImmediate(resolve));assert.equal(calls,4);
  t.mock.timers.tick(5000);await new Promise(resolve=>setImmediate(resolve));assert.equal(calls,4);
  assert.deepEqual(await first,{captured:0,unavailable:0,skipped:9,skipReasons:{timeout:4,capacity:5}});
  assert.deepEqual(await s.syncOpenTabs(),{captured:0,unavailable:0,skipped:9,skipReasons:{capacity:9}});assert.equal(calls,4);
  for(const resolve of release)resolve([{frameId:0,result:{title:'Late',url:'https://example.com/',readyState:'complete'}}]);
  await new Promise(resolve=>setImmediate(resolve));
});


test('list exposes live diagnostic fields and sync prioritizes discarded over unloaded status', async () => {
  const f=fake([tab(1,{active:false,status:'unloaded',discarded:true}),tab(2,{active:false,status:undefined}),tab(3,{active:false,status:'loading'}),tab(4,{active:false,frozen:true}),tab(5,{active:false,status:'unloaded'})]);
  const s=service(f.api);await s.ready;
  const rows=await s.list();
  assert.equal(rows[0].status,'unloaded');assert.equal(rows[0].discarded,true);
  assert.equal(rows[1].status,'unknown');assert.equal(rows[1].discarded,null);assert.equal(rows[1].frozen,null);
  assert.deepEqual((await s.syncOpenTabs()).skipReasons,{discarded:1,unknown_status:1,loading:1,frozen:1,unloaded:1});
  assert.equal(f.calls.length,0);
});

test('active pages are sampled after unfreezing or restoration, but inactive pages are not', async t => {
  for(const field of ['frozen','discarded']) for(const active of [true,false]) await t.test(`${field}, active=${active}`,async()=>{
    const f=fake([tab(1,{active:false,[field]:true})]);const s=service(f.api);await s.ready;
    f.current.set(1,tab(1,{active,[field]:true}));f.api.tabs.onActivated.emit({tabId:1});await s.idle();
    assert.equal(f.calls.length,0);
    f.current.set(1,tab(1,{active,[field]:false}));f.api.tabs.onUpdated.emit(1,{[field]:false},f.current.get(1));await s.idle();
    f.current.set(1,{...f.current.get(1),active:false});
    const row=(await s.list())[0];
    assert.equal(row.titleState,active?'captured':'pending');
    assert.equal(row.title,active?'DOM 1':'');
    assert.equal(f.calls.filter(call=>call[0]==='script').length,active?1:0);
  });
});

test('repeated URLs preserve background titles while real URL changes clear them', async () => {
  const f=fake([tab(1)]);const s=service(f.api);await s.ready;
  f.current.set(1,tab(1,{active:false}));f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url,title:'Browser label'},f.current.get(1));await s.idle();
  assert.equal((await s.list())[0].title,'DOM 1');
  assert.equal(f.calls.filter(call=>call[0]==='script').length,1);
  f.current.set(1,tab(1));f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url},f.current.get(1));await s.idle();
  assert.equal(f.calls.filter(call=>call[0]==='script').length,2);
  f.current.set(1,tab(1,{active:false,url:'https://example.org/next'}));
  f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url},f.current.get(1));await s.idle();
  const row=(await s.list())[0];assert.equal(row.title,'');assert.equal(row.titleState,'pending');
});

test('repeated URLs keep an in-flight manual sample valid, while a real same-URL reload rejects it', async t => {
  for(const reload of [false,true]) await t.test(`reload=${reload}`,async()=>{
    const f=fake([tab(1)]);const s=service(f.api);await s.ready;
    f.current.set(1,tab(1,{active:false}));let release;
    f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
    const pending=s.syncOpenTabs();await new Promise(resolve=>setImmediate(resolve));
    f.current.set(1,tab(1,{active:false,status:reload?'loading':'complete'}));
    f.api.tabs.onUpdated.emit(1,reload?{status:'loading'}:{url:f.current.get(1).url},f.current.get(1));
    f.current.set(1,tab(1,{active:false}));
    if(reload) f.api.tabs.onUpdated.emit(1,{status:'complete'},f.current.get(1));
    release([{frameId:0,result:{title:'Current DOM',url:'https://example.com/',readyState:'complete'}}]);
    assert.deepEqual(await pending,reload?{captured:0,unavailable:0,skipped:1,skipReasons:{stale:1}}:{captured:1,unavailable:0,skipped:0,skipReasons:{}});
    await s.idle();assert.equal((await s.list())[0].title,reload?'':'Current DOM');
  });
});

test('lifecycle loading preserves captured DOM titles unless the URL changes', async t => {
  for(const lifecycle of [{frozen:true},{discarded:true},{status:'unloaded'}]) await t.test(JSON.stringify(lifecycle),async()=>{
    const f=fake([tab(1)]);const s=service(f.api);await s.ready;
    const state={active:false,status:'loading',title:'Browser placeholder',...lifecycle};
    f.current.set(1,tab(1,state));
    f.api.tabs.onUpdated.emit(1,{status:'loading',...lifecycle,title:'Browser placeholder'},f.current.get(1));await s.idle();
    const retained=(await s.list())[0];assert.equal(retained.title,'DOM 1');assert.equal(retained.titleState,'captured');
    f.current.set(1,tab(1,{...state,url:'https://example.org/next'}));
    f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url,status:'loading'},f.current.get(1));await s.idle();
    const changed=(await s.list())[0];assert.equal(changed.title,'');assert.equal(changed.titleState,'pending');
  });
});

test('a sample crossing a complete lifecycle suspension cannot replace the retained title', async t => {
  for(const field of ['frozen','discarded']) for(const loading of [false,true]) await t.test(`${field}, loading=${loading}`,async()=>{
    const f=fake([tab(1)]);const s=service(f.api);await s.ready;
    f.current.set(1,tab(1,{active:false}));let release;
    f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
    const pending=s.syncOpenTabs();await new Promise(resolve=>setImmediate(resolve));
    f.current.set(1,tab(1,{active:false,[field]:true,status:loading?'loading':'complete'}));
    f.api.tabs.onUpdated.emit(1,{[field]:true,...loading?{status:'loading'}:{}},f.current.get(1));
    f.current.set(1,tab(1,{active:false,[field]:false}));
    f.api.tabs.onUpdated.emit(1,{[field]:false},f.current.get(1));
    release([{frameId:0,result:{title:'Before suspension',url:'https://example.com/',readyState:'complete'}}]);
    assert.deepEqual(await pending,{captured:0,unavailable:0,skipped:1,skipReasons:{stale:1}});await s.idle();
    assert.equal((await s.list())[0].title,'DOM 1');
  });
});

test('an unloaded page rejects an earlier sample after returning to complete', async () => {
  const f=fake([tab(1)]);const s=service(f.api);await s.ready;
  f.current.set(1,tab(1,{active:false}));let release;
  f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
  const pending=s.syncOpenTabs();await new Promise(resolve=>setImmediate(resolve));
  f.current.set(1,tab(1,{active:false,status:'unloaded'}));f.api.tabs.onUpdated.emit(1,{status:'unloaded'},f.current.get(1));
  f.current.set(1,tab(1,{active:false}));f.api.tabs.onUpdated.emit(1,{status:'complete'},f.current.get(1));
  release([{frameId:0,result:{title:'Before unload',url:'https://example.com/',readyState:'complete'}}]);
  assert.deepEqual(await pending,{captured:0,unavailable:0,skipped:1,skipReasons:{stale:1}});
  await s.idle();assert.equal((await s.list())[0].title,'DOM 1');
});

test('an automatic sample finishing after the tab becomes inactive retains the previous title', async () => {
  const f=fake([tab(1)]);const s=service(f.api);await s.ready;let release;
  f.api.scripting.executeScript=()=>new Promise(resolve=>{release=resolve;});
  const pending=s.refreshTab(1);await new Promise(resolve=>setImmediate(resolve));
  f.current.set(1,tab(1,{active:false}));
  release([{frameId:0,result:{title:'After leaving',url:'https://example.com/',readyState:'complete'}}]);await pending;
  assert.equal((await s.list())[0].title,'DOM 1');
});

test('initial updates distinguish repeated URLs, navigation and real same-URL reloads', async t => {
  for(const change of ['repeat','navigation','roundtrip','reload','lifecycle']) await t.test(change,async()=>{
    const saved={diaTabsSession:'saved-session',diaTabsRecords:{1:{url:'https://example.com/',title:'Saved DOM',titleState:'captured'}}};
    const f=fake([tab(1,{active:false})]);let release;
    f.api.storage.session.get=()=>new Promise(resolve=>{release=resolve;});
    const s=service(f.api);
    if(change==='navigation' || change==='roundtrip') {
      f.current.set(1,tab(1,{active:false,url:'https://example.org/next'}));
      f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url},f.current.get(1));
    }
    if(change==='roundtrip') f.current.set(1,tab(1,{active:false}));
    if(change==='reload') f.current.set(1,tab(1,{active:false,status:'loading'}));
    if(change==='lifecycle') f.current.set(1,tab(1,{active:false,status:'loading',discarded:true}));
    f.api.tabs.onUpdated.emit(1,change==='reload'?{status:'loading'}:change==='lifecycle'?{discarded:true,status:'loading'}:{url:f.current.get(1).url},f.current.get(1));
    release(saved);await s.ready;await s.idle();
    const row=(await s.list())[0],preserve=change==='repeat'||change==='lifecycle';
    assert.equal(row.title,preserve?'Saved DOM':'');assert.equal(row.titleState,preserve?'captured':'pending');
    assert.equal(f.calls.length,0);
  });
});

test('initial query updates cannot restore closed or replaced tab records', async t => {
  for(const change of ['navigation','remove','replace']) await t.test(change,async()=>{
    const saved={diaTabsSession:'saved-session',diaTabsRecords:{1:{url:'https://example.com/',title:'Saved DOM',titleState:'captured'}}};
    const f=fake([tab(1,{active:false})],saved);let release;
    f.api.tabs.query=()=>new Promise(resolve=>{release=resolve;});
    const s=service(f.api);await new Promise(resolve=>setImmediate(resolve));
    f.current.set(1,tab(1,{active:false,url:'https://example.org/next'}));
    f.api.tabs.onUpdated.emit(1,{url:f.current.get(1).url},f.current.get(1));
    if(change==='remove') {f.current.delete(1);f.api.tabs.onRemoved.emit(1);}
    if(change==='replace') {f.current.delete(1);f.current.set(2,tab(2,{active:false}));f.api.tabs.onReplaced.emit(2,1);}
    release([tab(1,{active:false})]);await s.ready;await s.idle();
    if(change==='navigation') assert.deepEqual(f.storage.diaTabsRecords[1],{url:'https://example.org/next',title:'',titleState:'pending'});
    else assert.equal(f.storage.diaTabsRecords[1],undefined);
  });
});

test('a stale initial query cannot inject into a page suspended while the query was pending', async () => {
  const saved={diaTabsSession:'saved-session',diaTabsRecords:{1:{url:'https://example.com/',title:'Saved DOM',titleState:'captured'}}};
  const f=fake([tab(1)],saved);let release;
  f.api.tabs.query=()=>new Promise(resolve=>{release=resolve;});
  const s=service(f.api);await new Promise(resolve=>setImmediate(resolve));
  f.current.set(1,tab(1,{frozen:true}));f.api.tabs.onUpdated.emit(1,{frozen:true},f.current.get(1));
  release([tab(1)]);await s.ready;await s.idle();
  assert.equal(f.calls.length,0);assert.equal(f.storage.diaTabsRecords[1].title,'Saved DOM');
});

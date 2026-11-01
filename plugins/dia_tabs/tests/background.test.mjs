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

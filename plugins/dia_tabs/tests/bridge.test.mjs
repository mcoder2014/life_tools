import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {runInNewContext} from 'node:vm';
const source=(await readFile(new URL('../extension/background.js',import.meta.url),'utf8')).replace(/^import .*;\n/,'');

function harness(unavailable=false) {
  const events={},ports=[],alarms=[],notices=[];
  const event=name=>({addListener(fn){events[name]=fn;}});
  const service={ready:Promise.resolve(),refreshTab:id=>notices.push(id),async handleRequest(request){return {v:1,id:request.id,ok:true,sessionId:'session',tabs:[]};}};
  const api={
    runtime:{id:'extension-id',getURL:path=>`chrome-extension://extension-id/${path}`,onMessage:event('message'),
      connectNative(name){assert.equal(name,'com.mcoder2014.life_tools.dia_tabs');if(unavailable)throw Error('not installed');
        const messages=[],port={name,messages,postMessage:value=>messages.push(value),
          onMessage:{addListener(fn){port.receive=fn;}},onDisconnect:{addListener(fn){port.disconnect=fn;}}};ports.push(port);return port;}},
    alarms:{onAlarm:event('alarm'),create(name,options){alarms.push({name,options});}},
    action:{onClicked:event('action')},
  };
  runInNewContext(source,{chrome:api,createTabService:()=>service});
  return {api,events,ports,alarms,notices,service};
}

test('native bridge forwards protocol, retries only on minute alarm and reports disconnection',async()=>{
  const f=harness();assert.equal(f.ports.length,1);assert.equal(f.alarms[0].options.periodInMinutes,1);
  f.ports[0].receive({v:1,id:'native-request',op:'list'});await new Promise(resolve=>setImmediate(resolve));
  assert.equal(f.ports[0].messages[0].id,'native-request');assert.equal(f.ports[0].messages[0].ok,true);
  f.api.runtime.lastError={message:'Host disconnected'};f.ports[0].disconnect();delete f.api.runtime.lastError;
  let status;f.events.message({type:'dia-tabs-bridge-status'},{id:'extension-id',url:'chrome-extension://extension-id/control.html'},value=>status=value);
  assert.equal(status.state,'disconnected');assert.equal(f.ports.length,1);
  f.events.alarm({name:'unrelated'});assert.equal(f.ports.length,1);
  f.events.alarm({name:'dia-tabs-reconnect'});assert.equal(f.ports.length,2);
});

test('only trusted extension pages can sync; errors reply and native cannot call sync',async()=>{
  const f=harness();let calls=0,response;
  f.service.syncOpenTabs=async()=>{calls++;return {captured:2,unavailable:1,skipped:3};};
  const control={id:'extension-id',url:'chrome-extension://extension-id/control.html'};
  assert.equal(f.events.message({type:'dia-tabs-sync'},control,value=>response=value),true);
  await new Promise(resolve=>setImmediate(resolve));assert.equal(calls,1);assert.equal(response.captured,2);
  response=undefined;
  assert.equal(f.events.message({type:'dia-tabs-sync'},{id:'extension-id',url:'https://example.com/',tab:{id:7},frameId:0},()=>{}),false);
  f.ports[0].receive({v:1,id:'native',type:'dia-tabs-sync'});await new Promise(resolve=>setImmediate(resolve));assert.equal(calls,1);
  f.service.syncOpenTabs=async()=>{throw Error('private diagnostic');};
  f.events.message({type:'dia-tabs-sync'},control,value=>response=value);await new Promise(resolve=>setImmediate(resolve));
  assert.equal(response.ok,false);assert.equal(response.error,'INTERNAL_ERROR');assert.ok(!response.message.includes('private diagnostic'));
});

test('control remains usable without native host and content/foreign messages cannot activate tabs',async()=>{
  const f=harness(true);let response;
  const control={id:'extension-id',url:'chrome-extension://extension-id/control.html'};
  assert.equal(f.events.message({v:1,id:'ui-request',op:'list'},control,value=>response=value),true);
  await new Promise(resolve=>setImmediate(resolve));assert.equal(response.ok,true);
  response=undefined;
  assert.equal(f.events.message({v:1,id:'bad',op:'activate'}, {id:'other-id',url:control.url},()=>{throw Error('foreign reply');}),false);
  assert.equal(f.events.message({v:1,id:'bad',op:'activate'}, {id:'extension-id',url:'https://example.com/',tab:{id:7},frameId:0},()=>{throw Error('content activation');}),false);
  f.events.message({type:'dia-tabs-title-changed'},{id:'extension-id',tab:{id:7},frameId:0},()=>{});
  assert.deepEqual(f.notices,[7]);
  f.events.message({type:'dia-tabs-title-changed'},{id:'extension-id',tab:{id:8},frameId:1},()=>{});
  assert.deepEqual(f.notices,[7]);
});

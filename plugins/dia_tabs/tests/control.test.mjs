import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const source = await readFile(new URL('../extension/control-core.js', import.meta.url), 'utf8').catch(() => '');
const core = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const cases = JSON.parse(await readFile(new URL('./markdown_cases.json', import.meta.url)));

test('Markdown escapes titles and destinations consistently with Alfred fixtures', () => {
  assert.equal(typeof core.markdownLink, 'function');
  for (const item of cases) assert.equal(core.markdownLink(item.title, item.url), item.expected);
});

test('selection supports plain, Ctrl/Cmd toggle and Shift range in displayed order', () => {
  assert.equal(typeof core.updateSelection, 'function');
  const ids = [11, 22, 33, 44];
  let state = core.updateSelection(new Set(), null, 22, ids, {});
  assert.deepEqual([...state.selected], [22]);
  state = core.updateSelection(state.selected, state.anchor, 44, ids, {metaKey: true});
  assert.deepEqual([...state.selected], [22,44]);
  state = core.updateSelection(state.selected, state.anchor, 33, ids, {shiftKey:true});
  assert.deepEqual([...state.selected], [33,44]);
  state = core.updateSelection(state.selected, state.anchor, 33, ids, {ctrlKey:true});
  assert.deepEqual([...state.selected], [44]);
});

test('copy uses displayed order, keeps duplicate URLs and clears closed selection', () => {
  assert.equal(typeof core.selectedOutput, 'function');
  const tabs = [
    {tabId:3, title:'Third', titleState:'captured', url:'https://example.com/'},
    {tabId:1, title:'ignored stale title', titleState:'pending', url:'https://example.org/'},
    {tabId:2, title:'Duplicate', titleState:'captured', url:'https://example.com/'},
  ];
  const selected = core.retainSelection(new Set([1,2,3,99]), tabs);
  assert.deepEqual([...selected], [1,2,3]);
  assert.equal(core.selectedOutput(tabs, selected, 'url'), 'https://example.com/\nhttps://example.org/\nhttps://example.com/');
  assert.equal(core.selectedOutput(tabs, selected, 'markdown'), '[Third](https://example.com/)\n[https://example.org/](https://example.org/)\n[Duplicate](https://example.com/)');
});

test('control page prevents default URL navigation, preserves modified selection and stops copy bubbling', async () => {
  const {runInNewContext} = await import('node:vm');
  class Element {
    constructor(tag='div') {this.tag=tag;this.children=[];this.listeners={};this.value='';this.classList={toggle(){}};}
    addEventListener(type, fn) {this.listeners[type]=fn;}
    setAttribute() {}
    append(...children) {this.children.push(...children);}
    replaceChildren() {this.children=[];}
  }
  const ids=['search','tabs','copy-urls','copy-markdown','select-all','empty','count','notice','refresh','sync','reconnect','bridge-status','bridge-detail'];
  const elements=Object.fromEntries(ids.map(id=>[id,new Element()]));
  const requests=[],copied=[];
  const rows=[{tabId:11,windowId:1,index:0,url:'https://example.com/',title:'DOM title',titleState:'captured',active:false}];
  const eventApi={addListener(){}};
  const context={...core,crypto:{randomUUID:()=> 'request'},setTimeout,window:{addEventListener(){}},
    document:{getElementById:id=>elements[id],createElement:tag=>new Element(tag)},
    navigator:{clipboard:{async writeText(text){copied.push(text);}}},
    chrome:{runtime:{async sendMessage(request){requests.push(request);if(request.op==='list')return {ok:true,sessionId:'session',tabs:rows};if(request.op==='activate')return {ok:true};return {state:'disconnected'};}},
      tabs:{onCreated:eventApi,onRemoved:eventApi,onUpdated:eventApi,onActivated:eventApi},storage:{onChanged:eventApi}},
  };
  const uiSource=(await readFile(new URL('../extension/control.js',import.meta.url),'utf8')).replace(/^import .*;\n/,'');
  runInNewContext(uiSource,context);await new Promise(resolve=>setImmediate(resolve));
  const first=elements.tabs.children[0];
  const title=first.children[1].children[0];
  const link=first.children[2].children[0];
  assert.equal(title.title,'DOM title');
  assert.equal(link.title,'https://example.com/');
  const click={defaultPrevented:false,stopped:false,preventDefault(){this.defaultPrevented=true;},stopPropagation(){this.stopped=true;}};
  link.listeners.click(click);await new Promise(resolve=>setImmediate(resolve));
  assert.equal(click.defaultPrevented,true);assert.equal(click.stopped,true);
  assert.equal(requests.filter(request=>request.op==='activate').length,1);
  assert.equal(requests.find(request=>request.op==='activate').tabId,11);
  const modified={...click,defaultPrevented:false,stopped:false,metaKey:true};
  link.listeners.click(modified);assert.equal(modified.defaultPrevented,true);assert.equal(modified.stopped,false);
  first.listeners.click(modified); // Native event bubbling reaches the selectable row.
  assert.equal(requests.filter(request=>request.op==='activate').length,1);
  assert.equal(elements['copy-urls'].disabled,false);
  const copyClick={stopped:false,stopPropagation(){this.stopped=true;}};
  elements.tabs.children[0].children[4].children[0].listeners.click(copyClick);
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(copyClick.stopped,true);assert.deepEqual(copied,['https://example.com/']);
});

test('refresh queues events during list and refreshes captured titles after session change', async () => {
  const {runInNewContext}=await import('node:vm');
  class Element {
    constructor(){this.children=[];this.listeners={};this.value='';this.classList={toggle(){}};this.renders=0;}
    addEventListener(type,fn){this.listeners[type]=fn;}setAttribute(){}append(...items){this.children.push(...items);}
    replaceChildren(){this.children=[];this.renders++;}
  }
  const elements={};const events={};
  const tab={tabId:1,windowId:1,index:0,url:'https://example.com/',title:'Old',titleState:'captured',active:false};
  let resolveFirst, calls=0, rows=[tab];
  const makeEvent=name=>({addListener(fn){events[name]=fn;}});
  const context={...core,crypto:{randomUUID:()=> 'request'},setTimeout,window:{addEventListener(){}},
    document:{getElementById:id=>elements[id] ||= new Element(),createElement:()=>new Element()},navigator:{clipboard:{async writeText(){}}},
    chrome:{runtime:{async sendMessage(request){if(request.op!=='list')return {state:'connected'};calls++;if(calls===1)return new Promise(resolve=>{resolveFirst=resolve;});return {ok:true,sessionId:'s',tabs:rows};}},
      tabs:{onCreated:makeEvent('created'),onRemoved:makeEvent('removed'),onUpdated:makeEvent('updated'),onActivated:makeEvent('activated')},storage:{onChanged:makeEvent('storage')}},
  };
  const ui=(await readFile(new URL('../extension/control.js',import.meta.url),'utf8')).replace(/^import .*;\n/,'');
  runInNewContext(ui,context);
  rows=[];events.removed(1);resolveFirst({ok:true,sessionId:'s',tabs:[tab]});
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(calls,2);assert.equal(elements.tabs.children.length,0);
  rows=[{...tab,title:'New DOM title'}];
  assert.equal(typeof events.storage,'function');
  events.storage({diaTabsRecords:{oldValue:{1:{title:'Old'}},newValue:{1:{title:'New DOM title'}}}},'session');
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(elements.tabs.children[0].children[1].children[0].textContent,'New DOM title');
  const previousCalls=calls,previousRenders=elements.tabs.renders;
  events.storage({diaTabsRecords:{oldValue:{1:{title:'New DOM title'}},newValue:{1:{title:'New DOM title'}}}},'session');
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(calls,previousCalls);
  events.updated(1,{title:'Unrelated UI title'});await new Promise(resolve=>setImmediate(resolve));
  assert.equal(elements.tabs.renders,previousRenders);
});

test('sync button shows counts, preserves selection, ignores repeat clicks and recovers after failure',async()=>{
  const {runInNewContext}=await import('node:vm');
  class Element {
    constructor(){this.children=[];this.listeners={};this.value='';this.classList={toggle(){}};}
    addEventListener(type,fn){this.listeners[type]=fn;}setAttribute(){}append(...items){this.children.push(...items);}replaceChildren(){this.children=[];}
  }
  const textOf=element=>[element.textContent,...element.children.map(textOf)].filter(Boolean).join(' ');
  const elements={},event={addListener(){}};let resolveSync,syncCalls=0,fail=false;
  let rows=[{tabId:1,windowId:1,index:0,url:'https://example.com/',title:'Old DOM',titleState:'captured',active:false,status:'unloaded',discarded:true,frozen:false}];
  const context={...core,crypto:{randomUUID:()=> 'request'},setTimeout,window:{addEventListener(){}},
    document:{getElementById:id=>elements[id] ||= new Element(),createElement:()=>new Element()},navigator:{clipboard:{async writeText(){}}},
    chrome:{runtime:{async sendMessage(request){if(request.type==='dia-tabs-sync'){syncCalls++;if(fail)throw Error('bridge gone');return new Promise(resolve=>{resolveSync=resolve;});}if(request.op==='list')return {ok:true,sessionId:'s',tabs:rows};return {state:'connected'};}},
      tabs:{onCreated:event,onRemoved:event,onUpdated:event,onActivated:event},storage:{onChanged:event}},
  };
  const ui=(await readFile(new URL('../extension/control.js',import.meta.url),'utf8')).replace(/^import .*;\n/,'');
  runInNewContext(ui,context);await new Promise(resolve=>setImmediate(resolve));
  assert.match(textOf(elements.tabs.children[0].children[3]),/已记录.*已释放/);
  elements.tabs.children[0].listeners.click({});
  assert.equal(typeof elements.sync?.listeners.click,'function');
  const pending=elements.sync.listeners.click();assert.equal(elements.sync.disabled,true);assert.equal(elements['sync-label'].textContent,'正在同步…');
  elements.sync.listeners.click();assert.equal(syncCalls,1);
  rows=[{...rows[0],title:'New DOM'}];resolveSync({ok:true,captured:1,unavailable:2,skipped:4,skipReasons:{discarded:1,frozen:1,unknown_status:1,title_regression:1}});await pending;
  assert.equal(elements.sync.disabled,false);assert.equal(elements['sync-label'].textContent,'同步已打开标签');
  assert.equal(elements.tabs.children[0].children[0].children[0].checked,true);
  assert.equal(elements.tabs.children[0].children[1].children[0].textContent,'New DOM');
  assert.match(elements.notice.textContent,/成功 1.*无法读取 2.*跳过 4.*已释放 1.*状态未知 1/);
  assert.match(elements.notice.textContent,/浏览器标记冻结 1 个/);
  assert.match(elements.notice.textContent,/标题退化，已保留 1 个/);
  for (const [status, frozen, expected] of [['complete',true,'浏览器标记冻结'],['loading',false,'加载中'],['unloaded',false,'尚未加载'],['unknown',false,'状态未知']]) {
    rows=[{...rows[0],discarded:false,frozen,status}];
    await elements.refresh.listeners.click();
    assert.ok(textOf(elements.tabs.children[0].children[3]).includes(expected));
    assert.equal(elements.tabs.children[0].children[1].children[0].textContent,'New DOM');
  }
  fail=true;await elements.sync.listeners.click();assert.equal(elements.sync.disabled,false);assert.match(elements.notice.textContent,/同步失败/);
});

test('bridge failure keeps list actions available and clears stale detail after recovery',async()=>{
  const {runInNewContext}=await import('node:vm');
  class Element {
    constructor(){this.children=[];this.listeners={};this.value='';this.classList={toggle(){}};}
    addEventListener(type,fn){this.listeners[type]=fn;}setAttribute(){}append(...items){this.children.push(...items);}replaceChildren(){this.children=[];}
  }
  const elements={},event={addListener(){}},copied=[];
  const rows=[{tabId:1,windowId:1,index:0,url:'https://example.com/',title:'Page',titleState:'captured',active:false}];
  let bridgeResult={state:'disconnected',message:'Specified native messaging host not found'};
  const context={...core,crypto:{randomUUID:()=> 'request'},setTimeout,window:{addEventListener(){}},
    document:{getElementById:id=>elements[id] ||= new Element(),createElement:()=>new Element()},navigator:{clipboard:{async writeText(text){copied.push(text);}}},
    chrome:{runtime:{async sendMessage(request){if(request.op==='list')return {ok:true,sessionId:'s',tabs:rows};return bridgeResult;}},
      tabs:{onCreated:event,onRemoved:event,onUpdated:event,onActivated:event},storage:{onChanged:event}},
  };
  const ui=(await readFile(new URL('../extension/control.js',import.meta.url),'utf8')).replace(/^import .*;\n/,'');
  runInNewContext(ui,context);await new Promise(resolve=>setImmediate(resolve));
  assert.equal(elements['bridge-status'].textContent,'Alfred 连接未就绪');
  assert.equal(elements['bridge-detail'].hidden,false);
  assert.equal(elements['bridge-detail'].textContent,bridgeResult.message);
  assert.equal(elements.tabs.children.length,1);
  elements.tabs.children[0].listeners.click({});
  await elements['copy-urls'].listeners.click();
  assert.deepEqual(copied,['https://example.com/']);
  bridgeResult={state:'connected'};
  await elements.refresh.listeners.click();
  assert.equal(elements['bridge-status'].textContent,'Alfred 连接就绪');
  assert.equal(elements['bridge-detail'].hidden,true);
  assert.equal(elements['bridge-detail'].textContent,'');
});

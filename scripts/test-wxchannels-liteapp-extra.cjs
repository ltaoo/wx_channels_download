const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname + '/wxchannels-liteapp-extra.js', 'utf8');

function make_page(origin, mounted) {
  const nodes = [], reports = [], timers = [];
  const app = {_uid:42};
  const root = {__vue_app__: mounted ? app : null};
  const document = {
    getElementById: () => root,
    querySelector: () => nodes[0] || null,
    createElement: () => ({style:{},attributes:{},setAttribute(key,value){this.attributes[key]=value;}}),
    body: mounted ? {appendChild(node){nodes.push(node);}} : null
  };
  const context = vm.createContext({location:{origin,pathname:'/a/finder/wxalite28e098851253e34f4777f7ca024236fd/pages/lite/feed'},document,URLSearchParams,
    fetch(url){reports.push(new URL(url));return Promise.resolve();},setTimeout(callback){timers.push(callback);}});
  return {context,nodes,reports,timers,app,root,document};
}

const page = make_page('https://liteapp.weixin.qq.com',true);
vm.runInContext(source,page.context);
vm.runInContext(source,page.context);
assert.equal(page.nodes.length,1);
assert.equal(page.nodes[0].textContent,'WXMITM_LITEAPP_H');
assert.equal(page.nodes[0].attributes['data-n'],'wxchannels-mitm-extra-marker');
assert.equal(page.root.__vue_app__,page.app);
assert.equal(page.reports.length,2);
assert.equal(page.reports[1].searchParams.get('app_preserved'),'true');

const waiting = make_page('https://liteapp.weixin.qq.com',false);
vm.runInContext(source,waiting.context);
assert.equal(waiting.nodes.length,0);
waiting.root.__vue_app__ = waiting.app;
waiting.document.body = {appendChild(node){waiting.nodes.push(node);}};
waiting.timers.shift()();
assert.equal(waiting.nodes.length,1);
assert.equal(waiting.root.__vue_app__,waiting.app);

const h5 = make_page('https://channels.weixin.qq.com',true);
vm.runInContext(source,h5.context);
assert.equal(h5.nodes.length,0);
assert.equal(h5.reports.length,0);
console.log('PASS: native page scope, mount wait, idempotence, marker and Vue app preservation');

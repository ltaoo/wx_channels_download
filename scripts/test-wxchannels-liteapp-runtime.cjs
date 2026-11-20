const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync("pkg/scraper/wxchannels/liteapp_runtime.js", "utf8");

async function run(origin = "https://liteapp.weixin.qq.com", fail = false) {
  const reports = [], requests = [], nodes = [], events = new Map();
  const app = {config: {globalProperties: {$pinia: {_s: new Map([["feed", {feed: {id: "current-video", objectNonceId: "nonce"}}]])}}}};
  const root = {__vue_app__: app};
  class API {
    async finderInit() { return this.post("init"); }
    async finderGetCommentDetail(payload) { return this.post(payload); }
  }
  const api = new API();
  api.token = "keep-existing-instance";
  api.post = async function (payload) { return {data: {object: payload, token: this.token}}; };
  const require = id => id === "1" ? {Z: API} : {mh: api};
  require.m = {
    1: function (module, exports, require) { class API { async finderInit() {} async finderGetCommentDetail(e) {} } exports.Z = API; },
    2: function () { return "async finderPcFlow( getRecommendTabsFromService("; }
  };
  const context = vm.createContext({URL, URLSearchParams, console, setTimeout, Map,
    location: {origin, pathname: "/a/finder/wxalite28e098851253e34f4777f7ca024236fd/pages/lite/feed", search: ""},
    document: {readyState: "complete", querySelector: () => root,
      createElement: tag => ({tag, setAttribute(key, value) { this[key] = value; }}),
      head: {appendChild(node) {
        nodes.push(node);
        if (node.src) {
          vm.runInContext(`window.__wxchannels_liteapp_rewrite__.register("1", function(module,exports,require){
            const Base = require("1").Z;
            class Modified extends Base {
              async finderInit(){const result=await this.post("init");WXU.emit("channels:Init",result.data);return result;}
              async finderGetCommentDetail(e){const result=await this.post(e);WXU.emit("channels:OnFeedProfileLoaded",result.data.object);return result;}
            }
            exports.Z=Modified;
          });`, context);
          node.onload();
        }
      }}},
    DOMParser: class {parseFromString() {return {head: {children: []}};}},
    fetch: async (url, options) => {
      requests.push(url);
      if (url.endsWith("status")) { reports.push(JSON.parse(options.body)); return {}; }
      assert(url.endsWith("compile"));
      assert.equal(JSON.parse(options.body).module_id, "1");
      return {ok: !fail, json: async () => fail ? {error: "no rewrite matched"} :
        {revision: "new-content-hash", url: "https://res.wx.qq.com/__assets/wxchannels/liteapp/modules/new-content-hash.js?v=new-content-hash", bootstrap: "<head></head>"}};
    },
    WXU: {on(name, handler) { events.set(name, handler); }, emit(name, data) { events.get(name)?.(data); }},
    WXE: {Events: {DOMContentLoaded: "DOMContentLoaded"}, emit() {}}
  });
  context.window = context;
  context.webpackChunk = {push(chunk) { chunk[2](require); }};
  vm.runInContext(source + source, context);
  if (origin !== "https://liteapp.weixin.qq.com") {
    assert.equal(requests.length, 0);
    assert.equal(context.__wxchannels_liteapp_rewrite__, undefined);
    return;
  }
  for (let attempt = 0; attempt < 100 && !["ready", "failed"].includes(context.__wxchannels_liteapp_rewrite__.stage); attempt++) {
    await new Promise(resolve => setImmediate(resolve));
  }
  const model = context.__wxchannels_liteapp_rewrite__;
  assert.equal(model.stage, fail ? "failed" : "ready");
  assert.equal(root.__vue_app__, app);
  assert.equal(require("2").mh, api);
  assert.equal(api.token, "keep-existing-instance");
  assert.equal(requests.filter(url => url.endsWith("compile")).length, 1);
  if (fail) { assert.equal(model.methods, 0); assert.equal(nodes.length, 0); return; }
  assert.equal(model.methods, 2);
  assert.equal(model.init_events, 1);
  assert.equal(model.detail_events, 1);
  assert.equal(nodes.length, 1);
  assert.equal(nodes[0]["data-n"], "wxchannels-liteapp-business-module");
  assert(nodes[0].src.includes("?v=new-content-hash"));
  const result = await api.finderGetCommentDetail("new-video");
  assert.equal(result.data.object, "new-video");
  assert.equal(result.data.token, "keep-existing-instance");
  assert.equal(model.detail_events, 2);
  assert(reports.every(report => report.app_preserved));
}

(async () => {
  await run();
  await run("https://channels.weixin.qq.com");
  await run("https://liteapp.weixin.qq.com", true);
  console.log("LiteApp runtime: versioned JS, real method execution, existing instance/Vue, scope, once, and failure checks passed");
})().catch(error => {console.error(error);process.exitCode = 1;});

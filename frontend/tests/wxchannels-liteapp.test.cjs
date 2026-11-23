const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const root = path.resolve(__dirname, "../..");
const runtime = fs.readFileSync(path.join(root, "pkg/scraper/wxchannels/liteapp_runtime.js"), "utf8");
const eventbus = fs.readFileSync(path.join(root, "frontend/inject/eventbus.js"), "utf8");
const channelEvents = fs.readFileSync(path.join(root, "pkg/scraper/wxchannels/inject/channels.events.js"), "utf8");

const eventCases = [
  ["finderInit", "channels:Init", { data: { user: "test" } }, (r) => r.data],
  ["finderGetCommentDetail", "channels:OnFeedProfileLoaded", { data: { object: { id: "detail" } } }, (r) => r.data.object],
  ["finderGetCommentList", "channels:FeedCommentListLoaded", { data: { comments: [] } }, (r) => r.data],
  ["finderPcFlow", "channels:PCFlowLoaded", { data: { object: [1] } }, (r) => r.data.object],
  ["finderGetRecommend", "channels:RecommendFeedsLoaded", { data: { object: [2] } }, (r) => r.data.object],
  ["getRecommendTabsFromService", "channels:RecommendFeedsLoaded", { resp: { data: { object: [3] } } }, (r) => r.resp.data.object],
  ["finderGetInteractionedFeedList", "channels:InteractionedFeedsLoaded", { data: { object: [4] } }, (r) => r.data.object],
  ["finderGetFeedH5Url", "channels:GetFeedH5Url", { data: { object: { url: "test" } } }, (r) => r.data.object],
  ["finderUserPage", "channels:UserFeedsLoaded", { data: { object: [5] } }, (r) => r.data.object],
  ["finderLiveUserPage", "channels:LiveUserFeedsLoaded", { data: { object: [6] } }, (r) => r.data.object],
  ["finderGetLiveInfo", "channels:OnLiveProfileLoaded", { data: { live: 1 } }, (r) => r.data],
  ["joinLive", "channels:JoinLive", { data: { joined: true } }, (r) => r.data],
];

async function boot(options = {}) {
  const events = [], requests = [], calls = [], consumed = [], warnings = [];
  const fixture = { calls, results: Object.fromEntries(eventCases.map(([name, , result]) => [name, result])), fail: null, pending: null, executions: 0 };
  const context = vm.createContext({
    fixture, URL, URLSearchParams, setTimeout, alert() {},
    console: { warn: (...args) => warnings.push(args), log() {} },
    location: new URL("https://liteapp.weixin.qq.com/a/finder/wxalite28e098851253e34f4777f7ca024236fd/pages/feed"),
    addEventListener() {},
    Timeless: { mitt() {
      const listeners = new Map();
      return {
        on(name, handler) { if (!listeners.has(name)) listeners.set(name, []); listeners.get(name).push(handler); },
        off(name, handler) { const list = listeners.get(name) || []; const index = list.indexOf(handler); if (index >= 0) list.splice(index, 1); },
        emit(name, value) { events.push([name, value]); for (const handler of [...(listeners.get(name) || [])]) handler(value); },
      };
    } },
  }, { codeGeneration: { strings: false, wasm: false } });
  context.window = context;
  vm.runInContext(`
    const factories = {
      1(module, exports, require) {
        const api = new (require(2).API)();
        class FlowService {
          async finderPcFlow(...args) { return api.finderPcFlow(...args); }
          getRecommendTabsFromService(...args) { return api.getRecommendTabsFromService(...args); }
        }
        exports.api = api;
        exports.flow = new FlowService();
      },
      2(module, exports) {
        fixture.executions++;
        const token = Symbol("module closure");
        function invoke(receiver, name, args, secret) {
          if (secret !== token) throw new Error("lost module closure or receiver");
          fixture.calls.push({ receiver, name, args });
          if (fixture.fail) throw fixture.fail;
          return fixture.pending || fixture.results[name];
        }
        class API {
          #token = token;
          async finderInit(...args) { return invoke(this, "finderInit", args, this.#token); }
          async finderGetCommentDetail(...args) { return invoke(this, "finderGetCommentDetail", args, this.#token); }
          async finderGetCommentList(...args) { return invoke(this, "finderGetCommentList", args, this.#token); }
          async finderPcFlow(...args) { return invoke(this, "finderPcFlow", args, this.#token); }
          async finderGetRecommend(...args) { return invoke(this, "finderGetRecommend", args, this.#token); }
          async getRecommendTabsFromService(...args) { return invoke(this, "getRecommendTabsFromService", args, this.#token); }
          async finderGetInteractionedFeedList(...args) { return invoke(this, "finderGetInteractionedFeedList", args, this.#token); }
          async finderGetFeedH5Url(...args) { return invoke(this, "finderGetFeedH5Url", args, this.#token); }
          async finderUserPage(...args) { return invoke(this, "finderUserPage", args, this.#token); }
          async finderLiveUserPage(...args) { return invoke(this, "finderLiveUserPage", args, this.#token); }
          async finderGetLiveInfo(...args) { return invoke(this, "finderGetLiveInfo", args, this.#token); }
          async joinLive(...args) { return invoke(this, "joinLive", args, this.#token); }
        }
        exports.API = API;
        exports.alias = API;
      },
    };
    const cache = {};
    function require(id) {
      if (!cache[id]) {
        cache[id] = { exports: {} };
        factories[id](cache[id], cache[id].exports, require);
      }
      return cache[id].exports;
    }
    require.m = factories;
    window.webpackChunk = { push(chunk) { chunk[2](require); } };
    window.services = require(1);
    window.apiExports = require(2);
  `, context);
  const api = context.services.api;
  const proto = context.apiExports.API.prototype;
  const originalInit = proto.finderInit;
  if (options.locked) Object.defineProperty(proto, "finderGetCommentDetail", { configurable: false });
  if (options.missingInit) delete proto.finderInit;
  const app = { config: { globalProperties: {
    $route: { query: options.query || {} },
    $pinia: { _s: new Map([["feed", { feed: { id: "feed-id", objectNonceId: "nonce" } }]]) },
  } } };
  const appRoot = { __vue_app__: app };
  context.document = {
    readyState: "complete", addEventListener() {},
    querySelector(selector) { return selector === "#app" ? appRoot : null; },
    createElement(tag) { return { tagName: tag.toUpperCase(), setAttribute() {} }; },
    head: { appendChild(node) {
      assert.equal(node.src, "https://res.wx.qq.com/__assets/test-bootstrap.js");
      vm.runInContext(eventbus, context);
      context.WXU = { emit: context.WXE.emit, on: context.WXE.on };
      vm.runInContext(channelEvents, context);
      context.WXU.onInit((data) => consumed.push(["init", data]));
      context.WXU.onFetchFeedProfile((data) => consumed.push(["detail", data]));
      context.WXU.onAPILoaded((exports) => { context.exposed = exports; });
      node.onload();
    } },
  };
  context.DOMParser = class {
    parseFromString(html) {
      assert.equal(html, "bootstrap-html");
      return { head: { children: [{ tagName: "SCRIPT", hasAttribute: () => true, getAttribute: () => "/__assets/test-bootstrap.js" }] } };
    }
  };
  context.fetchFeedProfileWith = async (data) => {
    context.profileRequest = data;
    await api.finderGetCommentDetail(data);
    return [null];
  };
  context.fetch = async (url, init) => {
    requests.push({ url, init });
    if (url.endsWith("/status")) return { ok: true };
    assert.equal(url, "https://res.wx.qq.com/__assets/wxchannels/liteapp/bootstrap");
    assert.equal(init, undefined);
    return { ok: !options.bootstrapError, json: async () => options.bootstrapError ? { error: "bootstrap unavailable" } : { bootstrap: "bootstrap-html" } };
  };
  vm.runInContext(runtime, context);
  const model = context.__wxchannels_liteapp_rewrite__;
  for (let i = 0; i < 100 && !["ready", "failed"].includes(model.stage); i++) {
    await new Promise(setImmediate);
  }
  assert.ok(["ready", "failed"].includes(model.stage), "startup did not finish");
  return { context, model, fixture, events, requests, calls, consumed, warnings, app, appRoot, api, proto, originalInit };
}

test("LiteApp starts without compiling or evaluating source and preserves the existing app/service", async () => {
  const h = await boot();
  assert.equal(h.model.stage, "ready", h.model.error);
  assert.equal(h.model.methods, 12);
  assert.equal(h.fixture.executions, 1);
  assert.equal(h.context.exposed, h.context.services);
  assert.equal(h.context.exposed.api, h.api);
  assert.equal(h.appRoot.__vue_app__, h.app);
  assert.deepEqual(h.calls.map((c) => c.name), ["finderInit", "finderGetCommentDetail"]);
  assert.equal(h.calls[1].args[0].objectid, "feed-id");
  assert.deepEqual(h.consumed.map(([name]) => name), ["init", "detail"]);
  const status = h.requests.filter((r) => r.url.endsWith("/status")).map((r) => JSON.parse(r.init.body));
  assert.equal(status.at(-1).stage, "ready");
  assert.equal(status.at(-1).app_preserved, true);
  assert.equal(status.at(-1).init_events, 1);
  assert.equal(status.at(-1).detail_events, 1);
  assert.equal(h.warnings.length, 0);
  vm.runInContext(runtime, h.context);
  assert.equal(h.fixture.executions, 1);
  assert.equal(h.model.init_events, 1);
});

test("all existing API events retain their payloads, receiver, arguments and original response", async (t) => {
  const h = await boot();
  for (const [name, event, response, extract] of eventCases) {
    await t.test(name, async () => {
      h.events.length = 0;
      const payload = { marker: name }, extra = Symbol(name);
      const result = await h.api[name](payload, extra);
      assert.equal(result, response);
      assert.equal(h.calls.at(-1).receiver, h.api);
      assert.equal(h.calls.at(-1).args[0], payload);
      assert.equal(h.calls.at(-1).args[1], extra);
      assert.equal(h.events.length, 1);
      assert.equal(h.events[0][0], event);
      assert.equal(h.events[0][1], extract(response));
    });
  }
  h.fixture.results.getRecommendTabsFromService = { data: { object: ["direct"] } };
  h.events.length = 0;
  await h.api.getRecommendTabsFromService();
  assert.equal(h.events[0][1], h.fixture.results.getRecommendTabsFromService.data.object);
  h.fixture.results.getRecommendTabsFromService = {};
  h.events.length = 0;
  await h.api.getRecommendTabsFromService();
  assert.equal(h.events[0][1].length, 0);
});

test("completion events wait for the request, and rejected calls retain the error without emitting success", async () => {
  const h = await boot();
  h.events.length = 0;
  let resolve;
  h.fixture.pending = new Promise((done) => { resolve = done; });
  const pending = h.api.finderGetCommentDetail({ id: "pending" });
  await new Promise(setImmediate);
  assert.equal(h.events.length, 0);
  const response = { data: { object: { id: "finished" } } };
  resolve(response);
  assert.equal(await pending, response);
  assert.equal(h.events[0][1], response.data.object);
  h.fixture.pending = null;
  h.fixture.fail = new Error("upstream error");
  for (const name of ["finderInit", "finderGetCommentDetail"]) {
    h.events.length = 0;
    await assert.rejects(h.api[name](), (error) => error === h.fixture.fail);
    assert.equal(h.events.length, 0);
  }
});

test("installing twice or exporting the class twice does not duplicate notifications", async () => {
  const h = await boot();
  const wrapped = h.proto.finderGetCommentDetail;
  const descriptor = Object.getOwnPropertyDescriptor(h.proto, "finderGetCommentDetail");
  assert.equal(descriptor.enumerable, false);
  assert.equal(descriptor.writable, true);
  assert.equal(descriptor.configurable, true);
  h.model.install(h.context.apiExports);
  assert.equal(h.proto.finderGetCommentDetail, wrapped);
  assert.equal(h.model.methods, 12);
  h.events.length = 0;
  await h.api.finderGetCommentDetail();
  assert.equal(h.events.length, 1);
});

test("unsupported APIs and bootstrap failures stop before changing the original methods", async (t) => {
  for (const options of [{ locked: true }, { missingInit: true }, { bootstrapError: true }]) {
    await t.test(JSON.stringify(options), async () => {
      const h = await boot(options);
      assert.equal(h.model.stage, "failed");
      assert.equal(h.model.methods, 0);
      if (!options.missingInit) assert.equal(h.proto.finderInit, h.originalInit);
      assert.equal(h.calls.length, 0);
    });
  }
});

test("detail replay supports both eid and oid/nid routes", async (t) => {
  for (const query of [{ eid: "export-id" }, { oid: "object-id", nid: "object-nonce" }]) {
    await t.test(JSON.stringify(query), async () => {
      const h = await boot({ query });
      assert.equal(h.model.stage, "ready", h.model.error);
      if (query.eid) assert.equal(h.context.profileRequest.eid, query.eid);
      else {
        const url = new URL(h.context.profileRequest.url);
        assert.equal(url.searchParams.get("oid"), query.oid);
        assert.equal(url.searchParams.get("nid"), query.nid);
      }
      assert.equal(h.model.init_events, 1);
      assert.equal(h.model.detail_events, 1);
    });
  }
});

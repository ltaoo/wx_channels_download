const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function load_model(t, client, query = {}) {
  const root = path.join(__dirname, "..");
  const context = vm.createContext({
    console, URL, clearTimeout, clearInterval,
    setTimeout(fn, ms) { const timer = setTimeout(fn, ms); timer.unref(); return timer; },
    setInterval(fn, ms) { const timer = setInterval(fn, ms); timer.unref(); return timer; },
    window: {},
  });
  vm.runInContext(fs.readFileSync(path.join(root, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  Object.assign(context, context.Timeless);
  vm.runInContext(fs.readFileSync(path.join(root, "src/biz/request.js"), "utf8").replace(/^export /gm, ""), context);
  vm.runInContext(fs.readFileSync(path.join(root, "src/pages/platforms.model.js"), "utf8").replace(/^import .*;\n/gm, "").replace(/^export /gm, ""), context);
  const copied = [], tips = [];
  const view = { pathname: "/platforms", query };
  const router = { href: "/platforms", replaceState(href) { this.href = href; } };
  const model = context.PlatformsPageModel({ client, view, history: { $router: router }, app: { copy: async (text) => copied.push(JSON.parse(text)) }, tip: (value) => tips.push(value) });
  t.after(() => model.methods.destroy());
  return { model, copied, tips, router, Result: context.Timeless.Result };
}

const douyin = { id: "douyin", name: "抖音", source: "douyin", detail: true, list: true, tabs: [{ name: "作品", scope: "posts" }] };
const cctv = { id: "cctv", name: "央视网", source: "cctv", detail: true, list: false, tabs: [] };

test("platform selection updates the query and is restored after reloading", async (t) => {
  const client = {};
  const { model, router, Result } = load_model(t, client, { keep: "a & b" });
  client.get = async () => Result.Ok({ list: [douyin, cctv] });
  await model.methods.load();
  assert.equal(model.state.selected.value, null);
  await model.state.visible.value[0].button$.click();
  let url = new URL(router.href, "http://localhost");
  assert.equal(url.pathname, "/platforms");
  assert.equal(url.searchParams.get("platform"), "douyin");
  assert.equal(url.searchParams.get("keep"), "a & b");
  await model.state.visible.value[1].button$.click();
  url = new URL(router.href, "http://localhost");
  assert.equal(url.searchParams.get("platform"), "cctv");
  const restored = load_model(t, client, Object.fromEntries(url.searchParams)).model;
  await restored.methods.load();
  assert.equal(restored.state.selected.value.id, "cctv");
  assert.equal(restored.state.supports_list.value, false);
  model.ui.url$.setValue("keep my draft");
  await model.methods.load();
  assert.equal(model.state.selected.value.id, "cctv");
  assert.equal(model.ui.url$.value, "keep my draft");
});

test("unknown platform queries leave the catalog usable", async (t) => {
  const client = {};
  const { model, Result } = load_model(t, client, { platform: "unknown" });
  client.get = async () => Result.Ok({ list: [douyin] });
  await model.methods.load();
  assert.equal(model.state.selected.value, null);
  assert.equal(model.state.catalog_error.value, "");
  await model.state.visible.value[0].button$.click();
  assert.equal(model.state.selected.value.id, "douyin");
});

test("platform catalog is searchable and unsupported lists cannot be requested", async (t) => {
  const client = {};
  const { model, Result } = load_model(t, client);
  client.get = async () => Result.Ok({ list: [douyin, cctv] });
  await model.methods.load();
  assert.equal(model.state.visible.value.length, 2);
  model.ui.keyword$.setValue("DOUYIN");
  assert.equal(model.state.visible.value[0].id, "douyin");
  model.methods.open(cctv);
  await model.methods.run("list");
  assert.equal(model.state.detail_result.has_result.value, false);
  await model.methods.run("detail");
  assert.match(model.state.detail_result.error.value, /请填写内容链接/);
});

test("details invoke the selected platform and copy its result", async (t) => {
  const calls = [], client = {};
  const { model, copied, Result } = load_model(t, client);
  client.post = async (url, body) => { calls.push({ url, body }); return Result.Ok({ aweme_detail: { aweme_id: "123" } }); };
  model.methods.open(douyin);
  model.ui.url$.setValue("https://www.douyin.com/video/123");
  await model.methods.run("detail");
  assert.equal(calls[0].url, "/api/scraper/platform/fetch");
  assert.equal(calls[0].body.platform, "douyin");
  assert.equal(calls[0].body.operation, "detail");
  assert.equal(model.state.busy.value, false);
  assert.match(model.state.detail_result.result_text.value, /123/);
  await model.ui.copy_detail$.click();
  assert.deepEqual(copied, [{ aweme_detail: { aweme_id: "123" } }]);
});

test("list calls keep scope and opaque cursor unchanged", async (t) => {
  const calls = [], client = {};
  const { model, Result } = load_model(t, client);
  client.post = async (url, body) => { calls.push({ ...body }); return Result.Ok({ list: [], next_marker: calls.length === 1 ? "18446744073709551615" : "" }); };
  model.methods.open(douyin);
  model.ui.profile$.setValue("owner");
  await model.methods.run("list");
  await model.methods.run("list", true);
  assert.equal(calls.length, 2);
  assert.equal(calls[0].scope, "posts");
  assert.equal(calls[0].page, "");
  assert.equal(calls[1].page, "18446744073709551615");
  assert.equal(model.state.next_marker.value, "");
});

test("list and detail keep separate results, copies, errors and pagination", async (t) => {
  const calls = [], client = {};
  const { model, copied, Result } = load_model(t, client);
  client.post = async (url, body) => {
    calls.push({ ...body });
    return Result.Ok(body.operation === "list" ? { list: ["item"], next_marker: "list-cursor" } : { detail: "item", next_marker: "detail-cursor" });
  };
  model.methods.open(douyin);
  model.ui.url$.setValue("https://www.douyin.com/video/123");
  model.ui.profile$.setValue("owner");
  await model.methods.run("list");
  const list_text = model.state.list_result.result_text.value;
  const list_elapsed = model.state.list_result.elapsed.value;
  await model.methods.run("detail");
  assert.equal(model.state.list_result.result_text.value, list_text);
  assert.equal(model.state.list_result.elapsed.value, list_elapsed);
  assert.match(model.state.detail_result.result_text.value, /detail-cursor/);
  assert.equal(model.state.next_marker.value, "list-cursor");
  await model.ui.copy_list$.click();
  await model.ui.copy_detail$.click();
  assert.deepEqual(copied, [{ list: ["item"], next_marker: "list-cursor" }, { detail: "item", next_marker: "detail-cursor" }]);
  await model.methods.run("list", true);
  assert.equal(calls[2].page, "list-cursor");
  assert.match(model.state.detail_result.result_text.value, /detail-cursor/);
  client.post = async () => Result.Err("详情失败");
  await model.methods.run("detail");
  assert.equal(model.state.detail_result.error.value, "详情失败");
  assert.equal(model.state.list_result.error.value, "");
  assert.equal(model.state.list_result.result_text.value, list_text);
  assert.equal(model.state.detail_result.has_result.value, false);
  model.methods.open(cctv);
  for (const state of [model.state.list_result, model.state.detail_result]) {
    assert.equal(state.has_result.value, false);
    assert.equal(state.error.value, "");
    assert.equal(state.elapsed.value, "");
  }
});

test("switching platforms discards late results from each previous selection", async (t) => {
  const pending = [], client = {};
  const { model, Result } = load_model(t, client);
  client.post = () => new Promise((resolve) => pending.push(resolve));
  model.methods.open(douyin);
  model.ui.url$.setValue("https://www.douyin.com/video/123");
  const first = model.methods.run("detail");
  model.methods.open(cctv);
  pending[0](Result.Ok({ old: true }));
  await first;
  assert.equal(model.state.selected.value.id, "cctv");
  assert.equal(model.state.detail_result.has_result.value, false);
  model.ui.url$.setValue("https://v.cctv.com/123");
  const second = model.methods.run("detail");
  model.methods.open(douyin);
  pending[1](Result.Ok({ previous_selection: true }));
  await second;
  assert.equal(model.state.detail_result.has_result.value, false);
  assert.equal(model.state.busy.value, false);
});

test("platform errors are visible and do not leave an old result", async (t) => {
  const client = {};
  const { model, Result } = load_model(t, client);
  model.methods.open(douyin);
  model.ui.url$.setValue("https://www.douyin.com/video/123");
  client.post = async () => Result.Ok({ success: true });
  await model.methods.run("detail");
  client.post = async () => Result.Err("签名执行失败");
  await model.methods.run("detail");
  assert.equal(model.state.detail_result.has_result.value, false);
  assert.equal(model.state.detail_result.error.value, "签名执行失败");
});

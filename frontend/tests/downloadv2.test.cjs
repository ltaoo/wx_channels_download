const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function load_model(t, client = {}) {
  const root = path.join(__dirname, "..");
  const context = vm.createContext({
    console, URL, clearTimeout, clearInterval,
    setTimeout(fn, ms) { const timer = setTimeout(fn, ms); timer.unref(); return timer; },
    setInterval(fn, ms) { const timer = setInterval(fn, ms); timer.unref(); return timer; },
    window: { config: {} },
    track_error: (source) => source,
  });
  vm.runInContext(fs.readFileSync(path.join(root, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  Object.assign(context, context.Timeless);
  const requests = fs.readFileSync(path.join(root, "src/biz/request.js"), "utf8");
  vm.runInContext(requests.split("/**")[0].replace("export const", "const") + `
    function fetchDownloadTaskDetail(params) { return request.get("/api/v1/download_task/detail", params); }
    function updateDownloadTask(body) { return request.post("/api/v1/download_task/update", body); }
  `, context);
  for (const file of ["src/list.model.js", "src/pages/downloadv2.model.js"]) {
    vm.runInContext(fs.readFileSync(path.join(root, file), "utf8")
      .replace(/^import .*;\n/gm, "").replace(/^export \{[\s\S]*?\};/gm, "").replace(/^export /gm, ""), context);
  }
  const tips = [], copied = [];
  const app = { copy: async (url) => copied.push(url) };
  const model = context.DownloadV2Model({
    client,
    downloader: { refresh: async () => [], task_list: context.refarr([]) },
    tip: (message) => tips.push(message),
    app,
  });
  t.after(() => model.methods.clean());
  return { model, tips, copied, app, Result: context.Timeless.Result };
}

test("details expose each original download URL, copy it, and handle missing links", async (t) => {
  const client = {};
  const { model, copied, tips, app, Result } = load_model(t, client);
  client.get = async () => Result.Ok({ id: 1, name: "task", source_url: "source", files: [
    { id: 2, name: "video", url: "https://example.com/video?token=one&x=2", file_url: "/api/file" },
    { id: 3, name: "audio", url: "https://example.com/audio" },
  ] });
  await model.methods.requestTaskDetail(1);
  assert.equal(model.state.task_detail_loading.value, false);
  assert.equal(model.state.task_detail_links.value.length, 2);
  const url = model.state.task_detail_links.value[0].url;
  await model.methods.copyTaskDownloadURL(url);
  assert.deepEqual(copied, ["https://example.com/video?token=one&x=2"]);
  assert.equal(tips.at(-1).type, "success");
  app.copy = async () => { throw new Error("clipboard unavailable"); };
  await model.methods.copyTaskDownloadURL(url);
  assert.equal(tips.at(-1).type, "error");
  client.get = async () => Result.Ok({ id: 1, files: [], url: "https://example.com/fallback" });
  await model.methods.requestTaskDetail(1);
  assert.equal(model.state.task_detail_links.value[0].url, "https://example.com/fallback");
  client.get = async () => Result.Ok({ id: 1, files: [], source_url: "source" });
  await model.methods.requestTaskDetail(1);
  assert.equal(model.state.task_detail_links.value.length, 0);
});

test("late detail responses cannot replace the current task, and load errors are visible", async (t) => {
  const pending = [];
  const { model, Result } = load_model(t, { get: () => new Promise((resolve) => pending.push(resolve)) });
  const first = model.methods.requestTaskDetail(1);
  const second = model.methods.requestTaskDetail(2);
  pending[1](Result.Ok({ id: 2, files: [] }));
  await second;
  pending[0](Result.Ok({ id: 1, files: [] }));
  await first;
  assert.equal(model.state.task_detail.value.id, 2);
  const failed = model.methods.requestTaskDetail(3);
  pending[2](Result.Err("offline"));
  await failed;
  assert.equal(model.state.task_detail.value, null);
  assert.equal(model.state.task_detail_error.value, "offline");
  assert.equal(model.state.task_detail_loading.value, false);
});

test("only failed tasks can be edited and invalid URLs are not submitted", async (t) => {
  const calls = [], client = {};
  const { model, Result, tips } = load_model(t, client);
  client.get = async () => Result.Ok({ id: 1, name: "task", status: 2, files: [] });
  await model.methods.requestTaskDetail(1, true);
  assert.equal(model.state.task_detail_editing.value, false);
  assert.match(model.state.task_detail_error.value, /不允许编辑/);
  client.get = async () => Result.Ok({ id: 1, name: "task", status: 6, files: [{ id: 8, name: "video", url: "https://example.com/old" }] });
  client.post = async (url, body) => { calls.push({ url, body }); return Result.Ok({}); };
  await model.methods.requestTaskDetail(1, true);
  assert.equal(model.state.task_detail_editing.value, true);
  model.ui.input_task_detail_name$.setValue("new name");
  model.state.task_detail_edit_files.value[0].input$.setValue("javascript:alert(1)");
  await model.methods.saveTaskDetail();
  assert.equal(calls.length, 0);
  assert.equal(tips.at(-1).type, "error");
  model.state.task_detail_edit_files.value[0].input$.setValue("https://example.com/new?token=ok");
  await model.methods.saveTaskDetail();
  assert.equal(calls[0].url, "/api/v1/download_task/update");
  assert.deepEqual(JSON.parse(JSON.stringify(calls[0].body)), {
    id: 1, name: "new name", files: [{ id: 8, url: "https://example.com/new?token=ok" }],
  });
  assert.equal(tips.at(-1).type, "success");
});

test("a failed save preserves the draft for correction", async (t) => {
  const client = {};
  const { model, Result, tips } = load_model(t, client);
  client.get = async () => Result.Ok({ id: 1, name: "task", status: 6, files: [] });
  client.post = async () => Result.Err("save failed");
  await model.methods.requestTaskDetail(1, true);
  model.ui.input_task_detail_name$.setValue("draft");
  await model.methods.saveTaskDetail();
  assert.equal(model.ui.input_task_detail_name$.value, "draft");
  assert.equal(model.state.task_detail_editing.value, true);
  assert.equal(tips.at(-1).type, "error");
});

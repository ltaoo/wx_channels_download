const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function load_model(t, client = {}, use_sdk = false, runtime_config = null) {
  const root = path.join(__dirname, "..");
  const context = vm.createContext({
    console, URL, clearTimeout, clearInterval, queueMicrotask,
    setTimeout(fn, ms) { const timer = setTimeout(fn, ms); timer.unref(); return timer; },
    setInterval(fn, ms) { const timer = setInterval(fn, ms); timer.unref(); return timer; },
    window: { config: {}, location: new URL("http://localhost") },
    track_error: (source) => source,
  });
  if (runtime_config) {
    const html = fs.readFileSync(path.join(root, "index.html"), "utf8");
    const config_script = html.match(/<script>\s*(window\.__d_config = [\s\S]*?)<\/script>/)[1];
    vm.runInContext(config_script.replace("__WX_DOWNLOAD_CONFIG_JSON__", JSON.stringify(runtime_config)), context);
  }
  vm.runInContext(fs.readFileSync(path.join(root, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  Object.assign(context, context.Timeless);
  let downloader = { refresh: async () => [], get: () => null, task_list: context.refarr([]) };
  if (use_sdk) {
    context.window.Timeless = context.Timeless;
    context.window.queueMicrotask = queueMicrotask;
    context.window.setTimeout = context.setTimeout;
    context.window.clearTimeout = clearTimeout;
    vm.runInContext(fs.readFileSync(path.join(root, "public/dl.sdk.js"), "utf8"), context);
    const logger = { Info() { return this; }, Debug() { return this; }, Error() { return this; }, Str() { return this; }, Int() { return this; }, Bool() { return this; }, JSON() { return this; }, Msg() {} };
    downloader = context.window.DownloaderModel({ client, auto_start: false, logger });
    t.after(() => downloader.destroy());
  }
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
    downloader,
    tip: (message) => tips.push(message),
    app,
  });
  t.after(() => model.methods.clean());
  return { model, downloader, tips, copied, app, Result: context.Timeless.Result };
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

async function load_download_page(t) {
  const client = {}, list_calls = [];
  const loaded = load_model(t, client, true);
  const { model, downloader, Result } = loaded;
  const records = [1, 2, 3].map((id) => ({ id, name: `task ${id}`, status: 6 }));
  client.get = async (url, params) => {
    const parsed = new URL(url, "http://localhost");
    params = Object.assign(Object.fromEntries(parsed.searchParams), params);
    url = parsed.pathname;
    if (url.endsWith("/detail")) return Result.Ok({
      ...records.find((task) => task.id === Number(params.id)),
      files: [{ id: 8, name: "video", url: "https://example.com/old", size: 1024 }],
    });
    assert.ok(url.endsWith("/list"));
    list_calls.push(params);
    return Result.Ok({ list: records, total: 120, page: params.page, page_size: 50, stats: { 6: 120 } });
  };
  client.post = async (url, body) => {
    return Result.Ok({ results: [{ task_id: body.task_ids?.[0], success: true }] });
  };
  await model.methods.ready();
  assert.equal(model.state.error.value, "");
  await model.methods.changePage(2);
  const scroll = { scrollTop: 240 };
  model.methods.setListViewElement(scroll);
  model.methods.handleListViewScroll({ scrollTop: 240 });
  model.methods.toggleTaskSelected(downloader.get(3));
  const completed = model.state.list.state.completed.value;
  function assert_stable() {
    assert.equal(list_calls.length, 2);
    assert.equal(model.state.page.value, 2);
    assert.equal(scroll.scrollTop, 240);
    assert.equal(model.state.list.state.completed.value, completed);
    assert.equal(model.state.loading.value, false);
  }
  return { ...loaded, client, list_calls, assert_stable };
}

test("the management page keeps its task socket on the API origin with remote downloads enabled", (t) => {
  const { downloader } = load_model(t, {}, true, { remoteServerEnabled: true });
  assert.equal(downloader.websocket_url, "ws://localhost/ws/v1/download_task");
});

test("task actions and socket status updates preserve the page, rows, selection and scroll", async (t) => {
  const { model, downloader, client, Result, assert_stable, tips } = await load_download_page(t);
  const first = downloader.get(1), other = downloader.get(3);
  for (const [method, status] of [["startTask", "preparing"], ["pauseTask", "paused"], ["resumeTask", "preparing"], ["retryTask", "preparing"]]) {
    await model.methods[method](first);
    assert.equal(first.state.status.value, status);
    assert_stable();
  }
  const notify = downloader.handler.handle_web_socket_message;
  notify({ type: "task_update", updates: [{ id: 1, status: 2 }, { id: 99, status: 3 }] });
  notify({ type: "task_upsert", tasks: [{ id: 1, status: 3 }, { id: 99, status: 6 }] });
  notify({ type: "task_snapshot", task_id: 1, status: 2, resources: [] });
  notify({ type: "batch_tasks", data: [{ id: 1, status: 3 }] });
  notify({ type: "event", data: { task: { id: 1, status: 2 } } });
  await new Promise((resolve) => setTimeout(resolve, 120));
  assert_stable();
  assert.equal(model.state.tasks.value[0], first);
  assert.equal(downloader.get(3), other);
  assert.equal(downloader.get(99), null);
  assert.deepEqual(Array.from(model.state.selected_task_ids.value), [3]);
  notify({ type: "task_create", tasks: [{ id: 4, name: "new task", status: 0 }] });
  await new Promise((resolve) => setTimeout(resolve, 120));
  assert.equal(model.state.tasks.value[0], downloader.get(4));
  assert.equal(model.state.tasks.value[1], first);
  assert_stable();
  client.post = async () => Result.Err("retry failed");
  await model.methods.retryTask(first);
  assert.equal(first.state.status.value, "downloading");
  assert.equal(tips.at(-1).type, "error");
  assert_stable();
});

test("editing updates the existing task and file metadata without reloading", async (t) => {
  const { model, downloader, assert_stable } = await load_download_page(t);
  const first = downloader.get(1), other = downloader.get(2);
  await model.methods.requestTaskDetail(first, true);
  model.ui.input_task_detail_name$.setValue("edited task");
  model.state.task_detail_edit_files.value[0].input$.setValue("https://example.com/new");
  await model.methods.saveTaskDetail();
  assert.equal(first.state.name.value, "edited task");
  assert.equal(first.state.raw.value.files[0].url, "https://example.com/new");
  assert.equal(first.state.raw.value.files[0].size, 1024);
  assert.equal(model.state.tasks.value[0], first);
  assert.equal(model.state.tasks.value[1], other);
  assert_stable();
});

test("single and partially failed batch deletions only remove successful records", async (t) => {
  const { model, downloader, client, Result, tips, assert_stable } = await load_download_page(t);
  model.methods.requestDeleteTask(downloader.get(1));
  await model.methods.confirmDeleteTask();
  assert.equal(downloader.get(1), null);
  assert_stable();
  client.post = async (_url, body) => Result.Ok({ results: [{ task_id: body.task_ids[0], success: body.task_ids[0] === 2, error: "delete failed" }] });
  model.methods.toggleTaskSelected(downloader.get(2));
  model.methods.toggleTaskSelected(downloader.get(3));
  model.methods.requestDeleteSelectedTasks();
  await model.methods.confirmDeleteTask();
  assert.equal(downloader.get(2), null);
  assert.equal(model.state.tasks.value.length, 1);
  assert.equal(model.state.tasks.value[0], downloader.get(3));
  assert.deepEqual(Array.from(model.state.selected_task_ids.value), [3]);
  assert.equal(tips.at(-1).type, "error");
  downloader.handler.handle_web_socket_message({ type: "task_delete", task_ids: [3] });
  downloader.handler.handle_web_socket_message({ type: "event", data: { key: "delete", task_id: 99 } });
  await new Promise((resolve) => setTimeout(resolve, 120));
  assert.equal(model.state.tasks.value.length, 0);
  assert_stable();
});

test("bulk actions use socket updates and clearing preserves failures without reloading", async (t) => {
  const { model, downloader, client, Result, tips, assert_stable } = await load_download_page(t);
  client.post = async (url) => {
    if (url.endsWith("/clear_all")) return Result.Ok({ results: [
      { task_id: 1, success: true }, { task_id: 2, success: false, error: "file is busy" }, { task_id: 3, success: true },
    ] });
    downloader.handler.handle_web_socket_message({ type: "task_update", updates: [{ id: 2, status: url.endsWith("/pause_all") ? 3 : 2 }] });
    return Result.Ok({ total: 1 });
  };
  await model.methods.startAllTasks();
  assert.equal(downloader.get(2).state.status.value, "downloading");
  await model.methods.pauseAllTasks();
  assert.equal(downloader.get(2).state.status.value, "paused");
  model.methods.requestClearTasks();
  await model.methods.confirmClearTasks();
  assert.deepEqual(Array.from(model.state.tasks.value, (task) => task.id), [2]);
  assert.equal(tips.at(-1).type, "error");
  assert_stable();
});

test("socket statistics update totals while manual navigation and refresh still load the list", async (t) => {
  const { model, downloader, list_calls, assert_stable } = await load_download_page(t);
  downloader.handler.handle_web_socket_message({ type: "task_stats", stats: { 3: 1, 6: 118 } });
  assert.equal(model.state.total.value, 119);
  assert.equal(model.state.task_count.value, 119);
  assert_stable();
  await model.methods.setStatusFilter("error");
  assert.equal(list_calls.at(-1).status, "6,7");
  downloader.handler.handle_web_socket_message({ type: "task_stats", stats: { 3: 120 } });
  assert.equal(model.state.total.value, 0);
  assert.equal(model.state.status_counts.value.pause, 120);
  await model.methods.refreshTasks();
  assert.equal(list_calls.length, 4);
  assert.equal(model.state.page.value, 1);
});

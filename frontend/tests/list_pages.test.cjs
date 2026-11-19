const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup(model_file, respond) {
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval() {}, clearInterval() {},
    URL, URLSearchParams, AbortController, requestAnimationFrame: () => 0, cancelAnimationFrame() {},
    fetch: async () => ({ ok: true, json: async () => ({ data: { version: "test" } }) }),
  });
  context.window = context;
  context.config = {};
  const read = (file) => fs.readFileSync(path.join(__dirname, "..", file), "utf8");
  vm.runInContext(read("public/timeless/0.33.1/timeless.umd.min.js"), context);
  Object.assign(context, context.Timeless);
  for (const file of ["src/utils.js", "src/list.model.js", "src/biz/request.js", "src/image-proxy.model.js", model_file]) {
    vm.runInContext(read(file).replace(/^import[\s\S]*?;\n/gm, "")
      .replace(/^export \{[^}]+\};\s*$/gm, "").replace(/^export /gm, ""), context);
  }
  const calls = [];
  async function request(url, body) {
    const parsed = new URL(url, "http://test");
    const params = { ...Object.fromEntries(parsed.searchParams), ...body };
    calls.push({ url: parsed.pathname, params });
    try { return context.Timeless.Result.Ok(await respond(parsed.pathname, params)); }
    catch (error) { return context.Timeless.Result.Err(error); }
  }
  return { context, calls, client: { get: request, post: request }, props: { app: { copy() {}, setTitle() {} }, tip() {} } };
}

const flush = () => new Promise(setImmediate);

test("logs use domain pagination, append without resetting scroll, then search and clear correctly", async () => {
  const { context, calls, client, props } = setup("src/pages/logs.model.js", (url, params) => {
    if (url.endsWith("/clear")) return {};
    const offset = (Number(params.page) - 1) * 300;
    return { entries: Array.from({ length: 300 }, (_, index) => ({ message: `log ${offset + index}`, level: "error" })),
      total: 600, files: [{ path: "/logs/test.log" }] };
  });
  const model = context.LogsPageViewModel({ ...props, client });
  await model.methods.ready();
  assert.equal(model.state.status.value, "normal");
  assert.equal(model.state.entries.value.length, 300);
  assert.equal(model.state.log_file_path.value, "/logs/test.log");
  const count = model.state.list.state.completed.value;
  await model.methods.handleListScroll({ scrollTop: 100, clientHeight: 300, scrollHeight: 500 });
  assert.equal(model.state.entries.value.length, 600);
  assert.equal(model.state.list.state.completed.value, count);
  assert.equal(Number(calls.at(-1).params.page), 2);
  model.methods.setKeyword("query");
  await model.methods.search();
  assert.equal(model.state.entries.value.length, 300);
  assert.equal(calls.at(-1).params.keyword, "query");
  await model.methods.confirmClearLogs();
  assert.equal(model.state.status.value, "empty");
  assert.equal(model.state.list.response.dataSource.length, 0);
  model.methods.destroy();
});

test("download clamping keeps domain parameters and live task instances synchronized", async () => {
  const { context, props } = setup("src/pages/downloadv2.model.js", () => ({}));
  const task_list = context.refarr([]);
  const requests = [];
  let total = 101;
  const downloader = { task_list, refresh() {}, async loadPage(options) {
    requests.push({ ...options });
    const count = Math.max(0, Math.min(options.page_size, total - (options.page - 1) * options.page_size));
    task_list.as(Array.from({ length: count }, (_, index) => ({ state: { id: context.ref((options.page - 1) * 50 + index) } })));
    return { total, page: options.page, page_size: 50 };
  } };
  const model = context.DownloadV2Model({ ...props, downloader });
  await model.methods.ready();
  await model.methods.changePage(3);
  assert.equal(model.state.page.value, 3);
  total = 51;
  await model.methods.refreshTasks();
  await model.methods.changePage(2);
  assert.equal(model.state.tasks.value[0], task_list.value[0]);
  total = 1;
  await model.state.list.goto(3);
  assert.equal(model.state.page.value, 1);
  assert.equal(model.state.list.params.page, 1);
  assert.equal(requests.at(-1).page, 1);
  model.methods.clean();
});

test("sync filters cached server models through search and refresh replaces them", async () => {
  const { context, calls, client, props } = setup("src/pages/sync.model.js", () => ({ servers: [
    { id: 1, name: "alpha", url: "http://alpha" }, { id: 2, name: "beta", url: "http://beta" },
  ] }));
  const model = context.SyncPageModel({ ...props, client });
  await model.methods.ready();
  const server = model.state.visible_servers.value[1];
  model.ui.input_keyword$.setValue("beta");
  await flush();
  assert.equal(model.state.visible_servers.value.length, 1);
  assert.equal(model.state.visible_servers.value[0], server);
  assert.equal(calls.length, 1);
  model.ui.input_keyword$.setValue("missing");
  await flush();
  assert.equal(model.state.status.value, "empty");
  await model.methods.ready();
  assert.equal(calls.length, 2);
  model.methods.destroy();
});

test("filehelper reads message snapshots through ListCore, deduplicates and reports sync failures", async () => {
  let fail = false;
  const { context, client, props } = setup("src/pages/filehelper.model.js", (url) => {
    if (url.endsWith("/status")) return { logged_in: true };
    if (url.endsWith("/synccheck")) return new Promise(() => {});
    if (url.endsWith("/sync")) {
      if (fail) throw new Error("消息同步失败");
      return { AddMsgList: [{ MsgId: "one", MsgType: 1, Content: "hello", FromUserName: "filehelper" },
        { MsgId: "one", FromUserName: "filehelper" }, { MsgId: "other", FromUserName: "other", ToUserName: "other" }] };
    }
    return {};
  });
  const model = context.FileHelperViewModel({ ...props, client });
  await model.methods.ready();
  await flush();
  assert.equal(model.state.message_list.state.status.value, "normal");
  assert.equal(model.state.messages.value.length, 1);
  assert.equal(model.state.messages.value[0].text, "hello");
  await model.state.message_list.search({});
  assert.equal(model.state.messages.value.length, 1);
  fail = true;
  await model.state.message_list.search({});
  assert.equal(model.state.message_list.state.status.value, "error");
  model.methods.destroy();
});

test("tag options use list states and local search without repeating the network request", async () => {
  let fail = false;
  const { context, calls, client, props } = setup("src/components/index.js", () => {
    if (fail) throw new Error("标签加载失败");
    return [{ id: 1, name: "alpha" }, { id: 2, name: "beta" }];
  });
  const model = context.TagSelectModel({ ...props, client });
  model.ui.popover_store.show();
  await flush();
  assert.equal(model.state.list.state.status.value, "normal");
  assert.equal(calls.length, 1);
  model.methods.set_search_keyword("beta", true);
  await flush();
  assert.equal(model.state.filtered_.value.length, 1);
  assert.equal(model.state.filtered_.value[0].name, "beta");
  assert.equal(calls.length, 1);
  fail = true;
  await model.ui.retry$.click();
  await flush();
  assert.equal(model.state.list.state.status.value, "error");
  model.methods.destroy();
});

test("account content cursors advance, deduplicate appended rows and stop at the final marker", async () => {
  const { context, calls, client, props } = setup("src/pages/account.model.js", (_url, params) => {
    const page = params.page || "";
    const contents = page === "two" ? [{ id: "one" }, { id: "two" }] : page === "three" ? [{ id: "three" }] : [{ id: "one" }];
    return { scope: "feed", scopes: [{ value: "feed", label: page || "first" }], contents,
      next_marker: page === "two" ? "three" : page === "three" ? "" : "two" };
  });
  context.normalize_content_item = (item) => item;
  context.content_type_label = () => "";
  const model = context.AccountViewModel({ ...props, client });
  await model.methods.openAccount({ id: "account", platform_id: "wx_channels" });
  const revision = model.state.drawer_list.state.completed.value;
  assert.equal(model.state.drawer_next_marker.value, "two");
  await model.methods.loadMoreAccountContents();
  assert.equal(model.state.drawer_contents.value.length, 2);
  assert.equal(model.state.drawer_next_marker.value, "three");
  assert.equal(model.state.drawer_tabs.value[0].name, "two");
  await model.methods.loadMoreAccountContents();
  assert.equal(model.state.drawer_contents.value.length, 3);
  assert.equal(model.state.drawer_next_marker.value, "");
  assert.equal(model.state.drawer_list.state.completed.value, revision);
  await model.methods.loadMoreAccountContents();
  assert.equal(calls.length, 3);
  assert.equal(calls[2].params.page, "three");
  model.methods.destroy();
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup() {
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval() {}, clearInterval() {} });
  context.window = context;
  const read = (file) => fs.readFileSync(path.join(__dirname, "..", file), "utf8");
  vm.runInContext(read("public/timeless/0.33.1/timeless.umd.min.js"), context);
  Object.assign(context, context.Timeless);
  for (const file of ["src/utils.js", "src/list.model.js", "src/dmui.js"]) {
    vm.runInContext(read(file).replace(/^import[\s\S]*?;\n/gm, "")
      .replace(/^export \{[^}]+\};\n/gm, "").replace(/^export /gm, ""), context);
  }
  return context;
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

const flush = () => new Promise((resolve) => setImmediate(resolve));

test("ListCore drives skeleton, loading, empty, error and rows; completion observes committed data", async () => {
  const context = setup();
  let response = deferred();
  const tips = [];
  const list = context.create_list_model({
    request: context.create_list_request(() => response.promise), page_size: 2,
    tip(tip) { tips.push(tip); },
  });
  assert.ok(list instanceof context.Timeless.kit.ListCore);
  assert.equal(list.state.status.value, "initial");
  const snapshots = [];
  list.state.completed.subscribe({ onChange() {
    snapshots.push([list.state.status.value, list.state.rows.value.map((item) => item.id).join(",")]);
  } });
  let task = list.search({ keyword: "first" });
  await flush();
  assert.equal(list.state.status.value, "initial");
  response.resolve({ list: [{ id: "a" }, { id: "b" }], total: 3, page: 1 });
  await task;
  assert.deepEqual(snapshots[0], ["normal", "a,b"]);
  response = deferred();
  task = list.search({ keyword: "empty" });
  await flush();
  assert.equal(list.state.loading.value, true);
  assert.equal(list.state.status.value, "normal");
  response.resolve({ list: [], total: 0, page: 1 });
  await task;
  assert.equal(list.state.status.value, "empty");
  response = deferred();
  task = list.search({ keyword: "again" });
  await flush();
  assert.equal(list.state.status.value, "loading");
  response.resolve({ list: [{ id: "c" }], total: 1, page: 1 });
  await task;
  list.request.run = async () => context.Timeless.Result.Err(new Error("网络断开"));
  await list.goto(1);
  assert.deepEqual(snapshots.at(-1), ["error", "c"]);
  assert.equal(tips.length, 1);
  assert.equal(list.state.loading.value, false);
  list.destroy();
});

test("search clears removed filters, pagination calls the domain and append keeps the scroll position", async () => {
  const context = setup();
  const params = [];
  const list = context.create_list_model({ page_size: 2, request: context.create_list_request((query) => {
    params.push(JSON.parse(JSON.stringify(query)));
    return { list: [{ id: query.page * 2 }, { id: query.page * 2 + 1 }], total: 8, page: query.page };
  }) });
  const scrolls = [];
  const props = context.list_scroll_props(list);
  const unlisten = props.onMounted({ target: { get$elm: () => ({ scrollTo: (position) => scrolls.push(position.top) }) } });
  await list.search({ keyword: "remove", platform_id: "platform" });
  await list.next();
  await list.prev();
  await list.goto(3);
  assert.equal(list.state.page.value, 3);
  assert.equal(params[3].platform_id, "platform");
  await list.search({ keyword: "" });
  assert.equal("platform_id" in params[4], false);
  assert.equal("pageSize" in params[4], false);
  assert.equal(params[4].page_size, 2);
  assert.equal(list.state.page.value, 1);
  assert.equal(scrolls.length, 5);
  await list.loadMore();
  assert.equal(list.state.rows.value.length, 4);
  assert.equal(scrolls.length, 5);
  unlisten();
  await list.search({});
  assert.equal(scrolls.length, 5);
  list.destroy();
});

test("rapid searches are serialized and disposing prevents queued requests and late view changes", async () => {
  const context = setup();
  const response = deferred();
  const queries = [];
  const list = context.create_list_model({ request: context.create_list_request(async (params) => {
    queries.push(params.keyword);
    if (params.keyword === "first") await response.promise;
    return { list: [{ id: params.keyword }], page: 1, total: 1 };
  }) });
  const first = list.search({ keyword: "first" });
  const last = list.search({ keyword: "last" });
  await flush();
  assert.deepEqual(queries, ["first"]);
  response.resolve();
  await Promise.all([first, last]);
  assert.equal(list.state.rows.value[0].id, "last");
  const pending = list.search({ keyword: "disposed" });
  list.destroy();
  await pending;
  assert.deepEqual(queries, ["first", "last"]);
  assert.equal(list.state.rows.value[0].id, "last");
});

test("table scrolls after every committed replacement, including failed searches, and unsubscribes", async () => {
  const context = setup();
  const response = deferred();
  let fail = false;
  const list = context.create_list_model({ request: context.create_list_request(async () => {
    if (fail) throw new Error("失败");
    await response.promise;
    return { list: [{ id: "a" }], page: 1, total: 1 };
  }) });
  let table_props;
  let container;
  const scrolls = [];
  context.TablePanel = (props) => { table_props = props; return null; };
  context.View = (props, children) => { container = props; return { props, children }; };
  context.Show = () => null;
  context.Table({ name: "test-list", list, rows: list.state.rows, status: list.state.status });
  table_props.onScrollView({ setScrollTop(top) { scrolls.push([top, list.state.rows.value.length]); } });
  const task = list.search({});
  await flush();
  assert.equal(scrolls.length, 0);
  response.resolve();
  await task;
  assert.deepEqual(scrolls, [[0, 1]]);
  fail = true;
  await list.search({});
  assert.deepEqual(scrolls, [[0, 1], [0, 1]]);
  container.onUnmounted();
  await list.search({});
  assert.equal(scrolls.length, 2);
  list.destroy();
});

test("a failed next page does not advance the cursor used by the retry", async () => {
  const context = setup();
  let fail = false;
  const list = context.create_list_model({ page_size: 2, request: context.create_list_request((params) => {
    if (fail) throw new Error("断开");
    return { list: [{ id: params.page * 2 }, { id: params.page * 2 + 1 }], page: params.page, total: 6 };
  }) });
  await list.search({});
  fail = true;
  await list.next();
  assert.equal(list.params.page, 1);
  fail = false;
  await list.next();
  assert.equal(list.state.page.value, 2);
  list.destroy();
});

function find_pagination_node(root, name) {
  const queue = [root];
  while (queue.length) {
    const node = queue.shift();
    if (node?.state?.attributes?.n === name) return node;
    queue.push(...(node?.children || []));
  }
}

for (const table of [false, true]) {
  test(`${table ? "table" : "standalone"} pagination shows skeletons only for ListCore.initial and preserves controls while paging`, async () => {
    const context = setup();
    let response = deferred();
    let fail = false;
    const list = context.create_list_model({ page_size: 2, request: context.create_list_request(async (params) => {
      await response.promise;
      if (fail) throw new Error("加载失败");
      return { list: [{ id: params.page * 2 }, { id: params.page * 2 + 1 }], total: 6, page: params.page };
    }) });
    const props = { list, page: list.state.page, pageCount: 3, loading: false, summary: "共 6 条", onChange: (page) => list.goto(page) };
    const pagination = table
      ? context.TablePagination({ list, status: context.ref("initial"), loading: false, pagination: props })
      : context.Pagination(props);
    const find = (name) => find_pagination_node(pagination, name);
    const skeletons = () => [find("pagination-summary-skeleton"), find("pagination-controls-skeleton")];
    assert.ok(skeletons().every(Boolean));
    assert.equal(find("pagination-loading"), undefined);
    assert.equal(find("pagination-next").state.attributes.disabled, true);
    const initial = list.search({});
    await flush();
    assert.ok(skeletons().every(Boolean));
    response.resolve();
    await initial;
    assert.ok(skeletons().every((node) => !node));
    assert.ok(!find("pagination-controls").state.styleSet.includes("is-loading"));
    const page_two = find("pagination-page-2");
    response = deferred();
    const next = page_two.events.onClick();
    await flush();
    assert.equal(list.state.initial.value, false);
    assert.ok(skeletons().every((node) => !node));
    assert.ok(find("pagination-loading"));
    assert.equal(find("pagination-page-2"), page_two);
    assert.equal(page_two.state.attributes.disabled, true);
    assert.ok(!find("pagination-summary").state.styleSet.includes("is-loading"));
    assert.ok(!find("pagination-controls").state.styleSet.includes("is-loading"));
    response.resolve();
    await next;
    assert.equal(list.state.page.value, 2);
    assert.equal(find("pagination-loading"), undefined);
    assert.ok(!find("pagination-next").state.attributes.disabled);
    fail = true;
    await list.search({});
    assert.equal(list.state.status.value, "error");
    assert.ok(skeletons().every((node) => !node));
    assert.equal(find("pagination-loading"), undefined);
    find("pagination").onUnmounted();
    list.destroy();
  });
}

test("retrying a failed initial request uses pagination loading, not skeletons", async () => {
  const context = setup();
  let response = deferred();
  let fail = true;
  const list = context.create_list_model({ request: context.create_list_request(async () => {
    await response.promise;
    if (fail) throw new Error("首次失败");
    return { list: [], page: 1, total: 0 };
  }) });
  const pagination = context.Pagination({ list });
  const first = list.search({});
  response.resolve();
  await first;
  fail = false;
  response = deferred();
  const retry = list.search({});
  await flush();
  assert.equal(find_pagination_node(pagination, "pagination-summary-skeleton"), undefined);
  assert.equal(find_pagination_node(pagination, "pagination-controls-skeleton"), undefined);
  assert.ok(find_pagination_node(pagination, "pagination-loading"));
  response.resolve();
  await retry;
  assert.equal(find_pagination_node(pagination, "pagination-loading"), undefined);
  pagination.onUnmounted();
  list.destroy();
});

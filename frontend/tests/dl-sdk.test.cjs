const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

function load_model(request, initial = { id: 42, status: 2 }) {
  const context = {
    console, setTimeout, clearTimeout, clearInterval, queueMicrotask,
    setInterval(...args) { const timer = setInterval(...args); timer.unref(); return timer; },
  };
  context.window = context;
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../public/dl.sdk.js"), "utf8"), context);
  const logger = new Proxy({}, { get: () => () => logger });
  return context.DownloadTaskModel({
    data: initial, logger,
    owner: { reqs: { download: { detail: { run: request } } } },
  });
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

function task_detail(name = "成片.mp4") {
  return {
    id: 42, name: "测试任务", platform_id: "wxchannels", status: 5, error: "",
    content: { id: "wxchannels:42", accounts: [{ id: "author" }] },
    config_json: "{}", metadata_json: "{}", file_count: 1,
    files: [{ id: 7, name, file_path: `/downloads/${name}`, local_path: `/downloads/${name}`, exists: true, status: "finished", playback_available: false }],
  };
}

test("success awaits canonical detail and replaces merged intermediate files", async () => {
  const response = deferred();
  let requests = 0;
  const model = load_model(({ id }) => { assert.equal(id, 42); requests++; return response.promise; }, {
    id: 42, status: 2, resources: [{ id: 7 }, { id: 8 }], files: [{ id: 7 }, { id: 8 }],
  });
  const received = [];
  model.onSuccess((detail) => received.push(detail));
  model._update({ id: 42, status: 4, progress: 100 });
  assert.equal(requests, 0);
  model._update({ id: 42, status: 5, files: [{ id: 7 }] });
  model._update({ id: 42, status: 5 });
  assert.equal(requests, 1);
  assert.equal(received.length, 0);
  const expected = task_detail();
  response.resolve({ data: expected });
  await model.finished;
  assert.equal(received.length, 1);
  assert.deepEqual(JSON.parse(JSON.stringify(received[0])), expected);
  assert.equal(model.files.length, 1);
  assert.equal(model.state.raw.value.resources, undefined);
  let late;
  model.onSuccess((detail) => { late = detail; });
  await Promise.resolve();
  assert.strictEqual(late, received[0]);
  model._dispose();
});

test("detail errors reject completion instead of emitting a partial success", async () => {
  const model = load_model(async () => ({ error: new Error("详情加载失败") }));
  let successes = 0;
  model.onSuccess(() => successes++);
  model._update({ id: 42, status: 5 });
  await assert.rejects(model.finished, /详情加载失败/);
  assert.equal(successes, 0);
  assert.equal(model.state.status.value, "finished");
  model._dispose();
});

test("subscribing to an already completed task loads the same detail", async () => {
  const expected = task_detail();
  let requests = 0;
  const model = load_model(async () => { requests++; return { data: expected }; }, { id: 42, status: 5 });
  assert.equal(requests, 0);
  let received;
  model.onSuccess((detail) => { received = detail; });
  await model.finished;
  assert.equal(requests, 1);
  assert.deepEqual(JSON.parse(JSON.stringify(received)), expected);
  model._dispose();
});

test("a stale detail response cannot complete a retried task", async () => {
  const first = deferred();
  const second = deferred();
  let requests = 0;
  const model = load_model(() => (++requests === 1 ? first.promise : second.promise));
  const received = [];
  model.onSuccess((detail) => received.push(detail));
  model._update({ id: 42, status: 5 });
  model._begin("downloading");
  model._update({ id: 42, status: 5 });
  assert.equal(requests, 2);
  first.resolve({ data: task_detail("旧文件.mp4") });
  await Promise.resolve();
  assert.equal(received.length, 0);
  second.resolve({ data: task_detail("新文件.mp4") });
  await model.finished;
  assert.equal(received.length, 1);
  assert.equal(received[0].files[0].name, "新文件.mp4");
  model._dispose();
});

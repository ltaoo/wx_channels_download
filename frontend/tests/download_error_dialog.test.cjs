const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup() {
  const timers = new Map();
  let timer_id = 0;
  const context = vm.createContext({
    setTimeout(fn) { timers.set(++timer_id, fn); return timer_id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval() {},
    clearInterval() {},
  });
  context.window = context;
  context.config = {};
  const read = (file) => fs.readFileSync(path.join(__dirname, "..", file), "utf8");
  vm.runInContext(read("public/timeless/0.33.1/timeless.umd.min.js"), context);
  Object.assign(context, context.Timeless);
  const script = (file) => read(file)
    .replace(/^import[\s\S]*?;\n/gm, "")
    .replace(/^export \{[^}]+\};\s*$/gm, "")
    .replace(/^export /gm, "");
  for (const file of ["src/utils.js", "src/image-proxy.model.js", "src/dmui.js",
    "src/pages/downloadv2.model.js", "src/pages/downloadv2.components.js"]) {
    vm.runInContext(script(file), context);
  }
  const tips = [];
  const model = context.DownloadV2Model({
    downloader: { refresh() {} },
    tip(value) { tips.push(value); },
  });
  return { context, model, tips, timers };
}

function find(nodes, name) {
  const queue = Array.isArray(nodes) ? [...nodes] : [nodes];
  while (queue.length) {
    const node = queue.shift();
    if (node?.state?.attributes?.n === name) return node;
    queue.push(...(node?.children || []));
  }
}

test("the first click on a failed record displays its reason in the dialog", () => {
  const { context, model, tips } = setup();
  const dialog = context.TaskErrorDialog({ store: model });
  const task = { state: {
    status: context.ref("error"),
    progress: context.refobj({}),
    error: context.ref(null),
    raw: context.refobj({ error: "记录中的错误原因\n网络连接中断", content_id: "content" }),
    name: context.ref("下载任务"),
  } };
  const row = context.DownloadV2TaskMain({ task, store: model });
  const trigger = find(row, "download-task-status");
  assert.equal(trigger.state.attributes.type, "button");
  assert.equal(find(dialog, "download-task-error-reason"), undefined);
  trigger.events.onClick({ stopPropagation() {} });
  assert.equal(model.state.task_error.value, task.state.raw.value.error);
  assert.equal(find(dialog, "download-task-error-reason").children[0].state.value,
    task.state.raw.value.error);
  assert.equal(tips.length, 0);
  model.methods.clean();
});

test("reopening the dialog renders the new record's reason after the old content unmounts", () => {
  const { context, model, timers } = setup();
  const dialog = context.TaskErrorDialog({ store: model });
  model.methods.requestTaskError("第一个任务的错误");
  assert.equal(find(dialog, "download-task-error-reason").children[0].state.value, "第一个任务的错误");
  model.ui.taskErrorDialog$.hide();
  while (timers.size) {
    const pending = [...timers.values()];
    timers.clear();
    pending.forEach((fn) => fn());
  }
  model.methods.requestTaskError("第二个任务的错误");
  assert.equal(find(dialog, "download-task-error-reason").children[0].state.value, "第二个任务的错误");
  model.methods.clean();
});

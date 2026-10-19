const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup(options) {
  let now = 0;
  let sequence = 0;
  const timers = new Map();
  const context = vm.createContext({
    Date: class extends Date { static now() { return now; } },
    setTimeout(callback, delay) {
      timers.set(++sequence, { callback, deadline: now + delay });
      return sequence;
    },
    clearTimeout(id) { timers.delete(id); },
    setInterval() {},
    clearInterval() {},
  });
  context.window = context;
  const frontend = path.join(__dirname, "..");
  vm.runInContext(fs.readFileSync(path.join(frontend, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/dmui.js"), "utf8")
    .replace(/^export \{[^}]+\};\n/gm, "").replace(/^export /gm, ""), context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/utils.js"), "utf8").replace(/^export /gm, ""), context);
  return {
    context,
    model: context.SonnerModel(options),
    ref: context.Timeless.ref,
    advance(milliseconds) {
      now += milliseconds;
      for (const [id, timer] of [...timers]) {
        if (timer.deadline <= now) {
          timers.delete(id);
          timer.callback();
        }
      }
    },
  };
}

function mount_root(context) {
  let model;
  let page_props;
  context.Toaster = ({ store }) => { model = store; return context.Timeless.View(); };
  context.View = context.Timeless.View;
  context.history$ = { $view: {} };
  context.router = { views: {} };
  context.http_client$ = {};
  context.hls_player$ = {};
  context.storage$ = {};
  context.LoadingView = () => null;
  context.ErrorFallbackView = () => null;
  context.Timeless = { ...context.Timeless, ui: { ...context.Timeless.ui,
    KeepAliveSubViews(props) { page_props = props; return context.View(); },
  } };
  const source = fs.readFileSync(path.join(__dirname, "../src/index.js"), "utf8");
  vm.runInContext(source.slice(source.indexOf("function ApplicationRootView("), source.indexOf("\nfunction bootstrap(")), context);
  const app = new context.Timeless.kit.ApplicationModel({ storage: {}, clipboard: {} });
  const root = context.ApplicationRootView({ app });
  return { app, root, model, props: page_props };
}

test("identical errors create separate cards with independent expiration timers", () => {
  const { context, model, ref, advance } = setup({ duration: 1000 });
  const error = ref("");
  assert.equal(context.track_error(error, ({ msg }) => model.error(msg.join("\n"))), error);
  error.as("读取失败");
  assert.equal(model.state.toasts.value[0].message, "读取失败");
  const first_id = model.state.toasts.value[0].id;
  advance(600);
  error.as("读取失败");
  assert.equal(model.state.toasts.value.length, 2);
  assert.notEqual(model.state.toasts.value[1].id, first_id);
  advance(600);
  assert.equal(model.state.toasts.value.length, 2);
  assert.equal(model.state.toasts.value[0].removed, true);
  assert.equal(model.state.toasts.value[1].removed, false);
  advance(400);
  assert.equal(model.state.toasts.value[0].removed, true);
  advance(400);
  assert.equal(model.state.toasts.value.length, 0);
  error.as("");
  assert.equal(model.state.toasts.value.length, 0);
  error.as("读取失败");
  assert.equal(error.value, "读取失败");
  assert.equal(model.state.toasts.value.length, 1);
});

test("hover and keyboard focus pause independently, including notifications received while paused", () => {
  const { model, advance } = setup({ duration: 1000 });
  model.error(new Error("网络断开"));
  advance(400);
  model.pause("hover");
  model.pause("focus");
  advance(2000);
  model.error("保存失败");
  model.resume("hover");
  advance(2000);
  assert.equal(model.state.toasts.value.length, 2);
  model.resume("focus");
  advance(600);
  assert.equal(model.state.toasts.value.filter((toast) => !toast.removed).length, 1);
  advance(400);
  assert.equal(model.state.toasts.value.length, 1);
  advance(400);
  assert.equal(model.state.toasts.value.length, 0);
});

test("notifications are bounded and manual dismissal and cleanup cancel expiration timers", () => {
  const { model, advance } = setup({ duration: 1000, limit: 2 });
  model.error("旧错误");
  const second = model.error("新错误");
  const third = model.message("同步完成");
  advance(400);
  assert.equal(model.state.toasts.value.length, 2);
  assert.equal(model.state.toasts.value[0].id, second);
  model.dismiss(second);
  advance(400);
  assert.equal(model.state.toasts.value[0].id, third);
  model.clear();
  advance(1000);
  assert.equal(model.state.toasts.value.length, 0);
  assert.equal(model.error(""), null);
});

test("dismissing the last focused notification does not leave new notifications paused", () => {
  const { model, advance } = setup({ duration: 1000 });
  const id = model.error("保存失败");
  model.pause("focus");
  model.pause("hover");
  model.dismiss(id);
  model.error("再次保存失败");
  advance(1000);
  advance(400);
  assert.equal(model.state.toasts.value.length, 0);
});

test("error details use a native keyboard-accessible button and read the current error", () => {
  const { context, ref } = setup();
  const { model, props, root } = mount_root(context);
  const error = ref("旧错误");
  const button = context.ErrorMessage({ error, tip: props.tip });
  assert.equal(button.t, "button");
  assert.equal(model.state.toasts.value.length, 0);
  error.as("最新错误");
  button.events.onClick({ stopPropagation() {} });
  assert.equal(model.state.toasts.value[0].message, "最新错误");
  root.onUnmounted();
});

test("toast focus handlers supported by the bundled runtime pause and resume remaining notifications", () => {
  const { context, model, advance } = setup({ duration: 1000 });
  model.error("第一个错误");
  model.error("第二个错误");
  context.Timeless.For = ({ each, render }) => render(each.value[0]);
  const nodes = [context.Toaster({ store: model })];
  let close;
  while (nodes.length) {
    const node = nodes.shift();
    if (node.t === "button") { close = node; break; }
    nodes.push(...(node.children || []));
  }
  assert.ok(close);
  close.events.onFocus();
  advance(2000);
  assert.equal(model.state.toasts.value.length, 2);
  close.events.onClick();
  advance(1000);
  advance(400);
  assert.equal(model.state.toasts.value.length, 0);
});

test("account retries and separate detail dialogs each show a card for identical request errors", async () => {
  const { context } = setup();
  const { model: toaster, props, root } = mount_root(context);
  const frontend = path.join(__dirname, "..");
  for (const name of ["ref", "refarr", "refobj", "combine", "computed"]) context[name] = context.Timeless[name];
  context.format_time = () => "";
  context.content_type_label = () => "";
  const detail_requests = [];
  context.Timeless = { ...context.Timeless, kit: { ...context.Timeless.kit,
    RequestCore: class {
      constructor(handler) { this.handler = handler; }
      async run(params) {
        if (this.handler === context.fetchAccountContents) detail_requests.push(params);
        return context.Timeless.Result.Err(new Error("账号读取失败"));
      }
    },
  } };
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/biz/request.js"), "utf8").replace(/^export /gm, ""), context);
  const source = fs.readFileSync(path.join(frontend, "src/pages/account.model.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export \{[^}]+\};\s*$/m, "");
  vm.runInContext(source, context);
  const model = context.AccountViewModel({ client: {}, tip: props.tip });
  await model.methods.ready();
  assert.equal(model.state.error.value, "账号读取失败");
  assert.equal(model.state.status.value, "error");
  assert.equal(toaster.state.toasts.value[0].message, "账号读取失败");
  await model.methods.refresh();
  assert.equal(toaster.state.toasts.value.length, 2);
  toaster.clear();
  await model.methods.openAccount({ id: "account-a", platform_id: "wxchannels" });
  assert.equal(toaster.state.toasts.value.length, 1);
  await model.methods.openAccount({ id: "account-b", platform_id: "wxchannels" });
  assert.equal(model.state.drawer_error.value, "账号读取失败");
  assert.equal(model.state.drawer_status.value, "error");
  assert.deepEqual(detail_requests.map((request) => request.params.id), ["account-a", "account-b"]);
  assert.ok(detail_requests.every((request) => request.scope === "feed"));
  assert.equal(toaster.state.toasts.value.length, 2);
  assert.equal(toaster.state.toasts.value[0].message, toaster.state.toasts.value[1].message);
  assert.notEqual(toaster.state.toasts.value[0].id, toaster.state.toasts.value[1].id);
  root.onUnmounted();
});

test("the application root bridges message arrays and unsubscribes from app tips when removed", () => {
  const { context } = setup();
  const { app, props, model, root } = mount_root(context);
  assert.equal(context.sonner, undefined);
  props.tip({ msg: ["保存失败", "请检查网络连接"], type: "error" });
  assert.equal(model.state.toasts.value[0].message, "保存失败\n请检查网络连接");
  assert.equal(model.state.toasts.value[0].type, "error");
  app.tip({ text: ["同步完成"] });
  assert.equal(model.state.toasts.value[1].type, "normal");
  root.onUnmounted();
  assert.equal(model.state.toasts.value.length, 0);
  props.tip({ msg: ["卸载后不应再展示"] });
  assert.equal(model.state.toasts.value.length, 0);
});

test("the root dispatches notification types, warning aliases and unknown types safely", () => {
  const { context } = setup();
  const { props, model, root } = mount_root(context);
  for (const [type, expected] of [
    ["error", "error"], ["warn", "warn"], ["warning", "warn"],
    ["success", "success"], ["info", "info"], [undefined, "normal"],
    ["unknown", "normal"], ["clear", "normal"],
  ]) {
    model.clear();
    props.tip({ msg: ["操作结果"], type });
    assert.equal(model.state.toasts.value[0].type, expected);
    assert.equal(model.state.toasts.value[0].message, "操作结果");
  }
  model.clear();
  props.tip({ msg: ["相同内容"], type: "error" });
  props.tip({ msg: ["相同内容"], type: "success" });
  props.tip({ msg: ["相同内容"], type: "success" });
  assert.equal(model.state.toasts.value.length, 3);
  root.onUnmounted();
});

test("every notification type uses icons available in the bundled runtime", () => {
  const { context, model } = setup();
  const registry = context.Timeless.getIconRegistry();
  const icon = context.Timeless.Icon;
  context.Timeless.Icon = (props) => {
    assert.ok(registry[props.name], `Missing icon: ${props.name}`);
    return icon(props);
  };
  context.Timeless.For = ({ each, render }) => render(each.value[0]);
  for (const method of ["error", "warn", "success", "info", "message"]) {
    model.clear();
    model[method]("操作结果");
    context.Toaster({ store: model });
  }
  model.clear();
});

test("notifications collapse behind the newest card and expand using measured heights", () => {
  const { model } = setup();
  const oldest = model.error("较长的错误信息");
  const middle = model.warn("提醒");
  const newest = model.success("完成");
  model.measure(oldest, 150);
  model.measure(middle, 100);
  model.measure(newest, 80);
  let { cards, height } = model.state.layout.value;
  assert.equal(model.state.expanded.value, false);
  assert.equal(height, 108);
  assert.equal(cards[newest].offset, 0);
  assert.equal(cards[newest].hidden, false);
  assert.equal(cards[oldest].height, 80);
  assert.equal(cards[oldest].offset, 28);
  assert.equal(cards[oldest].scale, 0.9);
  assert.equal(cards[oldest].hidden, true);
  model.pause("hover");
  ({ cards, height } = model.state.layout.value);
  assert.equal(model.state.expanded.value, true);
  assert.equal(height, 358);
  assert.equal(cards[oldest].offset, 208);
  assert.equal(cards[oldest].height, 150);
  assert.equal(cards[oldest].scale, 1);
  assert.equal(cards[oldest].hidden, false);
  model.measure(newest, 120);
  assert.equal(model.state.layout.value.cards[oldest].offset, 248);
  model.pause("focus");
  model.resume("hover");
  assert.equal(model.state.expanded.value, true);
  model.resume("focus");
  assert.equal(model.state.expanded.value, false);
  model.pause("document");
  assert.equal(model.state.expanded.value, false);
  model.clear();
});

test("dismissal keeps the exit position until animation ends and immediately closes the layout gap", () => {
  const { model, advance } = setup();
  const oldest = model.error("旧消息");
  const newest = model.info("新消息");
  model.measure(oldest, 100);
  model.measure(newest, 80);
  model.pause("hover");
  model.dismiss(oldest);
  assert.equal(model.state.layout.value.cards[oldest].offset, 94);
  assert.equal(model.state.layout.value.cards[oldest].removed, true);
  assert.equal(model.state.layout.value.height, 80);
  model.dismiss(oldest);
  advance(399);
  assert.equal(model.state.toasts.value.length, 2);
  advance(1);
  assert.equal(model.state.toasts.value.length, 1);
  model.dismiss(newest);
  assert.equal(model.state.expanded.value, false);
  const replacement = model.info("新消息");
  assert.notEqual(replacement, newest);
  model.clear();
  advance(10000);
  assert.equal(model.state.toasts.value.length, 0);
  assert.equal(model.state.layout.value.height, 0);
});

test("card measurement unwraps the runtime DOM handle and waits until after mount", () => {
  const { context, model } = setup();
  const id = model.info("多行内容");
  let observed;
  let measure;
  let disconnected = false;
  context.ResizeObserver = class {
    constructor(callback) { measure = callback; }
    observe(element) { observed = element; }
    disconnect() { disconnected = true; }
  };
  context.Timeless.For = ({ each, render }) => render(each.value[0]);
  const nodes = [context.Toaster({ store: model })];
  let content;
  while (nodes.length) {
    const node = nodes.shift();
    if (node.state?.attributes?.n === "sonner-toast-content") { content = node; break; }
    nodes.push(...(node.children || []));
  }
  assert.ok(content);
  const element = {
    offsetHeight: 120,
    parentElement: { offsetHeight: 122, clientHeight: 120 },
    addEventListener() {}, getBoundingClientRect() {},
  };
  content.onMounted({ target: { get$elm: () => element } });
  assert.equal(observed, element);
  assert.equal(model.state.toasts.value[0].height, 0);
  measure();
  assert.equal(model.state.layout.value.cards[id].height, 122);
  content.onUnmounted();
  assert.equal(disconnected, true);
  model.clear();
});

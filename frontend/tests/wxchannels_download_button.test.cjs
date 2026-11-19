const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

const root = path.join(__dirname, "../..");
const utils = fs.readFileSync(path.join(root, "frontend/inject/utils.js"), "utf8");
const observe_node = utils.slice(utils.indexOf("    observe_node("),
  utils.indexOf("\n    /**", utils.indexOf("    observe_node("))).replace(/,\s*$/, "");

function setup(page, { loading = true, target = false, parsing = false, mounted = true } = {}) {
  const observers = [];
  const timers = new Map();
  const listeners = new Map();
  const nodes = new Map();
  function element() {
    const node = {
      children: [], attributes: {}, style: {}, className: "",
      setAttribute(key, value) { this.attributes[key] = value; },
      appendChild(child) { child.parentElement = this; this.children.push(child); },
      matches(selector) { return selector.split(".").filter(Boolean).every((name) => this.className.split(" ").includes(name)); },
      closest(selector) { return this.matches(selector) ? this : this.parentElement?.closest(selector) || null; },
      querySelector(selector) {
        for (const child of this.children) {
          const match = child.matches(selector) ? child : child.querySelector(selector);
          if (match) return match;
        }
        return null;
      },
    };
    node.classList = {
      add(name) { node.className += ` ${name}`; },
      remove(name) { node.className = node.className.split(" ").filter((item) => item !== name).join(" "); },
    };
    return node;
  }
  const body = element();
  const app = element();
  const operation = element();
  const selector = page === "profile" ? ".opr-area" : ".slides-item .click-box.op-item";
  const op_item = element();
  operation.appendChild(op_item);
  app.querySelector = (selector) => nodes.get(selector) || null;
  nodes.set("#app", app);
  if (mounted) app.appendChild(element());
  if (loading) nodes.set(".loading-content", element());
  if (target) nodes.set(selector, page === "profile" ? operation : op_item);
  const document = {
    body, documentElement: body, readyState: parsing ? "loading" : "complete",
    createElement: element,
    querySelector: (selector) => nodes.get(selector) || null,
    querySelectorAll: (selector) => nodes.has(selector) ? [nodes.get(selector)] : [],
    addEventListener: (event, fn) => listeners.set(event, fn),
    removeEventListener: (event) => listeners.delete(event),
  };
  const context = vm.createContext({
    document,
    MutationObserver: class {
      constructor(callback) { this.callback = callback; observers.push(this); }
      observe() { this.active = true; }
      disconnect() { this.active = false; }
    },
    setTimeout(fn) { const id = timers.size + 1; timers.set(id, fn); return id; },
    clearTimeout(id) { timers.delete(id); },
    __wx_channels_store__: { feed: null },
    Icons: { download_btn2: element, download_icon1: "" },
    WXU: { onFeed() {}, onInit() {}, onDOMContentLoaded() {}, attach_download_dropdown_menu() {} },
  });
  vm.runInContext(`WXU.observe_node = function ${observe_node};`, context);
  vm.runInContext(fs.readFileSync(path.join(root,
    `pkg/scraper/wxchannels/inject/channels.${page}.js`), "utf8"), context);
  return {
    body, app, operation, op_item, element, observers, document, listeners, nodes, selector, timers, context,
    set_target() { nodes.set(selector, page === "profile" ? operation : op_item); },
    mutate() {
      for (const observer of observers) {
        if (observer.active) observer.callback([], observer);
      }
    },
    button_count: () => operation.children.length - 1,
  };
}

for (const page of ["home", "profile", "feed"]) {
  test(`${page}: waits through loading, then inserts into a late target once`, () => {
    const env = setup(page);
    for (const timer of env.timers.values()) timer();
    env.set_target();
    env.mutate();
    assert.equal(env.body.children.length, 0);
    assert.equal(env.button_count(), 0);
    env.nodes.delete(".loading-content");
    env.mutate();
    env.mutate();
    assert.equal(env.body.children.length, 0);
    assert.equal(env.button_count(), 1);
  });

  test(`${page}: renders fixed only after loading ends without a target`, () => {
    const env = setup(page);
    env.mutate();
    assert.equal(env.body.children.length, 0);
    env.nodes.delete(".loading-content");
    env.mutate();
    env.mutate();
    assert.equal(env.body.children.length, 1);
    assert.equal(env.body.children[0].className, "wx-sider");
    assert.equal(env.button_count(), 0);
  });

  for (const target of [true, false]) {
    test(`${page}: already loaded, target=${target}`, () => {
      const env = setup(page, { loading: false, target });
      assert.equal(env.button_count(), target ? 1 : 0);
      assert.equal(env.body.children.length, target ? 0 : 1);
    });
  }

  test(`${page}: waits for parsing and the application to mount`, () => {
    const env = setup(page, { loading: false, parsing: true, mounted: false });
    env.nodes.delete("#app");
    env.document.readyState = "interactive";
    env.listeners.get("DOMContentLoaded")();
    assert.equal(env.body.children.length, 0);
    env.nodes.set("#app", env.app);
    env.mutate();
    assert.equal(env.body.children.length, 0);
    env.nodes.set(".loading-content", {});
    env.app.appendChild({});
    env.mutate();
    assert.equal(env.body.children.length, 0);
    env.nodes.delete(".loading-content");
    env.mutate();
    assert.equal(env.body.children.length, 1);
    assert.equal(env.listeners.has("DOMContentLoaded"), false);
  });

  test(`${page}: does not insert while the document is still parsing`, () => {
    const env = setup(page, { loading: false, parsing: true, target: true });
    assert.equal(env.button_count(), 0);
    env.document.readyState = "interactive";
    env.listeners.get("DOMContentLoaded")();
    assert.equal(env.button_count(), 1);
    assert.equal(env.body.children.length, 0);
  });

  for (const target of [true, false]) {
    test(`${page}: waits when the loading node appears after the application shell, target=${target}`, () => {
      const env = setup(page, { loading: false, parsing: true });
      env.document.readyState = "interactive";
      env.listeners.get("DOMContentLoaded")();
      env.mutate();
      assert.equal(env.body.children.length, 0);
      env.nodes.set(".loading-content", {});
      env.mutate();
      assert.equal(env.body.children.length, 0);
      if (target) env.set_target();
      env.nodes.delete(".loading-content");
      env.mutate();
      assert.equal(env.body.children.length, target ? 0 : 1);
      assert.equal(env.button_count(), target ? 1 : 0);
    });
  }
}

test("home: repeated insertion into the same action container is ignored", () => {
  const env = setup("home", { loading: false, target: true });
  env.context.__wx_insert_download_btn(env.operation);
  assert.equal(env.button_count(), 1);
});

for (const page of ["home", "feed"]) {
  for (const nested_first of [false, true]) {
    test(`${page}: nested share action uses the same download button, nested first=${nested_first}`, () => {
      const env = setup(page);
      env.operation.className = "feed-interact";
      const wrapper = env.element();
      const share = env.element();
      wrapper.appendChild(share);
      env.operation.appendChild(wrapper);
      const actions = nested_first ? [share, env.op_item] : [env.op_item, share];
      env.document.querySelectorAll = () => actions;
      env.set_target();
      env.nodes.delete(".loading-content");
      env.mutate();
      assert.equal(env.operation.children.filter((node) => node.matches(`.wx-${page}-download-btn`)).length, 1);
      assert.equal(wrapper.children.length, 1);
    });
  }

  test(`${page}: dynamically added nested share action inserts into the feed action bar`, () => {
    const env = setup(page);
    const scroll = env.element();
    scroll.className = "slides-scroll";
    scroll.nodeType = 1;
    const mount_observer = env.observers.at(-1);
    mount_observer.callback([{ type: "childList", addedNodes: [scroll] }], mount_observer);
    const slide = env.element();
    slide.className = "slides-item";
    env.operation.className = "feed-interact";
    slide.appendChild(env.operation);
    const wrapper = env.element();
    const share = env.element();
    share.className = "click-box op-item";
    share.nodeType = 1;
    wrapper.appendChild(share);
    env.operation.appendChild(wrapper);
    const observer = env.observers.at(-1);
    const mutation = [{ type: "childList", addedNodes: [share] }];
    observer.callback(mutation);
    observer.callback(mutation);
    assert.equal(env.operation.children.filter((node) => node.matches(`.wx-${page}-download-btn`)).length, 1);
    assert.equal(wrapper.children.length, 1);
  });
}

test("observe_node: callers without a loading selector retain timeout fallback", () => {
  const env = setup("profile");
  let failed = false;
  env.context.WXU.observe_node({ selector: ".missing", container: "#app",
    onOk() { assert.fail("target does not exist"); }, onError() { failed = true; } });
  assert.equal(env.timers.size, 1);
  for (const timer of env.timers.values()) timer();
  assert.equal(failed, true);
});

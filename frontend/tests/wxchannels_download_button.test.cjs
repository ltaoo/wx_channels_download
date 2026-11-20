const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

function element(class_name = "") {
  const node = {
    nodeType: 1, className: class_name, children: [], style: {},
    setAttribute() {},
    appendChild(child) { child.parentElement = this; this.children.push(child); },
    matches(selector) {
      return selector.split(".").filter(Boolean).every((name) => this.className.split(" ").includes(name));
    },
    closest(selector) { return this.matches(selector) ? this : this.parentElement?.closest(selector) || null; },
    querySelectorAll(selector) {
      return this.children.flatMap((child) => [
        ...(child.matches(selector) ? [child] : []), ...child.querySelectorAll(selector),
      ]);
    },
    querySelector(selector) { return this.querySelectorAll(selector)[0] || null; },
  };
  node.classList = { add(name) { node.className += ` ${name}`; } };
  return node;
}

function setup(page, { nested_first = false, action_bar = true } = {}) {
  const slide = element("slides-item");
  const operation = element(action_bar ? "feed-interact" : "");
  const op_item = element("click-box op-item");
  const wrapper = element();
  const share = element("click-box op-item");
  slide.appendChild(operation);
  operation.appendChild(op_item);
  wrapper.appendChild(share);
  if (action_bar) operation.appendChild(wrapper);
  const actions = action_bar
    ? (nested_first ? [share, op_item] : [op_item, share]) : [op_item];
  const callbacks = new Map();
  const observers = [];
  const context = vm.createContext({
    document: { querySelectorAll: () => actions },
    MutationObserver: class {
      constructor(callback) { this.callback = callback; observers.push(this); }
      observe() {}
    },
    __wx_channels_store__: { feed: null },
    Icons: { download_btn2: element },
    WXU: {
      onFeed() {}, onDOMContentLoaded() {}, attach_download_dropdown_menu() {},
      observe_node({ selector, onOk }) { callbacks.set(selector, onOk); },
    },
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, "../../pkg/scraper/wxchannels/inject",
    `channels.${page}.js`), "utf8"), context);
  const initialize = () => callbacks.get(".slides-item")(slide);
  callbacks.get(".slides-scroll")(element("slides-scroll"));
  return {
    operation, wrapper, actions, initialize,
    button_count: () => operation.querySelectorAll(`.wx-${page}-download-btn`).length,
    mutate(node) { observers[0].callback([{ type: "childList", addedNodes: [node] }]); },
  };
}

for (const page of ["home", "feed"]) {
  for (const nested_first of [false, true]) {
    test(`${page}: nested share action reuses the action bar button, nested first=${nested_first}`, () => {
      const env = setup(page, { nested_first });
      env.initialize();
      assert.equal(env.button_count(), 1);
      assert.equal(env.operation.children.at(-1).matches(`.wx-${page}-download-btn`), true);
      assert.equal(env.wrapper.children.length, 1);
    });
  }

  test(`${page}: repeated initialization does not insert another button`, () => {
    const env = setup(page);
    env.initialize();
    env.initialize();
    assert.equal(env.button_count(), 1);
  });

  for (const initialized of [false, true]) {
    test(`${page}: dynamically added nested share action reuses the action bar, initialized=${initialized}`, () => {
      const env = setup(page);
      if (initialized) env.initialize();
      const share = element("click-box op-item");
      env.wrapper.appendChild(share);
      env.mutate(share);
      env.mutate(share);
      assert.equal(env.button_count(), 1);
      assert.equal(env.operation.children.at(-1).matches(`.wx-${page}-download-btn`), true);
      assert.equal(env.wrapper.children.length, 2);
    });
  }

  test(`${page}: action containers without feed-interact still get one button`, () => {
    const env = setup(page, { action_bar: false });
    env.initialize();
    env.initialize();
    assert.equal(env.button_count(), 1);
  });

  test(`${page}: separate action bars each get a download button`, () => {
    const env = setup(page);
    const operation = element("feed-interact");
    const op_item = element("click-box op-item");
    operation.appendChild(op_item);
    env.actions.push(op_item);
    env.initialize();
    assert.equal(env.button_count(), 1);
    assert.equal(operation.querySelectorAll(`.wx-${page}-download-btn`).length, 1);
  });
}

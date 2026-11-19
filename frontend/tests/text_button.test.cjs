const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function setup() {
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval() {}, clearInterval() {}, URLSearchParams });
  context.window = context;
  const read = (file) => fs.readFileSync(path.join(__dirname, "..", file), "utf8");
  vm.runInContext(read("public/timeless/0.33.1/timeless.umd.min.js"), context);
  Object.assign(context, context.Timeless);
  for (const file of ["src/dmui.js", "src/pages/browsehistory.js", "src/pages/automation.components.js"]) {
    vm.runInContext(read(file)
      .replace(/^import[\s\S]*?;\n/gm, "")
      .replace(/^export \{[^}]+\};\n/gm, "")
      .replace(/^export default .*;$/gm, "")
      .replace(/^export /gm, ""), context);
  }
  return context;
}

test("text actions preserve clicks, labels and reactive attributes", () => {
  const context = setup();
  const label = context.ref("暂停");
  let clicks = 0;
  const button = context.TextButton({ onClick() { clicks++; }, attributes: { n: "pause-task", title: label } }, [label]);
  assert.equal(button.state.attributes.type, "button");
  assert.equal(button.state.attributes.n, "pause-task");
  assert.ok(button.state.styleSet.includes("dm-button--link"));
  assert.equal(button.children[0].state.value, "暂停");
  button.events.onClick({});
  assert.equal(clicks, 1);
  label.as("停止录制");
  assert.equal(button.state.attributes.title, "停止录制");
});

test("store text buttons retain loading and disabled behavior", () => {
  const context = setup();
  let clicks = 0;
  const store = new context.Timeless.vm.ButtonCore({ variant: "link", size: "sm", onClick() { clicks++; } });
  const button = context.TextButton({ store }, ["删除"]);
  button.events.onClick({});
  assert.equal(clicks, 1);
  store.setLoading(true);
  assert.equal(button.state.attributes.disabled, true);
  assert.equal(button.state.attributes["aria-busy"], "true");
  button.events.onClick({});
  assert.equal(clicks, 1);
  store.setLoading(false);
  store.disable();
  assert.equal(button.state.attributes.disabled, true);
  button.events.onClick({});
  assert.equal(clicks, 1);
});

test("history and automation text links retain destinations and new-tab protection", () => {
  const context = setup();
  const links = [
    [context.BrowseHistorySourceAction({ href: "https://example.com/video" }), "https://example.com/video"],
    [context.AutomationPipelineLink({ flowId: "flow a", mode: "edit", label: "编辑流程" }), "/automation/edit?id=flow+a"],
    [context.AutomationPipelineLink({ flowId: "instance", mode: "detail", label: "详情" }), "/automation/detail?id=instance"],
  ];
  for (const [link, href] of links) {
    assert.equal(link.state.href, href);
    assert.equal(link.state.target, "_blank");
    assert.equal(link.state.attributes.rel, "noopener noreferrer");
    assert.ok(link.state.styleSet.includes("dm-button--link"));
  }
});

test("buttons and text buttons render single or multiple suffix nodes after the label", () => {
  const context = setup();
  const store = new context.Timeless.vm.ButtonCore({ variant: "link", size: "sm" });
  const icon = context.Timeless.Icon({ name: "chevron-down", size: 14, attributes: { n: "more-arrow", "aria-hidden": "true" } });
  const badge = context.View({ attributes: { n: "more-badge" } }, ["2"]);
  for (const [render, props] of [
    [context.Button, { store }],
    [context.TextButton, { store }],
    [context.TextButton, {}],
    [context.TextButton, { href: "https://example.com" }],
  ]) {
    for (const suffix of [icon, [icon, badge]]) {
      const button = render({ ...props, suffix }, ["更多"]);
      const trailing = button.children.at(-1);
      assert.equal(trailing.state.attributes.n, "button-suffix");
      assert.ok(trailing.state.styleSet.includes("dm-button__suffix"));
      assert.deepEqual(Array.from(trailing.children), Array.isArray(suffix) ? suffix : [suffix]);
      const content = button.children.at(-2);
      assert.equal(content.state.value || content.children[0].state.value, "更多");
    }
  }
});

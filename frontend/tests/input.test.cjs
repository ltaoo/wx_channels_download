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
  vm.runInContext(read("src/dmui.js")
    .replace(/^export \{[^}]+\};\n/gm, "")
    .replace(/^export /gm, ""), context);
  return context;
}

function input_node(view) {
  return view.children.find((node) => node?.state?.attributes?.n === "input");
}

test("Input focuses its mounted DOM element when autoFocus is enabled, including after reopening", () => {
  const context = setup();
  for (const autoFocus of [true, false]) {
    const store = new context.Timeless.vm.InputCore({ defaultValue: "名称", autoFocus });
    const input = input_node(context.Input({ store }));
    let focuses = 0;
    const mounted = { target: { get$elm: () => ({ focus() { focuses++; } }) } };
    input.onMounted(mounted);
    assert.equal(focuses, autoFocus ? 1 : 0);
    input.onUnmounted();
    input.onMounted(mounted);
    assert.equal(focuses, autoFocus ? 2 : 0);
    input.onUnmounted();
  }
});

test("Input onEnter receives the current value and ignores Chinese composition events", () => {
  const context = setup();
  const entered = [];
  const model_entered = [];
  const keys = [];
  const store = new context.Timeless.vm.InputCore({ defaultValue: "旧名称", onEnter(value) { model_entered.push(value); } });
  const input = input_node(context.Input({ store, onEnter(value) { entered.push(value); }, onKeyDown(event) { keys.push(event.key); } }));
  input.events.onChange({ target: { value: "新名称" } });
  for (const event of [{ key: "a" }, { key: "Enter", isComposing: true }, { key: "Enter", keyCode: 229 }]) {
    input.events.onKeyDown(event);
  }
  assert.equal(entered.length, 0);
  assert.equal(model_entered.length, 0);
  input.events.onKeyDown({ key: "Enter" });
  assert.deepEqual(entered, ["新名称"]);
  assert.deepEqual(model_entered, ["新名称"]);
  assert.deepEqual(keys, ["a", "Enter", "Enter", "Enter"]);
});

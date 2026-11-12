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

test("TableFields separates labels and values without adding colons or hiding zero and false", () => {
  const context = setup();
  const table = context.TableFields({ name: "parameters", fields: [
    { label: "重试次数", value: 0 },
    { name: "enabled", label: "启用", value: false },
  ] });
  assert.equal(table.state.attributes.n, "parameters");
  assert.equal(table.children[0].state.attributes.n, "parameters-field-0");
  assert.equal(table.children[1].state.attributes.n, "enabled");
  assert.deepEqual(Array.from(table.children, (row) =>
    Array.from(row.children, (cell) => cell.children[0].state.value)), [["重试次数", "0"], ["启用", "false"]]);
  assert.equal(context.TableFields({ fields: [] }).children.length, 0);
});

test("TableFields preserves custom value nodes and their actions", () => {
  const context = setup();
  let clicks = 0;
  const value = context.View({ attributes: { n: "masked-value" } }, ["••••••"]);
  const action = context.TextButton({ attributes: { n: "view-secret" }, onClick() { clicks++; } }, ["查看"]);
  const table = context.TableFields({ fields: [{ label: "令牌", value: [value, action] }] });
  const cells = table.children[0].children;
  assert.equal(cells[0].children[0].state.value, "令牌");
  assert.equal(cells[1].children[0], value);
  assert.equal(cells[1].children[1], action);
  cells[1].children[1].events.onClick({});
  assert.equal(clicks, 1);
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const context = vm.createContext({
  setTimeout,
  clearTimeout,
  setInterval: () => 0,
  clearInterval,
});
context.window = context;
vm.runInContext(
  fs.readFileSync(path.join(frontend, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"),
  context,
);
const source = fs.readFileSync(path.join(frontend, "src/pages/automation.model.js"), "utf8");
vm.runInContext(
  source.slice(source.indexOf("function flow_label("), source.indexOf("function request_result_error(")),
  context,
);

test("every supported node type has a distinct icon available in the bundled runtime", () => {
  const types = [
    "StartNode", "EndNode", "ExprNode", "GatewayNode", "APICallNode",
    "ServiceNode", "ManualNode", "FuncNode", "LoopNode", "WorkflowNode",
    "JSCodeNode", "SetVariableNode", "WaitNode", "FailNode",
  ];
  const registry = context.Timeless.getIconRegistry();
  const icons = types.map((type) => context.flow_icon(type));
  assert.equal(new Set(icons).size, types.length);
  types.forEach((type, index) => {
    assert.ok(registry[icons[index]], `${type}: missing icon ${icons[index]}`);
    assert.notEqual(context.flow_label(type), type);
  });
});

test("unknown node types have an available fallback icon and keep their label", () => {
  assert.equal(context.flow_icon("UnknownNode"), "box");
  assert.ok(context.Timeless.getIconRegistry().box);
  assert.equal(context.flow_label("UnknownNode"), "UnknownNode");
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function load_model(t, client) {
  const root = path.join(__dirname, "..");
  const context = vm.createContext({
    console: { ...console, log() {} }, URL, clearTimeout, clearInterval, queueMicrotask,
    setTimeout(fn, ms) { const timer = setTimeout(fn, ms); timer.unref(); return timer; },
    setInterval(fn, ms) { const timer = setInterval(fn, ms); timer.unref(); return timer; },
    window: { __store: { socket_client$: {} }, location: { origin: "http://localhost" } },
    track_error: (state) => state,
  });
  try {
    vm.runInContext(fs.readFileSync(path.join(root, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
    Object.assign(context, context.Timeless);
    for (const file of ["src/biz/request.js", "src/list.model.js", "src/pages/pipeline_editor.model.js", "src/pages/automation.model.js"]) {
      vm.runInContext(fs.readFileSync(path.join(root, file), "utf8")
        .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export \{[\s\S]*?\};/gm, "").replace(/^export /gm, ""), context);
    }
    const model = context.AutomationPageViewModel({ client, tip() {}, view: { query: {} } });
    t.after(() => model.methods.destroy());
    return { model, Result: context.Timeless.Result };
  } catch (error) { throw new Error(error.message); }
}

function toggle_instance(model, pipeline) {
  const button = model.methods.pipelineActionButton("toggle", pipeline);
  return new Promise((resolve) => {
    button.onStateChange((state) => {
      if (!state.loading) resolve();
    });
    button.click();
  });
}

test("instance action button disables and reenables execution without changing triggers", { timeout: 3000 }, async (t) => {
  const client = {}, calls = [];
  const { model, Result } = load_model(t, client);
  let instance = { id: "instance-one", flow_id: "flow-one", name: "One", disabled: false };
  client.post = async (url, body) => {
    calls.push({ url, body });
    if (url.endsWith("update_flow_instance")) {
      assert.deepEqual(JSON.parse(JSON.stringify(body)), { id: instance.id, disabled: !instance.disabled });
      instance = { ...instance, disabled: body.disabled };
      return Result.Ok(instance);
    }
    if (url.endsWith("list_flow_instances")) return Result.Ok({ list: [instance] });
    return Result.Ok({ list: [], total: 0 });
  };
  await model.methods.refresh();
  let pipeline = model.state.pipelines.value[0];
  assert.equal(model.methods.pipelineActionButton("trigger", pipeline).state.disabled, false);
  await toggle_instance(model, pipeline);
  pipeline = model.state.pipelines.value[0];
  assert.equal(pipeline.disabled, true);
  assert.equal(model.methods.pipelineActionButton("trigger", pipeline).state.disabled, true);
  // A stale row must also be rejected by the model before issuing a request.
  await model.ui.btn_pipeline_trigger$.bind({ ...pipeline, disabled: false }).click();
  assert.match(model.state.error.value, /已禁用/);
  assert.equal(calls.some(({ url }) => url.endsWith("trigger_flow_instance")), false);
  await toggle_instance(model, pipeline);
  pipeline = model.state.pipelines.value[0];
  assert.equal(pipeline.disabled, false);
  assert.equal(model.methods.pipelineActionButton("trigger", pipeline).state.disabled, false);
});

test("a failed instance state update preserves the saved state and allows retry", { timeout: 3000 }, async (t) => {
  const client = {};
  const { model, Result } = load_model(t, client);
  const instance = { id: "instance-one", name: "One", disabled: true };
  client.post = async (url) => {
    if (url.endsWith("update_flow_instance")) return Result.Err("状态保存失败");
    return Result.Ok({ list: url.endsWith("list_flow_instances") ? [instance] : [], total: 0 });
  };
  await model.methods.refresh();
  const pipeline = model.state.pipelines.value[0];
  await toggle_instance(model, pipeline);
  assert.equal(model.state.pipelines.value[0].disabled, true);
  assert.match(model.state.error.value, /状态保存失败/);
  assert.equal(model.ui.btn_pipeline_toggle$.bind(pipeline).state.loading, false);
  assert.equal(model.methods.pipelineActionButton("trigger", pipeline).state.disabled, true);
});

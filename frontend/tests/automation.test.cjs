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
    return { model, context, Result: context.Timeless.Result };
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


test("schedule forms serialize each mode and preserve legacy cron", (t) => {
  const { context } = load_model(t, {});
  const plain = (value) => JSON.parse(JSON.stringify(value));
  const form = (mode, fields = {}) => ({
    schedule_edited: true, input_cron$: { state: { value: "0 8 * * 1-5" } },
    schedule_form: { mode: { value: mode }, ...Object.fromEntries(Object.entries({ timezone: "UTC", interval: "5", times: "08:00, 12:30", days: "1,2,3,4,5", windows: "6 23:00-01:00 10", outside_interval: "0", ...fields }).map(([key, value]) => [key, { state: { value } }])) },
  });
  assert.equal(context.schedule_form_value({ cron_expr: "@daily" }), undefined);
  assert.deepEqual(plain(context.schedule_form_value(form("interval"))), { mode: "interval", timezone: "UTC", interval: 300 });
  assert.deepEqual(plain(context.schedule_form_value(form("times"))), { mode: "times", timezone: "UTC", days: [1,2,3,4,5], times: ["08:00", "12:30"] });
  assert.deepEqual(plain(context.schedule_form_value(form("windows"))), { mode: "windows", timezone: "UTC", outside_interval: 0, windows: [{ days: [6], start: "23:00", end: "01:00", interval: 600 }] });
  assert.equal(context.schedule_form_value(form("cron")).cron, "0 8 * * 1-5");
  assert.deepEqual(plain(context.schedule_form_value(form("manual"))), { mode: "manual" });
  assert.throws(() => context.schedule_form_value(form("interval", { interval: "0" })), /间隔/);
  assert.throws(() => context.schedule_form_value(form("times", { times: "25:00" })), /HH:mm/);
  assert.throws(() => context.schedule_form_value(form("times", { days: "1,,2" })), /星期/);
  const trigger = context.pipeline_triggers({ id: "instance" }, [{ id: "schedule", instance_id: "instance", cron_expr: "@daily", initial_data: JSON.stringify({ __automation: { schedule: { mode: "interval", interval: 600 } } }) }])[0];
  assert.deepEqual(plain(trigger.schedule), { mode: "interval", interval: 600 });
  assert.equal(context.schedule_label(trigger), "每 10 分钟");
});


test("new timed trigger exposes bound controls for all schedule modes", (t) => {
  const { model, context } = load_model(t, {});
  model.ui.btn_add_cron_trigger$.click();
  const trigger = model.state.edit_triggers.value[0];
  assert.equal(context.schedule_form_value(trigger).interval, 3600);
  trigger.schedule_form.select_mode.setValue("times");
  trigger.schedule_form.times.setValue("09:00, 18:00");
  trigger.schedule_form.days.setValue("1,3,5");
  const policy = JSON.parse(JSON.stringify(context.schedule_form_value(trigger)));
  assert.deepEqual(policy, { mode: "times", timezone: "Asia/Shanghai", days: [1,3,5], times: ["09:00", "18:00"] });
  trigger.schedule_form.select_mode.setValue("manual");
  assert.equal(context.schedule_form_value(trigger).mode, "manual");
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const source = fs.readFileSync(path.join(frontend, "src/pages/automation.model.js"), "utf8");

function setup({ legacy_event = false, schedules = [], reject_save = false, live_channel = null, flows = null, runs = [] } = {}) {
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval: () => 0, clearInterval, URLSearchParams });
  context.window = context;
  context.__store = {};
  vm.runInContext(fs.readFileSync(path.join(frontend, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  for (const name of ["ref", "refarr", "refobj", "combine", "computed"]) context[name] = context.Timeless[name];
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/pipeline_editor.model.js"), "utf8").replace(/export \{[^}]+\};\s*$/, ""), context);
  // Only the HTTP transport is replaced. Input,
  // checkbox, array, dirty-state, and selection behavior use the bundled runtime.
  const flow = {
    id: "flow-test", name: "test", trigger_type: legacy_event ? "Event" : "Manual", event_key: legacy_event ? "old.event" : "",
    definition: JSON.stringify({ start_node: "start", nodes: {
      start: { id: "start", name: "开始", type: "StartNode", next_node_ids: ["end"] },
      end: { id: "end", name: "结束", type: "EndNode" },
    } }),
  };
  const calls = [];
  context.request = { post: (url, body) => ({ url, body }) };
  const TestRequestCore = class {
    constructor(fn) { this.fn = fn; }
    async run(params) {
      const { url, body } = this.fn(params);
      calls.push({ url, body: JSON.parse(JSON.stringify(body)) });
      if (url.endsWith("list_runs")) return { data: { list: runs, total: runs.length } };
      if (url.endsWith("get_run")) return { data: runs.find((run) => run.id === body.id) };
      if (url.endsWith("resume_run")) { runs = runs.map((run) => run.id === body.id ? { ...run, status: "RUNNING" } : run); return { data: runs.find((run) => run.id === body.id) }; }
      if (url.endsWith("cancel_run")) { runs = runs.map((run) => run.id === body.id ? { ...run, status: "CANCELLED" } : run); return { data: { id: body.id } }; }
      if (url.endsWith("list_flows")) return { data: { list: flows || [flow] } };
      if (url.endsWith("get_flow")) return { data: flow };
      if (url.endsWith("list_flow_nodes")) return { data: { nodes: [] } };
      if (url.endsWith("list_schedules")) {
        const filtered = schedules.filter((item) => !body.flow_id || item.flow_id === body.flow_id);
        return { data: { list: filtered.slice((body.page - 1) * body.page_size, body.page * body.page_size), total: filtered.length } };
      }
      if (url.endsWith("update_flow")) {
        if (reject_save) return { error: new Error("invalid Cron") };
        schedules = body.triggers.map((trigger, index) => ({
          ...trigger, id: trigger.id || `new-${index}`, flow_id: flow.id,
          initial_data: JSON.stringify({ ...trigger.initial_data, __automation: { type: trigger.type, event_key: trigger.event_key, start_node: "start" } }),
        }));
        flow.trigger_type = "Manual";
        flow.event_key = "";
        return { data: flow };
      }
      throw new Error(`unexpected request: ${url}`);
    }
  };
  context.Timeless = { ...context.Timeless, kit: { ...context.Timeless.kit, RequestCore: TestRequestCore,
    ...(live_channel ? { ChannelCore: class { constructor() { return live_channel; } } } : {}),
  } };
  const executable = source.replace(/^import .*;\n/gm, "");
  vm.runInContext(executable.slice(0, executable.lastIndexOf("\nexport {")), context);
  const model = context.AutomationPageViewModel({ client: {}, view: { query: { id: flow.id } } }, { mode: "edit" });
  return { model, calls, context, flow };
}

test("automation pagination shows bounded pages and clamps the last page after a reload", async () => {
  const flows = Array.from({ length: 101 }, (_, index) => ({ id: `flow-${index}`, name: `任务 ${index}` }));
  const { model } = setup({ flows });
  await model.methods.refresh();
  assert.equal(model.state.page_size.value, 50);
  assert.equal(model.state.page_count.value, 3);
  assert.equal(model.state.paged_pipelines.value.length, 50);
  assert.equal(model.state.range_text.value, "第 1-50 条，共 101 条");
  model.methods.changePage(2);
  assert.equal(model.state.paged_pipelines.value[0].id, "flow-50");
  assert.equal(model.state.range_text.value, "第 51-100 条，共 101 条");
  model.methods.changePage(99);
  assert.equal(model.state.page.value, 3);
  assert.equal(model.state.paged_pipelines.value.length, 1);
  assert.equal(model.state.paged_pipelines.value[0].id, "flow-100");
  flows.splice(100);
  await model.methods.refresh();
  assert.equal(model.state.page.value, 2);
  assert.equal(model.state.range_text.value, "第 51-100 条，共 100 条");
  assert.equal(model.state.paged_pipelines.value.length, 50);
  model.methods.changePage(-1);
  assert.equal(model.state.page.value, 1);
  model.methods.destroy();
});

test("automation search and reset return to page one and paginate the matching names", async () => {
  const flows = Array.from({ length: 120 }, (_, index) => ({
    id: `flow-${index}`, name: index < 60 ? `下载 ${index}` : `归档 ${index}`,
  }));
  const { model } = setup({ flows });
  await model.methods.refresh();
  model.methods.changePage(3);
  model.ui.input_keyword$.setValue("  下载  ");
  model.methods.search();
  assert.equal(model.state.page.value, 1);
  assert.equal(model.state.total.value, 60);
  assert.equal(model.state.page_count.value, 2);
  model.methods.changePage(2);
  assert.equal(model.state.paged_pipelines.value.length, 10);
  assert.equal(model.state.range_text.value, "第 51-60 条，共 60 条");
  assert.ok(model.state.paged_pipelines.value.every((flow) => flow.name.includes("下载")));
  model.methods.search();
  assert.equal(model.state.page.value, 1, "resubmitting the same search resets pagination");
  model.ui.input_keyword$.setValue("不存在");
  model.methods.search();
  assert.equal(model.state.paged_pipelines.value.length, 0);
  assert.equal(model.state.page_count.value, 1);
  assert.equal(model.state.range_text.value, "共 0 条");
  await model.ui.btn_reset$.click();
  assert.equal(model.state.page.value, 1);
  assert.equal(model.state.total.value, 120);
  assert.equal(model.state.range_text.value, "第 1-50 条，共 120 条");
  model.methods.destroy();
});

test("a start node saves simultaneous cron and event rules with edited inputs", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  model.ui.btn_add_cron_trigger$.click();
  model.ui.btn_add_event_trigger$.click();
  const [cron, event] = model.state.edit_triggers.value;
  cron.input_name$.setValue("daily download");
  cron.input_cron$.setValue("0 2 * * *");
  cron.checkbox_enabled$.toggle();
  event.input_event_key$.setValue("video.created");
  event.input_data$.setValue('{"url":"https://example.com"}');
  model.methods.selectEditNode("end");
  model.methods.selectEditNode("start");
  assert.equal(model.state.edit_triggers.value.length, 2, "switching nodes lost trigger drafts");
  assert.equal(model.state.dirty.value, true);
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body;
  assert.deepEqual(saved.triggers.map((trigger) => trigger.type), ["Cron", "Event"]);
  assert.equal(saved.triggers[0].name, "daily download");
  assert.equal(saved.triggers[0].cron_expr, "0 2 * * *");
  assert.equal(saved.triggers[0].enabled, false);
  assert.equal(saved.triggers[1].event_key, "video.created");
  assert.deepEqual(saved.triggers[1].initial_data, { url: "https://example.com" });
  assert.equal(model.state.dirty.value, false);
});

test("invalid input JSON and server failures retain unsaved triggers", async () => {
  const { model, calls } = setup({ reject_save: true });
  await model.methods.refresh();
  model.ui.btn_add_cron_trigger$.click();
  const trigger = model.state.edit_triggers.value[0];
  trigger.input_data$.setValue("[]");
  await model.methods.saveFlow();
  assert.match(model.state.error.value, /JSON 对象/);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
  trigger.input_data$.setValue("{}");
  await model.methods.saveFlow();
  assert.match(model.state.error.value, /invalid Cron/);
  assert.equal(model.state.dirty.value, true);
  assert.equal(model.state.edit_triggers.value.length, 1);
});

test("removing the last trigger explicitly saves an empty list", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  model.ui.btn_add_event_trigger$.click();
  const trigger = model.state.edit_triggers.value[0];
  model.ui.btn_remove_trigger$.bind(trigger).click();
  await model.methods.saveFlow();
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow")).body.triggers, []);
});

test("legacy event keys are retained once and are not resurrected after removal", async () => {
  const { model, calls } = setup({ legacy_event: true });
  await model.methods.refresh();
  assert.equal(model.state.edit_triggers.value[0].event_key, "old.event");
  await model.methods.saveFlow();
  assert.equal(model.state.edit_triggers.value.length, 1);
  assert.equal(model.state.edit_triggers.value[0].event_key, "old.event");
  model.ui.btn_remove_trigger$.bind(model.state.edit_triggers.value[0]).click();
  await model.methods.saveFlow();
  assert.equal(model.state.edit_triggers.value.length, 0);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 2);
});

test("all existing schedules load before replacement, including pages after 200", async () => {
  const schedules = Array.from({ length: 201 }, (_, index) => ({
    id: `rule-${index}`, flow_id: "flow-test", name: `rule ${index}`, cron_expr: "@daily", enabled: true,
    initial_data: JSON.stringify({ url: "original", __automation: { type: "Cron" } }), timeout_sec: 120,
  }));
  const { model, calls } = setup({ schedules });
  await model.methods.refresh();
  assert.equal(model.state.edit_triggers.value.length, 201);
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body;
  assert.equal(saved.triggers.length, 201);
  assert.equal(saved.triggers[200].id, "rule-200");
  assert.equal(saved.triggers[200].timeout_sec, 120);
  assert.deepEqual(saved.triggers[200].initial_data, { url: "original" });
});

function test_editor_channel() {
  const listeners = [], states = [];
  return {
    sent: [],
    onMessage(fn) { listeners.push(fn); return () => {}; },
    onStateChange(fn) { states.push(fn); return () => {}; },
    async connect() { states.forEach((fn) => fn({ connected: true })); return { data: true }; },
    async send(raw) { this.sent.push(JSON.parse(raw)); return { data: true }; },
    destroy() {},
    async receive(message) {
      listeners.forEach((fn) => fn({ flow_id: 'flow-test', expires_at: Date.now() + 4000, request_id: 'req', ...message }));
      await new Promise(setImmediate);
      return this.sent.at(-1);
    },
  };
}

test('MCP reads the real editor including staged positions and applies nodes and triggers to reactive state', async () => {
  const channel = test_editor_channel();
  const { model } = setup({ live_channel: channel });
  await model.methods.ready();
  assert.equal(channel.sent[0].type, 'pipeline_editor_register');
  model.methods.stageEditNodePosition('end', { x: 700, y: 80 });
  let reply = await channel.receive({ type: 'pipeline_editor_read' });
  assert.equal(reply.snapshot.draft.nodes.find((node) => node.id === 'end').position.x, 700);
  const draft = JSON.parse(JSON.stringify(reply.snapshot.draft));
  draft.nodes.find((node) => node.id === 'end').name = '对话修改结束节点';
  draft.triggers = [{ id: '', type: 'Event', name: '收到事件', event_key: 'message.received', initial_data: {} }];
  reply = await channel.receive({ type: 'pipeline_editor_apply', revision: reply.snapshot.revision, draft });
  assert.equal(reply.error, undefined);
  assert.equal(model.state.edit_nodes.value.find((node) => node.id === 'end').name, '对话修改结束节点');
  assert.equal(model.state.edit_triggers.value[0].event_key, 'message.received');
  assert.equal(model.state.dirty.value, true);
  assert.match(model.state.notice.value, /MCP 已更新/);
  model.methods.destroy();
});

test('MCP retains invalid unsaved trigger input when only the graph changes', async () => {
  const channel = test_editor_channel();
  const { model } = setup({ live_channel: channel });
  await model.methods.ready();
  model.ui.btn_add_event_trigger$.click();
  model.state.edit_triggers.value[0].input_data$.setValue('{unfinished');
  const read = await channel.receive({ type: 'pipeline_editor_read' });
  assert.equal(read.snapshot.draft.triggers, null);
  assert.equal(read.snapshot.trigger_forms[0].data_text, '{unfinished');
  const draft = JSON.parse(JSON.stringify(read.snapshot.draft));
  const reply = await channel.receive({ type: 'pipeline_editor_apply', revision: read.snapshot.revision, draft });
  assert.equal(reply.error, undefined);
  assert.equal(model.state.edit_triggers.value[0].data_text, '{unfinished');
  model.methods.destroy();
});


test("node execution policy survives applying configuration and saving the canvas", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  model.methods.selectEditNode("end");
  model.ui.input_edit_node_policy$.setValue(JSON.stringify({ retry: { max_attempts: 3, delay_ms: 100, backoff_multiplier: 2 }, error_next_node: "start", input_schema: [], output_schema: [] }));
  model.methods.applySelectedNodeConfig();
  assert.equal(model.state.edit_nodes.value.find((node) => node.id === "end").retry.max_attempts, 3);
  await model.methods.saveFlow();
  const node = calls.find((call) => call.url.endsWith("update_flow")).body.nodes.find((node) => node.id === "end");
  assert.equal(node.retry.max_attempts, 3);
  assert.equal(node.error_next_node, "start");
  model.methods.destroy();
});

test("Webhook active state and workflow timeout are serialized with trigger rules", async () => {
  const { model, calls } = setup(); await model.methods.refresh();
  model.ui.btn_add_webhook_trigger$.click();
  const trigger = model.state.edit_triggers.value[0];
  trigger.input_timeout$.setValue("120"); trigger.checkbox_active$.toggle();
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body.triggers[0];
  assert.equal(saved.type, "Webhook"); assert.equal(saved.active, false); assert.equal(saved.timeout_sec, 120);
  model.methods.destroy();
});

test("manual confirmation sends typed JSON and cancellation refreshes persisted status", async () => {
  const run = { id: "run-confirm", flow_id: "flow-test", status: "WAITING", current_node: "confirm", node_logs: "[]" };
  const { model, calls } = setup({ runs: [run] });
  await model.methods.refresh(); await model.methods.refreshRuns();
  model.ui.input_manual_response$.setValue("[]"); await model.methods.actOnRun("resume");
  assert.equal(calls.filter((call) => call.url.endsWith("resume_run")).length, 0);
  model.ui.input_manual_response$.setValue('{"approved":true}'); await model.methods.actOnRun("resume");
  const submitted = calls.find((call) => call.url.endsWith("resume_run")).body;
  assert.deepEqual(submitted, { id: run.id, node_id: "confirm", input: { approved: true } });
  assert.equal(model.state.selected_run.value.status, "RUNNING");
  await model.methods.actOnRun("cancel"); assert.equal(model.state.selected_run.value.status, "CANCELLED");
  model.methods.destroy();
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const source = fs.readFileSync(path.join(frontend, "src/pages/automation.model.js"), "utf8");

function setup({ legacy_event = false, schedules = [], reject_save = false, reject_trigger = false, trigger_wait = null, live_channel = null, flows = null, runs = [], run_detail_handler = null, node_logs_handler = null, clear_runs_handler = null, mode = "edit" } = {}) {
  const timers = new Map();
  const context = vm.createContext({ setTimeout, clearTimeout, setInterval(fn, delay) {
    const id = timers.size + 1;
    timers.set(id, { fn, delay });
    return id;
  }, clearInterval(id) { timers.delete(id); }, URLSearchParams });
  context.window = context;
  context.__store = {};
  vm.runInContext(fs.readFileSync(path.join(frontend, "public/timeless/0.33.1/timeless.umd.min.js"), "utf8"), context);
  for (const name of ["ref", "refarr", "refobj", "combine", "computed"]) context[name] = context.Timeless[name];
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/utils.js"), "utf8").replace(/^export /gm, ""), context);
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
  vm.runInContext(
    fs.readFileSync(path.join(frontend, "src/biz/request.js"), "utf8").replace(/^export /gm, ""),
    context,
  );
  const TestRequestCore = class {
    constructor(fn) { this.fn = fn; }
    async run(params) {
      const { url, body } = this.fn(params);
      calls.push({ url, body: JSON.parse(JSON.stringify(body)) });
      if (url.endsWith("list_runs")) return { data: { list: body.summary ? runs.map(({ id, created_at, status }) => ({ id, created_at, status })) : runs, total: runs.length } };
      if (url.endsWith("clear_runs")) {
        if (clear_runs_handler) return clear_runs_handler(body);
        runs = runs.filter((run) => run.flow_id !== body.flow_id || !["COMPLETED", "FAILED", "CANCELLED"].includes(run.status));
        return { data: {} };
      }
      if (url.endsWith("get_run_node_states")) {
        if (run_detail_handler) return run_detail_handler(body.id, body);
        const run = runs.find((run) => run.id === body.id);
        if (!run) return { data: {} };
        let logs = [], definition = {}, attempts = {};
        try { logs = JSON.parse(run.node_logs || "[]") || []; } catch { /* Legacy invalid logs are empty. */ }
        try { definition = JSON.parse(run.definition || "{}"); } catch { /* Legacy definitions may be missing. */ }
        try { attempts = JSON.parse(run.node_attempts || "{}"); } catch { /* Legacy attempts may be missing. */ }
        const states = Object.fromEntries(Object.keys(definition.nodes || {}).map((id) => [id, "PENDING"]));
        for (const [id, attempt] of Object.entries(attempts)) if (attempt > 0) states[id] = "UNKNOWN";
        for (const entry of logs) {
          if (entry.flow_id && entry.flow_id !== run.flow_id && entry.flow_id !== definition.id) continue;
          states[entry.node_id] = entry.outcome === "waiting" ? "WAITING_FOR_USER" : String(entry.outcome || "COMPLETED").toUpperCase();
        }
        if (run.current_node && run.status !== "COMPLETED") states[run.current_node] = ({ WAITING: "WAITING_FOR_USER", QUEUED: "PENDING" })[run.status] || run.status;
        return { data: states };
      }
      if (url.endsWith("get_run_node_logs")) {
        if (node_logs_handler) return node_logs_handler(body);
        const run = runs.find((run) => run.id === body.id);
        let logs = [], definition = {};
        try { logs = JSON.parse(run?.node_logs || "[]") || []; } catch { /* Legacy invalid logs are empty. */ }
        try { definition = JSON.parse(run?.definition || "{}"); } catch { /* Legacy definitions may be missing. */ }
        return { data: { list: logs.filter((entry) => entry.node_id === body.node_id && (!entry.flow_id || entry.flow_id === run.flow_id || entry.flow_id === definition.id)) } };
      }
      if (url.endsWith("resume_run")) { runs = runs.map((run) => run.id === body.id ? { ...run, status: "RUNNING" } : run); return { data: runs.find((run) => run.id === body.id) }; }
      if (url.endsWith("cancel_run")) { runs = runs.map((run) => run.id === body.id ? { ...run, status: "CANCELLED" } : run); return { data: { id: body.id } }; }
      if (url.endsWith("list_flows")) return { data: { list: flows || [flow] } };
      if (url.endsWith("trigger_flow")) {
        if (trigger_wait) await trigger_wait;
        if (reject_trigger) return { error: new Error("trigger rejected") };
        const run = { id: `run-${body.id}`, flow_id: body.id, status: "RUNNING", current_node: "start", node_logs: "[]" };
        runs = [run, ...runs.filter((item) => item.id !== run.id)];
        return { data: run };
      }
      if (url.endsWith("get_flow")) return { data: flow };
      if (url.endsWith("list_flow_nodes")) return { data: { nodes: [] } };
      if (url.endsWith("import_flow")) {
        const definition = JSON.parse(body.definition);
        const imported = { ...flow, id: "flow-imported", name: definition.name, definition: body.definition };
        flows = [...(flows || [flow]), imported];
        return { data: imported };
      }
      if (url.endsWith("list_schedules")) {
        const filtered = schedules.filter((item) => !body.flow_id || item.flow_id === body.flow_id);
        return { data: { list: filtered.slice((body.page - 1) * body.page_size, body.page * body.page_size), total: filtered.length } };
      }
      if (url.endsWith("update_flow")) {
        if (reject_save) return { error: new Error("invalid Cron") };
        if (body.name !== undefined) flow.name = body.name;
        const definition = JSON.parse(flow.definition);
        if (body.name !== undefined) definition.name = body.name;
        if (body.start_node_id !== undefined) definition.start_node = body.start_node_id;
        if (body.context_schema !== undefined) definition.context_schema = body.context_schema;
        if (body.nodes !== undefined) definition.nodes = Object.fromEntries(body.nodes.map((node) => [node.id, { ...node, next_node_ids: node.next_ids }]));
        flow.definition = JSON.stringify(definition);
        if (body.node_configs !== undefined) flow.node_configs = JSON.stringify(body.node_configs);
        if (body.triggers !== undefined) schedules = body.triggers.map((trigger, index) => ({
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
  const executable = source.replace(/^import[\s\S]*?;\n/gm, "");
  vm.runInContext(executable.slice(0, executable.lastIndexOf("\nexport {")), context);
  const tips = [];
  const model = context.AutomationPageViewModel({ client: {}, tip(message) { tips.push(message); }, view: { query: { id: flow.id } } }, { mode });
  return { model, calls, context, flow, timers, tips };
}

function load_canvas_components(context) {
  for (const name of ["View", "For", "Show", "Fragment", "SVG"]) context[name] = context.Timeless[name];
  context.vm = context.Timeless.vm;
  context.ui = context.Timeless.ui;
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/dmui.js"), "utf8")
    .replace(/^export \{[^}]+\};\n/gm, "").replace(/^export /gm, ""), context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.components.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export /gm, ""), context);
}

function find_view(node, name) {
  if (node.state?.attributes?.n === name) return node;
  for (const child of node.children || []) {
    if (!child || typeof child !== "object") continue;
    const found = find_view(child, name);
    if (found) return found;
  }
  return null;
}

test("editing the pipeline title marks the draft dirty and saves the trimmed name", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  assert.equal(model.ui.input_edit_name$.state.value, "test");
  assert.equal(model.state.dirty.value, false);
  model.ui.input_edit_name$.setValue("  下载任务  ");
  assert.equal(model.state.dirty.value, true);
  assert.equal(model.state.selected_pipeline.value.name, "test");
  await model.methods.saveFlow();
  assert.equal(calls.find((call) => call.url.endsWith("update_flow")).body.name, "下载任务");
  assert.equal(model.state.selected_pipeline.value.name, "下载任务");
  assert.equal(model.ui.input_edit_name$.state.value, "下载任务");
  assert.equal(model.state.dirty.value, false);
  model.methods.destroy();
});

test("editor JSON import replaces the draft and saves to the existing flow without copying runtime configuration", async () => {
  const { model, calls, flow } = setup();
  flow.node_configs = JSON.stringify({ validate: { username: "creator", token: "keep-existing-secret", method: "POST" } });
  await model.methods.refresh();
  const definition = JSON.parse(fs.readFileSync(path.join(frontend, "../docs/feature/pipelines/01_creator_video_sync.json"), "utf8"));
  definition.context_schema = [{ key: "tenant", type: "string", required: false }];
  definition.nodes.validate.retry = { max_attempts: 2, delay_ms: 10 };
  definition.nodes.validate.output_schema = [{ key: "creator", type: "object" }];
  definition.nodes.validate.error_next_node = "finish_summary";
  definition.triggers[0].id = "foreign-trigger-id";
  model.methods.openImportDialog();
  model.ui.input_import_json$.setValue(JSON.stringify(definition), { silence: true });
  await model.methods.submitImport();
  assert.equal(model.state.selected_flow_id.value, flow.id);
  assert.equal(model.state.selected_pipeline.value.id, flow.id);
  assert.equal(model.state.selected_pipeline.value.node_configs, flow.node_configs);
  assert.equal(model.ui.input_edit_name$.state.value, definition.name);
  assert.equal(model.state.dirty.value, true);
  assert.equal(model.state.import_open.value, false);
  assert.equal(model.state.import_submitting.value, false);
  assert.equal(calls.filter((call) => /(?:import_flow|update_flow)$/.test(call.url)).length, 0, "import must only change the draft");
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body;
  assert.equal(saved.id, flow.id);
  assert.deepEqual(saved.context_schema, definition.context_schema);
  assert.equal(saved.triggers.length, 1);
  assert.equal(saved.triggers[0].id, undefined);
  assert.equal(Object.hasOwn(saved, "node_configs"), false);
  const page = saved.nodes.find((node) => node.id === "fetch_page");
  assert.equal(page.config.arguments.method, "{{output.validate.creator.method}}");
  const list = page.config.workflow.nodes.fetch_list.config;
  assert.equal(list.arguments.method, "{{input.method}}");
  assert.equal(list.workflow.nodes.fetch_list.config.method, "{{input.method}}");
  assert.deepEqual(list.workflow.nodes.fetch_list.config.body, { username: "{{input.username}}", next_marker: "{{input.next_marker}}" });
  const validate = saved.nodes.find((node) => node.id === "validate");
  assert.deepEqual(validate.retry, definition.nodes.validate.retry);
  assert.deepEqual(validate.output_schema, definition.nodes.validate.output_schema);
  assert.equal(validate.error_next_node, "finish_summary");
  assert.equal(model.state.edit_nodes.value.find((node) => node.id === "validate").retry.max_attempts, 2, "reload must retain execution policy");
  assert.equal(model.state.dirty.value, false);
  model.methods.destroy();
});

test("invalid editor imports preserve the existing graph, pending inspector text and node positions", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  model.ui.input_edit_node_config$.setValue('{"keep":"unapplied draft"}');
  model.methods.stageEditNodePosition("start", { x: 123, y: 456 });
  const original = JSON.stringify(model.state.edit_nodes.value);
  const definition = { name: "replacement", start_node: "start", nodes: { start: { type: "StartNode", config: {} } } };
  for (const raw of ["{", "[]", JSON.stringify({ ...definition, name: "" }), JSON.stringify({ ...definition, start_node: "missing" }), JSON.stringify({ ...definition, start_node: undefined }), JSON.stringify({ ...definition, context_schema: {} }), JSON.stringify({ ...definition, triggers: {} }), JSON.stringify({ ...definition, nodes: { start: null } })]) {
    model.methods.openImportDialog();
    model.ui.input_import_json$.setValue(raw);
    await model.methods.submitImport();
    assert.ok(model.state.import_error.value, raw);
    assert.equal(model.state.import_open.value, true);
    assert.equal(model.state.import_submitting.value, false);
    assert.equal(JSON.stringify(model.state.edit_nodes.value), original);
    assert.equal(model.state.edit_node_config.value, '{"keep":"unapplied draft"}');
    assert.equal(model.ui.input_edit_name$.state.value, "test");
  }
  await model.methods.saveFlow();
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow")).body.nodes.find((node) => node.id === "start").position, { x: 123, y: 456 });
  assert.equal(calls.filter((call) => call.url.endsWith("import_flow")).length, 0);
  model.methods.destroy();
});

test("editor import preserves unsaved triggers when omitted and clears them only for an explicit empty array", async () => {
  const schedule = { id: "existing-trigger", flow_id: "flow-test", name: "saved trigger", enabled: false, cron_expr: "0 9 * * *" };
  const { model, calls } = setup({ schedules: [schedule] });
  await model.methods.refresh();
  model.state.edit_triggers.value[0].input_name$.setValue("unsaved trigger name");
  const definition = { name: "replacement", start_node: "start", nodes: { start: { type: "StartNode", config: {} } } };
  model.ui.input_import_json$.setValue(JSON.stringify(definition));
  await model.methods.submitImport();
  assert.equal(model.state.edit_triggers.value[0].id, "existing-trigger");
  assert.equal(model.state.edit_triggers.value[0].name, "unsaved trigger name");
  model.ui.input_import_json$.setValue(JSON.stringify({ ...definition, triggers: [] }));
  await model.methods.submitImport();
  assert.equal(model.state.edit_triggers.value.length, 0);
  await model.methods.saveFlow();
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow")).body.triggers, []);
  model.methods.destroy();
});

test("list import still creates a new pipeline through the import API", async () => {
  const { model, calls } = setup({ mode: "list" });
  model.methods.openImportDialog();
  model.ui.input_import_json$.setValue(JSON.stringify({ name: "new flow", start_node: "entry", nodes: { entry: { type: "StartNode" } } }));
  await model.methods.submitImport();
  assert.equal(calls.filter((call) => call.url.endsWith("import_flow")).length, 1);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
  assert.equal(model.state.import_open.value, false);
  model.methods.destroy();
});

test("the editor renders an import action and a shared draft import dialog", async () => {
  const { model, context } = setup();
  await model.methods.refresh();
  load_canvas_components(context);
  const toolbar = context.AutomationWorkspaceToolbar({ store: model, mode: "edit" });
  assert.ok(find_view(toolbar, "automation-edit-import-json"));
  await model.ui.btn_import_pipeline$.click();
  assert.equal(model.state.import_open.value, true);
  const dialog = context.AutomationImportPipelineDialog({ store: model });
  assert.ok(find_view(dialog, "automation-import-json"));
  assert.ok(find_view(dialog, "automation-import-submit"));
  assert.ok(JSON.stringify(dialog).includes("导入到草稿"));
  model.methods.destroy();
});

test("an empty title blocks saving and a failed save retains the entered title", async () => {
  const { model, calls } = setup({ reject_save: true });
  await model.methods.refresh();
  model.ui.input_edit_name$.setValue("   ");
  await model.methods.saveFlow();
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
  assert.match(model.state.error.value, /请输入 Pipeline 标题/);
  model.ui.input_edit_name$.setValue("新标题");
  await model.methods.saveFlow();
  assert.equal(model.ui.input_edit_name$.state.value, "新标题");
  assert.equal(model.state.selected_pipeline.value.name, "test");
  assert.equal(model.state.dirty.value, true);
  model.methods.destroy();
});

test("closing the node inspector retains unapplied text; selecting a node reopens it", async () => {
  const { model } = setup();
  await model.methods.refresh();
  assert.equal(model.state.node_inspector_open.value, true);
  model.ui.input_edit_node_config$.setValue('{"draft":true}');
  model.methods.closeNodeInspector();
  assert.equal(model.state.node_inspector_open.value, false);
  assert.equal(model.state.edit_node_config.value, '{"draft":true}');
  model.methods.selectEditNode("end");
  assert.equal(model.state.node_inspector_open.value, true);
  assert.equal(model.state.selected_edit_node_id.value, "end");
  model.methods.destroy();
});

test("run history loads on demand without a polling timer", async () => {
  for (const mode of ["edit", "detail"]) {
    const { model, calls, timers } = setup({ mode });
    await model.methods.ready();
    assert.equal(calls.filter((call) => call.url.endsWith("list_runs")).length, mode === "detail" ? 1 : 0);
    assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_states") || call.url.endsWith("get_run_node_logs")).length, 0);
    const polling = [...timers.values()].filter((timer) => timer.delay === 3000);
    assert.equal(polling.length, 0);
    if (mode === "detail") {
      await model.methods.refreshRuns();
      assert.equal(calls.filter((call) => call.url.endsWith("list_runs")).length, 2);
    }
    model.methods.destroy();
    assert.equal([...timers.values()].filter((timer) => timer.delay === 3000).length, 0);
  }
});

test("automation pagination shows bounded pages and clamps the last page after a reload", async () => {
  const flows = Array.from({ length: 101 }, (_, index) => ({ id: `flow-${index}`, name: `任务 ${index}` }));
  const { model } = setup({ flows });
  await model.methods.refresh();
  assert.equal(model.state.page_size.value, 50);
  assert.equal(model.state.page_count.value, 3);
  assert.equal(model.state.paged_pipelines.value.length, 50);
  assert.equal(model.state.range_text.value, "共 101 条");
  model.methods.changePage(2);
  assert.equal(model.state.paged_pipelines.value[0].id, "flow-50");
  assert.equal(model.state.range_text.value, "共 101 条");
  model.methods.changePage(99);
  assert.equal(model.state.page.value, 3);
  assert.equal(model.state.paged_pipelines.value.length, 1);
  assert.equal(model.state.paged_pipelines.value[0].id, "flow-100");
  flows.splice(100);
  await model.methods.refresh();
  assert.equal(model.state.page.value, 2);
  assert.equal(model.state.range_text.value, "共 100 条");
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
  assert.equal(model.state.range_text.value, "共 60 条");
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
  assert.equal(model.state.range_text.value, "共 120 条");
  model.methods.destroy();
});

test("the automation list renders a trigger button in its actions column and mounts the run dialog", async () => {
  const { model, context } = setup({ mode: "list" });
  await model.methods.refresh();
  context.Table = (props) => props;
  context.View = (props, children) => ({ props, children });
  context.Button = (props, children) => ({ props, children });
  context.DropdownMenu = (props, children) => ({ props, children });
  context.Timeless.Icon = (props) => ({ props });
  context.AutomationPipelineTriggerBadges = () => ({ component: "badges" });
  context.AutomationPipelineLink = () => ({ component: "pipeline-link" });
  context.AutomationRunPipelineDialog = () => ({ component: "run-dialog" });
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export default .*;$/gm, ""), context);
  const table = context.AutomationPipelineList({ store: model });
  const pipeline = model.state.pipelines.value[0];
  const trigger_cell = table.columns.find((column) => column.name === "trigger").render(pipeline);
  assert.equal(trigger_cell.length, 1);
  assert.equal(trigger_cell[0].component, "badges");
  const cell = table.columns.find((column) => column.name === "actions").render(pipeline);
  const action = cell.find((item) => item.props?.attributes?.n === `automation-pipeline-trigger-${pipeline.id}`);
  assert.ok(action);
  assert.equal(action.children[0], "触发");
  assert.equal(action.props.attributes["aria-label"], "触发 test");
  assert.equal(cell.some((item) => item.props?.attributes?.n === `automation-pipeline-config-${pipeline.id}`), false);
  const config_column = table.columns.find((column) => column.name === "config");
  assert.equal(config_column.render(pipeline).length, 0, "no declared fields should leave the cell empty");
  pipeline.nodes[0].config_schema = [
    { key: "username", type: "string" }, { key: "count", type: "integer" }, { key: "enabled", type: "boolean" },
  ];
  const config_cell = config_column.render(pipeline);
  assert.equal(config_cell.length, 1, "unfilled configuration should show only the button");
  const config_button = config_cell[0].children.find((item) => item.props?.attributes?.n === `automation-pipeline-config-${pipeline.id}`);
  assert.ok(config_button);
  assert.equal(config_button.children[0], "配置");
  const config_menu = config_cell[0].children.find((item) => item.props?.attributes?.n === `automation-pipeline-config-menu-${pipeline.id}`);
  assert.deepEqual(Array.from(config_menu.props.store.state.items, (item) => item.label), ["导入 JSON", "导出 JSON"]);
  pipeline.node_configs = { [pipeline.nodes[0].id]: { count: 0, enabled: false } };
  const configured_cell = config_column.render(pipeline);
  assert.equal(configured_cell.length, 2);
  const configured_content = JSON.stringify(configured_cell[0]);
  assert.ok(configured_content.includes("count: 0"));
  assert.ok(configured_content.includes("enabled: false"));
  assert.ok(!configured_content.includes("username:"));
  assert.ok(!configured_content.includes("未填写"));
  assert.ok(!configured_content.includes("暂无配置"));
  context.AutomationPageViewModel = () => model;
  for (const name of ["AutomationListPage", "AutomationCreatePipelineDialog", "AutomationImportPipelineDialog", "AutomationDeletePipelineDialog", "AutomationConfigDialog"]) {
    context[name] = () => ({ component: name });
  }
  const page = context.AutomationPageView({});
  assert.ok(page.children.some((item) => item.component === "run-dialog"));
  model.methods.destroy();
});

test("triggering a parameterless list row executes that row's pipeline", async () => {
  const { model, calls, tips } = setup({ mode: "list", flows: [{ id: "row-target", name: "目标任务" }] });
  await model.methods.refresh();
  const pipeline = model.state.pipelines.value[0];
  await model.ui.btn_pipeline_trigger$.bind(pipeline).click();
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow")).body, { id: "row-target", initial_data: {} });
  assert.equal(model.state.run_open.value, false);
  assert.match(tips.at(-1).msg[0], /已触发执行/);
  model.methods.destroy();
});

test("a list row collects its own required parameters before manual execution", async () => {
  const flows = [
    { id: "first", context_schema: [{ key: "other", type: "string", required: true }] },
    { id: "second", context_schema: [{ key: "username", type: "string", required: true }] },
  ];
  const { model, calls } = setup({ mode: "list", flows });
  await model.methods.refresh();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  assert.equal(model.state.run_params.value[0].key, "other");
  await model.ui.btn_run_cancel$.click();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[1]).click();
  assert.equal(model.state.run_open.value, true);
  assert.equal(model.state.run_params.value[0].key, "username");
  await model.methods.submitRun();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow")).length, 0);
  assert.match(model.state.run_error.value, /username/);
  model.state.run_params.value[0].input$.setValue("creator@finder");
  await model.methods.submitRun();
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow")).body,
    { id: "second", initial_data: { username: "creator@finder" } });
  assert.equal(model.state.run_open.value, false);
  model.methods.destroy();
});

test("manual execution reads current control values even when change callbacks were silenced", async () => {
  const { model, calls } = setup({ mode: "list", flows: [{ id: "target", context_schema: [
    { key: "username", type: "string", required: true },
    { key: "page", type: "number", required: true },
    { key: "enabled", type: "boolean", required: true },
  ] }] });
  await model.methods.refresh();
  model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  const [username, page, enabled] = model.state.run_params.value;
  username.input$.setValue("creator@finder", { silence: true });
  page.input$.setValue("0", { silence: true });
  enabled.checkbox$.setValue(false, { silence: true });
  await model.methods.submitRun();
  assert.equal(model.state.run_error.value, "");
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow"))?.body,
    { id: "target", initial_data: { username: "creator@finder", page: 0, enabled: false } });
  model.methods.destroy();
});

test("blank required numeric input is rejected before conversion, while zero is accepted", async () => {
  const { model, calls } = setup({ mode: "list", flows: [{ id: "target", context_schema: [
    { key: "page", type: "number", required: true },
  ] }] });
  await model.methods.refresh();
  model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  const page = model.state.run_params.value[0];
  for (const raw of ["", "   "]) {
    page.input$.setValue(raw);
    await model.methods.submitRun();
    assert.match(model.state.run_error.value, /必填参数：page/);
    assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow")).length, 0);
  }
  page.input$.handleChange({ target: { value: "0" } });
  await model.methods.submitRun();
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow"))?.body,
    { id: "target", initial_data: { page: 0 } });
  model.methods.destroy();
});

test("the rendered run dialog submits input events without requiring blur", async () => {
  const { model, calls, context } = setup({ mode: "list", flows: [{ id: "target", context_schema: [
    { key: "username", type: "string", required: true },
  ] }] });
  await model.methods.refresh();
  model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  load_canvas_components(context);
  const dialog = context.AutomationRunPipelineDialog({ store: model });
  function find_input(node) {
    if (node.t === "input" && node.state?.attributes?.n === "automation-run-param-username") return node;
    for (const child of node.children || []) {
      if (!child || typeof child !== "object") continue;
      const input = find_input(child);
      if (input) return input;
    }
    return null;
  }
  const input = find_input(dialog);
  assert.ok(input, "the actual parameter input must be rendered");
  input.events.onInput({ target: { value: "creator@finder" } });
  assert.equal(model.state.run_params.value[0].input$.state.value, "creator@finder");
  await model.methods.submitRun();
  assert.equal(model.state.run_error.value, "");
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow"))?.body,
    { id: "target", initial_data: { username: "creator@finder" } });
  model.methods.destroy();
});

test("list trigger failures retain entered parameters and allow retrying", async () => {
  const { model, calls } = setup({ mode: "list", reject_trigger: true, flows: [
    { id: "target", context_schema: [{ key: "username", type: "string", required: true }] },
  ] });
  await model.methods.refresh();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  model.state.run_params.value[0].input$.setValue("creator@finder");
  await model.methods.submitRun();
  assert.match(model.state.run_error.value, /trigger rejected/);
  assert.equal(model.state.run_open.value, true);
  assert.equal(model.state.run_params.value[0].input$.state.value, "creator@finder");
  assert.equal(model.state.run_submitting.value, false);
  await model.methods.submitRun();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow")).length, 2);
  model.methods.destroy();
});

test("an in-flight list trigger cannot switch targets or submit a second execution", async () => {
  let release;
  const trigger_wait = new Promise((resolve) => { release = resolve; });
  const { model, calls } = setup({ mode: "list", trigger_wait, flows: [{ id: "first" }, { id: "second" }] });
  await model.methods.refresh();
  const pending = model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[1]).click();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow")).length, 1);
  assert.equal(model.state.selected_pipeline.value.id, "first");
  release();
  await pending;
  await new Promise(setImmediate);
  assert.equal(model.state.run_submitting.value, false);
  model.methods.destroy();
});

test("selecting history maps node states and fetches detailed payloads only when a node is opened", async () => {
  const definition = { id: "flow-test", start_node: "start", nodes: {
    start: { type: "StartNode", next_node_ids: ["fetch"] },
    fetch: { type: "ServiceNode", next_node_ids: ["fail"] },
    fail: { type: "JSCodeNode", next_node_ids: ["end"] },
    end: { type: "EndNode" },
  } };
  const logs = [
    { flow_id: "flow-test", node_id: "start", outcome: "completed", success: true },
    { flow_id: "flow-test", node_id: "fetch", attempt: 1, outcome: "retrying", error: "timeout" },
    { flow_id: "flow-test", node_id: "fetch", attempt: 2, outcome: "completed", output: { videos: [1] } },
    { flow_id: "flow-test:child", node_id: "start", outcome: "failed", error: "child error" },
    { flow_id: "flow-test", node_id: "fail", outcome: "failed", error: "invalid result" },
  ];
  const runs = [
    { id: "historical", flow_id: "flow-test", status: "FAILED", current_node: "fail", definition: JSON.stringify(definition), node_logs: JSON.stringify(logs) },
    { id: "other", flow_id: "flow-test", status: "COMPLETED", node_logs: "[]" },
  ];
  const { model, calls, flow } = setup({ mode: "detail", runs });
  flow.definition = JSON.stringify(definition);
  await model.methods.refresh();
  await model.methods.selectRun("historical");
  assert.deepEqual(Array.from(model.state.edit_nodes.value, (node) => node.id), ["start", "fetch", "fail", "end"]);
  const states = model.state.node_execution_states.value;
  assert.equal(states.start.status, "COMPLETED", "a child node with the same ID must not overwrite its parent");
  assert.equal(states.fetch.status, "COMPLETED");
  assert.equal(states.fail.status, "FAILED");
  assert.equal(states.end.status, "PENDING");
  assert.equal(model.methods.nodeExecutionRecord("fetch").executions.length, 0);
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 0);
  assert.deepEqual(calls.find((call) => call.url.endsWith("get_run_node_states")).body, { id: "historical" });
  model.methods.openNodeExecutionWindow("fetch");
  await new Promise(setImmediate);
  const record = JSON.parse(JSON.stringify(model.methods.nodeExecutionRecord("fetch")));
  assert.equal(record.run_id, "historical");
  assert.equal(record.attempt, 2);
  assert.equal(record.executions.length, 2);
  assert.deepEqual(record.executions[1].output, { videos: [1] });
  assert.equal("_execution_key" in record.executions[0], false);
  const revision = model.state.canvas_revision.value;
  const log_requests = calls.filter((call) => call.url.endsWith("get_run_node_logs")).length;
  await model.methods.selectRun("historical");
  assert.equal(model.state.canvas_revision.value, revision, "selecting a record must preserve the canvas viewport");
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, log_requests, "record selection must only request the status map");
  model.methods.openNodeExecutionWindow("fetch");
  await model.methods.selectRun("other");
  assert.equal(model.state.execution_windows.value.length, 0);
  assert.equal(model.state.node_execution_states.value.fetch.status, "PENDING");
  assert.equal(model.state.node_execution_states.value.start.status, "PENDING");
  assert.equal(model.state.execution_logs.value.length, 0);
  model.methods.destroy();
});

test("node details are requested once per open window and live logs only refresh open nodes", async () => {
  const channel = test_editor_channel();
  const { model, calls } = setup({ mode: "detail", live_channel: channel, runs: [
    { id: "active", flow_id: "flow-test", status: "RUNNING", node_logs: "[]" },
  ] });
  await model.methods.ready();
  await model.methods.selectRun("active");
  await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "start", input: { large: "ignored" } } });
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 0);
  assert.equal(model.state.execution_logs.value.length, 0);
  model.methods.openNodeExecutionWindow("start");
  await new Promise(setImmediate);
  model.methods.openNodeExecutionWindow("start");
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 1);
  await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "end" } });
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 1);
  await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "start" } });
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 2);
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: { node_id: "end", status: "WAITING_FOR_USER" } });
  await channel.receive({ type: "automation_run_waiting", run_id: "active", run_status: "WAITING" });
  assert.equal(model.state.selected_run.value.status, "WAITING");
  assert.equal(model.state.selected_run.value.current_node, "end");
  model.methods.closeNodeExecutionWindow("start");
  await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "start" } });
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 2);
  model.methods.destroy();
});

test("node loading shows failures, allows retry and discards responses from an older run", async () => {
  let release, fail = true;
  const { model, context } = setup({ mode: "detail", runs: [
    { id: "old", flow_id: "flow-test", status: "COMPLETED" },
    { id: "new", flow_id: "flow-test", status: "COMPLETED" },
  ], node_logs_handler: () => fail ? { error: new Error("node logs unavailable") } : new Promise((resolve) => { release = resolve; }) });
  await model.methods.ready();
  await model.methods.selectRun("old");
  model.methods.openNodeExecutionWindow("start");
  await new Promise(setImmediate);
  load_canvas_components(context);
  const window = context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  assert.ok(find_view(window, "automation-node-log-load-error-start"));
  assert.equal(model.methods.nodeExecutionDetail("start").value.error, "node logs unavailable");
  model.methods.closeNodeExecutionWindow("start");
  fail = false;
  model.methods.openNodeExecutionWindow("start");
  assert.equal(model.methods.nodeExecutionDetail("start").value.loading, true);
  await model.methods.selectRun("new");
  release({ data: { list: [{ node_id: "start", output: { old: true } }] } });
  await new Promise(setImmediate);
  assert.equal(model.methods.nodeExecutionRecord("start").executions.length, 0);
  assert.equal(model.state.execution_windows.value.length, 0);
  model.methods.destroy();
});

test("live node log refreshes coalesce requests and fetch the final pending update", async () => {
  const channel = test_editor_channel();
  const releases = [];
  const { model, calls } = setup({ mode: "detail", live_channel: channel, runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    node_logs_handler: () => new Promise((resolve) => { releases.push(resolve); }),
  });
  await model.methods.ready();
  await model.methods.selectRun("active");
  model.methods.openNodeExecutionWindow("start");
  for (let index = 0; index < 5; index++) {
    await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "start" } });
  }
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 1);
  releases[0]({ data: { list: [] } });
  await new Promise(setImmediate);
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 2);
  releases[1]({ data: { list: [{ node_id: "start", output: { final: true } }] } });
  await new Promise(setImmediate);
  assert.equal(model.methods.nodeExecutionRecord("start").executions[0].output.final, true);
  model.methods.destroy();
});

test("subflow reentry clears states and cached logs without fetching payloads or accepting an older response", async () => {
  const channel = test_editor_channel();
  const releases = [];
  const { model, calls } = setup({ mode: "detail", live_channel: channel, runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    node_logs_handler: () => new Promise((resolve) => { releases.push(resolve); }),
  });
  await model.methods.ready();
  await model.methods.selectRun("active");
  model.state.node_execution_states.as({ start: { status: "FAILED", error: "old failure", attempt: 3 }, end: { status: "COMPLETED" } });
  model.methods.openNodeExecutionWindow("start");
  releases[0]({ data: { list: [{ node_id: "start", outcome: "failed", error: "old failure" }] } });
  await new Promise(setImmediate);
  const detail = model.methods.nodeExecutionDetail("start");
  await channel.receive({ type: "automation_node_log", run_id: "active", execution_log: { node_id: "start" } });
  assert.equal(detail.value.loading, true);
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: { node_states: { start: "PENDING", end: "PENDING" } } });
  assert.equal(model.state.node_execution_states.value.start.status, "PENDING");
  assert.equal(model.state.node_execution_states.value.start.error, undefined);
  assert.equal(model.state.node_execution_states.value.start.attempt, undefined);
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(model.methods.nodeExecutionDetail("start"), detail, "open windows must retain their reactive detail reference");
  assert.equal(detail.value.logs.length, 0);
  assert.equal(detail.value.loading, false);
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 2);
  releases[1]({ data: { list: [{ node_id: "start", output: { stale: true } }] } });
  await new Promise(setImmediate);
  assert.equal(detail.value.logs.length, 0);
  model.methods.openNodeExecutionWindow("start");
  releases[2]({ data: { list: [{ node_id: "start", output: { current: true } }] } });
  await new Promise(setImmediate);
  assert.equal(model.methods.nodeExecutionRecord("start").executions[0].output.current, true);
  model.methods.destroy();
});

test("subflow reset rejects a status map requested before reentry", async () => {
  const channel = test_editor_channel();
  let release;
  const { model } = setup({ mode: "detail", live_channel: channel, runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    run_detail_handler: () => new Promise((resolve) => { release = resolve; }),
  });
  await model.methods.ready();
  const selection = model.methods.selectRun("active");
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: { node_states: { start: "PENDING", end: "PENDING" } } });
  release({ data: { start: "COMPLETED", end: "FAILED" } });
  await selection;
  assert.equal(model.state.selected_run.value.id, "active");
  assert.equal(model.state.node_execution_states.value.start.status, "PENDING");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  model.methods.destroy();
});

test("node state reset ignores another flow or an unselected run", async () => {
  const channel = test_editor_channel();
  const { model } = setup({ mode: "detail", live_channel: channel, runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }] });
  await model.methods.ready();
  await model.methods.selectRun("active");
  model.state.node_execution_states.as({ start: { status: "COMPLETED" } });
  const states = model.state.node_execution_states.value;
  await channel.receive({ type: "automation_node_status", flow_id: "another-child", run_id: "active", node_status: { node_states: { start: "PENDING" } } });
  await channel.receive({ type: "automation_node_status", run_id: "another-run", node_status: { node_states: { start: "PENDING" } } });
  assert.equal(model.state.node_execution_states.value, states);
  model.methods.destroy();
});

test("another node status update does not reformat an open node's payload", async () => {
  const channel = test_editor_channel();
  const { model, context } = setup({ mode: "detail", live_channel: channel, runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }] });
  await model.methods.ready();
  await model.methods.selectRun("active");
  model.methods.openNodeExecutionWindow("start");
  await new Promise(setImmediate);
  load_canvas_components(context);
  let formats = 0;
  const format = model.methods.formatExecutionValue;
  model.methods.formatExecutionValue = (value) => { formats++; return format(value); };
  context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  const before = formats;
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: { node_id: "end", status: "RUNNING" } });
  assert.equal(formats, before);
  model.methods.destroy();
});

test("manual execution from detail selects the new run and clears historical node results", async () => {
  const { model } = setup({ mode: "detail", runs: [{ id: "old", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([
    { node_id: "start", outcome: "completed" }, { node_id: "end", outcome: "completed" },
  ]) }] });
  await model.methods.ready();
  await model.methods.selectRun("old");
  assert.equal(model.state.node_execution_states.value.end.status, "COMPLETED");
  await model.methods.triggerFlow();
  await new Promise(setImmediate);
  assert.equal(model.state.selected_run_id.value, "run-flow-test");
  assert.equal(model.state.runs.value[0].id, "run-flow-test");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(model.state.execution_logs.value.length, 0);
  model.methods.destroy();
});

test("clearing run history resets the selected record, canvas traces and log windows", async () => {
  const { model, calls, context } = setup({ mode: "detail", runs: [
    { id: "finished", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([{ node_id: "start", outcome: "completed" }]) },
  ] });
  await model.methods.ready();
  await model.methods.selectRun("finished");
  model.methods.openNodeExecutionWindow("start", { clientX: 100, clientY: 100 });
  load_canvas_components(context);
  const history = context.AutomationRunHistory({ store: model });
  const actions = find_view(history, "automation-run-history-actions");
  assert.deepEqual(Array.from(actions.children, (child) => child.state.attributes.n), ["automation-clear-runs", "automation-refresh-runs"]);
  await model.ui.btn_clear_runs$.click();
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("clear_runs")).body, { flow_id: "flow-test" });
  assert.equal(model.state.runs.value.length, 0);
  assert.equal(model.state.selected_run.value, null);
  assert.equal(model.state.selected_run_id.value, "");
  assert.equal(model.state.execution_run_id.value, "");
  assert.equal(model.state.execution_logs.value.length, 0);
  assert.equal(model.state.execution_windows.value.length, 0);
  await model.methods.refreshRuns();
  assert.equal(model.state.runs.value.length, 0);
  model.methods.destroy();
});

test("clearing history keeps active runs available for selection and control", async () => {
  const { model } = setup({ mode: "detail", runs: [
    { id: "finished", flow_id: "flow-test", status: "FAILED" },
    { id: "waiting", flow_id: "flow-test", status: "WAITING", current_node: "end" },
  ] });
  await model.methods.ready();
  await model.ui.btn_clear_runs$.click();
  await new Promise(setImmediate);
  assert.deepEqual(Array.from(model.state.runs.value, (run) => run.id), ["waiting"]);
  assert.equal(model.state.selected_run.value, null);
  await model.methods.selectRun("waiting");
  assert.equal(model.state.selected_run.value.id, "waiting");
  await model.methods.actOnRun("cancel");
  assert.equal(model.state.selected_run.value.status, "CANCELLED");
  model.methods.destroy();
});

test("failed history clearing preserves the selected record and reports the error", async () => {
  const { model } = setup({ mode: "detail", runs: [{ id: "finished", flow_id: "flow-test", status: "COMPLETED" }],
    clear_runs_handler: async () => ({ error: new Error("clear rejected") }),
  });
  await model.methods.ready();
  await model.methods.selectRun("finished");
  await model.ui.btn_clear_runs$.click();
  await new Promise(setImmediate);
  assert.equal(model.state.runs.value.length, 1);
  assert.equal(model.state.selected_run.value.id, "finished");
  assert.equal(model.state.error.value, "clear rejected");
  model.methods.destroy();
});

test("clearing history ignores an older detail response and duplicate clicks", async () => {
  let release_detail, release_clear;
  const { model, calls } = setup({ mode: "detail",
    run_detail_handler: () => new Promise((resolve) => { release_detail = resolve; }),
    clear_runs_handler: () => new Promise((resolve) => { release_clear = resolve; }),
  });
  await model.methods.refresh();
  const detail = model.methods.selectRun("finished");
  const clearing = model.ui.btn_clear_runs$.click();
  await model.ui.btn_clear_runs$.click();
  await model.methods.refreshRuns();
  assert.equal(calls.filter((call) => call.url.endsWith("clear_runs")).length, 1);
  assert.equal(calls.filter((call) => call.url.endsWith("list_runs")).length, 0);
  release_clear({ data: { cleared: 1 } });
  await clearing;
  await new Promise(setImmediate);
  release_detail({ data: { start: "COMPLETED" } });
  await detail;
  assert.equal(model.state.selected_run.value, null);
  assert.equal(model.state.execution_run_id.value, "");
  model.methods.destroy();
});

test("historical selection survives manual refresh and ignores other runs on the live channel", async () => {
  const channel = test_editor_channel();
  const runs = [
    { id: "latest", flow_id: "flow-test", status: "RUNNING", current_node: "end", node_logs: "[]" },
    { id: "old", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([{ node_id: "start", success: true, outcome: "completed" }]) },
  ];
  const { model } = setup({ mode: "detail", runs, live_channel: channel });
  await model.methods.ready();
  assert.equal(model.state.selected_run_id.value, "");
  await model.methods.selectRun("old");
  await model.methods.refreshRuns();
  assert.equal(model.state.selected_run_id.value, "old");
  await channel.receive({ type: "automation_run_started", run_id: "latest", run_status: "RUNNING" });
  await channel.receive({ type: "automation_node_status", run_id: "latest", node_status: { node_id: "start", status: "FAILED" } });
  assert.equal(model.state.execution_run_id.value, "old");
  assert.equal(model.state.node_execution_states.value.start.status, "COMPLETED");
  await model.methods.selectRun("latest");
  await channel.receive({ type: "automation_node_status", run_id: "latest", node_status: { node_id: "end", status: "FAILED" } });
  assert.equal(model.state.node_execution_states.value.end.status, "FAILED");
  model.methods.destroy();
});

test("a slower run detail response cannot replace the newly selected record", async () => {
  const releases = {};
  const { model } = setup({ mode: "detail", run_detail_handler(id) {
    return new Promise((resolve) => { releases[id] = resolve; });
  } });
  await model.methods.refresh();
  const first = model.methods.selectRun("first");
  const second = model.methods.selectRun("second");
  releases.second({ data: { end: "FAILED" } });
  await second;
  releases.first({ data: { start: "COMPLETED" } });
  await first;
  assert.equal(model.state.selected_run.value.id, "second");
  assert.equal(model.state.execution_run_id.value, "second");
  assert.equal(model.state.node_execution_states.value.end.status, "FAILED");
  model.methods.destroy();
});

test("missing legacy traces remain readable and do not imply successful execution", async () => {
  const { model } = setup({ mode: "detail", runs: [{ id: "legacy", flow_id: "flow-test", status: "WAITING", current_node: "end", node_logs: "malformed", node_attempts: '{"start":1}' }] });
  await model.methods.refresh();
  await model.methods.selectRun("legacy");
  assert.equal(model.state.node_execution_states.value.start.status, "UNKNOWN");
  assert.equal(model.state.node_execution_states.value.end.status, "WAITING_FOR_USER");
  assert.equal(model.methods.nodeExecutionRecord("end").executions.length, 0);
  model.methods.destroy();
});

test("running and retrying detail nodes show a reactive decorative border that stops on completion", async () => {
  const { model, context } = setup({ mode: "detail" });
  await model.methods.refresh();
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  assert.equal(find_view(graph, "automation-flow-node-running-border-start"), null);
  for (const status of ["RUNNING", "RETRYING"]) {
    model.state.node_execution_states.as({ start: { node_id: "start", status } });
    await new Promise(setImmediate);
    const border = find_view(graph, "automation-flow-node-running-border-start");
    assert.ok(border, status);
    assert.equal(border.state.attributes["aria-hidden"], "true");
    assert.ok(find_view(border, "automation-flow-node-running-track-start"));
  }
  for (const status of ["COMPLETED", "FAILED", "WAITING", "PENDING", "CANCELLED"]) {
    model.state.node_execution_states.as({ start: { node_id: "start", status } });
    await new Promise(setImmediate);
    assert.equal(find_view(graph, "automation-flow-node-running-border-start"), null, status);
  }
  model.methods.destroy();
});

test("editor nodes keep their existing appearance while execution states change", async () => {
  const { model, context } = setup();
  await model.methods.refresh();
  load_canvas_components(context);
  model.state.node_execution_states.as({ start: { node_id: "start", status: "RUNNING" } });
  const graph = context.AutomationFlowGraph({ store: model, editable: true, executionLogs: true });
  assert.equal(find_view(graph, "automation-flow-node-running-border-start"), null);
  model.methods.destroy();
});

test("read-only canvas nodes open their run JSON without editing or dragging", async () => {
  const { model, context } = setup({ mode: "detail", runs: [{ id: "record", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([
    { node_id: "start", outcome: "completed", input: { username: "creator@finder" }, output: { ready: true } },
  ]) }] });
  await model.methods.refresh();
  await model.methods.selectRun("record");
  model.methods.closeNodeInspector();
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  const start = find_view(graph, "automation-flow-node-start");
  assert.ok(start);
  assert.equal(start.state.attributes.role, "button");
  assert.equal(start.state.attributes.tabindex, "0");
  assert.equal(find_view(graph, "automation-flow-connectors-start"), null);
  assert.equal(find_view(graph, "automation-start-run"), null);
  start.events.onClick({ clientX: 100, clientY: 120 });
  assert.equal(model.state.node_inspector_open.value, false);
  assert.equal(model.state.execution_windows.value[0].id, "start");
  await new Promise(setImmediate);
  const window = context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  const json = find_view(window, "automation-node-record-json-start");
  assert.ok(json);
  const record = JSON.parse(json.children.map((child) => child.state?.value || "").join(""));
  assert.equal(record.run_id, "record");
  assert.equal(record.status, "COMPLETED");
  assert.deepEqual(record.executions[0].output, { ready: true });
  assert.equal(find_view(window, "automation-node-error-start"), null);
  model.state.node_execution_states.as({ ...model.state.node_execution_states.value, start: { node_id: "start", status: "FAILED" } });
  model.methods.nodeExecutionDetail("start").as({ logs: [{ node_id: "start", outcome: "failed", error: "获取创作者列表失败：请求超时" }], loaded: true, loading: false, error: "" });
  const failed_window = context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  const status_row = find_view(failed_window, "automation-node-status-row-start");
  const error = find_view(status_row, "automation-node-error-start");
  assert.ok(error, "the log error must appear alongside the status badge");
  assert.equal(error.state.attributes.role, "alert");
  assert.equal(error.children[0].state.value, "获取创作者列表失败：请求超时");
  model.methods.destroy();
});

test("rendered nodes and log windows tolerate cleared states while switching execution records", async () => {
  const runs = [
    { id: "first", flow_id: "flow-test", status: "FAILED", current_node: "start", node_logs: JSON.stringify([{ node_id: "start", outcome: "failed", error: "first run failed" }]) },
    { id: "second", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([{ node_id: "start", outcome: "completed" }, { node_id: "end", outcome: "completed" }]) },
  ];
  const { model, context } = setup({ mode: "detail", runs });
  await model.methods.refresh();
  await model.methods.refreshRuns();
  await model.methods.selectRun("first");
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  find_view(graph, "automation-flow-node-start").events.onClick({ clientX: 100, clientY: 100 });
  await new Promise(setImmediate);
  const log_window = context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  assert.ok(find_view(log_window, "automation-node-error-start"));
  assert.doesNotThrow(() => model.state.node_execution_states.as({}), "empty states must not break existing badge subscriptions");
  const history = context.AutomationRunHistory({ store: model });
  const second = find_view(history, "automation-run-select-second");
  assert.ok(second);
  second.events.onClick({});
  await new Promise(setImmediate);
  assert.equal(model.state.error.value, "");
  assert.equal(model.state.selected_run.value.id, "second");
  assert.equal(model.state.node_execution_states.value.start.status, "COMPLETED");
  assert.equal(model.state.execution_windows.value.length, 0);
  await model.methods.selectRun("first");
  assert.equal(model.state.node_execution_states.value.start.status, "FAILED");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
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
  const { model, tips } = setup({ live_channel: channel });
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
  assert.match(tips.at(-1).msg[0], /MCP 已更新/);
  assert.equal(tips.at(-1).type, "success");
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
  await model.methods.selectRun("run-confirm");
  model.ui.input_manual_response$.setValue("[]"); await model.methods.actOnRun("resume");
  assert.equal(calls.filter((call) => call.url.endsWith("resume_run")).length, 0);
  model.ui.input_manual_response$.setValue('{"approved":true}'); await model.methods.actOnRun("resume");
  const submitted = calls.find((call) => call.url.endsWith("resume_run")).body;
  assert.deepEqual(submitted, { id: run.id, node_id: "confirm", input: { approved: true } });
  assert.equal(model.state.selected_run.value.status, "RUNNING");
  await model.methods.actOnRun("cancel"); assert.equal(model.state.selected_run.value.status, "CANCELLED");
  model.methods.destroy();
});

function add_config_declarations(flow) {
  const definition = JSON.parse(flow.definition);
  definition.nodes.start.config_schema = [{ key: "apikey", label: "入口 Key", type: "string", required: true, secret: true }];
  definition.nodes.end.config_schema = [
    { key: "apikey", label: "API Key", type: "string", required: true, secret: true },
    { key: "count", type: "integer", required: true },
    { key: "enabled", type: "boolean", required: true },
    { key: "options", type: "object" },
  ];
  flow.definition = JSON.stringify(definition);
  flow.node_configs = JSON.stringify({ start: { apikey: "entry-key" }, end: { apikey: "request-key", count: 0, enabled: false, options: { test: true } } });
}

test("configuration JSON export downloads saved values and imports them into a draft before saving", async () => {
  const { model, flow, context, calls, tips } = setup({ mode: "list" });
  add_config_declarations(flow);
  const original = flow.node_configs;
  let blob, downloaded = false, removed = false, revoked = false;
  const anchor = { setAttribute() {}, click() { downloaded = true; }, remove() { removed = true; } };
  context.Blob = Blob;
  context.URL = { createObjectURL(value) { blob = value; return "blob:config"; }, revokeObjectURL(url) { assert.equal(url, "blob:config"); revoked = true; } };
  context.document = { createElement(type) { assert.equal(type, "a"); return anchor; }, body: { appendChild(value) { assert.equal(value, anchor); } } };
  await model.methods.exportPipelineConfig(flow);
  assert.equal(downloaded, true);
  assert.equal(removed, true);
  assert.equal(anchor.download, "flow-test-config.json");
  assert.equal(blob.type, "application/json;charset=utf-8");
  const text = await blob.text();
  assert.deepEqual(JSON.parse(text), JSON.parse(original), "exports retain secrets and false/zero values");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(revoked, true);
  flow.node_configs = JSON.stringify({ start: { apikey: "different" } });
  await model.methods.importPipelineConfig(flow, { text: async () => `\uFEFF${text}` });
  assert.equal(model.state.config_groups.value[0].fields[0].input$.state.value, "entry-key");
  assert.equal(model.state.config_groups.value[1].fields[2].input$.state.checked, false);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
  assert.match(tips.at(-1).msg[0], /保存配置后生效/);
  await model.methods.savePipelineConfig();
  assert.deepEqual(JSON.parse(flow.node_configs), JSON.parse(original));
  assert.deepEqual(Object.keys(calls.find((call) => call.url.endsWith("update_flow")).body).sort(), ["id", "node_configs"]);
  model.methods.destroy();
});

test("configuration JSON imports merge partial typed values and reject invalid input without partial updates", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  const definition = JSON.parse(flow.definition);
  definition.nodes.end.config_schema.push(
    { key: "list", type: "array" },
    { key: "method", type: "string", options: [{ label: "GET", value: "GET" }] },
  );
  flow.definition = JSON.stringify(definition);
  for (const values of [[], null, { missing: {} }, { end: [] }, { end: { missing: "x" } },
    { start: { apikey: "changed" }, end: { count: 0.5 } }, { end: { enabled: "false" } },
    { end: { count: "1" } }, { end: { apikey: 1 } }, { end: { options: [] } },
    { end: { list: {} } }, { end: { method: "POST" } }]) {
    await model.methods.importPipelineConfig(flow, { text: async () => JSON.stringify(values) });
    assert.ok(model.state.config_error.value, JSON.stringify(values));
    if (model.state.config_groups.value.length) assert.equal(model.state.config_groups.value[0].fields[0].input$.state.value, "entry-key");
  }
  await model.methods.importPipelineConfig(flow, { text: async () => "{" });
  assert.ok(model.state.config_error.value);
  await model.methods.importPipelineConfig(flow, { text: async () => { throw new Error("read failed"); } });
  assert.match(model.state.config_error.value, /read failed/);
  await model.methods.importPipelineConfig(flow, { text: async () => JSON.stringify({ end: { enabled: true, count: 3, options: { nested: [false, 0] }, list: ["x"], method: "GET" } }) });
  assert.equal(model.state.config_error.value, "");
  assert.equal(model.state.config_groups.value[0].fields[0].input$.state.value, "entry-key");
  assert.equal(model.state.config_groups.value[1].fields[2].input$.state.checked, true);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
  await model.methods.savePipelineConfig();
  assert.deepEqual(JSON.parse(flow.node_configs).end, { apikey: "request-key", count: 3, enabled: true, options: { nested: [false, 0] }, list: ["x"], method: "GET" });
  model.methods.destroy();
});

test("the configuration column groups values by node and masks secrets", async () => {
  const { model, flow } = setup({ mode: "list" });
  add_config_declarations(flow);
  const definition = JSON.parse(flow.definition);
  definition.nodes.end.config_schema.push({ key: "username", label: "创作者", type: "string", required: true });
  flow.definition = JSON.stringify(definition);
  await model.methods.refresh();
  const pipeline = model.state.pipelines.value[0];
  let groups = model.methods.pipelineConfigGroups(pipeline);
  assert.equal(groups.length, 2);
  assert.equal(groups[0].fields[0].text, "••••••");
  assert.equal(groups[1].fields[0].text, "••••••");
  assert.equal(groups[1].fields[1].text, "0");
  assert.equal(groups[1].fields[2].text, "false");
  assert.equal(groups[1].fields[3].text, '{"test":true}');
  assert.equal(groups[1].fields[4].text, "未填写");
  pipeline.node_configs = { end: { username: "creator@finder" } };
  groups = model.methods.pipelineConfigGroups(pipeline);
  assert.equal(groups[1].fields[4].text, "creator@finder");
  pipeline.node_configs = "invalid JSON";
  assert.equal(model.methods.pipelineConfigGroups(pipeline)[1].fields[4].text, "未填写");
  model.methods.destroy();
});

test("automation settings load by node and save typed values independently of the graph", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  await model.methods.refresh();
  await model.methods.openPipelineConfig(model.state.pipelines.value[0]);
  const [start, end] = model.state.config_groups.value;
  assert.equal(start.fields[0].input$.state.value, "entry-key");
  assert.equal(end.fields[0].input$.state.value, "request-key");
  end.fields[0].input$.setValue("new-key", { silence: true });
  await model.methods.savePipelineConfig();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body;
  assert.deepEqual(saved, { id: flow.id, node_configs: { start: { apikey: "entry-key" }, end: { apikey: "new-key", count: 0, enabled: false, options: { test: true } } } });
  await model.methods.openPipelineConfig(flow);
  assert.equal(model.state.config_groups.value[1].fields[0].input$.state.value, "new-key");
  model.methods.destroy();
});

test("automation settings reject missing or invalid values and retain inputs after failure", async () => {
  const { model, flow, calls } = setup({ mode: "list", reject_save: true });
  add_config_declarations(flow);
  await model.methods.openPipelineConfig(flow);
  const fields = model.state.config_groups.value[1].fields;
  for (const [index, input, message] of [[0, " ", /不能为空/], [1, "0.5", /整数/], [3, "[]", /对象/]]) {
    const previous = fields[index].input$.state.value;
    fields[index].input$.setValue(input);
    await model.methods.savePipelineConfig();
    assert.match(model.state.config_error.value, message);
    assert.equal(calls.filter((call) => call.url.endsWith("update_flow")).length, 0);
    fields[index].input$.setValue(previous);
  }
  fields[0].input$.setValue("retained-key");
  await model.methods.savePipelineConfig();
  assert.match(model.state.config_error.value, /invalid Cron/);
  assert.equal(fields[0].input$.state.value, "retained-key");
  model.methods.destroy();
});

test("node configuration declarations survive editing, applying and saving", async () => {
  const { model, flow, calls } = setup();
  add_config_declarations(flow);
  await model.methods.refresh();
  model.methods.selectEditNode("end");
  const declaration = JSON.parse(model.state.edit_node_config_schema.value);
  assert.equal(declaration[0].key, "apikey");
  model.ui.input_edit_node_config_schema$.setValue('[{"key":"apikey","type":"string"},{"key":"apikey","type":"string"}]');
  model.methods.applySelectedNodeConfig();
  assert.match(model.state.error.value, /唯一/);
  model.ui.input_edit_node_config_schema$.setValue('[{"key":"apikey","label":"令牌","type":"string","secret":true}]');
  model.methods.applySelectedNodeConfig();
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body.nodes.find((node) => node.id === "end");
  assert.deepEqual(saved.config_schema, [{ key: "apikey", label: "令牌", type: "string", secret: true }]);
  model.methods.destroy();
});

test("rendered configuration forms mask secrets and submit input before blur", async () => {
  const { model, context, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  await model.methods.openPipelineConfig(flow);
  load_canvas_components(context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export default .*;$/gm, ""), context);
  const dialog = context.AutomationConfigDialog({ store: model });
  await model.methods.refresh();
  context.Table = (props) => props;
  const table = context.AutomationPipelineList({ store: model });
  const config_cell = table.columns.find((column) => column.name === "config").render(model.state.pipelines.value[0]);
  assert.ok(find_view({ children: config_cell }, `automation-pipeline-config-menu-trigger-${flow.id}`), "configuration dropdown renders with real component stores");
  const input = find_view(dialog, "automation-config-end-apikey");
  assert.equal(input.state.attributes.type, "secret");
  assert.equal(input.state.attributes.autocomplete, "off");
  assert.equal(input.state.attributes.spellcheck, "false");
  input.events.onChange({ target: { value: "typed-without-blur" } });
  const options = find_view(dialog, "automation-config-end-options");
  options.events.onChange({ target: { value: '{"updated":true}' } });
  await model.methods.savePipelineConfig();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body.node_configs.end;
  assert.equal(saved.apikey, "typed-without-blur");
  assert.deepEqual(saved.options, { updated: true });
  model.methods.destroy();
});

test("configuration choices and multiline templates render, load and save through the settings form", async () => {
  const { model, context, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  const definition = JSON.parse(flow.definition);
  definition.nodes.end.config_schema.push(
    { key: "push_method", label: "推送方式", type: "string", required: true, options: [{ label: "企业微信", value: "wecom" }, { label: "Webhook", value: "webhook" }] },
    { key: "message_template", label: "消息模板", type: "string", multiline: true },
  );
  flow.definition = JSON.stringify(definition);
  const values = JSON.parse(flow.node_configs);
  values.end.push_method = "wecom";
  values.end.message_template = "{username}\n{title}";
  flow.node_configs = JSON.stringify(values);
  await model.methods.openPipelineConfig(flow);
  const fields = model.state.config_groups.value[1].fields;
  const method = fields.find((field) => field.key === "push_method");
  assert.equal(method.input$.state.value, "wecom");
  method.input$.setValue("webhook");
  load_canvas_components(context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export default .*;$/gm, ""), context);
  const dialog = context.AutomationConfigDialog({ store: model });
  const template = find_view(dialog, "automation-config-end-message_template");
  assert.equal(template.state.attributes.rows, "4");
  template.events.onChange({ target: { value: "账号：{username}\n新视频：{title}" } });
  await model.methods.savePipelineConfig();
  const saved = calls.find((call) => call.url.endsWith("update_flow")).body.node_configs.end;
  assert.equal(saved.push_method, "webhook");
  assert.equal(saved.message_template, "账号：{username}\n新视频：{title}");
  model.methods.destroy();
});

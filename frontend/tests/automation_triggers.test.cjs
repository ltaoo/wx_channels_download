const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const source = fs.readFileSync(path.join(frontend, "src/pages/automation.model.js"), "utf8");

function setup({ legacy_event = false, schedules = [], reject_save = false, reject_trigger = false, trigger_wait = null, live_channel = null, flows = null, runs = [], run_detail_handler = null, mode = "edit" } = {}) {
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
      if (url.endsWith("list_runs")) return { data: { list: runs, total: runs.length } };
      if (url.endsWith("get_run")) return run_detail_handler ? run_detail_handler(body.id) : { data: runs.find((run) => run.id === body.id) };
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
      if (url.endsWith("list_schedules")) {
        const filtered = schedules.filter((item) => !body.flow_id || item.flow_id === body.flow_id);
        return { data: { list: filtered.slice((body.page - 1) * body.page_size, body.page * body.page_size), total: filtered.length } };
      }
      if (url.endsWith("update_flow")) {
        if (reject_save) return { error: new Error("invalid Cron") };
        if (body.name !== undefined) flow.name = body.name;
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

test("the editor does not poll removed run history; the detail page polls and releases its timer", async () => {
  for (const mode of ["edit", "detail"]) {
    const { model, calls, timers } = setup({ mode });
    await model.methods.ready();
    assert.equal(calls.filter((call) => call.url.endsWith("list_runs")).length, mode === "detail" ? 1 : 0);
    const polling = [...timers.values()].filter((timer) => timer.delay === 3000);
    assert.equal(polling.length, mode === "detail" ? 1 : 0);
    if (polling.length) {
      await polling[0].fn();
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
  const config_button = config_cell.find((item) => item.props?.attributes?.n === `automation-pipeline-config-${pipeline.id}`);
  assert.ok(config_button);
  assert.equal(config_button.children[0], "配置");
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

test("selecting a historical run maps its snapshot, retries and pending nodes onto the canvas", async () => {
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
  const { model } = setup({ mode: "detail", runs });
  await model.methods.refresh();
  await model.methods.selectRun("historical");
  assert.deepEqual(Array.from(model.state.edit_nodes.value, (node) => node.id), ["start", "fetch", "fail", "end"]);
  const states = model.state.node_execution_states.value;
  assert.equal(states.start.status, "COMPLETED", "a child node with the same ID must not overwrite its parent");
  assert.equal(states.fetch.status, "COMPLETED");
  assert.equal(states.fail.status, "FAILED");
  assert.equal(states.end.status, "PENDING");
  const record = JSON.parse(JSON.stringify(model.methods.nodeExecutionRecord("fetch")));
  assert.equal(record.run_id, "historical");
  assert.equal(record.attempt, 2);
  assert.equal(record.executions.length, 2);
  assert.deepEqual(record.executions[1].output, { videos: [1] });
  assert.equal("_execution_key" in record.executions[0], false);
  const revision = model.state.canvas_revision.value;
  await model.methods.selectRun("historical");
  assert.equal(model.state.canvas_revision.value, revision, "polling the same definition must preserve the canvas viewport");
  model.methods.openNodeExecutionWindow("fetch");
  await model.methods.selectRun("other");
  assert.equal(model.state.execution_windows.value.length, 0);
  assert.equal(model.state.node_execution_states.value.fetch, undefined);
  assert.equal(model.state.node_execution_states.value.start.status, "PENDING");
  assert.equal(model.state.execution_logs.value.length, 0);
  model.methods.destroy();
});

test("manual execution from detail selects the new run and clears historical node results", async () => {
  const { model } = setup({ mode: "detail", runs: [{ id: "old", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([
    { node_id: "start", outcome: "completed" }, { node_id: "end", outcome: "completed" },
  ]) }] });
  await model.methods.ready();
  assert.equal(model.state.node_execution_states.value.end.status, "COMPLETED");
  await model.methods.triggerFlow();
  await new Promise(setImmediate);
  assert.equal(model.state.selected_run_id.value, "run-flow-test");
  assert.equal(model.state.runs.value[0].id, "run-flow-test");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(model.state.execution_logs.value.length, 0);
  model.methods.destroy();
});

test("historical selection survives polling and ignores other runs on the live channel", async () => {
  const channel = test_editor_channel();
  const runs = [
    { id: "latest", flow_id: "flow-test", status: "RUNNING", current_node: "end", node_logs: "[]" },
    { id: "old", flow_id: "flow-test", status: "COMPLETED", node_logs: JSON.stringify([{ node_id: "start", success: true, outcome: "completed" }]) },
  ];
  const { model } = setup({ mode: "detail", runs, live_channel: channel });
  await model.methods.ready();
  assert.equal(model.state.selected_run_id.value, "latest");
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
  releases.second({ data: { id: "second", flow_id: "flow-test", status: "FAILED", current_node: "end", node_logs: "[]" } });
  await second;
  releases.first({ data: { id: "first", flow_id: "flow-test", status: "COMPLETED", node_logs: "[]" } });
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
  const window = context.AutomationNodeExecutionWindow({ store: model, node_id: "start" });
  const json = find_view(window, "automation-node-record-json-start");
  assert.ok(json);
  const record = JSON.parse(json.children.map((child) => child.state?.value || "").join(""));
  assert.equal(record.run_id, "record");
  assert.equal(record.status, "COMPLETED");
  assert.deepEqual(record.executions[0].output, { ready: true });
  assert.equal(find_view(window, "automation-node-error-start"), null);
  model.state.node_execution_states.as({ ...model.state.node_execution_states.value, start: { node_id: "start", status: "FAILED" } });
  model.state.execution_logs.as([{ node_id: "start", outcome: "failed", error: "获取创作者列表失败：请求超时" }]);
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
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  find_view(graph, "automation-flow-node-start").events.onClick({ clientX: 100, clientY: 100 });
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
  const input = find_view(dialog, "automation-config-end-apikey");
  assert.equal(input.state.attributes.type, "password");
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

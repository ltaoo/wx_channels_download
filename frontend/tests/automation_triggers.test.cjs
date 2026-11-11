const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

const frontend = path.join(__dirname, "..");
const source = fs.readFileSync(path.join(frontend, "src/pages/automation.model.js"), "utf8");

function setup({ definitions = null, legacy_event = false, schedules = [], reject_save = false, reject_trigger = false, trigger_wait = null, copy_handler = null, live_channel = null, flows = null, runs = [], run_list_handler = null, run_detail_handler = null, node_logs_handler = null, clear_runs_handler = null, catalog = [], mode = "edit" } = {}) {
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
    id: "flow-test", flow_id: "template-test", name: "test", trigger_type: legacy_event ? "Event" : "Manual", event_key: legacy_event ? "old.event" : "",
    definition: JSON.stringify({ start_node: "start", nodes: {
      start: { id: "start", name: "开始", type: "StartNode", next_node_ids: ["end"] },
      end: { id: "end", name: "结束", type: "EndNode" },
    } }),
  };
  const template = { id: "template-test", name: "test", default_node_configs: "{}", definition: flow.definition, instance_count: 1 };
  const flow_definitions = definitions || (flows ? flows.map((entry) => ({ ...entry, instance_count: 1 })) : [template]);
  const current_definitions = () => {
    if (!definitions && !flows) { template.definition = flow.definition; template.default_node_configs = flow.default_node_configs || template.default_node_configs; }
    return flow_definitions;
  };
  schedules = schedules.map(({ flow_id, ...entry }) => ({ ...entry, instance_id: entry.instance_id || flow_id }));
  const calls = [];
  vm.runInContext(
    fs.readFileSync(path.join(frontend, "src/biz/request.js"), "utf8").replace(/^export /gm, ""),
    context,
  );
  const actual_request_core = context.Timeless.kit.RequestCore;
  const TestRequestCore = class {
    constructor(fn) { this.fn = fn; }
    async run(params) {
      const { url, body } = this.fn(params);
      calls.push({ url, body: JSON.parse(JSON.stringify(body)) });
      if (url.endsWith("list_runs")) {
        if (run_list_handler) return run_list_handler(body);
        return { data: { list: body.summary ? runs.map(({ id, created_at, status }) => ({ id, created_at, status })) : runs, total: runs.length } };
      }
      if (url.endsWith("get_run")) {
        assert.equal(body.metadata_only, true);
        const { definition, initial_data, node_logs, node_outputs, node_attempts, ...metadata } = runs.find((run) => run.id === body.id) || {};
        return { data: metadata };
      }
      if (url.endsWith("clear_runs")) {
        if (clear_runs_handler) return clear_runs_handler(body);
        runs = runs.filter((run) => (run.instance_id || run.flow_id) !== body.instance_id || !["COMPLETED", "FAILED", "CANCELLED"].includes(run.status));
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
      if (url.endsWith("list_flows")) return { data: { list: current_definitions() } };
      if (url.endsWith("get_flow")) return { data: current_definitions().find((entry) => entry.id === body.id) || template };
      if (url.endsWith("update_flow")) {
        const target = current_definitions().find((entry) => entry.id === body.id) || template;
        if (body.name !== undefined) target.name = body.name;
        if (body.default_node_configs !== undefined) target.default_node_configs = JSON.stringify(body.default_node_configs);
        const definition = JSON.parse(target.definition || "{}");
        if (body.nodes !== undefined) definition.nodes = Object.fromEntries(body.nodes.map((node) => [node.id, { ...node, next_node_ids: node.next_ids }]));
        target.definition = JSON.stringify(definition);
        for (const item of flows || [flow]) if (item.flow_id === target.id) item.default_node_configs = target.default_node_configs;
        return { data: target };
      }
      if (url.endsWith("list_flow_instances")) return { data: { list: flows || [flow] } };
      if (url.endsWith("copy_flow_instance")) {
        if (copy_handler) return copy_handler(body);
        const copied = { ...flow, id: "flow-copy", name: `${flow.name}（副本）` };
        flows = [...(flows || [flow]), copied];
        return { data: copied };
      }
      if (url.endsWith("create_flow_instance")) {
        const source = current_definitions().find((entry) => entry.id === body.flow_id) || template;
        const instance = { id: "flow-instance", flow_id: source.id, name: `${source.name}（实例）`, definition: source.definition, default_node_configs: source.default_node_configs || "{}", node_configs: "{}", trigger_type: "Manual" };
        flows = [...(flows || [flow]), instance];
        return { data: instance };
      }
      if (url.endsWith("trigger_flow_instance")) {
        if (trigger_wait) await trigger_wait;
        if (reject_trigger) return { error: new Error("trigger rejected") };
        const run = { id: `run-${body.id}`, flow_id: body.id, status: "RUNNING", current_node: "start", node_logs: "[]" };
        runs = [run, ...runs.filter((item) => item.id !== run.id)];
        return { data: run };
      }
      if (url.endsWith("get_flow_instance")) return { data: (flows || [flow]).find((item) => item.id === body.id) || flow };
      if (url.endsWith("list_flow_nodes")) return { data: { nodes: catalog } };
      if (url.endsWith("import_flow")) {
        const definition = JSON.parse(body.definition);
        const imported = { ...flow, id: "flow-imported", name: definition.name, definition: body.definition };
        flows = [...(flows || [flow]), imported];
        return { data: imported };
      }
      if (url.endsWith("list_schedules")) {
        const filtered = schedules.filter((item) => !body.instance_id || (item.instance_id || item.flow_id) === body.instance_id);
        return { data: { list: filtered.slice((body.page - 1) * body.page_size, body.page * body.page_size), total: filtered.length } };
      }
      if (url.endsWith("update_flow_instance")) {
        if (reject_save) return { error: new Error("invalid Cron") };
        const target = (flows || [flow]).find((item) => item.id === body.id) || flow;
        if (body.name !== undefined) target.name = body.name;
        const definition = JSON.parse(target.definition);
        if (body.name !== undefined) definition.name = body.name;
        if (body.start_node_id !== undefined) definition.start_node = body.start_node_id;
        if (body.context_schema !== undefined) definition.context_schema = body.context_schema;
        if (body.subflows !== undefined) definition.subflows = body.subflows;
        if (body.nodes !== undefined) definition.nodes = Object.fromEntries(body.nodes.map((node) => [node.id, { ...node, next_node_ids: node.next_ids }]));
        target.definition = JSON.stringify(definition);
        const shared = current_definitions().find((entry) => entry.id === target.flow_id);
        if (shared && body.nodes !== undefined) shared.definition = target.definition;
        if (body.node_configs !== undefined) target.node_configs = JSON.stringify(body.node_configs);
        if (body.triggers !== undefined) schedules = body.triggers.map((trigger, index) => ({
          ...trigger, id: trigger.id || `new-${index}`, instance_id: target.id,
          initial_data: JSON.stringify({ ...trigger.initial_data, __automation: { type: trigger.type, event_key: trigger.event_key, start_node: "start" } }),
        }));
        target.trigger_type = "Manual";
        target.event_key = "";
        return { data: target };
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
  const copied_texts = [];
  const app = { copy(text) { copied_texts.push(text); } };
  const client = {};
  const model = context.AutomationPageViewModel({ client, app, tip(message) { tips.push(message); }, view: { query: { id: flow.id } } }, { mode });
  return { model, calls, context, flow, timers, tips, copied_texts, app, client, actual_request_core };
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

test("the page reads independent flow and instance lists, including flows with no instances", async () => {
  const flows = [
    { id: "account-a", flow_id: "sync", name: "账号 A" },
    { id: "account-b", flow_id: "sync", name: "账号 B" },
  ];
  const definitions = [
    { id: "sync", name: "共享采集", instance_count: 2 },
    { id: "notify", name: "通知", instance_count: 0 },
  ];
  const { model, calls } = setup({ flows, definitions, mode: "list" });
  await model.methods.refresh();
  assert.ok(calls.some((call) => call.url.endsWith("list_flows")));
  assert.ok(calls.some((call) => call.url.endsWith("list_flow_instances")));
  assert.equal(model.state.total.value, 2);
  assert.equal(model.state.definitions.value.length, 2);
  assert.equal(model.state.definitions.value.find((item) => item.id === "notify").instance_count, 0);
  model.ui.input_keyword$.setValue("账号 A"); model.methods.search();
  assert.equal(model.state.paged_pipelines.value[0].id, "account-a");
  assert.equal(model.state.filtered_definitions.value.length, 2);
  model.ui.input_definition_keyword$.setValue("通知");
  assert.equal(model.state.filtered_definitions.value.length, 1);
  assert.equal(model.state.paged_pipelines.value[0].id, "account-a");
  model.ui.input_keyword$.setValue("共享采集"); model.methods.search();
  assert.equal(model.state.total.value, 2);
  await model.ui.btn_definitions$.click(); assert.equal(model.ui.definitions_drawer$.state.visible, true);
  model.methods.destroy();
});

test("flow drawer actions create inherited instances and keep flow editing out of the main list", async () => {
  const { model, context, calls } = setup({ mode: "list" });
  await model.methods.refresh();
  for (const name of ["View", "Button", "Input", "DropdownMenu", "DrawerHeader", "DrawerTitle", "DrawerDescription", "DrawerBody"]) {
    context[name] = (props, children) => ({ props, children });
  }
  context.Drawer = (props, children) => ({ props, children: children() });
  context.Table = (props) => ({ props });
  context.Timeless.Icon = (props) => ({ props });
  context.AutomationPipelineLink = (props) => ({ link: props });
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export default .*;$/gm, ""), context);
  const toolbar = context.AutomationPageToolbar({ store: model });
  const buttons = toolbar.children.at(-1).children;
  assert.ok(buttons.some((button) => button.children[0] === "新增配置实例"));
  assert.ok(!buttons.some((button) => ["导入 JSON", "创建流程"].includes(button.children[0])));
  const instance_table = context.AutomationPipelineList({ store: model }).props;
  const instance_actions = instance_table.columns.find((column) => column.name === "actions").render(model.state.pipelines.value[0]);
  assert.equal(instance_actions[0].children[0], "执行");
  assert.ok(!instance_actions.some((item) => item.link?.mode === "edit"));
  const drawer = context.AutomationDefinitionsDrawer({ store: model });
  assert.equal(drawer.props.store, model.ui.definitions_drawer$);
  const drawer_table = drawer.children[1].children[1].props;
  assert.equal(drawer_table.rows, model.state.filtered_definitions);
  const definition = model.state.definitions.value[0];
  const actions = drawer_table.columns.find((column) => column.name === "actions").render(definition);
  assert.ok(actions.some((action) => action.link?.mode === "edit"));
  await model.ui.btn_create_instance$.click();
  await actions[0].props.store.click();
  await new Promise(setImmediate);
  assert.equal(calls.filter((call) => call.url.endsWith("create_flow_instance")).length, 1);
  assert.equal(model.state.pipelines.value.length, 2);
  assert.equal(model.state.definitions.value.length, 1);
  assert.equal(model.ui.definitions_drawer$.state.exit, true, "show the created instance after closing the drawer");
  model.methods.destroy();
});

test("new configuration instances share the definition and refresh the list", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  await model.methods.refresh();
  const before = model.methods.pipelineDefinitionLabel(model.state.pipelines.value[0]);
  const menu = model.methods.pipelineActionsMenu(model.state.pipelines.value[0]);
  menu.show();
  menu.state.items.find((item) => item.label === "新增配置实例").click();
  assert.equal(menu.state.exit, true);
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("create_flow_instance")).body, { flow_id: flow.flow_id });
  assert.equal(calls.some((call) => call.url.endsWith("copy_flow_instance")), false);
  assert.equal(model.state.pipelines.value.length, 2);
  assert.equal(new Set(model.state.pipelines.value.map((item) => item.flow_id)).size, 1);
  assert.equal(model.methods.pipelineDefinitionLabel(model.state.pipelines.value[0]), before);
  assert.equal(model.state.definitions.value.length, 1);
  model.methods.destroy();
});

test("configuration instances can be named without changing their graph or triggers", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  await model.methods.openPipelineConfig(flow);
  model.ui.input_config_name$.setValue(" ");
  await model.methods.savePipelineConfig();
  assert.match(model.state.config_error.value, /名称不能为空/);
  assert.equal(calls.some((call) => call.url.endsWith("update_flow_instance")), false);
  model.ui.input_config_name$.setValue("  每天同步账号 A  ");
  await model.methods.savePipelineConfig();
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow_instance")).body, {
    id: flow.id, name: "每天同步账号 A", node_configs: {},
  });
  model.methods.destroy();
});

test("instance settings save only explicit overrides and can restore inheritance", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  flow.default_node_configs = flow.node_configs;
  await model.methods.refresh();
  model.methods.pipelineActionsMenu(model.state.pipelines.value[0]).state.items.find((item) => item.label === "新增配置实例").click();
  await new Promise(setImmediate);
  const instance = model.state.pipelines.value.find((item) => item.id !== flow.id);
  await model.methods.openPipelineConfig(instance);
  assert.equal(model.state.config_defaults.value, false);
  assert.ok(model.state.config_groups.value.every((group) => group.fields.every((field) => field.inherited.value)));
  await model.methods.savePipelineConfig();
  assert.deepEqual(calls.filter((call) => call.url.endsWith("update_flow_instance")).at(-1).body, { id: instance.id, node_configs: {} });
  await model.methods.openPipelineConfig(instance);
  const field = model.state.config_groups.value[1].fields[0];
  field.inherit$.setValue(false);
  field.input$.setValue("instance-key");
  await model.methods.savePipelineConfig();
  assert.deepEqual(calls.filter((call) => call.url.endsWith("update_flow_instance")).at(-1).body, { id: instance.id, node_configs: { end: { apikey: "instance-key" } } });
  await model.methods.openPipelineConfig(instance);
  model.state.config_groups.value[1].fields[0].inherit$.setValue(true);
  await model.methods.savePipelineConfig();
  assert.deepEqual(calls.filter((call) => call.url.endsWith("update_flow_instance")).at(-1).body.node_configs, {});
  model.methods.pipelineConfigMenu(instance).state.items.find((item) => item.label === "流程默认配置").click();
  await new Promise(setImmediate);
  assert.equal(model.state.config_defaults.value, true);
  model.state.config_groups.value[1].fields[0].input$.setValue("new-shared-key");
  await model.methods.savePipelineConfig();
  const saved = calls.filter((call) => call.url.endsWith("update_flow")).at(-1).body;
  assert.equal(saved.id, instance.flow_id);
  assert.equal(saved.default_node_configs.end.apikey, "new-shared-key");
  assert.equal(saved.node_configs, undefined);
  model.methods.destroy();
});

test("custom JS node metadata populates the picker, configuration and saved graph", async () => {
  const defaults = { arguments: { task: "{{input.event.data.task}}" }, output_key: "uploaded", timeout_seconds: 600 };
  const schema = [
    { key: "access_key", type: "string", required: true, secret: true },
    { key: "secret_key", type: "string", required: true, secret: true },
    { key: "bucket", type: "string", required: true },
    { key: "domain", type: "string", required: true },
    { key: "upload_url", type: "string", default: "https://upload-z2.qiniup.com" },
  ];
  const { model, calls, flow } = setup({ catalog: [{ type: "QiniuUploadNode", name: "上传到七牛云", default_config: defaults, config_schema: schema }] });
  await model.methods.refresh();
  model.methods.openAddDialog("QiniuUploadNode", "start", "end");
  assert.equal(model.state.add_name.value, "上传到七牛云");
  assert.deepEqual(JSON.parse(model.state.add_config.value), defaults);
  assert.equal(model.methods.flowLabel("QiniuUploadNode"), "上传到七牛云");
  await model.ui.btn_add_submit$.click();
  const node = model.state.edit_nodes.value.find((item) => item.type === "QiniuUploadNode");
  assert.ok(node);
  assert.equal(node.name, "上传到七牛云");
  assert.deepEqual(JSON.parse(JSON.stringify(node.config)), defaults);
  assert.deepEqual(JSON.parse(JSON.stringify(node.config_schema)), schema);
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance"));
  assert.ok(saved.body.nodes.some((item) => item.id === node.id && item.type === "QiniuUploadNode"));
  await model.methods.openPipelineConfig(flow);
  const fields = model.state.config_groups.value.find((item) => item.id === node.id).fields;
  assert.equal(fields[4].input$.state.value, "https://upload-z2.qiniup.com");
  await model.methods.savePipelineConfig();
  assert.match(model.state.config_error.value, /不能为空/);
  fields.slice(0, 4).forEach((field) => { field.inherit$.setValue(false); field.input$.setValue(`test-${field.key}`); });
  fields[4].input$.setValue("");
  await model.methods.savePipelineConfig();
  const settings = calls.filter((call) => call.url.endsWith("update_flow_instance") && call.body.node_configs).pop().body.node_configs[node.id];
  assert.equal(settings.upload_url, undefined, "declared defaults remain inherited");
  assert.equal(settings.secret_key, "test-secret_key");
  model.methods.destroy();
});

function find_view(node, name) {
  if (node.state?.attributes?.n === name) return node;
  for (const child of node.children || []) {
    if (!child || typeof child !== "object") continue;
    const found = find_view(child, name);
    if (found) return found;
  }
  return null;
}

test("instance history stays lightweight and repeated node executions load by cursor only after clicking", async () => {
  const channel = test_editor_channel();
  let newest = false;
  const entry = (log_id) => ({ log_id, node_id: "start", attempt: 1, timestamp: "2026-11-11T00:00:00Z", outcome: "completed", output: { occurrence: log_id } });
  const { model, calls } = setup({ mode: "detail", live_channel: channel,
    runs: [{ id: "history", flow_id: "template-test", instance_id: "flow-test", status: "RUNNING", created_at: 1 }],
    node_logs_handler(body) {
      if (body.before_id) {
        assert.equal(body.before_id, 201);
        return { data: { list: [entry(199), entry(200)], has_more: false, next_before_id: 0 } };
      }
      return { data: { list: newest ? [entry(201), entry(202), entry(203)] : [entry(201), entry(202)], has_more: true, next_before_id: 201 } };
    },
  });
  await model.methods.ready();
  await model.methods.selectRun("history");
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 0);
  assert.ok(calls.filter((call) => call.url.endsWith("get_run")).every((call) => call.body.metadata_only === true));
  assert.ok(calls.filter((call) => call.url.endsWith("list_runs")).every((call) => call.body.summary === true && call.body.instance_id === "flow-test"));
  model.methods.openNodeExecutionWindow("start");
  await new Promise(setImmediate);
  assert.deepEqual(Array.from(model.methods.nodeExecutionDetail("start").value.logs, (log) => log.log_id), [201, 202]);
  await model.ui.btn_node_logs_more$.bind({ id: "start" }).click();
  await new Promise(setImmediate);
  assert.deepEqual(Array.from(model.methods.nodeExecutionDetail("start").value.logs, (log) => log.log_id), [199, 200, 201, 202]);
  newest = true;
  await channel.receive({ type: "automation_node_log", flow_id: "template-test", run_id: "history", execution_log: { node_id: "start" } });
  await new Promise(setImmediate);
  const detail = model.methods.nodeExecutionDetail("start").value;
  assert.deepEqual(Array.from(detail.logs, (log) => log.log_id), [199, 200, 201, 202, 203]);
  assert.equal(detail.next_before_id, 0, "refreshing the latest page keeps already loaded older attempts");
  assert.equal(model.methods.nodeExecutionRecord("start").executions.length, 5);
  model.methods.destroy();
});

test("dropdown content animates on its root and unmounts when the exit animation ends", () => {
  const { context, model } = setup();
  load_canvas_components(context);
  const menu = new context.Timeless.vm.DropdownMenuCore();
  menu.show();
  const tree = context.DropdownMenu({ store: menu, attributes: { n: "animated-menu" } });
  const content = find_view(tree, "animated-menu");
  assert.ok(content.state.styleSet.includes("dm-dropdown-content"));
  assert.ok(content.state.styleSet.includes("is-entering"));
  const element = {};
  content.events.onAnimationEnd({ target: element, currentTarget: element });
  assert.equal(menu.state.enter, false);
  menu.hide();
  assert.equal(menu.state.visible, true);
  content.events.onAnimationEnd({ target: {}, currentTarget: element });
  assert.equal(menu.state.visible, true);
  content.events.onAnimationEnd({ target: element, currentTarget: element });
  assert.equal(menu.state.visible, false);
  model.methods.destroy();
});

test("editing the pipeline title marks the draft dirty and saves the trimmed name", async () => {
  const { model, calls } = setup();
  await model.methods.refresh();
  assert.equal(model.ui.input_edit_name$.state.value, "test");
  assert.equal(model.state.dirty.value, false);
  model.ui.input_edit_name$.setValue("  下载任务  ");
  assert.equal(model.state.dirty.value, true);
  assert.equal(model.state.selected_pipeline.value.name, "test");
  await model.methods.saveFlow();
  assert.equal(calls.find((call) => call.url.endsWith("update_flow_instance")).body.name, "下载任务");
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
  definition.triggers = [{ id: "foreign-trigger-id", type: "Cron", name: "daily", cron_expr: "0 12,18 * * *", enabled: true }];
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow_instance")).body.nodes.find((node) => node.id === "start").position, { x: 123, y: 456 });
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow_instance")).body.triggers, []);
  model.methods.destroy();
});

test("list import still creates a new pipeline through the import API", async () => {
  const { model, calls } = setup({ mode: "list" });
  model.methods.openImportDialog();
  model.ui.input_import_json$.setValue(JSON.stringify({ name: "new flow", start_node: "entry", nodes: { entry: { type: "StartNode" } } }));
  await model.methods.submitImport();
  assert.equal(calls.filter((call) => call.url.endsWith("import_flow")).length, 1);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
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
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
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

test("the automation list groups trigger, copy and delete under more and mounts the run dialog", async () => {
  const { model, context, calls } = setup({ mode: "list" });
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
  assert.notEqual(table.rowKey({ ...pipeline, updated_at: 1 }), table.rowKey({ ...pipeline, updated_at: 2 }), "saved configurations must rerender the row");
  const trigger_cell = table.columns.find((column) => column.name === "trigger").render(pipeline);
  assert.equal(trigger_cell.length, 1);
  assert.equal(trigger_cell[0].component, "badges");
  const cell = table.columns.find((column) => column.name === "actions").render(pipeline);
  const more_menu = cell.find((item) => item.props?.attributes?.n === `automation-pipeline-more-menu-${pipeline.id}`);
  assert.equal(cell.length, 3);
  const more_button = more_menu.children[0];
  assert.equal(more_button.props.store.state.variant, "outline");
  assert.equal(more_button.props.store.state.size, "sm");
  assert.equal(cell.some((item) => item.children?.some((label) => ["触发", "复制", "删除"].includes(label))), false);
  const menu_items = more_menu.props.store.state.items;
  assert.deepEqual(Array.from(menu_items, (item) => item.label), ["触发", "复制", "新增配置实例", "删除"]);
  const delete_item = menu_items.find((item) => item.label === "删除");
  delete_item.click();
  assert.equal(model.ui.delete_dialog$.state.visible, true);
  assert.equal(calls.filter((call) => call.url.endsWith("delete_flow_instance")).length, 0);
  assert.equal(cell.some((item) => item.props?.attributes?.n === `automation-pipeline-config-${pipeline.id}`), false);
  const config_column = table.columns.find((column) => column.name === "config");
  assert.equal(config_column.render(pipeline).length, 1, "instances without declared fields can still edit their name");
  pipeline.nodes[0].config_schema = [
    { key: "username", type: "string" }, { key: "count", type: "integer" }, { key: "enabled", type: "boolean" },
  ];
  const config_cell = config_column.render(pipeline);
  assert.equal(config_cell.length, 1, "unfilled configuration should show only the button");
  const config_button = config_cell[0].children.find((item) => item.props?.attributes?.n === `automation-pipeline-config-${pipeline.id}`);
  assert.ok(config_button);
  assert.equal(config_button.children[0], "配置");
  const config_menu = config_cell[0].children.find((item) => item.props?.attributes?.n === `automation-pipeline-config-menu-${pipeline.id}`);
  assert.deepEqual(Array.from(config_menu.props.store.state.items, (item) => item.label), ["流程默认配置", "复制JSON", "粘贴JSON"]);
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
  for (const name of ["AutomationListPage", "AutomationDefinitionsDrawer", "AutomationCreatePipelineDialog", "AutomationImportPipelineDialog", "AutomationDeletePipelineDialog", "AutomationConfigDialog", "AutomationPasteConfigDialog"]) {
    context[name] = () => ({ component: name });
  }
  const page = context.AutomationPageView({});
  assert.ok(page.children.some((item) => item.component === "run-dialog"));
  model.methods.destroy();
});

test("copying a pipeline refreshes the list and prevents duplicate submissions", async () => {
  let resolve_copy;
  const pending = new Promise((resolve) => { resolve_copy = resolve; });
  const { model, calls, flow, tips } = setup({ mode: "list", copy_handler: () => pending });
  await model.methods.refresh();
  const copying = model.methods.copyPipeline(flow);
  assert.equal(model.ui.btn_pipeline_copy$.bind(flow).state.loading, true);
  await model.methods.copyPipeline(flow);
  assert.equal(calls.filter((call) => call.url.endsWith("copy_flow_instance")).length, 1);
  assert.deepEqual(calls.find((call) => call.url.endsWith("copy_flow_instance")).body, { id: flow.id });
  const lists_before = calls.filter((call) => call.url.endsWith("list_flow_instances")).length;
  resolve_copy({ data: { id: "flow-copy" } });
  await copying;
  assert.equal(model.ui.btn_pipeline_copy$.bind(flow).state.loading, false);
  assert.equal(calls.filter((call) => call.url.endsWith("list_flow_instances")).length, lists_before + 1);
  assert.equal(tips.at(-1).type, "success");
  model.methods.destroy();
});

test("copy failures retain the source and report the error", async () => {
  const { model, flow, calls } = setup({ mode: "list", copy_handler: () => ({ error: new Error("copy rejected") }) });
  await model.methods.refresh();
  const lists_before = calls.filter((call) => call.url.endsWith("list_flow_instances")).length;
  await model.methods.copyPipeline(flow);
  assert.match(model.state.error.value, /copy rejected/);
  assert.equal(model.ui.btn_pipeline_copy$.bind(flow).state.loading, false);
  assert.equal(calls.filter((call) => call.url.endsWith("list_flow_instances")).length, lists_before);
  assert.equal(model.state.pipelines.value[0].id, flow.id);
  model.methods.destroy();
});

test("more menu actions close immediately, including when triggering or copying fails", async () => {
  for (const label of ["触发", "复制", "删除"]) {
    const { model, calls } = setup({ mode: "list", reject_trigger: true,
      copy_handler: async () => ({ error: new Error("copy rejected") }),
    });
    await model.methods.refresh();
    const menu = model.methods.pipelineActionsMenu(model.state.pipelines.value[0]);
    menu.show();
    assert.equal(menu.state.visible, true);
    assert.equal(menu.state.exit, false);
    menu.state.items.find((item) => item.label === label).click();
    assert.equal(menu.state.exit, true, `${label} must close the menu before the action finishes`);
    await new Promise(setImmediate);
    assert.equal(menu.state.exit, true);
    if (label === "删除") {
      assert.equal(model.ui.delete_dialog$.state.visible, true);
      assert.equal(calls.filter((call) => call.url.endsWith("delete_flow_instance")).length, 0);
    } else {
      assert.match(model.state.error.value, /rejected/);
    }
    model.methods.destroy();
  }
});

test("the more menu copies its own pipeline and lists the new record", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  await model.methods.refresh();
  model.methods.pipelineActionsMenu(model.state.pipelines.value[0]).state.items.find((item) => item.label === "复制").click();
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("copy_flow_instance")).body, { id: flow.id });
  assert.ok(model.state.pipelines.value.some((pipeline) => pipeline.id === "flow-copy"));
  model.methods.destroy();
});

test("triggering a parameterless list row executes that row's pipeline", async () => {
  const { model, calls, tips } = setup({ mode: "list", flows: [{ id: "row-target", name: "目标任务" }] });
  await model.methods.refresh();
  const pipeline = model.state.pipelines.value[0];
  model.methods.pipelineActionsMenu(pipeline).state.items.find((item) => item.label === "触发").click();
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance")).body, { id: "row-target", initial_data: {} });
  assert.equal(model.state.run_open.value, false);
  assert.match(tips.at(-1).msg[0], /已触发执行/);
  model.methods.destroy();
});

test("active list rows disable manual triggering without blocking other pipelines", async () => {
  const { model, calls } = setup({ mode: "list", flows: [
    { id: "active", active_run_id: "cron-run", context_schema: [{ key: "username", required: true }] },
    { id: "other" },
  ] });
  await model.methods.refresh();
  const [active, other] = model.state.pipelines.value;
  assert.equal(model.methods.pipelineActionsMenu(active).state.items[0].state.disabled, true);
  await model.ui.btn_pipeline_trigger$.bind(active).click();
  assert.equal(model.state.run_open.value, false);
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 0);
  assert.equal(model.ui.btn_pipeline_trigger$.bind(active).state.disabled, true);
  await model.ui.btn_pipeline_trigger$.bind(other).click();
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance")).body, { id: "other", initial_data: {} });
  assert.equal(model.methods.pipelineActionsMenu(other).state.items[0].state.disabled, true);
  model.methods.destroy();
});

test("run lifecycle events update trigger availability for every list row", async () => {
  const channel = test_editor_channel();
  const { model } = setup({ mode: "list", live_channel: channel, flows: [{ id: "target" }] });
  await model.methods.ready();
  const pipeline = model.state.pipelines.value[0];
  const disabled = () => model.methods.pipelineActionsMenu(pipeline).state.items[0].state.disabled;
  assert.equal(disabled(), false);
  await channel.receive({ type: "automation_run_started", flow_id: "target", run_id: "cron-run" });
  assert.equal(disabled(), true);
  await channel.receive({ type: "automation_run_waiting", flow_id: "target", run_id: "cron-run" });
  assert.equal(disabled(), true);
  await channel.receive({ type: "automation_run_completed", flow_id: "target", run_id: "old-run" });
  assert.equal(disabled(), true, "an old completion must not unlock the active execution");
  await channel.receive({ type: "automation_run_failed", flow_id: "target", run_id: "cron-run", run_status: "CANCELLED" });
  assert.equal(disabled(), false);
  model.methods.destroy();
});

test("an execution starting while parameters are entered blocks submitting another run", async () => {
  const channel = test_editor_channel();
  const { model, calls } = setup({ mode: "list", live_channel: channel, flows: [
    { id: "target", context_schema: [{ key: "username", type: "string", required: true }] },
  ] });
  await model.methods.ready();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  model.state.run_params.value[0].input$.setValue("creator");
  await channel.receive({ type: "automation_run_started", flow_id: "target", run_id: "scheduled" });
  assert.equal(model.ui.btn_run_flow$.state.disabled, true);
  assert.equal(model.ui.btn_run_submit$.state.disabled, true);
  await model.methods.submitRun();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 0);
  assert.match(model.state.run_error.value, /已有进行中的执行/);
  await channel.receive({ type: "automation_run_completed", flow_id: "target", run_id: "scheduled" });
  assert.equal(model.ui.btn_run_submit$.state.disabled, false);
  await model.methods.submitRun();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 1);
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
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 0);
  assert.match(model.state.run_error.value, /username/);
  model.state.run_params.value[0].input$.setValue("creator@finder");
  await model.methods.submitRun();
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance")).body,
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance"))?.body,
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
    assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 0);
  }
  page.input$.handleChange({ target: { value: "0" } });
  await model.methods.submitRun();
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance"))?.body,
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("trigger_flow_instance"))?.body,
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
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 2);
  model.methods.destroy();
});

test("an in-flight list trigger cannot switch targets or submit a second execution", async () => {
  let release;
  const trigger_wait = new Promise((resolve) => { release = resolve; });
  const { model, calls } = setup({ mode: "list", trigger_wait, flows: [{ id: "first" }, { id: "second" }] });
  await model.methods.refresh();
  const pending = model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[0]).click();
  await model.ui.btn_pipeline_trigger$.bind(model.state.pipelines.value[1]).click();
  assert.equal(calls.filter((call) => call.url.endsWith("trigger_flow_instance")).length, 1);
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

test("a reentered workflow resets downstream states without clearing upstream states or node history", async () => {
  const channel = test_editor_channel();
  let delay = false, release;
  const { model, calls } = setup({ mode: "detail", live_channel: channel,
    runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    run_detail_handler: () => delay ? new Promise((resolve) => { release = resolve; })
      : { data: { before: "COMPLETED", start: "COMPLETED", end: "COMPLETED" } },
    node_logs_handler: () => ({ data: { list: [{ node_id: "end", outcome: "completed", attempt: 1 }] } }),
  });
  await model.methods.ready();
  model.methods.openNodeExecutionWindow("end");
  await new Promise(setImmediate);
  const detail = model.methods.nodeExecutionDetail("end");
  delay = true;
  const selection = model.methods.selectRun("active");
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: {
    node_id: "start", status: "RUNNING", attempt: 2, node_states: { start: "RUNNING", end: "PENDING" },
  } });
  assert.equal(model.state.node_execution_states.value.before.status, "COMPLETED");
  assert.equal(model.state.node_execution_states.value.start.status, "RUNNING");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(detail.value.logs.length, 1);
  assert.equal(model.state.selected_run.value.current_node, "start");
  release({ data: { before: "COMPLETED", start: "COMPLETED", end: "COMPLETED" } });
  await selection;
  assert.equal(model.state.node_execution_states.value.start.status, "RUNNING");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(model.state.node_execution_states.value.start.attempt, 2);
  assert.equal(model.methods.nodeExecutionDetail("end"), detail);
  assert.equal(detail.value.logs.length, 1);
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 1);
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
  const selection = model.methods.ready();
  await new Promise(setImmediate);
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("clear_runs")).body, { instance_id: "flow-test" });
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
  assert.equal(model.state.selected_run.value.id, "waiting");
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

test("validation failure without a started event appears in history and retains historical selection", async () => {
  for (const historical of [false, true]) {
    const channel = test_editor_channel();
    const runs = historical ? [{ id: "old", flow_id: "flow-test", status: "COMPLETED", node_logs: "[]" }] : [];
    const { model } = setup({ mode: "detail", runs, live_channel: channel });
    await model.methods.ready();
    if (historical) await model.methods.selectRun("old");
    const error = "启动校验失败: 节点 上传到七牛云 的配置项 access_key 不能为空";
    runs.unshift({ id: "rejected", flow_id: "flow-test", status: "FAILED", error, trigger_type: "Event", trigger_key: "downloadtask.finished", node_logs: "[]" });
    await channel.receive({ type: "automation_run_failed", flow_id: "flow-test", run_id: "rejected", run_status: "FAILED", error });
    await new Promise(setImmediate);
    assert.ok(model.state.runs.value.some((run) => run.id === "rejected" && run.status === "FAILED"));
    if (historical) assert.equal(model.state.selected_run_id.value, "old");
    await model.methods.selectRun("rejected");
    assert.equal(model.state.selected_run.value.error, error);
    assert.equal(model.state.selected_run.value.trigger_key, "downloadtask.finished");
    model.methods.destroy();
  }
});

test("run history rows receive live lifecycle states even while another record is selected", async () => {
  for (const selected of ["active", "old"]) {
    const channel = test_editor_channel();
    const runs = [{ id: "active", flow_id: "flow-test", status: "RUNNING" }, { id: "old", flow_id: "flow-test", status: "COMPLETED" }];
    const { model, calls } = setup({ mode: "detail", runs, live_channel: channel });
    await model.methods.ready();
    await model.methods.selectRun(selected);
    const requests = calls.length;
    await channel.receive({ type: "automation_run_waiting", run_id: "active", run_status: "WAITING" });
    assert.equal(model.state.runs.value.find((run) => run.id === "active").status, "WAITING");
    await channel.receive({ type: "automation_run_started", run_id: "active", run_status: "RUNNING" });
    assert.equal(model.state.runs.value.find((run) => run.id === "active").status, "RUNNING");
    await channel.receive({ type: "automation_run_completed", run_id: "active", run_status: "COMPLETED" });
    assert.equal(model.state.runs.value.find((run) => run.id === "active").status, "COMPLETED");
    assert.equal(model.state.selected_run_id.value, selected);
    assert.equal(model.state.selected_run.value.status, "COMPLETED");
    assert.equal(calls.length, requests, "known run updates do not need a refresh request");
    await channel.receive({ type: "automation_run_started", flow_id: "another-flow", run_id: "active", run_status: "RUNNING" });
    assert.equal(model.state.runs.value.find((run) => run.id === "active").status, "COMPLETED");
    model.methods.destroy();
  }
});

test("run history rows show live failures and cancellations", async () => {
  for (const status of ["FAILED", "CANCELLED"]) {
    const channel = test_editor_channel();
    const { model } = setup({ mode: "detail", runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }], live_channel: channel });
    await model.methods.ready();
    await channel.receive({ type: "automation_run_failed", run_id: "active", run_status: status, error: "execution stopped" });
    assert.equal(model.state.runs.value[0].status, status);
    assert.equal(model.state.selected_run.value.status, status);
    model.methods.destroy();
  }
});

test("new runs appear in history without replacing the selected historical record", async () => {
  const channel = test_editor_channel();
  const runs = [{ id: "old", flow_id: "flow-test", status: "COMPLETED" }];
  const { model } = setup({ mode: "detail", runs, live_channel: channel });
  await model.methods.ready();
  await model.methods.selectRun("old");
  runs.unshift({ id: "new", flow_id: "flow-test", status: "RUNNING" });
  await channel.receive({ type: "automation_run_started", run_id: "new", run_status: "RUNNING" });
  assert.equal(model.state.runs.value[0].id, "new");
  assert.equal(model.state.selected_run_id.value, "old");
  assert.equal(model.state.execution_run_id.value, "old");
  model.methods.destroy();
});

test("a stale history response cannot overwrite a live completion status", async () => {
  const channel = test_editor_channel();
  const runs = [{ id: "active", flow_id: "flow-test", status: "RUNNING" }];
  let release;
  let delay = false;
  const { model } = setup({ mode: "detail", runs, live_channel: channel, run_list_handler() {
    const response = { data: { list: runs.map(({ id, status }) => ({ id, status })), total: runs.length } };
    return delay ? new Promise((resolve) => { release = () => resolve(response); }) : response;
  } });
  await model.methods.ready();
  delay = true;
  const refresh = model.methods.refreshRuns();
  await new Promise(setImmediate);
  runs[0].status = "COMPLETED";
  await channel.receive({ type: "automation_run_completed", run_id: "active", run_status: "COMPLETED" });
  assert.equal(model.state.runs.value[0].status, "COMPLETED");
  release();
  await refresh;
  assert.equal(model.state.runs.value[0].status, "COMPLETED");
  assert.equal(model.state.selected_run.value.status, "COMPLETED");
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
  assert.equal(model.state.selected_run_id.value, "latest");
  assert.equal(model.state.node_execution_states.value.end.status, "RUNNING");
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

test("refresh and reconnect recover the selected run's node states without loading payloads", async () => {
  const channel = test_editor_channel();
  const runs = [{ id: "active", flow_id: "flow-test", status: "RUNNING", current_node: "start" }];
  const { model, calls } = setup({ mode: "detail", runs, live_channel: channel });
  await model.methods.ready();
  assert.equal(model.state.selected_run_id.value, "active");
  assert.equal(model.state.node_execution_states.value.start.status, "RUNNING");
  runs[0].current_node = "end";
  runs[0].node_logs = JSON.stringify([{ node_id: "start", outcome: "completed" }]);
  await model.methods.refreshRuns();
  assert.equal(model.state.node_execution_states.value.start.status, "COMPLETED");
  assert.equal(model.state.node_execution_states.value.end.status, "RUNNING");
  channel.changeState(false);
  runs[0].status = "COMPLETED";
  runs[0].node_logs = JSON.stringify([{ node_id: "start", outcome: "completed" }, { node_id: "end", outcome: "completed" }]);
  channel.changeState(true);
  await new Promise(setImmediate);
  assert.equal(model.state.selected_run_id.value, "active");
  assert.equal(model.state.execution_run_status.value, "COMPLETED");
  assert.equal(model.state.node_execution_states.value.end.status, "COMPLETED");
  assert.equal(calls.filter((call) => call.url.endsWith("get_run_node_logs")).length, 0);
  const requests = calls.length;
  model.methods.destroy();
  channel.changeState(false);
  channel.changeState(true);
  await new Promise(setImmediate);
  assert.equal(calls.length, requests);
});

test("a run starting on an idle detail page is selected from its persisted states", async () => {
  const channel = test_editor_channel();
  const runs = [];
  const { model } = setup({ mode: "detail", runs, live_channel: channel });
  await model.methods.ready();
  assert.equal(model.state.selected_run_id.value, "");
  runs.push({ id: "active", flow_id: "flow-test", status: "RUNNING", current_node: "end" });
  await channel.receive({ type: "automation_run_started", run_id: "active", run_status: "RUNNING" });
  assert.equal(model.state.selected_run_id.value, "active");
  assert.equal(model.state.node_execution_states.value.end.status, "RUNNING");
  model.methods.destroy();
});

test("live node and run statuses received during selection override an older state response", async () => {
  const channel = test_editor_channel();
  let release;
  const { model } = setup({ mode: "detail", live_channel: channel,
    runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    run_detail_handler: () => new Promise((resolve) => { release = resolve; }),
  });
  const ready = model.methods.ready();
  await new Promise(setImmediate);
  await channel.receive({ type: "automation_node_status", run_id: "active", node_status: { node_id: "end", status: "WAITING_FOR_USER", attempt: 2 } });
  await channel.receive({ type: "automation_run_waiting", run_id: "active", run_status: "WAITING" });
  release({ data: { start: "COMPLETED", end: "PENDING" } });
  await ready;
  assert.equal(model.state.node_execution_states.value.start.status, "COMPLETED");
  assert.equal(model.state.node_execution_states.value.end.status, "WAITING_FOR_USER");
  assert.equal(model.state.node_execution_states.value.end.attempt, 2);
  assert.equal(model.state.selected_run.value.current_node, "end");
  assert.equal(model.state.selected_run.value.status, "WAITING");
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

test("reusable subflow containers restore real states, isolate duplicate node IDs, and open scoped logs", async () => {
  const channel = test_editor_channel();
  const { model, flow, calls, context, client, actual_request_core } = setup({ mode: "detail", live_channel: channel,
    runs: [{ id: "active", flow_id: "flow-test", status: "RUNNING" }],
    run_detail_handler: (id, body) => ({ data: body.flow_id ? { start: "COMPLETED", end: "RUNNING" } : { start: "RUNNING", end: "PENDING" } }),
    node_logs_handler: (body) => ({ data: { list: [{ node_id: body.node_id, flow_id: body.flow_id, attempt: 1, output: { scope: body.flow_id || "root" } }] } }),
  });
  flow.definition = JSON.stringify({ start_node: "start", subflows: { reusable: {
    id: "child-flow", name: "可复用子流程", start_node: "start", nodes: {
      start: { type: "WaitNode", next_node_ids: ["end"] }, end: { type: "WaitNode", name: "子流程结束" },
    },
  } }, nodes: {
    start: { type: "WorkflowNode", name: "调用子流程", config: { flow_id: "reusable", output_key: "result" }, next_node_ids: ["end"] },
    end: { type: "EndNode" },
  } });
  await model.methods.ready();
  const group = model.state.subflow_groups.value[0];
  const child_end = group.nodes.find((node) => node.id === "end");
  assert.equal(model.state.node_execution_states.value.end.status, "PENDING");
  assert.equal(model.state.node_execution_states.value[child_end.execution_key].status, "RUNNING");
  assert.deepEqual(calls.find((call) => call.url.endsWith("get_run_node_states") && call.body.flow_id).body, { id: "active", flow_id: "child-flow" });
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  assert.equal(find_view(graph, "automation-flow-node-start"), null, "the invocation card must become the actual container");
  const node_view = find_view(graph, `automation-subflow-node-${group.id}-end`);
  assert.ok(node_view);
  assert.ok(find_view(graph, `automation-subflow-running-border-${group.id}-end`));
  node_view.events.onClick({ clientX: 300, clientY: 200, stopPropagation() {} });
  await new Promise(setImmediate);
  assert.deepEqual(calls.find((call) => call.url.endsWith("get_run_node_logs")).body, { id: "active", node_id: "end", flow_id: "child-flow" });
  assert.equal(model.methods.nodeExecutionRecord(child_end.execution_key).node_id, "end");
  assert.equal(model.methods.nodeExecutionRecord(child_end.execution_key).flow_id, "child-flow");
  await channel.receive({ type: "automation_node_status", flow_id: "child-flow", run_id: "active", node_status: { node_states: { start: "PENDING", end: "PENDING" } } });
  assert.equal(model.state.node_execution_states.value[child_end.execution_key].status, "PENDING");
  assert.equal(model.state.node_execution_states.value.start.status, "RUNNING");
  assert.equal(model.methods.nodeExecutionDetail(child_end.execution_key).value.logs.length, 0);
  await channel.receive({ type: "automation_node_status", flow_id: "child-flow", run_id: "other", node_status: { node_id: "end", status: "FAILED" } });
  assert.equal(model.state.node_execution_states.value[child_end.execution_key].status, "PENDING");

  // The bundled request core coalesces calls on one instance: scopes need independent instances.
  context.Timeless.kit.RequestCore = actual_request_core;
  const requests = [];
  client.post = async (url, body) => {
    requests.push({ url, body });
    await Promise.resolve();
    const data = url.endsWith("get_run_node_logs") ? { list: [{ node_id: body.node_id, output: { scope: body.flow_id || "root" } }] }
      : body.flow_id ? { start: "RUNNING", end: "PENDING" } : { start: "RUNNING", end: "COMPLETED" };
    return { data: { code: 0, data } };
  };
  await model.methods.selectRun("active");
  assert.equal(requests.filter((request) => request.url.endsWith("get_run_node_states")).length, 2);
  assert.equal(model.state.node_execution_states.value.end.status, "COMPLETED");
  assert.equal(model.state.node_execution_states.value[child_end.execution_key].status, "PENDING");
  model.methods.openNodeExecutionWindow("end");
  model.methods.openNodeExecutionWindow(child_end.execution_key);
  await new Promise(setImmediate);
  assert.equal(model.methods.nodeExecutionRecord("end").executions[0].output.scope, "root");
  assert.equal(model.methods.nodeExecutionRecord(child_end.execution_key).executions[0].output.scope, "child-flow");
  model.methods.destroy();
});

test("subflow containers enclose negative positions and resize with their child nodes", () => {
  const { context, model } = setup();
  const workflow = { name: "子流程", start_node: "a", nodes: {
    a: { type: "WaitNode", position: { x: -120, y: -60 }, next_node_ids: ["b"] },
    b: { type: "WaitNode", position: { x: 200, y: 140 } },
  } };
  const nodes = [{ id: "child", type: "WorkflowNode", name: "调用子流程", position: { x: -40, y: 10 }, config: { workflow } }];
  const first = context.calculate_pipeline_subflow_groups(nodes)[0];
  assert.equal(first.x, -40);
  assert.equal(first.width, 548);
  assert.equal(first.height, 376);
  assert.equal(first.nodes[0].x, 24);
  assert.equal(first.nodes[0].y, 48);
  for (const node of first.nodes) {
    assert.ok(node.x >= 24 && node.x + 180 <= first.width - 24);
    assert.ok(node.y >= 48 && node.y + 104 <= first.height - 24);
  }
  workflow.nodes.b.position.x += 300;
  workflow.nodes.b.position.y += 200;
  const expanded = context.calculate_pipeline_subflow_groups(nodes)[0];
  assert.equal(expanded.width, first.width + 300);
  assert.equal(expanded.height, first.height + 200);
  model.methods.destroy();
});

test("edit and detail share the expanded subflow graph and preserve invocation editing", async () => {
  const { context, model, flow } = setup();
  const nested = { name: "内层流程", start_node: "wait", nodes: { wait: { type: "WaitNode", name: "内部等待" } } };
  flow.definition = JSON.stringify({ name: "主流程", start_node: "child", nodes: {
    child: { type: "WorkflowNode", name: "外层调用", config: { workflow: { name: "外层流程", start_node: "nested", nodes: {
      nested: { type: "WorkflowNode", name: "内层调用", config: { workflow: nested } },
    } } } },
  } });
  await model.methods.refresh();
  const original = JSON.stringify(model.state.edit_nodes.value);
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  const detail = context.AutomationFlowGraph({ store: model, editable: false, executionLogs: true });
  const outer = find_view(graph, "automation-subflow-group-child");
  const inner = find_view(outer, "automation-subflow-group-child/nested");
  assert.ok(outer && inner);
  assert.ok(find_view(inner, "automation-subflow-node-child/nested-wait"));
  assert.equal(find_view(graph, "automation-flow-node-child"), null);
  assert.equal(find_view(graph, "automation-subflow-node-child-nested"), null, "a nested invocation is represented by its container");
  for (const name of ["automation-subflow-group-child", "automation-subflow-group-child/nested", "automation-subflow-node-child/nested-wait"]) {
    for (const property of ["left", "top", "width", "height"]) {
      assert.equal(find_view(graph, name).state.style[property], find_view(detail, name).state.style[property]);
    }
  }
  const heading = find_view(outer, "automation-subflow-heading-child");
  assert.equal(heading.state.attributes.role, "button");
  model.methods.closeNodeInspector();
  heading.events.onClick({ stopPropagation() {} });
  assert.equal(model.state.selected_edit_node_id.value, "child");
  assert.equal(model.state.node_inspector_open.value, true);
  assert.ok(find_view(graph, "automation-flow-connectors-child"));
  assert.equal(find_view(detail, "automation-flow-connectors-child"), null);
  assert.ok(find_view(graph, "automation-subflow-minimap-child"));
  assert.equal(JSON.stringify(model.state.edit_nodes.value), original);
  const group = model.state.subflow_groups.value[0];
  assert.ok(group.groups[0].x + group.groups[0].width <= group.width - 24);
  assert.ok(group.groups[0].y + group.groups[0].height <= group.height - 24);
  const bounds = context.automation_edit_layout(model.state.edit_nodes.value, {}, [group]);
  assert.ok(bounds.height >= group.y + group.height);
  assert.ok(bounds.width >= group.x + group.width);
  const referenced = context.calculate_pipeline_subflow_groups([
    { id: "reference", type: "WorkflowNode", config: { flow_id: "saved" } },
  ], [{ id: "saved", definition: JSON.stringify(nested) }]);
  assert.equal(referenced[0].nodes[0].name, "内部等待");
  const moved = JSON.parse(original);
  moved[0].position.x += 300;
  model.state.edit_nodes.as(moved, { reset: true });
  await new Promise(setImmediate);
  assert.equal(model.state.subflow_groups.value[0].x, group.x + 300);
  assert.ok(find_view(graph, "automation-subflow-group-child"));
  const layout = context.calculate_execution_layout(model.state.edit_nodes.value, model.state.subflow_groups.value, { child: { left: -100, top: 400 } });
  assert.equal(layout.groups[0].x, -100);
  assert.equal(layout.groups[0].y, 400);
  assert.ok(layout.height >= 400 + layout.groups[0].height);
  assert.equal(layout.min_left, -100);
  model.methods.destroy();
});

test("subflow selection opens the correct inspector and saves edits without node ID collisions", async () => {
  const { context, model, flow, calls } = setup();
  flow.definition = JSON.stringify({ start_node: "end", nodes: {
    end: { type: "StartNode", name: "主流程开始" },
    call: { type: "WorkflowNode", config: { workflow: { start_node: "end", nodes: {
      end: { type: "WaitNode", name: "外层等待", config: { seconds: 1 } },
      nested: { type: "WorkflowNode", name: "内层调用", config: { workflow: { start_node: "end", context_schema: [{ key: "child_input", type: "string" }], nodes: {
        end: { type: "WaitNode", name: "内层等待", config: { seconds: 2 } },
      } } } },
    } } } },
  } });
  await model.methods.refresh();
  load_canvas_components(context);
  context.TemplateEditor = (props) => context.View({ attributes: props.attributes });
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation_edit.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/export default FlowEditPageView;/, ""), context);
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  const inspector = context.AutomationNodeInspector({ store: model });
  const outer = find_view(graph, "automation-subflow-node-call-end");
  model.methods.closeNodeInspector();
  outer.events.onClick({ stopPropagation() {} });
  assert.equal(model.state.node_inspector_open.value, true);
  assert.equal(model.state.selected_edit_node.value.name, "外层等待");
  assert.equal(model.state.selected_edit_node.value.type, "WaitNode");
  assert.equal(outer.state.attributes["aria-pressed"], "true");
  assert.ok(find_view(inspector, "automation-edit-node-config"));
  assert.equal(find_view(inspector, "automation-start-triggers"), null, "the child with the root start ID has no root triggers");
  find_view(graph, "automation-subflow-heading-call/nested").events.onClick({ stopPropagation() {} });
  assert.equal(model.state.selected_edit_node.value.name, "内层调用");
  const inner = find_view(graph, "automation-subflow-node-call/nested-end");
  inner.events.onKeyDown({ key: "Enter", preventDefault() {}, stopPropagation() {} });
  assert.equal(model.state.selected_edit_node.value.name, "内层等待");
  assert.equal(model.ui.input_edit_node_config$.state.value, '{\n  "seconds": 2\n}');
  assert.ok(model.methods.availableContextKeys().some((key) => key.namespace === "input" && key.key === "child_input"));
  model.ui.input_edit_node_name$.setValue("已修改的内层等待");
  model.ui.input_edit_node_config$.setValue('{"seconds":7}');
  model.methods.applySelectedNodeConfig();
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
  const workflow = saved.nodes.find((node) => node.id === "call").config.workflow;
  assert.equal(workflow.nodes.nested.config.workflow.nodes.end.name, "已修改的内层等待");
  assert.equal(workflow.nodes.nested.config.workflow.nodes.end.config.seconds, 7);
  assert.equal(workflow.nodes.end.config.seconds, 1);
  assert.equal(saved.nodes.find((node) => node.id === "end").name, "主流程开始");
  assert.equal(model.state.selected_edit_node.value.name, "已修改的内层等待");
  model.methods.destroy();
});

test("reusable subflow configuration edits retain references and synchronize gateway routes", async () => {
  const { context, model, flow, calls } = setup();
  flow.definition = JSON.stringify({ start_node: "call", subflows: { local: {
    start_node: "check", nodes: {
      check: { type: "GatewayNode", config: { rules: [{ condition: "true", target_id: "old" }] }, next_node_ids: ["old"], next_nodes: [{ target_id: "old" }] },
      old: { type: "EndNode" }, next: { type: "EndNode" },
    },
  } }, nodes: { call: { type: "WorkflowNode", config: { flow_id: "local" } } } });
  await model.methods.refresh();
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  find_view(graph, "automation-subflow-node-call/local-check").events.onClick({ stopPropagation() {} });
  model.ui.input_edit_node_config$.setValue('{"rules":[{"condition":"true","target_id":"next"}]}');
  model.methods.applySelectedNodeConfig();
  assert.deepEqual([...model.state.selected_edit_node.value.next_ids], ["next"]);
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
  assert.equal(saved.nodes[0].config.flow_id, "local");
  assert.deepEqual(saved.subflows.local.nodes.check.next_node_ids, ["next"]);
  assert.deepEqual(saved.subflows.local.nodes.check.next_nodes, [{ target_id: "next" }]);
  model.methods.destroy();
});

test("selecting a referenced node does not edit a different saved pipeline", async () => {
  const external = { id: "external", definition: JSON.stringify({ start_node: "wait", nodes: { wait: { type: "WaitNode", config: { seconds: 5 } } } }) };
  const flows = [external];
  const { context, model, flow } = setup({ flows });
  flow.definition = JSON.stringify({ start_node: "call", nodes: { call: { type: "WorkflowNode", config: { flow_id: "external" } } } });
  flows.push(flow);
  await model.methods.refresh();
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  find_view(graph, "automation-subflow-node-call/external-wait").events.onClick({ stopPropagation() {} });
  assert.equal(model.state.selected_edit_node.value.config.seconds, 5);
  model.ui.input_edit_node_config$.setValue('{"seconds":8}');
  model.methods.applySelectedNodeConfig();
  assert.match(model.state.error.value, /被引用的自动化/);
  assert.equal(model.state.selected_edit_node.value.config.seconds, 5);
  assert.equal(JSON.parse(external.definition).nodes.wait.config.seconds, 5);
  assert.equal(model.state.edit_nodes.value[0].config.flow_id, "external");
  assert.equal(model.state.dirty.value, false);
  model.methods.destroy();
});

test("subflow gateways show yes, no and case labels in edit and detail views", async () => {
  const { context, model, flow } = setup();
  flow.definition = JSON.stringify({ start_node: "call", nodes: { call: { type: "WorkflowNode", config: { workflow: { start_node: "check", nodes: {
    check: { type: "GatewayNode", config: { condition_language: "js", rules: [{ condition: "input.ready", target_id: "yes" }, { condition: "true", target_id: "no" }] } },
    cases: { type: "GatewayNode", config: { rules: [{ condition: 'input.type === "VIDEO"', target_id: "yes" }, { condition: 'input.type === "IMAGE"', target_id: "no" }, { condition: "true", target_id: "end" }] } },
    parallel: { type: "GatewayNode", config: { gateway_type: "Parallel" }, next_node_ids: ["yes", "no"] },
    yes: { type: "WaitNode" }, no: { type: "WaitNode" }, end: { type: "EndNode" },
  } } } } } });
  await model.methods.refresh();
  load_canvas_components(context);
  for (const editable of [true, false]) {
    const graph = context.AutomationFlowGraph({ store: model, editable, executionLogs: !editable });
    for (const [id, label] of [["check-yes-0", "是"], ["check-no-1", "否"], ["cases-yes-0", "VIDEO"], ["cases-no-1", "IMAGE"], ["cases-end-2", "否"]]) {
      const view = find_view(graph, `automation-subflow-edge-label-call-${id}`);
      assert.ok(view);
      assert.equal(view.children.map((child) => child.state.value).join(""), label);
    }
    assert.match(find_view(graph, "automation-subflow-edge-label-call-check-yes-0").state.attributes.title, /\[JS\] input.ready/);
    assert.equal(find_view(graph, "automation-subflow-edge-label-call-parallel-yes-0"), null);
  }
  model.methods.destroy();
});

test("top-level nodes keep their drop coordinates across subflow rows and columns", async (t) => {
  for (const positioned of [true, false]) await t.test(positioned ? "saved position" : "automatic position", async () => {
    const { context, model, flow, calls } = setup();
    flow.definition = JSON.stringify({ start_node: "start", nodes: {
      start: { type: "StartNode", position: { x: 0, y: 0 }, next_node_ids: ["child"] },
      child: { type: "WorkflowNode", position: { x: 260, y: 0 }, next_node_ids: ["end"], config: { workflow: {
        start_node: "a", nodes: {
          a: { type: "WaitNode", position: { x: 0, y: 0 }, next_node_ids: ["b"] },
          b: { type: "WaitNode", position: { x: 600, y: 400 } },
        },
      } } },
      end: { type: "EndNode", ...(positioned ? { position: { x: 520, y: 130 } } : {}) },
    } });
    await model.methods.refresh();
    load_canvas_components(context);
    const listeners = new Map();
    context.document = {
      addEventListener(name, handler) { listeners.set(name, handler); },
      removeEventListener(name) { listeners.delete(name); },
    };
    let canvas_model;
    const CanvasModel = context.Timeless.vm.FlowCanvasModel;
    context.Timeless = { ...context.Timeless, vm: { ...context.Timeless.vm, FlowCanvasModel: class extends CanvasModel {
      constructor(options) { super(options); canvas_model = this; }
    } } };
    const graph = context.AutomationFlowGraph({ store: model, editable: true });
    const zoom = positioned ? 0.5 : 1;
    canvas_model.setViewport({ x: 80, y: 50, zoom });
    const group = model.state.execution_layout.value.groups[0];
    if (!positioned) assert.ok(model.state.execution_layout.value.positions.end.left >= group.x + group.width, "automatic placement leaves room for the subflow");
    const canvas = { style: {}, querySelectorAll: () => [] };
    const shell = { style: {}, classList: { add() {}, remove() {} } };
    const target = { closest: (selector) => selector.includes("node-shell") ? shell : canvas };
    let destination;
    for (destination of [
      { x: group.x + 40, y: group.y + group.height + 40 },
      { x: group.x + group.width + 40, y: group.y + 40 },
      { x: -100, y: -80 },
      { x: group.x + 40, y: group.y + 40 },
    ]) {
      const before = model.state.execution_layout.value.positions.end;
      find_view(graph, "automation-flow-node-end").events.onMouseDown({ button: 0,
        clientX: 80 + before.left * zoom, clientY: 50 + before.top * zoom,
        currentTarget: target, preventDefault() {}, stopPropagation() {},
      });
      const pointer = { clientX: 80 + destination.x * zoom, clientY: 50 + destination.y * zoom };
      listeners.get("mousemove")(pointer);
      assert.equal(shell.style.left, `${destination.x}px`);
      assert.equal(shell.style.top, `${destination.y}px`);
      listeners.get("mouseup")(pointer);
      const after = model.state.execution_layout.value.positions.end;
      assert.equal(after.left, destination.x, "release must retain the horizontal drop coordinate");
      assert.equal(after.top, destination.y, "release must retain the vertical drop coordinate");
      assert.equal(listeners.size, 0);
      for (const key of ["x", "y", "width", "height"]) assert.equal(model.state.execution_layout.value.groups[0][key], group[key]);
    }
    await model.methods.saveFlow();
    assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow_instance")).body.nodes.find((node) => node.id === "end").position, destination);
    assert.equal(model.state.execution_layout.value.positions.end.left, destination.x);
    assert.equal(model.state.execution_layout.value.positions.end.top, destination.y);
    model.methods.destroy();
  });
});

test("dragging expanded graph nodes saves their movement without including automatic subflow spacing", async () => {
  const { context, model, flow, calls } = setup();
  flow.definition = JSON.stringify({ start_node: "start", nodes: {
    start: { type: "StartNode", position: { x: 0, y: 0 }, next_node_ids: ["child"] },
    child: { type: "WorkflowNode", position: { x: 260, y: 0 }, next_node_ids: ["end"], config: { workflow: {
      start_node: "wait", nodes: { wait: { type: "WaitNode" } },
    } } },
    end: { type: "EndNode", position: { x: 520, y: 0 } },
  } });
  await model.methods.refresh();
  load_canvas_components(context);
  const listeners = new Map();
  context.document = {
    addEventListener(name, handler) { listeners.set(name, handler); },
    removeEventListener(name) { listeners.delete(name); },
  };
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  const canvas = { style: {}, querySelectorAll: () => [] };
  const shell = { style: {}, classList: { add() {}, remove() {} } };
  const target = { closest: (selector) => selector.includes("node-shell") ? shell : canvas };
  const event = { button: 0, clientX: 100, clientY: 100, currentTarget: target, preventDefault() {}, stopPropagation() {} };
  find_view(graph, "automation-flow-node-end").events.onMouseDown(event);
  listeners.get("mousemove")({ clientX: 130, clientY: 100 });
  listeners.get("mouseup")({ clientX: 130, clientY: 100 });
  assert.equal(model.state.edit_nodes.value.find((node) => node.id === "end").position.x, 550);
  find_view(graph, "automation-subflow-heading-child").events.onMouseDown(event);
  listeners.get("mousemove")({ clientX: 140, clientY: 120 });
  listeners.get("mouseup")({ clientX: 140, clientY: 120 });
  assert.deepEqual(JSON.parse(JSON.stringify(model.state.edit_nodes.value.find((node) => node.id === "child").position)), { x: 300, y: 20 });
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance"));
  assert.deepEqual(saved.body.nodes.find((node) => node.id === "child").position, { x: 300, y: 20 });
  model.methods.destroy();
});

test("subflow edges follow dragging live while container bounds update only on release", async () => {
  const { context, model, flow, calls } = setup();
  flow.definition = JSON.stringify({ start_node: "call", nodes: {
    call: { type: "WorkflowNode", position: { x: 200, y: 100 }, config: { workflow: {
      start_node: "a", nodes: {
        a: { type: "GatewayNode", position: { x: 0, y: 0 }, next_node_ids: ["b", "c"], config: { rules: [{ condition: "input.ready", target_id: "b" }, { condition: "true", target_id: "c" }] } },
        b: { type: "WaitNode", position: { x: 200, y: 100 }, next_node_ids: ["c"] },
        c: { type: "WaitNode", position: { x: 400, y: 200 }, next_node_ids: ["a"] },
      },
    } } },
  } });
  await model.methods.refresh();
  load_canvas_components(context);
  const listeners = new Map();
  context.document = {
    addEventListener(name, handler) { listeners.set(name, handler); },
    removeEventListener(name) { listeners.delete(name); },
  };
  let canvas_model;
  const CanvasModel = context.Timeless.vm.FlowCanvasModel;
  const DragCanvasModel = class extends CanvasModel {
    constructor(options) { super(options); canvas_model = this; }
  };
  context.Timeless = { ...context.Timeless, vm: { ...context.Timeless.vm, FlowCanvasModel: DragCanvasModel } };
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  const first = model.state.execution_layout.value.groups[0];
  canvas_model.setViewport({ x: 0, y: 0, zoom: 0.5 });
  const edges = ["a-b", "c-a", "b-c"].map((ids) => {
    const attributes = { ...find_view(graph, `automation-subflow-edge-call-${ids}`).state.attributes, d: "original" };
    return { getAttribute(name) { return attributes[name]; }, setAttribute(name, value) { attributes[name] = value; } };
  });
  const labels = ["a-b-0", "a-c-1"].map((id) => {
    const view = find_view(graph, `automation-subflow-edge-label-call-${id}`);
    return { style: { ...view.state.style }, getAttribute(name) { return view.state.attributes[name]; } };
  });
  const initial_label = { ...labels[0].style };
  const element = { style: {}, parentElement: { querySelectorAll(selector) {
    if (selector === ":scope > [data-subflow-edge-label]") return labels;
    assert.equal(selector, ":scope > svg > [data-flow-edge]", "only edges in the same subflow are queried");
    return edges;
  } } };
  const event = { button: 0, clientX: 150, clientY: 150, currentTarget: element, preventDefault() {}, stopPropagation() {} };
  const card = find_view(graph, "automation-subflow-node-call-a");
  assert.equal(card.state.attributes.tabindex, "0");
  card.events.onMouseDown(event);
  listeners.get("mousemove")({ clientX: 100, clientY: 110 });
  assert.equal(element.style.left, "-76px");
  assert.equal(element.style.top, "-32px");
  assert.match(edges[0].getAttribute("d"), /^M 104 20 C /, "outgoing edges follow the moved source");
  assert.match(edges[1].getAttribute("d"), /, -76 20$/, "incoming edges follow the moved target");
  assert.equal(edges[2].getAttribute("d"), "original", "unconnected edges remain untouched");
  assert.equal(parseFloat(labels[0].style.left), parseFloat(initial_label.left) - 50);
  assert.equal(parseFloat(labels[0].style.top), parseFloat(initial_label.top) - 40);
  for (const key of ["x", "y", "width", "height"]) assert.equal(model.state.execution_layout.value.groups[0][key], first[key]);
  assert.equal(model.state.dirty.value, false);
  listeners.get("mousemove")({ clientX: 125, clientY: 130 });
  assert.match(edges[0].getAttribute("d"), /^M 154 60 C /);
  assert.match(edges[1].getAttribute("d"), /, -26 60$/);
  listeners.get("mousemove")({ clientX: 100, clientY: 110 });
  listeners.get("mouseup")({ clientX: 100, clientY: 110 });
  assert.equal(listeners.size, 0);
  assert.match(edges[0].getAttribute("d"), /^M 204 100 C /, "transient paths are restored on cleanup");
  const moved = model.state.execution_layout.value.groups[0];
  assert.equal(moved.x, first.x - 100);
  assert.equal(moved.y, first.y - 80);
  assert.equal(moved.width, first.width + 100);
  assert.equal(moved.height, first.height + 80);
  assert.equal(moved.x + moved.nodes[1].x, first.x + first.nodes[1].x, "the other node stays in place");
  assert.equal(moved.y + moved.nodes[1].y, first.y + first.nodes[1].y);
  assert.equal(find_view(graph, "automation-subflow-group-call").state.style.width, `${moved.width}px`);
  assert.equal(model.state.dirty.value, true);
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body.nodes[0];
  assert.deepEqual(saved.position, { x: 100, y: 20 });
  assert.deepEqual(saved.config.node_positions.call.a, { x: -100, y: -80 });
  assert.deepEqual(saved.config.workflow.nodes.a.position, { x: 0, y: 0 });
  assert.equal(model.state.execution_layout.value.groups[0].x, moved.x);
  assert.equal(model.state.execution_layout.value.groups[0].width, moved.width);
  const current = find_view(graph, "automation-subflow-node-call-a");
  current.events.onKeyDown({ key: "ArrowRight", preventDefault() {}, stopPropagation() {} });
  assert.equal(model.state.execution_layout.value.groups[0].width, moved.width - 10);
  model.methods.destroy();
});

test("moving nested and reusable subflow nodes preserves declarations and ancestor bounds", async () => {
  const { context, model, flow, calls } = setup();
  flow.definition = JSON.stringify({ start_node: "call", subflows: { reusable: {
    id: "reusable", start_node: "nested", nodes: {
      nested: { type: "WorkflowNode", position: { x: 0, y: 0 }, config: { workflow: {
        start_node: "a", nodes: { a: { type: "WaitNode", position: { x: 0, y: 0 } } },
      } } },
    },
  } }, nodes: {
    call: { type: "WorkflowNode", position: { x: 200, y: 100 }, config: { flow_id: "reusable" } },
    again: { type: "WorkflowNode", position: { x: 900, y: 100 }, config: { flow_id: "reusable" } },
  } });
  await model.methods.refresh();
  load_canvas_components(context);
  const graph = context.AutomationFlowGraph({ store: model, editable: true });
  const first = model.state.execution_layout.value.groups[0];
  const untouched = model.state.execution_layout.value.groups[1];
  const nested = first.groups[0];
  const key_event = { key: "ArrowLeft", preventDefault() {}, stopPropagation() {} };
  find_view(graph, `automation-subflow-node-${nested.id}-a`).events.onKeyDown(key_event);
  const moved = model.state.execution_layout.value.groups[0];
  assert.equal(moved.x + moved.groups[0].x + moved.groups[0].nodes[0].x,
    first.x + nested.x + nested.nodes[0].x - 10);
  assert.equal(moved.width, first.width, "moving the only child shifts its containers without changing their width");
  const unchanged = model.state.execution_layout.value.groups[1];
  for (const key of ["x", "y", "width", "height"]) assert.equal(unchanged[key], untouched[key], "another invocation retains its layout");
  assert.deepEqual(JSON.parse(JSON.stringify(unchanged.node_positions)), JSON.parse(JSON.stringify(untouched.node_positions)));
  const listeners = new Map();
  context.document = {
    addEventListener(name, handler) { listeners.set(name, handler); },
    removeEventListener(name) { listeners.delete(name); },
  };
  const heading = find_view(graph, `automation-subflow-heading-${nested.id}`);
  heading.events.onMouseDown({ button: 0, clientX: 0, clientY: 0,
    currentTarget: { parentElement: { style: {}, parentElement: { querySelectorAll: () => [] } } }, preventDefault() {}, stopPropagation() {},
  });
  listeners.get("mousemove")({ clientX: 30, clientY: 20 });
  listeners.get("mouseup")({ clientX: 30, clientY: 20 });
  await model.methods.saveFlow();
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
  assert.equal(saved.nodes[0].config.flow_id, "reusable");
  assert.equal(saved.nodes[0].config.workflow, undefined);
  assert.deepEqual(saved.nodes[0].position, { x: 220, y: 120 });
  assert.deepEqual(saved.nodes[0].config.node_positions[nested.id].a, { x: -10, y: 0 });
  assert.deepEqual(saved.subflows.reusable.nodes.nested.config.workflow.nodes.a.position, { x: 0, y: 0 });
  assert.equal(saved.nodes[1].config.node_positions, undefined);
  model.methods.destroy();
});

test("subflow dragging preserves pending inspector text and does not edit referenced pipelines", async () => {
  const referenced = { id: "saved", name: "Shared", definition: JSON.stringify({ start_node: "a", nodes: { a: { type: "WaitNode" } } }) };
  const flows = [referenced];
  const { model, flow } = setup({ flows });
  flow.definition = JSON.stringify({ start_node: "call", nodes: { call: { type: "WorkflowNode", config: { flow_id: "saved" } } } });
  flows.push(flow);
  await model.methods.refresh();
  model.methods.selectEditNode("call");
  model.ui.input_edit_node_config$.setValue('{"flow_id":"saved","output_key":"pending"}');
  const group = model.state.subflow_groups.value[0];
  assert.equal(model.methods.moveSubflowNode(group.id, "a", { x: 20, y: 10 }), null);
  assert.match(model.state.error.value, /先应用/);
  assert.match(model.ui.input_edit_node_config$.state.value, /pending/);
  model.ui.input_edit_node_config$.setValue('{"flow_id":"saved"}');
  assert.ok(model.methods.moveSubflowNode(group.id, "a", { x: 20, y: 10 }));
  assert.equal(model.state.edit_nodes.value[0].config.flow_id, "saved");
  assert.equal(model.state.edit_nodes.value[0].config.workflow, undefined);
  assert.ok(model.state.edit_nodes.value[0].config.node_positions);
  assert.equal(JSON.parse(referenced.definition).nodes.a.position, undefined);
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
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
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
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
  assert.deepEqual(calls.find((call) => call.url.endsWith("update_flow_instance")).body.triggers, []);
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
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 2);
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
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
    changeState(connected) { states.forEach((fn) => fn({ connected })); },
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
  const node = calls.find((call) => call.url.endsWith("update_flow_instance")).body.nodes.find((node) => node.id === "end");
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body.triggers[0];
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

test("configuration JSON is copied and pasted into a dialog that saves only the current configuration", async () => {
  const { model, flow, calls, copied_texts } = setup({ mode: "list" });
  add_config_declarations(flow);
  const original = flow.node_configs;
  const original_definition = flow.definition;
  const original_name = flow.name;
  await model.methods.copyPipelineConfig(flow);
  const text = copied_texts[0];
  assert.deepEqual(JSON.parse(text), JSON.parse(original), "copies retain secrets and false/zero values");
  assert.deepEqual(calls.find((call) => call.url.endsWith("get_flow_instance")).body, { id: flow.id });
  flow.node_configs = JSON.stringify({ start: { apikey: "different" } });
  const config_menu = model.methods.pipelineConfigMenu(flow);
  let menu_closed = false;
  config_menu.hide = () => {
    assert.equal(model.ui.config_json_dialog$.state.visible, false, "close the menu before opening the paste dialog");
    menu_closed = true;
  };
  config_menu.state.items.find((item) => item.label === "粘贴JSON").click();
  assert.equal(menu_closed, true);
  assert.equal(model.ui.config_json_dialog$.state.visible, true);
  assert.equal(model.ui.input_config_json$.state.value, "");
  model.ui.input_config_json$.setValue(`\uFEFF${text}`);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
  await model.methods.savePastedPipelineConfig();
  assert.equal(model.ui.config_dialog$.state.visible, false);
  assert.equal(model.state.config_error.value, "");
  assert.equal(model.ui.config_json_dialog$.state.exit, true);
  assert.deepEqual(JSON.parse(flow.node_configs), JSON.parse(original));
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
  assert.deepEqual(Object.keys(saved).sort(), ["id", "node_configs"]);
  assert.equal(saved.id, flow.id);
  assert.equal(flow.definition, original_definition);
  assert.equal(flow.name, original_name);
  assert.equal(calls.some((call) => /\/(create_flow|import_flow)$/.test(call.url)), false);
  model.methods.destroy();
});

test("pasted configuration JSON merges partial values and rejects invalid input without saving", async () => {
  const { model, flow, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  const definition = JSON.parse(flow.definition);
  definition.nodes.end.config_schema.push(
    { key: "list", type: "array" },
    { key: "method", type: "string", options: [{ label: "GET", value: "GET" }] },
  );
  flow.definition = JSON.stringify(definition);
  for (const values of [[], null, JSON.parse(flow.definition), { ...flow }, { missing: {} }, { end: [] }, { end: { missing: "x" } },
    { start: { apikey: "changed" }, end: { count: 0.5 } }, { end: { enabled: "false" } },
    { end: { count: "1" } }, { end: { apikey: 1 } }, { end: { options: [] } },
    { end: { list: {} } }, { end: { method: "POST" } }]) {
    model.methods.openPastePipelineConfig(flow);
    model.ui.input_config_json$.setValue(JSON.stringify(values));
    await model.methods.savePastedPipelineConfig();
    assert.ok(model.state.config_error.value, JSON.stringify(values));
    if (model.state.config_groups.value.length) assert.equal(model.state.config_groups.value[0].fields[0].input$.state.value, "entry-key");
  }
  model.ui.input_config_json$.setValue("{");
  await model.methods.savePastedPipelineConfig();
  assert.ok(model.state.config_error.value);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
  model.ui.input_config_json$.setValue(JSON.stringify({ end: { enabled: true, count: 3, options: { nested: [false, 0] }, list: ["x"], method: "GET" } }));
  await model.methods.savePastedPipelineConfig();
  assert.equal(model.state.config_error.value, "");
  assert.equal(model.state.config_groups.value[0].fields[0].input$.state.value, "entry-key");
  assert.equal(model.state.config_groups.value[1].fields[2].input$.state.checked, true);
  assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 1);
  assert.deepEqual(JSON.parse(flow.node_configs).end, { apikey: "request-key", count: 3, enabled: true, options: { nested: [false, 0] }, list: ["x"], method: "GET" });
  model.methods.destroy();
});

test("paste save failures retain the textarea and the original configuration", async () => {
  const { model, flow } = setup({ mode: "list", reject_save: true });
  add_config_declarations(flow);
  const original = flow.node_configs;
  const text = JSON.stringify({ end: { count: 5 } });
  model.methods.openPastePipelineConfig(flow);
  model.ui.input_config_json$.setValue(text);
  await model.methods.savePastedPipelineConfig();
  assert.match(model.state.config_error.value, /invalid Cron/);
  assert.equal(flow.node_configs, original);
  assert.equal(model.ui.input_config_json$.state.value, text);
  assert.equal(model.ui.config_json_dialog$.state.visible, true);
  assert.equal(model.ui.config_json_dialog$.state.exit, false);
  assert.equal(model.ui.btn_config_json_save$.state.loading, false);
  model.methods.destroy();
});

test("the paste dialog renders a textarea and saves its input directly", async () => {
  const { model, flow, context, calls } = setup({ mode: "list" });
  add_config_declarations(flow);
  model.methods.openPastePipelineConfig(flow);
  load_canvas_components(context);
  vm.runInContext(fs.readFileSync(path.join(frontend, "src/pages/automation.js"), "utf8")
    .replace(/^import[\s\S]*?;\n/gm, "").replace(/^export default .*;$/gm, ""), context);
  const dialog = context.AutomationPasteConfigDialog({ store: model });
  const textarea = find_view(dialog, "automation-paste-config-json");
  assert.ok(textarea);
  assert.equal(textarea.t, "textarea");
  textarea.events.onChange({ target: { value: JSON.stringify({ end: { count: 7 } }) } });
  await model.methods.savePastedPipelineConfig();
  assert.equal(calls.find((call) => call.url.endsWith("update_flow_instance")).body.node_configs.end.count, 7);
  assert.equal(model.ui.config_dialog$.state.visible, false);
  model.methods.destroy();
});

test("copying configuration reports clipboard errors without a success notice", async () => {
  const { model, flow, app, tips } = setup({ mode: "list" });
  app.copy = () => { throw new Error("clipboard denied"); };
  await model.methods.copyPipelineConfig(flow);
  assert.equal(tips.at(-1).type, "error");
  assert.match(tips.at(-1).msg[0], /clipboard denied/);
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body;
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
    assert.equal(calls.filter((call) => call.url.endsWith("update_flow_instance")).length, 0);
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body.nodes.find((node) => node.id === "end");
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body.node_configs.end;
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
  const saved = calls.find((call) => call.url.endsWith("update_flow_instance")).body.node_configs.end;
  assert.equal(saved.push_method, "webhook");
  assert.equal(saved.message_template, "账号：{username}\n新视频：{title}");
  model.methods.destroy();
});

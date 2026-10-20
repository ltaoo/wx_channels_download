import { track_error } from "@/utils.js";
import {
  fetchPipelineList,
  createPipeline,
  updatePipeline,
  deletePipeline,
  fetchPipelineDetail,
  fetchPipelineNodeCatalog,
  triggerPipeline,
  importPipeline,
  fetchScheduleList,
  triggerSchedule,
  fetchPipelineRunList,
  fetchPipelineRunDetail,
  cancelPipelineRun,
  resumePipelineRun,
  retryPipelineRun,
} from "@/biz/request.js";
import { PipelineEditorBridge } from "./pipeline_editor.model.js";

const AutomationChannelCore = Timeless.kit.ChannelCore;
const automation_socket_client = window.__store.socket_client$;
const automation_execution_log_limit = 200;

function parse_initial_data(value) {
  const trimmed = String(value || "").trim();
  if (!trimmed) return {};
  const parsed = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("初始数据必须是 JSON 对象");
  }
  return parsed;
}

function schedule_metadata(schedule) {
  const metadata = { type: "Cron", start_node: "", event_key: "", active: true, webhook_token: "" };
  if (!schedule) return metadata;
  try {
    const data = parse_initial_data(schedule.initial_data);
    const raw = data.__automation;
    if (raw && typeof raw === "object") {
      if (raw.type) metadata.type = String(raw.type);
      if (raw.start_node) metadata.start_node = String(raw.start_node);
      if (raw.event_key) metadata.event_key = String(raw.event_key);
      metadata.active = raw.active !== false;
      metadata.webhook_token = String(raw.webhook_token || "");
    }
  } catch (err) {
    void err;
  }
  if (!["Cron", "Event", "Manual", "Webhook"].includes(metadata.type)) {
    metadata.type = "Cron";
  }
  return metadata;
}

function trigger_label(type) {
  if (type === "Webhook") return "Webhook";
  if (type === "Event") return "事件触发";
  if (type === "Manual") return "手动触发";
  return "定时触发";
}

function schema_text(schema) {
  if (!Array.isArray(schema) || schema.length === 0) return "无";
  return schema
    .map((field) => `${field.key}:${field.type || "any"}`)
    .join("、");
}

function format_time(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) return "-";
  const date = new Date(number < 1000000000000 ? number * 1000 : number);
  if (Number.isNaN(date.getTime())) return "-";
  const pad = (input) => String(input).padStart(2, "0");
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  );
}

function flow_label(type) {
  const names = {
    StartNode: "开始",
    EndNode: "结束",
    ExprNode: "表达式",
    GatewayNode: "条件分支",
    APICallNode: "HTTP 请求",
    ServiceNode: "服务调用",
    ManualNode: "人工确认",
    FuncNode: "函数",
    LoopNode: "数组迭代",
    WaitNode: "延时等待",
    FailNode: "终止并报错",
    WorkflowNode: "子流程",
    JSCodeNode: "执行 JS",
    SetVariableNode: "设置变量",
  };
  return names[type] || type;
}

function flow_icon(type) {
  const icons = {
    StartNode: "play",
    EndNode: "square",
    ExprNode: "braces",
    GatewayNode: "git-branch",
    APICallNode: "globe",
    ServiceNode: "wrench",
    ManualNode: "user",
    FuncNode: "settings",
    LoopNode: "refresh-cw",
    WaitNode: "clock",
    FailNode: "circle-alert",
    WorkflowNode: "git-fork",
    JSCodeNode: "file-code",
    SetVariableNode: "pen-line",
  };
  return icons[type] || "box";
}

function request_result_error(result, fallback) {
  const error = result && result.error;
  if (error instanceof Error) return error;
  if (error && error.message) return new Error(error.message);
  return new Error(String(error || fallback || "请求失败"));
}

function calculate_pipeline_node_positions(nodes) {
  const list = Array.isArray(nodes) ? nodes : [];
  const by_id = new Map(list.map((node) => [node.id, node]));
  const depths = new Map();
  const roots = list.filter((node) => node.type === "StartNode");
  if (roots.length === 0 && list.length > 0) roots.push(list[0]);
  const queue = roots.map((node) => ({ id: node.id, depth: 0 }));

  while (queue.length > 0) {
    const current = queue.shift();
    if (!current || !by_id.has(current.id)) continue;
    const known_depth = depths.get(current.id);
    if (known_depth !== undefined && known_depth <= current.depth) continue;
    depths.set(current.id, current.depth);
    const node = by_id.get(current.id);
    (node.next_ids || []).forEach((next_id) => {
      if (by_id.has(next_id)) {
        queue.push({ id: next_id, depth: current.depth + 1 });
      }
    });
  }

  const per_depth = new Map();
  list.forEach((node) => {
    const depth = depths.has(node.id) ? depths.get(node.id) : 0;
    const layer = per_depth.get(depth) || [];
    layer.push(node.id);
    per_depth.set(depth, layer);
  });

  const positions = {};
  per_depth.forEach((ids, depth) => {
    ids.forEach((node_id, index) => {
      positions[node_id] = {
        x: 40 + depth * 260,
        y: 40 + index * 130,
      };
    });
  });
  list.forEach((node) => {
    const position = node.position;
    const x = position && Number(position.x);
    const y = position && Number(position.y);
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;
    positions[node.id] = { x, y };
  });
  return positions;
}

// —— 条件分支（GatewayNode/Exclusive）的 rules 同步 ——
// 引擎只按 config.rules 顺序取第一条成立的条件跳到 target_id，完全不看 next_ids。
// 所以画布上插/删节点时必须把 rules 里的 target_id 一起改掉，否则连线看起来接上了，
// 引擎仍然按旧 rules 走，新节点永远不会被执行。

function gateway_rules_of(node) {
  if (!node || node.type !== "GatewayNode") return null;
  const config = node.config || {};
  if (String(config.gateway_type || "Exclusive") !== "Exclusive") return null;
  const rules = config.rules;
  if (!Array.isArray(rules) || rules.length === 0) return null;
  return rules;
}

// a→b 之间插入新节点：把指向 b 的规则改成指向新节点（条件保持不变）
function remap_gateway_rules(node, to_id, new_id) {
  const rules = gateway_rules_of(node);
  if (!rules) return node ? node.config : undefined;
  const target = String(to_id || "");
  let changed = false;
  const next_rules = rules.map((rule) => {
    if (!rule || typeof rule !== "object") return rule;
    if (String(rule.target_id || rule.target || "") !== target) return rule;
    changed = true;
    return { ...rule, target_id: new_id };
  });
  return changed ? { ...(node.config || {}), rules: next_rules } : node.config;
}

// 删除节点：把它从 rules 里摘掉，避免留下指向不存在节点的悬空分支
function drop_gateway_rules(node, node_id) {
  const rules = gateway_rules_of(node);
  if (!rules) return node ? node.config : undefined;
  const removed = String(node_id || "");
  const next_rules = rules.filter(
    (rule) =>
      !rule ||
      typeof rule !== "object" ||
      String(rule.target_id || rule.target || "") !== removed,
  );
  if (next_rules.length === rules.length) return node.config;
  return { ...(node.config || {}), rules: next_rules };
}

// 用户改完网关的 rules 后，连线（next_ids）跟着 rules 走；rules 为空时保持原样，
// 方便先画线再补条件
function gateway_next_ids_from_rules(node, config) {
  if (!node || node.type !== "GatewayNode") return null;
  if (String((config && config.gateway_type) || "Exclusive") !== "Exclusive") {
    return null;
  }
  const rules = config && config.rules;
  if (!Array.isArray(rules) || rules.length === 0) return null;
  const next_ids = [];
  rules.forEach((rule) => {
    const target = String(
      (rule && (rule.target_id || rule.target)) || "",
    ).trim();
    if (target && !next_ids.includes(target)) next_ids.push(target);
  });
  return next_ids.length > 0 ? next_ids : null;
}

// insert_node_positions 为「在 a、b 之间插入」腾位置：新节点落在 b 原来那一列（与 a 同高），
// 原本处在 b 这一列及更右的所有节点（含其它分支）统一右移一个间距，这样整张图仍然保持在
// 同一个网格上，而不是只有被插入的那条分支单独错开。
function insert_node_positions(nodes, from_id, to_id, new_id) {
  const positions = calculate_pipeline_node_positions(nodes);
  const from_position = positions[from_id];
  const to_position = positions[to_id];
  if (!from_position || !to_position) return positions;

  const gap = 260;
  Object.keys(positions).forEach((node_id) => {
    const position = positions[node_id];
    if (position && position.x >= to_position.x) {
      positions[node_id] = { x: position.x + gap, y: position.y };
    }
  });
  positions[new_id] = { x: to_position.x, y: from_position.y };
  return positions;
}

function calculate_flow_minimap_geometry(
  layout,
  viewport_size,
  viewport,
  minimap_size = {},
) {
  const minimap_width = Math.max(1, Number(minimap_size.width) || 176);
  const minimap_height = Math.max(1, Number(minimap_size.height) || 112);
  const padding = Math.max(0, Number(minimap_size.padding) || 8);
  const zoom = Math.max(0.01, Number(viewport && viewport.zoom) || 1);
  const viewport_width = Math.max(
    1,
    Number(viewport_size && viewport_size.width) || 1,
  );
  const viewport_height = Math.max(
    1,
    Number(viewport_size && viewport_size.height) || 1,
  );
  // 世界原点：节点可以拖到画布原点左上（负坐标），
  // 世界范围 = [world_left, 内容右边界]，宽度按跨度计算
  const world_left = Math.min(0, Number(layout && layout.min_left) || 0);
  const world_top = Math.min(0, Number(layout && layout.min_top) || 0);
  const world_width = Math.max(
    1,
    (Number(layout && layout.width) || 1) - world_left,
    viewport_width / zoom,
  );
  const world_height = Math.max(
    1,
    (Number(layout && layout.height) || 1) - world_top,
    viewport_height / zoom,
  );
  const content_width = Math.max(1, minimap_width - padding * 2);
  const content_height = Math.max(1, minimap_height - padding * 2);
  const scale = Math.min(
    content_width / world_width,
    content_height / world_height,
    1,
  );
  const offset_x = (minimap_width - world_width * scale) / 2;
  const offset_y = (minimap_height - world_height * scale) / 2;
  const viewport_x = Number(viewport && viewport.x) || 0;
  const viewport_y = Number(viewport && viewport.y) || 0;
  // The viewport rect is kept unclipped: it only depends on the viewport size
  // and zoom, so panning slides it without changing its dimensions. The
  // minimap clips the overflow.
  return {
    width: minimap_width,
    height: minimap_height,
    scale,
    offset_x,
    offset_y,
    world_width,
    world_height,
    world_left,
    world_top,
    viewport: {
      left: offset_x + (-viewport_x / zoom - world_left) * scale,
      top: offset_y + (-viewport_y / zoom - world_top) * scale,
      width: (viewport_width / zoom) * scale,
      height: (viewport_height / zoom) * scale,
    },
  };
}

function normalize_pipeline(value) {
  const pipeline = value && typeof value === "object" ? value : {};
  let definition = {};
  try {
    definition =
      typeof pipeline.definition === "string"
        ? JSON.parse(pipeline.definition)
        : pipeline.definition || {};
  } catch (err) {
    void err;
  }

  const raw_nodes = definition.nodes || {};
  const node_entries = Array.isArray(raw_nodes)
    ? raw_nodes.map((node, index) => [
        node && node.id ? node.id : String(index),
        node,
      ])
    : Object.entries(raw_nodes);
  const nodes = node_entries.map(([node_id, raw_node]) => {
    const node = raw_node && typeof raw_node === "object" ? raw_node : {};
    const next_ids = [];
    const append_next_id = (target_id) => {
      const normalized = String(target_id || "").trim();
      if (normalized && !next_ids.includes(normalized)) next_ids.push(normalized);
    };
    (node.next_node_ids || node.next_ids || []).forEach(append_next_id);
    (node.next_nodes || []).forEach((target) => {
      append_next_id(target && (target.target_id || target.targetId));
    });
    if (node.type === "GatewayNode") {
      const rules = node.config && node.config.rules;
      (Array.isArray(rules) ? rules : []).forEach((rule) => {
        append_next_id(rule && (rule.target_id || rule.target));
      });
    }
    const raw_position = node.position;
    const position_x = raw_position && Number(raw_position.x);
    const position_y = raw_position && Number(raw_position.y);
    const position =
      raw_position &&
      Number.isFinite(position_x) &&
      Number.isFinite(position_y)
        ? { x: position_x, y: position_y }
        : null;
    return {
      id: node.id || node_id,
      type: node.type || "",
      name: node.name || node.id || node_id,
      config: node.config || {},
      position,
      next_ids,
      input_schema: node.input_schema || [],
      output_schema: node.output_schema || [],
      retry: node.retry || null,
      error_next_node: node.error_next_node || "",
    };
  });
  const edges = [];
  nodes.forEach((node) => {
    node.next_ids.forEach((target_id, index) => {
      edges.push({
        id: `${node.id}-${target_id}-${index}`,
        from: node.id,
        to: target_id,
        type: "next",
      });
    });
  });

  return {
    ...pipeline,
    id: pipeline.id || definition.id || "",
    name: pipeline.name || definition.name || "",
    start_node_id:
      definition.start_node ||
      definition.start_node_id ||
      pipeline.start_node_id ||
      "",
    context_schema: definition.context_schema || pipeline.context_schema || [],
    nodes,
    edges,
  };
}

function pipeline_triggers(pipeline, schedules) {
  if (!pipeline) return [];
  const triggers = (schedules || [])
    .filter((schedule) => schedule.flow_id === pipeline.id)
    .map((schedule) => {
      const metadata = schedule_metadata(schedule);
      let initial_data = {};
      try {
        initial_data = typeof schedule.initial_data === "string"
          ? JSON.parse(schedule.initial_data || "{}")
          : { ...schedule.initial_data };
      } catch { /* Keep malformed legacy input editable. */ }
      if (!initial_data || typeof initial_data !== "object" || Array.isArray(initial_data)) initial_data = {};
      delete initial_data.__automation;
      return {
        id: schedule.id, type: metadata.type, name: schedule.name,
        cron_expr: schedule.cron_expr, event_key: metadata.event_key,
        enabled: schedule.enabled, next_run_at: schedule.next_run_at,
        active: metadata.active, webhook_token: metadata.webhook_token,
        initial_data, timeout_sec: schedule.timeout_sec,
      };
    });
  // Older event pipelines stored only a single event key on UserFlow.
  // Surface it as an unsaved rule; the next save moves it into schedules.
  if (pipeline.trigger_type === "Event" && pipeline.event_key &&
      !triggers.some((trigger) => trigger.type === "Event" && trigger.event_key === pipeline.event_key)) {
    triggers.push({ id: "", type: "Event", name: "事件触发", event_key: pipeline.event_key, initial_data: {} });
  }
  return triggers;
}

function automation_view_query(props, fallback_mode) {
  const query = (props && props.view && props.view.query) || {};
  const requested_mode = String(
    query.mode || fallback_mode || "list",
  ).toLowerCase();
  return {
    mode: ["detail", "edit"].includes(requested_mode)
      ? requested_mode
      : "list",
    flow_id: String(query.id || query.flow_id || "").trim(),
  };
}

function AutomationPageViewModel(props, options) {
  const view_query = automation_view_query(props, options && options.mode);
  const view_mode_ = ref(view_query.mode);
  const pipelines_ = refarr([]);
  const filtered_pipelines_ = refarr([]);
  const keyword_ = ref("");
  const search_keyword_ = ref("");
  const page_ = ref(1);
  const page_size_ = ref(50);
  const total_ = computed(filtered_pipelines_, (pipelines) => pipelines.length);
  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    ({ total, pageSize }) => Math.max(1, Math.ceil(total / pageSize)),
  );
  const paged_pipelines_ = combine(
    { pipelines: filtered_pipelines_, page: page_, pageSize: page_size_ },
    ({ pipelines, page, pageSize }) => {
      const start = (page - 1) * pageSize;
      return pipelines.slice(start, start + pageSize);
    },
  );
  const range_text_ = combine(
    { total: total_, page: page_, pageSize: page_size_ },
    ({ total, page, pageSize }) => {
      if (!total) return "共 0 条";
      const start = (page - 1) * pageSize + 1;
      return `第 ${start}-${Math.min(page * pageSize, total)} 条，共 ${total} 条`;
    },
  );
  const catalog_ = refarr([]);
  const schedules_ = refarr([]);
  const selected_flow_id_ = ref(view_query.flow_id);
  const selected_pipeline_ = refobj(null);
  const loading_ = ref(false);
  const error_ = track_error(ref(""), props.tip);
  const notice_ = ref("");
  const dirty_ = ref(false);
  const saving_ = ref(false);
  const execution_channel_connected_ = ref(false);
  const execution_run_id_ = ref("");
  const execution_run_status_ = ref("");
  const runs_ = refarr([]);
  const selected_run_ = refobj(null);
  const run_action_busy_ = ref(false);
  const manual_response_ = ref("{}");
  const edit_node_policy_ = ref("{}");
  let run_poll_timer = null;
  let run_query_sequence = 0;
  let run_detail_sequence = 0;
  let selected_run_id = "";
  const node_execution_states_ = refobj({});
  const execution_logs_ = refarr([]);
  // 画布节点执行日志浮窗：可同时存在多个，各自位置独立，层级由 WindowManager 维护
  const execution_windows_ = refarr([]);
  const execution_window_z_ = refobj({});
  const execution_window_active_id_ = ref("");
  const execution_windows = new Timeless.vm.WindowManager({ baseZ: 240 });
  execution_windows.onStateChange(sync_execution_windows);

  // editor working copy: [{id,type,name,config,next_ids,input_schema}]
  const edit_name_ = ref("");
  const edit_nodes_ = refarr([]);
  const edit_triggers_ = refarr([]);
  const edit_start_node_id_ = ref("");
  const selected_edit_node_id_ = ref("");
  const node_inspector_open_ = ref(false);
  const edit_node_config_ = ref("{}");
  const staged_node_positions = new Map();

  // create-pipeline dialog state
  const create_open_ = ref(false);
  const create_name_ = ref("");
  const create_description_ = ref("");
  const create_params_ = refarr([]);
  const create_submitting_ = ref(false);

  // add-node dialog state
  const add_open_ = ref(false);
  const add_type_ = ref("");
  const add_name_ = ref("");
  const add_config_ = ref("{}");
  const add_service_tool_ = ref("");
  const add_service_form_ = refobj(null);
  const add_service_form_schema_ = refarr([]);
  const add_service_form_error_ = track_error(ref(""), props.tip);
  const add_from_ = ref("");
  // 非空即「插入」模式：在该节点与其 next 之间插入新节点
  const add_insert_to_ = ref("");
  const add_submitting_ = ref(false);

  // run-pipeline dialog state
  const run_open_ = ref(false);
  const run_submitting_ = ref(false);
  const run_params_ = refarr([]);
  const run_error_ = track_error(ref(""), props.tip);

  // import-pipeline dialog state
  const import_open_ = ref(false);
  const import_submitting_ = ref(false);
  const import_json_ = ref("");
  const import_error_ = track_error(ref(""), props.tip);

  // list-row delete state
  const pending_delete_flow_id_ = ref("");

  let request_sequence = 0;
  let node_sequence = 0;
  let param_sequence = 0;
  let run_param_sequence = 0;
  let trigger_sequence = 0;
  let disposed = false;

  const execution_channel = new AutomationChannelCore(
    "/ws/v1/automation",
    {
      client: automation_socket_client,
      process: decode_automation_channel_message,
      reconnect: {
        enabled: true,
        interval: 1000,
      },
    },
  );
  execution_channel.onMessage(handle_automation_channel_message);
  execution_channel.onStateChange((channel_state) => {
    execution_channel_connected_.as(Boolean(channel_state.connected));
  });

  const run_request = (fn) =>
    new Timeless.kit.RequestCore(fn, { client: props.client });
  const reqs = {
    run: {
      list: run_request(fetchPipelineRunList),
      detail: run_request(fetchPipelineRunDetail),
      cancel: run_request(cancelPipelineRun),
      resume: run_request(resumePipelineRun),
      retry: run_request(retryPipelineRun),
    },
    pipeline: {
      list: new Timeless.kit.RequestCore(
        fetchPipelineList,
        { client: props.client },
      ),
      create: new Timeless.kit.RequestCore(
        createPipeline,
        { client: props.client },
      ),
      update: new Timeless.kit.RequestCore(
        updatePipeline,
        { client: props.client },
      ),
      delete: new Timeless.kit.RequestCore(
        deletePipeline,
        { client: props.client },
      ),
      detail: new Timeless.kit.RequestCore(
        fetchPipelineDetail,
        { client: props.client },
      ),
      catalog: new Timeless.kit.RequestCore(
        fetchPipelineNodeCatalog,
        { client: props.client },
      ),
      trigger: new Timeless.kit.RequestCore(
        triggerPipeline,
        { client: props.client },
      ),
      import: new Timeless.kit.RequestCore(
        importPipeline,
        { client: props.client },
      ),
    },
    schedule: {
      list: new Timeless.kit.RequestCore(
        fetchScheduleList,
        { client: props.client },
      ),
      trigger: new Timeless.kit.RequestCore(
        triggerSchedule,
        { client: props.client },
      ),
    },
  };

  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "搜索名称",
      type: "search",
      allowClear: true,
      onChange(value) {
        keyword_.as(String(value || ""));
      },
      onEnter() {
        return search_pipelines();
      },
    }),
    btn_search$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: loading_.value,
    }),
    btn_reset$: new Timeless.vm.ButtonCore({
      variant: "outline",
      disabled: loading_.value,
      onClick() {
        ui.input_keyword$.setValue("");
        search_pipelines();
        return load_data({ silent: true });
      },
    }),
    create_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    add_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    run_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    import_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    delete_dialog$: new Timeless.vm.DialogCore({
      onOk() {
        return delete_flow();
      },
    }),
    select_add_type$: new Timeless.vm.SelectCore({
      placeholder: "选择节点类型",
      position: "popper",
      options: [],
      onChange(value) {
        add_type_.as(String(value || ""));
        const default_name = flow_label(add_type_.value);
        const default_config = default_config_json(add_type_.value);
        add_name_.as(default_name);
        add_config_.as(default_config);
        ui.input_add_name$.setValue(default_name, {
          silence: true,
        });
        ui.input_add_config$.setValue(default_config, {
          silence: true,
        });
        add_service_tool_.as("");
        add_service_form_.as(null);
        add_service_form_schema_.as([], { reset: true });
        add_service_form_error_.as("");
        ui.select_add_service_tool$.setValue("", { silence: true });
      },
    }),
    select_add_service_tool$: new Timeless.vm.SelectCore({
      placeholder: "选择 Service tool",
      position: "popper",
      options: [],
      onChange(value) {
        const tool_name = String(value || "");
        add_service_tool_.as(tool_name);
        select_service_tool(tool_name);
      },
    }),
    select_add_from$: new Timeless.vm.SelectCore({
      placeholder: "连接自（上一个节点）",
      position: "popper",
      options: [],
    }),
    input_edit_name$: new Timeless.vm.InputCore({
      placeholder: "Pipeline 标题",
      onChange(value) {
        edit_name_.as(String(value || ""));
        dirty_.as(true);
      },
    }),
    input_create_name$: new Timeless.vm.InputCore({
      placeholder: "Pipeline 名称",
      onChange(value) {
        create_name_.as(value);
      },
    }),
    input_create_description$: new Timeless.vm.InputCore({
      placeholder: "可选",
      onChange(value) {
        create_description_.as(value);
      },
    }),
    input_add_name$: new Timeless.vm.InputCore({
      placeholder: "节点名称",
      onChange(value) {
        add_name_.as(value);
      },
    }),
    input_add_config$: new Timeless.vm.InputCore({
      placeholder: "{}",
      onChange(value) {
        add_config_.as(value);
      },
    }),
    input_edit_node_name$: new Timeless.vm.InputCore({
      placeholder: "节点名称",
      onChange(value) {
        update_selected_node({ name: String(value || "") });
      },
    }),
    input_manual_response$: new Timeless.vm.InputCore({ defaultValue: "{}", onChange(value) { manual_response_.as(value); } }),
    input_edit_node_policy$: new Timeless.vm.InputCore({ defaultValue: "{}", onChange(value) { edit_node_policy_.as(value); } }),
    btn_refresh_runs$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return load_runs(); } }),
    btn_select_run$: new Timeless.vm.ButtonInListCore({ variant: "ghost", onClick(run) { return select_run(run.id); } }),
    btn_cancel_run$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return act_on_run("cancel"); } }),
    btn_resume_run$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return act_on_run("resume"); } }),
    btn_retry_run$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return act_on_run("retry"); } }),
    btn_retry_current_run$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { return act_on_run("retry_current"); } }),
    input_edit_node_config$: new Timeless.vm.InputCore({
      defaultValue: "{}",
      placeholder: "{}",
      onChange(value) {
        edit_node_config_.as(String(value || ""));
      },
    }),
    input_import_json$: new Timeless.vm.InputCore({
      defaultValue: "",
      onChange(value) {
        import_json_.as(value);
      },
    }),
    btn_refresh$: new Timeless.vm.ButtonCore({
      variant: "outline",
      disabled: loading_.value,
      onClick() {
        return load_data({ silent: true });
      },
    }),
    btn_create_pipeline$: new Timeless.vm.ButtonCore({
      variant: "default",
      onClick() {
        return open_create_dialog();
      },
    }),
    btn_import_pipeline$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        return open_import_dialog();
      },
    }),
    btn_create_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: create_submitting_.value,
      onClick() {
        return submit_pipeline_create();
      },
    }),
    btn_create_cancel$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        ui.create_dialog$.hide();
      },
    }),
    btn_add_node$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      onClick() {
        return open_add_dialog();
      },
    }),
    btn_close_node_inspector$: new Timeless.vm.ButtonCore({
      variant: "outline", size: "sm",
      onClick() { node_inspector_open_.as(false); },
    }),
    btn_add_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: add_submitting_.value,
      onClick() {
        return submit_add_node();
      },
    }),
    btn_apply_node_config$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      onClick() {
        return apply_selected_node_config();
      },
    }),
    btn_remove_selected_node$: new Timeless.vm.ButtonCore({
      variant: "danger",
      size: "sm",
      onClick() {
        return remove_node(selected_edit_node_id_.value);
      },
    }),
    btn_add_cancel$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        ui.add_dialog$.hide();
      },
    }),
    btn_save_flow$: new Timeless.vm.ButtonCore({
      variant: "primary",
      size: "sm",
      disabled: true,
      onClick() {
        return save_flow();
      },
    }),
    btn_run_flow$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      onClick() {
        return trigger_flow();
      },
    }),
    btn_run_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: run_submitting_.value,
      onClick() {
        return submit_run();
      },
    }),
    btn_run_cancel$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        ui.run_dialog$.hide();
      },
    }),
    btn_import_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: import_submitting_.value,
      onClick() {
        return submit_import();
      },
    }),
    btn_import_cancel$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        ui.import_dialog$.hide();
      },
    }),
    btn_add_cron_trigger$: new Timeless.vm.ButtonCore({
      variant: "outline", size: "sm",
      onClick() { add_trigger("Cron"); },
    }),
    btn_add_webhook_trigger$: new Timeless.vm.ButtonCore({ variant: "outline", onClick() { add_trigger("Webhook"); } }),
    btn_add_event_trigger$: new Timeless.vm.ButtonCore({
      variant: "outline", size: "sm",
      onClick() { add_trigger("Event"); },
    }),
    btn_remove_trigger$: new Timeless.vm.ButtonInListCore({
      variant: "danger", size: "xs",
      onClick(trigger) {
        edit_triggers_.as(edit_triggers_.value.filter((item) => item.uid !== trigger.uid), { reset: true });
        dirty_.as(true);
      },
    }),
    btn_schedule_trigger$: new Timeless.vm.ButtonInListCore({
      variant: "outline",
      size: "xs",
      onClick(schedule) {
        return trigger_schedule(schedule && schedule.id);
      },
    }),
    btn_pipeline_delete$: new Timeless.vm.ButtonInListCore({
      variant: "danger",
      size: "xs",
      onClick(pipeline) {
        return ask_delete_pipeline(pipeline);
      },
    }),
    btn_param_remove$: new Timeless.vm.ButtonInListCore({
      variant: "ghost",
      size: "sm",
      onClick(param) {
        remove_create_param(param);
      },
    }),
    btn_param_add$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      onClick() {
        add_create_param();
      },
    }),
  };

  if (ui.delete_dialog$.okBtn) {
    ui.delete_dialog$.okBtn.setVariant("destructive");
  }
  loading_.subscribe({
    onChange(loading) {
      [ui.btn_refresh$, ui.btn_search$, ui.btn_reset$].forEach((button) => {
        if (loading) button.disable();
        else button.enable();
      });
    },
  });
  pipelines_.subscribe({ onChange: filter_pipelines });
  search_keyword_.subscribe({ onChange: filter_pipelines });

  function filter_pipelines() {
    const keyword = search_keyword_.value.toLowerCase();
    filtered_pipelines_.as(
      pipelines_.value.filter((pipeline) =>
        String(pipeline.name || "").toLowerCase().includes(keyword),
      ),
      { reset: true },
    );
    page_.as(Math.min(page_.value, page_count_.value));
  }

  function search_pipelines() {
    if (loading_.value) return;
    page_.as(1);
    search_keyword_.as(keyword_.value.trim());
  }

  function change_page(target_page) {
    if (loading_.value) return;
    const page = Math.min(
      page_count_.value,
      Math.max(1, Math.floor(Number(target_page) || 1)),
    );
    page_.as(page);
  }
  create_submitting_.subscribe({
    onChange(submitting) {
      if (submitting) ui.btn_create_submit$.disable();
      else ui.btn_create_submit$.enable();
    },
  });
  add_submitting_.subscribe({
    onChange(submitting) {
      if (submitting) ui.btn_add_submit$.disable();
      else ui.btn_add_submit$.enable();
    },
  });
  dirty_.subscribe({
    onChange(dirty) {
      if (dirty) ui.btn_save_flow$.enable();
      else ui.btn_save_flow$.disable();
    },
  });
  catalog_.subscribe({
    onChange(catalog) {
      ui.select_add_type$.setOptions(
        catalog.map(
          (item) =>
            new Timeless.vm.SelectItemCore({
              label: `${item.name}（${item.type}）`,
              value: item.type,
            }),
        ),
      );
      const service_node = catalog.find(
        (item) => item.type === "ServiceNode",
      );
      ui.select_add_service_tool$.setOptions(
        ((service_node && service_node.tools) || []).map(
          (tool) =>
            new Timeless.vm.SelectItemCore({
              label: `${tool.title || tool.name}（${tool.name}）`,
              value: tool.name,
            }),
        ),
      );
    },
  });
  edit_nodes_.subscribe({
    onChange() {
      const options = (edit_nodes_.value || []).map(
        (node) =>
          new Timeless.vm.SelectItemCore({
            label: `${node.name || node.id}（${node.id}）`,
            value: node.id,
          }),
      );
      ui.select_add_from$.setOptions(options);
    },
  });

  function default_config_json(node_type) {
    const defaults = {
      ExprNode: { expression: "", output_key: "calc_out" },
      // keys maps a request parameter name to a scope path, e.g.
      // {"username": "output.cleanup.cleanup"}.
      APICallNode: { url: "", method: "GET", headers: {}, query: {}, keys: {}, timeout_seconds: 15, fail_on_status: true, output_key: "api_response" },
      WorkflowNode: { flow_id: "", arguments: {}, output_key: "workflow_result" },
      LoopNode: { items: [], flow_id: "", arguments: {}, max_iterations: 1000, concurrency: 1, continue_on_error: false, output_key: "loop_result" },
      WaitNode: { seconds: 1 },
      FailNode: { message: "流程条件不满足" },
      ManualNode: { output_key: "response" },
      // condition_language 显式写出来，好在「原始 JSON」编辑框里被发现；
      // 老流程没有这个键，缺省即 expr。
      GatewayNode: {
        gateway_type: "Exclusive",
        condition_language: "expr",
        rules: [],
      },
      ServiceNode: {
        tool_name: "",
        arguments: {},
        output_key: "service_result",
      },
      JSCodeNode: { code: "", output_key: "" },
      SetVariableNode: { variables: {} },
    };
    const config = defaults[node_type] || {};
    return JSON.stringify(config, null, 2);
  }

  function find_service_tool(tool_name) {
    const service_node = (catalog_.value || []).find(
      (entry) => entry.type === "ServiceNode",
    );
    return (
      ((service_node && service_node.tools) || []).find(
        (tool) => tool.name === tool_name,
      ) || null
    );
  }

  // available_context_keys lists the reads a template may contain, structured as
  // { namespace, key } entries whose concatenation is the token inserted after
  // "{{". The three namespaces are disjoint and each one is the only place its
  // data can come from:
  //   input  — the pipeline's declared context schema keys
  //   output — "<node_id>.<produced_key>" for every node, so two producers
  //            writing the same key name stay distinguishable
  //   global — variable names written by a SetVariableNode
  // There is deliberately no bare-key or "ctx." suggestion: the runtime
  // resolvers reject both.
  function available_context_keys() {
    const keys = [];
    const seen = new Set();
    const push = (namespace, key) => {
      const normalized = String(key || "").trim();
      if (!normalized) return;
      const token = `${namespace}.${normalized}`;
      if (seen.has(token)) return;
      seen.add(token);
      keys.push({ namespace, key: normalized });
    };
    const pipeline = selected_pipeline_.value;
    const schema =
      pipeline && Array.isArray(pipeline.context_schema)
        ? pipeline.context_schema
        : [];
    schema.forEach((field) => push("input", field && field.key));
    (edit_nodes_.value || []).forEach((node) => {
      const node_id = node && node.id;
      if (!node_id) return;
      const config = node && node.config;
      const produced = [];
      if (config && config.output_key) produced.push(config.output_key);
      const output_schema = node && node.output_schema;
      if (Array.isArray(output_schema)) {
        output_schema.forEach((field) => produced.push(field && field.key));
      }
      if (node.type === "APICallNode") {
        produced.forEach((key) => push("output", `${node_id}.${key}_status`));
      }
      produced.forEach((key) => push("output", `${node_id}.${key}`));
      if (node.type === "SetVariableNode" && config) {
        Object.keys(config.variables || {}).forEach((key) =>
          push("global", key),
        );
      }
    });
    return keys;
  }

  function service_form_initial_value(field_schema) {
    if (field_schema.has_default) return field_schema.default;
    if (field_schema.control === "checkbox") return false;
    return "";
  }

  function service_form_input_value(field_schema, value) {
    if (
      field_schema.control === "textarea" &&
      value !== "" &&
      value !== undefined &&
      value !== null
    ) {
      return JSON.stringify(value, null, 2);
    }
    if (
      field_schema.control === "input" &&
      value !== "" &&
      value !== undefined &&
      value !== null
    ) {
      return String(value);
    }
    return value;
  }

  function service_tool_form_schema(tool) {
    const declared_schema = Array.isArray(tool && tool.form_schema)
      ? tool.form_schema
      : [];
    if (declared_schema.length > 0) return declared_schema;

    const input_schema =
      (tool && (tool.input_schema || tool.inputSchema)) || {};
    const properties =
      input_schema.properties && typeof input_schema.properties === "object"
        ? input_schema.properties
        : {};
    const required_names = new Set(
      Array.isArray(input_schema.required) ? input_schema.required : [],
    );
    return Object.entries(properties)
      .sort(([left_name], [right_name]) => {
        const left_required = required_names.has(left_name);
        const right_required = required_names.has(right_name);
        if (left_required !== right_required) return left_required ? -1 : 1;
        return left_name.localeCompare(right_name);
      })
      .map(([name, property_value]) => {
        const property =
          property_value && typeof property_value === "object"
            ? property_value
            : {};
        const options = Array.isArray(property.enum)
          ? property.enum.map((value) => ({
              label: String(value),
              value,
            }))
          : [];
        let control = "input";
        if (options.length > 0) control = "select";
        else if (property.type === "boolean") control = "checkbox";
        else if (property.type === "array" || property.type === "object") {
          control = "textarea";
        }
        const has_default = Object.prototype.hasOwnProperty.call(
          property,
          "default",
        );
        return {
          name,
          label: property.title || name,
          description: property.description || "",
          type: property.type || "string",
          format: property.format || "",
          control,
          required: required_names.has(name),
          default: property.default,
          has_default,
          minimum: property.minimum,
          maximum: property.maximum,
          options,
        };
      });
  }

  function create_service_tool_form(form_schema) {
    const context_keys = available_context_keys();
    const fields = {};
    form_schema.forEach((field_schema) => {
      const initial_value = service_form_input_value(
        field_schema,
        service_form_initial_value(field_schema),
      );

      let input;
      if (field_schema.control === "select") {
        input = new Timeless.vm.SelectCore({
          defaultValue: initial_value,
          placeholder: `选择 ${field_schema.label}`,
          position: "popper",
          options: (field_schema.options || []).map(
            (option) =>
              new Timeless.vm.SelectItemCore({
                label: option.label,
                value: option.value,
              }),
          ),
          onChange() {
            sync_service_form_config();
          },
        });
      } else if (field_schema.control === "checkbox") {
        input = new Timeless.vm.CheckboxCore({
          checked: Boolean(initial_value),
          onChange() {
            sync_service_form_config();
          },
        });
      } else {
        input = new Timeless.vm.InputCore({
          defaultValue: initial_value,
          placeholder: field_schema.description || field_schema.name,
          onChange() {
            sync_service_form_config();
          },
        });
      }
      const field = new Timeless.vm.SingleFieldCore({
        label: field_schema.label,
        name: field_schema.name,
        help: field_schema.description || "",
        input,
      });
      field.form_schema = field_schema;
      if (
        field_schema.control === "input" ||
        field_schema.control === "textarea"
      ) {
        field.suggest_visible = ref(false);
        field.suggest_keyword = ref("");
        field.context_keys = context_keys;
      }
      fields[field_schema.name] = field;
    });
    return new Timeless.vm.ObjectFieldCore({ fields });
  }

  function is_template(value) {
    return typeof value === "string" && value.includes("{{");
  }

  function service_form_arguments(options = {}) {
    const form = add_service_form_.value;
    const form_schema = add_service_form_schema_.value || [];
    const raw_values = form ? form.value : {};
    const arguments_value = {};
    form_schema.forEach((field_schema) => {
      const raw_value = raw_values[field_schema.name];
      const blank =
        raw_value === undefined ||
        raw_value === null ||
        (typeof raw_value === "string" && raw_value.trim() === "");
      if (blank) {
        if (field_schema.required && options.validate_required) {
          throw new Error(`${field_schema.label} 不能为空`);
        }
        return;
      }

      if (is_template(raw_value)) {
        arguments_value[field_schema.name] = raw_value;
        return;
      }

      let value = raw_value;
      if (field_schema.type === "integer" || field_schema.type === "number") {
        value = Number(raw_value);
        if (!Number.isFinite(value)) {
          throw new Error(`${field_schema.label} 必须是数字`);
        }
        if (field_schema.type === "integer" && !Number.isInteger(value)) {
          throw new Error(`${field_schema.label} 必须是整数`);
        }
        if (
          field_schema.minimum !== undefined &&
          value < Number(field_schema.minimum)
        ) {
          throw new Error(
            `${field_schema.label} 不能小于 ${field_schema.minimum}`,
          );
        }
        if (
          field_schema.maximum !== undefined &&
          value > Number(field_schema.maximum)
        ) {
          throw new Error(
            `${field_schema.label} 不能大于 ${field_schema.maximum}`,
          );
        }
      } else if (
        field_schema.type === "array" ||
        field_schema.type === "object"
      ) {
        if (typeof raw_value === "string") {
          try {
            value = JSON.parse(raw_value);
          } catch {
            throw new Error(`${field_schema.label} 必须是有效 JSON`);
          }
        }
        if (field_schema.type === "array" && !Array.isArray(value)) {
          throw new Error(`${field_schema.label} 必须是 JSON 数组`);
        }
        if (
          field_schema.type === "object" &&
          (!value || typeof value !== "object" || Array.isArray(value))
        ) {
          throw new Error(`${field_schema.label} 必须是 JSON 对象`);
        }
      } else if (field_schema.type === "string") {
        value = String(raw_value).trim();
      }
      arguments_value[field_schema.name] = value;
    });
    return arguments_value;
  }

  function update_service_config(tool_name, arguments_value) {
    let config;
    try {
      config = JSON.parse(String(add_config_.value || "{}"));
    } catch {
      config = {};
    }
    if (!config || typeof config !== "object" || Array.isArray(config)) {
      config = {};
    }
    config.tool_name = tool_name;
    config.arguments = arguments_value || {};
    const encoded = JSON.stringify(config, null, 2);
    add_config_.as(encoded);
    ui.input_add_config$.setValue(encoded, { silence: true });
  }

  function sync_service_form_config() {
    try {
      const arguments_value = service_form_arguments();
      add_service_form_error_.as("");
      update_service_config(add_service_tool_.value, arguments_value);
    } catch (error) {
      add_service_form_error_.as(error.message || String(error));
    }
  }

  function select_service_tool(tool_name) {
    const tool = find_service_tool(tool_name);
    const form_schema = service_tool_form_schema(tool);
    const schema_log = {
      tool_name,
      tool_found: Boolean(tool),
      tool: tool || null,
      input_schema:
        (tool && (tool.input_schema || tool.inputSchema)) || null,
      declared_form_schema:
        tool && Array.isArray(tool.form_schema) ? tool.form_schema : null,
      resolved_form_schema: form_schema,
    };
    let schema_log_text;
    try {
      schema_log_text = JSON.stringify(schema_log, null, 2);
    } catch (error) {
      schema_log_text = JSON.stringify({
        tool_name,
        tool_found: Boolean(tool),
        serialization_error: error.message || String(error),
      });
    }
    console.info("[automation][service-tool-schema]", schema_log_text);
    add_service_form_.as(create_service_tool_form(form_schema));
    add_service_form_schema_.as(form_schema, { reset: true });
    add_service_form_error_.as("");
    update_service_config(tool_name, {});
    sync_service_form_config();
  }

  function find_pipeline(flow_id) {
    const pipelines = pipelines_.value || [];
    return pipelines.find((pipeline) => pipeline.id === flow_id) || null;
  }

  function pipeline_name(flow_id) {
    const pipeline = find_pipeline(flow_id);
    return pipeline ? pipeline.name || pipeline.id : flow_id || "-";
  }

  function set_error(error) {
    error_.as(error && error.message ? error.message : String(error || ""));
    loading_.as(false);
  }

  function decode_automation_channel_message(value) {
    if (typeof value !== "string") return value;
    try {
      return JSON.parse(value);
    } catch {
      return null;
    }
  }

  function reset_execution_tracking() {
    execution_run_id_.as("");
    execution_run_status_.as("");
    node_execution_states_.as({});
    execution_logs_.as([], { reset: true });
  }

  function ensure_execution_run(run_id, run_status) {
    const normalized_run_id = String(run_id || "").trim();
    if (
      normalized_run_id &&
      execution_run_id_.value &&
      execution_run_id_.value !== normalized_run_id
    ) {
      node_execution_states_.as({});
      execution_logs_.as([], { reset: true });
    }
    if (normalized_run_id) execution_run_id_.as(normalized_run_id);
    if (run_status) execution_run_status_.as(String(run_status));
  }

  // 浮窗尺寸需与 style.css 中节点日志浮窗使用的尺寸原子类保持一致
  const execution_window_width = 420;
  const execution_window_margin = 12;
  // 纵向至少留出标题栏，避免点在画布底部时窗口整块跑出屏幕
  const execution_window_min_visible = 48;

  function clamp_window_offset(value, min, max) {
    const offset = Number(value);
    const safe_offset = Number.isFinite(offset) ? offset : min;
    return Math.min(Math.max(safe_offset, min), Math.max(min, max));
  }

  /**
   * WindowManager 全量快照 → 视图状态。
   * 成员集合用 refarr 的 insert/delete patch 驱动 For，逐个增删，
   * 避免整表 reset 导致其它窗口被销毁重建。
   */
  function sync_execution_windows(state) {
    const windows = (state && state.windows) || [];
    const ids = windows.map((item) => item.id);
    const current = (execution_windows_.value || []).map((item) => item.id);
    for (let i = current.length - 1; i >= 0; i -= 1) {
      if (!ids.includes(current[i])) execution_windows_.delete(i);
    }
    const now = (execution_windows_.value || []).map((item) => item.id);
    ids.forEach((id) => {
      if (!now.includes(id)) execution_windows_.push({ id });
    });
    const z = {};
    windows.forEach((item) => {
      z[item.id] = item.z;
    });
    execution_window_z_.as(z);
    execution_window_active_id_.as((state && state.activeId) || "");
  }

  function open_node_execution_window(node_id, anchor) {
    const id = String(node_id || "");
    if (!id) return null;
    const point = anchor && typeof anchor === "object" ? anchor : {};
    const viewport_width = Number(window.innerWidth) || 0;
    const viewport_height = Number(window.innerHeight) || 0;
    execution_windows.open(id, {
      x: clamp_window_offset(
        point.x,
        execution_window_margin,
        viewport_width - execution_window_width,
      ),
      y: clamp_window_offset(
        point.y,
        execution_window_margin,
        viewport_height - execution_window_min_visible,
      ),
    });
    return id;
  }

  function close_node_execution_window(node_id) {
    return execution_windows.close(String(node_id || ""));
  }

  function focus_node_execution_window(node_id) {
    return execution_windows.focus(String(node_id || ""));
  }

  /** 拖动结束回写位置；clamp 到「至少 min_visible 露在视口内」，返回最终位置 */
  function move_node_execution_window(node_id, position) {
    const id = String(node_id || "");
    if (!id) return null;
    const point = position && typeof position === "object" ? position : {};
    const viewport_width = Number(window.innerWidth) || 0;
    const viewport_height = Number(window.innerHeight) || 0;
    const clamped = {
      x: clamp_window_offset(
        point.x,
        execution_window_min_visible - execution_window_width,
        viewport_width - execution_window_min_visible,
      ),
      y: clamp_window_offset(
        point.y,
        0,
        viewport_height - execution_window_min_visible,
      ),
    };
    execution_windows.moveTo(id, clamped);
    return clamped;
  }

  function close_all_node_execution_windows() {
    execution_windows.closeAll();
  }

  function execution_window_position(node_id) {
    return execution_windows.positionOf(String(node_id || ""));
  }

  function handle_automation_channel_message(message) {
    if (!message || typeof message !== "object") return;
    const flow_id = String(message.flow_id || "").trim();
    if (!flow_id || flow_id !== selected_flow_id_.value) return;

    if (message.type === "automation_run_started") {
      ensure_execution_run(message.run_id, message.run_status || "RUNNING");
      return;
    }
    if (
      message.type === "automation_run_completed" ||
      message.type === "automation_run_failed" ||
      message.type === "automation_run_waiting"
    ) {
      ensure_execution_run(message.run_id, message.run_status);
      return;
    }
    if (message.type === "automation_node_status") {
      const status = message.node_status;
      if (!status || typeof status !== "object" || !status.node_id) return;
      ensure_execution_run(message.run_id || status.run_id, "RUNNING");
      node_execution_states_.as({
        ...(node_execution_states_.value || {}),
        [status.node_id]: { ...status },
      });
      return;
    }
    if (message.type === "automation_node_log") {
      const entry = message.execution_log;
      if (!entry || typeof entry !== "object") return;
      ensure_execution_run(message.run_id || entry.run_id, "RUNNING");
      const log_entry = {
        ...entry,
        _execution_key: [
          entry.run_id,
          entry.node_id,
          entry.attempt,
          entry.timestamp || Date.now(),
        ].join("-"),
      };
      const next_logs = [...(execution_logs_.value || []), log_entry];
      execution_logs_.as(
        next_logs.slice(-automation_execution_log_limit),
        { reset: true },
      );
    }
  }

  async function connect_execution_channel() {
    if (disposed) return false;
    const result = await execution_channel.connect();
    if (!result || result.error) {
      throw (
        (result && result.error) ||
        new Error("automation execution channel connection failed")
      );
    }
    return true;
  }

  function dispose() {
    disposed = true;
    editor_bridge.destroy();
    request_sequence += 1;
    execution_channel_connected_.as(false);
    execution_channel.destroy();
    if (run_poll_timer !== null) clearInterval(run_poll_timer);
    run_query_sequence += 1;
  }

  async function load_pipelines() {
    const r = await reqs.pipeline.list.run({});
    if (r.error) {
      throw request_result_error(r, "Pipeline 列表加载失败");
    }
    const payload = r.data;
    return ((payload && payload.list) || [])
      .map(normalize_pipeline)
      .filter((pipeline) => pipeline.id);
  }

  async function load_catalog() {
    const r = await reqs.pipeline.catalog.run({});
    if (r.error) {
      throw request_result_error(r, "节点类型加载失败");
    }
    const payload = r.data;
    return (payload && payload.nodes) || [];
  }

  async function load_schedules() {
    const schedules = [];
    for (let page = 1; ; page += 1) {
      const r = await reqs.schedule.list.run({
        page, page_size: 200, flow_id: view_query.flow_id || "",
      });
      if (r.error) throw request_result_error(r, "触发器列表加载失败");
      const payload = r.data || {};
      const list = payload.list || [];
      schedules.push(...list);
      if (!list.length || schedules.length >= payload.total || list.length < 200) break;
    }
    return schedules;
  }

  async function load_data(options) {
    options = options || {};
    const sequence = ++request_sequence;
    loading_.as(true);
    error_.as("");
    if (!options.silent) notice_.as("");
    try {
      const [pipelines, catalog, schedules] = await Promise.all([
        load_pipelines(),
        load_catalog(),
        load_schedules(),
      ]);
      if (sequence !== request_sequence) return;
      pipelines_.as(pipelines, { reset: true });
      catalog_.as(catalog, { reset: true });
      schedules_.as(schedules, { reset: true });
      if (selected_flow_id_.value) {
        const fresh = pipelines.find(
          (item) => item.id === selected_flow_id_.value,
        );
        if (fresh) enter_edit(fresh);
        else {
          selected_pipeline_.as(null);
          edit_nodes_.as([], { reset: true });
          if (view_mode_.value === "list") selected_flow_id_.as("");
        }
      }
      if (sequence !== request_sequence) return;
      loading_.as(false);
      if (execution_channel_connected_.value) editor_bridge.register();
    } catch (err) {
      if (sequence !== request_sequence) return;
      set_error(err);
    }
  }

  function enter_edit(flow) {
    staged_node_positions.clear();
    selected_pipeline_.as(flow);
    edit_name_.as(flow.name || flow.id);
    ui.input_edit_name$.setValue(edit_name_.value, { silence: true });
    edit_triggers_.as(pipeline_triggers(flow, schedules_.value).map(new_trigger_row), { reset: true });
    const edit_nodes = (flow.nodes || []).map((node) => ({
      id: node.id,
      type: node.type,
      name: node.name || node.id,
      config: node.config || {},
      position: node.position ? { ...node.position } : null,
      next_ids: (node.next_ids || []).slice(),
      input_schema: node.input_schema || [],
    }));
    const calculated_positions = calculate_pipeline_node_positions(edit_nodes);
    edit_nodes_.as(
      edit_nodes.map((node) => ({
        ...node,
        position: calculated_positions[node.id] || null,
      })),
      { reset: true },
    );
    edit_start_node_id_.as(flow.start_node_id || "");
    const preferred_node_id = (flow.nodes || []).some(
      (node) => node.id === selected_edit_node_id_.value,
    )
      ? selected_edit_node_id_.value
      : flow.start_node_id || ((flow.nodes || [])[0] || {}).id || "";
    select_edit_node(preferred_node_id);
    dirty_.as(false);
    if (execution_channel_connected_.value) editor_bridge.register();
  }

  function new_trigger_row(value) {
    const trigger = { ...value, uid: ++trigger_sequence };
    const change = (key, value) => {
      trigger[key] = value;
      dirty_.as(true);
    };
    trigger.input_name$ = new Timeless.vm.InputCore({
      defaultValue: trigger.name || "", onChange(value) { change("name", value); },
    });
    trigger.input_cron$ = new Timeless.vm.InputCore({
      defaultValue: trigger.cron_expr || "@daily", onChange(value) { change("cron_expr", value); },
    });
    trigger.input_event_key$ = new Timeless.vm.InputCore({
      defaultValue: trigger.event_key || "", onChange(value) { change("event_key", value); },
    });
    trigger.checkbox_active$ = new Timeless.vm.CheckboxCore({ checked: trigger.active !== false, onChange(value) { change("active", value); } });
    trigger.input_timeout$ = new Timeless.vm.InputCore({ defaultValue: String(trigger.timeout_sec || 3600), onChange(value) { change("timeout_sec", Number(value)); } });
    trigger.checkbox_enabled$ = new Timeless.vm.CheckboxCore({
      checked: Boolean(trigger.enabled), onChange(value) { change("enabled", value); },
    });
    trigger.input_data$ = new Timeless.vm.InputCore({
      defaultValue: JSON.stringify(trigger.initial_data || {}, null, 2),
      onChange(value) { change("data_text", value); },
    });
    trigger.data_text = JSON.stringify(trigger.initial_data || {}, null, 2);
    return trigger;
  }

  function add_trigger(type) {
    edit_triggers_.as([...edit_triggers_.value, new_trigger_row({
      id: "", type, name: type === "Cron" ? "定时触发" : type === "Webhook" ? "Webhook" : "事件触发",
      cron_expr: "@daily", event_key: "", enabled: type === "Cron", initial_data: {},
    })], { reset: true });
    dirty_.as(true);
  }

  function serialize_triggers() {
    return edit_triggers_.value.map((trigger, index) => {
      const initial_data = JSON.parse(trigger.data_text || "{}");
      if (!initial_data || typeof initial_data !== "object" || Array.isArray(initial_data)) {
        throw new Error(`触发器 ${index + 1} 的输入参数必须是 JSON 对象`);
      }
      return {
        id: trigger.id, type: trigger.type, name: trigger.name,
        cron_expr: trigger.cron_expr, event_key: trigger.event_key,
        enabled: Boolean(trigger.enabled), active: trigger.active !== false, initial_data,
        timeout_sec: trigger.timeout_sec || undefined,
      };
    });
  }

  function find_edit_node(node_id) {
    return (edit_nodes_.value || []).find((node) => node.id === node_id) || null;
  }

  function select_edit_node(node_id) {
    const node = find_edit_node(String(node_id || ""));
    node_inspector_open_.as(Boolean(node));
    selected_edit_node_id_.as(node ? node.id : "");
    const config_text = JSON.stringify((node && node.config) || {}, null, 2);
    edit_node_config_.as(config_text);
    const policy_text = JSON.stringify({ retry: node && node.retry || { max_attempts: 1, delay_ms: 0, backoff_multiplier: 1 }, error_next_node: node && node.error_next_node || "", input_schema: node && node.input_schema || [], output_schema: node && node.output_schema || [] }, null, 2);
    edit_node_policy_.as(policy_text);
    ui.input_edit_node_policy$.setValue(policy_text, { silence: true });
    ui.input_edit_node_name$.setValue((node && node.name) || "", {
      silence: true,
    });
    ui.input_edit_node_config$.setValue(config_text, { silence: true });
    return node;
  }

  function update_selected_node(patch) {
    const node_id = selected_edit_node_id_.value;
    if (!node_id) return null;
    edit_nodes_.as(
      (edit_nodes_.value || []).map((node) =>
        node.id === node_id ? { ...node, ...patch } : node,
      ),
      { reset: true },
    );
    dirty_.as(true);
    return find_edit_node(node_id);
  }

  function move_edit_node(node_id, position) {
    const id = String(node_id || "");
    if (!position || typeof position !== "object") return null;
    const x = Number(position.x);
    const y = Number(position.y);
    if (!id || !Number.isFinite(x) || !Number.isFinite(y)) return null;
    staged_node_positions.set(id, { x, y });
    let moved_node = null;
    edit_nodes_.as(
      (edit_nodes_.value || []).map((node) => {
        if (node.id !== id) return node;
        moved_node = { ...node, position: { x, y } };
        return moved_node;
      }),
      { reset: true },
    );
    if (moved_node) dirty_.as(true);
    return moved_node;
  }

  function stage_edit_node_position(node_id, position) {
    const id = String(node_id || "");
    if (!position || typeof position !== "object") return null;
    const x = Number(position.x);
    const y = Number(position.y);
    if (!id || !Number.isFinite(x) || !Number.isFinite(y)) return null;
    const staged_position = { x, y };
    staged_node_positions.set(id, staged_position);
    dirty_.as(true);
    return staged_position;
  }

  function apply_selected_node_config() {
    const node_id = selected_edit_node_id_.value;
    if (!node_id) return null;
    error_.as("");
    try {
      const raw = String(edit_node_config_.value || "").trim();
      const config = raw ? JSON.parse(raw) : {};
      if (!config || typeof config !== "object" || Array.isArray(config)) {
        throw new Error("节点配置必须是 JSON 对象");
      }
      // 条件分支的 rules 是走向的唯一依据，改完 rules 后连线也要跟着变
      const next_ids = gateway_next_ids_from_rules(
        find_edit_node(node_id),
        config,
      );
      const policy = JSON.parse(edit_node_policy_.value || "{}");
      if (!policy || typeof policy !== "object" || Array.isArray(policy)) throw new Error("执行策略必须是 JSON 对象");
      const patch = { config, retry: policy.retry || { max_attempts: 1, delay_ms: 0, backoff_multiplier: 1 }, error_next_node: String(policy.error_next_node || ""), input_schema: policy.input_schema || [], output_schema: policy.output_schema || [] };
      update_selected_node(next_ids ? { ...patch, next_ids } : patch);
      notice_.as("节点配置已应用，保存 Pipeline 后生效");
    } catch (err) {
      set_error(err);
    }
    return null;
  }

  async function load_runs() {
    const flow_id = selected_flow_id_.value;
    if (!flow_id || disposed) return;
    const sequence = ++run_query_sequence;
    const response = await reqs.run.list.run({ flow_id, page: 1, page_size: 20 });
    if (response.error) throw request_result_error(response, "运行记录加载失败");
    if (sequence !== run_query_sequence || flow_id !== selected_flow_id_.value || disposed) return;
    runs_.as(response.data.list || [], { reset: true });
    const current_id = selected_run_id || selected_run_.value && selected_run_.value.id;
    const latest = (response.data.list || [])[0];
    const id = current_id || latest && latest.id;
    if (id) await select_run(id, flow_id);
  }

  async function select_run(id, expected_flow_id = selected_flow_id_.value) {
    selected_run_id = id;
    const sequence = ++run_detail_sequence;
    const response = await reqs.run.detail.run({ id });
    if (response.error) throw request_result_error(response, "运行详情加载失败");
    if (disposed || sequence !== run_detail_sequence || selected_flow_id_.value !== expected_flow_id) return;
    selected_run_.as(response.data);
    return response.data;
  }

  async function act_on_run(action) {
    const run = selected_run_.value;
    if (!run || run_action_busy_.value) return;
    run_action_busy_.as(true); error_.as("");
    const action_buttons = [ui.btn_cancel_run$, ui.btn_resume_run$, ui.btn_retry_run$, ui.btn_retry_current_run$];
    action_buttons.forEach((button) => button.disable());
    try {
      let response;
      if (action === "resume") {
        const input = JSON.parse(manual_response_.value || "{}");
        if (!input || typeof input !== "object" || Array.isArray(input)) throw new Error("人工确认输入必须为 JSON 对象");
        response = await reqs.run.resume.run({ id: run.id, node_id: run.current_node, input });
      } else if (action === "cancel") response = await reqs.run.cancel.run({ id: run.id });
      else response = await reqs.run.retry.run({ id: run.id, use_current_definition: action === "retry_current" });
      if (response.error) throw request_result_error(response, "运行操作失败");
      if (response.data && response.data.id && action !== "cancel") { selected_run_id = response.data.id; selected_run_.as(response.data); }
      notice_.as(action === "resume" ? "运行已恢复" : action === "cancel" ? "运行已取消" : "已创建重试运行，请核对外部操作结果");
      await load_runs();
    } catch (error) { set_error(error); }
    run_action_busy_.as(false);
    action_buttons.forEach((button) => button.enable());
  }

  function execution_policy_is_pending(node) {
    try {
      const policy = JSON.parse(edit_node_policy_.value || "{}");
      return JSON.stringify(policy) !== JSON.stringify({ retry: node && node.retry || { max_attempts: 1, delay_ms: 0, backoff_multiplier: 1 }, error_next_node: node && node.error_next_node || "", input_schema: node && node.input_schema || [], output_schema: node && node.output_schema || [] });
    } catch { return true; }
  }

  async function select_pipeline(flow_id, options) {
    options = options || {};
    if (selected_flow_id_.value !== flow_id) { reset_execution_tracking(); selected_run_.as(null); selected_run_id = ""; runs_.as([], { reset: true }); }
    selected_flow_id_.as(flow_id);
    const pipeline = find_pipeline(flow_id);
    if (pipeline) enter_edit(pipeline);
    else selected_pipeline_.as(null);
    if (!options.silent) {
      const r = await reqs.pipeline.detail.run({ id: flow_id });
      if (r.error) {
        set_error(request_result_error(r, "Pipeline 详情加载失败"));
        return;
      }
      if (selected_flow_id_.value !== flow_id) return;
      const flow = normalize_pipeline(r.data);
      if (flow.id) {
        pipelines_.as(
          (pipelines_.value || []).map((item) =>
            item.id === flow.id ? flow : item,
          ),
          { reset: true },
        );
        enter_edit(flow);
      }
    }
  }

  // --- create pipeline ---
  function open_create_dialog() {
    create_name_.as("");
    create_description_.as("");
    create_params_.as([new_create_param_row()], {
      reset: true,
    });
    ui.input_create_name$.setValue("", { silence: true });
    ui.input_create_description$.setValue("", { silence: true });
    create_open_.as(true);
    ui.create_dialog$.show();
  }

  function new_create_param_row() {
    const uid = ++param_sequence;
    const param = { uid, key: "", type: "string", required: false };
    param.input_key$ = new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "例如 url",
      onChange(value) {
        update_create_param(param, { key: value });
      },
    });
    param.select_type$ = new Timeless.vm.SelectCore({
      value: param.type,
      position: "popper",
      options: ["string", "number", "boolean", "any"].map(
        (type) => new Timeless.vm.SelectItemCore({ label: type, value: type }),
      ),
      onChange(value) {
        update_create_param(param, { type: String(value || "string") });
      },
    });
    return param;
  }

  function add_create_param() {
    create_params_.as([...create_params_.value, new_create_param_row()], {
      reset: true,
    });
  }

  function remove_create_param(param) {
    create_params_.as(
      create_params_.value.filter((item) => item.uid !== param.uid),
      { reset: true },
    );
  }

  function update_create_param(param, patch) {
    create_params_.as(
      create_params_.value.map((item) =>
        item.uid === param.uid ? { ...item, ...patch } : item,
      ),
      { reset: true },
    );
  }

  async function submit_pipeline_create() {
    if (create_submitting_.value) {
      return null;
    }
    create_submitting_.as(true);
    error_.as("");
    try {
      const name = String(create_name_.value || "").trim();
      if (!name) throw new Error("请输入 Pipeline 名称");
      const context_schema = [];
      (create_params_.value || []).forEach((param) => {
        const key = String(param.key || "").trim();
        if (!key) return;
        context_schema.push({
          key,
          type: param.type || "string",
          required: Boolean(param.required),
        });
      });
      const r = await reqs.pipeline.create.run({
        name,
        description: String(create_description_.value || "").trim(),
        trigger_type: "Manual",
        context_schema,
      });
      if (r.error) throw request_result_error(r, "Pipeline 创建失败");
      const flow = r.data;
      ui.create_dialog$.hide();
      create_open_.as(false);
      await load_data({ silent: true });
      notice_.as("Pipeline 已创建，请在开始节点配置触发器");
      if (props.history && typeof props.history.push === "function") {
        props.history.push("root.shell.flow_edit", { id: flow.id });
      }
    } catch (err) {
      set_error(err);
    } finally {
      create_submitting_.as(false);
    }
    return null;
  }

  // --- editor: add / remove nodes ---
  function open_add_dialog(node_type, from_node_id, insert_to_node_id) {
    if (!selected_pipeline_.value) return null;
    const nodes = edit_nodes_.value || [];
    const last = nodes.length > 0 ? nodes[nodes.length - 1] : null;
    const selected = find_edit_node(selected_edit_node_id_.value);
    const requested_from = find_edit_node(String(from_node_id || ""));
    const from_node = requested_from || selected || last;
    if (requested_from) {
      select_edit_node(requested_from.id);
    }
    // 插入模式：只在 from 确实连到 insert_to 时才生效，否则退化成普通新增
    const requested_insert_to = String(insert_to_node_id || "");
    const insert_to =
      from_node &&
      requested_insert_to &&
      (from_node.next_ids || []).includes(requested_insert_to)
        ? requested_insert_to
        : "";
    const requested_type = String(node_type || "").trim();
    add_type_.as(requested_type);
    add_name_.as("");
    add_config_.as("{}");
    add_service_tool_.as("");
    add_service_form_.as(null);
    add_service_form_schema_.as([], { reset: true });
    add_service_form_error_.as("");
    add_from_.as(from_node ? from_node.id : "");
    ui.input_add_name$.setValue("", { silence: true });
    ui.input_add_config$.setValue("{}", { silence: true });
    ui.select_add_service_tool$.setValue("", { silence: true });
    ui.select_add_type$.setValue(requested_type, {
      silence: !requested_type,
    });
    ui.select_add_from$.setValue(from_node ? from_node.id : "", {
      silence: true,
    });
    // 放在最后：它一翻，弹窗就会切到「插入」文案，此时 from 等状态必须已是新值
    add_insert_to_.as(insert_to);
    add_open_.as(true);
    ui.add_dialog$.show();
    return null;
  }

  // 插入模式：在 from → to 这条连线上插入一个新节点
  function open_insert_dialog(from_node_id, to_node_id) {
    return open_add_dialog("", from_node_id, to_node_id);
  }

  async function submit_add_node() {
    if (add_submitting_.value) return null;
    add_submitting_.as(true);
    error_.as("");
    try {
      const node_type = String(add_type_.value || "").trim();
      if (!node_type) throw new Error("请选择节点类型");
      let config = {};
      const config_raw = String(add_config_.value || "").trim();
      if (config_raw) {
        config = JSON.parse(config_raw);
        if (!config || typeof config !== "object" || Array.isArray(config)) {
          throw new Error("节点配置必须是 JSON 对象");
        }
      }
      if (
        node_type === "ServiceNode" &&
        !String(add_service_tool_.value || config.tool_name || "").trim()
      ) {
        throw new Error("请选择 Service tool");
      }
      if (node_type === "ServiceNode") {
        config.tool_name = String(add_service_tool_.value || config.tool_name);
        config.arguments = service_form_arguments({ validate_required: true });
      }
      const node_id = `node-${Date.now()}-${++node_sequence}`;
      const from_id = String(add_from_.value || "").trim();
      const insert_to = String(add_insert_to_.value || "").trim();
      const nodes = (edit_nodes_.value || []).slice();
      const new_node = {
        id: node_id,
        type: node_type,
        name: String(add_name_.value || "").trim() || flow_label(node_type),
        config,
        // 插入模式下新节点接管 from→insert_to 这段连线
        next_ids: insert_to ? [insert_to] : [],
        input_schema: [],
      };
      let gateway_append = false;
      if (from_id) {
        const from_index = nodes.findIndex((node) => node.id === from_id);
        if (from_index >= 0) {
          const from_node = nodes[from_index];
          // 条件分支上「新增节点」= 多开一条分支，得由用户去 rules 里补条件
          gateway_append = !insert_to && gateway_rules_of(from_node) !== null;
          const next_ids = (from_node.next_ids || []).slice();
          if (insert_to) {
            const at = next_ids.indexOf(insert_to);
            if (at >= 0) next_ids[at] = node_id;
            else next_ids.push(node_id);
          } else if (!next_ids.includes(node_id)) {
            next_ids.push(node_id);
          }
          nodes[from_index] = {
            ...from_node,
            next_ids,
            // 条件分支的走向看 rules，不跟着改的话新节点不会被路由到
            config: insert_to
              ? remap_gateway_rules(from_node, insert_to, node_id)
              : from_node.config,
          };
        }
      }
      nodes.push(new_node);
      const calculated_positions = insert_to
        ? insert_node_positions(nodes, from_id, insert_to, node_id)
        : calculate_pipeline_node_positions(nodes);
      edit_nodes_.as(
        nodes.map((node) => ({
          ...node,
          position: calculated_positions[node.id] || null,
        })),
        { reset: true },
      );
      // 所有节点的坐标刚刚重算过（插入会把后面的整体右移），保存时不能再被
      // 之前拖拽留下的 staged 坐标覆盖，否则被移动过的节点会保存回旧位置
      staged_node_positions.clear();
      select_edit_node(node_id);
      dirty_.as(true);
      ui.add_dialog$.hide();
      add_open_.as(false);
      add_insert_to_.as("");
      notice_.as(
        insert_to
          ? "节点已插入，记得保存 Pipeline"
          : gateway_append
            ? "节点已作为新分支添加；还要在该节点的 rules 里补上条件，否则引擎不会走到它"
            : "节点已添加，记得保存 Pipeline",
      );
    } catch (err) {
      set_error(err);
    }
    add_submitting_.as(false);
    return null;
  }

  function remove_node(node_id) {
    if (node_id === edit_start_node_id_.value) {
      error_.as("开始节点不能删除");
      return;
    }
    staged_node_positions.delete(node_id);
    const nodes = (edit_nodes_.value || [])
      .filter((node) => node.id !== node_id)
      .map((node) => ({
        ...node,
        next_ids: (node.next_ids || []).filter((id) => id !== node_id),
        // 条件分支的 rules 里也可能指着它，一起摘掉
        config: drop_gateway_rules(node, node_id),
      }));
    edit_nodes_.as(nodes, { reset: true });
    if (selected_edit_node_id_.value === node_id) {
      select_edit_node(edit_start_node_id_.value);
    }
    dirty_.as(true);
  }

  async function save_flow() {
    if (!selected_pipeline_.value || saving_.value) return null;
    saving_.as(true);
    error_.as("");
    try {
      const name = edit_name_.value.trim();
      if (!name) throw new Error("请输入 Pipeline 标题");
      const calculated_positions = calculate_pipeline_node_positions(
        edit_nodes_.value,
      );
      const r = await reqs.pipeline.update.run({
        id: selected_flow_id_.value,
        name,
        triggers: serialize_triggers(),
        start_node_id: edit_start_node_id_.value,
        nodes: (edit_nodes_.value || []).map((node) => ({
          id: node.id,
          type: node.type,
          name: node.name,
          config: node.config || {},
          position:
            staged_node_positions.get(node.id) ||
            calculated_positions[node.id] ||
            null,
          input_schema: node.input_schema || [],
          output_schema: node.output_schema || [],
          retry: node.retry || undefined,
          error_next_node: node.error_next_node || "",
          next_ids: node.next_ids || [],
        })),
      });
      if (r.error) throw request_result_error(r, "Pipeline 保存失败");
      dirty_.as(false);
      notice_.as("Pipeline 已保存");
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
    saving_.as(false);
    return null;
  }

  function ask_delete_pipeline(pipeline) {
    pending_delete_flow_id_.as((pipeline && pipeline.id) || "");
    ui.delete_dialog$.show();
    return null;
  }

  async function delete_flow() {
    const flow_id = pending_delete_flow_id_.value || selected_flow_id_.value;
    if (!flow_id) return null;
    error_.as("");
    try {
      const r = await reqs.pipeline.delete.run({ id: flow_id });
      if (r.error) throw request_result_error(r, "Pipeline 删除失败");
      ui.delete_dialog$.hide();
      pending_delete_flow_id_.as("");
      if (flow_id === selected_flow_id_.value) {
        selected_flow_id_.as("");
        selected_pipeline_.as(null);
        edit_nodes_.as([], { reset: true });
      }
      notice_.as("Pipeline 已删除");
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
    return null;
  }

  function run_schema_fields() {
    const pipeline = selected_pipeline_.value;
    const schema =
      pipeline && Array.isArray(pipeline.context_schema)
        ? pipeline.context_schema
        : [];
    return schema.filter(
      (field) => field && typeof field === "object" && field.key,
    );
  }

  function new_run_param_row(field) {
    const key = String(field.key || "").trim();
    const type = String(field.type || "string").toLowerCase();
    const required = Boolean(field.required);
    const row = { uid: ++run_param_sequence, key, type, required, value: undefined };
    if (type === "boolean") {
      row.checkbox$ = new Timeless.vm.CheckboxCore({
        checked: false,
        onChange(value) {
          row.value = Boolean(value);
        },
      });
    } else {
      row.input$ = new Timeless.vm.InputCore({
        defaultValue: "",
        placeholder: required ? "必填" : "可选",
        onChange(value) {
          row.value = value;
        },
      });
    }
    return row;
  }

  function open_run_dialog() {
    run_params_.as(run_schema_fields().map(new_run_param_row), {
      reset: true,
    });
    run_error_.as("");
    run_open_.as(true);
    ui.run_dialog$.show();
    return null;
  }

  function coerce_run_param(type, raw) {
    if (raw === undefined || raw === null) return raw;
    switch (String(type || "string").toLowerCase()) {
      case "number": {
        const num = Number(raw);
        return Number.isFinite(num) ? num : String(raw);
      }
      case "boolean":
        return raw === true || raw === "true" || raw === "1";
      case "any":
        return raw;
      default:
        return String(raw);
    }
  }

  function build_run_params() {
    const initial_data = {};
    const params = run_params_.value || [];
    for (const param of params) {
      const key = String(param.key || "").trim();
      if (!key) continue;
      const value = coerce_run_param(param.type, param.value);
      if (value === undefined || value === null || value === "") {
        if (param.required) {
          return { error: `请填写必填参数：${key}` };
        }
        continue;
      }
      initial_data[key] = value;
    }
    return { initial_data };
  }

  async function run_flow_direct(initial_data) {
    const flow_id = selected_flow_id_.value;
    if (!flow_id) return null;
    error_.as("");
    execution_run_status_.as("QUEUED");
    const r = await reqs.pipeline.trigger.run({ id: flow_id, initial_data });
    if (r.error) {
      execution_run_status_.as("FAILED");
      return r;
    }
    if (r.data.id) { execution_run_id_.as(r.data.id); selected_run_id = r.data.id; selected_run_.as(r.data); }
    load_runs().catch(set_error);
    notice_.as(`已触发执行（${r.data.status || "RUNNING"}）`);
    return r;
  }

  async function trigger_flow() {
    const flow_id = selected_flow_id_.value;
    if (!flow_id) return null;
    if (run_schema_fields().length > 0) {
      open_run_dialog();
      return null;
    }
    try {
      const r = await run_flow_direct({});
      if (r && r.error) {
        set_error(request_result_error(r, "Pipeline 触发失败"));
      }
    } catch (err) {
      execution_run_status_.as("FAILED");
      set_error(err);
    }
    return null;
  }

  async function submit_run() {
    if (run_submitting_.value) return null;
    run_submitting_.as(true);
    run_error_.as("");
    try {
      const built = build_run_params();
      if (built.error) {
        run_error_.as(built.error);
        return null;
      }
      const r = await run_flow_direct(built.initial_data);
      if (r && r.error) {
        run_error_.as(request_result_error(r, "Pipeline 触发失败").message);
        return null;
      }
      ui.run_dialog$.hide();
      run_open_.as(false);
    } catch (err) {
      run_error_.as(err && err.message ? err.message : String(err || ""));
      execution_run_status_.as("FAILED");
    } finally {
      run_submitting_.as(false);
    }
    return null;
  }

  function open_import_dialog() {
    import_json_.as("");
    import_error_.as("");
    ui.input_import_json$.setValue("", { silence: true });
    import_open_.as(true);
    ui.import_dialog$.show();
    return null;
  }

  async function submit_import() {
    if (import_submitting_.value) return null;
    import_submitting_.as(true);
    import_error_.as("");
    try {
      const raw = String(import_json_.value || "").trim();
      if (!raw) throw new Error("请粘贴流程定义 JSON");
      const r = await reqs.pipeline.import.run({ definition: raw });
      if (r.error) throw request_result_error(r, "Pipeline 导入失败");
      const flow = r.data;
      ui.import_dialog$.hide();
      import_open_.as(false);
      await load_data({ silent: true });
      notice_.as("Pipeline 已导入");
      if (props.history && typeof props.history.push === "function") {
        props.history.push("root.shell.flow_edit", { id: flow.id });
      }
    } catch (err) {
      import_error_.as(err && err.message ? err.message : String(err || ""));
    }
    import_submitting_.as(false);
    return null;
  }

  async function trigger_schedule(id) {
    const schedule = (schedules_.value || []).find((item) => item.id === id);
    if (!schedule) return null;
    const metadata = schedule_metadata(schedule);
    error_.as("");
    try {
      const r = await reqs.schedule.trigger.run({
        id,
        trigger_type: metadata.type === "Event" ? "Event" : "Manual",
        event_key: metadata.type === "Event" ? metadata.event_key : "",
      });
      if (r.error) throw request_result_error(r, "触发器执行失败");
      notice_.as(metadata.type === "Event" ? "事件已触发" : "Pipeline 已手动触发");
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
    return null;
  }

  const editor_bridge = PipelineEditorBridge({
    channel: execution_channel,
    read() {
      if (view_mode_.value !== "edit" || !selected_pipeline_.value || loading_.value) return null;
      const selected = find_edit_node(selected_edit_node_id_.value);
      let pending_node_config = false;
      try {
        pending_node_config = JSON.stringify(JSON.parse(edit_node_config_.value || "{}")) !== JSON.stringify((selected && selected.config) || {}) || execution_policy_is_pending(selected);
      } catch { pending_node_config = true; }
      let triggers = null;
      let trigger_error = "";
      try { triggers = serialize_triggers(); } catch (error) { trigger_error = error.message; }
      return {
        flow_id: selected_flow_id_.value,
        name: edit_name_.value,
        dirty: dirty_.value,
        saving: saving_.value,
        selected_node_id: selected_edit_node_id_.value,
        node_config_text: edit_node_config_.value,
        node_policy_text: edit_node_policy_.value,
        pending_node_config,
        trigger_error,
        trigger_forms: edit_triggers_.value.map((trigger) => ({
          id: trigger.id, type: trigger.type, name: trigger.name,
          cron_expr: trigger.cron_expr, event_key: trigger.event_key,
          enabled: trigger.enabled, data_text: trigger.data_text,
        })),
        draft: {
          start_node_id: edit_start_node_id_.value,
          context_schema: selected_pipeline_.value.context_schema || [],
          nodes: edit_nodes_.value.map((node) => ({
            ...node, position: staged_node_positions.get(node.id) || node.position || null,
          })),
          triggers,
        },
      };
    },
    apply(draft) {
      if (!draft || !Array.isArray(draft.nodes) || !draft.nodes.length) throw new Error("流程草稿缺少节点");
      if (draft.context_schema && JSON.stringify(draft.context_schema) !== JSON.stringify(selected_pipeline_.value.context_schema || [])) {
        throw new Error("当前接口仅编辑节点和触发器，请保留读取到的 context_schema");
      }
      const positions = calculate_pipeline_node_positions(draft.nodes);
      staged_node_positions.clear();
      edit_nodes_.as(draft.nodes.map((node) => ({
        ...node, name: node.name || node.id, config: node.config || {},
        input_schema: node.input_schema || [],
        next_ids: gateway_next_ids_from_rules(node, node.config) || node.next_ids || [],
        position: positions[node.id] || null,
      })), { reset: true });
      edit_start_node_id_.as(draft.start_node_id);
      if (Array.isArray(draft.triggers)) edit_triggers_.as(draft.triggers.map(new_trigger_row), { reset: true });
      select_edit_node(find_edit_node(selected_edit_node_id_.value) ? selected_edit_node_id_.value : draft.start_node_id);
      dirty_.as(true);
      notice_.as("MCP 已更新流程画布，保存 Pipeline 后生效");
    },
    on_error(error) {
      if (!disposed) error_.as(error && error.message ? error.message : "编辑器通信失败");
    },
  });

  const methods = {
    tip: props.tip,
    async ready() {
      await load_data();
      if (view_mode_.value !== "list" && selected_flow_id_.value) {
        await select_pipeline(selected_flow_id_.value);
      }
    connect_execution_channel().catch(() => false);
      if (view_mode_.value === "detail" && !disposed) {
        await load_runs().catch(set_error);
        if (!disposed && run_poll_timer === null) {
          run_poll_timer = setInterval(() => { load_runs().catch(set_error); }, 3000);
        }
      }
      return null;
    },
    destroy: dispose,
    refresh() {
      return load_data({ silent: true });
    },
    search: search_pipelines,
    changePage: change_page,
    selectPipeline: select_pipeline,
    triggerSchedule(id) {
      return trigger_schedule(id);
    },
    openCreateDialog: open_create_dialog,
    addCreateParam: add_create_param,
    removeCreateParam: remove_create_param,
    updateCreateParam: update_create_param,
    openAddDialog: open_add_dialog,
    openInsertDialog: open_insert_dialog,
    selectEditNode: select_edit_node,
    closeNodeInspector() {
      node_inspector_open_.as(false);
    },
    stageEditNodePosition: stage_edit_node_position,
    moveEditNode: move_edit_node,
    applySelectedNodeConfig: apply_selected_node_config,
    removeNode: remove_node,
    saveFlow: save_flow,
    deleteFlow: delete_flow,
    askDeletePipeline: ask_delete_pipeline,
    triggerFlow: trigger_flow,
    submitRun: submit_run,
    runSchemaFields: run_schema_fields,
    openImportDialog: open_import_dialog,
    submitImport: submit_import,
    refreshRuns: load_runs,
    selectRun: select_run,
    actOnRun: act_on_run,
    scheduleMetadata: schedule_metadata,
    triggerLabel: trigger_label,
    schemaText: schema_text,
    formatTime: format_time,
    pipelineName: pipeline_name,
    pipelineTriggers(flow) { return pipeline_triggers(flow, schedules_.value); },
    pipelineTriggerTypes(flow) {
      return [...new Set(["Manual", ...pipeline_triggers(flow, schedules_.value).map((trigger) => trigger.type)])];
    },
    flowLabel: flow_label,
    flowIcon: flow_icon,
    nodeExecutionStatusLabel(status) {
      const labels = {
      PENDING: "等待",
      WAITING: "等待",
      RUNNING: "执行中",
        COMPLETED: "已完成",
        FAILED: "失败",
        RETRYING: "重试中",
        WAITING_FOR_USER: "等待确认",
        WAITING_FOR_SUBPROCESS: "等待子流程",
      WAITING_FOR_MERGE: "等待汇聚",
      CANCELLED: "已取消",
      };
      return labels[String(status || "").toUpperCase()] || String(status || "");
    },
    formatExecutionValue(value) {
      try {
        return JSON.stringify(value == null ? {} : value, null, 2);
      } catch {
        return String(value);
      }
    },
    openNodeExecutionWindow: open_node_execution_window,
    closeNodeExecutionWindow: close_node_execution_window,
    focusNodeExecutionWindow: focus_node_execution_window,
    moveNodeExecutionWindow: move_node_execution_window,
    closeAllNodeExecutionWindows: close_all_node_execution_windows,
    executionWindowPosition: execution_window_position,
    catalogDescription(node_type) {
      const item = (catalog_.value || []).find(
        (entry) => entry.type === node_type,
      );
      return item ? item.description : "";
    },
    catalogConfigKeys(node_type) {
      const item = (catalog_.value || []).find(
        (entry) => entry.type === node_type,
      );
      return item ? item.config_keys || [] : [];
    },
    catalogServiceTool(tool_name) {
      return find_service_tool(tool_name);
    },
  };

  const state = {
    view_mode: view_mode_,
    pipelines: pipelines_,
    filtered_pipelines: filtered_pipelines_,
    paged_pipelines: paged_pipelines_,
    total: total_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    search_keyword: search_keyword_,
    catalog: catalog_,
    schedules: schedules_,
    selected_flow_id: selected_flow_id_,
    selected_pipeline: selected_pipeline_,
    edit_nodes: edit_nodes_,
    edit_triggers: edit_triggers_,
    edit_start_node_id: edit_start_node_id_,
    selected_edit_node_id: selected_edit_node_id_,
    node_inspector_open: node_inspector_open_,
    edit_node_config: edit_node_config_,
    loading: loading_,
    error: error_,
    notice: notice_,
    dirty: dirty_,
    saving: saving_,
    execution_channel_connected: execution_channel_connected_,
    execution_run_id: execution_run_id_,
    execution_run_status: execution_run_status_,
    runs: runs_, selected_run: selected_run_, run_action_busy: run_action_busy_,
    manual_response: manual_response_, edit_node_policy: edit_node_policy_,
    node_execution_states: node_execution_states_,
    execution_logs: execution_logs_,
    execution_windows: execution_windows_,
    execution_window_z: execution_window_z_,
    execution_window_active_id: execution_window_active_id_,
    create_open: create_open_,
    create_name: create_name_,
    create_description: create_description_,
    create_params: create_params_,
    add_open: add_open_,
    add_type: add_type_,
    add_name: add_name_,
    add_config: add_config_,
    add_service_tool: add_service_tool_,
    add_service_form: add_service_form_,
    add_service_form_schema: add_service_form_schema_,
    add_service_form_error: add_service_form_error_,
    add_from: add_from_,
    add_insert_to: add_insert_to_,
    run_open: run_open_,
    run_submitting: run_submitting_,
    run_params: run_params_,
    run_error: run_error_,
    import_open: import_open_,
    import_submitting: import_submitting_,
    import_json: import_json_,
    import_error: import_error_,
    delete_flow_id: pending_delete_flow_id_,
  };

  return { state, ui, methods };
}

export {
  AutomationPageViewModel,
  calculate_flow_minimap_geometry,
  calculate_pipeline_node_positions,
  normalize_pipeline,
  pipeline_triggers,
};

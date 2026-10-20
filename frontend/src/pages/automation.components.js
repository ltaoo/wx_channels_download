import { ErrorMessage } from "@/dmui.js";
import {
  calculate_flow_minimap_geometry,
  calculate_pipeline_node_positions,
} from "./automation.model.js";

export function automation_trigger_badge(props) {
  const type = props.type || "Cron";
  const variants = {
    cron: "info",
    event: "success",
    manual: "warning",
  };
  const normalized = String(type).toLowerCase();
  return Tag(
    {
      variant: variants[normalized] || "default",
      class: "min-w-52px",
      attributes: { n: `automation-trigger-${normalized}` },
    },
    [props.label],
  );
}

export function AutomationPipelineTriggerBadges(props) {
  const vm$ = props.store;
  return View({ class: "flex flex-wrap items-center gap-1" }, [For({
    each: computed(vm$.state.schedules, () =>
      vm$.methods.pipelineTriggerTypes(props.pipeline).map((type) => ({ type }))),
    key: "type",
    render(value) {
      const item = value && value.value !== undefined ? value.value : value;
      return automation_trigger_badge({ type: item.type, label: vm$.methods.triggerLabel(item.type) });
    },
  })]);
}

export function automation_status_badge(value) {
  const normalized = String(value || "UNKNOWN").toUpperCase();
  let tone = "info";
  if (["COMPLETED", "ENABLED", "TRUE", "YES"].includes(normalized)) {
    tone = "success";
  } else if (["FAILED", "FALSE", "NO", "CANCELLED"].includes(normalized)) {
    tone = "danger";
  } else if (["RUNNING", "WAITING", "QUEUED"].includes(normalized)) {
    tone = "warning";
  }
  return Tag(
    {
      variant: tone,
      class: "min-w-52px",
      attributes: { n: `automation-status-${normalized.toLowerCase()}` },
    },
    [normalized],
  );
}

export function AutomationStartTriggers(props) {
  const vm$ = props.store;
  return View({ class: "grid min-w-0 py-4 gap-3 border-t border-t-solid border-t-color-default", attributes: { n: "automation-start-triggers" } }, [
    View({ class: "font-display text-base font-semibold text-color-text-primary" }, ["开始节点触发器"]),
    View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary" }, [
      "手动执行始终可用。可同时添加多个定时和事件触发器，保存 Pipeline 后生效。",
    ]),
    View({ class: "flex min-w-0 flex-wrap items-center gap-2" }, [
      Button({ store: vm$.ui.btn_add_cron_trigger$, attributes: { n: "automation-add-cron-trigger", type: "button" } }, ["添加定时触发"]),
      Button({ store: vm$.ui.btn_add_event_trigger$, attributes: { n: "automation-add-event-trigger", type: "button" } }, ["添加事件触发"]),
      Button({ store: vm$.ui.btn_add_webhook_trigger$, attributes: { n: "automation-add-webhook-trigger", type: "button" } }, ["添加 Webhook"]),
    ]),
    Show({
      when: computed(vm$.state.edit_triggers, (triggers) => !triggers.length),
      ok() { return View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary" }, ["尚未配置定时或事件触发器，可通过开始节点上方的「手动执行」启动。"]); },
    }),
    For({
      each: vm$.state.edit_triggers, key: "uid",
      render(value) {
        const trigger = value && value.value !== undefined ? value.value : value;
        const prefix = `automation-trigger-${trigger.uid}`;
        const field = (label, store, name, textarea = false) => View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
          View({ as: "label", class: "text-xs font-semibold text-color-text-secondary", attributes: { for: `${prefix}-${name}` } }, [label]),
          (textarea ? Textarea : Input)({ store, attributes: {
            id: `${prefix}-${name}`, n: `${prefix}-${name}`, "aria-label": label,
            ...(textarea ? { rows: "4", spellcheck: "false" } : {}),
          } }),
        ]);
        return View({ class: "grid min-w-0 pt-3 gap-3 border-t border-t-solid border-t-color-default", attributes: { n: prefix } }, [
          View({ class: "flex min-w-0 flex-wrap items-center justify-between gap-2" }, [
            automation_trigger_badge({ type: trigger.type, label: vm$.methods.triggerLabel(trigger.type) }),
            Button({ store: vm$.ui.btn_remove_trigger$.bind(trigger), attributes: {
              n: `${prefix}-remove`, type: "button", "aria-label": `移除${vm$.methods.triggerLabel(trigger.type)}触发器`,
            } }, ["移除触发器"]),
          ]),
          field("触发器名称", trigger.input_name$, "name"),
          trigger.type === "Cron" ? field("Cron 表达式", trigger.input_cron$, "cron") : null,
          trigger.type === "Cron" ? View({ class: "flex min-w-0 flex-wrap items-center gap-2" }, [
            Checkbox({ store: trigger.checkbox_enabled$, attributes: { id: `${prefix}-enabled`, n: `${prefix}-enabled`, "aria-label": "启用定时触发" } }),
            View({ as: "label", class: "text-xs font-semibold text-color-text-secondary", attributes: { for: `${prefix}-enabled` } }, ["启用定时触发"]),
          ]) : null,
          trigger.type === "Event" ? field("事件 Key", trigger.input_event_key$, "event-key") : null,
          trigger.type !== "Cron" ? View({ class: "flex items-center gap-2" }, [
            Checkbox({ store: trigger.checkbox_active$, attributes: { id: `${prefix}-active`, "aria-label": "启用触发器" } }),
            View({ as: "label", attributes: { for: `${prefix}-active` }, class: "text-xs text-color-text-secondary" }, ["启用触发器"]),
          ]) : null,
          field("整体超时（秒，包含人工等待）", trigger.input_timeout$, "timeout"),
          trigger.type === "Webhook" ? View({ class: "min-w-0 text-caption word-break-break-all text-color-text-secondary" }, [trigger.id ? `POST /api/v1/automation/webhook/${trigger.id}；Authorization: Bearer ${trigger.webhook_token || "保存后生成"}` : "保存后生成 Webhook 地址与令牌。事件载荷从 input.event.data 读取，需在流程参数中声明 event。"]): null,
          field("输入参数 JSON", trigger.input_data$, "data", true),
          View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary" }, ["输入参数作为开始节点的 input；所有触发器共用后续流程。"]),
        ].filter(Boolean));
      },
    }),
  ]);
}

export function AutomationTriggerSummary(props) {
  const vm$ = props.store;
  const triggers = combine({ flow: vm$.state.selected_pipeline, schedules: vm$.state.schedules },
    ({ flow }) => vm$.methods.pipelineTriggers(flow));
  return View({ class: "grid min-w-0 py-4 gap-3 border-t border-t-solid border-t-color-default", attributes: { n: "automation-trigger-summary" } }, [
    View({ class: "text-base font-semibold text-color-text-primary" }, ["开始节点触发器"]),
    View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary" }, ["手动执行始终可用。选择「编辑 Pipeline」，在开始节点中管理触发器。"]),
    For({ each: triggers, render(value) {
      const trigger = value && value.value !== undefined ? value.value : value;
      return View({ class: "grid min-w-0 pt-3 gap-3 border-t border-t-solid border-t-color-default" }, [
        View({ class: "flex min-w-0 flex-wrap items-center justify-between gap-2" }, [
          View({ class: "flex min-w-0 flex-wrap items-center gap-2" }, [
            automation_trigger_badge({ type: trigger.type, label: vm$.methods.triggerLabel(trigger.type) }),
            View({ attributes: { title: trigger.name } }, [trigger.name]),
          ]),
          trigger.id ? Button({
            store: vm$.ui.btn_schedule_trigger$.bind(trigger),
            attributes: { type: "button", n: `automation-trigger-event-${trigger.id}` },
          }, [trigger.type === "Event" ? "触发事件" : "手动执行"]) : null,
        ].filter(Boolean)),
        View({ class: "overflow-visible font-mono text-sm whitespace-pre-wrap word-break-break-all text-overflow-initial code-label" }, [
          trigger.type === "Cron" ? trigger.cron_expr : trigger.event_key || "手动执行",
        ]),
        trigger.type === "Cron" ? View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary" }, [
          trigger.enabled ? `已启用 · 下次 ${vm$.methods.formatTime(trigger.next_run_at)}` : "已暂停",
        ]) : null,
      ].filter(Boolean));
    } }),
  ]);
}

export function automation_activate(event, callback) {
  if (event.target !== event.currentTarget) return;
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  callback();
}

export function AutomationEmptyState(props = {}) {
  return View(
    {
      class: [
        "dm-empty-state",
        props.compact ? "min-h-168px p-5" : "",
        props.detail ? "min-h-fluid-state" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        n: props.name || "automation-empty-state",
        role: "status",
        "aria-live": "polite",
      },
    },
    [
      View({ class: "text-base font-semibold text-content-text state-title" }, [props.title]),
      props.description
        ? View({ class: "max-w-480px text-sm leading-relaxed state-description" }, [props.description])
        : null,
    ].filter(Boolean),
  );
}

export function automation_pipeline_href(flow_id, mode) {
  const pathname = mode === "edit" ? "/automation/edit" : "/automation/detail";
  const search = new URLSearchParams({ id: String(flow_id || "") });
  return `${pathname}?${search.toString()}`;
}

export function AutomationPipelineLink(props) {
  const mode = props.mode === "edit" ? "edit" : "detail";
  return Link(
    {
      class: [
        "text-decoration-none dm-button dm-focus-ring dm-button--sm",
        props.primary ? "dm-button--primary" : "dm-button--outline",
      ].join(" "),
      href: automation_pipeline_href(props.flowId, mode),
      target: "_blank",
      attributes: {
        n: `automation-open-${mode}-${props.flowId}`,
        rel: "noopener noreferrer",
        "aria-label": `${props.label}（新标签页）`,
      },
    },
    [
      Timeless.Icon({
        name: mode === "edit" ? "pen-line" : "eye",
        size: 15,
        attributes: { "aria-hidden": "true" },
      }),
      props.label,
    ],
  );
}

export function AutomationFeedback(props) {
  const vm$ = props.store;
  return View({ class: "grid pt-4 pb-0 px-4 gap-2 feedback-stack " }, [

    Show({
      when: vm$.state.notice,
      ok() {
        return Alert(
          {
            class: "m-0",
            attributes: { n: "automation-notice-alert" },
          },
          [AlertDescription({}, [vm$.state.notice.value])],
        );
      },
    }),
  ]);
}

export function AutomationWorkspaceToolbar(props) {
  const vm$ = props.store;
  const is_edit = props.mode === "edit";
  return View({
    class: is_edit
      ? "flex fixed top-4 left-4 z-sticky max-w-screen-inset flex-col items-start gap-2"
      : "flex min-w-0 max-760:flex-wrap max-900:flex-col items-center justify-between max-900:items-stretch gap-3 group-content-toolbar",
    attributes: { n: "automation-workspace-toolbar" },
  }, [
    !is_edit ? View({ class: "flex min-w-0 items-center gap-2", attributes: { n: "automation-workspace-heading" } }, [
      View({ class: "min-w-0", attributes: { n: "automation-workspace-title" } }, [
        View({ class: "text-caption font-semibold text-transform-uppercase text-color-text-secondary", attributes: { n: "automation-workspace-mode" } }, [
          "Pipeline 详情",
        ]),
        View({ class: "overflow-hidden font-display text-lg font-semibold whitespace-nowrap text-ellipsis text-color-text-primary", attributes: { n: "automation-workspace-name" } }, [
          computed(vm$.state.selected_pipeline, (flow) =>
            flow ? flow.name || flow.id : vm$.state.selected_flow_id.value,
          ),
        ]),
      ]),
    ]) : null,
    View(
      { class: is_edit ? "flex min-w-0 flex-wrap items-center justify-start gap-2" : "flex min-w-0 flex-wrap items-center justify-end gap-2", attributes: { n: "automation-workspace-actions" } },
      [
        !is_edit ? Button(
          {
            store: vm$.ui.btn_run_flow$,
            attributes: { n: "automation-run-flow", type: "button" },
          },
          ["手动执行"],
        ) : null,
        is_edit
          ? View({ class: "flex min-w-0 items-center gap-2", attributes: { n: "automation-save-controls" } }, [
            Input({
              store: vm$.ui.input_edit_name$,
              attributes: { n: "automation-pipeline-name", "aria-label": "Pipeline 标题" },
            }),
            Button(
              {
                store: vm$.ui.btn_save_flow$,
                attributes: { n: "automation-save-flow", type: "button" },
              },
              ["保存"],
            ),
            Tag(
              {
                variant: "warning",
                class: computed(vm$.state.dirty, (dirty) =>
                  dirty
                    ? "status-badge-success "
                    : "status-badge-success is-clean",
                ),
              },
              [
                computed(vm$.state.dirty, (dirty) =>
                  dirty ? "有未保存变更" : "已保存",
                ),
              ],
            ),
          ])
          : AutomationPipelineLink({
              flowId: vm$.state.selected_flow_id.value,
              mode: "edit",
              label: "编辑 Pipeline",
              primary: true,
            }),
      ].filter(Boolean),
    ),
  ]);
}

export function automation_execution_status_class(status) {
  return String(status || "")
    .trim()
    .toLowerCase()
    .replaceAll("_", "-");
}

function automation_edit_layout(nodes, position_overrides) {
  const by_id = {};
  (nodes || []).forEach((node) => {
    by_id[node.id] = node;
  });
  const positions = Object.fromEntries(
    Object.entries(calculate_pipeline_node_positions(nodes)).map(
      ([node_id, position]) => [node_id, { left: position.x, top: position.y }],
    ),
  );
  Object.entries(position_overrides || {}).forEach(([node_id, position]) => {
    if (!by_id[node_id] || !position) return;
    positions[node_id] = {
      left: Number(position.left) || 0,
      top: Number(position.top) || 0,
    };
  });
  const width = Object.values(positions).reduce(
    (max, position) => Math.max(max, position.left + 270),
    350,
  );
  const max_bottom = Object.values(positions).reduce(
    (max, position) => Math.max(max, position.top + 110),
    160,
  );
  // 节点可以拖到画布原点左上（负坐标），画布原点仍是 0,0，
  // 这里只把负向边界一起返回，供缩略图/世界范围使用
  const min_left = Object.values(positions).reduce(
    (min, position) => Math.min(min, position.left),
    0,
  );
  const min_top = Object.values(positions).reduce(
    (min, position) => Math.min(min, position.top),
    0,
  );
  return {
    positions,
    width,
    height: Math.max(max_bottom + 40, 320),
    min_left,
    min_top,
  };
}

// 节点右侧边框上的连接圆点，从连线出发的位置（top + 52）往下排：
// 每个已有后继一个黄点（在它与该后继之间插入节点），末尾补一个绿点（在其后新增节点）
const automation_flow_connector_gap = 24;
const automation_flow_connector_anchor = 52;

// 条件分支（GatewayNode/Exclusive）的走向由 config.rules 决定：引擎按顺序取第一条
// condition 成立的规则，跳到它的 target_id，完全不看 next_ids。所以出边上要直接标出
// 这条分支在 rules 里对应什么：是 / 否（condition 为 true 的兜底）/ 多分支时的 case 值。
function automation_gateway_rules(node) {
  if (!node || node.type !== "GatewayNode") return [];
  const config = node.config || {};
  if (String(config.gateway_type || "Exclusive") !== "Exclusive") return [];
  const rules = config.rules;
  if (!Array.isArray(rules)) return [];
  return rules.filter((rule) => rule && typeof rule === "object");
}

function automation_is_fallback_rule(rule) {
  return String((rule && rule.condition) || "").trim().toLowerCase() === "true";
}

// case 场景（多分支）：`xxx == "STREAM"` 这类单值比较只显示等号右边的值，
// 判断不出来（有 && / ||、或多个 ==）就退回条件本身，截断后用 tooltip 补全
function automation_case_label(condition) {
  const text = String(condition || "").trim();
  // Match the longest comparator first so `a === "X"` does not split on the
  // inner `==` and leave a stray `=` in the label.
  const parts = text.split(/(?:===|!==|==|!=)/);
  if (parts.length !== 2 || /(&&|\|\|)/.test(text)) {
    return text.length > 10 ? `${text.slice(0, 10)}…` : text;
  }
  return parts[1].trim().replace(/^["']|["']$/g, "");
}

function automation_gateway_branch(node, target_id) {
  const rules = automation_gateway_rules(node);
  if (rules.length === 0) return null;
  // JS 条件（condition_language: "js"）在标签上没有任何差别，只在 tooltip
  // 里加个 [JS] 前缀，免得把两种语法的条件看混。
  const is_js =
    String((node.config && node.config.condition_language) || "")
      .trim()
      .toLowerCase() === "js";
  const prefix = is_js ? "[JS] " : "";
  const target = String(target_id || "");
  const rule = rules.find(
    (item) => String(item.target_id || item.target || "") === target,
  );
  if (!rule) {
    return {
      kind: "unconfigured",
      is_js,
      label: "未配置",
      title: `${prefix}这条连线在「${node.name || node.id}」的 rules 里没有对应条件，引擎不会走到这个节点`,
    };
  }
  const condition = String(rule.condition || "").trim();
  if (automation_is_fallback_rule(rule)) {
    return {
      kind: "no",
      is_js,
      label: "否",
      title: `${prefix}其它情况（condition: true）→ ${target}`,
    };
  }
  const condition_count = rules.filter(
    (item) => !automation_is_fallback_rule(item),
  ).length;
  // 只有一个条件分支时就是 yes/no 语义；多个条件（case）时直接标出各自的条件值
  if (condition_count === 1) {
    return {
      kind: "yes",
      is_js,
      label: "是",
      title: `${prefix}${condition} → ${target}`,
    };
  }
  return {
    kind: "case",
    is_js,
    label: automation_case_label(condition),
    title: `${prefix}${condition} → ${target}`,
  };
}

function automation_node_connectors(node, nodes) {
  const by_id = new Map((nodes || []).map((item) => [item.id, item]));
  const node_label = (node_id) => {
    const target = by_id.get(node_id);
    return (target && (target.name || target.id)) || node_id;
  };
  const source = (node && (node.name || node.id)) || "";
  const is_gateway = automation_gateway_rules(node).length > 0;
  const connectors = ((node && node.next_ids) || []).map((to) => {
    const branch = automation_gateway_branch(node, to);
    return {
      kind: "insert",
      to,
      title: branch
        ? `在「${source}」的「${branch.label}」分支（${node_label(to)}）前插入节点`
        : `在「${source}」与「${node_label(to)}」之间插入节点`,
    };
  });
  connectors.push({
    kind: "add",
    to: "",
    title: is_gateway
      ? `在「${source}」后新增分支（新分支还需要在该节点的 rules 里补条件）`
      : `在「${source}」后新增节点`,
  });
  // 连接圆点列表会随 next_ids 变（改 rules / 插入 / 删除），要有稳定的 key
  connectors.forEach((connector) => {
    connector.id = `${connector.kind}-${connector.to}`;
  });
  return connectors;
}

function automation_edit_edges(nodes) {
  const edges = [];
  (nodes || []).forEach((node) => {
    (node.next_ids || []).forEach((target, index) => {
      edges.push({
        id: `${node.id}-${target}-${index}`,
        from: node.id,
        to: target,
        // 第几条出边：一个节点有多条出边时，每条线要接在自己那个连接圆点上
        index,
      });
    });
  });
  return edges;
}

function automation_edge_path(edge, positions) {
  const from = positions[edge.from];
  const to = positions[edge.to];
  if (!from || !to) return "";
  const x1 = from.left + 180;
  // 第 index 条出边对齐到第 index 个连接圆点（绿点是「新增」，排在所有出边之后）
  const y1 =
    from.top +
    automation_flow_connector_anchor +
    (Number(edge.index) || 0) * automation_flow_connector_gap;
  const x2 = to.left;
  const y2 = to.top + automation_flow_connector_anchor;
  const middle = Math.max(36, Math.abs(x2 - x1) / 2);
  return `M ${x1} ${y1} C ${x1 + middle} ${y1}, ${x2 - middle} ${y2}, ${x2} ${y2}`;
}

// 条件分支出边的分支标签：按 edge id 给出「是 / 否 / case 值」和标签要贴的曲线中点
function automation_edge_labels(nodes, position_overrides) {
  const labels = {};
  const layout = automation_edit_layout(nodes, position_overrides);
  automation_edit_edges(nodes).forEach((edge) => {
    const source = (nodes || []).find((item) => item.id === edge.from);
    const branch = automation_gateway_branch(source, edge.to);
    if (!branch) return;
    const from = layout.positions[edge.from];
    const to = layout.positions[edge.to];
    if (!from || !to) return;
    const x1 = from.left + 180;
    const y1 =
      from.top +
      automation_flow_connector_anchor +
      (Number(edge.index) || 0) * automation_flow_connector_gap;
    const x2 = to.left;
    const y2 = to.top + automation_flow_connector_anchor;
    labels[edge.id] = {
      ...branch,
      left: (x1 + x2) / 2,
      top: (y1 + y2) / 2,
    };
  });
  return labels;
}

function automation_refresh_flow_canvas(canvas, nodes, position_overrides) {
  if (!canvas) return;
  const layout = automation_edit_layout(nodes, position_overrides);
  canvas.style.width = `${layout.width}px`;
  canvas.style.height = `${layout.height}px`;
  canvas.querySelectorAll(".group-automation-flow-edge").forEach((path) => {
    const edge = {
      from: path.getAttribute("data-flow-from"),
      to: path.getAttribute("data-flow-to"),
      index: Number(path.getAttribute("data-flow-index")) || 0,
    };
    path.setAttribute("d", automation_edge_path(edge, layout.positions));
  });
  // 拖动节点时 edit_nodes 还没提交，标签位置得跟连线一样手动刷新，否则会滞后到松手才跳
  const labels = automation_edge_labels(nodes, position_overrides);
  canvas.querySelectorAll(".group-automation-flow-edge-label").forEach((element) => {
    const label = labels[element.getAttribute("data-flow-edge")];
    if (!label) return;
    element.style.left = `${label.left}px`;
    element.style.top = `${label.top}px`;
  });
}

const automation_flow_min_zoom = 0.25;
const automation_flow_max_zoom = 2;
const automation_flow_grid_size = 22;
const automation_flow_minimap_size = {
  width: 176,
  height: 112,
  padding: 8,
};

export function AutomationFlowGraph(props) {
  const vm$ = props.store;
  const editable = Boolean(props.editable);
  const FlowPrimitive = Timeless.ui.FlowPrimitive;
  const flow$ = new Timeless.vm.FlowCanvasModel({
    nodes: [],
    edges: [],
    nodesDraggable: editable,
    nodesConnectable: false,
    multiSelect: false,
    minZoom: automation_flow_min_zoom,
    maxZoom: automation_flow_max_zoom,
  });
  const position_overrides = {};
  const flow_node_models = new Map();
  const viewport_ = refobj({ ...flow$.viewport });
  const minimap_geometry_ = refobj(
    calculate_flow_minimap_geometry(
      { width: 350, height: 320 },
      { width: 350, height: 320 },
      flow$.viewport,
      automation_flow_minimap_size,
    ),
  );
  const flow_edges_ = refarr(
    automation_edit_edges(vm$.state.edit_nodes.value || []),
  );
  // 条件分支出边上的分支标签（是 / 否 / case 值）：一次算好所有边再按 id 取，
  // 每条边各建一个 computed 的话订阅者太多，某个订阅者出错会静默吞掉后面的通知
  const flow_edge_labels_ = computed(vm$.state.edit_nodes, (nodes) =>
    automation_edge_labels(nodes, position_overrides),
  );
  let root_element = null;
  let canvas_element = null;
  let resize_observer = null;
  let active_pointer_cleanup = null;
  let wheel_handler = null;
  const stop_edge_sync = vm$.state.edit_nodes.subscribe({
    onChange(nodes) {
      flow_edges_.as(automation_edit_edges(nodes), { reset: true });
      refresh_minimap(nodes);
    },
  });
  const stop_viewport_sync = flow$.onViewportChange((viewport) => {
    viewport_.as({ ...viewport });
    refresh_minimap();
  });

  function viewport_size() {
    return {
      width: Math.max(1, root_element ? root_element.clientWidth : 1),
      height: Math.max(1, root_element ? root_element.clientHeight : 1),
    };
  }

  function refresh_minimap(nodes = vm$.state.edit_nodes.value) {
    const layout = automation_edit_layout(nodes, position_overrides);
    const size = viewport_size();
    const zoom = Math.max(automation_flow_min_zoom, flow$.viewport.zoom || 1);
    if (canvas_element) {
      canvas_element.style.width = `${Math.max(
        layout.width,
        size.width / zoom,
      )}px`;
      canvas_element.style.height = `${Math.max(
        layout.height,
        size.height / zoom,
      )}px`;
    }
    minimap_geometry_.as(
      calculate_flow_minimap_geometry(
        layout,
        size,
        flow$.viewport,
        automation_flow_minimap_size,
      ),
    );
  }

  function set_flow_zoom(next_zoom, anchor) {
    const viewport = flow$.viewport;
    const old_zoom = Math.max(automation_flow_min_zoom, viewport.zoom || 1);
    const zoom = Math.min(
      automation_flow_max_zoom,
      Math.max(automation_flow_min_zoom, next_zoom),
    );
    const size = viewport_size();
    const point = anchor || {
      x: size.width / 2,
      y: size.height / 2,
    };
    const world_x = (point.x - viewport.x) / old_zoom;
    const world_y = (point.y - viewport.y) / old_zoom;
    flow$.setViewport({
      x: point.x - world_x * zoom,
      y: point.y - world_y * zoom,
      zoom,
    });
  }

  function fit_flow_to_view() {
    const nodes = vm$.state.edit_nodes.value || [];
    if (nodes.length === 0) {
      flow$.resetView();
      return;
    }
    const layout = automation_edit_layout(nodes, position_overrides);
    const bounds = Object.values(layout.positions).reduce(
      (result, position) => ({
        min_x: Math.min(result.min_x, position.left),
        min_y: Math.min(result.min_y, position.top),
        max_x: Math.max(result.max_x, position.left + 180),
        max_y: Math.max(result.max_y, position.top + 104),
      }),
      {
        min_x: Infinity,
        min_y: Infinity,
        max_x: -Infinity,
        max_y: -Infinity,
      },
    );
    const size = viewport_size();
    const padding = 48;
    const content_width = Math.max(1, bounds.max_x - bounds.min_x);
    const content_height = Math.max(1, bounds.max_y - bounds.min_y);
    const zoom = Math.min(
      1,
      automation_flow_max_zoom,
      Math.max(
        automation_flow_min_zoom,
        Math.min(
          (size.width - padding * 2) / content_width,
          (size.height - padding * 2) / content_height,
        ),
      ),
    );
    flow$.setViewport({
      x: (size.width - content_width * zoom) / 2 - bounds.min_x * zoom,
      y: (size.height - content_height * zoom) / 2 - bounds.min_y * zoom,
      zoom,
    });
  }

  function ensure_flow_node(node, position) {
    let flow_node = flow_node_models.get(node.id);
    if (!flow_node) {
      flow_node = new Timeless.vm.FlowNodeModel({
        id: node.id,
        type: node.type,
        position: { x: position.left, y: position.top },
        width: 180,
        height: 104,
        data: node,
      });
      flow_node.setCanvas$(flow$);
      flow$.addNode(flow_node);
      flow_node_models.set(node.id, flow_node);
    } else {
      flow_node.type = node.type;
      flow_node.data = node;
      if (!position_overrides[node.id]) {
        flow_node.position = { x: position.left, y: position.top };
      }
    }
    return flow_node;
  }

  function reconcile_flow_nodes(nodes) {
    const active_ids = new Set((nodes || []).map((node) => node.id));
    flow_node_models.forEach((flow_node, node_id) => {
      if (active_ids.has(node_id)) return;
      flow_node_models.delete(node_id);
      delete position_overrides[node_id];
      flow$.removeNode(node_id);
    });
    const layout = automation_edit_layout(nodes, position_overrides);
    // 布局会随 edit_nodes 变化（插入节点把后面的整体右移），模型里的坐标要一起走，
    // 否则下次从该节点开始拖拽会以旧坐标起算，节点会跳回老位置
    Object.entries(layout.positions).forEach(([node_id, position]) => {
      const flow_node = flow_node_models.get(node_id);
      if (flow_node) flow_node.position = { x: position.left, y: position.top };
    });
    return layout;
  }

  function stop_active_pointer() {
    if (!active_pointer_cleanup) return;
    const cleanup = active_pointer_cleanup;
    active_pointer_cleanup = null;
    cleanup();
  }

  function start_canvas_pan(event) {
    if (event.button !== 0) return;
    const target = event.target;
    if (
      target &&
      typeof target.closest === "function" &&
      target.closest(
        ".graph-node-shell-target, .group-automation-flow-controls, .group-automation-flow-minimap",
      )
    ) {
      return;
    }
    event.preventDefault();
    stop_active_pointer();
    const start_x = event.clientX - flow$.viewport.x;
    const start_y = event.clientY - flow$.viewport.y;
    if (root_element) root_element.classList.add("is-panning");

    const handle_move = (move_event) => {
      flow$.setViewport({
        x: move_event.clientX - start_x,
        y: move_event.clientY - start_y,
      });
    };
    const handle_up = () => stop_active_pointer();
    active_pointer_cleanup = () => {
      document.removeEventListener("mousemove", handle_move);
      document.removeEventListener("mouseup", handle_up);
      if (root_element) root_element.classList.remove("is-panning");
    };
    document.addEventListener("mousemove", handle_move);
    document.addEventListener("mouseup", handle_up);
  }

  function move_viewport_from_minimap(event, minimap_element) {
    const geometry = minimap_geometry_.value;
    if (!geometry || geometry.scale <= 0) return;
    const rect = minimap_element.getBoundingClientRect();
    // 面板有 1px 边框：绘制用的 left/top 以 padding 盒为原点，
    // 所以点击坐标也要减去边框宽度，才能和画出来的节点/视口框对齐
    const origin_x = rect.left + minimap_element.clientLeft;
    const origin_y = rect.top + minimap_element.clientTop;
    // 以缩略图整框为可拖范围：内容之外的留白同样映射到世界坐标，
    // 否则宽扁流程图的竖直方向只能在内容那一条细带里拖
    const local_x = Math.min(
      geometry.width,
      Math.max(0, event.clientX - origin_x),
    );
    const local_y = Math.min(
      geometry.height,
      Math.max(0, event.clientY - origin_y),
    );
    const world_x =
      geometry.world_left + (local_x - geometry.offset_x) / geometry.scale;
    const world_y =
      geometry.world_top + (local_y - geometry.offset_y) / geometry.scale;
    const size = viewport_size();
    flow$.setViewport({
      x: size.width / 2 - world_x * flow$.viewport.zoom,
      y: size.height / 2 - world_y * flow$.viewport.zoom,
    });
  }

  function start_minimap_drag(event) {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    stop_active_pointer();
    const minimap_element = event.currentTarget;
    move_viewport_from_minimap(event, minimap_element);

    const handle_move = (move_event) => {
      move_viewport_from_minimap(move_event, minimap_element);
    };
    const handle_up = () => stop_active_pointer();
    active_pointer_cleanup = () => {
      document.removeEventListener("mousemove", handle_move);
      document.removeEventListener("mouseup", handle_up);
    };
    document.addEventListener("mousemove", handle_move);
    document.addEventListener("mouseup", handle_up);
  }

  function handle_minimap_keydown(event) {
    const amount = event.shiftKey ? 120 : 48;
    const viewport = flow$.viewport;
    const movement = {
      ArrowLeft: { x: viewport.x + amount },
      ArrowRight: { x: viewport.x - amount },
      ArrowUp: { y: viewport.y + amount },
      ArrowDown: { y: viewport.y - amount },
    }[event.key];
    if (movement) {
      event.preventDefault();
      flow$.setViewport(movement);
      return;
    }
    if (event.key === "Home") {
      event.preventDefault();
      fit_flow_to_view();
    }
  }

  function handle_canvas_wheel(event) {
    event.preventDefault();
    if (!root_element) return;
    const rect = root_element.getBoundingClientRect();
    const sensitivity = event.ctrlKey ? 0.01 : 0.001;
    const factor = Math.max(0.5, 1 - event.deltaY * sensitivity);
    set_flow_zoom(flow$.viewport.zoom * factor, {
      x: event.clientX - rect.left,
      y: event.clientY - rect.top,
    });
  }

  function start_node_drag(event, node, flow_node) {
    if (!editable || event.button !== 0) return;
    // 开始节点位置固定，不进入拖拽（选中/编辑配置不受影响）
    if (node.id === vm$.state.edit_start_node_id.value) return;
    event.preventDefault();
    event.stopPropagation();
    vm$.methods.selectEditNode(node.id);

    const node_element = event.currentTarget;
    const shell_element = node_element.closest(".graph-node-shell-target");
    const canvas_element = node_element.closest(".graph-content");
    if (!shell_element || !canvas_element) return;

    stop_active_pointer();
    flow_node.pointerDown(event.clientX, event.clientY);
    shell_element.classList.add("is-dragging");
    let has_moved = false;

    const handle_move = (move_event) => {
      flow_node.pointerMove(move_event.clientX, move_event.clientY);
      // 不做边界限制：可以拖到画布原点的左上（负坐标）
      const position = {
        left: flow_node.position.x,
        top: flow_node.position.y,
      };
      has_moved = true;
      flow_node.position = { x: position.left, y: position.top };
      position_overrides[node.id] = position;
      vm$.methods.stageEditNodePosition(node.id, {
        x: position.left,
        y: position.top,
      });
      shell_element.style.left = `${position.left}px`;
      shell_element.style.top = `${position.top}px`;
      automation_refresh_flow_canvas(
        canvas_element,
        vm$.state.edit_nodes.value,
        position_overrides,
      );
      refresh_minimap();
    };

    const handle_up = (up_event) => {
      flow_node.pointerUp(up_event.clientX, up_event.clientY);
      if (has_moved) {
        vm$.methods.moveEditNode(node.id, {
          x: flow_node.position.x,
          y: flow_node.position.y,
        });
      }
      stop_active_pointer();
    };

    active_pointer_cleanup = () => {
      document.removeEventListener("mousemove", handle_move);
      document.removeEventListener("mouseup", handle_up);
      shell_element.classList.remove("is-dragging");
    };
    document.addEventListener("mousemove", handle_move);
    document.addEventListener("mouseup", handle_up);
  }

  function flow_control_button(options) {
    return View(
      {
        as: "button",
        class:
          "w-control-height-sm min-w-control-height-sm h-control-height-sm min-h-control-height-sm p-0 dm-button--surface dm-button dm-button--icon dm-focus-ring",
        attributes: {
          type: "button",
          title: options.label,
          "aria-label": options.label,
        },
        onMouseDown(event) {
          event.stopPropagation();
        },
        onClick(event) {
          event.stopPropagation();
          options.action();
        },
      },
      [
        Timeless.Icon({
          name: options.icon,
          size: 16,
          attributes: { "aria-hidden": "true" },
        }),
      ],
    );
  }

  function minimap_node_style(node) {
    return computed(minimap_geometry_, (geometry) => {
      const layout = automation_edit_layout(
        vm$.state.edit_nodes.value,
        position_overrides,
      );
      const position = layout.positions[node.id] || { left: 0, top: 0 };
      return {
        left: `${
          geometry.offset_x +
          (position.left - geometry.world_left) * geometry.scale
        }px`,
        top: `${
          geometry.offset_y +
          (position.top - geometry.world_top) * geometry.scale
        }px`,
        width: `${Math.max(4, 180 * geometry.scale)}px`,
        height: `${Math.max(4, 104 * geometry.scale)}px`,
      };
    });
  }

  function minimap_viewport_style() {
    return computed(minimap_geometry_, (geometry) => ({
      left: `${geometry.viewport.left}px`,
      top: `${geometry.viewport.top}px`,
      width: `${geometry.viewport.width}px`,
      height: `${geometry.viewport.height}px`,
    }));
  }

  return FlowPrimitive.Root(
    {
      store: flow$,
      class: [
        "relative min-h-320px p-0 overflow-hidden isolation-isolate dm-panel--soft dm-panel cursor-grab touch-action-none graph-surface graph-canvas ",
        editable ? "is-editable" : "is-readonly",
        props.class || "",
      ].join(" "),
      attributes: {
        n: "automation-flow",
        "aria-label": "流程画布",
      },
      onMounted(event) {
        root_element = event.target.get$elm();
        wheel_handler = handle_canvas_wheel;
        root_element.addEventListener("wheel", wheel_handler, {
          passive: false,
        });
        if (typeof ResizeObserver !== "undefined") {
          resize_observer = new ResizeObserver(() => refresh_minimap());
          resize_observer.observe(root_element);
        }
        refresh_minimap();
      },
      onMouseDown(event) {
        start_canvas_pan(event);
      },
      beforeUnmounted() {
        stop_active_pointer();
        if (resize_observer) resize_observer.disconnect();
        if (root_element && wheel_handler) {
          root_element.removeEventListener("wheel", wheel_handler);
        }
        stop_edge_sync();
        stop_viewport_sync();
      },
    },
    [
      FlowPrimitive.Background({
        class: "absolute z-base inset-0 bg-color-canvas bg-image-flow-grid pointer-events-none",
        style: computed(viewport_, (viewport) => {
          const zoom = Math.max(automation_flow_min_zoom, viewport.zoom || 1);
          const grid_size = automation_flow_grid_size * zoom;
          return {
            "background-position": `${viewport.x}px ${viewport.y}px`,
            "background-size": `${grid_size}px ${grid_size}px`,
          };
        }),
        attributes: {
          n: "automation-flow-background",
          "aria-hidden": "true",
        },
      }),
      FlowPrimitive.Canvas(
        {
          store: flow$,
          class: "absolute z-raised top-0 left-0 box-sizing-border-box min-w-full min-h-full origin-top-left will-change-transform graph-content",
          style: combine(
            {
              nodes: vm$.state.edit_nodes,
              minimap: minimap_geometry_,
              viewport: viewport_,
            },
            ({ nodes, viewport }) => {
              const layout = reconcile_flow_nodes(nodes);
              const size = viewport_size();
              const zoom = Math.max(
                automation_flow_min_zoom,
                viewport.zoom || 1,
              );
              return {
                width: `${Math.max(layout.width, size.width / zoom)}px`,
                height: `${Math.max(layout.height, size.height / zoom)}px`,
                transform:
                  `translate(${viewport.x}px, ${viewport.y}px) ` +
                  `scale(${zoom})`,
                "transform-origin": "0 0",
              };
            },
          ),
          onMounted(event) {
            canvas_element = event.target.get$elm();
            refresh_minimap();
          },
        },
        [
          FlowPrimitive.EdgeLayer({ class: "absolute z-base inset-0 pointer-events-none" }, [
            For({
              each: flow_edges_,
              key: "id",
              render(edge) {
                const path_ = computed(vm$.state.edit_nodes, (nodes) => {
                  const layout = automation_edit_layout(
                    nodes,
                    position_overrides,
                  );
                  return automation_edge_path(edge, layout.positions);
                });
                return SVG.SVG(
                  {
                    class: "absolute inset-0 w-full h-full overflow-visible text-color-border-strong pointer-events-none",
                    width: "100%",
                    height: "100%",
                    xmlns: "http://www.w3.org/2000/svg",
                    "aria-hidden": "true",
                  },
                  [
                    SVG.Path({
                      class: "stroke-currentcolor stroke-linecap-round vector-effect-non-scaling-stroke group-automation-flow-edge graph-edge ",
                      d: path_,
                      stroke: "currentColor",
                      "stroke-width": "2",
                      "stroke-linecap": "round",
                      fill: "none",
                      dataset: {
                        "flow-from": edge.from,
                        "flow-to": edge.to,
                        "flow-index": String(edge.index || 0),
                      },
                    }),
                  ],
                );
              },
            }),
          ]),
          // 条件分支出边上的分支标签（是 / 否 / case 值），贴在曲线中点。
          // 这里刻意不用 Show：Show 在 For 的 DOM 补丁路径（新增/复用）下不会重建子树，
          // 没有分支的边也要渲染一个 div，靠 is-hidden 类隐藏。
          FlowPrimitive.NodeLayer({ class: "absolute z-base inset-0 pointer-events-none" }, [
            For({
              each: flow_edges_,
              key: "id",
              render(edge) {
                const label_ = computed(
                  flow_edge_labels_,
                  (labels) => labels[edge.id] || null,
                );
                return View(
                  {
                    class: computed(label_, (label) =>
                      label
                        ? `absolute max-w-160px py-0 px-1 overflow-hidden text-11px leading-16px whitespace-nowrap text-ellipsis border-default border-solid border-color-default rounded-full bg-color-bg-surface text-color-text-secondary translate-center group-automation-flow-edge-label graph-edge-label is-${label.kind}${label.is_js ? " is-js" : ""}`
                        : "absolute max-w-160px py-0 px-1 overflow-hidden text-11px leading-16px whitespace-nowrap text-ellipsis border-default border-solid border-color-default rounded-full bg-color-bg-surface text-color-text-secondary translate-center group-automation-flow-edge-label graph-edge-label is-hidden",
                    ),
                    style: computed(label_, (label) => ({
                      left: `${label ? label.left : 0}px`,
                      top: `${label ? label.top : 0}px`,
                    })),
                    attributes: {
                      n: `automation-flow-edge-label-${edge.from}-${edge.to}`,
                      "data-flow-edge": edge.id,
                      title: computed(label_, (label) =>
                        label ? label.title : "",
                      ),
                    },
                  },
                  [computed(label_, (label) => (label ? label.label : ""))],
                );
              },
            }),
          ]),
          For({
            each: vm$.state.edit_nodes,
            key: "id",
            render(node_) {
              const node =
                node_ && node_.value !== undefined ? node_.value : node_;
              // 坐标必须是响应式的：For 只对新增的 key 重跑 render，复用的 key
              // 只做 registry 代理更新，一次性的 left/top 会永远停在旧位置。
              // 插入节点会把 to 及其右侧的所有节点整体右移，连线（computed）跟着走，
              // 但节点壳停在原地 → 连线和节点脱节，必须再拖一下才恢复。
              const position_ = computed(vm$.state.edit_nodes, () => {
                const layout = automation_edit_layout(
                  vm$.state.edit_nodes.value,
                  position_overrides,
                );
                return layout.positions[node.id] || { left: 20, top: 20 };
              });
              const position = position_.value;
              const is_start =
                node.id === vm$.state.edit_start_node_id.value ||
                node.type === "StartNode";
              // 结束节点是流程终点，不再挂连接圆点
              const is_end = node.type === "EndNode";
              const selected = computed(
                vm$.state.selected_edit_node_id,
                (node_id) => editable && node_id === node.id,
              );
              const execution_status = computed(
                vm$.state.node_execution_states,
                (states) => (states && states[node.id]) || null,
              );
              // 连接圆点也要响应式：改 rules / 增删分支后 next_ids 会变，
              // 而 For 复用节点壳时不会重跑 render，一次性的圆点列表会永远停在旧分支上
              const connectors_ = computed(vm$.state.edit_nodes, (nodes) => {
                const current = (nodes || []).find(
                  (item) => item.id === node.id,
                );
                if (!current) return [];
                return automation_node_connectors(current, nodes);
              });
              const flow_node = ensure_flow_node(node, position);
              return FlowPrimitive.Node(
                {
                  store: flow$,
                  nodeId: node.id,
                  class: "absolute w-180px min-h-104px graph-node-shell-target graph-node-shell",
                  style: computed(position_, (p) => ({
                    left: `${p.left}px`,
                    top: `${p.top}px`,
                  })),
                },
                [
                  editable && is_start
                    ? View({
                        class: "absolute bottom-full left-0 z-raised",
                        attributes: { n: "automation-start-run" },
                        onMouseDown(event) { event.stopPropagation(); },
                        onClick(event) { event.stopPropagation(); },
                      }, [Button({
                        store: vm$.ui.btn_run_flow$,
                        attributes: { n: "automation-run-flow", type: "button" },
                      }, ["手动执行"])])
                    : null,
                  View(
                    {
                    class: combine(
                      { selected, execution_status },
                      ({ selected: is_selected, execution_status: status }) =>
                        `relative w-full min-h-104px py-2 px-2.5 dm-panel user-select-none graph-node-target graph-node ${is_start ? " is-start" : ""}${is_selected ? " is-selected" : ""}${status
                            ? ` has-execution is-${automation_execution_status_class(status.status)}`
                            : ""}`,
                      ),
                      attributes: {
                        n: `automation-flow-node-${node.id}`,
                        title: node.name || node.id,
                        role: editable ? "button" : undefined,
                        tabindex: editable ? "0" : undefined,
                        "aria-pressed": computed(selected, (is_selected) =>
                          editable ? String(is_selected) : undefined,
                        ),
                      },
                      onMouseDown(event) {
                        start_node_drag(event, node, flow_node);
                      },
                      // 点节点只做选中/编辑，执行日志走 header 上的日志浮标
                      onClick() {
                        if (!editable) return;
                        vm$.methods.selectEditNode(node.id);
                      },
                      onKeyDown(event) {
                        if (!editable) return;
                        automation_activate(event, () => {
                          vm$.methods.selectEditNode(node.id);
                        });
                      },
                    },
                    [
                      View({
                        class: "flex items-center gap-1.5",
                        attributes: { n: `automation-flow-node-header-${node.id}` },
                      }, [
                        View({
                          class: "grid h-8 grow-0 shrink-0 basis-8 place-items-center rounded-control bg-color-primary-soft text-color-primary",
                          attributes: {
                            n: `automation-flow-node-icon-${node.id}`,
                            "aria-hidden": "true",
                          },
                        }, [
                          Timeless.Icon({
                            name: vm$.methods.flowIcon(node.type),
                            size: 20,
                            attributes: { n: `automation-flow-node-type-icon-${node.id}` },
                          }),
                        ]),
                        View({
                          class: "flex min-w-0 flex-1 flex-col items-start gap-1",
                          attributes: { n: `automation-flow-node-heading-${node.id}` },
                        }, [
                          Tag(
                            {
                              variant: is_start ? "success" : "info",
                              class: "min-h-4.5 flex-none px-1 text-caption",
                              attributes: { n: `automation-flow-node-type-${node.id}` },
                            },
                            [vm$.methods.flowLabel(node.type)],
                          ),
                          View({
                            class: "max-w-full min-w-0 overflow-hidden text-base font-bold whitespace-nowrap text-ellipsis text-color-text-primary",
                            attributes: {
                              n: `automation-flow-node-name-${node.id}`,
                              title: node.name || vm$.methods.flowLabel(node.type),
                            },
                          }, [node.name || vm$.methods.flowLabel(node.type)]),
                        ]),
                        View({
                          class: "inline-flex flex-none items-center gap-1",
                          attributes: { n: `automation-flow-node-badges-${node.id}` },
                        }, [
                          // 执行过的节点才有日志可看（when 为空则不渲染）；
                          // 浮标只在可编辑的画布上出现，只读页没有浮窗容器
                          editable
                            ? Show({
                                when: execution_status,
                                ok() {
                                  return View(
                                    {
                                      as: "button",
                                      class:
                                        "box-sizing-border-box w-4.5 min-h-4.5 p-0 dm-button--ghost text-color-text-secondary dm-button icon-action-soft dm-focus-ring",
                                      attributes: {
                                        n: `automation-flow-node-log-${node.id}`,
                                        type: "button",
                                        title: `查看「${node.name || node.id}」执行日志`,
                                        "aria-label": `查看「${node.name || node.id}」执行日志`,
                                      },
                                      onMouseDown(event) {
                                        // 不要冒泡到节点根，否则会顺带触发节点拖拽
                                        event.stopPropagation();
                                      },
                                      onClick(event) {
                                        if (!editable) return;
                                        event.stopPropagation();
                                        vm$.methods.selectEditNode(node.id);
                                        vm$.methods.openNodeExecutionWindow(
                                          node.id,
                                          {
                                            x: event.clientX,
                                            y: event.clientY,
                                          },
                                        );
                                      },
                                    },
                                    [
                                      Timeless.Icon({
                                        name: "scroll-text",
                                        size: 12,
                                        attributes: { "aria-hidden": "true" },
                                      }),
                                    ],
                                  );
                                },
                              })
                            : null,
                        ]),
                      ]),
                      View({
                        class: "flex items-center mt-1 gap-1.5",
                        attributes: { n: `automation-flow-node-meta-${node.id}` },
                      }, [
                        View({
                          class: "block min-w-0 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-color-text-secondary",
                          attributes: {
                            n: `automation-flow-node-id-${node.id}`,
                            title: node.id,
                          },
                        }, [node.id]),
                      ]),
                      is_start ? View({
                        class: "mt-1 overflow-hidden font-mono text-caption leading-base whitespace-nowrap text-ellipsis text-color-text-secondary graph-node-description",
                        attributes: { n: "automation-start-trigger-count" },
                      }, [computed(vm$.state.edit_triggers, (triggers) => {
                        const cron_count = triggers.filter((trigger) => trigger.type === "Cron").length;
                        const event_count = triggers.filter((trigger) => trigger.type === "Event").length;
                        return `手动 · ${cron_count} 个定时 · ${event_count} 个事件`;
                      })]) : null,
                      Show({
                        when:
                          !is_start && node.config && Object.keys(node.config).length > 0,
                        ok() {
                          const config_text = JSON.stringify(node.config);
                          return View(
                            {
                              class: "mt-1 overflow-hidden font-mono text-caption leading-base whitespace-nowrap text-ellipsis text-color-text-secondary graph-node-description",
                              attributes: {
                                n: `automation-flow-node-schema-${node.id}`,
                                title: config_text,
                              },
                            },
                            [config_text],
                          );
                        },
                      }),
                    Show({
                      when: execution_status,
                      ok() {
                        return View(
                          {
                            class: computed(
                              execution_status,
                              (status) =>
                                `inline-flex absolute right-2 bottom-2 min-h-4.5 items-center px-1.5 text-caption font-semibold leading-1 border-default border-solid border-color-info rounded-badge bg-color-info-soft text-color-info execution-status-badge is-${automation_execution_status_class(status.status)}`,
                            ),
                            attributes: { role: "status" },
                          },
                          [
                            computed(execution_status, (status) =>
                              vm$.methods.nodeExecutionStatusLabel(status.status),
                            ),
                          ],
                        );
                      },
                    }),
                    ],
                  ),
                  editable && !is_end
                    ? View(
                        {
                          class: "absolute inset-0 pointer-events-none",
                          attributes: {
                            n: `automation-flow-connectors-${node.id}`,
                          },
                        },
                        [
                          For({
                            each: connectors_,
                            key: "id",
                            render(connector, index_) {
                              const is_insert = connector.kind === "insert";
                              return View(
                                {
                                  as: "button",
                                  class: `absolute -right-1 z-raised w-2 h-2 min-w-0 p-0 border-strong border-solid border-color-surface rounded-full cursor-pointer -translate-y-1/2 hover:brightness-108 pointer-events-auto dm-focus-ring graph-connector ${is_insert ? "is-connected" : "is-empty"}`,
                                  attributes: {
                                    n: is_insert
                                      ? `automation-flow-insert-${node.id}-${connector.to}`
                                      : `automation-flow-add-next-${node.id}`,
                                    type: "button",
                                    title: connector.title,
                                    "aria-label": connector.title,
                                    "aria-haspopup": "dialog",
                                  },
                                  // 第几个圆点决定它的纵向位置，插入/删除分支后要跟着挪
                                  style: computed(index_, (idx) => ({
                                    top: `${
                                      automation_flow_connector_anchor +
                                      Math.max(0, idx) *
                                        automation_flow_connector_gap
                                    }px`,
                                  })),
                                  onMouseDown(event) {
                                    // 不要冒泡到节点根，否则会顺带触发节点拖拽
                                    event.stopPropagation();
                                  },
                                  onClick(event) {
                                    event.stopPropagation();
                                    if (is_insert) {
                                      vm$.methods.openInsertDialog(
                                        node.id,
                                        connector.to,
                                      );
                                    } else {
                                      vm$.methods.openAddDialog("", node.id);
                                    }
                                  },
                                },
                                [],
                              );
                            },
                          }),
                        ],
                      )
                    : null,
                ],
              );
            },
          }),
        ],
      ),
      FlowPrimitive.Controls(
        {
          store: flow$,
          class: "flex absolute z-sticky bottom-3 left-3 flex-col p-1 gap-1 bg-color-bg-surface dm-panel shadow-md group-automation-flow-controls",
          attributes: {
            n: "automation-flow-controls",
            "aria-label": "画布缩放控制",
            role: "group",
          },
          onMouseDown(event) {
            event.stopPropagation();
          },
        },
        [
          flow_control_button({
            icon: "plus",
            label: "放大画布",
            action() {
              set_flow_zoom(flow$.viewport.zoom + 0.1);
            },
          }),
          View(
            {
              class: "min-w-control-height-sm font-mono text-caption leading-snug text-center text-color-text-secondary",
              attributes: { "aria-live": "polite" },
            },
            [
              computed(
                minimap_geometry_,
                () => `${Math.round(flow$.viewport.zoom * 100)}%`,
              ),
            ],
          ),
          flow_control_button({
            icon: "minus",
            label: "缩小画布",
            action() {
              set_flow_zoom(flow$.viewport.zoom - 0.1);
            },
          }),
          flow_control_button({
            icon: "maximize",
            label: "适应全部节点",
            action: fit_flow_to_view,
          }),
          flow_control_button({
            icon: "rotate-ccw",
            label: "重置画布视图",
            action() {
              flow$.resetView();
            },
          }),
        ],
      ),
      FlowPrimitive.Minimap(
        {
          store: flow$,
          class: "absolute z-sticky left-16 bottom-3 w-176px h-112px p-0 overflow-hidden border-color-color-border-strong bg-color-bg-surface dm-panel shadow-md cursor-crosshair touch-action-none group-automation-flow-minimap dm-focus-ring",
          attributes: {
            n: "automation-flow-minimap",
            role: "button",
            tabindex: "0",
            title: "点击或拖动定位画布；方向键移动视图",
            "aria-label": "流程缩略图，点击或拖动定位，方向键移动视图",
          },
          onMouseDown(event) {
            start_minimap_drag(event);
          },
          onClick(event) {
            event.stopPropagation();
          },
          onKeyDown(event) {
            handle_minimap_keydown(event);
          },
        },
        [
          View({ class: "absolute inset-0 pointer-events-none" }, [
            For({
              each: vm$.state.edit_nodes,
              key: "id",
              render(node_) {
                const node =
                  node_ && node_.value !== undefined ? node_.value : node_;
                const selected = computed(
                  vm$.state.selected_edit_node_id,
                  (node_id) => editable && node_id === node.id,
                );
                const execution_status = computed(
                  vm$.state.node_execution_states,
                  (states) => (states && states[node.id]) || null,
                );
                return View({
                  class: combine(
                    { selected, execution_status },
                    ({ selected: is_selected, execution_status: status }) =>
                      `absolute box-sizing-border-box border-default border-solid border-color-strong rounded-xs bg-color-bg-interactive minimap-node ${node.type === "StartNode" ? " is-start" : ""}${is_selected ? " is-selected" : ""}${status
                          ? ` has-execution is-${automation_execution_status_class(status.status)}`
                          : ""}`,
                  ),
                  style: minimap_node_style(node),
                  attributes: { "aria-hidden": "true" },
                });
              },
            }),
          ]),
          View({
            class: "absolute box-sizing-border-box border-strong border-solid border-color-primary-border-strong rounded-xs bg-color-primary-soft pointer-events-none",
            style: minimap_viewport_style(),
            attributes: { "aria-hidden": "true" },
          }),
        ],
      ),
    ],
  );
}

export function AutomationTriggerOption(props) {
  const value = props.value;
  const selected = computed(props.current, (type) => type === value);
  return View(
    {
      class: computed(
        selected,
        (is_selected) =>
          `min-h-72px flex-col gap-0-5 motion-reduce:transition-none dm-button--choice-card dm-button dm-interactive dm-focus-ring${is_selected ? " is-selected" : ""}`,
      ),
      attributes: {
        n: `${props.namePrefix}-trigger-option-${value.toLowerCase()}`,
        role: "radio",
        tabindex: "0",
        "aria-checked": computed(selected, (is_selected) =>
          is_selected ? "true" : "false",
        ),
      },
      onClick() {
        props.onSelect(value);
      },
      onKeyDown(event) {
        automation_activate(event, () => props.onSelect(value));
      },
    },
    [
      View({ class: "text-sm font-semibold text-color-text-primary" }, [props.label]),
      View({ class: "text-caption leading-base text-color-text-secondary" }, [props.hint]),
    ],
  );
}

export function AutomationRunPipelineDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.run_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-run-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["开始执行"]),
        DialogDescription({}, [
          "填写开始节点需要的参数后开始执行；可选参数可留空。开始后立即返回，执行进度在画布上实时更新。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "flex flex-col gap-3" }, [
          For({
            key: "uid",
            each: vm$.state.run_params,
            render(param_) {
              const param =
                param_ && param_.value !== undefined ? param_.value : param_;
              const label = [param.key, param.required ? " *" : ""];
              const control =
                param.type === "boolean"
                  ? View({ class: "dm-flex" }, [
                      Checkbox({
                        store: param.checkbox$,
                        attributes: { n: `automation-run-param-${param.key}` },
                      }),
                    ])
                  : Input({
                      store: param.input$,
                      attributes: {
                        n: `automation-run-param-${param.key}`,
                        placeholder: param.required ? "必填" : "可选",
                      },
                    });
              return View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
                View({ class: "text-xs font-semibold text-color-text-secondary" }, label),
                control,
              ]);
            },
          }),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_run_cancel$,
            attributes: { n: "automation-run-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_run_submit$,
            attributes: { n: "automation-run-submit", type: "button" },
          },
          ["开始执行"],
        ),
      ]),
    ],
  );
}

/** For 的 render 回调里拿到的是 ref，取值时统一解包 */
export function automation_unwrap_ref(value) {
  return value && value.value !== undefined ? value.value : value;
}

/** 单条执行日志的入参 / 节点行为 / 输出 / error，面板与浮窗共用 */
export function automation_execution_log_data(entry, vm$) {
  const data = [
    ["入参", entry.input],
    ["节点行为", entry.behavior],
    ["输出", entry.output],
  ].map(([label, value]) =>
    View({ class: "min-w-0" }, [
      View({ class: "mb-1 text-caption font-semibold text-color-text-secondary" }, [label]),
      View({ as: "pre", class: "max-h-180px m-0 p-2 overflow-auto font-mono text-caption leading-base whitespace-pre-wrap word-break-break-word border-default border-solid border-color-default rounded-content bg-color-bg-surface text-color-text-primary log-panel-value" }, [
        vm$.methods.formatExecutionValue(value),
      ]),
    ]),
  );
  if (entry.error) {
    data.push(
      ErrorMessage({ tip: vm$.methods.tip, error: entry.error }),
    );
  }
  return data;
}

/**
 * 画布节点执行日志浮窗容器。多窗口同时存在，各自独立位置；
 * Portal 挂到 body，彻底脱离画布的 transform / overflow。
 *
 * 必须 Portal 包住 For：For 通过自己的 host 在挂载点内增删节点，而 primitive
 * Portal 没有 destroy、「For 里放 Portal」会在每次关窗时把窗口 DOM 泄漏在 body 里。
 * onUnmounted 时 closeAll，路由离开先清掉窗口节点。
 */
export function AutomationNodeExecutionWindows(props) {
  const vm$ = props.store;
  return Timeless.Portal(
    {
      onUnmounted() {
        vm$.methods.closeAllNodeExecutionWindows();
      },
    },
    [
      For({
        each: vm$.state.execution_windows,
        key: "id",
        render(item_) {
          return AutomationNodeExecutionWindow({
            store: vm$,
            node_id: item_.id,
          });
        },
      }),
    ],
  );
}

/**
 * 单个节点的执行日志浮窗。窗口自身位置只被 Window 构建时读一次，
 * 因此天然独立；拖动只改本窗口，结束拖拽时回写 WindowManager。
 */
export function AutomationNodeExecutionWindow(props) {
  const vm$ = props.store;
  const node_id = props.node_id;
  const node_id_ = ref(node_id);
  const node_ = computed(
    combine({ nodes: vm$.state.edit_nodes, node_id: node_id_ }, (v) => v),
    ({ nodes, node_id: id }) =>
      (nodes || []).find((item) => item.id === id) || null,
  );
  const status_ = computed(
    combine(
      { states: vm$.state.node_execution_states, node_id: node_id_ },
      (v) => v,
    ),
    ({ states, node_id: id }) => (states && states[id]) || null,
  );
  const logs_ = computed(
    combine({ logs: vm$.state.execution_logs, node_id: node_id_ }, (v) => v),
    ({ logs, node_id: id }) =>
      (logs || []).filter((entry) => entry.node_id === id).reverse(),
  );
  const z_ = computed(
    combine(
      { z: vm$.state.execution_window_z, node_id: node_id_ },
      (v) => v,
    ),
    ({ z, node_id: id }) => (z && z[id]) || 240,
  );
  const focused_ = computed(
    combine(
      { active: vm$.state.execution_window_active_id, node_id: node_id_ },
      (v) => v,
    ),
    ({ active, node_id: id }) => active === id,
  );

  const position = vm$.methods.executionWindowPosition(node_id);

  const window$ = Timeless.Window(
    {
      title: computed(
        node_,
        (node) =>
          `${(node && (node.name || node.id)) || node_id} · 执行日志`,
      ),
      x: position.x,
      y: position.y,
      width: 420,
      // ref → 层级变化只改 style，不重建窗口
      zIndex: z_,
      class: computed(
        focused_,
        (is_focused) =>
          `flex min-w-0 flex-col overflow-hidden border-default border-solid border-color-default rounded-overlay bg-color-bg-surface shadow-popover floating-window ${is_focused ? " is-focused" : ""}`,
      ),
      headerClass: "flex items-center py-2 px-3 gap-2 border-b-default border-b-solid border-b-color-default bg-color-bg-subtle cursor-grab active:cursor-grabbing touch-action-none user-select-none floating-window-header ",
      bodyClass: "flex max-h-60vh flex-col p-3 gap-2 overflow-auto",
      closeClass: "inline-flex w-6 h-6 flex-none items-center justify-center text-md leading-1 border-0 rounded-content bg-transparent hover:bg-color-bg-interactive text-color-text-secondary hover:text-color-text-primary cursor-pointer dm-focus-ring",
      closeLabel: "关闭执行日志",
      attributes: {
        n: "automation-node-execution-window",
        "data-node-id": node_id,
      },
      onActivate() {
        vm$.methods.focusNodeExecutionWindow(node_id);
      },
      onDragFinish(next) {
        const applied = vm$.methods.moveNodeExecutionWindow(node_id, next);
        // clamp 生效时把最终位置同步回窗口
        if (applied && (applied.x !== next.x || applied.y !== next.y)) {
          window$.methods.setPosition(applied);
        }
      },
      onClose() {
        vm$.methods.closeNodeExecutionWindow(node_id);
      },
    },
    [
      Show({
        when: status_,
        ok() {
          return View(
            {
              class: computed(
                status_,
                (status) =>
                  `inline-flex min-h-4.5 align-self-flex-start items-center px-1.5 text-caption font-semibold leading-1 border-default border-solid border-color-info rounded-badge bg-color-info-soft text-color-info execution-status-badge is-${automation_execution_status_class(
                    status.status,
                  )}`,
              ),
            },
            [
              computed(status_, (status) =>
                vm$.methods.nodeExecutionStatusLabel(status.status),
              ),
            ],
          );
        },
      }),
      Show({
        when: computed(logs_, (logs) => logs.length === 0),
        ok() {
          return View({ class: "text-sm text-color-text-secondary" }, [
            "本次执行暂无该节点的日志。",
          ]);
        },
      }),
      For({
        each: logs_,
        key: "_execution_key",
        render(entry_) {
          const entry = automation_unwrap_ref(entry_);
          return View(
            {
              class: "overflow-hidden border-default border-solid border-color-default rounded-content log-panel-compact ",
              attributes: { n: `automation-node-log-${entry.node_id}` },
            },
            [
              View({ class: "flex items-center py-1.5 px-3 gap-2 border-b-default border-b-solid border-b-color-default bg-color-bg-subtle" }, [
                View(
                  {
                    class: `inline-flex min-h-4.5 items-center px-1.5 text-caption font-semibold leading-1 border-default border-solid border-color-info rounded-badge bg-color-info-soft text-color-info execution-status-badge is-${automation_execution_status_class(
                      entry.outcome,
                    )}`,
                  },
                  [vm$.methods.nodeExecutionStatusLabel(entry.outcome)],
                ),
                View({ class: "font-mono text-caption font-variant-numeric-tabular-nums text-color-text-secondary" }, [
                  `第 ${entry.attempt || 1} 次 · ${entry.duration_ms || 0} ms`,
                ]),
              ]),
              View(
                { class: "grid grid-cols-3 max-900:grid-cols-1fr pt-2 pb-3 px-3 gap-2 border-t-default border-t-solid border-t-color-default bg-color-bg-subtle log-panel-body" },
                automation_execution_log_data(entry, vm$),
              ),
            ],
          );
        },
      }),
    ],
  );
  return window$;
}


export function AutomationRunHistory(props) {
  const vm$ = props.store;
  return View({ class: "grid min-w-0 m-4 p-3 gap-3 dm-panel", attributes: { n: "automation-run-history" } }, [
    View({ class: "flex items-center justify-between gap-2" }, [
      View({ class: "font-semibold text-color-text-primary" }, ["最近运行"]),
      Button({ store: vm$.ui.btn_refresh_runs$, attributes: { type: "button", "aria-label": "刷新运行记录" } }, ["刷新运行记录"]),
    ]),
    View({ class: "flex max-h-200px min-w-0 flex-wrap gap-2 overflow-auto" }, [
      For({ each: vm$.state.runs, key: "id", render(value) {
        const run = value && value.value !== undefined ? value.value : value;
        return Button({ store: vm$.ui.btn_select_run$.bind(run), attributes: { type: "button", title: run.id, "aria-label": `查看运行 ${run.id}` } }, [`${vm$.methods.formatTime(run.created_at)} · ${run.status}`]);
      } }),
      Show({ when: computed(vm$.state.runs, (runs) => !runs.length), ok() { return View({ class: "text-sm text-color-text-secondary" }, ["暂无运行记录。保存 Pipeline 后手动执行或配置触发器。"]); } }),
    ]),
    For({ each: computed(vm$.state.selected_run, (run) => run ? [{ ...run, render_key: `${run.id}-${run.status}-${run.updated_at}` }] : []), key: "render_key", render(value) {
      const run = value && value.value !== undefined ? value.value : value;
      let logs = [];
      try { logs = JSON.parse(run.node_logs || "[]"); } catch { /* Legacy records may have no trace. */ }
      return View({ class: "grid min-w-0 pt-3 gap-2 border-t border-t-solid border-t-color-default" }, [
        View({ class: "flex min-w-0 flex-wrap items-center gap-2 text-sm" }, [
          automation_status_badge(run.status), View({ class: "font-mono text-xs word-break-break-all" }, [run.id]),
          View({ class: "text-color-text-secondary" }, [`当前节点：${run.current_node || "等待执行"}`]),
        ]),
        run.error ? ErrorMessage({ tip: vm$.methods.tip, error: run.error }) : null,
        run.retry_of ? View({ class: "text-xs text-color-text-secondary" }, [`重试来源：${run.retry_of}`]) : null,
        run.status === "WAITING" ? View({ class: "grid min-w-0 gap-2" }, [
          View({ as: "label", attributes: { for: "automation-manual-response" }, class: "text-sm text-color-text-secondary" }, ["人工确认输入 JSON"]),
          Textarea({ store: vm$.ui.input_manual_response$, class: "font-mono text-xs", attributes: { id: "automation-manual-response", rows: "3", spellcheck: "false", "aria-label": "人工确认输入 JSON" } }),
          View({ class: "text-xs text-color-text-secondary" }, ["按人工节点的 input_schema 填写，例如 {\"approved\":true}；恢复沿用启动时的流程定义。"]),
          Button({ store: vm$.ui.btn_resume_run$, attributes: { type: "button" } }, ["提交确认并继续"]),
        ]) : null,
        ["QUEUED", "RUNNING", "WAITING"].includes(run.status) ? Button({ store: vm$.ui.btn_cancel_run$, attributes: { type: "button" } }, ["取消运行"]) : null,
        ["FAILED", "CANCELLED"].includes(run.status) ? View({ class: "grid gap-2" }, [
          View({ class: "text-xs text-color-text-secondary" }, ["重试会从头执行，已发生的下载或通知可能重复，请先核对外部结果。"]),
          View({ class: "flex flex-wrap gap-2" }, [
            Button({ store: vm$.ui.btn_retry_run$, attributes: { type: "button" } }, ["按原流程重试"]),
            Button({ store: vm$.ui.btn_retry_current_run$, attributes: { type: "button" } }, ["按当前已保存流程重试"]),
          ]),
        ]) : null,
        logs.length ? View({ as: "details", class: "min-w-0 text-sm", attributes: { n: "automation-persisted-logs" } }, [
          View({ as: "summary", class: "text-color-text-secondary" }, [`节点执行记录（最近 ${logs.length} 条）`]),
          View({ as: "pre", class: "max-h-200px overflow-auto font-mono text-xs whitespace-pre-wrap word-break-break-all" }, [JSON.stringify(logs, null, 2)]),
        ]) : null,
      ].filter(Boolean));
    } }),
  ]);
}

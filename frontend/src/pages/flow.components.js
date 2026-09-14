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
      class: "automation-trigger",
      attributes: { n: `automation-trigger-${normalized}` },
    },
    [props.label],
  );
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
      class: "automation-status",
      attributes: { n: `automation-status-${normalized.toLowerCase()}` },
    },
    [normalized],
  );
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
        "dm-empty-state automation-empty-state",
        props.compact ? "automation-empty-state--compact" : "",
        props.detail ? "automation-empty-state--detail" : "",
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
      View({ class: "content-state-title" }, [props.title]),
      props.description
        ? View({ class: "content-state-text" }, [props.description])
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
        "dm-button dm-focus-ring dm-button--sm automation-open-link",
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
  return View({ class: "automation-feedback container" }, [
    Show({
      when: vm$.state.error,
      ok() {
        return Alert(
          {
            variant: "destructive",
            class: "automation-alert",
            attributes: { n: "automation-error-alert" },
          },
          [
            AlertTitle({ attributes: { n: "automation-error-title" } }, [
              "请求失败",
            ]),
            AlertDescription({}, [vm$.state.error.value]),
          ],
        );
      },
    }),
    Show({
      when: vm$.state.notice,
      ok() {
        return Alert(
          {
            class: "automation-alert",
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
  return View({ class: "content-toolbar automation-workspace-toolbar" }, [
    View({ class: "automation-workspace-toolbar__identity" }, [
      View({ class: "automation-workspace-toolbar__title-wrap" }, [
        View({ class: "automation-workspace-toolbar__eyebrow" }, [
          is_edit ? "编辑 Pipeline" : "Pipeline 详情",
        ]),
        View({ class: "automation-workspace-toolbar__title" }, [
          computed(vm$.state.selected_pipeline, (flow) =>
            flow ? flow.name || flow.id : vm$.state.selected_flow_id.value,
          ),
        ]),
      ]),
    ]),
    View(
      { class: "automation-workspace-toolbar__actions" },
      [
        is_edit
          ? Tag(
              {
                variant: "warning",
                class: computed(vm$.state.dirty, (dirty) =>
                  dirty
                    ? "automation-dirty-tag"
                    : "automation-dirty-tag is-clean",
                ),
              },
              [
                computed(vm$.state.dirty, (dirty) =>
                  dirty ? "有未保存变更" : "已保存",
                ),
              ],
            )
          : null,
        Button(
          {
            store: vm$.ui.btn_run_flow$,
            attributes: { n: "automation-run-flow", type: "button" },
          },
          ["立即执行"],
        ),
        !is_edit
          ? Button(
              {
                store: vm$.ui.btn_schedule_create$,
                attributes: {
                  n: "automation-create-schedule",
                  type: "button",
                },
              },
              ["创建自动化"],
            )
          : null,
        is_edit
          ? Button(
              {
                store: vm$.ui.btn_save_flow$,
                attributes: { n: "automation-save-flow", type: "button" },
              },
              ["保存 Pipeline"],
            )
          : AutomationPipelineLink({
              flowId: vm$.state.selected_flow_id.value,
              mode: "edit",
              label: "编辑 Pipeline",
              primary: true,
            }),
        is_edit
          ? Button(
              {
                store: vm$.ui.btn_delete_flow$,
                attributes: {
                  n: "automation-delete-flow",
                  type: "button",
                },
              },
              ["删除 Pipeline"],
            )
          : null,
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
      left: Math.max(16, Number(position.left) || 0),
      top: Math.max(16, Number(position.top) || 0),
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
  return { positions, width, height: Math.max(max_bottom + 40, 320) };
}

function automation_edit_edges(nodes) {
  const edges = [];
  (nodes || []).forEach((node) => {
    (node.next_ids || []).forEach((target, index) => {
      edges.push({
        id: `${node.id}-${target}-${index}`,
        from: node.id,
        to: target,
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
  const y1 = from.top + 52;
  const x2 = to.left;
  const y2 = to.top + 52;
  const middle = Math.max(36, Math.abs(x2 - x1) / 2);
  return `M ${x1} ${y1} C ${x1 + middle} ${y1}, ${x2 - middle} ${y2}, ${x2} ${y2}`;
}

function automation_refresh_flow_canvas(canvas, nodes, position_overrides) {
  if (!canvas) return;
  const layout = automation_edit_layout(nodes, position_overrides);
  canvas.style.width = `${layout.width}px`;
  canvas.style.height = `${layout.height}px`;
  canvas.querySelectorAll(".automation-flow-edge").forEach((path) => {
    const edge = {
      from: path.getAttribute("data-flow-from"),
      to: path.getAttribute("data-flow-to"),
    };
    path.setAttribute("d", automation_edge_path(edge, layout.positions));
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
    return automation_edit_layout(nodes, position_overrides);
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
        ".automation-flow-node-shell, .automation-flow-controls, .automation-flow-minimap",
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
    const local_x = Math.min(
      geometry.offset_x + geometry.world_width * geometry.scale,
      Math.max(geometry.offset_x, event.clientX - rect.left),
    );
    const local_y = Math.min(
      geometry.offset_y + geometry.world_height * geometry.scale,
      Math.max(geometry.offset_y, event.clientY - rect.top),
    );
    const world_x = (local_x - geometry.offset_x) / geometry.scale;
    const world_y = (local_y - geometry.offset_y) / geometry.scale;
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
    event.preventDefault();
    event.stopPropagation();
    vm$.methods.selectEditNode(node.id);

    const node_element = event.currentTarget;
    const shell_element = node_element.closest(".automation-flow-node-shell");
    const canvas_element = node_element.closest(".automation-flow-canvas");
    if (!shell_element || !canvas_element) return;

    stop_active_pointer();
    flow_node.pointerDown(event.clientX, event.clientY);
    shell_element.classList.add("is-dragging");
    let has_moved = false;

    const handle_move = (move_event) => {
      flow_node.pointerMove(move_event.clientX, move_event.clientY);
      const position = {
        left: Math.max(16, flow_node.position.x),
        top: Math.max(16, flow_node.position.y),
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
          "dm-button dm-button--surface dm-button--icon dm-focus-ring automation-flow-control",
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
        left: `${geometry.offset_x + position.left * geometry.scale}px`,
        top: `${geometry.offset_y + position.top * geometry.scale}px`,
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
        "dm-panel dm-panel--soft automation-flow-scroll",
        editable ? "is-editable" : "is-readonly",
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
        class: "automation-flow-background",
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
          class: "automation-flow-canvas",
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
          FlowPrimitive.EdgeLayer({ class: "automation-flow-edge-layer" }, [
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
                    class: "automation-flow-edges",
                    width: "100%",
                    height: "100%",
                    xmlns: "http://www.w3.org/2000/svg",
                    "aria-hidden": "true",
                  },
                  [
                    SVG.Path({
                      class: "automation-flow-edge",
                      d: path_,
                      stroke: "currentColor",
                      "stroke-width": "2",
                      "stroke-linecap": "round",
                      fill: "none",
                      dataset: {
                        "flow-from": edge.from,
                        "flow-to": edge.to,
                      },
                    }),
                  ],
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
              const layout = automation_edit_layout(
                vm$.state.edit_nodes.value,
                position_overrides,
              );
              const position = layout.positions[node.id] || {
                left: 20,
                top: 20,
              };
              const is_start =
                node.id === vm$.state.edit_start_node_id.value ||
                node.type === "StartNode";
              const selected = computed(
                vm$.state.selected_edit_node_id,
                (node_id) => editable && node_id === node.id,
              );
              const execution_status = computed(
                vm$.state.node_execution_states,
                (states) => (states && states[node.id]) || null,
              );
              const flow_node = ensure_flow_node(node, position);
              return FlowPrimitive.Node(
                {
                  store: flow$,
                  nodeId: node.id,
                  class: "automation-flow-node-shell",
                  style: {
                    left: `${position.left}px`,
                    top: `${position.top}px`,
                  },
                },
                [
                  View(
                    {
                    class: combine(
                      { selected, execution_status },
                      ({ selected: is_selected, execution_status: status }) =>
                        `dm-panel automation-flow-node${
                          is_start ? " is-start" : ""
                        }${is_selected ? " is-selected" : ""}${
                          status
                            ? ` has-execution is-${automation_execution_status_class(status.status)}`
                            : ""
                        }`,
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
                      onClick() {
                        if (editable) vm$.methods.selectEditNode(node.id);
                      },
                      onKeyDown(event) {
                        if (!editable) return;
                        automation_activate(event, () =>
                          vm$.methods.selectEditNode(node.id),
                        );
                      },
                    },
                    [
                      View({ class: "automation-flow-node__header" }, [
                        View({ class: "automation-flow-node__name" }, [
                          node.name || node.id,
                        ]),
                        Tag(
                          {
                            variant: is_start ? "success" : "info",
                            class: "automation-flow-node__type",
                          },
                          [vm$.methods.flowLabel(node.type)],
                        ),
                      ]),
                      View({ class: "automation-flow-node__id" }, [node.id]),
                      Show({
                        when:
                          node.config && Object.keys(node.config).length > 0,
                        ok() {
                          const config_text = JSON.stringify(node.config);
                          return View(
                            {
                              class: "automation-flow-node__schema",
                              attributes: { title: config_text },
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
                                `automation-flow-node__execution is-${automation_execution_status_class(status.status)}`,
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
                  editable
                    ? View(
                        {
                          as: "button",
                          class:
                            "dm-button dm-button--outline dm-button--icon dm-focus-ring automation-flow-node__add-next",
                          attributes: {
                            n: `automation-flow-add-next-${node.id}`,
                            type: "button",
                            title: `在「${node.name || node.id}」后新增节点`,
                            "aria-label": `在「${node.name || node.id}」后新增节点`,
                            "aria-haspopup": "dialog",
                          },
                          onClick(event) {
                            event.stopPropagation();
                            vm$.methods.openAddDialog("", node.id);
                          },
                        },
                        [
                          Timeless.Icon({
                            name: "plus",
                            size: 16,
                            attributes: { "aria-hidden": "true" },
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
          class: "dm-panel automation-flow-controls",
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
              class: "automation-flow-controls__zoom",
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
          class: "dm-panel automation-flow-minimap dm-focus-ring",
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
          View({ class: "automation-flow-minimap__nodes" }, [
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
                      `automation-flow-minimap__node${
                        node.type === "StartNode" ? " is-start" : ""
                      }${is_selected ? " is-selected" : ""}${
                        status
                          ? ` has-execution is-${automation_execution_status_class(status.status)}`
                          : ""
                      }`,
                  ),
                  style: minimap_node_style(node),
                  attributes: { "aria-hidden": "true" },
                });
              },
            }),
          ]),
          View({
            class: "automation-flow-minimap__viewport",
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
          `dm-button dm-button--choice-card dm-interactive dm-focus-ring automation-trigger-option${
            is_selected ? " is-selected" : ""
          }`,
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
      View({ class: "automation-trigger-option__name" }, [props.label]),
      View({ class: "automation-trigger-option__hint" }, [props.hint]),
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
        DialogTitle({}, ["立即执行"]),
        DialogDescription({}, [
          "填写开始节点需要的参数后执行；可选参数可留空。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
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
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, label),
                control,
              ]);
            },
          }),
          Show({
            when: vm$.state.run_error,
            ok() {
              return View(
                {
                  class: "dm-alert is-destructive",
                  attributes: { role: "alert" },
                },
                [vm$.state.run_error],
              );
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
          ["执行"],
        ),
      ]),
    ],
  );
}

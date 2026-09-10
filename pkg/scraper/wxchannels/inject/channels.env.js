/**
 * @file Channels page runtime environment override.
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before channels.env.js");
}

var ChannelsEnv = (() => {
  var defaults = {
    apiOrigin: "127.0.0.1:2022",
    apiProtocol: "http",
    channelsWSURL: "ws://127.0.0.1:2022/ws/channels"
  };
  var derived = {};
  return {
    get(name) {
      let v = WXEnv.get(name);
      if (typeof v !== "undefined") {
        return v;
      }
      return defaults[name];
    },
  }
})();

/**
 * Channels automation console.
 *
 * The panel is self-contained and uses the API envelope exposed by the
 * application plus the read-only flow visualization payload from
 * pkg/flowengine. It does not depend on Channel page internals.
 */
var ChannelsAutomation = (() => {
  var panel_root = null;
  var style_element = null;
  var state = {
    loading: false,
    error: "",
    notice: "",
    pipelines: [],
    schedules: [],
    runs: [],
    selected_flow_id: "",
    selected_pipeline: null,
    selected_schedule_id: "",
    form_open: false,
    form_flow_id: "",
    tab: "pipelines",
  };

  function api_origin() {
    var origin = String(ChannelsEnv.get("apiOrigin") || "").trim();
    if (origin && !/^https?:\/\//i.test(origin)) {
      origin = ChannelsEnv.get("apiProtocol") + "://" + origin;
    }
    return origin.replace(/\/$/, "");
  }

  function api_url(path, query) {
    var url = api_origin() + path;
    var params = new URLSearchParams();
    Object.keys(query || {}).forEach(function (key) {
      if (query[key] !== undefined && query[key] !== null && query[key] !== "") {
        params.append(key, query[key]);
      }
    });
    var search = params.toString();
    return search ? url + "?" + search : url;
  }

  async function request_api(path, options) {
    options = options || {};
    var response = await fetch(api_url(path, options.query), {
      method: options.method || "GET",
      credentials: "include",
      headers: options.body ? { "Content-Type": "application/json" } : undefined,
      body: options.body ? JSON.stringify(options.body) : undefined,
    });
    var payload = null;
    try {
      payload = await response.json();
    } catch (err) {
      void err;
    }
    if (!response.ok) {
      throw new Error("HTTP " + response.status);
    }
    if (!payload || payload.code !== 0) {
      throw new Error((payload && payload.msg) || "接口返回异常");
    }
    return payload.data;
  }

  function el(tag, options) {
    options = options || {};
    var node = document.createElement(tag);
    if (options.class) node.className = options.class;
    if (options.text !== undefined) node.textContent = options.text;
    if (options.attrs) {
      Object.keys(options.attrs).forEach(function (key) {
        if (options.attrs[key] !== undefined && options.attrs[key] !== null) {
          node.setAttribute(key, options.attrs[key]);
        }
      });
    }
    if (options.style) Object.assign(node.style, options.style);
    (options.children || []).forEach(function (child) {
      if (child) node.appendChild(child);
    });
    if (options.props) {
      Object.keys(options.props).forEach(function (key) {
        node[key] = options.props[key];
      });
    }
    if (typeof options.on_click === "function") {
      node.addEventListener("click", function (event) {
        event.stopPropagation();
        options.on_click(event);
      });
    }
    if (typeof options.on_keydown === "function") {
      node.addEventListener("keydown", function (event) {
        event.stopPropagation();
        options.on_keydown(event);
      });
    }
    return node;
  }

  function text(tag, value, class_name) {
    return el(tag, {
      text: value == null || value === "" ? "-" : String(value),
      class: class_name,
    });
  }

  function button(label, class_name, handler, disabled) {
    return el("button", {
      text: label,
      class: ["ca-button", class_name].filter(Boolean).join(" "),
      attrs: { type: "button" },
      props: { disabled: !!disabled },
      on_click: handler,
    });
  }

  function status_node(value) {
    var normalized = String(value || "UNKNOWN").toUpperCase();
    var tone = "info";
    if (["COMPLETED", "ENABLED", "TRUE", "YES"].includes(normalized)) {
      tone = "success";
    } else if (["FAILED", "FALSE", "NO", "CANCELLED"].includes(normalized)) {
      tone = "danger";
    } else if (["RUNNING", "WAITING", "QUEUED"].includes(normalized)) {
      tone = "warning";
    }
    return el("span", {
      text: normalized,
      class: "ca-status ca-status--" + tone,
    });
  }

  function format_time(value) {
    var number = Number(value);
    if (!Number.isFinite(number) || number <= 0) return "-";
    if (number < 1000000000000) number *= 1000;
    var date = new Date(number);
    if (Number.isNaN(date.getTime())) return "-";
    var pad = function (input) {
      return String(input).padStart(2, "0");
    };
    return (
      date.getFullYear() +
      "-" +
      pad(date.getMonth() + 1) +
      "-" +
      pad(date.getDate()) +
      " " +
      pad(date.getHours()) +
      ":" +
      pad(date.getMinutes()) +
      ":" +
      pad(date.getSeconds())
    );
  }

  function schema_text(schema) {
    if (!Array.isArray(schema) || schema.length === 0) return "无";
    return schema
      .map(function (field) {
        return field.key + ":" + (field.type || "any");
      })
      .join("、");
  }

  function keyboard_activate(handler) {
    return function (event) {
      if (event.key !== "Enter" && event.key !== " " && event.key !== "Spacebar") {
        return;
      }
      event.preventDefault();
      handler();
    };
  }

  function find_pipeline(flow_id) {
    return state.pipelines.find(function (pipeline) {
      return pipeline.id === flow_id;
    });
  }

  function set_error(error) {
    state.error = error && error.message ? error.message : String(error || "");
    state.loading = false;
    render();
  }

  async function load_data(options) {
    options = options || {};
    state.loading = true;
    state.error = "";
    if (!options.silent) state.notice = "";
    render();
    try {
      var pipeline_payload = await request_api(
        "/api/channels/postprocess/flows",
      );
      state.pipelines = (pipeline_payload && pipeline_payload.flows) || [];
      var schedule_payload = await request_api("/api/v1/automation/schedules", {
        query: { page: 1, page_size: 100 },
      });
      state.schedules = (schedule_payload && schedule_payload.list) || [];
      if (!state.selected_flow_id && state.pipelines.length > 0) {
        state.selected_flow_id = state.pipelines[0].id;
        state.selected_pipeline = state.pipelines[0];
      }
      if (state.selected_schedule_id) {
        var selected = state.schedules.find(function (item) {
          return item.id === state.selected_schedule_id;
        });
        if (!selected) {
          state.selected_schedule_id = "";
          state.runs = [];
        }
      }
      if (state.selected_schedule_id) {
        await load_runs(state.selected_schedule_id, true);
      }
      state.loading = false;
      render();
    } catch (error) {
      set_error(error);
    }
  }

  async function select_pipeline(flow_id) {
    state.selected_flow_id = flow_id;
    state.selected_pipeline = find_pipeline(flow_id) || null;
    state.tab = "pipelines";
    render();
    try {
      var payload = await request_api("/api/channels/postprocess/flows", {
        query: { flow_id: flow_id },
      });
      state.selected_pipeline = payload && payload.flows && payload.flows[0];
      state.loading = false;
      render();
    } catch (error) {
      set_error(error);
    }
  }

  async function load_runs(schedule_id, skip_render) {
    var payload = await request_api("/api/v1/automation/runs", {
      query: { schedule_id: schedule_id, page: 1, page_size: 20 },
    });
    state.runs = (payload && payload.list) || [];
    if (!skip_render) render();
  }

  async function select_schedule(id) {
    state.selected_schedule_id = id;
    state.tab = "schedules";
    state.loading = true;
    render();
    try {
      var schedule = await request_api(
        "/api/v1/automation/schedules/" + encodeURIComponent(id),
      );
      var index = state.schedules.findIndex(function (item) {
        return item.id === id;
      });
      if (index >= 0) state.schedules[index] = schedule;
      else state.schedules.unshift(schedule);
      state.selected_flow_id = schedule.flow_id;
      state.selected_pipeline = find_pipeline(schedule.flow_id) || null;
      await load_runs(id, true);
      if (!state.selected_pipeline) {
        await select_pipeline(schedule.flow_id);
      }
      state.loading = false;
      state.tab = "schedules";
      render();
    } catch (error) {
      set_error(error);
    }
  }

  async function schedule_action(id, action) {
    try {
      var schedule = state.schedules.find(function (item) {
        return item.id === id;
      });
      var metadata = schedule_metadata(schedule);
      if (action === "toggle" && metadata.type === "Cron") {
        await request_api(
          "/api/v1/automation/schedules/" + encodeURIComponent(id) + "/toggle",
          { method: "POST" },
        );
        state.notice = "自动化状态已更新";
      } else if (action === "trigger") {
        await request_api(
          "/api/v1/automation/schedules/" + encodeURIComponent(id) + "/trigger",
          {
            method: "POST",
            body: {
              trigger_type:
                metadata.type === "Event"
                  ? "Event"
                  : "Manual",
              event_key: metadata.type === "Event" ? metadata.event_key : "",
            },
          },
        );
        state.notice =
          metadata.type === "Event" ? "事件已触发" : "自动化流程已手动触发";
      }
      await load_data({ keep_selection: true, silent: true });
    } catch (error) {
      set_error(error);
    }
  }

  function parse_initial_data(value) {
    var trimmed = String(value || "").trim();
    if (!trimmed) return {};
    var parsed = JSON.parse(trimmed);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("初始数据必须是 JSON 对象");
    }
    return parsed;
  }

  function schedule_metadata(schedule) {
    var metadata = {
      type: "Cron",
      start_node: "",
      event_key: "",
    };
    if (!schedule) return metadata;
    try {
      var data = parse_initial_data(schedule.initial_data);
      var raw = data.__automation;
      if (raw && typeof raw === "object") {
        if (raw.type) metadata.type = String(raw.type);
        if (raw.start_node) metadata.start_node = String(raw.start_node);
        if (raw.event_key) metadata.event_key = String(raw.event_key);
      }
    } catch (err) {
      void err;
    }
    if (["Cron", "Event", "Manual"].indexOf(metadata.type) < 0) {
      metadata.type = "Cron";
    }
    return metadata;
  }

  function trigger_label(type) {
    if (type === "Event") return "事件触发";
    if (type === "Manual") return "手动触发";
    return "定时触发";
  }

  function effective_start_node(flow, schedule) {
    var metadata = schedule_metadata(schedule);
    if (metadata.start_node && (!flow || !flow.nodes || flow.nodes.some(function (node) {
      return node.id === metadata.start_node;
    }))) {
      return metadata.start_node;
    }
    return flow ? flow.start_node_id : "";
  }

  async function submit_schedule(event) {
    event.preventDefault();
    event.stopPropagation();
    var form = event.currentTarget;
    var submit = form.querySelector("button[type=submit]");
    try {
      var trigger_type = form.trigger_type.value;
      var event_key = form.event_key.value.trim();
      var start_node = form.start_node.value;
      var initial_data = parse_initial_data(form.initial_data.value);
      if (!start_node) throw new Error("请选择开始节点");
      if (trigger_type === "Event" && !event_key) {
        throw new Error("请输入事件 Key");
      }
      initial_data.__automation = {
        type: trigger_type,
        start_node: start_node,
        event_key: trigger_type === "Event" ? event_key : "",
      };
      var body = {
        name: form.name.value.trim(),
        description: form.description.value.trim(),
        cron_expr:
          trigger_type === "Cron"
            ? form.cron_expr.value.trim() || "@daily"
            : "@daily",
        flow_id: form.flow_id.value,
        initial_data: initial_data,
        enabled: trigger_type === "Cron" && form.enabled.checked,
        timeout_sec: Number(form.timeout_sec.value) || 3600,
      };
      if (!body.name) throw new Error("请输入流程名称");
      if (!body.flow_id) throw new Error("请选择 Pipeline");
      if (submit) submit.disabled = true;
      var schedule = await request_api("/api/v1/automation/schedules", {
        method: "POST",
        body: body,
      });
      state.form_open = false;
      state.notice = "自动化流程创建成功";
      state.selected_schedule_id = schedule.id;
      state.tab = "schedules";
      await load_data({ keep_selection: true, silent: true });
      await select_schedule(schedule.id);
    } catch (error) {
      set_error(error);
      if (submit) submit.disabled = false;
    }
    return false;
  }

  function trigger_badge(type) {
    return el("span", {
      text: trigger_label(type),
      class: "ca-trigger ca-trigger--" + String(type || "Cron").toLowerCase(),
    });
  }

  function pipeline_list_view() {
    var list = el("div", {
      class: "ca-pipeline-list",
      children: state.pipelines.map(function (pipeline) {
        var selected = pipeline.id === state.selected_flow_id;
        return el("div", {
          class: "ca-pipeline-card" + (selected ? " is-selected" : ""),
          attrs: { role: "button", tabindex: "0" },
          on_click: function () {
            select_pipeline(pipeline.id);
          },
          on_keydown: keyboard_activate(function () {
            select_pipeline(pipeline.id);
          }),
          children: [
            text("strong", pipeline.name, "ca-pipeline-card__title"),
            text("code", pipeline.id, "ca-pipeline-card__id"),
            el("div", {
              class: "ca-meta",
              children: [
                text("span", (pipeline.nodes || []).length + " 节点"),
                text("span", (pipeline.edges || []).length + " 连线"),
              ],
            }),
          ],
        });
      }),
    });
    if (state.pipelines.length === 0) {
      list.appendChild(
        el("div", {
          class: "ca-empty",
          text: state.loading ? "正在加载 Pipeline..." : "暂无 Pipeline",
        }),
      );
    }
    return list;
  }

  function schedule_list_view() {
    var list = el("div", {
      class: "ca-schedule-list",
      children: state.schedules.map(function (schedule) {
        var selected = schedule.id === state.selected_schedule_id;
        var pipeline = find_pipeline(schedule.flow_id);
        return el("div", {
          class: "ca-schedule-card" + (selected ? " is-selected" : ""),
          on_click: function () {
            select_schedule(schedule.id);
          },
          on_keydown: keyboard_activate(function () {
            select_schedule(schedule.id);
          }),
          children: [
            el("div", {
              class: "ca-schedule-card__header",
              children: [
                text("strong", schedule.name, "ca-schedule-card__title"),
                el("div", {
                  class: "ca-schedule-card__badges",
                  children: [
                    trigger_badge(schedule_metadata(schedule).type),
                    schedule_metadata(schedule).type === "Cron"
                      ? status_node(schedule.enabled ? "ENABLED" : "DISABLED")
                      : null,
                  ],
                }),
              ],
            }),
            text(
              "span",
              pipeline ? pipeline.name : schedule.flow_id,
              "ca-schedule-card__flow",
            ),
            el("div", {
              class: "ca-meta",
              children: [
                text(
                  "code",
                  schedule_metadata(schedule).type === "Cron"
                    ? schedule.cron_expr
                    : schedule_metadata(schedule).event_key ||
                        schedule_metadata(schedule).start_node,
                ),
                text(
                  "span",
                  schedule_metadata(schedule).type === "Cron"
                    ? "下次 " + format_time(schedule.next_run_at)
                    : "开始 " + effective_start_node(pipeline, schedule),
                ),
              ],
            }),
            el("div", {
              class: "ca-card-actions",
              on_click: function (event) {
                event.stopPropagation();
              },
              children: [
                schedule_metadata(schedule).type === "Cron"
                  ? button(
                      schedule.enabled ? "暂停" : "启用",
                      "ca-button--small",
                      function () {
                        schedule_action(schedule.id, "toggle");
                      },
                    )
                  : null,
                button(
                  schedule_metadata(schedule).type === "Event"
                    ? "触发事件"
                    : "手动执行",
                  "ca-button--small",
                  function () {
                    schedule_action(schedule.id, "trigger");
                  },
                ),
              ],
            }),
          ],
        });
      }),
    });
    if (state.schedules.length === 0) {
      list.appendChild(
        el("div", {
          class: "ca-empty",
          text: state.loading ? "正在加载自动化..." : "暂无自动化流程",
        }),
      );
    }
    return list;
  }

  function flow_node_view(node, positions) {
    var position = positions[node.id] || { left: 20, top: 20 };
    return el("div", {
      class:
        "ca-flow-node" +
        (node.id ===
        effective_start_node(
          state.selected_pipeline,
          state.tab === "schedules"
            ? state.schedules.find(function (item) {
                return item.id === state.selected_schedule_id;
              })
            : null,
        )
          ? " is-start"
          : ""),
      style: { left: position.left + "px", top: position.top + "px" },
      children: [
        el("div", {
          class: "ca-flow-node__header",
          children: [
            text("strong", node.name || node.id),
            text("span", node.type, "ca-flow-node__type"),
          ],
        }),
        text("code", node.id, "ca-flow-node__id"),
        text(
          "div",
          "输入：" + schema_text(node.input_schema),
          "ca-flow-node__schema",
        ),
      ],
    });
  }

  function edge_path(edge, positions) {
    var from = positions[edge.from];
    var to = positions[edge.to];
    if (!from || !to) return "";
    var x1 = from.left + 90;
    var y1 = from.top + 58;
    var x2 = to.left + 90;
    var y2 = to.top + 6;
    var middle = Math.max(36, Math.abs(y2 - y1) / 2);
    return (
      "M " +
      x1 +
      " " +
      y1 +
      " C " +
      x1 +
      " " +
      (y1 + middle) +
      ", " +
      x2 +
      " " +
      (y2 - middle) +
      ", " +
      x2 +
      " " +
      y2
    );
  }

  function flow_graph_view(flow) {
    var nodes = (flow && flow.nodes) || [];
    var edges = (flow && flow.edges) || [];
    var min_x = 0;
    var min_y = 0;
    nodes.forEach(function (node) {
      min_x = Math.min(min_x, Number(node.layout && node.layout.x) || 0);
      min_y = Math.min(min_y, Number(node.layout && node.layout.y) || 0);
    });
    var positions = {};
    var max_right = 0;
    var max_bottom = 0;
    var x_values = nodes
      .map(function (node) {
        return Number(node.layout && node.layout.x) || 0;
      })
      .filter(function (value, index, values) {
        return values.indexOf(value) === index;
      })
      .sort(function (a, b) {
        return a - b;
      });
    var min_x_gap = 0;
    for (var index = 1; index < x_values.length; index += 1) {
      var gap = x_values[index] - x_values[index - 1];
      if (gap > 0 && (min_x_gap === 0 || gap < min_x_gap)) {
        min_x_gap = gap;
      }
    }
    var x_scale = min_x_gap > 0 ? Math.max(1, 190 / min_x_gap) : 1;
    nodes.forEach(function (node) {
      var layout = node.layout || {};
      var left = ((Number(layout.x) || 0) - min_x) * x_scale + 28;
      var top = (Number(layout.y) || 0) - min_y + 28;
      positions[node.id] = { left: left, top: top };
      max_right = Math.max(max_right, left + 180);
      max_bottom = Math.max(max_bottom, top + 88);
    });
    var graph = el("div", {
      class: "ca-flow-canvas",
      style: {
        width: Math.max(max_right + 40, 720) + "px",
        height: Math.max(max_bottom + 40, 320) + "px",
      },
    });
    var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("class", "ca-flow-edges");
    svg.setAttribute("width", "100%");
    svg.setAttribute("height", "100%");
    var defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
    var marker = document.createElementNS("http://www.w3.org/2000/svg", "marker");
    marker.setAttribute("id", "ca-flow-arrow");
    marker.setAttribute("viewBox", "0 0 8 8");
    marker.setAttribute("refX", "7");
    marker.setAttribute("refY", "4");
    marker.setAttribute("markerWidth", "7");
    marker.setAttribute("markerHeight", "7");
    marker.setAttribute("orient", "auto-start-reverse");
    var marker_path = document.createElementNS(
      "http://www.w3.org/2000/svg",
      "path",
    );
    marker_path.setAttribute("d", "M 0 0 L 8 4 L 0 8 z");
    marker_path.setAttribute("fill", "rgba(15, 23, 42, 0.45)");
    marker.appendChild(marker_path);
    defs.appendChild(marker);
    svg.appendChild(defs);
    edges.forEach(function (edge) {
      var d = edge_path(edge, positions);
      if (!d) return;
      var line = document.createElementNS("http://www.w3.org/2000/svg", "path");
      line.setAttribute("d", d);
      line.setAttribute(
        "class",
        "ca-flow-edge" + (edge.type === "rule" ? " is-rule" : ""),
      );
      line.setAttribute("marker-end", "url(#ca-flow-arrow)");
      svg.appendChild(line);
      if (edge.label) {
        var label = document.createElementNS(
          "http://www.w3.org/2000/svg",
          "text",
        );
        label.setAttribute(
          "x",
          String(
            (positions[edge.from].left + positions[edge.to].left) / 2 + 80,
          ),
        );
        label.setAttribute(
          "y",
          String(
            (positions[edge.from].top + positions[edge.to].top) / 2 + 52,
          ),
        );
        label.setAttribute("class", "ca-flow-edge-label");
        label.textContent = edge.label;
        svg.appendChild(label);
      }
    });
    graph.appendChild(svg);
    nodes.forEach(function (node) {
      graph.appendChild(flow_node_view(node, positions));
    });
    return el("div", { class: "ca-flow-scroll", children: [graph] });
  }

  function schedule_properties_view(schedule) {
    var metadata = schedule_metadata(schedule);
    var pipeline = find_pipeline(schedule.flow_id);
    var summary = el("section", {
      class: "ca-schedule-summary",
      children: [
        el("div", {
          children: [
            text("span", "流程名称", "ca-property__label"),
            text("strong", schedule.name),
          ],
        }),
        el("div", {
          children: [
            text("span", "触发方式", "ca-property__label"),
            trigger_badge(metadata.type),
          ],
        }),
        el("div", {
          children: [
            text("span", "开始节点", "ca-property__label"),
            text("code", effective_start_node(pipeline, schedule)),
          ],
        }),
        el("div", {
          children: [
            text("span", "触发配置", "ca-property__label"),
            text(
              "code",
              metadata.type === "Cron"
                ? schedule.cron_expr
                : metadata.type === "Event"
                  ? metadata.event_key
                  : "manual",
            ),
          ],
        }),
        el("div", {
          children: [
            text("span", "下次执行", "ca-property__label"),
            text(
              "span",
              metadata.type === "Cron"
                ? format_time(schedule.next_run_at)
                : "由触发动作执行",
            ),
          ],
        }),
        el("div", {
          children: [
            text("span", "状态", "ca-property__label"),
            metadata.type === "Cron"
              ? status_node(schedule.enabled ? "ENABLED" : "DISABLED")
              : trigger_badge(metadata.type),
          ],
        }),
        el("div", {
          class: "ca-property--wide",
          children: [
            text("span", "初始数据", "ca-property__label"),
            text("code", schedule.initial_data || "{}"),
          ],
        }),
      ],
    });
    if (metadata.type === "Event") {
      summary.appendChild(
        el("div", {
          class: "ca-property--wide",
          children: [
            text("span", "事件调用", "ca-property__label"),
            text(
              "code",
              'POST /api/v1/automation/schedules/' +
                schedule.id +
                '/trigger {"trigger_type":"Event"}',
            ),
          ],
        }),
      );
    }
    return summary;
  }

  function detail_view() {
    var flow = state.selected_pipeline;
    if (!flow) {
      return el("div", {
        class: "ca-empty ca-empty--detail",
        text: "请选择一个 Pipeline",
      });
    }
    var schedule = state.schedules.find(function (item) {
      return item.id === state.selected_schedule_id;
    });
    var detail = el("section", {
      class: "ca-detail",
      children: [
        el("header", {
          class: "ca-detail__header",
          children: [
            el("div", {
              children: [
                text("h3", flow.name || flow.id, "ca-detail__title"),
                text("code", flow.id, "ca-detail__subtitle"),
              ],
            }),
            button(
              "从此 Pipeline 创建流程",
              "ca-button--primary",
              function () {
                state.form_open = true;
                state.form_flow_id = flow.id;
                render();
              },
            ),
          ],
        }),
        el("div", {
          class: "ca-properties",
          children: [
            el("div", {
              children: [
                text("span", "上下文 Schema", "ca-property__label"),
                text("code", schema_text(flow.context_schema)),
              ],
            }),
            el("div", {
              children: [
                text("span", "开始节点", "ca-property__label"),
                text("code", flow.start_node_id),
              ],
            }),
            el("div", {
              children: [
                text("span", "节点 / 连线", "ca-property__label"),
                text(
                  "span",
                  (flow.nodes || []).length +
                    " / " +
                    (flow.edges || []).length,
                ),
              ],
            }),
          ],
        }),
        flow_graph_view(flow),
      ],
    });
    if (state.tab === "schedules" && schedule) {
      var runs = el("section", {
        class: "ca-runs",
        children: [
          text("h4", "执行记录", "ca-section-title"),
          el("div", {
            class: "ca-run-list",
            children: state.runs.map(function (run) {
              return el("div", {
                class: "ca-run",
                children: [
                  status_node(run.status),
                  el("div", {
                    class: "ca-run__main",
                    children: [
                      text("strong", run.trigger_type || "Manual"),
                      text(
                        "span",
                        format_time(run.started_at || run.created_at),
                        "ca-run__time",
                      ),
                    ],
                  }),
                  text("span", run.current_node || "-", "ca-run__node"),
                  text("span", run.error || "", "ca-run__error"),
                ],
              });
            }),
          }),
        ],
      });
      if (state.runs.length === 0) {
        runs.appendChild(
          el("div", { class: "ca-empty", text: "暂无执行记录" }),
        );
      }
      detail.insertBefore(
        schedule_properties_view(schedule),
        detail.firstChild,
      );
      detail.appendChild(runs);
    }
    return detail;
  }

  function form_view() {
    var selected_flow_id =
      state.form_flow_id ||
      state.selected_flow_id ||
      (state.pipelines[0] && state.pipelines[0].id) ||
      "";
    var selected_flow = find_pipeline(selected_flow_id);
    var selected_start_node = effective_start_node(selected_flow, null);
    var name_default = selected_flow ? selected_flow.name + " 自动化" : "";
    var form = el("form", {
      class: "ca-form",
      attrs: { "data-trigger": "Cron" },
      props: {
        onsubmit: submit_schedule,
        onchange: function (event) {
          if (event.target.name === "trigger_type") {
            form.dataset.trigger = event.target.value;
          }
        },
      },
      children: [
        el("fieldset", {
          class: "ca-trigger-list ca-form__wide",
          children: [
            text("legend", "1. 选择触发节点", "ca-form__legend"),
            el("div", {
              class: "ca-trigger-options",
              children: [
                el("label", {
                  class: "ca-trigger-option",
                  children: [
                    el("input", {
                      props: { name: "trigger_type", type: "radio", value: "Cron", checked: true },
                    }),
                    el("span", { children: [text("strong", "定时触发"), text("small", "按 Cron 计划执行")] }),
                  ],
                }),
                el("label", {
                  class: "ca-trigger-option",
                  children: [
                    el("input", {
                      props: { name: "trigger_type", type: "radio", value: "Event" },
                    }),
                    el("span", { children: [text("strong", "事件触发"), text("small", "通过事件 Key 触发")] }),
                  ],
                }),
                el("label", {
                  class: "ca-trigger-option",
                  children: [
                    el("input", {
                      props: { name: "trigger_type", type: "radio", value: "Manual" },
                    }),
                    el("span", { children: [text("strong", "手动触发"), text("small", "在列表中手动执行")] }),
                  ],
                }),
              ],
            }),
            el("label", {
              class: "ca-event-only",
              children: [
                text("span", "事件 Key"),
                el("input", {
                  props: {
                    name: "event_key",
                    value: "",
                    placeholder: "例如：channels.feed.received",
                  },
                  attrs: { autocomplete: "off" },
                }),
              ],
            }),
            el("div", {
              class: "ca-manual-only ca-form-hint",
              text: "手动触发流程创建后默认停用 Cron，可在自动化列表点击“手动执行”。",
            }),
          ],
        }),
        el("div", {
          class: "ca-form__row",
          children: [
            el("label", {
              children: [
                text("span", "2. Pipeline"),
                el("select", {
                  props: {
                    name: "flow_id",
                    value: selected_flow_id,
                    onchange: function (event) {
                      var pipeline = find_pipeline(event.target.value);
                      var start_select = event.target.closest("form").start_node;
                      start_select.replaceChildren();
                      (pipeline && pipeline.nodes ? pipeline.nodes : []).forEach(function (node) {
                        start_select.appendChild(
                          el("option", {
                            text: node.name + "（" + node.id + "）",
                            props: { value: node.id },
                          }),
                        );
                      });
                      start_select.value = effective_start_node(pipeline, null);
                    },
                  },
                  children: state.pipelines.map(function (pipeline) {
                    return el("option", {
                      text: pipeline.name + "（" + pipeline.id + "）",
                      props: { value: pipeline.id },
                    });
                  }),
                }),
              ],
            }),
            el("label", {
              children: [
                text("span", "开始节点"),
                el("select", {
                  props: { name: "start_node", value: selected_start_node },
                  children: (selected_flow && selected_flow.nodes ? selected_flow.nodes : []).map(function (node) {
                    return el("option", {
                      text:
                        node.name +
                        "（" +
                        node.id +
                        (node.id === selected_start_node ? " · 默认开始" : "") +
                        "）",
                      props: { value: node.id },
                    });
                  }),
                }),
              ],
            }),
          ],
        }),
        el("div", {
          class: "ca-form__row",
          children: [
            el("label", {
              children: [
                text("span", "3. 流程名称"),
                el("input", {
                  props: {
                    name: "name",
                    value: name_default,
                    placeholder: "流程名称",
                  },
                  attrs: { required: "required" },
                }),
              ],
            }),
            el("label", {
              children: [
                text("span", "超时（秒）"),
                el("input", {
                  props: {
                    name: "timeout_sec",
                    value: "3600",
                    type: "number",
                    min: "1",
                  },
                  attrs: { required: "required" },
                }),
              ],
            }),
          ],
        }),
        el("label", {
          class: "ca-cron-only",
          children: [
            text("span", "Cron 表达式"),
            el("input", {
              props: {
                name: "cron_expr",
                value: "@daily",
                placeholder: "@daily 或 0 8 * * *",
              },
              attrs: { required: "required" },
            }),
          ],
        }),
        el("label", {
          class: "ca-form__wide",
          children: [
            text("span", "描述"),
            el("input", { props: { name: "description", placeholder: "可选" } }),
          ],
        }),
        el("label", {
          class: "ca-form__wide",
          children: [
            text("span", "初始数据 JSON"),
            el("textarea", {
              props: {
                name: "initial_data",
                value: "{}",
                rows: 4,
                spellcheck: false,
              },
            }),
          ],
        }),
        el("label", {
          class: "ca-checkbox ca-cron-only",
          children: [
            el("input", { props: { name: "enabled", type: "checkbox", checked: true } }),
            text("span", "创建后立即启用定时计划"),
          ],
        }),
        el("div", {
          class: "ca-form__footer",
          children: [
            button("取消", "ca-button--ghost", function () {
              state.form_open = false;
              render();
            }),
            el("button", {
              text: "创建流程",
              class: "ca-button ca-button--primary",
              attrs: { type: "submit" },
            }),
          ],
        }),
      ],
    });
    return form;
  }

  function panel_content() {
    var side = el("aside", {
      class: "ca-side",
      children: [
        el("div", {
          class: "ca-tabs",
          children: [
            button(
              "Pipeline",
              state.tab === "pipelines" ? "ca-tab is-active" : "ca-tab",
              function () {
                state.tab = "pipelines";
                render();
              },
            ),
            button(
              "自动化",
              state.tab === "schedules" ? "ca-tab is-active" : "ca-tab",
              function () {
                state.tab = "schedules";
                render();
              },
            ),
          ],
        }),
        state.tab === "pipelines" ? pipeline_list_view() : schedule_list_view(),
      ],
    });
    var main_children = [];
    if (state.error) {
      main_children.push(
        el("div", { class: "ca-alert ca-alert--error", text: state.error }),
      );
    }
    if (state.notice) {
      main_children.push(
        el("div", {
          class: "ca-alert ca-alert--success",
          text: state.notice,
        }),
      );
    }
    if (state.form_open) main_children.push(form_view());
    main_children.push(detail_view());
    return el("div", {
      class: "ca-panel",
      attrs: {
        role: "dialog",
        "aria-modal": "true",
        "aria-label": "Channels 自动化管理",
      },
      children: [
        el("header", {
          class: "ca-header",
          children: [
            el("div", {
              children: [
                text("h2", "Channels 自动化", "ca-header__title"),
                text(
                  "span",
                  "Pipeline 详情与定时执行管理",
                  "ca-header__subtitle",
                ),
              ],
            }),
            el("div", {
              class: "ca-header__actions",
              children: [
                button(
                  "刷新",
                  "ca-button--ghost",
                  function () {
                    load_data({ keep_selection: true, silent: true });
                  },
                  state.loading,
                ),
                button("创建流程", "ca-button--primary", function () {
                  state.form_open = true;
                  render();
                }),
                button("关闭", "ca-button--ghost", close),
              ],
            }),
          ],
        }),
        el("div", {
          class: "ca-body",
          children: [
            side,
            el("main", { class: "ca-main", children: main_children }),
          ],
        }),
      ],
    });
  }

  function render() {
    if (!panel_root || !panel_root.isConnected) return;
    panel_root.replaceChildren(panel_content());
  }

  function ensure_style() {
    if (style_element && style_element.isConnected) return;
    style_element = document.createElement("style");
    style_element.id = "channels-automation-style";
    style_element.textContent = `
.ca-overlay{position:fixed;inset:0;z-index:100000;display:flex;align-items:center;justify-content:center;background:rgba(15,23,42,.62);backdrop-filter:blur(10px);color:#172033;font-family:inherit;}
.ca-panel{display:flex;flex-direction:column;width:min(1180px,calc(100vw - 48px));height:min(780px,calc(100vh - 48px));overflow:hidden;border:1px solid rgba(148,163,184,.35);border-radius:18px;background:rgba(248,250,252,.98);box-shadow:0 24px 80px rgba(15,23,42,.35);}
.ca-header{display:flex;align-items:center;justify-content:space-between;gap:20px;padding:18px 22px;border-bottom:1px solid #dbe3ec;background:#fff;}
.ca-header__title{margin:0;font-size:19px;font-weight:750;letter-spacing:-.02em;}
.ca-header__subtitle{display:block;margin-top:3px;font-size:12px;color:#64748b;}
.ca-header__actions{display:grid;grid-template-columns:1fr 1fr 1fr;gap:5px;width:300px;}
.ca-body{display:grid;grid-template-columns:305px minmax(0,1fr);min-height:0;flex:1;}
.ca-side{display:flex;flex-direction:column;min-height:0;border-right:1px solid #dbe3ec;background:#f2f6fa;}
.ca-tabs{display:grid;grid-template-columns:1fr 1fr;gap:5px;padding:12px;border-bottom:1px solid #dbe3ec;}
.ca-tab{border-radius:8px;background:#e2e8f0;color:#475569;}
.ca-tab.is-active{background:#172033;color:#fff;}
.ca-pipeline-list,.ca-schedule-list{flex:1;overflow:auto;padding:12px;display:flex;flex-direction:column;gap:10px;}
.ca-pipeline-card,.ca-schedule-card{display:flex;flex-direction:column;gap:7px;padding:12px;border:1px solid transparent;border-radius:11px;background:#fff;box-shadow:0 2px 8px rgba(15,23,42,.05);cursor:pointer;transition:.15s;}
.ca-pipeline-card:hover,.ca-schedule-card:hover{border-color:#93c5fd;transform:translateY(-1px);}
.ca-pipeline-card.is-selected,.ca-schedule-card.is-selected{border-color:#2563eb;box-shadow:0 6px 18px rgba(37,99,235,.15);}
.ca-pipeline-card__title,.ca-schedule-card__title{display:block;font-size:14px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.ca-pipeline-card__id,.ca-pipeline-card__flow,.ca-schedule-card__flow{font-size:11px;color:#64748b;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.ca-schedule-card__header{display:flex;align-items:center;justify-content:space-between;gap:8px;}
.ca-meta{display:flex;align-items:center;justify-content:space-between;gap:8px;color:#64748b;font-size:11px;}
.ca-card-actions{display:flex;gap:6px;}
.ca-main{position:relative;min-width:0;min-height:0;overflow:auto;padding:18px 20px 28px;}
.ca-detail{display:flex;flex-direction:column;gap:15px;}
.ca-detail__header{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;}
.ca-detail__title{margin:0;font-size:19px;}
.ca-detail__subtitle{font-size:11px;color:#64748b;}
.ca-properties,.ca-schedule-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;padding:12px;border:1px solid #dbe3ec;border-radius:11px;background:#fff;}
.ca-schedule-summary{margin-bottom:15px;}
.ca-property,.ca-schedule-summary>div{min-width:0;}
.ca-property__label{display:block;margin-bottom:4px;font-size:11px;color:#64748b;}
.ca-property code,.ca-schedule-summary code{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;}
.ca-property--wide{grid-column:1/-1;}
.ca-property--wide code{white-space:pre-wrap!important;overflow:visible!important;text-overflow:initial!important;}
.ca-status{display:inline-flex;align-items:center;justify-content:center;min-width:58px;padding:2px 6px;border-radius:999px;font-size:10px;font-weight:700;letter-spacing:.02em;}
.ca-status--success{color:#166534;background:#dcfce7;}
.ca-status--danger{color:#991b1b;background:#fee2e2;}
.ca-status--warning{color:#92400e;background:#fef3c7;}
.ca-status--info{color:#334155;background:#e2e8f0;}
.ca-flow-scroll{overflow:auto;padding:6px;border:1px solid #dbe3ec;border-radius:12px;background:#fff;}
.ca-flow-canvas{position:relative;background-image:radial-gradient(rgba(148,163,184,.32) 1px,transparent 1px);background-size:22px 22px;}
.ca-flow-edges{position:absolute;inset:0;width:100%;height:100%;pointer-events:none;}
.ca-flow-edge{fill:none;stroke:rgba(15,23,42,.38);stroke-width:1.6;}
.ca-flow-edge.is-rule{stroke:#2563eb;stroke-dasharray:5 4;}
.ca-flow-edge-label{fill:#1d4ed8;font-size:10px;}
.ca-flow-node{position:absolute;width:180px;padding:9px 10px;border:1px solid #bfdbfe;border-radius:10px;background:#fff;box-shadow:0 6px 16px rgba(15,23,42,.09);}
.ca-flow-node.is-start{border-color:#16a34a;box-shadow:0 0 0 2px rgba(22,163,74,.12),0 6px 16px rgba(15,23,42,.09);}
.ca-flow-node__header{display:flex;align-items:center;justify-content:space-between;gap:6px;}
.ca-flow-node__header strong{font-size:12px;}
.ca-flow-node__type{flex:0 0 auto;padding:1px 4px;border-radius:4px;background:#eff6ff;color:#1d4ed8;font-size:9px;}
.ca-flow-node__id{display:block;margin-top:3px;color:#64748b;font-size:9px;}
.ca-flow-node__schema{margin-top:5px;color:#475569;font-size:9px;line-height:1.35;}
.ca-runs{display:flex;flex-direction:column;gap:9px;}
.ca-section-title{margin:0;font-size:14px;}
.ca-run-list{display:flex;flex-direction:column;gap:7px;}
.ca-run{display:grid;grid-template-columns:86px minmax(120px,.8fr) minmax(120px,.7fr) minmax(160px,1.5fr);align-items:center;gap:10px;padding:9px 10px;border:1px solid #e2e8f0;border-radius:9px;background:#fff;font-size:11px;}
.ca-run__main{display:flex;flex-direction:column;gap:2px;}
.ca-run__time,.ca-run__node{color:#64748b;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.ca-run__error{color:#b91c1c;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.ca-trigger-list{border:1px solid #bfdbfe;border-radius:10px;background:#fff;padding:12px;}
.ca-form__legend{padding:0 4px;font-size:11px;font-weight:750;color:#1d4ed8;}
.ca-trigger-options{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;}
.ca-trigger-option{display:flex!important;flex-direction:row!important;align-items:center;gap:8px!important;padding:9px;border:1px solid #dbe3ec;border-radius:9px;background:#f8fafc;cursor:pointer;}
.ca-trigger-option:has(input:checked){border-color:#2563eb;background:#eff6ff;}
.ca-trigger-option span{display:flex;flex-direction:column;gap:2px;}
.ca-trigger-option small{color:#64748b;font-size:9px;}
.ca-event-only,.ca-cron-only,.ca-manual-only{display:none;}
.ca-form[data-trigger="Cron"] .ca-cron-only{display:flex;}
.ca-form[data-trigger="Event"] .ca-event-only{display:flex;}
.ca-form[data-trigger="Manual"] .ca-manual-only{display:block;}
.ca-form-hint{padding:8px 10px;border-radius:8px;background:#fff7ed;color:#9a3412;font-size:10px;}
.ca-schedule-card__badges{display:flex;align-items:center;justify-content:flex-end;gap:4px;}
.ca-trigger{display:inline-flex;align-items:center;justify-content:center;min-width:46px;padding:2px 6px;border-radius:999px;background:#e2e8f0;color:#334155;font-size:9px;font-weight:750;}
.ca-trigger--cron{color:#1d4ed8;background:#dbeafe;}
.ca-trigger--event{color:#166534;background:#dcfce7;}
.ca-trigger--manual{color:#92400e;background:#fef3c7;}
.ca-form{display:flex;flex-direction:column;gap:12px;margin-bottom:18px;padding:14px;border:1px solid #bfdbfe;border-radius:13px;background:#eff6ff;}
.ca-form__row{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;}
.ca-form label{display:flex;flex-direction:column;gap:4px;font-size:11px;font-weight:650;color:#475569;}
.ca-form__wide{flex:1;}
.ca-form[data-trigger="Cron"] .ca-event-only,.ca-form[data-trigger="Cron"] .ca-manual-only,.ca-form[data-trigger="Event"] .ca-cron-only,.ca-form[data-trigger="Event"] .ca-manual-only,.ca-form[data-trigger="Manual"] .ca-cron-only,.ca-form[data-trigger="Manual"] .ca-event-only{display:none!important;}.ca-form input,.ca-form select,.ca-form textarea{width:100%;padding:7px 8px;border:1px solid #cbd5e1;border-radius:7px;background:#fff;color:#172033;font:inherit;font-size:12px;box-sizing:border-box;}
.ca-form textarea{resize:vertical;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;}
.ca-checkbox{flex-direction:row!important;align-items:center;gap:7px!important;}
.ca-checkbox input{width:15px;height:15px;}
.ca-form__footer{display:flex;justify-content:flex-end;gap:8px;}
.ca-button{border:0;border-radius:8px;padding:6px 10px;background:#e2e8f0;color:#334155;font-size:11px;font-weight:650;line-height:1.5;cursor:pointer;transition:.15s;}
.ca-button:hover{filter:brightness(.96);}
.ca-button:disabled{opacity:.55;cursor:not-allowed;}
.ca-button--primary{background:#2563eb;color:#fff;}
.ca-button--ghost{background:#fff;border:1px solid #cbd5e1;}
.ca-button--small{padding:4px 7px;font-size:10px;}
.ca-empty{padding:22px 14px;border-radius:10px;background:rgba(226,232,240,.55);color:#64748b;font-size:12px;text-align:center;}
.ca-empty--detail{padding:60px 20px;}
.ca-alert{margin-bottom:12px;padding:9px 11px;border-radius:9px;font-size:12px;}
.ca-alert--error{background:#fee2e2;color:#991b1b;}
.ca-alert--success{background:#dcfce7;color:#166534;}
@media (max-width:900px){.ca-body{grid-template-columns:1fr;grid-template-rows:220px minmax(0,1fr);}.ca-side{border-right:0;border-bottom:1px solid #dbe3ec;}.ca-properties,.ca-schedule-summary,.ca-form__row{grid-template-columns:1fr;}.ca-header{align-items:flex-start;}.ca-header__actions{width:100%;}.ca-panel{width:calc(100vw - 20px);height:calc(100vh - 20px);}}
`;
    document.head.appendChild(style_element);
  }

  function open() {
    ensure_style();
    if (!panel_root || !panel_root.isConnected) {
      panel_root = el("div", { class: "ca-overlay" });
      panel_root.addEventListener("click", function (event) {
        if (event.target === panel_root) close();
      });
      document.body.appendChild(panel_root);
    }
    render();
    load_data({ keep_selection: true });
  }

  function close() {
    if (panel_root && panel_root.isConnected) panel_root.remove();
    panel_root = null;
    state.form_open = false;
  }

  function register_menu() {
    if (
      typeof WXU === "undefined" ||
      typeof WXU.unshiftMenuItems !== "function"
    ) {
      return;
    }
    WXU.unshiftMenuItems({
      label: "自动化",
      onClick: function () {
        open();
      },
    });
  }

  document.addEventListener(
    "keydown",
    function (event) {
      if (event.key === "Escape" && panel_root) {
        event.stopPropagation();
        close();
      }
    },
    true,
  );

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", register_menu, { once: true });
  } else {
    register_menu();
  }

  return {
    open: open,
    close: close,
    refresh: function () {
      return load_data({ keep_selection: true, silent: true });
    },
  };
})();

if (typeof window !== "undefined") {
  window.ChannelsAutomation = ChannelsAutomation;
}

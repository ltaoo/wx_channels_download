import { request } from "@/biz/request.js";

function build_query(params) {
  const search = new URLSearchParams();
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      search.append(key, value);
    }
  });
  const query = search.toString();
  return query ? `?${query}` : "";
}

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
  const metadata = { type: "Cron", start_node: "", event_key: "" };
  if (!schedule) return metadata;
  try {
    const data = parse_initial_data(schedule.initial_data);
    const raw = data.__automation;
    if (raw && typeof raw === "object") {
      if (raw.type) metadata.type = String(raw.type);
      if (raw.start_node) metadata.start_node = String(raw.start_node);
      if (raw.event_key) metadata.event_key = String(raw.event_key);
    }
  } catch (err) {
    void err;
  }
  if (!["Cron", "Event", "Manual"].includes(metadata.type)) {
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
  const metadata = schedule_metadata(schedule);
  const nodes = flow && flow.nodes;
  if (
    metadata.start_node &&
    (!nodes || nodes.some((node) => node.id === metadata.start_node))
  ) {
    return metadata.start_node;
  }
  return flow ? flow.start_node_id : "";
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
    ManualNode: "人工确认",
    FuncNode: "函数",
    LoopNode: "循环",
    WorkflowNode: "子流程",
  };
  return names[type] || type;
}

function AutomationPageViewModel(props) {
  void props;
  const tab_ = ref("pipelines");
  const pipelines_ = refarr([]);
  const catalog_ = refarr([]);
  const schedules_ = refarr([]);
  const runs_ = refarr([]);
  const selected_flow_id_ = ref("");
  const selected_pipeline_ = refobj(null);
  const selected_schedule_id_ = ref("");
  const loading_ = ref(false);
  const error_ = ref("");
  const notice_ = ref("");
  const dirty_ = ref(false);
  const saving_ = ref(false);

  // editor working copy: [{id,type,name,config,next_ids,input_schema}]
  const edit_nodes_ = refarr([]);
  const edit_start_node_id_ = ref("");

  // create-pipeline dialog state
  const create_open_ = ref(false);
  const create_trigger_type_ = ref("Cron");
  const create_name_ = ref("");
  const create_description_ = ref("");
  const create_event_key_ = ref("");
  const create_cron_ = ref("@daily");
  const create_enabled_ = ref(true);
  const create_params_ = refarr([]);
  const create_submitting_ = ref(false);

  // add-node dialog state
  const add_open_ = ref(false);
  const add_type_ = ref("");
  const add_name_ = ref("");
  const add_config_ = ref("{}");
  const add_from_ = ref("");
  const add_submitting_ = ref(false);

  // create-schedule dialog state
  const schedule_open_ = ref(false);
  const schedule_trigger_type_ = ref("Cron");
  const schedule_event_key_ = ref("");
  const schedule_name_ = ref("");
  const schedule_cron_ = ref("@daily");
  const schedule_start_node_ = ref("");
  const schedule_flow_id_ = ref("");
  const schedule_submitting_ = ref(false);

  let request_sequence = 0;
  let node_sequence = 0;
  let param_sequence = 0;

  const selected_schedule_ = combine(
    { schedules: schedules_, id: selected_schedule_id_ },
    (state) => state.schedules.find((item) => item.id === state.id) || null,
  );

  const reqs = {
    pipeline: {
      create: new Timeless.kit.RequestCore(
        function createPipeline(body) {
          return request.post("/api/v1/automation/flows", body);
        },
        { client: props.client },
      ),
      update: new Timeless.kit.RequestCore(
        function updatePipeline(body) {
          return request.put(
            `/api/v1/automation/flows/${encodeURIComponent(body.id)}`,
            body,
          );
        },
        { client: props.client },
      ),
      delete: new Timeless.kit.RequestCore(
        function deletePipeline(body) {
          return request.delete(
            `/api/v1/automation/flows/${encodeURIComponent(body.flow_id)}`,
            body,
          );
        },
        { client: props.client },
      ),
      getGraph: new Timeless.kit.RequestCore(
        function fetchGraphOfPipeline(body) {
          return request.post(
            `/api/v1/automation/flows/graph${body.flow_id}`,
            body,
          );
        },
        { client: props.client },
      ),
      trigger: new Timeless.kit.RequestCore(
        function triggerPipeline(body) {
          return request.post(
            `/api/v1/automation/flows/${encodeURIComponent(body.flow_id)}/trigger`,
            body,
          );
        },
        { client: props.client },
      ),
    },
    schedule: {
      list: new Timeless.kit.RequestCore(
        function fetchScheduleList(body) {
          return request.post(`/api/v1/automation/schedules`, body);
        },
        { client: props.client },
      ),
      create: new Timeless.kit.RequestCore(
        function createSchedule(body) {
          return request.post(`/api/v1/automation/schedules`, body);
        },
        { client: props.client },
      ),
      detail: new Timeless.kit.RequestCore(
        function fetchScheduleDetail(body) {
          return request.post(`/api/v1/automation/schedules/${body.id}`, body);
        },
        { client: props.client },
      ),
      trigger: new Timeless.kit.RequestCore(
        function triggerSchedule(body) {
          return request.post(
            `/api/v1/automation/schedules/${body.id}/trigger`,
            body,
          );
        },
        { client: props.client },
      ),
      toggle: new Timeless.kit.RequestCore(
        function toggleSchedule(body) {
          return request.post(
            `/api/v1/automation/schedules/${body.id}/toggle`,
            body,
          );
        },
        { client: props.client },
      ),
    },
    running: {
      list: new Timeless.kit.RequestCore(
        function executeNode(body) {
          return request.post(`/api/v1/automation/runs`, body);
        },
        { client: props.client },
      ),
    },
  };

  const ui = {
    create_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    add_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    schedule_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    select_add_type$: new Timeless.vm.SelectCore({
      placeholder: "选择节点类型",
      position: "popper",
      options: [],
      onChange(value) {
        add_type_.as(String(value || ""));
        add_name_.as("");
        ui.input_add_name$.setValue(flow_label(add_type_.value), {
          silence: true,
        });
        ui.input_add_config$.setValue(default_config_json(add_type_.value), {
          silence: true,
        });
      },
    }),
    select_add_from$: new Timeless.vm.SelectCore({
      placeholder: "连接自（上一个节点）",
      position: "popper",
      options: [],
    }),
    select_schedule_flow$: new Timeless.vm.SelectCore({
      placeholder: "选择 Pipeline",
      position: "popper",
      options: [],
      onChange(value) {
        schedule_flow_id_.as(String(value || ""));
        reset_schedule_start_node(String(value || ""));
      },
    }),
    select_schedule_start$: new Timeless.vm.SelectCore({
      placeholder: "开始节点（默认）",
      position: "popper",
      options: [],
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
    input_create_event_key$: new Timeless.vm.InputCore({
      placeholder: "例如：channels.feed.received",
      onChange(value) {
        create_event_key_.as(value);
      },
    }),
    input_create_cron$: new Timeless.vm.InputCore({
      placeholder: "@daily 或 0 8 * * *",
      onChange(value) {
        create_cron_.as(value);
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
    input_schedule_name$: new Timeless.vm.InputCore({
      placeholder: "流程名称",
      onChange(value) {
        schedule_name_.as(value);
      },
    }),
    input_schedule_cron$: new Timeless.vm.InputCore({
      placeholder: "@daily 或 0 8 * * *",
      onChange(value) {
        schedule_cron_.as(value);
      },
    }),
    input_schedule_event_key$: new Timeless.vm.InputCore({
      placeholder: "例如：channels.feed.received",
      onChange(value) {
        schedule_event_key_.as(value);
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
      variant: "primary",
      onClick() {
        return open_create_dialog();
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
      variant: "primary",
      size: "sm",
      onClick() {
        return open_add_dialog();
      },
    }),
    btn_add_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: add_submitting_.value,
      onClick() {
        return submit_add_node();
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
    btn_delete_flow$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "sm",
      onClick() {
        return delete_flow();
      },
    }),
    btn_schedule_create$: new Timeless.vm.ButtonCore({
      variant: "primary",
      size: "sm",
      onClick() {
        return open_schedule_dialog();
      },
    }),
    btn_schedule_submit$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: schedule_submitting_.value,
      onClick() {
        return submit_schedule();
      },
    }),
    btn_schedule_cancel$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        ui.schedule_dialog$.hide();
      },
    }),
    checkbox_create_enabled$: new Timeless.vm.CheckboxCore({
      checked: true,
      onChange(value) {
        create_enabled_.as(value);
      },
    }),
    btn_schedule_toggle$: new Timeless.vm.ButtonInListCore({
      variant: "outline",
      size: "xs",
      onClick(schedule) {
        return schedule_action(schedule && schedule.id, "toggle");
      },
    }),
    btn_schedule_trigger$: new Timeless.vm.ButtonInListCore({
      variant: "outline",
      size: "xs",
      onClick(schedule) {
        return schedule_action(schedule && schedule.id, "trigger");
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

  loading_.subscribe({
    onChange(loading) {
      if (loading) ui.btn_refresh$.disable();
      else ui.btn_refresh$.enable();
    },
  });
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
  schedule_submitting_.subscribe({
    onChange(submitting) {
      if (submitting) ui.btn_schedule_submit$.disable();
      else ui.btn_schedule_submit$.enable();
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
      ui.select_schedule_start$.setOptions(options);
    },
  });

  function default_config_json(node_type) {
    const defaults = {
      ExprNode: { expression: "", output_key: "calc_out" },
      APICallNode: { url: "", method: "GET", keys: [] },
      GatewayNode: { gateway_type: "Exclusive", rules: [] },
    };
    const config = defaults[node_type] || {};
    return JSON.stringify(config, null, 2);
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

  async function load_pipelines() {
    const r = await reqs.graph.get.run({});
    if (r.error) {
      return;
    }
    const payload = r.data;
    // const payload = await request_json("GET", `${FLOWS_ENDPOINT}/graph`);
    return (payload && payload.flows) || [];
  }

  async function load_catalog() {
    // const payload = await request_json("GET", `${FLOWS_ENDPOINT}/node-catalog`);
    const r = await reqs.catelog.get.run({});
    if (r.error) {
      return;
    }
    const payload = r.data;
    return (payload && payload.nodes) || [];
  }

  async function load_schedules() {
    const r = await reqs.schedule.list.run({
      page: 1,
      page_size: 100,
    });
    if (r.error) {
      return [];
    }
    const payload = r.data;
    return (payload && payload.list) || [];
  }

  async function load_runs(schedule_id) {
    const r = await reqs.running.list.run({
      schedule_id,
      page: 1,
      page_size: 20,
    });
    if (r.error) {
      return [];
    }
    const payload = r.data;
    runs_.as((payload && payload.list) || [], { reset: true });
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
      if (!selected_flow_id_.value && pipelines.length > 0) {
        select_pipeline(pipelines[0].id, { silent: true });
      } else if (selected_flow_id_.value) {
        const fresh = pipelines.find(
          (item) => item.id === selected_flow_id_.value,
        );
        if (fresh) enter_edit(fresh);
        else {
          selected_flow_id_.as("");
          selected_pipeline_.as(null);
          edit_nodes_.as([], { reset: true });
        }
      }
      if (selected_schedule_id_.value) {
        const selected = schedules.find(
          (item) => item.id === selected_schedule_id_.value,
        );
        if (!selected) {
          selected_schedule_id_.as("");
          runs_.as([], { reset: true });
        } else {
          await load_runs(selected.id);
        }
      }
      if (sequence !== request_sequence) return;
      loading_.as(false);
    } catch (err) {
      if (sequence !== request_sequence) return;
      set_error(err);
    }
  }

  function enter_edit(flow) {
    selected_pipeline_.as(flow);
    edit_nodes_.as(
      (flow.nodes || []).map((node) => ({
        id: node.id,
        type: node.type,
        name: node.name || node.id,
        config: node.config || {},
        next_ids: (node.next_ids || []).slice(),
        input_schema: node.input_schema || [],
      })),
      { reset: true },
    );
    edit_start_node_id_.as(flow.start_node_id || "");
    dirty_.as(false);
  }

  async function select_pipeline(flow_id, options) {
    options = options || {};
    selected_flow_id_.as(flow_id);
    tab_.as("pipelines");
    selected_schedule_id_.as("");
    runs_.as([], { reset: true });
    const pipeline = find_pipeline(flow_id);
    if (pipeline) enter_edit(pipeline);
    else selected_pipeline_.as(null);
    if (!options.silent) {
      const r = await reqs.pipeline.getGraph.run({
        id: build_query({ flow_id }),
      });
      if (r.error) {
        return;
      }
      const payload = r.data;
      const flow = payload && payload.flows && payload.flows[0];
      if (flow) {
        enter_edit(flow);
      }
    }
  }

  async function select_schedule(id) {
    selected_schedule_id_.as(id);
    tab_.as("schedules");
    const sequence = ++request_sequence;
    loading_.as(true);
    try {
      const r = await reqs.schedule.detail.run({
        id: encodeURIComponent(id),
      });
      if (r.error) {
        return;
      }
      const schedule = r.data;
      if (sequence !== request_sequence) return;
      schedules_.as(
        (schedules_.value || []).map((item) =>
          item.id === id ? schedule : item,
        ),
        { reset: true },
      );
      const pipeline = find_pipeline(schedule.flow_id);
      selected_flow_id_.as(schedule.flow_id);
      if (pipeline) enter_edit(pipeline);
      await load_runs(id);
      loading_.as(false);
    } catch (err) {
      if (sequence !== request_sequence) return;
      set_error(err);
    }
  }

  // --- create pipeline ---
  function open_create_dialog() {
    create_trigger_type_.as("Cron");
    create_name_.as("");
    create_description_.as("");
    create_event_key_.as("");
    create_cron_.as("@daily");
    create_enabled_.as(true);
    create_params_.as([new_create_param_row()], {
      reset: true,
    });
    ui.input_create_name$.setValue("", { silence: true });
    ui.input_create_description$.setValue("", { silence: true });
    ui.input_create_event_key$.setValue("", { silence: true });
    ui.input_create_cron$.setValue("@daily", { silence: true });
    if (ui.checkbox_create_enabled$.value !== true) {
      ui.checkbox_create_enabled$.check();
    }
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

  function set_create_trigger_type(type) {
    create_trigger_type_.as(type);
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
      const trigger_type = create_trigger_type_.value;
      const event_key = String(create_event_key_.value || "").trim();
      const name = String(create_name_.value || "").trim();
      if (!name) throw new Error("请输入 Pipeline 名称");
      if (trigger_type === "Event" && !event_key) {
        throw new Error("请输入事件 Key");
      }
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
        trigger_type,
        event_key: trigger_type === "Event" ? event_key : "",
        context_schema,
      });
      if (r.error) {
        return;
      }
      const flow = r.data;
      ui.create_dialog$.hide();
      create_open_.as(false);
      notice_.as("Pipeline 创建成功，可继续添加后续节点");
      // Cron-triggered pipelines also need a schedule to fire.
      if (trigger_type === "Cron" && create_enabled_.value) {
        const r = await reqs.schedule.create.run({
          name: `${name} 定时`,
          cron_expr: String(create_cron_.value || "").trim() || "@daily",
          flow_id: flow.id,
          initial_data: {
            __automation: { type: "Cron", start_node: "start" },
          },
          enabled: true,
        });
        if (r.error) {
          set_error(r.error);
        }
      }
      await load_data({ silent: true });
      select_pipeline(flow.id);
    } catch (err) {
      set_error(err);
    }
    create_submitting_.as(false);
    return null;
  }

  // --- editor: add / remove nodes ---
  function open_add_dialog() {
    if (!selected_pipeline_.value) return null;
    const nodes = edit_nodes_.value || [];
    const last = nodes.length > 0 ? nodes[nodes.length - 1] : null;
    add_type_.as("");
    add_name_.as("");
    add_config_.as("{}");
    add_from_.as(last ? last.id : "");
    ui.select_add_type$.setValue("", { silence: true });
    ui.select_add_from$.setValue(last ? last.id : "", { silence: true });
    ui.input_add_name$.setValue("", { silence: true });
    ui.input_add_config$.setValue("{}", { silence: true });
    add_open_.as(true);
    ui.add_dialog$.show();
    return null;
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
      const node_id = `node-${Date.now()}-${++node_sequence}`;
      const nodes = (edit_nodes_.value || []).slice();
      const new_node = {
        id: node_id,
        type: node_type,
        name: String(add_name_.value || "").trim() || flow_label(node_type),
        config,
        next_ids: [],
        input_schema: [],
      };
      const from_id = String(add_from_.value || "").trim();
      if (from_id) {
        const from_index = nodes.findIndex((node) => node.id === from_id);
        if (from_index >= 0) {
          new_node.next_ids = nodes[from_index].next_ids.slice();
          nodes[from_index] = {
            ...nodes[from_index],
            next_ids: [node_id],
          };
        }
      }
      nodes.push(new_node);
      edit_nodes_.as(nodes, { reset: true });
      dirty_.as(true);
      ui.add_dialog$.hide();
      add_open_.as(false);
      notice_.as("节点已添加，记得保存 Pipeline");
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
    const nodes = (edit_nodes_.value || [])
      .filter((node) => node.id !== node_id)
      .map((node) => ({
        ...node,
        next_ids: (node.next_ids || []).filter((id) => id !== node_id),
      }));
    edit_nodes_.as(nodes, { reset: true });
    dirty_.as(true);
  }

  async function save_flow() {
    if (!selected_pipeline_.value || saving_.value) return null;
    saving_.as(true);
    error_.as("");
    try {
      const r = await reqs.pipeline.update.run({
        id: selected_flow_id_.value,
        start_node_id: edit_start_node_id_.value,
        nodes: (edit_nodes_.value || []).map((node) => ({
          id: node.id,
          type: node.type,
          name: node.name,
          config: node.config || {},
          input_schema: node.input_schema || [],
          next_ids: node.next_ids || [],
        })),
      });
      dirty_.as(false);
      notice_.as("Pipeline 已保存");
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
    saving_.as(false);
    return null;
  }

  async function delete_flow() {
    const flow_id = selected_flow_id_.value;
    if (!flow_id) return null;
    error_.as("");
    try {
      await reqs.pipeline.delete.run({ flow_id });
      selected_flow_id_.as("");
      selected_pipeline_.as(null);
      edit_nodes_.as([], { reset: true });
      notice_.as("Pipeline 已删除");
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
    return null;
  }

  async function trigger_flow() {
    const flow_id = selected_flow_id_.value;
    if (!flow_id) return null;
    error_.as("");
    try {
      const r = await reqs.pipeline.trigger.run({ flow_id });
      if (r.error) {
        return;
      }
      notice_.as(`已触发执行（${r.data.status || "RUNNING"}）`);
    } catch (err) {
      set_error(err);
    }
    return null;
  }

  // --- schedules ---
  function open_schedule_dialog(flow_id) {
    const target_flow_id = flow_id || selected_flow_id_.value;
    if (!target_flow_id) {
      error_.as("请先创建或选择一个 Pipeline");
      return null;
    }
    schedule_flow_id_.as(target_flow_id);
    schedule_trigger_type_.as("Cron");
    schedule_event_key_.as("");
    schedule_name_.as(`${pipeline_name(target_flow_id)} 自动化`);
    schedule_cron_.as("@daily");
    schedule_start_node_.as("");
    ui.select_schedule_flow$.setOptions(
      (pipelines_.value || []).map(
        (item) =>
          new Timeless.vm.SelectItemCore({
            label: `${item.name || item.id}（${item.id}）`,
            value: item.id,
          }),
      ),
    );
    ui.select_schedule_flow$.setValue(target_flow_id, { silence: true });
    reset_schedule_start_node(target_flow_id);
    ui.input_schedule_name$.setValue(schedule_name_.value, { silence: true });
    ui.input_schedule_cron$.setValue("@daily", { silence: true });
    ui.input_schedule_event_key$.setValue("", { silence: true });
    schedule_open_.as(true);
    ui.schedule_dialog$.show();
    return null;
  }

  function reset_schedule_start_node(flow_id) {
    const pipeline = find_pipeline(flow_id);
    const start_node_id = pipeline ? pipeline.start_node_id : "";
    schedule_start_node_.as(start_node_id);
    ui.select_schedule_start$.setValue(start_node_id, { silence: true });
  }

  function set_schedule_trigger_type(type) {
    schedule_trigger_type_.as(type);
  }

  async function submit_schedule() {
    if (schedule_submitting_.value) return null;
    schedule_submitting_.as(true);
    error_.as("");
    try {
      const trigger_type = schedule_trigger_type_.value;
      const event_key = String(schedule_event_key_.value || "").trim();
      const name = String(schedule_name_.value || "").trim();
      if (!name) throw new Error("请输入流程名称");
      if (trigger_type === "Event" && !event_key) {
        throw new Error("请输入事件 Key");
      }
      const initial_data = {};
      initial_data.__automation = {
        type: trigger_type,
        start_node: String(ui.select_schedule_start$.value || ""),
        event_key: trigger_type === "Event" ? event_key : "",
      };
      const r = await reqs.schedule.create.run({
        name,
        cron_expr:
          trigger_type === "Cron"
            ? String(schedule_cron_.value || "").trim() || "@daily"
            : "@daily",
        flow_id: String(
          ui.select_schedule_flow$.value || schedule_flow_id_.value || "",
        ),
        initial_data,
        enabled: trigger_type === "Cron",
      });
      if (r.error) {
        return;
      }
      const schedule = r.data;
      ui.schedule_dialog$.hide();
      schedule_open_.as(false);
      notice_.as("自动化流程创建成功");
      selected_schedule_id_.as(schedule.id);
      tab_.as("schedules");
      await load_data({ silent: true });
      await select_schedule(schedule.id);
    } catch (err) {
      set_error(err);
    }
    schedule_submitting_.as(false);
    return null;
  }

  async function schedule_action(id, action) {
    const schedule = (schedules_.value || []).find((item) => item.id === id);
    const metadata = schedule_metadata(schedule);
    error_.as("");
    try {
      if (action === "toggle" && metadata.type === "Cron") {
        await reqs.schedule.toggle.run({ id: encodeURIComponent(id) });
        notice_.as("自动化状态已更新");
      } else if (action === "trigger") {
        await reqs.schedule.trigger.run({
          id: encodeURIComponent(id),
          trigger_type: metadata.type === "Event" ? "Event" : "Manual",
          event_key: metadata.type === "Event" ? metadata.event_key : "",
        });
        notice_.as(
          metadata.type === "Event" ? "事件已触发" : "自动化流程已手动触发",
        );
      }
      await load_data({ silent: true });
    } catch (err) {
      set_error(err);
    }
  }

  const methods = {
    ready() {
      return load_data();
    },
    refresh() {
      return load_data({ silent: true });
    },
    switchTab(tab) {
      tab_.as(tab);
    },
    selectPipeline: select_pipeline,
    selectSchedule: select_schedule,
    toggleSchedule(id) {
      return schedule_action(id, "toggle");
    },
    triggerSchedule(id) {
      return schedule_action(id, "trigger");
    },
    openCreateDialog: open_create_dialog,
    setCreateTriggerType: set_create_trigger_type,
    addCreateParam: add_create_param,
    removeCreateParam: remove_create_param,
    updateCreateParam: update_create_param,
    openAddDialog: open_add_dialog,
    removeNode: remove_node,
    saveFlow: save_flow,
    deleteFlow: delete_flow,
    triggerFlow: trigger_flow,
    openScheduleDialog: open_schedule_dialog,
    setScheduleTriggerType: set_schedule_trigger_type,
    scheduleMetadata: schedule_metadata,
    triggerLabel: trigger_label,
    effectiveStartNode: effective_start_node,
    schemaText: schema_text,
    formatTime: format_time,
    pipelineName: pipeline_name,
    flowLabel: flow_label,
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
  };

  const state = {
    tab: tab_,
    pipelines: pipelines_,
    catalog: catalog_,
    schedules: schedules_,
    runs: runs_,
    selected_flow_id: selected_flow_id_,
    selected_pipeline: selected_pipeline_,
    selected_schedule_id: selected_schedule_id_,
    selected_schedule: selected_schedule_,
    edit_nodes: edit_nodes_,
    edit_start_node_id: edit_start_node_id_,
    loading: loading_,
    error: error_,
    notice: notice_,
    dirty: dirty_,
    saving: saving_,
    create_open: create_open_,
    create_trigger_type: create_trigger_type_,
    create_name: create_name_,
    create_description: create_description_,
    create_event_key: create_event_key_,
    create_cron: create_cron_,
    create_enabled: create_enabled_,
    create_params: create_params_,
    add_open: add_open_,
    add_type: add_type_,
    add_name: add_name_,
    add_config: add_config_,
    add_from: add_from_,
    schedule_open: schedule_open_,
    schedule_trigger_type: schedule_trigger_type_,
    schedule_event_key: schedule_event_key_,
    schedule_name: schedule_name_,
    schedule_cron: schedule_cron_,
  };

  return { state, ui, methods };
}

export { AutomationPageViewModel };

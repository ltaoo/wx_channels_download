import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationPipelineLink,
  AutomationPipelineTriggerBadges,
  AutomationRunPipelineDialog,
} from "./flow.components.js";

function AutomationPageView(props) {
  const vm$ = AutomationPageViewModel(props);
  return View(
    {
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [.group-content-page&]:h-full [.group-content-page&]:min-h-0 [.group-content-page&]:overflow-hidden [&_.group-automation-toolbar]:flex [&_.group-automation-toolbar]:items-center [&_.group-automation-toolbar]:justify-between [&_.group-automation-toolbar]:gap-4 max-900:[&_.group-automation-toolbar]:items-stretch max-900:[&_.group-automation-toolbar]:flex-col min-w-0 min-h-full font-sans",
      attributes: { n: "automation-page" },
      onMounted() {
        // 兼容旧链接 /automation?mode=edit|detail&id=xxx，重定向到独立页面
        const query = (props.view && props.view.query) || {};
        const mode = String(query.mode || "").toLowerCase();
        const flow_id = String(query.id || query.flow_id || "").trim();
        if (
          (mode === "edit" || mode === "detail") &&
          flow_id &&
          props.history &&
          typeof props.history.push === "function"
        ) {
          props.history.push(
            mode === "edit" ? "root.shell.flow_edit" : "root.shell.flow_detail",
            { id: flow_id },
          );
          return;
        }
        vm$.methods.ready();
      },
      beforeUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      AutomationListPage({ store: vm$ }),
      AutomationCreatePipelineDialog({ store: vm$ }),
      AutomationImportPipelineDialog({ store: vm$ }),
      AutomationDeletePipelineDialog({ store: vm$ }),
      AutomationRunPipelineDialog({ store: vm$ }),
      AutomationConfigDialog({ store: vm$ }),
    ],
  );
}

function AutomationListPage(props) {
  const vm$ = props.store;
  return View({ class: "automation-list-page flex flex-col flex-[1_1_auto] min-h-0 [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto]" }, [
    View({ class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0] " }, [
      AutomationPageToolbar({ store: vm$ }),
    ]),
    AutomationPipelineList({ store: vm$ }),
  ]);
}

function AutomationPageToolbar(props) {
  const vm$ = props.store;
  return View({
    type: "form",
    class: "group-content-toolbar flex items-center justify-between gap-3 max-760:flex-wrap group-automation-toolbar",
    attributes: { role: "search" },
    onSubmit(event) {
      event.preventDefault();
      vm$.methods.search();
    },
  }, [
    View({
      class: "min-w-0 flex-[1_1_0] max-760:flex-[1_1_100%]",
      attributes: { n: "automation-search-field" },
    }, [
      Input({
        store: vm$.ui.input_keyword$,
        rootAttributes: { n: "automation-search-control" },
        prefix: Timeless.Icon({
          name: "search",
          size: 16,
          attributes: { n: "automation-search-icon" },
        }),
        attributes: {
          n: "automation-search-input",
          name: "keyword",
          type: "text",
          autocomplete: "off",
          "aria-label": "搜索 Pipeline 名称",
        },
      }),
    ]),
    View({ class: "flex-[0_0_auto] flex-wrap max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2" }, [
      Button(
        {
          store: vm$.ui.btn_search$,
          class: "dm-button--toolbar",
          attributes: { n: "automation-search-action", type: "submit" },
          prefix: Timeless.Icon({ name: "search", size: 16 }),
          onClick(event) {
            event.preventDefault();
            vm$.methods.search();
          },
        },
        ["搜索"],
      ),
      Button(
        {
          store: vm$.ui.btn_reset$,
          class: "dm-button--toolbar",
          attributes: { n: "automation-reset-action", type: "button" },
          prefix: Timeless.Icon({ name: "rotate-ccw", size: 16 }),
        },
        ["重置"],
      ),
      Button(
        {
          store: vm$.ui.btn_import_pipeline$,
          attributes: { n: "automation-import-pipeline", type: "button" },
        },
        ["导入 JSON"],
      ),
      Button(
        {
          store: vm$.ui.btn_create_pipeline$,
          attributes: { n: "automation-create-pipeline", type: "button" },
        },
        ["创建自动化"],
      ),
    ]),
  ]);
}

function AutomationPipelineList(props) {
  const vm$ = props.store;
  return Table({
    name: "automation-pipeline-table",
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4  automation-index",
    containerAttributes: { n: "automation-pipeline-table-container" },
    panelAttributes: { n: "automation-pipeline-table-panel" },
    rowKey: "id",
    columns: [
      {
        name: "name",
        title: "名称",
        width: "minmax(240px, 2fr)",
        cellClass: "min-w-0 grid gap-0-5 justify-items-start",
        render(pipeline) {
          return [
            View(
              {
                class: "min-w-0 max-w-full overflow-hidden text-color-text-primary text-base font-semibold text-ellipsis whitespace-nowrap",
                attributes: { title: pipeline.name || pipeline.id },
              },
              [pipeline.name || pipeline.id],
            ),
            View(
              {
                class: "min-w-0 max-w-full overflow-hidden text-color-text-secondary font-mono text-xs text-ellipsis whitespace-nowrap",
                attributes: { title: pipeline.id },
              },
              [pipeline.id],
            ),
            pipeline.description
              ? View(
                  {
                    class: "min-w-0 max-w-full overflow-hidden text-color-text-secondary text-sm text-ellipsis whitespace-nowrap",
                    attributes: { title: pipeline.description },
                  },
                  [pipeline.description],
                )
              : null,
          ].filter(Boolean);
        },
      },
      {
        name: "trigger",
        title: "触发方式",
        width: 210,
        cellClass: "flex items-center gap-1 flex-wrap",
        render(pipeline) {
          return [
            AutomationPipelineTriggerBadges({ store: vm$, pipeline }),
          ].filter(Boolean);
        },
      },
      {
        name: "config",
        title: "配置",
        width: "minmax(240px, 1fr)",
        cellClass: "min-w-0 flex flex-col items-start gap-3",
        render(pipeline) {
          const groups = vm$.methods.pipelineConfigGroups(pipeline);
          if (!groups.length) return [];
          const configured_groups = groups.map((group) => ({
            ...group,
            fields: group.fields.filter((field) => field.filled),
          })).filter((group) => group.fields.length);
          return [
            configured_groups.length ? View({ class: "w-full min-w-0 grid gap-2", attributes: { n: `automation-pipeline-config-values-${pipeline.id}` } },
              configured_groups.map((group) => View({ class: "min-w-0 grid gap-0-5" }, [
                View({ class: "overflow-hidden text-color-text-secondary text-xs font-semibold text-ellipsis whitespace-nowrap", attributes: { title: `${group.name} (${group.id})` } }, [group.name]),
                ...group.fields.map((field) => {
                  const text = `${field.label}: ${field.text}`;
                  return View({ class: "overflow-hidden text-color-text-primary text-sm text-ellipsis whitespace-nowrap", attributes: { title: text } }, [text]);
                }),
              ]))) : null,
            Button({ store: vm$.ui.btn_pipeline_config$.bind(pipeline), class: "flex-none", attributes: { n: `automation-pipeline-config-${pipeline.id}`, type: "button", "aria-label": `配置 ${pipeline.name || pipeline.id}` } }, ["配置"]),
          ].filter(Boolean);
        },
      },
      {
        name: "updated",
        title: "最后更新",
        width: 170,
        cellClass: "overflow-hidden text-color-text-primary font-mono text-sm font-variant-numeric-tabular-nums text-ellipsis whitespace-nowrap",
        render(pipeline) {
          return [
            vm$.methods.formatTime(pipeline.updated_at || pipeline.created_at),
          ];
        },
      },
      {
        name: "actions",
        title: "操作",
        width: 230,
        cellClass: "flex flex-wrap items-center justify-end gap-1-5 max-900:justify-start",
        render(pipeline) {
          return [
            vm$.methods.pipelineTriggerTypes(pipeline).includes("Manual")
              ? Button(
                  {
                    store: vm$.ui.btn_pipeline_trigger$.bind(pipeline),
                    attributes: {
                      n: `automation-pipeline-trigger-${pipeline.id}`,
                      type: "button",
                      "aria-label": `触发 ${pipeline.name || pipeline.id}`,
                    },
                  },
                  ["触发"],
                )
              : null,
            AutomationPipelineLink({
              flowId: pipeline.id,
              mode: "detail",
              label: "详情",
            }),
            AutomationPipelineLink({
              flowId: pipeline.id,
              mode: "edit",
              label: "编辑",
            }),
            Button(
              {
                store: vm$.ui.btn_pipeline_delete$.bind(pipeline),
                attributes: {
                  n: `automation-pipeline-delete-${pipeline.id}`,
                  type: "button",
                  title: "删除 Pipeline",
                },
              },
              ["删除"],
            ),
          ].filter(Boolean);
        },
      },
    ],
    rows: vm$.state.paged_pipelines,
    pagination: {
      class: " dm-px-4",
      summary: vm$.state.range_text,
      page: vm$.state.page,
      pageCount: vm$.state.page_count,
      pageSize: vm$.state.page_size,
      loading: vm$.state.loading,
      onChange(page) {
        return vm$.methods.changePage(page);
      },
    },
    status: computed(vm$.state.filtered_pipelines, (pipelines) =>
      pipelines.length === 0 ? "empty" : "normal",
    ),
    loading: vm$.state.loading,
    emptyTitle: combine(
      { loading: vm$.state.loading, keyword: vm$.state.search_keyword },
      ({ loading, keyword }) =>
        loading ? "正在加载 Pipeline…" : keyword ? "没有匹配的 Pipeline" : "暂无 Pipeline",
    ),
    emptyDescription: combine(
      { loading: vm$.state.loading, keyword: vm$.state.search_keyword },
      ({ loading, keyword }) =>
        loading
          ? "正在获取最新的流程配置。"
          : keyword
            ? "请尝试其他名称或重置搜索条件。"
            : "点击右上角「创建自动化」开始搭建流程。",
    ),
  });
}

function AutomationConfigDialog(props) {
  const vm$ = props.store;
  return Dialog({ store: vm$.ui.config_dialog$, class: "automation-form-dialog", attributes: { n: "automation-config-dialog" } }, [
    DialogHeader({}, [
      DialogTitle({}, ["自动化配置"]),
      DialogDescription({}, [computed(vm$.state.config_pipeline, (pipeline) => `${pipeline ? pipeline.name || pipeline.id : ""} · 按节点填写，所有触发方式共用。`)]),
    ]),
    DialogBody({}, [
      Show({ when: vm$.state.config_loading, ok: () => View({ class: "text-color-text-secondary" }, ["正在加载配置…"]) }),
      Show({ when: computed(vm$.state.config_error, Boolean), ok: () => View({ class: "text-color-danger text-sm", attributes: { role: "alert" } }, [vm$.state.config_error]) }),
      Show({ when: combine({ groups: vm$.state.config_groups, loading: vm$.state.config_loading, error: vm$.state.config_error }, ({ groups, loading, error }) => !loading && !error && !groups.length), ok: () => View({ class: "text-color-text-secondary text-sm" }, ["此自动化暂无所需配置。在编辑页选择节点，声明所需配置后保存。"]) }),
      View({ class: "flex flex-col gap-4" }, [For({ each: vm$.state.config_groups, key: "id", render(group_) {
        const group = group_ && group_.value !== undefined ? group_.value : group_;
        return View({ class: "flex flex-col gap-3" }, [
          View({ class: "text-color-text-primary font-semibold word-break-break-all" }, [`${group.name} (${group.id})`]),
          ...group.fields.map((field) => {
            const id = `automation-config-${group.id}-${field.key}`;
            const attributes = { id, n: id, "aria-label": field.label || field.key, "aria-required": String(Boolean(field.required)), "aria-describedby": field.description ? `${id}-help` : undefined };
            const onChange = (event) => field.input$.setValue(String(event.target.value));
            const input = field.type === "boolean"
              ? Checkbox({ store: field.input$, attributes }, [field.label || field.key])
              : field.options && field.options.length
                ? Select({ store: field.input$, attributes })
              : field.multiline || field.type === "array" || field.type === "object"
                ? Textarea({ store: field.input$, onChange, class: "font-mono text-xs", attributes: { ...attributes, rows: "4", spellcheck: "false" } })
                : Input({ store: field.input$, onChange, attributes: { ...attributes, type: field.secret ? "password" : field.type === "number" || field.type === "integer" ? "number" : "text", step: field.type === "number" ? "any" : undefined, autocomplete: field.secret ? "new-password" : "off" } });
            return View({ class: "min-w-0 flex flex-col gap-1" }, [
              field.type !== "boolean" ? Label({ class: "text-color-text-secondary text-xs font-semibold", attributes: { for: id } }, [`${field.label || field.key}${field.required ? " *" : ""}`]) : null,
              input,
              field.description ? View({ class: "text-color-text-secondary text-caption", attributes: { id: `${id}-help` } }, [field.description]) : null,
            ].filter(Boolean));
          }),
        ]);
      } })]),
    ]),
    DialogFooter({}, [
      Button({ store: vm$.ui.btn_config_cancel$, attributes: { type: "button" } }, ["取消"]),
      Button({ store: vm$.ui.btn_config_save$, attributes: { n: "automation-config-save", type: "button" } }, [computed(vm$.state.config_saving, (saving) => saving ? "正在保存…" : "保存配置")]),
    ]),
  ]);
}

function AutomationCreatePipelineDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.create_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-create-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["创建自动化"]),
        DialogDescription({}, [
          "为开始节点声明接收参数；创建后在编辑器中添加节点，并配置定时或事件触发器。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "flex flex-col gap-3" }, [
          View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-3 [&_.dm-button]:align-self-flex-end max-900:grid-cols-1fr" }, [
            View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
              View({ class: "text-color-text-secondary text-xs font-semibold" }, ["Pipeline 名称"]),
              Input({
                store: vm$.ui.input_create_name$,
                attributes: { n: "automation-create-name" },
              }),
            ]),
            View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
              View({ class: "text-color-text-secondary text-xs font-semibold" }, ["描述"]),
              Input({
                store: vm$.ui.input_create_description$,
                attributes: { n: "automation-create-description" },
              }),
            ]),
          ]),
          View({ class: "text-color-primary text-xs font-bold" }, ["开始节点接收的参数"]),
          For({
            key: "uid",
            each: vm$.state.create_params,
            render(param_) {
              const param =
                param_ && param_.value !== undefined ? param_.value : param_;
              return View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-3 [&_.dm-button]:align-self-flex-end max-900:grid-cols-1fr" }, [
                View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
                  View({ class: "text-color-text-secondary text-xs font-semibold" }, ["参数名"]),
                  Input({
                    store: param.input_key$,
                    attributes: {
                      n: `automation-create-param-key-${param.uid}`,
                      placeholder: "例如 url",
                    },
                  }),
                ]),
                View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
                  View({ class: "text-color-text-secondary text-xs font-semibold" }, ["类型"]),
                  Select({
                    store: param.select_type$,
                    attributes: {
                      n: `automation-create-param-type-${param.uid}`,
                    },
                  }),
                ]),
                Button(
                  {
                    store: vm$.ui.btn_param_remove$.bind(param),
                    attributes: {
                      n: `automation-create-param-remove-${param.uid}`,
                      type: "button",
                      title: "移除参数",
                    },
                  },
                  ["移除"],
                ),
              ]);
            },
          }),
          Button(
            {
              store: vm$.ui.btn_param_add$,
              attributes: {
                n: "automation-create-param-add",
                type: "button",
              },
            },
            ["+ 添加参数"],
          ),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_create_cancel$,
            attributes: { n: "automation-create-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_create_submit$,
            attributes: { n: "automation-create-submit", type: "button" },
          },
          ["创建自动化"],
        ),
      ]),
    ],
  );
}

function AutomationImportPipelineDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.import_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-import-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["导入 Pipeline"]),
        DialogDescription({}, [
          "粘贴流程定义 JSON（含 name、start_node、context_schema、nodes），导入后自动创建 Pipeline。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "flex flex-col gap-3" }, [
          View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
            Label({ class: "text-color-text-secondary text-xs font-semibold" }, ["流程定义 JSON"]),
            Textarea({
              store: vm$.ui.input_import_json$,
              class: "min-h-220px font-mono text-xs",
              attributes: {
                n: "automation-import-json",
                rows: "16",
                spellcheck: "false",
                placeholder:
                  '{"name":"...","start_node":"start","context_schema":[],"nodes":{...}}',
                "aria-label": "流程定义 JSON",
              },
            }),
          ]),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_import_cancel$,
            attributes: { n: "automation-import-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_import_submit$,
            attributes: { n: "automation-import-submit", type: "button" },
          },
          ["导入"],
        ),
      ]),
    ],
  );
}

function AutomationDeletePipelineDialog(props) {
  const vm$ = props.store;
  return Confirm({
    store: vm$.ui.delete_dialog$,
    class: "dm-dialog--sm",
    name: "automation-delete-pipeline",
    title: "删除 Pipeline",
    description: combine(
      { flow_id: vm$.state.delete_flow_id, pipelines: vm$.state.pipelines },
      ({ flow_id, pipelines }) => {
        const flow = (pipelines || []).find((item) => item.id === flow_id);
        return flow
          ? `确定删除「${flow.name || flow.id}」？删除后不可恢复。`
          : "确定删除当前 Pipeline？删除后不可恢复。";
      },
    ),
    cancelText: "取消",
    okText: "删除 Pipeline",
  });
}

export default AutomationPageView;

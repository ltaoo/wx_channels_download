import { AutomationPageViewModel } from "./automation.model.js";
import { DrawerHeader, DrawerTitle, DrawerDescription, DrawerBody } from "@/dmui.js";
import {
  AutomationPipelineLink,
  AutomationPipelineTriggerBadges,
  AutomationRunPipelineDialog,
  AutomationImportPipelineDialog,
} from "./automation.components.js";

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
      AutomationDefinitionsDrawer({ store: vm$ }),
      AutomationCreatePipelineDialog({ store: vm$ }),
      AutomationImportPipelineDialog({ store: vm$ }),
      AutomationDeletePipelineDialog({ store: vm$ }),
      AutomationRunPipelineDialog({ store: vm$ }),
      AutomationConfigDialog({ store: vm$ }),
      AutomationPasteConfigDialog({ store: vm$ }),
    ],
  );
}

function AutomationListPage(props) {
  const vm$ = props.store;
  return View({ class: "automation-list-page flex flex-col flex-[1_1_auto] min-h-0 [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto]", attributes: { n: "automation-instance-list-page" } }, [
    View({ class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0]", attributes: { n: "automation-instance-toolbar-wrap" } }, [
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
    attributes: { n: "automation-instance-toolbar", role: "search" },
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
          "aria-label": "搜索配置实例或所属流程",
        },
      }),
    ]),
    View({ class: "flex-[0_0_auto] flex-wrap max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2", attributes: { n: "automation-instance-toolbar-actions" } }, [
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
          store: vm$.ui.btn_definitions$,
          attributes: { n: "automation-open-definitions", type: "button" },
        },
        ["自动化流程"],
      ),
      Button(
        {
          store: vm$.ui.btn_create_instance$,
          attributes: { n: "automation-create-instance", type: "button" },
        },
        ["新增配置实例"],
      ),
    ]),
  ]);
}

function AutomationSecretPopover(props) {
  const display = props.store;
  return Popover({ store: display.popover$, triggerClass: "flex-none", class: "grid gap-2", style: { width: "360px", "max-width": "calc(100vw - 32px)" }, attributes: {
    n: `${props.name}-secret-popover`, id: `${props.name}-secret-popover`, role: "dialog", "aria-label": props.label,
  }, content: [
    View({ class: "overflow-auto overflow-wrap-anywhere whitespace-pre-wrap font-mono text-sm", style: { "max-height": "240px" }, attributes: { n: `${props.name}-secret-content` } }, [display.value]),
    Button({ store: display.copy$, attributes: { n: `${props.name}-copy-secret`, type: "button", "aria-label": `复制${props.label}` } }, ["复制"]),
  ] }, [TextButton({ store: display.trigger$, class: "flex-none", onClick(event) {
    // Popover handles pointerdown; keyboard clicks only emit click.
    if (event.detail === 0) display.popover$.toggle();
  }, attributes: {
    n: `${props.name}-toggle-secret`, type: "button",
    "aria-label": `查看${props.label}`, "aria-haspopup": "dialog", "aria-controls": `${props.name}-secret-popover`,
  } }, ["查看"])]);
}

function AutomationConfigValues(props) {
  const groups = props.store.methods.pipelineConfigGroups(props.pipeline).map((group) => ({
    ...group, fields: group.fields.filter((field) => field.filled),
  })).filter((group) => group.fields.length);
  if (!groups.length) return null;
  const name = `automation-pipeline-config-values-${props.pipeline.id}`;
  return View({ class: "w-full min-w-0 grid gap-2", attributes: { n: name } }, groups.map((group) =>
    View({ class: "min-w-0 grid gap-0-5", attributes: { n: `${name}-${group.id}` } }, [
      View({ class: "overflow-hidden text-color-text-secondary text-xs font-semibold text-ellipsis whitespace-nowrap", attributes: { n: `${name}-${group.id}-heading`, title: `${group.name} (${group.id})` } }, [group.name]),
      TableFields({ name: `${name}-${group.id}-fields`, fields: group.fields.map((field) => {
        const field_name = `${name}-${group.id}-${field.key}`;
        return { name: field_name, label: field.label, value: [
          View({ class: "min-w-0 overflow-hidden text-color-text-primary text-sm text-ellipsis whitespace-nowrap", attributes: { n: `${field_name}-text`, title: field.text } }, [field.text]),
          field.secret_display ? AutomationSecretPopover({ store: field.secret_display, name: field_name, label: field.label }) : null,
        ].filter(Boolean) };
      }) }),
    ]),
  ));
}

function AutomationInstanceNameEditor(props) {
  const editor = props.store;
  const name = props.name;
  return Popover({
    store: editor.popover$,
    triggerClass: "flex-none",
    class: "grid gap-3",
    style: { width: "280px", "max-width": "calc(100vw - 32px)" },
    attributes: { n: `${name}-popover`, id: `${name}-popover`, role: "dialog", "aria-label": "修改实例名称" },
    onUnmounted: editor.destroy,
    content: [
      Label({ attributes: { n: `${name}-label`, for: `${name}-input` } }, ["实例名称"]),
      Input({ store: editor.input$, onEnter() { return editor.save$.click(); }, attributes: { n: `${name}-input`, id: `${name}-input`, "aria-label": "实例名称", autocomplete: "off" } }),
      View({ class: "text-color-danger text-sm", style: computed(editor.error, (error) => ({ display: error ? "block" : "none" })), attributes: { n: `${name}-error`, role: "alert" } }, [editor.error]),
      View({ class: "flex items-center justify-end gap-2", attributes: { n: `${name}-actions` } }, [
        Button({ store: editor.cancel$, attributes: { n: `${name}-cancel`, type: "button" } }, ["取消"]),
        Button({ store: editor.save$, attributes: { n: `${name}-save`, type: "button" } }, ["确定"]),
      ]),
    ],
  }, [TextButton({ store: editor.trigger$, attributes: {
    n: `${name}-trigger`, type: "button", title: "修改实例名称", "aria-label": "修改实例名称",
    "aria-haspopup": "dialog", "aria-controls": `${name}-popover`,
  }, onClick(event) {
    if (event.detail === 0) editor.popover$.toggle();
  } }, [Timeless.Icon({ name: "pen-line", size: 14, attributes: { n: `${name}-icon`, "aria-hidden": "true" } })])]);
}

function AutomationPipelineList(props) {
  const vm$ = props.store;
  return Table({
    name: "automation-instance-table",
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4  automation-index",
    containerAttributes: { n: "automation-pipeline-table-container" },
    panelAttributes: { n: "automation-pipeline-table-panel" },
    rowKey: (pipeline) => `${pipeline.id}:${pipeline.name || ""}:${pipeline.updated_at || pipeline.created_at || 0}:${pipeline.active_run_id || ""}:${vm$.methods.pipelineDefinitionLabel(pipeline)}`,
    columns: [
      {
        name: "name",
        title: "配置实例",
        width: 240,
        cellClass: "min-w-0",
        render(pipeline) {
          return [
            View({ class: "min-w-0 max-w-full grid gap-0-5 justify-items-start", attributes: { n: `automation-pipeline-titles-${pipeline.id}` } }, [
              View({ class: "text-color-text-secondary text-xs", attributes: { n: `automation-pipeline-definition-${pipeline.id}` } }, [vm$.methods.pipelineInstanceDescription(pipeline)]),
              View({ class: "min-w-0 max-w-full flex items-baseline gap-1", attributes: { n: `automation-pipeline-name-row-${pipeline.id}` } }, [
                View(
                  {
                    class: "min-w-0 max-w-full text-color-text-primary text-base font-semibold overflow-wrap-anywhere",
                    attributes: { n: `automation-pipeline-name-${pipeline.id}`, title: pipeline.name || pipeline.id },
                  },
                  [pipeline.name || pipeline.id],
                ),
                AutomationInstanceNameEditor({ store: vm$.methods.pipelineNameEditor(pipeline), name: `automation-pipeline-rename-${pipeline.id}` }),
              ]),
              pipeline.description
                ? View(
                    {
                      class: "min-w-0 max-w-full overflow-hidden text-color-text-secondary text-sm text-ellipsis whitespace-nowrap",
                      attributes: { n: `automation-pipeline-description-${pipeline.id}`, title: pipeline.description },
                    },
                    [pipeline.description],
                  )
                : null,
            ].filter(Boolean)),
          ];
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
          return [
            AutomationConfigValues({ store: vm$, pipeline }),
            View({ class: "dm-button-group", attributes: { n: `automation-pipeline-config-actions-${pipeline.id}`, role: "group", "aria-label": `配置 ${pipeline.name || pipeline.id}` } }, [
              Button({ store: vm$.ui.btn_pipeline_config$.bind(pipeline), class: "flex-none", attributes: { n: `automation-pipeline-config-${pipeline.id}`, type: "button", "aria-label": `配置 ${pipeline.name || pipeline.id}` } }, ["配置"]),
              DropdownMenu({ store: vm$.methods.pipelineConfigMenu(pipeline), attributes: { n: `automation-pipeline-config-menu-${pipeline.id}` } }, [
                Button({ store: vm$.ui.btn_pipeline_config_menu$.bind(pipeline), class: "dm-button--icon", attributes: { n: `automation-pipeline-config-menu-trigger-${pipeline.id}`, type: "button", "aria-label": `${pipeline.name || pipeline.id} 复制或粘贴配置 JSON` } }, [
                  Timeless.Icon({ name: "chevron-down", size: 14, attributes: { n: `automation-pipeline-config-menu-icon-${pipeline.id}`, "aria-hidden": "true" } }),
                ]),
              ]),
            ]),
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
        cellClass: "flex flex-nowrap items-center justify-end gap-1-5 max-900:justify-start",
        render(pipeline) {
          return [
            TextButton({ store: vm$.methods.pipelineActionButton("trigger", pipeline), attributes: { n: `automation-instance-run-${pipeline.id}`, type: "button", "aria-label": `执行配置实例 ${pipeline.name || pipeline.id}` } }, ["执行"]),
            AutomationPipelineLink({
              flowId: pipeline.id,
              mode: "detail",
              label: "详情",
            }),
            DropdownMenu(
              {
                store: vm$.methods.pipelineActionsMenu(pipeline),
                attributes: { n: `automation-pipeline-more-menu-${pipeline.id}` },
              },
              [TextButton({
                store: vm$.methods.pipelineActionButton("more", pipeline),
                suffix: Timeless.Icon({ name: "chevron-down", size: 14, attributes: { n: `automation-pipeline-more-icon-${pipeline.id}`, "aria-hidden": "true" } }),
                attributes: { n: `automation-pipeline-more-${pipeline.id}`, type: "button", "aria-label": `${pipeline.name || pipeline.id} 更多操作` },
              }, ["更多"])],
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
        loading ? "正在加载配置实例…" : keyword ? "没有匹配的配置实例" : "暂无配置实例",
    ),
    emptyDescription: combine(
      { loading: vm$.state.loading, keyword: vm$.state.search_keyword },
      ({ loading, keyword }) =>
        loading
          ? "正在获取配置实例。"
          : keyword
            ? "请尝试其他名称或重置搜索条件。"
            : "点击「新增配置实例」选择自动化流程；也可在流程抽屉中创建或导入流程。",
    ),
  });
}

function AutomationDefinitionsDrawer(props) {
  const vm$ = props.store;
  return Drawer({
    store: vm$.ui.definitions_drawer$,
    class: "dm-drawer--wide",
    attributes: { n: "automation-definitions-drawer", "aria-labelledby": "automation-definitions-title" },
  }, () => [
    DrawerHeader({ attributes: { n: "automation-definitions-header" } }, [
      DrawerTitle({ attributes: { n: "automation-definitions-title", id: "automation-definitions-title" } }, ["自动化流程"]),
      DrawerDescription({ attributes: { n: "automation-definitions-description" } }, ["维护共享流程与默认配置，选择流程新增配置实例。"]),
    ]),
    DrawerBody({ class: "flex flex-col gap-3 overflow-hidden", style: { padding: "var(--dm-space-4) 0 0" }, attributes: { n: "automation-definitions-body" } }, [
      View({ class: "flex flex-wrap items-center gap-2 px-4", attributes: { n: "automation-definitions-toolbar" } }, [
        Input({ store: vm$.ui.input_definition_keyword$, rootClass: "flex-1 min-w-0", attributes: { n: "automation-definitions-search", "aria-label": "搜索自动化流程" } }),
        Button({ store: vm$.ui.btn_import_pipeline$, attributes: { n: "automation-import-pipeline", type: "button" } }, ["导入 JSON"]),
        Button({ store: vm$.ui.btn_create_pipeline$, attributes: { n: "automation-create-pipeline", type: "button" } }, ["创建流程"]),
      ]),
      Table({
        name: "automation-definition-table",
        containerClass: "flex-1 min-h-0 min-w-0 px-4 pb-4 flex flex-col",
        containerAttributes: { n: "automation-definition-table-container" },
        panelAttributes: { n: "automation-definition-table-panel" },
        rowKey: (definition) => `${definition.id}:${definition.updated_at}:${definition.instance_count}:${definition.id}`,
        rows: vm$.state.filtered_definitions,
        loading: vm$.state.loading,
        status: computed(vm$.state.filtered_definitions, (definitions) => definitions.length ? "normal" : "empty"),
        emptyTitle: "暂无匹配的自动化流程",
        emptyDescription: "创建或导入一个流程，再添加配置实例。",
        columns: [
          { name: "name", title: "自动化流程", width: "minmax(180px, 1fr)", render(definition) {
            return [View({ class: "font-semibold text-color-text-primary overflow-wrap-anywhere", attributes: { n: `automation-definition-name-${definition.id}` } }, [definition.name])];
          } },
          { name: "instances", title: "配置实例", width: 95, render(definition) { return [String(definition.instance_count)]; } },
          { name: "config", title: "配置", width: "minmax(240px, 1fr)", cellClass: "min-w-0 flex flex-col items-start gap-3", render(definition) {
            return [
              AutomationConfigValues({ store: vm$, pipeline: definition }),
              View({ class: "dm-button-group", attributes: { n: `automation-definition-config-actions-${definition.id}`, role: "group", "aria-label": `默认配置 ${definition.name || definition.id}` } }, [
                Button({ store: vm$.ui.btn_definition_config$.bind(definition), class: "flex-none", attributes: { n: `automation-definition-defaults-${definition.id}`, type: "button" } }, ["默认配置"]),
                DropdownMenu({ store: vm$.methods.pipelineConfigMenu(definition), attributes: { n: `automation-definition-config-menu-${definition.id}` } }, [
                  Button({ store: vm$.ui.btn_definition_config_menu$.bind(definition), class: "dm-button--icon-sm", attributes: { n: `automation-definition-config-menu-trigger-${definition.id}`, type: "button", "aria-label": `${definition.name || definition.id} 复制或导入默认配置 JSON` } }, [
                    Timeless.Icon({ name: "chevron-down", size: 14, attributes: { n: `automation-definition-config-menu-icon-${definition.id}`, "aria-hidden": "true" } }),
                  ]),
                ]),
              ]),
            ].filter(Boolean);
          } },
          { name: "actions", title: "操作", width: 320, cellClass: "flex flex-nowrap items-center justify-end gap-2 whitespace-nowrap", render(definition) {
            return [
              TextButton({ store: vm$.ui.btn_definition_instance$.bind(definition), attributes: { n: `automation-definition-create-instance-${definition.id}`, type: "button" } }, ["新增配置实例"]),
              AutomationPipelineLink({ flowId: definition.id, mode: "edit", label: "编辑流程" }),
            ];
          } },
        ],
      }),
    ]),
  ]);
}

function AutomationConfigDialog(props) {
  const vm$ = props.store;
  return Dialog({ store: vm$.ui.config_dialog$, class: "automation-form-dialog", attributes: { n: "automation-config-dialog" } }, [
    DialogHeader({}, [
      DialogTitle({}, [computed(vm$.state.config_defaults, (defaults) => defaults ? "流程默认配置" : "实例覆盖配置")]),
      DialogDescription({}, [computed(vm$.state.config_defaults, (defaults) => defaults ? "修改后，所有未覆盖相应配置项的实例在下次执行时使用新值。" : "勾选继承的配置项在执行时读取流程的最新配置；取消勾选后可为当前实例填写覆盖值。")]),
    ]),
    DialogBody({}, [
      Label({ attributes: { n: "automation-config-name-label", for: "automation-config-name" } }, [computed(vm$.state.config_defaults, (defaults) => defaults ? "流程名称" : "配置实例名称")]),
      Input({ store: vm$.ui.input_config_name$, attributes: { n: "automation-config-name", id: "automation-config-name", "aria-label": computed(vm$.state.config_defaults, (defaults) => defaults ? "流程名称" : "配置实例名称") } }),
      Show({ when: vm$.state.config_loading, ok: () => View({ class: "text-color-text-secondary" }, ["正在加载配置…"]) }),
      Show({ when: computed(vm$.state.config_error, Boolean), ok: () => View({ class: "text-color-danger text-sm", attributes: { role: "alert" } }, [vm$.state.config_error]) }),
      Show({ when: combine({ groups: vm$.state.config_groups, loading: vm$.state.config_loading, error: vm$.state.config_error }, ({ groups, loading, error }) => !loading && !error && !groups.length), ok: () => View({ class: "text-color-text-secondary text-sm" }, ["此流程尚未声明配置项；可先修改实例名称，配置项在共享流程中声明。"]) }),
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
                : Input({ store: field.input$, onChange, attributes: { ...attributes, type: field.secret ? "secret" : field.type === "number" || field.type === "integer" ? "number" : "text", step: field.type === "number" ? "any" : undefined, autocomplete: "off", spellcheck: field.secret ? "false" : undefined } });
            return View({ class: "min-w-0 flex flex-col gap-1" }, [
              field.type !== "boolean" ? Label({ class: "text-color-text-secondary text-xs font-semibold", attributes: { for: id } }, [`${field.label || field.key}${field.required ? " *" : ""}`]) : null,
              field.inherit$ ? Checkbox({ store: field.inherit$, attributes: { n: `${id}-inherit`, "aria-label": `${field.label || field.key} 继承流程配置` } }, ["继承流程配置"]) : null,
              field.inherited ? Show({ when: field.inherited, ok: () => View({ class: "text-color-text-secondary text-sm", attributes: { n: `${id}-inherited-value` } }, [field.inherited_text]), else: () => input }) : input,
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

function AutomationPasteConfigDialog(props) {
  const vm$ = props.store;
  return Dialog({ store: vm$.ui.config_json_dialog$, class: "automation-form-dialog", attributes: { n: "automation-paste-config-dialog" } }, [
    DialogHeader({ attributes: { n: "automation-paste-config-header" } }, [
      DialogTitle({ attributes: { n: "automation-paste-config-title" } }, [computed(vm$.state.config_defaults, (defaults) => defaults ? "导入JSON" : "粘贴JSON")]),
      DialogDescription({ attributes: { n: "automation-paste-config-description", id: "automation-paste-config-description" } }, [computed(vm$.state.config_defaults, (defaults) => defaults ? "输入流程默认配置 JSON，保存后供实例继承；未提供的配置项保持原值。" : "输入当前记录的配置 JSON，点击保存后生效。")]),
    ]),
    DialogBody({ attributes: { n: "automation-paste-config-body" } }, [
      Label({ attributes: { n: "automation-paste-config-label", for: "automation-paste-config-json" } }, ["配置 JSON"]),
      Textarea({ store: vm$.ui.input_config_json$, class: "font-mono text-sm", onChange(event) { vm$.ui.input_config_json$.setValue(String(event.target.value)); }, attributes: { n: "automation-paste-config-json", id: "automation-paste-config-json", rows: "12", spellcheck: "false", "aria-label": "配置 JSON", "aria-describedby": "automation-paste-config-description" } }),
      Show({ when: computed(vm$.state.config_error, Boolean), ok: () => View({ class: "text-color-danger text-sm", attributes: { n: "automation-paste-config-error", role: "alert" } }, [vm$.state.config_error]) }),
    ]),
    DialogFooter({ attributes: { n: "automation-paste-config-actions" } }, [
      Button({ store: vm$.ui.btn_config_json_cancel$, attributes: { n: "automation-paste-config-cancel", type: "button" } }, ["取消"]),
      Button({ store: vm$.ui.btn_config_json_save$, attributes: { n: "automation-paste-config-save", type: "button" } }, ["保存"]),
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
        DialogTitle({}, ["创建自动化流程"]),
        DialogDescription({}, [
          "为开始节点声明接收参数；创建后编辑共享流程，再新增配置实例并设置触发器。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "flex flex-col gap-3" }, [
          View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-3 [&_.dm-button]:align-self-flex-end max-900:grid-cols-1fr" }, [
            View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
              View({ class: "text-color-text-secondary text-xs font-semibold" }, ["流程名称"]),
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
          ["创建流程"],
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
    title: "删除流程实例",
    description: combine(
      { flow_id: vm$.state.delete_flow_id, pipelines: vm$.state.pipelines },
      ({ flow_id, pipelines }) => {
        const flow = (pipelines || []).find((item) => item.id === flow_id);
        return flow
          ? `确定删除配置实例「${flow.name || flow.id}」？其他实例不受影响，删除后不可恢复。`
          : "确定删除当前流程实例？删除后不可恢复。";
      },
    ),
    cancelText: "取消",
    okText: "删除流程实例",
  });
}

export default AutomationPageView;

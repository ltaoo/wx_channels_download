import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationFeedback,
  AutomationPipelineLink,
  AutomationPipelineTriggerBadges,
} from "./flow.components.js";

function AutomationPageView(props) {
  const vm$ = AutomationPageViewModel(props);
  return View(
    {
      class: "content-page automation-page page",
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
    ],
  );
}

function AutomationListPage(props) {
  const vm$ = props.store;
  return View({ class: "automation-list-page" }, [
    View({ class: "content-toolbar-wrap container" }, [
      AutomationPageToolbar({ store: vm$ }),
    ]),
    AutomationFeedback({ store: vm$ }),
    AutomationPipelineList({ store: vm$ }),
  ]);
}

function AutomationPageToolbar(props) {
  const vm$ = props.store;
  return View({ class: "content-toolbar automation-toolbar" }, [
    View({ class: "dm-flex dm-items-center dm-gap-2" }, [
      View({ class: "automation-toolbar-title" }, ["自动化"]),
    ]),
    View({ class: "dm-flex dm-items-center dm-gap-2" }, [
      Button(
        {
          store: vm$.ui.btn_refresh$,
          attributes: { n: "automation-refresh-button", type: "button" },
        },
        ["刷新"],
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
        ["创建 Pipeline"],
      ),
    ]),
  ]);
}

function AutomationPipelineList(props) {
  const vm$ = props.store;
  return Table({
    name: "automation-pipeline-table",
    containerClass: "content-main container automation-index",
    containerAttributes: { n: "automation-pipeline-table-container" },
    panelAttributes: { n: "automation-pipeline-table-panel" },
    rowKey: "id",
    columns: [
      {
        name: "name",
        title: "名称",
        width: "minmax(240px, 2fr)",
        cellClass: "automation-table-name-cell",
        render(pipeline) {
          return [
            View(
              {
                class: "automation-table-name",
                attributes: { title: pipeline.name || pipeline.id },
              },
              [pipeline.name || pipeline.id],
            ),
            View(
              {
                class: "automation-table-subtitle",
                attributes: { title: pipeline.id },
              },
              [pipeline.id],
            ),
            pipeline.description
              ? View(
                  {
                    class: "automation-table-description",
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
        cellClass: "automation-table-badge-cell",
        render(pipeline) {
          return AutomationPipelineTriggerBadges({ store: vm$, pipeline });
        },
      },
      {
        name: "scale",
        title: "节点 / 连线",
        width: 130,
        cellClass: "automation-table-metric",
        render(pipeline) {
          return [
            `${(pipeline.nodes || []).length} / ${(pipeline.edges || []).length}`,
          ];
        },
      },
      {
        name: "updated",
        title: "最后更新",
        width: 170,
        cellClass: "automation-table-metric",
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
        cellClass: "automation-table-actions",
        render(pipeline) {
          return [
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
          ];
        },
      },
    ],
    rows: vm$.state.pipelines,
    status: computed(vm$.state.pipelines, (pipelines) =>
      pipelines.length === 0 ? "empty" : "normal",
    ),
    loading: vm$.state.loading,
    emptyTitle: computed(vm$.state.loading, (loading) =>
      loading ? "正在加载 Pipeline…" : "暂无 Pipeline",
    ),
    emptyDescription: computed(vm$.state.loading, (loading) =>
      loading
        ? "正在获取最新的流程配置。"
        : "点击右上角「创建 Pipeline」开始搭建流程。",
    ),
  });
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
        DialogTitle({}, ["创建 Pipeline"]),
        DialogDescription({}, [
          "为开始节点声明接收参数；创建后在编辑器中添加节点，并配置定时或事件触发器。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
          View({ class: "automation-form__row" }, [
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["Pipeline 名称"]),
              Input({
                store: vm$.ui.input_create_name$,
                attributes: { n: "automation-create-name" },
              }),
            ]),
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["描述"]),
              Input({
                store: vm$.ui.input_create_description$,
                attributes: { n: "automation-create-description" },
              }),
            ]),
          ]),
          View({ class: "automation-form__legend" }, ["开始节点接收的参数"]),
          For({
            key: "uid",
            each: vm$.state.create_params,
            render(param_) {
              const param =
                param_ && param_.value !== undefined ? param_.value : param_;
              return View({ class: "automation-form__row" }, [
                View({ class: "automation-form__field" }, [
                  View({ class: "automation-form__label" }, ["参数名"]),
                  Input({
                    store: param.input_key$,
                    attributes: {
                      n: `automation-create-param-key-${param.uid}`,
                      placeholder: "例如 url",
                    },
                  }),
                ]),
                View({ class: "automation-form__field" }, [
                  View({ class: "automation-form__label" }, ["类型"]),
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
          ["创建 Pipeline"],
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
        View({ class: "automation-form" }, [
          View({ class: "automation-form__field" }, [
            Label({ class: "automation-form__label" }, ["流程定义 JSON"]),
            Textarea({
              store: vm$.ui.input_import_json$,
              class: "automation-node-config-input",
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
          Show({
            when: vm$.state.import_error,
            ok() {
              return View(
                {
                  class: "dm-alert is-destructive",
                  attributes: { role: "alert" },
                },
                [vm$.state.import_error],
              );
            },
          }),
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

import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFeedback,
  AutomationFlowGraph,
  AutomationRunPipelineDialog,
  AutomationTriggerOption,
  AutomationWorkspaceToolbar,
  automation_trigger_badge,
} from "./flow.components.js";

function AutomationPipelineDetail(props) {
  const vm$ = props.store;
  return View({ class: "dm-panel automation-detail" }, [
    Show({
      when: vm$.state.selected_pipeline,
      ok() {
        const flow = vm$.state.selected_pipeline.value;
        return View({ class: "automation-detail__body" }, [
          View({ class: "automation-detail__header" }, [
            View({}, [
              View({ class: "automation-detail__title" }, [
                flow.name || flow.id,
              ]),
              View(
                {
                  class: "automation-detail__subtitle",
                  attributes: { title: flow.id },
                },
                [flow.id],
              ),
            ]),
            automation_trigger_badge({
              type: flow.trigger_type,
              label: vm$.methods.triggerLabel(flow.trigger_type),
            }),
          ]),
          View({ class: "automation-detail__description" }, [
            flow.description || "暂无描述",
          ]),
          View({ class: "dm-panel dm-panel--soft automation-properties" }, [
            View({ class: "automation-summary__item" }, [
              View({ class: "automation-property__label" }, ["上下文 Schema"]),
              View({ class: "automation-code" }, [
                vm$.methods.schemaText(flow.context_schema),
              ]),
            ]),
            View({ class: "automation-summary__item" }, [
              View({ class: "automation-property__label" }, ["开始节点"]),
              View({ class: "automation-code" }, [flow.start_node_id]),
            ]),
            View({ class: "automation-summary__item" }, [
              View({ class: "automation-property__label" }, ["节点 / 连线"]),
              View({}, [
                `${(flow.nodes || []).length} / ${(flow.edges || []).length}`,
              ]),
            ]),
          ]),
          View({ class: "automation-section-title" }, ["流程结构"]),
          AutomationFlowGraph({ store: vm$, editable: false }),
        ]);
      },
    }),
  ]);
}

function AutomationCreateScheduleDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.schedule_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-schedule-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["创建自动化流程"]),
        DialogDescription({}, ["把 Pipeline 接入定时 / 事件 / 手动触发。"]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
          View({ class: "automation-form__legend" }, ["触发方式"]),
          View(
            {
              class: "automation-trigger-options",
              attributes: {
                role: "radiogroup",
                "aria-label": "自动化流程触发方式",
              },
            },
            [
              AutomationTriggerOption({
                store: vm$,
                current: vm$.state.schedule_trigger_type,
                namePrefix: "schedule",
                value: "Cron",
                label: "定时触发",
                hint: "按 Cron 计划执行",
                onSelect: vm$.methods.setScheduleTriggerType,
              }),
              AutomationTriggerOption({
                store: vm$,
                current: vm$.state.schedule_trigger_type,
                namePrefix: "schedule",
                value: "Event",
                label: "事件触发",
                hint: "通过事件 Key 触发",
                onSelect: vm$.methods.setScheduleTriggerType,
              }),
              AutomationTriggerOption({
                store: vm$,
                current: vm$.state.schedule_trigger_type,
                namePrefix: "schedule",
                value: "Manual",
                label: "手动触发",
                hint: "在列表中手动执行",
                onSelect: vm$.methods.setScheduleTriggerType,
              }),
            ],
          ),
          Show({
            when: computed(
              vm$.state.schedule_trigger_type,
              (type) => type === "Event",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["事件 Key"]),
                Input({
                  store: vm$.ui.input_schedule_event_key$,
                  attributes: { n: "automation-schedule-event-key" },
                }),
              ]);
            },
          }),
          Show({
            when: computed(
              vm$.state.schedule_trigger_type,
              (type) => type === "Cron",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["Cron 表达式"]),
                Input({
                  store: vm$.ui.input_schedule_cron$,
                  attributes: { n: "automation-schedule-cron" },
                }),
              ]);
            },
          }),
          View({ class: "automation-form__row" }, [
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["Pipeline"]),
              Select({
                store: vm$.ui.select_schedule_flow$,
                attributes: { n: "automation-schedule-flow" },
              }),
            ]),
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["开始节点"]),
              Select({
                store: vm$.ui.select_schedule_start$,
                attributes: { n: "automation-schedule-start" },
              }),
            ]),
          ]),
          View({ class: "automation-form__field" }, [
            View({ class: "automation-form__label" }, ["流程名称"]),
            Input({
              store: vm$.ui.input_schedule_name$,
              attributes: { n: "automation-schedule-name" },
            }),
          ]),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_schedule_cancel$,
            attributes: { n: "automation-schedule-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_schedule_submit$,
            attributes: { n: "automation-schedule-submit", type: "button" },
          },
          ["创建流程"],
        ),
      ]),
    ],
  );
}

function FlowDetailPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "detail" });
  return View(
    {
      class: "content-page automation-page page",
      attributes: { n: "automation-page" },
      onMounted() {
        vm$.methods.ready();
      },
      beforeUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "automation-detail-page" }, [
        View({ class: "content-toolbar-wrap container" }, [
          AutomationWorkspaceToolbar({ store: vm$, mode: "detail" }),
        ]),
        AutomationFeedback({ store: vm$ }),
        View({ class: "content-main container" }, [
          Show({
            when: vm$.state.selected_pipeline,
            ok() {
              return AutomationPipelineDetail({ store: vm$ });
            },
          }),
          Show({
            when: computed(vm$.state.selected_pipeline, (flow) => !flow),
            ok() {
              return AutomationEmptyState({
                detail: true,
                name: "automation-pipeline-detail-empty",
                title: computed(vm$.state.loading, (loading) =>
                  loading ? "正在加载 Pipeline…" : "未找到 Pipeline",
                ),
                description: computed(vm$.state.loading, (loading) =>
                  loading
                    ? "正在获取流程定义和节点信息。"
                    : "请返回列表确认 Pipeline 是否仍然存在。",
                ),
              });
            },
          }),
        ]),
      ]),
      AutomationCreateScheduleDialog({ store: vm$ }),
      AutomationRunPipelineDialog({ store: vm$ }),
    ],
  );
}

export default FlowDetailPageView;

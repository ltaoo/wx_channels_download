import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFeedback,
  AutomationFlowGraph,
  AutomationRunPipelineDialog,
  AutomationTriggerSummary,
  AutomationWorkspaceToolbar,
  AutomationPipelineTriggerBadges,
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
            AutomationPipelineTriggerBadges({ store: vm$, pipeline: flow }),
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
          AutomationTriggerSummary({ store: vm$ }),
          View({ class: "automation-section-title" }, ["流程结构"]),
          AutomationFlowGraph({ store: vm$, editable: false }),
        ]);
      },
    }),
  ]);
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
      AutomationRunPipelineDialog({ store: vm$ }),
    ],
  );
}

export default FlowDetailPageView;

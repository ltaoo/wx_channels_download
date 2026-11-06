import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFeedback,
  AutomationFlowGraph,
  AutomationRunPipelineDialog,
  AutomationTriggerSummary,
  AutomationWorkspaceToolbar,
  AutomationPipelineTriggerBadges,
  AutomationRunHistory,
} from "./automation.components.js";

function AutomationPipelineDetail(props) {
  const vm$ = props.store;
  return View({ class: "flex flex-col p-4 gap-4 dm-panel" }, [
    Show({
      when: vm$.state.selected_pipeline,
      ok() {
        const flow = vm$.state.selected_pipeline.value;
        return View({ class: "flex flex-col gap-4" }, [
          View({ class: "flex max-900:flex-col items-start justify-between max-900:items-stretch gap-5" }, [
            View({}, [
              View({ class: "font-display text-xl font-semibold text-color-text-primary" }, [
                flow.name || flow.id,
              ]),
              View(
                {
                  class: "mt-0.5 font-mono text-xs text-color-text-secondary",
                  attributes: { title: flow.id },
                },
                [flow.id],
              ),
            ]),
            AutomationPipelineTriggerBadges({ store: vm$, pipeline: flow }),
          ]),
          View({ class: "text-sm leading-relaxed text-color-text-secondary" }, [
            flow.description || "暂无描述",
          ]),
          View({ class: "grid grid-cols-3 max-900:grid-cols-1fr p-3 gap-3 dm-panel--soft dm-panel" }, [
            View({ class: "min-w-0" }, [
              View({ class: "block mb-1 text-xs font-medium text-color-text-secondary" }, ["上下文 Schema"]),
              View({ class: "overflow-hidden font-mono text-sm whitespace-nowrap text-ellipsis code-label" }, [
                vm$.methods.schemaText(flow.context_schema),
              ]),
            ]),
            View({ class: "min-w-0" }, [
              View({ class: "block mb-1 text-xs font-medium text-color-text-secondary" }, ["开始节点"]),
              View({ class: "overflow-hidden font-mono text-sm whitespace-nowrap text-ellipsis code-label" }, [flow.start_node_id]),
            ]),
            View({ class: "min-w-0" }, [
              View({ class: "block mb-1 text-xs font-medium text-color-text-secondary" }, ["节点 / 连线"]),
              View({}, [
                `${(flow.nodes || []).length} / ${(flow.edges || []).length}`,
              ]),
            ]),
          ]),
          AutomationTriggerSummary({ store: vm$ }),
          View({ class: "text-base font-semibold text-color-text-primary" }, ["流程结构"]),
          AutomationFlowGraph({ store: vm$, editable: false }),
        ]);
      },
    }),
  ]);
}

function AutomationDetailPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "detail" });
  return View(
    {
      class: "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page toolbar-layout",
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
        View({ class: "pt-4 pb-0 px-4 group-content-toolbar-wrap " }, [
          AutomationWorkspaceToolbar({ store: vm$, mode: "detail" }),
        ]),
        AutomationFeedback({ store: vm$ }),
        AutomationRunHistory({ store: vm$ }),
        View({ class: "min-w-0 flex-auto p-4 group-content-main " }, [
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

export default AutomationDetailPageView;

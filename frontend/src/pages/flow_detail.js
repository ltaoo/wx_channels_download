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
} from "./flow.components.js";

function AutomationPipelineDetail(props) {
  const vm$ = props.store;
  return View({ class: "dm-panel flex flex-col gap-4 p-4" }, [
    Show({
      when: vm$.state.selected_pipeline,
      ok() {
        const flow = vm$.state.selected_pipeline.value;
        return View({ class: "flex flex-col gap-4" }, [
          View({ class: "flex items-start justify-between gap-5 max-900:items-stretch max-900:flex-col" }, [
            View({}, [
              View({ class: "text-color-text-primary font-display text-xl font-semibold" }, [
                flow.name || flow.id,
              ]),
              View(
                {
                  class: "mt-0-5 text-color-text-secondary font-mono text-xs",
                  attributes: { title: flow.id },
                },
                [flow.id],
              ),
            ]),
            AutomationPipelineTriggerBadges({ store: vm$, pipeline: flow }),
          ]),
          View({ class: "text-color-text-secondary text-sm leading-relaxed" }, [
            flow.description || "暂无描述",
          ]),
          View({ class: "dm-panel dm-panel--soft grid grid-cols-[repeat(3,_minmax(0,_1fr))] gap-3 p-3 max-900:grid-cols-1fr" }, [
            View({ class: "min-w-0 [&.is-wide]:grid-column-[1_/_-1]" }, [
              View({ class: "block mb-1 text-color-text-secondary text-xs font-medium" }, ["上下文 Schema"]),
              View({ class: "group-automation-code overflow-hidden font-mono text-sm text-ellipsis whitespace-nowrap" }, [
                vm$.methods.schemaText(flow.context_schema),
              ]),
            ]),
            View({ class: "min-w-0 [&.is-wide]:grid-column-[1_/_-1]" }, [
              View({ class: "block mb-1 text-color-text-secondary text-xs font-medium" }, ["开始节点"]),
              View({ class: "group-automation-code overflow-hidden font-mono text-sm text-ellipsis whitespace-nowrap" }, [flow.start_node_id]),
            ]),
            View({ class: "min-w-0 [&.is-wide]:grid-column-[1_/_-1]" }, [
              View({ class: "block mb-1 text-color-text-secondary text-xs font-medium" }, ["节点 / 连线"]),
              View({}, [
                `${(flow.nodes || []).length} / ${(flow.edges || []).length}`,
              ]),
            ]),
          ]),
          AutomationTriggerSummary({ store: vm$ }),
          View({ class: "text-color-text-primary text-base font-semibold" }, ["流程结构"]),
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
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [&_.group-automation-toolbar]:flex [&_.group-automation-toolbar]:items-center [&_.group-automation-toolbar]:justify-between [&_.group-automation-toolbar]:gap-4 max-900:[&_.group-automation-toolbar]:items-stretch max-900:[&_.group-automation-toolbar]:flex-col min-w-0 min-h-full font-sans",
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
        View({ class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0] " }, [
          AutomationWorkspaceToolbar({ store: vm$, mode: "detail" }),
        ]),
        AutomationFeedback({ store: vm$ }),
        AutomationRunHistory({ store: vm$ }),
        View({ class: "group-content-main flex-[1_1_auto] min-w-0 p-4 " }, [
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

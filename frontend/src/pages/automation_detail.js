import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFlowGraph,
  AutomationNodeExecutionWindows,
  AutomationRunPipelineDialog,
  AutomationRunHistory,
  automation_status_badge,
} from "./automation.components.js";

function AutomationDetailToolbar(props) {
  const vm$ = props.store;
  return View({
    class: "dm-panel fixed top-4 left-4 right-4 z-sticky flex items-center justify-between gap-3 p-3 min-w-0 shadow-md",
    attributes: { n: "automation-detail-toolbar" },
  }, [
    View({ class: "min-w-0 flex-1" }, [
      View({ class: "text-color-text-secondary text-caption" }, ["Pipeline 执行详情"]),
      View({
        class: "overflow-hidden text-color-text-primary font-display text-base font-semibold text-ellipsis whitespace-nowrap",
        attributes: { title: computed(vm$.state.selected_pipeline, (flow) => flow ? flow.name || flow.id : "") },
      }, [computed(vm$.state.selected_pipeline, (flow) => flow ? flow.name || flow.id : "正在加载 Pipeline…")]),
    ]),
    View({ class: "flex items-center gap-2 flex-none" }, [
      For({ each: computed(vm$.state.execution_run_status, (status) => status ? [{ status }] : []), key: "status", render(item) {
        return automation_status_badge(item.status);
      } }),
      Button({ store: vm$.ui.btn_run_flow$, attributes: { n: "automation-run-flow", type: "button" } }, ["手动执行"]),
    ]),
  ]);
}

function FlowDetailPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "detail" });
  return View({
    class: "group-content-page relative h-full min-h-0 overflow-hidden bg-content-bg text-content-text min-w-0 font-sans",
    attributes: { n: "automation-page" },
    onMounted() { vm$.methods.ready(); },
    beforeUnmounted() { vm$.methods.destroy(); },
  }, [
    View({
      class: "absolute inset-0 [&_.group-automation-flow-scroll]:h-full [&_.group-automation-flow-scroll]:min-h-0 [&_.group-automation-flow-scroll]:border-0 [&_.group-automation-flow-scroll]:rounded-0",
      attributes: { n: "automation-detail-workspace" },
    }, [
      Show({ when: vm$.state.selected_pipeline, ok() {
        return For({ each: computed(vm$.state.canvas_revision, (revision) => [{ revision }]), key: "revision", render() {
          return AutomationFlowGraph({ store: vm$, editable: false, executionLogs: true });
        } });
      } }),
      Show({ when: computed(vm$.state.selected_pipeline, (flow) => !flow), ok() {
        return AutomationEmptyState({
          detail: true,
          name: "automation-pipeline-detail-empty",
          title: computed(vm$.state.loading, (loading) => loading ? "正在加载 Pipeline…" : "未找到 Pipeline"),
          description: "请返回列表确认 Pipeline 是否仍然存在。",
        });
      } }),
    ]),
    AutomationDetailToolbar({ store: vm$ }),
    View({
      class: "fixed z-sticky top-[var(--dm-space-24)] left-4 bottom-4 w-[var(--dm-layout-sidebar-width)] max-760:w-[calc(var(--dm-space-24)_*_2)] min-w-0 min-h-0",
      attributes: { n: "automation-run-sidebar", "aria-label": "执行记录" },
    }, [AutomationRunHistory({ store: vm$ })]),
    AutomationRunPipelineDialog({ store: vm$ }),
    AutomationNodeExecutionWindows({ store: vm$ }),
  ]);
}

export default FlowDetailPageView;

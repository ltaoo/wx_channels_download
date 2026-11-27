import { PlatformIcon } from "@/dmui.js";
import { PlatformsPageModel } from "./platforms.model.js";

function PlatformField({ label, name, store, busy, select = false }) {
  return View({ class: "flex flex-col gap-2", attributes: { n: `${name}-field` } }, [
    View({ as: "label", class: "text-sm font-semibold", attributes: { n: `${name}-label`, for: name } }, [label]),
    select
      ? Select({ store, attributes: { n: name, id: name, "aria-label": label, disabled: busy } })
      : Input({ store, rootAttributes: { n: `${name}-control` }, attributes: { n: name, id: name, "aria-label": label, disabled: busy } }),
  ]);
}

function PlatformResultPanel({ operation, label, state, copy }) {
  return View({ class: "platform-result-panel", attributes: { n: `platform-${operation}-result-panel`, "aria-label": `${label}结果 JSON` } }, [
    View({ class: "platform-result-toolbar", attributes: { n: `platform-${operation}-result-toolbar` } }, [
      View({ as: "h3", class: "m-0 text-base font-semibold", attributes: { n: `platform-${operation}-result-title` } }, [`${label} JSON`]),
      View({ class: "text-sm text-content-muted", attributes: { n: `platform-${operation}-fetch-elapsed`, role: "status" } }, [state.elapsed]),
      Show({ when: state.has_result, ok() { return Button({ store: copy, attributes: { n: `platform-${operation}-copy-result`, type: "button" } }, ["复制 JSON"]); } }),
    ]),
    View({ class: "platform-result-scroll", attributes: { n: `platform-${operation}-result-scroll` } }, [
      Show({ when: state.busy, ok() { return View({ class: "p-4 text-sm text-content-muted", attributes: { n: `platform-${operation}-fetch-loading`, role: "status" } }, ["正在请求平台，请稍候…"]); } }),
      Show({ when: state.error, ok() { return View({ class: "p-4 text-sm text-color-danger whitespace-pre-wrap break-words", attributes: { n: `platform-${operation}-fetch-error`, role: "alert" } }, [state.error]); } }),
      Show({ when: state.has_result, ok() {
        return View({ as: "pre", class: "platform-result-json text-xs font-mono", attributes: { n: `platform-${operation}-result-json`, tabindex: "0", "aria-label": `${label}返回的 JSON` } }, [state.result_text]);
      } }),
      Show({ when: combine({ busy: state.busy, error: state.error, result: state.has_result }, ({ busy, error, result }) => !busy && !error && !result), ok() {
        return View({ class: "p-4 text-sm text-content-muted", attributes: { n: `platform-${operation}-result-empty` } }, [`获取${label}后，在此查看 JSON。`]);
      } }),
    ]),
  ]);
}

function PlatformDetailPanel({ store: model }) {
  return View({ as: "section", class: "platform-detail-panel", attributes: { n: "platform-detail-panel", "aria-labelledby": "platform-detail-title" } }, [
    View({ class: "platform-detail-header", attributes: { n: "platform-detail-header" } }, [
      View({ as: "h2", class: "m-0 text-base font-semibold", attributes: { n: "platform-detail-title", id: "platform-detail-title" } }, [model.state.title]),
    ]),
    Show({ when: model.state.selected, ok() {
      return View({ class: "platform-detail-body", attributes: { n: "platform-detail-body" } }, [
        View({ class: "platform-forms", attributes: { n: "platform-forms" } }, [
          View({ as: "form", class: "platform-fetch-section", attributes: { n: "platform-list-form", "aria-labelledby": "platform-list-form-title" }, onSubmit(event) { event.preventDefault(); return model.methods.run("list"); } }, [
            View({ as: "h3", class: "m-0 text-base font-semibold", attributes: { n: "platform-list-form-title", id: "platform-list-form-title" } }, ["获取列表"]),
            Show({ when: model.state.supports_list, ok() {
              return View({ class: "flex flex-col gap-4", attributes: { n: "platform-list-fields" } }, [
                PlatformField({ label: "主页或账号", name: "platform-profile", store: model.ui.profile$, busy: model.state.busy }),
                Show({ when: model.state.needs_user_id, ok() { return PlatformField({ label: "用户数字 ID", name: "platform-user-id", store: model.ui.user_id$, busy: model.state.busy }); } }),
                PlatformField({ label: "列表范围", name: "platform-scope", store: model.ui.scope$, busy: model.state.busy, select: true }),
                PlatformField({ label: "分页游标", name: "platform-page", store: model.ui.page$, busy: model.state.busy }),
              ]);
            }, else() {
              return View({ class: "text-sm text-content-muted", attributes: { n: "platform-list-unavailable" } }, ["该平台尚未实现主页列表。"]);
            } }),
            View({ class: "platform-list-actions", attributes: { n: "platform-list-actions" } }, [
              Button({ store: model.ui.list$, attributes: { n: "platform-fetch-list", type: "submit" } }, ["获取列表"]),
              Show({ when: model.state.next_marker, ok() { return Button({ store: model.ui.next$, attributes: { n: "platform-next-page", type: "button" } }, ["获取下一页"]); } }),
            ]),
          ]),
          View({ as: "form", class: "platform-fetch-section", attributes: { n: "platform-detail-form", "aria-labelledby": "platform-detail-form-title" }, onSubmit(event) { event.preventDefault(); return model.methods.run("detail"); } }, [
            View({ as: "h3", class: "m-0 text-base font-semibold", attributes: { n: "platform-detail-form-title", id: "platform-detail-form-title" } }, ["获取详情"]),
            PlatformField({ label: "内容链接", name: "platform-content-url", store: model.ui.url$, busy: model.state.busy }),
            Button({ store: model.ui.detail$, attributes: { n: "platform-fetch-detail", type: "submit" } }, ["获取详情"]),
          ]),
          PlatformResultPanel({ operation: "list", label: "列表", state: model.state.list_result, copy: model.ui.copy_list$ }),
          PlatformResultPanel({ operation: "detail", label: "详情", state: model.state.detail_result, copy: model.ui.copy_detail$ }),
        ]),
      ]);
    }, else() {
      return View({ class: "platform-detail-empty text-content-muted", attributes: { n: "platform-detail-empty" } }, ["选择一个平台，查看详情或获取主页列表。"]);
    } }),
  ]);
}

export default function PlatformsPageView(props) {
  const model = PlatformsPageModel(props);
  return View({ class: "platforms-page bg-content-bg text-content-text", attributes: { n: "platforms-page" }, onMounted() { return model.methods.load(); }, beforeUnmounted() { model.methods.destroy(); } }, [
    View({ class: "flex items-start justify-between gap-3 max-520:flex-col", attributes: { n: "platforms-header" } }, [
      View({ class: "flex flex-col gap-1", attributes: { n: "platforms-heading" } }, [
        View({ as: "h1", class: "m-0 text-xl font-semibold", attributes: { n: "platforms-title" } }, ["平台列表"]),
        View({ class: "text-sm text-content-muted", attributes: { n: "platforms-description" } }, ["选择平台，测试内容详情与主页列表。"]),
      ]),
      View({ class: "text-sm text-content-muted", attributes: { n: "platforms-count" } }, [model.state.count]),
    ]),
    View({ class: "platforms-layout", attributes: { n: "platforms-layout" } }, [
      View({ as: "aside", class: "platforms-catalog", attributes: { n: "platforms-catalog", "aria-label": "选择平台" } }, [
        View({ class: "platforms-catalog-toolbar", attributes: { n: "platforms-toolbar" } }, [
          Input({ store: model.ui.keyword$, rootClass: "flex-1 min-w-0", rootAttributes: { n: "platforms-search-control" }, attributes: { n: "platforms-search", "aria-label": "搜索平台" } }),
          Button({ store: model.ui.refresh$, attributes: { n: "platforms-refresh", type: "button" } }, ["刷新"]),
        ]),
        Show({ when: model.state.catalog_error, ok() { return View({ class: "px-4 text-sm text-color-danger", attributes: { n: "platforms-error", role: "alert" } }, [model.state.catalog_error]); } }),
        Show({ when: model.state.loading, ok() { return View({ class: "px-4 text-sm text-content-muted", attributes: { n: "platforms-loading", role: "status" } }, ["正在读取平台…"]); } }),
        View({ class: "platforms-directory", attributes: { n: "platforms-directory", "aria-busy": model.state.loading } }, [
          View({ class: "platforms-items", attributes: { n: "platforms-items" } }, [
            For({ each: model.state.visible, key: "id", render(platform) {
              return Button({ store: platform.button$, class: "platform-catalog-row", attributes: { n: `platform-open-${platform.id}`, type: "button", "aria-label": `选择${platform.name}`, "aria-pressed": computed(model.state.selected, (selected) => selected?.id === platform.id ? "true" : "false") } }, [
                PlatformIcon({ favicon: platform.favicon, name: `platform-icon-${platform.id}`, class: "platform-catalog-icon" }),
                View({ class: "flex-1 min-w-0 flex flex-col gap-1", attributes: { n: `platform-identity-${platform.id}` } }, [
                  View({ class: "font-semibold text-base", attributes: { n: `platform-name-${platform.id}` } }, [platform.name]),
                  View({ class: "text-xs text-content-muted", attributes: { n: `platform-id-${platform.id}` } }, [platform.id]),
                ]),
                View({ class: "text-xs text-content-muted flex-none", attributes: { n: `platform-capabilities-${platform.id}` } }, [platform.capabilities]),
              ]);
            } }),
            Show({ when: combine({ loading: model.state.loading, rows: model.state.visible }, ({ loading, rows }) => !loading && !rows.length), ok() { return View({ class: "p-6 text-center text-content-muted", attributes: { n: "platforms-empty" } }, ["没有匹配的平台"]); } }),
          ]),
        ]),
      ]),
      PlatformDetailPanel({ store: model }),
    ]),
  ]);
}

/// <reference path="../utils.js" />
/// <reference path="model.js" />
/**
 * @file Logs page entry.
 */
function LogsPageActionButton(props) {
  return View(
    {
      type: "button",
      class: [
        "wx-content-action",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
    },
    [
      props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
      typeof props.label !== "undefined"
        ? View({ class: "wx-content-action-label" }, [props.label])
        : null,
    ].filter(Boolean),
  );
}

function LogsPageEntryValue(entry_) {
  return entry_ && entry_.value !== undefined ? entry_.value : entry_;
}

function LogsPageFormatTime(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const pad = (number) => String(number).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function LogsPageLevelClass(level) {
  const normalized = String(level || "info").toLowerCase();
  if (normalized === "debug" || normalized === "warn" || normalized === "error") {
    return `wx-logs-level wx-logs-level-${normalized}`;
  }
  return "wx-logs-level wx-logs-level-info";
}

function LogsPageHeader(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "scroll-text", size: 28 }),
        ]),
        View({}, [
          View({ class: "wx-content-title" }, ["日志"]),
          View({ class: "wx-content-subtitle" }, [
            computed(vm$.state.last_loaded_at, (time) => {
              return time
                ? `运行日志、请求代理和下载器事件 · 更新于 ${time}`
                : "运行日志、请求代理和下载器事件";
            }),
          ]),
        ]),
      ]),
      View({ class: "wx-logs-header-actions" }, [
        View({ class: "wx-logs-toggle" }, [
          View({
            type: "input",
            attributes: {
              type: "checkbox",
              "aria-label": "自动刷新",
              checked: computed(vm$.state.auto_refresh, (value) =>
                value ? true : undefined,
              ),
            },
            onChange(event) {
              vm$.methods.setAutoRefresh(event.target.checked);
            },
          }),
          View({}, ["自动刷新"]),
        ]),
        LogsPageActionButton({
          icon: "refresh-cw",
          label: computed(vm$.state.loading, (loading) =>
            loading ? "刷新中" : "刷新",
          ),
          onClick() {
            vm$.methods.refresh();
          },
        }),
      ]),
    ]),
  ]);
}

function LogsPageToolbar(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-toolbar wx-logs-toolbar" }, [
    View({ class: "wx-logs-toolbar-row" }, [
      Timeless.Select({
        store: vm$.ui.source,
        class: "wx-content-type-select wx-logs-select wx-logs-source-select",
      }),
      Timeless.Select({
        store: vm$.ui.level,
        class: "wx-content-type-select wx-logs-select",
      }),
      View({ class: "wx-content-search wx-logs-search" }, [
        Timeless.Icon({ name: "search", size: 16 }),
        View({
          type: "input",
          class: "wx-content-search-input",
          attributes: {
            id: "wxLogKeywordInput",
            type: "search",
            placeholder: "搜索消息、字段、原始日志",
            autocomplete: "off",
          },
          onInput(event) {
            vm$.methods.setKeyword(event.target.value);
          },
          onKeyDown(event) {
            if (event.key === "Enter") {
              vm$.methods.search();
            }
          },
        }),
      ]),
      LogsPageActionButton({
        icon: "search",
        label: "搜索",
        onClick() {
          vm$.methods.search();
        },
      }),
      LogsPageActionButton({
        icon: "rotate-ccw",
        label: "重置",
        onClick() {
          vm$.methods.resetFilters();
          const input = document.getElementById("wxLogKeywordInput");
          if (input) {
            input.value = "";
          }
        },
      }),
      LogsPageActionButton({
        icon: "download",
        label: "导出",
        onClick() {
          vm$.methods.exportLogs();
        },
      }),
    ]),
    View({ class: "wx-logs-toolbar-extra" }, [
      Timeless.Select({
        store: vm$.ui.limit,
        class: "wx-content-type-select wx-logs-select",
      }),
      View({ class: "wx-logs-toggle" }, [
        View({
          type: "input",
          attributes: {
            type: "checkbox",
            "aria-label": "格式化 JSON",
            checked: computed(vm$.state.format_json, (value) =>
              value ? true : undefined,
            ),
          },
          onChange(event) {
            vm$.methods.setFormatJson(event.target.checked);
          },
        }),
        View({}, ["格式化 JSON"]),
      ]),
      View({ class: "wx-logs-summary" }, [vm$.state.summary_text]),
    ]),
  ]);
}

function LogsPageError(props) {
  const vm$ = props.store;
  return Show({
    when: computed(vm$.state.error, (message) => Boolean(message)),
    ok() {
      return View({ class: "wx-logs-error" }, [
        Timeless.Icon({ name: "circle-alert", size: 18 }),
        View({}, [vm$.state.error]),
      ]);
    },
  });
}

function LogsPageRow(props) {
  const vm$ = props.store;
  const entry = LogsPageEntryValue(props.entry) || {};
  const file = entry.file || entry.File || "";
  const component = entry.component || entry.Component || "";
  return View({ class: "wx-logs-table-row", attributes: { role: "row" } }, [
    View({ class: "wx-logs-level-cell", attributes: { role: "cell" } }, [
      View({ class: LogsPageLevelClass(entry.level) }, [entry.level || "info"]),
    ]),
    View({ class: "wx-logs-time-cell", attributes: { role: "cell" } }, [
      LogsPageFormatTime(entry.time),
    ]),
    View(
      {
        class: "wx-logs-file-cell",
        attributes: { role: "cell", title: file },
      },
      [file],
    ),
    View(
      {
        class: "wx-logs-component-cell",
        attributes: { role: "cell", title: component },
      },
      [component],
    ),
    View({ class: "wx-logs-msg-cell", attributes: { role: "cell" } }, [
      entry.message || entry.raw || "",
    ]),
    View({ class: "wx-logs-fields-cell", attributes: { role: "cell" } }, [
      LogsPageFieldsCell({ entry, store: vm$ }),
    ]),
  ]);
}

function LogsPageFieldsCell(props) {
  const vm$ = props.store;
  const fields = vm$.methods.fieldRows(props.entry);
  if (fields.length === 0) {
    return View({ class: "wx-logs-fields-empty" }, ["-"]);
  }
  return View({ class: "wx-logs-fields" }, [
    For({
      each: fields,
      render(row) {
        const text = `${row[0]}=${row[1]}`;
        const copyable = vm$.methods.isJsonFieldValue(row[1]);
        return View({ class: "wx-logs-field", attributes: { title: text } }, [
          View({ class: "wx-logs-field-key" }, [row[0]]),
          copyable
            ? View(
                {
                  type: "button",
                  class:
                    "wx-logs-field-value wx-logs-field-value-copyable",
                  attributes: {
                    type: "button",
                    title: `点击复制 ${row[0]} 的 JSON 值`,
                    "aria-label": `复制 ${row[0]} 的 JSON 值`,
                  },
                  onClick() {
                    vm$.methods.copyJsonFieldValue(row[1]);
                  },
                },
                [
                  View({ class: "wx-logs-field-value-text" }, [row[1]]),
                  View({ class: "wx-logs-field-copy-icon" }, [
                    Timeless.Icon({ name: "copy", size: 12 }),
                  ]),
                ],
              )
            : View({ class: "wx-logs-field-value" }, [row[1]]),
        ]);
      },
    }),
  ]);
}

function LogsPageTableHead() {
  return View({ class: "wx-logs-table-head", attributes: { role: "row" } }, [
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["level"]),
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["time"]),
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["file"]),
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["component"]),
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["msg"]),
    View({ class: "wx-logs-table-head-cell", attributes: { role: "columnheader" } }, ["fields"]),
  ]);
}

function LogsPageTable(props) {
  const vm$ = props.store;
  return View({ class: "wx-logs-table", attributes: { role: "table" } }, [
    LogsPageTableHead(),
    VirtualListView({
      class: "wx-logs-virtual-list",
      key(entry, index) {
        return `${entry.index || index}-${entry.time || ""}-${entry.message || ""}`;
      },
      size: 18,
      buffer: 8,
      gutter: 0,
      itemHeight: 48,
      each: vm$.state.entries,
      render(entry) {
        return LogsPageRow({ entry, store: vm$ });
      },
    }),
  ]);
}

function LogsPageSkeletonTable() {
  return View({ class: "wx-logs-table", attributes: { role: "table" } }, [
    LogsPageTableHead(),
    View(
      { class: "wx-logs-virtual-list" },
      Array.from({ length: 6 }, function () {
        return View({ class: "wx-logs-table-row", attributes: { role: "row" } }, [
          View({ class: "wx-logs-level-cell" }, [
            View({ class: "wx-logs-level wx-logs-level-info" }, ["info"]),
          ]),
          View({ class: "wx-logs-time-cell" }, [
            View({ class: "wx-logs-skeleton-cell", style: { width: "150px" } }),
          ]),
          View({ class: "wx-logs-file-cell" }, [
            View({ class: "wx-logs-skeleton-cell", style: { width: "80px" } }),
          ]),
          View({ class: "wx-logs-component-cell" }, [
            View({ class: "wx-logs-skeleton-cell", style: { width: "100px" } }),
          ]),
          View({ class: "wx-logs-msg-cell" }, [
            View({ class: "wx-logs-skeleton-cell", style: { width: "100%" } }),
          ]),
          View({ class: "wx-logs-fields-cell" }, [
            View({ class: "wx-logs-skeleton-cell", style: { width: "80%" } }),
          ]),
        ]);
      }),
    ),
  ]);
}

function LogsPageLoadingState() {
  return LogsPageSkeletonTable();
}

function LogsPageEmptyState() {
  return View({ class: "wx-logs-state" }, [
    Timeless.Icon({ name: "file-search", size: 32 }),
    View({}, ["没有匹配的日志"]),
  ]);
}

function LogsPageList(props) {
  const vm$ = props.store;
  return View({ class: "wx-logs-list" }, [
    View({ class: "wx-logs-list-head" }, [
      View({ class: "wx-logs-list-title" }, ["日志流"]),
      Show({
        when: vm$.state.loading,
        ok() {
          return View({ class: "wx-logs-loading-badge" }, [
            View({ class: "weui-loading" }),
            View({}, ["加载中"]),
          ]);
        },
      }),
    ]),
    View({ class: "wx-logs-scroll" }, [
      Show({
        when: computed(
          vm$.state.loading,
          (loading) => loading && vm$.state.entries.value.length === 0,
        ),
        ok() {
          return LogsPageLoadingState();
        },
        else() {
          return Show({
            when: computed(vm$.state.entries, (entries) => entries.length > 0),
            ok() {
              return LogsPageTable({ store: vm$ });
            },
            else() {
              return LogsPageEmptyState();
            },
          });
        },
      }),
    ]),
  ]);
}

function LogsPageView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-page wx-logs-page",
      onMounted() {
        vm$.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      LogsPageHeader({ store: vm$ }),
      View({ class: "wx-content-toolbar-wrap" }, [
        LogsPageToolbar({ store: vm$ }),
      ]),
      View({ class: "wx-content-main" }, [
        LogsPageError({ store: vm$ }),
        LogsPageList({ store: vm$ }),
      ]),
    ],
  );
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = LogsPageModel();
    Timeless.DOM.render(LogsPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

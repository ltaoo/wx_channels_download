import {
  Button,
  Checkbox,
  Dialog,
  DialogBody,
  DialogTitle,
  Input,
  Pagination,
} from "../components.js";

import { LogsPageViewModel } from "./logs.model.js";

function LogsPageView(props) {
  const vm$ = LogsPageViewModel(props);

  return View(
    {
      class: "wx-content-page wx-logs-page dm-page",
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "wx-content-toolbar-wrap dm-container" }, [
        LogsPageToolbar({ store: vm$ }),
      ]),
      View({ class: "wx-content-main dm-container" }, [
        LogsPageError({ store: vm$ }),
        LogsPageList({ store: vm$ }),
      ]),
      Show({
        when: computed(vm$.state.entries, (entries) => entries.length > 0),
        ok() {
          return Pagination({
            summary: vm$.state.range_text,
            page: vm$.state.page,
            pageCount: vm$.state.page_count,
            loading: vm$.state.loading,
            onPrevious() {
              vm$.methods.previousPage();
            },
            onNext() {
              vm$.methods.nextPage();
            },
          });
        },
      }),
      LogsPageJsonPreviewDialog({ store: vm$ }),
    ],
  );
}

function LogsPageActionButton(props) {
  return Button(
    {
      store: props.store,
      class: [
        "wx-logs-page-button",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: props.type || "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
      prefix: props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
    },
    typeof props.label !== "undefined"
      ? [View({ class: "wx-content-action-label" }, [props.label])]
      : [],
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
  if (
    normalized === "debug" ||
    normalized === "warn" ||
    normalized === "error"
  ) {
    return `wx-logs-level wx-logs-level-${normalized}`;
  }
  return "wx-logs-level wx-logs-level-info";
}

function LogsPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-content-toolbar wx-logs-toolbar",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View({ class: "wx-logs-toolbar-row" }, [
        // Select({
        //   store: vm$.ui.select_source$,
        //   class:
        //     "wx-content-type-select wx-logs-select wx-logs-source-select",
        // }),
        // Select({
        //   store: vm$.ui.select_level$,
        //   class: "wx-content-type-select wx-logs-select",
        // }),
        View({ class: "wx-content-search wx-logs-search" }, [
          Timeless.Icon({ name: "search", size: 16 }),
          Input({
            store: vm$.ui.input_keyword$,
            class: "wx-content-search-input",
            attributes: {
              name: "keyword",
              type: "text",
              autocomplete: "off",
              "aria-label": "搜索消息、字段或原始日志",
            },
          }),
        ]),
        View({ class: "wx-content-filter-actions" }, [
          LogsPageActionButton({
            store: vm$.ui.btn_search$,
            icon: "search",
            label: "搜索",
            type: "submit",
            onClick(event) {
              event.preventDefault();
              vm$.methods.search();
            },
          }),
          LogsPageActionButton({
            store: vm$.ui.btn_reset$,
            icon: "rotate-ccw",
            label: "重置",
          }),
          LogsPageActionButton({
            store: vm$.ui.btn_export$,
            icon: "download",
            label: "导出",
            onClick() {
              vm$.methods.exportLogs();
            },
          }),
        ]),
      ]),
      View({ class: "wx-logs-toolbar-extra" }, [
        View({ class: "wx-logs-toggle" }, [
          Checkbox({
            store: vm$.ui.checkbox_auto_refresh$,
            id: "wxLogsAutoRefresh",
          }),
          View(
            {
              type: "label",
              attributes: { for: "wxLogsAutoRefresh" },
            },
            ["自动刷新"],
          ),
        ]),
        LogsPageActionButton({
          store: vm$.ui.btn_refresh$,
          icon: "refresh-cw",
          label: computed(vm$.state.loading, (loading) =>
            loading ? "刷新中" : "刷新",
          ),
          compact: true,
          onClick() {
            vm$.methods.refresh();
          },
        }),
        LogsPageActionButton({
          store: vm$.ui.btn_copy_log_file_path$,
          icon: "file",
          title: "复制日志文件绝对路径",
          attributes: { "aria-label": "复制日志文件绝对路径" },
          compact: true,
          onClick() {
            vm$.methods.copyLogFilePath();
          },
        }),
      ]),
    ],
  );
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
        const is_json = vm$.methods.isJsonFieldValue(row[1]);
        return View({ class: "wx-logs-field", attributes: { title: text } }, [
          View({ class: "wx-logs-field-key" }, [row[0]]),
          is_json
            ? View({ class: "wx-logs-field-json-actions" }, [
                View(
                  {
                    type: "button",
                    class: "wx-logs-field-value wx-logs-field-value-preview",
                    attributes: {
                      type: "button",
                      title: `点击查看 ${row[0]} 的格式化 JSON`,
                      "aria-label": `查看 ${row[0]} 的格式化 JSON`,
                    },
                    onClick() {
                      vm$.methods.showJsonFieldValue(row[0], row[1]);
                    },
                  },
                  [View({ class: "wx-logs-field-value-text" }, [row[1]])],
                ),
                View(
                  {
                    type: "button",
                    class: "wx-logs-field-copy-button",
                    attributes: {
                      type: "button",
                      title: `复制 ${row[0]} 的 JSON 值`,
                      "aria-label": `复制 ${row[0]} 的 JSON 值`,
                    },
                    onClick() {
                      vm$.methods.copyJsonFieldValue(row[1]);
                    },
                  },
                  [
                    View({ class: "wx-logs-field-copy-icon" }, [
                      Timeless.Icon({ name: "copy", size: 12 }),
                    ]),
                  ],
                ),
              ])
            : View({ class: "wx-logs-field-value" }, [row[1]]),
        ]);
      },
    }),
  ]);
}

function LogsPageJsonPreviewDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.json_preview_dialog$,
      class: "wx-logs-json-dialog",
      closeLabel: "关闭 JSON 预览",
    },
    [
      DialogTitle({ class: "wx-logs-json-dialog-header" }, [
        View({ class: "wx-logs-json-dialog-heading-icon" }, [
          Timeless.Icon({ name: "braces", size: 18 }),
        ]),
        View({ class: "wx-logs-json-dialog-heading" }, [
          View({ as: "span", class: "wx-logs-json-dialog-title" }, [
            vm$.state.json_preview_title,
          ]),
          View({ as: "span", class: "wx-logs-json-dialog-subtitle" }, [
            "格式化 JSON 内容",
          ]),
        ]),
      ]),
      DialogBody({ class: "wx-logs-json-dialog-body" }, [
        View({ type: "pre", class: "wx-logs-json-dialog-content" }, [
          vm$.state.json_preview_text,
        ]),
      ]),
    ],
  );
}

function LogsPageTableHead() {
  return View({ class: "wx-logs-table-head", attributes: { role: "row" } }, [
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["level"],
    ),
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["time"],
    ),
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["file"],
    ),
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["component"],
    ),
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["msg"],
    ),
    View(
      {
        class: "wx-logs-table-head-cell",
        attributes: { role: "columnheader" },
      },
      ["fields"],
    ),
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
        return View(
          { class: "wx-logs-table-row", attributes: { role: "row" } },
          [
            View({ class: "wx-logs-level-cell" }, [
              View({ class: "wx-logs-level wx-logs-level-info" }, ["info"]),
            ]),
            View({ class: "wx-logs-time-cell" }, [
              View({
                class: "wx-logs-skeleton-cell",
                style: { width: "150px" },
              }),
            ]),
            View({ class: "wx-logs-file-cell" }, [
              View({
                class: "wx-logs-skeleton-cell",
                style: { width: "80px" },
              }),
            ]),
            View({ class: "wx-logs-component-cell" }, [
              View({
                class: "wx-logs-skeleton-cell",
                style: { width: "100px" },
              }),
            ]),
            View({ class: "wx-logs-msg-cell" }, [
              View({
                class: "wx-logs-skeleton-cell",
                style: { width: "100%" },
              }),
            ]),
            View({ class: "wx-logs-fields-cell" }, [
              View({
                class: "wx-logs-skeleton-cell",
                style: { width: "80%" },
              }),
            ]),
          ],
        );
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
  return View(
    {
      class: "wx-content-rows wx-content-history-rows wx-logs-list dm-panel",
    },
    [
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
              when: computed(
                vm$.state.entries,
                (entries) => entries.length > 0,
              ),
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
    ],
  );
}

export default LogsPageView;

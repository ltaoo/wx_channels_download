import { LogsPageViewModel } from "./logs.model.js";

const logs_level_classes = {
  __proto__: null,
  "debug": "bg-content-panel-soft text-content-muted",
  "info": "bg-color-info-soft text-color-info",
  "warn": "bg-color-warning-soft text-color-warning",
  "error": "bg-color-danger-soft text-content-danger",
};

function LogsPageView(props) {
  const vm$ = LogsPageViewModel(props);

  return View(
    {
      class: "flex h-full min-h-0 min-w-0 flex-col overflow-hidden font-sans bg-content-bg text-content-text group-content-page bounded-list-page",
      attributes: { n: "logs-page" },
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View(
        {
          class: "pt-4 pb-0 px-4 group-content-toolbar-wrap ",
          attributes: { n: "logs-toolbar-container" },
        },
        [LogsPageToolbar({ store: vm$ })],
      ),
      LogsPageList({ store: vm$ }),
      LogsPageClearConfirm({ store: vm$ }),
      LogsPageImportDialog({ store: vm$ }),
      LogsPageJsonPreviewDialog({ store: vm$ }),
    ],
  );
}

function LogsPageClearConfirm(props) {
  const vm$ = props.store;
  return Confirm({
    store: vm$.ui.clear_logs_confirm_dialog$,
    class: "dm-dialog--sm",
    name: "logs-clear-confirm",
    title: "清空日志",
    description: "这会同时清空当前页面记录和日志文件内容，此操作不可恢复。",
    cancelText: "取消",
    okText: "清空",
  });
}

function LogsPageActionButton(props) {
  const semantic_name = props.name || "logs-action";
  return Button(
    {
      store: props.store,
      class: props.compact ? "" : "dm-button--toolbar",
      attributes: {
        n: semantic_name,
        type: props.type || "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
      prefix: props.icon
        ? Timeless.Icon({
            name: props.icon,
            size: props.iconSize || 16,
            attributes: { n: `${semantic_name}-icon` },
          })
        : null,
    },
    typeof props.label !== "undefined" ? [props.label] : [],
  );
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
  const tone = logs_level_classes[normalized] || logs_level_classes.info;
  return `w-fit-content min-w-46px font-sans text-center text-transform-uppercase ${tone}`;
}

function LogsPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "flex min-w-0 max-760:flex-wrap items-center justify-between gap-3 group-content-toolbar",
      attributes: { n: "logs-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class: "flex min-w-0 flex-auto max-760:flex-wrap items-center gap-2",
          attributes: { n: "logs-toolbar-primary-row" },
        },
        [
          // Select({
          //   store: vm$.ui.select_source$,
          //   class:
          //     "content-type-select logs-select logs-source-select",
          // }),
          View(
            {
              class: "w-120px grow-0 shrink-0 basis-30",
              attributes: { n: "logs-level-filter" },
            },
            [
              Select({
                store: vm$.ui.select_level$,
                attributes: {
                  n: "logs-level-select",
                  "aria-label": "筛选日志级别",
                },
              }),
            ],
          ),
          View(
            {
              class: "min-w-220px min-h-control-height-md grow shrink basis-80",
              attributes: { n: "logs-search-field" },
            },
            [
              Input({
                store: vm$.ui.input_keyword$,
                rootAttributes: { n: "logs-search-control" },
                prefix: Timeless.Icon({
                  name: "search",
                  size: 16,
                  attributes: { n: "logs-search-icon" },
                }),
                attributes: {
                  n: "logs-search-input",
                  name: "keyword",
                  type: "text",
                  autocomplete: "off",
                  "aria-label": "搜索消息、字段或原始日志",
                },
              }),
            ],
          ),
          View(
            {
              class: "max-520:w-full flex-none dm-items-center max-760:ml-auto max-520:ml-0 dm-gap-2 dm-flex group-content-filter-actions toolbar-actions",
              attributes: { n: "logs-primary-actions" },
            },
            [
              LogsPageActionButton({
                name: "logs-search-action",
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
                name: "logs-reset-action",
                store: vm$.ui.btn_reset$,
                icon: "rotate-ccw",
                label: "重置",
              }),
              View(
                {
                  class: "max-1280:!hidden contents",
                  attributes: { n: "logs-import-action-container" },
                },
                [
                  LogsPageActionButton({
                    name: "logs-import-action",
                    store: vm$.ui.btn_import$,
                    icon: "upload",
                    label: "导入",
                    onClick() {
                      vm$.methods.showImportDialog();
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "max-1280:!hidden contents",
                  attributes: { n: "logs-export-action-container" },
                },
                [
                  LogsPageActionButton({
                    name: "logs-export-action",
                    store: vm$.ui.btn_export$,
                    icon: "download",
                    label: "导出",
                    onClick() {
                      vm$.methods.exportLogs();
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "max-1280:!hidden contents",
                  attributes: { n: "logs-clear-action-container" },
                },
                [
                  LogsPageActionButton({
                    name: "logs-clear-action",
                    store: vm$.ui.btn_clear_logs$,
                    icon: "trash2",
                    label: computed(vm$.state.clearing, (clearing) =>
                      clearing ? "清空中" : "清空",
                    ),
                    onClick() {
                      vm$.methods.clearLogs();
                    },
                  }),
                ],
              ),
            ],
          ),
        ],
      ),
      View(
        {
          class: "flex max-760:w-full flex-none max-760:flex-wrap items-center max-760:justify-between gap-2",
          attributes: { n: "logs-toolbar-secondary-row" },
        },
        [
          Show({
            when: vm$.state.imported,
            ok() {
              return View(
                {
                  class: "flex min-w-0 items-center gap-2 text-sm text-content-muted status-icon-primary ",
                  attributes: {
                    n: "logs-import-status",
                    role: "status",
                    title: vm$.state.imported_file_name,
                  },
                },
                [
                  Timeless.Icon({
                    name: "file",
                    size: 14,
                    attributes: { n: "logs-import-status-icon" },
                  }),
                  View(
                    {
                      as: "span",
                      class: "max-w-220px overflow-hidden whitespace-nowrap text-ellipsis",
                      attributes: { n: "logs-import-status-text" },
                    },
                    [vm$.state.imported_label],
                  ),
                  LogsPageActionButton({
                    name: "logs-restore-server-action",
                    store: vm$.ui.btn_restore_server_logs$,
                    label: "恢复服务日志",
                    compact: true,
                    onClick() {
                      vm$.methods.restoreServerLogs();
                    },
                  }),
                ],
              );
            },
          }),
          View(
            {
              class: "flex max-1280:!hidden min-h-control-height-md items-center py-0 px-2.5 gap-2 text-sm whitespace-nowrap text-content-muted",
              attributes: { n: "logs-auto-refresh-control" },
            },
            [
              Checkbox({
                store: vm$.ui.checkbox_auto_refresh$,
                id: "wxLogsAutoRefresh",
                text: "自动刷新",
                textAttributes: { n: "logs-auto-refresh-text" },
                attributes: { n: "logs-auto-refresh-checkbox" },
              }),
            ],
          ),
          View(
            {
              class: "max-1280:!hidden contents",
              attributes: { n: "logs-refresh-action-container" },
            },
            [
              LogsPageActionButton({
                name: "logs-refresh-action",
                store: vm$.ui.btn_refresh$,
                icon: "refresh-cw",
                label: computed(vm$.state.loading, (loading) =>
                  loading ? "刷新中" : "刷新",
                ),
                compact: true,
              }),
            ],
          ),
          View(
            {
              class: "max-1280:!hidden contents",
              attributes: { n: "logs-copy-file-path-action-container" },
            },
            [
              LogsPageActionButton({
                name: "logs-copy-file-path-action",
                store: vm$.ui.btn_copy_log_file_path$,
                icon: "file",
                title: "复制日志文件绝对路径",
                attributes: { "aria-label": "复制日志文件绝对路径" },
                compact: true,
                onClick() {
                  vm$.methods.copyLogFilePath();
                },
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

function LogsPageImportDialog(props) {
  const vm$ = props.store;

  return Dialog(
    {
      store: vm$.ui.import_dialog$,
      class: "dm-dialog--md",
      closeLabel: "关闭日志导入",
      attributes: { n: "logs-import-dialog" },
    },
    () => [
      DialogTitle(
        {
          class: "flex flex-none items-center py-4 pr-12 pl-5 gap-3 border-b-default border-b-solid border-b-color-content",
          attributes: { n: "logs-import-dialog-header" },
        },
        [
          View(
            {
              class: "grid w-control-height-base h-control-height-base flex-none place-items-center border-default border-solid border-color-primary-border rounded-content bg-color-primary-soft text-color-primary",
              attributes: { n: "logs-import-dialog-heading-icon" },
            },
            [
              Timeless.Icon({
                name: "upload",
                size: 18,
                attributes: { n: "logs-import-dialog-upload-icon" },
              }),
            ],
          ),
          View(
            {
              class: "flex min-w-0 flex-col gap-0.5",
              attributes: { n: "logs-import-dialog-heading" },
            },
            [
              View(
                {
                  as: "span",
                  class: "text-lg font-bold text-content-text",
                  attributes: { n: "logs-import-dialog-title" },
                },
                ["导入外部日志"],
              ),
              View(
                {
                  as: "span",
                  class: "text-sm font-regular text-content-muted",
                  attributes: { n: "logs-import-dialog-subtitle" },
                },
                ["导入成功后将替换当前显示的日志记录"],
              ),
            ],
          ),
        ],
      ),
      DialogBody(
        {
          class: "p-5",
          attributes: { n: "logs-import-dialog-body" },
        },
        [
          FilePicker({
            store: vm$.ui.log_file_picker$,
            name: "logs-import-file-picker",
            accept: vm$.ui.log_file_picker$.accept,
            dragging: vm$.state.import_dragging,
            invalid: vm$.state.import_drag_invalid,
            loading: vm$.state.importing,
            title: computed(vm$.state.importing, (importing) =>
              importing ? "正在导入日志…" : "拖拽日志文件到此处",
            ),
            hint: "或点击此区域选择文件",
            formats: "支持 .log、.txt、.json、.jsonl、.ndjson",
            inputLabel: "选择要导入的日志文件",
            dropZoneLabel: "点击选择或拖拽上传日志文件",
            onChange(event) {
              vm$.methods.handleLogFilePickerChange(event);
            },
            onKeyDown(event) {
              vm$.methods.handleImportDropZoneKeyDown(event);
            },
          }),
        ],
      ),
    ],
  );
}

function LogsPageTableColumns(props) {
  const vm$ = props.store;
  return [
    {
      name: "level",
      title: "level",
      width: 76,
      cellClass: "min-w-0",
      render(entry) {
        return Tag(
          {
            class: LogsPageLevelClass(entry.level),
            attributes: { n: "logs-level-badge" },
          },
          [entry.level || "info"],
        );
      },
    },
    {
      name: "time",
      title: "time",
      width: 166,
      cellClass: "text-content-muted",
      render(entry) {
        return LogsPageFormatTime(entry.time);
      },
    },
    {
      name: "file",
      title: "file",
      width: 120,
      cellClass: "text-content-muted",
      cellAttributes(entry) {
        return { title: entry.file || entry.File || "" };
      },
      render(entry) {
        return entry.file || entry.File || "";
      },
    },
    {
      name: "component",
      title: "component",
      width: 132,
      cellClass: "text-content-muted",
      cellAttributes(entry) {
        return { title: entry.component || entry.Component || "" };
      },
      render(entry) {
        return entry.component || entry.Component || "";
      },
    },
    {
      name: "message",
      title: "msg",
      width: "minmax(260px, 1.2fr)",
      cellClass: "min-w-0 overflow-hidden whitespace-nowrap text-ellipsis",
      render(entry) {
        return entry.message || entry.raw || "";
      },
    },
    {
      name: "fields",
      title: "fields",
      width: "minmax(280px, 1fr)",
      cellClass: "min-w-0",
      render(entry) {
        return LogsPageFieldsCell({ entry, store: vm$ });
      },
    },
  ];
}

function LogsPageFieldsCell(props) {
  const vm$ = props.store;
  const fields = vm$.methods.fieldRows(props.entry);
  if (fields.length === 0) {
    return View(
      {
        class: "text-content-muted",
        attributes: { n: "logs-fields-empty" },
      },
      ["-"],
    );
  }
  return View(
    {
      class: "flex flex-wrap gap-1",
      attributes: { n: "logs-fields" },
    },
    [
      For({
        each: fields,
        render(row) {
          const text = `${row[0]}=${row[1]}`;
          const is_json = vm$.methods.isJsonFieldValue(row[1]);
          return View(
            {
              class: "inline-flex min-w-0 grow-0 shrink basis-auto items-center overflow-hidden border-default border-solid border-color-content rounded-content bg-content-panel-soft text-content-muted",
              attributes: { n: "logs-field", title: text },
            },
            [
              View(
                {
                  class: "py-1 px-1.5 bg-color-primary-soft text-color-primary",
                  attributes: { n: "logs-field-key" },
                },
                [row[0]],
              ),
              is_json
                ? View(
                    {
                      class: "flex min-w-0",
                      attributes: { n: "logs-json-field-actions" },
                    },
                    [
                      View(
                        {
                          type: "button",
                          class: "min-w-0 max-w-180px py-1 px-1.5 overflow-hidden whitespace-nowrap text-ellipsis border-0 bg-transparent hover:bg-color-primary-soft text-content-muted hover:text-color-primary",
                          attributes: {
                            n: "logs-json-field-preview-action",
                            type: "button",
                            title: `点击查看 ${row[0]} 的格式化 JSON`,
                            "aria-label": `查看 ${row[0]} 的格式化 JSON`,
                          },
                          onClick() {
                            vm$.methods.showJsonFieldValue(row[0], row[1]);
                          },
                        },
                        [
                          View(
                            {
                              class: "overflow-hidden whitespace-nowrap text-ellipsis",
                              attributes: { n: "logs-json-field-value" },
                            },
                            [row[1]],
                          ),
                        ],
                      ),
                      View(
                        {
                          type: "button",
                          class: "grid place-items-center p-1 border-0 border-l-default border-l-solid border-l-color-content bg-transparent hover:bg-color-primary-soft text-content-muted hover:text-color-primary cursor-pointer",
                          attributes: {
                            n: "logs-json-field-copy-action",
                            type: "button",
                            title: `复制 ${row[0]} 的 JSON 值`,
                            "aria-label": `复制 ${row[0]} 的 JSON 值`,
                          },
                          onClick() {
                            vm$.methods.copyJsonFieldValue(row[1]);
                          },
                        },
                        [
                          View(
                            {
                              class: "grid place-items-center",
                              attributes: { n: "logs-json-field-copy-icon" },
                            },
                            [
                              Timeless.Icon({
                                name: "copy",
                                size: 12,
                                attributes: { n: "logs-json-field-copy-svg" },
                              }),
                            ],
                          ),
                        ],
                      ),
                    ],
                  )
                : View(
                    {
                      class: "min-w-0 max-w-180px py-1 px-1.5 overflow-hidden whitespace-nowrap text-ellipsis border-0 bg-transparent text-content-muted",
                      attributes: { n: "logs-field-value" },
                    },
                    [row[1]],
                  ),
            ],
          );
        },
      }),
    ],
  );
}

function LogsPageJsonPreviewDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.json_preview_dialog$,
      closeLabel: "关闭 JSON 预览",
      attributes: { n: "logs-json-preview-dialog" },
    },
    () => [
      DialogTitle(
        {
          class: "flex flex-none items-center py-4 pr-12 pl-5 gap-3 border-b-default border-b-solid border-b-color-content",
          attributes: { n: "logs-json-preview-header" },
        },
        [
          View(
            {
              class: "grid w-control-height-base h-control-height-base flex-none place-items-center border-default border-solid border-color-primary-border rounded-content bg-color-primary-soft text-color-primary",
              attributes: { n: "logs-json-preview-heading-icon" },
            },
            [
              Timeless.Icon({
                name: "braces",
                size: 18,
                attributes: { n: "logs-json-preview-braces-icon" },
              }),
            ],
          ),
          View(
            {
              class: "flex min-w-0 flex-col gap-0.5",
              attributes: { n: "logs-json-preview-heading" },
            },
            [
              View(
                {
                  as: "span",
                  class: "text-lg font-bold",
                  attributes: { n: "logs-json-preview-title" },
                },
                [vm$.state.json_preview_title],
              ),
              View(
                {
                  as: "span",
                  class: "text-sm font-regular text-content-muted",
                  attributes: { n: "logs-json-preview-subtitle" },
                },
                ["格式化 JSON 内容"],
              ),
            ],
          ),
        ],
      ),
      DialogBody(
        {
          class: "max-h-code-pane p-4 overflow-auto bg-color-media-canvas",
          attributes: { n: "logs-json-preview-body" },
        },
        [
          View(
            {
              type: "pre",
              class: "m-0 font-mono text-sm leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere text-color-media-text",
              attributes: { n: "logs-json-preview-content" },
            },
            [vm$.state.json_preview_text],
          ),
        ],
      ),
    ],
  );
}

function LogsPageSkeletonRow() {
  return View(
    {
      class:
        "dm-grid grid min-h-48px dm-items-center items-center border-b-default border-b-solid border-b-color-content bg-content-panel hover:bg-content-panel-soft pointer-events-none dm-table-row table-row-ellipsis",
      attributes: { n: "logs-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "min-w-0 dm-table-cell",
          attributes: { n: "logs-skeleton-level-cell", role: "cell" },
        },
        [
          View(
            {
              class: "w-fit-content min-w-46px font-sans text-center text-transform-uppercase bg-color-info-soft text-color-info",
              attributes: { n: "logs-skeleton-level-badge" },
            },
            ["info"],
          ),
        ],
      ),
      View(
        {
          class: "text-content-muted dm-table-cell",
          attributes: { n: "logs-skeleton-time-cell", role: "cell" },
        },
        [
          View({
            class: "relative w-150px overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "logs-skeleton-time-value" },
          }),
        ],
      ),
      View(
        {
          class: "text-content-muted dm-table-cell",
          attributes: { n: "logs-skeleton-file-cell", role: "cell" },
        },
        [
          View({
            class: "relative w-80px overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "logs-skeleton-file-value" },
          }),
        ],
      ),
      View(
        {
          class: "text-content-muted dm-table-cell",
          attributes: { n: "logs-skeleton-component-cell", role: "cell" },
        },
        [
          View({
            class: "relative w-100px overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "logs-skeleton-component-value" },
          }),
        ],
      ),
      View(
        {
          class: "min-w-0 overflow-hidden whitespace-nowrap text-ellipsis dm-table-cell",
          attributes: { n: "logs-skeleton-message-cell", role: "cell" },
        },
        [
          View({
            class: "relative w-full overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "logs-skeleton-message-value" },
          }),
        ],
      ),
      View(
        {
          class: "min-w-0 dm-table-cell",
          attributes: { n: "logs-skeleton-fields-cell", role: "cell" },
        },
        [
          View({
            class: "relative w-4/5 overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "logs-skeleton-fields-value" },
          }),
        ],
      ),
    ],
  );
}

function LogsPageList(props) {
  const vm$ = props.store;
  return TableWithVirtualList({
    name: "logs-table",
    containerClass: "min-w-0 min-h-0 flex-auto p-4 overflow-x-auto overflow-y-hidden group-content-main",
    containerAttributes: { n: "logs-table-container" },
    panelClass: "flex min-h-0 max-h-full flex-auto flex-col overflow-hidden font-mono text-xs text-content-text",
    panelAttributes: { n: "logs-list-panel" },
    headerClass: "grid sticky top-0 z-raised min-h-control-height-base flex-none items-center font-sans text-caption font-semibold tracking-0.06em text-transform-uppercase border-b-default border-b-solid border-b-color-content bg-content-panel-soft text-content-muted",
    headerCellClass: "min-w-0 overflow-hidden whitespace-nowrap text-ellipsis",
    listClass: "min-h-0 h-auto flex-auto overflow-y-auto",
    listStyle: { "overflow-x": "hidden" },
    columns: LogsPageTableColumns({ store: vm$ }),
    rows: vm$.state.entries,
    rowKey(entry, index) {
      return `${entry.index || index}-${entry.time || ""}-${entry.message || ""}`;
    },
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "grid min-h-48px items-center border-b-default border-b-solid border-b-color-content bg-content-panel hover:bg-content-panel-soft table-row-ellipsis ",
    skeletonCount: 6,
    renderSkeletonRow: LogsPageSkeletonRow,
    size: 18,
    buffer: 8,
    gutter: 0,
    itemHeight: 48,
    onListScroll(position) {
      vm$.methods.handleListScroll(position);
    },
    errorTitle: "日志加载失败",
    retry: { store: vm$.ui.btn_refresh$ },
    emptyIcon() {
      return Timeless.Icon({
        name: "file-search",
        size: 32,
        attributes: { n: "logs-empty-state-icon" },
      });
    },
    emptyTitle: "没有匹配的日志",
  });
}

export default LogsPageView;

import { LogsPageViewModel } from "./logs.model.js";

const logs_level_classes = {
  __proto__: null,
  "debug": "text-content-muted bg-content-panel-soft",
  "info": "text-color-info bg-color-info-soft",
  "warn": "text-color-warning bg-color-warning-soft",
  "error": "text-content-danger bg-color-danger-soft",
};

function LogsPageView(props) {
  const vm$ = LogsPageViewModel(props);

  return View(
    {
      class: "group-content-page flex flex-col bg-content-bg text-content-text h-full min-h-0 overflow-hidden [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-[1_1_auto] [&_>_.group-content-main]:flex-col min-w-0 font-sans",
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
          class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0] ",
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
  return `w-fit-content min-w-46px text-center font-sans text-transform-uppercase ${tone}`;
}

function LogsPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "group-content-toolbar flex items-center justify-between gap-3 max-760:flex-wrap min-w-0",
      attributes: { n: "logs-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class: "flex items-center gap-2 min-w-0 flex-[1_1_auto] max-760:flex-wrap",
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
              class: "w-120px flex-[0_0_120px]",
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
              class: "min-w-220px flex-[1_1_320px] min-h-control-height-md",
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
              class: "group-content-filter-actions flex-[0_0_auto] max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2",
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
          class: "flex items-center gap-2 flex-[0_0_auto] max-760:flex-wrap max-760:w-full max-760:justify-between",
          attributes: { n: "logs-toolbar-secondary-row" },
        },
        [
          Show({
            when: vm$.state.imported,
            ok() {
              return View(
                {
                  class: "min-w-0 flex items-center gap-2 text-content-muted text-sm [&_>_svg]:flex-[0_0_auto] [&_>_svg]:text-color-primary",
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
                      class: "max-w-220px overflow-hidden text-ellipsis whitespace-nowrap",
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
              class: "flex items-center min-h-control-height-md gap-2 p-[0_var(--dm-space-2-5)] text-content-muted text-sm whitespace-nowrap max-1280:!hidden",
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
          class: "flex items-center gap-3 flex-[0_0_auto] p-[var(--dm-space-4)_var(--dm-space-12)_var(--dm-space-4)_var(--dm-space-5)] border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)]",
          attributes: { n: "logs-import-dialog-header" },
        },
        [
          View(
            {
              class: "grid w-control-height-base h-control-height-base flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content",
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
              class: "min-w-0 flex flex-col gap-0-5",
              attributes: { n: "logs-import-dialog-heading" },
            },
            [
              View(
                {
                  as: "span",
                  class: "text-content-text text-lg font-bold",
                  attributes: { n: "logs-import-dialog-title" },
                },
                ["导入外部日志"],
              ),
              View(
                {
                  as: "span",
                  class: "text-content-muted text-sm font-regular",
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
      cellClass: "min-w-0 overflow-hidden text-ellipsis whitespace-nowrap",
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
              class: "min-w-0 inline-flex flex-[0_1_auto] items-center overflow-hidden text-content-muted bg-content-panel-soft border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-content",
              attributes: { n: "logs-field", title: text },
            },
            [
              View(
                {
                  class: "p-[var(--dm-space-1)_var(--dm-space-1-5)] text-color-primary bg-color-primary-soft",
                  attributes: { n: "logs-field-key" },
                },
                [row[0]],
              ),
              is_json
                ? View(
                    {
                      class: "min-w-0 flex",
                      attributes: { n: "logs-json-field-actions" },
                    },
                    [
                      View(
                        {
                          type: "button",
                          class: "min-w-0 max-w-180px p-[var(--dm-space-1)_var(--dm-space-1-5)] overflow-hidden text-content-muted text-ellipsis whitespace-nowrap bg-transparent border-0 min-w-0 max-w-180px p-[var(--dm-space-1)_var(--dm-space-1-5)] overflow-hidden text-content-muted text-ellipsis whitespace-nowrap bg-transparent border-0 hover:text-color-primary hover:bg-color-primary-soft",
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
                              class: "overflow-hidden text-ellipsis whitespace-nowrap",
                              attributes: { n: "logs-json-field-value" },
                            },
                            [row[1]],
                          ),
                        ],
                      ),
                      View(
                        {
                          type: "button",
                          class: "grid place-items-center p-1 text-content-muted bg-transparent border-0 border-left-[var(--dm-border-width)_solid_var(--dm-content-border)] cursor-pointer hover:text-color-primary hover:bg-color-primary-soft",
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
                      class: "min-w-0 max-w-180px p-[var(--dm-space-1)_var(--dm-space-1-5)] overflow-hidden text-content-muted text-ellipsis whitespace-nowrap bg-transparent border-0",
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
          class: "flex items-center gap-3 flex-[0_0_auto] p-[var(--dm-space-4)_var(--dm-space-12)_var(--dm-space-4)_var(--dm-space-5)] border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)]",
          attributes: { n: "logs-json-preview-header" },
        },
        [
          View(
            {
              class: "grid w-control-height-base h-control-height-base flex-[0_0_auto] place-items-center text-color-primary bg-color-primary-soft border-[var(--dm-border-width)_solid_var(--dm-color-primary-border)] rounded-content",
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
              class: "min-w-0 flex flex-col gap-0-5",
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
                  class: "text-content-muted text-sm font-regular",
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
          class: "max-h-[min(66vh,_640px)] p-4 overflow-auto bg-color-media-canvas",
          attributes: { n: "logs-json-preview-body" },
        },
        [
          View(
            {
              type: "pre",
              class: "m-0 text-color-media-text font-mono text-sm leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere",
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
        "dm-table-row dm-grid dm-items-center grid items-center min-h-48px bg-content-panel border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)] hover:bg-content-panel-soft [&_>_[role=\"cell\"]]:min-w-0 [&_>_[role=\"cell\"]]:overflow-hidden [&_>_[role=\"cell\"]]:text-ellipsis [&_>_[role=\"cell\"]]:whitespace-nowrap pointer-events-none",
      attributes: { n: "logs-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "dm-table-cell min-w-0",
          attributes: { n: "logs-skeleton-level-cell", role: "cell" },
        },
        [
          View(
            {
              class: "w-fit-content min-w-46px text-color-info text-center bg-color-info-soft font-sans text-transform-uppercase text-color-info bg-color-info-soft",
              attributes: { n: "logs-skeleton-level-badge" },
            },
            ["info"],
          ),
        ],
      ),
      View(
        {
          class: "dm-table-cell text-content-muted",
          attributes: { n: "logs-skeleton-time-cell", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-content [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-150px",
            attributes: { n: "logs-skeleton-time-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell text-content-muted",
          attributes: { n: "logs-skeleton-file-cell", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-content [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-80px",
            attributes: { n: "logs-skeleton-file-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell text-content-muted",
          attributes: { n: "logs-skeleton-component-cell", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-content [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-100px",
            attributes: { n: "logs-skeleton-component-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell min-w-0 overflow-hidden text-ellipsis whitespace-nowrap",
          attributes: { n: "logs-skeleton-message-cell", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-content [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-full",
            attributes: { n: "logs-skeleton-message-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell min-w-0",
          attributes: { n: "logs-skeleton-fields-cell", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-content [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-[80%]",
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
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4  min-w-0 min-h-0 flex-[1_1_auto] overflow-x-auto overflow-y-hidden",
    containerAttributes: { n: "logs-table-container" },
    panelClass: "min-h-0 flex flex-[1_1_auto] flex-col overflow-hidden max-h-full flex flex-col text-content-text font-mono text-xs",
    panelAttributes: { n: "logs-list-panel" },
    headerClass: "grid items-center sticky top-0 z-raised min-h-control-height-base flex-[0_0_auto] text-content-muted bg-content-panel-soft border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)] font-sans text-caption font-semibold tracking-[0.06em] text-transform-uppercase",
    headerCellClass: "min-w-0 overflow-hidden text-ellipsis whitespace-nowrap",
    listClass: "min-h-0 h-auto flex-[1_1_auto] overflow-y-auto",
    listStyle: { "overflow-x": "hidden" },
    columns: LogsPageTableColumns({ store: vm$ }),
    rows: vm$.state.entries,
    rowKey(entry, index) {
      return `${entry.index || index}-${entry.time || ""}-${entry.message || ""}`;
    },
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "grid items-center min-h-48px bg-content-panel border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)] hover:bg-content-panel-soft [&_>_[role=\"cell\"]]:min-w-0 [&_>_[role=\"cell\"]]:overflow-hidden [&_>_[role=\"cell\"]]:text-ellipsis [&_>_[role=\"cell\"]]:whitespace-nowrap",
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

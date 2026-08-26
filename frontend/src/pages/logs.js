import { LogsPageViewModel } from "./logs.model.js";

function LogsPageView(props) {
  const vm$ = LogsPageViewModel(props);

  return View(
    {
      class: "wx-content-page wx-logs-page dm-page",
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
          class: "wx-content-toolbar-wrap dm-container",
          attributes: { n: "logs-toolbar-container" },
        },
        [LogsPageToolbar({ store: vm$ })],
      ),
      View(
        {
          class: "wx-content-main dm-container",
          attributes: { n: "logs-content" },
        },
        [LogsPageError({ store: vm$ }), LogsPageList({ store: vm$ })],
      ),
      Show({
        when: computed(vm$.state.entries, (entries) => entries.length > 0),
        ok() {
          return Pagination({
            summary: vm$.state.range_text,
            page: vm$.state.page,
            pageCount: vm$.state.page_count,
            loading: vm$.state.loading,
            attributes: { n: "logs-pagination" },
            onPrevious() {
              vm$.methods.previousPage();
            },
            onNext() {
              vm$.methods.nextPage();
            },
          });
        },
      }),
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
    name: "logs-clear-confirm",
    title: "清空日志",
    description:
      "这会同时清空当前页面记录和日志文件内容，此操作不可恢复。",
    cancelText: "取消",
    okText: "清空",
    style: { width: "min(440px, calc(100vw - 32px))" },
  });
}

function LogsPageActionButton(props) {
  const semantic_name = props.name || "logs-action";
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
    typeof props.label !== "undefined"
      ? [
          View(
            {
              class: "wx-content-action-label",
              attributes: { n: `${semantic_name}-label` },
            },
            [props.label],
          ),
        ]
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
      attributes: { n: "logs-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class: "wx-logs-toolbar-row",
          attributes: { n: "logs-toolbar-primary-row" },
        },
        [
          // Select({
          //   store: vm$.ui.select_source$,
          //   class:
          //     "wx-content-type-select wx-logs-select wx-logs-source-select",
          // }),
          Select({
            store: vm$.ui.select_level$,
            rootClass: "wx-logs-level-select",
            class: "wx-content-type-select wx-logs-select",
            attributes: {
              n: "logs-level-select",
              "aria-label": "筛选日志级别",
            },
          }),
          View(
            {
              class: "wx-content-search wx-logs-search",
              attributes: { n: "logs-search-field" },
            },
            [
              Timeless.Icon({
                name: "search",
                size: 16,
                attributes: { n: "logs-search-icon" },
              }),
              Input({
                store: vm$.ui.input_keyword$,
                class: "wx-content-search-input",
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
              class: "wx-content-filter-actions",
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
              LogsPageActionButton({
                name: "logs-import-action",
                store: vm$.ui.btn_import$,
                icon: "upload",
                label: "导入",
                onClick() {
                  vm$.methods.showImportDialog();
                },
              }),
              LogsPageActionButton({
                name: "logs-export-action",
                store: vm$.ui.btn_export$,
                icon: "download",
                label: "导出",
                onClick() {
                  vm$.methods.exportLogs();
                },
              }),
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
      View(
        {
          class: "wx-logs-toolbar-extra",
          attributes: { n: "logs-toolbar-secondary-row" },
        },
        [
          Show({
            when: vm$.state.imported,
            ok() {
              return View(
                {
                  class: "wx-logs-import-status",
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
                      class: "wx-logs-import-status-text",
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
              class: "wx-logs-toggle",
              attributes: { n: "logs-auto-refresh-control" },
            },
            [
              Checkbox({
                store: vm$.ui.checkbox_auto_refresh$,
                id: "wxLogsAutoRefresh",
                attributes: { n: "logs-auto-refresh-checkbox" },
              }),
              View(
                {
                  type: "label",
                  attributes: {
                    n: "logs-auto-refresh-label",
                    for: "wxLogsAutoRefresh",
                  },
                },
                ["自动刷新"],
              ),
            ],
          ),
          LogsPageActionButton({
            name: "logs-refresh-action",
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
  );
}

function LogsPageImportDialog(props) {
  const vm$ = props.store;
  const file_picker = Timeless.ui.FilePickerPrimitive;
  const drop_zone_class = combine(
    {
      dragging: vm$.state.import_dragging,
      invalid: vm$.state.import_drag_invalid,
      importing: vm$.state.importing,
    },
    (state) =>
      [
        "wx-logs-import-drop-zone",
        state.dragging ? "is-dragging" : "",
        state.invalid ? "is-invalid" : "",
        state.importing ? "is-importing" : "",
      ]
        .filter(Boolean)
        .join(" "),
  );

  return Dialog(
    {
      store: vm$.ui.import_dialog$,
      class: "wx-logs-import-dialog",
      closeLabel: "关闭日志导入",
      attributes: { n: "logs-import-dialog" },
    },
    () => [
      DialogTitle(
        {
          class: "wx-logs-import-dialog-header",
          attributes: { n: "logs-import-dialog-header" },
        },
        [
          View(
            {
              class: "wx-logs-import-dialog-heading-icon",
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
              class: "wx-logs-import-dialog-heading",
              attributes: { n: "logs-import-dialog-heading" },
            },
            [
              View(
                {
                  as: "span",
                  class: "wx-logs-import-dialog-title",
                  attributes: { n: "logs-import-dialog-title" },
                },
                ["导入外部日志"],
              ),
              View(
                {
                  as: "span",
                  class: "wx-logs-import-dialog-subtitle",
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
          class: "wx-logs-import-dialog-body",
          attributes: { n: "logs-import-dialog-body" },
        },
        [
          file_picker.Root(
            {
              store: vm$.ui.log_file_picker$,
              class: "wx-logs-import-file-picker",
              attributes: { n: "logs-import-file-picker" },
            },
            [
              file_picker.Input({
                store: vm$.ui.log_file_picker$,
                accept: vm$.ui.log_file_picker$.accept,
                multiple: false,
                class: "wx-logs-import-file-input",
                attributes: {
                  n: "logs-import-file-input",
                  "aria-label": "选择要导入的日志文件",
                },
                onChange(event) {
                  vm$.methods.handleLogFilePickerChange(event);
                },
              }),
              file_picker.DropZone(
                {
                  store: vm$.ui.log_file_picker$,
                  class: drop_zone_class,
                  attributes: {
                    n: "logs-import-drop-zone",
                    role: "button",
                    tabindex: "0",
                    "aria-label": "点击选择或拖拽上传日志文件",
                  },
                  onKeyDown(event) {
                    vm$.methods.handleImportDropZoneKeyDown(event);
                  },
                },
                [
                  View(
                    {
                      class: "wx-logs-import-drop-zone-icon",
                      attributes: { n: "logs-import-drop-zone-icon" },
                    },
                    [
                      Timeless.Icon({
                        name: "upload",
                        size: 24,
                        attributes: { n: "logs-import-drop-zone-upload-icon" },
                      }),
                    ],
                  ),
                  View(
                    {
                      class: "wx-logs-import-drop-zone-title",
                      attributes: { n: "logs-import-drop-zone-title" },
                    },
                    [
                      computed(vm$.state.importing, (importing) =>
                        importing ? "正在导入日志…" : "拖拽日志文件到此处",
                      ),
                    ],
                  ),
                  View(
                    {
                      class: "wx-logs-import-drop-zone-hint",
                      attributes: { n: "logs-import-drop-zone-hint" },
                    },
                    ["或点击此区域选择文件"],
                  ),
                  View(
                    {
                      class: "wx-logs-import-drop-zone-formats",
                      attributes: { n: "logs-import-drop-zone-formats" },
                    },
                    ["支持 .log、.txt、.json、.jsonl、.ndjson"],
                  ),
                  Show({
                    when: computed(vm$.state.import_file_error, (message) =>
                      Boolean(message),
                    ),
                    ok() {
                      return View(
                        {
                          class: "wx-logs-import-drop-zone-error",
                          attributes: {
                            n: "logs-import-drop-zone-error",
                            role: "alert",
                          },
                        },
                        [
                          Timeless.Icon({
                            name: "circle-alert",
                            size: 14,
                            attributes: {
                              n: "logs-import-drop-zone-error-icon",
                            },
                          }),
                          View(
                            {
                              as: "span",
                              attributes: {
                                n: "logs-import-drop-zone-error-message",
                              },
                            },
                            [vm$.state.import_file_error],
                          ),
                        ],
                      );
                    },
                  }),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function LogsPageError(props) {
  const vm$ = props.store;
  return Show({
    when: computed(vm$.state.error, (message) => Boolean(message)),
    ok() {
      return View(
        {
          class: "wx-logs-error",
          attributes: { n: "logs-error-banner" },
        },
        [
          Timeless.Icon({
            name: "circle-alert",
            size: 18,
            attributes: { n: "logs-error-icon" },
          }),
          View({ attributes: { n: "logs-error-message" } }, [vm$.state.error]),
        ],
      );
    },
  });
}

function LogsPageRow(props) {
  const vm$ = props.store;
  const entry = LogsPageEntryValue(props.entry) || {};
  const file = entry.file || entry.File || "";
  const component = entry.component || entry.Component || "";
  return View(
    {
      class: "wx-logs-table-row",
      attributes: { n: "logs-table-row", role: "row" },
    },
    [
      View(
        {
          class: "wx-logs-level-cell",
          attributes: { n: "logs-level-cell", role: "cell" },
        },
        [
          View(
            {
              class: LogsPageLevelClass(entry.level),
              attributes: { n: "logs-level-badge" },
            },
            [entry.level || "info"],
          ),
        ],
      ),
      View(
        {
          class: "wx-logs-time-cell",
          attributes: { n: "logs-time-cell", role: "cell" },
        },
        [LogsPageFormatTime(entry.time)],
      ),
      View(
        {
          class: "wx-logs-file-cell",
          attributes: { n: "logs-file-cell", role: "cell", title: file },
        },
        [file],
      ),
      View(
        {
          class: "wx-logs-component-cell",
          attributes: {
            n: "logs-component-cell",
            role: "cell",
            title: component,
          },
        },
        [component],
      ),
      View(
        {
          class: "wx-logs-msg-cell",
          attributes: { n: "logs-message-cell", role: "cell" },
        },
        [entry.message || entry.raw || ""],
      ),
      View(
        {
          class: "wx-logs-fields-cell",
          attributes: { n: "logs-fields-cell", role: "cell" },
        },
        [LogsPageFieldsCell({ entry, store: vm$ })],
      ),
    ],
  );
}

function LogsPageFieldsCell(props) {
  const vm$ = props.store;
  const fields = vm$.methods.fieldRows(props.entry);
  if (fields.length === 0) {
    return View(
      {
        class: "wx-logs-fields-empty",
        attributes: { n: "logs-fields-empty" },
      },
      ["-"],
    );
  }
  return View(
    {
      class: "wx-logs-fields",
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
              class: "wx-logs-field",
              attributes: { n: "logs-field", title: text },
            },
            [
              View(
                {
                  class: "wx-logs-field-key",
                  attributes: { n: "logs-field-key" },
                },
                [row[0]],
              ),
              is_json
                ? View(
                    {
                      class: "wx-logs-field-json-actions",
                      attributes: { n: "logs-json-field-actions" },
                    },
                    [
                      View(
                        {
                          type: "button",
                          class:
                            "wx-logs-field-value wx-logs-field-value-preview",
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
                              class: "wx-logs-field-value-text",
                              attributes: { n: "logs-json-field-value" },
                            },
                            [row[1]],
                          ),
                        ],
                      ),
                      View(
                        {
                          type: "button",
                          class: "wx-logs-field-copy-button",
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
                              class: "wx-logs-field-copy-icon",
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
                      class: "wx-logs-field-value",
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
      class: "wx-logs-json-dialog",
      closeLabel: "关闭 JSON 预览",
      attributes: { n: "logs-json-preview-dialog" },
    },
    () => [
      DialogTitle(
        {
          class: "wx-logs-json-dialog-header",
          attributes: { n: "logs-json-preview-header" },
        },
        [
          View(
            {
              class: "wx-logs-json-dialog-heading-icon",
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
              class: "wx-logs-json-dialog-heading",
              attributes: { n: "logs-json-preview-heading" },
            },
            [
              View(
                {
                  as: "span",
                  class: "wx-logs-json-dialog-title",
                  attributes: { n: "logs-json-preview-title" },
                },
                [vm$.state.json_preview_title],
              ),
              View(
                {
                  as: "span",
                  class: "wx-logs-json-dialog-subtitle",
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
          class: "wx-logs-json-dialog-body",
          attributes: { n: "logs-json-preview-body" },
        },
        [
          View(
            {
              type: "pre",
              class: "wx-logs-json-dialog-content",
              attributes: { n: "logs-json-preview-content" },
            },
            [vm$.state.json_preview_text],
          ),
        ],
      ),
    ],
  );
}

function LogsPageTableHead() {
  return View(
    {
      class: "wx-logs-table-head",
      attributes: { n: "logs-table-header", role: "row" },
    },
    [
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-level-header", role: "columnheader" },
        },
        ["level"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-time-header", role: "columnheader" },
        },
        ["time"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-file-header", role: "columnheader" },
        },
        ["file"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-component-header", role: "columnheader" },
        },
        ["component"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-message-header", role: "columnheader" },
        },
        ["msg"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { n: "logs-fields-header", role: "columnheader" },
        },
        ["fields"],
      ),
    ],
  );
}

function LogsPageTable(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-logs-table",
      attributes: { n: "logs-table", role: "table" },
    },
    [
      LogsPageTableHead(),
      VirtualListView({
        class: "wx-logs-virtual-list",
        attributes: { n: "logs-virtual-list" },
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
    ],
  );
}

function LogsPageSkeletonTable() {
  return View(
    {
      class: "wx-logs-table",
      attributes: { n: "logs-skeleton-table", role: "table" },
    },
    [
      LogsPageTableHead(),
      View(
        {
          class: "wx-logs-virtual-list",
          attributes: { n: "logs-skeleton-list" },
        },
        Array.from({ length: 6 }, function () {
          return View(
            {
              class: "wx-logs-table-row",
              attributes: { n: "logs-skeleton-row", role: "row" },
            },
            [
              View(
                {
                  class: "wx-logs-level-cell",
                  attributes: { n: "logs-skeleton-level-cell" },
                },
                [
                  View(
                    {
                      class: "wx-logs-level wx-logs-level-info",
                      attributes: { n: "logs-skeleton-level-badge" },
                    },
                    ["info"],
                  ),
                ],
              ),
              View(
                {
                  class: "wx-logs-time-cell",
                  attributes: { n: "logs-skeleton-time-cell" },
                },
                [
                  View({
                    class: "wx-logs-skeleton-cell",
                    style: { width: "150px" },
                    attributes: { n: "logs-skeleton-time-value" },
                  }),
                ],
              ),
              View(
                {
                  class: "wx-logs-file-cell",
                  attributes: { n: "logs-skeleton-file-cell" },
                },
                [
                  View({
                    class: "wx-logs-skeleton-cell",
                    style: { width: "80px" },
                    attributes: { n: "logs-skeleton-file-value" },
                  }),
                ],
              ),
              View(
                {
                  class: "wx-logs-component-cell",
                  attributes: { n: "logs-skeleton-component-cell" },
                },
                [
                  View({
                    class: "wx-logs-skeleton-cell",
                    style: { width: "100px" },
                    attributes: { n: "logs-skeleton-component-value" },
                  }),
                ],
              ),
              View(
                {
                  class: "wx-logs-msg-cell",
                  attributes: { n: "logs-skeleton-message-cell" },
                },
                [
                  View({
                    class: "wx-logs-skeleton-cell",
                    style: { width: "100%" },
                    attributes: { n: "logs-skeleton-message-value" },
                  }),
                ],
              ),
              View(
                {
                  class: "wx-logs-fields-cell",
                  attributes: { n: "logs-skeleton-fields-cell" },
                },
                [
                  View({
                    class: "wx-logs-skeleton-cell",
                    style: { width: "80%" },
                    attributes: { n: "logs-skeleton-fields-value" },
                  }),
                ],
              ),
            ],
          );
        }),
      ),
    ],
  );
}

function LogsPageLoadingState() {
  return LogsPageSkeletonTable();
}

function LogsPageEmptyState() {
  return View(
    {
      class: "wx-logs-state",
      attributes: { n: "logs-empty-state" },
    },
    [
      Timeless.Icon({
        name: "file-search",
        size: 32,
        attributes: { n: "logs-empty-state-icon" },
      }),
      View({ attributes: { n: "logs-empty-state-message" } }, [
        "没有匹配的日志",
      ]),
    ],
  );
}

function LogsPageList(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-rows wx-content-history-rows wx-logs-list dm-panel",
      attributes: { n: "logs-list-panel" },
    },
    [
      View(
        {
          class: "wx-logs-scroll",
          attributes: { n: "logs-scroll-container" },
        },
        [
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
        ],
      ),
    ],
  );
}

export default LogsPageView;

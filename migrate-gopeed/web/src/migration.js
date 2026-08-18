import {
  Alert,
  AlertDescription,
  Badge,
  Button,
  Card,
  CardContent,
  Dialog,
  DialogBody,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Pagination,
  Progress,
} from "./components.js";
import {
  DETAIL_FILTER_ITEMS,
  MigrationViewModel,
  STATUS_ITEMS,
  format_progress,
  format_size,
  status_text,
  status_variant,
  stringify_labels,
} from "./migration.model.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

const { For, Show, View, computed } = Timeless;

function bind_button(store, value, variant, size) {
  const button = store.bind(value);
  if (variant) button.setVariant(variant);
  if (size) button.setSize(size);
  return button;
}

function action_button(props) {
  return Button(
    {
      store: props.store,
      class: props.class || "",
      attributes: {
        type: "button",
        title: props.title || props.label,
        ...(props.attributes || {}),
      },
      prefix: props.icon
        ? Timeless.Icon({ name: props.icon, size: props.icon_size || 15 })
        : null,
      onDoubleClick: props.onDoubleClick,
    },
    [props.label],
  );
}

function header_view(model) {
  return View({ as: "header", class: "migration-header dm-page-header" }, [
    View({ class: "migration-header__inner dm-container" }, [
      View({ class: "migration-brand" }, [
        View({ class: "migration-brand__icon dm-icon-box" }, [
          Timeless.Icon({ name: "database-zap", size: 22 }),
        ]),
        View({ class: "migration-brand__copy" }, [
          View({ as: "h1", class: "migration-title" }, [
            "Gopeed 任务迁移预览",
          ]),
          View(
            {
              class: "migration-subtitle",
              attributes: { title: model.state.subtitle },
            },
            [model.state.subtitle],
          ),
        ]),
      ]),
      View({ class: "migration-header__actions" }, [
        action_button({
          store: model.ui.btn_refresh$,
          icon: "refresh-cw",
          label: "刷新",
        }),
        action_button({
          store: model.ui.btn_cleanup_cache$,
          icon: "trash2",
          label: "清理无效缓存",
          title: "清理 object.id 为空等不合法的详情缓存",
        }),
        action_button({
          store: model.ui.btn_fetch_all_details$,
          icon: "scan-search",
          label: model.state.fetch_all_label,
          title: "获取当前筛选下所有任务的详情",
        }),
      ]),
    ]),
  ]);
}

function status_button(model, item) {
  const key = item[0];
  const label = item[1];
  const active_ = computed(model.state.version, () =>
    model.methods.status_active(key),
  );
  const count_ = computed(model.state.version, () =>
    model.methods.status_count(key).toLocaleString(),
  );
  return Button(
    {
      store: bind_button(model.ui.btn_status$, key, "ghost", "sm"),
      class: computed(active_, (active) =>
        active
          ? "migration-status migration-status--active"
          : "migration-status",
      ),
      attributes: {
        type: "button",
        role: "tab",
        "aria-selected": active_,
      },
    },
    [
      View({ as: "span", class: "migration-status__label" }, [label]),
      View({ as: "span", class: "migration-status__count" }, [count_]),
    ],
  );
}

function source_toolbar_view(model) {
  return View({ class: "migration-source dm-container" }, [
    Card({ class: "migration-source__card" }, [
      CardContent({ class: "migration-source__content" }, [
        View({ class: "migration-path" }, [
          Input({
            store: model.ui.path_input$,
            class: "migration-path__input",
            attributes: {
              name: "gopeed_path",
              autocomplete: "off",
              "aria-label": "Gopeed .db 数据库文件路径",
            },
          }),
          action_button({
            store: model.ui.btn_browse$,
            icon: "file",
            label: "浏览",
            title: "选择 .db 数据库文件",
          }),
          action_button({
            store: model.ui.btn_load$,
            icon: "database",
            label: "加载",
            title: "加载选中的 .db 数据库文件",
          }),
        ]),
        View(
          {
            class: "migration-statuses",
            attributes: { role: "tablist", "aria-label": "任务状态筛选" },
          },
          [
            For({
              each: STATUS_ITEMS,
              render(item) {
                return status_button(model, item);
              },
            }),
          ],
        ),
      ]),
    ]),
  ]);
}

function field_view(props, children) {
  return View({ class: "migration-field " + (props.class || "") }, [
    Label({}, [props.label]),
    children,
  ]);
}

function migration_controls_view(model) {
  return View({ class: "migration-controls dm-container" }, [
    Card({ class: "migration-controls__card" }, [
      CardContent({ class: "migration-controls__content" }, [
        View({ class: "migration-control-grid" }, [
          field_view(
            { label: "目标服务", class: "migration-field--service" },
            Input({
              store: model.ui.target_input$,
              attributes: {
                name: "target_url",
                autocomplete: "off",
                "aria-label": "目标服务",
              },
            }),
          ),
          field_view(
            { label: "目标数据库", class: "migration-field--database" },
            Input({
              store: model.ui.target_db_input$,
              attributes: {
                name: "target_db",
                autocomplete: "off",
                "aria-label": "目标数据库",
              },
            }),
          ),
        ]),
        View({ class: "migration-control-actions" }, [
          action_button({
            store: model.ui.btn_migration_execute$,
            icon: "play",
            label: "执行迁移",
          }),
        ]),
      ]),
    ]),
  ]);
}

function detail_filter_button(model, item) {
  const key = item[0];
  const label = item[1];
  const active_ = computed(model.state.version, () =>
    model.methods.detail_filter_active(key),
  );
  const count_ = computed(model.state.version, () =>
    model.methods.detail_filter_count(key).toLocaleString(),
  );
  return Button(
    {
      store: bind_button(model.ui.btn_detail_filter$, key, "ghost", "xs"),
      class: computed(active_, (active) =>
        active
          ? "migration-detail-filter migration-detail-filter--active"
          : "migration-detail-filter",
      ),
      attributes: {
        type: "button",
        "aria-pressed": active_,
      },
    },
    [label, " ", count_],
  );
}

function detail_progress_view(model) {
  return Show({
    when: model.state.detail_visible,
    ok() {
      return View({ class: "migration-detail dm-container" }, [
        Card({ class: "migration-detail__card" }, [
          CardContent({ class: "migration-detail__content" }, [
            View({ class: "migration-detail__summary" }, [
              Timeless.Icon({ name: "activity", size: 16 }),
              View({ as: "span" }, [model.state.detail_summary]),
            ]),
            Progress({
              store: model.ui.detail_progress$,
              class: "migration-detail__progress",
              attributes: { "aria-label": "详情获取进度" },
            }),
            View({ class: "migration-detail__filters" }, [
              For({
                each: DETAIL_FILTER_ITEMS,
                render(item) {
                  return detail_filter_button(model, item);
                },
              }),
            ]),
          ]),
        ]),
      ]);
    },
  });
}

function task_detail_result_view(model, row) {
  const result = model.methods.detail_result_view(row);
  if (!result) {
    return View({ as: "span", class: "migration-muted" }, ["—"]);
  }
  return View({ class: "migration-task-detail" }, [
    Badge({ variant: result.variant }, [result.label]),
    Show({
      when: Boolean(result.detail),
      ok() {
        return View(
          {
            class:
              result.status === "failed"
                ? "migration-task-detail__text is-error"
                : "migration-task-detail__text",
            attributes: { title: result.detail },
          },
          [result.detail],
        );
      },
    }),
  ]);
}

function task_row_view(model, row) {
  const status = String(row.status || "");
  const labels = stringify_labels(row.labels);
  return View(
    {
      class: "migration-task-row",
      attributes: { role: "row" },
    },
    [
      View(
        {
          class: "migration-task-cell migration-task-id",
          attributes: {
            role: "cell",
            title: String(row.id || ""),
          },
        },
        [String(row.id || "")],
      ),
      View(
        {
          class: "migration-task-cell migration-task-name",
          attributes: {
            role: "cell",
            title: String(row.name || ""),
          },
        },
        [
          View({ class: "migration-task-name__main" }, [
            String(row.name || ""),
          ]),
          View({ class: "migration-task-name__meta" }, [
            Badge({ variant: status_variant(status) }, [status_text(status)]),
            View({ as: "span" }, ["size ", format_size(row.size)]),
            View({ as: "span" }, [
              "downloaded ",
              format_size(row.downloaded),
            ]),
            View({ as: "span" }, [
              "progress ",
              format_progress(row.progress),
            ]),
          ]),
        ],
      ),
      View(
        {
          class: "migration-task-cell migration-task-labels",
          attributes: { role: "cell", title: labels },
        },
        [
          View({ class: "migration-task-labels__actions" }, [
            Show({
              when: model.methods.is_task_detail_fetched(row),
              ok() {
                return Badge(
                  {
                    variant: "success",
                    attributes: {
                      title: model.methods.detail_fetched_title(row),
                    },
                  },
                  ["已获取详情"],
                );
              },
            }),
            action_button({
              store: bind_button(
                model.ui.btn_profile_detail$,
                row,
                "outline",
                "xs",
              ),
              icon: "file-json",
              icon_size: 13,
              label: "详情",
            }),
            action_button({
              store: bind_button(
                model.ui.btn_migrate_row$,
                row,
                "primary",
                "xs",
              ),
              icon: "move-right",
              icon_size: 13,
              label: "迁移",
            }),
          ]),
          View({ as: "pre", class: "migration-labels-json" }, [labels]),
        ],
      ),
      View(
        {
          class: "migration-task-cell migration-task-detail-cell",
          attributes: { role: "cell" },
        },
        [task_detail_result_view(model, row)],
      ),
      View(
        {
          class: "migration-task-cell migration-task-time",
          attributes: {
            role: "cell",
            title: String(row.created_at || ""),
          },
        },
        [String(row.created_at || "")],
      ),
      View(
        {
          class: "migration-task-cell migration-task-time",
          attributes: {
            role: "cell",
            title: String(row.updated_at || ""),
          },
        },
        [String(row.updated_at || "")],
      ),
    ],
  );
}

function task_table_head_view() {
  const labels = [
    "ID",
    "名称 / 状态",
    "Labels / 操作",
    "详情结果",
    "创建时间",
    "更新时间",
  ];
  return View(
    {
      class: "migration-task-row migration-task-row--head",
      attributes: { role: "row" },
    },
    labels.map((label) =>
      View(
        {
          class: "migration-task-head-cell",
          attributes: { role: "columnheader" },
        },
        [label],
      ),
    ),
  );
}

function task_table_view(model) {
  const has_rows_ = computed(
    model.state.table_rows,
    (rows) => Array.isArray(rows) && rows.length > 0,
  );
  return Card({ class: "migration-table-card" }, [
    View({ class: "migration-table-card__header" }, [
      View({ class: "migration-table-card__title" }, [
        Timeless.Icon({ name: "list-tree", size: 16 }),
        View({ as: "span" }, ["task"]),
      ]),
      View({ class: "migration-table-card__meta" }, [
        model.state.table_meta,
      ]),
    ]),
    Show({
      when: has_rows_,
      ok() {
        return View({ class: "migration-table-scroll" }, [
          View(
            {
              class: "migration-task-table",
              attributes: { role: "table", "aria-label": "Gopeed tasks" },
            },
            [
              task_table_head_view(),
              View(
                {
                  class: "migration-task-list",
                  attributes: { role: "rowgroup" },
                },
                [
                  For({
                    each: model.state.table_rows,
                    render(row) {
                      return task_row_view(model, row);
                    },
                  }),
                ],
              ),
            ],
          ),
        ]);
      },
      else() {
        return View({ class: "migration-empty dm-empty-state" }, [
          Timeless.Icon({ name: "database", size: 34 }),
          View({ as: "span" }, [model.state.table_empty_text]),
        ]);
      },
    }),
    Pagination({
      class: "migration-pagination",
      summary: model.state.table_meta,
      page: model.state.table_page,
      pageCount: model.state.table_page_count,
      loading: model.state.loading,
      onPrevious() {
        model.methods.previous_page();
      },
      onNext() {
        model.methods.next_page();
      },
    }),
  ]);
}

function result_summary_view(result_) {
  return View({ class: "migration-result-summary" }, [
    View({ as: "span" }, [
      "目标: ",
      computed(result_, (result) => (result && result.target_url) || ""),
    ]),
    View({ as: "span" }, [
      "目标库: ",
      computed(result_, (result) => (result && result.target_db) || ""),
    ]),
    View({ as: "span" }, [
      "总数: ",
      computed(result_, (result) =>
        Number((result && result.total) || 0).toLocaleString(),
      ),
    ]),
    Badge({ variant: "success" }, [
      "成功 ",
      computed(result_, (result) =>
        Number((result && result.success) || 0).toLocaleString(),
      ),
    ]),
    Badge({ variant: "warning" }, [
      "跳过 ",
      computed(result_, (result) =>
        Number((result && result.skipped) || 0).toLocaleString(),
      ),
    ]),
    Badge({ variant: "danger" }, [
      "失败 ",
      computed(result_, (result) =>
        Number((result && result.failed) || 0).toLocaleString(),
      ),
    ]),
  ]);
}

function migration_result_head_view() {
  const labels = ["动作", "Task ID", "名称", "oid", "uid", "文件", "结果"];
  return View(
    {
      class: "migration-result-row migration-result-row--head",
      attributes: { role: "row" },
    },
    labels.map((label) =>
      View(
        {
          class: "migration-result-head-cell",
          attributes: { role: "columnheader" },
        },
        [label],
      ),
    ),
  );
}

function migration_result_row_view(item) {
  const succeeded = item.action === "migrated";
  const result = String(
    item.error ||
      item.target_id ||
      (item.profile_cache_hit ? "profile cache" : ""),
  );
  const cells = [
    String(item.task_id || ""),
    String(item.name || ""),
    String(item.oid || ""),
    String(item.uid || ""),
    String(item.save_path || ""),
    result,
  ];
  return View(
    {
      class: "migration-result-row",
      attributes: { role: "row" },
    },
    [
      View(
        {
          class: "migration-result-cell migration-result-action",
          attributes: { role: "cell" },
        },
        [
          Badge({ variant: succeeded ? "success" : "danger" }, [
            item.action || "failed",
          ]),
        ],
      ),
      ...cells.map((value) =>
        View(
          {
            class: "migration-result-cell",
            attributes: { role: "cell", title: value },
          },
          [value],
        ),
      ),
    ],
  );
}

function migration_result_view(model) {
  const items_ = computed(model.state.migration_result, (result) =>
    result && Array.isArray(result.items) ? result.items.slice(0, 100) : [],
  );
  return Show({
    when: model.state.migration_result,
    ok() {
      return Card({ class: "migration-result-card" }, [
        CardContent({ class: "migration-result-card__content" }, [
          result_summary_view(model.state.migration_result),
          View({ class: "migration-result-table-scroll" }, [
            View(
              {
                class: "migration-result-table",
                attributes: {
                  role: "table",
                  "aria-label": "迁移结果",
                },
              },
              [
                migration_result_head_view(),
                View(
                  {
                    class: "migration-result-rows",
                    attributes: { role: "rowgroup" },
                  },
                  [
                    For({
                      each: items_,
                      render(item) {
                        return migration_result_row_view(item);
                      },
                    }),
                  ],
                ),
              ],
            ),
          ]),
        ]),
      ]);
    },
  });
}

function profile_dialog_view(model) {
  return Dialog(
    {
      store: model.ui.profile_dialog$,
      class: "migration-dialog migration-profile-dialog",
      showClose: true,
    },
    () => [
      DialogHeader({}, [
        DialogTitle({}, [model.state.profile_title]),
      ]),
      DialogBody({}, [
        View({ type: "pre", class: "migration-profile-json" }, [
          model.state.profile_json,
        ]),
      ]),
      DialogFooter({}, [
        action_button({
          store: model.ui.btn_profile_close$,
          label: "关闭",
        }),
      ]),
    ],
  );
}

function picker_file_view(model, file) {
  const selected_ = computed(
    model.state.version,
    () => model.state.picker_selected_path.value === file.path,
  );
  return Button(
    {
      store: bind_button(
        model.ui.btn_picker_file$,
        file,
        "ghost",
        "default",
      ),
      class: computed(selected_, (selected) =>
        selected
          ? "migration-file-row migration-file-row--selected"
          : "migration-file-row",
      ),
      attributes: {
        type: "button",
        title: String(file.path || ""),
      },
      onDoubleClick() {
        model.methods.activate_picker_entry(file);
      },
    },
    [
      Timeless.Icon({
        name: file.isDir ? "folder" : "file",
        size: 16,
      }),
      View({ as: "span", class: "migration-file-row__name" }, [
        String(file.name || ""),
      ]),
      View({ as: "span", class: "migration-file-row__size" }, [
        file.isDir ? "" : format_size(file.size),
      ]),
      View({ as: "span", class: "migration-file-row__time" }, [
        String(file.modTime || ""),
      ]),
    ],
  );
}

function picker_dialog_view(model) {
  const has_entries_ = computed(
    model.state.picker_entries,
    (entries) => Array.isArray(entries) && entries.length > 0,
  );
  return Dialog(
    {
      store: model.ui.picker_dialog$,
      class: "migration-dialog migration-picker-dialog",
      showClose: true,
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["选择 .db 数据库文件"]),
      ]),
      DialogBody({ class: "migration-picker-body" }, [
        View({ class: "migration-breadcrumb" }, [
          For({
            each: model.state.picker_breadcrumbs,
            render(crumb) {
              return View({ class: "migration-breadcrumb__item" }, [
                Show({
                  when: crumb.path !== "/",
                  ok() {
                    return View(
                      { as: "span", class: "migration-breadcrumb__separator" },
                      ["/"],
                    );
                  },
                }),
                Button(
                  {
                    store: bind_button(
                      model.ui.btn_picker_breadcrumb$,
                      crumb.path,
                      "ghost",
                      "xs",
                    ),
                    class: "migration-breadcrumb__button",
                    attributes: { type: "button", title: crumb.path },
                  },
                  [crumb.label],
                ),
              ]);
            },
          }),
        ]),
        Show({
          when: model.state.picker_loading,
          ok() {
            return View({ class: "migration-picker-state" }, [
              View({
                class: "dm-ui-spinner",
                attributes: { "aria-hidden": "true" },
              }),
              "读取中…",
            ]);
          },
          else() {
            return Show({
              when: model.state.picker_error,
              ok() {
                return Alert({ variant: "danger" }, [
                  AlertDescription({}, [model.state.picker_error]),
                ]);
              },
              else() {
                return Show({
                  when: has_entries_,
                  ok() {
                    return View({ class: "migration-file-list" }, [
                      For({
                        each: model.state.picker_entries,
                        render(file) {
                          return picker_file_view(model, file);
                        },
                      }),
                    ]);
                  },
                  else() {
                    return View({ class: "migration-picker-state" }, [
                      "当前目录中没有可选择的 .db 文件",
                    ]);
                  },
                });
              },
            });
          },
        }),
      ]),
      DialogFooter({}, [
        action_button({
          store: model.ui.btn_picker_cancel$,
          label: "取消",
        }),
        action_button({
          store: model.ui.btn_picker_select$,
          icon: "check",
          label: "选择文件",
        }),
      ]),
    ],
  );
}

function confirm_dialog_view(model) {
  return Dialog(
    {
      store: model.ui.confirm_dialog$,
      class: "migration-dialog migration-confirm-dialog",
      showClose: true,
      cancelText: "取消",
      okText: "确认",
    },
    [
      DialogBody({}, [
        View({ class: "migration-confirm-message" }, [
          model.state.confirm_message,
        ]),
      ]),
    ],
  );
}

function toast_view(model) {
  return Show({
    when: model.state.toast_message,
    ok() {
      return Alert({
        class: computed(model.state.toast_error, (is_error) =>
          is_error
            ? "migration-toast is-destructive"
            : "migration-toast",
        ),
        attributes: {
          role: "status",
          "aria-live": "polite",
        },
      }, [
        AlertDescription({}, [model.state.toast_message]),
      ]);
    },
  });
}

export default function MigrationPageView() {
  const model = MigrationViewModel();

  return View(
    {
      class: "migration-page dm-page",
      onMounted() {
        model.methods.ready();
      },
      onUnmounted() {
        model.methods.destroy();
      },
    },
    [
      header_view(model),
      View({ as: "main", class: "migration-main" }, [
        source_toolbar_view(model),
        migration_controls_view(model),
        detail_progress_view(model),
        View({ class: "migration-content dm-container" }, [
          migration_result_view(model),
          task_table_view(model),
        ]),
      ]),
      profile_dialog_view(model),
      picker_dialog_view(model),
      confirm_dialog_view(model),
      toast_view(model),
    ],
  );
}

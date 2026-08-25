import {
  DOWNLOAD_STATUS_COUNT_ITEMS,
  MaxRunning,
  format_download_percent,
  format_download_size,
  format_download_speed,
  format_download_time,
  get_download_status_count,
  is_download_open_external,
  is_download_waiting_status,
  normalize_download_status,
} from "./downloadv2.model.js";

const OVERWRITE_ACTION_ITEMS = [
  {
    value: "overwrite",
    label: "覆盖",
    description: "删除已有任务和文件后重新创建",
    icon: "refresh-cw",
  },
  {
    value: "skip",
    label: "跳过",
    description: "保留已有任务，不创建当前任务",
    icon: "corner-down-right",
  },
  {
    value: "duplicate",
    label: "重复",
    description: "保留已有任务，再创建一份",
    icon: "copy",
  },
];

const DIALOG_STYLE = {
  width: "min(560px, calc(100vw - 32px))",
};

const FIELD_GROUP_STYLE = {
  display: "grid",
  gap: "6px",
};

const FIELD_LABEL_STYLE = {
  "font-size": "13px",
  "font-weight": "600",
  color: "var(--dm-color-text-secondary)",
};

function task_value(task_) {
  return task_ && task_.value !== undefined ? task_.value : task_;
}

function task_files(raw) {
  if (!raw || typeof raw !== "object") return [];
  if (Array.isArray(raw.files)) return raw.files;
  return Array.isArray(raw.resources) ? raw.resources : [];
}

function format_progress_text(percent) {
  const value = Number(percent);
  if (!Number.isFinite(value)) return "0";
  return String(Math.round(Math.max(0, Math.min(100, value))));
}

function is_live_stream_task(raw) {
  return task_files(raw).some((file) => {
    if (!file) return false;
    return [file.type, file.resource_type].some((type) => {
      return String(type || "").toUpperCase() === "STREAM";
    });
  });
}

function task_has_content(raw) {
  if (!raw || typeof raw !== "object") return false;
  const content_id = raw.content_id ?? raw.contentId ?? raw.ContentID;
  if (content_id === undefined || content_id === null) return false;
  return String(content_id).trim() !== "";
}

function DownloadV2ActionButton(props) {
  const {
    attributes,
    class: extra_class,
    compact,
    icon,
    iconSize: icon_size,
    label,
    store,
    title,
  } = props;
  return Button(
    {
      store,
      class: [
        "wx-dl-v2-action",
        compact ? "wx-content-action-compact" : "",
        extra_class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: title || "",
        ...(attributes || {}),
      },
      prefix: icon
        ? Timeless.Icon({ name: icon, size: icon_size || 16 })
        : null,
    },
    typeof label !== "undefined"
      ? [View({ class: "wx-content-action-label" }, [label])]
      : [],
  );
}

function DownloadV2Number(props = {}) {
  const {
    attributes,
    characterStyle: character_style,
    characterWidth: character_width = "0.65em",
    class: extra_class,
    decimalWidth: decimal_width = "0.3em",
    number,
    separatorWidth: separator_width = "0.4em",
    spaceWidth: space_width = "0.25em",
    style,
    value: provided_value,
  } = props;
  const value = provided_value === undefined ? number : provided_value;
  const to_characters = (current_value) =>
    Array.from(current_value == null ? "" : String(current_value)).map(
      (character, index) => ({ key: `${index}:${character}`, character }),
    );
  const characters =
    value && value.__is_ref
      ? computed(value, to_characters)
      : to_characters(value);

  return View(
    {
      class: ["wx-number-view", extra_class].filter(Boolean).join(" "),
      style: {
        display: "inline-flex",
        "align-items": "center",
        "white-space": "nowrap",
        ...(style || {}),
      },
      attributes: attributes || {},
    },
    [
      For({
        key: "key",
        each: characters,
        render(item) {
          let width = character_width;
          if (item.character === ".") width = decimal_width;
          else if (/\s/.test(item.character)) width = space_width;
          else if (item.character === "/") width = separator_width;
          return View(
            {
              class: "wx-number-view-character",
              style: {
                display: "inline-flex",
                width,
                "min-width": width,
                "flex-basis": width,
                "flex-shrink": "0",
                "align-items": "center",
                "justify-content": "center",
                "text-align": "center",
                ...(character_style || {}),
              },
            },
            [item.character],
          );
        },
      }),
    ],
  );
}

function DownloadV2InfinityIcon(props = {}) {
  const { size = 14 } = props;
  return SVG.SVG(
    {
      style: { display: "block", "flex-shrink": "0" },
      attributes: {
        width: String(size),
        height: String(size),
        xmlns: "http://www.w3.org/2000/svg",
        viewBox: "0 0 24 24",
        fill: "none",
        stroke: "currentColor",
        "stroke-width": "2",
        "stroke-linecap": "round",
        "stroke-linejoin": "round",
        "aria-hidden": "true",
      },
    },
    [
      SVG.Path({
        attributes: {
          d: "M6 16c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8",
        },
      }),
    ],
  );
}

function DownloadV2Skeleton(props = {}) {
  const { attributes, class: extra_class, style } = props;
  return View({
    class: ["wx-skeleton", extra_class].filter(Boolean).join(" "),
    style: style || {},
    attributes: attributes || {},
  });
}

function DownloadV2TaskState(task$) {
  return combine(
    {
      status: task$.state.status,
      progress: task$.state.progress,
      error: task$.state.error,
      raw: task$.state.raw,
    },
    (source) => {
      const raw = source.raw || {};
      const progress = source.progress || {};
      const percent = format_download_percent({ progress });
      const is_live_stream = is_live_stream_task(raw);
      const status = normalize_download_status(source.status);
      const is_paused = status === "pause";
      const is_running = status === "running";
      const is_failed = status === "error";
      const is_pending = is_download_waiting_status(status);
      const is_completed =
        status === "done" ||
        (percent === 100 &&
          !is_running &&
          !is_failed &&
          !is_paused &&
          !is_pending);
      const files = task_files(raw);
      const deleted_file_count = files.filter((file) => {
        return String((file && file.status) || "").toLowerCase() === "deleted";
      }).length;
      const has_deleted_files = deleted_file_count > 0;
      const all_files_deleted =
        files.length > 0 && deleted_file_count === files.length;
      const files_downloaded_size = files.reduce(
        (sum, file) => sum + (Number(file && file.downloaded) || 0),
        0,
      );
      const files_total_size = files.reduce(
        (sum, file) => sum + (Number(file && file.size) || 0),
        0,
      );
      const downloaded_size = Math.max(
        files_downloaded_size,
        Number(progress.downloaded) || 0,
      );
      const total_size = Math.max(
        files_total_size,
        Number(progress.total) || 0,
      );
      let status_text = source.status || "";
      let status_color = "var(--dm-dl-page-muted)";
      let error_text = "";
      let progress_text = "";
      let speed_text = "";

      if (is_running) {
        speed_text = format_download_speed(progress.speed);
        status_text = "下载中";
        progress_text = is_live_stream ? "" : format_progress_text(percent);
        status_color = "var(--dm-dl-page-primary)";
      } else if (is_completed) {
        status_text = has_deleted_files
          ? all_files_deleted
            ? "文件已删除"
            : "部分文件已删除"
          : "已完成";
        status_color = has_deleted_files
          ? "var(--dm-color-danger)"
          : "var(--dm-color-success)";
      } else if (is_failed) {
        status_text = "失败";
        error_text =
          (source.error && (source.error.message || String(source.error))) ||
          raw.error ||
          raw.error_message ||
          raw._errMsg ||
          "下载失败";
        status_color = "var(--dm-color-danger)";
      } else if (is_pending) {
        status_text = "等待中...";
      } else if (is_paused) {
        status_text = "已暂停";
        status_color = "var(--dm-color-warning)";
        progress_text = is_live_stream ? "" : format_progress_text(percent);
      }

      return {
        percent,
        is_live_stream,
        is_completed,
        is_paused,
        is_running,
        is_failed,
        is_pending,
        status_text,
        status_color,
        error_text,
        progress_text,
        speed_text,
        downloaded_size_text: format_download_size(downloaded_size),
        total_size_text: format_download_size(total_size),
      };
    },
  );
}

function task_cover_url(raw) {
  if (!raw || typeof raw !== "object") return "";
  return String(raw.cover_url || raw.coverUrl || raw.CoverURL || "").trim();
}

function DownloadV2TaskCover(props) {
  const { state: state_, task: task$ } = props;
  const raw_ = task$.state.raw;
  const progress = Show({
    when: computed(
      state_,
      (state) => !state.is_live_stream && (state.is_running || state.is_paused),
    ),
    ok() {
      return View({ class: "wx-dl-page-task-cover-progress" }, [
        DownloadV2Number({
          value: computed(state_, (state) => state.progress_text),
        }),
      ]);
    },
  });
  const fallback = () =>
    View({ class: "wx-dl-page-task-cover wx-dl-page-task-cover-fallback" });

  return Show({
    when: computed(raw_, (raw) => Boolean(task_cover_url(raw))),
    ok() {
      return View({ class: "wx-dl-page-task-cover-wrap" }, [
        fallback(),
        Img({
          class: "wx-dl-page-task-cover",
          src: computed(raw_, task_cover_url),
          alt: task$.state.name,
          attributes: {
            loading: "lazy",
            referrerpolicy: "no-referrer",
          },
          onError(event) {
            event.target.style.display = "none";
          },
        }),
        progress,
      ]);
    },
  });
}

function DownloadV2TaskActionButton(props) {
  const { danger, icon, iconSize: icon_size, onClick: on_click, title } = props;
  return View(
    {
      type: "button",
      class: [
        "wx-dl-page-task-action",
        danger ? "wx-dl-page-task-action-danger" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: title || "",
        "aria-label": title || "",
      },
      onClick: on_click,
    },
    [Timeless.Icon({ name: icon, size: icon_size || 18 })],
  );
}

function DownloadV2TaskActions(props) {
  const { state: state_, store: vm$, task: task$ } = props;
  const is_open_external = is_download_open_external();

  return [
    Match({
      when: combine(
        { state: state_, running_count: vm$.state.running_count },
        (value) => {
          if (value.state.is_completed) return 1;
          if (value.state.is_running) return 2;
          if (value.state.is_live_stream && value.state.is_paused) return 5;
          if (value.running_count >= MaxRunning) return 5;
          if (value.state.is_paused) return 3;
          if (value.state.is_failed) return 4;
          return 0;
        },
      ),
      cases: {
        0() {
          return DownloadV2TaskActionButton({
            icon: "play",
            title: "开始",
            onClick() {
              vm$.methods.startTask(task$);
            },
          });
        },
        1() {
          return DownloadV2TaskActionButton({
            icon: is_open_external ? "file-symlink" : "folder",
            title: is_open_external ? "打开链接" : "打开文件夹",
            onClick() {
              vm$.methods.openTask(task$);
            },
          });
        },
        2() {
          return View(
            {
              type: "button",
              class: "wx-dl-page-task-action",
              attributes: {
                type: "button",
                title: computed(state_, (state) =>
                  state.is_live_stream ? "停止录制" : "暂停",
                ),
                "aria-label": computed(state_, (state) =>
                  state.is_live_stream ? "停止录制" : "暂停",
                ),
              },
              onClick() {
                vm$.methods.pauseTask(task$, {
                  liveStream: state_.value.is_live_stream,
                });
              },
            },
            [
              Show({
                when: computed(state_, (state) => state.is_live_stream),
                ok() {
                  return Timeless.Icon({ name: "square", size: 18 });
                },
                else() {
                  return Timeless.Icon({ name: "pause", size: 18 });
                },
              }),
            ],
          );
        },
        3() {
          return DownloadV2TaskActionButton({
            icon: "play",
            title: "继续",
            onClick() {
              vm$.methods.resumeTask(task$);
            },
          });
        },
        4() {
          return DownloadV2TaskActionButton({
            icon: "refresh-ccw",
            title: "重试",
            onClick() {
              vm$.methods.retryTask(task$);
            },
          });
        },
        5() {
          return View({ class: "wx-dl-page-task-action-spacer" });
        },
      },
    }),
    DownloadV2TaskActionButton({
      icon: "trash2",
      title: "删除",
      danger: true,
      onClick() {
        vm$.methods.requestDeleteTask(task$);
      },
    }),
  ];
}

function DownloadV2TaskMain(props) {
  const { store: vm$ } = props;
  const task$ = task_value(props.task);
  const state_ = DownloadV2TaskState(task$);

  return [
    DownloadV2TaskCover({ task: task$, state: state_ }),
    View(
      {
        class: "wx-dl-page-task-info",
        attributes: { n: "download-task-info" },
      },
      [
        View(
          {
            class: "wx-dl-page-task-title-line",
            attributes: { n: "download-task-title-line" },
          },
          [
            View(
              {
                as: "button",
                class: "wx-dl-page-task-title",
                attributes: {
                  n: "download-task-preview-trigger",
                  type: "button",
                  title: task$.state.name,
                },
                onClick() {
                  vm$.methods.requestTaskPreview(task$);
                },
              },
              [
                computed(task$.state.name, (name) => name || "未命名任务"),
              ],
            ),
            Show({
              when: computed(state_, (state) => state.is_live_stream),
              ok() {
                return View(
                  {
                    class: "wx-dl-page-task-live",
                    attributes: { n: "download-task-live-badge" },
                  },
                  ["流媒体"],
                );
              },
            }),
            Show({
              when: computed(task$.state.raw, (raw) => !task_has_content(raw)),
              ok() {
                return View(
                  {
                    class:
                      "wx-dl-page-task-missing-detail dm-badge dm-badge--warning",
                    attributes: {
                      n: "download-task-missing-detail-badge",
                      title: "该下载任务没有关联内容详情",
                    },
                  },
                  ["缺少详情"],
                );
              },
            }),
          ],
        ),
        View(
          {
            class: "wx-dl-page-task-desc",
            attributes: { n: "download-task-description" },
          },
          [
            View(
              {
                class: "wx-dl-page-task-status",
                style: computed(state_, (state) => ({
                  color: state.status_color,
                })),
                attributes: { n: "download-task-status" },
              },
              [computed(state_, (state) => state.status_text)],
            ),
            "·",
            Show({
              when: computed(state_, (state) => state.is_completed),
              ok() {
                return DownloadV2Number({
                  value: computed(state_, (state) => state.total_size_text),
                });
              },
              else() {
                return [
                  DownloadV2Number({
                    value: computed(
                      state_,
                      (state) => `${state.downloaded_size_text} /`,
                    ),
                  }),
                  Show({
                    when: computed(state_, (state) => state.is_live_stream),
                    ok() {
                      return DownloadV2InfinityIcon({ size: 14 });
                    },
                    else() {
                      return DownloadV2Number({
                        value: computed(
                          state_,
                          (state) => state.total_size_text,
                        ),
                      });
                    },
                  }),
                ];
              },
            }),
            Show({
              when: computed(
                state_,
                (state) => state.is_running && Boolean(state.speed_text),
              ),
              ok() {
                return [
                  "·",
                  DownloadV2Number({
                    value: computed(state_, (state) => state.speed_text),
                  }),
                ];
              },
            }),
          ],
        ),
        Show({
          when: computed(
            state_,
            (state) => state.is_failed && Boolean(state.error_text),
          ),
          ok() {
            return View(
              {
                class: "wx-dl-page-task-error",
                attributes: {
                  n: "download-task-error",
                  title: computed(state_, (state) => state.error_text),
                },
              },
              [computed(state_, (state) => state.error_text)],
            );
          },
        }),
      ],
    ),
  ];
}

export function DownloadV2TaskColumns(props) {
  const vm$ = props.store;
  return [
    {
      name: "task",
      title: "下载任务",
      cellClass: "wx-dl-page-task-main-cell",
      render(task$) {
        return DownloadV2TaskMain({
          store: vm$,
          task: task$,
        });
      },
    },
    {
      name: "created-at",
      title: "下载时间",
      cellClass: "wx-dl-page-task-time-cell",
      cellAttributes(task$) {
        return {
          title: computed(task$.state.raw, (raw) =>
            format_download_time(raw && raw.created_at),
          ),
        };
      },
      render(task$) {
        return computed(task$.state.raw, (raw) =>
          format_download_time(raw && raw.created_at),
        );
      },
    },
    {
      name: "actions",
      title: "操作",
      headerClass: "wx-dl-page-table-head-action",
      cellClass: "wx-dl-page-task-actions-cell",
      render(task$) {
        const state_ = DownloadV2TaskState(task$);
        return DownloadV2TaskActions({
          store: vm$,
          task: task$,
          state: state_,
        });
      },
    },
  ];
}

export function DownloadV2TaskSkeletonRow() {
  return View(
    {
      class: "wx-table-row wx-dl-page-task-row wx-dl-page-task-skeleton",
      attributes: { n: "download-task-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "wx-table-selection-cell",
          attributes: { n: "download-task-skeleton-selection", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            attributes: { n: "download-task-skeleton-checkbox" },
            style: {
              width: "18px",
              height: "18px",
              "border-radius": "4px",
            },
          }),
        ],
      ),
      View(
        {
          class: "wx-dl-page-task-main-cell",
          attributes: { n: "download-task-skeleton-main", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            attributes: { n: "download-task-skeleton-cover" },
            style: {
              width: "var(--dm-control-height-lg)",
              height: "var(--dm-control-height-lg)",
              "border-radius": "6px",
              "margin-right": "12px",
            },
          }),
          View(
            {
              class: "wx-dl-page-task-info",
              attributes: { n: "download-task-skeleton-info" },
            },
            [
              DownloadV2Skeleton({
                class: "wx-dl-skeleton-line",
                attributes: { n: "download-task-skeleton-title" },
                style: {
                  width: "56%",
                  height: "14px",
                  "border-radius": "5px",
                },
              }),
              DownloadV2Skeleton({
                class: "wx-dl-skeleton-line",
                attributes: { n: "download-task-skeleton-status" },
                style: {
                  width: "36%",
                  height: "12px",
                  "border-radius": "5px",
                  "margin-top": "7px",
                },
              }),
            ],
          ),
        ],
      ),
      DownloadV2Skeleton({
        attributes: {
          n: "download-task-skeleton-created-at",
          role: "cell",
        },
        style: { width: "128px", height: "12px", "border-radius": "5px" },
      }),
      View(
        {
          class: "wx-dl-page-task-actions-cell",
          attributes: { n: "download-task-skeleton-actions", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            attributes: { n: "download-task-skeleton-primary-action" },
            style: {
              width: "34px",
              height: "34px",
              "border-radius": "8px",
            },
          }),
          DownloadV2Skeleton({
            attributes: { n: "download-task-skeleton-delete-action" },
            style: {
              width: "34px",
              height: "34px",
              "border-radius": "8px",
            },
          }),
        ],
      ),
    ],
  );
}

function DownloadV2StatusCounts(props) {
  const { store: vm$ } = props;
  const active_status_ = vm$.state.active_status;
  const status_counts_ = vm$.state.status_counts;

  return View(
    {
      class: "wx-dl-page-counts wx-dl-v2-page-counts",
      attributes: {
        role: "group",
        "aria-label": "Download status filters",
      },
    },
    [
      For({
        each: DOWNLOAD_STATUS_COUNT_ITEMS,
        render(item) {
          return View(
            {
              type: "button",
              attributes: {
                type: "button",
                "aria-pressed": computed(active_status_, (status) =>
                  status === item.key ? "true" : "false",
                ),
              },
              class: computed(active_status_, (status) =>
                [
                  "wx-dl-v2-tab dm-focus-ring",
                  status === item.key ? "wx-dl-v2-tab-active" : "",
                  item.key === "error" ? "wx-dl-v2-tab-error" : "",
                ]
                  .filter(Boolean)
                  .join(" "),
              ),
              onClick() {
                vm$.methods.setStatusFilter(item.key);
              },
            },
            [
              View({ class: "wx-dl-v2-tab-label" }, [item.label]),
              View({ class: "wx-dl-v2-tab-count" }, [
                computed(status_counts_, (counts) => {
                  return String(get_download_status_count(counts, item));
                }),
              ]),
            ],
          );
        },
      }),
    ],
  );
}

function DownloadV2StatusActions(props) {
  const { store: vm$ } = props;
  const running_count_ = vm$.state.running_count;

  return View(
    { class: "wx-dl-page-status-actions wx-dl-v2-page-status-actions" },
    [
      DownloadV2ActionButton({
        store: vm$.ui.btn_refresh_tasks$,
        icon: "refresh-cw",
        label: "刷新",
      }),
      Show({
        when: computed(running_count_, (count) => count < MaxRunning),
        ok() {
          return DownloadV2ActionButton({
            store: vm$.ui.btn_start_all_tasks$,
            icon: "play",
            label: "全部开始",
          });
        },
      }),
      DownloadV2ActionButton({
        store: vm$.ui.btn_pause_all_tasks$,
        icon: "pause",
        label: "全部暂停",
      }),
      DownloadV2ActionButton({
        store: vm$.ui.btn_clear_tasks$,
        icon: "trash2",
        label: "清空记录",
      }),
    ],
  );
}

export function DownloadV2StatusBar(props) {
  const { store: vm$ } = props;
  return View({ class: "wx-content-toolbar-wrap wx-dl-v2-toolbar-wrap" }, [
    View({ class: "wx-content-toolbar wx-dl-v2-toolbar" }, [
      DownloadV2StatusCounts({ store: vm$ }),
      DownloadV2StatusActions({ store: vm$ }),
    ]),
  ]);
}

export function DownloadV2SelectionBar(props) {
  const { store: vm$ } = props;
  const selected_task_count_ = vm$.state.selected_task_count;

  return Show({
    when: computed(selected_task_count_, (count) => count > 0),
    ok() {
      return View(
        {
          class: "wx-dl-page-selection-bar",
          attributes: {
            role: "toolbar",
            "aria-label": "选中任务操作",
          },
        },
        [
          View({ class: "wx-dl-page-selection-summary" }, [
            computed(selected_task_count_, (count) => `已选中 ${count} 个任务`),
          ]),
          DownloadV2ActionButton({
            store: vm$.ui.btn_delete_selected_tasks$,
            icon: "trash2",
            label: computed(
              selected_task_count_,
              (count) => `删除选中 ${count}`,
            ),
          }),
        ],
      );
    },
  });
}

function DownloadV2Field(props) {
  const { control, hint, label } = props;
  return View(
    { style: FIELD_GROUP_STYLE },
    [
      View({ type: "label", style: FIELD_LABEL_STYLE }, [label]),
      control,
      hint
        ? View(
            {
              style: {
                "font-size": "12px",
                "line-height": "18px",
                color: "var(--dm-color-text-secondary)",
              },
            },
            [hint],
          )
        : null,
    ].filter(Boolean),
  );
}

function DownloadV2DialogHeading(props) {
  const { description, title } = props;
  return DialogHeader(
    {},
    [
      DialogTitle({}, [title]),
      Show({
        when: description,
        ok() {
          return DialogDescription({}, [description]);
        },
      }),
    ].filter(Boolean),
  );
}

function boolean_toggle(props) {
  const { checked: checked_, label, onToggle: on_toggle } = props;
  return View(
    {
      role: "checkbox",
      tabIndex: "0",
      attributes: {
        "aria-checked": computed(checked_, (checked) =>
          checked ? "true" : "false",
        ),
      },
      style: {
        display: "flex",
        "align-items": "center",
        gap: "10px",
        padding: "8px 0",
        cursor: "pointer",
        "user-select": "none",
        "font-size": "14px",
        "line-height": "20px",
      },
      onClick: on_toggle,
      onKeyDown(event) {
        if (event.key === " " || event.key === "Enter") {
          event.preventDefault();
          on_toggle();
        }
      },
    },
    [
      View(
        {
          style: computed(checked_, (checked) => ({
            width: "18px",
            height: "18px",
            "box-sizing": "border-box",
            "border-radius": "4px",
            border: `1px solid ${checked ? "var(--dm-color-primary-fill)" : "var(--dm-color-border)"}`,
            background: checked
              ? "var(--dm-color-primary-fill)"
              : "transparent",
            color: "var(--dm-color-on-primary)",
            display: "inline-flex",
            "align-items": "center",
            "justify-content": "center",
            "flex-shrink": "0",
          })),
        },
        [
          Show({
            when: checked_,
            ok() {
              return Timeless.Icon({ name: "check", size: 14 });
            },
          }),
        ],
      ),
      View({}, [label]),
    ],
  );
}

export function CreateTaskDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.createTaskDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "下一步",
    },
    [
      DownloadV2DialogHeading({
        title: "创建下载任务",
        description: "输入可直接访问的资源地址，确认信息后创建任务。",
      }),
      DialogBody(
        {
          style: { display: "grid", gap: "16px" },
        },
        [
          DownloadV2Field({
            label: "下载地址",
            control: Input({
              store: vm$.ui.input_create_task_url$,
              attributes: {
                type: "url",
                inputmode: "url",
                autocomplete: "off",
                "aria-label": "下载地址",
              },
            }),
          }),
          DownloadV2Field({
            label: "文件名",
            hint: "留空时由服务端根据下载地址自动识别。",
            control: Input({
              store: vm$.ui.input_create_task_filename$,
              attributes: {
                type: "text",
                autocomplete: "off",
                "aria-label": "文件名",
              },
            }),
          }),
        ],
      ),
    ],
  );
}

export function CreatePlatformTaskDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.createPlatformTaskDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "下一步",
    },
    [
      DownloadV2DialogHeading({
        title: "创建平台任务",
        description: "使用平台解析器准备资源，并在预览后创建下载任务。",
      }),
      DialogBody(
        {
          style: {
            display: "grid",
            gap: "16px",
            "max-height": "min(62vh, 560px)",
            overflow: "auto",
          },
        },
        [
          DownloadV2Field({
            label: "平台名称",
            control: Input({
              store: vm$.ui.input_create_platform$,
              attributes: {
                type: "text",
                autocomplete: "off",
                "aria-label": "平台名称",
              },
            }),
          }),
          DownloadV2Field({
            label: "内容 JSON",
            control: Textarea({
              store: vm$.ui.input_create_platform_json$,
              style: { "min-height": "112px", resize: "vertical" },
              attributes: {
                rows: "5",
                spellcheck: "false",
                "aria-label": "平台内容 JSON",
              },
            }),
          }),
          DownloadV2Field({
            label: "下载目录（可选）",
            control: Input({
              store: vm$.ui.input_create_platform_download_dir$,
              attributes: { type: "text", "aria-label": "下载目录" },
            }),
          }),
          DownloadV2Field({
            label: "文件名（可选）",
            control: Input({
              store: vm$.ui.input_create_platform_filename$,
              attributes: { type: "text", "aria-label": "文件名" },
            }),
          }),
          boolean_toggle({
            checked: vm$.state.create_platform_download_cover,
            label: "同时下载封面",
            onToggle() {
              vm$.state.create_platform_download_cover.as(
                !vm$.state.create_platform_download_cover.value,
              );
            },
          }),
        ],
      ),
    ],
  );
}

function preview_value(value, fallback = "-") {
  if (value === undefined || value === null || value === "") return fallback;
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

function resource_file_icon(name) {
  const extension = String(name || "")
    .split(".")
    .pop()
    .toLowerCase();
  if (/^(jpe?g|png|gif|webp|svg|bmp|ico)$/.test(extension)) {
    return "file-image";
  }
  if (/^(mp4|avi|mkv|mov|webm|flv|wmv|m4v)$/.test(extension)) {
    return "file-play";
  }
  if (/^(mp3|wav|aac|flac|ogg|wma|m4a)$/.test(extension)) {
    return "file-volume";
  }
  if (/^(html?|css|js|json|xml)$/.test(extension)) return "file-code";
  return "file";
}

function build_preview_tree(preview) {
  if (preview && preview.tree && typeof preview.tree === "object") {
    return preview.tree;
  }
  const resources =
    preview && Array.isArray(preview.resources) ? preview.resources : [];
  const root = { type: "directory", name: "", children: [] };

  resources.forEach((resource, index) => {
    const name = preview_value(
      resource &&
        (resource.name ||
          resource.filename ||
          resource.file_name ||
          resource.title),
      `资源 ${index + 1}`,
    );
    const parts = name.split("/").filter(Boolean);
    const file_name = parts.pop() || name;
    let parent = root;

    parts.forEach((part) => {
      let directory = parent.children.find((node) => {
        return node.type === "directory" && node.name === part;
      });
      if (!directory) {
        directory = { type: "directory", name: part, children: [] };
        parent.children.push(directory);
      }
      parent = directory;
    });
    parent.children.push({
      type: "file",
      name: file_name,
      kind: resource && resource.kind,
      endpoints: resource && resource.endpoints,
    });
  });
  return root;
}

function count_preview_tree_files(node) {
  if (!node || node.type !== "directory") return node ? 1 : 0;
  return (node.children || []).reduce((count, child) => {
    return count + count_preview_tree_files(child);
  }, 0);
}

function PreviewResourceNode(props) {
  const { level, node } = props;
  const indent = `${Math.min((Number(level) || 0) * 18, 90)}px`;
  const is_directory = node && node.type === "directory";

  if (is_directory) {
    return View({ style: { "margin-left": indent, "margin-bottom": "2px" } }, [
      View(
        {
          style: {
            display: "flex",
            "align-items": "center",
            gap: "6px",
            padding: "3px 6px",
            "border-radius": "4px",
            "font-size": "13px",
            "font-weight": "600",
            color: "var(--dm-color-text-secondary)",
          },
        },
        [
          View(
            {
              style: { width: "18px", "flex-shrink": "0" },
              attributes: { n: "preview-directory-icon" },
            },
            [Timeless.Icon({ name: "folder", size: 16 })],
          ),
          View(
            {
              style: {
                overflow: "hidden",
                "text-overflow": "ellipsis",
                "white-space": "nowrap",
              },
            },
            [node.name || "根目录"],
          ),
        ],
      ),
      View({ style: { "margin-left": "6px" } }, [
        For({
          each: node.children || [],
          render(child) {
            return PreviewResourceNode({ node: child, level: level + 1 });
          },
        }),
      ]),
    ]);
  }

  return View({ style: { "margin-left": indent, "margin-bottom": "2px" } }, [
    View(
      {
        style: {
          display: "flex",
          "align-items": "center",
          gap: "6px",
          padding: "3px 6px",
          "border-radius": "4px",
          "font-size": "13px",
          color: "var(--dm-color-text-primary)",
        },
      },
      [
        View(
          {
            style: { width: "18px", "flex-shrink": "0" },
            attributes: { n: "preview-resource-icon" },
          },
          [
            Timeless.Icon({
              name: resource_file_icon(node && node.name),
              size: 16,
            }),
          ],
        ),
        View(
          {
            style: {
              overflow: "hidden",
              "text-overflow": "ellipsis",
              "white-space": "nowrap",
            },
            attributes: { title: (node && node.name) || "文件" },
          },
          [(node && node.name) || "文件"],
        ),
      ],
    ),
  ]);
}

function PreviewDetailRow(props) {
  const { label, value } = props;
  return View(
    {
      style: {
        display: "grid",
        "grid-template-columns": "104px minmax(0, 1fr)",
        gap: "12px",
        padding: "9px 0",
        "border-bottom": "1px solid var(--dm-color-border-translucent)",
      },
    },
    [
      View({ style: FIELD_LABEL_STYLE }, [label]),
      View(
        {
          style: {
            "font-size": "14px",
            "line-height": "20px",
            "word-break": "break-all",
            color: "var(--dm-color-text-primary)",
          },
        },
        [value],
      ),
    ],
  );
}

function PreviewResourceList(props) {
  const { preview: preview_ } = props;
  const tree_ = computed(preview_, (preview) => build_preview_tree(preview));
  const tree_nodes_ = computed(tree_, (tree) => {
    return tree && Array.isArray(tree.children) ? tree.children : [];
  });
  const resource_count_ = computed(tree_, (tree) => {
    return count_preview_tree_files(tree);
  });

  return Show({
    when: computed(tree_nodes_, (nodes) => nodes.length > 0),
    ok() {
      return View(
        {
          style: {
            display: "grid",
            gap: "8px",
            padding: "12px",
            "border-radius": "10px",
            border: "1px solid var(--dm-color-border-translucent)",
            background: "var(--dm-color-bg-subtle)",
          },
        },
        [
          View({ style: FIELD_LABEL_STYLE }, [
            "资源列表（",
            computed(resource_count_, (count) => String(count)),
            " 项）",
          ]),
          For({
            each: tree_nodes_,
            render(node) {
              return PreviewResourceNode({ node, level: 0 });
            },
          }),
        ],
      );
    },
  });
}

function PreviewDialogContent(props) {
  const { platform, preview: preview_ } = props;
  return DialogBody(
    {
      style: {
        display: "grid",
        gap: "14px",
        "max-height": "min(62vh, 560px)",
        overflow: "auto",
      },
    },
    [
      View({}, [
        platform
          ? PreviewDetailRow({
              label: "平台",
              value: computed(preview_, (preview) =>
                preview_value(preview && preview.platform),
              ),
            })
          : PreviewDetailRow({
              label: "协议",
              value: computed(preview_, (preview) =>
                preview_value(preview && preview.protocol),
              ),
            }),
        PreviewDetailRow({
          label: "任务名称",
          value: computed(preview_, (preview) =>
            preview_value(preview && (preview.task_name || preview.name)),
          ),
        }),
        PreviewDetailRow({
          label: "资源类型",
          value: computed(preview_, (preview) =>
            preview_value(preview && preview.resource_type),
          ),
        }),
        PreviewDetailRow({
          label: "下载目录",
          value: computed(preview_, (preview) =>
            preview_value(preview && preview.download_dir),
          ),
        }),
      ]),
      PreviewResourceList({ preview: preview_ }),
    ],
  );
}

export function CreateTaskPreviewDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.createTaskPreviewDialog$,
      zIndex: 10001,
      style: DIALOG_STYLE,
      okText: "创建任务",
    },
    [
      DownloadV2DialogHeading({
        title: "下载任务预览",
        description: "请确认解析后的资源信息。",
      }),
      PreviewDialogContent({ preview: vm$.state.create_task_preview }),
    ],
  );
}

export function CreatePlatformTaskPreviewDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.createPlatformTaskPreviewDialog$,
      zIndex: 10001,
      style: DIALOG_STYLE,
      okText: "创建任务",
    },
    [
      DownloadV2DialogHeading({
        title: "平台任务预览",
        description: "请确认平台解析后的资源信息。",
      }),
      PreviewDialogContent({
        platform: true,
        preview: vm$.state.create_platform_preview,
      }),
    ],
  );
}

function DeleteFilesControl(props) {
  const { store: vm$ } = props;
  return boolean_toggle({
    checked: vm$.state.delete_delete_files,
    label: "同时删除已下载的文件",
    onToggle() {
      vm$.methods.handleClickCheckboxConfirmDeleteFiles();
    },
  });
}

export function TaskDeleteConfirmDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.deleteConfirmDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "删除",
    },
    [
      DownloadV2DialogHeading({
        title: "删除下载任务",
        description: computed(vm$.state.pending_delete_task_count, (count) => {
          const total = Number(count) || 1;
          return `确定删除 ${total} 个下载任务记录？此操作不可恢复。`;
        }),
      }),
      DialogBody({}, [
        DeleteFilesControl({ store: vm$ }),
        Show({
          when: vm$.state.deleting_task,
          ok() {
            return View(
              {
                style: {
                  "font-size": "12px",
                  color: "var(--dm-color-text-secondary)",
                },
              },
              ["正在删除任务..."],
            );
          },
        }),
      ]),
    ],
  );
}

export function ClearTasksConfirmDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.clearConfirmDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "清空",
    },
    [
      DownloadV2DialogHeading({
        title: "清空下载记录",
        description: computed(vm$.state.task_count, (count) => {
          return `确定清空当前的 ${Number(count) || 0} 个下载任务记录？`;
        }),
      }),
      DialogBody({}, [
        DeleteFilesControl({ store: vm$ }),
        Show({
          when: vm$.state.clearing_tasks,
          ok() {
            return View(
              {
                style: {
                  "font-size": "12px",
                  color: "var(--dm-color-text-secondary)",
                },
              },
              ["正在清空任务..."],
            );
          },
        }),
      ]),
    ],
  );
}

function OverwriteActionList(props) {
  const { store: vm$ } = props;
  const processing_ = vm$.state.overwrite_processing;
  const selected_action_ = vm$.state.overwrite;

  function select(action) {
    if (!processing_.value) vm$.methods.setOverwriteAction(action);
  }

  return View(
    {
      role: "radiogroup",
      style: { display: "grid", gap: "8px" },
    },
    [
      For({
        each: OVERWRITE_ACTION_ITEMS,
        render(item) {
          const row_state_ = combine(
            { processing: processing_, selected: selected_action_ },
            (state) => ({
              checked: state.selected && state.selected.value === item.value,
              processing: Boolean(state.processing),
            }),
          );
          return View(
            {
              role: "radio",
              tabIndex: "0",
              attributes: {
                "aria-checked": computed(row_state_, (state) =>
                  state.checked ? "true" : "false",
                ),
                "aria-disabled": computed(row_state_, (state) =>
                  state.processing ? "true" : undefined,
                ),
              },
              style: computed(row_state_, (state) => ({
                display: "flex",
                "align-items": "center",
                gap: "12px",
                padding: "11px 12px",
                "border-radius": "10px",
                border: `1px solid ${state.checked ? "var(--dm-color-primary-fill)" : "var(--dm-color-border)"}`,
                background: state.checked
                  ? "color-mix(in srgb, var(--dm-color-primary-fill) 10%, transparent)"
                  : "transparent",
                cursor: state.processing ? "wait" : "pointer",
                opacity: state.processing ? "0.72" : "1",
                "user-select": "none",
              })),
              onClick() {
                select(item.value);
              },
              onKeyDown(event) {
                if (event.key === " " || event.key === "Enter") {
                  event.preventDefault();
                  select(item.value);
                }
              },
            },
            [
              View(
                {
                  style: computed(row_state_, (state) => ({
                    width: "30px",
                    height: "30px",
                    "border-radius": "50%",
                    background: state.checked
                      ? "var(--dm-color-primary-fill)"
                      : "var(--dm-color-bg-subtle)",
                    color: state.checked
                      ? "var(--dm-color-on-primary)"
                      : "var(--dm-color-text-secondary)",
                    display: "inline-flex",
                    "align-items": "center",
                    "justify-content": "center",
                    "flex-shrink": "0",
                  })),
                },
                [Timeless.Icon({ name: item.icon, size: 16 })],
              ),
              View({ style: { "min-width": "0", flex: "1 1 auto" } }, [
                View(
                  {
                    style: {
                      "font-size": "14px",
                      "font-weight": "600",
                      "line-height": "20px",
                    },
                  },
                  [item.label],
                ),
                View(
                  {
                    style: {
                      "font-size": "12px",
                      "line-height": "18px",
                      color: "var(--dm-color-text-secondary)",
                      "margin-top": "2px",
                    },
                  },
                  [item.description],
                ),
              ]),
              Show({
                when: computed(row_state_, (state) => state.checked),
                ok() {
                  return Timeless.Icon({ name: "check", size: 18 });
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function OverwriteConflictCard(props) {
  const { store: vm$ } = props;
  const conflict_ = vm$.state.overwrite_conflict;

  return Show({
    when: computed(conflict_, (conflict) => Boolean(conflict && conflict.name)),
    ok() {
      return View(
        {
          style: {
            display: "flex",
            "align-items": "center",
            gap: "10px",
            padding: "10px 12px",
            "border-radius": "10px",
            background: "var(--dm-color-bg-subtle)",
            border: "1px solid var(--dm-color-border-translucent)",
          },
        },
        [
          Timeless.Icon({ name: "circle-alert", size: 18 }),
          View(
            {
              style: {
                "min-width": "0",
                flex: "1 1 auto",
                overflow: "hidden",
                "text-overflow": "ellipsis",
                "white-space": "nowrap",
                "font-size": "13px",
                "font-weight": "600",
              },
              attributes: {
                title: computed(conflict_, (conflict) => conflict.name || ""),
              },
            },
            [computed(conflict_, (conflict) => conflict.name || "")],
          ),
          Show({
            when: computed(conflict_, (conflict) => Number(conflict.total) > 1),
            ok() {
              return View(
                {
                  style: {
                    "font-size": "12px",
                    color: "var(--dm-color-text-secondary)",
                    "white-space": "nowrap",
                  },
                },
                [
                  computed(conflict_, (conflict) => {
                    return `${conflict.index || 1}/${conflict.total || 1}`;
                  }),
                ],
              );
            },
          }),
        ],
      );
    },
  });
}

function OverwriteApplyAllControl(props) {
  const { store: vm$ } = props;
  return boolean_toggle({
    checked: vm$.state.overwrite_apply_all,
    label: "将此选择应用给本批次的所有冲突任务",
    onToggle() {
      if (!vm$.state.overwrite_processing.value) {
        vm$.methods.toggleOverwriteApplyAll();
      }
    },
  });
}

function OverwriteDialogBody(props) {
  const { batch, store: vm$ } = props;
  return DialogBody(
    { style: { display: "grid", gap: "14px" } },
    [
      OverwriteConflictCard({ store: vm$ }),
      OverwriteActionList({ store: vm$ }),
      batch ? OverwriteApplyAllControl({ store: vm$ }) : null,
      Show({
        when: vm$.state.overwrite_processing,
        ok() {
          return View(
            {
              role: "status",
              style: {
                display: "flex",
                "align-items": "center",
                gap: "8px",
                "font-size": "12px",
                color: "var(--dm-color-text-secondary)",
              },
            },
            [
              View({
                class: "dm-ui-spinner",
                attributes: { "aria-hidden": "true" },
              }),
              "正在处理冲突...",
            ],
          );
        },
      }),
    ].filter(Boolean),
  );
}

export function OverwriteConfirmDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.overwriteConfirmDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "继续",
    },
    [
      DownloadV2DialogHeading({
        title: "文件已存在",
        description: "已存在相同下载内容，请选择本次创建方式。",
      }),
      OverwriteDialogBody({ store: vm$, batch: false }),
    ],
  );
}

export function SingleOverwriteConfirmDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.singleOverwriteConfirmDialog$,
      zIndex: 10000,
      style: DIALOG_STYLE,
      okText: "继续",
    },
    [
      DownloadV2DialogHeading({
        title: "已存在确认",
        description: "请选择如何处理当前冲突任务。",
      }),
      OverwriteDialogBody({ store: vm$, batch: false }),
    ],
  );
}

export function BatchOverwriteConfirmDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.batchOverwriteConfirmDialog$,
      zIndex: 10001,
      style: DIALOG_STYLE,
      okText: "继续",
    },
    [
      DownloadV2DialogHeading({
        title: "批量任务冲突",
        description: "当前批次中存在相同下载内容，请选择处理方式。",
      }),
      OverwriteDialogBody({ store: vm$, batch: true }),
    ],
  );
}

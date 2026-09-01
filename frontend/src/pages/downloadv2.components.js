import { Checkbox, createCheckboxStore } from "../dmui.js";
import {
  MaxRunning,
  format_download_percent,
  format_download_size,
  format_download_speed,
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
    icon,
    iconSize: icon_size,
    label,
    store,
    title,
  } = props;
  const semantic_name = props.name || "download-action";
  return Button(
    {
      store,
      class: "dm-button--toolbar",
      attributes: {
        n: semantic_name,
        type: "button",
        title: title || "",
        ...(attributes || {}),
      },
      prefix: icon
        ? Timeless.Icon({
            name: icon,
            size: icon_size || 16,
            attributes: { n: `${semantic_name}-icon` },
          })
        : null,
    },
    typeof label !== "undefined" ? [label] : [],
  );
}

function DownloadV2Number(props = {}) {
  const {
    attributes,
    characterWidth: character_width = "0.65em",
    class: extra_class,
    decimalWidth: decimal_width = "0.3em",
    number,
    separatorWidth: separator_width = "0.4em",
    spaceWidth: space_width = "0.25em",
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
      class: ["number-view", extra_class].filter(Boolean).join(" "),
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
              class: "number-view-character",
              style: {
                width,
                "min-width": width,
                "flex-basis": width,
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
      class: "number-view-infinity",
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
  const { attributes, class: extra_class } = props;
  return View({
    class: ["download-skeleton", extra_class].filter(Boolean).join(" "),
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
      let status_color = "var(--dm-color-text-secondary)";
      let error_text = "";
      let progress_text = "";
      let speed_text = "";

      if (is_running) {
        speed_text = format_download_speed(progress.speed);
        status_text = "下载中";
        progress_text = is_live_stream ? "" : format_progress_text(percent);
        status_color = "var(--dm-color-primary)";
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
        status_text = is_live_stream ? "录制已中断" : "已暂停";
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

function DownloadV2TaskCover(props) {
  const { state: state_, store: vm$, task: task$ } = props;
  const raw_ = task$.state.raw;
  const progress = Show({
    when: computed(
      state_,
      (state) => !state.is_live_stream && (state.is_running || state.is_paused),
    ),
    ok() {
      return View({
        class: "dl-page-task-cover-progress",
        attributes: { n: "download-task-cover-progress" },
      }, [
        DownloadV2Number({
          value: computed(state_, (state) => state.progress_text),
        }),
      ]);
    },
  });
  const fallback = () =>
    View({
      class: "dl-page-task-cover dl-page-task-cover-fallback",
      attributes: {
        n: "download-task-cover-fallback",
        "aria-hidden": "true",
      },
    });

  return Show({
    when: computed(raw_, (raw) => Boolean(vm$.methods.taskCoverURL(raw))),
    ok() {
      return View({
        class: "dl-page-task-cover-wrap",
        attributes: { n: "download-task-cover-wrap" },
      }, [
        fallback(),
        LazyImg({
          class: "dl-page-task-cover",
          src: computed(raw_, vm$.methods.taskCoverURL),
          alt: task$.state.name,
          attributes: {
            n: "download-task-cover-image",
            loading: "lazy",
            referrerpolicy: "no-referrer",
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
        "dl-page-task-action",
        danger ? "dl-page-task-action-danger" : "",
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

export function DownloadV2TaskActions(props) {
  const { store: vm$, task: task$ } = props;
  const state_ = DownloadV2TaskState(task$);
  const is_open_external = is_download_open_external();

  return [
    Match({
      when: combine(
        { state: state_, running_count: vm$.state.running_count },
        (value) => {
          if (value.state.is_completed) return 1;
          if (value.state.is_running) return 2;
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
              class: "dl-page-task-action",
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
            title: state_.value.is_live_stream ? "恢复录制" : "继续",
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
          return View({ class: "dl-page-task-action-spacer" });
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

export function DownloadV2TaskMain(props) {
  const { store: vm$ } = props;
  const task$ = task_value(props.task);
  const state_ = DownloadV2TaskState(task$);

  return [
    DownloadV2TaskCover({ task: task$, state: state_, store: vm$ }),
    View(
      {
        class: "dl-page-task-info",
        attributes: { n: "download-task-info" },
      },
      [
        View(
          {
            class: "dl-page-task-title-line",
            attributes: { n: "download-task-title-line" },
          },
          [
            View(
              {
                as: "button",
                class: "dl-page-task-title",
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
                    class: "dl-page-task-live",
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
                      "dl-page-task-missing-detail dm-badge dm-badge--warning",
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
            class: "dl-page-task-desc",
            attributes: { n: "download-task-description" },
          },
          [
            View(
              {
                class: "dl-page-task-status",
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
                class: "dl-page-task-error",
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

export function DownloadV2TaskSkeletonRow() {
  return View(
    {
      class:
        "dm-table-row dm-grid dm-items-center dl-page-task-row dl-page-task-skeleton",
      attributes: { n: "download-task-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-table-selection-cell dm-flex dm-items-center dm-justify-center dm-min-w-0",
          attributes: { n: "download-task-skeleton-selection", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            class: "dl-skeleton-checkbox",
            attributes: { n: "download-task-skeleton-checkbox" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell dl-page-task-main-cell",
          attributes: { n: "download-task-skeleton-main", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            class: "dl-skeleton-cover",
            attributes: { n: "download-task-skeleton-cover" },
          }),
          View(
            {
              class: "dl-page-task-info",
              attributes: { n: "download-task-skeleton-info" },
            },
            [
              DownloadV2Skeleton({
                class: "dl-skeleton-line dl-skeleton-title",
                attributes: { n: "download-task-skeleton-title" },
              }),
              DownloadV2Skeleton({
                class: "dl-skeleton-line dl-skeleton-status",
                attributes: { n: "download-task-skeleton-status" },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: {
            n: "download-task-skeleton-created-at",
            role: "cell",
          },
        },
        [
          DownloadV2Skeleton({
            class: "dl-skeleton-created-at",
            attributes: { n: "download-task-skeleton-created-at-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell dl-page-task-actions-cell",
          attributes: { n: "download-task-skeleton-actions", role: "cell" },
        },
        [
          DownloadV2Skeleton({
            class: "dl-skeleton-action",
            attributes: { n: "download-task-skeleton-primary-action" },
          }),
          DownloadV2Skeleton({
            class: "dl-skeleton-action",
            attributes: { n: "download-task-skeleton-delete-action" },
          }),
        ],
      ),
    ],
  );
}

function DownloadV2StatusTab(vm$, status, label) {
  const active_status_ = vm$.state.active_status;
  const status_counts_ = vm$.state.status_counts;

  return View(
    {
      type: "button",
      attributes: {
        n: `download-status-${status}-tab`,
        type: "button",
        "aria-pressed": computed(active_status_, (active_status) =>
          active_status === status ? "true" : "false",
        ),
      },
      class: computed(active_status_, (active_status) =>
        [
          "dl-v2-tab dm-focus-ring",
          active_status === status ? "dl-v2-tab-active" : "",
          status === "error" ? "dl-v2-tab-error" : "",
        ]
          .filter(Boolean)
          .join(" "),
      ),
      onClick() {
        vm$.methods.setStatusFilter(status);
      },
    },
    [
      View(
        {
          class: "dl-v2-tab-label",
          attributes: { n: `download-status-${status}-label` },
        },
        [label],
      ),
      View(
        {
          class: "dl-v2-tab-count",
          attributes: { n: `download-status-${status}-count` },
        },
        [
          computed(status_counts_, (counts) => {
            return String(Number(counts[status]) || 0);
          }),
        ],
      ),
    ],
  );
}

function DownloadV2StatusCounts(props) {
  const { store: vm$ } = props;

  return View(
    {
      class: "dl-page-counts dl-v2-page-counts",
      attributes: {
        n: "download-status-tabs",
        role: "group",
        "aria-label": "Download status filters",
      },
    },
    [
      DownloadV2StatusTab(vm$, "total", "全部"),
      DownloadV2StatusTab(vm$, "running", "下载中"),
      DownloadV2StatusTab(vm$, "pause", "暂停"),
      DownloadV2StatusTab(vm$, "wait", "等待中"),
      DownloadV2StatusTab(vm$, "done", "已完成"),
      DownloadV2StatusTab(vm$, "error", "失败"),
    ],
  );
}

function DownloadV2StatusActions(props) {
  const { store: vm$ } = props;
  const running_count_ = vm$.state.running_count;

  return View(
    { class: "dl-page-status-actions dl-v2-page-status-actions" },
    [
      DownloadV2ActionButton({
        name: "download-refresh-action",
        store: vm$.ui.btn_refresh_tasks$,
        icon: "refresh-cw",
        label: "刷新",
      }),
      Show({
        when: computed(running_count_, (count) => count < MaxRunning),
        ok() {
          return DownloadV2ActionButton({
            name: "download-start-all-action",
            store: vm$.ui.btn_start_all_tasks$,
            icon: "play",
            label: "全部开始",
          });
        },
      }),
      DownloadV2ActionButton({
        name: "download-pause-all-action",
        store: vm$.ui.btn_pause_all_tasks$,
        icon: "pause",
        label: "全部暂停",
      }),
      DownloadV2ActionButton({
        name: "download-clear-action",
        store: vm$.ui.btn_clear_tasks$,
        icon: "trash2",
        label: "清空记录",
      }),
    ],
  );
}

export function DownloadV2StatusBar(props) {
  const { store: vm$ } = props;
  return View({ class: "content-toolbar-wrap dl-v2-toolbar-wrap" }, [
    View({ class: "content-toolbar dl-v2-toolbar" }, [
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
          class: "dl-page-selection-bar",
          attributes: {
            role: "toolbar",
            "aria-label": "选中任务操作",
          },
        },
        [
          View({ class: "dl-page-selection-summary" }, [
            computed(selected_task_count_, (count) => `已选中 ${count} 个任务`),
          ]),
          DownloadV2ActionButton({
            name: "download-delete-selected-action",
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
    { class: "dl-dialog-field" },
    [
      View({ type: "label", class: "dl-dialog-field-label" }, [label]),
      control,
      hint
        ? View(
            {
              class: "dl-dialog-field-hint",
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
  const { checked: checked_, label, name, onToggle: on_toggle } = props;
  const checkbox_store = createCheckboxStore({
    checked: checked_,
    onChange: on_toggle,
  });
  return View(
    {
      class: "dl-checkbox-control",
      attributes: { n: `${name}-control` },
      onClick() {
        checkbox_store.toggle();
      },
    },
    [
      Checkbox({
        store: checkbox_store,
        attributes: {
          n: `${name}-checkbox`,
          "aria-label": label,
        },
        onClick(event) {
          event.stopPropagation();
        },
      }),
      View(
        {
          as: "span",
          attributes: { n: `${name}-label` },
        },
        [label],
      ),
    ],
  );
}

export function CreateTaskDialog(props) {
  const { store: vm$ } = props;
  return Dialog(
    {
      store: vm$.ui.createTaskDialog$,
      zIndex: 10000,
      class: "dm-dialog--form",
      okText: "下一步",
    },
    [
      DownloadV2DialogHeading({
        title: "创建下载任务",
        description: "输入可直接访问的资源地址，确认信息后创建任务。",
      }),
      DialogBody(
        {
          class: "dl-dialog-fields",
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
      class: "dm-dialog--form",
      okText: "下一步",
    },
    [
      DownloadV2DialogHeading({
        title: "创建平台任务",
        description: "使用平台解析器准备资源，并在预览后创建下载任务。",
      }),
      DialogBody(
        {
          class: "dl-dialog-fields dm-dialog-body--scrollable",
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
              class: "dm-textarea--tall",
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
            name: "create-platform-download-cover",
            onToggle(checked) {
              vm$.state.create_platform_download_cover.as(checked);
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
    return View({
      class: "dl-preview-tree-item",
      style: { "margin-left": indent },
    }, [
      View(
        {
          class: "dl-preview-tree-row is-directory",
        },
        [
          View(
            {
              class: "dl-preview-tree-icon",
              attributes: { n: "preview-directory-icon" },
            },
            [Timeless.Icon({ name: "folder", size: 16 })],
          ),
          View(
            {
              class: "dm-truncate",
            },
            [node.name || "根目录"],
          ),
        ],
      ),
      View({ class: "dl-preview-tree-children" }, [
        For({
          each: node.children || [],
          render(child) {
            return PreviewResourceNode({ node: child, level: level + 1 });
          },
        }),
      ]),
    ]);
  }

  return View({
    class: "dl-preview-tree-item",
    style: { "margin-left": indent },
  }, [
    View(
      {
        class: "dl-preview-tree-row",
      },
      [
        View(
          {
            class: "dl-preview-tree-icon",
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
            class: "dm-truncate",
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
      class: "dl-preview-detail-row",
    },
    [
      View({ class: "dl-dialog-field-label" }, [label]),
      View(
        {
          class: "dl-preview-detail-value",
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
          class: "dl-preview-resource-list",
        },
        [
          View({ class: "dl-dialog-field-label" }, [
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
      class: "dm-grid dm-gap-3 dm-dialog-body--scrollable",
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
      class: "dm-dialog--form",
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
      class: "dm-dialog--form",
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
    name: "delete-downloaded-files",
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
      class: "dm-dialog--form",
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
                class: "dm-text-xs dm-text-muted",
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
      class: "dm-dialog--form",
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
                class: "dm-text-xs dm-text-muted",
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
      class: "dm-choice-list",
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
              class: computed(row_state_, (state) =>
                [
                  "dm-choice-row",
                  state.checked ? "is-selected" : "",
                  state.processing ? "is-processing" : "",
                ]
                  .filter(Boolean)
                  .join(" "),
              ),
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
                  class: "dm-choice-row__icon",
                },
                [Timeless.Icon({ name: item.icon, size: 16 })],
              ),
              View({ class: "dm-min-w-0 dm-flex-1" }, [
                View(
                  {
                    class: "dm-choice-row__title",
                  },
                  [item.label],
                ),
                View(
                  {
                    class: "dm-choice-row__description",
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
          class: "dm-notice-row",
        },
        [
          Timeless.Icon({ name: "circle-alert", size: 18 }),
          View(
            {
              class: "dm-truncate dm-flex-1 dm-text-sm dm-font-semibold",
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
                  class: "dm-text-xs dm-text-muted",
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
    name: "overwrite-apply-all",
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
    { class: "dm-grid dm-gap-3" },
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
              class: "dm-status-inline",
            },
            [
              View({
                class: "dm-spinner",
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
      class: "dm-dialog--form",
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
      class: "dm-dialog--form",
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
      class: "dm-dialog--form",
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

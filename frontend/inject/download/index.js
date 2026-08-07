/// <reference path="../utils.js" />
/// <reference path="../file.js" />
/// <reference path="model.js" />
/// <reference path="view.js" />
/**
 * @file Download manager page entry
 */
function DownloadPageActionButton(props) {
  return View(
    {
      type: "button",
      class: [
        "wx-dl-page-action",
        props.compact ? "wx-dl-page-action-compact" : "",
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
        ? View({ class: "wx-dl-page-action-label" }, [props.label])
        : null,
    ].filter(Boolean),
  );
}

function DownloadPageStatusCounts(props) {
  const vm$ = props.store;
  const status_counts_ = vm$.state.status_counts;
  const active_status_ = vm$.state.active_status;
  return View({ class: "wx-dl-page-counts" }, [
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
            class: computed(active_status_, (status) => {
              return [
                "wx-dl-page-count",
                "wx-dl-page-count-filter",
                status === item.key ? "wx-dl-page-count-active" : "",
                item.key === "error" ? "wx-dl-page-count-error" : "",
              ]
                .filter(Boolean)
                .join(" ");
            }),
            onClick() {
              vm$.methods.setStatusFilter(item.key);
            },
          },
          [
            View({ class: "wx-dl-page-count-label" }, [item.label]),
            View({ class: "wx-dl-page-count-value" }, [
              computed(status_counts_, (counts) => {
                return String(get_download_status_count(counts, item));
              }),
            ]),
          ],
        );
      },
    }),
  ]);
}

function DownloadPageStatusActions(props) {
  const vm$ = props.store;
  const running_count_ = vm$.state.running_count;
  return View({ class: "wx-dl-page-status-actions" }, [
    Show({
      when: computed(running_count_, function (t) {
        var maxRunning =
          WXEnv.config.MaxRunning || WXEnv.defaults.MaxRunning;
        return t < maxRunning;
      }),
      ok: function () {
        return DownloadPageActionButton({
          icon: "play",
          label: "全部开始",
          onClick: function () {
            vm$.methods.startAllTasks();
          },
        });
      },
    }),
    DownloadPageActionButton({
      icon: "pause",
      label: "全部暂停",
      onClick() {
        vm$.methods.pauseAllTasks();
      },
    }),
  ]);
}

function DownloadPageTaskValue(task_) {
  return task_ && task_.value !== undefined ? task_.value : task_;
}

function DownloadPageTaskState(task_) {
  return computed(task_, (source) => {
    const task = source || {};
    const pr = format_download_percent(task);
    const isLiveStream = is_live_stream_download_task(task);
    const normalizedStatus = normalize_download_status(task.status);
    const isPaused = normalizedStatus === "pause";
    const isRunning = normalizedStatus === "running";
    const isFailed = normalizedStatus === "error";
    const isPending = is_download_waiting_status(normalizedStatus);
    const isCompleted =
      normalizedStatus === "done" ||
      (pr === 100 && !isRunning && !isFailed && !isPaused && !isPending);

    const files = Array.isArray(task.files) ? task.files : [];
    const deletedFileCount = files.filter((file) => {
      return String((file && file.status) || "").toLowerCase() === "deleted";
    }).length;
    const hasDeletedFiles = deletedFileCount > 0;
    const allFilesDeleted =
      files.length > 0 && deletedFileCount === files.length;
    const filesDownloadedSize = files.reduce(
      (sum, file) => sum + (Number(file && file.downloaded) || 0),
      0,
    );
    const filesTotalSize = files.reduce(
      (sum, file) => sum + (Number(file && file.size) || 0),
      0,
    );
    const downloadedSize = Math.max(
      filesDownloadedSize,
      Number(task.downloaded) || 0,
    );
    const totalFileSize = Math.max(filesTotalSize, Number(task.size) || 0);

    let statusText = task.status;
    let statusColor = "var(--wx-dl-page-muted)";
    let errorText = "";
    let progressText = "";
    let speedText = "";
    if (isRunning) {
      speedText = format_download_speed(
        task.speed ||
          (task.progress && typeof task.progress === "object"
            ? task.progress.speed
            : 0),
      );
      statusText = "下载中";
      progressText = format_download_progress_text(pr);
      statusColor = "var(--wx-dl-page-primary)";
    } else if (isCompleted) {
      statusText = hasDeletedFiles
        ? allFilesDeleted
          ? "文件已删除"
          : "部分文件已删除"
        : "已完成";
      statusColor = hasDeletedFiles ? "#FA5151" : "#07C160";
    } else if (isFailed) {
      statusText = "失败";
      errorText = task.error || task._errMsg || "下载失败";
      statusColor = "#FA5151";
    } else if (isPending) {
      statusText = "等待中...";
    } else if (isPaused) {
      statusText = "已暂停";
      statusColor = "#FBC02D";
      progressText = format_download_progress_text(pr);
    }

    return {
      pr,
      isLiveStream,
      isCompleted,
      isPaused,
      isRunning,
      isFailed,
      isPending,
      statusText,
      statusColor,
      errorText,
      progressText,
      speedText,
      downloadedSizeText: format_download_size(downloadedSize),
      totalSizeText: format_download_size(totalFileSize),
    };
  });
}

function DownloadPageTaskCoverURL(task) {
  if (!task || typeof task !== "object") {
    return "";
  }
  return String(task.cover_url || task.coverUrl || task.CoverURL || "").trim();
}

function DownloadPageTaskCoverOpacity(state) {
  if (!state || state.isCompleted) {
    return "1";
  }
  if (
    state.isRunning ||
    state.isPaused ||
    state.isPending ||
    state.isFailed
  ) {
    const progress = Number(state.pr) || 0;
    return String(Math.max(0.04, Math.min(1, progress / 100)));
  }
  return "1";
}

function DownloadPageTaskCover(props) {
  const task_ = props.task;
  const state_ = props.state;
  const fallback = View(
    { class: "wx-dl-page-task-cover wx-dl-page-task-cover-fallback" },
    [],
  );
  const progress = Show({
    when: computed(state_, (state) => state.isRunning || state.isPaused),
    ok() {
      return View({ class: "wx-dl-page-task-cover-progress" }, [
        NumberView({
          value: computed(state_, (state) =>
            format_download_progress_text(state.pr),
          ),
        }),
      ]);
    },
  });

  return Show({
    when: computed(task_, (task) => !!DownloadPageTaskCoverURL(task)),
    ok() {
      return View({ class: "wx-dl-page-task-cover-wrap" }, [
        fallback,
        Img({
          class: "wx-dl-page-task-cover",
          src: computed(task_, (task) => DownloadPageTaskCoverURL(task)),
          alt: computed(task_, (task) => (task && task.name) || ""),
          style: computed(state_, (state) => ({
            opacity: DownloadPageTaskCoverOpacity(state),
          })),
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
    else() {
      return View({ class: "wx-dl-page-task-cover-wrap" }, [
        fallback,
        progress,
      ]);
    },
  });
}

function DownloadPageTaskActionButton(props) {
  return View(
    {
      type: "button",
      class: [
        "wx-dl-page-task-action",
        props.danger ? "wx-dl-page-task-action-danger" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: props.title || "",
        "aria-label": props.title || "",
      },
      onClick: props.onClick,
    },
    [Timeless.Icon({ name: props.icon, size: props.iconSize || 18 })],
  );
}

function DownloadPageTaskActions(props) {
  const vm$ = props.store;
  const task_ = props.task;
  const state_ = props.state;
  const isOpenExternal =
    WXEnv.config.remoteServerEnabled === true || WXEnv.config.inDocker === true;

  return View({ class: "wx-dl-page-task-actions-cell" }, [
    Match({
      when: combine(
        {
          state: state_,
          running_count: vm$.state.running_count,
        },
        (value) => {
          const maxRunning = WXEnv.config.MaxRunning || WXEnv.defaults.MaxRunning;
          if (value.state.isCompleted) return 1;
          if (value.state.isRunning) return 2;
          if (value.state.isLiveStream && value.state.isPaused) return 5;
          if (value.running_count >= maxRunning) return 5;
          if (value.state.isPaused) return 3;
          if (value.state.isFailed) return 4;
          return 0;
        },
      ),
      cases: {
        0() {
          return DownloadPageTaskActionButton({
            icon: "play",
            title: "开始",
            onClick() {
              vm$.methods.startTask(DownloadPageTaskValue(task_));
            },
          });
        },
        1() {
          return DownloadPageTaskActionButton({
            icon: isOpenExternal ? "file-symlink" : "folder",
            title: isOpenExternal ? "打开链接" : "打开文件夹",
            onClick() {
              vm$.methods.openTask(DownloadPageTaskValue(task_));
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
                  state.isLiveStream ? "停止录制" : "暂停",
                ),
                "aria-label": computed(state_, (state) =>
                  state.isLiveStream ? "停止录制" : "暂停",
                ),
              },
              onClick() {
                vm$.methods.pauseTask(DownloadPageTaskValue(task_), {
                  liveStream: state_.value.isLiveStream,
                });
              },
            },
            [
              Show({
                when: computed(state_, (state) => state.isLiveStream),
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
          return DownloadPageTaskActionButton({
            icon: "play",
            title: "继续",
            onClick() {
              vm$.methods.resumeTask(DownloadPageTaskValue(task_));
            },
          });
        },
        4() {
          return DownloadPageTaskActionButton({
            icon: "refresh-ccw",
            title: "重试",
            onClick() {
              vm$.methods.retryTask(DownloadPageTaskValue(task_));
            },
          });
        },
        5() {
          return View({ class: "wx-dl-page-task-action-spacer" });
        },
      },
    }),
    DownloadPageTaskActionButton({
      icon: "trash2",
      title: "删除",
      danger: true,
      onClick() {
        vm$.methods.requestDeleteTask(DownloadPageTaskValue(task_));
      },
    }),
  ]);
}

function DownloadPageTaskRow(props) {
  const vm$ = props.store;
  const task_ = props.task;
  const state_ = DownloadPageTaskState(task_);
  const taskId = (DownloadPageTaskValue(task_) || {}).id;
  const selected_ = computed(vm$.state.selected_task_ids, (ids) => {
    return (ids || []).some((id) => id === taskId);
  });

  return View({ class: "wx-dl-page-task-row" }, [
    View({ class: "wx-dl-page-task-main-cell" }, [
      DownloadTaskSelectionCheckbox({
        checked: selected_,
        ariaLabel: "选择下载任务",
        style: {
          "margin-right": "10px",
        },
        onToggle(e) {
          vm$.methods.toggleTaskSelected(DownloadPageTaskValue(task_), {
            shiftKey: !!(e && e.shiftKey),
          });
        },
      }),
      DownloadPageTaskCover({ task: task_, state: state_ }),
      View({ class: "wx-dl-page-task-info" }, [
        View({ class: "wx-dl-page-task-title-line" }, [
          View(
            {
              type: "a",
              class: "wx-dl-page-task-title",
              attributes: {
                href: computed(task_, (task) => download_task_preview_url(task)),
                title: computed(task_, (task) => (task && task.name) || ""),
                target: "_blank",
                rel: "noopener noreferrer",
              },
              style: {
                cursor: "pointer",
                "text-decoration": "none",
              },
              onClick(event) {
                if (event && typeof event.preventDefault === "function") {
                  event.preventDefault();
                }
                open_download_task_preview(DownloadPageTaskValue(task_));
              },
            },
            [computed(task_, (task) => (task && task.name) || "")],
          ),
          Show({
            when: computed(state_, (state) => state.isLiveStream),
            ok() {
              return View({ class: "wx-dl-page-task-live" }, ["直播"]);
            },
          }),
        ]),
        View({ class: "wx-dl-page-task-desc" }, [
          View(
            {
              class: "wx-dl-page-task-status",
              style: computed(state_, (state) => ({ color: state.statusColor })),
            },
            [computed(state_, (state) => state.statusText)],
          ),
          "·",
          Show({
            when: computed(state_, (state) => state.isCompleted),
            ok() {
              return NumberView({
                value: computed(state_, (state) => state.totalSizeText),
              });
            },
            else() {
              return [
                NumberView({
                  value: computed(
                    state_,
                    (state) => `${state.downloadedSizeText} /`,
                  ),
                }),
                Show({
                  when: computed(state_, (state) => state.isLiveStream),
                  ok() {
                    return DownloadInfinityIcon({ size: 14 });
                  },
                  else() {
                    return NumberView({
                      value: computed(state_, (state) => state.totalSizeText),
                    });
                  },
                }),
              ];
            },
          }),
          Show({
            when: computed(state_, (state) => state.isRunning && !!state.speedText),
            ok() {
              return [
                "·",
                NumberView({
                  value: computed(state_, (state) => state.speedText),
                }),
              ];
            },
          }),
        ]),
        Show({
          when: computed(state_, (state) => state.isFailed && !!state.errorText),
          ok() {
            return View(
              {
                class: "wx-dl-page-task-error",
                attributes: {
                  title: computed(state_, (state) => state.errorText),
                },
              },
              [computed(state_, (state) => state.errorText)],
            );
          },
        }),
      ]),
    ]),
    DownloadPageTaskActions({ store: vm$, task: task_, state: state_ }),
  ]);
}

function DownloadPageTaskSkeletonRow() {
  return View({ class: "wx-dl-page-task-row wx-dl-page-task-skeleton" }, [
    View({ class: "wx-dl-page-task-main-cell" }, [
      Skeleton({
        style: {
          width: "18px",
          height: "18px",
          "border-radius": "4px",
          "margin-right": "10px",
        },
      }),
      Skeleton({
        style: {
          width: "52px",
          height: "52px",
          "border-radius": "6px",
          "margin-right": "12px",
        },
      }),
      View({ class: "wx-dl-page-task-info" }, [
        Skeleton({
          class: "wx-dl-skeleton-line",
          style: {
            width: "56%",
            height: "14px",
            "border-radius": "5px",
          },
        }),
        Skeleton({
          class: "wx-dl-skeleton-line",
          style: {
            width: "36%",
            height: "12px",
            "border-radius": "5px",
            "margin-top": "7px",
          },
        }),
      ]),
    ]),
    View({ class: "wx-dl-page-task-actions-cell" }, [
      Skeleton({
        style: {
          width: "34px",
          height: "34px",
          "border-radius": "8px",
        },
      }),
      Skeleton({
        style: {
          width: "34px",
          height: "34px",
          "border-radius": "8px",
        },
      }),
    ]),
  ]);
}

function DownloadPageTaskListView(props) {
  const vm$ = props.store;
  const tasks_ = vm$.state.tasks;

  return View({ class: "wx-dl-page-list wx-dl-dark-scroll" }, [
    Show({
      when: computed(tasks_, (items) => items.length > 0),
      ok() {
        return Show({
          when: vm$.state.list_render_enabled,
          ok() {
            const listHeightStyle = vm$.state.fixed_list_height
              ? {
                  height: `${vm$.state.list_height}px`,
                  "max-height": `${vm$.state.list_height}px`,
                }
              : {
                  "max-height": "100%",
                };
            return [
              VirtualListView({
                style: {
                  ...listHeightStyle,
                  overflow: "auto",
                  position: "relative",
                  padding: "0",
                  "box-sizing": "border-box",
                  "background-color": "transparent",
                },
                key: "id",
                size: props.size || 12,
                buffer: vm$.state.list_buffer,
                gutter: 0,
                itemHeight: vm$.state.list_item_height,
                paddingBottom: 0,
                each: tasks_,
                onMounted(e) {
                  vm$.methods.setListViewElement(e);
                },
                onScroll(pos) {
                  vm$.methods.handleListViewScroll(pos);
                },
                render(task_) {
                  const task = DownloadPageTaskValue(task_);
                  if (vm$.methods.isPlaceholderTask(task)) {
                    vm$.methods.ensureTaskPageForIndex(task.__index);
                    return DownloadPageTaskSkeletonRow();
                  }
                  return DownloadPageTaskRow({ store: vm$, task: task_ });
                },
              }),
            ];
          },
        });
      },
      else() {
        return View({ class: "wx-dl-page-empty" }, ["暂无下载任务"]);
      },
    }),
  ]);
}

function DownloadPageTaskTableView(props) {
  return View({ class: "wx-dl-page-task-table" }, [
    View({ class: "wx-dl-page-table-head" }, [
      View({ class: "wx-dl-page-table-head-cell" }, ["下载任务"]),
      View(
        { class: "wx-dl-page-table-head-cell wx-dl-page-table-head-action" },
        ["操作"],
      ),
    ]),
    DownloadPageTaskListView({ store: props.store }),
  ]);
}

function DownloadPageTopBar(props) {
  const vm$ = props.store;
  const task_count_ = vm$.state.task_count;
  const selected_task_count_ = vm$.state.selected_task_count;
  return View({ class: "wx-dl-page-topbar" }, [
    View({ class: "wx-dl-page-topbar-inner" }, [
      View({ class: "wx-dl-page-brand" }, [
        View({ class: "wx-dl-page-brand-icon" }, [
          Timeless.Icon({ name: "download", size: 24 }),
        ]),
        View({ class: "wx-dl-page-heading" }, [
          View({ class: "wx-dl-page-title" }, ["下载任务"]),
          View({ class: "wx-dl-page-subtitle" }, [
            computed(
              task_count_,
              (count) => `管理已创建的 ${count || 0} 个下载任务`,
            ),
          ]),
        ]),
      ]),
      View({ class: "wx-dl-page-actions" }, [
        DownloadPageActionButton({
          icon: "refresh-cw",
          label: "刷新",
          onClick() {
            vm$.methods.refreshTasks();
          },
        }),
        DownloadPageActionButton({
          icon: "trash2",
          label: computed(selected_task_count_, (count) => {
            return count > 0 ? `删除选中 ${count}` : "删除选中";
          }),
          class: "wx-dl-page-action-danger",
          onClick() {
            vm$.methods.requestDeleteSelectedTasks(false);
          },
        }),
        DownloadPageActionButton({
          icon: "trash2",
          label: "清空记录",
          class: "wx-dl-page-action-danger",
          onClick() {
            vm$.methods.requestClearTasks(false);
          },
        }),
      ]),
    ]),
  ]);
}

function DownloaderPageView(props) {
  const vm$ = props.store;

  return View(
    {
      class: "wx-dl-page-root",
      onMounted() {
        vm$.ready();
      },
    },
    [
      DownloadPageTopBar({ store: vm$ }),
      View({ class: "wx-dl-page-statusbar" }, [
        View({ class: "wx-dl-page-statusbar-inner" }, [
          DownloadPageStatusCounts({ store: vm$ }),
          DownloadPageStatusActions({ store: vm$ }),
        ]),
      ]),
      View({ class: "wx-dl-page-main" }, [
        View({ class: "wx-dl-page-list-wrap" }, [
          DownloadPageTaskTableView({ store: vm$ }),
        ]),
        CreateTaskDialogView({
          store: vm$,
        }),
        CreatePlatformTaskDialogView({
          store: vm$,
        }),
        CreateTaskPreviewDialogView({
          store: vm$,
        }),
        CreatePlatformTaskPreviewDialogView({
          store: vm$,
        }),
        FilePickerDialog({
          dialogStore: vm$.ui.importFileDialog$,
          title: "导入文件",
          accept: ".db",
        }),
        TaskDeleteConfirmDialog({
          store: vm$,
        }),
        ClearTasksConfirmDialog({
          store: vm$,
        }),
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
    document.body.classList.add("wx-dl-page-body");
    document.body.dataset.weuiTheme = document.documentElement.classList.contains(
      "dark",
    )
      ? "dark"
      : "light";
    const vm$ = DownloaderPanelViewModel({
      enableDropdownMenu: false,
      fixedListHeight: false,
      initial_status: "all",
      itemHeight: 72,
      listGutter: 0,
      sort_by_status: false,
      syncListContentHeight: false,
    });
    Timeless.DOM.render(DownloaderPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

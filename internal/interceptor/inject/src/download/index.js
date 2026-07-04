/// <reference path="../utils.js" />
/// <reference path="core.js" />
/**
 * @file 下载管理页面入口
 */
function DownloadPageActionButton(props) {
  return View(
    {
      type: "button",
      class: ["wx-dl-page-action", props.class].filter(Boolean).join(" "),
      onClick: props.onClick,
    },
    [
      Timeless.Icon({ name: props.icon, size: props.iconSize || 16 }),
      View({ class: "wx-dl-page-action-label" }, [props.label]),
    ],
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
              "aria-pressed": computed(active_status_, (status) =>
                status === item.key ? "true" : "false",
              ),
            },
            class: computed(active_status_, (status) =>
              [
                "wx-dl-page-count",
                "wx-dl-page-count-filter",
                status === item.key ? "wx-dl-page-count-active" : "",
                item.key === "error" ? "wx-dl-page-count-error" : "",
              ]
                .filter(Boolean)
                .join(" "),
            ),
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
  return View({ class: "wx-dl-page-status-actions" }, [
    DownloadPageActionButton({
      icon: "play",
      iconSize: 14,
      label: "全部开始",
      class: "wx-dl-page-action-compact",
      onClick() {
        vm$.methods.startAllTasks();
      },
    }),
    DownloadPageActionButton({
      icon: "pause",
      iconSize: 14,
      label: "全部暂停",
      class: "wx-dl-page-action-compact",
      onClick() {
        vm$.methods.pauseAllTasks();
      },
    }),
  ]);
}

function DownloadPageTopBar(props) {
  const vm$ = props.store;
  const selected_task_count_ = vm$.state.selected_task_count;
  return View({ class: "wx-dl-page-topbar" }, [
    View({ class: "wx-dl-page-brand" }, [
      View({ class: "wx-dl-page-brand-icon" }, [
        Timeless.Icon({ name: "history", size: 30 }),
      ]),
      View({ class: "wx-dl-page-title" }, ["Downloads"]),
    ]),
    View({ class: "wx-dl-page-actions" }, [
      DownloadPageActionButton({
        icon: "plus",
        label: "创建下载任务",
        onClick() {
          vm$.methods.requestCreateTask();
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
          DownloadTaskListView({
            store: vm$,
            class: "wx-dl-page-list wx-dl-dark-scroll",
            padding: "0",
            itemClass: "wx-dl-page-item",
            skeletonClass: "wx-dl-page-item",
            emptyClass: "wx-dl-page-empty",
            size: 12,
            paddingBottom: 24,
          }),
        ]),
        TaskDeleteConfirmDialog({
          store: vm$.ui.deleteConfirmDialog$,
          deleteFiles: vm$.state.delete_delete_files,
          loading: vm$.state.deleting_task,
          taskCount: vm$.state.pending_delete_task_count,
          onConfirm() {
            vm$.methods.confirmDeleteTask();
          },
        }),
        ClearTasksConfirmDialog({
          store: vm$.ui.clearConfirmDialog$,
          deleteFiles: vm$.state.clear_delete_files,
          loading: vm$.state.clearing_tasks,
          onConfirm() {
            vm$.methods.confirmClearTasks();
          },
        }),
        CreateDownloadTaskDialog({
          store: vm$.ui.createTaskDialog$,
          text: vm$.state.create_task_text,
          loading: vm$.state.creating_task,
          onInput(value) {
            vm$.methods.setCreateTaskText(value);
          },
          onConfirm() {
            vm$.methods.confirmCreateTask();
          },
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
    const vm$ = DownloaderPanelViewModel({
      enableDropdownMenu: false,
      fixedListHeight: false,
      initial_status: "all",
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

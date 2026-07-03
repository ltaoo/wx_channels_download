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
      Timeless.Icon({ name: props.icon, size: 16 }),
      View({ class: "wx-dl-page-action-label" }, [props.label]),
    ],
  );
}

function DownloadPageStatusCounts(props) {
  const vm$ = props.store;
  const status_counts_ = vm$.state.status_counts;
  return View({ class: "wx-dl-page-counts" }, [
    View({ class: "wx-dl-page-count wx-dl-page-count-total" }, [
      View({ class: "wx-dl-page-count-label" }, ["总计"]),
      View({ class: "wx-dl-page-count-value" }, [
        computed(vm$.state.task_count, (count) => String(count || 0)),
      ]),
    ]),
    ...DOWNLOAD_STATUS_COUNT_ITEMS.map((item) => {
      return View(
        {
          class: [
            "wx-dl-page-count",
            item.key === "error" ? "wx-dl-page-count-error" : "",
          ]
            .filter(Boolean)
            .join(" "),
        },
        [
          View({ class: "wx-dl-page-count-label" }, [item.label]),
          View({ class: "wx-dl-page-count-value" }, [
            computed(status_counts_, (counts) => {
              const c = normalize_download_status_counts(counts);
              return String(c[item.key] || 0);
            }),
          ]),
        ],
      );
    }),
  ]);
}

function DownloadPageTopBar(props) {
  const vm$ = props.store;
  return View({ class: "wx-dl-page-topbar" }, [
    View({ class: "wx-dl-page-brand" }, [
      View({ class: "wx-dl-page-brand-icon" }, [
        Timeless.Icon({ name: "history", size: 30 }),
      ]),
      View({ class: "wx-dl-page-title" }, ["Downloads"]),
    ]),
    DownloadPageStatusCounts({ store: vm$ }),
    View({ class: "wx-dl-page-actions" }, [
      DownloadPageActionButton({
        icon: "play",
        label: "全部开始",
        onClick() {
          vm$.methods.startAllTasks();
        },
      }),
      DownloadPageActionButton({
        icon: "pause",
        label: "全部暂停",
        onClick() {
          vm$.methods.pauseAllTasks();
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
          }),
        ]),
        TaskDeleteConfirmDialog({
          store: vm$.ui.deleteConfirmDialog$,
          deleteFiles: vm$.state.delete_delete_files,
          loading: vm$.state.deleting_task,
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

/// <reference path="../utils.js" />
/// <reference path="core.js" />
/**
 * @file 下载管理弹出面板入口
 */
function DownloaderEntry(props) {
  const vm$ = DownloaderPanelViewModel({
    onRequestClose() {
      props.popover$.hide();
    },
  });
  return Fragment({}, [
    Popover(
      {
        store: props.popover$,
        content: [
          DownloaderPanelView({
            store: vm$,
            renderConfirmDialog: false,
            showStatusCounts: false,
          }),
        ],
      },
      [
        View(
          {
            class:
              "mr-2 relative h-5 w-5 flex-initial flex-shrink-0 cursor-pointer",
          },
          [Timeless.Icon({ name: "download", size: 20 })],
        ),
      ],
    ),
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
  ]);
}

(() => {
  function insert_downloader($wrap, $trigger) {
    $wrap.insertBefore($trigger, $wrap.firstChild);
    const popover$ = new Timeless.ui.PopoverCore({
      offsetY: 4,
      destroyOnClose: false,
    });
    WXU.downloader.show = function () {
      popover$.show();
    };
    WXU.downloader.hide = function () {
      popover$.hide();
    };
    WXU.downloader.toggle = function () {
      popover$.toggle();
    };
    Timeless.DOM.render(DownloaderEntry({ popover$ }), $trigger);
  }
  let mounted = false;
  if (window.location.pathname === "/web/pages/profile") {
    WXU.observe_node(".page-profile", () => {
      var $page = document.querySelector(".page-profile");
      if (mounted) return;
      if (!$page) return;
      var $box = $page;
      var $btn_wrap = document.createElement("div");
      $btn_wrap.style.cssText =
        "z-index: 999; position: fixed; right: 40px; top: 36px;";
      insert_downloader($box, $btn_wrap);
      mounted = true;
    });
  } else if (window.location.hostname === "mp.weixin.qq.com") {
    //
  } else {
    WXU.observe_node(".home-header", () => {
      var $header = document.querySelector(".home-header");
      console.log("[DOWNLOADER]insert_downloader", mounted, $header);
      if (mounted) return;
      if (!$header) return;
      var $box = $header.children[$header.children.length - 1];
      if (!$box) return;
      var $btn_wrap = $box.children[0];
      if (!$btn_wrap) {
        $btn_wrap = $box;
      }
      var $download_panel_button = download_btn7();
      insert_downloader($btn_wrap, $download_panel_button);
      mounted = true;
    });
  }
})();

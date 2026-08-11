/// <reference path="../utils.js" />
/// <reference path="model.js" />
/// <reference path="view.js" />
/**
 * @file Download manager popup panel entry
 */
function DownloaderConnectionStatusDot(props) {
  return View(
    {
      style: computed(props.connected, (connected) => ({
        position: "absolute",
        top: "-2px",
        right: "-2px",
        width: "6px",
        height: "6px",
        "border-radius": "9999px",
        "background-color": connected ? "#22c55e" : "#ef4444",
        "box-sizing": "border-box",
      })),
      attributes: {
        title: computed(props.connected, (connected) =>
          connected ? "WebSocket 已连接" : "WebSocket 已断开",
        ),
        "aria-label": computed(props.connected, (connected) =>
          connected ? "WebSocket 已连接" : "WebSocket 已断开",
        ),
      },
    },
    [],
  );
}

function DownloaderEntry(props) {
  const vm$ =
    typeof __d_vm$ !== undefined ? __d_vm$ : DownloaderPanelViewModel({});
  return Fragment({}, [
    Popover(
      {
        store: props.popover$,
        content: [
          DownloaderPanelView({
            store: vm$,
            showStatusCounts: false,
            showViewAll: true,
          }),
        ],
      },
      [
        View(
          {
            class:
              "mr-2 relative h-5 w-5 flex-initial flex-shrink-0 cursor-pointer",
            onMounted() {
              vm$.methods.connect().catch(() => {});
              if (typeof props.onMounted === "function") {
                props.onMounted(vm$);
              }
            },
          },
          [
            Timeless.Icon({ name: "download", size: 20 }),
            DownloaderConnectionStatusDot({
              connected: vm$.state.websocket_connected,
            }),
          ],
        ),
      ],
    ),
    TaskDeleteConfirmDialog({
      store: vm$,
    }),
    ClearTasksConfirmDialog({
      store: vm$,
    }),
    // SingleOverwriteDownloadConfirmDialog({
    //   store: vm$,
    // }),
    BatchOverwriteDownloadConfirmDialog({
      store: vm$,
    }),
    OverwriteDownloadConfirmDialog({
      store: vm$,
    }),
  ]);
}

(() => {
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
  function insert_downloader($wrap, $trigger) {
    $wrap.insertBefore($trigger, $wrap.firstChild);
    Timeless.DOM.render(
      DownloaderEntry({
        popover$,
        onMounted(vm$) {
          WXU.downloader.create = function (feeds, opt) {
            return vm$.methods.createDownloadTask(feeds, opt);
          };
          WXU.downloader.browse = function (feeds, opt) {
            return vm$.methods.createBrowseHistories(feeds, opt);
          };
        },
      }),
      $trigger,
    );
  }
  let mounted = false;
  if (window.location.pathname === "/web/pages/profile") {
    WXU.observe_node({
      selector: ".page-profile",
      container: "#app",
      onOk() {
        var $page = document.querySelector(".page-profile");
        if (mounted || !$page) {
          return;
        }
        var $box = $page;
        var $btn_wrap = document.createElement("div");
        $btn_wrap.style.cssText =
          "z-index: 999; position: fixed; right: 40px; top: 36px;";
        insert_downloader($box, $btn_wrap);
        mounted = true;
      },
    });
    return;
  }
  WXU.log
    .Info()
    .Str("file", "/download/panel.js")
    .Msg("before observer .home-header");
  WXU.observe_node({
    selector: ".home-header",
    container: "#app",
    onOk() {
      var $header = document.querySelector(".home-header");
      WXU.log
        .Info()
        .Str("file", "/download/panel.js")
        .Bool("$header", !!$header)
        .Bool("mounted", !!mounted)
        .Msg("in observer callback");
      if (mounted || !$header) {
        return;
      }
      var $box = $header.children[$header.children.length - 1];
      if (!$box) return;
      var $btn_wrap = $box.children[0];
      if (!$btn_wrap) {
        $btn_wrap = $box;
      }
      var $download_panel_button = Icons.download_btn7();
      WXU.log
        .Info()
        .Str("file", "/download/panel.js:124")
        .Msg("before insert_downloader");
      insert_downloader($btn_wrap, $download_panel_button);
      mounted = true;
    },
  });
})();

/**
 * @file 首页推荐
 */

/**
 * 创建下载按钮并插入到指定父节点
 * @param {HTMLElement} $parent
 */
function __wx_insert_download_btn($parent) {
  var $btn = Icons.download_btn2();
  WXU.attach_download_dropdown_menu($btn);
  $btn.onclick = WXU.downloadBtnHandler;
  $parent.appendChild($btn);
}

/**
 * 在首页右侧添加悬浮下载按钮
 */
function __wx_render_sider_tools() {
  const $fixed_sider = document.createElement("div");
  $fixed_sider.className = "wx-sider";
  const $sider_bg = document.createElement("div");
  $sider_bg.className = "wx-sider-bg";
  const $tools = document.createElement("div");
  $tools.className = "wx-sider-tools";
  const $btn = document.createElement("div");
  $btn.className = "wx-sider-tools-btn";
  $btn.innerHTML = Icons.download_icon1;
  $btn.onclick = WXU.downloadBtnHandler;
  WXU.attach_download_dropdown_menu($btn);
  document.body.appendChild($fixed_sider);
  $fixed_sider.appendChild($sider_bg);
  $fixed_sider.appendChild($tools);
  $tools.appendChild($btn);
}

async function __wx_insert_download_btn_to_home_page() {
  var $operations_parents = [];
  var $op_items = Array.from(
    document.querySelectorAll(".slides-item .click-box.op-item"),
  );
  for (let i = 0; i < $op_items.length; i += 1) {
    var $op_item = $op_items[i];
    var $parent = $op_item.parentElement;
    if (!$operations_parents.includes($parent)) {
      $operations_parents.push($parent);
    }
  }
  for (let i = 0; i < $operations_parents.length; i += 1) {
    __wx_insert_download_btn($operations_parents[i]);
  }
}

WXU.observe_node({
  selector: ".slides-item",
  container: "#app",
  onOk: function () {
    __wx_insert_download_btn_to_home_page();
  },
  onError: function () {
    __wx_render_sider_tools();
  },
});

WXU.observe_node({
  selector: ".slides-scroll",
  container: "#app",
  onOk: function ($scroll_view) {
    // console.log("[feed.js].slides-scroll found, setting up MutationObserver");
    var observer = new MutationObserver(function (mutations) {
      mutations.forEach(function (mutation) {
        if (mutation.type !== "childList") return;
        mutation.addedNodes.forEach(function (node) {
          if (node.nodeType !== 1 || !node.matches) return;
          // .slides-item is reused DOM and won't be added; listen for new .click-box.op-item children instead
          if (node.matches(".click-box.op-item")) {
            // console.log("[feed.js]matched .click-box.op-item node");
            handleOpItem(node);
            return;
          }
          if (node.querySelectorAll) {
            var $opItems = node.querySelectorAll(".click-box.op-item");
            if ($opItems.length > 0) {
              // console.log(
              //   "[feed.js]found",
              //   $opItems.length,
              //   ".click-box.op-item inside added node",
              // );
              $opItems.forEach(function ($opItem) {
                handleOpItem($opItem);
              });
            }
          }
        });
      });
      function handleOpItem($opItem) {
        // Confirm inside .slides-item
        var $slide = $opItem.closest(".slides-item");
        if (!$slide) {
          console.log("[feed.js]op-item not inside .slides-item, skip");
          return;
        }
        if ($slide.querySelector(".download-icon")) {
          return;
        }
        var $parent = $opItem.parentElement;
        if ($parent) {
          __wx_insert_download_btn($parent);
        }
      }
    });
    observer.observe($scroll_view, { childList: true, subtree: true });
  },
});

WXU.onDOMContentLoaded(function () {
  var error_tip_timer = setTimeout(() => {
    WXU.error({
      msg: "没有获取到视频详情",
      alert: 0,
      source: "channels.home.js:114",
    });
  }, 5000);
  function clear_tip_timer() {
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
  }
  var home_page_mounted = false;
  WXU.onFetchFeedProfile((feed) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js")
      .Bool("mounted", home_page_mounted)
      .Msg("onFetchFeedProfile callback");
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    WXU.set_cur_video();
    WXU.set_feed(feed);
    WXU.emit(WXE.Events.Feed, feed);
    clear_tip_timer();
  });
  WXU.on("channels:PreloadFeeds", (feeds) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js:147")
      .JSON("feeds", feeds)
      .Msg("PreloadFeeds callback");
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    WXU.set_cur_video();
    WXU.set_feed(feeds[0]);
    WXU.emit(WXE.Events.Feed, feeds[0]);
    clear_tip_timer();
  });
  WXU.onPCFlowLoaded((feeds) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js:157")
      .Str("feed", feeds[0])
      .Str("title", feeds[0] ? feeds[0].objectDesc.description : null)
      .Str("nickname", feeds[0] ? feeds[0].nickname : null)
      .Bool("mounted", home_page_mounted)
      .Msg("onPCFlowLoaded callback");
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    WXU.set_cur_video();
    WXU.set_feed(feeds[0]);
    WXU.emit(WXE.Events.Feed, feeds[0]);
    clear_tip_timer();
  });
  WXU.onGotoNextFeed((feed) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js")
      .JSON("feed", feed)
      .Msg("onGotoNextFeed callback");
    WXU.set_cur_video();
    WXU.set_feed(feed);
    WXU.emit(WXE.Events.Feed, feed);
  });
  WXU.onGotoPrevFeed((feed) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js")
      .JSON("feed", feed)
      .Msg("onGotoPrevFeed callback");
    WXU.set_cur_video();
    WXU.set_feed(feed);
    WXU.emit(WXE.Events.Feed, feed);
  });
  WXU.onHomeFeedChanged((feed) => {
    WXU.log
      .Info()
      .Str("file", "channels.home.js")
      .JSON("feed", feed)
      .Msg("onHomeFeedChanged callback");
    WXU.set_cur_video();
    WXU.set_feed(feed);
    WXU.emit(WXE.Events.Feed, feed);
  });
});

/**
 * @file 首页推荐
 */

async function __wx_insert_download_btn_to_home_page() {
  var $container = await WXU.find_elm(function () {
    return document.querySelector(".slides-scroll");
  });
  if (!$container) {
    return false;
  }
  var cssText = $container.style.cssText;
  var re = /translate3d\([0-9]{1,}px, {0,1}-{0,1}([0-9]{1,})%/;
  var matched = cssText.match(re);
  var idx = matched ? Number(matched[1]) / 100 : 0;
  var $item = document.querySelectorAll(".slides-item")[idx];
  var $existing_download_btn = $item.querySelector(".download-icon");
  if ($existing_download_btn) {
    return false;
  }
  var $elm3 = await WXU.find_elm(
    () => $item.getElementsByClassName("click-box op-item")[0],
  );
  if (!$elm3) {
    return false;
  }
  const $parent = $elm3.parentElement;
  if ($parent) {
    const $btn = download_btn2();
    __wx_attach_download_dropdown_menu($btn);
    $btn.onclick = __wx_download_btn_handler;
    $parent.appendChild($btn);
    return true;
  }
  __wx_render_sider_tools();
  return false;
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
  $btn.innerHTML = download_icon1;
  $btn.onclick = __wx_download_btn_handler;
  __wx_attach_download_dropdown_menu($btn);
  document.body.appendChild($fixed_sider);
  $fixed_sider.appendChild($sider_bg);
  $fixed_sider.appendChild($tools);
  $tools.appendChild($btn);
}

(() => {
  insert_channels_style();
  var error_tip_timer = setTimeout(() => {
    WXU.error({ msg: "没有获取到视频详情", alert: 0 });
  }, 5000);
  var home_page_mounted = false;
  WXU.onFetchFeedProfile((feed) => {
    console.log("[home.js]WXU.onFetchFeedProfile", feed, home_page_mounted);
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    WXU.set_cur_video();
    WXU.set_feed(feed);
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onPCFlowLoaded((feeds) => {
    console.log("[main.js]WXU.onPCFlowLoaded", feeds, home_page_mounted);
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    WXU.set_cur_video();
    WXU.set_feed(feeds[0]);
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onGotoNextFeed((feed) => {
    console.log("[main.js]WXU.onGotoNextFeed", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onGotoPrevFeed((feed) => {
    console.log("[main.js]WXU.onGotoPrevFeed", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onHomeFeedChanged((feed) => {
    console.log("[main.js]WXU.onHomeFeedChanged", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_home_page();
  });
  WXE.onFeed((feed) => {
    console.log("[main.js]WXU.onFeed", feed);
    WXU.set_feed(feed);
  });
})();

/**
 * @file 视频详情页
 */
async function __wx_insert_download_btn_to_feed_profile_page() {
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
  if ($elm3) {
    const $parent = $elm3.parentElement;
    if ($parent) {
      const $btn = download_btn2();
      __wx_attach_download_dropdown_menu($btn);
      $btn.onclick = __wx_download_btn_handler;
      $parent.appendChild($btn);
      return true;
    }
  }
  const $btn = download_btn1();
  __wx_attach_download_dropdown_menu($btn);
  $btn.onclick = __wx_download_btn_handler;
  var $elm2 = await WXU.find_elm(function () {
    return document.getElementsByClassName("full-opr-wrp layout-col")[0];
  });
  if ($elm2) {
    var relative_node = $elm2.children[$elm2.children.length - 1];
    if (!relative_node) {
      $elm2.appendChild($btn);
      return true;
    }
    $elm2.insertBefore($btn, relative_node);
    return true;
  }
  var $elm1 = await WXU.find_elm(function () {
    return document.getElementsByClassName("full-opr-wrp layout-row")[0];
  });
  if ($elm1) {
    var relative_node = $elm1.children[$elm1.children.length - 1];
    if (!relative_node) {
      $elm1.appendChild($btn);
      return true;
    }
    $elm1.insertBefore($btn, relative_node);
    return true;
  }
  __wx_render_footer_tools();
  return false;
}

/**
 * 在视频详情页底部添加悬浮下载按钮
 */
function __wx_render_footer_tools() {
  const $fixed_footer = document.createElement("div");
  $fixed_footer.className = "wx-footer";
  const $tools = document.createElement("div");
  $tools.className = "wx-footer-tools";
  const $btn = document.createElement("div");
  $btn.className = "weui-btn weui-btn_default weui-btn_mini";
  $btn.innerHTML = "下载";
  $btn.onclick = __wx_download_btn_handler;
  __wx_attach_download_dropdown_menu($btn);
  document.body.appendChild($fixed_footer);
  $fixed_footer.appendChild($tools);
  $tools.appendChild($btn);
}

(() => {
  insert_channels_style();

  var error_tip_timer = setTimeout(() => {
    WXU.error({ msg: "没有获取到视频详情", alert: 0 });
  }, 5000);
  var prev_feed = null;
  var loaded = false;
  WXU.onFetchFeedProfile((feed) => {
    if (loaded) {
      return;
    }
    console.log("[feed.js]WXU.onFetchFeedProfile for page", feed);
    loaded = true;
    WXU.set_cur_video();
    WXU.set_feed(feed);
    // 这里是为了旧版本，在详情页直接切换视频，会重复插入下载按钮，以及视频没有切换到新的 做的修复
    // if (!prev_feed || prev_feed.id !== feed.id) {
    //   WXU.set_feed(feed);
    // }
    // prev_feed = feed;
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    __wx_insert_download_btn_to_feed_profile_page();
  });
  WXU.onGotoNextFeed((feed) => {
    console.log("[feed.js]WXU.onGotoNextFeed", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_feed_profile_page();
  });
  WXU.onGotoPrevFeed((feed) => {
    console.log("[feed.js]WXU.onGotoPrevFeed", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_feed_profile_page();
  });
  WXU.onHomeFeedChanged((feed) => {
    console.log("[feed.js]WXU.onHomeFeedChanged", feed);
    WXU.set_cur_video();
    WXU.set_feed(feed);
    __wx_insert_download_btn_to_feed_profile_page();
  });
  WXE.onFeed((feed) => {
    console.log("[feed.js]WXU.onFeed", feed);
    WXU.set_feed(feed);
  });
})();

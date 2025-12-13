window.__wx_channels_live_store__ = {};
function __wx_copy_live_download_command() {
  var profile = __wx_channels_live_store__.profile;
  if (!profile) {
    WXU.error({ msg: "检测不到视频，请将本工具更新到最新版" });
    return;
  }
  var filename = (() => {
    return new Date().valueOf();
  })();
  var _profile = {
    ...profile,
  };
  var command = `ffmpeg -i "${_profile.url}" -c copy -y "live_${filename}.flv"`;
  WXU.log({ prefix: "", msg: "" });
  WXU.log({ prefix: "", msg: "直播下载命令" });
  WXU.log({ prefix: "", msg: command });
  WXU.toast("请在终端查看下载命令");
}

async function insert_live_download_btn() {
  var $elm1 = await WXU.find_elm(function () {
    return document.querySelector(".host__info .extra");
  });
  if ($elm1) {
    var relative_node = $elm1.children[0];
    if (!relative_node) {
      return;
    }
    var __wx_channels_live_download_btn__ = download_btn4();
    __wx_channels_live_download_btn__.onclick = function () {
      __wx_copy_live_download_command();
    };
    $elm1.insertBefore(__wx_channels_live_download_btn__, relative_node);
    return;
  }
}

(() => {
  var live_timer = setTimeout(() => {
    WXU.error({ msg: "没有捕获到视频详情", alert: 0 });
  }, 5000);
  WXU.onFetchLiveProfile((feed) => {
    console.log("[live.js]onFetchLiveProfile", feed);
    clearTimeout(live_timer);
    live_timer = null;
    WXU.set_live_feed(feed);
    insert_live_download_btn();
  });
})();

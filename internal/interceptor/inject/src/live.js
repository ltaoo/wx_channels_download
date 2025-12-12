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
  WXU.log({ msg: "等待注入按钮" });
  var $elm1 = await WXU.find_elm(function () {
    return document.querySelector(".host__info .extra");
  });
  if ($elm1) {
    var relative_node = $elm1.children[0];
    if (!relative_node) {
      WXU.error({ msg: "注入按钮失败!" });
      return;
    }
    var __wx_channels_live_download_btn__ = download_btn4();
    __wx_channels_live_download_btn__.onclick = function () {
      __wx_copy_live_download_command();
    };
    $elm1.insertBefore(__wx_channels_live_download_btn__, relative_node);
    WXU.log({ msg: "注入下载按钮成功!" });
    return;
  }
  WXU.error({ msg: "没有找到操作栏，注入按钮失败\n" });
}

(() => {
  var live_timer = setTimeout(() => {
    WXU.error({ msg: "没有捕获到视频详情", alert: 0 });
  }, 5000);
  WXU.onFetchLiveProfile((feed) => {
    WXU.set_live_feed(feed);
    clearTimeout(live_timer);
    live_timer = null;
    insert_live_download_btn();
  });
})();

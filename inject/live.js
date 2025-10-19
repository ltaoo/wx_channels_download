
function __wx_copy_live_download_command() {
  var profile = __wx_channels_live_store__.profile;
  if (!profile) {
    alert("检测不到视频，请将本工具更新到最新版");
    return;
  }
  var filename = (() => {
    return new Date().valueOf();
  })();
  var _profile = {
    ...profile,
  };
  var command = `ffmpeg -i "${_profile.url}" -c copy -y "live_${filename}.flv"`;
  __wx_log({
    prefix: "",
    msg: "",
  });
  __wx_log({
    prefix: "",
    msg: "直播下载命令",
  });
  __wx_log({
    prefix: "",
    msg: command,
  });
  if (window.__wx_channels_tip__ && window.__wx_channels_tip__.toast) {
    window.__wx_channels_tip__.toast("请在终端查看下载命令", 1e3);
  }
}

var __wx_channels_live_download_btn__ = icon_download4();
__wx_channels_live_download_btn__.onclick = function() {
  __wx_copy_live_download_command();
};

async function insert_live_download_btn() {
  __wx_log({
    msg: "等待注入命令按钮",
  });
  var $elm1 = await __wx_find_elm(function () {
    return document.querySelector(".host__info .extra");
  });
  if ($elm1) {
    var relative_node = $elm1.children[0];
    if (!relative_node) {
      __wx_log({
        msg: "注入按钮失败!",
      });
      return;
    }
    $elm1.insertBefore(__wx_channels_live_download_btn__, relative_node);
    __wx_log({
      msg: "注入下载按钮成功2!",
    });
    return;
  }
  __wx_log({
    msg: "没有找到操作栏，注入命令按钮失败\n",
  });
}
setTimeout(() => {
  window.__wx_channels_live_store__ = {};
  insert_live_download_btn();
}, 800);

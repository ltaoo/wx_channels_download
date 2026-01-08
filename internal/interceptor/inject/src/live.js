/**
 * @file 直播页
 */
window.__wx_channels_live_store__ = {};
function __wx_copy_live_download_command(url) {
  var filename = (() => {
    return new Date().valueOf();
  })();
  var command = `ffmpeg -i "${url}" -c copy -y "live_${filename}.flv"`;
  WXU.log({ prefix: "", msg: "" });
  WXU.log({ prefix: "", msg: "直播下载命令" });
  WXU.log({ prefix: "", msg: command });
  WXU.toast("请在终端查看下载命令");
}

async function __wx_insert_live_download_btn($btn) {
  var $elm1 = await WXU.find_elm(function () {
    return document.querySelector(".host__info .extra");
  });
  if ($elm1) {
    var relative_node = $elm1.children[0];
    if (!relative_node) {
      return false;
    }
    $elm1.insertBefore($btn, relative_node);
    return true;
  }
  return false;
}
/**
 * 为指定按钮添加额外的下载选项菜单
 * @param {HTMLElement} trigger
 */
function __wx_attach_live_download_dropdown_menu(trigger) {
  const { DropdownMenu, Menu, MenuItem } = WUI;
  const submenu$ = Menu({
    children: [],
  });
  const dropdown$ = DropdownMenu({
    $trigger: trigger,
    zIndex: 99999,
    children: [
      MenuItem({
        label: "更多",
        submenu: submenu$,
        onMouseEnter() {
          submenu$.show();
        },
        onMouseLeave() {
          if (!submenu$.isHover) {
            submenu$.hide();
          }
        },
      }),
    ],
    onMouseEnter() {
      if (submenu$.isOpen) {
        submenu$.hide();
      }
    },
  });
  dropdown$.ui.$trigger.onMouseLeave(() => {
    if (dropdown$.isHover) {
      return;
    }
    dropdown$.hide();
  });
  return [dropdown$, submenu$];
}

(() => {
  insert_channels_style();
  var error_tip_timer = setTimeout(() => {
    WXU.error({ msg: "没有捕获到视频详情", alert: 0 });
  }, 5000);
  var live_page_mounted = false;
  WXU.onFetchLiveProfile((feed) => {
    console.log("[live.js]onFetchLiveProfile", feed);
    if (live_page_mounted) {
      return;
    }
    live_page_mounted = true;
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    WXU.set_live_feed(feed);
  });
  WXU.onJoinLive(async (data) => {
    console.log("[live.js]onJoinLive", data);
    var $btn = download_btn4();
    $btn.onclick = function () {
      var profile = __wx_channels_live_store__.profile;
      if (!profile) {
        WXU.error({ msg: "检测不到视频，请将本工具更新到最新版" });
        return;
      }
      __wx_copy_live_download_command(profile.url);
    };
    var success = await __wx_insert_live_download_btn($btn);
    console.log("[live.js]insert btn success", success);
    if (!success) {
      return;
    }
    const i = WXU.API.createAdapterFromGlobalMapper(
      data,
      WXU.API.finderJoinLiveMapper,
      ["room", "stream", "liveUser"],
      "poll"
    );
    console.log("[live.js]has more options", i[1]);
    var { DropdownMenu, Menu, MenuItem } = WUI;
    if (i[1] && i[1].payload.channelParams) {
      var options = i[1].payload.channelParams.cdn_trans_info.filter(
        (vv) => vv.url
      );
      var [dropdown$, submenu$] = __wx_attach_live_download_dropdown_menu($btn);
      dropdown$.ui.$trigger.onMouseEnter(() => {
        const download_menus = [
          ...(() => {
            return options.map((opt) => {
              return MenuItem({
                label: opt.tag_name,
                onClick() {
                  __wx_copy_live_download_command(opt.url);
                  dropdown$.hide();
                },
              });
            });
          })(),
        ];
        submenu$.setChildren(download_menus);
        dropdown$.show();
      });
    }
  });
})();

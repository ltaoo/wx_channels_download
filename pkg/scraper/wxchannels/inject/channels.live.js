/**
 * @file 直播页
 */
var ChannelsAPIOrigin = WXEnv.get("apiOrigin");

(() => {
  function __wx_copy_live_download_command(url) {
    var filename = (() => {
      return new Date().valueOf();
    })();
    var command = `ffmpeg -i "${url}" -c copy -y "live_${filename}.flv"`;
    // WXU.log({ prefix: "", msg: "" });
    // WXU.log({ prefix: "", msg: "直播下载命令" });
    // WXU.log({ prefix: "", msg: command });
    WXU.copy(command);
    WXU.toast("直播下载命令已复制到粘贴板");
  }

  function __wx_live_download_menu_label(label) {
    if (typeof Node !== "undefined" && label instanceof Node) {
      return label.textContent || "";
    }
    return label == null ? "" : String(label);
  }

  function __wx_live_download_menu_click_payload(trigger) {
    const [err, profile, live] = WXU.check_live_existing({
      silence: true,
    });
    return {
      profile: err ? null : profile,
      live: err ? null : live,
      trigger,
    };
  }

  function __wx_render_extra_live_download_menu_items(items, trigger, close) {
    if (!Array.isArray(items)) {
      items = items ? [items] : [];
    }
    return items
      .filter((item) => {
        return item && item.label && item.onClick;
      })
      .map((item) => {
        return new Timeless.vm.MenuItemCore({
          label: __wx_live_download_menu_label(item.label),
          tooltip: item.tooltip || item.title,
          disabled: !!item.disabled,
          async onClick() {
            await item.onClick(__wx_live_download_menu_click_payload(trigger));
            close();
          },
        });
      });
  }

  /**
   * 为指定按钮添加额外的下载选项菜单
   * @param {HTMLElement} trigger
   * @param {Array} options - 直播流选项列表
   */
  function __wx_attach_live_download_dropdown_menu(trigger, options) {
    if (trigger.__attacheddropdown) {
      return trigger.__attacheddropdown;
    }

    function close_dropdown() {
      if (dropdown$) {
        dropdown$.hide({ reason: "download menu action" });
      }
    }

    function build_menu_items() {
      return [
        ...__wx_render_extra_live_download_menu_items(
          WXU.before_menu_items,
          trigger,
          close_dropdown,
        ),
        ...options.map((opt) => {
          return new Timeless.vm.MenuItemCore({
            label: [opt.tag_name, opt.rate, opt.video_quality_level_desc]
              .filter(Boolean)
              .join(" "),
            onClick() {
              __wx_copy_live_download_command(opt.url);
              close_dropdown();
            },
          });
        }),
        ...__wx_render_extra_live_download_menu_items(
          WXU.after_menu_items,
          trigger,
          close_dropdown,
        ),
      ];
    }

    const dropdown$ = new Timeless.vm.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: build_menu_items(),
    });

    const mount = document.createElement("span");
    mount.className = "wx-download-dropdown-menu-root";
    mount.style.display = "contents";
    document.body.appendChild(mount);
    Timeless.DOM.render(Timeless.weui.DropdownMenu({ store: dropdown$ }), mount);

    function set_reference() {
      dropdown$.setReference(
        {
          $el: trigger,
          getRect() {
            return trigger.getBoundingClientRect();
          },
        },
        { force: true },
      );
    }

    trigger.addEventListener("mouseenter", () => {
      set_reference();
      dropdown$.setItems(build_menu_items());
      dropdown$.handleEnterTrigger();
    });
    trigger.addEventListener("mouseleave", () => {
      dropdown$.handleLeaveTrigger();
    });
    trigger.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });
    if (trigger.dataset) {
      trigger.dataset.dropdownMenuImpl = "Timeless.weui.DropdownMenu";
    }
    trigger.__attacheddropdown = dropdown$;
    return dropdown$;
  }

  var error_tip_timer = setTimeout(() => {
    WXU.error({ msg: "没有捕获到视频详情", alert: 0, source: "channels.live.js:92" });
  }, 5000);
  var live_page_mounted = false;
  var profile = null;
  var live = null;
  var __wx_live_download_btn = null;

  function __wx_setup_live_btn() {
    if (!__wx_live_download_btn || !live) {
      return;
    }
    if (__wx_live_download_btn.__setup) {
      return;
    }
    __wx_live_download_btn.__setup = true;
    var $btn = __wx_live_download_btn;
    $btn.onclick = async function () {
      var [liveErr, p, liveData] = WXU.check_live_existing({ silence: true });
      if (
        liveErr ||
        !liveData ||
        !liveData.liveSdkInfo ||
        !liveData.liveSdkInfo.liveCdnUrl
      ) {
        WXU.error({ msg: "检测不到直播流，请将本工具更新到最新版", source: "channels.live.js:115" });
        return;
      }
      var content = Object.assign({}, liveData, p);
      try {
        var ins = WXU.loading({ msg: "正在创建直播下载任务..." });
        var [err, data] = await WXU.request({
          method: "POST",
          url: ChannelsAPIOrigin + "/api/v1/download_task/create",
          body: {
            objects: [
              {
                platform: "wxchannels",
                content: content,
                config: {},
              },
            ],
          },
        });
        ins.hide();
        if (err) {
          WXU.error({ msg: err.message || "创建下载任务失败", source: "channels.live.js:137" });
          return;
        }
        WXU.toast("直播下载任务已创建");
      } catch (e) {
        WXU.error({ msg: "创建下载任务失败: " + e.message, source: "channels.live.js:142" });
      }
    };
    if (!WXU.API.finderJoinLiveMapper) {
      console.log("missing WXU.API.finderJoinLiveMapper");
      return;
    }
    if (!WXU.API.createAdapterFromGlobalMapper) {
      console.log("missing WXU.API.createAdapterFromGlobalMapper");
      return;
    }
    const i = WXU.API.createAdapterFromGlobalMapper(
      live,
      WXU.API.finderJoinLiveMapper,
      ["room", "stream", "liveUser"],
      "poll",
    );
    console.log("[live.js]has more options", i[1]);
    if (i && i[1] && i[1].payload.channelParams) {
      var options = i[1].payload.channelParams.cdn_trans_info.filter(
        (vv) => vv.url,
      );
      __wx_attach_live_download_dropdown_menu($btn, options);
    }
  }

  WXU.observe_node({ selector: ".host__info .extra", container: "#app", onOk: function ($elm) {
    if (__wx_live_download_btn) {
      return;
    }
    var relative_node = $elm.children[0];
    if (!relative_node) {
      return;
    }
    var $btn = Icons.download_btn8();
    $elm.insertBefore($btn, relative_node);
    __wx_live_download_btn = $btn;
    __wx_setup_live_btn();
  }});

  async function handleLoaded(profile, data) {
    if (!data) {
      return;
    }
    if (live_page_mounted) {
      return;
    }
    live_page_mounted = true;
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    const feed = {
      id: data.liveInfo.liveId,
      liveInfo: data.liveInfo,
      liveDescription: data.liveDescription,
      objectDesc: {
        description: data.liveDescription,
      },
      contact: (() => {
        if (data.bizUserInfo) {
          return {
            nickname: data.bizUserInfo.bizNickname,
            username: data.bizUserInfo.bizUsername,
          };
        }
        if (profile) {
          return {
            nickname: profile.nickname,
            username: profile.username,
          };
        }
        return {
          nickname: "",
          username: "",
        };
      })(),
    };
    WXU.set_live_feed(feed);
    __wx_setup_live_btn();
  }
  WXU.onFetchFeedProfile((data) => {
    console.log("[live.js]onFetchFeedProfile", data);
    profile = data;
    __wx_channels_live_store__.profile = data;
    handleLoaded(profile, live);
  });
  WXU.onJoinLive(async (data) => {
    console.log("[live.js]onJoinLive", JSON.stringify(data));
    live = data;
    __wx_channels_live_store__.liveData = data;
    handleLoaded(profile, live);
  });
})();

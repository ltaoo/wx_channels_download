/**
 * 视频号专用工具。
 *
 * 这里集中放置视频号页面的下载入口，通用 DOM、请求和文件工具仍由
 * utils.js 提供。该文件必须在 channels.ws.js 之前加载。
 */
console.log("set extra methods to WXU");
if (typeof WXU === "undefined") {
  throw new Error("utils.js must be loaded before channels.utils.js");
}

console.log("set extra methods to WXU");

var __wx_username;
var __wx_channels_tip__ = {};
var __wx_channels_cur_video = null;
/** 全局的存储 */
var __wx_channels_store__ = {
  profile: null,
  buffers: [],
};
var __wx_channels_live_store__ = {
  profile: null,
};
function __wx_channels_video_decrypt(t, e, p) {
  for (
    var r = new Uint8Array(t), n = 0;
    n < t.byteLength && e + n < p.decryptor_array.length;
    n++
  )
    r[n] ^= p.decryptor_array[n];
  return r;
}
window.VTS_WASM_URL =
  "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.wasm";
window.MAX_HEAP_SIZE = 33554432;
var decryptor_array;
let decryptor;
/** t 是要解码的视频内容长度    e 是 decryptor_array 的长度 */
function wasm_isaac_generate(t, e) {
  decryptor_array = new Uint8Array(e);
  var r = new Uint8Array(Module.HEAPU8.buffer, t, e);
  decryptor_array.set(r.reverse());
  if (decryptor) {
    decryptor.delete();
  }
}
let loaded = false;
/** 获取 decrypt_array */
async function __wx_channels_decrypt(seed) {
  if (!loaded) {
    await WXU.load_script(
      "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/decrypt-video-core/1.3.0/wasm_video_decode.js",
    );
    loaded = true;
  }
  await WXU.sleep();
  decryptor = new Module.WxIsaac64(seed);
  // 调用该方法时，会调用 wasm_isaac_generate 方法
  // 131072 是 decryptor_array 的长度
  decryptor.generate(131072);
  // decryptor.delete();
  // const r = Uint8ArrayToBase64(decryptor_array);
  // decryptor_array = undefined;
  return decryptor_array;
}

(() => {
  function get_media_url(media) {
    if (!media) {
      return "";
    }
    return (media.url || "") + (media.urlToken || "");
  }
  function get_picture_cover_url(media) {
    if (!media) {
      return "";
    }
    return (
      media.coverUrl ||
      media.thumbUrl ||
      media.fullThumbUrl ||
      media.fullUrl ||
      get_media_url(media)
    );
  }
  function get_feed_title(feed) {
    if (feed.objectDesc && feed.objectDesc.description) {
      return feed.objectDesc.description;
    }
    if (
      feed.objectDesc &&
      feed.objectDesc.flowCardDesc &&
      feed.objectDesc.flowCardDesc.description
    ) {
      return feed.objectDesc.flowCardDesc.description;
    }
    if (
      feed.objectDesc &&
      feed.objectDesc.finderNewlifeDesc &&
      feed.objectDesc.finderNewlifeDesc.richTextTitle
    ) {
      return feed.objectDesc.finderNewlifeDesc.richTextTitle;
    }
    return feed.description || feed.id || "";
  }
  function format_bgm(feed) {
    var musicInfo =
      feed.objectDesc &&
      feed.objectDesc.followPostInfo &&
      feed.objectDesc.followPostInfo.musicInfo;
    if (!musicInfo || !musicInfo.mediaStreamingUrl) {
      return null;
    }
    return {
      url: musicInfo.mediaStreamingUrl,
      filename: "bgm.mp3",
      name: musicInfo.name || "",
      artist: musicInfo.artist || "",
      doc_id: musicInfo.docId || "",
      doc_type: musicInfo.docType || 0,
    };
  }
  function build_picture_zip_files(feed) {
    var files = [];
    var mediaList = feed.files || [];
    for (let i = 0; i < mediaList.length; i += 1) {
      var item = mediaList[i];
      var media_url = get_media_url(item);
      if (!media_url) {
        continue;
      }
      files.push({
        url: media_url,
        filename: `${files.length + 1}.jpg`,
      });
    }
    if (feed.bgm && feed.bgm.url) {
      files.push({
        url: feed.bgm.url,
        filename: feed.bgm.filename || "bgm.mp3",
      });
    }
    return files;
  }
  function build_picture_zip_url(feed) {
    return `zip://weixin.qq.com?files=${encodeURIComponent(
      JSON.stringify(build_picture_zip_files(feed)),
    )}`;
  }
  /**
   * 格式化 FeedProfile，增加了一些字段
   * @param {ChannelsFeed} feed
   * @returns {FeedProfile | null}
   */
  function format_feed(feed) {
    if (feed.liveInfo) {
      return {
        ...feed,
        type: "live",
        // id: feed.id,
        title: feed.description || "直播",
        url: feed.liveInfo.streamUrl,
        cover_url: (() => {
          if (feed.anchorContact) {
            return feed.anchorContact.liveCoverImgUrl;
          }
          if (
            feed.objectDesc &&
            feed.objectDesc.media &&
            feed.objectDesc.media[0]
          ) {
            return feed.objectDesc.media[0].coverUrl;
          }
          return "";
        })(),
        contact: (() => {
          if (feed.anchorContact) {
            return {
              id: feed.anchorContact.username,
              avatar_url: feed.anchorContact.headUrl,
              nickname: feed.anchorContact.nickname,
            };
          }
          return {
            id: feed.contact.username,
            nickname: feed.contact.nickname,
            avatar_url: feed.contact.headUrl,
          };
        })(),
      };
    }
    if (!feed.objectDesc) {
      return null;
    }
    var type = feed.objectDesc.mediaType;
    if (type === 9) {
      // 直播没有 media
      return null;
    }
    var mediaList = feed.objectDesc.media || [];
    var media = mediaList[0];
    if (type === 2) {
      // 图片视频
      return {
        ...feed,
        type: "picture",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        cover_url: get_picture_cover_url(media),
        title: get_feed_title(feed),
        files: mediaList,
        bgm: format_bgm(feed),
        url: "",
        key: 0,
        spec: [],
        contact: {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        },
      };
    }
    if (type === 4) {
      return {
        ...feed,
        type: "media",
        id: feed.id,
        nonce_id: feed.objectNonceId,
        title: get_feed_title(feed),
        url: get_media_url(media),
        key: media.decodeKey,
        cover_url: media.coverUrl,
        createtime: feed.createtime,
        spec: media.spec || [],
        size: media.fileSize,
        duration: media.videoPlayLen,
        contact: {
          id: feed.contact.username,
          avatar_url: feed.contact.headUrl,
          nickname: feed.contact.nickname,
        },
      };
    }
    return null;
  }
  /**
   * @param {string} text
   */
  function __wx_channels_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }
  function __wx_channels_play_cur_video() {
    if (
      __wx_channels_cur_video &&
      typeof __wx_channels_cur_video.player.play === "function"
    ) {
      __wx_channels_cur_video.player.play();
    }
  }
  function __wx_channels_pause_cur_video() {
    if (
      __wx_channels_cur_video &&
      typeof __wx_channels_cur_video.player.pause === "function"
    ) {
      __wx_channels_cur_video.player.pause();
    }
  }

  function set_feed(feed) {
    var profile = format_feed(feed);
    if (!profile) {
      return;
    }
    fetch("/__wx_channels_api/profile", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(profile),
    });
    __wx_channels_store__.profile = profile;
  }
  /**
   *
   * @param {ChannelsFeed} feed
   */
  function set_live_feed(feed) {
    var profile = format_feed(feed);
    if (!profile) {
      return;
    }
    fetch("/__wx_channels_api/profile", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(profile),
    });
    __wx_channels_live_store__.profile = profile;
  }

  /**
   * 检查是否存在视频
   * @param {{ silence?: boolean }} opt
   * @returns {[boolean, FeedProfile]}
   */
  function __wx_check_feed_existing(opt = {}) {
    var profile = __wx_channels_store__.profile;
    if (!profile) {
      WXU.error({
        alert: Number(!opt.silence),
        msg: "检测不到视频，请提交 issue 反馈",
      });
      return [true, null];
    }
    return [false, profile];
  }
  /**
   *
   * @param {ChannelsFeed} feed
   * @returns
   */
  // format_feed,
  // get cur_video() {
  //   return __wx_channels_cur_video;
  // },
  // pause_cur_video: __wx_channels_pause_cur_video,
  // play_cur_video: __wx_channels_play_cur_video,
  /**
   * 构建文件名
   * @param {FeedProfile} profile
   * @param {string} spec
   * @param {string} template
   */
  function build_filename(profile, spec, template) {
    var default_name = (() => {
      if (profile.title) {
        return profile.title;
      }
      if (profile.id) {
        return profile.id;
      }
      return new Date().valueOf();
    })();
    var params = {
      filename: default_name,
      id: profile.id,
      title: profile.title,
      spec: null,
      created_at: profile.createtime,
      download_at: (new Date().valueOf() / 1000).toFixed(0),
    };
    if (profile.contact) {
      params.author = profile.contact.nickname;
    }
    if (spec && profile.spec) {
      var matched = profile.spec.find((item) => item.fileFormat === spec);
      if (matched) {
        params.spec = matched.fileFormat;
      }
    }
    // if (!params.spec) {
    //   params.spec = "original";
    // }
    var filename = template
      ? template.replace(/\{\{([^}]+)\}\}/g, (match, key) =>
          params[key] === null || params[key] === undefined ? "" : params[key],
        )
      : default_name;
    if (typeof window.beforeFilename === "function") {
      return window.beforeFilename(filename, params, profile, spec);
    }
    return filename;
  }

  function format_codec_label(codec) {
    const v = String(codec || "").toLowerCase();
    if (!v) return "";
    if (v === "h264" || v === "avc") return "H.264";
    if (v === "h265" || v === "hevc") return "H.265";
    if (v === "av1") return "AV1";
    return v.toUpperCase();
  }
  function format_mbps_from_kbps(kbps) {
    const v = Number(kbps);
    if (!Number.isFinite(v) || v <= 0) return "";
    if (v >= 1000) {
      return `${remove_zero((v / 1000).toFixed(1))}Mbps`;
    }
    return `${remove_zero(v.toFixed(0))}Kbps`;
  }
  function format_mbps_from_kb_per_s(kbPerS) {
    const v = Number(kbPerS);
    if (!Number.isFinite(v) || v <= 0) return "";
    const mbps = (v * 8) / 1024;
    return `${remove_zero(mbps.toFixed(1))}Mbps`;
  }
  function format_quality_label(levelOrder) {
    const v = Number(levelOrder);
    if (!Number.isFinite(v)) return "";
    if (v <= 100) return "高";
    if (v <= 200) return "中";
    if (v <= 300) return "低";
    return "";
  }

  function format_media_spec_label(spec) {
    const parts = [];
    const w = Number(spec.width);
    const h = Number(spec.height);
    if (Number.isFinite(w) && Number.isFinite(h) && w > 0 && h > 0) {
      parts.push(`${w}x${h}`);
    }
    const codec = format_codec_label(spec.codingFormat || spec.codec);
    if (codec) {
      parts.push(codec);
    }
    const videoKbps = Number(spec.videoBitrate);
    const audioKbps = Number(spec.audioBitrate);
    if (
      Number.isFinite(videoKbps) &&
      videoKbps > 0 &&
      Number.isFinite(audioKbps) &&
      audioKbps >= 0
    ) {
      const total = videoKbps + audioKbps;
      const rate = format_mbps_from_kbps(total);
      if (rate) parts.push(rate);
    } else {
      const rate = format_mbps_from_kb_per_s(spec.bitRate);
      if (rate) parts.push(rate);
    }
    const q = format_quality_label(spec.levelOrder);
    if (q) parts.push(q);
    const ff = spec.fileFormat ? String(spec.fileFormat) : "";
    if (parts.length === 0) return ff;
    if (ff) return `${parts.join(" · ")} (${ff})`;
    return parts.join(" · ");
  }
  function format_media_spec_short_label(spec) {
    const parts = [];
    const w = Number(spec.width);
    const h = Number(spec.height);
    if (Number.isFinite(w) && Number.isFinite(h) && w > 0 && h > 0) {
      parts.push(`${w}x${h}`);
    }
    const ff = spec.fileFormat ? String(spec.fileFormat) : "";
    if (parts.length === 0) return ff;
    if (ff) return `${parts.join(" ")} ${ff}`;
    return parts.join(" ");
  }

  function __wx_download_menu_label(label) {
    if (typeof Node !== "undefined" && label instanceof Node) {
      return label.textContent || "";
    }
    return label == null ? "" : String(label);
  }

  function __wx_download_menu_click_payload(trigger) {
    const [err, profile] = WXU.check_feed_existing({
      silence: true,
    });
    return {
      profile: err ? null : profile,
      trigger,
    };
  }

  function __wx_create_timeless_download_menu_item(options, trigger, close) {
    return new Timeless.ui.MenuItemCore({
      label: __wx_download_menu_label(options.label),
      tooltip: options.title,
      disabled: !!options.disabled,
      async onClick() {
        if (typeof options.onClick === "function") {
          await options.onClick(__wx_download_menu_click_payload(trigger));
        }
        close();
      },
    });
  }

  function __wx_render_extra_download_dropdown_items(items, trigger, close) {
    return (items || [])
      .filter((item) => {
        return item.label && item.onClick;
      })
      .map((item) => {
        return __wx_create_timeless_download_menu_item(item, trigger, close);
      });
  }

  /**
   * 为指定按钮添加额外的下载选项菜单
   * @param {HTMLElement} trigger
   */
  function __wx_attach_download_dropdown_menu(trigger) {
    if (trigger.__wxTimelessDownloadDropdown) {
      return trigger.__wxTimelessDownloadDropdown;
    }

    const submenu$ = new Timeless.ui.MenuCore({
      items: [],
      trigger: "hover",
    });
    let dropdown$ = null;

    function close_dropdown() {
      submenu$.hide({ reason: "download menu action" });
      if (dropdown$) {
        dropdown$.hide({ reason: "download menu action" });
      }
    }

    function build_root_menu_items() {
      return [
        ...__wx_render_extra_download_dropdown_items(
          WXU.before_menu_items,
          trigger,
          close_dropdown,
        ),
        new Timeless.ui.MenuItemCore({
          label: "更多下载",
          menu: submenu$,
        }),
        new Timeless.ui.MenuItemCore({
          label: "下载为MP3",
          onClick() {
            const [err, profile] = WXU.check_feed_existing({ silence: true });
            if (err) return;
            var spec = (() => {
              if (WXU.config.defaultHighest) {
                return null;
              }
              if (profile.spec[0]) {
                return profile.spec[0].fileFormat;
              }
              return null;
            })();
            __wx_channels_handle_click_download__(spec, true);
            close_dropdown();
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "下载封面",
          onClick() {
            __wx_channels_handle_download_cover();
            close_dropdown();
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "复制页面链接",
          onClick() {
            WXU.copy(location.href);
            WXU.toast("复制成功");
            close_dropdown();
          },
        }),
        ...__wx_render_extra_download_dropdown_items(
          WXU.after_menu_items,
          trigger,
          close_dropdown,
        ),
      ];
    }

    function build_download_menu_items() {
      return [
        new Timeless.ui.MenuItemCore({
          label: "原始视频",
          onClick() {
            __wx_channels_handle_click_download__(null);
            close_dropdown();
          },
        }),
        ...(() => {
          const [err, profile] = WXU.check_feed_existing({
            silence: true,
          });
          if (err) {
            return [];
          }
          return (profile.spec || []).map((spec) => {
            const title = WXU.format_media_spec_label(spec) || spec.fileFormat;
            return new Timeless.ui.MenuItemCore({
              label: WXU.format_media_spec_short_label(spec),
              tooltip: title,
              onClick() {
                __wx_channels_handle_click_download__(spec.fileFormat);
                close_dropdown();
              },
            });
          });
        })(),
      ];
    }

    dropdown$ = new Timeless.ui.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: build_root_menu_items(),
    });

    const mount = document.createElement("span");
    mount.className = "wx-download-dropdown-menu-root";
    mount.style.display = "contents";
    document.body.appendChild(mount);
    Timeless.DOM.render(Timeless.DropdownMenu({ store: dropdown$ }), mount);

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
      submenu$.setItems(build_download_menu_items());
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
    trigger.__wxTimelessDownloadDropdown = dropdown$;
    return dropdown$;
  }

  var before_menus_items = [];
  var after_menus_items = [];
  var before_level2_menus_items = [];
  var after_level2_menus_items = [];
  var WXAPI = {};
  var WXAPI2 = {};
  var WXAPI3 = {};
  var WXAPI4 = {};

  WXE.onAPILoaded((variables) => {
    const keys = Object.keys(variables);
    for (let i = 0; i < keys.length; i++) {
      (() => {
        const variable = keys[i];
        const methods = variables[variable];
        // console.log("variable", {
        //   api: typeof methods.finderGetCommentDetail,
        //   api2: typeof methods.finderSearch,
        //   api3: typeof methods.finderLiveUserPage,
        //   api4: typeof methods.finderGetFollowList,
        // });
        if (typeof methods.finderGetFollowList === "function") {
          WXAPI4 = methods;
          return;
        }
        if (typeof methods.finderGetCommentDetail === "function") {
          WXAPI = methods;
          return;
        }
        if (typeof methods.finderSearch === "function") {
          WXAPI2 = methods;
          return;
        }
        if (typeof methods.finderLiveUserPage === "function") {
          WXAPI3 = methods;
          return;
        }
      })();
    }
  });

  WXE.onUtilsLoaded((methods) => {
    Object.assign(WXAPI, methods);
  });

  console.log("set extra methods to WXU");

  Object.assign(WXU, {
    get API() {
      return WXAPI;
    },
    get API2() {
      return WXAPI2;
    },
    get API3() {
      return WXAPI3;
    },
    get API4() {
      return WXAPI4;
    },
    /**
     * 视频解密
     */
    build_decrypt_arr: __wx_channels_decrypt,
    video_decrypt: __wx_channels_video_decrypt,
    async decrypt_video(buf, key) {
      try {
        const r = await WXU.build_decrypt_arr(key);
        if (r) {
          buf = WXU.video_decrypt(buf, 0, {
            decryptor_array: r,
          });
          return [null, buf];
        }
        return [new Error("前端解密失败"), null];
      } catch (err) {
        return [err, null];
      }
    },
    format_media_spec_label,
    format_media_spec_short_label,
    // build_filename,
    build_picture_zip_files,
    build_picture_zip_url,
    append_media_buf(buf) {
      __wx_channels_store__.buffers.push(buf);
    },
    set_cur_video() {
      setTimeout(() => {
        window.__wx_channels_cur_video = document.querySelector(
          ".feed-video.video-js",
        );
      }, 800);
    },
    /**
     * @param {ChannelsFeed} feed
     */
    set_feed(feed) {
      var profile = format_feed(feed);
      if (!profile) {
        return;
      }
      fetch("/__wx_channels_api/profile", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(profile),
      });
      __wx_channels_store__.profile = profile;
    },
    /**
     *
     * @param {ChannelsFeed} feed
     */
    set_live_feed(feed) {
      var profile = format_feed(feed);
      if (!profile) {
        return;
      }
      fetch("/__wx_channels_api/profile", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(profile),
      });
      __wx_channels_live_store__.profile = profile;
    },
    /**
     *
     * @param {ChannelsFeed} feed
     * @returns
     */
    format_feed,
    get cur_video() {
      return __wx_channels_cur_video;
    },
    pause_cur_video: __wx_channels_pause_cur_video,
    play_cur_video: __wx_channels_play_cur_video,
    check_feed_existing: __wx_check_feed_existing,
    attach_download_dropdown_menu: __wx_attach_download_dropdown_menu,
    unshiftMenuItems(items) {
      before_menus_items = items.concat(before_menus_items);
    },
    /**
     * 向菜单后面插入额外菜单
     * @param {{label: string; onClick?:(event: { profile: ChannelsMedia }) => void}[]} items
     */
    pushMenuItems(items) {
      after_menus_items = after_menus_items.concat(items);
    },
    get before_menu_items() {
      return before_menus_items;
    },
    get after_menu_items() {
      return after_menus_items;
    },
    get version() {
      return __wx_channels_version__;
    },
  });

  /**
   * 用于下载已经播放的视频内容
   * @param {FeedProfile} profile 视频信息
   */
  async function __wx_channels_download(profile) {
    console.log("__wx_channels_download");
    const data = profile.data;
    const blob = new Blob(data, { type: "video/mp4" });
    WXU.save(blob, profile.filename);
  }
  /**
   * 下载图片视频
   * @param {FeedProfile} profile 视频信息
   */
  async function __wx_channels_download3(profile) {
    console.log("__wx_channels_download3");
    const files = WXU.build_picture_zip_files(profile);
    if (files.length === 0) {
      WXU.error({ msg: "没有可下载的内容" });
      return;
    }
    const zip = await WXU.Zip();
    zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
    const ins = WXU.loading();
    const fetchPromises = files.map(async (item) => {
      const response = await fetch(item.url);
      if (!response.ok) {
        throw new Error(`${item.filename} ${response.status}`);
      }
      const blob = await response.blob();
      zip.file(item.filename, blob);
    });
    try {
      await Promise.all(fetchPromises);
      const content = await zip.generateAsync({ type: "blob" });
      await WXU.save(content, profile.filename + ".zip");
    } catch (err) {
      WXU.error({ msg: "下载失败，" + err.message });
    } finally {
      ins.hide();
    }
  }
  /**
   * 下载加密视频
   * @param {FeedProfile} feed 视频信息
   * @param {object} opt 选项
   * @param {string} opt.spec 规格
   * @param {string} opt.suffix 后缀
   */
  async function __wx_channels_download4(feed, opt) {
    console.log("__wx_channels_download4");
    if (!WXU.config.downloadInFrontend) {
      var [err, data] = await WXU.downloader.create(feed, opt);
      if (err) {
        WXU.error({ msg: err.message });
        return;
      }
      return;
    }
    var filename = WXU.build_filename(
      feed,
      opt.spec,
      WXU.config.downloadFilenameTemplate,
    );
    if (!filename) {
      WXU.error({ msg: "文件名生成失败" });
      return;
    }
    feed.filename = filename;
    if (feed.type === "picture") {
      __wx_channels_download3(feed);
      return;
    }
    if (opt.spec) {
      feed.url = feed.url + "&X-snsvideoflag=" + opt.spec;
    } else {
      var u = new URL(decodeURIComponent(feed.url));
      var filekey = u.searchParams.get("encfilekey");
      var token = u.searchParams.get("token");
      if (filekey && token) {
        var new_url = new URL(u.origin + u.pathname);
        new_url.searchParams.set("encfilekey", filekey);
        new_url.searchParams.set("token", token);
        feed.url = new_url.toString();
      }
    }
    if (WXU.config.downloadPauseWhenDownload) {
      WXU.pause_cur_video();
    }
    const ins = WXU.loading();
    var [err, response] = await WXU.fetch(feed.url);
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    const media_blob = await WXU.download_with_progress(response, {
      onStart({ total_size }) {
        WXU.log({
          msg: `总大小 ${WXU.bytes_to_size(total_size)}`,
        });
      },
      onProgress({ loaded_size, progress }) {
        WXU.log({
          replace: 1,
          msg:
            progress === null
              ? `${WXU.bytes_to_size(loaded_size)}`
              : `${progress}%`,
        });
      },
      onEnd() {},
    });
    WXU.log({ ignore_prefix: 1, msg: "" });
    var media_buf = new Uint8Array(await media_blob.arrayBuffer());
    if (feed.key) {
      WXU.log({ msg: "下载完成，开始解密" });
      var [err, data] = await WXU.decrypt_video(media_buf, feed.key);
      if (err) {
        WXU.error({ msg: "解密失败，" + err.message, alert: 0 });
        WXU.error({ msg: "尝试使用 decrypt 命令解密", alert: 0 });
      } else {
        WXU.log({ msg: "解密成功" });
        media_buf = data;
      }
    }
    if (opt.suffix === ".mp3") {
      const [err, mp3_blob] = await WXU.media_to_mp3(media_buf.buffer);
      if (err) {
        WXU.error({ msg: err.message });
        return;
      }
      WXU.emit(WXU.Events.MP3Downloaded, feed);
      WXU.save(mp3_blob, feed.filename + opt.suffix);
    } else {
      WXU.emit(WXU.Events.MediaDownloaded, feed);
      const result = new Blob([media_buf], { type: "video/mp4" });
      WXU.save(result, feed.filename + opt.suffix);
    }
    ins.hide();
    if (WXU.config.downloadPauseWhenDownload) {
      WXU.play_cur_video();
    }
  }
  /**
   * 所有下载功能统一先调用该方法
   * 由该方法分发到具体的 download 方法中
   * @param {string | null} spec 规格信息
   * @param {boolean} mp3 是否转换为 MP3
   */
  async function __wx_channels_handle_click_download__(spec, mp3) {
    const [err, feed] = WXU.check_feed_existing();
    if (err) return;
    const payload = { ...feed };
    payload.mp3 = !!mp3;
    payload.original_url = feed.url;
    payload.target_spec = spec;
    payload.source_url = location.href;
    WXU.log({
      msg: `${payload.source_url}
${payload.original_url}
${payload.key || ""}`,
    });
    WXU.emit(WXU.Events.BeforeDownloadMedia, payload);
    var suffix = ".mp4";
    if (mp3) {
      suffix = ".mp3";
    }
    if (payload.type === "picture") {
      suffix = ".zip";
    }
    __wx_channels_download4(payload, { spec, suffix });
  }
  /** 下载已加载的视频 */
  function __wx_channels_download_cur__() {
    const [err, profile] = WXU.check_feed_existing();
    if (err) return;
    if (__wx_channels_store__.buffers.length === 0) {
      WXU.error({ msg: "没有可下载的内容" });
      return;
    }
    var filename = WXU.build_filename(
      profile,
      null,
      WXU.config.downloadFilenameTemplate,
    );
    if (!filename) {
      WXU.error({ msg: "文件名生成失败" });
      return;
    }
    profile.filename = filename;
    profile.data = __wx_channels_store__.buffers;
    __wx_channels_download(profile);
  }
  /** 打印下载原始文件命令 */
  function __wx_channels_handle_print_download_command() {
    const [err, profile] = WXU.check_feed_existing();
    if (err) return;
    var _profile = { ...profile };
    var filename = WXU.build_filename(
      _profile,
      null,
      WXU.config.downloadFilenameTemplate,
    );
    if (!filename) {
      alert("文件名生成失败");
      return;
    }
    var command = `download --url "${_profile.url}"`;
    if (_profile.key) {
      command += ` --key ${_profile.key}`;
    }
    command += ` --filename "${filename}.mp4"`;
    WXU.log({ msg: command });
    WXU.toast("请在终端查看下载命令");
  }
  /** 下载视频封面 */
  async function __wx_channels_handle_download_cover() {
    var [err, profile] = WXU.check_feed_existing();
    if (err) return;
    var url = profile.cover_url.replace(/^http:/, "https:");
    if (!WXU.config.downloadInFrontend) {
      var [err, data] = await WXU.downloader.create(
        {
          id: profile.id,
          url,
          title: profile.title,
          spec: profile.spec,
          contact: profile.contact,
        },
        {
          suffix: ".jpg",
        },
      );
      if (err) {
        WXU.error({ msg: err.message });
        return;
      }
      return;
    }
    var filename = WXU.build_filename(
      profile,
      null,
      WXU.config.downloadFilenameTemplate,
    );
    if (!filename) {
      WXU.error({ msg: "文件名生成失败" });
      return;
    }
    WXU.log({ msg: `下载封面\n${url}` });
    const ins = WXU.loading();
    var [err, response] = await WXU.fetch(url);
    ins.hide();
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    const blob = await response.blob();
    WXU.save(blob, filename + ".jpg");
  }

  /** 下载图标 按钮，点击时的处理函数 */
  function __wx_download_btn_handler() {
    const [err, profile] = WXU.check_feed_existing();
    if (err) return;
    var spec = (() => {
      if (WXU.config.defaultHighest) {
        return null;
      }
      if (profile.spec[0]) {
        return profile.spec[0].fileFormat;
      }
      return null;
    })();
    __wx_channels_handle_click_download__(spec, false);
  }

  Object.assign(WXU, {
    downloadBtnHandler: __wx_download_btn_handler,
  });

  WXU.onInit((data) => {
    __wx_username = data.mainFinderUsername;
  });
})();

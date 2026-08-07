/**
 * 视频号专用工具。
 *
 * 这里集中放置视频号页面的下载入口，通用 DOM、请求和文件工具仍由
 * utils.js 提供。该文件必须在 channels.ws.js 之前加载。
 */
if (typeof WXU === "undefined") {
  throw new Error("utils.js must be loaded before channels.utils.js");
}

var __wx_username;
var __wx_channels_tip__ = {};
var __wx_channels_cur_video = null;
/** 全局的存储 */
var __wx_channels_store__ = {
  feed: null,
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
  // Calling this method invokes the wasm_isaac_generate function
  // 131072 is the length of decryptor_array
  decryptor.generate(131072);
  // decryptor.delete();
  // const r = Uint8ArrayToBase64(decryptor_array);
  // decryptor_array = undefined;
  return decryptor_array;
}

var WXBase64 = (() => {
  function bytesToString(bytes) {
    let encoded = "";
    for (let i = 0; i < bytes.length; i += 1) {
      encoded += `%${bytes[i].toString(16)}`;
    }
    return decodeURIComponent(encoded);
  }

  function stringToArrayBuffer(str) {
    const bytes = [];
    const n = str.length;
    for (let i = 0; i < n; i += 1) {
      let codePoint = str.charCodeAt(i);
      if (codePoint >= 55296 && codePoint <= 56319 && n > i + 1) {
        const next = str.charCodeAt(i + 1);
        if (next >= 56320 && next <= 57343) {
          codePoint = (codePoint - 55296) * 1024 + next - 56320 + 65536;
          i += 1;
        }
      }

      if (codePoint < 128) {
        bytes.push(codePoint);
        continue;
      }
      if (codePoint < 2048) {
        bytes.push((codePoint >> 6) | 192);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      if (codePoint < 55296 || (codePoint >= 57344 && codePoint < 65536)) {
        bytes.push((codePoint >> 12) | 224);
        bytes.push(((codePoint >> 6) & 63) | 128);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      if (codePoint >= 65536 && codePoint <= 1114111) {
        bytes.push((codePoint >> 18) | 240);
        bytes.push(((codePoint >> 12) & 63) | 128);
        bytes.push(((codePoint >> 6) & 63) | 128);
        bytes.push((codePoint & 63) | 128);
        continue;
      }
      bytes.push(239, 191, 189);
    }
    return new Uint8Array(bytes).buffer;
  }

  function urlEncode(base64Std) {
    return base64Std
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/g, "");
  }

  function urlDecode(base64UrlNoPad) {
    let s = base64UrlNoPad.replace(/-/g, "+").replace(/_/g, "/");
    while (s.length % 4) {
      s += "=";
    }
    return s;
  }

  function base64ToArrayBuffer(base64Std) {
    const bin = window.atob(base64Std);
    const n = bin.length;
    const out = new Uint8Array(n);
    for (let i = 0; i < n; i += 1) {
      out[i] = bin.charCodeAt(i);
    }
    return out.buffer;
  }

  function arrayBufferToBase64(buf) {
    let bin = "";
    const u8 = new Uint8Array(buf);
    for (let i = 0; i < u8.byteLength; i += 1) {
      bin += String.fromCharCode(u8[i]);
    }
    return window.btoa(bin);
  }

  function decodeBase64JSON(base64Std) {
    try {
      const ab = base64ToArrayBuffer(base64Std);
      const s = bytesToString(new Uint8Array(ab));
      return JSON.parse(s);
    } catch (e) {
      void e;
      return {};
    }
  }

  function encodeStringBase64(value) {
    try {
      const s = typeof value === "string" ? value : JSON.stringify(value);
      const ab = stringToArrayBuffer(s);
      return arrayBufferToBase64(ab);
    } catch (e) {
      void e;
      return "";
    }
  }

  function decodeBase64String(base64Std) {
    try {
      const ab = base64ToArrayBuffer(base64Std);
      return bytesToString(new Uint8Array(ab));
    } catch (e) {
      void e;
      return "";
    }
  }

  function encodeUint64ToBase64(decimalUint64) {
    try {
      let n = BigInt(decimalUint64);
      if (n < 0n || n > 18446744073709551615n) {
        return "";
      }
      const bytes = new Uint8Array(8);
      for (let i = 7; i >= 0; i -= 1) {
        bytes[i] = Number(n & 255n);
        n >>= 8n;
      }
      return urlEncode(arrayBufferToBase64(bytes.buffer));
    } catch (e) {
      void e;
      return "";
    }
  }

  function decodeBase64ToUint64String(base64UrlNoPad) {
    try {
      const base64Std = urlDecode(base64UrlNoPad);
      const ab = base64ToArrayBuffer(base64Std);
      let bytes = new Uint8Array(ab);
      if (bytes.length === 0) {
        return "";
      }
      if (bytes.length < 8) {
        const padded = new Uint8Array(8);
        padded.set(bytes, 8 - bytes.length);
        bytes = padded;
      } else if (bytes.length > 8) {
        bytes = bytes.subarray(0, 8);
      }
      let n = 0n;
      for (let i = 0; i < 8; i += 1) {
        n = (n << 8n) | BigInt(bytes[i]);
      }
      return n.toString(10);
    } catch (e) {
      void e;
      return "";
    }
  }

  const api = {
    urlEncode,
    urlDecode,
    base64ToArrayBuffer,
    arrayBufferToBase64,
    decodeBase64JSON,
    encodeStringBase64,
    decodeBase64String,
    encodeUint64ToBase64,
    decodeBase64ToUint64String,
  };
  return api;
})();

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
      // Live streams have no media
      return null;
    }
    var mediaList = feed.objectDesc.media || [];
    var media = mediaList[0];
    if (type === 2) {
      // Picture/video
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
  /**
   * 检查是否存在视频
   * @param {{ silence?: boolean }} opt
   * @returns {[boolean, FeedProfile]}
   */
  function __wx_check_feed_existing(opt = {}) {
    var feed = __wx_channels_store__.feed;
    if (!feed) {
      WXU.error({
        alert: Number(!opt.silence),
        msg: "检测不到视频，请提交 issue 反馈",
      });
      return [true, null];
    }
    return [false, feed];
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
   * @param {ChannelsFeed} feed
   * @param {string} spec
   * @param {string} template
   */
  function build_filename(feed, spec, template) {
    var profile = WXU.format_feed(feed);
    if (!profile) {
      throw new Error("there is no feed");
    }
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
      return `${WXU.remove_zero((v / 1000).toFixed(1))}Mbps`;
    }
    return `${WXU.remove_zero(v.toFixed(0))}Kbps`;
  }
  function format_mbps_from_kb_per_s(kbPerS) {
    const v = Number(kbPerS);
    if (!Number.isFinite(v) || v <= 0) return "";
    const mbps = (v * 8) / 1024;
    return `${WXU.remove_zero(mbps.toFixed(1))}Mbps`;
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

  function __wx_download_menu_items(items) {
    if (Array.isArray(items)) {
      return items;
    }
    return items ? [items] : [];
  }

  function __wxmp_create_download_menu_item(options, trigger, close) {
    return new Timeless.ui.MenuItemCore({
      label: __wx_download_menu_label(options.label),
      tooltip: options.tooltip || options.title,
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
    return __wx_download_menu_items(items)
      .filter((item) => {
        return item && item.label && item.onClick;
      })
      .map((item) => {
        return __wxmp_create_download_menu_item(item, trigger, close);
      });
  }

  /**
   * 为指定按钮添加额外的下载选项菜单
   * @param {HTMLElement} trigger
   */
  function __wx_attach_download_dropdown_menu(trigger) {
    if (trigger.__attacheddropdown) {
      return trigger.__attacheddropdown;
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
      var feed = __wx_channels_store__.feed;
      return [
        ...__wx_render_extra_download_dropdown_items(
          before_menu_items,
          trigger,
          close_dropdown,
        ),
        ...(feed && feed.objectDesc && feed.objectDesc.mediaType === 4
          ? [
              new Timeless.ui.MenuItemCore({
                label: "更多下载",
                menu: submenu$,
              }),
            ]
          : []),
        ...(feed && feed.objectDesc && feed.objectDesc.mediaType === 4
          ? [
              new Timeless.ui.MenuItemCore({
                label: "下载为MP3",
                onClick() {
                  const [err, profile] = WXU.check_feed_existing({
                    silence: true,
                  });
                  if (err) return;
                  __wx_channels_handle_click_download__({ suffix: ".mp3" });
                  close_dropdown();
                },
              }),
            ]
          : []),
        ...(feed && feed.objectDesc && feed.objectDesc.mediaType === 4
          ? [
              new Timeless.ui.MenuItemCore({
                label: "下载封面",
                onClick() {
                  __wx_channels_handle_download_cover();
                  close_dropdown();
                },
              }),
            ]
          : []),
        new Timeless.ui.MenuItemCore({
          label: "复制页面链接",
          onClick() {
            WXU.copy(location.href);
            WXU.toast("复制成功");
            close_dropdown();
          },
        }),
        ...__wx_render_extra_download_dropdown_items(
          after_menu_items,
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
            __wx_channels_handle_click_download__({ spec: "original" });
            close_dropdown();
          },
        }),
        ...(() => {
          const [err, feed] = WXU.check_feed_existing({
            silence: true,
          });
          if (err) {
            return [];
          }
          const profile = WXU.format_feed(feed);
          return (profile.spec || []).map((spec) => {
            return new Timeless.ui.MenuItemCore({
              label: format_media_spec_short_label(spec),
              tooltip: format_media_spec_label(spec) || spec.fileFormat,
              onClick() {
                __wx_channels_handle_click_download__({
                  spec: spec.fileFormat,
                });
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
    Timeless.DOM.render(
      Timeless.weui.DropdownMenu({ store: dropdown$ }),
      mount,
    );

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
      dropdown$.setItems(build_root_menu_items());
      submenu$.setItems(build_download_menu_items());
      dropdown$.handleEnterTrigger();
    });
    trigger.addEventListener("mouseleave", () => {
      dropdown$.handleLeaveTrigger();
    });
    trigger.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });
    trigger.__attacheddropdown = dropdown$;
    return dropdown$;
  }

  var before_menu_items = [];
  var after_menu_items = [];
  var before_level2_menus_items = [];
  var after_level2_menus_items = [];
  var WXAPI = {};
  var WXAPI2 = {};
  var WXAPI3 = {};
  var WXAPI4 = {};

  WXU.onAPILoaded((variables) => {
    const keys = Object.keys(variables);
    for (let i = 0; i < keys.length; i++) {
      (() => {
        const variable = keys[i];
        const methods = variables[variable];
        if (typeof methods.finderGetFollowList === "function") {
          WXAPI4 = methods;
          return;
        }
        if (typeof methods.finderGetCommentDetail === "function") {
          // console.log("WXU.onAPILoaded", methods);
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
  WXU.onUtilsLoaded((methods) => {
    Object.assign(WXAPI, methods);
  });

  Object.defineProperties(WXU, {
    API: {
      get() {
        return WXAPI;
      },
      configurable: true,
      enumerable: true,
    },
    API2: {
      get() {
        return WXAPI2;
      },
      configurable: true,
      enumerable: true,
    },
    API3: {
      get() {
        return WXAPI3;
      },
      configurable: true,
      enumerable: true,
    },
    API4: {
      get() {
        return WXAPI4;
      },
      configurable: true,
      enumerable: true,
    },
    Base64: {
      get() {
        return WXBase64;
      },
      configurable: true,
      enumerable: true,
    },
    cur_video: {
      get() {
        return __wx_channels_cur_video;
      },
      configurable: true,
      enumerable: true,
    },
    before_menu_items: {
      get() {
        return before_menu_items;
      },
      configurable: true,
      enumerable: true,
    },
    after_menu_items: {
      get() {
        return after_menu_items;
      },
      configurable: true,
      enumerable: true,
    },
  });
  Object.assign(WXU, {
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
      __wx_channels_store__.feed = feed;
      WXU.log
        .Info()
        .Str("file", "channels.utils.js")
        .JSON("feed", feed)
        .Msg("set_feed");
      WXU.downloader.browse([feed], { platform: "wxchannels" });
    },
    /**
     *
     * @param {ChannelsFeed} feed
     */
    set_live_feed(feed) {
      __wx_channels_live_store__.feed = feed;
      WXU.log
        .Info()
        .Str("file", "channels.utils.js")
        .JSON("feed", feed)
        .Msg("set_live_feed");
      WXU.downloader.browse([feed], { platform: "wxchannels" });
    },
    /**
     *
     * @param {ChannelsFeed} feed
     * @returns
     */
    format_feed,
    build_filename,
    pause_cur_video: __wx_channels_pause_cur_video,
    play_cur_video: __wx_channels_play_cur_video,
    check_feed_existing: __wx_check_feed_existing,
    attach_download_dropdown_menu: __wx_attach_download_dropdown_menu,
    unshiftMenuItems(items) {
      items = __wx_download_menu_items(items);
      before_menu_items = items.concat(before_menu_items);
    },
    /**
     * 向菜单后面插入额外菜单
     * @param {{label: string; onClick?:(event: { profile: ChannelsMedia }) => void}[]} items
     */
    pushMenuItems(items) {
      items = __wx_download_menu_items(items);
      after_menu_items = after_menu_items.concat(items);
    },
    get version() {
      return window.__d_config.version;
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
    console.log("__wx_channels_download4", opt);
    var filename = WXU.build_filename(
      feed,
      opt.spec,
      WXU.config.downloadFilenameTemplate,
    );
    if (!WXU.config.downloadInFrontend) {
      var something = await WXU.downloader.create([feed], {
        platform: "wxchannels",
        // filename: window.beforeFilename ? filename : undefined,
        ...opt,
      });
      console.log("after download create", something);
      const [err, data] = something;
      if (err) {
        WXU.error({ msg: err.message });
        return;
      }
      return;
    }
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
   * @param {string} [suffix] 后缀
   */
  async function __wx_channels_handle_click_download__({ spec, suffix }) {
    const [err, feed] = WXU.check_feed_existing();
    if (err) return;
    const payload = { ...feed };
    payload.source_url = location.href;
    WXU.log({
      msg: `${payload.source_url}
${payload.original_url}
${payload.key || ""}`,
    });
    WXU.emit(WXU.Events.BeforeDownloadMedia, payload);
    if (!suffix) {
      suffix = ".mp4";
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
    var [err, feed] = WXU.check_feed_existing();
    if (err) return;
    var filename = WXU.build_filename(
      feed,
      null,
      WXU.config.downloadFilenameTemplate,
    );
    if (!WXU.config.downloadInFrontend) {
      var [err, data] = await WXU.downloader.create([feed], {
        platform: "wxchannels",
        // filename: window.beforeFilename ? filename : undefined,
        suffix: ".jpg",
      });
      if (err) {
        WXU.error({ msg: err.message });
        return;
      }
      return;
    }
    var url = feed.cover_url.replace(/^http:/, "https:");
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
    const [err, feed] = WXU.check_feed_existing();
    if (err) return;
    __wx_channels_handle_click_download__({ spec: "" });
  }

  Object.assign(WXU, {
    downloadBtnHandler: __wx_download_btn_handler,
  });

  WXU.onInit((data) => {
    __wx_username = data.mainFinderUsername;
  });
})();

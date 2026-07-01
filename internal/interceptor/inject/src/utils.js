/**
 * @file 所有的工具函数 + API + 事件总线
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before utils.js");
}
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
var WXU = (() => {
  var defaultRandomAlphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  function __wx_uid__() {
    return random_string(12);
  }
  /**
   * 返回一个指定长度的随机字符串
   * @param length
   * @returns
   */
  function random_string(length) {
    return random_string_with_alphabet(length, defaultRandomAlphabet);
  }
  function random_string_with_alphabet(length, alphabet) {
    let b = new Array(length);
    let max = alphabet.length;
    for (let i = 0; i < b.length; i++) {
      let n = Math.floor(Math.random() * max);
      b[i] = alphabet[n];
    }
    return b.join("");
  }
  function sleep() {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve();
      }, 1000);
    });
  }
  function __wx_ensure_feedback_style() {
    if (document.getElementById("wx-feedback-style")) {
      return;
    }
    const style = document.createElement("style");
    style.id = "wx-feedback-style";
    style.textContent = `
.wx-top-tip {
  position: fixed;
  top: 16px;
  left: 50%;
  z-index: 2147483647;
  max-width: min(420px, calc(100vw - 32px));
  box-sizing: border-box;
  padding: 10px 14px;
  border-radius: 6px;
  background: #fa5151;
  color: #fff;
  font-size: 14px;
  line-height: 20px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.24);
  transform: translateX(-50%);
}
.wx-toast {
  position: fixed;
  left: 50%;
  bottom: 72px;
  z-index: 2147483647;
  max-width: min(360px, calc(100vw - 32px));
  box-sizing: border-box;
  padding: 10px 14px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.78);
  color: #fff;
  font-size: 14px;
  line-height: 20px;
  text-align: center;
  transform: translateX(-50%);
}
.wx-loading-mask {
  position: fixed;
  inset: 0;
  z-index: 2147483647;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.08);
}
.wx-loading-box {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.78);
  color: #fff;
  font-size: 14px;
}
.wx-loading-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: wx-feedback-spin 0.8s linear infinite;
}
@keyframes wx-feedback-spin {
  to {
    transform: rotate(360deg);
  }
}`;
    document.head.appendChild(style);
  }
  function __wx_top_tip(text) {
    __wx_ensure_feedback_style();
    const tip = document.createElement("div");
    tip.className = "wx-top-tip";
    tip.textContent = text || "";
    document.body.appendChild(tip);
    setTimeout(() => {
      tip.remove();
    }, 3000);
    return {
      hide() {
        tip.remove();
      },
    };
  }
  function __wx_toast(text) {
    __wx_ensure_feedback_style();
    const toast = document.createElement("div");
    toast.className = "wx-toast";
    toast.textContent = text || "";
    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2200);
    return {
      hide() {
        toast.remove();
      },
    };
  }
  function __wx_loading(text = "加载中") {
    __wx_ensure_feedback_style();
    const mask = document.createElement("div");
    mask.className = "wx-loading-mask";
    mask.innerHTML = `<div class="wx-loading-box"><span class="wx-loading-spinner"></span><span>${text}</span></div>`;
    document.body.appendChild(mask);
    return {
      hide() {
        mask.remove();
      },
    };
  }
  function __wx_menu_item(options) {
    return options || null;
  }
  function __wx_create_menu(options = {}) {
    const root = document.createElement("div");
    root.className = options.className || "wx-simple-menu";
    root.style.position = "fixed";
    root.style.display = "none";
    root.style.zIndex = String(options.zIndex || 99999);
    document.body.appendChild(root);

    let anchor = null;
    let placement = options.placement || "bottom-end";
    let isHover = false;
    let isOpen = false;
    let hideTimer = null;
    let childMenus = [];
    let cleanupAutoUpdate = null;

    function clearHideTimer() {
      if (hideTimer) {
        clearTimeout(hideTimer);
        hideTimer = null;
      }
    }
    function normalizePlacement(value) {
      if (value === "right") {
        return "right-start";
      }
      if (value === "left") {
        return "left-start";
      }
      return value || "bottom-end";
    }
    function stopAutoUpdate() {
      if (typeof cleanupAutoUpdate === "function") {
        cleanupAutoUpdate();
      }
      cleanupAutoUpdate = null;
    }
    function startAutoUpdate() {
      stopAutoUpdate();
      if (!anchor) {
        return;
      }
      const update = () => {
        if (isOpen) {
          position();
        }
      };
      window.addEventListener("resize", update);
      window.addEventListener("scroll", update, true);
      cleanupAutoUpdate = () => {
        window.removeEventListener("resize", update);
        window.removeEventListener("scroll", update, true);
      };
    }
    function scheduleHide() {
      clearHideTimer();
      hideTimer = setTimeout(() => {
        if (!isHover) {
          api.hide();
        }
      }, 100);
    }
    function fallbackPosition() {
      if (!anchor) {
        return;
      }
      const rect = anchor.getBoundingClientRect();
      const width = root.offsetWidth || 160;
      const height = root.offsetHeight || 40;
      const gap = 6;
      const padding = 8;
      const normalizedPlacement = normalizePlacement(placement);
      const side = normalizedPlacement.split("-")[0];
      const align = normalizedPlacement.split("-")[1] || "center";
      let left = rect.left + rect.width / 2 - width / 2;
      let top = rect.bottom + gap;

      if (align === "start") {
        left = side === "top" || side === "bottom" ? rect.left : left;
        top = side === "left" || side === "right" ? rect.top : top;
      } else if (align === "end") {
        left = side === "top" || side === "bottom" ? rect.right - width : left;
        top = side === "left" || side === "right" ? rect.bottom - height : top;
      }

      if (side === "top") {
        top = rect.top - height - gap;
      } else if (side === "left") {
        left = rect.left - width - gap;
        if (
          left < padding &&
          rect.right + gap + width <= window.innerWidth - padding
        ) {
          left = rect.right + gap;
        }
      } else if (side === "right") {
        left = rect.right + gap;
        if (
          left + width > window.innerWidth - padding &&
          rect.left - width - gap >= padding
        ) {
          left = rect.left - width - gap;
        }
      } else if (top + height > window.innerHeight - padding) {
        const topSide = rect.top - height - gap;
        if (topSide >= padding) {
          top = topSide;
        }
      }

      left = Math.max(
        padding,
        Math.min(left, window.innerWidth - width - padding),
      );
      top = Math.max(
        padding,
        Math.min(top, window.innerHeight - height - padding),
      );
      root.style.left = `${left}px`;
      root.style.top = `${top}px`;
    }
    function position() {
      if (!anchor) {
        return Promise.resolve();
      }
      fallbackPosition();
      return Promise.resolve();
    }
    function renderLabel(item, target) {
      if (item.label instanceof Node) {
        target.appendChild(item.label);
        return;
      }
      target.innerHTML = item.label == null ? "" : String(item.label);
    }
    function renderItem(item) {
      if (!item) {
        return null;
      }
      const itemEl = document.createElement("div");
      itemEl.className = "wx-simple-menu-item";
      if (item.title) {
        itemEl.title = item.title;
      }
      const labelEl = document.createElement("span");
      labelEl.className = "wx-simple-menu-item-label";
      renderLabel(item, labelEl);
      itemEl.appendChild(labelEl);
      if (item.submenu) {
        const arrow = document.createElement("span");
        arrow.className = "wx-simple-menu-item-arrow";
        arrow.textContent = ">";
        itemEl.appendChild(arrow);
        itemEl.addEventListener("mouseenter", () => {
          item.submenu.show(itemEl, "right");
        });
        itemEl.addEventListener("mouseleave", () => {
          setTimeout(() => {
            if (!item.submenu.isHover) {
              item.submenu.hide();
            }
          }, 100);
        });
      }
      if (typeof item.onMouseEnter === "function") {
        itemEl.addEventListener("mouseenter", item.onMouseEnter);
      }
      if (typeof item.onMouseLeave === "function") {
        itemEl.addEventListener("mouseleave", item.onMouseLeave);
      }
      if (typeof item.onClick === "function") {
        itemEl.addEventListener("click", async (event) => {
          event.preventDefault();
          event.stopPropagation();
          await item.onClick(event);
        });
      }
      return itemEl;
    }

    root.addEventListener("mouseenter", () => {
      isHover = true;
      clearHideTimer();
    });
    root.addEventListener("mouseleave", () => {
      isHover = false;
      scheduleHide();
    });

    const api = {
      ui: {
        $trigger: {
          onMouseEnter(fn) {
            options.trigger?.addEventListener("mouseenter", fn);
          },
          onMouseLeave(fn) {
            options.trigger?.addEventListener("mouseleave", (event) => {
              setTimeout(() => {
                fn(event);
              }, 100);
            });
          },
        },
      },
      get isHover() {
        return isHover;
      },
      get isOpen() {
        return isOpen;
      },
      setChildren(items) {
        childMenus.forEach((menu) => menu.hide());
        childMenus = [];
        root.innerHTML = "";
        (items || []).filter(Boolean).forEach((item) => {
          if (item.submenu) {
            childMenus.push(item.submenu);
          }
          const itemEl = renderItem(item);
          if (itemEl) {
            root.appendChild(itemEl);
          }
        });
        if (isOpen) {
          position();
        }
      },
      show(nextAnchor, nextPlacement) {
        anchor = nextAnchor || anchor || options.trigger || null;
        placement = nextPlacement || options.placement || placement;
        clearHideTimer();
        stopAutoUpdate();
        root.style.display = "block";
        root.style.visibility = "hidden";
        isOpen = true;
        position().then(() => {
          if (!isOpen) {
            return;
          }
          root.style.visibility = "visible";
          startAutoUpdate();
        });
      },
      hide() {
        clearHideTimer();
        stopAutoUpdate();
        childMenus.forEach((menu) => menu.hide());
        root.style.display = "none";
        isOpen = false;
      },
      destroy() {
        api.hide();
        root.remove();
      },
    };
    api.setChildren(options.children || []);
    return api;
  }
  function __wx_create_dropdown_menu(trigger, options = {}) {
    return __wx_create_menu({
      ...options,
      trigger,
      placement: options.placement || "bottom-end",
    });
  }
  function __wx_create_popover(trigger, options = {}) {
    const root = document.createElement("div");
    root.className = options.className || "wx-simple-popover";
    root.style.position = "fixed";
    root.style.display = "none";
    root.style.zIndex = String(options.zIndex || 99998);
    root.innerHTML = options.content || "";
    document.body.appendChild(root);
    let isOpen = false;
    function position() {
      const rect = trigger.getBoundingClientRect();
      const width = root.offsetWidth || 320;
      const height = root.offsetHeight || 40;
      let left = rect.right - width;
      let top = rect.bottom + 6;
      left = Math.max(8, Math.min(left, window.innerWidth - width - 8));
      top = Math.max(8, Math.min(top, window.innerHeight - height - 8));
      root.style.left = `${left}px`;
      root.style.top = `${top}px`;
    }
    const api = {
      open() {
        root.style.display = "block";
        root.style.visibility = "hidden";
        isOpen = true;
        position();
        root.style.visibility = "visible";
      },
      close() {
        root.style.display = "none";
        isOpen = false;
      },
      toggle() {
        if (isOpen) {
          api.close();
        } else {
          api.open();
        }
      },
    };
    trigger.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      api.toggle();
    });
    if (options.closeOnClickOutside) {
      document.addEventListener("click", (event) => {
        if (!isOpen) {
          return;
        }
        if (root.contains(event.target) || trigger.contains(event.target)) {
          return;
        }
        api.close();
      });
    }
    return api;
  }
  function confirm_overwrite_download(msg) {
    return new Promise((resolve) => {
      const content =
        (msg || "已存在该下载内容") + "，是否重新下载并覆盖已存在文件？";
      if (typeof document === "undefined" || !document.body) {
        resolve(window.confirm(content));
        return;
      }
      const timeless = window.Timeless;
      if (
        !timeless ||
        !timeless.ui ||
        !timeless.ui.DialogCore ||
        typeof timeless.render !== "function" ||
        typeof window.Dialog !== "function" ||
        typeof window.View !== "function"
      ) {
        resolve(window.confirm(content));
        return;
      }
      const dialog$ = new timeless.ui.DialogCore({
        closeable: true,
      });
      let settled = false;
      const $root = document.createElement("div");
      let offStateChange = null;
      const dialogView = OverwriteDownloadConfirmDialog({
        store: dialog$,
        content,
        onConfirm() {
          close(true);
        },
      });
      function handleKeydown(e) {
        if (e.key === "Escape") {
          close(false);
        }
      }
      function close(overwrite) {
        if (settled) {
          return;
        }
        settled = true;
        document.removeEventListener("keydown", handleKeydown);
        if (typeof offStateChange === "function") {
          offStateChange();
        }
        dialog$.hide();
        dialogView.onUnmounted();
        $root.remove();
        resolve(overwrite);
      }
      offStateChange = dialog$.onStateChange((state) => {
        if (settled || state.visible || state.enter) {
          return;
        }
        close(false);
      });
      timeless.render(dialogView, $root);
      document.addEventListener("keydown", handleKeydown);
      document.body.appendChild($root);
      dialog$.show();
    });
  }
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
  /**
   * @param {LogMsg} params
   */
  function __wx_log(params) {
    console.log("[log]", params);
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
  }
  /**
   * @param {ErrorMsg} params
   */
  function __wx_error(params) {
    var _alert = params.alert ?? 1;
    fetch("/__wx_channels_api/error", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
    if (_alert) {
      __wx_top_tip(params.msg);
    }
  }
  const script_loaded_map = {};
  function __wx_load_script(src) {
    const existing = script_loaded_map[src];
    if (existing) {
      return existing;
    }
    const p = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.type = "text/javascript";
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
    script_loaded_map[src] = p;
    return p;
  }

  /**
   * @param {() => HTMLElement} selector
   * @returns
   */
  function __wx_find_elm(selector) {
    return new Promise((resolve) => {
      var __count = 0;
      var __timer = setInterval(() => {
        __count += 1;
        var $elm = selector();
        if (!$elm) {
          if (__count >= 5) {
            clearInterval(__timer);
            __timer = null;
            resolve(null);
          }
          return;
        }
        resolve($elm);
        return;
      }, 200);
    });
  }

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
  function remove_zero(num) {
    let result = Number(num);
    if (String(num).indexOf(".") > -1) {
      result = parseFloat(num.toString().replace(/0+?$/g, ""));
    }
    return result;
  }
  function bytes_to_size(bytes) {
    if (!bytes) {
      return "0KB";
    }
    if (bytes === 0) {
      return "0KB";
    }
    const symbols = ["bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
    let exp = Math.floor(Math.log(bytes) / Math.log(1024));
    if (exp < 1) {
      return bytes + " " + symbols[0];
    }
    bytes = Number((bytes / Math.pow(1024, exp)).toFixed(2));
    const size = bytes;
    const unit = symbols[exp];
    if (Number.isInteger(size)) {
      return `${size}${unit}`;
    }
    return `${remove_zero(size.toFixed(2))}${unit}`;
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
  function get_queries(href) {
    var [pathname, search] = decodeURIComponent(href).split("?");
    var queries = decodeURIComponent(search)
      .split("&")
      .map((item) => {
        const [key, value] = item.split("=");
        return {
          [key]: value,
        };
      })
      .reduce(
        (prev, cur) => ({
          ...prev,
          ...cur,
        }),
        {},
      );
    return queries;
  }

  function mediaBufferToWav(abuffer, len) {
    len = len || abuffer.length;
    var num_of_chan = abuffer.numberOfChannels;
    var length = len * num_of_chan * 2 + 44;
    var buffer = new ArrayBuffer(length);
    var view = new DataView(buffer);
    var channels = [];
    var i;
    var sample;
    var offset = 0;
    var pos = 0;

    function setUint16(data) {
      view.setUint16(pos, data, true);
      pos += 2;
    }
    function setUint32(data) {
      view.setUint32(pos, data, true);
      pos += 4;
    }

    setUint32(0x46464952);
    setUint32(length - 8);
    setUint32(0x45564157);
    setUint32(0x20746d66);
    setUint32(16);
    setUint16(1);
    setUint16(num_of_chan);
    setUint32(abuffer.sampleRate);
    setUint32(abuffer.sampleRate * num_of_chan * 2);
    setUint16(num_of_chan * 2);
    setUint16(16);
    setUint32(0x61746164);
    setUint32(length - pos - 4);

    for (i = 0; i < abuffer.numberOfChannels; i += 1) {
      channels.push(abuffer.getChannelData(i));
    }
    while (pos < length) {
      for (i = 0; i < num_of_chan; i += 1) {
        sample = Math.max(-1, Math.min(1, channels[i][offset]));
        sample = (0.5 + sample < 0 ? sample * 32768 : sample * 32767) | 0;
        view.setInt16(pos, sample, true);
        pos += 2;
      }
      offset += 1;
    }
    return new Blob([buffer], { type: "audio/wav" });
  }
  // https://blog.csdn.net/qq_18643245/article/details/141157149
  function wav2Other(newSet, wavBlob, True, False) {
    const reader = new FileReader();
    reader.onloadend = async function () {
      //检测wav文件头
      const wavView = new Uint8Array(reader.result);
      const eq = function (p, s) {
        for (var i = 0; i < s.length; i++) {
          if (wavView[p + i] != s.charCodeAt(i)) {
            return false;
          }
        }
        return true;
      };
      let pcm;
      if (eq(0, "RIFF") && eq(8, "WAVEfmt ")) {
        var numCh = wavView[22];
        if (wavView[20] == 1 && (numCh == 1 || numCh == 2)) {
          //raw pcm 单或双声道
          var sampleRate =
            wavView[24] +
            (wavView[25] << 8) +
            (wavView[26] << 16) +
            (wavView[27] << 24);
          var bitRate = wavView[34] + (wavView[35] << 8);
          //搜索data块的位置
          var dataPos = 0; // 44 或有更多块
          for (var i = 12, iL = wavView.length - 8; i < iL; ) {
            if (
              wavView[i] == 100 &&
              wavView[i + 1] == 97 &&
              wavView[i + 2] == 116 &&
              wavView[i + 3] == 97
            ) {
              //eq(i,"data")
              dataPos = i + 8;
              break;
            }
            i += 4;
            i +=
              4 +
              wavView[i] +
              (wavView[i + 1] << 8) +
              (wavView[i + 2] << 16) +
              (wavView[i + 3] << 24);
          }
          console.log("wav info", sampleRate, bitRate, numCh, dataPos);
          if (dataPos) {
            if (bitRate == 16) {
              pcm = new Int16Array(wavView.buffer.slice(dataPos));
            } else if (bitRate == 8) {
              pcm = new Int16Array(wavView.length - dataPos);
              //8位转成16位
              for (var j = dataPos, d = 0; j < wavView.length; j++, d++) {
                var b = wavView[j];
                pcm[d] = (b - 128) << 8;
              }
            }
          }
          if (pcm && numCh == 2) {
            //双声道简单转单声道
            var pcm1 = new Int16Array(pcm.length / 2);
            for (var i = 0; i < pcm1.length; i++) {
              pcm1[i] = (pcm[i * 2] + pcm[i * 2 + 1]) / 2;
            }
            pcm = pcm1;
          }
        }
      }
      if (!pcm) {
        False && False("非单或双声道wav raw pcm格式音频，无法转码");
        return;
      }
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
      var rec = Recorder(newSet).mock(pcm, sampleRate);
      rec.stop(function (blob, duration) {
        True(blob, duration, rec);
      }, False);
    };
    reader.readAsArrayBuffer(wavBlob);
  }

  async function wavBlobToMP3(wavBlob) {
    return new Promise((resolve) => {
      if (!wavBlob) {
        resolve([new Error("Missing the wav blob"), null]);
        return;
      }
      var set = {
        type: "mp3",
        sampleRate: 48000,
        bitRate: 96,
      };
      wav2Other(
        set,
        wavBlob,
        function (blob) {
          resolve([null, blob]);
        },
        function (msg) {
          resolve([new Error(msg || "Conversion failed"), null]);
        },
      );
    });
  }
  /**
   * 支持回调的下载
   * @param {Response} response
   * @param {{ onStart: (v: { total_size: number }) => void, onProgress: (v: { loaded_size: number, progress: number | null }) => void, onEnd: (v: { blob: Blob }) => void }} handlers
   */
  async function download_with_progress(response, handlers) {
    var content_length = response.headers.get("Content-Length");
    var chunks = [];
    var total_size = content_length ? parseInt(content_length, 10) : 0;
    if (total_size) {
      if (handlers.onStart) {
        handlers.onStart({ total_size });
      }
    }
    var loaded_size = 0;
    var reader = response.body.getReader();
    while (true) {
      var { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
      loaded_size += value.length;
      if (handlers.onProgress) {
        handlers.onProgress({
          loaded_size,
          progress: total_size
            ? Number(((loaded_size / total_size) * 100).toFixed(2))
            : null,
        });
      }
    }
    var blob = new Blob(chunks);
    if (handlers.onEnd) {
      handlers.onEnd({ blob });
    }
    return blob;
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
   * @param {RequestInfo | URL} url
   * @returns {Promise<[null, Response] | [Error, null]>}
   */
  async function __wx_fetch(url) {
    try {
      const r = await fetch(url);
      return [null, r];
    } catch (err) {
      return [/** @type {Error} */ (err), null];
    }
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
  return {
    ...WXE,
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
    downloader: {
      show() {},
      hide() {},
      toggle() {},
      /**
       * 提交下载任务
       * @param {FeedProfile} feed
       * @param {object} opt 配置
       * @param {string} [opt.spec] 规格
       * @param {string} [opt.suffix] 后缀
       */
      async create(feed, opt = {}) {
        console.log("[downloader.create]create", feed);
        var spec = (() => {
          if (opt.spec) {
            return opt.spec;
          }
          if (WXU.config.defaultHighest || opt.spec === null) {
            return null;
          }
          if (feed.spec && feed.spec[0]) {
            return feed.spec[0].fileFormat;
          }
          return null;
        })();
        var filename = WXU.build_filename(
          feed,
          spec,
          WXU.config.downloadFilenameTemplate,
        );
        if (!filename) {
          return [new Error("filename 为空"), null];
        }
        if (feed.type === "picture") {
          opt.suffix = ".zip";
          feed.url = build_picture_zip_url(feed);
          console.log("[]feed.url", feed.url);
        }
        if (opt.suffix !== ".jpg") {
          if (spec) {
            feed.url = feed.url + "&X-snsvideoflag=" + spec;
          } else {
            // 该下载原始视频逻辑参考自 https://github.com/putyy/res-downloader/blob/master/core/resource.go#L142
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
        }
        const requestBody = {
          id: feed.id,
          nonce_id: feed.nonce_id || feed.objectNonceId || "",
          url: feed.url,
          title: feed.title,
          filename: filename,
          key: Number(feed.key),
          spec,
          suffix: opt.suffix,
        };
        const createTask = (overwrite) =>
          WXU.request({
            method: "POST",
            url: WXEnv.apiOrigin + "/api/task/create",
            body: {
              ...requestBody,
              overwrite: !!overwrite,
            },
          });
        // console.log("[downloader.create]before WXU.request");
        var [err, data] = await createTask(false);
        if (err && err.code === 409) {
          const overwrite = await confirm_overwrite_download(err.message);
          if (!overwrite) {
            return [null, { skipped: true }];
          }
          [err, data] = await createTask(true);
        }
        WXU.downloader.show();
        if (err) {
          return [err, null];
        }
        return [null, data];
      },
      /**
       * 批量提交下载任务
       * @param {FeedProfile[]} feeds
       * @param {object} opt 配置
       * @param {string} [opt.spec] 规格
       * @param {string} [opt.suffix] 后缀
       * @returns
       */
      async create_batch(feeds, opt = {}) {
        var body = {
          feeds: [],
        };
        for (let i = 0; i < feeds.length; i += 1) {
          var feed = feeds[i];
          var spec = (() => {
            if (opt.spec) {
              return opt.spec;
            }
            if (WXU.config.defaultHighest || opt.spec === null) {
              return null;
            }
            if (feed.spec && feed.spec[0]) {
              return feed.spec[0].fileFormat;
            }
            return null;
          })();
          var filename = WXU.build_filename(
            feed,
            spec,
            WXU.config.downloadFilenameTemplate,
          );
          if (filename) {
            var suffix = opt.suffix;
            if (feed.type === "picture") {
              suffix = ".zip";
              feed.url = build_picture_zip_url(feed);
            }
            if (suffix !== ".jpg") {
              if (spec) {
                feed.url = feed.url + "&X-snsvideoflag=" + spec;
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
            }
            body.feeds.push({
              id: feed.id,
              nonce_id: feed.nonce_id || feed.objectNonceId || "",
              url: feed.url,
              title: feed.title,
              key: Number(feed.key),
              filename,
              spec,
              suffix,
            });
          }
        }
        WXU.downloader.show();
        var [err, data] = await WXU.request({
          method: "POST",
          url: WXEnv.apiOrigin + "/api/task/create_batch",
          body,
        });
        if (err) {
          return [err, null];
        }
        return [null, data];
      },
    },
    /**
     * 视频解密
     */
    build_decrypt_arr: __wx_channels_decrypt,
    video_decrypt: __wx_channels_video_decrypt,
    async decrypt_video(buf, key) {
      try {
        const r = await __wx_channels_decrypt(key);
        if (r) {
          buf = __wx_channels_video_decrypt(buf, 0, {
            decryptor_array: r,
          });
          return [null, buf];
        }
        return [new Error("前端解密失败"), null];
      } catch (err) {
        return [err, null];
      }
    },
    /**
     * 类型转换相关
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
      return mediaBufferToWav(...args);
    },
    wav_to_mp3_blob: wavBlobToMP3,
    async media_to_mp3(buf) {
      var audioCtx = new AudioContext();
      return new Promise((resolve) => {
        audioCtx.decodeAudioData(buf, async function (buffer) {
          var blob = await mediaBufferToWav(buffer);
          var [err, data] = await wavBlobToMP3(blob);
          if (err) {
            return resolve([err, null]);
          }
          return resolve([null, data]);
        });
      });
    },
    download_with_progress,
    /**  */
    sleep,
    resultify(fn) {
      return (...args) => {
        return new Promise((resolve) => {
          fn(...args)
            .then((data) => {
              resolve([null, data]);
            })
            .catch((err) => {
              resolve([err, null]);
            });
        });
      };
    },
    uid: __wx_uid__,
    bytes_to_size,
    format_media_spec_label,
    format_media_spec_short_label,
    parseJSON(v) {
      try {
        var r = JSON.parse(v);
        return [null, r];
      } catch (err) {
        return [err, null];
      }
    },
    build_filename,
    build_picture_zip_files,
    build_picture_zip_url,
    load_script: __wx_load_script,
    find_elm: __wx_find_elm,
    get_queries,
    /**
     * 提示相关
     */
    copy: __wx_channels_copy,
    log: __wx_log,
    error: __wx_error,
    loading() {
      return __wx_loading();
    },
    toast(text) {
      return __wx_toast(text);
    },
    menu_item: __wx_menu_item,
    create_dropdown_menu: __wx_create_dropdown_menu,
    create_popover: __wx_create_popover,
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
    fetch: __wx_fetch,
    observe_node(selector, cb, error_cb) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        cb($existing);
        return;
      }
      var timer = null;
      if (error_cb) {
        timer = setTimeout(() => {
          observer.disconnect();
          error_cb();
        }, 5000);
      }
      var observer = new MutationObserver((mutations, obs) => {
        mutations.forEach((mutation) => {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach((node) => {
              if (node.nodeType === 1) {
                if (node.matches(selector) || node.querySelector(selector)) {
                  clearTimeout(timer);
                  cb(
                    node.matches(selector)
                      ? node
                      : node.querySelector(selector),
                  );
                  if (document.querySelector(selector)) {
                    obs.disconnect();
                  }
                }
              }
            });
          }
        });
      });
      WXU.onWindowLoaded(() => {
        const $root = document.getElementById("app");
        if (!$root) {
          return;
        }
        observer.observe($root, {
          childList: true,
          subtree: true,
        });
      });
    },
    /**
     * @param {{ url: string; method: 'GET' | 'POST'; body?: any }} opt
     */
    async request(opt) {
      return new Promise((resolve, reject) => {
        var xhr = new XMLHttpRequest();
        xhr.open(opt.method, opt.url);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onload = async function () {
          // console.log("[request]xhr.responseText", xhr.responseText);
          try {
            var data = JSON.parse(xhr.responseText);
            if (data.code !== 0) {
              const err = new Error(data.msg);
              err.code = data.code;
              err.data = data.data;
              err.response = data;
              resolve([err, null]);
              return;
            }
            resolve([null, data.data]);
          } catch (e) {
            // ignore
          }
          resolve([null, xhr.responseText]);
        };
        xhr.onerror = function (err) {
          // console.log("[request]xhr.onerror", err);
          resolve([new Error(err.type), null]);
        };
        xhr.send(JSON.stringify(opt.body));
      });
    },
    async save(blob, filename) {
      await __wx_load_script(__wx_asset_url("/lib/FileSaver.min.js"));
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(__wx_asset_url("/lib/jszip.min.js"));
      const zip = new JSZip();
      return zip;
    },
    /**
     * 向菜单前面插入额外菜单
     * @param {{label: string; onClick?:(event: { profile: ChannelsMedia }) => void}[]} items
     */
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
    /**
     * @returns {ChannelsConfig}
     */
    get config() {
      return WXEnv.config;
    },
    get version() {
      return __wx_channels_version__;
    },
    env: {
      get isChannels() {
        return WXEnv.isChannels;
      },
      get isWxwork() {
        return WXEnv.isWxwork;
      },
    },
  };
})();

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
/** 复制当前页面地址 */
function __wx_channels_handle_copy__() {
  WXU.copy(location.href);
  WXU.toast("复制成功");
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
          __wx_channels_handle_click_download__(null, true);
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
          __wx_channels_handle_copy__();
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
  Timeless.DOM.render(Timeless.shadcn.DropdownMenu({ store: dropdown$ }), mount);

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
    trigger.dataset.dropdownMenuImpl = "Timeless.shadcn.DropdownMenu";
  }
  trigger.__wxTimelessDownloadDropdown = dropdown$;
  return dropdown$;
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

if (typeof window.Timeless !== "undefined") {
  const timeless = window.Timeless;
  Object.assign(timeless, timeless.kit);
  Object.assign(timeless, timeless.headless);
  // Rendering
  window.h = timeless.h;
  window.View = timeless.View;
  window.Fragment = timeless.Fragment;
  // Control flow
  window.Show = timeless.Show;
  window.For = timeless.For;
  window.Switch = timeless.Switch;
  window.Match = timeless.Match;
  // Reactivity
  window.ref = timeless.ref;
  window.refobj = timeless.refobj;
  window.refarr = timeless.refarr;
  window.computed = timeless.computed;
  window.combine = timeless.combine;
  window.isElement = timeless.isElement;
  // Styling
  window.cn = timeless.cn;
  window.classNames = timeless.classNames;
  // Primitives
  window.PopoverPrimitive = timeless.PopoverPrimitive;
  window.DropdownMenuPrimitive = timeless.DropdownMenuPrimitive;
  window.WaterfallPrimitive = timeless.WaterfallPrimitive;
  window.ScrollViewPrimitive = timeless.ScrollViewPrimitive;
  window.DialogPrimitive = timeless.DialogPrimitive;
  // SVG helpers
  window.SVG = timeless.SVG;
  window.Circle = timeless.Circle;
}

WXU.onInit((data) => {
  __wx_username = data.mainFinderUsername;
});

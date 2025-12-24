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
  const files = profile.files;
  const zip = await WXU.Zip();
  zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
  const folder = zip.folder("images");
  const fetchPromises = files
    .map((f) => f.url)
    .map(async (url, index) => {
      const response = await fetch(url);
      const blob = await response.blob();
      folder.file(index + 1 + ".png", blob);
    });
  const ins = WXU.loading();
  try {
    await Promise.all(fetchPromises);
    const content = await zip.generateAsync({ type: "blob" });
    await WXU.save(content, profile.filename + ".zip");
  } catch (err) {
    WXU.error({ msg: "下载失败，" + err.message });
  }
  ins.hide();
}
/**
 * 下载加密视频
 * @param {FeedProfile} profile 视频信息
 * @param {object} opt 选项
 * @param {boolean} opt.toMP3 是否转换为 MP3
 */
async function __wx_channels_download4(profile, opt) {
  console.log("__wx_channels_download4");
  if (WXU.config.downloadLocalServerEnabled) {
    var fullname = profile.filename + (opt.toMP3 ? ".mp3" : ".mp4");
    var url = `http://${
      WXU.config.downloadLocalServerAddr
    }/download?url=${encodeURIComponent(profile.url)}&key=${
      profile.key
    }&filename=${encodeURIComponent(fullname)}&mp3=${Number(opt.toMP3)}`;
    var a = document.createElement("a");
    a.href = url;
    a.download = fullname;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    return;
  }
  if (WXU.config.downloadPauseWhenDownload) {
    WXU.pause_cur_video();
  }
  const ins = WXU.loading();
  var [err, response] = await WXU.fetch(profile.url);
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
  });
  WXU.log({ ignore_prefix: 1, msg: "" });
  var media_buf = new Uint8Array(await media_blob.arrayBuffer());
  if (profile.key) {
    WXU.log({ msg: "下载完成，开始解密" });
    var [err, data] = await WXU.decrypt_video(media_buf, profile.key);
    if (err) {
      WXU.error({ msg: "解密失败，" + err.message, alert: 0 });
      WXU.error({ msg: "尝试使用 decrypt 命令解密", alert: 0 });
    } else {
      WXU.log({ msg: "解密成功" });
      media_buf = data;
    }
  }
  if (opt.toMP3) {
    const [err, mp3_blob] = await WXU.media_to_mp3(media_buf.buffer);
    if (err) {
      WXU.error({ msg: err.message });
      return;
    }
    WXU.emit(WXU.Events.MP3Downloaded, profile);
    WXU.save(mp3_blob, profile.filename + ".mp3");
  } else {
    WXU.emit(WXU.Events.MediaDownloaded, profile);
    const result = new Blob([media_buf], { type: "video/mp4" });
    WXU.save(result, profile.filename + ".mp4");
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
 * @param {ChannelsMediaSpec | null} spec 规格信息
 * @param {boolean} mp3 是否转换为 MP3
 */
async function __wx_channels_handle_click_download__(spec, mp3) {
  const [err, feed] = WXU.check_feed_existing();
  if (err) return;
  const payload = { ...feed };
  var filename = WXU.build_filename(
    feed,
    spec,
    WXU.config.downloadFilenameTemplate
  );
  if (!filename) {
    WXU.error({ msg: "文件名生成失败" });
    return;
  }
  payload.mp3 = !!mp3;
  payload.filename = filename;
  payload.original_url = feed.url;
  payload.target_spec = null;
  if (spec) {
    payload.target_spec = spec;
    payload.url = feed.url + "&X-snsvideoflag=" + spec.fileFormat;
  }
  payload.source_url = location.href;
  WXU.log({
    msg: `${payload.source_url}
${payload.url}
${payload.key || "该视频未加密"}
${payload.filename}`,
  });
  WXU.emit(WXU.Events.BeforeDownloadMedia, payload);
  if (payload.type === "picture") {
    __wx_channels_download3(payload);
    return;
  }
  payload.data = __wx_channels_store__.buffers;
  __wx_channels_download4(payload, { toMP3: mp3 });
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
    WXU.config.downloadFilenameTemplate
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
    WXU.config.downloadFilenameTemplate
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
  var filename = WXU.build_filename(
    profile,
    null,
    WXU.config.downloadFilenameTemplate
  );
  if (!filename) {
    WXU.error({ msg: "文件名生成失败" });
    return;
  }
  WXU.log({ msg: `下载封面\n${profile.cover_url}` });
  const ins = WXU.loading();
  const url = profile.cover_url.replace(/^http:/, "https:");
  var [err, response] = await WXU.fetch(url);
  ins.hide();
  if (err) {
    WXU.error({ msg: err.message });
    return;
  }
  const blob = await response.blob();
  WXU.save(blob, filename + ".jpg");
}
/**
 * 为指定按钮添加额外的下载选项菜单
 * @param {HTMLElement} trigger
 */
function __wx_attach_download_dropdown_menu(trigger) {
  if (typeof window.Weui === "undefined") {
    return null;
  }
  const { DropdownMenu, Menu, MenuItem } = Weui;
  MenuItem.setTemplate(
    '<div class="custom-menu-item"><span class="label">{{ label }}</span></div>'
  );
  MenuItem.setIndicatorTemplate(
    '<span class="custom-menu-item-arrow">›</span>'
  );
  Menu.setTemplate('<div><div class="custom-menu">{{ list }}</div></div>');
  const submenu = Menu({
    children: [],
  });
  const $dropdown = DropdownMenu({
    $trigger: trigger,
    zIndex: 99999,
    children: [
      ...(() => {
        if (WXU.before_menu_items) {
          return render_extra_menu_items(WXU.before_menu_items, {
            hide() {
              $dropdown.hide();
            },
          });
        }
        return [];
      })(),
      MenuItem({
        label: "更多下载",
        submenu,
        onMouseEnter() {
          submenu.show();
        },
        onMouseLeave() {
          if (!submenu.isHover) {
            submenu.hide();
          }
        },
      }),
      MenuItem({
        label: "下载为MP3",
        onClick() {
          __wx_channels_handle_click_download__(null, true);
          $dropdown.hide();
        },
      }),
      MenuItem({
        label: "下载封面",
        onClick() {
          __wx_channels_handle_download_cover();
          $dropdown.hide();
        },
      }),
      MenuItem({
        label: "打印下载命令",
        onClick() {
          __wx_channels_handle_print_download_command();
          $dropdown.hide();
        },
      }),
      MenuItem({
        label: "复制页面链接",
        onClick() {
          __wx_channels_handle_copy__();
          $dropdown.hide();
        },
      }),
      ...(() => {
        if (WXU.after_menu_items) {
          return render_extra_menu_items(WXU.after_menu_items, {
            hide() {
              $dropdown.hide();
            },
          });
        }
        return [];
      })(),
    ],
    onMouseEnter() {
      if (submenu.isOpen) {
        submenu.hide();
      }
    },
  });
  $dropdown.ui.$trigger.onMouseEnter(() => {
    const download_menus = [
      MenuItem({
        label: "原始视频",
        onClick() {
          __wx_channels_handle_click_download__(null);
          $dropdown.hide();
        },
      }),
      MenuItem({
        label: "当前视频",
        onClick() {
          __wx_channels_download_cur__();
          $dropdown.hide();
        },
      }),
      ...(() => {
        const [err, profile] = WXU.check_feed_existing({
          silence: true,
        });
        if (err) {
          return [];
        }
        // console.log("[main.js]before profile.spec.map", profile);
        return profile.spec.map((spec) => {
          return MenuItem({
            label: spec.fileFormat,
            onClick() {
              __wx_channels_handle_click_download__(spec);
              $dropdown.hide();
            },
          });
        });
      })(),
    ];
    submenu.setChildren(download_menus);
    $dropdown.show();
  });
  $dropdown.ui.$trigger.onMouseLeave(() => {
    if ($dropdown.isHover) {
      return;
    }
    $dropdown.hide();
  });
  return $dropdown;
}
function __wx_download_btn_handler() {
  const [err, profile] = WXU.check_feed_existing();
  if (err) return;
  var spec = WXU.config.defaultHighest ? null : profile.spec[0];
  __wx_channels_handle_click_download__(spec);
}
/**
 * 为「首页/推荐」添加下载按钮
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
    () => $item.getElementsByClassName("click-box op-item")[0]
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

async function __wx_insert_download_btn() {
  if (window.location.pathname.includes("/pages/home")) {
    const success = await __wx_insert_download_btn_to_home_page();
    if (success) {
      return;
    }
    return;
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
      return;
    }
    $elm2.insertBefore($btn, relative_node);
    return;
  }
  var $elm1 = await WXU.find_elm(function () {
    return document.getElementsByClassName("full-opr-wrp layout-row")[0];
  });
  if ($elm1) {
    var relative_node = $elm1.children[$elm1.children.length - 1];
    if (!relative_node) {
      $elm1.appendChild($btn);
      return;
    }
    $elm1.insertBefore($btn, relative_node);
    return;
  }
  __wx_render_footer_tools();
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
  var error_tip_timer = setTimeout(() => {
    WXU.error({ msg: "没有获取到视频详情", alert: 0 });
  }, 5000);
  var home_page_mounted = false;
  WXU.onPCFlowLoaded((feeds) => {
    console.log("[main.js]WXU.onPCFlowLoaded", feeds);
    if (home_page_mounted) {
      return;
    }
    home_page_mounted = true;
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    __wx_insert_download_btn_to_home_page();
  });
  var profile_page_mounted = false;
  WXU.onFetchFeedProfile((feed) => {
    console.log("[main.js]WXU.onFetchFeedProfile for page", feed);
    if (profile_page_mounted) {
      return;
    }
    profile_page_mounted = true;
    clearTimeout(error_tip_timer);
    error_tip_timer = null;
    WXU.set_cur_video();
    __wx_insert_download_btn();
  });
  WXU.onGotoNextFeed((feed) => {
    console.log("[main.js]WXU.onGotoNextFeed", feed);
    WXU.set_cur_video();
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onGotoPrevFeed((feed) => {
    console.log("[main.js]WXU.onGotoPrevFeed", feed);
    WXU.set_cur_video();
    __wx_insert_download_btn_to_home_page();
  });
  WXU.onFeed((feed) => {
    console.log("[main.js]WXU.onFeed", feed);
    WXU.set_feed(feed);
  });
})();

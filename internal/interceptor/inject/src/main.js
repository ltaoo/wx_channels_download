/** 全局的存储 */
var __wx_channels_store__ = {
  profile: null,
  profiles: [],
  keys: {},
  buffers: [],
};
/**
 * 用于下载已经播放的视频内容
 * @param {ChannelsMediaProfile} profile 视频信息
 */
async function __wx_channels_download(profile) {
  console.log("__wx_channels_download");
  const data = profile.data;
  const blob = new Blob(data, { type: "video/mp4" });
  ChannelsUtil.save(blob, profile.filename);
}
/**
 * 下载非加密视频
 * @deprecated 现在统一调用 __wx_channels_download4 方法，在方法内会判断是否需要解密
 * @param {ChannelsMediaProfile} profile 视频信息
 * @param {string} filename 文件名
 */
async function __wx_channels_download2(profile) {
  console.log("__wx_channels_download2");
  const url = profile.url;
  const ins = ChannelsUtil.loading();
  var [err, response] = await ChannelsUtil.fetch(url);
  ins.hide();
  if (err) {
    ChannelsUtil.error({ msg: err.message });
    return;
  }
  const blob = await ChannelsUtil.download_with_progress(response, {
    onStart({ total_size }) {
      ChannelsUtil.log({ msg: `总大小 ${total_size} Bytes` });
    },
    onProgress({ loaded_size, progress }) {
      ChannelsUtil.log({
        replace: 1,
        msg: progress === null ? `${loaded_size} Bytes` : `${progress}%`,
      });
    },
  });
  ChannelsUtil.log({ ignore_prefix: 1, msg: "" });
  ChannelsUtil.log({ msg: "下载完成" });
  ChannelsUtil.save(blob, profile.filename + ".mp4");
}
/**
 * 下载图片视频
 * @param {ChannelsMediaProfile} profile 视频信息
 */
async function __wx_channels_download3(profile) {
  console.log("__wx_channels_download3");
  const files = profile.files;
  const zip = ChannelsUtil.Zip();
  zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
  const folder = zip.folder("images");
  const fetchPromises = files
    .map((f) => f.url)
    .map(async (url, index) => {
      const response = await fetch(url);
      const blob = await response.blob();
      folder.file(index + 1 + ".png", blob);
    });
  const ins = ChannelsUtil.loading();
  try {
    await Promise.all(fetchPromises);
    const content = await zip.generateAsync({ type: "blob" });
    await ChannelsUtil.save(content, profile.filename + ".zip");
  } catch (err) {
    const msg = "下载失败\n" + err.message;
    ChannelsUtil.error({ msg });
  }
  ins.hide();
}
/**
 * 下载加密视频
 * @param {ChannelsMediaProfile} profile 视频信息
 * @param {object} opt 选项
 * @param {boolean} opt.toMP3 是否转换为 MP3
 */
async function __wx_channels_download4(profile, opt) {
  console.log("__wx_channels_download4");
  if (__wx_channels_config__.downloadLocalServerEnabled) {
    var fullname = profile.filename + (opt.toMP3 ? ".mp3" : ".mp4");
    var url = `http://${
      __wx_channels_config__.downloadLocalServerAddr
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
  if (__wx_channels_config__.downloadPauseWhenDownload) {
    ChannelsUtil.pause_cur_video();
  }
  const ins = ChannelsUtil.loading();
  var [err, response] = await ChannelsUtil.fetch(profile.url);
  if (err) {
    ChannelsUtil.error({ msg: err.message });
    return;
  }
  const blob = await ChannelsUtil.download_with_progress(response, {
    onStart({ total_size }) {
      ChannelsUtil.log({ msg: `总大小 ${total_size} Bytes` });
    },
    onProgress({ loaded_size, progress }) {
      ChannelsUtil.log({
        replace: 1,
        msg: progress === null ? `${loaded_size} Bytes` : `${progress}%`,
      });
    },
  });
  ChannelsUtil.log({ ignore_prefix: 1, msg: "" });
  ChannelsUtil.log({ msg: "下载完成，开始解密" });
  var media_buf = new Uint8Array(await blob.arrayBuffer());
  if (profile.key) {
    const [err, data] = ChannelsUtil.decrypt_video(media_buf, profile.key);
    if (err) {
      const tip = "前端解密失败，通过命令行使用 decrypt 命令解密";
      ChannelsUtil.error({ msg: tip });
    } else {
      media_buf = data;
    }
  }
  if (opt.toMP3) {
    const [err, mp3_blob] = await ChannelsUtil.media_to_mp3(media_buf.buffer);
    if (err) {
      ChannelsUtil.error({ msg: err.message });
      return;
    }
    ChannelsEventBus.emit(ChannelsEvents.MP3Downloaded, profile);
    ChannelsUtil.save(mp3_blob, profile.filename + ".mp3");
  } else {
    ChannelsEventBus.emit(ChannelsEvents.MediaDownloaded, profile);
    const result = new Blob([media_buf], { type: "video/mp4" });
    ChannelsUtil.save(result, profile.filename + ".mp4");
  }
  ins.hide();
  if (__wx_channels_config__.downloadPauseWhenDownload) {
    ChannelsUtil.play_cur_video();
  }
}
/**
 * 使用本地下载服务转换为mp3并下载
 * @deprecated 使用 __wx_channels_download4 方法并指定下载为 mp3
 * @param {ChannelsMediaProfile} profile 视频信息
 */
async function __wx_channels_download_as_mp3(profile) {
  console.log("__wx_channels_download_as_mp3");
  if (!__wx_channels_config__.downloadLocalServerEnabled) {
    ChannelsUtil.error({ msg: "请先开启本地下载服务" });
    return;
  }
  const url = `http://${
    __wx_channels_config__.downloadLocalServerAddr
  }/download?url=${encodeURIComponent(profile.url)}&key=${
    profile.key
  }&mp3=1&filename=${encodeURIComponent(profile.filename + ".mp3")}`;
  window.open(url);
}
/** 复制当前页面地址 */
function __wx_channels_handle_copy__() {
  ChannelsUtil.copy(location.href);
  ChannelsUtil.toast("复制成功");
}
async function __wx_channels_handle_log__() {
  const content = document.body.innerHTML;
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
  ChannelsUtil.save(blob, "log.txt");
}
/**
 * 所有下载功能统一先调用该方法
 * 由该方法分发到具体的 download 方法中
 * @param {ChannelsMediaSpec | null} spec 规格信息
 * @param {boolean} mp3 是否转换为 MP3
 */
async function __wx_channels_handle_click_download__(spec, mp3) {
  const [err, profile] = ChannelsUtil.check_profile_existing();
  if (err) return;
  const _profile = { ...profile };
  var filename = ChannelsUtil.build_filename(
    profile,
    spec,
    __wx_channels_config__.downloadFilenameTemplate
  );
  if (!filename) {
    ChannelsUtil.error({ msg: "文件名生成失败" });
    return;
  }
  _profile.filename = filename;
  _profile.original_url = profile.url;
  _profile.target_spec = null;
  if (spec) {
    _profile.target_spec = spec;
    _profile.url = profile.url + "&X-snsvideoflag=" + spec.fileFormat;
  }
  _profile.source_url = location.href;
  ChannelsUtil.log({
    msg: `${_profile.filename}
${_profile.source_url}

${_profile.url}
${_profile.key || "该视频未加密"}`,
  });
  ChannelsEventBus.emit(ChannelsEvents.BeforeDownloadMedia, _profile);
  if (_profile.type === "picture") {
    __wx_channels_download3(_profile);
    return;
  }
  _profile.data = __wx_channels_store__.buffers;
  __wx_channels_download4(_profile, { toMP3: mp3 });
}
/** 下载已加载的视频 */
function __wx_channels_download_cur__() {
  const [err, profile] = ChannelsUtil.check_profile_existing();
  if (err) return;
  if (__wx_channels_store__.buffers.length === 0) {
    alert("没有可下载的内容");
    return;
  }
  var filename = ChannelsUtil.build_filename(
    profile,
    null,
    __wx_channels_config__.downloadFilenameTemplate
  );
  if (!filename) {
    alert("文件名生成失败");
    return;
  }
  profile.filename = filename;
  profile.data = __wx_channels_store__.buffers;
  __wx_channels_download(profile);
}
/** 打印下载原始文件命令 */
function __wx_channels_handle_print_download_command() {
  const [err, profile] = ChannelsUtil.check_profile_existing();
  if (err) return;
  var _profile = { ...profile };
  var filename = ChannelsUtil.build_filename(
    _profile,
    null,
    __wx_channels_config__.downloadFilenameTemplate
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
  ChannelsUtil.log({ msg: command });
  ChannelsUtil.toast("请在终端查看下载命令");
}
/** 下载视频封面 */
async function __wx_channels_handle_download_cover() {
  var [err, profile] = ChannelsUtil.check_profile_existing();
  if (err) return;
  var filename = ChannelsUtil.build_filename(
    profile,
    null,
    __wx_channels_config__.downloadFilenameTemplate
  );
  if (!filename) {
    ChannelsUtil.error({ msg: "文件名生成失败" });
    return;
  }
  ChannelsUtil.log({ msg: `下载封面\n${profile.coverUrl}` });
  const ins = ChannelsUtil.loading();
  const url = profile.coverUrl.replace(/^http:/, "https:");
  var [err, response] = await ChannelsUtil.fetch(url);
  ins.hide();
  if (err) {
    ChannelsUtil.error({ msg: err.message });
    return;
  }
  const blob = await response.blob();
  ChannelsUtil.save(blob, filename + ".jpg");
}
/**
 * 为指定按钮添加额外的下载选项菜单
 * @param {HTMLElement} trigger
 */
function attach_download_dropdown_menu(trigger) {
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
        if (window.beforeExtraMenuItems) {
          return render_extra_menu_items(window.beforeExtraMenuItems, {
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
        if (window.postExtraMenuItems) {
          return render_extra_menu_items(window.postExtraMenuItems, {
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
        const [err, profile] = ChannelsUtil.check_profile_existing({
          silence: true,
        });
        if (err) {
          return [];
        }
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
  const [err, profile] = ChannelsUtil.check_profile_existing();
  if (err) return;
  var spec = __wx_channels_config__.defaultHighest ? null : profile.spec[0];
  __wx_channels_handle_click_download__(spec);
}
/**
 * 为「首页/推荐」添加下载按钮
 */
async function __insert_download_btn_to_home_page() {
  var $container = await ChannelsUtil.find_elm(function () {
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
  var $elm3 = await ChannelsUtil.find_elm(
    () => $item.getElementsByClassName("click-box op-item")[0]
  );
  if (!$elm3) {
    return false;
  }
  const $parent = $elm3.parentElement;
  if ($parent) {
    const $btn = download_btn2();
    attach_download_dropdown_menu($btn);
    $btn.onclick = __wx_download_btn_handler;
    $parent.appendChild($btn);
    return true;
  }
  render_sider_tools();
  return false;
}

async function insert_download_btn() {
  ChannelsUtil.log({ msg: "等待注入下载按钮" });
  if (window.location.pathname.includes("/pages/home")) {
    const success = await __insert_download_btn_to_home_page();
    if (success) {
      ChannelsUtil.log({ msg: "注入下载按钮成功!" });
      return;
    }
    return;
  }
  const $btn = download_btn1();
  attach_download_dropdown_menu($btn);
  $btn.onclick = __wx_download_btn_handler;
  var $elm2 = await ChannelsUtil.find_elm(function () {
    return document.getElementsByClassName("full-opr-wrp layout-col")[0];
  });
  if ($elm2) {
    var relative_node = $elm2.children[$elm2.children.length - 1];
    if (!relative_node) {
      ChannelsUtil.log({ msg: "注入下载按钮3成功!" });
      $elm2.appendChild($btn);
      return;
    }
    ChannelsUtil.log({ msg: "注入下载按钮4成功!" });
    $elm2.insertBefore($btn, relative_node);
    return;
  }
  var $elm1 = await ChannelsUtil.find_elm(function () {
    return document.getElementsByClassName("full-opr-wrp layout-row")[0];
  });
  if ($elm1) {
    var relative_node = $elm1.children[$elm1.children.length - 1];
    if (!relative_node) {
      ChannelsUtil.log({ msg: "注入下载按钮1成功!" });
      $elm1.appendChild($btn);
      return;
    }
    ChannelsUtil.log({ msg: "注入下载按钮2成功!" });
    $elm1.insertBefore($btn, relative_node);
    return;
  }
  render_footer_tools();
}
/**
 * 在视频详情页底部添加悬浮下载按钮
 */
function render_footer_tools() {
  const $fixed_footer = document.createElement("div");
  $fixed_footer.className = "wx-footer";
  const $tools = document.createElement("div");
  $tools.className = "wx-footer-tools";
  const $btn = document.createElement("div");
  $btn.className = "weui-btn weui-btn_default weui-btn_mini";
  $btn.innerHTML = "下载";
  $btn.onclick = __wx_download_btn_handler;
  attach_download_dropdown_menu($btn);
  document.body.appendChild($fixed_footer);
  $fixed_footer.appendChild($tools);
  $tools.appendChild($btn);
}
/**
 * 在首页右侧添加悬浮下载按钮
 */
function render_sider_tools() {
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
  attach_download_dropdown_menu($btn);
  document.body.appendChild($fixed_sider);
  $fixed_sider.appendChild($sider_bg);
  $fixed_sider.appendChild($tools);
  $tools.appendChild($btn);
}

var timer = setTimeout(() => {
  ChannelsUtil.error({ msg: "没有捕获到视频详情" });
}, 2000);
var home_mounted = false;
ChannelsEventBus.onFetchFeedProfile((profile) => {
  if (home_mounted) {
    return;
  }
  home_mounted = true;
  clearTimeout(timer);
  timer = null;
  insert_download_btn();
});

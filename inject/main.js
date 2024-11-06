function __wx_channels_copy(text) {
  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
  document.body.appendChild(textArea);
  textArea.select();
  document.execCommand("copy");
  document.body.removeChild(textArea);
}
function __wx_channels_video_decrypt(t, e, p) {
  for (
    var r = new Uint8Array(t), n = 0;
    n < t.byteLength && e + n < p.decryptor_array.length;
    n++
  )
    r[n] ^= p.decryptor_array[n];
  return r;
}
async function __wx_channels_download(profile, filename) {
  console.log("__wx_channels_download", profile.data);
  const data = profile.data;
  const blob = new Blob(data, { type: "video/mp4" });
  await __wx_load_script(
    "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
  );
  saveAs(blob, filename + ".mp4");
}
async function __wx_channels_download2(profile, filename) {
  console.log("__wx_channels_download2");
  const url = profile.url;
  await __wx_load_script(
    "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
  );
  const response = await fetch(url);
  const blob = await response.blob();
  saveAs(blob, filename + ".mp4");
}
async function __wx_channels_download3(profile, filename) {
  console.log("__wx_channels_download3");
  const files = profile.files;
  await __wx_load_script(
    "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
  );
  const zip = new JSZip();
  zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
  const folder = zip.folder("images");
  const fetchPromises = files
    .map((f) => f.url)
    .map(async (url, index) => {
      const response = await fetch(url);
      const blob = await response.blob();
      folder.file(index + 1 + ".png", blob);
    });
  await Promise.all(fetchPromises);
  const content = await zip.generateAsync({ type: "blob" });
  saveAs(content, filename + ".zip");
}
async function __wx_channels_download4(profile, filename) {
  const url = profile.url;
  console.log("__wx_channels_download4", url);
  fetch("/__wx_channels_api/tip", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      msg: `${filename}
${url}`,
    }),
  });
  await __wx_load_script(
    "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
  );
  const ins = window.__wx_channels_tip__.loading("下载中");
  const response = await fetch(url);
  const blob = await response.blob();
  let array = new Uint8Array(await blob.arrayBuffer());
  if (profile.decryptor_array) {
    array = __wx_channels_video_decrypt(array, 0, profile);
  }
  ins.hide();
  const result = new Blob([array], { type: "video/mp4" });
  saveAs(result, filename + ".mp4");
}
function __wx_load_script(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.type = "text/javascript";
    script.src = src;
    script.onload = resolve;
    script.onerror = reject;
    document.head.appendChild(script);
  });
}
function __wx_channels_handle_copy__() {
  __wx_channels_copy(location.href);
  if (window.__wx_channels_tip__ && window.__wx_channels_tip__.toast) {
    window.__wx_channels_tip__.toast("复制成功", 1e3);
  }
}
async function __wx_channels_handle_log__() {
  await __wx_load_script(
    "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js"
  );
  const content = document.body.innerHTML;
  const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
  saveAs(blob, "log.txt");
}
function __wx_channels_handle_click_download__(spec) {
  var profile = __wx_channels_store__.profile;
  profile = __wx_channels_store__.profiles.find((p) => p.id === profile.id);
  if (!profile) {
    alert("检测不到视频，请将本工具更新到最新版");
    return;
  }
  console.log(__wx_channels_store__);
  var filename = (() => {
    if (profile.title) {
      return profile.title;
    }
    if (profile.id) {
      return profile.id;
    }
    return new Date().valueOf();
  })();
  if (profile && profile.type === "picture") {
    __wx_channels_download3(profile, filename);
    return;
  }
  if (profile && __wx_channels_store__.buffers.length === 0) {
    __wx_channels_download2(profile, filename);
    return;
  }
  profile.data = __wx_channels_store__.buffers;
  profile.decryptor_array = __wx_channels_store__.keys[profile.key];
  if (!profile.decryptor_array) {
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        msg: `解密失败，停止下载`,
      }),
    });
    alert("解密失败，停止下载");
    return;
  }
  const _profile = {
    ...profile,
  };
  if (spec) {
    _profile.url = profile.url + "&X-snsvideoflag=" + spec.fileFormat;
    filename = filename + "_" + spec.fileFormat;
  }
  __wx_channels_download4(_profile, filename);
}
function __wx_channels_download_cur__() {
  var profile = __wx_channels_store__.profile;
  if (!profile) {
    alert("检测不到视频，请将本工具更新到最新版");
    return;
  }
  if (__wx_channels_store__.buffers.length === 0) {
    alert("没有可下载的内容");
    return;
  }
  var filename = (() => {
    if (profile.title) {
      return profile.title;
    }
    if (profile.id) {
      return profile.id;
    }
    return new Date().valueOf();
  })();
  profile.data = __wx_channels_store__.buffers;
  __wx_channels_download(profile, filename);
}
var __wx_channels_tip__ = {};
var __wx_channels_store__ = {
  profile: null,
  profiles: [],
  keys: {},
  buffers: [],
};
var $icon = document.createElement("div");
$icon.innerHTML =
  '<div data-v-6548f11a data-v-c2373d00 class="click-box op-item item-gap-combine" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;"><svg data-v-c2373d00 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg></div>';
var __wx_channels_video_download_btn__ = $icon.firstChild;
__wx_channels_video_download_btn__.onclick = () => {
  if (!window.__wx_channels_store__.profile) {
    return;
  }
  __wx_channels_handle_click_download__(
    window.__wx_channels_store__.profile.spec[0]
  );
};
var count = 0;
fetch("/__wx_channels_api/tip", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    msg: "等待注入下载按钮",
  }),
});
var __timer = setInterval(() => {
  count += 1;
  // const $wrap1 = document.getElementsByClassName("feed-card-wrap")[0];
  // const $wrap2 = document.getElementsByClassName(
  //   "operate-row transition-show"
  // )[0];
  const $wrap3 = document.getElementsByClassName("full-opr-wrp layout-row")[0];
  if (!$wrap3) {
    if (count >= 5) {
      clearInterval(__timer);
      __timer = null;
      fetch("/__wx_channels_api/tip", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ msg: "没有找到操作栏，注入下载按钮失败" }),
      });
    }
    return;
  }
  clearInterval(__timer);
  __timer = null;
  const relative_node = $wrap3.children[$wrap3.children.length - 1];
  if (!relative_node) {
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ msg: "注入下载按钮成功1!" }),
    });
    $wrap3.appendChild(__wx_channels_video_download_btn__);
    return;
  }
  fetch("/__wx_channels_api/tip", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ msg: "注入下载按钮成功2!" }),
  });
  $wrap3.insertBefore(__wx_channels_video_download_btn__, relative_node);
}, 1000);

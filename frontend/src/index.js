import * as dmui from "./dmui.js";
import * as components from "./components.js";
import * as store from "./store.js";

const {
  app$,
  history$,
  http_client$,
  router,
  router$,
  storage$,
} = store;

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

Timeless.ui.ScrollViewPrimitive.setScrollViewProvider(Timeless.web);

window.config = window.__d_config || {};

window.format_time = function format_time(
  value,
  fallback_message = "时间未知",
  format_options = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  },
) {
  const timestamp = Number(value);
  let date;
  if (Number.isFinite(timestamp)) {
    if (timestamp <= 0) {
      return fallback_message;
    }
    const normalized =
      timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
    date = new Date(normalized);
  } else {
    date = new Date(value);
  }
  return Number.isNaN(date.getTime())
    ? fallback_message
    : new Intl.DateTimeFormat("zh-CN", format_options).format(date);
};

window.request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    if (typeof payload.code === "undefined") {
      return Timeless.Result.Ok(payload);
    }
    if (payload.code !== 0) {
      return Timeless.Result.Err(
        payload.msg || "请求失败",
        payload.code,
        payload.data,
      );
    }
    return Timeless.Result.Ok(payload.data || {});
  },
});

window.__store = store;

Object.assign(window, dmui, components);

window.h = Timeless.h;
window.View = Timeless.View;
window.Fragment = Timeless.Fragment;
window.Img = Timeless.Img;
window.Link = Timeless.Link;
// Control flow
window.Show = Timeless.Show;
window.For = Timeless.For;
window.Switch = Timeless.Switch;
window.Match = Timeless.Match;
// Reactivity
window.ref = Timeless.ref;
window.refobj = Timeless.refobj;
window.refarr = Timeless.refarr;
window.computed = Timeless.computed;
window.combine = Timeless.combine;
window.isElement = Timeless.isElement;
// Styling
window.cn = Timeless.classNames;
window.classNames = Timeless.classNames;
// SVG helpers
window.SVG = Timeless.SVG;
window.Circle = Timeless.Circle;

window.API_ORIGIN = window.config.remoteServerEnabled
  ? "https://weixin110.qq.com"
  : window.config.apiOrigin || window.location.origin;

window.PLATFORM_FAVICONS = Object.freeze({
  wxchannels:
    "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
  wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
  douyin: "https://p-pc-weboff.byteimg.com/tos-cn-i-9r5gewecjs/favicon.png",
  youtube: "https://www.youtube.com/s/desktop/0084d708/img/favicon.ico",
  bilibili: "https://static.hdslb.com/images/favicon.ico",
  cctv: "https://v.cctv.com/favicon.ico",
});

window.PLATFORM_NAMES = Object.freeze({
  wxchannels: "视频号",
  wxmp: "公众号",
  officialaccount: "公众号",
  douyin: "抖音",
  bilibili: "Bilibili",
  xiaohongshu: "小红书",
  xhs: "小红书",
  youtube: "YouTube",
  zhihu: "知乎",
  douban: "豆瓣",
  weibo: "微博",
  qidian: "起点中文网",
  fanqienovel: "番茄小说",
  "69shuba": "69书吧",
  ttk: "TT看书",
  webpage: "网页",
});

window.CONTENT_TYPE_NAMES = Object.freeze({
  video: "视频",
  long_video: "长视频",
  episode: "单集",
  series: "系列",
  collection: "合集",
  short_video: "短视频",
  image: "图片",
  image_set: "图集",
  album: "图集",
  article: "文章",
  answer: "回答",
  question: "问题",
  post: "帖子",
  blog: "文章",
  webpage: "网页",
  novel: "小说",
  audio: "音频",
  podcast: "播客",
  music: "音乐",
  document: "文档",
  course: "课程",
  comic: "漫画",
  live: "直播",
});

window.CONTENT_RELATION_NAMES = Object.freeze({
  answer_of: "回答所属问题",
  contains: "包含",
  part_of: "属于",
  episode_of: "单集属于系列",
  reply_to: "回复",
  quote_of: "引用",
  repost_of: "转发",
  translation_of: "翻译自",
  derived_from: "派生自",
  related: "相关内容",
});

window.TYPE_ICONS = Object.freeze({
  image: "file-image",
  video: "file-play",
  audio: "file-volume",
  html: "file-code",
  zip: "file-box",
  pdf: "file-text",
  other: "file",
});

window.TYPE_LABELS = Object.freeze({
  image: "图片",
  video: "视频",
  audio: "音频",
  html: "HTML",
  zip: "压缩包",
  pdf: "PDF",
  other: "文件",
});

window.DOWNLOAD_RESOURCE_SUFFIXES = Object.freeze({
  image: ".jpg",
  video: ".mp4",
  audio: ".mp3",
  html: ".html",
  text: ".txt",
  json: ".json",
  "image/jpeg": ".jpg",
  "image/png": ".png",
  "image/gif": ".gif",
  "image/webp": ".webp",
  "image/avif": ".avif",
  "image/svg+xml": ".svg",
  "image/bmp": ".bmp",
  "image/tiff": ".tiff",
  "video/mp4": ".mp4",
  "video/webm": ".webm",
  "video/quicktime": ".mov",
  "video/x-msvideo": ".avi",
  "video/x-matroska": ".mkv",
  "video/mp2t": ".ts",
  "video/x-flv": ".flv",
  "audio/mpeg": ".mp3",
  "audio/mp4": ".m4a",
  "audio/aac": ".aac",
  "audio/ogg": ".ogg",
  "audio/wav": ".wav",
  "audio/flac": ".flac",
  "text/html": ".html",
  "text/plain": ".txt",
  "text/css": ".css",
  "text/csv": ".csv",
  "text/markdown": ".md",
  "application/json": ".json",
  "application/xml": ".xml",
  "application/pdf": ".pdf",
  "application/zip": ".zip",
});

function ApplicationRootView() {
  return View({ class: "application-root page" }, [
    Timeless.ui.KeepAliveSubViews({
      app: app$,
      client: http_client$,
      history: history$,
      storage: storage$,
      view: history$.$view,
      views: router.views,
      placeholder: LoadingView,
      ErrorFallback: ErrorFallbackView,
    }),
  ]);
}

function bootstrap() {
  var $root = document.querySelector("#root");
  if (!$root) {
    throw new Error("应用无法启动：缺少 App Model 或根节点");
  }
  router$.prepare(window.location);
  app$.start({
    width: window.innerWidth,
    height: window.innerHeight,
  });
  Timeless.DOM.render(ApplicationRootView(), $root);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
} else {
  bootstrap();
}

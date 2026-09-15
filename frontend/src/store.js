import "./dmui.js";
import SiderLayoutView from "./pages/shell.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

Timeless.ui.ScrollViewPrimitive.setScrollViewProvider(Timeless.web);
Timeless.ui.InputPrimitive.setInputProvider(Timeless.web);

const storage_key = "wx_channels_download";
const legacy_scraper_job_key = "scraper.active_scraper_job_id";
const legacy_downloader_key = "wx_channels_download.third_party_downloader.v1";

function read_json(client, key) {
  try {
    const value = JSON.parse(client.getItem(key) || "{}");
    return value && typeof value === "object" && !Array.isArray(value)
      ? value
      : {};
  } catch {
    return {};
  }
}

function has_own(source, key) {
  return Object.prototype.hasOwnProperty.call(source, key);
}

function load_storage_values(client) {
  const values = read_json(client, storage_key);
  if (!has_own(values, "third_party_downloader")) {
    const legacy_downloader = read_json(client, legacy_downloader_key);
    if (Object.keys(legacy_downloader).length > 0) {
      values.third_party_downloader = legacy_downloader;
    }
  }
  if (!has_own(values, "scraper_active_job_id")) {
    try {
      values.scraper_active_job_id =
        client.getItem(legacy_scraper_job_key) || "";
    } catch {
      values.scraper_active_job_id = "";
    }
  }
  return values;
}

const css_records = Object.create(null);
const load_records = Object.create(null);

function versioned_url(resource_url) {
  const resource = new URL(resource_url, document.baseURI);
  const version = String(
    (window.__d_config && window.__d_config.version) || "",
  ).trim();
  if (version) {
    resource.searchParams.set("v", version);
  }
  return resource.href;
}

/**
 * @param {string} module_url
 * @param {string | undefined} css_url
 * @returns {Promise<HTMLLinkElement | null>}
 */
function load_style(module_url, css_url) {
  if (!css_url) {
    return Promise.resolve(null);
  }

  const style_href = versioned_url(css_url);
  const current_record = css_records[style_href];
  if (current_record && current_record.status === "loaded") {
    return Promise.resolve(current_record.link);
  }
  if (current_record && current_record.status === "loading") {
    return current_record.promise;
  }

  let resolve_load;
  let reject_load;
  const promise = new Promise(function (resolve, reject) {
    resolve_load = resolve;
    reject_load = reject;
  });
  const link = document.createElement("link");
  const record = {
    link,
    promise,
    status: "loading",
    url: css_url,
  };

  css_records[style_href] = record;
  link.rel = "stylesheet";
  link.href = style_href;
  link.dataset.module = module_url;
  link.onload = function () {
    record.status = "loaded";
    resolve_load(link);
  };
  link.onerror = function () {
    record.status = "failed";
    reject_load(new Error("页面样式加载失败：" + css_url));
  };
  document.head.appendChild(link);

  return promise;
}

/**
 * @param {string} module_url
 * @param {string | undefined} css_url
 * @returns {Promise<Function>}
 */
function load(module_url, css_url) {
  const module_href = versioned_url(module_url);
  const current_record = load_records[module_href];
  if (current_record && current_record.status === "loaded") {
    return load_style(module_url, css_url).then(function () {
      return current_record.component;
    });
  }
  if (current_record && current_record.status === "loading") {
    return current_record.promise;
  }

  const record = {
    component: null,
    promise: null,
    status: "loading",
    css_url,
    url: module_href,
  };

  load_records[module_href] = record;
  record.promise = Promise.all([
    load_style(module_url, css_url),
    import(module_href),
  ])
    .then(function (results) {
      const page_module = results[1];
      const component = page_module && page_module.default;
      if (typeof component !== "function") {
        throw new TypeError(
          "页面模块必须 default export View 工厂函数：" + module_url,
        );
      }
      record.component = component;
      record.status = "loaded";
      return component;
    })
    .catch(function (error) {
      record.status = "failed";
      throw error;
    });

  return record.promise;
}

function lazy(module_url, css_url) {
  return function () {
    return load(module_url, css_url);
  };
}

const route_animation = Object.freeze({
  in: "route-view--enter",
  out: "route-view--exit",
});
const animated_route_options = Object.freeze({ animation: route_animation });

const routes_configure = {
  filehelper: {
    title: "微信文件传输助手",
    pathname: "/filehelper",
    component: lazy("src/pages/filehelper.js", "src/pages/filehelper.css"),
    options: animated_route_options,
  },
  preview: {
    title: "预览",
    pathname: "/preview",
    component: lazy("src/pages/preview.js", "src/pages/preview.css"),
    options: animated_route_options,
  },
  shell: {
    title: "首页",
    pathname: "/",
    component: SiderLayoutView,
    children: {
      download: {
        is_default: true,
        title: "下载",
        pathname: "/download",
        component: lazy("src/pages/downloadv2.js", "src/pages/downloadv2.css"),
        // options: animated_route_options,
      },
      scraper: {
        title: "内容抓取",
        pathname: "/scraper",
        component: lazy("src/pages/scraper.js", "src/pages/scraper.css"),
        // options: animated_route_options,
      },
      content: {
        title: "内容管理",
        pathname: "/content",
        component: lazy("src/pages/content.js", "src/pages/content.css"),
        // options: animated_route_options,
      },
      content_detail: {
        title: "内容详情",
        pathname: "/content/detail",
        component: lazy(
          "src/pages/content_detail.js",
          "src/pages/content_detail.css",
        ),
        // options: animated_route_options,
      },
      browsehistory: {
        title: "浏览记录",
        pathname: "/browsehistory",
        component: lazy(
          "src/pages/browsehistory.js",
          "src/pages/browsehistory.css",
        ),
        // options: animated_route_options,
      },
      account: {
        title: "帐号管理",
        pathname: "/account",
        component: lazy("src/pages/account.js", "src/pages/account.css"),
        // options: animated_route_options,
      },
      automation: {
        title: "自动化",
        pathname: "/automation",
        component: lazy("src/pages/automation.js", "src/pages/automation.css"),
        // options: animated_route_options,
      },
      logs: {
        title: "日志",
        pathname: "/logs",
        component: lazy("src/pages/logs.js", "src/pages/logs.css"),
        // options: animated_route_options,
      },
    },
  },
  flow_detail: {
    title: "Pipeline 详情",
    pathname: "/automation/detail",
    component: lazy("src/pages/flow_detail.js", "src/pages/automation.css"),
    // options: animated_route_options,
  },
  flow_edit: {
    title: "编辑 Pipeline",
    pathname: "/automation/edit",
    component: lazy("src/pages/flow_edit.js", "src/pages/automation.css"),
    // options: animated_route_options,
  },
};

export const router = Timeless.kit.buildRoutes(routes_configure);
export const router$ = new Timeless.kit.NavigatorCore();
export const root_view_model = new Timeless.kit.RouteViewCore({
  name: "root",
  pathname: "/",
  title: "ROOT",
  visible: true,
  parent: null,
  views: [],
});
root_view_model.isRoot = true;

export const storage$ = new Timeless.kit.StorageCore({
  key: "wx_channels_download",
  defaultValues: {
    theme: "system",
    scraper_active_job_id: "",
    third_party_downloader: {},
  },
  values: load_storage_values(window.localStorage),
  client: window.localStorage,
});
export const http_client$ = new Timeless.kit.HttpClientCore({
  headers: {
    "Content-Type": "application/json",
  },
});
export const socket_client$ = new Timeless.kit.SocketClientCore();
export const hls_player$ = new Timeless.kit.HLSPlayerCore();
export const history$ = new Timeless.kit.HistoryCore({
  view: root_view_model,
  router: router$,
  routes: router.routes,
  views: {
    root: root_view_model,
  },
});
export const app$ = new Timeless.kit.ApplicationModel({
  clipboard: Timeless.kit.ClipboardModel(),
  storage: storage$,
  async beforeReady() {
    const route = router.routesWithPathname[router$.pathname];
    const route_name = route ? route.name : router.defaultRouteName;
    history$.push(route_name, router$.query, { ignore: true });
    return Timeless.Result.Ok(null);
  },
});
app$.openWindow = function (url) {
  return window.open(url, "_blank", "noopener,noreferrer");
};

Timeless.web.provide_http_client(http_client$);
Timeless.web.provide_socket_client(socket_client$, {
  WebSocket,
});
Timeless.web.provide_hls_player(hls_player$, { Hls: window.Hls });
if (!window.dl$) {
  window.dl$ = window.DL({
    client: http_client$,
    socket_client: socket_client$,
    auto_start: false,
    logger: window.DLUtils.log,
  });
  window.scraper$ = window.ScraperModel({
    client: http_client$,
    socket_client: socket_client$,
  });
}
Timeless.web.provide_history(history$);
Timeless.web.provide_app(app$);

history$.onRouteChange(function (event) {
  if (event.view && event.view.title) {
    app$.setTitle(event.view.title);
  }
  if (event.ignore) {
    return;
  }
  if (event.reason === "push") {
    router$.pushState(String(event.href));
  }
  if (event.reason === "replace") {
    router$.replaceState(String(event.href));
  }
});

const default_platform_favicon = "public/platform-icons.svg#default";
const platform_favicons = Object.freeze({
  default: default_platform_favicon,
  wxchannels: "public/platform-icons.svg#wxchannels",
  wxmp: "public/platform-icons.svg#wxmp",
  weibo: "public/platform-icons.svg#weibo",
  officialaccount: "public/platform-icons.svg#wxmp",
  zhihu: "public/platform-icons.svg#zhihu",
  juejin: "public/platform-icons.svg?v=20260907#juejin",
  jianshu: "public/platform-icons.svg?v=20260907#jianshu",
  douyin: "public/platform-icons.svg#douyin",
  youtube: "public/platform-icons.svg#youtube",
  bilibili: "public/platform-icons.svg#bilibili",
  cctv: "public/platform-icons.svg#cctv",
  ucdrive: "public/platform-icons.svg#ucdrive",
  feishu: "public/platform-icons.svg#feishu",
  x: "public/platform-icons.svg#x",
  twitter: "public/platform-icons.svg#x",
  instagram: "public/platform-icons.svg#instagram",
  insgram: "public/platform-icons.svg#instagram",
  telegram: "public/platform-icons.svg#telegram",
  facebook: "public/platform-icons.svg#facebook",
  threads: "public/platform-icons.svg#threads",
  tiktok: "public/platform-icons.svg#tiktok",
  reddit: "public/platform-icons.svg#reddit",
  linkedin: "public/platform-icons.svg#linkedin",
  pinterest: "public/platform-icons.svg#pinterest",
  snapchat: "public/platform-icons.svg#snapchat",
  whatsapp: "public/platform-icons.svg#whatsapp",
  discord: "public/platform-icons.svg#discord",
  twitch: "public/platform-icons.svg#twitch",
  github: "public/platform-icons.svg#github",
  stackoverflow: "public/platform-icons.svg#stackoverflow",
  kuaishou: "public/platform-icons.svg#kuaishou",
  xiaohongshu: "public/platform-icons.svg#xiaohongshu",
  xhs: "public/platform-icons.svg#xiaohongshu",
  fanqienovel: "public/platform-icons.svg#fanqienovel",
  douban: "public/platform-icons.svg#douban",
  tieba: "public/platform-icons.svg#tieba",
  baidutieba: "public/platform-icons.svg#tieba",
  qidian: "public/platform-icons.svg#qidian",
});

window.PLATFORM_FAVICONS = new Proxy(platform_favicons, {
  get(target, property, receiver) {
    const favicon = Reflect.get(target, property, receiver);
    if (favicon !== undefined || typeof property !== "string" || !property) {
      return favicon;
    }
    return default_platform_favicon;
  },
});

window.PLATFORM_NAMES = Object.freeze({
  wxchannels: "视频号",
  wxmp: "公众号",
  douyin: "抖音",
  kuaishou: "快手",
  xiaohongshu: "小红书",
  instagram: "Instagram",
  youtube: "YouTube",
  bilibili: "Bilibili",
  x: "X",
  weibo: "微博",
  zhihu: "知乎",
  feishu: "飞书",
  // juejin: "掘金",
  // jianshu: "简书",
  // webpage: "网页",
  singlefile: "网页",
  // officialaccount: "公众号",
  // twitter: "X",
  // insgram: "Instagram",
  // telegram: "Telegram",
  // facebook: "Facebook",
  // threads: "Threads",
  // tiktok: "TikTok",
  // reddit: "Reddit",
  // linkedin: "LinkedIn",
  // pinterest: "Pinterest",
  // snapchat: "Snapchat",
  // whatsapp: "WhatsApp",
  // discord: "Discord",
  // twitch: "Twitch",
  // github: "GitHub",
  // stackoverflow: "Stack Overflow",
  // xhs: "小红书",
  // douban: "豆瓣",
  // tieba: "百度贴吧",
  // baidutieba: "百度贴吧",
  // qidian: "起点中文网",
  // fanqienovel: "番茄小说",
  // jianshu: "简书",
  // "69shuba": "69书吧",
  // ttk: "TT看书",
  // ucdrive: "UC网盘",
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
  text: "TXT",
  html: "HTML",
  pdf: "PDF",
  conversation: "对话",
  other: "其他",
});

const content_type_icon_base = "public/content-type-icons.svg?v=20260910-4#";
const content_type_icons = Object.freeze({
  default: `${content_type_icon_base}default`,
  video: `${content_type_icon_base}video`,
  long_video: `${content_type_icon_base}long_video`,
  episode: `${content_type_icon_base}episode`,
  series: `${content_type_icon_base}series`,
  collection: `${content_type_icon_base}collection`,
  short_video: `${content_type_icon_base}short_video`,
  image: `${content_type_icon_base}image`,
  image_set: `${content_type_icon_base}image_set`,
  album: `${content_type_icon_base}album`,
  article: `${content_type_icon_base}article`,
  answer: `${content_type_icon_base}answer`,
  question: `${content_type_icon_base}question`,
  post: `${content_type_icon_base}post`,
  blog: `${content_type_icon_base}blog`,
  webpage: `${content_type_icon_base}webpage`,
  novel: `${content_type_icon_base}novel`,
  audio: `${content_type_icon_base}audio`,
  podcast: `${content_type_icon_base}podcast`,
  music: `${content_type_icon_base}music`,
  document: `${content_type_icon_base}document`,
  course: `${content_type_icon_base}course`,
  comic: `${content_type_icon_base}comic`,
  live: `${content_type_icon_base}live`,
  text: `${content_type_icon_base}text`,
  txt: `${content_type_icon_base}text`,
  html: `${content_type_icon_base}html`,
  pdf: `${content_type_icon_base}pdf`,
  conversation: `${content_type_icon_base}conversation`,
  other: `${content_type_icon_base}other`,
});

window.CONTENT_TYPE_ICONS = new Proxy(content_type_icons, {
  get(target, property, receiver) {
    const icon = Reflect.get(target, property, receiver);
    if (icon !== undefined || typeof property !== "string" || !property) {
      return icon;
    }
    return content_type_icons.default;
  },
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

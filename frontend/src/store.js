import SiderLayoutView from "./pages/shell.js";

const storage_key = "wx_channels_download";
const legacy_scraper_job_key = "scraper.active_scraper_job_id";
const legacy_downloader_key =
  "wx_channels_download.third_party_downloader.v1";

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

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
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
      logs: {
        title: "日志",
        pathname: "/logs",
        component: lazy("src/pages/logs.js", "src/pages/logs.css"),
        // options: animated_route_options,
      },
    },
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

/**
 * @file Store 入口 - 路由管理
 */
import NotFoundPageView from "@/pages/notfound/index.js";
import HomeLayoutView from "@/pages/home/layout.js";
import HomePageView from "@/pages/home/index.js";
import HomeDownloadView from "@/pages/home/index.download.js";

const routesConfigure = {
  home_layout: {
    title: "首页",
    pathname: "/home",
    component: HomeLayoutView,
    children: {
      index: {
        default: true,
        title: "视频号",
        pathname: "/home/index",
        component: HomePageView,
      },
      download: {
        title: "下载列表",
        pathname: "/home/download",
        component: HomeDownloadView,
      },
      settings: {
        title: "设置",
        pathname: "/settings",
        component: Timeless.lazy("@/pages/settings/index.js"),
      },
    },
  },
  notfound: {
    title: "404",
    pathname: "/notfound",
    component: NotFoundPageView,
    notfound: true,
  },
};

const {
  routes,
  routesWithPathname,
  views: generatedViews,
  defaultRouteName: generatedDefaultRouteName,
  notfoundRouteName: generatedNotfoundRouteName,
} = Timeless.buildRoutes(routesConfigure);

export const views = generatedViews;
export const defaultRouteName = generatedDefaultRouteName;
export const notfoundRouteName = generatedNotfoundRouteName;

// LocalStorage
const DEFAULT_CACHE_VALUES = {
  theme: "system",
  downloadDir: "",
};
const key = "wx_channels";
const e = globalThis.localStorage.getItem(key);
export const storage = new Timeless.StorageCore({
  key,
  defaultValues: DEFAULT_CACHE_VALUES,
  values: (() => {
    const prev = JSON.parse(e || "{}");
    return {
      ...prev,
    };
  })(),
  client: globalThis.localStorage,
});

// HttpClient
export const client = new Timeless.HttpClientCore({
  headers: {
    "Content-Type": "application/json",
  },
});
Timeless.web.provide_http_client(client);

// History
Timeless.NavigatorCore.prefix = "";
export const router = new Timeless.NavigatorCore();
export const rootview = new Timeless.RouteViewCore({
  name: "root",
  pathname: "/",
  title: "ROOT",
  visible: true,
  parent: null,
  views: [],
});
rootview.isRoot = true;
export const history = new Timeless.HistoryCore({
  view: rootview,
  router,
  routes,
  views: {
    root: rootview,
  },
});
Timeless.web.provide_history(history);

export const app = new Timeless.ApplicationModel({
  storage,
  async beforeReady() {
    const { pathname, query } = router;
    const route = routesWithPathname[pathname];
    if (!route) {
      history.push(notfoundRouteName, { replace: true });
      return Timeless.Result.Err("not found");
    }
    if (!history.isLayout(route.name)) {
      history.push(route.name, query, { ignore: true });
      return Timeless.Result.Ok(null);
    }
    history.push(defaultRouteName, {}, { ignore: true });
    return Timeless.Result.Ok(null);
  },
});
Timeless.web.provide_app(app);

history.onRouteChange(({ reason, view, href, ignore }) => {
  const { title } = view || {};
  if (title) {
    app.setTitle(title);
  }
  if (ignore) {
    return;
  }
  if (reason === "push") {
    router.pushState(href);
  }
  if (reason === "replace") {
    router.replaceState(href);
  }
});
history.onClickLink(({ href, target }) => {
  const { pathname, query } = Timeless.NavigatorCore.parse(href);
  const route = routesWithPathname[pathname];
  if (!route) {
    app.tip?.({ text: ["没有匹配的页面"] });
    return;
  }
  if (target === "_blank") {
    window.open(href);
    return;
  }
  history.push(route.name, query);
});

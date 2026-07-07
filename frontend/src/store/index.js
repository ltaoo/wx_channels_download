/**
 * @file Store 入口 - 路由管理
 */
import NotFoundPageView from "@/pages/notfound/index.js";
import HomeLayoutView from "@/pages/home/layout.js";
import HomeIndexPageView from "@/pages/home/dashboard.js";
import DownloadsPageView from "@/pages/home/downloads.js";

ScrollViewPrimitive.setScrollViewProvider(Timeless.web);
InputPrimitive.setInputProvider(Timeless.web);
TextareaPrimitive.setTextareaProvider(Timeless.web);
Timeless.NavigatorCore.prefix = "/";

export const routes_configure = /** @type {const} */ ({
  home_layout: {
    title: "首页",
    pathname: "/home",
    component: HomeLayoutView,
    children: {
      index: {
        is_default: true,
        title: "首页",
        pathname: "/home/index",
        component: HomeIndexPageView,
      },
      download: {
        title: "下载列表",
        pathname: "/home/downloads",
        component: DownloadsPageView,
      },
      accounts: {
        title: "帐号",
        pathname: "/accounts",
        component: Timeless.lazy("@/pages/home/accounts.js"),
      },
      content: {
        title: "内容列表",
        pathname: "/content",
        component: Timeless.lazy("@/pages/home/content.js"),
      },
      browse: {
        title: "浏览记录",
        pathname: "/browse",
        component: Timeless.lazy("@/pages/home/browse.js"),
      },
      settings: {
        title: "设置",
        pathname: "/settings",
        component: Timeless.lazy("@/pages/settings/index.js"),
      },
      sandboxes: {
        title: "沙箱",
        pathname: "/sandboxes",
        component: Timeless.lazy("@/pages/sandboxes/index.js"),
      },
      tools: {
        title: "工具箱",
        pathname: "/tools",
        component: Timeless.lazy("@/pages/tools/index.js"),
      },
      proxy: {
        title: "代理配置",
        pathname: "/proxy",
        component: Timeless.lazy("@/pages/proxy/index.js"),
      },
      logs: {
        title: "日志",
        pathname: "/logs",
        component: Timeless.lazy("@/pages/logs/index.js"),
      },
    },
  },
  login: {
    title: "登录",
    pathname: "/login",
    component: Timeless.lazy("@/pages/login/index.js"),
  },
  notfound: {
    title: "404",
    pathname: "/notfound",
    component: NotFoundPageView,
    notfound: true,
  },
});

const router = Timeless.buildRoutes(routes_configure);

const routes = router.routes;
export const views = router.views;
export const defaultRouteName = router.defaultRouteName;
export const notfoundRouteName = router.notfoundRouteName;

function routeHasRequirement(route, requireKey) {
  let cur = route;
  while (cur) {
    const requires = cur.options?.require;
    if (Array.isArray(requires) && requires.includes(requireKey)) {
      return true;
    }
    const parentName = cur.parent?.name;
    if (!parentName) {
      return false;
    }
    cur = routes[parentName];
  }
  return false;
}

// LocalStorage
const DEFAULT_CACHE_VALUES = {
  user: {
    id: "",
    username: "anonymous",
    email: "",
    token: "",
    avatar: "",
  },
  theme: "system",
};
const key = "timeless";
const e = globalThis.localStorage.getItem(key);
export const storage$ = new Timeless.StorageCore({
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
export const client$ = new Timeless.HttpClientCore({
  headers: {
    "Content-Type": "application/json",
  },
});
Timeless.web.provide_http_client(client$);
export const api_client$ = new Timeless.HttpClientCore({
  hostname: "http://127.0.0.1:2022",
  // hostname: 'http://100.78.198.69:2022',
  // hostname: 'http://192.168.1.118:2022',
  headers: {
    "Content-Type": "application/json",
  },
});
Timeless.web.provide_http_client(api_client$);
export const user$ = (() => {
  let profile = storage$.get("user");
  const loginListeners = [];
  const logoutListeners = [];

  storage$.onStateChange(() => {
    profile = storage$.get("user");
  });

  function removeListener(list, cb) {
    const idx = list.indexOf(cb);
    if (idx >= 0) list.splice(idx, 1);
  }

  return {
    get profile() {
      return profile;
    },
    get token() {
      return profile?.token || "";
    },
    get isLogin() {
      return !!(profile && profile.token);
    },
    login(nextProfile) {
      const merged = {
        ...profile,
        ...(nextProfile || {}),
      };
      profile = merged;
      storage$.set("user", merged);
      client$.appendHeaders({ Authorization: merged.token || "" });
      for (const cb of loginListeners) cb(merged);
    },
    logout() {
      storage$.clear("user");
      profile = storage$.get("user");
      client$.appendHeaders({ Authorization: "" });
      for (const cb of logoutListeners) cb();
    },
    onLogin(cb) {
      loginListeners.push(cb);
      return () => removeListener(loginListeners, cb);
    },
    onLogout(cb) {
      logoutListeners.push(cb);
      return () => removeListener(logoutListeners, cb);
    },
  };
})();
client$.appendHeaders({ Authorization: user$.token });
// client$.fetch = async (options) => {
//   const { url, method, id, data, headers } = options;
//   try {
//     const r = await invoke(url as string, data as any);
//     return Promise.resolve({ data: r });
//   } catch (err) {
//     throw err;
//   }
// };
export const router$ = new Timeless.NavigatorCore();
export const view$ = new Timeless.RouteViewCore({
  name: "root",
  pathname: "/",
  title: "ROOT",
  visible: true,
  parent: null,
  views: [],
});
view$.isRoot = true;
export const history$ = new Timeless.HistoryCore({
  view: view$,
  router: router$,
  routes,
  views: {
    root: view$,
  },
});
Timeless.web.provide_history(history$);

const clipboard = Timeless.ClipboardModel();
export const app = new Timeless.ApplicationModel({
  clipboard,
  storage: storage$,
  async beforeReady() {
    const { pathname, query } = router$;
    const route = router.routesWithPathname[pathname];
    console.log(
      "[Store] beforeReady",
      pathname,
      route,
      router.routesWithPathname,
    );
    if (!route) {
      history$.push("root.home_layout", {}, { ignore: true });
      return Timeless.Result.Ok(null);
    }
    if (routeHasRequirement(route, "login") && !user$.isLogin) {
      history$.push("root.login", {
        redirect: route.name,
        redirect_query: encodeURIComponent(JSON.stringify(query || {})),
      });
      return Timeless.Result.Err("need login");
    }
    history$.push(route.name, query, { ignore: true });
    return Timeless.Result.Ok(null);
  },
});
Timeless.web.provide_app(app);

history$.onRouteChange(({ reason, view, href, ignore }) => {
  if (!ignore) {
    const pathname = String(view?.pathname || "");
    const route = router.routesWithPathname[pathname];
    if (route && routeHasRequirement(route, "login") && !user$.isLogin) {
      history$.replace("root.login", {
        redirect: route.name,
        redirect_query: encodeURIComponent(JSON.stringify(view?.query || {})),
      });
      return;
    }
  }
  const { title } = view || {};
  if (title) {
    app.setTitle(title);
  }
  if (ignore) {
    return;
  }
  if (reason === "push") {
    router$.pushState(String(href));
  }
  if (reason === "replace") {
    router$.replaceState(String(href));
  }
});
history$.onClickLink(({ href, target }) => {
  const hrefText = String(href || "");
  const { pathname, query } = Timeless.NavigatorCore.parse(hrefText);
  const route = router.routesWithPathname[pathname];
  if (!route) {
    app.tip?.({ text: ["没有匹配的页面"] });
    return;
  }
  if (target === "_blank") {
    window.open(hrefText);
    return;
  }
  history$.push(/** @type {PageKey} */ (route.name), query);
});

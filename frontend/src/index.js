/** @type {any} */
var Timeless;

/** @type {(page_name: string, component: Function) => void} */
var register;

window.h = Timeless.h;
window.View = Timeless.View;
window.Button = Timeless.Button;
window.Fragment = Timeless.Fragment;
window.Input = Timeless.Input;
window.Img = Timeless.Img;
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

(function (global) {
  "use strict";

  var components = Object.create(null);
  var css_records = Object.create(null);
  var load_records = Object.create(null);

  /**
   *
   * @param {string} page_name
   * @param {Function} component
   */
  register = function (page_name, component) {
    if (!page_name || typeof component !== "function") {
      throw new TypeError("页面注册需要名称和 View 工厂函数");
    }

    components[page_name] = component;

    var record = load_records[page_name];
    if (record && record.status === "loading") {
      record.status = "loaded";
      record.resolve(component);
    }
  };
  global.register = register;

  /**
   *
   * @param {string} page_name
   * @param {string | undefined} css_url
   * @returns {Promise<HTMLLinkElement | null>}
   */
  function load_style(page_name, css_url) {
    if (!css_url) {
      return Promise.resolve(null);
    }

    var current_record = css_records[css_url];
    if (current_record && current_record.status === "loaded") {
      return Promise.resolve(current_record.link);
    }
    if (current_record && current_record.status === "loading") {
      return current_record.promise;
    }

    /** @type {undefined | Function} */
    var resolve_load;
    /** @type {undefined | Function} */
    var reject_load;
    var promise = new Promise(function (resolve, reject) {
      resolve_load = resolve;
      reject_load = reject;
    });
    var link = document.createElement("link");
    var record = {
      link: link,
      promise: promise,
      status: "loading",
      url: css_url,
    };

    css_records[css_url] = record;
    link.rel = "stylesheet";
    link.href = css_url;
    link.dataset.page = page_name;
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
   *
   * @param {string} page_name
   * @param {string} script_url
   * @param {string | undefined} css_url
   * @returns
   */
  function load(page_name, script_url, css_url) {
    if (components[page_name]) {
      return load_style(page_name, css_url).then(function () {
        return components[page_name];
      });
    }

    var current_record = load_records[page_name];
    if (current_record && current_record.status === "loading") {
      return current_record.promise;
    }

    /** @type {undefined | Function} */
    var resolve_load;
    /** @type {undefined | Function} */
    var reject_load;
    var promise = new Promise(function (resolve, reject) {
      resolve_load = resolve;
      reject_load = reject;
    });
    var script = document.createElement("script");
    var record = {
      promise: promise,
      reject: reject_load,
      resolve: resolve_load,
      script: script,
      status: "loading",
      css_url: css_url,
      url: script_url,
    };

    load_records[page_name] = record;
    script.async = true;
    script.src = script_url;
    script.dataset.page = page_name;
    script.onload = function () {
      var component = components[page_name];
      if (component) {
        record.status = "loaded";
        resolve_load(component);
        return;
      }

      record.status = "failed";
      reject_load(new Error("页面脚本已加载，但未注册页面：" + page_name));
    };
    script.onerror = function () {
      record.status = "failed";
      reject_load(new Error("页面脚本加载失败：" + script_url));
    };
    load_style(page_name, css_url)
      .then(function () {
        document.head.appendChild(script);
      })
      .catch(function (error) {
        record.status = "failed";
        reject_load(error);
      });

    return promise;
  }

  /**
   *
   * @param {string} page_name
   * @param {string} script_url
   * @param {string | undefined} css_url
   * @returns
   */
  function lazy(page_name, script_url, css_url) {
    return function () {
      return load(page_name, script_url, css_url);
    };
  }

  var routes_configure = {
    preview: {
      title: "预览",
      pathname: "/preview",
      component: lazy(
        "preview_page",
        "src/pages/preview.js",
        "src/pages/preview.css",
      ),
    },
    shell: {
      title: "路由示例",
      pathname: "/",
      component: SiderLayoutView,
      children: {
        download: {
          title: "下载",
          pathname: "/",
          component: lazy("download_page", "src/pages/download.js"),
        },
        scraper: {
          is_default: true,
          title: "内容抓取",
          pathname: "/scraper",
          component: lazy(
            "scraper_page",
            "src/pages/scraper.js",
            "src/pages/scraper.css",
          ),
        },
        content: {
          title: "内容管理",
          pathname: "/content",
          component: lazy("content_page", "src/pages/content.js"),
        },
        browsehistory: {
          title: "浏览记录",
          pathname: "/browsehistory",
          component: lazy("browsehistory_page", "src/pages/browsehistory.js"),
        },
        account: {
          title: "帐号管理",
          pathname: "/account",
          component: lazy(
            "account_page",
            "src/pages/account.js",
            "src/pages/account.css",
          ),
        },
        logs: {
          title: "日志",
          pathname: "/logs",
          component: lazy("logs_page", "src/pages/logs.js"),
        },
      },
    },
  };
  var router = Timeless.kit.buildRoutes(routes_configure);
  var router$ = new Timeless.kit.NavigatorCore();
  var root_view_model = new Timeless.kit.RouteViewCore({
    name: "root",
    pathname: "/",
    title: "ROOT",
    visible: true,
    parent: null,
    views: [],
  });
  root_view_model.isRoot = true;
  var storage$ = new Timeless.kit.StorageCore({
    key: "wx_channels_download",
    defaultValues: {
      theme: "system",
    },
    values: {},
    client: global.localStorage,
  });
  var http_client$ = new Timeless.kit.HttpClientCore({
    headers: {
      "Content-Type": "application/json",
    },
  });
  var history$ = new Timeless.kit.HistoryCore({
    view: root_view_model,
    router: router$,
    routes: router.routes,
    views: {
      root: root_view_model,
    },
  });
  var app$ = new Timeless.kit.ApplicationModel({
    clipboard: Timeless.kit.ClipboardModel(),
    storage: storage$,
    async beforeReady() {
      var route = router.routesWithPathname[router$.pathname];
      var route_name = route ? route.name : router.defaultRouteName;
      history$.push(route_name, router$.query, { ignore: true });
      return Result.Ok(null);
    },
  });

  Timeless.web.provide_http_client(http_client$);
  Timeless.web.provide_history(history$);
  Timeless.web.provide_app(app$);

  history$.onRouteChange(function (/** @type {any} */ event) {
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

  function LoadingView() {
    return View({ class: "route-loading", role: "status" }, ["页面加载中…"]);
  }

  /**
   *
   * @param {Error} error
   * @param {string} view_name
   * @returns
   */
  function ErrorFallbackView(error, view_name) {
    return View({ class: "route-error" }, [
      View({ as: "strong" }, ["页面加载失败"]),
      View({ as: "span" }, [view_name || "未知页面"]),
      View({ as: "pre" }, [error.message]),
    ]);
  }

  function SiderLayoutView(props) {
    var menu$ = Timeless.kit.RouteMenusModel({
      view: props.view,
      history: history$,
      menus: [
        { title: "下载", name: "root.shell.download" },
        { title: "内容抓取", name: "root.shell.scraper" },
        { title: "内容管理", name: "root.shell.content" },
        { title: "浏览记录", name: "root.shell.browsehistory" },
        { title: "帐号管理", name: "root.shell.account" },
        { title: "日志", name: "root.shell.logs" },
      ],
    });

    return View(
      {
        class: "app-shell",
        onUnmounted: function () {
          menu$.destroy();
        },
      },
      [
        View({ as: "aside", class: "app-sider" }, [
          View({ class: "app-brand" }, [
            View({ class: "app-brand__mark" }, ["W"]),
            View({}, [
              View({ class: "app-brand__title" }, ["内容工作台"]),
              View({ class: "app-brand__subtitle" }, ["解析 · 下载 · 归档"]),
            ]),
          ]),
          View({ as: "nav", class: "app-menu", ariaLabel: "页面导航" }, [
            For({
              each: menu$.menus,
              render: function (menu) {
                return Button(
                  {
                    class: Timeless.classNames([
                      "app-menu__item",
                      Timeless.computed(menu$.cur, function (current) {
                        return menu$.isSelected(current, menu)
                          ? "app-menu__item--active"
                          : null;
                      }),
                    ]),
                    onClick: function () {
                      menu$.handleClick(menu);
                    },
                  },
                  [View({ as: "span" }, [menu.title])],
                );
              },
            }),
          ]),
        ]),
        View({ as: "main", class: "app-content" }, [
          Timeless.ui.StandardSubViews({
            app: props.app,
            client: props.client,
            ErrorFallback: ErrorFallbackView,
            history: props.history,
            placeholder: [LoadingView()],
            storage: props.storage,
            view: props.view,
            views: props.views,
          }),
        ]),
      ],
    );
  }

  function ApplicationRootView() {
    return View({ class: "application-root" }, [
      Timeless.ui.StandardSubViews({
        app: app$,
        client: http_client$,
        history: history$,
        storage: storage$,
        view: history$.$view,
        views: router.views,
        placeholder: [LoadingView()],
        ErrorFallback: ErrorFallbackView,
      }),
    ]);
  }

  function bootstrap() {
    var $root = document.querySelector("#root");
    if (!$root) {
      throw new Error("应用无法启动：缺少 App Model 或根节点");
    }
    router$.prepare(global.location);
    app$.start({
      width: global.innerWidth,
      height: global.innerHeight,
    });
    Timeless.DOM.render(ApplicationRootView(), $root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
  } else {
    bootstrap();
  }
})(window);

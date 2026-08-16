import { Button, createButtonStore } from "./components.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

window.h = Timeless.h;
window.View = Timeless.View;
window.Fragment = Timeless.Fragment;
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

window.PLATFORM_FAVICONS = Object.freeze({
  wxchannels:
    "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
  wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
  douyin: "https://p-pc-weboff.byteimg.com/tos-cn-i-9r5gewecjs/favicon.png",
  youtube: "https://www.youtube.com/s/desktop/0084d708/img/favicon.ico",
  bilibili: "https://static.hdslb.com/images/favicon.ico",
});

(function (global) {
  "use strict";

  var css_records = Object.create(null);
  var load_records = Object.create(null);

  /**
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {Promise<HTMLLinkElement | null>}
   */
  function load_style(module_url, css_url) {
    if (!css_url) {
      return Promise.resolve(null);
    }

    var style_href = new URL(css_url, document.baseURI).href;
    var current_record = css_records[style_href];
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
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {Promise<Function>}
   */
  function load(module_url, css_url) {
    var module_href = new URL(module_url, document.baseURI).href;
    var current_record = load_records[module_href];
    if (current_record && current_record.status === "loaded") {
      return load_style(module_url, css_url).then(function () {
        return current_record.component;
      });
    }
    if (current_record && current_record.status === "loading") {
      return current_record.promise;
    }

    var record = {
      component: null,
      promise: null,
      status: "loading",
      css_url: css_url,
      url: module_href,
    };

    load_records[module_href] = record;
    record.promise = Promise.all([
      load_style(module_url, css_url),
      import(module_href),
    ])
      .then(function (results) {
        var page_module = results[1];
        var component = page_module && page_module.default;
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

  /**
   *
   * @param {string} module_url
   * @param {string | undefined} css_url
   * @returns {() => Promise<Function>}
   */
  function lazy(module_url, css_url) {
    return function () {
      return load(module_url, css_url);
    };
  }

  var routes_configure = {
    filehelper: {
      title: "微信文件传输助手",
      pathname: "/filehelper",
      component: lazy("src/pages/filehelper.js", "src/pages/filehelper.css"),
    },
    preview: {
      title: "预览",
      pathname: "/preview",
      component: lazy("src/pages/preview.js", "src/pages/preview.css"),
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
          component: lazy("src/pages/downloadv2.js"),
        },
        scraper: {
          title: "内容抓取",
          pathname: "/scraper",
          component: lazy("src/pages/scraper.js", "src/pages/scraper.css"),
        },
        content: {
          title: "内容管理",
          pathname: "/content",
          component: lazy("src/pages/content.js"),
        },
        content_detail: {
          title: "内容详情",
          pathname: "/content/detail",
          component: lazy(
            "src/pages/content_detail.js",
            "src/pages/content_detail.css",
          ),
        },
        browsehistory: {
          title: "浏览记录",
          pathname: "/browsehistory",
          component: lazy("src/pages/browsehistory.js"),
        },
        account: {
          title: "帐号管理",
          pathname: "/account",
          component: lazy("src/pages/account.js", "src/pages/account.css"),
        },
        // logs: {
        //   title: "日志",
        //   pathname: "/logs",
        //   component: lazy("src/pages/logs.js"),
        // },
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
  const socket_client$ = new Timeless.kit.SocketClientCore();
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
      return Timeless.Result.Ok(null);
    },
  });

  Timeless.web.provide_http_client(http_client$);
  Timeless.web.provide_socket_client(socket_client$, {
    WebSocket: WebSocket,
    // debug: debug === true,
  });
  if (!global.dl$) {
    global.dl$ = DL({
      client: http_client$,
      socket_client: socket_client$,
    });
  }
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
    return View(
      {
        class:
          "route-loading dm-page dm-grid dm-place-center dm-text-muted dm-p-8",
        role: "status",
      },
      ["页面加载中…"],
    );
  }

  /**
   *
   * @param {Error} error
   * @param {string} view_name
   * @returns
   */
  function ErrorFallbackView(error, view_name) {
    return View({ class: "route-error dm-page dm-grid dm-place-center dm-p-8" }, [
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
        { title: "下载", name: "root.shell.download", icon: "download" },
        { title: "Get", name: "root.shell.scraper", icon: "search" },
        { title: "内容管理", name: "root.shell.content", icon: "library" },
        { title: "浏览记录", name: "root.shell.browsehistory", icon: "history" },
        { title: "帐号管理", name: "root.shell.account", icon: "user" },
        // { title: "日志", name: "root.shell.logs", icon: "scroll-text" },
      ],
    });

    return View(
      {
        class: "app-shell dm-page",
        onUnmounted: function () {
          menu$.destroy();
        },
      },
      [
        View({ as: "aside", class: "app-sider dm-flex dm-flex-col" }, [
          View({ class: "app-brand" }, [
            Img({
              class: "app-brand__logo",
              src: "public/logo.png?v=logo-only-v4",
              alt: "D&M",
              attributes: {
                draggable: "false",
              },
            }),
          ]),
          View(
            {
              as: "nav",
              class: "app-menu dm-flex dm-flex-col dm-gap-1",
              ariaLabel: "页面导航",
            },
            [
            For({
              each: menu$.menus,
              render: function (menu) {
                return Button(
                  {
                    store: createButtonStore({
                      variant: "ghost",
                      onClick() {
                        menu$.handleClick(menu);
                      },
                    }),
                    class: Timeless.classNames([
                      "app-menu__item",
                      "dm-focus-ring",
                      "dm-justify-start",
                      Timeless.computed(menu$.cur, function (current) {
                        return menu$.isSelected(current, menu)
                          ? "app-menu__item--active"
                          : null;
                      }),
                    ]),
                  },
                  [
                    View({ class: "app-menu__icon" }, [
                      Timeless.Icon({ name: menu.icon, size: 17 }),
                    ]),
                    View({ as: "span", class: "app-menu__label" }, [
                      menu.title,
                    ]),
                  ],
                );
              },
            }),
            ],
          ),
          // View({ class: "app-sider__footer" }, [
          //   View({ class: "app-sider__footer-icon" }, [
          //     Timeless.Icon({ name: "hard-drive", size: 16 }),
          //   ]),
          //   View({ class: "app-sider__footer-copy" }, [
          //     View({ class: "app-sider__footer-title" }, ["本地工作区"]),
          //     View({ class: "app-sider__footer-text" }, [
          //       "内容与任务保存在本机",
          //     ]),
          //   ]),
          // ]),
        ]),
        View(
          {
            as: "main",
            class: "app-content dm-min-w-0 dm-overflow-auto",
          },
          [
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
          ],
        ),
      ],
    );
  }

  function ApplicationRootView() {
    return View({ class: "application-root dm-page" }, [
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

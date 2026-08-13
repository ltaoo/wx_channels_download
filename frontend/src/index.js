/** @type {any} */
var Timeless;

/** @type {(page_name: string, component: Function) => void} */
var register;

(function (global) {
  "use strict";

  var View = Timeless.View;
  var Button = Timeless.Button;
  var For = Timeless.For;
  var Result = Timeless.Result;

  var components = Object.create(null);
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
   * @param {string} script_url
   * @returns
   */
  function load(page_name, script_url) {
    if (components[page_name]) {
      return Promise.resolve(components[page_name]);
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
    document.head.appendChild(script);

    return promise;
  }

  /**
   *
   * @param {string} page_name
   * @param {string} script_url
   * @returns
   */
  function lazy(page_name, script_url) {
    return function () {
      return load(page_name, script_url);
    };
  }

  var routes_configure = {
    shell: {
      title: "路由示例",
      pathname: "/",
      component: SiderLayoutView,
      children: {
        a: {
          is_default: true,
          title: "页面 A",
          pathname: "/a",
          component: lazy("a", "src/pages/a.js"),
        },
        b: {
          title: "页面 B",
          pathname: "/b",
          component: lazy("b", "src/pages/b.js"),
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
        { title: "页面 A", name: "root.shell.a", label: "A" },
        { title: "页面 B", name: "root.shell.b", label: "B" },
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
              View({ class: "app-brand__title" }, ["路由示例"]),
              View({ class: "app-brand__subtitle" }, ["Script lazy load"]),
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
                  [
                    View({ as: "span", class: "app-menu__icon" }, [menu.label]),
                    View({ as: "span" }, [menu.title]),
                  ],
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

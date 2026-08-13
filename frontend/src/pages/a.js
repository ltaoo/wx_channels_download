(function (global) {
  "use strict";

  var View = Timeless.View;

  function PageAView() {
    return View({ class: "demo-page demo-page--a" }, [
      View({ class: "demo-page__eyebrow" }, ["PAGE A"]),
      View({ as: "h1", class: "demo-page__title" }, ["这是页面 A"]),
      View({ as: "p", class: "demo-page__description" }, [
        "页面 A 的脚本只会在第一次进入该路由时加载。",
      ]),
      View({ class: "demo-page__card" }, [
        View({ as: "strong" }, ["共享布局"]),
        View({ as: "span" }, ["切换到页面 B 时，左侧 Sider Menu 会继续保留。"]),
      ]),
    ]);
  }

  global.register("a", PageAView);
})(window);

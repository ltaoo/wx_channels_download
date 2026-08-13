(function (global) {
  "use strict";

  var View = Timeless.View;

  function PageBView() {
    return View({ class: "demo-page demo-page--b" }, [
      View({ class: "demo-page__eyebrow" }, ["PAGE B"]),
      View({ as: "h1", class: "demo-page__title" }, ["这是页面 B"]),
      View({ as: "p", class: "demo-page__description" }, [
        "页面 B 通过动态插入 script 标签完成懒加载。",
      ]),
      View({ class: "demo-page__card" }, [
        View({ as: "strong" }, ["无 ES Module"]),
        View({ as: "span" }, [
          "所有模块通过 window 命名空间协作，没有使用 import 或 export。",
        ]),
      ]),
    ]);
  }

  global.register("b", PageBView);
})(window);

import * as components from "./components.js";
import {
  app$,
  history$,
  http_client$,
  router,
  router$,
  storage$,
} from "./store.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

Object.assign(window, components);

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

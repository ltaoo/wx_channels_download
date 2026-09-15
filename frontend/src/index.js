import * as components from "./components/index.js";
import * as store from "./store.js";

const { app$, history$, hls_player$, http_client$, router, router$, storage$ } =
  store;

window.config = window.__d_config || {};
window.__store = store;

Object.assign(window, components);

window.API_ORIGIN = window.config.apiOrigin || window.location.origin;

function ApplicationRootView() {
  return View({ class: "application-root page" }, [
    Timeless.ui.KeepAliveSubViews({
      app: app$,
      client: http_client$,
      history: history$,
      hlsPlayer: hls_player$,
      storage: storage$,
      view: history$.$view,
      views: router.views,
      placeholder: LoadingView,
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

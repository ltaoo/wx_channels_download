import * as components from "./components/index.js";
import * as store from "./store.js";
import { SonnerModel, Toaster } from "./dmui.js";

const { app$, history$, hls_player$, http_client$, router, router$, storage$ } =
  store;

window.config = window.__d_config || {};
window.__store = store;

Object.assign(window, components);

window.API_ORIGIN = window.config.apiOrigin || window.location.origin;

function ApplicationRootView(props) {
  const sonner = SonnerModel();
  const tip = ({ msg, type }) => props.app.tip({ text: msg, type });
  const unlisten_tip = props.app.onTip(({ text, type }) => {
    const message = text.join("\n");
    const method = type === "warning" ? "warn" : type;
    if (["error", "warn", "success", "info"].includes(method)) sonner[method](message);
    else sonner.message(message);
  });

  return View({
    class: "relative w-full h-full min-w-0 min-h-full text-color-text-primary font-sans bg-color-bg-canvas",
    attributes: { n: "application-root" },
    onUnmounted() {
      unlisten_tip();
      sonner.clear();
    },
  }, [
    Toaster({ store: sonner }),
    Timeless.ui.KeepAliveSubViews({
      app: props.app,
      tip,
      client: http_client$,
      history: history$,
      hlsPlayer: hls_player$,
      storage: storage$,
      view: history$.$view,
      views: router.views,
      placeholder: LoadingView,
      ErrorFallback: (error, name) => ErrorFallbackView(error, name, tip),
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
  Timeless.DOM.render(ApplicationRootView({ app: app$ }), $root);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
} else {
  bootstrap();
}

import MigrationPageView from "./migration.js";

const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

function bootstrap() {
  const root = document.querySelector("#root");
  if (!root) {
    throw new Error("应用无法启动：缺少根节点");
  }
  Timeless.DOM.render(MigrationPageView(), root);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
} else {
  bootstrap();
}

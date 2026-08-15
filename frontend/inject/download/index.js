/// <reference path="../utils.js" />
/// <reference path="../file.js" />
/// <reference path="model.js" />
/// <reference path="view.js" />
/**
 * @file Download manager page entry
 */


(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    document.body.classList.add("wx-dl-page-body");
    document.body.dataset.weuiTheme =
      document.documentElement.classList.contains("dark") ? "dark" : "light";
    const vm$ = DownloaderPanelViewModel({
      enableDropdownMenu: false,
      fixedListHeight: false,
      initial_status: "all",
      itemHeight: 72,
      listGutter: 0,
      sort_by_status: false,
      syncListContentHeight: false,
    });
    Timeless.DOM.render(DownloaderPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

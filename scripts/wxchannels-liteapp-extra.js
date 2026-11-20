(function () {
  if (location.origin !== "https://liteapp.weixin.qq.com" ||
      !location.pathname.startsWith("/a/finder/wxalite28e098851253e34f4777f7ca024236fd/") ||
      globalThis.__wxchannels_mitm_extra__) return;

  const model = {
    marker: "WXMITM_LITEAPP_H",
    attempts: 0,
    app: null,
    errors: [],
    report(stage) {
      const root = document.getElementById("app");
      const query = new URLSearchParams({
        marker: this.marker,
        stage,
        page: location.pathname,
        app_preserved: String(!!this.app && root?.__vue_app__ === this.app)
      });
      fetch("https://mmwxalite.wxqcloud.qq.com/liteapp/__local_js_probe__/loaded?" + query,
        {mode: "no-cors", cache: "no-store"}).catch(error => this.errors.push(String(error)));
    },
    start() {
      const root = document.getElementById("app");
      if (!root?.__vue_app__ || !document.body) {
        if (++this.attempts < 300) setTimeout(() => this.start(), 100);
        else this.report("mount_timeout");
        return;
      }
      this.app = root.__vue_app__;
      view.render(this.marker);
      this.report("badge_added");
    }
  };
  const view = {
    render(marker) {
      const badge = document.querySelector('[data-n="wxchannels-mitm-extra-marker"]') || document.createElement("div");
      badge.setAttribute("data-n", "wxchannels-mitm-extra-marker");
      badge.textContent = marker;
      badge.style.cssText = "position:fixed;top:8px;left:8px;z-index:2147483647;background:#ffeb00;color:#000;padding:8px;font:14px monospace;pointer-events:none";
      document.body.appendChild(badge);
    }
  };
  globalThis.__wxchannels_mitm_extra__ = model;
  model.report("executed");
  model.start();
})();

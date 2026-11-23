(function () {
  if (
    typeof window === "undefined" ||
    typeof document === "undefined" ||
    location.origin !== "https://liteapp.weixin.qq.com" ||
    !location.pathname.startsWith(
      "/a/finder/wxalite28e098851253e34f4777f7ca024236fd/",
    ) ||
    window.__wxchannels_liteapp_rewrite__
  ) {
    return;
  }

  const base = "https://res.wx.qq.com/__assets/wxchannels/liteapp/";
  const model = {
    stage: "waiting",
    revision: "direct-wrap-v2-navigation",
    methods: 0,
    init_events: 0,
    detail_events: 0,
    app: null,
    require: null,
    wrapped: new WeakSet(),
    navigation_unsubscribe: null,
    error: "",
    async report(stage) {
      this.stage = stage;
      const root = document.querySelector("#app");
      try {
        await fetch(base + "status", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            stage,
            revision: this.revision,
            methods: this.methods,
            init_events: this.init_events,
            detail_events: this.detail_events,
            app_preserved: !!this.app && root?.__vue_app__ === this.app,
            error: this.error,
          }),
        });
      } catch (error) {
        console.warn("[wxchannels LiteApp status]", error);
      }
    },
    capture() {
      const chunks = window.webpackChunk;
      if (!chunks || !document.querySelector("#app")?.__vue_app__) {
        return false;
      }
      chunks.push([
        ["wxchannels_mitm_" + Date.now()],
        {},
        (require) => {
          this.require = require;
        },
      ]);
      return !!this.require?.m;
    },
    find_api_module() {
      for (const [id, factory] of Object.entries(this.require.m)) {
        const source = factory.toString();
        if (
          !source.includes("async finderGetCommentDetail(") ||
          !source.includes("async finderInit(") ||
          source.includes("new class")
        ) {
          continue;
        }
        const exports = this.require(id);
        if (
          Object.values(exports).some(
            (value) =>
              typeof value === "function" &&
              value.prototype?.finderGetCommentDetail,
          )
        ) {
          return { id, exports };
        }
      }
      throw new Error("no supported LiteApp API class module found");
    },
    install(exports) {
      // Keep the original functions and their closures. Only observe successful results.
      const handle_data = (result) => result.data;
      const handle_object = (result) => result.data.object;
      const events = {
        finderInit: ["channels:Init", handle_data],
        finderGetCommentDetail: ["channels:OnFeedProfileLoaded", handle_object],
        finderGetCommentList: ["channels:FeedCommentListLoaded", handle_data],
        finderPcFlow: ["channels:PCFlowLoaded", handle_object],
        finderGetRecommend: ["channels:RecommendFeedsLoaded", handle_object],
        getRecommendTabsFromService: [
          "channels:RecommendFeedsLoaded",
          (result) => {
            const response = result.resp || result;
            return response.data ? response.data.object : [];
          },
        ],
        finderGetInteractionedFeedList: [
          "channels:InteractionedFeedsLoaded",
          handle_object,
        ],
        finderGetFeedH5Url: ["channels:GetFeedH5Url", handle_object],
        finderUserPage: ["channels:UserFeedsLoaded", handle_object],
        finderLiveUserPage: ["channels:LiveUserFeedsLoaded", handle_object],
        finderGetLiveInfo: ["channels:OnLiveProfileLoaded", handle_data],
        joinLive: ["channels:JoinLive", handle_data],
      };
      const changes = [];
      const targets = new Set();
      const installed = new Set();
      for (const value of Object.values(exports)) {
        if (
          typeof value !== "function" ||
          !value.prototype?.finderGetCommentDetail
        ) {
          continue;
        }
        const target = value.prototype;
        if (targets.has(target)) {
          continue;
        }
        targets.add(target);
        for (const [name, [event, extract]] of Object.entries(events)) {
          const previous = Object.getOwnPropertyDescriptor(target, name);
          if (!previous) {
            continue;
          }
          if (!previous.configurable || typeof previous.value !== "function") {
            throw new Error("API method cannot be replaced: " + name);
          }
          installed.add(name);
          if (this.wrapped.has(previous.value)) {
            continue;
          }
          const original = previous.value;
          const descriptor = {
            ...previous,
            async value(...args) {
              const result = await Reflect.apply(original, this, args);
              WXU.emit(event, extract(result));
              return result;
            },
          };
          changes.push({ target, name, descriptor, previous });
        }
      }
      if (
        !installed.has("finderGetCommentDetail") ||
        !installed.has("finderInit")
      ) {
        throw new Error("required API wrappers missing");
      }
      const applied = [];
      try {
        for (const change of changes) {
          Object.defineProperty(change.target, change.name, change.descriptor);
          applied.push(change);
        }
      } catch (error) {
        for (const change of applied.reverse()) {
          Object.defineProperty(change.target, change.name, change.previous);
        }
        throw error;
      }
      for (const change of changes) {
        this.wrapped.add(change.descriptor.value);
      }
      this.methods += changes.length;
    },
    install_navigation() {
      const store = this.app.config.globalProperties.$pinia?._s?.get("home");
      if (!store?.$onAction || this.navigation_unsubscribe) {
        return;
      }
      const events = {
        goToPrevFlowFeed: "channels:GotoPrevFeed",
        goToNextFlowFeed: "channels:GotoNextFeed",
        loadLocalPlaylist: "channels:HomeFeedChanged",
      };
      // Pinia subscriptions also observe actions already captured by component setup.
      this.navigation_unsubscribe = store.$onAction(({ name, after }) => {
        const event = events[name];
        if (!event) {
          return;
        }
        const previous_id = store.flowCurrentFeed?.id;
        after(() => {
          const feed = store.flowCurrentFeed;
          if (!feed?.id || feed.id === "0" || feed.id === previous_id) {
            return;
          }
          WXU.emit(event, feed);
        });
      }, true);
    },
    services() {
      const match = Object.entries(this.require.m).find(([, factory]) => {
        const source = factory.toString();
        return (
          source.includes("async finderPcFlow(") &&
          source.includes("getRecommendTabsFromService(")
        );
      });
      if (!match) {
        throw new Error("LiteApp API service exports unavailable");
      }
      return this.require(match[0]);
    },
    async replay(services) {
      const api = Object.values(services).find(
        (value) => typeof value?.finderGetCommentDetail === "function",
      );
      if (!api) {
        throw new Error("LiteApp detail service unavailable");
      }
      // The SDK is asynchronous: the original page may have finished its first request already.
      await api.finderInit();
      const properties = this.app.config.globalProperties;
      const route =
        properties.$route || properties.$router?.currentRoute?.value;
      const query =
        route?.query ||
        Object.fromEntries(new URLSearchParams(location.search));
      if (query.eid) {
        const [error] = await window.fetchFeedProfileWith({ eid: query.eid });
        if (error) {
          throw error;
        }
      } else if (query.oid && query.nid) {
        const url = new URL("https://channels.weixin.qq.com/web/pages/feed");
        url.searchParams.set("oid", query.oid);
        url.searchParams.set("nid", query.nid);
        const [error] = await window.fetchFeedProfileWith({ url: url.href });
        if (error) {
          throw error;
        }
      } else {
        const feed = properties.$pinia?._s?.get("feed")?.feed;
        if (feed?.id && feed.objectNonceId) {
          await api.finderGetCommentDetail({
            needObject: 1,
            lastBuffer: "",
            scene: 146,
            direction: 2,
            identityScene: 2,
            pullScene: 6,
            objectid: feed.id,
            objectNonceId: feed.objectNonceId,
          });
        }
      }
    },
    async start() {
      try {
        for (let attempt = 0; !this.capture(); attempt++) {
          if (attempt >= 300) {
            throw new Error("LiteApp Webpack or Vue runtime unavailable");
          }
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
        this.app = document.querySelector("#app").__vue_app__;
        await this.report("loading");
        const api_module = this.find_api_module();
        const response = await fetch(base + "bootstrap");
        const result = await response.json();
        if (!response.ok) {
          throw new Error(result.error || "LiteApp bootstrap failed");
        }
        if (typeof window.__wx_username === "undefined") {
          window.__wx_username = "";
        }
        await view.load_bootstrap(result.bootstrap);
        if (typeof WXU === "undefined") {
          throw new Error("project injection assets did not initialize");
        }
        if (document.readyState !== "loading") {
          WXE.emit(WXE.Events.DOMContentLoaded, { href: location.href });
        }
        WXU.on("channels:Init", () => {
          this.init_events++;
          void this.report("business_event");
        });
        WXU.on("channels:OnFeedProfileLoaded", () => {
          this.detail_events++;
          void this.report("business_event");
        });
        this.install(api_module.exports);
        this.install_navigation();
        const services = this.services();
        WXU.emit("channels:APILoaded", services);
        await this.report("installed");
        await this.replay(services);
        await this.report("ready");
      } catch (error) {
        this.error = String(error?.message || error).slice(0, 300);
        console.warn("[wxchannels LiteApp rewrite]", error);
        await this.report("failed");
      }
    },
  };
  const view = {
    load_script(url, name) {
      const parsed = new URL(url, "https://res.wx.qq.com");
      if (
        parsed.origin !== "https://res.wx.qq.com" ||
        !parsed.pathname.startsWith("/__assets/")
      ) {
        return Promise.reject(new Error("unexpected injection asset URL"));
      }
      return new Promise((resolve, reject) => {
        const script = document.createElement("script");
        script.setAttribute("data-n", name);
        script.src = parsed.href;
        script.onload = resolve;
        script.onerror = function () {
          return reject(
            new Error("failed to load injection asset: " + parsed.pathname),
          );
        };
        document.head.appendChild(script);
      });
    },
    async load_bootstrap(html) {
      const parsed = new DOMParser().parseFromString(html, "text/html");
      for (const node of parsed.head.children) {
        if (node.tagName === "SCRIPT") {
          if (node.hasAttribute("src")) {
            await this.load_script(
              node.getAttribute("src"),
              "wxchannels-liteapp-project-script",
            );
          } else {
            const script = document.createElement("script");
            script.setAttribute(
              "data-n",
              "wxchannels-liteapp-project-inline-script",
            );
            script.textContent = node.textContent;
            document.head.appendChild(script);
          }
        } else if (node.tagName === "LINK" && node.rel === "stylesheet") {
          const link = document.createElement("link");
          link.setAttribute("data-n", "wxchannels-liteapp-project-stylesheet");
          link.rel = "stylesheet";
          link.href = new URL(
            node.getAttribute("href"),
            "https://res.wx.qq.com",
          ).href;
          document.head.appendChild(link);
        } else if (node.tagName === "STYLE") {
          const style = document.createElement("style");
          style.setAttribute(
            "data-n",
            "wxchannels-liteapp-project-inline-style",
          );
          style.textContent = node.textContent;
          document.head.appendChild(style);
        }
      }
    },
  };
  window.__wxchannels_liteapp_rewrite__ = model;
  void model.start();
})();

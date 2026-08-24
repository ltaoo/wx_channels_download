/**
 * @file Injected runtime environment, service addresses, and global config entry.
 */
if (typeof window.__d_config === "undefined") {
  window.__d_config = {};
}

var WXEnv = (() => {
  const defaults = {
    apiHost: "127.0.0.1",
    apiOrigin: "http://127.0.0.1:2022",
    apiProtocol: "http",
    remoteServerEnabled: false,
    remoteServerOrigin: "https://support.weixin.qq.com",
    maxRunning: 3,
    downloadFilenameTemplate: undefined,
    defaultHighest: false,
    downloadPauseWhenDownload: false,
    downloadInFrontend: false,
    downloadForceCheckAllFeeds: false,
    assetsFallbackBase: "/__assets",
  };
  const runtime_env = { ...window.__d_config };
  const api_origin = runtime_env.apiOrigin;
  const api_protocol = runtime_env.apiProtocol;
  const api_host = runtime_env.apiHost;
  const derived = {
    apiOrigin: api_origin,
    apiProtocol: api_protocol,
    apiHost: api_host,
    downloaderOrigin: api_origin,
    downloaderProtocol: api_protocol,
    downloaderWSURL:
      api_protocol && api_host
        ? `${ws_protocol(api_protocol)}://${api_host}/ws/v1/download_task`
        : undefined,
  };
  const ua = navigator.userAgent || navigator.platform || "";

  function config() {
    return window.__d_config || {};
  }

  function own_value(source, name) {
    if (source && Object.prototype.hasOwnProperty.call(source, name)) {
      return source[name];
    }
    return undefined;
  }

  function get(name) {
    let v = own_value(derived, name);
    console.log("env.js - get", name, v, derived, runtime_env);
    if (typeof v !== "undefined") {
      return v;
    }
    v = own_value(runtime_env, name);
    if (typeof v !== "undefined") {
      return v;
    }
    return defaults[name];
  }

  function apply_runtime_env(values) {
    if (!values || typeof values !== "object") {
      return runtime_env;
    }
    Object.assign(runtime_env, values);
    return runtime_env;
  }

  function host_port(hostname, port) {
    const host = String(hostname || "").trim();
    if (!host) {
      return "";
    }
    if (
      port === undefined ||
      port === null ||
      port === "" ||
      Number(port) === 0
    ) {
      return host;
    }
    return host + ":" + port;
  }

  function origin(protocol, addr) {
    if (!protocol || !addr) {
      return "";
    }
    return protocol + "://" + addr;
  }

  function ws_protocol(protocol) {
    return protocol === "https" ? "wss" : "ws";
  }

  function assets_base_URL() {
    const cfg = config();
    const explicitBase = own_value(cfg, "assets_base_url");
    if (explicitBase) {
      return String(explicitBase).replace(/\/$/, "");
    }
    const api_origin = get("apiOrigin");
    if (api_origin) {
      return String(api_origin).replace(/\/$/, "") + "/__assets";
    }
    return get("assetsFallbackBase");
  }

  function asset_URL(path) {
    const base = assets_base_URL();
    if (path.startsWith("/public/")) {
      const version = encodeURIComponent(config().version || "static");
      return `${base}${path}?v=${version}`;
    }
    return `${base}${path}`;
  }

  return {
    get,
    merge(value) {
      Object.assign(derived, value);
    },
    get config() {
      return config();
    },
    get userAgent() {
      return ua;
    },
    get isWin() {
      return /Windows|Win/i.test(ua);
    },
    get isWeChatBrowser() {
      return /MicroMessenger/i.test(ua);
    },
    get isWxwork() {
      return window.ua && window.ua.includes("wxwork");
    },
    hostPort: host_port,
    origin,
    wsProtocol: ws_protocol,
    assetUrl: asset_URL,
  };
})();

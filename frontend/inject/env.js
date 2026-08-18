/**
 * @file Injected runtime environment, service addresses, and global config entry.
 */
if (typeof window.__d_config === "undefined") {
  window.__d_config = {};
}

var WXEnv = (() => {
  const proxy_origin = "https://weixin110.qq.com";
  const defaults = {
    apiHost: "weixin110.qq.com",
    apiOrigin: proxy_origin,
    apiProtocol: "https",
    remoteServerEnabled: false,
    remoteServerOrigin: proxy_origin,
    maxRunning: 3,
    downloadFilenameTemplate: undefined,
    defaultHighest: false,
    downloadPauseWhenDownload: false,
    downloadInFrontend: false,
    downloadForceCheckAllFeeds: false,
    assetsFallbackBase: `${proxy_origin}/__assets`,
  };
  const runtime_env = { ...window.__d_config };
  const api_url = new URL(proxy_origin);
  const api_protocol = api_url.protocol.replace(":", "");
  const api_host = api_url.host;
  const derived = {
    apiOrigin: api_url.origin,
    apiProtocol: api_protocol,
    apiHost: api_host,
    downloaderOrigin: api_url.origin,
    downloaderProtocol: api_protocol,
    downloaderWSURL: `${ws_protocol(api_protocol)}://${api_host}/ws/v1/download_task`,
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
    const host = normalize_hostname(hostname);
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

  function normalize_hostname(hostname) {
    const value = String(hostname || "").trim();
    if (!value) {
      return "";
    }
    const unwrapped =
      value.startsWith("[") && value.endsWith("]") ? value.slice(1, -1) : value;
    if (unwrapped === "0.0.0.0" || unwrapped === "::") {
      return "127.0.0.1";
    }
    return value;
  }

  function normalize_host_addr(addr) {
    const value = String(addr || "").trim();
    if (!value) {
      return "";
    }
    const match = value.match(/^(\[[^\]]+\]|[^:]+)(?::(\d+))?$/);
    if (!match) {
      return value;
    }
    const host = normalize_hostname(match[1]);
    return match[2] ? host + ":" + match[2] : host;
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
    normalizeHostname: normalize_hostname,
    normalizeHostAddr: normalize_host_addr,
    origin,
    wsProtocol: ws_protocol,
    assetUrl: asset_URL,
  };
})();

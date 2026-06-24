/**
 * @file 注入脚本运行环境、服务地址和全局配置入口
 */
if (typeof window.__wx_channels_config__ === "undefined") {
  window.__wx_channels_config__ = {};
}
if (typeof window.WXVariable === "undefined") {
  window.WXVariable = {};
}
if (typeof window.__wx_channels_env__ === "undefined") {
  window.__wx_channels_env__ = {};
}

// console.log(window.__wx_channels_config__, window.WXVariable);

var WXEnv = (() => {
  //   var FakeLocalAPIServerAddr = "kf.qq.com";
  // var FakeRemoteAPIServerAddr = "weixin110.qq.com";
  // var FakeRemoteAPIServerProtocol = "https";
  // var FakeLocalAPIServerProtocol = "https";
  // var WSServerProtocol = "wss";
  const defaults = {
    /** 本地接口 */
    localAPIServerProtocol: "https",
    localAPIServerAddr: "kf.qq.com",
    /** 远端接口 */
    remoteAPIServerProtocol: "https",
    remoteAPIServerAddr: "weixin110.qq.com",
    /** 下载面板接口地址 */
    downloadPanelAPIServerAddr: "kf.qq.com",
    downloadPanelAPIServerProtocol: "https",
    /** 静态资源 prefix */
    assetsFallbackBase: "http://127.0.0.1:2022/__wx_channels_assets",
  };
  const runtimeEnv = window.__wx_channels_env__;
  const ua = navigator.userAgent || navigator.platform || "";

  function config() {
    return {
      ...(window.__wx_channels_config__ || {}),
      ...(window.WXVariable || {}),
    };
  }

  function ownValue(source, name) {
    if (source && Object.prototype.hasOwnProperty.call(source, name)) {
      return source[name];
    }
    return undefined;
  }

  function explicitEnvValue(name) {
    const runtimeValue = ownValue(runtimeEnv, name);
    if (typeof runtimeValue !== "undefined") {
      return runtimeValue;
    }
    return ownValue(config(), name);
  }

  function envValue(name) {
    const value = explicitEnvValue(name);
    if (typeof value !== "undefined") {
      return value;
    }
    return defaults[name];
  }

  function applyRuntimeEnv(values) {
    if (!values || typeof values !== "object") {
      return runtimeEnv;
    }
    Object.assign(runtimeEnv, values);
    refreshLegacyGlobals();
    return runtimeEnv;
  }

  function hostPort(hostname, port) {
    const host = normalizeHostname(hostname);
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

  function normalizeHostname(hostname) {
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

  function normalizeHostAddr(addr) {
    const value = String(addr || "").trim();
    if (!value) {
      return "";
    }
    const match = value.match(/^(\[[^\]]+\]|[^:]+)(?::(\d+))?$/);
    if (!match) {
      return value;
    }
    const host = normalizeHostname(match[1]);
    return match[2] ? host + ":" + match[2] : host;
  }

  function origin(protocol, addr) {
    if (!protocol || !addr) {
      return "";
    }
    return protocol + "://" + addr;
  }

  function wsProtocol(protocol) {
    return protocol === "https" ? "wss" : "ws";
  }

  function configuredAPIServer() {
    const cfg = config();
    if (cfg.remoteServerEnabled) {
      return {
        addr:
          explicitEnvValue("remoteAPIServerAddr") ||
          hostPort(cfg.remoteServerHostname, cfg.remoteServerPort) ||
          defaults.remoteAPIServerAddr,
        protocol:
          explicitEnvValue("remoteAPIServerProtocol") ||
          cfg.remoteServerProtocol ||
          defaults.remoteAPIServerProtocol,
      };
    }
    return {
      addr:
        explicitEnvValue("localAPIServerAddr") ||
        cfg.apiServerAddr ||
        hostPort(cfg.apiServerHostname, cfg.apiServerPort) ||
        defaults.localAPIServerAddr,
      protocol:
        explicitEnvValue("localAPIServerProtocol") ||
        cfg.apiServerProtocol ||
        defaults.localAPIServerProtocol,
    };
  }

  function apiServer() {
    const configured = configuredAPIServer();
    const addr = explicitEnvValue("downloadPanelAPIServerAddr");
    const protocol = explicitEnvValue("downloadPanelAPIServerProtocol");
    return {
      addr: addr || configured.addr || defaults.downloadPanelAPIServerAddr,
      protocol:
        protocol ||
        configured.protocol ||
        defaults.downloadPanelAPIServerProtocol,
    };
  }

  function remoteAPIServer() {
    const cfg = config();
    return {
      addr:
        explicitEnvValue("remoteAPIServerAddr") ||
        hostPort(cfg.remoteServerHostname, cfg.remoteServerPort) ||
        defaults.remoteAPIServerAddr,
      protocol:
        explicitEnvValue("remoteAPIServerProtocol") ||
        cfg.remoteServerProtocol ||
        defaults.remoteAPIServerProtocol,
    };
  }

  function configuredLocalAPIServer() {
    const cfg = config();
    return {
      addr:
        explicitEnvValue("localAPIServerAddr") ||
        cfg.apiServerAddr ||
        hostPort(cfg.apiServerHostname, cfg.apiServerPort) ||
        defaults.localAPIServerAddr,
      protocol:
        explicitEnvValue("localAPIServerProtocol") ||
        cfg.apiServerProtocol ||
        defaults.localAPIServerProtocol,
    };
  }

  function officialRemoteServer() {
    const cfg = config();
    return {
      addr: hostPort(
        cfg.officialRemoteServerHostname,
        cfg.officialRemoteServerPort,
      ),
      protocol: cfg.officialRemoteServerProtocol || "https",
    };
  }

  function assetsBaseURL() {
    const cfg = window.__wx_channels_config__ || {};
    if (cfg.apiServerProtocol && cfg.apiServerAddr) {
      return (
        origin(cfg.apiServerProtocol, cfg.apiServerAddr) +
        "/__wx_channels_assets"
      );
    }
    if (cfg.Protocol && cfg.Addr) {
      return origin(cfg.Protocol, cfg.Addr) + "/__wx_channels_assets";
    }
    return envValue("assetsFallbackBase");
  }

  function assetUrl(path) {
    const base = assetsBaseURL();
    if (path.startsWith("/lib/")) {
      const version = encodeURIComponent(
        window.__wx_channels_version__ || "static",
      );
      return `${base}${path}?v=${version}`;
    }
    return `${base}${path}`;
  }

  function legacyGlobals() {
    return {
      FakeLocalAPIServerAddr: envValue("localAPIServerAddr"),
      FakeRemoteAPIServerAddr: envValue("remoteAPIServerAddr"),
      FakeRemoteAPIServerProtocol: envValue("remoteAPIServerProtocol"),
      FakeLocalAPIServerProtocol: envValue("localAPIServerProtocol"),
      DownloadPanelAPIServerAddr: apiServer().addr,
      DownloadPanelAPIServerProtocol: apiServer().protocol,
      FakeAPIServerAddr: apiServer().addr,
      APIServerProtocol: apiServer().protocol,
      WSServerProtocol: wsProtocol(apiServer().protocol),
      WXUserAgent: ua,
      isWin: /Windows|Win/i.test(ua),
      __wx_assets_base: assetsBaseURL(),
    };
  }

  function refreshLegacyGlobals() {
    const values = legacyGlobals();
    Object.keys(values).forEach((name) => {
      window[name] = values[name];
    });
    return values;
  }

  return {
    defaults,
    runtimeEnv,
    applyRuntimeEnv,
    refreshLegacyGlobals,
    get config() {
      return config();
    },
    get userAgent() {
      return ua;
    },
    get isWin() {
      return /Windows|Win/i.test(ua);
    },
    get isChannels() {
      return window.location.href.includes("weixin.qq.com");
    },
    get isWxwork() {
      return window.ua && window.ua.includes("wxwork");
    },
    hostPort,
    normalizeHostname,
    normalizeHostAddr,
    origin,
    wsProtocol,
    get configuredAPI() {
      return configuredAPIServer();
    },
    get localAPI() {
      return configuredLocalAPIServer();
    },
    get remoteAPI() {
      return remoteAPIServer();
    },
    get api() {
      return apiServer();
    },
    get apiServerAddr() {
      return apiServer().addr;
    },
    get apiServerProtocol() {
      return apiServer().protocol;
    },
    get apiOrigin() {
      const api = apiServer();
      return origin(api.protocol, api.addr);
    },
    get wsServerProtocol() {
      return wsProtocol(apiServer().protocol);
    },
    get remoteAPIOrigin() {
      const remote = remoteAPIServer();
      return origin(remote.protocol, remote.addr);
    },
    get localAPIOrigin() {
      const local = configuredLocalAPIServer();
      return origin(local.protocol, local.addr);
    },
    get officialAccountOrigin() {
      const remote = officialRemoteServer();
      if (remote.addr) {
        return origin(remote.protocol, remote.addr);
      }
      return this.localAPIOrigin;
    },
    get assetsBaseURL() {
      return assetsBaseURL();
    },
    assetUrl,
    get channelsLocalWSURL() {
      return (
        wsProtocol(configuredLocalAPIServer().protocol) +
        "://" +
        configuredLocalAPIServer().addr +
        "/ws/channels"
      );
    },
    get channelsWSURL() {
      return (
        this.wsServerProtocol + "://" + this.apiServerAddr + "/ws/channels"
      );
    },
    get downloaderWSURL() {
      return (
        this.wsServerProtocol + "://" + this.apiServerAddr + "/ws/downloader"
      );
    },
    get mpWSURL() {
      return this.wsServerProtocol + "://" + this.apiServerAddr + "/ws/mp";
    },
  };
})();

WXEnv.refreshLegacyGlobals();
var FakeLocalAPIServerAddr = window.FakeLocalAPIServerAddr;
var FakeRemoteAPIServerAddr = window.FakeRemoteAPIServerAddr;
var FakeRemoteAPIServerProtocol = window.FakeRemoteAPIServerProtocol;
var FakeLocalAPIServerProtocol = window.FakeLocalAPIServerProtocol;
var DownloadPanelAPIServerAddr = window.DownloadPanelAPIServerAddr;
var DownloadPanelAPIServerProtocol = window.DownloadPanelAPIServerProtocol;
var FakeAPIServerAddr = window.FakeAPIServerAddr;
var APIServerProtocol = window.APIServerProtocol;
var WSServerProtocol = window.WSServerProtocol;
var WXUserAgent = window.WXUserAgent;
var isWin = window.isWin;
var __wx_assets_base = window.__wx_assets_base;
function __wx_asset_url(path) {
  return WXEnv.assetUrl(path);
}

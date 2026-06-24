/**
 * @file 注入脚本运行环境、服务地址和全局配置入口
 */
if (typeof window.__wx_channels_config__ === "undefined") {
  window.__wx_channels_config__ = {};
}
if (typeof window.WXVariable === "undefined") {
  window.WXVariable = {};
}

// console.log(window.__wx_channels_config__, window.WXVariable);

var WXEnv = (() => {
  const defaults = {
    /** 本地接口 */
    localAPIServerProtocol: "http",
    localAPIServerAddr: "127.0.0.1:2022",
    /** 远端接口 */
    remoteAPIServerProtocol: "http",
    remoteAPIServerAddr: "127.0.0.1:2022",
    /** 下载面板接口地址 */
    downloadPanelAPIServerAddr: "127.0.0.1:2022",
    downloadPanelAPIServerProtocol: "http",
    /** 静态资源 prefix */
    assetsFallbackBase: "http://127.0.0.1:2022/__wx_channels_assets",
  };
  const runtimeEnv = window.__wx_channels_env__ || {};
  const ua = navigator.userAgent || navigator.platform || "";

  function config() {
    return {
      ...(window.__wx_channels_config__ || {}),
      ...(window.WXVariable || {}),
    };
  }

  function envValue(name) {
    const cfg = config();
    if (Object.prototype.hasOwnProperty.call(runtimeEnv, name)) {
      return runtimeEnv[name];
    }
    if (Object.prototype.hasOwnProperty.call(cfg, name)) {
      return cfg[name];
    }
    return defaults[name];
  }

  function hostPort(hostname, port) {
    if (!hostname) {
      return "";
    }
    if (
      port === undefined ||
      port === null ||
      port === "" ||
      Number(port) === 0
    ) {
      return hostname;
    }
    return hostname + ":" + port;
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
          hostPort(cfg.remoteServerHostname, cfg.remoteServerPort) ||
          defaults.remoteAPIServerAddr,
        protocol: cfg.remoteServerProtocol || defaults.remoteAPIServerProtocol,
      };
    }
    return {
      addr:
        cfg.apiServerAddr ||
        hostPort(cfg.apiServerHostname, cfg.apiServerPort) ||
        defaults.localAPIServerAddr,
      protocol: cfg.apiServerProtocol || defaults.localAPIServerProtocol,
    };
  }

  function apiServer() {
    const configured = configuredAPIServer();
    return {
      addr: envValue("downloadPanelAPIServerAddr") || configured.addr,
      protocol:
        envValue("downloadPanelAPIServerProtocol") || configured.protocol,
    };
  }

  function remoteAPIServer() {
    const cfg = config();
    return {
      addr:
        hostPort(cfg.remoteServerHostname, cfg.remoteServerPort) ||
        defaults.remoteAPIServerAddr,
      protocol: cfg.remoteServerProtocol || defaults.remoteAPIServerProtocol,
    };
  }

  function configuredLocalAPIServer() {
    const cfg = config();
    return {
      addr:
        cfg.apiServerAddr ||
        hostPort(cfg.apiServerHostname, cfg.apiServerPort) ||
        defaults.localAPIServerAddr,
      protocol: cfg.apiServerProtocol || defaults.localAPIServerProtocol,
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

  return {
    defaults,
    runtimeEnv,
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

var FakeLocalAPIServerAddr = WXEnv.defaults.localAPIServerAddr;
var FakeRemoteAPIServerAddr = WXEnv.defaults.remoteAPIServerAddr;
var FakeRemoteAPIServerProtocol = WXEnv.defaults.remoteAPIServerProtocol;
var FakeLocalAPIServerProtocol = WXEnv.defaults.localAPIServerProtocol;
var DownloadPanelAPIServerAddr = WXEnv.apiServerAddr;
var DownloadPanelAPIServerProtocol = WXEnv.apiServerProtocol;
var FakeAPIServerAddr = WXEnv.apiServerAddr;
var APIServerProtocol = WXEnv.apiServerProtocol;
var WSServerProtocol = WXEnv.wsServerProtocol;
var WXUserAgent = WXEnv.userAgent;
var isWin = WXEnv.isWin;
var __wx_assets_base = WXEnv.assetsBaseURL;
function __wx_asset_url(path) {
  return WXEnv.assetUrl(path);
}

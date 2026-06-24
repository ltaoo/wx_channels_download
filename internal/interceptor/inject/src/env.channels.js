/**
 * @file 视频号页面运行环境覆盖
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before env.channels.js");
}

(() => {
  const cfg = WXEnv.config;
  const localAPI = {
    addr:
      WXEnv.normalizeHostAddr(cfg.apiServerAddr) ||
      WXEnv.hostPort(cfg.apiServerHostname, cfg.apiServerPort) ||
      WXEnv.defaults.localAPIServerAddr,
    protocol: cfg.apiServerProtocol || WXEnv.defaults.localAPIServerProtocol,
  };
  const remoteAPI = {
    addr:
      WXEnv.hostPort(cfg.remoteServerHostname, cfg.remoteServerPort) ||
      WXEnv.defaults.remoteAPIServerAddr,
    protocol:
      cfg.remoteServerProtocol || WXEnv.defaults.remoteAPIServerProtocol,
  };
  const panelAPI = cfg.remoteServerEnabled ? remoteAPI : localAPI;
  const env = {
    localAPIServerAddr: "kf.qq.com",
    localAPIServerProtocol: "https",
    remoteAPIServerAddr: "kf.qq.com",
    remoteAPIServerProtocol: "https",
    downloadPanelAPIServerAddr: "kf.qq.com",
    downloadPanelAPIServerProtocol: "https",
  };

  if (cfg.apiServerProtocol && cfg.apiServerAddr) {
    env.assetsFallbackBase =
      WXEnv.origin(
        cfg.apiServerProtocol,
        WXEnv.normalizeHostAddr(cfg.apiServerAddr),
      ) + "/__wx_channels_assets";
  }

  WXEnv.applyRuntimeEnv(env);
})();

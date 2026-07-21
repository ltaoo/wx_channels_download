/**
 * @file Channels page runtime environment override.
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before channels.env.js");
}

(() => {
  const env = {
    // channelsHostname: "kf.qq.com",
    // channelsProtocol: "https",
    // downloadHostname: "weixin110.qq.com",
    // downloadProtocol: "https",
    channelsHostname: "127.0.0.1:2022",
    channelsProtocol: "http",
    downloadHostname: "127.0.0.1:2022",
    downloadProtocol: "http",
  };

  const cfg = WXEnv.config;
  if (cfg.apiServerProtocol && cfg.apiServerAddr) {
    env.assetsFallbackBase =
      WXEnv.origin(
        cfg.apiServerProtocol,
        WXEnv.normalizeHostAddr(cfg.apiServerAddr),
      ) + "/__assets";
  }

  WXEnv.applyRuntimeEnv(env);
})();

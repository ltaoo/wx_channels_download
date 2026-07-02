/**
 * @file Channels page runtime environment override.
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before env.channels.js");
}

(() => {
  const env = {
    channelsHostname: "kf.qq.com",
    channelsProtocol: "https",
    downloadHostname: "weixin110.qq.com",
    downloadProtocol: "https",
  };

  const cfg = WXEnv.config;
  if (cfg.apiServerProtocol && cfg.apiServerAddr) {
    env.assetsFallbackBase =
      WXEnv.origin(
        cfg.apiServerProtocol,
        WXEnv.normalizeHostAddr(cfg.apiServerAddr),
      ) + "/__wx_channels_assets";
  }

  WXEnv.applyRuntimeEnv(env);
})();

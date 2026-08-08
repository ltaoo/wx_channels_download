/**
 * @file Channels page runtime environment override.
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before channels.env.js");
}

var ChannelsEnv = (() => {
  var defaults = {
    apiOrigin: "127.0.0.1:2022",
    apiProtocol: "http",
    channelsWSURL: "ws://127.0.0.1:2022/ws/channels"
  };
  var derived = {};
  return {
    get(name) {
      let v = WXEnv.get(name);
      if (typeof v !== "undefined") {
        return v;
      }
      return defaults[name];
    },
  }
})();
// var ChannelsEnvGet = ChannelsEnv.get;
// var ChannelsEnvOrigin = ChannelsEnv.origin;
// var ChannelsEnvWSProtocol = ChannelsEnv.wsProtocol;
// var ChannelsEnvNormalizeHostAddr = ChannelsEnv.normalizeHostAddr;
// var ChannelsEnvConfig = ChannelsEnv.config;

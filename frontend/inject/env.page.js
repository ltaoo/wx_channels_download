/**
 * @file fakefeed.html debug environment overrides and mock API
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before env.page.js");
}

(() => {
  function ws_protocol(protocol) {
    return protocol === "https" ? "wss" : "ws";
  }
  var api_host = location.host;
  var api_protocol = location.protocol.replace(":", "");
  var page_env = {
    apiHost: api_host,
    apiOrigin: location.origin,
    apiProtocol: api_protocol,
    downloaderOrigin: location.origin,
    downloaderProtocol: api_protocol,
    downloaderWSURL: `${ws_protocol(api_protocol)}://${api_host}/ws/v1/download_task`,
  };
  WXEnv.merge(page_env);
})();

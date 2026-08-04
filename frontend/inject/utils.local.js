/**
 * WXU.error implementation for the local download manager page.
 *
 * The default implementation in utils.js targets proxied WeChat pages and reports
 * errors to /__wx_channels_api/error. The local management page does not have this
 * proxy virtual endpoint, so we only keep console logging and UI tips to avoid
 * making invalid requests.
 */
(function overrideLocalWXUError() {
  if (typeof WXU === "undefined") {
    return;
  }

  WXU.error = function localWXUError(params) {
    const options =
      typeof params === "string" ? { msg: params } : params || {};
    const message = String(options.msg || options.message || "未知错误");

    console.error("[WXU ERROR]", message, options);

    if ((options.alert ?? 1) && typeof WXU.toast === "function") {
      WXU.toast(message);
    }
  };
})();

/**
 * 本地下载管理页的 WXU.error 实现。
 *
 * utils.js 中的默认实现面向被代理的微信页面，会把错误上报到
 * /__wx_channels_api/error。本地管理页没有这个代理虚拟接口，因此只保留
 * 控制台记录和界面提示，避免产生无效请求。
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

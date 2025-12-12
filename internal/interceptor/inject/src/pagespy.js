var __wx_defaultConfig = {
  api: "debug.weixin.qq.com",
  clientOrigin: "https://debug.weixin.qq.com",
};
const config = __wx_defaultConfig;
if (WXD.config.pagespyServerAPI) {
  config.api = WXD.config.pagespyServerAPI;
}
if (WXD.config.pagespyServerProtocol) {
  config.clientOrigin =
    WXD.config.pagespyServerProtocol + "://" + config.api;
}
try {
  window.$pageSpy = new PageSpy({
    ...config,
    project: "WXChannel",
    autoRender: true,
    title: "WXChannel Debug",
  });
} catch (err) {
  alert(err.message);
}

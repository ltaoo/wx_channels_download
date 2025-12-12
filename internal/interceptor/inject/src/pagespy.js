var __wx_defaultConfig = {
  api: "debug.weixin.qq.com",
  clientOrigin: "https://debug.weixin.qq.com",
};
const config = __wx_defaultConfig;
if (WXU.config.pagespyServerAPI) {
  config.api = WXU.config.pagespyServerAPI;
}
if (WXU.config.pagespyServerProtocol) {
  config.clientOrigin =
    WXU.config.pagespyServerProtocol + "://" + config.api;
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

var defaultConfig = {
  api: "debug.weixin.qq.com",
  clientOrigin: "https://debug.weixin.qq.com",
};
const config = defaultConfig;
if (__wx_channels_config__.pagespyServerAPI) {
  config.api = __wx_channels_config__.pagespyServerAPI;
}
if (__wx_channels_config__.pagespyServerProtocol) {
  config.clientOrigin = __wx_channels_config__.pagespyServerProtocol + "://" + config.api;
}
try {
  window.$pageSpy = new PageSpy({
    ...config,
    project: "WXChannel",
    autoRender: true,
    title: "WXChannel Debug"
  });
} catch (err) {
  alert(err.message);
}
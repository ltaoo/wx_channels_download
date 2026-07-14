(() => {
  const config = {
    api: "127.0.0.1:6752",
    clientOrigin: "http://127.0.0.1:6752",
    enableSSL: false,
  };
  if (typeof WXU !== "undefined" && WXU.config.pagespyServerAPI) {
    config.api = WXU.config.pagespyServerAPI;
  }
  if (typeof WXU !== "undefined" && WXU.config.pagespyServerProtocol) {
    config.clientOrigin = WXU.config.pagespyServerProtocol + "://" + config.api;
    if (WXU.config.pagespyServerProtocol === 'https') {
      config.enableSSL = true;
    }
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
})();

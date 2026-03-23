(() => {
  const config = {
    api: "127.0.0.1:6752",
    // api: "debug.weixin.qq.com",
    // clientOrigin: "https://debug.weixin.qq.com",
  };
  try {
    // @ts-ignore
    new PageSpy({
      ...config,
      project: "WXChannel",
      autoRender: true,
      title: "WXChannel Debug",
    });
  } catch (err) {
    alert(err.message);
  }
})();

setTimeout(function () {
  try {
    window.$pageSpy = new PageSpy({
      api: "debug.funzm.com",
      clientOrigin: "https://debug.funzm.com",
      project: "WXChannel",
      autoRender: true,
      title: "WXChannel Debug"
    });
  } catch (err) {
    alert(err.message);
  }
}, 800);
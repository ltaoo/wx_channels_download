(function () {
  var runtime = typeof chrome !== "undefined" && chrome.runtime ? chrome.runtime : null;
  if (!runtime || !runtime.onMessage) {
    return;
  }

  runtime.onMessage.addListener(function (message, sender, sendResponse) {
    if (!message || message.type !== "WX_BROWSER_EXTENSION_REPORT") {
      return false;
    }

    var endpoint = message.endpoint;
    if (!endpoint && sender && sender.tab && sender.tab.url) {
      try {
        endpoint = new URL("/__wx_channels_api/platform/browser", sender.tab.url).href;
      } catch (e) {}
    }
    if (!endpoint) {
      sendResponse({ ok: false, error: "missing endpoint" });
      return false;
    }

    fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(message.payload || {}),
    })
      .then(function (resp) {
        sendResponse({ ok: resp.ok, status: resp.status });
      })
      .catch(function (err) {
        sendResponse({ ok: false, error: err && (err.message || String(err)) });
      });
    return true;
  });
})();

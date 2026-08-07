(function () {
  const state = {
    loading: false,
    lastResponse: null,
  };

  document.addEventListener("DOMContentLoaded", render);

  function render() {
    const app = document.getElementById("app");
    if (!app) return;
    app.innerHTML = [
      '<section class="wx-home-shell">',
      '  <header class="wx-home-header">',
      '    <div>',
      '      <h1>Scraper Fetch</h1>',
      "    </div>",
      '    <nav class="wx-home-nav">',
      '      <a href="/download">下载任务</a>',
      '      <a href="/content">内容库</a>',
      "    </nav>",
      "  </header>",
      '  <form id="fetchForm" class="wx-home-form">',
      '    <input id="urlInput" name="url" type="url" placeholder="https://" autocomplete="off" aria-label="URL" autofocus />',
      '    <button id="fetchButton" type="submit">获取</button>',
      "  </form>",
      '  <div id="status" class="wx-home-status" hidden></div>',
      '  <section id="result" class="wx-home-result" hidden>',
      '    <div class="wx-home-result-head">',
      '      <span id="resultPlatform"></span>',
      '      <button id="copyButton" type="button">复制 JSON</button>',
      "    </div>",
      '    <pre id="resultBody"></pre>',
      "  </section>",
      "</section>",
    ].join("");

    document.getElementById("fetchForm").addEventListener("submit", onSubmit);
    document.getElementById("copyButton").addEventListener("click", copyResult);
  }

  async function onSubmit(event) {
    event.preventDefault();
    if (state.loading) return;

    const input = document.getElementById("urlInput");
    const rawURL = input.value.trim();
    if (!rawURL) {
      showStatus("请输入 URL", "error");
      return;
    }

    setLoading(true);
    hideResult();
    showStatus("获取中...", "");

    try {
      const params = new URLSearchParams({ url: rawURL });
      const response = await fetch(`/api/scraper/fetch?${params.toString()}`);
      const payload = await response.json().catch(() => ({}));
      if (!response.ok || payload.code !== 0) {
        throw new Error(payload.msg || response.statusText || "请求失败");
      }
      state.lastResponse = payload.data;
      showStatus("", "");
      renderResult(payload.data);
    } catch (error) {
      showStatus(error.message || String(error), "error");
    } finally {
      setLoading(false);
    }
  }

  function renderResult(data) {
    const result = document.getElementById("result");
    const platform = document.getElementById("resultPlatform");
    const body = document.getElementById("resultBody");
    platform.textContent = data && data.platform ? data.platform : "result";
    body.textContent = JSON.stringify(data, null, 2);
    result.hidden = false;
  }

  function hideResult() {
    const result = document.getElementById("result");
    result.hidden = true;
  }

  function showStatus(message, type) {
    const status = document.getElementById("status");
    status.textContent = message;
    status.className = "wx-home-status" + (type ? " " + type : "");
    status.hidden = !message;
  }

  function setLoading(loading) {
    state.loading = loading;
    const button = document.getElementById("fetchButton");
    button.disabled = loading;
    button.textContent = loading ? "获取中" : "获取";
  }

  async function copyResult() {
    if (!state.lastResponse) return;
    const text = JSON.stringify(state.lastResponse, null, 2);
    try {
      await navigator.clipboard.writeText(text);
      showStatus("已复制 JSON", "");
    } catch (error) {
      showStatus("复制失败: " + (error.message || String(error)), "error");
    }
  }
})();

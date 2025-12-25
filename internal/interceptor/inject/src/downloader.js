/**
 * @file 下载管理
 */
(() => {
  const tasks = new Map();
  function format_speed(bps) {
    const kb = 1024,
      mb = kb * 1024;
    if (!bps) return "0 B/s";
    if (bps >= mb) return (bps / mb).toFixed(2) + " MB/s";
    if (bps >= kb) return (bps / kb).toFixed(2) + " KB/s";
    return bps + " B/s";
  }
  function percent(t) {
    const total = t.meta && t.meta.res ? t.meta.res.size : 0;
    const cur = t.progress ? t.progress.downloaded : 0;
    if (!total) return 0;
    return Math.min(100, Math.floor((cur * 100) / total));
  }

  function name_of(t) {
    if (t.meta && t.meta.opts && t.meta.opts.name) return t.meta.opts.name;
    if (t.meta && t.meta.res) {
      if (t.meta.res.name) return t.meta.res.name;
      if (t.meta.res.files && t.meta.res.files.length > 0)
        return t.meta.res.files[0].name;
    }
    return "unknown";
  }
  function total_speed() {
    let sum = 0;
    tasks.forEach((t) => {
      if (
        t.status === "running" &&
        t.progress &&
        typeof t.progress.speed === "number"
      ) {
        sum += t.progress.speed;
      }
    });
    return sum;
  }
  function render(selector) {
    const tbody = document.querySelector(selector);
    if (!tbody) return;
    tbody.innerHTML = "";
    Array.from(tasks.values()).forEach((t) => {
      const tr = document.createElement("tr");
      const pr = percent(t);
      tr.innerHTML = `
          <td>${t.id}</td>
          <td>${name_of(t)}</td>
          <td>${t.status}</td>
          <td>${format_speed(t.progress ? t.progress.speed : 0)}</td>
          <td>
            <div class="bar"><div style="width:${pr}%"></div></div>
            ${pr}%
          </td>`;
      tbody.appendChild(tr);
    });
    // document.getElementById("totalSpeed").textContent = format_speed(
    //   total_speed()
    // );
  }

  function upsert(task) {
    if (!task || !task.id) return;
    tasks.set(task.id, task);
  }

  function connect(selector) {
    const ws = new WebSocket(
      (location.protocol === "https:" ? "wss://" : "ws://") +
        "api.channels.qq.com" +
        "/ws"
    );
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.type === "tasks") {
        if (Array.isArray(msg.data)) {
          msg.data.forEach(upsert);
        }
        render(selector);
        return;
      }
      if (msg.type === "event") {
        const evt = msg && msg.data ? msg.data : null;
        const task = evt ? evt.Task || evt.task : null; // 兼容大小写字段
        if (task) {
          upsert(task);
        }
        render(selector);
      }
    };

    document.addEventListener("click", async (e) => {
      if (e.target && e.target.classList.contains("start-btn")) {
        const id = e.target.getAttribute("data-id");
        var [err, data] = await WXU.request({
          method: "POST",
          url: "https://api.channels.qq.com/api/task/start",
          body: { id },
        });
        if (err) {
          WXU.error({
            msg: err.message,
          });
          return;
        }
        console.log(data);
      }
    });
  }

  function insert_downloader() {
    var $button = download_btn5();
    var $download_panel = document.createElement("div");
    $download_panel.innerHTML = `<table style="width: 460px;">
  <thead>
    <tr><th>ID</th><th>Name</th><th>Status</th><th>Speed</th><th>Progress</th></tr>
  </thead>
  <tbody id="downloader_container" style="min-width: 200px;">
</table>`;
    Weui.Popover($button, {
      content: $download_panel.innerHTML,
      placement: "bottom-end", // Default is now bottom-start (arrow on left)
      closeOnClickOutside: true,
    });
    var $header = document.querySelector(".home-header");
    var $box = $header.children[$header.children.length - 1];
    var $btn_wrap = $box.children[0];
    $btn_wrap.insertBefore($button, $btn_wrap.firstChild);

    // var $absolute = document.createElement("div");
    // $absolute.style.position = "absolute";
    // $absolute.style.top = "50px";
    // $absolute.style.left = "50px";
    // $absolute.style.zIndex = 9999;
    // document.body.appendChild($absolute);
    // $absolute.appendChild($download_panel);

    connect("#downloader_container");
  }

  setTimeout(() => {
    insert_downloader();
  }, 3000);
})();

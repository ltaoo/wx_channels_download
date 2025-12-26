/**
 * @file 下载管理
 */
(() => {
  // Styles
  const style = document.createElement("style");
  style.textContent = `
    .download-list {
      width: 400px;
      max-height: 400px;
      overflow-y: auto;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    }
    .download-item {
      display: flex;
      align-items: center;
      padding: 10px;
      border-bottom: 1px solid #f0f0f0;
      background: #fff;
    }
    .download-item:hover {
      background: #f9f9f9;
    }
    .download-icon {
      width: 32px;
      height: 32px;
      margin-right: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: #f5f5f5;
      border-radius: 4px;
      color: #666;
    }
    .download-info {
      flex: 1;
      overflow: hidden;
      min-width: 0;
    }
    .download-title {
      font-size: 14px;
      color: #333;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-bottom: 4px;
      line-height: 1.4;
    }
    .download-meta {
      font-size: 12px;
      color: #999;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .download-action {
      margin-left: 10px;
      cursor: pointer;
      color: #555;
      padding: 6px;
      border-radius: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .download-action:hover {
      background: #eee;
    }
    .download-progress-bar {
      height: 3px;
      background: #eee;
      border-radius: 1.5px;
      width: 100%;
      margin-top: 6px;
      overflow: hidden;
    }
    .download-progress-inner {
      height: 100%;
      background: #07c160;
      transition: width 0.3s;
    }
  `;
  document.head.appendChild(style);

  // Icons
  const FileIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/></svg>`;
  const FolderIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>`;

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
    const container = document.querySelector(selector);
    if (!container) return;
    container.innerHTML = "";

    const list = Array.from(tasks.values()).reverse(); // Newest first

    if (list.length === 0) {
      container.innerHTML = `<div style="padding: 20px; text-align: center; color: #999; font-size: 14px;">暂无下载任务</div>`;
      return;
    }

    list.forEach((t) => {
      const item = document.createElement("div");
      item.className = "download-item";

      const pr = percent(t);
      const isCompleted =
        t.status === "completed" ||
        t.status === "success" ||
        t.status === "finished" ||
        (pr === 100 && t.status !== "running");

      let statusText = t.status;
      let progressDisplay = "";

      if (t.status === "running") {
        const speed = format_speed(t.progress ? t.progress.speed : 0);
        statusText = `${speed} • ${pr}%`;
        progressDisplay = `<div class="download-progress-bar"><div class="download-progress-inner" style="width:${pr}%"></div></div>`;
      } else if (isCompleted) {
        statusText = "已完成";
        // Calculate size
        const total = t.meta && t.meta.res ? t.meta.res.size : 0;
        if (total) {
          const mb = 1024 * 1024;
          statusText = (total / mb).toFixed(2) + " MB";
        }
      } else if (t.status === "failed") {
        statusText = "下载失败";
      } else if (t.status === "pending") {
        statusText = "等待中...";
      }

      const actionHtml = isCompleted
        ? `<div class="download-action" title="打开文件夹" data-filepath="${t.filepath}" data-id="${t.id}" data-action="open">${FolderIcon}</div>`
        : "";
      var filename = name_of(t);
      item.innerHTML = `
          <div class="download-icon">${FileIcon}</div>
          <div class="download-info">
            <div class="download-title" title="${filename}">${filename}</div>
            <div class="download-meta">
                <span>${statusText}</span>
            </div>
            ${progressDisplay}
          </div>
          ${actionHtml}
      `;
      container.appendChild(item);
    });

    // Bind events - Removed in favor of delegation
  }

  function upsert(task) {
    if (!task || !task.id) return;
    tasks.set(task.id, {
      ...task,
      filepath: (() => {
        if (task.meta.opts) {
          return `${task.meta.opts.path}/${task.meta.opts.name}`;
        }
        return "";
      })(),
    });
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

      const openBtn = e.target.closest('[data-action="open"]');
      if (openBtn) {
        e.stopPropagation();
        const id = openBtn.getAttribute("data-id");
        const filepath = openBtn.getAttribute("data-filepath");
        if (!filepath) {
          return;
        }
        var [err, data] = await WXU.request({
          method: "POST",
          url: "https://api.channels.qq.com/api/show_file",
          body: { filepath, id },
        });
        if (err) {
          WXU.error({
            msg: err.message,
          });
          return;
        }
      }
    });
  }

  function insert_downloader() {
    var $button = download_btn5();
    var $download_panel = document.createElement("div");
    // Change to div container
    $download_panel.innerHTML = `<div id="downloader_container" class="download-list"></div>`;

    var popover$ = Weui.Popover($button, {
      content: $download_panel.innerHTML,
      placement: "bottom-end", // Default is now bottom-start (arrow on left)
      closeOnClickOutside: true,
    });
    var $header = document.querySelector(".home-header");
    var $box = $header.children[$header.children.length - 1];
    var $btn_wrap = $box.children[0];
    $btn_wrap.insertBefore($button, $btn_wrap.firstChild);

    WXU.downloader.show = function () {
      popover$.open();
    };
    WXU.downloader.hide = function () {
      popover$.close();
    };
    WXU.downloader.toggle = function () {
      popover$.toggle();
    };
    connect("#downloader_container");
  }

  setTimeout(() => {
    insert_downloader();
  }, 3000);
})();

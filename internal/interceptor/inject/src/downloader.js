/**
 * @file 下载管理
 */
(() => {
  var { DropdownMenu, MenuItem } = WUI;
  // Icons
  const FileIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/></svg>`;
  const FolderIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>`;
  const PauseIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>`;
  const PlayIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>`;
  const DeleteIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg>`;

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
      container.innerHTML = `<div class="weui-loadmore weui-loadmore_line"><span class="weui-loadmore__tips">暂无下载任务</span></div>`;
      return;
    }

    list.forEach((t) => {
      const item = document.createElement("div");
      item.className = "weui-cell";

      const pr = percent(t);
      const isCompleted =
        t.status === "completed" ||
        t.status === "success" ||
        t.status === "finished" ||
        (pr === 100 && t.status !== "running");

      const isPaused = t.status === "paused" || t.status === "pause";
      const isRunning = t.status === "running";

      let statusText = t.status;
      let progressDisplay = "";

      if (isRunning) {
        const speed = format_speed(t.progress ? t.progress.speed : 0);
        statusText = `${speed} • ${pr}%`;
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
      } else if (isPaused) {
        statusText = `已暂停 • ${pr}%`;
      }

      // Action Buttons Logic
      let actionButtons = "";
      const btnStyle =
        "color: #FFFFFF; opacity: 0.8; margin-left: 12px; cursor: pointer; display: flex; align-items: center; justify-content: center;";

      if (isCompleted) {
        actionButtons += `
             <a href="javascript:" class="wx-download-item-open" aria-label="打开文件夹" title="打开文件夹" data-filepath="${t.filepath}" data-id="${t.id}" data-action="open" style="${btnStyle}">
               ${FolderIcon}
             </a>
             <a href="javascript:" class="wx-download-item-delete" aria-label="删除" title="删除" data-id="${t.id}" data-action="delete" style="${btnStyle}">
               ${DeleteIcon}
             </a>
           `;
      } else {
        if (isRunning) {
          actionButtons += `
                <a href="javascript:" class="wx-download-item-pause" aria-label="暂停" title="暂停" data-id="${t.id}" data-action="pause" style="${btnStyle}">
                  ${PauseIcon}
                </a>
              `;
        } else if (isPaused || t.status === "failed") {
          // Allow resume if paused or failed
          actionButtons += `
                <a href="javascript:" class="wx-download-item-resume" aria-label="继续" title="继续" data-id="${t.id}" data-action="resume" style="${btnStyle}">
                  ${PlayIcon}
                </a>
              `;
        }
        actionButtons += `
             <a href="javascript:" class="wx-download-item-delete" aria-label="删除" title="删除" data-id="${t.id}" data-action="delete" style="${btnStyle}">
               ${DeleteIcon}
             </a>
           `;
      }

      const actionHtml = `<div class="weui-cell__ft" style="display: flex; align-items: center;">${actionButtons}</div>`;

      var filename = name_of(t);

      // Custom dark theme styles inline for specific elements, plus classes
      item.setAttribute(
        "style",
        "padding: 16px; background-color: #191919; border-radius: 8px; margin-bottom: 8px; align-items: center;"
      );

      // File Icon size increase
      // We wrap the icon in a slightly larger container
      const iconSize = "50px";

      // Icon preparation
      let iconInner = FileIcon.replace('width="20"', 'width="32"').replace(
        'height="20"',
        'height="32"'
      );

      if (isRunning || isPaused) {
        const radius = 22;
        const circumference = 2 * Math.PI * radius;
        const offset = circumference - (pr / 100) * circumference;
        const strokeColor = isPaused ? "#FBC02D" : "#07C160"; // Wechat Green or Warning Yellow for pause

        iconInner = `
        <div style="position: relative; width: 50px; height: 50px; display: flex; align-items: center; justify-content: center;">
             <svg width="50" height="50" style="position: absolute; top: 0; left: 0; transform: rotate(-90deg);">
                <circle cx="25" cy="25" r="${radius}" stroke="rgba(255,255,255,0.1)" stroke-width="3" fill="none"></circle>
                <circle cx="25" cy="25" r="${radius}" stroke="${strokeColor}" stroke-width="3" fill="none" stroke-dasharray="${circumference}" stroke-dashoffset="${offset}" stroke-linecap="round"></circle>
             </svg>
             <div style="position: relative; z-index: 1; display: flex;">
               ${iconInner}
             </div>
        </div>
        `;
      }

      item.innerHTML = `
          <div class="weui-cell__hd" aria-hidden="true" style="position: relative; margin-right: 16px; width: ${iconSize}; height: ${iconSize}; display: flex; align-items: center; justify-content: center; color: #FFFFFF;">
            ${iconInner}
          </div>
          <div class="weui-cell__bd" style="min-width:0;">
            <p class="weui-ellipsis" style="color: #FFFFFF; font-weight: 500; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" title="${filename}">${filename}</p>
            <div class="weui-cell__desc" style="margin-top: 4px; color: #AAAAAA; font-size: 12px;">${statusText}</div>
            ${typeof pr === "number" && !isCompleted
          ? `<div style="height: 4px; background: rgba(255,255,255,0.1); border-radius: 2px; margin-top: 6px; overflow: hidden; display: none;"><div style="width: ${pr}%; background: #07C160; height: 100%; transition: width 0.2s;"></div></div>`
          : ""
        }
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
      "127.0.0.1:2022" +
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

      // Handle Action Buttons (Pause, Resume, Delete)
      const actionBtn = e.target.closest("[data-action]");
      if (actionBtn) {
        e.stopPropagation();
        const action = actionBtn.getAttribute("data-action");
        const id = actionBtn.getAttribute("data-id");

        if (action === "open") {
          const filepath = actionBtn.getAttribute("data-filepath");
          if (!filepath) return;
          var [err, data] = await WXU.request({
            method: "POST",
            url: "https://api.channels.qq.com/api/show_file",
            body: { filepath, id },
          });
          if (err) {
            WXU.error({
              msg: err.message,
            });
          }
          return;
        }

        let url = "";
        if (action === "pause")
          url = "https://api.channels.qq.com/api/task/pause";
        else if (action === "resume")
          url = "https://api.channels.qq.com/api/task/resume";
        else if (action === "delete")
          url = "https://api.channels.qq.com/api/task/delete";

        if (url) {
          var [err, data] = await WXU.request({
            method: "POST",
            url: url,
            body: { id },
          });
          if (err) {
            WXU.error({
              msg: err.message,
            });
            return;
          }
        }
      }
    });
  }

  function insert_downloader() {
    var $button = download_btn5();
    var $download_panel = document.createElement("div");

    // Inject Custom Styles for Dark Theme Scrollbar and Container
    const style_id = "wx-downloader-style";
    if (!document.getElementById(style_id)) {
      const style = document.createElement("style");
      style.id = style_id;
      style.textContent = `
          .wx-dl-panel-container { 
            width: 400px; 
            max-height: 450px; 
            background-color: #252525; 
            border-radius: 8px; 
            display: flex; 
            flex-direction: column; 
            padding: 12px; 
            box-sizing: border-box;
          }
          .wx-dl-dark-scroll::-webkit-scrollbar { width: 6px; }
          .wx-dl-dark-scroll::-webkit-scrollbar-track { background: transparent; }
          .wx-dl-dark-scroll::-webkit-scrollbar-thumb { background-color: rgba(255, 255, 255, 0.2); border-radius: 3px; }
          
          /* Custom Menu Styles */
          .wx-dl-header { display: flex; justify-content: space-between; align-items: center; padding-bottom: 8px; margin-bottom: 4px; flex-shrink: 0; border-bottom: 1px solid rgba(255,255,255,0.05); }
          .wx-dl-title { font-size: 16px; font-weight: 600; color: #fff; }
          .wx-dl-more-btn { color: #fff; cursor: pointer; padding: 4px; border-radius: 4px; opacity: 0.8; transition: opacity 0.2s; position: relative; }
          .wx-dl-more-btn:hover { opacity: 1; background-color: rgba(255,255,255,0.1); }
          
          .wx-dl-dropdown { 
            position: absolute; top: 100%; right: 0; 
            background-color: #333; border-radius: 8px; 
            box-shadow: 0 4px 12px rgba(0,0,0,0.5); 
            width: 160px; z-index: 1000;
            display: none; flex-direction: column; overflow: hidden;
            border: 1px solid rgba(255,255,255,0.1);
          }
          .wx-dl-dropdown.show { display: flex; }
          .wx-dl-menu-item { padding: 10px 16px; color: #ddd; font-size: 14px; cursor: pointer; transition: background 0.2s; text-decoration: none; display: flex; align-items: center; }
          .wx-dl-menu-item:hover { background-color: rgba(255,255,255,0.1); color: #fff; }
          .wx-dl-menu-item svg { margin-right: 8px; fill: currentColor; }

          .wx-dl-list {
            flex: 1;
            overflow-y: auto;
            position: relative; /* Ensure it contains its children properly */
          }
          .wx-dl-list .weui-cells:before, .wx-dl-list .weui-cells:after { display: none; }
          .wx-dl-list .weui-cell:before { display: none; }
        `;
      document.head.appendChild(style);
    }

    const MoreIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/></svg>`;

    // Container with Header and List
    $download_panel.innerHTML = `
      <div class="wx-dl-panel-container">
        <div class="wx-dl-header">
           <div class="wx-dl-title">Downloads</div>
        </div>
        <div id="downloader_container" class="wx-dl-list wx-dl-dark-scroll weui-cells" style="background-color: transparent; margin-top: 0;"></div>
      </div>
    `;
    var popover$ = WUI.Popover($button, {
      content: $download_panel.innerHTML,
      placement: "bottom-end",
      closeOnClickOutside: true,
    });
    var $more = document.createElement("div");
    $more.innerHTML = `<div class="wx-dl-more-btn" id="wx_dl_more_btn">${MoreIcon}</div>`;
    var moredropdown$ = WUI.DropdownMenu($more, {
      zIndex: 99999,
      children: [
        MenuItem({
          label: "打开目录",
          onClick: async () => {
            await WXU.request({
              method: "POST",
              url: "https://api.channels.qq.com/api/open_download_dir",
            });
            moredropdown$.hide();
          },
        }),
        MenuItem({
          label: "清空记录",
          onClick: async () => {
            await WXU.request({
              method: "POST",
              url: "https://api.channels.qq.com/api/task/clear",
            });
            // moredropdown$.hide();
          },
        }),
      ],
    });
    moredropdown$.ui.$trigger.onMouseEnter(() => {
      moredropdown$.show();
    });
    moredropdown$.ui.$trigger.onMouseLeave(() => {
      if (!moredropdown$.isHover) {
        moredropdown$.hide();
      }
    });
    function mountMoreIntoHeader() {
      var header = document.querySelector(".wx-dl-header");
      if (!header) return;
      if (!document.getElementById("wx_dl_more_btn")) {
        header.appendChild($more);
      }
    }
    $button.addEventListener("mouseenter", () => {
      setTimeout(mountMoreIntoHeader, 0);
    });
    var $header = document.querySelector(".home-header");
    var $box = $header.children[$header.children.length - 1];
    var $btn_wrap = $box.children[0];
    $btn_wrap.insertBefore($button, $btn_wrap.firstChild);

    WXU.downloader.show = function () {
      popover$.open();
      setTimeout(mountMoreIntoHeader, 0);
    };
    WXU.downloader.hide = function () {
      popover$.close();
    };
    WXU.downloader.toggle = function () {
      popover$.toggle();
      setTimeout(mountMoreIntoHeader, 0);
    };
    connect("#downloader_container");
  }

  insert_downloader();
})();

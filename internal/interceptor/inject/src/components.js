const inserted_style = `<style>
:root {
  --popup-bg-color: #f6f6f6;
  --popup-content-bg-color: #e7e7e7;
}
body[data-weui-theme=dark] {
  --popup-bg-color: #272727;
  --popup-content-bg-color: #323232;
}
@media (prefers-color-scheme: dark) {
  body:not([data-weui-theme=light]) {
    --popup-bg-color: #272727;
    --popup-content-bg-color: #323232;
  }
}
.flex {
  display: flex;
}
.custom-menu {
  z-index: 99999;
  background: var(--popup-bg-color);
  box-shadow: 0 0 6px rgb(0 0 0 / 20%);
  border-radius: 4px;
  color: var(--weui-FG-0);
  padding: 8px;
}
.custom-menu-item {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  min-width: 6em;
  transition: background .15s ease-in-out
}
.custom-menu-item:hover {
  background: var(--popup-content-bg-color);
}
.custom-menu-item-arrow {
  position: absolute;
  right: 4px;
  top: 5px;
  font-size: 18px;
  line-height: 12px;
}
.custom-menu .weui-cells {
  margin: 0;
  background: transparent;
}
.custom-menu .weui-cell {
  align-items: center;
  padding: 8px;
  border-radius: 4px;
}
.custom-menu .weui-cell:hover {
  background: var(--FG-6);
}
.custom-menu .weui-cell__bd p {
  color: var(--weui-FG-0);
  font-size: 14px;
  line-height: 1.4;
}
.custom-menu .wx-download-item-open {
  display: none;
  margin-left: 8px;
}
.custom-menu .weui-cell:hover .wx-download-item-open {
  display: inline-flex;
}
.wx-footer {
  position: fixed;
  right: 0;
  bottom: 18px;
  z-index: 99998;
  text-align: center;
  font-size: 14px;
  padding: 4px 48px;
}
.wx-footer-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.wx-sider {
  position: relative;
  position: fixed;
  right: 27px;
  top: 50%;
  z-index: 99998;
  text-align: center;
  font-size: 14px;
}
.wx-sider-bg {
  z-index: 10;
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0.5;
  background-color: var(--BG-0);
}
.wx-sider-tools {
  z-index: 11;
  position: relative;
  color: var(--weui-FG-0);
}
.wx-sider-tools-btn {
  z-index: 11;
  position: relative;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
}
.wx-sider-tools-btn:hover {
  background: var(--weui-BG-COLOR-ACTIVE);
}
.wx-dl-panel-container { 
  width: 400px; 
  max-height: 450px; 
  background-color: var(--popup-bg-color); 
  border-radius: 8px; 
  display: flex; 
  flex-direction: column; 
  padding: 12px; 
  box-sizing: border-box;
  color: var(--weui-FG-0);
  box-shadow: 0 0 6px rgb(0 0 0 / 20%);
}
.wx-dl-dark-scroll::-webkit-scrollbar { width: 6px; }
.wx-dl-dark-scroll::-webkit-scrollbar-track { background: transparent; }
.wx-dl-dark-scroll::-webkit-scrollbar-thumb { background-color: var(--weui-FG-3); border-radius: 3px; }

/* Custom Menu Styles */
.wx-dl-header { display: flex; justify-content: space-between; align-items: center; padding-bottom: 8px; margin-bottom: 4px; flex-shrink: 0; }
.wx-dl-title { font-size: 16px; font-weight: 600; color: var(--weui-FG-0); }
.wx-dl-more-btn { color: var(--weui-FG-0); cursor: pointer; padding: 4px; border-radius: 4px; opacity: 0.8; transition: opacity 0.2s; position: relative; }
.wx-dl-more-btn:hover { opacity: 1; background-color: var(--weui-BG-COLOR-ACTIVE); }

.wx-dl-dropdown { 
  position: absolute; top: 100%; right: 0; 
  background-color: var(--weui-BG-2); border-radius: 8px; 
  box-shadow: 0 0 6px rgb(0 0 0 / 20%);
  width: 160px; z-index: 1000;
  display: none; flex-direction: column; overflow: hidden;
}
.wx-dl-dropdown.show { display: flex; }
.wx-dl-menu-item { padding: 10px 16px; color: var(--weui-FG-0); font-size: 14px; cursor: pointer; transition: background 0.2s; text-decoration: none; display: flex; align-items: center; }
.wx-dl-menu-item:hover { background-color: var(--weui-BG-COLOR-ACTIVE); }
.wx-dl-menu-item svg { margin-right: 8px; fill: currentColor; }

.wx-dl-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  position: relative; /* Ensure it contains its children properly */
}
.wx-dl-list.weui-cells:before, .wx-dl-list.weui-cells:after, .wx-dl-list .weui-cells:before, .wx-dl-list .weui-cells:after { display: none; }
.wx-dl-list .weui-cell:before { display: none; }

.wx-dl-item {
  padding: 16px;
  background-color: var(--popup-content-bg-color);
  border-radius: 8px;
  margin-bottom: 8px;
  align-items: center;
}
</style>`;

function insert_channels_style() {
  document.head.insertAdjacentHTML("beforeend", inserted_style);
}

var download_icon1 = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
var download_icon2 = `<svg class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1567" width="1em" height="1em"><path d="M510.507 161c136.08 0 254.917 86.968 298.77 212.61l0.947 2.772 0.209 0.056c85.945 23.138 148.202 101.3 149.545 193.033L960 572.5c0 112.114-90.482 203-202.098 203-9.832 0-23.115-1.567-39.848-4.7-8.686-1.626-14.409-9.985-12.782-18.67a16 16 0 0 1 0.316-1.361l10.739-38.438c2.221-7.951 10.128-12.907 18.253-11.44 9.632 1.74 17.406 2.609 23.322 2.609 72.028 0 130.418-58.65 130.418-131 0-64.593-46.855-119.203-109.508-129.32l-1.904-0.293-23.48-3.436-6.02-23.055C719.407 309.16 622.489 233 510.507 233c-106.271 0-199.349 68.62-232.37 168.113l-0.982 3.024-6.578 20.706-21.305 3.796C183.982 440.275 135.68 497.59 135.68 565c0 76.492 61.733 138.5 137.884 138.5 4.118 0 9.58-0.435 16.386-1.305 8.178-1.045 15.812 4.296 17.633 12.337l8.838 39.024c1.952 8.618-3.452 17.187-12.07 19.139a16 16 0 0 1-1.327 0.242c-12.264 1.709-22.084 2.563-29.46 2.563C157.824 775.5 64 681.256 64 565c0-94.273 62.11-175.514 149.425-201.7l2.4-0.701 1.368-3.441c47.328-116.945 160.436-196.452 289.156-198.13l4.158-0.028z" fill="currentColor" p-id="1568"></path><path d="M505.744 860.925c3.186 4.1 9.358 4.1 12.544 0l111.502-141.8c4.082-5.2 0.399-12.9-6.272-12.9h-73.77l-1.909 0.003L547.84 464a8 8 0 0 0-8-8h-55.68a8 8 0 0 0-8 8v242.322l-2.075 0.003h-73.571c-6.67 0-10.354 7.7-6.272 12.9z" fill="currentColor" p-id="1569"></path></svg>`;
var download_icon3 = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
var download_icon4 = `<svg class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg"><path d="M512 706.608L781.968 436.64a32 32 0 1 0-45.248-45.256L544 584.096V192a32 32 0 0 0-64 0v392.096l-192.712-192.72a32 32 0 0 0-45.256 45.256L512 706.608z" fill="currentColor"></path><path d="M824 640a32 32 0 0 0-32 32v128.36c0 3.112 0 8.496-0.48 11.472l-1.008 1.024c-0.952 0.984-2.104 2.168-3.112 3.152h-538.48c-2.448-0.664-7.808-3.56-10.608-6.36-2.776-2.784-5.656-8.128-6.32-10.568V672a32 32 0 0 0-64 0v128c0 20.632 12.608 42.456 25.088 54.912C205.584 867.4 227.408 880 248 880h544c22.496 0 36.208-14.112 44.408-22.536l2.48-2.528c17.128-17.088 17.12-41.472 17.12-54.928V672A32.016 32.016 0 0 0 824 640z" fill="currentColor"></path></svg>`;
var FileIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/></svg>`;
var MP3Icon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 13.5c0 1.38-1.12 2.5-2.5 2.5S11 16.88 11 15.5 12.12 13 13.5 13c.57 0 1.08.19 1.5.51V9h3v2h-2v4.5z"/></svg>`;
var MP4Icon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zM10 15V9l5 3-5 3z"/></svg>`;
var ImageIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/></svg>`;
var FolderIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>`;
var ExternalLinkIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M19 19H5V5h7V3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2v-7h-2v7zM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7z"/></svg>`;
var PauseIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>`;
var PlayIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>`;
var RetryIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg>`;
var DeleteIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg>`;
var MoreIcon = `<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/></svg>`;
var RSSIcon = `<svg viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" width="1em" height="1em"><path d="M204.8 938.666667 204.8 938.666667C140.8 938.666667 85.333333 883.2 85.333333 819.2L85.333333 819.2C85.333333 755.2 140.8 699.733333 204.8 699.733333L204.8 699.733333C268.8 699.733333 324.266667 755.2 324.266667 819.2L324.266667 819.2C324.266667 883.2 273.066667 938.666667 204.8 938.666667M85.333333 85.333333 85.333333 213.333333C486.4 213.333333 810.666667 537.6 810.666667 938.666667L938.666667 938.666667C938.666667 467.2 556.8 85.333333 85.333333 85.333333M85.333333 345.6 85.333333 473.6C341.333333 473.6 550.4 682.666667 550.4 938.666667L678.4 938.666667C678.4 610.133333 413.866667 345.6 85.333333 345.6Z" fill="currentColor"></path></svg>`;

/**
 * @returns {HTMLDivElement}
 */
function download_btn1() {
  const icon_download_html = download_icon1;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-6548f11a data-v-132dee25 class="click-box op-item item-gap-combine" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<span data-v-132dee25="" class="text">下载</span></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn2() {
  var icon_download_html = `<div class="op-icon download-icon" style="font-size: 28px;" data-v-5611c716">${download_icon2}</div>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div class=""><div data-v-6548f11a data-v-1fe2ed37 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<div data-v-1fe2ed37 class="op-text">下载</div></div></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn3() {
  var icon_download_html = download_icon3;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-132dee25 class="context-menu__wrp item-gap-combine op-more-btn"><div class="context-menu__target"><div data-v-6548f11a data-v-132dee25 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;"></div>${icon_download_html}</div></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn4() {
  var icon_download_html = download_icon3;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-ecf44def="" class="click-box__btn small" ml-key="live-menu-share"><div data-v-ecf44def="" class="text-[20px]" style="height: 1em;">${icon_download_html}</div></div>`;
  return $icon.firstChild;
}

/**
 * @returns {HTMLDivElement}
 */
function download_btn5() {
  var icon_download_html = download_icon4;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-bf57a568="" class="mr-4 h-6 w-6 flex-initial flex-shrink-0 text-fg-0 cursor-pointer">${icon_download_html}</div>`;
  return $icon.firstChild;
}

/**
 * @param {DropdownMenuItemPayload[]} items
 * @param {{ hide: () => void }} $dropdown
 */
function render_extra_menu_items(items, $dropdown) {
  if (!window.WUI) {
    return [];
  }
  const { MenuItem } = window.WUI;
  return items
    .filter((item) => {
      return item.label && item.onClick;
    })
    .map((item) => {
      return MenuItem({
        label: item.label,
        async onClick(event) {
          await item.onClick(event);
          $dropdown.hide();
        },
      });
    });
}

function format_download_speed(bps) {
  const kb = 1024,
    mb = kb * 1024;
  if (!bps) return "0 B/s";
  if (bps >= mb) return (bps / mb).toFixed(2) + " MB/s";
  if (bps >= kb) return (bps / kb).toFixed(2) + " KB/s";
  return bps + " B/s";
}
function format_download_percent(t) {
  const total = t.meta && t.meta.res ? t.meta.res.size : 0;
  const cur = t.progress ? t.progress.downloaded : 0;
  if (!total) return 0;
  return Math.min(100, Math.floor((cur * 100) / total));
}
function get_name_of_download_task(t) {
  if (t.meta && t.meta.opts && t.meta.opts.name) return t.meta.opts.name;
  if (t.meta && t.meta.res) {
    if (t.meta.res.name) return t.meta.res.name;
    if (t.meta.res.files && t.meta.res.files.length > 0)
      return t.meta.res.files[0].name;
  }
  return "unknown";
}
function total_speed(tasks) {
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

function __wx_refresh_downloader(selector, tasks) {
  const container = document.querySelector(selector);
  if (!container) return;
  container.innerHTML = "";

  const list = Array.from(tasks.values()).reverse(); // Newest first
  const runningCount = list.filter((t) => t.status === "running").length;

  const countEl = document.getElementById("wx-dl-count");
  if (countEl) {
    countEl.innerText = list.length > 0 ? `(${list.length})` : "";
  }

  if (list.length === 0) {
    container.innerHTML = `<div class="weui-loadmore weui-loadmore_line"><span class="weui-loadmore__tips">暂无下载任务</span></div>`;
    return;
  }

  list.forEach((t) => {
    const item = document.createElement("div");
    item.className = "weui-cell";

    const pr = format_download_percent(t);
    const isCompleted =
      t.status === "completed" ||
      t.status === "success" ||
      t.status === "finished" ||
      (pr === 100 && t.status !== "running");

    const isPaused = t.status === "paused" || t.status === "pause";
    const isRunning = t.status === "running";

    let statusText = t.status;
    let progressDisplay = "";
    let statusColor = "var(--FG-1)";

    if (isRunning) {
      const speed = format_download_speed(t.progress ? t.progress.speed : 0);
      statusText = `${speed} • ${pr}%`;
    } else if (isCompleted) {
      statusText = "已完成";
      // Calculate size
      const total = t.meta && t.meta.res ? t.meta.res.size : 0;
      if (total) {
        statusText = WXU.bytes_to_size(total);
      }
    } else if (t.status === "failed" || t.status === "error") {
      statusText = "下载失败";
      statusColor = "#FA5151";
    } else if (t.status === "pending") {
      statusText = "等待中...";
    } else if (isPaused) {
      statusText = `已暂停 • ${pr}%`;
    }

    // Action Buttons Logic
    let actionButtons = "";
    const btnStyle =
      "color: var(--weui-FG-0); opacity: 0.8; margin-left: 12px; cursor: pointer; display: flex; align-items: center; justify-content: center;";

    if (isCompleted) {
      const isOpenExternal = WXU.config.remoteServerEnabled;
      const openIcon = isOpenExternal ? ExternalLinkIcon : FolderIcon;
      const openTitle = isOpenExternal ? "打开外部链接" : "打开文件夹";
      actionButtons += `
             <a href="javascript:" class="wx-download-item-open" aria-label="${openTitle}" title="${openTitle}" data-name="${t.name}" data-path="${t.path}" data-filepath="${t.filepath}" data-id="${t.id}" data-action="open" style="${btnStyle}">
               ${openIcon}
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
      } else if (isPaused || t.status === "failed" || t.status === "error") {
        // Allow resume if paused or failed
        var MaxRunning = WXU.config.downloadMaxRunning;
        if (runningCount < MaxRunning) {
          const isFailed = t.status === "failed" || t.status === "error";
          const label = isFailed ? "重试" : "继续";
          const icon = isFailed ? RetryIcon : PlayIcon;
          actionButtons += `
                <a href="javascript:" class="wx-download-item-resume" aria-label="${label}" title="${label}" data-id="${t.id}" data-action="resume" style="${btnStyle}">
                  ${icon}
                </a>
              `;
        }
      }
      actionButtons += `
             <a href="javascript:" class="wx-download-item-delete" aria-label="删除" title="删除" data-id="${t.id}" data-action="delete" style="${btnStyle}">
               ${DeleteIcon}
             </a>
           `;
    }

    const actionHtml = `<div class="weui-cell__ft" style="display: flex; align-items: center;">${actionButtons}</div>`;

    var filename = get_name_of_download_task(t);

    // Custom dark theme styles inline for specific elements, plus classes
    item.className = "weui-cell wx-dl-item";

    // File Icon size increase
    // We wrap the icon in a slightly larger container
    const iconSize = "50px";

    // Icon preparation
    let selectedIcon = FileIcon;
    if (filename) {
      const ext = filename.split(".").pop().toLowerCase();
      if (ext === "mp3") {
        selectedIcon = MP3Icon;
      } else if (ext === "mp4") {
        selectedIcon = MP4Icon;
      } else if (["jpg", "jpeg", "png", "gif", "webp"].includes(ext)) {
        selectedIcon = ImageIcon;
      }
    }

    let iconInner = selectedIcon
      .replace('width="20"', 'width="32"')
      .replace('height="20"', 'height="32"');

    if (isRunning || isPaused) {
      const radius = 22;
      const circumference = 2 * Math.PI * radius;
      const offset = circumference - (pr / 100) * circumference;
      const strokeColor = isPaused ? "#FBC02D" : "#07C160"; // Wechat Green or Warning Yellow for pause

      iconInner = `
        <div style="position: relative; width: 50px; height: 50px; display: flex; align-items: center; justify-content: center;">
             <svg width="50" height="50" style="position: absolute; top: 0; left: 0; transform: rotate(-90deg);">
                <circle cx="25" cy="25" r="${radius}" stroke="var(--FG-3)" stroke-width="3" fill="none"></circle>
                <circle cx="25" cy="25" r="${radius}" stroke="${strokeColor}" stroke-width="3" fill="none" stroke-dasharray="${circumference}" stroke-dashoffset="${offset}" stroke-linecap="round"></circle>
             </svg>
             <div style="position: relative; z-index: 1; display: flex;">
               ${iconInner}
             </div>
        </div>
        `;
    }

    item.innerHTML = `
          <div class="weui-cell__hd" aria-hidden="true" style="position: relative; margin-right: 16px; width: ${iconSize}; height: ${iconSize}; display: flex; align-items: center; justify-content: center; color: var(--weui-FG-0);">
            ${iconInner}
          </div>
          <div class="weui-cell__bd" style="min-width:0;">
            <p class="weui-ellipsis" style="color: var(--weui-FG-0); font-weight: 500; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" title="${filename}">${filename}</p>
            <div class="weui-cell__desc" style="margin-top: 4px; color: ${statusColor}; font-size: 12px;">${statusText}</div>
            ${
              typeof pr === "number" && !isCompleted
                ? `<div style="height: 4px; background: var(--FG-3); border-radius: 2px; margin-top: 6px; overflow: hidden; display: none;"><div style="width: ${pr}%; background: #07C160; height: 100%; transition: width 0.2s;"></div></div>`
                : ""
            }
            ${progressDisplay}
          </div>
          ${actionHtml}
      `;
    container.appendChild(item);
  });

  if (list.length > 0) {
    const footer = document.createElement("div");
    footer.className = "weui-loadmore weui-loadmore_line";
    footer.style.marginTop = "20px";
    footer.innerHTML =
      '<span class="weui-loadmore__tips">没有更多内容了</span>';
    container.appendChild(footer);
  }
}

var { Menu, MenuItem } = WUI;
MenuItem.setTemplate(
  '<div class="custom-menu-item"><span class="label">{{ label }}</span></div>'
);
MenuItem.setIndicatorTemplate('<span class="custom-menu-item-arrow">›</span>');
Menu.setTemplate('<div><div class="custom-menu">{{ list }}</div></div>');

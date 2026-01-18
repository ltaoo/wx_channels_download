/**
 * @file 下载管理
 */
var __wx_username;
var ua = navigator.userAgent || navigator.platform || "";
var isWin = /Windows|Win/i.test(ua);
(() => {
  const tasks = new Map();
  function upsert(task) {
    if (!task || !task.id) return;
    tasks.set(task.id, {
      ...task,
      ...(() => {
        if (!task.meta.opts) {
          return {};
        }
        var p = task.meta.opts.path || "";
        var n = task.meta.opts.name || "";
        var sep = isWin ? "\\" : "/";
        if (!p || !n) {
          return {};
        }
        return {
          path: p,
          name: n,
          filepath: p.endsWith(sep) ? p + n : p + sep + n,
        };
      })(),
    });
  }
  function connect(selector) {
    return new Promise((resolve, reject) => {
      const protocol = "wss://";
      const pathname = FakeAPIServerAddr;
      const ws = new WebSocket(protocol + pathname + "/ws/channels");

      ws.onopen = () => {
        if (WXU.downloader) {
          WXU.downloader.status = "connected";
        }
        resolve(true);
      };
      ws.onclose = () => {
        WXU.error({ msg: "ws连接已关闭，请刷新页面" });
        if (WXU.downloader) {
          WXU.downloader.status = "disconnected";
        }
      };
      ws.onerror = (e) => {
        if (WXU.downloader && WXU.downloader.status !== "connected") {
          reject(e);
        }
      };
      ws.onmessage = (ev) => {
        const [err, msg] = WXU.parseJSON(ev.data);
        if (err) {
          return;
        }
        if (msg.type === "tasks") {
          if (Array.isArray(msg.data)) {
            msg.data.forEach(upsert);
          }
          __wx_refresh_downloader(selector, tasks);
          return;
        }
        if (msg.type === "clear") {
          tasks.clear();
          __wx_refresh_downloader(selector, tasks);
          return;
        }
        if (msg.type === "event") {
          const evt = msg && msg.data ? msg.data : null;
          const task = evt ? evt.Task || evt.task : null; // 兼容大小写字段
          if (task) {
            if (evt.Type === "delete") {
              tasks.delete(task.id);
            } else {
              upsert(task);
            }
          }
          __wx_refresh_downloader(selector, tasks);
          return;
        }
        if (msg.type === "api_call") {
          __wx_handle_api_call(msg.data, ws);
        }
      };
    });
  }

  document.addEventListener("click", async (e) => {
    if (e.target && e.target.classList.contains("start-btn")) {
      const id = e.target.getAttribute("data-id");
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/start",
        body: { id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
    }
    const $task_action_btn = e.target.closest("[data-action]");
    if ($task_action_btn) {
      e.stopPropagation();
      const action = $task_action_btn.getAttribute("data-action");
      const id = $task_action_btn.getAttribute("data-id");
      if (action === "open") {
        const path = $task_action_btn.getAttribute("data-path");
        const name = $task_action_btn.getAttribute("data-name");
        if (!path || !name) {
          WXU.error({
            msg: "path or name is empty",
          });
          return;
        }
        if (WXU.config.remoteServerEnabled) {
          var u =
            WXU.config.remoteServerProtocol +
            "://" +
            WXU.config.remoteServerHostname;
          if (WXU.config.remoteServerPort !== 80) {
            u += ":" + WXU.config.remoteServerPort;
          }
          u += "/video?id=" + id;
          window.open(u);
        } else {
          // Use original API for local file
          var [err, data] = await WXU.request({
            method: "POST",
            url: "https://" + FakeAPIServerAddr + "/api/show_file",
            body: { path, name, id },
          });
          if (err) {
            WXU.error({
              msg: err.message,
            });
          }
        }
        return;
      }
      let url = "https://" + FakeAPIServerAddr + "/api/task/pause";
      if (action === "resume") {
        url = "https://" + FakeAPIServerAddr + "/api/task/resume";
      } else if (action === "delete") {
        url = "https://" + FakeAPIServerAddr + "/api/task/delete";
      }
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
  });

  var mounted = false;
  function insert_downloader() {
    var $header = document.querySelector(".home-header");
    console.log("[DOWNLOADER]insert_downloader", mounted, $header);
    if (mounted) {
      return;
    }
    if (!$header) return;
    var $box = $header.children[$header.children.length - 1];
    if (!$box) return;
    var $btn_wrap = $box.children[0];
    if (!$btn_wrap) return;
    var $download_panel_button = download_btn5();
    var $download_panel = document.createElement("div");
    $download_panel.innerHTML = `
      <div class="wx-dl-panel-container">
        <div class="wx-dl-header">
           <div class="wx-dl-title">Downloads <span id="wx-dl-count"></span></div>
        </div>
        <div id="downloader_container" class="wx-dl-list wx-dl-dark-scroll" style="background-color: transparent; margin-top: 0;"></div>
      </div>
    `;
    var download_popover$ = WUI.Popover($download_panel_button, {
      content: $download_panel.innerHTML,
      placement: "bottom-end",
      closeOnClickOutside: true,
      offset: { mainAxis: -4, crossAxis: 20 },
    });
    var $more = document.createElement("div");
    $more.innerHTML = `<div class="wx-dl-more-btn" id="wx_dl_more_btn">${MoreIcon}</div>`;
    var moredropdown$ = WUI.DropdownMenu($more, {
      zIndex: 99999,
      children: [
        !WXU.config.remoteServerEnabled
          ? WUI.MenuItem({
              label: "打开目录",
              onClick: async () => {
                await WXU.request({
                  method: "POST",
                  url:
                    "https://" + FakeAPIServerAddr + "/api/open_download_dir",
                });
                moredropdown$.hide();
              },
            })
          : null,
        WUI.MenuItem({
          label: "清空记录",
          onClick: async () => {
            moredropdown$.hide();
            await WXU.request({
              method: "POST",
              url: "https://" + FakeAPIServerAddr + "/api/task/clear",
            });
          },
        }),
      ].filter(Boolean),
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
    $download_panel_button.addEventListener("mouseenter", () => {
      setTimeout(mountMoreIntoHeader, 0);
    });

    $btn_wrap.insertBefore($download_panel_button, $btn_wrap.firstChild);
    mounted = true;

    WXU.downloader.show = function () {
      download_popover$.open();
    };
    WXU.downloader.hide = function () {
      download_popover$.close();
    };
    WXU.downloader.toggle = function () {
      download_popover$.toggle();
    };

    WXU.downloader.status = "disconnected";
    WXU.downloader.reconnect = async function () {
      if (WXU.downloader.status === "connected") return true;
      const selector = "#downloader_container";
      for (let i = 0; i < 3; i++) {
        try {
          await connect(selector);
          return true;
        } catch (e) {
          console.warn("Reconnect attempt " + (i + 1) + " failed");
          await new Promise((r) => setTimeout(r, 1000));
        }
      }
      return false;
    };
    connect("#downloader_container").catch((e) =>
      WXU.error({ msg: "建立ws连接失败" })
    );
  }
  WXU.observe_node(".home-header", () => {
    insert_downloader();
  });
})();

async function __wx_handle_api_call(msg, socket) {
  var { id, key, data } = msg;
  console.log("[DOWNLOADER]__wx_handle_api_call", id, key, data);
  function resp(body) {
    socket.send(
      JSON.stringify({
        id,
        data: body,
      })
    );
  }
  if (key === "key:channels:contact_list") {
    var payload = {
      query: data.keyword,
      scene: 13,
      requestId: String(new Date().valueOf()),
    };
    var r = await WXU.API2.finderSearch(payload);
    console.log("[DOWNLOADER]finderSearch", r);
    /** @type {SearchResp} */
    var { infoList, objectList } = r.data;
    resp({
      ...r,
      payload,
    });
    return;
  }
  if (key === "key:channels:feed_list") {
    var payload = {
      username: data.username,
      finderUsername: __wx_username,
      lastBuffer: data.next_marker ? decodeURIComponent(data.next_marker) : "",
      needFansCount: 0,
      objectId: "0",
    };
    var r = await WXU.API.finderUserPage(payload);
    console.log("[DOWNLOADER]finderUserPage", r);
    /** @type {ChannelsObject[]} */
    const object = r.data.object || [];
    resp({
      ...r,
      payload,
    });
    return;
  }
  if (key === "key:channels:feed_profile") {
    console.log("before finderGetCommentProfile", data.oid, data.nid);
    try {
      if (data.url) {
        var u = new URL(decodeURIComponent(data.url));
        data.oid = WXU.API.decodeBase64ToUint64String(
          u.searchParams.get("oid")
        );
        data.nid = WXU.API.decodeBase64ToUint64String(
          u.searchParams.get("nid")
        );
      }
      var payload = {
        needObject: 1,
        lastBuffer: "",
        scene: 146,
        direction: 2,
        identityScene: 2,
        pullScene: 6,
        objectid: (() => {
          if (data.oid.includes("_")) {
            return data.oid.split("_")[0];
          }
          return data.oid;
        })(),
        objectNonceId: data.nid,
        encrypted_objectid: "",
      };
      var r = await WXU.API.finderGetCommentDetail(payload);
      /** @type {MediaProfileResp} */
      var { object } = r.data;
      resp({
        ...r,
        payload,
      });
      return;
    } catch (err) {
      resp({
        errCode: 1011,
        errMsg: err.message,
        payload,
      });
      return;
    }
  }
  resp({
    errCode: 1000,
    errMsg: "未匹配的key",
    payload: msg,
  });
}

WXU.onInit((data) => {
  __wx_username = data.mainFinderUsername;
});

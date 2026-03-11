/**
 * @file 下载管理
 */
var __wx_username;
var ua = navigator.userAgent || navigator.platform || "";
var isWin = /Windows|Win/i.test(ua);
(() => {
  const tasks = new Map();
  let ws_conn = null;
  let cur_page = 1;
  let is_loading = false;
  let has_more = true;
  let total_count = 0;

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
  function connect_local_ws() {
    const ws = new WebSocket(
      LocalApiServerProtocol + "://" + FakeLocalAPIServerAddr + "/ws/channels",
    );
    ws.onclose = (e) => {
      WXU.error({ msg: "本地ws连接已关闭，" + JSON.stringify(e) });
    };
    ws.onerror = (e) => {
      WXU.error({ msg: "本地ws连接发生错误，" + JSON.stringify(e) });
    };
    ws.onmessage = (ev) => {
      const [err, msg] = WXU.parseJSON(ev.data);
      if (err) {
        return;
      }
      if (msg.type === "api_call") {
        __wx_handle_api_call(msg.data, ws);
      }
    };
  }
  function connect(selector) {
    console.log("[]download connect websocket", FakeAPIServerAddr);
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(
        APIWSServerProtocol + "://" + FakeAPIServerAddr + "/ws/channels",
      );
      ws_conn = ws;

      ws.onopen = () => {
        if (WXU.downloader) {
          WXU.downloader.status = "connected";
        }
        cur_page = 1;
        has_more = true;
        is_loading = false;
        resolve(true);
      };
      ws.onclose = (e) => {
        WXU.error({ msg: "ws连接已关闭，请刷新页面，Code: " + e.code + ", Reason: " + e.reason + ", Clean: " + e.wasClean });
        if (WXU.downloader) {
          WXU.downloader.status = "disconnected";
        }
        ws_conn = null;
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
        console.log("[]remote ws event", msg);
        if (msg.type === "tasks") {
          let list = msg.data;
          if (msg.data && msg.data.list) {
            list = msg.data.list;
            total_count = msg.data.total;
          }
          if (Array.isArray(list)) {
            list.reverse().forEach(upsert);
            if (list.length < 50) {
              has_more = false;
            }
          } else {
            has_more = false;
          }
          if (is_loading) {
            is_loading = false;
            cur_page++;
          }
          __wx_refresh_downloader(selector, tasks, total_count);
          return;
        }
        if (msg.type === "clear") {
          tasks.clear();
          total_count = 0;
          __wx_refresh_downloader(selector, tasks, total_count);
          return;
        }
        if (msg.type === "event") {
          const evt = msg && msg.data ? msg.data : null;
          const task = evt ? evt.Task || evt.task : null; // 兼容大小写字段
          if (task) {
            if (!tasks.has(task.id)) {
              total_count++;
            }
            upsert(task);
          }
          __wx_refresh_downloader(selector, tasks, total_count);
          return;
        }
        if (msg.type === "api_call") {
          __wx_handle_api_call(msg.data, ws);
        }
      };
      if (WXU.config.remoteServerEnabled) {
        // 额外再连接本地ws用于API调用
        connect_local_ws();
      }
    });
  }

  document.addEventListener("click", async (e) => {
    if (e.target && e.target.classList.contains("start-btn")) {
      const id = e.target.getAttribute("data-id");
      var [err, data] = await WXU.request({
        method: "POST",
        url: APIServerProtocol + "://" + FakeAPIServerAddr + "/api/task/start",
        body: { id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      const t = tasks.get(id);
      if (t) {
        tasks.set(id, { ...t, status: "running" });
        __wx_refresh_downloader("#downloader_container", tasks, total_count);
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
          var u = APIServerProtocol + "://" + FakeAPIServerAddr + "/preview?id=" + id;
          window.open(u);
        } else {
          // Use original API for local file
          var [err, data] = await WXU.request({
            method: "POST",
            url: APIServerProtocol + "://" + FakeAPIServerAddr + "/api/show_file",
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
      let url = APIServerProtocol + "://" + FakeAPIServerAddr + "/api/task/pause";
      if (action === "resume") {
        url = APIServerProtocol + "://" + FakeAPIServerAddr + "/api/task/resume";
      } else if (action === "delete") {
        url = APIServerProtocol + "://" + FakeAPIServerAddr + "/api/task/delete";
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
      if (action === "delete") {
        tasks.delete(id);
        if (total_count > 0) total_count--;
        __wx_refresh_downloader("#downloader_container", tasks, total_count);
      } else if (action === "pause") {
        const t = tasks.get(id);
        if (t) {
          tasks.set(id, { ...t, status: "paused" });
          __wx_refresh_downloader("#downloader_container", tasks, total_count);
        }
      } else if (action === "resume") {
        const t = tasks.get(id);
        if (t) {
          tasks.set(id, { ...t, status: "running" });
          __wx_refresh_downloader("#downloader_container", tasks, total_count);
        }
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
                  url: APIServerProtocol + "://" + FakeAPIServerAddr + "/api/open_download_dir",
                });
                moredropdown$.hide();
              },
            })
          : null,
        WUI.MenuItem({
          label: "清空记录",
          onClick: async () => {
            moredropdown$.hide();

            // methods
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
    connect("#downloader_container").catch((e) => {
      WXU.error({ msg: "建立ws连接失败，" + JSON.stringify(e) });
    });
  }
  WXU.observe_node(".home-header", () => {
    insert_downloader();
  });
<<<<<<< Updated upstream
  // console.log("[]check is wxwork", window.ua.includes("wxwork"), window.ua);
  if (WXU.config.remoteServerEnabled) {
=======
  if (WXU.env.isWxwork || WXU.config.remoteServerEnabled) {
>>>>>>> Stashed changes
    connect_local_ws();
  }

  // document.addEventListener(
  //   "scroll",
  //   (e) => {
  //     if (e.target && e.target.id === "downloader_container") {
  //       const el = e.target;
  //       if (el.scrollTop + el.clientHeight >= el.scrollHeight - 50) {
  //         if (
  //           !is_loading &&
  //           has_more &&
  //           ws_conn &&
  //           ws_conn.readyState === WebSocket.OPEN
  //         ) {
  //           is_loading = true;
  //           console.log("[]fetching next page", cur_page + 1);
  //           ws_conn.send(
  //             JSON.stringify({
  //               type: "fetch_tasks",
  //               page: cur_page + 1,
  //               limit: 50,
  //             }),
  //           );
  //         }
  //       }
  //     }
  //   },
  //   true,
  // );
})();

async function __wx_handle_api_call(msg, socket) {
  var { id, key, data } = msg;
  console.log("[DOWNLOADER]__wx_handle_api_call", id, key, data);
  function resp(body) {
    socket.send(
      JSON.stringify({
        id,
        data: body,
      }),
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
  if (key === "key:channels:live_replay_list") {
    var payload = {
      username: data.username,
      finderUsername: __wx_username || data.username,
      lastBuffer: data.next_marker ? decodeURIComponent(data.next_marker) : "",
      needFansCount: 0,
      objectId: "0",
    };
    var r = await WXU.API3.finderLiveUserPage(payload);
    console.log("[DOWNLOADER]finderLiveUserPage", r);
    resp({
      ...r,
      payload,
    });
    return;
  }
  if (key === "key:channels:interactioned_list") {
    var payload = {
      lastBuffer: data.next_marker ? decodeURIComponent(data.next_marker) : "",
      tabFlag: data.flag ? Number(data.flag) : 7,
    };
    var r = await WXU.API4.finderGetInteractionedFeedList(payload);
    console.log("[DOWNLOADER]finderGetInteractionedFeedList", r);
    resp({
      ...r,
      payload,
    });
    return;
  }
  if (key === "key:channels:feed_profile") {
    console.log("before finderGetCommentProfile", data.oid, data.nid, data.eid);
    try {
      if (data.url) {
        var u = new URL(decodeURIComponent(data.url));
        data.oid = WXU.API.decodeBase64ToUint64String(
          u.searchParams.get("oid"),
        );
        data.nid = WXU.API.decodeBase64ToUint64String(
          u.searchParams.get("nid"),
        );
      }
      var payload = {
        needObject: 1,
        lastBuffer: "",
        scene: data.eid ? 141 : 146,
        direction: 2,
        identityScene: 2,
        pullScene: 6,
        objectid: (() => {
          if (data.eid) {
            return undefined;
          }
          if (data.oid.includes("_")) {
            return data.oid.split("_")[0];
          }
          return data.oid;
        })(),
        objectNonceId: data.eid ? undefined : data.nid,
        encrypted_objectid: data.eid || "",
      };
      if (data.eid) {
        payload.traceBuffer = undefined;
      }
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
  if (key === "key:channels:reload") {
    console.log("[DOWNLOADER]reloading page");
    resp({
      msg: "reloading",
    });
    setTimeout(() => {
      window.location.reload();
    }, 500);
    return;
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

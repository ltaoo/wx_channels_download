/**
 * @file 下载管理面板2
 */
var __wx_username;
var ua = navigator.userAgent || navigator.platform || "";
var isWin = /Windows|Win/i.test(ua);

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
    console.log("before finderGetCommentProfile", data.oid, data.nid);
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

function GopeedDownloaderPanel() {
  const tasks = [];
  const tasks_ = ref(tasks);
  const task_count = BoxUI.computed({ tasks: tasks_ }, (draft) => {
    return draft.tasks.length;
  });

  const methods = {
    upsert(task) {
      if (!task || !task.id) return;
      const matched = tasks.find((t) => t.id === task.id);
      if (!matched) {
        return;
      }
      tasks[tasks.indexOf(matched)] = {
        ...matched,
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
      };
    },
    connect() {
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
            tasks_.value = msg.data;
            return;
          }
          if (msg.type === "clear") {
            tasks.clear();
            // __wx_refresh_downloader(selector, tasks);
            return;
          }
          if (msg.type === "event") {
            const evt = msg && msg.data ? msg.data : null;
            const task = evt ? evt.Task || evt.task : null; // 兼容大小写字段
            if (task) {
              methods.upsert(task);
            }
            // __wx_refresh_downloader(selector, tasks);
            return;
          }
          if (msg.type === "api_call") {
            __wx_handle_api_call(msg.data, ws);
          }
        };
      });
    },
    async start() {
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
      const t = tasks.get(id);
      if (t) {
        tasks.set(id, { ...t, status: "running" });
        // __wx_refresh_downloader("#downloader_container", tasks);
      }
    },
    async open(task) {
      const { path, name } = task;
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
        u += "/preview?id=" + id;
        window.open(u);
        return;
      }
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
    },
    async pause() {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/pause",
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
        tasks.set(id, { ...t, status: "paused" });
        __wx_refresh_downloader("#downloader_container", tasks);
      }
    },
    async delete() {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/delete",
        body: { id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      tasks.delete(id);
      __wx_refresh_downloader("#downloader_container", tasks);
    },
    async resume() {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/resume",
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
        __wx_refresh_downloader("#downloader_container", tasks);
      }
    },
  };

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
    for (let i = 0; i < 3; i++) {
      try {
        await methods.connect();
        return true;
      } catch (e) {
        console.warn("Reconnect attempt " + (i + 1) + " failed");
        await new Promise((r) => setTimeout(r, 1000));
      }
    }
    return false;
  };
  methods.connect().catch((e) => WXU.error({ msg: "建立ws连接失败" }));

  const more$ = BoxUI.View({ class: "wx-dl-more-btn" }, [
    BoxUI.DangerouslyInnerHTML(MoreIcon),
  ]);
  const panel$ = BoxUI.View({ class: "wx-dl-panel-container" }, [
    BoxUI.View({ class: "wx-dl-header" }, [
      BoxUI.View({}, [
        BoxUI.Txt("Downloads"),
        BoxUI.View({}, BoxUI.Txt(task_count)),
      ]),
      more$,
    ]),
    BoxUI.View(
      {
        class: "wx-dl-list wx-dl-dark-scroll",
        style: "background-color: transparent; margin-top: 0;",
      },
      [],
    ),
  ]);
  var $download_panel_button = download_btn5();
  const $download_panel = panel$.$elm;
  var download_popover$ = WUI.Popover($download_panel_button, {
    content: $download_panel.innerHTML,
    placement: "bottom-end",
    closeOnClickOutside: true,
    offset: { mainAxis: -4, crossAxis: 20 },
  });
  var moredropdown$ = WUI.DropdownMenu(more$.$elm, {
    zIndex: 99999,
    children: [
      !WXU.config.remoteServerEnabled
        ? WUI.MenuItem({
            label: "打开目录",
            onClick: async () => {
              await WXU.request({
                method: "POST",
                url: "https://" + FakeAPIServerAddr + "/api/open_download_dir",
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
          tasks.clear();
          __wx_refresh_downloader("#downloader_container", tasks);
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

  return $download_panel_button;
}

(() => {
  // document.addEventListener("click", async (e) => {
  //   if (e.target && e.target.classList.contains("start-btn")) {
  //   }
  //   const $task_action_btn = e.target.closest("[data-action]");
  //   if ($task_action_btn) {
  //     e.stopPropagation();
  //   }
  // });

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

    var $panel_button = GopeedDownloaderPanel();
    $btn_wrap.insertBefore($panel_button, $btn_wrap.firstChild);
    mounted = true;
  }
  WXU.observe_node(".home-header", () => {
    insert_downloader();
  });
})();

WXU.onInit((data) => {
  __wx_username = data.mainFinderUsername;
});

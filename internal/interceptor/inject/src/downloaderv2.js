/**
 * @file 下载管理面板2
 */
var __wx_username;
var ua = navigator.userAgent || navigator.platform || "";
var isWin = /Windows|Win/i.test(ua);

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
  const task_count_ = computed(tasks_, (t) => {
    return t.length;
  });
  const runningCount_ = computed(tasks_, (t) => {
    return t.filter((v) => v.status === "running").length;
  });
  const methods = {
    upsert(task) {
      if (!task || !task.id) {
        return;
      }
      const matched = tasks_.find((v) => v.id === task.id);
      if (!matched) {
        return;
      }
      matched.as({
        ...matched.value,
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
    async start(task) {
      const id = task.id;
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/start",
        body: { id: task.id },
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
        u += "/preview?id=" + task.id;
        window.open(u);
        return;
      }
      // Use original API for local file
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/show_file",
        body: { path, name, id: task.id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
      }
    },
    async pause(task) {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/pause",
        body: { id: task.id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      const t = tasks.get(task.id);
      if (t) {
        tasks.set(task.id, { ...t, status: "paused" });
        __wx_refresh_downloader("#downloader_container", tasks);
      }
    },
    async delete(task) {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/delete",
        body: { id: task.id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      tasks.delete(task.id);
      __wx_refresh_downloader("#downloader_container", tasks);
    },
    async resume(task) {
      var [err, data] = await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/resume",
        body: { id: task.id },
      });
      if (err) {
        WXU.error({
          msg: err.message,
        });
        return;
      }
      const t = tasks.get(task.id);
      if (t) {
        tasks.set(task.id, { ...t, status: "running" });
        __wx_refresh_downloader("#downloader_container", tasks);
      }
    },
    async clear() {
      await WXU.request({
        method: "POST",
        url: "https://" + FakeAPIServerAddr + "/api/task/clear",
      });
      tasks.clear();
      __wx_refresh_downloader("#downloader_container", tasks);
    },
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

  return View({ class: "wx-dl-panel-container" }, [
    View({ class: "wx-dl-header" }, [
      View({ class: "wx-dl-title" }, [
        Txt("Downloads"),
        View({ type: "span" }, [
          Show(
            {
              when: computed(task_count_, (d) => {
                return d > 0;
              }),
            },
            [
              Txt(
                computed(task_count_, (d) => {
                  return d > 0 ? `(${d})` : "";
                }),
              ),
            ],
          ),
        ]),
      ]),
      View(
        {
          class: "wx-dl-more-btn",
        },
        [DangerouslyInnerHTML(MoreIcon)],
      ),
    ]),
    View(
      {
        class: "wx-dl-list wx-dl-dark-scroll",
        style: "background-color: transparent; margin-top: 0;",
      },
      [
        Show(
          {
            when: computed(task_count_, (d) => {
              return d > 0;
            }),
            fallback: [
              View(
                {
                  class: "weui-loadmore weui-loadmore_line",
                },
                [
                  View(
                    {
                      class: "weui-loadmore__tips",
                    },
                    [Txt("暂无下载任务")],
                  ),
                ],
              ),
            ],
          },
          [
            For({
              each: tasks_,
              render(task) {
                const iconSize = "50px";
                const state = computed(task, (t) => {
                  // console.log("the task is changed", t.status);
                  const pr = format_download_percent(t);
                  const isCompleted =
                    t.status === "completed" ||
                    t.status === "success" ||
                    t.status === "finished" ||
                    (pr === 100 && t.status !== "running");

                  const isPaused =
                    t.status === "paused" || t.status === "pause";
                  const isRunning = t.status === "running";

                  let statusText = t.status;
                  let statusColor = "var(--FG-1)";

                  if (isRunning) {
                    const speed = format_download_speed(
                      t.progress ? t.progress.speed : 0,
                    );
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
                  return {
                    pr,
                    isCompleted,
                    isPaused,
                    isRunning,
                    isFailed: t.status === "failed" || t.status === "error",
                    statusText,
                    statusColor,
                  };
                });
                const isOpenExternal = WXU.config.remoteServerEnabled;
                const filename = computed(task, (t) => {
                  return get_name_of_download_task(t);
                });
                const PrefixIcon = computed(filename, (t) => {
                  const filename = t;
                  let selectedIcon = FileIcon;
                  if (filename) {
                    const ext = filename.split(".").pop().toLowerCase();
                    if (ext === "mp3") {
                      selectedIcon = MP3Icon;
                    } else if (ext === "mp4") {
                      selectedIcon = MP4Icon;
                    } else if (
                      ["jpg", "jpeg", "png", "gif", "webp"].includes(ext)
                    ) {
                      selectedIcon = ImageIcon;
                    }
                  }
                  return selectedIcon
                    .replace('width="20"', 'width="32"')
                    .replace('height="20"', 'height="32"');
                });
                const radius = 22;
                const circumference = 2 * Math.PI * radius;
                const offset = computed(state, (d) => {
                  return circumference - (d.pr / 100) * circumference;
                });
                const strokeColor = computed(state, (d) => {
                  return d.isPaused ? "#FBC02D" : "#07C160";
                });

                return View({ class: "weui-cell wx-dl-item" }, [
                  View(
                    {
                      class: "weui-cell__hd",
                      style: `position: relative; margin-right: 16px; width: ${iconSize}; height: ${iconSize}; display: flex; align-items: center; justify-content: center; color: var(--weui-FG-0);`,
                    },
                    [
                      Show(
                        {
                          when: computed(state, (t) => {
                            return t.isRunning || t.isPaused;
                          }),
                          fallback: [DangerouslyInnerHTML(PrefixIcon.value)],
                        },
                        [
                          View(
                            {
                              style:
                                "position: relative; width: 50px; height: 50px; display: flex; align-items: center; justify-content: center;",
                            },
                            [
                              SVG(
                                {
                                  style:
                                    "position: absolute; top: 0; left: 0; transform: rotate(-90deg);",
                                  width: "50",
                                  height: "50",
                                  viewBox: "0 0 50 50",
                                },
                                [
                                  Circle({
                                    cx: "25",
                                    cy: "25",
                                    r: radius,
                                    stroke: "var(--FG-3)",
                                    "stroke-width": "3",
                                    fill: "none",
                                  }),
                                  Circle({
                                    cx: "25",
                                    cy: "25",
                                    r: radius,
                                    stroke: strokeColor,
                                    "stroke-width": "3",
                                    fill: "none",
                                    "stroke-dasharray": circumference,
                                    "stroke-dashoffset": offset,
                                    "stroke-linecap": "round",
                                  }),
                                ],
                              ),
                              View(
                                {
                                  style:
                                    "position: relative; z-index: 1; display: flex;",
                                },
                                [DangerouslyInnerHTML(PrefixIcon.value)],
                              ),
                            ],
                          ),
                        ],
                      ),
                    ],
                  ),
                  View(
                    {
                      class: "weui-cell__bd",
                      style: "min-width:0;",
                    },
                    [
                      View(
                        {
                          class: "weui-ellipsis",
                          style:
                            "color: var(--weui-FG-0); font-weight: 500; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;",
                        },
                        [Txt(filename)],
                      ),
                      View(
                        {
                          class: "weui-cell__desc",
                          style: computed(state, (d) => {
                            return `margin-top: 4px; color: ${d.statusColor}; font-size: 12px;`;
                          }),
                        },
                        [
                          Txt(
                            computed(state, (d) => {
                              return d.statusText;
                            }),
                          ),
                        ],
                      ),
                    ],
                  ),
                  View(
                    {
                      class: "weui-cell__ft",
                      style: "display: flex; align-items: center;",
                    },
                    (() => {
                      const btnStyle =
                        "color: var(--weui-FG-0); opacity: 0.8; margin-left: 12px; cursor: pointer; display: flex; align-items: center; justify-content: center;";
                      return [
                        Switch({}, [
                          // 场景 1: 已完成 -> 显示打开按钮
                          Match(
                            { when: computed(state, (s) => s.isCompleted) },
                            [
                              View(
                                {
                                  type: "a",
                                  class: "wx-download-item-open",
                                  style: btnStyle,
                                },
                                [
                                  Show(
                                    {
                                      when: isOpenExternal,
                                      fallback: [
                                        DangerouslyInnerHTML(FolderIcon),
                                      ],
                                    },
                                    [DangerouslyInnerHTML(ExternalLinkIcon)],
                                  ),
                                ],
                              ),
                            ],
                          ),
                          // 场景 2: 正在运行 -> 显示暂停按钮
                          Match({ when: computed(state, (t) => t.isRunning) }, [
                            View(
                              {
                                type: "a",
                                class: "wx-download-item-pause",
                                style: btnStyle,
                                onClick() {
                                  methods.pause(task);
                                },
                              },
                              [DangerouslyInnerHTML(PauseIcon)],
                            ),
                          ]),
                          // 场景 3: 暂停或失败且未达最大并发 -> 显示恢复按钮
                          Match(
                            {
                              when: combine([state, runningCount_], (t, c) => {
                                // console.log('the state is change', runningCount.value);
                                return (
                                  (t.isPaused || t.isFailed) &&
                                  c < WXU.config.MaxRunning
                                );
                              }),
                            },
                            [
                              View(
                                {
                                  type: "a",
                                  class: "wx-download-item-resume",
                                  style: btnStyle,
                                  onClick() {
                                    methods.resume(task);
                                  },
                                },
                                [
                                  Show(
                                    {
                                      when: computed(state, (t) => t.isFailed),
                                      fallback: [
                                        DangerouslyInnerHTML(PlayIcon),
                                      ],
                                    },
                                    [DangerouslyInnerHTML(RetryIcon)],
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ]),
                        View(
                          {
                            type: "a",
                            class: "wx-download-item-delete",
                            style: btnStyle,
                          },
                          [DangerouslyInnerHTML(DeleteIcon)],
                        ),
                      ];
                    })(),
                  ),
                ]);
              },
            }),
          ],
        ),
      ],
    ),
  ]);
}

(() => {
  var mounted = false;
  function insert_download_panel() {
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
    WXU.downloader.show = function () {
      // download_popover$.open();
    };
    WXU.downloader.hide = function () {
      // download_popover$.close();
    };
    WXU.downloader.toggle = function () {
      // download_popover$.toggle();
    };
    $btn_wrap.insertBefore($panel_button, $btn_wrap.firstChild);
    mounted = true;
  }
  WXU.observe_node(".home-header", () => {
    insert_download_panel();
  });
})();

WXU.onInit((data) => {
  __wx_username = data.mainFinderUsername;
});

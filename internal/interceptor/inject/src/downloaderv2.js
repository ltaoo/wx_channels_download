/// <reference path="utils.js" />
/**
 * @file 下载管理面板2
 */
var ua = navigator.userAgent || navigator.platform || "";
var isWin = /Windows|Win/i.test(ua);

var APIHostname = APIServerProtocol + "://" + FakeAPIServerAddr;
var RemoteAPIHostname =
  WXU.config.remoteServerProtocol + "://" + WXU.config.remoteServerHostname;
if (WXU.config.remoteServerPort !== 80) {
  RemoteAPIHostname += ":" + WXU.config.remoteServerPort;
}

console.log("[]downloaderv2.js - ", FakeAPIServerAddr, APIHostname);

const http_client = new Timeless.HttpClientCore({
  headers: { "Content-Type": "application/json" },
  hostname: APIHostname,
});
Timeless.web.provide_http_client(http_client);
const request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(r) {
    if (r.error) {
      return Timeless.Result.Err(r.error);
    }
    const { code, msg, data } = r.data;
    if (code !== 0) {
      return Timeless.Result.Err(msg, code, data);
    }
    return Timeless.Result.Ok(data);
  },
});

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
  if (t.meta && t.meta.opts && t.meta.opts.name) {
    return t.meta.opts.name;
  }
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

function DownloaderPanelViewModel() {
  const ITEM_HEIGHT = 82;
  const GUTTER = 8;
  const _pageSize = 50;

  const taskListReq = new Timeless.kit.RequestCore(
    (params) => request.get("/api/task/list", params),
    {
      client: http_client,
      process(r) {
        if (r.error) {
          return r.error;
        }
        return Timeless.Result.Ok({
          list: (r.data.list || []).map((t) => methods.formatTask(t)),
          total: r.data.total || 0,
          page: r.data.page || 1,
          pageSize: r.data.page_size || 50,
        });
      },
    },
  );
  const deleteReq = new Timeless.kit.RequestCore(
    (id) => request.post("/api/task/delete", { id }),
    { client: http_client },
  );
  const startReq = new Timeless.kit.RequestCore(
    (id) => request.post("/api/task/start", { id }),
    { client: http_client },
  );
  const pauseReq = new Timeless.kit.RequestCore(
    (id) => request.post("/api/task/pause", { id }),
    { client: http_client },
  );
  const resumeReq = new Timeless.kit.RequestCore(
    (id) => request.post("/api/task/resume", { id }),
    { client: http_client },
  );
  const clearReq = new Timeless.kit.RequestCore(
    () => request.post("/api/task/clear"),
    { client: http_client },
  );
  const showFileReq = new Timeless.kit.RequestCore(
    ({ path, name, id }) => request.post("/api/show_file", { path, name, id }),
    { client: http_client },
  );
  const list$ = new Timeless.kit.ListCore(taskListReq, {
    pageSize: _pageSize,
  });

  const tasks_ = refarr([]);
  const task_count_ = ref(0);
  const running_count_ = computed(tasks_, (t) => {
    return t.filter((v) => v.status === "running").length;
  });

  const methods = {
    formatTask(task) {
      const r = {
        height: ITEM_HEIGHT,
        ...task,
        ...(() => {
          if (!task.meta || !task.meta.opts) {
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
      return r;
    },
    async startTask(task) {
      const r = await startReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      list$.modifyItem((t) =>
        t.id === task.id ? { ...t, status: "running" } : t,
      );
      const matched = tasks_.find((t) => t.id === task.id);
      if (!matched) {
        return;
      }
      matched.assign({
        status: "running",
      });
    },
    async pauseTask(task) {
      const r = await pauseReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      list$.modifyItem((t) =>
        t.id === task.id ? { ...t, status: "paused" } : t,
      );
      const matched = tasks_.find((t) => t.id === task.id);
      if (!matched) {
        return;
      }
      matched.assign({
        status: "paused",
      });
    },
    async deleteTask(task) {
      const r = await deleteReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const matched = tasks_.find((t) => t.id === task.id);
      if (!matched) {
        WXU.error({ msg: "异常操作" });
        return;
      }
      tasks_.remove(matched);
      task_count_.as((prev) => prev - 1);
      ui.waterfall$.methods.deleteCell((t) => t.id === task.id);
      list$.deleteItem((t) => t.id === task.id);
    },
    async resumeTask(task) {
      const r = await resumeReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const matched = tasks_.find((t) => t.id === task.id);
      if (!matched) {
        return;
      }
      matched.assign({
        status: "running",
      });
      list$.modifyItem((t) =>
        t.id === task.id ? { ...t, status: "running" } : t,
      );
    },
    async clearTasks() {
      await clearReq.run();
      list$.clear();
      tasks_.as([]);
      task_count_.as(0);
      ui.waterfall$.methods.cleanColumns();
    },
    async openTask(task) {
      const { path, name } = task;
      if (!path || !name) {
        WXU.error({
          msg: "path or name is empty",
        });
        return;
      }
      if (WXU.config.remoteServerEnabled) {
        var u = RemoteAPIHostname + "/preview?id=" + task.id;
        window.open(u);
        return;
      }
      showFileReq.run({ path, name, id: task.id });
    },
    connect() {
      return new Promise((resolve, reject) => {
        const ws = new WebSocket(
          WSServerProtocol + "://" + FakeAPIServerAddr + "/ws/downloader",
        );
        ws.onopen = () => {
          if (WXU.downloader) {
            WXU.downloader.status = "connected";
          }
          resolve(true);
        };
        ws.onclose = () => {
          WXU.error({ msg: "download ws连接已关闭，请刷新页面" });
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
          if (msg.type === "batch_tasks") {
            const list = Array.isArray(msg.data) ? msg.data : [];
            const tasks = list.map((t) => methods.formatTask(t));
            methods.batchInsert(tasks);
            return;
          }
          if (msg.type === "event") {
            const data = msg && msg.data ? msg.data : null;
            if (!data || !data.Key) {
              return;
            }
            if (data.Key === "delete") {
              return;
            }
            const task = data.Task || data.task; // 兼容大小写字段
            if (!task) {
              return;
            }
            methods.upsert(methods.formatTask(task));
          }
        };
      });
    },
    batchInsert(tasks) {
      if (!tasks || !tasks.length) return;
      const newTasks = [];
      for (let i = 0; i < tasks.length; i++) {
        const t = tasks[i];
        if (!t || !t.id) continue;
        const matched = tasks_.find((v) => v.id === t.id);
        if (matched) {
          matched.assign(t);
        } else {
          newTasks.push(t);
        }
      }
      if (newTasks.length) {
        tasks_.unshift(...newTasks);
        task_count_.as((prev) => prev + newTasks.length);
        ui.waterfall$.methods.unshiftItems(newTasks);
        const addedHeight = newTasks.length * (ITEM_HEIGHT + GUTTER);
        ui.view$.addScrollTop(addedHeight);
      }
    },
    upsert(task) {
      console.log("[]upsert task", task);
      if (!task || !task.id) {
        return;
      }
      const matched = tasks_.find((v) => v.id === task.id);
      if (!matched) {
        console.log("[]insert task", task);
        task_count_.as((prev) => {
          return prev + 1;
        });
        tasks_.unshift(task);
        // ui.waterfall$.methods.appendItems([task]);
        ui.waterfall$.methods.unshiftItems([task]);
        const addedHeight = 1 * ITEM_HEIGHT + GUTTER;
        ui.view$.addScrollTop(addedHeight);
        return;
      }
      console.log("[]update task", task);
      matched.assign(task);
    },
  };

  list$.onDataSourceAdded((list) => {
    const tasks = list;
    task_count_.as((prev) => prev + tasks.length);
    tasks_.push(...tasks);
    ui.waterfall$.methods.appendItems(tasks);
  });
  list$.onError((err) => {
    WXU.error({
      msg: err.message,
    });
  });
  let _scrollTop = 0;
  const ui = {
    view$: new Timeless.ui.ScrollViewCore({
      onScroll(pos) {
        _scrollTop = pos.scrollTop;
        ui.waterfall$.methods.handleScroll({ scrollTop: pos.scrollTop });
      },
      async onReachBottom() {
        if (list$.response.loading) {
          return;
        }
        if (list$.response.noMore) {
          ui.view$.finishLoadingMore();
          return;
        }
        await list$.loadMore();
        ui.view$.finishLoadingMore();
      },
    }),
    waterfall$: Timeless.ui.WaterfallModel({
      column: 1,
      size: _pageSize,
      buffer: 10,
      gutter: GUTTER,
    }),
    dropdown$: new Timeless.ui.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: [
        new Timeless.ui.MenuItemCore({
          label: "清空下载记录",
          async onClick() {
            await methods.clearTasks();
            ui.dropdown$.hide();
          },
        }),
        // new Timeless.ui.MenuItemCore({
        //   label: "刷新",
        //   async onClick() {
        //     list$.clear();
        //     tasks_.as([]);
        //     task_count_.as(0);
        //     ui.waterfall$.methods.cleanColumns();
        //     const r = await list$.init();
        //     if (r.error) {
        //       WXU.error({ msg: r.error.message });
        //       ui.dropdown$.hide();
        //       return;
        //     }
        //     const tasks = list$.response.dataSource;
        //     tasks_.push(tasks);
        //     task_count_.as(list$.response.total);
        //     ui.waterfall$.methods.appendItems(tasks);
        //     ui.dropdown$.hide();
        //   },
        // }),
      ],
    }),
  };
  let ready = false;
  return {
    ui,
    state: {
      tasks: tasks_,
      task_count: task_count_,
      running_count: running_count_,
      get scrollTop() {
        return _scrollTop;
      },
    },
    methods,
    async ready() {
      if (ready) {
        return;
      }
      WXU.downloader.status = "disconnected";
      WXU.downloader.reconnect = async function () {
        if (WXU.downloader.status === "connected") return true;
        for (let i = 0; i < 3; i++) {
          try {
            await list$.init();
            return true;
          } catch (e) {
            console.warn("Reconnect attempt " + (i + 1) + " failed");
            await new Promise((r) => setTimeout(r, 1000));
          }
        }
        return false;
      };
      methods.connect();
      const r = await list$.init();
      if (r.error) {
        WXU.error({
          msg: r.error.message,
        });
        return;
      }
      const tasks = list$.response.dataSource;
      tasks_.push(...tasks);
      task_count_.as(list$.response.total);
      console.log("before waterfall$.methods.appendItems", tasks);
      ui.waterfall$.methods.appendItems(tasks);
      ready = true;
    },
    clean() {
      tasks_.as([]);
      task_count_.as(0);
      list$.clear();
      ui.waterfall$.methods.cleanColumns();
    },
  };
}

function DownloaderPanelView(props, children) {
  // const vm$ = props.store;
  const vm$ = DownloaderPanelViewModel();
  const tasks_ = vm$.state.tasks;
  const task_count_ = vm$.state.task_count;
  const running_count_ = vm$.state.running_count;

  return View(
    {
      class: "wx-dl-panel-container",
      onMounted() {
        vm$.ready();
      },
      onUnmounted() {
        // vm$.clean();
      },
    },
    [
      View({ class: "wx-dl-header" }, [
        View({ class: "wx-dl-title" }, [
          "Downloads",
          computed(task_count_, (d) => {
            return d > 0 ? `（${d}）` : "";
          }),
        ]),
        DropdownMenu(
          {
            store: vm$.ui.dropdown$,
          },
          [
            View(
              {
                class: "wx-dl-more-btn",
              },
              [
                Timeless.icons.EllipsisVerticalOutlined({
                  style: "font-size: 18px;",
                }),
              ],
            ),
          ],
        ),
      ]),
      View(
        {
          class: "wx-dl-list wx-dl-dark-scroll",
        },
        [
          ScrollView(
            {
              style: "background-color: transparent; margin-top: 0;",
              store: vm$.ui.view$,
            },
            [
              Show(
                {
                  when: computed(task_count_, (d) => d > 0),
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
                          ["暂无下载任务"],
                        ),
                      ],
                    ),
                  ],
                },
                [
                  Waterfall({
                    store: vm$.ui.waterfall$,
                    class: "scroll-view-waterfall !overflow-visible !h-auto",
                    style: "overflow: visible; height: auto;",
                    render(task) {
                      console.log("task in waterfall", task);
                      const iconSize = "50px";
                      const state_ = computed(task, (t) => {
                        // console.log("the task is changed", t.status);
                        const pr = format_download_percent(t);
                        const isCompleted =
                          t.status === "done" ||
                          t.status === "completed" ||
                          t.status === "success" ||
                          t.status === "finished" ||
                          (pr === 100 && t.status !== "running");

                        const isPaused =
                          t.status === "paused" || t.status === "pause";
                        const isRunning = t.status === "running";

                        let statusText = t.status;
                        let statusColor = "var(--weui-FG-1)";
                        var isFailed =
                          t.status === "failed" || t.status === "error";
                        var isPending = t.status === "pending";
                        if (isRunning) {
                          const speed = format_download_speed(
                            t.progress ? t.progress.speed : 0,
                          );
                          statusText = `${speed} • ${pr}%`;
                        } else if (isCompleted) {
                          statusText = "已完成";
                          // Calculate size
                          const total =
                            t.meta && t.meta.res ? t.meta.res.size : 0;
                          if (total) {
                            statusText = WXU.bytes_to_size(total);
                          }
                        } else if (isFailed) {
                          statusText = "下载失败";
                          statusColor = "#FA5151";
                        } else if (isPending) {
                          statusText = "等待中...";
                        } else if (isPaused) {
                          statusText = `已暂停 • ${pr}%`;
                        }
                        return {
                          pr,
                          isCompleted,
                          isPaused,
                          isRunning,
                          isFailed,
                          canResume: isFailed || isPaused,
                          statusText,
                          statusColor,
                        };
                      });
                      const isOpenExternal = WXU.config.remoteServerEnabled;
                      const filename = task.name;
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
                      const offset = computed(state_, (d) => {
                        return circumference - (d.pr / 100) * circumference;
                      });
                      const strokeColor = computed(state_, (d) => {
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
                                when: computed(state_, (t) => {
                                  return t.isRunning || t.isPaused;
                                }),
                                fallback: [
                                  DangerouslyInnerHTML(PrefixIcon.value),
                                ],
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
                                          stroke: "var(--weui-FG-3)",
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
                              [computed(task, (t) => t.name)],
                            ),
                            View(
                              {
                                class: "weui-cell__desc",
                                style: computed(state_, (d) => {
                                  return `margin-top: 4px; color: ${d.statusColor}; font-size: 12px;`;
                                }),
                              },
                              [
                                computed(state_, (d) => {
                                  return d.statusText;
                                }),
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
                              Switch(
                                {
                                  when: combine(
                                    {
                                      state: state_,
                                      running_count: running_count_,
                                    },
                                    (t) => {
                                      if (t.state.isCompleted) {
                                        return 1;
                                      }
                                      if (t.state.isRunning) {
                                        return 2;
                                      }
                                      if (t.state.isPaused) {
                                        return 3;
                                      }
                                      if (t.state.isFailed) {
                                        return 4;
                                      }
                                      return 0;
                                    },
                                  ),
                                },
                                [
                                  // 场景 1: 已完成 -> 显示打开按钮
                                  Match(1, [
                                    Timeless.h(
                                      View,
                                      {
                                        type: "a",
                                        class: "wx-download-item-open",
                                        style: btnStyle,
                                        onClick() {
                                          vm$.methods.openTask(task);
                                        },
                                      },
                                      [
                                        Show(
                                          {
                                            when: !!isOpenExternal,
                                            fallback: [
                                              DangerouslyInnerHTML(FolderIcon),
                                            ],
                                          },
                                          [
                                            DangerouslyInnerHTML(
                                              ExternalLinkIcon,
                                            ),
                                          ],
                                        ),
                                      ],
                                    ),
                                  ]),
                                  // 场景 2: 正在运行 -> 显示暂停按钮
                                  Match(2, [
                                    Timeless.h(
                                      View,
                                      {
                                        type: "a",
                                        class: "wx-download-item-pause",
                                        style: btnStyle,
                                        onClick() {
                                          vm$.methods.pauseTask(task);
                                        },
                                      },
                                      [DangerouslyInnerHTML(PauseIcon)],
                                    ),
                                  ]),
                                  // 场景 3: 暂停或失败且未达最大并发 -> 显示恢复按钮
                                  Match(3, [
                                    Timeless.h(
                                      View,
                                      {
                                        type: "a",
                                        class: "wx-download-item-resume",
                                        style: Timeless.sn([
                                          btnStyle,
                                          computed(running_count_, (t) => {
                                            return t > WXU.config.MaxRunning
                                              ? "opacity: 0.6; cursor: not-allowed;"
                                              : "";
                                          }),
                                        ]),
                                        onClick() {
                                          vm$.methods.resumeTask(task);
                                        },
                                      },
                                      [DangerouslyInnerHTML(PlayIcon)],
                                    ),
                                  ]),
                                  Match(4, [
                                    Timeless.h(
                                      View,
                                      {
                                        type: "a",
                                        class: "wx-download-item-resume",
                                        style: Timeless.sn([
                                          btnStyle,
                                          computed(running_count_, (t) => {
                                            return t > WXU.config.MaxRunning
                                              ? "opacity: 0.6; cursor: not-allowed;"
                                              : "";
                                          }),
                                        ]),
                                        onClick() {
                                          vm$.methods.resumeTask(task);
                                        },
                                      },
                                      [DangerouslyInnerHTML(RetryIcon)],
                                    ),
                                  ]),
                                ],
                              ),
                              View(
                                {
                                  type: "a",
                                  class: "wx-download-item-delete",
                                  style: btnStyle,
                                  onClick() {
                                    vm$.methods.deleteTask(task);
                                  },
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
        ],
      ),
    ],
  );
}

function DownloaderEntry(props) {
  return Popover(
    {
      store: props.popover$,
      content: [DownloaderPanelView({})],
    },
    [
      View(
        {
          class:
            "mr-2 relative h-5 w-5 flex-initial flex-shrink-0 cursor-pointer",
        },
        [
          DangerouslyInnerHTML(
            `<svg class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg"><path d="M512 706.608L781.968 436.64a32 32 0 1 0-45.248-45.256L544 584.096V192a32 32 0 0 0-64 0v392.096l-192.712-192.72a32 32 0 0 0-45.256 45.256L512 706.608z" fill="currentColor"></path><path d="M824 640a32 32 0 0 0-32 32v128.36c0 3.112 0 8.496-0.48 11.472l-1.008 1.024c-0.952 0.984-2.104 2.168-3.112 3.152h-538.48c-2.448-0.664-7.808-3.56-10.608-6.36-2.776-2.784-5.656-8.128-6.32-10.568V672a32 32 0 0 0-64 0v128c0 20.632 12.608 42.456 25.088 54.912C205.584 867.4 227.408 880 248 880h544c22.496 0 36.208-14.112 44.408-22.536l2.48-2.528c17.128-17.088 17.12-41.472 17.12-54.928V672A32.016 32.016 0 0 0 824 640z" fill="currentColor"></path></svg>`,
          ),
        ],
      ),
    ],
  );
}

(() => {
  function insert_downloader($wrap, $trigger) {
    $wrap.insertBefore($trigger, $wrap.firstChild);
    const popover$ = new Timeless.ui.PopoverCore({
      offsetY: 4,
      destroyOnClose: false,
    });
    WXU.downloader.show = function () {
      popover$.show();
    };
    WXU.downloader.hide = function () {
      popover$.hide();
    };
    WXU.downloader.toggle = function () {
      popover$.toggle();
    };
    Timeless.render(DownloaderEntry({ popover$ }), $trigger);
  }
  let mounted = false;
  if (window.location.pathname === "/web/pages/profile") {
    WXU.observe_node(".page-profile", () => {
      var $page = document.querySelector(".page-profile");
      if (mounted) return;
      if (!$page) return;
      var $box = $page;
      var $btn_wrap = document.createElement("div");
      $btn_wrap.style.cssText =
        "z-index: 999; position: fixed; right: 40px; top: 36px;";
      insert_downloader($box, $btn_wrap);
      mounted = true;
    });
  } else if (window.location.hostname === "mp.weixin.qq.com") {
    //
  } else {
    WXU.observe_node(".home-header", () => {
      var $header = document.querySelector(".home-header");
      console.log("[DOWNLOADER]insert_downloader", mounted, $header);
      if (mounted) return;
      if (!$header) return;
      var $box = $header.children[$header.children.length - 1];
      if (!$box) return;
      var $btn_wrap = $box.children[0];
      if (!$btn_wrap) {
        $btn_wrap = $box;
      }
      var $download_panel_button = download_btn7();
      insert_downloader($btn_wrap, $download_panel_button);
      mounted = true;
    });
  }
})();

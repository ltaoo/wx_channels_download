var _isWin = /Windows|Win/i.test(
  navigator.userAgent || navigator.platform || "",
);

var STORAGE_KEY = "wx_download_servers";

function loadServers() {
  try {
    var raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch (e) {
    return [];
  }
}

function saveServers(servers) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(servers));
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

function bytes_to_size(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

// ── Flex Layout Component ──
// Props shorthand: col, wrap, between, center, align, grow, shrink0, g (gap number)
// Also accepts class/style for extra customization
function Flex(p, children) {
  var props = p || {};
  var cls = "flex";
  if (props.col) cls += " flex-col";
  if (props.wrap) cls += " flex-wrap";
  if (props.between) cls += " items-center justify-between";
  if (props.center) cls += " items-center justify-center";
  if (props.align) cls += " items-center";
  if (props.grow) cls += " flex-1";
  if (props.expand) cls += " flex-1 min-h-0";
  if (props.shrink0) cls += " flex-shrink-0";
  if (props.g) cls += " gap-" + props.g;
  if (props.sy) cls += " space-y-" + props.sy;
  if (props.class) cls += " " + props.class;
  var extra = {};
  if (cls) extra.class = cls;
  if (props.style) extra.style = props.style;
  if (props.type) extra.type = props.type;
  if (props.onClick) extra.onClick = props.onClick;
  if (props.innerHTML) extra.innerHTML = props.innerHTML;
  return View(extra, children);
}

// ── PageView (React Native style) ──
// Default: h-screen w-full, flex-col container
function PageView(p, children) {
  var props = p || {};
  var cls = "w-full h-screen flex flex-col overflow-hidden";
  if (props.g) cls += " gap-" + props.g;
  if (props.sy) cls += " space-y-" + props.sy;
  if (props.class) cls += " " + props.class;
  var extra = { class: cls };
  if (props.style) extra.style = props.style;
  return View(extra, children);
}

// ── Icons ──
var FileIcon =
  '<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" stroke-linecap="round" stroke-linejoin="round"/><path d="M14 2v6h6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var MP3Icon =
  '<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9 18V5l12-2v13" stroke-linecap="round" stroke-linejoin="round"/><circle cx="6" cy="18" r="3" stroke-linecap="round" stroke-linejoin="round"/><circle cx="18" cy="16" r="3" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var MP4Icon =
  '<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M10 9l5 3-5 3V9z" fill="currentColor" stroke="none"/></svg>';
var ImageIcon =
  '<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var FolderIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var ExternalLinkIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" stroke-linecap="round" stroke-linejoin="round"/><polyline points="15 3 21 3 21 9" stroke-linecap="round" stroke-linejoin="round"/><line x1="10" y1="14" x2="21" y2="3" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var PauseIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>';
var PlayIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3" fill="currentColor" stroke="none"/></svg>';
var RetryIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="23 4 23 10 17 10" stroke-linecap="round" stroke-linejoin="round"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" stroke-linecap="round" stroke-linejoin="round"/></svg>';
var DeleteIcon =
  '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="3 6 5 6 21 6" stroke-linecap="round" stroke-linejoin="round"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

/** 下载列表页 */
export default function HomeDownloadView(props) {
  const ITEM_HEIGHT = 82;
  const GUTTER = 8;

  // ── Server management ──
  const servers_ = refarr(loadServers());
  const selectedServer_ = ref(null);
  const connected_ = ref(false);

  const hostInput$ = new Timeless.ui.InputCore({
    defaultValue: "",
    placeholder: "IP (如 192.168.1.100)",
  });
  const portInput$ = new Timeless.ui.InputCore({
    defaultValue: "",
    placeholder: "端口",
  });

  function persistServers() {
    saveServers(
      servers_.value.map((s) => ({ host: s.host, port: s.port, name: s.name })),
    );
  }

  function addServer() {
    var host = hostInput$.state.value.trim();
    var port = portInput$.state.value.trim();
    if (!host || !port) return;
    var exists = servers_.value.find((s) => s.host === host && s.port === port);
    if (exists) return;
    servers_.push({ host: host, port: port, name: `${host}:${port}` });
    persistServers();
    hostInput$.clear();
    portInput$.clear();
  }

  function removeServer(server) {
    var matched = servers_.value.find(
      (s) => s.host === server.host && s.port === server.port,
    );
    if (matched) {
      servers_.remove(matched);
      persistServers();
    }
    if (
      selectedServer_.value &&
      selectedServer_.value.host === server.host &&
      selectedServer_.value.port === server.port
    ) {
      disconnectServer();
    }
  }

  // ── Remote HTTP client + request factory ──
  const remoteClient = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
  });
  Timeless.web.provide_http_client(remoteClient);

  const remoteApi = Timeless.kit.request_factory({
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

  // ── RequestCore instances ──
  function formatTask(task) {
    var p = task.meta && task.meta.opts ? task.meta.opts.path || "" : "";
    var n = task.meta && task.meta.opts ? task.meta.opts.name || "" : "";
    var sep = _isWin ? "\\" : "/";
    var filepath = "";
    if (p && n) {
      filepath = p.endsWith(sep) ? p + n : p + sep + n;
    }
    return {
      height: ITEM_HEIGHT,
      ...task,
      path: p,
      name: n,
      filepath: filepath,
    };
  }

  const taskListReq = new Timeless.kit.RequestCore(
    (params) => remoteApi.get("/api/task/list", params),
    {
      client: remoteClient,
      process(r) {
        if (r.error) {
          return r.error;
        }
        console.log(r.data);
        return Timeless.Result.Ok({
          list: (r.data.list || []).map((t) => formatTask(t)),
          total: r.data.total || 0,
          page: r.data.page || 1,
          pageSize: r.data.page_size || 50,
        });
      },
    },
  );

  const deleteReq = new Timeless.kit.RequestCore(
    (id) => remoteApi.post("/api/task/delete", { id }),
    { client: remoteClient },
  );
  const startReq = new Timeless.kit.RequestCore(
    (id) => remoteApi.post("/api/task/start", { id }),
    { client: remoteClient },
  );
  const pauseReq = new Timeless.kit.RequestCore(
    (id) => remoteApi.post("/api/task/pause", { id }),
    { client: remoteClient },
  );
  const resumeReq = new Timeless.kit.RequestCore(
    (id) => remoteApi.post("/api/task/resume", { id }),
    { client: remoteClient },
  );
  const clearReq = new Timeless.kit.RequestCore(
    () => remoteApi.post("/api/task/clear"),
    { client: remoteClient },
  );
  const showFileReq = new Timeless.kit.RequestCore(
    ({ path, name, id }) =>
      remoteApi.post("/api/show_file", { path, name, id }),
    { client: remoteClient },
  );

  // ── ListCore ──
  const list$ = new Timeless.kit.ListCore(taskListReq, {
    pageSize: 50,
  });

  // Reactive refs driven by ListCore events
  const tasks_ = refarr([]);
  const task_count_ = ref(0);
  const running_count_ = computed(tasks_, (t) => {
    return t.filter((v) => v.status === "running").length;
  });

  // ── Waterfall + ScrollView ──
  const waterfall$ = Timeless.ui.WaterfallModel({
    column: 1,
    size: 20,
    buffer: 10,
    gutter: GUTTER,
  });

  // Sync ListCore → reactive refs + waterfall
  function syncFromList() {
    var ds = list$.response.dataSource || [];
    console.log("[]syncFromList", list$.response);
    tasks_.as(ds);
    task_count_.as(list$.response.total || 0);
    waterfall$.methods.cleanColumns();
    waterfall$.methods.appendItems(ds);
  }

  list$.onStateChange(() => {
    syncFromList();
  });

  const view$ = new Timeless.ui.ScrollViewCore({
    onScroll(pos) {
      console.log('view$ is scrolling', pos.scrollTop);
      waterfall$.methods.handleScroll({ scrollTop: pos.scrollTop });
    },
    async onReachBottom() {
      if (list$.response.loading) {
        view$.finishLoadingMore();
        return;
      }
      if (list$.response.noMore) {
        view$.finishLoadingMore();
        return;
      }
      await list$.loadMore();
      view$.finishLoadingMore();
    },
  });

  // ── Task actions ──
  async function deleteTask(task) {
    await deleteReq.run(task.id);
    list$.deleteItem((t) => t.id === task.id);
    syncFromList();
  }

  async function startTask(task) {
    await startReq.run(task.id);
    list$.modifyItem((t) =>
      t.id === task.id ? { ...t, status: "running" } : t,
    );
    syncFromList();
  }

  async function pauseTask(task) {
    await pauseReq.run(task.id);
    list$.modifyItem((t) =>
      t.id === task.id ? { ...t, status: "paused" } : t,
    );
    syncFromList();
  }

  async function resumeTask(task) {
    if (running_count_.value > 5) return;
    await resumeReq.run(task.id);
    list$.modifyItem((t) =>
      t.id === task.id ? { ...t, status: "running" } : t,
    );
    syncFromList();
  }

  async function clearTasks() {
    await clearReq.run();
    list$.clear();
    tasks_.as([]);
    task_count_.as(0);
    waterfall$.methods.cleanColumns();
  }

  async function openTask(task) {
    if (!task.path || !task.name) return;
    await showFileReq.run({ path: task.path, name: task.name, id: task.id });
  }

  // ── Server connect / disconnect ──
  async function connectServer(server) {
    selectedServer_.as(server);
    connected_.as(true);
    remoteClient.hostname = `http://${server.host}:${server.port}`;
    waterfall$.methods.cleanColumns();
    await list$.init();
  }

  function disconnectServer() {
    selectedServer_.as(null);
    connected_.as(false);
    remoteClient.hostname = "";
    list$.clear();
    tasks_.as([]);
    task_count_.as(0);
    waterfall$.methods.cleanColumns();
  }

  // ── Dot indicator ──
  function Dot(active) {
    return View({
      style: active
        ? "width: 6px; height: 6px; border-radius: 50%; background: #07C160;"
        : "width: 6px; height: 6px; border-radius: 50%; background: var(--weui-FG-3);",
    });
  }

  return PageView({ class: "p-6" }, [
    Flex({ col: true, g: 6, expand: true, class: "max-w-4xl mx-auto overflow-hidden" }, [
      // Server management panel
      View(
        {
          class:
            "rounded-lg bg-[var(--weui-BG-2)] border border-[var(--weui-FG-5)] p-4 space-y-4",
        },
        [
          Flex({ between: true }, [
            View({ class: "text-lg font-bold text-[var(--weui-FG-0)]" }, [
              "服务器管理",
            ]),
          ]),
          // Add server form
          Flex({ align: true, g: 2 }, [
            View(
              {
                class:
                  "flex-1 h-8 px-3 rounded-lg border border-[var(--weui-FG-5)] bg-[var(--weui-BG-1)] text-[var(--weui-FG-0)] text-sm flex items-center",
              },
              [
                Input({
                  store: hostInput$,
                  class: "w-full bg-transparent outline-none",
                }),
              ],
            ),
            View(
              {
                class:
                  "w-24 h-8 px-3 rounded-lg border border-[var(--weui-FG-5)] bg-[var(--weui-BG-1)] text-[var(--weui-FG-0)] text-sm flex items-center",
              },
              [
                Input({
                  store: portInput$,
                  class: "w-full bg-transparent outline-none",
                }),
              ],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  onClick() {
                    addServer();
                  },
                }),
                class:
                  "px-3 h-8 rounded-lg bg-[var(--weui-TAG-BLUE)] text-white text-sm hover:opacity-90 transition-opacity",
              },
              [Txt("添加")],
            ),
          ]),
          // Server list
          Show(
            {
              when: computed(servers_, (s) => s.length > 0),
              fallback: [
                View(
                  {
                    class: "text-sm text-[var(--weui-FG-2)] py-2",
                  },
                  [Txt("暂无服务器，请添加")],
                ),
              ],
            },
            [
              Flex({ wrap: true, g: 2 }, [
                For({
                  each: combine(
                    { servers: servers_, selected: selectedServer_ },
                    (t) => {
                      return t.servers.map((server) => {
                        return {
                          ...server,
                          isActive:
                            t.selected &&
                            t.selected.host === server.host &&
                            t.selected.port === server.port,
                        };
                      });
                    },
                  ),
                  render(server) {
                    var chipClass = server.isActive
                      ? "inline-flex items-center gap-1 px-3 py-1.5 rounded-full text-xs bg-[var(--weui-TAG-BLUE)] text-white cursor-pointer"
                      : "inline-flex items-center gap-1 px-3 py-1.5 rounded-full text-xs bg-[var(--weui-BG-1)] border border-[var(--weui-FG-5)] text-[var(--weui-FG-0)] cursor-pointer hover:border-[var(--weui-TAG-BLUE)] transition-colors";
                    return View({ class: chipClass }, [
                      Flex(
                        {
                          align: true,
                          g: 1,
                          type: "a",
                          onClick() {
                            if (server.isActive) {
                              disconnectServer();
                            } else {
                              connectServer(server);
                            }
                          },
                        },
                        [
                          Dot(server.isActive),
                          Txt(server.name || `${server.host}:${server.port}`),
                        ],
                      ),
                      View({
                        type: "span",
                        style:
                          "margin-left: 4px; cursor: pointer; opacity: 0.6; font-size: 14px; line-height: 1;",
                        onClick() {
                          removeServer(server);
                        },
                        innerHTML: "\u00d7",
                      }),
                    ]);
                  },
                }),
              ]),
            ],
          ),
          // Connection status
          computed(connected_, (isConnected) => {
            if (!isConnected) {
              return View(
                {
                  class: "text-xs text-[var(--weui-FG-2)]",
                },
                [Txt("点击服务器名称进行连接")],
              );
            }
            return Flex({ align: true, g: 2, class: "text-xs" }, [
              Dot(true),
              computed(selectedServer_, (s) => {
                return Txt(
                  `已连接: ${s ? s.name || `${s.host}:${s.port}` : ""}`,
                );
              }),
            ]);
          }),
        ],
      ),
      // Header
      Flex({ between: true }, [
        View({ class: "space-y-2" }, [
          View({ class: "text-2xl font-bold text-[var(--weui-FG-0)]" }, [
            Txt("下载列表"),
            computed(task_count_, (d) => {
              return d > 0 ? `（${d}）` : "";
            }),
          ]),
          View({ class: "text-sm text-[var(--weui-FG-1)]" }, [
            Txt("管理所有下载任务"),
          ]),
        ]),
        Flex({ g: 2 }, [
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                onClick() {
                  if (!connected_.value) return;
                  waterfall$.methods.cleanColumns();
                  list$.init();
                },
              }),
              class:
                "px-3 h-8 rounded-lg border border-[var(--weui-FG-3)] bg-[var(--weui-BG-2)] text-[var(--weui-FG-0)] text-sm hover:bg-[var(--weui-BG-COLOR-ACTIVE)] transition-colors",
            },
            ["刷新"],
          ),
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                async onClick() {
                  await clearTasks();
                },
              }),
              class:
                "px-3 h-8 rounded-lg border border-[var(--weui-FG-3)] bg-[var(--weui-BG-2)] text-[var(--weui-FG-0)] text-sm hover:bg-[var(--weui-BG-COLOR-ACTIVE)] transition-colors",
            },
            [Txt("清空")],
          ),
        ]),
      ]),
      // Scrollable task list with Waterfall
      View({ class: "flex-1 min-h-0 overflow-hidden" }, [
        ScrollView(
          {
            // class: "h-full",
            // style: "background-color: transparent;",
            store: view$,
          },
        [
          Show(
            {
              when: computed(task_count_, (d) => d > 0),
              fallback: [
                Flex({
                  col: true,
                  center: true,
                  class: "py-20 text-[var(--weui-FG-2)]",
                  children: [
                    View(
                      { class: "text-sm" },
                      computed(connected_, (t) => {
                        if (!t) {
                          return "请先连接服务器";
                        }
                        return "暂无下载任务";
                      }),
                    ),
                  ],
                }),
              ],
            },
            [
              Waterfall({
                store: waterfall$,
                class: "scroll-view-waterfall !overflow-visible !h-auto",
                render(task) {
                  const iconSize = "50px";
                  const state_ = computed(task, (t) => {
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
                    const isFailed =
                      t.status === "failed" || t.status === "error";
                    const isPending = t.status === "pending";

                    let statusText = t.status;
                    let statusColor = "var(--weui-FG-1)";
                    if (isRunning) {
                      const speed = format_download_speed(
                        t.progress ? t.progress.speed : 0,
                      );
                      statusText = `${speed} • ${pr}%`;
                    } else if (isCompleted) {
                      statusText = "已完成";
                      const total = t.meta && t.meta.res ? t.meta.res.size : 0;
                      if (total) {
                        statusText = bytes_to_size(total);
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

                  const filename = task.name;
                  const PrefixIcon = computed(filename, (t) => {
                    let selectedIcon = FileIcon;
                    if (t) {
                      const ext = t.split(".").pop().toLowerCase();
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
                    return selectedIcon;
                  });

                  const radius = 22;
                  const circumference = 2 * Math.PI * radius;
                  const offset = computed(state_, (d) => {
                    return circumference - (d.pr / 100) * circumference;
                  });
                  const strokeColor = computed(state_, (d) => {
                    return d.isPaused ? "#FBC02D" : "#07C160";
                  });

                  return Flex(
                    {
                      align: true,
                      g: 4,
                      class:
                        "p-4 rounded-lg bg-[var(--weui-BG-2)] border border-[var(--weui-FG-5)]",
                    },
                    [
                      // Icon with progress circle
                      Flex({
                        shrink0: true,
                        center: true,
                        style: `position: relative; width: ${iconSize}; height: ${iconSize}; color: var(--weui-FG-0);`,
                        children: [
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
                      }),
                      // Info
                      Flex({ col: true, grow: true, sy: 1, class: "min-w-0" }, [
                        View(
                          {
                            class:
                              "text-sm font-medium text-[var(--weui-FG-0)] truncate",
                          },
                          [task.name],
                        ),
                        View(
                          {
                            class: "text-xs",
                            style: computed(state_, (d) => {
                              return `color: ${d.statusColor};`;
                            }),
                          },
                          [
                            computed(state_, (d) => {
                              return d.statusText;
                            }),
                          ],
                        ),
                      ]),
                      // Actions
                      Flex(
                        { align: true, shrink0: true },
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
                                    if (t.state.isCompleted) return 1;
                                    if (t.state.isRunning) return 2;
                                    if (t.state.isPaused) return 3;
                                    if (t.state.isFailed) return 4;
                                    return 0;
                                  },
                                ),
                              },
                              [
                                Match(1, [
                                  h(
                                    View,
                                    {
                                      type: "a",
                                      style: btnStyle,
                                      onClick() {
                                        openTask(task);
                                      },
                                    },
                                    [DangerouslyInnerHTML(FolderIcon)],
                                  ),
                                ]),
                                Match(2, [
                                  h(
                                    View,
                                    {
                                      type: "a",
                                      style: btnStyle,
                                      onClick() {
                                        pauseTask(task);
                                      },
                                    },
                                    [DangerouslyInnerHTML(PauseIcon)],
                                  ),
                                ]),
                                Match(3, [
                                  h(
                                    View,
                                    {
                                      type: "a",
                                      style: sn([
                                        btnStyle,
                                        computed(running_count_, (t) => {
                                          return t > 5
                                            ? "opacity: 0.6; cursor: not-allowed;"
                                            : "";
                                        }),
                                      ]),
                                      onClick() {
                                        resumeTask(task);
                                      },
                                    },
                                    [DangerouslyInnerHTML(PlayIcon)],
                                  ),
                                ]),
                                Match(4, [
                                  h(
                                    View,
                                    {
                                      type: "a",
                                      style: sn([
                                        btnStyle,
                                        computed(running_count_, (t) => {
                                          return t > 5
                                            ? "opacity: 0.6; cursor: not-allowed;"
                                            : "";
                                        }),
                                      ]),
                                      onClick() {
                                        resumeTask(task);
                                      },
                                    },
                                    [DangerouslyInnerHTML(RetryIcon)],
                                  ),
                                ]),
                              ],
                            ),
                            h(
                              View,
                              {
                                type: "a",
                                style: btnStyle,
                                onClick() {
                                  deleteTask(task);
                                },
                              },
                              [DangerouslyInnerHTML(DeleteIcon)],
                            ),
                          ];
                        })(),
                      ),
                    ],
                  );
                },
              }),
            ],
          ),
        ],
      ),
      ]),
    ]),
  ]);
}

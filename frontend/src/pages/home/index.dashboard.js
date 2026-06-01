import {
  savePage,
  getTask,
  daemonStatus,
  daemonStart,
  daemonStop,
  daemonRestart,
  daemonTabs,
  daemonPageInfo,
  discoverDebuggerURL,
  fetchViaDaemon,
} from "@/pages/tools/index.model.js";

const FETCH_METHODS = [
  { value: "http", label: "HTTP Request", desc: "标准 HTTP GET 请求" },
  {
    value: "daemon",
    label: "Daemon Client",
    desc: "通过 daemon 复用浏览器标签页",
  },
  { value: "cdp", label: "CDP", desc: "Chrome DevTools Protocol 完整渲染" },
];

const HTTP_METHODS = [
  { value: "GET", label: "GET" },
  { value: "POST", label: "POST" },
];

export default function DashboardPageView(props) {
  // ---- Daemon state ----
  const dmRunning_ = ref(false);
  const dmLoading_ = ref(false);
  const dmError_ = ref("");
  const dmDebuggerURL_ = ref("");
  const dmTabs_ = refarr([]);
  const dmPageInfo_ = ref(null);
  const dmShowAdvanced_ = ref(false);

  // ---- Quick Get state ----
  const getUrl_ = ref("");
  const getFetchMode_ = ref("http");
  const getHttpMethod_ = ref("GET");
  const getHeaders_ = ref("");
  const getBody_ = ref("");
  const getUserAgent_ = ref("");
  const getTimeout_ = ref("30");
  const getShowAdvanced_ = ref(false);
  const getSubmitting_ = ref(false);
  const getResult_ = ref(null);
  const getError_ = ref("");
  const getTaskId_ = ref("");
  const getStatusEvtSrc_ = ref(null);

  // ==================== Daemon Methods ====================

  async function refreshDaemon() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStatus("default");
    dmLoading_.as(false);
    if (r.error) {
      dmError_.as(String(r.error));
      return;
    }
    dmRunning_.as(!!r.data.running);
  }

  async function handleDaemonStart() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStart("default");
    dmLoading_.as(false);
    if (r.error) {
      dmError_.as(String(r.error));
      return;
    }
    dmRunning_.as(true);
  }

  async function handleDaemonStop() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStop("default");
    dmLoading_.as(false);
    if (r.error) {
      dmError_.as(String(r.error));
      return;
    }
    dmRunning_.as(false);
    dmDebuggerURL_.as("");
  }

  async function handleDaemonRestart() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonRestart("default");
    dmLoading_.as(false);
    if (r.error) {
      dmError_.as(String(r.error));
      return;
    }
    dmRunning_.as(true);
  }

  async function loadDebuggerURL() {
    const r = await discoverDebuggerURL();
    if (!r.error) {
      dmDebuggerURL_.as(r.data.webSocketDebuggerUrl || "");
    }
  }

  async function loadDaemonTabs() {
    const r = await daemonTabs("default");
    if (!r.error && r.data) dmTabs_.as(r.data.tabs || []);
  }

  async function loadPageInfo() {
    const r = await daemonPageInfo("default");
    if (!r.error) dmPageInfo_.as(r.data);
  }

  // ==================== Quick Get Methods ====================

  function closeStatusStream() {
    if (getStatusEvtSrc_.value) {
      getStatusEvtSrc_.value.close();
      getStatusEvtSrc_.as(null);
    }
  }

  async function handleGet() {
    const url = getUrl_.value.trim();
    if (!url) {
      getError_.as("请输入 URL");
      return;
    }
    getSubmitting_.as(true);
    getError_.as("");
    getResult_.as(null);
    getTaskId_.as("");
    closeStatusStream();

    const mode = getFetchMode_.value;

    if (mode === "daemon") {
      // Use daemon client to fetch
      const opts = { method: getHttpMethod_.value };
      if (getTimeout_.value) {
        const t = parseInt(getTimeout_.value, 10);
        if (!isNaN(t)) opts.timeout = t;
      }
      const headersStr = getHeaders_.value.trim();
      if (headersStr) {
        const headers = {};
        for (const line of headersStr.split("\n")) {
          const idx = line.indexOf(":");
          if (idx > 0)
            headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
        }
        if (Object.keys(headers).length) opts.headers = headers;
      }
      if (getBody_.value.trim()) opts.body = getBody_.value.trim();

      const r = await fetchViaDaemon(url, opts);
      getSubmitting_.as(false);
      if (r.error) {
        getError_.as(String(r.error));
        return;
      }
      getResult_.as({
        url: r.data.url,
        title: r.data.title,
        htmlSize: r.data.htmlSize,
        html: r.data.html,
        method: "daemon",
      });
    } else {
      // Use save API (http or cdp)
      const opts = { fetch_mode: mode };
      if (getUserAgent_.value) opts.user_agent = getUserAgent_.value;
      if (getTimeout_.value) {
        const t = parseInt(getTimeout_.value, 10);
        if (!isNaN(t)) opts.timeout = t * 1e9;
      }
      const headersStr = getHeaders_.value.trim();
      if (headersStr) {
        const headers = {};
        for (const line of headersStr.split("\n")) {
          const idx = line.indexOf(":");
          if (idx > 0)
            headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
        }
        if (Object.keys(headers).length) opts.headers = headers;
      }

      const r = await savePage(url, opts);
      getSubmitting_.as(false);
      if (r.error) {
        getError_.as(String(r.error));
        return;
      }
      const taskId = r.data.task_id;
      getTaskId_.as(taskId);

      // Watch task via SSE
      const src = new EventSource(`/api/v1/tasks/${taskId}/events`);
      getStatusEvtSrc_.as(src);
      src.onmessage = function (event) {
        try {
          const data = JSON.parse(event.data);
          if (data.status === "completed") {
            getResult_.as({
              ...data.result,
              method: mode,
            });
            src.close();
            getStatusEvtSrc_.as(null);
          } else if (data.status === "failed" || data.status === "cancelled") {
            getError_.as(data.error || data.status);
            src.close();
            getStatusEvtSrc_.as(null);
          }
        } catch (_) {
          /* ignore */
        }
      };
      src.onerror = function () {
        src.close();
        getStatusEvtSrc_.as(null);
        // Fallback poll
        getTask(taskId).then(function (r2) {
          if (!r2.error && r2.data) {
            if (r2.data.status === "completed")
              getResult_.as({ ...r2.data.result, method: mode });
            else if (r2.data.status === "failed")
              getError_.as(r2.data.error || r2.data.status);
          }
        });
      };
    }
  }

  // ==================== Helpers ====================

  function formatBytes(n) {
    if (n == null) return "-";
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / (1024 * 1024)).toFixed(1) + " MB";
  }

  function statusBadge(running) {
    return View(
      {
        class: computed(running, function (v) {
          return v
            ? "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-800 dark:text-green-200"
            : "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400";
        }),
      },
      [
        View({
          class: computed(running, function (v) {
            return v
              ? "w-1.5 h-1.5 rounded-full bg-green-500"
              : "w-1.5 h-1.5 rounded-full bg-zinc-400";
          }),
        }),
        computed(running, function (v) {
          return v ? "运行中" : "已停止";
        }),
      ],
    );
  }

  function methodBadge(mode) {
    const colors = {
      http: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
      daemon:
        "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
      cdp: "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300",
    };
    const labels = { http: "HTTP", daemon: "Daemon", cdp: "CDP" };
    return View(
      {
        class:
          "inline-flex px-1.5 py-0.5 rounded text-[10px] font-mono font-medium " +
          (colors[mode] || colors.http),
      },
      [labels[mode] || mode],
    );
  }

  // ==================== View ====================

  return ScrollView(
    {
      store: new Timeless.ui.ScrollViewCore({}),
      class: "h-full",
      onMounted: function () {
        refreshDaemon();
      },
      onUnmounted: function () {
        closeStatusStream();
      },
    },
    [
      View({ class: "p-6 max-w-4xl space-y-6" }, [
        // ---- Header ----
        View(
          {
            class:
              "text-2xl font-bold tracking-tight text-zinc-900 dark:text-zinc-50",
          },
          ["工作台"],
        ),

        // ---- Stats Row ----
        View({ class: "grid grid-cols-3 gap-4" }, [
          // Daemon status
          View(
            {
              class:
                "p-4 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-950 space-y-1",
            },
            [
              View({ class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
                "Daemon",
              ]),
              View({ class: "flex items-center gap-2" }, [
                statusBadge(dmRunning_),
              ]),
            ],
          ),
          // Tabs count
          View(
            {
              class:
                "p-4 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-950 space-y-1",
            },
            [
              View({ class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
                "浏览器标签页",
              ]),
              View(
                { class: "text-2xl font-bold text-zinc-900 dark:text-zinc-50" },
                [
                  computed(dmTabs_, function (v) {
                    return String(v.length);
                  }),
                ],
              ),
            ],
          ),
        ]),

        // ---- Section 1: Quick Get ----
        View({ class: "space-y-4" }, [
          View({ class: "flex items-center gap-2" }, [
            View(
              {
                class: "text-lg font-semibold text-zinc-900 dark:text-zinc-50",
              },
              ["Quick Get"],
            ),
            View({ class: "text-xs text-zinc-400" }, ["快速获取网页"]),
          ]),

          // URL input row
          View({ class: "flex items-stretch gap-2" }, [
            View({ class: "flex-1" }, [
              Input({
                store: new Timeless.ui.InputCore({
                  value: getUrl_.value,
                  placeholder: "https://example.com",
                  onChange: function (value) {
                    getUrl_.as(value);
                  },
                }),
              }),
            ]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  disabled: getSubmitting_,
                  onClick: handleGet,
                }),
              },
              [
                computed(getSubmitting_, function (v) {
                  return v ? "请求中..." : "Get";
                }),
              ],
            ),
          ]),

          // Method selector
          View({ class: "flex items-center gap-2 flex-wrap" }, [
            View({ class: "text-xs text-zinc-500 mr-1" }, ["方式:"]),
            For({
              each: FETCH_METHODS,
              render(m) {
                const sel = computed(getFetchMode_, function (v) {
                  return v === m.value;
                });
                return View(
                  {
                    class: computed(sel, function (v) {
                      return v
                        ? "px-3 py-1.5 rounded-md text-xs font-medium bg-zinc-900 text-white cursor-pointer dark:bg-zinc-100 dark:text-zinc-900"
                        : "px-3 py-1.5 rounded-md text-xs font-medium bg-zinc-100 text-zinc-600 hover:bg-zinc-200 cursor-pointer dark:bg-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-700";
                    }),
                    title: m.desc,
                    onClick: function () {
                      getFetchMode_.as(m.value);
                    },
                  },
                  [m.label],
                );
              },
            }),
          ]),

          // HTTP Method selector (only for daemon mode)
          Show({
            when: computed(getFetchMode_, function (v) {
              return v === "daemon";
            }),
            ok: function () {
              return View({ class: "flex items-center gap-2" }, [
                View({ class: "text-xs text-zinc-500" }, ["HTTP Method:"]),
                HTTP_METHODS.map(function (m) {
                  const sel = computed(getHttpMethod_, function (v) {
                    return v === m.value;
                  });
                  return View(
                    {
                      class: computed(sel, function (v) {
                        return v
                          ? "px-2 py-1 rounded text-xs font-medium bg-zinc-800 text-white cursor-pointer dark:bg-zinc-200 dark:text-zinc-900"
                          : "px-2 py-1 rounded text-xs font-medium bg-zinc-100 text-zinc-600 hover:bg-zinc-200 cursor-pointer dark:bg-zinc-800 dark:text-zinc-400";
                      }),
                      onClick: function () {
                        getHttpMethod_.as(m.value);
                      },
                    },
                    [m.label],
                  );
                }),
              ]);
            },
          }),

          // Toggle advanced options
          View(
            {
              class:
                "text-xs text-zinc-400 cursor-pointer hover:text-zinc-600 dark:hover:text-zinc-300 select-none",
              onClick: function () {
                getShowAdvanced_.toggle();
              },
            },
            [
              computed(getShowAdvanced_, function (v) {
                return v ? "收起参数" : "请求参数...";
              }),
            ],
          ),

          // Advanced options panel
          Show({
            when: getShowAdvanced_,
            ok: function () {
              return View(
                {
                  class:
                    "space-y-3 p-4 rounded-lg border border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900/50",
                },
                [
                  // Headers
                  View({ class: "space-y-1" }, [
                    View({ class: "text-xs font-medium text-zinc-500" }, [
                      "Headers（每行 key: value）",
                    ]),
                    Textarea({
                      store: new Timeless.ui.InputCore({
                        value: getHeaders_.value,
                        placeholder:
                          "Authorization: Bearer xxx\nContent-Type: application/json",
                        onChange: function (value) {
                          getHeaders_.as(value);
                        },
                      }),
                    }),
                  ]),
                  // Body (only for daemon POST)
                  Show({
                    when: computed(getFetchMode_, function (v) {
                      return v === "daemon";
                    }),
                    ok: function () {
                      return View({ class: "space-y-1" }, [
                        View({ class: "text-xs font-medium text-zinc-500" }, [
                          "Body",
                        ]),
                        Textarea({
                          store: new Timeless.ui.InputCore({
                            value: getBody_.value,
                            placeholder: '{"key": "value"}',
                            onChange: function (value) {
                              getBody_.as(value);
                            },
                          }),
                        }),
                      ]);
                    },
                  }),
                  // User-Agent + Timeout
                  View({ class: "grid grid-cols-2 gap-3" }, [
                    View({ class: "space-y-1" }, [
                      View({ class: "text-xs text-zinc-500" }, ["User-Agent"]),
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: getUserAgent_.value,
                          placeholder: "auto",
                          onChange: function (value) {
                            getUserAgent_.as(value);
                          },
                        }),
                      }),
                    ]),
                    View({ class: "space-y-1" }, [
                      View({ class: "text-xs text-zinc-500" }, ["超时 (秒)"]),
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: getTimeout_.value,
                          placeholder: "30",
                          onChange: function (value) {
                            getTimeout_.as(value);
                          },
                        }),
                      }),
                    ]),
                  ]),
                ],
              );
            },
          }),

          // Error display
          Show({
            when: computed(getError_, function (v) {
              return !!v;
            }),
            ok: function () {
              return View(
                {
                  class:
                    "p-3 rounded-lg border border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/30 text-sm text-red-700 dark:text-red-300",
                },
                [
                  computed(getError_, function (v) {
                    return v;
                  }),
                ],
              );
            },
          }),

          // Task progress
          Show({
            when: computed(getTaskId_, function (v) {
              return !!v && !getResult_.value && !getError_.value;
            }),
            ok: function () {
              return View(
                {
                  class:
                    "flex items-center gap-2 text-xs text-zinc-500 dark:text-zinc-400",
                },
                [
                  View({
                    class:
                      "w-3 h-3 border-2 border-zinc-300 border-t-zinc-600 rounded-full animate-spin",
                  }),
                  computed(getTaskId_, function (v) {
                    return "任务 " + v + " 处理中...";
                  }),
                ],
              );
            },
          }),

          // Result display
          Show({
            when: computed(getResult_, function (v) {
              return !!v;
            }),
            ok: function () {
              return View(
                {
                  class:
                    "space-y-3 p-4 rounded-xl border border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950/30",
                },
                [
                  View({ class: "flex items-center gap-2" }, [
                    View(
                      {
                        class:
                          "text-sm font-semibold text-green-800 dark:text-green-200",
                      },
                      ["获取成功"],
                    ),
                    computed(getResult_, function (v) {
                      return methodBadge(v.method);
                    }),
                  ]),
                  View(
                    {
                      class:
                        "text-xs text-green-700 dark:text-green-300 space-y-1",
                    },
                    [
                      View({}, [
                        computed(getResult_, function (v) {
                          return "URL: " + (v.url || "-");
                        }),
                      ]),
                      View({}, [
                        computed(getResult_, function (v) {
                          return "标题: " + (v.Title || v.title || "-");
                        }),
                      ]),
                      View({}, [
                        computed(getResult_, function (v) {
                          var sz = v.htmlSize != null ? v.htmlSize : v.FileSize;
                          return "大小: " + formatBytes(sz);
                        }),
                      ]),
                      View({}, [
                        computed(getResult_, function (v) {
                          if (v.FilePath) return "路径: " + v.FilePath;
                          if (v.file_path) return "路径: " + v.file_path;
                          return null;
                        }),
                      ]),
                    ],
                  ),
                  // Preview toggle for daemon mode
                  Show({
                    when: computed(getResult_, function (v) {
                      return !!v.html;
                    }),
                    ok: function () {
                      const previewOpen_ = ref(false);
                      return View({ class: "space-y-2" }, [
                        View(
                          {
                            class:
                              "text-xs text-green-600 dark:text-green-400 cursor-pointer hover:underline",
                            onClick: function () {
                              previewOpen_.toggle();
                            },
                          },
                          [
                            computed(previewOpen_, function (v) {
                              return v ? "收起预览" : "查看 HTML 预览";
                            }),
                          ],
                        ),
                        Show({
                          when: previewOpen_,
                          ok: function () {
                            return View(
                              {
                                class:
                                  "max-h-64 overflow-auto rounded border border-green-200 dark:border-green-800 bg-white dark:bg-zinc-950 p-3",
                              },
                              [
                                View(
                                  {
                                    class:
                                      "text-[11px] font-mono text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap break-all",
                                  },
                                  [
                                    computed(getResult_, function (v) {
                                      var h = v.html || "";
                                      return h.length > 4000
                                        ? h.slice(0, 4000) + "\n\n... (截断)"
                                        : h;
                                    }),
                                  ],
                                ),
                              ],
                            );
                          },
                        }),
                      ]);
                    },
                  }),
                ],
              );
            },
          }),
        ]),

        // ---- Separator ----
        Separator({ class: "my-2" }),

        // ---- Section 2: Daemon Control ----
        View({ class: "space-y-4" }, [
          View({ class: "flex items-center gap-2" }, [
            View(
              {
                class: "text-lg font-semibold text-zinc-900 dark:text-zinc-50",
              },
              ["Daemon"],
            ),
            View({ class: "text-xs text-zinc-400" }, ["浏览器远程调试"]),
          ]),

          // Status + Actions row
          View({ class: "flex items-center gap-3 flex-wrap" }, [
            statusBadge(dmRunning_),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  size: "sm",
                  disabled: dmLoading_,
                  onClick: handleDaemonStart,
                }),
              },
              ["启动"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "secondary",
                  size: "sm",
                  disabled: dmLoading_,
                  onClick: handleDaemonStop,
                }),
              },
              ["停止"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  size: "sm",
                  disabled: dmLoading_,
                  onClick: handleDaemonRestart,
                }),
              },
              ["重启"],
            ),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "ghost",
                  size: "sm",
                  disabled: dmLoading_,
                  onClick: refreshDaemon,
                }),
              },
              ["刷新"],
            ),
          ]),

          // Daemon error
          Show({
            when: computed(dmError_, function (v) {
              return !!v;
            }),
            ok: function () {
              return View(
                {
                  class:
                    "p-3 rounded-lg border border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/30 text-sm text-red-700 dark:text-red-300",
                },
                [
                  computed(dmError_, function (v) {
                    return v;
                  }),
                ],
              );
            },
          }),

          // Debugger URL row
          Show({
            when: dmRunning_,
            ok: function () {
              return View({ class: "space-y-2" }, [
                // Load debugger URL button
                View({ class: "flex items-center gap-2" }, [
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick: loadDebuggerURL,
                      }),
                    },
                    ["获取 Remote Debugger URL"],
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "ghost",
                        size: "sm",
                        onClick: loadDaemonTabs,
                      }),
                    },
                    ["加载标签页"],
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "ghost",
                        size: "sm",
                        onClick: loadPageInfo,
                      }),
                    },
                    ["页面信息"],
                  ),
                ]),

                // Debugger URL display
                Show({
                  when: computed(dmDebuggerURL_, function (v) {
                    return !!v;
                  }),
                  ok: function () {
                    return View(
                      {
                        class:
                          "p-3 rounded-lg border border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900/50 space-y-1",
                      },
                      [
                        View({ class: "text-xs font-medium text-zinc-500" }, [
                          "CDP WebSocket URL",
                        ]),
                        View(
                          {
                            class:
                              "text-xs font-mono text-zinc-700 dark:text-zinc-300 break-all select-all",
                          },
                          [
                            computed(dmDebuggerURL_, function (v) {
                              return v;
                            }),
                          ],
                        ),
                        View({ class: "text-[10px] text-zinc-400" }, [
                          "将此 URL 粘贴到兼容 CDP 的调试器（如 Chrome DevTools、Playwright）中，即可远程控制浏览器。",
                        ]),
                      ],
                    );
                  },
                }),

                // Tabs list
                Show({
                  when: computed(dmTabs_, function (v) {
                    return v.length > 0;
                  }),
                  ok: function () {
                    return View({ class: "space-y-2" }, [
                      View({ class: "text-xs font-medium text-zinc-500" }, [
                        computed(dmTabs_, function (v) {
                          return "浏览器标签页 (" + v.length + ")";
                        }),
                      ]),
                      View({ class: "space-y-1" }, [
                        For({
                          each: dmTabs_,
                          render: function (tab, idx) {
                            return View(
                              {
                                class:
                                  "flex items-center gap-2 text-xs text-zinc-600 dark:text-zinc-400 p-2 rounded bg-white dark:bg-zinc-950 border border-zinc-100 dark:border-zinc-800",
                              },
                              [
                                View({ class: "text-zinc-400 font-mono w-5" }, [
                                  String(idx + 1),
                                ]),
                                View({ class: "truncate flex-1" }, [
                                  tab.title || tab.url || "-",
                                ]),
                                View(
                                  {
                                    class:
                                      "text-[10px] text-zinc-400 truncate max-w-[300px]",
                                  },
                                  [tab.url || ""],
                                ),
                              ],
                            );
                          },
                        }),
                      ]),
                    ]);
                  },
                }),

                // Page info
                Show({
                  when: computed(dmPageInfo_, function (v) {
                    return !!v;
                  }),
                  ok: function () {
                    return View({ class: "space-y-1" }, [
                      View({ class: "text-xs font-medium text-zinc-500" }, [
                        "当前页面",
                      ]),
                      View(
                        {
                          class:
                            "p-3 rounded-lg bg-white dark:bg-zinc-950 border border-zinc-100 dark:border-zinc-800 text-xs font-mono text-zinc-600 dark:text-zinc-400 whitespace-pre-wrap max-h-48 overflow-auto",
                        },
                        [
                          computed(dmPageInfo_, function (v) {
                            return JSON.stringify(v, null, 2);
                          }),
                        ],
                      ),
                    ]);
                  },
                }),
              ]);
            },
          }),
        ]),
      ]),
    ],
  );
}

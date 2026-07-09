import {
  savePage, getTask, getTaskLogs,
  daemonStatus, daemonStart, daemonStop, daemonRestart,
  remoteDaemonStart, remoteDaemonStop, daemonTabs, daemonPageInfo,
  listApps, installApp, removeApp, updateApp, runApp, listAppTasks, getAppTask,
} from "./index.model.js";

const FETCH_MODES = [
  { value: "http", label: "HTTP" },
  { value: "cdp", label: "CDP" },
  { value: "api", label: "API" },
];

const TAB_KEYS = ["request", "daemon", "appstore"];
const TAB_LABELS = ["URL请求", "Daemon管理", "App管理"];

export default function ToolsPageView(props) {
  // ---- shared ----
  const activeTab_ = ref("request");

  // ---- URL Request state ----
  const reqUrl_ = ref("");
  const reqMode_ = ref("http");
  const reqHeaders_ = ref("");
  const reqUserAgent_ = ref("");
  const reqTimeout_ = ref("");
  const reqFilename_ = ref("");
  const reqSubmitting_ = ref(false);
  const reqResult_ = ref(null);
  const reqError_ = ref("");
  const reqTaskId_ = ref("");
  const reqPolling_ = ref(null);
  const reqLogs_ = refarr([]);
  const reqLogOpen_ = ref(false);
  const reqStatusEvtSrc_ = ref(null);
  const reqLogEvtSrc_ = ref(null);

  // ---- Daemon state ----
  const dmName_ = ref("default");
  const dmRunning_ = ref(false);
  const dmLoading_ = ref(false);
  const dmError_ = ref("");
  const dmTabs_ = refarr([]);
  const dmPageInfo_ = ref(null);
  // remote
  const dmRemoteProfile_ = ref("");
  const dmRemoteProxy_ = ref("");
  const dmRemoteTimeout_ = ref("");
  const dmRemoteResult_ = ref(null);

  // ---- Appstore state ----
  const appList_ = refarr([]);
  const appLoading_ = ref(false);
  const appError_ = ref("");
  const appTasks_ = refarr([]);
  const appInstallName_ = ref("");
  const appActionResult_ = ref("");

  // ==================== URL Request Methods ====================

  async function handleSave() {
    const url = reqUrl_.value.trim();
    if (!url) {
      reqError_.as("请输入 URL");
      return;
    }
    reqSubmitting_.as(true);
    reqError_.as("");
    reqResult_.as(null);
    reqTaskId_.as("");
    reqLogs_.as([]);
    reqLogOpen_.as(true);

    // Close any existing SSE connections
    closeSseConnections();

    const opts = { fetch_mode: reqMode_.value };
    if (reqUserAgent_.value) opts.user_agent = reqUserAgent_.value;
    if (reqFilename_.value) opts.filename = reqFilename_.value;
    if (reqTimeout_.value) {
      const t = parseInt(reqTimeout_.value, 10);
      if (!isNaN(t)) opts.timeout = t * 1e9; // seconds -> nanoseconds
    }
    const headersStr = reqHeaders_.value.trim();
    if (headersStr) {
      const headers = {};
      for (const line of headersStr.split("\n")) {
        const idx = line.indexOf(":");
        if (idx > 0) headers[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
      }
      if (Object.keys(headers).length) opts.headers = headers;
    }

    const r = await savePage(url, opts);
    reqSubmitting_.as(false);

    if (r.error) {
      reqError_.as(String(r.error));
      return;
    }
    reqTaskId_.as(r.data.task_id);
    startLogStream(r.data.task_id);
  }

  function startLogStream(taskId) {
    // Open log SSE stream
    const logSrc = new EventSource(`/api/v1/tasks/${taskId}/logs/stream`);
    reqLogEvtSrc_.as(logSrc);
    logSrc.onmessage = function (event) {
      try {
        const entry = JSON.parse(event.data);
        reqLogs_.as([...reqLogs_.value, entry]);
      } catch (_) { /* ignore parse errors */ }
    };
    logSrc.onerror = function () {
      logSrc.close();
      reqLogEvtSrc_.as(null);
    };

    // Open status SSE stream (replaces polling)
    const statusSrc = new EventSource(`/api/v1/tasks/${taskId}/events`);
    reqStatusEvtSrc_.as(statusSrc);
    statusSrc.onmessage = function (event) {
      try {
        const data = JSON.parse(event.data);
        if (data.status === "completed") {
          reqResult_.as(data.result);
          statusSrc.close();
          reqStatusEvtSrc_.as(null);
        } else if (data.status === "failed" || data.status === "cancelled") {
          reqError_.as(data.error || data.status);
          statusSrc.close();
          reqStatusEvtSrc_.as(null);
        }
      } catch (_) { /* ignore parse errors */ }
    };
    statusSrc.onerror = function () {
      statusSrc.close();
      reqStatusEvtSrc_.as(null);
      // Fallback: poll once to get final state
      getTask(taskId).then(function (r2) {
        if (!r2.error && r2.data) {
          if (r2.data.status === "completed") reqResult_.as(r2.data.result);
          else if (r2.data.status === "failed" || r2.data.status === "cancelled") reqError_.as(r2.data.error || r2.data.status);
        }
      });
    };
  }

  function closeSseConnections() {
    if (reqLogEvtSrc_.value) { reqLogEvtSrc_.value.close(); reqLogEvtSrc_.as(null); }
    if (reqStatusEvtSrc_.value) { reqStatusEvtSrc_.value.close(); reqStatusEvtSrc_.as(null); }
  }

  /** Phase color mapping */ function phaseColor(p) {
    switch (p) {
      case "fetch": return "#3b82f6";
      case "inline": return "#a855f7";
      case "save": return "#22c55e";
      case "engine": return "#6b7280";
      default: return "#9ca3af";
    }
  }

  // ==================== Daemon Methods ====================

  async function refreshDaemon() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStatus(dmName_.value);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error)); return; }
    dmRunning_.as(!!r.data.running);
  }

  async function handleDaemonStart() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStart(dmName_.value);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error)); return; }
    dmRunning_.as(true);
  }

  async function handleDaemonStop() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonStop(dmName_.value);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error) || "daemon not running"); return; }
    dmRunning_.as(false);
  }

  async function handleDaemonRestart() {
    dmLoading_.as(true);
    dmError_.as("");
    const r = await daemonRestart(dmName_.value);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error)); return; }
    dmRunning_.as(true);
  }

  async function loadDaemonTabs() {
    const r = await daemonTabs(dmName_.value);
    if (!r.error && r.data) dmTabs_.as(r.data.tabs || []);
  }

  async function loadPageInfo() {
    const r = await daemonPageInfo(dmName_.value);
    if (!r.error) dmPageInfo_.as(r.data);
  }

  async function handleRemoteStart() {
    dmLoading_.as(true);
    dmError_.as("");
    dmRemoteResult_.as(null);
    const opts = {};
    if (dmRemoteProfile_.value) opts.profileName = dmRemoteProfile_.value;
    if (dmRemoteProxy_.value) opts.proxyCountryCode = dmRemoteProxy_.value;
    if (dmRemoteTimeout_.value) {
      const t = parseInt(dmRemoteTimeout_.value, 10);
      if (!isNaN(t)) opts.timeout = t;
    }
    const r = await remoteDaemonStart(dmName_.value, opts);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error)); return; }
    dmRemoteResult_.as(r.data);
    dmRunning_.as(true);
  }

  async function handleRemoteStop() {
    dmLoading_.as(true);
    dmError_.as("");
    const browserId = dmRemoteResult_.value?.browserId || "";
    const r = await remoteDaemonStop(dmName_.value, browserId);
    dmLoading_.as(false);
    if (r.error) { dmError_.as(String(r.error)); return; }
    dmRunning_.as(false);
    dmRemoteResult_.as(null);
  }

  // ==================== Appstore Methods ====================

  async function loadApps() {
    appLoading_.as(true);
    appError_.as("");
    const r = await listApps(true);
    appLoading_.as(false);
    if (r.error) { appError_.as(String(r.error)); return; }
    const data = r.data || {};
    appList_.as(data.apps || data.installed || []);
  }

  async function loadAppTasks() {
    const r = await listAppTasks();
    if (!r.error && r.data) {
      appTasks_.as(r.data.tasks || []);
    }
  }

  async function handleInstallApp() {
    const name = appInstallName_.value.trim();
    if (!name) return;
    appLoading_.as(true);
    const r = await installApp(name);
    appLoading_.as(false);
    appActionResult_.as(r.error ? `安装失败: ${r.error}` : `安装 ${name} 已提交`);
    if (!r.error) { appInstallName_.as(""); loadApps(); loadAppTasks(); }
  }

  async function handleRemoveApp(name) {
    appLoading_.as(true);
    const r = await removeApp(name);
    appLoading_.as(false);
    appActionResult_.as(r.error ? `移除失败: ${r.error}` : `${name} 已移除`);
    if (!r.error) { loadApps(); loadAppTasks(); }
  }

  async function handleUpdateApp(name) {
    appLoading_.as(true);
    const r = await updateApp(name);
    appLoading_.as(false);
    appActionResult_.as(r.error ? `更新失败: ${r.error}` : `${name || "all"} 更新已提交`);
    if (!r.error) loadAppTasks();
  }

  async function handleRunApp(name) {
    appLoading_.as(true);
    const r = await runApp(name);
    appLoading_.as(false);
    appActionResult_.as(r.error ? `运行失败: ${r.error}` : `${name} 已启动`);
  }

  // ==================== Log Panel ====================

  function renderLogPanel() {
    return View({ dataset: { t: "tools-page-request-workflow-log-panel-card-stack-工作流日志-string-收起-or-展开" }, class: "space-y-2 border border-zinc-200 dark:border-zinc-800 rounded-lg overflow-hidden" }, [
      // Header with toggle
      View({
        dataset: { t: "tools-page-request-workflow-log-panel-row-工作流日志-string-收起-or-展开" },
        class: "flex items-center justify-between px-3 py-2 bg-zinc-50 dark:bg-zinc-900 cursor-pointer",
        onClick() { reqLogOpen_.toggle(); },
      }, [
        View({ dataset: { t: "tools-page-request-workflow-log-panel-row-工作流日志-string" }, class: "flex items-center gap-2" }, [
          View({ dataset: { t: "tools-page-request-workflow-log-panel-工作流日志-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["工作流日志"]),
          View({
            dataset: { t: "tools-page-request-workflow-log-panel-avatar-or-badge-row-string-text" },
            class: "inline-flex items-center px-1.5 py-0.5 rounded-full text-xs font-medium bg-zinc-200 text-zinc-600 dark:bg-zinc-700 dark:text-zinc-300",
          }, [computed(reqLogs_, (v) => String(v.length))]),
        ]),
        computed(reqLogOpen_, (v) => v ? "收起" : "展开"),
      ]),
      // Log entries
      Show({
        when: reqLogOpen_,
        ok() {
          return View({
            dataset: { t: "tools-page-request-workflow-log-panel-stack-scroll-area-req-logs_-list" },
            class: "p-3 max-h-72 overflow-auto space-y-2 bg-white dark:bg-zinc-950",
          }, [
            Show({
              when: computed(reqLogs_, (v) => v.length === 0),
              ok() {
                return View({ dataset: { t: "tools-page-request-workflow-log-panel-等待日志-text" }, class: "text-xs text-zinc-400 py-4 text-center" }, ["等待日志..."]);

              },
            }),
            For({ each: reqLogs_, render(entry) {
              const color = phaseColor(entry.phase);
              const detailOpen_ = ref(false);
              const hasData = entry.data && Object.keys(entry.data).length > 0;
              return View({ dataset: { t: "tools-page-request-workflow-log-panel-row-entry-phase-info-entry-timestamp-entry-message-收起详情-or-详情-text" }, class: "flex gap-2 text-xs" }, [
                // Timeline dot
                View({
                  dataset: { t: "tools-page-request-log-entry-phase-color-dot" },
                  style: `width:8px;height:8px;border-radius:50%;background:${color};margin-top:3px;flex-shrink:0;`,
                }),
                View({ dataset: { t: "tools-page-request-workflow-log-panel-stack-row-entry-phase-info-entry-timestamp-entry-message-收起详情-or-详情" }, class: "flex-1 min-w-0 space-y-0.5" }, [
                  View({ dataset: { t: "tools-page-request-workflow-log-panel-row-entry-phase-info-entry-timestamp" }, class: "flex items-center gap-2 flex-wrap" }, [
                    View({ dataset: { t: "tools-page-request-workflow-log-panel-entry-phase-info" }, class: "font-medium text-zinc-700 dark:text-zinc-300" }, [entry.phase || "info"]),
                    View({ dataset: { t: "tools-page-request-workflow-log-panel-entry-timestamp" }, class: "text-zinc-400 text-[10px]" }, [entry.timestamp || ""]),
                  ]),
                  View({ dataset: { t: "tools-page-request-workflow-log-panel-entry-message" }, class: "text-zinc-600 dark:text-zinc-400" }, [entry.message || ""]),
                  hasData ? View({
                    dataset: { t: "tools-page-request-workflow-log-panel-收起详情-or-详情" },
                    class: "text-[10px] text-zinc-400 cursor-pointer hover:text-zinc-600",
                    onClick() { detailOpen_.toggle(); },
                  }, [computed(detailOpen_, (v) => v ? "收起详情" : "详情")]) : null,
                  Show({
                    when: detailOpen_,
                    ok() {
                      const data = entry.data || {};
                      const rows = Object.keys(data).map(function (k) {
                        const val = data[k];
                        const str = typeof val === "object" ? JSON.stringify(val) : String(val ?? "");
                        return View({ dataset: { t: "tools-page-request-workflow-log-panel-row-k-value-str-value" }, class: "flex gap-2" }, [
                          View({ dataset: { t: "tools-page-request-workflow-log-panel-monospace-row-k-value" }, class: "text-zinc-500 font-mono flex-shrink-0" }, [k + ":"]),
                          View({ dataset: { t: "tools-page-request-workflow-log-panel-monospace-str-value" }, class: "text-zinc-700 dark:text-zinc-300 font-mono break-all" }, [str]),
                        ]);
                      });
                      return View({ dataset: { t: "tools-page-request-workflow-log-panel-stack-rows-value" }, class: "mt-1 p-2 rounded bg-zinc-50 dark:bg-zinc-900 space-y-0.5" }, rows);
                    },
                  }),
                ]),
              ]);
            } }),
          ]);
        },
      }),
    ]);
  }

  function formatBytes(n) {
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / (1024 * 1024)).toFixed(1) + " MB";
  }

  // ==================== Tab UI Helpers ====================

  function tabBtn(key, label) {
    const isActive = computed(activeTab_, (t) => t === key);
    return View({
      dataset: { t: "tools-page-tab-btn-label-value" },
      class: computed(isActive, (v) =>
        v
          ? "px-4 py-2 text-sm font-medium border-b-2 border-zinc-900 text-zinc-900 cursor-pointer dark:border-zinc-100 dark:text-zinc-100"
          : "px-4 py-2 text-sm font-medium border-b-2 border-transparent text-zinc-500 hover:text-zinc-700 cursor-pointer dark:text-zinc-400 dark:hover:text-zinc-200"
      ),
      onClick() { activeTab_.as(key); },
    }, [label]);
  }

  function statusBadge(running) {
    return View({
      dataset: { t: "tools-page-status-badge-Running-or-Stopped" },
      class: computed(running, (v) =>
        v
          ? "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
          : "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400"
      ),
    }, [computed(running, (v) => v ? "Running" : "Stopped")]);
  }

  // ==================== Full View ====================

  return ScrollView({
    store: new Timeless.ui.ScrollViewCore({}),
    class: "h-full",
    onMounted() { refreshDaemon(); loadApps(); },
    onUnmounted() {
      if (reqPolling_.value) clearInterval(reqPolling_.value);
      closeSseConnections();
    },
  }, [
    View({ dataset: { t: "tools-page-stack-工具箱-tab-btn-tab-btn-tab-btn" }, class: "p-6 max-w-3xl space-y-6" }, [
      // ---- Header ----
      View({ dataset: { t: "tools-page-工具箱-heading" }, class: "text-2xl font-bold tracking-tight text-zinc-900 dark:text-zinc-50" }, ["工具箱"]),

      // ---- Tab Bar ----
      View({ dataset: { t: "tools-page-row-tab-btn-tab-btn-tab-btn" }, class: "flex border-b border-zinc-200 dark:border-zinc-800" }, [
        tabBtn("request", "URL请求"),
        tabBtn("daemon", "Daemon管理"),
        tabBtn("appstore", "App管理"),
      ]),

      // ---- Tab 1: URL Request ----
      Show({
        when: computed(activeTab_, (t) => t === "request"),
        ok() {
          return View({ dataset: { t: "tools-page-stack-URL-input-请求方式-row-button" }, class: "space-y-4 pt-2" }, [
            // URL input
            View({ dataset: { t: "tools-page-stack-URL-input" }, class: "space-y-1.5" }, [
              View({ dataset: { t: "tools-page-URL-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["URL"]),
              Input({
                store: new Timeless.ui.InputCore({
                  value: reqUrl_.value,
                  placeholder: "https://example.com",
                  onChange(value) { reqUrl_.as(value); },
                }),
              }),
            ]),

            // Fetch mode selector
            View({ dataset: { t: "tools-page-request-tab-fetch-mode-field" }, class: "space-y-1.5" }, [
              View({ dataset: { t: "tools-page-请求方式-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["请求方式"]),
              View({ dataset: { t: "tools-page-request-tab-fetch-mode-buttons" }, class: "flex gap-2" }, FETCH_MODES.map((m) => {
                const sel = computed(reqMode_, (v) => v === m.value);
                return View({
                  dataset: { t: "tools-page-m-label" },
                  class: computed(sel, (v) =>
                    v
                      ? "px-3 py-1.5 rounded-md text-sm font-medium bg-zinc-900 text-white cursor-pointer dark:bg-zinc-100 dark:text-zinc-900"
                      : "px-3 py-1.5 rounded-md text-sm font-medium bg-zinc-100 text-zinc-600 hover:bg-zinc-200 cursor-pointer dark:bg-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-700"
                  ),
                  onClick() { reqMode_.as(m.value); },
                }, [m.label]);
              })),
            ]),

            // Advanced options (collapsible)
            (() => {
              const open_ = ref(false);
              return View({ dataset: { t: "tools-page-stack-收起高级选项-or-展开高级选项" }, class: "space-y-2" }, [
                View({
                  dataset: { t: "tools-page-收起高级选项-or-展开高级选项-text" },
                  class: "text-xs text-zinc-400 cursor-pointer hover:text-zinc-600 dark:hover:text-zinc-300",
                  onClick() { open_.toggle(); },
                }, [computed(open_, (v) => v ? "收起高级选项" : "展开高级选项")]),
                Show({
                  when: open_,
                  ok() {
                    return View({ dataset: { t: "tools-page-card-stack-自定义-Headers-每行-key-value-textarea-User-Agent-input-超时-秒-input-文件名-input" }, class: "space-y-3 p-3 border rounded-lg border-zinc-200 dark:border-zinc-800" }, [
                      View({ dataset: { t: "tools-page-自定义-Headers-每行-key-value-text" }, class: "text-xs font-medium text-zinc-500" }, ["自定义 Headers（每行 key: value）"]),
                      Textarea({
                        store: new Timeless.ui.InputCore({
                          value: reqHeaders_.value,
                          placeholder: "Authorization: Bearer xxx\nX-Custom: value",
                          onChange(value) { reqHeaders_.as(value); },
                        }),
                      }),
                      View({ dataset: { t: "tools-page-grid-User-Agent-input-超时-秒-input-文件名-input" }, class: "grid grid-cols-3 gap-3" }, [
                        View({ dataset: { t: "tools-page-stack-User-Agent-input" }, class: "space-y-1" }, [
                          View({ dataset: { t: "tools-page-User-Agent-text" }, class: "text-xs text-zinc-500" }, ["User-Agent"]),
                          Input({
                        store: new Timeless.ui.InputCore({
                          value: reqUserAgent_.value,
                          placeholder: "auto",
                          onChange(value) { reqUserAgent_.as(value); },
                        }),
                      }),
                        ]),
                        View({ dataset: { t: "tools-page-stack-超时-秒-input" }, class: "space-y-1" }, [
                          View({ dataset: { t: "tools-page-超时-秒-text" }, class: "text-xs text-zinc-500" }, ["超时(秒)"]),
                          Input({
                        store: new Timeless.ui.InputCore({
                          value: reqTimeout_.value,
                          placeholder: "60",
                          onChange(value) { reqTimeout_.as(value); },
                        }),
                      }),
                        ]),
                        View({ dataset: { t: "tools-page-stack-文件名-input" }, class: "space-y-1" }, [
                          View({ dataset: { t: "tools-page-文件名-text" }, class: "text-xs text-zinc-500" }, ["文件名"]),
                          Input({
                        store: new Timeless.ui.InputCore({
                          value: reqFilename_.value,
                          placeholder: "auto",
                          onChange(value) { reqFilename_.as(value); },
                        }),
                      }),
                        ]),
                      ]),
                    ]);
                  },
                }),
              ]);
            })(),

            // Submit
            View({ dataset: { t: "tools-page-button" }, class: "pt-2" }, [
              Button({
                store: new Timeless.ui.ButtonCore({
                  disabled: reqSubmitting_,
                  onClick: handleSave,
                }),
              }, [computed(reqSubmitting_, (v) => v ? "提交中..." : "发起请求")]),
            ]),

            // Error
            Show({
              when: computed(reqError_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-error-panel-v-value-text" }, class: "p-3 rounded-md border border-red-200 bg-red-50 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300" }, [
                  computed(reqError_, (v) => v),
                ]);
              },
            }),

            // Result (enhanced with more fields)
            Show({
              when: computed(reqResult_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-success-card-stack-保存完成-路径-v-file-path-v-file_path-大小-format-bytes-资源数-v-resource-count-computed-value" }, class: "space-y-2 p-3 rounded-lg border border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950" }, [
                  View({ dataset: { t: "tools-page-success-保存完成-text" }, class: "text-sm font-medium text-green-800 dark:text-green-200" }, ["保存完成"]),
                  View({ dataset: { t: "tools-page-success-stack-路径-v-file-path-v-file_path-大小-format-bytes-资源数-v-resource-count-computed-value-text" }, class: "text-xs text-green-700 dark:text-green-300 space-y-1" }, [
                    View({ dataset: { t: "tools-page-路径-v-file-path-v-file_path" } }, [computed(reqResult_, (v) => `路径: ${v?.FilePath || v?.file_path || "-"}`)]),
                    View({ dataset: { t: "tools-page-大小-format-bytes" } }, [computed(reqResult_, (v) => `大小: ${v?.FileSize != null ? formatBytes(v.FileSize) : "-"}`)]),
                    View({ dataset: { t: "tools-page-资源数-v-resource-count" } }, [computed(reqResult_, (v) => `资源数: ${v?.ResourceCount != null ? v.ResourceCount : "-"}`)]),
                    View({ dataset: { t: "tools-page-request-result-duration-row" } }, [computed(reqResult_, (v) => {
                      const d = v?.Duration;
                      if (d == null) return `耗时: -`;
                      const sec = typeof d === "number" ? (d / 1e9).toFixed(1) : d;
                      return `耗时: ${sec}s`;
                    })]),
                  ]),
                ]);
              },
            }),

            // Task progress indicator
            Show({
              when: computed(reqTaskId_, (v) => !!v && !reqResult_.value && !reqError_.value),
              ok() {
                return View({ dataset: { t: "tools-page-任务-进行中-v-value-text" }, class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
                  computed(reqTaskId_, (v) => `任务 ${v} 进行中...`),
                ]);
              },
            }),

            // ---- Workflow Log Panel ----
            Show({
              when: computed(reqLogs_, (v) => v.length > 0 || reqTaskId_.value),
              ok: renderLogPanel,
            }),
          ]);
        },
      }),

      // ---- Tab 2: Daemon Management ----
      Show({
        when: computed(activeTab_, (t) => t === "daemon"),
        ok() {
          return View({ dataset: { t: "tools-page-stack-Daemon-input-status-badge-button-button-button-button-button-button-远程-Daemon-Profile-Name-input-Proxy-Country-如-de-input-Timeout-秒-input-button-button" }, class: "space-y-6 pt-2" }, [
            // Daemon name + status
            View({ dataset: { t: "tools-page-row-Daemon-input-status-badge" }, class: "flex items-center gap-3 flex-wrap" }, [
              View({ dataset: { t: "tools-page-Daemon-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["Daemon:"]),
              Input({
                store: new Timeless.ui.InputCore({
                  value: dmName_.value,
                  placeholder: "default",
                  onChange(value) { dmName_.as(value); },
                }),
              }),
              statusBadge(dmRunning_),
            ]),

            // Action buttons
            View({ dataset: { t: "tools-page-row-button-button-button-button" }, class: "flex items-center gap-2 flex-wrap" }, [
              Button({
                store: new Timeless.ui.ButtonCore({
                  disabled: dmLoading_,
                  onClick: handleDaemonStart,
                }),
              }, ["启动"]),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "secondary",
                  disabled: dmLoading_,
                  onClick: handleDaemonStop,
                }),
              }, ["停止"]),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  // disabled: dmLoading_,
                  onClick: handleDaemonRestart,
                }),
              }, ["重启"]),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "ghost",
                  // disabled: dmLoading_,
                  onClick: refreshDaemon,
                }),
              }, ["刷新状态"]),
            ]),

            // Error
            Show({
              when: computed(dmError_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-error-panel-v-value-text-2" }, class: "p-3 rounded-md border border-red-200 bg-red-50 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300" }, [
                  computed(dmError_, (v) => v),
                ]);
              },
            }),

            // Tabs & Page Info
            View({ dataset: { t: "tools-page-row-button-button" }, class: "flex items-center gap-2" }, [
              Button({
                store: new Timeless.ui.ButtonCore({ variant: "outline", onClick: loadDaemonTabs }),
              }, ["加载标签页"]),
              Button({
                store: new Timeless.ui.ButtonCore({ variant: "outline", onClick: loadPageInfo }),
              }, ["加载页面信息"]),
            ]),

            // Tabs list
            Show({
              when: computed(dmTabs_, (v) => v.length > 0),
              ok() {
                return View({ dataset: { t: "tools-page-stack-浏览器标签页-dm-tabs_-list" }, class: "space-y-2" }, [
                  View({ dataset: { t: "tools-page-浏览器标签页-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["浏览器标签页"]),
                  View({ dataset: { t: "tools-page-stack-dm-tabs_-list" }, class: "space-y-1" }, [
                    For({ each: dmTabs_, render(tab, idx) {
                      return View({ dataset: { t: "tools-page-idx-value-tab-title-tab-url-text" }, class: "text-xs text-zinc-600 dark:text-zinc-400 p-2 rounded bg-zinc-50 dark:bg-zinc-900" }, [
                        `${idx + 1}. ${tab.title || tab.url || JSON.stringify(tab)}`,
                      ]);
                    } }),
                  ]),
                ]);
              },
            }),

            // Page info
            Show({
              when: computed(dmPageInfo_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-stack-当前页面信息-computed-value" }, class: "space-y-2" }, [
                  View({ dataset: { t: "tools-page-当前页面信息-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["当前页面信息"]),
                  View({
                    dataset: { t: "tools-page-monospace-scroll-area-text" },
                    class: "p-3 rounded-lg bg-zinc-50 dark:bg-zinc-900 text-xs text-zinc-600 dark:text-zinc-400 font-mono whitespace-pre-wrap max-h-48 overflow-auto",
                  }, [computed(dmPageInfo_, (v) => JSON.stringify(v, null, 2))]),
                ]);
              },
            }),

            // ---- Remote Daemon ----
            View({ dataset: { t: "tools-page-stack-远程-Daemon-Profile-Name-input-Proxy-Country-如-de-input-Timeout-秒-input-button-button" }, class: "pt-4 border-t border-zinc-200 dark:border-zinc-800 space-y-3" }, [
              View({ dataset: { t: "tools-page-远程-Daemon-heading" }, class: "text-sm font-semibold text-zinc-700 dark:text-zinc-300" }, ["远程 Daemon"]),
              View({ dataset: { t: "tools-page-grid-Profile-Name-input-Proxy-Country-如-de-input-Timeout-秒-input" }, class: "grid grid-cols-3 gap-3" }, [
                View({ dataset: { t: "tools-page-stack-Profile-Name-input" }, class: "space-y-1" }, [
                  View({ dataset: { t: "tools-page-Profile-Name-text" }, class: "text-xs text-zinc-500" }, ["Profile Name"]),
                  Input({
                    store: new Timeless.ui.InputCore({
                      value: dmRemoteProfile_.value,
                      placeholder: "my-profile",
                      onChange(value) { dmRemoteProfile_.as(value); },
                    }),
                  }),
                ]),
                View({ dataset: { t: "tools-page-stack-Proxy-Country-如-de-input" }, class: "space-y-1" }, [
                  View({ dataset: { t: "tools-page-Proxy-Country-如-de-text" }, class: "text-xs text-zinc-500" }, ["Proxy Country (如 de)"]),
                  Input({
                    store: new Timeless.ui.InputCore({
                      value: dmRemoteProxy_.value,
                      placeholder: "de",
                      onChange(value) { dmRemoteProxy_.as(value); },
                    }),
                  }),
                ]),
                View({ dataset: { t: "tools-page-stack-Timeout-秒-input" }, class: "space-y-1" }, [
                  View({ dataset: { t: "tools-page-Timeout-秒-text" }, class: "text-xs text-zinc-500" }, ["Timeout (秒)"]),
                  Input({
                    store: new Timeless.ui.InputCore({
                      value: dmRemoteTimeout_.value,
                      placeholder: "120",
                      onChange(value) { dmRemoteTimeout_.as(value); },
                    }),
                  }),
                ]),
              ]),
              View({ dataset: { t: "tools-page-row-button-button-2" }, class: "flex items-center gap-2" }, [
                Button({
                  store: new Timeless.ui.ButtonCore({ disabled: dmLoading_, onClick: handleRemoteStart }),
                }, ["启动远程"]),
                Button({
                  store: new Timeless.ui.ButtonCore({ variant: "secondary", disabled: dmLoading_, onClick: handleRemoteStop }),
                }, ["停止远程"]),
              ]),
              Show({
                when: computed(dmRemoteResult_, (v) => !!v),
                ok() {
                  return View({
                    dataset: { t: "tools-page-monospace-text" },
                    class: "p-3 rounded-lg bg-zinc-50 dark:bg-zinc-900 text-xs text-zinc-600 dark:text-zinc-400 font-mono whitespace-pre-wrap",
                  }, [computed(dmRemoteResult_, (v) => JSON.stringify(v, null, 2))]);
                },
              }),
            ]),
          ]);
        },
      }),

      // ---- Tab 3: Appstore ----
      Show({
        when: computed(activeTab_, (t) => t === "appstore"),
        ok() {
          return View({ dataset: { t: "tools-page-stack-button-button-input-button" }, class: "space-y-4 pt-2" }, [
            // Actions row
            View({ dataset: { t: "tools-page-row-button-button-3" }, class: "flex items-center gap-3 flex-wrap" }, [
              Button({
                store: new Timeless.ui.ButtonCore({
                  // disabled: appLoading_,
                  onClick: loadApps,
                }),
              }, ["刷新应用列表"]),
              Button({
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  // disabled: appLoading_,
                  onClick: loadAppTasks,
                }),
              }, ["刷新任务"]),
            ]),

            // Install section
            View({ dataset: { t: "tools-page-row-input-button" }, class: "flex items-center gap-2" }, [
              Input({
                store: new Timeless.ui.InputCore({
                  value: appInstallName_.value,
                  placeholder: "应用名 (如 yt-dlp)",
                  onChange(value) { appInstallName_.as(value); },
                }),
              }),
              Button({
                store: new Timeless.ui.ButtonCore({ disabled: appLoading_, onClick: handleInstallApp }),
              }, ["安装"]),
            ]),

            // Action result
            Show({
              when: computed(appActionResult_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-v-value-text" }, class: "text-xs text-zinc-600 dark:text-zinc-400 p-2 rounded bg-zinc-50 dark:bg-zinc-900" }, [
                  computed(appActionResult_, (v) => v),
                ]);
              },
            }),

            // Error
            Show({
              when: computed(appError_, (v) => !!v),
              ok() {
                return View({ dataset: { t: "tools-page-error-panel-v-value-text-3" }, class: "p-3 rounded-md border border-red-200 bg-red-50 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300" }, [
                  computed(appError_, (v) => v),
                ]);
              },
            }),

            // Loading
            Show({
              when: appLoading_,
              ok() { return View({ dataset: { t: "tools-page-加载中-text" }, class: "text-sm text-zinc-500" }, ["加载中..."]); },
            }),

            // App list
            Show({
              when: computed(appList_, (v) => v.length > 0),
              ok() {
                return View({ dataset: { t: "tools-page-stack-应用列表-v-length-app-list_-list" }, class: "space-y-3" }, [
                  View({ dataset: { t: "tools-page-应用列表-v-length-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, [
                    computed(appList_, (v) => `应用列表 (${v.length})`),
                  ]),
                  For({ each: appList_, render(app) {
                    const name = app.name || app.Name || "-";
                    const desc = app.description || app.Description || "";
                    const version = app.version || app.Version || "";
                    const installed = app.installed_at || app.InstalledAt || app.binary_path || app.BinaryPath;
                    return View({
                      dataset: { t: "tools-page-card-stack-name-value-v-version-value-已安装-button-button-button-or-button-desc-value" },
                      class: "p-3 rounded-lg border border-zinc-200 dark:border-zinc-800 space-y-2",
                    }, [
                      View({ dataset: { t: "tools-page-row-name-value-v-version-value-已安装-button-button-button-or-button" }, class: "flex items-center justify-between" }, [
                        View({ dataset: { t: "tools-page-row-name-value-v-version-value-已安装" }, class: "flex items-center gap-2" }, [
                          View({ dataset: { t: "tools-page-name-value-text" }, class: "text-sm font-medium text-zinc-900 dark:text-zinc-100" }, [name]),
                          version ? View({ dataset: { t: "tools-page-v-version-value-text" }, class: "text-xs text-zinc-400" }, [`v${version}`]) : null,
                          installed
                            ? View({ dataset: { t: "tools-page-success-row-已安装-text" }, class: "inline-flex px-1.5 py-0.5 rounded text-xs bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300" }, ["已安装"])
                            : null,
                        ]),
                        View({ dataset: { t: "tools-page-row-button-button-button-or-button" }, class: "flex items-center gap-1" }, [
                          installed
                            ? [
                              Button({
                                store: new Timeless.ui.ButtonCore({
                                  variant: "ghost",
                                  onClick() { handleRunApp(name); },
                                }),
                              }, ["运行"]),
                              Button({
                                store: new Timeless.ui.ButtonCore({
                                  variant: "ghost",
                                  onClick() { handleUpdateApp(name); },
                                }),
                              }, ["更新"]),
                              Button({
                                store: new Timeless.ui.ButtonCore({
                                  variant: "ghost",
                                  onClick() { handleRemoveApp(name); },
                                }),
                              }, ["移除"]),
                            ]
                            : Button({
                              store: new Timeless.ui.ButtonCore({
                                variant: "outline",
                                onClick() {
                                  appInstallName_.as(name);
                                  handleInstallApp();
                                },
                              }),
                            }, ["安装"]),
                        ]),
                      ]),
                      desc ? View({ dataset: { t: "tools-page-desc-value-text" }, class: "text-xs text-zinc-500 dark:text-zinc-400" }, [desc]) : null,
                    ]);
                  } }),
                ]);
              },
            }),

            // Empty state
            Show({
              when: computed(appList_, (v) => v.length === 0 && !appLoading_.value && !appError_.value),
              ok() {
                return View({ dataset: { t: "tools-page-暂无应用-点击刷新应用列表加载-text" }, class: "text-sm text-zinc-400 py-8 text-center" }, [
                  '暂无应用，点击"刷新应用列表"加载',
                ]);
              },
            }),

            // App tasks
            Show({
              when: computed(appTasks_, (v) => v.length > 0),
              ok() {
                return View({ dataset: { t: "tools-page-stack-任务列表-app-tasks_-list" }, class: "space-y-2 pt-2" }, [
                  View({ dataset: { t: "tools-page-任务列表-text" }, class: "text-sm font-medium text-zinc-700 dark:text-zinc-300" }, ["任务列表"]),
                  For({ each: appTasks_, render(task) {
                    const tid = task.id || task.ID || "-";
                    const action = task.action || task.Action || "-";
                    const appName = task.app_name || task.AppName || task.name || "-";
                    const status = task.status || task.Status || "-";
                    return View({
                      dataset: { t: "tools-page-row-tid-value-action-value-app-name-value-status-value-text" },
                      class: "p-2 rounded text-xs border border-zinc-100 dark:border-zinc-800 flex items-center gap-3",
                    }, [
                      View({ dataset: { t: "tools-page-monospace-tid-value" }, class: "font-mono text-zinc-500" }, [tid]),
                      View({ dataset: { t: "tools-page-action-value" }, class: "text-zinc-700 dark:text-zinc-300" }, [action]),
                      View({ dataset: { t: "tools-page-app-name-value" }, class: "text-zinc-600 dark:text-zinc-400" }, [appName]),
                      View({ dataset: { t: "tools-page-status-value" }, class: status === "completed" ? "text-green-600" : status === "failed" ? "text-red-600" : "text-zinc-500" }, [status]),
                    ]);
                  } }),
                ]);
              },
            }),
          ]);
        },
      }),

    ]),
  ]);
}

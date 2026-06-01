import {
  fetchAccountList,
  fetchAppStatus,
  fetchBrowseHistoryList,
  fetchDownloadList,
  fetchVideoList,
} from "@/biz/request.js";

function pickList(data) {
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.list)) return data.list;
  if (Array.isArray(data?.data?.list)) return data.data.list;
  return [];
}

function pickTotal(data) {
  const total = data?.total ?? data?.data?.total;
  if (total !== undefined && total !== null) return Number(total) || 0;
  return pickList(data).length;
}

function getConfig() {
  if (typeof WXU !== "undefined" && WXU.config) return WXU.config;
  if (typeof window !== "undefined" && window.__wx_channels_config__) {
    return window.__wx_channels_config__;
  }
  return {};
}

function buildAPIService(statusData) {
  const cfg = getConfig();
  const protocol =
    cfg.apiServerProtocol ||
    cfg.Protocol ||
    window.location.protocol.replace(":", "") ||
    "http";
  const hostname =
    cfg.apiServerHostname ||
    cfg.Hostname ||
    window.location.hostname ||
    "127.0.0.1";
  const port = Number(
    cfg.apiServerPort || cfg.Port || window.location.port || 0,
  );
  const addr =
    statusData?.api?.addr ||
    cfg.apiServerAddr ||
    (port ? `${hostname}:${port}` : hostname);
  const listening = statusData?.api?.listening;
  return {
    id: "api",
    name: "API Server",
    description: "提供下载任务、帐号、视频和浏览记录管理接口",
    icon: "server",
    addr,
    protocol,
    host: hostname,
    port: port || "-",
    status: listening === false ? "error" : statusData ? "running" : "error",
    version: statusData?.version || cfg.version || cfg.Version || "-",
  };
}

function buildProxyService(statusData) {
  const cfg = getConfig();
  const statusAddr = statusData?.proxy?.addr || "";
  const hostname =
    cfg.ProxyServerHostname ||
    cfg.proxyServerHostname ||
    statusAddr.split(":")[0] ||
    "127.0.0.1";
  const port = Number(
    cfg.ProxyServerPort ||
      cfg.proxyServerPort ||
      statusAddr.split(":").pop() ||
      2023,
  );
  const listening = statusData?.proxy?.listening;
  return {
    id: "proxy",
    name: "Proxy Server",
    description: "拦截视频号页面请求并注入下载能力",
    icon: "radio-tower",
    addr: statusAddr || `${hostname}:${port}`,
    protocol: "http/https",
    host: hostname,
    port,
    status:
      listening === true
        ? "running"
        : listening === false
          ? "error"
          : "unknown",
    version: "-",
  };
}

function statCard(label, value, icon, desc) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex items-start justify-between gap-3" }, [
        View({ class: "min-w-0" }, [
          View({ class: "text-sm text-zinc-500 dark:text-zinc-400" }, [label]),
          View(
            {
              class:
                "mt-2 text-3xl font-semibold tracking-tight text-zinc-950 dark:text-zinc-50",
            },
            [value],
          ),
          desc
            ? View({ class: "mt-1 text-xs text-zinc-400 dark:text-zinc-500" }, [
                desc,
              ])
            : null,
        ]),
        View(
          {
            class:
              "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-600 dark:bg-zinc-900 dark:text-zinc-300",
          },
          [Icon({ name: icon, size: 20 })],
        ),
      ]),
    ],
  );
}

function statusBadge(status) {
  let cls =
    "inline-flex items-center gap-1.5 rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300";
  let dot = "h-1.5 w-1.5 rounded-full bg-zinc-400";
  let text = "未检测";
  if (status === "running") {
    cls =
      "inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-2.5 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
    dot = "h-1.5 w-1.5 rounded-full bg-emerald-500";
    text = "运行中";
  }
  if (status === "error") {
    cls =
      "inline-flex items-center gap-1.5 rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-700 dark:bg-red-950 dark:text-red-300";
    dot = "h-1.5 w-1.5 rounded-full bg-red-500";
    text = "异常";
  }
  return View({ class: cls }, [View({ class: dot }), text]);
}

function serviceCard(service) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex items-start justify-between gap-4" }, [
        View({ class: "flex min-w-0 gap-3" }, [
          View(
            {
              class:
                "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-600 dark:bg-zinc-900 dark:text-zinc-300",
            },
            [Icon({ name: service.icon, size: 20 })],
          ),
          View({ class: "min-w-0" }, [
            View(
              {
                class:
                  "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
              },
              [service.name],
            ),
            View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
              service.description,
            ]),
          ]),
        ]),
        statusBadge(service.status),
      ]),
      View(
        {
          class:
            "mt-5 grid gap-3 rounded-lg bg-zinc-50 p-3 text-sm dark:bg-zinc-900/70 sm:grid-cols-2",
        },
        [
          serviceInfo("地址", service.addr, "network"),
          serviceInfo("端口", String(service.port || "-"), "plug"),
          serviceInfo("协议", service.protocol || "-", "globe"),
          serviceInfo("版本", service.version || "-", "badge-info"),
        ],
      ),
    ],
  );
}

function serviceInfo(label, value, icon) {
  return View({ class: "min-w-0" }, [
    View(
      {
        class:
          "flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400",
      },
      [Icon({ name: icon, size: 13 }), label],
    ),
    View(
      {
        class:
          "mt-1 truncate font-mono text-sm text-zinc-800 dark:text-zinc-200",
        title: value,
      },
      [value],
    ),
  ]);
}

export default function DashboardPageView(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: props.client,
    }),
    videos: new Timeless.RequestCore(fetchVideoList, {
      client: props.client,
    }),
    browse: new Timeless.RequestCore(fetchBrowseHistoryList, {
      client: props.client,
    }),
    downloads: new Timeless.RequestCore(fetchDownloadList, {
      client: props.client,
    }),
    status: new Timeless.RequestCore(fetchAppStatus, {
      client: props.client,
    }),
  };
  const loading_ = ref(false);
  const error_ = ref("");
  const stats_ = ref({
    accounts: 0,
    videos: 0,
    browse: 0,
    downloads: 0,
  });
  const services_ = refarr([]);

  async function refresh() {
    loading_.as(true);
    error_.as("");
    const [accounts, videos, browse, downloads, status] = await Promise.all([
      reqs.accounts.run({}),
      reqs.videos.run({ page: 1, pageSize: 1 }),
      reqs.browse.run({}),
      reqs.downloads.run({ page: 1, pageSize: 1 }),
      reqs.status.run(),
    ]);
    loading_.as(false);

    const errors = [accounts, videos, browse, downloads, status]
      .filter((r) => r.error)
      .map((r) => r.error?.message || String(r.error));
    if (errors.length) {
      error_.as(errors[0]);
    }

    stats_.as({
      accounts: accounts.error ? 0 : pickTotal(accounts.data),
      videos: videos.error ? 0 : pickTotal(videos.data),
      browse: browse.error ? 0 : pickTotal(browse.data),
      downloads: downloads.error ? 0 : pickTotal(downloads.data),
    });
    services_.as([
      buildAPIService(status.error ? null : status.data),
      buildProxyService(status.error ? null : status.data),
    ]);
  }

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        refresh();
      },
    },
    [
      View(
        {
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({}, [
              View(
                {
                  class:
                    "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["首页"],
              ),
              View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "数据统计和服务状态总览",
              ]),
            ]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  disabled: loading_,
                  onClick() {
                    refresh();
                  },
                }),
              },
              [
                Icon({ name: "refresh-cw", size: 16 }),
                computed(loading_, (v) => (v ? "刷新中..." : "刷新")),
              ],
            ),
          ]),
        ],
      ),
      ScrollView(
        { store: new Timeless.ui.ScrollViewCore({}), class: "flex-1" },
        [
          View({ class: "space-y-6 p-6" }, [
            Show({
              when: error_,
              ok() {
                return View(
                  {
                    class:
                      "rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
                  },
                  [error_],
                );
              },
            }),
            View({ class: "grid gap-4 md:grid-cols-2 xl:grid-cols-4" }, [
              statCard(
                "帐号数",
                computed(stats_, (v) => String(v.accounts)),
                "users",
                "已记录的视频号帐号",
              ),
              statCard(
                "视频数",
                computed(stats_, (v) => String(v.videos)),
                "film",
                "数据库中的视频条目",
              ),
              statCard(
                "浏览记录数",
                computed(stats_, (v) => String(v.browse)),
                "history",
                "已捕获的页面访问记录",
              ),
              statCard(
                "下载任务数",
                computed(stats_, (v) => String(v.downloads)),
                "hard-drive-download",
                "全部下载任务",
              ),
            ]),
            View({ class: "space-y-4" }, [
              View({ class: "flex items-center justify-between" }, [
                View(
                  {
                    class:
                      "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
                  },
                  ["服务状态"],
                ),
                View({ class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
                  "API 状态来自 /api/status，Proxy 地址来自前端配置或默认配置",
                ]),
              ]),
              View({ class: "grid gap-4 lg:grid-cols-2" }, [
                For({
                  each: services_,
                  render(service) {
                    return serviceCard(service);
                  },
                }),
              ]),
            ]),
          ]),
        ],
      ),
    ],
  );
}

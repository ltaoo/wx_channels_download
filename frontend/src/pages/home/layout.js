import {
  fetchAppStatus,
  fetchRootCertificateStatus,
  installRootCertificate,
  startService,
  stopService,
  uninstallRootCertificate,
  updateServiceConfig,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

/** @type {{ title: string; name: PageKey; icon: string }[]} */
const HOME_MENUS = [
  { title: "下载", name: "root.home_layout.download", icon: "hard-drive" },
  { title: "帐号", name: "root.home_layout.accounts", icon: "user" },
  { title: "内容", name: "root.home_layout.content", icon: "file-stack" },
  { title: "浏览记录", name: "root.home_layout.browse", icon: "history" },
  { title: "工具", name: "root.home_layout.tools", icon: "wrench" },
  { title: "日志", name: "root.home_layout.logs", icon: "scroll-text" },
  { title: "设置", name: "root.home_layout.settings", icon: "settings" },
];

function ChannelsStatusBadge(status_) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900",
      // title: "检测是否已和视频号页面建立 WebSocket 连接",
    },
    [
      View({ class: "flex items-center justify-between gap-3" }, [
        View({ class: "flex min-w-0 items-center gap-2" }, [
          View({
            class: computed(status_, (status) => {
              if (status === "connected") {
                return "h-2 w-2 rounded-full bg-emerald-500";
              }
              if (status === "checking") {
                return "h-2 w-2 rounded-full bg-amber-500";
              }
              return "h-2 w-2 rounded-full bg-red-500";
            }),
          }),
          View(
            {
              class:
                "truncate text-xs font-medium text-zinc-600 dark:text-zinc-300",
            },
            ["视频号"],
          ),
        ]),
        View(
          {
            class: computed(status_, (status) => {
              if (status === "connected") {
                return "text-xs font-medium text-emerald-700 dark:text-emerald-300";
              }
              if (status === "checking") {
                return "text-xs font-medium text-amber-700 dark:text-amber-300";
              }
              return "text-xs font-medium text-red-700 dark:text-red-300";
            }),
          },
          [
            computed(status_, (status) => {
              if (status === "connected") {
                return "已连接";
              }
              if (status === "checking") {
                return "检测中";
              }
              if (status === "error") {
                return "检测失败";
              }
              return "未连接";
            }),
          ],
        ),
      ]),
    ],
  );
}

function getConfig() {
  if (typeof WXU !== "undefined" && WXU.config) return WXU.config;
  // if (typeof window !== "undefined" && window.__wx_channels_config__) {
  //   return window.__wx_channels_config__;
  // }
  return {};
}

function serviceStatusText(status) {
  if (status === "running") return "运行中";
  if (status === "starting") return "启动中";
  if (status === "stopping") return "停止中";
  if (status === "stopped") return "已停止";
  if (status === "error") return "异常";
  return "未检测";
}

function serviceDotClass(status) {
  if (status === "running") return "h-2 w-2 rounded-full bg-emerald-500";
  if (status === "starting" || status === "stopping")
    return "h-2 w-2 rounded-full bg-amber-500";
  if (status === "error") return "h-2 w-2 rounded-full bg-red-500";
  return "h-2 w-2 rounded-full bg-zinc-400";
}

function serviceTextClass(status) {
  if (status === "running")
    return "text-xs font-medium text-emerald-700 dark:text-emerald-300";
  if (status === "starting" || status === "stopping")
    return "text-xs font-medium text-amber-700 dark:text-amber-300";
  if (status === "error")
    return "text-xs font-medium text-red-700 dark:text-red-300";
  return "text-xs font-medium text-zinc-500 dark:text-zinc-400";
}

function normalizeServerStatus(raw, listening) {
  if (raw) return raw;
  if (listening === true) return "running";
  if (listening === false) return "stopped";
  return "unknown";
}

function buildServices(statusData) {
  const cfg = getConfig();
  const apiProtocol =
    cfg.apiServerProtocol ||
    cfg.Protocol ||
    window.location.protocol.replace(":", "") ||
    "http";
  const apiHost =
    cfg.apiServerHostname ||
    cfg.Hostname ||
    window.location.hostname ||
    "127.0.0.1";
  const apiPort = Number(
    cfg.apiServerPort || cfg.Port || window.location.port || 0,
  );
  const proxyAddr = statusData?.proxy?.addr || "";
  const proxyHost =
    cfg.ProxyServerHostname ||
    cfg.proxyServerHostname ||
    proxyAddr.split(":")[0] ||
    "127.0.0.1";
  const proxyPort = Number(
    cfg.ProxyServerPort ||
      cfg.proxyServerPort ||
      proxyAddr.split(":").pop() ||
      2023,
  );

  return [
    {
      id: "api",
      title: "API Server",
      serviceName: "api",
      icon: "server",
      description: "下载任务、帐号、视频和浏览记录接口",
      addr:
        statusData?.api?.addr || (apiPort ? `${apiHost}:${apiPort}` : apiHost),
      protocol: apiProtocol,
      host: apiHost,
      port: apiPort || "",
      status: normalizeServerStatus(
        statusData?.api?.status,
        statusData?.api?.listening,
      ),
      configKeys: { host: "api.hostname", port: "api.port" },
    },
    {
      id: "proxy",
      title: "Proxy Server",
      serviceName: "interceptor",
      icon: "radio-tower",
      description: "拦截视频号页面请求并注入下载能力",
      addr: proxyAddr || `${proxyHost}:${proxyPort}`,
      protocol: "http/https",
      host: proxyHost,
      port: proxyPort,
      status: normalizeServerStatus(
        statusData?.proxy?.status,
        statusData?.proxy?.listening,
      ),
      configKeys: { host: "proxy.hostname", port: "proxy.port" },
    },
  ];
}

function ServiceInfo(label, value, icon) {
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
          "mt-1 truncate font-mono text-xs text-zinc-800 dark:text-zinc-200",
        // title: value,
      },
      [value || "-"],
    ),
  ]);
}

function ServiceStatusItem(props) {
  const service = props.service;
  const popover$ = new Timeless.ui.PopoverCore({
    // off: 10,
    destroyOnClose: false,
    // placement: "right",
  });
  let hideTimer = null;
  const host_ = ref(service.host || "");
  const port_ = ref(String(service.port || ""));
  const hostInput$ = new Timeless.ui.InputCore({
    defaultValue: service.host || "",
    onChange(value) {
      host_.as(value);
    },
  });
  const portInput$ = new Timeless.ui.InputCore({
    defaultValue: String(service.port || ""),
    onChange(value) {
      port_.as(value);
    },
  });

  function show() {
    if (hideTimer) {
      window.clearTimeout(hideTimer);
      hideTimer = null;
    }
    popover$.show();
    if (service.id === "proxy") props.refreshCertStatus();
  }

  function hide() {
    hideTimer = window.setTimeout(() => {
      popover$.hide();
      hideTimer = null;
    }, 140);
  }

  function content() {
    return View(
      {
        class:
          "w-80 rounded-lg border border-zinc-200 bg-white p-4 shadow-lg dark:border-zinc-800 dark:bg-zinc-950",
        onMouseEnter: show,
        onMouseLeave: hide,
      },
      [
        View({ class: "flex items-start justify-between gap-3" }, [
          View({ class: "min-w-0" }, [
            View(
              {
                class:
                  "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
              },
              [service.title],
            ),
            View({ class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
              service.description,
            ]),
          ]),
          View({ class: serviceTextClass(service.status) }, [
            serviceStatusText(service.status),
          ]),
        ]),
        View(
          {
            class:
              "mt-4 grid grid-cols-2 gap-3 rounded-lg bg-zinc-50 p-3 dark:bg-zinc-900/70",
          },
          [
            ServiceInfo("地址", service.addr, "network"),
            ServiceInfo("协议", service.protocol, "globe"),
          ],
        ),
        View({ class: "mt-4 grid grid-cols-2 gap-3" }, [
          View({}, [
            Label(
              { class: "mb-1 block text-xs text-zinc-500 dark:text-zinc-400" },
              ["Host"],
            ),
            Input({ store: hostInput$ }),
          ]),
          View({}, [
            Label(
              { class: "mb-1 block text-xs text-zinc-500 dark:text-zinc-400" },
              ["Port"],
            ),
            Input({ store: portInput$ }),
          ]),
        ]),
        View({ class: "mt-3 flex flex-wrap gap-2" }, [
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                size: "sm",
                variant: "outline",
                disabled: props.busy_,
                onClick() {
                  props.saveServiceConfig(service, host_.value, port_.value);
                },
              }),
            },
            [Icon({ name: "save", size: 14 }), "编辑"],
          ),
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                size: "sm",
                disabled: props.busy_,
                onClick() {
                  props.startService(service);
                },
              }),
            },
            [Icon({ name: "play", size: 14 }), "启动"],
          ),
          Button(
            {
              // title:
              //   service.id === "api"
              //     ? "API Server 不能通过自身 HTTP 请求停止"
              //     : "停止服务",
              store: new Timeless.ui.ButtonCore({
                size: "sm",
                variant: "outline",
                // disabled: computed(
                //   props.busy_,
                //   (busy) => busy || service.id === "api",
                // ),
                onClick() {
                  props.stopService(service);
                },
              }),
            },
            [Icon({ name: "square", size: 14 }), "停止"],
          ),
        ]),
        Show({
          when: computed(service, (t) => {
            return t.id === "proxy";
          }),
          ok() {
            return View(
              {
                class:
                  "mt-4 border-t border-zinc-200 pt-4 dark:border-zinc-800",
              },
              [
                View({ class: "flex items-center justify-between gap-3" }, [
                  View(
                    {
                      class:
                        "text-xs font-medium text-zinc-600 dark:text-zinc-300",
                    },
                    ["系统根证书"],
                  ),
                  View(
                    {
                      class: computed(props.certStatus_, (v) =>
                        v.installed
                          ? "text-xs font-medium text-emerald-700 dark:text-emerald-300"
                          : "text-xs font-medium text-zinc-500 dark:text-zinc-400",
                      ),
                    },
                    [
                      computed(props.certStatus_, (v) =>
                        v.loading
                          ? "检测中"
                          : v.error
                            ? "检测失败"
                            : v.installed
                              ? "已安装"
                              : "未安装",
                      ),
                    ],
                  ),
                ]),
                View({ class: "mt-3 flex flex-wrap gap-2" }, [
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        size: "sm",
                        variant: "outline",
                        disabled: props.certBusy_,
                        onClick() {
                          props.installCert();
                        },
                      }),
                    },
                    [Icon({ name: "badge-check", size: 14 }), "安装系统根证书"],
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        size: "sm",
                        variant: "ghost",
                        disabled: props.certBusy_,
                        onClick() {
                          props.uninstallCert();
                        },
                      }),
                    },
                    [Icon({ name: "trash-2", size: 14 }), "卸载系统根证书"],
                  ),
                ]),
              ],
            );
          },
        }),
      ],
    );
  }

  return Popover({ store: popover$, content: [content()] }, [
    View(
      {
        class:
          "rounded-lg border border-zinc-200 bg-white p-3 transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700",
        // title: "查看服务详情",
        // onMouseEnter: show,
        // onMouseLeave: hide,
      },
      [
        View({ class: "flex items-center justify-between gap-3" }, [
          View({ class: "flex min-w-0 items-center gap-2" }, [
            View({ class: serviceDotClass(service.status) }),
            Icon({ name: service.icon, size: 15 }),
            View(
              {
                class:
                  "truncate text-xs font-medium text-zinc-600 dark:text-zinc-300",
              },
              [service.title],
            ),
          ]),
          View({ class: serviceTextClass(service.status) }, [
            serviceStatusText(service.status),
          ]),
        ]),
      ],
    ),
  ]);
}

/**
 * @param {ViewComponentProps} props
 */
export default function HomeLayoutView(props) {
  const reqs = {
    reqStatus: new Timeless.RequestCore(fetchAppStatus, {
      client: api_client$,
    }),
    reqStartService: new Timeless.RequestCore(startService, {
      client: api_client$,
    }),
    reqStopService: new Timeless.RequestCore(stopService, {
      client: api_client$,
    }),
    reqUpdateConfig: new Timeless.RequestCore(updateServiceConfig, {
      client: api_client$,
    }),
    reqCertStatus: new Timeless.RequestCore(fetchRootCertificateStatus, {
      client: api_client$,
    }),
    reqInstallCert: new Timeless.RequestCore(installRootCertificate, {
      client: api_client$,
    }),
    reqUninstallCert: new Timeless.RequestCore(uninstallRootCertificate, {
      client: api_client$,
    }),
  };
  const channelsStatus_ = ref("checking");
  const services_ = refarr([]);
  const serviceBusy_ = ref(false);
  const certBusy_ = ref(false);
  const certStatus_ = ref({ loading: false, installed: false, error: "" });

  let statusTimer = null;
  let disposed = false;
  let statusWS = null;
  let statusReconnectTimer = null;
  let statusWSClosed = false;

  const ui = {
    sidemenu$: Timeless.RouteMenusModel({
      view: props.view,
      history: props.history,
      menus: HOME_MENUS,
    }),
    btn_toggle_theme$: new Timeless.ui.ButtonCore({
      variant: "outline",
      onClick() {
        const cur = props.app.getTheme();
        props.app.setTheme(cur === "dark" ? "light" : "dark");
      },
    }),
  };
  const methods = {
    applyChannelsAvailable(available) {
      channelsStatus_.as(available ? "connected" : "disconnected");
    },

    async checkChannelsStatus() {
      if (
        channelsStatus_.value === "checking" ||
        channelsStatus_.value === "error"
      ) {
        channelsStatus_.as("checking");
      }
      const result = await reqs.reqStatus.run();
      if (disposed) return;
      if (result.error) {
        channelsStatus_.as("error");
        services_.as(buildServices(null));
        return;
      }
      methods.applyChannelsAvailable(!!result.data?.channels?.available);
      services_.as(buildServices(result.data));
    },
    async refreshCertStatus() {
      if (certStatus_.value.loading) return;
      certStatus_.as({ ...certStatus_.value, loading: true, error: "" });
      const result = await reqs.reqCertStatus.run();
      if (result.error) {
        certStatus_.as({
          loading: false,
          installed: false,
          error: result.error.message || String(result.error),
        });
        return;
      }
      certStatus_.as({
        loading: false,
        installed: !!result.data?.installed,
        error: "",
      });
    },

    async runServiceAction(service, action) {
      serviceBusy_.as(true);
      const result =
        action === "start"
          ? await reqs.reqStartService.run({ name: service.serviceName })
          : await reqs.reqStopService.run({ name: service.serviceName });
      serviceBusy_.as(false);
      if (result.error) {
        props.app.tip?.({
          type: "error",
          text: [result.error.message || String(result.error)],
        });
        return;
      }
      props.app.tip?.({
        type: "success",
        text: [`${service.title} ${action === "start" ? "已启动" : "已停止"}`],
      });
      // await methods.checkChannelsStatus();
    },
    async saveServiceConfig(service, host, port) {
      const values = {};
      values[service.configKeys.host] = String(host || "").trim();
      values[service.configKeys.port] = Number(port) || 0;
      serviceBusy_.as(true);
      const result = await reqs.reqUpdateConfig.run({ values });
      serviceBusy_.as(false);
      if (result.error) {
        props.app.tip?.({
          type: "error",
          text: [result.error.message || String(result.error)],
        });
        return;
      }
      props.app.tip?.({ type: "success", text: ["服务配置已保存"] });
      // await methods.checkChannelsStatus();
    },

    async runCertAction(action) {
      certBusy_.as(true);
      const result =
        action === "install"
          ? await reqs.reqInstallCert.run()
          : await reqs.reqUninstallCert.run();
      certBusy_.as(false);
      if (result.error) {
        props.app.tip?.({
          type: "error",
          text: [result.error.message || String(result.error)],
        });
        await methods.refreshCertStatus();
        return;
      }
      props.app.tip?.({
        type: "success",
        text: [action === "install" ? "系统根证书已安装" : "系统根证书已卸载"],
      });
      await methods.refreshCertStatus();
    },

    getAPIOrigin() {
      return props.client.hostname || window.location.origin;
    },

    handleStatusWSMessage(message) {
      if (message.type !== "channels_status") return;
      const data = message.data || {};
      const available = data.channels?.available ?? data.available;
      methods.applyChannelsAvailable(!!available);
    },

    connectStatusWS() {
      // if (statusWS || typeof WebSocket === "undefined") return;
      // statusWSClosed = false;
      // const wsURL = new URL(methods.getAPIOrigin());
      // wsURL.protocol = wsURL.protocol === "https:" ? "wss:" : "ws:";
      // wsURL.pathname = "/ws/status";
      // wsURL.search = "";
      // const ws = new WebSocket(wsURL.toString());
      // statusWS = ws;
      // ws.onmessage = (ev) => {
      //   try {
      //     methods.handleStatusWSMessage(JSON.parse(ev.data));
      //   } catch {
      //     return;
      //   }
      // };
      // ws.onclose = () => {
      //   if (statusWS === ws) statusWS = null;
      //   if (!statusWSClosed && !disposed) {
      //     statusReconnectTimer = window.setTimeout(
      //       methods.connectStatusWS,
      //       2000,
      //     );
      //   }
      // };
      // ws.onerror = () => {
      //   ws.close();
      // };
    },
    closeStatusWS() {
      statusWSClosed = true;
      if (statusReconnectTimer) {
        window.clearTimeout(statusReconnectTimer);
        statusReconnectTimer = null;
      }
      if (statusWS) {
        statusWS.close();
        statusWS = null;
      }
    },
  };

  return SplitView({
    resizable: false,
    panels: [
      {
        size: 260,
        style: { overflow: "hidden" },
        content() {
          return View(
            {
              class:
                "h-full border-r border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-950",
              onMounted() {
                disposed = false;
                // methods.checkChannelsStatus();
                // methods.connectStatusWS();
                // statusTimer = setInterval(methods.checkChannelsStatus, 30000);
              },
              onUnmounted() {
                disposed = true;
                methods.closeStatusWS();
                if (statusTimer) {
                  clearInterval(statusTimer);
                  statusTimer = null;
                }
              },
            },
            [
              Flex(
                {
                  direction: "col",
                  justify: "between",
                  class: "h-full gap-4",
                },
                [
                  Flex({ direction: "col", class: "gap-5" }, [
                    View(
                      {
                        class:
                          "flex items-center gap-3 px-2 py-2 cursor-pointer",
                        onClick() {
                          props.history.push("root.home_layout.index");
                        },
                      },
                      [
                        View(
                          {
                            class:
                              "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-zinc-900 text-sm font-bold text-white shadow-sm dark:bg-zinc-100 dark:text-zinc-900",
                            // title: "WX",
                          },
                          ["WX"],
                        ),
                        View({ class: "min-w-0" }, [
                          View(
                            {
                              class:
                                "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                            },
                            ["Channels Download"],
                          ),
                          View(
                            {
                              class:
                                "truncate text-xs text-zinc-500 dark:text-zinc-400",
                            },
                            ["视频号下载管理"],
                          ),
                        ]),
                      ],
                    ),
                    View({ class: "space-y-1" }, [
                      For({
                        each: HOME_MENUS,
                        render(item) {
                          return View(
                            {
                              class: classNames([
                                "relative flex w-full cursor-pointer items-center gap-3 rounded-lg",
                                computed(ui.sidemenu$.cur, (cur) => {
                                  const active = ui.sidemenu$.isSelected(
                                    cur,
                                    item,
                                  );
                                  return active
                                    ? "bg-white px-4 py-3 text-sm font-medium text-zinc-950 shadow-sm ring-1 ring-black/5 dark:bg-zinc-900 dark:text-zinc-50 dark:ring-white/10"
                                    : "px-4 py-3 text-sm font-medium text-zinc-500 transition hover:bg-white/70 hover:text-zinc-950 dark:text-zinc-400 dark:hover:bg-zinc-900/70 dark:hover:text-zinc-50";
                                }),
                              ]),
                              // title: item.title,
                              onClick() {
                                props.history.push(item.name);
                              },
                            },
                            [
                              Show({
                                when: computed(ui.sidemenu$.cur, (cur) =>
                                  ui.sidemenu$.isSelected(cur, item),
                                ),
                                ok() {
                                  return View({
                                    class:
                                      "absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r-full bg-zinc-900 dark:bg-zinc-100",
                                  });
                                },
                              }),
                              View(
                                {
                                  class:
                                    "flex h-5 w-5 shrink-0 items-center justify-center",
                                },
                                [Icon({ name: item.icon, size: 20 })],
                              ),
                              View({ class: "truncate" }, [item.title]),
                            ],
                          );
                        },
                      }),
                    ]),
                  ]),
                  Flex({ direction: "col", class: "gap-3" }, [
                    ChannelsStatusBadge(channelsStatus_),
                    View({ class: "space-y-2" }, [
                      // For({
                      //   each: services_,
                      //   render(service) {
                      //     return ServiceStatusItem({
                      //       service,
                      //       busy_: serviceBusy_,
                      //       certBusy_,
                      //       certStatus_,
                      //       refreshCertStatus: methods.refreshCertStatus,
                      //       startService(item) {
                      //         methods.runServiceAction(item, "start");
                      //       },
                      //       stopService(item) {
                      //         methods.runServiceAction(item, "stop");
                      //       },
                      //       saveServiceConfig: methods.saveServiceConfig,
                      //       installCert() {
                      //         methods.runCertAction("install");
                      //       },
                      //       uninstallCert() {
                      //         methods.runCertAction("uninstall");
                      //       },
                      //     });
                      //   },
                      // }),
                    ]),
                    Button(
                      {
                        store: ui.btn_toggle_theme$,
                      },
                      [Icon({ name: "sun", size: 16 }), "切换主题"],
                    ),
                  ]),
                ],
              ),
            ],
          );
        },
      },
      {
        size: "auto",
        content() {
          return KeepAliveSubViews(props);
        },
      },
    ],
  });
}

import { SandboxModel } from "./index.model.js";

function endpointOf(rec) {
  return rec?.endpoint || {};
}

function statusLabel(status) {
  const labels = {
    idle: "空闲",
    busy: "使用中",
    invalid: "无效",
    running: "运行中",
    stopped: "已停止",
    error: "异常",
    destroyed: "已删除",
  };
  return labels[status] || status || "-";
}

function statusClass(status) {
  if (status === "idle" || status === "running") {
    return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  }
  if (status === "busy") {
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  }
  if (status === "stopped") {
    return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
  }
  return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
}

function shortID(id) {
  const value = String(id || "");
  return value.length > 12 ? value.slice(0, 12) : value;
}

function parseOptionalPort(value) {
  const text = String(value || "").trim();
  if (!text) return 0;
  const n = Number(text);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}

function Field(label, child) {
  return View({ class: "space-y-1" }, [
    View({ class: "text-xs font-medium text-zinc-500 dark:text-zinc-400" }, [
      label,
    ]),
    child,
  ]);
}

/**
 * Sandbox management page
 * @param {ViewComponentProps} props
 */
export default function SandboxesPageView(props) {
  const sandbox$ = new SandboxModel(props.client);
  const list$ = refarr([]);
  const selected$ = ref(null);
  const loading$ = ref(false);
  const createMode$ = ref("docker");
  const dockerAlias$ = ref("69shuba-browser");
  const dockerImage$ = ref("");
  const cdpPort$ = ref("");
  const desktopPort$ = ref("");
  const localAlias$ = ref("local-browser");
  const localCDPURL$ = ref("http://127.0.0.1:9222");

  function syncSelected(list) {
    const current = selected$.value;
    if (!current) return;
    const next = (list || []).find((item) => item.id === current.id);
    selected$.as(next || null);
  }

  async function refreshList() {
    if (loading$.value) return;
    loading$.as(true);
    try {
      const data = await sandbox$.list();
      const list = data || [];
      list$.as(list);
      syncSelected(list);
    } catch (e) {
      props.app.tip({ type: "error", text: ["加载 sandbox 失败", String(e)] });
    } finally {
      loading$.as(false);
    }
  }

  async function createSandbox() {
    if (loading$.value) return;
    const mode = createMode$.value;
    const body =
      mode === "local"
        ? {
            kind: "local",
            alias: localAlias$.value,
            cdp_url: localCDPURL$.value,
          }
        : {
            kind: "docker",
            alias: dockerAlias$.value,
            image: dockerImage$.value,
            cdp_host_port: parseOptionalPort(cdpPort$.value),
            desktop_host_port: parseOptionalPort(desktopPort$.value),
          };
    loading$.as(true);
    try {
      const rec = await sandbox$.create(body);
      props.app.tip({ text: ["浏览器容器已创建"] });
      await refreshList();
      if (rec?.id) selected$.as(rec);
    } catch (e) {
      props.app.tip({ type: "error", text: ["创建失败", String(e)] });
    } finally {
      loading$.as(false);
    }
  }

  async function destroySandbox(id) {
    if (!confirm("删除这个 sandbox？容器会被强制移除。")) return;
    try {
      await sandbox$.destroy(id);
      if (selected$.value?.id === id) selected$.as(null);
      props.app.tip({ text: ["Sandbox 已删除"] });
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["删除失败", String(e)] });
    }
  }

  async function toggleStop(rec) {
    try {
      if (rec.status === "stopped") {
        await sandbox$.resume(rec.id);
      } else {
        await sandbox$.pause(rec.id);
      }
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["操作失败", String(e)] });
    }
  }

  async function restartBrowser(rec) {
    try {
      await sandbox$.restartBrowser(rec.id);
      props.app.tip({ text: ["浏览器已重启"] });
      await refreshList();
    } catch (e) {
      props.app.tip({ type: "error", text: ["重启失败", String(e)] });
    }
  }

  async function selectSandbox(rec) {
    selected$.as(rec);
  }

  async function takeScreenshot() {
    const rec = selected$.value;
    if (!rec) return;
    if (!endpointOf(rec).cdp_url) return;
    try {
      const result = await sandbox$.screenshot(rec.id, { format: "png" });
      if (result?.data) {
        const w = window.open("", "_blank");
        if (w) {
          w.document.write(
            `<img src="data:image/${result.format};base64,${result.data}" />`,
          );
        }
      }
    } catch (e) {
      props.app.tip({ type: "error", text: ["截图失败", String(e)] });
    }
  }

  async function navigateURL() {
    const rec = selected$.value;
    if (!rec) return;
    if (!endpointOf(rec).cdp_url) return;
    const url = prompt("URL", "https://www.69shuba.com/");
    if (!url) return;
    try {
      await sandbox$.actions(rec.id, [{ type: "navigate", url }]);
      props.app.tip({ text: ["已导航", url] });
    } catch (e) {
      props.app.tip({ type: "error", text: ["导航失败", String(e)] });
    }
  }

  function openDesktop(rec) {
    const url = endpointOf(rec).desktop_url;
    if (url) window.open(url, "_blank");
  }

  refreshList();

  return View({ class: "sandboxes-page flex h-full min-h-0 flex-col" }, [
    View(
      {
        class:
          "flex items-center justify-between gap-3 border-b border-zinc-200 px-6 py-4 dark:border-zinc-800",
      },
      [
        View({}, [
          Text(
            {
              class:
                "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
            },
            "浏览器沙箱",
          ),
          Text(
            { class: "mt-0.5 text-xs text-zinc-500 dark:text-zinc-400" },
            "创建并管理带 Web 桌面和 CDP 的浏览器容器",
          ),
        ]),
        Button(
          {
            store: new Timeless.ui.ButtonCore({
              onClick: refreshList,
              variant: "outline",
              size: "sm",
            }),
          },
          [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
        ),
      ],
    ),

    View({ class: "grid min-h-0 flex-1 grid-cols-[320px_minmax(0,1fr)_260px]" }, [
      View(
        {
          class:
            "min-h-0 overflow-y-auto border-r border-zinc-200 p-4 dark:border-zinc-800",
        },
        [
          View(
            {
              class:
                "rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950",
            },
            [
              View(
                {
                  class:
                    "mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                "创建浏览器容器",
              ),
              View({ class: "mb-3 grid grid-cols-2 gap-2" }, [
                Button(
                  {
                    store: new Timeless.ui.ButtonCore({
                      variant: computed(createMode$, (v) =>
                        v === "docker" ? "default" : "outline",
                      ),
                      size: "sm",
                      onClick() {
                        createMode$.as("docker");
                      },
                    }),
                  },
                  [Icon({ name: "box", size: 14 }), "Docker"],
                ),
                Button(
                  {
                    store: new Timeless.ui.ButtonCore({
                      variant: computed(createMode$, (v) =>
                        v === "local" ? "default" : "outline",
                      ),
                      size: "sm",
                      onClick() {
                        createMode$.as("local");
                      },
                    }),
                  },
                  [Icon({ name: "monitor", size: 14 }), "已有 CDP"],
                ),
              ]),
              Show({
                when: computed(createMode$, (v) => v === "docker"),
                ok() {
                  return View({ class: "space-y-3" }, [
                    Field(
                      "名称",
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: dockerAlias$.value,
                          placeholder: "69shuba-browser",
                          onChange(value) {
                            dockerAlias$.as(value);
                          },
                        }),
                      }),
                    ),
                    Field(
                      "镜像",
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: dockerImage$.value,
                          placeholder: "默认: lscr.io/linuxserver/chromium:latest",
                          onChange(value) {
                            dockerImage$.as(value);
                          },
                        }),
                      }),
                    ),
                    View({ class: "grid grid-cols-2 gap-3" }, [
                      Field(
                        "CDP 端口",
                        Input({
                          store: new Timeless.ui.InputCore({
                            value: cdpPort$.value,
                            placeholder: "自动",
                            onChange(value) {
                              cdpPort$.as(value);
                            },
                          }),
                        }),
                      ),
                      Field(
                        "桌面端口",
                        Input({
                          store: new Timeless.ui.InputCore({
                            value: desktopPort$.value,
                            placeholder: "自动",
                            onChange(value) {
                              desktopPort$.as(value);
                            },
                          }),
                        }),
                      ),
                    ]),
                  ]);
                },
              }),
              Show({
                when: computed(createMode$, (v) => v === "local"),
                ok() {
                  return View({ class: "space-y-3" }, [
                    Field(
                      "名称",
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: localAlias$.value,
                          placeholder: "local-browser",
                          onChange(value) {
                            localAlias$.as(value);
                          },
                        }),
                      }),
                    ),
                    Field(
                      "CDP 地址",
                      Input({
                        store: new Timeless.ui.InputCore({
                          value: localCDPURL$.value,
                          placeholder: "http://127.0.0.1:9222",
                          onChange(value) {
                            localCDPURL$.as(value);
                          },
                        }),
                      }),
                    ),
                  ]);
                },
              }),
              Button(
                {
                  class: "mt-4 w-full",
                  store: new Timeless.ui.ButtonCore({
                    onClick: createSandbox,
                  }),
                },
                [
                  Icon({ name: "plus", size: 16 }),
                  computed(loading$, (v) => (v ? "创建中..." : "创建并加入池")),
                ],
              ),
            ],
          ),

          View({ class: "mt-4 space-y-2" }, [
            Show({
              when: computed(list$, (list) => list.length === 0),
              ok() {
                return View(
                  {
                    class:
                      "rounded-lg border border-dashed border-zinc-200 p-6 text-center text-sm text-zinc-500 dark:border-zinc-800 dark:text-zinc-400",
                  },
                  "暂无浏览器容器",
                );
              },
            }),
            For({
              each: list$,
              render(rec) {
                return View(
                  {
                    class: computed(selected$, (sel) =>
                      [
                        "cursor-pointer rounded-lg border p-3 transition-colors",
                        sel?.id === rec.id
                          ? "border-blue-500 bg-blue-50 dark:bg-blue-950"
                          : "border-zinc-200 bg-white hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:bg-zinc-900",
                      ].join(" "),
                    ),
                    onClick() {
                      selectSandbox(rec);
                    },
                  },
                  [
                    View({ class: "flex items-start justify-between gap-2" }, [
                      View({ class: "min-w-0" }, [
                        Text(
                          {
                            class:
                              "block truncate text-sm font-medium text-zinc-950 dark:text-zinc-50",
                          },
                          rec.alias || rec.id,
                        ),
                        Text(
                          { class: "mt-0.5 block text-xs text-zinc-500" },
                          `${rec.kind || "docker"} · ${shortID(rec.id)}`,
                        ),
                      ]),
                      View(
                        {
                          class: `shrink-0 rounded-full px-2 py-0.5 text-xs ${statusClass(rec.status)}`,
                        },
                        statusLabel(rec.status),
                      ),
                    ]),
                    View({ class: "mt-2 space-y-1 text-xs text-zinc-500" }, [
                      Text(
                        { class: "block truncate" },
                        endpointOf(rec).desktop_url
                          ? `桌面 ${endpointOf(rec).desktop_url}`
                          : "桌面 -",
                      ),
                      Text(
                        { class: "block truncate" },
                        endpointOf(rec).cdp_url
                          ? `CDP ${endpointOf(rec).cdp_url}`
                          : "CDP -",
                      ),
                    ]),
                  ],
                );
              },
            }),
          ]),
        ],
      ),

      View({ class: "flex min-h-0 flex-col bg-zinc-100 dark:bg-zinc-950" }, [
        Show({
          when: computed(selected$, (sel) => !sel),
          ok() {
            return View(
              {
                class:
                  "flex flex-1 items-center justify-center text-sm text-zinc-500",
              },
              "选择一个浏览器容器查看桌面",
            );
          },
        }),
        Show({
          when: computed(selected$, (sel) => !!sel && !!endpointOf(sel).desktop_url),
          ok() {
            return View({ class: "flex min-h-0 flex-1 flex-col" }, [
              View(
                {
                  class:
                    "flex items-center justify-between border-b border-zinc-200 bg-white px-4 py-2 dark:border-zinc-800 dark:bg-zinc-950",
                },
                [
                  Text(
                    { class: "truncate text-sm text-zinc-600 dark:text-zinc-300" },
                    computed(selected$, (sel) => endpointOf(sel).desktop_url || ""),
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick() {
                          const sel = selected$.value;
                          if (sel) openDesktop(sel);
                        },
                      }),
                    },
                    [Icon({ name: "external-link", size: 14 }), "新窗口"],
                  ),
                ],
              ),
              View({ class: "min-h-0 flex-1 bg-zinc-900" }, [
                View({
                  tag: "iframe",
                  class: "h-full w-full border-0",
                  src: computed(selected$, (sel) => endpointOf(sel).desktop_url || ""),
                }),
              ]),
            ]);
          },
        }),
        Show({
          when: computed(selected$, (sel) => !!sel && !endpointOf(sel).desktop_url),
          ok() {
            return View(
              {
                class:
                  "flex flex-1 items-center justify-center text-sm text-zinc-500",
              },
              "这个 sandbox 没有桌面预览地址",
            );
          },
        }),
      ]),

      View(
        {
          class:
            "min-h-0 overflow-y-auto border-l border-zinc-200 p-4 dark:border-zinc-800",
        },
        [
          Show({
            when: computed(selected$, (sel) => !sel),
            ok() {
              return null;
            },
          }),
          Show({
            when: computed(selected$, (sel) => !!sel),
            ok() {
              const sel = selected$.value;
              const ep = endpointOf(sel);
              const stopped = sel?.status === "stopped";
              return View({ class: "space-y-4" }, [
                View({}, [
                  Text(
                    {
                      class:
                        "block text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                    },
                    sel?.alias || sel?.id || "-",
                  ),
                  Text(
                    { class: "mt-1 block text-xs text-zinc-500" },
                    statusLabel(sel?.status),
                  ),
                ]),
                View({ class: "grid grid-cols-2 gap-2" }, [
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick() {
                          if (sel) toggleStop(sel);
                        },
                      }),
                    },
                    stopped ? "启动" : "停止",
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick() {
                          if (sel) restartBrowser(sel);
                        },
                      }),
                    },
                    "重启",
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick: takeScreenshot,
                      }),
                    },
                    "截图",
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "outline",
                        size: "sm",
                        onClick: navigateURL,
                      }),
                    },
                    "导航",
                  ),
                ]),
                Button(
                  {
                    class: "w-full",
                    store: new Timeless.ui.ButtonCore({
                      variant: "destructive",
                      size: "sm",
                      onClick() {
                        if (sel?.id) destroySandbox(sel.id);
                      },
                    }),
                  },
                  [Icon({ name: "trash-2", size: 14 }), "删除 sandbox"],
                ),
                View(
                  {
                    class:
                      "space-y-2 rounded-lg border border-zinc-200 bg-white p-3 text-xs dark:border-zinc-800 dark:bg-zinc-950",
                  },
                  [
                    Text({ class: "block text-zinc-500" }, `ID: ${sel?.id || "-"}`),
                    Text({ class: "block text-zinc-500" }, `Kind: ${sel?.kind || "-"}`),
                    Text({ class: "block break-all text-zinc-500" }, `Image: ${sel?.image || "-"}`),
                    Text({ class: "block break-all text-zinc-500" }, `Desktop: ${ep.desktop_url || "-"}`),
                    Text({ class: "block break-all text-zinc-500" }, `CDP: ${ep.cdp_url || "-"}`),
                  ],
                ),
              ]);
            },
          }),
        ],
      ),
    ]),
  ]);
}

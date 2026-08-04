import { SandboxModel } from "./index.model.js";

function Field(label, child) {
  return View({ dataset: { t: "sandboxes-page-field-stack-label-value-child-value" }, class: "space-y-1" }, [
    View({ dataset: { t: "sandboxes-page-field-label-value-text" }, class: "text-xs font-medium text-zinc-500 dark:text-zinc-400" }, [
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
  const vm$ = SandboxModel(props);
  const { state, ui, methods } = vm$;

  return View({
    dataset: { t: "sandboxes-page-sandboxes-page-page-root-row-浏览器沙箱-创建并管理带-Web-桌面和-CDP-的浏览器容器-button-创建或添加-Sandbox-button-button-button-state-list-list-row-scroll-area" },
    class: "sandboxes-page flex h-full min-h-0 flex-col",
    onMounted() {
      methods.init();
    },
  }, [
    View(
      {
        dataset: { t: "sandboxes-page-header-row-浏览器沙箱-创建并管理带-Web-桌面和-CDP-的浏览器容器-button" },
        class:
          "flex items-center justify-between gap-3 border-b border-zinc-200 px-6 py-4 dark:border-zinc-800",
      },
      [
        View({ dataset: { t: "sandboxes-page-浏览器沙箱-创建并管理带-Web-桌面和-CDP-的浏览器容器" } }, [
          View(
            {
              dataset: { t: "sandboxes-page-浏览器沙箱-heading" },
              class: "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
            },
            "浏览器沙箱",
          ),
          View(
            { dataset: { t: "sandboxes-page-创建并管理带-Web-桌面和-CDP-的浏览器容器-text" }, class: "mt-0.5 text-xs text-zinc-500 dark:text-zinc-400" },
            "创建并管理带 Web 桌面和 CDP 的浏览器容器",
          ),
        ]),
        Button(
          {
            store: ui.refreshBtn,
          },
          [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
        ),
      ],
    ),

    View(
      { dataset: { t: "sandboxes-page-grid-row-创建或添加-Sandbox-button-button-button-state-list-list-row-scroll-area" }, class: "grid min-h-0 flex-1 grid-cols-[320px_minmax(0,1fr)_260px]" },
      [
        View(
          {
            dataset: { t: "sandboxes-page-scroll-area-创建或添加-Sandbox-button-button-button-state-list-list" },
            class:
              "min-h-0 overflow-y-auto border-r border-zinc-200 p-4 dark:border-zinc-800",
          },
          [
            View(
              {
                dataset: { t: "sandboxes-page-card-创建或添加-Sandbox-button-button-button" },
                class:
                  "rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950",
              },
              [
                View(
                  {
                    dataset: { t: "sandboxes-page-创建或添加-Sandbox-heading" },
                    class:
                      "mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                  },
                  "创建或添加 Sandbox",
                ),
                View({ dataset: { t: "sandboxes-page-grid-button-button" }, class: "mb-3 grid grid-cols-2 gap-2" }, [
                  Button(
                    {
                      store: ui.dockerModeBtn,
                    },
                    [Icon({ name: "box", size: 14 }), "Docker"],
                  ),
                  Button(
                    {
                      store: ui.localModeBtn,
                    },
                    [Icon({ name: "monitor", size: 14 }), "已有 Sandbox"],
                  ),
                ]),
                Show({
                  when: computed(state.createMode, (v) => v === "docker"),
                  ok() {
                    return View({ dataset: { t: "sandboxes-page-stack-field-field-field-field" }, class: "space-y-3" }, [
                      Field(
                        "名称",
                        Input({
                          store: ui.dockerAliasInput,
                        }),
                      ),
                      Field(
                        "镜像",
                        Input({
                          store: ui.dockerImageInput,
                        }),
                      ),
                      View({ dataset: { t: "sandboxes-page-grid-field-field" }, class: "grid grid-cols-2 gap-3" }, [
                        Field(
                          "CDP 端口",
                          Input({
                            store: ui.cdpPortInput,
                          }),
                        ),
                        Field(
                          "桌面端口",
                          Input({
                            store: ui.desktopPortInput,
                          }),
                        ),
                      ]),
                    ]);
                  },
                }),
                Show({
                  when: computed(state.createMode, (v) => v === "local"),
                  ok() {
                    return View({ dataset: { t: "sandboxes-page-stack-field-field-field" }, class: "space-y-3" }, [
                      Field(
                        "名称",
                        Input({
                          store: ui.localAliasInput,
                        }),
                      ),
                      Field(
                        "预览地址",
                        Input({
                          store: ui.localPreviewURLInput,
                        }),
                      ),
                      Field(
                        "CDP 地址",
                        Input({
                          store: ui.localCDPURLInput,
                        }),
                      ),
                    ]);
                  },
                }),
                Button(
                  {
                    class: "mt-4 w-full",
                    store: ui.createBtn,
                  },
                  [
                    Icon({ name: "plus", size: 16 }),
                    state.createButtonText,
                  ],
                ),
              ],
            ),

            View({ dataset: { t: "sandboxes-page-stack-state-list-list" }, class: "mt-4 space-y-2" }, [
              Show({
                when: state.empty,
                ok() {
                  return View(
                    {
                      dataset: { t: "sandboxes-page-card-暂无浏览器容器-text" },
                      class:
                        "rounded-lg border border-dashed border-zinc-200 p-6 text-center text-sm text-zinc-500 dark:border-zinc-800 dark:text-zinc-400",
                    },
                    "暂无浏览器容器",
                  );
                },
              }),
              For({
                each: state.list,
                render(rec) {
                  return View(
                    {
                      dataset: { t: "sandboxes-page-text-text-text-text" },
                      class: computed(state.selected, (sel) =>
                        [
                          "cursor-pointer rounded-lg border p-3 transition-colors",
                          sel?.id === rec.id
                            ? "border-blue-500 bg-blue-50 dark:bg-blue-950"
                            : "border-zinc-200 bg-white hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:bg-zinc-900",
                        ].join(" "),
                      ),
                      onClick() {
                        methods.selectSandbox(rec);
                      },
                    },
                    [
                      View(
                        { dataset: { t: "sandboxes-page-row-text-text" }, class: "flex items-start justify-between gap-2" },
                        [
                          View({ dataset: { t: "sandboxes-page-text-text" }, class: "min-w-0" }, [
                            Text(
                              {
                                class:
                                  "block truncate text-sm font-medium text-zinc-950 dark:text-zinc-50",
                              },
                              rec.alias || rec.id,
                            ),
                            Text(
                              { class: "mt-0.5 block text-xs text-zinc-500" },
                              `${rec.kind || "docker"} · ${methods.deviceLabel(rec)} · ${methods.shortID(rec.id)}`,
                            ),
                          ]),
                          View(
                            {
                              dataset: { t: "sandboxes-page-sandbox-list-item-status-badge" },
                              class: `shrink-0 rounded-full px-2 py-0.5 text-xs ${methods.statusClass(rec.status)}`,
                            },
                            methods.statusLabel(rec.status),
                          ),
                        ],
                      ),
                      View({ dataset: { t: "sandboxes-page-stack-text-text-text" }, class: "mt-2 space-y-1 text-xs text-zinc-500" }, [
                        Text(
                          { class: "block truncate" },
                          methods.desktopEndpointOf(rec)
                            ? `桌面 ${methods.desktopEndpointOf(rec)}`
                            : "桌面 -",
                        ),
                        Text(
                          { class: "block truncate" },
                          methods.endpointOf(rec).cdp_url
                            ? `CDP ${methods.endpointOf(rec).cdp_url}`
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

        View({ dataset: { t: "sandboxes-page-selected-sandbox-desktop-preview-pane" }, class: "flex min-h-0 flex-col bg-zinc-100 dark:bg-zinc-950" }, [
          Show({
            when: computed(state.selected, (sel) => !sel),
            ok() {
              return View(
                {
                  dataset: { t: "sandboxes-page-row-选择一个浏览器容器查看桌面-text" },
                  class:
                    "flex flex-1 items-center justify-center text-sm text-zinc-500",
                },
                "选择一个浏览器容器查看桌面",
              );
            },
          }),
          Show({
            when: computed(state.previewURL, (url) => !!url),
            ok() {
              return View({ dataset: { t: "sandboxes-page-row-text-button-iframe-node" }, class: "flex min-h-0 flex-1 flex-col" }, [
                View(
                  {
                    dataset: { t: "sandboxes-page-header-row-text-button" },
                    class:
                      "flex items-center justify-between border-b border-zinc-200 bg-white px-4 py-2 dark:border-zinc-800 dark:bg-zinc-950",
                  },
                  [
                    Text(
                      {
                        class:
                          "truncate text-sm text-zinc-600 dark:text-zinc-300",
                      },
                      state.previewEndpoint,
                    ),
                    Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          onClick() {
                            const sel = state.selected.value;
                            if (sel) methods.openDesktop(sel);
                          },
                        }),
                      },
                      [Icon({ name: "external-link", size: 14 }), "新窗口"],
                    ),
                  ],
                ),
                View({ dataset: { t: "sandboxes-page-row-iframe-node" }, class: "min-h-0 flex-1 bg-zinc-900" }, [
                  View({
                    dataset: { t: "sandboxes-page-iframe-node" },
                    tag: "iframe",
                    class: "h-full w-full border-0",
                    src: state.previewURL,
                  }),
                ]),
              ]);
            },
          }),
          Show({
            when: computed(
              { selected: state.selected, previewURL: state.previewURL },
              ({ selected, previewURL }) => !!selected && !previewURL,
            ),
            ok() {
              return View(
                {
                  dataset: { t: "sandboxes-page-row-state-preview-message-text" },
                  class:
                    "flex flex-1 items-center justify-center text-sm text-zinc-500",
                },
                state.previewMessage,
              );
            },
          }),
        ]),

        View(
          {
            dataset: { t: "sandboxes-page-scroll-area" },
            class:
              "min-h-0 overflow-y-auto border-l border-zinc-200 p-4 dark:border-zinc-800",
          },
          [
            Show({
              when: computed(state.selected, (sel) => !sel),
              ok() {
                return null;
              },
            }),
            Show({
              when: computed(state.selected, (sel) => !!sel),
              ok() {
                const sel = state.selected.value;
                const ep = methods.endpointOf(sel);
                const stopped = methods.isPausedStatus(sel?.status);
                const usable = methods.isDeviceUsable(sel);
                return View({ dataset: { t: "sandboxes-page-stack-text-text-button-button-button-button-button-button-text-text-text-text-text" }, class: "space-y-4" }, [
                  View({ dataset: { t: "sandboxes-page-text-text-2" } }, [
                    Text(
                      {
                        class:
                          "block text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                      },
                      sel?.alias || sel?.id || "-",
                    ),
                    Text(
                      { class: "mt-1 block text-xs text-zinc-500" },
                      methods.statusLabel(sel?.status),
                    ),
                  ]),
                  View({ dataset: { t: "sandboxes-page-grid-button-button-button-button-button" }, class: "grid grid-cols-2 gap-2" }, [
                    Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          disabled: !usable,
                          onClick() {
                            if (sel) methods.toggleStop(sel);
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
                          disabled: !usable,
                          onClick() {
                            if (sel) methods.restartBrowser(sel);
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
                          // disabled: state.statusRefreshing,
                          onClick() {
                            if (sel) methods.refreshSandboxStatus(sel);
                          },
                        }),
                      },
                      [
                        Icon({ name: "refresh-cw", size: 14 }),
                        state.refreshStatusButtonText,
                      ],
                    ),
                    Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          disabled: !usable,
                          onClick: methods.takeScreenshot,
                        }),
                      },
                      "截图",
                    ),
                    Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          disabled: !usable,
                          onClick: methods.navigateURL,
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
                          if (sel?.id) methods.destroySandbox(sel.id);
                        },
                      }),
                    },
                    [Icon({ name: "trash-2", size: 14 }), "删除 sandbox"],
                  ),
                  View(
                    {
                      dataset: { t: "sandboxes-page-card-stack-text-text-text-text-text-text" },
                      class:
                        "space-y-2 rounded-lg border border-zinc-200 bg-white p-3 text-xs dark:border-zinc-800 dark:bg-zinc-950",
                    },
                    [
                      Text(
                        { class: "block text-zinc-500" },
                        `ID: ${sel?.id || "-"}`,
                      ),
                      Text(
                        { class: "block text-zinc-500" },
                        `Kind: ${sel?.kind || "-"}`,
                      ),
                      Text(
                        { class: "block break-all text-zinc-500" },
                        `Device: ${methods.deviceDetail(sel)}`,
                      ),
                      Text(
                        { class: "block break-all text-zinc-500" },
                        `Image: ${sel?.image || "-"}`,
                      ),
                      Text(
                        { class: "block break-all text-zinc-500" },
                        `Desktop: ${methods.desktopEndpointOf(sel) || "-"}`,
                      ),
                      Text(
                        { class: "block break-all text-zinc-500" },
                        `CDP: ${ep.cdp_url || "-"}`,
                      ),
                      Text(
                        { class: "block break-all text-zinc-500" },
                        `Error: ${sel?.error || "-"}`,
                      ),
                    ],
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

import {
  DownloadTaskStatus,
  DownloadsPageModel,
  formatBytes,
} from "./downloads.model.js";

function HeaderStat(label, value, icon) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex items-center justify-between" }, [
        View({ class: "text-sm text-zinc-500 dark:text-zinc-400" }, [label]),
        Icon({ name: icon, size: 18 }),
      ]),
      View(
        {
          class: "mt-2 text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
        },
        [value],
      ),
    ],
  );
}

function SmallButton(label, onClick, variant = "ghost") {
  return Button(
    {
      store: new Timeless.ui.ButtonCore({
        variant,
        size: "sm",
        onClick,
      }),
    },
    [label],
  );
}

function statusClass(status) {
  if (status === DownloadTaskStatus.Running)
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  if (status === DownloadTaskStatus.Done)
    return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  if (status === DownloadTaskStatus.Error)
    return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
  if (status === DownloadTaskStatus.Paused)
    return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
  if (status === DownloadTaskStatus.Wait || status === DownloadTaskStatus.Ready)
    return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
  return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
}

function progressClass(status) {
  if (status === DownloadTaskStatus.Running)
    return "bg-blue-500 dark:bg-blue-400";
  if (status === DownloadTaskStatus.Done)
    return "bg-emerald-500 dark:bg-emerald-400";
  if (status === DownloadTaskStatus.Error) return "bg-red-500 dark:bg-red-400";
  if (status === DownloadTaskStatus.Paused)
    return "bg-zinc-500 dark:bg-zinc-400";
  return "bg-amber-500 dark:bg-amber-400";
}

function countForTab(stats, tab) {
  if (!stats || !tab) return 0;
  return Number(stats[tab.countKey] || 0);
}

function isStartableStatus(status) {
  return (
    status === DownloadTaskStatus.Ready ||
    status === DownloadTaskStatus.Wait ||
    status === DownloadTaskStatus.Paused ||
    status === DownloadTaskStatus.Error
  );
}

function isPlayableStatus(status) {
  return status === DownloadTaskStatus.Done;
}

function parseTaskJSON(value) {
  if (!value) return {};
  if (typeof value === "object") return value;
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

function isHTMLTask(task) {
  const metadata2 = parseTaskJSON(task.metadata2 || task.Metadata2);
  const labels = parseTaskJSON(
    task.labels || task.Labels || task.extra || task.Extra,
  );
  const contentType = String(
    task.content_type ||
      task.contentType ||
      task.mime_type ||
      task.mimeType ||
      metadata2.content_type ||
      metadata2.contentType ||
      metadata2.mime_type ||
      metadata2.mimeType ||
      labels.content_type ||
      labels.contentType ||
      labels.mime_type ||
      labels.mimeType ||
      "",
  )
    .trim()
    .toLowerCase();
  if (contentType === "html" || contentType === "text/html") return true;

  const path = String(task.filepath || task.path || task.name || task.url || "")
    .split("?")[0]
    .split("#")[0]
    .toLowerCase();
  return path.endsWith(".html") || path.endsWith(".htm");
}

function taskPlayLabel(task) {
  return isHTMLTask(task) ? "在浏览器打开" : "播放";
}

function DownloadInfoItem(label, value, extraClass = "") {
  return View(
    {
      class: classNames([
        "flex min-w-0 items-center gap-1 text-xs text-zinc-500 dark:text-zinc-400",
        extraClass,
      ]),
    },
    [
      View({ class: "shrink-0 text-zinc-400 dark:text-zinc-500" }, [label]),
      View(
        {
          class:
            "min-w-0 truncate font-medium text-zinc-700 dark:text-zinc-200",
        },
        [value],
      ),
    ],
  );
}

function DownloadInfoDivider() {
  return View(
    { class: "hidden h-3 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800 sm:block" },
    [],
  );
}

function DownloadInfoBar(task) {
  return View(
    {
      class:
        "min-w-0 rounded-md border border-zinc-100 bg-zinc-50 px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900/60",
    },
    [
      View({ class: "flex items-center gap-3" }, [
        View(
          {
            class:
              "w-11 shrink-0 text-right text-sm font-semibold tabular-nums text-zinc-950 dark:text-zinc-50",
          },
          [
            computed(task, (t) => {
              return `${Math.floor(t.percent ?? t.progress_info.percent)}%`;
            }),
          ],
        ),
        View(
          {
            class:
              "h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800",
          },
          [
            View({
              class: classNames([
                "h-full rounded-full transition-all",
                computed(task, (t) => {
                  return progressClass(t.status);
                }),
              ]),
              style: {
                width: computed(task, (t) => {
                  return `${t.percent ?? t.progress_info.percent}%`;
                }),
              },
            }),
          ],
        ),
      ]),
      View(
        {
          class:
            "mt-2 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 pl-0 sm:pl-14",
        },
        [
          DownloadInfoItem(
            "大小",
            computed(task, (t) => t.size_text),
          ),
          DownloadInfoDivider(),
          DownloadInfoItem(
            "已下",
            computed(task, (t) =>
              t.progress_info.total
                ? formatBytes(t.progress_info.downloaded)
                : "-",
            ),
          ),
          DownloadInfoDivider(),
          DownloadInfoItem(
            "速度",
            computed(task, (t) =>
              t.status === DownloadTaskStatus.Running ? t.speed_text : "-",
            ),
            "tabular-nums",
          ),
          DownloadInfoDivider(),
          DownloadInfoItem(
            "更新",
            computed(task, (t) => t.updated_at_text),
          ),
          Show({
            when: computed(
              task,
              (t) => t.status === DownloadTaskStatus.Error && !!t.error,
            ),
            ok() {
              return [
                DownloadInfoDivider(),
                DownloadInfoItem(
                  "原因",
                  computed(task, (t) => t.error),
                  "max-w-full text-red-600 dark:text-red-300",
                ),
              ];
            },
          }),
        ],
      ),
    ],
  );
}

function TaskCard(task, vm$) {
  const deleteFileCheckbox$ = new Timeless.ui.CheckboxCore({
    defaultValue: false,
  });
  const deleteFileCheckboxId = `delete-file-${task.id || task.task_id}`;

  return View(
    {
      class:
        "group rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View({ class: "flex flex-col gap-4 lg:flex-row lg:items-start" }, [
        View(
          {
            class:
              "grid min-w-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,360px)_auto]",
          },
          [
            View({ class: "min-w-0" }, [
              View({ class: "flex items-start gap-3" }, [
                Show({
                  when: computed(task, (t) => t.display_cover_url || t.cover_url),
                  ok() {
                    return View(
                      {
                        class:
                          "h-20 w-20 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                      },
                      [
                        Img({
                          class: "h-full w-full object-cover",
                          src: computed(
                            task,
                            (t) => t.display_cover_url || t.cover_url,
                          ),
                          alt: computed(task, (t) => t.title || "cover"),
                        }),
                      ],
                    );
                  },
                }),
                View({ class: "min-w-0 flex-1" }, [
                  View(
                    {
                      class:
                        "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                      title: task.title || task.task_id,
                    },
                    [task.title || task.name || task.task_id || "未命名任务"],
                  ),
                  View(
                    {
                      class:
                        "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [task.filepath || task.url || "-"],
                  ),
                  View({ class: "mt-2" }, [
                    Show({
                      when: computed(task, (t) => isPlayableStatus(t.status)),
                      ok() {
                        return SmallButton(
                          computed(task, taskPlayLabel),
                          () =>
                            isHTMLTask(task)
                              ? vm$.methods.openInBrowser(task)
                              : vm$.methods.play(task),
                          "outline",
                        );
                      },
                    }),
                    Show({
                      when: computed(
                        task,
                        (t) => t.status === DownloadTaskStatus.Done,
                      ),
                      ok() {
                        return SmallButton("定位", () =>
                          vm$.methods.openFile(task),
                        );
                      },
                    }),
                  ]),
                ]),
                View(
                  {
                    class: classNames([
                      "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                      computed(task, (t) => statusClass(t.status)),
                    ]),
                  },
                  [computed(task, (t) => t.status_text)],
                ),
              ]),
            ]),
            DownloadInfoBar(task),
            View(
              {
                class:
                  "flex shrink-0 flex-wrap items-center gap-2 xl:w-28 xl:flex-col xl:items-stretch",
              },
              [
                Show({
                  when: computed(task, (t) => {
                    return t.status === DownloadTaskStatus.Error;
                  }),
                  ok() {
                    return SmallButton(
                      "重试",
                      () => vm$.methods.retry(task),
                      "outline",
                    );
                  },
                }),
                Show({
                  when: computed(task, (t) => isStartableStatus(t.status)),
                  ok() {
                    return SmallButton(
                      "开始",
                      () => vm$.methods.start(task),
                      "outline",
                    );
                  },
                }),
                SmallButton(
                  "删除",
                  () => vm$.methods.remove(task, deleteFileCheckbox$.value),
                  "ghost",
                ),
                View({ class: "flex items-center gap-1.5 xl:justify-start" }, [
                  Checkbox({
                    id: deleteFileCheckboxId,
                    store: deleteFileCheckbox$,
                  }),
                  Label(
                    {
                      for: deleteFileCheckboxId,
                      class:
                        "cursor-pointer whitespace-nowrap text-xs text-zinc-600 dark:text-zinc-300",
                    },
                    ["同时删除文件"],
                  ),
                ]),
              ],
            ),
          ],
        ),
      ]),
    ],
  );
}

function RemoteTaskCard(task) {
  return View(
    {
      class:
        "rounded-lg border border-sky-200 bg-white p-4 shadow-sm dark:border-sky-900 dark:bg-zinc-950",
    },
    [
      View({ class: "flex gap-4" }, [
        View(
          {
            class:
              "flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-300",
          },
          [Icon({ name: "server", size: 24 })],
        ),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex items-start justify-between gap-3" }, [
            View({ class: "min-w-0" }, [
              View(
                {
                  class:
                    "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                  title: task.title || task.task_id,
                },
                [task.title || task.task_id || "未命名任务"],
              ),
              View(
                {
                  class:
                    "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                },
                [task.filepath || task.url || "-"],
              ),
            ]),
            View(
              {
                class: classNames([
                  "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                  computed(task, (t) => statusClass(t.status)),
                ]),
              },
              [computed(task, (t) => t.status_text)],
            ),
          ]),
          View({ class: "mt-3 space-y-2" }, [
            View(
              {
                class:
                  "h-2 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
              },
              [
                View({
                  class: "h-full rounded-full bg-sky-600 dark:bg-sky-300",
                  style: {
                    width: computed(task, (t) => `${t.progress_info.percent}%`),
                  },
                }),
              ],
            ),
            View(
              {
                class:
                  "flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400",
              },
              [
                computed(task, (t) => `${t.progress_info.percent}%`),
                computed(task, (t) => t.size_text),
                computed(task, (t) =>
                  t.status === DownloadTaskStatus.Running ? t.speed_text : "",
                ),
                "更新",
                computed(task, (t) => t.updated_at_text),
              ],
            ),
          ]),
        ]),
      ]),
    ],
  );
}

function RemoteServerPanel(vm$) {
  return Show({
    when: vm$.state.remoteEnabled,
    ok() {
      return View(
        {
          class:
            "space-y-3 rounded-lg border border-sky-200 bg-sky-50/50 p-4 dark:border-sky-900 dark:bg-sky-950/20",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({}, [
              View(
                {
                  class:
                    "flex items-center gap-2 text-base font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [Icon({ name: "server", size: 18 }), "RemoteServer"],
              ),
              View({ class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
                vm$.state.remoteLabel,
              ]),
            ]),
            View({ class: "flex flex-wrap gap-3" }, [
              HeaderStat(
                "远端任务",
                computed(vm$.state.remoteTotal, (v) => String(v)),
                "list",
              ),
              HeaderStat(
                "远端下载中",
                computed(vm$.state.remoteRunningCount, (v) => String(v)),
                "activity",
              ),
              HeaderStat("远端速度", vm$.state.remoteTotalSpeed, "gauge"),
            ]),
          ]),
          Show({
            when: vm$.state.remoteError,
            ok() {
              return View(
                {
                  class:
                    "rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.remoteError],
              );
            },
          }),
          Show({
            when: computed(vm$.state.remoteTasks, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-32 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "inbox", size: 28 }),
                  computed(vm$.state.remoteLoading, (loading) =>
                    loading ? "远端加载中..." : "暂无远端下载任务",
                  ),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.remoteTasks,
                  render(task) {
                    return RemoteTaskCard(task);
                  },
                }),
                Show({
                  when: computed(vm$.state.remoteNoMore, (v) => !v),
                  ok() {
                    return View({ class: "flex justify-center py-2" }, [
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            disabled: vm$.state.remoteLoading,
                            onClick() {
                              vm$.methods.loadMoreRemote();
                            },
                          }),
                        },
                        [
                          computed(vm$.state.remoteLoading, (v) =>
                            v ? "加载中..." : "加载更多远端任务",
                          ),
                        ],
                      ),
                    ]);
                  },
                }),
              ]);
            },
          }),
        ],
      );
    },
  });
}

export default function DownloadsPageView(props) {
  const vm$ = DownloadsPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
      onUnmounted() {
        vm$.methods.destroy();
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
                ["下载列表"],
              ),
              View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "管理视频号下载任务和本地文件",
              ]),
            ]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.refresh();
                  },
                }),
              },
              [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
            ),
          ]),
          View({ class: "mt-5 grid gap-3 md:grid-cols-3" }, [
            HeaderStat(
              "任务总数",
              computed(vm$.state.statusStats, (v) => String(v.total || 0)),
              "hard-drive",
            ),
            HeaderStat(
              "下载中",
              computed(vm$.state.statusStats, (v) => String(v.running || 0)),
              "activity",
            ),
            HeaderStat("总速度", vm$.state.totalSpeed, "gauge"),
          ]),
          View({ class: "mt-4 flex flex-wrap gap-2" }, [
            For({
              each: vm$.state.tabs,
              render(tab) {
                return View(
                  {
                    class: computed(vm$.state.activeTab, (v) => {
                      const active = v === tab.value;
                      return active
                        ? "flex cursor-pointer items-center gap-2 rounded-md bg-zinc-900 px-3 py-1.5 text-sm text-white dark:bg-zinc-100 dark:text-zinc-900"
                        : "flex cursor-pointer items-center gap-2 rounded-md border border-zinc-200 px-3 py-1.5 text-sm text-zinc-600 hover:bg-zinc-100 dark:border-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-800";
                    }),
                    onClick() {
                      vm$.methods.filter(tab.value);
                    },
                  },
                  [
                    tab.label,
                    View(
                      {
                        class: computed(vm$.state.activeTab, (activeTab) => {
                          const active = activeTab === tab.value;
                          return active
                            ? "min-w-5 rounded-full bg-white/20 px-1.5 text-center text-xs font-semibold"
                            : "min-w-5 rounded-full bg-zinc-100 px-1.5 text-center text-xs font-semibold text-zinc-500 dark:bg-zinc-800 dark:text-zinc-300";
                        }),
                      },
                      [
                        computed(vm$.state.statusStats, (stats) =>
                          String(countForTab(stats, tab)),
                        ),
                      ],
                    ),
                  ],
                );
              },
            }),
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "space-y-3 p-6" }, [
          // RemoteServerPanel(vm$),
          Show({
            when: vm$.state.error,
            ok() {
              return View(
                {
                  class:
                    "rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.error],
              );
            },
          }),
          Show({
            when: computed(vm$.state.tasks, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "inbox", size: 36 }),
                  computed(vm$.state.loading, (loading) =>
                    loading ? "加载中..." : "暂无下载任务",
                  ),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.tasks,
                  render(task) {
                    return TaskCard(task, vm$);
                  },
                }),
                Show({
                  when: computed(vm$.state.noMore, (v) => !v),
                  ok() {
                    return View({ class: "flex justify-center py-4" }, [
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            disabled: vm$.state.loading,
                            onClick() {
                              vm$.methods.loadMore();
                            },
                          }),
                        },
                        [
                          computed(vm$.state.loading, (v) =>
                            v ? "加载中..." : "加载更多",
                          ),
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

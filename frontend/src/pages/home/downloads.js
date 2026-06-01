import { DownloadTaskStatus, DownloadsPageModel } from "./downloads.model.js";

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
      View({ class: "mt-2 text-2xl font-semibold text-zinc-950 dark:text-zinc-50" }, [value]),
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
  if (status === DownloadTaskStatus.Running) return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  if (status === DownloadTaskStatus.Done) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  if (status === DownloadTaskStatus.Error) return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
  if (status === DownloadTaskStatus.Paused) return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
  return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
}

function TaskCard(task, vm$) {
  const canPlay = task.status === DownloadTaskStatus.Done;
  return View(
    {
      class:
        "group rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View({ class: "flex gap-4" }, [
        View(
          {
            class:
              "h-16 w-16 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
          },
          [
            task.cover_url
              ? Img({ class: "h-full w-full object-cover", src: task.cover_url, alt: task.title || "cover" })
              : View({ class: "flex h-full w-full items-center justify-center text-zinc-400" }, [
                  Icon({ name: "file-video", size: 24 }),
                ]),
          ],
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
              View({ class: "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400" }, [
                task.filepath || task.url || "-",
              ]),
            ]),
            View(
              {
                class:
                  "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium " + statusClass(task.status),
              },
              [task.status_text],
            ),
          ]),
          View({ class: "mt-3 space-y-2" }, [
            View({ class: "h-2 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900" }, [
              View({
                class: "h-full rounded-full bg-zinc-900 dark:bg-zinc-100",
                style: {
                  width: `${task.progress_info.percent}%`,
                },
              }),
            ]),
            View({ class: "flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
              `${task.progress_info.percent}%`,
              task.size_text,
              task.status === DownloadTaskStatus.Running ? task.speed_text : "",
              `更新 ${task.updated_at_text}`,
            ]),
          ]),
          View({ class: "mt-3 flex flex-wrap gap-2" }, [
            task.status === DownloadTaskStatus.Error
              ? SmallButton("重试", () => vm$.methods.retry(task), "outline")
              : null,
            task.status === DownloadTaskStatus.Ready || task.status === DownloadTaskStatus.Paused
              ? SmallButton("开始", () => vm$.methods.start(task), "outline")
              : null,
            canPlay ? SmallButton("播放", () => vm$.methods.play(task), "outline") : null,
            SmallButton("定位", () => vm$.methods.openFile(task)),
            SmallButton("删除", () => vm$.methods.remove(task), "ghost"),
          ]),
        ]),
      ]),
    ],
  );
}

export default function DownloadsPageView(props) {
  const vm$ = DownloadsPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
    },
    [
      View({ class: "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950" }, [
        View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
          View({}, [
            View({ class: "text-2xl font-semibold text-zinc-950 dark:text-zinc-50" }, ["下载列表"]),
            View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, ["管理视频号下载任务和本地文件"]),
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
          HeaderStat("任务总数", computed(vm$.state.total, (v) => String(v)), "hard-drive"),
          HeaderStat("下载中", computed(vm$.state.runningCount, (v) => String(v)), "activity"),
          HeaderStat("总速度", vm$.state.totalSpeed, "gauge"),
        ]),
        View({ class: "mt-4 flex flex-wrap gap-2" }, [
          For({
            each: vm$.state.tabs,
            render(tab) {
              return View(
                {
                  class: computed(vm$.state.activeStatus, (v) => {
                    const active = v === tab.value;
                    return active
                      ? "cursor-pointer rounded-md bg-zinc-900 px-3 py-1.5 text-sm text-white dark:bg-zinc-100 dark:text-zinc-900"
                      : "cursor-pointer rounded-md border border-zinc-200 px-3 py-1.5 text-sm text-zinc-600 hover:bg-zinc-100 dark:border-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-800";
                  }),
                  onClick() {
                    vm$.methods.filter(tab.value);
                  },
                },
                [tab.label],
              );
            },
          }),
        ]),
      ]),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "space-y-3 p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View({ class: "rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300" }, [
                vm$.state.error,
              ]);
            },
          }),
          Show({
            when: computed(vm$.state.tasks, (list) => list.length === 0),
            ok() {
              return View({ class: "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500" }, [
                Icon({ name: "inbox", size: 36 }),
                computed(vm$.state.loading, (loading) => (loading ? "加载中..." : "暂无下载任务")),
              ]);
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
                        [computed(vm$.state.loading, (v) => (v ? "加载中..." : "加载更多"))],
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

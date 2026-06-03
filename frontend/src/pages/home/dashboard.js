import { HomeDashboardPageModel } from "./dashboard.model.js";

function StatCard(props) {
  const { label, value, icon, desc } = props;
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
          Show({
            when: desc,
            ok() {
              return View(
                { class: "mt-1 text-xs text-zinc-400 dark:text-zinc-500" },
                [desc],
              );
            },
          }),
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

/**
 * @param {ViewComponentProps} props
 */
export default function DashboardPageView(props) {
  const vm$ = HomeDashboardPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.refresh();
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
                "数据统计和快捷任务入口",
              ]),
            ]),
            Button({ store: vm$.ui.btn_refresh_stats }, [
              Icon({ name: "refresh-cw", size: 16 }),
              computed(vm$.state.loading, (v) => (v ? "刷新中..." : "刷新")),
            ]),
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.$view, class: "flex-1" }, [
        View({ class: "space-y-6 p-6" }, [
          Show({
            when: computed(vm$.state.error, (t) => !!t),
            ok() {
              return View(
                {
                  class:
                    "rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
                },
                [vm$.state.error],
              );
            },
          }),
          View(
            {
              class:
                "rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
            },
            [
              View(
                {
                  class: "flex flex-wrap items-center justify-between gap-3",
                },
                [
                  View({}, [
                    View(
                      {
                        class:
                          "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
                      },
                      ["创建下载任务"],
                    ),
                  ]),
                ],
              ),
              View({ class: "mt-4 flex flex-col gap-3 sm:flex-row" }, [
                View({ class: "min-w-0 flex-1" }, [
                  Input({ store: vm$.ui.taskUrlInput$ }),
                ]),
                Button({ store: vm$.ui.btn_create_task$ }, [
                  Icon({ name: "download", size: 16 }),
                  computed(vm$.state.creatingTask, (v) =>
                    v ? "创建中..." : "下载",
                  ),
                ]),
              ]),
              View({ class: "mt-3 flex items-center gap-2" }, [
                Checkbox({
                  id: "download-cover-checkbox",
                  store: vm$.ui.downloadCoverCheckbox$,
                }),
                Label(
                  {
                    for: "download-cover-checkbox",
                    class:
                      "cursor-pointer text-sm text-zinc-700 dark:text-zinc-300",
                  },
                  ["同时下载封面"],
                ),
              ]),
            ],
          ),
          View({ class: "grid gap-4 md:grid-cols-2 xl:grid-cols-4" }, [
            StatCard({
              label: "帐号数",
              value: computed(vm$.state.stats, (v) => String(v.accounts)),
              icon: "users",
              desc: "已记录的视频号帐号",
            }),
            StatCard({
              label: "视频数",
              value: computed(vm$.state.stats, (v) => String(v.videos)),
              icon: "film",
              desc: "数据库中的视频条目",
            }),
            StatCard({
              label: "浏览记录数",
              value: computed(vm$.state.stats, (v) => String(v.browse)),
              icon: "history",
              desc: "已捕获的页面访问记录",
            }),
            StatCard({
              label: "下载任务数",
              value: computed(vm$.state.stats, (v) => String(v.downloads)),
              icon: "hard-drive-download",
              desc: "全部下载任务",
            }),
          ]),
        ]),
      ]),
    ],
  );
}

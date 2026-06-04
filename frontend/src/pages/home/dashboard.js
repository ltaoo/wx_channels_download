/* global Img, Select */
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

function platformLabel(platform) {
  const map = {
    wx_channels: "视频号",
    douyin: "抖音",
    zhihu: "知乎",
    officialaccount: "公众号",
    youtube: "YouTube",
  };
  return map[platform] || platform || "-";
}

function existingTaskText(list) {
  const total = Array.isArray(list) ? list.length : 0;
  if (!total) return "";
  const latest = list[0] || {};
  const statusMap = {
    0: "待下载",
    1: "下载中",
    2: "已暂停",
    3: "排队中",
    4: "已完成",
    5: "失败",
  };
  return `已存在 ${total} 个下载任务，最新状态：${statusMap[latest.status] || "未知"}`;
}

function formatJSON(value) {
  try {
    return JSON.stringify(value || {}, null, 2);
  } catch {
    return "{}";
  }
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
        vm$.methods.refresh();
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
      ScrollView({ store: vm$.ui.view$, class: "flex-1" }, [
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
                    v ? "创建中..." : "开始下载",
                  ),
                ]),
              ]),
              Show({
                when: vm$.state.probingTask,
                ok() {
                  return View(
                    {
                      class:
                        "mt-3 text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    ["正在解析链接..."],
                  );
                },
              }),
              Show({
                when: vm$.state.probeError,
                ok() {
                  return View(
                    {
                      class:
                        "mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                    },
                    [vm$.state.probeError],
                  );
                },
              }),
              Show({
                when: vm$.state.taskProbe,
                ok() {
                  return View(
                    {
                      class:
                        "mt-4 grid gap-4 border-t border-zinc-100 pt-4 dark:border-zinc-800 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]",
                    },
                    [
                      View({ class: "min-w-0" }, [
                        Show({
                          when: computed(
                            vm$.state.taskExisting,
                            (list) => Array.isArray(list) && list.length > 0,
                          ),
                          ok() {
                            return View(
                              {
                                class:
                                  "mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
                              },
                              [
                                computed(vm$.state.taskExisting, (list) =>
                                  existingTaskText(list),
                                ),
                              ],
                            );
                          },
                        }),
                        View({ class: "flex items-start gap-3" }, [
                          Show({
                            when: computed(
                              vm$.state.taskContent,
                              (content) => content?.cover_url,
                            ),
                            ok() {
                              return View(
                                {
                                  class:
                                    "h-16 w-16 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                                },
                                [
                                  Img({
                                    class: "h-full w-full object-cover",
                                    src: computed(
                                      vm$.state.taskContent,
                                      (content) => content?.cover_url,
                                    ),
                                    alt: "cover",
                                  }),
                                ],
                              );
                            },
                          }),
                          View({ class: "min-w-0 flex-1" }, [
                            View({ class: "flex flex-wrap items-center gap-2" }, [
                              View(
                                {
                                  class:
                                    "rounded-full bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200",
                                },
                                [
                                  computed(vm$.state.taskProbe, (probe) =>
                                    platformLabel(probe?.platform),
                                  ),
                                ],
                              ),
                              View(
                                {
                                  class:
                                    "min-w-0 truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                                },
                                [
                                  computed(
                                    vm$.state.taskContent,
                                    (content) => content?.title || "未命名内容",
                                  ),
                                ],
                              ),
                            ]),
                            View(
                              {
                                class:
                                  "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                              },
                              [
                                computed(vm$.state.taskContent, (content) => {
                                  return (
                                    content?.author || content?.external_id || "-"
                                  );
                                }),
                              ],
                            ),
                          ]),
                        ]),
                      ]),
                      View({ class: "grid gap-3 sm:grid-cols-2 lg:grid-cols-1" }, [
                        View({}, [
                          Label(
                            {
                              class:
                                "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                            },
                            ["下载内容"],
                          ),
                          Select({ store: vm$.ui.taskVariantSelect$ }),
                        ]),
                        View({}, [
                          Label(
                            {
                              class:
                                "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                            },
                            ["文件名"],
                          ),
                          Input({ store: vm$.ui.taskFilenameInput$ }),
                        ]),
                      ]),
                      View(
                        {
                          class:
                            "lg:col-span-2 overflow-auto rounded-md border border-zinc-200 bg-zinc-50 p-3 text-xs text-zinc-700 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-200",
                        },
                        [
                          View(
                            {
                              class:
                                "mb-2 font-medium text-zinc-500 dark:text-zinc-400",
                            },
                            ["预请求 JSON"],
                          ),
                          View(
                            {
                              as: "pre",
                              class:
                                "max-h-80 whitespace-pre-wrap break-words font-mono leading-relaxed",
                            },
                            [
                              computed(vm$.state.taskProbeRaw, (value) =>
                                formatJSON(value),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ],
                  );
                },
              }),
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

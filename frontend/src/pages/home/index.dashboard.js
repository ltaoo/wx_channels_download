import {
  createTask,
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
    createTask: new Timeless.RequestCore(createTask, {
      client: props.client,
    }),
  };
  const loading_ = ref(false);
  const error_ = ref("");
  const taskUrl_ = ref("");
  const creatingTask_ = ref(false);
  const taskUrlInput$ = new Timeless.ui.InputCore({
    placeholder: "粘贴视频号下载链接",
    onChange(value) {
      taskUrl_.as(value);
    },
  });
  const downloadCoverCheckbox$ = new Timeless.ui.CheckboxCore({
    defaultValue: false,
  });
  const stats_ = ref({
    accounts: 0,
    videos: 0,
    browse: 0,
    downloads: 0,
  });

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
  }

  async function createDownloadTaskFromURL() {
    const url = taskUrl_.value.trim();
    if (!url) {
      props.app.tip?.({ type: "warning", text: ["请输入下载链接"] });
      return;
    }
    creatingTask_.as(true);
    const result = await reqs.createTask.run({ url });
    const coverResult = downloadCoverCheckbox$.value
      ? await reqs.createTask.run({ url, cover: true })
      : null;
    creatingTask_.as(false);
    if (result.error) {
      props.app.tip?.({
        type: "error",
        text: [result.error.message || String(result.error)],
      });
      return;
    }
    if (coverResult?.error) {
      props.app.tip?.({
        type: "error",
        text: [coverResult.error.message || String(coverResult.error)],
      });
      return;
    }
    props.app.tip?.({
      type: "success",
      text: [coverResult ? "已创建下载任务和封面下载任务" : "已创建下载任务"],
    });
    taskUrl_.as("");
    taskUrlInput$.setValue?.("");
    await refresh();
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
                "数据统计和快捷任务入口",
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
            View(
              {
                class:
                  "rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
              },
              [
                View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
                  View({}, [
                    View(
                      {
                        class:
                          "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
                      },
                      ["创建下载任务"],
                    ),
                    View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                      "通过 /api/task/create 统一创建任务",
                    ]),
                  ]),
                ]),
                View({ class: "mt-4 flex flex-col gap-3 sm:flex-row" }, [
                  View({ class: "min-w-0 flex-1" }, [
                    Input({ store: taskUrlInput$ }),
                  ]),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        // disabled: creatingTask_,
                        onClick() {
                          createDownloadTaskFromURL();
                        },
                      }),
                    },
                    [
                      Icon({ name: "download", size: 16 }),
                      computed(creatingTask_, (v) =>
                        v ? "创建中..." : "下载",
                      ),
                    ],
                  ),
                ]),
                View({ class: "mt-3 flex items-center gap-2" }, [
                  Checkbox({
                    id: "download-cover-checkbox",
                    store: downloadCoverCheckbox$,
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
          ]),
        ],
      ),
    ],
  );
}

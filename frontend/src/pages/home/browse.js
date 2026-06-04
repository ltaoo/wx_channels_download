import { BrowseHistoryPageModel } from "./browse.model.js";
import { ProxyImg } from "@/components/proxy-img.js";

function AccountAvatar(account, context = {}) {
  const avatarURL = account.display_avatar_url || account.avatar_url || "";
  return View(
    {
      class:
        "h-6 w-6 shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
    },
    [
      Show({
        when: avatarURL,
        ok() {
          return ProxyImg({
            class: "h-full w-full object-cover",
            src: avatarURL,
            alt: account.nickname,
            platformId: context.platform_id,
            contentType: context.content_type || context.type,
          });
        },
        else() {
          return View(
            {
              class:
                "flex h-full w-full items-center justify-center text-sm font-medium text-zinc-500",
            },
            [String(account.nickname || "?").slice(0, 1)],
          );
        },
      }),
    ],
  );
}

function Badge(props) {
  const { label, variant } = props;
  const classes = {
    platform:
      "inline-flex h-6 items-center rounded-md bg-emerald-100 px-2.5 text-xs font-semibold text-emerald-800 ring-1 ring-inset ring-emerald-200 dark:bg-emerald-950 dark:text-emerald-200 dark:ring-emerald-800",
    type: "inline-flex h-6 items-center rounded-md bg-sky-100 px-2.5 text-xs font-semibold text-sky-800 ring-1 ring-inset ring-sky-200 dark:bg-sky-950 dark:text-sky-200 dark:ring-sky-800",
  };
  return View({ class: classes[variant] || classes.type }, [label]);
}

function BrowseCard(item, vm$, props) {
  const copyURL =
    item.copy_url || item.source_url || item.content_source_url || "";
  // const urlLabel = item.is_article ? "文章链接" : "ContentSourceURL";
  const coverURL = item.display_cover_url || item.cover_url;
  const contentBadge = (() => {
    if (item.is_article) {
      return "文章";
    }
    if (item.type === "answer") {
      return "回答";
    }
    if (item.type === "image") {
      return "图片";
    }
    if (item.type === "live") {
      return "直播";
    }
    if (item.type === "other") {
      return "其他";
    }
    return "视频";
  })();

  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View({ class: "min-w-0" }, [
        View(
          {
            class:
              "mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400",
          },
          [
            View({}, [item.updated_at_text]),
            View({}, [
              computed(item, (t) => {
                return t.visited_times > 1 ? `浏览 ${t.visited_times} 次` : "";
              }),
            ]),
          ],
        ),
      ]),
      View({ class: "mt-2 flex flex-wrap gap-2" }, [
        Badge({
          label: item.platform_label || item.platform_id || "未知平台",
          variant: "platform",
        }),
        Badge({ label: contentBadge, variant: "type" }),
      ]),
      View({ class: "mt-4" }, [
        View({ class: "flex items-center gap-2" }, [
          AccountAvatar(item.account, item),
          View(
            {
              class:
                "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
            },
            [item.account.nickname],
          ),
        ]),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex flex-wrap items-start justify-between gap-3" }, [
            View({ class: "flex gap-2" }, [
              // Button(
              //   {
              //     store: new Timeless.ui.ButtonCore({
              //       variant: "outline",
              //       size: "sm",
              //       onClick() {
              //         vm$.methods.download(item);
              //       },
              //     }),
              //   },
              //   [Icon({ name: "download", size: 15 }), "下载"],
              // ),
              // Button(
              //   {
              //     store: new Timeless.ui.ButtonCore({
              //       variant: "ghost",
              //       size: "sm",
              //       onClick() {
              //         vm$.methods.open(item);
              //       },
              //     }),
              //   },
              //   [Icon({ name: "external-link", size: 15 }), "打开"],
              // ),
            ]),
          ]),
          View({ class: "mt-3 flex gap-3" }, [
            Show({
              when: coverURL,
              ok() {
                return View(
                  {
                    class:
                      "h-20 w-28 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                  },
                  [
                    ProxyImg({
                      class: "h-full w-full object-cover",
                      src: coverURL,
                      alt: item.title,
                      platformId: item.platform_id,
                      contentType: item.content_type || item.type,
                    }),
                  ],
                );
              },
            }),
            View({ class: "min-w-0 flex-1" }, [
              View(
                {
                  class:
                    "line-clamp-2 text-sm font-medium text-zinc-800 dark:text-zinc-200",
                },
                [item.title],
              ),
              View({ class: "mt-2 flex min-w-0 items-center gap-2" }, [
                View(
                  {
                    class:
                      "min-w-0 flex-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                    // title: copyURL,
                  },
                  [`${copyURL || "-"}`],
                ),
                Button(
                  {
                    store: new Timeless.ui.ButtonCore({
                      variant: "ghost",
                      size: "sm",
                      disabled: !copyURL,
                      onClick() {
                        if (!copyURL) {
                          props.app.tip?.({
                            type: "warning",
                            text: ["链接为空"],
                          });
                          return;
                        }
                        props.app.copy(copyURL);
                        props.app.tip({
                          type: "success",
                          text: ["已复制"],
                        });
                      },
                    }),
                  },
                  [Icon({ name: "copy", size: 15 }), "复制"],
                ),
              ]),
            ]),
          ]),
        ]),
      ]),
    ],
  );
}

export default function BrowseHistoryPageView(props) {
  const vm$ = BrowseHistoryPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
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
                ["浏览记录"],
              ),
              View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "查看已捕获的视频号内容和公众号文章",
              ]),
            ]),
            View({ class: "flex min-w-[280px] gap-2" }, [
              View({ class: "flex-1" }, [Input({ store: vm$.ui.keyword })]),
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
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "space-y-3 p-6" }, [
          Show({
            when: computed(vm$.state.error, (t) => !!t),
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
            when: computed(vm$.state.filtered, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "history", size: 36 }),
                  computed(vm$.state.loading, (loading) => {
                    return loading ? "加载中..." : "暂无浏览记录";
                  }),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.filtered,
                  render(item) {
                    return BrowseCard(item, vm$, props);
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

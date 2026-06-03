import { AccountsPageModel } from "./accounts.model.js";

function Avatar(account) {
  const avatarURL = account.display_avatar_url || account.avatar_url || "";
  return View(
    {
      class:
        "h-11 w-11 shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
    },
    [
      avatarURL
        ? Img({
            class: "h-full w-full object-cover",
            src: avatarURL,
            alt: account.nickname,
          })
        : View(
            {
              class:
                "flex h-full w-full items-center justify-center text-sm font-medium text-zinc-500",
            },
            [String(account.nickname || "?").slice(0, 1)],
          ),
    ],
  );
}

function contentIconName(content) {
  switch (content.content_type || content.type) {
    case "video":
    case "short_video":
      return "film";
    case "article":
    case "html":
    case "blog":
      return "file-text";
    case "image":
    case "image_set":
      return "image";
    case "audio":
    case "podcast":
    case "music":
      return "music";
    default:
      return "file";
  }
}

function MediaThumb(content, onClick) {
  const coverURL = content.display_cover_url || content.cover_url || "";
  return View(
    {
      class: "h-[98px]",
    },
    [
      Show({
        when: coverURL,
        ok() {
          return Img({
            class:
              "h-full w-full object-cover transition group-hover:scale-105",
            src: coverURL,
            alt: content.title || "cover",
            onClick,
          });
        },
        else() {
          return View(
            {
              class:
                "flex h-full w-full items-center justify-center text-zinc-400",
              onClick,
            },
            [Icon({ name: contentIconName(content), size: 22 })],
          );
        },
      }),
    ],
  );
  // return AspectRatio(
  //   {
  //     ratio: 3 / 4,
  //     class:
  //       "overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900 cursor-pointer",
  //     onClick,
  //     title: video.title || "视频",
  //   },
  //   [
  //     Show({
  //       when: video.cover_url,
  //       ok() {
  //         return Img({
  //           class:
  //             "h-full w-full object-cover transition group-hover:scale-105",
  //           src: video.cover_url,
  //           alt: video.title || "cover",
  //         });
  //       },
  //       else() {
  //         return View(
  //           {
  //             class:
  //               "flex h-full w-full items-center justify-center text-zinc-400",
  //           },
  //           [Icon({ name: "film", size: 22 })],
  //         );
  //       },
  //     }),
  //   ],
  // );
}

function PlatformBadge(account) {
  return View(
    {
      class:
        "inline-flex h-6 shrink-0 items-center rounded-md border border-zinc-200 bg-zinc-50 px-2 text-xs font-medium text-zinc-600 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-300",
      title: account.platform_id || account.platform_name || "平台",
    },
    [account.platform_name || account.platform_id || "未知平台"],
  );
}

const CONTENT_FILTERS = [
  { value: "with", label: "有关联内容" },
  { value: "all", label: "全部帐号" },
  { value: "without", label: "无关联内容" },
];

function ContentFilterButton(option, vm$) {
  const active_ = computed(
    vm$.state.contentFilter,
    (value) => value === option.value,
  );
  return View(
    {
      as: "button",
      type: "button",
      class: Timeless.classNames([
        "h-9 whitespace-nowrap rounded-md border px-3 text-sm font-medium transition",
        computed(active_, (active) =>
          active
            ? "border-zinc-900 bg-zinc-900 text-white dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950"
            : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-200 dark:hover:bg-zinc-900",
        ),
      ]),
      onClick() {
        vm$.methods.setContentFilter(option.value);
      },
    },
    [option.label],
  );
}

export default function AccountsPageView(props) {
  const vm$ = AccountsPageModel(props);

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
                ["帐号"],
              ),
              View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "浏览已保存的平台帐号及最近内容",
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
          View(
            { class: "mt-4 flex flex-wrap items-center gap-3" },
            [
              View({ class: "w-full max-w-md" }, [Input({ store: vm$.ui.keyword })]),
              View({ class: "flex flex-wrap items-center gap-2" }, [
                For({
                  each: CONTENT_FILTERS,
                  render(option) {
                    return ContentFilterButton(option, vm$);
                  },
                }),
              ]),
            ],
          ),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "space-y-3 p-6" }, [
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
            when: computed(vm$.state.filtered, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "user-search", size: 36 }),
                  computed(vm$.state.loading, (loading) =>
                    loading ? "加载中..." : "暂无帐号",
                  ),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.filtered,
                  render(account) {
                    return View(
                      {
                        class:
                          "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
                      },
                      [
                        View({ class: "flex items-start gap-4" }, [
                          Avatar(account),
                          View({ class: "min-w-0 flex-1" }, [
                            View(
                              {
                                class:
                                  "flex flex-wrap items-start justify-between gap-3",
                              },
                              [
                                View({ class: "min-w-0" }, [
                                  View(
                                    {
                                      class:
                                        "flex min-w-0 flex-wrap items-center gap-2",
                                    },
                                    [
                                      View(
                                        {
                                          class:
                                            "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                                        },
                                        [account.nickname],
                                      ),
                                      PlatformBadge(account),
                                    ],
                                  ),
                                  View(
                                    {
                                      class:
                                        "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                                    },
                                    [
                                      account.external_id ||
                                        account.username ||
                                        "-",
                                    ],
                                  ),
                                ]),
                                View({ class: "flex items-center gap-2" }, [
                                  View({ class: "text-xs text-zinc-500" }, [
                                    `${account.content_count || account.medias.length} 个内容`,
                                  ]),
                                  Button(
                                    {
                                      store: new Timeless.ui.ButtonCore({
                                        variant: "outline",
                                        size: "sm",
                                        onClick() {
                                          vm$.methods.synchronize(account);
                                        },
                                      }),
                                    },
                                    ["同步"],
                                  ),
                                ]),
                              ],
                            ),
                            Show({
                              when: account.medias.length > 0,
                              ok() {
                                return View(
                                  {
                                    class:
                                      "mt-4 grid grid-cols-3 gap-2 sm:grid-cols-5 lg:grid-cols-8",
                                  },
                                  [
                                    For({
                                      each: account.medias.slice(0, 10),
                                      render(content) {
                                        return MediaThumb(content, () =>
                                          vm$.methods.play(content),
                                        );
                                      },
                                    }),
                                  ],
                                );
                              },
                            }),
                          ]),
                        ]),
                      ],
                    );
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

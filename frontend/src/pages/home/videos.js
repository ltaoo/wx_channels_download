import { VideosPageModel } from "./videos.model.js";

function VideoCard(video, vm$) {
  const accountName = video.account?.nickname || video.account?.username || "";
  const description = String(video.description || "").trim();
  const title = String(video.title || "").trim();
  const visibleDescription = description && description !== title ? description : "";

  return View(
    {
      class:
        "group h-full cursor-pointer overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm transition hover:border-zinc-300 hover:shadow-md dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
      onClick() {
        vm$.methods.play(video);
      },
    },
    [
      View({ class: "relative aspect-[3/4] bg-zinc-100 dark:bg-zinc-900" }, [
        video.cover_url
          ? Img({ class: "h-full w-full object-cover transition group-hover:scale-105", src: video.cover_url, alt: video.title })
          : View({ class: "flex h-full w-full items-center justify-center text-zinc-400" }, [
              Icon({ name: "film", size: 36 }),
            ]),
        View({ class: "absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent p-3 text-white" }, [
          View({ class: "line-clamp-2 text-sm font-medium" }, [video.title]),
          View({ class: "mt-1 flex items-center justify-between text-xs text-white/80" }, [
            vm$.methods.formatBytes(video.file_size),
            video.publish_time ? vm$.methods.formatDate(Number(video.publish_time) * 1000) : "",
          ]),
        ]),
      ]),
      View({ class: "space-y-2 p-3" }, [
        Show({
          when: !!video.account,
          ok() {
            return View({ class: "flex items-center gap-2" }, [
              View(
                {
                  class:
                    "h-7 w-7 shrink-0 overflow-hidden rounded-full bg-zinc-100 ring-1 ring-zinc-200 dark:bg-zinc-900 dark:ring-zinc-800",
                },
                [
                  video.account.avatar_url
                    ? Img({
                        class: "h-full w-full object-cover",
                        src: video.account.avatar_url,
                        alt: accountName,
                      })
                    : View(
                        {
                          class:
                            "flex h-full w-full items-center justify-center text-xs font-medium text-zinc-500 dark:text-zinc-400",
                        },
                        [String(accountName || "?").slice(0, 1)],
                      ),
                ],
              ),
              View({ class: "min-w-0 truncate text-sm font-medium text-zinc-700 dark:text-zinc-300" }, [
                accountName || "未知帐号",
              ]),
            ]);
          },
        }),
        Show({
          when: !!visibleDescription,
          ok() {
            return View({ class: "line-clamp-2 text-sm text-zinc-500 dark:text-zinc-400" }, [visibleDescription]);
          },
        }),
      ]),
    ],
  );
}

export default function VideosPageView(props) {
  const vm$ = VideosPageModel(props);

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
            View({ class: "text-2xl font-semibold text-zinc-950 dark:text-zinc-50" }, ["视频列表"]),
            View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, ["查看已归档的视频号内容"]),
          ]),
          View({ class: "flex min-w-[280px] gap-2" }, [
            View({ class: "flex-1" }, [Input({ store: vm$.ui.keyword })]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.search();
                  },
                }),
              },
              ["搜索"],
            ),
          ]),
        ]),
      ]),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View({ class: "mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300" }, [
                vm$.state.error,
              ]);
            },
          }),
          Show({
            when: computed(vm$.state.videos, (list) => list.length === 0),
            ok() {
              return View({ class: "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500" }, [
                Icon({ name: "film", size: 36 }),
                computed(vm$.state.loading, (loading) => (loading ? "加载中..." : "暂无视频")),
              ]);
            },
            else() {
              return View({ class: "space-y-5" }, [
                View({ class: "grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-5" }, [
                  For({
                    each: vm$.state.videos,
                    render(video) {
                      return VideoCard(video, vm$);
                    },
                  }),
                ]),
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

import { BrowseHistoryPageModel } from "./browse.model.js";

function AccountAvatar(account) {
  return View(
    {
      class:
        "h-11 w-11 shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
    },
    [
      account.avatar_url
        ? Img({ class: "h-full w-full object-cover", src: account.avatar_url, alt: account.nickname })
        : View({ class: "flex h-full w-full items-center justify-center text-sm font-medium text-zinc-500" }, [
            String(account.nickname || "?").slice(0, 1),
          ]),
    ],
  );
}

function BrowseCard(item, vm$) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View({ class: "flex items-start gap-4" }, [
        AccountAvatar(item.account),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex flex-wrap items-start justify-between gap-3" }, [
            View({ class: "min-w-0" }, [
              View({ class: "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50" }, [
                item.account.nickname,
              ]),
              View({ class: "mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
                item.updated_at_text,
                item.visited_times > 1 ? `浏览 ${item.visited_times} 次` : "",
              ]),
            ]),
            View({ class: "flex gap-2" }, [
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    size: "sm",
                    onClick() {
                      vm$.methods.download(item);
                    },
                  }),
                },
                [Icon({ name: "download", size: 15 }), "下载"],
              ),
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "ghost",
                    size: "sm",
                    onClick() {
                      vm$.methods.open(item);
                    },
                  }),
                },
                [Icon({ name: "external-link", size: 15 }), "打开"],
              ),
            ]),
          ]),
          View({ class: "mt-3 flex gap-3" }, [
            View(
              {
                class:
                  "h-20 w-28 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
              },
              [
                item.cover_url
                  ? Img({ class: "h-full w-full object-cover", src: item.cover_url, alt: item.title })
                  : View({ class: "flex h-full w-full items-center justify-center text-zinc-400" }, [
                      Icon({ name: "film", size: 24 }),
                    ]),
              ],
            ),
            View({ class: "min-w-0 flex-1" }, [
              View({ class: "line-clamp-2 text-sm font-medium text-zinc-800 dark:text-zinc-200" }, [
                item.title,
              ]),
              View({ class: "mt-2 truncate text-xs text-zinc-500 dark:text-zinc-400" }, [
                item.source_url || item.url || "-",
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
      View({ class: "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950" }, [
        View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
          View({}, [
            View({ class: "text-2xl font-semibold text-zinc-950 dark:text-zinc-50" }, ["浏览记录"]),
            View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
              "查看已捕获的视频号浏览内容并快速下载",
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
            when: computed(vm$.state.filtered, (list) => list.length === 0),
            ok() {
              return View({ class: "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500" }, [
                Icon({ name: "history", size: 36 }),
                computed(vm$.state.loading, (loading) => (loading ? "加载中..." : "暂无浏览记录")),
              ]);
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.filtered,
                  render(item) {
                    return BrowseCard(item, vm$);
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

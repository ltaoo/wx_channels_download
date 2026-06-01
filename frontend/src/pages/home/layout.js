/** 首页布局 */

const HOME_MENUS = [
  { title: "下载", name: "root.home_layout.index", icon: "hard-drive" },
  { title: "帐号", name: "root.home_layout.accounts", icon: "user" },
  { title: "视频", name: "root.home_layout.videos", icon: "film" },
  { title: "浏览记录", name: "root.home_layout.browse", icon: "history" },
  { title: "工具", name: "root.home_layout.tools", icon: "wrench" },
  { title: "设置", name: "root.home_layout.settings", icon: "settings" },
];

function MenuButton(props, item, sidemenu$) {
  return View(
    {
      class: computed(sidemenu$.cur, (cur) => {
        const active = sidemenu$.isSelected(cur, item);
        return active
          ? "relative flex w-full cursor-pointer items-center gap-3 rounded-lg bg-white px-4 py-3 text-sm font-medium text-zinc-950 shadow-sm ring-1 ring-black/5 dark:bg-zinc-900 dark:text-zinc-50 dark:ring-white/10"
          : "relative flex w-full cursor-pointer items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium text-zinc-500 transition hover:bg-white/70 hover:text-zinc-950 dark:text-zinc-400 dark:hover:bg-zinc-900/70 dark:hover:text-zinc-50";
      }),
      title: item.title,
      onClick() {
        props.history.push(item.name);
      },
    },
    [
      Show({
        when: computed(sidemenu$.cur, (cur) => sidemenu$.isSelected(cur, item)),
        ok() {
          return View({
            class:
              "absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r-full bg-zinc-900 dark:bg-zinc-100",
          });
        },
      }),
      View({ class: "flex h-5 w-5 shrink-0 items-center justify-center" }, [
        Icon({ name: item.icon, size: 20 }),
      ]),
      View({ class: "truncate" }, [item.title]),
    ],
  );
}

/**
 * @param {ViewComponentProps} props
 */
export default function HomeLayoutView(props) {
  const sidemenu$ = Timeless.RouteMenusModel({
    view: props.view,
    history: props.history,
    menus: HOME_MENUS,
  });

  return SplitView({
    resizable: false,
    panels: [
      {
        size: 260,
        style: { overflow: "hidden" },
        content() {
          return View(
            {
              class:
                "h-full border-r border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-950",
            },
            [
              Flex(
                {
                  direction: "col",
                  justify: "between",
                  class: "h-full gap-4",
                },
                [
                  Flex({ direction: "col", class: "gap-5" }, [
                    View(
                      {
                        class: "flex items-center gap-3 px-2 py-2 cursor-pointer",
                        onClick() {
                          props.history.push("root.home_layout.index");
                        },
                      },
                      [
                        View(
                          {
                            class:
                              "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-zinc-900 text-sm font-bold text-white shadow-sm dark:bg-zinc-100 dark:text-zinc-900",
                            title: "WX",
                          },
                          ["WX"],
                        ),
                        View({ class: "min-w-0" }, [
                          View(
                            {
                              class:
                                "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                            },
                            ["Channels Download"],
                          ),
                          View(
                            {
                              class:
                                "truncate text-xs text-zinc-500 dark:text-zinc-400",
                            },
                            ["视频号下载管理"],
                          ),
                        ]),
                      ],
                    ),
                    View({ class: "space-y-1" }, [
                      For({
                        each: HOME_MENUS,
                        render(item) {
                          return MenuButton(props, item, sidemenu$);
                        },
                      }),
                    ]),
                  ]),
                  View(
                    {
                      class:
                        "rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900",
                    },
                    [
                      View(
                        {
                          class:
                            "mb-3 text-xs font-medium uppercase text-zinc-500 dark:text-zinc-400",
                        },
                        ["Display"],
                      ),
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            onClick() {
                              const cur = props.app.getTheme();
                              props.app.setTheme(
                                cur === "dark" ? "light" : "dark",
                              );
                            },
                          }),
                        },
                        [Icon({ name: "sun", size: 16 }), "切换主题"],
                      ),
                    ],
                  ),
                ],
              ),
            ],
          );
        },
      },
      {
        size: "auto",
        content() {
          return KeepAliveSubViews(props);
        },
      },
    ],
  });
}

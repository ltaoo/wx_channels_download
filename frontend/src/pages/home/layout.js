/** 首页布局 */
/**
 * @param {ViewComponentProps} props
 */
export default function HomeLayoutView(props) {
  const sidemenu$ = Timeless.RouteMenusModel({
    view: props.view,
    history: props.history,
    menus: [
      { title: "书架", name: "root.home_layout.books", children: [] },
      { title: "翻书", name: "root.home_layout.flipbook" },
      { title: "Article", name: "root.home_layout.article" },
      { title: "Project", name: "root.home_layout.project" },
      { title: "Settings", name: "root.home_layout.settings" },
      { title: "Tools", name: "root.home_layout.tools" },
      { title: "Chat", name: "root.home_layout.chat" },
      { title: "Sandboxes", name: "root.home_layout.sandboxes" },
    ],
  });

  return SplitView({
    resizable: false,
    panels: [
      {
        size: 72,
        style: { overflow: "hidden" },
        content() {
          return View(
            {
              class:
                "sidebar-wrapper py-6 border-r border-zinc-200 dark:border-zinc-800 h-full",
            },
            [
              Flex(
                {
                  direction: "col",
                  items: "center",
                  justify: "between",
                  class: "h-full",
                },
                [
                  // Logo
                  Flex(
                    {
                      items: "center",
                      justify: "center",
                      class:
                        "relative w-10 h-10 rounded-xl font-bold text-xl mb-8 shadow-sm cursor-pointer hover:opacity-90 transition-opacity",
                      onClick() {
                        props.history.push("root.home_layout.books");
                      },
                    },
                    [
                      "T",
                      Show({
                        when: computed(sidemenu$.cur, (t) => {
                          return sidemenu$.isSelected(t, sidemenu$.menus[0]);
                        }),
                        ok() {
                          return View({
                            class:
                              "absolute top-[-4px] right-[-4px] w-2 h-2 rounded-full bg-zinc-500",
                          });
                        },
                      }),
                    ],
                  ),

                  // Middle spacer
                  Flex(
                    {
                      direction: "col",
                      items: "center",
                      class: "flex-1 flex gap-3",
                    },
                    [
                      // Flipbook icon
                      View(
                        {
                          class: computed(sidemenu$.cur, (t) => {
                            return sidemenu$.isSelected(t, sidemenu$.menus[1])
                              ? "w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center text-zinc-700 cursor-pointer transition-colors dark:bg-zinc-800 dark:text-white"
                              : "w-10 h-10 rounded-lg hover:bg-zinc-100 flex items-center justify-center text-zinc-500 hover:text-black cursor-pointer transition-colors dark:hover:bg-zinc-800 dark:hover:text-white";
                          }),
                          onClick() {
                            props.history.push("root.home_layout.flipbook");
                          },
                        },
                        [Icon({ name: "book-open", size: 24 })],
                      ),
                      // Article icon
                      View(
                        {
                          class: computed(sidemenu$.cur, (t) => {
                            return sidemenu$.isSelected(t, sidemenu$.menus[2])
                              ? "w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center text-zinc-700 cursor-pointer transition-colors dark:bg-zinc-800 dark:text-white"
                              : "w-10 h-10 rounded-lg hover:bg-zinc-100 flex items-center justify-center text-zinc-500 hover:text-black cursor-pointer transition-colors dark:hover:bg-zinc-800 dark:hover:text-white";
                          }),
                          onClick() {
                            props.history.push("root.home_layout.article");
                          },
                        },
                        [Icon({ name: "rss", size: 24 })],
                      ),
                      View(
                        {
                          class: computed(sidemenu$.cur, (t) => {
                            return sidemenu$.isSelected(t, sidemenu$.menus[5])
                              ? "w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center text-zinc-700 cursor-pointer transition-colors dark:bg-zinc-800 dark:text-white"
                              : "w-10 h-10 rounded-lg hover:bg-zinc-100 flex items-center justify-center text-zinc-500 hover:text-black cursor-pointer transition-colors dark:hover:bg-zinc-800 dark:hover:text-white";
                          }),
                          onClick() {
                            props.history.push("root.home_layout.chat");
                          },
                        },
                        [Icon({ name: "message-square-more", size: 24 })],
                      ),
                      View(
                        {
                          class: computed(sidemenu$.cur, (t) => {
                            return sidemenu$.isSelected(t, sidemenu$.menus[6])
                              ? "w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center text-zinc-700 cursor-pointer transition-colors dark:bg-zinc-800 dark:text-white"
                              : "w-10 h-10 rounded-lg hover:bg-zinc-100 flex items-center justify-center text-zinc-500 hover:text-black cursor-pointer transition-colors dark:hover:bg-zinc-800 dark:hover:text-white";
                          }),
                          onClick() {
                            props.history.push("root.home_layout.sandboxes");
                          },
                        },
                        [Icon({ name: "container", size: 24 })],
                      ),
                      Separator({
                        orientation: "horizontal",
                        class: "w-8 mx-auto",
                      }),
                      Flex(
                        { direction: "col", items: "center", class: "gap-2" },
                        [],
                      ),
                    ],
                  ),

                  // Bottom Actions
                  Flex(
                    { direction: "col", items: "center", class: "gap-6 mb-4" },
                    [
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            onClick() {
                              props.history.push("root.admin_layout.dashboard");
                            },
                          }),
                        },
                        [Icon({ name: "grid-3x3", size: 24 })],
                      ),
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            onClick() {
                              const cur = props.app.getTheme();
                              const next = cur === "dark" ? "light" : "dark";
                              props.app.setTheme(next);
                            },
                          }),
                        },
                        [Icon({ name: "sun", size: 24 })],
                      ),
                      // User Avatar
                      (() => {
                        const dropdown$ = new Timeless.ui.DropdownMenuCore({
                          trigger: "hover",
                          side: "right",
                          align: "end",
                          offsetX: 4,
                          offsetY: -8,
                          items: [
                            new Timeless.ui.MenuItemCore({
                              label: "Profile",
                              onClick() {
                                const toasts = [
                                  {
                                    type: "success",
                                    text: [
                                      "Welcome back!",
                                      "Have a productive day.",
                                    ],
                                  },
                                  {
                                    type: "success",
                                    text: ["Profile updated successfully."],
                                  },
                                  {
                                    type: "info",
                                    text: ["You have 3 unread notifications."],
                                  },
                                  {
                                    type: "info",
                                    text: [
                                      "Session active",
                                      "Last login: 2 hours ago.",
                                    ],
                                  },
                                  {
                                    type: "loading",
                                    text: [
                                      "Syncing your data...",
                                      "This may take a moment.",
                                    ],
                                  },
                                  {
                                    type: "warning",
                                    text: [
                                      "Storage almost full.",
                                      "Consider cleaning up.",
                                    ],
                                  },
                                  {
                                    type: "error",
                                    text: [
                                      "Connection lost.",
                                      "Retrying in 5s...",
                                    ],
                                  },
                                  { text: ["All systems operational."] },
                                ];
                                const pick =
                                  toasts[
                                    Math.floor(Math.random() * toasts.length)
                                  ];
                                // @ts-ignore
                                props.app.tip(pick);
                              },
                            }),
                            new Timeless.ui.MenuItemCore({
                              label: "Bill",
                              onClick() {
                                console.log("Bill clicked");
                              },
                            }),
                            new Timeless.ui.MenuItemCore({
                              label: "Other",
                              menu: new Timeless.ui.MenuCore({
                                items: [
                                  new Timeless.ui.MenuItemCore({
                                    label: "Toast",
                                    onClick() {
                                      const toasts = [
                                        { text: ["Task completed!"] },
                                        {
                                          text: [
                                            "File saved.",
                                            "Auto-backup enabled.",
                                          ],
                                        },
                                        {
                                          text: [
                                            "Reminder:",
                                            "Meeting starts in 15 minutes.",
                                          ],
                                        },
                                        {
                                          text: [
                                            "Download finished.",
                                            "3 files ready.",
                                          ],
                                        },
                                        { text: ["Settings applied."] },
                                      ];
                                      const pick =
                                        toasts[
                                          Math.floor(
                                            Math.random() * toasts.length,
                                          )
                                        ];
                                      props.app.tip(pick);
                                    },
                                  }),
                                  new Timeless.ui.MenuItemCore({
                                    label: "Item 2",
                                    onClick() {
                                      console.log("Item 2 clicked");
                                    },
                                  }),
                                  new Timeless.ui.MenuItemCore({
                                    label: "Item 3",
                                    onClick() {
                                      console.log("Item 3 clicked");
                                    },
                                  }),
                                  new Timeless.ui.MenuItemCore({
                                    label: "Close DropdownMenu",
                                    onClick() {
                                      console.log("Item 4 clicked");
                                      dropdown$.hide();
                                    },
                                  }),
                                  new Timeless.ui.MenuItemCore({
                                    label: "Item 5",
                                    onClick() {
                                      console.log("Item 5 clicked");
                                    },
                                  }),
                                ],
                              }),
                              onClick() {
                                console.log("Other clicked");
                              },
                            }),
                            new Timeless.ui.MenuItemCore({
                              label: "Logout",
                              onClick() {
                                console.log("Logout clicked");
                                dropdown$.hide();
                                props.history.destroyAllAndPush("root.login");
                              },
                            }),
                          ],
                        });
                        return DropdownMenu(
                          {
                            store: dropdown$,
                          },
                          [
                            View(
                              {
                                class:
                                  "w-10 h-10 rounded-full bg-zinc-100 overflow-hidden cursor-pointer border border-zinc-200 hover:ring-2 ring-zinc-200 transition-all dark:bg-zinc-800 dark:border-zinc-700 dark:ring-zinc-700",
                                onClick() {
                                  console.log("Avatar clicked");
                                },
                              },
                              [
                                Img({
                                  class: "w-full h-full object-cover",
                                  src: "public/avatar.jpeg",
                                  alt: "User Avatar",
                                }),
                              ],
                            ),
                          ],
                        );
                      })(),
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

import { app, history$, client$, views, storage$ } from "./store/index.js";
import NotFoundPageView from "./pages/notfound/index.js";

function ErrorFallbackView(error, viewName) {
  return View(
    {
      class:
        "flex flex-col items-center justify-center gap-4 p-8 min-h-[200px]",
    },
    [
      View(
        {
          class:
            "w-full max-w-lg rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/30 p-6",
        },
        [
          View({ class: "flex items-center gap-2 mb-3" }, [
            View(
              {
                as: "span",
                class: "text-red-500 dark:text-red-400 text-lg",
              },
              ["\u26A0"],
            ),
            View(
              {
                as: "span",
                class: "text-sm font-semibold text-red-700 dark:text-red-300",
              },
              [`"${viewName}" render error`],
            ),
          ]),
          View(
            {
              as: "pre",
              class:
                "text-xs text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-950/50 rounded p-3 overflow-auto max-h-[200px] whitespace-pre-wrap break-words font-mono",
            },
            [error.message],
          ),
          Show({
            when: !!error.stack,
            ok() {
              return View(
                {
                  as: "details",
                  class: "mt-3",
                },
                [
                  View(
                    {
                      as: "summary",
                      class:
                        "text-xs text-red-500 dark:text-red-400 cursor-pointer select-none",
                    },
                    ["Stack trace"],
                  ),
                  View(
                    {
                      as: "pre",
                      class:
                        "mt-2 text-xs text-red-500/80 dark:text-red-400/60 bg-red-100 dark:bg-red-950/50 rounded p-3 overflow-auto whitespace-pre-wrap break-words font-mono",
                    },
                    [error.stack],
                  ),
                ],
              );
            },
          }),
        ],
      ),
    ],
  );
}

function ApplicationRootView() {
  const root_view$ = history$.$view;
  const toaster$ = Timeless.ui.ToasterModel({ position: "top-center" });
  const icon_name_ = ref("info");

  app.onTip((msg) => {
    const { text, type } = msg;
    const content = View(
      {
        class: "flex items-center gap-4 p-4",
        onMounted() {
          setTimeout(() => {
            icon_name_.as("check");
          }, 1000);
        },
      },
      [
        View({}, [
          Show({
            when: computed(icon_name_, (t) => t === "check"),
            ok() {
              return Icon({ name: "check", size: 16 });
            },
            else() {
              return Icon({ name: "loader", size: 16 });
            },
          }),
        ]),
        View({}, [
          For({
            each: text,
            render(t) {
              return View({}, [t]);
            },
          }),
        ]),
      ],
    );
    const method = type && toaster$[type] ? type : "message";
    toaster$[method](content);
  });
  app.onError((err) => {
    console.error(err);
  });

  // return View(
  //   {
  //     style: {
  //       width: "320px",
  //     },
  //   },
  //   [
  //     DropdownMenu(
  //       {
  //         store: new Timeless.ui.DropdownMenuCore({
  //           trigger: "click",
  //           items: [
  //             new Timeless.ui.MenuItemCore({
  //               label: "Cut",
  //               onClick() {
  //                 console.log("cut");
  //               },
  //             }),
  //             new Timeless.ui.MenuItemCore({
  //               label: "Copy",
  //               menu: new Timeless.ui.MenuCore({
  //                 items: [
  //                   new Timeless.ui.MenuItemCore({
  //                     label: "CopySubMenu",
  //                     onClick() {
  //                       console.log("CopySubMenu");
  //                     },
  //                   }),
  //                 ],
  //               }),
  //               onClick() {
  //                 console.log("copy");
  //               },
  //             }),
  //             new Timeless.ui.MenuItemCore({
  //               label: "Share",
  //               menu: new Timeless.ui.MenuCore({
  //                 _name: "3",
  //                 items: [
  //                   new Timeless.ui.MenuItemCore({
  //                     label: "Email",
  //                     onClick() {
  //                       console.log("email");
  //                     },
  //                   }),
  //                   new Timeless.ui.MenuItemCore({
  //                     label: "Message",
  //                     menu: new Timeless.ui.MenuCore({
  //                       _name: "3-2",
  //                       items: [
  //                         new Timeless.ui.MenuItemCore({
  //                           label: "Wechat",
  //                           onClick() {
  //                             console.log("wechat message");
  //                           },
  //                         }),
  //                         new Timeless.ui.MenuItemCore({
  //                           label: "QQ",
  //                           onClick() {
  //                             console.log("QQ message");
  //                           },
  //                         }),
  //                         new Timeless.ui.MenuItemCore({
  //                           label: "Telegram",
  //                           onClick() {
  //                             console.log("Telegram message");
  //                           },
  //                         }),
  //                       ],
  //                     }),
  //                     onClick() {
  //                       console.log("message");
  //                     },
  //                   }),
  //                 ],
  //               }),
  //             }),
  //           ],
  //         }),
  //       },
  //       [
  //         Button(
  //           {
  //             store: new Timeless.ui.ButtonCore({}),
  //           },
  //           ["Click it"],
  //         ),
  //       ],
  //     ),
  //   ],
  // );

  return Fragment({}, [
    ErrorBoundary(
      {
        fallback(error) {
          return ErrorFallbackView(error);
        },
      },
      () => [
        StandardSubViews({
          view: root_view$,
          views,
          history: history$,
          app,
          client: client$,
          storage: storage$,
          NotFound: NotFoundPageView,
          ErrorFallback: ErrorFallbackView,
        }),
      ],
    ),
    Portal({}, [Toaster({ store: toaster$, position: "top-center" })]),
  ]);
}

document.addEventListener("DOMContentLoaded", function () {
  const { innerWidth, innerHeight, location } = window;
  history$.$router.prepare(location);
  app.start({
    width: innerWidth,
    height: innerHeight,
  });
  Timeless.DOM.render(ApplicationRootView(), document.querySelector("#root"));
});

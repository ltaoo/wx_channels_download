export default function SettingsPageView(props) {
  return View({ class: "p-6 max-w-2xl space-y-8" }, [
    // Header Section
    View(
      { class: "space-y-4 border-b border-zinc-200 dark:border-zinc-800 pb-6" },
      [
        View(
          {
            class:
              "text-3xl font-bold tracking-tight text-zinc-900 dark:text-zinc-50",
          },
          ["Timeless"],
        ),
        View(
          { class: "text-lg text-zinc-500 dark:text-zinc-400 leading-relaxed" },
          ["定位为「可以写出具有长久生命力代码」的一套前端框架/脚手架"],
        ),
      ],
    ),

    // Content Section
    View({ class: "space-y-6" }, [
      View({ class: "text-base text-zinc-700 dark:text-zinc-300" }, [
        "核心功能都是端、框架无关的，包括",
      ]),

      // List
      View({ class: "space-y-3 pl-4" }, [
        View({ class: "flex items-center gap-3" }, [
          View({ class: "w-1.5 h-1.5 rounded-full bg-zinc-400 shrink-0" }),
          View({ class: "text-sm text-zinc-600 dark:text-zinc-400" }, [
            "接口请求",
          ]),
        ]),
        View({ class: "flex items-center gap-3" }, [
          View({ class: "w-1.5 h-1.5 rounded-full bg-zinc-400 shrink-0" }),
          View({ class: "text-sm text-zinc-600 dark:text-zinc-400" }, [
            "数据持久化",
          ]),
        ]),
        View({ class: "flex items-center gap-3" }, [
          View({ class: "w-1.5 h-1.5 rounded-full bg-zinc-400 shrink-0" }),
          View({ class: "text-sm text-zinc-600 dark:text-zinc-400" }, ["路由"]),
        ]),
        View({ class: "flex items-center gap-3" }, [
          View({ class: "w-1.5 h-1.5 rounded-full bg-zinc-400 shrink-0" }),
          View({ class: "text-sm text-zinc-600 dark:text-zinc-400" }, [
            "常用 UI 组件",
          ]),
        ]),
      ]),
    ]),
  ]);
}

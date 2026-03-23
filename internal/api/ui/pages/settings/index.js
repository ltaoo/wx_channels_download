/** 设置页 */
export default function SettingsPageView(props) {
  return View({ class: "p-6 max-w-2xl space-y-8" }, [
    // Header
    View(
      {
        class:
          "space-y-4 border-b border-[var(--weui-FG-3)] pb-6",
      },
      [
        View(
          { class: "text-3xl font-bold tracking-tight text-[var(--weui-FG-0)]" },
          [Txt("设置")],
        ),
        View(
          { class: "text-base text-[var(--weui-FG-1)] leading-relaxed" },
          [Txt("视频号下载助手配置")],
        ),
      ],
    ),

    // Download directory
    View({ class: "space-y-4" }, [
      View(
        { class: "text-lg font-medium text-[var(--weui-FG-0)]" },
        [Txt("下载目录")],
      ),
      View(
        {
          class:
            "flex items-center gap-3 p-4 rounded-lg bg-[var(--weui-BG-2)] border border-[var(--weui-FG-5)]",
        },
        [
          View(
            { class: "flex-1 text-sm text-[var(--weui-FG-1)] truncate" },
            [Txt("使用默认下载目录")],
          ),
          // Button(
          //   {
          //     store: new Timeless.ui.ButtonCore({
          //       onClick() {
          //         fetch("/api/open_download_dir", { method: "POST" });
          //       },
          //     }),
          //     class:
          //       "px-3 py-1.5 rounded-lg border border-[var(--weui-FG-3)] bg-[var(--weui-BG-2)] text-[var(--weui-FG-0)] text-sm hover:bg-[var(--weui-BG-COLOR-ACTIVE)] transition-colors",
          //   },
          //   [Txt("打开目录")],
          // ),
        ],
      ),
    ]),

    // About
    View({ class: "space-y-4" }, [
      View(
        { class: "text-lg font-medium text-[var(--weui-FG-0)]" },
        [Txt("关于")],
      ),
      View({ class: "space-y-3 pl-4" }, [
        View({ class: "flex items-center gap-3" }, [
          View({
            class:
              "w-1.5 h-1.5 rounded-full bg-[var(--weui-FG-2)] shrink-0",
          }),
          View({ class: "text-sm text-[var(--weui-FG-1)]" }, [
            Txt("支持视频号视频、直播回放下载"),
          ]),
        ]),
        View({ class: "flex items-center gap-3" }, [
          View({
            class:
              "w-1.5 h-1.5 rounded-full bg-[var(--weui-FG-2)] shrink-0",
          }),
          View({ class: "text-sm text-[var(--weui-FG-1)]" }, [
            Txt("支持批量下载和任务管理"),
          ]),
        ]),
        View({ class: "flex items-center gap-3" }, [
          View({
            class:
              "w-1.5 h-1.5 rounded-full bg-[var(--weui-FG-2)] shrink-0",
          }),
          View({ class: "text-sm text-[var(--weui-FG-1)]" }, [
            Txt("支持公众号文章下载"),
          ]),
        ]),
      ]),
    ]),
  ]);
}

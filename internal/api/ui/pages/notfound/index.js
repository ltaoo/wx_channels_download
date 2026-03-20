/** 404 页面 */
export default function NotFoundPageView(props) {
  return View(
    {
      class: cn([
        "flex flex-col items-center justify-center min-h-screen",
        "bg-[var(--background)] text-[var(--foreground)]",
      ]),
    },
    [
      View({ class: "flex flex-col items-center space-y-4 text-center" }, [
        View(
          { class: "text-9xl font-bold opacity-10 select-none" },
          [Txt("404")],
        ),
        View({ class: "text-2xl font-medium" }, [Txt("页面未找到")]),
        View({ class: "opacity-60" }, [Txt("抱歉，您访问的页面不存在。")]),
        View({ class: "mt-8" }, [
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                onClick() {
                  props.history.push("root.home_layout.index");
                },
              }),
              class: cn([
                "px-6 py-3 rounded-lg font-medium transition-opacity",
                "bg-[var(--weui-GREEN)] text-white hover:opacity-90",
              ]),
            },
            [Txt("返回首页")],
          ),
        ]),
      ]),
    ],
  );
}

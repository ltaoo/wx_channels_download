export default function NotFoundPageView(props) {
  return View(
    {
      dataset: { t: "not-found-page-404-页面未找到-抱歉-您访问的页面不存在-button" },
      class: classNames([
        "flex flex-col items-center justify-center min-h-screen",
        "bg-[var(--background)] text-[var(--foreground)]",
      ]),
    },
    [
      View({ dataset: { t: "not-found-page-stack-row-404-页面未找到-抱歉-您访问的页面不存在-button" }, class: "flex flex-col items-center space-y-4 text-center" }, [
        View(
          {
            dataset: { t: "not-found-page-404-heading" },
            class: "text-9xl font-bold opacity-10 select-none",
          },
          ["404"],
        ),
        View({ dataset: { t: "not-found-page-页面未找到-heading" }, class: "text-2xl font-medium" }, ["页面未找到"]),
        View({ dataset: { t: "not-found-page-抱歉-您访问的页面不存在" }, class: "opacity-60" }, ["抱歉，您访问的页面不存在。"]),
        View({ dataset: { t: "not-found-page-button" }, class: "mt-8" }, [
          Button(
            {
              class: classNames([
                "px-6 py-3 rounded-lg font-medium transition-opacity",
                "bg-[var(--foreground)] text-[var(--background)] hover:opacity-90",
              ]),
              store: new Timeless.ui.ButtonCore({
                onClick() {
                  props.history.push("root.home_layout.index");
                },
              }),
            },
            ["返回首页"],
          ),
        ]),
      ]),
    ],
  );
}

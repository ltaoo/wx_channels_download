/** 下载列表页 */
export default function HomeDownloadView(props) {
  const tasks = refarr([]);
  const loading = ref(false);

  async function fetchTasks() {
    loading.as(true);
    try {
      const resp = await fetch("/api/task/list?page=1&page_size=50");
      const result = await resp.json();
      if (result.code === 0 && result.data && result.data.list) {
        tasks.as(result.data.list);
      }
    } catch (e) {
      console.error("获取任务列表失败:", e);
    }
    loading.as(false);
  }

  fetchTasks();

  return View({ class: "p-6 h-full" }, [
    View({ class: "max-w-4xl mx-auto space-y-6" }, [
      // Header
      View({ class: "flex items-center justify-between" }, [
        View({ class: "space-y-2" }, [
          View(
            { class: "text-2xl font-bold text-[var(--weui-FG-0)]" },
            [Txt("下载列表")],
          ),
          View(
            { class: "text-sm text-[var(--weui-FG-1)]" },
            [Txt("管理所有下载任务")],
          ),
        ]),
        View({ class: "flex gap-2" }, [
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                onClick() {
                  fetchTasks();
                },
              }),
              class:
                "px-3 h-8 rounded-lg border border-[var(--weui-FG-3)] bg-[var(--weui-BG-2)] text-[var(--weui-FG-0)] text-sm hover:bg-[var(--weui-BG-COLOR-ACTIVE)] transition-colors",
            },
            [Txt("刷新")],
          ),
        ]),
      ]),
      // Task list
      View({ class: "space-y-2" }, [
        Show(
          {
            when: computed(tasks, (t) => t.length === 0),
          },
          [
            View(
              {
                class:
                  "flex flex-col items-center justify-center py-20 text-[var(--weui-FG-2)]",
              },
              [
                View({ class: "text-4xl mb-4 opacity-30" }, [Txt("📥")]),
                View({ class: "text-sm" }, [Txt("暂无下载任务")]),
              ],
            ),
          ],
        ),
        For({
          each: tasks,
          render(task) {
            return View(
              {
                class:
                  "flex items-center gap-4 p-4 rounded-lg bg-[var(--weui-BG-2)] border border-[var(--weui-FG-5)]",
              },
              [
                // Info
                View({ class: "flex-1 min-w-0 space-y-1" }, [
                  View(
                    {
                      class:
                        "text-sm font-medium text-[var(--weui-FG-0)] truncate",
                    },
                    [
                      Txt(
                        task.meta && task.meta.opts
                          ? task.meta.opts.name
                          : "未知",
                      ),
                    ],
                  ),
                  View(
                    { class: "text-xs text-[var(--weui-FG-1)]" },
                    [Txt(task.status || "")],
                  ),
                ]),
              ],
            );
          },
        }),
      ]),
    ]),
  ]);
}

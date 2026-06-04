import { LogsPageModel } from "./index.model.js";

/* global Alert, AlertDescription, AlertTitle, Badge, Card, CardContent, CardHeader, CardTitle, Select, Skeleton */

const LEVEL_STYLE = {
  debug: {
    badge:
      "border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
    dot: "bg-violet-500",
  },
  info: {
    badge:
      "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-200",
    dot: "bg-sky-500",
  },
  warn: {
    badge:
      "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
    dot: "bg-amber-500",
  },
  error: {
    badge:
      "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200",
    dot: "bg-red-500",
  },
};

function cn(parts) {
  return Timeless.classNames(parts);
}

function formatTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function fileSize(size) {
  const n = Number(size || 0);
  if (n > 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  if (n > 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${n} B`;
}

function levelStyle(level) {
  return LEVEL_STYLE[String(level || "").toLowerCase()] || LEVEL_STYLE.info;
}

function jsonValueText(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function logDetailRows(entry) {
  const rows = [
    ["time", entry.time],
    ["level", entry.level],
    ["source", entry.source],
    ["file", entry.file],
    ["message", entry.message],
  ];
  const data = entry.json || entry.JSON || null;
  if (data && typeof data === "object") {
    const existing = new Set(rows.map((row) => row[0]));
    for (const key of [
      "error",
      "err",
      "stack",
      "trace",
      "request",
      "response",
      "url",
      "task_id",
    ]) {
      if (data[key] !== undefined && !existing.has(key)) {
        rows.push([key, jsonValueText(data[key])]);
        existing.add(key);
      }
    }
    for (const [key, value] of Object.entries(data)) {
      if (!existing.has(key)) {
        rows.push([key, jsonValueText(value)]);
      }
    }
  }
  return rows.filter(
    (row) => row[1] !== undefined && row[1] !== null && String(row[1]) !== "",
  );
}

function LogDetailPanel(entry) {
  return View({ class: "mt-3 space-y-3" }, [
    View(
      {
        class:
          "overflow-hidden rounded-md border border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900",
      },
      [
        For({
          each: logDetailRows(entry),
          render(row) {
            return View(
              {
                class:
                  "grid grid-cols-[96px_minmax(0,1fr)] border-b border-zinc-200 last:border-b-0 dark:border-zinc-800",
              },
              [
                View(
                  {
                    class:
                      "bg-zinc-100 px-3 py-2 font-mono text-[11px] text-zinc-500 dark:bg-zinc-950 dark:text-zinc-400",
                  },
                  [row[0]],
                ),
                View(
                  {
                    class:
                      "whitespace-pre-wrap break-words px-3 py-2 font-mono text-xs leading-5 text-zinc-900 dark:text-zinc-100",
                  },
                  [row[1]],
                ),
              ],
            );
          },
        }),
      ],
    ),
    View(
      {
        as: "pre",
        class:
          "max-h-96 overflow-auto rounded-md bg-zinc-950 p-3 font-mono text-xs leading-5 text-zinc-100",
      },
      [entry.formatted || entry.raw || ""],
    ),
  ]);
}

function PageHeader(vm$) {
  return View(
    {
      class:
        "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex flex-wrap items-center justify-between gap-4" }, [
        View({ class: "min-w-0" }, [
          View(
            {
              class: "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
            },
            ["日志"],
          ),
          View(
            {
              class:
                "mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-zinc-500 dark:text-zinc-400",
            },
            [
              "运行日志、请求代理和下载器事件",
              Show({
                when: vm$.state.lastLoadedAt,
                ok() {
                  return View({ class: "text-xs" }, [
                    computed(
                      vm$.state.lastLoadedAt,
                      (time) => `更新于 ${time}`,
                    ),
                  ]);
                },
              }),
            ],
          ),
        ]),
        View({ class: "flex items-center gap-2" }, [
          View(
            {
              class:
                "flex items-center gap-2 rounded-md px-3 py-2 dark:border-zinc-800",
            },
            [
              Switch({
                store: vm$.ui.autoRefreshSwitch,
              }),
              View({ class: "text-sm text-zinc-700 dark:text-zinc-200" }, [
                "自动刷新",
              ]),
            ],
          ),
          Button(
            {
              store: vm$.ui.refreshBtn,
              prefix: [
                Show({
                  when: computed(vm$.state.loading, (v) => {
                    return v;
                  }),
                  ok() {
                    return Icon({
                      name: "loader",
                      size: 16,
                    });
                  },
                  else() {
                    return Icon({
                      name: "refresh-cw",
                      size: 16,
                    });
                  },
                }),
              ],
            },
            ["刷新"],
          ),
        ]),
      ]),
    ],
  );
}

function Toolbar(vm$) {
  return Card({ class: "border-zinc-200 shadow-none dark:border-zinc-800" }, [
    CardContent({ class: "p-3" }, [
      View(
        {
          class: "flex items-center gap-2 overflow-x-auto whitespace-nowrap",
        },
        [
          View({ class: "w-48 shrink-0" }, [
            Select({ store: vm$.ui.sourceSelect }),
          ]),
          View({ class: "w-36 shrink-0" }, [
            Select({ store: vm$.ui.levelSelect }),
          ]),
          Input({ store: vm$.ui.keywordInput, class: "min-w-[260px] flex-1" }),
          Button(
            {
              store: vm$.ui.resetBtn,
              class: "shrink-0",
              prefix: [Icon({ name: "rotate-ccw", size: 16 })],
            },
            ["重置"],
          ),
        ],
      ),
      View({ class: "flex items-center gap-2 mt-2" }, [
        View({ class: "w-32 shrink-0" }, [
          Select({ store: vm$.ui.limitSelect }),
        ]),
        View(
          {
            class:
              "flex h-8 shrink-0 items-center gap-2 rounded-md border border-zinc-200 px-3 dark:border-zinc-800",
          },
          [
            Checkbox({
              store: vm$.ui.formatJsonCheckbox,
            }),
            View(
              {
                class:
                  "whitespace-nowrap text-sm text-zinc-700 dark:text-zinc-200",
              },
              ["格式化 JSON"],
            ),
          ],
        ),
        View({ class: "shrink-0 text-xs text-zinc-500 dark:text-zinc-400" }, [
          combine(
            {
              total: vm$.state.total,
              entries: vm$.state.entries,
              files: vm$.state.files,
            },
            (d) => {
              return `${d.entries.length}/${d.total} 条 · ${d.files.length} 个文件`;
            },
          ),
        ]),
      ]),
    ]),
  ]);
}

function FileStrip(vm$) {
  return Show({
    when: computed(vm$.state.files, (files) => files.length > 0),
    ok() {
      return View({ class: "flex flex-wrap gap-2" }, [
        For({
          each: vm$.state.files,
          render(file) {
            return Badge(
              {
                variant: "secondary",
                class:
                  "max-w-full gap-1 truncate border border-zinc-200 bg-white font-normal dark:border-zinc-800 dark:bg-zinc-950",
                // title: file.path,
              },
              [
                Icon({ name: "file-text", size: 13 }),
                `${file.name || "log"} · ${fileSize(file.size)}`,
              ],
            );
          },
        }),
      ]);
    },
  });
}

function LogEntryRow(entry, vm$) {
  const expanded = computed(vm$.state.expanded, (map) => !!map[entry.index]);
  const style = levelStyle(entry.level);
  return View(
    {
      class:
        "grid grid-cols-[auto_minmax(0,1fr)_auto] gap-3 border-b border-zinc-100 px-4 py-3 last:border-b-0 dark:border-zinc-900",
    },
    [
      Badge(
        {
          class: cn([
            "mt-0.5 h-6 shrink-0 border px-2 font-mono text-[11px] font-semibold uppercase",
            style.badge,
          ]),
        },
        [entry.level || "info"],
      ),
      View({ class: "min-w-0" }, [
        View({ class: "flex flex-wrap items-center gap-x-3 gap-y-1" }, [
          View(
            { class: "font-mono text-[11px] text-zinc-500 dark:text-zinc-400" },
            [formatTime(entry.time)],
          ),
          Badge(
            {
              variant: "outline",
              class: "h-5 max-w-[240px] truncate px-1.5 text-[11px]",
            },
            [entry.source || entry.file || "app"],
          ),
          View(
            {
              class: "truncate text-[11px] text-zinc-400 dark:text-zinc-500",
              // title: entry.file,
            },
            [entry.file || ""],
          ),
        ]),
        View(
          {
            class:
              "mt-1 whitespace-pre-wrap break-words font-mono text-xs leading-5 text-zinc-900 dark:text-zinc-100",
          },
          [entry.message || entry.raw || ""],
        ),
        Show({
          when: expanded,
          ok() {
            return LogDetailPanel(entry);
          },
        }),
      ]),
      Button(
        {
          title: "查看日志详情",
          store: new Timeless.ui.ButtonCore({
            size: "sm",
            variant: "ghost",
            onClick() {
              vm$.methods.toggleExpanded(entry.index);
            },
          }),
        },
        [
          computed(expanded, (v) => (v ? "收起" : "详情")),
          Icon({
            name: computed(expanded, (v) =>
              v ? "chevron-up" : "chevron-down",
            ),
            size: 16,
          }),
        ],
      ),
    ],
  );
}

function LoadingState() {
  return View({ class: "space-y-0" }, [
    For({
      each: [0, 1, 2, 3, 4],
      render() {
        return View(
          {
            class:
              "border-b border-zinc-100 px-4 py-4 last:border-b-0 dark:border-zinc-900",
          },
          [
            View({ class: "flex items-center gap-3" }, [
              Skeleton({ class: "h-6 w-14" }),
              View({ class: "min-w-0 flex-1 space-y-2" }, [
                Skeleton({ class: "h-3 w-48" }),
                Skeleton({ class: "h-4 w-full" }),
              ]),
            ]),
          ],
        );
      },
    }),
  ]);
}

function EmptyState() {
  return View(
    {
      class:
        "flex h-72 flex-col items-center justify-center gap-2 text-sm text-zinc-500 dark:text-zinc-400",
    },
    [Icon({ name: "file-search", size: 24 }), "没有匹配的日志"],
  );
}

function LogList(vm$) {
  return Card(
    {
      class:
        "flex min-h-0 flex-1 flex-col overflow-hidden border-zinc-200 shadow-none dark:border-zinc-800",
    },
    [
      CardHeader(
        { class: "border-b border-zinc-100 px-4 py-3 dark:border-zinc-900" },
        [
          View({ class: "flex items-center justify-between gap-3" }, [
            CardTitle({ class: "text-sm" }, ["日志流"]),
            Show({
              when: vm$.state.loading,
              ok() {
                return Badge({ variant: "secondary", class: "gap-1" }, [
                  Icon({ name: "loader", size: 13 }),
                  "加载中",
                ]);
              },
            }),
          ]),
        ],
      ),
      CardContent({ class: "min-h-0 flex-1 overflow-hidden p-0" }, [
        Show({
          when: computed(
            vm$.state.loading,
            (loading) => loading && vm$.state.entries.value.length === 0,
          ),
          ok() {
            return LoadingState();
          },
          else() {
            return Show({
              when: computed(
                vm$.state.entries,
                (entries) => entries.length > 0,
              ),
              ok() {
                return View({ class: "h-full min-h-0 overflow-auto" }, [
                  For({
                    each: vm$.state.entries,
                    render(entry) {
                      return LogEntryRow(entry, vm$);
                    },
                  }),
                ]);
              },
              else() {
                return EmptyState();
              },
            });
          },
        }),
      ]),
    ],
  );
}

/**
 * @param {ViewComponentProps} props
 */
export default function LogsPageView(props) {
  const vm$ = LogsPageModel(props);

  return View(
    {
      class: "flex h-full min-h-0 flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.load();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      PageHeader(vm$),
      View({ class: "flex min-h-0 flex-1 flex-col gap-3 p-4" }, [
        Toolbar(vm$),
        FileStrip(vm$),
        Show({
          when: computed(vm$.state.error, (t) => !!t),
          ok() {
            return Alert({ variant: "destructive" }, [
              AlertTitle({}, ["加载失败"]),
              AlertDescription({}, [vm$.state.error]),
            ]);
          },
        }),
        LogList(vm$),
      ]),
    ],
  );
}

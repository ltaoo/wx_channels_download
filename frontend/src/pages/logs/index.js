import { fetchLogs } from "@/biz/request.js";

const LEVELS = [
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const LEVEL_STYLE = {
  debug: "bg-violet-50 text-violet-700 ring-violet-200 dark:bg-violet-950/40 dark:text-violet-200 dark:ring-violet-900",
  info: "bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-950/40 dark:text-sky-200 dark:ring-sky-900",
  warn: "bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-200 dark:ring-amber-900",
  error: "bg-red-50 text-red-700 ring-red-200 dark:bg-red-950/40 dark:text-red-200 dark:ring-red-900",
};

function uniqueSources(files, entries) {
  const set = new Set(["all"]);
  for (const file of files || []) set.add(file.name || file.path || "log");
  for (const entry of entries || []) set.add(entry.source || entry.file || "log");
  return Array.from(set);
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

function levelClass(level) {
  return LEVEL_STYLE[String(level || "").toLowerCase()] || LEVEL_STYLE.info;
}

function ToggleButton(label, active, onClick) {
  const activeClass =
    typeof active === "boolean"
      ? active
        ? "border-zinc-900 bg-zinc-900 text-white dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950"
        : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-200 dark:hover:bg-zinc-900"
      : computed(active, (v) =>
          v
            ? "border-zinc-900 bg-zinc-900 text-white dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950"
            : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-200 dark:hover:bg-zinc-900",
        );
  return View(
    {
      as: "button",
      type: "button",
      class: Timeless.classNames([
        "h-9 whitespace-nowrap rounded-md border px-3 text-sm font-medium transition",
        activeClass,
      ]),
      onClick,
    },
    [label],
  );
}

function IconButton(icon, title, onClick, variant = "outline") {
  return Button(
    {
      title,
      store: new Timeless.ui.ButtonCore({
        size: "sm",
        variant,
        onClick,
      }),
    },
    [Icon({ name: icon, size: 16 })],
  );
}

function NativeSelect(props, children) {
  return View(
    {
      as: "select",
      class:
        "h-9 rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900 outline-none transition focus:border-zinc-400 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-100",
      value: props.value,
      onChange(e) {
        props.onChange?.(e.target.value);
      },
    },
    children,
  );
}

function Stat(label, value, icon) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex items-center justify-between gap-3" }, [
        View({ class: "text-xs font-medium text-zinc-500 dark:text-zinc-400" }, [
          label,
        ]),
        Icon({ name: icon, size: 17 }),
      ]),
      View(
        {
          class:
            "mt-2 truncate text-xl font-semibold text-zinc-950 dark:text-zinc-50",
        },
        [value],
      ),
    ],
  );
}

function LogEntryRow(entry, expanded_, toggle) {
  const expanded = computed(expanded_, (m) => !!m[entry.index]);
  return View(
    {
      class:
        "border-b border-zinc-100 px-4 py-3 last:border-b-0 dark:border-zinc-900",
    },
    [
      View({ class: "flex items-start gap-3" }, [
        View(
          {
            class: Timeless.classNames([
              "mt-0.5 shrink-0 rounded px-2 py-0.5 text-[11px] font-semibold uppercase ring-1",
              levelClass(entry.level),
            ]),
          },
          [entry.level || "info"],
        ),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex flex-wrap items-center gap-x-3 gap-y-1" }, [
            View(
              {
                class:
                  "font-mono text-[11px] text-zinc-500 dark:text-zinc-400",
              },
              [formatTime(entry.time)],
            ),
            View(
              {
                class:
                  "rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-600 dark:bg-zinc-900 dark:text-zinc-300",
              },
              [entry.source || entry.file || "app"],
            ),
            View(
              {
                class:
                  "truncate text-[11px] text-zinc-400 dark:text-zinc-500",
                title: entry.file,
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
              return View(
                {
                  as: "pre",
                  class:
                    "mt-3 max-h-80 overflow-auto rounded-md bg-zinc-950 p-3 font-mono text-xs leading-5 text-zinc-100",
                },
                [entry.formatted || entry.raw || ""],
              );
            },
          }),
        ]),
        IconButton(
          computed(expanded, (v) => (v ? "chevron-up" : "chevron-down")),
          "展开日志详情",
          () => toggle(entry.index),
          "ghost",
        ),
      ]),
    ],
  );
}

export default function LogsPageView(props) {
  const entries_ = refarr([]);
  const files_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const total_ = ref(0);
  const keyword_ = ref("");
  const source_ = ref("all");
  const selectedLevels_ = ref({
    debug: true,
    info: true,
    warn: true,
    error: true,
  });
  const limit_ = ref("300");
  const formatJson_ = ref(true);
  const autoRefresh_ = ref(false);
  const expanded_ = ref({});
  let timer = null;

  const keywordInput$ = new Timeless.ui.InputCore({
    placeholder: "按关键字搜索消息、字段、原始日志",
    onChange(value) {
      keyword_.as(value);
    },
  });

  async function load() {
    loading_.as(true);
    error_.as("");
    const selected = Object.keys(selectedLevels_.value).filter(
      (k) => selectedLevels_.value[k],
    );
    const r = await fetchLogs({
      levels: selected.length ? selected.join(",") : "__none",
      keyword: keyword_.value.trim(),
      source: source_.value,
      limit: Number(limit_.value) || 300,
      format_json: formatJson_.value,
    });
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    entries_.as(r.data.entries || []);
    files_.as(r.data.files || []);
    total_.as(r.data.total || 0);
  }

  function toggleLevel(level) {
    selectedLevels_.as({
      ...selectedLevels_.value,
      [level]: !selectedLevels_.value[level],
    });
    setTimeout(load, 0);
  }

  function toggleExpanded(index) {
    expanded_.as({
      ...expanded_.value,
      [index]: !expanded_.value[index],
    });
  }

  function resetFilters() {
    keyword_.as("");
    keywordInput$.setValue?.("");
    source_.as("all");
    selectedLevels_.as({ debug: true, info: true, warn: true, error: true });
    limit_.as("300");
    formatJson_.as(true);
    expanded_.as({});
    load();
  }

  function restartTimer() {
    if (timer) clearInterval(timer);
    timer = null;
    if (autoRefresh_.value) {
      timer = setInterval(load, 5000);
    }
  }

  return View(
    {
      class: "flex h-full min-h-0 flex-col bg-zinc-50 dark:bg-zinc-950",
      onMounted() {
        load();
      },
      onUnmounted() {
        if (timer) clearInterval(timer);
      },
    },
    [
      View(
        {
          class:
            "border-b border-zinc-200 bg-white px-6 py-4 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({}, [
              View(
                {
                  class:
                    "text-lg font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["日志"],
              ),
              View(
                {
                  class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400",
                },
                ["查看当前项目产生的 API、Proxy、下载器等运行日志"],
              ),
            ]),
            View({ class: "flex items-center gap-2" }, [
              IconButton("rotate-cw", "刷新", load),
              ToggleButton(
                computed(autoRefresh_, (v) => (v ? "自动刷新中" : "自动刷新")),
                autoRefresh_,
                () => {
                  autoRefresh_.as(!autoRefresh_.value);
                  restartTimer();
                },
              ),
            ]),
          ]),
          View(
            {
              class:
                "mt-4 grid grid-cols-1 gap-3 xl:grid-cols-[minmax(220px,1fr)_auto_auto_auto]",
            },
            [
              Input({ store: keywordInput$ }),
              View({ class: "flex flex-wrap items-center gap-2" }, [
                For({
                  each: LEVELS,
                  render(level) {
                    return ToggleButton(
                      level.label,
                      computed(selectedLevels_, (m) => !!m[level.value]),
                      () => toggleLevel(level.value),
                    );
                  },
                }),
              ]),
              NativeSelect(
                {
                  value: computed(source_, (v) => v),
                  onChange(value) {
                    source_.as(value);
                    setTimeout(load, 0);
                  },
                },
                [
                  For({
                    each: computed({ files: files_, entries: entries_ }, (d) =>
                      uniqueSources(d.files, d.entries),
                    ),
                    render(source) {
                      return View(
                        { as: "option", value: source },
                        [source === "all" ? "全部来源" : source],
                      );
                    },
                  }),
                ],
              ),
              View({ class: "flex items-center gap-2" }, [
                NativeSelect(
                  {
                    value: computed(limit_, (v) => v),
                    onChange(value) {
                      limit_.as(value);
                      setTimeout(load, 0);
                    },
                  },
                  [
                    View({ as: "option", value: "100" }, ["100 条"]),
                    View({ as: "option", value: "300" }, ["300 条"]),
                    View({ as: "option", value: "800" }, ["800 条"]),
                    View({ as: "option", value: "2000" }, ["2000 条"]),
                  ],
                ),
                ToggleButton("格式化 JSON", formatJson_, () => {
                  formatJson_.as(!formatJson_.value);
                  setTimeout(load, 0);
                }),
                IconButton("filter-x", "重置筛选", resetFilters, "ghost"),
              ]),
            ],
          ),
        ],
      ),
      View({ class: "grid grid-cols-1 gap-4 p-6 md:grid-cols-3" }, [
        Stat("匹配日志", computed(total_, (v) => String(v)), "list-filter"),
        Stat("当前显示", computed(entries_, (v) => String(v.length)), "rows-3"),
        Stat(
          "日志文件",
          computed(files_, (v) => String(v.length)),
          "file-text",
        ),
      ]),
      Show({
        when: computed(files_, (files) => files.length > 0),
        ok() {
          return View(
            {
              class:
                "mx-6 mb-4 flex flex-wrap gap-2 text-xs text-zinc-500 dark:text-zinc-400",
            },
            [
              For({
                each: files_,
                render(file) {
                  return View(
                    {
                      class:
                        "max-w-full truncate rounded-md border border-zinc-200 bg-white px-2.5 py-1 dark:border-zinc-800 dark:bg-zinc-950",
                      title: file.path,
                    },
                    [`${file.name} · ${fileSize(file.size)}`],
                  );
                },
              }),
            ],
          );
        },
      }),
      Show({
        when: computed(error_, (v) => !!v),
        ok() {
          return View(
            {
              class:
                "mx-6 mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300",
            },
            [error_],
          );
        },
      }),
      View(
        {
          class:
            "mx-6 mb-6 min-h-0 flex-1 overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          Show({
            when: computed(loading_, (v) => v && entries_.value.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-64 items-center justify-center gap-2 text-sm text-zinc-500 dark:text-zinc-400",
                },
                [Icon({ name: "loader", size: 16 }), "正在加载日志"],
              );
            },
            else() {
              return Show({
                when: computed(entries_, (v) => v.length > 0),
                ok() {
                  return View({ class: "h-full overflow-auto" }, [
                    For({
                      each: entries_,
                      render(entry) {
                        return LogEntryRow(entry, expanded_, toggleExpanded);
                      },
                    }),
                  ]);
                },
                else() {
                  return View(
                    {
                      class:
                        "flex h-64 flex-col items-center justify-center gap-2 text-sm text-zinc-500 dark:text-zinc-400",
                    },
                    [
                      Icon({ name: "file-search", size: 22 }),
                      "没有匹配的日志",
                    ],
                  );
                },
              });
            },
          }),
        ],
      ),
    ],
  );
}

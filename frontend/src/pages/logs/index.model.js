import { fetchLogs } from "@/biz/request.js";

export const LOG_LEVELS = [
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

export const LOG_LIMITS = [
  { value: "100", label: "100 条" },
  { value: "300", label: "300 条" },
  { value: "800", label: "800 条" },
  { value: "2000", label: "2000 条" },
];

export const LOG_LEVEL_OPTIONS = [
  { value: "all", label: "全部级别" },
  ...LOG_LEVELS,
];

function option(label, value) {
  return new Timeless.ui.SelectItemCore({ label, value });
}

function uniqueSources(files, entries) {
  const set = new Set(["all"]);
  for (const file of files || []) set.add(file.name || file.path || "log");
  for (const entry of entries || [])
    set.add(entry.source || entry.file || "log");
  return Array.from(set);
}

/**
 * @param {ViewComponentProps} props
 */
export function LogsPageModel(props) {
  const reqs = {
    logs: new Timeless.RequestCore(fetchLogs, {
      client: props.client,
    }),
  };

  const state = {
    entries: refarr([]),
    files: refarr([]),
    loading: ref(false),
    error: ref(""),
    total: ref(0),
    keyword: ref(""),
    source: ref("all"),
    level: ref("all"),
    limit: ref("300"),
    formatJson: ref(true),
    autoRefresh: ref(false),
    expanded: ref({}),
    lastLoadedAt: ref(""),
  };

  let timer = null;

  const ui = {
    keywordInput: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "搜索消息、字段、原始日志",
      onChange(value) {
        state.keyword.as(value);
      },
    }),
    refreshBtn: null,
    resetBtn: null,
    autoRefreshSwitch: Timeless.ui.SwitchCore({ defaultValue: false }),
    formatJsonCheckbox: new Timeless.ui.CheckboxCore({}),
    sourceSelect: new Timeless.ui.SelectCore({
      defaultValue: "all",
      placeholder: "全部来源",
      options: [option("全部来源", "all")],
      onChange(value) {
        state.source.as(value || "all");
        methods.load();
      },
    }),
    levelSelect: new Timeless.ui.SelectCore({
      defaultValue: "all",
      placeholder: "全部级别",
      options: LOG_LEVEL_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        state.level.as(value || "all");
        methods.load();
      },
    }),
    limitSelect: new Timeless.ui.SelectCore({
      defaultValue: "300",
      placeholder: "显示条数",
      options: LOG_LIMITS.map((item) => option(item.label, item.value)),
      onChange(value) {
        state.limit.as(value || "300");
        methods.load();
      },
    }),
  };

  ui.refreshBtn = new Timeless.ui.ButtonCore({
    variant: "outline",
    // disabled: state.loading,
    onClick() {
      methods.load();
    },
  });

  ui.resetBtn = new Timeless.ui.ButtonCore({
    variant: "ghost",
    // disabled: state.loading,
    onClick() {
      methods.resetFilters();
    },
  });

  const methods = {
    async load() {
      state.loading.as(true);
      state.error.as("");
      const result = await reqs.logs.run({
        levels: state.level.value === "all" ? "" : state.level.value,
        keyword: state.keyword.value.trim(),
        source: state.source.value,
        limit: Number(state.limit.value) || 300,
        format_json: state.formatJson.value,
      });
      state.loading.as(false);
      if (result.error) {
        state.error.as(
          result.error.message || result.error.msg || String(result.error),
        );
        return;
      }
      const data = result.data || {};
      state.entries.as(data.entries || []);
      state.files.as(data.files || []);
      state.total.as(data.total || 0);
      state.lastLoadedAt.as(new Date().toLocaleTimeString());
      methods.syncSourceOptions();
    },

    syncSourceOptions() {
      const sources = uniqueSources(state.files.value, state.entries.value);
      ui.sourceSelect.setOptions(
        sources.map((source) =>
          option(source === "all" ? "全部来源" : source, source),
        ),
      );
      if (!sources.includes(state.source.value)) {
        state.source.as("all");
        ui.sourceSelect.setValue("all");
      }
    },

    setFormatJson(value) {
      state.formatJson.as(!!value);
      ui.formatJsonCheckbox.setValue(!!value);
      methods.load();
    },

    setAutoRefresh(value) {
      state.autoRefresh.as(!!value);
      ui.autoRefreshSwitch.setValue(!!value);
      methods.restartTimer();
    },

    toggleExpanded(index) {
      state.expanded.as({
        ...state.expanded.value,
        [index]: !state.expanded.value[index],
      });
    },

    resetFilters() {
      state.keyword.as("");
      ui.keywordInput.setValue?.("");
      state.source.as("all");
      ui.sourceSelect.setValue("all");
      state.level.as("all");
      ui.levelSelect.setValue("all");
      state.limit.as("300");
      ui.limitSelect.setValue("300");
      state.formatJson.as(true);
      ui.formatJsonCheckbox.setValue(true);
      state.expanded.as({});
      methods.load();
    },

    restartTimer() {
      if (timer) clearInterval(timer);
      timer = null;
      if (state.autoRefresh.value) {
        timer = setInterval(methods.load, 5000);
      }
    },

    destroy() {
      if (timer) clearInterval(timer);
      timer = null;
    },
  };

  ui.autoRefreshSwitch.onChange((value) => {
    state.autoRefresh.as(!!value);
    methods.restartTimer();
  });
  ui.formatJsonCheckbox.onChange((value) => {
    state.formatJson.as(!!value);
    methods.load();
  });

  return { state, ui, methods };
}

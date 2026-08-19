const LOG_LEVEL_OPTIONS = [
  { value: "all", label: "全部级别" },
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const logs_request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    if (payload.code !== 0) {
      return Timeless.Result.Err(
        payload.msg || "获取日志失败",
        payload.code,
        payload.data,
      );
    }
    return Timeless.Result.Ok(payload.data || {});
  },
});

function number_or_default(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function first_non_empty() {
  for (let i = 0; i < arguments.length; i += 1) {
    const value = arguments[i];
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
  }
  return "";
}

function option(label, value) {
  return new Timeless.vm.SelectItemCore({ label, value });
}

function format_datetime(value) {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const pad = (number) => String(number).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function structured_field_value(data, field) {
  if (!data || typeof data !== "object") {
    return "";
  }
  const wanted = String(field || "").toLowerCase();
  for (const [key, value] of Object.entries(data)) {
    if (String(key).toLowerCase() === wanted) {
      const text = first_non_empty(value);
      if (text === "") {
        return "";
      }
      if (typeof text === "string") {
        return text;
      }
      try {
        return JSON.stringify(text);
      } catch {
        return String(text);
      }
    }
  }
  return "";
}

function field_text(value) {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function log_field_rows(entry) {
  const data = (entry && (entry.json || entry.JSON)) || null;
  const skip = new Set([
    "time",
    "timestamp",
    "level",
    "message",
    "msg",
    "file",
    "component",
  ]);
  const rows = [];
  if (data && typeof data === "object") {
    for (const [key, value] of Object.entries(data)) {
      if (!skip.has(String(key).toLowerCase())) {
        const text = field_text(value);
        if (text !== "") {
          rows.push([key, text]);
        }
      }
    }
  }
  return rows;
}

function json_object_field(value) {
  const text = field_text(value);
  const normalized = text.trim();
  if (!normalized.startsWith("{") || !normalized.endsWith("}")) {
    return null;
  }
  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return null;
    }
    return {
      text,
      formatted_text: JSON.stringify(parsed, null, 2),
    };
  } catch {
    return null;
  }
}

function json_object_field_text(value) {
  const json_field = json_object_field(value);
  return json_field ? json_field.text : "";
}

function redact_export_text(value) {
  const text = field_text(value);
  return text.replace(/\/Users\/[^\r\n"'<>]*/g, (path) => {
    const size = Math.min(10, path.length);
    return `${"*".repeat(size)}${path.slice(size)}`;
  });
}

function csv_escape(value) {
  const text = redact_export_text(value);
  if (/[",\r\n]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
}

function export_filename() {
  const stamp = format_datetime(new Date()).replace(/[^\d]/g, "");
  return `wx-logs-${stamp || Date.now()}.csv`;
}

function download_text(filename, text) {
  const blob = new Blob([text], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

function export_entries(entries) {
  const rows = [["level", "time", "file", "component", "msg", "fields"]];
  for (const entry of entries || []) {
    const item = entry || {};
    const fields = log_field_rows(item)
      .map((row) => `${row[0]}=${row[1]}`)
      .join("; ");
    rows.push([
      item.level || "info",
      format_datetime(item.time),
      structured_field_value(item.json || item.JSON, "file") ||
        item.file ||
        "",
      structured_field_value(item.json || item.JSON, "component") ||
        item.component ||
        "",
      item.message || item.raw || "",
      fields,
    ]);
  }
  const csv = rows.map((row) => row.map(csv_escape).join(",")).join("\n");
  download_text(export_filename(), `\ufeff${csv}\n`);
}

function normalize_log_entry(raw, fallbackIndex) {
  const source = raw && typeof raw === "object" ? raw : {};
  const json = first_non_empty(source.json, source.JSON, null);
  const file = first_non_empty(
    source.file,
    source.File,
    structured_field_value(json, "file"),
  );
  const component = first_non_empty(
    source.component,
    source.Component,
    structured_field_value(json, "component"),
  );
  return {
    ...source,
    index: number_or_default(
      first_non_empty(source.index, source.Index),
      fallbackIndex,
    ),
    file,
    component,
    source: first_non_empty(source.source, source.Source, component, ""),
    time: first_non_empty(
      source.time,
      source.Time,
      source.timestamp,
      source.Timestamp,
    ),
    level: String(
      first_non_empty(source.level, source.Level, "info"),
    ).toLowerCase(),
    message: first_non_empty(
      source.message,
      source.Message,
      source.msg,
      source.Msg,
      source.raw,
      source.Raw,
    ),
    raw: first_non_empty(
      source.raw,
      source.Raw,
      source.message,
      source.Message,
    ),
    json,
    formatted: first_non_empty(source.formatted, source.Formatted),
  };
}

function normalize_logs_response(data, fallbackPage, fallbackSize) {
  const source = data && typeof data === "object" ? data : {};
  const rawEntries = Array.isArray(source.entries)
    ? source.entries
    : Array.isArray(source.Entries)
      ? source.Entries
      : Array.isArray(source.list)
        ? source.list
        : [];
  const entries = rawEntries.map((item, index) =>
    normalize_log_entry(item, index + 1),
  );
  const raw_files = Array.isArray(source.files)
    ? source.files
    : Array.isArray(source.Files)
      ? source.Files
      : [];
  const files = raw_files.map((item) => {
    const file = item && typeof item === "object" ? item : {};
    return {
      ...file,
      name: first_non_empty(file.name, file.Name),
      path: first_non_empty(file.path, file.Path),
      size: number_or_default(first_non_empty(file.size, file.Size), 0),
    };
  });
  return {
    entries,
    files,
    total: Math.max(
      0,
      number_or_default(
        first_non_empty(source.total, source.Total),
        entries.length,
      ),
    ),
    page: Math.max(
      1,
      number_or_default(
        first_non_empty(source.page, source.Page),
        fallbackPage,
      ),
    ),
    page_size: Math.max(
      1,
      number_or_default(
        first_non_empty(
          source.page_size,
          source.pageSize,
          source.PageSize,
          source.limit,
          source.Limit,
        ),
        fallbackSize,
      ),
    ),
  };
}

function unique_components(entries) {
  const set = new Set(["all"]);
  for (const entry of entries || []) {
    if (entry.component) {
      set.add(entry.component);
    }
  }
  return Array.from(set).filter(Boolean);
}

function LogsPageViewModel(props) {
  const PAGE_SIZE_DEFAULT = 300;
  const entries_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const loading_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");
  const source_ = ref("all");
  const level_ = ref("all");
  const format_json_ = ref(true);
  const auto_refresh_ = ref(false);
  const last_loaded_at_ = ref("");
  const log_file_path_ = ref("");
  const json_preview_title_ = ref("JSON 预览");
  const json_preview_text_ = ref("");
  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索消息、字段、原始日志",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
    }),
    btn_search$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "primary",
    }),
    btn_reset$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
      onClick() {
        return reset_filters();
      },
    }),
    btn_export$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
    }),
    btn_refresh$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
    }),
    btn_copy_log_file_path$: new Timeless.vm.ButtonCore({
      disabled: true,
      variant: "outline",
    }),
    checkbox_auto_refresh$: new Timeless.vm.CheckboxCore({
      checked: auto_refresh_.value,
      onChange(value) {
        set_auto_refresh(value);
      },
    }),
    json_preview_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    select_source$: new Timeless.vm.SelectCore({
      defaultValue: "all",
      placeholder: "全部组件",
      options: [option("全部组件", "all")],
      onChange(value) {
        source_.as(value || "all");
        load(1);
      },
    }),
    select_level$: new Timeless.vm.SelectCore({
      defaultValue: "all",
      placeholder: "全部级别",
      options: LOG_LEVEL_OPTIONS.map((item) =>
        option(item.label, item.value),
      ),
      onChange(value) {
        level_.as(value || "all");
        load(1);
      },
    }),
  };
  keyword_.subscribe({
    onChange(value) {
      if (ui.input_keyword$.value !== value) {
        ui.input_keyword$.setValue(value, { silence: true });
      }
    },
  });
  auto_refresh_.subscribe({
    onChange(value) {
      const checked = Boolean(value);
      if (ui.checkbox_auto_refresh$.checked !== checked) {
        ui.checkbox_auto_refresh$.setValue(checked, { silence: true });
      }
    },
  });
  log_file_path_.subscribe({
    onChange(value) {
      if (value) {
        ui.btn_copy_log_file_path$.enable();
      } else {
        ui.btn_copy_log_file_path$.disable();
      }
    },
  });
  loading_.subscribe({
    onChange(loading) {
      [
        ui.btn_search$,
        ui.btn_reset$,
        ui.btn_export$,
        ui.btn_refresh$,
      ].forEach((button) => {
        if (loading) {
          button.disable();
        } else {
          button.enable();
        }
      });
    },
  });
  let timer = null;
  let request_sequence = 0;

  const logs_request_core = new Timeless.kit.RequestCore(
    (params) => logs_request.get("/api/logs", params),
    {
      client: props.client,
      process(response) {
        if (response.error) {
          return Timeless.Result.Err(response.error);
        }
        return Timeless.Result.Ok(
          normalize_logs_response(
            response.data,
            page_.value,
            page_size_.value,
          ),
        );
      },
    },
  );

  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const range_text_ = combine(
    {
      entries: entries_,
      total: total_,
      page: page_,
      pageSize: page_size_,
    },
    (state) => {
      if (!state.total || !state.entries.length) {
        return `共 ${state.total || 0} 条`;
      }
      const start = (state.page - 1) * state.pageSize + 1;
      return `第 ${start}-${start + state.entries.length - 1} 条，共 ${state.total} 条`;
    },
  );

  function sync_source_options() {
    const sources = unique_components(entries_.value);
    ui.select_source$.setOptions(
      sources.map((source) =>
        option(source === "all" ? "全部组件" : source, source),
      ),
    );
    if (!sources.includes(source_.value)) {
      source_.as("all");
      ui.select_source$.setValue("all");
    }
  }

  function restart_timer() {
    if (timer) {
      clearInterval(timer);
    }
    timer = null;
    if (auto_refresh_.value) {
      timer = setInterval(load, 5000);
    }
  }

  async function load(targetPage = page_.value) {
    const sequence = ++request_sequence;
    const requestedPage = Math.max(1, Number(targetPage) || 1);
    loading_.as(true);
    error_.as("");
    let result;
    try {
      result = await logs_request_core.run({
        levels: level_.value === "all" ? "" : level_.value,
        keyword: String(keyword_.value || "").trim(),
        source: source_.value,
        page: requestedPage,
        page_size: page_size_.value,
        format_json: format_json_.value,
      });
    } catch (error) {
      if (sequence === request_sequence) {
        error_.as(error && error.message ? error.message : String(error));
      }
      return Timeless.Result.Err(error);
    } finally {
      if (sequence === request_sequence) {
        loading_.as(false);
      }
    }
    if (sequence !== request_sequence) {
      return result;
    }
    if (result.error) {
      error_.as(
        result.error.message || result.error.msg || String(result.error),
      );
      return result;
    }
    const data = result.data || {};
    entries_.as(data.entries || [], { reset: true });
    const log_file = (data.files || []).find((file) => file && file.path);
    log_file_path_.as(log_file ? String(log_file.path) : "");
    total_.as(data.total || 0);
    page_.as(data.page || requestedPage);
    page_size_.as(data.page_size || page_size_.value);
    last_loaded_at_.as(format_datetime(new Date()));
    sync_source_options();
    return result;
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function set_auto_refresh(value) {
    auto_refresh_.as(Boolean(value));
    restart_timer();
  }

  function reset_filters() {
    keyword_.as("");
    source_.as("all");
    ui.select_source$.setValue("all");
    level_.as("all");
    ui.select_level$.setValue("all");
    format_json_.as(true);
    return load(1);
  }

  const methods = {
    ready() {
      return load(1);
    },
    refresh() {
      return load(page_.value);
    },
    copyLogFilePath() {
      const log_file_path = String(log_file_path_.value || "").trim();
      if (!log_file_path) {
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("日志文件路径不可用");
        }
        return false;
      }
      try {
        props.app.copy(log_file_path);
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("日志文件路径已复制");
        }
        return true;
      } catch {
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("复制失败");
        }
        return false;
      }
    },
    search() {
      return load(1);
    },
    setKeyword: set_keyword,
    setFormatJson(value) {
      format_json_.as(Boolean(value));
      return load(page_.value);
    },
    setAutoRefresh: set_auto_refresh,
    resetFilters: reset_filters,
    previousPage() {
      if (page_.value <= 1 || loading_.value) {
        return null;
      }
      return load(page_.value - 1);
    },
    nextPage() {
      if (page_.value >= page_count_.value || loading_.value) {
        return null;
      }
      return load(page_.value + 1);
    },
    exportLogs() {
      const list = Array.isArray(entries_.value) ? entries_.value : [];
      if (list.length === 0) {
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("没有可导出的日志");
        }
        return;
      }
      export_entries(list);
      if (window.DLUtils && DLUtils.toast) {
        DLUtils.toast("日志已导出");
      }
    },
    fieldRows(entry) {
      return log_field_rows(entry);
    },
    isJsonFieldValue(value) {
      return json_object_field_text(value) !== "";
    },
    showJsonFieldValue(field, value) {
      const json_field = json_object_field(value);
      if (!json_field) {
        return false;
      }
      const field_name = String(field || "").trim();
      json_preview_title_.as(
        field_name ? `${field_name} · JSON` : "JSON 预览",
      );
      json_preview_text_.as(json_field.formatted_text);
      ui.json_preview_dialog$.show();
      return true;
    },
    async copyJsonFieldValue(value) {
      const text = json_object_field_text(value);
      if (!text) {
        return false;
      }
      try {
        props.app.copy(text);
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("JSON 已复制");
        }
        return true;
      } catch {
        if (window.DLUtils && DLUtils.toast) {
          DLUtils.toast("复制失败");
        }
        return false;
      }
    },
    destroy() {
      if (timer) {
        clearInterval(timer);
      }
      timer = null;
      ui.json_preview_dialog$.destroy();
    },
  };

  const state = {
    entries: entries_,
    total: total_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    loading: loading_,
    error: error_,
    keyword: keyword_,
    source: source_,
    level: level_,
    format_json: format_json_,
    auto_refresh: auto_refresh_,
    last_loaded_at: last_loaded_at_,
    log_file_path: log_file_path_,
    json_preview_title: json_preview_title_,
    json_preview_text: json_preview_text_,
  };

  return { state, ui, methods };
}

export { LogsPageViewModel };

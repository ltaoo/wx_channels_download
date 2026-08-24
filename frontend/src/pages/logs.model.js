const LOG_LEVEL_OPTIONS = [
  { value: "all", label: "全部级别" },
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const LOG_FILE_ACCEPT =
  ".log,.txt,.json,.jsonl,.ndjson,text/plain,application/json,application/x-ndjson";

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

function event_target_element(event) {
  const target = event && event.target;
  return target && typeof target.get$elm === "function"
    ? target.get$elm()
    : target;
}

function infer_log_level(value) {
  const text = String(value || "").toLowerCase();
  if (text.includes("error") || text.includes("失败")) {
    return "error";
  }
  if (
    text.includes("warn") ||
    text.includes("warning") ||
    text.includes("警告")
  ) {
    return "warn";
  }
  if (text.includes("debug")) {
    return "debug";
  }
  return "info";
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
      structured_field_value(item.json || item.JSON, "file") || item.file || "",
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
  const raw_log = first_non_empty(
    source.raw,
    source.Raw,
    source.message,
    source.Message,
  );
  let json = first_non_empty(source.json, source.JSON, null);
  if (!json && typeof raw_log === "string") {
    const normalized_raw = raw_log.trim();
    if (normalized_raw.startsWith("{") && normalized_raw.endsWith("}")) {
      try {
        const parsed_json = JSON.parse(normalized_raw);
        if (
          parsed_json &&
          typeof parsed_json === "object" &&
          !Array.isArray(parsed_json)
        ) {
          json = parsed_json;
        }
      } catch {
        json = null;
      }
    }
  }
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
      first_non_empty(source.level, source.Level, infer_log_level(raw_log)),
    ).toLowerCase(),
    message: first_non_empty(
      source.message,
      source.Message,
      source.msg,
      source.Msg,
      source.raw,
      source.Raw,
    ),
    raw: raw_log,
    json,
  };
}

function imported_log_values(value) {
  if (Array.isArray(value)) {
    return value;
  }
  if (!value || typeof value !== "object") {
    return [value];
  }
  const collection_keys = [
    "entries",
    "Entries",
    "list",
    "List",
    "logs",
    "Logs",
  ];
  for (const key of collection_keys) {
    if (Array.isArray(value[key])) {
      return value[key];
    }
  }
  const data = value.data || value.Data;
  if (data && typeof data === "object") {
    for (const key of collection_keys) {
      if (Array.isArray(data[key])) {
        return data[key];
      }
    }
  }
  return [value];
}

function normalize_imported_log_value(value, fallback_index) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    const raw = field_text(value);
    return normalize_log_entry({ raw, message: raw }, fallback_index);
  }
  let raw = first_non_empty(value.raw, value.Raw);
  if (!raw) {
    try {
      raw = JSON.stringify(value);
    } catch {
      raw = field_text(value);
    }
  }
  return normalize_log_entry({ ...value, raw }, fallback_index);
}

function parse_imported_log_content(content) {
  const text = String(content || "")
    .replace(/^\ufeff/, "")
    .trim();
  if (!text) {
    throw new Error("日志文件内容为空");
  }

  let values = null;
  try {
    values = imported_log_values(JSON.parse(text));
  } catch {
    values = text
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        try {
          return JSON.parse(line);
        } catch {
          return line;
        }
      });
  }

  return values
    .filter(
      (value) =>
        value !== null &&
        value !== undefined &&
        (typeof value !== "string" || value.trim() !== ""),
    )
    .map((value, index) => normalize_imported_log_value(value, index + 1));
}

function read_text_file(file) {
  if (file && typeof file.text === "function") {
    return file.text();
  }
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () =>
      reject(reader.error || new Error("读取日志文件失败"));
    reader.readAsText(file);
  });
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
  const clearing_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");
  const source_ = ref("all");
  const level_ = ref("all");
  const auto_refresh_ = ref(false);
  const last_loaded_at_ = ref("");
  const log_file_path_ = ref("");
  const imported_ = ref(false);
  const imported_file_name_ = ref("");
  const imported_entry_count_ = ref(0);
  const importing_ = ref(false);
  const import_dragging_ = ref(false);
  const import_drag_invalid_ = ref(false);
  const import_file_error_ = ref("");
  const json_preview_title_ = ref("JSON 预览");
  const json_preview_text_ = ref("");
  let imported_entries = null;
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
    btn_import$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
    }),
    btn_restore_server_logs$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
    }),
    btn_clear_logs$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      loading: clearing_.value,
      variant: "destructive",
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
    log_file_picker$: new Timeless.vm.FilePickerCore({
      accept: LOG_FILE_ACCEPT,
      multiple: false,
      onChange(files) {
        void select_log_files(files);
      },
    }),
    import_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
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
        reload_current(1);
      },
    }),
    select_level$: new Timeless.vm.SelectCore({
      defaultValue: "all",
      placeholder: "全部级别",
      options: LOG_LEVEL_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        level_.as(value || "all");
        reload_current(1);
      },
    }),
  };
  ui.log_file_picker$.onStateChange((state) => {
    import_dragging_.as(Boolean(state.dragging));
    import_drag_invalid_.as(Boolean(state.invalid));
  });
  ui.log_file_picker$.onReject((data) => {
    reject_log_files(data.files);
  });

  function reject_log_files(rejected_files) {
    const files = Array.from(rejected_files || []);
    const file_names = files
      .map((file) => String((file && file.name) || ""))
      .filter(Boolean)
      .join("、");
    const message = file_names
      ? `不支持文件 ${file_names}，请选择日志文件`
      : "不支持该文件类型，请选择日志文件";
    import_file_error_.as(message);
    show_toast(message);
    return false;
  }
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
  clearing_.subscribe({
    onChange(clearing) {
      ui.btn_clear_logs$.setLoading(Boolean(clearing));
    },
  });
  loading_.subscribe({
    onChange(loading) {
      [
        ui.btn_search$,
        ui.btn_reset$,
        ui.btn_export$,
        ui.btn_import$,
        ui.btn_restore_server_logs$,
        ui.btn_clear_logs$,
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
          normalize_logs_response(response.data, page_.value, page_size_.value),
        );
      },
    },
  );
  const clear_logs_request_core = new Timeless.kit.RequestCore(
    () => logs_request.post("/api/logs/clear"),
    { client: props.client },
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
  const imported_label_ = combine(
    {
      fileName: imported_file_name_,
      entryCount: imported_entry_count_,
    },
    (state) => `${state.fileName || "外部日志"} · ${state.entryCount} 条`,
  );

  function sync_source_options(source_entries = entries_.value) {
    const sources = unique_components(source_entries);
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

  function matches_imported_entry(entry) {
    if (level_.value !== "all" && entry.level !== level_.value) {
      return false;
    }
    const source = String(source_.value || "").toLowerCase();
    if (
      source &&
      source !== "all" &&
      ![entry.source, entry.file, entry.component].some((value) =>
        String(value || "")
          .toLowerCase()
          .includes(source),
      )
    ) {
      return false;
    }
    const keyword = String(keyword_.value || "")
      .trim()
      .toLowerCase();
    if (!keyword) {
      return true;
    }
    return [
      entry.message,
      entry.raw,
      entry.source,
      entry.file,
      entry.component,
      field_text(entry.json),
    ].some((value) =>
      String(value || "")
        .toLowerCase()
        .includes(keyword),
    );
  }

  function apply_imported_entries(target_page = page_.value) {
    const all_entries = Array.isArray(imported_entries) ? imported_entries : [];
    const filtered_entries = all_entries.filter(matches_imported_entry);
    const page_size = PAGE_SIZE_DEFAULT;
    const page_count = Math.max(
      1,
      Math.ceil(filtered_entries.length / page_size),
    );
    const requested_page = Math.min(
      page_count,
      Math.max(1, Number(target_page) || 1),
    );
    const offset = (requested_page - 1) * page_size;
    entries_.as(filtered_entries.slice(offset, offset + page_size), {
      reset: true,
    });
    total_.as(filtered_entries.length);
    page_.as(requested_page);
    page_size_.as(page_size);
    log_file_path_.as("");
    last_loaded_at_.as(format_datetime(new Date()));
    sync_source_options(all_entries);
    return true;
  }

  function reload_current(target_page = page_.value) {
    if (imported_.value) {
      return apply_imported_entries(target_page);
    }
    return load(target_page);
  }

  function clear_imported_entries() {
    imported_entries = null;
    imported_.as(false);
    imported_file_name_.as("");
    imported_entry_count_.as(0);
  }

  function show_toast(message) {
    if (window.DLUtils && DLUtils.toast) {
      DLUtils.toast(message);
    }
  }

  async function import_log_file(file) {
    const resume_auto_refresh = auto_refresh_.value;
    set_auto_refresh(false);
    const sequence = ++request_sequence;
    importing_.as(true);
    import_file_error_.as("");
    ui.log_file_picker$.setLoading(true);
    loading_.as(true);
    error_.as("");
    try {
      const content = await read_text_file(file);
      const parsed_entries = parse_imported_log_content(content);
      if (parsed_entries.length === 0) {
        throw new Error("日志文件中没有可导入的记录");
      }
      if (sequence !== request_sequence) {
        return false;
      }
      imported_entries = parsed_entries;
      imported_.as(true);
      imported_file_name_.as(String(file.name || "外部日志"));
      imported_entry_count_.as(parsed_entries.length);
      keyword_.as("");
      source_.as("all");
      level_.as("all");
      ui.select_source$.setValue("all");
      ui.select_level$.setValue("all");
      apply_imported_entries(1);
      ui.import_dialog$.hide();
      show_toast(`已导入 ${parsed_entries.length} 条日志`);
      return true;
    } catch (error) {
      if (sequence === request_sequence) {
        const message = error && error.message ? error.message : String(error);
        error_.as(message || "导入日志失败");
        import_file_error_.as(message || "无法读取日志文件");
        show_toast(`导入失败：${message || "无法读取日志文件"}`);
        if (resume_auto_refresh) {
          set_auto_refresh(true);
        }
      }
      return false;
    } finally {
      if (sequence === request_sequence) {
        importing_.as(false);
        ui.log_file_picker$.setLoading(false);
        ui.log_file_picker$.setValue(null, { silence: true });
        loading_.as(false);
      }
    }
  }

  function select_log_files(files) {
    const selected_files = Array.from(files || []);
    const valid_files = ui.log_file_picker$.filter_valid_files({
      files: selected_files,
    });
    const file = valid_files[0];
    if (!file) {
      return selected_files.length > 0
        ? reject_log_files(selected_files)
        : false;
    }
    return import_log_file(file);
  }

  function restore_server_logs() {
    clear_imported_entries();
    return load(1);
  }

  async function clear_logs() {
    if (
      clearing_.value ||
      !window.confirm(
        "确定要清空日志吗？这会同时清空当前页面记录和日志文件内容，此操作不可恢复。",
      )
    ) {
      return false;
    }

    const resume_auto_refresh = auto_refresh_.value;
    set_auto_refresh(false);
    const sequence = ++request_sequence;
    clearing_.as(true);
    loading_.as(true);
    error_.as("");
    try {
      const clear_result = await clear_logs_request_core.run();
      if (sequence !== request_sequence) {
        return false;
      }
      if (clear_result.error) {
        const message =
          clear_result.error.message ||
          clear_result.error.msg ||
          String(clear_result.error);
        error_.as(message);
        show_toast(`清空失败：${message}`);
        if (resume_auto_refresh) {
          set_auto_refresh(true, true);
        }
        return false;
      }

      const data = clear_result.data || {};
      const cleared_log_file = (data.files || []).find(
        (file) => file && file.path,
      );
      clear_imported_entries();
      entries_.as([], { reset: true });
      total_.as(0);
      page_.as(1);
      page_size_.as(PAGE_SIZE_DEFAULT);
      log_file_path_.as(cleared_log_file ? String(cleared_log_file.path) : "");
      last_loaded_at_.as(format_datetime(new Date()));
      if (resume_auto_refresh) {
        set_auto_refresh(true, true);
      }
      show_toast("日志文件及当前记录已清空");
      return true;
    } catch (error) {
      if (sequence === request_sequence) {
        const message = error && error.message ? error.message : String(error);
        error_.as(message || "清空日志失败");
        show_toast(`清空失败：${message || "请求失败"}`);
        if (resume_auto_refresh) {
          set_auto_refresh(true, true);
        }
      }
      return false;
    } finally {
      if (sequence === request_sequence) {
        clearing_.as(false);
        loading_.as(false);
      }
    }
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function set_auto_refresh(value, force = false) {
    if (clearing_.value && !force) {
      ui.checkbox_auto_refresh$.setValue(auto_refresh_.value, {
        silence: true,
      });
      return;
    }
    const enabled = Boolean(value);
    if (enabled && imported_.value) {
      clear_imported_entries();
      load(1);
    }
    auto_refresh_.as(enabled);
    restart_timer();
  }

  function reset_filters() {
    keyword_.as("");
    source_.as("all");
    ui.select_source$.setValue("all");
    level_.as("all");
    ui.select_level$.setValue("all");
    return reload_current(1);
  }

  const methods = {
    ready() {
      return load(1);
    },
    refresh() {
      return reload_current(page_.value);
    },
    showImportDialog() {
      import_file_error_.as("");
      ui.log_file_picker$.setValue(null, { silence: true });
      ui.import_dialog$.show();
    },
    handleLogFilePickerChange(event) {
      const target = event_target_element(event);
      const files = target && target.files ? target.files : null;
      ui.log_file_picker$.handleChange({ target: { files } });
      if (target) {
        target.value = "";
      }
      return Boolean(files && files.length);
    },
    handleImportDropZoneKeyDown(event) {
      if (!event || (event.key !== "Enter" && event.key !== " ")) {
        return false;
      }
      event.preventDefault();
      const drop_zone = event.currentTarget || event_target_element(event);
      const file_input =
        drop_zone && drop_zone.parentElement
          ? drop_zone.parentElement.querySelector('input[type="file"]')
          : null;
      if (!file_input) {
        return false;
      }
      file_input.click();
      return true;
    },
    restoreServerLogs: restore_server_logs,
    clearLogs: clear_logs,
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
      return reload_current(1);
    },
    setKeyword: set_keyword,
    setAutoRefresh: set_auto_refresh,
    resetFilters: reset_filters,
    previousPage() {
      if (page_.value <= 1 || loading_.value) {
        return null;
      }
      return reload_current(page_.value - 1);
    },
    nextPage() {
      if (page_.value >= page_count_.value || loading_.value) {
        return null;
      }
      return reload_current(page_.value + 1);
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
      json_preview_title_.as(field_name ? `${field_name} · JSON` : "JSON 预览");
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
      request_sequence += 1;
      ui.log_file_picker$.destroy();
      ui.import_dialog$.destroy();
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
    clearing: clearing_,
    error: error_,
    keyword: keyword_,
    source: source_,
    level: level_,
    auto_refresh: auto_refresh_,
    last_loaded_at: last_loaded_at_,
    log_file_path: log_file_path_,
    imported: imported_,
    imported_file_name: imported_file_name_,
    imported_entry_count: imported_entry_count_,
    imported_label: imported_label_,
    importing: importing_,
    import_dragging: import_dragging_,
    import_drag_invalid: import_drag_invalid_,
    import_file_error: import_file_error_,
    json_preview_title: json_preview_title_,
    json_preview_text: json_preview_text_,
  };

  return { state, ui, methods };
}

export { LogsPageViewModel };

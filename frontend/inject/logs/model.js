/// <reference path="../utils.js" />
/**
 * @file Logs page data requests, filter state, and formatting helpers.
 */
var LogsPageModel = (() => {
  const LOG_LEVEL_OPTIONS = [
    { value: "all", label: "全部级别" },
    { value: "debug", label: "Debug" },
    { value: "info", label: "Info" },
    { value: "warn", label: "Warn" },
    { value: "error", label: "Error" },
  ];

  const LOG_LIMIT_OPTIONS = [
    { value: "100", label: "100 条" },
    { value: "300", label: "300 条" },
    { value: "800", label: "800 条" },
    { value: "2000", label: "2000 条" },
  ];

  const logs_api_origin = WXEnv.get("apiOrigin");
  const logs_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: logs_api_origin,
  });
  Timeless.web.provide_http_client(logs_http_client);

  const logs_request = Timeless.request_factory({
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
    return new Timeless.ui.SelectItemCore({ label, value });
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
      index: number_or_default(first_non_empty(source.index, source.Index), fallbackIndex),
      file,
      component,
      source: first_non_empty(source.source, source.Source, component, ""),
      time: first_non_empty(source.time, source.Time, source.timestamp, source.Timestamp),
      level: String(first_non_empty(source.level, source.Level, "info")).toLowerCase(),
      message: first_non_empty(source.message, source.Message, source.msg, source.Msg, source.raw, source.Raw),
      raw: first_non_empty(source.raw, source.Raw, source.message, source.Message),
      json,
      formatted: first_non_empty(source.formatted, source.Formatted),
    };
  }

  function normalize_logs_response(data, fallbackLimit) {
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
    return {
      entries,
      total: Math.max(
        0,
        number_or_default(
          first_non_empty(source.total, source.Total),
          entries.length,
        ),
      ),
      limit: Math.max(
        1,
        number_or_default(
          first_non_empty(source.limit, source.Limit),
          fallbackLimit,
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

  function create_model() {
    const entries_ = refarr([]);
    const total_ = ref(0);
    const loading_ = ref(false);
    const error_ = ref("");
    const keyword_ = ref("");
    const source_ = ref("all");
    const level_ = ref("all");
    const limit_ = ref("300");
    const format_json_ = ref(true);
    const auto_refresh_ = ref(false);
    const last_loaded_at_ = ref("");
    let timer = null;
    let request_sequence = 0;

    const source_select_ = new Timeless.ui.SelectCore({
      defaultValue: "all",
      placeholder: "全部组件",
      options: [option("全部组件", "all")],
      onChange(value) {
        source_.as(value || "all");
        load();
      },
    });
    const level_select_ = new Timeless.ui.SelectCore({
      defaultValue: "all",
      placeholder: "全部级别",
      options: LOG_LEVEL_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        level_.as(value || "all");
        load();
      },
    });
    const limit_select_ = new Timeless.ui.SelectCore({
      defaultValue: "300",
      placeholder: "显示条数",
      options: LOG_LIMIT_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        limit_.as(value || "300");
        load();
      },
    });

    const logs_request_core = new Timeless.RequestCore(
      (params) => logs_request.get("/api/logs", params),
      {
        client: logs_http_client,
        process(response) {
          if (response.error) {
            return Timeless.Result.Err(response.error);
          }
          return Timeless.Result.Ok(
            normalize_logs_response(response.data, Number(limit_.value) || 300),
          );
        },
      },
    );

    const summary_text_ = combine(
      { entries: entries_, total: total_ },
      (state) =>
        `${state.entries.length}/${state.total || 0} 条`,
    );

    function sync_source_options() {
      const sources = unique_components(entries_.value);
      source_select_.setOptions(
        sources.map((source) =>
          option(source === "all" ? "全部组件" : source, source),
        ),
      );
      if (!sources.includes(source_.value)) {
        source_.as("all");
        source_select_.setValue("all");
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

    async function load() {
      const sequence = ++request_sequence;
      loading_.as(true);
      error_.as("");
      let result;
      try {
        result = await logs_request_core.run({
          levels: level_.value === "all" ? "" : level_.value,
          keyword: String(keyword_.value || "").trim(),
          source: source_.value,
          limit: Number(limit_.value) || 300,
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
        error_.as(result.error.message || result.error.msg || String(result.error));
        return result;
      }
      const data = result.data || {};
      entries_.as(data.entries || [], { reset: true });
      total_.as(data.total || 0);
      limit_.as(String(data.limit || limit_.value || "300"));
      last_loaded_at_.as(format_datetime(new Date()));
      sync_source_options();
      return result;
    }

    const methods = {
      ready() {
        return load();
      },
      refresh() {
        return load();
      },
      search() {
        return load();
      },
      setKeyword(value) {
        keyword_.as(String(value || ""));
      },
      setFormatJson(value) {
        format_json_.as(Boolean(value));
        return load();
      },
      setAutoRefresh(value) {
        auto_refresh_.as(Boolean(value));
        restart_timer();
      },
      resetFilters() {
        keyword_.as("");
        source_.as("all");
        source_select_.setValue("all");
        level_.as("all");
        level_select_.setValue("all");
        limit_.as("300");
        limit_select_.setValue("300");
        format_json_.as(true);
        return load();
      },
      exportLogs() {
        const list = Array.isArray(entries_.value) ? entries_.value : [];
        if (list.length === 0) {
          if (window.WXU && WXU.toast) {
            WXU.toast("没有可导出的日志");
          }
          return;
        }
        export_entries(list);
        if (window.WXU && WXU.toast) {
          WXU.toast("日志已导出");
        }
      },
      destroy() {
        if (timer) {
          clearInterval(timer);
        }
        timer = null;
      },
    };

    return {
      state: {
        entries: entries_,
        total: total_,
        loading: loading_,
        error: error_,
        keyword: keyword_,
        source: source_,
        level: level_,
        limit: limit_,
        format_json: format_json_,
        auto_refresh: auto_refresh_,
        last_loaded_at: last_loaded_at_,
        summary_text: summary_text_,
      },
      ui: {
        source: source_select_,
        level: level_select_,
        limit: limit_select_,
      },
      methods,
      ready: methods.ready,
    };
  }

  return create_model;
})();

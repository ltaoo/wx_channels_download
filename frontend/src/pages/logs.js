(function (global) {
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
  const logs_http_client = new Timeless.kit.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: logs_api_origin,
  });
  Timeless.web.provide_http_client(logs_http_client);

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

  async function write_clipboard(text) {
    if (
      navigator.clipboard &&
      typeof navigator.clipboard.writeText === "function"
    ) {
      try {
        await navigator.clipboard.writeText(text);
        return;
      } catch {
        // Fall back to the document-based helper for non-secure/denied contexts.
      }
    }
    if (window.WXU && typeof WXU.copy === "function") {
      WXU.copy(text);
      return;
    }
    throw new Error("当前浏览器不支持复制到剪贴板");
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

  function LogsPageViewModel() {
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
    const json_preview_title_ = ref("JSON 预览");
    const json_preview_text_ = ref("");
    const json_preview_dialog_ = new Timeless.vm.DialogCore({
      closeable: true,
    });
    const hide_json_preview_dialog =
      json_preview_dialog_.hide.bind(json_preview_dialog_);
    json_preview_dialog_.hide = (options = {}) =>
      hide_json_preview_dialog({ destroy: false, ...options });
    let timer = null;
    let request_sequence = 0;

    const source_select_ = new Timeless.vm.SelectCore({
      defaultValue: "all",
      placeholder: "全部组件",
      options: [option("全部组件", "all")],
      onChange(value) {
        source_.as(value || "all");
        load();
      },
    });
    const level_select_ = new Timeless.vm.SelectCore({
      defaultValue: "all",
      placeholder: "全部级别",
      options: LOG_LEVEL_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        level_.as(value || "all");
        load();
      },
    });
    const limit_select_ = new Timeless.vm.SelectCore({
      defaultValue: "300",
      placeholder: "显示条数",
      options: LOG_LIMIT_OPTIONS.map((item) => option(item.label, item.value)),
      onChange(value) {
        limit_.as(value || "300");
        load();
      },
    });

    const logs_request_core = new Timeless.kit.RequestCore(
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
      (state) => `${state.entries.length}/${state.total || 0} 条`,
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
        error_.as(
          result.error.message || result.error.msg || String(result.error),
        );
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
        json_preview_dialog_.show();
        json_preview_title_.as(
          field_name ? `${field_name} · JSON` : "JSON 预览",
        );
        json_preview_text_.as(json_field.formatted_text);
        return true;
      },
      closeJsonFieldValue() {
        json_preview_dialog_.hide();
      },
      async copyJsonFieldValue(value) {
        const text = json_object_field_text(value);
        if (!text) {
          return false;
        }
        try {
          await write_clipboard(text);
          if (window.WXU && WXU.toast) {
            WXU.toast("JSON 已复制");
          }
          return true;
        } catch {
          if (window.WXU && WXU.toast) {
            WXU.toast("复制失败");
          }
          return false;
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
        json_preview_title: json_preview_title_,
        json_preview_text: json_preview_text_,
      },
      ui: {
        source: source_select_,
        level: level_select_,
        limit: limit_select_,
        json_preview_dialog: json_preview_dialog_,
      },
      methods,
      ready: methods.ready,
    };
  }

  function LogsPageView(props) {
    const vm$ = LogsPageViewModel(props);

    return View(
      {
        class: "wx-content-page wx-logs-page",
        onMounted() {
          vm$.ready();
        },
        onUnmounted() {
          vm$.methods.destroy();
        },
      },
      [
        LogsPageHeader({ store: vm$ }),
        View({ class: "wx-content-toolbar-wrap" }, [
          LogsPageToolbar({ store: vm$ }),
        ]),
        View({ class: "wx-content-main" }, [
          LogsPageError({ store: vm$ }),
          LogsPageList({ store: vm$ }),
        ]),
        LogsPageJsonPreviewDialog({ store: vm$ }),
      ],
    );
  }

  function LogsPageActionButton(props) {
    return View(
      {
        type: "button",
        class: [
          "wx-content-action",
          props.compact ? "wx-content-action-compact" : "",
          props.class,
        ]
          .filter(Boolean)
          .join(" "),
        attributes: {
          type: "button",
          title: props.title || "",
          ...(props.attributes || {}),
        },
        onClick: props.onClick,
      },
      [
        props.icon
          ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
          : null,
        typeof props.label !== "undefined"
          ? View({ class: "wx-content-action-label" }, [props.label])
          : null,
      ].filter(Boolean),
    );
  }

  function LogsPageEntryValue(entry_) {
    return entry_ && entry_.value !== undefined ? entry_.value : entry_;
  }

  function LogsPageFormatTime(value) {
    if (!value) {
      return "-";
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return value;
    }
    const pad = (number) => String(number).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
  }

  function LogsPageLevelClass(level) {
    const normalized = String(level || "info").toLowerCase();
    if (
      normalized === "debug" ||
      normalized === "warn" ||
      normalized === "error"
    ) {
      return `wx-logs-level wx-logs-level-${normalized}`;
    }
    return "wx-logs-level wx-logs-level-info";
  }

  function LogsPageHeader(props) {
    const vm$ = props.store;
    return View({ class: "wx-content-header" }, [
      View({ class: "wx-content-header-inner" }, [
        View({ class: "wx-content-brand" }, [
          View({ class: "wx-content-brand-icon" }, [
            Timeless.Icon({ name: "scroll-text", size: 28 }),
          ]),
          View({}, [
            View({ class: "wx-content-title" }, ["日志"]),
            View({ class: "wx-content-subtitle" }, [
              computed(vm$.state.last_loaded_at, (time) => {
                return time
                  ? `运行日志、请求代理和下载器事件 · 更新于 ${time}`
                  : "运行日志、请求代理和下载器事件";
              }),
            ]),
          ]),
        ]),
        View({ class: "wx-logs-header-actions" }, [
          View({ class: "wx-logs-toggle" }, [
            View({
              type: "input",
              attributes: {
                type: "checkbox",
                "aria-label": "自动刷新",
                checked: computed(vm$.state.auto_refresh, (value) =>
                  value ? true : undefined,
                ),
              },
              onChange(event) {
                vm$.methods.setAutoRefresh(event.target.checked);
              },
            }),
            View({}, ["自动刷新"]),
          ]),
          LogsPageActionButton({
            icon: "refresh-cw",
            label: computed(vm$.state.loading, (loading) =>
              loading ? "刷新中" : "刷新",
            ),
            onClick() {
              vm$.methods.refresh();
            },
          }),
        ]),
      ]),
    ]);
  }

  function LogsPageToolbar(props) {
    const vm$ = props.store;
    return View({ class: "wx-content-toolbar wx-logs-toolbar" }, [
      View({ class: "wx-logs-toolbar-row" }, [
        Timeless.Select({
          store: vm$.ui.source,
          class: "wx-content-type-select wx-logs-select wx-logs-source-select",
        }),
        Timeless.Select({
          store: vm$.ui.level,
          class: "wx-content-type-select wx-logs-select",
        }),
        View({ class: "wx-content-search wx-logs-search" }, [
          Timeless.Icon({ name: "search", size: 16 }),
          View({
            type: "input",
            class: "wx-content-search-input",
            attributes: {
              id: "wxLogKeywordInput",
              type: "search",
              placeholder: "搜索消息、字段、原始日志",
              autocomplete: "off",
            },
            onInput(event) {
              vm$.methods.setKeyword(event.target.value);
            },
            onKeyDown(event) {
              if (event.key === "Enter") {
                vm$.methods.search();
              }
            },
          }),
        ]),
        LogsPageActionButton({
          icon: "search",
          label: "搜索",
          onClick() {
            vm$.methods.search();
          },
        }),
        LogsPageActionButton({
          icon: "rotate-ccw",
          label: "重置",
          onClick() {
            vm$.methods.resetFilters();
            const input = document.getElementById("wxLogKeywordInput");
            if (input) {
              input.value = "";
            }
          },
        }),
        LogsPageActionButton({
          icon: "download",
          label: "导出",
          onClick() {
            vm$.methods.exportLogs();
          },
        }),
      ]),
      View({ class: "wx-logs-toolbar-extra" }, [
        Timeless.Select({
          store: vm$.ui.limit,
          class: "wx-content-type-select wx-logs-select",
        }),
        View({ class: "wx-logs-toggle" }, [
          View({
            type: "input",
            attributes: {
              type: "checkbox",
              "aria-label": "格式化 JSON",
              checked: computed(vm$.state.format_json, (value) =>
                value ? true : undefined,
              ),
            },
            onChange(event) {
              vm$.methods.setFormatJson(event.target.checked);
            },
          }),
          View({}, ["格式化 JSON"]),
        ]),
        View({ class: "wx-logs-summary" }, [vm$.state.summary_text]),
      ]),
    ]);
  }

  function LogsPageError(props) {
    const vm$ = props.store;
    return Show({
      when: computed(vm$.state.error, (message) => Boolean(message)),
      ok() {
        return View({ class: "wx-logs-error" }, [
          Timeless.Icon({ name: "circle-alert", size: 18 }),
          View({}, [vm$.state.error]),
        ]);
      },
    });
  }

  function LogsPageRow(props) {
    const vm$ = props.store;
    const entry = LogsPageEntryValue(props.entry) || {};
    const file = entry.file || entry.File || "";
    const component = entry.component || entry.Component || "";
    return View({ class: "wx-logs-table-row", attributes: { role: "row" } }, [
      View({ class: "wx-logs-level-cell", attributes: { role: "cell" } }, [
        View({ class: LogsPageLevelClass(entry.level) }, [
          entry.level || "info",
        ]),
      ]),
      View({ class: "wx-logs-time-cell", attributes: { role: "cell" } }, [
        LogsPageFormatTime(entry.time),
      ]),
      View(
        {
          class: "wx-logs-file-cell",
          attributes: { role: "cell", title: file },
        },
        [file],
      ),
      View(
        {
          class: "wx-logs-component-cell",
          attributes: { role: "cell", title: component },
        },
        [component],
      ),
      View({ class: "wx-logs-msg-cell", attributes: { role: "cell" } }, [
        entry.message || entry.raw || "",
      ]),
      View({ class: "wx-logs-fields-cell", attributes: { role: "cell" } }, [
        LogsPageFieldsCell({ entry, store: vm$ }),
      ]),
    ]);
  }

  function LogsPageFieldsCell(props) {
    const vm$ = props.store;
    const fields = vm$.methods.fieldRows(props.entry);
    if (fields.length === 0) {
      return View({ class: "wx-logs-fields-empty" }, ["-"]);
    }
    return View({ class: "wx-logs-fields" }, [
      For({
        each: fields,
        render(row) {
          const text = `${row[0]}=${row[1]}`;
          const is_json = vm$.methods.isJsonFieldValue(row[1]);
          return View({ class: "wx-logs-field", attributes: { title: text } }, [
            View({ class: "wx-logs-field-key" }, [row[0]]),
            is_json
              ? View({ class: "wx-logs-field-json-actions" }, [
                  View(
                    {
                      type: "button",
                      class: "wx-logs-field-value wx-logs-field-value-preview",
                      attributes: {
                        type: "button",
                        title: `点击查看 ${row[0]} 的格式化 JSON`,
                        "aria-label": `查看 ${row[0]} 的格式化 JSON`,
                      },
                      onClick() {
                        vm$.methods.showJsonFieldValue(row[0], row[1]);
                      },
                    },
                    [View({ class: "wx-logs-field-value-text" }, [row[1]])],
                  ),
                  View(
                    {
                      type: "button",
                      class: "wx-logs-field-copy-button",
                      attributes: {
                        type: "button",
                        title: `复制 ${row[0]} 的 JSON 值`,
                        "aria-label": `复制 ${row[0]} 的 JSON 值`,
                      },
                      onClick() {
                        vm$.methods.copyJsonFieldValue(row[1]);
                      },
                    },
                    [
                      View({ class: "wx-logs-field-copy-icon" }, [
                        Timeless.Icon({ name: "copy", size: 12 }),
                      ]),
                    ],
                  ),
                ])
              : View({ class: "wx-logs-field-value" }, [row[1]]),
          ]);
        },
      }),
    ]);
  }

  function LogsPageJsonPreviewDialog(props) {
    const vm$ = props.store;
    return Dialog(
      {
        store: vm$.ui.json_preview_dialog,
        class: "wx-logs-json-dialog",
      },
      [
        View({ class: "wx-logs-json-dialog-header" }, [
          View({ class: "wx-logs-json-dialog-heading" }, [
            View({ class: "wx-logs-json-dialog-title" }, [
              vm$.state.json_preview_title,
            ]),
            View({ class: "wx-logs-json-dialog-subtitle" }, [
              "格式化 JSON 内容",
            ]),
          ]),
          View(
            {
              type: "button",
              class: "wx-logs-json-dialog-close",
              attributes: {
                type: "button",
                title: "关闭",
                "aria-label": "关闭 JSON 预览",
              },
              onClick() {
                vm$.methods.closeJsonFieldValue();
              },
            },
            ["×"],
          ),
        ]),
        View({ class: "wx-logs-json-dialog-body" }, [
          View({ type: "pre", class: "wx-logs-json-dialog-content" }, [
            vm$.state.json_preview_text,
          ]),
        ]),
      ],
    );
  }

  function LogsPageTableHead() {
    return View({ class: "wx-logs-table-head", attributes: { role: "row" } }, [
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["level"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["time"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["file"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["component"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["msg"],
      ),
      View(
        {
          class: "wx-logs-table-head-cell",
          attributes: { role: "columnheader" },
        },
        ["fields"],
      ),
    ]);
  }

  function LogsPageTable(props) {
    const vm$ = props.store;
    return View({ class: "wx-logs-table", attributes: { role: "table" } }, [
      LogsPageTableHead(),
      VirtualListView({
        class: "wx-logs-virtual-list",
        key(entry, index) {
          return `${entry.index || index}-${entry.time || ""}-${entry.message || ""}`;
        },
        size: 18,
        buffer: 8,
        gutter: 0,
        itemHeight: 48,
        each: vm$.state.entries,
        render(entry) {
          return LogsPageRow({ entry, store: vm$ });
        },
      }),
    ]);
  }

  function LogsPageSkeletonTable() {
    return View({ class: "wx-logs-table", attributes: { role: "table" } }, [
      LogsPageTableHead(),
      View(
        { class: "wx-logs-virtual-list" },
        Array.from({ length: 6 }, function () {
          return View(
            { class: "wx-logs-table-row", attributes: { role: "row" } },
            [
              View({ class: "wx-logs-level-cell" }, [
                View({ class: "wx-logs-level wx-logs-level-info" }, ["info"]),
              ]),
              View({ class: "wx-logs-time-cell" }, [
                View({
                  class: "wx-logs-skeleton-cell",
                  style: { width: "150px" },
                }),
              ]),
              View({ class: "wx-logs-file-cell" }, [
                View({
                  class: "wx-logs-skeleton-cell",
                  style: { width: "80px" },
                }),
              ]),
              View({ class: "wx-logs-component-cell" }, [
                View({
                  class: "wx-logs-skeleton-cell",
                  style: { width: "100px" },
                }),
              ]),
              View({ class: "wx-logs-msg-cell" }, [
                View({
                  class: "wx-logs-skeleton-cell",
                  style: { width: "100%" },
                }),
              ]),
              View({ class: "wx-logs-fields-cell" }, [
                View({
                  class: "wx-logs-skeleton-cell",
                  style: { width: "80%" },
                }),
              ]),
            ],
          );
        }),
      ),
    ]);
  }

  function LogsPageLoadingState() {
    return LogsPageSkeletonTable();
  }

  function LogsPageEmptyState() {
    return View({ class: "wx-logs-state" }, [
      Timeless.Icon({ name: "file-search", size: 32 }),
      View({}, ["没有匹配的日志"]),
    ]);
  }

  function LogsPageList(props) {
    const vm$ = props.store;
    return View({ class: "wx-logs-list" }, [
      View({ class: "wx-logs-list-head" }, [
        View({ class: "wx-logs-list-title" }, ["日志流"]),
        Show({
          when: vm$.state.loading,
          ok() {
            return View({ class: "wx-logs-loading-badge" }, [
              View({ class: "weui-loading" }),
              View({}, ["加载中"]),
            ]);
          },
        }),
      ]),
      View({ class: "wx-logs-scroll" }, [
        Show({
          when: computed(
            vm$.state.loading,
            (loading) => loading && vm$.state.entries.value.length === 0,
          ),
          ok() {
            return LogsPageLoadingState();
          },
          else() {
            return Show({
              when: computed(
                vm$.state.entries,
                (entries) => entries.length > 0,
              ),
              ok() {
                return LogsPageTable({ store: vm$ });
              },
              else() {
                return LogsPageEmptyState();
              },
            });
          },
        }),
      ]),
    ]);
  }

  global.register("logs_page", LogsPageView);
})(window);

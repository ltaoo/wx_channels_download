const Timeless = window.Timeless;

if (!Timeless) {
  throw new Error("应用无法启动：Timeless 运行时未加载");
}

export const VISIBLE_PAGE_SIZE = 100;
export const BULK_PAGE_SIZE = 1000;

export const STATUS_ITEMS = Object.freeze([
  ["all", "全部"],
  ["running", "下载中"],
  ["pause", "暂停"],
  ["wait", "等待中"],
  ["done", "已完成"],
  ["error", "失败"],
]);

export const DETAIL_FILTER_ITEMS = Object.freeze([
  ["all", "全部"],
  ["success", "成功"],
  ["failed", "失败"],
  ["skipped", "跳过"],
]);

const DETAIL_FILTER_NAMES = Object.freeze({
  all: "全部",
  success: "成功",
  failed: "失败",
  skipped: "跳过",
});

const http_client = new Timeless.kit.HttpClientCore({
  headers: { "Content-Type": "application/json" },
});

Timeless.web.provide_http_client(http_client);

const request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    const code = payload.code;
    const msg = payload.msg;
    const data = payload.data;
    if (code !== 0) {
      return Timeless.Result.Err(msg, code, data);
    }
    return Timeless.Result.Ok(data);
  },
});

const api_requests = {
  table: new Timeless.kit.RequestCore(
    (body) => request.post("/api/v1/migration/table", body),
    { client: http_client },
  ),
  common_dirs: new Timeless.kit.RequestCore(
    () => request.get("/api/v1/migration/common_dirs"),
    { client: http_client },
  ),
  file_list: new Timeless.kit.RequestCore(
    (body) => request.post("/api/v1/fs/list", body),
    { client: http_client },
  ),
  execute: new Timeless.kit.RequestCore(
    (body) => request.post("/api/v1/migration/execute", body),
    { client: http_client },
  ),
  profile_cache_cleanup: new Timeless.kit.RequestCore(
    (body) =>
      request.post("/api/v1/migration/profile-cache/cleanup", body),
    { client: http_client },
  ),
  profile: new Timeless.kit.RequestCore(
    (query) =>
      request.get(
        "/api/channels/feed/profile?" +
          new URLSearchParams(query).toString(),
      ),
    { client: http_client },
  ),
  article_profile: new Timeless.kit.RequestCore(
    (query) =>
      request.get(
        "/api/mp/article/profile?" + new URLSearchParams(query).toString(),
      ),
    { client: http_client },
  ),
  profile_agent_status: new Timeless.kit.RequestCore(
    () => request.get("/api/channels/profile-agent/status"),
    { client: http_client },
  ),
};

export function error_message(error) {
  if (!error) return "请求失败";
  return error.message || String(error);
}

export function format_size(bytes) {
  const value = Number(bytes) || 0;
  if (value < 1024) return value + " B";
  const units = ["KB", "MB", "GB", "TB"];
  let size = value / 1024;
  let unit_index = 0;
  while (size >= 1024 && unit_index < units.length - 1) {
    size /= 1024;
    unit_index += 1;
  }
  return size.toFixed(1) + " " + units[unit_index];
}

export function format_progress(value) {
  return Number(value || 0).toFixed(2) + "%";
}

export function status_text(status) {
  const labels = {
    ready: "就绪",
    running: "下载中",
    pause: "暂停",
    wait: "等待中",
    error: "失败",
    done: "已完成",
  };
  return labels[status] || status || "";
}

export function status_variant(status) {
  if (status === "done" || status === "success") return "success";
  if (status === "error" || status === "failed") return "danger";
  if (status === "pause" || status === "wait" || status === "skipped") {
    return "warning";
  }
  return "info";
}

export function stringify_labels(labels) {
  if (!labels || typeof labels !== "object") {
    return "";
  }
  try {
    return JSON.stringify(labels, null, 2);
  } catch {
    return String(labels);
  }
}

function status_filter_text(status) {
  const item = STATUS_ITEMS.find(([key]) => key === status);
  return item ? item[1] : status_text(status) || status || "全部";
}

function first_label_value(labels, keys) {
  if (!labels || typeof labels !== "object") {
    return "";
  }
  for (const key of keys) {
    const value = labels[key];
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      return String(value).trim();
    }
  }
  return "";
}

function row_oid(row) {
  return (
    String(row.oid || "").trim() ||
    first_label_value(row.labels, [
      "oid",
      "id",
      "external_id",
      "objectid",
      "object_id",
    ])
  );
}

function row_uid(row) {
  return (
    String(row.uid || "").trim() ||
    first_label_value(row.labels, [
      "uid",
      "nid",
      "nonce_id",
      "objectNonceId",
      "object_nonce_id",
    ])
  );
}

function row_article_id(row) {
  return (
    String(row.article_id || "").trim() ||
    first_label_value(row.labels, [
      "article_id",
      "articleid",
      "mp_article_id",
    ])
  );
}

function row_cache_key(row) {
  return (
    first_label_value(row.labels, [
      "id",
      "oid",
      "external_id",
      "objectid",
      "object_id",
    ]) || row_oid(row)
  );
}

function task_detail_key(row) {
  if (!row) return "";
  return (
    String(row.id || row.task_id || "").trim() ||
    row_article_id(row) ||
    row_cache_key(row) ||
    row_oid(row)
  );
}

function validate_channels_profile_detail(data) {
  let profile = data && data.profile;
  if (typeof profile === "string") {
    try {
      profile = JSON.parse(profile);
    } catch {
      throw new Error("获取详情失败：后端返回的 profile 不是有效 JSON");
    }
  }
  const object_value =
    profile && profile.data && profile.data.object;
  const object_id =
    object_value && object_value.id !== undefined && object_value.id !== null
      ? String(object_value.id).trim()
      : "";
  if (!object_id) {
    throw new Error("获取详情失败：后端返回的 object.id 为空");
  }
  return data;
}

function dir_name(value) {
  const clean = String(value || "/").replace(/[\\/]+$/, "");
  const index = Math.max(clean.lastIndexOf("/"), clean.lastIndexOf("\\"));
  if (index <= 0) return "/";
  return clean.slice(0, index);
}

function is_db_file_path(value) {
  return /\.db$/i.test(String(value || "").trim());
}

export function MigrationViewModel() {
  let state = {
    data_dir: "",
    db_path: "",
    status: "all",
    detail_filter: "all",
    detail_page: 1,
    page: 1,
    total_pages: 1,
    columns: [],
    rows: [],
    total: 0,
    stats: { total: 0 },
    cache_hit: false,
  };
  let detail_fetch_batch = null;
  let toast_timer = 0;
  let confirm_resolve = null;
  let picker_dir = "";
  let picker_files = [];
  let picker_selected_path = "";
  let picker_loading = false;
  let picker_error = "";

  const detail_fetched_task_ids = new Set();
  const detail_fetched_sources = new Map();
  const detail_fetch_results = new Map();

  const version_ = Timeless.ref(0);
  const loading_ = Timeless.ref(false);
  const subtitle_ = Timeless.ref(
    "选择 Gopeed .db 数据库文件，读取旧下载任务列表",
  );
  const fetch_all_label_ = Timeless.ref("获取所有任务详情");
  const migration_result_ = Timeless.ref(null);
  const toast_message_ = Timeless.ref("");
  const toast_error_ = Timeless.ref(false);
  const profile_title_ = Timeless.ref("任务详情");
  const profile_json_ = Timeless.ref("");
  const confirm_message_ = Timeless.ref("");

  const ui = {
    path_input$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "Gopeed .db 数据库文件路径",
      type: "text",
      allowClear: true,
      onEnter() {
        reset_detail_filter();
        return load_tasks(1);
      },
    }),
    target_input$: new Timeless.vm.InputCore({
      defaultValue: "http://127.0.0.1:2022",
      placeholder: "目标服务地址",
      type: "url",
      allowClear: true,
    }),
    target_db_input$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "wx_channels_download data.db 路径",
      type: "text",
      allowClear: true,
    }),
    btn_load$: new Timeless.vm.ButtonCore({
      variant: "primary",
      onClick() {
        reset_detail_filter();
        return load_tasks(1);
      },
    }),
    btn_browse$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        return open_picker();
      },
    }),
    btn_refresh$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        reset_detail_filter();
        return load_tasks(state.page, { force_reload: true });
      },
    }),
    btn_cleanup_cache$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        return cleanup_invalid_profile_cache();
      },
    }),
    btn_fetch_all_details$: new Timeless.vm.ButtonCore({
      variant: "outline",
      disabled: true,
      onClick() {
        return fetch_all_task_details();
      },
    }),
    btn_migration_execute$: new Timeless.vm.ButtonCore({
      variant: "primary",
      onClick() {
        return execute_migration();
      },
    }),
    btn_status$: new Timeless.vm.ButtonInListCore({
      variant: "ghost",
      size: "sm",
      onClick(status) {
        state.status = status || "all";
        state.page = 1;
        reset_detail_filter();
        touch();
        return load_tasks(1);
      },
    }),
    btn_detail_filter$: new Timeless.vm.ButtonInListCore({
      variant: "ghost",
      size: "xs",
      onClick(filter) {
        state.detail_filter = filter || "all";
        state.detail_page = 1;
        touch();
      },
    }),
    btn_profile_detail$: new Timeless.vm.ButtonInListCore({
      variant: "outline",
      size: "xs",
      onClick(row) {
        return show_profile_detail(row);
      },
    }),
    btn_migrate_row$: new Timeless.vm.ButtonInListCore({
      variant: "primary",
      size: "xs",
      onClick(row) {
        return execute_single_migration(row);
      },
    }),
    profile_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      destroyOnClose: false,
      footer: false,
    }),
    btn_profile_close$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        ui.profile_dialog$.hide();
      },
    }),
    picker_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      destroyOnClose: false,
      footer: false,
    }),
    btn_picker_cancel$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        close_picker();
      },
    }),
    btn_picker_select$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: true,
      onClick() {
        return choose_picker_path(picker_selected_path);
      },
    }),
    btn_picker_breadcrumb$: new Timeless.vm.ButtonInListCore({
      variant: "ghost",
      size: "xs",
      onClick(path) {
        return load_picker_dir(path || "/");
      },
    }),
    btn_picker_file$: new Timeless.vm.ButtonInListCore({
      variant: "ghost",
      onClick(file) {
        picker_selected_path =
          file && !file.isDir && is_db_file_path(file.path)
            ? file.path
            : "";
        sync_picker_select_button();
        touch();
      },
    }),
    confirm_dialog$: new Timeless.vm.DialogCore({
      title: "确认操作",
      closeable: true,
      footer: true,
      onOk() {
        settle_confirm(true);
      },
      onCancel() {
        settle_confirm(false);
      },
    }),
    detail_progress$: new Timeless.vm.ProgressCore({
      value: 0,
      max: 100,
    }),
  };

  function touch() {
    version_.as(version_.value + 1);
  }

  function set_button_enabled(button, enabled) {
    if (enabled) {
      button.enable();
    } else {
      button.disable();
    }
  }

  function sync_action_buttons() {
    const has_tasks = Number(state.total || 0) > 0;
    set_button_enabled(ui.btn_fetch_all_details$, has_tasks);
  }

  function sync_detail_progress() {
    const total = Number((detail_fetch_batch && detail_fetch_batch.total) || 0);
    const processed = Number(
      (detail_fetch_batch && detail_fetch_batch.processed) || 0,
    );
    ui.detail_progress$.setValue(
      total > 0 ? Math.min(100, (processed / total) * 100) : 0,
    );
  }

  function notify(message, is_error) {
    window.clearTimeout(toast_timer);
    toast_error_.as(Boolean(is_error));
    toast_message_.as(String(message || ""));
    toast_timer = window.setTimeout(() => {
      toast_message_.as("");
    }, 3000);
  }

  async function run_request(request_core, params) {
    const result = await request_core.run(params);
    if (result.error) {
      throw result.error;
    }
    return result.data || {};
  }

  function explicit_task_detail_result(row) {
    if (!row) return null;
    const key = task_detail_key(row);
    if (key && detail_fetch_results.has(key)) {
      return detail_fetch_results.get(key);
    }
    if (row.detail_fetch_status) {
      return {
        status: row.detail_fetch_status,
        error: row.detail_fetch_error || "",
        source: row.detail_fetch_source || "",
        cached: Boolean(row.profile_cached),
        cache_key: row.profile_cache_key || "",
      };
    }
    return null;
  }

  function task_detail_result(row) {
    if (!row) return null;
    const explicit = explicit_task_detail_result(row);
    if (explicit) return explicit;
    if (row.detail_fetched || row.profile_cached) {
      return {
        status: "success",
        error: "",
        source: row.profile_cached ? "cache" : "",
        cached: Boolean(row.profile_cached),
        cache_key: row.profile_cache_key || "",
      };
    }
    return null;
  }

  function set_task_detail_result(row, status, values) {
    if (!row) return;
    const result = Object.assign(
      {
        status,
        error: "",
        source: "",
        cached: false,
        profile_url: "",
        cache_key: "",
      },
      values || {},
    );
    const key = task_detail_key(row);
    if (key) {
      detail_fetch_results.set(key, result);
    }
    row.detail_fetch_status = result.status;
    row.detail_fetch_error = result.error || "";
    row.detail_fetch_source = result.source || "";
    if (result.cache_key) {
      row.profile_cache_key = result.cache_key;
    }
    if (result.cached || result.source === "cache") {
      row.profile_cached = true;
    }
    if (result.status === "success") {
      row.detail_fetched = true;
    } else {
      row.detail_fetched = false;
      row.profile_cached = false;
      if (key) {
        detail_fetched_task_ids.delete(key);
        detail_fetched_sources.delete(key);
      }
    }
  }

  function is_task_detail_fetched(row) {
    const result = task_detail_result(row);
    if (result && result.status === "success") return true;
    const key = task_detail_key(row);
    return Boolean(
      row &&
        (row.detail_fetched ||
          row.profile_cached ||
          (key && detail_fetched_task_ids.has(key))),
    );
  }

  function detail_fetched_title(row) {
    const result = task_detail_result(row);
    const key = task_detail_key(row);
    const source = result
      ? result.source
      : key
        ? detail_fetched_sources.get(key)
        : "";
    if (
      (result && (result.cached || result.source === "cache")) ||
      (row && row.profile_cached) ||
      source === "cache"
    ) {
      return "缓存中已有详情";
    }
    if (source === "agent") return "已通过 profile agent 获取详情";
    if (source === "http") return "已通过目标 HTTP 获取详情";
    return "已获取详情";
  }

  function mark_task_detail_fetched(row, data) {
    if (!row) return;
    const key = task_detail_key(row);
    const source = data && data.cached
      ? "cache"
      : (data && data.source) || "fetched";
    if (key) {
      detail_fetched_task_ids.add(key);
      detail_fetched_sources.set(key, source);
    }
    set_task_detail_result(row, "success", {
      source,
      cached: Boolean(data && data.cached),
      profile_url: (data && (data.profile_url || data.article_url)) || "",
      cache_key: (data && data.cache_key) || "",
    });
    row.detail_fetched = true;
    if (data && data.cached) {
      row.profile_cached = true;
    }
  }

  function detail_filter_active() {
    return Boolean(
      detail_fetch_batch &&
        state.detail_filter &&
        state.detail_filter !== "all",
    );
  }

  function detail_rows_for_filter(filter) {
    const rows = (detail_fetch_batch && detail_fetch_batch.rows) || [];
    if (!filter || filter === "all") return rows;
    return rows.filter((row) => {
      const result = explicit_task_detail_result(row);
      return result && result.status === filter;
    });
  }

  function detail_filter_count(filter) {
    if (!detail_fetch_batch) return 0;
    if (!filter || filter === "all") {
      return Number(
        detail_fetch_batch.total || detail_rows_for_filter("all").length || 0,
      );
    }
    return Number(detail_fetch_batch[filter] || 0);
  }

  function detail_summary() {
    if (!detail_fetch_batch) return "";
    const total = Number(detail_fetch_batch.total || 0);
    const processed = Number(detail_fetch_batch.processed || 0);
    const success = Number(detail_fetch_batch.success || 0);
    const skipped = Number(detail_fetch_batch.skipped || 0);
    const failed = Number(detail_fetch_batch.failed || 0);
    const status_label = status_filter_text(
      detail_fetch_batch.status || "all",
    );
    const progress = processed < total
      ? " · 获取中 " +
        processed.toLocaleString() +
        "/" +
        total.toLocaleString()
      : " · 已完成";
    return (
      "最近一次详情获取(" +
      status_label +
      "): 总数 " +
      total.toLocaleString() +
      "，成功 " +
      success.toLocaleString() +
      "，跳过 " +
      skipped.toLocaleString() +
      "，失败 " +
      failed.toLocaleString() +
      progress
    );
  }

  function detail_result_view(row) {
    const result = task_detail_result(row);
    if (!result) return null;
    const status = result.status || "";
    const label = status === "success"
      ? "获取成功"
      : status === "failed"
        ? "获取失败"
        : status === "skipped"
          ? "已跳过"
          : status;
    let detail = "";
    if (status === "success") {
      if (result.cached || result.source === "cache") {
        detail = "缓存命中";
      } else if (result.source) {
        detail = "来源: " + result.source;
      }
      if (result.cache_key) {
        detail += (detail ? " · " : "") + "cache " + result.cache_key;
      }
    } else {
      detail = result.error || "未返回失败原因";
    }
    return {
      status,
      label,
      detail,
      variant: status_variant(status),
    };
  }

  function reset_detail_filter() {
    state.detail_filter = "all";
    state.detail_page = 1;
    touch();
  }

  function table_view() {
    const active_detail_filter = detail_filter_active();
    let rows = state.rows || [];
    let total = Number(state.total || 0);
    let page = Number(state.page || 1);
    let total_pages = Math.max(1, Number(state.total_pages || 1));
    let meta = "";
    let empty_text = "加载 .db 数据库文件后查看任务";

    if (active_detail_filter) {
      const detail_rows = detail_rows_for_filter(state.detail_filter);
      total = detail_rows.length;
      total_pages = Math.max(1, Math.ceil(total / VISIBLE_PAGE_SIZE));
      if (state.detail_page > total_pages) {
        state.detail_page = total_pages;
      }
      page = Number(state.detail_page || 1);
      const start = (page - 1) * VISIBLE_PAGE_SIZE;
      rows = detail_rows.slice(start, start + VISIBLE_PAGE_SIZE);
      const filter_label =
        DETAIL_FILTER_NAMES[state.detail_filter] || state.detail_filter;
      meta =
        "详情" +
        filter_label +
        " " +
        total.toLocaleString() +
        " 条 · 第 " +
        page +
        " / " +
        total_pages +
        " 页";
      empty_text = "没有详情" + filter_label + "记录";
    } else {
      rows = [...rows];
      meta =
        total.toLocaleString() +
        " 条 · 第 " +
        page +
        " / " +
        total_pages +
        " 页" +
        (state.cache_hit ? " · 缓存" : "");
    }

    return {
      rows,
      total,
      page,
      total_pages,
      meta,
      empty_text,
    };
  }

  async function fetch_task_detail(row, db_path_override, options) {
    const article_id = row_article_id(row);
    const oid = row_oid(row);
    const uid = row_uid(row);
    if (!article_id && !oid) {
      throw new Error("缺少 oid 或 article_id");
    }
    const db_path =
      db_path_override || String(ui.path_input$.value || "").trim();
    const target_url = String(ui.target_input$.value || "").trim();
    localStorage.setItem("gopeed_migration_target_url", target_url);
    if (article_id) {
      return run_request(api_requests.article_profile, {
        article_id,
        db_path,
      });
    }
    const query = {
      oid,
      uid,
      nid: uid,
      nonce_id: uid,
      cache_key: row_cache_key(row),
      db_path,
      target_url,
    };
    if (options && options.force_agent) {
      query.force_agent = "1";
    }
    const data = await run_request(api_requests.profile, query);
    return validate_channels_profile_detail(data);
  }

  function row_needs_channels_profile(row) {
    return !row_article_id(row) && Boolean(row_oid(row));
  }

  async function get_profile_agent_status() {
    try {
      return await run_request(api_requests.profile_agent_status);
    } catch (error) {
      return {
        connected: false,
        clients: 0,
        auth_required: false,
        error: error_message(error),
      };
    }
  }

  async function load_all_task_rows(db_path, status, total_hint) {
    const first = await run_request(api_requests.table, {
      db_path,
      table: "task",
      status,
      page: 1,
      page_size: BULK_PAGE_SIZE,
    });
    const rows = first.rows || [];
    const total = Number(first.total || total_hint || rows.length || 0);
    const total_pages = Math.max(1, Number(first.total_pages || 1));
    fetch_all_label_.as(
      "加载任务 " +
        rows.length.toLocaleString() +
        "/" +
        total.toLocaleString(),
    );
    for (let page = 2; page <= total_pages; page += 1) {
      const data = await run_request(api_requests.table, {
        db_path,
        table: "task",
        status,
        page,
        page_size: BULK_PAGE_SIZE,
      });
      rows.push.apply(rows, data.rows || []);
      fetch_all_label_.as(
        "加载任务 " +
          rows.length.toLocaleString() +
          "/" +
          total.toLocaleString(),
      );
    }
    return rows;
  }

  function show_profile_dialog(row, data) {
    ui.profile_dialog$.show();
    profile_title_.as(
      "详情 · " + String(row.title || row.name || row.id || ""),
    );
    profile_json_.as(JSON.stringify(data || {}, null, 2));
  }

  async function show_profile_detail(row) {
    try {
      const data = await fetch_task_detail(row);
      mark_task_detail_fetched(row, data);
      touch();
      show_profile_dialog(row, data);
    } catch (error) {
      const reason = error_message(error);
      set_task_detail_result(row, "failed", { error: reason });
      touch();
      notify(reason, true);
    }
  }

  async function fetch_all_task_details() {
    const db_path = String(ui.path_input$.value || "").trim();
    if (!is_db_file_path(db_path)) {
      notify("请先选择 .db 数据库文件", true);
      return;
    }
    if (!state.total) {
      await load_tasks(1);
    }
    const total = Number(state.total || 0);
    if (!total) {
      notify("没有可获取详情的任务", true);
      return;
    }

    const status = state.status;
    let processed = 0;
    let success = 0;
    let skipped = 0;
    let failed = 0;

    ui.btn_fetch_all_details$.disable();
    ui.btn_fetch_all_details$.setLoading(true);
    fetch_all_label_.as("加载任务 0/" + total.toLocaleString());
    detail_fetch_results.clear();
    detail_fetch_batch = {
      db_path,
      status,
      total,
      processed: 0,
      success: 0,
      skipped: 0,
      failed: 0,
      rows: [],
    };
    state.detail_filter = "all";
    state.detail_page = 1;
    sync_detail_progress();
    touch();

    try {
      const rows = await load_all_task_rows(db_path, status, total);
      if (!rows.length) {
        detail_fetch_batch = null;
        sync_detail_progress();
        touch();
        notify("没有可获取详情的任务", true);
        return;
      }
      detail_fetch_batch.rows = rows;
      detail_fetch_batch.total = rows.length;
      touch();

      const agent_status = await get_profile_agent_status();
      const channels_rows = rows.filter(row_needs_channels_profile);
      let blocked_channels_reason = "";
      if (
        channels_rows.length &&
        agent_status.auth_required &&
        !agent_status.connected
      ) {
        blocked_channels_reason =
          "profile agent token 已配置，但当前没有 agent 连接";
        notify(blocked_channels_reason, true);
      }
      const force_agent =
        channels_rows.length > 0 && Boolean(agent_status.connected);
      const detail_total = rows.length;
      fetch_all_label_.as("获取中 0/" + detail_total.toLocaleString());

      for (const row of rows) {
        processed += 1;
        fetch_all_label_.as(
          "获取中 " +
            processed.toLocaleString() +
            "/" +
            detail_total.toLocaleString(),
        );

        if (!row_article_id(row) && !row_oid(row)) {
          skipped += 1;
          set_task_detail_result(row, "skipped", {
            error: "缺少 oid 或 article_id",
          });
        } else if (
          blocked_channels_reason &&
          row_needs_channels_profile(row)
        ) {
          failed += 1;
          set_task_detail_result(row, "failed", {
            error: blocked_channels_reason,
          });
        } else {
          try {
            const data = await fetch_task_detail(row, db_path, {
              force_agent:
                force_agent && row_needs_channels_profile(row),
            });
            mark_task_detail_fetched(row, data);
            success += 1;
          } catch (error) {
            failed += 1;
            set_task_detail_result(row, "failed", {
              error: error_message(error),
            });
          }
        }

        if (detail_fetch_batch) {
          detail_fetch_batch.processed = processed;
          detail_fetch_batch.success = success;
          detail_fetch_batch.skipped = skipped;
          detail_fetch_batch.failed = failed;
        }
        sync_detail_progress();
        touch();
      }

      notify(
        "任务详情获取完成：成功 " +
          success.toLocaleString() +
          "，跳过 " +
          skipped.toLocaleString() +
          "，失败 " +
          failed.toLocaleString(),
        failed > 0,
      );
    } catch (error) {
      notify(error_message(error), true);
    } finally {
      ui.btn_fetch_all_details$.setLoading(false);
      fetch_all_label_.as("获取所有任务详情");
      sync_action_buttons();
    }
  }

  function confirm_action(message) {
    if (confirm_resolve) {
      confirm_resolve(false);
      confirm_resolve = null;
    }
    confirm_message_.as(message);
    ui.confirm_dialog$.show();
    return new Promise((resolve) => {
      confirm_resolve = resolve;
    });
  }

  function settle_confirm(confirmed) {
    const resolve = confirm_resolve;
    confirm_resolve = null;
    if (ui.confirm_dialog$.open) {
      ui.confirm_dialog$.hide();
    }
    if (resolve) resolve(Boolean(confirmed));
  }

  async function execute_single_migration(row) {
    if (!row || !row.id) {
      notify("缺少 task id", true);
      return;
    }
    const confirmed = await confirm_action(
      "确认迁移这一条记录？该操作会直接写入目标数据库，不会启动下载。",
    );
    if (!confirmed) return;
    try {
      const target_url = String(ui.target_input$.value || "").trim();
      const target_db = String(ui.target_db_input$.value || "").trim();
      localStorage.setItem("gopeed_migration_target_url", target_url);
      localStorage.setItem("gopeed_migration_target_db", target_db);
      const data = await run_request(api_requests.execute, {
        db_path: String(ui.path_input$.value || "").trim(),
        status: "all",
        target_url,
        target_db,
        limit: 1,
        task_ids: [String(row.id)],
      });
      migration_result_.as(data);
      notify("单条迁移完成", false);
      await load_tasks(state.page);
    } catch (error) {
      notify(error_message(error), true);
    }
  }

  async function load_tasks(page, options) {
    const input = String(ui.path_input$.value || "").trim();
    if (!is_db_file_path(input)) {
      notify("请选择 .db 数据库文件", true);
      return null;
    }
    loading_.as(true);
    ui.btn_load$.setLoading(true);
    ui.btn_load$.disable();
    try {
      const data = await run_request(api_requests.table, {
        db_path: input,
        table: "task",
        status: state.status,
        page: page || state.page,
        page_size: VISIBLE_PAGE_SIZE,
        force_reload: Boolean(options && options.force_reload),
      });
      state = Object.assign({}, state, data);
      state.page = data.page || 1;
      state.total_pages = data.total_pages || 1;
      if (
        detail_fetch_batch &&
        detail_fetch_batch.db_path &&
        detail_fetch_batch.db_path !== (data.db_path || input)
      ) {
        detail_fetch_batch = null;
        detail_fetch_results.clear();
        state.detail_filter = "all";
        state.detail_page = 1;
        sync_detail_progress();
      }
      const resolved_path = data.db_path || input;
      ui.path_input$.setValue(resolved_path, { silence: true });
      localStorage.setItem("gopeed_migration_path", resolved_path);
      subtitle_.as(data.db_path || data.data_dir || "");
      sync_action_buttons();
      touch();
      return data;
    } catch (error) {
      notify(error_message(error), true);
      return null;
    } finally {
      loading_.as(false);
      ui.btn_load$.setLoading(false);
      ui.btn_load$.enable();
    }
  }

  async function cleanup_invalid_profile_cache() {
    const db_path = String(ui.path_input$.value || "").trim();
    const confirmed = await confirm_action(
      "确认清理详情缓存中无法解析、object 为空或 object.id 为空的记录？公众号文章缓存不会被清理。",
    );
    if (!confirmed) return;

    ui.btn_cleanup_cache$.setLoading(true);
    ui.btn_cleanup_cache$.disable();
    try {
      const data = await run_request(api_requests.profile_cache_cleanup, {
        db_path,
      });
      detail_fetch_batch = null;
      detail_fetch_results.clear();
      detail_fetched_task_ids.clear();
      detail_fetched_sources.clear();
      state.detail_filter = "all";
      state.detail_page = 1;
      sync_detail_progress();
      touch();

      const removed = Number(data.removed || 0);
      const checked = Number(data.checked || 0);
      notify(
        removed > 0
          ? "已检查 " + checked.toLocaleString() +
              " 条详情缓存，清理 " + removed.toLocaleString() + " 条"
          : "已检查 " + checked.toLocaleString() +
              " 条详情缓存，未发现无效数据",
        false,
      );
      if (Number(state.total || 0) > 0) {
        await load_tasks(state.page);
      }
    } catch (error) {
      notify(error_message(error), true);
    } finally {
      ui.btn_cleanup_cache$.setLoading(false);
      ui.btn_cleanup_cache$.enable();
    }
  }

  async function load_common_dirs() {
    try {
      const data = await run_request(api_requests.common_dirs);
      const saved = localStorage.getItem("gopeed_migration_path");
      const saved_target = localStorage.getItem(
        "gopeed_migration_target_url",
      );
      const saved_target_db = localStorage.getItem(
        "gopeed_migration_target_db",
      );
      ui.path_input$.setValue(
        (is_db_file_path(saved) && saved) || data.default_db_path || "",
        { silence: true },
      );
      ui.target_input$.setValue(
        saved_target || data.target_url || ui.target_input$.value,
        { silence: true },
      );
      ui.target_db_input$.setValue(
        saved_target_db || data.target_db || ui.target_db_input$.value,
        { silence: true },
      );
    } catch (error) {
      notify(error_message(error), true);
    }
  }

  async function execute_migration() {
    ui.btn_migration_execute$.disable();
    ui.btn_migration_execute$.setLoading(true);

    try {
      const target_url = String(ui.target_input$.value || "").trim();
      const target_db = String(ui.target_db_input$.value || "").trim();
      localStorage.setItem("gopeed_migration_target_url", target_url);
      localStorage.setItem("gopeed_migration_target_db", target_db);
      const data = await run_request(api_requests.execute, {
        db_path: String(ui.path_input$.value || "").trim(),
        status: state.status,
        target_url,
        target_db,
      });
      migration_result_.as(data);
      notify("迁移执行完成", false);
      await load_tasks(state.page);
    } catch (error) {
      notify(error_message(error), true);
    } finally {
      ui.btn_migration_execute$.setLoading(false);
      ui.btn_migration_execute$.enable();
    }
  }

  function picker_breadcrumbs() {
    const dir = picker_dir || "/";
    const separator = dir.includes("\\") ? "\\" : "/";
    const parts = dir.split(/[\\/]/).filter(Boolean);
    const crumbs = [{ label: "/", path: "/" }];
    let accumulated = "";
    parts.forEach((part) => {
      accumulated += separator + part;
      crumbs.push({ label: part, path: accumulated });
    });
    return crumbs;
  }

  function picker_entries() {
    const entries = [];
    const parent = dir_name(picker_dir);
    if (parent && parent !== picker_dir) {
      entries.push({
        name: "..",
        path: parent,
        isDir: true,
        is_parent: true,
      });
    }
    return entries.concat(
      picker_files.filter(
        (file) => file && (file.isDir || is_db_file_path(file.path)),
      ),
    );
  }

  function sync_picker_select_button() {
    if (is_db_file_path(picker_selected_path)) {
      ui.btn_picker_select$.enable();
      return;
    }
    ui.btn_picker_select$.disable();
  }

  async function open_picker() {
    const current_path = String(ui.path_input$.value || "").trim();
    picker_dir = is_db_file_path(current_path)
      ? dir_name(current_path)
      : current_path || "/";
    picker_selected_path = "";
    picker_files = [];
    picker_error = "";
    sync_picker_select_button();
    ui.picker_dialog$.show();
    touch();
    await load_picker_dir(picker_dir);
  }

  function close_picker() {
    ui.picker_dialog$.hide();
  }

  async function choose_picker_path(path) {
    const selected = String(path || "").trim();
    if (!is_db_file_path(selected)) {
      notify("请选择 .db 数据库文件", true);
      return;
    }
    ui.path_input$.setValue(selected, { silence: true });
    close_picker();
    reset_detail_filter();
    await load_tasks(1);
  }

  async function load_picker_dir(dir) {
    picker_loading = true;
    picker_error = "";
    touch();
    try {
      const data = await run_request(api_requests.file_list, { dir });
      picker_dir = data.dir || dir;
      picker_selected_path = "";
      picker_files = data.files || [];
      sync_picker_select_button();
    } catch (error) {
      picker_error = error_message(error);
      picker_files = [];
      picker_selected_path = "";
      sync_picker_select_button();
    } finally {
      picker_loading = false;
      touch();
    }
  }

  async function activate_picker_entry(file) {
    if (!file) return;
    if (file.isDir) {
      await load_picker_dir(file.path);
      return;
    }
    await choose_picker_path(file.path);
  }

  function previous_page() {
    if (detail_filter_active()) {
      if (state.detail_page > 1) {
        state.detail_page -= 1;
        touch();
      }
      return;
    }
    if (state.page > 1) {
      load_tasks(state.page - 1);
    }
  }

  function next_page() {
    const table = table_view();
    if (detail_filter_active()) {
      if (state.detail_page < table.total_pages) {
        state.detail_page += 1;
        touch();
      }
      return;
    }
    if (state.page < state.total_pages) {
      load_tasks(state.page + 1);
    }
  }

  function ready() {
    return load_common_dirs();
  }

  function destroy() {
    window.clearTimeout(toast_timer);
    if (confirm_resolve) {
      confirm_resolve(false);
      confirm_resolve = null;
    }
    Object.values(ui).forEach((store) => store.destroy?.());
    [
      version_,
      loading_,
      subtitle_,
      fetch_all_label_,
      migration_result_,
      toast_message_,
      toast_error_,
      profile_title_,
      profile_json_,
      confirm_message_,
    ].forEach((source) => source.destroy?.());
  }

  const table_rows_ = Timeless.computed(version_, () => table_view().rows);
  const table_meta_ = Timeless.computed(version_, () => table_view().meta);
  const table_page_ = Timeless.computed(version_, () => table_view().page);
  const table_page_count_ = Timeless.computed(
    version_,
    () => table_view().total_pages,
  );
  const table_empty_text_ = Timeless.computed(
    version_,
    () => table_view().empty_text,
  );
  const detail_visible_ = Timeless.computed(
    version_,
    () => Boolean(detail_fetch_batch),
  );
  const detail_summary_ = Timeless.computed(version_, detail_summary);
  const picker_loading_ = Timeless.computed(version_, () => picker_loading);
  const picker_error_ = Timeless.computed(version_, () => picker_error);
  const picker_entries_ = Timeless.computed(version_, picker_entries);
  const picker_breadcrumbs_ = Timeless.computed(
    version_,
    picker_breadcrumbs,
  );
  const picker_selected_path_ = Timeless.computed(
    version_,
    () => picker_selected_path,
  );

  return {
    state: {
      version: version_,
      loading: loading_,
      subtitle: subtitle_,
      fetch_all_label: fetch_all_label_,
      migration_result: migration_result_,
      toast_message: toast_message_,
      toast_error: toast_error_,
      profile_title: profile_title_,
      profile_json: profile_json_,
      confirm_message: confirm_message_,
      table_rows: table_rows_,
      table_meta: table_meta_,
      table_page: table_page_,
      table_page_count: table_page_count_,
      table_empty_text: table_empty_text_,
      detail_visible: detail_visible_,
      detail_summary: detail_summary_,
      picker_loading: picker_loading_,
      picker_error: picker_error_,
      picker_entries: picker_entries_,
      picker_breadcrumbs: picker_breadcrumbs_,
      picker_selected_path: picker_selected_path_,
    },
    ui,
    methods: {
      ready,
      destroy,
      status_count(status) {
        const stats = state.stats || {};
        return Number(stats[status === "all" ? "total" : status] || 0);
      },
      status_active(status) {
        return state.status === status;
      },
      detail_filter_count,
      detail_filter_active(filter) {
        return state.detail_filter === filter;
      },
      detail_result_view,
      is_task_detail_fetched,
      detail_fetched_title,
      previous_page,
      next_page,
      activate_picker_entry,
    },
  };
}

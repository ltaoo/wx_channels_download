const MaxRunning = Math.max(1, Number(window.config.maxRunning) || 3);
const DOWNLOAD_PAGE_SIZE_DEFAULT = 12;

const DOWNLOAD_STATUS_COUNT_ITEMS = [
  { key: "total", label: "全部" },
  { key: "running", label: "下载中" },
  { key: "pause", label: "暂停" },
  { key: "wait", label: "等待中" },
  { key: "done", label: "已完成" },
  { key: "error", label: "失败" },
];

const DOWNLOAD_SERVER_STATUS_FILTERS = {
  wait: "0,1",
  running: "2,4",
  pause: "3",
  done: "5",
  error: "6,7",
};

function runtime_flag(value) {
  if (value === true || value === 1) return true;
  if (typeof value !== "string") return false;
  return ["1", "true", "yes", "on"].includes(value.trim().toLowerCase());
}

function is_download_open_external() {
  return (
    runtime_flag(window.config.remoteServerEnabled) ||
    runtime_flag(window.config.inDocker)
  );
}

function normalize_download_status(status) {
  const value = String(status ?? "")
    .trim()
    .toLowerCase();
  if (["0", "waiting", "creating", "preparing", "pending", "queued", "ready"].includes(value)) {
    return "wait";
  }
  if (["1", "2", "4", "running", "downloading", "merging"].includes(value)) {
    return "running";
  }
  if (["3", "pause", "paused"].includes(value)) return "pause";
  if (["5", "done", "finished", "completed", "success"].includes(value)) {
    return "done";
  }
  if (
    [
      "6",
      "7",
      "error",
      "fail",
      "failed",
      "failure",
      "errored",
      "cancelled",
      "canceled",
    ].includes(value)
  ) {
    return "error";
  }
  return value || "wait";
}

function is_download_waiting_status(status) {
  return normalize_download_status(status) === "wait";
}

function format_download_speed(bytes_per_second) {
  const value = Math.max(0, Number(bytes_per_second) || 0);
  const kilobyte = 1024;
  const megabyte = kilobyte * 1024;
  if (value >= megabyte) return `${(value / megabyte).toFixed(1)} MB/s`;
  if (value >= kilobyte) return `${(value / kilobyte).toFixed(1)} KB/s`;
  return `${value.toFixed(1)} B/s`;
}

function format_download_size(bytes) {
  const value = Math.max(0, Number(bytes) || 0);
  if (value === 0) return "0.0KB";
  const units = ["bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  const exponent = Math.min(
    Math.floor(Math.log(value) / Math.log(1024)),
    units.length - 1,
  );
  if (exponent < 1) return `${value.toFixed(1)} ${units[0]}`;
  return `${(value / Math.pow(1024, exponent)).toFixed(1)}${units[exponent]}`;
}

function format_download_percent(record) {
  const progress = record && record.progress;
  const direct = Number(progress);
  if (Number.isFinite(direct)) {
    return Math.min(100, Math.max(0, Math.round(direct * 100) / 100));
  }
  const detail = progress && typeof progress === "object" ? progress : {};
  const detail_percent = Number(detail.percent ?? detail.progress);
  if (Number.isFinite(detail_percent)) {
    return Math.min(100, Math.max(0, Math.round(detail_percent * 100) / 100));
  }
  const total = Number(
    (record && record.size) || detail.total || detail.size || 0,
  );
  const downloaded = Number(
    (record && record.downloaded) || detail.downloaded || 0,
  );
  if (total <= 0) return 0;
  return Math.min(
    100,
    Math.max(0, Math.round(((downloaded * 100) / total) * 100) / 100),
  );
}

function get_download_status_count(counts, item) {
  if (!counts || !item) return 0;
  return Number(counts[item.key]) || 0;
}

function empty_status_counts() {
  return {
    total: 0,
    running: 0,
    pause: 0,
    wait: 0,
    done: 0,
    error: 0,
  };
}

function server_status_filter(status) {
  return DOWNLOAD_SERVER_STATUS_FILTERS[status] || "";
}

function normalize_server_status_counts(source) {
  const counts = empty_status_counts();
  Object.entries(source || {}).forEach(([status, count]) => {
    const value = Number(count) || 0;
    const key = String(status).trim().toLowerCase();
    if (key === "total") counts.total += value;
    else if (["0", "1", "waiting", "preparing", "wait"].includes(key)) {
      counts.wait += value;
    } else if (["2", "4", "downloading", "merging", "running"].includes(key)) {
      counts.running += value;
    } else if (["3", "paused", "pause"].includes(key)) {
      counts.pause += value;
    } else if (["5", "finished", "completed", "done"].includes(key)) {
      counts.done += value;
    } else if (["6", "7", "failed", "cancelled", "error"].includes(key)) {
      counts.error += value;
    }
  });
  if (!Object.prototype.hasOwnProperty.call(source || {}, "total")) {
    counts.total =
      counts.running +
      counts.pause +
      counts.wait +
      counts.done +
      counts.error;
  }
  return counts;
}

function task_identifier(value) {
  if (!value || typeof value !== "object") return value;
  if (value.state && value.state.id) return value.state.id.value;
  if (value.id && typeof value.id === "object" && "value" in value.id) {
    return value.id.value;
  }
  return value.id ?? value.task_id;
}

function DownloadV2Model(props = {}) {
  const {
    downloader = window.dl$,
    fixedListHeight: fixed_list_height = false,
    itemHeight: item_height = 82,
    listBuffer: list_buffer = 10,
    listHeight: list_height = 380,
  } = props;
  if (!downloader || typeof downloader.refresh !== "function") {
    throw new TypeError("DownloadV2Model requires the global dl$ instance");
  }

  const tasks_ = refarr([]);
  const task_count_ = ref(0);
  const filtered_task_count_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(DOWNLOAD_PAGE_SIZE_DEFAULT);
  const running_count_ = ref(0);
  const status_counts_ = refobj(empty_status_counts());
  const active_status_ = ref("total");
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  const list_render_enabled_ = ref(true);
  const selected_task_ids_ = refarr([]);
  const delete_task_ = ref(null);
  const delete_task_ids_ = refarr([]);
  const delete_delete_files_ = ref(false);
  const deleting_task_ = ref(false);
  const clearing_tasks_ = ref(false);
  const create_task_text_ = ref("");
  const create_task_filename_ = ref("");
  const create_platform_text_ = ref("");
  const create_platform_json_ = ref("");
  const create_platform_download_dir_ = ref("");
  const create_platform_filename_ = ref("");
  const create_platform_download_cover_ = ref(false);
  const creating_task_ = ref(false);
  const create_task_preview_ = ref(null);
  const create_platform_preview_ = ref(null);
  const preview_task_id_ = ref("");
  const overwrite_ = refobj({ value: "overwrite" });
  const overwrite_apply_all_ = ref(false);
  const overwrite_processing_ = ref(false);
  const overwrite_conflict_ = refobj({ index: 0, total: 0, name: "" });
  const disposables = [];
  let list_view_element = null;
  let selection_anchor_task_id = null;
  let pending_create_object = null;
  let started = false;
  let disposed = false;
  let request_sequence = 0;
  let scroll_top = 0;

  const selected_task_count_ = computed(
    selected_task_ids_,
    (ids) => (ids || []).length,
  );
  const pending_delete_task_count_ = combine(
    { ids: delete_task_ids_, task: delete_task_ },
    (state) => {
      if (state.ids && state.ids.length) return state.ids.length;
      return state.task ? 1 : 0;
    },
  );
  const page_count_ = combine(
    { total: filtered_task_count_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const range_text_ = combine(
    {
      total: filtered_task_count_,
      page: page_,
      pageSize: page_size_,
      count: computed(tasks_, (tasks) => tasks.length),
    },
    (state) => {
      if (!state.total || !state.count) {
        return `共 ${state.total || 0} 条`;
      }
      const start = (state.page - 1) * state.pageSize + 1;
      return `第 ${start}-${start + state.count - 1} 条，共 ${state.total} 条`;
    },
  );
  const loaded_task_selection_ = combine(
    {
      tasks: tasks_,
      selected_ids: selected_task_ids_,
    },
    (data) => {
      const task_ids = [];
      (data.tasks || []).forEach((task) => {
        const id = task_identifier(task);
        if (
          !task ||
          task.__placeholder ||
          id === undefined ||
          id === null ||
          id === ""
        ) {
          return;
        }
        if (!task_ids.some((task_id) => task_id === id)) {
          task_ids.push(id);
        }
      });
      const selected_ids = data.selected_ids || [];
      const selected = task_ids.filter((id) => {
        return selected_ids.some((selected_id) => selected_id === id);
      }).length;
      return {
        total: task_ids.length,
        selected,
        checked: task_ids.length > 0 && selected === task_ids.length,
        indeterminate: selected > 0 && selected < task_ids.length,
      };
    },
  );
  const list_status_ = combine(
    {
      initial: initial_,
      error: error_,
      tasks: tasks_,
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error) return "error";
      return state.tasks.length > 0 ? "normal" : "empty";
    },
  );

  const methods = {};
  const ui = {
    btn_refresh_tasks$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        return methods.refreshTasks();
      },
    }),
    btn_start_all_tasks$: new Timeless.vm.ButtonCore({
      variant: "primary",
      onClick() {
        return methods.startAllTasks();
      },
    }),
    btn_pause_all_tasks$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        return methods.pauseAllTasks();
      },
    }),
    btn_clear_tasks$: new Timeless.vm.ButtonCore({
      variant: "danger",
      onClick() {
        return methods.requestClearTasks(false);
      },
    }),
    btn_delete_selected_tasks$: new Timeless.vm.ButtonCore({
      variant: "danger",
      onClick() {
        return methods.requestDeleteSelectedTasks(false);
      },
    }),
    input_create_task_url$: new Timeless.vm.InputCore({
      defaultValue: create_task_text_.value,
      placeholder: "请输入下载地址，例如 https://example.com/file.mp4",
      type: "url",
      allowClear: true,
      autoFocus: true,
      onChange(value) {
        methods.setCreateTaskText(value);
      },
    }),
    input_create_task_filename$: new Timeless.vm.InputCore({
      defaultValue: create_task_filename_.value,
      placeholder: "自动识别，可手动修改",
      allowClear: true,
      onChange(value) {
        create_task_filename_.as(value || "");
      },
    }),
    input_create_platform$: new Timeless.vm.InputCore({
      defaultValue: create_platform_text_.value,
      placeholder: "如 wx_channels、bilibili",
      allowClear: true,
      autoFocus: true,
      onChange(value) {
        create_platform_text_.as(value || "");
      },
    }),
    input_create_platform_json$: new Timeless.vm.InputCore({
      defaultValue: create_platform_json_.value,
      placeholder: '平台内容原始 JSON，如 {"feed_id":"xxx"}',
      allowClear: false,
      onChange(value) {
        create_platform_json_.as(value || "");
      },
    }),
    input_create_platform_download_dir$: new Timeless.vm.InputCore({
      defaultValue: create_platform_download_dir_.value,
      placeholder: "留空则使用默认下载目录",
      allowClear: true,
      onChange(value) {
        create_platform_download_dir_.as(value || "");
      },
    }),
    input_create_platform_filename$: new Timeless.vm.InputCore({
      defaultValue: create_platform_filename_.value,
      placeholder: "留空则自动命名",
      allowClear: true,
      onChange(value) {
        create_platform_filename_.as(value || "");
      },
    }),
    createTaskDialog$: new Timeless.vm.DialogCore({
      closeable: true,
      onOk() {
        return methods.confirmCreateTask();
      },
    }),
    createPlatformTaskDialog$: new Timeless.vm.DialogCore({
      closeable: true,
      onOk() {
        return methods.confirmCreatePlatformTask();
      },
    }),
    createTaskPreviewDialog$: new Timeless.vm.DialogCore({
      closeable: true,
      onOk() {
        return methods.confirmCreateTaskFromPreview();
      },
    }),
    createPlatformTaskPreviewDialog$: new Timeless.vm.DialogCore({
      closeable: true,
      onOk() {
        return methods.confirmCreatePlatformTaskFromPreview();
      },
    }),
    taskPreviewDrawer$: new Timeless.vm.DialogCore({
      title: "任务详情",
      closeable: true,
      footer: false,
    }),
    deleteConfirmDialog$: new Timeless.vm.DialogCore({
      onOk() {
        return methods.confirmDeleteTask();
      },
    }),
    clearConfirmDialog$: new Timeless.vm.DialogCore({
      onOk() {
        return methods.confirmClearTasks();
      },
    }),
    overwriteConfirmDialog$: new Timeless.vm.DialogCore({
      onOk() {
        return methods.confirmOverwriteDownloadConflict();
      },
    }),
    singleOverwriteConfirmDialog$: new Timeless.vm.DialogCore({
      onOk() {
        return methods.confirmOverwriteDownloadConflict();
      },
    }),
    batchOverwriteConfirmDialog$: new Timeless.vm.DialogCore({
      onOk() {
        return methods.confirmOverwriteDownloadConflict();
      },
    }),
  };

  function bind_ui_input(input, source) {
    const unlisten = source.subscribe({
      onChange(value) {
        if (input.value !== value) {
          input.setValue(value, { silence: true });
        }
      },
    });
    if (typeof unlisten === "function") {
      disposables.push(unlisten);
    }
  }

  bind_ui_input(ui.input_create_task_url$, create_task_text_);
  bind_ui_input(ui.input_create_task_filename$, create_task_filename_);
  bind_ui_input(ui.input_create_platform$, create_platform_text_);
  bind_ui_input(ui.input_create_platform_json$, create_platform_json_);
  bind_ui_input(
    ui.input_create_platform_download_dir$,
    create_platform_download_dir_,
  );
  bind_ui_input(
    ui.input_create_platform_filename$,
    create_platform_filename_,
  );

  function page_request_options(target_page) {
    const options = {
      all: false,
      page: Math.max(1, Number(target_page) || 1),
      page_size: page_size_.value,
    };
    const status = server_status_filter(active_status_.value);
    if (status) options.status = status;
    return options;
  }

  function apply_list_meta(meta) {
    if (!meta || typeof meta !== "object") return;
    const counts = normalize_server_status_counts(meta.stats);
    const response_total = Math.max(0, Number(meta.total) || 0);
    const filtered_total =
      response_total || Math.max(0, Number(counts[active_status_.value]) || 0);
    const response_page = Math.max(1, Number(meta.page) || page_.value || 1);
    const response_page_size = Math.max(
      1,
      Number(meta.page_size) || page_size_.value,
    );
    if (!counts.total && active_status_.value === "total") {
      counts.total = response_total;
    }
    task_count_.as(counts.total);
    filtered_task_count_.as(filtered_total);
    page_.as(response_page);
    page_size_.as(response_page_size);
    running_count_.as(counts.running);
    status_counts_.as(counts);
  }

  function rebuild_derived_state() {
    if (disposed) return;
    const domain_tasks = (downloader.task_list.value || []).filter((task$) => {
      const id = task_identifier(task$);
      return id !== undefined && id !== null && id !== "";
    });

    const valid_ids = domain_tasks.map(task_identifier);
    const selected_ids = (selected_task_ids_.value || []).filter((id) => {
      return valid_ids.some((valid_id) => valid_id === id);
    });
    if (selected_ids.length !== selected_task_ids_.value.length) {
      selected_task_ids_.as(selected_ids);
    }
    // Keep DownloadTaskModel instances intact. Their `state.*` refs drive row
    // updates without rebuilding plain records for every progress event.
    tasks_.assign(domain_tasks);
  }

  async function load_page(target_page = page_.value) {
    const requested_page = Math.max(1, Number(target_page) || 1);
    const sequence = ++request_sequence;
    const options = page_request_options(requested_page);
    loading_.as(true);
    try {
      let meta;
      if (typeof downloader.loadPage === "function") {
        meta = await downloader.loadPage(options);
      } else if (
        downloader.handler &&
        typeof downloader.handler.fetch_task_page === "function" &&
        typeof downloader.handler.replace_server_tasks === "function"
      ) {
        const request_options = { ...options };
        delete request_options.all;
        const data = await downloader.handler.fetch_task_page(request_options);
        if (sequence !== request_sequence || disposed) return null;
        downloader.handler.replace_server_tasks(data.records || []);
        meta = data;
      } else {
        await downloader.refresh(options);
        meta = downloader.list_meta && downloader.list_meta.value;
      }
      if (sequence !== request_sequence || disposed) return null;
      const meta_counts = normalize_server_status_counts(meta && meta.stats);
      const total =
        Math.max(0, Number(meta && meta.total) || 0) ||
        Math.max(0, Number(meta_counts[active_status_.value]) || 0);
      const page_size = Math.max(
        1,
        Number(meta && meta.page_size) || page_size_.value,
      );
      const last_page = Math.max(1, Math.ceil(total / page_size));
      if (requested_page > last_page) {
        return load_page(last_page);
      }
      sync_domain_tasks();
      apply_list_meta(meta);
      error_.as("");
      return tasks_;
    } catch (error) {
      if (sequence === request_sequence) {
        error_.as(
          (error && error.message) || String(error || "获取下载任务失败"),
        );
        report_error(error, "获取下载任务失败");
      }
      return null;
    } finally {
      if (sequence === request_sequence) {
        loading_.as(false);
        initial_.as(false);
      }
    }
  }

  function sync_domain_tasks() {
    rebuild_derived_state();
  }

  function domain_task(value) {
    if (value && value.state && value.state.id) return value;
    return downloader.get(task_identifier(value));
  }

  function report_error(error, fallback) {
    const message = (error && error.message) || String(error || fallback);
    DLUtils.error({ msg: message || fallback });
    return error;
  }

  async function run_task_action(value, action, fallback, refresh_page = true) {
    const task$ = domain_task(value);
    if (
      !task$ ||
      !task$.methods ||
      typeof task$.methods[action] !== "function"
    ) {
      report_error(null, "下载任务不存在");
      return null;
    }
    try {
      const result = await task$.methods[action]();
      if (refresh_page) await load_page(page_.value);
      return result;
    } catch (error) {
      report_error(error, fallback);
      return null;
    }
  }

  function set_status_filter(status) {
    const value = DOWNLOAD_STATUS_COUNT_ITEMS.some((item) => item.key === status)
      ? status
      : "total";
    active_status_.as(value);
    page_.as(1);
    reset_list_scroll();
    return load_page(1);
  }

  function reset_list_scroll() {
    scroll_top = 0;
    if (list_view_element) list_view_element.scrollTop = 0;
  }

  function previous_page() {
    if (page_.value <= 1 || loading_.value) return null;
    const target_page = page_.value - 1;
    reset_list_scroll();
    return load_page(target_page);
  }

  function next_page() {
    if (page_.value >= page_count_.value || loading_.value) return null;
    const target_page = page_.value + 1;
    reset_list_scroll();
    return load_page(target_page);
  }

  function refresh_tasks() {
    page_.as(1);
    reset_list_scroll();
    return load_page(1);
  }

  function start_task(task) {
    return run_task_action(task, "start", "开始下载任务失败");
  }

  function pause_task(task) {
    return run_task_action(task, "pause", "暂停下载任务失败");
  }

  function resume_task(task) {
    return run_task_action(task, "resume", "继续下载任务失败");
  }

  function retry_task(task) {
    return run_task_action(task, "retry", "重试下载任务失败");
  }

  function open_task(task) {
    return run_task_action(task, "open", "打开下载文件失败", false);
  }

  async function start_all_tasks() {
    if (running_count_.value >= MaxRunning) {
      DLUtils.warning({ msg: `已达到最大同时下载任务数（${MaxRunning}）` });
      return null;
    }
    try {
      return await downloader.startAll({ status: active_status_.value });
    } catch (error) {
      report_error(error, "开始全部下载任务失败");
      return null;
    }
  }

  async function pause_all_tasks() {
    try {
      return await downloader.pauseAll({ status: active_status_.value });
    } catch (error) {
      report_error(error, "暂停全部下载任务失败");
      return null;
    }
  }

  function visible_records() {
    return tasks_.value || [];
  }

  function visible_task_ids() {
    return visible_records().map(task_identifier).filter((id) => id !== undefined);
  }

  function request_delete_task(task) {
    if (!domain_task(task)) {
      report_error(null, "下载任务不存在");
      return;
    }
    delete_task_.as(task);
    delete_task_ids_.as([]);
    delete_delete_files_.as(false);
    ui.deleteConfirmDialog$.show();
  }

  function request_delete_selected_tasks() {
    const ids = selected_task_ids_.value || [];
    if (!ids.length) {
      DLUtils.warning({ msg: "请选择要删除的下载任务" });
      return;
    }
    delete_task_.as(null);
    delete_task_ids_.as([...ids]);
    delete_delete_files_.as(false);
    ui.deleteConfirmDialog$.show();
  }

  async function confirm_delete_task() {
    if (deleting_task_.value) return;
    const direct_task = delete_task_.value;
    const ids = delete_task_ids_.value || [];
    const targets = direct_task
      ? [domain_task(direct_task)]
      : ids.map((id) => downloader.get(id));
    const tasks = targets.filter(Boolean);
    if (!tasks.length) {
      ui.deleteConfirmDialog$.hide();
      return;
    }
    deleting_task_.as(true);
    const errors = [];
    try {
      for (const task$ of tasks) {
        try {
          await task$.methods.delete({
            deleteFiles: delete_delete_files_.value,
          });
        } catch (error) {
          errors.push(error);
        }
      }
      if (!errors.length) {
        delete_task_.as(null);
        delete_task_ids_.as([]);
        selected_task_ids_.as([]);
        ui.deleteConfirmDialog$.hide();
      } else {
        report_error(errors[0], "删除下载任务失败");
      }
    } finally {
      deleting_task_.as(false);
      await load_page(page_.value);
    }
  }

  function request_clear_tasks(delete_files = false) {
    delete_delete_files_.as(Boolean(delete_files));
    ui.clearConfirmDialog$.show();
  }

  async function confirm_clear_tasks() {
    if (clearing_tasks_.value) return;
    clearing_tasks_.as(true);
    try {
      await downloader.clear({ deleteFiles: delete_delete_files_.value });
      selected_task_ids_.as([]);
      ui.clearConfirmDialog$.hide();
      await load_page(1);
    } catch (error) {
      report_error(error, "清空下载任务失败");
    } finally {
      clearing_tasks_.as(false);
    }
  }

  function set_loaded_tasks_selected(selected) {
    const ids = visible_task_ids();
    if (selected) {
      const selected_ids = [...(selected_task_ids_.value || [])];
      ids.forEach((id) => {
        if (!selected_ids.some((selected_id) => selected_id === id)) {
          selected_ids.push(id);
        }
      });
      selected_task_ids_.as(selected_ids);
      selection_anchor_task_id = ids.length ? ids[ids.length - 1] : null;
      return;
    }
    const visible_ids = new Set(ids);
    selected_task_ids_.as(
      (selected_task_ids_.value || []).filter((id) => !visible_ids.has(id)),
    );
    selection_anchor_task_id = null;
  }

  function toggle_loaded_tasks_selected() {
    set_loaded_tasks_selected(!loaded_task_selection_.value.checked);
  }

  function task_selection_state(task) {
    const task_id = task_identifier(task);
    return computed(selected_task_ids_, (ids) => ({
      checked: (ids || []).some((selected_id) => selected_id === task_id),
      indeterminate: false,
    }));
  }

  function toggle_task_selected(task, options = {}) {
    const id = task_identifier(task);
    if (id === undefined || id === null || id === "") return;
    let ids = [...(selected_task_ids_.value || [])];
    if (options.shiftKey && selection_anchor_task_id !== null) {
      const visible_ids = visible_task_ids();
      const from = visible_ids.findIndex((value) => value === selection_anchor_task_id);
      const to = visible_ids.findIndex((value) => value === id);
      if (from >= 0 && to >= 0) {
        const start = Math.min(from, to);
        const end = Math.max(from, to);
        visible_ids.slice(start, end + 1).forEach((range_id) => {
          if (!ids.some((selected_id) => selected_id === range_id)) ids.push(range_id);
        });
        selected_task_ids_.as(ids);
        selection_anchor_task_id = id;
        return;
      }
    }
    if (ids.some((selected_id) => selected_id === id)) {
      ids = ids.filter((selected_id) => selected_id !== id);
    } else {
      ids.push(id);
    }
    selected_task_ids_.as(ids);
    selection_anchor_task_id = id;
  }

  function request_create_task() {
    create_task_text_.as("");
    create_task_filename_.as("");
    create_task_preview_.as(null);
    ui.createTaskDialog$.show();
  }

  function request_create_platform_task() {
    create_platform_text_.as("");
    create_platform_json_.as("");
    create_platform_download_dir_.as("");
    create_platform_filename_.as("");
    create_platform_download_cover_.as(false);
    create_platform_preview_.as(null);
    ui.createPlatformTaskDialog$.show();
  }

  function request_task_preview(task) {
    const task_id = task_identifier(task);
    if (task_id === undefined || task_id === null || task_id === "") {
      DLUtils.error({ msg: "无法打开详情：下载任务 ID 为空" });
      return;
    }
    preview_task_id_.as(String(task_id));
    ui.taskPreviewDrawer$.show();
  }

  function extract_filename(value) {
    try {
      const url = new URL(String(value || ""));
      const filename = url.pathname.split("/").filter(Boolean).pop() || "";
      return decodeURIComponent(filename);
    } catch {
      return "";
    }
  }

  function set_create_task_text(value) {
    create_task_text_.as(value || "");
    const filename = extract_filename(value);
    if (filename) create_task_filename_.as(filename);
  }

  async function confirm_create_task() {
    if (creating_task_.value) return;
    const url = String(create_task_text_.value || "").trim();
    if (!url) {
      DLUtils.warning({ msg: "请输入下载地址" });
      return;
    }
    creating_task_.as(true);
    try {
      const preview = await downloader.prepare({
        url,
        filename: create_task_filename_.value || "",
      });
      create_task_preview_.as(preview);
      ui.createTaskDialog$.hide();
      ui.createTaskPreviewDialog$.show();
    } catch (error) {
      report_error(error, "准备下载任务失败");
    } finally {
      creating_task_.as(false);
    }
  }

  function normalize_platform_preview(preview, fallback) {
    if (!preview || typeof preview !== "object") return preview;
    if (Array.isArray(preview.resources)) return preview;
    const task_record =
      preview.Task && typeof preview.Task === "object" ? preview.Task : {};
    const source_resources = Array.isArray(preview.Resources) ? preview.Resources : [];
    const resources = source_resources.map((item, index) => {
      const resource = item && item.Resource ? item.Resource : item || {};
      return {
        index,
        name: resource.name || resource.Name || `资源 ${index + 1}`,
        kind: resource.kind || resource.Kind || "",
        type: resource.type || resource.Type || "",
        endpoints:
          (item && (item.Endpoints || item.endpoints)) ||
          resource.endpoints ||
          [],
      };
    });
    return {
      platform:
        task_record.platform_id ||
        task_record.PlatformID ||
        fallback.platform ||
        "",
      task_name:
        task_record.name || task_record.Name || fallback.filename || "",
      download_dir: fallback.download_dir || "",
      resource_type: resources[0]
        ? resources[0].kind || resources[0].type || ""
        : "",
      resources,
      resource_count: resources.length,
      endpoint_count: resources.reduce((count, resource) => {
        return count + (Array.isArray(resource.endpoints) ? resource.endpoints.length : 0);
      }, 0),
      content: preview.Content || preview.content || null,
      account: preview.Account || preview.account || null,
      download_info: preview,
    };
  }

  async function confirm_create_platform_task() {
    if (creating_task_.value) return;
    const platform = String(create_platform_text_.value || "").trim();
    if (!platform) {
      DLUtils.warning({ msg: "请输入平台名称" });
      return;
    }
    let content = {};
    try {
      content = JSON.parse(String(create_platform_json_.value || "{}").trim() || "{}");
    } catch (error) {
      DLUtils.warning({ msg: `内容 JSON 格式错误：${error.message}` });
      return;
    }
    creating_task_.as(true);
    try {
      const prepare_object = {
        platform,
        content,
        download_dir: create_platform_download_dir_.value || "",
        filename: create_platform_filename_.value || "",
        config: {
          download_cover: create_platform_download_cover_.value,
        },
      };
      const preview = await downloader.prepare(prepare_object);
      create_platform_preview_.as(
        normalize_platform_preview(preview, prepare_object),
      );
      ui.createPlatformTaskDialog$.hide();
      ui.createPlatformTaskPreviewDialog$.show();
    } catch (error) {
      report_error(error, "准备平台下载任务失败");
    } finally {
      creating_task_.as(false);
    }
  }

  function duplicate_error(error) {
    const code = Number(
      error &&
        (error.code ||
          error.status ||
          (error.item && error.item.code) ||
          (error.data && error.data.code)),
    );
    if (code === 409) return true;
    return /already exists|duplicate|已存在|重复/i.test(
      (error && error.message) || "",
    );
  }

  function create_object_name(object, error) {
    const data = (error && error.data) || {};
    const content = object && object.content;
    return (
      data.name ||
      (object && (object.filename || object.name || object.title)) ||
      (content && (content.title || content.name || content.id)) ||
      "相同下载内容"
    );
  }

  function close_overwrite_dialogs() {
    ui.overwriteConfirmDialog$.hide();
    ui.singleOverwriteConfirmDialog$.hide();
    ui.batchOverwriteConfirmDialog$.hide();
  }

  function show_overwrite_dialog(object, error) {
    pending_create_object = object;
    overwrite_.as({ value: "overwrite" });
    overwrite_apply_all_.as(false);
    overwrite_conflict_.as({
      index: 1,
      total: 1,
      name: create_object_name(object, error),
    });
    ui.createTaskPreviewDialog$.hide();
    ui.createPlatformTaskPreviewDialog$.hide();
    ui.overwriteConfirmDialog$.show();
  }

  async function create_domain_task(object, success_message, options = {}) {
    try {
      const task$ =
        object &&
        object.platform &&
        Object.prototype.hasOwnProperty.call(object, "content")
          ? await downloader.create(object.content, object)
          : await downloader.create(object);
      await load_page(page_.value);
      if (options.hideDialog) options.hideDialog.hide();
      DLUtils.toast(success_message);
      return task$;
    } catch (error) {
      if (!options.overwriteRetry && duplicate_error(error)) {
        show_overwrite_dialog(object, error);
        return null;
      }
      report_error(error, "创建下载任务失败");
      return null;
    }
  }

  async function confirm_create_task_from_preview() {
    if (creating_task_.value || !create_task_preview_.value) return;
    creating_task_.as(true);
    try {
      return await create_domain_task(
        {
          url: create_task_text_.value || create_task_preview_.value.url || "",
          filename:
            create_task_filename_.value ||
            create_task_preview_.value.task_name ||
            "",
        },
        "下载任务创建成功",
        { hideDialog: ui.createTaskPreviewDialog$ },
      );
    } finally {
      creating_task_.as(false);
    }
  }

  async function confirm_create_platform_task_from_preview() {
    if (creating_task_.value || !create_platform_preview_.value) return;
    let content = {};
    try {
      content = JSON.parse(create_platform_json_.value || "{}");
    } catch {
      content = {};
    }
    const object = {
      platform:
        create_platform_text_.value || create_platform_preview_.value.platform || "",
      content,
      download_dir: create_platform_download_dir_.value || "",
      filename: create_platform_filename_.value || "",
      config: {
        download_cover: create_platform_download_cover_.value,
      },
    };
    creating_task_.as(true);
    try {
      return await create_domain_task(object, "平台下载任务创建成功", {
        hideDialog: ui.createPlatformTaskPreviewDialog$,
      });
    } finally {
      creating_task_.as(false);
    }
  }

  function set_overwrite_action(action) {
    if (["overwrite", "skip", "duplicate"].includes(action)) {
      overwrite_.as({ value: action });
    }
  }

  function toggle_overwrite_apply_all() {
    overwrite_apply_all_.as(!overwrite_apply_all_.value);
  }

  async function confirm_overwrite_download_conflict() {
    if (overwrite_processing_.value || !pending_create_object) return;
    const action = overwrite_.value && overwrite_.value.value;
    if (!["overwrite", "skip", "duplicate"].includes(action)) return;
    if (action === "skip") {
      pending_create_object = null;
      close_overwrite_dialogs();
      return;
    }
    overwrite_processing_.as(true);
    try {
      const object = {
        ...pending_create_object,
        config: {
          ...(pending_create_object.config || {}),
          overwrite: action === "overwrite",
          duplicate: action === "duplicate",
        },
      };
      const task$ = await create_domain_task(object, "下载任务创建成功", {
        overwriteRetry: true,
      });
      if (task$) {
        pending_create_object = null;
        close_overwrite_dialogs();
      }
    } finally {
      overwrite_processing_.as(false);
    }
  }

  function handle_click_delete_files() {
    delete_delete_files_.as(!delete_delete_files_.value);
  }

  function set_list_view_element(element) {
    list_view_element = element || null;
  }

  function handle_list_view_scroll(position) {
    scroll_top = Number(position && (position.scrollTop ?? position.top ?? position)) || 0;
  }

  function is_placeholder_task() {
    return false;
  }

  function ensure_task_page_for_index() {}

  Object.assign(methods, {
    setStatusFilter: set_status_filter,
    previousPage: previous_page,
    nextPage: next_page,
    refreshTasks: refresh_tasks,
    startTask: start_task,
    pauseTask: pause_task,
    resumeTask: resume_task,
    retryTask: retry_task,
    openTask: open_task,
    startAllTasks: start_all_tasks,
    pauseAllTasks: pause_all_tasks,
    requestDeleteTask: request_delete_task,
    requestDeleteSelectedTasks: request_delete_selected_tasks,
    confirmDeleteTask: confirm_delete_task,
    requestClearTasks: request_clear_tasks,
    confirmClearTasks: confirm_clear_tasks,
    setLoadedTasksSelected: set_loaded_tasks_selected,
    toggleLoadedTasksSelected: toggle_loaded_tasks_selected,
    taskSelectionState: task_selection_state,
    toggleTaskSelected: toggle_task_selected,
    requestCreateTask: request_create_task,
    requestCreatePlatformTask: request_create_platform_task,
    requestTaskPreview: request_task_preview,
    setCreateTaskText: set_create_task_text,
    confirmCreateTask: confirm_create_task,
    confirmCreatePlatformTask: confirm_create_platform_task,
    confirmCreateTaskFromPreview: confirm_create_task_from_preview,
    confirmCreatePlatformTaskFromPreview:
      confirm_create_platform_task_from_preview,
    setOverwriteAction: set_overwrite_action,
    toggleOverwriteApplyAll: toggle_overwrite_apply_all,
    confirmOverwriteDownloadConflict: confirm_overwrite_download_conflict,
    handleClickCheckboxConfirmDeleteFiles: handle_click_delete_files,
    setListViewElement: set_list_view_element,
    handleListViewScroll: handle_list_view_scroll,
    isPlaceholderTask: is_placeholder_task,
    ensureTaskPageForIndex: ensure_task_page_for_index,
  });

  const state = {
    tasks: tasks_,
    task_count: task_count_,
    total: filtered_task_count_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    list_render_enabled: list_render_enabled_,
    running_count: running_count_,
    delete_task: delete_task_,
    delete_task_ids: delete_task_ids_,
    pending_delete_task_count: pending_delete_task_count_,
    delete_delete_files: delete_delete_files_,
    deleting_task: deleting_task_,
    selected_task_ids: selected_task_ids_,
    selected_task_count: selected_task_count_,
    loaded_task_selection: loaded_task_selection_,
    clearing_tasks: clearing_tasks_,
    create_task_text: create_task_text_,
    create_task_filename: create_task_filename_,
    create_platform_text: create_platform_text_,
    create_platform_json: create_platform_json_,
    create_platform_download_dir: create_platform_download_dir_,
    create_platform_filename: create_platform_filename_,
    create_platform_download_cover: create_platform_download_cover_,
    creating_task: creating_task_,
    create_task_preview: create_task_preview_,
    create_platform_preview: create_platform_preview_,
    preview_task_id: preview_task_id_,
    websocket_connected: downloader.websocket_connected,
    websocket_connecting: downloader.websocket_connecting,
    status_counts: status_counts_,
    active_status: active_status_,
    initial: initial_,
    status: list_status_,
    loading: loading_,
    error: error_,
    overwrite: overwrite_,
    overwrite_apply_all: overwrite_apply_all_,
    overwrite_processing: overwrite_processing_,
    overwrite_conflict: overwrite_conflict_,
    fixed_list_height: Boolean(fixed_list_height),
    list_item_height: Math.max(1, Number(item_height) || 82),
    list_gutter: 0,
    list_height: Math.max(1, Number(list_height) || 380),
    list_size: 50,
    list_buffer: Math.max(0, Number(list_buffer) || 10),
    get scrollTop() {
      return scroll_top;
    },
  };
  const handler = {
    applyListMeta: apply_list_meta,
    rebuildDerivedState: rebuild_derived_state,
    syncDomainTasks: sync_domain_tasks,
  };

  async function ready() {
    if (started) return load_page(page_.value);
    started = true;
    const unlisten = downloader.task_list.subscribe({
      onChange() {
        sync_domain_tasks();
      },
    });
    if (typeof unlisten === "function") disposables.push(unlisten);
    if (downloader.list_meta && typeof downloader.list_meta.subscribe === "function") {
      const unlisten_meta = downloader.list_meta.subscribe({
        onChange(meta) {
          apply_list_meta(meta);
        },
      });
      if (typeof unlisten_meta === "function") disposables.push(unlisten_meta);
    }
    sync_domain_tasks();
    try {
      const results = await Promise.allSettled([
        load_page(page_.value),
        typeof downloader.connect === "function"
          ? downloader.connect()
          : Promise.resolve(false),
      ]);
      if (results[0].status === "rejected") throw results[0].reason;
      return {
        tasks: downloader.task_list,
        connected: !!(
          downloader.websocket_connected &&
          downloader.websocket_connected.value
        ),
        results,
      };
    } catch (error) {
      report_error(error, "下载服务初始化失败");
      return null;
    }
  }

  function clean() {
    if (disposed) return;
    disposed = true;
    request_sequence += 1;
    loading_.as(false);
    disposables.splice(0).forEach((unlisten) => {
      if (typeof unlisten === "function") unlisten();
    });
    Object.values(ui).forEach((store) => {
      if (typeof store.destroy === "function") store.destroy();
    });
  }

  Object.assign(methods, {
    ready,
    clean,
    loadPage: load_page,
    applyListMeta: handler.applyListMeta,
    rebuildDerivedState: handler.rebuildDerivedState,
    syncDomainTasks: handler.syncDomainTasks,
  });

  return { state, ui, methods };
}

export {
  DOWNLOAD_STATUS_COUNT_ITEMS,
  DownloadV2Model,
  MaxRunning,
  format_download_percent,
  format_download_size,
  format_download_speed,
  get_download_status_count,
  is_download_open_external,
  is_download_waiting_status,
  normalize_download_status,
};

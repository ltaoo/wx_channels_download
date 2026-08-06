/// <reference path="../utils.js" />
/**
 * @file Download manager data model, formatting helpers, and ViewModel
 */
var APIHostname = WXEnv.apiOrigin;
var DownloadHostname = WXEnv.downloadOrigin;

const http_client = new Timeless.HttpClientCore({
  headers: { "Content-Type": "application/json" },
  hostname: APIHostname,
});
Timeless.web.provide_http_client(http_client);
const request = Timeless.request_factory({
  headers: { "Content-Type": "application/json" },
  process(r) {
    if (r.error) {
      return Timeless.Result.Err(r.error);
    }
    const { code, msg, data } = r.data;
    if (code !== 0) {
      return Timeless.Result.Err(msg, code, data);
    }
    return Timeless.Result.Ok(data);
  },
});

function format_download_speed(bps) {
  const kb = 1024,
    mb = kb * 1024;
  if (!bps) return "0 B/s";
  if (bps >= mb) return (bps / mb).toFixed(2) + " MB/s";
  if (bps >= kb) return (bps / kb).toFixed(2) + " KB/s";
  return bps + " B/s";
}
function format_download_percent(t) {
  const direct = Number(t && t.progress);
  if (Number.isFinite(direct)) {
    return Math.min(100, Math.max(0, Math.round(direct * 100) / 100));
  }
  const detail =
    t && t.progress && typeof t.progress === "object" ? t.progress : {};
  const total = Number(t.size || detail.total || detail.size || 0);
  const downloaded = Number(t.downloaded || detail.downloaded || 0);
  if (total <= 0) {
    return 0;
  }
  return Math.min(
    100,
    Math.max(0, Math.round(((downloaded * 100) / total) * 100) / 100),
  );
}
function get_name_of_download_task(t) {
  if (t.meta && t.meta.opts && t.meta.opts.name) {
    return t.meta.opts.name;
  }
  if (t.meta && t.meta.res) {
    if (t.meta.res.name) return t.meta.res.name;
    if (t.meta.res.files && t.meta.res.files.length > 0)
      return t.meta.res.files[0].name;
  }
  return "unknown";
}
function total_speed(tasks) {
  let sum = 0;
  tasks.forEach((t) => {
    if (normalize_download_status(t && t.status) === "running") {
      const speed = Number(
        t.speed ||
          (t.progress && typeof t.progress === "object"
            ? t.progress.speed
            : 0) ||
          0,
      );
      if (Number.isFinite(speed)) {
        sum += speed;
      }
    }
  });
  return sum;
}
function empty_download_status_counts() {
  return {
    total: 0,
    ready: 0,
    running: 0,
    wait: 0,
    pause: 0,
    error: 0,
    done: 0,
  };
}
const DOWNLOAD_WAITING_STATUS_KEYS = ["ready", "wait"];
const DOWNLOAD_STATUS_COUNT_ITEMS = [
  { key: "all", label: "全部", statuses: ["total"] },
  { key: "running", label: "下载中" },
  { key: "pause", label: "暂停" },
  { key: "wait", label: "等待中", statuses: DOWNLOAD_WAITING_STATUS_KEYS },
  { key: "done", label: "已完成" },
  { key: "error", label: "失败" },
];
function normalize_download_status(status) {
  const value = String(status ?? "")
    .trim()
    .toLowerCase();
  // V1 numeric statuses: 0=waiting,1=preparing,2=downloading,3=paused,4=merging,5=finished,6=failed,7=cancelled
  if (value === "0" || value === "waiting") return "wait";
  if (value === "1" || value === "preparing") return "wait";
  if (value === "2" || value === "downloading") return "running";
  if (value === "3" || value === "paused") return "pause";
  if (value === "4" || value === "merging") return "running";
  if (value === "5" || value === "finished") return "done";
  if (value === "6" || value === "failed") return "error";
  if (value === "7" || value === "cancelled") return "error";
  // legacy string statuses
  if (value === "fail" || value === "failure" || value === "errored") {
    return "error";
  }
  if (value === "pending" || value === "queued") {
    return "wait";
  }
  if (value === "completed" || value === "success") {
    return "done";
  }
  return value;
}

function download_resource_status_text(file) {
  const status = String((file && file.status) || "").toLowerCase();
  if (status === "finished" || status === "done") return "已完成";
  if (status === "error" || status === "failed") return "失败";
  if (status === "downloading" || status === "running") return "下载中";
  if (status === "paused" || status === "pause") return "已暂停";
  if (status === "cancelled" || status === "canceled") return "已取消";
  return "等待中";
}

function download_resource_metrics(file) {
  const size = WXU.bytes_to_size(Number((file && file.size) || 0));
  const downloaded = WXU.bytes_to_size(Number((file && file.downloaded) || 0));
  const speed = format_download_speed(Number((file && file.speed) || 0));
  const progress = Math.min(
    100,
    Math.max(0, Number((file && file.progress) || 0)),
  );
  return `${downloaded} / ${size} · ${speed} · ${progress.toFixed(2)}%`;
}
function is_download_waiting_status(status) {
  const normalized = normalize_download_status(status);
  return DOWNLOAD_WAITING_STATUS_KEYS.includes(normalized);
}
// Map UI status filters to V1 numeric status query values
const V1_STATUS_FILTER_MAP = {
  wait: "0,1", // Waiting(0), Preparing(1)
  running: "2,4", // Downloading(2), Merging(4)
  pause: "3", // Paused(3)
  done: "5", // Finished(5)
  error: "6,7", // Failed(6), Cancelled(7)
};
function v1_status_filter(status) {
  const normalized = normalize_download_status(status);
  return V1_STATUS_FILTER_MAP[normalized] || normalized;
}
function extract_filename_from_url(url) {
  try {
    const parsed = new URL(url);
    const parts = parsed.pathname.split("/").filter(Boolean);
    if (parts.length > 0) {
      return decodeURIComponent(parts[parts.length - 1]);
    }
  } catch (e) {}
  return "";
}
const DOWNLOAD_TASK_STATUS_ORDER = {
  running: 1,
  pause: 2,
  ready: 3,
  wait: 3,
  done: 4,
  error: 5,
};
function download_task_status_order(status) {
  const normalized = normalize_download_status(status);
  return DOWNLOAD_TASK_STATUS_ORDER[normalized] || 7;
}
function get_download_task_field(task, fields) {
  if (!task || typeof task !== "object") {
    return undefined;
  }
  for (let i = 0; i < fields.length; i += 1) {
    const value = task[fields[i]];
    if (typeof value !== "undefined" && value !== null && value !== "") {
      return value;
    }
  }
  return undefined;
}
function parse_download_task_time(value) {
  if (value instanceof Date) {
    const n = value.valueOf();
    return Number.isFinite(n) ? n : 0;
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) {
      return 0;
    }
    return value > 0 && value < 1000000000000 ? value * 1000 : value;
  }
  const text = String(value || "").trim();
  if (!text) {
    return 0;
  }
  if (/^\d+$/.test(text)) {
    return parse_download_task_time(Number(text));
  }
  const n = Date.parse(text);
  return Number.isFinite(n) ? n : 0;
}
function download_task_created_time(task) {
  return parse_download_task_time(
    get_download_task_field(task, [
      "created_at",
      "createdAt",
      "CreatedAt",
      "created",
      "create_time",
      "createTime",
      "CreateTime",
      "createtime",
    ]),
  );
}
function download_task_updated_time(task) {
  return parse_download_task_time(
    get_download_task_field(task, [
      "updated_at",
      "updatedAt",
      "UpdatedAt",
      "updated",
      "update_time",
      "updateTime",
      "UpdateTime",
      "updatetime",
    ]),
  );
}
function download_task_sort_time(task) {
  return download_task_created_time(task) || download_task_updated_time(task);
}
function download_task_sort_id(task) {
  const value = get_download_task_field(task, [
    "id",
    "ID",
    "task_id",
    "taskId",
    "TaskId",
  ]);
  return typeof value === "undefined" ? "" : value;
}
function compare_download_task_ids(a, b) {
  const av = download_task_sort_id(a);
  const bv = download_task_sort_id(b);
  const an = Number(av);
  const bn = Number(bv);
  if (Number.isFinite(an) && Number.isFinite(bn) && an !== bn) {
    return an - bn;
  }
  return String(av).localeCompare(String(bv), undefined, {
    numeric: true,
    sensitivity: "base",
  });
}
function compare_download_tasks_by_time_desc(a, b) {
  const timeA = download_task_sort_time(a);
  const timeB = download_task_sort_time(b);
  if (timeA !== timeB) {
    return timeB - timeA;
  }
  return -compare_download_task_ids(a, b);
}
function compare_download_tasks_by_status(a, b) {
  const statusA = normalize_download_status(a && a.status);
  const statusB = normalize_download_status(b && b.status);
  const orderA = download_task_status_order(statusA);
  const orderB = download_task_status_order(statusB);
  if (orderA !== orderB) {
    return orderA - orderB;
  }
  return compare_download_tasks_by_time_desc(a, b);
}
function sort_download_task_list(tasks, options = {}) {
  const compare = options.sort_by_status
    ? compare_download_tasks_by_status
    : compare_download_tasks_by_time_desc;
  return (Array.isArray(tasks) ? tasks : []).slice().sort(compare);
}
function normalize_download_status_filter(status) {
  const value = normalize_download_status(status);
  if (value === "all" || value === "") {
    return "all";
  }
  if (is_download_waiting_status(value)) {
    return "wait";
  }
  return typeof DOWNLOAD_TASK_STATUS_ORDER[value] === "number" ? value : "all";
}
function normalize_download_status_counts(counts) {
  const next = empty_download_status_counts();
  Object.keys(counts || {}).forEach((status) => {
    const normalized = normalize_download_status(status);
    if (typeof next[normalized] === "undefined") {
      return;
    }
    next[normalized] += Number(counts[status]) || 0;
  });
  if (!counts || typeof counts.total === "undefined") {
    next.total =
      next.ready +
      next.running +
      next.wait +
      next.pause +
      next.error +
      next.done;
  }
  return next;
}
function get_download_status_count(counts, item) {
  const c = normalize_download_status_counts(counts);
  const keys = Array.isArray(item && item.statuses)
    ? item.statuses
    : [typeof item === "string" ? item : item && item.key];
  return keys.reduce((sum, key) => {
    return sum + (Number(c[key]) || 0);
  }, 0);
}
function format_download_status_counts(counts) {
  const c = normalize_download_status_counts(counts);
  return [
    `下载中 ${c.running}`,
    `暂停 ${c.pause}`,
    `等待中 ${get_download_status_count(c, {
      statuses: DOWNLOAD_WAITING_STATUS_KEYS,
    })}`,
    `已完成 ${c.done}`,
    `失败 ${c.error}`,
  ].join(" · ");
}
function count_download_statuses(tasks) {
  const counts = empty_download_status_counts();
  (tasks || []).forEach((task) => {
    counts.total += 1;
    const status = normalize_download_status(task && task.status);
    if (typeof counts[status] !== "undefined") {
      counts[status] += 1;
    }
  });
  return counts;
}

function number_or_fallback(value, fallback) {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function DownloaderPanelViewModel(props = {}) {
  const ITEM_HEIGHT = Number(props.itemHeight) || 82;
  const ITEM_TITLE_LINE_HEIGHT = 20;
  const ITEM_STATUS_LINE_HEIGHT = 18;
  const ITEM_VERTICAL_PADDING = 24;
  const ITEM_TITLE_STATUS_GAP = 4;
  const ITEM_MAX_TITLE_LINES = 3;
  const ITEM_TITLE_UNITS_PER_LINE = 34;
  const GUTTER = 8;
  const fixed_list_height_ = props.fixedListHeight !== false;
  const sync_list_content_height_ = props.syncListContentHeight !== false;
  const LIST_HEIGHT = Number(props.listHeight) || 380;
  const _pageSize = 50;
  const LIST_BUFFER = 10;
  const PAGE_PREFETCH = 2;
  const onRequestClose =
    typeof props.onRequestClose === "function"
      ? props.onRequestClose
      : () => {};
  const sort_by_status = props.sort_by_status === true;
  // const initial_status = normalize_download_status_filter(
  //   typeof props.initial_status !== "undefined"
  //     ? props.initial_status
  //     : typeof props.initialStatus !== "undefined"
  //       ? props.initialStatus
  //       : "running",
  // );
  const initial_status = undefined;

  const reqs = {
    task: {
      list: new Timeless.RequestCore(
        (params) => request.get("/api/v1/download_task/list", params),
        {
          client: http_client,
          process(r) {
            if (r.error) {
              return Timeless.Result.Err(r.error);
            }
            const data = r.data;
            const counts = r.data.stats;
            setStatusCounts(counts, data.total);
            return Timeless.Result.Ok({
              list: data.list.map((t) => methods.formatTask(t)),
              total: data.total || 0,
              page: data.page || 1,
              page_size: data.page_size || _pageSize,
            });
          },
        },
      ),
      delete: new Timeless.RequestCore(
        (params = {}) => {
          return request.post("/api/v1/download_task/delete", {
            task_ids: params.ids,
            delete_files: !!params.deleteFiles,
          });
        },
        { client: http_client },
      ),
      start: new Timeless.RequestCore(
        (id) => request.post("/api/v1/download_task/start", { task_ids: [id] }),
        { client: http_client },
      ),
      startAll: new Timeless.RequestCore(
        (params = {}) => {
          const body = {};
          if (params.status && params.status !== "all") {
            body.status = params.status;
          }
          return request.post("/api/v1/download_task/start_all", body);
        },
        { client: http_client },
      ),
      pause: new Timeless.RequestCore(
        (id) => request.post("/api/v1/download_task/pause", { task_ids: [id] }),
        { client: http_client },
      ),
      pauseAll: new Timeless.RequestCore(
        (params = {}) => {
          const body = {};
          if (params.status && params.status !== "all") {
            body.status = params.status;
          }
          return request.post("/api/v1/download_task/pause_all", body);
        },
        { client: http_client },
      ),
      resume: new Timeless.RequestCore(
        (id) =>
          request.post("/api/v1/download_task/resume", { task_ids: [id] }),
        { client: http_client },
      ),
      retry: new Timeless.RequestCore(
        (id) =>
          request.post("/api/v1/download_task/retry", { task_ids: [id] }),
        { client: http_client },
      ),
      clear: new Timeless.RequestCore(
        (params = {}) => {
          return request.post("/api/v1/download_task/clear", {
            delete_files: !!params.deleteFiles,
          });
        },
        { client: http_client },
      ),
      create: new Timeless.RequestCore(
        (params = {}) =>
          request.post("/api/v1/download_task/create_by_url", {
            objects: [
              {
                url: params.url,
                config: params.config,
              },
            ],
          }),
        { client: http_client },
      ),
      createFromPlatform: new Timeless.RequestCore(
        (params = {}) =>
          request.post("/api/v1/download_task/create", {
            objects: [
              {
                platform: params.platform || "",
                content: params.content || {},
                config: params.config,
              },
            ],
          }),
        { client: http_client },
      ),
      prepare: new Timeless.RequestCore(
        (params = {}) =>
          request.post("/api/v1/download_task/prepare_by_url", [
            {
              url: params.url,
              filename: params.filename || "",
            },
          ]),
        { client: http_client },
      ),
      prepareFromPlatform: new Timeless.RequestCore(
        (params = {}) =>
          request.post("/api/v1/download_task/prepare", [
            {
              platform: params.platform || "",
              content: params.content || {},
              config: {
                download_cover: !!params.download_cover,
              },
            },
          ]),
        { client: http_client },
      ),
    },
    file: {
      show: new Timeless.RequestCore(
        ({ path, name, id }) =>
          request.post("/api/show_file", { path, name, id }),
        { client: http_client },
      ),
    },
  };

  const tasks_ = refarr([]);
  const task_count_ = ref(0);
  const list_render_enabled_ = ref(true);
  const delete_task_ = ref(null);
  const delete_task_ids_ = refarr([]);
  const delete_delete_files_ = ref(false);
  const deleting_task_ = ref(false);
  const clear_delete_files_ = ref(false);
  const clearing_tasks_ = ref(false);
  const create_task_text_ = ref("");
  const create_task_filename_ = ref("");
  const create_platform_text_ = ref("");
  const create_platform_json_ = ref("");
  const create_platform_save_path_ = ref("");
  const create_platform_filename_ = ref("");
  const create_platform_download_cover_ = ref(false);
  const creating_task_ = ref(false);
  const create_task_preview_ = ref(null);
  const create_platform_preview_ = ref(null);
  const selected_task_ids_ = refarr([]);
  const running_count_ = computed(tasks_, (t) => {
    return t.filter(
      (v) => normalize_download_status(v && v.status) === "running",
    ).length;
  });
  const status_counts_ = ref(empty_download_status_counts());
  const active_status_ = ref(initial_status);
  const overwrite_ = refobj({ value: "overwrite" });

  let duplicated_feed_prepare_download = null;

  function setStatusCounts(counts, total) {
    const normalized = normalize_download_status_counts(counts);
    status_counts_.as(normalized);
    task_count_.as(
      typeof total === "undefined" ? normalized.total : normalizeTotal(total),
    );
  }
  function adjustStatusCounts(fromStatus, toStatus, totalDelta) {
    status_counts_.as((prev) => {
      const next = normalize_download_status_counts(prev);
      const from = normalize_download_status(fromStatus);
      const to = normalize_download_status(toStatus);
      if (from && typeof next[from] !== "undefined" && next[from] > 0) {
        next[from] -= 1;
      }
      if (to && typeof next[to] !== "undefined") {
        next[to] += 1;
      }
      next.total = Math.max(0, next.total + (totalDelta || 0));
      return next;
    });
  }
  const updateTaskStatus = (id, status) => {
    const current = tasks_.value || [];
    const index = current.findIndex((t) => isLoadedTask(t) && t.id === id);
    if (index !== -1) {
      const oldStatus = current[index].status || "";
      const next = current.slice();
      next[index] = { ...current[index], status };
      if (matchesActiveStatusFilter(next[index])) {
        tasks_.as(sortTaskSlots(next, virtual_total || next.length));
      } else {
        const nextTotal = Math.max(0, (virtual_total || current.length) - 1);
        virtual_total = nextTotal;
        loaded_pages.clear();
        loading_pages.clear();
        pending_pages.clear();
        task_count_.as(nextTotal);
        tasks_.as(
          sortTaskSlots(
            next.filter((task) => !(isLoadedTask(task) && task.id === id)),
            nextTotal,
          ),
        );
        removeSelectedTaskIds([id]);
        setTimeout(maybeLoadMoreTasks, 0);
      }
      if (
        normalize_download_status(oldStatus) !==
        normalize_download_status(status)
      ) {
        adjustStatusCounts(oldStatus, status, 0);
      }
    }
  };
  let _scrollTop = 0;
  let list_view_el = null;
  let virtual_total = 0;
  let remount_list_scheduled = false;
  let sync_slot_heights_scheduled = false;
  let unbind_list_view_scroll = null;
  const loaded_pages = new Set();
  const loading_pages = new Set();
  const pending_pages = new Set();
  let selection_anchor_task_id = null;
  let draining_pages = false;
  const isDOMElement = (value) => {
    return !!(
      value &&
      typeof value.appendChild === "function" &&
      typeof value.setAttribute === "function"
    );
  };
  const getMountedElement = (event) => {
    let target = event && event.target ? event.target : event;
    for (let depth = 0; depth < 4; depth += 1) {
      if (isDOMElement(target)) {
        return target;
      }
      if (target && typeof target.get$elm === "function") {
        target = target.get$elm();
        continue;
      }
      if (target && target.$elm) {
        target = target.$elm;
        continue;
      }
      break;
    }
    return null;
  };
  const getCurrentListScrollTop = (payload) => {
    if (typeof payload === "number" && Number.isFinite(payload)) {
      return payload;
    }
    if (payload && typeof payload.scrollTop === "number") {
      return payload.scrollTop;
    }
    const target = payload && payload.target ? payload.target : null;
    if (target && typeof target.scrollTop === "number") {
      return target.scrollTop;
    }
    if (list_view_el && typeof list_view_el.scrollTop === "number") {
      return list_view_el.scrollTop;
    }
    return _scrollTop || 0;
  };
  const getVirtualTotal = () => {
    return virtual_total || task_count_.value || (tasks_.value || []).length;
  };
  const estimateTitleLineCount = (title) => {
    const text = String(title || "").trim();
    if (!text) {
      return 1;
    }
    let units = 0;
    for (let i = 0; i < text.length; i += 1) {
      units += text.charCodeAt(i) > 255 ? 2 : 1;
    }
    return Math.max(
      1,
      Math.min(
        ITEM_MAX_TITLE_LINES,
        Math.ceil(units / ITEM_TITLE_UNITS_PER_LINE),
      ),
    );
  };
  const estimateTaskItemHeight = (task) => {
    const lines = estimateTitleLineCount(task && task.name);
    const errorLineHeight =
      normalize_download_status(task && task.status) === "error"
        ? ITEM_STATUS_LINE_HEIGHT * 2 + 4
        : 0;
    const resourceCount = Array.isArray(task && task.files)
      ? task.files.length
      : 0;
    const resourceLineHeight = resourceCount > 1 ? resourceCount * 34 + 6 : 0;
    const textHeight =
      lines * ITEM_TITLE_LINE_HEIGHT +
      ITEM_TITLE_STATUS_GAP +
      ITEM_STATUS_LINE_HEIGHT +
      errorLineHeight +
      resourceLineHeight;
    return Math.max(ITEM_HEIGHT, ITEM_VERTICAL_PADDING + textHeight);
  };
  const getTaskEstimatedHeight = (task) => {
    const height = Number(task && task.height);
    return Number.isFinite(height) && height > 0
      ? height
      : estimateTaskItemHeight(task);
  };
  const getEstimatedItemHeight = () => {
    return ITEM_HEIGHT;
  };
  const getEstimatedRowHeight = () => getEstimatedItemHeight() + GUTTER;
  const getVirtualContentHeight = () => {
    const total = getVirtualTotal();
    if (total <= 0) {
      return 0;
    }
    const items = tasks_.value || [];
    let height = 0;
    for (let i = 0; i < total; i += 1) {
      height += getTaskEstimatedHeight(items[i]);
    }
    return height + Math.max(0, total - 1) * GUTTER;
  };
  const getListViewMaxScrollTop = () => {
    const contentHeight = getVirtualContentHeight();
    const clientHeight =
      list_view_el && typeof list_view_el.clientHeight === "number"
        ? list_view_el.clientHeight
        : 0;
    return Math.max(0, contentHeight - clientHeight);
  };
  const clampVirtualScrollTop = (scrollTop) => {
    const n = Number(scrollTop);
    const value = Number.isFinite(n) ? n : 0;
    return Math.max(0, Math.min(value, getListViewMaxScrollTop()));
  };
  const dispatchListViewScroll = () => {
    if (!list_view_el || typeof list_view_el.dispatchEvent !== "function") {
      return;
    }
    let event;
    if (typeof Event === "function") {
      event = new Event("scroll");
    } else if (typeof document !== "undefined" && document.createEvent) {
      event = document.createEvent("Event");
      event.initEvent("scroll", false, false);
    }
    if (event) {
      list_view_el.dispatchEvent(event);
    }
  };
  const syncListViewSlotHeights = () => {
    if (typeof document === "undefined") {
      return;
    }
    let items = [];
    if (
      sync_list_content_height_ &&
      list_view_el &&
      typeof list_view_el.querySelectorAll === "function"
    ) {
      items = Array.from(
        list_view_el.querySelectorAll("[data-list-view-item]"),
      );
    }
    if (sync_list_content_height_ && !items.length) {
      items = Array.from(
        document.querySelectorAll(".wx-dl-list [data-list-view-item]"),
      );
    }
    items.forEach((item) => {
      item.style.height = "";
      item.style.minHeight = "";
      item.style.maxHeight = "";
      item.style.boxSizing = "border-box";
    });
    if (list_view_el) {
      if (fixed_list_height_) {
        list_view_el.style.height = `${LIST_HEIGHT}px`;
        list_view_el.style.maxHeight = `${LIST_HEIGHT}px`;
      } else {
        list_view_el.style.height = "";
        list_view_el.style.maxHeight = "100%";
      }
      list_view_el.style.overflowY = "auto";
    }
    const isDynamicVirtualList =
      list_view_el &&
      typeof list_view_el.getAttribute === "function" &&
      list_view_el.getAttribute("data-virtual-list-view") === "dynamic";
    if (!sync_list_content_height_ || isDynamicVirtualList) {
      return;
    }
    const contentHeight = getVirtualContentHeight();
    const content =
      (list_view_el &&
        typeof list_view_el.querySelector === "function" &&
        list_view_el.querySelector("[data-list-view-content]")) ||
      document.querySelector(".wx-dl-list [data-list-view-content]");
    if (content) {
      content.style.height = `${contentHeight}px`;
      content.style.minHeight = `${contentHeight}px`;
      let spacer = content.querySelector("[data-wx-download-list-spacer]");
      if (!spacer) {
        spacer = document.createElement("div");
        spacer.setAttribute("data-wx-download-list-spacer", "");
        spacer.style.pointerEvents = "none";
        spacer.style.width = "1px";
        spacer.style.opacity = "0";
        content.insertBefore(spacer, content.firstChild);
      }
      spacer.style.height = `${contentHeight}px`;
    }
  };
  const scheduleSyncListViewSlotHeights = () => {
    if (sync_slot_heights_scheduled) {
      return;
    }
    sync_slot_heights_scheduled = true;
    setTimeout(() => {
      sync_slot_heights_scheduled = false;
      syncListViewSlotHeights();
      setTimeout(syncListViewSlotHeights, 32);
    }, 0);
  };
  const syncListViewScrollPosition = (scrollTop) => {
    syncListViewSlotHeights();
    if (!list_view_el) {
      return;
    }
    const maxScrollTop = getListViewMaxScrollTop();
    const nextScrollTop = Math.max(
      0,
      Math.min(Number(scrollTop) || 0, maxScrollTop),
    );
    if (Math.abs(list_view_el.scrollTop - nextScrollTop) > 0.5) {
      list_view_el.scrollTop = nextScrollTop;
    }
    dispatchListViewScroll();
    maybeLoadMoreTasks(nextScrollTop);
  };
  const scheduleListViewScrollSync = (scrollTop = _scrollTop) => {
    const run = () => syncListViewScrollPosition(scrollTop);
    setTimeout(run, 0);
    setTimeout(run, 32);
    if (typeof requestAnimationFrame === "function") {
      requestAnimationFrame(run);
    }
  };

  const isPlaceholderTask = (task) => {
    return !!(task && task.__placeholder);
  };
  const isLoadedTask = (task) => {
    return !!(task && !isPlaceholderTask(task) && task.id);
  };
  const uniqueTaskIds = (ids) => {
    const next = [];
    (ids || []).forEach((id) => {
      if (!id || next.some((item) => item === id)) {
        return;
      }
      next.push(id);
    });
    return next;
  };
  const getLoadedTasks = () => {
    return (tasks_.value || []).filter((task) => isLoadedTask(task));
  };
  const getLoadedTaskIds = () => {
    return getLoadedTasks().map((task) => task.id);
  };
  const getTaskRangeIds = (fromId, toId) => {
    if (!fromId || !toId) {
      return [];
    }
    const current = tasks_.value || [];
    const fromIndex = current.findIndex(
      (task) => isLoadedTask(task) && task.id === fromId,
    );
    const toIndex = current.findIndex(
      (task) => isLoadedTask(task) && task.id === toId,
    );
    if (fromIndex === -1 || toIndex === -1) {
      return [];
    }
    const start = Math.min(fromIndex, toIndex);
    const end = Math.max(fromIndex, toIndex);
    const ids = [];
    for (let i = start; i <= end; i += 1) {
      if (isLoadedTask(current[i])) {
        ids.push(current[i].id);
      }
    }
    return uniqueTaskIds(ids);
  };
  const isTaskIdSelected = (id, ids = selected_task_ids_.value || []) => {
    return (ids || []).some((selectedId) => selectedId === id);
  };
  const getSelectedTaskIds = () => {
    const loadedIds = getLoadedTaskIds();
    return uniqueTaskIds(selected_task_ids_.value || []).filter((id) => {
      return loadedIds.some((loadedId) => loadedId === id);
    });
  };
  const removeSelectedTaskIds = (ids) => {
    const removeIds = uniqueTaskIds(ids);
    if (!removeIds.length) {
      return;
    }
    if (removeIds.some((id) => id === selection_anchor_task_id)) {
      selection_anchor_task_id = null;
    }
    selected_task_ids_.as(
      (selected_task_ids_.value || []).filter((id) => {
        return !removeIds.some((removeId) => removeId === id);
      }),
    );
  };
  const selected_task_count_ = combine(
    {
      tasks: tasks_,
      selected_ids: selected_task_ids_,
    },
    (data) => {
      const loadedIds = (data.tasks || [])
        .filter((task) => isLoadedTask(task))
        .map((task) => task.id);
      return uniqueTaskIds(data.selected_ids || []).filter((id) => {
        return loadedIds.some((loadedId) => loadedId === id);
      }).length;
    },
  );
  const pending_delete_task_count_ = combine(
    {
      task: delete_task_,
      ids: delete_task_ids_,
    },
    (data) => {
      const ids = uniqueTaskIds(data.ids || []);
      if (ids.length > 0) {
        return ids.length;
      }
      return data.task && data.task.id ? 1 : 0;
    },
  );
  const getActiveStatusFilter = () => {
    return normalize_download_status_filter(active_status_.value);
  };
  const matchesActiveStatusFilter = (task) => {
    const activeStatus = getActiveStatusFilter();
    if (activeStatus === "all") {
      return true;
    }
    if (activeStatus === "wait") {
      return is_download_waiting_status(task && task.status);
    }
    return normalize_download_status(task && task.status) === activeStatus;
  };
  const normalizeTotal = (total) => {
    const n = Number(total);
    if (!Number.isFinite(n) || n < 0) {
      return 0;
    }
    return Math.floor(n);
  };
  const normalizePage = (page) => {
    const n = Number(page);
    if (!Number.isFinite(n) || n < 1) {
      return 1;
    }
    return Math.floor(n);
  };
  const makeTaskPlaceholder = (index) => {
    return {
      id: `__wx_download_placeholder_${index}`,
      __placeholder: true,
      __index: index,
      height: ITEM_HEIGHT,
    };
  };
  const normalizeTaskSlots = (items, total) => {
    const next = new Array(total);
    for (let i = 0; i < total; i += 1) {
      const task = items && items[i];
      next[i] = isLoadedTask(task) ? task : makeTaskPlaceholder(i);
    }
    return next;
  };
  const sortTaskSlots = (items, total) => {
    const normalizedTotal = normalizeTotal(total);
    const loaded = sort_download_task_list(
      (items || []).filter((task) => isLoadedTask(task)),
      { sort_by_status },
    );
    const next = new Array(normalizedTotal);
    for (let i = 0; i < normalizedTotal; i += 1) {
      next[i] = loaded[i] || makeTaskPlaceholder(i);
    }
    return next;
  };
  const applyDeletedTaskIds = (ids) => {
    const deleteIds = uniqueTaskIds(ids);
    if (!deleteIds.length) {
      return 0;
    }
    const current = tasks_.value || [];
    const removedTasks = current.filter((task) => {
      return (
        isLoadedTask(task) && deleteIds.some((deleteId) => deleteId === task.id)
      );
    });
    const removedCount = removedTasks.length;
    if (!removedCount) {
      removeSelectedTaskIds(deleteIds);
      return 0;
    }
    const nextTotal = Math.max(
      0,
      (virtual_total || current.length) - removedCount,
    );
    const next = current
      .filter((task) => {
        return !(
          isLoadedTask(task) &&
          deleteIds.some((deleteId) => deleteId === task.id)
        );
      })
      .slice(0, nextTotal);
    while (next.length < nextTotal) {
      next.push(makeTaskPlaceholder(next.length));
    }
    for (let i = 0; i < next.length; i += 1) {
      if (isPlaceholderTask(next[i])) {
        next[i] = makeTaskPlaceholder(i);
      }
    }
    virtual_total = nextTotal;
    loaded_pages.clear();
    loading_pages.clear();
    pending_pages.clear();
    task_count_.as(nextTotal);
    tasks_.as(sortTaskSlots(next, nextTotal));
    removedTasks.forEach((task) => {
      adjustStatusCounts(task.status || "", "", -1);
    });
    removeSelectedTaskIds(deleteIds);
    setTimeout(maybeLoadMoreTasks, 0);
    return removedCount;
  };
  const remountListView = () => {
    if (remount_list_scheduled) {
      return;
    }
    remount_list_scheduled = true;
    list_render_enabled_.as(false);
    setTimeout(() => {
      remount_list_scheduled = false;
      list_render_enabled_.as(true);
    }, 0);
  };
  const resetVirtualTasks = () => {
    virtual_total = 0;
    loaded_pages.clear();
    loading_pages.clear();
    pending_pages.clear();
    draining_pages = false;
    remount_list_scheduled = false;
    list_render_enabled_.as(true);
    tasks_.as([], { reset: true });
    task_count_.as(0);
    selected_task_ids_.as([]);
    delete_task_ids_.as([]);
    selection_anchor_task_id = null;
    _scrollTop = 0;
    if (list_view_el) {
      list_view_el.scrollTop = 0;
    }
  };
  const applyTaskPage = (data, options = {}) => {
    const page = normalizePage(data && data.page);
    const pageSize = normalizePage((data && data.page_size) || _pageSize);
    const total = normalizeTotal(data && data.total);
    const current = options.reset ? [] : tasks_.value || [];
    const next = normalizeTaskSlots(current, total);
    const start = (page - 1) * pageSize;
    const list = Array.isArray(data && data.list) ? data.list : [];
    for (let i = 0; i < list.length; i += 1) {
      const index = start + i;
      if (index >= 0 && index < total) {
        next[index] = list[i];
      }
    }
    virtual_total = total;
    task_count_.as(total);
    loaded_pages.add(page);
    pending_pages.delete(page);
    const lastPage = Math.max(1, Math.ceil(total / pageSize));
    if (!options.reset && page >= lastPage - 1 && page < lastPage) {
      pending_pages.add(lastPage);
      setTimeout(drainPendingTaskPages, 0);
    }
    if (!options.reset && list_view_el) {
      _scrollTop = getCurrentListScrollTop();
    }
    tasks_.as(
      sortTaskSlots(next, total),
      options.reset ? { reset: true } : undefined,
    );
    scheduleSyncListViewSlotHeights();
    if (!options.reset) {
      scheduleListViewScrollSync(_scrollTop);
    }
  };
  const loadWaitingTaskPage = async (page) => {
    const normalizedPage = normalizePage(page);
    const pageSize = normalizedPage * _pageSize;
    const [waitResult, readyResult] = await Promise.all([
      createTaskListReq().run({
        page: 1,
        page_size: pageSize,
        status: "0",
      }),
      createTaskListReq().run({
        page: 1,
        page_size: pageSize,
        status: "1",
      }),
    ]);
    if (waitResult && waitResult.error) {
      return waitResult;
    }
    if (readyResult && readyResult.error) {
      return readyResult;
    }
    const waitData = (waitResult && waitResult.data) || {};
    const readyData = (readyResult && readyResult.data) || {};
    const merged = sort_download_task_list(
      [
        ...((waitData && waitData.list) || []),
        ...((readyData && readyData.list) || []),
      ],
      { sort_by_status },
    );
    const start = (normalizedPage - 1) * _pageSize;
    const countTotal = get_download_status_count(status_counts_.value, {
      statuses: DOWNLOAD_WAITING_STATUS_KEYS,
    });
    const responseTotal =
      normalizeTotal(waitData.total) + normalizeTotal(readyData.total);
    return Timeless.Result.Ok({
      list: merged.slice(start, start + _pageSize),
      total: countTotal || responseTotal,
      page: normalizedPage,
      page_size: _pageSize,
    });
  };
  const loadTaskPage = async (page, options = {}) => {
    const normalizedPage = normalizePage(page);
    if (
      !options.force &&
      (loaded_pages.has(normalizedPage) || loading_pages.has(normalizedPage))
    ) {
      return;
    }
    loading_pages.add(normalizedPage);
    try {
      const params = {
        page: normalizedPage,
        page_size: _pageSize,
      };
      const activeStatus = getActiveStatusFilter();
      if (activeStatus !== "all") {
        params.status = v1_status_filter(activeStatus);
      }
      const r =
        activeStatus === "wait"
          ? await loadWaitingTaskPage(normalizedPage)
          : await reqs.task.list.run(params);
      if (r && r.error) {
        if (!options.silent) {
          WXU.error({ msg: r.error.message });
        }
        return r;
      }
      applyTaskPage(r.data, options);
      setTimeout(maybeLoadMoreTasks, 0);
      return r;
    } finally {
      loading_pages.delete(normalizedPage);
    }
  };
  const drainPendingTaskPages = async () => {
    if (draining_pages) {
      return;
    }
    draining_pages = true;
    try {
      while (pending_pages.size > 0) {
        const page = Array.from(pending_pages).sort((a, b) => a - b)[0];
        pending_pages.delete(page);
        if (loaded_pages.has(page) || loading_pages.has(page)) {
          continue;
        }
        await loadTaskPage(page, { silent: true });
      }
    } finally {
      draining_pages = false;
      if (pending_pages.size > 0) {
        setTimeout(drainPendingTaskPages, 0);
      }
    }
  };
  const ensureTaskPage = (page) => {
    const normalizedPage = normalizePage(page);
    if (loaded_pages.has(normalizedPage) || loading_pages.has(normalizedPage)) {
      return;
    }
    pending_pages.add(normalizedPage);
    drainPendingTaskPages();
  };
  const ensureTaskPageForIndex = (index) => {
    const n = Number(index);
    if (!Number.isFinite(n) || n < 0) {
      return;
    }
    const page = Math.floor(n / _pageSize) + 1;
    ensureTaskPage(page);
    const total = getVirtualTotal();
    const lastPage = Math.max(1, Math.ceil(total / _pageSize));
    if (page >= lastPage - 1) {
      ensureTaskPage(lastPage);
    }
  };
  const getHighestLoadedPage = () => {
    let highest = 0;
    loaded_pages.forEach((page) => {
      highest = Math.max(highest, page);
    });
    return highest;
  };
  const getScrollEndPage = (scrollTop, clientHeight) => {
    const rowHeight = getEstimatedRowHeight();
    const endIndex = Math.ceil(
      ((Number(scrollTop) || 0) + (Number(clientHeight) || 0)) / rowHeight,
    );
    return Math.max(1, Math.floor(endIndex / _pageSize) + 1);
  };
  const maybeQueueNextPageNearLoadedTail = (scrollTop, clientHeight) => {
    const total = getVirtualTotal();
    if (!total) {
      return;
    }
    const lastPage = Math.max(1, Math.ceil(total / _pageSize));
    const highestLoadedPage = getHighestLoadedPage();
    if (!highestLoadedPage || highestLoadedPage >= lastPage) {
      return;
    }
    const scrollEndPage = getScrollEndPage(scrollTop, clientHeight);
    if (scrollEndPage + PAGE_PREFETCH >= highestLoadedPage) {
      ensureTaskPage(highestLoadedPage + 1);
    }
  };
  const ensurePagesForScroll = (scrollTopFromEvent) => {
    if (!list_view_el) {
      return;
    }
    const total = getVirtualTotal();
    if (!total) {
      return;
    }
    const rowHeight = getEstimatedRowHeight();
    const scrollTop = getCurrentListScrollTop(scrollTopFromEvent);
    const clientHeight = list_view_el.clientHeight || rowHeight * 4;
    const domScrollHeight = list_view_el.scrollHeight || 0;
    const contentHeight = Math.max(
      getVirtualContentHeight(),
      domScrollHeight,
      clientHeight,
    );
    const maxScrollTop = Math.max(0, contentHeight - clientHeight);
    const effectiveScrollTop = Math.max(0, Math.min(scrollTop, maxScrollTop));
    const nearVirtualBottom =
      effectiveScrollTop + clientHeight >= contentHeight - rowHeight * 2;
    const nearDomBottom =
      domScrollHeight > 0 &&
      scrollTop + clientHeight >= domScrollHeight - rowHeight * 2;
    const nearBottom =
      nearVirtualBottom || (domScrollHeight < contentHeight && nearDomBottom);
    const indexScrollTop =
      nearBottom && domScrollHeight < contentHeight
        ? maxScrollTop
        : effectiveScrollTop;
    if (nearBottom && domScrollHeight < contentHeight) {
      _scrollTop = maxScrollTop;
    }
    const startIndex = Math.max(0, Math.floor(indexScrollTop / rowHeight) - 5);
    const viewportEndIndex =
      Math.ceil((indexScrollTop + clientHeight) / rowHeight) + 5;
    const slotEndIndex = startIndex + _pageSize + LIST_BUFFER * 2;
    const endIndex = Math.min(
      total - 1,
      Math.max(viewportEndIndex, slotEndIndex),
    );
    const lastPage = Math.max(1, Math.ceil(total / _pageSize));
    const startPage = Math.floor(startIndex / _pageSize) + 1;
    const endPage = Math.floor(endIndex / _pageSize) + 1;
    const prefetchEndPage = Math.min(lastPage, endPage + PAGE_PREFETCH);
    for (let page = startPage; page <= prefetchEndPage; page += 1) {
      ensureTaskPage(page);
    }
    if (endPage >= lastPage - 1 || nearBottom) {
      ensureTaskPage(lastPage);
    }
    maybeQueueNextPageNearLoadedTail(indexScrollTop, clientHeight);
  };
  const maybeLoadMoreTasks = (scrollTop) => {
    ensurePagesForScroll(scrollTop);
  };
  const bindNativeListViewScroll = () => {
    if (unbind_list_view_scroll) {
      unbind_list_view_scroll();
      unbind_list_view_scroll = null;
    }
    if (!list_view_el || typeof list_view_el.addEventListener !== "function") {
      return;
    }
    const target = list_view_el;
    const onScroll = () => {
      if (target !== list_view_el) {
        return;
      }
      const scrollTop = getCurrentListScrollTop({ target });
      _scrollTop = scrollTop;
      if (sync_list_content_height_) {
        scheduleSyncListViewSlotHeights();
      }
      maybeLoadMoreTasks(scrollTop);
    };
    const onWheel = (event) => {
      if (target !== list_view_el) {
        return;
      }
      const deltaY = Number(event && event.deltaY) || 0;
      if (!deltaY) {
        return;
      }
      const currentScrollTop = Math.max(
        _scrollTop || 0,
        getCurrentListScrollTop({ target }),
      );
      _scrollTop = clampVirtualScrollTop(currentScrollTop + deltaY);
      maybeLoadMoreTasks(_scrollTop);
      setTimeout(() => {
        const latestScrollTop = Math.max(
          _scrollTop || 0,
          getCurrentListScrollTop({ target }),
        );
        _scrollTop = clampVirtualScrollTop(latestScrollTop);
        maybeLoadMoreTasks(_scrollTop);
      }, 0);
    };
    target.addEventListener("scroll", onScroll, { passive: true });
    target.addEventListener("wheel", onWheel, { passive: true });
    unbind_list_view_scroll = () => {
      target.removeEventListener("scroll", onScroll);
      target.removeEventListener("wheel", onWheel);
    };
  };
  async function reloadTasks() {
    resetVirtualTasks();
    const r = await loadTaskPage(1, { reset: true, force: true });
    setTimeout(maybeLoadMoreTasks, 0);
    return r;
  }
  const methods = {
    async setStatusFilter(status) {
      const nextStatus = normalize_download_status_filter(status);
      if (nextStatus === getActiveStatusFilter() && tasks_.value.length > 0) {
        return;
      }
      active_status_.as(nextStatus);
      const r = await reloadTasks();
      if (r && r.error) {
        WXU.error({ msg: r.error.message });
      }
    },
    formatTask(task) {
      const r = {
        ...task,
        ...(() => {
          if (!Array.isArray(task.files)) {
            return {};
          }
          const file = task.files[0];
          if (!file) {
            return {};
          }
          if (task.files.length > 1) {
            return {
              name: `${task.name} +${task.files.length}个文件`,
              filename: file.name,
              path: file.output_path,
            };
          }
          return {
            name: file.name,
            filename: file.name,
            path: file.output_path,
          };
        })(),
      };
      r.height = estimateTaskItemHeight(r);
      return r;
    },
    async startTask(task) {
      const r = await reqs.task.start.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const result = r.data && r.data.results && r.data.results[0];
      if (result && result.success) {
        updateTaskStatus(task.id, "running");
      } else {
        WXU.error({ msg: (result && result.error) || "启动失败" });
      }
    },
    async pauseTask(task) {
      const r = await reqs.task.pause.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const result = r.data && r.data.results && r.data.results[0];
      if (result && result.success) {
        updateTaskStatus(task.id, "paused");
      } else {
        WXU.error({ msg: (result && result.error) || "暂停失败" });
      }
    },
    isTaskSelected(task) {
      return isLoadedTask(task) && isTaskIdSelected(task.id);
    },
    setTaskSelected(task, selected) {
      if (!isLoadedTask(task)) {
        return;
      }
      const current = uniqueTaskIds(selected_task_ids_.value || []);
      const exists = isTaskIdSelected(task.id, current);
      if (selected && !exists) {
        selected_task_ids_.as([...current, task.id]);
        return;
      }
      if (!selected && exists) {
        selected_task_ids_.as(current.filter((id) => id !== task.id));
      }
    },
    selectTaskRange(fromId, toId) {
      const rangeIds = getTaskRangeIds(fromId, toId);
      if (!rangeIds.length) {
        return false;
      }
      selected_task_ids_.as(
        uniqueTaskIds([...(selected_task_ids_.value || []), ...rangeIds]),
      );
      return true;
    },
    toggleTaskSelected(task, options = {}) {
      if (!isLoadedTask(task)) {
        return;
      }
      if (
        options.shiftKey &&
        selection_anchor_task_id &&
        selection_anchor_task_id !== task.id &&
        methods.selectTaskRange(selection_anchor_task_id, task.id)
      ) {
        selection_anchor_task_id = task.id;
        return;
      }
      methods.setTaskSelected(task, !isTaskIdSelected(task.id));
      selection_anchor_task_id = task.id;
    },
    clearTaskSelection() {
      selected_task_ids_.as([]);
      selection_anchor_task_id = null;
    },
    requestDeleteTask(task) {
      if (!task || !task.id) {
        WXU.error({ msg: "异常操作" });
        return;
      }
      delete_task_.as(task);
      delete_task_ids_.as([]);
      ui.deleteConfirmDialog$.show();
    },
    requestDeleteSelectedTasks() {
      const ids = getSelectedTaskIds();
      if (!ids.length) {
        WXU.error({ msg: "请选择要删除的下载任务" });
        return;
      }
      delete_task_.as(null);
      delete_task_ids_.as(ids);
      ui.deleteConfirmDialog$.show();
    },
    async deleteTask(task, params = {}) {
      const r = await reqs.task.delete.run({
        ids: [task.id],
        deleteFiles: params.deleteFiles,
      });
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return false;
      }
      const result = r.data && r.data.results && r.data.results[0];
      if (result && result.success) {
        applyDeletedTaskIds([task.id]);
        return true;
      }
      WXU.error({ msg: (result && result.error) || "删除失败" });
      return false;
    },
    async deleteTasksByIds(ids, params = {}) {
      const taskIds = uniqueTaskIds(ids);
      if (!taskIds.length) {
        WXU.error({ msg: "请选择要删除的下载任务" });
        return false;
      }
      const r = await reqs.task.delete.run({
        ids: taskIds,
        deleteFiles: params.deleteFiles,
      });
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return false;
      }
      const results = r.data && r.data.results ? r.data.results : [];
      const deletedIds = [];
      const errors = [];
      for (const result of results) {
        if (result.success) {
          deletedIds.push(result.task_id);
        } else {
          errors.push(result.error || "删除失败");
        }
      }
      if (deletedIds.length) {
        applyDeletedTaskIds(deletedIds);
      }
      if (errors.length) {
        WXU.error({ msg: errors.join("; ") });
      }
      return deletedIds.length > 0;
    },
    async confirmDeleteTask() {
      if (deleting_task_.value) {
        return;
      }
      const task = delete_task_.value;
      const taskIds = uniqueTaskIds(delete_task_ids_.value || []);
      if ((!task || !task.id) && !taskIds.length) {
        WXU.error({ msg: "异常操作" });
        ui.deleteConfirmDialog$.hide();
        return;
      }
      deleting_task_.as(true);
      try {
        const ok = taskIds.length
          ? await methods.deleteTasksByIds(taskIds, {
              deleteFiles: delete_delete_files_.value,
            })
          : await methods.deleteTask(task, {
              deleteFiles: delete_delete_files_.value,
            });
        if (ok) {
          delete_task_.as(null);
          delete_task_ids_.as([]);
          ui.deleteConfirmDialog$.hide();
        }
      } finally {
        deleting_task_.as(false);
      }
    },
    async resumeTask(task) {
      const r = await reqs.task.resume.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const result = r.data && r.data.results && r.data.results[0];
      if (result && result.success) {
        updateTaskStatus(task.id, "running");
      } else {
        WXU.error({ msg: (result && result.error) || "恢复失败" });
      }
    },
    async retryTask(task) {
      const r = await reqs.task.retry.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const result = r.data && r.data.results && r.data.results[0];
      if (result && result.success) {
        updateTaskStatus(task.id, "running");
      } else {
        WXU.error({ msg: (result && result.error) || "重试失败" });
      }
    },
    async startAllTasks() {
      var maxRunning =
        WXEnv.config.MaxRunning || WXEnv.defaults.MaxRunning;
      if (running_count_.value >= maxRunning) {
        WXU.warning({
          msg:
            "已达到最大同时下载任务数（" + maxRunning + "），请等待当前任务完成",
        });
        return;
      }
      const r = await reqs.task.startAll.run({
        status: getActiveStatusFilter(),
      });
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const reloadResult = await reloadTasks();
      if (reloadResult && reloadResult.error) {
        WXU.error({ msg: reloadResult.error.message });
      }
    },
    async pauseAllTasks() {
      const r = await reqs.task.pauseAll.run({
        status: getActiveStatusFilter(),
      });
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const reloadResult = await reloadTasks();
      if (reloadResult && reloadResult.error) {
        WXU.error({ msg: reloadResult.error.message });
      }
    },
    async clearTasks(params = {}) {
      const r = await reqs.task.clear.run(params);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return false;
      }
      resetVirtualTasks();
      status_counts_.as(empty_download_status_counts());
      return true;
    },
    async confirmClearTasks() {
      try {
        const ok = await methods.clearTasks({
          deleteFiles: delete_delete_files_.value,
        });
        if (ok) {
          ui.clearConfirmDialog$.hide();
        }
      } finally {
        clearing_tasks_.as(false);
      }
    },
    requestCreateTask() {
      create_task_text_.as("");
      create_task_filename_.as("");
      ui.createTaskDialog$.show();
    },
    requestCreatePlatformTask() {
      create_platform_text_.as("");
      create_platform_json_.as("");
      create_platform_save_path_.as("");
      create_platform_filename_.as("");
      create_platform_download_cover_.as(false);
      ui.createPlatformTaskDialog$.show();
    },
    async confirmCreatePlatformTask() {
      if (creating_task_.value) {
        return;
      }
      const platform = String(create_platform_text_.value || "").trim();
      if (!platform) {
        WXU.error({ msg: "请输入平台名称" });
        return;
      }
      const jsonStr = String(create_platform_json_.value || "").trim();
      var content = {};
      if (jsonStr) {
        try {
          content = JSON.parse(jsonStr);
        } catch (e) {
          WXU.error({ msg: "内容 JSON 格式错误: " + e.message });
          return;
        }
      }
      creating_task_.as(true);
      try {
        const r = await reqs.task.prepareFromPlatform.run({
          platform: platform,
          content: content,
          config: {
            save_path: create_platform_save_path_.value || "",
            filename: create_platform_filename_.value || "",
          },
        });
        if (r.error) {
          WXU.error({ msg: r.error.message });
          return;
        }
        const platformPreview =
          r.data && r.data.previews && r.data.previews[0]
            ? r.data.previews[0].data
            : null;
        create_platform_preview_.as(platformPreview);
        ui.createPlatformTaskDialog$.hide();
        ui.createPlatformTaskPreviewDialog$.show();
      } finally {
        creating_task_.as(false);
      }
    },
    setCreateTaskText(value) {
      create_task_text_.as(value || "");
      const filename = extract_filename_from_url(value || "");
      if (filename) {
        create_task_filename_.as(filename);
      }
    },
    async confirmCreateTask() {
      if (creating_task_.value) {
        return;
      }
      const text = String(create_task_text_.value || "").trim();
      if (!text) {
        WXU.error({ msg: "请输入下载地址" });
        return;
      }
      creating_task_.as(true);
      try {
        const r = await prepareTaskReq.run({
          url: text,
          filename: create_task_filename_.value || "",
        });
        if (r.error) {
          WXU.error({ msg: r.error.message });
          return;
        }
        const taskPreview =
          r.data && r.data.previews && r.data.previews[0]
            ? r.data.previews[0].data
            : null;
        create_task_preview_.as(taskPreview);
        ui.createTaskDialog$.hide();
        ui.createTaskPreviewDialog$.show();
      } finally {
        creating_task_.as(false);
      }
    },
    async confirmCreateTaskFromPreview() {
      const preview = create_task_preview_.value;
      if (!preview) {
        return;
      }
      if (creating_task_.value) {
        return;
      }
      creating_task_.as(true);
      try {
        const url = create_task_text_.value || preview.url || "";
        const filename = create_task_filename_.value || preview.task_name || "";
        const r = await reqs.task.create.run({
          url: url,
          filename: filename,
        });
        if (r.error) {
          WXU.error({ msg: r.error.message });
          return;
        }
        const taskResult = r.data && r.data.tasks && r.data.tasks[0];
        if (taskResult && !taskResult.success) {
          WXU.error({ msg: taskResult.error || "创建下载任务失败" });
          return;
        }
        ui.createTaskPreviewDialog$.hide();
        WXU.toast("下载任务创建成功");
        const reloadResult = await reloadTasks();
        if (reloadResult && reloadResult.error) {
          WXU.error({ msg: reloadResult.error.message });
        }
      } finally {
        creating_task_.as(false);
      }
    },
    async confirmCreatePlatformTaskFromPreview() {
      const preview = create_platform_preview_.value;
      if (!preview) {
        return;
      }
      if (creating_task_.value) {
        return;
      }
      creating_task_.as(true);
      try {
        const platform = create_platform_text_.value || preview.platform || "";
        var content = {};
        try {
          content = JSON.parse(create_platform_json_.value || "{}");
        } catch (e) {}
        const r = await reqs.task.createFromPlatform.run({
          platform: platform,
          content: content,
        });
        if (r.error) {
          var code = (r.error && (r.error.code || r.error.status)) || 0;
          if (code === 409) {
            // 409: Task already exists, show duplicate handling dialog
            duplicated_feed_prepare_download = {
              objects: [
                {
                  platform: platform,
                  content: content,
                  config: {
                    download_cover: create_platform_download_cover_.value,
                  },
                },
              ],
            };
            ui.createPlatformTaskPreviewDialog$.hide();
            ui.overwriteConfirmDialog$.show();
            return;
          }
          WXU.error({ msg: r.error.message });
          return;
        }
        const taskResult = r.data && r.data.tasks && r.data.tasks[0];
        if (taskResult && !taskResult.success) {
          WXU.error({ msg: taskResult.error || "创建下载任务失败" });
          return;
        }
        ui.createPlatformTaskPreviewDialog$.hide();
        WXU.toast("平台下载任务创建成功");
        const reloadResult = await reloadTasks();
        if (reloadResult && reloadResult.error) {
          WXU.error({ msg: reloadResult.error.message });
        }
      } finally {
        creating_task_.as(false);
      }
    },
    async openTask(task) {
      const { id, path, filename } = task;
      if (!id) {
        WXU.error({
          msg: "task id is empty",
        });
        return;
      }
      if (WXU.config.remoteServerEnabled || WXU.config.inDocker) {
        var u = DownloadHostname + "/preview?id=" + id;
        window.open(u);
        return;
      }
      reqs.file.show.run({ path, name: filename, id });
    },
    connect() {
      return new Promise((resolve, reject) => {
        const ws = new WebSocket(WXEnv.downloaderWSURL);
        ws.onopen = () => {
          if (WXU.downloader) {
            WXU.downloader.status = "connected";
          }
          resolve(true);
        };
        ws.onclose = () => {
          WXU.error({ msg: "download ws连接已关闭，请刷新页面" });
          if (WXU.downloader) {
            WXU.downloader.status = "disconnected";
          }
        };
        ws.onerror = (e) => {
          if (WXU.downloader && WXU.downloader.status !== "connected") {
            reject(e);
          }
        };
        ws.onmessage = (ev) => {
          const [err, msg] = WXU.parseJSON(ev.data);
          if (err) {
            return;
          }
          if (msg.type === "task_upsert") {
            const tasks = msg.tasks;
            if (tasks && tasks.length) {
              tasks.forEach((task) => {
                if (task && task.id) {
                  methods.upsert(methods.formatTask(task));
                }
              });
            }
            return;
          }
          if (msg.type === "task_delete") {
            const tasks = msg.tasks;
            if (tasks && tasks.length) {
              const taskIds = [];
              tasks.forEach((t) => {
                if (t && t.id) taskIds.push(t.id);
              });
              if (taskIds.length) {
                applyDeletedTaskIds(taskIds);
              }
            }
            return;
          }
          if (msg.type === "task_snapshot") {
            // V1 task progress snapshot
            const taskId = msg.task_id;
            if (!taskId) {
              return;
            }
            // Convert V1 snapshot to task object the UI expects
            const snapshot = {
              id: taskId,
              status: msg.status,
              name: msg.name || "",
            };
            if (msg.resources && msg.resources.length > 0) {
              const totalSize = msg.resources.reduce(
                (sum, r) => sum + (r.size || 0),
                0,
              );
              const totalDownloaded = msg.resources.reduce(
                (sum, r) => sum + (r.downloaded || 0),
                0,
              );
              const totalSpeed = msg.resources.reduce(
                (sum, r) => sum + (r.speed || 0),
                0,
              );
              snapshot.progress = {
                downloaded: totalDownloaded,
                size: totalSize,
                speed: totalSpeed,
              };
            }
            methods.upsert(methods.formatTask(snapshot));
            return;
          }
          if (msg.type === "batch_tasks") {
            const list = Array.isArray(msg.data) ? msg.data : [];
            const tasks = list.map((t) => methods.formatTask(t));
            methods.batchInsert(tasks);
            return;
          }
          if (msg.type === "event") {
            const data = msg && msg.data ? msg.data : null;
            if (!data) {
              return;
            }
            if (data.status_counts) {
              setStatusCounts(data.status_counts);
            }
            const key = data.Key || data.key || "";
            if (key === "delete") {
              return;
            }
            const task = data.Task || data.task; // Handle case-insensitive field names
            if (!task) {
              return;
            }
            const errMsg = data.Err || data.err || "";
            if (errMsg && key === "error") {
              task._errMsg = errMsg;
            }
            if (key === "start") {
              task._errMsg = "";
            }
            methods.upsert(methods.formatTask(task), {
              prepend: key === "create" || !key,
            });
          }
        };
      });
    },
    batchInsert(tasks) {
      if (!tasks || !tasks.length) return;
      const counts = count_download_statuses(tasks);
      const filteredTasks = tasks.filter((task) =>
        matchesActiveStatusFilter(task),
      );
      if (tasks.length > _pageSize || tasks.length >= (virtual_total || 0)) {
        virtual_total = filteredTasks.length;
        loaded_pages.clear();
        loading_pages.clear();
        pending_pages.clear();
        for (
          let page = 1;
          page <= Math.ceil(filteredTasks.length / _pageSize);
          page += 1
        ) {
          loaded_pages.add(page);
        }
        setStatusCounts(counts, filteredTasks.length);
        tasks_.as(sortTaskSlots(filteredTasks, filteredTasks.length));
        return;
      }
      const taskById = new Map();
      for (let i = 0; i < tasks.length; i++) {
        const t = tasks[i];
        if (!t || !t.id) continue;
        taskById.set(t.id, t);
      }
      if (taskById.size) {
        let changed = false;
        const next = (tasks_.value || []).map((task) => {
          if (!isLoadedTask(task)) {
            return task;
          }
          const replacement = taskById.get(task.id);
          if (!replacement) {
            return task;
          }
          changed = true;
          return replacement;
        });
        if (changed) {
          tasks_.as(
            sortTaskSlots(
              next.filter(matchesActiveStatusFilter),
              virtual_total,
            ),
          );
        }
      }
    },
    upsert(task, options) {
      if (!task || !task.id) {
        return;
      }
      const current = tasks_.value || [];
      const matchesFilter = matchesActiveStatusFilter(task);
      const index = current.findIndex(
        (v) => isLoadedTask(v) && String(v.id) === String(task.id),
      );
      if (index === -1) {
        if (!matchesFilter) {
          return;
        }
        if (
          (!options || !options.prepend) &&
          getActiveStatusFilter() === "all"
        ) {
          return;
        }
        // console.log("[]insert task", task);
        const nextTotal =
          (virtual_total || current.length || task_count_.value) + 1;
        const next = [task, ...current].slice(0, nextTotal);
        while (next.length < nextTotal) {
          next.push(makeTaskPlaceholder(next.length));
        }
        for (let i = 0; i < next.length; i += 1) {
          if (isPlaceholderTask(next[i])) {
            next[i] = makeTaskPlaceholder(i);
          }
        }
        virtual_total = nextTotal;
        loaded_pages.clear();
        loading_pages.clear();
        pending_pages.clear();
        task_count_.as(nextTotal);
        tasks_.as(sortTaskSlots(next, nextTotal));
        adjustStatusCounts("", task.status, 1);
        setTimeout(maybeLoadMoreTasks, 0);
        return;
      }
      // console.log("[]update task", task);
      const oldStatus = current[index].status || "";
      const next = current.slice();
      const merged = Object.assign({}, current[index], task);
      if (!merged.name && current[index].name) {
        merged.name = current[index].name;
      }
      if (!merged.path && current[index].path) {
        merged.path = current[index].path;
      }
      if (!merged.filepath && current[index].filepath) {
        merged.filepath = current[index].filepath;
      }
      next[index] = merged;
      if (matchesFilter) {
        tasks_.as(sortTaskSlots(next, virtual_total || next.length));
      } else {
        const nextTotal = Math.max(0, (virtual_total || current.length) - 1);
        virtual_total = nextTotal;
        loaded_pages.clear();
        loading_pages.clear();
        pending_pages.clear();
        task_count_.as(nextTotal);
        tasks_.as(
          sortTaskSlots(
            next.filter((item) => !(isLoadedTask(item) && item.id === task.id)),
            nextTotal,
          ),
        );
        removeSelectedTaskIds([task.id]);
        setTimeout(maybeLoadMoreTasks, 0);
      }
      if (
        normalize_download_status(oldStatus) !==
        normalize_download_status(task.status)
      ) {
        adjustStatusCounts(oldStatus, task.status, 0);
      }
    },
    setListViewElement(event) {
      list_view_el = getMountedElement(event);
      bindNativeListViewScroll();
      syncListViewSlotHeights();
      if (list_view_el && _scrollTop > 0) {
        list_view_el.scrollTop = _scrollTop;
      }
      scheduleListViewScrollSync(_scrollTop);
    },
    handleListViewScroll(pos) {
      _scrollTop = getCurrentListScrollTop(pos);
      maybeLoadMoreTasks(_scrollTop);
    },
    isPlaceholderTask(task) {
      return isPlaceholderTask(task);
    },
    ensureTaskPageForIndex(index) {
      ensureTaskPageForIndex(index);
    },
    scheduleListViewSlotHeightSync() {
      if (sync_list_content_height_) {
        scheduleSyncListViewSlotHeights();
      }
    },
    handleClickCheckboxConfirmDeleteFiles() {
      // if (loading_.value) {
      //   return;
      // }
      delete_delete_files_.as((prev) => !prev);
    },
    async createDownloadTask(feeds, opt = {}) {
      WXU.log
        .Info()
        .Str("feed_count", feeds.length)
        .Str("spec", opt.spec)
        .Bool("overwrite", !!opt.overwrite)
        .Bool("duplicate", !!opt.duplicate)
        .Msg("[downloader.create]create");
      var requestBody = {
        objects: feeds.map((feed) => {
          return {
            platform: opt.platform,
            content: feed,
            config: {
              spec: opt.spec,
              suffix: opt.suffix,
              overwrite: !!opt.overwrite,
              duplicate: !!opt.duplicate,
            },
          };
        }),
      };
      var [err, data] = await WXU.request({
        method: "POST",
        url: WXEnv.apiOrigin + "/api/v1/download_task/create",
        body: requestBody,
      });
      if (err && (err.code === 409 || err.status === 409)) {
        duplicated_feed_prepare_download = requestBody;
        ui.overwriteConfirmDialog$.show();
        return [null, { skipped: true }];
      }
      WXU.downloader.show();
      if (err) {
        return [err, null];
      }
      return [null, data];
    },
    createBrowseHistories(feeds, opt) {
      WXU.log
        .Info()
        .Str("file", "download/model.js")
        .Msg("createBrowseHistories");
      var requestBody = {
        objects: feeds.map((feed) => {
          return {
            platform: opt.platform,
            content: feed,
          };
        }),
      };
      WXU.request({
        method: "POST",
        url: WXEnv.apiOrigin + "/api/browse_history/create",
        body: requestBody,
      });
    },
  };

  const dropdownItems = [];
  dropdownItems.push(
    new Timeless.ui.MenuItemCore({
      label: "清空下载记录",
      async onClick() {
        ui.dropdown$.hide();
        ui.clearConfirmDialog$.show();
      },
    }),
  );
  const dropdown$ =
    props.enableDropdownMenu === false
      ? null
      : new Timeless.ui.DropdownMenuCore({
          trigger: "hover",
          align: "end",
          items: dropdownItems,
        });
  const ui = {
    dropdown$,
    importFileDialog$: new Timeless.ui.DialogCore({
      closeable: true,
    }),
    createTaskDialog$: new Timeless.ui.DialogCore({
      closeable: true,
      onOk() {
        methods.confirmCreateTask();
      },
    }),
    createPlatformTaskDialog$: new Timeless.ui.DialogCore({
      closeable: true,
      onOk() {
        methods.confirmCreatePlatformTask();
      },
    }),
    createTaskPreviewDialog$: new Timeless.ui.DialogCore({
      closeable: true,
      onOk() {
        methods.confirmCreateTaskFromPreview();
      },
    }),
    createPlatformTaskPreviewDialog$: new Timeless.ui.DialogCore({
      closeable: true,
      onOk() {
        methods.confirmCreatePlatformTaskFromPreview();
      },
    }),
    input_create_download_task$: new Timeless.ui.InputCore({
      defaultValue: "",
    }),
    deleteConfirmDialog$: new Timeless.ui.DialogCore({
      onOk() {
        methods.confirmDeleteTask();
      },
    }),
    clearConfirmDialog$: new Timeless.ui.DialogCore({
      onOk() {
        methods.confirmClearTasks();
      },
    }),
    overwriteConfirmDialog$: new Timeless.ui.DialogCore({
      async onOk() {
        const action = overwrite_.value.value;
        WXU.log
          .Info()
          .Str("action", action || "")
          .Msg("overwriteConfirmDialog: onOk user selected action type");
        if (!action) {
          WXU.log
            .Warn()
            .Msg("overwriteConfirmDialog: action is empty, aborting retry");
          return;
        }
        if (!duplicated_feed_prepare_download) {
          WXU.log
            .Warn()
            .Msg(
              "overwriteConfirmDialog: duplicated_feed_prepare_download is empty, aborting retry",
            );
          return;
        }

        const obj = duplicated_feed_prepare_download.objects[0];
        const overwrite = action === "overwrite";
        const duplicate = action === "duplicate";
        WXU.log
          .Info()
          .Str("feed_id", obj.content.id || "")
          .Str("action", action)
          .Bool("overwrite", overwrite)
          .Bool("duplicate", duplicate)
          .Str("stored_config", JSON.stringify(obj.config))
          .Msg(
            "overwriteConfirmDialog: preparing to retry creating download task",
          );

        const [err, data] = await methods.createDownloadTask([obj.content], {
          platform: "wxchannels",
          spec: obj.config.spec,
          suffix: obj.config.suffix,
          overwrite,
          duplicate,
        });
        if (err) {
          WXU.log
            .Error()
            .Str("error", err.message || "")
            .Int("code", err.code || 0)
            .Msg("overwriteConfirmDialog: retry creating download task failed");
          WXU.error({ msg: err.message || "创建下载任务失败" });
          return;
        }
        if (data && data.skipped) {
          WXU.log
            .Warn()
            .Msg("overwriteConfirmDialog: retry still returned 409 conflict");
          return;
        }

        WXU.log
          .Info()
          .Msg(
            "overwriteConfirmDialog: retry creating download task succeeded",
          );
        duplicated_feed_prepare_download = null;
        ui.overwriteConfirmDialog$.hide();
        await reloadTasks();
        return;
      },
    }),
  };
  let ready = false;
  return {
    ui,
    state: {
      /** Download task record list */
      tasks: tasks_,
      /** Total number of download tasks */
      task_count: task_count_,
      list_render_enabled: list_render_enabled_,
      /** Total number of currently downloading tasks */
      running_count: running_count_,
      delete_task: delete_task_,
      delete_task_ids: delete_task_ids_,
      pending_delete_task_count: pending_delete_task_count_,
      /** Whether to also delete files */
      delete_delete_files: delete_delete_files_,
      deleting_task: deleting_task_,
      /** List of selected download task IDs */
      selected_task_ids: selected_task_ids_,
      /** Total number of selected download tasks */
      selected_task_count: selected_task_count_,
      clearing_tasks: clearing_tasks_,
      /** Text in the new download task input field */
      create_task_text: create_task_text_,
      /** Auto-detected or manually entered filename */
      create_task_filename: create_task_filename_,
      /** Platform task creation: platform name */
      create_platform_text: create_platform_text_,
      /** Platform task creation: content JSON */
      create_platform_json: create_platform_json_,
      /** Platform task creation: save path */
      create_platform_save_path: create_platform_save_path_,
      /** Platform task creation: filename */
      create_platform_filename: create_platform_filename_,
      /** Platform task creation: also download cover */
      create_platform_download_cover: create_platform_download_cover_,
      /** Creating download task in progress */
      creating_task: creating_task_,
      /** URL task preview data */
      create_task_preview: create_task_preview_,
      /** Platform task preview data */
      create_platform_preview: create_platform_preview_,
      /** Download task count for each status */
      status_counts: status_counts_,
      /** Currently selected status filter */
      active_status: active_status_,
      overwrite: overwrite_,
      fixed_list_height: fixed_list_height_,
      list_item_height: ITEM_HEIGHT,
      list_gutter: GUTTER,
      list_height: LIST_HEIGHT,
      list_size: _pageSize,
      list_buffer: LIST_BUFFER,
      get scrollTop() {
        return _scrollTop;
      },
    },
    methods,
    async ready() {
      if (ready) {
        return;
      }
      WXU.downloader.status = "disconnected";
      WXU.downloader.reconnect = async function () {
        if (WXU.downloader.status === "connected") return true;
        for (let i = 0; i < 3; i++) {
          try {
            await reloadTasks();
            return true;
          } catch (e) {
            console.warn("Reconnect attempt " + (i + 1) + " failed");
            await new Promise((r) => setTimeout(r, 1000));
          }
        }
        return false;
      };
      const r = await reloadTasks();
      if (r.error) {
        WXU.error({
          msg: r.error.message,
        });
        return;
      }
      ready = true;
      methods.connect();
      setTimeout(maybeLoadMoreTasks, 0);
    },
    clean() {
      resetVirtualTasks();
      status_counts_.as(empty_download_status_counts());
    },
  };
}

var downloadermodel$ = DownloaderPanelViewModel({});
WXU.downloader.create = function (feeds, opt) {
  return downloadermodel$.methods.createDownloadTask(feeds, opt);
};
WXU.downloader.browse = function (feeds, opt) {
  return downloadermodel$.methods.createBrowseHistories(feeds, opt);
};

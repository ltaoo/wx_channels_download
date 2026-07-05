/// <reference path="../utils.js" />
/**
 * @file 下载管理核心逻辑、任务列表状态和共用视图
 */
var APIHostname = WXEnv.apiOrigin;
var DownloadHostname = WXEnv.downloadOrigin;

console.log("[]download/core.js - ", WXEnv.apiServerAddr, APIHostname);

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
  const total = t.meta && t.meta.res ? t.meta.res.size : 0;
  const cur = t.progress ? t.progress.downloaded : 0;
  if (!total) return 0;
  return Math.min(100, Math.floor((cur * 100) / total));
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
function get_download_task_icon_name(filename) {
  if (!filename) return "file";
  const ext = String(filename).split(".").pop().toLowerCase();
  if (ext === "mp3") return "file-volume";
  if (ext === "mp4") return "file-play";
  if (["jpg", "jpeg", "png", "gif", "webp"].includes(ext)) {
    return "file-image";
  }
  return "file";
}
function DownloadTaskFileIcon(props) {
  const size = props.size || 32;
  const iconName_ = computed(props.task, (task) => {
    return get_download_task_icon_name(task.name);
  });
  return Match({
    when: iconName_,
    cases: {
      "file-volume"() {
        return Timeless.Icon({ name: "file-volume", size });
      },
      "file-play"() {
        return Timeless.Icon({ name: "file-play", size });
      },
      "file-image"() {
        return Timeless.Icon({ name: "file-image", size });
      },
      file() {
        return Timeless.Icon({ name: "file", size });
      },
    },
  });
}
function Skeleton(props = {}) {
  return View({
    class: ["wx-skeleton", props.class].filter(Boolean).join(" "),
    style: props.style || {},
  });
}
function DownloadTaskSkeletonCard(props = {}) {
  return View(
    {
      class: ["weui-cell wx-dl-item wx-dl-item-skeleton", props.class]
        .filter(Boolean)
        .join(" "),
      style: {
        "box-sizing": "border-box",
      },
    },
    [
      View(
        {
          class: "weui-cell__hd",
          style: {
            "margin-right": "16px",
            width: "50px",
            height: "50px",
            display: "flex",
            "align-items": "center",
            "justify-content": "center",
          },
        },
        [
          Skeleton({
            class: "wx-dl-skeleton-icon",
            style: {
              width: "50px",
              height: "50px",
              "border-radius": "12px",
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__bd",
          style: { "min-width": "0" },
        },
        [
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "74%",
              height: "14px",
              "border-radius": "4px",
            },
          }),
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "52%",
              height: "14px",
              "border-radius": "4px",
              "margin-top": "6px",
            },
          }),
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "34%",
              height: "12px",
              "border-radius": "4px",
              "margin-top": "10px",
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__ft",
          style: {
            display: "flex",
            "align-items": "center",
            gap: "12px",
            "margin-left": "12px",
          },
        },
        [
          Skeleton({
            style: {
              width: "20px",
              height: "20px",
              "border-radius": "50%",
            },
          }),
          Skeleton({
            style: {
              width: "20px",
              height: "20px",
              "border-radius": "50%",
            },
          }),
        ],
      ),
    ],
  );
}
function total_speed(tasks) {
  let sum = 0;
  tasks.forEach((t) => {
    if (
      t.status === "running" &&
      t.progress &&
      typeof t.progress.speed === "number"
    ) {
      sum += t.progress.speed;
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
  const value = String(status || "")
    .trim()
    .toLowerCase();
  if (value === "paused") return "pause";
  if (
    value === "failed" ||
    value === "fail" ||
    value === "failure" ||
    value === "errored"
  ) {
    return "error";
  }
  if (value === "pending" || value === "waiting" || value === "queued") {
    return "wait";
  }
  if (value === "completed" || value === "success" || value === "finished") {
    return "done";
  }
  return value;
}
function is_download_waiting_status(status) {
  const normalized = normalize_download_status(status);
  return DOWNLOAD_WAITING_STATUS_KEYS.includes(normalized);
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
function unwrap_download_task_list_response(data) {
  let source = data;
  for (let i = 0; i < 4; i += 1) {
    if (Array.isArray(source)) {
      return { list: source };
    }
    if (!source || typeof source !== "object") {
      return {};
    }
    if (
      Array.isArray(source.list) ||
      Array.isArray(source.tasks) ||
      Array.isArray(source.List) ||
      Array.isArray(source.Tasks)
    ) {
      return source;
    }
    if (source.data && typeof source.data === "object") {
      source = source.data;
      continue;
    }
    if (source.Data && typeof source.Data === "object") {
      source = source.Data;
      continue;
    }
    return source;
  }
  return source && typeof source === "object" ? source : {};
}
function number_or_fallback(value, fallback) {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}
function normalize_download_task_list_response(data, pageSize) {
  const source = unwrap_download_task_list_response(data);
  const list = Array.isArray(source.list)
    ? source.list
    : Array.isArray(source.tasks)
      ? source.tasks
      : Array.isArray(source.List)
        ? source.List
        : Array.isArray(source.Tasks)
          ? source.Tasks
          : [];
  const normalizedPageSize =
    number_or_fallback(
      source.page_size || source.pageSize || source.PageSize,
      pageSize,
    ) || pageSize;
  const normalizedTotal = number_or_fallback(
    typeof source.total !== "undefined"
      ? source.total
      : typeof source.count !== "undefined"
        ? source.count
        : typeof source.Total !== "undefined"
          ? source.Total
          : list.length,
    list.length,
  );
  return {
    list,
    total: Math.max(0, normalizedTotal),
    page:
      number_or_fallback(
        source.page || source.page_num || source.pageNum || source.Page,
        1,
      ) || 1,
    page_size: normalizedPageSize,
    status_counts:
      source.status_counts ||
      source.statusCounts ||
      source.StatusCounts ||
      null,
  };
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

  const createTaskListReq = () => {
    return new Timeless.RequestCore(
      (params) => request.get("/api/task/list", params),
      {
        client: http_client,
        process(r) {
          if (r.error) {
            return Timeless.Result.Err(r.error);
          }
          const data = normalize_download_task_list_response(r.data, _pageSize);
          const counts =
            data.status_counts || count_download_statuses(data.list);
          setStatusCounts(counts, data.total);
          return Timeless.Result.Ok({
            list: data.list.map((t) => methods.formatTask(t)),
            total: data.total || 0,
            page: data.page || 1,
            page_size: data.page_size || _pageSize,
          });
        },
      },
    );
  };
  const deleteReq = new Timeless.RequestCore(
    (params = {}) => {
      return request.post("/api/task/delete", {
        id: params.id,
        delete_files: !!params.deleteFiles,
      });
    },
    { client: http_client },
  );
  const startReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/start", { id }),
    { client: http_client },
  );
  const startAllReq = new Timeless.RequestCore(
    (params = {}) => {
      const body = {};
      if (params.status && params.status !== "all") {
        body.status = params.status;
      }
      return request.post("/api/task/start_all", body);
    },
    { client: http_client },
  );
  const pauseReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/pause", { id }),
    { client: http_client },
  );
  const pauseAllReq = new Timeless.RequestCore(
    (params = {}) => {
      const body = {};
      if (params.status && params.status !== "all") {
        body.status = params.status;
      }
      return request.post("/api/task/pause_all", body);
    },
    { client: http_client },
  );
  const resumeReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/resume", { id }),
    { client: http_client },
  );
  const showFileReq = new Timeless.RequestCore(
    ({ path, name, id }) => request.post("/api/show_file", { path, name, id }),
    { client: http_client },
  );
  const clearReq = new Timeless.RequestCore(
    (params = {}) => {
      return request.post("/api/task/clear", {
        delete_files: !!params.deleteFiles,
      });
    },
    { client: http_client },
  );
  const createTaskReq = new Timeless.RequestCore(
    (params = {}) => request.post("/api/task/create3", params),
    { client: http_client },
  );

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
  const creating_task_ = ref(false);
  const selected_task_ids_ = refarr([]);
  const running_count_ = computed(tasks_, (t) => {
    return t.filter(
      (v) => normalize_download_status(v && v.status) === "running",
    ).length;
  });
  const status_counts_ = ref(empty_download_status_counts());
  const active_status_ = ref(initial_status);

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
    const textHeight =
      lines * ITEM_TITLE_LINE_HEIGHT +
      ITEM_TITLE_STATUS_GAP +
      ITEM_STATUS_LINE_HEIGHT;
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
        status: "wait",
      }),
      createTaskListReq().run({
        page: 1,
        page_size: pageSize,
        status: "ready",
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
        params.status = activeStatus;
      }
      const r =
        activeStatus === "wait"
          ? await loadWaitingTaskPage(normalizedPage)
          : await createTaskListReq().run(params);
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
          if (!task.meta || !task.meta.opts) {
            return {};
          }
          var p = task.meta.opts.path || "";
          var n = task.meta.opts.name || "";
          var sep = isWin ? "\\" : "/";
          if (!p || !n) {
            return {};
          }
          return {
            path: p,
            name: n,
            filepath: p.endsWith(sep) ? p + n : p + sep + n,
          };
        })(),
      };
      r.height = estimateTaskItemHeight(r);
      return r;
    },
    async startTask(task) {
      const r = await startReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      updateTaskStatus(task.id, "running");
    },
    async pauseTask(task) {
      const r = await pauseReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      updateTaskStatus(task.id, "paused");
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
    requestDeleteTask(task, deleteFiles = false) {
      if (!task || !task.id) {
        WXU.error({ msg: "异常操作" });
        return;
      }
      delete_task_.as(task);
      delete_task_ids_.as([]);
      delete_delete_files_.as(!!deleteFiles);
      ui.deleteConfirmDialog$.show();
    },
    requestDeleteSelectedTasks(deleteFiles = false) {
      const ids = getSelectedTaskIds();
      if (!ids.length) {
        WXU.error({ msg: "请选择要删除的下载任务" });
        return;
      }
      delete_task_.as(null);
      delete_task_ids_.as(ids);
      delete_delete_files_.as(!!deleteFiles);
      ui.deleteConfirmDialog$.show();
    },
    async deleteTask(task, params = {}) {
      const r = await deleteReq.run({
        id: task.id,
        deleteFiles: params.deleteFiles,
      });
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return false;
      }
      const removedCount = applyDeletedTaskIds([task.id]);
      if (!removedCount) {
        WXU.error({ msg: "异常操作" });
        return false;
      }
      return true;
    },
    async deleteTasksByIds(ids, params = {}) {
      const taskIds = uniqueTaskIds(ids);
      if (!taskIds.length) {
        WXU.error({ msg: "请选择要删除的下载任务" });
        return false;
      }
      const deletedIds = [];
      for (let i = 0; i < taskIds.length; i += 1) {
        const id = taskIds[i];
        const r = await deleteReq.run({
          id,
          deleteFiles: params.deleteFiles,
        });
        if (r.error) {
          if (deletedIds.length) {
            applyDeletedTaskIds(deletedIds);
            delete_task_ids_.as(
              taskIds.filter((taskId) => {
                return !deletedIds.some((deletedId) => deletedId === taskId);
              }),
            );
          }
          WXU.error({ msg: r.error.message });
          return false;
        }
        deletedIds.push(id);
      }
      applyDeletedTaskIds(deletedIds);
      return true;
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
      const r = await resumeReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      updateTaskStatus(task.id, "running");
    },
    async startAllTasks() {
      const r = await startAllReq.run({
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
      const r = await pauseAllReq.run({
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
    requestClearTasks(deleteFiles = false) {
      clear_delete_files_.as(!!deleteFiles);
      ui.clearConfirmDialog$.show();
    },
    async clearTasks(params = {}) {
      const r = await clearReq.run(params);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return false;
      }
      resetVirtualTasks();
      status_counts_.as(empty_download_status_counts());
      return true;
    },
    async confirmClearTasks() {
      if (clearing_tasks_.value) {
        return;
      }
      clearing_tasks_.as(true);
      try {
        const ok = await methods.clearTasks({
          deleteFiles: clear_delete_files_.value,
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
      ui.createTaskDialog$.show();
    },
    setCreateTaskText(value) {
      create_task_text_.as(value || "");
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
        const r = await createTaskReq.run({ text });
        if (r.error) {
          WXU.error({ msg: r.error.message });
          return;
        }
        const data = r.data || {};
        const ids = Array.isArray(data.ids) ? data.ids : [];
        const skipped = Array.isArray(data.skipped) ? data.skipped : [];
        const failed = Array.isArray(data.failed) ? data.failed : [];
        if (ids.length > 0) {
          WXU.toast(`已创建 ${ids.length} 个下载任务`);
        } else if (skipped.length > 0 && failed.length === 0) {
          WXU.toast("没有新的下载任务");
        } else if (failed.length > 0) {
          WXU.error({ msg: failed[0].message || "创建任务失败" });
          return;
        }
        create_task_text_.as("");
        ui.createTaskDialog$.hide();
        const reloadResult = await reloadTasks();
        if (reloadResult && reloadResult.error) {
          WXU.error({ msg: reloadResult.error.message });
        }
      } finally {
        creating_task_.as(false);
      }
    },
    async openTask(task) {
      const { id, path, name } = task;
      if (!id) {
        WXU.error({
          msg: "task id is empty",
        });
        return;
      }
      if (WXU.config.remoteServerEnabled) {
        var u = DownloadHostname + "/preview?id=" + id;
        window.open(u);
        return;
      }
      showFileReq.run({ path, name, id });
    },
    connect() {
      return new Promise((resolve, reject) => {
        console.log("connect -----");
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
            const task = data.Task || data.task; // 兼容大小写字段
            if (!task) {
              return;
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
      console.log("[]upsert task", task);
      if (!task || !task.id) {
        return;
      }
      const current = tasks_.value || [];
      const matchesFilter = matchesActiveStatusFilter(task);
      const index = current.findIndex(
        (v) => isLoadedTask(v) && v.id === task.id,
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
        console.log("[]insert task", task);
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
      console.log("[]update task", task);
      const oldStatus = current[index].status || "";
      const next = current.slice();
      next[index] = task;
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
  };

  const dropdown$ =
    props.enableDropdownMenu === false
      ? null
      : new Timeless.ui.DropdownMenuCore({
          trigger: "hover",
          align: "end",
          items: [
            new Timeless.ui.MenuItemCore({
              label: "开始所有任务",
              async onClick() {
                await methods.startAllTasks();
                ui.dropdown$.hide();
              },
            }),
            new Timeless.ui.MenuItemCore({
              label: "暂停所有任务",
              async onClick() {
                await methods.pauseAllTasks();
                ui.dropdown$.hide();
              },
            }),
            new Timeless.ui.MenuItemCore({
              label: "删除选中",
              onClick() {
                ui.dropdown$.hide();
                onRequestClose();
                methods.requestDeleteSelectedTasks(false);
              },
            }),
            new Timeless.ui.MenuItemCore({
              label: "清空下载记录",
              async onClick() {
                ui.dropdown$.hide();
                onRequestClose();
                methods.requestClearTasks(false);
              },
            }),
          ],
        });
  const ui = {
    dropdown$,
    createTaskDialog$: new Timeless.ui.DialogCore({
      closeable: true,
      onOk() {
        methods.confirmCreateTask();
      },
    }),
    deleteConfirmDialog$: new Timeless.ui.DialogCore({
      closeable: true,
    }),
    clearConfirmDialog$: new Timeless.ui.DialogCore({
      closeable: true,
    }),
  };
  let ready = false;
  return {
    ui,
    state: {
      tasks: tasks_,
      task_count: task_count_,
      list_render_enabled: list_render_enabled_,
      running_count: running_count_,
      delete_task: delete_task_,
      delete_task_ids: delete_task_ids_,
      pending_delete_task_count: pending_delete_task_count_,
      delete_delete_files: delete_delete_files_,
      deleting_task: deleting_task_,
      selected_task_ids: selected_task_ids_,
      selected_task_count: selected_task_count_,
      clear_delete_files: clear_delete_files_,
      clearing_tasks: clearing_tasks_,
      create_task_text: create_task_text_,
      creating_task: creating_task_,
      status_counts: status_counts_,
      active_status: active_status_,
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

function DownloadDeleteConfirmDialog(props) {
  const deleteFiles_ = props.deleteFiles;
  const loading_ = props.loading;
  const title = props.title || "删除下载记录";
  const message = props.message || "确定删除下载任务记录？此操作不可恢复。";
  const checkboxLabel = props.checkboxLabel || "同时删除视频文件";
  const cancelText = props.cancelText || "取消";
  const confirmText = props.confirmText || "确认删除";
  const loadingText = props.loadingText || "删除中...";
  const checkboxStyle = computed(deleteFiles_, (checked) => {
    return {
      width: "18px",
      height: "18px",
      "box-sizing": "border-box",
      "border-radius": "4px",
      border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
      background: checked ? "#07C160" : "transparent",
      color: "#fff",
      display: "inline-flex",
      "align-items": "center",
      "justify-content": "center",
      flex: "0 0 auto",
    };
  });

  return Timeless.shadcn.Dialog(
    {
      store: props.store,
      style: {
        "z-index": "10000",
      },
    },
    [
      View({ style: { padding: "20px 20px 16px" } }, [
        View(
          {
            style: {
              "font-size": "17px",
              "font-weight": "600",
              "line-height": "24px",
              "margin-bottom": "8px",
            },
          },
          [title],
        ),
        View(
          {
            style: {
              "font-size": "14px",
              "line-height": "20px",
              color: "var(--weui-FG-1)",
              "margin-bottom": "16px",
            },
          },
          [message],
        ),
        View(
          {
            role: "checkbox",
            tabIndex: "0",
            attributes: {
              "aria-checked": computed(deleteFiles_, (checked) =>
                checked ? "true" : "false",
              ),
            },
            style: {
              display: "flex",
              "align-items": "center",
              gap: "10px",
              padding: "10px 0",
              cursor: "pointer",
              "user-select": "none",
              "font-size": "14px",
              "line-height": "20px",
            },
            onClick() {
              if (loading_.value) {
                return;
              }
              deleteFiles_.as((prev) => !prev);
            },
            onKeyDown(e) {
              if (loading_.value) {
                return;
              }
              if (e.key === " " || e.key === "Enter") {
                e.preventDefault();
                deleteFiles_.as((prev) => !prev);
              }
            },
          },
          [
            View({ style: checkboxStyle }, [
              Show({
                when: deleteFiles_,
                ok() {
                  return [Timeless.Icon({ name: "check", size: 14 })];
                },
              }),
            ]),
            View({}, [checkboxLabel]),
          ],
        ),
      ]),
    ],
  );
}

function ClearTasksConfirmDialog(props) {
  return DownloadDeleteConfirmDialog({
    ...props,
    title: "清空下载记录",
    message: "确定删除全部下载任务记录？此操作不可恢复。",
    confirmText: "确认清空",
    loadingText: "清空中...",
  });
}

function TaskDeleteConfirmDialog(props) {
  const taskCount_ = props.taskCount;
  return DownloadDeleteConfirmDialog({
    ...props,
    title: "删除下载记录",
    message: taskCount_
      ? computed(taskCount_, (count) => {
          return count > 1
            ? `确定删除选中的 ${count} 个下载任务记录？此操作不可恢复。`
            : "确定删除该下载任务记录？此操作不可恢复。";
        })
      : "确定删除该下载任务记录？此操作不可恢复。",
    confirmText: "确认删除",
    loadingText: "删除中...",
  });
}

function CreateDownloadTaskDialog(props) {
  const text_ = props.text;
  const loading_ = props.loading;

  return Timeless.shadcn.Dialog({ store: props.store }, [
    View({ style: { width: "520px", padding: "20px 20px 16px" } }, [
      View(
        {
          style: {
            "font-size": "17px",
            "font-weight": "600",
            "line-height": "24px",
            "margin-bottom": "14px",
          },
        },
        ["创建下载任务"],
      ),
      Timeless.Textarea({
        value: text_,
        disabled: loading_,
        placeholder: "https://example.com/video.mp4",
        attributes: {
          rows: "10",
          spellcheck: "false",
        },
        class: "wx-dl-create-task-textarea wx-dl-dark-scroll",
        onInput(e) {
          const target =
            e && e.target && typeof e.target.get$elm === "function"
              ? e.target.get$elm()
              : e && e.target;
          props.onInput(
            target && typeof target.value === "string" ? target.value : "",
          );
        },
      }),
    ]),
  ]);
}

function DownloadTaskSelectionCheckbox(props) {
  const checked_ = props.checked;
  const size = props.size || 18;
  const boxStyle = computed(checked_, (checked) => {
    return {
      width: `${size}px`,
      height: `${size}px`,
      "box-sizing": "border-box",
      "border-radius": "4px",
      border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
      background: checked ? "#07C160" : "transparent",
      color: "#fff",
      display: "inline-flex",
      "align-items": "center",
      "justify-content": "center",
      flex: "0 0 auto",
    };
  });

  return View(
    {
      role: "checkbox",
      tabIndex: "0",
      attributes: {
        "aria-label": props.ariaLabel || "选择下载任务",
        "aria-checked": computed(checked_, (checked) =>
          checked ? "true" : "false",
        ),
      },
      class: props.class || "",
      style: {
        width: `${size + 4}px`,
        height: `${size + 4}px`,
        display: "inline-flex",
        "align-items": "center",
        "justify-content": "center",
        cursor: "pointer",
        "user-select": "none",
        flex: "0 0 auto",
        ...(props.style || {}),
      },
      onClick(e) {
        if (e && typeof e.stopPropagation === "function") {
          e.stopPropagation();
        }
        if (typeof props.onToggle === "function") {
          props.onToggle(e);
        }
      },
      onKeyDown(e) {
        if (e.key === " " || e.key === "Enter") {
          e.preventDefault();
          if (typeof props.onToggle === "function") {
            props.onToggle(e);
          }
        }
      },
    },
    [
      View({ style: boxStyle }, [
        Show({
          when: checked_,
          ok() {
            return [
              Timeless.Icon({ name: "check", size: Math.max(12, size - 4) }),
            ];
          },
        }),
      ]),
    ],
  );
}

function DownloadTaskCard(props) {
  const vm$ = props.store;
  const task = props.task;
  const running_count_ = vm$.state.running_count;
  const iconSize = "50px";
  const state_ = computed(task, (t) => {
    const pr = format_download_percent(t);
    const normalizedStatus = normalize_download_status(t.status);
    const isPaused = normalizedStatus === "pause";
    const isRunning = normalizedStatus === "running";
    const isFailed = normalizedStatus === "error";
    const isPending = is_download_waiting_status(normalizedStatus);
    const isCompleted =
      normalizedStatus === "done" ||
      (pr === 100 && !isRunning && !isFailed && !isPaused && !isPending);

    let statusText = t.status;
    let statusColor = "var(--weui-FG-1)";
    if (isRunning) {
      const speed = format_download_speed(t.progress ? t.progress.speed : 0);
      statusText = `${speed} • ${pr}%`;
    } else if (isCompleted) {
      statusText = "已完成";
      const total = t.meta && t.meta.res ? t.meta.res.size : 0;
      if (total) {
        statusText = WXU.bytes_to_size(total);
      }
    } else if (isFailed) {
      statusText = "下载失败";
      statusColor = "#FA5151";
    } else if (isPending) {
      statusText = "等待中...";
    } else if (isPaused) {
      statusText = `已暂停 • ${pr}%`;
    }
    return {
      pr,
      isCompleted,
      isPaused,
      isRunning,
      isFailed,
      canResume: isFailed || isPaused,
      statusText,
      statusColor,
    };
  });
  const isOpenExternal = WXEnv.config.remoteServerEnabled === true;
  const radius = 22;
  const circumference = 2 * Math.PI * radius;
  const offset = computed(state_, (d) => {
    return circumference - (d.pr / 100) * circumference;
  });
  const strokeColor = computed(state_, (d) => {
    return d.isPaused ? "#FBC02D" : "#07C160";
  });
  const selected_ = computed(vm$.state.selected_task_ids, (ids) => {
    return (ids || []).some((id) => id === task.id);
  });
  const btnStyle = {
    color: "var(--weui-FG-0)",
    opacity: "0.8",
    "margin-left": "12px",
    cursor: "pointer",
    display: "flex",
    "align-items": "center",
    "justify-content": "center",
  };

  return View(
    {
      class: ["weui-cell wx-dl-item", props.class].filter(Boolean).join(" "),
      style: {
        "box-sizing": "border-box",
      },
    },
    [
      DownloadTaskSelectionCheckbox({
        checked: selected_,
        ariaLabel: "选择下载任务",
        style: {
          "margin-right": "10px",
        },
        onToggle(e) {
          vm$.methods.toggleTaskSelected(task, {
            shiftKey: !!(e && e.shiftKey),
          });
        },
      }),
      View(
        {
          class: "weui-cell__hd",
          style: {
            position: "relative",
            "margin-right": "16px",
            width: iconSize,
            height: iconSize,
            display: "flex",
            "align-items": "center",
            "justify-content": "center",
            color: "var(--weui-FG-0)",
          },
        },
        [
          Show({
            when: computed(state_, (t) => {
              return t.isRunning || t.isPaused;
            }),
            ok() {
              return [
                View(
                  {
                    style: {
                      position: "relative",
                      width: "50px",
                      height: "50px",
                      display: "flex",
                      "align-items": "center",
                      "justify-content": "center",
                    },
                  },
                  [
                    SVG.SVG(
                      {
                        style: {
                          position: "absolute",
                          top: "0",
                          left: "0",
                          transform: "rotate(-90deg)",
                        },
                        attributes: {
                          width: "50",
                          height: "50",
                          viewBox: "0 0 50 50",
                        },
                      },
                      [
                        SVG.Circle({
                          attributes: {
                            cx: "25",
                            cy: "25",
                            r: radius,
                            stroke: "var(--weui-FG-3)",
                            "stroke-width": "3",
                            fill: "none",
                          },
                        }),
                        SVG.Circle({
                          attributes: {
                            cx: "25",
                            cy: "25",
                            r: radius,
                            stroke: strokeColor,
                            "stroke-width": "3",
                            fill: "none",
                            "stroke-dasharray": circumference,
                            "stroke-dashoffset": offset,
                            "stroke-linecap": "round",
                          },
                        }),
                      ],
                    ),
                    View(
                      {
                        style: {
                          position: "relative",
                          "z-index": "1",
                          display: "flex",
                        },
                      },
                      [
                        DownloadTaskFileIcon({
                          task,
                          size: 32,
                        }),
                      ],
                    ),
                  ],
                ),
              ];
            },
            else() {
              return [
                DownloadTaskFileIcon({
                  task,
                  size: 32,
                }),
              ];
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__bd",
          style: { "min-width": "0" },
        },
        [
          View(
            {
              class: "wx-dl-item-title",
              style: {
                color: "var(--weui-FG-0)",
                "font-weight": "500",
                "font-size": "14px",
              },
            },
            [computed(task, (t) => t.name)],
          ),
          View(
            {
              class: "weui-cell__desc",
              style: computed(state_, (d) => {
                return {
                  "margin-top": "4px",
                  color: d.statusColor,
                  "font-size": "12px",
                };
              }),
            },
            [
              computed(state_, (d) => {
                return d.statusText;
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "weui-cell__ft",
          style: {
            display: "flex",
            "align-items": "center",
          },
        },
        [
          Match({
            when: combine(
              {
                state: state_,
                running_count: running_count_,
              },
              (t) => {
                if (t.state.isCompleted) {
                  return 1;
                }
                if (t.state.isRunning) {
                  return 2;
                }
                if (t.state.isPaused) {
                  return 3;
                }
                if (t.state.isFailed) {
                  return 4;
                }
                return 0;
              },
            ),
            cases: {
              1() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-open",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.openTask(task);
                    },
                  },
                  [
                    Show({
                      when: !!isOpenExternal,
                      ok() {
                        return [
                          Timeless.Icon({
                            name: "file-symlink",
                            size: 20,
                          }),
                        ];
                      },
                      else() {
                        return [
                          Timeless.Icon({
                            name: "folder",
                            size: 20,
                          }),
                        ];
                      },
                    }),
                  ],
                );
              },
              2() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-pause",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.pauseTask(task);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "pause",
                      size: 20,
                    }),
                  ],
                );
              },
              3() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-resume",
                    style: computed(running_count_, (t) => {
                      return {
                        ...btnStyle,
                        ...(t > WXU.config.MaxRunning
                          ? {
                              opacity: "0.6",
                              cursor: "not-allowed",
                            }
                          : {}),
                      };
                    }),
                    onClick() {
                      vm$.methods.resumeTask(task);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "play",
                      size: 20,
                    }),
                  ],
                );
              },
              4() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-resume",
                    style: computed(running_count_, (t) => {
                      return {
                        ...btnStyle,
                        ...(t > WXU.config.MaxRunning
                          ? {
                              opacity: "0.6",
                              cursor: "not-allowed",
                            }
                          : {}),
                      };
                    }),
                    onClick() {
                      vm$.methods.resumeTask(task);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "refresh-ccw",
                      size: 20,
                    }),
                  ],
                );
              },
            },
          }),
          View(
            {
              class: "wx-download-item-delete",
              style: btnStyle,
              onClick() {
                vm$.methods.requestDeleteTask(task);
              },
            },
            [
              Timeless.Icon({
                name: "trash2",
                size: 20,
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

function DownloadTaskListView(props) {
  const vm$ = props.store;
  const tasks_ = vm$.state.tasks;
  const listPaddingBottom =
    typeof props.paddingBottom !== "undefined"
      ? props.paddingBottom
      : typeof props.listPaddingBottom !== "undefined"
        ? props.listPaddingBottom
        : typeof props.bottomPadding !== "undefined"
          ? props.bottomPadding
          : 0;

  return View(
    {
      class: props.class || "wx-dl-list wx-dl-dark-scroll",
      style: props.style || {},
    },
    [
      Show({
        when: computed(tasks_, (items) => items.length > 0),
        ok() {
          return Show({
            when: vm$.state.list_render_enabled,
            ok() {
              const listHeightStyle = vm$.state.fixed_list_height
                ? {
                    height: `${vm$.state.list_height}px`,
                    "max-height": `${vm$.state.list_height}px`,
                  }
                : {
                    "max-height": "100%",
                  };
              return [
                VirtualListView({
                  style: {
                    ...listHeightStyle,
                    overflow: "auto",
                    position: "relative",
                    padding: props.padding || "0 12px",
                    "box-sizing": "border-box",
                    "background-color": "transparent",
                    ...(props.listViewStyle || {}),
                  },
                  key: "id",
                  size: props.size || 10,
                  buffer: vm$.state.list_buffer,
                  gutter: vm$.state.list_gutter,
                  itemHeight: vm$.state.list_item_height,
                  paddingBottom: listPaddingBottom,
                  each: tasks_,
                  onMounted(e) {
                    vm$.methods.setListViewElement(e);
                  },
                  onScroll(pos) {
                    vm$.methods.handleListViewScroll(pos);
                  },
                  render(task) {
                    console.log("is render");
                    if (vm$.methods.isPlaceholderTask(task)) {
                      vm$.methods.ensureTaskPageForIndex(task.__index);
                      return DownloadTaskSkeletonCard({
                        class: props.skeletonClass,
                      });
                    }
                    return DownloadTaskCard({
                      store: vm$,
                      task,
                      class: props.itemClass,
                    });
                  },
                }),
              ];
            },
          });
        },
        else() {
          return [
            View(
              {
                class: props.emptyClass || "weui-loadmore weui-loadmore_line",
              },
              [
                View(
                  {
                    class: "weui-loadmore__tips",
                  },
                  ["暂无下载任务"],
                ),
              ],
            ),
          ];
        },
      }),
    ],
  );
}

function DownloaderPanelView(props) {
  const vm$ = props.store;
  const task_count_ = vm$.state.task_count;
  const status_counts_ = vm$.state.status_counts;
  const active_status_ = vm$.state.active_status;
  const selected_task_count_ = vm$.state.selected_task_count;
  const showStatusCounts = props.showStatusCounts === true;
  const renderConfirmDialog = props.renderConfirmDialog !== false;

  return View(
    {
      class: "wx-dl-panel-container",
      onMounted() {
        vm$.ready();
      },
      onUnmounted() {
        // vm$.clean();
      },
    },
    [
      View({ class: "wx-dl-header" }, [
        View({ class: "wx-dl-heading" }, [
          View({ class: "wx-dl-title" }, [
            "Downloads",
            computed(task_count_, (d) => {
              return d > 0 ? `（${d}）` : "";
            }),
          ]),
          Show({
            when: computed(status_counts_, (counts) => {
              return (
                showStatusCounts &&
                normalize_download_status_counts(counts).total > 0
              );
            }),
            ok() {
              return [
                View({ class: "wx-dl-status-counts" }, [
                  ...DOWNLOAD_STATUS_COUNT_ITEMS.map((item) => {
                    return View(
                      {
                        role: "button",
                        tabIndex: "0",
                        attributes: {
                          "aria-pressed": computed(active_status_, (status) =>
                            status === item.key ? "true" : "false",
                          ),
                        },
                        class: computed(active_status_, (status) =>
                          [
                            "wx-dl-status-count",
                            "wx-dl-status-count-filter",
                            status === item.key
                              ? "wx-dl-status-count-active"
                              : "",
                            item.key === "error"
                              ? "wx-dl-status-count-error"
                              : "",
                          ]
                            .filter(Boolean)
                            .join(" "),
                        ),
                        onClick() {
                          vm$.methods.setStatusFilter(item.key);
                        },
                        onKeyDown(e) {
                          if (e.key === " " || e.key === "Enter") {
                            e.preventDefault();
                            vm$.methods.setStatusFilter(item.key);
                          }
                        },
                      },
                      [
                        View({ class: "wx-dl-status-count-label" }, [
                          item.label,
                        ]),
                        View({ class: "wx-dl-status-count-value" }, [
                          computed(status_counts_, (counts) => {
                            return String(
                              get_download_status_count(counts, item),
                            );
                          }),
                        ]),
                      ],
                    );
                  }),
                ]),
              ];
            },
          }),
        ]),
        Show({
          when: computed(selected_task_count_, (count) => count > 0),
          ok() {
            return [
              View(
                {
                  type: "button",
                  style: {
                    height: "28px",
                    display: "inline-flex",
                    "align-items": "center",
                    "justify-content": "center",
                    gap: "4px",
                    padding: "0 8px",
                    border: "1px solid rgba(250,81,81,0.42)",
                    "border-radius": "4px",
                    background: "transparent",
                    color: "#FA5151",
                    cursor: "pointer",
                    "font-size": "12px",
                    "line-height": "18px",
                    "white-space": "nowrap",
                  },
                  onClick() {
                    vm$.methods.requestDeleteSelectedTasks(false);
                  },
                },
                [
                  Timeless.Icon({ name: "trash2", size: 14 }),
                  computed(
                    selected_task_count_,
                    (count) => `删除选中 ${count}`,
                  ),
                ],
              ),
            ];
          },
        }),
        DropdownMenu(
          {
            store: vm$.ui.dropdown$,
          },
          [
            View(
              {
                class: "wx-dl-more-btn",
              },
              [
                Timeless.Icon({
                  name: "ellipsis-vertical",
                  style: { "font-size": "18px" },
                }),
              ],
            ),
          ],
        ),
      ]),
      DownloadTaskListView({
        store: vm$,
        paddingBottom:
          typeof props.listPaddingBottom !== "undefined"
            ? props.listPaddingBottom
            : props.paddingBottom,
      }),
      renderConfirmDialog
        ? TaskDeleteConfirmDialog({
            store: vm$.ui.deleteConfirmDialog$,
            deleteFiles: vm$.state.delete_delete_files,
            loading: vm$.state.deleting_task,
            taskCount: vm$.state.pending_delete_task_count,
            onConfirm() {
              vm$.methods.confirmDeleteTask();
            },
          })
        : null,
      renderConfirmDialog
        ? ClearTasksConfirmDialog({
            store: vm$.ui.clearConfirmDialog$,
            deleteFiles: vm$.state.clear_delete_files,
            loading: vm$.state.clearing_tasks,
            onConfirm() {
              vm$.methods.confirmClearTasks();
            },
          })
        : null,
    ],
  );
}

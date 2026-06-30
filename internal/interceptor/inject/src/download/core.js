/// <reference path="../utils.js" />
/**
 * @file 下载管理核心逻辑、任务列表状态和共用视图
 */
var APIHostname = WXEnv.apiOrigin;
var RemoteAPIHostname = WXEnv.remoteAPIOrigin;

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
const DOWNLOAD_STATUS_COUNT_ITEMS = [
  { key: "ready", label: "未开始" },
  { key: "running", label: "下载中" },
  { key: "wait", label: "排队" },
  { key: "pause", label: "已暂停" },
  { key: "error", label: "失败" },
  { key: "done", label: "已完成" },
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
function format_download_status_counts(counts) {
  const c = normalize_download_status_counts(counts);
  return [
    `未开始 ${c.ready}`,
    `下载中 ${c.running}`,
    `排队 ${c.wait}`,
    `已暂停 ${c.pause}`,
    `失败 ${c.error}`,
    `已完成 ${c.done}`,
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
  const ITEM_HEIGHT = Number(props.itemHeight) || 94;
  const ITEM_TITLE_LINE_HEIGHT = 20;
  const ITEM_STATUS_LINE_HEIGHT = 18;
  const ITEM_VERTICAL_PADDING = 32;
  const ITEM_TITLE_STATUS_GAP = 4;
  const ITEM_MAX_TITLE_LINES = 3;
  const ITEM_TITLE_UNITS_PER_LINE = 34;
  const GUTTER = 12;
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
    (id) => request.post("/api/task/delete", { id }),
    { client: http_client },
  );
  const startReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/start", { id }),
    { client: http_client },
  );
  const startAllReq = new Timeless.RequestCore(
    () => request.post("/api/task/start_all"),
    { client: http_client },
  );
  const pauseReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/pause", { id }),
    { client: http_client },
  );
  const pauseAllReq = new Timeless.RequestCore(
    () => request.post("/api/task/pause_all"),
    { client: http_client },
  );
  const resumeReq = new Timeless.RequestCore(
    (id) => request.post("/api/task/resume", { id }),
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
  const showFileReq = new Timeless.RequestCore(
    ({ path, name, id }) => request.post("/api/show_file", { path, name, id }),
    { client: http_client },
  );

  const tasks_ = refarr([]);
  const task_count_ = ref(0);
  const list_render_enabled_ = ref(true);
  const clear_delete_files_ = ref(false);
  const clearing_tasks_ = ref(false);
  const running_count_ = computed(tasks_, (t) => {
    return t.filter((v) => v.status === "running").length;
  });
  const status_counts_ = ref(empty_download_status_counts());

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
      tasks_.as(next);
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
  let draining_pages = false;
  const getMountedElement = (event) => {
    const target = event && event.target ? event.target : null;
    if (target && typeof target.get$elm === "function") {
      return target.get$elm();
    }
    return target;
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
    if (!sync_list_content_height_) {
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
    tasks_.as(next, options.reset ? { reset: true } : undefined);
    scheduleSyncListViewSlotHeights();
    if (!options.reset) {
      scheduleListViewScrollSync(_scrollTop);
    }
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
      const r = await createTaskListReq().run({
        page: normalizedPage,
        page_size: _pageSize,
      });
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
    const contentHeight =
      getVirtualContentHeight() || domScrollHeight || clientHeight;
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
    async deleteTask(task) {
      const r = await deleteReq.run(task.id);
      if (r.error) {
        WXU.error({ msg: r.error.message });
        return;
      }
      const current = tasks_.value || [];
      const index = current.findIndex(
        (t) => isLoadedTask(t) && t.id === task.id,
      );
      if (index === -1) {
        WXU.error({ msg: "异常操作" });
        return;
      }
      const oldStatus = current[index].status || "";
      const nextTotal = Math.max(0, (virtual_total || current.length) - 1);
      const next = current
        .filter((t) => !(isLoadedTask(t) && t.id === task.id))
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
      tasks_.as(next);
      adjustStatusCounts(oldStatus, "", -1);
      setTimeout(maybeLoadMoreTasks, 0);
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
      const r = await startAllReq.run();
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
      const r = await pauseAllReq.run();
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
    async openTask(task) {
      const { path, name } = task;
      if (!path || !name) {
        WXU.error({
          msg: "path or name is empty",
        });
        return;
      }
      if (WXU.config.remoteServerEnabled) {
        var u = RemoteAPIHostname + "/preview?id=" + task.id;
        window.open(u);
        return;
      }
      showFileReq.run({ path, name, id: task.id });
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
      if (tasks.length > _pageSize || tasks.length >= (virtual_total || 0)) {
        virtual_total = tasks.length;
        loaded_pages.clear();
        loading_pages.clear();
        pending_pages.clear();
        for (
          let page = 1;
          page <= Math.ceil(tasks.length / _pageSize);
          page += 1
        ) {
          loaded_pages.add(page);
        }
        setStatusCounts(counts);
        tasks_.as(tasks);
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
          tasks_.as(next);
        }
      }
    },
    upsert(task, options) {
      console.log("[]upsert task", task);
      if (!task || !task.id) {
        return;
      }
      const current = tasks_.value || [];
      const index = current.findIndex(
        (v) => isLoadedTask(v) && v.id === task.id,
      );
      if (index === -1) {
        if (!options || !options.prepend) {
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
        tasks_.as(next);
        adjustStatusCounts("", task.status, 1);
        setTimeout(maybeLoadMoreTasks, 0);
        return;
      }
      console.log("[]update task", task);
      const oldStatus = current[index].status || "";
      const next = current.slice();
      next[index] = task;
      tasks_.as(next);
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
          // trigger: "hover",
          trigger: "click",
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
      clear_delete_files: clear_delete_files_,
      clearing_tasks: clearing_tasks_,
      status_counts: status_counts_,
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

function ClearTasksConfirmDialog(props) {
  const deleteFiles_ = props.deleteFiles;
  const loading_ = props.loading;
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

  return Dialog(
    {
      store: props.store,
      style: {
        width: "320px",
        "max-width": "calc(100vw - 32px)",
        "box-sizing": "border-box",
        "border-radius": "8px",
        background: "var(--popup-bg-color)",
        color: "var(--weui-FG-0)",
        "box-shadow": "0 8px 28px rgba(0,0,0,0.28)",
        overflow: "hidden",
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
          ["清空下载记录"],
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
          ["确定删除全部下载任务记录？此操作不可恢复。"],
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
              // if (loading_.value) {
              //   return;
              // }
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
            View({}, ["同时删除视频文件"]),
          ],
        ),
      ]),
      View(
        {
          style: {
            display: "flex",
            "border-top": "1px solid var(--weui-DIALOG-LINE-COLOR)",
          },
        },
        [
          View(
            {
              type: "button",
              style: computed(loading_, (loading) => {
                return {
                  flex: "1",
                  height: "48px",
                  border: "0",
                  background: "transparent",
                  color: "var(--weui-FG-0)",
                  "font-size": "16px",
                  cursor: loading ? "not-allowed" : "pointer",
                  opacity: loading ? "0.6" : "1",
                };
              }),
              onClick() {
                if (loading_.value) {
                  return;
                }
                props.store.hide();
              },
            },
            ["取消"],
          ),
          View(
            {
              type: "button",
              style: computed(loading_, (loading) => {
                return {
                  flex: "1",
                  height: "48px",
                  border: "0",
                  "border-left": "1px solid var(--weui-DIALOG-LINE-COLOR)",
                  background: "transparent",
                  color: "#FA5151",
                  "font-size": "16px",
                  "font-weight": "500",
                  cursor: loading ? "not-allowed" : "pointer",
                  opacity: loading ? "0.6" : "1",
                };
              }),
              onClick() {
                if (loading_.value) {
                  return;
                }
                props.onConfirm();
              },
            },
            [
              computed(loading_, (loading) =>
                loading ? "清空中..." : "确认清空",
              ),
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
  const running_count_ = vm$.state.running_count;

  return View(
    {
      class: props.class || "wx-dl-list wx-dl-dark-scroll",
      style: props.style || {},
    },
    [
      Show({
        when: computed(tasks_, (items) => items.length > 0),
        ok() {
          return [
            Show({
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
                  Timeless.ListView({
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
                    each: tasks_,
                    onMounted(e) {
                      vm$.methods.setListViewElement(e);
                    },
                    onScroll(pos) {
                      vm$.methods.handleListViewScroll(pos);
                    },
                    render(task) {
                      if (vm$.methods.isPlaceholderTask(task)) {
                        vm$.methods.ensureTaskPageForIndex(task.__index);
                        return DownloadTaskSkeletonCard({
                          class: props.skeletonClass,
                        });
                      }
                      const iconSize = "50px";
                      const state_ = computed(task, (t) => {
                        // console.log("the task is changed", t.status);
                        const pr = format_download_percent(t);
                        const normalizedStatus = normalize_download_status(
                          t.status,
                        );
                        const isPaused = normalizedStatus === "pause";
                        const isRunning = normalizedStatus === "running";
                        const isFailed = normalizedStatus === "error";
                        const isPending = normalizedStatus === "wait";
                        const isCompleted =
                          normalizedStatus === "done" ||
                          (pr === 100 &&
                            !isRunning &&
                            !isFailed &&
                            !isPaused &&
                            !isPending);

                        let statusText = t.status;
                        let statusColor = "var(--weui-FG-1)";
                        if (isRunning) {
                          const speed = format_download_speed(
                            t.progress ? t.progress.speed : 0,
                          );
                          statusText = `${speed} • ${pr}%`;
                        } else if (isCompleted) {
                          statusText = "已完成";
                          // Calculate size
                          const total =
                            t.meta && t.meta.res ? t.meta.res.size : 0;
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
                      const isOpenExternal = WXU.config.remoteServerEnabled;
                      const radius = 22;
                      const circumference = 2 * Math.PI * radius;
                      const offset = computed(state_, (d) => {
                        return circumference - (d.pr / 100) * circumference;
                      });
                      const strokeColor = computed(state_, (d) => {
                        return d.isPaused ? "#FBC02D" : "#07C160";
                      });

                      return View(
                        {
                          class: ["weui-cell wx-dl-item", props.itemClass]
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
                                                "stroke-dasharray":
                                                  circumference,
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
                            (() => {
                              const btnStyle = {
                                color: "var(--weui-FG-0)",
                                opacity: "0.8",
                                "margin-left": "12px",
                                cursor: "pointer",
                                display: "flex",
                                "align-items": "center",
                                "justify-content": "center",
                              };
                              return [
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
                                    // 场景 1: 已完成 -> 显示打开按钮
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
                                    // 场景 2: 正在运行 -> 显示暂停按钮
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
                                    // 场景 3: 暂停或失败且未达最大并发 -> 显示恢复按钮
                                    3() {
                                      return View(
                                        {
                                          type: "a",
                                          class: "wx-download-item-resume",
                                          style: computed(
                                            running_count_,
                                            (t) => {
                                              return {
                                                ...btnStyle,
                                                ...(t > WXU.config.MaxRunning
                                                  ? {
                                                      opacity: "0.6",
                                                      cursor: "not-allowed",
                                                    }
                                                  : {}),
                                              };
                                            },
                                          ),
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
                                          style: computed(
                                            running_count_,
                                            (t) => {
                                              return {
                                                ...btnStyle,
                                                ...(t > WXU.config.MaxRunning
                                                  ? {
                                                      opacity: "0.6",
                                                      cursor: "not-allowed",
                                                    }
                                                  : {}),
                                              };
                                            },
                                          ),
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
                                      vm$.methods.deleteTask(task);
                                    },
                                  },
                                  [
                                    Timeless.Icon({
                                      name: "trash2",
                                      size: 20,
                                    }),
                                  ],
                                ),
                              ];
                            })(),
                          ),
                        ],
                      );
                    },
                  }),
                ];
              },
            }),
          ];
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
                        class: [
                          "wx-dl-status-count",
                          item.key === "error"
                            ? "wx-dl-status-count-error"
                            : "",
                        ]
                          .filter(Boolean)
                          .join(" "),
                      },
                      [
                        View({ class: "wx-dl-status-count-label" }, [
                          item.label,
                        ]),
                        View({ class: "wx-dl-status-count-value" }, [
                          computed(status_counts_, (counts) => {
                            const c = normalize_download_status_counts(counts);
                            return String(c[item.key] || 0);
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
      DownloadTaskListView({ store: vm$ }),
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

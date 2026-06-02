import {
  deleteDownloadTask,
  fetchDownloadAppConfig,
  fetchDownloadList,
  fetchRemoteDownloadList,
  highlightDownloadFile,
  retryDownloadTask,
  startDownloadTask,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

export const DownloadTaskStatus = {
  Ready: 0,
  Running: 1,
  Paused: 2,
  Wait: 3,
  Done: 4,
  Error: 5,
  Unknown: 6,
};

export const DownloadTaskTabs = {
  All: "all",
  Running: "running",
  Queued: "queued",
  Done: "done",
  Error: "error",
  Paused: "paused",
};

const STATUS_TEXT = {
  [DownloadTaskStatus.Ready]: "待下载",
  [DownloadTaskStatus.Running]: "下载中",
  [DownloadTaskStatus.Paused]: "已暂停",
  [DownloadTaskStatus.Wait]: "排队中",
  [DownloadTaskStatus.Done]: "已完成",
  [DownloadTaskStatus.Error]: "失败",
  [DownloadTaskStatus.Unknown]: "未知",
};

const REMOTE_STATUS_MAP = {
  ready: DownloadTaskStatus.Ready,
  running: DownloadTaskStatus.Running,
  downloading: DownloadTaskStatus.Running,
  pause: DownloadTaskStatus.Paused,
  paused: DownloadTaskStatus.Paused,
  wait: DownloadTaskStatus.Wait,
  waiting: DownloadTaskStatus.Wait,
  pending: DownloadTaskStatus.Wait,
  done: DownloadTaskStatus.Done,
  completed: DownloadTaskStatus.Done,
  success: DownloadTaskStatus.Done,
  finished: DownloadTaskStatus.Done,
  error: DownloadTaskStatus.Error,
  failed: DownloadTaskStatus.Error,
};

const TAB_DEFS = [
  { label: "全部", value: DownloadTaskTabs.All, countKey: "total" },
  { label: "下载中", value: DownloadTaskTabs.Running, countKey: "running" },
  { label: "待下载", value: DownloadTaskTabs.Queued, countKey: "queued" },
  { label: "已完成", value: DownloadTaskTabs.Done, countKey: "done" },
  { label: "失败", value: DownloadTaskTabs.Error, countKey: "error" },
  { label: "已暂停", value: DownloadTaskTabs.Paused, countKey: "paused" },
];

export function formatBytes(value) {
  const n = Number(value || 0);
  if (!n) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = n;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

export function formatDate(value) {
  const n = Number(value || 0);
  if (!n) return "-";
  return new Date(n).toLocaleString();
}

function parseProgress(raw, size) {
  if (!raw) {
    return { downloaded: 0, total: Number(size || 0), speed: 0, percent: 0 };
  }
  try {
    const data = typeof raw === "string" ? JSON.parse(raw) : raw;
    const downloaded = Number(data.downloaded || 0);
    const total = Number(data.total || size || 0);
    return {
      downloaded,
      total,
      speed: Number(data.speed || 0),
      percent: total
        ? Math.min(100, Math.floor((downloaded / total) * 100))
        : 0,
    };
  } catch {
    return { downloaded: 0, total: Number(size || 0), speed: 0, percent: 0 };
  }
}

export function normalizeTask(task) {
  const progress = parseProgress(task.progress, task.size);
  const percent =
    task.status === DownloadTaskStatus.Done ? 100 : progress.percent;
  return {
    ...task,
    progress_info: progress,
    percent,
    status_text: STATUS_TEXT[task.status] || "未知",
    size_text: formatBytes(task.size),
    speed_text: `${formatBytes(progress.speed)}/s`,
    created_at_text: formatDate(task.created_at),
    updated_at_text: formatDate(task.updated_at),
  };
}

function pickRemoteName(task) {
  if (task.name) return task.name;
  if (task.meta?.opts?.name) return task.meta.opts.name;
  if (task.meta?.res?.name) return task.meta.res.name;
  if (Array.isArray(task.meta?.res?.files) && task.meta.res.files.length > 0) {
    return task.meta.res.files[0].name;
  }
  return task.id || "unknown";
}

function normalizeRemoteDate(value) {
  if (!value) return 0;
  const parsed = Date.parse(value);
  if (!Number.isNaN(parsed)) return parsed;
  return Number(value || 0);
}

export function normalizeRemoteTask(task) {
  const status =
    REMOTE_STATUS_MAP[String(task.status || "").toLowerCase()] ||
    DownloadTaskStatus.Unknown;
  const size = Number(task.meta?.res?.size || 0);
  const downloaded = Number(task.progress?.downloaded || 0);
  const progress = {
    downloaded,
    total: size,
    speed: Number(task.progress?.speed || 0),
    percent: size ? Math.min(100, Math.floor((downloaded / size) * 100)) : 0,
  };
  const path = task.meta?.opts?.path || "";
  const name = pickRemoteName(task);
  return {
    ...task,
    id: task.id,
    task_id: task.id,
    title: name,
    url: task.meta?.req?.url || "",
    filepath: path && name ? `${path.replace(/[\\/]$/, "")}/${name}` : path,
    size,
    status,
    progress_info: progress,
    status_text: STATUS_TEXT[status] || "未知",
    size_text: formatBytes(size),
    speed_text: `${formatBytes(progress.speed)}/s`,
    created_at_text: formatDate(normalizeRemoteDate(task.createdAt)),
    updated_at_text: formatDate(normalizeRemoteDate(task.updatedAt)),
  };
}

function readTaskField(task, lower, upper) {
  if (!task) return undefined;
  return task[lower] ?? task[upper];
}

function normalizeTaskStatus(status) {
  if (typeof status === "number") return status;
  const text = String(status || "").toLowerCase();
  return REMOTE_STATUS_MAP[text] ?? DownloadTaskStatus.Unknown;
}

function normalizeRuntimeDate(value) {
  if (!value) return Date.now();
  if (typeof value === "number") return value;
  const parsed = Date.parse(value);
  if (!Number.isNaN(parsed)) return parsed;
  return Number(value || Date.now());
}

function normalizeRuntimeTask(raw) {
  const meta = readTaskField(raw, "meta", "Meta") || {};
  const req = readTaskField(meta, "req", "Req") || {};
  const opts = readTaskField(meta, "opts", "Opts") || {};
  const res = readTaskField(meta, "res", "Res") || {};
  const progressRaw = readTaskField(raw, "progress", "Progress") || {};
  const id = readTaskField(raw, "id", "ID") || "";
  const name =
    readTaskField(raw, "name", "Name") || opts.name || res.name || id;
  const size = Number(res.size || res.Size || 0);
  const downloaded = Number(
    progressRaw.downloaded || progressRaw.Downloaded || 0,
  );
  const speed = Number(progressRaw.speed || progressRaw.Speed || 0);
  const status = normalizeTaskStatus(readTaskField(raw, "status", "Status"));
  const updatedAt = normalizeRuntimeDate(
    readTaskField(raw, "updatedAt", "UpdatedAt"),
  );
  const createdAt = normalizeRuntimeDate(
    readTaskField(raw, "createdAt", "CreatedAt"),
  );
  const progress = {
    downloaded,
    total: size,
    speed,
    percent:
      status === DownloadTaskStatus.Done
        ? 100
        : size
          ? Math.min(100, Math.floor((downloaded / size) * 100))
          : 0,
  };
  const path = opts.path || opts.Path || "";
  return normalizeTask({
    id,
    task_id: id,
    status,
    title: name,
    name,
    url: req.url || req.URL || "",
    size,
    progress,
    filepath:
      path && name ? `${String(path).replace(/[\\/]$/, "")}/${name}` : path,
    created_at: createdAt,
    updated_at: updatedAt,
  });
}

function normalizeRemoteServerConfig(values) {
  const protocol = values["download.remoteServer.protocol"] || "http";
  const hostname = String(
    values["download.remoteServer.hostname"] || "",
  ).trim();
  const port = Number(values["download.remoteServer.port"] || 0);
  const exists = !!hostname && port > 0;
  return {
    exists,
    label: exists ? `${protocol}://${hostname}:${port}` : "",
  };
}

export function DownloadTaskPanelModel(props = {}) {
  const reqs = {
    list: new Timeless.RequestCore(fetchDownloadList, {
      client: props.client,
    }),
    start: new Timeless.RequestCore(startDownloadTask, {
      client: props.client,
    }),
    retry: new Timeless.RequestCore(retryDownloadTask, {
      client: props.client,
    }),
    remove: new Timeless.RequestCore(deleteDownloadTask, {
      client: props.client,
    }),
    highlight: new Timeless.RequestCore(highlightDownloadFile, {
      client: props.client,
    }),
  };
  const tasks_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = props.pageSize || 20;
  const active_tab_ = ref(DownloadTaskTabs.All);
  const status_counts_ = ref({
    total: 0,
    running: 0,
    queued: 0,
    done: 0,
    error: 0,
    paused: 0,
  });

  function statusesForTab(tab) {
    if (tab === DownloadTaskTabs.Running) return [DownloadTaskStatus.Running];
    if (tab === DownloadTaskTabs.Queued)
      return [DownloadTaskStatus.Ready, DownloadTaskStatus.Wait];
    if (tab === DownloadTaskTabs.Done) return [DownloadTaskStatus.Done];
    if (tab === DownloadTaskTabs.Error) return [DownloadTaskStatus.Error];
    if (tab === DownloadTaskTabs.Paused) return [DownloadTaskStatus.Paused];
    return null;
  }

  function normalizeCounts(data, list) {
    const counts = data?.counts || {};
    const total = Number(
      data?.all_total ?? counts.total ?? data?.total ?? list.length,
    );
    return {
      total,
      running: Number(counts.running || 0),
      queued: Number(counts.queued || 0),
      done: Number(counts.done || 0),
      error: Number(counts.error || 0),
      paused: Number(counts.paused || 0),
    };
  }

  function tabMatchesStatus(tab, status) {
    const statuses = statusesForTab(tab);
    return !statuses || statuses.includes(status);
  }

  function sortTasks(list) {
    const order = {
      [DownloadTaskStatus.Running]: 1,
      [DownloadTaskStatus.Wait]: 2,
      [DownloadTaskStatus.Ready]: 3,
      [DownloadTaskStatus.Paused]: 4,
      [DownloadTaskStatus.Error]: 5,
      [DownloadTaskStatus.Done]: 6,
      [DownloadTaskStatus.Unknown]: 7,
    };
    return [...list].sort((a, b) => {
      if (order[a.status] !== order[b.status])
        return order[a.status] - order[b.status];
      return Number(b.updated_at || 0) - Number(a.updated_at || 0);
    });
  }

  function countKeyForStatus(status) {
    if (status === DownloadTaskStatus.Running) return "running";
    if (
      status === DownloadTaskStatus.Ready ||
      status === DownloadTaskStatus.Wait
    )
      return "queued";
    if (status === DownloadTaskStatus.Done) return "done";
    if (status === DownloadTaskStatus.Error) return "error";
    if (status === DownloadTaskStatus.Paused) return "paused";
    return null;
  }

  function updateCounts(
    oldStatus,
    newStatus,
    inserted = false,
    removed = false,
  ) {
    const counts = { ...status_counts_.value };
    if (inserted) counts.total = Number(counts.total || 0) + 1;
    if (removed) counts.total = Math.max(0, Number(counts.total || 0) - 1);
    const oldKey = countKeyForStatus(oldStatus);
    const newKey = countKeyForStatus(newStatus);
    if (oldKey && oldKey !== newKey)
      counts[oldKey] = Math.max(0, Number(counts[oldKey] || 0) - 1);
    if (newKey && (inserted || oldKey !== newKey))
      counts[newKey] = Number(counts[newKey] || 0) + 1;
    status_counts_.as(counts);
  }

  function mergeRuntimeTask(rawTask) {
    if (!rawTask) return;
    const nextTask = normalizeRuntimeTask(rawTask);
    if (!nextTask.task_id) return;
    const current = tasks_.value || [];
    const idx = current.findIndex(
      (task) => task.task_id === nextTask.task_id || task.id === nextTask.id,
    );
    const matches = tabMatchesStatus(active_tab_.value, nextTask.status);
    if (idx >= 0) {
      const prevTask = current[idx];
      const merged = normalizeTask({
        ...prevTask,
        ...nextTask,
        id: prevTask.id || nextTask.id,
        task_id: prevTask.task_id || nextTask.task_id,
        title: prevTask.title || nextTask.title,
        cover_url: prevTask.cover_url || nextTask.cover_url,
        url: prevTask.url || nextTask.url,
        filepath: nextTask.filepath || prevTask.filepath,
      });
      const nextList = matches
        ? current.map((task, i) => (i === idx ? merged : task))
        : current.filter((_, i) => i !== idx);
      tasks_.as(sortTasks(nextList));
      if (!matches) total_.as(Math.max(0, total_.value - 1));
      updateCounts(prevTask.status, merged.status);
      return;
    }
    updateCounts(null, nextTask.status, true);
    if (!matches) return;
    tasks_.as(sortTasks([nextTask, ...current]));
    total_.as(total_.value + 1);
  }

  function removeRuntimeTask(rawTask) {
    const id =
      readTaskField(rawTask, "id", "ID") || rawTask?.task_id || rawTask;
    if (!id) return;
    const current = tasks_.value || [];
    const found = current.find((task) => task.task_id === id || task.id === id);
    tasks_.as(current.filter((task) => task.task_id !== id && task.id !== id));
    if (found) {
      total_.as(Math.max(0, total_.value - 1));
      updateCounts(found.status, null, false, true);
    }
  }

  function handleWSMessage(message) {
    if (!message || !message.type) return;
    if (message.type === "event") {
      const data = message.data || {};
      mergeRuntimeTask(data.Task || data.task);
      return;
    }
    if (message.type === "batch_tasks" || message.type === "tasks") {
      const data = message.data || {};
      const list = Array.isArray(data) ? data : data.list || [];
      for (const task of list) mergeRuntimeTask(task);
      return;
    }
    if (message.type === "clear") {
      tasks_.as([]);
      total_.as(0);
      status_counts_.as({
        total: 0,
        running: 0,
        queued: 0,
        done: 0,
        error: 0,
        paused: 0,
      });
      return;
    }
    if (message.type === "delete") {
      removeRuntimeTask(
        message.data?.task || message.data?.Task || message.data,
      );
    }
  }

  let ws_ = null;
  let ws_reconnect_timer_ = null;
  let ws_closed_ = false;

  function getDownloaderHostname() {
    return props.hostname || props.client?.hostname || window.location.origin;
  }

  function connectWS() {
    if (ws_ || typeof WebSocket === "undefined") return;
    ws_closed_ = false;
    const wsURL = new URL(getDownloaderHostname());
    wsURL.protocol = wsURL.protocol === "https:" ? "wss:" : "ws:";
    wsURL.pathname = "/ws/downloader";
    wsURL.search = "";
    const ws = new WebSocket(wsURL.toString());
    ws_ = ws;
    ws.onmessage = (ev) => {
      try {
        const message = JSON.parse(ev.data);
        handleWSMessage(message);
      } catch {
        return;
      }
    };
    ws.onclose = () => {
      if (ws_ === ws) ws_ = null;
      if (!ws_closed_) {
        ws_reconnect_timer_ = window.setTimeout(connectWS, 2000);
      }
    };
    ws.onerror = () => {
      ws.close();
    };
  }

  function closeWS() {
    ws_closed_ = true;
    if (ws_reconnect_timer_) {
      window.clearTimeout(ws_reconnect_timer_);
      ws_reconnect_timer_ = null;
    }
    if (ws_) {
      ws_.close();
      ws_ = null;
    }
  }

  async function load(page = 1) {
    loading_.as(true);
    error_.as("");
    const body = { page, pageSize: page_size_ };
    const statuses = statusesForTab(active_tab_.value);
    if (statuses) {
      body.status = statuses;
    }
    const r = await reqs.list.run(body);
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    const list = (r.data.list || []).map(normalizeTask);
    status_counts_.as(normalizeCounts(r.data, list));
    if (page === 1) {
      tasks_.as(list);
    } else {
      tasks_.push(...list);
    }
    total_.as(r.data.total || list.length);
    page_.as(page);
  }

  async function act(fn, successText) {
    const r = await fn();
    if (r.error) {
      props.app?.tip?.({
        type: "error",
        text: [r.error.message || String(r.error)],
      });
      return;
    }
    props.app?.tip?.({ type: "success", text: [successText] });
    await load(1);
  }

  return {
    state: {
      tasks: tasks_,
      loading: loading_,
      error: error_,
      total: total_,
      page: page_,
      pageSize: page_size_,
      activeTab: active_tab_,
      statusStats: status_counts_,
      tabs: TAB_DEFS,
      noMore: computed(
        { tasks: tasks_, total: total_ },
        (d) => d.tasks.length >= d.total,
      ),
      runningCount: computed(
        tasks_,
        (list) =>
          list.filter((t) => t.status === DownloadTaskStatus.Running).length,
      ),
      totalSpeed: computed(
        tasks_,
        (list) =>
          formatBytes(
            list.reduce((sum, t) => {
              if (t.status !== DownloadTaskStatus.Running) return sum;
              return sum + Number(t.progress_info?.speed || 0);
            }, 0),
          ) + "/s",
      ),
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
    },
    methods: {
      init() {
        connectWS();
        return load(1);
      },
      destroy() {
        closeWS();
      },
      refresh() {
        return load(1);
      },
      loadMore() {
        if (loading_.value) return null;
        return load(page_.value + 1);
      },
      filter(status) {
        active_tab_.as(status);
        return load(1);
      },
      start(task) {
        return act(
          () => reqs.start.run({ download_task_id: task.id }),
          "已开始下载",
        );
      },
      retry(task) {
        return act(() => reqs.retry.run({ id: task.id }), "已重试下载");
      },
      remove(task) {
        return act(
          () => reqs.remove.run({ download_task_id: task.id }),
          "已删除下载任务",
        );
      },
      openFile(task) {
        if (!task.filepath) {
          props.app?.tip?.({ type: "warning", text: ["该任务还没有文件路径"] });
          return null;
        }
        return act(
          () => reqs.highlight.run({ file_path: task.filepath }),
          "已定位文件",
        );
      },
      play(task) {
        window.open(`/api/download_task/play?id=${task.id}`, "_blank");
      },
    },
  };
}

function remoteStatusForTab(tab) {
  if (tab === DownloadTaskTabs.Running) return "running";
  if (tab === DownloadTaskTabs.Queued) return "ready";
  if (tab === DownloadTaskTabs.Done) return "done";
  if (tab === DownloadTaskTabs.Error) return "error";
  if (tab === DownloadTaskTabs.Paused) return "paused";
  return null;
}

export function DownloadsPageModel(props) {
  const taskPanel = DownloadTaskPanelModel({
    ...props,
    client: api_client$,
  });
  const reqs = {
    config: new Timeless.RequestCore(fetchDownloadAppConfig, {
      client: props.client,
    }),
    remoteList: new Timeless.RequestCore(fetchRemoteDownloadList, {
      client: props.client,
    }),
  };
  const remote_tasks_ = refarr([]);
  const remote_loading_ = ref(false);
  const remote_error_ = ref("");
  const remote_total_ = ref(0);
  const remote_page_ = ref(1);
  const remote_enabled_ = ref(false);
  const remote_label_ = ref("");

  async function loadRemote(page = 1) {
    if (!remote_enabled_.value) return;
    remote_loading_.as(true);
    remote_error_.as("");
    const params = { page, page_size: taskPanel.state.pageSize };
    const status = remoteStatusForTab(taskPanel.state.activeTab.value);
    if (status) {
      params.status = status;
    }
    const r = await reqs.remoteList.run(params);
    remote_loading_.as(false);
    if (r.error) {
      remote_error_.as(r.error.message || String(r.error));
      return;
    }
    const list = (r.data.list || []).map(normalizeRemoteTask);
    if (page === 1) {
      remote_tasks_.as(list);
    } else {
      remote_tasks_.push(...list);
    }
    remote_total_.as(r.data.total || list.length);
    remote_page_.as(page);
  }

  async function loadConfig() {
    const r = await reqs.config.run();
    if (r.error) {
      return;
    }
    const values = r.data?.values || {};
    const remoteServer = normalizeRemoteServerConfig(values);
    remote_enabled_.as(remoteServer.exists);
    remote_label_.as(remoteServer.label);
    if (remoteServer.exists) {
      await loadRemote(1);
    }
  }

  return {
    state: {
      ...taskPanel.state,
      remoteTasks: remote_tasks_,
      remoteLoading: remote_loading_,
      remoteError: remote_error_,
      remoteTotal: remote_total_,
      remoteEnabled: remote_enabled_,
      remoteLabel: remote_label_,
      remoteNoMore: computed(
        { tasks: remote_tasks_, total: remote_total_ },
        (d) => d.tasks.length >= d.total,
      ),
      remoteRunningCount: computed(
        remote_tasks_,
        (list) =>
          list.filter((t) => t.status === DownloadTaskStatus.Running).length,
      ),
      remoteTotalSpeed: computed(
        remote_tasks_,
        (list) =>
          formatBytes(
            list.reduce((sum, t) => {
              if (t.status !== DownloadTaskStatus.Running) return sum;
              return sum + Number(t.progress_info?.speed || 0);
            }, 0),
          ) + "/s",
      ),
    },
    ui: taskPanel.ui,
    methods: {
      ...taskPanel.methods,
      init() {
        loadConfig();
        return taskPanel.methods.init();
      },
      refresh() {
        loadRemote(1);
        return taskPanel.methods.refresh();
      },
      filter(status) {
        const result = taskPanel.methods.filter(status);
        loadRemote(1);
        return result;
      },
      loadMoreRemote() {
        if (remote_loading_.value) return null;
        return loadRemote(remote_page_.value + 1);
      },
    },
  };
}

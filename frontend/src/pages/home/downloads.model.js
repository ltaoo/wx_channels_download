import {
  deleteDownloadTask,
  fetchDownloadAppConfig,
  fetchDownloadList,
  fetchDownloadProfile,
  fetchRemoteDownloadList,
  highlightDownloadFile,
  openURL,
  pauseDownloadTask,
  resumeTaskPipeline,
  resumeDownloadTask,
  retryDownloadTask,
  retryDownloadTaskChildren,
  startDownloadTask,
  startTaskPipeline,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

/** V1 下载任务状态 */
export const DownloadTaskStatus = {
  Waiting: 0,
  Preparing: 1,
  Downloading: 2,
  Paused: 3,
  Merging: 4,
  Finished: 5,
  Failed: 6,
  Cancelled: 7,
  Unknown: 99,
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
  [DownloadTaskStatus.Waiting]: "等待中",
  [DownloadTaskStatus.Preparing]: "准备中",
  [DownloadTaskStatus.Downloading]: "下载中",
  [DownloadTaskStatus.Paused]: "已暂停",
  [DownloadTaskStatus.Merging]: "合并中",
  [DownloadTaskStatus.Finished]: "已完成",
  [DownloadTaskStatus.Failed]: "失败",
  [DownloadTaskStatus.Cancelled]: "已取消",
  [DownloadTaskStatus.Unknown]: "未知",
};

const REMOTE_STATUS_MAP = {
  ready: DownloadTaskStatus.Waiting,
  running: DownloadTaskStatus.Downloading,
  downloading: DownloadTaskStatus.Downloading,
  pause: DownloadTaskStatus.Paused,
  paused: DownloadTaskStatus.Paused,
  wait: DownloadTaskStatus.Waiting,
  waiting: DownloadTaskStatus.Waiting,
  pending: DownloadTaskStatus.Waiting,
  done: DownloadTaskStatus.Finished,
  completed: DownloadTaskStatus.Finished,
  success: DownloadTaskStatus.Finished,
  finished: DownloadTaskStatus.Finished,
  error: DownloadTaskStatus.Failed,
  failed: DownloadTaskStatus.Failed,
};

const TAB_DEFS = [
  { label: "全部", value: DownloadTaskTabs.All, countKey: "total" },
  { label: "下载中", value: DownloadTaskTabs.Running, countKey: "running" },
  { label: "等待中", value: DownloadTaskTabs.Queued, countKey: "queued" },
  { label: "已完成", value: DownloadTaskTabs.Done, countKey: "done" },
  { label: "失败", value: DownloadTaskTabs.Error, countKey: "error" },
  { label: "已暂停", value: DownloadTaskTabs.Paused, countKey: "paused" },
];

function getAPIClientOrigin() {
  return String(api_client$.hostname || "").trim();
}

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
  if (!n) {
    return "-";
  }
  return new Date(n).toLocaleString();
}

function normalizePercent(value) {
  const percent = Number(value || 0);
  if (!Number.isFinite(percent)) return 0;
  return Math.min(100, Math.max(0, Math.round(percent * 100) / 100));
}

function parseProgress(raw, size, fallbackDownloaded = 0, fallbackSpeed = 0) {
  const fallbackTotal = Number(size || 0);
  const fallback = {
    downloaded: Number(fallbackDownloaded || 0),
    total: fallbackTotal,
    speed: Number(fallbackSpeed || 0),
    percent: 0,
  };
  if (fallbackTotal > 0) {
    fallback.percent = normalizePercent(
      (fallback.downloaded * 100) / fallbackTotal,
    );
  }
  if (typeof raw === "number") {
    fallback.percent = normalizePercent(raw);
    return fallback;
  }
  if (!raw) return fallback;
  try {
    const data = typeof raw === "string" ? JSON.parse(raw) : raw;
    if (typeof data === "number") {
      fallback.percent = normalizePercent(data);
      return fallback;
    }
    const downloaded = Number(data.downloaded || 0);
    const total = Number(data.total || size || 0);
    return {
      downloaded,
      total,
      speed: Number(data.speed || 0),
      percent:
        data.percent != null
          ? normalizePercent(data.percent)
          : total
            ? normalizePercent((downloaded / total) * 100)
            : 0,
    };
  } catch {
    return fallback;
  }
}

function parseJSONValue(value) {
  if (!value) return {};
  if (typeof value === "object") return value;
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

function normalizeFileNodes(files) {
  if (!Array.isArray(files)) return [];
  return files
    .map((file) => {
      const children = normalizeFileNodes(file.children || file.Children);
      const rawProgress = file.progress ?? file.Progress ?? 0;
      const progress =
        typeof rawProgress === "object"
          ? Number(rawProgress.percent || rawProgress.Percent || 0)
          : Number(rawProgress || 0);
      return {
        id: Number(file.id || file.ID || 0),
        name: String(file.name || file.Name || ""),
        kind: String(file.kind || file.Kind || "file"),
        path: String(file.path || file.Path || ""),
        output_path: String(
          file.output_path ||
            file.outputPath ||
            file.OutputPath ||
            "",
        ),
        type: String(file.type || file.Type || (children.length ? "dir" : "file")),
        size: Number(file.size || file.Size || 0),
        downloaded: Number(file.downloaded || file.Downloaded || 0),
        speed: Number(file.speed || file.Speed || 0),
        progress,
        url: String(file.url || file.URL || ""),
        status: String(file.status || file.Status || ""),
        error: String(file.error || file.Error || ""),
        children,
      };
    })
    .filter((file) => file.name || file.path || file.children.length);
}

function fileNodesSize(files) {
  if (!Array.isArray(files)) return 0;
  return files.reduce((sum, file) => {
    if (file.children.length) return sum + fileNodesSize(file.children);
    return sum + Number(file.size || 0);
  }, 0);
}

function fileNodesCount(files) {
  if (!Array.isArray(files)) return 0;
  return files.reduce((sum, file) => {
    if (file.children.length) return sum + fileNodesCount(file.children);
    return sum + 1;
  }, 0);
}

function fileNodeErrors(files, out = []) {
  if (!Array.isArray(files)) return out;
  for (const file of files) {
    if (file.children.length) {
      fileNodeErrors(file.children, out);
      continue;
    }
    if (file.status === "error" || file.error) {
      out.push({
        path: file.output_path || file.path || file.name,
        error: file.error || "下载失败",
      });
    }
  }
  return out;
}

function isProblemFileStatus(status) {
  const value = String(status || "").toLowerCase();
  return (
    value === "error" ||
    value === "failed" ||
    value === "pause" ||
    value === "paused"
  );
}

function fileNodeProblems(files, out = []) {
  if (!Array.isArray(files)) return out;
  for (const file of files) {
    if (file.children.length) {
      fileNodeProblems(file.children, out);
      continue;
    }
    if (isProblemFileStatus(file.status) || file.error) {
      out.push({
        path: file.output_path || file.path || file.name,
        error:
          file.error ||
          (isProblemFileStatus(file.status) ? "下载已暂停或失败" : "下载异常"),
      });
    }
  }
  return out;
}

function taskEventError(task) {
  const events = task.events || task.Events;
  if (!Array.isArray(events)) return "";
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const event = events[i] || {};
    const type = String(event.type || event.Type || "").toLowerCase();
    if (type !== "error" && type !== "failed" && type !== "fail") continue;
    const message = event.message || event.Message;
    if (message) return String(message);
    const data = parseJSONValue(event.data || event.Data);
    const error = data.error || data.Error || data.err || data.Err;
    if (error) return String(error);
  }
  return "";
}

export function normalizeTask(task) {
  // V1: 解析 config_json 获取平台元数据
  const config = parseJSONValue(task.config_json || task.ConfigJSON);
  const metadata = parseJSONValue(task.metadata || task.Metadata);
  const metadata2 = parseJSONValue(task.metadata2 || task.Metadata2);
  const taskMetadata = Object.keys(metadata).length ? metadata : (Object.keys(metadata2).length ? metadata2 : config);
  const files = normalizeFileNodes(
    task.files || task.Files || taskMetadata.files || taskMetadata.Files,
  );
  const fileErrors = fileNodeErrors(files);
  const fileProblems = fileNodeProblems(files);
  const subtasks = normalizeSubtasks(task.subtasks || task.Subtasks || task.resources);
  const fileSize = fileNodesSize(files);
  const totalSize = Number(task.size || task.Size || config.size || fileSize || 0);
  const progress = parseProgress(
    task.progress,
    totalSize,
    task.downloaded || task.Downloaded,
    task.speed || task.Speed,
  );
  const percent =
    task.status === DownloadTaskStatus.Finished ? 100 : progress.percent;
  const error =
    task.error ||
    task.Error ||
    task.err ||
    task.Err ||
    taskEventError(task) ||
    (task.status === DownloadTaskStatus.Failed ? fileErrors[0]?.error : "") ||
    "";
  return {
    ...task,
    id: task.id || task.ID || 0,
    task_id: String(task.id || task.ID || ""),
    title: task.name || task.Name || task.title || task.Title || "unknown",
    name: task.name || task.Name || task.title || task.Title || "unknown",
    url: config.content_url || config.source_url || task.url || task.URL || "",
    cover_url: task.cover_url || task.CoverURL || task.display_cover_url || config.cover_url || "",
    error,
    size: totalSize,
    files,
    subtasks,
    output_path: task.output_path || task.outputPath || task.OutputPath || task.save_path || task.filepath || "",
    file_error_count: fileErrors.length,
    file_error: fileErrors[0] || null,
    file_problem_count: fileProblems.length,
    file_problem: fileProblems[0] || null,
    file_count: Number(
      task.file_count ||
        task.fileCount ||
        taskMetadata.file_count ||
        taskMetadata.fileCount ||
        config.file_count ||
        fileNodesCount(files) ||
        1,
    ),
    progress_info: progress,
    percent,
    status_text: STATUS_TEXT[task.status] || "未知",
    size_text: formatBytes(totalSize),
    speed_text: `${formatBytes(progress.speed)}/s`,
    created_at_text: formatDate(task.created_at),
    updated_at_text: formatDate(task.updated_at),
    display_cover_url: task.cover_url || task.CoverURL || config.cover_url || "",
  };
}

function normalizeSubtasks(subtasks) {
  if (!Array.isArray(subtasks)) return [];
  return subtasks.map((item) => normalizeTask(item));
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
  const files = normalizeFileNodes(task.meta?.res?.files || []);
  const fileErrors = fileNodeErrors(files);
  const fileProblems = fileNodeProblems(files);
  const size = Number(task.meta?.res?.size || fileNodesSize(files) || 0);
  const downloaded = Number(task.progress?.downloaded || 0);
  const progressTotal = Number(task.progress?.total || size || 0);
  const progress = {
    downloaded,
    total: progressTotal,
    speed: Number(task.progress?.speed || 0),
    percent: progressTotal
      ? normalizePercent((downloaded / progressTotal) * 100)
      : 0,
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
    output_path: path && name ? `${path.replace(/[\\/]$/, "")}/${name}` : path,
    size,
    files,
    file_error_count: fileErrors.length,
    file_error: fileErrors[0] || null,
    file_problem_count: fileProblems.length,
    file_problem: fileProblems[0] || null,
    file_count: Number(task.meta?.res?.file_count || fileNodesCount(files)),
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
  const downloadTaskID = Number(
    readTaskField(raw, "download_task_id", "DownloadTaskID") ||
      readTaskField(raw, "downloadTaskID", "downloadTaskId") ||
      readTaskField(raw, "db_id", "DBID") ||
      0,
  );
  const name =
    readTaskField(raw, "name", "Name") || opts.name || res.name || id;
  const files = normalizeFileNodes(res.files || res.Files || []);
  const fileErrors = fileNodeErrors(files);
  const fileProblems = fileNodeProblems(files);
  const size = Number(res.size || res.Size || fileNodesSize(files) || 0);
  const downloaded = Number(
    progressRaw.downloaded || progressRaw.Downloaded || 0,
  );
  const progressTotal = Number(progressRaw.total || progressRaw.Total || size || 0);
  const speed = Number(progressRaw.speed || progressRaw.Speed || 0);
  const status = normalizeTaskStatus(readTaskField(raw, "status", "Status"));
  const error =
    readTaskField(raw, "error", "Error") ||
    readTaskField(raw, "Err", "err") ||
    "";
  const updatedAt = normalizeRuntimeDate(
    readTaskField(raw, "updatedAt", "UpdatedAt"),
  );
  const createdAt = normalizeRuntimeDate(
    readTaskField(raw, "createdAt", "CreatedAt"),
  );
  const progress = {
    downloaded,
    total: progressTotal,
    speed,
    percent:
      status === DownloadTaskStatus.Finished
        ? 100
        : progressTotal
          ? normalizePercent((downloaded / progressTotal) * 100)
          : 0,
  };
  const path = opts.path || opts.Path || "";
  return normalizeTask({
    id: Number.isFinite(downloadTaskID) && downloadTaskID > 0 ? downloadTaskID : "",
    task_id: id,
    engine_task_id: id,
    status,
    error,
    title: name,
    name,
    url: req.url || req.URL || "",
    size,
    files,
    file_error_count: fileErrors.length,
    file_error: fileErrors[0] || null,
    file_problem_count: fileProblems.length,
    file_problem: fileProblems[0] || null,
    file_count: Number(res.file_count || res.fileCount || fileNodesCount(files)),
    progress,
    filepath:
      path && name ? `${String(path).replace(/[\\/]$/, "")}/${name}` : path,
    output_path:
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

function selectOption(label, value) {
  return new Timeless.ui.SelectItemCore({
    label: String(label || value || ""),
    value: String(value || ""),
  });
}

function variantOptions(probe) {
  const variants = probe?.variants || probe?.Variants || [];
  return variants.map((item) => {
    const id = item.id || item.ID || item.spec || item.Spec || "";
    const label = item.label || item.Label || item.id || item.ID || id;
    const suffix = item.suffix || item.Suffix || "";
    const requires = Array.isArray(item.requires || item.Requires)
      ? item.requires || item.Requires
      : [];
    const extra = requires.length ? ` (${requires.join(", ")})` : "";
    return selectOption(`${label}${suffix ? ` ${suffix}` : ""}${extra}`, id);
  });
}

function findVariant(probe, id) {
  const variants = probe?.variants || probe?.Variants || [];
  return variants.find((item) => String(item.id || item.ID || "") === String(id || ""));
}

/**
 * @param {ViewComponentProps} props
 */
export function DownloadTaskPanelModel(props) {
  const reqs = {
    list: new Timeless.RequestCore(fetchDownloadList, {
      client: props.client,
    }),
    profile: new Timeless.RequestCore(fetchDownloadProfile, {
      client: props.client,
    }),
    start: new Timeless.RequestCore(startDownloadTask, {
      client: props.client,
    }),
    pause: new Timeless.RequestCore(pauseDownloadTask, {
      client: props.client,
    }),
    resume: new Timeless.RequestCore(resumeDownloadTask, {
      client: props.client,
    }),
    retry: new Timeless.RequestCore(retryDownloadTask, {
      client: props.client,
    }),
    retryChildren: new Timeless.RequestCore(retryDownloadTaskChildren, {
      client: props.client,
    }),
    remove: new Timeless.RequestCore(deleteDownloadTask, {
      client: props.client,
    }),
    highlight: new Timeless.RequestCore(highlightDownloadFile, {
      client: props.client,
    }),
    open: new Timeless.RequestCore(openURL, {
      client: props.client,
    }),
    startPipeline: new Timeless.RequestCore(startTaskPipeline, {
      client: props.client,
    }),
    resumePipeline: new Timeless.RequestCore(resumeTaskPipeline, {
      client: props.client,
    }),
  };
  const tasks_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const expanded_task_ids_ = ref({});
  const loading_subtask_ids_ = ref({});
  const subtask_errors_ = ref({});
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = 20;
  const active_tab_ = ref(DownloadTaskTabs.All);
  const status_counts_ = ref({
    total: 0,
    running: 0,
    queued: 0,
    done: 0,
    error: 0,
    paused: 0,
  });
  const create_url_ = ref("");
  const create_probe_ = ref(null);
  const create_content_ = ref(null);
  const create_existing_ = refarr([]);
  const create_form_ = refarr([]);
  const create_probe_raw_ = ref(null);
  const create_error_ = ref("");
  const create_loading_ = ref(false);
  const create_creating_ = ref(false);
  const create_variant_id_ = ref("");
  const create_filename_ = ref("");
  const create_download_cover_ = ref(false);
  let probe_timer_ = null;
  let probe_seq_ = 0;
  let pipeline_ws_ = null;

  function statusesForTab(tab) {
    if (tab === DownloadTaskTabs.Running) return [DownloadTaskStatus.Downloading, DownloadTaskStatus.Preparing, DownloadTaskStatus.Merging];
    if (tab === DownloadTaskTabs.Queued)
      return [DownloadTaskStatus.Waiting];
    if (tab === DownloadTaskTabs.Done) return [DownloadTaskStatus.Finished];
    if (tab === DownloadTaskTabs.Error) return [DownloadTaskStatus.Failed];
    if (tab === DownloadTaskTabs.Paused) return [DownloadTaskStatus.Paused];
    return null;
  }

  function normalizeCounts(data, list) {
    const total = Number(data?.total || list.length);
    // V1: 从列表统计各状态数量
    return {
      total,
      running: list.filter((t) => t.status === DownloadTaskStatus.Downloading || t.status === DownloadTaskStatus.Preparing || t.status === DownloadTaskStatus.Merging).length,
      queued: list.filter((t) => t.status === DownloadTaskStatus.Waiting).length,
      done: list.filter((t) => t.status === DownloadTaskStatus.Finished).length,
      error: list.filter((t) => t.status === DownloadTaskStatus.Failed).length,
      paused: list.filter((t) => t.status === DownloadTaskStatus.Paused).length,
    };
  }

  function tabMatchesStatus(tab, status) {
    const statuses = statusesForTab(tab);
    return !statuses || statuses.includes(status);
  }

  function sortTasks(list) {
    const order = {
      [DownloadTaskStatus.Downloading]: 1,
      [DownloadTaskStatus.Preparing]: 2,
      [DownloadTaskStatus.Merging]: 3,
      [DownloadTaskStatus.Waiting]: 4,
      [DownloadTaskStatus.Paused]: 5,
      [DownloadTaskStatus.Failed]: 6,
      [DownloadTaskStatus.Finished]: 7,
      [DownloadTaskStatus.Cancelled]: 8,
      [DownloadTaskStatus.Unknown]: 9,
    };
    return [...list].sort((a, b) => {
      if (order[a.status] !== order[b.status])
        return order[a.status] - order[b.status];
      return Number(b.updated_at || 0) - Number(a.updated_at || 0);
    });
  }

  function countKeyForStatus(status) {
    if (status === DownloadTaskStatus.Downloading || status === DownloadTaskStatus.Preparing || status === DownloadTaskStatus.Merging) return "running";
    if (status === DownloadTaskStatus.Waiting) return "queued";
    if (status === DownloadTaskStatus.Finished) return "done";
    if (status === DownloadTaskStatus.Failed) return "error";
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

  function runtimeTaskDBID(task) {
    const id = Number(
      task?.id ||
        task?.download_task_id ||
        task?.downloadTaskID ||
        task?.downloadTaskId ||
        0,
    );
    return Number.isFinite(id) && id > 0 ? id : 0;
  }

  function sameRuntimeTask(left, right) {
    const leftDBID = runtimeTaskDBID(left);
    const rightDBID = runtimeTaskDBID(right);
    if (leftDBID && rightDBID) return leftDBID === rightDBID;

    const leftTaskID = String(left?.task_id || left?.taskId || "");
    const rightTaskID = String(right?.task_id || right?.taskId || "");
    if (leftTaskID && rightTaskID && leftTaskID === rightTaskID) return true;

    const leftEngineID = String(left?.engine_task_id || left?.engineTaskID || "");
    const rightEngineID = String(right?.engine_task_id || right?.engineTaskID || "");
    if (leftEngineID && rightEngineID && leftEngineID === rightEngineID) {
      return true;
    }

    const leftPath = String(left?.output_path || left?.filepath || "");
    const rightPath = String(right?.output_path || right?.filepath || "");
    return Boolean(leftPath && rightPath && leftPath === rightPath);
  }

  function mergeRuntimeTask(rawTask) {
    if (!rawTask) return;
    const nextTask = normalizeRuntimeTask(rawTask);
    if (!nextTask.task_id) return;
    const current = tasks_.value || [];
    const idx = current.findIndex((task) => sameRuntimeTask(task, nextTask));
    const matches = tabMatchesStatus(active_tab_.value, nextTask.status);
    if (idx >= 0) {
      const prevTask = current[idx];
      const merged = normalizeTask({
        ...prevTask,
        ...nextTask,
        id: prevTask.id || nextTask.id,
        task_id: nextTask.task_id || prevTask.task_id,
        engine_task_id: nextTask.engine_task_id || prevTask.engine_task_id,
        title: prevTask.title || nextTask.title,
        cover_url: prevTask.cover_url || nextTask.cover_url,
        url: prevTask.url || nextTask.url,
        filepath: nextTask.filepath || prevTask.filepath,
        subtasks:
          Array.isArray(prevTask.subtasks) && prevTask.subtasks.length
            ? prevTask.subtasks
            : nextTask.subtasks,
      });
      const nextList = matches
        ? current.flatMap((task, i) => {
            if (!sameRuntimeTask(task, merged)) return [task];
            return i === idx ? [merged] : [];
          })
        : current.filter((task) => !sameRuntimeTask(task, merged));
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

  function upsertTaskRecord(record) {
    if (!record) return;
    const nextTask = normalizeTask(record);
    const nextID = downloadTaskID(nextTask);
    if (!nextID) return;

    const current = tasks_.value || [];
    const idx = current.findIndex(
      (task) => downloadTaskID(task) === nextID,
    );
    const matches = tabMatchesStatus(active_tab_.value, nextTask.status);
    if (idx >= 0) {
      const previous = current[idx];
      const merged = normalizeTask({
        ...previous,
        ...record,
        files: Array.isArray(record.files) ? record.files : previous.files,
        subtasks: Array.isArray(record.subtasks)
          ? record.subtasks
          : previous.subtasks,
      });
      const nextList = matches
        ? current.map((task, index) => (index === idx ? merged : task))
        : current.filter((_, index) => index !== idx);
      tasks_.as(sortTasks(nextList));
      if (!matches) total_.as(Math.max(0, total_.value - 1));
      updateCounts(previous.status, merged.status);
      return;
    }

    updateCounts(null, nextTask.status, true);
    if (!matches) return;
    tasks_.as(sortTasks([nextTask, ...current]));
    total_.as(total_.value + 1);
  }

  function removeRuntimeTask(rawTask) {
    const id =
      rawTask?.task_id || readTaskField(rawTask, "id", "ID") || rawTask;
    if (!id) return;
    const current = tasks_.value || [];
    const found = current.find((task) => task.task_id === id || task.id === id);
    tasks_.as(current.filter((task) => task.task_id !== id && task.id !== id));
    if (found) {
      total_.as(Math.max(0, total_.value - 1));
      updateCounts(found.status, null, false, true);
    }
  }

  function downloadTaskID(task) {
    // V1: task.id 就是数据库主键
    const id = Number(task?.id || task?.Id || 0);
    return Number.isFinite(id) && id > 0 ? id : 0;
  }

  function taskStateKey(task) {
    const id = downloadTaskID(task);
    if (id) return String(id);
    return String(task?.task_id || task?.task_uid || "");
  }

  function setObjectFlag(ref_, key, value) {
    if (!key) return;
    const next = { ...(ref_.value || {}) };
    if (value) {
      next[key] = value;
    } else {
      delete next[key];
    }
    ref_.as(next);
  }

  function patchTask(key, patcher) {
    if (!key) return;
    const current = tasks_.value || [];
    let changed = false;
    const next = current.map((task) => {
      if (taskStateKey(task) !== key) return task;
      changed = true;
      return patcher(task);
    });
    if (changed) tasks_.as(next);
  }

  function handleWSMessage(message) {
    if (!message || !message.type) return;
    if (message.type === "task_upsert") {
      upsertTaskRecord(message.task);
      return;
    }
    if (message.type === "task_delete") {
      removeRuntimeTask(message.task);
      return;
    }
    if (message.type === "event") {
      const data = message.data || {};
      const task = data.Task || data.task;
      if (task && (data.error || data.Err)) {
        task.error = data.error || data.Err;
      }
      mergeRuntimeTask(task);
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
    return props.client.hostname || window.location.origin;
  }

  function connectWS() {
    if (ws_ || typeof WebSocket === "undefined") return;
    ws_closed_ = false;
    const wsURL = new URL(getDownloaderHostname());
    wsURL.protocol = wsURL.protocol === "https:" ? "wss:" : "ws:";
    wsURL.pathname = "/ws/v1/download_task";
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

  function newPipelineRunID() {
    const rand =
      typeof crypto !== "undefined" && crypto.randomUUID
        ? crypto.randomUUID()
        : `${Date.now()}_${Math.random().toString(36).slice(2)}`;
    return `run_${String(rand).replace(/[^a-zA-Z0-9_.-]/g, "_")}`;
  }

  function pipelineWSURL(runID) {
    const wsURL = new URL(getDownloaderHostname());
    wsURL.protocol = wsURL.protocol === "https:" ? "wss:" : "ws:";
    wsURL.pathname = "/admin";
    wsURL.search = "";
    wsURL.searchParams.set("run_id", runID);
    return wsURL.toString();
  }

  function closePipelineWS() {
    if (pipeline_ws_) {
      pipeline_ws_.close();
      pipeline_ws_ = null;
    }
  }

  function mergeWorkflowMessage(message) {
    if (!message || message.type !== "pipeline_workflow") return;
    const runID = message.run_id || message.runId || message.data?.run_id;
    const currentRunID =
      create_probe_raw_.value?.run_id || create_probe_raw_.value?.probe_id;
    if (runID && currentRunID && runID !== currentRunID) return;
    const workflow = message.data?.workflow || message.workflow;
    if (!workflow) return;
    const current = create_probe_raw_.value || {};
    create_probe_raw_.as({
      ...current,
      run_id: workflow.id || runID || current.run_id,
      probe_id: workflow.id || runID || current.probe_id,
      workflow,
    });
  }

  function connectPipelineWS(runID) {
    closePipelineWS();
    if (!runID || typeof WebSocket === "undefined") return;
    const ws = new WebSocket(pipelineWSURL(runID));
    pipeline_ws_ = ws;
    ws.onmessage = (ev) => {
      try {
        mergeWorkflowMessage(JSON.parse(ev.data));
      } catch {
        return;
      }
    };
    ws.onerror = () => {
      ws.close();
    };
    ws.onclose = () => {
      if (pipeline_ws_ === ws) pipeline_ws_ = null;
    };
  }

  function resetProbe() {
    closePipelineWS();
    create_probe_.as(null);
    create_content_.as(null);
    create_existing_.as([]);
    create_form_.as([]);
    create_probe_raw_.as(null);
    create_error_.as("");
    create_variant_id_.as("");
    create_filename_.as("");
    create_download_cover_.as(false);
    ui.variantSelect.setOptions([]);
    ui.variantSelect.setValue("");
    ui.filenameInput.setValue("");
    ui.downloadCoverCheckbox.as(false);
  }

  function clearCreateDraft() {
    probe_seq_ += 1;
    create_loading_.as(false);
    create_url_.as("");
    ui.createUrlInput.setValue("");
    if (probe_timer_) {
      window.clearTimeout(probe_timer_);
      probe_timer_ = null;
    }
    resetProbe();
  }

  async function runProbe(url) {
    const trimmed = String(url || "").trim();
    if (!trimmed) {
      resetProbe();
      return;
    }
    const seq = ++probe_seq_;
    const runID = newPipelineRunID();
    create_loading_.as(true);
    create_error_.as("");
    create_probe_raw_.as({
      run_id: runID,
      probe_id: runID,
      workflow: {
        id: runID,
        url: trimmed,
        status: "running",
        current_node: "start",
        nodes: [],
      },
    });
    connectPipelineWS(runID);
    const r = await reqs.startPipeline.run({ url: trimmed, run_id: runID });
    if (seq !== probe_seq_) return;
    create_loading_.as(false);
    if (r.error) {
      resetProbe();
      create_error_.as(r.error.message || String(r.error));
      return;
    }
    const probe = r.data?.probe || null;
    const defaults = probe?.defaults || probe?.Defaults || {};
    const defaultVariant = defaults.variant_id || defaults.VariantID || "";
    create_probe_.as(probe);
    create_content_.as(r.data?.content || null);
    create_existing_.as(r.data?.existing || []);
    create_form_.as(r.data?.form || []);
    create_probe_raw_.as(r.data || null);
    closePipelineWS();
    create_variant_id_.as(defaultVariant);
    create_filename_.as(r.data?.content?.title || probe?.title || "");
    ui.variantSelect.setOptions(variantOptions(probe));
    ui.variantSelect.setValue(defaultVariant);
    ui.filenameInput.setValue(r.data?.content?.title || probe?.title || "");
  }

  function scheduleProbe(value) {
    create_url_.as(value);
    if (probe_timer_) {
      window.clearTimeout(probe_timer_);
      probe_timer_ = null;
    }
    probe_timer_ = window.setTimeout(() => runProbe(value), 450);
  }

  async function load(page = 1) {
    loading_.as(true);
    error_.as("");
    const params = { page, page_size: page_size_ };
    const statuses = statusesForTab(active_tab_.value);
    if (statuses && statuses.length >= 1) {
      params.status = statuses.join(",");
    }
    const r = await reqs.list.run(params);
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

  const methods = {
    init() {
      connectWS();
      return load(1);
    },
    destroy() {
      closeWS();
      closePipelineWS();
      if (probe_timer_) {
        window.clearTimeout(probe_timer_);
        probe_timer_ = null;
      }
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
    async toggleSubtasks(task) {
      const key = taskStateKey(task);
      if (!key) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return;
      }
      if (expanded_task_ids_.value?.[key]) {
        setObjectFlag(expanded_task_ids_, key, false);
        return;
      }
      setObjectFlag(expanded_task_ids_, key, true);
      if (
        (Array.isArray(task.files) && task.files.length > 0) ||
        (Array.isArray(task.subtasks) && task.subtasks.length > 0)
      ) {
        return;
      }
      const id = downloadTaskID(task);
      if (!id) {
        setObjectFlag(subtask_errors_, key, "该任务缺少数据库下载任务 ID");
        return;
      }
      setObjectFlag(loading_subtask_ids_, key, true);
      setObjectFlag(subtask_errors_, key, false);
      const r = await reqs.profile.run({ download_task_id: id });
      setObjectFlag(loading_subtask_ids_, key, false);
      if (r.error) {
        setObjectFlag(subtask_errors_, key, r.error.message || String(r.error));
        return;
      }
      const profile = normalizeTask(r.data || {});
      patchTask(key, (current) =>
        normalizeTask({
          ...current,
          ...profile,
          subtasks: profile.subtasks || [],
        }),
      );
    },
    start(task) {
      const id = downloadTaskID(task);
      if (!id) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      return act(() => reqs.start.run({ task_id: id }), "已开始下载");
    },
    pause(task) {
      const id = downloadTaskID(task);
      if (!id) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      return act(() => reqs.pause.run({ task_id: id }), "已暂停下载");
    },
    resume(task) {
      const id = downloadTaskID(task);
      if (!id) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      return act(() => reqs.resume.run({ task_id: id }), "已继续下载");
    },
    retry(task) {
      const id = downloadTaskID(task);
      return act(() => reqs.retry.run({ task_id: id }), "已重试下载");
    },
    retryChildren(task) {
      const id = downloadTaskID(task);
      if (!id) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      return act(
        () => reqs.retryChildren.run({ task_id: id }),
        "已重新开始异常子任务",
      );
    },
    remove(task, deleteFile = false) {
      const id = downloadTaskID(task);
      if (!id) {
        props.app?.tip?.({
          type: "error",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      return act(
        () =>
          reqs.remove.run({
            task_id: id,
            delete_file: !!deleteFile,
          }),
        "已删除下载任务",
      );
    },
    openFile(task) {
      const filePath = task.output_path || task.filepath;
      if (!filePath) {
        props.app?.tip?.({ type: "warning", text: ["该任务还没有文件路径"] });
        return null;
      }
      return act(
        () => reqs.highlight.run({ file_path: filePath }),
        "已定位文件",
      );
    },
    play(task) {
      window.open(`/api/download_task/play?id=${task.id}`, "_blank");
    },
    openInBrowser(task) {
      if (!task.id) {
        props.app?.tip?.({
          type: "warning",
          text: ["该任务缺少数据库下载任务 ID，请刷新列表后重试"],
        });
        return null;
      }
      const url = new URL(
        `/api/download_task/play?id=${encodeURIComponent(task.id)}`,
        getAPIClientOrigin() || window.location.origin,
      ).toString();
      return act(() => reqs.open.run({ url }), "已在浏览器打开");
    },
    async createFromURL() {
      const url = String(create_url_.value || "").trim();
      if (!url) {
        props.app?.tip?.({ type: "warning", text: ["请先输入下载链接"] });
        return;
      }
      const probe = create_probe_.value;
      const variant = findVariant(probe, create_variant_id_.value);
      create_creating_.as(true);
      const r = await reqs.resumePipeline.run({
        url,
        run_id: create_probe_raw_.value?.run_id,
        probe_id: create_probe_raw_.value?.probe_id,
        variant_id: create_variant_id_.value,
        spec: variant?.spec || variant?.Spec || "",
        suffix: variant?.suffix || variant?.Suffix || "",
        filename: create_filename_.value,
        options: {
          variant_id: create_variant_id_.value,
          spec: variant?.spec || variant?.Spec || "",
          suffix: variant?.suffix || variant?.Suffix || "",
          filename: create_filename_.value,
          download_cover: create_download_cover_.value,
        },
        download_cover: create_download_cover_.value,
      });
      create_creating_.as(false);
      if (r.error) {
        props.app?.tip?.({
          type: "error",
          text: [r.error.message || String(r.error)],
        });
        return;
      }
      props.app?.tip?.({ type: "success", text: ["已开始下载"] });
      clearCreateDraft();
      await load(1);
    },
  };

  const ui = {
    view: new Timeless.ui.ScrollViewCore({}),
    btn_refresh$: new Timeless.ui.ButtonCore({
      variant: "outline",
      onClick() {
        methods.refresh();
      },
    }),
    btn_load_more$: new Timeless.ui.ButtonCore({
      variant: "outline",
      // disabled: vm$.state.loading,
      onClick() {
        methods.loadMore();
      },
    }),
    createUrlInput: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "粘贴视频号、抖音、知乎、公众号、YouTube 链接",
      onChange(value) {
        scheduleProbe(value);
      },
    }),
    variantSelect: new Timeless.ui.SelectCore({
      defaultValue: "",
      placeholder: "选择下载内容",
      options: [],
      onChange(value) {
        create_variant_id_.as(value || "");
        const variant = findVariant(create_probe_.value, value);
        if (variant?.suffix || variant?.Suffix) {
          const current = String(create_filename_.value || "");
          const suffix = variant.suffix || variant.Suffix;
          if (current && current.toLowerCase().endsWith(String(suffix).toLowerCase())) {
            create_filename_.as(current.slice(0, -String(suffix).length));
            ui.filenameInput.setValue(create_filename_.value);
          }
        }
      },
    }),
    filenameInput: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "文件名",
      onChange(value) {
        create_filename_.as(value);
      },
    }),
    downloadCoverCheckbox: new Timeless.ui.CheckboxCore({
      defaultValue: false,
      onChange(value) {
        create_download_cover_.as(!!value);
      },
    }),
    btnCreatePlatformTask: new Timeless.ui.ButtonCore({
      onClick() {
        methods.createFromURL();
      },
    }),
  };

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
      expandedTaskIDs: expanded_task_ids_,
      loadingSubtaskIDs: loading_subtask_ids_,
      subtaskErrors: subtask_errors_,
      tabs: TAB_DEFS,
      noMore: combine(
        { tasks: tasks_, total: total_ },
        (d) => d.tasks.length >= d.total,
      ),
      runningCount: computed(tasks_, (list) => {
        return list.filter((t) => t.status === DownloadTaskStatus.Downloading || t.status === DownloadTaskStatus.Preparing)
          .length;
      }),
      totalSpeed: computed(tasks_, (list) => {
        return (
          formatBytes(
            list.reduce((sum, t) => {
              if (t.status !== DownloadTaskStatus.Downloading && t.status !== DownloadTaskStatus.Preparing) return sum;
              return sum + Number(t.progress_info?.speed || 0);
            }, 0),
          ) + "/s"
        );
      }),
      createUrl: create_url_,
      createProbe: create_probe_,
      createContent: create_content_,
      createExisting: create_existing_,
      createForm: create_form_,
      createProbeRaw: create_probe_raw_,
      createError: create_error_,
      createLoading: create_loading_,
      createCreating: create_creating_,
      createVariantID: create_variant_id_,
      createFilename: create_filename_,
      createDownloadCover: create_download_cover_,
    },
    ui,
    methods,
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
      client: api_client$,
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
      remoteNoMore: combine(
        { tasks: remote_tasks_, total: remote_total_ },
        (d) => d.tasks.length >= d.total,
      ),
      remoteRunningCount: computed(
        remote_tasks_,
        (list) =>
          list.filter((t) => t.status === DownloadTaskStatus.Downloading).length,
      ),
      remoteTotalSpeed: computed(
        remote_tasks_,
        (list) =>
          formatBytes(
            list.reduce((sum, t) => {
              if (t.status !== DownloadTaskStatus.Downloading) return sum;
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

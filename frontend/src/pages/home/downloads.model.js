import {
  deleteDownloadTask,
  fetchDownloadList,
  highlightDownloadFile,
  retryDownloadTask,
  startDownloadTask,
} from "@/biz/request.js";

export const DownloadTaskStatus = {
  Ready: 1,
  Running: 2,
  Paused: 3,
  Wait: 4,
  Done: 5,
  Error: 6,
  Unknown: 7,
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
      percent: total ? Math.min(100, Math.floor((downloaded / total) * 100)) : 0,
    };
  } catch {
    return { downloaded: 0, total: Number(size || 0), speed: 0, percent: 0 };
  }
}

export function normalizeTask(task) {
  const progress = parseProgress(task.progress, task.size);
  return {
    ...task,
    progress_info: progress,
    status_text: STATUS_TEXT[task.status] || "未知",
    size_text: formatBytes(task.size),
    speed_text: `${formatBytes(progress.speed)}/s`,
    created_at_text: formatDate(task.created_at),
    updated_at_text: formatDate(task.updated_at),
  };
}

export function DownloadsPageModel(props) {
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
  const page_size_ = 20;
  const active_status_ = ref(null);

  const tabs = [
    { label: "全部", value: null },
    { label: "下载中", value: DownloadTaskStatus.Running },
    { label: "待下载", value: DownloadTaskStatus.Ready },
    { label: "已完成", value: DownloadTaskStatus.Done },
    { label: "失败", value: DownloadTaskStatus.Error },
  ];

  async function load(page = 1) {
    loading_.as(true);
    error_.as("");
    const body = { page, pageSize: page_size_ };
    if (active_status_.value !== null) {
      body.status = active_status_.value;
    }
    const r = await reqs.list.run(body);
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    const list = (r.data.list || []).map(normalizeTask);
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
      props.app.tip?.({ type: "error", text: [r.error.message || String(r.error)] });
      return;
    }
    props.app.tip?.({ type: "success", text: [successText] });
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
      activeStatus: active_status_,
      tabs,
      noMore: computed({ tasks: tasks_, total: total_ }, (d) => d.tasks.length >= d.total),
      runningCount: computed(tasks_, (list) => list.filter((t) => t.status === DownloadTaskStatus.Running).length),
      totalSpeed: computed(tasks_, (list) => formatBytes(list.reduce((sum, t) => {
        if (t.status !== DownloadTaskStatus.Running) return sum;
        return sum + Number(t.progress_info?.speed || 0);
      }, 0)) + "/s"),
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
    },
    methods: {
      init() {
        return load(1);
      },
      refresh() {
        return load(1);
      },
      loadMore() {
        if (loading_.value) return null;
        return load(page_.value + 1);
      },
      filter(status) {
        active_status_.as(status);
        return load(1);
      },
      start(task) {
        return act(() => reqs.start.run({ download_task_id: task.id }), "已开始下载");
      },
      retry(task) {
        return act(() => reqs.retry.run({ id: task.id }), "已重试下载");
      },
      remove(task) {
        return act(() => reqs.remove.run({ download_task_id: task.id }), "已删除下载任务");
      },
      openFile(task) {
        if (!task.filepath) {
          props.app.tip?.({ type: "warning", text: ["该任务还没有文件路径"] });
          return null;
        }
        return act(() => reqs.highlight.run({ file_path: task.filepath }), "已定位文件");
      },
      play(task) {
        window.open(`/api/download_task/play?id=${task.id}`, "_blank");
      },
    },
  };
}

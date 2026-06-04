import {
  fetchAccountList,
  fetchAppStatus,
  fetchBrowseHistoryList,
  fetchDownloadList,
  fetchVideoList,
  resumeTaskPipeline,
  startTaskPipeline,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

function pickList(data) {
  if (Array.isArray(data)) {
    return data;
  }
  if (data && Array.isArray(data.list)) {
    return data.list;
  }
  if (Array.isArray(data?.data?.list)) {
    return data.data.list;
  }
  return [];
}

function pickTotal(data) {
  const total = data?.total ?? data?.data?.total;
  if (total !== undefined && total !== null) return Number(total) || 0;
  return pickList(data).length;
}

function pickCounts(data) {
  return data?.counts || data?.data?.counts || {};
}

function formatBytes(value) {
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

function parseProgress(raw) {
  if (!raw) return {};
  if (typeof raw === "object") return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
}

function sumRunningSpeed(list) {
  return pickList(list).reduce((sum, task) => {
    if (task.status !== 1) return sum;
    return sum + Number(parseProgress(task.progress).speed || 0);
  }, 0);
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
  return variants.find(
    (item) => String(item.id || item.ID || "") === String(id || ""),
  );
}

/**
 * @param {ViewComponentProps} props
 */
export function HomeDashboardPageModel(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: api_client$,
    }),
    videos: new Timeless.RequestCore(fetchVideoList, {
      client: api_client$,
    }),
    browse: new Timeless.RequestCore(fetchBrowseHistoryList, {
      client: api_client$,
    }),
    downloads: new Timeless.RequestCore(fetchDownloadList, {
      client: api_client$,
    }),
    runningDownloads: new Timeless.RequestCore(fetchDownloadList, {
      client: api_client$,
    }),
    status: new Timeless.RequestCore(fetchAppStatus, {
      client: api_client$,
    }),
    resumePipeline: new Timeless.RequestCore(resumeTaskPipeline, {
      client: api_client$,
    }),
    startPipeline: new Timeless.RequestCore(startTaskPipeline, {
      client: api_client$,
    }),
  };
  const loading_ = ref(false);
  const error_ = ref("");
  const taskUrl_ = ref("");
  const creatingTask_ = ref(false);
  const probingTask_ = ref(false);
  const probeError_ = ref("");
  const taskProbe_ = ref(null);
  const taskContent_ = ref(null);
  const taskExisting_ = refarr([]);
  const taskProbeRaw_ = ref(null);
  const taskVariantID_ = ref("");
  const taskFilename_ = ref("");
  const stats_ = ref({
    accounts: 0,
    videos: 0,
    browse: 0,
    downloads: 0,
    runningDownloads: 0,
    totalSpeed: "0 B/s",
  });

  let probeSeq = 0;
  const debounce =
    Timeless.utils?.debounce ||
    ((fn, wait) => {
      let timer = null;
      return (...args) => {
        if (timer) window.clearTimeout(timer);
        timer = window.setTimeout(() => fn(...args), wait);
      };
    });
  const debouncedProbeTask = debounce((value) => {
    methods.probeDownloadTaskURL(value);
  }, 500);

  const ui = {
    view$: new Timeless.ui.ScrollViewCore({}),
    btn_create_task$: new Timeless.ui.ButtonCore({
      // disabled: creatingTask_,
      onClick() {
        methods.createDownloadTaskFromURL();
      },
    }),
    btn_refresh_stats: new Timeless.ui.ButtonCore({
      variant: "outline",
      // disabled: loading_,
      onClick() {
        methods.refresh();
      },
    }),
    taskUrlInput$: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "粘贴视频号、抖音、知乎、公众号、YouTube 链接",
      onChange(value) {
        taskUrl_.as(value);
        debouncedProbeTask(value);
      },
    }),
    taskVariantSelect$: new Timeless.ui.SelectCore({
      defaultValue: "",
      placeholder: "选择下载内容",
      options: [],
      onChange(value) {
        taskVariantID_.as(value || "");
      },
    }),
    taskFilenameInput$: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "文件名",
      onChange(value) {
        taskFilename_.as(value);
      },
    }),
  };

  const methods = {
    async refresh() {
      loading_.as(true);
      error_.as("");
      const [accounts, videos, browse, downloads, runningDownloads] =
        await Promise.all([
          reqs.accounts.run({}),
          reqs.videos.run({ page: 1, pageSize: 1 }),
          reqs.browse.run({}),
          reqs.downloads.run({ page: 1, pageSize: 1 }),
          reqs.runningDownloads.run({ page: 1, pageSize: 200, status: [1] }),
          // reqs.status.run(),
        ]);
      loading_.as(false);

      const errors = [accounts, videos, browse, downloads, runningDownloads]
        .filter((r) => r.error)
        .map((r) => r.error?.message || String(r.error));
      if (errors.length) {
        error_.as(errors[0]);
      }

      stats_.as({
        accounts: accounts.error ? 0 : pickTotal(accounts.data),
        videos: videos.error ? 0 : pickTotal(videos.data),
        browse: browse.error ? 0 : pickTotal(browse.data),
        downloads: downloads.error ? 0 : pickTotal(downloads.data),
        runningDownloads: downloads.error
          ? 0
          : Number(pickCounts(downloads.data).running || 0),
        totalSpeed: runningDownloads.error
          ? "0 B/s"
          : `${formatBytes(sumRunningSpeed(runningDownloads.data))}/s`,
      });
    },
    resetProbe() {
      probingTask_.as(false);
      probeError_.as("");
      taskProbe_.as(null);
      taskContent_.as(null);
      taskExisting_.as([]);
      taskProbeRaw_.as(null);
      taskVariantID_.as("");
      taskFilename_.as("");
      ui.taskVariantSelect$.setOptions([]);
      ui.taskVariantSelect$.setValue("");
      ui.taskFilenameInput$.setValue("");
    },
    async probeDownloadTaskURL(url) {
      const trimmed = String(url || "").trim();
      if (!trimmed) {
        methods.resetProbe();
        return;
      }
      const seq = ++probeSeq;
      probingTask_.as(true);
      probeError_.as("");
      const result = await reqs.startPipeline.run({ url: trimmed });
      if (seq !== probeSeq) return;
      probingTask_.as(false);
      if (result.error) {
        taskProbe_.as(null);
        taskContent_.as(null);
        taskExisting_.as([]);
        taskProbeRaw_.as(null);
        taskVariantID_.as("");
        ui.taskVariantSelect$.setOptions([]);
        ui.taskVariantSelect$.setValue("");
        probeError_.as(result.error.message || String(result.error));
        return;
      }
      const probe = result.data?.probe || null;
      const content = result.data?.content || null;
      const defaults = probe?.defaults || probe?.Defaults || {};
      const defaultVariant = defaults.variant_id || defaults.VariantID || "";
      const defaultFilename = content?.title || probe?.title || "";
      taskProbe_.as(probe);
      taskContent_.as(content);
      taskExisting_.as(result.data?.existing || []);
      taskProbeRaw_.as(result.data || null);
      taskVariantID_.as(defaultVariant);
      taskFilename_.as(defaultFilename);
      ui.taskVariantSelect$.setOptions(variantOptions(probe));
      ui.taskVariantSelect$.setValue(defaultVariant);
      ui.taskFilenameInput$.setValue(defaultFilename);
    },
    async createDownloadTaskFromURL() {
      const url = taskUrl_.value.trim();
      if (!url) {
        props.app.tip?.({ type: "warning", text: ["请输入下载链接"] });
        return;
      }
      if (!taskProbe_.value) {
        props.app.tip?.({ type: "warning", text: ["请等待链接解析完成"] });
        return;
      }
      const variant = findVariant(taskProbe_.value, taskVariantID_.value);
      creatingTask_.as(true);
      const result = await reqs.resumePipeline.run({
        url,
        run_id: taskProbeRaw_.value?.run_id,
        probe_id: taskProbeRaw_.value?.probe_id,
        variant_id: taskVariantID_.value,
        spec: variant?.spec || variant?.Spec || "",
        suffix: variant?.suffix || variant?.Suffix || "",
        filename: taskFilename_.value,
        options: {
          variant_id: taskVariantID_.value,
          spec: variant?.spec || variant?.Spec || "",
          suffix: variant?.suffix || variant?.Suffix || "",
          filename: taskFilename_.value,
        },
      });
      creatingTask_.as(false);
      if (result.error) {
        props.app.tip?.({
          type: "error",
          text: [result.error.message || String(result.error)],
        });
        return;
      }
      props.app.tip?.({
        type: "success",
        text: ["已开始下载"],
      });
      taskUrl_.as("");
      ui.taskUrlInput$.setValue?.("");
      methods.resetProbe();
      await methods.refresh();
    },
  };

  //   return Timeless.defineModel({
  //     state: {},
  //     methods,
  //   });
  return {
    state: {
      loading: loading_,
      error: error_,
      creatingTask: creatingTask_,
      probingTask: probingTask_,
      probeError: probeError_,
      taskProbe: taskProbe_,
      taskContent: taskContent_,
      taskExisting: taskExisting_,
      taskProbeRaw: taskProbeRaw_,
      taskVariantID: taskVariantID_,
      taskFilename: taskFilename_,
      stats: stats_,
    },
    ui,
    methods,
  };
}

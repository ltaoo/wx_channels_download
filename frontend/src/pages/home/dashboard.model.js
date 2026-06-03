import {
  createTask,
  fetchAccountList,
  fetchAppStatus,
  fetchBrowseHistoryList,
  fetchDownloadList,
  fetchVideoList,
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
    status: new Timeless.RequestCore(fetchAppStatus, {
      client: api_client$,
    }),
    createTask: new Timeless.RequestCore(createTask, {
      client: api_client$,
    }),
  };
  const loading_ = ref(false);
  const error_ = ref("");
  const taskUrl_ = ref("");
  const creatingTask_ = ref(false);
  const stats_ = ref({
    accounts: 0,
    videos: 0,
    browse: 0,
    downloads: 0,
  });

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
      placeholder: "粘贴视频号下载链接",
      onChange(value) {
        taskUrl_.as(value);
      },
    }),
    downloadCoverCheckbox$: new Timeless.ui.CheckboxCore({}),
  };

  const methods = {
    async refresh() {
      loading_.as(true);
      error_.as("");
      const [accounts, videos, browse, downloads, status] = await Promise.all([
        reqs.accounts.run({}),
        reqs.videos.run({ page: 1, pageSize: 1 }),
        reqs.browse.run({}),
        reqs.downloads.run({ page: 1, pageSize: 1 }),
        reqs.status.run(),
      ]);
      loading_.as(false);

      const errors = [accounts, videos, browse, downloads, status]
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
      });
    },
    async createDownloadTaskFromURL() {
      const url = taskUrl_.value.trim();
      if (!url) {
        props.app.tip?.({ type: "warning", text: ["请输入下载链接"] });
        return;
      }
      creatingTask_.as(true);
      const result = await reqs.createTask.run({ url });
      const coverResult = ui.downloadCoverCheckbox$.value
        ? await reqs.createTask.run({ url, cover: true })
        : null;
      creatingTask_.as(false);
      if (result.error) {
        props.app.tip?.({
          type: "error",
          text: [result.error.message || String(result.error)],
        });
        return;
      }
      if (coverResult?.error) {
        props.app.tip?.({
          type: "error",
          text: [coverResult.error.message || String(coverResult.error)],
        });
        return;
      }
      props.app.tip?.({
        type: "success",
        text: [coverResult ? "已创建下载任务和封面下载任务" : "已创建下载任务"],
      });
      taskUrl_.as("");
      ui.taskUrlInput$.setValue?.("");
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
      stats: stats_,
    },
    ui,
    methods,
  };
}

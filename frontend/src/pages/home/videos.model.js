import { fetchAccountList, fetchVideoList } from "@/biz/request.js";
import { formatBytes, formatDate } from "./downloads.model.js";

function normalizeVideo(video, account) {
  return {
    ...video,
    id: video.id || video.Id || `${video.external_id1 || video.title}-${Math.random()}`,
    title: video.title || video.Title || video.description || "未命名视频",
    description: video.description || video.Description || "",
    cover_url: video.cover_url || video.CoverURL || video.coverUrl || "",
    url: video.url || video.URL || "",
    file_size: video.file_size || video.size || video.Size || 0,
    duration: video.duration || video.Duration || 0,
    publish_time: video.publish_time || video.PublishTime || 0,
    account,
  };
}

async function fetchVideosFallback(keyword, reqs) {
  const r = await reqs.accounts.run({});
  if (r.error) return r;
  const k = String(keyword || "").trim().toLowerCase();
  const list = [];
  for (const account of r.data.list || []) {
    const acc = {
      nickname: account.nickname,
      username: account.username || account.external_id,
      avatar_url: account.avatar_url,
    };
    for (const row of account.video_accounts || []) {
      if (row.video) {
        const video = normalizeVideo(row.video, acc);
        if (!k || String(video.title || video.description || "").toLowerCase().includes(k)) {
          list.push(video);
        }
      }
    }
  }
  return Timeless.Result.Ok({ list, total: list.length, page: 1, page_size: list.length });
}

export function VideosPageModel(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: props.client,
    }),
    videos: new Timeless.RequestCore(fetchVideoList, {
      client: props.client,
    }),
  };
  const videos_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const total_ = ref(0);
  const page_ = ref(1);
  const keyword_ = ref("");
  const page_size_ = 24;

  async function load(page = 1) {
    loading_.as(true);
    error_.as("");
    const r = await reqs.videos.run({
      page,
      pageSize: page_size_,
      keyword: keyword_.value.trim(),
    });
    let result = r;
    if (!r.error && page === 1 && (!r.data.list || r.data.list.length === 0)) {
      result = await fetchVideosFallback(keyword_.value, reqs);
    }
    loading_.as(false);
    if (result.error) {
      error_.as(result.error.message || String(result.error));
      return;
    }
    const list = (result.data.list || []).map((it) => normalizeVideo(it));
    if (page === 1) {
      videos_.as(list);
    } else {
      videos_.push(...list);
    }
    total_.as(result.data.total || list.length);
    page_.as(page);
  }

  return {
    state: {
      videos: videos_,
      loading: loading_,
      error: error_,
      total: total_,
      keyword: keyword_,
      page: page_,
      noMore: computed({ videos: videos_, total: total_ }, (d) => d.videos.length >= d.total),
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      keyword: new Timeless.ui.InputCore({
        placeholder: "搜索视频标题",
        onChange(value) {
          keyword_.as(value);
        },
      }),
    },
    methods: {
      init() {
        return load(1);
      },
      search() {
        return load(1);
      },
      loadMore() {
        if (loading_.value) return null;
        return load(page_.value + 1);
      },
      play(video) {
        if (!video.url) {
          props.app.tip?.({ type: "warning", text: ["该视频没有可播放地址"] });
          return;
        }
        window.open(video.url, "_blank");
      },
      formatBytes,
      formatDate,
    },
  };
}

import { createDownloadTask, fetchBrowseHistoryList } from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

import { formatDate } from "./downloads.model.js";

function pick(...values) {
  for (let i = 0; i < values.length; i += 1) {
    const v = values[i];
    if (v !== undefined && v !== null && String(v).trim() !== "") return v;
  }
  return "";
}

function safeFilename(value) {
  const name = String(value || "wx_channels_video")
    .replace(/[\\/:*?"<>|]/g, "_")
    .trim();
  return name.endsWith(".mp4") ? name : `${name || "wx_channels_video"}.mp4`;
}

export function normalizeBrowseHistory(item) {
  const title = pick(
    item.content_title,
    item.video_title,
    item.title,
    "未命名内容",
  );
  const contentUrl = pick(item.content_url, item.video_url, item.url);
  const sourceUrl = pick(item.content_source_url, item.source_url);
  return {
    ...item,
    title,
    url: contentUrl,
    source_url: sourceUrl,
    cover_url: pick(
      item.content_cover_url,
      item.video_cover_url,
      item.cover_url,
    ),
    visited_times: Number(item.visited_times || 0),
    created_at_text: formatDate(item.created_at),
    updated_at_text: formatDate(item.updated_at),
    account: {
      id: pick(
        item.account_external_id,
        item.account_username,
        item.contact_username,
      ),
      username: pick(item.account_username, item.contact_username),
      nickname: pick(item.account_nickname, item.contact_nickname, "未知帐号"),
      avatar_url: pick(item.account_avatar_url, item.contact_avatar_url),
    },
  };
}

function pickListFromResponse(data) {
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.list)) return data.list;
  if (Array.isArray(data?.data?.list)) return data.data.list;
  return [];
}

export function BrowseHistoryPageModel(props) {
  const pageSize = 50;
  const reqs = {
    history: new Timeless.RequestCore(fetchBrowseHistoryList, {
      client: api_client$,
      process(r) {
        if (r.error) return r;
        const list = pickListFromResponse(r.data).map(normalizeBrowseHistory);
        return Timeless.Result.Ok({
          list,
          total: list.length,
          page: 1,
          pageSize: list.length || pageSize,
          isEnd: true,
        });
      },
    }),
    createDownloadTask: new Timeless.RequestCore(createDownloadTask, {
      client: props.client,
    }),
  };
  const items_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");

  async function load() {
    loading_.as(true);
    error_.as("");
    const r = await reqs.history.run({ page: 1, pageSize });
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    items_.as(r.data.list || []);
  }

  const filtered_ = combine({ items: items_, keyword: keyword_ }, (d) => {
    const k = String(d.keyword || "")
      .trim()
      .toLowerCase();
    if (!k) {
      return d.items;
    }
    return d.items.filter((it) => {
      return [it.title, it.account.nickname, it.account.username].some((v) =>
        String(v || "")
          .toLowerCase()
          .includes(k),
      );
    });
  });

  return {
    state: {
      items: items_,
      filtered: filtered_,
      loading: loading_,
      error: error_,
      keyword: keyword_,
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      keyword: new Timeless.ui.InputCore({
        placeholder: "搜索标题、帐号",
        onChange(value) {
          keyword_.as(value);
        },
      }),
    },
    methods: {
      init: load,
      async refresh() {
        await load();
      },
      open(item) {
        const url = item.source_url || item.url;
        if (!url) {
          props.app.tip?.({ type: "warning", text: ["没有可打开的链接"] });
          return;
        }
        window.open(url, "_blank");
      },
      async download(item) {
        if (!item.url) {
          props.app.tip?.({ type: "warning", text: ["该记录没有视频地址"] });
          return;
        }
        const r = await reqs.createDownloadTask.run({
          url: item.url,
          filename: safeFilename(item.title),
          extra: {
            source_url: item.source_url || "",
            account_username: item.account.username || "",
          },
        });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["已创建下载任务"] });
      },
    },
  };
}

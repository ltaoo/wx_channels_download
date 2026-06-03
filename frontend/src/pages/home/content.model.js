import { fetchAccountList, fetchContentList } from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";
import { formatBytes, formatDate } from "./downloads.model.js";

function pick(...values) {
  return values.find((v) => v !== undefined && v !== null && v !== "") || "";
}

function normalizeAccount(account) {
  if (!account) return null;
  return {
    ...account,
    nickname: pick(account.nickname, account.account_nickname, account.name),
    username: pick(account.username, account.account_username, account.external_id, account.account_external_id),
    avatar_url: pick(account.avatar_url, account.account_avatar_url, account.avatarUrl, account.headUrl),
  };
}

function accountFromContent(content, fallback) {
  if (fallback) return normalizeAccount(fallback);
  if (content.account) return normalizeAccount(content.account);
  if (Array.isArray(content.accounts) && content.accounts.length > 0) return normalizeAccount(content.accounts[0]);
  const account = normalizeAccount({
    id: content.account_id,
    nickname: content.account_nickname,
    username: pick(content.account_username, content.account_external_id),
    avatar_url: content.account_avatar_url,
  });
  return account.nickname || account.username || account.avatar_url ? account : null;
}

function normalizeContent(content, account) {
  return {
    ...content,
    id: content.id || content.Id || `${content.external_id || content.title}-${Math.random()}`,
    title: content.title || content.Title || content.description || "未命名内容",
    description: content.description || content.Description || "",
    content_type: content.content_type || content.type || "",
    cover_url: content.cover_url || content.CoverURL || content.coverUrl || "",
    url: pick(content.url, content.URL, content.content_url, content.ContentURL, content.source_url, content.SourceURL),
    file_size: content.file_size || content.size || content.Size || 0,
    duration: content.duration || content.Duration || 0,
    publish_time: content.publish_time || content.PublishTime || 0,
    account: accountFromContent(content, account),
  };
}

async function fetchContentFallback(keyword, reqs) {
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
    for (const row of account.content_accounts || []) {
      const content = row.content || row.Content;
      if (content) {
        const item = normalizeContent(content, acc);
        if (!k || String(item.title || item.description || "").toLowerCase().includes(k)) {
          list.push(item);
        }
      }
    }
  }
  return Timeless.Result.Ok({ list, total: list.length, page: 1, page_size: list.length });
}

export function ContentPageModel(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: api_client$,
    }),
    content: new Timeless.RequestCore(fetchContentList, {
      client: api_client$,
    }),
  };
  const content_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const total_ = ref(0);
  const page_ = ref(1);
  const keyword_ = ref("");
  const page_size_ = 24;

  async function load(page = 1) {
    loading_.as(true);
    error_.as("");
    const r = await reqs.content.run({
      page,
      pageSize: page_size_,
      keyword: keyword_.value.trim(),
    });
    let result = r;
    if (!r.error && page === 1 && (!r.data.list || r.data.list.length === 0)) {
      result = await fetchContentFallback(keyword_.value, reqs);
    }
    loading_.as(false);
    if (result.error) {
      error_.as(result.error.message || String(result.error));
      return;
    }
    const list = (result.data.list || []).map((it) => normalizeContent(it));
    if (page === 1) {
      content_.as(list);
    } else {
      content_.push(...list);
    }
    total_.as(result.data.total || list.length);
    page_.as(page);
  }

  return {
    state: {
      content: content_,
      loading: loading_,
      error: error_,
      total: total_,
      keyword: keyword_,
      page: page_,
      noMore: computed({ content: content_, total: total_ }, (d) => d.content.length >= d.total),
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      keyword: new Timeless.ui.InputCore({
        placeholder: "搜索内容标题",
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
      open(content) {
        if (!content.url) {
          props.app.tip?.({ type: "warning", text: ["该内容没有可打开地址"] });
          return;
        }
        window.open(content.url, "_blank");
      },
      formatBytes,
      formatDate,
    },
  };
}

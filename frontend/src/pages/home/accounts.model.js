import {
  fetchAccountList,
  fetchOfficialAccountMsgList,
  highlightDownloadFile,
  openURL,
  synchronizeAccount,
} from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

function isOfficialAccount(account) {
  const platformId = String(account.platform_id || account.platform?.id || "");
  return platformId === "wx_official_account" || platformId === "officialaccount";
}

function normalizeContent(content, context = {}) {
  const type = String(content.content_type || content.type || "").trim();
  const sourceType = String(content.source_content_type || "").trim();
  const outputFormat = String(content.output_format || "").trim();
  const displayType = String(
    content.display_type ||
      content.type_label ||
      [sourceType, outputFormat]
        .filter(Boolean)
        .filter((value, index, values) => values.indexOf(value) === index)
        .join(" ") ||
      outputFormat ||
      type ||
      "file",
  ).trim();
  const title = content.title || content.Title || content.description || "";
  const coverURL =
    content.cover_url || content.CoverURL || content.coverUrl || "";
  const platformId =
    content.platform_id || content.platform?.id || context.platformId || "";
  return {
    ...content,
    id: content.id || content.Id || content.content_id || content.ContentId,
    platform_id: platformId,
    type,
    content_type: type,
    source_content_type: sourceType,
    output_format: outputFormat,
    display_type: displayType,
    type_label: displayType,
    cover_url: coverURL,
    display_cover_url: coverURL,
    title: title || "未命名内容",
    download_path:
      content.download_path ||
      content.DownloadPath ||
      content.output_path ||
      content.filepath ||
      "",
    source_url: content.source_url || content.SourceURL || "",
    url:
      content.url ||
      content.URL ||
      content.content_url ||
      content.ContentURL ||
      content.source_url ||
      content.SourceURL ||
      "",
  };
}

function decodeHTML(value) {
  const text = String(value || "");
  if (!text) return "";
  const el = document.createElement("textarea");
  el.innerHTML = text;
  return el.value;
}

function normalizeOfficialAccountURL(rawURL) {
  const url = decodeHTML(rawURL).trim();
  if (!url) return "";
  if (url.startsWith("http://") || url.startsWith("https://")) return url;
  if (url.startsWith("//")) return "https:" + url;
  if (url.startsWith("/")) return "https://mp.weixin.qq.com" + url;
  return url;
}

function normalizeOfficialAccountMessage(article, context = {}) {
  const publishTime = Number(context.datetime || article.publish_time || 0);
  const title = article.title || "未命名推送";
  return {
    title,
    digest: article.digest || "",
    author: article.author || context.author || "",
    cover_url: normalizeOfficialAccountURL(article.cover || article.cover_url),
    url: normalizeOfficialAccountURL(
      article.content_url || article.url || article.source_url,
    ),
    source_url: normalizeOfficialAccountURL(article.source_url),
    fileid: article.fileid || 0,
    publish_time: publishTime,
    publish_time_text: publishTime
      ? new Date(publishTime * 1000).toLocaleString()
      : "",
  };
}

function normalizeOfficialAccountMsgList(data) {
  const raw = data?.general_msg_list || data?.MsgList || "";
  if (!raw) return [];
  let parsed = null;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  const list = Array.isArray(parsed?.list) ? parsed.list : [];
  const out = [];
  for (const item of list) {
    const msg = item.app_msg_ext_info || item.MsgExtInfo || {};
    const common = item.comm_msg_info || item.CommonMsgInfo || {};
    const context = {
      datetime: common.datetime || common.Datetime,
      author: msg.author,
    };
    if (msg.title || msg.content_url || msg.cover) {
      out.push(normalizeOfficialAccountMessage(msg, context));
    }
    const multi = msg.multi_app_msg_item_list || msg.MultiAppMsgItemList || [];
    for (const article of multi) {
      out.push(normalizeOfficialAccountMessage(article, context));
    }
  }
  return out;
}

function normalizeAccount(account) {
  const platformId =
    account.platform_id || account.platform?.id || account.platform?.code || "";
  const platformName =
    account.platform_name ||
    account.platform?.name ||
    platformNameOf(platformId);
  const avatarURL = account.avatar_url || "";
  const contentMedias = (account.content_accounts || [])
    .map((row) => row.content || row.Content || null)
    .filter(Boolean)
    .map((content) => normalizeContent(content, { platformId }));
  return {
    ...account,
    nickname:
      account.nickname ||
      account.username ||
      account.external_id ||
      "未命名帐号",
    avatar_url: avatarURL,
    display_avatar_url: avatarURL,
    platform_id: platformId,
    platform_name: platformName,
    content_count: Number(account.content_count || 0),
    has_content: !!account.has_content || Number(account.content_count || 0) > 0,
    is_official_account: isOfficialAccount({ ...account, platform_id: platformId }),
    medias: contentMedias,
  };
}

function createdAtOf(account) {
  const value = account.created_at || account.CreatedAt || 0;
  const timestamp = Number(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function sortAccountsByCreatedAtDesc(accounts) {
  return accounts.toSorted
    ? accounts.toSorted(
        (a, b) => createdAtOf(b) - createdAtOf(a) || (b.id || 0) - (a.id || 0),
      )
    : [...accounts].sort(
        (a, b) => createdAtOf(b) - createdAtOf(a) || (b.id || 0) - (a.id || 0),
      );
}

function platformNameOf(platformId) {
  switch (platformId) {
    case "wx_official_account":
      return "公众号";
    case "wx_channels":
      return "视频号";
    case "douyin":
      return "抖音";
    case "bilibili":
      return "Bilibili";
    case "xiaohongshu":
    case "xhs":
    case "rednote":
      return "小红书";
    case "tiktok":
      return "TikTok";
    case "youtube":
      return "YouTube";
    case "zhihu":
      return "知乎";
    default:
      return platformId || "未知平台";
  }
}

export function AccountsPageModel(props) {
  const reqs = {
    accounts: new Timeless.RequestCore(fetchAccountList, {
      client: api_client$,
    }),
    synchronize: new Timeless.RequestCore(synchronizeAccount, {
      client: api_client$,
    }),
    officialMessages: new Timeless.RequestCore(fetchOfficialAccountMsgList, {
      client: api_client$,
    }),
    open: new Timeless.RequestCore(openURL, {
      client: api_client$,
    }),
    highlight: new Timeless.RequestCore(highlightDownloadFile, {
      client: api_client$,
    }),
  };
  const accounts_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");
  const content_filter_ = ref("with");
  const selected_ = ref(null);
  const playing_url_ = ref("");
  const official_account_ = ref(null);
  const official_messages_ = refarr([]);
  const official_messages_loading_ = ref(false);
  const official_messages_error_ = ref("");
  const official_messages_has_more_ = ref(false);
  const official_messages_next_offset_ = ref(0);

  async function load() {
    loading_.as(true);
    error_.as("");
    const r = await reqs.accounts.run({ content_filter: content_filter_.value });
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    accounts_.as(
      sortAccountsByCreatedAtDesc((r.data.list || []).map(normalizeAccount)),
    );
  }

  async function loadOfficialMessages(account, offset = 0) {
    const biz = String(account?.external_id || account?.username || "").trim();
    if (!biz) {
      official_messages_error_.as("缺少公众号 biz，无法获取推送列表");
      return;
    }
    official_messages_loading_.as(true);
    official_messages_error_.as("");
    const r = await reqs.officialMessages.run({ biz, offset });
    official_messages_loading_.as(false);
    if (r.error) {
      official_messages_error_.as(r.error.message || String(r.error));
      return;
    }
    const messages = normalizeOfficialAccountMsgList(r.data);
    official_messages_has_more_.as(Number(r.data?.can_msg_continue || 0) !== 0);
    official_messages_next_offset_.as(Number(r.data?.next_offset || 0));
    if (offset > 0) {
      official_messages_.as([...official_messages_.value, ...messages]);
      return;
    }
    official_messages_.as(messages);
  }

  const filtered_ = combine(
    { accounts: accounts_, keyword: keyword_ },
    (d) => {
      const k = String(d.keyword || "")
        .trim()
        .toLowerCase();
      if (!k) return d.accounts;
      return d.accounts.filter((it) => {
        return [it.nickname, it.username, it.external_id, it.platform_name, it.platform_id].some((v) =>
          String(v || "")
            .toLowerCase()
            .includes(k),
        );
      });
    },
  );

  return {
    state: {
      accounts: accounts_,
      filtered: filtered_,
      loading: loading_,
      error: error_,
      keyword: keyword_,
      contentFilter: content_filter_,
      selected: selected_,
      playingUrl: playing_url_,
      officialAccount: official_account_,
      officialMessages: official_messages_,
      officialMessagesLoading: official_messages_loading_,
      officialMessagesError: official_messages_error_,
      officialMessagesHasMore: official_messages_has_more_,
      officialMessagesNextOffset: official_messages_next_offset_,
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      keyword: new Timeless.ui.InputCore({
        placeholder: "搜索昵称、用户名",
        onChange(value) {
          keyword_.as(value);
        },
      }),
    },
    methods: {
      init: load,
      refresh: load,
      async setContentFilter(value) {
        if (content_filter_.value === value) return;
        content_filter_.as(value);
        await load();
      },
      select(account) {
        selected_.as(account);
      },
      async synchronize(account) {
        const r = await reqs.synchronize.run({ account_id: account.id });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["同步完成"] });
        await load();
      },
      async openOfficialMessages(account) {
        official_account_.as(account);
        official_messages_.as([]);
        official_messages_has_more_.as(false);
        official_messages_next_offset_.as(0);
        await loadOfficialMessages(account, 0);
      },
      closeOfficialMessages() {
        official_account_.as(null);
        official_messages_.as([]);
        official_messages_error_.as("");
        official_messages_has_more_.as(false);
        official_messages_next_offset_.as(0);
      },
      async loadMoreOfficialMessages() {
        const account = official_account_.value;
        if (!account || official_messages_loading_.value) return;
        const offset = official_messages_next_offset_.value;
        if (!offset) return;
        await loadOfficialMessages(account, offset);
      },
      async openOfficialMessage(message) {
        const url = message.url || message.source_url || "";
        if (!url) {
          props.app.tip?.({ type: "warning", text: ["该推送没有可打开地址"] });
          return;
        }
        const r = await reqs.open.run({ url });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["已在浏览器打开"] });
      },
      async openContentFile(content) {
        const filePath =
          content.download_path || content.output_path || content.filepath || "";
        if (!filePath) {
          props.app.tip?.({ type: "warning", text: ["该内容还没有文件路径"] });
          return;
        }
        const r = await reqs.highlight.run({ file_path: filePath });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["已定位文件"] });
      },
      async openContentSource(content) {
        const url =
          content.source_url ||
          content.SourceURL ||
          content.url ||
          content.URL ||
          content.content_url ||
          content.ContentURL ||
          "";
        if (!url) {
          props.app.tip?.({ type: "warning", text: ["该内容没有源地址"] });
          return;
        }
        const r = await reqs.open.run({ url });
        if (r.error) {
          props.app.tip?.({
            type: "error",
            text: [r.error.message || String(r.error)],
          });
          return;
        }
        props.app.tip?.({ type: "success", text: ["已在浏览器打开源地址"] });
      },
    },
  };
}

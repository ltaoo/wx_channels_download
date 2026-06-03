import { fetchAccountList, openURL, synchronizeAccount } from "@/biz/request.js";
import { api_client$ } from "@/store/index.js";

function getConfig() {
  if (typeof WXU !== "undefined" && WXU.config) return WXU.config;
  if (typeof window !== "undefined" && window.__wx_channels_config__) {
    return window.__wx_channels_config__;
  }
  return {};
}

function getAPIClientOrigin() {
  const hostname = String(api_client$?.hostname || "").trim();
  if (!hostname) {
    return "";
  }
  try {
    return new URL(hostname, window.location.origin).origin;
  } catch (e) {
    return hostname.replace(/\/+$/, "");
  }
}

function mpProxyURL(rawURL) {
  const url = String(rawURL || "").trim();
  if (!url || url.includes("/mp/proxy?")) {
    return url;
  }
  const cfg = getConfig();
  const token = cfg.officialServerRefreshToken || "";
  const params = new URLSearchParams();
  if (token) {
    params.set("token", token);
  }
  params.set("url", url);
  return `${getAPIClientOrigin()}/mp/proxy?${params.toString()}`;
}

function normalizeContent(content, options = {}) {
  const type = String(content.content_type || content.type || "").trim();
  const title = content.title || content.Title || content.description || "";
  const coverURL =
    content.cover_url || content.CoverURL || content.coverUrl || "";
  return {
    ...content,
    id: content.id || content.Id || content.content_id || content.ContentId,
    type,
    content_type: type,
    cover_url: coverURL,
    display_cover_url: options.proxyImages ? mpProxyURL(coverURL) : coverURL,
    title,
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

function isArticleContent(content) {
  const type = String(content.content_type || content.type || "").trim();
  return type === "article";
}

function normalizeAccount(account) {
  const platformId =
    account.platform_id || account.platform?.id || account.platform?.code || "";
  const platformName =
    account.platform_name ||
    account.platform?.name ||
    platformNameOf(platformId);
  const avatarURL = account.avatar_url || "";
  const proxyImages = platformId === "wx_official_account";
  const contentMedias = (account.content_accounts || [])
    .map((row) => row.content || row.Content || null)
    .filter(Boolean)
    .map((content) => normalizeContent(content, { proxyImages }));
  return {
    ...account,
    nickname:
      account.nickname ||
      account.username ||
      account.external_id ||
      "未命名帐号",
    avatar_url: avatarURL,
    display_avatar_url: proxyImages ? mpProxyURL(avatarURL) : avatarURL,
    platform_id: platformId,
    platform_name: platformName,
    content_count: Number(account.content_count || 0),
    has_content: !!account.has_content || Number(account.content_count || 0) > 0,
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
    open: new Timeless.RequestCore(openURL, {
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
      async play(content) {
        const url = content.url || content.URL || "";
        if (!url) {
          props.app.tip?.({ type: "warning", text: ["该内容没有可打开地址"] });
          return;
        }
        if (isArticleContent(content)) {
          const r = await reqs.open.run({ url });
          if (r.error) {
            props.app.tip?.({
              type: "error",
              text: [r.error.message || String(r.error)],
            });
            return;
          }
          props.app.tip?.({ type: "success", text: ["已在浏览器打开"] });
          return;
        }
        playing_url_.as(url);
        window.open(url, "_blank");
      },
    },
  };
}

/// <reference path="../utils.js" />
/**
 * @file Account list page data requests, state, and formatting logic.
 */
var AccountModel = (() => {
  const account_api_origin = WXEnv.get("apiOrigin");
  const account_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: account_api_origin,
  });
  Timeless.web.provide_http_client(account_http_client);

  const account_request = Timeless.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取账号列表失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

  function first_non_empty(...values) {
    for (const value of values) {
      if (value !== undefined && value !== null && value !== "") {
        return value;
      }
    }
    return "";
  }

  function number_or_default(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function normalize_account_list_response(data) {
    const source = data && typeof data === "object" ? data : {};
    return Array.isArray(source.list)
      ? source.list
      : Array.isArray(source.List)
        ? source.List
        : [];
  }

  function normalize_account_item(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    return {
      ...source,
      id: first_non_empty(source.id, source.ID),
      platform_id: first_non_empty(
        source.platform_id,
        source.platformId,
        source.PlatformID,
      ),
      external_id: first_non_empty(
        source.external_id,
        source.externalId,
        source.ExternalID,
      ),
      nickname: first_non_empty(
        source.nickname,
        source.Nickname,
        source.alias,
        source.Alias,
        source.external_id,
        source.ExternalID,
        "未命名账号",
      ),
      avatar_url: first_non_empty(
        source.avatar_url,
        source.avatarUrl,
        source.AvatarURL,
      ),
      content_count: Math.max(
        0,
        number_or_default(
          first_non_empty(
            source.content_count,
            source.contentCount,
            source.ContentCount,
          ),
          0,
        ),
      ),
      created_at: number_or_default(
        first_non_empty(source.created_at, source.createdAt, source.CreatedAt),
        0,
      ),
    };
  }

  function account_platform_name(account) {
    const names = {
      wxchannels: "视频号",
      wxmp: "公众号",
      officialaccount: "公众号",
      douyin: "抖音",
      bilibili: "Bilibili",
      xiaohongshu: "小红书",
      xhs: "小红书",
      youtube: "YouTube",
      zhihu: "知乎",
      douban: "豆瓣",
      weibo: "微博",
      qidian: "起点中文网",
      fanqienovel: "番茄小说",
      "69shuba": "69书吧",
    };
    const platform_id = String((account && account.platform_id) || "").trim();
    return names[platform_id] || platform_id || "未知平台";
  }

  function account_platform_favicon(account) {
    const icons = {
      wxchannels:
        "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
      wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      officialaccount:
        "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
    };
    const platform_id = String((account && account.platform_id) || "").trim();
    return icons[platform_id] || "";
  }

  function normalize_epoch_ms(value) {
    const timestamp = Number(value);
    if (!Number.isFinite(timestamp) || timestamp <= 0) {
      return 0;
    }
    return timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
  }

  function format_account_time(value) {
    const timestamp = normalize_epoch_ms(value);
    if (!timestamp) {
      return "时间未知";
    }
    return new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(timestamp));
  }

  function format_content_count(value) {
    return `${Math.max(0, number_or_default(value, 0))} 条`;
  }

  function create_model() {
    const accounts_ = refarr([]);
    const loading_ = ref(false);
    const error_ = ref("");
    let request_sequence = 0;

    const list_request = new Timeless.RequestCore(
      () => account_request.get("/api/account/list"),
      {
        client: account_http_client,
        process(response) {
          if (response.error) {
            return Timeless.Result.Err(response.error);
          }
          return Timeless.Result.Ok(
            normalize_account_list_response(response.data),
          );
        },
      },
    );

    const total_ = computed(accounts_, (accounts) => accounts.length);
    const initial_loading_ = combine(
      { loading: loading_, accounts: accounts_ },
      (state) => state.loading && state.accounts.length === 0,
    );
    const summary_text_ = computed(
      total_,
      (total) => `共 ${total} 个账号`,
    );

    async function load() {
      const sequence = ++request_sequence;
      loading_.as(true);
      error_.as("");

      const result = await list_request.run();
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      if (result.error) {
        error_.as(result.error.message || String(result.error));
        return result;
      }

      accounts_.as(result.data.map(normalize_account_item), { reset: true });
      return result;
    }

    const methods = {
      ready() {
        return load();
      },
      refresh() {
        return load();
      },
      platformName: account_platform_name,
      platformFavicon: account_platform_favicon,
      formatTime: format_account_time,
      formatContentCount: format_content_count,
    };

    return {
      state: {
        accounts: accounts_,
        total: total_,
        initial_loading: initial_loading_,
        summary_text: summary_text_,
        loading: loading_,
        error: error_,
      },
      methods,
      ready: methods.ready,
    };
  }

  return create_model;
})();

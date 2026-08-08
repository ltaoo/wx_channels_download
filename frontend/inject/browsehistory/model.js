/// <reference path="../utils.js" />
/**
 * @file Browse history list page data requests, pagination state, and formatting logic.
 */
var BrowseHistoryModel = (() => {
  const browse_history_api_origin = window.location.origin || WXEnv.get("apiOrigin");
  const browse_history_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: browse_history_api_origin,
  });
  Timeless.web.provide_http_client(browse_history_http_client);

  const browse_history_request = Timeless.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取浏览记录列表失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

  function number_or_default(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function normalize_browse_history_list_response(
    data,
    fallbackPage,
    fallbackSize,
  ) {
    const source = data && typeof data === "object" ? data : {};
    const list = Array.isArray(source.list)
      ? source.list
      : Array.isArray(source.List)
        ? source.List
        : [];
    return {
      list,
      total: Math.max(
        0,
        number_or_default(
          typeof source.total !== "undefined" ? source.total : source.Total,
          list.length,
        ),
      ),
      page: Math.max(
        1,
        number_or_default(source.page || source.Page, fallbackPage),
      ),
      page_size: Math.max(
        1,
        number_or_default(
          source.page_size || source.pageSize || source.PageSize,
          fallbackSize,
        ),
      ),
    };
  }

  function first_non_empty(...values) {
    for (const value of values) {
      if (value !== undefined && value !== null && value !== "") {
        return value;
      }
    }
    return "";
  }

  function normalize_browse_history_item(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    const accounts = Array.isArray(source.accounts)
      ? source.accounts.map(normalize_account_brief)
      : [];
    const primary =
      accounts.length > 0
        ? accounts[0]
        : { nickname: "", avatar_url: "", external_id: "" };
    return {
      ...source,
      id: first_non_empty(source.id, source.ID),
      platform_id: first_non_empty(source.platform_id, source.PlatformID),
      platform_name: first_non_empty(
        source.platform_name,
        source.platformName,
        source.PlatformName,
      ),
      content_type: first_non_empty(
        source.type,
        source.Type,
        source.content_type,
        source.ContentType,
      ),
      title: first_non_empty(
        source.title,
        source.Title,
        source.content_title,
        source.ContentTitle,
        "未命名内容",
      ),
      cover_url: first_non_empty(
        source.cover_url,
        source.CoverURL,
        source.coverUrl,
      ),
      accounts: accounts,
      author_nickname: primary.nickname,
      author_external_id: primary.external_id,
      author_avatar_url: primary.avatar_url,
      url: first_non_empty(
        source.url,
        source.URL,
        source.content_url,
        source.ContentURL,
        source.source_url,
        source.SourceURL,
      ),
      visited_times: number_or_default(
        first_non_empty(
          source.visited_times,
          source.VisitedTimes,
          source.visits,
          source.visit_count,
        ),
        0,
      ),
      updated_at: number_or_default(
        first_non_empty(source.updated_at, source.UpdatedAt),
        0,
      ),
      publish_time: number_or_default(
        first_non_empty(source.publish_time, source.PublishTime),
        0,
      ),
    };
  }

  function normalize_account_brief(acc) {
    if (!acc || typeof acc !== "object") {
      return { nickname: "", avatar_url: "", external_id: "" };
    }
    return {
      nickname: first_non_empty(acc.nickname, acc.Nickname, ""),
      avatar_url: first_non_empty(
        acc.avatar_url,
        acc.avatarUrl,
        acc.AvatarURL,
        "",
      ),
      external_id: first_non_empty(
        acc.external_id,
        acc.externalId,
        acc.ExternalId,
        "",
      ),
    };
  }

  function browse_history_platform_favicon(history) {
    const icons = {
      wxchannels:
        "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
      wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
    };
    const key = history && (history.platform_id || history.platformId);
    return icons[key] || "";
  }

  function browse_history_platform_name(history) {
    if (history.platform_name) {
      return history.platform_name;
    }
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
      qidian: "起点中文网",
      fanqienovel: "番茄小说",
      "69shuba": "69书吧",
    };
    return names[history.platform_id] || history.platform_id || "未知平台";
  }

  function browse_history_type_label(value) {
    const type = String(value || "")
      .trim()
      .toLowerCase();
    const labels = {
      video: "视频",
      short_video: "短视频",
      image: "图片",
      image_set: "图集",
      album: "图集",
      article: "文章",
      blog: "文章",
      novel: "小说",
      audio: "音频",
      podcast: "播客",
      music: "音乐",
      document: "文档",
      live: "直播",
    };
    return labels[type] || type || "内容";
  }

  function normalize_epoch_ms(value) {
    const timestamp = Number(value);
    if (!Number.isFinite(timestamp) || timestamp <= 0) {
      return 0;
    }
    return timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
  }

  function format_history_time(value) {
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

  function normalize_author_name(raw) {
    const name = first_non_empty(
      raw && raw.author_nickname,
      raw && raw.author_external_id,
      "未知作者",
    );
    return name;
  }

  function browse_history_item_key(history) {
    return String(
      first_non_empty(
        history && history.id,
        history && history.url,
        history && history.source_url,
      ),
    );
  }

  function append_unique_browse_histories(current, incoming) {
    const next = Array.isArray(current) ? current.slice() : [];
    const keys = new Set(next.map(browse_history_item_key).filter(Boolean));
    for (const history of incoming || []) {
      const key = browse_history_item_key(history);
      if (key && keys.has(key)) {
        continue;
      }
      next.push(history);
      if (key) {
        keys.add(key);
      }
    }
    return next;
  }

  function create_model() {
    const PAGE_SIZE_DEFAULT = 24;
    const histories_ = refarr([]);
    const total_ = ref(0);
    const page_ = ref(1);
    const page_size_ = ref(PAGE_SIZE_DEFAULT);
    const platform_id_ = ref("");
    const loading_ = ref(false);
    const loading_more_ = ref(false);
    const error_ = ref("");
    const load_more_error_ = ref("");
    let request_sequence = 0;

    const platform_options = [
      new Timeless.ui.SelectItemCore({ label: "全部平台", value: "" }),
      new Timeless.ui.SelectItemCore({ label: "视频号", value: "wxchannels" }),
      new Timeless.ui.SelectItemCore({ label: "公众号", value: "wxmp" }),
      new Timeless.ui.SelectItemCore({ label: "抖音", value: "douyin" }),
      new Timeless.ui.SelectItemCore({ label: "Bilibili", value: "bilibili" }),
      new Timeless.ui.SelectItemCore({ label: "小红书", value: "xiaohongshu" }),
      new Timeless.ui.SelectItemCore({ label: "YouTube", value: "youtube" }),
      new Timeless.ui.SelectItemCore({ label: "知乎", value: "zhihu" }),
      new Timeless.ui.SelectItemCore({ label: "豆瓣", value: "douban" }),
      new Timeless.ui.SelectItemCore({ label: "微博", value: "weibo" }),
    ];
    const platform_select_ = new Timeless.ui.SelectCore({
      defaultValue: "",
      placeholder: "全部平台",
      options: platform_options,
      onChange(value) {
        platform_id_.as(String(value || ""));
        load(1, { reset: true });
      },
    });

    const list_request = new Timeless.RequestCore(
      (params) =>
        browse_history_request.post("/api/browse_history/list", params),
      {
        client: browse_history_http_client,
        process(response) {
          if (response.error) {
            return Timeless.Result.Err(response.error);
          }
          return Timeless.Result.Ok(
            normalize_browse_history_list_response(
              response.data,
              page_.value,
              page_size_.value,
            ),
          );
        },
      },
    );

    const initial_loading_ = combine(
      { loading: loading_, histories: histories_ },
      (state) => state.loading && state.histories.length === 0,
    );
    const has_more_ = combine(
      {
        histories: histories_,
        total: total_,
        page: page_,
        pageSize: page_size_,
      },
      (state) =>
        state.histories.length < state.total &&
        state.page * state.pageSize < state.total,
    );
    const loaded_text_ = combine(
      {
        total: total_,
        histories: histories_,
      },
      (state) => {
        const count = state.histories.length;
        if (!state.total) {
          return `共 ${state.total || 0} 条`;
        }
        return count >= state.total
          ? `已加载全部 ${state.total} 条`
          : `已加载 ${count} / ${state.total} 条`;
      },
    );

    async function load(targetPage = page_.value, options = {}) {
      const append = options.append === true;
      if (append && (loading_.value || !has_more_.value)) {
        return null;
      }
      const sequence = ++request_sequence;
      const requestedPage = Math.max(1, Number(targetPage) || 1);
      loading_.as(true);
      loading_more_.as(append);
      if (append) {
        load_more_error_.as("");
      } else {
        error_.as("");
        load_more_error_.as("");
      }
      if (options.reset === true) {
        histories_.as([], { reset: true });
        total_.as(0);
        page_.as(1);
      }
      const params = {
        page: requestedPage,
        page_size: page_size_.value,
      };
      const platformId = String(platform_id_.value || "").trim();
      if (platformId) {
        params.platform_id = platformId;
      }

      const result = await list_request.run(params);
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      loading_more_.as(false);
      if (result.error) {
        const message = result.error.message || String(result.error);
        if (append) {
          load_more_error_.as(message);
        } else {
          error_.as(message);
        }
        return result;
      }

      const data = result.data;
      const incoming = data.list.map(normalize_browse_history_item);
      const next = append
        ? append_unique_browse_histories(histories_.value, incoming)
        : incoming;
      histories_.as(next, { reset: true });
      total_.as(Math.max(data.total, next.length));
      page_.as(data.page || requestedPage);
      page_size_.as(data.page_size);
      return result;
    }

    const methods = {
      ready() {
        return load(1, { reset: true });
      },
      refresh() {
        return load(1, { reset: true });
      },
      loadMore() {
        if (loading_.value || !has_more_.value) {
          return null;
        }
        return load(page_.value + 1, { append: true });
      },
      openSource(history) {
        if (!history || !history.url) {
          return;
        }
        window.open(history.url, "_blank", "noopener,noreferrer");
      },
      platformFavicon: browse_history_platform_favicon,
      platformName: browse_history_platform_name,
      typeLabel: browse_history_type_label,
      formatTime: format_history_time,
      authorName: normalize_author_name,
    };

    return {
      state: {
        histories: histories_,
        total: total_,
        page: page_,
        page_size: page_size_,
        initial_loading: initial_loading_,
        has_more: has_more_,
        loaded_text: loaded_text_,
        platform_id: platform_id_,
        loading: loading_,
        loading_more: loading_more_,
        error: error_,
        load_more_error: load_more_error_,
      },
      ui: {
        platform: platform_select_,
      },
      methods,
      ready: methods.ready,
    };
  }

  return create_model;
})();

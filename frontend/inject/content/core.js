/// <reference path="../utils.js" />
/**
 * @file Content list page data requests, pagination state, and formatting logic.
 */
var ContentLibraryModel = (() => {
  const content_api_origin = window.location.origin || WXEnv.apiOrigin;
  const content_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: content_api_origin,
  });
  Timeless.web.provide_http_client(content_http_client);

  const content_request = Timeless.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取内容列表失败",
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

  function normalize_content_list_response(data, fallbackPage, fallbackSize) {
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

  function normalize_content_account(account) {
    if (!account || typeof account !== "object") {
      return null;
    }
    return {
      id: first_non_empty(account.id, account.ID),
      platform_id: first_non_empty(
        account.platform_id,
        account.PlatformID,
      ),
      influencer_id: first_non_empty(
        account.influencer_id,
        account.InfluencerID,
      ),
      external_id: first_non_empty(
        account.external_id,
        account.ExternalID,
      ),
      alias: first_non_empty(account.alias, account.Alias),
      nickname: first_non_empty(
        account.nickname,
        account.Nickname,
        account.name,
      ),
      signature: first_non_empty(
        account.signature,
        account.Signature,
      ),
      avatar_url: first_non_empty(
        account.avatar_url,
        account.AvatarURL,
      ),
      profile_url: first_non_empty(
        account.profile_url,
        account.ProfileURL,
      ),
      is_listen: number_or_default(
        first_non_empty(account.is_listen, account.IsListen),
        0,
      ),
      follower_count: number_or_default(
        first_non_empty(account.follower_count, account.FollowerCount),
        0,
      ),
      role: first_non_empty(account.role, account.Role),
      created_at: number_or_default(
        first_non_empty(account.created_at, account.CreatedAt),
        0,
      ),
      updated_at: number_or_default(
        first_non_empty(account.updated_at, account.UpdatedAt),
        0,
      ),
    };
  }

  function normalize_content_item(content) {
    const source = content && typeof content === "object" ? content : {};
    const accounts = Array.isArray(source.accounts)
      ? source.accounts.map(normalize_content_account).filter(Boolean)
      : [];
    return {
      ...source,
      id: first_non_empty(source.id, source.ID),
      platform_id: first_non_empty(
        source.platform_id,
        source.PlatformID,
      ),
      platform_name: first_non_empty(
        source.platform_name,
        source.PlatformName,
      ),
      content_type: first_non_empty(
        source.content_type,
        source.ContentType,
        source.type,
      ),
      external_id: first_non_empty(
        source.external_id,
        source.ExternalID,
      ),
      title: first_non_empty(
        source.title,
        source.Title,
        source.description,
        "未命名内容",
      ),
      description: first_non_empty(
        source.description,
        source.Description,
      ),
      url: first_non_empty(
        source.source_url,
        source.SourceURL,
        source.url,
        source.URL,
        source.content_url,
        source.ContentURL,
      ),
      cover_url: first_non_empty(
        source.cover_url,
        source.CoverURL,
        source.coverUrl,
      ),
      file_size: number_or_default(
        first_non_empty(source.file_size, source.FileSize, source.size),
        0,
      ),
      publish_time: number_or_default(
        first_non_empty(source.publish_time, source.PublishTime),
        0,
      ),
      download_status: number_or_default(
        first_non_empty(source.download_status, source.DownloadStatus),
        0,
      ),
      download_path: first_non_empty(
        source.download_path,
        source.DownloadPath,
      ),
      error_msg: first_non_empty(source.error_msg, source.ErrorMsg),
      accounts,
    };
  }

  function content_platform_name(content) {
    if (content.platform_name) {
      return content.platform_name;
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
    return names[content.platform_id] || content.platform_id || "未知平台";
  }

  function content_type_label(value) {
    const type = String(value || "").trim().toLowerCase();
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

  function content_download_status(downloadTasks) {
    if (!downloadTasks || !downloadTasks.length) {
      return { label: "未下载", tone: "waiting" };
    }
    var count = downloadTasks.length;
    // 1=Preparing, 2=Downloading, 4=Merging → still running
    if (downloadTasks.some(function (t) {
      return t.status === 1 || t.status === 2 || t.status === 4;
    })) {
      return { label: count + "个下载任务", tone: "running" };
    }
    // 6=Failed
    if (downloadTasks.some(function (t) { return t.status === 6; })) {
      return { label: count + "个下载任务", tone: "failed" };
    }
    // 5=Finished
    if (downloadTasks.every(function (t) { return t.status === 5; })) {
      return { label: count + "个下载任务", tone: "finished" };
    }
    return { label: count + "个下载任务", tone: "waiting" };
  }

  function normalize_epoch_ms(value) {
    const timestamp = Number(value);
    if (!Number.isFinite(timestamp) || timestamp <= 0) {
      return 0;
    }
    return timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
  }

  function format_content_time(value) {
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

  function format_content_bytes(value) {
    const bytes = Number(value);
    if (!Number.isFinite(bytes) || bytes <= 0) {
      return "";
    }
    const units = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(
      units.length - 1,
      Math.floor(Math.log(bytes) / Math.log(1024)),
    );
    const amount = bytes / Math.pow(1024, index);
    return `${amount >= 100 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
  }

  function content_account_role_label(value) {
    const role = String(value || "").trim().toLowerCase();
    const labels = {
      author: "作者",
      creator: "创作者",
      publisher: "发布者",
      editor: "编辑",
      owner: "账号主体",
      guest: "嘉宾",
      host: "主播",
    };
    return labels[role] || role;
  }

  function format_content_count(value) {
    const count = Number(value);
    if (!Number.isFinite(count) || count <= 0) {
      return "";
    }
    if (count >= 100000000) {
      return `${(count / 100000000).toFixed(count >= 1000000000 ? 0 : 1)}亿`;
    }
    if (count >= 10000) {
      return `${(count / 10000).toFixed(count >= 100000 ? 0 : 1)}万`;
    }
    return new Intl.NumberFormat("zh-CN").format(count);
  }

  function create_model() {
    const PAGE_SIZE_DEFAULT = 24;
    const contents_ = refarr([]);
    const total_ = ref(0);
    const page_ = ref(1);
    const page_size_ = ref(PAGE_SIZE_DEFAULT);
    const keyword_ = ref("");
    const content_type_ = ref("");
    const loading_ = ref(false);
    const error_ = ref("");
    let request_sequence = 0;

    const content_type_select_ = new Timeless.ui.SelectCore({
      defaultValue: "",
      placeholder: "全部类型",
      options: [
        new Timeless.ui.SelectItemCore({ label: "全部类型", value: "" }),
        new Timeless.ui.SelectItemCore({ label: "视频", value: "video" }),
        new Timeless.ui.SelectItemCore({
          label: "短视频",
          value: "short_video",
        }),
        new Timeless.ui.SelectItemCore({ label: "图片", value: "image" }),
        new Timeless.ui.SelectItemCore({
          label: "图集",
          value: "image_set",
        }),
        new Timeless.ui.SelectItemCore({ label: "文章", value: "article" }),
        new Timeless.ui.SelectItemCore({ label: "小说", value: "novel" }),
        new Timeless.ui.SelectItemCore({ label: "音频", value: "audio" }),
        new Timeless.ui.SelectItemCore({ label: "播客", value: "podcast" }),
        new Timeless.ui.SelectItemCore({ label: "直播", value: "live" }),
      ],
      onChange(value) {
        content_type_.as(String(value || ""));
        load(1);
      },
    });

    const list_request = new Timeless.RequestCore(
      (params) => content_request.get("/api/content/list", params),
      {
        client: content_http_client,
        process(response) {
          if (response.error) {
            return Timeless.Result.Err(response.error);
          }
          return Timeless.Result.Ok(
            normalize_content_list_response(
              response.data,
              page_.value,
              page_size_.value,
            ),
          );
        },
      },
    );

    const page_count_ = computed(
      { total: total_, pageSize: page_size_ },
      (state) =>
        Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
    );
    const range_text_ = computed(
      {
        total: total_,
        page: page_,
        pageSize: page_size_,
        count: computed(contents_, (items) => items.length),
      },
      (state) => {
        if (!state.total || !state.count) {
          return `共 ${state.total || 0} 条`;
        }
        const start = (state.page - 1) * state.pageSize + 1;
        const end = start + state.count - 1;
        return `第 ${start}-${end} 条，共 ${state.total} 条`;
      },
    );

    async function load(targetPage = page_.value) {
      const sequence = ++request_sequence;
      const requestedPage = Math.max(1, Number(targetPage) || 1);
      loading_.as(true);
      error_.as("");
      const params = {
        page: requestedPage,
        page_size: page_size_.value,
      };
      const keyword = String(keyword_.value || "").trim();
      const contentType = String(content_type_.value || "").trim();
      if (keyword) {
        params.keyword = keyword;
      }
      if (contentType) {
        params.content_type = contentType;
      }

      const result = await list_request.run(params);
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      if (result.error) {
        error_.as(result.error.message || String(result.error));
        return result;
      }

      const data = result.data;
      contents_.as(data.list.map(normalize_content_item), { reset: true });
      total_.as(data.total);
      page_.as(data.page);
      page_size_.as(data.page_size);
      return result;
    }

    const methods = {
      ready() {
        return load(1);
      },
      refresh() {
        return load(page_.value);
      },
      search() {
        return load(1);
      },
      setKeyword(value) {
        keyword_.as(String(value || ""));
      },
      setPageSize(value) {
        const size = Number(value);
        page_size_.as([12, 24, 48, 96].includes(size) ? size : PAGE_SIZE_DEFAULT);
        return load(1);
      },
      previousPage() {
        if (page_.value <= 1 || loading_.value) {
          return null;
        }
        return load(page_.value - 1);
      },
      nextPage() {
        if (page_.value >= page_count_.value || loading_.value) {
          return null;
        }
        return load(page_.value + 1);
      },
      openSource(content) {
        if (!content || !content.url) {
          return;
        }
        window.open(content.url, "_blank", "noopener,noreferrer");
      },
      platformName: content_platform_name,
      typeLabel: content_type_label,
      downloadStatus: content_download_status,
      formatTime: format_content_time,
      formatBytes: format_content_bytes,
      accountRoleLabel: content_account_role_label,
      formatCount: format_content_count,
    };

    return {
      state: {
        contents: contents_,
        total: total_,
        page: page_,
        page_size: page_size_,
        page_count: page_count_,
        range_text: range_text_,
        keyword: keyword_,
        content_type: content_type_,
        loading: loading_,
        error: error_,
      },
      ui: {
        content_type: content_type_select_,
      },
      methods,
      ready: methods.ready,
    };
  }

  return create_model;
})();

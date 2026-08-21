const content_request = Timeless.kit.request_factory({
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

function first_non_empty(...values) {
  for (const value of values) {
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
  }
  return "";
}

function content_detail_href(content) {
  const id = String(first_non_empty(content && content.id, content && content.ID)).trim();
  return id ? `/content/detail?id=${encodeURIComponent(id)}` : "";
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

function normalize_content_account(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    id: first_non_empty(source.id, source.ID),
    external_id: first_non_empty(source.external_id, source.ExternalID),
    alias: first_non_empty(source.alias, source.Alias),
    nickname: first_non_empty(
      source.nickname,
      source.Nickname,
      source.name,
      source.Name,
    ),
    avatar_url: first_non_empty(source.avatar_url, source.AvatarURL),
    profile_url: first_non_empty(source.profile_url, source.ProfileURL),
  };
}

function normalize_content_item(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const accounts_source = Array.isArray(source.accounts)
    ? source.accounts
    : Array.isArray(source.Accounts)
      ? source.Accounts
      : [];
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : Array.isArray(source.DownloadTasks)
      ? source.DownloadTasks
      : [];
  const file_count = Math.max(
    0,
    number_or_default(
      first_non_empty(source.file_count, source.fileCount, source.FileCount),
      0,
    ),
  );

  return {
    ...source,
    id: first_non_empty(source.id, source.ID),
    platform_id: first_non_empty(source.platform_id, source.PlatformID),
    platform_name: first_non_empty(source.platform_name, source.PlatformName),
    content_type: first_non_empty(
      source.content_type,
      source.ContentType,
      source.type,
      source.Type,
    ),
    content_subtype: first_non_empty(
      source.content_subtype,
      source.ContentSubtype,
      source.subtype,
      source.Subtype,
    ),
    title: first_non_empty(
      source.title,
      source.Title,
      source.description,
      source.Description,
      "未命名内容",
    ),
    description: first_non_empty(source.description, source.Description),
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
    publish_time: number_or_default(
      first_non_empty(source.publish_time, source.PublishTime),
      0,
    ),
    accounts: accounts_source.map(normalize_content_account),
    download_tasks: tasks,
    file_count,
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
    ttk: "TT看书",
  };
  return names[content.platform_id] || content.platform_id || "未知平台";
}

function content_type_label(value, subtypeValue) {
  const type = String(value || "")
    .trim()
    .toLowerCase();
  const subtype = String(subtypeValue || "")
    .trim()
    .toLowerCase();
  const labels = {
    video: "视频",
    long_video: "长视频",
    episode: "单集",
    series: "系列",
    collection: "合集",
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
  return labels[subtype] || labels[type] || subtype || type || "内容";
}

function normalize_content_download_status(value) {
  const status = String(value ?? "")
    .trim()
    .toLowerCase();
  if (["2", "4", "running", "downloading", "merging"].includes(status)) {
    return "running";
  }
  if (["3", "pause", "paused"].includes(status)) return "paused";
  if (["5", "done", "finished", "completed", "success"].includes(status)) {
    return "succeeded";
  }
  if (
    [
      "6",
      "7",
      "error",
      "fail",
      "failed",
      "failure",
      "cancelled",
      "canceled",
    ].includes(status)
  ) {
    return "failed";
  }
  return status;
}

function content_statistics(content) {
  const source = content && typeof content === "object" ? content : {};
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : [];
  const statistics = {
    total_tasks: tasks.length,
    in_progress: 0,
    failed: 0,
    files: Math.max(0, number_or_default(source.file_count, 0)),
  };
  tasks.forEach((task) => {
    const status = normalize_content_download_status(
      first_non_empty(task && task.status, task && task.Status),
    );
    if (status === "running" || status === "paused") {
      statistics.in_progress += 1;
    } else if (status === "failed") {
      statistics.failed += 1;
    }
  });
  return statistics;
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

function ContentViewModel(props) {
  const PAGE_SIZE_DEFAULT = 24;
  const contents_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref("");
  const content_type_ = ref("");
  const scope_ = ref("task");
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  const detail_id_ = ref("");
  let request_sequence = 0;

  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索标题或描述",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
    }),
    checkbox_all$: new Timeless.vm.CheckboxCore({
      checked: scope_.value === "all",
      onChange(value) {
        set_all_scope(value);
      },
    }),
    select_content_type$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部类型",
      options: [
        new Timeless.vm.SelectItemCore({ label: "全部类型", value: "" }),
        new Timeless.vm.SelectItemCore({ label: "视频", value: "video" }),
        new Timeless.vm.SelectItemCore({
          label: "短视频",
          value: "short_video",
        }),
        new Timeless.vm.SelectItemCore({ label: "图集", value: "image_set" }),
        new Timeless.vm.SelectItemCore({ label: "文章", value: "article" }),
        new Timeless.vm.SelectItemCore({ label: "小说", value: "novel" }),
        new Timeless.vm.SelectItemCore({ label: "音频", value: "audio" }),
        new Timeless.vm.SelectItemCore({ label: "直播", value: "live" }),
      ],
      onChange(value) {
        content_type_.as(String(value || ""));
        load(1);
      },
    }),
    btn_search$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "primary",
    }),
    btn_refresh$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
      onClick() {
        return load(1);
      },
    }),
    btn_retry$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "primary",
      onClick() {
        return load(page_.value);
      },
    }),
    contentDetailDrawer$: new Timeless.vm.DialogCore({
      title: "内容详情",
      closeable: true,
      footer: false,
    }),
  };

  keyword_.subscribe({
    onChange(value) {
      if (ui.input_keyword$.value !== value) {
        ui.input_keyword$.setValue(value, { silence: true });
      }
    },
  });
  scope_.subscribe({
    onChange(value) {
      const checked = value === "all";
      if (ui.checkbox_all$.checked !== checked) {
        ui.checkbox_all$.setValue(checked, { silence: true });
      }
    },
  });
  loading_.subscribe({
    onChange(loading) {
      [ui.btn_search$, ui.btn_refresh$, ui.btn_retry$].forEach((button) => {
        if (loading) {
          button.disable();
        } else {
          button.enable();
        }
      });
    },
  });

  const list_request = new Timeless.kit.RequestCore(
    (params) => content_request.get("/api/content/list", params),
    {
      client: props.client,
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

  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const range_text_ = combine(
    {
      total: total_,
      page: page_,
      pageSize: page_size_,
      count: computed(contents_, (contents) => contents.length),
    },
    (state) => {
      if (!state.total || !state.count) {
        return `共 ${state.total || 0} 条`;
      }
      const start = (state.page - 1) * state.pageSize + 1;
      return `第 ${start}-${start + state.count - 1} 条，共 ${state.total} 条`;
    },
  );
  const list_status_ = combine(
    {
      initial: initial_,
      error: error_,
      contents: contents_,
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error) return "error";
      return state.contents.length > 0 ? "normal" : "empty";
    },
  );

  async function load(targetPage = page_.value) {
    const sequence = ++request_sequence;
    const requestedPage = Math.max(1, Number(targetPage) || 1);
    loading_.as(true);

    const params = {
      page: requestedPage,
      page_size: page_size_.value,
      scope: scope_.value,
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
    if (result.error) {
      error_.as(result.error.message || String(result.error));
      loading_.as(false);
      initial_.as(false);
      return result;
    }

    const data = result.data;
    error_.as("");
    contents_.as(data.list.map(normalize_content_item), { reset: true });
    total_.as(data.total);
    page_.as(data.page);
    page_size_.as(data.page_size);
    loading_.as(false);
    initial_.as(false);
    return result;
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function set_all_scope(value) {
    scope_.as(value ? "all" : "task");
    return load(1);
  }

  const methods = {
    ready() {
      return load(1);
    },
    refresh() {
      return load(1);
    },
    search() {
      return load(1);
    },
    setKeyword: set_keyword,
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
    openDetail(content) {
      const id = String(
        first_non_empty(content && content.id, content && content.ID),
      ).trim();
      if (!id) return;
      detail_id_.as(id);
      ui.contentDetailDrawer$.show();
    },
    detailHref: content_detail_href,
    platformName: content_platform_name,
    typeLabel: content_type_label,
    statistics: content_statistics,
    formatTime: format_content_time,
  };

  const state = {
    contents: contents_,
    total: total_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    scope: scope_,
    initial: initial_,
    status: list_status_,
    loading: loading_,
    error: error_,
    detail_id: detail_id_,
  };

  return { state, ui, methods };
}

export { ContentViewModel };

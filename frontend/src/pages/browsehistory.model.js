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
    ),
    source_url: first_non_empty(
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
  const key = history && (history.platform_id || history.platformId);
  return window.PLATFORM_FAVICONS[key] || "";
}

function browse_history_platform_name(history) {
  if (history.platform_name) {
    return history.platform_name;
  }
  return (
    window.PLATFORM_NAMES[history.platform_id] ||
    history.platform_id ||
    "未知平台"
  );
}

function browse_history_type_label(value) {
  const type = String(value || "")
    .trim()
    .toLowerCase();
  return window.CONTENT_TYPE_NAMES[type] || type || "内容";
}

function normalize_author_name(raw) {
  const name = first_non_empty(
    raw && raw.author_nickname,
    raw && raw.author_external_id,
    "未知作者",
  );
  return name;
}

function BrowseHistoryViewModel(props) {
  const PAGE_SIZE_DEFAULT = 24;
  const histories_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref("");
  const platform_id_ = ref("");
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  let request_sequence = 0;

  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索标题、账号或链接",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
      onEnter() {
        return methods.search();
      },
    }),
    select_platform$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部平台",
      position: "item-aligned",
      options: [
        ["", "全部平台"],
        ...Object.entries(window.PLATFORM_NAMES),
      ].map(
        ([value, label]) => new Timeless.vm.SelectItemCore({ label, value }),
      ),
      onChange(value) {
        platform_id_.as(String(value || ""));
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
        keyword_.as("");
        return load(1);
      },
    }),
    btn_retry$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
      onClick() {
        return load(page_.value);
      },
    }),
  };

  keyword_.subscribe({
    onChange(value) {
      if (ui.input_keyword$.value !== value) {
        ui.input_keyword$.setValue(value, { silence: true });
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
    (params) =>
      window.request.post("/api/browse_history/list", params),
    {
      client: props.client,
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
      count: computed(histories_, (histories) => histories.length),
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
      histories: histories_,
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error) return "error";
      return state.histories.length > 0 ? "normal" : "empty";
    },
  );

  async function load(targetPage = page_.value) {
    const sequence = ++request_sequence;
    const requestedPage = Math.max(1, Number(targetPage) || 1);
    loading_.as(true);
    const params = {
      page: requestedPage,
      page_size: page_size_.value,
    };
    const platformId = String(platform_id_.value || "").trim();
    if (platformId) {
      params.platform_id = platformId;
    }
    const keyword = String(keyword_.value || "").trim();
    if (keyword) {
      params.keyword = keyword;
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
    histories_.as(data.list.map(normalize_browse_history_item), { reset: true });
    total_.as(data.total);
    page_.as(data.page || requestedPage);
    page_size_.as(data.page_size);
    loading_.as(false);
    initial_.as(false);
    return result;
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function change_page(target_page) {
    const page = Math.min(
      page_count_.value,
      Math.max(1, Number(target_page) || 1),
    );
    if (page === page_.value || loading_.value) return null;
    return load(page);
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
    changePage: change_page,
    previousPage() {
      return change_page(page_.value - 1);
    },
    nextPage() {
      return change_page(page_.value + 1);
    },
    openSource(history) {
      if (!history || !history.source_url) {
        return;
      }
      window.open(history.source_url, "_blank", "noopener,noreferrer");
    },
    platformFavicon: browse_history_platform_favicon,
    platformName: browse_history_platform_name,
    typeLabel: browse_history_type_label,
    formatTime: window.format_time,
    authorName: normalize_author_name,
  };

  const state = {
    histories: histories_,
    total: total_,
    page: page_,
    page_size: page_size_,
    keyword: keyword_,
    page_count: page_count_,
    range_text: range_text_,
    initial: initial_,
    status: list_status_,
    platform_id: platform_id_,
    loading: loading_,
    error: error_,
  };

  return { state, ui, methods };
}

export { BrowseHistoryViewModel };

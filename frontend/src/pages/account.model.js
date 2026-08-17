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

function account_search_from_location(query) {
  if (query && typeof query === "object") {
    const account_id = String(
      first_non_empty(query.id, query.account_id),
    ).trim();
    return {
      keyword: String(first_non_empty(query.keyword, account_id)),
      account_id,
    };
  }
  try {
    const params = new URLSearchParams(window.location.search);
    const account_id = params.get("id") || params.get("account_id") || "";
    return {
      keyword: params.get("keyword") || account_id,
      account_id,
    };
  } catch {
    return { keyword: "", account_id: "" };
  }
}

function normalize_account_list_response(data, fallbackPage, fallbackSize) {
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
    ttk: "TT看书",
  };
  const platform_id = String((account && account.platform_id) || "").trim();
  return names[platform_id] || platform_id || "未知平台";
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

const request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    if (payload.code !== 0) {
      return Timeless.Result.Err(
        payload.msg,
        payload.code,
        payload.data,
      );
    }
    return Timeless.Result.Ok(payload.data || {});
  },
});

function AccountViewModel(props) {
  const PAGE_SIZE_DEFAULT = 24;
  const initial_search = account_search_from_location(
    props.view && props.view.query,
  );
  const accounts_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref(initial_search.keyword);
  const account_id_ = ref(initial_search.account_id);
  const loading_ = ref(false);
  const error_ = ref("");
  let request_sequence = 0;

  const reqs = {
    account: {
      list: new Timeless.kit.RequestCore(
        (params) => request.get("/api/account/list", params),
        { client: props.client },
      ),
    },
  };

  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const initial_loading_ = combine(
    { loading: loading_, accounts: accounts_ },
    (state) => state.loading && state.accounts.length === 0,
  );
  const range_text_ = combine(
    {
      total: total_,
      page: page_,
      pageSize: page_size_,
      count: computed(accounts_, (accounts) => accounts.length),
    },
    (state) => {
      if (!state.total || !state.count) {
        return `共 ${state.total || 0} 个账号`;
      }
      const start = (state.page - 1) * state.pageSize + 1;
      return `第 ${start}-${start + state.count - 1} 个，共 ${state.total} 个账号`;
    },
  );

  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索昵称或账号 ID",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
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

  function sync_search_location() {
    try {
      const url = new URL(window.location.href);
      const keyword = String(keyword_.value || "").trim();
      const account_id = String(account_id_.value || "").trim();
      if (keyword && keyword !== account_id) {
        url.searchParams.set("keyword", keyword);
      } else {
        url.searchParams.delete("keyword");
      }
      if (account_id) {
        url.searchParams.set("id", account_id);
      } else {
        url.searchParams.delete("id");
      }
      url.searchParams.delete("account_id");
      window.history.replaceState(
        window.history.state,
        "",
        `${url.pathname}${url.search}${url.hash}`,
      );
    } catch {
      // Searching still works if browser history is unavailable.
    }
  }

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
    if (keyword && keyword !== account_id_.value) {
      params.keyword = keyword;
    }
    const account_id = String(account_id_.value || "").trim();
    if (account_id) {
      params.account_id = account_id;
    }
    const r = await reqs.account.list.run(params);
    if (sequence !== request_sequence) {
      return r;
    }
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return r;
    }
    const data = normalize_account_list_response(
      r.data,
      requestedPage,
      page_size_.value,
    );
    accounts_.as(data.list.map(normalize_account_item), { reset: true });
    total_.as(data.total);
    page_.as(data.page);
    page_size_.as(data.page_size);
    return r;
  }

  function set_keyword(value) {
    const keyword = String(value || "");
    if (keyword !== keyword_.value) {
      account_id_.as("");
    }
    keyword_.as(keyword);
  }

  const methods = {
    ready() {
      return load(1);
    },
    refresh() {
      return load(1);
    },
    search() {
      sync_search_location();
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
    platformName: account_platform_name,
    formatTime: format_account_time,
    formatContentCount: format_content_count,
  };

  const state = {
    accounts: accounts_,
    total: total_,
    page: page_,
    page_size: page_size_,
    keyword: keyword_,
    page_count: page_count_,
    initial_loading: initial_loading_,
    range_text: range_text_,
    loading: loading_,
    error: error_,
  };

  return { state, ui, methods };
}

export { AccountViewModel };

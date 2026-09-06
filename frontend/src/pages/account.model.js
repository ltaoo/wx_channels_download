import { proxy_image_url } from "@/image-proxy.model.js";
import {
  content_type_label,
  normalize_content_item,
} from "./content.model.js";

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

function select_item(label, value) {
  return new Timeless.vm.SelectItemCore({ label, value });
}

function select_search(placeholder) {
  return new Timeless.vm.InputCore({
    defaultValue: "",
    placeholder,
    allowClear: true,
    autocomplete: false,
  });
}

function account_search_from_query(query = {}) {
  const account_id = String(
    first_non_empty(query.id, query.account_id),
  ).trim();
  return {
    keyword: String(first_non_empty(query.keyword, account_id)),
    account_id,
    platform_id: String(
      first_non_empty(query.platform_id, query.platform),
    ).trim(),
  };
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
  const platform_id = first_non_empty(
    source.platform_id,
    source.platformId,
    source.PlatformID,
  );
  return {
    ...source,
    id: first_non_empty(source.id, source.ID),
    platform_id,
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
    avatar_url: proxy_image_url(
      platform_id,
      first_non_empty(source.avatar_url, source.avatarUrl, source.AvatarURL),
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
  const platform_id = String((account && account.platform_id) || "").trim();
  return window.PLATFORM_NAMES[platform_id] || platform_id || "未知平台";
}

function format_content_count(value) {
  return `${Math.max(0, number_or_default(value, 0))} 条`;
}

function account_home_default_scope(account) {
  const platform_id = String(account && account.platform_id || "").trim();
  if (platform_id === "zhihu") return "answers";
  if (platform_id === "bilibili") return "video";
  if (platform_id === "douyin") return "posts";
  return "feed";
}

function normalize_home_tab(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    scope: String(
      first_non_empty(source.value, source.Value, source.scope, source.Scope),
    ).trim(),
    name: String(
      first_non_empty(
        source.label,
        source.Label,
        source.name,
        source.Name,
        source.value,
        source.scope,
      ),
    ).trim(),
    content_types: Array.isArray(source.content_types)
      ? source.content_types.map((item) => String(item || "").trim()).filter(Boolean)
      : [],
  };
}

function normalize_home_detail_content(raw, account) {
  const source = raw && typeof raw === "object" ? raw : {};
  const object_desc_source = first_non_empty(
    source.objectDesc,
    source.object_desc,
    source.ObjectDesc,
  );
  const object_desc = object_desc_source && typeof object_desc_source === "object"
    ? object_desc_source
    : {};
  const media_list = Array.isArray(object_desc.media) ? object_desc.media : [];
  const media = media_list[0] && typeof media_list[0] === "object"
    ? media_list[0]
    : {};
  const media_type = number_or_default(
    first_non_empty(object_desc.mediaType, media.mediaType),
    0,
  );
  const declared_type = String(
    first_non_empty(
      source.content_type,
      source.contentType,
      source.object_type,
      source.objectType,
      source.type,
      source.Type,
    ),
  ).trim().toLowerCase();
  const content_type = declared_type || (media_type === 2
    ? "album"
    : media_type === 9
      ? "live"
      : media_list.length > 0
        ? "video"
        : "text");
  const media_url = String(first_non_empty(media.url, media.URL)).trim();
  const media_token = String(
    first_non_empty(media.urlToken, media.URLToken),
  ).trim();
  const preview_images = media_list.map((item) => {
    const item_url = String(first_non_empty(item.url, item.URL)).trim();
    const item_token = String(
      first_non_empty(item.urlToken, item.URLToken),
    ).trim();
    return proxy_image_url(
      account && account.platform_id,
      first_non_empty(
        item.thumbUrl,
        item.coverUrl,
        item.thumb_url,
        item.cover_url,
        item_url ? `${item_url}${item_token}` : "",
      ),
    );
  }).filter(Boolean);
  const fallback_cover_url = proxy_image_url(
    account && account.platform_id,
    first_non_empty(
      source.cover_url,
      source.coverUrl,
      source.CoverURL,
      source.image_url,
      source.imageUrl,
    ),
  );
  if (preview_images.length === 0 && fallback_cover_url) {
    preview_images.push(fallback_cover_url);
  }
  const media_width = number_or_default(
    first_non_empty(media.width, media.Width),
    0,
  );
  const media_height = number_or_default(
    first_non_empty(media.height, media.Height),
    0,
  );

  return normalize_content_item({
    ...source,
    id: first_non_empty(source.id, source.ID),
    external_id: first_non_empty(source.id, source.ID),
    platform_id: account && account.platform_id,
    content_type,
    title: first_non_empty(
      source.title,
      source.Title,
      object_desc.description,
      source.description,
    ),
    description: first_non_empty(
      source.description,
      source.Description,
      object_desc.description,
    ),
    source_url: first_non_empty(
      source.source_url,
      source.sourceUrl,
      media_url ? `${media_url}${media_token}` : "",
    ),
    cover_url: first_non_empty(
      media.thumbUrl,
      media.coverUrl,
      media.thumb_url,
      media.cover_url,
      source.cover_url,
      source.coverUrl,
      source.CoverURL,
      source.image_url,
      source.imageUrl,
    ),
    publish_time: first_non_empty(
      source.createtime,
      source.createTime,
      source.publish_time,
    ),
    preview_images,
    preview_aspect_ratio: media_width > 0 && media_height > 0
      ? media_width / media_height
      : 0,
    duration: number_or_default(
      first_non_empty(
        media.videoPlayLen,
        media.duration,
        source.duration,
      ),
      0,
    ),
    account_name: first_non_empty(
      source.contact && source.contact.nickname,
      account && account.nickname,
    ),
  });
}

function normalize_home_details_response(data) {
  const source = data && typeof data === "object" ? data : {};
  return {
    scopes: Array.isArray(source.scopes) ? source.scopes : [],
    scope: String(first_non_empty(source.scope, source.Scope)).trim(),
    contents: Array.isArray(source.contents) ? source.contents : [],
    next_marker: String(
      first_non_empty(source.next_marker, source.nextMarker),
    ),
  };
}

function AccountViewModel(props) {
  const PAGE_SIZE_DEFAULT = 50;
  const initial_search = account_search_from_query(
    props.view && props.view.query,
  );
  const accounts_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref(initial_search.keyword);
  const account_id_ = ref(initial_search.account_id);
  const platform_id_ = ref(initial_search.platform_id);
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  const copied_account_id_ = ref("");
  const selected_account_ = ref(null);
  const drawer_tabs_ = refarr([]);
  const drawer_scope_ = ref("");
  const drawer_contents_ = refarr([]);
  const drawer_initial_ = ref(true);
  const drawer_loading_ = ref(false);
  const drawer_loading_more_ = ref(false);
  const drawer_next_marker_ = ref("");
  const drawer_more_error_ = ref("");
  const drawer_error_ = ref("");
  let request_sequence = 0;
  let drawer_request_sequence = 0;
  let copy_feedback_timer = null;

  const reqs = {
    account: {
      list: new Timeless.kit.RequestCore(
        (params) => window.request.get("/api/account/list", params),
        { client: props.client },
      ),
      details: new Timeless.kit.RequestCore(
        (request) => window.request.get(
          `/api/account/${encodeURIComponent(request.scope)}/content/list`,
          request.params,
        ),
        { client: props.client },
      ),
    },
  };

  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const list_status_ = combine(
    {
      initial: initial_,
      error: error_,
      accounts: accounts_,
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error) return "error";
      return state.accounts.length === 0 ? "empty" : "normal";
    },
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
  const drawer_status_ = combine(
    {
      initial: drawer_initial_,
      error: drawer_error_,
      count: computed(drawer_contents_, (contents) => contents.length),
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error && state.count === 0) return "error";
      return state.count === 0 ? "empty" : "normal";
    },
  );
  const drawer_empty_description_ = computed(
    drawer_scope_,
    (scope) => scope ? "该分类暂未返回内容" : "点击上方 tab 获取对应内容",
  );

  const ui = {
    select_platform$: new Timeless.vm.SelectCore({
      defaultValue: platform_id_.value,
      placeholder: "全部平台",
      search: select_search("搜索平台"),
      position: "popper",
      options: [
        ["", "全部平台"],
        ...Object.entries(window.PLATFORM_NAMES || {}),
      ].map(([value, label]) => select_item(label, value)),
      onChange(value) {
        const platform_id = String(value || "");
        if (platform_id === platform_id_.value) return null;
        platform_id_.as(platform_id);
        sync_search_location();
        return load(1);
      },
    }),
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索昵称或账号 ID",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
      onEnter() {
        return methods.search();
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
        set_keyword("");
        platform_id_.as("");
        ui.select_platform$.setValue("");
        sync_search_location();
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
    account_contents_drawer$: new Timeless.vm.DialogCore({
      title: "账号内容",
      closeable: true,
      footer: false,
    }),
    btn_drawer_retry$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        const account = selected_account_.value;
        return load_account_contents(
          account,
          drawer_scope_.value || account_home_default_scope(account),
        );
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
  drawer_loading_.subscribe({
    onChange(loading) {
      ui.btn_drawer_retry$.setLoading(Boolean(loading));
    },
  });
  function sync_search_location() {
    const keyword = String(keyword_.value || "").trim();
    const account_id = String(account_id_.value || "").trim();
    const platform_id = String(platform_id_.value || "").trim();
    const query = {};
    if (keyword && keyword !== account_id) query.keyword = keyword;
    if (account_id) query.id = account_id;
    if (platform_id) query.platform_id = platform_id;
    const search = Timeless.utils.qs_stringify(query);
    props.history.$router.replaceState(
      `${(props.view && props.view.pathname) || "/account"}${search ? `?${search}` : ""}`,
    );
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
    const platform_id = String(platform_id_.value || "").trim();
    if (platform_id) {
      params.platform_id = platform_id;
    }
    const r = await reqs.account.list.run(params);
    if (sequence !== request_sequence) {
      return r;
    }
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      initial_.as(false);
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
    initial_.as(false);
    return r;
  }

  function set_keyword(value) {
    const keyword = String(value || "");
    if (keyword !== keyword_.value) {
      account_id_.as("");
    }
    keyword_.as(keyword);
  }

  function change_page(target_page) {
    const page = Math.min(
      page_count_.value,
      Math.max(1, Number(target_page) || 1),
    );
    if (page === page_.value || loading_.value) return null;
    return load(page);
  }

  async function load_account_contents(account, scope, options = {}) {
    if (!account || !account.id || !scope) return null;
    const append = Boolean(options.append);
    if (append && drawer_loading_more_.value) return null;
    const sequence = ++drawer_request_sequence;
    drawer_scope_.as(String(scope));
    drawer_loading_more_.as(append);
    drawer_more_error_.as("");
    if (!append) {
      drawer_initial_.as(true);
      drawer_loading_.as(true);
      drawer_error_.as("");
      drawer_next_marker_.as("");
      drawer_contents_.as([], { reset: true });
    }

    const result = await reqs.account.details.run({
      scope: String(scope),
      params: {
        id: account.id,
        page: String(
          options.page || (append ? drawer_next_marker_.value : ""),
        ),
      },
    });
    if (sequence !== drawer_request_sequence) return result;
    drawer_loading_.as(false);
    drawer_loading_more_.as(false);
    drawer_initial_.as(false);
    if (result.error) {
      const message = result.error.message || String(result.error);
      if (append) {
        drawer_more_error_.as(message);
      } else {
        drawer_error_.as(message);
      }
      return result;
    }
    const details = normalize_home_details_response(result.data);
    drawer_tabs_.as(
      details.scopes.map(normalize_home_tab).filter((tab) => tab.scope),
      { reset: true },
    );
    drawer_scope_.as(details.scope || String(scope));
    const loaded_contents = details.contents.map((content) =>
      normalize_home_detail_content(content, account)
    );
    const contents = append
      ? [...drawer_contents_.value, ...loaded_contents]
      : loaded_contents;
    const unique_contents = [];
    const seen = new Set();
    contents.forEach((content, index) => {
      const key = String(content.id || `${content.content_type}:${index}`);
      if (seen.has(key)) return;
      seen.add(key);
      unique_contents.push(content);
    });
    drawer_contents_.as(unique_contents, { reset: true });
    drawer_next_marker_.as(details.next_marker);
    return result;
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
    changePage: change_page,
    previousPage() {
      return change_page(page_.value - 1);
    },
    nextPage() {
      return change_page(page_.value + 1);
    },
    copyId(account) {
      const result = props.app.copy(account.id);
      copied_account_id_.as(account.id);
      clearTimeout(copy_feedback_timer);
      copy_feedback_timer = setTimeout(() => copied_account_id_.as(""), 3000);
      return result;
    },
    openAccount(account) {
      const scope = account_home_default_scope(account);
      selected_account_.as(account);
      drawer_tabs_.as([], { reset: true });
      drawer_scope_.as(scope);
      ui.account_contents_drawer$.show();
      return load_account_contents(account, scope);
    },
    selectHomeTab(tab) {
      const scope = String((tab && tab.scope) || "").trim();
      if (
        !scope ||
        drawer_loading_.value ||
        drawer_loading_more_.value
      ) return null;
      return load_account_contents(selected_account_.value, scope);
    },
    loadMoreAccountContents() {
      const account = selected_account_.value;
      const scope = drawer_scope_.value;
      const page = drawer_next_marker_.value;
      if (!account || !scope || !page || drawer_loading_more_.value) return null;
      return load_account_contents(account, scope, { append: true, page });
    },
    openContent(content) {
      if (content && content.url) props.app.openWindow(content.url);
    },
    platformName: account_platform_name,
    contentTypeLabel: content_type_label,
    formatTime: window.format_time,
    formatContentCount: format_content_count,
  };

  const state = {
    accounts: accounts_,
    total: total_,
    page: page_,
    page_size: page_size_,
    keyword: keyword_,
    platform_id: platform_id_,
    page_count: page_count_,
    initial: initial_,
    status: list_status_,
    range_text: range_text_,
    loading: loading_,
    error: error_,
    copied_account_id: copied_account_id_,
    selected_account: selected_account_,
    drawer_tabs: drawer_tabs_,
    drawer_scope: drawer_scope_,
    drawer_contents: drawer_contents_,
    drawer_next_marker: drawer_next_marker_,
    drawer_loading_more: drawer_loading_more_,
    drawer_more_error: drawer_more_error_,
    drawer_error: drawer_error_,
    drawer_status: drawer_status_,
    drawer_empty_description: drawer_empty_description_,
  };

  return { state, ui, methods };
}

export { AccountViewModel };

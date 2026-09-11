import { request } from "@/biz/request.js";
import { proxy_image_url } from "@/image-proxy.model.js";

import { task_status } from "./content_detail.model.js";

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

function normalize_filter_account(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const platform_id = first_non_empty(
    source.platform_id,
    source.platformId,
    source.PlatformID,
  );
  return {
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
      source.id,
      source.ID,
      "未命名账号",
    ),
    avatar_url: proxy_image_url(
      platform_id,
      first_non_empty(source.avatar_url, source.avatarUrl, source.AvatarURL),
    ),
  };
}

function select_item(label, value, data = {}) {
  const item = new Timeless.vm.SelectItemCore({ label, value });
  Object.assign(item, data);
  return item;
}

function select_search(placeholder) {
  return new Timeless.vm.InputCore({
    defaultValue: "",
    placeholder,
    allowClear: true,
    autocomplete: false,
  });
}

function content_detail_href(content) {
  const id = String(
    first_non_empty(content && content.id, content && content.ID),
  ).trim();
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

function normalize_content_account(raw, fallback_platform_id) {
  const source = raw && typeof raw === "object" ? raw : {};
  const platform_id = first_non_empty(
    source.platform_id,
    source.PlatformID,
    fallback_platform_id,
  );
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
    avatar_url: proxy_image_url(
      platform_id,
      first_non_empty(source.avatar_url, source.AvatarURL),
    ),
    profile_url: first_non_empty(source.profile_url, source.ProfileURL),
  };
}

function normalize_content_item(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const platform_id = first_non_empty(source.platform_id, source.PlatformID);
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
    platform_id,
    platform_name: first_non_empty(source.platform_name, source.PlatformName),
    tags: normalize_content_tags(source),
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
    cover_url: proxy_image_url(
      platform_id,
      first_non_empty(source.cover_url, source.CoverURL, source.coverUrl),
    ),
    publish_time: number_or_default(
      first_non_empty(source.publish_time, source.PublishTime),
      0,
    ),
    accounts: accounts_source.map((account) =>
      normalize_content_account(account, platform_id),
    ),
    download_tasks: tasks,
    file_count,
  };
}

function normalize_content_tags(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const list = Array.isArray(source.tags)
    ? source.tags
    : Array.isArray(source.Tags)
      ? source.Tags
      : [];
  return list.map((tag) => {
    const item = tag && typeof tag === "object" ? tag : {};
    return {
      id: number_or_default(first_non_empty(item.id, item.ID), 0),
      name: first_non_empty(item.name, item.Name, item.tag, item.Tag, ""),
    };
  });
}

function content_platform_name(content) {
  if (content.platform_name) {
    return content.platform_name;
  }
  return (
    window.PLATFORM_NAMES[content.platform_id] ||
    content.platform_id ||
    "未知平台"
  );
}

function content_type_label(value, subtypeValue) {
  const type = String(value || "")
    .trim()
    .toLowerCase();
  const subtype = String(subtypeValue || "")
    .trim()
    .toLowerCase();
  return (
    window.CONTENT_TYPE_NAMES[subtype] ||
    window.CONTENT_TYPE_NAMES[type] ||
    subtype ||
    type ||
    "内容"
  );
}

function content_type_icon(value, subtypeValue) {
  const type = String(value || "")
    .trim()
    .toLowerCase();
  const subtype = String(subtypeValue || "")
    .trim()
    .toLowerCase();
  const has_icon = (key) =>
    Object.prototype.hasOwnProperty.call(window.CONTENT_TYPE_ICONS, key);
  if (has_icon(subtype)) return window.CONTENT_TYPE_ICONS[subtype];
  if (has_icon(type)) return window.CONTENT_TYPE_ICONS[type];
  return window.CONTENT_TYPE_ICONS.default;
}

function content_statistics(content) {
  const source = content && typeof content === "object" ? content : {};
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : [];
  const task_statuses = new Map();
  tasks.forEach((task) => {
    const status = task_status(task.status);
    const statistic = task_statuses.get(status.tone);
    if (statistic) {
      statistic.value += 1;
    } else {
      task_statuses.set(status.tone, {
        key: status.tone,
        label: `${status.label}任务`,
        value: 1,
      });
    }
  });
  return {
    task_statuses: Array.from(task_statuses.values()),
    files: Math.max(0, number_or_default(source.file_count, 0)),
  };
}

const saved_filter_fields = [
  "keyword",
  "content_type",
  "platform_id",
  "account_id",
  "scope",
];
const saved_filters_storage_key = "content.saved_filters";
const content_layout_storage_key = "content.layout";

function normalize_content_layout(value) {
  return value === "card" ? "card" : "table";
}

function load_content_layout() {
  try {
    return normalize_content_layout(
      window.localStorage.getItem(content_layout_storage_key),
    );
  } catch {
    return "table";
  }
}

function normalize_saved_filter(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const id = String(source.id || "")
    .trim()
    .slice(0, 100);
  if (!id) return null;

  const filter = {
    id,
    name:
      String(source.name || "")
        .trim()
        .slice(0, 60) || "未命名筛选器",
    platform_name: String(source.platform_name || "")
      .trim()
      .slice(0, 60),
    account_name: String(source.account_name || "")
      .trim()
      .slice(0, 120),
  };
  saved_filter_fields.forEach((field) => {
    filter[field] = String(source[field] || "").trim();
  });
  filter.scope = filter.scope === "all" ? "all" : "task";
  return filter;
}

function normalize_saved_filters(raw) {
  if (!Array.isArray(raw)) return [];
  const filters = [];
  const ids = new Set();
  raw.forEach((item) => {
    const filter = normalize_saved_filter(item);
    if (filter && !ids.has(filter.id)) {
      filters.push(filter);
      ids.add(filter.id);
    }
  });
  return filters;
}

function saved_filters_match(left, right) {
  return saved_filter_fields.every((field) => {
    const left_value = String(left?.[field] || "").trim();
    const right_value = String(right?.[field] || "").trim();
    return left_value === right_value;
  });
}

function load_saved_filters() {
  try {
    return normalize_saved_filters(
      JSON.parse(
        window.localStorage.getItem(saved_filters_storage_key) || "[]",
      ),
    );
  } catch {
    return [];
  }
}

function persist_saved_filters(filters) {
  try {
    window.localStorage.setItem(
      saved_filters_storage_key,
      JSON.stringify(filters),
    );
  } catch {
    // Local storage can be unavailable in private modes; quick filters remain usable for this page.
  }
}

function ContentViewModel(props) {
  const PAGE_SIZE_DEFAULT = 48;
  const contents_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref("");
  const content_type_ = ref("");
  const platform_id_ = ref("");
  const account_id_ = ref("");
  const scope_ = ref("task");
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  const detail_id_ = ref("");
  const copied_content_id_ = ref("");
  const saved_filters_ = refarr(load_saved_filters());
  const layout_ = ref(load_content_layout());
  let request_sequence = 0;
  let account_request_sequence = 0;
  let copy_feedback_timer = null;

  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索标题或描述",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
      onEnter() {
        return methods.search();
      },
    }),
    checkbox_all$: new Timeless.vm.CheckboxCore({
      checked: scope_.value === "all",
      onChange(value) {
        set_all_scope(value);
      },
    }),
    select_platform$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部平台",
      search: select_search("搜索平台"),
      position: "popper",
      options: [
        ["", "全部平台"],
        ...Object.entries(window.PLATFORM_NAMES || {}),
      ].map(([value, label]) => select_item(label, value)),
      onChange(value) {
        account_id_.as("");
        ui.select_account$.setValue("", { silence: true });
        platform_id_.as(String(value || ""));
        void load_accounts();
        return load(1);
      },
    }),
    select_account$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部账号",
      search: select_search("搜索账号"),
      position: "popper",
      options: [select_item("全部账号", "")],
      onChange(value) {
        account_id_.as(String(value || ""));
        return load(1);
      },
    }),
    select_content_type$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部类型",
      position: "item-aligned",
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
        reset_filters();
        void load_accounts();
        return load(1);
      },
    }),
    input_filter_name$: new Timeless.vm.InputCore({
      defaultValue: "",
      placeholder: "筛选器名称",
      allowClear: true,
      onEnter() {
        return save_current_filter();
      },
    }),
    btn_save_filter$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
    }),
    btn_layout$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "icon",
    }),
    dropdown_layout$: new Timeless.vm.DropdownMenuCore({
      trigger: "click",
      side: "bottom",
      align: "end",
      items: content_layout_items(),
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
  layout_.subscribe({
    onChange(layout) {
      ui.dropdown_layout$.setItems(content_layout_items());
      try {
        window.localStorage.setItem(content_layout_storage_key, layout);
      } catch {
        // Layout still works for this page when storage is unavailable.
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
    (params) => request.get("/api/content/list", params),
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
  const account_list_request = new Timeless.kit.RequestCore(
    (params) => request.get("/api/account/list", params),
    { client: props.client },
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
  const active_filter_id_ = combine(
    {
      filters: saved_filters_,
      keyword: keyword_,
      content_type: content_type_,
      platform_id: platform_id_,
      account_id: account_id_,
      scope: scope_,
    },
    (state) => {
      const current = {
        keyword: state.keyword,
        content_type: state.content_type,
        platform_id: state.platform_id,
        account_id: state.account_id,
        scope: state.scope,
      };
      return (
        state.filters.find((filter) => saved_filters_match(filter, current))
          ?.id || ""
      );
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
    const platform_id = String(platform_id_.value || "").trim();
    const account_id = String(account_id_.value || "").trim();
    if (keyword) {
      params.keyword = keyword;
    }
    if (contentType) {
      params.content_type = contentType;
    }
    if (platform_id) {
      params.platform_id = platform_id;
    }
    if (account_id) {
      params.account_id = account_id;
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

  function content_layout_items() {
    return ["table", "card"].map(
      (layout) =>
        new Timeless.vm.MenuItemCore({
          label: layout === "table" ? "表格布局" : "卡片布局",
          shortcut: layout_.value === layout ? "当前" : undefined,
          onClick() {
            set_content_layout(layout);
          },
        }),
    );
  }

  async function load_accounts() {
    const sequence = ++account_request_sequence;
    const selected_account_id = String(account_id_.value || "");
    const params = { page: 1, page_size: 200 };
    const platform_id = String(platform_id_.value || "").trim();
    if (platform_id) {
      params.platform_id = platform_id;
    }

    ui.select_account$.setLoading(true);
    const result = await account_list_request.run(params);
    if (sequence !== account_request_sequence) return result;

    const source = result.error ? {} : result.data || {};
    const list = Array.isArray(source.list)
      ? source.list
      : Array.isArray(source.List)
        ? source.List
        : [];
    const accounts = list.map(normalize_filter_account);
    const options = [
      select_item("全部账号", ""),
      ...accounts
        .filter((account) => account.id)
        .map((account) =>
          select_item(account.nickname, account.id, {
            account,
            search_text: [
              account.nickname,
              account.external_id,
              account.id,
            ].join(" "),
          }),
        ),
    ];
    const next_account_id = accounts.some(
      (account) => account.id === selected_account_id,
    )
      ? selected_account_id
      : "";
    account_id_.as(next_account_id);
    ui.select_account$.setOptions(options);
    ui.select_account$.setValue(next_account_id, { silence: true });
    ui.select_account$.setLoading(false);
    return result;
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function reset_filters() {
    set_keyword("");
    content_type_.as("");
    platform_id_.as("");
    account_id_.as("");
    ui.input_keyword$.setValue("", { silence: true });
    ui.select_content_type$.setValue("", { silence: true });
    ui.select_platform$.setValue("", { silence: true });
    ui.select_account$.setValue("", { silence: true });
  }

  function current_filter() {
    const account_id = String(account_id_.value || "").trim();
    const platform_id = String(platform_id_.value || "").trim();
    return {
      keyword: String(keyword_.value || "").trim(),
      content_type: String(content_type_.value || "").trim(),
      platform_id,
      account_id,
      scope: scope_.value === "all" ? "all" : "task",
      platform_name: platform_id
        ? window.PLATFORM_NAMES?.[platform_id] || platform_id
        : "",
      account_name: account_id
        ? ui.select_account$.state.selectedOption?.label || ""
        : "",
    };
  }

  function save_current_filter() {
    const name = String(ui.input_filter_name$.value || "").trim();
    if (!name) return;

    const filter = normalize_saved_filter({
      ...current_filter(),
      id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
      name,
    });
    const filters = [
      filter,
      ...saved_filters_.value.filter((item) => item.name !== filter.name),
    ];
    saved_filters_.as(filters, { reset: true });
    persist_saved_filters(filters);
    ui.input_filter_name$.setValue("", { silence: true });
  }

  async function apply_saved_filter(raw_filter) {
    const filter = normalize_saved_filter(raw_filter);
    if (!filter) return;

    keyword_.as(filter.keyword);
    content_type_.as(filter.content_type);
    platform_id_.as(filter.platform_id);
    account_id_.as(filter.account_id);
    scope_.as(filter.scope);
    ui.input_keyword$.setValue(filter.keyword, { silence: true });
    ui.select_content_type$.setValue(filter.content_type, { silence: true });
    ui.select_platform$.setValue(filter.platform_id, { silence: true });
    ui.select_account$.setValue("", { silence: true });

    await load_accounts();
    return load(1);
  }

  function delete_saved_filter(filter_id) {
    const filters = saved_filters_.value.filter(
      (filter) => filter.id !== filter_id,
    );
    saved_filters_.as(filters, { reset: true });
    persist_saved_filters(filters);
  }

  function saved_filter_summary(filter) {
    const items = [];
    if (filter.keyword) items.push(`关键词：${filter.keyword}`);
    if (filter.platform_id) {
      items.push(`平台：${filter.platform_name || filter.platform_id}`);
    }
    if (filter.account_id) {
      items.push(`账号：${filter.account_name || filter.account_id}`);
    }
    if (filter.content_type) {
      items.push(`类型：${content_type_label(filter.content_type)}`);
    }
    items.push(filter.scope === "all" ? "范围：所有内容" : "范围：任务内容");
    return items.join("；");
  }

  function set_all_scope(value) {
    scope_.as(value ? "all" : "task");
    return load(1);
  }

  function set_content_layout(value) {
    layout_.as(normalize_content_layout(value));
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
      return Promise.all([load_accounts(), load(1)]);
    },
    refresh() {
      return load(1);
    },
    search() {
      return load(1);
    },
    saveFilter: save_current_filter,
    applyFilter: apply_saved_filter,
    deleteFilter: delete_saved_filter,
    filterSummary: saved_filter_summary,
    setLayout: set_content_layout,
    setKeyword: set_keyword,
    changePage: change_page,
    previousPage() {
      return change_page(page_.value - 1);
    },
    nextPage() {
      return change_page(page_.value + 1);
    },
    copyId(content) {
      const result = props.app.copy(content.id);
      copied_content_id_.as(content.id);
      clearTimeout(copy_feedback_timer);
      copy_feedback_timer = setTimeout(() => copied_content_id_.as(""), 3000);
      return result;
    },
    openSource(content) {
      if (!content || !content.url) {
        return;
      }
      props.app.openWindow(content.url);
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
    typeIcon: content_type_icon,
    statistics: content_statistics,
    formatTime: window.format_time,
  };

  const state = {
    contents: contents_,
    total: total_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    scope: scope_,
    platform_id: platform_id_,
    account_id: account_id_,
    initial: initial_,
    status: list_status_,
    loading: loading_,
    error: error_,
    detail_id: detail_id_,
    copied_content_id: copied_content_id_,
    saved_filters: saved_filters_,
    active_filter_id: active_filter_id_,
    layout: layout_,
  };

  return { state, ui, methods };
}

export {
  ContentViewModel,
  content_type_label,
  normalize_content_item,
  normalize_content_list_response,
  normalize_content_layout,
  content_type_icon,
  normalize_saved_filter,
  normalize_saved_filters,
  saved_filters_match,
};

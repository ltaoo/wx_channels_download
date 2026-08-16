const active_job_storage_key = "scraper.active_scraper_job_id";
const platform_status_popover_hide_delay = 240;
const home_api_origin =
  (window.__d_config && window.__d_config.apiOrigin) || window.location.origin;

const home_request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(response) {
    if (response.error) {
      return Timeless.Result.Err(response.error);
    }
    const payload = response.data || {};
    if (payload.code !== 0) {
      return Timeless.Result.Err(
        payload.msg || "获取失败",
        payload.code,
        payload.data,
      );
    }
    return Timeless.Result.Ok(payload.data || {});
  },
});

const platform_names = {
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

const scraper_fetch_stage_names = {
  start: "创建任务",
  queued: "排队中",
  restore: "恢复任务",
  started: "准备解析",
  fetch: "请求平台",
  fetched: "平台已响应",
  raw: "原始内容",
  content: "内容信息",
  account: "账号信息",
  content_detail: "内容详情",
  cache: "抓取缓存",
  cache_entry: "缓存文件",
  download_preview: "下载预览",
  finished: "完成",
  failed: "失败",
  interrupted: "已中断",
};

const content_type_names = {
  video: "视频",
  short_video: "短视频",
  image: "图片",
  image_set: "图集",
  album: "图集",
  article: "文章",
  answer: "回答",
  question: "问题",
  post: "帖子",
  blog: "文章",
  novel: "小说",
  audio: "音频",
  podcast: "播客",
  music: "音乐",
  document: "文档",
  course: "课程",
  comic: "漫画",
  live: "直播",
};

const content_relation_names = {
  answer_of: "回答所属问题",
  contains: "包含",
  part_of: "属于",
  episode_of: "单集属于系列",
  reply_to: "回复",
  quote_of: "引用",
  repost_of: "转发",
  translation_of: "翻译自",
  derived_from: "派生自",
  related: "相关内容",
};

const download_resource_suffixes = {
  image: ".jpg",
  video: ".mp4",
  audio: ".mp3",
  html: ".html",
  text: ".txt",
  json: ".json",
  "image/jpeg": ".jpg",
  "image/png": ".png",
  "image/gif": ".gif",
  "image/webp": ".webp",
  "image/avif": ".avif",
  "image/svg+xml": ".svg",
  "image/bmp": ".bmp",
  "image/tiff": ".tiff",
  "video/mp4": ".mp4",
  "video/webm": ".webm",
  "video/quicktime": ".mov",
  "video/x-msvideo": ".avi",
  "video/x-matroska": ".mkv",
  "video/mp2t": ".ts",
  "video/x-flv": ".flv",
  "audio/mpeg": ".mp3",
  "audio/mp4": ".m4a",
  "audio/aac": ".aac",
  "audio/ogg": ".ogg",
  "audio/wav": ".wav",
  "audio/flac": ".flac",
  "text/html": ".html",
  "text/plain": ".txt",
  "text/css": ".css",
  "text/csv": ".csv",
  "text/markdown": ".md",
  "application/json": ".json",
  "application/xml": ".xml",
  "application/pdf": ".pdf",
  "application/zip": ".zip",
};

const content_detail_field_names = {
  type: "内容格式",
  duration: "时长",
  width: "宽度",
  height: "高度",
  fps: "帧率",
  bitrate: "码率",
  size: "大小",
  codec: "编码",
  format: "格式",
  audio_track_count: "音轨数",
  word_count: "字数",
  reading_time: "阅读时长",
  chapter_number: "章节序号",
  volume_number: "卷序号",
  series_name: "系列",
  is_finished: "完结状态",
  publish_platform: "发布平台",
  is_original: "原创",
  image_count: "图片数",
  cover_width: "封面宽度",
  cover_height: "封面高度",
};

function ScraperPageViewModel(props) {
  const { client: home_http_client, downloader } = props;

  const url_ = ref("");
  const loading_ = ref(false);
  const error_ = ref("");
  const result_ = ref(null);
  const json_expanded_ = ref(false);
  const raw_result_expanded_ = ref(true);
  const download_loading_ = ref(false);
  const download_error_ = ref("");
  const download_success_ = ref("");
  const download_overwrite_action_ = ref("overwrite");
  const download_overwrite_processing_ = ref(false);
  const download_overwrite_conflict_ = refobj({ name: "" });
  const download_preview_loading_ = ref(false);
  const download_preview_error_ = ref("");
  const selected_video_variant_ = ref(null);
  const fetch_progress_ = ref(null);
  const fetch_notice_ = ref("");
  const interrupt_loading_ = ref(false);
  const cache_loading_ = ref(false);
  const cache_error_ = ref("");
  const cache_content_loading_ = ref(false);
  const cache_content_error_ = ref("");
  const cache_content_entry_ = ref(null);
  const cache_content_data_ = ref(null);
  const chapter_display_limit_ = ref(100);
  const platform_statuses_ = ref([]);
  let request_sequence = 0;
  let active_job_id = "";
  let websocket_ = null;
  let websocket_connect_promise = null;
  let websocket_reconnect_timeout_id = 0;
  let poll_timeout_id = 0;
  let resolving_terminal_job_id = "";
  let platform_status_hide_timeout_id = 0;
  let cache_content_request_sequence = 0;
  let disposed = false;
  let progress_updated_at = 0;
  let download_preview_request_sequence = 0;
  let pending_download_object = null;
  const article_html_cache = new Map();
  const article_html_tasks = new Set();

  function schedule_article_html_task(callback) {
    let task = null;
    if (typeof window.requestIdleCallback === "function") {
      const task_id = window.requestIdleCallback(callback, {
        timeout: 500,
      });
      task = { id: task_id, idle: true };
    } else {
      const task_id = window.setTimeout(callback, 0);
      task = { id: task_id, idle: false };
    }
    article_html_tasks.add(task);
    return task;
  }

  function cancel_article_html_tasks() {
    article_html_tasks.forEach((task) => {
      if (task.idle && typeof window.cancelIdleCallback === "function") {
        window.cancelIdleCallback(task.id);
        return;
      }
      window.clearTimeout(task.id);
    });
    article_html_tasks.clear();
  }

  function create_article_html_content(value) {
    const html = String(value || "");
    if (article_html_cache.has(html)) {
      return article_html_cache.get(html);
    }
    const content_ = ref(html);
    article_html_cache.set(html, content_);
    const task = schedule_article_html_task(() => {
      article_html_tasks.delete(task);
      if (disposed) {
        return;
      }
      try {
        const normalized_html = normalize_article_theme_html(html);
        if (!content_.isStrictEqual(normalized_html)) {
          content_.as(normalized_html);
        }
      } catch (error) {
        console.warn("normalize article theme html failed", error);
      }
    });
    return content_;
  }

  function reset_article_html_processing() {
    cancel_article_html_tasks();
    article_html_cache.clear();
  }

  const fetch_request = new Timeless.kit.RequestCore(
    (body) => home_request.post("/api/scraper/fetch", body),
    {
      client: home_http_client,
    },
  );
  const job_request = new Timeless.kit.RequestCore(
    (params) => home_request.get("/api/scraper/job", params),
    {
      client: home_http_client,
    },
  );
  const interrupt_request = new Timeless.kit.RequestCore(
    (body) => home_request.post("/api/scraper/fetch/interrupt", body),
    {
      client: home_http_client,
    },
  );
  const cache_clear_request = new Timeless.kit.RequestCore(
    (body) => home_request.post("/api/scraper/cache/clear", body),
    {
      client: home_http_client,
    },
  );
  const cache_content_request = new Timeless.kit.RequestCore(
    (params) => home_request.get("/api/scraper/cache/content", params),
    {
      client: home_http_client,
    },
  );

  const submit_disabled_ = combine(
    {
      url: url_,
      loading: loading_,
      download_loading: download_loading_,
      download_preview_loading: download_preview_loading_,
      cache_loading: cache_loading_,
    },
    (state) =>
      state.loading ||
      state.download_loading ||
      state.download_preview_loading ||
      state.cache_loading ||
      !String(state.url || "").trim(),
  );
  const platform_status_items_ = computed(platform_statuses_, (statuses) =>
    (Array.isArray(statuses) ? statuses : [])
      .slice()
      .sort((left, right) => {
        const platform_compare = String(left.platform || "").localeCompare(
          String(right.platform || ""),
        );
        if (platform_compare !== 0) {
          return platform_compare;
        }
        return String(left.key || left.platform || "").localeCompare(
          String(right.key || right.platform || ""),
        );
      })
      .map((status) => {
        const status_key = String(status.key || status.platform || "").trim();
        const reason =
          !status.available && status.status !== "checking"
            ? status.reason
            : "";
        return {
          key: status_key,
          render_key: `${status_key}:${status.status}:${reason || ""}`,
          platform_name:
            String(status.name || "").trim() ||
            platform_name(status.platform),
          available: status.available,
          status: status.status,
          reason,
          has_reason: Boolean(reason),
          status_text:
            status.status === "checking"
              ? "检测中"
              : status.available
                ? "可用"
                : "不可用",
          status_class:
            status.status === "checking"
              ? "wx-home-platform-status-item is-checking"
              : status.available
                ? "wx-home-platform-status-item is-available"
                : "wx-home-platform-status-item is-unavailable",
        };
      }),
  );
  const platform_status_has_items_ = computed(
    platform_statuses_,
    (statuses) => Array.isArray(statuses) && statuses.length > 0,
  );
  const platform_status_summary_ = computed(
    platform_statuses_,
    (statuses) => {
      const items = Array.isArray(statuses) ? statuses : [];
      if (items.length === 0) {
        return "等待状态";
      }
      const available_count = items.filter(
        (status) => status.available,
      ).length;
      return `${available_count}/${items.length} 可用`;
    },
  );
  const platform_status_trigger_class_ = computed(
    platform_statuses_,
    (statuses) => {
      const items = Array.isArray(statuses) ? statuses : [];
      if (items.length === 0) {
        return "wx-home-platform-status-trigger is-pending";
      }
      if (
        items.some(
          (status) => !status.available && status.status !== "checking",
        )
      ) {
        return "wx-home-platform-status-trigger is-unavailable";
      }
      if (items.some((status) => status.status === "checking")) {
        return "wx-home-platform-status-trigger is-checking";
      }
      return "wx-home-platform-status-trigger is-available";
    },
  );
  const status_text_ = combine(
    {
      loading: loading_,
      error: error_,
      download_loading: download_loading_,
      download_error: download_error_,
      download_success: download_success_,
      download_preview_loading: download_preview_loading_,
      download_preview_error: download_preview_error_,
      fetch_progress: fetch_progress_,
      fetch_notice: fetch_notice_,
      interrupt_loading: interrupt_loading_,
      cache_loading: cache_loading_,
      cache_error: cache_error_,
    },
    (state) => {
      if (state.interrupt_loading) {
        return "正在中断获取...";
      }
      if (state.loading) {
        const progress = state.fetch_progress;
        return progress && progress.message
          ? progress.message
          : "正在获取...";
      }
      if (state.download_loading) {
        return "正在创建下载任务...";
      }
      if (state.download_preview_loading) {
        return "正在更新下载任务预览...";
      }
      if (state.cache_loading) {
        return "正在清理缓存...";
      }
      return (
        state.error ||
        state.download_error ||
        state.download_preview_error ||
        state.cache_error ||
        state.fetch_notice ||
        state.download_success ||
        ""
      );
    },
  );
  const busy_ = combine(
    {
      loading: loading_,
      download_loading: download_loading_,
      download_preview_loading: download_preview_loading_,
      cache_loading: cache_loading_,
    },
    (state) =>
      state.loading ||
      state.download_loading ||
      state.download_preview_loading ||
      state.cache_loading,
  );
  const has_error_ = combine(
    {
      error: error_,
      download_error: download_error_,
      download_preview_error: download_preview_error_,
      cache_error: cache_error_,
    },
    (state) =>
      Boolean(
        state.error ||
          state.download_error ||
          state.download_preview_error ||
          state.cache_error,
      ),
  );
  const has_result_ = computed(result_, (data) => Boolean(data));
  const display_result_present_ = computed(result_, has_display_result);
  const result_visible_ = combine(
    { present: display_result_present_, loading: loading_ },
    (state) => Boolean(state.present) && !state.loading,
  );
  const normalized_raw_result_ = computed(result_, normalize_raw_result);
  const raw_result_visible_ = combine(
    { raw: normalized_raw_result_, result_visible: result_visible_ },
    (state) => state.raw.present && !state.result_visible,
  );
  const normalized_cache_ = computed(result_, normalize_cache);
  const normalized_download_info_ = computed(
    result_,
    normalize_download_info,
  );
  const progress_visible_ = combine(
    { loading: loading_, progress: fetch_progress_ },
    (state) => Boolean(state.loading && state.progress),
  );
  const progress_percent_ = computed(fetch_progress_, (progress) => {
    const percent = number_or_default(progress && progress.percent, 0);
    return Math.min(100, Math.max(0, percent));
  });
  const progress_has_total_ = computed(
    fetch_progress_,
    (progress) => number_or_default(progress && progress.total, 0) > 0,
  );
  const progress_has_percent_ = computed(
    progress_percent_,
    (percent) => percent > 0,
  );
  const progress_stage_text_ = computed(fetch_progress_, (progress) => {
    const stage = String((progress && progress.stage) || "").trim();
    return scraper_fetch_stage_names[stage] || stage || "正在处理";
  });
  const progress_message_ = computed(fetch_progress_, (progress) =>
    String((progress && progress.message) || "").trim(),
  );
  const progress_percent_text_ = computed(
    progress_percent_,
    (percent) => `${Math.round(percent)}%`,
  );
  const progress_count_text_ = computed(fetch_progress_, (progress) => {
    const current = Math.max(
      0,
      number_or_default(progress && progress.current, 0),
    );
    const total = Math.max(
      0,
      number_or_default(progress && progress.total, 0),
    );
    const cache_hits = Math.max(
      0,
      number_or_default(progress && progress.cache_hits, 0),
    );
    if (total <= 0) {
      return "";
    }
    const unit = progress_unit_name(progress);
    return cache_hits > 0
      ? `${current}/${total} ${unit} · 已复用 ${cache_hits} 页缓存`
      : `${current}/${total} ${unit}`;
  });
  const progress_updated_text_ = computed(fetch_progress_, (progress) => {
    const updated_at = number_or_default(progress && progress.updated_at, 0);
    const text = format_clock_time(updated_at);
    return text ? `更新 ${text}` : "";
  });
  const progress_bar_class_ = combine(
    { has_percent: progress_has_percent_, loading: loading_ },
    (state) =>
      state.loading && !state.has_percent
        ? "wx-home-fetch-progress-bar is-indeterminate"
        : "wx-home-fetch-progress-bar",
  );
  const submit_button_text_ = computed(fetch_progress_, (progress) => {
    if (
      progress &&
      ["failed", "interrupted"].includes(String(progress.status || "")) &&
      number_or_default(progress.current, 0) > 0
    ) {
      return "继续获取";
    }
    return "确认";
  });
  const cache_action_disabled_ = combine(
    {
      cache: normalized_cache_,
      loading: loading_,
      download_loading: download_loading_,
      download_preview_loading: download_preview_loading_,
      cache_loading: cache_loading_,
    },
    (state) =>
      state.loading ||
      state.download_loading ||
      state.download_preview_loading ||
      state.cache_loading ||
      !state.cache.present,
  );
  const cache_button_text_ = computed(cache_loading_, (loading) =>
    loading ? "清除中" : "清除缓存",
  );
  const interrupt_disabled_ = computed(interrupt_loading_, (loading) =>
    Boolean(loading),
  );
  const normalized_content_ = computed(result_, normalize_content);
  const normalized_account_ = computed(result_, normalize_account);
  const normalized_content_details_ = combine(
    { result: result_, selected_video_variant: selected_video_variant_ },
    (state) =>
      normalize_content_details(
        state.result,
        create_article_html_content,
        state.selected_video_variant,
      ),
  );
  const content_ = {
    present: computed(normalized_content_, (content) => content.present),
    id: computed(normalized_content_, (content) => content.id),
    title: computed(normalized_content_, (content) => content.title),
    description: computed(
      normalized_content_,
      (content) => content.description,
    ),
    show_description: computed(
      normalized_content_,
      (content) => content.show_description,
    ),
    cover_url: computed(normalized_content_, (content) => content.cover_url),
    content_url: computed(
      normalized_content_,
      (content) => content.content_url,
    ),
    platform_name: computed(
      normalized_content_,
      (content) => content.platform_name,
    ),
    content_type_name: computed(
      normalized_content_,
      (content) => content.content_type_name,
    ),
    publish_time_text: computed(
      normalized_content_,
      (content) => content.publish_time_text,
    ),
  };
  const cache_ = {
    present: computed(normalized_cache_, (cache) => cache.present),
    entries: computed(normalized_cache_, (cache) => cache.entries),
    summary_text: computed(normalized_cache_, (cache) => cache.summary_text),
  };
  const cache_content_ = {
    loading: cache_content_loading_,
    error: cache_content_error_,
    title: computed(cache_content_entry_, (entry) =>
      String((entry && entry.name) || "缓存内容"),
    ),
    path: computed(cache_content_entry_, (entry) =>
      String((entry && entry.path) || ""),
    ),
    meta_text: combine(
      { entry: cache_content_entry_, data: cache_content_data_ },
      (state) => {
        const data = state.data || {};
        const entry = state.entry || {};
        const size_text =
          format_bytes(number_or_default(data.size, entry.size || 0)) ||
          "大小未知";
        const content_type = String(data.content_type || "").trim();
        return [size_text, content_type].filter(Boolean).join(" · ");
      },
    ),
    text: computed(cache_content_data_, (data) =>
      String((data && data.content) || ""),
    ),
  };
  const download_info_ = {
    present: computed(
      normalized_download_info_,
      (download_info) => download_info.present,
    ),
    resource_count_text: computed(
      normalized_download_info_,
      (download_info) => download_info.resource_count_text,
    ),
    resources: computed(
      normalized_download_info_,
      (download_info) => download_info.resources,
    ),
    loading: download_preview_loading_,
    error: download_preview_error_,
    badge_text: combine(
      {
        loading: download_preview_loading_,
        error: download_preview_error_,
        success: download_success_,
      },
      (state) =>
        state.loading
          ? "更新中"
          : state.error
            ? "更新失败"
            : state.success
              ? "已创建"
              : "",
    ),
    badge_class: combine(
      {
        loading: download_preview_loading_,
        error: download_preview_error_,
      },
      (state) =>
        state.error
          ? "wx-home-download-preview-badge is-error"
          : state.loading
            ? "wx-home-download-preview-badge is-loading"
            : "wx-home-download-preview-badge",
    ),
    task: {
      id_text: computed(
        normalized_download_info_,
        (download_info) => download_info.task.id_text,
      ),
      name: computed(
        normalized_download_info_,
        (download_info) => download_info.task.name,
      ),
      meta_text: computed(
        normalized_download_info_,
        (download_info) => download_info.task.meta_text,
      ),
      status_text: computed(
        normalized_download_info_,
        (download_info) => download_info.task.status_text,
      ),
    },
  };
  const account_ = {
    present: computed(normalized_account_, (account) => account.present),
    nickname: computed(normalized_account_, (account) => account.nickname),
    identity: computed(normalized_account_, (account) => account.identity),
    signature: computed(normalized_account_, (account) => account.signature),
    avatar_url: computed(
      normalized_account_,
      (account) => account.avatar_url,
    ),
    profile_url: computed(
      normalized_account_,
      (account) => account.profile_url,
    ),
    platform_name: computed(
      normalized_account_,
      (account) => account.platform_name,
    ),
    avatar_fallback: computed(
      normalized_account_,
      (account) => account.avatar_fallback,
    ),
  };
  const visible_novel_chapters_ = combine(
    {
      details: normalized_content_details_,
      limit: chapter_display_limit_,
    },
    (state) => state.details.novel.chapters.slice(0, state.limit),
  );
  const content_details_ = {
    present: computed(
      normalized_content_details_,
      (details) => details.present,
    ),
    count_text: computed(
      normalized_content_details_,
      (details) => details.count_text,
    ),
    items: computed(normalized_content_details_, (details) => details.items),
    novel: {
      present: computed(
        normalized_content_details_,
        (details) => details.novel.present,
      ),
      title: computed(
        normalized_content_details_,
        (details) => details.novel.title,
      ),
      subtitle: computed(
        normalized_content_details_,
        (details) => details.novel.subtitle,
      ),
      progress_text: computed(
        normalized_content_details_,
        (details) => details.novel.progress_text,
      ),
      metrics: computed(
        normalized_content_details_,
        (details) => details.novel.metrics,
      ),
      volumes: computed(
        normalized_content_details_,
        (details) => details.novel.volumes,
      ),
      chapters: visible_novel_chapters_,
      has_volumes: computed(
        normalized_content_details_,
        (details) => details.novel.has_volumes,
      ),
      has_chapters: computed(
        normalized_content_details_,
        (details) => details.novel.has_chapters,
      ),
      empty_chapter_text: computed(
        normalized_content_details_,
        (details) => details.novel.empty_chapter_text,
      ),
      has_more_chapters: combine(
        {
          details: normalized_content_details_,
          limit: chapter_display_limit_,
        },
        (state) => state.details.novel.chapters.length > state.limit,
      ),
      more_chapters_text: combine(
        {
          details: normalized_content_details_,
          limit: chapter_display_limit_,
        },
        (state) => {
          const remaining = Math.max(
            0,
            state.details.novel.chapters.length - state.limit,
          );
          return `再显示 ${Math.min(100, remaining)} 章`;
        },
      ),
    },
  };
  const content_relations_ = {
    present: computed(
      normalized_content_details_,
      (details) => details.relations.present,
    ),
    count_text: computed(
      normalized_content_details_,
      (details) => details.relations.count_text,
    ),
    content_present: computed(
      normalized_content_details_,
      (details) => details.relations.content_present,
    ),
    items: computed(
      normalized_content_details_,
      (details) => details.relations.items,
    ),
    influencers: {
      present: computed(
        normalized_content_details_,
        (details) => details.relations.influencers.present,
      ),
      count_text: computed(
        normalized_content_details_,
        (details) => details.relations.influencers.count_text,
      ),
      items: computed(
        normalized_content_details_,
        (details) => details.relations.influencers.items,
      ),
    },
  };
  const result_text_ = computed(result_, (data) =>
    data ? JSON.stringify(data, null, 2) : "",
  );
  const json_toggle_text_ = computed(json_expanded_, (expanded) =>
    expanded ? "收起原始 JSON" : "查看原始 JSON",
  );
  const raw_result_ = {
    visible: raw_result_visible_,
    expanded: raw_result_expanded_,
    toggle_text: computed(raw_result_expanded_, (expanded) =>
      expanded ? "收起抓取内容" : "查看抓取内容",
    ),
    meta_text: computed(
      normalized_raw_result_,
      (raw_result) => raw_result.meta_text,
    ),
    text: computed(normalized_raw_result_, (raw_result) => raw_result.text),
  };
  const download_disabled_ = combine(
    {
      result: result_,
      fetch_loading: loading_,
      loading: download_loading_,
      preview_loading: download_preview_loading_,
      preview_error: download_preview_error_,
      success: download_success_,
    },
    (state) =>
      !state.result ||
      state.fetch_loading ||
      state.loading ||
      state.preview_loading ||
      Boolean(state.preview_error) ||
      Boolean(state.success),
  );
  const video_variant_selection_disabled_ = combine(
    {
      fetch_loading: loading_,
      preview_loading: download_preview_loading_,
      download_loading: download_loading_,
      download_success: download_success_,
    },
    (state) =>
      state.fetch_loading ||
      state.preview_loading ||
      state.download_loading ||
      Boolean(state.download_success),
  );
  const download_button_text_ = combine(
    { loading: download_loading_, success: download_success_ },
    (state) => {
      if (state.loading) {
        return "创建中";
      }
      return state.success ? "已创建" : "下载";
    },
  );

  const ui = {
    input_url$: new Timeless.vm.InputCore({
      defaultValue: url_.value,
      placeholder: "输入需要获取的内容 URL",
      type: "text",
      allowClear: false,
      autoFocus: true,
      onChange(value) {
        set_url(value);
      },
    }),
    btn_interrupt_fetch$: new Timeless.vm.ButtonCore({
      disabled: interrupt_disabled_.value,
      loading: interrupt_loading_.value,
      variant: "destructive",
      onClick() {
        return interrupt_fetch();
      },
    }),
    btn_submit$: new Timeless.vm.ButtonCore({
      disabled: submit_disabled_.value,
      variant: "primary",
      onClick() {
        return submit();
      },
    }),
    btn_video_play$: new Timeless.vm.ButtonInListCore(),
    btn_novel_chapter$: new Timeless.vm.ButtonInListCore({
      onClick(chapter) {
        open_external_url(chapter && chapter.url);
      },
    }),
    btn_show_more_chapters$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        chapter_display_limit_.as(chapter_display_limit_.value + 100);
      },
    }),
    btn_video_variant$: new Timeless.vm.ButtonInListCore({
      onClick(selection) {
        const value = selection || {};
        return select_video_variant(value.detail_key, value.variant);
      },
    }),
    btn_detail_subject_open$: new Timeless.vm.ButtonInListCore({
      onClick(subject) {
        open_external_url(subject && subject.url);
      },
    }),
    btn_toggle_json$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        json_expanded_.as(!json_expanded_.value);
      },
    }),
    btn_toggle_raw_result$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick() {
        raw_result_expanded_.as(!raw_result_expanded_.value);
      },
    }),
    btn_create_download_task$: new Timeless.vm.ButtonCore({
      disabled: download_disabled_.value,
      loading: download_loading_.value,
      variant: "primary",
      onClick() {
        return create_download_task();
      },
    }),
    btn_force_refresh$: new Timeless.vm.ButtonCore({
      disabled: cache_action_disabled_.value,
      variant: "outline",
      onClick() {
        return submit({ force_refresh: true });
      },
    }),
    btn_clear_fetch_cache$: new Timeless.vm.ButtonCore({
      disabled: cache_action_disabled_.value,
      loading: cache_loading_.value,
      variant: "destructive",
      onClick() {
        return clear_fetch_cache();
      },
    }),
    btn_cache_entry$: new Timeless.vm.ButtonInListCore({
      onClick(entry) {
        return open_cache_content(entry);
      },
    }),
    btn_close_cache_content$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "icon-sm",
      onClick() {
        close_cache_content();
      },
    }),
    btn_platform_status$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "sm",
    }),
    platform_status_popover$: new Timeless.vm.PopoverCore({
      offsetY: 8,
      destroyOnClose: false,
    }),
    cache_content_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      footer: false,
    }),
    task_overwrite_confirm_dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      onOk() {
        return confirm_task_overwrite();
      },
      onCancel() {
        clear_pending_download_conflict(false);
      },
    }),
  };
  const ui_source_unsubscribes_ = [];

  function bind_ui_source(source, on_change) {
    on_change(source.value);
    const unlisten = source.subscribe({ onChange: on_change });
    if (typeof unlisten === "function") {
      ui_source_unsubscribes_.push(unlisten);
    }
  }

  function bind_ui_button_state(button, sources) {
    if (sources.disabled) {
      bind_ui_source(sources.disabled, (disabled) => {
        if (button.state.disabled === Boolean(disabled)) {
          return;
        }
        if (disabled) {
          button.disable();
        } else {
          button.enable();
        }
      });
    }
    if (sources.loading) {
      bind_ui_source(sources.loading, (loading) => {
        button.setLoading(Boolean(loading));
      });
    }
  }

  function configure_ui_button_list(button_list, options) {
    const settings = options || {};
    const bind = button_list.bind.bind(button_list);
    const buttons = new Set();
    const apply_button_state = (button) => {
      buttons.add(button);
      if (settings.variant && button.state.variant !== settings.variant) {
        button.setVariant(settings.variant);
      }
      if (settings.size && button.state.size !== settings.size) {
        button.setSize(settings.size);
      }
      if (settings.disabled) {
        const disabled = Boolean(settings.disabled.value);
        if (button.state.disabled !== disabled) {
          if (disabled) {
            button.disable();
          } else {
            button.enable();
          }
        }
      }
      if (settings.loading) {
        button.setLoading(Boolean(settings.loading.value));
      }
      return button;
    };
    button_list.bind = (value) => apply_button_state(bind(value));
    if (settings.disabled) {
      bind_ui_source(settings.disabled, (disabled) => {
        buttons.forEach((button) => {
          if (button.state.disabled === Boolean(disabled)) {
            return;
          }
          if (disabled) {
            button.disable();
          } else {
            button.enable();
          }
        });
      });
    }
    if (settings.loading) {
      bind_ui_source(settings.loading, (loading) => {
        button_list.setLoading(Boolean(loading));
      });
    }
  }

  bind_ui_source(url_, (value) => {
    if (ui.input_url$.value !== value) {
      ui.input_url$.setValue(value, { silence: true });
    }
  });
  bind_ui_button_state(ui.btn_interrupt_fetch$, {
    disabled: interrupt_disabled_,
    loading: interrupt_loading_,
  });
  bind_ui_button_state(ui.btn_submit$, { disabled: submit_disabled_ });
  configure_ui_button_list(ui.btn_video_play$, {
    variant: "ghost",
    size: "icon",
  });
  configure_ui_button_list(ui.btn_novel_chapter$, { variant: "ghost" });
  configure_ui_button_list(ui.btn_video_variant$, {
    variant: "ghost",
    disabled: video_variant_selection_disabled_,
  });
  configure_ui_button_list(ui.btn_detail_subject_open$, {
    variant: "outline",
    size: "sm",
  });
  bind_ui_button_state(ui.btn_create_download_task$, {
    disabled: download_disabled_,
    loading: download_loading_,
  });
  bind_ui_button_state(ui.btn_force_refresh$, {
    disabled: cache_action_disabled_,
  });
  bind_ui_button_state(ui.btn_clear_fetch_cache$, {
    disabled: cache_action_disabled_,
    loading: cache_loading_,
  });
  configure_ui_button_list(ui.btn_cache_entry$, { variant: "ghost" });

  function scraper_websocket_url() {
    const websocket_url = new URL(home_api_origin, window.location.href);
    websocket_url.protocol =
      websocket_url.protocol === "https:" ? "wss:" : "ws:";
    websocket_url.pathname = "/ws/scraper";
    websocket_url.search = "";
    return websocket_url.toString();
  }

  function apply_fetch_progress(progress) {
    if (!progress || typeof progress !== "object") {
      return;
    }
    const request_id = String(progress.request_id || "").trim();
    if (!request_id || request_id !== active_job_id) {
      return;
    }
    progress_updated_at = Date.now();
    const stage = String(progress.stage || "").trim();
    const status = String(progress.status || "").trim();
    const message = String(progress.message || "").trim();
    const normalized_progress = {
      ...progress,
      request_id,
      stage,
      status,
      current: Math.max(0, number_or_default(progress.current, 0)),
      total: Math.max(0, number_or_default(progress.total, 0)),
      percent: Math.min(
        100,
        Math.max(0, number_or_default(progress.percent, 0)),
      ),
      message:
        message ||
        scraper_fetch_stage_names[stage] ||
        (status === "pending" ? "等待执行..." : "正在获取..."),
      updated_at:
        number_or_default(progress.updated_at, 0) || progress_updated_at,
    };
    fetch_progress_.as(normalized_progress);
    if (normalized_progress.status === "interrupted") {
      error_.as("");
      fetch_notice_.as("获取已中断；再次继续获取会复用已缓存章节");
    }
  }

  function apply_platform_status(status) {
    if (!status || typeof status !== "object") {
      return;
    }
    const platform_id = String(status.platform || "").trim();
    if (!platform_id) {
      return;
    }
    const status_key = String(status.key || platform_id).trim();
    const next_statuses = Array.isArray(platform_statuses_.value)
      ? [...platform_statuses_.value]
      : [];
    const status_index = next_statuses.findIndex(
      (item) => String(item.key || item.platform || "").trim() === status_key,
    );
    const raw_status = String(status.status || "")
      .trim()
      .toLowerCase();
    const normalized_status =
      raw_status === "checking" ||
      raw_status === "available" ||
      raw_status === "unavailable"
        ? raw_status
        : status.available === true
          ? "available"
          : "unavailable";
    const next_status = {
      key: status_key,
      platform: platform_id,
      name: String(status.name || "").trim(),
      available: normalized_status === "available",
      status: normalized_status,
      reason: String(status.reason || status.message || status.error || "")
        .trim()
        .replace(/\s+/g, " "),
    };
    if (status_index >= 0) {
      next_statuses[status_index] = next_status;
    } else {
      next_statuses.push(next_status);
    }
    platform_statuses_.as(next_statuses);
  }

  function clear_job_poll() {
    if (poll_timeout_id) {
      window.clearTimeout(poll_timeout_id);
      poll_timeout_id = 0;
    }
  }

  function clear_websocket_reconnect() {
    if (websocket_reconnect_timeout_id) {
      window.clearTimeout(websocket_reconnect_timeout_id);
      websocket_reconnect_timeout_id = 0;
    }
  }

  function close_scraper_websocket() {
    clear_websocket_reconnect();
    const websocket = websocket_;
    websocket_ = null;
    websocket_connect_promise = null;
    if (websocket) {
      try {
        websocket.close();
      } catch {
        // Closing an already closed websocket is harmless.
      }
    }
  }

  function clear_platform_status_popover_hide() {
    if (platform_status_hide_timeout_id) {
      window.clearTimeout(platform_status_hide_timeout_id);
      platform_status_hide_timeout_id = 0;
    }
  }

  function show_platform_status_popover() {
    clear_platform_status_popover_hide();
    if (!ui.platform_status_popover$.visible) {
      ui.platform_status_popover$.show();
    }
  }

  function schedule_platform_status_popover_hide() {
    clear_platform_status_popover_hide();
    platform_status_hide_timeout_id = window.setTimeout(() => {
      platform_status_hide_timeout_id = 0;
      ui.platform_status_popover$.hide();
    }, platform_status_popover_hide_delay);
  }

  function finish_job_tracking() {
    clear_job_poll();
  }

  async function resolve_terminal_job(job, sequence) {
    const job_id = String((job && job.id) || "").trim();
    if (
      !job_id ||
      job_id !== active_job_id ||
      sequence !== request_sequence ||
      resolving_terminal_job_id === job_id
    ) {
      return;
    }
    resolving_terminal_job_id = job_id;

    let final_job = job;
    const current_raw = raw_result_payload(result_.value, true);
    const job_raw = raw_result_payload(job);
    if (
      (job.status === "completed" && !job.output) ||
      (job.status === "failed" && !current_raw.present && !job_raw.present)
    ) {
      const result = await job_request.run({ id: job_id });
      if (sequence !== request_sequence || job_id !== active_job_id) {
        return;
      }
      if (result.error) {
        resolving_terminal_job_id = "";
        schedule_job_poll(sequence, 500);
        return;
      }
      final_job = result.data || job;
    }
    apply_fetch_job_payload(final_job, null);

    if (final_job.status === "completed") {
      if (!final_job.output) {
        error_.as("fetch job 已完成，但缺少抓取结果");
      } else {
        error_.as("");
        result_.as(final_job.output);
      }
    } else if (final_job.status === "interrupted") {
      error_.as("");
      fetch_notice_.as("获取已中断；再次继续获取会复用已缓存章节");
    } else {
      error_.as(final_job.error || "获取失败");
    }
    interrupt_loading_.as(false);
    finish_job_tracking();
    loading_.as(false);
  }

  function upsert_content_detail(details, content_detail) {
    if (!content_detail || typeof content_detail !== "object") {
      return details;
    }
    const key = String(
      content_detail.key || content_detail.type || "",
    ).trim();
    const next_details = Array.isArray(details) ? [...details] : [];
    const detail_index = next_details.findIndex(
      (detail) =>
        String((detail && (detail.key || detail.type)) || "").trim() === key,
    );
    if (detail_index >= 0) {
      next_details[detail_index] = content_detail;
    } else {
      next_details.push(content_detail);
    }
    return next_details;
  }

  function upsert_cache_entry(entries, cache_entry) {
    if (!cache_entry || typeof cache_entry !== "object") {
      return entries;
    }
    const path = String(cache_entry.path || "").trim();
    if (!path) {
      return entries;
    }
    const key = String(cache_entry.key || path).trim();
    const next_entries = Array.isArray(entries) ? [...entries] : [];
    const entry_index = next_entries.findIndex((entry) => {
      const entry_key = String((entry && entry.key) || "").trim();
      const entry_path = String((entry && entry.path) || "").trim();
      return (entry_key && entry_key === key) || entry_path === path;
    });
    const next_entry = { ...cache_entry, key, path };
    if (entry_index >= 0) {
      next_entries[entry_index] = next_entry;
    } else {
      next_entries.push(next_entry);
    }
    return next_entries;
  }

  function progress_from_scraper_event(job, event) {
    if (!event || typeof event !== "object") {
      return null;
    }
    if (event.progress) {
      return event.progress;
    }
    const stage = String(event.stage || "").trim();
    const current = Math.max(0, number_or_default(event.current, 0));
    const total = Math.max(0, number_or_default(event.total, 0));
    if (!stage || total <= 0) {
      return null;
    }
    const messages = {
      content_detail: `正在整理内容详情 ${current}/${total}`,
      cache_entry: `正在整理缓存文件 ${current}/${total}`,
    };
    if (!Object.prototype.hasOwnProperty.call(messages, stage)) {
      return null;
    }
    return {
      request_id: job.id,
      platform: job.platform,
      url: job.url,
      stage,
      status: job.status,
      current,
      total,
      percent: total > 0 ? (current * 100) / total : 0,
      message: messages[stage],
    };
  }

  function apply_fetch_job_payload(job, event) {
    const event_raw = raw_result_payload(event);
    const job_raw = raw_result_payload(job);
    const content = (event && event.content) || job.content;
    const account = (event && event.account) || job.account;
    const job_details = Array.isArray(job.content_details)
      ? job.content_details
      : [];
    const event_detail = event && event.content_detail;
    const job_cache_entries = Array.isArray(job.cache_entries)
      ? job.cache_entries
      : [];
    const event_cache_entry = event && event.cache_entry;
    if (
      !event_raw.present &&
      !job_raw.present &&
      !content &&
      !account &&
      job_details.length === 0 &&
      !event_detail &&
      job_cache_entries.length === 0 &&
      !event_cache_entry
    ) {
      return;
    }

    const current_result = result_.value || {
      job_id: job.id,
      platform: job.platform,
      url: job.url,
    };
    let content_details = Array.isArray(current_result.content_details)
      ? current_result.content_details
      : [];
    for (const content_detail of job_details) {
      content_details = upsert_content_detail(
        content_details,
        content_detail,
      );
    }
    content_details = upsert_content_detail(content_details, event_detail);
    let cache_entries = Array.isArray(current_result.cache_entries)
      ? current_result.cache_entries
      : [];
    for (const cache_entry of job_cache_entries) {
      cache_entries = upsert_cache_entry(cache_entries, cache_entry);
    }
    cache_entries = upsert_cache_entry(cache_entries, event_cache_entry);
    result_.as({
      ...current_result,
      job_id: job.id,
      platform: job.platform,
      url: job.url,
      ...(event_raw.present
        ? { raw_result: event_raw.value }
        : job_raw.present
          ? { raw_result: job_raw.value }
          : {}),
      content: content || current_result.content,
      account: account || current_result.account,
      content_details,
      cache_entries,
    });
  }

  function apply_scraper_job(job, sequence, event = null) {
    if (!job || typeof job !== "object") {
      return;
    }
    const job_id = String(job.id || "").trim();
    if (
      !job_id ||
      job_id !== active_job_id ||
      sequence !== request_sequence
    ) {
      return;
    }
    apply_fetch_job_payload(job, event);
    const progress = progress_from_scraper_event(job, event) || job.progress;
    if (progress) {
      apply_fetch_progress(progress);
    } else {
      const current_progress = fetch_progress_.value || {};
      fetch_progress_.as({
        ...current_progress,
        request_id: job_id,
        status: job.status,
        message:
          job.status === "pending"
            ? "抓取任务已创建，等待执行..."
            : "正在获取...",
      });
    }
    if (["completed", "failed", "interrupted"].includes(job.status)) {
      void resolve_terminal_job(job, sequence);
    }
  }

  async function refresh_scraper_job(sequence) {
    const job_id = active_job_id;
    if (!job_id || sequence !== request_sequence || disposed) {
      return null;
    }
    const result = await job_request.run({ id: job_id });
    if (sequence !== request_sequence || job_id !== active_job_id) {
      return result;
    }
    if (result.error) {
      return result;
    }
    apply_scraper_job(result.data || {}, sequence);
    return result;
  }

  async function restore_scraper_job() {
    if (disposed || loading_.value || active_job_id) {
      return null;
    }
    const job_id = load_active_job_id();
    if (!job_id) {
      return null;
    }

    const sequence = ++request_sequence;
    active_job_id = job_id;
    resolving_terminal_job_id = "";
    loading_.as(true);
    error_.as("");
    fetch_notice_.as("");
    fetch_progress_.as({
      request_id: job_id,
      stage: "restore",
      status: "pending",
      current: 0,
      total: 0,
      percent: 0,
      message: "正在恢复抓取任务...",
    });

    const result = await refresh_scraper_job(sequence);
    if (
      disposed ||
      sequence !== request_sequence ||
      job_id !== active_job_id
    ) {
      return result;
    }
    if (result && result.error) {
      loading_.as(false);
      active_job_id = "";
      save_active_job_id("");
      fetch_progress_.as(null);
      return result;
    }

    const job = (result && result.data) || {};
    const raw_url = String(job.url || "").trim();
    if (raw_url) {
      url_.as(raw_url);
    }
    return result;
  }

  function schedule_job_poll(sequence, delay = 1000) {
    clear_job_poll();
    if (
      disposed ||
      !loading_.value ||
      !active_job_id ||
      sequence !== request_sequence
    ) {
      return;
    }
    poll_timeout_id = window.setTimeout(async () => {
      poll_timeout_id = 0;
      await refresh_scraper_job(sequence);
      if (loading_.value && sequence === request_sequence) {
        schedule_job_poll(sequence);
      }
    }, delay);
  }

  function schedule_scraper_websocket_reconnect(delay = 1000) {
    if (disposed || websocket_reconnect_timeout_id) {
      return;
    }
    websocket_reconnect_timeout_id = window.setTimeout(() => {
      websocket_reconnect_timeout_id = 0;
      void connect_scraper_websocket().catch(() => false);
    }, delay);
  }

  function connect_scraper_websocket() {
    if (disposed) {
      return Promise.resolve(false);
    }
    if (websocket_ && websocket_.readyState === WebSocket.OPEN) {
      return Promise.resolve(true);
    }
    if (websocket_connect_promise) {
      return websocket_connect_promise;
    }
    clear_websocket_reconnect();

    const connection_promise = new Promise((resolve, reject) => {
      let websocket;
      try {
        websocket = new WebSocket(scraper_websocket_url());
      } catch (error) {
        schedule_scraper_websocket_reconnect();
        reject(error);
        return;
      }
      websocket_ = websocket;
      let settled = false;
      const timeout_id = window.setTimeout(() => {
        if (settled) {
          return;
        }
        settled = true;
        reject(new Error("scraper progress websocket connection timeout"));
        try {
          websocket.close();
        } catch {
          // The timed-out websocket may already be closed.
        }
      }, 1500);

      websocket.onopen = () => {
        if (disposed || websocket_ !== websocket) {
          websocket.close();
          return;
        }
        if (loading_.value && active_job_id) {
          schedule_job_poll(request_sequence, 2000);
          const current_progress = fetch_progress_.value || {};
          fetch_progress_.as({
            ...current_progress,
            message: current_progress.message || "已连接实时进度推送",
            updated_at: Date.now(),
          });
        }
        window.clearTimeout(timeout_id);
        if (!settled) {
          settled = true;
          resolve(true);
        }
      };
      websocket.onmessage = (event) => {
        const [parse_error, message] = DLUtils.parseJSON(event.data);
        if (parse_error || !message) {
          return;
        }
        if (message.type === "platform_status") {
          apply_platform_status(message.platform_status);
          return;
        }
        if (message.type !== "scraper_job") {
          return;
        }
        apply_scraper_job(
          message.job,
          request_sequence,
          message.event || null,
        );
      };
      websocket.onerror = () => {
        if (settled) {
          return;
        }
        window.clearTimeout(timeout_id);
        settled = true;
        reject(new Error("scraper progress websocket connection failed"));
        try {
          websocket.close();
        } catch {
          // The failed websocket may already be closed.
        }
      };
      websocket.onclose = () => {
        window.clearTimeout(timeout_id);
        if (websocket_ === websocket) {
          websocket_ = null;
        }
        if (!settled) {
          settled = true;
          reject(new Error("scraper progress websocket connection closed"));
        }
        if (loading_.value && active_job_id) {
          const current_progress = fetch_progress_.value || {};
          fetch_progress_.as({
            ...current_progress,
            message: "实时进度连接已断开，正在用轮询继续更新...",
            updated_at: Date.now(),
          });
          schedule_job_poll(request_sequence, 250);
        }
        if (!disposed) {
          schedule_scraper_websocket_reconnect();
        }
      };
    });
    websocket_connect_promise = connection_promise;
    connection_promise.then(
      () => {
        if (websocket_connect_promise === connection_promise) {
          websocket_connect_promise = null;
        }
      },
      () => {
        if (websocket_connect_promise === connection_promise) {
          websocket_connect_promise = null;
        }
      },
    );
    return connection_promise;
  }

  function dispose() {
    disposed = true;
    clear_pending_download_conflict();
    reset_article_html_processing();
    reset_cache_content(true);
    request_sequence += 1;
    download_preview_request_sequence += 1;
    download_preview_loading_.as(false);
    active_job_id = "";
    resolving_terminal_job_id = "";
    finish_job_tracking();
    close_scraper_websocket();
    clear_platform_status_popover_hide();
    ui.platform_status_popover$.hide();
    while (ui_source_unsubscribes_.length > 0) {
      ui_source_unsubscribes_.pop()();
    }
  }

  async function submit(options = {}) {
    if (loading_.value || download_loading_.value || cache_loading_.value) {
      return null;
    }
    const raw_url = String(url_.value || "").trim();
    if (!raw_url) {
      error_.as("请输入 URL");
      return null;
    }

    const sequence = ++request_sequence;
    const force_refresh = Boolean(options.force_refresh);
    clear_pending_download_conflict();
    finish_job_tracking();
    reset_article_html_processing();
    reset_cache_content(true);
    disposed = false;
    active_job_id = "";
    save_active_job_id("");
    resolving_terminal_job_id = "";
    loading_.as(true);
    error_.as("");
    result_.as(null);
    download_preview_request_sequence += 1;
    download_preview_loading_.as(false);
    download_preview_error_.as("");
    selected_video_variant_.as(null);
    chapter_display_limit_.as(100);
    json_expanded_.as(false);
    download_error_.as("");
    download_success_.as("");
    fetch_notice_.as("");
    cache_error_.as("");
    fetch_progress_.as({
      stage: "start",
      status: "pending",
      current: 0,
      total: 0,
      percent: 0,
      message: force_refresh
        ? "正在创建重新抓取任务..."
        : "正在创建抓取任务...",
    });
    const result = await fetch_request.run({
      url: raw_url,
      force_refresh,
    });
    if (sequence !== request_sequence) {
      return result;
    }
    if (result.error) {
      loading_.as(false);
      error_.as(result.error.message || String(result.error));
      return result;
    }
    const job = result.data || {};
    const job_id = String(job.id || "").trim();
    if (!job_id) {
      loading_.as(false);
      error_.as("创建 fetch job 失败：响应缺少 id");
      return result;
    }
    active_job_id = job_id;
    save_active_job_id(job_id);
    apply_scraper_job(job, sequence);
    if (loading_.value) {
      try {
        await connect_scraper_websocket();
        void refresh_scraper_job(sequence);
      } catch (error) {
        schedule_job_poll(sequence, 250);
      }
    }
    return result;
  }

  async function interrupt_fetch() {
    if (!loading_.value || interrupt_loading_.value) {
      return null;
    }
    const job_id = active_job_id;
    if (!job_id) {
      return null;
    }
    interrupt_loading_.as(true);
    const current_progress = fetch_progress_.value || {};
    fetch_progress_.as({
      ...current_progress,
      message: "正在中断获取...",
    });
    const result = await interrupt_request.run({ id: job_id });
    interrupt_loading_.as(false);
    if (result.error) {
      error_.as(result.error.message || String(result.error));
    } else if (!result.data || !result.data.interrupted) {
      fetch_notice_.as("当前获取已结束，无需中断");
    }
    return result;
  }

  function reset_cache_content(hide_dialog = false) {
    cache_content_request_sequence += 1;
    cache_content_loading_.as(false);
    cache_content_error_.as("");
    cache_content_entry_.as(null);
    cache_content_data_.as(null);
    // if (hide_dialog) {
    //   ui.cache_content_dialog$.hide();
    // }
  }

  async function open_cache_content(cache_entry) {
    const entry =
      cache_entry && typeof cache_entry === "object" ? cache_entry : {};
    const cache_key = String(entry.key || entry.path || "").trim();
    const fetch_result = result_.value || {};
    const job_id = String(fetch_result.job_id || active_job_id || "").trim();
    const sequence = ++cache_content_request_sequence;
    cache_content_entry_.as(entry);
    cache_content_data_.as(null);
    cache_content_error_.as("");
    ui.cache_content_dialog$.show();
    if (!job_id || !cache_key) {
      cache_content_loading_.as(false);
      cache_content_error_.as("无法定位该缓存条目");
      return null;
    }

    cache_content_loading_.as(true);
    const result = await cache_content_request.run({
      id: job_id,
      key: cache_key,
    });
    if (sequence !== cache_content_request_sequence) {
      return result;
    }
    cache_content_loading_.as(false);
    if (result.error) {
      cache_content_error_.as(result.error.message || String(result.error));
      return result;
    }
    cache_content_data_.as(result.data || {});
    return result;
  }

  function close_cache_content() {
    ui.cache_content_dialog$.hide();
  }

  async function clear_fetch_cache() {
    if (cache_action_disabled_.value) {
      return null;
    }
    const fetch_result = result_.value;
    const raw_url = String((fetch_result && fetch_result.url) || "").trim();
    if (!raw_url) {
      return null;
    }
    cache_loading_.as(true);
    cache_error_.as("");
    fetch_notice_.as("");
    const result = await cache_clear_request.run({ url: raw_url });
    cache_loading_.as(false);
    if (result.error) {
      cache_error_.as(result.error.message || String(result.error));
      return result;
    }
    fetch_progress_.as(null);
    const removed = Boolean(result.data && result.data.removed);
    result_.as({ ...fetch_result, cache_entries: [] });
    reset_cache_content(true);
    fetch_notice_.as(removed ? "抓取缓存已清理" : "该 URL 暂无抓取缓存");
    return result;
  }

  function download_task_conflict_name(object, error) {
    const data = (error && error.data) || {};
    const content = object && object.content;
    return (
      data.name ||
      (object && (object.filename || object.name || object.title)) ||
      (content && (content.title || content.name || content.id)) ||
      "相同下载内容"
    );
  }

  function clear_pending_download_conflict(hide_dialog = true) {
    pending_download_object = null;
    download_overwrite_conflict_.as({ name: "" });
    // if (hide_dialog) {
    //   ui.task_overwrite_confirm_dialog$.hide();
    // }
  }

  function handle_download_task_create_failure(error, object, options) {
    const create_options = options || {};
    if (!create_options.overwrite_retry && Number(error && error.code) === 409) {
      pending_download_object = object;
      download_overwrite_action_.as("overwrite");
      download_overwrite_conflict_.as({
        name: download_task_conflict_name(object, error),
      });
      download_error_.as("");
      ui.task_overwrite_confirm_dialog$.show();
      return;
    }
    download_error_.as((error && error.message) || String(error));
  }

  async function create_download_task_from_object(object, options) {
    const create_options = options || {};
    const task$ = downloader.create(object);
    let failure_handled = false;
    let unsubscribe_fail = null;

    if (task$ && typeof task$.onFail === "function") {
      unsubscribe_fail = task$.onFail((event) => {
        const error = event && event.error ? event.error : event;
        failure_handled = true;
        handle_download_task_create_failure(error, object, create_options);
      });
    }

    try {
      await task$.ready;
    } catch (error) {
      if (!failure_handled) {
        handle_download_task_create_failure(error, object, create_options);
      }
      return task$;
    } finally {
      if (typeof unsubscribe_fail === "function") {
        unsubscribe_fail();
      }
      download_loading_.as(false);
    }

    download_success_.as("下载任务创建成功");
    if (create_options.overwrite_retry) {
      clear_pending_download_conflict();
    }
    return task$;
  }

  async function create_download_task() {
    if (
      download_loading_.value ||
      download_preview_loading_.value ||
      download_preview_error_.value ||
      download_success_.value
    ) {
      return null;
    }

    const fetch_result = result_.value;
    const platform = String(
      (fetch_result && fetch_result.platform) || "",
    ).trim();
    const content = fetch_result && fetch_result.result;
    if (!platform || content === undefined || content === null) {
      download_error_.as("解析结果缺少 platform 或 result");
      return null;
    }

    download_loading_.as(true);
    download_error_.as("");
    download_success_.as("");
    if (!downloader || typeof downloader.create !== "function") {
      download_loading_.as(false);
      download_error_.as("下载服务尚未初始化");
      return null;
    }
    const object = {
      platform,
      content,
      build_from_fetch: Boolean(fetch_result.download_info),
      config: selected_video_variant_config(platform),
    };
    return create_download_task_from_object(object);
  }

  function set_task_overwrite_action(action) {
    if (
      !download_overwrite_processing_.value &&
      ["overwrite", "duplicate"].includes(action)
    ) {
      download_overwrite_action_.as(action);
    }
  }

  async function confirm_task_overwrite() {
    if (download_overwrite_processing_.value || !pending_download_object) {
      return null;
    }
    const action = download_overwrite_action_.value;
    if (!["overwrite", "duplicate"].includes(action)) {
      return null;
    }

    const object = {
      ...pending_download_object,
      config: {
        ...(pending_download_object.config || {}),
        overwrite: action === "overwrite",
        duplicate: action === "duplicate",
      },
    };
    download_overwrite_processing_.as(true);
    download_loading_.as(true);
    download_error_.as("");
    download_success_.as("");
    try {
      return await create_download_task_from_object(object, {
        overwrite_retry: true,
      });
    } finally {
      download_overwrite_processing_.as(false);
    }
  }

  function selected_video_variant_config(platform) {
    const config = { platform };
    const selected = selected_video_variant_.value;
    if (!selected || typeof selected !== "object") {
      return config;
    }
    const variant_key = String(selected.variant_key || "").trim();
    const spec = String(selected.spec || "").trim();
    if (variant_key) {
      config.video_variant_key = variant_key;
    }
    if (spec) {
      config.video_variant_spec = spec;
      // Keep the established adapter option while all adapters migrate to the
      // shared ContentVideoVariant keys.
      config.spec = spec;
    }
    return config;
  }

  async function select_video_variant(detail_key, variant_value) {
    if (video_variant_selection_disabled_.value) {
      return null;
    }
    const variant =
      variant_value && typeof variant_value === "object"
        ? variant_value
        : {};
    const variant_key = String(variant.variant_key || "").trim();
    if (!variant_key) {
      download_preview_error_.as("该视频规格缺少 variant_key");
      return null;
    }

    const fetch_result = result_.value;
    const platform = String(
      (fetch_result && fetch_result.platform) || "",
    ).trim();
    const content = fetch_result && fetch_result.result;
    if (!platform || content === undefined || content === null) {
      download_preview_error_.as("解析结果缺少 platform 或 result");
      return null;
    }

    const selected = {
      detail_key: String(detail_key || "").trim(),
      variant_key,
      spec: String(variant.spec || "").trim(),
    };
    selected_video_variant_.as(selected);
    download_preview_loading_.as(true);
    download_preview_error_.as("");
    download_error_.as("");
    download_success_.as("");
    const sequence = ++download_preview_request_sequence;
    if (!downloader || typeof downloader.prepare !== "function") {
      download_preview_loading_.as(false);
      download_preview_error_.as("下载服务尚未初始化");
      return null;
    }
    let download_info;
    try {
      download_info = await downloader.prepare({
        platform,
        content,
        build_from_fetch: true,
        config: selected_video_variant_config(platform),
      });
    } catch (error) {
      if (sequence !== download_preview_request_sequence) return null;
      download_preview_loading_.as(false);
      download_preview_error_.as(
        error.message || String(error),
      );
      return null;
    }
    if (sequence !== download_preview_request_sequence) return download_info;
    download_preview_loading_.as(false);
    if (!download_info || typeof download_info !== "object") {
      download_preview_error_.as("预览响应缺少 download_info");
      return null;
    }

    result_.as({ ...fetch_result, download_info });
    return download_info;
  }

  function open_external_url(value) {
    const target_url = String(value || "").trim();
    if (!target_url) {
      return;
    }
    window.open(target_url, "_blank", "noopener,noreferrer");
  }

  function set_url(value) {
    url_.as(String(value || ""));
    if (error_.value) {
      error_.as("");
    }
    if (cache_error_.value) {
      cache_error_.as("");
    }
    if (fetch_notice_.value) {
      fetch_notice_.as("");
    }
  }

  const methods = {
    setURL: set_url,
    submit,
    forceRefresh() {
      return submit({ force_refresh: true });
    },
    interruptFetch: interrupt_fetch,
    clearFetchCache: clear_fetch_cache,
    openCacheContent: open_cache_content,
    closeCacheContent: close_cache_content,
    createDownloadTask: create_download_task,
    setTaskOverwriteAction: set_task_overwrite_action,
    confirmTaskOverwrite: confirm_task_overwrite,
    selectVideoVariant: select_video_variant,
    toggleJSON() {
      json_expanded_.as(!json_expanded_.value);
    },
    toggleRawResult() {
      raw_result_expanded_.as(!raw_result_expanded_.value);
    },
    openContent() {
      open_external_url(normalized_content_.value.content_url);
    },
    openAccount() {
      open_external_url(normalized_account_.value.profile_url);
    },
    openDetailURL(value) {
      open_external_url(value);
    },
    showMoreChapters() {
      chapter_display_limit_.as(chapter_display_limit_.value + 100);
    },
    showPlatformStatusPopover: show_platform_status_popover,
    schedulePlatformStatusPopoverHide: schedule_platform_status_popover_hide,
    async connectProgress() {
      disposed = false;
      await restore_scraper_job();
      if (disposed) {
        return false;
      }
      return connect_scraper_websocket().catch(() => false);
    },
    dispose,
  };

  const state = {
    url: url_,
    loading: loading_,
    error: error_,
    result: result_,
    content: content_,
    account: account_,
    content_details: content_details_,
    content_relations: content_relations_,
    download_info: download_info_,
    cache: cache_,
    cache_content: cache_content_,
    raw_result: raw_result_,
    json_expanded: json_expanded_,
    download_loading: download_loading_,
    download_error: download_error_,
    download_success: download_success_,
    download_overwrite_action: download_overwrite_action_,
    download_overwrite_processing: download_overwrite_processing_,
    download_overwrite_conflict: download_overwrite_conflict_,
    download_preview_loading: download_preview_loading_,
    download_preview_error: download_preview_error_,
    selected_video_variant: selected_video_variant_,
    video_variant_selection_disabled: video_variant_selection_disabled_,
    fetch_progress: fetch_progress_,
    fetch_notice: fetch_notice_,
    interrupt_loading: interrupt_loading_,
    cache_loading: cache_loading_,
    cache_error: cache_error_,
    progress_visible: progress_visible_,
    progress_percent: progress_percent_,
    progress_has_total: progress_has_total_,
    progress_has_percent: progress_has_percent_,
    progress_stage_text: progress_stage_text_,
    progress_message: progress_message_,
    progress_percent_text: progress_percent_text_,
    progress_count_text: progress_count_text_,
    progress_updated_text: progress_updated_text_,
    progress_bar_class: progress_bar_class_,
    submit_button_text: submit_button_text_,
    cache_action_disabled: cache_action_disabled_,
    cache_button_text: cache_button_text_,
    interrupt_disabled: interrupt_disabled_,
    submit_disabled: submit_disabled_,
    status_text: status_text_,
    busy: busy_,
    has_error: has_error_,
    has_result: has_result_,
    result_visible: result_visible_,
    result_text: result_text_,
    json_toggle_text: json_toggle_text_,
    download_disabled: download_disabled_,
    download_button_text: download_button_text_,
    platform_status: {
      has_items: platform_status_has_items_,
      items: platform_status_items_,
      summary: platform_status_summary_,
      trigger_class: platform_status_trigger_class_,
    },
  };

  return { state, ui, methods };
}

function first_non_empty(...values) {
  for (const value of values) {
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
  }
  return "";
}

function has_own_property(source, key) {
  return Boolean(
    source &&
    typeof source === "object" &&
    Object.prototype.hasOwnProperty.call(source, key),
  );
}

function number_or_default(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function load_active_job_id() {
  try {
    return String(
      window.localStorage.getItem(active_job_storage_key) || "",
    ).trim();
  } catch (error) {
    return "";
  }
}

function save_active_job_id(job_id) {
  const normalized_job_id = String(job_id || "").trim();
  try {
    if (normalized_job_id) {
      window.localStorage.setItem(active_job_storage_key, normalized_job_id);
      return;
    }
    window.localStorage.removeItem(active_job_storage_key);
  } catch {
    // Storage can be unavailable in restricted browser contexts.
  }
}

function platform_name(value) {
  const platform_id = String(value || "").trim();
  return platform_names[platform_id] || platform_id || "未知平台";
}

function content_type_name(value) {
  const content_type = String(value || "")
    .trim()
    .toLowerCase();
  return content_type_names[content_type] || content_type || "内容";
}

function normalize_epoch_ms(value) {
  const timestamp = Number(value);
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return 0;
  }
  return timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
}

function format_time(value) {
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

function format_clock_time(value) {
  const timestamp = normalize_epoch_ms(value);
  if (!timestamp) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(timestamp));
}

function format_count(value) {
  const count = Math.max(0, number_or_default(value, 0));
  return new Intl.NumberFormat("zh-CN", { notation: "compact" }).format(
    count,
  );
}

function format_bytes(value) {
  const size = Math.max(0, number_or_default(value, 0));
  if (!size) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  const unit_index = Math.min(
    units.length - 1,
    Math.floor(Math.log(size) / Math.log(1024)),
  );
  const amount = size / 1024 ** unit_index;
  const precision = amount >= 100 || unit_index === 0 ? 0 : 1;
  return `${amount.toFixed(precision)} ${units[unit_index]}`;
}

function format_duration(value) {
  const total_seconds = Math.max(0, Math.round(number_or_default(value, 0)));
  if (!total_seconds) {
    return "";
  }
  const hours = Math.floor(total_seconds / 3600);
  const minutes = Math.floor((total_seconds % 3600) / 60);
  const seconds = total_seconds % 60;
  if (hours > 0) {
    return `${hours}时${minutes}分${seconds}秒`;
  }
  return minutes > 0 ? `${minutes}分${seconds}秒` : `${seconds}秒`;
}

function progress_unit_name(progress) {
  const stage = String((progress && progress.stage) || "").trim();
  const platform = String((progress && progress.platform) || "").trim();
  if (stage === "content_detail") {
    return platform === "fanqienovel" ||
      platform === "ttk" ||
      platform === "69shuba"
      ? "章"
      : "项详情";
  }
  if (stage === "cache" || stage === "cache_entry") {
    return "个缓存";
  }
  return "步";
}

function normalize_cache(result) {
  const source_entries =
    result && Array.isArray(result.cache_entries) ? result.cache_entries : [];
  const entries = source_entries
    .map((entry, index) => {
      const source = entry && typeof entry === "object" ? entry : {};
      const path = String(source.path || "").trim();
      if (!path) {
        return null;
      }
      const path_parts = path.split(/[\\/]/).filter(Boolean);
      const fallback_name = path_parts[path_parts.length - 1] || "缓存文件";
      const size = Math.max(0, number_or_default(source.size, 0));
      return {
        key: String(source.key || `${path}:${index}`),
        name: String(source.name || fallback_name),
        path,
        url: String(source.url || "").trim(),
        size,
        size_text: format_bytes(size) || "大小未知",
      };
    })
    .filter(Boolean);
  const total_size = entries.reduce((sum, entry) => sum + entry.size, 0);
  const count = entries.length;
  return {
    present: count > 0,
    entries,
    count,
    total_size,
    summary_text: total_size
      ? `${count} 个文件 · ${format_bytes(total_size)}`
      : `${count} 个文件`,
  };
}

function download_info_value(source, ...keys) {
  if (!source || typeof source !== "object") {
    return undefined;
  }
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      return source[key];
    }
  }
  return undefined;
}

function download_info_object(source, ...keys) {
  const value = download_info_value(source, ...keys);
  return value && typeof value === "object" && !Array.isArray(value)
    ? value
    : {};
}

function download_info_array(source, ...keys) {
  const value = download_info_value(source, ...keys);
  return Array.isArray(value) ? value : [];
}

function download_resource_icon(kind) {
  const normalized_kind = String(kind || "").toLowerCase();
  if (normalized_kind.includes("image")) {
    return "file-image";
  }
  if (
    normalized_kind.includes("video") ||
    normalized_kind.includes("audio")
  ) {
    return "file-play";
  }
  if (
    normalized_kind.includes("html") ||
    normalized_kind.includes("text") ||
    normalized_kind.includes("json")
  ) {
    return "file-text";
  }
  return "file";
}

function suffix_map(kind) {
  const normalized_kind = String(kind || "")
    .split(";", 1)[0]
    .trim()
    .toLowerCase();
  return download_resource_suffixes[normalized_kind] || "";
}

function normalize_content_video_variant(variant, index) {
  const source = variant && typeof variant === "object" ? variant : {};
  const variant_key = String(
    first_non_empty(
      source.variant_key,
      source.VariantKey,
      source.spec,
      source.Spec,
      `variant-${index + 1}`,
    ),
  );
  const width = number_or_default(
    first_non_empty(source.width, source.Width),
    0,
  );
  const height = number_or_default(
    first_non_empty(source.height, source.Height),
    0,
  );
  const dimensions = width > 0 && height > 0 ? `${width} × ${height}` : "";
  const spec = String(first_non_empty(source.spec, source.Spec)).trim();
  const is_default =
    Number(first_non_empty(source.is_default, source.IsDefault)) > 0;
  const meta_text = [
    dimensions,
    first_non_empty(source.codec, source.Codec),
    first_non_empty(source.format, source.Format),
    first_non_empty(source.stream_type, source.StreamType),
    format_bytes(first_non_empty(source.size, source.Size)),
  ]
    .filter(Boolean)
    .join(" · ");
  return {
    key: `${variant_key}:${index}`,
    title: String(
      first_non_empty(
        source.quality,
        source.Quality,
        source.spec,
        source.Spec,
        variant_key,
      ),
    ),
    variant_key,
    spec,
    meta_text,
    is_default,
    default_text: is_default ? "默认" : "可选",
    url: String(first_non_empty(source.url, source.URL)).trim(),
  };
}

function normalize_content_text_track_source(source_value, index) {
  const source =
    source_value && typeof source_value === "object" ? source_value : {};
  const source_key = String(
    first_non_empty(
      source.source_key,
      source.SourceKey,
      source.format,
      source.Format,
      `source-${index + 1}`,
    ),
  );
  return {
    key: `${source_key}:${index}`,
    title: String(
      first_non_empty(source.format, source.Format, source_key),
    ).toUpperCase(),
    meta_text: [
      first_non_empty(source.encoding, source.Encoding),
      format_bytes(first_non_empty(source.size, source.Size)),
    ]
      .filter(Boolean)
      .join(" · "),
    url: String(first_non_empty(source.url, source.URL)).trim(),
  };
}

function normalize_content_text_track(track, index) {
  const source = track && typeof track === "object" ? track : {};
  const track_key = String(
    first_non_empty(source.track_key, source.TrackKey, `track-${index + 1}`),
  );
  const flags = [];
  if (Number(first_non_empty(source.is_default, source.IsDefault)) > 0) {
    flags.push("默认");
  }
  if (Number(first_non_empty(source.is_forced, source.IsForced)) > 0) {
    flags.push("强制");
  }
  if (
    Number(
      first_non_empty(source.is_auto_generated, source.IsAutoGenerated),
    ) > 0
  ) {
    flags.push("自动生成");
  }
  const sources = download_info_array(source, "sources", "Sources").map(
    normalize_content_text_track_source,
  );
  return {
    key: `${track_key}:${index}`,
    title: String(
      first_non_empty(
        source.label,
        source.Label,
        source.language_name,
        source.LanguageName,
        source.language_code,
        source.LanguageCode,
        track_key,
      ),
    ),
    track_key,
    meta_text: [
      first_non_empty(source.language_code, source.LanguageCode),
      first_non_empty(source.type, source.Type),
      ...flags,
    ]
      .filter(Boolean)
      .join(" · "),
    sources,
    has_sources: sources.length > 0,
  };
}

function normalize_download_content_asset(asset, index) {
  const source = asset && typeof asset === "object" ? asset : {};
  const asset_key = String(
    first_non_empty(source.asset_key, source.AssetKey, `asset-${index + 1}`),
  );
  const subject_text = [
    first_non_empty(source.subject_type, source.SubjectType),
    first_non_empty(source.subject_key, source.SubjectKey),
  ]
    .filter(Boolean)
    .join(" · ");
  return {
    key: `${asset_key}:${index}`,
    asset_key,
    kind: String(first_non_empty(source.kind, source.Kind, "asset")),
    role: String(first_non_empty(source.role, source.Role, "关联资产")),
    relation: String(
      first_non_empty(source.relation, source.Relation, "source"),
    ),
    subject_text,
  };
}

function normalize_download_resource(resource_info, index, content_id) {
  const source =
    resource_info && typeof resource_info === "object" ? resource_info : {};
  const resource = download_info_object(source, "Resource", "resource");
  const endpoints = download_info_array(source, "Endpoints", "endpoints");
  const content_assets = download_info_array(
    source,
    "ContentAssets",
    "content_assets",
  ).map(normalize_download_content_asset);
  const name = String(
    first_non_empty(
      resource.name,
      resource.Name,
      resource.unique_id,
      `资源 ${index + 1}`,
    ),
  );
  const kind = String(
    first_non_empty(resource.kind, resource.Kind, resource.type, "file"),
  );
  const resource_content_id = String(
    first_non_empty(
      resource.content_id,
      resource.ContentId,
      resource.ContentID,
      content_id,
    ),
  );
  return {
    key: String(
      first_non_empty(
        resource.unique_id,
        resource.UniqueID,
        `${name}:${index}`,
      ),
    ),
    index_text: String(index + 1).padStart(2, "0"),
    name,
    display_name: `${name}${suffix_map(kind)}`,
    kind,
    icon: download_resource_icon(kind),
    meta_text: [
      kind,
      format_bytes(first_non_empty(resource.size, resource.Size)),
      `${endpoints.length} 个下载端点`,
    ]
      .filter(Boolean)
      .join(" · "),
    content_id: resource_content_id,
    content_assets,
    has_content_assets: content_assets.length > 0,
  };
}

function normalize_download_info(result) {
  const download_info =
    result && result.download_info && typeof result.download_info === "object"
      ? result.download_info
      : null;
  if (!download_info) {
    return {
      present: false,
      resource_count_text: "0",
      resources: [],
      task: {
        id_text: "-",
        name: "下载任务",
        meta_text: "",
        status_text: "",
      },
    };
  }
  const task = download_info_object(download_info, "Task", "task");
  const content = download_info_object(download_info, "Content", "content");
  const content_id = String(
    first_non_empty(
      content.id,
      content.Id,
      task.content_id,
      task.ContentId,
      result && result.content && result.content.id,
    ),
  );
  const resources = download_info_array(
    download_info,
    "Resources",
    "resources",
  ).map((resource, index) =>
    normalize_download_resource(resource, index, content_id),
  );
  const task_statuses = {
    0: "",
    1: "准备中",
    2: "下载中",
    3: "已暂停",
    4: "合并中",
    5: "已完成",
    6: "失败",
    7: "已取消",
  };
  const task_status = number_or_default(
    first_non_empty(task.status, task.Status),
    0,
  );
  const task_id = number_or_default(first_non_empty(task.id, task.Id), 0);
  return {
    present: true,
    resource_count_text: String(resources.length),
    resources,
    task: {
      id_text: task_id > 0 ? `#${task_id}` : "-",
      name: String(first_non_empty(task.name, task.Name, "下载任务")),
      meta_text: [
        platform_name(first_non_empty(task.platform_id, task.PlatformId)),
        first_non_empty(task.unique_id, task.UniqueID),
      ]
        .filter(Boolean)
        .join(" · "),
      status_text: task_statuses[task_status] || `状态 ${task_status}`,
    },
  };
}

function parse_article_color(value, color_probe, color_cache) {
  const color = String(value || "").trim();
  if (!color || !color_probe) {
    return null;
  }
  if (color_cache.has(color)) {
    return color_cache.get(color);
  }
  color_probe.style.color = "";
  color_probe.style.color = color;
  if (!color_probe.style.color) {
    color_cache.set(color, null);
    return null;
  }
  const resolved_color = window.getComputedStyle(color_probe).color;
  if (!/^rgba?\(/i.test(String(resolved_color || ""))) {
    color_cache.set(color, null);
    return null;
  }
  const components = String(resolved_color || "").match(/[\d.]+/g) || [];
  if (components.length < 3) {
    color_cache.set(color, null);
    return null;
  }
  const red = Math.min(255, Math.max(0, Number(components[0])));
  const green = Math.min(255, Math.max(0, Number(components[1])));
  const blue = Math.min(255, Math.max(0, Number(components[2])));
  const alpha =
    components.length > 3
      ? Math.min(1, Math.max(0, Number(components[3])))
      : 1;
  const maximum = Math.max(red, green, blue);
  const minimum = Math.min(red, green, blue);
  const saturation = maximum > 0 ? (maximum - minimum) / maximum : 0;
  const linearize = (component) => {
    const normalized = component / 255;
    return normalized <= 0.04045
      ? normalized / 12.92
      : ((normalized + 0.055) / 1.055) ** 2.4;
  };
  const luminance =
    0.2126 * linearize(red) +
    0.7152 * linearize(green) +
    0.0722 * linearize(blue);
  const parsed_color = { alpha, saturation, luminance };
  color_cache.set(color, parsed_color);
  return parsed_color;
}

function normalize_article_theme_html(value) {
  const html = String(value || "");
  if (!html || typeof document === "undefined") {
    return html;
  }
  const template = document.createElement("template");
  const color_probe = document.createElement("span");
  const color_cache = new Map();
  template.innerHTML = html;
  color_probe.style.cssText =
    "position:fixed;left:-9999px;visibility:hidden;pointer-events:none";
  document.documentElement.appendChild(color_probe);
  try {
    const parse_color = (color) =>
      parse_article_color(color, color_probe, color_cache);
    const is_neutral_dark = (color) => {
      const parsed_color = parse_color(color);
      return Boolean(
        parsed_color &&
        parsed_color.alpha > 0.05 &&
        parsed_color.saturation <= 0.18 &&
        parsed_color.luminance <= 0.35,
      );
    };
    const is_neutral_light = (color) => {
      const parsed_color = parse_color(color);
      return Boolean(
        parsed_color &&
        parsed_color.alpha > 0.05 &&
        parsed_color.saturation <= 0.18 &&
        parsed_color.luminance >= 0.72,
      );
    };
    template.content.querySelectorAll("[style]").forEach((element) => {
      const color = element.style.getPropertyValue("color");
      const text_fill_color = element.style.getPropertyValue(
        "-webkit-text-fill-color",
      );
      const background_color =
        element.style.getPropertyValue("background-color");
      if (is_neutral_dark(color)) {
        element.style.removeProperty("color");
      }
      if (is_neutral_dark(text_fill_color)) {
        element.style.removeProperty("-webkit-text-fill-color");
      }
      if (is_neutral_light(background_color)) {
        element.style.removeProperty("background-color");
      }
      if (!element.style.cssText.trim()) {
        element.removeAttribute("style");
      }
    });
    template.content.querySelectorAll("font[color]").forEach((element) => {
      if (is_neutral_dark(element.getAttribute("color"))) {
        element.removeAttribute("color");
      }
    });
    template.content.querySelectorAll("[bgcolor]").forEach((element) => {
      if (is_neutral_light(element.getAttribute("bgcolor"))) {
        element.removeAttribute("bgcolor");
      }
    });
    return template.innerHTML;
  } finally {
    color_probe.remove();
  }
}

function normalize_article_body(data, create_article_html_content) {
  const source = data && typeof data === "object" ? data : {};
  const declared_format = String(source.type || "")
    .trim()
    .toLowerCase();
  const supported_formats = ["html", "markdown", "text"];
  const declared_content = supported_formats.includes(declared_format)
    ? first_non_empty(source[declared_format], source.content)
    : "";
  if (declared_content !== "") {
    const content = String(declared_content);
    const is_html = declared_format === "html";
    return {
      present: Boolean(content.trim()),
      format: declared_format,
      is_html,
      content:
        is_html && typeof create_article_html_content === "function"
          ? create_article_html_content(content)
          : content,
    };
  }

  for (const format of supported_formats) {
    const value = source[format];
    if (value !== undefined && value !== null && value !== "") {
      const content = String(value);
      const is_html = format === "html";
      return {
        present: Boolean(content.trim()),
        format,
        is_html,
        content:
          is_html && typeof create_article_html_content === "function"
            ? create_article_html_content(content)
            : content,
      };
    }
  }

  const content = String(source.content || "");
  const format = supported_formats.includes(declared_format)
    ? declared_format
    : "text";
  const is_html = format === "html";
  return {
    present: Boolean(content.trim()),
    format,
    is_html,
    content:
      is_html && typeof create_article_html_content === "function"
        ? create_article_html_content(content)
        : content,
  };
}

function normalize_detail_field(label, value) {
  if (value === undefined || value === null || value === "") {
    return null;
  }
  return { label, value: String(value) };
}

function normalize_generic_detail_fields(data) {
  const skipped_fields = new Set([
    "id",
    "url",
    "stream_url",
    "subtitle_url",
    "text",
    "html",
    "markdown",
    "images",
    "cover_url",
    "CoverURL",
    "coverUrl",
    "poster_url",
    "PosterURL",
    "posterUrl",
    "thumb_url",
    "ThumbURL",
    "thumbUrl",
    "thumbnail_url",
    "ThumbnailURL",
    "thumbnailUrl",
    "video_url",
    "VideoURL",
    "videoUrl",
    "origin_video_url",
    "OriginVideoURL",
    "originVideoUrl",
    "play_url",
    "PlayURL",
    "playUrl",
    "media_url",
    "MediaURL",
    "mediaUrl",
    "download_url",
    "DownloadURL",
    "downloadUrl",
    "deleted_at",
    "view_count",
    "ViewCount",
    "views",
    "like_count",
    "LikeCount",
    "likes",
    "comment_count",
    "CommentCount",
    "comments",
    "follower_count",
    "FollowerCount",
    "followers",
    "fans_count",
    "FansCount",
    "play_count",
    "play_times",
    "share_count",
    "favorite_count",
    "collect_count",
  ]);
  const fields = [];
  for (const [key, value] of Object.entries(data || {})) {
    if (
      skipped_fields.has(key) ||
      value === undefined ||
      value === null ||
      value === "" ||
      typeof value === "object"
    ) {
      continue;
    }
    let formatted_value = value;
    if (key === "duration") {
      formatted_value = format_duration(value);
    } else if (key === "size") {
      formatted_value = format_bytes(value);
    } else if (["is_finished", "is_original"].includes(key)) {
      formatted_value = Number(value) > 0 ? "是" : "否";
    }
    if (formatted_value === "" || formatted_value === 0) {
      continue;
    }
    fields.push({
      label: content_detail_field_names[key] || key,
      value: String(formatted_value),
    });
    if (fields.length >= 8) {
      break;
    }
  }
  return fields;
}

function normalize_video_detail_media(data, content) {
  const video_url = String(
    first_non_empty(
      data.video_url,
      data.VideoURL,
      data.videoUrl,
      data.origin_video_url,
      data.OriginVideoURL,
      data.originVideoUrl,
      data.play_url,
      data.PlayURL,
      data.playUrl,
      data.media_url,
      data.MediaURL,
      data.mediaUrl,
      data.download_url,
      data.DownloadURL,
      data.downloadUrl,
      data.url,
      data.URL,
      data.stream_url,
      data.StreamURL,
      data.streamUrl,
    ),
  ).trim();
  const cover_url = String(
    first_non_empty(
      data.cover_url,
      data.CoverURL,
      data.coverUrl,
      data.poster_url,
      data.PosterURL,
      data.posterUrl,
      data.thumb_url,
      data.ThumbURL,
      data.thumbUrl,
      data.thumbnail_url,
      data.ThumbnailURL,
      data.thumbnailUrl,
      data.cover,
      content && content.cover_url,
    ),
  ).trim();
  return {
    present: Boolean(video_url || cover_url),
    video_url,
    cover_url,
    has_video: Boolean(video_url),
    has_cover: Boolean(cover_url),
  };
}

function normalize_content_detail_subject(
  detail,
  fallback_content,
  detail_type,
) {
  const source =
    detail && detail.content && typeof detail.content === "object"
      ? detail.content
      : {};
  const id = String(
    first_non_empty(source.id, source.ID, detail && detail.key),
  ).trim();
  const type = String(
    first_non_empty(
      source.subtype,
      source.Subtype,
      source.type,
      source.Type,
      detail_type,
    ),
  )
    .trim()
    .toLowerCase();
  const title = String(
    first_non_empty(
      source.title,
      source.Title,
      source.description,
      source.Description,
      fallback_content && fallback_content.title,
      id,
    ),
  ).trim();
  const url = String(
    first_non_empty(
      source.url,
      source.URL,
      source.source_url,
      source.SourceURL,
      fallback_content && fallback_content.content_url,
    ),
  ).trim();
  const type_name = content_type_name(type);
  return {
    present: Boolean(Object.keys(source).length > 0 || id),
    id,
    type,
    type_name,
    title,
    url,
    has_url: Boolean(url),
    meta_text: [type_name, id].filter(Boolean).join(" · "),
  };
}

function normalize_content_detail_relation(detail) {
  const source =
    detail && detail.relation && typeof detail.relation === "object"
      ? detail.relation
      : {};
  const type = String(first_non_empty(source.type, source.Type))
    .trim()
    .toLowerCase();
  const source_content_id = String(
    first_non_empty(
      source.source_content_id,
      source.SourceContentId,
      source.SourceContentID,
    ),
  ).trim();
  const target_content_id = String(
    first_non_empty(
      source.target_content_id,
      source.TargetContentId,
      source.TargetContentID,
    ),
  ).trim();
  return {
    present: Boolean(type && source_content_id && target_content_id),
    type,
    type_name: content_relation_names[type] || type,
    source_content_id,
    target_content_id,
  };
}

function normalize_content_influencer_node(source) {
  const influencer = source && typeof source === "object" ? source : {};
  let id = String(
    first_non_empty(
      influencer.id,
      influencer.ID,
      influencer.influencer_id,
      influencer.InfluencerId,
      influencer.InfluencerID,
    ),
  ).trim();
  if (id === "0") {
    id = "";
  }
  const name = String(
    first_non_empty(
      influencer.name,
      influencer.Name,
      influencer.alias,
      influencer.Alias,
      id,
    ),
  ).trim();
  if (!name) {
    return null;
  }
  const alias = String(
    first_non_empty(influencer.alias, influencer.Alias),
  ).trim();
  const department = String(
    first_non_empty(
      influencer.known_for_department,
      influencer.KnownForDepartment,
    ),
  ).trim();
  const profile_url = String(
    first_non_empty(
      influencer.profile_url,
      influencer.ProfileURL,
      influencer.url,
      influencer.URL,
    ),
  ).trim();
  const identity = String(
    first_non_empty(
      influencer.tmdb_id,
      influencer.TMDBId,
      influencer.TMDBID,
      influencer.douban_id,
      influencer.DoubanId,
      influencer.DoubanID,
      influencer.imdb_id,
      influencer.IMDBId,
      influencer.IMDBID,
    ),
  ).trim();
  return {
    id,
    key: id || name.toLowerCase(),
    type: "influencer",
    type_name: "Influencer",
    title: name,
    url: profile_url,
    has_url: Boolean(profile_url),
    avatar_url: String(
      first_non_empty(influencer.avatar_url, influencer.AvatarURL),
    ).trim(),
    avatar_fallback: name.slice(0, 1),
    meta_text:
      [alias && alias !== name ? alias : "", department, identity]
        .filter(Boolean)
        .join(" · ") || "人物档案",
  };
}

function normalize_content_influencer_roles(reference, influencer) {
  let sources = [];
  if (reference && Array.isArray(reference.roles)) {
    sources = reference.roles;
  } else if (reference && Array.isArray(reference.Roles)) {
    sources = reference.Roles;
  } else if (reference && first_non_empty(reference.role, reference.Role)) {
    sources = [reference];
  }
  if (sources.length === 0 && influencer) {
    const department = first_non_empty(
      influencer.known_for_department,
      influencer.KnownForDepartment,
    );
    if (department) {
      sources = [{ role: department }];
    }
  }
  const roles_by_name = new Map();
  for (let role_index = 0; role_index < sources.length; role_index += 1) {
    const source = sources[role_index] || {};
    const role = String(first_non_empty(source.role, source.Role)).trim();
    if (!role) {
      continue;
    }
    const sort_order = number_or_default(
      first_non_empty(source.sort_order, source.SortOrder),
      role_index,
    );
    const current = roles_by_name.get(role);
    if (!current || sort_order < current.sort_order) {
      roles_by_name.set(role, {
        key: role,
        role,
        sort_order,
        metadata_json: String(
          first_non_empty(
            source.metadata_json,
            source.MetadataJSON,
            source.role_metadata_json,
            source.RoleMetadataJSON,
          ),
        ).trim(),
      });
    }
  }
  return Array.from(roles_by_name.values()).sort(
    (left, right) =>
      left.sort_order - right.sort_order ||
      left.role.localeCompare(right.role, "zh-CN"),
  );
}

function normalize_content_detail_influencers(detail, subject) {
  let references = [];
  if (detail && Array.isArray(detail.influencers)) {
    references = detail.influencers;
  } else if (detail && Array.isArray(detail.Influencers)) {
    references = detail.Influencers;
  }
  if (references.length === 0 || !subject || !subject.present) {
    return [];
  }
  const relations_by_influencer = new Map();
  for (
    let reference_index = 0;
    reference_index < references.length;
    reference_index += 1
  ) {
    const reference = references[reference_index] || {};
    const influencer_source =
      (reference.influencer && typeof reference.influencer === "object"
        ? reference.influencer
        : null) ||
      (reference.Influencer && typeof reference.Influencer === "object"
        ? reference.Influencer
        : null) ||
      reference;
    const influencer = normalize_content_influencer_node(influencer_source);
    if (!influencer) {
      continue;
    }
    const roles = normalize_content_influencer_roles(
      reference,
      influencer_source,
    );
    if (roles.length === 0) {
      continue;
    }
    const relation_key = `${subject.id}:content_influencer:${influencer.key}`;
    const existing = relations_by_influencer.get(relation_key);
    if (existing) {
      const roles_by_name = new Map(
        existing.roles.map((role) => [role.role, role]),
      );
      for (const role of roles) {
        const current = roles_by_name.get(role.role);
        if (!current || role.sort_order < current.sort_order) {
          roles_by_name.set(role.role, role);
        }
      }
      existing.roles = Array.from(roles_by_name.values()).sort(
        (left, right) =>
          left.sort_order - right.sort_order ||
          left.role.localeCompare(right.role, "zh-CN"),
      );
      existing.role_text = existing.roles
        .map((role) => role.role)
        .join(" / ");
      existing.sort_order = existing.roles[0].sort_order;
      continue;
    }
    relations_by_influencer.set(relation_key, {
      key: relation_key,
      type: "content_influencer",
      type_name: "人物角色",
      type_path_text: `influencer → content_influencer → ${subject.type || "content"}`,
      source: influencer,
      target: subject,
      roles,
      role_text: roles.map((role) => role.role).join(" / "),
      sort_order: roles[0].sort_order,
    });
  }
  return Array.from(relations_by_influencer.values()).sort(
    (left, right) =>
      left.sort_order - right.sort_order ||
      left.source.title.localeCompare(right.source.title, "zh-CN"),
  );
}

function normalize_content_relations(items, content) {
  const content_by_id = new Map();
  if (content && content.id) {
    content_by_id.set(String(content.id), {
      id: String(content.id),
      type: String(content.type || ""),
      type_name: content.content_type_name,
      title: content.title,
      url: content.content_url,
      has_url: Boolean(content.content_url),
      meta_text: [content.content_type_name, content.id]
        .filter(Boolean)
        .join(" · "),
    });
  }
  for (const item of items) {
    if (item.subject && item.subject.id) {
      content_by_id.set(item.subject.id, item.subject);
    }
  }
  const relation_items = [];
  const influencer_relations = [];
  for (const item of items) {
    influencer_relations.push(...(item.influencer_relations || []));
    const relation = item.relation || {};
    if (!relation.present) {
      continue;
    }
    const source = content_by_id.get(relation.source_content_id) || {
      id: relation.source_content_id,
      type_name: "内容",
      title: relation.source_content_id,
      url: "",
      has_url: false,
      meta_text: relation.source_content_id,
    };
    const target = content_by_id.get(relation.target_content_id) || {
      id: relation.target_content_id,
      type_name: "内容",
      title: relation.target_content_id,
      url: "",
      has_url: false,
      meta_text: relation.target_content_id,
    };
    relation_items.push({
      key: `${relation.source_content_id}:${relation.type}:${relation.target_content_id}`,
      type: relation.type,
      type_name: relation.type_name,
      source,
      target,
    });
  }
  const relation_chains = merge_content_relation_chains(relation_items);
  const count_parts = [];
  if (relation_chains.length > 0) {
    count_parts.push(`${relation_chains.length} 条关系链`);
  }
  if (influencer_relations.length > 0) {
    count_parts.push(`${influencer_relations.length} 个人物关联`);
  }
  return {
    present: relation_chains.length > 0 || influencer_relations.length > 0,
    content_present: relation_chains.length > 0,
    count_text: count_parts.join(" · "),
    items: relation_chains,
    influencers: {
      present: influencer_relations.length > 0,
      count_text: `${influencer_relations.length} 个关联`,
      items: influencer_relations,
    },
  };
}

function merge_content_relation_chains(relation_items) {
  if (!Array.isArray(relation_items) || relation_items.length === 0) {
    return [];
  }
  const incoming_by_content_id = new Map();
  const outgoing_by_content_id = new Map();
  for (const relation of relation_items) {
    const source_id = relation.source.id;
    const target_id = relation.target.id;
    if (!outgoing_by_content_id.has(source_id)) {
      outgoing_by_content_id.set(source_id, []);
    }
    if (!incoming_by_content_id.has(target_id)) {
      incoming_by_content_id.set(target_id, []);
    }
    outgoing_by_content_id.get(source_id).push(relation);
    incoming_by_content_id.get(target_id).push(relation);
  }

  const used_relation_keys = new Set();
  const relation_chains = [];
  const append_chain = (first_relation) => {
    if (!first_relation || used_relation_keys.has(first_relation.key)) {
      return;
    }
    const nodes = [first_relation.source];
    const edges = [];
    let relation = first_relation;
    while (relation && !used_relation_keys.has(relation.key)) {
      used_relation_keys.add(relation.key);
      edges.push(relation);
      nodes.push(relation.target);

      const target_id = relation.target.id;
      const incoming = incoming_by_content_id.get(target_id) || [];
      const outgoing = (outgoing_by_content_id.get(target_id) || []).filter(
        (candidate) => !used_relation_keys.has(candidate.key),
      );
      relation =
        incoming.length === 1 && outgoing.length === 1 ? outgoing[0] : null;
    }

    const segments = [];
    for (let node_index = 0; node_index < nodes.length; node_index += 1) {
      const node = nodes[node_index];
      segments.push({
        key: `node:${node_index}:${node.id}`,
        kind: "node",
        node,
      });
      if (node_index < edges.length) {
        const edge = edges[node_index];
        segments.push({
          key: `edge:${edge.key}`,
          kind: "edge",
          edge,
        });
      }
    }
    relation_chains.push({
      key: `chain:${edges.map((edge) => edge.key).join("|")}`,
      type_path_text: nodes.map((node) => node.type || "content").join(" → "),
      nodes,
      edges,
      segments,
    });
  };

  for (const relation of relation_items) {
    const source_incoming =
      incoming_by_content_id.get(relation.source.id) || [];
    if (source_incoming.length !== 1) {
      append_chain(relation);
    }
  }
  for (const relation of relation_items) {
    append_chain(relation);
  }
  return relation_chains;
}

function normalize_typed_content_detail(
  detail,
  detail_index,
  content,
  create_article_html_content,
  selected_video_variant,
) {
  const type = String((detail && detail.type) || "")
    .trim()
    .toLowerCase();
  const data =
    detail && detail.data && typeof detail.data === "object"
      ? detail.data
      : {};
  const key = String(
    (detail && detail.key) || data.id || `${type}:${detail_index}`,
  );
  const subject = normalize_content_detail_subject(detail, content, type);
  const relation = normalize_content_detail_relation(detail);
  const influencer_relations = normalize_content_detail_influencers(
    detail,
    subject,
  );
  const article_types = ["article", "answer", "question", "post", "blog"];
  const image_types = ["album", "image_set", "image"];
  let kind = "generic";
  let icon = "file-text";
  let title = `${content_type_name(type)}详情`;
  let model_name = "ContentDetail";
  let article_body = {
    present: false,
    format: "text",
    is_html: false,
    content: "",
  };
  let images = [];
  let variants = [];
  let text_tracks = Array.isArray(content.text_tracks)
    ? content.text_tracks
    : [];
  let media = {
    present: false,
    video_url: "",
    cover_url: "",
    has_video: false,
    has_cover: false,
  };
  if (["video", "short_video"].includes(type)) {
    kind = "video";
    icon = "file-play";
    model_name = "ContentVideo";
    media = normalize_video_detail_media(data, content);
    variants = download_info_array(data, "variants", "Variants").map(
      normalize_content_video_variant,
    );
    const selected_detail_key = String(
      (selected_video_variant && selected_video_variant.detail_key) || "",
    ).trim();
    const selected_variant_key =
      !selected_detail_key || selected_detail_key === key
        ? String(
            (selected_video_variant &&
              selected_video_variant.variant_key) ||
              "",
          ).trim()
        : "";
    const default_variant_index = Math.max(
      0,
      variants.findIndex((variant) => variant.is_default),
    );
    variants = variants.map((variant, variant_index) => {
      const selected = selected_variant_key
        ? variant.variant_key === selected_variant_key
        : variant_index === default_variant_index;
      return {
        ...variant,
        selected,
        badge_text: selected
          ? "已选择"
          : variant.is_default
            ? "默认"
            : "可选",
      };
    });
  } else if (article_types.includes(type)) {
    kind = "article";
    icon = "file-text";
    model_name = "ContentArticle";
    if (type === "answer") {
      title = "回答正文";
    } else if (type === "question") {
      title = "问题描述";
    } else if (type === "article" || type === "blog") {
      title = "文章正文";
    }
    article_body = normalize_article_body(data, create_article_html_content);
  } else if (image_types.includes(type)) {
    kind = "album";
    icon = "file-image";
    model_name = "ContentAlbum";
    images = (Array.isArray(data.images) ? data.images : [])
      .map((image, image_index) => {
        const live_photo =
          image.live_photo ||
          image.live_info ||
          image.livePhoto ||
          image.liveInfo;
        const is_live_photo = Boolean(
          image.image_type === "live_photo" ||
          (live_photo && typeof live_photo === "object"),
        );
        return {
          key: String(image.id || image.url || image_index),
          url: String(image.url || "").trim(),
          meta:
            [image.width, image.height].filter(Boolean).join(" × ") ||
            image.ext ||
            "图片",
          is_live_photo,
          live_photo_label: is_live_photo ? "实况" : "",
        };
      })
      .filter((image) => image.url)
      .slice(0, 12);
  } else if (type === "live") {
    kind = "live";
    icon = "file-play";
    title = "直播详情";
    model_name = "ContentLive";
  }
  return {
    key,
    type,
    type_name: content_type_name(type),
    kind,
    icon,
    title,
    model_name: [model_name, subject.meta_text].filter(Boolean).join(" · "),
    subject,
    relation,
    influencer_relations,
    fields: normalize_generic_detail_fields(data),
    article_body,
    images,
    media,
    variants,
    text_tracks,
    has_variants: variants.length > 0,
    has_text_tracks: text_tracks.length > 0,
    link_url: String(first_non_empty(data.url, data.stream_url)).trim(),
  };
}

function normalize_content_details(
  result,
  create_article_html_content,
  selected_video_variant,
) {
  const source_details =
    result && Array.isArray(result.content_details)
      ? result.content_details
      : [];
  const details = source_details.map((detail) => ({
    ...(detail || {}),
    data:
      detail && detail.data && typeof detail.data === "object"
        ? { ...detail.data }
        : {},
  }));
  const download_info =
    result && result.download_info && typeof result.download_info === "object"
      ? result.download_info
      : {};
  const download_content = download_info_object(
    download_info,
    "Content",
    "content",
  );
  const download_detail = download_info_object(
    download_info,
    "ContentDetail",
    "content_detail",
  );
  const download_detail_type = String(
    first_non_empty(download_content.type, download_content.Type),
  )
    .trim()
    .toLowerCase();
  if (download_detail_type && Object.keys(download_detail).length > 0) {
    const detail_index = details.findIndex(
      (detail) =>
        String((detail && detail.type) || "")
          .trim()
          .toLowerCase() === download_detail_type,
    );
    if (detail_index >= 0) {
      details[detail_index].data = {
        ...details[detail_index].data,
        ...download_detail,
      };
    } else {
      details.push({
        key: String(
          first_non_empty(
            download_content.id,
            download_content.Id,
            download_detail.id,
            download_detail.Id,
            download_detail_type,
          ),
        ),
        type: download_detail_type,
        data: download_detail,
      });
    }
  }
  let novel_data = null;
  const volumes = [];
  const chapters = [];
  const items = [];
  const content = normalize_content(result || {});
  for (
    let detail_index = 0;
    detail_index < details.length;
    detail_index += 1
  ) {
    const detail = details[detail_index] || {};
    const type = String(detail.type || "")
      .trim()
      .toLowerCase();
    const data =
      detail.data && typeof detail.data === "object" ? detail.data : {};
    if (type === "novel") {
      novel_data = data;
      continue;
    }
    if (type === "novel_volume") {
      const idx = Math.max(
        0,
        number_or_default(data.idx, volumes.length + 1),
      );
      volumes.push({
        key: String(
          detail.key || `${data.novel_id || "novel"}:volume:${idx}`,
        ),
        idx,
        index_text: idx > 0 ? `第 ${idx} 卷` : "卷",
        title: String(data.title || `第 ${idx || volumes.length + 1} 卷`),
      });
      continue;
    }
    if (type === "novel_chapter") {
      const idx = Math.max(
        0,
        number_or_default(data.idx, chapters.length + 1),
      );
      const word_count = Math.max(0, number_or_default(data.word_count, 0));
      chapters.push({
        key: String(
          detail.key || `${data.novel_id || "novel"}:chapter:${idx}`,
        ),
        idx,
        index_text: idx > 0 ? `第 ${idx} 章` : "章节",
        title: String(data.title || `第 ${idx || chapters.length + 1} 章`),
        url: String(data.url || "").trim(),
        locked: Boolean(data.locked),
        meta_text: data.locked
          ? "受限章节"
          : word_count > 0
            ? `${format_count(word_count)} 字`
            : "",
      });
      continue;
    }
    items.push(
      normalize_typed_content_detail(
        detail,
        detail_index,
        content,
        create_article_html_content,
        selected_video_variant,
      ),
    );
  }

  volumes.sort((left, right) => left.idx - right.idx);
  chapters.sort((left, right) => left.idx - right.idx);
  const novel_present = Boolean(
    novel_data || volumes.length || chapters.length,
  );
  const chapter_total = Math.max(
    chapters.length,
    number_or_default(novel_data && novel_data.chapter_count, 0),
  );
  const volume_total = Math.max(
    volumes.length,
    number_or_default(novel_data && novel_data.volume_count, 0),
  );
  const word_count = Math.max(
    0,
    number_or_default(novel_data && novel_data.word_count, 0),
  );
  const novel_title = String(
    first_non_empty(
      novel_data && novel_data.series_name,
      content.title,
      "小说详情",
    ),
  );
  const author_name = String(
    (novel_data && novel_data.author_name) || "",
  ).trim();
  const novel_metrics = [
    normalize_detail_field(
      "章节",
      chapter_total > 0 ? `${chapter_total} 章` : "统计中",
    ),
    normalize_detail_field(
      "分卷",
      volume_total > 0 ? `${volume_total} 卷` : "未分卷",
    ),
    normalize_detail_field(
      "字数",
      word_count > 0 ? `${format_count(word_count)} 字` : "统计中",
    ),
    normalize_detail_field(
      "状态",
      novel_data
        ? Number(novel_data.is_finished) > 0
          ? "已完结"
          : "连载中"
        : "获取中",
    ),
  ].filter(Boolean);
  const relations = normalize_content_relations(items, content);

  return {
    present: details.length > 0,
    count_text: `${details.length} 项详情`,
    relations,
    novel: {
      present: novel_present,
      title: novel_title,
      subtitle: author_name
        ? `ContentNovel · 作者：${author_name}`
        : "ContentNovel · 小说结构",
      progress_text:
        chapter_total > 0
          ? `已获取 ${chapters.length}/${chapter_total} 章`
          : `已获取 ${chapters.length} 章`,
      metrics: novel_metrics,
      volumes,
      chapters,
      has_volumes: volumes.length > 0,
      has_chapters: chapters.length > 0,
      empty_chapter_text:
        chapter_total > 0 ? "章节详情正在获取中…" : "暂无章节详情",
    },
    items,
  };
}

function normalize_content(result) {
  const source =
    result && result.content && typeof result.content === "object"
      ? result.content
      : {};
  const platform_id = first_non_empty(
    source.platform_id,
    source.PlatformID,
    result && result.platform,
  );
  const content_type = first_non_empty(
    source.type,
    source.content_type,
    source.ContentType,
  );
  const title = first_non_empty(
    source.title,
    source.Title,
    source.description,
    source.Description,
    "未命名内容",
  );
  const description = String(
    first_non_empty(source.description, source.Description),
  ).trim();
  const text_tracks = download_info_array(
    source,
    "text_tracks",
    "TextTracks",
  ).map(normalize_content_text_track);
  return {
    present: Boolean(result && result.content),
    id: first_non_empty(source.id, source.ID),
    type: String(content_type || "")
      .trim()
      .toLowerCase(),
    title,
    description,
    show_description: Boolean(description && description !== title),
    cover_url: first_non_empty(
      source.cover_url,
      source.CoverURL,
      source.coverUrl,
    ),
    content_url: first_non_empty(
      source.url,
      source.URL,
      source.source_url,
      source.SourceURL,
      result && result.url,
    ),
    platform_name: platform_name(platform_id),
    content_type_name: content_type_name(content_type),
    publish_time_text: format_time(
      first_non_empty(source.publish_time, source.PublishTime),
    ),
    text_tracks,
  };
}

function normalize_account(result) {
  const source =
    result && result.account && typeof result.account === "object"
      ? result.account
      : {};
  const platform_id = first_non_empty(
    source.platform_id,
    source.PlatformID,
    result && result.platform,
  );
  const nickname = first_non_empty(
    source.nickname,
    source.Nickname,
    source.alias,
    source.Alias,
    source.external_id,
    source.ExternalID,
    "未获取到账号",
  );
  const alias = first_non_empty(source.alias, source.Alias);
  const external_id = first_non_empty(source.external_id, source.ExternalID);
  const identity = alias && alias !== nickname ? alias : external_id;
  return {
    present: Boolean(result && result.account),
    nickname,
    identity: identity && identity !== nickname ? identity : "",
    signature: String(
      first_non_empty(source.signature, source.Signature),
    ).trim(),
    avatar_url: first_non_empty(
      source.avatar_url,
      source.AvatarURL,
      source.avatarUrl,
    ),
    profile_url: first_non_empty(
      source.profile_url,
      source.ProfileURL,
      source.profileUrl,
    ),
    platform_name: platform_name(platform_id),
    avatar_fallback: String(nickname).slice(0, 1),
  };
}

function raw_result_payload(source, include_result_key = false) {
  if (!source || typeof source !== "object") {
    return { present: false, value: undefined };
  }
  if (has_own_property(source, "raw_result")) {
    return {
      present: source.raw_result !== undefined,
      value: source.raw_result,
    };
  }
  if (has_own_property(source, "RawResult")) {
    return {
      present: source.RawResult !== undefined,
      value: source.RawResult,
    };
  }
  if (include_result_key && has_own_property(source, "result")) {
    return { present: source.result !== undefined, value: source.result };
  }
  return { present: false, value: undefined };
}

function raw_result_type(value) {
  if (Array.isArray(value)) {
    return "array";
  }
  if (value === null) {
    return "null";
  }
  return typeof value;
}

function raw_result_text(value) {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed && /^[{[]/.test(trimmed)) {
      try {
        return JSON.stringify(JSON.parse(trimmed), null, 2);
      } catch {
        // Preserve non-JSON strings as-is.
      }
    }
    return value;
  }
  const serialized = JSON.stringify(value, null, 2);
  return serialized === undefined ? String(value) : serialized;
}

function normalize_raw_result(result) {
  const payload = raw_result_payload(result, true);
  if (!payload.present) {
    return {
      present: false,
      type_text: "",
      meta_text: "",
      text: "",
    };
  }
  const text = raw_result_text(payload.value);
  const type_text = raw_result_type(payload.value);
  const char_count = text.length;
  const platform_text = platform_name(result && result.platform);
  return {
    present: true,
    type_text,
    meta_text: [platform_text, type_text, `${char_count} 字符`]
      .filter(Boolean)
      .join(" · "),
    text,
  };
}

function has_display_result(result) {
  if (!result || typeof result !== "object") {
    return false;
  }
  return Boolean(
    result.content ||
    result.account ||
    (Array.isArray(result.content_details) &&
      result.content_details.length) ||
    (Array.isArray(result.cache_entries) && result.cache_entries.length) ||
    result.download_info,
  );
}

export { ScraperPageViewModel };

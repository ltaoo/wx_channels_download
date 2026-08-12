/// <reference path="../utils.js" />
/**
 * @file Scraper fetch page data, state, and formatting logic.
 */
var HomePageModel = (() => {
  const home_api_origin = WXEnv.get("apiOrigin");
  const home_http_client = new Timeless.kit.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: home_api_origin,
  });
  Timeless.web.provide_http_client(home_http_client);

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
    has_subtitle: "字幕",
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

  function platform_name(value) {
    const platform_id = String(value || "").trim();
    return platform_names[platform_id] || platform_id || "未知平台";
  }

  function content_type_name(value) {
    const content_type = String(value || "").trim().toLowerCase();
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

  function normalize_detail_preview(value) {
    const text = String(value || "")
      .replace(/<style[\s\S]*?<\/style>/gi, " ")
      .replace(/<script[\s\S]*?<\/script>/gi, " ")
      .replace(/<[^>]+>/g, " ")
      .replace(/&(?:nbsp|ensp|emsp);/gi, " ")
      .replace(/&amp;/gi, "&")
      .replace(/&lt;/gi, "<")
      .replace(/&gt;/gi, ">")
      .replace(/&quot;/gi, '"')
      .replace(/&#39;/gi, "'")
      .replace(/\s+/g, " ")
      .trim();
    return text.length > 280 ? `${text.slice(0, 280)}…` : text;
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
      "lyrics_url",
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
      } else if (["is_finished", "is_original", "has_subtitle"].includes(key)) {
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

  function normalize_typed_content_detail(detail, detail_index, content) {
    const type = String((detail && detail.type) || "").trim().toLowerCase();
    const data =
      detail && detail.data && typeof detail.data === "object"
        ? detail.data
        : {};
    const key = String(
      (detail && detail.key) || data.id || `${type}:${detail_index}`,
    );
    const article_types = ["article", "answer", "question", "post", "blog"];
    const image_types = ["album", "image_set", "image"];
    let kind = "generic";
    let icon = "file-text";
    let title = `${content_type_name(type)}详情`;
    let preview = "";
    let images = [];
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
      media = normalize_video_detail_media(data, content);
    } else if (article_types.includes(type)) {
      kind = "article";
      icon = "file-text";
      preview = normalize_detail_preview(
        first_non_empty(data.markdown, data.text, data.html),
      );
    } else if (image_types.includes(type)) {
      kind = "album";
      icon = "file-image";
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
    }
    return {
      key,
      type,
      type_name: content_type_name(type),
      kind,
      icon,
      title,
      fields: normalize_generic_detail_fields(data),
      preview,
      images,
      media,
      link_url: String(first_non_empty(data.url, data.stream_url)).trim(),
    };
  }

  function normalize_content_details(result) {
    const details =
      result && Array.isArray(result.content_details)
        ? result.content_details
        : [];
    let novel_data = null;
    const volumes = [];
    const chapters = [];
    const items = [];
    const content = normalize_content(result || {});
    for (let detail_index = 0; detail_index < details.length; detail_index += 1) {
      const detail = details[detail_index] || {};
      const type = String(detail.type || "").trim().toLowerCase();
      const data = detail.data && typeof detail.data === "object" ? detail.data : {};
      if (type === "novel") {
        novel_data = data;
        continue;
      }
      if (type === "novel_volume") {
        const idx = Math.max(0, number_or_default(data.idx, volumes.length + 1));
        volumes.push({
          key: String(detail.key || `${data.novel_id || "novel"}:volume:${idx}`),
          idx,
          index_text: idx > 0 ? `第 ${idx} 卷` : "卷",
          title: String(data.title || `第 ${idx || volumes.length + 1} 卷`),
        });
        continue;
      }
      if (type === "novel_chapter") {
        const idx = Math.max(0, number_or_default(data.idx, chapters.length + 1));
        const word_count = Math.max(0, number_or_default(data.word_count, 0));
        chapters.push({
          key: String(detail.key || `${data.novel_id || "novel"}:chapter:${idx}`),
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
      items.push(normalize_typed_content_detail(detail, detail_index, content));
    }

    volumes.sort((left, right) => left.idx - right.idx);
    chapters.sort((left, right) => left.idx - right.idx);
    const novel_present = Boolean(novel_data || volumes.length || chapters.length);
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
      first_non_empty(novel_data && novel_data.series_name, content.title, "小说详情"),
    );
    const author_name = String((novel_data && novel_data.author_name) || "").trim();
    const novel_metrics = [
      normalize_detail_field("章节", chapter_total > 0 ? `${chapter_total} 章` : "统计中"),
      normalize_detail_field("分卷", volume_total > 0 ? `${volume_total} 卷` : "未分卷"),
      normalize_detail_field("字数", word_count > 0 ? `${format_count(word_count)} 字` : "统计中"),
      normalize_detail_field(
        "状态",
        novel_data ? (Number(novel_data.is_finished) > 0 ? "已完结" : "连载中") : "获取中",
      ),
    ].filter(Boolean);

    return {
      present: details.length > 0,
      count_text: `${details.length} 项详情`,
      novel: {
        present: novel_present,
        title: novel_title,
        subtitle: author_name ? `作者：${author_name}` : "小说结构",
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
    return {
      present: Boolean(result && result.content),
      id: first_non_empty(source.id, source.ID),
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
    const external_id = first_non_empty(
      source.external_id,
      source.ExternalID,
    );
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

  function create_model() {
    const url_ = ref("");
    const loading_ = ref(false);
    const error_ = ref("");
    const result_ = ref(null);
    const json_expanded_ = ref(false);
    const download_loading_ = ref(false);
    const download_error_ = ref("");
    const download_success_ = ref("");
    const fetch_progress_ = ref(null);
    const fetch_notice_ = ref("");
    const interrupt_loading_ = ref(false);
    const cache_loading_ = ref(false);
    const cache_error_ = ref("");
    const chapter_display_limit_ = ref(100);
    const platform_statuses_ = ref([]);
    const platform_status_popover_ = new Timeless.vm.PopoverCore({
      offsetY: 8,
      destroyOnClose: false,
    });
    let request_sequence = 0;
    let active_job_id_ = "";
    let websocket_ = null;
    let websocket_connect_promise_ = null;
    let websocket_reconnect_timeout_id_ = 0;
    let poll_timeout_id_ = 0;
    let resolving_terminal_job_id_ = "";
    let platform_status_hide_timeout_id_ = 0;
    let disposed_ = false;

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
    const download_request = new Timeless.kit.RequestCore(
      (body) => home_request.post("/api/v1/download_task/create", body),
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

    const submit_disabled_ = combine(
      {
        url: url_,
        loading: loading_,
        download_loading: download_loading_,
        cache_loading: cache_loading_,
      },
      (state) =>
        state.loading ||
        state.download_loading ||
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
              String(status.name || "").trim() || platform_name(status.platform),
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
        if (state.cache_loading) {
          return "正在清理缓存...";
        }
        return (
          state.error ||
          state.download_error ||
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
        cache_loading: cache_loading_,
      },
      (state) =>
        state.loading || state.download_loading || state.cache_loading,
    );
    const has_error_ = combine(
      {
        error: error_,
        download_error: download_error_,
        cache_error: cache_error_,
      },
      (state) =>
        Boolean(state.error || state.download_error || state.cache_error),
    );
    const has_result_ = computed(result_, (data) => Boolean(data));
    const progress_visible_ = combine(
      { loading: loading_, progress: fetch_progress_ },
      (state) =>
        Boolean(
          state.loading &&
            state.progress &&
            number_or_default(state.progress.total, 0) > 0,
        ),
    );
    const progress_percent_ = computed(fetch_progress_, (progress) => {
      const percent = number_or_default(progress && progress.percent, 0);
      return Math.min(100, Math.max(0, percent));
    });
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
      return cache_hits > 0
        ? `${current}/${total} 章 · 已复用 ${cache_hits} 页缓存`
        : `${current}/${total} 章`;
    });
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
        url: url_,
        loading: loading_,
        download_loading: download_loading_,
        cache_loading: cache_loading_,
      },
      (state) =>
        state.loading ||
        state.download_loading ||
        state.cache_loading ||
        !String(state.url || "").trim(),
    );
    const interrupt_disabled_ = computed(
      interrupt_loading_,
      (loading) => Boolean(loading),
    );
    const normalized_content_ = computed(result_, normalize_content);
    const normalized_account_ = computed(result_, normalize_account);
    const normalized_content_details_ = computed(
      result_,
      normalize_content_details,
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
      items: computed(
        normalized_content_details_,
        (details) => details.items,
      ),
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
    const result_text_ = computed(result_, (data) =>
      data ? JSON.stringify(data, null, 2) : "",
    );
    const json_toggle_text_ = computed(json_expanded_, (expanded) =>
      expanded ? "收起原始 JSON" : "查看原始 JSON",
    );
    const download_disabled_ = combine(
      {
        result: result_,
        fetch_loading: loading_,
        loading: download_loading_,
        success: download_success_,
      },
      (state) =>
        !state.result ||
        state.fetch_loading ||
        state.loading ||
        Boolean(state.success),
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
      if (!request_id || request_id !== active_job_id_) {
        return;
      }
      const normalized_progress = {
        ...progress,
        current: Math.max(0, number_or_default(progress.current, 0)),
        total: Math.max(0, number_or_default(progress.total, 0)),
        percent: Math.min(
          100,
          Math.max(0, number_or_default(progress.percent, 0)),
        ),
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
      const raw_status = String(status.status || "").trim().toLowerCase();
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
      if (poll_timeout_id_) {
        window.clearTimeout(poll_timeout_id_);
        poll_timeout_id_ = 0;
      }
    }

    function clear_websocket_reconnect() {
      if (websocket_reconnect_timeout_id_) {
        window.clearTimeout(websocket_reconnect_timeout_id_);
        websocket_reconnect_timeout_id_ = 0;
      }
    }

    function close_scraper_websocket() {
      clear_websocket_reconnect();
      const websocket = websocket_;
      websocket_ = null;
      websocket_connect_promise_ = null;
      if (websocket) {
        try {
          websocket.close();
        } catch (error) {}
      }
    }

    function clear_platform_status_popover_hide() {
      if (platform_status_hide_timeout_id_) {
        window.clearTimeout(platform_status_hide_timeout_id_);
        platform_status_hide_timeout_id_ = 0;
      }
    }

    function show_platform_status_popover() {
      clear_platform_status_popover_hide();
      platform_status_popover_.show();
    }

    function schedule_platform_status_popover_hide() {
      clear_platform_status_popover_hide();
      platform_status_hide_timeout_id_ = window.setTimeout(() => {
        platform_status_hide_timeout_id_ = 0;
        platform_status_popover_.hide();
      }, 120);
    }

    function finish_job_tracking() {
      clear_job_poll();
    }

    async function resolve_terminal_job(job, sequence) {
      const job_id = String((job && job.id) || "").trim();
      if (
        !job_id ||
        job_id !== active_job_id_ ||
        sequence !== request_sequence ||
        resolving_terminal_job_id_ === job_id
      ) {
        return;
      }
      resolving_terminal_job_id_ = job_id;

      let final_job = job;
      if (job.status === "completed" && !job.output) {
        const result = await job_request.run({ id: job_id });
        if (sequence !== request_sequence || job_id !== active_job_id_) {
          return;
        }
        if (result.error) {
          resolving_terminal_job_id_ = "";
          schedule_job_poll(sequence, 500);
          return;
        }
        final_job = result.data || job;
      }

      loading_.as(false);
      interrupt_loading_.as(false);
      finish_job_tracking();
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

    function apply_fetch_job_payload(job, event) {
      const content = (event && event.content) || job.content;
      const account = (event && event.account) || job.account;
      const job_details = Array.isArray(job.content_details)
        ? job.content_details
        : [];
      const event_detail = event && event.content_detail;
      if (!content && !account && job_details.length === 0 && !event_detail) {
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
      result_.as({
        ...current_result,
        job_id: job.id,
        platform: job.platform,
        url: job.url,
        content: content || current_result.content,
        account: account || current_result.account,
        content_details,
      });
    }

    function apply_scraper_job(job, sequence, event = null) {
      if (!job || typeof job !== "object") {
        return;
      }
      const job_id = String(job.id || "").trim();
      if (
        !job_id ||
        job_id !== active_job_id_ ||
        sequence !== request_sequence
      ) {
        return;
      }
      apply_fetch_job_payload(job, event);
      const progress = (event && event.progress) || job.progress;
      if (progress) {
        apply_fetch_progress(progress);
      } else if (
        event &&
        event.stage === "content_detail" &&
        number_or_default(event.total, 0) > 0
      ) {
        const current = Math.max(0, number_or_default(event.current, 0));
        const total = Math.max(0, number_or_default(event.total, 0));
        apply_fetch_progress({
          request_id: job_id,
          platform: job.platform,
          stage: event.stage,
          status: job.status,
          current,
          total,
          percent: total > 0 ? (current * 100) / total : 0,
          message: `正在获取内容详情 ${current}/${total}`,
        });
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
      const job_id = active_job_id_;
      if (!job_id || sequence !== request_sequence || disposed_) {
        return null;
      }
      const result = await job_request.run({ id: job_id });
      if (sequence !== request_sequence || job_id !== active_job_id_) {
        return result;
      }
      if (result.error) {
        return result;
      }
      apply_scraper_job(result.data || {}, sequence);
      return result;
    }

    function schedule_job_poll(sequence, delay = 1000) {
      clear_job_poll();
      if (
        disposed_ ||
        !loading_.value ||
        !active_job_id_ ||
        sequence !== request_sequence
      ) {
        return;
      }
      poll_timeout_id_ = window.setTimeout(async () => {
        poll_timeout_id_ = 0;
        await refresh_scraper_job(sequence);
        if (loading_.value && sequence === request_sequence) {
          schedule_job_poll(sequence);
        }
      }, delay);
    }

    function schedule_scraper_websocket_reconnect(delay = 1000) {
      if (disposed_ || websocket_reconnect_timeout_id_) {
        return;
      }
      websocket_reconnect_timeout_id_ = window.setTimeout(() => {
        websocket_reconnect_timeout_id_ = 0;
        void connect_scraper_websocket().catch(() => false);
      }, delay);
    }

    function connect_scraper_websocket() {
      if (disposed_) {
        return Promise.resolve(false);
      }
      if (websocket_ && websocket_.readyState === WebSocket.OPEN) {
        return Promise.resolve(true);
      }
      if (websocket_connect_promise_) {
        return websocket_connect_promise_;
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
          } catch (error) {}
        }, 1500);

        websocket.onopen = () => {
          if (disposed_ || websocket_ !== websocket) {
            websocket.close();
            return;
          }
          if (loading_.value && active_job_id_) {
            schedule_job_poll(request_sequence, 2000);
          }
          window.clearTimeout(timeout_id);
          if (!settled) {
            settled = true;
            resolve(true);
          }
        };
        websocket.onmessage = (event) => {
          const [parse_error, message] = WXU.parseJSON(event.data);
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
          } catch (error) {}
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
          if (loading_.value && active_job_id_) {
            schedule_job_poll(request_sequence, 250);
          }
          if (!disposed_) {
            schedule_scraper_websocket_reconnect();
          }
        };
      });
      websocket_connect_promise_ = connection_promise;
      connection_promise.then(
        () => {
          if (websocket_connect_promise_ === connection_promise) {
            websocket_connect_promise_ = null;
          }
        },
        () => {
          if (websocket_connect_promise_ === connection_promise) {
            websocket_connect_promise_ = null;
          }
        },
      );
      return connection_promise;
    }

    function dispose() {
      disposed_ = true;
      request_sequence += 1;
      active_job_id_ = "";
      resolving_terminal_job_id_ = "";
      finish_job_tracking();
      close_scraper_websocket();
      clear_platform_status_popover_hide();
      platform_status_popover_.hide();
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
      finish_job_tracking();
      disposed_ = false;
      active_job_id_ = "";
      resolving_terminal_job_id_ = "";
      loading_.as(true);
      error_.as("");
      result_.as(null);
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
      active_job_id_ = job_id;
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
      const job_id = active_job_id_;
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

    async function clear_fetch_cache() {
      if (cache_action_disabled_.value) {
        return null;
      }
      const raw_url = String(url_.value || "").trim();
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
      fetch_notice_.as(removed ? "抓取缓存已清理" : "该 URL 暂无抓取缓存");
      return result;
    }

    async function create_download_task() {
      if (download_loading_.value || download_success_.value) {
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
      const body = {
        objects: [
          {
            platform,
            content,
            config: { platform },
          },
        ],
      };
      const result = await download_request.run(body);
      download_loading_.as(false);
      if (result.error) {
        download_error_.as(result.error.message || String(result.error));
        return result;
      }

      const tasks =
        result.data && Array.isArray(result.data.tasks)
          ? result.data.tasks
          : [];
      const failed_task = tasks.find(
        (task) => task && Number(task.code || 0) !== 0,
      );
      if (failed_task) {
        download_error_.as(failed_task.msg || "创建下载任务失败");
        return result;
      }

      download_success_.as("下载任务创建成功");
      return result;
    }

    function open_external_url(value) {
      const target_url = String(value || "").trim();
      if (!target_url) {
        return;
      }
      window.open(target_url, "_blank", "noopener,noreferrer");
    }

    const methods = {
      setURL(value) {
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
      },
      submit,
      forceRefresh() {
        return submit({ force_refresh: true });
      },
      interruptFetch: interrupt_fetch,
      clearFetchCache: clear_fetch_cache,
      createDownloadTask: create_download_task,
      toggleJSON() {
        json_expanded_.as(!json_expanded_.value);
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
      schedulePlatformStatusPopoverHide:
        schedule_platform_status_popover_hide,
      connectProgress() {
        disposed_ = false;
        return connect_scraper_websocket().catch(() => false);
      },
      dispose,
    };

    return {
      state: {
        url: url_,
        loading: loading_,
        error: error_,
        result: result_,
        content: content_,
        account: account_,
        content_details: content_details_,
        json_expanded: json_expanded_,
        download_loading: download_loading_,
        download_error: download_error_,
        download_success: download_success_,
        fetch_progress: fetch_progress_,
        fetch_notice: fetch_notice_,
        interrupt_loading: interrupt_loading_,
        cache_loading: cache_loading_,
        cache_error: cache_error_,
        progress_visible: progress_visible_,
        progress_percent: progress_percent_,
        progress_percent_text: progress_percent_text_,
        progress_count_text: progress_count_text_,
        submit_button_text: submit_button_text_,
        cache_action_disabled: cache_action_disabled_,
        interrupt_disabled: interrupt_disabled_,
        submit_disabled: submit_disabled_,
        status_text: status_text_,
        busy: busy_,
        has_error: has_error_,
        has_result: has_result_,
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
      },
      ui: {
        platform_status_popover: platform_status_popover_,
      },
      methods,
    };
  }

  return create_model;
})();

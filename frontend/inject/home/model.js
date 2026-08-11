/// <reference path="../utils.js" />
/**
 * @file Scraper fetch page data, state, and formatting logic.
 */
var HomePageModel = (() => {
  const home_api_origin = WXEnv.get("apiOrigin");
  const home_http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: home_api_origin,
  });
  Timeless.web.provide_http_client(home_http_client);

  const home_request = Timeless.request_factory({
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
    const view_count = number_or_default(
      first_non_empty(source.view_count, source.ViewCount),
      0,
    );
    const like_count = number_or_default(
      first_non_empty(source.like_count, source.LikeCount),
      0,
    );
    const comment_count = number_or_default(
      first_non_empty(source.comment_count, source.CommentCount),
      0,
    );
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
      view_count_text: format_count(view_count),
      like_count_text: format_count(like_count),
      comment_count_text: format_count(comment_count),
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
    const follower_count = number_or_default(
      first_non_empty(source.follower_count, source.FollowerCount),
      0,
    );

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
      follower_count_text: `${format_count(follower_count)} 粉丝`,
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
    let request_sequence = 0;

    const fetch_request = new Timeless.RequestCore(
      (params) => home_request.get("/api/scraper/fetch", params),
      {
        client: home_http_client,
      },
    );
    const download_request = new Timeless.RequestCore(
      (body) => home_request.post("/api/v1/download_task/create", body),
      {
        client: home_http_client,
      },
    );

    const submit_disabled_ = combine(
      {
        url: url_,
        loading: loading_,
        download_loading: download_loading_,
      },
      (state) =>
        state.loading ||
        state.download_loading ||
        !String(state.url || "").trim(),
    );
    const status_text_ = combine(
      {
        loading: loading_,
        error: error_,
        download_loading: download_loading_,
        download_error: download_error_,
        download_success: download_success_,
      },
      (state) => {
        if (state.loading) {
          return "正在获取...";
        }
        if (state.download_loading) {
          return "正在创建下载任务...";
        }
        return (
          state.error || state.download_error || state.download_success || ""
        );
      },
    );
    const busy_ = combine(
      { loading: loading_, download_loading: download_loading_ },
      (state) => state.loading || state.download_loading,
    );
    const has_error_ = combine(
      { error: error_, download_error: download_error_ },
      (state) => Boolean(state.error || state.download_error),
    );
    const has_result_ = computed(result_, (data) => Boolean(data));
    const normalized_content_ = computed(result_, normalize_content);
    const normalized_account_ = computed(result_, normalize_account);
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
      view_count_text: computed(
        normalized_content_,
        (content) => content.view_count_text,
      ),
      like_count_text: computed(
        normalized_content_,
        (content) => content.like_count_text,
      ),
      comment_count_text: computed(
        normalized_content_,
        (content) => content.comment_count_text,
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
      follower_count_text: computed(
        normalized_account_,
        (account) => account.follower_count_text,
      ),
      avatar_fallback: computed(
        normalized_account_,
        (account) => account.avatar_fallback,
      ),
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
        loading: download_loading_,
        success: download_success_,
      },
      (state) => !state.result || state.loading || Boolean(state.success),
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

    async function submit() {
      if (loading_.value || download_loading_.value) {
        return null;
      }
      const raw_url = String(url_.value || "").trim();
      if (!raw_url) {
        error_.as("请输入 URL");
        return null;
      }

      const sequence = ++request_sequence;
      loading_.as(true);
      error_.as("");
      result_.as(null);
      json_expanded_.as(false);
      download_error_.as("");
      download_success_.as("");

      const result = await fetch_request.run({ url: raw_url });
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      if (result.error) {
        error_.as(result.error.message || String(result.error));
        return result;
      }

      result_.as(result.data || {});
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
      },
      submit,
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
    };

    return {
      state: {
        url: url_,
        loading: loading_,
        error: error_,
        result: result_,
        content: content_,
        account: account_,
        json_expanded: json_expanded_,
        download_loading: download_loading_,
        download_error: download_error_,
        download_success: download_success_,
        submit_disabled: submit_disabled_,
        status_text: status_text_,
        busy: busy_,
        has_error: has_error_,
        has_result: has_result_,
        result_text: result_text_,
        json_toggle_text: json_toggle_text_,
        download_disabled: download_disabled_,
        download_button_text: download_button_text_,
      },
      methods,
    };
  }

  return create_model;
})();

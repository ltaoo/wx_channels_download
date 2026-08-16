const TYPE_ICONS = {
  image: "\u{1F5BC}",
  video: "\u{1F3AC}",
  audio: "\u{1F3B5}",
  html: "\u{1F310}",
  zip: "\u{1F4E6}",
  pdf: "\u{1F4C4}",
  other: "\u{1F4C1}",
};

const PLATFORM_FAVICONS = {
  wxchannels:
    "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
  wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
};

const PLATFORM_NAMES = {
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

const preview_detail_request = create_request("获取下载任务详情失败");

const preview_file_request = create_request("读取压缩包失败");

function create_request(fallback_message) {
  return Timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || fallback_message,
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });
}

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

function error_message(error, fallback) {
  if (error && error.message) {
    return error.message;
  }
  return error ? String(error) : fallback;
}

function platform_favicon(platform_id) {
  return PLATFORM_FAVICONS[platform_id] || "";
}

function platform_name(platform_id) {
  return PLATFORM_NAMES[platform_id] || platform_id || "";
}

function format_bytes(bytes) {
  const value = number_or_default(bytes, 0);
  if (value <= 0) {
    return "0 B";
  }
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(
    sizes.length - 1,
    Math.floor(Math.log(value) / Math.log(1024)),
  );
  return `${parseFloat((value / Math.pow(1024, index)).toFixed(1))} ${sizes[index]}`;
}

function file_type_icon(file_type) {
  return TYPE_ICONS[file_type] || TYPE_ICONS.other;
}

function file_url(file) {
  if (!file) {
    return "";
  }
  if (file.file_url) {
    return file.file_url;
  }
  return `/api/file?path=${encodeURIComponent(file.local_path || "")}`;
}

function normalize_account(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    ...source,
    nickname: first_non_empty(source.nickname, source.Nickname),
    avatar_url: first_non_empty(
      source.avatar_url,
      source.avatarUrl,
      source.AvatarURL,
    ),
    external_id: first_non_empty(
      source.external_id,
      source.externalId,
      source.ExternalID,
    ),
  };
}

function normalize_file(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    ...source,
    name: first_non_empty(source.name, source.Name, "未命名文件"),
    local_path: first_non_empty(
      source.local_path,
      source.localPath,
      source.LocalPath,
    ),
    file_url: first_non_empty(
      source.file_url,
      source.fileUrl,
      source.FileURL,
    ),
    file_type: first_non_empty(
      source.file_type,
      source.fileType,
      source.FileType,
      "other",
    ),
    status: first_non_empty(source.status, source.Status),
    size: Math.max(0, number_or_default(source.size || source.Size, 0)),
    progress: Math.min(
      100,
      Math.max(
        0,
        number_or_default(source.progress || source.Progress, 0),
      ),
    ),
    exists: Boolean(
      typeof source.exists !== "undefined"
        ? source.exists
        : source.Exists,
    ),
  };
}

function normalize_task(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const content =
    source.content && typeof source.content === "object"
      ? source.content
      : {};
  const accounts = Array.isArray(content.accounts)
    ? content.accounts.map(normalize_account)
    : [];
  const files = Array.isArray(source.files)
    ? source.files.map(normalize_file)
    : [];
  const platform_id = first_non_empty(
    content.platform_id,
    content.platformId,
    source.platform_id,
    source.platformId,
  );
  return {
    ...source,
    content: {
      ...content,
      accounts,
    },
    files,
    title: first_non_empty(
      content.title,
      content.Title,
      source.name,
      source.Name,
      "未命名内容",
    ),
    name: first_non_empty(source.name, source.Name, "Preview"),
    platform_id,
    platform_name: platform_name(platform_id),
    platform_favicon: platform_favicon(platform_id),
    content_type: first_non_empty(
      source.content_type,
      source.contentType,
      content.type,
      content.Type,
    ),
    account: accounts.length > 0 ? accounts[0] : null,
  };
}

function normalize_zip_images(data) {
  const images = data && Array.isArray(data.images) ? data.images : [];
  return images
    .filter((image) => image && image.url)
    .map((image) => ({
      name: first_non_empty(image.name, "未命名图片"),
      url: image.url,
    }));
}

function PreviewViewModel(props) {
  const task_ = ref(null);
  const loading_ = ref(false);
  const error_ = ref("");
  const active_file_ = ref(null);
  const zip_images_ = refarr([]);
  const zip_loading_ = ref(false);
  const zip_error_ = ref("");
  let detail_request_sequence = 0;
  let zip_request_sequence = 0;

  const detail_request = new Timeless.kit.RequestCore(
    (params) =>
      preview_detail_request.get("/api/v1/download_task/detail", params),
    { client: props.client },
  );
  const zip_request = new Timeless.kit.RequestCore(
    (params) => preview_file_request.get("/api/file", params),
    { client: props.client },
  );

  async function load() {
    const task_id = new URLSearchParams(window.location.search).get("id");
    if (!task_id) {
      error_.as("Missing task id");
      task_.as(null);
      return null;
    }

    const sequence = ++detail_request_sequence;
    loading_.as(true);
    error_.as("");
    const result = await detail_request.run({ id: task_id });
    if (sequence !== detail_request_sequence) {
      return result;
    }
    loading_.as(false);
    if (result.error) {
      error_.as(error_message(result.error, "获取下载任务详情失败"));
      task_.as(null);
      return result;
    }

    const task = normalize_task(result.data);
    task_.as(task);
    props.app.setTitle(task.name || "Preview");
    return result;
  }

  async function load_zip_preview(file) {
    const sequence = ++zip_request_sequence;
    zip_loading_.as(true);
    zip_error_.as("");
    zip_images_.as([], { reset: true });
    const result = await zip_request.run({
      preview: 1,
      path: file.local_path,
    });
    if (sequence !== zip_request_sequence || active_file_.value !== file) {
      return result;
    }
    zip_loading_.as(false);
    if (result.error) {
      zip_error_.as(error_message(result.error, "读取压缩包失败"));
      return result;
    }
    zip_images_.as(normalize_zip_images(result.data), { reset: true });
    return result;
  }

  const methods = {
    ready() {
      return load();
    },
    retry() {
      return load();
    },
    openPreview(file) {
      if (!file || !file.exists) {
        return;
      }
      active_file_.as(file);
      zip_images_.as([], { reset: true });
      zip_error_.as("");
      window.document.body.style.overflow = "hidden";
      if (file.file_type === "zip") {
        load_zip_preview(file);
      }
    },
    closePreview() {
      zip_request_sequence += 1;
      active_file_.as(null);
      zip_loading_.as(false);
      zip_error_.as("");
      zip_images_.as([], { reset: true });
      window.document.body.style.overflow = "";
    },
    fileURL: file_url,
    fileTypeIcon: file_type_icon,
    formatBytes: format_bytes,
  };

  const state = {
    task: task_,
    loading: loading_,
    error: error_,
    active_file: active_file_,
    zip_images: zip_images_,
    zip_loading: zip_loading_,
    zip_error: zip_error_,
  };
  const ui = {};

  return { state, ui, methods };
}

export { PreviewViewModel };

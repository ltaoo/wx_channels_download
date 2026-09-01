function prop_value(value) {
  if (value && typeof value === "object" && "value" in value) {
    return value.value;
  }
  return value;
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

const text_preview_chunk_bytes = 64 * 1024;
const text_preview_scroll_threshold = 320;

function normalized_content_type(value) {
  return String(prop_value(value) || "")
    .split(";", 1)[0]
    .trim()
    .toLowerCase();
}

function is_text_file(file) {
  if (!file) {
    return false;
  }
  return [
    file.kind,
    file.type,
    file.mime_type,
    file.mimeType,
  ].some((value) => {
    const content_type = normalized_content_type(value);
    return (
      content_type === "text" ||
      content_type.startsWith("text/") ||
      [
        "application/json",
        "application/javascript",
        "application/x-javascript",
        "application/xml",
        "application/yaml",
        "application/x-yaml",
      ].includes(content_type)
    );
  });
}

function platform_favicon(platform_id) {
  const key = String(platform_id || "")
    .trim()
    .toLowerCase();
  return window.PLATFORM_FAVICONS[key] || "";
}

function platform_name(platform_id) {
  const key = String(platform_id || "")
    .trim()
    .toLowerCase();
  return window.PLATFORM_NAMES[key] || platform_id || "";
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
  return window.TYPE_ICONS[file_type] || window.TYPE_ICONS.other;
}

function file_type_label(file_type) {
  return window.TYPE_LABELS[file_type] || window.TYPE_LABELS.other;
}

function file_url(file) {
  if (!file) {
    return "";
  }
  if (file.file_url) {
    return new URL(file.file_url, window.API_ORIGIN).href;
  }
  return `/api/file?path=${encodeURIComponent(file.local_path || "")}`;
}

function playback_url(file) {
  if (!file || !file.playback_url) {
    return "";
  }
  return new URL(file.playback_url, window.API_ORIGIN).href;
}

function should_use_stream_playback(file) {
  return Boolean(
    file &&
      playback_url(file) &&
      (!file.exists || file.playback_available),
  );
}

function live_playback_message(state, file) {
  const paused = file && file.status === "paused";
  switch (state.reason) {
    case "waiting-source":
      return paused
        ? "尚无已完成分片；恢复录制后会自动开始播放。"
        : "正在等待首个直播分片…";
    case "source-missing":
      return "录制已结束，但没有生成可播放的直播分片。";
    case "source-loading":
      return "正在载入已录制内容…";
    case "source-ready":
      return paused ? "已载入已有录制内容" : "已载入已录制内容";
    case "playing":
      return paused ? "正在播放已录制内容" : "边录边播";
    case "buffering":
      return paused
        ? "录制已中断；已播放到现有内容末尾。"
        : "正在缓冲直播分片…";
    case "network-retry":
      return "播放连接中断，正在重试…";
    case "media-recovery":
      return "媒体解码异常，正在恢复…";
    case "unsupported":
      return "当前宿主不支持 HLS 直播回看。";
    case "invalid-target":
      return "播放器宿主不可用。";
    case "invalid-source":
      return "缺少直播播放地址。";
    case "playback-error":
      return "无法播放当前直播编码。";
    default:
      return "";
  }
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
    file_url: first_non_empty(source.file_url, source.fileUrl, source.FileURL),
    kind: first_non_empty(source.kind, source.Kind),
    type: first_non_empty(source.type, source.Type),
    mime_type: first_non_empty(
      source.mime_type,
      source.mimeType,
      source.MIMEType,
    ),
    playback_url: first_non_empty(
      source.playback_url,
      source.playbackUrl,
      source.PlaybackURL,
    ),
    playback_available: Boolean(
      typeof source.playback_available !== "undefined"
        ? source.playback_available
        : source.playbackAvailable || source.PlaybackAvailable,
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
      Math.max(0, number_or_default(source.progress || source.Progress, 0)),
    ),
    exists: Boolean(
      typeof source.exists !== "undefined" ? source.exists : source.Exists,
    ),
  };
}

function normalize_task(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const content =
    source.content && typeof source.content === "object" ? source.content : {};
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
      url: new URL(image.url, window.API_ORIGIN).href,
    }));
}

function PreviewViewModel(props) {
  const platform = Timeless.getPlatform();
  const hls_player$ = props.hlsPlayer;
  const task_ = ref(null);
  const loading_ = ref(false);
  const error_ = ref("");
  const active_file_ = ref(null);
  const gallery_file_ = ref(null);
  const zip_images_ = refarr([]);
  const zip_loading_ = ref(false);
  const zip_error_ = ref("");
  const text_lines_ = refarr([]);
  const text_loading_ = ref(false);
  const text_error_ = ref("");
  const text_has_more_ = ref(false);
  const text_scroll_top_ = ref(0);
  const text_viewport_height_ = ref(1);
  const live_playback_status_ = ref("idle");
  const live_playback_message_ = ref("");
  let detail_request_sequence = 0;
  let zip_request_sequence = 0;
  let detail_task_id = "";
  let live_file = null;
  let text_reader_file = null;
  let text_reader_sequence = 0;
  let text_reader_offset = 0;
  let text_reader_line_number = 1;
  let text_reader_remainder = "";
  let text_reader_decoder = null;
  let text_reader_total = 0;
  let text_reader_abort_controller = null;

  function handle_text_scroll(value) {
    const target = event_target_element(value);
    const scroll_top = Number(value?.scrollTop ?? target?.scrollTop) || 0;
    const client_height = Number(value?.clientHeight ?? target?.clientHeight) || 0;
    const scroll_height = Number(value?.scrollHeight ?? target?.scrollHeight) || 0;
    text_scroll_top_.as(scroll_top);
    if (client_height > 0) {
      text_viewport_height_.as(client_height);
    }
    if (
      text_reader_file &&
      text_has_more_.value &&
      scroll_height > 0 &&
      scroll_top + client_height >=
        scroll_height - text_preview_scroll_threshold
    ) {
      load_text_more(text_reader_file);
    }
  }

  const text_scroll_view$ = new Timeless.vm.ScrollViewCore({
    onScroll: handle_text_scroll,
  });

  const unlisten_hls_player = hls_player$.onStateChange((state) => {
    live_playback_status_.as(state.status);
    live_playback_message_.as(live_playback_message(state, live_file));
  });

  function is_live_playback(file) {
    return Boolean(file && playback_url(file) && !file.exists);
  }

  function file_playable(file) {
    return Boolean(file && (file.exists || playback_url(file)));
  }

  function destroy_live_player() {
    hls_player$.unmount();
    live_file = null;
  }

  function mount_video(target, file, options) {
    if (!should_use_stream_playback(file)) {
      destroy_live_player();
      return null;
    }
    live_file = file;
    return hls_player$.mount(target, {
      url: playback_url(file),
      autoplay: Boolean(options && options.autoplay),
      terminal: ["error", "cancelled"].includes(file.status),
    });
  }

  function unmount_video(session) {
    if (!hls_player$.unmount(session)) return false;
    live_file = null;
    return true;
  }

  function event_target_element(event) {
    let target = event && event.target ? event.target : event;
    for (let depth = 0; depth < 4; depth += 1) {
      if (
        target &&
        target.nodeType === 1 &&
        typeof target.addEventListener === "function"
      ) {
        return target;
      }
      if (target && typeof target.get$elm === "function") {
        target = target.get$elm();
        continue;
      }
      if (target && target.$elm) {
        target = target.$elm;
        continue;
      }
      break;
    }
    return null;
  }

  function reset_text_reader(file) {
    text_reader_sequence += 1;
    if (text_reader_abort_controller) {
      text_reader_abort_controller.abort();
      text_reader_abort_controller = null;
    }
    text_reader_file = file;
    text_reader_offset = 0;
    text_reader_line_number = 1;
    text_reader_remainder = "";
    text_reader_decoder = new TextDecoder("utf-8");
    text_reader_total = Math.max(0, Number(file && file.size) || 0);
    text_scroll_top_.as(0);
    text_viewport_height_.as(1);
    text_lines_.as([], { reset: true });
    text_loading_.as(false);
    text_error_.as("");
    text_has_more_.as(Boolean(file && file_playable(file)));
  }

  function append_text_lines(text, is_final) {
    const pieces = (text_reader_remainder + text).split("\n");
    text_reader_remainder = is_final ? "" : pieces.pop() || "";
    if (is_final && pieces[pieces.length - 1] === "") {
      pieces.pop();
    }
    const lines = pieces.map((line) => ({
      number: text_reader_line_number++,
      text: line.endsWith("\r") ? line.slice(0, -1) : line,
    }));
    if (lines.length > 0) {
      text_lines_.as([...text_lines_.value, ...lines], { reset: true });
    }
  }

  async function read_response_bytes(response, maximum_bytes) {
    if (!response.body || typeof response.body.getReader !== "function") {
      return {
        bytes: new Uint8Array(await response.arrayBuffer()),
        ended: true,
      };
    }
    const reader = response.body.getReader();
    const chunks = [];
    let byte_count = 0;
    let ended = false;
    while (byte_count < maximum_bytes) {
      const result = await reader.read();
      if (result.done) {
        ended = true;
        break;
      }
      const bytes = result.value || new Uint8Array();
      const remaining = maximum_bytes - byte_count;
      const chunk = bytes.byteLength > remaining ? bytes.slice(0, remaining) : bytes;
      chunks.push(chunk);
      byte_count += chunk.byteLength;
      if (chunk.byteLength < bytes.byteLength) {
        await reader.cancel();
        break;
      }
    }
    const bytes = new Uint8Array(byte_count);
    let offset = 0;
    chunks.forEach((chunk) => {
      bytes.set(chunk, offset);
      offset += chunk.byteLength;
    });
    return { bytes, ended };
  }

  async function load_text_more(file) {
    if (
      !file ||
      text_reader_file !== file ||
      text_loading_.value ||
      !text_has_more_.value
    ) {
      return null;
    }
    const sequence = text_reader_sequence;
    const start = text_reader_offset;
    const end = start + text_preview_chunk_bytes - 1;
    const abort_controller = new AbortController();
    text_reader_abort_controller = abort_controller;
    text_loading_.as(true);
    text_error_.as("");
    try {
      const response = await window.fetch(file_url(file), {
        headers: {
          Accept: "text/plain, application/json, text/*;q=0.9, */*;q=0.1",
          Range: `bytes=${start}-${end}`,
        },
        signal: abort_controller.signal,
      });
      if (!response.ok) {
        throw new Error(`读取文本失败（HTTP ${response.status}）`);
      }
      if (response.status !== 206 && start > 0) {
        throw new Error("文件服务不支持文本分段读取");
      }
      const content_range = String(response.headers.get("content-range") || "").match(
        /^bytes\s+(\d+)-(\d+)\/(\d+|\*)$/i,
      );
      if (content_range && Number(content_range[1]) !== start) {
        throw new Error("文本分段位置异常");
      }
      const response_total = content_range && content_range[3] !== "*"
        ? Number(content_range[3])
        : 0;
      if (response.status !== 206 && response_total > text_preview_chunk_bytes) {
        throw new Error("文件服务不支持文本分段读取");
      }
      if (response_total > 0) {
        text_reader_total = response_total;
      }
      const result = await read_response_bytes(response, text_preview_chunk_bytes);
      if (sequence !== text_reader_sequence || text_reader_file !== file) {
        return null;
      }
      text_reader_offset = start + result.bytes.byteLength;
      const is_final =
        result.ended ||
        (text_reader_total > 0 && text_reader_offset >= text_reader_total);
      append_text_lines(text_reader_decoder.decode(result.bytes, { stream: !is_final }), false);
      if (is_final) {
        append_text_lines(text_reader_decoder.decode(), true);
      }
      text_has_more_.as(!is_final);
      return result;
    } catch (error) {
      if (error && error.name === "AbortError") {
        return null;
      }
      if (sequence === text_reader_sequence && text_reader_file === file) {
        text_error_.as(error_message(error, "读取文本失败"));
        text_has_more_.as(false);
      }
      return null;
    } finally {
      if (sequence === text_reader_sequence) {
        text_loading_.as(false);
        text_reader_abort_controller = null;
      }
    }
  }

  function unmount_text_reader() {
    text_reader_sequence += 1;
    if (text_reader_abort_controller) {
      text_reader_abort_controller.abort();
      text_reader_abort_controller = null;
    }
    text_reader_file = null;
    text_loading_.as(false);
  }

  function mount_text_reader(event, file) {
    const target = event_target_element(event);
    if (!target) {
      return;
    }
    unmount_text_reader();
    reset_text_reader(file);
    handle_text_scroll(target);
    load_text_more(file);
  }

  const detail_request = new Timeless.kit.RequestCore(
    (params) => window.request.get("/api/v1/download_task/detail", params),
    { client: props.client },
  );
  const zip_request = new Timeless.kit.RequestCore(
    (params) => window.request.get("/api/file", params),
    { client: props.client },
  );

  async function load(requested_task_id) {
    const task_id = String(
      prop_value(requested_task_id) ||
        prop_value(props.taskId) ||
        (props.view && props.view.query && props.view.query.id) ||
        "",
    ).trim();
    if (!task_id) {
      unmount_text_reader();
      loading_.as(false);
      error_.as("Missing task id");
      task_.as(null);
      gallery_file_.as(null);
      return null;
    }
    if (task_id === detail_task_id && loading_.value) {
      return null;
    }
    const task_changed = task_id !== detail_task_id;
    detail_task_id = task_id;
    if (task_changed) {
      unmount_text_reader();
      destroy_live_player();
      task_.as(null);
      gallery_file_.as(null);
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
      gallery_file_.as(null);
      return result;
    }

    const task = normalize_task(result.data);
    task_.as(task);
    gallery_file_.as(task.files.find(file_playable) || task.files[0] || null);
    if (!props.embedded && props.app) {
      props.app.setTitle(task.name || "Preview");
    }
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
    loadTask(task_id) {
      methods.closePreview();
      return load(task_id);
    },
    selectGalleryFile(file) {
      if (!file_playable(file)) {
        return;
      }
      destroy_live_player();
      unmount_text_reader();
      gallery_file_.as(file);
    },
    showFile(file) {
      return window.dl$.requests.file.show.run({
        path: file.local_path,
        name: file.name,
      });
    },
    openPreview(file) {
      if (!file_playable(file)) {
        return;
      }
      active_file_.as(file);
      unmount_text_reader();
      zip_images_.as([], { reset: true });
      zip_error_.as("");
      platform.patchBodyStyle({ overflow: "hidden" });
      if (file.file_type === "zip") {
        load_zip_preview(file);
      }
    },
    closePreview() {
      unmount_text_reader();
      zip_request_sequence += 1;
      active_file_.as(null);
      zip_loading_.as(false);
      zip_error_.as("");
      zip_images_.as([], { reset: true });
      platform.patchBodyStyle({ overflow: "" });
    },
    destroy() {
      methods.closePreview();
      destroy_live_player();
      unlisten_hls_player();
      text_scroll_view$.destroy?.();
      detail_request_sequence += 1;
      if (typeof detail_request.cancel === "function") detail_request.cancel();
      if (typeof live_playback_status_.destroy === "function") {
        live_playback_status_.destroy();
      }
      if (typeof live_playback_message_.destroy === "function") {
        live_playback_message_.destroy();
      }
    },
    fileURL: file_url,
    playbackURL: playback_url,
    videoSource(file) {
      return should_use_stream_playback(file) ? "" : file_url(file);
    },
    isLivePlayback: is_live_playback,
    filePlayable: file_playable,
    mountVideo: mount_video,
    unmountVideo: unmount_video,
    fileTypeIcon: file_type_icon,
    fileTypeLabel: file_type_label,
    formatBytes: format_bytes,
    isTextFile: is_text_file,
    textScrollView() {
      return text_scroll_view$;
    },
    mountTextReader: mount_text_reader,
    unmountTextReader: unmount_text_reader,
    loadTextMore: load_text_more,
    retryText() {
      if (!text_reader_file) {
        return null;
      }
      text_error_.as("");
      text_has_more_.as(true);
      return load_text_more(text_reader_file);
    },
  };

  const state = {
    task: task_,
    loading: loading_,
    error: error_,
    active_file: active_file_,
    gallery_file: gallery_file_,
    zip_images: zip_images_,
    zip_loading: zip_loading_,
    zip_error: zip_error_,
    text_lines: text_lines_,
    text_loading: text_loading_,
    text_error: text_error_,
    text_has_more: text_has_more_,
    text_scroll_top: text_scroll_top_,
    text_viewport_height: text_viewport_height_,
    live_playback_status: live_playback_status_,
    live_playback_message: live_playback_message_,
  };
  const ui = {};

  return { state, ui, methods };
}

export {
  PreviewViewModel,
  normalize_file,
  is_text_file,
  playback_url,
  should_use_stream_playback,
};

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

function mounted_media_element(event) {
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
  const task_ = ref(null);
  const loading_ = ref(false);
  const error_ = ref("");
  const active_file_ = ref(null);
  const gallery_file_ = ref(null);
  const zip_images_ = refarr([]);
  const zip_loading_ = ref(false);
  const zip_error_ = ref("");
  const live_playback_status_ = ref("idle");
  const live_playback_message_ = ref("");
  let detail_request_sequence = 0;
  let zip_request_sequence = 0;
  let detail_task_id = "";
  let live_mount_sequence = 0;
  let live_player = null;
  let live_video = null;
  let live_file_id = null;
  let live_poll_timer = null;
  let live_poll_controller = null;
  let live_video_cleanup = null;

  function is_live_playback(file) {
    return Boolean(file && playback_url(file) && !file.exists);
  }

  function file_playable(file) {
    return Boolean(file && (file.exists || playback_url(file)));
  }

  function clear_live_poll() {
    if (live_poll_timer !== null) {
      window.clearTimeout(live_poll_timer);
      live_poll_timer = null;
    }
    if (live_poll_controller) {
      live_poll_controller.abort();
      live_poll_controller = null;
    }
  }

  function destroy_live_player(reset_status = true) {
    live_mount_sequence += 1;
    clear_live_poll();
    if (typeof live_video_cleanup === "function") {
      live_video_cleanup();
      live_video_cleanup = null;
    }
    if (live_player && typeof live_player.destroy === "function") {
      live_player.destroy();
    }
    live_player = null;
    if (live_video && live_video.dataset.livePlaybackUrl) {
      live_video.removeAttribute("src");
      live_video.load();
    }
    live_video = null;
    live_file_id = null;
    if (reset_status) {
      live_playback_status_.as("idle");
      live_playback_message_.as("");
    }
  }

  function set_live_playback_state(status, message) {
    live_playback_status_.as(status);
    live_playback_message_.as(message || "");
  }

  function schedule_live_playlist_probe(sequence, file, options) {
    if (sequence !== live_mount_sequence || !live_video) {
      return;
    }
    live_poll_timer = window.setTimeout(() => {
      live_poll_timer = null;
      probe_live_playlist(sequence, file, options);
    }, 1000);
  }

  async function probe_live_playlist(sequence, file, options) {
    if (sequence !== live_mount_sequence || !live_video) {
      return;
    }
    const url = playback_url(file);
    const poll_controller = new AbortController();
    live_poll_controller = poll_controller;
    let response = null;
    try {
      response = await window.fetch(url, {
        method: "HEAD",
        cache: "no-store",
        credentials: "same-origin",
        signal: poll_controller.signal,
      });
    } catch (error) {
      if (error && error.name === "AbortError") {
        return;
      }
    } finally {
      if (live_poll_controller === poll_controller) {
        live_poll_controller = null;
      }
    }
    if (sequence !== live_mount_sequence || !live_video) {
      return;
    }
    if (response && response.ok) {
      start_live_player(sequence, file, options);
      return;
    }
    const terminal_status = ["error", "cancelled"].includes(file.status);
    set_live_playback_state(
      terminal_status ? "error" : "waiting",
      terminal_status
        ? "录制已结束，但没有生成可播放的直播分片。"
        : file.status === "paused"
          ? "尚无已完成分片；恢复录制后会自动开始播放。"
          : "正在等待首个直播分片…",
    );
    if (!terminal_status) {
      schedule_live_playlist_probe(sequence, file, options);
    }
  }

  function start_live_player(sequence, file, options) {
    if (sequence !== live_mount_sequence || !live_video) {
      return;
    }
    const video = live_video;
    const url = playback_url(file);
    const autoplay = Boolean(options && options.autoplay);
    video.dataset.livePlaybackUrl = url;
    set_live_playback_state("loading", "正在载入已录制内容…");

    const handle_playing = () => {
      if (sequence === live_mount_sequence) {
        set_live_playback_state(
          "playing",
          file.status === "paused" ? "正在播放已录制内容" : "边录边播",
        );
      }
    };
    const handle_waiting = () => {
      if (sequence === live_mount_sequence) {
        set_live_playback_state(
          "loading",
          file.status === "paused"
            ? "录制已中断；已播放到现有内容末尾。"
            : "正在缓冲直播分片…",
        );
      }
    };
    video.addEventListener("playing", handle_playing);
    video.addEventListener("waiting", handle_waiting);
    live_video_cleanup = () => {
      video.removeEventListener("playing", handle_playing);
      video.removeEventListener("waiting", handle_waiting);
    };

    const Hls = window.Hls;
    const hls_supported = Boolean(
      Hls && typeof Hls.isSupported === "function" && Hls.isSupported(),
    );
    if (
      !hls_supported &&
      video.canPlayType("application/vnd.apple.mpegurl")
    ) {
      const handle_native_loaded_metadata = () => {
        if (sequence !== live_mount_sequence || video.seekable.length === 0) {
          return;
        }
        try {
          video.currentTime = video.seekable.start(0);
        } catch {
          // Safari may update its seekable window between the length and start
          // calls. Playback can still proceed from its native default position.
        }
      };
      const handle_native_error = () => {
        if (sequence === live_mount_sequence) {
          set_live_playback_state("error", "无法播放当前直播编码。");
        }
      };
      const remove_common_listeners = live_video_cleanup;
      video.addEventListener("loadedmetadata", handle_native_loaded_metadata);
      video.addEventListener("error", handle_native_error);
      live_video_cleanup = () => {
        remove_common_listeners();
        video.removeEventListener(
          "loadedmetadata",
          handle_native_loaded_metadata,
        );
        video.removeEventListener("error", handle_native_error);
      };
      video.src = url;
      video.load();
      if (autoplay) {
        video.play().catch(() => {});
      }
      return;
    }

    if (!hls_supported) {
      set_live_playback_state("error", "当前浏览器不支持 HLS/MSE 直播回看。");
      return;
    }

    const player = new Hls({
      startPosition: 0,
      liveDurationInfinity: true,
      maxBufferLength: 60,
      backBufferLength: 600,
    });
    live_player = player;
    player.on(Hls.Events.MANIFEST_PARSED, () => {
      if (sequence !== live_mount_sequence) {
        return;
      }
      set_live_playback_state(
        "ready",
        file.status === "paused" ? "已载入已有录制内容" : "已载入已录制内容",
      );
      if (autoplay) {
        video.play().catch(() => {});
      }
    });
    player.on(Hls.Events.ERROR, (_event, data) => {
      if (sequence !== live_mount_sequence || !data || !data.fatal) {
        return;
      }
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
        set_live_playback_state("loading", "播放连接中断，正在重试…");
        player.startLoad();
        return;
      }
      if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
        set_live_playback_state("loading", "媒体解码异常，正在恢复…");
        player.recoverMediaError();
        return;
      }
      set_live_playback_state("error", "无法播放当前直播编码。");
      player.destroy();
      if (live_player === player) {
        live_player = null;
      }
    });
    player.loadSource(url);
    player.attachMedia(video);
  }

  function mount_video(event, file, options) {
    const video = mounted_media_element(event);
    if (!video) {
      return;
    }
    destroy_live_player(false);
    if (!should_use_stream_playback(file)) {
      set_live_playback_state("idle", "");
      return;
    }
    live_video = video;
    live_file_id = file.id;
    const sequence = live_mount_sequence;
    set_live_playback_state("waiting", "正在等待首个直播分片…");
    probe_live_playlist(sequence, file, options || {});
  }

  function unmount_video(file) {
    if (!file || live_file_id === file.id) {
      destroy_live_player();
    }
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
        new URLSearchParams(window.location.search).get("id") ||
        "",
    ).trim();
    if (!task_id) {
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
      gallery_file_.as(file);
    },
    openPreview(file) {
      if (!file_playable(file)) {
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
    destroy() {
      methods.closePreview();
      destroy_live_player();
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
    live_playback_status: live_playback_status_,
    live_playback_message: live_playback_message_,
  };
  const ui = {};

  return { state, ui, methods };
}

export {
  PreviewViewModel,
  normalize_file,
  playback_url,
  should_use_stream_playback,
};

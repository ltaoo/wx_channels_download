const THIRD_PARTY_DOWNLOADER_STORAGE_KEY =
  "wx_channels_download.third_party_downloader.v1";

export const THIRD_PARTY_DOWNLOADER_OPTIONS = Object.freeze([
  {
    value: "aria2",
    label: "aria2",
    endpoint: "http://127.0.0.1:6800/jsonrpc",
    description: "连接 aria2 RPC，也兼容 AriaNg 管理的 aria2 服务。",
    protocols: "支持 HTTP(S)、FTP、SFTP、BitTorrent 和 Magnet 地址。",
    setup: "请先以 --enable-rpc 启动 aria2，并保持 RPC 仅监听本机。",
    tokenLabel: "RPC Secret",
    tokenPlaceholder: "未设置密钥时可留空",
    icon: "globe",
  },
  {
    value: "motrix",
    label: "Motrix",
    endpoint: "http://127.0.0.1:16800/jsonrpc",
    description: "通过 Motrix 内置的 aria2 JSON-RPC 创建任务。",
    protocols: "支持 HTTP(S)、FTP、SFTP、BitTorrent 和 Magnet 地址。",
    setup: "请先启动 Motrix；RPC 地址和密钥可在偏好设置中查看。",
    tokenLabel: "RPC Secret",
    tokenPlaceholder: "填写 Motrix 偏好设置中的 RPC 密钥",
    icon: "hard-drive",
  },
  {
    value: "gopeed",
    label: "Gopeed",
    endpoint: "http://127.0.0.1:9999",
    description: "调用 Gopeed 本机 REST API 创建 HTTP、BT 或磁力任务。",
    protocols: "支持 HTTP(S)、BitTorrent、Magnet 和 ED2K 地址。",
    setup: "请在 Gopeed 高级设置中将通信协议设为 TCP，并重启 Gopeed。",
    tokenLabel: "API Token",
    tokenPlaceholder: "未设置 API Token 时可留空",
    icon: "download",
  },
]);

function downloader_option(kind) {
  return (
    THIRD_PARTY_DOWNLOADER_OPTIONS.find((option) => option.value === kind) ||
    THIRD_PARTY_DOWNLOADER_OPTIONS[0]
  );
}

function default_profiles() {
  return Object.fromEntries(
    THIRD_PARTY_DOWNLOADER_OPTIONS.map((option) => [
      option.value,
      { endpoint: option.endpoint, token: "" },
    ]),
  );
}

function read_saved_settings(storage) {
  try {
    const raw_value = storage && storage.getItem(THIRD_PARTY_DOWNLOADER_STORAGE_KEY);
    const saved = raw_value ? JSON.parse(raw_value) : {};
    const profiles = default_profiles();
    Object.keys(profiles).forEach((kind) => {
      const profile = saved.profiles && saved.profiles[kind];
      if (!profile || typeof profile !== "object") return;
      profiles[kind] = {
        endpoint:
          String(profile.endpoint || "").trim() || profiles[kind].endpoint,
        token: String(profile.token || ""),
      };
    });
    return {
      kind: downloader_option(saved.kind).value,
      directory: String(saved.directory || ""),
      profiles,
    };
  } catch {
    return {
      kind: "aria2",
      directory: "",
      profiles: default_profiles(),
    };
  }
}

function extract_filename(value) {
  try {
    const parsed_url = new URL(String(value || ""));
    if (!["http:", "https:", "ftp:", "sftp:"].includes(parsed_url.protocol)) {
      return "";
    }
    const last_segment = parsed_url.pathname.split("/").filter(Boolean).pop();
    return last_segment ? decodeURIComponent(last_segment) : "";
  } catch {
    return "";
  }
}

function validate_download_url(value, kind) {
  const raw_url = String(value || "").trim();
  if (!raw_url) return "请输入下载地址";
  const lower_url = raw_url.toLowerCase();
  if (lower_url.startsWith("magnet:?")) return "";
  if (lower_url.startsWith("ed2k://")) {
    return kind === "gopeed" ? "" : "ED2K 地址请改用 Gopeed 下载";
  }
  try {
    const parsed_url = new URL(raw_url);
    const protocols = ["http:", "https:", "ftp:", "sftp:", "magnet:", "ed2k:"];
    if (!protocols.includes(parsed_url.protocol.toLowerCase())) {
      return `暂不支持 ${parsed_url.protocol} 协议`;
    }
  } catch {
    return "下载地址格式不正确";
  }
  return "";
}

function format_download_bytes(value) {
  const bytes = Math.max(0, Number(value) || 0);
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let size = bytes / 1024;
  let unit_index = 0;
  while (size >= 1024 && unit_index < units.length - 1) {
    size /= 1024;
    unit_index += 1;
  }
  return `${size >= 100 ? size.toFixed(0) : size.toFixed(1)} ${units[unit_index]}`;
}

function download_status_text(status) {
  const status_names = {
    waiting: "等待下载",
    downloading: "正在下载",
    paused: "已暂停",
    completed: "下载完成",
    failed: "下载失败",
  };
  return status_names[String(status || "")] || "正在获取进度";
}

function normalized_progress(data, previous = {}) {
  const source = data && typeof data === "object" ? data : {};
  const completed = Math.max(0, Number(source.completed) || 0);
  const total = Math.max(0, Number(source.total) || 0);
  const speed = Math.max(0, Number(source.speed) || 0);
  const calculated_percent = total > 0 ? (completed * 100) / total : 0;
  const percent = Math.min(
    100,
    Math.max(0, Number(source.percent) || calculated_percent),
  );
  const status = String(source.status || previous.status || "waiting");
  return {
    visible: true,
    task_id: String(source.task_id || previous.task_id || ""),
    status,
    status_text: download_status_text(status),
    completed,
    total,
    speed,
    percent,
    percent_text: `${Math.round(percent)}%`,
    bytes_text: total
      ? `${format_download_bytes(completed)} / ${format_download_bytes(total)}`
      : `${format_download_bytes(completed)} / 大小未知`,
    speed_text: speed ? `${format_download_bytes(speed)}/s` : "--",
    file_path: String(source.file_path || previous.file_path || ""),
    error_message: String(source.error_message || ""),
    decryption_status: String(previous.decryption_status || ""),
    decryption_text: String(previous.decryption_text || ""),
  };
}

function empty_progress() {
  return {
    visible: false,
    task_id: "",
    status: "",
    status_text: "",
    completed: 0,
    total: 0,
    speed: 0,
    percent: 0,
    percent_text: "0%",
    bytes_text: "0 B / 大小未知",
    speed_text: "--",
    file_path: "",
    error_message: "",
    decryption_status: "",
    decryption_text: "",
  };
}

function show_downloader_success(message) {
  if (window.DLUtils && typeof DLUtils.toast === "function") {
    DLUtils.toast(message);
  }
}

export function ThirdPartyDownloaderModel(props = {}) {
  const storage = props.storage || window.localStorage;
  const saved = read_saved_settings(storage);
  const profiles = saved.profiles;
  const initial_profile = profiles[saved.kind];

  const kind_ = ref(saved.kind);
  const endpoint_ = ref(initial_profile.endpoint);
  const token_ = ref(initial_profile.token);
  const url_ = ref("");
  const filename_ = ref("");
  const directory_ = ref(saved.directory);
  const referer_ = ref("");
  const cookie_ = ref("");
  const user_agent_ = ref("");
  const advanced_open_ = ref(false);
  const checking_ = ref(false);
  const submitting_ = ref(false);
  const status_checking_ = ref(false);
  const tracking_ = ref(false);
  const decrypting_ = ref(false);
  const progress_ = refobj(empty_progress());
  const notice_ = refobj({
    tone: "neutral",
    title: "尚未检测连接",
    message: "选择下载器并检测连接后，即可发送下载任务。",
  });
  const last_result_ = ref(null);
  let resource_context = { decode_key: "", requires_decryption: false };
  let active_task_context = null;
  let poll_timeout_id = 0;
  let input_sync_timeout_id = 0;
  let tracking_sequence = 0;
  const decrypted_task_ids = new Set();
  const option_ = computed(kind_, (kind) => downloader_option(kind));
  const submit_disabled_ = combine(
    { url: url_, checking: checking_, submitting: submitting_ },
    (state) =>
      !String(state.url || "").trim() || state.checking || state.submitting,
  );
  const check_disabled_ = combine(
    { endpoint: endpoint_, checking: checking_, submitting: submitting_ },
    (state) =>
      !String(state.endpoint || "").trim() || state.checking || state.submitting,
  );
  const refresh_disabled_ = combine(
    { result: last_result_, checking: status_checking_, decrypting: decrypting_ },
    (state) =>
      !state.result?.task_id || Boolean(state.checking) || Boolean(state.decrypting),
  );

  const methods = {};
  const ui = {
    advanced_button$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      size: "sm",
      onClick() {
        return methods.toggleAdvanced();
      },
    }),
    refresh_button$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      disabled: Boolean(refresh_disabled_.value),
      loading: Boolean(status_checking_.value),
      onClick() {
        return methods.refreshProgress();
      },
    }),
    probe_button$: new Timeless.vm.ButtonCore({
      variant: "outline",
      size: "sm",
      disabled: Boolean(check_disabled_.value),
      loading: Boolean(checking_.value),
      onClick() {
        return methods.probe();
      },
    }),
    submit_button$: new Timeless.vm.ButtonCore({
      variant: "primary",
      size: "sm",
      disabled: Boolean(submit_disabled_.value),
      loading: Boolean(submitting_.value),
      onClick() {
        return methods.submit();
      },
    }),
    dialog$: new Timeless.vm.DialogCore({
      closeable: true,
      destroyOnClose: false,
      footer: false,
    }),
    select_kind$: new Timeless.vm.SelectCore({
      defaultValue: saved.kind,
      placeholder: "选择下载器",
      position: "item-aligned",
      options: THIRD_PARTY_DOWNLOADER_OPTIONS.map(
        (option) =>
          new Timeless.vm.SelectItemCore({
            label: option.label,
            value: option.value,
          }),
      ),
      onChange(value) {
        methods.setKind(value);
      },
    }),
    input_endpoint$: createInputStore({
      value: endpoint_,
      defaultValue: endpoint_.value,
      placeholder: initial_profile.endpoint,
      type: "url",
      allowClear: true,
      onChange(value) {
        methods.setEndpoint(value);
      },
    }),
    input_token$: createInputStore({
      value: token_,
      defaultValue: token_.value,
      placeholder: downloader_option(saved.kind).tokenPlaceholder,
      type: "password",
      allowClear: true,
      onChange(value) {
        methods.setToken(value);
      },
    }),
    input_url$: createInputStore({
      value: url_,
      defaultValue: "",
      placeholder: "https://example.com/video.mp4 或 magnet:?xt=...",
      type: "url",
      allowClear: true,
      autoFocus: true,
      onChange(value) {
        methods.setURL(value);
      },
      onEnter() {
        return methods.submit();
      },
    }),
    input_filename$: createInputStore({
      value: filename_,
      defaultValue: "",
      placeholder: "自动识别，可手动修改",
      allowClear: true,
      onChange(value) {
        filename_.as(String(value || ""));
      },
    }),
    input_directory$: createInputStore({
      value: directory_,
      defaultValue: directory_.value,
      placeholder: "留空则使用下载器默认目录",
      allowClear: true,
      onChange(value) {
        directory_.as(String(value || ""));
        methods.persistSettings();
      },
    }),
    input_referer$: createInputStore({
      value: referer_,
      defaultValue: "",
      placeholder: "https://来源页面地址/",
      type: "url",
      allowClear: true,
      onChange(value) {
        referer_.as(String(value || ""));
      },
    }),
    input_cookie$: createInputStore({
      value: cookie_,
      defaultValue: "",
      placeholder: "name=value; session=...",
      allowClear: true,
      onChange(value) {
        cookie_.as(String(value || ""));
      },
    }),
    input_user_agent$: createInputStore({
      value: user_agent_,
      defaultValue: "",
      placeholder: "留空则使用下载器默认 User-Agent",
      allowClear: true,
      onChange(value) {
        user_agent_.as(String(value || ""));
      },
    }),
  };
  const ui_state_unlistens = [];

  function bind_ui_button_state(button, sources) {
    if (sources.disabled) {
      const disabled_unlisten = sources.disabled.subscribe({
        onChange(value) {
          if (value) {
            button.disable();
          } else {
            button.enable();
          }
        },
      });
      if (typeof disabled_unlisten === "function") {
        ui_state_unlistens.push(disabled_unlisten);
      }
    }
    if (sources.loading) {
      const loading_unlisten = sources.loading.subscribe({
        onChange(value) {
          button.setLoading(Boolean(value));
        },
      });
      if (typeof loading_unlisten === "function") {
        ui_state_unlistens.push(loading_unlisten);
      }
    }
  }

  bind_ui_button_state(ui.refresh_button$, {
    disabled: refresh_disabled_,
    loading: status_checking_,
  });
  bind_ui_button_state(ui.probe_button$, {
    disabled: check_disabled_,
    loading: checking_,
  });
  bind_ui_button_state(ui.submit_button$, {
    disabled: submit_disabled_,
    loading: submitting_,
  });

  const probe_request = new Timeless.kit.RequestCore(
    (body) =>
      window.request.post(
        "/api/v1/third_party_downloader/probe",
        body,
      ),
    { client: props.client },
  );
  const create_request = new Timeless.kit.RequestCore(
    (body) =>
      window.request.post(
        "/api/v1/third_party_downloader/create",
        body,
      ),
    { client: props.client },
  );
  const status_request = new Timeless.kit.RequestCore(
    (body) =>
      window.request.post(
        "/api/v1/third_party_downloader/status",
        body,
      ),
    { client: props.client },
  );
  const decrypt_request = new Timeless.kit.RequestCore(
    (body) => {
      const query = new URLSearchParams({
        filepath: String(body.file_path || ""),
        key: String(body.decode_key || ""),
      });
      return window.request.post(
        `/api/channels/decrypt?${query.toString()}`,
        {},
      );
    },
    { client: props.client },
  );

  function stop_tracking() {
    tracking_sequence += 1;
    tracking_.as(false);
    if (poll_timeout_id) {
      window.clearTimeout(poll_timeout_id);
      poll_timeout_id = 0;
    }
  }

  function schedule_input_sync() {
    if (input_sync_timeout_id) window.clearTimeout(input_sync_timeout_id);
    input_sync_timeout_id = window.setTimeout(() => {
      input_sync_timeout_id = 0;
      const input_values = [
        [ui.input_endpoint$, endpoint_.value],
        [ui.input_token$, token_.value],
        [ui.input_url$, url_.value],
        [ui.input_filename$, filename_.value],
        [ui.input_directory$, directory_.value],
        [ui.input_referer$, referer_.value],
        [ui.input_cookie$, cookie_.value],
        [ui.input_user_agent$, user_agent_.value],
      ];
      input_values.forEach(([input, value]) => {
        input.setValue(value, { silence: true });
      });
    }, 0);
  }

  function reset_task_tracking() {
    stop_tracking();
    active_task_context = null;
    last_result_.as(null);
    progress_.as(empty_progress());
  }

  function reset_notice() {
    reset_task_tracking();
    notice_.as({
      tone: "neutral",
      title: "连接配置已更改",
      message: "请重新检测连接，确认本机下载器可以访问。",
    });
  }

  function snapshot_profile() {
    profiles[kind_.value] = {
      endpoint: String(endpoint_.value || "").trim(),
      token: String(token_.value || ""),
    };
  }

  function persist_settings() {
    snapshot_profile();
    try {
      storage?.setItem(
        THIRD_PARTY_DOWNLOADER_STORAGE_KEY,
        JSON.stringify({
          kind: kind_.value,
          directory: String(directory_.value || ""),
          profiles,
        }),
      );
    } catch {
      // Local storage may be unavailable in private or restricted contexts.
    }
  }

  function set_kind(value) {
    const next_kind = downloader_option(value).value;
    if (next_kind === kind_.value) return;
    snapshot_profile();
    kind_.as(next_kind);
    const profile = profiles[next_kind] || {
      endpoint: downloader_option(next_kind).endpoint,
      token: "",
    };
    endpoint_.as(profile.endpoint);
    token_.as(profile.token);
    ui.input_endpoint$.setPlaceholder?.(downloader_option(next_kind).endpoint);
    ui.input_token$.setPlaceholder?.(
      downloader_option(next_kind).tokenPlaceholder,
    );
    reset_notice();
    persist_settings();
  }

  function set_endpoint(value) {
    endpoint_.as(String(value || ""));
    reset_notice();
    persist_settings();
  }

  function set_token(value) {
    token_.as(String(value || ""));
    reset_notice();
    persist_settings();
  }

  function set_url(value, options = {}) {
    const next_url = String(value || "");
    url_.as(next_url);
    if (!options.preserve_resource_context) {
      resource_context = { decode_key: "", requires_decryption: false };
    }
    if (!String(filename_.value || "").trim()) {
      const detected_filename = extract_filename(next_url);
      if (detected_filename) {
        filename_.as(detected_filename);
      }
    }
    if (last_result_.value) {
      reset_task_tracking();
      notice_.as({
        tone: "neutral",
        title: "准备发送新任务",
        message: "确认下载信息后，点击发送任务。",
      });
    }
  }

  function connection_payload() {
    return {
      kind: kind_.value,
      endpoint: String(endpoint_.value || "").trim(),
      token: String(token_.value || ""),
    };
  }

  async function probe() {
    if (checking_.value || submitting_.value) return null;
    if (!String(endpoint_.value || "").trim()) {
      notice_.as({
        tone: "danger",
        title: "缺少连接地址",
        message: "请输入本机下载器的 API 或 RPC 地址。",
      });
      return null;
    }
    checking_.as(true);
    notice_.as({
      tone: "progress",
      title: "正在检测连接",
      message: `正在连接 ${downloader_option(kind_.value).label}…`,
    });
    try {
      const response = await probe_request.run(connection_payload());
      if (response.error) {
        notice_.as({
          tone: "danger",
          title: "连接失败",
          message: response.error.message || String(response.error),
        });
        return response;
      }
      const data = response.data || {};
      notice_.as({
        tone: "success",
        title: `${data.name || downloader_option(kind_.value).label} 已连接`,
        message: data.version ? `版本 ${data.version}` : "本机下载器响应正常。",
      });
      persist_settings();
      return response;
    } catch (error) {
      notice_.as({
        tone: "danger",
        title: "连接失败",
        message: error.message || String(error),
      });
      return null;
    } finally {
      checking_.as(false);
    }
  }

  function progress_message(progress) {
    const parts = [progress.bytes_text];
    if (progress.speed > 0) parts.push(progress.speed_text);
    if (progress.task_id) parts.push(`任务 ${progress.task_id}`);
    return parts.join(" · ");
  }

  function schedule_progress_poll(sequence, delay = 1200) {
    if (!tracking_.value || sequence !== tracking_sequence) return;
    if (poll_timeout_id) window.clearTimeout(poll_timeout_id);
    poll_timeout_id = window.setTimeout(() => {
      poll_timeout_id = 0;
      refresh_progress({ automatic: true, sequence });
    }, delay);
  }

  async function decrypt_completed_download(progress, sequence) {
    const task_context = active_task_context;
    if (
      !task_context ||
      sequence !== tracking_sequence ||
      decrypted_task_ids.has(task_context.task_id)
    ) {
      return null;
    }
    if (!task_context.requires_decryption) {
      notice_.as({
        tone: "success",
        title: "下载完成",
        message: progress.file_path || "三方下载器已完成任务。",
      });
      show_downloader_success("三方下载已完成");
      return null;
    }
    if (!task_context.decode_key) {
      notice_.as({
        tone: "danger",
        title: "无法自动解密",
        message: "下载完成，但 DownloadResource 中缺少微信解密密钥。",
      });
      return null;
    }
    if (!progress.file_path) {
      notice_.as({
        tone: "danger",
        title: "无法自动解密",
        message:
          "下载完成，但下载器没有返回绝对文件路径；请在任务前填写绝对保存目录。",
      });
      progress_.as({
        ...progress_.value,
        decryption_status: "failed",
        decryption_text: "缺少下载文件的绝对路径",
      });
      return null;
    }

    decrypting_.as(true);
    progress_.as({
      ...progress_.value,
      decryption_status: "decrypting",
      decryption_text: "正在调用微信解密接口…",
    });
    notice_.as({
      tone: "progress",
      title: "下载完成，正在微信解密",
      message: progress.file_path,
    });
    try {
      const response = await decrypt_request.run({
        file_path: progress.file_path,
        decode_key: task_context.decode_key,
      });
      if (sequence !== tracking_sequence) return response;
      if (response.error) {
        progress_.as({
          ...progress_.value,
          decryption_status: "failed",
          decryption_text: response.error.message || String(response.error),
        });
        notice_.as({
          tone: "danger",
          title: "微信解密失败",
          message: response.error.message || String(response.error),
        });
        return response;
      }
      decrypted_task_ids.add(task_context.task_id);
      const decrypted_path = String(
        response.data?.filepath || progress.file_path,
      );
      progress_.as({
        ...progress_.value,
        file_path: decrypted_path,
        decryption_status: "completed",
        decryption_text: "微信解密已完成",
      });
      notice_.as({
        tone: "success",
        title: "下载并解密完成",
        message: decrypted_path,
      });
      show_downloader_success("三方下载及微信解密已完成");
      return response;
    } catch (error) {
      if (sequence !== tracking_sequence) return null;
      const message = error.message || String(error);
      progress_.as({
        ...progress_.value,
        decryption_status: "failed",
        decryption_text: message,
      });
      notice_.as({ tone: "danger", title: "微信解密失败", message });
      return null;
    } finally {
      decrypting_.as(false);
    }
  }

  async function refresh_progress(options = {}) {
    const task_context = active_task_context;
    if (!task_context || status_checking_.value || decrypting_.value) {
      return null;
    }
    const sequence = Number.isFinite(options.sequence)
      ? options.sequence
      : tracking_sequence;
    status_checking_.as(true);
    try {
      const response = await status_request.run({
        kind: task_context.kind,
        endpoint: task_context.endpoint,
        token: task_context.token,
        task_id: task_context.task_id,
        filename: task_context.filename,
        directory: task_context.directory,
      });
      if (sequence !== tracking_sequence) return response;
      if (response.error) {
        stop_tracking();
        notice_.as({
          tone: "danger",
          title: "获取下载进度失败",
          message: response.error.message || String(response.error),
        });
        return response;
      }
      const progress = normalized_progress(response.data, progress_.value);
      progress_.as(progress);
      if (progress.status === "completed") {
        tracking_.as(false);
        await decrypt_completed_download(progress, sequence);
        return response;
      }
      if (progress.status === "failed") {
        tracking_.as(false);
        notice_.as({
          tone: "danger",
          title: "三方下载失败",
          message: progress.error_message || progress_message(progress),
        });
        return response;
      }
      notice_.as({
        tone: "progress",
        title: progress.status_text,
        message: progress_message(progress),
      });
      if (tracking_.value) schedule_progress_poll(sequence);
      return response;
    } catch (error) {
      if (sequence !== tracking_sequence) return null;
      stop_tracking();
      notice_.as({
        tone: "danger",
        title: "获取下载进度失败",
        message: error.message || String(error),
      });
      return null;
    } finally {
      status_checking_.as(false);
    }
  }

  function start_tracking(data, task_context) {
    stop_tracking();
    active_task_context = {
      ...task_context,
      task_id: String(data.task_id || ""),
    };
    const sequence = tracking_sequence;
    tracking_.as(true);
    progress_.as(
      normalized_progress({
        task_id: active_task_context.task_id,
        status: "waiting",
      }),
    );
    notice_.as({
      tone: "progress",
      title: "任务已发送，正在获取进度",
      message: `${data.name || downloader_option(kind_.value).label} 任务 ID：${active_task_context.task_id}`,
    });
    schedule_progress_poll(sequence, 250);
  }

  async function submit() {
    if (checking_.value || submitting_.value) return null;
    const validation_error = validate_download_url(url_.value, kind_.value);
    if (validation_error) {
      notice_.as({
        tone: "danger",
        title: "无法发送任务",
        message: validation_error,
      });
      return null;
    }
    submitting_.as(true);
    reset_task_tracking();
    const task_context = {
      ...connection_payload(),
      filename: String(filename_.value || "").trim(),
      directory: String(directory_.value || "").trim(),
      decode_key: String(resource_context.decode_key || ""),
      requires_decryption: Boolean(resource_context.requires_decryption),
    };
    notice_.as({
      tone: "progress",
      title: "正在发送任务",
      message: `正在交给 ${downloader_option(kind_.value).label}…`,
    });
    try {
      const response = await create_request.run({
        kind: task_context.kind,
        endpoint: task_context.endpoint,
        token: task_context.token,
        url: String(url_.value || "").trim(),
        filename: task_context.filename,
        directory: task_context.directory,
        referer: String(referer_.value || "").trim(),
        cookie: String(cookie_.value || "").trim(),
        user_agent: String(user_agent_.value || "").trim(),
      });
      if (response.error) {
        notice_.as({
          tone: "danger",
          title: "发送失败",
          message: response.error.message || String(response.error),
        });
        return response;
      }
      const data = response.data || {};
      last_result_.as(data);
      if (data.task_id) {
        start_tracking(data, task_context);
      } else {
        notice_.as({
          tone: "success",
          title: "任务已发送",
          message: `已交给 ${data.name || downloader_option(kind_.value).label}。`,
        });
      }
      persist_settings();
      show_downloader_success("已发送到三方下载器");
      return response;
    } catch (error) {
      notice_.as({
        tone: "danger",
        title: "发送失败",
        message: error.message || String(error),
      });
      return null;
    } finally {
      submitting_.as(false);
    }
  }

  function open(initial = {}) {
    if (initial && initial.url !== undefined) {
      const initial_url = String(initial.url || "");
      set_url(initial_url, { preserve_resource_context: true });
      const initial_filename = String(initial.filename || "");
      filename_.as(initial_filename);
      const initial_referer = String(initial.referer || "");
      const initial_cookie = String(initial.cookie || "");
      const initial_user_agent = String(initial.user_agent || "");
      referer_.as(initial_referer);
      cookie_.as(initial_cookie);
      user_agent_.as(initial_user_agent);
      resource_context = {
        decode_key: String(initial.decode_key || "").trim(),
        requires_decryption: Boolean(
          initial.requires_decryption && initial.decode_key,
        ),
      };
      if (initial_referer || initial_cookie || initial_user_agent) {
        advanced_open_.as(true);
      }
    }
    ui.dialog$.show();
    schedule_input_sync();
  }

  function toggle_advanced() {
    advanced_open_.as(!advanced_open_.value);
  }

  function destroy() {
    stop_tracking();
    if (input_sync_timeout_id) {
      window.clearTimeout(input_sync_timeout_id);
      input_sync_timeout_id = 0;
    }
    probe_request.destroy?.();
    create_request.destroy?.();
    status_request.destroy?.();
    decrypt_request.destroy?.();
    while (ui_state_unlistens.length > 0) {
      ui_state_unlistens.pop()();
    }
    Object.values(ui).forEach((store) => store.destroy?.());
  }

  Object.assign(methods, {
    destroy,
    open,
    persistSettings: persist_settings,
    probe,
    refreshProgress: refresh_progress,
    setEndpoint: set_endpoint,
    setKind: set_kind,
    setToken: set_token,
    setURL: set_url,
    submit,
    toggleAdvanced: toggle_advanced,
  });

  return {
    state: {
      advanced_open: advanced_open_,
      check_disabled: check_disabled_,
      checking: checking_,
      decrypting: decrypting_,
      directory: directory_,
      endpoint: endpoint_,
      filename: filename_,
      kind: kind_,
      last_result: last_result_,
      notice: notice_,
      option: option_,
      progress: progress_,
      referer: referer_,
      refresh_disabled: refresh_disabled_,
      status_checking: status_checking_,
      submit_disabled: submit_disabled_,
      submitting: submitting_,
      tracking: tracking_,
      token: token_,
      url: url_,
      cookie: cookie_,
      user_agent: user_agent_,
    },
    ui,
    methods,
  };
}

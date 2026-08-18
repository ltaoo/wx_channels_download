const UPDATE_POLL_INTERVAL = 250;
const RESTART_POLL_INTERVAL = 600;
const RESTART_TIMEOUT = 60000;
const REQUEST_TIMEOUT = 20000;

function normalize_version(value) {
  return String(value || "")
    .trim()
    .replace(/^v/i, "");
}

function format_version_label(value) {
  const version = String(value || "").trim();
  if (!version) return "开发版";
  return /^v/i.test(version) ? version : `v${version}`;
}

function number_or_zero(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function format_bytes(value) {
  const bytes = Math.max(0, number_or_zero(value));
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

function format_published_at(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
}

function update_error_message(error, fallback) {
  if (error && error.name === "AbortError") return fallback;
  if (error && error.message) return error.message;
  return error ? String(error) : fallback;
}

export function createUpdateModel(options = {}) {
  const fetch_client = options.fetch || window.fetch.bind(window);
  const reload = options.reload || (() => window.location.reload());
  const snapshot_ = refobj({
    status: "idle",
    available: false,
    current_version: String(options.currentVersion || ""),
    latest_version: "",
    downloaded: 0,
    total_size: 0,
    speed: 0,
    percent: 0,
    error: "",
  });
  const checking_ = ref(false);
  const latest_version_ = ref("");
  const destroyed_ = ref(false);
  const controllers = new Set();
  let poll_timer = null;
  let restart_started_at = 0;
  let restart_requested = false;
  let observed_offline = false;

  const status_ = computed(snapshot_, (snapshot) => snapshot.status || "idle");
  const current_version_text_ = computed(snapshot_, (snapshot) =>
    format_version_label(snapshot.current_version),
  );
  const latest_version_text_ = computed(
    latest_version_,
    (latest_version) => latest_version || "未知",
  );
  const busy_ = computed(status_, (status) =>
    ["downloading", "ready", "restarting"].includes(status),
  );
  const downloading_ = computed(
    status_,
    (status) => status === "downloading",
  );
  const can_download_ = computed(snapshot_, (snapshot) => {
    return (
      Boolean(snapshot.available) &&
      ["available", "error"].includes(snapshot.status)
    );
  });
  const percent_ = computed(snapshot_, (snapshot) => {
    return Math.max(0, Math.min(100, number_or_zero(snapshot.percent)));
  });
  const has_total_ = computed(
    snapshot_,
    (snapshot) => number_or_zero(snapshot.total_size) > 0,
  );
  const has_latest_version_ = computed(
    snapshot_,
    (snapshot) => Boolean(snapshot.latest_version || snapshot.name),
  );
  const notice_visible_ = computed(
    snapshot_,
    (snapshot) => Boolean(snapshot.available),
  );
  const published_text_ = computed(snapshot_, (snapshot) =>
    format_published_at(snapshot.published_at),
  );
  const progress_text_ = computed(snapshot_, (snapshot) => {
    const downloaded = format_bytes(snapshot.downloaded);
    const total = number_or_zero(snapshot.total_size);
    const speed = number_or_zero(snapshot.speed);
    const size_text = total
      ? `${downloaded} / ${format_bytes(total)}`
      : downloaded;
    return speed > 0 ? `${size_text} · ${format_bytes(speed)}/s` : size_text;
  });
  const phase_title_ = computed(snapshot_, (snapshot) => {
    switch (snapshot.status) {
      case "downloading":
        return "正在下载更新";
      case "ready":
        return "下载完成";
      case "restarting":
        return "正在重启服务";
      case "error":
        return "更新失败";
      default:
        return "发现新版本";
    }
  });
  const phase_message_ = computed(snapshot_, (snapshot) => {
    switch (snapshot.status) {
      case "downloading":
        return "更新包下载完成后将自动替换程序并重启服务。";
      case "ready":
        return "更新已安装，正在准备重启。";
      case "restarting":
        return "服务恢复后页面会自动刷新，请勿关闭此页面。";
      case "error":
        return snapshot.error || "更新过程中发生未知错误。";
      default:
        return "是否下载并安装此版本？";
    }
  });

  const ui = {
    cancel_button$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      disabled: Boolean(busy_.value),
      onClick: dismiss,
    }),
    dialog$: new Timeless.vm.DialogCore({
      closeable: false,
      footer: false,
    }),
    download_button$: new Timeless.vm.ButtonCore({
      variant: "primary",
      disabled: !can_download_.value,
      loading: Boolean(downloading_.value),
      onClick: download,
    }),
    notice_button$: new Timeless.vm.ButtonCore({
      variant: "ghost",
      onClick: show,
    }),
  };
  const ui_state_unlistens = [
    busy_.subscribe({
      onChange(value) {
        if (value) {
          ui.cancel_button$.disable();
        } else {
          ui.cancel_button$.enable();
        }
      },
    }),
    can_download_.subscribe({
      onChange(value) {
        if (value) {
          ui.download_button$.enable();
        } else {
          ui.download_button$.disable();
        }
      },
    }),
    downloading_.subscribe({
      onChange(value) {
        ui.download_button$.setLoading(Boolean(value));
      },
    }),
  ];

  function clear_poll_timer() {
    if (poll_timer !== null) {
      window.clearTimeout(poll_timer);
      poll_timer = null;
    }
  }

  function schedule_poll(callback, delay) {
    clear_poll_timer();
    if (!destroyed_.value) {
      poll_timer = window.setTimeout(callback, delay);
    }
  }

  async function request(path, request_options = {}, timeout = REQUEST_TIMEOUT) {
    const controller = new AbortController();
    controllers.add(controller);
    const timeout_id = window.setTimeout(() => controller.abort(), timeout);
    try {
      const response = await fetch_client(path, {
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        ...request_options,
        signal: controller.signal,
      });
      if (!response.ok) {
        throw new Error(`请求失败：HTTP ${response.status}`);
      }
      const payload = await response.json();
      if (payload.code !== 0) {
        throw new Error(payload.msg || "更新请求失败");
      }
      return payload.data || {};
    } finally {
      window.clearTimeout(timeout_id);
      controllers.delete(controller);
    }
  }

  function apply_snapshot(snapshot) {
    const latest_version = String(
      (snapshot && (snapshot.latest_version || snapshot.name)) || "",
    ).trim();
    if (latest_version) {
      latest_version_.as(latest_version);
    }
    snapshot_.as({
      ...snapshot_.value,
      ...(snapshot || {}),
    });
  }

  function set_error(error, fallback) {
    apply_snapshot({
      status: "error",
      speed: 0,
      error: update_error_message(error, fallback),
    });
    ui.dialog$.show();
  }

  async function check() {
    if (destroyed_.value || checking_.value) return null;
    checking_.as(true);
    try {
      const snapshot = await request("/api/update/check");
      apply_snapshot(snapshot);
      return snapshot;
    } catch {
      // Automatic checks stay silent so a transient GitHub failure does not
      // interrupt normal use of the application.
      return null;
    } finally {
      checking_.as(false);
    }
  }

  async function download() {
    if (destroyed_.value || !can_download_.value) return null;
    ui.dialog$.show();
    restart_requested = false;
    try {
      const snapshot = await request("/api/update/download", {
        method: "POST",
        body: "{}",
      });
      apply_snapshot(snapshot);
      schedule_poll(poll_status, UPDATE_POLL_INTERVAL);
      return snapshot;
    } catch (error) {
      set_error(error, "启动更新下载失败");
      return null;
    }
  }

  async function poll_status() {
    if (destroyed_.value) return;
    try {
      const snapshot = await request("/api/update/status");
      apply_snapshot(snapshot);
      if (snapshot.status === "ready") {
        await restart();
        return;
      }
      if (snapshot.status === "error") {
        return;
      }
      schedule_poll(poll_status, UPDATE_POLL_INTERVAL);
    } catch (error) {
      set_error(error, "读取更新进度失败");
    }
  }

  async function restart() {
    if (destroyed_.value || restart_requested) return;
    restart_requested = true;
    try {
      const snapshot = await request("/api/update/restart", {
        method: "POST",
        body: "{}",
      });
      apply_snapshot(snapshot);
      restart_started_at = Date.now();
      observed_offline = false;
      schedule_poll(wait_for_restart, RESTART_POLL_INTERVAL);
    } catch (error) {
      restart_requested = false;
      set_error(error, "请求服务重启失败");
    }
  }

  async function wait_for_restart() {
    if (destroyed_.value) return;
    if (Date.now() - restart_started_at > RESTART_TIMEOUT) {
      set_error(null, "服务重启超时，请手动刷新页面");
      return;
    }
    try {
      const status = await request(
        `/api/status?restart_check=${Date.now()}`,
        {},
        3000,
      );
      const expected_version = normalize_version(latest_version_.value);
      const running_version = normalize_version(status.version);
      if (
        observed_offline ||
        (expected_version && running_version === expected_version)
      ) {
        reload();
        return;
      }
    } catch {
      observed_offline = true;
    }
    schedule_poll(wait_for_restart, RESTART_POLL_INTERVAL);
  }

  function dismiss() {
    if (!busy_.value) {
      ui.dialog$.hide();
    }
  }

  function show() {
    if (snapshot_.value.available) {
      ui.dialog$.show();
    }
  }

  function destroy() {
    if (destroyed_.value) return;
    destroyed_.as(true);
    clear_poll_timer();
    controllers.forEach((controller) => controller.abort());
    controllers.clear();
    ui_state_unlistens.forEach((unlisten) => unlisten?.());
    Object.values(ui).forEach((store) => store.destroy?.());
  }

  return {
    state: {
      busy: busy_,
      can_download: can_download_,
      checking: checking_,
      current_version: current_version_text_,
      downloading: downloading_,
      has_latest_version: has_latest_version_,
      has_total: has_total_,
      latest_version: latest_version_text_,
      notice_visible: notice_visible_,
      percent: percent_,
      phase_message: phase_message_,
      phase_title: phase_title_,
      progress_text: progress_text_,
      published_text: published_text_,
      snapshot: snapshot_,
      status: status_,
    },
    ui,
    methods: {
      check,
      destroy,
      dismiss,
      download,
      show,
    },
  };
}

const Timeless = window.Timeless;
if (!Timeless || !Timeless.DOM) {
  throw new Error("Bridge 管理页无法启动：Timeless 运行时未加载");
}
const {
  Button,
  For,
  Input,
  Select,
  Show,
  View,
  combine,
  computed,
  ref,
  refarr,
  refobj,
} = Timeless;
const { ui, vm } = Timeless;

function empty_metrics() {
  return {
    device_count: 0,
    online_count: 0,
    offline_count: 0,
    task_count: 0,
  };
}

function DashboardViewModel() {
  const loading_ = ref(false);
  const error_ = ref("");
  const devices_ = refarr([]);
  const download_devices_ = refarr([]);
  const task_counts_ = refarr([]);
  const available_methods_ = refarr([]);
  const metrics_ = refobj(empty_metrics());
  const generated_at_ = ref(0);
  const refreshed_at_ = ref(0);
  const access_tokens_ = refarr([]);
  const access_token_draft_ = refobj({
    name: "",
    token: "",
    expires_in_seconds: "604800",
    credits: "0",
  });
  const access_token_action_ = refobj({
    creating: false,
    busy_id: "",
    busy_action: "",
    created_token: "",
    created_token_name: "",
    copy_message: "",
    error: "",
  });
  const access_token_drawer$ = new vm.DialogCore({
    title: "调用 Token",
    closeable: true,
    footer: false,
  });
  const test_drafts_ = refobj({});
  const download_drafts_ = refobj({});
  const tests_ = refobj({});
  const device_sources = new Map();
  const access_token_sources = new Map();
  const test_poll_timers = new Map();
  const download_poll_timers = new Map();
  let refresh_promise = null;
  let refresh_timer = null;
  let started = false;

  function normalize_access_token(access_token) {
    return {
      id: String(access_token.id || ""),
      name: String(access_token.name || "未填写用途或使用人"),
      token_hint: String(access_token.token_hint || ""),
      status: access_token.status === "expired" ? "expired" : "active",
      expires_at: Number(access_token.expires_at || 0),
      created_at: Number(access_token.created_at || 0),
      last_used_at: Number(access_token.last_used_at || 0),
      credit_balance: Number(access_token.credit_balance || 0),
      total_credits_granted: Number(access_token.total_credits_granted || 0),
      total_credits_used: Number(access_token.total_credits_used || 0),
    };
  }

  function normalize_overview(overview) {
    const devices = overview.devices.map((device) => {
      const methods_source = Array.isArray(device.methods)
        ? device.methods
        : Array.isArray(device.capabilities)
          ? device.capabilities
          : [];
      const methods = methods_source.map(String);
      const status = ["online", "busy", "offline"].includes(device.status)
        ? device.status
        : "offline";
      return {
        device_id: String(device.device_id || ""),
        device_name: String(
          device.device_name || device.device_id || "未命名设备",
        ),
        device_os: String(device.device_os || "unknown"),
        methods,
        connected_at: Number(device.connected_at || 0),
        last_seen_at: Number(device.last_seen_at || 0),
        disconnected_at: Number(device.disconnected_at || 0),
        status,
        can_test:
          status !== "offline" && methods.includes("wxchannels.fetch"),
      };
    });
    const task_counts = Array.isArray(overview.task_counts)
      ? overview.task_counts
      : [];
    const methods_source = Array.isArray(overview.methods)
      ? overview.methods
      : Array.isArray(overview.capabilities)
        ? overview.capabilities
        : [];
    const methods = methods_source.map(String);
    const online_count = devices.filter(
      (device) => device.status === "online" || device.status === "busy",
    ).length;
    const offline_count = devices.filter(
      (device) => device.status === "offline",
    ).length;
    const task_count = task_counts.reduce(
      (total, item) => total + Number(item.count || 0),
      0,
    );
    const access_tokens = Array.isArray(overview.access_tokens)
      ? overview.access_tokens.map(normalize_access_token)
      : [];
    return Object.assign({}, overview, {
      devices,
      download_devices: devices.filter(
        (device) =>
          device.status !== "offline" &&
          device.methods.includes("download.create"),
      ),
      task_counts,
      methods,
      access_tokens,
      metrics: {
        device_count: devices.length,
        online_count,
        offline_count,
        task_count,
      },
    });
  }

  function set_access_token_draft(field, value) {
    access_token_draft_.as(
      Object.assign({}, access_token_draft_.value, { [field]: value }),
    );
  }

  function set_access_token_action(next_action) {
    access_token_action_.as(
      Object.assign({}, access_token_action_.value, next_action),
    );
  }

  function open_access_token_drawer() {
    access_token_drawer$.show();
  }

  async function create_access_token() {
    const draft = access_token_draft_.value;
    const name = String(draft.name || "").trim();
    const token = String(draft.token || "").trim();
    if (
      token !== "" &&
      (token.length < 16 ||
        token.length > 256 ||
        !/^[A-Za-z0-9._~+/=-]+$/.test(token))
    ) {
      set_access_token_action({
        error: "自定义 Token 必须为 16–256 位，只能包含字母、数字和 . _ ~ + / = -",
      });
      return;
    }
    const expires_value = String(draft.expires_in_seconds || "");
    const credits = Number(draft.credits || 0);
    if (!Number.isInteger(credits) || credits < 0 || credits > 1000000000) {
      set_access_token_action({
        error: "初始积分必须是 0–1000000000 之间的整数",
      });
      return;
    }
    set_access_token_action({
      creating: true,
      copy_message: "",
      error: "",
    });
    try {
      const response = await fetch("/admin/api/access-tokens", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name,
          token: token || null,
          expires_in_seconds: expires_value === "" ? null : Number(expires_value),
          credits,
        }),
      });
      const value = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(value.error || "创建调用 Token 失败：HTTP " + response.status);
      }
      if (!value.access_token || !value.token) {
        throw new Error("创建调用 Token 的返回格式不正确");
      }
      const normalized_access_token = normalize_access_token(value.access_token);
      sync_access_tokens([
        normalized_access_token,
        ...access_tokens_.value.filter(
          (access_token) => access_token.id !== normalized_access_token.id,
        ),
      ]);
      access_token_draft_.as({
        name: "",
        token: "",
        expires_in_seconds: expires_value,
        credits: String(credits),
      });
      set_access_token_action({
        creating: false,
        created_token: String(value.token),
        created_token_name: name || "未填写用途或使用人",
        copy_message: "",
        error: "",
      });
      refresh(false);
    } catch (error) {
      set_access_token_action({
        creating: false,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function expire_access_token(access_token_id) {
    const access_token = access_tokens_.value.find(
      (candidate) => candidate.id === access_token_id,
    );
    if (!access_token || access_token.status === "expired") return;
    if (!window.confirm("立即让调用 Token“" + access_token.name + "”过期？")) return;
    set_access_token_action({
      busy_id: access_token_id,
      busy_action: "expire",
      error: "",
    });
    try {
      const response = await fetch(
        "/admin/api/access-tokens/" + encodeURIComponent(access_token_id) + "/expire",
        {
          method: "POST",
          credentials: "same-origin",
          cache: "no-store",
        },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(value.error || "设置 Token 过期失败：HTTP " + response.status);
      }
      sync_access_tokens(
        access_tokens_.value.map((candidate) =>
          candidate.id === access_token_id
            ? normalize_access_token(value.access_token)
            : candidate,
        ),
      );
      set_access_token_action({ busy_id: "", busy_action: "", error: "" });
    } catch (error) {
      set_access_token_action({
        busy_id: "",
        busy_action: "",
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function grant_access_token_credits(access_token_id) {
    const access_token = access_tokens_.value.find(
      (candidate) => candidate.id === access_token_id,
    );
    if (!access_token) return;
    const requested_amount = window.prompt(
      "为调用 Token“" + access_token.name + "”充值多少积分？",
      "100",
    );
    if (requested_amount === null) return;
    const amount = Number(requested_amount.trim());
    if (!Number.isInteger(amount) || amount <= 0 || amount > 10000000) {
      set_access_token_action({ error: "单次充值积分必须是 1–10000000 之间的整数" });
      return;
    }
    set_access_token_action({
      busy_id: access_token_id,
      busy_action: "credits",
      error: "",
    });
    try {
      const response = await fetch(
        "/admin/api/access-tokens/" + encodeURIComponent(access_token_id) + "/credits",
        {
          method: "POST",
          credentials: "same-origin",
          cache: "no-store",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ amount, reason: "admin console credit grant" }),
        },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(value.error || "充值积分失败：HTTP " + response.status);
      }
      sync_access_tokens(
        access_tokens_.value.map((candidate) =>
          candidate.id === access_token_id
            ? normalize_access_token(value.access_token)
            : candidate,
        ),
      );
      set_access_token_action({ busy_id: "", busy_action: "", error: "" });
    } catch (error) {
      set_access_token_action({
        busy_id: "",
        busy_action: "",
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function remove_access_token(access_token_id) {
    const access_token = access_tokens_.value.find(
      (candidate) => candidate.id === access_token_id,
    );
    if (!access_token) return;
    if (!window.confirm("移除调用 Token“" + access_token.name + "”？此操作无法撤销。")) return;
    set_access_token_action({
      busy_id: access_token_id,
      busy_action: "remove",
      error: "",
    });
    try {
      const response = await fetch(
        "/admin/api/access-tokens/" + encodeURIComponent(access_token_id),
        {
          method: "DELETE",
          credentials: "same-origin",
          cache: "no-store",
        },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(value.error || "移除调用 Token 失败：HTTP " + response.status);
      }
      sync_access_tokens(
        access_tokens_.value.filter((candidate) => candidate.id !== access_token_id),
      );
      set_access_token_action({ busy_id: "", busy_action: "", error: "" });
    } catch (error) {
      set_access_token_action({
        busy_id: "",
        busy_action: "",
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function copy_created_access_token() {
    const token = access_token_action_.value.created_token;
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token);
      set_access_token_action({ copy_message: "已复制到剪贴板", error: "" });
    } catch (error) {
      set_access_token_action({
        copy_message: "",
        error: "复制失败，请手动选择 Token：" +
          (error instanceof Error ? error.message : String(error)),
      });
    }
  }

  function dismiss_created_access_token() {
    set_access_token_action({
      created_token: "",
      created_token_name: "",
      copy_message: "",
    });
  }

  function reconcile_test_drafts(devices) {
    const test_drafts = Object.assign({}, test_drafts_.value);
    for (const device of devices) {
      const current = test_drafts[device.device_id] || { url: "" };
      test_drafts[device.device_id] = { url: current.url || "" };
    }
    return test_drafts;
  }

  function reconcile_download_drafts(devices, download_devices) {
    const download_drafts = Object.assign({}, download_drafts_.value);
    for (const device of devices) {
      const current = download_drafts[device.device_id] || {
        target_device_id: "",
        download_dir: "",
        filename: "",
      };
      const target_exists = download_devices.some(
        (download_device) =>
          download_device.device_id === current.target_device_id,
      );
      download_drafts[device.device_id] = {
        target_device_id: target_exists
          ? current.target_device_id
          : download_devices[0]
            ? download_devices[0].device_id
            : "",
        download_dir: current.download_dir || "",
        filename: current.filename || "",
      };
    }
    return download_drafts;
  }

  function sync_devices(devices) {
    const active_device_ids = new Set();
    for (const device of devices) {
      const device_id = String(device.device_id);
      active_device_ids.add(device_id);
      const device_ = device_sources.get(device_id);
      if (device_) device_.as(device);
    }
    devices_.assign(devices);
    for (const [device_id, device_] of device_sources) {
      if (active_device_ids.has(device_id)) continue;
      device_sources.delete(device_id);
      device_.destroy();
    }
  }

  function sync_access_tokens(access_tokens) {
    const active_access_token_ids = new Set();
    for (const access_token of access_tokens) {
      const access_token_id = String(access_token.id);
      active_access_token_ids.add(access_token_id);
      const access_token_ = access_token_sources.get(access_token_id);
      if (access_token_) access_token_.as(access_token);
    }
    access_tokens_.assign(access_tokens);
    for (const [access_token_id, access_token_] of access_token_sources) {
      if (active_access_token_ids.has(access_token_id)) continue;
      access_token_sources.delete(access_token_id);
      access_token_.destroy();
    }
  }

  function device_source(device_id, index) {
    const current_device_ = device_sources.get(device_id);
    if (current_device_) return current_device_;
    const device_ = devices_.get(index);
    device_sources.set(device_id, device_);
    return device_;
  }

  function access_token_source(access_token_id, index) {
    const current_access_token_ = access_token_sources.get(access_token_id);
    if (current_access_token_) return current_access_token_;
    const access_token_ = access_tokens_.get(index);
    access_token_sources.set(access_token_id, access_token_);
    return access_token_;
  }

  function set_test_draft(device_id, field, value) {
    const current = test_drafts_.value[device_id] || { url: "" };
    test_drafts_.as(
      Object.assign({}, test_drafts_.value, {
        [device_id]: Object.assign({}, current, { [field]: value }),
      }),
    );
  }

  function set_download_draft(device_id, field, value) {
    const current = download_drafts_.value[device_id] || {
      target_device_id: "",
      download_dir: "",
      filename: "",
    };
    download_drafts_.as(
      Object.assign({}, download_drafts_.value, {
        [device_id]: Object.assign({}, current, { [field]: value }),
      }),
    );
  }

  function set_device_test(device_id, next_test) {
    const current = tests_.value[device_id] || {};
    tests_.as(
      Object.assign({}, tests_.value, {
        [device_id]: Object.assign({}, current, next_test),
      }),
    );
  }

  async function run_test(device_id) {
    const draft = test_drafts_.value[device_id] || { url: "" };
    if (!draft.url.trim()) {
      set_device_test(device_id, {
        submitting: false,
        task: null,
        error: "请填写视频号 URL",
      });
      return;
    }
    const old_timer = test_poll_timers.get(device_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    set_device_test(device_id, {
      submitting: true,
      task: null,
      download: null,
      error: "",
    });
    try {
      const response = await fetch("/admin/api/call", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          target_device_id: device_id,
          method: "wxchannels.fetch",
          args: { url: draft.url.trim() },
        }),
      });
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "发起测试调用失败：HTTP " + response.status,
        );
      if (!value.task || !value.task.id)
        throw new Error("测试调用返回格式不正确");
      set_device_test(device_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
      schedule_test_poll(device_id, value.task.id);
    } catch (error) {
      set_device_test(device_id, {
        submitting: false,
        task: null,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  function schedule_test_poll(device_id, task_id) {
    const current = tests_.value[device_id];
    if (!current || !current.task || current.task.id !== task_id) return;
    if (current.task.status === "completed" || current.task.status === "failed")
      return;
    const old_timer = test_poll_timers.get(device_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    const timer = setTimeout(() => refresh_test(device_id, task_id), 1500);
    test_poll_timers.set(device_id, timer);
  }

  async function refresh_test(device_id, task_id) {
    const current = tests_.value[device_id];
    if (!current || !current.task || current.task.id !== task_id) return;
    try {
      const response = await fetch(
        "/admin/api/tasks/" + encodeURIComponent(task_id),
        { credentials: "same-origin", cache: "no-store" },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "查询测试调用失败：HTTP " + response.status,
        );
      if (!value.task || value.task.id !== task_id)
        throw new Error("测试调用查询返回格式不正确");
      set_device_test(device_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
    } catch (error) {
      set_device_test(device_id, {
        error: error instanceof Error ? error.message : String(error),
      });
    }
    schedule_test_poll(device_id, task_id);
  }

  function set_download_test(device_id, next_download) {
    const current_test = tests_.value[device_id] || {};
    const current_download = current_test.download || {};
    set_device_test(device_id, {
      download: Object.assign({}, current_download, next_download),
    });
  }

  async function run_download_test(device_id) {
    const current_test = tests_.value[device_id];
    const source_task = current_test && current_test.task;
    const draft = download_drafts_.value[device_id] || {
      target_device_id: "",
      download_dir: "",
      filename: "",
    };
    if (!source_task || source_task.status !== "completed") {
      set_download_test(device_id, {
        submitting: false,
        task: null,
        error: "视频号内容调用尚未完成",
      });
      return;
    }
    if (!draft.target_device_id) {
      set_download_test(device_id, {
        submitting: false,
        task: null,
        error: "请选择支持 download.create 的在线设备",
      });
      return;
    }
    const old_timer = download_poll_timers.get(device_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    set_download_test(device_id, { submitting: true, task: null, error: "" });
    try {
      const response = await fetch("/admin/api/call", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          target_device_id: draft.target_device_id,
          method: "download.create",
          args: {
            request: {
              platform: "wxchannels",
              content: source_task.result,
              build_from_fetch: true,
              download_dir: draft.download_dir.trim(),
              filename: draft.filename.trim(),
              config: {},
              auto_start: true,
            },
          },
        }),
      });
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "创建下载任务失败：HTTP " + response.status,
        );
      if (!value.task || !value.task.id)
        throw new Error("创建下载任务返回格式不正确");
      set_download_test(device_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
      schedule_download_poll(device_id, value.task.id);
    } catch (error) {
      set_download_test(device_id, {
        submitting: false,
        task: null,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  function schedule_download_poll(device_id, task_id) {
    const current = tests_.value[device_id];
    const download = current && current.download;
    if (!download || !download.task || download.task.id !== task_id) return;
    if (
      download.task.status === "completed" ||
      download.task.status === "failed"
    )
      return;
    const old_timer = download_poll_timers.get(device_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    const timer = setTimeout(
      () => refresh_download_test(device_id, task_id),
      1500,
    );
    download_poll_timers.set(device_id, timer);
  }

  async function refresh_download_test(device_id, task_id) {
    const current = tests_.value[device_id];
    const download = current && current.download;
    if (!download || !download.task || download.task.id !== task_id) return;
    try {
      const response = await fetch(
        "/admin/api/tasks/" + encodeURIComponent(task_id),
        { credentials: "same-origin", cache: "no-store" },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "查询下载任务失败：HTTP " + response.status,
        );
      if (!value.task || value.task.id !== task_id)
        throw new Error("下载任务查询返回格式不正确");
      set_download_test(device_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
    } catch (error) {
      set_download_test(device_id, {
        error: error instanceof Error ? error.message : String(error),
      });
    }
    schedule_download_poll(device_id, task_id);
  }

  async function refresh(show_loading = true) {
    if (refresh_promise !== null) return refresh_promise;
    if (show_loading) loading_.as(true);
    error_.as("");
    refresh_promise = fetch("/admin/api/overview", {
      credentials: "same-origin",
      cache: "no-store",
    })
      .then(async (response) => {
        if (!response.ok) throw new Error("加载失败：HTTP " + response.status);
        const overview = await response.json();
        if (!overview || !Array.isArray(overview.devices))
          throw new Error("管理接口返回格式不正确");
        const normalized_overview = normalize_overview(overview);
        test_drafts_.as(reconcile_test_drafts(normalized_overview.devices));
        download_drafts_.as(
          reconcile_download_drafts(
            normalized_overview.devices,
            normalized_overview.download_devices,
          ),
        );
        metrics_.as(normalized_overview.metrics);
        download_devices_.assign(normalized_overview.download_devices);
        task_counts_.assign(normalized_overview.task_counts);
        available_methods_.assign(normalized_overview.methods);
        sync_access_tokens(normalized_overview.access_tokens);
        generated_at_.as(normalized_overview.generated_at || 0);
        sync_devices(normalized_overview.devices);
        refreshed_at_.as(Date.now());
        error_.as("");
      })
      .catch((error) => {
        error_.as(error instanceof Error ? error.message : String(error));
      })
      .finally(() => {
        if (show_loading) loading_.as(false);
        refresh_promise = null;
      });
    return refresh_promise;
  }

  function ready() {
    if (started) return;
    started = true;
    refresh();
    refresh_timer = setInterval(() => refresh(false), 5000);
  }

  function dispose() {
    started = false;
    if (refresh_timer !== null) clearInterval(refresh_timer);
    refresh_timer = null;
    for (const timer of test_poll_timers.values()) clearTimeout(timer);
    for (const timer of download_poll_timers.values()) clearTimeout(timer);
    test_poll_timers.clear();
    download_poll_timers.clear();
    access_token_drawer$.destroy();
  }

  const refresh_status_ = combine(
    { loading: loading_, refreshedAt: refreshed_at_ },
    ({ loading, refreshedAt }) =>
      loading
        ? "正在刷新…"
        : refreshedAt
          ? "更新于 " + format_time(refreshedAt)
          : "等待刷新",
  );

  const state = {
    loading: loading_,
    error: error_,
    devices: devices_,
    download_devices: download_devices_,
    task_counts: task_counts_,
    available_methods: available_methods_,
    metrics: metrics_,
    generated_at: generated_at_,
    refreshed_at: refreshed_at_,
    refresh_status: refresh_status_,
    access_tokens: access_tokens_,
    access_token_draft: access_token_draft_,
    access_token_action: access_token_action_,
    test_drafts: test_drafts_,
    download_drafts: download_drafts_,
    tests: tests_,
  };

  const methods = {
    ready,
    dispose,
    refresh,
    runTest: run_test,
    runDownloadTest: run_download_test,
    setTestDraft: set_test_draft,
    setDownloadDraft: set_download_draft,
    setAccessTokenDraft: set_access_token_draft,
    createAccessToken: create_access_token,
    grantAccessTokenCredits: grant_access_token_credits,
    expireAccessToken: expire_access_token,
    removeAccessToken: remove_access_token,
    copyCreatedAccessToken: copy_created_access_token,
    dismissCreatedAccessToken: dismiss_created_access_token,
    openAccessTokenDrawer: open_access_token_drawer,
    deviceSource: device_source,
    accessTokenSource: access_token_source,
  };

  const ui_stores = {
    access_token_drawer: access_token_drawer$,
  };

  return { state, methods, ui: ui_stores };
}

function format_time(value) {
  if (!value) return "未知";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "未知";
  const pad = (part) => String(part).padStart(2, "0");
  return (
    date.getFullYear() +
    "-" +
    pad(date.getMonth() + 1) +
    "-" +
    pad(date.getDate()) +
    " " +
    pad(date.getHours()) +
    ":" +
    pad(date.getMinutes()) +
    ":" +
    pad(date.getSeconds())
  );
}

function format_json(value) {
  try {
    const content = JSON.stringify(value, null, 2);
    return content.length > 50000
      ? content.slice(0, 50000) + "\n… 内容过长，已截断显示"
      : content;
  } catch {
    return String(value);
  }
}

function source_value(value) {
  return value && value.__is_ref ? value.value : value;
}

function device_select_entries(devices, empty_label) {
  const options =
    devices.length === 0
      ? [{ value: "", label: empty_label }]
      : devices.map((device) => ({
          value: String(device.device_id),
          label:
            String(device.device_name) +
            (device.status === "busy" ? "（正在执行调用）" : "（在线）"),
        }));

  return [
    {
      key:
        devices.length === 0
          ? "empty"
          : devices
              .map(
                (device) =>
                  String(device.device_id) +
                  ":" +
                  String(device.device_name) +
                  ":" +
                  String(device.status),
              )
              .join("|"),
      options,
    },
  ];
}

function MetricView(props) {
  return View(
    {
      class: "metric",
      attributes: { n: props.name + "-metric" },
    },
    [
      View(
        {
          class: "metric-label",
          attributes: { n: props.name + "-label" },
        },
        [props.label],
      ),
      View(
        {
          class: "metric-value",
          attributes: { n: props.name + "-value" },
        },
        [
          computed(props.metrics, (metrics) =>
            String(Number(metrics[props.field] || 0)),
          ),
        ],
      ),
    ],
  );
}

function MetricsView(props) {
  return View(
    {
      class: "overview",
      attributes: {
        n: "overview-metrics",
        "aria-label": "汇总指标",
      },
    },
    [
      MetricView({
        name: "registered-devices",
        label: "已登记设备",
        field: "device_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "online-devices",
        label: "在线设备",
        field: "online_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "offline-devices",
        label: "离线设备",
        field: "offline_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "total-tasks",
        label: "保留调用",
        field: "task_count",
        metrics: props.metrics,
      }),
    ],
  );
}

function access_token_expiration_text(access_token) {
  if (access_token.status === "expired") {
    return "已过期于 " + format_time(access_token.expires_at);
  }
  return access_token.expires_at
    ? "有效期至 " + format_time(access_token.expires_at)
    : "永不过期";
}

function DrawerView(props, children = []) {
  const store = props.store;
  const state_ = refobj(store.state);
  const presence_state_ = refobj(store.presence.state);
  const was_exiting_ = ref(false);
  const layer_manager =
    typeof vm.getGlobalLayerManager === "function"
      ? vm.getGlobalLayerManager()
      : null;
  const z_index = 200 + (layer_manager ? layer_manager.size * 50 : 0);
  const unlistens = [
    store.onStateChange((state) => state_.as(state)),
    store.presence.onStateChange((state) => {
      presence_state_.as(state);
      if (state.exit) was_exiting_.as(true);
      if (state.mounted) was_exiting_.as(false);
    }),
  ];
  return ui.DialogPrimitive.Root(
    {
      store,
      attributes: { n: "access-token-drawer-root" },
      onUnmounted() {
        for (const unlisten of unlistens) {
          if (typeof unlisten === "function") unlisten();
        }
      },
    },
    () => {
      const content_children =
        typeof children === "function" ? children() : children;
      return [
        ui.DialogPrimitive.Overlay({
          store,
          zIndex: z_index,
          attributes: { n: "access-token-drawer-overlay" },
          onClick(event) {
            if (event.target === event.currentTarget && state_.value.closeable) {
              store.hide();
            }
          },
          class: computed(presence_state_, (state) =>
            [
              "access-token-drawer-overlay",
              state.enter ? "is-entering" : "",
              state.exit || (!state.mounted && was_exiting_.value)
                ? "is-exiting"
                : "",
            ]
              .filter(Boolean)
              .join(" "),
          ),
        }),
        View(
          {
            class: "access-token-drawer-positioner",
            style: { "z-index": z_index + 1 },
            attributes: { n: "access-token-drawer-positioner" },
          },
          [
            ui.DialogPrimitive.Content(
              {
                store,
                zIndex: z_index + 1,
                style: { width: "min(760px, 100vw)" },
                attributes: {
                  n: "access-token-drawer-content",
                  role: "dialog",
                  "aria-modal": "true",
                },
                class: computed(presence_state_, (state) =>
                  [
                    "access-token-drawer-content",
                    state.enter ? "is-entering" : "",
                    state.exit || (!state.mounted && was_exiting_.value)
                      ? "is-exiting"
                      : "",
                  ]
                    .filter(Boolean)
                    .join(" "),
                ),
              },
              [
                Show({
                  when: computed(state_, (state) => Boolean(state.title)),
                  ok() {
                    return ui.DialogPrimitive.Header(
                      {
                        store,
                        class: "access-token-drawer-header",
                        attributes: { n: "access-token-drawer-header" },
                      },
                      [
                        ui.DialogPrimitive.Title(
                          {
                            store,
                            class: "access-token-drawer-title",
                            attributes: { n: "access-token-drawer-title" },
                          },
                          [computed(state_, (state) => state.title || "")],
                        ),
                      ],
                    );
                  },
                }),
                ...content_children,
                ui.DialogPrimitive.Close(
                  {
                    store,
                    class: "access-token-drawer-close",
                    attributes: {
                      n: "access-token-drawer-close",
                      "aria-label": "关闭调用 Token 抽屉",
                      role: "button",
                      tabindex: "0",
                    },
                  },
                  ["×"],
                ),
              ],
            ),
          ],
        ),
      ];
    },
  );
}

function AccessTokenCardView(props) {
  const vm$ = props.store;
  const access_token_ = props.access_token;
  const access_token_id = String(access_token_.value.id);
  const name_ = computed(access_token_, (access_token) => access_token.name);
  const hint_ = computed(access_token_, (access_token) => access_token.token_hint);
  const status_ = computed(access_token_, (access_token) => access_token.status);
  const card_class_ = computed(
    status_,
    (status) => "access-token-card " + String(status),
  );
  const status_label_ = computed(status_, (status) =>
    status === "expired" ? "已过期" : "有效",
  );
  const expiration_ = computed(access_token_, access_token_expiration_text);
  const usage_ = computed(access_token_, (access_token) =>
    access_token.last_used_at
      ? "最近使用 " + format_time(access_token.last_used_at)
      : "尚未使用",
  );
  const created_ = computed(
    access_token_,
    (access_token) => "创建于 " + format_time(access_token.created_at),
  );
  const credits_ = computed(
    access_token_,
    (access_token) =>
      "积分余额 " + String(access_token.credit_balance) +
      " · 已消耗 " + String(access_token.total_credits_used) +
      " · 累计发放 " + String(access_token.total_credits_granted),
  );
  const busy_ = computed(
    vm$.state.access_token_action,
    (action) => Boolean(action.busy_id),
  );
  const expire_visible_ = computed(status_, (status) => status !== "expired");
  const expire_label_ = computed(
    vm$.state.access_token_action,
    (action) =>
      action.busy_id === access_token_id && action.busy_action === "expire"
        ? "处理中…"
        : "立即过期",
  );
  const remove_label_ = computed(
    vm$.state.access_token_action,
    (action) =>
      action.busy_id === access_token_id && action.busy_action === "remove"
        ? "移除中…"
        : "移除",
  );
  const credits_label_ = computed(
    vm$.state.access_token_action,
    (action) =>
      action.busy_id === access_token_id && action.busy_action === "credits"
        ? "充值中…"
        : "充值积分",
  );

  return View(
    {
      class: card_class_,
      attributes: {
        n: "access-token-card",
        "data-access-token-id": access_token_id,
      },
    },
    [
      View(
        {
          class: "access-token-identity",
          attributes: { n: "access-token-identity" },
        },
        [
          View(
            {
              class: "access-token-name",
              attributes: { n: "access-token-name" },
            },
            [name_],
          ),
          View(
            {
              class: "access-token-hint",
              attributes: { n: "access-token-hint" },
            },
            [hint_],
          ),
        ],
      ),
      View(
        {
          class: "access-token-status-group",
          attributes: { n: "access-token-status-group" },
        },
        [
          View(
            {
              class: computed(
                status_,
                (status) => "access-token-status " + String(status),
              ),
              attributes: { n: "access-token-status" },
            },
            [status_label_],
          ),
          View(
            {
              class: "access-token-time",
              attributes: { n: "access-token-expiration" },
            },
            [expiration_],
          ),
          View(
            {
              class: "access-token-time",
              attributes: { n: "access-token-credits" },
            },
            [credits_],
          ),
          View(
            {
              class: "access-token-time",
              attributes: { n: "access-token-last-used" },
            },
            [usage_],
          ),
          View(
            {
              class: "access-token-time",
              attributes: { n: "access-token-created-at" },
            },
            [created_],
          ),
        ],
      ),
      View(
        {
          class: "access-token-card-actions",
          attributes: { n: "access-token-card-actions" },
        },
        [
          Button(
            {
              attributes: { n: "grant-access-token-credits-button" },
              disabled: busy_,
              onClick() {
                return vm$.methods.grantAccessTokenCredits(access_token_id);
              },
            },
            [credits_label_],
          ),
          Show({
            when: expire_visible_,
            ok() {
              return Button(
                {
                  class: "secondary-button",
                  attributes: { n: "expire-access-token-button" },
                  disabled: busy_,
                  onClick() {
                    return vm$.methods.expireAccessToken(access_token_id);
                  },
                },
                [expire_label_],
              );
            },
          }),
          Button(
            {
              class: "danger-button",
              attributes: { n: "remove-access-token-button" },
              disabled: busy_,
              onClick() {
                return vm$.methods.removeAccessToken(access_token_id);
              },
            },
            [remove_label_],
          ),
        ],
      ),
    ],
  );
}

function AccessTokenManagementView(props) {
  const vm$ = props.store;
  const name_ = computed(
    vm$.state.access_token_draft,
    (draft) => String(draft.name || ""),
  );
  const token_ = computed(
    vm$.state.access_token_draft,
    (draft) => String(draft.token || ""),
  );
  const expires_in_seconds_ = computed(
    vm$.state.access_token_draft,
    (draft) => String(draft.expires_in_seconds || ""),
  );
  const credits_ = computed(
    vm$.state.access_token_draft,
    (draft) => String(draft.credits ?? "0"),
  );
  const creating_ = computed(
    vm$.state.access_token_action,
    (action) => Boolean(action.creating),
  );
  const create_label_ = computed(creating_, (creating) =>
    creating ? "创建中…" : "创建调用 Token",
  );
  const error_ = computed(
    vm$.state.access_token_action,
    (action) => String(action.error || ""),
  );
  const created_token_ = computed(
    vm$.state.access_token_action,
    (action) => String(action.created_token || ""),
  );
  const created_token_name_ = computed(
    vm$.state.access_token_action,
    (action) => String(action.created_token_name || ""),
  );
  const copy_message_ = computed(
    vm$.state.access_token_action,
    (action) => String(action.copy_message || ""),
  );
  const access_token_count_ = computed(
    vm$.state.access_tokens,
    (access_tokens) => String(access_tokens.length) + " 个 Token",
  );
  const expiration_options = [
    { value: "86400", label: "1 天" },
    { value: "604800", label: "7 天" },
    { value: "2592000", label: "30 天" },
    { value: "7776000", label: "90 天" },
    { value: "", label: "永不过期" },
  ];

  return View(
    {
      class: "access-token-management",
      attributes: {
        n: "access-token-management",
        "aria-label": "Bridge 调用 Token 管理",
      },
    },
    [
      View(
        {
          class: "access-token-management-header",
          attributes: { n: "access-token-management-header" },
        },
        [
          View(
            {
              attributes: { n: "access-token-management-heading" },
            },
            [
              View(
                {
                  class: "section-description",
                  attributes: { n: "access-token-management-description" },
                },
                [
                  "每次调用消耗 1 积分。Token 可设置初始积分并随时充值；明文只显示一次，过期或移除后立即停止访问。",
                ],
              ),
            ],
          ),
          View(
            {
              class: "access-token-count",
              attributes: { n: "access-token-count" },
            },
            [access_token_count_],
          ),
        ],
      ),
      View(
        {
          class: "access-token-create-form",
          attributes: { n: "access-token-create-form" },
        },
        [
          View(
            {
              class: "test-field",
              attributes: { n: "access-token-name-field" },
            },
            [
              "用途或使用人（可选）",
              Input({
                class: "test-control",
                value: name_,
                placeholder: "例如：合作方 A",
                attributes: {
                  n: "access-token-name-input",
                  "aria-label": "Token 用途或使用人",
                  maxlength: "128",
                },
                onInput(event) {
                  vm$.methods.setAccessTokenDraft("name", String(event.target.value));
                },
              }),
            ],
          ),
          View(
            {
              class: "test-field",
              attributes: { n: "access-token-value-field" },
            },
            [
              "自定义 Token（可选）",
              Input({
                class: "test-control",
                value: token_,
                placeholder: "留空时自动生成，至少 16 位",
                attributes: {
                  n: "access-token-value-input",
                  "aria-label": "自定义调用 Token",
                  autocomplete: "off",
                  maxlength: "256",
                  spellcheck: "false",
                },
                onInput(event) {
                  vm$.methods.setAccessTokenDraft("token", String(event.target.value));
                },
              }),
            ],
          ),
          View(
            {
              class: "test-field",
              attributes: { n: "access-token-credits-field" },
            },
            [
              "初始积分",
              Input({
                class: "test-control",
                value: credits_,
                attributes: {
                  n: "access-token-credits-input",
                  "aria-label": "Token 初始积分",
                  inputmode: "numeric",
                },
                onInput(event) {
                  vm$.methods.setAccessTokenDraft("credits", String(event.target.value));
                },
              }),
            ],
          ),
          View(
            {
              class: "test-field",
              attributes: { n: "access-token-expiration-field" },
            },
            [
              "有效期",
              Select({
                class: "test-control",
                value: expires_in_seconds_,
                options: expiration_options,
                attributes: {
                  n: "access-token-expiration-select",
                  "aria-label": "Token 有效期",
                },
                onChange(event) {
                  vm$.methods.setAccessTokenDraft(
                    "expires_in_seconds",
                    String(event.target.value),
                  );
                },
              }),
            ],
          ),
          Button(
            {
              attributes: { n: "create-access-token-button" },
              disabled: creating_,
              onClick() {
                return vm$.methods.createAccessToken();
              },
            },
            [create_label_],
          ),
        ],
      ),
      Show({
        when: computed(error_, Boolean),
        ok() {
          return View(
            {
              class: "access-token-message error-message",
              attributes: {
                n: "access-token-error",
                role: "alert",
              },
            },
            [error_],
          );
        },
      }),
      Show({
        when: computed(created_token_, Boolean),
        ok() {
          return View(
            {
              class: "created-access-token",
              attributes: { n: "created-access-token" },
            },
            [
              View(
                {
                  class: "created-access-token-title",
                  attributes: { n: "created-access-token-title" },
                },
                ["请立即保存“", created_token_name_, "”的 Token"],
              ),
              View(
                {
                  class: "created-access-token-description",
                  attributes: { n: "created-access-token-description" },
                },
                ["关闭此提示后无法再次查看明文，只能创建新 Token。"],
              ),
              View(
                {
                  class: "created-access-token-value-row",
                  attributes: { n: "created-access-token-value-row" },
                },
                [
                  Input({
                    class: "test-control created-access-token-value",
                    value: created_token_,
                    readonly: true,
                    attributes: {
                      n: "created-access-token-value",
                      "aria-label": "新创建的调用 Token",
                    },
                  }),
                  Button(
                    {
                      attributes: { n: "copy-access-token-button" },
                      onClick() {
                        return vm$.methods.copyCreatedAccessToken();
                      },
                    },
                    ["复制"],
                  ),
                  Button(
                    {
                      class: "secondary-button",
                      attributes: { n: "dismiss-access-token-button" },
                      onClick() {
                        vm$.methods.dismissCreatedAccessToken();
                      },
                    },
                    ["已保存，关闭"],
                  ),
                ],
              ),
              Show({
                when: computed(copy_message_, Boolean),
                ok() {
                  return View(
                    {
                      class: "access-token-message success-message",
                      attributes: { n: "access-token-copy-message" },
                    },
                    [copy_message_],
                  );
                },
              }),
            ],
          );
        },
      }),
      View(
        {
          class: "access-token-list",
          attributes: { n: "access-token-list" },
        },
        [
          Show({
            when: computed(
              vm$.state.access_tokens,
              (access_tokens) => access_tokens.length === 0,
            ),
            ok() {
              return View(
                {
                  class: "empty access-token-list-empty",
                  attributes: { n: "empty-access-token-list" },
                },
                ["尚未创建调用 Token。设备 Secret 不应分发给外部调用者。"],
              );
            },
          }),
          For({
            key: "id",
            each: vm$.state.access_tokens,
            render(access_token_value, index_) {
              const access_token_id = String(source_value(access_token_value).id);
              return AccessTokenCardView({
                store: vm$,
                access_token: vm$.methods.accessTokenSource(
                  access_token_id,
                  index_.value,
                ),
              });
            },
          }),
        ],
      ),
    ],
  );
}

function AccessTokenDrawerView(props) {
  return DrawerView(
    { store: props.store.ui.access_token_drawer },
    () => [
      View(
        {
          class: "access-token-drawer-body",
          attributes: { n: "access-token-drawer-body" },
        },
        [AccessTokenManagementView({ store: props.store })],
      ),
    ],
  );
}

function task_meta_text(task) {
  if (!task) return "";
  let meta =
    "调用 " +
    String(task.id) +
    " · 执行设备 " +
    String(
      task.assigned_device_id ||
        task.assigned_client_id ||
        task.target_device_id ||
        task.target_client_id ||
        "等待分配",
    ) +
    " · 尝试 " +
    String(task.attempt_count || 0) +
    " · 创建 " +
    format_time(task.created_at) +
    " · 更新 " +
    format_time(task.updated_at);
  if (task.completed_at) meta += " · 完成 " + format_time(task.completed_at);
  return meta;
}

function task_output_text(task) {
  if (!task) return "";
  if (task.status === "completed") return format_json(task.result);
  if (task.status === "failed") return String(task.error || "调用执行失败");
  return "";
}

function TaskResultView(props) {
  const task_ = computed(props.result, (result) => result.task || null);
  const error_ = computed(props.result, (result) => String(result.error || ""));
  const submitting_ = computed(props.result, (result) =>
    Boolean(result.submitting),
  );
  const status_text_ = computed(task_, (task) =>
    task ? props.status_labels[task.status] || String(task.status) : "",
  );
  const status_class_ = computed(
    task_,
    (task) => "test-status " + String(task ? task.status : ""),
  );
  const meta_ = computed(task_, task_meta_text);
  const output_ = computed(task_, task_output_text);
  const output_visible_ = computed(task_, (task) =>
    Boolean(task && (task.status === "completed" || task.status === "failed")),
  );

  return View(
    {
      class: "test-result",
      attributes: { n: props.name + "-result" },
    },
    [
      Show({
        when: computed(error_, Boolean),
        ok() {
          return View(
            {
              class: "test-status failed",
              attributes: { n: props.name + "-error" },
            },
            [error_],
          );
        },
      }),
      Show({
        when: submitting_,
        ok() {
          return View(
            {
              class: "test-status",
              attributes: { n: props.name + "-submitting" },
            },
            [props.submitting_text],
          );
        },
      }),
      Show({
        when: computed(task_, Boolean),
        ok() {
          return [
            View(
              {
                class: status_class_,
                attributes: { n: props.name + "-status" },
              },
              [status_text_],
            ),
            View(
              {
                class: "test-meta",
                attributes: { n: props.name + "-meta" },
              },
              [meta_],
            ),
            Show({
              when: output_visible_,
              ok() {
                return View(
                  {
                    class: "test-output",
                    attributes: { n: props.name + "-output" },
                  },
                  [output_],
                );
              },
            }),
          ];
        },
      }),
    ],
  );
}

function TestResultView(props) {
  return TaskResultView({
    name: "bridge-test",
    result: props.test,
    submitting_text: "正在发起测试调用…",
    status_labels: {
      queued: "等待目标设备接收调用",
      assigned: "调用已推送，等待设备确认",
      running: "目标设备正在获取数据",
      completed: "测试成功：设备已获取并提交数据",
      failed: "测试失败",
    },
  });
}

function DownloadResultView(props) {
  return TaskResultView({
    name: "bridge-download",
    result: props.download,
    submitting_text: "正在发起 Bridge 下载调用…",
    status_labels: {
      queued: "等待下载设备接收调用",
      assigned: "下载调用已推送，等待设备确认",
      running: "目标设备正在创建本地下载任务",
      completed: "提交成功：目标设备已创建并启动本地下载任务",
      failed: "下载任务提交失败",
    },
  });
}

function DownloadPanelView(props) {
  const vm$ = props.store;
  const device_id = props.device_id;
  const download_select_entries_ = computed(
    props.download_devices,
    (download_devices) =>
      device_select_entries(
        download_devices,
        "没有在线且支持 download.create 的设备",
      ),
  );
  const panel_visible_ = computed(props.test, (test) =>
    Boolean(test.task && test.task.status === "completed"),
  );
  const download_ = computed(props.test, (test) => test.download || {});
  const active_ = computed(download_, (download) =>
    Boolean(
      download.submitting ||
      (download.task &&
        !["completed", "failed"].includes(download.task.status)),
    ),
  );
  const disabled_ = combine(
    { active: active_, devices: props.download_devices },
    ({ active, devices }) => active || devices.length === 0,
  );
  const target_device_id_ = computed(props.draft, (draft) =>
    String(draft.target_device_id || ""),
  );
  const download_dir_ = computed(props.draft, (draft) =>
    String(draft.download_dir || ""),
  );
  const filename_ = computed(props.draft, (draft) =>
    String(draft.filename || ""),
  );
  const submit_label_ = computed(active_, (active) =>
    active ? "提交中…" : "发起下载调用",
  );

  return Show({
    when: panel_visible_,
    ok() {
      return View(
        {
          class: "download-panel",
          attributes: { n: "bridge-download-panel" },
        },
        [
          View(
            {
              class: "test-title",
              attributes: { n: "bridge-download-title" },
            },
            ["将获取内容提交给下载设备"],
          ),
          View(
            {
              class: "test-description",
              attributes: { n: "bridge-download-description" },
            },
            [
              "把上一步回传的 content 作为 args 调用 download.create；成功后目标设备会创建并自动启动本地下载任务。",
            ],
          ),
          View(
            {
              class: "test-form download-form",
              attributes: { n: "bridge-download-form" },
            },
            [
              View(
                {
                  class: "test-field",
                  attributes: { n: "bridge-download-device-field" },
                },
                [
                  "下载设备",
                  For({
                    key: "key",
                    each: download_select_entries_,
                    render(entry_value) {
                      const entry = source_value(entry_value);
                      return Select({
                        class: "test-control",
                        value: target_device_id_,
                        options: entry.options,
                        attributes: {
                          n: "bridge-download-device-select",
                          "aria-label": "下载设备",
                        },
                        onChange(event) {
                          vm$.methods.setDownloadDraft(
                            device_id,
                            "target_device_id",
                            String(event.target.value),
                          );
                        },
                      });
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "test-field",
                  attributes: { n: "bridge-download-directory-field" },
                },
                [
                  "下载目录（可选）",
                  Input({
                    class: "test-control",
                    value: download_dir_,
                    placeholder: "留空使用目标设备默认目录",
                    attributes: {
                      n: "bridge-download-directory-input",
                      "aria-label": "下载目录（可选）",
                    },
                    onInput(event) {
                      vm$.methods.setDownloadDraft(
                        device_id,
                        "download_dir",
                        String(event.target.value),
                      );
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "test-field",
                  attributes: { n: "bridge-download-filename-field" },
                },
                [
                  "文件名（可选）",
                  Input({
                    class: "test-control",
                    value: filename_,
                    placeholder: "留空使用内容标题",
                    attributes: {
                      n: "bridge-download-filename-input",
                      "aria-label": "文件名（可选）",
                    },
                    onInput(event) {
                      vm$.methods.setDownloadDraft(
                        device_id,
                        "filename",
                        String(event.target.value),
                      );
                    },
                  }),
                ],
              ),
              Button(
                {
                  attributes: {
                    n: "bridge-download-submit-button",
                  },
                  disabled: disabled_,
                  onClick() {
                    return vm$.methods.runDownloadTest(device_id);
                  },
                },
                [submit_label_],
              ),
            ],
          ),
          DownloadResultView({ download: download_ }),
        ],
      );
    },
  });
}

function TestPanelView(props) {
  const vm$ = props.store;
  const device_id = props.device_id;
  const active_ = computed(props.test, (test) =>
    Boolean(
      test.submitting ||
      (test.task && !["completed", "failed"].includes(test.task.status)),
    ),
  );
  const disabled_ = combine(
    { active: active_, device: props.device },
    ({ active, device }) => active || !device.can_test,
  );
  const url_ = computed(props.draft, (draft) => String(draft.url || ""));
  const submit_label_ = computed(active_, (active) =>
    active ? "测试中…" : "发起测试调用",
  );

  return View(
    {
      class: "test-panel",
      attributes: { n: "bridge-test-panel" },
    },
    [
      View(
        {
          class: "test-title",
          attributes: { n: "bridge-test-title" },
        },
        ["调用此设备的 wxchannels.fetch 方法"],
      ),
      View(
        {
          class: "test-description",
          attributes: { n: "bridge-test-description" },
        },
        [
          "发起真实调用并观察当前设备是否接收、获取视频号数据并把结果提交回 Bridge。",
        ],
      ),
      View(
        {
          class: "test-form device-test-form",
          attributes: { n: "bridge-test-form" },
        },
        [
          View(
            {
              class: "test-field",
              attributes: { n: "bridge-test-url-field" },
            },
            [
              "视频号 URL",
              Input({
                class: "test-control",
                value: url_,
                placeholder: "https://channels.weixin.qq.com/...",
                type: "url",
                attributes: {
                  n: "bridge-test-url-input",
                  "aria-label": "视频号 URL",
                },
                onInput(event) {
                  vm$.methods.setTestDraft(
                    device_id,
                    "url",
                    String(event.target.value),
                  );
                },
              }),
            ],
          ),
          Button(
            {
              attributes: {
                n: "bridge-test-submit-button",
              },
              disabled: disabled_,
              onClick() {
                return vm$.methods.runTest(device_id);
              },
            },
            [submit_label_],
          ),
        ],
      ),
      TestResultView({ test: props.test }),
      DownloadPanelView({
        store: vm$,
        device_id,
        download_devices: props.download_devices,
        test: props.test,
        draft: props.download_draft,
      }),
    ],
  );
}

function format_os(value) {
  const labels = {
    darwin: "macOS",
    linux: "Linux",
    windows: "Windows",
    freebsd: "FreeBSD",
  };
  return labels[value] || value || "未知系统";
}

function BridgeSummaryView(props) {
  return View(
    {
      class: "bridge-summary",
      attributes: { n: "bridge-summary" },
    },
    [
      View(
        {
          class: "bridge-summary-section",
          attributes: { n: "bridge-method-summary" },
        },
        [
          View(
            {
              class: "summary-label",
              attributes: { n: "bridge-method-label" },
            },
            ["当前在线方法"],
          ),
          View(
            {
              class: "methods",
              attributes: { n: "bridge-methods" },
            },
            [
              Show({
                when: computed(
                  props.methods,
                  (methods) => methods.length === 0,
                ),
                ok() {
                  return View(
                    {
                      class: "method muted",
                      attributes: { n: "empty-bridge-method" },
                    },
                    ["暂无在线方法"],
                  );
                },
              }),
              For({
                each: props.methods,
                render(method_value) {
                  return View(
                    {
                      class: "method",
                      attributes: { n: "bridge-method" },
                    },
                    [String(source_value(method_value))],
                  );
                },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "bridge-summary-section",
          attributes: { n: "bridge-task-summary" },
        },
        [
          View(
            {
              class: "summary-label",
              attributes: { n: "bridge-task-label" },
            },
            ["Bridge 调用"],
          ),
          View(
            {
              class: "task-counts",
              attributes: { n: "bridge-task-counts" },
            },
            [
              Show({
                when: computed(
                  props.task_counts,
                  (task_counts) => task_counts.length === 0,
                ),
                ok() {
                  return View(
                    {
                      class: "task-badge",
                      attributes: { n: "empty-task-count" },
                    },
                    ["暂无调用"],
                  );
                },
              }),
              For({
                each: props.task_counts,
                render(item_value) {
                  const item = source_value(item_value);
                  return View(
                    {
                      class: "task-badge",
                      attributes: { n: "task-count" },
                    },
                    [String(item.status) + " · " + String(item.count)],
                  );
                },
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

function DeviceView(props) {
  const vm$ = props.store;
  const device_ = props.device;
  const device_id = String(device_.value.device_id);
  const status_ = computed(device_, (device) => device.status);
  const status_class_ = computed(
    status_,
    (status) => "connection-status " + status,
  );
  const status_label_ = computed(status_, (status) => {
    const labels = { online: "在线", busy: "执行调用", offline: "离线" };
    return labels[status] || "离线";
  });
  const card_class_ = computed(
    status_,
    (status) => "device-card " + status,
  );
  const device_name_ = computed(device_, (device) => device.device_name);
  const os_label_ = computed(device_, (device) => format_os(device.device_os));
  const activity_text_ = computed(device_, (device) =>
    device.status === "offline"
      ? "最近活跃 " +
        format_time(device.last_seen_at) +
        " · 离线于 " +
        format_time(device.disconnected_at)
      : "连接于 " +
        format_time(device.connected_at) +
        " · 最近活跃 " +
        format_time(device.last_seen_at),
  );
  const methods_ = computed(device_, (device) => device.methods);
  const test_visible_ = computed(device_, (device) =>
    device.methods.includes("wxchannels.fetch"),
  );
  const test_ = computed(vm$.state.tests, (tests) => tests[device_id] || {});
  const test_draft_ = computed(
    vm$.state.test_drafts,
    (drafts) => drafts[device_id] || { url: "" },
  );
  const download_draft_ = computed(
    vm$.state.download_drafts,
    (drafts) =>
      drafts[device_id] || {
        target_device_id: "",
        download_dir: "",
        filename: "",
      },
  );

  return View(
    {
      class: card_class_,
      attributes: {
        n: "device-card",
        "data-device-id": device_id,
      },
    },
    [
      View(
        {
          class: "device-header",
          attributes: { n: "device-card-header" },
        },
        [
          View(
            {
              attributes: { n: "device-identity" },
            },
            [
              View(
                {
                  class: "device-name",
                  attributes: { n: "device-name" },
                },
                [device_name_],
              ),
              View(
                {
                  class: "device-meta",
                  attributes: { n: "device-meta" },
                },
                [os_label_, " · ", device_id],
              ),
              View(
                {
                  class: "device-time",
                  attributes: { n: "device-activity-time" },
                },
                [activity_text_],
              ),
            ],
          ),
          View(
            {
              class: status_class_,
              attributes: { n: "device-connection-status" },
            },
            [status_label_],
          ),
        ],
      ),
      View(
        {
          class: "device-method-section",
          attributes: { n: "device-method-section" },
        },
        [
          View(
            {
              class: "summary-label",
              attributes: { n: "device-method-label" },
            },
            ["可调用方法"],
          ),
          View(
            {
              class: "methods",
              attributes: { n: "device-methods" },
            },
            [
              Show({
                when: computed(
                  methods_,
                  (methods) => methods.length === 0,
                ),
                ok() {
                  return View(
                    {
                      class: "method muted",
                      attributes: { n: "empty-device-method" },
                    },
                    ["仅发布调用"],
                  );
                },
              }),
              For({
                each: methods_,
                render(method_value) {
                  return View(
                    {
                      class: "method",
                      attributes: { n: "device-method" },
                    },
                    [String(source_value(method_value))],
                  );
                },
              }),
            ],
          ),
        ],
      ),
      Show({
        when: test_visible_,
        ok() {
          return TestPanelView({
            store: vm$,
            device: device_,
            device_id,
            download_devices: vm$.state.download_devices,
            test: test_,
            draft: test_draft_,
            download_draft: download_draft_,
          });
        },
      }),
    ],
  );
}

function DeviceListView(props) {
  const vm$ = props.store;

  return View(
    {
      class: "device-list",
      attributes: {
        n: "device-list",
        "aria-label": "操作系统设备列表",
      },
    },
    [
      Show({
        when: computed(vm$.state.devices, (devices) => devices.length === 0),
        ok() {
          return View(
            {
              class: "empty device-list-empty",
              attributes: { n: "empty-device-list" },
            },
            ["尚无设备注册。请在一台操作系统中启用 Bridge 连接。"],
          );
        },
      }),
      For({
        key: "device_id",
        each: vm$.state.devices,
        render(device_value, index_) {
          const device_id = String(source_value(device_value).device_id);
          return DeviceView({
            store: vm$,
            device: vm$.methods.deviceSource(device_id, index_.value),
          });
        },
      }),
    ],
  );
}

function ApplicationView() {
  const vm$ = DashboardViewModel();
  const refresh_disabled_ = computed(vm$.state.loading, (loading) =>
    loading ? true : undefined,
  );
  const access_token_button_label_ = computed(
    vm$.state.access_tokens,
    (access_tokens) => "调用 Token · " + String(access_tokens.length),
  );

  return View(
    {
      class: "dashboard-view",
      attributes: { n: "dashboard-view" },
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.dispose();
      },
    },
    [
      View(
        {
          class: "dashboard-header",
          attributes: { n: "dashboard-header" },
        },
        [
          View(
            {
              attributes: { n: "dashboard-heading" },
            },
            [
              View(
                {
                  class: "eyebrow",
                  attributes: { n: "dashboard-eyebrow" },
                },
                ["Durable Objects Control Plane"],
              ),
              View(
                {
                  class: "dashboard-title",
                  attributes: { n: "dashboard-title" },
                },
                ["WX Channels Bridge"],
              ),
              View(
                {
                  class: "subtitle",
                  attributes: { n: "dashboard-subtitle" },
                },
                [
                  "管理操作系统设备、调用 Token，并观察 Bridge 请求的桥接、转发与执行状态。",
                ],
              ),
            ],
          ),
          View(
            {
              class: "actions",
              attributes: { n: "dashboard-actions" },
            },
            [
              View(
                {
                  class: "status",
                  attributes: { n: "refresh-status" },
                },
                [vm$.state.refresh_status],
              ),
              Button(
                {
                  class: "secondary-button",
                  attributes: { n: "open-access-token-drawer-button" },
                  onClick() {
                    vm$.methods.openAccessTokenDrawer();
                  },
                },
                [access_token_button_label_],
              ),
              Button(
                {
                  attributes: {
                    n: "refresh-button",
                  },
                  disabled: refresh_disabled_,
                  onClick() {
                    return vm$.methods.refresh();
                  },
                },
                [
                  computed(vm$.state.loading, (loading) =>
                    loading ? "正在刷新…" : "立即刷新",
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
      MetricsView({ metrics: vm$.state.metrics }),
      Show({
        when: computed(vm$.state.error, Boolean),
        ok() {
          return View(
            {
              class: "error visible",
              attributes: {
                n: "dashboard-error",
                role: "alert",
              },
            },
            [vm$.state.error],
          );
        },
      }),
      BridgeSummaryView({
        methods: vm$.state.available_methods,
        task_counts: vm$.state.task_counts,
      }),
      DeviceListView({ store: vm$ }),
      AccessTokenDrawerView({ store: vm$ }),
      View(
        {
          class: "dashboard-footer",
          attributes: { n: "dashboard-footer" },
        },
        ["页面每 5 秒自动刷新 · Token 明文仅在创建时显示一次"],
      ),
    ],
  );
}

function bootstrap() {
  const root_node = document.querySelector('[data-n="dashboard-root"]');
  if (!root_node) {
    throw new Error("Bridge 管理页无法启动：缺少根节点");
  }
  Timeless.DOM.render(ApplicationView(), root_node);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
} else {
  bootstrap();
}

const Timeless = window.Timeless;
if (!Timeless || !Timeless.DOM) {
  throw new Error("Hub 管理页无法启动：Timeless 运行时未加载");
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

function empty_metrics() {
  return {
    hub_count: 0,
    online_count: 0,
    offline_count: 0,
    task_count: 0,
  };
}

function DashboardViewModel() {
  const loading_ = ref(false);
  const error_ = ref("");
  const hubs_ = refarr([]);
  const metrics_ = refobj(empty_metrics());
  const generated_at_ = ref(0);
  const refreshed_at_ = ref(0);
  const test_drafts_ = refobj({});
  const download_drafts_ = refobj({});
  const tests_ = refobj({});
  const test_poll_timers = new Map();
  const download_poll_timers = new Map();
  let refresh_promise = null;
  let refresh_timer = null;
  let started = false;

  function normalize_overview(overview) {
    let online_count = 0;
    let offline_count = 0;
    let task_count = 0;
    const hubs = overview.hubs.map((hub) => {
      const clients = Array.isArray(hub.clients) ? hub.clients : [];
      const task_counts = Array.isArray(hub.task_counts) ? hub.task_counts : [];
      const hub_online_count = clients.filter(
        (client) => client.status === "online" || client.status === "busy",
      ).length;
      const hub_offline_count = clients.filter(
        (client) => client.status === "offline",
      ).length;
      const hub_task_count = task_counts.reduce(
        (total, item) => total + Number(item.count || 0),
        0,
      );
      const test_clients = clients.filter(
        (client) =>
          (client.status === "online" || client.status === "busy") &&
          Array.isArray(client.capabilities) &&
          client.capabilities.includes("wxchannels.fetch"),
      );
      const download_clients = clients.filter(
        (client) =>
          (client.status === "online" || client.status === "busy") &&
          Array.isArray(client.capabilities) &&
          client.capabilities.includes("download.create"),
      );
      online_count += hub_online_count;
      offline_count += hub_offline_count;
      task_count += hub_task_count;
      return Object.assign({}, hub, {
        clients,
        task_counts,
        test_clients,
        download_clients,
        online_count: hub_online_count,
        offline_count: hub_offline_count,
        task_count: hub_task_count,
      });
    });
    return Object.assign({}, overview, {
      hubs,
      metrics: {
        hub_count: hubs.length,
        online_count,
        offline_count,
        task_count,
      },
    });
  }

  function reconcile_test_drafts(hubs) {
    const test_drafts = Object.assign({}, test_drafts_.value);
    for (const hub of hubs) {
      const current = test_drafts[hub.id] || { target_client_id: "", url: "" };
      const clients = Array.isArray(hub.test_clients) ? hub.test_clients : [];
      const target_exists = clients.some(
        (client) => client.client_id === current.target_client_id,
      );
      test_drafts[hub.id] = {
        target_client_id: target_exists
          ? current.target_client_id
          : clients[0]
            ? clients[0].client_id
            : "",
        url: current.url || "",
      };
    }
    return test_drafts;
  }

  function reconcile_download_drafts(hubs) {
    const download_drafts = Object.assign({}, download_drafts_.value);
    for (const hub of hubs) {
      const current = download_drafts[hub.id] || {
        target_client_id: "",
        download_dir: "",
        filename: "",
      };
      const clients = Array.isArray(hub.download_clients)
        ? hub.download_clients
        : [];
      const target_exists = clients.some(
        (client) => client.client_id === current.target_client_id,
      );
      download_drafts[hub.id] = {
        target_client_id: target_exists
          ? current.target_client_id
          : clients[0]
            ? clients[0].client_id
            : "",
        download_dir: current.download_dir || "",
        filename: current.filename || "",
      };
    }
    return download_drafts;
  }

  function set_test_draft(hub_id, field, value) {
    const current = test_drafts_.value[hub_id] || {
      target_client_id: "",
      url: "",
    };
    test_drafts_.as(
      Object.assign({}, test_drafts_.value, {
        [hub_id]: Object.assign({}, current, { [field]: value }),
      }),
    );
  }

  function set_download_draft(hub_id, field, value) {
    const current = download_drafts_.value[hub_id] || {
      target_client_id: "",
      download_dir: "",
      filename: "",
    };
    download_drafts_.as(
      Object.assign({}, download_drafts_.value, {
        [hub_id]: Object.assign({}, current, { [field]: value }),
      }),
    );
  }

  function set_hub_test(hub_id, next_test) {
    const current = tests_.value[hub_id] || {};
    tests_.as(
      Object.assign({}, tests_.value, {
        [hub_id]: Object.assign({}, current, next_test),
      }),
    );
  }

  async function run_test(hub_id) {
    const draft = test_drafts_.value[hub_id] || {
      target_client_id: "",
      url: "",
    };
    if (!draft.target_client_id || !draft.url.trim()) {
      set_hub_test(hub_id, {
        submitting: false,
        task: null,
        error: "请选择在线电脑并填写视频号 URL",
      });
      return;
    }
    const old_timer = test_poll_timers.get(hub_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    set_hub_test(hub_id, {
      submitting: true,
      task: null,
      download: null,
      error: "",
    });
    try {
      const response = await fetch("/admin/api/tests", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          hub_id,
          target_client_id: draft.target_client_id,
          url: draft.url.trim(),
        }),
      });
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "创建测试任务失败：HTTP " + response.status,
        );
      if (!value.task || !value.task.id)
        throw new Error("创建测试任务返回格式不正确");
      set_hub_test(hub_id, { submitting: false, task: value.task, error: "" });
      schedule_test_poll(hub_id, value.task.id);
    } catch (error) {
      set_hub_test(hub_id, {
        submitting: false,
        task: null,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  function schedule_test_poll(hub_id, task_id) {
    const current = tests_.value[hub_id];
    if (!current || !current.task || current.task.id !== task_id) return;
    if (current.task.status === "completed" || current.task.status === "failed")
      return;
    const old_timer = test_poll_timers.get(hub_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    const timer = setTimeout(() => refresh_test(hub_id, task_id), 1500);
    test_poll_timers.set(hub_id, timer);
  }

  async function refresh_test(hub_id, task_id) {
    const current = tests_.value[hub_id];
    if (!current || !current.task || current.task.id !== task_id) return;
    try {
      const response = await fetch(
        "/admin/api/tasks/" +
          encodeURIComponent(task_id) +
          "?hub_id=" +
          encodeURIComponent(hub_id),
        {
          credentials: "same-origin",
          cache: "no-store",
        },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "查询测试任务失败：HTTP " + response.status,
        );
      if (!value.task || value.task.id !== task_id)
        throw new Error("测试任务查询返回格式不正确");
      set_hub_test(hub_id, { submitting: false, task: value.task, error: "" });
    } catch (error) {
      set_hub_test(hub_id, {
        error: error instanceof Error ? error.message : String(error),
      });
    }
    schedule_test_poll(hub_id, task_id);
  }

  function set_download_test(hub_id, next_download) {
    const current_test = tests_.value[hub_id] || {};
    const current_download = current_test.download || {};
    set_hub_test(hub_id, {
      download: Object.assign({}, current_download, next_download),
    });
  }

  async function run_download_test(hub_id) {
    const current_test = tests_.value[hub_id];
    const source_task = current_test && current_test.task;
    const draft = download_drafts_.value[hub_id] || {
      target_client_id: "",
      download_dir: "",
      filename: "",
    };
    if (!source_task || source_task.status !== "completed") {
      set_download_test(hub_id, {
        submitting: false,
        task: null,
        error: "视频号内容任务尚未完成",
      });
      return;
    }
    if (!draft.target_client_id) {
      set_download_test(hub_id, {
        submitting: false,
        task: null,
        error: "请选择支持 download.create 的在线电脑",
      });
      return;
    }
    const old_timer = download_poll_timers.get(hub_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    set_download_test(hub_id, { submitting: true, task: null, error: "" });
    try {
      const response = await fetch("/admin/api/downloads", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          hub_id,
          target_client_id: draft.target_client_id,
          source_task_id: source_task.id,
          download_dir: draft.download_dir.trim(),
          filename: draft.filename.trim(),
        }),
      });
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "创建下载任务失败：HTTP " + response.status,
        );
      if (!value.task || !value.task.id)
        throw new Error("创建下载任务返回格式不正确");
      set_download_test(hub_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
      schedule_download_poll(hub_id, value.task.id);
    } catch (error) {
      set_download_test(hub_id, {
        submitting: false,
        task: null,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  function schedule_download_poll(hub_id, task_id) {
    const current = tests_.value[hub_id];
    const download = current && current.download;
    if (!download || !download.task || download.task.id !== task_id) return;
    if (
      download.task.status === "completed" ||
      download.task.status === "failed"
    )
      return;
    const old_timer = download_poll_timers.get(hub_id);
    if (old_timer !== undefined) clearTimeout(old_timer);
    const timer = setTimeout(
      () => refresh_download_test(hub_id, task_id),
      1500,
    );
    download_poll_timers.set(hub_id, timer);
  }

  async function refresh_download_test(hub_id, task_id) {
    const current = tests_.value[hub_id];
    const download = current && current.download;
    if (!download || !download.task || download.task.id !== task_id) return;
    try {
      const response = await fetch(
        "/admin/api/tasks/" +
          encodeURIComponent(task_id) +
          "?hub_id=" +
          encodeURIComponent(hub_id),
        {
          credentials: "same-origin",
          cache: "no-store",
        },
      );
      const value = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          value.error || "查询下载任务失败：HTTP " + response.status,
        );
      if (!value.task || value.task.id !== task_id)
        throw new Error("下载任务查询返回格式不正确");
      set_download_test(hub_id, {
        submitting: false,
        task: value.task,
        error: "",
      });
    } catch (error) {
      set_download_test(hub_id, {
        error: error instanceof Error ? error.message : String(error),
      });
    }
    schedule_download_poll(hub_id, task_id);
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
        if (!overview || !Array.isArray(overview.hubs))
          throw new Error("管理接口返回格式不正确");
        const normalized_overview = normalize_overview(overview);
        test_drafts_.as(reconcile_test_drafts(normalized_overview.hubs));
        download_drafts_.as(
          reconcile_download_drafts(normalized_overview.hubs),
        );
        metrics_.as(normalized_overview.metrics);
        generated_at_.as(normalized_overview.generated_at || 0);
        hubs_.as(normalized_overview.hubs);
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
    hubs: hubs_,
    metrics: metrics_,
    generated_at: generated_at_,
    refreshed_at: refreshed_at_,
    refresh_status: refresh_status_,
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
  };

  return { state, methods };
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
        name: "registered-hubs",
        label: "已登记 Hub",
        field: "hub_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "online-clients",
        label: "在线电脑",
        field: "online_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "offline-clients",
        label: "离线电脑",
        field: "offline_count",
        metrics: props.metrics,
      }),
      MetricView({
        name: "total-tasks",
        label: "保留任务",
        field: "task_count",
        metrics: props.metrics,
      }),
    ],
  );
}

function task_meta_text(task) {
  if (!task) return "";
  let meta =
    "任务 " +
    String(task.id) +
    " · 执行电脑 " +
    String(task.assigned_client_id || task.target_client_id || "等待分配") +
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
  if (task.status === "failed") return String(task.error || "任务执行失败");
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
    name: "hub-test",
    result: props.test,
    submitting_text: "正在创建测试任务…",
    status_labels: {
      queued: "等待目标电脑领取",
      assigned: "任务已推送，等待电脑确认",
      running: "目标电脑正在获取数据",
      completed: "测试成功：电脑已获取并提交数据",
      failed: "测试失败",
    },
  });
}

function DownloadResultView(props) {
  return TaskResultView({
    name: "hub-download",
    result: props.download,
    submitting_text: "正在创建 Hub 下载任务…",
    status_labels: {
      queued: "等待下载电脑领取",
      assigned: "下载任务已推送，等待电脑确认",
      running: "目标电脑正在创建本地下载任务",
      completed: "提交成功：目标电脑已创建并启动本地下载任务",
      failed: "下载任务提交失败",
    },
  });
}

function DownloadPanelView(props) {
  const vm$ = props.store;
  const hub = props.hub;
  const hub_id = String(hub.id);
  const download_clients = Array.isArray(hub.download_clients)
    ? hub.download_clients
    : [];
  const download_options =
    download_clients.length === 0
      ? [{ value: "", label: "没有在线且支持 download.create 的电脑" }]
      : download_clients.map((client) => ({
          value: String(client.client_id),
          label:
            String(client.client_id) +
            (client.status === "busy" ? "（正在执行任务）" : "（在线）"),
        }));
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
  const disabled_ = computed(
    active_,
    (active) => active || download_clients.length === 0,
  );
  const target_client_id_ = computed(props.draft, (draft) =>
    String(draft.target_client_id || ""),
  );
  const download_dir_ = computed(props.draft, (draft) =>
    String(draft.download_dir || ""),
  );
  const filename_ = computed(props.draft, (draft) =>
    String(draft.filename || ""),
  );
  const submit_label_ = computed(active_, (active) =>
    active ? "提交中…" : "提交下载任务",
  );

  return Show({
    when: panel_visible_,
    ok() {
      return View(
        {
          class: "download-panel",
          attributes: { n: "hub-download-panel" },
        },
        [
          View(
            {
              class: "test-title",
              attributes: { n: "hub-download-title" },
            },
            ["将获取内容提交给下载电脑"],
          ),
          View(
            {
              class: "test-description",
              attributes: { n: "hub-download-description" },
            },
            [
              "使用上一步回传的 content 创建 download.create 任务；成功后目标电脑会创建并自动启动本地下载任务。",
            ],
          ),
          View(
            {
              class: "test-form download-form",
              attributes: { n: "hub-download-form" },
            },
            [
              View(
                {
                  class: "test-field",
                  attributes: { n: "hub-download-client-field" },
                },
                [
                  "下载电脑",
                  Select({
                    key: "value",
                    class: "test-control",
                    value: target_client_id_,
                    options: download_options,
                    attributes: {
                      n: "hub-download-client-select",
                      "aria-label": "下载电脑",
                    },
                    onChange(event) {
                      vm$.methods.setDownloadDraft(
                        hub_id,
                        "target_client_id",
                        String(event.target.value),
                      );
                    },
                  }),
                ],
              ),
              View(
                {
                  class: "test-field",
                  attributes: { n: "hub-download-directory-field" },
                },
                [
                  "下载目录（可选）",
                  Input({
                    class: "test-control",
                    value: download_dir_,
                    placeholder: "留空使用目标电脑默认目录",
                    attributes: {
                      n: "hub-download-directory-input",
                      "aria-label": "下载目录（可选）",
                    },
                    onInput(event) {
                      vm$.methods.setDownloadDraft(
                        hub_id,
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
                  attributes: { n: "hub-download-filename-field" },
                },
                [
                  "文件名（可选）",
                  Input({
                    class: "test-control",
                    value: filename_,
                    placeholder: "留空使用内容标题",
                    attributes: {
                      n: "hub-download-filename-input",
                      "aria-label": "文件名（可选）",
                    },
                    onInput(event) {
                      vm$.methods.setDownloadDraft(
                        hub_id,
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
                    n: "hub-download-submit-button",
                  },
                  disabled: disabled_,
                  onClick() {
                    return vm$.methods.runDownloadTest(hub_id);
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
  const hub = props.hub;
  const hub_id = String(hub.id);
  const test_clients = Array.isArray(hub.test_clients) ? hub.test_clients : [];
  const test_options =
    test_clients.length === 0
      ? [{ value: "", label: "没有在线且支持 wxchannels.fetch 的电脑" }]
      : test_clients.map((client) => ({
          value: String(client.client_id),
          label:
            String(client.client_id) +
            (client.status === "busy" ? "（正在执行任务）" : "（在线）"),
        }));
  const active_ = computed(props.test, (test) =>
    Boolean(
      test.submitting ||
      (test.task && !["completed", "failed"].includes(test.task.status)),
    ),
  );
  const disabled_ = computed(
    active_,
    (active) => active || test_clients.length === 0,
  );
  const target_client_id_ = computed(props.draft, (draft) =>
    String(draft.target_client_id || ""),
  );
  const url_ = computed(props.draft, (draft) => String(draft.url || ""));
  const submit_label_ = computed(active_, (active) =>
    active ? "测试中…" : "创建测试任务",
  );

  return View(
    {
      class: "test-panel",
      attributes: { n: "hub-test-panel" },
    },
    [
      View(
        {
          class: "test-title",
          attributes: { n: "hub-test-title" },
        },
        ["测试 wxchannels.fetch 能力"],
      ),
      View(
        {
          class: "test-description",
          attributes: { n: "hub-test-description" },
        },
        [
          "创建真实任务并观察目标电脑是否领取、获取视频号数据并把结果提交回 Hub。",
        ],
      ),
      View(
        {
          class: "test-form",
          attributes: { n: "hub-test-form" },
        },
        [
          View(
            {
              class: "test-field",
              attributes: { n: "hub-test-client-field" },
            },
            [
              "目标电脑",
              Select({
                key: "value",
                class: "test-control",
                value: target_client_id_,
                options: test_options,
                attributes: {
                  n: "hub-test-client-select",
                  "aria-label": "目标电脑",
                },
                onChange(event) {
                  vm$.methods.setTestDraft(
                    hub_id,
                    "target_client_id",
                    String(event.target.value),
                  );
                },
              }),
            ],
          ),
          View(
            {
              class: "test-field",
              attributes: { n: "hub-test-url-field" },
            },
            [
              "视频号 URL",
              Input({
                class: "test-control",
                value: url_,
                placeholder: "https://channels.weixin.qq.com/...",
                type: "url",
                attributes: {
                  n: "hub-test-url-input",
                  "aria-label": "视频号 URL",
                },
                onInput(event) {
                  vm$.methods.setTestDraft(
                    hub_id,
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
                n: "hub-test-submit-button",
              },
              disabled: disabled_,
              onClick() {
                return vm$.methods.runTest(hub_id);
              },
            },
            [submit_label_],
          ),
        ],
      ),
      TestResultView({ test: props.test }),
      DownloadPanelView({
        store: vm$,
        hub,
        test: props.test,
        draft: props.download_draft,
      }),
    ],
  );
}

function ClientView(props) {
  const client = props.client;
  const status =
    client.status === "busy" || client.status === "online"
      ? client.status
      : "offline";
  const status_labels = {
    online: "在线",
    busy: "执行任务",
    offline: "离线",
  };
  const time_text =
    status === "offline"
      ? "最近活跃 " +
        format_time(client.last_seen_at) +
        " · 离线于 " +
        format_time(client.disconnected_at)
      : "连接于 " +
        format_time(client.connected_at) +
        " · 最近活跃 " +
        format_time(client.last_seen_at);
  const capabilities = Array.isArray(client.capabilities)
    ? client.capabilities
    : [];

  return View(
    {
      class: "client " + status,
      attributes: {
        n: "client-row",
        "data-client-id": String(client.client_id),
      },
    },
    [
      View(
        {
          attributes: { n: "client-identity" },
        },
        [
          View(
            {
              class: "client-id",
              attributes: { n: "client-id" },
            },
            [String(client.client_id)],
          ),
          View(
            {
              class: "client-time",
              attributes: { n: "client-activity-time" },
            },
            [time_text],
          ),
        ],
      ),
      View(
        {
          class: "capabilities",
          attributes: { n: "client-capabilities" },
        },
        [
          Show({
            when: capabilities.length === 0,
            ok() {
              return View(
                {
                  class: "capability",
                  attributes: { n: "empty-capability" },
                },
                ["仅发布任务"],
              );
            },
          }),
          For({
            each: capabilities,
            render(value) {
              return View(
                {
                  class: "capability",
                  attributes: { n: "client-capability" },
                },
                [String(source_value(value))],
              );
            },
          }),
        ],
      ),
      View(
        {
          class: "connection-status " + status,
          attributes: { n: "client-connection-status" },
        },
        [status_labels[status]],
      ),
    ],
  );
}

function HubView(props) {
  const vm$ = props.store;
  const hub = props.hub;
  const hub_id = String(hub.id);
  const clients = Array.isArray(hub.clients) ? hub.clients : [];
  const task_counts = Array.isArray(hub.task_counts) ? hub.task_counts : [];
  const test_ = computed(vm$.state.tests, (tests) => tests[hub_id] || {});
  const test_draft_ = computed(
    vm$.state.test_drafts,
    (drafts) => drafts[hub_id] || { target_client_id: "", url: "" },
  );
  const download_draft_ = computed(
    vm$.state.download_drafts,
    (drafts) =>
      drafts[hub_id] || {
        target_client_id: "",
        download_dir: "",
        filename: "",
      },
  );

  return View(
    {
      class: "hub-card",
      attributes: {
        n: "hub-card",
        "data-hub-id": hub_id,
      },
    },
    [
      View(
        {
          class: "hub-header",
          attributes: { n: "hub-card-header" },
        },
        [
          View(
            {
              attributes: { n: "hub-card-heading" },
            },
            [
              View(
                {
                  class: "hub-title",
                  attributes: { n: "hub-name" },
                },
                [hub_id],
              ),
              View(
                {
                  class: "hub-meta",
                  attributes: { n: "hub-last-seen" },
                },
                ["最近登记：" + format_time(hub.last_seen_at)],
              ),
            ],
          ),
          View(
            {
              class: "online",
              attributes: { n: "hub-online-count" },
            },
            [hub.online_count + " 台在线 · " + hub.offline_count + " 台离线"],
          ),
        ],
      ),
      Show({
        when: Boolean(hub.error),
        ok() {
          return View(
            {
              class: "error visible",
              attributes: {
                n: "hub-error",
                role: "alert",
              },
            },
            [String(hub.error || "")],
          );
        },
      }),
      View(
        {
          class: "task-counts",
          attributes: { n: "hub-task-counts" },
        },
        [
          Show({
            when: task_counts.length === 0,
            ok() {
              return View(
                {
                  class: "task-badge",
                  attributes: { n: "empty-task-count" },
                },
                ["暂无任务"],
              );
            },
          }),
          For({
            each: task_counts,
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
      TestPanelView({
        store: vm$,
        hub,
        test: test_,
        draft: test_draft_,
        download_draft: download_draft_,
      }),
      View(
        {
          class: "clients",
          attributes: { n: "hub-clients" },
        },
        [
          Show({
            when: clients.length === 0,
            ok() {
              return View(
                {
                  class: "empty",
                  attributes: { n: "empty-client-list" },
                },
                ["尚无电脑连接记录"],
              );
            },
          }),
          For({
            key: "client_id",
            each: clients,
            render(client_value) {
              return ClientView({ client: source_value(client_value) });
            },
          }),
        ],
      ),
    ],
  );
}

function HubListView(props) {
  const vm$ = props.store;

  return View(
    {
      class: "hub-list",
      attributes: {
        n: "hub-list",
        "aria-label": "Hub 列表",
      },
    },
    [
      Show({
        when: computed(vm$.state.hubs, (hubs) => hubs.length === 0),
        ok() {
          return View(
            {
              class: "empty",
              attributes: { n: "empty-hub-list" },
            },
            ["尚未发现 Hub。启动至少一个已配置的客户端后会自动登记。"],
          );
        },
      }),
      For({
        key: "id",
        each: vm$.state.hubs,
        render(hub_value) {
          return HubView({
            store: vm$,
            hub: source_value(hub_value),
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
                ["WX Channels Hub"],
              ),
              View(
                {
                  class: "subtitle",
                  attributes: { n: "dashboard-subtitle" },
                },
                [
                  "跨 Hub 查看电脑在线、执行任务和离线状态，以及能力声明、活跃时间和任务统计。",
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
      HubListView({ store: vm$ }),
      View(
        {
          class: "dashboard-footer",
          attributes: { n: "dashboard-footer" },
        },
        ["页面每 5 秒自动刷新 · 离线电脑记录会继续保留"],
      ),
    ],
  );
}

function bootstrap() {
  const root_node = document.querySelector('[data-n="dashboard-root"]');
  if (!root_node) {
    throw new Error("Hub 管理页无法启动：缺少根节点");
  }
  Timeless.DOM.render(ApplicationView(), root_node);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap, { once: true });
} else {
  bootstrap();
}

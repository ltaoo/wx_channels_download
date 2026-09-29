import { request } from "@/biz/request.js";

const STATUS_TIMEOUT = 5000;

const REMOTE_SERVICE_LABELS = {
  running: "运行中",
  stopped: "已停止",
  stopping: "停止中",
  error: "异常",
};

const DEVICE_STATUS_LABELS = {
  checking: "检测中",
  online: "在线",
  offline: "离线",
};

function result_message(data) {
  const value = data || {};
  return `已同步 ${value.contents || 0} 条内容、${value.tasks || 0} 个任务、${value.resources || 0} 个资源、${value.files || 0} 个文件`;
}

function version_label(value) {
  const version = String(value || "").trim();
  if (!version) return "";
  return /^v/i.test(version) ? version : `v${version}`;
}

function status_error_message(error) {
  if (!error || error.name === "AbortError") return "连接超时";
  if (error.name === "TypeError") return "无法连接";
  return String(error.message || error);
}

function remote_service_detail(data) {
  const parts = [];
  const api_status = data && data.api && data.api.status;
  const proxy_status = data && data.proxy && data.proxy.status;
  if (api_status) {
    parts.push(`API ${REMOTE_SERVICE_LABELS[api_status] || api_status}`);
  }
  if (proxy_status) {
    parts.push(`代理 ${REMOTE_SERVICE_LABELS[proxy_status] || proxy_status}`);
  }
  return parts.join(" · ");
}

export function SyncPageModel(props) {
  const servers_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const keyword_ = ref("");
  const status_controllers = new Map();
  const visible_servers_ = combine(
    { servers: servers_, keyword: keyword_ },
    (state) => {
      const needle = String(state.keyword || "")
        .trim()
        .toLowerCase();
      if (!needle) return state.servers;
      return state.servers.filter((server) =>
        `${server.name} ${server.url}`.toLowerCase().includes(needle),
      );
    },
  );
  const empty_message_ = combine(
    { loading: loading_, servers: servers_, keyword: keyword_ },
    (state) => {
      if (state.loading) return "正在读取服务配置…";
      const needle = String(state.keyword || "").trim();
      if (state.servers.length > 0 && needle) {
        return `没有匹配「${needle}」的下载服务`;
      }
      return "尚未配置其他下载服务";
    },
  );
  const list_request = new Timeless.kit.RequestCore(
    () => request.get("/api/v1/sync/servers"),
    { client: props.client },
  );
  const sync_request = new Timeless.kit.RequestCore(
    (server_id) => request.post(`/api/v1/sync/servers/${server_id}`),
    { client: props.client },
  );
  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      placeholder: "搜索服务名称或地址",
      type: "search",
      allowClear: true,
      onChange(value) {
        keyword_.as(String(value || ""));
      },
    }),
    refresh_button$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick: load,
    }),
  };

  function destroy_servers() {
    servers_.value.forEach((server) => server.button$.destroy());
  }

  function set_buttons_enabled(enabled) {
    servers_.value.forEach((server) => {
      if (enabled) server.button$.enable();
      else server.button$.disable();
    });
  }

  function device_status_class_names(status) {
    const modifier = status === "unknown" ? "is-hidden" : `is-${status}`;
    return `sync-server-card__chip sync-server-card__status ${modifier}`;
  }

  function make_server(raw) {
    const status_ = ref("idle");
    const message_ = ref("");
    const version_ = ref("");
    const device_status_ = ref("unknown");
    const device_message_ = ref("");
    const server = {
      id: Number(raw.id),
      name: String(raw.name || "未命名服务"),
      url: String(raw.url || ""),
      status: status_,
      message: message_,
      message_class: computed(status_, (status) =>
        status === "idle" ? "" : `is-${status}`,
      ),
      version: version_,
      version_text: computed(version_, version_label),
      version_class: computed(
        version_,
        (version) =>
          `sync-server-card__chip sync-server-card__version${
            version ? "" : " is-hidden"
          }`,
      ),
      device_status: device_status_,
      device_message: device_message_,
      device_status_label: computed(
        device_status_,
        (status) => DEVICE_STATUS_LABELS[status] || "",
      ),
      device_status_class: computed(device_status_, device_status_class_names),
      device_status_title: computed(device_message_, (message) => message || ""),
      button$: null,
    };
    server.button$ = new Timeless.vm.ButtonCore({
      variant: "primary",
      onClick() {
        return sync_to(server);
      },
    });
    return server;
  }

  function abort_status_checks() {
    status_controllers.forEach((controller) => controller.abort());
    status_controllers.clear();
  }

  async function check_server_status(server) {
    const controller = new AbortController();
    status_controllers.set(server.id, controller);
    const timer = setTimeout(() => controller.abort(), STATUS_TIMEOUT);
    server.device_status.as("checking");
    server.device_message.as("");
    try {
      const response = await fetch(`${server.url}/api/status`, {
        cache: "no-store",
        headers: { Accept: "application/json" },
        signal: controller.signal,
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const payload = await response.json();
      if (payload && payload.code !== undefined && payload.code !== 0) {
        throw new Error(payload.msg || "设备返回异常状态");
      }
      const data = (payload && payload.data) || {};
      server.version.as(String(data.version || ""));
      server.device_status.as("online");
      server.device_message.as(remote_service_detail(data));
    } catch (error) {
      server.version.as("");
      server.device_status.as("offline");
      server.device_message.as(status_error_message(error));
    } finally {
      clearTimeout(timer);
      if (status_controllers.get(server.id) === controller) {
        status_controllers.delete(server.id);
      }
    }
  }

  function check_servers_status() {
    abort_status_checks();
    servers_.value.forEach((server) => {
      check_server_status(server);
    });
  }

  async function load() {
    if (loading_.value) return null;
    loading_.as(true);
    error_.as("");
    ui.refresh_button$.setLoading(true);
    const response = await list_request.run();
    ui.refresh_button$.setLoading(false);
    loading_.as(false);
    if (response.error) {
      error_.as(response.error.message || String(response.error));
      return response;
    }
    destroy_servers();
    const data = response.data || {};
    servers_.as((data.servers || []).map(make_server), { reset: true });
    check_servers_status();
    return response;
  }

  async function sync_to(server) {
    if (loading_.value) return null;
    loading_.as(true);
    error_.as("");
    set_buttons_enabled(false);
    server.button$.setLoading(true);
    server.status.as("syncing");
    server.message.as("正在上传记录和文件…");
    const response = await sync_request.run(server.id);
    server.button$.setLoading(false);
    set_buttons_enabled(true);
    loading_.as(false);
    if (response.error) {
      server.status.as("error");
      server.message.as(response.error.message || String(response.error));
      return response;
    }
    server.status.as("success");
    server.message.as(result_message(response.data));
    return response;
  }

  function destroy() {
    abort_status_checks();
    destroy_servers();
    list_request.destroy();
    sync_request.destroy();
    ui.refresh_button$.destroy();
  }

  return {
    state: {
      servers: servers_,
      visible_servers: visible_servers_,
      loading: loading_,
      error: error_,
      empty_message: empty_message_,
    },
    ui,
    methods: { ready: load, destroy },
  };
}

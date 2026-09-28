import { request } from "@/biz/request.js";

function result_message(data) {
  const value = data || {};
  return `已同步 ${value.contents || 0} 条内容、${value.tasks || 0} 个任务、${value.resources || 0} 个资源、${value.files || 0} 个文件`;
}

export function SyncPageModel(props) {
  const servers_ = refarr([]);
  const loading_ = ref(false);
  const error_ = ref("");
  const empty_message_ = computed(loading_, (loading) =>
    loading ? "正在读取服务配置…" : "尚未配置其他下载服务",
  );
  const list_request = new Timeless.kit.RequestCore(
    () => request.get("/api/v1/sync/servers"),
    { client: props.client },
  );
  const sync_request = new Timeless.kit.RequestCore(
    (server_id) => request.post(`/api/v1/sync/servers/${server_id}`),
    { client: props.client },
  );
  const refresh_button$ = new Timeless.vm.ButtonCore({
    variant: "outline",
    onClick: load,
  });

  function destroy_servers() {
    servers_.value.forEach((server) => server.button$.destroy());
  }

  function set_buttons_enabled(enabled) {
    servers_.value.forEach((server) => {
      if (enabled) server.button$.enable();
      else server.button$.disable();
    });
  }

  function make_server(raw) {
    const status_ = ref("idle");
    const message_ = ref("");
    const server = {
      id: Number(raw.id),
      name: String(raw.name || "未命名服务"),
      url: String(raw.url || ""),
      status: status_,
      message: message_,
      message_class: computed(status_, (status) =>
        status === "idle" ? "" : `is-${status}`,
      ),
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

  async function load() {
    if (loading_.value) return null;
    loading_.as(true);
    error_.as("");
    refresh_button$.setLoading(true);
    const response = await list_request.run();
    refresh_button$.setLoading(false);
    loading_.as(false);
    if (response.error) {
      error_.as(response.error.message || String(response.error));
      return response;
    }
    destroy_servers();
    const data = response.data || {};
    servers_.as((data.servers || []).map(make_server), { reset: true });
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
    destroy_servers();
    list_request.destroy();
    sync_request.destroy();
    refresh_button$.destroy();
  }

  return {
    state: {
      servers: servers_,
      loading: loading_,
      error: error_,
      empty_message: empty_message_,
    },
    ui: { refresh_button$ },
    methods: { ready: load, destroy },
  };
}

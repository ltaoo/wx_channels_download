import {
  applySandboxCDP,
  applySandboxSession,
  createSandbox,
  destroySandbox,
  diagnoseSandboxCDP,
  fetchSandboxBrowserContent,
  fetchSandboxDetail,
  fetchSandboxList,
  pauseSandbox,
  refreshSandboxStatus,
  restartSandboxBrowser,
  resumeSandbox,
  runSandboxBrowserActions,
  screenshotSandboxBrowser,
  updateSandboxAlias,
} from "@/biz/request.js";

function endpointOf(rec) {
  return rec?.endpoint || {};
}

function isRunningStatus(status) {
  return status === "running" || status === "idle" || status === "busy";
}

function isPausedStatus(status) {
  return status === "paused" || status === "stopped";
}

function isCreatingStatus(status) {
  return status === "creating";
}

function desktopEndpointOf(rec) {
  const ep = endpointOf(rec);
  return ep.session_url || ep.desktop_url || "";
}

function deviceNameOf(rec) {
  const device = rec?.device || {};
  return device.name || device.id || "未知设备";
}

function deviceLabel(rec) {
  if (!rec?.device_bound) return "不限设备";
  const name = deviceNameOf(rec);
  return rec.local_device ? `本机 ${name}` : name;
}

function deviceDetail(rec) {
  if (!rec?.device_bound) return "不限设备";
  const name = deviceNameOf(rec);
  return rec.local_device ? `本机 ${name}` : `${name} (非本机)`;
}

function isDeviceUsable(rec) {
  return !rec?.device_bound || rec.local_device === true;
}

function statusLabel(status) {
  const labels = {
    creating: "启动中",
    idle: "空闲",
    busy: "使用中",
    invalid: "无效",
    running: "运行中",
    paused: "已暂停",
    stopped: "已暂停",
    error: "异常",
    unavailable: "非本机",
    destroyed: "已删除",
  };
  return labels[status] || status || "-";
}

function statusClass(status) {
  if (status === "idle" || status === "running") {
    return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  }
  if (status === "creating") {
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  }
  if (status === "busy") {
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  }
  if (isPausedStatus(status)) {
    return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
  }
  if (status === "unavailable") {
    return "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300";
  }
  return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
}

function shortID(id) {
  const value = String(id || "");
  return value.length > 12 ? value.slice(0, 12) : value;
}

function parseOptionalPort(value) {
  const text = String(value || "").trim();
  if (!text) return 0;
  const n = Number(text);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}

function errorText(error) {
  return error?.message || error?.msg || String(error || "未知错误");
}

function normalizeList(data) {
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.list)) return data.list;
  return [];
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function sandboxSessionURL(id, ticket) {
  return `${location.protocol}//${location.host}/api/v1/sandboxes/${id}/session/vnc_lite.html?ticket=${encodeURIComponent(ticket)}`;
}

/**
 * Sandbox management model
 * @param {ViewComponentProps} props
 */
export function SandboxModel(props) {
  const reqs = {
    list: new Timeless.RequestCore(fetchSandboxList, {
      client: props.client,
    }),
    create: new Timeless.RequestCore(createSandbox, {
      client: props.client,
    }),
    detail: new Timeless.RequestCore(fetchSandboxDetail, {
      client: props.client,
    }),
    update: new Timeless.RequestCore(updateSandboxAlias, {
      client: props.client,
    }),
    destroy: new Timeless.RequestCore(destroySandbox, {
      client: props.client,
    }),
    pause: new Timeless.RequestCore(pauseSandbox, {
      client: props.client,
    }),
    resume: new Timeless.RequestCore(resumeSandbox, {
      client: props.client,
    }),
    refreshStatus: new Timeless.RequestCore(refreshSandboxStatus, {
      client: props.client,
    }),
    restartBrowser: new Timeless.RequestCore(restartSandboxBrowser, {
      client: props.client,
    }),
    screenshot: new Timeless.RequestCore(screenshotSandboxBrowser, {
      client: props.client,
    }),
    actions: new Timeless.RequestCore(runSandboxBrowserActions, {
      client: props.client,
    }),
    content: new Timeless.RequestCore(fetchSandboxBrowserContent, {
      client: props.client,
    }),
    diagnoseCDP: new Timeless.RequestCore(diagnoseSandboxCDP, {
      client: props.client,
    }),
    applyCDP: new Timeless.RequestCore(applySandboxCDP, {
      client: props.client,
    }),
    applySession: new Timeless.RequestCore(applySandboxSession, {
      client: props.client,
    }),
  };

  const methods = {};

  const state = {
    list: refarr([]),
    selected: ref(null),
    vncTicket: ref(""),
    sessionLoading: ref(false),
    loading: ref(false),
    statusRefreshing: ref(false),
    error: ref(""),
    createMode: ref("docker"),
    dockerAlias: ref("69shuba-browser"),
    dockerImage: ref(""),
    cdpPort: ref(""),
    desktopPort: ref(""),
    localAlias: ref("existing-sandbox"),
    localPreviewURL: ref("http://127.0.0.1:39017"),
    localCDPURL: ref("http://127.0.0.1:39228"),
  };

  state.empty = computed(state.list, (list) => list.length === 0);
  state.createButtonText = computed(
    { loading: state.loading, mode: state.createMode },
    ({ loading, mode }) => {
      if (loading) return "提交中...";
      return mode === "local" ? "添加到管理" : "创建并加入池";
    },
  );
  state.previewURL = computed(
    { selected: state.selected, ticket: state.vncTicket },
    ({ selected, ticket }) => {
      if (!selected?.id || !ticket || !isRunningStatus(selected.status)) {
        return "";
      }
      return sandboxSessionURL(selected.id, ticket);
    },
  );
  state.previewEndpoint = computed(state.selected, desktopEndpointOf);
  state.previewMessage = computed(
    { selected: state.selected, sessionLoading: state.sessionLoading },
    ({ selected, sessionLoading }) => {
      if (!selected) return "选择一个浏览器容器查看桌面";
      if (isCreatingStatus(selected.status)) return "Sandbox 正在启动";
      if (selected.status === "unavailable") return "这个 sandbox 不在当前设备上";
      if (isPausedStatus(selected.status)) return "Sandbox 已暂停";
      if (sessionLoading) return "正在连接桌面";
      if (!desktopEndpointOf(selected)) return "这个 sandbox 没有桌面预览地址";
      if (!isRunningStatus(selected.status)) return "Sandbox 不可用";
      return "正在准备桌面预览";
    },
  );
  state.refreshStatusButtonText = computed(state.statusRefreshing, (loading) =>
    loading ? "刷新中..." : "刷新状态",
  );

  let initialized = false;

  function tipError(title, error) {
    const text = errorText(error);
    state.error.as(text);
    props.app.tip?.({ type: "error", text: [title, text] });
  }

  function syncSelected(list) {
    const current = state.selected.value;
    if (!current) return;
    const next = (list || []).find((item) => item.id === current.id);
    state.selected.as(next || null);
  }

  function createBody() {
    if (state.createMode.value === "local") {
      return {
        kind: "existing",
        alias: state.localAlias.value,
        desktop_url: state.localPreviewURL.value,
        session_url: state.localPreviewURL.value,
        cdp_url: state.localCDPURL.value,
      };
    }
    return {
      kind: "docker",
      alias: state.dockerAlias.value,
      image: state.dockerImage.value,
      cdp_host_port: parseOptionalPort(state.cdpPort.value),
      desktop_host_port: parseOptionalPort(state.desktopPort.value),
    };
  }

  const ui = {
    refreshBtn: new Timeless.ui.ButtonCore({
      variant: "outline",
      size: "sm",
      onClick() {
        methods.refreshList();
      },
    }),
    dockerModeBtn: new Timeless.ui.ButtonCore({
      variant: computed(state.createMode, (value) =>
        value === "docker" ? "default" : "outline",
      ),
      size: "sm",
      onClick() {
        methods.setCreateMode("docker");
      },
    }),
    localModeBtn: new Timeless.ui.ButtonCore({
      variant: computed(state.createMode, (value) =>
        value === "local" ? "default" : "outline",
      ),
      size: "sm",
      onClick() {
        methods.setCreateMode("local");
      },
    }),
    dockerAliasInput: new Timeless.ui.InputCore({
      defaultValue: state.dockerAlias.value,
      placeholder: "69shuba-browser",
      onChange(value) {
        state.dockerAlias.as(value);
      },
    }),
    dockerImageInput: new Timeless.ui.InputCore({
      defaultValue: state.dockerImage.value,
      placeholder: "默认: lscr.io/linuxserver/chromium:latest",
      onChange(value) {
        state.dockerImage.as(value);
      },
    }),
    cdpPortInput: new Timeless.ui.InputCore({
      defaultValue: state.cdpPort.value,
      placeholder: "自动",
      onChange(value) {
        state.cdpPort.as(value);
      },
    }),
    desktopPortInput: new Timeless.ui.InputCore({
      defaultValue: state.desktopPort.value,
      placeholder: "自动",
      onChange(value) {
        state.desktopPort.as(value);
      },
    }),
    localAliasInput: new Timeless.ui.InputCore({
      defaultValue: state.localAlias.value,
      placeholder: "existing-sandbox",
      onChange(value) {
        state.localAlias.as(value);
      },
    }),
    localPreviewURLInput: new Timeless.ui.InputCore({
      defaultValue: state.localPreviewURL.value,
      placeholder: "http://127.0.0.1:39017",
      onChange(value) {
        state.localPreviewURL.as(value);
      },
    }),
    localCDPURLInput: new Timeless.ui.InputCore({
      defaultValue: state.localCDPURL.value,
      placeholder: "http://127.0.0.1:39228",
      onChange(value) {
        state.localCDPURL.as(value);
      },
    }),
    createBtn: new Timeless.ui.ButtonCore({
      onClick() {
        methods.createSandbox();
      },
    }),
  };

  Object.assign(methods, {
    init() {
      if (initialized) return null;
      initialized = true;
      return methods.refreshList();
    },

    setCreateMode(mode) {
      state.createMode.as(mode);
    },

    async refreshList(options = {}) {
      if (state.loading.value && options.force !== true) return null;
      if (!options.silent) state.loading.as(true);
      state.error.as("");
      const result = await reqs.list.run();
      if (!options.silent) state.loading.as(false);
      if (result.error) {
        if (!options.silent) tipError("加载 sandbox 失败", result.error);
        return result;
      }
      const list = normalizeList(result.data);
      state.list.as(list);
      syncSelected(list);
      return result;
    },

    async createSandbox() {
      if (state.loading.value) return null;
      state.loading.as(true);
      state.error.as("");
      const result = await reqs.create.run(createBody());
      state.loading.as(false);
      if (result.error) {
        tipError("创建失败", result.error);
        return result;
      }
      props.app.tip?.({
        text: [
          state.createMode.value === "local"
            ? "已有 Sandbox 已加入管理"
            : "Sandbox 启动任务已创建",
        ],
      });
      await methods.refreshList({ force: true });
      if (result.data?.id) {
        state.selected.as(result.data);
        if (isCreatingStatus(result.data.status)) {
          methods.pollSandboxStart(result.data.id);
        }
      }
      return result;
    },

    async destroySandbox(id) {
      if (!id) return null;
      if (!globalThis.confirm?.("删除这个 sandbox？容器会被强制移除。")) {
        return null;
      }
      const result = await reqs.destroy.run({ id });
      if (result.error) {
        tipError("删除失败", result.error);
        return result;
      }
      if (state.selected.value?.id === id) state.selected.as(null);
      props.app.tip?.({ text: ["Sandbox 已删除"] });
      await methods.refreshList({ force: true });
      return result;
    },

    async toggleStop(rec) {
      if (!rec?.id) return null;
      const result =
        isPausedStatus(rec.status)
          ? await reqs.resume.run(rec.id)
          : await reqs.pause.run(rec.id);
      if (result.error) {
        tipError("操作失败", result.error);
        return result;
      }
      state.vncTicket.as("");
      await methods.refreshList({ force: true });
      const next = state.list.value.find((item) => item.id === rec.id);
      if (next && isRunningStatus(next.status)) {
        await methods.selectSandbox(next);
      }
      return result;
    },

    async restartBrowser(rec) {
      if (!rec?.id) return null;
      const result = await reqs.restartBrowser.run(rec.id);
      if (result.error) {
        tipError("重启失败", result.error);
        return result;
      }
      props.app.tip?.({ text: ["浏览器已重启"] });
      await methods.refreshList({ force: true });
      return result;
    },

    async refreshSandboxStatus(rec) {
      if (!rec?.id || state.statusRefreshing.value) return null;
      state.statusRefreshing.as(true);
      const result = await reqs.refreshStatus.run(rec.id);
      state.statusRefreshing.as(false);
      if (result.error) {
        tipError("刷新状态失败", result.error);
        return result;
      }
      const updated = result.data;
      if (updated?.id) {
        state.list.as(
          state.list.value.map((item) =>
            item.id === updated.id ? updated : item,
          ),
        );
        if (state.selected.value?.id === updated.id) {
          state.selected.as(updated);
          state.vncTicket.as("");
          if (isRunningStatus(updated.status) && desktopEndpointOf(updated)) {
            await methods.selectSandbox(updated);
          }
        }
        props.app.tip?.({
          text: ["Sandbox 状态已刷新", statusLabel(updated.status)],
        });
      } else {
        await methods.refreshList({ force: true });
      }
      return result;
    },

    async selectSandbox(rec) {
      state.selected.as(rec);
      state.vncTicket.as("");
      if (!rec?.id || !isRunningStatus(rec.status) || !desktopEndpointOf(rec)) {
        return null;
      }
      state.sessionLoading.as(true);
      const result = await reqs.applySession.run({
        id: rec.id,
        opts: { ttl_sec: 3600 },
      });
      state.sessionLoading.as(false);
      if (state.selected.value?.id !== rec.id) {
        return result;
      }
      if (result.error) {
        tipError("桌面授权失败", result.error);
        return result;
      }
      state.vncTicket.as(result.data?.ticket || "");
      return result;
    },

    async takeScreenshot() {
      const rec = state.selected.value;
      if (!rec?.id || !isRunningStatus(rec.status) || !endpointOf(rec).cdp_url) {
        return null;
      }
      const result = await reqs.screenshot.run({
        id: rec.id,
        opts: { format: "png" },
      });
      if (result.error) {
        tipError("截图失败", result.error);
        return result;
      }
      if (result.data?.data) {
        const w = window.open("", "_blank");
        if (w) {
          w.document.write(
            `<img src="data:image/${result.data.format};base64,${result.data.data}" />`,
          );
        }
      }
      return result;
    },

    async navigateURL() {
      const rec = state.selected.value;
      if (!rec?.id || !isRunningStatus(rec.status) || !endpointOf(rec).cdp_url) {
        return null;
      }
      const url = globalThis.prompt?.("URL", "https://www.69shuba.com/");
      if (!url) return null;
      const result = await reqs.actions.run({
        id: rec.id,
        actions: [{ type: "navigate", url }],
      });
      if (result.error) {
        tipError("导航失败", result.error);
        return result;
      }
      props.app.tip?.({ text: ["已导航", url] });
      return result;
    },

    async openDesktop(rec) {
      if (!rec?.id) return null;
      let ticket = state.vncTicket.value;
      if (!ticket || state.selected.value?.id !== rec.id) {
        const result = await methods.selectSandbox(rec);
        if (result?.error) return result;
        ticket = state.vncTicket.value;
      }
      if (ticket) window.open(methods.getVncURL(rec.id, ticket), "_blank");
      return null;
    },

    async pollSandboxStart(id) {
      for (let i = 0; i < 60; i += 1) {
        await sleep(1000);
        const result = await methods.refreshList({ force: true, silent: true });
        if (result?.error) return result;
        const rec = state.list.value.find((item) => item.id === id);
        if (!rec || !isCreatingStatus(rec.status)) {
          if (rec && state.selected.value?.id === id) {
            state.selected.as(rec);
            if (isRunningStatus(rec.status)) await methods.selectSandbox(rec);
          }
          return result;
        }
      }
      return null;
    },

    getVncURL(id, ticket) {
      return sandboxSessionURL(id, ticket);
    },

    getCDPWebSocketURL(id, ticket) {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      return `${proto}://${location.host}/api/v1/sandboxes/${id}/cdp/browser?ticket=${ticket}`;
    },

    endpointOf,
    desktopEndpointOf,
    deviceLabel,
    deviceDetail,
    isDeviceUsable,
    isRunningStatus,
    isPausedStatus,
    isCreatingStatus,
    statusLabel,
    statusClass,
    shortID,
  });

  return { state, ui, methods, reqs };
}

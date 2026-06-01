/**
 * Sandbox API model
 */
export function SandboxModel(http) {
  const base = "/api/v1/sandboxes";

  return {
    async list() {
      const r = await http.get(base);
      return r.data;
    },

    async create(opts = {}) {
      const r = await http.post(base, opts);
      return r.data;
    },

    async get(id) {
      const r = await http.get(`${base}/${id}`);
      return r.data;
    },

    async update(id, alias) {
      const r = await http.patch(`${base}/${id}`, { alias });
      return r.data;
    },

    async destroy(id) {
      const r = await http.delete(`${base}/${id}`);
      return r.data;
    },

    async pause(id) {
      const r = await http.post(`${base}/${id}/pause`);
      return r.data;
    },

    async resume(id) {
      const r = await http.post(`${base}/${id}/resume`);
      return r.data;
    },

    async restartBrowser(id) {
      const r = await http.post(`${base}/${id}/browser/restart`);
      return r.data;
    },

    async screenshot(id, opts = {}) {
      const r = await http.post(`${base}/${id}/browser/screenshot`, opts);
      return r.data;
    },

    async actions(id, actions) {
      const r = await http.post(`${base}/${id}/browser/actions`, { actions });
      return r.data;
    },

    async content(id) {
      const r = await http.post(`${base}/${id}/browser/content`);
      return r.data;
    },

    async diagnoseCDP(id) {
      const r = await http.get(`${base}/${id}/browser/diagnose`);
      return r.data;
    },

    async applyCDP(id, opts = {}) {
      const r = await http.post(`${base}/${id}/cdp/apply`, opts);
      return r.data;
    },

    async applySession(id, opts = {}) {
      const r = await http.post(`${base}/${id}/session/apply`, opts);
      return r.data;
    },

    getVncURL(id, ticket) {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      return `${proto}://${location.host}/api/v1/sandboxes/${id}/session/vnc_lite.html?ticket=${ticket}`;
    },

    getCDPWebSocketURL(id, ticket) {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      return `${proto}://${location.host}/api/v1/sandboxes/${id}/cdp/browser?ticket=${ticket}`;
    },
  };
}

/**
 * Tools Page Model — URL请求 / Daemon管理 / App管理
 */

/** Simple fetch wrapper for backend APIs (no {code,msg,data} envelope) */
async function api(url, options = {}) {
  try {
    const resp = await fetch(url, {
      headers: { "Content-Type": "application/json", ...options.headers },
      ...options,
    });
    const data = await resp.json();
    return { ok: resp.ok, status: resp.status, data, error: resp.ok ? null : (data.error || `HTTP ${resp.status}`) };
  } catch (err) {
    return { ok: false, status: 0, data: null, error: err.message };
  }
}

// ==================== URL Request ====================

/** @param {string} url
 *  @param {Object} opts */
export async function savePage(url, opts = {}) {
  const body = { url, ...opts };
  return api("/api/v1/save", { method: "POST", body: JSON.stringify(body) });
}

/** @param {string} taskId */
export async function getTask(taskId) {
  return api(`/api/v1/tasks/${taskId}`);
}

/** @param {string} taskId */
export async function getTaskLogs(taskId) {
  return api(`/api/v1/tasks/${taskId}/logs`);
}

// ==================== Daemon Management ====================

/** @param {string} [name] */
export async function daemonStatus(name = "default") {
  return api(`/api/v1/daemons/status?name=${encodeURIComponent(name)}`);
}

/** @param {string} [name] */
export async function daemonStart(name = "default") {
  return api("/api/v1/daemons/start", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

/** @param {string} [name] */
export async function daemonStop(name = "default") {
  return api("/api/v1/daemons/stop", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

/** @param {string} [name] */
export async function daemonRestart(name = "default") {
  return api("/api/v1/daemons/restart", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

/** @param {string} name @param {Object} opts */
export async function remoteDaemonStart(name, opts = {}) {
  return api("/api/v1/daemons/remote/start", {
    method: "POST",
    body: JSON.stringify({ name, ...opts }),
  });
}

/** @param {string} name @param {string} browserId */
export async function remoteDaemonStop(name, browserId) {
  return api("/api/v1/daemons/remote/stop", {
    method: "POST",
    body: JSON.stringify({ name, browserId }),
  });
}

/** @param {string} [name] */
export async function daemonTabs(name = "default") {
  return api(`/api/v1/daemons/tabs?name=${encodeURIComponent(name)}`);
}

/** @param {string} [name] */
export async function daemonPageInfo(name = "default") {
  return api(`/api/v1/daemons/page-info?name=${encodeURIComponent(name)}`);
}

// ==================== Appstore Management ====================

/** @param {boolean} [all] */
export async function listApps(all = false) {
  const q = all ? "?all=true" : "";
  return api(`/api/v1/apps${q}`);
}

/** @param {string} appName */
export async function installApp(appName) {
  return api("/api/v1/apps/install", {
    method: "POST",
    body: JSON.stringify({ name: appName }),
  });
}

/** @param {string} appName */
export async function removeApp(appName) {
  return api("/api/v1/apps/remove", {
    method: "POST",
    body: JSON.stringify({ name: appName }),
  });
}

/** @param {string} [appName] — empty = update all */
export async function updateApp(appName) {
  return api("/api/v1/apps/update", {
    method: "POST",
    body: JSON.stringify({ name: appName || "" }),
  });
}

/** @param {string} appName */
export async function runApp(appName) {
  return api("/api/v1/apps/run", {
    method: "POST",
    body: JSON.stringify({ name: appName }),
  });
}

export async function listAppTasks() {
  return api("/api/v1/apps/tasks");
}

/** @param {string} taskId */
export async function getAppTask(taskId) {
  return api(`/api/v1/apps/tasks/${taskId}`);
}

// ==================== Daemon Debugger ====================

/** Get the Chrome DevTools WebSocket URL from the daemon discovery */
export async function discoverDebuggerURL() {
  return api("/api/v1/daemons/debugger-url");
}

/**
 * Fetch a URL via the daemon client (uses existing browser tab).
 * @param {string} url
 * @param {Object} [opts]
 * @param {string} [opts.name] - daemon name
 * @param {string} [opts.method] - HTTP method (GET/POST)
 * @param {Object<string,string>} [opts.headers]
 * @param {string} [opts.body]
 * @param {number} [opts.timeout]
 */
export async function fetchViaDaemon(url, opts = {}) {
  return api("/api/v1/daemons/fetch", {
    method: "POST",
    body: JSON.stringify({ url, ...opts }),
  });
}

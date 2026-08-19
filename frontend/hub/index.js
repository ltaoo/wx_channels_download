import { DurableObject } from "cloudflare:workers";

/**
 * @typedef {Object} Env
 * @property {DurableObjectNamespace<HubDurableObject>} HUBS
 * @property {string} HUB_TOKEN
 * @property {string} HUB_ADMIN_TOKEN
 */

/** @typedef {"queued" | "assigned" | "running" | "completed" | "failed"} TaskStatus */

/**
 * @typedef {Object} TaskRow
 * @property {string} id
 * @property {string} kind
 * @property {string} publisher_id
 * @property {string | null} target_client_id
 * @property {string} required_capability
 * @property {string | null} idempotency_key
 * @property {string} payload_json
 * @property {TaskStatus} status
 * @property {string | null} assigned_client_id
 * @property {string | null} lease_token
 * @property {number | null} lease_expires_at
 * @property {number} attempt_count
 * @property {string | null} result_json
 * @property {string | null} error_message
 * @property {number} created_at
 * @property {number} updated_at
 * @property {number | null} completed_at
 */

/**
 * @typedef {Object} SocketAttachment
 * @property {string} client_id
 * @property {string} connection_id
 * @property {string[]} capabilities
 * @property {number} [connected_at]
 */

/**
 * @typedef {Object} HubRegistryRow
 * @property {string} id
 * @property {number} last_seen_at
 */

/**
 * @typedef {Object} ClientRegistryRow
 * @property {string} client_id
 * @property {string} connection_id
 * @property {string} capabilities_json
 * @property {number} connected_at
 * @property {number} last_seen_at
 * @property {number | null} disconnected_at
 * @property {"online" | "offline"} status
 */

/**
 * @typedef {Object} CreateTaskBody
 * @property {string} [kind]
 * @property {string} [target_client_id]
 * @property {string} [required_capability]
 * @property {string} [idempotency_key]
 * @property {unknown} [payload]
 */

/**
 * @typedef {Object} ClientMessage
 * @property {string} [type]
 * @property {string} [task_id]
 * @property {string} [lease_token]
 * @property {unknown} [result]
 * @property {string} [error]
 * @property {boolean} [retryable]
 */

/**
 * @typedef {Object} TaskCountRow
 * @property {string} status
 * @property {number} count
 */

/**
 * @typedef {Object} BusyClientRow
 * @property {string} assigned_client_id
 */

/**
 * @typedef {Object} LeaseRow
 * @property {number | null} lease_expires_at
 */

/**
 * @typedef {Object} AdminTestBody
 * @property {string} hub_id
 * @property {string} target_client_id
 * @property {string} url
 */

/**
 * @typedef {Object} AdminDownloadBody
 * @property {string} hub_id
 * @property {string} target_client_id
 * @property {string} source_task_id
 * @property {string} [download_dir]
 * @property {string} [filename]
 */

/**
 * @typedef {Object} ClientSummary
 * @property {string} client_id
 * @property {string[]} capabilities
 * @property {"online" | "busy" | "offline"} status
 */

const MAX_BODY_BYTES = 1024 * 1024;
const MAX_ATTEMPTS = 10;
const LEASE_MILLISECONDS = 120_000;
const RETENTION_MILLISECONDS = 7 * 24 * 60 * 60 * 1000;
const MAINTENANCE_MILLISECONDS = 24 * 60 * 60 * 1000;
const VALID_TASK_KINDS = new Set(["wxchannels.fetch", "download.create"]);
const HUB_DIRECTORY_NAME = "__hub_directory__";

/**
 * @param {unknown} value
 * @param {number} [status]
 * @returns {Response}
 */
function json_response(value, status = 200) {
  return Response.json(value, {
    status,
    headers: { "Cache-Control": "no-store" },
  });
}

/**
 * @param {string} error
 * @param {number} status
 * @returns {Response}
 */
function error_response(error, status) {
  return json_response({ error }, status);
}

/**
 * @param {string} value
 * @returns {boolean}
 */
function valid_identifier(value) {
  return /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(value);
}

/**
 * @param {Request} request
 * @returns {string}
 */
function bearer_token(request) {
  const authorization = request.headers.get("Authorization") ?? "";
  return authorization.startsWith("Bearer ") ? authorization.slice(7) : "";
}

/**
 * @param {string} left
 * @param {string} right
 * @returns {boolean}
 */
function safe_equal(left, right) {
  const encoder = new TextEncoder();
  const left_bytes = encoder.encode(left);
  const right_bytes = encoder.encode(right);
  const length = Math.max(left_bytes.length, right_bytes.length);
  let difference = left_bytes.length ^ right_bytes.length;
  for (let index = 0; index < length; index += 1) {
    difference |= (left_bytes[index] ?? 0) ^ (right_bytes[index] ?? 0);
  }
  return difference === 0;
}

/**
 * @param {Request} request
 * @param {Env} env
 * @returns {boolean}
 */
function admin_authorized(request, env) {
  if (!env.HUB_ADMIN_TOKEN) {
    return false;
  }
  const authorization = request.headers.get("Authorization") ?? "";
  if (authorization.startsWith("Bearer ")) {
    return safe_equal(authorization.slice(7), env.HUB_ADMIN_TOKEN);
  }
  if (!authorization.startsWith("Basic ")) {
    return false;
  }
  try {
    const credentials = atob(authorization.slice(6));
    const separator = credentials.indexOf(":");
    if (separator < 0) {
      return false;
    }
    const username = credentials.slice(0, separator);
    const password = credentials.slice(separator + 1);
    return username === "admin" && safe_equal(password, env.HUB_ADMIN_TOKEN);
  } catch {
    return false;
  }
}

/** @returns {Response} */
function admin_auth_required() {
  return new Response("Authorization required", {
    status: 401,
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": "text/plain; charset=utf-8",
      "WWW-Authenticate": 'Basic realm="WX Channels Hub Admin", charset="UTF-8"',
    },
  });
}

/**
 * @param {Env} env
 * @param {string} hub_id
 * @returns {Promise<void>}
 */
async function register_hub(env, hub_id) {
  const directory = env.HUBS.getByName(HUB_DIRECTORY_NAME);
  const response = await directory.fetch("https://internal/directory/register", {
    method: "POST",
    body: hub_id,
  });
  if (!response.ok) {
    throw new Error(`failed to register hub ${hub_id}: ${response.status}`);
  }
}

/**
 * @param {Env} env
 * @returns {Promise<Response>}
 */
async function admin_overview(env) {
  const directory = env.HUBS.getByName(HUB_DIRECTORY_NAME);
  const directory_response = await directory.fetch("https://internal/directory/hubs");
  if (!directory_response.ok) {
    return error_response("failed to load hub directory", 500);
  }
  /** @type {{ hubs?: HubRegistryRow[] }} */
  const directory_value = await directory_response.json();
  const registered_hubs = Array.isArray(directory_value.hubs) ? directory_value.hubs : [];
  const hubs = await Promise.all(
    registered_hubs.map(async (registered_hub) => {
      try {
        const object = env.HUBS.getByName(registered_hub.id);
        const response = await object.fetch("https://internal/");
        if (!response.ok) {
          throw new Error(`status ${response.status}`);
        }
        /** @type {Record<string, unknown>} */
        const summary = await response.json();
        return {
          id: registered_hub.id,
          last_seen_at: registered_hub.last_seen_at,
          ...summary,
        };
      } catch (error) {
        return {
          id: registered_hub.id,
          last_seen_at: registered_hub.last_seen_at,
          clients: [],
          task_counts: [],
          error: error instanceof Error ? error.message : String(error),
        };
      }
    }),
  );
  return json_response({ generated_at: Date.now(), hubs });
}

/**
 * @param {Env} env
 * @param {Request} request
 * @returns {Promise<Response>}
 */
async function admin_create_test(env, request) {
  const content_length = Number(request.headers.get("Content-Length") ?? "0");
  if (content_length > 16 * 1024) {
    return error_response("request body is too large", 413);
  }
  /** @type {AdminTestBody} */
  let body;
  try {
    body = await request.json();
  } catch {
    return error_response("invalid JSON", 400);
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)) {
    return error_response("invalid JSON body", 400);
  }
  const hub_id = typeof body.hub_id === "string" ? body.hub_id.trim() : "";
  const target_client_id = typeof body.target_client_id === "string" ? body.target_client_id.trim() : "";
  const test_url = typeof body.url === "string" ? body.url.trim() : "";
  if (!valid_identifier(hub_id) || hub_id === HUB_DIRECTORY_NAME) {
    return error_response("invalid hub_id", 400);
  }
  if (!valid_identifier(target_client_id)) {
    return error_response("invalid target_client_id", 400);
  }
  if (test_url === "" || test_url.length > 4096) {
    return error_response("invalid url", 400);
  }
  try {
    const parsed_url = new URL(test_url);
    if (parsed_url.protocol !== "http:" && parsed_url.protocol !== "https:") {
      return error_response("url must use http or https", 400);
    }
  } catch {
    return error_response("invalid url", 400);
  }

  const object = env.HUBS.getByName(hub_id);
  const summary_response = await object.fetch("https://internal/");
  if (!summary_response.ok) {
    return error_response("failed to load hub clients", 502);
  }
  /** @type {{ clients?: ClientSummary[] }} */
  const summary = await summary_response.json();
  const clients = Array.isArray(summary.clients) ? summary.clients : [];
  const target_client = clients.find((client) => client.client_id === target_client_id);
  if (target_client === undefined) {
    return error_response("target client is not registered", 404);
  }
  if (target_client.status === "offline") {
    return error_response("target client is offline", 409);
  }
  if (!Array.isArray(target_client.capabilities) || !target_client.capabilities.includes("wxchannels.fetch")) {
    return error_response("target client does not provide wxchannels.fetch", 409);
  }

  return object.fetch("https://internal/tasks", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Hub-Client-ID": "admin-console",
    },
    body: JSON.stringify({
      kind: "wxchannels.fetch",
      target_client_id,
      required_capability: "wxchannels.fetch",
      idempotency_key: "admin-test-" + crypto.randomUUID(),
      payload: { url: test_url },
    }),
  });
}

/**
 * @param {Env} env
 * @param {Request} request
 * @returns {Promise<Response>}
 */
async function admin_create_download(env, request) {
  const content_length = Number(request.headers.get("Content-Length") ?? "0");
  if (content_length > 16 * 1024) {
    return error_response("request body is too large", 413);
  }
  /** @type {AdminDownloadBody} */
  let body;
  try {
    body = await request.json();
  } catch {
    return error_response("invalid JSON", 400);
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)) {
    return error_response("invalid JSON body", 400);
  }
  const hub_id = typeof body.hub_id === "string" ? body.hub_id.trim() : "";
  const target_client_id = typeof body.target_client_id === "string" ? body.target_client_id.trim() : "";
  const source_task_id = typeof body.source_task_id === "string" ? body.source_task_id.trim() : "";
  const download_dir = typeof body.download_dir === "string" ? body.download_dir.trim() : "";
  const filename = typeof body.filename === "string" ? body.filename.trim() : "";
  if (!valid_identifier(hub_id) || hub_id === HUB_DIRECTORY_NAME) {
    return error_response("invalid hub_id", 400);
  }
  if (!valid_identifier(target_client_id)) {
    return error_response("invalid target_client_id", 400);
  }
  if (!/^[A-Za-z0-9-]{1,128}$/.test(source_task_id)) {
    return error_response("invalid source_task_id", 400);
  }
  if (download_dir.length > 4096 || filename.length > 512) {
    return error_response("download path or filename is too long", 400);
  }

  const object = env.HUBS.getByName(hub_id);
  const summary_response = await object.fetch("https://internal/");
  if (!summary_response.ok) {
    return error_response("failed to load hub clients", 502);
  }
  /** @type {{ clients?: ClientSummary[] }} */
  const summary = await summary_response.json();
  const clients = Array.isArray(summary.clients) ? summary.clients : [];
  const target_client = clients.find((client) => client.client_id === target_client_id);
  if (target_client === undefined) {
    return error_response("target client is not registered", 404);
  }
  if (target_client.status === "offline") {
    return error_response("target client is offline", 409);
  }
  if (!Array.isArray(target_client.capabilities) || !target_client.capabilities.includes("download.create")) {
    return error_response("target client does not provide download.create", 409);
  }

  const source_response = await object.fetch("https://internal/tasks/" + encodeURIComponent(source_task_id));
  if (!source_response.ok) {
    return error_response("source fetch task not found", 404);
  }
  /** @type {{ task?: Record<string, unknown> }} */
  const source_value = await source_response.json();
  const source_task = source_value.task;
  if (source_task === undefined || source_task.kind !== "wxchannels.fetch") {
    return error_response("source task is not wxchannels.fetch", 409);
  }
  if (source_task.status !== "completed" || source_task.result === null || source_task.result === undefined) {
    return error_response("source fetch task has not completed", 409);
  }

  return object.fetch("https://internal/tasks", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Hub-Client-ID": "admin-console",
    },
    body: JSON.stringify({
      kind: "download.create",
      target_client_id,
      required_capability: "download.create",
      idempotency_key: "admin-download-" + crypto.randomUUID(),
      payload: {
        request: {
          platform: "wxchannels",
          content: source_task.result,
          build_from_fetch: true,
          download_dir,
          filename,
          config: {},
          auto_start: true,
        },
      },
    }),
  });
}

/**
 * @param {Env} env
 * @param {string} hub_id
 * @param {string} task_id
 * @returns {Promise<Response>}
 */
async function admin_task_status(env, hub_id, task_id) {
  if (!valid_identifier(hub_id) || hub_id === HUB_DIRECTORY_NAME) {
    return error_response("invalid hub_id", 400);
  }
  if (!/^[A-Za-z0-9-]{1,128}$/.test(task_id)) {
    return error_response("invalid task id", 400);
  }
  const object = env.HUBS.getByName(hub_id);
  return object.fetch("https://internal/tasks/" + encodeURIComponent(task_id));
}

/**
 * @param {TaskRow} row
 * @returns {Record<string, unknown>}
 */
function task_value(row) {
  return {
    id: row.id,
    kind: row.kind,
    publisher_id: row.publisher_id,
    target_client_id: row.target_client_id,
    required_capability: row.required_capability,
    idempotency_key: row.idempotency_key,
    payload: JSON.parse(row.payload_json),
    status: row.status,
    assigned_client_id: row.assigned_client_id,
    lease_expires_at: row.lease_expires_at,
    attempt_count: row.attempt_count,
    result: row.result_json === null ? null : JSON.parse(row.result_json),
    error: row.error_message,
    created_at: row.created_at,
    updated_at: row.updated_at,
    completed_at: row.completed_at,
  };
}

/**
 * @param {WebSocket} socket
 * @returns {SocketAttachment | null}
 */
function socket_attachment(socket) {
  try {
    /** @type {SocketAttachment | null} */
    const value = socket.deserializeAttachment();
    return value?.client_id ? value : null;
  } catch {
    return null;
  }
}

/**
 * @param {string} value
 * @returns {string[]}
 */
function parse_capabilities(value) {
  try {
    const capabilities = JSON.parse(value);
    return Array.isArray(capabilities)
      ? capabilities.filter((capability) => typeof capability === "string")
      : [];
  } catch {
    return [];
  }
}

export class HubDurableObject extends DurableObject {
  /**
   * @param {DurableObjectState} ctx
   * @param {Env} env
   */
  constructor(ctx, env) {
    super(ctx, env);
    this.ctx.storage.sql.exec(`
      CREATE TABLE IF NOT EXISTS tasks (
        id TEXT PRIMARY KEY,
        kind TEXT NOT NULL,
        publisher_id TEXT NOT NULL,
        target_client_id TEXT,
        required_capability TEXT NOT NULL,
        idempotency_key TEXT,
        payload_json TEXT NOT NULL,
        status TEXT NOT NULL,
        assigned_client_id TEXT,
        lease_token TEXT,
        lease_expires_at INTEGER,
        attempt_count INTEGER NOT NULL DEFAULT 0,
        result_json TEXT,
        error_message TEXT,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL,
        completed_at INTEGER
      );
      CREATE UNIQUE INDEX IF NOT EXISTS tasks_idempotency
        ON tasks(publisher_id, idempotency_key)
        WHERE idempotency_key IS NOT NULL;
      CREATE INDEX IF NOT EXISTS tasks_dispatch
        ON tasks(status, created_at);
      CREATE INDEX IF NOT EXISTS tasks_lease
        ON tasks(status, lease_expires_at);
      CREATE TABLE IF NOT EXISTS hub_registry (
        id TEXT PRIMARY KEY,
        last_seen_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS client_registry (
        client_id TEXT PRIMARY KEY,
        connection_id TEXT NOT NULL,
        capabilities_json TEXT NOT NULL,
        connected_at INTEGER NOT NULL,
        last_seen_at INTEGER NOT NULL,
        disconnected_at INTEGER,
        status TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS client_registry_status
        ON client_registry(status, last_seen_at);
    `);
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname === "/directory/register" && request.method === "POST") {
      return this.register_hub(request);
    }
    if (url.pathname === "/directory/hubs" && request.method === "GET") {
      return this.list_registered_hubs();
    }
    if (url.pathname === "/connect") {
      return this.accept_connection(request);
    }
    if (url.pathname === "/" && request.method === "GET") {
      return this.summary();
    }
    if (url.pathname === "/tasks" && request.method === "POST") {
      return this.create_task(request);
    }
    if (url.pathname === "/tasks" && request.method === "GET") {
      return this.list_tasks(url);
    }
    const task_match = url.pathname.match(/^\/tasks\/([A-Za-z0-9-]+)$/);
    if (task_match && request.method === "GET") {
      return this.get_task(task_match[1]);
    }
    return error_response("not found", 404);
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async register_hub(request) {
    const hub_id = (await request.text()).trim();
    if (!valid_identifier(hub_id)) {
      return error_response("invalid hub id", 400);
    }
    this.ctx.storage.sql.exec(
      `INSERT INTO hub_registry (id, last_seen_at) VALUES (?, ?)
       ON CONFLICT(id) DO UPDATE SET last_seen_at = excluded.last_seen_at`,
      hub_id,
      Date.now(),
    );
    return json_response({ ok: true });
  }

  /** @returns {Response} */
  list_registered_hubs() {
    /** @type {HubRegistryRow[]} */
    const hubs = this.ctx.storage.sql
      .exec("SELECT id, last_seen_at FROM hub_registry ORDER BY id")
      .toArray();
    return json_response({ hubs });
  }

  /**
   * @param {WebSocket} socket
   * @param {ArrayBuffer | string} message
   * @returns {Promise<void>}
   */
  async webSocketMessage(socket, message) {
    const attachment = socket_attachment(socket);
    if (attachment === null) {
      socket.close(1008, "missing client attachment");
      return;
    }
    this.record_client_activity(attachment);

    const text = typeof message === "string" ? message : new TextDecoder().decode(message);
    if (text.length > MAX_BODY_BYTES) {
      socket.send(JSON.stringify({ type: "error", error: "message is too large" }));
      return;
    }

    /** @type {ClientMessage} */
    let value;
    try {
      value = JSON.parse(text);
    } catch {
      socket.send(JSON.stringify({ type: "error", error: "invalid JSON" }));
      return;
    }

    switch (value.type) {
      case "client.heartbeat":
        socket.send(JSON.stringify({ type: "client.heartbeat.ack", at: Date.now() }));
        return;
      case "task.accept":
        await this.accept_task(socket, attachment, value);
        return;
      case "task.heartbeat":
        await this.heartbeat_task(socket, attachment, value);
        return;
      case "task.complete":
        await this.complete_task(socket, attachment, value);
        return;
      case "task.fail":
        await this.fail_task(socket, attachment, value);
        return;
      default:
        socket.send(JSON.stringify({ type: "error", error: "unknown message type" }));
    }
  }

  /**
   * @param {WebSocket} socket
   * @param {number} code
   * @param {string} reason
   * @param {boolean} was_clean
   * @returns {Promise<void>}
   */
  async webSocketClose(socket, code, reason, was_clean) {
    this.mark_client_offline(socket);
    socket.close(code, reason || (was_clean ? "closed" : "connection lost"));
  }

  /**
   * @param {WebSocket} socket
   * @returns {Promise<void>}
   */
  async webSocketError(socket) {
    this.mark_client_offline(socket);
    socket.close(1011, "websocket error");
  }

  /** @returns {Promise<void>} */
  async alarm() {
    const now = Date.now();
    /** @type {TaskRow[]} */
    const exhausted = this.ctx.storage.sql
      .exec(
        `SELECT * FROM tasks
         WHERE status IN ('assigned', 'running')
           AND lease_expires_at <= ? AND attempt_count >= ?`,
        now,
        MAX_ATTEMPTS,
      )
      .toArray();

    this.ctx.storage.sql.exec(
      `UPDATE tasks
       SET status = 'failed', error_message = 'task lease expired too many times',
           updated_at = ?, completed_at = ?
       WHERE status IN ('assigned', 'running')
         AND lease_expires_at <= ? AND attempt_count >= ?`,
      now,
      now,
      now,
      MAX_ATTEMPTS,
    );
    this.ctx.storage.sql.exec(
      `UPDATE tasks
       SET status = 'queued', assigned_client_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, updated_at = ?
       WHERE status IN ('assigned', 'running')
         AND lease_expires_at <= ? AND attempt_count < ?`,
      now,
      now,
      MAX_ATTEMPTS,
    );
    this.ctx.storage.sql.exec(
      `DELETE FROM tasks
       WHERE status IN ('completed', 'failed') AND completed_at < ?`,
      now - RETENTION_MILLISECONDS,
    );

    for (const row of exhausted) {
      const failed_row = this.find_task(row.id);
      if (failed_row !== null) {
        this.send_to_client(failed_row.publisher_id, {
          type: "task.failed",
          task: task_value(failed_row),
        });
      }
    }
    await this.dispatch_pending();
    await this.schedule_alarm();
  }

  /**
   * @param {SocketAttachment} attachment
   * @returns {void}
   */
  record_client_activity(attachment) {
    this.ctx.storage.sql.exec(
      `UPDATE client_registry
       SET last_seen_at = ?, status = 'online', disconnected_at = NULL
       WHERE client_id = ? AND connection_id = ?`,
      Date.now(),
      attachment.client_id,
      attachment.connection_id,
    );
  }

  /**
   * @param {WebSocket} socket
   * @returns {void}
   */
  mark_client_offline(socket) {
    const attachment = socket_attachment(socket);
    if (attachment === null) {
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      `UPDATE client_registry
       SET status = 'offline', last_seen_at = ?, disconnected_at = ?
       WHERE client_id = ? AND connection_id = ?`,
      now,
      now,
      attachment.client_id,
      attachment.connection_id,
    );
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async accept_connection(request) {
    if (request.headers.get("Upgrade")?.toLowerCase() !== "websocket") {
      return error_response("websocket upgrade required", 426);
    }
    const client_id = (request.headers.get("X-Hub-Client-ID") ?? "").trim();
    if (!valid_identifier(client_id)) {
      return error_response("invalid X-Hub-Client-ID", 400);
    }
    const capabilities = (request.headers.get("X-Hub-Capabilities") ?? "")
      .split(",")
      .map((item) => item.trim())
      .filter((item, index, values) => valid_identifier(item) && values.indexOf(item) === index)
      .slice(0, 8);

    for (const old_socket of this.ctx.getWebSockets(`client:${client_id}`)) {
      old_socket.close(1000, "replaced by a newer connection");
    }

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];
    const connection_id = crypto.randomUUID();
    const connected_at = Date.now();
    const tags = [`client:${client_id}`, ...capabilities.map((value) => `cap:${value}`)];
    server.serializeAttachment({
      client_id,
      connection_id,
      capabilities,
      connected_at,
    });
    this.ctx.storage.sql.exec(
      `INSERT INTO client_registry (
         client_id, connection_id, capabilities_json, connected_at,
         last_seen_at, disconnected_at, status
       ) VALUES (?, ?, ?, ?, ?, NULL, 'online')
       ON CONFLICT(client_id) DO UPDATE SET
         connection_id = excluded.connection_id,
         capabilities_json = excluded.capabilities_json,
         connected_at = excluded.connected_at,
         last_seen_at = excluded.last_seen_at,
         disconnected_at = NULL,
         status = 'online'`,
      client_id,
      connection_id,
      JSON.stringify(capabilities),
      connected_at,
      connected_at,
    );
    this.ctx.acceptWebSocket(server, tags);
    server.send(
      JSON.stringify({
        type: "client.connected",
        client_id,
        capabilities,
        at: Date.now(),
      }),
    );
    await this.dispatch_pending();
    return new Response(null, { status: 101, webSocket: client });
  }

  /** @returns {Response} */
  summary() {
    /** @type {TaskCountRow[]} */
    const task_counts = this.ctx.storage.sql
      .exec(
        "SELECT status, COUNT(*) AS count FROM tasks GROUP BY status ORDER BY status",
      )
      .toArray();
    /** @type {SocketAttachment[]} */
    const active_clients = this.ctx
      .getWebSockets()
      .map(socket_attachment)
      .filter((item) => item !== null);
    const active_connection_ids = new Set(active_clients.map((client) => client.connection_id));
    /** @type {BusyClientRow[]} */
    const busy_rows = this.ctx.storage.sql
      .exec(
        `SELECT DISTINCT assigned_client_id FROM tasks
         WHERE status IN ('assigned', 'running') AND assigned_client_id IS NOT NULL`,
      )
      .toArray();
    const busy_clients = new Set(busy_rows.map((row) => row.assigned_client_id));
    /** @type {ClientRegistryRow[]} */
    const registry_rows = this.ctx.storage.sql
      .exec(
        `SELECT client_id, connection_id, capabilities_json, connected_at,
                last_seen_at, disconnected_at, status
         FROM client_registry ORDER BY client_id`,
      )
      .toArray();
    const now = Date.now();
    const clients = registry_rows.map((row) => {
      const online = active_connection_ids.has(row.connection_id);
      if (!online && row.status === "online") {
        this.ctx.storage.sql.exec(
          `UPDATE client_registry
           SET status = 'offline', disconnected_at = ?
           WHERE client_id = ? AND connection_id = ?`,
          now,
          row.client_id,
          row.connection_id,
        );
      }
      return {
        client_id: row.client_id,
        capabilities: parse_capabilities(row.capabilities_json),
        connected_at: row.connected_at,
        last_seen_at: row.last_seen_at,
        disconnected_at: online ? null : row.disconnected_at ?? now,
        status: online ? (busy_clients.has(row.client_id) ? "busy" : "online") : "offline",
      };
    });
    return json_response({ clients, task_counts });
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async create_task(request) {
    const publisher_id = (request.headers.get("X-Hub-Client-ID") ?? "").trim();
    if (!valid_identifier(publisher_id)) {
      return error_response("invalid X-Hub-Client-ID", 400);
    }
    const content_length = Number(request.headers.get("Content-Length") ?? "0");
    if (content_length > MAX_BODY_BYTES) {
      return error_response("request body is too large", 413);
    }

    /** @type {CreateTaskBody} */
    let body;
    try {
      body = await request.json();
    } catch {
      return error_response("invalid JSON", 400);
    }
    const kind = (body.kind ?? "").trim();
    if (!VALID_TASK_KINDS.has(kind)) {
      return error_response("unsupported task kind", 400);
    }
    const target_client_id = (body.target_client_id ?? "").trim();
    if (target_client_id !== "" && !valid_identifier(target_client_id)) {
      return error_response("invalid target_client_id", 400);
    }
    const required_capability = (body.required_capability ?? kind).trim();
    if (!valid_identifier(required_capability)) {
      return error_response("invalid required_capability", 400);
    }
    const idempotency_key = (body.idempotency_key ?? "").trim();
    if (idempotency_key.length > 128) {
      return error_response("idempotency_key is too long", 400);
    }
    const payload_json = JSON.stringify(body.payload ?? {});
    if (payload_json.length > MAX_BODY_BYTES) {
      return error_response("payload is too large", 413);
    }

    if (idempotency_key !== "") {
      /** @type {TaskRow | undefined} */
      const existing = this.ctx.storage.sql
        .exec(
          "SELECT * FROM tasks WHERE publisher_id = ? AND idempotency_key = ? LIMIT 1",
          publisher_id,
          idempotency_key,
        )
        .toArray()[0];
      if (existing !== undefined) {
        return json_response({ task: task_value(existing), idempotent_replay: true });
      }
    }

    const now = Date.now();
    const task_id = crypto.randomUUID();
    this.ctx.storage.sql.exec(
      `INSERT INTO tasks (
        id, kind, publisher_id, target_client_id, required_capability,
        idempotency_key, payload_json, status, attempt_count, created_at, updated_at
      ) VALUES (?, ?, ?, ?, ?, ?, ?, 'queued', 0, ?, ?)`,
      task_id,
      kind,
      publisher_id,
      target_client_id || null,
      required_capability,
      idempotency_key || null,
      payload_json,
      now,
      now,
    );
    await this.dispatch_pending();
    await this.schedule_alarm();
    const row = this.find_task(task_id);
    return json_response({ task: task_value(/** @type {TaskRow} */ (row)) }, 201);
  }

  /**
   * @param {URL} url
   * @returns {Response}
   */
  list_tasks(url) {
    const publisher_id = (url.searchParams.get("publisher_id") ?? "").trim();
    const status = (url.searchParams.get("status") ?? "").trim();
    const limit_value = Number(url.searchParams.get("limit") ?? "50");
    const limit = Number.isFinite(limit_value) ? Math.max(1, Math.min(200, limit_value)) : 50;

    /** @type {TaskRow[]} */
    let rows;
    if (publisher_id !== "" && status !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM tasks WHERE publisher_id = ? AND status = ? ORDER BY created_at DESC LIMIT ?",
          publisher_id,
          status,
          limit,
        )
        .toArray();
    } else if (publisher_id !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM tasks WHERE publisher_id = ? ORDER BY created_at DESC LIMIT ?",
          publisher_id,
          limit,
        )
        .toArray();
    } else if (status !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM tasks WHERE status = ? ORDER BY created_at DESC LIMIT ?",
          status,
          limit,
        )
        .toArray();
    } else {
      rows = this.ctx.storage.sql
        .exec("SELECT * FROM tasks ORDER BY created_at DESC LIMIT ?", limit)
        .toArray();
    }
    return json_response({ tasks: rows.map(task_value) });
  }

  /**
   * @param {string} task_id
   * @returns {Response}
   */
  get_task(task_id) {
    const row = this.find_task(task_id);
    return row === null
      ? error_response("task not found", 404)
      : json_response({ task: task_value(row) });
  }

  /**
   * @param {string} task_id
   * @returns {TaskRow | null}
   */
  find_task(task_id) {
    return (
      this.ctx.storage.sql
        .exec("SELECT * FROM tasks WHERE id = ? LIMIT 1", task_id)
        .toArray()[0] ?? null
    );
  }

  /**
   * @param {WebSocket} socket
   * @param {SocketAttachment} attachment
   * @param {ClientMessage} message
   * @returns {Promise<void>}
   */
  async accept_task(socket, attachment, message) {
    const row = this.owned_task(attachment.client_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      "UPDATE tasks SET status = 'running', lease_expires_at = ?, updated_at = ? WHERE id = ?",
      now + LEASE_MILLISECONDS,
      now,
      row.id,
    );
    socket.send(JSON.stringify({ type: "task.accepted", task_id: row.id }));
    await this.schedule_alarm();
  }

  /**
   * @param {WebSocket} socket
   * @param {SocketAttachment} attachment
   * @param {ClientMessage} message
   * @returns {Promise<void>}
   */
  async heartbeat_task(socket, attachment, message) {
    const row = this.owned_task(attachment.client_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      "UPDATE tasks SET lease_expires_at = ?, updated_at = ? WHERE id = ?",
      now + LEASE_MILLISECONDS,
      now,
      row.id,
    );
    socket.send(JSON.stringify({ type: "task.heartbeat.ack", task_id: row.id }));
    await this.schedule_alarm();
  }

  /**
   * @param {WebSocket} socket
   * @param {SocketAttachment} attachment
   * @param {ClientMessage} message
   * @returns {Promise<void>}
   */
  async complete_task(socket, attachment, message) {
    const existing = message.task_id ? this.find_task(message.task_id) : null;
    if (
      existing !== null &&
      existing.status === "completed" &&
      existing.assigned_client_id === attachment.client_id &&
      existing.lease_token === message.lease_token
    ) {
      socket.send(JSON.stringify({ type: "task.ack", task_id: existing.id }));
      return;
    }
    const row = this.owned_task(attachment.client_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const result_json = JSON.stringify(message.result ?? null);
    if (result_json.length > MAX_BODY_BYTES) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: row.id, error: "result is too large" }));
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      `UPDATE tasks SET status = 'completed', result_json = ?, error_message = NULL,
       lease_expires_at = NULL, updated_at = ?, completed_at = ? WHERE id = ?`,
      result_json,
      now,
      now,
      row.id,
    );
    const completed = /** @type {TaskRow} */ (this.find_task(row.id));
    socket.send(JSON.stringify({ type: "task.ack", task_id: row.id }));
    this.send_to_client(completed.publisher_id, {
      type: "task.completed",
      task: task_value(completed),
    });
    await this.dispatch_pending();
    await this.schedule_alarm();
  }

  /**
   * @param {WebSocket} socket
   * @param {SocketAttachment} attachment
   * @param {ClientMessage} message
   * @returns {Promise<void>}
   */
  async fail_task(socket, attachment, message) {
    const row = this.owned_task(attachment.client_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    const retryable = message.retryable === true && row.attempt_count < MAX_ATTEMPTS;
    if (retryable) {
      this.ctx.storage.sql.exec(
        `UPDATE tasks SET status = 'queued', assigned_client_id = NULL, lease_token = NULL,
         lease_expires_at = NULL, error_message = ?, updated_at = ? WHERE id = ?`,
        (message.error ?? "task failed").slice(0, 4000),
        now,
        row.id,
      );
      socket.send(JSON.stringify({ type: "task.ack", task_id: row.id, requeued: true }));
    } else {
      this.ctx.storage.sql.exec(
        `UPDATE tasks SET status = 'failed', error_message = ?, lease_expires_at = NULL,
         updated_at = ?, completed_at = ? WHERE id = ?`,
        (message.error ?? "task failed").slice(0, 4000),
        now,
        now,
        row.id,
      );
      const failed = /** @type {TaskRow} */ (this.find_task(row.id));
      socket.send(JSON.stringify({ type: "task.ack", task_id: row.id }));
      this.send_to_client(failed.publisher_id, {
        type: "task.failed",
        task: task_value(failed),
      });
    }
    await this.dispatch_pending();
    await this.schedule_alarm();
  }

  /**
   * @param {string} client_id
   * @param {ClientMessage} message
   * @returns {TaskRow | null}
   */
  owned_task(client_id, message) {
    if (!message.task_id || !message.lease_token) {
      return null;
    }
    const row = this.find_task(message.task_id);
    if (
      row === null ||
      (row.status !== "assigned" && row.status !== "running") ||
      row.assigned_client_id !== client_id ||
      row.lease_token !== message.lease_token
    ) {
      return null;
    }
    return row;
  }

  /** @returns {Promise<void>} */
  async dispatch_pending() {
    /** @type {TaskRow[]} */
    const queued = this.ctx.storage.sql
      .exec("SELECT * FROM tasks WHERE status = 'queued' ORDER BY created_at LIMIT 100")
      .toArray();
    if (queued.length === 0) {
      return;
    }
    /** @type {BusyClientRow[]} */
    const busy_rows = this.ctx.storage.sql
      .exec(
        `SELECT DISTINCT assigned_client_id FROM tasks
         WHERE status IN ('assigned', 'running') AND assigned_client_id IS NOT NULL`,
      )
      .toArray();
    const busy_clients = new Set(busy_rows.map((row) => row.assigned_client_id));

    for (const row of queued) {
      const candidates = row.target_client_id
        ? this.ctx.getWebSockets(`client:${row.target_client_id}`)
        : this.ctx.getWebSockets(`cap:${row.required_capability}`);
      const candidate = candidates.find((socket) => {
        const attachment = socket_attachment(socket);
        return attachment !== null && !busy_clients.has(attachment.client_id);
      });
      if (candidate === undefined) {
        continue;
      }
      const attachment = /** @type {SocketAttachment} */ (socket_attachment(candidate));
      if (!attachment.capabilities.includes(row.required_capability)) {
        continue;
      }

      const now = Date.now();
      const lease_token = crypto.randomUUID();
      this.ctx.storage.sql.exec(
        `UPDATE tasks SET status = 'assigned', assigned_client_id = ?, lease_token = ?,
         lease_expires_at = ?, attempt_count = attempt_count + 1, updated_at = ?
         WHERE id = ? AND status = 'queued'`,
        attachment.client_id,
        lease_token,
        now + LEASE_MILLISECONDS,
        now,
        row.id,
      );
      const assigned = /** @type {TaskRow} */ (this.find_task(row.id));
      try {
        candidate.send(
          JSON.stringify({
            type: "task.assigned",
            task: task_value(assigned),
            lease_token,
            lease_milliseconds: LEASE_MILLISECONDS,
          }),
        );
        busy_clients.add(attachment.client_id);
      } catch {
        this.ctx.storage.sql.exec(
          `UPDATE tasks SET status = 'queued', assigned_client_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, updated_at = ? WHERE id = ?`,
          Date.now(),
          row.id,
        );
      }
    }
  }

  /**
   * @param {string} client_id
   * @param {unknown} value
   * @returns {void}
   */
  send_to_client(client_id, value) {
    const message = JSON.stringify(value);
    for (const socket of this.ctx.getWebSockets(`client:${client_id}`)) {
      try {
        socket.send(message);
      } catch {
        // Persisted task state remains available for polling after reconnect.
      }
    }
  }

  /** @returns {Promise<void>} */
  async schedule_alarm() {
    /** @type {number | null | undefined} */
    const next_lease = this.ctx.storage.sql
      .exec(
        `SELECT MIN(lease_expires_at) AS lease_expires_at FROM tasks
         WHERE status IN ('assigned', 'running') AND lease_expires_at IS NOT NULL`,
      )
      .toArray()[0]?.lease_expires_at;
    const desired = next_lease ?? Date.now() + MAINTENANCE_MILLISECONDS;
    const current = await this.ctx.storage.getAlarm();
    if (current === null || Math.abs(current - desired) > 1000) {
      await this.ctx.storage.setAlarm(desired);
    }
  }
}

/** @type {ExportedHandler<Env>} */
export default {
  async fetch(request, env, execution_context) {
    const url = new URL(request.url);
    if (url.pathname === "/health" && request.method === "GET") {
      return json_response({ ok: true });
    }
    const is_admin_route = url.pathname === "/admin" || url.pathname.startsWith("/admin/");
    if (is_admin_route) {
      if (!admin_authorized(request, env)) {
        return admin_auth_required();
      }
      if (url.pathname === "/admin/api/overview" && request.method === "GET") {
        return admin_overview(env);
      }
      if (url.pathname === "/admin/api/tests" && request.method === "POST") {
        return admin_create_test(env, request);
      }
      if (url.pathname === "/admin/api/downloads" && request.method === "POST") {
        return admin_create_download(env, request);
      }
      const admin_task_match = url.pathname.match(/^\/admin\/api\/tasks\/([^/]+)$/);
      if (admin_task_match !== null && request.method === "GET") {
        return admin_task_status(
          env,
          (url.searchParams.get("hub_id") ?? "").trim(),
          admin_task_match[1],
        );
      }
      return error_response("not found", 404);
    }
    if (!env.HUB_TOKEN || bearer_token(request) !== env.HUB_TOKEN) {
      return error_response("unauthorized", 401);
    }

    const match = url.pathname.match(/^\/v1\/hubs\/([^/]+)(\/.*)?$/);
    if (match === null) {
      return error_response("not found", 404);
    }
    const hub_id = decodeURIComponent(match[1]);
    if (!valid_identifier(hub_id)) {
      return error_response("invalid hub id", 400);
    }
    execution_context.waitUntil(register_hub(env, hub_id).catch(() => undefined));
    const forwarded_url = new URL(request.url);
    forwarded_url.pathname = match[2] || "/";
    const object = env.HUBS.getByName(hub_id);
    return object.fetch(new Request(forwarded_url, request));
  },
};

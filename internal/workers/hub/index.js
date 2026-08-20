import { DurableObject } from "cloudflare:workers";

/**
 * @typedef {Object} Env
 * @property {DurableObjectNamespace<HubDurableObject>} HUBS
 * @property {string} HUB_TOKEN
 * @property {string} HUB_ADMIN_TOKEN
 */

/** @typedef {"queued" | "assigned" | "running" | "completed" | "failed"} TaskStatus */

/**
 * @typedef {Object} CallRow
 * @property {string} id
 * @property {string} method
 * @property {string} publisher_device_id
 * @property {string | null} target_device_id
 * @property {string | null} idempotency_key
 * @property {string} args_json
 * @property {TaskStatus} status
 * @property {string | null} assigned_device_id
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
 * @property {string} device_id
 * @property {string} [client_id]
 * @property {string} connection_id
 * @property {string[]} methods
 * @property {string[]} [capabilities]
 * @property {string} device_name
 * @property {string} device_os
 * @property {number} [connected_at]
 */

/**
 * @typedef {Object} DeviceRegistryRow
 * @property {string} device_id
 * @property {string} connection_id
 * @property {string} methods_json
 * @property {string} device_name
 * @property {string} device_os
 * @property {number} connected_at
 * @property {number} last_seen_at
 * @property {number | null} disconnected_at
 * @property {"online" | "offline"} status
 */

/**
 * @typedef {Object} AccessTokenRow
 * @property {string} id
 * @property {string} name
 * @property {string} token_hash
 * @property {string} token_hint
 * @property {number | null} expires_at
 * @property {number} created_at
 * @property {number | null} last_used_at
 */

/**
 * @typedef {Object} InvokeWaiter
 * @property {(row: CallRow | null) => void} resolve
 * @property {number} timeout_id
 */

/**
 * @typedef {Object} CreateAccessTokenBody
 * @property {string} [name]
 * @property {string | null} [token]
 * @property {number | null} [expires_in_seconds]
 */

/**
 * @typedef {Object} CreateTaskBody
 * @property {string} [method]
 * @property {string} [kind]
 * @property {string} [target_device_id]
 * @property {string} [target_client_id]
 * @property {string} [idempotency_key]
 * @property {unknown} [args]
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
 * @typedef {Object} BusyDeviceRow
 * @property {string} assigned_device_id
 */

/**
 * @typedef {Object} LeaseRow
 * @property {number | null} lease_expires_at
 */

/**
 * @typedef {Object} AdminCallBody
 * @property {string} target_device_id
 * @property {string} method
 * @property {Record<string, unknown>} args
 * @property {string} [idempotency_key]
 */

/**
 * @typedef {Object} DeviceSummary
 * @property {string} device_id
 * @property {string} device_name
 * @property {string} device_os
 * @property {string[]} methods
 * @property {"online" | "busy" | "offline"} status
 */

const MAX_BODY_BYTES = 1024 * 1024;
const MAX_ATTEMPTS = 10;
const LEASE_MILLISECONDS = 120_000;
const RETENTION_MILLISECONDS = 7 * 24 * 60 * 60 * 1000;
const MAINTENANCE_MILLISECONDS = 24 * 60 * 60 * 1000;
const HUB_OBJECT_NAME = "hub";
const MAX_ACCESS_TOKENS = 500;
const MIN_ACCESS_TOKEN_LENGTH = 16;
const MAX_ACCESS_TOKEN_LENGTH = 256;
const MAX_ACCESS_TOKEN_LIFETIME_SECONDS = 366 * 24 * 60 * 60;
const INVOKE_TIMEOUT_MILLISECONDS = 10_000;

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
 * @param {string} value
 * @returns {Promise<string>}
 */
async function sha256_hex(value) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return [...new Uint8Array(digest)]
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

/** @returns {string} */
function generate_access_token() {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return "hub_call_" + btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

/**
 * @param {string} value
 * @returns {boolean}
 */
function valid_access_token(value) {
  return (
    value.length >= MIN_ACCESS_TOKEN_LENGTH &&
    value.length <= MAX_ACCESS_TOKEN_LENGTH &&
    /^[A-Za-z0-9._~+/=-]+$/.test(value)
  );
}

/**
 * @param {string} token
 * @returns {string}
 */
function access_token_hint(token) {
  const visible_prefix_length = token.startsWith("hub_call_") ? 17 : 4;
  return token.slice(0, visible_prefix_length) + "…" + token.slice(-4);
}

/**
 * @param {AccessTokenRow} row
 * @param {number} [now]
 * @returns {Record<string, unknown>}
 */
function access_token_value(row, now = Date.now()) {
  const expired = row.expires_at !== null && row.expires_at <= now;
  return {
    id: row.id,
    name: row.name,
    token_hint: row.token_hint,
    status: expired ? "expired" : "active",
    expires_at: row.expires_at,
    created_at: row.created_at,
    last_used_at: row.last_used_at,
  };
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
 * @returns {Promise<Response>}
 */
async function admin_overview(env) {
  const object = env.HUBS.getByName(HUB_OBJECT_NAME);
  const [response, access_tokens_response] = await Promise.all([
    object.fetch("https://internal/"),
    object.fetch("https://internal/_admin/access-tokens"),
  ]);
  if (!response.ok || !access_tokens_response.ok) {
    return error_response("failed to load Hub overview", 500);
  }
  /** @type {Record<string, unknown>} */
  const summary = await response.json();
  /** @type {{ access_tokens?: Record<string, unknown>[] }} */
  const access_tokens = await access_tokens_response.json();
  return json_response({
    generated_at: Date.now(),
    ...summary,
    access_tokens: Array.isArray(access_tokens.access_tokens)
      ? access_tokens.access_tokens
      : [],
  });
}

/**
 * @param {Env} env
 * @param {Request} request
 * @param {string} path
 * @returns {Promise<Response>}
 */
async function admin_access_token_request(env, request, path) {
  const object = env.HUBS.getByName(HUB_OBJECT_NAME);
  return object.fetch(new Request("https://internal/_admin/access-tokens" + path, request));
}

/**
 * @param {Env} env
 * @param {Request} request
 * @returns {Promise<Response>}
 */
async function admin_create_call(env, request) {
  const content_length = Number(request.headers.get("Content-Length") ?? "0");
  if (content_length > MAX_BODY_BYTES) {
    return error_response("request body is too large", 413);
  }
  /** @type {AdminCallBody} */
  let body;
  try {
    body = await request.json();
  } catch {
    return error_response("invalid JSON", 400);
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)) {
    return error_response("invalid JSON body", 400);
  }
  const target_device_id =
    typeof body.target_device_id === "string" ? body.target_device_id.trim() : "";
  const method = typeof body.method === "string" ? body.method.trim() : "";
  const args = body.args ?? {};
  if (target_device_id !== "" && !valid_identifier(target_device_id)) {
    return error_response("invalid target_device_id", 400);
  }
  if (!valid_identifier(method)) {
    return error_response("invalid method", 400);
  }
  if (args === null || typeof args !== "object" || Array.isArray(args)) {
    return error_response("args must be an object", 400);
  }

  const object = env.HUBS.getByName(HUB_OBJECT_NAME);
  if (target_device_id !== "") {
    const summary_response = await object.fetch("https://internal/");
    if (!summary_response.ok) {
      return error_response("failed to load Hub devices", 502);
    }
    /** @type {{ devices?: DeviceSummary[] }} */
    const summary = await summary_response.json();
    const devices = Array.isArray(summary.devices) ? summary.devices : [];
    const target_device = devices.find((device) => device.device_id === target_device_id);
    if (target_device === undefined) {
      return error_response("target device is not registered", 404);
    }
    if (target_device.status === "offline") {
      return error_response("target device is offline", 409);
    }
    if (!Array.isArray(target_device.methods) || !target_device.methods.includes(method)) {
      return error_response("target device does not provide method " + method, 409);
    }
  }

  return object.fetch("https://internal/call", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Hub-Publisher-ID": "admin-console",
    },
    body: JSON.stringify({
      method,
      target_device_id,
      idempotency_key:
        typeof body.idempotency_key === "string" && body.idempotency_key.trim() !== ""
          ? body.idempotency_key.trim()
          : "admin-call-" + crypto.randomUUID(),
      args,
    }),
  });
}

/**
 * @param {Env} env
 * @param {string} task_id
 * @returns {Promise<Response>}
 */
async function admin_task_status(env, task_id) {
  if (!/^[A-Za-z0-9-]{1,128}$/.test(task_id)) {
    return error_response("invalid task id", 400);
  }
  const object = env.HUBS.getByName(HUB_OBJECT_NAME);
  return object.fetch("https://internal/tasks/" + encodeURIComponent(task_id));
}

/**
 * @param {CallRow} row
 * @returns {Record<string, unknown>}
 */
function task_value(row) {
  return {
    id: row.id,
    method: row.method,
    kind: row.method,
    publisher_id: row.publisher_device_id,
    publisher_device_id: row.publisher_device_id,
    target_client_id: row.target_device_id,
    target_device_id: row.target_device_id,
    idempotency_key: row.idempotency_key,
    args: JSON.parse(row.args_json),
    payload: JSON.parse(row.args_json),
    status: row.status,
    assigned_client_id: row.assigned_device_id,
    assigned_device_id: row.assigned_device_id,
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
    if (!value?.device_id && value?.client_id) {
      value.device_id = value.client_id;
    }
    if (!value?.device_id) return null;
    value.methods = Array.isArray(value.methods)
      ? value.methods
      : Array.isArray(value.capabilities)
        ? value.capabilities
        : [];
    return value;
  } catch {
    return null;
  }
}

/**
 * @param {string} value
 * @returns {string[]}
 */
function parse_methods(value) {
  try {
    const methods = JSON.parse(value);
    return Array.isArray(methods)
      ? methods.filter((method) => typeof method === "string")
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
    /** @type {Map<string, InvokeWaiter>} */
    this.invoke_waiters = new Map();
    this.ctx.storage.sql.exec(`
      CREATE TABLE IF NOT EXISTS calls (
        id TEXT PRIMARY KEY,
        method TEXT NOT NULL,
        publisher_device_id TEXT NOT NULL,
        target_device_id TEXT,
        idempotency_key TEXT,
        args_json TEXT NOT NULL,
        status TEXT NOT NULL,
        assigned_device_id TEXT,
        lease_token TEXT,
        lease_expires_at INTEGER,
        attempt_count INTEGER NOT NULL DEFAULT 0,
        result_json TEXT,
        error_message TEXT,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL,
        completed_at INTEGER
      );
      CREATE UNIQUE INDEX IF NOT EXISTS calls_idempotency
        ON calls(publisher_device_id, idempotency_key)
        WHERE idempotency_key IS NOT NULL;
      CREATE INDEX IF NOT EXISTS calls_dispatch
        ON calls(status, created_at);
      CREATE INDEX IF NOT EXISTS calls_lease
        ON calls(status, lease_expires_at);
      CREATE TABLE IF NOT EXISTS devices (
        device_id TEXT PRIMARY KEY,
        connection_id TEXT NOT NULL,
        methods_json TEXT NOT NULL,
        device_name TEXT NOT NULL DEFAULT '',
        device_os TEXT NOT NULL DEFAULT '',
        connected_at INTEGER NOT NULL,
        last_seen_at INTEGER NOT NULL,
        disconnected_at INTEGER,
        status TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS devices_status
        ON devices(status, last_seen_at);
      CREATE TABLE IF NOT EXISTS access_tokens (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        token_hash TEXT NOT NULL UNIQUE,
        token_hint TEXT NOT NULL,
        expires_at INTEGER,
        created_at INTEGER NOT NULL,
        last_used_at INTEGER
      );
      CREATE INDEX IF NOT EXISTS access_tokens_expiration
        ON access_tokens(expires_at);
    `);
    const legacy_tables = new Set(
      this.ctx.storage.sql
        .exec("SELECT name FROM sqlite_master WHERE type = 'table'")
        .toArray()
        .map((row) => row.name),
    );
    if (legacy_tables.has("tasks")) {
      this.ctx.storage.sql.exec(`
        INSERT OR IGNORE INTO calls (
          id, method, publisher_device_id, target_device_id, idempotency_key,
          args_json, status, assigned_device_id, lease_token, lease_expires_at,
          attempt_count, result_json, error_message, created_at, updated_at, completed_at
        )
        SELECT id, kind, publisher_id, target_client_id, idempotency_key,
          payload_json, status, assigned_client_id, lease_token, lease_expires_at,
          attempt_count, result_json, error_message, created_at, updated_at, completed_at
        FROM tasks
      `);
    }
    if (legacy_tables.has("client_registry")) {
      const registry_columns = this.ctx.storage.sql
        .exec("PRAGMA table_info(client_registry)")
        .toArray();
      if (!registry_columns.some((column) => column.name === "device_name")) {
        this.ctx.storage.sql.exec(
          "ALTER TABLE client_registry ADD COLUMN device_name TEXT NOT NULL DEFAULT ''",
        );
      }
      if (!registry_columns.some((column) => column.name === "device_os")) {
        this.ctx.storage.sql.exec(
          "ALTER TABLE client_registry ADD COLUMN device_os TEXT NOT NULL DEFAULT ''",
        );
      }
      this.ctx.storage.sql.exec(`
        INSERT OR IGNORE INTO devices (
          device_id, connection_id, methods_json, device_name, device_os,
          connected_at, last_seen_at, disconnected_at, status
        )
        SELECT client_id, connection_id, capabilities_json, device_name, device_os,
          connected_at, last_seen_at, disconnected_at, status
        FROM client_registry
      `);
    }
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async fetch(request) {
    const url = new URL(request.url);
    if (url.pathname === "/connect") {
      return this.accept_connection(request);
    }
    if (url.pathname === "/" && request.method === "GET") {
      return this.summary();
    }
    if ((url.pathname === "/call" || url.pathname === "/tasks") && request.method === "POST") {
      return this.create_task(request);
    }
    if (url.pathname === "/invoke" && request.method === "POST") {
      return this.invoke(request);
    }
    if (url.pathname === "/tasks" && request.method === "GET") {
      return this.list_tasks(request, url);
    }
    const task_match = url.pathname.match(/^\/tasks\/([A-Za-z0-9-]+)$/);
    if (task_match && request.method === "GET") {
      return this.get_task(request, task_match[1]);
    }
    if (url.pathname === "/_admin/access-tokens" && request.method === "GET") {
      return this.list_access_tokens();
    }
    if (url.pathname === "/_admin/access-tokens" && request.method === "POST") {
      return this.create_access_token(request);
    }
    const access_token_match = url.pathname.match(
      /^\/_admin\/access-tokens\/([A-Za-z0-9-]+)$/,
    );
    if (access_token_match && request.method === "DELETE") {
      return this.delete_access_token(access_token_match[1]);
    }
    const access_token_expire_match = url.pathname.match(
      /^\/_admin\/access-tokens\/([A-Za-z0-9-]+)\/expire$/,
    );
    if (access_token_expire_match && request.method === "POST") {
      return this.expire_access_token(access_token_expire_match[1]);
    }
    if (url.pathname === "/_internal/access-tokens/verify" && request.method === "POST") {
      return this.verify_access_token(request);
    }
    return error_response("not found", 404);
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async invoke(request) {
    const deadline = Date.now() + INVOKE_TIMEOUT_MILLISECONDS;
    const task_response = await this.create_task(request, false);
    if (!task_response.ok) {
      return task_response;
    }

    /** @type {{ task?: { id?: string } }} */
    let created;
    try {
      created = await task_response.json();
    } catch {
      return error_response("failed to create invoke task", 500);
    }
    const task_id = String(created.task?.id ?? "");
    if (!/^[A-Za-z0-9-]{1,128}$/.test(task_id)) {
      return error_response("failed to create invoke task", 500);
    }

    const remaining_milliseconds = Math.max(0, deadline - Date.now());
    const row = await this.wait_for_invoke(task_id, remaining_milliseconds);
    if (row === null) {
      return error_response("invoke timed out after 10 seconds", 504);
    }
    if (row.status === "failed") {
      return error_response(row.error_message ?? "invoke failed", 502);
    }
    if (row.status !== "completed") {
      return error_response("invoke ended without a result", 500);
    }
    try {
      return json_response(row.result_json === null ? null : JSON.parse(row.result_json));
    } catch {
      return error_response("invoke returned invalid JSON", 502);
    }
  }

  /**
   * @param {string} task_id
   * @param {number} timeout_milliseconds
   * @returns {Promise<CallRow | null>}
   */
  wait_for_invoke(task_id, timeout_milliseconds) {
    const existing = this.find_task(task_id);
    if (existing === null || existing.status === "completed" || existing.status === "failed") {
      return Promise.resolve(existing);
    }
    if (timeout_milliseconds <= 0) {
      return Promise.resolve(null);
    }

    return new Promise((resolve) => {
      const timeout_id = setTimeout(() => {
        this.invoke_waiters.delete(task_id);
        resolve(null);
      }, timeout_milliseconds);
      this.invoke_waiters.set(task_id, { resolve, timeout_id });

      const current = this.find_task(task_id);
      if (current === null || current.status === "completed" || current.status === "failed") {
        this.resolve_invoke_waiter(task_id, current);
      }
    });
  }

  /**
   * @param {string} task_id
   * @param {CallRow | null} row
   * @returns {void}
   */
  resolve_invoke_waiter(task_id, row) {
    const waiter = this.invoke_waiters.get(task_id);
    if (waiter === undefined) {
      return;
    }
    clearTimeout(waiter.timeout_id);
    this.invoke_waiters.delete(task_id);
    waiter.resolve(row);
  }

  /** @returns {Response} */
  list_access_tokens() {
    /** @type {AccessTokenRow[]} */
    const rows = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens ORDER BY created_at DESC")
      .toArray();
    const now = Date.now();
    return json_response({ access_tokens: rows.map((row) => access_token_value(row, now)) });
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async create_access_token(request) {
    const content_length = Number(request.headers.get("Content-Length") ?? "0");
    if (content_length > 16 * 1024) {
      return error_response("request body is too large", 413);
    }
    /** @type {CreateAccessTokenBody} */
    let body;
    try {
      body = await request.json();
    } catch {
      return error_response("invalid JSON", 400);
    }
    if (body === null || typeof body !== "object" || Array.isArray(body)) {
      return error_response("invalid JSON body", 400);
    }
    if (body.name !== undefined && typeof body.name !== "string") {
      return error_response("name must be a string", 400);
    }
    if (
      body.token !== undefined &&
      body.token !== null &&
      typeof body.token !== "string"
    ) {
      return error_response("token must be a string", 400);
    }
    const name = typeof body.name === "string" ? body.name.trim() : "";
    if (name.length > 128 || /[\r\n]/.test(name)) {
      return error_response(
        "name must not exceed 128 characters or contain newlines",
        400,
      );
    }
    const requested_token =
      typeof body.token === "string" ? body.token.trim() : "";
    if (requested_token !== "" && !valid_access_token(requested_token)) {
      return error_response(
        "token must be 16 to 256 characters using letters, numbers, or . _ ~ + / = -",
        400,
      );
    }
    let expires_in_seconds = null;
    if (
      body.expires_in_seconds !== undefined &&
      body.expires_in_seconds !== null
    ) {
      expires_in_seconds = Number(body.expires_in_seconds);
      if (
        !Number.isInteger(expires_in_seconds) ||
        expires_in_seconds < 60 ||
        expires_in_seconds > MAX_ACCESS_TOKEN_LIFETIME_SECONDS
      ) {
        return error_response("invalid expires_in_seconds", 400);
      }
    }
    const token_count = Number(
      this.ctx.storage.sql
        .exec("SELECT COUNT(*) AS count FROM access_tokens")
        .toArray()[0]?.count ?? 0,
    );
    if (token_count >= MAX_ACCESS_TOKENS) {
      return error_response("access token limit reached", 409);
    }

    const token = requested_token || generate_access_token();
    const token_hash = await sha256_hex(token);
    const existing_token = this.ctx.storage.sql
      .exec("SELECT id FROM access_tokens WHERE token_hash = ? LIMIT 1", token_hash)
      .toArray()[0];
    if (existing_token !== undefined) {
      return error_response("access token already exists", 409);
    }
    const now = Date.now();
    const expires_at =
      expires_in_seconds === null ? null : now + expires_in_seconds * 1000;
    const id = crypto.randomUUID();
    const token_hint = access_token_hint(token);
    this.ctx.storage.sql.exec(
      `INSERT INTO access_tokens (
        id, name, token_hash, token_hint, expires_at, created_at, last_used_at
      ) VALUES (?, ?, ?, ?, ?, ?, NULL)`,
      id,
      name,
      token_hash,
      token_hint,
      expires_at,
      now,
    );
    /** @type {AccessTokenRow} */
    const row = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", id)
      .toArray()[0];
    return json_response({ access_token: access_token_value(row, now), token }, 201);
  }

  /**
   * @param {string} access_token_id
   * @returns {Response}
   */
  expire_access_token(access_token_id) {
    /** @type {AccessTokenRow | undefined} */
    const existing = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
      .toArray()[0];
    if (existing === undefined) {
      return error_response("access token not found", 404);
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      "UPDATE access_tokens SET expires_at = ? WHERE id = ?",
      now,
      access_token_id,
    );
    /** @type {AccessTokenRow} */
    const row = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
      .toArray()[0];
    return json_response({ access_token: access_token_value(row, now) });
  }

  /**
   * @param {string} access_token_id
   * @returns {Response}
   */
  delete_access_token(access_token_id) {
    const existing = this.ctx.storage.sql
      .exec("SELECT id FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
      .toArray()[0];
    if (existing === undefined) {
      return error_response("access token not found", 404);
    }
    this.ctx.storage.sql.exec("DELETE FROM access_tokens WHERE id = ?", access_token_id);
    return json_response({ removed: true, id: access_token_id });
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async verify_access_token(request) {
    /** @type {{ token?: unknown }} */
    let body;
    try {
      body = await request.json();
    } catch {
      return error_response("unauthorized", 401);
    }
    const token = typeof body.token === "string" ? body.token : "";
    if (!valid_access_token(token)) {
      return error_response("unauthorized", 401);
    }
    const token_hash = await sha256_hex(token);
    /** @type {AccessTokenRow | undefined} */
    const row = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE token_hash = ? LIMIT 1", token_hash)
      .toArray()[0];
    const now = Date.now();
    if (row === undefined || (row.expires_at !== null && row.expires_at <= now)) {
      return error_response("unauthorized", 401);
    }
    this.ctx.storage.sql.exec(
      "UPDATE access_tokens SET last_used_at = ? WHERE id = ?",
      now,
      row.id,
    );
    return json_response({
      access_token: {
        id: row.id,
        name: row.name,
        publisher_id: "caller:" + row.id,
      },
    });
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
    this.record_device_activity(attachment);

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
    this.mark_device_offline(socket);
    socket.close(code, reason || (was_clean ? "closed" : "connection lost"));
  }

  /**
   * @param {WebSocket} socket
   * @returns {Promise<void>}
   */
  async webSocketError(socket) {
    this.mark_device_offline(socket);
    socket.close(1011, "websocket error");
  }

  /** @returns {Promise<void>} */
  async alarm() {
    const now = Date.now();
    /** @type {CallRow[]} */
    const exhausted = this.ctx.storage.sql
      .exec(
        `SELECT * FROM calls
         WHERE status IN ('assigned', 'running')
           AND lease_expires_at <= ? AND attempt_count >= ?`,
        now,
        MAX_ATTEMPTS,
      )
      .toArray();

    this.ctx.storage.sql.exec(
      `UPDATE calls
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
      `UPDATE calls
       SET status = 'queued', assigned_device_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, updated_at = ?
       WHERE status IN ('assigned', 'running')
         AND lease_expires_at <= ? AND attempt_count < ?`,
      now,
      now,
      MAX_ATTEMPTS,
    );
    this.ctx.storage.sql.exec(
      `DELETE FROM calls
       WHERE status IN ('completed', 'failed') AND completed_at < ?`,
      now - RETENTION_MILLISECONDS,
    );

    for (const row of exhausted) {
      const failed_row = this.find_task(row.id);
      if (failed_row !== null) {
        this.resolve_invoke_waiter(failed_row.id, failed_row);
        this.send_to_device(failed_row.publisher_device_id, {
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
  record_device_activity(attachment) {
    this.ctx.storage.sql.exec(
      `UPDATE devices
       SET last_seen_at = ?, status = 'online', disconnected_at = NULL
       WHERE device_id = ? AND connection_id = ?`,
      Date.now(),
      attachment.device_id,
      attachment.connection_id,
    );
  }

  /**
   * @param {WebSocket} socket
   * @returns {void}
   */
  mark_device_offline(socket) {
    const attachment = socket_attachment(socket);
    if (attachment === null) {
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      `UPDATE devices
       SET status = 'offline', last_seen_at = ?, disconnected_at = ?
       WHERE device_id = ? AND connection_id = ?`,
      now,
      now,
      attachment.device_id,
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
    const device_id = (
      request.headers.get("X-Hub-Device-ID") ??
      request.headers.get("X-Hub-Client-ID") ??
      ""
    ).trim();
    if (!valid_identifier(device_id)) {
      return error_response("invalid X-Hub-Device-ID", 400);
    }
    const device_name = (request.headers.get("X-Hub-Device-Name") ?? device_id)
      .trim()
      .slice(0, 128) || device_id;
    const requested_device_os = (request.headers.get("X-Hub-Device-OS") ?? "unknown")
      .trim()
      .slice(0, 32);
    const device_os = valid_identifier(requested_device_os) ? requested_device_os : "unknown";
    const methods = (
      request.headers.get("X-Hub-Methods") ??
      request.headers.get("X-Hub-Capabilities") ??
      ""
    )
      .split(",")
      .map((item) => item.trim())
      .filter((item, index, values) => valid_identifier(item) && values.indexOf(item) === index)
      .slice(0, 64);

    for (const old_socket of this.ctx.getWebSockets(`client:${device_id}`)) {
      old_socket.close(1000, "replaced by a newer connection");
    }

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];
    const connection_id = crypto.randomUUID();
    const connected_at = Date.now();
    const tags = [`client:${device_id}`, ...methods.map((value) => `method:${value}`)];
    server.serializeAttachment({
      device_id,
      connection_id,
      methods,
      device_name,
      device_os,
      connected_at,
    });
    this.ctx.storage.sql.exec(
      `INSERT INTO devices (
         device_id, connection_id, methods_json, connected_at,
         device_name, device_os, last_seen_at, disconnected_at, status
       ) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, 'online')
       ON CONFLICT(device_id) DO UPDATE SET
         connection_id = excluded.connection_id,
         methods_json = excluded.methods_json,
         device_name = excluded.device_name,
         device_os = excluded.device_os,
         connected_at = excluded.connected_at,
         last_seen_at = excluded.last_seen_at,
         disconnected_at = NULL,
         status = 'online'`,
      device_id,
      connection_id,
      JSON.stringify(methods),
      connected_at,
      device_name,
      device_os,
      connected_at,
    );
    this.ctx.acceptWebSocket(server, tags);
    server.send(
      JSON.stringify({
        type: "device.connected",
        device_id,
        device_name,
        device_os,
        methods,
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
        "SELECT status, COUNT(*) AS count FROM calls GROUP BY status ORDER BY status",
      )
      .toArray();
    /** @type {SocketAttachment[]} */
    const active_devices = this.ctx
      .getWebSockets()
      .map(socket_attachment)
      .filter((item) => item !== null);
    const active_connection_ids = new Set(active_devices.map((device) => device.connection_id));
    /** @type {BusyDeviceRow[]} */
    const busy_rows = this.ctx.storage.sql
      .exec(
        `SELECT DISTINCT assigned_device_id FROM calls
         WHERE status IN ('assigned', 'running') AND assigned_device_id IS NOT NULL`,
      )
      .toArray();
    const busy_devices = new Set(busy_rows.map((row) => row.assigned_device_id));
    /** @type {DeviceRegistryRow[]} */
    const registry_rows = this.ctx.storage.sql
      .exec(
        `SELECT device_id, connection_id, methods_json, device_name, device_os, connected_at,
                last_seen_at, disconnected_at, status
         FROM devices ORDER BY device_id`,
      )
      .toArray();
    const now = Date.now();
    const devices = registry_rows.map((row) => {
      const online = active_connection_ids.has(row.connection_id);
      if (!online && row.status === "online") {
        this.ctx.storage.sql.exec(
          `UPDATE devices
           SET status = 'offline', disconnected_at = ?
           WHERE device_id = ? AND connection_id = ?`,
          now,
          row.device_id,
          row.connection_id,
        );
      }
      return {
        device_id: row.device_id,
        device_name: row.device_name || row.device_id,
        device_os: row.device_os || "unknown",
        methods: parse_methods(row.methods_json),
        connected_at: row.connected_at,
        last_seen_at: row.last_seen_at,
        disconnected_at: online ? null : row.disconnected_at ?? now,
        status: online ? (busy_devices.has(row.device_id) ? "busy" : "online") : "offline",
      };
    });
    const methods = [
      ...new Set(
        devices
          .filter((device) => device.status !== "offline")
          .flatMap((device) => device.methods),
      ),
    ].sort();
    return json_response({ devices, methods, task_counts });
  }

  /**
   * @param {Request} request
   * @param {boolean} [allow_idempotency_key]
   * @returns {Promise<Response>}
   */
  async create_task(request, allow_idempotency_key = true) {
    const publisher_device_id = (
      request.headers.get("X-Hub-Authenticated-Publisher-ID") ??
      request.headers.get("X-Hub-Publisher-ID") ??
      request.headers.get("X-Hub-Device-ID") ??
      request.headers.get("X-Hub-Client-ID") ??
      ""
    ).trim();
    if (!valid_identifier(publisher_device_id)) {
      return error_response("invalid X-Hub-Device-ID", 400);
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
    if (body === null || typeof body !== "object" || Array.isArray(body)) {
      return error_response("invalid JSON body", 400);
    }
    const requested_method = body.method ?? body.kind ?? "";
    const method = typeof requested_method === "string" ? requested_method.trim() : "";
    if (!valid_identifier(method)) {
      return error_response("invalid method", 400);
    }
    const requested_target_device_id = body.target_device_id ?? body.target_client_id ?? "";
    if (typeof requested_target_device_id !== "string") {
      return error_response("invalid target_device_id", 400);
    }
    const target_device_id =
      requested_target_device_id.trim();
    if (target_device_id !== "" && !valid_identifier(target_device_id)) {
      return error_response("invalid target_device_id", 400);
    }
    if (
      allow_idempotency_key &&
      body.idempotency_key !== undefined &&
      typeof body.idempotency_key !== "string"
    ) {
      return error_response("invalid idempotency_key", 400);
    }
    const idempotency_key = allow_idempotency_key
      ? (body.idempotency_key ?? "").trim()
      : "";
    if (idempotency_key.length > 128) {
      return error_response("idempotency_key is too long", 400);
    }
    const args = body.args ?? body.payload ?? {};
    if (args === null || typeof args !== "object" || Array.isArray(args)) {
      return error_response("args must be an object", 400);
    }
    const args_json = JSON.stringify(args);
    if (args_json.length > MAX_BODY_BYTES) {
      return error_response("args is too large", 413);
    }

    if (target_device_id !== "") {
      /** @type {DeviceRegistryRow | undefined} */
      const target_device = this.ctx.storage.sql
        .exec(
          "SELECT * FROM devices WHERE device_id = ? LIMIT 1",
          target_device_id,
        )
        .toArray()[0];
      if (target_device === undefined) {
        return error_response("target device is not registered", 404);
      }
      if (!parse_methods(target_device.methods_json).includes(method)) {
        return error_response("target device does not provide method " + method, 409);
      }
    }

    if (idempotency_key !== "") {
      /** @type {CallRow | undefined} */
      const existing = this.ctx.storage.sql
        .exec(
          "SELECT * FROM calls WHERE publisher_device_id = ? AND idempotency_key = ? LIMIT 1",
          publisher_device_id,
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
      `INSERT INTO calls (
        id, method, publisher_device_id, target_device_id,
        idempotency_key, args_json, status, attempt_count, created_at, updated_at
      ) VALUES (?, ?, ?, ?, ?, ?, 'queued', 0, ?, ?)`,
      task_id,
      method,
      publisher_device_id,
      target_device_id || null,
      idempotency_key || null,
      args_json,
      now,
      now,
    );
    await this.dispatch_pending();
    await this.schedule_alarm();
    const row = this.find_task(task_id);
    return json_response({ task: task_value(/** @type {CallRow} */ (row)) }, 201);
  }

  /**
   * @param {Request} request
   * @param {URL} url
   * @returns {Response}
   */
  list_tasks(request, url) {
    const publisher_device_id = (
      request.headers.get("X-Hub-Authenticated-Publisher-ID") ??
      url.searchParams.get("publisher_device_id") ??
      url.searchParams.get("publisher_id") ??
      ""
    ).trim();
    const status = (url.searchParams.get("status") ?? "").trim();
    const limit_value = Number(url.searchParams.get("limit") ?? "50");
    const limit = Number.isFinite(limit_value) ? Math.max(1, Math.min(200, limit_value)) : 50;

    /** @type {CallRow[]} */
    let rows;
    if (publisher_device_id !== "" && status !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM calls WHERE publisher_device_id = ? AND status = ? ORDER BY created_at DESC LIMIT ?",
          publisher_device_id,
          status,
          limit,
        )
        .toArray();
    } else if (publisher_device_id !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM calls WHERE publisher_device_id = ? ORDER BY created_at DESC LIMIT ?",
          publisher_device_id,
          limit,
        )
        .toArray();
    } else if (status !== "") {
      rows = this.ctx.storage.sql
        .exec(
          "SELECT * FROM calls WHERE status = ? ORDER BY created_at DESC LIMIT ?",
          status,
          limit,
        )
        .toArray();
    } else {
      rows = this.ctx.storage.sql
        .exec("SELECT * FROM calls ORDER BY created_at DESC LIMIT ?", limit)
        .toArray();
    }
    return json_response({ tasks: rows.map(task_value) });
  }

  /**
   * @param {Request} request
   * @param {string} task_id
   * @returns {Response}
   */
  get_task(request, task_id) {
    const row = this.find_task(task_id);
    const publisher_device_id = (
      request.headers.get("X-Hub-Authenticated-Publisher-ID") ?? ""
    ).trim();
    if (
      row === null ||
      (publisher_device_id !== "" && row.publisher_device_id !== publisher_device_id)
    ) {
      return error_response("task not found", 404);
    }
    return json_response({ task: task_value(row) });
  }

  /**
   * @param {string} task_id
   * @returns {CallRow | null}
   */
  find_task(task_id) {
    return (
      this.ctx.storage.sql
        .exec("SELECT * FROM calls WHERE id = ? LIMIT 1", task_id)
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
    const row = this.owned_task(attachment.device_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      "UPDATE calls SET status = 'running', lease_expires_at = ?, updated_at = ? WHERE id = ?",
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
    const row = this.owned_task(attachment.device_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    this.ctx.storage.sql.exec(
      "UPDATE calls SET lease_expires_at = ?, updated_at = ? WHERE id = ?",
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
      existing.assigned_device_id === attachment.device_id &&
      existing.lease_token === message.lease_token
    ) {
      socket.send(JSON.stringify({ type: "task.ack", task_id: existing.id }));
      return;
    }
    const row = this.owned_task(attachment.device_id, message);
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
      `UPDATE calls SET status = 'completed', result_json = ?, error_message = NULL,
       lease_expires_at = NULL, updated_at = ?, completed_at = ? WHERE id = ?`,
      result_json,
      now,
      now,
      row.id,
    );
    const completed = /** @type {CallRow} */ (this.find_task(row.id));
    this.resolve_invoke_waiter(completed.id, completed);
    socket.send(JSON.stringify({ type: "task.ack", task_id: row.id }));
    this.send_to_device(completed.publisher_device_id, {
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
    const row = this.owned_task(attachment.device_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      return;
    }
    const now = Date.now();
    const retryable = message.retryable === true && row.attempt_count < MAX_ATTEMPTS;
    if (retryable) {
      this.ctx.storage.sql.exec(
        `UPDATE calls SET status = 'queued', assigned_device_id = NULL, lease_token = NULL,
         lease_expires_at = NULL, error_message = ?, updated_at = ? WHERE id = ?`,
        (message.error ?? "task failed").slice(0, 4000),
        now,
        row.id,
      );
      socket.send(JSON.stringify({ type: "task.ack", task_id: row.id, requeued: true }));
    } else {
      this.ctx.storage.sql.exec(
        `UPDATE calls SET status = 'failed', error_message = ?, lease_expires_at = NULL,
         updated_at = ?, completed_at = ? WHERE id = ?`,
        (message.error ?? "task failed").slice(0, 4000),
        now,
        now,
        row.id,
      );
      const failed = /** @type {CallRow} */ (this.find_task(row.id));
      this.resolve_invoke_waiter(failed.id, failed);
      socket.send(JSON.stringify({ type: "task.ack", task_id: row.id }));
      this.send_to_device(failed.publisher_device_id, {
        type: "task.failed",
        task: task_value(failed),
      });
    }
    await this.dispatch_pending();
    await this.schedule_alarm();
  }

  /**
   * @param {string} device_id
   * @param {ClientMessage} message
   * @returns {CallRow | null}
   */
  owned_task(device_id, message) {
    if (!message.task_id || !message.lease_token) {
      return null;
    }
    const row = this.find_task(message.task_id);
    if (
      row === null ||
      (row.status !== "assigned" && row.status !== "running") ||
      row.assigned_device_id !== device_id ||
      row.lease_token !== message.lease_token
    ) {
      return null;
    }
    return row;
  }

  /** @returns {Promise<void>} */
  async dispatch_pending() {
    /** @type {CallRow[]} */
    const queued = this.ctx.storage.sql
      .exec("SELECT * FROM calls WHERE status = 'queued' ORDER BY created_at LIMIT 100")
      .toArray();
    if (queued.length === 0) {
      return;
    }
    /** @type {BusyDeviceRow[]} */
    const busy_rows = this.ctx.storage.sql
      .exec(
        `SELECT DISTINCT assigned_device_id FROM calls
         WHERE status IN ('assigned', 'running') AND assigned_device_id IS NOT NULL`,
      )
      .toArray();
    const busy_devices = new Set(busy_rows.map((row) => row.assigned_device_id));

    for (const row of queued) {
      const candidates = row.target_device_id
        ? this.ctx.getWebSockets(`client:${row.target_device_id}`)
        : this.ctx.getWebSockets(`method:${row.method}`);
      const candidate = candidates.find((socket) => {
        const attachment = socket_attachment(socket);
        return attachment !== null && !busy_devices.has(attachment.device_id);
      });
      if (candidate === undefined) {
        continue;
      }
      const attachment = /** @type {SocketAttachment} */ (socket_attachment(candidate));
      if (!attachment.methods.includes(row.method)) {
        continue;
      }

      const now = Date.now();
      const lease_token = crypto.randomUUID();
      this.ctx.storage.sql.exec(
        `UPDATE calls SET status = 'assigned', assigned_device_id = ?, lease_token = ?,
         lease_expires_at = ?, attempt_count = attempt_count + 1, updated_at = ?
         WHERE id = ? AND status = 'queued'`,
        attachment.device_id,
        lease_token,
        now + LEASE_MILLISECONDS,
        now,
        row.id,
      );
      const assigned = /** @type {CallRow} */ (this.find_task(row.id));
      try {
        candidate.send(
          JSON.stringify({
            type: "task.assigned",
            task: task_value(assigned),
            lease_token,
            lease_milliseconds: LEASE_MILLISECONDS,
          }),
        );
        busy_devices.add(attachment.device_id);
      } catch {
        this.ctx.storage.sql.exec(
          `UPDATE calls SET status = 'queued', assigned_device_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, updated_at = ? WHERE id = ?`,
          Date.now(),
          row.id,
        );
      }
    }
  }

  /**
   * @param {string} device_id
   * @param {unknown} value
   * @returns {void}
   */
  send_to_device(device_id, value) {
    const message = JSON.stringify(value);
    for (const socket of this.ctx.getWebSockets(`client:${device_id}`)) {
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
        `SELECT MIN(lease_expires_at) AS lease_expires_at FROM calls
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
  async fetch(request, env) {
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
      if (url.pathname === "/admin/api/call" && request.method === "POST") {
        return admin_create_call(env, request);
      }
      if (url.pathname === "/admin/api/access-tokens") {
        if (request.method !== "GET" && request.method !== "POST") {
          return error_response("method not allowed", 405);
        }
        return admin_access_token_request(env, request, "");
      }
      const admin_access_token_expire_match = url.pathname.match(
        /^\/admin\/api\/access-tokens\/([A-Za-z0-9-]+)\/expire$/,
      );
      if (admin_access_token_expire_match !== null && request.method === "POST") {
        return admin_access_token_request(
          env,
          request,
          "/" + admin_access_token_expire_match[1] + "/expire",
        );
      }
      const admin_access_token_match = url.pathname.match(
        /^\/admin\/api\/access-tokens\/([A-Za-z0-9-]+)$/,
      );
      if (admin_access_token_match !== null && request.method === "DELETE") {
        return admin_access_token_request(env, request, "/" + admin_access_token_match[1]);
      }
      const admin_task_match = url.pathname.match(/^\/admin\/api\/tasks\/([^/]+)$/);
      if (admin_task_match !== null && request.method === "GET") {
        return admin_task_status(
          env,
          admin_task_match[1],
        );
      }
      return error_response("not found", 404);
    }

    const legacy_match = url.pathname.match(/^\/v1\/hubs\/[^/]+(\/.*)?$/);
    let forwarded_path = "";
    if (url.pathname === "/v1") {
      forwarded_path = "/";
    } else if (legacy_match !== null) {
      forwarded_path = legacy_match[1] || "/";
    } else if (url.pathname.startsWith("/v1/")) {
      forwarded_path = url.pathname.slice(3);
    } else {
      return error_response("not found", 404);
    }
    if (forwarded_path.startsWith("/_admin/") || forwarded_path.startsWith("/_internal/")) {
      return error_response("not found", 404);
    }
    const forwarded_url = new URL(request.url);
    forwarded_url.pathname = forwarded_path;
    const object = env.HUBS.getByName(HUB_OBJECT_NAME);
    const token = bearer_token(request);
    const device_id = (
      request.headers.get("X-Hub-Device-ID") ??
      request.headers.get("X-Hub-Client-ID") ??
      ""
    ).trim();
    const device_authorized =
      Boolean(env.HUB_TOKEN) &&
      safe_equal(token, env.HUB_TOKEN) &&
      valid_identifier(device_id);
    if (forwarded_path === "/connect" && !device_authorized) {
      return error_response("unauthorized", 401);
    }
    if (device_authorized) {
      return object.fetch(new Request(forwarded_url, request));
    }

    const authorization_response = await object.fetch(
      "https://internal/_internal/access-tokens/verify",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      },
    );
    if (!authorization_response.ok) {
      return error_response("unauthorized", 401);
    }
    /** @type {{ access_token?: { id?: string, name?: string, publisher_id?: string } }} */
    const authorization = await authorization_response.json();
    const publisher_id = String(authorization.access_token?.publisher_id ?? "");
    if (!valid_identifier(publisher_id)) {
      return error_response("unauthorized", 401);
    }
    const headers = new Headers(request.headers);
    headers.delete("X-Hub-Publisher-ID");
    headers.delete("X-Hub-Device-ID");
    headers.delete("X-Hub-Client-ID");
    headers.set("X-Hub-Authenticated-Publisher-ID", publisher_id);
    headers.set("X-Hub-Access-Token-ID", String(authorization.access_token?.id ?? ""));
    return object.fetch(new Request(forwarded_url, new Request(request, { headers })));
  },
};

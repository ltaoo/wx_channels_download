import { DurableObject } from "cloudflare:workers";

/**
 * @typedef {Object} Env
 * @property {DurableObjectNamespace<BridgeDurableObject>} BRIDGES
 * @property {string} BRIDGE_TOKEN
 * @property {string} BRIDGE_ADMIN_TOKEN
 */

/** @typedef {"queued" | "assigned" | "running" | "completed" | "failed"} TaskStatus */

/**
 * @typedef {Object} CallRow
 * @property {string} id
 * @property {string} method
 * @property {string} publisher_device_id
 * @property {string | null} access_token_id
 * @property {number} credit_cost
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
 * @property {number | null} revoked_at
 * @property {number} credit_balance
 * @property {number} total_credits_granted
 * @property {number} total_credits_used
 */

/**
 * @typedef {Object} CreditTransactionRow
 * @property {string} id
 * @property {string} access_token_id
 * @property {string | null} task_id
 * @property {"grant" | "charge" | "adjustment"} type
 * @property {number} amount
 * @property {number} balance_after
 * @property {string | null} method
 * @property {string} note
 * @property {number} created_at
 */

/**
 * @typedef {Object} DeviceLogOptions
 * @property {string} [connection_id]
 * @property {string} [direction]
 * @property {string} [level]
 * @property {string} [task_id]
 * @property {string} [method]
 * @property {unknown} [metadata]
 * @property {number} [created_at]
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
 * @property {number} [credits]
 */

/**
 * @typedef {Object} AdjustCreditsBody
 * @property {number} [amount]
 * @property {string} [reason]
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

/**
 * @typedef {Object} DeviceLogRow
 * @property {number} id
 * @property {string} device_id
 * @property {string | null} connection_id
 * @property {string} category
 * @property {string} event_type
 * @property {string} direction
 * @property {string} level
 * @property {string | null} task_id
 * @property {string | null} method
 * @property {string} message
 * @property {string} metadata_json
 * @property {number} created_at
 */

const MAX_BODY_BYTES = 1024 * 1024;
const MAX_ATTEMPTS = 10;
const LEASE_MILLISECONDS = 120_000;
const RETENTION_MILLISECONDS = 7 * 24 * 60 * 60 * 1000;
const MAINTENANCE_MILLISECONDS = 24 * 60 * 60 * 1000;
const BRIDGE_OBJECT_NAME = "bridge";
const MAX_ACCESS_TOKENS = 500;
const MIN_ACCESS_TOKEN_LENGTH = 16;
const MAX_ACCESS_TOKEN_LENGTH = 256;
const MAX_ACCESS_TOKEN_LIFETIME_SECONDS = 366 * 24 * 60 * 60;
const INVOKE_TIMEOUT_MILLISECONDS = 10_000;
const DEFAULT_CALL_CREDIT_COST = 1;
const MAX_CREDIT_BALANCE = 1_000_000_000;
const MAX_CREDIT_ADJUSTMENT = 10_000_000;
const DEFAULT_DEVICE_LOG_LIMIT = 100;
const MAX_DEVICE_LOG_LIMIT = 500;
const MAX_DEVICE_LOG_MESSAGE_LENGTH = 2000;
const MAX_DEVICE_LOG_METADATA_LENGTH = 64 * 1024;
const DEVICE_LOG_CATEGORIES = new Set([
  "connection",
  "heartbeat",
  "call",
  "response",
  "system",
]);
const DEVICE_LOG_LEVELS = new Set(["info", "warn", "error"]);
const DEVICE_LOG_DIRECTIONS = new Set(["inbound", "outbound", "internal"]);
const SENSITIVE_LOG_FIELD_PATTERN =
  /(^|[-_])(authorization|cookie|password|passwd|secret|token)([-_]|$)/i;

/**
 * @param {unknown} value
 * @param {number} [status]
 * @param {HeadersInit} [headers]
 * @returns {Response}
 */
function json_response(value, status = 200, headers = {}) {
  return Response.json(value, {
    status,
    headers: { ...headers, "Cache-Control": "no-store" },
  });
}

/**
 * @param {string} error
 * @param {number} status
 * @param {HeadersInit} [headers]
 * @returns {Response}
 */
function error_response(error, status, headers = {}) {
  return json_response({ error }, status, headers);
}

/**
 * @param {string} value
 * @returns {boolean}
 */
function valid_identifier(value) {
  return /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(value);
}

/**
 * @param {string} value
 * @returns {string | null}
 */
function decode_identifier(value) {
  try {
    const decoded = decodeURIComponent(value);
    return valid_identifier(decoded) ? decoded : null;
  } catch {
    return null;
  }
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
  return "bridge_call_" + btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
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
  const visible_prefix_length = token.startsWith("bridge_call_") ? 20 : 4;
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
    status: row.revoked_at !== null ? "revoked" : expired ? "expired" : "active",
    expires_at: row.expires_at,
    created_at: row.created_at,
    last_used_at: row.last_used_at,
    credit_balance: Number(row.credit_balance),
    total_credits_granted: Number(row.total_credits_granted),
    total_credits_used: Number(row.total_credits_used),
  };
}

/**
 * @param {CreditTransactionRow} row
 * @returns {Record<string, unknown>}
 */
function credit_transaction_value(row) {
  return {
    id: row.id,
    access_token_id: row.access_token_id,
    task_id: row.task_id,
    type: row.type,
    amount: Number(row.amount),
    balance_after: Number(row.balance_after),
    method: row.method,
    note: row.note,
    created_at: row.created_at,
  };
}

/**
 * Keep pricing in one place so method-specific costs can be introduced without
 * changing the accounting or task schema.
 * @param {string} method
 * @returns {number}
 */
function credit_cost_for_method(method) {
  void method;
  return DEFAULT_CALL_CREDIT_COST;
}

/**
 * @param {number | null} balance
 * @returns {Record<string, string>}
 */
function credit_headers(balance) {
  if (balance === null) return {};
  return { "X-Bridge-Credit-Balance": String(balance) };
}

/**
 * @param {Request} request
 * @param {Env} env
 * @returns {boolean}
 */
function admin_authorized(request, env) {
  if (!env.BRIDGE_ADMIN_TOKEN) {
    return false;
  }
  const authorization = request.headers.get("Authorization") ?? "";
  if (authorization.startsWith("Bearer ")) {
    return safe_equal(authorization.slice(7), env.BRIDGE_ADMIN_TOKEN);
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
    return username === "admin" && safe_equal(password, env.BRIDGE_ADMIN_TOKEN);
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
      "WWW-Authenticate": 'Basic realm="WX Channels Bridge Admin", charset="UTF-8"',
    },
  });
}

/**
 * @param {Env} env
 * @returns {Promise<Response>}
 */
async function admin_overview(env) {
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  const [response, access_tokens_response] = await Promise.all([
    object.fetch("https://internal/"),
    object.fetch("https://internal/_admin/access-tokens"),
  ]);
  if (!response.ok || !access_tokens_response.ok) {
    return error_response("failed to load Bridge overview", 500);
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
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  return object.fetch(new Request("https://internal/_admin/access-tokens" + path, request));
}

/**
 * @param {Env} env
 * @param {Request} request
 * @returns {Promise<Response>}
 */
async function admin_credit_transactions(env, request) {
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  const source_url = new URL(request.url);
  const internal_url = new URL("https://internal/_admin/credit-transactions");
  internal_url.search = source_url.search;
  return object.fetch(new Request(internal_url, request));
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

  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  if (target_device_id !== "") {
    const summary_response = await object.fetch("https://internal/");
    if (!summary_response.ok) {
      return error_response("failed to load Bridge devices", 502);
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
      "X-Bridge-Publisher-ID": "admin-console",
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
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  return object.fetch("https://internal/tasks/" + encodeURIComponent(task_id));
}

/**
 * @param {Env} env
 * @param {Request} request
 * @param {string} device_id
 * @returns {Promise<Response>}
 */
async function admin_reset_device(env, request, device_id) {
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  return object.fetch(
    new Request("https://internal/_admin/devices/" + device_id + "/reset", request),
  );
}

/**
 * @param {Env} env
 * @param {Request} request
 * @param {string} device_id
 * @returns {Promise<Response>}
 */
async function admin_device_logs(env, request, device_id) {
  const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
  const source_url = new URL(request.url);
  const internal_url = new URL(
    "https://internal/_admin/devices/" + device_id + "/logs",
  );
  internal_url.search = source_url.search;
  return object.fetch(new Request(internal_url, request));
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
    credit_cost: Number(row.credit_cost),
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

/**
 * @param {unknown} value
 * @param {number} [depth]
 * @returns {unknown}
 */
function redact_log_value(value, depth = 0) {
  if (depth >= 12) return "[max depth]";
  if (Array.isArray(value)) {
    return value.map((item) => redact_log_value(item, depth + 1));
  }
  if (value === null || typeof value !== "object") {
    return value;
  }
  /** @type {Record<string, unknown>} */
  const result = {};
  for (const [key, item] of Object.entries(value)) {
    const normalized_key = key.replace(/([a-z0-9])([A-Z])/g, "$1_$2");
    result[key] = SENSITIVE_LOG_FIELD_PATTERN.test(normalized_key)
      ? "[redacted]"
      : redact_log_value(item, depth + 1);
  }
  return result;
}

/**
 * @param {unknown} metadata
 * @returns {string}
 */
function serialize_log_metadata(metadata) {
  let value;
  try {
    value = JSON.stringify(redact_log_value(metadata ?? {}));
  } catch (error) {
    value = JSON.stringify({
      serialization_error: error instanceof Error ? error.message : String(error),
    });
  }
  if (value.length <= MAX_DEVICE_LOG_METADATA_LENGTH) {
    return value;
  }
  return JSON.stringify({
    truncated: true,
    original_length: value.length,
    preview: value.slice(0, MAX_DEVICE_LOG_METADATA_LENGTH - 200),
  });
}

/**
 * @param {DeviceLogRow} row
 * @returns {Record<string, unknown>}
 */
function device_log_value(row) {
  let metadata = {};
  try {
    metadata = JSON.parse(row.metadata_json);
  } catch {
    metadata = { serialization_error: "invalid stored metadata" };
  }
  return {
    id: Number(row.id),
    device_id: row.device_id,
    connection_id: row.connection_id,
    category: row.category,
    event_type: row.event_type,
    direction: row.direction,
    level: row.level,
    task_id: row.task_id,
    method: row.method,
    message: row.message,
    metadata,
    created_at: row.created_at,
  };
}

export class BridgeDurableObject extends DurableObject {
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
        access_token_id TEXT,
        credit_cost INTEGER NOT NULL DEFAULT 0,
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
      CREATE TABLE IF NOT EXISTS device_logs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        device_id TEXT NOT NULL,
        connection_id TEXT,
        category TEXT NOT NULL,
        event_type TEXT NOT NULL,
        direction TEXT NOT NULL,
        level TEXT NOT NULL,
        task_id TEXT,
        method TEXT,
        message TEXT NOT NULL,
        metadata_json TEXT NOT NULL DEFAULT '{}',
        created_at INTEGER NOT NULL
      );
      CREATE INDEX IF NOT EXISTS device_logs_device_time
        ON device_logs(device_id, created_at DESC, id DESC);
      CREATE INDEX IF NOT EXISTS device_logs_retention
        ON device_logs(created_at);
      CREATE TABLE IF NOT EXISTS access_tokens (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        token_hash TEXT NOT NULL UNIQUE,
        token_hint TEXT NOT NULL,
        expires_at INTEGER,
        created_at INTEGER NOT NULL,
        last_used_at INTEGER,
        revoked_at INTEGER,
        credit_balance INTEGER NOT NULL DEFAULT 0,
        total_credits_granted INTEGER NOT NULL DEFAULT 0,
        total_credits_used INTEGER NOT NULL DEFAULT 0
      );
      CREATE INDEX IF NOT EXISTS access_tokens_expiration
        ON access_tokens(expires_at);
    `);
    const call_columns = this.ctx.storage.sql.exec("PRAGMA table_info(calls)").toArray();
    if (!call_columns.some((column) => column.name === "access_token_id")) {
      this.ctx.storage.sql.exec("ALTER TABLE calls ADD COLUMN access_token_id TEXT");
    }
    if (!call_columns.some((column) => column.name === "credit_cost")) {
      this.ctx.storage.sql.exec(
        "ALTER TABLE calls ADD COLUMN credit_cost INTEGER NOT NULL DEFAULT 0",
      );
    }
    const access_token_columns = this.ctx.storage.sql
      .exec("PRAGMA table_info(access_tokens)")
      .toArray();
    if (!access_token_columns.some((column) => column.name === "revoked_at")) {
      this.ctx.storage.sql.exec("ALTER TABLE access_tokens ADD COLUMN revoked_at INTEGER");
    }
    if (!access_token_columns.some((column) => column.name === "credit_balance")) {
      this.ctx.storage.sql.exec(
        "ALTER TABLE access_tokens ADD COLUMN credit_balance INTEGER NOT NULL DEFAULT 0",
      );
    }
    if (!access_token_columns.some((column) => column.name === "total_credits_granted")) {
      this.ctx.storage.sql.exec(
        "ALTER TABLE access_tokens ADD COLUMN total_credits_granted INTEGER NOT NULL DEFAULT 0",
      );
    }
    if (!access_token_columns.some((column) => column.name === "total_credits_used")) {
      this.ctx.storage.sql.exec(
        "ALTER TABLE access_tokens ADD COLUMN total_credits_used INTEGER NOT NULL DEFAULT 0",
      );
    }
    this.ctx.storage.sql.exec(`
      CREATE INDEX IF NOT EXISTS calls_access_token
        ON calls(access_token_id, created_at);
      CREATE TABLE IF NOT EXISTS credit_transactions (
        id TEXT PRIMARY KEY,
        access_token_id TEXT NOT NULL,
        task_id TEXT,
        type TEXT NOT NULL,
        amount INTEGER NOT NULL,
        balance_after INTEGER NOT NULL,
        method TEXT,
        note TEXT NOT NULL DEFAULT '',
        created_at INTEGER NOT NULL
      );
      CREATE INDEX IF NOT EXISTS credit_transactions_token_time
        ON credit_transactions(access_token_id, created_at DESC);
      CREATE UNIQUE INDEX IF NOT EXISTS credit_transactions_task_charge
        ON credit_transactions(task_id) WHERE type = 'charge';
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
    if (url.pathname === "/credits" && request.method === "GET") {
      return this.credit_usage(request);
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
    if (url.pathname === "/_admin/credit-transactions" && request.method === "GET") {
      return this.list_credit_transactions(url);
    }
    const access_token_credits_match = url.pathname.match(
      /^\/_admin\/access-tokens\/([A-Za-z0-9-]+)\/credits$/,
    );
    if (access_token_credits_match && request.method === "POST") {
      return this.adjust_access_token_credits(request, access_token_credits_match[1]);
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
    const rejected_connection_match = url.pathname.match(
      /^\/_internal\/devices\/([^/]+)\/connection-rejected$/,
    );
    const rejected_connection_device_id = rejected_connection_match
      ? decode_identifier(rejected_connection_match[1])
      : null;
    if (rejected_connection_device_id !== null && request.method === "POST") {
      this.record_device_log(
        rejected_connection_device_id,
        "connection",
        "connection.auth_rejected",
        "设备连接鉴权失败",
        { direction: "inbound", level: "warn" },
      );
      return json_response({ recorded: true });
    }
    const device_reset_match = url.pathname.match(/^\/_admin\/devices\/([^/]+)\/reset$/);
    const reset_device_id = device_reset_match
      ? decode_identifier(device_reset_match[1])
      : null;
    if (reset_device_id !== null && request.method === "POST") {
      return this.reset_device(reset_device_id);
    }
    const device_logs_match = url.pathname.match(/^\/_admin\/devices\/([^/]+)\/logs$/);
    const logs_device_id = device_logs_match
      ? decode_identifier(device_logs_match[1])
      : null;
    if (logs_device_id !== null && request.method === "GET") {
      return this.list_device_logs(url, logs_device_id);
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

    /** @type {{ task?: { id?: string }, credits?: { charged?: number, balance?: number } }} */
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
    const credit_balance = Number.isInteger(created.credits?.balance)
      ? Number(created.credits.balance)
      : null;
    const invoke_headers = credit_balance === null
      ? {}
      : {
          ...credit_headers(credit_balance),
          "X-Bridge-Credits-Charged": String(created.credits?.charged ?? 0),
        };

    const remaining_milliseconds = Math.max(0, deadline - Date.now());
    const row = await this.wait_for_invoke(task_id, remaining_milliseconds);
    if (row === null) {
      return error_response("invoke timed out after 10 seconds", 504, invoke_headers);
    }
    if (row.status === "failed") {
      return error_response(row.error_message ?? "invoke failed", 502, invoke_headers);
    }
    if (row.status !== "completed") {
      return error_response("invoke ended without a result", 500, invoke_headers);
    }
    try {
      return json_response(
        row.result_json === null ? null : JSON.parse(row.result_json),
        200,
        invoke_headers,
      );
    } catch {
      return error_response("invoke returned invalid JSON", 502, invoke_headers);
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
      .exec("SELECT * FROM access_tokens WHERE revoked_at IS NULL ORDER BY created_at DESC")
      .toArray();
    const now = Date.now();
    return json_response({ access_tokens: rows.map((row) => access_token_value(row, now)) });
  }

  /**
   * @param {URL} url
   * @returns {Response}
   */
  list_credit_transactions(url) {
    const access_token_id = (url.searchParams.get("access_token_id") ?? "").trim();
    if (access_token_id !== "" && !valid_identifier(access_token_id)) {
      return error_response("invalid access_token_id", 400);
    }
    const limit_value = Number(url.searchParams.get("limit") ?? "100");
    const limit = Number.isFinite(limit_value)
      ? Math.max(1, Math.min(500, Math.floor(limit_value)))
      : 100;
    /** @type {CreditTransactionRow[]} */
    const rows = access_token_id === ""
      ? this.ctx.storage.sql
          .exec("SELECT * FROM credit_transactions ORDER BY created_at DESC LIMIT ?", limit)
          .toArray()
      : this.ctx.storage.sql
          .exec(
            `SELECT * FROM credit_transactions
             WHERE access_token_id = ? ORDER BY created_at DESC LIMIT ?`,
            access_token_id,
            limit,
          )
          .toArray();
    return json_response({ credit_transactions: rows.map(credit_transaction_value) });
  }

  /**
   * @param {Request} request
   * @returns {Response}
   */
  credit_usage(request) {
    const access_token_id = (request.headers.get("X-Bridge-Access-Token-ID") ?? "").trim();
    if (!valid_identifier(access_token_id)) {
      return error_response("credit usage is only available to access tokens", 403);
    }
    /** @type {AccessTokenRow | undefined} */
    const row = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
      .toArray()[0];
    const now = Date.now();
    if (
      row === undefined ||
      row.revoked_at !== null ||
      (row.expires_at !== null && row.expires_at <= now)
    ) {
      return error_response("unauthorized", 401);
    }
    return json_response({
      credits: {
        balance: Number(row.credit_balance),
        total_granted: Number(row.total_credits_granted),
        total_used: Number(row.total_credits_used),
        default_call_cost: DEFAULT_CALL_CREDIT_COST,
      },
    });
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
    const credits = body.credits === undefined ? 0 : Number(body.credits);
    if (!Number.isInteger(credits) || credits < 0 || credits > MAX_CREDIT_BALANCE) {
      return error_response("credits must be an integer between 0 and 1000000000", 400);
    }
    const token_count = Number(
      this.ctx.storage.sql
        .exec("SELECT COUNT(*) AS count FROM access_tokens WHERE revoked_at IS NULL")
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
    this.ctx.storage.transactionSync(() => {
      this.ctx.storage.sql.exec(
        `INSERT INTO access_tokens (
          id, name, token_hash, token_hint, expires_at, created_at, last_used_at,
          revoked_at, credit_balance, total_credits_granted, total_credits_used
        ) VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, 0)`,
        id,
        name,
        token_hash,
        token_hint,
        expires_at,
        now,
        credits,
        credits,
      );
      if (credits > 0) {
        this.ctx.storage.sql.exec(
          `INSERT INTO credit_transactions (
            id, access_token_id, task_id, type, amount, balance_after, method, note, created_at
          ) VALUES (?, ?, NULL, 'grant', ?, ?, NULL, 'initial credits', ?)`,
          crypto.randomUUID(),
          id,
          credits,
          credits,
          now,
        );
      }
    });
    /** @type {AccessTokenRow} */
    const row = this.ctx.storage.sql
      .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", id)
      .toArray()[0];
    return json_response({ access_token: access_token_value(row, now), token }, 201);
  }

  /**
   * @param {Request} request
   * @param {string} access_token_id
   * @returns {Promise<Response>}
   */
  async adjust_access_token_credits(request, access_token_id) {
    const content_length = Number(request.headers.get("Content-Length") ?? "0");
    if (content_length > 16 * 1024) {
      return error_response("request body is too large", 413);
    }
    /** @type {AdjustCreditsBody} */
    let body;
    try {
      body = await request.json();
    } catch {
      return error_response("invalid JSON", 400);
    }
    if (body === null || typeof body !== "object" || Array.isArray(body)) {
      return error_response("invalid JSON body", 400);
    }
    const amount = Number(body.amount);
    if (
      !Number.isInteger(amount) ||
      amount === 0 ||
      Math.abs(amount) > MAX_CREDIT_ADJUSTMENT
    ) {
      return error_response("amount must be a non-zero integer between -10000000 and 10000000", 400);
    }
    if (body.reason !== undefined && typeof body.reason !== "string") {
      return error_response("reason must be a string", 400);
    }
    const reason = typeof body.reason === "string" ? body.reason.trim() : "";
    if (reason.length > 256 || /[\r\n]/.test(reason)) {
      return error_response("reason must not exceed 256 characters or contain newlines", 400);
    }
    const now = Date.now();
    const adjustment = this.ctx.storage.transactionSync(() => {
      /** @type {AccessTokenRow | undefined} */
      const existing = this.ctx.storage.sql
        .exec(
          "SELECT * FROM access_tokens WHERE id = ? AND revoked_at IS NULL LIMIT 1",
          access_token_id,
        )
        .toArray()[0];
      if (existing === undefined) return { error: "not_found" };
      const balance = Number(existing.credit_balance) + amount;
      if (balance < 0) return { error: "negative_balance", balance: Number(existing.credit_balance) };
      if (balance > MAX_CREDIT_BALANCE) return { error: "balance_limit" };
      this.ctx.storage.sql.exec(
        `UPDATE access_tokens
         SET credit_balance = ?,
             total_credits_granted = total_credits_granted + ?
         WHERE id = ?`,
        balance,
        Math.max(0, amount),
        access_token_id,
      );
      this.ctx.storage.sql.exec(
        `INSERT INTO credit_transactions (
          id, access_token_id, task_id, type, amount, balance_after, method, note, created_at
        ) VALUES (?, ?, NULL, ?, ?, ?, NULL, ?, ?)`,
        crypto.randomUUID(),
        access_token_id,
        amount > 0 ? "grant" : "adjustment",
        amount,
        balance,
        reason || (amount > 0 ? "admin credit grant" : "admin credit adjustment"),
        now,
      );
      /** @type {AccessTokenRow} */
      const row = this.ctx.storage.sql
        .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
        .toArray()[0];
      return { row };
    });
    if (adjustment.error === "not_found") {
      return error_response("access token not found", 404);
    }
    if (adjustment.error === "negative_balance") {
      return json_response(
        { error: "credit adjustment would make the balance negative", balance: adjustment.balance },
        409,
      );
    }
    if (adjustment.error === "balance_limit") {
      return error_response("credit balance limit exceeded", 409);
    }
    return json_response({ access_token: access_token_value(adjustment.row, now) });
  }

  /**
   * @param {string} access_token_id
   * @returns {Response}
   */
  expire_access_token(access_token_id) {
    /** @type {AccessTokenRow | undefined} */
    const existing = this.ctx.storage.sql
      .exec(
        "SELECT * FROM access_tokens WHERE id = ? AND revoked_at IS NULL LIMIT 1",
        access_token_id,
      )
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
      .exec(
        "SELECT id FROM access_tokens WHERE id = ? AND revoked_at IS NULL LIMIT 1",
        access_token_id,
      )
      .toArray()[0];
    if (existing === undefined) {
      return error_response("access token not found", 404);
    }
    this.ctx.storage.sql.exec(
      "UPDATE access_tokens SET revoked_at = ? WHERE id = ?",
      Date.now(),
      access_token_id,
    );
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
    if (
      row === undefined ||
      row.revoked_at !== null ||
      (row.expires_at !== null && row.expires_at <= now)
    ) {
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
        credit_balance: Number(row.credit_balance),
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
      this.record_device_log(
        attachment.device_id,
        "system",
        "protocol.message_rejected",
        "拒绝过大的设备消息",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          metadata: { message_length: text.length, reason: "message is too large" },
        },
      );
      return;
    }

    /** @type {ClientMessage} */
    let value;
    try {
      value = JSON.parse(text);
    } catch {
      socket.send(JSON.stringify({ type: "error", error: "invalid JSON" }));
      this.record_device_log(
        attachment.device_id,
        "system",
        "protocol.message_rejected",
        "拒绝无法解析的设备消息",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          metadata: { message_length: text.length, reason: "invalid JSON" },
        },
      );
      return;
    }

    switch (value.type) {
      case "client.heartbeat":
        socket.send(JSON.stringify({ type: "client.heartbeat.ack", at: Date.now() }));
        this.record_device_log(
          attachment.device_id,
          "heartbeat",
          "heartbeat.connection",
          "收到连接心跳并已响应",
          {
            connection_id: attachment.connection_id,
            direction: "inbound",
            metadata: { request: value, response_type: "client.heartbeat.ack" },
          },
        );
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
        this.record_device_log(
          attachment.device_id,
          "system",
          "protocol.message_rejected",
          "拒绝未知类型的设备消息",
          {
            connection_id: attachment.connection_id,
            direction: "inbound",
            level: "warn",
            task_id: value.task_id,
            metadata: { request: value, reason: "unknown message type" },
          },
        );
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
    this.mark_device_offline(socket, "connection.closed", "设备连接已关闭", {
      code,
      reason,
      was_clean,
    });
    socket.close(code, reason || (was_clean ? "closed" : "connection lost"));
  }

  /**
   * @param {WebSocket} socket
   * @returns {Promise<void>}
   */
  async webSocketError(socket) {
    this.mark_device_offline(socket, "connection.error", "设备连接发生错误", {});
    socket.close(1011, "websocket error");
  }

  /** @returns {Promise<void>} */
  async alarm() {
    const now = Date.now();
    /** @type {CallRow[]} */
    const expired = this.ctx.storage.sql
      .exec(
        `SELECT * FROM calls
         WHERE status IN ('assigned', 'running')
           AND lease_expires_at <= ?`,
        now,
      )
      .toArray();
    const exhausted = expired.filter((row) => row.attempt_count >= MAX_ATTEMPTS);

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
    this.ctx.storage.sql.exec(
      "DELETE FROM device_logs WHERE created_at < ?",
      now - RETENTION_MILLISECONDS,
    );

    for (const row of expired) {
      if (!row.assigned_device_id) continue;
      const task_failed = row.attempt_count >= MAX_ATTEMPTS;
      this.record_device_log(
        row.assigned_device_id,
        task_failed ? "response" : "call",
        task_failed ? "response.lease_exhausted" : "call.lease_expired",
        task_failed
          ? "调用租约反复过期，任务已失败"
          : "调用租约已过期，任务重新排队",
        {
          direction: "internal",
          level: task_failed ? "error" : "warn",
          task_id: row.id,
          method: row.method,
          metadata: {
            attempt_count: row.attempt_count,
            max_attempts: MAX_ATTEMPTS,
            lease_expires_at: row.lease_expires_at,
          },
          created_at: now,
        },
      );
    }

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
   * @param {string} event_type
   * @param {string} message
   * @param {unknown} metadata
   * @returns {void}
   */
  mark_device_offline(socket, event_type, message, metadata) {
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
    this.record_device_log(attachment.device_id, "connection", event_type, message, {
      connection_id: attachment.connection_id,
      direction: "internal",
      level: event_type === "connection.error" ? "error" : "info",
      metadata,
      created_at: now,
    });
  }

  /**
   * @param {Request} request
   * @returns {Promise<Response>}
   */
  async accept_connection(request) {
    const device_id = (
      request.headers.get("X-Bridge-Device-ID") ??
      request.headers.get("X-Bridge-Client-ID") ??
      ""
    ).trim();
    if (!valid_identifier(device_id)) {
      return error_response("invalid X-Bridge-Device-ID", 400);
    }
    if (request.headers.get("Upgrade")?.toLowerCase() !== "websocket") {
      this.record_device_log(
        device_id,
        "connection",
        "connection.upgrade_rejected",
        "连接请求不是 WebSocket Upgrade，已拒绝",
        { direction: "inbound", level: "warn" },
      );
      return error_response("websocket upgrade required", 426);
    }
    const device_name = (request.headers.get("X-Bridge-Device-Name") ?? device_id)
      .trim()
      .slice(0, 128) || device_id;
    const requested_device_os = (request.headers.get("X-Bridge-Device-OS") ?? "unknown")
      .trim()
      .slice(0, 32);
    const device_os = valid_identifier(requested_device_os) ? requested_device_os : "unknown";
    const methods = (
      request.headers.get("X-Bridge-Methods") ??
      request.headers.get("X-Bridge-Capabilities") ??
      ""
    )
      .split(",")
      .map((item) => item.trim())
      .filter((item, index, values) => valid_identifier(item) && values.indexOf(item) === index)
      .slice(0, 64);

    const connection_id = crypto.randomUUID();
    const connected_at = Date.now();

    for (const old_socket of this.ctx.getWebSockets(`client:${device_id}`)) {
      const old_attachment = socket_attachment(old_socket);
      this.record_device_log(
        device_id,
        "connection",
        "connection.replaced",
        "旧连接被新连接替换",
        {
          connection_id: old_attachment?.connection_id,
          direction: "internal",
          level: "warn",
          metadata: { replacement_connection_id: connection_id },
          created_at: connected_at,
        },
      );
      old_socket.close(1000, "replaced by a newer connection");
    }

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];
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
    this.record_device_log(
      device_id,
      "connection",
      "connection.connected",
      "设备已连接并完成注册",
      {
        connection_id,
        direction: "inbound",
        metadata: { device_name, device_os, methods },
        created_at: connected_at,
      },
    );
    await this.dispatch_pending();
    await this.schedule_alarm();
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
        this.record_device_log(
          row.device_id,
          "connection",
          "connection.stale",
          "连接已不存在，设备状态已修正为离线",
          {
            connection_id: row.connection_id,
            direction: "internal",
            level: "warn",
            created_at: now,
          },
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
   * @param {string} device_id
   * @param {string} category
   * @param {string} event_type
   * @param {string} message
   * @param {DeviceLogOptions} [options]
   * @returns {void}
   */
  record_device_log(device_id, category, event_type, message, options = {}) {
    if (!valid_identifier(device_id)) return;
    const normalized_category = DEVICE_LOG_CATEGORIES.has(category) ? category : "system";
    const direction = DEVICE_LOG_DIRECTIONS.has(options.direction ?? "")
      ? options.direction
      : "internal";
    const level = DEVICE_LOG_LEVELS.has(options.level ?? "") ? options.level : "info";
    try {
      this.ctx.storage.sql.exec(
        `INSERT INTO device_logs (
          device_id, connection_id, category, event_type, direction, level,
          task_id, method, message, metadata_json, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        device_id,
        options.connection_id || null,
        normalized_category,
        String(event_type || "system.event").slice(0, 128),
        direction,
        level,
        options.task_id || null,
        options.method || null,
        String(message || event_type || "设备事件").slice(0, MAX_DEVICE_LOG_MESSAGE_LENGTH),
        serialize_log_metadata(options.metadata),
        options.created_at ?? Date.now(),
      );
    } catch (error) {
      console.error("failed to persist device log", {
        device_id,
        event_type,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }

  /**
   * @param {URL} url
   * @param {string} device_id
   * @returns {Response}
   */
  list_device_logs(url, device_id) {
    /** @type {DeviceRegistryRow | undefined} */
    const device = this.ctx.storage.sql
      .exec("SELECT * FROM devices WHERE device_id = ? LIMIT 1", device_id)
      .toArray()[0];
    if (device === undefined) {
      return error_response("device not found", 404);
    }
    const limit_value = Number(url.searchParams.get("limit") ?? DEFAULT_DEVICE_LOG_LIMIT);
    const limit = Number.isFinite(limit_value)
      ? Math.max(1, Math.min(MAX_DEVICE_LOG_LIMIT, Math.floor(limit_value)))
      : DEFAULT_DEVICE_LOG_LIMIT;
    const before_value = url.searchParams.get("before_id");
    const before_id = before_value === null || before_value === "" ? null : Number(before_value);
    if (before_id !== null && (!Number.isSafeInteger(before_id) || before_id <= 0)) {
      return error_response("invalid before_id", 400);
    }
    const category = (url.searchParams.get("category") ?? "").trim();
    if (category !== "" && !DEVICE_LOG_CATEGORIES.has(category)) {
      return error_response("invalid category", 400);
    }

    /** @type {DeviceLogRow[]} */
    let rows;
    if (before_id !== null && category !== "") {
      rows = this.ctx.storage.sql
        .exec(
          `SELECT * FROM device_logs
           WHERE device_id = ? AND category = ? AND id < ?
           ORDER BY id DESC LIMIT ?`,
          device_id,
          category,
          before_id,
          limit + 1,
        )
        .toArray();
    } else if (before_id !== null) {
      rows = this.ctx.storage.sql
        .exec(
          `SELECT * FROM device_logs WHERE device_id = ? AND id < ?
           ORDER BY id DESC LIMIT ?`,
          device_id,
          before_id,
          limit + 1,
        )
        .toArray();
    } else if (category !== "") {
      rows = this.ctx.storage.sql
        .exec(
          `SELECT * FROM device_logs WHERE device_id = ? AND category = ?
           ORDER BY id DESC LIMIT ?`,
          device_id,
          category,
          limit + 1,
        )
        .toArray();
    } else {
      rows = this.ctx.storage.sql
        .exec(
          `SELECT * FROM device_logs WHERE device_id = ?
           ORDER BY id DESC LIMIT ?`,
          device_id,
          limit + 1,
        )
        .toArray();
    }
    const has_more = rows.length > limit;
    const page = has_more ? rows.slice(0, limit) : rows;
    return json_response({
      device: {
        device_id: device.device_id,
        device_name: device.device_name || device.device_id,
        device_os: device.device_os || "unknown",
      },
      logs: page.map(device_log_value),
      has_more,
      next_before_id: has_more && page.length > 0 ? page[page.length - 1].id : null,
      retention_milliseconds: RETENTION_MILLISECONDS,
    });
  }

  /**
   * @param {Request} request
   * @param {boolean} [allow_idempotency_key]
   * @returns {Promise<Response>}
   */
  async create_task(request, allow_idempotency_key = true) {
    const publisher_device_id = (
      request.headers.get("X-Bridge-Authenticated-Publisher-ID") ??
      request.headers.get("X-Bridge-Publisher-ID") ??
      request.headers.get("X-Bridge-Device-ID") ??
      request.headers.get("X-Bridge-Client-ID") ??
      ""
    ).trim();
    if (!valid_identifier(publisher_device_id)) {
      return error_response("invalid X-Bridge-Device-ID", 400);
    }
    const access_token_id = (request.headers.get("X-Bridge-Access-Token-ID") ?? "").trim();
    if (
      access_token_id !== "" &&
      (!valid_identifier(access_token_id) || publisher_device_id !== "caller:" + access_token_id)
    ) {
      return error_response("unauthorized", 401);
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
        this.record_device_log(
          target_device_id,
          "call",
          "call.rejected",
          "设备不支持请求的方法，调用已拒绝",
          {
            direction: "internal",
            level: "warn",
            method,
            metadata: { publisher_device_id, args, reason: "unsupported method" },
          },
        );
        return error_response("target device does not provide method " + method, 409);
      }
    }

    const now = Date.now();
    const task_id = crypto.randomUUID();
    const credit_cost = access_token_id === "" ? 0 : credit_cost_for_method(method);
    const creation = this.ctx.storage.transactionSync(() => {
      /** @type {AccessTokenRow | undefined} */
      const access_token = access_token_id === ""
        ? undefined
        : this.ctx.storage.sql
            .exec("SELECT * FROM access_tokens WHERE id = ? LIMIT 1", access_token_id)
            .toArray()[0];
      if (
        access_token_id !== "" &&
        (
          access_token === undefined ||
          access_token.revoked_at !== null ||
          (access_token.expires_at !== null && access_token.expires_at <= now)
        )
      ) {
        return { error: "unauthorized" };
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
          return {
            row: existing,
            replay: true,
            balance: access_token === undefined ? null : Number(access_token.credit_balance),
          };
        }
      }
      const current_balance = access_token === undefined
        ? null
        : Number(access_token.credit_balance);
      if (current_balance !== null && current_balance < credit_cost) {
        return { error: "insufficient_credits", balance: current_balance };
      }
      const balance_after = current_balance === null ? null : current_balance - credit_cost;
      if (access_token !== undefined) {
        this.ctx.storage.sql.exec(
          `UPDATE access_tokens
           SET credit_balance = ?, total_credits_used = total_credits_used + ?
           WHERE id = ?`,
          balance_after,
          credit_cost,
          access_token_id,
        );
      }
      this.ctx.storage.sql.exec(
        `INSERT INTO calls (
          id, method, publisher_device_id, access_token_id, credit_cost, target_device_id,
          idempotency_key, args_json, status, attempt_count, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'queued', 0, ?, ?)`,
        task_id,
        method,
        publisher_device_id,
        access_token_id || null,
        credit_cost,
        target_device_id || null,
        idempotency_key || null,
        args_json,
        now,
        now,
      );
      if (access_token !== undefined) {
        this.ctx.storage.sql.exec(
          `INSERT INTO credit_transactions (
            id, access_token_id, task_id, type, amount, balance_after, method, note, created_at
          ) VALUES (?, ?, ?, 'charge', ?, ?, ?, 'Bridge call', ?)`,
          crypto.randomUUID(),
          access_token_id,
          task_id,
          -credit_cost,
          balance_after,
          method,
          now,
        );
      }
      return {
        row: /** @type {CallRow} */ (this.find_task(task_id)),
        replay: false,
        balance: balance_after,
      };
    });
    if (creation.error === "unauthorized") {
      return error_response("unauthorized", 401);
    }
    if (creation.error === "insufficient_credits") {
      if (target_device_id !== "") {
        this.record_device_log(
          target_device_id,
          "call",
          "call.rejected",
          "调用方积分不足，调用未创建",
          {
            direction: "internal",
            level: "warn",
            method,
            metadata: {
              publisher_device_id,
              required_credits: credit_cost,
              balance: creation.balance,
            },
          },
        );
      }
      return json_response(
        {
          error: "insufficient credits",
          required: credit_cost,
          balance: creation.balance,
        },
        402,
        credit_headers(creation.balance),
      );
    }
    if (creation.replay) {
      const replay_device_id = creation.row.assigned_device_id ?? target_device_id;
      if (replay_device_id !== "") {
        this.record_device_log(
          replay_device_id,
          "call",
          "call.idempotent_replay",
          "返回已有调用，未重复创建任务",
          {
            direction: "internal",
            task_id: creation.row.id,
            method: creation.row.method,
            metadata: {
              publisher_device_id,
              idempotency_key,
              status: creation.row.status,
            },
          },
        );
      }
      const replay_publisher_is_device = this.ctx.storage.sql
        .exec("SELECT 1 FROM devices WHERE device_id = ? LIMIT 1", publisher_device_id)
        .toArray().length > 0;
      if (replay_publisher_is_device && replay_device_id !== publisher_device_id) {
        this.record_device_log(
          publisher_device_id,
          "call",
          "call.idempotent_replay",
          "设备发布的调用命中已有任务",
          {
            direction: "outbound",
            task_id: creation.row.id,
            method: creation.row.method,
            metadata: {
              target_device_id: creation.row.target_device_id,
              idempotency_key,
              status: creation.row.status,
            },
          },
        );
      }
      return json_response(
        {
          task: task_value(creation.row),
          idempotent_replay: true,
          credits: { charged: 0, balance: creation.balance },
        },
        200,
        credit_headers(creation.balance),
      );
    }
    const publisher_is_device = this.ctx.storage.sql
      .exec("SELECT 1 FROM devices WHERE device_id = ? LIMIT 1", publisher_device_id)
      .toArray().length > 0;
    if (publisher_is_device) {
      this.record_device_log(
        publisher_device_id,
        "call",
        "call.published",
        "设备已发布调用",
        {
          direction: "outbound",
          task_id,
          method,
          metadata: {
            target_device_id: target_device_id || null,
            idempotency_key: idempotency_key || null,
            args,
          },
        },
      );
    }
    if (target_device_id !== "") {
      this.record_device_log(
        target_device_id,
        "call",
        "call.queued",
        "收到定向调用并进入等待队列",
        {
          direction: "inbound",
          task_id: task_id,
          method,
          metadata: {
            publisher_device_id,
            idempotency_key: idempotency_key || null,
            args,
          },
        },
      );
    }
    await this.dispatch_pending();
    await this.schedule_alarm();
    return json_response(
      {
        task: task_value(creation.row),
        credits: { charged: credit_cost, balance: creation.balance },
      },
      201,
      credit_headers(creation.balance),
    );
  }

  /**
   * @param {Request} request
   * @param {URL} url
   * @returns {Response}
   */
  list_tasks(request, url) {
    const publisher_device_id = (
      request.headers.get("X-Bridge-Authenticated-Publisher-ID") ??
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
      request.headers.get("X-Bridge-Authenticated-Publisher-ID") ?? ""
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
   * @param {string} device_id
   * @returns {Promise<Response>}
   */
  async reset_device(device_id) {
    const device = this.ctx.storage.sql
      .exec("SELECT device_id FROM devices WHERE device_id = ? LIMIT 1", device_id)
      .toArray()[0];
    if (device === undefined) {
      return error_response("device not found", 404);
    }

    /** @type {CallRow[]} */
    const rows = this.ctx.storage.sql
      .exec(
        `SELECT * FROM calls
         WHERE assigned_device_id = ? AND status IN ('assigned', 'running')
         ORDER BY created_at`,
        device_id,
      )
      .toArray();
    const now = Date.now();
    this.ctx.storage.sql.exec(
      `UPDATE calls
       SET status = 'failed', assigned_device_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, error_message = 'task force reset by admin',
           updated_at = ?, completed_at = ?
       WHERE assigned_device_id = ? AND status IN ('assigned', 'running')`,
      now,
      now,
      device_id,
    );

    const tasks = [];
    for (const row of rows) {
      const failed = this.find_task(row.id);
      if (failed === null) continue;
      tasks.push(task_value(failed));
      this.record_device_log(
        device_id,
        "response",
        "response.admin_reset",
        "管理员强制终止了设备上的调用",
        {
          direction: "internal",
          level: "error",
          task_id: failed.id,
          method: failed.method,
          metadata: { error: failed.error_message },
          created_at: now,
        },
      );
      this.resolve_invoke_waiter(failed.id, failed);
      this.send_to_device(failed.publisher_device_id, {
        type: "task.failed",
        task: task_value(failed),
      });
    }
    this.record_device_log(
      device_id,
      "system",
      "system.admin_reset",
      "管理员执行了设备强制重置",
      {
        direction: "internal",
        level: rows.length > 0 ? "warn" : "info",
        metadata: { reset_count: tasks.length },
        created_at: now,
      },
    );
    await this.schedule_alarm();
    return json_response({ device_id, reset_count: tasks.length, tasks });
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
      this.record_device_log(
        attachment.device_id,
        "call",
        "call.accept_rejected",
        "设备确认的调用不属于当前连接，已拒绝",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: message.task_id,
          metadata: { request_type: message.type },
        },
      );
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
    this.record_device_log(
      attachment.device_id,
      "call",
      "call.accepted",
      "设备已接收调用并开始执行",
      {
        connection_id: attachment.connection_id,
        direction: "inbound",
        task_id: row.id,
        method: row.method,
        metadata: {
          attempt_count: row.attempt_count,
          lease_expires_at: now + LEASE_MILLISECONDS,
        },
        created_at: now,
      },
    );
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
      this.record_device_log(
        attachment.device_id,
        "heartbeat",
        "heartbeat.task_rejected",
        "调用心跳与当前租约不匹配，已拒绝",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: message.task_id,
        },
      );
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
    this.record_device_log(
      attachment.device_id,
      "heartbeat",
      "heartbeat.task",
      "收到调用心跳并续期租约",
      {
        connection_id: attachment.connection_id,
        direction: "inbound",
        task_id: row.id,
        method: row.method,
        metadata: { lease_expires_at: now + LEASE_MILLISECONDS },
        created_at: now,
      },
    );
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
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.duplicate",
        "收到重复的成功响应并再次确认",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: existing.id,
          method: existing.method,
        },
      );
      return;
    }
    const row = this.owned_task(attachment.device_id, message);
    if (row === null) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: message.task_id }));
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.rejected",
        "调用响应与当前租约不匹配，已拒绝",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: message.task_id,
        },
      );
      return;
    }
    const result_json = JSON.stringify(message.result ?? null);
    if (result_json.length > MAX_BODY_BYTES) {
      socket.send(JSON.stringify({ type: "task.rejected", task_id: row.id, error: "result is too large" }));
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.rejected",
        "设备响应过大，已拒绝",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "error",
          task_id: row.id,
          method: row.method,
          metadata: { result_length: result_json.length, reason: "result is too large" },
        },
      );
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
    this.record_device_log(
      attachment.device_id,
      "response",
      "response.completed",
      "设备已返回成功响应",
      {
        connection_id: attachment.connection_id,
        direction: "inbound",
        task_id: completed.id,
        method: completed.method,
        metadata: {
          result: message.result ?? null,
          attempt_count: completed.attempt_count,
          elapsed_milliseconds: now - completed.created_at,
        },
        created_at: now,
      },
    );
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
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.rejected",
        "调用失败响应与当前租约不匹配，已拒绝",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: message.task_id,
        },
      );
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
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.failed_retryable",
        "设备返回可重试失败，调用已重新排队",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "warn",
          task_id: row.id,
          method: row.method,
          metadata: {
            error: message.error ?? "task failed",
            retryable: true,
            attempt_count: row.attempt_count,
          },
          created_at: now,
        },
      );
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
      this.record_device_log(
        attachment.device_id,
        "response",
        "response.failed",
        "设备返回失败响应，调用已终止",
        {
          connection_id: attachment.connection_id,
          direction: "inbound",
          level: "error",
          task_id: failed.id,
          method: failed.method,
          metadata: {
            error: failed.error_message,
            retryable: message.retryable === true,
            attempt_count: failed.attempt_count,
          },
          created_at: now,
        },
      );
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
        this.record_device_log(
          attachment.device_id,
          "call",
          "call.assigned",
          "调用已下发到设备",
          {
            connection_id: attachment.connection_id,
            direction: "outbound",
            task_id: assigned.id,
            method: assigned.method,
            metadata: {
              publisher_device_id: assigned.publisher_device_id,
              target_device_id: assigned.target_device_id,
              args: JSON.parse(assigned.args_json),
              attempt_count: assigned.attempt_count,
              lease_expires_at: assigned.lease_expires_at,
            },
            created_at: now,
          },
        );
        busy_devices.add(attachment.device_id);
      } catch (error) {
        this.ctx.storage.sql.exec(
          `UPDATE calls SET status = 'queued', assigned_device_id = NULL, lease_token = NULL,
           lease_expires_at = NULL, updated_at = ? WHERE id = ?`,
          Date.now(),
          row.id,
        );
        this.record_device_log(
          attachment.device_id,
          "call",
          "call.delivery_failed",
          "调用下发失败，任务已重新排队",
          {
            connection_id: attachment.connection_id,
            direction: "outbound",
            level: "error",
            task_id: row.id,
            method: row.method,
            metadata: {
              error: error instanceof Error ? error.message : String(error),
            },
          },
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
      const attachment = socket_attachment(socket);
      try {
        socket.send(message);
        this.record_device_log(
          device_id,
          "response",
          "response.notification_sent",
          "调用状态通知已发送到发布设备",
          {
            connection_id: attachment?.connection_id,
            direction: "outbound",
            task_id: value?.task?.id,
            method: value?.task?.method,
            metadata: { notification: value },
          },
        );
      } catch (error) {
        // Persisted task state remains available for polling after reconnect.
        this.record_device_log(
          device_id,
          "response",
          "response.notification_failed",
          "调用状态通知发送失败，发布设备可通过查询恢复",
          {
            connection_id: attachment?.connection_id,
            direction: "outbound",
            level: "error",
            task_id: value?.task?.id,
            method: value?.task?.method,
            metadata: {
              error: error instanceof Error ? error.message : String(error),
              notification_type: value?.type,
            },
          },
        );
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
      if (url.pathname === "/admin/api/credit-transactions" && request.method === "GET") {
        return admin_credit_transactions(env, request);
      }
      const admin_access_token_credits_match = url.pathname.match(
        /^\/admin\/api\/access-tokens\/([A-Za-z0-9-]+)\/credits$/,
      );
      if (admin_access_token_credits_match !== null && request.method === "POST") {
        return admin_access_token_request(
          env,
          request,
          "/" + admin_access_token_credits_match[1] + "/credits",
        );
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
      const admin_device_reset_match = url.pathname.match(
        /^\/admin\/api\/devices\/([^/]+)\/reset$/,
      );
      const reset_device_id = admin_device_reset_match
        ? decode_identifier(admin_device_reset_match[1])
        : null;
      if (reset_device_id !== null && request.method === "POST") {
        return admin_reset_device(env, request, reset_device_id);
      }
      const admin_device_logs_match = url.pathname.match(
        /^\/admin\/api\/devices\/([^/]+)\/logs$/,
      );
      const logs_device_id = admin_device_logs_match
        ? decode_identifier(admin_device_logs_match[1])
        : null;
      if (logs_device_id !== null && request.method === "GET") {
        return admin_device_logs(env, request, logs_device_id);
      }
      return error_response("not found", 404);
    }

    const legacy_match = url.pathname.match(/^\/v1\/bridges\/[^/]+(\/.*)?$/);
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
    const object = env.BRIDGES.getByName(BRIDGE_OBJECT_NAME);
    const token = bearer_token(request);
    const device_id = (
      request.headers.get("X-Bridge-Device-ID") ??
      request.headers.get("X-Bridge-Client-ID") ??
      ""
    ).trim();
    const device_authorized =
      Boolean(env.BRIDGE_TOKEN) &&
      safe_equal(token, env.BRIDGE_TOKEN) &&
      valid_identifier(device_id);
    if (forwarded_path === "/connect" && !device_authorized) {
      if (valid_identifier(device_id)) {
        await object.fetch(
          "https://internal/_internal/devices/" + device_id + "/connection-rejected",
          { method: "POST" },
        );
      }
      return error_response("unauthorized", 401);
    }
    if (device_authorized) {
      const headers = new Headers(request.headers);
      headers.delete("X-Bridge-Publisher-ID");
      headers.delete("X-Bridge-Authenticated-Publisher-ID");
      headers.delete("X-Bridge-Access-Token-ID");
      headers.set("X-Bridge-Authenticated-Publisher-ID", device_id);
      return object.fetch(new Request(forwarded_url, new Request(request, { headers })));
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
    headers.delete("X-Bridge-Publisher-ID");
    headers.delete("X-Bridge-Authenticated-Publisher-ID");
    headers.delete("X-Bridge-Device-ID");
    headers.delete("X-Bridge-Client-ID");
    headers.delete("X-Bridge-Access-Token-ID");
    headers.set("X-Bridge-Authenticated-Publisher-ID", publisher_id);
    headers.set("X-Bridge-Access-Token-ID", String(authorization.access_token?.id ?? ""));
    return object.fetch(new Request(forwarded_url, new Request(request, { headers })));
  },
};

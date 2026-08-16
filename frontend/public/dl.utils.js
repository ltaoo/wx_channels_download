(function (global) {
  "use strict";

  if (global.DLUtils) {
    return;
  }

  const LOG_LEVEL_VALUES = { debug: 0, info: 1, warn: 2, error: 3 };
  const LOG_CONFIG = {
    bufferCapacity: 256,
    batchSize: 16,
    flushIntervalMs: 2000,
    minLevel: "debug",
    maxRetries: 3,
    baseRetryMs: 1000,
    maxRetryMs: 30000,
  };

  function apiOrigin() {
    const config = global.__d_config || {};
    if (config.remoteServerEnabled) {
      return "https://localhost.weixin.qq.com";
    }
    const configured = String(config.apiOrigin || "").replace(/\/$/, "");
    if (configured) {
      return configured;
    }
    if (config.assets_base_url) {
      try {
        return new URL(config.assets_base_url, global.location.href).origin;
      } catch {
        // Fall through to the current origin when the configured URL is invalid.
      }
    }
    return global.location.origin;
  }

  function sendReport(entry, immediate) {
    const url = apiOrigin() + "/report";
    const body = JSON.stringify(entry);
    if (
      immediate &&
      global.navigator &&
      typeof global.navigator.sendBeacon === "function"
    ) {
      const blob = new Blob([body], { type: "application/json" });
      if (global.navigator.sendBeacon(url, blob)) {
        return Promise.resolve({ error: null });
      }
    }
    return global
      .fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        keepalive: !!immediate,
      })
      .then((response) => {
        if (!response.ok) {
          return {
            error: new Error(`日志上报失败：HTTP ${response.status}`),
          };
        }
        return { error: null };
      })
      .catch((error) => ({ error }));
  }

  class CircularBuffer {
    constructor(capacity) {
      this.buffer = new Array(capacity);
      this.capacity = capacity;
      this.head = 0;
      this.tail = 0;
      this.size = 0;
    }

    push(item) {
      this.buffer[this.head] = item;
      this.head = (this.head + 1) % this.capacity;
      if (this.size < this.capacity) {
        this.size += 1;
      } else {
        this.tail = (this.tail + 1) % this.capacity;
      }
    }

    shift() {
      if (this.size === 0) {
        return null;
      }
      const item = this.buffer[this.tail];
      this.buffer[this.tail] = undefined;
      this.tail = (this.tail + 1) % this.capacity;
      this.size -= 1;
      return item;
    }
  }

  class LogTransport {
    constructor(config) {
      this.config = config;
      this.buffer = new CircularBuffer(config.bufferCapacity);
      this.timer = null;
      this.flushingGeneration = null;
      this.generation = 0;
      this.retryTimers = new Map();
    }

    enqueue(entry) {
      if (
        LOG_LEVEL_VALUES[entry.level] <
        LOG_LEVEL_VALUES[this.config.minLevel]
      ) {
        return;
      }
      entry._retries = 0;
      this.buffer.push(entry);
      this.scheduleFlush();
    }

    scheduleFlush() {
      if (this.timer !== null) {
        return;
      }
      if (this.buffer.size >= this.config.batchSize) {
        this.flush();
        return;
      }
      this.timer = global.setTimeout(() => {
        this.timer = null;
        this.flush();
      }, this.config.flushIntervalMs);
    }

    flush() {
      if (this.flushingGeneration !== null) {
        return;
      }
      const generation = this.generation;
      this.flushingGeneration = generation;
      let sent = 0;
      const finish = () => {
        if (this.flushingGeneration !== generation) {
          return;
        }
        this.flushingGeneration = null;
        if (this.buffer.size > 0) {
          this.scheduleFlush();
        }
      };
      const sendNext = () => {
        if (generation !== this.generation || sent >= this.config.batchSize) {
          finish();
          return;
        }
        const entry = this.buffer.shift();
        if (!entry) {
          finish();
          return;
        }
        sent += 1;
        this.send(entry, generation).then(sendNext);
      };
      sendNext();
    }

    send(entry, generation) {
      const retries = entry._retries;
      delete entry._retries;
      return sendReport(entry, false).then((result) => {
        if (result && result.error) {
          this.maybeRetry(entry, retries, generation);
        }
      });
    }

    maybeRetry(entry, retries, generation) {
      if (
        generation !== this.generation ||
        retries >= this.config.maxRetries
      ) {
        return;
      }
      entry._retries = retries + 1;
      const delay = Math.min(
        this.config.baseRetryMs * Math.pow(2, retries),
        this.config.maxRetryMs,
      );
      const retryTimer = global.setTimeout(() => {
        this.retryTimers.delete(retryTimer);
        if (generation !== this.generation) {
          return;
        }
        this.buffer.push(entry);
        this.scheduleFlush();
      }, delay);
      this.retryTimers.set(retryTimer, entry);
    }

    flushNow() {
      this.generation += 1;
      this.flushingGeneration = null;
      if (this.timer !== null) {
        global.clearTimeout(this.timer);
        this.timer = null;
      }
      const pending = [];
      this.retryTimers.forEach((entry, timer) => {
        global.clearTimeout(timer);
        pending.push(entry);
      });
      this.retryTimers.clear();
      let entry;
      while ((entry = this.buffer.shift())) {
        pending.push(entry);
      }
      return Promise.all(
        pending.map((current) => {
          delete current._retries;
          return sendReport(current, true);
        }),
      );
    }
  }

  const logTransport = new LogTransport(LOG_CONFIG);

  class LogBuilder {
    constructor(level, transport) {
      this.fields = {};
      this.level = level;
      this.transport = transport;
    }

    normalize(value) {
      if (value === null || typeof value === "undefined") {
        return value;
      }
      if (typeof value === "bigint") {
        return value.toString();
      }
      if (typeof value === "symbol" || typeof value === "function") {
        return String(value);
      }
      if (value instanceof Date) {
        return value.toISOString();
      }
      if (value instanceof Error) {
        return {
          name: value.name,
          message: value.message,
          stack: value.stack,
        };
      }
      return value;
    }

    setField(key, value) {
      this.fields[key] = this.normalize(value);
      return this;
    }

    jsonField(key, value) {
      if (typeof value === "string") {
        try {
          value = JSON.parse(value);
        } catch {
          // Preserve non-JSON strings.
        }
      }
      return this.setField(key, value);
    }

    Str(key, value) {
      return this.setField(key, value);
    }

    Err(error) {
      return this.setField("error", error);
    }

    Object(key, value) {
      return this.jsonField(key, value);
    }

    Obj(key, value) {
      return this.Object(key, value);
    }

    Dict(key, value) {
      return this.Object(key, value);
    }

    Interface(key, value) {
      return this.setField(key, value);
    }

    JSON(key, value) {
      return this.jsonField(key, value);
    }

    Int(key, value) {
      return this.setField(key, value);
    }

    RawJSON(key, value) {
      return this.jsonField(key, value);
    }

    Bool(key, value) {
      return this.setField(key, value);
    }

    Float(key, value) {
      return this.setField(key, value);
    }

    Msg(message) {
      const payload = {
        ...this.fields,
        message: this.normalize(message),
        level: this.level,
      };
      console.log("[log]", payload);
      this.transport.enqueue(payload);
    }
  }

  function createLogBuilder(level, error) {
    const builder = new LogBuilder(level, logTransport);
    return typeof error === "undefined" ? builder : builder.Err(error);
  }

  function log(params) {
    const payload = Object.assign({ level: "info" }, params || {});
    console.log("[log]", payload);
    logTransport.enqueue(payload);
  }
  log.Info = () => createLogBuilder("info");
  log.Warn = () => createLogBuilder("warn");
  log.Error = (error) => createLogBuilder("error", error);
  log.Debug = () => createLogBuilder("debug");
  log.flushNow = () => logTransport.flushNow();

  function feedbackText(value, fallback) {
    if (value instanceof Error) {
      return value.message || fallback;
    }
    if (value && typeof value === "object") {
      value = value.msg ?? value.message ?? value.text;
    }
    return value == null || value === "" ? fallback : String(value);
  }

  function ensureFeedbackStyle() {
    if (document.getElementById("dl-utils-feedback-style")) {
      return;
    }
    const style = document.createElement("style");
    style.id = "dl-utils-feedback-style";
    style.textContent = `
.dl-utils-tip {
  position: fixed; top: 8px; right: 8px; left: 8px; z-index: 2147483647;
  box-sizing: border-box; padding: 10px; border-radius: 8px;
  color: #fff; font-size: 14px; text-align: center;
  overflow-wrap: break-word; word-break: break-all;
}
.dl-utils-tip--error { background: var(--weui-RED, #fa5151); }
.dl-utils-tip--warning { background: var(--weui-ORANGE, #fa9d3b); color: #191919; }
.dl-utils-toast-layer .dl-utils-toast-mask {
  position: fixed; inset: 0; z-index: 2147483646;
}
.dl-utils-toast-layer .dl-utils-toast-wrap {
  position: fixed; inset: 0; z-index: 2147483647;
  display: flex; align-items: center; justify-content: center;
}
.dl-utils-toast {
  max-width: 320px; margin-top: -10%; padding: 12px 20px;
  box-sizing: border-box; border-radius: 8px;
  background: var(--weui-BG-4, #4c4c4c); color: rgba(255, 255, 255, .9);
  filter: drop-shadow(0 8px 25px rgba(0, 0, 0, .1));
  font-size: 14px; line-height: 1.4; text-align: center;
  overflow-wrap: break-word;
}`;
    (document.head || document.documentElement).appendChild(style);
  }

  function topTip(text, type) {
    ensureFeedbackStyle();
    const tip = document.createElement("div");
    tip.className = `dl-utils-tip dl-utils-tip--${type}`;
    tip.setAttribute("role", "alert");
    tip.setAttribute("aria-live", "assertive");
    tip.textContent = text;
    (document.body || document.documentElement).appendChild(tip);
    const timer = global.setTimeout(() => tip.remove(), 3000);
    return {
      hide() {
        global.clearTimeout(timer);
        tip.remove();
      },
    };
  }

  function notify(params, type) {
    const options =
      params && typeof params === "object" && !(params instanceof Error)
        ? params
        : { msg: params };
    const fallback = type === "warning" ? "警告" : "未知错误";
    const message = feedbackText(options, fallback);
    const builder = type === "warning" ? log.Warn() : log.Error();
    if (options.source) {
      builder.Str("file", options.source);
    }
    builder.Msg(message);
    if (options.alert == null || options.alert) {
      return topTip(message, type);
    }
  }

  function toast(value) {
    ensureFeedbackStyle();
    const root = document.createElement("div");
    root.className = "dl-utils-toast-layer";
    root.setAttribute("role", "status");
    root.setAttribute("aria-live", "polite");
    const mask = document.createElement("div");
    mask.className = "dl-utils-toast-mask";
    const wrapper = document.createElement("div");
    wrapper.className = "dl-utils-toast-wrap";
    const content = document.createElement("div");
    content.className = "dl-utils-toast";
    content.textContent = feedbackText(value, "");
    wrapper.appendChild(content);
    root.appendChild(mask);
    root.appendChild(wrapper);
    (document.body || document.documentElement).appendChild(root);
    const timer = global.setTimeout(() => root.remove(), 2200);
    return {
      hide() {
        global.clearTimeout(timer);
        root.remove();
      },
    };
  }

  function parseJSON(value) {
    try {
      return [null, JSON.parse(value)];
    } catch (error) {
      return [error, null];
    }
  }

  global.addEventListener("pagehide", () => logTransport.flushNow());
  global.addEventListener("beforeunload", () => logTransport.flushNow());

  global.DLUtils = Object.freeze({
    error(params) {
      return notify(params, "error");
    },
    warning(params) {
      return notify(params, "warning");
    },
    toast,
    log,
    parseJSON,
  });
})(window);

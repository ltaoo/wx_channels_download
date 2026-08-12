/**
 * @file All utility functions + API + event bus
 */
if (typeof window.Timeless !== "undefined") {
  var timeless = window.Timeless;
  // Rendering
  window.h = timeless.h;
  window.View = timeless.View;
  window.Button = timeless.Button;
  window.Fragment = timeless.Fragment;
  window.Input = timeless.Input;
  window.Img = timeless.Img;
  // Control flow
  window.Show = timeless.Show;
  window.For = timeless.For;
  window.Switch = timeless.Switch;
  window.Match = timeless.Match;
  // Reactivity
  window.ref = timeless.ref;
  window.refobj = timeless.refobj;
  window.refarr = timeless.refarr;
  window.computed = timeless.computed;
  window.combine = timeless.combine;
  window.isElement = timeless.isElement;
  // Styling
  window.cn = timeless.cn;
  window.classNames = timeless.classNames;
  // SVG helpers
  window.SVG = timeless.SVG;
  window.Circle = timeless.Circle;
}
if (typeof WXE === "undefined") {
  window.WXE = {};
}
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before utils.js");
}
var WXU = (() => {
  var APIOrigin = WXEnv.get("apiOrigin");
  const http_client = new Timeless.kit.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: APIOrigin,
  });
  Timeless.web.provide_http_client(http_client);
  const request = Timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(r) {
      if (r.error) {
        return Timeless.Result.Err(r.error);
      }
      const { code, msg, data } = r.data;
      if (code !== 0) {
        return Timeless.Result.Err(msg, code, data);
      }
      return Timeless.Result.Ok(data);
    },
  });
  const reqs = {
    report: new Timeless.kit.RequestCore(
      function report(params) {
        return request.post("/report", { ...params, level: "info" });
      },
      { client: http_client },
    ),
  };
  // ── log transport config ────────────────────────────────────────
  var LOG_LEVEL_VALUES = { debug: 0, info: 1, warn: 2, error: 3 };
  var LOG_CFG = {
    bufferCapacity: 256,
    batchSize: 16,
    flushIntervalMs: 2000,
    minLevel: "debug",
    maxRetries: 3,
    baseRetryMs: 1000,
    maxRetryMs: 30000,
  };

  // ── CircularBuffer (lossy-safe ring buffer) ─────────────────────
  function CircularBuffer(capacity) {
    this.buf = new Array(capacity);
    this.capacity = capacity;
    this.head = 0;
    this.tail = 0;
    this.size = 0;
  }
  CircularBuffer.prototype.push = function (item) {
    this.buf[this.head] = item;
    this.head = (this.head + 1) % this.capacity;
    if (this.size < this.capacity) {
      this.size++;
    } else {
      this.tail = (this.tail + 1) % this.capacity;
    }
  };
  CircularBuffer.prototype.shift = function () {
    if (this.size === 0) return null;
    var item = this.buf[this.tail];
    this.buf[this.tail] = undefined;
    this.tail = (this.tail + 1) % this.capacity;
    this.size--;
    return item;
  };

  // ── LogTransport (queue → batch → send with retry) ──────────────
  function LogTransport(reportFn, cfg) {
    this.reportFn = reportFn;
    this.cfg = cfg;
    this.buf = new CircularBuffer(cfg.bufferCapacity);
    this._timer = null;
    this._flushing = false;
  }
  LogTransport.prototype.enqueue = function (entry) {
    if (LOG_LEVEL_VALUES[entry.level] < LOG_LEVEL_VALUES[this.cfg.minLevel]) {
      return;
    }
    entry._retries = 0;
    this.buf.push(entry);
    this._scheduleFlush();
  };
  LogTransport.prototype._scheduleFlush = function () {
    if (this._timer !== null) return;
    if (this.buf.size >= this.cfg.batchSize) {
      this._flush();
      return;
    }
    var self = this;
    this._timer = setTimeout(function () {
      self._timer = null;
      self._flush();
    }, this.cfg.flushIntervalMs);
  };
  LogTransport.prototype._flush = function () {
    var self = this;
    if (self._flushing) return;
    self._flushing = true;
    var batch = [];
    var count = 0;
    while (count < self.cfg.batchSize) {
      var e = self.buf.shift();
      if (!e) break;
      batch.push(e);
      count++;
    }
    function sendOne(idx) {
      if (idx >= batch.length) {
        self._flushing = false;
        if (self.buf.size > 0) self._scheduleFlush();
        return;
      }
      self._send(batch[idx]).then(function () {
        sendOne(idx + 1);
      });
    }
    sendOne(0);
  };
  LogTransport.prototype._send = function (entry) {
    var _retries = entry._retries;
    delete entry._retries;
    try {
      var result = this.reportFn(entry);
      if (result && typeof result.then === "function") {
        var self = this;
        return result.then(function (r) {
          if (r && r.error) self._maybeRetry(entry, _retries);
        });
      }
    } catch (e) {
      this._maybeRetry(entry, _retries);
    }
    return Promise.resolve();
  };
  LogTransport.prototype._maybeRetry = function (entry, retries) {
    if (retries >= this.cfg.maxRetries) return;
    entry._retries = retries + 1;
    var delay = Math.min(
      this.cfg.baseRetryMs * Math.pow(2, retries),
      this.cfg.maxRetryMs,
    );
    var self = this;
    setTimeout(function () {
      self.buf.push(entry);
      self._scheduleFlush();
    }, delay);
  };
  LogTransport.prototype._flushSync = function () {
    var e;
    while ((e = this.buf.shift())) {
      delete e._retries;
      try {
        var blob = new Blob([JSON.stringify(e)], {
          type: "application/json",
        });
        navigator.sendBeacon(APIOrigin + "/report", blob);
      } catch (ignore) {}
    }
  };

  var defaultRandomAlphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  function __wx_uid__() {
    return random_string(12);
  }
  /**
   * Returns a random string of specified length
   * @param length
   * @returns
   */
  function random_string(length) {
    return random_string_with_alphabet(length, defaultRandomAlphabet);
  }
  function random_string_with_alphabet(length, alphabet) {
    let b = new Array(length);
    let max = alphabet.length;
    for (let i = 0; i < b.length; i++) {
      let n = Math.floor(Math.random() * max);
      b[i] = alphabet[n];
    }
    return b.join("");
  }
  function sleep() {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve();
      }, 1000);
    });
  }
  function __wx_ensure_feedback_style() {
    if (document.getElementById("wx-feedback-style")) {
      return;
    }
    const style = document.createElement("style");
    style.id = "wx-feedback-style";
    style.textContent = `
.wx-feedback-toptips.weui-toptips {
  display: block;
  position: fixed;
  top: 8px;
  right: 8px;
  left: 8px;
  z-index: 2147483647;
  box-sizing: border-box;
  padding: 10px;
  border-radius: 8px;
  background-color: var(--weui-RED, #fa5151);
  color: #fff;
  font-size: 14px;
  text-align: center;
  overflow-wrap: break-word;
  word-break: break-all;
  transform: translateZ(0);
}
.wx-feedback-layer .weui-mask_transparent {
  position: fixed;
  inset: 0;
  z-index: 2147483646;
}
.wx-feedback-layer .weui-toast__wrp {
  position: fixed;
  inset: 0;
  z-index: 2147483647;
  display: flex;
  align-items: center;
  justify-content: center;
}
.wx-feedback-layer .weui-toast {
  position: static;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-width: 132px;
  max-width: 320px;
  margin-top: -10%;
  padding: 28px 20px;
  box-sizing: border-box;
  border-radius: 8px;
  background-color: var(--weui-BG-4, #4c4c4c);
  color: rgba(255, 255, 255, 0.9);
  filter: drop-shadow(0 8px 25px rgba(0, 0, 0, 0.1));
  line-height: 1.4;
  text-align: center;
  transform: none;
}
.wx-feedback-layer .weui-toast_text {
  min-width: 0;
  min-height: 0;
  padding: 12px 20px;
}
.wx-feedback-layer .weui-toast__content {
  max-width: 100%;
  font-size: 14px;
  overflow-wrap: break-word;
  hyphens: auto;
}
.wx-feedback-layer .weui-primary-loading.weui-icon_toast {
  position: relative;
  display: inline-flex;
  width: 1em;
  height: 1em;
  margin-bottom: 16px;
  color: #ededed;
  font-size: 40px;
  animation: circleLoading 1s steps(60, end) infinite;
  flex-shrink: 0;
}
.wx-feedback-layer .weui-primary-loading::before,
.wx-feedback-layer .weui-primary-loading::after {
  content: "";
  display: block;
  width: 0.5em;
  height: 1em;
  box-sizing: border-box;
  border-style: solid;
  border-color: currentColor;
}
.wx-feedback-layer .weui-primary-loading::before {
  border-width: 4px 0 4px 4px;
  border-radius: 1em 0 0 1em;
  -webkit-mask-image: linear-gradient(#000 8%, rgba(0, 0, 0, 0.3) 95%);
}
.wx-feedback-layer .weui-primary-loading::after {
  border-width: 4px 4px 4px 0;
  border-radius: 0 1em 1em 0;
  -webkit-mask-image: linear-gradient(transparent 8%, rgba(0, 0, 0, 0.3) 95%);
}
.wx-feedback-layer .weui-primary-loading__dot {
  position: absolute;
  top: 0;
  left: 50%;
  width: 4px;
  height: 4px;
  margin-left: -2px;
  border-radius: 0 4px 4px 0;
  background: currentColor;
}
@keyframes circleLoading {
  from { transform: rotate(0); }
  to { transform: rotate(360deg); }
}`;
    (document.head || document.documentElement).appendChild(style);
  }
  function __wx_feedback_text(value, fallback = "") {
    if (value instanceof Error) {
      return value.message || fallback;
    }
    if (value && typeof value === "object") {
      value = value.msg ?? value.message ?? value.text;
    }
    return value == null || value === "" ? fallback : String(value);
  }
  function __wx_top_tip(text) {
    __wx_ensure_feedback_style();
    const tip = document.createElement("div");
    tip.className = "weui-toptips weui-toptips_warn wx-feedback-toptips";
    tip.setAttribute("role", "alert");
    tip.setAttribute("aria-live", "assertive");
    tip.textContent = __wx_feedback_text(text);
    document.body.appendChild(tip);
    setTimeout(() => {
      tip.remove();
    }, 3000);
    return {
      hide() {
        tip.remove();
      },
    };
  }
  function __wx_create_toast(text, loading) {
    __wx_ensure_feedback_style();
    const root = document.createElement("div");
    root.className = "wx-feedback-layer";
    root.setAttribute("role", "alert");
    root.setAttribute("aria-live", "assertive");
    const mask = document.createElement("div");
    mask.className = "weui-mask_transparent";
    const wrapper = document.createElement("div");
    wrapper.className = "weui-toast__wrp";
    const toast = document.createElement("div");
    toast.className = loading ? "weui-toast" : "weui-toast weui-toast_text";
    if (loading) {
      const spinner = document.createElement("span");
      spinner.className = "weui-primary-loading weui-icon_toast";
      spinner.setAttribute("role", "img");
      spinner.setAttribute("aria-label", "正在加载");
      const dot = document.createElement("span");
      dot.className = "weui-primary-loading__dot";
      spinner.appendChild(dot);
      toast.appendChild(spinner);
    }
    const content = document.createElement("p");
    content.className = "weui-toast__content";
    content.textContent = text;
    toast.appendChild(content);
    wrapper.appendChild(toast);
    root.appendChild(mask);
    root.appendChild(wrapper);
    document.body.appendChild(root);
    return root;
  }
  function __wx_toast(text) {
    const root = __wx_create_toast(__wx_feedback_text(text), false);
    setTimeout(() => {
      root.remove();
    }, 2200);
    return {
      hide() {
        root.remove();
      },
    };
  }
  function __wx_loading(options = "加载中") {
    const text = __wx_feedback_text(options, "加载中");
    const root = __wx_create_toast(text, true);
    return {
      hide() {
        root.remove();
      },
    };
  }
  /**
   * @param {string} text
   */
  function __wx_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }
  // ── fluent logger (zerolog-style) ──────────────────────────────────
  /**
   * LogBuilder chained log builder, consistent with zerolog API.
   *   Usage: WXU.log.Info().Str("key", val).Int("n", 1).Msg("description")
   */
  // ── shared log transport instance ──────────────────────────────
  var logTransport = new LogTransport(function (entry) {
    return reqs.report.run(entry);
  }, LOG_CFG);

  class LogBuilder {
    _setField(key, val) {
      this._fields[key] = val;
      return this;
    }
    _normalizeValue(val) {
      if (val === null || typeof val === "undefined") {
        return val;
      }
      if (typeof val === "bigint") {
        return val.toString();
      }
      if (typeof val === "symbol" || typeof val === "function") {
        return String(val);
      }
      if (val instanceof Date) {
        return val.toISOString();
      }
      if (val instanceof Error) {
        return {
          name: val.name,
          message: val.message,
          stack: val.stack,
        };
      }
      return val;
    }
    _coerceJSONValue(key, val) {
      if (typeof val === "string") {
        try {
          val = JSON.parse(val);
        } catch (_ignore) {}
      }
      return this._setField(key, this._normalizeValue(val));
    }
    constructor(level, transport) {
      this._fields = {};
      this._level = level;
      this._transport = transport;
    }
    Str(key, val) {
      return this._setField(key, this._normalizeValue(val));
    }
    Err(err) {
      return this._setField("error", this._normalizeValue(err));
    }
    Object(key, val) {
      return this._coerceJSONValue(key, val);
    }
    Obj(key, val) {
      return this.Object(key, val);
    }
    Dict(key, val) {
      return this.Object(key, val);
    }
    Interface(key, val) {
      return this._setField(key, this._normalizeValue(val));
    }
    JSON(key, val) {
      return this._coerceJSONValue(key, val);
    }
    Int(key, val) {
      return this._setField(key, val);
    }
    RawJSON(key, val) {
      return this._coerceJSONValue(key, val);
    }
    Bool(key, val) {
      return this._setField(key, val);
    }
    Float(key, val) {
      return this._setField(key, val);
    }
    Msg(msg) {
      const message = this._normalizeValue(msg);
      const payload = { ...this._fields, message, level: this._level };
      console.log("[log]", payload);
      this._transport.enqueue(payload);
    }
  }

  // WXU.log is both a function (backward compatible) and a namespace (fluent style)
  /** @param {LogMsg} params - Legacy call style, kept for backward compatibility */
  function Logger(params) {
    console.log("[log]", params);
    logTransport.enqueue(Object.assign({ level: "info" }, params));
  }
  Logger.Info = function () {
    return new LogBuilder("info", logTransport);
  };
  Logger.Warn = function () {
    return new LogBuilder("warn", logTransport);
  };
  Logger.Error = function () {
    return new LogBuilder("error", logTransport);
  };
  Logger.Debug = function () {
    return new LogBuilder("debug", logTransport);
  };
  /**
   * @param {ErrorMsg} params
   */
  function __wx_error(params) {
    const options =
      params && typeof params === "object" && !(params instanceof Error)
        ? params
        : { msg: params };
    const message = __wx_feedback_text(options, "未知错误");
    var _alert = options.alert != null ? options.alert : 1;
    const logger = Logger.Error();
    console.log("__wx_error - source", options.source);
    if (options.source) {
      logger.Str("file", options.source);
    }
    logger.Msg(message);
    if (_alert) {
      return __wx_top_tip(message);
    }
  }
  const script_loaded_map = {};
  function load_script(src) {
    const existing = script_loaded_map[src];
    if (existing) {
      return existing;
    }
    const p = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.type = "text/javascript";
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
    script_loaded_map[src] = p;
    return p;
  }
  async function load_audio_converter() {
    await load_script(WXEnv.assetUrl("/inject/audio-converter.js"));
    return WXAudio;
  }

  function remove_zero(num) {
    let result = Number(num);
    if (String(num).indexOf(".") > -1) {
      result = parseFloat(num.toString().replace(/0+?$/g, ""));
    }
    return result;
  }
  function bytes_to_size(bytes) {
    if (!bytes) {
      return "0KB";
    }
    if (bytes === 0) {
      return "0KB";
    }
    const symbols = ["bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
    let exp = Math.floor(Math.log(bytes) / Math.log(1024));
    if (exp < 1) {
      return bytes + " " + symbols[0];
    }
    bytes = Number((bytes / Math.pow(1024, exp)).toFixed(2));
    const size = bytes;
    const unit = symbols[exp];
    if (Number.isInteger(size)) {
      return `${size}${unit}`;
    }
    return `${remove_zero(size.toFixed(2))}${unit}`;
  }
  function get_queries(href) {
    var [pathname, search] = decodeURIComponent(href).split("?");
    var queries = decodeURIComponent(search)
      .split("&")
      .map((item) => {
        const [key, value] = item.split("=");
        return {
          [key]: value,
        };
      })
      .reduce(
        (prev, cur) => ({
          ...prev,
          ...cur,
        }),
        {},
      );
    return queries;
  }

  /**
   * Download with progress callbacks
   * @param {Response} response
   * @param {{ onStart: (v: { total_size: number }) => void, onProgress: (v: { loaded_size: number, progress: number | null }) => void, onEnd: (v: { blob: Blob }) => void }} handlers
   */
  async function download_with_progress(response, handlers) {
    var content_length = response.headers.get("Content-Length");
    var chunks = [];
    var total_size = content_length ? parseInt(content_length, 10) : 0;
    if (total_size) {
      if (handlers.onStart) {
        handlers.onStart({ total_size });
      }
    }
    var loaded_size = 0;
    var reader = response.body.getReader();
    while (true) {
      var { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
      loaded_size += value.length;
      if (handlers.onProgress) {
        handlers.onProgress({
          loaded_size,
          progress: total_size
            ? Number(((loaded_size / total_size) * 100).toFixed(2))
            : null,
        });
      }
    }
    var blob = new Blob(chunks);
    if (handlers.onEnd) {
      handlers.onEnd({ blob });
    }
    return blob;
  }

  /**
   * @param {RequestInfo | URL} url
   * @returns {Promise<[null, Response] | [Error, null]>}
   */
  async function __wx_fetch(url) {
    try {
      const r = await fetch(url);
      return [null, r];
    } catch (err) {
      return [/** @type {Error} */ (err), null];
    }
  }

  // ── page unload: best-effort drain pending logs via sendBeacon ──
  window.addEventListener("pagehide", function () {
    logTransport._flushSync();
  });
  window.addEventListener("beforeunload", function () {
    logTransport._flushSync();
  });

  return {
    ...WXE,
    downloader: {
      show() {
        console.warn("show - downloader not ready");
        return [new Error("downloader not ready"), null];
      },
      hide() {
        console.warn("hide - downloader not ready");
        return [new Error("downloader not ready"), null];
      },
      toggle() {},
      async create(feeds, opt) {
        console.warn("create - downloader not ready");
        return [new Error("downloader not ready"), null];
      },
      async browse(feeds, opt) {
        console.warn("browse - downloader not ready");
        return [new Error("downloader not ready"), null];
      },
    },
    /**
     * Type conversion utilities
     */
    async media_buffer_to_wav(...args) {
      const audio_converter = await load_audio_converter();
      return audio_converter.mediaBufferToWav(...args);
    },
    async media_to_mp3(buf) {
      const audio_converter = await load_audio_converter();
      return audio_converter.mediaToMp3(buf);
    },
    /**  */
    sleep,
    resultify(fn) {
      return (...args) => {
        return new Promise((resolve) => {
          fn(...args)
            .then((data) => {
              resolve([null, data]);
            })
            .catch((err) => {
              resolve([err, null]);
            });
        });
      };
    },
    uid: __wx_uid__,
    bytes_to_size,
    remove_zero,
    parseJSON(v) {
      try {
        var r = JSON.parse(v);
        return [null, r];
      } catch (err) {
        return [err, null];
      }
    },
    load_script,
    download_with_progress,
    /**
     * @param {() => HTMLElement} selector
     * @returns
     */
    find_elm(selector) {
      return new Promise((resolve) => {
        var __count = 0;
        var __timer = setInterval(() => {
          __count += 1;
          var $elm = selector();
          if (!$elm) {
            if (__count >= 5) {
              clearInterval(__timer);
              __timer = null;
              resolve(null);
            }
            return;
          }
          resolve($elm);
          return;
        }, 200);
      });
    },
    get_queries,
    /**
     * Notifications / tips
     */
    copy: __wx_copy,
    log: Logger,
    error: __wx_error,
    loading(options) {
      return __wx_loading(options);
    },
    toast(text) {
      return __wx_toast(text);
    },
    // menu_item: __wx_menu_item,
    // create_dropdown_menu: __wx_create_dropdown_menu,
    // create_popover: __wx_create_popover,
    fetch: __wx_fetch,
    observe_node({ selector, container, onOk, onError }) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        onOk($existing);
        return;
      }
      var observer = new MutationObserver((mutations, obs) => {
        mutations.forEach((mutation) => {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach((node) => {
              if (node.nodeType === 1) {
                if (node.matches(selector) || node.querySelector(selector)) {
                  clearTimeout(timer);
                  onOk(
                    node.matches(selector)
                      ? node
                      : node.querySelector(selector),
                  );
                  obs.disconnect();
                }
              }
            });
          }
        });
      });
      var timer = null;
      if (onError) {
        timer = setTimeout(() => {
          observer.disconnect();
          onError();
        }, 5000);
      }
      function startObserve() {
        var $root =
          typeof container === "string"
            ? document.querySelector(container)
            : container || null;
        if (!$root) {
          $root = document.body || document.documentElement;
        }
        observer.observe($root, {
          childList: true,
          subtree: true,
        });
      }
      startObserve();
    },
    /**
     * @param {{ url: string; method: 'GET' | 'POST'; body?: any; timeout?: number }} opt
     */
    async request(opt) {
      return new Promise((resolve) => {
        var xhr = new XMLHttpRequest();
        var method = String(opt.method || "GET").toUpperCase();
        var url = String(opt.url || "");
        var settled = false;
        function finish(err, data) {
          if (settled) return;
          settled = true;
          resolve([err, data]);
        }
        function requestError(reason, details = {}) {
          const err = new Error(`${method} ${url} 请求失败：${reason}`);
          Object.assign(err, { method, url }, details);
          return err;
        }
        function responseMessage(data) {
          if (!data || typeof data !== "object") return "";
          return (
            data.msg ||
            data.message ||
            (typeof data.error === "string" ? data.error : "")
          );
        }
        try {
          xhr.open(method, url);
          xhr.setRequestHeader("Content-Type", "application/json");
        } catch (err) {
          finish(
            requestError(err.message || String(err), {
              type: "setup",
              cause: err,
            }),
            null,
          );
          return;
        }
        const timeout = Number(opt.timeout);
        if (Number.isFinite(timeout) && timeout > 0) {
          xhr.timeout = timeout;
        }
        xhr.onload = function () {
          var response = null;
          try {
            response = JSON.parse(xhr.responseText);
          } catch (_ignore) {}
          if (xhr.status < 200 || xhr.status >= 300) {
            const status = xhr.status
              ? `HTTP ${xhr.status}${xhr.statusText ? ` ${xhr.statusText}` : ""}`
              : "无法连接服务器";
            const serverMessage = responseMessage(response);
            finish(
              requestError(
                serverMessage ? `${status}：${serverMessage}` : status,
                {
                  type: "http",
                  status: xhr.status,
                  statusText: xhr.statusText,
                  response: response || xhr.responseText,
                },
              ),
              null,
            );
            return;
          }
          if (response) {
            if (response.code !== 0) {
              const message =
                responseMessage(response) || `接口返回错误码 ${response.code}`;
              const err = requestError(message, {
                type: "api",
                code: response.code,
                data: response.data,
                response,
              });
              finish(err, null);
              return;
            }
            finish(null, response.data);
            return;
          }
          finish(null, xhr.responseText);
        };
        xhr.onerror = function () {
          finish(
            requestError(
              "无法连接服务器（服务未启动、连接被拒绝或 CORS 拦截）",
              {
                type: "network",
                status: xhr.status,
                statusText: xhr.statusText,
              },
            ),
            null,
          );
        };
        xhr.ontimeout = function () {
          finish(
            requestError(`请求超时（${xhr.timeout}ms）`, {
              type: "timeout",
              timeout: xhr.timeout,
            }),
            null,
          );
        };
        xhr.onabort = function () {
          finish(requestError("请求已取消", { type: "abort" }), null);
        };
        try {
          xhr.send(JSON.stringify(opt.body));
        } catch (err) {
          finish(
            requestError(err.message || String(err), {
              type: "send",
              cause: err,
            }),
            null,
          );
        }
      });
    },
    async save(blob, filename) {
      await load_script(WXEnv.assetUrl("/public/FileSaver.min.js"));
      saveAs(blob, filename);
    },
    async Zip() {
      await load_script(WXEnv.assetUrl("/public/jszip.min.js"));
      const zip = new JSZip();
      return zip;
    },
    /**
     * @returns {ChannelsConfig}
     */
    get config() {
      return WXEnv.config;
    },
    env: {
      get isChannels() {
        return WXEnv.isChannels;
      },
      get isWxwork() {
        return WXEnv.isWxwork;
      },
    },
  };
})();

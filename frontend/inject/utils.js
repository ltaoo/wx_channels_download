/**
 * @file All utility functions + API + event bus
 */
if (typeof window.Timeless !== "undefined") {
  var timeless = window.Timeless;
  Object.assign(timeless, timeless.weui.kit);
  timeless.ui = timeless.weui.ui;
  // Rendering
  window.h = timeless.h;
  window.View = timeless.View;
  window.Button = timeless.Button;
  window.Fragment = timeless.Fragment;
  window.Input = timeless.Input;
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

if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before utils.js");
}
var WXU = (() => {
  var APIHostname = WXEnv.apiOrigin;
  const http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: APIHostname,
  });
  Timeless.web.provide_http_client(http_client);
  const request = Timeless.request_factory({
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
    report: new Timeless.RequestCore(
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
    var apiOrigin = WXEnv.apiOrigin;
    var e;
    while ((e = this.buf.shift())) {
      delete e._retries;
      try {
        var blob = new Blob([JSON.stringify(e)], {
          type: "application/json",
        });
        navigator.sendBeacon(apiOrigin + "/report", blob);
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
  function __wx_top_tip(text) {
    const tip = document.createElement("div");
    tip.className = "wx-top-tip";
    tip.textContent = text || "";
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
  function __wx_toast(text) {
    const toast = document.createElement("div");
    toast.className = "wx-toast";
    toast.textContent = text || "";
    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2200);
    return {
      hide() {
        toast.remove();
      },
    };
  }
  function __wx_loading(text = "加载中") {
    const mask = document.createElement("div");
    mask.className = "wx-loading-mask";
    mask.innerHTML = `<div class="wx-loading-box"><span class="wx-loading-spinner"></span><span>${text}</span></div>`;
    document.body.appendChild(mask);
    return {
      hide() {
        mask.remove();
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
    constructor(level, transport) {
      this._fields = {};
      this._level = level;
      this._transport = transport;
    }
    Str(key, val) {
      this._fields[key] = val;
      return this;
    }
    Int(key, val) {
      this._fields[key] = val;
      return this;
    }
    Bool(key, val) {
      this._fields[key] = val;
      return this;
    }
    Float(key, val) {
      this._fields[key] = val;
      return this;
    }
    Msg(msg) {
      const payload = { ...this._fields, msg, level: this._level };
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
    var _alert = params.alert != null ? params.alert : 1;
    logTransport.enqueue({ msg: params.msg, level: "error" });
    if (_alert) {
      __wx_top_tip(params.msg);
    }
  }
  const script_loaded_map = {};
  function __wx_load_script(src) {
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
      show() {},
      hide() {},
      toggle() {},
      async create(feeds, opt) {
        return [new Error("downloader not ready"), null];
      },
      async browse(feeds, opt) {
        return [new Error("downloader not ready"), null];
      },
    },
    /**
     * Type conversion utilities
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
      return WXAudio.mediaBufferToWav(...args);
    },
    // wav_to_mp3_blob: WXAudio.wavBlobToMP3,
    async media_to_mp3(buf) {
      return WXAudio.mediaToMp3(buf);
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
    load_script: __wx_load_script,
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
    loading() {
      return __wx_loading();
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
        var $root = typeof container === "string"
          ? document.querySelector(container)
          : (container || null);
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
     * @param {{ url: string; method: 'GET' | 'POST'; body?: any }} opt
     */
    async request(opt) {
      return new Promise((resolve, reject) => {
        var xhr = new XMLHttpRequest();
        xhr.open(opt.method, opt.url);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onload = async function () {
          // console.log("[request]xhr.responseText", xhr.responseText);
          try {
            var data = JSON.parse(xhr.responseText);
            if (data.code !== 0) {
              const err = new Error(data.msg);
              err.code = data.code;
              err.data = data.data;
              err.response = data;
              resolve([err, null]);
              return;
            }
            resolve([null, data.data]);
          } catch (e) {
            // ignore
          }
          resolve([null, xhr.responseText]);
        };
        xhr.onerror = function (err) {
          // console.log("[request]xhr.onerror", err);
          resolve([new Error(err.type), null]);
        };
        xhr.send(JSON.stringify(opt.body));
      });
    },
    async save(blob, filename) {
      await __wx_load_script(__wx_asset_url("/lib/FileSaver.min.js"));
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(__wx_asset_url("/lib/jszip.min.js"));
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

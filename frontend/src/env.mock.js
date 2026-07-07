/**
 * @file fakefeed.html 调试环境覆盖和 mock API
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before env.mock.js");
}

(() => {
  window.ua = navigator.userAgent;
  window.__wx_channels_version__ = "fakefeed";

  var __wx_fake_params = new URLSearchParams(window.location.search);
  var __wx_fake_use_mock_api =
    __wx_fake_params.get("mock") === "1" ||
    __wx_fake_params.get("api") === "mock";
  function __wx_existing_api_base() {
    var cfg = window.__wx_channels_config__ || {};
    if (cfg.apiServerProtocol && cfg.apiServerAddr) {
      return cfg.apiServerProtocol + "://" + cfg.apiServerAddr;
    }
    return "";
  }
  var __wx_fake_api_base = "http://127.0.0.1:2022";
  // var __wx_fake_api_base =
  //   __wx_fake_params.get("api_base") ||
  //   __wx_existing_api_base() ||
  //   "http://192.168.1.118:2022";
  var __wx_fake_api_url = new URL(__wx_fake_api_base, window.location.href);
  var __wx_fake_api_protocol = __wx_fake_api_url.protocol.replace(":", "");
  window.__wx_fake_use_mock_api__ = __wx_fake_use_mock_api;
  window.__wx_channels_config__ = {
    defaultHighest: false,
    downloadFilenameTemplate: "{{title}}",
    downloadInFrontend: false,
    downloadPauseWhenDownload: false,
    apiServerProtocol: __wx_fake_api_protocol,
    apiServerAddr: __wx_fake_api_url.host,
    remoteServerEnabled: false,
  };
  window.WXVariable = {};

  WXEnv.applyRuntimeEnv({
    channelsProtocol: __wx_fake_api_protocol,
    channelsHostname: __wx_fake_api_url.host,
    downloadProtocol: __wx_fake_api_protocol,
    downloadHostname: __wx_fake_api_url.host,
    assetsFallbackBase: __wx_fake_api_base + "/__wx_channels_assets",
  });

  if (!window.__wx_fake_use_mock_api__) {
    return;
  }

  var taskSeq = 3;
  var wsClients = [];
  var fakeTasks = [
    {
      id: "fake-task-running",
      status: "running",
      progress: {
        downloaded: 7340032,
        speed: 368640,
      },
      meta: {
        opts: {
          path: "/Users/fake/Downloads",
          name: "Fake feed debug video.mp4",
        },
        res: {
          name: "Fake feed debug video.mp4",
          size: 12345678,
          files: [{ name: "Fake feed debug video.mp4" }],
        },
      },
    },
    {
      id: "fake-task-paused",
      status: "pause",
      progress: {
        downloaded: 2097152,
        speed: 0,
      },
      meta: {
        opts: {
          path: "/Users/fake/Downloads",
          name: "Paused fake task.mp4",
        },
        res: {
          name: "Paused fake task.mp4",
          size: 8388608,
          files: [{ name: "Paused fake task.mp4" }],
        },
      },
    },
    {
      id: "fake-task-done",
      status: "done",
      progress: {
        downloaded: 5242880,
        speed: 0,
      },
      meta: {
        opts: {
          path: "/Users/fake/Downloads",
          name: "Completed fake task.mp4",
        },
        res: {
          name: "Completed fake task.mp4",
          size: 5242880,
          files: [{ name: "Completed fake task.mp4" }],
        },
      },
    },
  ];
  var requestedTaskCount = Number(
    __wx_fake_params.get("tasks") ||
      __wx_fake_params.get("task_count") ||
      fakeTasks.length,
  );
  if (
    !Number.isFinite(requestedTaskCount) ||
    requestedTaskCount < fakeTasks.length
  ) {
    requestedTaskCount = fakeTasks.length;
  }

  function makeTask(index) {
    var statuses = ["ready", "running", "wait", "pause", "error", "done"];
    var status = statuses[index % statuses.length];
    var filename =
      "Fake bulk task " + String(index + 1).padStart(5, "0") + ".mp4";
    var size = 5242880 + (index % 20) * 1048576;
    var downloaded =
      status === "done"
        ? size
        : status === "running" || status === "pause"
          ? Math.floor((size * ((index % 80) + 10)) / 100)
          : 0;
    return {
      id: "fake-bulk-task-" + (index + 1),
      status: status,
      progress: {
        downloaded: downloaded,
        speed: status === "running" ? 131072 + (index % 8) * 32768 : 0,
      },
      meta: {
        opts: {
          path: "/Users/fake/Downloads",
          name: filename,
        },
        res: {
          name: filename,
          size: size,
          files: [{ name: filename }],
        },
      },
    };
  }

  for (var i = fakeTasks.length; i < requestedTaskCount; i++) {
    fakeTasks.push(makeTask(i));
  }
  taskSeq = Math.max(taskSeq, fakeTasks.length);
  var shouldSendWSBatch = !/^(0|false|off)$/i.test(
    __wx_fake_params.get("ws_batch") || "",
  );

  function clone(value) {
    return JSON.parse(JSON.stringify(value));
  }

  function parseBody(body) {
    if (!body) return {};
    if (typeof body === "string") {
      try {
        return JSON.parse(body);
      } catch {
        return {};
      }
    }
    return body;
  }

  function getPath(url) {
    try {
      return new URL(url, window.location.href).pathname;
    } catch {
      return String(url || "");
    }
  }

  function getURLSearchParams(url) {
    try {
      return new URL(url, window.location.href).searchParams;
    } catch {
      return new URLSearchParams();
    }
  }

  function getNumberParam(url, body, names, fallback) {
    var params = getURLSearchParams(url);
    for (var i = 0; i < names.length; i++) {
      var name = names[i];
      var value =
        body && typeof body[name] !== "undefined"
          ? body[name]
          : params.get(name);
      var numberValue = Number(value);
      if (Number.isFinite(numberValue) && numberValue > 0) {
        return Math.floor(numberValue);
      }
    }
    return fallback;
  }

  function getStringParam(url, body, names, fallback) {
    var params = getURLSearchParams(url);
    for (var i = 0; i < names.length; i++) {
      var name = names[i];
      var value =
        body && typeof body[name] !== "undefined"
          ? body[name]
          : params.get(name);
      if (typeof value === "string" && value.trim()) {
        return value.trim();
      }
    }
    return fallback;
  }

  function normalizeStatus(status) {
    var value = String(status || "")
      .trim()
      .toLowerCase();
    if (value === "paused") return "pause";
    if (
      value === "failed" ||
      value === "fail" ||
      value === "failure" ||
      value === "errored"
    ) {
      return "error";
    }
    if (value === "pending" || value === "waiting" || value === "queued") {
      return "wait";
    }
    if (value === "completed" || value === "success" || value === "finished") {
      return "done";
    }
    return value;
  }

  function matchesStatusFilter(task, status) {
    var value = normalizeStatus(status);
    if (!value || value === "all") return true;
    var taskStatus = normalizeStatus(task && task.status);
    if (value === "wait") {
      return taskStatus === "ready" || taskStatus === "wait";
    }
    return taskStatus === value;
  }

  function statusCounts() {
    var counts = {
      total: fakeTasks.length,
      ready: 0,
      running: 0,
      wait: 0,
      pause: 0,
      error: 0,
      done: 0,
    };
    fakeTasks.forEach(function (task) {
      var status = normalizeStatus(task.status);
      if (typeof counts[status] === "number") {
        counts[status] += 1;
      }
    });
    return counts;
  }

  function findTask(id) {
    return fakeTasks.find(function (task) {
      return task.id === id;
    });
  }

  function createTask(body) {
    taskSeq += 1;
    var suffix = typeof body.suffix === "string" ? body.suffix : "";
    var filename = body.filename || body.title || "";
    if (!filename && body.url) {
      try {
        filename =
          decodeURIComponent(new URL(body.url).pathname.split("/").pop()) || "";
      } catch {
        filename = "";
      }
    }
    filename = filename || "Fake created task";
    if (filename && suffix && !filename.endsWith(suffix)) {
      filename += suffix;
    }
    var task = {
      id: (body.id || "fake-created") + "-" + taskSeq,
      status: "ready",
      progress: {
        downloaded: 0,
        speed: 0,
      },
      meta: {
        opts: {
          path: "/Users/fake/Downloads",
          name: filename,
        },
        res: {
          name: filename,
          size: 15728640,
          files: [{ name: filename }],
        },
      },
    };
    fakeTasks.unshift(task);
    broadcast({
      type: "event",
      data: {
        Key: "create",
        Task: clone(task),
      },
    });
    return task;
  }

  function setTaskStatus(id, status) {
    var task = findTask(id);
    if (task) {
      task.status = status;
      if (status === "running") {
        task.progress.speed = 245760;
      } else {
        task.progress.speed = 0;
      }
      broadcast({
        type: "event",
        data: {
          Key: status,
          Task: clone(task),
        },
      });
    }
    return task || null;
  }

  function broadcast(message) {
    var text = JSON.stringify(message);
    wsClients.forEach(function (client) {
      if (client.readyState === 1 && typeof client.onmessage === "function") {
        client.onmessage({ data: text });
      }
    });
  }

  window.__wx_fake_tasks = fakeTasks;
  window.__wx_fake_api_response = function (url, rawBody) {
    var path = getPath(url);
    var body = parseBody(rawBody);
    console.log("[fakefeed] api", path, body);
    if (path === "/api/task/list") {
      var page = getNumberParam(url, body, ["page"], 1);
      var pageSize = getNumberParam(url, body, ["page_size", "pageSize"], 50);
      var status = normalizeStatus(
        getStringParam(url, body, ["status"], "all"),
      );
      var list =
        status && status !== "all"
          ? fakeTasks.filter(function (task) {
              return normalizeStatus(task.status) === status;
            })
          : fakeTasks;
      var start = (page - 1) * pageSize;
      return {
        code: 0,
        data: {
          list: clone(list.slice(start, start + pageSize)),
          total: list.length,
          page: page,
          page_size: pageSize,
          status_counts: statusCounts(),
        },
      };
    }
    if (path === "/api/task/create" || path === "/api/task/create2") {
      return { code: 0, data: clone(createTask(body)) };
    }
    if (path === "/api/task/create3") {
      var text = String((body && body.text) || "");
      var urls = Array.isArray(body && body.urls) ? body.urls : [];
      text.split(/\r?\n/).forEach(function (line) {
        var value = String(line || "").trim();
        if (value) urls.push(value);
      });
      var ids = urls.map(function (url) {
        return createTask({ url: url }).id;
      });
      return { code: 0, data: { ids: ids, skipped: [], failed: [] } };
    }
    if (path === "/api/task/create_batch") {
      var feeds = Array.isArray(body.feeds) ? body.feeds : [];
      return {
        code: 0,
        data: feeds.map(function (feed) {
          return clone(createTask(feed));
        }),
      };
    }
    if (path === "/api/task/start") {
      return { code: 0, data: clone(setTaskStatus(body.id, "running")) };
    }
    if (path === "/api/task/pause") {
      return { code: 0, data: clone(setTaskStatus(body.id, "pause")) };
    }
    if (path === "/api/task/resume") {
      return { code: 0, data: clone(setTaskStatus(body.id, "running")) };
    }
    if (path === "/api/task/delete") {
      fakeTasks = fakeTasks.filter(function (task) {
        return task.id !== body.id;
      });
      window.__wx_fake_tasks = fakeTasks;
      broadcast({ type: "event", data: { Key: "delete", id: body.id } });
      return { code: 0, data: { ok: true } };
    }
    if (path === "/api/task/start_all") {
      var startStatus = normalizeStatus(
        getStringParam(url, body, ["status"], "all"),
      );
      fakeTasks.forEach(function (task) {
        if (!matchesStatusFilter(task, startStatus)) {
          return;
        }
        if (task.status !== "done") setTaskStatus(task.id, "running");
      });
      return { code: 0, data: { ok: true } };
    }
    if (path === "/api/task/pause_all") {
      var pauseStatus = normalizeStatus(
        getStringParam(url, body, ["status"], "all"),
      );
      fakeTasks.forEach(function (task) {
        if (!matchesStatusFilter(task, pauseStatus)) {
          return;
        }
        if (task.status === "running") setTaskStatus(task.id, "pause");
      });
      return { code: 0, data: { ok: true } };
    }
    if (path === "/api/task/clear") {
      fakeTasks = fakeTasks.filter(function (task) {
        return task.status === "running";
      });
      window.__wx_fake_tasks = fakeTasks;
      return { code: 0, data: { ok: true } };
    }
    if (
      path === "/api/show_file" ||
      path === "/api/open_download_dir" ||
      path.indexOf("/__wx_channels_api/") === 0
    ) {
      return { code: 0, data: { ok: true } };
    }
    return { code: 0, data: { ok: true, path: path } };
  };

  window.fetch = async function (input, init) {
    var url = typeof input === "string" ? input : input && input.url;
    var body = init && init.body;
    var payload = window.__wx_fake_api_response(url, body);
    var text = JSON.stringify(payload);
    if (typeof Response === "function") {
      return new Response(text, {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    return {
      ok: true,
      status: 200,
      headers: {
        get: function () {
          return "application/json";
        },
      },
      json: async function () {
        return payload;
      },
      text: async function () {
        return text;
      },
    };
  };

  function createEventTarget() {
    return {
      listeners: {},
      addEventListener: function (type, cb) {
        if (!this.listeners[type]) this.listeners[type] = [];
        this.listeners[type].push(cb);
      },
      removeEventListener: function (type, cb) {
        if (!this.listeners[type]) return;
        this.listeners[type] = this.listeners[type].filter(function (item) {
          return item !== cb;
        });
      },
      dispatchEvent: function (type, event) {
        (this.listeners[type] || []).forEach(function (cb) {
          cb(event);
        });
      },
    };
  }

  window.XMLHttpRequest = function FakeXMLHttpRequest() {
    this.headers = {};
    this.listeners = {};
    this.upload = createEventTarget();
    this.readyState = 0;
    this.status = 0;
    this.statusText = "";
    this.responseText = "";
    this.response = "";
    this.responseURL = "";
    this.onloadend = null;
  };
  window.XMLHttpRequest.prototype.open = function (method, url) {
    this.method = method;
    this.url = url;
    this.responseURL = url;
    this.readyState = 1;
  };
  window.XMLHttpRequest.prototype.setRequestHeader = function (key, value) {
    this.headers[key] = value;
  };
  window.XMLHttpRequest.prototype.addEventListener = function (type, cb) {
    if (!this.listeners[type]) this.listeners[type] = [];
    this.listeners[type].push(cb);
  };
  window.XMLHttpRequest.prototype.removeEventListener = function (type, cb) {
    if (!this.listeners[type]) return;
    this.listeners[type] = this.listeners[type].filter(function (item) {
      return item !== cb;
    });
  };
  window.XMLHttpRequest.prototype.dispatchEvent = function (type, event) {
    (this.listeners[type] || []).forEach(function (cb) {
      cb(event);
    });
  };
  window.XMLHttpRequest.prototype.getResponseHeader = function (key) {
    return key && key.toLowerCase() === "content-type"
      ? "application/json"
      : null;
  };
  window.XMLHttpRequest.prototype.getAllResponseHeaders = function () {
    return "content-type: application/json\r\n";
  };
  window.XMLHttpRequest.prototype.send = function (body) {
    var xhr = this;
    setTimeout(function () {
      var payload = window.__wx_fake_api_response(xhr.url, body);
      xhr.status = 200;
      xhr.statusText = "OK";
      xhr.readyState = 4;
      xhr.responseText = JSON.stringify(payload);
      xhr.response = xhr.responseText;
      xhr.dispatchEvent("progress", {
        loaded: xhr.responseText.length,
        total: xhr.responseText.length,
      });
      if (typeof xhr.onreadystatechange === "function") {
        xhr.onreadystatechange();
      }
      if (typeof xhr.onload === "function") {
        xhr.onload();
      }
      xhr.upload.dispatchEvent("loadend", { target: xhr });
      if (typeof xhr.onloadend === "function") {
        xhr.onloadend();
      }
    }, 0);
  };
  window.XMLHttpRequest.prototype.abort = function () {
    this.readyState = 0;
  };

  window.WebSocket = function FakeWebSocket(url) {
    this.url = url;
    this.readyState = 0;
    wsClients.push(this);
    setTimeout(() => {
      this.readyState = 1;
      if (typeof this.onopen === "function") {
        this.onopen({ target: this });
      }
      if (typeof this.onmessage === "function") {
        if (shouldSendWSBatch) {
          this.onmessage({
            data: JSON.stringify({
              type: "batch_tasks",
              data: clone(fakeTasks),
            }),
          });
        }
      }
    }, 30);
  };
  window.WebSocket.CONNECTING = 0;
  window.WebSocket.OPEN = 1;
  window.WebSocket.CLOSING = 2;
  window.WebSocket.CLOSED = 3;
  window.WebSocket.prototype.send = function (data) {
    console.log("[fakefeed] websocket.send", data);
  };
  window.WebSocket.prototype.close = function () {
    this.readyState = 3;
    wsClients = wsClients.filter((client) => client !== this);
    if (typeof this.onclose === "function") {
      this.onclose({ target: this });
    }
  };
})();

(function (global) {
  "use strict";

  const timeless = window.Timeless;
  const RequestCore = timeless.kit.RequestCore;
  const SocketClientCore = timeless.kit.SocketClientCore;
  const ChannelCore = timeless.kit.ChannelCore;
  const request = timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (typeof payload.code !== "undefined" && payload.code !== 0) {
        return timeless.Result.Err(payload.msg, payload.code, payload.data);
      }
      return timeless.Result.Ok(
        typeof payload.data === "undefined" ? payload : payload.data,
      );
    },
  });

  if (global.DL && global.DownloaderModel && global.DownloadTaskModel) {
    return;
  }

  const task_status = Object.freeze({
    0: "waiting",
    1: "preparing",
    2: "downloading",
    3: "paused",
    4: "merging",
    5: "finished",
    6: "failed",
    7: "cancelled",
  });
  const success_statuses = new Set(["done", "finished", "completed", "success"]);
  const failure_statuses = new Set([
    "error",
    "failed",
    "failure",
    "cancelled",
    "canceled",
  ]);

  function normalize_status(value) {
    const raw = String(value ?? "")
      .trim()
      .toLowerCase();
    if (Object.prototype.hasOwnProperty.call(task_status, raw)) {
      return task_status[raw];
    }
    if (raw === "wait" || raw === "pending" || raw === "queued") {
      return "waiting";
    }
    if (raw === "running") {
      return "downloading";
    }
    if (raw === "pause") {
      return "paused";
    }
    if (raw === "done" || raw === "completed" || raw === "success") {
      return "finished";
    }
    if (raw === "error" || raw === "failure" || raw === "errored") {
      return "failed";
    }
    return raw || "waiting";
  }

  function number_value(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function error_value(value, fallback) {
    if (value instanceof Error) {
      return value;
    }
    if (value && typeof value === "object") {
      const message = value.message || value.msg || value.error;
      const error = new Error(message || fallback || "Download task failed");
      Object.assign(error, value);
      return error;
    }
    return new Error(String(value || fallback || "Download task failed"));
  }

  function deferred() {
    let resolve;
    let reject;
    const promise = new Promise((resolve_promise, reject_promise) => {
      resolve = resolve_promise;
      reject = reject_promise;
    });
    // A task may be consumed only through event handlers. Avoid reporting an
    // unhandled rejection while keeping the original promise rejectable.
    promise.catch(function () {});
    return { promise, resolve, reject, settled: false };
  }

  function event_channel() {
    const listeners = new Set();
    return {
      subscribe(listener) {
        if (typeof listener !== "function") {
          throw new TypeError("event listener must be a function");
        }
        listeners.add(listener);
        return function unsubscribe() {
          listeners.delete(listener);
        };
      },
      emit(payload) {
        listeners.forEach((listener) => {
          try {
            listener(payload);
          } catch (error) {
            global.setTimeout(() => {
              throw error;
            }, 0);
          }
        });
      },
      clear() {
        listeners.clear();
      },
    };
  }

  function first_task_file(record) {
    const files = Array.isArray(record && record.files)
      ? record.files
      : Array.isArray(record && record.resources)
        ? record.resources
        : [];
    return files[0] || null;
  }

  function file_path(record) {
    const direct = String(
      (record && (record.filepath || record.file_path || record.local_path)) || "",
    ).trim();
    if (direct) {
      return direct;
    }
    const file = first_task_file(record);
    if (!file) {
      return "";
    }
    const explicit = String(
      file.filepath || file.file_path || file.local_path || "",
    ).trim();
    if (explicit) {
      return explicit;
    }
    const directory = String(file.download_dir || file.downloadDir || "").trim();
    const name = String(file.name || file.filename || "").trim();
    if (!directory) {
      return name;
    }
    if (!name) {
      return directory;
    }
    const separator = directory.includes("\\") && !directory.includes("/") ? "\\" : "/";
    return directory.replace(/[\\/]+$/, "") + separator + name.replace(/^[\\/]+/, "");
  }

  function task_title(record, fallback) {
    const file = first_task_file(record);
    return String(
      (record && (record.name || record.title || record.filename)) ||
        (file && (file.name || file.filename)) ||
        fallback ||
        "",
    );
  }

  function aggregate_resources(resources) {
    return (Array.isArray(resources) ? resources : []).reduce(
      (total, resource) => {
        total.downloaded += number_value(resource && resource.downloaded, 0);
        total.total += number_value(resource && (resource.size || resource.total), 0);
        total.speed += number_value(resource && resource.speed, 0);
        return total;
      },
      { downloaded: 0, total: 0, speed: 0 },
    );
  }

  function task_progress(record, previous) {
    const progress = record && record.progress;
    const detail = progress && typeof progress === "object" ? progress : {};
    const resources =
      (record && (record.resources || record.files)) || detail.resources || [];
    const aggregate = aggregate_resources(resources);
    const downloaded = number_value(
      record && record.downloaded,
      number_value(detail.downloaded, aggregate.downloaded),
    );
    const total = number_value(
      record && (record.size || record.total),
      number_value(detail.size || detail.total, aggregate.total),
    );
    const speed = number_value(
      record && record.speed,
      number_value(detail.speed, aggregate.speed),
    );
    let percent =
      typeof progress === "number" || typeof progress === "string"
        ? number_value(progress, NaN)
        : number_value(detail.percent ?? detail.progress, NaN);
    if (!Number.isFinite(percent) && total > 0) {
      percent = (downloaded * 100) / total;
    }
    if (!Number.isFinite(percent)) {
      percent = previous ? number_value(previous.percent, 0) : 0;
    }
    return {
      percent: Math.min(100, Math.max(0, Math.round(percent * 100) / 100)),
      downloaded: Math.max(0, downloaded),
      total: Math.max(0, total),
      speed: Math.max(0, speed),
    };
  }

  function progress_changed(previous, next) {
    return (
      !previous ||
      previous.percent !== next.percent ||
      previous.downloaded !== next.downloaded ||
      previous.total !== next.total ||
      previous.speed !== next.speed
    );
  }

  function merge_files(current_files, updates) {
    if (!Array.isArray(updates)) {
      return current_files;
    }
    const current = Array.isArray(current_files) ? current_files : [];
    const update_map = new Map();
    updates.forEach((file) => {
      const id = file && (file.id ?? file.resource_id);
      if (id !== undefined && id !== null) {
        update_map.set(String(id), file);
      }
    });
    const merged = current.map((file) => {
      const id = file && (file.id ?? file.resource_id);
      const update = id === undefined || id === null ? null : update_map.get(String(id));
      if (!update) {
        return file;
      }
      update_map.delete(String(id));
      return Object.assign({}, file, update);
    });
    update_map.forEach((file) => merged.push(file));
    return merged;
  }

  function merge_task_record(current, update) {
    const next = Object.assign({}, current || {}, update || {});
    if (Object.prototype.hasOwnProperty.call(update || {}, "files")) {
      next.files = merge_files(current && current.files, update.files);
      // Platform create responses use `resources`, while REST/WS task records
      // use `files`. Keep the compatibility alias synchronized so a final
      // Hermes filename (extension and duplicate suffix included) cannot be
      // hidden by the stale create response.
      if (Array.isArray(current && current.resources)) {
        next.resources = merge_files(current.resources, update.files);
      }
    }
    if (Object.prototype.hasOwnProperty.call(update || {}, "resources")) {
      next.resources = merge_files(current && current.resources, update.resources);
    }
    return next;
  }

  /**
   * A stable domain object for one server-side download task.
   * Reactive fields are exposed directly; use `.value` outside Timeless views.
   */
  function DownloadTaskModel(props) {
    const {
      owner = null,
      record: initial = {},
      pending = false,
      name: initial_name = "",
    } = props || {};
    const id_ = timeless.ref(initial.id ?? initial.task_id ?? null);
    const status_ = timeless.ref(
      pending ? "creating" : normalize_status(initial.status),
    );
    const title_ = timeless.ref(task_title(initial, initial_name));
    const filepath_ = timeless.ref(file_path(initial));
    const progress_ = timeless.refobj(task_progress(initial));
    const error_ = timeless.ref(null);
    const raw_ = timeless.refobj(Object.assign({}, initial));
    const events = {
      change: event_channel(),
      fail: event_channel(),
      progress: event_channel(),
      success: event_channel(),
    };
    let ready_state = deferred();
    let finished_state = deferred();
    let terminal_state = null;
    let last_failure = null;
    let disposed = false;

    const state = {
      id: id_,
      status: status_,
      title: title_,
      name: title_,
      filepath: filepath_,
      progress: progress_,
      error: error_,
      raw: raw_,
    };
    const ui = {};
    const reqs = owner && owner.reqs ? owner.reqs : {};
    const methods = {
      onSuccess: on_success,
      onFail: on_fail,
      onProgress: on_progress,
      onChange: on_change,
      start,
      resume,
      pause,
      retry,
      open,
      delete: delete_task,
      snapshot,
    };
    const handler = {
      update,
      mark_ready,
      fail,
      begin,
      dispose,
    };
    const model = {
      state,
      ui,
      reqs,
      methods,
      handler,
      ...state,
      get ready() {
        return ready_state.promise;
      },
      get finished() {
        return finished_state.promise;
      },
      ...methods,
      _update: handler.update,
      _mark_ready: handler.mark_ready,
      _fail: handler.fail,
      _begin: handler.begin,
      _dispose: handler.dispose,
    };

    function on_success(listener) {
      const unsubscribe = events.success.subscribe(listener);
      if (terminal_state === "success") {
        global.queueMicrotask(() => listener(model));
      }
      return unsubscribe;
    }

    function on_fail(listener) {
      const unsubscribe = events.fail.subscribe(listener);
      if (terminal_state === "fail" && last_failure) {
        global.queueMicrotask(() => listener({ error: last_failure, task: model }));
      }
      return unsubscribe;
    }

    function on_progress(listener) {
      return events.progress.subscribe(listener);
    }

    function on_change(listener) {
      return events.change.subscribe(listener);
    }

    function start() {
      return require_owner("start")(model);
    }

    function resume() {
      return require_owner("resume")(model);
    }

    function pause() {
      return require_owner("pause")(model);
    }

    function retry() {
      return require_owner("retry")(model);
    }

    function open() {
      return require_owner("open")(model);
    }

    function delete_task(options) {
      return require_owner("delete")(model, options);
    }

    function snapshot() {
      return {
        id: id_.value,
        status: status_.value,
        title: title_.value,
        name: title_.value,
        filepath: filepath_.value,
        progress: Object.assign({}, progress_.value),
        error: error_.value,
        raw: Object.assign({}, raw_.value),
      };
    }

    function require_owner(method) {
      if (!owner || typeof owner[method] !== "function") {
        throw new Error(`Download task is not attached to a DL instance: ${method}`);
      }
      return owner[method];
    }

    function mark_ready() {
      if (!ready_state.settled) {
        ready_state.settled = true;
        ready_state.resolve(model);
      }
      return model;
    }

    function begin(status) {
      terminal_state = null;
      last_failure = null;
      error_.as(null);
      finished_state = deferred();
      if (status) {
        status_.as(normalize_status(status));
      }
      return model;
    }

    function fail(value, options) {
      const error = error_value(value);
      const terminal = !options || options.terminal !== false;
      last_failure = error;
      error_.as(error);
      if (terminal) {
        terminal_state = "fail";
        if (!options || options.preserve_status !== true) {
          status_.as("failed");
        }
        if (!finished_state.settled) {
          finished_state.settled = true;
          finished_state.reject(error);
        }
      }
      if (!ready_state.settled && (!options || options.creation !== false)) {
        ready_state.settled = true;
        ready_state.reject(error);
      }
      events.fail.emit({ error, task: model });
      events.change.emit({ task: model, type: "error", error });
      return error;
    }

    function update(record, options) {
      if (disposed || !record || typeof record !== "object") {
        return model;
      }
      const previous_raw = raw_.value || {};
      const next_raw =
        options && options.replace
          ? Object.assign({}, record)
          : merge_task_record(previous_raw, record);
      raw_.as(next_raw);
      const next_id = next_raw.id ?? next_raw.task_id;
      if (next_id !== undefined && next_id !== null && next_id !== "") {
        id_.as(next_id);
      }
      const next_title = task_title(next_raw, title_.value);
      if (next_title !== title_.value) {
        title_.as(next_title);
      }
      const next_path = file_path(next_raw);
      if (next_path && next_path !== filepath_.value) {
        filepath_.as(next_path);
      }
      const previous_progress = progress_.value;
      const next_progress = task_progress(next_raw, previous_progress);
      if (progress_changed(previous_progress, next_progress)) {
        progress_.as(next_progress);
        events.progress.emit({
          task: model,
          progress: next_progress,
          previous: previous_progress,
        });
      }
      const previous_status = status_.value;
      const next_status = normalize_status(next_raw.status ?? previous_status);
      if (next_status !== previous_status) {
        status_.as(next_status);
      }
      if (
        next_status !== "deleted" &&
        !success_statuses.has(next_status) &&
        !failure_statuses.has(next_status) &&
        (success_statuses.has(previous_status) ||
          failure_statuses.has(previous_status))
      ) {
        terminal_state = null;
        last_failure = null;
        error_.as(null);
        finished_state = deferred();
      }
      const message = next_raw.error || next_raw.error_message || next_raw._errMsg;
      if (success_statuses.has(next_status) && terminal_state !== "success") {
        terminal_state = "success";
        error_.as(null);
        if (!finished_state.settled) {
          finished_state.settled = true;
          finished_state.resolve(model);
        }
        events.success.emit(model);
      } else if (failure_statuses.has(next_status) && terminal_state !== "fail") {
        fail(message || `Download task ${next_status}`, {
          creation: false,
          preserve_status: true,
          terminal: true,
        });
      }
      events.change.emit({
        task: model,
        type: "update",
        record: next_raw,
        previousStatus: previous_status,
      });
      return model;
    }

    function dispose() {
      disposed = true;
      Object.values(events).forEach((channel) => channel.clear());
    }

    update(initial, { replace: true });
    if (!pending) {
      mark_ready();
    }
    return model;
  }

  function resolve_create_request(input) {
    const object = typeof input === "string" ? { url: input } : Object.assign({}, input || {});
    const is_url_task =
      !!object.url && !object.platform && !object.content && !object.platform_id;
    return {
      mode: is_url_task ? "url" : "platform",
      body: { objects: [object] },
      object,
    };
  }

  function created_task_record(response_data) {
    const items = response_data && response_data.tasks;
    const item = Array.isArray(items) ? items[0] : null;
    if (!item) {
      throw new Error("Create download task returned no task");
    }
    if (item.success === false) {
      throw error_value(
        {
          message: item.error || item.msg || "Create download task failed",
          code: item.code,
          data: item.data,
          item,
        },
        "Create download task failed",
      );
    }
    if (typeof item.code !== "undefined" && Number(item.code) !== 0) {
      throw error_value(
        {
          message: item.msg || "Create download task failed",
          code: Number(item.code),
          data: item.data,
          item,
        },
        "Create download task failed",
      );
    }
    const data = item.data || item.task || item;
    if (data && data.task) {
      const resources = [];
      if (data.resource) {
        resources.push(data.resource);
      }
      if (Array.isArray(data.resources)) {
        resources.push(...data.resources);
      }
      return Object.assign({}, data.task, resources.length ? { resources } : {});
    }
    return data;
  }

  function runtime_config_origin() {
    const config = global.__d_config || {};
    if (config.remoteServerEnabled) {
      return "https://weixin110.qq.com";
    }
    if (config.apiOrigin) {
      return String(config.apiOrigin);
    }
    if (config.assets_base_url) {
      try {
        return new URL(config.assets_base_url, global.location.href).origin;
      } catch {
        // Use the current origin below.
      }
    }
    return global.location.origin;
  }

  function default_web_socket_url(configured_url) {
    const config = global.__d_config || {};
    const configured =
      configured_url ||
      config.downloaderWSURL ||
      config.downloader_ws_url;
    if (configured) {
      return String(configured);
    }
    const url = new URL(runtime_config_origin());
    const protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${url.host}/ws/v1/download_task`;
  }


  function create_download_task(params) {
    const path =
      params.mode === "url"
        ? "/api/v1/download_task/create_by_url"
        : "/api/v1/download_task/create";
    return request.post(path, params.body);
  }

  function list_download_tasks(params) {
    return request.get("/api/v1/download_task/list", params);
  }

  function delete_download_task(params) {
    return request.post("/api/v1/download_task/delete", {
      task_ids: params.ids,
      delete_files: !!params.delete_files,
    });
  }

  function start_download_task(id) {
    return request.post("/api/v1/download_task/start", { task_ids: [id] });
  }

  function resume_download_task(id) {
    return request.post("/api/v1/download_task/resume", { task_ids: [id] });
  }

  function pause_download_task(id) {
    return request.post("/api/v1/download_task/pause", { task_ids: [id] });
  }

  function retry_download_task(id) {
    return request.post("/api/v1/download_task/retry", { task_ids: [id] });
  }

  function prepare_download_task(params) {
    const path =
      params.mode === "url"
        ? "/api/v1/download_task/prepare_by_url"
        : "/api/v1/download_task/prepare";
    return request.post(path, params.body);
  }

  function start_all_download_tasks(params) {
    const body = {};
    if (params && params.status && params.status !== "all") {
      body.status = params.status;
    }
    return request.post("/api/v1/download_task/start_all", body);
  }

  function pause_all_download_tasks(params) {
    const body = {};
    if (params && params.status && params.status !== "all") {
      body.status = params.status;
    }
    return request.post("/api/v1/download_task/pause_all", body);
  }

  function clear_download_tasks(params) {
    return request.post("/api/v1/download_task/clear_all", {
      delete_files: !!(params && (params.delete_files ?? params.deleteFiles)),
    });
  }

  function show_download_task_file(params) {
    return request.post("/api/show_file", params);
  }

  /**
   * Download manager domain model. Owns and synchronizes multiple
   * DownloadTaskModel instances.
   *
   * @example
   * const dl$ = DL({ client: http_client });
   * const task$ = dl$.create({ url: "https://example.com/video.mp4" });
   * task$.onProgress(({ progress }) => console.log(progress.percent));
   * task$.onSuccess(() => console.log(task$.filepath.value));
   * task$.onFail(({ error }) => console.error(error));
   */
  function DownloaderModel(props) {
    const {
      client: http_client,
      socket_client,
      debug = false,
      reconnect = true,
      reconnect_interval: reconnect_interval_value = 5000,
      auto_start = true,
    } = props || {};
    const reconnect_enabled = reconnect !== false;
    const auto_start_enabled = auto_start !== false;
    // const configured_websocket_url = ws_url;
    if (!http_client) {
      throw new TypeError("DownloaderModel requires a client");
    }

    const reqs = {
      download: {
        create: new RequestCore(create_download_task, { client: http_client }),
        list: new RequestCore(list_download_tasks, { client: http_client }),
        delete: new RequestCore(delete_download_task, { client: http_client }),
        start: new RequestCore(start_download_task, { client: http_client }),
        resume: new RequestCore(resume_download_task, { client: http_client }),
        pause: new RequestCore(pause_download_task, { client: http_client }),
        retry: new RequestCore(retry_download_task, { client: http_client }),
        prepare: new RequestCore(prepare_download_task, { client: http_client }),
        start_all: new RequestCore(start_all_download_tasks, {
          client: http_client,
        }),
        pause_all: new RequestCore(pause_all_download_tasks, {
          client: http_client,
        }),
        clear: new RequestCore(clear_download_tasks, { client: http_client }),
      },
      file: {
        show: new RequestCore(show_download_task_file, { client: http_client }),
      },
    };

    const task_list_ = timeless.refarr([]);
    const websocket_connected_ = timeless.ref(false);
    const websocket_connecting_ = timeless.ref(false);
    const last_error_ = timeless.ref(null);
    const tasks_by_id = new Map();
    const websocket_url = "/ws/v1/download_task";
    const reconnect_interval = Math.max(
      250,
      number_value(reconnect_interval_value, 5000),
    );
    let destroyed = false;
    let ready_promise = null;

    const state = {
      task_list: task_list_,
      tasks: task_list_,
      websocket_connected: websocket_connected_,
      websocket_connecting: websocket_connecting_,
      last_error: last_error_,
      websocket_url,
    };
    const ui = {};
    const methods = {
      create,
      list: refresh,
      refresh,
      get,
      delete: delete_task,
      start,
      resume,
      continue: resume,
      pause,
      retry,
      prepare,
      startAll: start_all,
      pauseAll: pause_all,
      clear: clear_all,
      open,
      connect,
      reconnect: reconnect_channel,
      disconnect,
      ready,
      destroy,
    };
    const handler = {
      decode_socket_message,
      sync_channel_state,
      handle_reconnected,
      task_id,
      append_task,
      remove_task,
      upsert,
      adopt_pending_task,
      initial_create_record,
      fetch_task_page,
      replace_server_tasks,
      action_result,
      run_action,
      handle_snapshot,
      handle_web_socket_message,
    };
    const channel = new ChannelCore(websocket_url, {
      client: socket_client,
      process: handler.decode_socket_message,
      reconnect: {
        enabled: reconnect_enabled,
        interval: reconnect_interval,
      },
    });

    channel.onMessage(handler.handle_web_socket_message);
    channel.onStateChange(handler.sync_channel_state);
    channel.onReconnected(handler.handle_reconnected);

    const domain = {
      state,
      ui,
      reqs,
      methods,
      handler,
      channel,
      socket_client,
      ...state,
      requests: reqs,
      ...methods,
    };

    function decode_socket_message(value) {
      if (typeof value !== "string") {
        return value;
      }
      try {
        return JSON.parse(value);
      } catch {
        return null;
      }
    }

    function sync_channel_state(channel_state) {
      websocket_connected_.as(!!channel_state.connected);
      websocket_connecting_.as(!!channel_state.connecting);
      if (channel_state.error) {
        last_error_.as(channel_state.error);
      } else if (channel_state.connected) {
        last_error_.as(null);
      }
    }

    function handle_reconnected() {
      if (!destroyed) {
        methods.refresh().catch((error) => last_error_.as(error));
      }
    }

    function task_id(target) {
      const value =
        target && typeof target === "object"
          ? target.id && typeof target.id === "object" && "value" in target.id
            ? target.id.value
            : target.id ?? target.task_id
          : target;
      if (value === undefined || value === null || value === "") {
        throw new Error("Download task id is required");
      }
      return value;
    }

    function get(target) {
      try {
        return tasks_by_id.get(String(task_id(target))) || null;
      } catch {
        return null;
      }
    }

    function append_task(task, prepend) {
      const current = task_list_.value || [];
      task_list_.as(prepend === false ? [...current, task] : [task, ...current]);
    }

    function remove_task(target, dispose) {
      const id = task_id(target);
      const key = String(id);
      const task = tasks_by_id.get(key) || (target && target.id ? target : null);
      tasks_by_id.delete(key);
      task_list_.as(
        (task_list_.value || []).filter((current) => current !== task && String(current.id.value) !== key),
      );
      if (dispose && task) {
        task._dispose();
      }
      return task;
    }

    function upsert(record, options) {
      if (!record || typeof record !== "object") {
        return null;
      }
      const id = record.id ?? record.task_id;
      if (id === undefined || id === null || id === "") {
        return null;
      }
      const key = String(id);
      let task = tasks_by_id.get(key);
      if (!task) {
        task = DownloadTaskModel({ owner: domain, record });
        tasks_by_id.set(key, task);
        append_task(task, !options || options.prepend !== false);
      } else {
        task._update(record, options);
      }
      return task;
    }

    function adopt_pending_task(task, record) {
      const id = record && (record.id ?? record.task_id);
      if (id === undefined || id === null || id === "") {
        throw new Error("Created download task has no id");
      }
      const key = String(id);
      const existing = tasks_by_id.get(key);
      task._update(
        Object.prototype.hasOwnProperty.call(record, "status")
          ? record
          : Object.assign({ status: "waiting" }, record),
      );
      if (existing && existing !== task) {
        // A WebSocket create/update can arrive before the REST create response.
        // Apply that newer server snapshot last while keeping the task object
        // returned by create() stable for its consumers.
        task._update(existing.raw.value || {});
        task_list_.as((task_list_.value || []).filter((current) => current !== existing));
        existing._dispose();
      }
      tasks_by_id.set(key, task);
      task._mark_ready();
      return task;
    }

    function initial_create_record(request_info) {
      const object = request_info.object;
      return {
        status: "creating",
        name: object.name || object.filename || object.title || "",
        source_url: object.url || "",
      };
    }

    function create(object) {
      const request_info = resolve_create_request(object);
      const task = DownloadTaskModel({
        owner: domain,
        pending: true,
        record: initial_create_record(request_info),
      });
      append_task(task, true);
      Promise.resolve()
        .then(() => reqs.download.create.run(request_info))
        .then((result) => {
          if (!result || result.error) {
            throw (result && result.error) || new Error("Create download task failed");
          }
          return adopt_pending_task(task, created_task_record(result.data));
        })
        .catch((error) => {
          task._fail(error, { creation: true, terminal: true });
          task_list_.as((task_list_.value || []).filter((current) => current !== task));
        });
      return task;
    }

    async function prepare(object) {
      const request_info = resolve_create_request(object);
      const result = await reqs.download.prepare.run(request_info);
      if (!result || result.error) {
        throw (result && result.error) || new Error("Prepare download task failed");
      }
      const previews = result.data && result.data.previews;
      const preview = Array.isArray(previews) ? previews[0] : null;
      if (!preview) {
        throw new Error("Prepare download task returned no preview");
      }
      if (preview.success === false) {
        throw error_value(preview.error, "Prepare download task failed");
      }
      return preview.data || preview;
    }

    async function fetch_task_page(params) {
      const result = await reqs.download.list.run(
        Object.assign({ page: 1, page_size: 100 }, params || {}),
      );
      if (!result || result.error) {
        const error = (result && result.error) || new Error("Load download tasks failed");
        last_error_.as(error);
        throw error;
      }
      const data = result.data || {};
      return {
        records: Array.isArray(data.list) ? data.list : [],
        total: number_value(data.total, 0),
        page: number_value(data.page, 1),
        page_size: number_value(data.page_size, 100),
        stats: data.stats || {},
      };
    }

    function replace_server_tasks(records) {
      const pending = (task_list_.value || []).filter((task) => !task.id.value);
      const next = [];
      const seen = new Set();
      records.forEach((record) => {
        const task = upsert(record, { prepend: false });
        if (task && !seen.has(String(task.id.value))) {
          seen.add(String(task.id.value));
          next.push(task);
        }
      });
      tasks_by_id.forEach((task, key) => {
        if (!seen.has(key)) {
          tasks_by_id.delete(key);
          if (!pending.includes(task)) {
            task._dispose();
          }
        }
      });
      task_list_.as([...pending, ...next]);
    }

    async function refresh(options) {
      const params = Object.assign({}, options || {});
      const load_all = params.all !== false;
      delete params.all;
      const page_size = Math.min(100, Math.max(1, number_value(params.page_size, 100)));
      const records = [];
      let page = Math.max(1, number_value(params.page, 1));
      let total = 0;
      do {
        const data = await fetch_task_page(
          Object.assign({}, params, { page, page_size: page_size }),
        );
        records.push(...data.records);
        total = data.total;
        page += 1;
        if (!load_all || data.records.length === 0) {
          break;
        }
      } while (records.length < total);
      replace_server_tasks(records);
      last_error_.as(null);
      return task_list_;
    }

    function action_result(result, fallback) {
      if (!result || result.error) {
        throw (result && result.error) || new Error(fallback);
      }
      const item = result.data && Array.isArray(result.data.results)
        ? result.data.results[0]
        : null;
      if (item && item.success === false) {
        throw error_value(item.error, fallback);
      }
      return item || {};
    }

    async function run_action(name, target, default_status) {
      const id = task_id(target);
      const task = get(id) || (target && target._update ? target : null);
      const previous_status = task ? task.status.value : null;
      const optimistic_status = normalize_status(default_status);
      if (task) {
        task._begin(default_status);
      }
      try {
        const item = action_result(
          await reqs.download[name].run(id),
          `${name} download task failed`,
        );
        let record = Object.assign(
          { id, status: item.status_text || default_status },
          item.task || {},
        );
        if (
          task &&
          task.status.value !== optimistic_status &&
          normalize_status(record.status) === optimistic_status
        ) {
          record = Object.assign({}, record);
          delete record.status;
        }
        return task ? task._update(record) : upsert(record);
      } catch (error) {
        if (task) {
          if (previous_status && task.status.value === optimistic_status) {
            task.status.as(previous_status);
          }
          task._fail(error, {
            creation: false,
            terminal: false,
            preserve_status: true,
          });
        }
        throw error;
      }
    }

    function start(target) {
      return run_action("start", target, "preparing");
    }

    function resume(target) {
      return run_action("resume", target, "preparing");
    }

    function pause(target) {
      return run_action("pause", target, "paused");
    }

    function retry(target) {
      return run_action("retry", target, "preparing");
    }

    async function run_all_action(request_core, options, fallback) {
      const result = await request_core.run(options || {});
      if (!result || result.error) {
        throw (result && result.error) || new Error(fallback);
      }
      await refresh();
      return task_list_;
    }

    function start_all(options) {
      return run_all_action(
        reqs.download.start_all,
        options,
        "Start all download tasks failed",
      );
    }

    function pause_all(options) {
      return run_all_action(
        reqs.download.pause_all,
        options,
        "Pause all download tasks failed",
      );
    }

    async function clear_all(options) {
      const result = await reqs.download.clear.run(options || {});
      if (!result || result.error) {
        throw (result && result.error) || new Error("Clear download tasks failed");
      }
      await refresh();
      return task_list_;
    }

    async function open(target) {
      const task = get(target) || (target && target.raw ? target : null);
      if (!task) {
        throw new Error("Download task is not available");
      }
      const config = global.__d_config || {};
      if (config.remoteServerEnabled || config.inDocker) {
        const url = new URL("/preview", runtime_config_origin());
        url.searchParams.set("id", String(task.id.value));
        return global.open(url.href, "_blank", "noopener");
      }
      const raw = task.raw.value || {};
      // `files` is the canonical REST/WS record and contains Hermes' final
      // output name. `resources` is only a create-response compatibility alias.
      const resources = raw.files || raw.resources || [];
      const resource = Array.isArray(resources) ? resources[0] || {} : {};
      const result = await reqs.file.show.run({
        id: task.id.value,
        path:
          raw.path ||
          raw.download_dir ||
          resource.path ||
          resource.download_dir ||
          task.filepath.value,
        name: raw.filename || resource.filename || resource.name || task.name.value,
      });
      if (!result || result.error) {
        throw (result && result.error) || new Error("Open download file failed");
      }
      return result.data;
    }

    async function delete_task(target, options) {
      const id = task_id(target);
      const task = get(id) || (target && target._update ? target : null);
      const delete_files = !!(
        options && (options.delete_files ?? options.deleteFiles)
      );
      try {
        action_result(
          await reqs.download.delete.run({
            ids: [id],
            delete_files,
          }),
          "Delete download task failed",
        );
        if (task) {
          task._update({ id, status: "deleted" });
        }
        remove_task(id, false);
        return task;
      } catch (error) {
        if (task) {
          task._fail(error, {
            creation: false,
            terminal: false,
            preserve_status: true,
          });
        }
        throw error;
      }
    }

    function handle_snapshot(message) {
      const resources = Array.isArray(message.resources) ? message.resources : [];
      const aggregate = aggregate_resources(resources);
      upsert({
        id: message.task_id,
        status: message.status,
        name: message.name || "",
        resources,
        downloaded: aggregate.downloaded,
        size: aggregate.total,
        speed: aggregate.speed,
      });
    }

    function handle_web_socket_message(message) {
      if (!message || typeof message !== "object") {
        return;
      }
      if (message.type === "task_create" || message.type === "task_upsert") {
        (Array.isArray(message.tasks) ? message.tasks : []).forEach((task) => upsert(task));
        return;
      }
      if (message.type === "task_update") {
        (Array.isArray(message.updates) ? message.updates : []).forEach((task) => upsert(task));
        return;
      }
      if (message.type === "task_delete") {
        (Array.isArray(message.task_ids) ? message.task_ids : []).forEach((id) => {
          const task = get(id);
          if (task) {
            task._update({ id, status: "deleted" });
            remove_task(id, false);
          }
        });
        return;
      }
      if (message.type === "task_snapshot") {
        if (message.task_id !== undefined && message.task_id !== null) {
          handle_snapshot(message);
        }
        return;
      }
      if (message.type === "batch_tasks") {
        (Array.isArray(message.data) ? message.data : []).forEach((task) => upsert(task, { prepend: false }));
        return;
      }
      if (message.type === "event") {
        const data = message.data || {};
        const key = data.Key || data.key || "";
        const task = data.Task || data.task;
        if (key === "delete") {
          const id = (task && (task.id ?? task.task_id)) || data.task_id;
          if (id !== undefined && id !== null && get(id)) {
            remove_task(id, false);
          }
          return;
        }
        if (task) {
          const error = data.Err || data.err;
          upsert(error ? Object.assign({}, task, { error }) : task);
        }
      }
    }

    async function connect() {
      if (destroyed) {
        throw new Error("DL instance has been destroyed");
      }
      const result = await channel.connect();
      if (!result || result.error) {
        const error =
          (result && result.error) || new Error("Download channel connect failed");
        last_error_.as(error);
        throw error;
      }
      return true;
    }

    async function reconnect_channel() {
      if (destroyed) {
        throw new Error("DL instance has been destroyed");
      }
      const result = await channel.reconnect();
      if (!result || result.error) {
        const error =
          (result && result.error) || new Error("Download channel reconnect failed");
        last_error_.as(error);
        throw error;
      }
      return true;
    }

    async function disconnect() {
      const result = await channel.disconnect(1000, "manual disconnect");
      if (!result || result.error) {
        const error =
          (result && result.error) || new Error("Download channel disconnect failed");
        last_error_.as(error);
        throw error;
      }
      return true;
    }

    function ready() {
      if (ready_promise) {
        return ready_promise;
      }
      ready_promise = Promise.allSettled([refresh(), connect()]).then((results) => ({
        tasks: task_list_,
        connected: websocket_connected_.value,
        results,
      }));
      return ready_promise;
    }

    function destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      channel.destroy();
      (task_list_.value || []).forEach((task) => task._dispose());
      tasks_by_id.clear();
      task_list_.as([]);
    }

    if (auto_start_enabled) {
      global.queueMicrotask(() => ready().catch(function () {}));
    }
    return domain;
  }

  /** SDK entry point. */
  function DL(props) {
    return DownloaderModel(props);
  }

  DL.DownloadTaskModel = DownloadTaskModel;
  DL.DownloaderModel = DownloaderModel;
  DL.TaskStatus = task_status;
  DownloaderModel.DownloadTaskModel = DownloadTaskModel;
  DownloaderModel.TaskStatus = task_status;
  global.DownloadTaskModel = DownloadTaskModel;
  global.DownloaderModel = DownloaderModel;
  global.DL = DL;
})(window);

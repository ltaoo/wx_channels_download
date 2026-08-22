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

  if (
    global.DL &&
    global.DownloaderModel &&
    global.DownloadTaskModel &&
    global.ScraperModel &&
    global.ScraperJobModel
  ) {
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

  function join_file_path(directory, name) {
    const normalized_directory = String(directory || "").trim();
    const normalized_name = String(name || "").trim();
    if (!normalized_directory) {
      return normalized_name;
    }
    if (!normalized_name) {
      return normalized_directory;
    }
    const separator =
      normalized_directory.includes("\\") && !normalized_directory.includes("/")
        ? "\\"
        : "/";
    return (
      normalized_directory.replace(/[\\/]+$/, "") +
      separator +
      normalized_name.replace(/^[\\/]+/, "")
    );
  }

  function is_absolute_file_path(value) {
    const path = String(value || "").trim();
    return /^(?:[a-zA-Z]:[\\/]|[\\/]{2}|\/)/.test(path);
  }

  function output_path(file) {
    if (!file || typeof file !== "object") {
      return "";
    }
    const explicit = String(
      file.filepath || file.file_path || file.local_path || "",
    ).trim();
    if (explicit) {
      return explicit;
    }
    const current_output_path = String(file.output_path || "").trim();
    if (is_absolute_file_path(current_output_path)) {
      return current_output_path;
    }
    return join_file_path(
      file.download_dir || file.downloadDir,
      current_output_path || file.name || file.filename,
    );
  }

  function task_files(record) {
    const files = Array.isArray(record && record.files)
      ? record.files
      : Array.isArray(record && record.resources)
        ? record.resources
        : [];
    return files.map((file) =>
      Object.assign({}, file, {
        output_path: output_path(file),
      }),
    );
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
    return output_path(file);
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
    let success_detail = null;
    let success_detail_promise = null;
    let success_detail_generation = 0;
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
      onFailed: on_failed,
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
      set_success_detail,
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
      get files() {
        return task_files(raw_.value);
      },
      ...methods,
      _update: handler.update,
      _mark_ready: handler.mark_ready,
      _fail: handler.fail,
      _begin: handler.begin,
      _dispose: handler.dispose,
      _setSuccessDetail: handler.set_success_detail,
    };

    function on_success(listener) {
      if (typeof listener !== "function") {
        throw new TypeError("event listener must be a function");
      }
      const unsubscribe = events.success.subscribe(listener);
      if (terminal_state === "success") {
        if (success_detail) {
          global.queueMicrotask(() => listener(success_detail));
        } else {
          start_success_detail_load();
        }
      }
      return unsubscribe;
    }

    function reset_success_detail() {
      success_detail_generation += 1;
      success_detail = null;
      success_detail_promise = null;
    }

    function set_success_detail(detail) {
      success_detail = detail && typeof detail === "object" ? detail : null;
      success_detail_promise = null;
      return model;
    }

    function start_success_detail_load() {
      if (disposed || terminal_state !== "success") {
        return;
      }
      if (success_detail) {
        events.success.emit(success_detail);
        return;
      }
      if (success_detail_promise) {
        return;
      }
      const id = id_.value;
      const detail_request = reqs.download && reqs.download.detail;
      if (
        id === undefined ||
        id === null ||
        id === "" ||
        !detail_request ||
        typeof detail_request.run !== "function"
      ) {
        success_detail = raw_.value;
        events.success.emit(success_detail);
        return;
      }

      const generation = success_detail_generation;
      success_detail_promise = detail_request
        .run({ id })
        .then((result) => {
          if (!result || result.error) {
            throw (
              (result && result.error) ||
              new Error("Load completed download task detail failed")
            );
          }
          if (
            disposed ||
            terminal_state !== "success" ||
            generation !== success_detail_generation
          ) {
            return null;
          }
          const detail = result.data || {};
          success_detail = detail;
          success_detail_promise = null;
          update(detail);
          events.success.emit(detail);
          return detail;
        })
        .catch((error) => {
          if (generation === success_detail_generation) {
            success_detail_promise = null;
            events.change.emit({
              task: model,
              type: "detail_error",
              error: error_value(error),
            });
          }
          return null;
        });
    }

    function on_fail(listener) {
      const unsubscribe = events.fail.subscribe(listener);
      if (terminal_state === "fail" && last_failure) {
        global.queueMicrotask(() => listener({ error: last_failure, task: model }));
      }
      return unsubscribe;
    }

    function on_failed(listener) {
      if (typeof listener !== "function") {
        throw new TypeError("event listener must be a function");
      }
      return on_fail((event) => listener(event.error));
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
        files: task_files(raw_.value),
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
      reset_success_detail();
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
        reset_success_detail();
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
        // A completed task restored from the list already has enough data for
        // the table. Hydrating every restored task here runs the shared detail
        // RequestCore concurrently, which coalesces calls and can apply one
        // task's detail response to the other task models. A late onSuccess
        // subscriber still loads the detail on demand.
        if (!success_statuses.has(previous_status)) {
          start_success_detail_load();
        }
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

  function create_request_object(input, options) {
    const create_options =
      options && typeof options === "object" ? Object.assign({}, options) : null;
    let object;
    if (create_options && create_options.platform) {
      object = {
        platform: create_options.platform,
        content: input,
      };
    } else {
      object =
        typeof input === "string" ? { url: input } : Object.assign({}, input || {});
    }
    if (!create_options) {
      return object;
    }

    [
      "build_from_fetch",
      "resource_indexes",
      "download_dir",
      "filename",
      "auto_start",
      "parent_task_id",
      "relation_type",
    ].forEach((key) => {
      if (Object.prototype.hasOwnProperty.call(create_options, key)) {
        object[key] = create_options[key];
      }
    });

    const config = Object.assign({}, object.config || {}, create_options.config || {});
    if (typeof create_options.existing_action === "string") {
      config.existing_action = create_options.existing_action;
    }
    if (create_options.skip === true) {
      config.existing_action = "skip";
    }
    if (Object.keys(config).length > 0) {
      object.config = config;
    }
    return object;
  }

  function resolve_create_request(input, options) {
    const object = create_request_object(input, options);
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

  function task_was_skipped(record) {
    return !!record && (record.skipped === true || record.action === "skip");
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

  function get_download_task_detail(params) {
    return request.get("/api/v1/download_task/detail", {
      id: params && (params.id ?? params.task_id),
    });
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
   * const task$ = await dl$.create(feed, {
   *   platform: "wxchannels",
   *   skip: true,
   * });
   * task$.onSuccess((detail) => console.log(detail.files[0].local_path));
   * task$.onFailed((error) => console.error(error));
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
        detail: new RequestCore(get_download_task_detail, { client: http_client }),
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
    const list_meta_ = timeless.refobj({
      total: 0,
      page: 1,
      page_size: 100,
      stats: {},
    });
    const websocket_connected_ = timeless.ref(false);
    const websocket_connecting_ = timeless.ref(false);
    const last_error_ = timeless.ref(null);
    const tasks_by_id = new Map();
    const socket_status_by_id = new Map();
    const websocket_url = "/ws/v1/download_task";
    const reconnect_interval = Math.max(
      250,
      number_value(reconnect_interval_value, 5000),
    );
    let destroyed = false;
    let ready_promise = null;
    let refresh_sequence = 0;
    let current_refresh_options = null;
    let paged_refresh_timer = null;

    const state = {
      task_list: task_list_,
      tasks: task_list_,
      list_meta: list_meta_,
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
      loadPage: load_task_page,
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
      load_task_page,
      refresh_current,
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
        refresh_current().catch((error) => last_error_.as(error));
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

    function adopt_pending_task(task, record, success_detail) {
      const id = record && (record.id ?? record.task_id);
      if (id === undefined || id === null || id === "") {
        throw new Error("Created download task has no id");
      }
      const key = String(id);
      const existing = tasks_by_id.get(key);
      if (success_detail) {
        task._setSuccessDetail(success_detail);
      }
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

    async function create(object, options) {
      const request_info = resolve_create_request(object, options);
      const task = DownloadTaskModel({
        owner: domain,
        pending: true,
        record: initial_create_record(request_info),
      });
      append_task(task, true);
      try {
        const result = await reqs.download.create.run(request_info);
        if (!result || result.error) {
          throw (result && result.error) || new Error("Create download task failed");
        }
        let record = created_task_record(result.data);
        let success_detail = null;
        if (task_was_skipped(record)) {
          const detail_result = await reqs.download.detail.run({
            id: record.id ?? record.task_id,
          });
          if (!detail_result || detail_result.error) {
            throw (
              (detail_result && detail_result.error) ||
              new Error("Load skipped download task failed")
            );
          }
          success_detail = detail_result.data || {};
          record = Object.assign({}, record, success_detail);
        }
        return adopt_pending_task(task, record, success_detail);
      } catch (error) {
        task._fail(error, { creation: true, terminal: true });
        task_list_.as((task_list_.value || []).filter((current) => current !== task));
        throw error;
      }
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
      if (paged_refresh_timer !== null) {
        global.clearTimeout(paged_refresh_timer);
        paged_refresh_timer = null;
      }
      const normalized_options = Object.assign({}, options || {});
      normalized_options.all = normalized_options.all !== false;
      normalized_options.page = Math.max(
        1,
        number_value(normalized_options.page, 1),
      );
      normalized_options.page_size = Math.min(
        100,
        Math.max(1, number_value(normalized_options.page_size, 100)),
      );
      current_refresh_options = normalized_options;

      const sequence = ++refresh_sequence;
      const params = Object.assign({}, normalized_options);
      const load_all = params.all;
      delete params.all;
      const page_size = params.page_size;
      const records = [];
      const requested_page = params.page;
      let page = requested_page;
      let total = 0;
      let response_meta = {
        total: 0,
        page: requested_page,
        page_size,
        stats: {},
      };
      do {
        const data = await fetch_task_page(
          Object.assign({}, params, { page, page_size: page_size }),
        );
        records.push(...data.records);
        total = data.total;
        response_meta = data;
        page += 1;
        if (!load_all || data.records.length === 0) {
          break;
        }
      } while (records.length < total);
      if (sequence !== refresh_sequence || destroyed) {
        return task_list_;
      }
      replace_server_tasks(records);
      list_meta_.as({
        total: response_meta.total,
        page: load_all ? requested_page : response_meta.page,
        page_size: response_meta.page_size,
        stats: response_meta.stats || {},
      });
      last_error_.as(null);
      return task_list_;
    }

    function refresh_current() {
      return refresh(Object.assign({}, current_refresh_options || {}));
    }

    async function load_task_page(options) {
      await refresh(Object.assign({}, options || {}, { all: false }));
      return Object.assign({}, list_meta_.value || {});
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
      await refresh_current();
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
      await refresh_current();
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

    function paged_mode_enabled() {
      return !!current_refresh_options && current_refresh_options.all === false;
    }

    function update_list_stats(stats) {
      if (!stats || typeof stats !== "object") {
        return;
      }
      list_meta_.as(
        Object.assign({}, list_meta_.value || {}, {
          stats,
        }),
      );
    }

    function schedule_paged_refresh() {
      if (!paged_mode_enabled() || destroyed) {
        return;
      }
      if (paged_refresh_timer !== null) {
        global.clearTimeout(paged_refresh_timer);
      }
      paged_refresh_timer = global.setTimeout(() => {
        paged_refresh_timer = null;
        refresh_current().catch((error) => last_error_.as(error));
      }, 80);
    }

    function socket_status_matches_current_filter(status) {
      const filter = String(
        (current_refresh_options && current_refresh_options.status) || "",
      );
      if (!filter || !status) {
        return false;
      }
      const status_code = {
        waiting: "0",
        preparing: "1",
        downloading: "2",
        paused: "3",
        merging: "4",
        finished: "5",
        failed: "6",
        cancelled: "7",
      }[status];
      return !!status_code && filter.split(",").includes(status_code);
    }

    function upsert_socket_task(record, options) {
      if (!paged_mode_enabled()) {
        return upsert(record, options);
      }
      const id = record && (record.id ?? record.task_id);
      const key = id === undefined || id === null ? "" : String(id);
      const previous_socket_status = socket_status_by_id.get(key);
      const socket_status = Object.prototype.hasOwnProperty.call(
        record || {},
        "status",
      )
        ? normalize_status(record.status)
        : previous_socket_status;
      if (key && socket_status) {
        socket_status_by_id.set(key, socket_status);
      }
      const task = id === undefined || id === null ? null : get(id);
      if (!task) {
        const should_refresh_missing =
          !options ||
          options.refresh_missing === true ||
          typeof options.refresh_missing === "undefined" ||
          (options.refresh_missing === "status" &&
            previous_socket_status !== socket_status &&
            (socket_status_matches_current_filter(previous_socket_status) ||
              socket_status_matches_current_filter(socket_status)));
        if (should_refresh_missing) {
          schedule_paged_refresh();
        }
        return null;
      }
      const previous_status = task.status.value;
      task._update(record, options);
      if (
        (!options || options.refresh_status !== false) &&
        previous_status !== task.status.value
      ) {
        schedule_paged_refresh();
      }
      return task;
    }

    function handle_snapshot(message) {
      const resources = Array.isArray(message.resources) ? message.resources : [];
      const aggregate = aggregate_resources(resources);
      upsert_socket_task(
        {
          id: message.task_id,
          status: message.status,
          name: message.name || "",
          resources,
          downloaded: aggregate.downloaded,
          size: aggregate.total,
          speed: aggregate.speed,
        },
        { refresh_missing: false },
      );
    }

    function handle_web_socket_message(message) {
      if (!message || typeof message !== "object") {
        return;
      }
      if (message.type === "task_stats") {
        update_list_stats(message.stats);
        return;
      }
      if (message.type === "task_create" || message.type === "task_upsert") {
        (Array.isArray(message.tasks) ? message.tasks : []).forEach((task) => {
          upsert_socket_task(task, {
            refresh_missing:
              message.type === "task_create" ? false : "status",
          });
        });
        if (message.type === "task_create") {
          schedule_paged_refresh();
        }
        return;
      }
      if (message.type === "task_update") {
        (Array.isArray(message.updates) ? message.updates : []).forEach((task) => {
          upsert_socket_task(task, { refresh_missing: "status" });
        });
        return;
      }
      if (message.type === "task_delete") {
        (Array.isArray(message.task_ids) ? message.task_ids : []).forEach((id) => {
          socket_status_by_id.delete(String(id));
          const task = get(id);
          if (task) {
            task._update({ id, status: "deleted" });
            remove_task(id, false);
          }
        });
        schedule_paged_refresh();
        return;
      }
      if (message.type === "task_snapshot") {
        if (message.task_id !== undefined && message.task_id !== null) {
          handle_snapshot(message);
        }
        return;
      }
      if (message.type === "batch_tasks") {
        (Array.isArray(message.data) ? message.data : []).forEach((task) => {
          upsert_socket_task(task, {
            prepend: false,
            refresh_missing: false,
          });
        });
        return;
      }
      if (message.type === "event") {
        const data = message.data || {};
        const key = data.Key || data.key || "";
        const task = data.Task || data.task;
        if (key === "delete") {
          const id = (task && (task.id ?? task.task_id)) || data.task_id;
          if (id !== undefined && id !== null) {
            socket_status_by_id.delete(String(id));
          }
          if (id !== undefined && id !== null && get(id)) {
            remove_task(id, false);
          }
          schedule_paged_refresh();
          return;
        }
        if (task) {
          const error = data.Err || data.err;
          upsert_socket_task(error ? Object.assign({}, task, { error }) : task);
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

    function ready(options) {
      if (ready_promise) {
        if (!options) {
          return ready_promise;
        }
        return ready_promise.then(() => refresh(options)).then(() => ({
          tasks: task_list_,
          connected: websocket_connected_.value,
          results: [],
        }));
      }
      ready_promise = Promise.allSettled([refresh(options), connect()]).then(
        (results) => ({
          tasks: task_list_,
          connected: websocket_connected_.value,
          results,
        }),
      );
      return ready_promise;
    }

    function destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      if (paged_refresh_timer !== null) {
        global.clearTimeout(paged_refresh_timer);
        paged_refresh_timer = null;
      }
      channel.destroy();
      (task_list_.value || []).forEach((task) => task._dispose());
      tasks_by_id.clear();
      socket_status_by_id.clear();
      task_list_.as([]);
    }

    if (auto_start_enabled) {
      global.queueMicrotask(() => ready().catch(function () {}));
    }
    return domain;
  }

  const scraper_terminal_statuses = new Set([
    "completed",
    "failed",
    "interrupted",
  ]);

  function scraper_result_lower_camel_case(value) {
    const key = String(value || "");
    if (!/^[A-Z]/.test(key)) {
      return key;
    }
    let uppercase_end = 0;
    while (uppercase_end < key.length && /[A-Z]/.test(key[uppercase_end])) {
      uppercase_end += 1;
    }
    if (uppercase_end > 1 && uppercase_end < key.length) {
      uppercase_end -= 1;
    }
    return key.slice(0, uppercase_end).toLowerCase() + key.slice(uppercase_end);
  }

  function normalize_scraper_result_keys(value) {
    if (Array.isArray(value)) {
      let normalized = value;
      value.forEach((item, index) => {
        const normalized_item = normalize_scraper_result_keys(item);
        if (normalized_item !== item) {
          if (normalized === value) normalized = value.slice();
          normalized[index] = normalized_item;
        }
      });
      return normalized;
    }
    if (!value || typeof value !== "object") {
      return value;
    }

    let normalized = value;
    Object.keys(value).forEach((key) => {
      const normalized_key = scraper_result_lower_camel_case(key);
      const normalized_item = normalize_scraper_result_keys(value[key]);
      if (normalized_key === key && normalized_item === value[key]) {
        return;
      }
      if (normalized === value) normalized = Object.assign({}, value);
      if (normalized_key !== key) {
        delete normalized[key];
      }
      if (
        normalized_key === key ||
        !Object.prototype.hasOwnProperty.call(value, normalized_key)
      ) {
        normalized[normalized_key] = normalized_item;
      }
    });
    return normalized;
  }

  function normalize_scraper_job_output(output) {
    if (
      !output ||
      typeof output !== "object" ||
      !Object.prototype.hasOwnProperty.call(output, "result")
    ) {
      return output;
    }
    const normalized_result = normalize_scraper_result_keys(output.result);
    return normalized_result === output.result
      ? output
      : Object.assign({}, output, { result: normalized_result });
  }

  function scraper_item_key(item) {
    if (!item || typeof item !== "object") {
      return "";
    }
    return String(item.key || item.type || item.path || item.id || "").trim();
  }

  function merge_scraper_items(current, updates) {
    const merged = Array.isArray(current) ? current.slice() : [];
    if (!Array.isArray(updates)) {
      return merged;
    }
    updates.forEach((item) => {
      const key = scraper_item_key(item);
      const index = key
        ? merged.findIndex((current_item) => scraper_item_key(current_item) === key)
        : -1;
      if (index >= 0) {
        merged[index] = item;
      } else {
        merged.push(item);
      }
    });
    return merged;
  }

  function scraper_output(current, record, event) {
    const job = record && typeof record === "object" ? record : {};
    const final_output = normalize_scraper_job_output(
      job.output && typeof job.output === "object" ? job.output : {},
    );
    const next = Object.assign({}, current || {}, final_output);
    const job_id = job.id || final_output.job_id;
    if (job_id) {
      next.job_id = job_id;
    }
    if (job.platform || final_output.platform) {
      next.platform = job.platform || final_output.platform;
    }
    if (job.url || final_output.url) {
      next.url = job.url || final_output.url;
    }

    const raw_sources = [job, final_output, event];
    raw_sources.forEach((source) => {
      if (
        source &&
        Object.prototype.hasOwnProperty.call(source, "raw_result")
      ) {
        next.raw_result = source.raw_result;
      }
    });

    next.content =
      (event && event.content) ||
      final_output.content ||
      job.content ||
      next.content ||
      null;
    next.account =
      (event && event.account) ||
      final_output.account ||
      job.account ||
      next.account ||
      null;
    next.content_details = merge_scraper_items(
      merge_scraper_items(
        merge_scraper_items(next.content_details, job.content_details),
        final_output.content_details,
      ),
      event && event.content_detail ? [event.content_detail] : [],
    );
    next.cache_entries = merge_scraper_items(
      merge_scraper_items(
        merge_scraper_items(next.cache_entries, job.cache_entries),
        final_output.cache_entries,
      ),
      event && event.cache_entry ? [event.cache_entry] : [],
    );
    if (final_output.download_info) {
      next.download_info = final_output.download_info;
    }
    if (Object.prototype.hasOwnProperty.call(final_output, "result")) {
      next.result = final_output.result;
    }
    return next;
  }

  function scraper_requests(client) {
    return {
      create: new RequestCore(
        (body) => request.post("/api/scraper/fetch", body),
        { client },
      ),
      detail: new RequestCore(
        (params) => request.get("/api/scraper/job", params),
        { client },
      ),
      interrupt: new RequestCore(
        (body) => request.post("/api/scraper/fetch/interrupt", body),
        { client },
      ),
    };
  }

  /**
   * A stable domain object for one asynchronous scraper job.
   *
   * `onMessage` receives the `/ws/scraper` envelope. `onComplete` receives
   * the final `/api/scraper/job` output and this model as its second argument.
   */
  function ScraperJobModel(props) {
    const options = props || {};
    const initial = options.record || {};
    const owner = options.owner || null;
    const shared_channel = options.channel || null;
    const client = options.client;
    const socket_client =
      options.socket_client ||
      new SocketClientCore();
    const reqs = options.requests || scraper_requests(client);
    const poll_interval = Math.max(
      100,
      number_value(options.poll_interval, 1000),
    );
    const reconnect_interval = Math.max(
      250,
      number_value(options.reconnect_interval, 1000),
    );
    const websocket_url = options.ws_url || "/ws/scraper";

    const id_ = timeless.ref(String(initial.id || "").trim());
    const url_ = timeless.ref(String(initial.url || options.url || "").trim());
    const platform_ = timeless.ref(String(initial.platform || "").trim());
    const status_ = timeless.ref(String(initial.status || "pending").trim());
    const progress_ = timeless.refobj(initial.progress || {});
    const output_ = timeless.ref(null);
    const content_ = timeless.ref(initial.content || null);
    const account_ = timeless.ref(initial.account || null);
    const content_details_ = timeless.refarr([]);
    const cache_entries_ = timeless.refarr([]);
    const raw_ = timeless.refobj({});
    const message_ = timeless.ref(null);
    const error_ = timeless.ref(null);
    const connected_ = timeless.ref(false);
    const events = {
      change: event_channel(),
      complete: event_channel(),
      fail: event_channel(),
      interrupted: event_channel(),
      message: event_channel(),
      progress: event_channel(),
    };
    const finished_state = deferred();
    let poll_timer = null;
    let resolving_terminal = false;
    let terminal_state = null;
    let terminal_error = null;
    let disposed = false;

    const state = {
      id: id_,
      url: url_,
      platform: platform_,
      status: status_,
      progress: progress_,
      output: output_,
      result: output_,
      content: content_,
      account: account_,
      content_details: content_details_,
      cache_entries: cache_entries_,
      raw: raw_,
      message: message_,
      error: error_,
      connected: connected_,
      websocket_connected: connected_,
    };
    const methods = {
      onMessage: on_message,
      onComplete: on_complete,
      onSuccess: on_complete,
      onFail: on_fail,
      onFailed: on_fail,
      onInterrupted: on_interrupted,
      onProgress: on_progress,
      onChange: on_change,
      refresh,
      detail: refresh,
      interrupt,
      connect,
      disconnect,
      wait,
      snapshot,
      destroy,
    };
    const channel =
      shared_channel ||
      new ChannelCore(websocket_url, {
        client: socket_client,
        process: decode_message,
        reconnect: {
          enabled: options.reconnect !== false,
          interval: reconnect_interval,
        },
      });
    const model = {
      state,
      methods,
      reqs,
      channel,
      ...state,
      ...methods,
      get finished() {
        return finished_state.promise;
      },
      get completed() {
        return finished_state.promise;
      },
      _handleMessage: handle_message,
      _update: apply_record,
      _start: start_tracking,
      _syncChannelState: sync_channel_state,
      _dispose: dispose,
    };

    if (!shared_channel) {
      channel.onMessage(handle_message);
      channel.onStateChange(sync_channel_state);
      if (typeof channel.onReconnected === "function") {
        channel.onReconnected(() => {
          refresh().catch(() => {});
        });
      }
    }

    function subscribe_with(channel_event, listener, transform, replay) {
      if (typeof listener !== "function") {
        throw new TypeError("event listener must be a function");
      }
      const wrapped = (payload) => transform(listener, payload);
      const unsubscribe = channel_event.subscribe(wrapped);
      if (replay) {
        global.queueMicrotask(() => wrapped(replay));
      }
      return unsubscribe;
    }

    function on_message(listener) {
      return subscribe_with(
        events.message,
        listener,
        (callback, payload) => callback(payload, model),
        message_.value,
      );
    }

    function on_complete(listener) {
      return subscribe_with(
        events.complete,
        listener,
        (callback, payload) => callback(payload.output, model),
        terminal_state === "completed"
          ? { output: output_.value, job: model }
          : null,
      );
    }

    function on_fail(listener) {
      return subscribe_with(
        events.fail,
        listener,
        (callback, payload) => callback(payload.error, model),
        terminal_state === "failed" || terminal_state === "interrupted"
          ? { error: terminal_error, job: model }
          : null,
      );
    }

    function on_interrupted(listener) {
      return subscribe_with(
        events.interrupted,
        listener,
        (callback, payload) => callback(payload.error, model),
        terminal_state === "interrupted"
          ? { error: terminal_error, job: model }
          : null,
      );
    }

    function on_progress(listener) {
      return subscribe_with(
        events.progress,
        listener,
        (callback, payload) => callback(payload, model),
        null,
      );
    }

    function on_change(listener) {
      return subscribe_with(
        events.change,
        listener,
        (callback, payload) => callback(payload, model),
        null,
      );
    }

    function decode_message(value) {
      if (typeof value !== "string") {
        return value;
      }
      try {
        return JSON.parse(value);
      } catch {
        return null;
      }
    }

    function synthetic_message(record, event, source) {
      return {
        type: "scraper_job",
        job: record,
        event: event || null,
        source,
      };
    }

    function handle_message(message) {
      if (!message || message.type !== "scraper_job" || !message.job) {
        return;
      }
      const message_id = String(message.job.id || "").trim();
      if (message_id && id_.value && message_id !== id_.value) {
        return;
      }
      apply_record(
        message.job,
        message.event || null,
        "websocket",
        message,
      );
    }

    function apply_record(record, event, source, incoming_message) {
      if (
        disposed ||
        terminal_state ||
        !record ||
        typeof record !== "object"
      ) {
        return model;
      }
      const record_id = String(record.id || "").trim();
      if (record_id && id_.value && record_id !== id_.value) {
        return model;
      }
      if (record_id) {
        id_.as(record_id);
      }
      if (record.url) {
        url_.as(String(record.url));
      }
      if (record.platform) {
        platform_.as(String(record.platform));
      }

      const previous_raw = raw_.value || {};
      const next_raw = Object.assign({}, previous_raw, record);
      next_raw.content_details = merge_scraper_items(
        previous_raw.content_details,
        record.content_details,
      );
      next_raw.cache_entries = merge_scraper_items(
        previous_raw.cache_entries,
        record.cache_entries,
      );
      raw_.as(next_raw);

      const next_output = scraper_output(output_.value, next_raw, event);
      output_.as(next_output);
      content_.as(next_output.content || null);
      account_.as(next_output.account || null);
      content_details_.as(next_output.content_details || []);
      cache_entries_.as(next_output.cache_entries || []);

      if (record.progress || (event && event.progress)) {
        const progress = (event && event.progress) || record.progress;
        progress_.as(progress);
        events.progress.emit(progress);
      }
      const next_status = String(
        (event && event.status) || record.status || status_.value,
      ).trim();
      if (!terminal_state && next_status) {
        status_.as(next_status);
      }

      const message =
        incoming_message || synthetic_message(record, event, source || "update");
      message_.as(message);
      events.message.emit(message);
      events.change.emit({
        job: model,
        record: next_raw,
        event: event || null,
        source: source || "update",
      });

      if (scraper_terminal_statuses.has(next_status) && !terminal_state) {
        if (source === "detail" || source === "poll") {
          finish(record);
        } else {
          resolve_terminal();
        }
      }
      return model;
    }

    function clear_poll() {
      if (poll_timer !== null) {
        global.clearTimeout(poll_timer);
        poll_timer = null;
      }
    }

    function schedule_poll(delay) {
      clear_poll();
      if (disposed || terminal_state || !id_.value) {
        return;
      }
      poll_timer = global.setTimeout(async () => {
        poll_timer = null;
        try {
          await refresh();
        } catch {
          // The WebSocket may still finish the job; polling will retry below.
        }
        if (!disposed && !terminal_state) {
          schedule_poll(poll_interval);
        }
      }, delay === undefined ? poll_interval : delay);
    }

    async function resolve_terminal() {
      if (disposed || terminal_state || resolving_terminal || !id_.value) {
        return;
      }
      resolving_terminal = true;
      let result;
      try {
        result = await reqs.detail.run({ id: id_.value });
      } catch (error) {
        result = { error };
      }
      resolving_terminal = false;
      if (disposed || terminal_state) {
        return;
      }
      if (!result || result.error) {
        error_.as(
          error_value(
            result && result.error,
            "Load final scraper job detail failed",
          ),
        );
        schedule_poll(500);
        return;
      }
      const final_record = result.data || {};
      apply_record(
        final_record,
        null,
        "detail",
        synthetic_message(final_record, null, "detail"),
      );
      if (!scraper_terminal_statuses.has(String(final_record.status || ""))) {
        schedule_poll(poll_interval);
      }
    }

    function finish(record) {
      if (terminal_state || disposed) {
        return;
      }
      clear_poll();
      const status = String(record.status || status_.value).trim();
      status_.as(status);
      if (status === "completed") {
        if (!record.output || typeof record.output !== "object") {
          fail_terminal(
            error_value("Scraper job completed without a result"),
            "failed",
          );
          return;
        }
        // Match the scraper page contract: once the detail request completes,
        // expose the server's canonical final output instead of the incremental
        // working copy assembled from WebSocket artifacts.
        const final_output = normalize_scraper_job_output(record.output);
        output_.as(final_output);
        content_.as(final_output.content || null);
        account_.as(final_output.account || null);
        content_details_.as(final_output.content_details || []);
        cache_entries_.as(final_output.cache_entries || []);
        terminal_state = "completed";
        error_.as(null);
        if (!finished_state.settled) {
          finished_state.settled = true;
          finished_state.resolve(output_.value);
        }
        events.complete.emit({ output: output_.value, job: model });
        return;
      }
      fail_terminal(
        error_value(
          record.error ||
            (status === "interrupted"
              ? "Scraper job interrupted"
              : "Scraper job failed"),
        ),
        status === "interrupted" ? "interrupted" : "failed",
      );
    }

    function fail_terminal(error, status) {
      terminal_state = status;
      terminal_error = error;
      error_.as(error);
      status_.as(status);
      if (!finished_state.settled) {
        finished_state.settled = true;
        finished_state.reject(error);
      }
      const payload = { error, job: model };
      events.fail.emit(payload);
      if (status === "interrupted") {
        events.interrupted.emit(payload);
      }
    }

    async function refresh() {
      if (disposed) {
        throw new Error("Scraper job has been destroyed");
      }
      if (!id_.value) {
        throw new Error("Scraper job id is required");
      }
      const result = await reqs.detail.run({ id: id_.value });
      if (!result || result.error) {
        const error = error_value(
          (result && result.error) || new Error("Load scraper job failed"),
        );
        error_.as(error);
        throw error;
      }
      apply_record(
        result.data || {},
        null,
        "poll",
        synthetic_message(result.data || {}, null, "poll"),
      );
      return model;
    }

    async function interrupt() {
      if (disposed) {
        throw new Error("Scraper job has been destroyed");
      }
      const result = await reqs.interrupt.run({ id: id_.value });
      if (!result || result.error) {
        throw error_value(
          result && result.error,
          "Interrupt scraper job failed",
        );
      }
      await refresh();
      return model;
    }

    async function connect() {
      if (disposed) {
        throw new Error("Scraper job has been destroyed");
      }
      if (owner && typeof owner.connect === "function") {
        return owner.connect();
      }
      const result = await channel.connect();
      if (!result || result.error) {
        throw (
          (result && result.error) ||
          new Error("Scraper progress channel connection failed")
        );
      }
      return true;
    }

    async function disconnect() {
      if (owner && typeof owner.disconnect === "function") {
        return owner.disconnect();
      }
      const result = await channel.disconnect(1000, "manual disconnect");
      if (!result || result.error) {
        throw (
          (result && result.error) ||
          new Error("Scraper progress channel disconnect failed")
        );
      }
      connected_.as(false);
      return true;
    }

    function sync_channel_state(channel_state) {
      const connected = !!(channel_state && channel_state.connected);
      connected_.as(connected);
      if (!terminal_state) {
        schedule_poll(connected ? poll_interval : 250);
      }
    }

    function start_tracking() {
      if (
        disposed ||
        terminal_state ||
        scraper_terminal_statuses.has(status_.value)
      ) {
        return Promise.resolve(model);
      }
      const tracking = [refresh()];
      if (!shared_channel && options.websocket !== false) {
        tracking.push(connect());
      }
      schedule_poll(poll_interval);
      return Promise.allSettled(tracking).then(() => model);
    }

    function wait() {
      return finished_state.promise;
    }

    function snapshot() {
      return {
        id: id_.value,
        url: url_.value,
        platform: platform_.value,
        status: status_.value,
        progress: progress_.value,
        output: output_.value,
        content: content_.value,
        account: account_.value,
        content_details: content_details_.value,
        cache_entries: cache_entries_.value,
        error: error_.value,
        connected: connected_.value,
        raw: raw_.value,
      };
    }

    function destroy() {
      if (owner && typeof owner._remove === "function") {
        owner._remove(model, true);
        return;
      }
      dispose();
    }

    function dispose() {
      if (disposed) {
        return;
      }
      disposed = true;
      clear_poll();
      if (!shared_channel) {
        channel.destroy();
      }
      if (!finished_state.settled) {
        const error = new Error("Scraper job destroyed before completion");
        error.name = "AbortError";
        finished_state.settled = true;
        finished_state.reject(error);
      }
      Object.values(events).forEach((event) => event.clear());
    }

    apply_record(
      initial,
      null,
      "create",
      synthetic_message(initial, null, "create"),
    );
    return model;
  }

  /**
   * Scraper job manager. Like DownloaderModel, it owns one shared WebSocket
   * channel and requires its transport clients to be injected explicitly.
   *
   * @example
   * const scraper$ = ScraperModel({
   *   client: http_client$,
   *   socket_client: socket_client$,
   *   build_from_fetch: true,
   * });
   * const job$ = await scraper$.create("https://example.com/post");
   * job$.onMessage((message) => console.log(message.event));
   * job$.onComplete((result) => console.log(result.content_details));
   */
  function ScraperModel(props) {
    const model_options = props || {};
    const {
      client: http_client,
      socket_client,
      auto_start = true,
      reconnect = true,
      reconnect_interval: reconnect_interval_value = 1000,
      poll_interval: poll_interval_value = 1000,
      ws_url = "/ws/scraper",
    } = model_options;
    const create_defaults = {};
    if (
      Object.prototype.hasOwnProperty.call(model_options, "build_from_fetch")
    ) {
      create_defaults.build_from_fetch = model_options.build_from_fetch === true;
    }
    if (!http_client) {
      throw new TypeError("ScraperModel requires a client");
    }
    if (!socket_client) {
      throw new TypeError("ScraperModel requires a socket_client");
    }

    const reqs = scraper_requests(http_client);
    const jobs_ = timeless.refarr([]);
    const websocket_connected_ = timeless.ref(false);
    const websocket_connecting_ = timeless.ref(false);
    const last_error_ = timeless.ref(null);
    const messages = event_channel();
    const jobs_by_id = new Map();
    const reconnect_interval = Math.max(
      250,
      number_value(reconnect_interval_value, 1000),
    );
    const poll_interval = Math.max(
      100,
      number_value(poll_interval_value, 1000),
    );
    let destroyed = false;
    let ready_promise = null;

    const state = {
      jobs: jobs_,
      job_list: jobs_,
      websocket_connected: websocket_connected_,
      websocket_connecting: websocket_connecting_,
      last_error: last_error_,
      websocket_url: ws_url,
    };
    const methods = {
      create,
      get,
      onMessage: on_message,
      connect,
      reconnect: reconnect_channel,
      disconnect,
      ready,
      destroy,
    };
    const channel = new ChannelCore(ws_url, {
      client: socket_client,
      process: decode_message,
      reconnect: {
        enabled: reconnect !== false,
        interval: reconnect_interval,
      },
    });
    const domain = {
      state,
      methods,
      reqs,
      channel,
      ...state,
      ...methods,
      _remove: remove_job,
    };

    channel.onMessage(handle_message);
    channel.onStateChange(sync_channel_state);
    if (typeof channel.onReconnected === "function") {
      channel.onReconnected(refresh_jobs);
    }

    function decode_message(value) {
      if (typeof value !== "string") {
        return value;
      }
      try {
        return JSON.parse(value);
      } catch {
        return null;
      }
    }

    function job_id(target) {
      const value =
        target && typeof target === "object"
          ? target.id && typeof target.id === "object" && "value" in target.id
            ? target.id.value
            : target.id
          : target;
      return String(value === undefined || value === null ? "" : value).trim();
    }

    function get(target) {
      const id = job_id(target);
      return id ? jobs_by_id.get(id) || null : null;
    }

    function append_job(job) {
      jobs_.as([job, ...(jobs_.value || [])]);
    }

    function remove_job(target, dispose) {
      const id = job_id(target);
      const job = get(id) || (target && target._dispose ? target : null);
      if (!job) {
        return null;
      }
      if (id) {
        jobs_by_id.delete(id);
      }
      jobs_.as((jobs_.value || []).filter((current) => current !== job));
      if (dispose !== false) {
        job._dispose();
      }
      return job;
    }

    function on_message(listener) {
      return messages.subscribe(listener);
    }

    function handle_message(message) {
      if (!message || typeof message !== "object") {
        return;
      }
      messages.emit(message);
      if (message.type !== "scraper_job" || !message.job) {
        return;
      }
      const job = get(message.job.id);
      if (job) {
        job._handleMessage(message);
      }
    }

    function sync_channel_state(channel_state) {
      const connected = !!(channel_state && channel_state.connected);
      const connecting = !!(channel_state && channel_state.connecting);
      websocket_connected_.as(connected);
      websocket_connecting_.as(connecting);
      if (channel_state && channel_state.error) {
        last_error_.as(error_value(channel_state.error));
      } else if (connected) {
        last_error_.as(null);
      }
      (jobs_.value || []).forEach((job) => {
        job._syncChannelState(channel_state || {});
      });
    }

    function refresh_jobs() {
      return Promise.allSettled(
        (jobs_.value || []).map((job) => job.refresh()),
      );
    }

    async function create(input, options) {
      if (destroyed) {
        throw new Error("ScraperModel has been destroyed");
      }
      const input_options =
        input && typeof input === "object" && !Array.isArray(input)
          ? Object.assign({}, input)
          : { url: input };
      const config = Object.assign(
        {},
        create_defaults,
        input_options,
        options || {},
      );
      const url = String(config.url || "").trim();
      if (!url) {
        throw new TypeError("ScraperModel.create requires a URL");
      }
      const body = {
        url,
        force_refresh: config.force_refresh === true,
      };
      if (Object.prototype.hasOwnProperty.call(config, "build_from_fetch")) {
        body.build_from_fetch = config.build_from_fetch === true;
      }
      const request_id = String(config.id || config.request_id || "").trim();
      if (request_id) {
        body.id = request_id;
      }
      const result = await reqs.create.run(body);
      if (!result || result.error) {
        throw error_value(result && result.error, "Create scraper job failed");
      }
      const record = result.data || {};
      const id = String(record.id || "").trim();
      if (!id) {
        throw new Error("Create scraper job response has no id");
      }

      const existing = jobs_by_id.get(id);
      if (existing) {
        existing._update(
          record,
          null,
          "create",
          { type: "scraper_job", job: record, event: null, source: "create" },
        );
        existing._start().catch(() => {});
        return existing;
      }

      const job = ScraperJobModel(
        Object.assign({}, config, {
          owner: domain,
          channel,
          client: http_client,
          socket_client,
          requests: reqs,
          record,
          url,
          poll_interval,
          reconnect_interval,
        }),
      );
      jobs_by_id.set(id, job);
      append_job(job);
      job._syncChannelState({
        connected: websocket_connected_.value,
        connecting: websocket_connecting_.value,
      });
      job._start().catch(() => {});
      return job;
    }

    async function connect() {
      if (destroyed) {
        throw new Error("ScraperModel has been destroyed");
      }
      const result = await channel.connect();
      if (!result || result.error) {
        const error = error_value(
          result && result.error,
          "Scraper progress channel connection failed",
        );
        last_error_.as(error);
        throw error;
      }
      return true;
    }

    async function reconnect_channel() {
      if (destroyed) {
        throw new Error("ScraperModel has been destroyed");
      }
      const result = await channel.reconnect();
      if (!result || result.error) {
        const error = error_value(
          result && result.error,
          "Scraper progress channel reconnect failed",
        );
        last_error_.as(error);
        throw error;
      }
      return true;
    }

    async function disconnect() {
      const result = await channel.disconnect(1000, "manual disconnect");
      if (!result || result.error) {
        const error = error_value(
          result && result.error,
          "Scraper progress channel disconnect failed",
        );
        last_error_.as(error);
        throw error;
      }
      sync_channel_state({ connected: false, connecting: false });
      return true;
    }

    function ready() {
      if (!ready_promise) {
        ready_promise = connect().catch((error) => {
          last_error_.as(error_value(error));
          return false;
        });
      }
      return ready_promise;
    }

    function destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      channel.destroy();
      (jobs_.value || []).forEach((job) => job._dispose());
      jobs_by_id.clear();
      jobs_.as([]);
      messages.clear();
      websocket_connected_.as(false);
      websocket_connecting_.as(false);
    }

    if (auto_start !== false) {
      global.queueMicrotask(() => ready().catch(() => {}));
    }
    return domain;
  }

  /** SDK entry point. */
  function DL(props) {
    return DownloaderModel(props);
  }

  DL.DownloadTaskModel = DownloadTaskModel;
  DL.DownloaderModel = DownloaderModel;
  DL.ScraperJobModel = ScraperJobModel;
  DL.ScraperModel = ScraperModel;
  DL.TaskStatus = task_status;
  DownloaderModel.DownloadTaskModel = DownloadTaskModel;
  DownloaderModel.TaskStatus = task_status;
  ScraperModel.ScraperJobModel = ScraperJobModel;
  global.DownloadTaskModel = DownloadTaskModel;
  global.DownloaderModel = DownloaderModel;
  global.ScraperJobModel = ScraperJobModel;
  global.ScraperModel = ScraperModel;
  global.DL = DL;
})(window);

/**
 * @file Runtime proxy for the native fetch and XMLHttpRequest APIs.
 *
 * Every completed fetch/XHR response is captured by its original request
 * pathname and can later be read with `wx_fetch.get_response(pathname)`.
 * Capture is enabled by default and does not require a proxy rule.
 *
 * Optional rules are evaluated in declaration order. A rule can either
 * redirect the original request (`target` / `proxy`) or return the contents of
 * another URL (`response`), which is useful for local mock responses.
 *
 * @example
 * wx_fetch.set_rules([
 *   { url: "/api/feed", method: "GET", target: "http://127.0.0.1:2022/api/feed" },
 *   { url: "/api/profile", response: "profile.json", base_url: "/mock-data/" },
 * ]);
 */
var wx_fetch = (() => {
  if (window.wx_fetch && window.wx_fetch.__wx_fetch_proxy) {
    return window.wx_fetch;
  }

  const native_fetch = window.fetch;
  const xhr_prototype =
    window.XMLHttpRequest && window.XMLHttpRequest.prototype;
  const native_xhr_open = xhr_prototype && xhr_prototype.open;
  const native_xhr_send = xhr_prototype && xhr_prototype.send;
  const native_xhr_abort = xhr_prototype && xhr_prototype.abort;
  const native_xhr_get_response_header =
    xhr_prototype && xhr_prototype.getResponseHeader;
  const native_xhr_get_all_response_headers =
    xhr_prototype && xhr_prototype.getAllResponseHeaders;
  const proxy_options = { base_url: "" };
  const response_by_pathname = new Map();
  let is_installed = false;

  window.__wx_fetch_proxy_rules__ = Array.isArray(
    window.__wx_fetch_proxy_rules__,
  )
    ? window.__wx_fetch_proxy_rules__
    : [];

  function get_rules() {
    return window.__wx_fetch_proxy_rules__;
  }

  function set_rules(value) {
    window.__wx_fetch_proxy_rules__ = Array.isArray(value)
      ? value
      : value
        ? [value]
        : [];
    return get_rules();
  }

  function clear_rules() {
    return set_rules([]);
  }

  function configure(value) {
    if (value && typeof value === "object") {
      Object.assign(proxy_options, value);
      if (value.baseURL && !value.base_url) {
        proxy_options.base_url = value.baseURL;
      }
    }
    return { ...proxy_options };
  }

  function absolute_url(url) {
    try {
      return new URL(String(url || ""), window.location.href).href;
    } catch {
      return String(url || "");
    }
  }

  function normalize_pathname(value) {
    try {
      return new URL(String(value || ""), window.location.href).pathname;
    } catch {
      return String(value || "").split("?")[0].split("#")[0];
    }
  }

  function save_response(url, response) {
    const pathname = normalize_pathname(url);
    if (!pathname) return response;
    let saved_response = response;
    if (response && typeof response.clone === "function") {
      try {
        saved_response = response.clone();
      } catch {
        saved_response = response;
      }
    }
    response_by_pathname.set(pathname, saved_response);
    return response;
  }

  function get_response(pathname) {
    const response = response_by_pathname.get(normalize_pathname(pathname));
    if (!response) return null;
    if (typeof response.clone === "function") {
      try {
        return response.clone();
      } catch {
        return response;
      }
    }
    return response;
  }

  function clear_responses(pathname) {
    if (typeof pathname !== "undefined") {
      return response_by_pathname.delete(normalize_pathname(pathname));
    }
    response_by_pathname.clear();
  }

  function observe_fetch_response(promise, request_url) {
    return Promise.resolve(promise).then((response) => {
      save_response(request_url, response);
      return response;
    });
  }

  function matches_url(matcher, url) {
    const absolute = absolute_url(url);
    if (typeof matcher === "function") {
      return Boolean(matcher(url, absolute));
    }
    if (matcher instanceof RegExp) {
      matcher.lastIndex = 0;
      return matcher.test(String(url)) || matcher.test(absolute);
    }
    const expected = String(matcher || "");
    return Boolean(
      expected &&
        (String(url).includes(expected) || absolute.includes(expected)),
    );
  }

  function match_rule(url, method) {
    const normalized_method = String(method || "GET").toUpperCase();
    const list = get_rules();
    for (let i = 0; i < list.length; i += 1) {
      const rule = list[i];
      if (!rule || rule.disabled) continue;
      const matcher =
        rule.url || rule.url_pattern || rule.urlPattern || rule.pattern || rule.match;
      if (!matcher || !matches_url(matcher, url)) continue;
      if (
        !rule.method ||
        String(rule.method).toUpperCase() === normalized_method
      ) {
        return rule;
      }
    }
    return null;
  }

  function join_base_url(base_url, value) {
    if (!base_url) return String(value);
    return (
      String(base_url).replace(/\/+$/, "") +
      "/" +
      String(value).replace(/^\/+/, "")
    );
  }

  function proxy_target(rule, context) {
    let target =
      rule.target ||
      rule.proxy ||
      rule.proxy_url ||
      rule.proxyURL ||
      rule.response_url ||
      rule.responseURL;
    const response_mode =
      !target && Object.prototype.hasOwnProperty.call(rule, "response");
    if (response_mode) target = rule.response;
    if (typeof target === "function") target = target(context);
    if (target === undefined || target === null || target === "") return null;
    return {
      response_mode,
      url: join_base_url(
        rule.base_url || rule.baseURL || proxy_options.base_url,
        target,
      ),
    };
  }

  function request_method(input, init) {
    return String(
      (init && init.method) || (input && input.method) || "GET",
    ).toUpperCase();
  }

  function copy_request_init(request) {
    const init = {
      method: request.method,
      headers: request.headers,
      mode: request.mode,
      credentials: request.credentials,
      cache: request.cache,
      redirect: request.redirect,
      referrer: request.referrer,
      referrerPolicy: request.referrerPolicy,
      integrity: request.integrity,
      keepalive: request.keepalive,
      signal: request.signal,
    };
    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = request.body;
      // Required by some fetch implementations when a ReadableStream is used.
      init.duplex = "half";
    }
    return init;
  }

  function proxy_fetch(input, init) {
    const url =
      typeof input === "string"
        ? input
        : input && typeof input.url === "string"
          ? input.url
          : String(input || "");
    const method = request_method(input, init);
    const rule = match_rule(url, method);
    if (!rule) {
      return observe_fetch_response(
        native_fetch.apply(this, arguments),
        url,
      );
    }

    const context = { type: "fetch", input, init, url, method, rule };
    const target = proxy_target(rule, context);
    if (!target) {
      return observe_fetch_response(
        native_fetch.apply(this, arguments),
        url,
      );
    }

    if (target.response_mode) {
      return observe_fetch_response(
        native_fetch.call(
          window,
          target.url,
          rule.response_init || rule.responseInit,
        ),
        url,
      );
    }
    if (typeof window.Request === "function" && input instanceof Request) {
      const request = new Request(input, init);
      return observe_fetch_response(
        native_fetch.call(window, target.url, copy_request_init(request)),
        url,
      );
    }
    return observe_fetch_response(
      native_fetch.call(window, target.url, init),
      url,
    );
  }

  function set_xhr_value(xhr, name, value) {
    try {
      Object.defineProperty(xhr, name, {
        configurable: true,
        value,
      });
    } catch {
      // Older WebViews occasionally expose writable data properties instead.
      try {
        xhr[name] = value;
      } catch {
        // Ignore a read-only optional property.
      }
    }
  }

  function dispatch_xhr(xhr, type, init) {
    let event;
    try {
      event = new ProgressEvent(type, init || {});
    } catch {
      event = new Event(type);
    }
    xhr.dispatchEvent(event);
  }

  function update_ready_state(xhr, ready_state) {
    set_xhr_value(xhr, "readyState", ready_state);
    dispatch_xhr(xhr, "readystatechange");
  }

  function headers_to_string(headers) {
    let value = "";
    headers.forEach((header_value, name) => {
      value += `${name}: ${header_value}\r\n`;
    });
    return value;
  }

  function parse_xhr_headers(xhr) {
    let value = "";
    try {
      value = xhr.getAllResponseHeaders() || "";
    } catch {
      return [];
    }
    return value
      .trim()
      .split(/[\r\n]+/)
      .map((line) => {
        const index = line.indexOf(":");
        if (index < 0) return null;
        return [line.slice(0, index).trim(), line.slice(index + 1).trim()];
      })
      .filter(Boolean);
  }

  function xhr_response_body(xhr) {
    if (xhr.responseType === "json") {
      return xhr.response === null ? "null" : JSON.stringify(xhr.response);
    }
    if (xhr.responseType === "blob" || xhr.responseType === "arraybuffer") {
      return xhr.response;
    }
    try {
      return xhr.responseText;
    } catch {
      return xhr.response;
    }
  }

  function save_xhr_response(xhr) {
    if (xhr.readyState !== (window.XMLHttpRequest.DONE || 4)) return;
    let response = xhr.response;
    if (typeof Response === "function") {
      try {
        const status = Number(xhr.status) || 200;
        const body = [204, 205, 304].includes(status)
          ? null
          : xhr_response_body(xhr);
        response = new Response(body, {
          status,
          statusText: xhr.statusText || "",
          headers: parse_xhr_headers(xhr),
        });
      } catch {
        response = xhr.response;
      }
    }
    save_response(xhr.__wx_fetch_proxy_url, response);
  }

  async function read_xhr_response(response, response_type) {
    if (response_type === "blob") {
      return { response: await response.blob(), response_text: "" };
    }
    if (response_type === "arraybuffer") {
      return { response: await response.arrayBuffer(), response_text: "" };
    }
    const text = await response.text();
    if (response_type === "json") {
      let json = null;
      try {
        json = JSON.parse(text);
      } catch {
        // Native XHR returns null when a JSON response cannot be parsed.
      }
      return { response: json, response_text: text };
    }
    return { response: text, response_text: text };
  }

  function finish_xhr_with_error(xhr, error) {
    if (xhr.__wx_fetch_proxy_aborted) return;
    set_xhr_value(xhr, "status", 0);
    set_xhr_value(xhr, "statusText", "");
    update_ready_state(xhr, window.XMLHttpRequest.DONE || 4);
    dispatch_xhr(xhr, error && error.name === "AbortError" ? "abort" : "error");
    dispatch_xhr(xhr, "loadend");
  }

  function proxy_xhr_open(method, url) {
    const normalized_method = String(method || "GET").toUpperCase();
    const rule = match_rule(url, normalized_method);
    const context = {
      type: "xhr",
      xhr: this,
      url,
      method: normalized_method,
      rule,
    };
    const target = rule && proxy_target(rule, context);

    this.__wx_fetch_proxy_rule = rule;
    this.__wx_fetch_proxy_target = target;
    this.__wx_fetch_proxy_method = normalized_method;
    this.__wx_fetch_proxy_url = url;
    this.__wx_fetch_proxy_async = arguments.length < 3 || arguments[2] !== false;
    this.__wx_fetch_proxy_aborted = false;
    this.__wx_fetch_proxy_response_headers = null;

    if (this.__wx_fetch_proxy_capture_handler) {
      this.removeEventListener(
        "readystatechange",
        this.__wx_fetch_proxy_capture_handler,
      );
    }
    this.__wx_fetch_proxy_capture_handler = () => save_xhr_response(this);
    this.addEventListener(
      "readystatechange",
      this.__wx_fetch_proxy_capture_handler,
    );

    if (target && !target.response_mode) {
      const args = Array.from(arguments);
      args[1] = target.url;
      return native_xhr_open.apply(this, args);
    }
    return native_xhr_open.apply(this, arguments);
  }

  function proxy_xhr_send(body) {
    const target = this.__wx_fetch_proxy_target;
    if (!target || !target.response_mode || !this.__wx_fetch_proxy_async) {
      return native_xhr_send.apply(this, arguments);
    }

    const xhr = this;
    const controller =
      typeof AbortController === "function" ? new AbortController() : null;
    xhr.__wx_fetch_proxy_controller = controller;
    dispatch_xhr(xhr, "loadstart");

    const response_init = {
      ...(xhr.__wx_fetch_proxy_rule.response_init ||
        xhr.__wx_fetch_proxy_rule.responseInit ||
        {}),
    };
    if (controller) response_init.signal = controller.signal;

    native_fetch
      .call(window, target.url, response_init)
      .then(async (response) => {
        if (xhr.__wx_fetch_proxy_aborted) return;
        xhr.__wx_fetch_proxy_response_headers = response.headers;
        set_xhr_value(xhr, "status", response.status);
        set_xhr_value(xhr, "statusText", response.statusText);
        set_xhr_value(xhr, "responseURL", xhr.__wx_fetch_proxy_url);
        update_ready_state(xhr, window.XMLHttpRequest.HEADERS_RECEIVED || 2);

        const result = await read_xhr_response(response, xhr.responseType);
        if (xhr.__wx_fetch_proxy_aborted) return;
        update_ready_state(xhr, window.XMLHttpRequest.LOADING || 3);
        set_xhr_value(xhr, "responseText", result.response_text);
        set_xhr_value(xhr, "response", result.response);
        const loaded =
          typeof result.response_text === "string"
            ? result.response_text.length
            : 0;
        dispatch_xhr(xhr, "progress", { lengthComputable: false, loaded });
        update_ready_state(xhr, window.XMLHttpRequest.DONE || 4);
        save_xhr_response(xhr);
        dispatch_xhr(xhr, "load");
        dispatch_xhr(xhr, "loadend");
      })
      .catch((error) => finish_xhr_with_error(xhr, error));
  }

  function proxy_xhr_abort() {
    if (this.__wx_fetch_proxy_controller) {
      this.__wx_fetch_proxy_aborted = true;
      this.__wx_fetch_proxy_controller.abort();
      set_xhr_value(this, "readyState", window.XMLHttpRequest.UNSENT || 0);
      dispatch_xhr(this, "abort");
      dispatch_xhr(this, "loadend");
      return;
    }
    return native_xhr_abort.apply(this, arguments);
  }

  function proxy_xhr_get_response_header(name) {
    if (this.__wx_fetch_proxy_response_headers) {
      return this.__wx_fetch_proxy_response_headers.get(name);
    }
    return native_xhr_get_response_header.apply(this, arguments);
  }

  function proxy_xhr_get_all_response_headers() {
    if (this.__wx_fetch_proxy_response_headers) {
      return headers_to_string(this.__wx_fetch_proxy_response_headers);
    }
    return native_xhr_get_all_response_headers.apply(this, arguments);
  }

  function install() {
    if (is_installed) return proxy_api;
    if (typeof native_fetch === "function") window.fetch = proxy_fetch;
    if (xhr_prototype) {
      xhr_prototype.open = proxy_xhr_open;
      xhr_prototype.send = proxy_xhr_send;
      xhr_prototype.abort = proxy_xhr_abort;
      xhr_prototype.getResponseHeader = proxy_xhr_get_response_header;
      xhr_prototype.getAllResponseHeaders =
        proxy_xhr_get_all_response_headers;
    }
    is_installed = true;
    return proxy_api;
  }

  function restore() {
    if (!is_installed) return proxy_api;
    if (typeof native_fetch === "function") window.fetch = native_fetch;
    if (xhr_prototype) {
      xhr_prototype.open = native_xhr_open;
      xhr_prototype.send = native_xhr_send;
      xhr_prototype.abort = native_xhr_abort;
      xhr_prototype.getResponseHeader = native_xhr_get_response_header;
      xhr_prototype.getAllResponseHeaders =
        native_xhr_get_all_response_headers;
    }
    is_installed = false;
    return proxy_api;
  }

  const proxy_api = {
    __wx_fetch_proxy: true,
    clear_rules,
    clear_responses,
    configure,
    get_response,
    install,
    match_rule,
    native_fetch,
    native_xml_http_request: window.XMLHttpRequest,
    restore,
    set_rules,
    get rules() {
      return get_rules();
    },
  };

  window.__wx_set_fetch_proxy_rules__ = set_rules;
  window.__wx_clear_fetch_proxy_rules__ = clear_rules;
  window.wx_fetch = proxy_api;
  install();
  return proxy_api;
})();

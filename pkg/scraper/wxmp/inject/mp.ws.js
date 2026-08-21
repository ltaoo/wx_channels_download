/**
 * 微信公众号页面 WebSocket Model。
 *
 * 负责维护连接状态、执行后端下发的页面请求并回传结果，不负责渲染视图。
 */
(() => {
  const WEBSOCKET_RETRY_INTERVAL = 5000;
  const WEBSOCKET_PING_INTERVAL = 5000;
  const SocketClientCore = Timeless.kit.SocketClientCore;
  const ChannelCore = Timeless.kit.ChannelCore;
  const JSAPI_DEFAULT_TIMEOUT = 15000;
  const JSAPI_MAX_TIMEOUT = 120000;
  const JSAPI_NO_AUTH_WAIT_METHODS = new Set([
    "notifyPageInfo",
    "updatePageAuth",
  ]);
  const JSAPI_INVOKE_METHODS = Object.freeze([
    "H5ExtTransfer",
    "addContact",
    "addDownloadTask",
    "addDownloadTaskStraight",
    "adDataReport",
    "batchAddCard",
    "batchPreloadMiniProgram",
    "calRqt",
    "closeWindow",
    "createWebViewForFastLoad",
    "currentMpInfo",
    "disableBounceScroll",
    "downloadAppInternal",
    "downloadPageDataForFastLoad",
    "getAdIdInfo",
    "getBackgroundAudioState",
    "getInstallState",
    "getNetworkType",
    "getOAID",
    "getTingAudioState",
    "handleAdAction",
    "handleDeviceInfo",
    "handleEcsAction",
    "handleImmersiveStream",
    "handleMPPageAction",
    "imagePreview",
    "installDownloadTask",
    "jumpToBizProfile",
    "kvReport",
    "launch3rdApp",
    "launchApplication",
    "log",
    "makePhoneCall",
    "notifyPageInfo",
    "openADCanvas",
    "openBizChat",
    "openCardDetail",
    "openCustomerServiceChat",
    "openEmbeddedWeApp",
    "openFinderTopicView",
    "openFinderView",
    "openLiteApp",
    "openUrlWithExtraWebview",
    "openWXSearchHalfPage",
    "openWXSearchPage",
    "openWeApp",
    "openWebViewUseFastLoad",
    "operateStar",
    "pauseDownloadTask",
    "predownloadMiniProgramPackage",
    "preloadMiniProgramContacts",
    "preloadMiniProgramEnv",
    "profile",
    "queryDownloadTask",
    "request",
    "resumeDownloadTask",
    "saveWaid",
    "sendAppMessage",
    "sendAppMessagePrivate",
    "sendEmail",
    "setNavigationBarButtons",
    "setNavigationBarColor",
    "shareFB",
    "shareQQ",
    "shareQZone",
    "shareTimeline",
    "shareTimelinePrivate",
    "shareWeibo",
    "shareWeiboApp",
    "showOpenIMContactProfile",
    "updatePageAuth",
    "webTransfer",
    "writeCommData",
    "writeLog",
    "wwapp2.openMPURLInWechat",
  ]);
  const JSAPI_EVENTS = Object.freeze([
    "activity:state_change",
    "fakeImmersiveUIStyleTopInsetChanged",
    "immersiveStreamEnterFullArticle",
    "immersiveStreamExitFullArticle",
    "immersiveStreamExposeArticle",
    "immersiveStreamSlideOutArticle",
    "menu:general:share",
    "menu:share:QZone",
    "menu:share:appmessage",
    "menu:share:email",
    "menu:share:facebook",
    "menu:share:qq",
    "menu:share:timeline",
    "menu:share:weibo",
    "menu:share:weiboApp",
    "onActionBarClickEventInImmersiveMode",
    "onBackgroundAudioStateChange",
    "onMPPageAction",
    "onPageStarStateChanged",
    "onScreenShot",
    "onTingAudioStateChanged",
    "onWebPageUrlExposed",
    "onWindowFocusChanged",
    "reportOnLeaveForMP",
    "wxdownload:progress_change",
  ]);
  const JSAPI_ACTIONS = Object.freeze({
    handleDeviceInfo: Object.freeze(["getSafeAreaInsets", "getUIParams"]),
    handleEcsAction: Object.freeze(["checkAction", "openEcs"]),
    handleImmersiveStream: Object.freeze(["enterFullArticle"]),
    handleMPPageAction: Object.freeze([
      "getBiz",
      "reportByLeaveForMPGateway",
      "wxConfig",
    ]),
    operateStar: Object.freeze(["addStar", "cancelStar"]),
  });
  const JSAPI_ACTION_FIELDS = Object.freeze({
    handleDeviceInfo: "action",
    handleEcsAction: "action",
    handleImmersiveStream: "action",
    handleMPPageAction: "action",
    operateStar: "opType",
  });

  function create_jsapi_error(code, message) {
    const error = new Error(message);
    error.code = code;
    return error;
  }

  function error_message(error) {
    return error instanceof Error ? error.message : String(error);
  }

  function normalize_timeout(value, fallback = JSAPI_DEFAULT_TIMEOUT) {
    const timeout = Number(value);
    if (!Number.isFinite(timeout) || timeout <= 0) {
      return fallback;
    }
    return Math.min(Math.max(Math.trunc(timeout), 1), JSAPI_MAX_TIMEOUT);
  }

  function normalize_transport_value(value, seen = new WeakSet()) {
    if (value === undefined || value === null) {
      return null;
    }
    if (typeof value === "bigint") {
      return value.toString();
    }
    if (typeof value === "function" || typeof value === "symbol") {
      return String(value);
    }
    if (typeof value !== "object") {
      return value;
    }
    if (value instanceof Error) {
      return {
        message: value.message,
        name: value.name,
        stack: value.stack || "",
      };
    }
    if (value instanceof Date) {
      return value.toISOString();
    }
    if (seen.has(value)) {
      return "[Circular]";
    }
    seen.add(value);
    if (Array.isArray(value)) {
      const result = value.map((item) =>
        normalize_transport_value(item, seen),
      );
      seen.delete(value);
      return result;
    }
    if (typeof ArrayBuffer !== "undefined") {
      if (value instanceof ArrayBuffer) {
        seen.delete(value);
        return {
          byte_length: value.byteLength,
          type: "ArrayBuffer",
        };
      }
      if (ArrayBuffer.isView(value)) {
        const result = Array.from(value);
        seen.delete(value);
        return result;
      }
    }
    const result = {};
    Object.keys(value).forEach((key) => {
      try {
        result[key] = normalize_transport_value(value[key], seen);
      } catch (error) {
        result[key] = `[Unserializable: ${error_message(error)}]`;
      }
    });
    seen.delete(value);
    return result;
  }

  function MPJSAPIModel() {
    const invoke_methods = new Set(JSAPI_INVOKE_METHODS);
    const event_names = new Set(JSAPI_EVENTS);
    const event_records = new Map();

    function bridge_context() {
      try {
        const bridge_window = window.top || window;
        const bridge_document = bridge_window.document;
        return { bridge_document, bridge_window };
      } catch {
        throw create_jsapi_error(
          1101,
          "cross-origin frame cannot access WeixinJSBridge",
        );
      }
    }

    function current_bridge() {
      const context = bridge_context();
      const bridge = context.bridge_window.WeixinJSBridge;
      if (!bridge || typeof bridge.invoke !== "function") {
        return null;
      }
      return bridge;
    }

    function wait_for_bridge(timeout_ms) {
      const available_bridge = current_bridge();
      if (available_bridge) {
        return Promise.resolve(available_bridge);
      }
      const context = bridge_context();
      return new Promise((resolve, reject) => {
        let settled = false;
        let timer = null;

        function cleanup() {
          if (timer !== null) {
            clearTimeout(timer);
            timer = null;
          }
          if (context.bridge_document.removeEventListener) {
            context.bridge_document.removeEventListener(
              "WeixinJSBridgeReady",
              handle_ready,
              false,
            );
          } else if (context.bridge_document.detachEvent) {
            context.bridge_document.detachEvent(
              "onWeixinJSBridgeReady",
              handle_ready,
            );
          }
        }

        function handle_ready() {
          if (settled) {
            return;
          }
          const bridge = context.bridge_window.WeixinJSBridge;
          if (!bridge || typeof bridge.invoke !== "function") {
            settled = true;
            cleanup();
            reject(
              create_jsapi_error(
                1102,
                "WeixinJSBridgeReady fired without an available bridge",
              ),
            );
            return;
          }
          settled = true;
          cleanup();
          resolve(bridge);
        }

        if (context.bridge_document.addEventListener) {
          context.bridge_document.addEventListener(
            "WeixinJSBridgeReady",
            handle_ready,
            false,
          );
        } else if (context.bridge_document.attachEvent) {
          context.bridge_document.attachEvent(
            "onWeixinJSBridgeReady",
            handle_ready,
          );
        } else {
          reject(
            create_jsapi_error(
              1102,
              "document cannot observe WeixinJSBridgeReady",
            ),
          );
          return;
        }
        timer = setTimeout(() => {
          if (settled) {
            return;
          }
          settled = true;
          cleanup();
          reject(create_jsapi_error(1102, "waiting for WeixinJSBridge timed out"));
        }, timeout_ms);
      });
    }

    async function wait_for_page_auth(method_name, deadline) {
      if (
        !window.__secPageAuthPromise ||
        window.__is_page_auth_ok__ ||
        JSAPI_NO_AUTH_WAIT_METHODS.has(method_name)
      ) {
        return;
      }
      const timeout_ms = Math.max(deadline - Date.now(), 1);
      let timer = null;
      try {
        await Promise.race([
          Promise.resolve(window.__secPageAuthPromise),
          new Promise((resolve, reject) => {
            timer = setTimeout(
              () => reject(create_jsapi_error(1102, "page auth timed out")),
              timeout_ms,
            );
          }),
        ]);
      } finally {
        if (timer !== null) {
          clearTimeout(timer);
        }
      }
    }

    function assert_method(method_name) {
      if (!invoke_methods.has(method_name)) {
        throw create_jsapi_error(
          1106,
          `unsupported JSAPI method: ${method_name || "<empty>"}`,
        );
      }
    }

    function assert_event(event_name) {
      if (!event_names.has(event_name)) {
        throw create_jsapi_error(
          1106,
          `unsupported JSAPI event: ${event_name || "<empty>"}`,
        );
      }
    }

    async function ready(timeout_value) {
      const timeout_ms = normalize_timeout(timeout_value);
      const started_at = Date.now();
      await wait_for_bridge(timeout_ms);
      return {
        bridge_available: true,
        elapsed_ms: Date.now() - started_at,
      };
    }

    async function invoke(method_name, args, timeout_value) {
      assert_method(method_name);
      const timeout_ms = normalize_timeout(timeout_value);
      const started_at = Date.now();
      const deadline = started_at + timeout_ms;
      await wait_for_page_auth(method_name, deadline);
      const bridge = await wait_for_bridge(Math.max(deadline - Date.now(), 1));
      return new Promise((resolve, reject) => {
        let settled = false;
        const timer = setTimeout(() => {
          if (settled) {
            return;
          }
          settled = true;
          reject(
            create_jsapi_error(
              1104,
              `JSAPI invoke callback timed out: ${method_name}`,
            ),
          );
        }, Math.max(deadline - Date.now(), 1));
        try {
          bridge.invoke(method_name, args ?? {}, function () {
            if (settled) {
              return;
            }
            settled = true;
            clearTimeout(timer);
            const callback_results = Array.from(arguments).map((item) =>
              normalize_transport_value(item),
            );
            resolve({
              elapsed_ms: Date.now() - started_at,
              method: method_name,
              result: callback_results[0] ?? null,
              results: callback_results,
            });
          });
        } catch (error) {
          settled = true;
          clearTimeout(timer);
          reject(
            create_jsapi_error(
              1103,
              `JSAPI invoke failed (${method_name}): ${error_message(error)}`,
            ),
          );
        }
      });
    }

    async function call(method_name, timeout_value) {
      assert_method(method_name);
      const timeout_ms = normalize_timeout(timeout_value);
      const started_at = Date.now();
      const deadline = started_at + timeout_ms;
      await wait_for_page_auth(method_name, deadline);
      const bridge = await wait_for_bridge(Math.max(deadline - Date.now(), 1));
      if (typeof bridge.call !== "function") {
        throw create_jsapi_error(1102, "WeixinJSBridge.call is unavailable");
      }
      try {
        bridge.call(method_name);
      } catch (error) {
        throw create_jsapi_error(
          1103,
          `JSAPI call failed (${method_name}): ${error_message(error)}`,
        );
      }
      return {
        called: true,
        elapsed_ms: Date.now() - started_at,
        method: method_name,
      };
    }

    function dispatch_event(event_name, callback_args) {
      const record = event_records.get(event_name);
      if (!record || record.waiters.size === 0) {
        return;
      }
      const event_results = callback_args.map((item) =>
        normalize_transport_value(item),
      );
      const waiters = Array.from(record.waiters);
      record.waiters.clear();
      waiters.forEach((waiter) => {
        clearTimeout(waiter.timer);
        waiter.resolve({
          event: event_name,
          result: event_results[0] ?? null,
          results: event_results,
        });
      });
    }

    async function ensure_event_record(event_name, timeout_ms) {
      let record = event_records.get(event_name);
      if (record) {
        return record;
      }
      const bridge = await wait_for_bridge(timeout_ms);
      if (typeof bridge.on !== "function") {
        throw create_jsapi_error(1102, "WeixinJSBridge.on is unavailable");
      }
      record = { waiters: new Set() };
      event_records.set(event_name, record);
      try {
        bridge.on(event_name, function () {
          dispatch_event(event_name, Array.from(arguments));
        });
      } catch (error) {
        event_records.delete(event_name);
        throw create_jsapi_error(
          1103,
          `JSAPI event registration failed (${event_name}): ${error_message(error)}`,
        );
      }
      return record;
    }

    async function wait_event(event_name, timeout_value) {
      assert_event(event_name);
      const timeout_ms = normalize_timeout(timeout_value, 30000);
      const started_at = Date.now();
      const record = await ensure_event_record(event_name, timeout_ms);
      const remaining_ms = Math.max(timeout_ms - (Date.now() - started_at), 1);
      return new Promise((resolve, reject) => {
        const waiter = { reject, resolve, timer: null };
        waiter.timer = setTimeout(() => {
          record.waiters.delete(waiter);
          reject(
            create_jsapi_error(
              1105,
              `waiting for JSAPI event timed out: ${event_name}`,
            ),
          );
        }, remaining_ms);
        record.waiters.add(waiter);
      });
    }

    function remove_event(event_name) {
      assert_event(event_name);
      const record = event_records.get(event_name);
      if (!record) {
        return { event: event_name, removed: 0 };
      }
      const waiters = Array.from(record.waiters);
      record.waiters.clear();
      waiters.forEach((waiter) => {
        clearTimeout(waiter.timer);
        waiter.reject(
          create_jsapi_error(1105, `JSAPI event wait removed: ${event_name}`),
        );
      });
      return { event: event_name, removed: waiters.length };
    }

    function capabilities() {
      let bridge_available = false;
      try {
        bridge_available = current_bridge() !== null;
      } catch {
        bridge_available = false;
      }
      return {
        action_fields: JSAPI_ACTION_FIELDS,
        actions: JSAPI_ACTIONS,
        bridge_available,
        events: JSAPI_EVENTS,
        invoke_methods: JSAPI_INVOKE_METHODS,
        operations: [
          "capabilities",
          "ready",
          "invoke",
          "call",
          "wait_event",
          "remove_event",
        ],
      };
    }

    async function execute(request) {
      const input = request && typeof request === "object" ? request : {};
      const operation = String(input.operation || "invoke").trim();
      switch (operation) {
        case "capabilities":
          return capabilities();
        case "ready":
          return ready(input.timeout_ms);
        case "invoke":
          return invoke(
            String(input.method || "").trim(),
            input.args,
            input.timeout_ms,
          );
        case "call":
          return call(String(input.method || "").trim(), input.timeout_ms);
        case "on":
        case "wait_event":
          return wait_event(
            String(input.event || "").trim(),
            input.timeout_ms,
          );
        case "remove":
        case "remove_event":
          return remove_event(String(input.event || "").trim());
        default:
          throw create_jsapi_error(
            1100,
            `unsupported JSAPI operation: ${operation || "<empty>"}`,
          );
      }
    }

    function destroy() {
      event_records.forEach((record, event_name) => {
        const waiters = Array.from(record.waiters);
        record.waiters.clear();
        waiters.forEach((waiter) => {
          clearTimeout(waiter.timer);
          waiter.reject(
            create_jsapi_error(
              1105,
              `JSAPI model destroyed while waiting for event: ${event_name}`,
            ),
          );
        });
      });
      event_records.clear();
    }

    return {
      call,
      capabilities,
      destroy,
      execute,
      invoke,
      ready,
      remove_event,
      wait_event,
    };
  }

  function build_mp_ws_url() {
    const configured_url = WXEnv.get("mpWSURL");
    if (configured_url) {
      return configured_url;
    }
    const api_url = new URL(WXEnv.get("apiOrigin"));
    api_url.protocol = WXEnv.wsProtocol(api_url.protocol.replace(":", ""));
    api_url.pathname = "/ws/mp";
    api_url.search = "";
    api_url.hash = "";
    return api_url.href;
  }

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

  async function fetch_page_content(data) {
    const raw_url = String((data && data.url) || "").trim();
    if (!raw_url) {
      throw new Error("missing url");
    }
    const target_url = new URL(raw_url, window.location.href);
    const response = await fetch(target_url.href, {
      credentials: "include",
    });
    const content = await response.text();
    if (!response.ok) {
      throw new Error(
        `fetch page content failed: ${response.status} ${response.statusText}`,
      );
    }
    return {
      url: response.url || target_url.href,
      status_code: response.status,
      content,
    };
  }

  function MPWebsocketClient() {
    const state = {
      connection_status: "disconnected",
      last_error: null,
    };
    const socket_client = new SocketClientCore();
    const jsapi_model = MPJSAPIModel();
    Timeless.web.provide_socket_client(socket_client, { WebSocket });

    const websocket_url = build_mp_ws_url();
    WXU.log
      .Info()
      .Str("file", "/mp.ws.js")
      .Str("websocket_url", websocket_url)
      .Str("page_url", window.location.href)
      .Msg("mp ws客户端初始化");
    WXU.log.flushNow();

    const channel = new ChannelCore(websocket_url, {
      client: socket_client,
      process: decode_socket_message,
      reconnect: {
        enabled: true,
        interval: WEBSOCKET_RETRY_INTERVAL,
      },
    });
    let ping_timer_ = null;
    let connected_once_ = false;

    function sync_channel_state(channel_state) {
      const previous_status = state.connection_status;
      if (channel_state.connected) {
        state.connection_status = "connected";
      } else if (channel_state.connecting) {
        state.connection_status = "connecting";
      } else if (channel_state.status === "reconnecting") {
        state.connection_status = "reconnecting";
      } else {
        state.connection_status = "disconnected";
      }
      state.last_error = channel_state.error || null;
      if (previous_status !== state.connection_status) {
        WXU.log
          .Info()
          .Str("file", "/mp.ws.js")
          .Str("websocket_url", websocket_url)
          .Str("previous_status", previous_status)
          .Str("connection_status", state.connection_status)
          .Msg("mp ws连接状态变化");
        WXU.log.flushNow();
      }
    }

    async function send_message(message) {
      const result = await channel.send(message);
      if (!result || result.error) {
        throw (result && result.error) || new Error("mp websocket send failed");
      }
    }

    function cancel_ping() {
      if (ping_timer_ === null) {
        return;
      }
      clearInterval(ping_timer_);
      ping_timer_ = null;
    }

    function send_ping() {
      if (!channel.connected) {
        return;
      }
      send_message({
        type: "ping",
        data: document.title || "公众号页面",
      }).catch(() => {});
    }

    function start_ping() {
      cancel_ping();
      send_ping();
      ping_timer_ = setInterval(send_ping, WEBSOCKET_PING_INTERVAL);
    }

    async function respond(id, body) {
      await send_message({
        id,
        data: body,
      });
    }

    const methods = {
      async connect_local_ws() {
        WXU.log
          .Info()
          .Str("file", "/mp.ws.js")
          .Str("websocket_url", websocket_url)
          .Msg("mp ws开始建立连接");
        WXU.log.flushNow();
        const result = await channel.connect();
        if (!result || result.error) {
          throw (
            (result && result.error) ||
            new Error("mp websocket connection failed")
          );
        }
        return true;
      },
      async reconnect_local_ws() {
        WXU.log
          .Info()
          .Str("file", "/mp.ws.js")
          .Str("websocket_url", websocket_url)
          .Msg("mp ws开始重新连接");
        WXU.log.flushNow();
        const result = await channel.reconnect();
        if (!result || result.error) {
          throw (
            (result && result.error) ||
            new Error("mp websocket reconnect failed")
          );
        }
        return true;
      },
      async disconnect_local_ws() {
        cancel_ping();
        const result = await channel.disconnect(1000, "manual disconnect");
        if (!result || result.error) {
          throw (
            (result && result.error) ||
            new Error("mp websocket disconnect failed")
          );
        }
        return true;
      },
      async handle_api_call(message) {
        const { id, key, data } = message;
        WXU.log
          .Info()
          .Str("file", "/mp.ws.js")
          .Str("id", String(id))
          .Str("key", key)
          .Str("operation", String((data && data.operation) || ""))
          .Str("method", String((data && data.method) || ""))
          .Str("event", String((data && data.event) || ""))
          .Msg("handle_api_call");
        if (key === "key:fetch_page_content") {
          try {
            const result = await fetch_page_content(data);
            await respond(id, {
              errCode: 0,
              data: result,
            });
          } catch (error) {
            await respond(id, {
              errCode: 1001,
              errMsg: error instanceof Error ? error.message : String(error),
            });
          }
          return;
        }
        if (key === "key:jsapi") {
          try {
            const result = await jsapi_model.execute(data);
            await respond(id, {
              errCode: 0,
              data: result,
            });
          } catch (error) {
            await respond(id, {
              errCode: Number(error && error.code) || 1100,
              errMsg: error_message(error),
            });
          }
          return;
        }
        await respond(id, {
          errCode: 1000,
          errMsg: "未匹配的key",
          payload: message,
        });
      },
      destroy() {
        cancel_ping();
        jsapi_model.destroy();
        channel.destroy();
      },
    };

    channel.onMessage((message) => {
      if (!message || message.type !== "api_call") {
        return;
      }
      methods.handle_api_call(message.data).catch((error) => {
        state.last_error = error;
        WXU.error({
          source: "mp.ws.js:handle_api_call",
          msg: error instanceof Error ? error.message : String(error),
        });
      });
    });
    channel.onStateChange(sync_channel_state);
    channel.onConnected(() => {
      connected_once_ = true;
      start_ping();
      WXU.log
        .Info()
        .Str("file", "/mp.ws.js")
        .Str("websocket_url", websocket_url)
        .Msg("mp ws连接已建立");
      WXU.log.flushNow();
    });
    channel.onClose((event) => {
      cancel_ping();
      if (!connected_once_) {
        return;
      }
      const close_event = event || {};
      WXU.error({
        source: "mp.ws.js:onclose",
        msg: `mp ws连接已关闭，url: "${websocket_url}"，reason: "${close_event.reason || ""}"，code: "${close_event.code || ""}"`,
      });
      WXU.log.flushNow();
    });

    return {
      state,
      methods,
      channel,
      jsapi_model,
      socket_client,
      websocket_url,
    };
  }

  window.mp_ws_client$ = MPWebsocketClient();
  window.mp_jsapi_model$ = window.mp_ws_client$.jsapi_model;
  window.mp_ws_client$.methods.connect_local_ws().catch((error) => {
    WXU.error({
      source: "mp.ws.js:connect_local_ws",
      msg: `mp ws连接失败，url: "${window.mp_ws_client$.websocket_url}"，error: "${error instanceof Error ? error.message : String(error)}"`,
    });
    WXU.log.flushNow();
  });
})();

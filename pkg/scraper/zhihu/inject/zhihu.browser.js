/**
 * @file Real-browser bridge for Zhihu pages.
 *
 * The Model owns the WebSocket, request lifecycle, hidden navigation frames,
 * and response state. There is no visual View because this bridge renders no
 * user interface.
 */
(function () {
  class ZhihuBrowserBridgeModel {
    constructor() {
      this.socket = null;
      this.reconnect_timer = null;
      this.frames = new Map();
      this.child_request_id = this.read_child_request_id();
      this.stopped = false;
    }

    start() {
      if (window.__wx_zhihu_browser_bridge_started__) {
        return;
      }
      window.__wx_zhihu_browser_bridge_started__ = true;
      this.connect();
    }

    read_child_request_id() {
      try {
        const params = new URLSearchParams(location.hash.replace(/^#/, ""));
        return String(params.get("wx_zhihu_bridge_request") || "").trim();
      } catch (_error) {
        return "";
      }
    }

    websocket_url() {
      const protocol = WXEnv.wsProtocol(WXEnv.get("apiProtocol"));
      return `${protocol}://${WXEnv.get("apiHost")}/ws/zhihu/browser`;
    }

    connect() {
      if (this.stopped || this.socket) {
        return;
      }
      let socket;
      try {
        socket = new WebSocket(this.websocket_url());
      } catch (_error) {
        this.schedule_reconnect();
        return;
      }
      this.socket = socket;
      socket.addEventListener("open", () => {
        if (this.socket !== socket) {
          return;
        }
        this.send({
          type: "hello",
          role: window.top === window ? "top" : "child",
          hostname: location.hostname,
        });
        if (this.child_request_id) {
          this.respond_from_child();
        }
      });
      socket.addEventListener("message", (event) => {
        if (this.socket !== socket) {
          return;
        }
        this.handle_message(event.data);
      });
      socket.addEventListener("close", () => {
        if (this.socket !== socket) {
          return;
        }
        this.socket = null;
        if (!this.child_request_id) {
          this.schedule_reconnect();
        }
      });
      socket.addEventListener("error", () => {
        if (this.socket === socket) {
          socket.close();
        }
      });
    }

    schedule_reconnect() {
      if (this.stopped || this.child_request_id || this.reconnect_timer) {
        return;
      }
      this.reconnect_timer = window.setTimeout(() => {
        this.reconnect_timer = null;
        this.connect();
      }, 1500);
    }

    send(message) {
      if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
        return false;
      }
      this.socket.send(JSON.stringify(message));
      return true;
    }

    handle_message(raw_message) {
      let message;
      try {
        message = JSON.parse(String(raw_message || ""));
      } catch (_error) {
        return;
      }
      if (!message || typeof message !== "object") {
        return;
      }
      if (message.type === "fetch") {
        this.handle_fetch(message);
        return;
      }
      if (message.type === "complete") {
        this.remove_frame(message.request_id);
      }
    }

    async handle_fetch(message) {
      const request_id = String(message.request_id || "").trim();
      if (!request_id) {
        return;
      }
      let target_url;
      try {
        target_url = new URL(String(message.url || ""), location.href);
      } catch (_error) {
        this.send_error(request_id, "知乎浏览器收到无效 URL");
        return;
      }
      if (
        target_url.protocol !== "https:" ||
        !["www.zhihu.com", "zhuanlan.zhihu.com"].includes(
          target_url.hostname,
        )
      ) {
        this.send_error(request_id, "知乎浏览器拒绝非知乎 URL");
        return;
      }

      const request_kind = String(message.kind || "fetch").trim();
      const request_headers = new Headers();
      Object.entries(message.headers || {}).forEach(([name, value]) => {
        request_headers.set(name, String(value));
      });
      const fetch_options = {
        method: "GET",
        credentials: "include",
        cache: "no-store",
        headers: request_headers,
      };
      if (message.referer) {
        fetch_options.referrer = String(message.referer);
      }

      try {
        const response = await fetch(target_url.href, fetch_options);
        const body = await response.text();
        if (request_kind !== "html") {
          this.send_result(request_id, response.status, body);
          return;
        }
        if (
          response.ok &&
          body.includes('id="js-initialData"') &&
          !body.includes('id="zh-zse-ck"')
        ) {
          this.send_result(request_id, response.status, body);
          return;
        }
      } catch (_error) {
        // Cross-origin article requests and ZSE challenge responses continue
        // through a real iframe navigation below.
      }
      this.navigate_in_frame(request_id, target_url);
    }

    navigate_in_frame(request_id, target_url) {
      this.remove_frame(request_id);
      const frame = document.createElement("iframe");
      frame.setAttribute("aria-hidden", "true");
      frame.style.cssText =
        "position:fixed;width:1px;height:1px;left:-10000px;top:-10000px;border:0;opacity:0;pointer-events:none";
      const frame_url = new URL(target_url.href);
      frame_url.hash = new URLSearchParams({
        wx_zhihu_bridge_request: request_id,
      }).toString();
      frame.src = frame_url.href;
      const timeout_id = window.setTimeout(() => {
        this.remove_frame(request_id);
        this.send_error(request_id, "知乎浏览器加载页面超时");
      }, 45000);
      this.frames.set(request_id, { frame, timeout_id });
      (document.body || document.documentElement).appendChild(frame);
    }

    respond_from_child() {
      const request_id = this.child_request_id;
      const respond = () => {
        const initial_data = document.querySelector("#js-initialData");
        const challenge = document.querySelector("#zh-zse-ck");
        if (initial_data && !challenge) {
          this.send_result(
            request_id,
            200,
            document.documentElement.outerHTML,
          );
          return true;
        }
        const body_text = String(document.body?.innerText || "");
        if (body_text.includes('"code":40362') || body_text.includes("40362")) {
          this.send_error(request_id, "知乎限制了当前浏览器访问（40362）");
          return true;
        }
        return false;
      };
      if (respond()) {
        return;
      }
      const observer = new MutationObserver(() => {
        if (respond()) {
          observer.disconnect();
        }
      });
      observer.observe(document.documentElement, {
        childList: true,
        subtree: true,
      });
      window.setTimeout(() => {
        observer.disconnect();
        if (!document.querySelector("#zh-zse-ck")) {
          this.send_error(request_id, "知乎页面未返回可解析内容");
        }
      }, 20000);
    }

    send_result(request_id, status_code, body) {
      this.send({
        type: "result",
        request_id,
        status_code: Number(status_code) || 200,
        body: String(body || ""),
      });
    }

    send_error(request_id, error) {
      this.send({
        type: "error",
        request_id,
        error: String(error || "知乎浏览器抓取失败"),
      });
    }

    remove_frame(request_id) {
      const frame_state = this.frames.get(String(request_id || ""));
      if (!frame_state) {
        return;
      }
      window.clearTimeout(frame_state.timeout_id);
      frame_state.frame.remove();
      this.frames.delete(String(request_id || ""));
    }
  }

  const bridge_model = new ZhihuBrowserBridgeModel();
  window.__wx_zhihu_browser_bridge_model__ = bridge_model;
  bridge_model.start();
})();

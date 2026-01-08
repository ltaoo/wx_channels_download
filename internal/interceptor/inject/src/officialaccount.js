var _OfficialAccountCredentials = {
  nickname: window.nickname,
  avatar_url: window.headimg,
  biz: window.biz || window.__biz,
  uin: window.uin,
  key: window.key,
  pass_ticket: window.pass_ticket,
  appmsg_token: window.appmsg_token,
};
(() => {
  var style = document.createElement("style");
  style.textContent = `
    #wechat-tools-container {
      position: fixed;
      top: 12px;
      right: 12px;
      z-index: 9999;
      display: flex;
      flex-direction: column;
      gap: 12px;
      width: 160px;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    }
    #__wx_channels_credentials__,
    #__wx_channels_curl__,
    #__wx_channels_api__ {
      padding: 12px;
      background-color: var(--weui-BG-2, #fff);
      color: var(--weui-FG-0, #000);
      border-radius: 8px;
      box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
      font-size: 11px;
      line-height: 1.4;
      cursor: pointer;
      transition: all 0.2s;
      backdrop-filter: blur(10px);
      text-align: center;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    #__wx_channels_credentials__:hover,
    #__wx_channels_curl__:hover,
    #__wx_channels_api__:hover {
      opacity: 1;
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    }
    @media (prefers-color-scheme: dark) {
      #__wx_channels_credentials__,
      #__wx_channels_curl__,
      #__wx_channels_api__ {
        background-color: var(--weui-BG-2, #2c2c2c);
        color: var(--weui-FG-0, #fff);
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
      }
    }
  `;
  document.head.appendChild(style);

  WXU.emit(WXU.Events.OfficialAccountRefresh, _OfficialAccountCredentials);
  (async () => {
    var origin = `https://${FakeAPIServerAddr}`;
    var [err, res] = await WXU.request({
      method: "POST",
      url: `${origin}/api/mp/refresh?token=${
        WXU.config.officialServerRefreshToken ?? ""
      }`,
      body: _OfficialAccountCredentials,
    });
    if (err) {
      WXU.error({
        msg: err.message,
      });
      return;
    }
  })();

  async function handle_api_call(msg, socket) {
    var { id, key, data } = msg;
    function resp(body) {
      socket.send(
        JSON.stringify({
          id,
          data: body,
        })
      );
    }
    if (key === "key:fetch_account_home") {
      var [error, res] = await fetchAccountHome(data);
      if (error) {
        resp({
          errCode: 1001,
          errMsg: error.message,
        });
        return;
      }
      resp({
        errCode: 0,
        data: res,
      });
      return;
    }
    resp({
      errCode: 1000,
      errMsg: "未匹配的key",
      payload: msg,
    });
  }
  function connect() {
    return new Promise((resolve, reject) => {
      const protocol = "wss://";
      const ws = new WebSocket(protocol + FakeAPIServerAddr + "/ws/mp");
      ws.onopen = () => {
        WXU.log({
          msg: "ws/mp connected",
        });
        resolve(true);
      };
      ws.onclose = () => {
        console.log("ws/mp disconnected");
      };
      ws.onerror = (e) => {
        console.error("ws/mp error", e);
        reject(e);
      };
      ws.onmessage = (ev) => {
        const [err, msg] = WXU.parseJSON(ev.data);
        if (err) {
          return;
        }
        if (msg.type === "api_call") {
          handle_api_call(msg.data, ws);
        }
      };
    });
  }
  async function fetchAccountHome(params) {
    return new Promise((resolve) => {
      var query = {
        action: "home",
        __biz: params.biz,
        scene: 124,
      };
      var endpoint =
        window.location.origin +
        "/mp/profile_ext?" +
        Object.keys(query)
          .map((key) => `${key}=${query[key]}`)
          .join("&");
      WXU.log({
        msg: endpoint,
      });
      window.location.href = endpoint;
      resolve([null, endpoint]);
    });
  }
  function render_rss_button() {
    var $btn = document.createElement("div");
    $btn.style.cssText = `position: relative; top: -3px; width: 16px; height: 16px; cursor: pointer;`;
    $btn.innerHTML = RSSIcon;
    $btn.onclick = function () {
      var origin = `${WXU.config.officialRemoteServerProtocol}://${WXU.config.officialRemoteServerHostname}`;
      if (WXU.config.officialRemoteServerPort != 80) {
        origin += `:${WXU.config.officialRemoteServerPort}`;
      }
      var url = `${origin}/rss/mp?biz=${_OfficialAccountCredentials.biz}`;
      WXU.copy(url);
      WXU.toast("RSS 地址已复制");
    };
    return $btn;
  }
  function insert_rss_button() {
    var $wraps = document.querySelectorAll(".wx_follow_media");
    var $container = $wraps[$wraps.length - 1];
    console.log("$container", $container);
    var $btn = render_rss_button();
    $container.appendChild($btn);
  }
  async function main() {
    if (!WXU.config.officialServerDisabled) {
      connect();
    }
    if (location.pathname === "/s") {
      WXU.observe_node(".wx_follow_media", () => {
        setTimeout(() => {
          insert_rss_button();
        }, 1200);
      });
    }
  }
  main();
})();

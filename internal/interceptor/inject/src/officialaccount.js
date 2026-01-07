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
    var origin = `${WXU.config.officialRemoteServerProtocol}://${WXU.config.officialRemoteServerHostname}`;
    if (WXU.config.officialRemoteServerPort != 80) {
      origin += `:${WXU.config.officialRemoteServerPort}`;
    }
    var [err, res] = await WXU.request({
      method: "POST",
      url: `${origin}/api/official_account/refresh?token=${
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

  // function insert_credential_button($container) {
  //   var credentials_text = JSON.stringify(_AccountCredentials);
  //   var $credentials = document.createElement("div");
  //   $credentials.id = "__wx_channels_credentials__";
  //   $credentials.title = "点击复制";
  //   $credentials.onclick = function () {
  //     WXU.copy(credentials_text);
  //     var originalText = $credentials.innerText;
  //     $credentials.innerText = "已复制";
  //     setTimeout(() => {
  //       $credentials.innerText = originalText;
  //     }, 1000);
  //   };
  //   $credentials.innerText = "复制 Credentials";
  //   $container.appendChild($credentials);
  // }
  // function insert_home_url_button($container) {
  //   var $home_url = document.createElement("div");
  //   $home_url.id = "__wx_channels_curl__";
  //   $home_url.title = "点击复制";
  //   var home_url = `https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=${_AccountCredentials.biz}&scene=124&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=${_AccountCredentials.pass_ticket}`;
  //   $home_url.onclick = function () {
  //     WXU.copy(home_url);
  //     var originalText = $home_url.innerText;
  //     $home_url.innerText = "已复制";
  //     setTimeout(() => {
  //       $home_url.innerText = originalText;
  //     }, 1000);
  //   };
  //   $home_url.innerText = "复制 CURL";
  //   $container.appendChild($home_url);
  // }
  // function insert_msg_list_url_button($container) {
  //   var $api = document.createElement("div");
  //   $api.id = "__wx_channels_api__";
  //   $api.title = "点击复制";
  //   var msg_list_url = `https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=${_AccountCredentials.biz}&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&pass_ticket=${_AccountCredentials.pass_ticket}&wxtoken=&appmsg_token=&x5=0&count=10&offset=0&f=json`;
  //   $api.onclick = function () {
  //     WXU.copy(msg_list_url);
  //     var originalText = $api.innerText;
  //     $api.innerText = "已复制";
  //     setTimeout(() => {
  //       $api.innerText = originalText;
  //     }, 1000);
  //   };
  //   $api.innerText = "复制 API";
  //   $container.appendChild($api);
  // }
  // var $container = document.createElement("div");
  // $container.id = "wechat-tools-container";
  // document.body.appendChild($container);

  // var API_PREFIX = "https://mp.weixin.qq.com";

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
    if (key === "/api/official_account/fetch_account_home") {
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
  async function fetchMsgList(params) {
    console.log("fetchMsgList params", params);
    var query = {
      action: "getmsg",
      __biz: params.biz,
      uin: params.uin,
      key: _OfficialAccountCredentials.key,
      pass_ticket: _OfficialAccountCredentials.pass_ticket,
      wxtoken: "",
      appmsg_token: _OfficialAccountCredentials.appmsg_token,
      x5: 0,
      count: params.count ?? 10,
      offset: params.offset ?? 0,
      f: "json",
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
    var resp = await fetch(endpoint, {
      credentials: "include",
    });
    var res = await resp.json();
    console.log("fetchMsgList res", res.data);
    return res;
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

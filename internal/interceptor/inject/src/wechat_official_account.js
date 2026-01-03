(() => {
  // Inject styles
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

  var _AccountCredentials = {
    biz: window.biz || window.__biz,
    uin: window.uin,
    key: window.key,
    pass_ticket: window.pass_ticket,
    appmsg_token: window.appmsg_token,
  };
  WXU.emit(WXU.Events.WechatArticleLoaded, _AccountCredentials);
  var credentials_text = JSON.stringify(_AccountCredentials);

  var $container = document.createElement("div");
  $container.id = "wechat-tools-container";
  document.body.appendChild($container);

  var $credentials = document.createElement("div");
  $credentials.id = "__wx_channels_credentials__";
  $credentials.title = "点击复制";

  $credentials.onclick = function () {
    WXU.copy(credentials_text);
    var originalText = $credentials.innerText;
    var originalText = $curl.innerText;
    $curl.innerText = "已复制";
    setTimeout(() => {
      $curl.innerText = originalText;
    }, 1000);
  };
  $credentials.innerText = "复制 Credentials";
  $container.appendChild($credentials);

  //   curl 'https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=MzI2NDk5NzA0Mw==&scene=124&uin=NzIyNzg0Mjg5&key=daf9bdc5abc4e8d024c44b5efab2e731529799575615bc0f3779046d2e448a5fbfc8116ff62eed74c88ca6a8d2f939a537357eedb5d2dbb6429c56a95d7622dba0fdd2127fa83d83026f13b21bb4dd9898ebd1b942ec07bfeed8227610582aa6ba7fa0f9885a8c65696c5eff942b63612c701cdaf3aaf403516892b09ef84007&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=B1y03MI%2FZ0cmvxk5CbeZzFOmTluTZ4Jf7qqiCdF03vEV2rcwE%2Bf7ZoqXefD1hQlx' \
  //   -H 'accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7' \
  //   -H 'accept-language: en-US,en;q=0.9' \
  //   -H 'cache-control: max-age=0' \
  //   -b 'wxuin=722784289; lang=zh_CN; devicetype=android-29; version=28004252; pass_ticket=KRt7o2719r0AAXY6cqj5cHC9upd9CfwDFX5pgsRkjETR/gQaB70IJWNnwojs/2C; wap_sid2=CKGg09gCEooBeV9ISWN0SzZsQ0RuVEJJamRyMVRMV1ZfTlFyamc4WGZrNU9GbzBsYTZMeFVhQ3c3NlNhWG9RU0kwQkhPeUNQREk4UU9PUXhTR1JycDk5b2FYZTdCcVRKYW1BTXBPTUkySHczUzl4X0labUZnaHlBejA0ZHVVN2JrT010dzZiYXY3aTN5Z1RBQUF+MP/R38oGOA1AlU4=' \
  //   -H 'priority: u=0, i' \
  //   -H 'referer: https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=MzI2NDk5NzA0Mw==&scene=124&uin=NzIyNzg0Mjg5&key=daf9bdc5abc4e8d0ab3e68569bb12245a3296216bc0eae50ebe69fc80045755463f628a455f83131b3883d750897b91fedbe2bf7e0921959d20f14beef576cad06f0ce2bf01836180b24a26e721bd03165207c70fda4a658daf548febfb908045d02813d9c0304400b0cd6c451699bb1694d1bb84f2a76c226175c82c791f7da&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=KRt7o2719r0AAXY6cqj5cHC9upd9CfwDFX5pgsRkjETR%2FgQa%2BB70IJWNnwojs%2F2C' \
  //   -H 'sec-fetch-dest: document' \
  //   -H 'sec-fetch-mode: navigate' \
  //   -H 'sec-fetch-site: none' \
  //   -H 'upgrade-insecure-requests: 1' \
  //   -H 'user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue'
  var $curl = document.createElement("div");
  $curl.id = "__wx_channels_curl__";
  $curl.title = "点击复制";

  var wxuin = "";
  try {
    wxuin = atob(_AccountCredentials.uin);
  } catch (e) {
    console.error("Failed to decode uin", e);
  }

  var wap_sid2 = "";
  var match = document.cookie.match(/wap_sid2=([^;]+)/);
  if (match) {
    wap_sid2 = match[1];
  }

  var targetUrl = `https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=${_AccountCredentials.biz}&scene=124&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=${_AccountCredentials.pass_ticket}`;

  var curl_text = `curl '${targetUrl}' \\
  -H 'accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7' \\
  -H 'accept-language: en-US,en;q=0.9' \\
  -H 'cache-control: max-age=0' \\
  -b 'wxuin=${wxuin}; lang=zh_CN; devicetype=android-29; version=28004252; pass_ticket=${_AccountCredentials.pass_ticket}; wap_sid2=${wap_sid2}' \\
  -H 'priority: u=0, i' \\
  -H 'referer: ${targetUrl}' \\
  -H 'sec-fetch-dest: document' \\
  -H 'sec-fetch-mode: navigate' \\
  -H 'sec-fetch-site: none' \\
  -H 'upgrade-insecure-requests: 1' \\
  -H 'user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue'`;

  $curl.onclick = function () {
    WXU.copy(curl_text);
    var originalText = $curl.innerText;
    $curl.innerText = "已复制";
    setTimeout(() => {
      $curl.innerText = originalText;
    }, 1000);
  };
  $curl.innerText = "复制 CURL";
  $container.appendChild($curl);

  //   https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=MzI2NDk5NzA0Mw==&uin=NzIyNzg0Mjg5&key=daf9bdc5abc4e8d05e47caccaebcf6909badd928bd0b683914818f2ca40f5a86eab1665701a978c1ab058c4fa31f4ab97ed66b63a4b9027f6c72f20a60d8e97c9e9c68c667ad534815bd594d4afa2d7f3be1e2ccab3b42c141caf167a35d0f689138447980f87ad03e00ca5046cbdc9d9ae64729e1f5b51d289cc05d79d85c4f&pass_ticket=iP%2BaAr4MU15FR5JXi6pwetcx8cc9xuhukJ6dV20bSXw3UcPBttNEcJshLgow%2BE0G&wxtoken=&appmsg_token=1355_YzQa3bgKNZJNQKMl9f1P5XvRMmVE2oyzmICVsA~~&x5=0&count=10&offset=0&f=json
  var $api = document.createElement("div");
  $api.id = "__wx_channels_api__";
  $api.title = "点击复制";

  var apiUrl = `https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=${_AccountCredentials.biz}&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&pass_ticket=${_AccountCredentials.pass_ticket}&wxtoken=&appmsg_token=${_AccountCredentials.appmsg_token}&x5=0&count=10&offset=0&f=json`;

  $api.onclick = function () {
    WXU.copy(apiUrl);
    var originalText = $api.innerText;
    $api.innerText = "已复制";
    setTimeout(() => {
      $api.innerText = originalText;
    }, 1000);
  };
  $api.innerText = "复制 API";
  $container.appendChild($api);

  var API_PREFIX = "https://mp.weixin.qq.com";

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

    if (key === "/api/official_account/fetch_msg_list") {
      try {
        const res = await fetchMsgList(data);
        resp({
          errCode: 0,
          data: res,
        });
      } catch (err) {
        resp({
          errCode: 1001,
          errMsg: err.message,
        });
      }
      return;
    }
    if (key === "/api/official_account/fetch_account_home") {
      try {
        const res = await fetchAccountHome(data);
        resp({
          errCode: 0,
          data: res,
        });
      } catch (err) {
        resp({
          errCode: 1001,
          errMsg: err.message,
        });
      }
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
      const protocol = "ws://";
      const pathname = WXU.config.apiServerAddr;
      const ws = new WebSocket(protocol + pathname + "/ws/mp");

      ws.onopen = () => {
        console.log("ws/mp connected");
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
  connect();

  async function fetchMsgList(params) {
    console.log("fetchMsgList params", params);
    var query = {
      action: "getmsg",
      __biz: params.biz,
      uin: params.uin,
      key: _AccountCredentials.key,
      pass_ticket: _AccountCredentials.pass_ticket,
      wxtoken: "",
      appmsg_token: _AccountCredentials.appmsg_token,
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
    console.log("fetchAccountHome params", params);
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
    var resp = await fetch(endpoint, {
      credentials: "include",
    });
    console.log("fetchAccountHome res", resp.data);
    return resp;
  }
})();

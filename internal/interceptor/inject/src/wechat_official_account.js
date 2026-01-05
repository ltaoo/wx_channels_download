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

  var _AccountCredentials = {
    nickname: window.nickname,
    avatar_url: window.headimg,
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
    $credentials.innerText = "已复制";
    setTimeout(() => {
      $credentials.innerText = originalText;
    }, 1000);
  };
  $credentials.innerText = "复制 Credentials";
  $container.appendChild($credentials);

  var $home_url = document.createElement("div");
  $home_url.id = "__wx_channels_curl__";
  $home_url.title = "点击复制";

  var home_url = `https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=${_AccountCredentials.biz}&scene=124&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=${_AccountCredentials.pass_ticket}`;

  $home_url.onclick = function () {
    WXU.copy(home_url);
    var originalText = $home_url.innerText;
    $home_url.innerText = "已复制";
    setTimeout(() => {
      $home_url.innerText = originalText;
    }, 1000);
  };
  $home_url.innerText = "复制 CURL";
  $container.appendChild($home_url);

  var $api = document.createElement("div");
  $api.id = "__wx_channels_api__";
  $api.title = "点击复制";
  var msg_list_url = `https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=${_AccountCredentials.biz}&uin=${_AccountCredentials.uin}&key=${_AccountCredentials.key}&pass_ticket=${_AccountCredentials.pass_ticket}&wxtoken=&appmsg_token=&x5=0&count=10&offset=0&f=json`;
  $api.onclick = function () {
    WXU.copy(msg_list_url);
    var originalText = $api.innerText;
    $api.innerText = "已复制";
    setTimeout(() => {
      $api.innerText = originalText;
    }, 1000);
  };
  $api.innerText = "复制 API";
  $container.appendChild($api);
})();

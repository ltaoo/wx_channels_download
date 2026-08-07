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
    body.wx-officialaccount-download-menu-mounted .t1-popper {
      z-index: 900 !important;
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
function insert_style() {
  document.head.appendChild(style);
}

function build_refresh_uri() {
  var current_url = window.location.href || "";
  var page_link = first_non_empty(get_page_data_value("link"));
  var biz = first_non_empty(
    get_url_param(current_url, "__biz"),
    get_url_param(page_link, "__biz"),
  );
  var mid = first_non_empty(
    get_url_param(current_url, "mid"),
    get_url_param(page_link, "mid"),
  );
  var idx = first_non_empty(
    get_url_param(current_url, "idx"),
    get_url_param(page_link, "idx"),
    "1",
  );
  var sn = first_non_empty(
    get_url_param(current_url, "sn"),
    get_url_param(page_link, "sn"),
  );
  if (biz && mid && sn) {
    return `https://mp.weixin.qq.com/s?__biz=${encodeURIComponent(biz)}&mid=${encodeURIComponent(mid)}&idx=${encodeURIComponent(idx)}&sn=${encodeURIComponent(sn)}`;
  }
  return first_non_empty(page_link, current_url);
}
function get_official_account_biz() {
  var refresh_uri = build_refresh_uri();
  return first_non_empty(
    window.biz,
    window.__biz,
    get_page_data_value("bizuin"),
    get_url_param(window.location.href, "__biz"),
    get_url_param(get_page_data_value("link"), "__biz"),
    get_url_param(refresh_uri, "__biz"),
  );
}
async function submit_credential(acct) {
  if (!acct.biz || !acct.key) {
    return;
  }
  WXU.emit(WXU.Events.OfficialAccountRefresh, acct);
  var r = await submitCredentialReq.run(acct);
  if (r.error) {
    WXU.error({ msg: r.error.message });
  }
}

async function fetchAccountHome(params) {
  // console.log("[]fetchAccountHome", params);
  return new Promise((resolve) => {
    window.location.href = params.refresh_uri;
    resolve([null, params.refresh_uri]);
  });
}

function render_rss_button(acct) {
  var $btn = document.createElement("div");
  $btn.style.cssText = `position: relative; top: -3px; width: 16px; height: 16px; margin-left: 6px; cursor: pointer;`;
  $btn.innerHTML = Icons.RSSIcon;
  $btn.onclick = function () {
    var origin = (() => {
      return WXEnv.officialAccountOrigin;
    })();
    if (origin === "") {
      return;
    }
    var url = `${origin}/rss/mp?biz=${acct.biz}`;
    WXU.copy(url);
    WXU.toast("RSS 地址已复制");
  };
  return $btn;
}
function insert_rss_button(acct) {
  if (!acct.biz || !acct.key) {
    return;
  }
  var $wraps = document.querySelectorAll(".wx_follow_media");
  var $container = $wraps[$wraps.length - 1];
  // console.log("$container", $container);
  var $btn = render_rss_button(acct);
  $container.appendChild($btn);
}

function buildCredentials() {
  var _OfficialAccountCredentials = {
    nickname: (() => {
      if (window.nickname) {
        return window.nickname;
      }
      if (window.cgiData) {
        if (window.cgiData.nick_name) {
          return window.cgiData.nick_name;
        }
      }
      if (window.cgiDataNew) {
        if (window.cgiDataNew.nick_name) {
          return window.cgiDataNew.nick_name;
        }
      }
      return "";
    })(),
    avatar_url: (() => {
      if (window.headimg) {
        return window.headimg;
      }
      if (window.cgiData) {
        if (window.cgiData.round_head_img) {
          return window.cgiData.round_head_img;
        }
        if (window.cgiData.hd_head_img) {
          return window.cgiData.hd_head_img;
        }
      }
      if (window.cgiDataNew) {
        if (window.cgiDataNew.round_head_img) {
          return window.cgiDataNew.round_head_img;
        }
        if (window.cgiDataNew.hd_head_img) {
          return window.cgiDataNew.hd_head_img;
        }
      }
      return "";
    })(),
    biz: get_official_account_biz(),
    author_id: first_non_empty(
      window.author_id,
      window.authorId,
      get_page_data_value("author_id"),
      get_page_data_value("user_name"),
    ),
    uin: first_non_empty(window.uin, get_page_data_value("user_uin")),
    key: window.key,
    refresh_uri: build_refresh_uri(),
    pass_ticket: window.pass_ticket,
    appmsg_token: window.appmsg_token,
  };
}

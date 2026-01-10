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
  function insert_style() {
    document.head.appendChild(style);
  }
  async function submit_credential(acct) {
    if (!acct.biz || !acct.key) {
      return;
    }
    WXU.emit(WXU.Events.OfficialAccountRefresh, acct);
    var origin = `https://${FakeAPIServerAddr}`;
    var [err, res] = await WXU.request({
      method: "POST",
      url: `${origin}/api/mp/refresh?token=${
        WXU.config.officialServerRefreshToken ?? ""
      }`,
      body: acct,
    });
    if (err) {
      WXU.error({
        msg: err.message,
      });
      return;
    }
  }

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
  function connect(acct) {
    return new Promise((resolve, reject) => {
      const protocol = "wss://";
      const ws = new WebSocket(protocol + FakeAPIServerAddr + "/ws/mp");
      let ping_timer = null;
      ws.onopen = () => {
        WXU.log({
          msg: "ws/mp connected",
        });
        submit_credential(acct);
        var page_title = document.title || acct.nickname || "公众号页面";
        try {
          ws.send(
            JSON.stringify({
              type: "ping",
              data: page_title,
            })
          );
        } catch (e) {
          // ...
        }
        ping_timer = setInterval(() => {
          console.log("[]ping");
          if (ws.readyState === 1) {
            try {
              ws.send(
                JSON.stringify({
                  type: "ping",
                  data: page_title,
                })
              );
            } catch (e) {
              // ...
            }
          }
        }, 5 * 1000);
        resolve(true);
      };
      ws.onclose = () => {
        console.log("ws/mp disconnected");
        if (ping_timer) {
          clearInterval(ping_timer);
          ping_timer = null;
        }
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
    console.log("[]fetchAccountHome", params);
    return new Promise((resolve) => {
      window.location.href = params.refresh_uri;
      resolve([null, params.refresh_uri]);
    });
  }
  function render_rss_button(acct) {
    var $btn = document.createElement("div");
    $btn.style.cssText = `position: relative; top: -3px; width: 16px; height: 16px; margin-left: 6px; cursor: pointer;`;
    $btn.innerHTML = RSSIcon;
    $btn.onclick = function () {
      var origin = `${WXU.config.officialRemoteServerProtocol}://${WXU.config.officialRemoteServerHostname}`;
      if (WXU.config.officialRemoteServerPort != 80) {
        origin += `:${WXU.config.officialRemoteServerPort}`;
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
    console.log("$container", $container);
    var $btn = render_rss_button(acct);
    $container.appendChild($btn);
  }
  insert_style();
  WXU.onWindowLoaded(() => {
    async function main() {
      if (location.pathname === "/s") {
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
          biz: window.biz || window.__biz,
          uin: window.uin,
          key: window.key,
          refresh_uri: (() => {
            const params = new URLSearchParams(window.location.search);
            const biz = params.get("__biz");
            const mid = params.get("mid");
            const idx = params.get("idx");
            const sn = params.get("sn");
            return `https://mp.weixin.qq.com/s?__biz=${biz}&mid=${mid}&idx=${idx}&sn=${sn}`;
          })(),
          pass_ticket: window.pass_ticket,
          appmsg_token: window.appmsg_token,
        };
        if (!WXU.config.officialServerDisabled) {
          connect(_OfficialAccountCredentials);
        }
        WXU.observe_node(".wx_follow_media", () => {
          setTimeout(() => {
            insert_rss_button(_OfficialAccountCredentials);
          }, 1200);
        });
      }
    }
    main();
  });
})();

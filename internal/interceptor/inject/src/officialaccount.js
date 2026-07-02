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
    body.wx-officialaccount-download-menu-mounted .t1-popper {
      z-index: 2147483647 !important;
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
    var origin = WXEnv.apiOrigin;
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
        }),
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
      const ws = new WebSocket(WXEnv.mpWSURL);
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
            }),
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
                }),
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
  async function create_officialaccount_download_task(dialog$) {
    var [err, data] = await WXU.request({
      method: "POST",
      url: WXEnv.apiOrigin + "/api/task/create2",
      body: {
        url: `officialaccount://${window.location.href}`,
        // filename: document.title,
      },
    });
    if (err) {
      WXU.error({
        msg: err.message,
      });
      return;
    }
    // WXU.toast("开始下载");
    dialog$.show();
  }
  function render_download_button() {
    var $btn = document.createElement("div");
    $btn.className = "sns_opr_btn_con";
    $btn.innerHTML = `<button aria-labelledby="__wx_download_bottom_text" class="sns_opr_btn sns_write_comment_btn bar-expand-hotarea js_wx_tap_highlight wx_tap_link"><span id="__wx_download_bottom_text" class="sns_opr_gap"> 下载 </span></button>`;
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
  function DownloaderPanel(props) {
    const vm$ = DownloaderPanelViewModel({
      onRequestClose() {
        props.dialog$.hide();
      },
    });
    return View({}, [
      Dialog({ store: props.dialog$ }, [
        DownloaderPanelView({
          store: vm$,
          showStatusCounts: false,
        }),
      ]),
    ]);
  }
  function insert_download_button() {
    insert_style();
    document.body.classList.add("wx-officialaccount-download-menu-mounted");
    var $wraps = document.querySelectorAll(".interaction_bar");
    var $container = $wraps[$wraps.length - 1];
    if (window.cgiDataNew.page_type === 2) {
      $container = $wraps[0];
    }
    if (!$container || !$container.lastElementChild) {
      return;
    }
    const dialog$ = new Timeless.ui.DialogCore({
      offsetY: 4,
    });
    var $btn = render_download_button();
    const dropdown$ = new Timeless.ui.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: [
        new Timeless.ui.MenuItemCore({
          label: "\u4e0b\u8f7d\u6587\u7ae0",
          async onClick() {
            dropdown$.hide();
            await create_officialaccount_download_task(dialog$);
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "\u590d\u5236\u6587\u7ae0HTML",
          onClick() {
            const content = window.cgiDataNew.content_noencode;
            if (!content) {
              WXU.toast(
                "\u6587\u7ae0HTML\u4e3a\u7a7a\uff0c\u8bf7\u4f7f\u7528\u300c\u590d\u5236\u9875\u9762HTML\u300d",
              );
              return;
            }
            WXU.copy(content);
            WXU.toast("\u590d\u5236\u6210\u529f");
            dropdown$.hide();
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "\u590d\u5236\u9875\u9762HTML",
          onClick() {
            const content = window.body.innerHTML;
            WXU.copy(content);
            WXU.toast("\u590d\u5236\u6210\u529f");
            dropdown$.hide();
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "\u4e0b\u8f7d\u8bb0\u5f55",
          onClick() {
            dialog$.show();
            dropdown$.hide();
          },
        }),
      ],
    });
    const dropdownRoot = document.createElement("span");
    dropdownRoot.className = "wx-download-dropdown-menu-root";
    dropdownRoot.style.display = "contents";
    document.body.appendChild(dropdownRoot);
    Timeless.DOM.render(
      Timeless.shadcn.DropdownMenu({ store: dropdown$ }),
      dropdownRoot,
    );
    function set_dropdown_reference() {
      dropdown$.setReference(
        {
          $el: $btn,
          getRect() {
            return $btn.getBoundingClientRect();
          },
        },
        { force: true },
      );
    }
    function show_dropdown() {
      set_dropdown_reference();
      dropdown$.handleEnterTrigger();
    }
    function hide_dropdown() {
      dropdown$.handleLeaveTrigger();
    }
    async function handle_download_click(event) {
      event.preventDefault();
      event.stopPropagation();
      dropdown$.hide({ reason: "download button click" });
      await create_officialaccount_download_task(dialog$);
    }
    $btn.addEventListener("mouseenter", show_dropdown);
    $btn.addEventListener("mouseleave", hide_dropdown);
    $btn.addEventListener("click", handle_download_click);
    $btn.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });
    $container.insertBefore($btn, $container.lastElementChild);
    Timeless.DOM.render(DownloaderPanel({ dialog$ }), document.body);
  }
  window.insert_download_button = insert_download_button;
  async function main() {
    if (location.pathname.startsWith("/s")) {
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
      WXU.observe_node(".wx_follow_media", () => {
        setTimeout(() => {
          // insert_style();
          // insert_rss_button(_OfficialAccountCredentials);
          insert_download_button();
        }, 800);
      });
    }
  }
  WXU.onWindowLoaded(() => {
    if (!WXU.config.officialAccountEnabled) {
      return;
    }
    main();
  });
})();

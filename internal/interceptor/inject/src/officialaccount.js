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
  async function create_officialaccount_download_task(popover$, $btn) {
    var body = {
      url: `officialaccount://${window.location.href}`,
      extra: {},
    };
    if (
      window.cgiDataNew &&
      window.cgiDataNew.create_time
    ) {
      body.extra.created_at = window.cgiDataNew.create_time;
    }
    var [err, data] = await WXU.request({
      method: "POST",
      url: WXEnv.apiOrigin + "/api/task/create2",
      body: body,
    });
    if (err) {
      WXU.error({
        msg: err.message,
      });
      return;
    }

    popover$.show(popover_pos($btn));
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
        props.popover$.hide();
      },
    });
    return View({}, [
      Popover(
        {
          store: props.popover$,
          content: [
            DownloaderPanelView({
              store: vm$,
              showStatusCounts: false,
            }),
          ],
        },
        [props.btn$],
      ),
      TaskDeleteConfirmDialog({
        store: vm$,
      }),
      ClearTasksConfirmDialog({
        store: vm$,
      }),
      OverwriteDownloadConfirmDialog({
        store: vm$,
      }),
    ]);
  }
  function MsgListPanel(props) {
    const { dialog$ } = props;
    const biz = window.biz || window.__biz || "";
    const token = WXU.config.officialServerRefreshToken ?? "";
    const uin = window.uin || "";
    const key = window.key || "";
    const passTicket = window.pass_ticket || "";
    const origin = WXEnv.apiOrigin;

    let currentOffset = 0;
    let loading = false;
    let msgList = [];
    let canLoadMore = true;
    const pageSize = 10;

    const container = document.createElement("div");
    container.className = "wx-dl-panel-container";
    container.style.cssText = "width: 400px; max-height: 512px;";

    const header = document.createElement("div");
    header.className = "wx-dl-header";
    const heading = document.createElement("div");
    heading.className = "wx-dl-heading";
    const title = document.createElement("div");
    title.className = "wx-dl-title";
    title.textContent = "推送列表";
    heading.appendChild(title);
    header.appendChild(heading);
    const closeBtn = document.createElement("div");
    closeBtn.className = "wx-dl-more-btn";
    closeBtn.style.cssText =
      "cursor: pointer; font-size: 18px; padding: 4px 8px; line-height: 1;";
    closeBtn.textContent = "✕";
    closeBtn.onclick = () => dialog$.hide();
    header.appendChild(closeBtn);
    container.appendChild(header);

    const listEl = document.createElement("div");
    listEl.className = "wx-dl-dark-scroll";
    listEl.style.cssText =
      "display: flex; flex-direction: column; gap: 8px; padding: 0 12px; overflow-y: auto; flex: 1; min-height: 0;";
    container.appendChild(listEl);

    const loadMoreBtn = document.createElement("button");
    loadMoreBtn.textContent = "加载更多";
    loadMoreBtn.style.cssText =
      "margin: 12px; padding: 8px 16px; border: 1px solid var(--weui-FG-6, #eee); border-radius: 4px; background: var(--popup-content-bg-color, #f7f7f7); color: var(--weui-FG-0); cursor: pointer; width: calc(100% - 24px); font-size: 13px; flex-shrink: 0;";
    loadMoreBtn.onclick = () => fetchList();
    container.appendChild(loadMoreBtn);

    function renderItem(item) {
      const el = document.createElement("div");
      el.style.cssText =
        "padding: 10px 12px; border-radius: 6px; background: var(--popup-content-bg-color, var(--weui-BG-2, #f7f7f7));";
      const msgInfo = item.app_msg_ext_info || {};
      const title = msgInfo.title || "无标题";
      const digest = msgInfo.digest || "";
      const link = msgInfo.content_url || "";
      const time = item.comm_msg_info?.datetime
        ? new Date(item.comm_msg_info.datetime * 1000).toLocaleString()
        : "";
      el.innerHTML = `
        <div style="font-size: 14px; font-weight: 500; margin-bottom: 4px; color: var(--weui-FG-0);">
          ${link ? `<a href="${escapeHtml(link)}" target="_blank" style="color: inherit; text-decoration: none;">${escapeHtml(unescapeHtml(title))}</a>` : escapeHtml(unescapeHtml(title))}
        </div>
        ${digest ? `<div style="font-size: 12px; color: var(--weui-FG-1, #888); margin-bottom: 4px;">${unescapeHtml(digest)}</div>` : ""}
        ${time ? `<div style="font-size: 11px; color: var(--weui-FG-1, #aaa);">${time}</div>` : ""}
      `;
      return el;
    }

    function escapeHtml(str) {
      const div = document.createElement("div");
      div.textContent = str;
      return div.innerHTML;
    }

    function unescapeHtml(str) {
      const div = document.createElement("div");
      div.innerHTML = str;
      return div.textContent;
    }

    async function fetchList() {
      if (loading) return;
      loading = true;
      loadMoreBtn.textContent = "加载中...";
      loadMoreBtn.disabled = true;
      const url = `${origin}/api/mp/msg/list?biz=${encodeURIComponent(biz)}&offset=${currentOffset}&count=${pageSize}&token=${encodeURIComponent(token)}&uin=${encodeURIComponent(uin)}&key=${encodeURIComponent(key)}&pass_ticket=${encodeURIComponent(passTicket)}`;
      const [err, res] = await WXU.request({ method: "GET", url });
      loading = false;
      loadMoreBtn.disabled = false;
      if (err) {
        WXU.error({ msg: "获取推送列表失败: " + err.message });
        loadMoreBtn.textContent = "重试";
        return;
      }
      const data = res.data || res;
      const rawList = data.general_msg_list || "";
      let list = [];
      if (rawList) {
        try {
          const parsed =
            typeof rawList === "string" ? JSON.parse(rawList) : rawList;
          list = parsed.list || [];
        } catch (e) {
          list = [];
        }
      }
      if (list.length === 0 || list.length < pageSize) {
        canLoadMore = false;
        loadMoreBtn.textContent = "没有更多了";
        loadMoreBtn.disabled = true;
        if (list.length === 0) return;
      }
      msgList = msgList.concat(list);
      list.forEach((item) => {
        listEl.appendChild(renderItem(item));
      });
      if (data.next_offset !== undefined) {
        currentOffset = data.next_offset;
      } else {
        currentOffset += list.length;
      }
      if (canLoadMore) {
        loadMoreBtn.textContent = "加载更多";
      }
    }

    dialog$.onStateChange((state) => {
      if (state.visible && msgList.length === 0) {
        fetchList();
      }
    });

    return container;
  }
  function popover_pos($btn) {
    const { x, y, width } = $btn.getBoundingClientRect();
    return {
      x: x + width,
      y: y - 48,
    };
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
    const popover$ = new Timeless.ui.PopoverCore({
      offsetY: 4,
      destroyOnClose: false,
    });
    const msgListDialog$ = new Timeless.ui.DialogCore({
      offsetY: 4,
    });
    const elm = View({ class: "sns_opr_btn_con" }, [
      View(
        {
          as: "button",
          attributes: {
            "aria-labelledby": "__wx_download_bottom_text",
          },
          class:
            "sns_opr_btn sns_write_comment_btn bar-expand-hotarea js_wx_tap_highlight wx_tap_link",
        },
        [
          View(
            {
              as: "span",
              attributes: { id: "__wx_download_bottom_text" },
              class: "sns_opr_gap",
            },
            ["下载"],
          ),
        ],
      ),
    ]);
    var vnode = Timeless.DOM.build(elm);
    const $btn = vnode.render(elm);
    const dropdown$ = new Timeless.ui.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: [
        new Timeless.ui.MenuItemCore({
          label: "复制文章HTML",
          onClick() {
            const content = window.cgiDataNew.content_noencode;
            if (!content) {
              WXU.toast("文章HTML为空，请使用「复制页面HTML」");
              return;
            }
            WXU.copy(content);
            WXU.toast("复制成功");
            dropdown$.hide();
          },
        }),
        new Timeless.ui.MenuItemCore({
          label: "复制页面HTML",
          onClick() {
            const content = window.body.innerHTML;
            WXU.copy(content);
            WXU.toast("复制成功");
            dropdown$.hide();
          },
        }),
        ...(WXEnv.isWeChatBrowser
          ? [
              new Timeless.ui.MenuItemCore({
                label: "推送列表",
                onClick() {
                  msgListDialog$.show();
                  dropdown$.hide();
                },
              }),
              new Timeless.ui.MenuItemCore({
                label: "下载所有推送",
                onClick() {
                  const biz = window.biz || window.__biz || "";
                  const token = WXU.config.officialServerRefreshToken ?? "";
                  const uin = window.uin || "";
                  const key = window.key || "";
                  const passTicket = window.pass_ticket || "";
                  if (!biz) {
                    WXU.error("缺少 biz 参数");
                    return;
                  }
                  WXU.toast("正在提交批量下载...");
                  var origin = WXEnv.apiOrigin;
                  WXU.request({
                    method: "POST",
                    url: `${origin}/api/mp/download_all`,
                    body: { biz, uin, key, pass_ticket: passTicket, token },
                  });
                  dropdown$.hide();
                  popover$.show(popover_pos($btn));
                },
              }),
            ]
          : []),
        new Timeless.ui.MenuItemCore({
          label: "下载面板",
          onClick() {
            popover$.show(popover_pos($btn));
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
      Timeless.DropdownMenu({ store: dropdown$ }),
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
      await create_officialaccount_download_task(popover$, $btn);
    }
    $btn.addEventListener("mouseenter", show_dropdown);
    $btn.addEventListener("mouseleave", hide_dropdown);
    $btn.addEventListener("click", handle_download_click);
    $btn.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });
    $container.insertBefore($btn, $container.lastElementChild);
    Timeless.DOM.render(
      DownloaderPanel({ popover$, btn$: vnode }),
      document.body,
    );
    // 推送列表面板
    const msgListPanel = MsgListPanel({ dialog$: msgListDialog$ });
    const msgListOverlay = document.createElement("div");
    msgListOverlay.style.cssText =
      "display: none; position: fixed; inset: 0; z-index: 10000; background: rgba(0,0,0,0.5); justify-content: center; align-items: center;";
    msgListOverlay.appendChild(msgListPanel);
    msgListOverlay.addEventListener("click", (e) => {
      if (e.target === msgListOverlay) msgListDialog$.hide();
    });
    document.body.appendChild(msgListOverlay);
    msgListDialog$.onStateChange((state) => {
      msgListOverlay.style.display = state.visible ? "flex" : "none";
    });
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
      var __download_btn_inserted = false;
      WXU.observe_node(".interaction_bar", () => {
        if (__download_btn_inserted) {
          return;
        }
        __download_btn_inserted = true;
        insert_download_button();
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

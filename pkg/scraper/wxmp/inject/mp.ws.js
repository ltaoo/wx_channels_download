(() => {
  var APIHostname = WXEnv.apiOrigin;

  const http_client = new Timeless.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: APIHostname,
  });
  Timeless.web.provide_http_client(http_client);
  const request = Timeless.request_factory({
    headers: { "Content-Type": "application/json" },
    process(r) {
      if (r.error) {
        return Timeless.Result.Err(r.error);
      }
      const { code, msg, data } = r.data;
      if (code !== 0) {
        return Timeless.Result.Err(msg, code, data);
      }
      return Timeless.Result.Ok(data);
    },
  });

  const submitCredentialReq = new Timeless.RequestCore(
    (acct) =>
      request.post(
        "/api/mp/refresh?token=" +
          (WXU.config.officialServerRefreshToken ?? ""),
        acct,
      ),
    { client: http_client },
  );

  const msgListReq = new Timeless.RequestCore(
    (params) => request.get("/api/mp/msg/list", params),
    { client: http_client },
  );

  const downloadAllReq = new Timeless.RequestCore(
    (body) => request.post("/api/mp/download_all", body),
    { client: http_client },
  );

  function first_non_empty() {
    for (var i = 0; i < arguments.length; i++) {
      var value = arguments[i];
      if (value === undefined || value === null) {
        continue;
      }
      value = String(value).trim();
      if (value) {
        return value;
      }
    }
    return "";
  }
  function get_page_data_value(key) {
    if (window.cgiDataNew && window.cgiDataNew[key] !== undefined) {
      return window.cgiDataNew[key];
    }
    if (window.cgiData && window.cgiData[key] !== undefined) {
      return window.cgiData[key];
    }
    return "";
  }
  function get_url_param(raw_url, key) {
    if (!raw_url) {
      return "";
    }
    try {
      return new URL(raw_url, window.location.href).searchParams.get(key) || "";
    } catch {
      return "";
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

  function report_article_loaded() {
    if (!window.cgiDataNew || window.__wxmp_article_reported__) {
      return;
    }
    window.__wxmp_article_reported__ = true;
    const article = window.cgiDataNew;
    const articles = [article];
    WXU.log
      .Info()
      .Str("file", "mp.ws.js")
      .JSON("article", { title: article.title })
      .Msg("before downloader.browse");
    WXU.downloader.browse(articles, { platform: "wxmp" });
  }

  async function create_download_task(popover$, $btn) {
    WXU.log.Info().Msg("[mp.wx.js]create_download_task");
    const [error, data] = await WXU.downloader.create([window.cgiDataNew], {
      platform: "wxmp",
    });
    if (error) {
      WXU.log.Error(error).Msg("[mp.wx.js]create failed");
      WXU.error({ msg: error.message });
      return;
    }
    WXU.log.Info().Msg("[mp.wx.js]create success");
    var taskResult = data && data.tasks && data.tasks[0];
    if (taskResult && !taskResult.success) {
      WXU.error({ msg: taskResult.error || "创建下载任务失败" });
      return;
    }
    popover$.show(popover_pos($btn));
  }

  function DownloaderEntry(props) {
    const vm$ =
      typeof downloadermodel$ !== undefined
        ? downloadermodel$
        : DownloaderPanelViewModel({});
    return Button(
      {
        attributes: {
          "aria-labelledby": "__wx_download_bottom_text",
        },
        class:
          "sns_opr_btn sns_write_comment_btn __wx_download_btn bar-expand-hotarea js_wx_tap_highlight wx_tap_link",
        onMounted() {
          if (props.onMounted) {
            props.onMounted(vm$);
          }
        },
        onClick(event) {
          // console.log("before props.onClick");
          props.onClick(event);
        },
      },
      [
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
          [
            View(
              {
                attributes: { id: "__wx_download_bottom_text" },
                class: "sns_opr_gap",
                style: { display: "inline" },
              },
              ["下载"],
            ),
          ],
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
      ],
    );
  }
  function MsgListPanel(props) {
    const { dialog$ } = props;
    const biz = window.biz || window.__biz || "";
    const token = WXU.config.officialServerRefreshToken ?? "";
    const uin = window.uin || "";
    const key = window.key || "";
    const passTicket = window.pass_ticket || "";

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
      var r = await msgListReq.run({
        biz: biz,
        offset: currentOffset,
        count: pageSize,
        token: token,
        uin: uin,
        key: key,
        pass_ticket: passTicket,
      });
      loading = false;
      loadMoreBtn.disabled = false;
      if (r.error) {
        WXU.error({ msg: "获取推送列表失败: " + r.error.message });
        loadMoreBtn.textContent = "重试";
        return;
      }
      const data = r.data || {};
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
    document.body.classList.add("wx-officialaccount-download-menu-mounted");
    var $wraps = document.querySelectorAll(".interaction_bar");
    var $container = $wraps[$wraps.length - 1];
    const no_container = !$container || !$container.lastElementChild;
    console.log("[mp.ws.js]before if (no_container", $container, no_container);
    if (no_container) {
      return;
    }
    const popover$ = new Timeless.ui.PopoverCore({
      offsetY: 4,
      destroyOnClose: false,
    });
    const msgListDialog$ = new Timeless.ui.DialogCore({
      offsetY: 4,
    });
    WXU.downloader.show = function() {
      popover$.show();
    }
    WXU.downloader.hide = function() {
      popover$.hide();
    }
    // Create button container and insert into page (following panel.js pattern: insert DOM element first, then render VDOM into it)
    const $btn = document.createElement("div");
    $btn.className = "sns_opr_btn_con";
    $container.insertBefore($btn, $container.lastElementChild);
    console.log("before render(DownloaderEntry");
    // Render download panel entry into button container
    Timeless.DOM.render(
      DownloaderEntry({
        popover$,
        onClick: handle_download_click,
      }),
      $btn,
    );
    // Replace :before mask-image with download icon (line style)
    (function () {
      var s = Icons.download_icon9;
      var st = document.createElement("style");
      st.textContent =
        ".__wx_download_btn.sns_write_comment_btn:before { mask-image: url(data:image/svg+xml," +
        encodeURIComponent(s) +
        ") !important; }";
      document.head.appendChild(st);
    })();
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
                  downloadAllReq.run({
                    biz,
                    uin,
                    key,
                    pass_ticket: passTicket,
                    token,
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
      Timeless.weui.DropdownMenu({ store: dropdown$ }),
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
      // console.log("handle_download_click");
      await create_download_task(popover$, $btn);
    }
    $btn.addEventListener("mouseenter", show_dropdown);
    $btn.addEventListener("mouseleave", hide_dropdown);
    // $btn.addEventListener("click", handle_download_click);
    $btn.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });
    // Push message list panel
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
  async function main() {
    const isArticleURL = !!(
      location.pathname.match(/\/s\/[0-9a-zA-Z-_]{1,}/) ||
      location.pathname === "/s"
    );
    WXU.log
      .Info()
      .Str("file", "mp.ws.js")
      .Str("url", location.href)
      .Bool("is_article_url", isArticleURL)
      .Msg("main");
    if (!isArticleURL) {
      return;
    }
    var sp = new URLSearchParams(location.search);
    if (!window.cgiDataNew?.bizuin) {
      Object.assign(window.cgiDataNew, {
        bizuin: sp.get("__biz"),
        mid: Number(sp.get("mid")),
        idx: Number(sp.get("idx")),
        sn: sp.get("sn"),
      });
    }
    report_article_loaded();
    WXU.observe_node({
      selector: ".interaction_bar",
      container: "body",
      onOk(node) {
        WXU.log
          .Info()
          .Msg("main - find the container to insert download button");
        if (__download_btn_inserted) {
          return;
        }
        __download_btn_inserted = true;
        insert_download_button();
      },
    });
  }
  var __download_btn_inserted = false;
  WXU.onWindowLoaded(() => {
    if (!WXU.config.officialAccountEnabled) {
      return;
    }
    main();
  });
})();

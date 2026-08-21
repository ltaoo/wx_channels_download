/// <reference path="./mp.utils.js" />

(() => {
  if (!window.WXMPUtils) {
    throw new Error("mp.utils.js must be loaded before mp.main.js");
  }
  const {
    build_download_article,
    collect_push_article_entries,
    first_non_empty,
    get_page_data_value,
    get_url_param,
    parse_official_account_msg_list,
  } = window.WXMPUtils;

  var APIHostname = WXEnv.get("apiOrigin");
  var credentials = {};

  function insert_page_style() {
    if (document.getElementById("__wxmp_page_style__")) {
      return;
    }
    const style = document.createElement("style");
    style.id = "__wxmp_page_style__";
    style.textContent = `
      #activity-detail .t1-popper {
        z-index: 99999 !important;
      }
      body .t1-popper {
        z-index: 99999 !important;
      }
      .interaction_bar .sns_opr_btn:after {
        min-width: 32px !important;
      }
      .interaction_bar .sns_opr_btn .sns_opr_gap {
        width: 32px !important;
      }
      .mm_appmsg .interaction_bar .sns_opr_btn:after {
        min-width: unset !important;
      }
      .mm_appmsg .interaction_bar .sns_opr_btn .sns_opr_gap {
        width: unset !important;
      }
    `;
    (document.head || document.documentElement).appendChild(style);
  }

  insert_page_style();

  const http_client = new Timeless.kit.HttpClientCore({
    headers: { "Content-Type": "application/json" },
    hostname: APIHostname,
  });
  Timeless.web.provide_http_client(http_client);
  const request = Timeless.kit.request_factory({
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

  const scraperFetchReq = new Timeless.kit.RequestCore(
    (body) => request.post("/api/scraper/fetch", body),
    { client: http_client },
  );

  const scraperJobReq = new Timeless.kit.RequestCore(
    (params) => request.get("/api/scraper/job", params),
    { client: http_client },
  );

  function wait_for_scraper_poll() {
    return new Promise((resolve) => window.setTimeout(resolve, 300));
  }

  function scraper_job_result(job) {
    const status = String((job && job.status) || "");
    if (status === "completed") {
      if (!job.output) {
        return Timeless.Result.Err("fetch job 已完成，但缺少抓取结果");
      }
      return Timeless.Result.Ok(job.output);
    }
    if (status === "failed") {
      return Timeless.Result.Err(job.error || "抓取失败");
    }
    if (status === "interrupted") {
      return Timeless.Result.Err("抓取已中断");
    }
    return null;
  }

  async function fetch_scraper_output(url) {
    const create_result = await scraperFetchReq.run({ url });
    if (create_result.error) {
      return create_result;
    }
    let job = create_result.data || {};
    const job_id = String(job.id || "").trim();
    if (!job_id) {
      return Timeless.Result.Err("创建 fetch job 失败：响应缺少 id");
    }
    while (true) {
      const terminal_result = scraper_job_result(job);
      if (terminal_result) {
        return terminal_result;
      }
      await wait_for_scraper_poll();
      const job_result = await scraperJobReq.run({ id: job_id });
      if (job_result.error) {
        return job_result;
      }
      job = job_result.data || {};
    }
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
      .Str("file", "mp.main.js")
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
      WXU.error({ msg: error.message, source: "mp.main.js:189" });
      return;
    }
    if (data.skipped) {
      return;
    }
    WXU.toast("创建下载任务成功");
    console.log(
      "[mp.main.js]handle_download_click - after create download task",
    );
    WXU.downloader.show();
  }

  function DownloadAllPushesModel() {
    const progress_ = ref({
      created: 0,
      failed: 0,
      fetched: 0,
      running: false,
      stopping: false,
    });
    const notice_ = ref(null);
    const open_download_panel_ = ref(0);
    let download_panel_opened = false;
    let stop_signal = false;

    function update_progress(patch) {
      progress_.as({ ...progress_.value, ...patch });
    }

    function notify(type, message) {
      notice_.as({ message, type });
    }

    function request_stop() {
      if (!progress_.value.running || stop_signal) {
        return;
      }
      stop_signal = true;
      update_progress({ stopping: true });
      notify("info", "正在取消批量下载...");
    }

    async function start() {
      if (progress_.value.running) {
        request_stop();
        return;
      }
      if (!credentials.biz) {
        notify("error", "缺少 biz 参数");
        return;
      }

      stop_signal = false;
      download_panel_opened = false;
      update_progress({
        created: 0,
        failed: 0,
        fetched: 0,
        running: true,
        stopping: false,
      });
      notify("info", "开始获取全部推送，再次点击可取消");

      let created_count = 0;
      let failed_count = 0;
      let fetched_count = 0;
      let offset = 0;
      const fetched_urls = new Set();
      try {
        while (!stop_signal) {
          const list_result = await msgListReq.run({
            biz: credentials.biz,
            count: 10,
            key: credentials.key,
            offset,
            pass_ticket: credentials.pass_ticket,
            token: credentials.token,
            uin: credentials.uin,
          });
          if (list_result.error) {
            throw list_result.error;
          }

          const list_data = list_result.data || {};
          const entries = collect_push_article_entries(
            parse_official_account_msg_list(list_data),
          ).filter((entry) => {
            if (fetched_urls.has(entry.url)) {
              return false;
            }
            fetched_urls.add(entry.url);
            return true;
          });
          const articles = [];
          for (const entry of entries) {
            if (stop_signal) {
              break;
            }
            const fetch_result = await fetch_scraper_output(entry.url);
            fetched_count += 1;
            if (fetch_result.error) {
              failed_count += 1;
              update_progress({ failed: failed_count, fetched: fetched_count });
              continue;
            }
            const article = build_download_article(
              fetch_result.data || {},
              entry,
              credentials,
            );
            if (!article.bizuin || !article.mid || !article.user_name) {
              failed_count += 1;
              update_progress({ failed: failed_count, fetched: fetched_count });
              continue;
            }
            articles.push(article);
            update_progress({ fetched: fetched_count });
          }

          if (!stop_signal && articles.length > 0) {
            const [error, data] = await WXU.downloader.create(articles, {
              platform: "wxmp",
            });
            if (error) {
              throw error;
            }
            const created_ids = Array.isArray(data && data.ids) ? data.ids : [];
            created_count += created_ids.length;
            update_progress({ created: created_count });
            if (created_ids.length > 0 && !download_panel_opened) {
              download_panel_opened = true;
              open_download_panel_.as(open_download_panel_.value + 1);
            } else if (
              !WXU.config.downloadForceCheckAllFeeds &&
              created_count === 0
            ) {
              break;
            }
          }

          const has_more = Number(list_data.can_msg_continue) !== 0;
          const next_offset = Number(list_data.next_offset);
          if (
            stop_signal ||
            !has_more ||
            entries.length === 0 ||
            !Number.isFinite(next_offset) ||
            next_offset <= offset
          ) {
            break;
          }
          offset = next_offset;
        }

        if (stop_signal) {
          notify("info", `已取消批量下载，已创建 ${created_count} 个任务`);
        } else if (failed_count > 0) {
          notify(
            "info",
            `批量下载完成：创建 ${created_count} 个任务，${failed_count} 篇解析失败`,
          );
        } else if (created_count === 0) {
          notify("info", "没有新的推送需要下载");
        } else {
          notify("info", `批量下载完成，共创建 ${created_count} 个任务`);
        }
      } catch (error) {
        notify("error", "批量下载失败: " + (error.message || String(error)));
      } finally {
        stop_signal = false;
        update_progress({ running: false, stopping: false });
      }
    }

    return {
      state: {
        notice: notice_,
        open_download_panel: open_download_panel_,
        progress: progress_,
      },
      methods: {
        requestStop: request_stop,
        start,
        toggle() {
          if (progress_.value.running) {
            request_stop();
            return;
          }
          start();
        },
      },
    };
  }

  function DownloadAllPushesMenuItem(props) {
    const model = props.store;
    const menu_item = new Timeless.vm.MenuItemCore({
      label: "下载所有推送",
      onClick() {
        props.close();
        model.methods.toggle();
      },
    });
    model.state.progress.subscribe({
      onChange(progress) {
        const label = progress.running
          ? progress.stopping
            ? "正在取消..."
            : "取消下载所有推送"
          : "下载所有推送";
        if (menu_item.label === label) {
          return;
        }
        menu_item.label = label;
        // MenuItemCore has no setLabel; setIcon emits its updated state.
        menu_item.setIcon(menu_item.icon);
      },
    });
    model.state.notice.subscribe({
      onChange(notice) {
        if (!notice || !notice.message) {
          return;
        }
        if (notice.type === "error") {
          WXU.error({
            msg: notice.message,
            source: "mp.main.js:DownloadAllPushesModel",
          });
          return;
        }
        WXU.toast(notice.message);
      },
    });
    model.state.open_download_panel.subscribe({
      onChange(sequence) {
        if (sequence > 0) {
          WXU.downloader.show();
        }
      },
    });
    return menu_item;
  }

  function MsgListModel(props = {}) {
    const page_size = props.page_size || 10;
    const items_ = refarr([]);
    const downloading_items_ = refarr([]);
    const download_notice_ = ref(null);
    const loading_ = ref(false);
    const error_ = ref("");
    const can_load_more_ = ref(true);
    let current_offset = 0;

    const reqs = {
      msg: {
        list: new Timeless.kit.RequestCore(
          (params) => request.get("/api/mp/msg/list", params),
          { client: http_client },
        ),
      },
    };

    function notify_download(type, message) {
      download_notice_.as({ type, message });
    }

    function error_message(error) {
      if (error && error.message) {
        return error.message;
      }
      return error ? String(error) : "未知错误";
    }

    function set_item_downloading(item, downloading) {
      const downloading_items = downloading_items_.value;
      const next_items = downloading
        ? downloading_items.includes(item)
          ? downloading_items
          : downloading_items.concat(item)
        : downloading_items.filter((downloading_item) => {
            return downloading_item !== item;
          });
      if (next_items !== downloading_items) {
        downloading_items_.as(next_items, { reset: true });
      }
    }

    async function download_item(item) {
      if (!item || downloading_items_.value.includes(item)) {
        return;
      }
      set_item_downloading(item, true);
      try {
        const entry = collect_push_article_entries([item])[0];
        if (!entry || !entry.url) {
          throw new Error("推送缺少有效的文章地址");
        }
        const fetch_result = await fetch_scraper_output(entry.url);
        if (fetch_result.error) {
          throw fetch_result.error;
        }
        const article = build_download_article(
          fetch_result.data || {},
          entry,
          credentials,
        );
        if (!article.bizuin || !article.mid || !article.user_name) {
          throw new Error("文章信息不完整，无法创建下载任务");
        }
        if (!WXU.downloader || typeof WXU.downloader.create !== "function") {
          throw new Error("下载器尚未就绪");
        }
        const [error, data] = await WXU.downloader.create([article], {
          platform: "wxmp",
        });
        if (error) {
          throw error;
        }
        if (data && data.skipped) {
          return;
        }
        notify_download("success", "创建下载任务成功");
      } catch (error) {
        notify_download(
          "error",
          "创建下载任务失败: " +
            (error && error.message ? error.message : String(error)),
        );
      } finally {
        set_item_downloading(item, false);
      }
    }

    async function load_more() {
      if (loading_.value || !can_load_more_.value) {
        return false;
      }
      if (!credentials.biz) {
        error_.as("缺少 biz 参数");
        return false;
      }
      error_.as("");
      loading_.as(true);
      const request_offset = current_offset;
      try {
        const r = await reqs.msg.list.run({
          biz: credentials.biz,
          count: page_size,
          key: credentials.key,
          offset: request_offset,
          pass_ticket: credentials.pass_ticket,
          token: credentials.token,
          uin: credentials.uin,
        });
        if (r.error) {
          error_.as(error_message(r.error));
          return false;
        }
        const data = r.data || {};
        const items = parse_official_account_msg_list(data);
        const has_next_offset =
          Object.prototype.hasOwnProperty.call(data, "next_offset") &&
          data.next_offset !== null &&
          String(data.next_offset).trim() !== "";
        const next_offset = has_next_offset
          ? Number(data.next_offset)
          : Number.NaN;
        const has_server_more =
          data.can_msg_continue === undefined
            ? items.length >= page_size
            : Number(data.can_msg_continue) !== 0;

        if (has_server_more && items.length === 0) {
          error_.as(
            `分页响应异常：offset=${request_offset} 返回空列表但仍标记有更多数据`,
          );
          return false;
        }

        if (has_server_more && has_next_offset) {
          if (
            !Number.isSafeInteger(next_offset) ||
            next_offset <= request_offset
          ) {
            error_.as(
              `分页游标未前进：offset=${request_offset}, next_offset=${String(data.next_offset)}`,
            );
            return false;
          }
          current_offset = next_offset;
        } else if (has_server_more) {
          current_offset = request_offset + items.length;
        }
        items_.as(items_.value.concat(items), { reset: true });
        can_load_more_.as(items.length > 0 && has_server_more);
        return true;
      } catch (error) {
        error_.as(error_message(error));
        return false;
      } finally {
        loading_.as(false);
      }
    }

    function ensure_loaded() {
      if (items_.value.length === 0) {
        return load_more();
      }
      return Promise.resolve(false);
    }

    return {
      state: {
        can_load_more: can_load_more_,
        download_notice: download_notice_,
        downloading_items: downloading_items_,
        error: error_,
        items: items_,
        loading: loading_,
      },
      methods: {
        downloadItem: download_item,
        ensureLoaded: ensure_loaded,
        loadMore: load_more,
      },
    };
  }

  function MsgListMenuItem(props) {
    const model = props.store;
    return new Timeless.vm.MenuItemCore({
      label: "推送列表",
      onClick() {
        props.close();
        props.open();
        model.methods.ensureLoaded();
      },
    });
  }

  var before_menus_items = [];

  function __wxmp_download_menu_label(label) {
    if (typeof Node !== "undefined" && label instanceof Node) {
      return label.textContent || "";
    }
    return label == null ? "" : String(label);
  }

  function __wxmp_create_download_menu_item(options, trigger, close) {
    return new Timeless.vm.MenuItemCore({
      label: __wxmp_download_menu_label(options.label),
      tooltip: options.tooltip || options.title,
      disabled: !!options.disabled,
      async onClick() {
        if (typeof options.onClick === "function") {
          await options.onClick({
            article: window.cgiDataNew || null,
            trigger,
          });
        }
        close();
      },
    });
  }

  function __wxmp_render_extra_download_menu_items(items, trigger, close) {
    return (items || [])
      .filter((item) => {
        return item && item.label && item.onClick;
      })
      .map((item) => {
        return __wxmp_create_download_menu_item(item, trigger, close);
      });
  }

  Object.assign(WXU, {
    unshiftMenuItems(items) {
      if (!Array.isArray(items)) {
        items = [items];
      }
      before_menus_items = items.concat(before_menus_items);
    },
    get before_menu_items() {
      return before_menus_items;
    },
  });

  function DownloaderConnectionStatusDot(props) {
    return View(
      {
        style: computed(props.connected, (connected) => ({
          position: "absolute",
          top: "-2px",
          right: "-2px",
          width: "6px",
          height: "6px",
          "border-radius": "9999px",
          "background-color": connected ? "#22c55e" : "#ef4444",
          "box-sizing": "border-box",
        })),
        attributes: {
          title: computed(props.connected, (connected) =>
            connected ? "WebSocket 已连接" : "WebSocket 已断开",
          ),
          "aria-label": computed(props.connected, (connected) =>
            connected ? "WebSocket 已连接" : "WebSocket 已断开",
          ),
        },
      },
      [],
    );
  }

  function DownloaderEntry(props) {
    const vm$ =
      props.store ||
      (typeof __d_vm$ !== "undefined" ? __d_vm$ : DownloaderPanelViewModel({}));

    return Button(
      {
        attributes: {
          "aria-labelledby": "__wx_download_bottom_text",
        },
        class:
          "sns_opr_btn sns_write_comment_btn __wx_download_btn bar-expand-hotarea js_wx_tap_highlight wx_tap_link",
        style: { position: "relative" },
        onMounted() {
          vm$.methods.connect().catch(() => {});
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
        View(
          {
            attributes: { id: "__wx_download_bottom_text" },
            class: "sns_opr_gap",
          },
          ["下载"],
        ),
        DownloaderConnectionStatusDot({
          connected: vm$.state.websocket_connected,
        }),
        Popover(
          {
            store: props.popover$,
            content: [
              DownloaderPanelView({
                store: vm$,
              }),
            ],
          },
          [],
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
        Dialog({ store: props.msg_list_dialog$ }, [
          MsgListPanel({
            dialog$: props.msg_list_dialog$,
            store: props.msglist$,
          }),
        ]),
      ],
    );
  }

  function insert_download_button() {
    document.body.classList.add("wx-officialaccount-download-menu-mounted");

    const popover$ = new Timeless.vm.PopoverCore({
      offsetY: 4,
      destroyOnClose: false,
    });
    const downloadervm$ =
      typeof __d_vm$ !== "undefined" ? __d_vm$ : DownloaderPanelViewModel({});

    const msglist$ = MsgListModel();
    const msg_list_dialog$ = new Timeless.vm.DialogCore({
      offsetY: 4,
    });
    // Create button container and insert into page (following panel.js pattern: insert DOM element first, then render VDOM into it)
    const $btn = document.createElement("div");
    $btn.className = "sns_opr_btn_con";
    setTimeout(() => {
      var $wraps = document.querySelectorAll(".interaction_bar");
      var $container = $wraps[$wraps.length - 1];
      const no_container = !$container || !$container.lastElementChild;
      if (no_container) {
        return;
      }
      $container.insertBefore($btn, $container.lastElementChild);
    }, 1800);
    WXU.downloader.show = function () {
      popover$.popper.setReference({
        $el: $btn,
        getRect() {
          return $btn.getBoundingClientRect();
        },
      });
      popover$.show();
    };
    WXU.downloader.hide = function () {
      popover$.hide();
    };
    // Render download panel entry into button container
    Timeless.DOM.render(
      DownloaderEntry({
        popover$,
        msglist$,
        msg_list_dialog$,
        store: downloadervm$,
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
    let dropdown$ = null;
    function close_dropdown() {
      if (dropdown$) {
        dropdown$.hide();
      }
    }
    // const download_all_model = DownloadAllPushesModel();
    // const download_all_menu_item = DownloadAllPushesMenuItem({
    //   close: close_dropdown,
    //   store: download_all_model,
    // });
    const msg_list_menu_item = MsgListMenuItem({
      close: close_dropdown,
      open() {
        msg_list_dialog$.show();
      },
      store: msglist$,
    });
    dropdown$ = new Timeless.vm.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      items: [
        ...__wxmp_render_extra_download_menu_items(
          before_menus_items,
          $btn,
          close_dropdown,
        ),
        new Timeless.vm.MenuItemCore({
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
        new Timeless.vm.MenuItemCore({
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
              msg_list_menu_item,
              // download_all_menu_item,
            ]
          : []),
        new Timeless.vm.MenuItemCore({
          label: "下载面板",
          onClick() {
            dropdown$.hide();
            WXU.downloader.show();
          },
        }),
      ],
    });
    const $dropdown_root = document.createElement("span");
    $dropdown_root.className = "wx-download-dropdown-menu-root";
    $dropdown_root.style.display = "contents";
    document.body.appendChild($dropdown_root);
    Timeless.DOM.render(
      Timeless.weui.DropdownMenu({ store: dropdown$ }),
      $dropdown_root,
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
    function is_downloader_connected() {
      return !!downloadervm$.state.websocket_connected.value;
    }
    function show_dropdown() {
      if (!is_downloader_connected()) {
        dropdown$.hide({ reason: "download websocket disconnected" });
        WXU.downloader.show();
        return;
      }
      WXU.downloader.hide();
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
      if (!is_downloader_connected()) {
        WXU.downloader.show();
        return;
      }
      await create_download_task(popover$, $btn);
    }
    downloadervm$.state.websocket_connected.subscribe({
      onChange(connected) {
        if (!connected) {
          dropdown$.hide({ reason: "download websocket disconnected" });
        }
      },
    });
    $btn.addEventListener("mouseenter", show_dropdown);
    $btn.addEventListener("mouseleave", hide_dropdown);
    $btn.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
    });

    // const msg_list_overlay = document.createElement("div");
    // msg_list_overlay.style.cssText =
    //   "display: none; position: fixed; inset: 0; z-index: 200; background: rgba(0,0,0,0.5); justify-content: center; align-items: center;";
    // const msg_list_panel_root = document.createElement("div");
    // msg_list_panel_root.style.display = "contents";
    // msg_list_overlay.appendChild(msg_list_panel_root);
    // msg_list_overlay.addEventListener("click", (event) => {
    //   if (event.target === msg_list_overlay) {
    //     msg_list_dialog$.hide();
    //   }
    // });
    // document.body.appendChild(msg_list_overlay);
    // Timeless.DOM.render(
    //   ,
    //   msg_list_panel_root,
    // );
    // msg_list_dialog$.onStateChange((state) => {
    //   msg_list_overlay.style.display = state.visible ? "flex" : "none";
    // });
  }
  async function main() {
    const isArticleURL = !!(
      location.pathname.match(/\/s\/[0-9a-zA-Z-_]{1,}/) ||
      location.pathname === "/s"
    );
    WXU.log
      .Info()
      .Str("file", "mp.main.js")
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
    credentials = {
      biz: first_non_empty(
        window.biz,
        window.__biz,
        get_page_data_value("bizuin"),
        get_url_param(window.location.href, "__biz"),
        get_url_param(get_page_data_value("link"), "__biz"),
      ),
      key: first_non_empty(
        window.key,
        get_page_data_value("key"),
        get_url_param(window.location.href, "key"),
      ),
      pass_ticket: first_non_empty(
        window.pass_ticket,
        get_page_data_value("pass_ticket"),
        get_url_param(window.location.href, "pass_ticket"),
      ),
      token: WXU.config.officialServerRefreshToken ?? "",
      uin: first_non_empty(
        window.uin,
        get_page_data_value("user_uin"),
        get_url_param(window.location.href, "uin"),
      ),
    };
    report_article_loaded();
    WXU.observe_node({
      selector: ".sns_opr_btn_con",
      container: "body",
      onOk() {
        WXU.log
          .Info()
          .Bool("download button is inserted", __download_btn_inserted)
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

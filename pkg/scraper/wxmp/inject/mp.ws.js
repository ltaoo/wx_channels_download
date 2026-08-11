(() => {
  var APIHostname = WXEnv.get("apiOrigin");
  var MPWSURL = "";

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

  const scraperFetchReq = new Timeless.RequestCore(
    (body) => request.post("/api/scraper/fetch", body),
    { client: http_client },
  );

  const scraperJobReq = new Timeless.RequestCore(
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
      const ws = new WebSocket(MPWSURL);
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
      WXU.error({ msg: error.message, source: "mp.ws.js:189" });
      return;
    }
    if (data.skipped) {
      return;
    }
    WXU.toast("创建下载任务成功");
    console.log("[mp.ws.js]handle_download_click - after create download task");
    WXU.downloader.show();
  }

  function parse_official_account_msg_list(data) {
    if (data && Array.isArray(data.list)) {
      return data.list;
    }
    const raw_list = (data && data.general_msg_list) || "";
    if (!raw_list) {
      return [];
    }
    try {
      const parsed =
        typeof raw_list === "string" ? JSON.parse(raw_list) : raw_list;
      return Array.isArray(parsed.list) ? parsed.list : [];
    } catch {
      return [];
    }
  }

  function decode_official_account_url(raw_url) {
    if (!raw_url) {
      return "";
    }
    const textarea = document.createElement("textarea");
    textarea.innerHTML = String(raw_url);
    try {
      const parsed_url = new URL(
        textarea.value,
        "https://mp.weixin.qq.com",
      );
      if (parsed_url.hostname !== "mp.weixin.qq.com") {
        return "";
      }
      return parsed_url.href;
    } catch {
      return "";
    }
  }

  function collect_push_article_entries(items) {
    const entries = [];
    const urls = new Set();
    function append(article, publish_time) {
      const url = decode_official_account_url(article && article.content_url);
      if (!url || urls.has(url)) {
        return;
      }
      urls.add(url);
      entries.push({ article, publish_time, url });
    }
    (items || []).forEach((item) => {
      const article = item.app_msg_ext_info || {};
      const publish_time = item.comm_msg_info?.datetime || 0;
      append(article, publish_time);
      (article.multi_app_msg_item_list || []).forEach((child) => {
        append(child, publish_time);
      });
    });
    return entries;
  }

  function article_ids_from_url(article_url, biz, external_id) {
    const parsed_url = new URL(article_url);
    let mid = Number(parsed_url.searchParams.get("mid")) || 0;
    let idx = Number(parsed_url.searchParams.get("idx")) || 0;
    const external_prefix = biz ? biz + "_" : "";
    if (
      (!mid || !idx) &&
      external_prefix &&
      external_id?.startsWith(external_prefix)
    ) {
      const external_parts = external_id
        .slice(external_prefix.length)
        .split("_");
      mid = mid || Number(external_parts[0]) || 0;
      idx = idx || Number(external_parts[1]) || 0;
    }
    return {
      biz: parsed_url.searchParams.get("__biz") || biz,
      idx: idx || 1,
      mid,
      sn: parsed_url.searchParams.get("sn") || "",
    };
  }

  function build_download_article(fetch_data, entry, credentials) {
    const parsed_article = (fetch_data && fetch_data.result) || {};
    const content = (fetch_data && fetch_data.content) || {};
    const account = (fetch_data && fetch_data.account) || {};
    const summary = entry.article || {};
    const ids = article_ids_from_url(
      entry.url,
      credentials.biz,
      content.external_id || "",
    );
    const publish_time = Number(content.publish_time || entry.publish_time) || 0;
    return {
      bizuin: ids.biz,
      mid: ids.mid,
      idx: ids.idx,
      sn: ids.sn,
      title: parsed_article.title || content.title || summary.title || "",
      desc: content.description || summary.digest || "",
      content_noencode: parsed_article.content || summary.content || "",
      cdn_url: content.cover_url || summary.cover || "",
      link: content.url || entry.url,
      source_url: content.source_url || summary.source_url || entry.url,
      user_name: parsed_article.author_id || account.external_id || "",
      nick_name:
        parsed_article.author_nickname || account.nickname || summary.author || "",
      round_head_img: parsed_article.author_avatar || account.avatar_url || "",
      author: parsed_article.creator || summary.author || "",
      ori_create_time:
        publish_time > 1000000000000
          ? Math.floor(publish_time / 1000)
          : publish_time,
      page_type: Number(parsed_article.type) || 0,
      item_show_type: Number(summary.item_show_type) || 0,
      picture_page_info_list: parsed_article.picture_page_info_list || [],
      video_page_infos: parsed_article.videos || [],
      copyright_info: {
        copyright_stat: Number(summary.copyright_stat) || 0,
      },
    };
  }

  function DownloadAllPushesModel(options) {
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
      const credentials = options.get_credentials();
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
    const menu_item = new Timeless.ui.MenuItemCore({
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
            source: "mp.ws.js:DownloadAllPushesModel",
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

  var before_menus_items = [];

  function __wxmp_download_menu_label(label) {
    if (typeof Node !== "undefined" && label instanceof Node) {
      return label.textContent || "";
    }
    return label == null ? "" : String(label);
  }

  function __wxmp_create_download_menu_item(options, trigger, close) {
    return new Timeless.ui.MenuItemCore({
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

  function DownloaderEntry(props) {
    const vm$ =
      typeof __d_vm$ !== undefined ? __d_vm$ : DownloaderPanelViewModel({});
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
        View(
          {
            attributes: { id: "__wx_download_bottom_text" },
            class: "sns_opr_gap",
            style: { display: "inline" },
          },
          ["下载"],
        ),
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
        WXU.error({
          msg: "获取推送列表失败: " + r.error.message,
          source: "mp.ws.js:404",
        });
        loadMoreBtn.textContent = "重试";
        return;
      }
      const data = r.data || {};
      const list = parse_official_account_msg_list(data);
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
    // Create button container and insert into page (following panel.js pattern: insert DOM element first, then render VDOM into it)
    const $btn = document.createElement("div");
    $btn.className = "sns_opr_btn_con";
    $container.insertBefore($btn, $container.lastElementChild);
    console.log("before render(DownloaderEntry");
    WXU.downloader.show = function () {
      popover$.popper.setReference({
        $el: {},
        getRect() {},
      });
      const pos = popover_pos($btn);
      popover$.show(pos);
    };
    WXU.downloader.hide = function () {
      popover$.hide();
    };
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
    let dropdown$ = null;
    function close_dropdown() {
      if (dropdown$) {
        dropdown$.hide();
      }
    }
    const download_all_model = DownloadAllPushesModel({
      get_credentials() {
        return {
          biz: window.biz || window.__biz || "",
          key: window.key || "",
          pass_ticket: window.pass_ticket || "",
          token: WXU.config.officialServerRefreshToken ?? "",
          uin: window.uin || "",
        };
      },
    });
    const download_all_menu_item = DownloadAllPushesMenuItem({
      close: close_dropdown,
      store: download_all_model,
    });
    dropdown$ = new Timeless.ui.DropdownMenuCore({
      trigger: "hover",
      align: "end",
      offsetY: -20,
      items: [
        ...__wxmp_render_extra_download_menu_items(
          before_menus_items,
          $btn,
          close_dropdown,
        ),
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
              // download_all_menu_item,
            ]
          : []),
        new Timeless.ui.MenuItemCore({
          label: "下载面板",
          onClick() {
            dropdown$.hide();
            console.log("[mp.ws.js]onClick - before popover$.show");
            WXU.downloader.show();
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

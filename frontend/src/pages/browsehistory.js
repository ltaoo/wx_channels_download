(function (global) {
  const browse_history_request = Timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取浏览记录列表失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

  function number_or_default(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function normalize_browse_history_list_response(
    data,
    fallbackPage,
    fallbackSize,
  ) {
    const source = data && typeof data === "object" ? data : {};
    const list = Array.isArray(source.list)
      ? source.list
      : Array.isArray(source.List)
        ? source.List
        : [];
    return {
      list,
      total: Math.max(
        0,
        number_or_default(
          typeof source.total !== "undefined" ? source.total : source.Total,
          list.length,
        ),
      ),
      page: Math.max(
        1,
        number_or_default(source.page || source.Page, fallbackPage),
      ),
      page_size: Math.max(
        1,
        number_or_default(
          source.page_size || source.pageSize || source.PageSize,
          fallbackSize,
        ),
      ),
    };
  }

  function first_non_empty(...values) {
    for (const value of values) {
      if (value !== undefined && value !== null && value !== "") {
        return value;
      }
    }
    return "";
  }

  function normalize_browse_history_item(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    const accounts = Array.isArray(source.accounts)
      ? source.accounts.map(normalize_account_brief)
      : [];
    const primary =
      accounts.length > 0
        ? accounts[0]
        : { nickname: "", avatar_url: "", external_id: "" };
    return {
      ...source,
      id: first_non_empty(source.id, source.ID),
      platform_id: first_non_empty(source.platform_id, source.PlatformID),
      platform_name: first_non_empty(
        source.platform_name,
        source.platformName,
        source.PlatformName,
      ),
      content_type: first_non_empty(
        source.type,
        source.Type,
        source.content_type,
        source.ContentType,
      ),
      title: first_non_empty(
        source.title,
        source.Title,
        source.content_title,
        source.ContentTitle,
        "未命名内容",
      ),
      cover_url: first_non_empty(
        source.cover_url,
        source.CoverURL,
        source.coverUrl,
      ),
      accounts: accounts,
      author_nickname: primary.nickname,
      author_external_id: primary.external_id,
      author_avatar_url: primary.avatar_url,
      url: first_non_empty(
        source.url,
        source.URL,
        source.content_url,
        source.ContentURL,
        source.source_url,
        source.SourceURL,
      ),
      visited_times: number_or_default(
        first_non_empty(
          source.visited_times,
          source.VisitedTimes,
          source.visits,
          source.visit_count,
        ),
        0,
      ),
      updated_at: number_or_default(
        first_non_empty(source.updated_at, source.UpdatedAt),
        0,
      ),
      publish_time: number_or_default(
        first_non_empty(source.publish_time, source.PublishTime),
        0,
      ),
    };
  }

  function normalize_account_brief(acc) {
    if (!acc || typeof acc !== "object") {
      return { nickname: "", avatar_url: "", external_id: "" };
    }
    return {
      nickname: first_non_empty(acc.nickname, acc.Nickname, ""),
      avatar_url: first_non_empty(
        acc.avatar_url,
        acc.avatarUrl,
        acc.AvatarURL,
        "",
      ),
      external_id: first_non_empty(
        acc.external_id,
        acc.externalId,
        acc.ExternalId,
        "",
      ),
    };
  }

  function browse_history_platform_favicon(history) {
    const icons = {
      wxchannels:
        "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
      wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
      zhihu: "https://static.zhihu.com/heifetz/favicon.ico",
    };
    const key = history && (history.platform_id || history.platformId);
    return icons[key] || "";
  }

  function browse_history_platform_name(history) {
    if (history.platform_name) {
      return history.platform_name;
    }
    const names = {
      wxchannels: "视频号",
      wxmp: "公众号",
      officialaccount: "公众号",
      douyin: "抖音",
      bilibili: "Bilibili",
      xiaohongshu: "小红书",
      xhs: "小红书",
      youtube: "YouTube",
      zhihu: "知乎",
      douban: "豆瓣",
      qidian: "起点中文网",
      fanqienovel: "番茄小说",
      "69shuba": "69书吧",
      ttk: "TT看书",
    };
    return names[history.platform_id] || history.platform_id || "未知平台";
  }

  function browse_history_type_label(value) {
    const type = String(value || "")
      .trim()
      .toLowerCase();
    const labels = {
      video: "视频",
      short_video: "短视频",
      image: "图片",
      image_set: "图集",
      album: "图集",
      article: "文章",
      blog: "文章",
      novel: "小说",
      audio: "音频",
      podcast: "播客",
      music: "音乐",
      document: "文档",
      live: "直播",
    };
    return labels[type] || type || "内容";
  }

  function normalize_epoch_ms(value) {
    const timestamp = Number(value);
    if (!Number.isFinite(timestamp) || timestamp <= 0) {
      return 0;
    }
    return timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
  }

  function format_history_time(value) {
    const timestamp = normalize_epoch_ms(value);
    if (!timestamp) {
      return "时间未知";
    }
    return new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(timestamp));
  }

  function normalize_author_name(raw) {
    const name = first_non_empty(
      raw && raw.author_nickname,
      raw && raw.author_external_id,
      "未知作者",
    );
    return name;
  }

  function browse_history_item_key(history) {
    return String(
      first_non_empty(
        history && history.id,
        history && history.url,
        history && history.source_url,
      ),
    );
  }

  function append_unique_browse_histories(current, incoming) {
    const next = Array.isArray(current) ? current.slice() : [];
    const keys = new Set(next.map(browse_history_item_key).filter(Boolean));
    for (const history of incoming || []) {
      const key = browse_history_item_key(history);
      if (key && keys.has(key)) {
        continue;
      }
      next.push(history);
      if (key) {
        keys.add(key);
      }
    }
    return next;
  }

  function BrowseHistoryViewModel(props) {
    const PAGE_SIZE_DEFAULT = 24;
    const histories_ = refarr([]);
    const total_ = ref(0);
    const page_ = ref(1);
    const page_size_ = ref(PAGE_SIZE_DEFAULT);
    const platform_id_ = ref("");
    const loading_ = ref(false);
    const loading_more_ = ref(false);
    const error_ = ref("");
    const load_more_error_ = ref("");
    let request_sequence = 0;

    const platform_options = [
      new Timeless.vm.SelectItemCore({ label: "全部平台", value: "" }),
      new Timeless.vm.SelectItemCore({ label: "视频号", value: "wxchannels" }),
      new Timeless.vm.SelectItemCore({ label: "公众号", value: "wxmp" }),
      new Timeless.vm.SelectItemCore({ label: "抖音", value: "douyin" }),
      new Timeless.vm.SelectItemCore({ label: "Bilibili", value: "bilibili" }),
      new Timeless.vm.SelectItemCore({ label: "小红书", value: "xiaohongshu" }),
      new Timeless.vm.SelectItemCore({ label: "YouTube", value: "youtube" }),
      new Timeless.vm.SelectItemCore({ label: "知乎", value: "zhihu" }),
      new Timeless.vm.SelectItemCore({ label: "豆瓣", value: "douban" }),
      new Timeless.vm.SelectItemCore({ label: "微博", value: "weibo" }),
    ];
    const platform_select_ = new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部平台",
      options: platform_options,
      onChange(value) {
        platform_id_.as(String(value || ""));
        load(1, { reset: true });
      },
    });

    const list_request = new Timeless.kit.RequestCore(
      (params) =>
        browse_history_request.post("/api/browse_history/list", params),
      {
        client: browse_history_http_client,
        process(response) {
          if (response.error) {
            return Timeless.Result.Err(response.error);
          }
          return Timeless.Result.Ok(
            normalize_browse_history_list_response(
              response.data,
              page_.value,
              page_size_.value,
            ),
          );
        },
      },
    );

    const initial_loading_ = combine(
      { loading: loading_, histories: histories_ },
      (state) => state.loading && state.histories.length === 0,
    );
    const has_more_ = combine(
      {
        histories: histories_,
        total: total_,
        page: page_,
        pageSize: page_size_,
      },
      (state) =>
        state.histories.length < state.total &&
        state.page * state.pageSize < state.total,
    );
    const loaded_text_ = combine(
      {
        total: total_,
        histories: histories_,
      },
      (state) => {
        const count = state.histories.length;
        if (!state.total) {
          return `共 ${state.total || 0} 条`;
        }
        return count >= state.total
          ? `已加载全部 ${state.total} 条`
          : `已加载 ${count} / ${state.total} 条`;
      },
    );

    async function load(targetPage = page_.value, options = {}) {
      const append = options.append === true;
      if (append && (loading_.value || !has_more_.value)) {
        return null;
      }
      const sequence = ++request_sequence;
      const requestedPage = Math.max(1, Number(targetPage) || 1);
      loading_.as(true);
      loading_more_.as(append);
      if (append) {
        load_more_error_.as("");
      } else {
        error_.as("");
        load_more_error_.as("");
      }
      if (options.reset === true) {
        histories_.as([], { reset: true });
        total_.as(0);
        page_.as(1);
      }
      const params = {
        page: requestedPage,
        page_size: page_size_.value,
      };
      const platformId = String(platform_id_.value || "").trim();
      if (platformId) {
        params.platform_id = platformId;
      }

      const result = await list_request.run(params);
      if (sequence !== request_sequence) {
        return result;
      }
      loading_.as(false);
      loading_more_.as(false);
      if (result.error) {
        const message = result.error.message || String(result.error);
        if (append) {
          load_more_error_.as(message);
        } else {
          error_.as(message);
        }
        return result;
      }

      const data = result.data;
      const incoming = data.list.map(normalize_browse_history_item);
      const next = append
        ? append_unique_browse_histories(histories_.value, incoming)
        : incoming;
      histories_.as(next, { reset: true });
      total_.as(Math.max(data.total, next.length));
      page_.as(data.page || requestedPage);
      page_size_.as(data.page_size);
      return result;
    }

    const methods = {
      ready() {
        return load(1, { reset: true });
      },
      refresh() {
        return load(1, { reset: true });
      },
      loadMore() {
        if (loading_.value || !has_more_.value) {
          return null;
        }
        return load(page_.value + 1, { append: true });
      },
      openSource(history) {
        if (!history || !history.url) {
          return;
        }
        window.open(history.url, "_blank", "noopener,noreferrer");
      },
      platformFavicon: browse_history_platform_favicon,
      platformName: browse_history_platform_name,
      typeLabel: browse_history_type_label,
      formatTime: format_history_time,
      authorName: normalize_author_name,
    };

    return {
      state: {
        histories: histories_,
        total: total_,
        page: page_,
        page_size: page_size_,
        initial_loading: initial_loading_,
        has_more: has_more_,
        loaded_text: loaded_text_,
        platform_id: platform_id_,
        loading: loading_,
        loading_more: loading_more_,
        error: error_,
        load_more_error: load_more_error_,
      },
      ui: {
        platform: platform_select_,
      },
      methods,
      ready: methods.ready,
    };
  }

  function BrowseHistoryPageView(props) {
    const vm$ = BrowseHistoryViewModel(props);
    return View(
      {
        class: "wx-content-page wx-browse-history-page",
        onMounted() {
          vm$.ready();
        },
      },
      [
        BrowseHistoryPageHeader({ store: vm$ }),
        BrowseHistoryPageBody({ store: vm$ }),
        Show({
          when: computed(
            vm$.state.histories,
            (histories) => histories.length > 0,
          ),
          ok() {
            return BrowseHistoryLoadStatus({ store: vm$ });
          },
        }),
      ],
    );
  }

  global.register("browsehistory_page", BrowseHistoryPageView);

  function BrowseHistoryPageActionButton(props) {
    return View(
      {
        type: "button",
        class: [
          "wx-content-action",
          props.compact ? "wx-content-action-compact" : "",
          props.class,
        ]
          .filter(Boolean)
          .join(" "),
        attributes: {
          type: "button",
          title: props.title || "",
          ...(props.attributes || {}),
        },
        onClick: props.onClick,
      },
      [
        props.icon
          ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
          : null,
        props.label
          ? View({ class: "wx-content-action-label" }, [props.label])
          : null,
      ].filter(Boolean),
    );
  }

  function BrowseHistoryPageHeader(props) {
    const vm$ = props.store;
    return View({ class: "wx-content-header" }, [
      View({ class: "wx-content-header-inner" }, [
        View({ class: "wx-content-brand" }, [
          View({ class: "wx-content-brand-icon" }, [
            Timeless.Icon({ name: "clock3", size: 28 }),
          ]),
          View({}, [
            View({ class: "wx-content-title" }, ["浏览记录"]),
            View({ class: "wx-content-subtitle" }, [
              computed(
                vm$.state.total,
                (total) => `管理已记录的 ${total} 条浏览记录`,
              ),
            ]),
          ]),
        ]),
        BrowseHistoryPageActionButton({
          icon: "refresh-cw",
          label: "刷新",
          onClick() {
            vm$.methods.refresh();
          },
        }),
      ]),
    ]);
  }

  function BrowseHistoryRowCover(props) {
    const history = props.history;
    if (!history.cover_url) {
      return View(
        { class: "wx-content-row-cover wx-content-row-cover-fallback" },
        [Timeless.Icon({ name: "file", size: 18 })],
      );
    }
    return View({ class: "wx-content-row-cover-wrap" }, [
      View({ class: "wx-content-row-cover wx-content-row-cover-fallback" }, [
        Timeless.Icon({ name: "file", size: 18 }),
      ]),
      Img({
        class: "wx-content-row-cover",
        src: history.cover_url,
        alt: history.title,
        attributes: {
          loading: "lazy",
          referrerpolicy: "no-referrer",
        },
        onError(event) {
          event.target.style.display = "none";
        },
      }),
    ]);
  }

  function BrowseHistoryRow(props) {
    const vm$ = props.store;
    const history = props.history;
    return View(
      {
        class: ["wx-content-row", history.url ? "wx-content-row-clickable" : ""]
          .filter(Boolean)
          .join(" "),
        onClick() {
          vm$.methods.openSource(history);
        },
      },
      [
        // 封面
        BrowseHistoryRowCover({ history: history }),
        // 标题
        View({ class: "wx-content-row-main" }, [
          View(
            {
              class: "wx-content-row-title",
              attributes: { title: history.title },
            },
            [history.title],
          ),
          View({ class: "wx-content-row-badges" }, [
            View({ class: "wx-content-row-platform" }, [
              Show({
                when: vm$.methods.platformFavicon(history),
                ok() {
                  return Img({
                    class: "wx-content-row-platform-icon",
                    src: vm$.methods.platformFavicon(history),
                    attributes: {
                      alt: "",
                      loading: "lazy",
                      referrerpolicy: "no-referrer",
                    },
                    onError(event) {
                      event.target.style.display = "none";
                    },
                  });
                },
              }),
              vm$.methods.platformName(history),
            ]),
            View({ class: "wx-content-row-type" }, [
              vm$.methods.typeLabel(history.content_type),
            ]),
          ]),
        ]),
        // 账号
        View(
          {
            class: "wx-content-row-author",
            attributes: { title: vm$.methods.authorName(history) },
          },
          [
            For({
              each: history.accounts || [],
              render(acc_) {
                const acc =
                  acc_ && acc_.value !== undefined ? acc_.value : acc_;
                return View({ class: "wx-content-row-author-account" }, [
                  Show({
                    when: acc.avatar_url,
                    ok() {
                      return Img({
                        class: "wx-content-row-author-avatar",
                        src: acc.avatar_url,
                        attributes: {
                          alt: acc.nickname || acc.external_id || "",
                          loading: "lazy",
                          referrerpolicy: "no-referrer",
                        },
                        onError(event) {
                          event.target.style.display = "none";
                        },
                      });
                    },
                  }),
                  View(
                    {
                      class: "wx-content-row-author-name",
                      attributes: {
                        title: acc.nickname || acc.external_id || "",
                      },
                    },
                    [acc.nickname || acc.external_id || "未知"],
                  ),
                ]);
              },
            }),
          ],
        ),
        // 访问时间
        View({ class: "wx-content-row-meta" }, [
          Timeless.Icon({ name: "clock3", size: 12 }),
          vm$.methods.formatTime(history.updated_at),
        ]),
        // 访问次数
        View({ class: "wx-content-row-visits" }, [
          history.visited_times > 0 ? `浏览 ${history.visited_times} 次` : "",
        ]),
      ],
    );
  }

  function BrowseHistoryTableHead() {
    return View({ class: "wx-content-row wx-content-row-head" }, [
      View({ class: "wx-content-row-head-cell" }, ["封面"]),
      View({ class: "wx-content-row-head-cell" }, ["标题"]),
      View({ class: "wx-content-row-head-cell wx-content-row-col-author" }, [
        "账号",
      ]),
      View({ class: "wx-content-row-head-cell wx-content-row-col-meta" }, [
        "访问时间",
      ]),
      View({ class: "wx-content-row-head-cell wx-content-row-col-visits" }, [
        "访问次数",
      ]),
    ]);
  }

  function BrowseHistorySkeletonRow() {
    return View({ class: "wx-content-row wx-content-skeleton-row" }, [
      View({ class: "wx-content-row-col-cover" }, [
        View({ class: "wx-content-row-cover wx-content-skeleton" }),
      ]),
      View({ class: "wx-content-row-col-main" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
      ]),
      View({ class: "wx-content-row-col-author" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
      ]),
      View({ class: "wx-content-row-col-meta" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      ]),
      View({ class: "wx-content-row-col-visits" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      ]),
    ]);
  }

  function BrowseHistoryListView(props) {
    const vm$ = props.store;
    const histories_ = vm$.state.histories;

    return View(
      {
        class: props.class || "wx-content-history-list",
        style: props.style || {},
      },
      [
        VirtualListView({
          style: {
            height: "100%",
            "max-height": "100%",
            overflow: "auto",
            position: "relative",
            "box-sizing": "border-box",
            "background-color": "transparent",
            ...(props.listViewStyle || {}),
          },
          key: "id",
          size: props.size || 10,
          buffer: props.buffer || 6,
          gutter: props.gutter || 1,
          itemHeight: props.itemHeight || 72,
          each: histories_,
          onReachBottom() {
            vm$.methods.loadMore();
          },
          render(history_) {
            const history =
              history_ && history_.value !== undefined
                ? history_.value
                : history_;
            return BrowseHistoryRow({ store: vm$, history });
          },
        }),
      ],
    );
  }

  function BrowseHistoryPageBody(props) {
    const vm$ = props.store;
    return View({ class: "wx-content-main" }, [
      Show({
        when: vm$.state.initial_loading,
        ok() {
          return View(
            { class: "wx-content-rows" },
            Array.from({ length: 8 }, () => BrowseHistorySkeletonRow()),
          );
        },
        else() {
          return Show({
            when: computed(vm$.state.error, (error) => Boolean(error)),
            ok() {
              return View({ class: "wx-content-state" }, [
                Timeless.Icon({ name: "circle-alert", size: 32 }),
                View({ class: "wx-content-state-title" }, ["浏览记录加载失败"]),
                View({ class: "wx-content-state-text" }, [vm$.state.error]),
                BrowseHistoryPageActionButton({
                  icon: "refresh-cw",
                  label: "重试",
                  onClick() {
                    vm$.methods.refresh();
                  },
                }),
              ]);
            },
            else() {
              return Show({
                when: computed(
                  vm$.state.histories,
                  (histories) => histories.length > 0,
                ),
                ok() {
                  return View(
                    {
                      class: "wx-content-rows wx-content-history-rows",
                    },
                    [
                      BrowseHistoryTableHead(),
                      BrowseHistoryListView({ store: vm$ }),
                    ],
                  );
                },
                else() {
                  return View({ class: "wx-content-state" }, [
                    Timeless.Icon({ name: "inbox", size: 36 }),
                    View({ class: "wx-content-state-title" }, ["暂无浏览记录"]),
                    View({ class: "wx-content-state-text" }, [
                      "当前条件下未找到浏览记录",
                    ]),
                  ]);
                },
              });
            },
          });
        },
      }),
    ]);
  }

  function BrowseHistoryLoadStatus(props) {
    const vm$ = props.store;
    return View({ class: "wx-content-pagination wx-content-load-more" }, [
      View({ class: "wx-content-pagination-summary" }, [vm$.state.loaded_text]),
      View({ class: "wx-content-load-more-status" }, [
        Show({
          when: vm$.state.loading_more,
          ok() {
            return [
              View({ class: "weui-loading" }),
              View({}, ["正在加载更多..."]),
            ];
          },
          else() {
            return Show({
              when: computed(vm$.state.load_more_error, (error) =>
                Boolean(error),
              ),
              ok() {
                return [
                  View({ class: "wx-content-load-more-error" }, [
                    vm$.state.load_more_error,
                  ]),
                  BrowseHistoryPageActionButton({
                    icon: "refresh-cw",
                    label: "重试",
                    compact: true,
                    onClick() {
                      vm$.methods.loadMore();
                    },
                  }),
                ];
              },
              else() {
                return View({}, [
                  computed(vm$.state.has_more, (hasMore) =>
                    hasMore ? "继续向下滚动加载更多" : "没有更多记录了",
                  ),
                ]);
              },
            });
          },
        }),
      ]),
    ]);
  }
})(window);

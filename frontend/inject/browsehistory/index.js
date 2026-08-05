/// <reference path="../utils.js" />
/// <reference path="model.js" />
/**
 * @file Browse history list page entry.
 */
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

function BrowseHistoryPageToolbar(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-toolbar" }, [
    Timeless.Select({
      store: vm$.ui.platform,
      class: "wx-content-type-select",
    }),
  ]);
}

function BrowseHistoryRowCover(props) {
  const history = props.history;
  if (!history.cover_url) {
    return View({ class: "wx-content-row-cover wx-content-row-cover-fallback" }, [
      Timeless.Icon({ name: "file", size: 18 }),
    ]);
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
              const acc = acc_ && acc_.value !== undefined ? acc_.value : acc_;
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
                    attributes: { title: acc.nickname || acc.external_id || "" },
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
        vm$.methods.formatTime(history.publish_time),
      ]),
      // 访问次数
      View({ class: "wx-content-row-visits" }, [
        history.visited_times > 0
          ? `浏览 ${history.visited_times} 次`
          : "",
      ]),
    ],
  );
}

function BrowseHistoryTableHead() {
  return View({ class: "wx-content-row wx-content-row-head" }, [
    View({ class: "wx-content-row-head-cell" }, ["封面"]),
    View({ class: "wx-content-row-head-cell" }, ["标题"]),
    View({ class: "wx-content-row-head-cell wx-content-row-col-author" }, ["账号"]),
    View({ class: "wx-content-row-head-cell wx-content-row-col-meta" }, ["访问时间"]),
    View({ class: "wx-content-row-head-cell wx-content-row-col-visits" }, ["访问次数"]),
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

function BrowseHistoryPageBody(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-main" }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return View(
          { class: "wx-content-rows" },
          Array.from({ length: 8 }, () => BrowseHistorySkeletonRow()),
        );
      },
      else() {
        return Show({
          when: computed(
            vm$.state.error,
            (error) => Boolean(error),
          ),
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
                return View({ class: "wx-content-rows" }, [
                  BrowseHistoryTableHead(),
                  For({
                    each: vm$.state.histories,
                    render(history_) {
                      const history =
                        history_ && history_.value !== undefined
                          ? history_.value
                          : history_;
                      return BrowseHistoryRow({ store: vm$, history });
                    },
                  }),
                ]);
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

function BrowseHistoryPagination(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-pagination" }, [
    View({ class: "wx-content-pagination-summary" }, [vm$.state.range_text]),
    View({ class: "wx-content-pagination-controls" }, [
      View(
        {
          type: "select",
          class: "wx-content-select wx-content-page-size",
          attributes: { "aria-label": "每页数量" },
          onChange(event) {
            vm$.methods.setPageSize(event.target.value);
          },
        },
        [
          View({ type: "option", attributes: { value: "12" } }, ["12 条/页"]),
          View(
            { type: "option", attributes: { value: "24", selected: true } },
            ["24 条/页"],
          ),
          View({ type: "option", attributes: { value: "48" } }, ["48 条/页"]),
          View({ type: "option", attributes: { value: "96" } }, ["96 条/页"]),
        ],
      ),
      BrowseHistoryPageActionButton({
        icon: "chevron-left",
        title: "上一页",
        compact: true,
        class: computed(vm$.state.page, (page) =>
          page <= 1 ? "wx-content-action-disabled" : "",
        ),
        onClick() {
          vm$.methods.previousPage();
        },
      }),
      View({ class: "wx-content-page-number" }, [
        computed(
          { page: vm$.state.page, pageCount: vm$.state.page_count },
          (state) => `${state.page} / ${state.pageCount}`,
        ),
      ]),
      BrowseHistoryPageActionButton({
        icon: "chevron-right",
        title: "下一页",
        compact: true,
        class: computed(
          { page: vm$.state.page, pageCount: vm$.state.page_count },
          (state) =>
            state.page >= state.pageCount
              ? "wx-content-action-disabled"
              : "",
        ),
        onClick() {
          vm$.methods.nextPage();
        },
      }),
    ]),
  ]);
}

function BrowseHistoryPageView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-page",
      onMounted() {
        vm$.ready();
      },
    },
    [
      BrowseHistoryPageHeader({ store: vm$ }),
      View({ class: "wx-content-toolbar-wrap" }, [
        BrowseHistoryPageToolbar({ store: vm$ }),
      ]),
      BrowseHistoryPageBody({ store: vm$ }),
      BrowseHistoryPagination({ store: vm$ }),
    ],
  );
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = BrowseHistoryModel();
    Timeless.DOM.render(BrowseHistoryPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

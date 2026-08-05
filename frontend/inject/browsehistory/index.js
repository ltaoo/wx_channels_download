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

function BrowseHistoryCover(props) {
  const history = props.history;
  const fallback = View({ class: "wx-content-cover-fallback" }, [
    Timeless.Icon({ name: "file", size: 32 }),
    View({ class: "wx-content-cover-type" }, [
      props.store.methods.typeLabel(history.content_type),
    ]),
  ]);
  if (!history.cover_url) {
    return fallback;
  }
  return View({ class: "wx-content-cover-wrap" }, [
    fallback,
    Img({
      class: "wx-content-cover",
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

function BrowseHistoryAuthorItem(props) {
  const vm$ = props.store;
  const history = props.history;
  const name = vm$.methods.authorName(history);
  const showClickable = false;
  const avatar = history.author_avatar_url || "";
  return View(
    {
      type: showClickable ? "button" : "div",
      class: [
        "wx-content-account-item",
        showClickable ? "wx-content-account-item-clickable" : "",
      ]
        .filter(Boolean)
        .join(" "),
    },
    [
      Show({
        when: avatar,
        ok() {
          return Img({
            class: "wx-content-avatar",
            src: avatar,
            attributes: {
              alt: name,
              loading: "lazy",
              referrerpolicy: "no-referrer",
            },
            onError(event) {
              event.target.style.display = "none";
            },
          });
        },
        else() {
          return View(
            { class: "wx-content-avatar wx-content-avatar-fallback" },
            [String(name).slice(0, 1)],
          );
        },
      }),
      View({ class: "wx-content-account-details" }, [
        View({ class: "wx-content-account-name-line" }, [
          View(
            {
              class: "wx-content-account-name",
              attributes: { title: name },
            },
            [name],
          ),
        ]),
        View({ class: "wx-content-account-meta" }, [
          Timeless.Icon({ name: "user", size: 11 }),
          history.author_external_id ? history.author_external_id : "作者账号",
        ]),
      ]),
    ],
  );
}

function BrowseHistoryCard(props) {
  const vm$ = props.store;
  const history = props.history;
  return View(
    {
      class: ["wx-content-card", history.url ? "wx-content-card-clickable" : ""]
        .filter(Boolean)
        .join(" "),
      onClick() {
        vm$.methods.openSource(history);
      },
    },
    [
      View({ class: "wx-content-card-media" }, [
        BrowseHistoryCover({ store: vm$, history }),
      ]),
      View({ class: "wx-content-card-body" }, [
        View({ class: "wx-content-card-tags" }, [
          View({ class: "wx-content-platform" }, [
            vm$.methods.platformName(history),
          ]),
          View({ class: "wx-content-type-badge" }, [
            vm$.methods.typeLabel(history.content_type),
          ]),
        ]),
        View(
          {
            class: "wx-content-card-title",
            attributes: { title: history.title },
          },
          [history.title],
        ),
        View(
          {
            class: "wx-content-accounts",
          },
          [BrowseHistoryAuthorItem({ store: vm$, history })],
        ),
        View({ class: "wx-content-card-footer" }, [
          View({ class: "wx-content-meta" }, [
            Timeless.Icon({ name: "clock3", size: 13 }),
            vm$.methods.formatTime(history.publish_time),
          ]),
          View(
            { class: "wx-content-card-footer-right" },
            [
              history.visited_times > 0
                ? View(
                  { class: "wx-content-size" },
                  [`浏览 ${history.visited_times} 次`],
                )
                : null,
            ].filter(Boolean),
          ),
        ]),
      ]),
    ],
  );
}

function BrowseHistorySkeletonCard() {
  return View({ class: "wx-content-card wx-content-skeleton-card" }, [
    View({ class: "wx-content-card-media wx-content-skeleton" }),
    View({ class: "wx-content-card-body" }, [
      View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
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
          { class: "wx-content-grid" },
          Array.from({ length: 8 }, () => BrowseHistorySkeletonCard()),
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
                return View({ class: "wx-content-grid" }, [
                  For({
                    each: vm$.state.histories,
                    render(history_) {
                      const history =
                        history_ && history_.value !== undefined
                          ? history_.value
                          : history_;
                      return BrowseHistoryCard({ store: vm$, history });
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

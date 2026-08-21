import { BrowseHistoryViewModel } from "./browsehistory.model.js";
import { Table } from "./table.js";

function BrowseHistoryPageView(props) {
  const vm$ = BrowseHistoryViewModel(props);
  return View(
    {
      class:
        "wx-content-page wx-content-library-page wx-browse-history-page dm-page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "wx-content-toolbar-wrap" }, [
        BrowseHistoryPageToolbar({ store: vm$ }),
      ]),
      BrowseHistoryPageBody({ store: vm$ }),
      Show({
        when: computed(
          vm$.state.histories,
          (histories) => histories.length > 0,
        ),
        ok() {
          return Pagination({
            summary: vm$.state.range_text,
            page: vm$.state.page,
            pageCount: vm$.state.page_count,
            loading: vm$.state.loading,
            onPrevious() {
              vm$.methods.previousPage();
            },
            onNext() {
              vm$.methods.nextPage();
            },
          });
        },
      }),
    ],
  );
}

function BrowseHistoryPageActionButton(props) {
  return Button(
    {
      store: props.store,
      class: [
        "wx-content-page-button",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: props.type || "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
      prefix: props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
    },
    props.label
      ? [View({ class: "wx-content-action-label" }, [props.label])]
      : [],
  );
}

function BrowseHistoryPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-content-toolbar wx-content-filter-form",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View({ class: "wx-content-filter-fields" }, [
        View({ class: "wx-content-search wx-content-filter-search" }, [
          Timeless.Icon({ name: "search", size: 16 }),
          Input({
            store: vm$.ui.input_keyword$,
            class: "wx-content-search-input",
            attributes: {
              name: "keyword",
              type: "text",
              autocomplete: "off",
              "aria-label": "搜索浏览记录标题、账号或链接",
            },
          }),
        ]),
      ]),
      View({ class: "wx-content-filter-actions" }, [
        BrowseHistoryPageActionButton({
          store: vm$.ui.btn_search$,
          icon: "search",
          label: "搜索",
          type: "submit",
          onClick(event) {
            event.preventDefault();
            vm$.methods.search();
          },
        }),
        BrowseHistoryPageActionButton({
          store: vm$.ui.btn_refresh$,
          icon: "rotate-ccw",
          label: "重置",
        }),
      ]),
    ],
  );
}

function browse_history_cover_url(history) {
  return String((history && history.cover_url) || "").trim();
}

function BrowseHistoryRowCover(props) {
  const history = props.history;
  const cover_url = browse_history_cover_url(history);
  if (!cover_url) return null;
  return View({ class: "wx-content-row-cover-wrap" }, [
    View({ class: "wx-content-row-cover wx-content-row-cover-fallback" }, [
      Timeless.Icon({ name: "file", size: 18 }),
    ]),
    Img({
      class: "wx-content-row-cover",
      src: cover_url,
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

function BrowseHistorySourceAction(props) {
  return View(
    {
      type: "button",
      class: "wx-browse-history-source-action dm-focus-ring",
      attributes: { type: "button", title: "打开源站" },
      onClick(event) {
        event.stopPropagation();
        props.onClick();
      },
    },
    [
      View({ class: "wx-browse-history-source-action-label" }, ["源站"]),
      Timeless.Icon({ name: "external-link", size: 15 }),
    ],
  );
}

function BrowseHistoryRowMain(props) {
  const vm$ = props.store;
  const history = props.history;
  return [
    BrowseHistoryRowCover({ history }),
    View(
      {
        class: "wx-content-row-main",
        attributes: { n: "browse-history-main" },
      },
      [
        View(
          {
            class: "wx-content-row-title",
            attributes: { n: "browse-history-title", title: history.title },
          },
          [history.title],
        ),
        View(
          {
            class: "wx-content-row-badges",
            attributes: { n: "browse-history-badges" },
          },
          [
            View(
              {
                class: "wx-content-row-platform",
                attributes: { n: "browse-history-platform" },
              },
              [
                Show({
                  when: vm$.methods.platformFavicon(history),
                  ok() {
                    return Img({
                      class: "wx-content-row-platform-icon",
                      src: vm$.methods.platformFavicon(history),
                      attributes: {
                        n: "browse-history-platform-icon",
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
              ],
            ),
            View(
              {
                class: "wx-content-row-type",
                attributes: { n: "browse-history-content-type" },
              },
              [vm$.methods.typeLabel(history.content_type)],
            ),
          ],
        ),
      ],
    ),
  ];
}

function BrowseHistoryRowAccounts(props) {
  return [
    For({
      each: props.history.accounts || [],
      render(acc_) {
        const acc = acc_ && acc_.value !== undefined ? acc_.value : acc_;
        return View(
          {
            class: "wx-content-row-author-account",
            attributes: { n: "browse-history-account" },
          },
          [
            Show({
              when: acc.avatar_url,
              ok() {
                return Img({
                  class: "wx-content-row-author-avatar",
                  src: acc.avatar_url,
                  attributes: {
                    n: "browse-history-account-avatar",
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
                  n: "browse-history-account-name",
                  title: acc.nickname || acc.external_id || "",
                },
              },
              [acc.nickname || acc.external_id || "未知"],
            ),
          ],
        );
      },
    }),
  ];
}

function BrowseHistoryRowAccess(props) {
  const vm$ = props.store;
  const history = props.history;
  return [
    history.visited_times > 0
      ? View(
          {
            class: "wx-content-row-visits",
            attributes: { n: "browse-history-visit-count" },
          },
          [`浏览 ${history.visited_times} 次`],
        )
      : null,
    View(
      {
        class: "wx-content-row-meta",
        attributes: { n: "browse-history-updated-at" },
      },
      [
        Timeless.Icon({ name: "clock3", size: 12 }),
        vm$.methods.formatTime(history.updated_at),
      ],
    ),
  ].filter(Boolean);
}

function BrowseHistoryRowActions(props) {
  const vm$ = props.store;
  const history = props.history;
  return history.source_url
    ? [
        BrowseHistorySourceAction({
          onClick() {
            vm$.methods.openSource(history);
          },
        }),
      ]
    : [];
}

function BrowseHistorySkeletonRow() {
  return View({ class: "wx-content-row wx-content-skeleton-row" }, [
    View({ class: "wx-content-row-main-cell" }, [
      View({ class: "wx-content-row-cover wx-content-skeleton" }),
      View({ class: "wx-content-row-main" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
      ]),
    ]),
    View({ class: "wx-content-row-col-author" }, [
      View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
    ]),
    View({ class: "wx-browse-history-access" }, [
      View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
    ]),
    View({ class: "wx-browse-history-action-cell" }, [
      View({ class: "wx-content-skeleton wx-browse-history-action-skeleton" }),
    ]),
  ]);
}

function BrowseHistoryPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "browse-history-table",
    containerAttributes: { n: "browse-history-page-main" },
    panelAttributes: { n: "browse-history-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        cellClass: "wx-content-row-main-cell",
        render(history) {
          return BrowseHistoryRowMain({ store: vm$, history });
        },
      },
      {
        name: "account",
        title: "账号",
        headerClass: "wx-content-row-col-author",
        cellClass: "wx-content-row-author",
        cellAttributes(history) {
          return { title: vm$.methods.authorName(history) };
        },
        render(history) {
          return BrowseHistoryRowAccounts({ history });
        },
      },
      {
        name: "access",
        title: "最近访问时间",
        headerClass: "wx-content-row-col-meta",
        cellClass: "wx-browse-history-access",
        render(history) {
          return BrowseHistoryRowAccess({ store: vm$, history });
        },
      },
      {
        name: "actions",
        title: "操作",
        headerClass: "wx-browse-history-action-head",
        cellClass: "wx-browse-history-action-cell",
        render(history) {
          return BrowseHistoryRowActions({ store: vm$, history });
        },
      },
    ],
    rows: vm$.state.histories,
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    skeletonCount: 8,
    renderSkeletonRow: BrowseHistorySkeletonRow,
    onRow(history) {
      return {
        class: browse_history_cover_url(history)
          ? ""
          : "wx-content-row-no-cover",
      };
    },
    errorTitle: "浏览记录加载失败",
    retry: {
      store: vm$.ui.btn_retry$,
    },
    emptyTitle: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim()
        ? "没有匹配的浏览记录"
        : "暂无浏览记录",
    ),
    emptyDescription: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim()
        ? "请尝试其他标题、账号或链接"
        : "当前条件下未找到浏览记录",
    ),
  });
}

export default BrowseHistoryPageView;

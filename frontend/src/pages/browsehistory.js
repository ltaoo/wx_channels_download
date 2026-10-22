import { BrowseHistoryViewModel } from "./browsehistory.model.js";

function BrowseHistoryPageView(props) {
  const vm$ = BrowseHistoryViewModel(props);
  return View(
    {
      class: "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page responsive-table-page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "pt-4 pb-0 px-4 group-content-toolbar-wrap" }, [
        BrowseHistoryPageToolbar({ store: vm$ }),
      ]),
      BrowseHistoryPageBody({ store: vm$ }),
    ],
  );
}

function BrowseHistoryPageActionButton(props) {
  const semantic_name = props.name || "browse-history-action";
  return Button(
    {
      store: props.store,
      class: "dm-button--toolbar",
      attributes: {
        n: semantic_name,
        type: props.type || "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
      prefix: props.icon
        ? Timeless.Icon({
            name: props.icon,
            size: props.iconSize || 16,
            attributes: { n: `${semantic_name}-icon` },
          })
        : null,
    },
    props.label ? [props.label] : [],
  );
}

function BrowseHistoryPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "flex max-760:flex-wrap justify-between items-stretch gap-3 group-content-toolbar",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class: "min-w-0 grow shrink basis-0 max-760:grow max-760:shrink max-760:basis-full max-520:flex-wrap dm-items-center dm-gap-2 dm-flex",
        },
        [
          PlatformSelect({
            name: "browse-history-platform-select",
            store: vm$.ui.select_platform$,
            attributes: {
              n: "browse-history-platform-select-trigger",
              "aria-label": "按平台筛选浏览记录",
            },
          }),
          View(
            {
              class: "min-w-0 grow shrink basis-0 max-520:basis-full",
              attributes: { n: "browse-history-search-field" },
            },
            [
              Input({
                store: vm$.ui.input_keyword$,
                rootAttributes: { n: "browse-history-search-control" },
                prefix: Timeless.Icon({
                  name: "search",
                  size: 16,
                  attributes: { n: "browse-history-search-icon" },
                }),
                attributes: {
                  n: "browse-history-search-input",
                  name: "keyword",
                  type: "text",
                  autocomplete: "off",
                  "aria-label": "搜索浏览记录标题、账号或链接",
                },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "max-520:w-full flex-none dm-items-center max-760:ml-auto max-520:ml-0 dm-gap-2 dm-flex group-content-filter-actions toolbar-actions",
        },
        [
          BrowseHistoryPageActionButton({
            name: "browse-history-search-action",
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
            name: "browse-history-reset-action",
            store: vm$.ui.btn_refresh$,
            icon: "rotate-ccw",
            label: "重置",
          }),
        ],
      ),
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
  return View({ class: "relative w-control-height-lg h-control-height-lg cover-anchor " }, [
    LazyImg({
      class: "grid w-control-height-lg h-control-height-lg place-items-center overflow-hidden object-cover border-default border-solid border-color-content rounded-md text-content-muted cover-target",
      src: cover_url,
      alt: history.title,
      attributes: {
        loading: "lazy",
        referrerpolicy: "no-referrer",
      },
    }),
  ]);
}

function BrowseHistorySourceAction(props) {
  return Link(
    {
      class: "inline-flex min-h-32px max-520:w-32px items-center justify-center py-0 px-2.5 max-520:p-0 gap-1.5 font-inherit text-sm border-default border-solid border-color-content rounded-control hover:border-color-color-primary-border bg-content-panel-soft hover:bg-color-primary-soft text-content-primary hover:text-color-primary-hover cursor-pointer transition-colors-base dm-focus-ring",
      href: props.href,
      target: "_blank",
      attributes: {
        n: "browse-history-source-action",
        title: "打开源站",
        rel: "noopener noreferrer",
        "aria-label": "打开源站（新窗口）",
      },
    },
    [
      View(
        {
          class: "max-520:hidden",
          attributes: { n: "browse-history-source-action-label" },
        },
        ["源站"],
      ),
      Timeless.Icon({
        name: "external-link",
        size: 15,
        attributes: { n: "browse-history-source-action-icon" },
      }),
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
        class: "dm-min-w-0 dm-flex-1 content-row-main",
        attributes: { n: "browse-history-main" },
      },
      [
        View(
          {
            class: "overflow-hidden text-base font-semibold whitespace-nowrap text-ellipsis text-content-text",
            attributes: { n: "browse-history-title", title: history.title },
          },
          [history.title],
        ),
        View(
          {
            class: "min-w-0 flex-nowrap dm-items-center mt-1.5 dm-gap-1-5 overflow-hidden dm-flex",
            attributes: { n: "browse-history-badges" },
          },
          [
            PlatformTag({
              name: "browse-history-platform",
              favicon: vm$.methods.platformFavicon(history),
              label: vm$.methods.platformName(history),
            }),
            Tag(
              {
                class: "content-row-type",
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
            class:
              "max-w-full dm-min-w-0 dm-items-center dm-gap-1-5 dm-flex",
            attributes: { n: "browse-history-account" },
          },
          [
            Show({
              when: acc.avatar_url,
              ok() {
                return LazyImg({
                  class: "w-4 h-4 flex-none object-cover rounded-full",
                  src: acc.avatar_url,
                  attributes: {
                    n: "browse-history-account-avatar",
                    alt: acc.nickname || acc.external_id || "",
                    loading: "lazy",
                    referrerpolicy: "no-referrer",
                  },
                });
              },
            }),
            View(
              {
                class: "overflow-hidden text-sm whitespace-nowrap text-ellipsis text-content-muted",
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
            class:
              "dm-items-center dm-gap-1-5 dm-text-sm dm-font-medium dm-tabular-nums dm-whitespace-nowrap dm-text-primary dm-flex",
            attributes: { n: "browse-history-visit-count" },
          },
          [`浏览 ${history.visited_times} 次`],
        )
      : null,
    View(
      {
        class:
          "dm-items-center dm-gap-1-5 dm-text-sm dm-tabular-nums dm-whitespace-nowrap dm-text-muted dm-flex",
        attributes: { n: "browse-history-updated-at" },
      },
      [vm$.methods.formatTime(history.updated_at)],
    ),
  ].filter(Boolean);
}

function BrowseHistoryRowActions(props) {
  const history = props.history;
  return history.source_url
    ? [
        BrowseHistorySourceAction({
          href: history.source_url,
        }),
      ]
    : [];
}

function BrowseHistorySkeletonRow() {
  return View(
    {
      class: "dm-grid dm-items-center pointer-events-none dm-table-row",
      attributes: { n: "browse-history-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-min-w-0 dm-items-center dm-gap-4 dm-flex dm-table-cell table-primary-cell",
          attributes: { n: "browse-history-skeleton-main", role: "cell" },
        },
        [
          View({ class: "grid relative w-control-height-lg h-control-height-lg place-items-center overflow-hidden object-cover border-default border-solid border-color-content rounded-sm bg-skeleton-bg text-content-muted cover-target skeleton-shimmer " }),
          View({ class: "dm-min-w-0 dm-flex-1 content-row-main" }, [
            View({ class: "relative w-4/5 max-w-90 h-4 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
            View({ class: "relative h-3 w-16 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
          ]),
        ],
      ),
      View(
        {
          class: "dm-min-w-0 dm-table-cell",
          attributes: { n: "browse-history-skeleton-account", role: "cell" },
        },
        [View({ class: "relative h-3 w-4/5 max-w-45 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" })],
      ),
      View(
        {
          class: "flex min-w-0 flex-col items-start gap-1 dm-table-cell table-meta-cell",
          attributes: { n: "browse-history-skeleton-access", role: "cell" },
        },
        [
          View({ class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
          View({ class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
        ],
      ),
      View(
        {
          class: "flex min-w-0 items-center dm-table-cell table-action-cell",
          attributes: { n: "browse-history-skeleton-action", role: "cell" },
        },
        [
          View({
            class: "relative w-64px h-32px overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
          }),
        ],
      ),
    ],
  );
}

function BrowseHistoryPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "browse-history-table",
    containerClass: "min-w-0 flex-auto p-4 group-content-main ",
    containerAttributes: { n: "browse-history-page-main" },
    panelAttributes: { n: "browse-history-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        width: "minmax(300px, 2fr)",
        cellClass:
          "dm-min-w-0 dm-items-center dm-gap-4 dm-flex table-primary-cell",
        render(history) {
          return BrowseHistoryRowMain({ store: vm$, history });
        },
      },
      {
        name: "account",
        title: "账号",
        width: "minmax(150px, 1fr)",
        cellClass:
          "dm-min-w-0 dm-items-center dm-gap-1-5 overflow-hidden dm-flex table-secondary-cell",
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
        width: 180,
        cellClass: "flex min-w-0 flex-col items-start gap-1 table-meta-cell",
        render(history) {
          return BrowseHistoryRowAccess({ store: vm$, history });
        },
      },
      {
        name: "actions",
        title: "操作",
        width: 120,
        cellClass: "flex min-w-0 items-center table-action-cell",
        render(history) {
          return BrowseHistoryRowActions({ history });
        },
      },
    ],
    rows: vm$.state.histories,
    pagination: {
      class: " dm-px-4",
      summary: vm$.state.range_text,
      page: vm$.state.page,
      pageCount: vm$.state.page_count,
      pageSize: vm$.state.page_size,
      loading: vm$.state.loading,
      onChange(page) {
        return vm$.methods.changePage(page);
      },
    },
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    skeletonCount: 8,
    renderSkeletonRow: BrowseHistorySkeletonRow,
    onRow(history) {
      return {
        class: browse_history_cover_url(history) ? "" : "cover-placeholder",
      };
    },
    errorTitle: "浏览记录加载失败",
    retry: {
      store: vm$.ui.btn_retry$,
    },
    emptyTitle: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim() ? "没有匹配的浏览记录" : "暂无浏览记录",
    ),
    emptyDescription: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim()
        ? "请尝试其他标题、账号或链接"
        : "当前条件下未找到浏览记录",
    ),
  });
}

export default BrowseHistoryPageView;

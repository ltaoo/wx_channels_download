import { BrowseHistoryViewModel } from "./browsehistory.model.js";

function BrowseHistoryPageView(props) {
  const vm$ = BrowseHistoryViewModel(props);
  return View(
    {
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [.group-content-page&]:h-full [.group-content-page&]:min-h-0 [.group-content-page&]:overflow-hidden [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto] max-760:[&_.dm-table-header]:hidden max-760:[&_.dm-table-row:not(.dm-table-header)]:grid-cols-[minmax(0,_1fr)_auto_auto] max-760:[&_.dm-table-row:not(.dm-table-header)]:gap-[var(--dm-space-2)_var(--dm-space-3)] max-760:[&_.dm-table-row:not(.dm-table-header)]:min-h-92px max-760:[&_.group-content-row-main-cell]:grid-row-1 max-760:[&_.group-content-row-main-cell]:grid-column-1 max-760:[&_.group-content-row-author]:grid-row-2 max-760:[&_.group-content-row-author]:grid-column-1 max-760:[&_.group-content-row-author]:ml-[calc(var(--dm-control-height-lg)_+_var(--dm-space-3))] max-760:[&_.group-content-row-no-cover_.group-content-row-author]:ml-0 max-760:[&_.group-browse-history-access]:grid-row-[1_/_3] max-760:[&_.group-browse-history-access]:grid-column-2 max-760:[&_.group-browse-history-access]:items-end max-760:[&_.group-browse-history-action-cell]:grid-row-[1_/_3] max-760:[&_.group-browse-history-action-cell]:grid-column-3 min-w-0 min-h-full font-sans",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0]" }, [
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
      class: "group-content-toolbar flex justify-between gap-3 max-760:flex-wrap items-stretch",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class: "min-w-0 flex-[1_1_0] max-760:flex-[1_1_100%] max-520:flex-wrap dm-flex dm-items-center dm-gap-2",
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
              class: "min-w-0 flex-[1_1_0] max-520:basis-full",
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
          class: "group-content-filter-actions flex-[0_0_auto] max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2",
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
  return View({ class: "w-control-height-lg h-control-height-lg relative [&_>_.group-content-row-cover]:absolute [&_>_.group-content-row-cover]:inset-0" }, [
    LazyImg({
      class: "group-content-row-cover w-control-height-lg h-control-height-lg grid overflow-hidden place-items-center object-cover text-content-muted border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-md",
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
      class: "min-h-32px inline-flex items-center justify-center gap-1-5 p-[0_var(--dm-space-2-5)] text-content-primary font-inherit text-sm bg-content-panel-soft border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-control cursor-pointer transition-[color_var(--dm-duration-base)_var(--dm-ease-standard),_background_var(--dm-duration-base)_var(--dm-ease-standard),_border-color_var(--dm-duration-base)_var(--dm-ease-standard)] hover:text-color-primary-hover hover:bg-color-primary-soft hover:border-color-color-primary-border max-520:w-32px max-520:p-0 dm-focus-ring",
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
        class: "content-row-main dm-min-w-0 dm-flex-1",
        attributes: { n: "browse-history-main" },
      },
      [
        View(
          {
            class: "overflow-hidden text-content-text text-base font-semibold text-ellipsis whitespace-nowrap",
            attributes: { n: "browse-history-title", title: history.title },
          },
          [history.title],
        ),
        View(
          {
            class: "mt-1-5 min-w-0 overflow-hidden flex-nowrap dm-flex dm-items-center dm-gap-1-5",
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
              "max-w-full dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
            attributes: { n: "browse-history-account" },
          },
          [
            Show({
              when: acc.avatar_url,
              ok() {
                return LazyImg({
                  class: "w-4 h-4 flex-[0_0_auto] object-cover rounded-full",
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
                class: "overflow-hidden text-content-muted text-sm text-ellipsis whitespace-nowrap",
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
              "dm-flex dm-items-center dm-gap-1-5 dm-text-primary dm-text-sm dm-font-medium dm-tabular-nums dm-whitespace-nowrap",
            attributes: { n: "browse-history-visit-count" },
          },
          [`浏览 ${history.visited_times} 次`],
        )
      : null,
    View(
      {
        class:
          "dm-flex dm-items-center dm-gap-1-5 dm-text-muted dm-text-sm dm-tabular-nums dm-whitespace-nowrap",
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
      class: "dm-table-row dm-grid dm-items-center pointer-events-none",
      attributes: { n: "browse-history-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-table-cell group-content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
          attributes: { n: "browse-history-skeleton-main", role: "cell" },
        },
        [
          View({ class: "group-content-row-cover w-control-height-lg h-control-height-lg grid overflow-hidden place-items-center object-cover text-content-muted border-[var(--dm-border-width)_solid_var(--dm-content-border)] relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite]" }),
          View({ class: "content-row-main dm-min-w-0 dm-flex-1" }, [
            View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-[min(360px,_80%)] h-4" }),
            View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-16" }),
          ]),
        ],
      ),
      View(
        {
          class: "dm-table-cell dm-min-w-0",
          attributes: { n: "browse-history-skeleton-account", role: "cell" },
        },
        [View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(180px,_80%)]" })],
      ),
      View(
        {
          class: "dm-table-cell group-browse-history-access min-w-0 flex items-start flex-col gap-1",
          attributes: { n: "browse-history-skeleton-access", role: "cell" },
        },
        [
          View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]" }),
          View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]" }),
        ],
      ),
      View(
        {
          class: "dm-table-cell group-browse-history-action-cell min-w-0 flex items-center",
          attributes: { n: "browse-history-skeleton-action", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-64px h-32px",
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
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4 ",
    containerAttributes: { n: "browse-history-page-main" },
    panelAttributes: { n: "browse-history-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        width: "minmax(300px, 2fr)",
        cellClass:
          "group-content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
        render(history) {
          return BrowseHistoryRowMain({ store: vm$, history });
        },
      },
      {
        name: "account",
        title: "账号",
        width: "minmax(150px, 1fr)",
        cellClass:
          "group-content-row-author overflow-hidden dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
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
        cellClass: "group-browse-history-access min-w-0 flex items-start flex-col gap-1",
        render(history) {
          return BrowseHistoryRowAccess({ store: vm$, history });
        },
      },
      {
        name: "actions",
        title: "操作",
        width: 120,
        cellClass: "group-browse-history-action-cell min-w-0 flex items-center",
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
        class: browse_history_cover_url(history) ? "" : "group-content-row-no-cover",
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

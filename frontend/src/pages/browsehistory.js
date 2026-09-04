import { BrowseHistoryViewModel } from "./browsehistory.model.js";
import { TablePlatformBadge } from "../components.js";

function BrowseHistoryPageView(props) {
  const vm$ = BrowseHistoryViewModel(props);
  return View(
    {
      class:
        "content-page content-library-page browse-history-page page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "content-toolbar-wrap" }, [
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
      class: "content-toolbar content-filter-form",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class:
            "content-filter-fields dm-flex dm-items-center dm-gap-2",
        },
        [
        View(
          {
            class: "content-filter-search",
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
          class:
            "content-filter-actions dm-flex dm-items-center dm-gap-2",
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
  return View({ class: "content-row-cover-wrap" }, [
    LazyImg({
      class: "content-row-cover",
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
      class: "browse-history-source-action dm-focus-ring",
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
          class: "browse-history-source-action-label",
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
            class: "content-row-title",
            attributes: { n: "browse-history-title", title: history.title },
          },
          [history.title],
        ),
        View(
          {
            class:
              "content-row-badges dm-flex dm-items-center dm-gap-1-5",
            attributes: { n: "browse-history-badges" },
          },
          [
            TablePlatformBadge({
              name: "browse-history-platform",
              favicon: vm$.methods.platformFavicon(history),
              label: vm$.methods.platformName(history),
            }),
            View(
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
              "content-row-author-account dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
            attributes: { n: "browse-history-account" },
          },
          [
            Show({
              when: acc.avatar_url,
              ok() {
                return LazyImg({
                  class: "content-row-author-avatar",
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
                class: "content-row-author-name",
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
      [
        vm$.methods.formatTime(history.updated_at),
      ],
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
      class: "dm-table-row dm-grid dm-items-center content-skeleton-row",
      attributes: { n: "browse-history-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-table-cell content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
          attributes: { n: "browse-history-skeleton-main", role: "cell" },
        },
        [
          View({ class: "content-row-cover content-skeleton" }),
          View({ class: "content-row-main dm-min-w-0 dm-flex-1" }, [
            View({ class: "content-skeleton content-skeleton-title" }),
            View({ class: "content-skeleton content-skeleton-tag" }),
          ]),
        ],
      ),
      View(
        {
          class: "dm-table-cell dm-min-w-0",
          attributes: { n: "browse-history-skeleton-account", role: "cell" },
        },
        [View({ class: "content-skeleton content-skeleton-line" })],
      ),
      View(
        {
          class: "dm-table-cell browse-history-access",
          attributes: { n: "browse-history-skeleton-access", role: "cell" },
        },
        [
          View({ class: "content-skeleton content-skeleton-line-short" }),
          View({ class: "content-skeleton content-skeleton-line-short" }),
        ],
      ),
      View(
        {
          class: "dm-table-cell browse-history-action-cell",
          attributes: { n: "browse-history-skeleton-action", role: "cell" },
        },
        [
          View({
            class: "content-skeleton browse-history-action-skeleton",
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
    containerClass: "content-main container",
    containerAttributes: { n: "browse-history-page-main" },
    panelAttributes: { n: "browse-history-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        width: "minmax(300px, 2fr)",
        cellClass:
          "content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
        render(history) {
          return BrowseHistoryRowMain({ store: vm$, history });
        },
      },
      {
        name: "account",
        title: "账号",
        width: "minmax(150px, 1fr)",
        cellClass:
          "content-row-author dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
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
        cellClass: "browse-history-access",
        render(history) {
          return BrowseHistoryRowAccess({ store: vm$, history });
        },
      },
      {
        name: "actions",
        title: "操作",
        width: 120,
        cellClass: "browse-history-action-cell",
        render(history) {
          return BrowseHistoryRowActions({ history });
        },
      },
    ],
    rows: vm$.state.histories,
    pagination: {
      class: "container dm-px-4",
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
        class: browse_history_cover_url(history)
          ? ""
          : "content-row-no-cover",
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

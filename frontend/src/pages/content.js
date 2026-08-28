import { ContentViewModel } from "./content.model.js";
import ContentDetailPageView from "./content_detail.js";
import { TablePlatformBadge } from "../components.js";

function ContentDetailDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.contentDetailDrawer$,
      class: "dm-drawer--wide",
      attributes: { n: "content-detail-drawer" },
    },
    [
      ContentDetailPageView({
        app: props.app,
        client: props.client,
        history: props.history,
        embedded: true,
        contentId: vm$.state.detail_id,
        onBack() {
          vm$.ui.contentDetailDrawer$.hide();
        },
      }),
    ],
  );
}

function ContentPageView(props) {
  const vm$ = ContentViewModel(props);
  return View(
    {
      class: "content-page content-library-page content-list-page page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "content-toolbar-wrap" }, [
        ContentPageToolbar({ store: vm$ }),
      ]),
      ContentPageBody({ store: vm$ }),
      ContentDetailDrawer({
        store: vm$,
        app: props.app,
        client: props.client,
        history: props.history,
      }),
    ],
  );
}

function ContentPageActionButton(props) {
  const semantic_name = props.name || "content-action";
  return Button(
    {
      store: props.store,
      class: "dm-button--toolbar",
      attributes: {
        n: semantic_name,
        type: (props.attributes && props.attributes.type) || "button",
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

function ContentPageToolbar(props) {
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
            attributes: { n: "content-search-field" },
          },
          [
            Input({
              store: vm$.ui.input_keyword$,
              rootAttributes: { n: "content-search-control" },
              prefix: Timeless.Icon({
                name: "search",
                size: 16,
                attributes: { n: "content-search-icon" },
              }),
              attributes: {
                n: "content-search-input",
                name: "keyword",
                type: "search",
                autocomplete: "off",
                "aria-label": "搜索内容标题或描述",
              },
            }),
          ],
        ),
        // Select({
        //   store: vm$.ui.select_content_type$,
        //   class: "content-type-select content-filter-select",
        //   attributes: { "aria-label": "筛选内容类型" },
        // }),
        ],
      ),
      View(
        {
          class:
            "content-filter-actions dm-flex dm-items-center dm-gap-2",
        },
        [
        ContentPageActionButton({
          name: "content-search-action",
          store: vm$.ui.btn_search$,
          icon: "search",
          label: "搜索",
          variant: "primary",
          attributes: { type: "submit" },
          onClick(event) {
            event.preventDefault();
            vm$.methods.search();
          },
        }),
        ContentPageActionButton({
          name: "content-reset-action",
          store: vm$.ui.btn_refresh$,
          icon: "rotate-ccw",
          label: "重置",
        }),
        View(
          {
            class: "content-scope-toggle",
            attributes: { n: "content-scope-toggle" },
          },
          [
            Checkbox({
              store: vm$.ui.checkbox_all$,
              id: "wxContentScopeAll",
              text: "所有",
              textAttributes: { n: "content-scope-all-text" },
              attributes: {
                n: "content-scope-all-checkbox",
                "aria-label": "显示所有内容",
              },
            }),
          ],
        ),
        ],
      ),
    ],
  );
}

function content_cover_url(content) {
  return String((content && content.cover_url) || "").trim();
}

function ContentRowCover(props) {
  const content = props.content;
  const cover_url = content_cover_url(content);
  if (!cover_url) return null;
  const fallback = View(
    { class: "content-row-cover content-row-cover-fallback" },
    [Timeless.Icon({ name: "file", size: 18 })],
  );
  return View({ class: "content-row-cover-wrap" }, [
    fallback,
    LazyImg({
      class: "content-row-cover",
      src: cover_url,
      alt: content.title,
      attributes: {
        referrerpolicy: "no-referrer",
      },
    }),
  ]);
}

function ContentRowAccounts(props) {
  const accounts = props.content.accounts || [];
  if (accounts.length === 0) {
    return ["暂无关联账号"];
  }
  return [
    For({
      each: accounts,
      render(account_) {
        const account =
          account_ && account_.value !== undefined
            ? account_.value
            : account_;
        const name =
          account.nickname || account.alias || account.external_id || "未知";
        return View(
          {
            class:
              "content-row-author-account dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
          },
          [
          Show({
            when: account.avatar_url,
            ok() {
              return Img({
                class: "content-row-author-avatar",
                src: account.avatar_url,
                alt: name,
                attributes: {
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
                class: "content-row-author-name",
                attributes: { title: name },
              },
              [name],
            ),
          ],
        );
      },
    }),
  ];
}

function ContentRowStatistics(props) {
  const statistics = props.statistics;
  const items = [
    { key: "in-progress", label: "进行中任务", value: statistics.in_progress },
    { key: "failed", label: "失败任务", value: statistics.failed },
    { key: "success", label: "成功任务", value: statistics.total_tasks },
    { key: "files", label: "文件", value: statistics.files },
  ].filter((item) => item.value > 0);
  return [
    For({
      each: items,
      render(item) {
        return View(
          {
            class: `content-row-stat content-row-stat-${item.key}`,
            attributes: { title: `${item.label}：${item.value}` },
          },
          [
            View({ class: "content-row-stat-value" }, [String(item.value)]),
            View({ class: "content-row-stat-label" }, [item.label]),
          ],
        );
      },
    }),
  ];
}

function ContentRowMain(props) {
  const vm$ = props.store;
  const content = props.content;
  const favicon = window.PLATFORM_FAVICONS[content.platform_id] || "";
  const title = content.title || "\u00a0";
  return [
    ContentRowCover({ content }),
    View({ class: "content-row-main dm-min-w-0 dm-flex-1" }, [
      View(
        {
          class: "content-row-title",
          attributes: { title: content.title },
        },
        [title],
      ),
      View(
        {
          class:
            "content-row-badges dm-flex dm-items-center dm-gap-1-5",
        },
        [
        TablePlatformBadge({
          name: "content-platform",
          favicon,
          label: vm$.methods.platformName(content),
        }),
        View({ class: "content-row-type" }, [
          vm$.methods.typeLabel(content.content_type),
        ]),
        Show({
          when: content.content_subtype,
          ok() {
            return View(
              {
                class: "content-row-type content-row-subtype",
                attributes: {
                  n: "content-subtype",
                  title: `subtype: ${content.content_subtype}`,
                },
              },
              [content.content_subtype],
            );
          },
        }),
        ],
      ),
    ]),
  ];
}

function ContentSkeletonRow() {
  return View(
    {
      class: "dm-table-row dm-grid dm-items-center content-skeleton-row",
      attributes: { n: "content-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-table-cell content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
          attributes: { n: "content-table-skeleton-main-cell", role: "cell" },
        },
        [
          View({
            class: "content-row-cover content-skeleton",
            attributes: { n: "content-table-skeleton-cover" },
          }),
          View(
            {
              class: "content-row-main dm-min-w-0 dm-flex-1",
              attributes: { n: "content-table-skeleton-main" },
            },
            [
              View({
                class: "content-skeleton content-skeleton-title",
                attributes: { n: "content-table-skeleton-title" },
              }),
              View({
                class: "content-skeleton content-skeleton-tag",
                attributes: { n: "content-table-skeleton-tag" },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "content-table-skeleton-account", role: "cell" },
        },
        [
          View({
            class: "content-skeleton content-skeleton-line",
            attributes: { n: "content-table-skeleton-account-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "content-table-skeleton-time", role: "cell" },
        },
        [
          View({
            class: "content-skeleton content-skeleton-line-short",
            attributes: { n: "content-table-skeleton-time-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: {
            n: "content-table-skeleton-statistics",
            role: "cell",
          },
        },
        [
          View({
            class: "content-skeleton content-skeleton-line-short",
            attributes: { n: "content-table-skeleton-statistics-value" },
          }),
        ],
      ),
    ],
  );
}

function ContentPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "content-table",
    containerClass: "content-main container",
    containerAttributes: { n: "content-page-main" },
    panelAttributes: { n: "content-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        width: "minmax(300px, 2fr)",
        cellClass:
          "content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
        render(content) {
          return ContentRowMain({ store: vm$, content });
        },
      },
      {
        name: "account",
        title: "账号",
        width: "minmax(150px, 1fr)",
        cellClass:
          "content-row-author dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
        render(content) {
          return ContentRowAccounts({ content });
        },
      },
      {
        name: "time",
        title: "时间",
        width: 240,
        cellClass:
          "content-row-meta dm-flex dm-items-center dm-gap-1-5 dm-text-muted dm-text-sm dm-tabular-nums dm-whitespace-nowrap",
        render(content) {
          return [
            View(
              { attributes: { n: "content-time" } },
              [
                View(
                  { attributes: { n: "content-publish-time" } },
                  [`发布时间: ${vm$.methods.formatTime(content.publish_time)}`],
                ),
                View(
                  { attributes: { n: "content-created-at" } },
                  [`创建时间: ${vm$.methods.formatTime(content.created_at)}`],
                ),
              ],
            ),
          ];
        },
      },
      {
        name: "statistics",
        title: "统计",
        width: 200,
        cellClass: "content-row-stats",
        render(content) {
          return ContentRowStatistics({
            statistics: vm$.methods.statistics(content),
          });
        },
      },
    ],
    rows: vm$.state.contents,
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
    renderSkeletonRow: ContentSkeletonRow,
    onRow(content) {
      const detail_href = vm$.methods.detailHref(content);
      return {
        class: detail_href ? "content-row-clickable" : "",
        attributes: detail_href ? { title: "查看内容详情" } : {},
        onClick() {
          vm$.methods.openDetail(content);
        },
      };
    },
    errorTitle: "内容加载失败",
    retry: {
      store: vm$.ui.btn_retry$,
    },
    emptyTitle: "暂无内容",
    emptyDescription: "当前筛选条件下没有内容",
  });
}

export default ContentPageView;

import { ContentViewModel } from "./content.model.js";
import ContentDetailPageView from "./content_detail.js";
import { Table } from "./table.js";

function ContentDetailDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.contentDetailDrawer$,
      class: "wx-content-detail-drawer",
      style: { width: "min(max(560px, 80vw), 100vw)" },
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
      class:
        "wx-content-page wx-content-library-page wx-content-list-page wx-browse-history-page dm-page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "wx-content-toolbar-wrap" }, [
        ContentPageToolbar({ store: vm$ }),
      ]),
      ContentPageBody({ store: vm$ }),
      Show({
        when: computed(vm$.state.contents, (contents) => contents.length > 0),
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
        type: (props.attributes && props.attributes.type) || "button",
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

function ContentPageToolbar(props) {
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
              type: "search",
              autocomplete: "off",
              "aria-label": "搜索内容标题或描述",
            },
          }),
        ]),
        View(
          {
            class: "wx-content-scope-toggle",
            style: {
              display: "flex",
              "align-items": "center",
              gap: "var(--dm-space-2)",
              "white-space": "nowrap",
            },
            attributes: { n: "content-scope-toggle" },
          },
          [
            Checkbox({
              store: vm$.ui.checkbox_all$,
              id: "wxContentScopeAll",
              attributes: {
                n: "content-scope-all-checkbox",
                "aria-label": "显示所有内容",
              },
            }),
            View(
              {
                type: "label",
                attributes: {
                  n: "content-scope-all-label",
                  for: "wxContentScopeAll",
                },
              },
              ["所有"],
            ),
          ],
        ),
        // Select({
        //   store: vm$.ui.select_content_type$,
        //   class: "wx-content-type-select wx-content-filter-select",
        //   attributes: { "aria-label": "筛选内容类型" },
        // }),
      ]),
      View({ class: "wx-content-filter-actions" }, [
        ContentPageActionButton({
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
          store: vm$.ui.btn_refresh$,
          icon: "rotate-ccw",
          label: "重置",
        }),
      ]),
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
    { class: "wx-content-row-cover wx-content-row-cover-fallback" },
    [Timeless.Icon({ name: "file", size: 18 })],
  );
  return View({ class: "wx-content-row-cover-wrap" }, [
    fallback,
    LazyImg({
      class: "wx-content-row-cover",
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
        return View({ class: "wx-content-row-author-account" }, [
          Show({
            when: account.avatar_url,
            ok() {
              return Img({
                class: "wx-content-row-author-avatar",
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
              class: "wx-content-row-author-name",
              attributes: { title: name },
            },
            [name],
          ),
        ]);
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
            class: `wx-content-row-stat wx-content-row-stat-${item.key}`,
            attributes: { title: `${item.label}：${item.value}` },
          },
          [
            View({ class: "wx-content-row-stat-value" }, [String(item.value)]),
            View({ class: "wx-content-row-stat-label" }, [item.label]),
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
  return [
    ContentRowCover({ content }),
    View({ class: "wx-content-row-main" }, [
      View(
        {
          class: "wx-content-row-title",
          attributes: { title: content.title },
        },
        [content.title],
      ),
      View({ class: "wx-content-row-badges" }, [
        View({ class: "wx-content-row-platform" }, [
          Show({
            when: favicon,
            ok() {
              return Img({
                class: "wx-content-row-platform-icon",
                src: favicon,
                alt: "",
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
          vm$.methods.platformName(content),
        ]),
        View({ class: "wx-content-row-type" }, [
          vm$.methods.typeLabel(content.content_type),
        ]),
        Show({
          when: content.content_subtype,
          ok() {
            return View(
              {
                class: "wx-content-row-type wx-content-row-subtype",
                attributes: {
                  n: "content-subtype",
                  title: `subtype: ${content.content_subtype}`,
                },
              },
              [content.content_subtype],
            );
          },
        }),
      ]),
    ]),
  ];
}

function ContentSkeletonRow() {
  return View(
    {
      class: "wx-content-row wx-content-skeleton-row",
      attributes: { n: "content-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "wx-content-row-main-cell",
          attributes: { n: "content-table-skeleton-main-cell", role: "cell" },
        },
        [
          View({
            class: "wx-content-row-cover wx-content-skeleton",
            attributes: { n: "content-table-skeleton-cover" },
          }),
          View(
            {
              class: "wx-content-row-main",
              attributes: { n: "content-table-skeleton-main" },
            },
            [
              View({
                class: "wx-content-skeleton wx-content-skeleton-title",
                attributes: { n: "content-table-skeleton-title" },
              }),
              View({
                class: "wx-content-skeleton wx-content-skeleton-tag",
                attributes: { n: "content-table-skeleton-tag" },
              }),
            ],
          ),
        ],
      ),
      View({
        class: "wx-content-skeleton wx-content-skeleton-line",
        attributes: { n: "content-table-skeleton-account", role: "cell" },
      }),
      View({
        class: "wx-content-skeleton wx-content-skeleton-line-short",
        attributes: {
          n: "content-table-skeleton-publish-time",
          role: "cell",
        },
      }),
      View({
        class: "wx-content-skeleton wx-content-skeleton-line-short",
        attributes: {
          n: "content-table-skeleton-statistics",
          role: "cell",
        },
      }),
    ],
  );
}

function ContentPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "content-table",
    containerAttributes: { n: "content-page-main" },
    panelAttributes: { n: "content-table-panel" },
    columns: [
      {
        name: "main",
        title: "封面 / 标题",
        cellClass: "wx-content-row-main-cell",
        render(content) {
          return ContentRowMain({ store: vm$, content });
        },
      },
      {
        name: "account",
        title: "账号",
        cellClass: "wx-content-row-author",
        render(content) {
          return ContentRowAccounts({ content });
        },
      },
      {
        name: "publish-time",
        title: "发布时间",
        cellClass: "wx-content-row-meta",
        render(content) {
          return [
            Timeless.Icon({ name: "clock3", size: 12 }),
            vm$.methods.formatTime(content.publish_time),
          ];
        },
      },
      {
        name: "statistics",
        title: "统计",
        cellClass: "wx-content-row-stats",
        render(content) {
          return ContentRowStatistics({
            statistics: vm$.methods.statistics(content),
          });
        },
      },
    ],
    rows: vm$.state.contents,
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    skeletonCount: 8,
    renderSkeletonRow: ContentSkeletonRow,
    onRow(content) {
      const detail_href = vm$.methods.detailHref(content);
      return {
        class: detail_href ? "wx-content-row-clickable" : "",
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

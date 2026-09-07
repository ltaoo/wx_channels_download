import { AccountViewModel } from "./account.model.js";
import {
  BrandEmpty,
  BrandError,
  BrandLoading,
  Tag,
  PlatformTag,
  Card,
  Tab,
  Tabs,
  Waterfall,
} from "../dmui.js";

const ACCOUNT_TEXT_CONTENT_TYPES = new Set(["answer", "webpage", "text"]);

function AccountPageView(props) {
  const vm$ = AccountViewModel(props);

  return View(
    {
      class: "content-page account-page page",
      attributes: { n: "account-page" },
      onMounted() {
        console.count("AccountPage onMounted");
        vm$.methods.ready();
      },
    },
    [
      View({
        class: "content-toolbar-wrap account-toolbar-wrap",
        attributes: { n: "account-toolbar-wrap" },
      }, [
        AccountPageToolbar({ store: vm$ }),
      ]),
      AccountPageBody({ store: vm$ }),
      AccountContentsDrawer({ store: vm$ }),
    ],
  );
}

function account_content_duration(value) {
  let seconds = Math.max(0, Number(value) || 0);
  if (seconds > 36000) seconds /= 1000;
  const minutes = Math.floor(seconds / 60);
  const remainder = Math.floor(seconds % 60);
  return `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function account_content_card_height(content, _index, column_width) {
  const width = Math.max(180, Number(column_width) || 240);
  const images = Array.isArray(content && content.preview_images)
    ? content.preview_images
    : [];
  const kind = String(content && content.content_type || "").toLowerCase();
  if (ACCOUNT_TEXT_CONTENT_TYPES.has(kind)) {
    const title = String(
      content && (content.title || content.external_id) || "未命名内容",
    );
    const description = String(content && content.description || "").trim();
    const characters_per_line = Math.max(12, Math.floor((width - 32) / 14));
    const title_lines = Math.min(
      4,
      Math.max(1, Math.ceil(title.length / characters_per_line)),
    );
    const description_lines = description
      ? Math.min(6, Math.ceil(description.length / characters_per_line))
      : 0;
    return 32 + title_lines * 24 + description_lines * 23 +
      (description_lines > 0 ? 8 : 0);
  }
  let height = 190;
  if (images.length > 0) {
    if (images.length > 1) {
      const ratio = images.length === 2 ? 3 / 2 : 4 / 3;
      height = Math.min(420, width / ratio);
    } else {
      const fallback_ratio = kind === "album" ? 4 / 3 : 16 / 9;
      const ratio = Math.max(
        0.56,
        Math.min(
          1.9,
          Number(content.preview_aspect_ratio) || fallback_ratio,
        ),
      );
      height = Math.max(170, Math.min(420, width / ratio));
    }
  }
  return height;
}

function AccountContentCoverMeta(props) {
  return View(
    {
      class: "account-content-card-cover-meta",
      attributes: { n: "account-content-card-cover-meta" },
    },
    [
      View(
        {
          class: "account-content-card-cover-kicker",
          attributes: { n: "account-content-card-cover-kicker" },
        },
        [
          Tag(
            {
              class: "account-content-card-type",
              attributes: { n: "account-content-card-type" },
            },
            [props.typeLabel],
          ),
          View(
            {
              class: "account-content-card-time",
              attributes: { n: "account-content-card-time" },
            },
            [props.publishTime],
          ),
        ],
      ),
      View(
        {
          as: "h3",
          class: "account-content-card-title",
          attributes: { n: "account-content-card-title" },
        },
        [props.title],
      ),
    ],
  );
}

function AccountContentMedia(props) {
  const content = props.content;
  const images = Array.isArray(content.preview_images)
    ? content.preview_images.filter(Boolean)
    : [];
  const kind = String(content.content_type || "").toLowerCase();
  if (images.length === 0) {
    return View(
      {
        class: `account-content-card-media account-content-card-media-empty account-content-card-media-${kind || "unknown"}`,
        attributes: { n: "account-content-card-media-empty" },
      },
      [
        View(
          {
            class: "account-content-card-media-symbol",
            attributes: { "aria-hidden": "true" },
          },
          [Timeless.Icon({ name: kind === "answer" ? "message-circle" : "file-text", size: 42 })],
        ),
        props.coverMeta,
      ],
    );
  }

  if (kind === "album" || images.length > 1) {
    const visible_images = images.slice(0, 4);
    const single_image_ratio = Math.max(
      0.56,
      Math.min(1.9, Number(content.preview_aspect_ratio) || 4 / 3),
    );
    return View(
      {
        class: `account-content-card-media account-content-card-gallery account-content-card-gallery-${Math.min(visible_images.length, 4)}`,
        style: visible_images.length === 1
          ? { "aspect-ratio": String(single_image_ratio) }
          : undefined,
        attributes: { n: "account-content-card-gallery" },
      },
      [
        ...visible_images.map((image_url, index) =>
          View(
            {
              class: "account-content-card-gallery-item",
              attributes: { n: "account-content-card-gallery-item" },
            },
            [
              LazyImg({
                class: "account-content-card-image",
                src: image_url,
                alt: `${content.title || "图集"} ${index + 1}`,
                attributes: {
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
              }),
              index === visible_images.length - 1 && images.length > 4
                ? View(
                    {
                      class: "account-content-card-gallery-more",
                      attributes: { n: "account-content-card-gallery-more" },
                    },
                    [`+${images.length - 4}`],
                  )
                : null,
            ].filter(Boolean),
          )
        ),
        props.coverMeta,
      ],
    );
  }

  return View(
    {
      class: "account-content-card-media account-content-card-poster",
      style: {
        "aspect-ratio": Number(content.preview_aspect_ratio) > 0
          ? String(content.preview_aspect_ratio)
          : "16 / 9",
      },
      attributes: { n: "account-content-card-poster" },
    },
    [
      LazyImg({
        class: "account-content-card-image",
        src: images[0],
        alt: content.title || "内容封面",
        attributes: {
          loading: "lazy",
          referrerpolicy: "no-referrer",
        },
      }),
      kind === "video"
        ? View(
            {
              class: "account-content-card-play",
              attributes: {
                n: "account-content-card-play",
                "aria-hidden": "true",
              },
            },
            [Timeless.Icon({ name: "play", size: 20 })],
          )
        : null,
      kind === "video" && Number(content.duration) > 0
        ? View(
            {
              class: "account-content-card-duration",
              attributes: { n: "account-content-card-duration" },
            },
            [account_content_duration(content.duration)],
          )
        : null,
      props.coverMeta,
    ].filter(Boolean),
  );
}

function AccountContentCard(props) {
  const vm$ = props.store;
  const content = props.content;
  const source_url = String(content.url || "").trim();
  const title = content.title || content.external_id || "未命名内容";
  const description = String(content.description || "").trim();
  const kind = String(content.content_type || "unknown").toLowerCase();
  const text_only = ACCOUNT_TEXT_CONTENT_TYPES.has(kind);
  const cover_meta = text_only
    ? null
    : AccountContentCoverMeta({
        title,
        typeLabel: vm$.methods.contentTypeLabel(
          content.content_type,
          content.content_subtype,
        ),
        publishTime: vm$.methods.formatTime(content.publish_time),
      });
  return Card(
    {
      as: source_url ? "button" : "article",
      class: [
        "account-content-card",
        `account-content-card-${kind}`,
        text_only ? "account-content-card-text-only" : "",
        source_url ? "account-content-card-clickable" : "",
      ].filter(Boolean).join(" "),
      attributes: {
        n: "account-content-card",
        type: source_url ? "button" : undefined,
        title: source_url ? "打开原内容" : undefined,
      },
      onClick() {
        if (source_url) vm$.methods.openContent(content);
      },
    },
    text_only
      ? [
          View(
            {
              as: "h3",
              class: "account-content-card-text-title",
              attributes: { n: "account-content-card-text-title" },
            },
            [title],
          ),
          description
            ? View(
                {
                  as: "p",
                  class: "account-content-card-description",
                  attributes: { n: "account-content-card-description" },
                },
                [description],
              )
            : null,
        ].filter(Boolean)
      : [AccountContentMedia({ content, coverMeta: cover_meta })],
  );
}

function AccountContentState(props) {
  const role = props.error ? "alert" : "status";
  return View(
    {
      class: `account-content-state${props.error ? " is-error" : ""}`,
      attributes: { n: "account-content-state", role },
    },
    [
      props.loading
        ? BrandLoading({
            size: 112,
            label: props.title || "正在加载账号内容",
            decorative: true,
            name: "account-content-loading-symbol",
          })
        : props.error
          ? BrandError({ size: 116, name: "account-content-error-symbol" })
          : BrandEmpty({ size: 116, name: "account-content-empty-symbol" }),
      View({ as: "h3", class: "account-content-state-title" }, [props.title]),
      props.description
        ? View({ as: "p", class: "account-content-state-description" }, [
            props.description,
          ])
        : null,
      props.action || null,
    ].filter(Boolean),
  );
}

function AccountContentsCollection(props) {
  const vm$ = props.store;
  return View(
    {
      class: "account-content-collection",
      attributes: { n: "account-content-collection" },
    },
    [
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "initial"),
        ok() {
          return AccountContentState({
            loading: true,
            title: "正在加载账号内容",
          });
        },
      }),
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "error"),
        ok() {
          return AccountContentState({
            error: true,
            title: "内容加载失败",
            description: vm$.state.drawer_error,
            action: AccountPageActionButton({
              name: "account-content-retry-action",
              store: vm$.ui.btn_drawer_retry$,
              icon: "rotate-ccw",
              label: "重试",
            }),
          });
        },
      }),
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "empty"),
        ok() {
          return AccountContentState({
            title: "暂无内容",
            description: vm$.state.drawer_empty_description,
          });
        },
      }),
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "normal"),
        ok() {
          return Waterfall({
            class: "account-content-waterfall",
            attributes: {
              n: "account-content-waterfall",
              "aria-label": "账号内容列表",
            },
            each: vm$.state.drawer_contents,
            key: "id",
            columns: 4,
            gap: 14,
            size: 8,
            buffer: 3,
            itemHeight: account_content_card_height,
            render(content) {
              return AccountContentCard({ store: vm$, content });
            },
            footer: [
              Show({
                when: computed(
                  vm$.state.drawer_loading_more,
                  (loading) => Boolean(loading),
                ),
                ok() {
                  return View(
                    {
                      class: "account-content-waterfall-footer",
                      attributes: {
                        n: "account-content-load-more-status",
                        role: "status",
                      },
                    },
                    [
                      BrandLoading({ size: 30, decorative: true }),
                      "正在加载更多…",
                    ],
                  );
                },
              }),
              Show({
                when: computed(
                  vm$.state.drawer_more_error,
                  (error) => Boolean(error),
                ),
                ok() {
                  return View(
                    {
                      class: "account-content-waterfall-footer is-error",
                      attributes: {
                        n: "account-content-load-more-error",
                        role: "alert",
                      },
                    },
                    [vm$.state.drawer_more_error],
                  );
                },
              }),
              Show({
                when: Timeless.combine(
                  {
                    marker: vm$.state.drawer_next_marker,
                    loading: vm$.state.drawer_loading_more,
                    error: vm$.state.drawer_more_error,
                  },
                  ({ marker, loading, error }) =>
                    !marker && !loading && !error,
                ),
                ok() {
                  return View(
                    {
                      class: "account-content-waterfall-footer is-end",
                      attributes: {
                        n: "account-content-no-more",
                        role: "status",
                      },
                    },
                    ["没有更多了"],
                  );
                },
              }),
            ],
            onReachBottom() {
              vm$.methods.loadMoreAccountContents();
            },
          });
        },
      }),
    ],
  );
}

function AccountContentsDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.account_contents_drawer$,
      class: "dm-drawer--wide account-content-drawer",
      attributes: { n: "account-content-drawer" },
    },
    () => [
      View(
        {
          class: "dm-drawer-body account-content-drawer-body",
          attributes: { n: "account-content-drawer-body" },
        },
        [
          Tabs(
            {
              class: "account-content-tabs",
              attributes: { n: "account-content-tabs" },
              each: vm$.state.drawer_tabs,
              key: "scope",
              render(tab) {
                return AccountContentTab({ store: vm$, tab });
              },
            },
          ),
          AccountContentsCollection({ store: vm$ }),
        ],
      ),
    ],
  );
}

function AccountContentTab(props) {
  const vm$ = props.store;
  const tab = props.tab;
  const scope = String(tab.scope || "");
  const content_types = Array.isArray(tab.content_types)
    ? tab.content_types.join("、")
    : "";
  return Tab(
    {
      selected: computed(
        vm$.state.drawer_scope,
        (active_scope) => active_scope === scope,
      ),
      attributes: {
        n: `account-content-tab-${scope}`,
        title: content_types ? `内容类型：${content_types}` : tab.name,
      },
      onClick() {
        return vm$.methods.selectHomeTab(tab);
      },
    },
    [tab.name || scope],
  );
}

function AccountPageActionButton(props) {
  const semantic_name = props.name || "account-action";
  return Button(
    {
      store: props.store,
      class: "dm-button--toolbar",
      attributes: {
        n: semantic_name,
        type: props.type || "button",
        title: props.title || "",
      },
      onClick: props.onClick,
      prefix: Timeless.Icon({
        name: props.icon,
        size: 16,
        attributes: { n: `${semantic_name}-icon` },
      }),
    },
    [props.label],
  );
}

function AccountPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "content-toolbar account-toolbar",
      attributes: { n: "account-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class:
            "content-filter-fields account-filter-fields dm-flex dm-items-center dm-gap-2",
        },
        [
          View({}, [
            PlatformSelect({
              store: vm$.ui.select_platform$,
              attributes: {
                "aria-label": "按平台筛选账号",
              },
            }),
          ]),
          View(
            {
              class: "account-search",
              attributes: { n: "account-search-field" },
            },
            [
              Input({
                store: vm$.ui.input_keyword$,
                rootAttributes: { n: "account-search-control" },
                prefix: Timeless.Icon({
                  name: "search",
                  size: 16,
                  attributes: { n: "account-search-icon" },
                }),
                attributes: {
                  n: "account-search-input",
                  name: "keyword",
                  type: "search",
                  autocomplete: "off",
                  "aria-label": "搜索账号昵称或 ID",
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
          attributes: { n: "account-toolbar-actions" },
        },
        [
          AccountPageActionButton({
            name: "account-search-action",
            store: vm$.ui.btn_search$,
            icon: "search",
            label: "搜索",
            variant: "primary",
            type: "submit",
            onClick(event) {
              event.preventDefault();
              vm$.methods.search();
            },
          }),
          AccountPageActionButton({
            name: "account-reset-action",
            store: vm$.ui.btn_refresh$,
            icon: "rotate-ccw",
            label: "重置",
          }),
        ],
      ),
    ],
  );
}

function AccountAvatar(props) {
  const account = props.account;
  return View({
    class: "account-avatar-wrap",
    attributes: { n: "account-avatar" },
  }, [
    Show({
      when: account.avatar_url,
      ok() {
        return LazyImg({
          class: "account-avatar",
          src: account.avatar_url,
          alt: account.nickname,
          attributes: {
            n: "account-avatar-image",
            loading: "lazy",
            referrerpolicy: "no-referrer",
          },
        });
      },
    }),
  ]);
}

function AccountPlatform(props) {
  const vm$ = props.store;
  const account = props.account;
  return PlatformTag({
    name: "account-platform",
    favicon: window.PLATFORM_FAVICONS[account.platform_id] || "",
    label: vm$.methods.platformName(account),
  });
}

function AccountIdentity(props) {
  const vm$ = props.store;
  const account = props.account;
  const copied_ = computed(
    vm$.state.copied_account_id,
    (copied_account_id) => copied_account_id === account.id,
  );
  return [
    AccountAvatar({ account }),
    View({
      class: "account-details",
      attributes: { n: "account-details" },
    }, [
      View({
        class: "account-name",
        attributes: { n: "account-name" },
      }, [account.nickname]),
      View({ class: "account-meta", attributes: { n: "account-meta" } }, [
        View(
          {
            type: "button",
            class: computed(copied_, (copied) =>
              copied
                ? "account-copy-id-action dm-focus-ring is-copied"
                : "account-copy-id-action dm-focus-ring",
            ),
            attributes: {
              n: "account-copy-id-action",
              type: "button",
              title: computed(copied_, (copied) =>
                copied ? "已复制" : "复制账号 ID",
              ),
              "aria-label": computed(copied_, (copied) =>
                copied ? "账号 ID 已复制" : "复制账号 ID",
              ),
              disabled: account.id ? undefined : true,
            },
            onClick(event) {
              event.stopPropagation();
              vm$.methods.copyId(account);
            },
          },
          [
            Show({
              when: copied_,
              ok() {
                return Timeless.Icon({
                  name: "check",
                  size: 12,
                  attributes: { n: "account-copy-id-success-icon" },
                });
              },
              else() {
                return Timeless.Icon({
                  name: "copy",
                  size: 12,
                  attributes: { n: "account-copy-id-icon" },
                });
              },
            }),
          ],
        ),
        View(
          {
            class: "account-id",
            attributes: {
              n: "account-id",
              title: account.id || "",
            },
          },
          [account.id || "-"],
        ),
      ]),
      AccountPlatform({ store: vm$, account }),
    ]),
  ];
}

function AccountSkeletonRow() {
  return View(
    {
      class:
        "dm-table-row dm-grid dm-items-center account-row content-skeleton-row",
      attributes: { n: "account-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "dm-table-cell account-identity",
          attributes: { n: "account-table-skeleton-account", role: "cell" },
        },
        [
          View({
            class: "account-avatar-wrap content-skeleton",
            attributes: { n: "account-table-skeleton-avatar" },
          }),
          View({
            class: "account-skeleton-details",
            attributes: { n: "account-table-skeleton-details" },
          }, [
            View({
              class: "content-skeleton content-skeleton-line",
              attributes: { n: "account-table-skeleton-name" },
            }),
            View({
              class: "content-skeleton content-skeleton-line-short",
              attributes: { n: "account-table-skeleton-id" },
            }),
          ]),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "account-table-skeleton-count", role: "cell" },
        },
        [View({
          class: "content-skeleton content-skeleton-line-short",
          attributes: { n: "account-table-skeleton-count-value" },
        })],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "account-table-skeleton-time", role: "cell" },
        },
        [View({
          class: "content-skeleton content-skeleton-line-short",
          attributes: { n: "account-table-skeleton-time-value" },
        })],
      ),
    ],
  );
}

function AccountPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "account-table",
    containerClass: "content-main container",
    containerAttributes: { n: "account-page-main" },
    panelAttributes: { n: "account-table-panel" },
    headerClass: "account-row",
    columns: [
      {
        name: "account",
        title: "账号",
        cellClass: "account-identity",
        cellAttributes(account) {
          return {
            title: account.nickname || account.external_id || "",
          };
        },
        render(account) {
          return AccountIdentity({ store: vm$, account });
        },
      },
      {
        name: "content-count",
        title: "关联内容",
        width: 110,
        cellClass: "account-content-count",
        render(account) {
          return [
            vm$.methods.formatContentCount(account.content_count),
          ];
        },
      },
      {
        name: "added",
        title: "添加时间",
        width: 180,
        cellClass: "account-added",
        render(account) {
          return [
            vm$.methods.formatTime(account.created_at),
          ];
        },
      },
    ],
    rows: vm$.state.accounts,
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
    rowKey: "id",
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "account-row",
    skeletonCount: 8,
    renderSkeletonRow: AccountSkeletonRow,
    onRow(account) {
      return {
        class: "account-row-clickable",
        attributes: {
          n: "account-record",
          role: "button",
          tabindex: 0,
          title: "查看账号关联内容",
        },
        onClick() {
          vm$.methods.openAccount(account);
        },
        onKeydown(event) {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            vm$.methods.openAccount(account);
          }
        },
      };
    },
    errorTitle: "账号加载失败",
    retry: {
      store: vm$.ui.btn_retry$,
    },
    emptyTitle: Timeless.combine(
      {
        keyword: vm$.state.keyword,
        platform: vm$.state.platform_id,
      },
      ({ keyword, platform }) =>
        String(keyword || "").trim() || String(platform || "").trim()
          ? "没有匹配的账号"
          : "暂无账号",
    ),
    emptyDescription: Timeless.combine(
      {
        keyword: vm$.state.keyword,
        platform: vm$.state.platform_id,
      },
      ({ keyword, platform }) =>
        String(keyword || "").trim() || String(platform || "").trim()
          ? "请尝试其他平台、昵称或账号 ID"
          : "还没有记录任何账号",
    ),
  });
}

export default AccountPageView;

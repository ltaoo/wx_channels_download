import { Tag, PlatformTag, PlatformIcon, IconButton } from "../dmui.js";
import { ContentViewModel } from "./content.model.js";
import ContentDetailPageView from "./content_detail.js";
import { TagSelect, ContentTagBadge } from "../components.js";

const Runtime = window.Timeless;

function ContentDetailDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.contentDetailDrawer$,
      class: "dm-drawer--wide",
      attributes: { n: "content-detail-drawer" },
    },
    () => [
      ContentDetailPageView({
        app: props.app,
        client: props.client,
        hlsPlayer: props.hlsPlayer,
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
      SplitView({
        class: "content-page-split",
        attributes: { n: "content-page-split" },
        panels: [
          {
            size: 280,
            minSize: 220,
            content() {
              return ContentSavedFilterPanel({ store: vm$ });
            },
          },
          {
            size: "auto",
            content() {
              return View(
                {
                  class: "content-page-results",
                  attributes: { n: "content-page-results" },
                },
                [
                  View(
                    {
                      class: "content-toolbar-wrap",
                      attributes: { n: "content-toolbar-container" },
                    },
                    [ContentPageToolbar({ store: vm$ })],
                  ),
                  ContentPageBody({ store: vm$, client: props.client }),
                ],
              );
            },
          },
        ],
      }),
      ContentDetailDrawer({
        store: vm$,
        app: props.app,
        client: props.client,
        hlsPlayer: props.hlsPlayer,
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

function ContentLayoutMenu(props) {
  const vm$ = props.store;
  return DropdownMenu(
    {
      store: vm$.ui.dropdown_layout$,
      attributes: { n: "content-layout-menu" },
    },
    [
      IconButton(
        {
          store: vm$.ui.btn_layout$,
          class: "content-layout-icon-button",
          attributes: {
            n: "content-layout-action",
            type: "button",
            title: "切换内容布局",
            "aria-label": "切换内容布局",
          },
          },
          [
            Show({
              when: computed(vm$.state.layout, (layout) => layout === "table"),
              ok() {
                return Timeless.Icon({
                  name: "table",
                  size: 16,
                  attributes: { n: "content-layout-action-table-icon" },
                });
              },
              else() {
                return Timeless.Icon({
                  name: "grid-3x3",
                  size: 16,
                  attributes: { n: "content-layout-action-card-icon" },
                });
              },
            }),
          ],
      ),
    ],
  );
}

function ContentSavedFilterPanel(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "content-saved-filter-panel",
      attributes: {
        n: "content-saved-filter-panel",
        "aria-label": "快捷筛选器",
      },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.saveFilter();
      },
    },
    [
      View(
        {
          class: "content-saved-filter-header",
          attributes: { n: "content-saved-filter-header" },
        },
        [
          View(
            {
              as: "h2",
              class: "content-saved-filter-title",
              attributes: { n: "content-saved-filter-title" },
            },
            ["快捷筛选器"],
          ),
          View(
            {
              class: "content-saved-filter-description",
              attributes: { n: "content-saved-filter-description" },
            },
            ["保存当前多个筛选条件，一键复用"],
          ),
        ],
      ),
      View(
        {
          class: "content-saved-filter-fields",
          attributes: {
            n: "content-saved-filter-fields",
            role: "group",
            "aria-label": "当前筛选条件",
          },
        },
        [
          View(
            {
              class: "content-saved-filter-field",
              attributes: { n: "content-platform-filter-field" },
            },
            [
              PlatformSelect({
                store: vm$.ui.select_platform$,
                attributes: {
                  "aria-label": "按平台筛选内容",
                },
              }),
            ],
          ),
          View(
            {
              class: "content-saved-filter-field",
              attributes: { n: "content-type-filter-field" },
            },
            [
              Select({
                store: vm$.ui.select_content_type$,
                attributes: {
                  n: "content-type-filter-select",
                  "aria-label": "按类型筛选内容",
                },
              }),
            ],
          ),
          View(
            {
              class: "content-saved-filter-field",
              attributes: { n: "content-account-filter-field" },
            },
            [
              AccountSelect({
                store: vm$.ui.select_account$,
                platform: vm$.state.platform_id,
                attributes: {
                  "aria-label": "按账号筛选内容",
                },
              }),
            ],
          ),
        ],
      ),
      Input({
        store: vm$.ui.input_filter_name$,
        rootAttributes: { n: "content-filter-name-control" },
        attributes: {
          n: "content-filter-name-input",
          name: "filter_name",
          placeholder: "筛选器名称",
          maxlength: "60",
          required: true,
          autocomplete: "off",
          "aria-label": "筛选器名称",
        },
      }),
      Button(
        {
          store: vm$.ui.btn_save_filter$,
          class: "content-saved-filter-save",
          attributes: {
            n: "content-save-filter-action",
            type: "submit",
          },
        },
        ["保存当前筛选"],
      ),
      View(
        {
          class: "content-saved-filter-list",
          attributes: {
            n: "content-saved-filter-list",
            role: "list",
            "aria-label": "已保存筛选器",
          },
        },
        [
          Show({
            when: computed(
              vm$.state.saved_filters,
              (filters) => filters.length === 0,
            ),
            ok() {
              return View(
                {
                  class: "content-saved-filter-empty",
                  attributes: { n: "content-saved-filter-empty" },
                },
                ["暂存筛选器后会显示在这里"],
              );
            },
          }),
          For({
            each: vm$.state.saved_filters,
            render(filter) {
              const active_ = computed(
                vm$.state.active_filter_id,
                (active_id) => active_id === filter.id,
              );
              return View(
                {
                  class: computed(active_, (active) =>
                    active
                      ? "content-saved-filter is-active"
                      : "content-saved-filter",
                  ),
                  attributes: {
                    n: "content-saved-filter",
                    role: "listitem",
                  },
                },
                [
                  View(
                    {
                      as: "button",
                      type: "button",
                      class: "content-saved-filter-apply dm-focus-ring",
                      attributes: {
                        n: "content-saved-filter-apply",
                        type: "button",
                        title: vm$.methods.filterSummary(filter),
                      },
                      onClick() {
                        void vm$.methods.applyFilter(filter);
                      },
                    },
                    [
                      View(
                        {
                          class: "content-saved-filter-name",
                          attributes: { n: "content-saved-filter-name" },
                        },
                        [filter.name],
                      ),
                      View(
                        {
                          class: "content-saved-filter-summary",
                          attributes: {
                            n: "content-saved-filter-summary",
                            title: vm$.methods.filterSummary(filter),
                          },
                        },
                        [vm$.methods.filterSummary(filter)],
                      ),
                    ],
                  ),
                  View(
                    {
                      as: "button",
                      type: "button",
                      class: "content-saved-filter-delete dm-focus-ring",
                      attributes: {
                        n: "content-saved-filter-delete",
                        type: "button",
                        title: "删除筛选器",
                        "aria-label": `删除筛选器 ${filter.name}`,
                      },
                      onClick() {
                        vm$.methods.deleteFilter(filter.id);
                      },
                    },
                    [
                      Timeless.Icon({
                        name: "x",
                        size: 14,
                        attributes: {
                          n: "content-saved-filter-delete-icon",
                        },
                      }),
                    ],
                  ),
                ],
              );
            },
          }),
        ],
      ),
    ],
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
          class: "content-filter-fields dm-flex dm-items-center dm-gap-2",
          attributes: { n: "content-filter-fields" },
        },
        [
          View(
            {
              // class: "content-filter-search",
              style: { flex: "1" },
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
        ],
      ),
      View(
        {
          class: "content-filter-actions dm-flex dm-items-center dm-gap-2",
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
          ContentLayoutMenu({ store: vm$ }),
        ],
      ),
    ],
  );
}

function content_cover_url(content) {
  return String((content && content.cover_url) || "").trim();
}

function content_badges_title(vm$, content) {
  return [
    vm$.methods.platformName(content),
    vm$.methods.typeLabel(content.content_type),
    content.content_subtype,
    ...(content.tags || []).map((tag) => tag && tag.name),
  ]
    .filter(Boolean)
    .join("、");
}

function ContentRowCover(props) {
  const content = props.content;
  const cover_url = content_cover_url(content);
  if (!cover_url) return null;
  return View({ class: "content-row-cover-wrap" }, [
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
          account_ && account_.value !== undefined ? account_.value : account_;
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
                return LazyImg({
                  class: "content-row-author-avatar",
                  src: account.avatar_url,
                  alt: name,
                  attributes: {
                    loading: "lazy",
                    referrerpolicy: "no-referrer",
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
    ...statistics.task_statuses,
    { key: "files", label: "文件", value: statistics.files },
  ].filter((item) => item.value > 0);
  return [
    For({
      each: items,
      render(item) {
        return Tag(
          {
            name: "content-row-stat",
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
  // Per-row reactive tag list, keyed by the content object identity so it
  // survives re-renders and stays in sync with the TagSelect popover.
  if (!content.__tag_ref__) {
    content.__tag_ref__ = ref((content.tags || []).slice());
  }
  const tags_ref = content.__tag_ref__;

  function remove_tag(tag) {
    const next = (tags_ref.value || []).filter((t) => t.id !== tag.id);
    tags_ref.as(next);
    window.request
      .post("/api/tag/content/set", {
        content_id: content.id,
        tag_ids: next.map((t) => t.id),
      })
      .catch(() => {});
  }
  const favicon = window.PLATFORM_FAVICONS[content.platform_id] || "";
  const title = content.title || "\u00a0";
  const copied_ = computed(
    vm$.state.copied_content_id,
    (copied_content_id) => copied_content_id === content.id,
  );
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
          class: "content-row-id",
          attributes: { n: "content-id" },
        },
        [
          View(
            {
              type: "button",
              class: computed(copied_, (copied) =>
                copied
                  ? "content-copy-id-action dm-focus-ring is-copied"
                  : "content-copy-id-action dm-focus-ring",
              ),
              attributes: {
                n: "content-copy-id-action",
                type: "button",
                title: computed(copied_, (copied) =>
                  copied ? "已复制" : "复制内容 ID",
                ),
                "aria-label": computed(copied_, (copied) =>
                  copied ? "内容 ID 已复制" : "复制内容 ID",
                ),
                disabled: content.id ? undefined : true,
              },
              onClick(event) {
                event.stopPropagation();
                vm$.methods.copyId(content);
              },
            },
            [
              Show({
                when: copied_,
                ok() {
                  return Timeless.Icon({
                    name: "check",
                    size: 12,
                    attributes: { n: "content-copy-id-success-icon" },
                  });
                },
                else() {
                  return Timeless.Icon({
                    name: "copy",
                    size: 12,
                    attributes: { n: "content-copy-id-icon" },
                  });
                },
              }),
            ],
          ),
          View(
            {
              class: "content-row-id-value",
              attributes: {
                n: "content-id-value",
                title: content.id || "",
              },
            },
            [content.id || "-"],
          ),
        ],
      ),
      View(
        {
          class: "content-row-badges dm-flex dm-items-center dm-gap-1-5",
        },
        [
          PlatformTag({
            name: "content-platform",
            favicon,
            label: vm$.methods.platformName(content),
          }),
          Tag({ name: "content-row-type", class: "content-row-type" }, [
            vm$.methods.typeLabel(content.content_type),
          ]),
          Show({
            when: content.content_subtype,
            ok() {
              return Tag(
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
          TagSelect({
            contentId: content.id,
            tagsRef: tags_ref,
            client: props.client,
          }),
          Runtime.For({
            each: computed(tags_ref, (list) => list),
            render(tag) {
              return ContentTagBadge({ tag, onRemove: remove_tag });
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

function ContentCardSkeleton() {
  return View(
    {
      class: "content-card content-card-skeleton",
      attributes: { n: "content-card-skeleton", "aria-hidden": "true" },
    },
    [
      View({
        class: "content-card-media content-skeleton",
        attributes: { n: "content-card-skeleton-media" },
      }),
      View(
        {
          class: "content-card-body",
          attributes: { n: "content-card-skeleton-body" },
        },
        [
          View({
            class: "content-skeleton content-skeleton-title",
            attributes: { n: "content-card-skeleton-title" },
          }),
          View({
            class: "content-skeleton content-skeleton-tag",
            attributes: { n: "content-card-skeleton-tag" },
          }),
          View({
            class: "content-skeleton content-skeleton-line-short",
            attributes: { n: "content-card-skeleton-meta" },
          }),
        ],
      ),
    ],
  );
}

function ContentCardAccounts(props) {
  const accounts = props.content.accounts || [];
  return Show({
    when: accounts.length === 0,
    ok() {
      return View(
        {
          class: "content-card-account",
          attributes: { n: "content-card-empty-account" },
        },
        ["暂无关联账号"],
      );
    },
    else() {
      return For({
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
              class: "content-card-account",
              attributes: { n: "content-card-account", title: name },
            },
            [
              Show({
                when: account.avatar_url,
                ok() {
                  return LazyImg({
                    class: "content-row-author-avatar",
                    src: account.avatar_url,
                    alt: name,
                    attributes: {
                      n: "content-card-account-avatar",
                      loading: "lazy",
                      referrerpolicy: "no-referrer",
                    },
                  });
                },
              }),
              View(
                {
                  class: "content-card-account-name",
                  attributes: { n: "content-card-account-name" },
                },
                [name],
              ),
            ],
          );
        },
      });
    },
  });
}

function ContentCard(props) {
  const vm$ = props.store;
  const content = props.content;
  const cover_url = content_cover_url(content);
  const favicon = window.PLATFORM_FAVICONS[content.platform_id] || "";
  return View(
    {
      as: "button",
      type: "button",
      class: "content-card",
      attributes: {
        n: "content-card",
        type: "button",
        title: "查看内容详情",
      },
      onClick() {
        vm$.methods.openDetail(content);
      },
    },
    [
      View(
        {
          class: "content-card-media",
          attributes: { n: "content-card-media" },
        },
        [
          Show({
            when: cover_url,
            ok() {
              return LazyImg({
                class: "content-card-image",
                src: cover_url,
                alt: content.title || "内容封面",
                attributes: {
                  n: "content-card-image",
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
              });
            },
            else() {
              return View(
                {
                  class: "content-card-media-placeholder",
                  attributes: {
                    n: "content-card-media-placeholder",
                    "aria-hidden": "true",
                  },
                },
                [
                  PlatformIcon({
                    class: "content-card-type-icon",
                    favicon: vm$.methods.typeIcon(
                      content.content_type,
                      content.content_subtype,
                    ),
                    name: "content-card-media-placeholder-icon",
                    attributes: { viewBox: "0 0 132 96" },
                  }),
                ],
              );
            },
          }),
          View(
            {
              class: "content-card-media-meta",
              attributes: { n: "content-card-media-meta" },
            },
            [
              View(
                {
                  class: "content-card-media-tags",
                  attributes: { n: "content-card-media-tags" },
                },
                [
                  PlatformTag({
                    name: "content-card-platform",
                    favicon,
                    label: vm$.methods.platformName(content),
                  }),
                  Tag(
                    {
                      name: "content-card-type",
                      class: "content-row-type",
                    },
                    [vm$.methods.typeLabel(content.content_type)],
                  ),
                ],
              ),
              View(
                {
                  class: "content-card-time",
                  attributes: { n: "content-card-publish-time" },
                },
                [vm$.methods.formatTime(content.publish_time)],
              ),
            ],
          ),
        ],
      ),
      View(
        {
          class: "content-card-body",
          attributes: { n: "content-card-body" },
        },
        [
          View(
            {
              as: "h3",
              class: "content-card-title",
              attributes: {
                n: "content-card-title",
                title: content.title,
              },
            },
            [content.title || "未命名内容"],
          ),
          View(
            {
              class: "content-card-badges",
              attributes: {
                n: "content-card-badges",
                title: content_badges_title(vm$, content),
              },
            },
            [
              For({
                each: content.tags || [],
                render(tag) {
                  return ContentTagBadge({ tag });
                },
              }),
            ],
          ),
          View(
            {
              class: "content-card-accounts",
              attributes: { n: "content-card-accounts" },
            },
            [ContentCardAccounts({ content })],
          ),
          View(
            {
              class: "content-card-footer",
              attributes: { n: "content-card-footer" },
            },
            [
              View(
                {
                  class: "content-card-statistics",
                  attributes: { n: "content-card-statistics" },
                },
                ContentRowStatistics({
                  statistics: vm$.methods.statistics(content),
                }),
              ),
              View(
                {
                  class: "content-card-created-time",
                  attributes: { n: "content-card-created-time" },
                },
                [vm$.methods.formatTime(content.created_at)],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function ContentCardState(props) {
  return View(
    {
      class: "content-card-state",
      attributes: {
        n: "content-card-state",
        role: props.error ? "alert" : "status",
      },
    },
    [
      View(
        {
          as: "strong",
          class: "content-card-state-title",
          attributes: { n: "content-card-state-title" },
        },
        [props.title],
      ),
      View(
        {
          class: "content-card-state-description",
          attributes: { n: "content-card-state-description" },
        },
        [props.description],
      ),
      Show({
        when: props.retry,
        ok() {
          return Button(
            {
              store: props.retry,
              variant: "primary",
              attributes: {
                n: "content-card-retry-action",
                type: "button",
              },
            },
            ["重试"],
          );
        },
      }),
    ],
  );
}

function ContentCardBody(props) {
  const vm$ = props.store;
  return View(
    {
      class: "content-main container",
      attributes: { n: "content-card-page-main" },
    },
    [
      Match({
        when: vm$.state.status,
        cases: {
          initial() {
            return View(
              {
                class: "content-card-grid",
                attributes: {
                  n: "content-card-skeleton-grid",
                  "aria-busy": "true",
                },
              },
              Array.from({ length: 12 }, () => ContentCardSkeleton()),
            );
          },
          empty() {
            return ContentCardState({
              title: "暂无内容",
              description: "当前筛选条件下没有内容",
            });
          },
          error() {
            return ContentCardState({
              error: true,
              title: "内容加载失败",
              description: vm$.state.error,
              retry: vm$.ui.btn_retry$,
            });
          },
          normal() {
            return View(
              {
                class: "content-card-grid",
                attributes: {
                  n: "content-card-grid",
                  role: "list",
                  "aria-label": "内容卡片列表",
                  "aria-busy": computed(vm$.state.loading, Boolean),
                },
              },
              [
                For({
                  each: vm$.state.contents,
                  render(content) {
                    return View(
                      {
                        class: "content-card-item",
                        attributes: {
                          n: "content-card-item",
                          role: "listitem",
                        },
                      },
                      [
                        ContentCard({
                          store: vm$,
                          content,
                        }),
                      ],
                    );
                  },
                }),
                Show({
                  when: vm$.state.loading,
                  ok() {
                    return View(
                      {
                        class: "content-card-loading-overlay",
                        attributes: {
                          n: "content-card-loading-overlay",
                          role: "status",
                          "aria-label": "列表加载中",
                        },
                      },
                      ["加载中…"],
                    );
                  },
                }),
              ],
            );
          },
        },
      }),
      Pagination({
        class: "container dm-px-4",
        summary: vm$.state.range_text,
        page: vm$.state.page,
        pageCount: vm$.state.page_count,
        pageSize: vm$.state.page_size,
        loading: vm$.state.loading,
        onChange(page) {
          return vm$.methods.changePage(page);
        },
        attributes: { n: "content-card-pagination" },
      }),
    ],
  );
}

function ContentPageBody(props) {
  const vm$ = props.store;
  return Match({
    when: vm$.state.layout,
    cases: {
      table() {
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
                return ContentRowMain({
                  store: vm$,
                  client: props.client,
                  content,
                });
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
                  View({ attributes: { n: "content-time" } }, [
                    View({ attributes: { n: "content-publish-time" } }, [
                      `发布时间: ${vm$.methods.formatTime(content.publish_time)}`,
                    ]),
                    View({ attributes: { n: "content-created-at" } }, [
                      `创建时间: ${vm$.methods.formatTime(content.created_at)}`,
                    ]),
                  ]),
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
            loading: vm$.state.initial,
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
      },
      card() {
        return ContentCardBody({ store: vm$, client: props.client });
      },
    },
  });
}

export default ContentPageView;

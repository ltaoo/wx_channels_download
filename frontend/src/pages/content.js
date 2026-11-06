import { TagSelect, ContentTagBadge } from "@/components/index.js";
import { EmptyCard } from "@/dmui.js";

import { ContentViewModel } from "./content.model.js";
import ContentDetailPageView from "./content_detail.js";

const content_row_stat_classes = {
  __proto__: null,
  "in-progress": "bg-color-warning-soft text-color-warning",
  "running": "bg-color-warning-soft text-color-warning",
  "paused": "bg-color-warning-soft text-color-warning",
  "success": "bg-color-success-soft text-color-success",
  "finished": "bg-color-success-soft text-color-success",
  "failed": "bg-color-danger-soft text-color-danger",
  "files": "bg-color-primary-soft text-color-primary",
};

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
        tip: vm$.methods.tip,
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
      class: "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page data-table-page",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      SplitView({
        class: "min-w-0 min-h-0 flex-auto",
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
                  class: "flex min-h-0 min-w-0 h-full flex-col paginated-results ",
                  attributes: { n: "content-page-results" },
                },
                [
                  View(
                    {
                      class: "pt-4 pb-0 px-4 group-content-toolbar-wrap",
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
          class: "w-28px h-28px min-h-28px p-0 border-0 rounded-sm bg-transparent view-mode-action ",
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

function ContentSaveFilterPopover(props) {
  const vm$ = props.store;
  return Popover(
    {
      store: vm$.ui.save_filter_popover$,
      class: "w-260px p-2",
      attributes: {
        n: "content-save-filter-popover",
        role: "dialog",
        "aria-label": "保存快捷筛选器",
      },
      onTriggerMouseEnter: vm$.methods.showSaveFilterPopover,
      onTriggerMouseLeave: vm$.methods.scheduleSaveFilterPopoverHide,
      onContentMouseEnter: vm$.methods.showSaveFilterPopover,
      onContentMouseLeave: vm$.methods.scheduleSaveFilterPopoverHide,
      onUnmounted: vm$.methods.hideSaveFilterPopover,
      content: [
        View(
          {
            class: "flex items-center gap-2",
            attributes: { n: "content-save-filter-controls" },
          },
          [
            Input({
              store: vm$.ui.input_filter_name$,
              rootClass: "min-w-0 flex-auto",
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
                class: "flex-none",
                attributes: {
                  n: "content-save-filter-action",
                  type: "submit",
                },
                onClick(event) {
                  event.preventDefault();
                  vm$.methods.saveFilter();
                },
              },
              ["保存"],
            ),
          ],
        ),
      ],
    },
    [
      IconButton(
        {
          store: vm$.ui.btn_add_filter$,
          attributes: {
            n: "content-add-filter-action",
            type: "button",
            title: "新增快捷筛选器",
            "aria-label": "新增快捷筛选器",
            "aria-haspopup": "dialog",
          },
          onClick: vm$.methods.showSaveFilterPopover,
          onFocus: vm$.methods.showSaveFilterPopover,
        },
        [
          Timeless.Icon({
            name: "plus",
            size: 16,
            attributes: { n: "content-add-filter-icon" },
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
      class: "flex w-full h-full min-h-0 flex-col p-3 overflow-hidden border-r-default border-r-solid border-r-color-content bg-content-panel",
      attributes: {
        n: "content-saved-filter-panel",
        "aria-label": "内容筛选",
      },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.saveFilter();
      },
    },
    [
      View(
        {
          class: "grid pb-3 gap-2 border-b-default border-b-solid border-b-color-content",
          attributes: {
            n: "content-saved-filter-fields",
            role: "group",
            "aria-label": "当前筛选条件",
          },
        },
        [
          View(
            {
              class: "min-w-0 select-field-stretch",
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
              class: "min-w-0 select-field-stretch",
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
              class: "min-w-0 select-field-stretch",
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
      View(
        {
          class: "mt-3 mb-3",
          attributes: { n: "content-saved-filter-header" },
        },
        [
          View(
            {
              class: "flex items-center justify-between gap-2",
              attributes: { n: "content-saved-filter-heading" },
            },
            [
              View(
                {
                  as: "h2",
                  class: "m-0 text-base font-semibold text-content-text",
                  attributes: { n: "content-saved-filter-title" },
                },
                ["快捷筛选器"],
              ),
              ContentSaveFilterPopover({ store: vm$ }),
            ],
          ),
          View(
            {
              class: "mt-1 text-caption leading-tight text-content-muted",
              attributes: { n: "content-saved-filter-description" },
            },
            ["保存当前多个筛选条件，一键复用"],
          ),
        ],
      ),
      View(
        {
          class: "flex min-h-0 flex-auto flex-wrap items-start content-start mt-3 gap-2 overflow-y-auto",
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
              return EmptyCard({
                name: "content-saved-filter",
                class: "w-full min-w-0",
                title: "暂无快捷筛选器",
                description: "暂存筛选器后会显示在这里",
              });
            },
          }),
          For({
            each: vm$.state.saved_filters,
            render(filter) {
              const active_ = computed(
                vm$.state.active_filter_id,
                (active_id) => active_id === filter.id,
              );
              return Tag(
                {
                  class: computed(active_, (active) =>
                    active
                      ? "min-w-0 bg-color-primary-soft text-color-primary"
                      : "min-w-0",
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
                      class: "min-w-0 p-0 font-inherit text-left border-0 rounded-inherit bg-transparent text-inherit hover:text-color-primary cursor-pointer dm-focus-ring",
                      attributes: {
                        n: "content-saved-filter-apply",
                        type: "button",
                        title: filter.name,
                        "aria-pressed": computed(active_, (active) => String(active)),
                      },
                      onClick() {
                        void vm$.methods.applyFilter(filter);
                      },
                    },
                    [
                      View(
                        {
                          class: "overflow-hidden text-sm whitespace-nowrap text-ellipsis",
                          attributes: { n: "content-saved-filter-name" },
                        },
                        [filter.name],
                      ),
                    ],
                  ),
                  View(
                    {
                      as: "button",
                      type: "button",
                      class: "inline-grid flex-none place-items-center p-0 border-0 rounded-inherit bg-transparent hover:bg-color-danger-soft focus-visible:bg-color-danger-soft text-inherit hover:text-color-danger focus-visible:text-color-danger cursor-pointer dm-focus-ring",
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
          class: "max-520:w-full flex-none dm-items-center max-760:ml-auto max-520:ml-0 dm-gap-2 dm-flex group-content-filter-actions toolbar-actions",
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
              class: "flex items-center gap-2 whitespace-nowrap",
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
  return View({ class: "relative w-control-height-lg h-control-height-lg cover-anchor " }, [
    LazyImg({
      class: "grid w-control-height-lg h-control-height-lg place-items-center overflow-hidden object-cover border-default border-solid border-color-content rounded-md text-content-muted cover-target",
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
              "max-w-full dm-min-w-0 dm-items-center dm-gap-1-5 dm-flex",
          },
          [
            Show({
              when: account.avatar_url,
              ok() {
                return LazyImg({
                  class: "w-4 h-4 flex-none object-cover rounded-full",
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
                class: "overflow-hidden text-sm whitespace-nowrap text-ellipsis text-content-muted",
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
            class: `flex-none border-default border-solid border-color-content ${content_row_stat_classes[item.key] || "bg-content-panel-soft"}`,
            attributes: { title: `${item.label}：${item.value}` },
          },
          [
            View({ class: "flex-none font-semibold" }, [String(item.value)]),
            View({ class: "overflow-hidden text-ellipsis" }, [item.label]),
          ],
        );
      },
    }),
  ];
}

function ContentRowMain(props) {
  const vm$ = props.store;
  const content = props.content;
  const tags_ref = vm$.methods.contentTags(content);
  const favicon = window.PLATFORM_FAVICONS[content.platform_id] || "";
  const title = content.title || "\u00a0";
  const copied_ = computed(
    vm$.state.copied_content_id,
    (copied_content_id) => copied_content_id === content.id,
  );
  return [
    ContentRowCover({ content }),
    View({ class: "dm-min-w-0 dm-flex-1 content-row-main" }, [
      View(
        {
          class: "overflow-hidden text-base font-semibold whitespace-nowrap text-ellipsis text-content-text",
          attributes: { title: content.title },
        },
        [title],
      ),
      View(
        {
          class: "flex min-w-0 max-w-full items-center mt-1 gap-2",
          attributes: { n: "content-id" },
        },
        [
          View(
            {
              type: "button",
              class: computed(copied_, (copied) =>
                copied
                  ? "grid flex-none place-items-center p-0 leading-1 border-0 bg-transparent text-content-muted cursor-pointer icon-action dm-focus-ring is-copied"
                  : "grid flex-none place-items-center p-0 leading-1 border-0 bg-transparent text-content-muted cursor-pointer icon-action dm-focus-ring",
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
              class: "min-w-0 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-content-muted",
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
          class: "min-w-0 flex-nowrap dm-items-center mt-1.5 dm-gap-1-5 overflow-hidden dm-flex",
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
              return ContentTagBadge({
                tag,
                onRemove(removed_tag) {
                  return vm$.methods.removeTag(content, removed_tag);
                },
              });
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
      class: "dm-grid dm-items-center pointer-events-none dm-table-row",
      attributes: { n: "content-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-min-w-0 dm-items-center dm-gap-4 dm-flex dm-table-cell table-primary-cell",
          attributes: { n: "content-table-skeleton-main-cell", role: "cell" },
        },
        [
          View({
            class: "grid relative w-control-height-lg h-control-height-lg place-items-center overflow-hidden object-cover border-default border-solid border-color-content rounded-sm bg-skeleton-bg text-content-muted cover-target skeleton-shimmer ",
            attributes: { n: "content-table-skeleton-cover" },
          }),
          View(
            {
              class: "dm-min-w-0 dm-flex-1 content-row-main",
              attributes: { n: "content-table-skeleton-main" },
            },
            [
              View({
                class: "relative w-4/5 max-w-90 h-4 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
                attributes: { n: "content-table-skeleton-title" },
              }),
              View({
                class: "relative h-3 w-16 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
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
            class: "relative h-3 w-4/5 max-w-45 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
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
            class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
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
            class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
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
      class: "flex relative min-w-0 h-full min-h-0 flex-col p-0 overflow-hidden isolation-isolate text-left border-default border-solid border-color-content rounded-md hover:border-color-color-primary-border focus-visible:border-color-color-primary-border bg-content-bg text-inherit hover:shadow-sm focus-visible:shadow-sm cursor-pointer hover:-translate-y-px focus-visible:-translate-y-px transition-content-card content-card-skeleton",
      attributes: { n: "content-card-skeleton", "aria-hidden": "true" },
    },
    [
      View({
        class: "absolute z-0 inset-0 overflow-hidden rounded-sm bg-content-panel-soft skeleton-shimmer ",
        attributes: { n: "content-card-skeleton-media" },
      }),
      View(
        {
          class: "flex absolute z-3 right-0 bottom-0 left-0 min-w-0 max-h-78pct flex-col p-3 gap-2 overflow-hidden bg-media-footer text-white media-caption-overlay ",
          attributes: { n: "content-card-skeleton-body" },
        },
        [
          View({
            class: "relative w-4/5 max-w-90 h-4 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "content-card-skeleton-title" },
          }),
          View({
            class: "relative h-3 w-16 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
            attributes: { n: "content-card-skeleton-tag" },
          }),
          View({
            class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer",
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
          class: "inline-flex min-w-0 items-center gap-1 text-sm text-white/82",
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
              class: "inline-flex min-w-0 items-center gap-1 text-sm text-white/82",
              attributes: { n: "content-card-account", title: name },
            },
            [
              Show({
                when: account.avatar_url,
                ok() {
                  return LazyImg({
                    class: "w-4 h-4 flex-none object-cover rounded-full",
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
                  class: "min-w-0 max-w-140px overflow-hidden whitespace-nowrap text-ellipsis",
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
      class: "flex relative min-w-0 h-full min-h-0 flex-col p-0 overflow-hidden isolation-isolate text-left border-default border-solid border-color-content rounded-md hover:border-color-color-primary-border focus-visible:border-color-color-primary-border bg-content-bg text-inherit hover:shadow-sm focus-visible:shadow-sm cursor-pointer hover:-translate-y-px focus-visible:-translate-y-px transition-content-card",
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
          class: "absolute z-0 inset-0 overflow-hidden bg-content-panel-soft",
          attributes: { n: "content-card-media" },
        },
        [
          Show({
            when: cover_url,
            ok() {
              return LazyImg({
                class: "w-full h-full object-cover object-position-center",
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
                  class: "grid w-full h-full place-items-center bg-primary-subtle text-primary-muted",
                  attributes: {
                    n: "content-card-media-placeholder",
                    "aria-hidden": "true",
                  },
                },
                [
                  PlatformIcon({
                    class: "h-auto w-fluid-illustration aspect-11/8 opacity-20",
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
              class: "flex absolute z-2 right-0 left-0 top-0 bottom-auto items-center justify-between py-1.5 px-2 gap-2 bg-media-header text-white media-caption-overlay ",
              attributes: { n: "content-card-media-meta" },
            },
            [
              View(
                {
                  class: "flex min-w-0 items-center gap-1.5 overflow-hidden",
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
                  class: "flex-none text-caption font-variant-numeric-tabular-nums whitespace-nowrap",
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
          class: "flex absolute z-3 right-0 bottom-0 left-0 min-w-0 max-h-78pct flex-col p-3 gap-2 overflow-hidden bg-media-footer text-white media-caption-overlay ",
          attributes: { n: "content-card-body" },
        },
        [
          View(
            {
              as: "h3",
              class: "-webkit-box max-h-two-lines m-0 overflow-hidden text-base font-semibold leading-tight -webkit-box-orient-vertical -webkit-line-clamp-2 text-ellipsis text-white",
              attributes: {
                n: "content-card-title",
                title: content.title,
              },
            },
            [content.title || "未命名内容"],
          ),
          View(
            {
              class: "flex min-w-0 flex-nowrap items-center gap-1.5 overflow-hidden",
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
              class: "flex min-w-0 flex-wrap items-center gap-1.5",
              attributes: { n: "content-card-accounts" },
            },
            [ContentCardAccounts({ content })],
          ),
          View(
            {
              class: "flex items-center justify-between pt-2 gap-2 border-t-default border-t-solid border-t-white/22",
              attributes: { n: "content-card-footer" },
            },
            [
              View(
                {
                  class: "flex min-w-0 flex-wrap gap-1",
                  attributes: { n: "content-card-statistics" },
                },
                ContentRowStatistics({
                  statistics: vm$.methods.statistics(content),
                }),
              ),
              View(
                {
                  class: "flex-none text-caption font-variant-numeric-tabular-nums whitespace-nowrap text-white/82",
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
      class: "grid flex-auto place-items-center p-8 text-center state-action-panel",
      attributes: {
        n: "content-card-state",
        role: props.error ? "alert" : "status",
      },
    },
    [
      View(
        {
          as: "strong",
          class: "m-0 text-lg text-content-text",
          attributes: { n: "content-card-state-title" },
        },
        [props.title],
      ),
      View(
        {
          class: "max-w-360px m-0 mt-1 text-sm text-content-muted",
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
      class: "min-w-0 flex-auto p-0 group-content-main",
      attributes: { n: "content-card-page-main" },
    },
    [
      Match({
        when: vm$.state.status,
        cases: {
          initial() {
            return View(
              {
                class: "grid relative min-h-0 flex-auto grid-auto-rows-360px grid-cols-fill-60 items-start-grid p-4 gap-3 overflow-y-auto",
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

              retry: vm$.ui.btn_retry$,
            });
          },
          normal() {
            return View(
              {
                class: "grid relative min-h-0 flex-auto grid-auto-rows-360px grid-cols-fill-60 items-start-grid p-4 gap-3 overflow-y-auto",
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
                        class: "relative min-w-0 h-full min-h-0 isolation-isolate",
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
                        class: "grid absolute inset-0 z-1 place-items-center bg-content-panel/78 text-content-text",
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
        class: " dm-px-4",
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
          containerClass: "min-w-0 flex-auto p-4 group-content-main ",
          containerAttributes: { n: "content-page-main" },
          panelAttributes: { n: "content-table-panel" },
          columns: [
            {
              name: "main",
              title: "封面 / 标题",
              width: "minmax(300px, 2fr)",
              cellClass:
                "dm-min-w-0 dm-items-center dm-gap-4 dm-flex table-primary-cell",
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
                "dm-min-w-0 dm-items-center dm-gap-1-5 overflow-hidden dm-flex table-secondary-cell",
              render(content) {
                return ContentRowAccounts({ content });
              },
            },
            {
              name: "time",
              title: "时间",
              width: 240,
              cellClass:
                "dm-items-center dm-gap-1-5 dm-text-sm dm-tabular-nums dm-whitespace-nowrap dm-text-muted dm-flex group-content-row-meta",
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
              cellClass: "flex min-w-0 flex-wrap items-center gap-1.5 text-xs font-variant-numeric-tabular-nums text-content-muted group-content-row-stats",
              render(content) {
                return ContentRowStatistics({
                  statistics: vm$.methods.statistics(content),
                });
              },
            },
          ],
          rows: vm$.state.contents,
          pagination: {
            class: " dm-px-4",
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
              class: detail_href ? "hover:relative hover:z-1 hover:bg-content-panel-soft hover:shadow-row-focus cursor-pointer" : "",
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

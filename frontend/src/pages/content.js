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
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [.group-content-page&]:h-full [.group-content-page&]:min-h-0 [.group-content-page&]:overflow-hidden [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto] bg-content-bg max-760:[&_.dm-table-row:not(.dm-table-header)]:grid-cols-[minmax(0,_1fr)_auto] max-760:[&_.group-content-row-meta]:grid-row-[1_/_3] max-760:[&_.group-content-row-meta]:grid-column-2 max-760:[&_.group-content-row-meta]:justify-self-end max-760:[&_.group-content-row-stats]:hidden min-w-0 min-h-full font-sans",
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      SplitView({
        class: "min-w-0 min-h-0 flex-[1_1_auto]",
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
                  class: "min-h-0 min-w-0 h-full flex flex-col [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex-[1_1_auto] [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col [&_>_.dm-pagination]:flex-[0_0_auto] [&_>_.group-content-toolbar-wrap]:flex-[0_0_auto]",
                  attributes: { n: "content-page-results" },
                },
                [
                  View(
                    {
                      class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0]",
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
          class: "w-28px h-28px min-h-28px p-0 border-0 rounded-sm bg-transparent [&:hover:not(:disabled)]:text-color-text-primary [&:hover:not(:disabled)]:bg-[color-mix(_in_srgb,_var(--dm-color-text-primary)_10%,_transparent_)] [&:hover:not(:disabled)]:shadow-none [&:focus-visible:not(:disabled)]:text-color-text-primary [&:focus-visible:not(:disabled)]:bg-[color-mix(_in_srgb,_var(--dm-color-text-primary)_10%,_transparent_)] [&:focus-visible:not(:disabled)]:shadow-none",
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
              rootClass: "min-w-0 flex-[1_1_auto]",
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
                class: "flex-[0_0_auto]",
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
      class: "w-full h-full min-h-0 flex flex-col p-3 overflow-hidden bg-content-panel border-right-[var(--dm-border-width)_solid_var(--dm-content-border)]",
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
          class: "grid gap-2 pb-3 border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)]",
          attributes: {
            n: "content-saved-filter-fields",
            role: "group",
            "aria-label": "当前筛选条件",
          },
        },
        [
          View(
            {
              class: "min-w-0 [&_.group-dm-platform-select-trigger]:w-full",
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
              class: "min-w-0 [&_.group-dm-platform-select-trigger]:w-full",
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
              class: "min-w-0 [&_.group-dm-platform-select-trigger]:w-full",
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
                  class: "m-0 text-content-text text-base font-semibold",
                  attributes: { n: "content-saved-filter-title" },
                },
                ["快捷筛选器"],
              ),
              ContentSaveFilterPopover({ store: vm$ }),
            ],
          ),
          View(
            {
              class: "mt-1 text-content-muted text-caption leading-tight",
              attributes: { n: "content-saved-filter-description" },
            },
            ["保存当前多个筛选条件，一键复用"],
          ),
        ],
      ),
      View(
        {
          class: "min-h-0 flex-[1_1_auto] overflow-y-auto flex flex-wrap items-start content-start gap-2 mt-3",
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
                      ? "min-w-0 text-color-primary bg-color-primary-soft"
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
                      class: "min-w-0 p-0 text-left text-inherit font-inherit bg-transparent border-0 rounded-inherit cursor-pointer hover:text-color-primary dm-focus-ring",
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
                          class: "overflow-hidden text-ellipsis whitespace-nowrap text-sm",
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
                      class: "flex-[0_0_auto] inline-grid place-items-center p-0 text-inherit bg-transparent border-0 rounded-inherit cursor-pointer hover:text-color-danger hover:bg-color-danger-soft focus-visible:text-color-danger focus-visible:bg-color-danger-soft dm-focus-ring",
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
          class: "group-content-filter-actions flex-[0_0_auto] max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2",
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
  return View({ class: "w-control-height-lg h-control-height-lg relative [&_>_.group-content-row-cover]:absolute [&_>_.group-content-row-cover]:inset-0" }, [
    LazyImg({
      class: "group-content-row-cover w-control-height-lg h-control-height-lg grid overflow-hidden place-items-center object-cover text-content-muted border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-md",
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
              "max-w-full dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
          },
          [
            Show({
              when: account.avatar_url,
              ok() {
                return LazyImg({
                  class: "w-4 h-4 flex-[0_0_auto] object-cover rounded-full",
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
                class: "overflow-hidden text-content-muted text-sm text-ellipsis whitespace-nowrap",
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
            class: `flex-[0_0_auto] border-[var(--dm-border-width)_solid_var(--dm-content-border)] ${content_row_stat_classes[item.key] || "bg-content-panel-soft"}`,
            attributes: { title: `${item.label}：${item.value}` },
          },
          [
            View({ class: "flex-[0_0_auto] font-semibold" }, [String(item.value)]),
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
    View({ class: "content-row-main dm-min-w-0 dm-flex-1" }, [
      View(
        {
          class: "overflow-hidden text-content-text text-base font-semibold text-ellipsis whitespace-nowrap",
          attributes: { title: content.title },
        },
        [title],
      ),
      View(
        {
          class: "min-w-0 max-w-full flex items-center gap-2 mt-1",
          attributes: { n: "content-id" },
        },
        [
          View(
            {
              type: "button",
              class: computed(copied_, (copied) =>
                copied
                  ? "grid flex-[0_0_auto] place-items-center p-0 text-content-muted leading-1 bg-transparent border-0 cursor-pointer [&:hover:not(:disabled)]:text-color-primary [&.is-copied]:text-color-success [&.is-copied:hover]:text-color-success dm-focus-ring is-copied"
                  : "grid flex-[0_0_auto] place-items-center p-0 text-content-muted leading-1 bg-transparent border-0 cursor-pointer [&:hover:not(:disabled)]:text-color-primary [&.is-copied]:text-color-success [&.is-copied:hover]:text-color-success dm-focus-ring",
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
              class: "min-w-0 overflow-hidden text-content-muted font-[var(--dm-font-family-mono,_monospace)] text-caption text-ellipsis whitespace-nowrap",
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
          class: "mt-1-5 min-w-0 overflow-hidden flex-nowrap dm-flex dm-items-center dm-gap-1-5",
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
      class: "dm-table-row dm-grid dm-items-center pointer-events-none",
      attributes: { n: "content-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class:
            "dm-table-cell group-content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
          attributes: { n: "content-table-skeleton-main-cell", role: "cell" },
        },
        [
          View({
            class: "group-content-row-cover w-control-height-lg h-control-height-lg grid overflow-hidden place-items-center object-cover text-content-muted border-[var(--dm-border-width)_solid_var(--dm-content-border)] relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite]",
            attributes: { n: "content-table-skeleton-cover" },
          }),
          View(
            {
              class: "content-row-main dm-min-w-0 dm-flex-1",
              attributes: { n: "content-table-skeleton-main" },
            },
            [
              View({
                class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-[min(360px,_80%)] h-4",
                attributes: { n: "content-table-skeleton-title" },
              }),
              View({
                class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-16",
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
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(180px,_80%)]",
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
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
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
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
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
      class: "isolation-isolate relative min-w-0 h-full min-h-0 flex flex-col p-0 overflow-hidden text-inherit text-left bg-content-bg border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-md cursor-pointer transition-[border-color_0.15s_ease,_box-shadow_0.15s_ease,_transform_0.15s_ease] hover:border-color-color-primary-border hover:shadow-sm hover:transform-[translateY(-1px)] focus-visible:border-color-color-primary-border focus-visible:shadow-sm focus-visible:transform-[translateY(-1px)] content-card-skeleton",
      attributes: { n: "content-card-skeleton", "aria-hidden": "true" },
    },
    [
      View({
        class: "z-0 absolute inset-0 overflow-hidden bg-content-panel-soft overflow-hidden rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite]",
        attributes: { n: "content-card-skeleton-media" },
      }),
      View(
        {
          class: "z-3 absolute right-0 bottom-0 left-0 min-w-0 max-h-[78%] flex flex-col gap-2 p-3 overflow-hidden text-white bg-[linear-gradient(_to_top,_rgb(0_0_0_/_92%),_rgb(0_0_0_/_82%)_65%,_transparent_)] [&_:where(.dm-tag)]:text-white [&_:where(.dm-tag)]:bg-[rgb(255_255_255_/_16%)] [&_:where(.dm-tag)]:border-color-[rgb(255_255_255_/_28%)]",
          attributes: { n: "content-card-skeleton-body" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-[min(360px,_80%)] h-4",
            attributes: { n: "content-card-skeleton-title" },
          }),
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-16",
            attributes: { n: "content-card-skeleton-tag" },
          }),
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
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
          class: "min-w-0 inline-flex items-center gap-1 text-[rgb(255_255_255_/_82%)] text-sm",
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
              class: "min-w-0 inline-flex items-center gap-1 text-[rgb(255_255_255_/_82%)] text-sm",
              attributes: { n: "content-card-account", title: name },
            },
            [
              Show({
                when: account.avatar_url,
                ok() {
                  return LazyImg({
                    class: "w-4 h-4 flex-[0_0_auto] object-cover rounded-full",
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
                  class: "min-w-0 overflow-hidden max-w-140px text-ellipsis whitespace-nowrap",
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
      class: "isolation-isolate relative min-w-0 h-full min-h-0 flex flex-col p-0 overflow-hidden text-inherit text-left bg-content-bg border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-md cursor-pointer transition-[border-color_0.15s_ease,_box-shadow_0.15s_ease,_transform_0.15s_ease] hover:border-color-color-primary-border hover:shadow-sm hover:transform-[translateY(-1px)] focus-visible:border-color-color-primary-border focus-visible:shadow-sm focus-visible:transform-[translateY(-1px)]",
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
          class: "z-0 absolute inset-0 overflow-hidden bg-content-panel-soft",
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
                  class: "w-full h-full grid place-items-center text-[color-mix(_in_srgb,_var(--dm-color-primary)_72%,_var(--dm-color-text-secondary)_)] bg-[color-mix(_in_srgb,_var(--dm-color-primary-soft)_58%,_var(--dm-color-bg-subtle)_)]",
                  attributes: {
                    n: "content-card-media-placeholder",
                    "aria-hidden": "true",
                  },
                },
                [
                  PlatformIcon({
                    class: "w-[clamp(140px,_68%,_220px)] h-auto aspect-ratio-[132_/_96] opacity-[0.2]",
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
              class: "z-2 absolute right-0 left-0 top-0 bottom-auto flex items-center justify-between gap-2 p-[var(--dm-space-1-5)_var(--dm-space-2)] bg-[linear-gradient(_to_bottom,_rgb(0_0_0_/_68%),_transparent_)] text-white [&_:where(.dm-tag)]:text-white [&_:where(.dm-tag)]:bg-[rgb(255_255_255_/_16%)] [&_:where(.dm-tag)]:border-color-[rgb(255_255_255_/_28%)]",
              attributes: { n: "content-card-media-meta" },
            },
            [
              View(
                {
                  class: "min-w-0 flex items-center gap-1-5 overflow-hidden",
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
                  class: "flex-[0_0_auto] text-caption font-variant-numeric-tabular-nums whitespace-nowrap",
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
          class: "z-3 absolute right-0 bottom-0 left-0 min-w-0 max-h-[78%] flex flex-col gap-2 p-3 overflow-hidden text-white bg-[linear-gradient(_to_top,_rgb(0_0_0_/_92%),_rgb(0_0_0_/_82%)_65%,_transparent_)] [&_:where(.dm-tag)]:text-white [&_:where(.dm-tag)]:bg-[rgb(255_255_255_/_16%)] [&_:where(.dm-tag)]:border-color-[rgb(255_255_255_/_28%)]",
          attributes: { n: "content-card-body" },
        },
        [
          View(
            {
              as: "h3",
              class: "m-0 overflow-hidden -webkit-box -webkit-box-orient-vertical text-ellipsis text-white text-base font-semibold leading-tight max-h-[calc(2_*_var(--dm-line-height-tight)_*_1em)] -webkit-line-clamp-2",
              attributes: {
                n: "content-card-title",
                title: content.title,
              },
            },
            [content.title || "未命名内容"],
          ),
          View(
            {
              class: "min-w-0 flex flex-nowrap items-center gap-1-5 overflow-hidden",
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
              class: "min-w-0 flex flex-wrap items-center gap-1-5",
              attributes: { n: "content-card-accounts" },
            },
            [ContentCardAccounts({ content })],
          ),
          View(
            {
              class: "flex items-center justify-between gap-2 pt-2 border-top-[var(--dm-border-width)_solid_rgb(255_255_255_/_22%)]",
              attributes: { n: "content-card-footer" },
            },
            [
              View(
                {
                  class: "min-w-0 flex flex-wrap gap-1",
                  attributes: { n: "content-card-statistics" },
                },
                ContentRowStatistics({
                  statistics: vm$.methods.statistics(content),
                }),
              ),
              View(
                {
                  class: "flex-[0_0_auto] text-[rgb(255_255_255_/_82%)] text-caption font-variant-numeric-tabular-nums whitespace-nowrap",
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
      class: "flex-[1_1_auto] grid place-items-center p-8 text-center [&_.dm-button]:mt-3",
      attributes: {
        n: "content-card-state",
        role: props.error ? "alert" : "status",
      },
    },
    [
      View(
        {
          as: "strong",
          class: "m-0 text-content-text text-lg",
          attributes: { n: "content-card-state-title" },
        },
        [props.title],
      ),
      View(
        {
          class: "m-0 mt-1 max-w-360px text-content-muted text-sm",
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
      class: "group-content-main flex-[1_1_auto] min-w-0 p-4 [.group-content-main&]:p-0 ",
      attributes: { n: "content-card-page-main" },
    },
    [
      Match({
        when: vm$.state.status,
        cases: {
          initial() {
            return View(
              {
                class: "relative min-h-0 flex-[1_1_auto] grid [align-items:start] grid-auto-rows-360px grid-cols-[repeat(auto-fill,_minmax(240px,_1fr))] gap-3 p-4 overflow-y-auto",
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
                class: "relative min-h-0 flex-[1_1_auto] grid [align-items:start] grid-auto-rows-360px grid-cols-[repeat(auto-fill,_minmax(240px,_1fr))] gap-3 p-4 overflow-y-auto",
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
                        class: "isolation-isolate relative min-w-0 h-full min-h-0",
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
                        class: "absolute inset-0 z-1 grid place-items-center text-content-text bg-[color-mix(_in_srgb,_var(--dm-content-panel)_78%,_transparent_)]",
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
          containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4 ",
          containerAttributes: { n: "content-page-main" },
          panelAttributes: { n: "content-table-panel" },
          columns: [
            {
              name: "main",
              title: "封面 / 标题",
              width: "minmax(300px, 2fr)",
              cellClass:
                "group-content-row-main-cell dm-flex dm-items-center dm-gap-4 dm-min-w-0",
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
                "group-content-row-author overflow-hidden dm-flex dm-items-center dm-gap-1-5 dm-min-w-0",
              render(content) {
                return ContentRowAccounts({ content });
              },
            },
            {
              name: "time",
              title: "时间",
              width: 240,
              cellClass:
                "group-content-row-meta dm-flex dm-items-center dm-gap-1-5 dm-text-muted dm-text-sm dm-tabular-nums dm-whitespace-nowrap",
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
              cellClass: "group-content-row-stats min-w-0 flex flex-wrap items-center gap-1-5 text-content-muted text-xs font-variant-numeric-tabular-nums",
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
              class: detail_href ? "cursor-pointer hover:relative hover:z-1 hover:bg-content-panel-soft hover:shadow-[inset_var(--dm-border-width-strong)_0_var(--dm-color-primary-fill)]" : "",
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

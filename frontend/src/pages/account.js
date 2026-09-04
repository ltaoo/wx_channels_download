import { AccountViewModel } from "./account.model.js";
import { TablePlatformBadge } from "../components.js";

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

function AccountContentRecord(props) {
  const vm$ = props.store;
  const content = props.content;
  const source_url = String(content.url || "").trim();
  return [
    Show({
      when: content.cover_url,
      ok() {
        return LazyImg({
          class: "account-content-cover",
          src: content.cover_url,
          alt: content.title,
          attributes: {
            n: "account-content-cover",
            loading: "lazy",
            referrerpolicy: "no-referrer",
          },
        });
      },
    }),
    View(
      {
        class: "account-content-record-main",
        attributes: { n: "account-content-record-main" },
      },
      [
        View(
          {
            type: source_url ? "a" : "span",
            class: "account-content-record-title account-name",
            attributes: {
              n: "account-content-record-title",
              href: source_url || undefined,
              title: source_url ? "打开原内容" : content.title,
            },
            onClick(event) {
              if (!source_url) return;
              event.preventDefault();
              vm$.methods.openContent(content);
            },
          },
          [content.title || content.external_id || "未命名内容"],
        ),
        View(
          {
            class: "account-content-record-id account-id",
            attributes: {
              n: "account-content-record-id",
              title: content.id || "",
            },
          },
          [content.id || "-"],
        ),
      ],
    ),
  ];
}

function AccountContentsDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.account_contents_drawer$,
      class: "dm-drawer--wide account-content-drawer",
      attributes: { n: "account-content-drawer" },
    },
    [
      View(
        {
          class: "dm-drawer-body account-content-drawer-body",
          attributes: { n: "account-content-drawer-body" },
        },
        [
          View(
            {
              class: "account-content-drawer-toolbar",
              attributes: { n: "account-content-drawer-toolbar" },
            },
            [
              View(
                {
                  class: "account-content-drawer-account",
                  attributes: { n: "account-content-drawer-account" },
                },
                [
                  View(
                    {
                      class: "account-content-drawer-account-name",
                      attributes: { n: "account-content-drawer-account-name" },
                    },
                    [
                      computed(
                        vm$.state.selected_account,
                        (account) => (account && account.nickname) || "账号",
                      ),
                    ],
                  ),
                  View(
                    {
                      class: "account-content-drawer-summary",
                      attributes: { n: "account-content-drawer-summary" },
                    },
                    [vm$.state.drawer_summary],
                  ),
                ],
              ),
              AccountPageActionButton({
                name: "account-synchronize-action",
                store: vm$.ui.btn_synchronize$,
                icon: "rotate-ccw",
                label: "同步",
                title: "抓取主页内容，仅预览不保存",
              }),
            ],
          ),
          Table({
            name: "account-content-table",
            containerClass: "account-content-table",
            containerAttributes: { n: "account-content-table" },
            panelAttributes: { n: "account-content-table-panel" },
            columns: [
              {
                name: "content",
                title: "内容",
                cellClass: "account-content-record",
                render(content) {
                  return AccountContentRecord({ store: vm$, content });
                },
              },
              {
                name: "type",
                title: "类型",
                width: 100,
                cellClass: "account-content-count",
                render(content) {
                  return vm$.methods.contentTypeLabel(
                    content.content_type,
                    content.content_subtype,
                  );
                },
              },
              {
                name: "time",
                title: "发布时间",
                width: 160,
                cellClass: "account-added",
                render(content) {
                  return vm$.methods.formatTime(content.publish_time);
                },
              },
              {
                name: "download-tasks",
                title: "下载任务",
                width: 110,
                cellClass: "account-content-count",
                render(content) {
                  return vm$.methods.formatDownloadTaskCount(content);
                },
              },
            ],
            rows: vm$.state.drawer_contents,
            rowKey: "id",
            status: vm$.state.drawer_status,
            error: vm$.state.drawer_error,
            skeletonCount: 6,
            errorTitle: "内容加载失败",
            retry: { store: vm$.ui.btn_drawer_retry$ },
            emptyTitle: "暂无内容",
            emptyDescription: "点击同步可预览平台主页内容",
          }),
        ],
      ),
    ],
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
              type: "text",
              autocomplete: "off",
              "aria-label": "搜索账号昵称或 ID",
            },
          }),
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
    View({
      class: "account-avatar-fallback",
      attributes: { n: "account-avatar-fallback" },
    }, [
      Timeless.Icon({
        name: "user",
        size: 20,
        attributes: { n: "account-avatar-fallback-icon" },
      }),
    ]),
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
  return TablePlatformBadge({
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
    emptyTitle: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim() ? "没有匹配的账号" : "暂无账号",
    ),
    emptyDescription: computed(vm$.state.keyword, (keyword) =>
      String(keyword || "").trim()
        ? "请尝试其他昵称或账号 ID"
        : "还没有记录任何账号",
    ),
  });
}

export default AccountPageView;

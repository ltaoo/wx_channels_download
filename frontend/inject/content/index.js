/// <reference path="../utils.js" />
/// <reference path="core.js" />
/**
 * @file Content list page entry.
 */
function ContentPageActionButton(props) {
  return View(
    {
      type: "button",
      class: [
        "wx-content-action",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
    },
    [
      props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
      props.label
        ? View({ class: "wx-content-action-label" }, [props.label])
        : null,
    ].filter(Boolean),
  );
}

function ContentPageHeader(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "library", size: 28 }),
        ]),
        View({}, [
          View({ class: "wx-content-title" }, ["Contents"]),
          View({ class: "wx-content-subtitle" }, [
            computed(vm$.state.total, (total) => `管理已记录的 ${total} 条内容`),
          ]),
        ]),
      ]),
      ContentPageActionButton({
        icon: "refresh-cw",
        label: "刷新",
        onClick() {
          vm$.methods.refresh();
        },
      }),
    ]),
  ]);
}

function ContentPageToolbar(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-toolbar" }, [
    View({ class: "wx-content-search" }, [
      Timeless.Icon({ name: "search", size: 16 }),
      View({
        type: "input",
        class: "wx-content-search-input",
        attributes: {
          type: "search",
          placeholder: "搜索标题或描述",
          autocomplete: "off",
        },
        onInput(event) {
          vm$.methods.setKeyword(event.target.value);
        },
        onKeyDown(event) {
          if (event.key === "Enter") {
            vm$.methods.search();
          }
        },
      }),
    ]),
    ContentPageActionButton({
      icon: "search",
      label: "搜索",
      onClick() {
        vm$.methods.search();
      },
    }),
    Timeless.Select({
      store: vm$.ui.content_type,
      class: "wx-content-type-select",
    }),
  ]);
}

function ContentCover(props) {
  const content = props.content;
  const fallback = View({ class: "wx-content-cover-fallback" }, [
    Timeless.Icon({ name: "file", size: 32 }),
    View({ class: "wx-content-cover-type" }, [
      props.store.methods.typeLabel(content.content_type),
    ]),
  ]);
  if (!content.cover_url) {
    return fallback;
  }
  return View({ class: "wx-content-cover-wrap" }, [
    fallback,
    View({
      type: "img",
      class: "wx-content-cover",
      attributes: {
        src: content.cover_url,
        alt: content.title,
        loading: "lazy",
        referrerpolicy: "no-referrer",
      },
      onError(event) {
        event.target.style.display = "none";
      },
    }),
  ]);
}

function ContentAccountItem(props) {
  const vm$ = props.store;
  const account = props.account;
  const name =
    account.nickname ||
    account.alias ||
    account.external_id ||
    "未知账号";
  const secondaryName =
    account.alias && account.alias !== name
      ? account.alias
      : account.external_id && account.external_id !== name
        ? account.external_id
        : "";
  const followerCount = vm$.methods.formatCount(account.follower_count);
  const role = vm$.methods.accountRoleLabel(account.role);
  const profileURL = String(account.profile_url || "").trim();
  const clickable = Boolean(profileURL);

  return View(
    {
      type: clickable ? "button" : "div",
      class: [
        "wx-content-account-item",
        clickable ? "wx-content-account-item-clickable" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: clickable
        ? {
            type: "button",
            title: `打开 ${name} 的主页`,
          }
        : {},
      onClick(event) {
        if (!clickable) {
          return;
        }
        event.stopPropagation();
        window.open(profileURL, "_blank", "noopener,noreferrer");
      },
    },
    [
      account.avatar_url
        ? View({
            type: "img",
            class: "wx-content-avatar",
            attributes: {
              src: account.avatar_url,
              alt: name,
              loading: "lazy",
              referrerpolicy: "no-referrer",
            },
            onError(event) {
              event.target.style.display = "none";
            },
          })
        : View({ class: "wx-content-avatar wx-content-avatar-fallback" }, [
            String(name).slice(0, 1),
          ]),
      View({ class: "wx-content-account-details" }, [
        View({ class: "wx-content-account-name-line" }, [
          View(
            {
              class: "wx-content-account-name",
              attributes: { title: name },
            },
            [name],
          ),
          role
            ? View({ class: "wx-content-account-role" }, [role])
            : null,
        ].filter(Boolean)),
        secondaryName || followerCount
          ? View({ class: "wx-content-account-meta" }, [
              secondaryName
                ? View(
                    {
                      class: "wx-content-account-alias",
                      attributes: { title: secondaryName },
                    },
                    [secondaryName],
                  )
                : null,
              followerCount
                ? View({ class: "wx-content-account-followers" }, [
                    `${followerCount} 粉丝`,
                  ])
                : null,
            ].filter(Boolean))
          : null,
      ].filter(Boolean)),
      clickable
        ? Timeless.Icon({ name: "external-link", size: 13 })
        : null,
    ].filter(Boolean),
  );
}

function ContentAccountList(props) {
  const accounts = props.content.accounts || [];
  if (accounts.length === 0) {
    return View({ class: "wx-content-accounts-empty wx-content-muted" }, [
      Timeless.Icon({ name: "user", size: 14 }),
      "暂无关联账号",
    ]);
  }
  return View({ class: "wx-content-accounts" }, [
    View({ class: "wx-content-accounts-header" }, [
      Timeless.Icon({ name: "users", size: 14 }),
      `${accounts.length} 个关联账号`,
    ]),
    View(
      { class: "wx-content-account-list" },
      accounts.map((account) =>
        ContentAccountItem({ store: props.store, account })),
    ),
  ]);
}

function ContentCard(props) {
  const vm$ = props.store;
  const content = props.content;
  const status = vm$.methods.downloadStatus(content.download_status);
  const description = String(content.description || "").trim();
  const showDescription = description && description !== content.title;
  return View(
    {
      class: [
        "wx-content-card",
        content.url ? "wx-content-card-clickable" : "",
      ]
        .filter(Boolean)
        .join(" "),
      onClick() {
        vm$.methods.openSource(content);
      },
    },
    [
      View({ class: "wx-content-card-media" }, [
        ContentCover({ store: vm$, content }),
        View({ class: `wx-content-status wx-content-status-${status.tone}` }, [
          status.label,
        ]),
      ]),
      View({ class: "wx-content-card-body" }, [
        View({ class: "wx-content-card-tags" }, [
          View({ class: "wx-content-platform" }, [
            vm$.methods.platformName(content),
          ]),
          View({ class: "wx-content-type-badge" }, [
            vm$.methods.typeLabel(content.content_type),
          ]),
        ]),
        View(
          {
            class: "wx-content-card-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        showDescription
          ? View({ class: "wx-content-description" }, [description])
          : null,
        ContentAccountList({ store: vm$, content }),
        View({ class: "wx-content-card-footer" }, [
          View({ class: "wx-content-meta" }, [
            Timeless.Icon({ name: "clock3", size: 13 }),
            vm$.methods.formatTime(content.publish_time),
          ]),
          View({ class: "wx-content-card-footer-right" }, [
            vm$.methods.formatBytes(content.file_size)
              ? View({ class: "wx-content-size" }, [
                  vm$.methods.formatBytes(content.file_size),
                ])
              : null,
            content.url
              ? Timeless.Icon({ name: "external-link", size: 15 })
              : null,
          ].filter(Boolean)),
        ]),
        content.error_msg
          ? View({ class: "wx-content-error-message" }, [content.error_msg])
          : null,
      ].filter(Boolean)),
    ],
  );
}

function ContentSkeletonCard() {
  return View({ class: "wx-content-card wx-content-skeleton-card" }, [
    View({ class: "wx-content-card-media wx-content-skeleton" }),
    View({ class: "wx-content-card-body" }, [
      View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
    ]),
  ]);
}

function ContentPageBody(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-main" }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return View({ class: "wx-content-grid" },
          Array.from({ length: 8 }, () => ContentSkeletonCard()),
        );
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "wx-content-state" }, [
              Timeless.Icon({ name: "circle-alert", size: 32 }),
              View({ class: "wx-content-state-title" }, ["内容加载失败"]),
              View({ class: "wx-content-state-text" }, [vm$.state.error]),
              ContentPageActionButton({
                icon: "refresh-cw",
                label: "重试",
                onClick() {
                  vm$.methods.refresh();
                },
              }),
            ]);
          },
          else() {
            return Show({
              when: computed(
                vm$.state.contents,
                (contents) => contents.length > 0,
              ),
              ok() {
                return View({ class: "wx-content-grid" }, [
                  For({
                    each: vm$.state.contents,
                    render(content_) {
                      const content =
                        content_ && content_.value !== undefined
                          ? content_.value
                          : content_;
                      return ContentCard({ store: vm$, content });
                    },
                  }),
                ]);
              },
              else() {
                return View({ class: "wx-content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "wx-content-state-title" }, ["暂无内容"]),
                  View({ class: "wx-content-state-text" }, [
                    "调整搜索条件后再试试",
                  ]),
                ]);
              },
            });
          },
        });
      },
    }),
  ]);
}

function ContentPagination(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-pagination" }, [
    View({ class: "wx-content-pagination-summary" }, [
      vm$.state.range_text,
    ]),
    View({ class: "wx-content-pagination-controls" }, [
      View({
        type: "select",
        class: "wx-content-select wx-content-page-size",
        attributes: { "aria-label": "每页数量" },
        onChange(event) {
          vm$.methods.setPageSize(event.target.value);
        },
      }, [
        View({ type: "option", attributes: { value: "12" } }, ["12 条/页"]),
        View(
          { type: "option", attributes: { value: "24", selected: true } },
          ["24 条/页"],
        ),
        View({ type: "option", attributes: { value: "48" } }, ["48 条/页"]),
        View({ type: "option", attributes: { value: "96" } }, ["96 条/页"]),
      ]),
      ContentPageActionButton({
        icon: "chevron-left",
        title: "上一页",
        compact: true,
        class: computed(
          vm$.state.page,
          (page) => page <= 1 ? "wx-content-action-disabled" : "",
        ),
        onClick() {
          vm$.methods.previousPage();
        },
      }),
      View({ class: "wx-content-page-number" }, [
        computed(
          { page: vm$.state.page, pageCount: vm$.state.page_count },
          (state) => `${state.page} / ${state.pageCount}`,
        ),
      ]),
      ContentPageActionButton({
        icon: "chevron-right",
        title: "下一页",
        compact: true,
        class: computed(
          { page: vm$.state.page, pageCount: vm$.state.page_count },
          (state) =>
            state.page >= state.pageCount
              ? "wx-content-action-disabled"
              : "",
        ),
        onClick() {
          vm$.methods.nextPage();
        },
      }),
    ]),
  ]);
}

function ContentLibraryPageView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-page",
      onMounted() {
        vm$.ready();
      },
    },
    [
      ContentPageHeader({ store: vm$ }),
      View({ class: "wx-content-toolbar-wrap" }, [
        ContentPageToolbar({ store: vm$ }),
      ]),
      ContentPageBody({ store: vm$ }),
      ContentPagination({ store: vm$ }),
    ],
  );
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = ContentLibraryModel();
    Timeless.DOM.render(ContentLibraryPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

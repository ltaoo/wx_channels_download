/// <reference path="../utils.js" />
/// <reference path="../virtual-list-view.js" />
/// <reference path="model.js" />
/**
 * @file Account list page entry and rendering.
 */
function AccountPageActionButton(props) {
  return View(
    {
      type: "button",
      class: "wx-content-action",
      attributes: {
        type: "button",
        title: props.title || "",
      },
      onClick: props.onClick,
    },
    [
      Timeless.Icon({ name: props.icon, size: 16 }),
      View({ class: "wx-content-action-label" }, [props.label]),
    ],
  );
}

function AccountPageHeader(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "user", size: 28 }),
        ]),
        View({}, [
          View({ class: "wx-content-title" }, ["账号"]),
          View({ class: "wx-content-subtitle" }, [
            computed(vm$.state.total, (total) => `管理已记录的 ${total} 个账号`),
          ]),
        ]),
      ]),
      AccountPageActionButton({
        icon: "refresh-cw",
        label: "刷新",
        onClick() {
          vm$.methods.refresh();
        },
      }),
    ]),
  ]);
}

function AccountAvatar(props) {
  const account = props.account;
  return View({ class: "wx-account-avatar-wrap" }, [
    View({ class: "wx-account-avatar-fallback" }, [
      Timeless.Icon({ name: "user", size: 20 }),
    ]),
    Show({
      when: account.avatar_url,
      ok() {
        return Img({
          class: "wx-account-avatar",
          src: account.avatar_url,
          alt: account.nickname,
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
  ]);
}

function AccountPlatform(props) {
  const vm$ = props.store;
  const account = props.account;
  const favicon = vm$.methods.platformFavicon(account);
  return View({ class: "wx-content-row-platform" }, [
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
    vm$.methods.platformName(account),
  ]);
}

function AccountRow(props) {
  const vm$ = props.store;
  const account = props.account;
  return View({ class: "wx-content-row wx-account-row" }, [
    View(
      {
        class: "wx-account-identity",
        attributes: {
          title: account.nickname || account.external_id || "",
        },
      },
      [
        AccountAvatar({ account }),
        View({ class: "wx-account-details" }, [
          View({ class: "wx-account-name" }, [account.nickname]),
          AccountPlatform({ store: vm$, account }),
        ]),
      ],
    ),
    View({ class: "wx-account-content-count" }, [
      Timeless.Icon({ name: "file", size: 12 }),
      vm$.methods.formatContentCount(account.content_count),
    ]),
    View({ class: "wx-account-added" }, [
      Timeless.Icon({ name: "clock3", size: 12 }),
      vm$.methods.formatTime(account.created_at),
    ]),
  ]);
}

function AccountTableHead() {
  return View({ class: "wx-content-row wx-content-row-head wx-account-row" }, [
    View({ class: "wx-content-row-head-cell" }, ["账号"]),
    View({ class: "wx-content-row-head-cell" }, ["关联内容"]),
    View({ class: "wx-content-row-head-cell" }, ["添加时间"]),
  ]);
}

function AccountSkeletonRow() {
  return View({ class: "wx-content-row wx-account-row wx-content-skeleton-row" }, [
    View({ class: "wx-account-identity" }, [
      View({ class: "wx-account-avatar-wrap wx-content-skeleton" }),
      View({ class: "wx-account-skeleton-details" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      ]),
    ]),
    View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
    View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
  ]);
}

function AccountListView(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-history-list" }, [
    VirtualListView({
      style: {
        height: "100%",
        "max-height": "100%",
        overflow: "auto",
        position: "relative",
        "box-sizing": "border-box",
        "background-color": "transparent",
      },
      key: "id",
      size: 10,
      buffer: 6,
      gutter: 1,
      itemHeight: 72,
      each: vm$.state.accounts,
      render(account_) {
        const account =
          account_ && account_.value !== undefined
            ? account_.value
            : account_;
        return AccountRow({ store: vm$, account });
      },
    }),
  ]);
}

function AccountPageBody(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-main" }, [
    Show({
      when: vm$.state.initial_loading,
      ok() {
        return View(
          { class: "wx-content-rows" },
          Array.from({ length: 8 }, () => AccountSkeletonRow()),
        );
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "wx-content-state" }, [
              Timeless.Icon({ name: "circle-alert", size: 32 }),
              View({ class: "wx-content-state-title" }, ["账号加载失败"]),
              View({ class: "wx-content-state-text" }, [vm$.state.error]),
              AccountPageActionButton({
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
                vm$.state.accounts,
                (accounts) => accounts.length > 0,
              ),
              ok() {
                return View(
                  { class: "wx-content-rows wx-content-history-rows" },
                  [AccountTableHead(), AccountListView({ store: vm$ })],
                );
              },
              else() {
                return View({ class: "wx-content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "wx-content-state-title" }, ["暂无账号"]),
                  View({ class: "wx-content-state-text" }, [
                    "还没有记录任何账号",
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

function AccountPageView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-page wx-account-page",
      onMounted() {
        vm$.ready();
      },
    },
    [
      AccountPageHeader({ store: vm$ }),
      AccountPageBody({ store: vm$ }),
      Show({
        when: computed(vm$.state.accounts, (accounts) => accounts.length > 0),
        ok() {
          return View({ class: "wx-content-pagination" }, [
            View({ class: "wx-content-pagination-summary" }, [
              vm$.state.summary_text,
            ]),
          ]);
        },
      }),
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
    const vm$ = AccountModel();
    Timeless.DOM.render(AccountPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

import {
  Button,
  Input,
  Pagination,
} from "../components.js";
import { AccountViewModel } from "./account.model.js";

function AccountPageView(props) {
  const vm$ = AccountViewModel(props);

  return View(
    {
      class: "wx-content-page wx-account-page dm-page",
      onMounted() {
        console.count("AccountPage onMounted");
        vm$.methods.ready();
      },
    },
    [
      View({ class: "wx-content-toolbar-wrap wx-account-toolbar-wrap" }, [
        AccountPageToolbar({ store: vm$ }),
      ]),
      AccountPageBody({ store: vm$ }),
      Show({
        when: computed(vm$.state.accounts, (accounts) => accounts.length > 0),
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
    ],
  );
}

function AccountPageActionButton(props) {
  return Button(
    {
      store: props.store,
      class: "wx-account-page-button",
      attributes: {
        type: props.type || "button",
        title: props.title || "",
      },
      prefix: Timeless.Icon({ name: props.icon, size: 16 }),
    },
    [View({ class: "wx-content-action-label" }, [props.label])],
  );
}

function AccountPageToolbar(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-content-toolbar wx-account-toolbar",
      attributes: { role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View({ class: "wx-content-search wx-account-search" }, [
        Timeless.Icon({ name: "search", size: 16 }),
        Input({
          store: vm$.ui.input_keyword$,
          class: "wx-content-search-input",
          attributes: {
            name: "keyword",
            type: "search",
            autocomplete: "off",
            "aria-label": "搜索账号昵称或 ID",
          },
        }),
      ]),
      View({ class: "wx-content-filter-actions" }, [
        AccountPageActionButton({
          store: vm$.ui.btn_search$,
          icon: "search",
          label: "搜索",
          variant: "primary",
          type: "submit",
        }),
        AccountPageActionButton({
          store: vm$.ui.btn_refresh$,
          icon: "refresh-cw",
          label: "刷新",
        }),
      ]),
    ],
  );
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
  const favicon = window.PLATFORM_FAVICONS[account.platform_id] || "";
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
          View({ class: "wx-account-meta" }, [
            AccountPlatform({ store: vm$, account }),
            View(
              {
                class: "wx-account-id",
                attributes: { title: account.id || "" },
              },
              [`ID: ${account.id || "-"}`],
            ),
          ]),
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
  return View(
    { class: "wx-content-row wx-content-row-head wx-account-row" },
    [
      View({ class: "wx-content-row-head-cell" }, ["账号"]),
      View({ class: "wx-content-row-head-cell" }, ["关联内容"]),
      View({ class: "wx-content-row-head-cell" }, ["添加时间"]),
    ],
  );
}

function AccountSkeletonRow() {
  return View(
    { class: "wx-content-row wx-account-row wx-content-skeleton-row" },
    [
      View({ class: "wx-account-identity" }, [
        View({ class: "wx-account-avatar-wrap wx-content-skeleton" }),
        View({ class: "wx-account-skeleton-details" }, [
          View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
          View({
            class: "wx-content-skeleton wx-content-skeleton-line-short",
          }),
        ]),
      ]),
      View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
    ],
  );
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
      gutter: 0,
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
  return View({ class: "wx-content-main dm-container" }, [
    Show({
      when: vm$.state.initial_loading,
      ok() {
        return View(
          { class: "wx-content-rows dm-panel" },
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
                store: vm$.ui.btn_retry$,
                icon: "refresh-cw",
                label: "重试",
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
                  {
                    class:
                      "wx-content-rows wx-content-history-rows dm-panel",
                  },
                  [AccountTableHead(), AccountListView({ store: vm$ })],
                );
              },
              else() {
                return View({ class: "wx-content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "wx-content-state-title" }, [
                    computed(vm$.state.keyword, (keyword) =>
                      String(keyword || "").trim()
                        ? "没有匹配的账号"
                        : "暂无账号",
                    ),
                  ]),
                  View({ class: "wx-content-state-text" }, [
                    computed(vm$.state.keyword, (keyword) =>
                      String(keyword || "").trim()
                        ? "请尝试其他昵称或账号 ID"
                        : "还没有记录任何账号",
                    ),
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

export default AccountPageView;

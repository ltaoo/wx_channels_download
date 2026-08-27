import { AccountViewModel } from "./account.model.js";
import { TableWithVirtualList } from "./table.js";

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
      onClick: props.onClick,
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
      Input({
        store: vm$.ui.input_keyword$,
        rootClass: "wx-content-search wx-account-search",
        rootAttributes: { n: "account-search-field" },
        prefix: Timeless.Icon({
          name: "search",
          size: 16,
          attributes: { n: "account-search-icon" },
        }),
        class: "wx-content-search-input",
        attributes: {
          n: "account-search-input",
          name: "keyword",
          type: "text",
          autocomplete: "off",
          "aria-label": "搜索账号昵称或 ID",
        },
      }),
      View({ class: "wx-content-filter-actions" }, [
        AccountPageActionButton({
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
          store: vm$.ui.btn_refresh$,
          icon: "rotate-ccw",
          label: "重置",
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

function AccountIdentity(props) {
  const vm$ = props.store;
  const account = props.account;
  return [
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
  ];
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

function AccountPageBody(props) {
  const vm$ = props.store;
  return TableWithVirtualList({
    name: "account-table",
    containerAttributes: { n: "account-page-main" },
    panelAttributes: { n: "account-table-panel" },
    headerClass: "wx-content-row wx-content-row-head wx-account-row",
    columns: [
      {
        name: "account",
        title: "账号",
        cellClass: "wx-account-identity",
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
        cellClass: "wx-account-content-count",
        render(account) {
          return [
            Timeless.Icon({ name: "file", size: 12 }),
            vm$.methods.formatContentCount(account.content_count),
          ];
        },
      },
      {
        name: "added",
        title: "添加时间",
        cellClass: "wx-account-added",
        render(account) {
          return [
            Timeless.Icon({ name: "clock3", size: 12 }),
            vm$.methods.formatTime(account.created_at),
          ];
        },
      },
    ],
    rows: vm$.state.accounts,
    rowKey: "id",
    status: vm$.state.status,
    loading: vm$.state.loading,
    error: vm$.state.error,
    rowClass: "wx-content-row wx-account-row",
    skeletonCount: 8,
    renderSkeletonRow: AccountSkeletonRow,
    size: 10,
    buffer: 6,
    gutter: 0,
    itemHeight: 72,
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

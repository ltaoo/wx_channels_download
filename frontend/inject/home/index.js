/// <reference path="../utils.js" />
/// <reference path="model.js" />
/**
 * @file Scraper fetch page rendering.
 */
function HomePageForm(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "wx-home-form",
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.submit();
      },
    },
    [
      View({ class: "wx-content-search wx-home-search" }, [
        Timeless.Icon({ name: "search", size: 16 }),
        Input({
          class: "wx-content-search-input",
          value: vm$.state.url,
          placeholder: "输入需要获取的内容 URL",
          attributes: {
            type: "text",
            name: "url",
            autocomplete: "off",
            autofocus: "autofocus",
            "aria-label": "内容 URL",
          },
          onInput(event) {
            const target =
              event &&
              event.target &&
              typeof event.target.get$elm === "function"
                ? event.target.get$elm()
                : event && event.target;
            vm$.methods.setURL(
              target && typeof target.value === "string" ? target.value : "",
            );
          },
        }),
      ]),
      Button(
        {
          class: "wx-content-action wx-home-submit",
          disabled: vm$.state.submit_disabled,
          attributes: { type: "button" },
          onClick() {
            vm$.methods.submit();
          },
        },
        [
          computed(vm$.state.loading, (loading) =>
            loading ? "获取中" : "确认",
          ),
        ],
      ),
    ],
  );
}

function HomeContentCover(props) {
  const content = props.content;
  return View({ class: "wx-home-content-cover" }, [
    View({ class: "wx-home-content-cover-fallback" }, [
      Timeless.Icon({ name: "file", size: 34 }),
      content.content_type_name,
    ]),
    Show({
      when: content.cover_url,
      ok() {
        return Img({
          class: "wx-home-content-cover-image",
          src: content.cover_url,
          alt: content.title,
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

function HomeContentCard(props) {
  const vm$ = props.store;
  const content = vm$.state.content;
  return View({ class: "wx-home-card wx-home-content-card" }, [
    View({ class: "wx-home-card-heading" }, [
      View({ class: "wx-home-card-title-group" }, [
        View({ class: "wx-home-card-icon" }, [
          Timeless.Icon({ name: "file", size: 17 }),
        ]),
        View({}, [
          View({ class: "wx-home-card-kicker" }, ["CONTENT"]),
          View({ class: "wx-home-card-title" }, ["内容"]),
        ]),
      ]),
      Button(
        {
          class: "wx-content-action wx-home-download",
          disabled: vm$.state.download_disabled,
          attributes: {
            type: "button",
            title: "创建下载任务",
          },
          onClick() {
            vm$.methods.createDownloadTask();
          },
        },
        [
          Timeless.Icon({ name: "download", size: 16 }),
          View({ class: "wx-content-action-label" }, [
            vm$.state.download_button_text,
          ]),
        ],
      ),
    ]),
    View({ class: "wx-home-content-card-body" }, [
      HomeContentCover({ content }),
      View({ class: "wx-home-content-info" }, [
        View({ class: "wx-home-badges" }, [
          View({ class: "wx-home-badge wx-home-badge-primary" }, [
            content.platform_name,
          ]),
          View({ class: "wx-home-badge" }, [content.content_type_name]),
        ]),
        View(
          {
            class: "wx-home-content-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        Show({
          when: content.show_description,
          ok() {
            return View({ class: "wx-home-content-description" }, [
              content.description,
            ]);
          },
        }),
        View({ class: "wx-home-content-meta" }, [
          Timeless.Icon({ name: "clock3", size: 14 }),
          content.publish_time_text,
        ]),
        View({ class: "wx-home-metrics" }, [
          HomeContentMetric({ label: "浏览", value: content.view_count_text }),
          HomeContentMetric({ label: "点赞", value: content.like_count_text }),
          HomeContentMetric({
            label: "评论",
            value: content.comment_count_text,
          }),
        ]),
        Show({
          when: content.content_url,
          ok() {
            return Button(
              {
                class: "wx-home-link-button",
                attributes: { type: "button", title: "打开原内容" },
                onClick() {
                  vm$.methods.openContent();
                },
              },
              [
                "打开原内容",
                Timeless.Icon({ name: "external-link", size: 14 }),
              ],
            );
          },
        }),
      ]),
    ]),
  ]);
}

function HomeContentMetric(props) {
  return View({ class: "wx-home-metric" }, [
    View({ class: "wx-home-metric-label" }, [props.label]),
    View({ class: "wx-home-metric-value" }, [props.value]),
  ]);
}

function HomeAccountAvatar(props) {
  const account = props.account;
  return View({ class: "wx-home-account-avatar" }, [
    View({ class: "wx-home-account-avatar-fallback" }, [
      account.avatar_fallback,
    ]),
    Show({
      when: account.avatar_url,
      ok() {
        return Img({
          class: "wx-home-account-avatar-image",
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

function HomeAccountCard(props) {
  const vm$ = props.store;
  const account = vm$.state.account;
  return View({ class: "wx-home-card wx-home-account-card" }, [
    View({ class: "wx-home-card-heading" }, [
      View({ class: "wx-home-card-title-group" }, [
        View({ class: "wx-home-card-icon" }, [
          Timeless.Icon({ name: "user", size: 17 }),
        ]),
        View({}, [
          View({ class: "wx-home-card-kicker" }, ["ACCOUNT"]),
          View({ class: "wx-home-card-title" }, ["账号"]),
        ]),
      ]),
      View({ class: "wx-home-badge wx-home-badge-primary" }, [
        account.platform_name,
      ]),
    ]),
    Show({
      when: account.present,
      ok() {
        return View({ class: "wx-home-account-card-body" }, [
          HomeAccountAvatar({ account }),
          View({ class: "wx-home-account-name" }, [account.nickname]),
          Show({
            when: account.identity,
            ok() {
              return View({ class: "wx-home-account-identity" }, [
                account.identity,
              ]);
            },
          }),
          Show({
            when: account.signature,
            ok() {
              return View({ class: "wx-home-account-signature" }, [
                account.signature,
              ]);
            },
          }),
          View({ class: "wx-home-account-follower" }, [
            Timeless.Icon({ name: "users", size: 15 }),
            account.follower_count_text,
          ]),
          Show({
            when: account.profile_url,
            ok() {
              return Button(
                {
                  class: "wx-home-link-button wx-home-account-link",
                  attributes: { type: "button", title: "打开账号主页" },
                  onClick() {
                    vm$.methods.openAccount();
                  },
                },
                [
                  "打开账号主页",
                  Timeless.Icon({ name: "external-link", size: 14 }),
                ],
              );
            },
          }),
        ]);
      },
      else() {
        return View({ class: "wx-home-account-empty" }, [
          View({ class: "wx-home-account-empty-icon" }, [
            Timeless.Icon({ name: "user-round-x", size: 26 }),
          ]),
          View({ class: "wx-home-account-name" }, [account.nickname]),
          View({ class: "wx-home-account-signature" }, [
            "当前解析结果没有关联账号信息",
          ]),
        ]);
      },
    }),
  ]);
}

function HomeRawJSON(props) {
  const vm$ = props.store;
  return View({ class: "wx-home-json" }, [
    Button(
      {
        class: computed(vm$.state.json_expanded, (expanded) =>
          expanded
            ? "wx-home-json-toggle is-expanded"
            : "wx-home-json-toggle",
        ),
        attributes: {
          type: "button",
          "aria-controls": "wx-home-json-body",
          "aria-expanded": vm$.state.json_expanded,
        },
        onClick() {
          vm$.methods.toggleJSON();
        },
      },
      [
        View({ class: "wx-home-json-toggle-main" }, [
          View({ class: "wx-home-json-icon" }, [
            Timeless.Icon({ name: "braces", size: 17 }),
          ]),
          View({}, [
            View({ class: "wx-home-json-title" }, [vm$.state.json_toggle_text]),
            View({ class: "wx-home-json-subtitle" }, [
              "完整接口响应，仅供调试与核对",
            ]),
          ]),
        ]),
        Show({
          when: vm$.state.json_expanded,
          ok() {
            return Timeless.Icon({ name: "chevron-up", size: 17 });
          },
          else() {
            return Timeless.Icon({ name: "chevron-down", size: 17 });
          },
        }),
      ],
    ),
    Show({
      when: vm$.state.json_expanded,
      ok() {
        return View(
          {
            type: "pre",
            class: "wx-home-json-body",
            attributes: { id: "wx-home-json-body" },
          },
          [vm$.state.result_text],
        );
      },
    }),
  ]);
}

function HomePageResult(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.has_result,
    ok() {
      return View({ class: "wx-home-result" }, [
        View({ class: "wx-home-card-grid" }, [
          HomeContentCard({ store: vm$ }),
          HomeAccountCard({ store: vm$ }),
        ]),
        HomeRawJSON({ store: vm$ }),
      ]);
    },
  });
}

function HomePageHeader() {
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "search", size: 28 }),
        ]),
        View({}, [
          View({ class: "wx-content-title" }, ["链接解析"]),
          View({ class: "wx-content-subtitle" }, [
            "输入内容链接，获取平台解析结果",
          ]),
        ]),
      ]),
    ]),
  ]);
}

function HomePageView(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-page wx-home-page" }, [
    HomePageHeader(),
    View({ class: "wx-content-main wx-home-main" }, [
      View({ class: "wx-home-content" }, [
        HomePageForm({ store: vm$ }),
        Show({
          when: computed(vm$.state.status_text, (text) => Boolean(text)),
          ok() {
            return View(
              {
                class: computed(vm$.state.has_error, (has_error) =>
                  has_error ? "wx-home-status error" : "wx-home-status",
                ),
              },
              [
                Show({
                  when: vm$.state.busy,
                  ok() {
                    return View({ class: "weui-loading" });
                  },
                }),
                vm$.state.status_text,
              ],
            );
          },
        }),
        HomePageResult({ store: vm$ }),
      ]),
    ]),
  ]);
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = HomePageModel();
    Timeless.DOM.render(HomePageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

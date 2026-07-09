import { AccountsPageModel } from "./accounts.model.js";
import { ProxyImg } from "@/components/proxy-img.js";

function Avatar(account) {
  const avatarURL = account.display_avatar_url || account.avatar_url || "";
  return View(
    {
      dataset: { t: "home-accounts-page-avatar-avatar-or-badge-image-or-row-text" },
      class:
        "h-11 w-11 shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
    },
    [
      avatarURL
        ? ProxyImg({
            class: "h-full w-full object-cover",
            src: avatarURL,
            alt: account.nickname,
            platformId: account.platform_id,
          })
        : View(
            {
              dataset: { t: "home-accounts-page-avatar-row-text" },
              class:
                "flex h-full w-full items-center justify-center text-sm font-medium text-zinc-500",
            },
            [String(account.nickname || "?").slice(0, 1)],
          ),
    ],
  );
}

function contentIconName(content) {
  const type = String(
    content.output_format || content.content_type || content.type || "",
  ).toLowerCase();
  switch (type) {
    case "video":
    case "short_video":
    case "mp4":
    case "webm":
      return "film";
    case "article":
    case "html":
    case "blog":
      return "file-code";
    case "json":
      return "braces";
    case "image":
    case "image_set":
    case "jpg":
    case "jpeg":
    case "png":
    case "webp":
      return "image";
    case "audio":
    case "podcast":
    case "music":
    case "mp3":
    case "m4a":
      return "music";
    case "zip":
    case "archive":
      return "archive";
    default:
      return "file";
  }
}

function shouldShowContentCover(content, coverURL) {
  if (!coverURL) return false;
  const text = [
    content.display_type,
    content.type_label,
    content.output_format,
    content.source_content_type,
    content.content_type,
    content.type,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  return !/(^|\s)(json|html|txt|md|pdf|zip|archive)(\s|$)/.test(text);
}

function MediaThumb(content, onOpenFile, onOpenSource) {
  const coverURL = content.display_cover_url || content.cover_url || "";
  const typeLabel =
    content.display_type ||
    content.type_label ||
    content.output_format ||
    content.content_type ||
    content.type ||
    "file";
  const sourceURL =
    content.source_url ||
    content.SourceURL ||
    content.url ||
    content.URL ||
    content.content_url ||
    content.ContentURL ||
    "";
  const showCover = shouldShowContentCover(content, coverURL);
  return View(
    {
      dataset: { t: "home-accounts-page-account-recent-content-thumbnail" },
      class:
        "group flex min-h-[54px] cursor-pointer items-center gap-2 rounded-md border border-zinc-200 bg-white px-2 py-1.5 text-left shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
      title: content.title || "内容",
      onClick: onOpenFile,
    },
    [
      View(
        {
          dataset: { t: "home-accounts-page-account-recent-content-cover" },
          class:
            "flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded bg-zinc-100 text-zinc-500 dark:bg-zinc-900 dark:text-zinc-400",
        },
        [
          Show({
            when: showCover,
            ok() {
              return ProxyImg({
                class: "h-full w-full object-cover",
                src: coverURL,
                alt: content.title || "cover",
                platformId: content.platform_id,
                contentType: content.content_type || content.type,
              });
            },
            else() {
              return View(
                {
                  dataset: { t: "home-accounts-page-media-thumb-row-icon-content-icon-name" },
                  class: "flex h-full w-full items-center justify-center",
                },
                [Icon({ name: contentIconName(content), size: 18 })],
              );
            },
          }),
        ],
      ),
      View(
        {
          dataset: { t: "home-accounts-page-account-recent-content-body" },
          class: "min-w-0 flex-1",
        },
        [
          View(
            {
              dataset: { t: "home-accounts-page-account-recent-content-title-text" },
              class:
                "line-clamp-1 text-xs font-medium leading-4 text-zinc-800 dark:text-zinc-100",
            },
            [content.title || "未命名内容"],
          ),
          View(
            {
              dataset: { t: "home-accounts-page-account-recent-content-type" },
              class:
                "mt-0.5 inline-flex max-w-full rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] font-medium leading-4 text-zinc-600 dark:bg-zinc-900 dark:text-zinc-300",
            },
            [typeLabel],
          ),
        ],
      ),
      Show({
        when: sourceURL,
        ok() {
          return Button(
            {
              store: new Timeless.ui.ButtonCore({
                variant: "ghost",
                size: "sm",
                onClick(event) {
                  event?.stopPropagation?.();
                  onOpenSource();
                },
              }),
              title: "打开源地址",
            },
            [Icon({ name: "external-link", size: 14 }), "源"],
          );
        },
      }),
    ],
  );
}

function PlatformBadge(account) {
  return View(
    {
      dataset: { t: "home-accounts-page-platform-badge-panel-row-account-platform_name-account-platform_id-未知平台-text" },
      class:
        "inline-flex h-6 shrink-0 items-center rounded-md border border-zinc-200 bg-zinc-50 px-2 text-xs font-medium text-zinc-600 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-300",
      title: account.platform_id || account.platform_name || "平台",
    },
    [account.platform_name || account.platform_id || "未知平台"],
  );
}

function OfficialAccountMessageItem(message, vm$) {
  const coverURL = message.cover_url || "";
  return View(
    {
      dataset: { t: "home-accounts-page-official-account-message-row" },
      class:
        "flex gap-3 border-b border-zinc-100 py-3 last:border-b-0 dark:border-zinc-800",
    },
    [
      View(
        {
          dataset: { t: "home-accounts-page-official-account-message-cover" },
          class:
            "h-[64px] w-[96px] shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
        },
        [
          Show({
            when: coverURL,
            ok() {
              return ProxyImg({
                class: "h-full w-full object-cover",
                src: coverURL,
                alt: message.title || "cover",
                platformId: "wx_official_account",
                contentType: "article",
              });
            },
            else() {
              return View(
                {
                  dataset: { t: "home-accounts-page-official-account-message-cover-empty" },
                  class:
                    "flex h-full w-full items-center justify-center text-zinc-400",
                },
                [Icon({ name: "file-text", size: 22 })],
              );
            },
          }),
        ],
      ),
      View(
        {
          dataset: { t: "home-accounts-page-official-account-message-body" },
          class: "min-w-0 flex-1",
        },
        [
          View(
            {
              dataset: { t: "home-accounts-page-official-account-message-title" },
              class:
                "line-clamp-2 text-sm font-medium text-zinc-950 dark:text-zinc-50",
            },
            [message.title],
          ),
          Show({
            when: message.digest,
            ok() {
              return View(
                {
                  dataset: { t: "home-accounts-page-official-account-message-digest" },
                  class: "mt-1 line-clamp-2 text-xs text-zinc-500",
                },
                [message.digest],
              );
            },
          }),
          View(
            {
              dataset: { t: "home-accounts-page-official-account-message-meta" },
              class:
                "mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-500",
            },
            [
              View(
                {
                  dataset: { t: "home-accounts-page-official-account-message-author-time" },
                  class: "min-w-0 truncate",
                },
                [
                  [message.author, message.publish_time_text]
                    .filter(Boolean)
                    .join(" · ") || "-",
                ],
              ),
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    size: "sm",
                    onClick() {
                      vm$.methods.openOfficialMessage(message);
                    },
                  }),
                },
                [Icon({ name: "external-link", size: 14 }), "打开"],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function OfficialAccountMessagesDialog(vm$) {
  return Show({
    when: vm$.state.officialAccount,
    ok() {
      return View(
        {
          dataset: { t: "home-accounts-page-official-account-messages-dialog" },
          class:
            "fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 p-4",
          onClick() {
            vm$.methods.closeOfficialMessages();
          },
        },
        [
          View(
            {
              dataset: { t: "home-accounts-page-official-account-messages-panel" },
              class:
                "flex max-h-[85vh] w-full max-w-3xl flex-col rounded-lg border border-zinc-200 bg-white shadow-xl dark:border-zinc-800 dark:bg-zinc-950",
              onClick(event) {
                event.stopPropagation();
              },
            },
            [
              View(
                {
                  dataset: { t: "home-accounts-page-official-account-messages-header" },
                  class:
                    "flex items-start justify-between gap-3 border-b border-zinc-200 px-5 py-4 dark:border-zinc-800",
                },
                [
                  View(
                    {
                      dataset: { t: "home-accounts-page-official-account-messages-title" },
                      class: "min-w-0",
                    },
                    [
                      View(
                        {
                          dataset: { t: "home-accounts-page-official-account-messages-title-text" },
                          class:
                            "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                        },
                        [
                          computed(vm$.state.officialAccount, (account) =>
                            account ? `${account.nickname} · 推送列表` : "推送列表",
                          ),
                        ],
                      ),
                      View(
                        {
                          dataset: { t: "home-accounts-page-official-account-messages-subtitle" },
                          class: "mt-1 truncate text-xs text-zinc-500",
                        },
                        [
                          computed(vm$.state.officialAccount, (account) =>
                            account?.external_id || account?.username || "",
                          ),
                        ],
                      ),
                    ],
                  ),
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "ghost",
                        size: "sm",
                        onClick() {
                          vm$.methods.closeOfficialMessages();
                        },
                      }),
                    },
                    [Icon({ name: "x", size: 16 })],
                  ),
                ],
              ),
              View(
                {
                  dataset: { t: "home-accounts-page-official-account-messages-content" },
                  class: "min-h-0 flex-1 overflow-y-auto px-5",
                },
                [
                  Show({
                    when: vm$.state.officialMessagesError,
                    ok() {
                      return View(
                        {
                          dataset: { t: "home-accounts-page-official-account-messages-error" },
                          class:
                            "my-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                        },
                        [vm$.state.officialMessagesError],
                      );
                    },
                  }),
                  Show({
                    when: computed(
                      vm$.state.officialMessages,
                      (messages) => messages.length === 0,
                    ),
                    ok() {
                      return View(
                        {
                          dataset: { t: "home-accounts-page-official-account-messages-empty" },
                          class:
                            "flex h-52 flex-col items-center justify-center gap-3 text-zinc-500",
                        },
                        [
                          Icon({ name: "newspaper", size: 34 }),
                          computed(vm$.state.officialMessagesLoading, (loading) =>
                            loading ? "加载中..." : "暂无推送",
                          ),
                        ],
                      );
                    },
                    else() {
                      return View(
                        {
                          dataset: { t: "home-accounts-page-official-account-messages-list" },
                          class: "py-1",
                        },
                        [
                          For({
                            each: vm$.state.officialMessages,
                            render(message) {
                              return OfficialAccountMessageItem(message, vm$);
                            },
                          }),
                        ],
                      );
                    },
                  }),
                ],
              ),
              View(
                {
                  dataset: { t: "home-accounts-page-official-account-messages-footer" },
                  class:
                    "flex items-center justify-between gap-3 border-t border-zinc-200 px-5 py-4 dark:border-zinc-800",
                },
                [
                  View(
                    {
                      dataset: { t: "home-accounts-page-official-account-messages-count" },
                      class: "text-xs text-zinc-500",
                    },
                    [
                      computed(
                        vm$.state.officialMessages,
                        (messages) => `${messages.length} 条`,
                      ),
                    ],
                  ),
                  Show({
                    when: vm$.state.officialMessagesHasMore,
                    ok() {
                      return Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            size: "sm",
                            onClick() {
                              vm$.methods.loadMoreOfficialMessages();
                            },
                          }),
                        },
                        [
                          computed(vm$.state.officialMessagesLoading, (loading) =>
                            loading ? "加载中..." : "加载更多",
                          ),
                        ],
                      );
                    },
                  }),
                ],
              ),
            ],
          ),
        ],
      );
    },
  });
}

const CONTENT_FILTERS = [
  { value: "with", label: "有关联内容" },
  { value: "all", label: "全部帐号" },
  { value: "without", label: "无关联内容" },
];

function ContentFilterButton(option, vm$) {
  const active_ = computed(
    vm$.state.contentFilter,
    (value) => value === option.value,
  );
  return View(
    {
      dataset: { t: "home-accounts-page-content-filter-button-button-node-option-label" },
      as: "button",
      type: "button",
      class: Timeless.classNames([
        "h-9 whitespace-nowrap rounded-md border px-3 text-sm font-medium transition",
        computed(active_, (active) =>
          active
            ? "border-zinc-900 bg-zinc-900 text-white dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950"
            : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-200 dark:hover:bg-zinc-900",
        ),
      ]),
      onClick() {
        vm$.methods.setContentFilter(option.value);
      },
    },
    [option.label],
  );
}

export default function AccountsPageView(props) {
  const vm$ = AccountsPageModel(props);

  return View(
    {
      dataset: { t: "home-page-accounts-page-root-row-帐号-浏览已保存的平台帐号及最近内容-button-input-content_filters-list-scroll-view" },
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
    },
    [
      View(
        {
          dataset: { t: "home-page-accounts-header-帐号-浏览已保存的平台帐号及最近内容-button-input-content_filters-list" },
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ dataset: { t: "home-page-accounts-row-帐号-浏览已保存的平台帐号及最近内容-button" }, class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({ dataset: { t: "home-page-accounts-帐号-浏览已保存的平台帐号及最近内容" } }, [
              View(
                {
                  dataset: { t: "home-page-accounts-帐号-heading" },
                  class:
                    "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["帐号"],
              ),
              View({ dataset: { t: "home-page-accounts-浏览已保存的平台帐号及最近内容-text" }, class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "浏览已保存的平台帐号及最近内容",
              ]),
            ]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.refresh();
                  },
                }),
              },
              [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
            ),
          ]),
          View(
            { dataset: { t: "home-page-accounts-row-input-content_filters-list" }, class: "mt-4 flex flex-wrap items-center gap-3" },
            [
              View({ dataset: { t: "home-page-accounts-input" }, class: "w-full max-w-md" }, [Input({ store: vm$.ui.keyword })]),
              View({ dataset: { t: "home-page-accounts-row-content_filters-list" }, class: "flex flex-wrap items-center gap-2" }, [
                For({
                  each: CONTENT_FILTERS,
                  render(option) {
                    return ContentFilterButton(option, vm$);
                  },
                }),
              ]),
            ],
          ),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ dataset: { t: "home-page-accounts-stack" }, class: "space-y-3 p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View(
                {
                  dataset: { t: "home-page-accounts-error-card-vm-state-error-text" },
                  class:
                    "rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.error],
              );
            },
          }),
          Show({
            when: computed(vm$.state.filtered, (list) => list.length === 0),
            ok() {
              return View(
                {
                  dataset: { t: "home-page-accounts-row-icon-user-search-加载中-or-暂无帐号" },
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "user-search", size: 36 }),
                  computed(vm$.state.loading, (loading) =>
                    loading ? "加载中..." : "暂无帐号",
                  ),
                ],
              );
            },
            else() {
              return View({ dataset: { t: "home-page-accounts-stack-vm-state-filtered-list" }, class: "space-y-3" }, [
                For({
                  each: vm$.state.filtered,
                  render(account) {
                    return View(
                      {
                        dataset: { t: "home-page-accounts-card-avatar-account-nickname-platform-badge-account-external_id-account-username-个内容-account-content_count-account-medias-length-button" },
                        class:
                          "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
                      },
                      [
                        View({ dataset: { t: "home-page-accounts-row-avatar-account-nickname-platform-badge-account-external_id-account-username-个内容-account-content_count-account-medias-length-button" }, class: "flex items-start gap-4" }, [
                          Avatar(account),
                          View({ dataset: { t: "home-page-accounts-row-account-nickname-platform-badge-account-external_id-account-username-个内容-account-content_count-account-medias-length-button" }, class: "min-w-0 flex-1" }, [
                            View(
                              {
                                dataset: { t: "home-page-accounts-row-account-nickname-platform-badge-account-external_id-account-username-个内容-account-content_count-account-medias-length-button-2" },
                                class:
                                  "flex flex-wrap items-start justify-between gap-3",
                              },
                              [
                                View({ dataset: { t: "home-page-accounts-account-nickname-platform-badge-account-external_id-account-username" }, class: "min-w-0" }, [
                                  View(
                                    {
                                      dataset: { t: "home-page-accounts-row-account-nickname-platform-badge" },
                                      class:
                                        "flex min-w-0 flex-wrap items-center gap-2",
                                    },
                                    [
                                      View(
                                        {
                                          dataset: { t: "home-page-accounts-account-nickname-heading" },
                                          class:
                                            "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                                        },
                                        [account.nickname],
                                      ),
                                      PlatformBadge(account),
                                    ],
                                  ),
                                  View(
                                    {
                                      dataset: { t: "home-page-accounts-account-external_id-account-username-text" },
                                      class:
                                        "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                                    },
                                    [
                                      account.external_id ||
                                        account.username ||
                                        "-",
                                    ],
                                  ),
                                ]),
                                View({ dataset: { t: "home-page-accounts-row-个内容-account-content_count-account-medias-length-button" }, class: "flex items-center gap-2" }, [
                                  View({ dataset: { t: "home-page-accounts-个内容-account-content_count-account-medias-length-text" }, class: "text-xs text-zinc-500" }, [
                                    `${account.content_count || account.medias.length} 个内容`,
                                  ]),
                                  Show({
                                    when: account.is_official_account,
                                    ok() {
                                      return Button(
                                        {
                                          store: new Timeless.ui.ButtonCore({
                                            variant: "outline",
                                            size: "sm",
                                            onClick() {
                                              vm$.methods.openOfficialMessages(account);
                                            },
                                          }),
                                        },
                                        [Icon({ name: "newspaper", size: 14 }), "推送列表"],
                                      );
                                    },
                                  }),
                                  Button(
                                    {
                                      store: new Timeless.ui.ButtonCore({
                                        variant: "outline",
                                        size: "sm",
                                        onClick() {
                                          vm$.methods.synchronize(account);
                                        },
                                      }),
                                    },
                                    ["同步"],
                                  ),
                                ]),
                              ],
                            ),
                            Show({
                              when: account.medias.length > 0,
                              ok() {
                                return View(
                                  {
                                    dataset: { t: "home-page-accounts-grid-list" },
                                    class:
                                      "mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6",
                                  },
                                  [
                                    For({
                                      each: account.medias.slice(0, 24),
                                      render(content) {
                                        return MediaThumb(
                                          content,
                                          () => vm$.methods.openContentFile(content),
                                          () => vm$.methods.openContentSource(content),
                                        );
                                      },
                                    }),
                                  ],
                                );
                              },
                            }),
                          ]),
                        ]),
                      ],
                    );
                  },
                }),
              ]);
            },
          }),
        ]),
      ]),
      OfficialAccountMessagesDialog(vm$),
    ],
  );
}

import { ErrorMessage } from "@/dmui.js";
import { WxChannelsPlayerView } from "@/components/wxchannels.player.js";

import { AccountViewModel } from "./account.model.js";

const account_content_card_media_classes = {
  __proto__: null,
  default: "bg-[radial-gradient(circle_at_82%_18%,_rgb(255_190_65_/_32%),_transparent_42%),_linear-gradient(145deg,_#314251,_#18232e_72%)]",
  answer: "bg-[radial-gradient(circle_at_78%_22%,_rgb(85_183_255_/_28%),_transparent_42%),_linear-gradient(145deg,_#314151,_#17212b_72%)]",
};

const account_content_card_gallery_classes = {
  __proto__: null,
  1: "block aspect-ratio-auto max-h-420px",
  2: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] aspect-ratio-[3_/_2]",
  3: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] aspect-ratio-[4_/_3] [&_.group-account-content-card-gallery-item:first-child]:grid-row-[span_2]",
  4: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] aspect-ratio-[4_/_3]",
};



const ACCOUNT_TEXT_CONTENT_TYPES = new Set(["answer", "webpage", "text"]);

function AccountPageView(props) {
  const vm$ = AccountViewModel(props);

  return View(
    {
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text [.group-content-page&]:h-full [.group-content-page&]:min-h-0 [.group-content-page&]:overflow-hidden [&_>_.dm-pagination]:flex-[0_0_auto] [&_>_.group-content-main]:min-h-0 [&_>_.group-content-main]:flex [&_>_.group-content-main]:flex-col min-w-0 min-h-full font-sans",
      attributes: { n: "account-page" },
      onMounted() {
        console.count("AccountPage onMounted");
        vm$.methods.ready();
      },
    },
    [
      View(
        {
          class: "group-content-toolbar-wrap p-[var(--dm-space-4)_var(--dm-space-4)_0] flex-[0_0_auto]",
          attributes: { n: "account-toolbar-wrap" },
        },
        [AccountPageToolbar({ store: vm$ })],
      ),
      AccountPageBody({ store: vm$ }),
      AccountContentsDrawer({ store: vm$ }),
      AccountPlayerDrawer({ store: vm$ }),
    ],
  );
}

function account_content_duration(value) {
  let seconds = Math.max(0, Number(value) || 0);
  if (seconds > 36000) seconds /= 1000;
  const minutes = Math.floor(seconds / 60);
  const remainder = Math.floor(seconds % 60);
  return `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function account_content_card_height(content, _index, column_width) {
  const width = Math.max(180, Number(column_width) || 240);
  const images = Array.isArray(content && content.preview_images)
    ? content.preview_images
    : [];
  const kind = String((content && content.content_type) || "").toLowerCase();
  if (ACCOUNT_TEXT_CONTENT_TYPES.has(kind)) {
    const title = String(
      (content && (content.title || content.external_id)) || "未命名内容",
    );
    const description = String((content && content.description) || "").trim();
    const characters_per_line = Math.max(12, Math.floor((width - 32) / 14));
    const title_lines = Math.min(
      4,
      Math.max(1, Math.ceil(title.length / characters_per_line)),
    );
    const description_lines = description
      ? Math.min(6, Math.ceil(description.length / characters_per_line))
      : 0;
    return (
      32 +
      title_lines * 24 +
      description_lines * 23 +
      (description_lines > 0 ? 8 : 0)
    );
  }
  let height = 190;
  if (images.length > 0) {
    if (images.length > 1) {
      const ratio = images.length === 2 ? 3 / 2 : 4 / 3;
      height = Math.min(420, width / ratio);
    } else {
      const fallback_ratio = kind === "album" ? 4 / 3 : 16 / 9;
      const ratio = Math.max(
        0.56,
        Math.min(1.9, Number(content.preview_aspect_ratio) || fallback_ratio),
      );
      height = Math.max(170, Math.min(420, width / ratio));
    }
  }
  return height;
}

function AccountContentCoverMeta(props) {
  return View(
    {
      class: "absolute inset-0 z-2 flex flex-col justify-end gap-2 p-[52px_var(--dm-space-3)_var(--dm-space-3)] text-[#fff] bg-[linear-gradient(_to_bottom,_transparent_18%,_rgb(0_0_0_/_10%)_42%,_rgb(0_0_0_/_82%)_100%_)] pointer-events-none",
      attributes: { n: "account-content-card-cover-meta" },
    },
    [
      View(
        {
          class: "min-w-0 flex items-center justify-between gap-2",
          attributes: { n: "account-content-card-cover-kicker" },
        },
        [
          Tag(
            {
              class: "flex-[0_0_auto] text-[#fff] border-[1px_solid_rgb(255_255_255_/_36%)] bg-[rgb(0_0_0_/_34%)] backdrop-filter-[blur(5px)]",
              attributes: { n: "account-content-card-type" },
            },
            [props.typeLabel],
          ),
          View(
            {
              class: "min-w-0 overflow-hidden text-[rgb(255_255_255_/_78%)] text-xs text-ellipsis whitespace-nowrap",
              attributes: { n: "account-content-card-time" },
            },
            [props.publishTime],
          ),
        ],
      ),
      View(
        {
          as: "h3",
          class: "-webkit-box overflow-hidden m-0 text-[#fff] text-base font-semibold leading-[1.5] overflow-wrap-anywhere -webkit-box-orient-vertical text-shadow-[0_1px_3px_rgb(0_0_0_/_52%)] -webkit-line-clamp-4",
          attributes: { n: "account-content-card-title" },
        },
        [props.title],
      ),
    ],
  );
}

function AccountContentMedia(props) {
  const content = props.content;
  const images = Array.isArray(content.preview_images)
    ? content.preview_images.filter(Boolean)
    : [];
  const kind = String(content.content_type || "").toLowerCase();
  if (images.length === 0) {
    return View(
      {
        class: `relative w-full overflow-hidden min-h-190px text-[rgb(255_255_255_/_74%)] ${account_content_card_media_classes[kind] || account_content_card_media_classes.default}`,
        attributes: { n: "account-content-card-media-empty" },
      },
      [
        View(
          {
            class: "absolute top-4 right-4 opacity-[0.32]",
            attributes: { "aria-hidden": "true" },
          },
          [
            Timeless.Icon({
              name: kind === "answer" ? "message-circle" : "file-text",
              size: 42,
            }),
          ],
        ),
        props.coverMeta,
      ],
    );
  }

  if (kind === "album" || images.length > 1) {
    const visible_images = images.slice(0, 4);
    const single_image_ratio = Math.max(
      0.56,
      Math.min(1.9, Number(content.preview_aspect_ratio) || 4 / 3),
    );
    return View(
      {
        class: `relative w-full overflow-hidden bg-content-panel-soft gap-2px ${account_content_card_gallery_classes[Math.min(visible_images.length, 4)] || ""}`,
        style:
          visible_images.length === 1
            ? { "aspect-ratio": String(single_image_ratio) }
            : undefined,
        attributes: { n: "account-content-card-gallery" },
      },
      [
        ...visible_images.map((image_url, index) =>
          View(
            {
              class: "group-account-content-card-gallery-item relative min-w-0 min-h-0 overflow-hidden",
              attributes: { n: "account-content-card-gallery-item" },
            },
            [
              LazyImg({
                class: "w-full h-full block object-cover",
                src: image_url,
                alt: `${content.title || "图集"} ${index + 1}`,
                attributes: {
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
              }),
              index === visible_images.length - 1 && images.length > 4
                ? View(
                    {
                      class: "absolute inset-0 grid place-items-center text-[#fff] text-lg font-semibold bg-[rgb(0_0_0_/_48%)]",
                      attributes: { n: "account-content-card-gallery-more" },
                    },
                    [`+${images.length - 4}`],
                  )
                : null,
            ].filter(Boolean),
          ),
        ),
        props.coverMeta,
      ],
    );
  }

  return View(
    {
      class: "relative w-full overflow-hidden bg-content-panel-soft max-h-420px",
      style: {
        "aspect-ratio":
          Number(content.preview_aspect_ratio) > 0
            ? String(content.preview_aspect_ratio)
            : "16 / 9",
      },
      attributes: { n: "account-content-card-poster" },
    },
    [
      LazyImg({
        class: "w-full h-full block object-cover",
        src: images[0],
        alt: content.title || "内容封面",
        attributes: {
          loading: "lazy",
          referrerpolicy: "no-referrer",
        },
      }),
      kind === "video"
        ? View(
            {
              class: "absolute top-[50%] left-[50%] w-44px h-44px grid place-items-center text-[#fff] border-[1px_solid_rgb(255_255_255_/_46%)] rounded-[50%] bg-[rgb(0_0_0_/_44%)] backdrop-filter-[blur(6px)] transform-[translate(-50%,_-50%)]",
              attributes: {
                n: "account-content-card-play",
                "aria-hidden": "true",
              },
            },
            [Timeless.Icon({ name: "play", size: 20 })],
          )
        : null,
      kind === "video" && Number(content.duration) > 0
        ? View(
            {
              class: "absolute top-2 right-2 bottom-auto p-[2px_6px] text-[#fff] text-xs font-variant-numeric-tabular-nums rounded-sm bg-[rgb(0_0_0_/_62%)]",
              attributes: { n: "account-content-card-duration" },
            },
            [account_content_duration(content.duration)],
          )
        : null,
      props.coverMeta,
    ].filter(Boolean),
  );
}

function AccountContentCard(props) {
  const vm$ = props.store;
  const content = props.content;
  const source_url = String(content.url || "").trim();
  const title = content.title || content.external_id || "未命名内容";
  const description = String(content.description || "").trim();
  const kind = String(content.content_type || "unknown").toLowerCase();
  const text_only = ACCOUNT_TEXT_CONTENT_TYPES.has(kind);
  const cover_meta = text_only
    ? null
    : AccountContentCoverMeta({
        title,
        typeLabel: vm$.methods.contentTypeLabel(
          content.content_type,
          content.content_subtype,
        ),
        publishTime: vm$.methods.formatTime(content.publish_time),
      });
  return Card(
    {
      as: source_url ? "button" : "article",
      class: [
        "w-full min-w-0 overflow-hidden p-0 text-inherit text-left border-[var(--dm-border-width)_solid_var(--dm-content-border)] bg-content-panel",
        text_only ? "flex flex-col gap-2 p-4" : "",
        source_url ? "cursor-pointer hover:border-color-color-primary-border hover:shadow-sm hover:outline-0 hover:transform-[translateY(-1px)] focus-visible:border-color-color-primary-border focus-visible:shadow-sm focus-visible:outline-0 focus-visible:transform-[translateY(-1px)]" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        n: "account-content-card",
        type: source_url ? "button" : undefined,
        title: source_url
          ? kind === "video" && content.decode_key
            ? "播放视频"
            : "打开原内容"
          : undefined,
      },
      onClick() {
        if (source_url) vm$.methods.openContent(content);
      },
    },
    text_only
      ? [
          View(
            {
              as: "h3",
              class: "-webkit-box overflow-hidden m-0 text-content-text text-base font-semibold leading-[1.5] overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-4",
              attributes: { n: "account-content-card-text-title" },
            },
            [title],
          ),
          description
            ? View(
                {
                  as: "p",
                  class: "-webkit-box overflow-hidden m-0 text-content-muted text-sm leading-[1.65] overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-6",
                  attributes: { n: "account-content-card-description" },
                },
                [description],
              )
            : null,
        ].filter(Boolean)
      : [AccountContentMedia({ content, coverMeta: cover_meta })],
  );
}

function AccountContentPlayer(props) {
  const vm$ = props.store;
  const content = props.content;
  return View(
    {
      class: "min-h-0 flex flex-[1_1_0] flex-col gap-3",
      attributes: {
        n: "account-content-player",
        role: "region",
        "aria-label": "账号视频播放",
      },
    },
    [
      View(
        {
          class: "flex items-center justify-between gap-3",
          attributes: { n: "account-content-player-header" },
        },
        [
          View(
            {
              as: "h3",
              class: "min-w-0 overflow-hidden m-0 text-content-text text-base font-semibold text-ellipsis whitespace-nowrap",
              attributes: { n: "account-content-player-title" },
            },
            [content.title || content.external_id || "未命名视频"],
          ),
          View(
            {
              class: "flex flex-[0_0_auto] items-center gap-2",
              attributes: { n: "account-content-player-actions" },
            },
            [
              AccountPageActionButton({
                name: "account-content-player-download-action",
                store: vm$.ui.btn_download_player$,
                icon: "download",
                label: "下载",
                title: "创建下载任务",
              }),
              View(
                {
                  as: "button",
                  class: "inline-flex flex-[0_0_auto] items-center gap-1 text-content-muted text-sm border-0 bg-transparent cursor-pointer hover:text-color-primary hover:outline-none focus-visible:text-color-primary focus-visible:outline-none",
                  attributes: {
                    n: "account-content-player-close-action",
                    type: "button",
                    title: "关闭播放器",
                  },
                  onClick() {
                    vm$.methods.closeContentPlayer();
                  },
                },
                [
                  Timeless.Icon({
                    name: "x",
                    size: 16,
                    attributes: { n: "account-content-player-close-icon" },
                  }),
                  "关闭",
                ],
              ),
            ],
          ),
        ],
      ),
      Show({
        when: vm$.state.drawer_player_download_success,
        ok() {
          return View(
            {
              class: "text-sm text-color-success",
              attributes: {
                n: "account-content-player-download-success",
                role: "status",
              },
            },
            [vm$.state.drawer_player_download_success],
          );
        },
      }),

      WxChannelsPlayerView({
        tip: vm$.methods.tip,
        url: content.url,
        decodeKey: content.decode_key,
        autoplay: true,
        class: "flex flex-[1_1_0] flex-col justify-center [&_.group-wxchannels-video]:max-h-[min(72vh,_calc(100vh_-_220px))]",
        nodeName: "account-content-player-media",
        ariaLabel: content.title || "账号视频",
      }),
    ],
  );
}

function AccountContentState(props) {
  const role = props.error ? "alert" : "status";
  return View(
    {
      class: `w-full min-h-360px flex flex-1 flex-col gap-3 items-center justify-center p-6 text-content-muted text-center${props.error ? " is-error" : ""}`,
      attributes: { n: "account-content-state", role },
    },
    [
      props.loading
        ? BrandLoading({
            size: 112,
            label: props.title || "正在加载账号内容",
            decorative: true,
            name: "account-content-loading-symbol",
          })
        : props.error
          ? BrandError({ size: 116, name: "account-content-error-symbol" })
          : BrandEmpty({ size: 116, name: "account-content-empty-symbol" }),
      View({ as: "h3", class: "m-0 text-content-text text-base font-semibold" }, [props.title]),
      props.description
        ? View({ as: "p", class: "max-w-440px m-0 text-sm" }, [
            props.description,
          ])
        : null,
      props.action || null,
    ].filter(Boolean),
  );
}

function AccountContentsCollection(props) {
  const vm$ = props.store;
  return View(
    {
      class: "min-h-0 flex flex-[1_1_0] flex-col overflow-hidden",
      attributes: { n: "account-content-collection" },
    },
    [
      Show({
        when: computed(
          vm$.state.drawer_status,
          (status) => status === "initial",
        ),
        ok() {
          return AccountContentState({
            loading: true,
            title: "正在加载账号内容",
          });
        },
      }),
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "error"),
        ok() {
          return AccountContentState({
            error: true,
            title: "内容加载失败",

            action: AccountPageActionButton({
              name: "account-content-retry-action",
              store: vm$.ui.btn_drawer_retry$,
              icon: "rotate-ccw",
              label: "重试",
            }),
          });
        },
      }),
      Show({
        when: computed(vm$.state.drawer_status, (status) => status === "empty"),
        ok() {
          return AccountContentState({
            title: "暂无内容",
            description: vm$.state.drawer_empty_description,
          });
        },
      }),
      Show({
        when: computed(
          vm$.state.drawer_status,
          (status) => status === "normal",
        ),
        ok() {
          return Waterfall({
            class: "w-full min-h-0 flex-[1_1_0] p-[0_var(--dm-space-1)_var(--dm-space-2)_0] overflow-x-hidden overflow-y-auto overscroll-behavior-contain",
            attributes: {
              n: "account-content-waterfall",
              "aria-label": "账号内容列表",
            },
            each: vm$.state.drawer_contents,
            key: "id",
            columns: 4,
            gap: 14,
            size: 8,
            buffer: 3,
            itemHeight: account_content_card_height,
            render(content) {
              return AccountContentCard({ store: vm$, content });
            },
            footer: [
              Show({
                when: computed(vm$.state.drawer_loading_more, (loading) =>
                  Boolean(loading),
                ),
                ok() {
                  return View(
                    {
                      class: "flex items-center justify-center gap-2 text-content-muted text-sm [&.is-error]:text-color-danger [&.is-end::before]:w-38px [&.is-end::before]:h-1px [&.is-end::before]:content-[\"\"] [&.is-end::before]:bg-content-border [&.is-end::after]:w-38px [&.is-end::after]:h-1px [&.is-end::after]:content-[\"\"] [&.is-end::after]:bg-content-border",
                      attributes: {
                        n: "account-content-load-more-status",
                        role: "status",
                      },
                    },
                    [
                      BrandLoading({ size: 30, decorative: true }),
                      "正在加载更多…",
                    ],
                  );
                },
              }),
              Show({
                when: computed(vm$.state.drawer_more_error, (error) =>
                  Boolean(error),
                ),
                ok() {
                  return ErrorMessage({ tip: vm$.methods.tip, error: vm$.state.drawer_more_error });
                },
              }),
              Show({
                when: Timeless.combine(
                  {
                    marker: vm$.state.drawer_next_marker,
                    loading: vm$.state.drawer_loading_more,
                    error: vm$.state.drawer_more_error,
                  },
                  ({ marker, loading, error }) => !marker && !loading && !error,
                ),
                ok() {
                  return View(
                    {
                      class: "flex items-center justify-center gap-2 text-content-muted text-sm [&.is-error]:text-color-danger [&.is-end::before]:w-38px [&.is-end::before]:h-1px [&.is-end::before]:content-[\"\"] [&.is-end::before]:bg-content-border [&.is-end::after]:w-38px [&.is-end::after]:h-1px [&.is-end::after]:content-[\"\"] [&.is-end::after]:bg-content-border is-end",
                      attributes: {
                        n: "account-content-no-more",
                        role: "status",
                      },
                    },
                    ["没有更多了"],
                  );
                },
              }),
            ],
            onReachBottom() {
              vm$.methods.loadMoreAccountContents();
            },
          });
        },
      }),
    ],
  );
}

function AccountContentsDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.account_contents_drawer$,
      class: "dm-drawer--wide account-content-drawer",
      attributes: { n: "account-content-drawer" },
    },
    () => [
      View(
        {
          class: "dm-drawer-body flex flex-col gap-4 min-h-0 overflow-hidden",
          attributes: { n: "account-content-drawer-body" },
        },
        [
          Tabs({
            class: "flex-[0_0_auto] overflow-x-auto",
            attributes: { n: "account-content-tabs" },
            each: vm$.state.drawer_tabs,
            key: "scope",
            render(tab) {
              return AccountContentTab({ store: vm$, tab });
            },
          }),
          AccountContentsCollection({ store: vm$ }),
        ],
      ),
    ],
  );
}

function AccountPlayerDrawer(props) {
  const vm$ = props.store;
  return Drawer(
    {
      store: vm$.ui.player_drawer$,
      class: "dm-drawer--wide account-player-drawer",
      attributes: { n: "account-player-drawer" },
    },
    () => [
      View(
        {
          class: "dm-drawer-body account-player-drawer-body",
          attributes: { n: "account-player-drawer-body" },
        },
        [
          Show({
            when: vm$.state.drawer_player_content,
            ok() {
              return AccountContentPlayer({
                store: vm$,
                content: vm$.state.drawer_player_content.value,
              });
            },
          }),
        ],
      ),
    ],
  );
}

function AccountContentTab(props) {
  const vm$ = props.store;
  const tab = props.tab;
  const scope = String(tab.scope || "");
  const content_types = Array.isArray(tab.content_types)
    ? tab.content_types.join("、")
    : "";
  return Tab(
    {
      selected: computed(
        vm$.state.drawer_scope,
        (active_scope) => active_scope === scope,
      ),
      attributes: {
        n: `account-content-tab-${scope}`,
        title: content_types ? `内容类型：${content_types}` : tab.name,
      },
      onClick() {
        return vm$.methods.selectHomeTab(tab);
      },
    },
    [tab.name || scope],
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
      class: "group-content-toolbar flex items-center gap-3 max-760:flex-wrap justify-start",
      attributes: { n: "account-toolbar", role: "search" },
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.search();
      },
    },
    [
      View(
        {
          class:
            "min-w-0 flex-[1_1_0] max-760:flex-[1_1_100%] max-520:flex-wrap account-filter-fields dm-flex dm-items-center dm-gap-2",
        },
        [
          View({}, [
            PlatformSelect({
              store: vm$.ui.select_platform$,
              attributes: {
                "aria-label": "按平台筛选账号",
              },
            }),
          ]),
          View(
            {
              class: "min-w-0 flex-[1_1_0] max-520:basis-full",
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
                  type: "search",
                  autocomplete: "off",
                  "aria-label": "搜索账号昵称或 ID",
                },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "group-content-filter-actions flex-[0_0_auto] max-760:ml-auto max-520:w-full max-520:ml-0 max-520:[&_>_.dm-button]:flex-[1_1_0] dm-flex dm-items-center dm-gap-2",
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
  return View(
    {
      class: "relative w-46px h-46px flex-[0_0_auto] overflow-hidden rounded-avatar bg-content-panel-soft max-520:w-40px max-520:h-40px",
      attributes: { n: "account-avatar" },
    },
    [
      Show({
        when: account.avatar_url,
        ok() {
          return LazyImg({
            class: "absolute inset-0 w-full h-full block object-cover",
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
    ],
  );
}

function AccountPlatform(props) {
  const vm$ = props.store;
  const account = props.account;
  return PlatformTag({
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
    View(
      {
        class: "min-w-0 flex items-start flex-col gap-1 max-520:max-w-full",
        attributes: { n: "account-details" },
      },
      [
        View(
          {
            class: "min-w-0 overflow-hidden text-content-text text-base font-620 text-ellipsis whitespace-nowrap max-520:max-w-full",
            attributes: { n: "account-name" },
          },
          [account.nickname],
        ),
        View({ class: "min-w-0 max-w-full flex items-center gap-2", attributes: { n: "account-meta" } }, [
          View(
            {
              type: "button",
              class: computed(copied_, (copied) =>
                copied
                  ? "grid flex-[0_0_auto] place-items-center p-0 text-content-muted leading-1 bg-transparent border-0 cursor-pointer [&:hover:not(:disabled)]:text-color-primary [&.is-copied]:text-color-success [&.is-copied:hover]:text-color-success dm-focus-ring is-copied"
                  : "grid flex-[0_0_auto] place-items-center p-0 text-content-muted leading-1 bg-transparent border-0 cursor-pointer [&:hover:not(:disabled)]:text-color-primary [&.is-copied]:text-color-success [&.is-copied:hover]:text-color-success dm-focus-ring",
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
              class: "min-w-0 overflow-hidden text-content-muted font-[var(--dm-font-family-mono,_monospace)] text-caption text-ellipsis whitespace-nowrap",
              attributes: {
                n: "account-id",
                title: account.id || "",
              },
            },
            [account.id || "-"],
          ),
        ]),
        AccountPlatform({ store: vm$, account }),
      ],
    ),
  ];
}

function AccountSkeletonRow() {
  return View(
    {
      class:
        "dm-table-row dm-grid dm-items-center [&:not(.dm-table-header)]:min-h-72px max-520:grid-cols-[minmax(0,_1fr)_58px_96px] max-520:gap-2 pointer-events-none",
      attributes: { n: "account-table-skeleton-row", role: "row" },
    },
    [
      View(
        {
          class: "dm-table-cell min-w-0 flex items-center gap-3 max-520:gap-2",
          attributes: { n: "account-table-skeleton-account", role: "cell" },
        },
        [
          View({
            class: "relative w-46px h-46px flex-[0_0_auto] overflow-hidden rounded-avatar bg-content-panel-soft max-520:w-40px max-520:h-40px relative overflow-hidden [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite]",
            attributes: { n: "account-table-skeleton-avatar" },
          }),
          View(
            {
              class: "w-[min(320px,_60%)]",
              attributes: { n: "account-table-skeleton-details" },
            },
            [
              View({
                class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(180px,_80%)]",
                attributes: { n: "account-table-skeleton-name" },
              }),
              View({
                class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
                attributes: { n: "account-table-skeleton-id" },
              }),
            ],
          ),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "account-table-skeleton-count", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
            attributes: { n: "account-table-skeleton-count-value" },
          }),
        ],
      ),
      View(
        {
          class: "dm-table-cell",
          attributes: { n: "account-table-skeleton-time", role: "cell" },
        },
        [
          View({
            class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]",
            attributes: { n: "account-table-skeleton-time-value" },
          }),
        ],
      ),
    ],
  );
}

function AccountPageBody(props) {
  const vm$ = props.store;
  return Table({
    name: "account-table",
    containerClass: "group-content-main flex-[1_1_auto] min-w-0 p-4 ",
    containerAttributes: { n: "account-page-main" },
    panelAttributes: { n: "account-table-panel" },
    headerClass: "[&:not(.dm-table-header)]:min-h-72px max-520:grid-cols-[minmax(0,_1fr)_58px_96px] max-520:gap-2",
    columns: [
      {
        name: "account",
        title: "账号",
        cellClass: "min-w-0 flex items-center gap-3 max-520:gap-2",
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
        cellClass: "flex items-center gap-1 text-content-muted text-sm font-variant-numeric-tabular-nums whitespace-nowrap max-520:text-xs max-520:whitespace-normal",
        render(account) {
          return [vm$.methods.formatContentCount(account.content_count)];
        },
      },
      {
        name: "added",
        title: "添加时间",
        width: 180,
        cellClass: "flex items-center gap-1 text-content-muted text-sm font-variant-numeric-tabular-nums whitespace-nowrap max-520:text-xs max-520:whitespace-normal",
        render(account) {
          return [vm$.methods.formatTime(account.created_at)];
        },
      },
    ],
    rows: vm$.state.accounts,
    pagination: {
      class: " dm-px-4",
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
    rowClass: "[&:not(.dm-table-header)]:min-h-72px max-520:grid-cols-[minmax(0,_1fr)_58px_96px] max-520:gap-2",
    skeletonCount: 8,
    renderSkeletonRow: AccountSkeletonRow,
    onRow(account) {
      return {
        class: "cursor-pointer hover:bg-content-panel-soft hover:shadow-[inset_var(--dm-border-width-strong)_0_var(--dm-color-primary-fill)] hover:outline-0 focus-visible:bg-content-panel-soft focus-visible:shadow-[inset_var(--dm-border-width-strong)_0_var(--dm-color-primary-fill)] focus-visible:outline-0",
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
    emptyTitle: Timeless.combine(
      {
        keyword: vm$.state.keyword,
        platform: vm$.state.platform_id,
      },
      ({ keyword, platform }) =>
        String(keyword || "").trim() || String(platform || "").trim()
          ? "没有匹配的账号"
          : "暂无账号",
    ),
    emptyDescription: Timeless.combine(
      {
        keyword: vm$.state.keyword,
        platform: vm$.state.platform_id,
      },
      ({ keyword, platform }) =>
        String(keyword || "").trim() || String(platform || "").trim()
          ? "请尝试其他平台、昵称或账号 ID"
          : "还没有记录任何账号",
    ),
  });
}

export default AccountPageView;

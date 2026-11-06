import { ErrorMessage } from "@/dmui.js";
import { ThirdPartyDownloaderPanel } from "@/third-party-downloader.js";
import {
  FileTreeView,
  resource_file_icon,
  format_file_size,
  count_tree_children,
} from "@/components/tree.js";

import { ScraperPageViewModel } from "./scraper.model.js";

const home_typed_detail_classes = {
  __proto__: null,
  "video": "icon-tone-violet ",
  "live": "icon-tone-violet ",
  "article": "icon-tone-sky ",
  "album": "icon-tone-amber ",
};

const {
  Tag,
  PlatformTag,
  Tooltip,
} = window;

const task_overwrite_actions = [
  {
    value: "overwrite",
    label: "覆盖",
    description: "删除已有任务和文件后重新创建",
    icon: "refresh-cw",
  },
  {
    value: "duplicate",
    label: "重复下载",
    description: "保留已有任务，再创建一份",
    icon: "copy",
  },
];

function ScraperPageView(props) {
  const page_props = props || {};
  const vm$ = ScraperPageViewModel({
    ...page_props,
    downloader: window.dl$,
  });
  let focus_input_frame = null;
  let unlisten_route_change = null;

  function focus_url_input() {
    if (focus_input_frame !== null) {
      cancelAnimationFrame(focus_input_frame);
    }
    focus_input_frame = requestAnimationFrame(() => {
      focus_input_frame = null;
      vm$.ui.input_url$.focus();
    });
  }

  return View(
    {
      class: "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page",
      onMounted() {
        vm$.methods.connectProgress();
        unlisten_route_change = props.history.onRouteChange(({ view }) => {
          if (view === props.view) {
            focus_url_input();
          }
        });
        focus_url_input();
      },
      onUnmounted() {
        if (typeof unlisten_route_change === "function") {
          unlisten_route_change();
          unlisten_route_change = null;
        }
        if (focus_input_frame !== null) {
          cancelAnimationFrame(focus_input_frame);
          focus_input_frame = null;
        }
        vm$.methods.dispose();
      },
    },
    [
      View({ class: "min-w-0 flex-auto p-4 group-content-main home-main " }, [
        View({ class: "flex flex-col gap-6" }, [
          View(
            {
              class: "flex relative min-w-0 flex-col p-6 max-520:p-2.5 gap-5 border border-solid border-color-content rounded-card bg-content-panel shadow-md dm-panel tabbed-panel",
              attributes: { "aria-busy": vm$.state.busy },
            },
            [
              View({ class: "flex max-520:flex-wrap items-start justify-between gap-3" }, [
                View({ class: "grid w-control-height-lg h-control-height-lg flex-none place-items-center rounded-content bg-color-primary-fill text-color-on-primary shadow-signal" }, [
                  Timeless.Icon({ name: "inbox", size: 20 }),
                ]),
                View({ class: "min-w-0 flex-auto" }, [
                  View({ class: "mb-1 text-caption font-semibold tracking-wide text-transform-uppercase text-color-primary" }, ["新建归档任务"]),
                  View({ class: "font-display text-xl font-semibold leading-tight tracking-tight text-content-text" }, [
                    "粘贴内容链接，开始解析",
                  ]),
                  View({ class: "mt-1.5 text-sm leading-base text-content-muted" }, [
                    "识别内容、作者与媒体资源，并生成可追踪的下载任务。",
                  ]),
                ]),
                View({ class: "flex max-520:w-full items-center ml-auto" }, [
                  ScraperPlatformStatus({ store: vm$ }),
                ]),
              ]),
              ScraperPageForm({ store: vm$ }),
              ScraperFetchProgress({ store: vm$ }),
            ],
          ),

          ScraperFetchedRawContent({ store: vm$ }),
          ScraperPageResult({ store: vm$ }),
        ]),
      ]),
      Show({
        when: vm$.state.build_download_task_result_visible,
        ok() {
          return ScraperResultActionBar({ store: vm$ });
        },
      }),
      ScraperCacheContentDialog({ store: vm$ }),
      TaskOverwriteConfirmDialog({ store: vm$ }),
      ThirdPartyDownloaderPanel({
        store: vm$.models.third_party_downloader,
      }),
    ],
  );
}

function TaskOverwriteConfirmDialog(props) {
  const { store: vm$ } = props;
  const action_ = vm$.state.download_overwrite_action;
  const conflict_ = vm$.state.download_overwrite_conflict;
  const processing_ = vm$.state.download_overwrite_processing;

  return Dialog(
    {
      store: vm$.ui.task_overwrite_confirm_dialog$,
      zIndex: 10000,
      class: "dm-dialog--form",
      okText: "确定",
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["下载任务已存在"]),
        DialogDescription({}, [
          "请选择覆盖已有任务，或者保留已有任务并重复下载。",
        ]),
      ]),
      DialogBody({ class: "dm-grid dm-gap-3" }, [
        View(
          {
            class: "dm-notice-row",
            attributes: { n: "download-overwrite-conflict" },
          },
          [
            Timeless.Icon({
              name: "circle-alert",
              size: 18,
              attributes: { n: "download-overwrite-conflict-icon" },
            }),
            View(
              {
                class: "dm-flex-1 dm-text-sm dm-font-semibold dm-truncate",
                attributes: {
                  n: "download-overwrite-conflict-name",
                  title: computed(conflict_, (conflict) => conflict.name || ""),
                },
              },
              [computed(conflict_, (conflict) => conflict.name || "")],
            ),
          ],
        ),
        View(
          {
            role: "radiogroup",
            class: "dm-choice-list",
            attributes: { n: "download-overwrite-actions" },
          },
          [
            For({
              each: task_overwrite_actions,
              render(item) {
                const row_state_ = combine(
                  { action: action_, processing: processing_ },
                  (state) => ({
                    selected: state.action === item.value,
                    processing: Boolean(state.processing),
                  }),
                );
                return View(
                  {
                    role: "radio",
                    tabIndex: "0",
                    attributes: {
                      n: `download-overwrite-${item.value}`,
                      "aria-checked": computed(row_state_, (state) =>
                        state.selected ? "true" : "false",
                      ),
                      "aria-disabled": computed(row_state_, (state) =>
                        state.processing ? "true" : undefined,
                      ),
                    },
                    class: computed(row_state_, (state) =>
                      [
                        "dm-choice-row",
                        state.selected ? "is-selected" : "",
                        state.processing ? "is-processing" : "",
                      ]
                        .filter(Boolean)
                        .join(" "),
                    ),
                    onClick() {
                      vm$.methods.setTaskOverwriteAction(item.value);
                    },
                    onKeyDown(event) {
                      if (event.key === " " || event.key === "Enter") {
                        event.preventDefault();
                        vm$.methods.setTaskOverwriteAction(item.value);
                      }
                    },
                  },
                  [
                    View(
                      {
                        class: "dm-choice-row__icon",
                        attributes: {
                          n: `download-overwrite-${item.value}-icon`,
                        },
                      },
                      [Timeless.Icon({ name: item.icon, size: 16 })],
                    ),
                    View(
                      {
                        class: "dm-min-w-0 dm-flex-1",
                        attributes: {
                          n: `download-overwrite-${item.value}-content`,
                        },
                      },
                      [
                        View(
                          {
                            class: "dm-choice-row__title",
                            attributes: {
                              n: `download-overwrite-${item.value}-title`,
                            },
                          },
                          [item.label],
                        ),
                        View(
                          {
                            class: "dm-choice-row__description",
                            attributes: {
                              n: `download-overwrite-${item.value}-description`,
                            },
                          },
                          [item.description],
                        ),
                      ],
                    ),
                    Show({
                      when: computed(row_state_, (state) => state.selected),
                      ok() {
                        return Timeless.Icon({ name: "check", size: 18 });
                      },
                    }),
                  ],
                );
              },
            }),
          ],
        ),
        Show({
          when: processing_,
          ok() {
            return View(
              {
                role: "status",
                class: "dm-status-inline",
                attributes: { n: "download-overwrite-processing" },
              },
              [
                View({
                  class: "dm-spinner",
                  attributes: { "aria-hidden": "true" },
                }),
                "正在重新创建下载任务...",
              ],
            );
          },
        }),
      ]),
    ],
  );
}

/**
 *
 * @param {object} props
 * @param {ReturnType<ScraperPageViewModel>} props.store
 * @returns
 */
function ScraperPageForm(props) {
  const vm$ = props.store;
  return View(
    {
      type: "form",
      class: "grid grid-cols-content-auto max-520:grid-cols-1 pt-1 gap-3",
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.submit();
      },
    },
    [
      View(
        {
          class: "w-full max-w-none",
          attributes: { n: "scraper-url-field" },
        },
        [
          Input({
            store: vm$.ui.input_url$,
            rootClass: "dm-input--lg",
            rootAttributes: { n: "scraper-url-control" },
            prefix: Timeless.Icon({
              name: "search",
              size: 16,
              attributes: { n: "scraper-url-icon" },
            }),
            attributes: {
              n: "scraper-url-input",
              type: "text",
              name: "url",
              autocomplete: "off",
              autofocus: "autofocus",
              inputmode: "url",
              spellcheck: "false",
              "aria-label": "内容 URL",
            },
          }),
        ],
      ),
      View({ class: "flex max-520:w-full items-center justify-end gap-2 form-actions " }, [
        Show({
          when: vm$.state.loading,
          ok() {
            return Show({
              when: vm$.state.interrupt_visible,
              ok() {
                return Button(
                  {
                    store: vm$.ui.btn_interrupt_fetch$,
                    attributes: {
                      n: "scraper-interrupt-action",
                      type: "button",
                    },
                  },
                  [
                    Timeless.Icon({ name: "square", size: 14 }),
                    computed(vm$.state.interrupt_loading, (loading) =>
                      loading ? "中断中" : "中断",
                    ),
                  ],
                );
              },
            });
          },
          else() {
            return View({ class: "flex max-520:w-full max-520:flex-wrap items-center justify-end gap-2 form-actions " }, [
              Button(
                {
                  store: vm$.ui.btn_submit$,
                  attributes: {
                    n: "scraper-submit-action",
                    type: "button",
                  },
                },
                [
                  vm$.state.submit_button_text,
                  Timeless.Icon({ name: "arrow-right", size: 15 }),
                ],
              ),
            ]);
          },
        }),
      ]),
    ],
  );
}

function ScraperContentCover(props) {
  const content = props.content;
  return Show({
    when: content.cover_url,
    ok() {
      return View({ class: "relative min-h-260px max-620:min-h-0 max-620:aspect-video overflow-hidden border-r border-r-solid border-r-color-content max-620:border-right-0 max-620:border-b max-620:border-b-solid max-620:border-b-color-content bg-content-panel-soft" }, [
        // View({ class: "home-content-cover-fallback" }, [
        //   Timeless.Icon({ name: "file", size: 34 }),
        //   content.content_type_name,
        // ]),
        LazyImg({
          class: "block absolute inset-0 w-full h-full object-cover",
          src: content.cover_url,
          alt: content.title,
          attributes: {
            loading: "lazy",
            referrerpolicy: "no-referrer",
          },
        }),
      ]);
    },
  });
}

function ScraperDecodeHTMLText(value) {
  const text = String(value === undefined || value === null ? "" : value);
  if (text.indexOf("&") === -1) {
    return text;
  }
  const entities = {
    amp: "&",
    lt: "<",
    gt: ">",
    quot: '"',
    apos: "'",
    nbsp: " ",
  };
  return text.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (match, entity) => {
    const key = String(entity || "").toLowerCase();
    if (Object.prototype.hasOwnProperty.call(entities, key)) {
      return entities[key];
    }
    if (key.charAt(0) !== "#") {
      return match;
    }
    const code_point =
      key.charAt(1) === "x"
        ? parseInt(key.slice(2), 16)
        : parseInt(key.slice(1), 10);
    if (!Number.isFinite(code_point)) {
      return match;
    }
    try {
      return String.fromCodePoint(code_point);
    } catch (ignore) {
      return match;
    }
  });
}

function ScraperContentCard(props) {
  const vm$ = props.store;
  const content = vm$.state.content;
  const account = vm$.state.account;
  const description = computed(content.description, ScraperDecodeHTMLText);
  return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow dm-panel home-content-card" }, [
    View({ class: "flex min-h-58px max-520:flex-wrap items-center justify-between py-2.5 px-3.5 max-520:pr-3 max-520:pl-3 gap-3 border-b border-b-solid border-b-color-content bg-content-panel card-heading" }, [
      View({ class: "flex min-w-0 items-center gap-2.5" }, [
        View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
          Timeless.Icon({ name: "file", size: 17 }),
        ]),
        View({}, [
          View({ class: "text-caption font-750 leading-1.2 tracking-0 text-content-muted" }, ["CONTENT"]),
          View({ class: "mt-0.5 text-ui font-680 leading-1.2" }, ["内容"]),
        ]),
      ]),
    ]),
    View(
      {
        class: computed(content.cover_url, (cover_url) =>
          cover_url
            ? "grid min-h-0 max-620:grid-cols-1 grid-cols-third-main split-card"
            : "grid min-h-0 max-620:grid-cols-1 grid-cols-third-main split-card is-coverless",
        ),
      },
      [
        ScraperContentCover({ content }),
        View({ class: "flex min-w-0 flex-col items-start p-4.5 max-520:p-4" }, [
          View(
            { class: "flex flex-wrap items-center gap-1.5", attributes: { n: "scraper-content-tags" } },
            [
              PlatformTag({
                name: "scraper-content-platform",
                favicon: content.platform_favicon,
                label: content.platform_name,
              }),
              Tag({ name: "scraper-content-type" }, [
                content.content_type_name,
              ]),
            ],
          ),
          View(
            {
              class: "-webkit-box w-full mt-3.5 overflow-hidden text-xl font-720 leading-1.42 overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2 text-content-text",
              attributes: { title: content.title },
            },
            [content.title],
          ),
          View({ class: "flex items-center mt-3 gap-1.5 text-sm text-content-muted" }, [
            Timeless.Icon({ name: "clock3", size: 14 }),
            content.publish_time_text,
          ]),
          Show({
            when: content.show_description,
            ok() {
              return View({ class: "-webkit-box w-full mt-2 overflow-hidden text-sm leading-1.6 overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2 text-content-muted" }, [description]);
            },
          }),
          ScraperContentAccount({ account }),
        ]),
      ],
    ),
  ]);
}

function ScraperContentRelationNode(props) {
  const node = props.node || {};
  return View(
    {
      class: "block min-w-0 grow shrink basis-0 py-2.5 px-3 text-left border border-solid border-color-content rounded-content bg-content-panel-soft text-inherit",
      attributes: {
        title: node.has_url ? node.url : node.id || node.title,
      },
    },
    [
      View({ class: "text-caption font-700 text-content-muted" }, [node.type_name]),
      View({ class: "mt-1 overflow-hidden text-md font-650 whitespace-nowrap text-ellipsis text-content-text" }, [node.title]),
      View({ class: "mt-1 overflow-hidden text-caption font-mono whitespace-nowrap text-ellipsis text-content-muted" }, [node.meta_text]),
    ],
  );
}

function HomeContentInfluencerEntity(props) {
  const node = props.node || {};
  return View(
    {
      class: [
        "flex min-w-0 items-baseline gap-1 leading-1.2 text-content-text inline-entity ",
        props.content ? "is-content" : "is-influencer",
      ].join(" "),
    },
    [
      View({ class: "flex-none font-mono text-caption text-content-muted" }, [node.type_name]),
      View({ class: "min-w-0 overflow-hidden text-xs font-620 whitespace-nowrap text-ellipsis" }, [node.title]),
    ],
  );
}

function ScraperContentInfluencers(props) {
  const vm$ = props.store;
  const influencers = vm$.state.content_relations.influencers;
  return Show({
    when: influencers.present,
    ok() {
      return View({ class: "pt-1.5 pb-2 px-2.5" }, [
        View({ class: "flex items-center justify-between p-0 gap-1.5" }, [
          View({ class: "inline-flex items-center gap-1 text-xs font-650 text-content-text heading-icon-primary" }, [
            Timeless.Icon({ name: "user", size: 12 }),
            "相关人物",
          ]),
          View({ class: "text-caption font-600 text-content-muted" }, [
            influencers.count_text,
          ]),
        ]),
        View(
          {
            class: " pt-1 pb-0 px-0 relation-list-compact ",
          },
          [
            For({
              key: "key",
              each: influencers.items,
              render(relation_) {
                const relation = ScraperDetailValue(relation_);
                return View(
                  {
                    class:
                      "block min-w-0 relation-root relation-inline ",
                  },
                  [
                    View({ class: "flex min-w-0 max-620:flex-col items-center max-620:items-stretch gap-2.5 relation-flow" }, [
                      HomeContentInfluencerEntity({
                        node: relation.source,
                      }),
                      View({ class: "flex min-w-0 grow-0 shrink-0 basis-24 flex-col max-620:basis-auto items-center gap-1 text-center text-content-primary relation-edge relation-edge-responsive" }, [
                        Timeless.Icon({ name: "arrow-right", size: 11 }),
                        View({ class: "text-xs font-680 text-content-text relation-edge-label" }, [
                          relation.role_text,
                        ]),
                        View({ class: "text-caption font-mono text-content-muted relation-edge-meta" }, [
                          relation.type,
                        ]),
                      ]),
                      HomeContentInfluencerEntity({
                        node: relation.target,
                        content: true,
                      }),
                    ]),
                  ],
                );
              },
            }),
          ],
        ),
      ]);
    },
  });
}

function ScraperContentRelations(props) {
  const vm$ = props.store;
  const relations = vm$.state.content_relations;
  return Show({
    when: relations.present,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow dm-panel home-content-relations" }, [
        View({ class: "flex min-h-58px max-520:flex-wrap items-center justify-between py-2.5 px-3.5 max-520:pr-3 max-520:pl-3 gap-3 border-b border-b-solid border-b-color-content bg-content-panel card-heading" }, [
          View({ class: "flex min-w-0 items-center gap-2.5" }, [
            View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
              Timeless.Icon({ name: "git-branch", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-caption font-750 leading-1.2 tracking-0 text-content-muted" }, ["RELATIONS"]),
              View({ class: "mt-0.5 text-ui font-680 leading-1.2" }, ["内容关联"]),
            ]),
          ]),
          Tag({ name: "home-detail-badge", class: "max-520:max-w-44pct flex-none max-520:overflow-hidden font-variant-numeric-tabular-nums max-520:whitespace-nowrap max-520:text-ellipsis border border-solid border-color-content-primary-soft bg-content-primary-soft text-content-primary" }, [
            relations.count_text,
          ]),
        ]),
        Show({
          when: relations.content_present,
          ok() {
            return View(
              {
                class: "py-3 px-3.5",
              },
              [
                For({
                  key: "key",
                  each: relations.items,
                  render(chain_) {
                    const chain = ScraperDetailValue(chain_);
                    return View({ class: "block min-w-0 relation-root relation-item" }, [
                      View({ class: "mb-2 font-mono text-xs font-700 tracking-0.02em text-content-muted" }, [
                        chain.type_path_text,
                      ]),
                      View({ class: "flex min-w-0 max-620:flex-col items-center max-620:items-stretch gap-2.5 relation-flow" }, [
                        For({
                          key: "key",
                          each: chain.segments,
                          render(segment_) {
                            const segment = ScraperDetailValue(segment_);
                            if (segment.kind === "node") {
                              return ScraperContentRelationNode({
                                node: segment.node,
                              });
                            }
                            const edge = segment.edge || {};
                            return View(
                              { class: "flex min-w-0 grow-0 shrink-0 basis-24 flex-col max-620:basis-auto items-center gap-1 text-center text-content-primary relation-edge relation-edge-responsive" },
                              [
                                Timeless.Icon({
                                  name: "arrow-right",
                                  size: 15,
                                }),
                                View(
                                  {
                                    class: "text-xs font-680 text-content-text relation-edge-label",
                                  },
                                  [edge.type_name],
                                ),
                                View(
                                  {
                                    class: "text-caption font-mono text-content-muted relation-edge-meta",
                                  },
                                  [edge.type],
                                ),
                              ],
                            );
                          },
                        }),
                      ]),
                    ]);
                  },
                }),
              ],
            );
          },
        }),
        ScraperContentInfluencers({ store: vm$ }),
      ]);
    },
  });
}

function ScraperContentAccount(props) {
  const account = props.account;
  return Show({
    when: account.present,
    ok() {
      return View({ class: "flex w-full items-center mt-3 gap-2.5" }, [
        HomeAccountAvatar({ account }),
        View({ class: "min-w-0 flex-auto" }, [
          View({ class: "min-w-0 overflow-hidden text-md font-720 leading-1.35 whitespace-nowrap text-ellipsis text-content-text" }, [account.nickname]),
        ]),
      ]);
    },
  });
}

function HomeAccountAvatar(props) {
  const account = props.account;
  return View({ class: "relative w-38px h-38px flex-none overflow-hidden border-2 border-solid border-color-content-panel rounded-avatar bg-content-primary-soft shadow-avatar-outline" }, [
    View({ class: "flex absolute inset-0 w-full h-full items-center justify-center text-ui font-720 text-content-primary" }, [account.avatar_fallback]),
    Show({
      when: account.avatar_url,
      ok() {
        return LazyImg({
          class: "block absolute inset-0 w-full h-full object-cover",
          src: account.avatar_url,
          alt: account.nickname,
          attributes: {
            loading: "lazy",
            referrerpolicy: "no-referrer",
          },
        });
      },
    }),
  ]);
}

function ScraperDetailValue(value_) {
  return value_ && value_.value !== undefined ? value_.value : value_;
}

function ScraperVideoDetailMedia(props) {
  const vm$ = props.store;
  const detail = props.detail || {};
  const media = detail.media || {};
  return Show({
    when: media.present,
    ok() {
      return View(
        {
          class: [
            "relative w-full aspect-video mb-2.5 overflow-hidden border border-solid border-color-content rounded-content bg-color-media-canvas group-home-detail-video-frame video-frame ",
            media.has_video ? "has-video" : "",
            media.has_cover ? "has-cover" : "",
          ]
            .filter(Boolean)
            .join(" "),
        },
        [
          Show({
            when: media.has_video,
            ok() {
              return Timeless.Video(
                {
                  class: "block absolute inset-0 w-full h-full object-contain bg-color-media-canvas",
                  attributes: {
                    src: media.video_url,
                    poster: media.cover_url,
                    controls: "controls",
                    preload: "metadata",
                    playsinline: "playsinline",
                    "webkit-playsinline": "webkit-playsinline",
                    referrerpolicy: "no-referrer",
                  },
                  onPlay(event) {
                    const root = event.currentTarget.closest(
                      ".group-home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.add("is-playing");
                    }
                  },
                  onPause(event) {
                    const root = event.currentTarget.closest(
                      ".group-home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.remove("is-playing");
                    }
                  },
                  onEnded(event) {
                    const root = event.currentTarget.closest(
                      ".group-home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.remove("is-playing");
                    }
                  },
                },
                ["当前浏览器不支持视频播放"],
              );
            },
            else() {
              return LazyImg({
                class: "block absolute inset-0 w-full h-full object-cover",
                src: media.cover_url,
                alt: detail.title,
                attributes: {
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
              });
            },
          }),
          Show({
            when: Boolean(media.has_video || detail.link_url),
            ok() {
              return View(
                {
                  class: "absolute top-1/2 left-1/2 translate-center video-play-control",
                  attributes: { n: "scraper-video-play-position" },
                },
                [
                  Button(
                    {
                      store: vm$.ui.btn_video_play$.bind(detail),
                      class: "dm-button--media",
                      attributes: {
                        n: "scraper-video-play-action",
                        type: "button",
                        title: media.has_video ? "播放视频" : "打开视频地址",
                        "aria-label": media.has_video
                          ? "播放视频"
                          : "打开视频地址",
                      },
                      onClick(event) {
                        event.stopPropagation();
                        const trigger = event.currentTarget || event.target;
                        const root =
                          trigger && typeof trigger.closest === "function"
                            ? trigger.closest(".group-home-detail-video-frame")
                            : null;
                        const video = root ? root.querySelector("video") : null;
                        if (video && typeof video.play === "function") {
                          video.controls = true;
                          const play_result = video.play();
                          if (
                            play_result &&
                            typeof play_result.catch === "function"
                          ) {
                            play_result.catch(() => {});
                          }
                          return;
                        }
                        if (detail.link_url) {
                          vm$.methods.openDetailURL(detail.link_url);
                        }
                      },
                    },
                    [Timeless.Icon({ name: "play", size: 22 })],
                  ),
                ],
              );
            },
          }),
        ],
      );
    },
  });
}

function ScraperNovelChapterItem(props) {
  const vm$ = props.store;
  const chapter = props.chapter || {};
  const children = [
    View({ class: "min-w-54px flex-none text-caption font-variant-numeric-tabular-nums text-content-muted" }, [chapter.index_text]),
    View({ class: "min-w-0 flex-auto" }, [
      View(
        {
          class: "overflow-hidden text-sm font-620 whitespace-nowrap text-ellipsis text-content-text",
          attributes: { title: chapter.title },
        },
        [chapter.title],
      ),
      Show({
        when: Boolean(chapter.meta_text),
        ok() {
          return View({ class: "mt-0.5 text-caption text-content-muted" }, [
            chapter.meta_text,
          ]);
        },
      }),
    ]),
    Show({
      when: Boolean(chapter.url),
      ok() {
        return View({ class: "flex flex-none text-content-primary" }, [
          Timeless.Icon({ name: "external-link", size: 13 }),
        ]);
      },
    }),
  ];
  if (!chapter.url) {
    return View({ class: "dm-list-card" }, children);
  }
  return Button(
    {
      store: vm$.ui.btn_novel_chapter$.bind(chapter),
      class: "dm-list-card is-link",
      attributes: { type: "button", title: "打开章节" },
    },
    children,
  );
}

function ScraperNovelDetails(props) {
  const vm$ = props.store;
  const novel = vm$.state.content_details.novel;
  return Show({
    when: novel.present,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel dm-panel home-novel-detail" }, [
        View({ class: "flex min-h-56px items-center justify-between max-520:items-start py-2.5 px-3 gap-2.5 border-b border-b-solid border-b-color-content bg-content-panel-soft" }, [
          View({ class: "flex min-w-0 items-center gap-2.5" }, [
            View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary detail-icon" }, [
              Timeless.Icon({ name: "file-stack", size: 18 }),
            ]),
            View({}, [
              View({ class: "max-w-full overflow-hidden text-base font-680 leading-1.25 whitespace-nowrap text-ellipsis text-content-text" }, [novel.title]),
              View({ class: "max-w-full mt-1 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [novel.subtitle]),
            ]),
          ]),
          Tag({ name: "home-detail-badge", class: "max-520:max-w-44pct flex-none max-520:overflow-hidden font-variant-numeric-tabular-nums max-520:whitespace-nowrap max-520:text-ellipsis border border-solid border-color-content-primary-soft bg-content-primary-soft text-content-primary" }, [
            novel.progress_text,
          ]),
        ]),
        View({ class: "p-3.5 max-520:p-2.5" }, [
          View({ class: "grid grid-cols-4 max-820:grid-cols-2 gap-2" }, [
            For({
              each: novel.metrics,
              render(metric_) {
                const metric = ScraperDetailValue(metric_);
                return View({ class: "min-w-0 p-2.5 rounded-content bg-content-panel-soft" }, [
                  View({ class: "text-caption text-content-muted" }, [metric.label]),
                  View({ class: "mt-1 overflow-hidden text-md font-680 leading-1.35 whitespace-nowrap text-ellipsis text-content-text" }, [metric.value]),
                ]);
              },
            }),
          ]),
          Show({
            when: novel.has_volumes,
            ok() {
              return View({ class: "mt-4" }, [
                View({ class: "flex items-center justify-between mb-2 gap-2.5" }, [
                  View({ class: "text-md font-680 text-content-text" }, ["分卷"]),
                ]),
                View({ class: "grid grid-cols-3 max-820:grid-cols-1 gap-2" }, [
                  For({
                    key: "key",
                    each: novel.volumes,
                    render(volume_) {
                      const volume = ScraperDetailValue(volume_);
                      return View({ class: "flex min-w-0 items-center p-2.5 gap-2 border border-solid border-color-content rounded-content bg-content-panel-soft" }, [
                        View({ class: "flex-none text-caption font-680 text-content-primary" }, [
                          volume.index_text,
                        ]),
                        View(
                          {
                            class: "min-w-0 overflow-hidden text-sm whitespace-nowrap text-ellipsis text-content-text",
                            attributes: { title: volume.title },
                          },
                          [volume.title],
                        ),
                      ]);
                    },
                  }),
                ]),
              ]);
            },
          }),
          View({ class: "mt-4 home-novel-chapters" }, [
            View({ class: "flex items-center justify-between mb-2 gap-2.5" }, [
              View({ class: "text-md font-680 text-content-text" }, ["章节列表"]),
              View({ class: "text-xs font-variant-numeric-tabular-nums text-content-muted" }, [novel.progress_text]),
            ]),
            Show({
              when: novel.has_chapters,
              ok() {
                return View({ class: "grid grid-cols-2 min-821:grid-cols-4 min-1280:grid-cols-fit-60 max-620:grid-cols-1 gap-1.5" }, [
                  For({
                    key: "key",
                    each: novel.chapters,
                    render(chapter_) {
                      return ScraperNovelChapterItem({
                        store: vm$,
                        chapter: ScraperDetailValue(chapter_),
                      });
                    },
                  }),
                ]);
              },
              else() {
                return View({ class: "p-4.5 text-sm text-center border border-dashed border-color-content rounded-content text-content-muted" }, [
                  novel.empty_chapter_text,
                ]);
              },
            }),
            Show({
              when: novel.has_more_chapters,
              ok() {
                return View(
                  {
                    class: "w-full mt-2",
                    attributes: { n: "scraper-novel-more-control" },
                  },
                  [
                    Button(
                      {
                        store: vm$.ui.btn_show_more_chapters$,
                        class: "w-full dm-button--dashed",
                        attributes: {
                          n: "scraper-novel-more-action",
                          type: "button",
                        },
                      },
                      [novel.more_chapters_text],
                    ),
                  ],
                );
              },
            }),
          ]),
        ]),
      ]);
    },
  });
}

function ScraperVideoVariantItem(props) {
  const vm$ = props.store;
  const variant = props.variant || {};
  const select_label = `选择视频规格 ${variant.title || variant.variant_key}`;
  const selected_ = computed(
    vm$.state.selected_video_variant,
    (selected_variant) => {
      if (!selected_variant || typeof selected_variant !== "object") {
        return Boolean(variant.selected);
      }
      const selected_detail_key = String(
        selected_variant.detail_key || "",
      ).trim();
      if (
        selected_detail_key &&
        selected_detail_key !== String(props.detailKey || "").trim()
      ) {
        return Boolean(variant.selected);
      }
      return (
        String(selected_variant.variant_key || "").trim() ===
        String(variant.variant_key || "").trim()
      );
    },
  );
  return Button(
    {
      store: vm$.ui.btn_video_variant$.bind({
        detail_key: props.detailKey,
        variant,
      }),
      class: computed(selected_, (selected) =>
        selected
          ? "motion-reduce:transition-none dm-button--choice-card is-selected"
          : "motion-reduce:transition-none dm-button--choice-card",
      ),
      attributes: {
        type: "button",
        role: "radio",
        "aria-checked": computed(selected_, (selected) =>
          selected ? "true" : "false",
        ),
        "aria-label": select_label,
        title: select_label,
      },
    },
    [
      View({ class: "flex w-18px h-18px grow-0 shrink-0 basis-4.5 items-center justify-center border border-solid border-color-content rounded-control bg-content-panel text-transparent transition-selection motion-reduce:transition-none choice-indicator" }, [
        Show({
          when: selected_,
          ok() {
            return Timeless.Icon({ name: "check", size: 12 });
          },
        }),
      ]),
      View({ class: "min-w-0 flex-auto" }, [
        View(
          {
            class: "overflow-hidden text-xs font-680 whitespace-nowrap text-ellipsis text-content-text choice-label",
            attributes: { title: variant.variant_key },
          },
          [variant.title],
        ),
        View({ class: "mt-1 overflow-hidden text-caption whitespace-nowrap text-ellipsis text-content-muted" }, [
          variant.meta_text || variant.variant_key,
        ]),
        Show({
          when: Boolean(variant.url),
          ok() {
            return View(
              {
                class: "mt-1 overflow-hidden text-caption font-mono whitespace-nowrap text-ellipsis text-content-muted",
                attributes: { title: variant.url },
              },
              [variant.url],
            );
          },
        }),
      ]),
      Show({
        when: Boolean(variant.is_default),
        ok() {
          return Tag(
            {
              name: "home-video-supplement-badge",
              class: "bg-content-primary-soft text-content-primary",
            },
            ["默认"],
          );
        },
      }),
    ],
  );
}

function ScraperContentTextTrackItem(props) {
  const vm$ = props.store;
  const track = props.track || {};
  const checkbox = vm$.methods.createTextTrackCheckbox(track);
  const selected_ = checkbox.selected;
  return View({ class: "min-w-0" }, [
    View(
      {
        class: computed(selected_, (selected) =>
          selected
            ? "flex min-w-0 items-start p-2.5 gap-2.5 border border-solid border-color-content rounded-content bg-content-panel-soft selectable-card is-selected"
            : "flex min-w-0 items-start p-2.5 gap-2.5 border border-solid border-color-content rounded-content bg-content-panel-soft selectable-card ",
        ),
      },
      [
        Checkbox({
          store: checkbox.store,
          class: "flex-none",
          attributes: {
            n: "scraper-text-track-checkbox",
            "aria-label": `下载字幕 ${track.title || track.track_key}`,
          },
          onUnmounted: checkbox.dispose,
        }),
        View({ class: "min-w-0 flex-auto" }, [
          View(
            {
              class: "overflow-hidden text-xs font-680 whitespace-nowrap text-ellipsis text-content-text choice-label",
              attributes: { title: track.track_key },
            },
            [track.title],
          ),
          View({ class: "mt-1 overflow-hidden text-caption whitespace-nowrap text-ellipsis text-content-muted" }, [
            track.meta_text || track.track_key,
          ]),
        ]),
        Tag(
          {
            name: "home-video-supplement-badge",
            class: "bg-content-primary-soft text-content-primary",
          },
          [`${track.sources.length} 个源`],
        ),
      ],
    ),
    Show({
      when: track.has_sources,
      ok() {
        return View({ class: "flex min-w-0 flex-col mt-1.5 mb-0 mr-2 ml-5 gap-2" }, [
          For({
            key: "key",
            each: track.sources,
            render(source_) {
              const source = ScraperDetailValue(source_);
              return View({ class: "grid min-w-0 grid-cols-compact-icon-content max-520:grid-cols-1 items-center py-2 px-2.5 gap-2 border-l-2 border-l-solid border-l-color-content-primary bg-content-panel-soft" }, [
                View({ class: "text-caption font-720 text-content-primary" }, [
                  source.title,
                ]),
                View(
                  {
                    class: "mt-0 overflow-hidden text-caption font-mono whitespace-nowrap text-ellipsis text-content-muted",
                    attributes: { title: source.url },
                  },
                  [source.url || source.meta_text || "未提供地址"],
                ),
              ]);
            },
          }),
        ]);
      },
    }),
  ]);
}

function ScraperVideoSupplements(props) {
  const detail = props.detail || {};
  return Fragment({}, [
    Show({
      when: detail.has_variants,
      ok() {
        return View({ class: "min-w-0 mt-3 pt-3 border-t border-t-solid border-t-color-content" }, [
          View({ class: "flex items-center justify-between mb-2 gap-2.5" }, [
            View({ class: "text-xs font-700 text-content-text" }, [
              "ContentVideoVariant",
            ]),
            Tag(
              {
                name: "home-video-supplement-count",
                class: "min-w-22px font-variant-numeric-tabular-nums bg-content-panel-soft text-content-muted",
              },
              [String(detail.variants.length)],
            ),
          ]),
          View(
            {
              class: "grid min-w-0 flex-col grid-cols-3 min-1280:grid-cols-4 min-1580:grid-cols-5 max-820:grid-cols-2 max-620:grid-cols-1 items-stretch gap-2",
              attributes: {
                role: "radiogroup",
                "aria-label": "选择下载视频规格",
              },
            },
            [
              For({
                key: "key",
                each: detail.variants,
                render(variant_) {
                  return ScraperVideoVariantItem({
                    store: props.store,
                    detailKey: detail.key,
                    variant: ScraperDetailValue(variant_),
                  });
                },
              }),
            ],
          ),
        ]);
      },
    }),
    Show({
      when: detail.has_text_tracks,
      ok() {
        return View({ class: "min-w-0 mt-3 pt-3 border-t border-t-solid border-t-color-content" }, [
          View({ class: "flex items-center justify-between mb-2 gap-2.5" }, [
            View({ class: "text-xs font-700 text-content-text" }, [
              "ContentTextTrack",
            ]),
            Tag(
              {
                name: "home-video-supplement-count",
                class: "min-w-22px font-variant-numeric-tabular-nums bg-content-panel-soft text-content-muted",
              },
              [String(detail.text_tracks.length)],
            ),
          ]),
          View({ class: "flex min-w-0 flex-col gap-2" }, [
            For({
              key: "key",
              each: detail.text_tracks,
              render(track_) {
                return ScraperContentTextTrackItem({
                  store: props.store,
                  track: ScraperDetailValue(track_),
                });
              },
            }),
          ]),
        ]);
      },
    }),
  ]);
}

function ScraperArticleBody(props) {
  const article_body = props.article_body || {};
  return Show({
    when: article_body.present,
    ok() {
      if (article_body.is_html) {
        return Timeless.RichText({
          class: "min-w-0 max-h-reading-pane mt-2.5 p-3.5 overflow-auto text-base leading-1.75 overflow-wrap-anywhere !border !border-solid !border-color-content rounded-content bg-content-panel-soft text-content-text rich-text-body is-html",
          content: article_body.content,
          attributes: {
            "data-content-format": "html",
          },
        });
      }
      return View(
        {
          class: `min-w-0 max-h-reading-pane mt-2.5 p-3.5 overflow-auto text-base leading-1.75 overflow-wrap-anywhere !border !border-solid !border-color-content rounded-content bg-content-panel-soft text-content-text rich-text-body is-${article_body.format || "text"}`,
        },
        [article_body.content],
      );
    },
  });
}

function ScraperTypedContentDetail(props) {
  const vm$ = props.store;
  const detail = props.detail || {};
  return View(
    {
      class: `min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel home-typed-detail ${home_typed_detail_classes[detail.kind] || ""} dm-panel`,
    },
    [
      View({ class: "flex min-h-56px items-center justify-between max-520:items-start py-2.5 px-3 gap-2.5 border-b border-b-solid border-b-color-content bg-content-panel-soft" }, [
        View({ class: "flex min-w-0 items-center gap-2.5" }, [
          View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary detail-icon" }, [
            Timeless.Icon({ name: detail.icon, size: 18 }),
          ]),
          View({}, [
            View({ class: "max-w-full overflow-hidden text-base font-680 leading-1.25 whitespace-nowrap text-ellipsis text-content-text" }, [detail.title]),
            View({ class: "max-w-full mt-1 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [detail.model_name]),
          ]),
        ]),
        Tag({ name: "home-detail-badge", class: "max-520:max-w-44pct flex-none max-520:overflow-hidden font-variant-numeric-tabular-nums max-520:whitespace-nowrap max-520:text-ellipsis border border-solid border-color-content-primary-soft bg-content-primary-soft text-content-primary" }, [
          detail.type_name,
        ]),
      ]),
      View({ class: "p-3.5 max-520:p-2.5" }, [
        Show({
          when: detail.subject.present,
          ok() {
            return View({ class: "flex items-center justify-between mb-3 p-2.5 gap-2.5 border border-solid border-color-content rounded-content bg-content-panel-soft" }, [
              View({ class: "min-w-0" }, [
                View({ class: "overflow-hidden text-sm font-650 whitespace-nowrap text-ellipsis text-content-text" }, [
                  detail.subject.title,
                ]),
                View({ class: "mt-1 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-content-muted" }, [
                  detail.subject.meta_text,
                ]),
              ]),
            ]);
          },
        }),
        ScraperVideoDetailMedia({ store: vm$, detail }),
        ScraperVideoSupplements({ store: vm$, detail }),
        ScraperArticleBody({ article_body: detail.article_body }),
        Show({
          when: Boolean(detail.is_collection),
          ok() {
            return FileTreeView({
              resources_: vm$.state.download_info.resources,
              renderRow(row_props) {
                return ScraperTreeRow({ store: vm$, ...row_props });
              },
            });
          },
        }),
        Show({
          when: detail.images.length > 0,
          ok() {
            return View({ class: "grid grid-cols-4 mt-2.5 gap-2" }, [
              For({
                key: "key",
                each: detail.images,
                render(image_) {
                  const image = ScraperDetailValue(image_);
                  return View({ class: "relative min-w-0 overflow-hidden border border-solid border-color-content rounded-content bg-content-panel-soft" }, [
                    LazyImg({
                      class: "block w-full aspect-ratio-1 object-cover",
                      src: image.url,
                      alt: "",
                      attributes: {
                        loading: "lazy",
                        referrerpolicy: "no-referrer",
                      },
                    }),
                    Show({
                      when: image.is_live_photo,
                      ok() {
                        return Tag(
                          {
                            name: "home-detail-image-live-badge",
                            class: "absolute top-6px right-6px border border-solid border-color-media bg-color-media-surface text-color-media-text",
                            attributes: { title: "实况图" },
                          },
                          [
                            Timeless.Icon({ name: "play", size: 10 }),
                            image.live_photo_label,
                          ],
                        );
                      },
                    }),
                    View({ class: "p-1.5 overflow-hidden text-caption text-center whitespace-nowrap text-ellipsis text-content-muted" }, [image.meta]),
                  ]);
                },
              }),
            ]);
          },
        }),
      ]),
    ],
  );
}

function ScraperContentDetails(props) {
  const vm$ = props.store;
  const details = vm$.state.content_details;
  return Show({
    when: details.present,
    ok() {
      return Fragment({}, [
        ScraperNovelDetails({ store: vm$ }),
        For({
          key: "key",
          each: details.items,
          render(detail_) {
            return ScraperTypedContentDetail({
              store: vm$,
              detail: ScraperDetailValue(detail_),
            });
          },
        }),
      ]);
    },
  });
}

function ScraperRawJSON(props) {
  const vm$ = props.store;
  return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow" }, [
    Button(
      {
        store: vm$.ui.btn_toggle_json$,
        class: computed(vm$.state.json_expanded, (expanded) =>
          expanded
            ? "dm-button--disclosure is-expanded"
            : "dm-button--disclosure",
        ),
        attributes: {
          type: "button",
          "aria-controls": "home-json-body",
          "aria-expanded": vm$.state.json_expanded,
        },
      },
      [
        View({ class: "flex min-w-0 items-center gap-3" }, [
          View({ class: "flex w-36px h-36px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
            Timeless.Icon({ name: "braces", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-md font-650 text-content-text" }, [vm$.state.json_toggle_text]),
            View({ class: "mt-1 text-xs text-content-muted" }, [
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
            class: "max-h-floating-list m-0 p-4 overflow-auto font-mono text-sm leading-1.6 whitespace-pre-wrap word-break-break-word border-t border-t-solid border-t-color-content bg-content-panel-soft text-content-text",
            attributes: { id: "home-json-body" },
          },
          [vm$.state.result_text],
        );
      },
    }),
  ]);
}

function ScraperFetchedRawContent(props) {
  const vm$ = props.store;
  const raw_result = vm$.state.raw_result;
  return Show({
    when: raw_result.visible,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow home-raw-fetch" }, [
        Button(
          {
            store: vm$.ui.btn_toggle_raw_result$,
            class: computed(raw_result.expanded, (expanded) =>
              expanded
                ? "dm-button--disclosure is-expanded"
                : "dm-button--disclosure",
            ),
            attributes: {
              type: "button",
              "aria-controls": "home-raw-fetch-body",
              "aria-expanded": raw_result.expanded,
            },
          },
          [
            View({ class: "flex min-w-0 items-center gap-3" }, [
              View({ class: "flex w-36px h-36px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
                Timeless.Icon({ name: "braces", size: 17 }),
              ]),
              View({}, [
                View({ class: "text-md font-650 text-content-text" }, ["抓取原始内容"]),
                View({ class: "mt-1 text-xs text-content-muted" }, [raw_result.meta_text]),
              ]),
            ]),
            Show({
              when: raw_result.expanded,
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
          when: raw_result.expanded,
          ok() {
            return View(
              {
                type: "pre",
                class: "max-h-floating-list m-0 p-4 overflow-auto font-mono text-sm leading-1.6 whitespace-pre-wrap word-break-break-word border-t border-t-solid border-t-color-content bg-content-panel-soft text-content-text",
                attributes: { id: "home-raw-fetch-body" },
              },
              [raw_result.text],
            );
          },
        }),
      ]);
    },
  });
}

function ScraperTreeRow(props) {
  const { store: vm$, row, collapsed_, onToggle } = props;
  const { node, depth } = row.value;
  const is_directory = node && node.type === "directory";
  const is_collapsed = is_directory && collapsed_.value.has(node._path);
  const indent_px = `${Math.min(depth * 18, 180)}px`;

  if (is_directory) {
    return View(
      {
        class: "grid grid-cols-file-tree items-center py-1 px-1.5 gap-1 text-sm rounded-content text-color-text-primary tree-row is-directory",
        style: { "padding-left": indent_px },
        attributes: { n: "scraper-tree-dir", title: node._path || node.name },
        onClick() {
          onToggle(node._path);
        },
      },
      [
        View({ class: "w-18px shrink-0 text-13px leading-1 text-center text-color-text-muted" }, [is_collapsed ? "›" : "⌄"]),
        View({ class: "w-18px shrink-0" }, [
          Timeless.Icon({ name: "folder", size: 16 }),
        ]),
        View({ class: "min-w-0 dm-truncate" }, [node.name || "根目录"]),
        View({ class: "pl-2 text-11px font-normal whitespace-nowrap text-color-text-muted" }, [`${count_tree_children(node)} 项`]),
      ],
    );
  }

  const resource = node && node.resource;
  return View(
    {
      class: "grid grid-cols-file-actions items-center py-1 px-1.5 gap-1 text-sm rounded-content text-color-text-primary tree-row-actions ",
      style: { "padding-left": indent_px },
      attributes: { n: "scraper-tree-file", title: node._path || node.name },
    },
    [
      View({ class: "w-18px shrink-0 text-13px leading-1 text-center text-color-text-muted" }),
      View({ class: "w-18px shrink-0" }, [
        Timeless.Icon({
          name: resource_file_icon(node && node.name),
          size: 16,
        }),
      ]),
      View({ class: "min-w-0 dm-truncate" }, [
        (node && node.name) || "文件",
      ]),
      resource && resource.is_cover
        ? Tag(
            {
              name: "scraper-tree-file-cover",
              class: "shrink-0 bg-content-primary-soft text-content-primary",
              attributes: { title: "额外下载的封面图片" },
            },
            ["封面"],
          )
        : null,
      View({ class: "pl-2 text-11px font-normal whitespace-nowrap text-color-text-muted" }, [
        resource
          ? [resource.kind, format_file_size(resource.size)]
              .filter(Boolean)
              .join(" · ")
          : format_file_size(node && node.size),
      ]),
      resource
        ? Button(
            {
              store: vm$.ui.btn_download_resource$.bind(resource),
              attributes: {
                type: "button",
                title: `下载 ${node.name}`,
                "aria-label": `下载 ${node.name}`,
              },
            },
            [Timeless.Icon({ name: "download", size: 14 })],
          )
        : null,
    ],
  );
}

function HomeDownloadSection(props, children = []) {
  return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel" }, [
    View({ class: "flex min-h-42px items-center justify-between py-2 px-3 gap-2.5 border-b border-b-solid border-b-color-content bg-content-panel-soft" }, [
      View({ class: "text-sm font-700 text-content-text" }, [props.title]),
      Tag(
        {
          name: "home-download-section-count",
          class: "min-w-22px font-variant-numeric-tabular-nums bg-content-panel-soft text-content-muted",
        },
        [props.count],
      ),
    ]),
    View({ class: "p-2.5" }, children),
  ]);
}

function ScraperDownloadInfo(props) {
  const vm$ = props.store;
  const download_info = vm$.state.download_info;
  const task = download_info.task;
  return Show({
    when: download_info.present,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow dm-panel home-download-info" }, [
        View({ class: "flex min-h-58px max-520:flex-wrap items-center justify-between py-2.5 px-3.5 max-520:pr-3 max-520:pl-3 gap-3 border-b border-b-solid border-b-color-content bg-content-panel card-heading" }, [
          View({ class: "flex min-w-0 items-center gap-2.5" }, [
            View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
              Timeless.Icon({ name: "download", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-caption font-750 leading-1.2 tracking-0 text-content-muted" }, ["DOWNLOAD INFO"]),
              View({ class: "mt-0.5 text-ui font-680 leading-1.2" }, ["下载任务预览"]),
            ]),
          ]),
          View({ class: "flex max-520:w-full flex-wrap items-center justify-end ml-auto gap-2" }, [
            Show({
              when: computed(download_info.badge_text, (text) => Boolean(text)),
              ok() {
                return Tag(
                  {
                    name: "scraper-download-preview-status",
                    class: download_info.badge_class,
                  },
                  [download_info.badge_text],
                );
              },
            }),
          ]),
        ]),
        View({ class: "flex flex-col p-3.5 max-520:p-2.5 gap-3" }, [

          Show({
            when: computed(download_info.hide_resources, (hide) => !hide),
            ok() {
              return HomeDownloadSection(
                {
                  title: "资源文件",
                  count: download_info.resource_count_text,
                },
                [
                  Show({
                    when: download_info.has_cover,
                    ok() {
                      return View(
                        {
                          class: "mb-2.5 text-xs leading-base text-color-text-secondary dm-notice-row",
                          attributes: { n: "scraper-download-cover-notice" },
                        },
                        [
                          Timeless.Icon({ name: "image", size: 15 }),
                          View({ class: "dm-min-w-0 dm-flex-1" }, [
                            download_info.cover_notice_text,
                          ]),
                        ],
                      );
                    },
                  }),
                  FileTreeView({
                    resources_: download_info.resources,
                    renderRow(row_props) {
                      return ScraperTreeRow({ store: vm$, ...row_props });
                    },
                  }),
                ],
              );
            },
          }),
          HomeDownloadSection({ title: "下载任务", count: 1 }, [
            View({ class: "flex min-w-0 flex-col gap-2" }, [
              View({ class: "grid min-w-0 max-520:grid-cols-icon-content grid-cols-34px-main-auto items-center p-2.5 gap-2.5 border border-solid border-color-content rounded-content bg-content-panel-soft icon-info-card " }, [
                View({ class: "font-mono text-caption font-variant-numeric-tabular-nums text-center text-content-muted" }, [task.id_text]),
                View({ class: "min-w-0" }, [
                  View(
                    {
                      class: "overflow-hidden text-sm font-680 whitespace-nowrap text-ellipsis text-content-text",
                      attributes: { title: task.name },
                    },
                    [task.name],
                  ),
                  View({ class: "mt-1 overflow-hidden text-caption whitespace-nowrap text-ellipsis text-content-muted" }, [
                    task.meta_text || "任务将在确认后创建",
                  ]),
                ]),
                Show({
                  when: computed(task.status_text, (text) => Boolean(text)),
                  ok() {
                    return Tag(
                      {
                        name: "home-download-status",
                        class: "bg-color-success-soft text-color-success info-status",
                      },
                      [task.status_text],
                    );
                  },
                }),
              ]),
            ]),
          ]),
        ]),
      ]);
    },
  });
}

function ScraperResultActionBar(props) {
  const vm$ = props.store;
  return View(
    {
      class: "flex sticky bottom-0 z-sticky box-sizing-border-box w-full min-w-0 min-h-72px max-520:flex-wrap items-center justify-between max-520:items-stretch py-3 px-4 max-520:p-2.5 gap-4 border-0 rounded-0 border-t-default border-t-solid border-t-color-content bg-content-panel/72 shadow-none backdrop-media",
      attributes: {
        n: "result-actions",
        role: "toolbar",
        "aria-label": "抓取完成操作",
      },
    },
    [
      View(
        {
          class: "flex min-w-0 flex-auto items-center gap-2.5",
          attributes: { n: "result-actions-summary" },
        },
        [
          View(
            {
              class: "grid w-36px h-36px flex-none place-items-center rounded-content bg-color-success-soft text-color-success",
              attributes: { n: "result-actions-status-icon" },
            },
            [Timeless.Icon({ name: "check", size: 17 })],
          ),
          View(
            {
              class: "min-w-0",
              attributes: { n: "result-actions-copy" },
            },
            [
              View(
                {
                  class: "text-sm font-700 text-content-text",
                  attributes: { n: "result-actions-title" },
                },
                ["抓取完成"],
              ),
              View(
                {
                  class: "mt-0.5 text-caption text-content-muted",
                  attributes: { n: "result-actions-description" },
                },
                ["请选择下载方式"],
              ),
            ],
          ),
        ],
      ),
      View(
        {
          class: "flex max-520:w-full flex-none items-center justify-end gap-2 action-row-stretch ",
          attributes: { n: "result-actions-controls" },
        },
        [
          Button(
            {
              store: vm$.ui.btn_third_party_download$,
              attributes: {
                n: "third-party-download-action",
                type: "button",
                title: "使用 DownloadResource endpoint 创建三方下载任务",
              },
            },
            [Timeless.Icon({ name: "hard-drive", size: 16 }), "三方下载"],
          ),
          Button(
            {
              store: vm$.ui.btn_create_download_task$,
              attributes: {
                n: "create-download-task-action",
                type: "button",
                title: "下载当前预览中的全部资源",
              },
            },
            [
              Timeless.Icon({ name: "download", size: 16 }),
              vm$.state.download_all_button_text,
            ],
          ),
        ],
      ),
    ],
  );
}

function ScraperCacheCard(props) {
  const vm$ = props.store;
  const cache = vm$.state.cache;
  return Show({
    when: cache.present,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow dm-panel responsive-card" }, [
        View({ class: "flex min-h-58px max-520:flex-wrap items-center justify-between py-2.5 px-3.5 max-520:pr-3 max-520:pl-3 gap-3 border-b border-b-solid border-b-color-content bg-content-panel card-heading" }, [
          View({ class: "flex min-w-0 items-center gap-2.5" }, [
            View({ class: "flex w-34px h-34px flex-none items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
              Timeless.Icon({ name: "file-stack", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-caption font-750 leading-1.2 tracking-0 text-content-muted" }, ["CACHE"]),
              View({ class: "mt-0.5 text-ui font-680 leading-1.2" }, ["抓取缓存"]),
            ]),
          ]),
          View({ class: "flex max-520:w-full max-520:flex-col items-center justify-end max-520:items-stretch gap-2.5 card-actions-stretch " }, [
            View({ class: "text-sm whitespace-nowrap max-520:whitespace-normal text-content-muted" }, [cache.summary_text]),
            Button(
              {
                store: vm$.ui.btn_clear_fetch_cache$,
                attributes: {
                  type: "button",
                  title: "清除该 URL 的抓取缓存",
                },
              },
              [
                Timeless.Icon({ name: "trash2", size: 14 }),
                vm$.state.cache_button_text,
              ],
            ),
          ]),
        ]),
        View({ class: "max-h-360px py-1.5 px-3.5 overflow-auto" }, [
          For({
            key: "key",
            each: cache.entries,
            render(entry_) {
              const entry = ScraperDetailValue(entry_);
              return Button(
                {
                  store: vm$.ui.btn_cache_entry$.bind(entry),
                  class: "max-520:grid-cols-small-icon-content dm-button--list-row",
                  attributes: {
                    type: "button",
                    title: `查看缓存内容：${entry.name}`,
                    "aria-label": `查看缓存内容：${entry.name}`,
                  },
                },
                [
                  View({ class: "flex w-30px h-30px items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
                    Timeless.Icon({ name: "file", size: 16 }),
                  ]),
                  View({ class: "min-w-0" }, [
                    View(
                      {
                        class: "overflow-hidden text-sm font-620 whitespace-nowrap text-ellipsis text-content-text",
                        attributes: { title: entry.name },
                      },
                      [entry.name],
                    ),
                    View(
                      {
                        class: "mt-1 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-content-muted",
                        attributes: { title: entry.path },
                      },
                      [entry.path],
                    ),
                  ]),
                  View({ class: "inline-flex max-520:grid-column-2 items-center max-520:justify-self-start gap-1.5 text-content-muted" }, [
                    View({ class: "max-520:grid-column-2 text-xs font-variant-numeric-tabular-nums whitespace-nowrap text-content-muted" }, [entry.size_text]),
                    Timeless.Icon({ name: "chevron-right", size: 15 }),
                  ]),
                ],
              );
            },
          }),
        ]),
      ]);
    },
  });
}

function ScraperCacheContentDialog(props) {
  const vm$ = props.store;
  const cache_content = vm$.state.cache_content;

  return Dialog(
    {
      store: vm$.ui.cache_content_dialog$,
      class: "dm-dialog--lg",
      showClose: false,
    },
    [
      View({ class: "flex items-start pt-4 pb-3 px-4.5 gap-3.5" }, [
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "overflow-hidden text-lg font-680 whitespace-nowrap text-ellipsis" }, [cache_content.title]),
          View({ class: "mt-1 text-xs text-content-muted" }, [cache_content.meta_text]),
        ]),
        Button(
          {
            store: vm$.ui.btn_close_cache_content$,
            attributes: { type: "button", "aria-label": "关闭缓存内容" },
          },
          ["×"],
        ),
      ]),
      View(
        {
          class: "pt-0 pb-3 px-4.5 overflow-hidden font-mono text-caption whitespace-nowrap text-ellipsis text-content-muted",
          attributes: { title: cache_content.path },
        },
        [cache_content.path],
      ),
      View({ class: "flex min-h-260px overflow-hidden border-t border-t-solid border-t-color-content bg-content-panel-soft" }, [
        Show({
          when: cache_content.loading,
          ok() {
            return View({ class: "flex min-h-260px w-full items-center justify-center gap-2.5 text-md text-content-muted" }, [
              View({
                class: "dm-spinner",
                attributes: { n: "scraper-cache-loading" },
              }),
              "正在读取缓存内容...",
            ]);
          },
          else() {
            return Show({
              when: computed(cache_content.error, (error) => Boolean(error)),
              ok() {
                return ErrorMessage({ tip: vm$.methods.tip, error: cache_content.error });
              },
              else() {
                return View(
                  {
                    type: "pre",
                    class: "box-sizing-border-box w-full min-h-260px max-h-stage m-0 py-4 px-4.5 overflow-auto font-mono text-sm leading-1.6 whitespace-pre-wrap word-break-break-word tab-size-2 text-content-text",
                  },
                  [cache_content.text],
                );
              },
            });
          },
        }),
      ]),
    ],
  );
}

function BuildDownloadTaskResultView(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.build_download_task_result_visible,
    ok() {
      return View(
        {
          class: "flex flex-col gap-3.5",
          attributes: { n: "scraper-result" },
        },
        [
          ScraperContentCard({ store: vm$ }),
          ScraperContentRelations({ store: vm$ }),
          ScraperContentDetails({ store: vm$ }),
          ScraperDownloadInfo({ store: vm$ }),
          ScraperRawJSON({ store: vm$ }),
          ScraperCacheCard({ store: vm$ }),
        ],
      );
    },
  });
}

function PlatformHomeScraperResultView(props) {
  const vm$ = props.store;
  const home = vm$.state.platform_home;
  return Show({
    when: home.present,
    ok() {
      return View(
        {
          class: "flex flex-col gap-3.5",
          attributes: { n: "platform-home-scraper-result" },
        },
        [
          View(
            {
              class: "min-w-0 overflow-hidden border border-solid border-color-content rounded-card bg-content-panel shadow-content-shadow dm-panel home-platform-preview-card",
              attributes: { n: "platform-home-preview-card" },
            },
            [
              View(
                {
                  class: "flex min-h-58px max-520:flex-wrap items-center justify-between py-2.5 px-3.5 max-520:pr-3 max-520:pl-3 gap-3 border-b border-b-solid border-b-color-content bg-content-panel card-heading",
                  attributes: { n: "platform-home-preview-heading" },
                },
                [
                  View(
                    {
                      class: "mt-0.5 text-ui font-680 leading-1.2",
                      attributes: { n: "platform-home-preview-title" },
                    },
                    [home.title],
                  ),
                ],
              ),
              View(
                {
                  class: "h-72vh max-h-205 bg-white",
                  attributes: { n: "platform-home-preview" },
                },
                [
                  View({
                    type: "iframe",
                    class: "w-full h-full border-0",
                    attributes: {
                      n: "platform-home-preview-frame",
                      srcdoc: home.html,
                      sandbox: "",
                      title: "平台主页抓取结果",
                    },
                  }),
                ],
              ),
            ],
          ),
          ScraperRawJSON({ store: vm$ }),
        ],
      );
    },
  });
}

function ScraperPageResult(props) {
  return Fragment({}, [
    BuildDownloadTaskResultView(props),
    PlatformHomeScraperResultView(props),
  ]);
}

function ScraperPlatformStatus(props) {
  const vm$ = props.store;
  const status = vm$.state.platform_status;
  return Popover(
    {
      store: vm$.ui.platform_status_popover$,
      class: "dm-popover--platform-status",
      side: "bottom",
      align: "end",
      onTriggerMouseEnter() {
        vm$.methods.showPlatformStatusPopover();
      },
      onTriggerMouseLeave() {
        vm$.methods.schedulePlatformStatusPopoverHide();
      },
      onContentMouseEnter() {
        vm$.methods.showPlatformStatusPopover();
      },
      onContentMouseLeave() {
        vm$.methods.schedulePlatformStatusPopoverHide();
      },
      content: [
        Show({
          when: status.has_items,
          ok() {
            return View(
              {
                class: "grid box-sizing-border-box w-full min-w-0 grid-cols-5 gap-1",
                attributes: {
                  n: "platform-status-list",
                  role: "list",
                  "aria-label": "平台状态",
                },
              },
              [
                For({
                  key: "render_key",
                  each: status.items,
                  render(item_) {
                    const item = ScraperDetailValue(item_);
                    const status_item = View(
                      {
                        class: item.status_class,
                        attributes: {
                          n: "platform-status-item",
                          role: "listitem",
                        },
                      },
                      [
                        View(
                          {
                            class: "grid relative w-5 h-5 min-w-5 min-h-5 max-w-5 max-h-5 grow-0 shrink-0 basis-5 place-items-center status-icon-wrap ",
                            attributes: {
                              n: "platform-status-logo-wrap",
                              "aria-hidden": "true",
                            },
                          },
                          [
                            PlatformIcon({
                              class: "block w-5 h-5 min-w-5 min-h-5 max-w-5 max-h-5",
                              favicon: item.platform_favicon,
                              name: "platform-status-logo",
                            }),
                            View({
                              class: "w-8px h-8px flex-none rounded-round bg-content-muted status-dot-target status-dot ",
                              attributes: { n: "platform-status-dot" },
                            }),
                          ],
                        ),
                        View(
                          {
                            class: "flex w-full min-w-0 flex-col items-center gap-0",
                            attributes: { n: "platform-status-main" },
                          },
                          [
                            View(
                              {
                                class: "flex w-full min-w-0 flex-col items-center gap-0",
                                attributes: { n: "platform-status-head" },
                              },
                              [
                                View(
                                  {
                                    class: "-webkit-box w-full min-w-0 min-h-2.4em overflow-hidden text-caption font-semibold leading-tight text-center overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2",
                                    attributes: { n: "platform-status-name" },
                                  },
                                  [item.platform_name],
                                ),
                                Show({
                                  when: item.has_status_text,
                                  ok() {
                                    return View(
                                      {
                                        class: "flex-none text-caption whitespace-nowrap text-content-muted status-action-label",
                                        attributes: {
                                          n: "platform-status-value",
                                        },
                                      },
                                      [item.status_text],
                                    );
                                  },
                                }),
                              ],
                            ),
                          ],
                        ),
                      ],
                    );
                    return item.has_reason
                      ? Tooltip(
                          {
                            store: item.tooltip$,
                            content: item.reason,
                            onContentMouseEnter:
                              vm$.methods.showPlatformStatusPopover,
                            onContentMouseLeave:
                              vm$.methods.schedulePlatformStatusPopoverHide,
                          },
                          [status_item],
                        )
                      : status_item;
                  },
                }),
              ],
            );
          },
          else() {
            return View({ class: "py-5 px-3.5 text-sm text-center text-content-muted" }, [
              "正在连接 /ws/scraper",
            ]);
          },
        }),
      ],
    },
    [
      Button(
        {
          store: vm$.ui.btn_platform_status$,
          class: status.trigger_class,
          attributes: {
            n: "platform-status-trigger",
            type: "button",
            title: "查看平台状态",
            "aria-label": "查看平台状态",
          },
        },
        [
          View({ class: "w-8px h-8px flex-none rounded-round bg-content-muted status-dot-target status-dot " }),
          View({ class: "home-platform-status-trigger-label" }, ["平台状态"]),
          View({ class: "max-520:hidden text-sm text-content-muted" }, [
            status.summary,
          ]),
        ],
      ),
    ],
  );
}

function ScraperFetchProgress(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.progress_visible,
    ok() {
      return View(
        {
          class: "flex flex-col p-3 gap-2 border border-solid border-transparent rounded-content bg-content-panel-soft",
          attributes: {
            role: "progressbar",
            "aria-label": "抓取进度",
            "aria-valuemin": "0",
            "aria-valuemax": "100",
            "aria-valuenow": vm$.state.progress_percent,
          },
        },
        [
          View({ class: "flex min-w-0 items-center justify-between gap-3" }, [
            View({ class: "min-w-0 overflow-hidden text-md font-600 whitespace-nowrap text-ellipsis text-content-text" }, [
              vm$.state.progress_stage_text,
            ]),
            View({ class: "flex flex-none items-center gap-2.5 text-sm text-content-muted" }, [
              Show({
                when: vm$.state.progress_updated_text,
                ok() {
                  return View({ class: "text-sm text-content-muted" }, [
                    vm$.state.progress_updated_text,
                  ]);
                },
              }),
              Show({
                when: vm$.state.progress_has_percent,
                ok() {
                  return View({ class: "font-variant-numeric-tabular-nums text-content-primary" }, [
                    vm$.state.progress_percent_text,
                  ]);
                },
              }),
            ]),
          ]),
          View({ class: "text-sm leading-1.45 overflow-wrap-anywhere text-content-muted" }, [
            vm$.state.progress_message,
          ]),
          View({ class: "h-6px overflow-hidden rounded-round bg-content-primary/13" }, [
            View({
              class: vm$.state.progress_bar_class,
              style: computed(vm$.state.progress_percent, (percent) => ({
                width: `${percent}%`,
              })),
            }),
          ]),
          Show({
            when: vm$.state.progress_has_total,
            ok() {
              return View({ class: "text-sm text-content-muted" }, [
                vm$.state.progress_count_text,
              ]);
            },
          }),
        ],
      );
    },
  });
}

export default ScraperPageView;

import { ThirdPartyDownloaderPanel } from "@/third-party-downloader.js";

import { PlatformIcon } from "../components.js";
import { ScraperPageViewModel } from "./scraper.model.js";

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

  return View(
    {
      class: "content-page home-page page",
      onMounted() {
        vm$.methods.connectProgress();
      },
      onUnmounted() {
        vm$.methods.dispose();
      },
    },
    [
      View({ class: "content-main home-main container" }, [
        View({ class: "home-content" }, [
          View(
            {
              class: "home-workbench dm-panel",
              attributes: { "aria-busy": vm$.state.busy },
            },
            [
              View({ class: "home-workbench-heading" }, [
                View({ class: "home-workbench-heading-icon" }, [
                  Timeless.Icon({ name: "inbox", size: 20 }),
                ]),
                View({ class: "home-workbench-heading-copy" }, [
                  View({ class: "home-workbench-kicker" }, ["新建归档任务"]),
                  View({ class: "home-workbench-title" }, [
                    "粘贴内容链接，开始解析",
                  ]),
                  View({ class: "home-workbench-description" }, [
                    "识别内容、作者与媒体资源，并生成可追踪的下载任务。",
                  ]),
                ]),
                View({ class: "home-header-actions" }, [
                  ScraperPlatformStatus({ store: vm$ }),
                ]),
              ]),
              ScraperPageForm({ store: vm$ }),
              Show({
                when: computed(vm$.state.status_text, (text) => Boolean(text)),
                ok() {
                  return View(
                    {
                      class: computed(vm$.state.has_error, (has_error) =>
                        has_error
                          ? "home-status error dm-badge dm-badge--danger"
                          : "home-status dm-badge dm-badge--info",
                      ),
                      attributes: {
                        role: "status",
                        "aria-live": "polite",
                      },
                    },
                    [
                      Show({
                        when: vm$.state.busy,
                        ok() {
                          return View({
                            class: "dm-spinner",
                            attributes: { n: "scraper-status-loading" },
                          });
                        },
                      }),
                      vm$.state.status_text,
                    ],
                  );
                },
              }),
              ScraperFetchProgress({ store: vm$ }),
            ],
          ),
          ScraperFetchedRawContent({ store: vm$ }),
          ScraperPageResult({ store: vm$ }),
        ]),
      ]),
      Show({
        when: vm$.state.result_visible,
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
                class: "dm-truncate dm-flex-1 dm-text-sm dm-font-semibold",
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
      class: "home-form",
      onSubmit(event) {
        event.preventDefault();
        vm$.methods.submit();
      },
    },
    [
      View(
        {
          class: "home-search-field",
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
      View({ class: "home-actions" }, [
        Show({
          when: vm$.state.loading,
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
          else() {
            return View({ class: "home-idle-actions" }, [
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
      return View({ class: "home-content-cover" }, [
        // View({ class: "home-content-cover-fallback" }, [
        //   Timeless.Icon({ name: "file", size: 34 }),
        //   content.content_type_name,
        // ]),
        LazyImg({
          class: "home-content-cover-image",
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
  return View({ class: "home-card home-content-card dm-panel" }, [
    View({ class: "home-card-heading" }, [
      View({ class: "home-card-title-group" }, [
        View({ class: "home-card-icon" }, [
          Timeless.Icon({ name: "file", size: 17 }),
        ]),
        View({}, [
          View({ class: "home-card-kicker" }, ["CONTENT"]),
          View({ class: "home-card-title" }, ["内容"]),
        ]),
      ]),
    ]),
    View({
      class: computed(content.cover_url, (cover_url) =>
        cover_url
          ? "home-content-card-body"
          : "home-content-card-body is-coverless",
      ),
    }, [
      ScraperContentCover({ content }),
      View({ class: "home-content-info" }, [
        View({ class: "home-badges" }, [
          View(
            {
              class:
                "home-badge home-badge-primary home-platform-badge",
            },
            [
              Show({
                when: content.platform_favicon,
                ok() {
                  return PlatformIcon({
                    class: "home-platform-favicon",
                    favicon: content.platform_favicon,
                    name: "home-platform-favicon",
                  });
                },
              }),
              content.platform_name,
            ],
          ),
          View({ class: "home-badge" }, [content.content_type_name]),
        ]),
        View(
          {
            class: "home-content-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        View({ class: "home-content-meta" }, [
          Timeless.Icon({ name: "clock3", size: 14 }),
          content.publish_time_text,
        ]),
        Show({
          when: content.show_description,
          ok() {
            return View({ class: "home-content-description" }, [
              description,
            ]);
          },
        }),
        ScraperContentAccount({ account }),
      ]),
    ]),
  ]);
}

function ScraperContentRelationNode(props) {
  const node = props.node || {};
  return View(
    {
      class: "home-content-relation-node",
      attributes: {
        title: node.has_url ? node.url : node.id || node.title,
      },
    },
    [
      View({ class: "home-content-relation-node-type" }, [node.type_name]),
      View({ class: "home-content-relation-node-title" }, [node.title]),
      View({ class: "home-content-relation-node-meta" }, [node.meta_text]),
    ],
  );
}

function HomeContentInfluencerEntity(props) {
  const node = props.node || {};
  return View(
    {
      class: [
        "home-content-influencer-entity",
        props.content ? "is-content" : "is-influencer",
      ].join(" "),
    },
    [
      View({ class: "home-content-influencer-entity-type" }, [
        node.type_name,
      ]),
      View({ class: "home-content-influencer-entity-title" }, [
        node.title,
      ]),
    ],
  );
}

function ScraperContentInfluencers(props) {
  const vm$ = props.store;
  const influencers = vm$.state.content_relations.influencers;
  return Show({
    when: influencers.present,
    ok() {
      return View({ class: "home-content-influencers" }, [
        View({ class: "home-content-influencer-heading" }, [
          View({ class: "home-content-influencer-title" }, [
            Timeless.Icon({ name: "user", size: 12 }),
            "相关人物",
          ]),
          View({ class: "home-content-influencer-count" }, [
            influencers.count_text,
          ]),
        ]),
        View(
          {
            class:
              "home-content-relation-list home-content-influencer-list",
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
                      "home-content-relation home-content-influencer-relation",
                  },
                  [
                    View({ class: "home-content-relation-flow" }, [
                      HomeContentInfluencerEntity({
                        node: relation.source,
                      }),
                      View({ class: "home-content-relation-edge" }, [
                        Timeless.Icon({ name: "arrow-right", size: 11 }),
                        View(
                          { class: "home-content-relation-edge-name" },
                          [relation.role_text],
                        ),
                        View(
                          { class: "home-content-relation-edge-type" },
                          [relation.type],
                        ),
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
      return View(
        { class: "home-card home-content-relations dm-panel" },
        [
        View({ class: "home-card-heading" }, [
          View({ class: "home-card-title-group" }, [
            View({ class: "home-card-icon" }, [
              Timeless.Icon({ name: "git-branch", size: 17 }),
            ]),
            View({}, [
              View({ class: "home-card-kicker" }, ["RELATIONS"]),
              View({ class: "home-card-title" }, ["内容关联"]),
            ]),
          ]),
          View({ class: "home-detail-badge" }, [relations.count_text]),
        ]),
        Show({
          when: relations.content_present,
          ok() {
            return View(
              {
                class: "home-content-relation-list",
              },
              [
                For({
                  key: "key",
                  each: relations.items,
                  render(chain_) {
                    const chain = ScraperDetailValue(chain_);
                    return View({ class: "home-content-relation" }, [
                      View({ class: "home-content-relation-path" }, [
                        chain.type_path_text,
                      ]),
                      View({ class: "home-content-relation-flow" }, [
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
                              { class: "home-content-relation-edge" },
                              [
                                Timeless.Icon({
                                  name: "arrow-right",
                                  size: 15,
                                }),
                                View(
                                  {
                                    class:
                                      "home-content-relation-edge-name",
                                  },
                                  [edge.type_name],
                                ),
                                View(
                                  {
                                    class:
                                      "home-content-relation-edge-type",
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
        ],
      );
    },
  });
}

function ScraperContentAccount(props) {
  const account = props.account;
  return Show({
    when: account.present,
    ok() {
      return View({ class: "home-content-account" }, [
        HomeAccountAvatar({ account }),
        View({ class: "home-content-account-main" }, [
          View({ class: "home-account-name" }, [account.nickname]),
        ]),
      ]);
    },
  });
}

function HomeAccountAvatar(props) {
  const account = props.account;
  return View({ class: "home-account-avatar" }, [
    View({ class: "home-account-avatar-fallback" }, [
      account.avatar_fallback,
    ]),
    Show({
      when: account.avatar_url,
      ok() {
        return LazyImg({
          class: "home-account-avatar-image",
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
            "home-detail-video-frame",
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
                  class: "home-detail-video",
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
                      ".home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.add("is-playing");
                    }
                  },
                  onPause(event) {
                    const root = event.currentTarget.closest(
                      ".home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.remove("is-playing");
                    }
                  },
                  onEnded(event) {
                    const root = event.currentTarget.closest(
                      ".home-detail-video-frame",
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
                class: "home-detail-video-cover",
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
                  class: "home-detail-video-play-position",
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
                            ? trigger.closest(".home-detail-video-frame")
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
    View({ class: "home-novel-chapter-index" }, [chapter.index_text]),
    View({ class: "home-novel-chapter-main" }, [
      View(
        {
          class: "home-novel-chapter-title",
          attributes: { title: chapter.title },
        },
        [chapter.title],
      ),
      Show({
        when: Boolean(chapter.meta_text),
        ok() {
          return View({ class: "home-novel-chapter-meta" }, [
            chapter.meta_text,
          ]);
        },
      }),
    ]),
    Show({
      when: Boolean(chapter.url),
      ok() {
        return View({ class: "home-novel-chapter-link" }, [
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
      return View(
        { class: "home-detail-card home-novel-detail dm-panel" },
        [
        View({ class: "home-detail-card-heading" }, [
          View({ class: "home-detail-card-title-group" }, [
            View({ class: "home-detail-card-icon" }, [
              Timeless.Icon({ name: "file-stack", size: 18 }),
            ]),
            View({}, [
              View({ class: "home-detail-card-title" }, [novel.title]),
              View({ class: "home-detail-card-subtitle" }, [
                novel.subtitle,
              ]),
            ]),
          ]),
          View({ class: "home-detail-badge" }, [novel.progress_text]),
        ]),
        View({ class: "home-novel-body" }, [
          View({ class: "home-detail-metrics" }, [
            For({
              each: novel.metrics,
              render(metric_) {
                const metric = ScraperDetailValue(metric_);
                return View({ class: "home-detail-metric" }, [
                  View({ class: "home-detail-metric-label" }, [
                    metric.label,
                  ]),
                  View({ class: "home-detail-metric-value" }, [
                    metric.value,
                  ]),
                ]);
              },
            }),
          ]),
          Show({
            when: novel.has_volumes,
            ok() {
              return View({ class: "home-novel-section" }, [
                View({ class: "home-novel-section-heading" }, [
                  View({ class: "home-novel-section-title" }, ["分卷"]),
                ]),
                View({ class: "home-novel-volume-list" }, [
                  For({
                    key: "key",
                    each: novel.volumes,
                    render(volume_) {
                      const volume = ScraperDetailValue(volume_);
                      return View({ class: "home-novel-volume" }, [
                        View({ class: "home-novel-volume-index" }, [
                          volume.index_text,
                        ]),
                        View(
                          {
                            class: "home-novel-volume-title",
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
          View({ class: "home-novel-section home-novel-chapters" }, [
            View({ class: "home-novel-section-heading" }, [
              View({ class: "home-novel-section-title" }, ["章节列表"]),
              View({ class: "home-novel-section-meta" }, [
                novel.progress_text,
              ]),
            ]),
            Show({
              when: novel.has_chapters,
              ok() {
                return View({ class: "home-novel-chapter-list" }, [
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
                return View({ class: "home-detail-empty" }, [
                  novel.empty_chapter_text,
                ]);
              },
            }),
            Show({
              when: novel.has_more_chapters,
              ok() {
                return View(
                  {
                    class: "home-novel-more-control",
                    attributes: { n: "scraper-novel-more-control" },
                  },
                  [
                    Button(
                      {
                        store: vm$.ui.btn_show_more_chapters$,
                        class: "dm-button--dashed",
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
        ],
      );
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
          ? "dm-button--choice-card is-selected"
          : "dm-button--choice-card",
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
      View({ class: "home-video-variant-indicator" }, [
        Show({
          when: selected_,
          ok() {
            return Timeless.Icon({ name: "check", size: 12 });
          },
        }),
      ]),
      View({ class: "home-video-supplement-main" }, [
        View(
          {
            class: "home-video-supplement-title",
            attributes: { title: variant.variant_key },
          },
          [variant.title],
        ),
        View({ class: "home-video-supplement-meta" }, [
          variant.meta_text || variant.variant_key,
        ]),
        Show({
          when: Boolean(variant.url),
          ok() {
            return View(
              {
                class: "home-video-supplement-url",
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
          return View({ class: "home-video-supplement-badge" }, ["默认"]);
        },
      }),
    ],
  );
}

function ScraperContentTextTrackItem(props) {
  const track = props.track || {};
  return View({ class: "home-video-subtitle-track" }, [
    View({ class: "home-video-supplement-item" }, [
      View({ class: "home-video-supplement-main" }, [
        View(
          {
            class: "home-video-supplement-title",
            attributes: { title: track.track_key },
          },
          [track.title],
        ),
        View({ class: "home-video-supplement-meta" }, [
          track.meta_text || track.track_key,
        ]),
      ]),
      View({ class: "home-video-supplement-badge" }, [
        `${track.sources.length} 个源`,
      ]),
    ]),
    Show({
      when: track.has_sources,
      ok() {
        return View({ class: "home-video-subtitle-sources" }, [
          For({
            key: "key",
            each: track.sources,
            render(source_) {
              const source = ScraperDetailValue(source_);
              return View({ class: "home-video-subtitle-source" }, [
                View({ class: "home-video-subtitle-source-type" }, [
                  source.title,
                ]),
                View(
                  {
                    class: "home-video-subtitle-source-url",
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
        return View({ class: "home-video-supplement" }, [
          View({ class: "home-video-supplement-heading" }, [
            View({ class: "home-video-supplement-label" }, [
              "ContentVideoVariant",
            ]),
            View({ class: "home-video-supplement-count" }, [
              String(detail.variants.length),
            ]),
          ]),
          View(
            {
              class:
                "home-video-supplement-list home-video-variant-list",
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
        return View({ class: "home-video-supplement" }, [
          View({ class: "home-video-supplement-heading" }, [
            View({ class: "home-video-supplement-label" }, [
              "ContentTextTrack",
            ]),
            View({ class: "home-video-supplement-count" }, [
              String(detail.text_tracks.length),
            ]),
          ]),
          View({ class: "home-video-supplement-list" }, [
            For({
              key: "key",
              each: detail.text_tracks,
              render(track_) {
                return ScraperContentTextTrackItem({
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
          class: "home-article-content is-html",
          content: article_body.content,
          attributes: {
            "data-content-format": "html",
          },
        });
      }
      return View(
        {
          class: `home-article-content is-${article_body.format || "text"}`,
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
      class: `home-detail-card home-typed-detail home-typed-detail-${detail.kind} dm-panel`,
    },
    [
      View({ class: "home-detail-card-heading" }, [
        View({ class: "home-detail-card-title-group" }, [
          View({ class: "home-detail-card-icon" }, [
            Timeless.Icon({ name: detail.icon, size: 18 }),
          ]),
          View({}, [
            View({ class: "home-detail-card-title" }, [detail.title]),
            View({ class: "home-detail-card-subtitle" }, [
              detail.model_name,
            ]),
          ]),
        ]),
        View({ class: "home-detail-badge" }, [detail.type_name]),
      ]),
      View({ class: "home-typed-detail-body" }, [
        Show({
          when: detail.subject.present,
          ok() {
            return View({ class: "home-detail-subject" }, [
              View({ class: "home-detail-subject-main" }, [
                View({ class: "home-detail-subject-title" }, [
                  detail.subject.title,
                ]),
                View({ class: "home-detail-subject-meta" }, [
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
          when: detail.images.length > 0,
          ok() {
            return View({ class: "home-detail-image-grid" }, [
              For({
                key: "key",
                each: detail.images,
                render(image_) {
                  const image = ScraperDetailValue(image_);
                  return View({ class: "home-detail-image-item" }, [
                    LazyImg({
                      class: "home-detail-image",
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
                        return View(
                          {
                            class: "home-detail-image-live-badge",
                            attributes: { title: "实况图" },
                          },
                          [
                            Timeless.Icon({ name: "play", size: 10 }),
                            image.live_photo_label,
                          ],
                        );
                      },
                    }),
                    View({ class: "home-detail-image-meta" }, [
                      image.meta,
                    ]),
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
  return View({ class: "home-json" }, [
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
        View({ class: "home-json-toggle-main" }, [
          View({ class: "home-json-icon" }, [
            Timeless.Icon({ name: "braces", size: 17 }),
          ]),
          View({}, [
            View({ class: "home-json-title" }, [
              vm$.state.json_toggle_text,
            ]),
            View({ class: "home-json-subtitle" }, [
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
            class: "home-json-body",
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
      return View({ class: "home-json home-raw-fetch" }, [
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
            View({ class: "home-json-toggle-main" }, [
              View({ class: "home-json-icon" }, [
                Timeless.Icon({ name: "braces", size: 17 }),
              ]),
              View({}, [
                View({ class: "home-json-title" }, ["抓取原始内容"]),
                View({ class: "home-json-subtitle" }, [
                  raw_result.meta_text,
                ]),
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
                class: "home-json-body",
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

function ScraperDownloadAssetRelation(props) {
  const resource = props.resource || {};
  const asset = props.asset || {};
  return View({ class: "home-download-relation" }, [
    View(
      {
        class: "home-download-relation-node",
        attributes: { title: resource.content_id },
      },
      [
        View({ class: "home-download-relation-node-type" }, ["Content"]),
        View({ class: "home-download-relation-node-value" }, [
          resource.content_id || "当前内容",
        ]),
      ],
    ),
    View({ class: "home-download-relation-arrow" }, [
      Timeless.Icon({ name: "arrow-right", size: 14 }),
    ]),
    View(
      {
        class: "home-download-relation-node is-asset",
        attributes: {
          title: [asset.kind, asset.role, asset.asset_key]
            .filter(Boolean)
            .join(" · "),
        },
      },
      [
        View({ class: "home-download-relation-node-type" }, [
          "ContentAsset",
        ]),
        View({ class: "home-download-relation-node-value" }, [
          [asset.kind, asset.role, asset.asset_key]
            .filter(Boolean)
            .join(" · "),
        ]),
        Show({
          when: Boolean(asset.subject_text),
          ok() {
            return View({ class: "home-download-relation-subject" }, [
              asset.subject_text,
            ]);
          },
        }),
      ],
    ),
    View({ class: "home-download-relation-arrow" }, [
      Timeless.Icon({ name: "arrow-right", size: 14 }),
    ]),
    View(
      {
        class: "home-download-relation-node",
        attributes: { title: resource.display_name },
      },
      [
        View({ class: "home-download-relation-node-type" }, [
          "DownloadResource",
        ]),
        View({ class: "home-download-relation-node-value" }, [
          resource.display_name,
        ]),
      ],
    ),
    View({ class: "home-download-relation-kind" }, [asset.relation]),
  ]);
}

function ScraperDownloadResourceItem(props) {
  const vm$ = props.store;
  const resource = props.resource || {};
  return View({ class: "home-download-resource" }, [
    View({ class: "home-download-resource-row" }, [
      View({ class: "home-download-resource-index" }, [
        resource.index_text,
      ]),
      View({ class: "home-download-resource-icon" }, [
        Timeless.Icon({ name: resource.icon, size: 18 }),
      ]),
      View({ class: "home-download-resource-main" }, [
        View(
          {
            class: "home-download-resource-name",
            attributes: { title: resource.display_name },
          },
          [resource.display_name],
        ),
        View({ class: "home-download-resource-meta" }, [
          resource.meta_text,
        ]),
      ]),
      Button(
        {
          store: vm$.ui.btn_download_resource$.bind(resource),
          attributes: {
            type: "button",
            title: `仅下载 ${resource.display_name}`,
            "aria-label": `下载 ${resource.display_name}`,
          },
        },
        [
          Timeless.Icon({ name: "download", size: 14 }),
          View({ class: "home-download-resource-action-label" }, [
            "下载",
          ]),
        ],
      ),
    ]),
    Show({
      when: resource.has_content_assets,
      ok() {
        return View({ class: "home-download-relations" }, [
          For({
            key: "key",
            each: resource.content_assets,
            render(asset_) {
              return ScraperDownloadAssetRelation({
                resource,
                asset: ScraperDetailValue(asset_),
              });
            },
          }),
        ]);
      },
    }),
  ]);
}

function HomeDownloadSection(props) {
  return View({ class: "home-download-section" }, [
    View({ class: "home-download-section-head" }, [
      View({ class: "home-download-section-title" }, [props.title]),
      View({ class: "home-download-section-count" }, [props.count]),
    ]),
    View({ class: "home-download-section-body" }, props.children || []),
  ]);
}

function ScraperDownloadInfo(props) {
  const vm$ = props.store;
  const download_info = vm$.state.download_info;
  const task = download_info.task;
  return Show({
    when: download_info.present,
    ok() {
      return View(
        { class: "home-card home-download-info dm-panel" },
        [
        View({ class: "home-card-heading" }, [
          View({ class: "home-card-title-group" }, [
            View({ class: "home-card-icon" }, [
              Timeless.Icon({ name: "download", size: 17 }),
            ]),
            View({}, [
              View({ class: "home-card-kicker" }, ["DOWNLOAD INFO"]),
              View({ class: "home-card-title" }, ["下载任务预览"]),
            ]),
          ]),
          View({ class: "home-card-actions" }, [
            Show({
              when: computed(download_info.badge_text, (text) => Boolean(text)),
              ok() {
                return View({ class: download_info.badge_class }, [
                  download_info.badge_text,
                ]);
              },
            }),
          ]),
        ]),
        View({ class: "home-download-body" }, [
          Show({
            when: computed(download_info.error, (error) => Boolean(error)),
            ok() {
              return View(
                {
                  class: "home-download-preview-error",
                  attributes: { role: "alert" },
                },
                [
                  Timeless.Icon({ name: "circle-alert", size: 15 }),
                  download_info.error,
                ],
              );
            },
          }),
          HomeDownloadSection({
            title: "资源文件",
            count: download_info.resource_count_text,
            children: [
              View({ class: "home-download-resource-list" }, [
                For({
                  key: "key",
                  each: download_info.resources,
                  render(resource_) {
                    return ScraperDownloadResourceItem({
                      store: vm$,
                      resource: ScraperDetailValue(resource_),
                    });
                  },
                }),
              ]),
            ],
          }),
          HomeDownloadSection({
            title: "下载任务",
            count: 1,
            children: [
              View({ class: "home-download-task-list" }, [
                View({ class: "home-download-task" }, [
                  View({ class: "home-download-task-id" }, [task.id_text]),
                  View({ class: "home-download-task-main" }, [
                    View(
                      {
                        class: "home-download-task-name",
                        attributes: { title: task.name },
                      },
                      [task.name],
                    ),
                    View({ class: "home-download-task-meta" }, [
                      task.meta_text || "任务将在确认后创建",
                    ]),
                  ]),
                  Show({
                    when: computed(task.status_text, (text) => Boolean(text)),
                    ok() {
                      return View({ class: "home-download-status" }, [
                        task.status_text,
                      ]);
                    },
                  }),
                ]),
              ]),
            ],
          }),
        ]),
        ],
      );
    },
  });
}

function ScraperResultActionBar(props) {
  const vm$ = props.store;
  return View(
    {
      class: "home-result-actions",
      attributes: {
        n: "result-actions",
        role: "toolbar",
        "aria-label": "抓取完成操作",
      },
    },
    [
      View(
        {
          class: "home-result-actions-summary",
          attributes: { n: "result-actions-summary" },
        },
        [
          View(
            {
              class: "home-result-actions-icon",
              attributes: { n: "result-actions-status-icon" },
            },
            [Timeless.Icon({ name: "check", size: 17 })],
          ),
          View(
            {
              class: "home-result-actions-copy",
              attributes: { n: "result-actions-copy" },
            },
            [
              View(
                {
                  class: "home-result-actions-title",
                  attributes: { n: "result-actions-title" },
                },
                ["抓取完成"],
              ),
              View(
                {
                  class: "home-result-actions-description",
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
          class: "home-result-actions-controls",
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
            [
              Timeless.Icon({ name: "hard-drive", size: 16 }),
              "三方下载",
            ],
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
      return View({ class: "home-card home-cache-card dm-panel" }, [
        View({ class: "home-card-heading" }, [
          View({ class: "home-card-title-group" }, [
            View({ class: "home-card-icon" }, [
              Timeless.Icon({ name: "file-stack", size: 17 }),
            ]),
            View({}, [
              View({ class: "home-card-kicker" }, ["CACHE"]),
              View({ class: "home-card-title" }, ["抓取缓存"]),
            ]),
          ]),
          View({ class: "home-cache-heading-actions" }, [
            View({ class: "home-cache-summary" }, [cache.summary_text]),
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
        View({ class: "home-cache-list" }, [
          For({
            key: "key",
            each: cache.entries,
            render(entry_) {
              const entry = ScraperDetailValue(entry_);
              return Button(
                {
                  store: vm$.ui.btn_cache_entry$.bind(entry),
                  class: "dm-button--list-row",
                  attributes: {
                    type: "button",
                    title: `查看缓存内容：${entry.name}`,
                    "aria-label": `查看缓存内容：${entry.name}`,
                  },
                },
                [
                  View({ class: "home-cache-entry-icon" }, [
                    Timeless.Icon({ name: "file", size: 16 }),
                  ]),
                  View({ class: "home-cache-entry-main" }, [
                    View(
                      {
                        class: "home-cache-entry-name",
                        attributes: { title: entry.name },
                      },
                      [entry.name],
                    ),
                    View(
                      {
                        class: "home-cache-entry-path",
                        attributes: { title: entry.path },
                      },
                      [entry.path],
                    ),
                  ]),
                  View({ class: "home-cache-entry-trailing" }, [
                    View({ class: "home-cache-entry-size" }, [
                      entry.size_text,
                    ]),
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
      View({ class: "home-cache-dialog-header" }, [
        View({ class: "home-cache-dialog-heading" }, [
          View({ class: "home-cache-dialog-title" }, [
            cache_content.title,
          ]),
          View({ class: "home-cache-dialog-meta" }, [
            cache_content.meta_text,
          ]),
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
          class: "home-cache-dialog-path",
          attributes: { title: cache_content.path },
        },
        [cache_content.path],
      ),
      View({ class: "home-cache-dialog-body" }, [
        Show({
          when: cache_content.loading,
          ok() {
            return View({ class: "home-cache-dialog-state" }, [
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
                return View(
                  {
                    class:
                      "home-cache-dialog-state home-cache-dialog-error",
                  },
                  [cache_content.error],
                );
              },
              else() {
                return View(
                  {
                    type: "pre",
                    class: "home-cache-dialog-content",
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

function ScraperPageResult(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.result_visible,
    ok() {
      return View(
        {
          class: "home-result",
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
              { class: "home-platform-status-list" },
              [
                For({
                  key: "render_key",
                  each: status.items,
                  render(item_) {
                    const item = ScraperDetailValue(item_);
                    return View({ class: item.status_class }, [
                      View({ class: "home-platform-status-dot" }),
                      View({ class: "home-platform-status-main" }, [
                        View({ class: "home-platform-status-head" }, [
                          View({ class: "home-platform-status-name" }, [
                            item.platform_name,
                          ]),
                          View({ class: "home-platform-status-value" }, [
                            item.status_text,
                          ]),
                        ]),
                        Show({
                          when: item.has_reason,
                          ok() {
                            return View(
                              {
                                class: "home-platform-status-reason",
                                attributes: { title: item.reason },
                              },
                              [item.reason],
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
          else() {
            return View(
              { class: "home-platform-status-empty" },
              ["正在连接 /ws/scraper"],
            );
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
          View({ class: "home-platform-status-dot" }),
          View({ class: "home-platform-status-trigger-label" }, [
            "平台状态",
          ]),
          View({ class: "home-platform-status-trigger-summary" }, [
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
      return View({
        class: "home-fetch-progress",
        attributes: {
          role: "progressbar",
          "aria-label": "抓取进度",
          "aria-valuemin": "0",
          "aria-valuemax": "100",
          "aria-valuenow": vm$.state.progress_percent,
        },
      }, [
        View({ class: "home-fetch-progress-head" }, [
          View({ class: "home-fetch-progress-stage" }, [
            vm$.state.progress_stage_text,
          ]),
          View({ class: "home-fetch-progress-right" }, [
            Show({
              when: vm$.state.progress_updated_text,
              ok() {
                return View({ class: "home-fetch-progress-updated" }, [
                  vm$.state.progress_updated_text,
                ]);
              },
            }),
            Show({
              when: vm$.state.progress_has_percent,
              ok() {
                return View({ class: "home-fetch-progress-percent" }, [
                  vm$.state.progress_percent_text,
                ]);
              },
            }),
          ]),
        ]),
        View({ class: "home-fetch-progress-message" }, [
          vm$.state.progress_message,
        ]),
        View({ class: "home-fetch-progress-track" }, [
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
            return View({ class: "home-fetch-progress-count" }, [
              vm$.state.progress_count_text,
            ]);
          },
        }),
      ]);
    },
  });
}

export default ScraperPageView;

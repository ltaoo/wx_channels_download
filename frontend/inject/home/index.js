/// <reference path="../utils.js" />
/// <reference path="../components.js" />
/// <reference path="model.js" />
/**
 * @file Scraper fetch page rendering.
 */
function HomePopover(props, children) {
  const {
    store,
    content,
    onTriggerMouseEnter,
    onTriggerMouseLeave,
    ...content_props
  } = props;
  const presence_state_ = refobj(props.store.presence.state);
  const was_exiting_ = ref(false);
  const unlistens = [
    props.store.presence.onStateChange((state) => {
      presence_state_.as(state);
      if (state.exit) {
        was_exiting_.as(true);
      }
      if (state.mounted) {
        was_exiting_.as(false);
      }
    }),
  ];

  return Timeless.ui.PopoverPrimitive.Root(
    {
      onUnmounted() {
        unlistens.forEach((unlisten) => {
          if (typeof unlisten === "function") {
            unlisten();
          }
        });
      },
    },
    [
      View(
        {
          class: "wx-home-popover-hover-trigger",
          onMounted(event) {
            const trigger_root = event.target;
            const trigger_children =
              typeof trigger_root.getChildren === "function"
                ? trigger_root.getChildren()
                : [];
            const trigger =
              trigger_children.find(
                (child) => child && child.getType() === "view",
              ) ||
              trigger_children[0] ||
              trigger_root;
            store.popper.setReference(
              {
                $el: trigger,
                getRect: () => trigger.getBoundingClientRect(),
              },
              { force: true },
            );
          },
          onMouseEnter(event) {
            if (typeof onTriggerMouseEnter === "function") {
              onTriggerMouseEnter(event);
            }
          },
          onMouseLeave(event) {
            if (typeof onTriggerMouseLeave === "function") {
              onTriggerMouseLeave(event);
            }
          },
        },
        children,
      ),
      Timeless.ui.PopoverPrimitive.Portal({ store }, [
        Timeless.ui.PopoverPrimitive.Content(
          {
            ...content_props,
            store,
            zIndex: 9999,
            class: computed(presence_state_, (state) => {
              const enter_class = "animate-in fade-in-0 zoom-in-95";
              const exit_class = "animate-out fade-out-0 zoom-out-95";
              return [
                state.enter ? enter_class : "",
                state.exit ? exit_class : "",
                !state.mounted && was_exiting_.value ? exit_class : "",
              ]
                .filter(Boolean)
                .join(" ");
            }),
          },
          content,
        ),
      ]),
    ],
  );
}

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
      View({ class: "wx-home-actions" }, [
        Show({
          when: vm$.state.loading,
          ok() {
            return Button(
              {
                class: "wx-content-action wx-home-interrupt",
                disabled: vm$.state.interrupt_disabled,
                attributes: { type: "button" },
                onClick() {
                  vm$.methods.interruptFetch();
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
            return View({ class: "wx-home-idle-actions" }, [
              Button(
                {
                  class: "wx-content-action wx-home-submit",
                  disabled: vm$.state.submit_disabled,
                  attributes: { type: "button" },
                  onClick() {
                    vm$.methods.submit();
                  },
                },
                [vm$.state.submit_button_text],
              ),
            ]);
          },
        }),
      ]),
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

function HomeDecodeHTMLText(value) {
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

function HomeContentCard(props) {
  const vm$ = props.store;
  const content = vm$.state.content;
  const account = vm$.state.account;
  const description = computed(content.description, HomeDecodeHTMLText);
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
      View({ class: "wx-home-card-actions" }, [
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
        Show({
          when: content.content_url,
          ok() {
            return Button(
              {
                class: "wx-content-action wx-home-open-content",
                attributes: { type: "button", title: "打开原内容" },
                onClick() {
                  vm$.methods.openContent();
                },
              },
              [
                Timeless.Icon({ name: "external-link", size: 15 }),
                View({ class: "wx-content-action-label" }, ["打开原内容"]),
              ],
            );
          },
        }),
      ]),
    ]),
    View({ class: "wx-home-content-card-body" }, [
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
        View({ class: "wx-home-content-meta" }, [
          Timeless.Icon({ name: "clock3", size: 14 }),
          content.publish_time_text,
        ]),
        Show({
          when: content.show_description,
          ok() {
            return View({ class: "wx-home-content-description" }, [
              description,
            ]);
          },
        }),
        HomeContentAccount({ account }),
      ]),
    ]),
  ]);
}

function HomeContentRelationNode(props) {
  const vm$ = props.store;
  const node = props.node || {};
  return Button(
    {
      class: "wx-home-content-relation-node",
      disabled: !node.has_url,
      attributes: {
        type: "button",
        title: node.has_url ? node.url : node.id,
      },
      onClick() {
        vm$.methods.openDetailURL(node.url);
      },
    },
    [
      View({ class: "wx-home-content-relation-node-type" }, [node.type_name]),
      View({ class: "wx-home-content-relation-node-title" }, [node.title]),
      View({ class: "wx-home-content-relation-node-meta" }, [node.meta_text]),
    ],
  );
}

function HomeContentRelations(props) {
  const vm$ = props.store;
  const relations = vm$.state.content_relations;
  return Show({
    when: relations.present,
    ok() {
      return View({ class: "wx-home-card wx-home-content-relations" }, [
        View({ class: "wx-home-card-heading" }, [
          View({ class: "wx-home-card-title-group" }, [
            View({ class: "wx-home-card-icon" }, [
              Timeless.Icon({ name: "git-branch", size: 17 }),
            ]),
            View({}, [
              View({ class: "wx-home-card-kicker" }, ["RELATIONS"]),
              View({ class: "wx-home-card-title" }, ["内容关联"]),
            ]),
          ]),
          View({ class: "wx-home-detail-badge" }, [relations.count_text]),
        ]),
        View({ class: "wx-home-content-relation-list" }, [
          For({
            key: "key",
            each: relations.items,
            render(relation_) {
              const relation = HomeDetailValue(relation_);
              return View({ class: "wx-home-content-relation" }, [
                HomeContentRelationNode({ store: vm$, node: relation.source }),
                View({ class: "wx-home-content-relation-edge" }, [
                  Timeless.Icon({ name: "arrow-right", size: 15 }),
                  View({ class: "wx-home-content-relation-edge-name" }, [
                    relation.type_name,
                  ]),
                  View({ class: "wx-home-content-relation-edge-type" }, [
                    relation.type,
                  ]),
                ]),
                HomeContentRelationNode({ store: vm$, node: relation.target }),
              ]);
            },
          }),
        ]),
      ]);
    },
  });
}

function HomeContentAccount(props) {
  const account = props.account;
  return Show({
    when: account.present,
    ok() {
      return View({ class: "wx-home-content-account" }, [
        HomeAccountAvatar({ account }),
        View({ class: "wx-home-content-account-main" }, [
          View({ class: "wx-home-account-name" }, [account.nickname]),
        ]),
      ]);
    },
  });
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

function HomeDetailValue(value_) {
  return value_ && value_.value !== undefined ? value_.value : value_;
}

function HomeVideoDetailMedia(props) {
  const vm$ = props.store;
  const detail = props.detail || {};
  const media = detail.media || {};
  return Show({
    when: media.present,
    ok() {
      return View(
        {
          class: [
            "wx-home-detail-video-frame",
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
                  class: "wx-home-detail-video",
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
                      ".wx-home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.add("is-playing");
                    }
                  },
                  onPause(event) {
                    const root = event.currentTarget.closest(
                      ".wx-home-detail-video-frame",
                    );
                    if (root) {
                      root.classList.remove("is-playing");
                    }
                  },
                  onEnded(event) {
                    const root = event.currentTarget.closest(
                      ".wx-home-detail-video-frame",
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
              return Img({
                class: "wx-home-detail-video-cover",
                src: media.cover_url,
                alt: detail.title,
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
          Show({
            when: Boolean(media.has_video || detail.link_url),
            ok() {
              return Button(
                {
                  class: "wx-home-detail-video-play",
                  attributes: {
                    type: "button",
                    title: media.has_video ? "播放视频" : "打开视频地址",
                    "aria-label": media.has_video ? "播放视频" : "打开视频地址",
                  },
                  onClick(event) {
                    event.stopPropagation();
                    const trigger = event.currentTarget || event.target;
                    const root =
                      trigger && typeof trigger.closest === "function"
                        ? trigger.closest(".wx-home-detail-video-frame")
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
              );
            },
          }),
        ],
      );
    },
  });
}

function HomeNovelChapterItem(props) {
  const vm$ = props.store;
  const chapter = props.chapter || {};
  const children = [
    View({ class: "wx-home-novel-chapter-index" }, [chapter.index_text]),
    View({ class: "wx-home-novel-chapter-main" }, [
      View(
        {
          class: "wx-home-novel-chapter-title",
          attributes: { title: chapter.title },
        },
        [chapter.title],
      ),
      Show({
        when: Boolean(chapter.meta_text),
        ok() {
          return View({ class: "wx-home-novel-chapter-meta" }, [
            chapter.meta_text,
          ]);
        },
      }),
    ]),
    Show({
      when: Boolean(chapter.url),
      ok() {
        return View({ class: "wx-home-novel-chapter-link" }, [
          Timeless.Icon({ name: "external-link", size: 13 }),
        ]);
      },
    }),
  ];
  if (!chapter.url) {
    return View({ class: "wx-home-novel-chapter" }, children);
  }
  return Button(
    {
      class: "wx-home-novel-chapter is-link",
      attributes: { type: "button", title: "打开章节" },
      onClick() {
        vm$.methods.openDetailURL(chapter.url);
      },
    },
    children,
  );
}

function HomeNovelDetails(props) {
  const vm$ = props.store;
  const novel = vm$.state.content_details.novel;
  return Show({
    when: novel.present,
    ok() {
      return View({ class: "wx-home-detail-card wx-home-novel-detail" }, [
        View({ class: "wx-home-detail-card-heading" }, [
          View({ class: "wx-home-detail-card-title-group" }, [
            View({ class: "wx-home-detail-card-icon" }, [
              Timeless.Icon({ name: "file-stack", size: 18 }),
            ]),
            View({}, [
              View({ class: "wx-home-detail-card-title" }, [novel.title]),
              View({ class: "wx-home-detail-card-subtitle" }, [
                novel.subtitle,
              ]),
            ]),
          ]),
          View({ class: "wx-home-detail-badge" }, [novel.progress_text]),
        ]),
        View({ class: "wx-home-novel-body" }, [
          View({ class: "wx-home-detail-metrics" }, [
            For({
              each: novel.metrics,
              render(metric_) {
                const metric = HomeDetailValue(metric_);
                return View({ class: "wx-home-detail-metric" }, [
                  View({ class: "wx-home-detail-metric-label" }, [
                    metric.label,
                  ]),
                  View({ class: "wx-home-detail-metric-value" }, [
                    metric.value,
                  ]),
                ]);
              },
            }),
          ]),
          Show({
            when: novel.has_volumes,
            ok() {
              return View({ class: "wx-home-novel-section" }, [
                View({ class: "wx-home-novel-section-heading" }, [
                  View({ class: "wx-home-novel-section-title" }, ["分卷"]),
                ]),
                View({ class: "wx-home-novel-volume-list" }, [
                  For({
                    key: "key",
                    each: novel.volumes,
                    render(volume_) {
                      const volume = HomeDetailValue(volume_);
                      return View({ class: "wx-home-novel-volume" }, [
                        View({ class: "wx-home-novel-volume-index" }, [
                          volume.index_text,
                        ]),
                        View(
                          {
                            class: "wx-home-novel-volume-title",
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
          View({ class: "wx-home-novel-section wx-home-novel-chapters" }, [
            View({ class: "wx-home-novel-section-heading" }, [
              View({ class: "wx-home-novel-section-title" }, ["章节列表"]),
              View({ class: "wx-home-novel-section-meta" }, [
                novel.progress_text,
              ]),
            ]),
            Show({
              when: novel.has_chapters,
              ok() {
                return View({ class: "wx-home-novel-chapter-list" }, [
                  For({
                    key: "key",
                    each: novel.chapters,
                    render(chapter_) {
                      return HomeNovelChapterItem({
                        store: vm$,
                        chapter: HomeDetailValue(chapter_),
                      });
                    },
                  }),
                ]);
              },
              else() {
                return View({ class: "wx-home-detail-empty" }, [
                  novel.empty_chapter_text,
                ]);
              },
            }),
            Show({
              when: novel.has_more_chapters,
              ok() {
                return Button(
                  {
                    class: "wx-home-novel-more",
                    attributes: { type: "button" },
                    onClick() {
                      vm$.methods.showMoreChapters();
                    },
                  },
                  [novel.more_chapters_text],
                );
              },
            }),
          ]),
        ]),
      ]);
    },
  });
}

function HomeVideoVariantItem(props) {
  const variant = props.variant || {};
  return View({ class: "wx-home-video-supplement-item" }, [
    View({ class: "wx-home-video-supplement-main" }, [
      View(
        {
          class: "wx-home-video-supplement-title",
          attributes: { title: variant.variant_key },
        },
        [variant.title],
      ),
      View({ class: "wx-home-video-supplement-meta" }, [
        variant.meta_text || variant.variant_key,
      ]),
      Show({
        when: Boolean(variant.url),
        ok() {
          return View(
            {
              class: "wx-home-video-supplement-url",
              attributes: { title: variant.url },
            },
            [variant.url],
          );
        },
      }),
    ]),
    View({ class: "wx-home-video-supplement-badge" }, [variant.default_text]),
  ]);
}

function HomeVideoSubtitleTrackItem(props) {
  const track = props.track || {};
  return View({ class: "wx-home-video-subtitle-track" }, [
    View({ class: "wx-home-video-supplement-item" }, [
      View({ class: "wx-home-video-supplement-main" }, [
        View(
          {
            class: "wx-home-video-supplement-title",
            attributes: { title: track.track_key },
          },
          [track.title],
        ),
        View({ class: "wx-home-video-supplement-meta" }, [
          track.meta_text || track.track_key,
        ]),
      ]),
      View({ class: "wx-home-video-supplement-badge" }, [
        `${track.sources.length} 个源`,
      ]),
    ]),
    Show({
      when: track.has_sources,
      ok() {
        return View({ class: "wx-home-video-subtitle-sources" }, [
          For({
            key: "key",
            each: track.sources,
            render(source_) {
              const source = HomeDetailValue(source_);
              return View({ class: "wx-home-video-subtitle-source" }, [
                View({ class: "wx-home-video-subtitle-source-type" }, [
                  source.title,
                ]),
                View(
                  {
                    class: "wx-home-video-subtitle-source-url",
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

function HomeVideoSupplements(props) {
  const detail = props.detail || {};
  return Fragment({}, [
    Show({
      when: detail.has_variants,
      ok() {
        return View({ class: "wx-home-video-supplement" }, [
          View({ class: "wx-home-video-supplement-heading" }, [
            View({ class: "wx-home-video-supplement-label" }, [
              "ContentVideoVariant",
            ]),
            View({ class: "wx-home-video-supplement-count" }, [
              String(detail.variants.length),
            ]),
          ]),
          View({ class: "wx-home-video-supplement-list" }, [
            For({
              key: "key",
              each: detail.variants,
              render(variant_) {
                return HomeVideoVariantItem({
                  variant: HomeDetailValue(variant_),
                });
              },
            }),
          ]),
        ]);
      },
    }),
    Show({
      when: detail.has_subtitle_tracks,
      ok() {
        return View({ class: "wx-home-video-supplement" }, [
          View({ class: "wx-home-video-supplement-heading" }, [
            View({ class: "wx-home-video-supplement-label" }, [
              "ContentVideoSubtitleTrack",
            ]),
            View({ class: "wx-home-video-supplement-count" }, [
              String(detail.subtitle_tracks.length),
            ]),
          ]),
          View({ class: "wx-home-video-supplement-list" }, [
            For({
              key: "key",
              each: detail.subtitle_tracks,
              render(track_) {
                return HomeVideoSubtitleTrackItem({
                  track: HomeDetailValue(track_),
                });
              },
            }),
          ]),
        ]);
      },
    }),
  ]);
}

function HomeArticleBody(props) {
  const article_body = props.article_body || {};
  return Show({
    when: article_body.present,
    ok() {
      if (article_body.is_html) {
        return Timeless.RichText({
          class: "wx-home-article-content is-html",
          content: article_body.content,
          attributes: {
            "data-content-format": "html",
          },
        });
      }
      return View(
        {
          class: `wx-home-article-content is-${article_body.format || "text"}`,
        },
        [article_body.content],
      );
    },
  });
}

function HomeTypedContentDetail(props) {
  const vm$ = props.store;
  const detail = props.detail || {};
  return View(
    {
      class: `wx-home-detail-card wx-home-typed-detail wx-home-typed-detail-${detail.kind}`,
    },
    [
      View({ class: "wx-home-detail-card-heading" }, [
        View({ class: "wx-home-detail-card-title-group" }, [
          View({ class: "wx-home-detail-card-icon" }, [
            Timeless.Icon({ name: detail.icon, size: 18 }),
          ]),
          View({}, [
            View({ class: "wx-home-detail-card-title" }, [detail.title]),
            View({ class: "wx-home-detail-card-subtitle" }, [
              detail.model_name,
            ]),
          ]),
        ]),
        View({ class: "wx-home-detail-badge" }, [detail.type_name]),
      ]),
      View({ class: "wx-home-typed-detail-body" }, [
        Show({
          when: detail.subject.present,
          ok() {
            return View({ class: "wx-home-detail-subject" }, [
              View({ class: "wx-home-detail-subject-main" }, [
                View({ class: "wx-home-detail-subject-title" }, [
                  detail.subject.title,
                ]),
                View({ class: "wx-home-detail-subject-meta" }, [
                  detail.subject.meta_text,
                ]),
              ]),
              Show({
                when: detail.subject.has_url,
                ok() {
                  return Button(
                    {
                      class: "wx-content-action wx-home-detail-subject-open",
                      attributes: { type: "button" },
                      onClick() {
                        vm$.methods.openDetailURL(detail.subject.url);
                      },
                    },
                    [Timeless.Icon({ name: "external-link", size: 13 }), "打开"],
                  );
                },
              }),
            ]);
          },
        }),
        HomeVideoDetailMedia({ store: vm$, detail }),
        HomeVideoSupplements({ detail }),
        HomeArticleBody({ article_body: detail.article_body }),
        Show({
          when: detail.images.length > 0,
          ok() {
            return View({ class: "wx-home-detail-image-grid" }, [
              For({
                key: "key",
                each: detail.images,
                render(image_) {
                  const image = HomeDetailValue(image_);
                  return View({ class: "wx-home-detail-image-item" }, [
                    Img({
                      class: "wx-home-detail-image",
                      src: image.url,
                      alt: "",
                      attributes: {
                        loading: "lazy",
                        referrerpolicy: "no-referrer",
                      },
                      onError(event) {
                        event.target.style.display = "none";
                      },
                    }),
                    Show({
                      when: image.is_live_photo,
                      ok() {
                        return View(
                          {
                            class: "wx-home-detail-image-live-badge",
                            attributes: { title: "实况图" },
                          },
                          [
                            Timeless.Icon({ name: "play", size: 10 }),
                            image.live_photo_label,
                          ],
                        );
                      },
                    }),
                    View({ class: "wx-home-detail-image-meta" }, [image.meta]),
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

function HomeContentDetails(props) {
  const vm$ = props.store;
  const details = vm$.state.content_details;
  return Show({
    when: details.present,
    ok() {
      return Fragment({}, [
        HomeNovelDetails({ store: vm$ }),
        For({
          key: "key",
          each: details.items,
          render(detail_) {
            return HomeTypedContentDetail({
              store: vm$,
              detail: HomeDetailValue(detail_),
            });
          },
        }),
      ]);
    },
  });
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

function HomeDownloadAssetRelation(props) {
  const resource = props.resource || {};
  const asset = props.asset || {};
  return View({ class: "wx-home-download-relation" }, [
    View(
      {
        class: "wx-home-download-relation-node",
        attributes: { title: resource.content_id },
      },
      [
        View({ class: "wx-home-download-relation-node-type" }, ["Content"]),
        View({ class: "wx-home-download-relation-node-value" }, [
          resource.content_id || "当前内容",
        ]),
      ],
    ),
    View({ class: "wx-home-download-relation-arrow" }, [
      Timeless.Icon({ name: "arrow-right", size: 14 }),
    ]),
    View(
      {
        class: "wx-home-download-relation-node is-asset",
        attributes: {
          title: [asset.kind, asset.role, asset.asset_key]
            .filter(Boolean)
            .join(" · "),
        },
      },
      [
        View({ class: "wx-home-download-relation-node-type" }, [
          "ContentAsset",
        ]),
        View({ class: "wx-home-download-relation-node-value" }, [
          [asset.kind, asset.role, asset.asset_key].filter(Boolean).join(" · "),
        ]),
        Show({
          when: Boolean(asset.subject_text),
          ok() {
            return View({ class: "wx-home-download-relation-subject" }, [
              asset.subject_text,
            ]);
          },
        }),
      ],
    ),
    View({ class: "wx-home-download-relation-arrow" }, [
      Timeless.Icon({ name: "arrow-right", size: 14 }),
    ]),
    View(
      {
        class: "wx-home-download-relation-node",
        attributes: { title: resource.display_name },
      },
      [
        View({ class: "wx-home-download-relation-node-type" }, [
          "DownloadResource",
        ]),
        View({ class: "wx-home-download-relation-node-value" }, [
          resource.display_name,
        ]),
      ],
    ),
    View({ class: "wx-home-download-relation-kind" }, [asset.relation]),
  ]);
}

function HomeDownloadResourceItem(props) {
  const resource = props.resource || {};
  return View({ class: "wx-home-download-resource" }, [
    View({ class: "wx-home-download-resource-row" }, [
      View({ class: "wx-home-download-resource-index" }, [
        resource.index_text,
      ]),
      View({ class: "wx-home-download-resource-icon" }, [
        Timeless.Icon({ name: resource.icon, size: 18 }),
      ]),
      View({ class: "wx-home-download-resource-main" }, [
        View(
          {
            class: "wx-home-download-resource-name",
            attributes: { title: resource.display_name },
          },
          [resource.display_name],
        ),
        View({ class: "wx-home-download-resource-meta" }, [
          resource.meta_text,
        ]),
      ]),
      View({ class: "wx-home-download-status" }, [resource.status_text]),
    ]),
    Show({
      when: resource.has_content_assets,
      ok() {
        return View({ class: "wx-home-download-relations" }, [
          For({
            key: "key",
            each: resource.content_assets,
            render(asset_) {
              return HomeDownloadAssetRelation({
                resource,
                asset: HomeDetailValue(asset_),
              });
            },
          }),
        ]);
      },
    }),
  ]);
}

function HomeDownloadSection(props) {
  return View({ class: "wx-home-download-section" }, [
    View({ class: "wx-home-download-section-head" }, [
      View({ class: "wx-home-download-section-title" }, [props.title]),
      View({ class: "wx-home-download-section-count" }, [props.count]),
    ]),
    View({ class: "wx-home-download-section-body" }, props.children || []),
  ]);
}

function HomeDownloadInfo(props) {
  const vm$ = props.store;
  const download_info = vm$.state.download_info;
  const task = download_info.task;
  return Show({
    when: download_info.present,
    ok() {
      return View({ class: "wx-home-card wx-home-download-info" }, [
        View({ class: "wx-home-card-heading" }, [
          View({ class: "wx-home-card-title-group" }, [
            View({ class: "wx-home-card-icon" }, [
              Timeless.Icon({ name: "download", size: 17 }),
            ]),
            View({}, [
              View({ class: "wx-home-card-kicker" }, ["DOWNLOAD INFO"]),
              View({ class: "wx-home-card-title" }, ["下载任务预览"]),
            ]),
          ]),
          View({ class: "wx-home-download-preview-badge" }, ["尚未创建"]),
        ]),
        View({ class: "wx-home-download-body" }, [
          HomeDownloadSection({
            title: "资源文件",
            count: download_info.resource_count_text,
            children: [
              View({ class: "wx-home-download-resource-list" }, [
                For({
                  key: "key",
                  each: download_info.resources,
                  render(resource_) {
                    return HomeDownloadResourceItem({
                      resource: HomeDetailValue(resource_),
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
              View({ class: "wx-home-download-task-list" }, [
                View({ class: "wx-home-download-task" }, [
                  View({ class: "wx-home-download-task-id" }, [task.id_text]),
                  View({ class: "wx-home-download-task-main" }, [
                    View(
                      {
                        class: "wx-home-download-task-name",
                        attributes: { title: task.name },
                      },
                      [task.name],
                    ),
                    View({ class: "wx-home-download-task-meta" }, [
                      task.meta_text || "任务将在确认后创建",
                    ]),
                  ]),
                  View({ class: "wx-home-download-status" }, [
                    task.status_text,
                  ]),
                ]),
              ]),
            ],
          }),
        ]),
      ]);
    },
  });
}

function HomeResultActions(props) {
  const vm$ = props.store;
  return View({ class: "wx-home-result-actions" }, [
    Button(
      {
        class: "wx-content-action wx-home-refresh",
        disabled: vm$.state.cache_action_disabled,
        attributes: {
          type: "button",
          title: "忽略现有缓存并重新抓取",
        },
        onClick() {
          vm$.methods.forceRefresh();
        },
      },
      ["重新抓取"],
    ),
  ]);
}

function HomeCacheCard(props) {
  const vm$ = props.store;
  const cache = vm$.state.cache;
  return Show({
    when: cache.present,
    ok() {
      return View({ class: "wx-home-card wx-home-cache-card" }, [
        View({ class: "wx-home-card-heading" }, [
          View({ class: "wx-home-card-title-group" }, [
            View({ class: "wx-home-card-icon" }, [
              Timeless.Icon({ name: "file-stack", size: 17 }),
            ]),
            View({}, [
              View({ class: "wx-home-card-kicker" }, ["CACHE"]),
              View({ class: "wx-home-card-title" }, ["抓取缓存"]),
            ]),
          ]),
          View({ class: "wx-home-cache-heading-actions" }, [
            View({ class: "wx-home-cache-summary" }, [cache.summary_text]),
            Button(
              {
                class: "wx-content-action wx-home-cache-action",
                disabled: vm$.state.cache_action_disabled,
                attributes: {
                  type: "button",
                  title: "清除该 URL 的抓取缓存",
                },
                onClick() {
                  vm$.methods.clearFetchCache();
                },
              },
              [
                Timeless.Icon({ name: "trash2", size: 14 }),
                vm$.state.cache_button_text,
              ],
            ),
          ]),
        ]),
        View({ class: "wx-home-cache-list" }, [
          For({
            key: "key",
            each: cache.entries,
            render(entry_) {
              const entry = HomeDetailValue(entry_);
              return Button(
                {
                  class: "wx-home-cache-entry",
                  attributes: {
                    type: "button",
                    title: `查看缓存内容：${entry.name}`,
                    "aria-label": `查看缓存内容：${entry.name}`,
                  },
                  onClick() {
                    vm$.methods.openCacheContent(entry);
                  },
                },
                [
                  View({ class: "wx-home-cache-entry-icon" }, [
                    Timeless.Icon({ name: "file", size: 16 }),
                  ]),
                  View({ class: "wx-home-cache-entry-main" }, [
                    View(
                      {
                        class: "wx-home-cache-entry-name",
                        attributes: { title: entry.name },
                      },
                      [entry.name],
                    ),
                    View(
                      {
                        class: "wx-home-cache-entry-path",
                        attributes: { title: entry.path },
                      },
                      [entry.path],
                    ),
                  ]),
                  View({ class: "wx-home-cache-entry-trailing" }, [
                    View({ class: "wx-home-cache-entry-size" }, [
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

function HomeCacheContentDialog(props) {
  const vm$ = props.store;
  const cache_content = vm$.state.cache_content;
  return Dialog(
    {
      store: vm$.ui.cache_content_dialog,
      class: "wx-home-cache-dialog",
    },
    [
      View({ class: "wx-home-cache-dialog-header" }, [
        View({ class: "wx-home-cache-dialog-heading" }, [
          View({ class: "wx-home-cache-dialog-title" }, [
            cache_content.title,
          ]),
          View({ class: "wx-home-cache-dialog-meta" }, [
            cache_content.meta_text,
          ]),
        ]),
        Button(
          {
            class: "wx-home-cache-dialog-close",
            attributes: { type: "button", "aria-label": "关闭缓存内容" },
            onClick() {
              vm$.methods.closeCacheContent();
            },
          },
          ["×"],
        ),
      ]),
      View(
        {
          class: "wx-home-cache-dialog-path",
          attributes: { title: cache_content.path },
        },
        [cache_content.path],
      ),
      View({ class: "wx-home-cache-dialog-body" }, [
        Show({
          when: cache_content.loading,
          ok() {
            return View({ class: "wx-home-cache-dialog-state" }, [
              View({ class: "weui-loading" }),
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
                      "wx-home-cache-dialog-state wx-home-cache-dialog-error",
                  },
                  [cache_content.error],
                );
              },
              else() {
                return View(
                  {
                    type: "pre",
                    class: "wx-home-cache-dialog-content",
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

function HomePageResult(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.has_result,
    ok() {
      return View({ class: "wx-home-result" }, [
        HomeContentCard({ store: vm$ }),
        HomeContentRelations({ store: vm$ }),
        HomeContentDetails({ store: vm$ }),
        HomeDownloadInfo({ store: vm$ }),
        HomeRawJSON({ store: vm$ }),
        HomeResultActions({ store: vm$ }),
        HomeCacheCard({ store: vm$ }),
      ]);
    },
  });
}

function HomePlatformStatus(props) {
  const vm$ = props.store;
  const status = vm$.state.platform_status;
  return HomePopover(
    {
      store: vm$.ui.platform_status_popover,
      side: "bottom",
      align: "end",
      onTriggerMouseEnter() {
        vm$.methods.showPlatformStatusPopover();
      },
      onTriggerMouseLeave() {
        vm$.methods.schedulePlatformStatusPopoverHide();
      },
      content: [
        View(
          {
            class: "wx-home-platform-status-popover",
            onMouseEnter() {
              vm$.methods.showPlatformStatusPopover();
            },
            onMouseLeave() {
              vm$.methods.schedulePlatformStatusPopoverHide();
            },
          },
          [
            Show({
              when: status.has_items,
              ok() {
                return View({ class: "wx-home-platform-status-list" }, [
                  For({
                    key: "render_key",
                    each: status.items,
                    render(item_) {
                      const item = HomeDetailValue(item_);
                      return View({ class: item.status_class }, [
                        View({ class: "wx-home-platform-status-dot" }),
                        View({ class: "wx-home-platform-status-main" }, [
                          View({ class: "wx-home-platform-status-head" }, [
                            View({ class: "wx-home-platform-status-name" }, [
                              item.platform_name,
                            ]),
                            View({ class: "wx-home-platform-status-value" }, [
                              item.status_text,
                            ]),
                          ]),
                          Show({
                            when: item.has_reason,
                            ok() {
                              return View(
                                {
                                  class: "wx-home-platform-status-reason",
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
                ]);
              },
              else() {
                return View({ class: "wx-home-platform-status-empty" }, [
                  "正在连接 /ws/scraper",
                ]);
              },
            }),
          ],
        ),
      ],
    },
    [
      Button(
        {
          class: status.trigger_class,
          attributes: {
            type: "button",
            title: "查看平台状态",
            "aria-label": "查看平台状态",
          },
        },
        [
          View({ class: "wx-home-platform-status-dot" }),
          View({ class: "wx-home-platform-status-trigger-label" }, [
            "平台状态",
          ]),
          View({ class: "wx-home-platform-status-trigger-summary" }, [
            status.summary,
          ]),
        ],
      ),
    ],
  );
}

function HomePageHeader(props) {
  const vm$ = props.store;
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
      View({ class: "wx-home-header-actions" }, [
        HomePlatformStatus({ store: vm$ }),
      ]),
    ]),
  ]);
}

function HomeFetchProgress(props) {
  const vm$ = props.store;
  return Show({
    when: vm$.state.progress_visible,
    ok() {
      return View({ class: "wx-home-fetch-progress" }, [
        View({ class: "wx-home-fetch-progress-meta" }, [
          View({ class: "wx-home-fetch-progress-count" }, [
            vm$.state.progress_count_text,
          ]),
          View({ class: "wx-home-fetch-progress-percent" }, [
            vm$.state.progress_percent_text,
          ]),
        ]),
        View({ class: "wx-home-fetch-progress-track" }, [
          View({
            class: "wx-home-fetch-progress-bar",
            style: computed(vm$.state.progress_percent, (percent) => ({
              width: `${percent}%`,
            })),
          }),
        ]),
      ]);
    },
  });
}

function HomePageView(props) {
  const vm$ = props.store;
  return View({
    class: "wx-content-page wx-home-page",
    onMounted() {
      vm$.methods.connectProgress();
    },
    onUnmounted() {
      vm$.methods.dispose();
    },
  }, [
    HomePageHeader({ store: vm$ }),
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
        HomeFetchProgress({ store: vm$ }),
        HomePageResult({ store: vm$ }),
      ]),
    ]),
    HomeCacheContentDialog({ store: vm$ }),
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

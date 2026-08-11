/// <reference path="../utils.js" />
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

  return Timeless.weui.PopoverPrimitive.Root(
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
      Timeless.weui.PopoverPrimitive.Portal({ store }, [
        Timeless.weui.PopoverPrimitive.Content(
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

function HomeDetailValue(value_) {
  return value_ && value_.value !== undefined ? value_.value : value_;
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
            View({ class: "wx-home-detail-card-subtitle" }, [detail.type]),
          ]),
        ]),
        View({ class: "wx-home-detail-badge" }, [detail.type_name]),
      ]),
      View({ class: "wx-home-typed-detail-body" }, [
        Show({
          when: detail.fields.length > 0,
          ok() {
            return View({ class: "wx-home-detail-field-grid" }, [
              For({
                each: detail.fields,
                render(field_) {
                  const field = HomeDetailValue(field_);
                  return View({ class: "wx-home-detail-field" }, [
                    View({ class: "wx-home-detail-field-label" }, [
                      field.label,
                    ]),
                    View(
                      {
                        class: "wx-home-detail-field-value",
                        attributes: { title: field.value },
                      },
                      [field.value],
                    ),
                  ]);
                },
              }),
            ]);
          },
        }),
        Show({
          when: Boolean(detail.preview),
          ok() {
            return View({ class: "wx-home-article-preview" }, [detail.preview]);
          },
        }),
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
                    View({ class: "wx-home-detail-image-meta" }, [image.meta]),
                  ]);
                },
              }),
            ]);
          },
        }),
        Show({
          when: Boolean(detail.link_url),
          ok() {
            return Button(
              {
                class: "wx-home-link-button wx-home-detail-link",
                attributes: { type: "button" },
                onClick() {
                  vm$.methods.openDetailURL(detail.link_url);
                },
              },
              ["打开详情地址", Timeless.Icon({ name: "external-link", size: 14 })],
            );
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

function HomeResultActions(props) {
  const vm$ = props.store;
  return View({ class: "wx-home-result-actions" }, [
    Button(
      {
        class: "wx-content-action wx-home-cache-action",
        disabled: vm$.state.cache_action_disabled,
        attributes: {
          type: "button",
          title: "清理该 URL 的抓取缓存",
        },
        onClick() {
          vm$.methods.clearFetchCache();
        },
      },
      ["清理缓存"],
    ),
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
        HomeContentDetails({ store: vm$ }),
        HomeRawJSON({ store: vm$ }),
        HomeResultActions({ store: vm$ }),
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
                          View({ class: "wx-home-platform-status-name" }, [
                            item.platform_name,
                          ]),
                          View({ class: "wx-home-platform-status-id" }, [
                            item.platform,
                          ]),
                        ]),
                        View({ class: "wx-home-platform-status-value" }, [
                          item.status_text,
                        ]),
                      ]);
                    },
                  }),
                ]);
              },
              else() {
                return View({ class: "wx-home-platform-status-empty" }, [
                  "等待平台状态推送",
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

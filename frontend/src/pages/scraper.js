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
  "video": "[&_.group-home-detail-card-icon]:bg-color-accent-violet-soft [&_.group-home-detail-card-icon]:text-color-accent-violet",
  "live": "[&_.group-home-detail-card-icon]:bg-color-accent-violet-soft [&_.group-home-detail-card-icon]:text-color-accent-violet",
  "article": "[&_.group-home-detail-card-icon]:bg-color-accent-sky-soft [&_.group-home-detail-card-icon]:text-color-accent-sky",
  "album": "[&_.group-home-detail-card-icon]:bg-color-accent-amber-soft [&_.group-home-detail-card-icon]:text-color-accent-amber",
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
      class: "group-content-page min-h-full flex flex-col bg-content-bg text-content-text min-h-full bg-content-bg min-w-0 min-h-full font-sans",
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
      View({ class: "group-content-main flex-[1_1_auto] min-w-0 p-4 home-main " }, [
        View({ class: "flex flex-col gap-6" }, [
          View(
            {
              class: "relative min-w-0 p-6 flex flex-col gap-5 border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-md [&::before]:content-[\"\"] [&::before]:absolute [&::before]:top-[calc(var(--dm-space-2)_*_-1)] [&::before]:left-7 [&::before]:w-16 [&::before]:h-2 [&::before]:bg-color-primary-fill [&::before]:rounded-[var(--dm-radius-content)_var(--dm-radius-content)_0_0] max-520:p-2-5 dm-panel",
              attributes: { "aria-busy": vm$.state.busy },
            },
            [
              View({ class: "flex items-start justify-between gap-3 max-520:flex-wrap" }, [
                View({ class: "w-control-height-lg h-control-height-lg grid flex-[0_0_auto] place-items-center text-color-on-primary bg-color-primary-fill rounded-content shadow-[0_var(--dm-space-2)_var(--dm-space-5)_var(--dm-color-signal-glow)]" }, [
                  Timeless.Icon({ name: "inbox", size: 20 }),
                ]),
                View({ class: "min-w-0 flex-[1_1_auto]" }, [
                  View({ class: "mb-1 text-color-primary text-caption font-semibold tracking-wide text-transform-uppercase" }, ["新建归档任务"]),
                  View({ class: "text-content-text font-display text-xl font-semibold leading-tight tracking-tight" }, [
                    "粘贴内容链接，开始解析",
                  ]),
                  View({ class: "mt-1-5 text-content-muted text-sm leading-base" }, [
                    "识别内容、作者与媒体资源，并生成可追踪的下载任务。",
                  ]),
                ]),
                View({ class: "ml-auto flex items-center max-520:w-full" }, [
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
      class: "grid grid-cols-[minmax(0,_1fr)_auto] gap-3 pt-1 max-520:grid-cols-[minmax(0,_1fr)]",
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
      View({ class: "[&_>_.dm-button]:min-w-120px flex items-center justify-end gap-2 max-520:w-full max-520:[&_>_.dm-button]:flex-[1_1_auto] max-520:[&_>_.dm-button]:w-full" }, [
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
            return View({ class: "[&_>_.dm-button]:min-w-120px flex items-center justify-end gap-2 max-520:w-full max-520:flex-wrap max-520:[&_>_.dm-button]:flex-[1_1_auto] max-520:[&_>_.dm-button]:w-full" }, [
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
      return View({ class: "relative min-h-260px overflow-hidden border-right-[1px_solid_var(--dm-content-border)] bg-content-panel-soft max-620:min-h-0 max-620:aspect-ratio-[16_/_9] max-620:border-right-0 max-620:border-bottom-[1px_solid_var(--dm-content-border)]" }, [
        // View({ class: "home-content-cover-fallback" }, [
        //   Timeless.Icon({ name: "file", size: 34 }),
        //   content.content_type_name,
        // ]),
        LazyImg({
          class: "absolute inset-0 w-full h-full block object-cover",
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
  return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow home-content-card dm-panel" }, [
    View({ class: "group-home-card-heading min-h-58px p-[var(--dm-space-2-5)_var(--dm-space-3-5)] flex items-center justify-between gap-3 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel max-520:pr-3 max-520:pl-3 max-520:flex-wrap" }, [
      View({ class: "min-w-0 flex items-center gap-2-5" }, [
        View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-34px h-34px flex-[0_0_auto] rounded-content" }, [
          Timeless.Icon({ name: "file", size: 17 }),
        ]),
        View({}, [
          View({ class: "text-content-muted text-caption font-750 tracking-0 leading-[1.2]" }, ["CONTENT"]),
          View({ class: "mt-0-5 text-ui font-680 leading-[1.2]" }, ["内容"]),
        ]),
      ]),
    ]),
    View(
      {
        class: computed(content.cover_url, (cover_url) =>
          cover_url
            ? "min-h-0 grid grid-cols-[minmax(220px,_31%)_minmax(0,_1fr)] [&.is-coverless]:grid-cols-[minmax(0,_1fr)] max-620:grid-cols-[minmax(0,_1fr)]"
            : "min-h-0 grid grid-cols-[minmax(220px,_31%)_minmax(0,_1fr)] [&.is-coverless]:grid-cols-[minmax(0,_1fr)] max-620:grid-cols-[minmax(0,_1fr)] is-coverless",
        ),
      },
      [
        ScraperContentCover({ content }),
        View({ class: "min-w-0 flex flex-col items-start p-4-5 max-520:p-4" }, [
          View(
            { class: "flex items-center flex-wrap gap-1-5", attributes: { n: "scraper-content-tags" } },
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
              class: "w-full mt-3-5 -webkit-box overflow-hidden text-content-text text-xl font-720 leading-[1.42] overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2",
              attributes: { title: content.title },
            },
            [content.title],
          ),
          View({ class: "mt-3 flex items-center gap-1-5 text-content-muted text-sm" }, [
            Timeless.Icon({ name: "clock3", size: 14 }),
            content.publish_time_text,
          ]),
          Show({
            when: content.show_description,
            ok() {
              return View({ class: "w-full mt-2 -webkit-box overflow-hidden text-content-muted text-sm leading-[1.6] overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2" }, [description]);
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
      class: "flex-[1_1_0] min-w-0 p-[var(--dm-space-2-5)_var(--dm-space-3)] block border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft text-inherit text-left",
      attributes: {
        title: node.has_url ? node.url : node.id || node.title,
      },
    },
    [
      View({ class: "text-content-muted text-caption font-700" }, [node.type_name]),
      View({ class: "mt-1 overflow-hidden text-content-text text-md font-650 text-ellipsis whitespace-nowrap" }, [node.title]),
      View({ class: "text-content-muted text-caption mt-1 overflow-hidden font-mono text-ellipsis whitespace-nowrap" }, [node.meta_text]),
    ],
  );
}

function HomeContentInfluencerEntity(props) {
  const node = props.node || {};
  return View(
    {
      class: [
        "min-w-0 flex items-baseline gap-1 text-content-text leading-[1.2] [&.is-content]:max-w-180px [&.is-content]:text-content-muted",
        props.content ? "is-content" : "is-influencer",
      ].join(" "),
    },
    [
      View({ class: "flex-[0_0_auto] text-content-muted font-mono text-caption" }, [node.type_name]),
      View({ class: "min-w-0 overflow-hidden text-xs font-620 text-ellipsis whitespace-nowrap" }, [node.title]),
    ],
  );
}

function ScraperContentInfluencers(props) {
  const vm$ = props.store;
  const influencers = vm$.state.content_relations.influencers;
  return Show({
    when: influencers.present,
    ok() {
      return View({ class: "p-[var(--dm-space-1-5)_var(--dm-space-2-5)_var(--dm-space-2)]" }, [
        View({ class: "p-0 flex items-center justify-between gap-1-5" }, [
          View({ class: "inline-flex items-center gap-1 text-content-text text-xs font-650 [&_svg]:text-content-primary" }, [
            Timeless.Icon({ name: "user", size: 12 }),
            "相关人物",
          ]),
          View({ class: "text-content-muted text-caption font-600" }, [
            influencers.count_text,
          ]),
        ]),
        View(
          {
            class: " p-[var(--dm-space-1)_0_0] [&_.group-home-content-relation_+_.group-home-content-relation]:mt-1 [&_.group-home-content-relation_+_.group-home-content-relation]:pt-0 [&_.group-home-content-relation_+_.group-home-content-relation]:border-top-0",
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
                      "group-home-content-relation [&_+_.group-home-content-relation]:mt-2-5 [&_+_.group-home-content-relation]:pt-2-5 [&_+_.group-home-content-relation]:border-top-[1px_solid_var(--dm-content-border)] min-w-0 block [&_.group-home-content-relation-flow]:w-auto [&_.group-home-content-relation-flow]:max-w-full [&_.group-home-content-relation-flow]:inline-flex [&_.group-home-content-relation-flow]:flex-row [&_.group-home-content-relation-flow]:items-baseline [&_.group-home-content-relation-flow]:gap-1-5 [&_.group-home-content-relation-edge]:min-w-0 [&_.group-home-content-relation-edge]:inline-flex [&_.group-home-content-relation-edge]:flex-none [&_.group-home-content-relation-edge]:flex-row [&_.group-home-content-relation-edge]:items-center [&_.group-home-content-relation-edge]:gap-1 [&_.group-home-content-relation-edge]:text-content-muted [&_.group-home-content-relation-edge]:opacity-[0.72] [&_.group-home-content-relation-edge]:whitespace-nowrap [&_.group-home-content-relation-edge_svg]:transform-none [&_.group-home-content-relation-edge-name]:text-content-muted [&_.group-home-content-relation-edge-name]:text-caption [&_.group-home-content-relation-edge-name]:font-600 [&_.group-home-content-relation-edge-type]:text-caption",
                  },
                  [
                    View({ class: "group-home-content-relation-flow min-w-0 flex items-center gap-2-5 max-620:flex-col max-620:items-stretch" }, [
                      HomeContentInfluencerEntity({
                        node: relation.source,
                      }),
                      View({ class: "group-home-content-relation-edge flex-[0_0_96px] min-w-0 flex flex-col items-center gap-1 text-content-primary text-center max-620:[&_svg]:transform-[rotate(90deg)] max-620:basis-auto" }, [
                        Timeless.Icon({ name: "arrow-right", size: 11 }),
                        View({ class: "group-home-content-relation-edge-name text-content-text text-xs font-680" }, [
                          relation.role_text,
                        ]),
                        View({ class: "group-home-content-relation-edge-type text-content-muted text-caption font-mono" }, [
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
      return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow home-content-relations dm-panel" }, [
        View({ class: "group-home-card-heading min-h-58px p-[var(--dm-space-2-5)_var(--dm-space-3-5)] flex items-center justify-between gap-3 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel max-520:pr-3 max-520:pl-3 max-520:flex-wrap" }, [
          View({ class: "min-w-0 flex items-center gap-2-5" }, [
            View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-34px h-34px flex-[0_0_auto] rounded-content" }, [
              Timeless.Icon({ name: "git-branch", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-content-muted text-caption font-750 tracking-0 leading-[1.2]" }, ["RELATIONS"]),
              View({ class: "mt-0-5 text-ui font-680 leading-[1.2]" }, ["内容关联"]),
            ]),
          ]),
          Tag({ name: "home-detail-badge", class: "flex-[0_0_auto] border-[1px_solid_color-mix(in_srgb,_var(--dm-content-primary)_28%,_var(--dm-content-border))] bg-content-primary-soft text-content-primary font-variant-numeric-tabular-nums max-520:max-w-[44%] max-520:overflow-hidden max-520:text-ellipsis max-520:whitespace-nowrap" }, [
            relations.count_text,
          ]),
        ]),
        Show({
          when: relations.content_present,
          ok() {
            return View(
              {
                class: "p-[var(--dm-space-3)_var(--dm-space-3-5)]",
              },
              [
                For({
                  key: "key",
                  each: relations.items,
                  render(chain_) {
                    const chain = ScraperDetailValue(chain_);
                    return View({ class: "group-home-content-relation [&_+_.group-home-content-relation]:mt-2-5 [&_+_.group-home-content-relation]:pt-2-5 [&_+_.group-home-content-relation]:border-top-[1px_solid_var(--dm-content-border)] min-w-0 block" }, [
                      View({ class: "mb-2 text-content-muted font-mono text-xs font-700 tracking-[0.02em]" }, [
                        chain.type_path_text,
                      ]),
                      View({ class: "group-home-content-relation-flow min-w-0 flex items-center gap-2-5 max-620:flex-col max-620:items-stretch" }, [
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
                              { class: "group-home-content-relation-edge flex-[0_0_96px] min-w-0 flex flex-col items-center gap-1 text-content-primary text-center max-620:[&_svg]:transform-[rotate(90deg)] max-620:basis-auto" },
                              [
                                Timeless.Icon({
                                  name: "arrow-right",
                                  size: 15,
                                }),
                                View(
                                  {
                                    class: "group-home-content-relation-edge-name text-content-text text-xs font-680",
                                  },
                                  [edge.type_name],
                                ),
                                View(
                                  {
                                    class: "group-home-content-relation-edge-type text-content-muted text-caption font-mono",
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
      return View({ class: "w-full mt-3 flex items-center gap-2-5" }, [
        HomeAccountAvatar({ account }),
        View({ class: "min-w-0 flex-[1_1_auto]" }, [
          View({ class: "min-w-0 overflow-hidden text-content-text text-md font-720 leading-[1.35] text-ellipsis whitespace-nowrap" }, [account.nickname]),
        ]),
      ]);
    },
  });
}

function HomeAccountAvatar(props) {
  const account = props.account;
  return View({ class: "relative w-38px h-38px flex-[0_0_auto] overflow-hidden border-[2px_solid_var(--dm-content-panel)] rounded-avatar bg-content-primary-soft shadow-[0_0_0_1px_var(--dm-content-border)]" }, [
    View({ class: "absolute inset-0 w-full h-full flex items-center justify-center text-content-primary text-ui font-720" }, [account.avatar_fallback]),
    Show({
      when: account.avatar_url,
      ok() {
        return LazyImg({
          class: "absolute inset-0 w-full h-full block object-cover",
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
            "group-home-detail-video-frame relative w-full mb-2-5 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-content bg-color-media-canvas aspect-ratio-[16_/_9] [&.is-playing_.group-home-detail-video-play-position]:opacity-0 [&.is-playing_.group-home-detail-video-play-position]:pointer-events-none [&.is-playing_.group-home-detail-video-play-position]:transform-[translate(-50%,_-50%)_scale(0.96)]",
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
                  class: "absolute inset-0 w-full h-full block bg-color-media-canvas object-contain",
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
                class: "absolute inset-0 w-full h-full block object-cover",
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
                  class: "group-home-detail-video-play-position absolute top-[50%] left-[50%] transform-[translate(-50%,_-50%)]",
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
    View({ class: "min-w-54px flex-[0_0_auto] text-content-muted text-caption font-variant-numeric-tabular-nums" }, [chapter.index_text]),
    View({ class: "min-w-0 flex-[1_1_auto]" }, [
      View(
        {
          class: "overflow-hidden text-content-text text-sm font-620 text-ellipsis whitespace-nowrap",
          attributes: { title: chapter.title },
        },
        [chapter.title],
      ),
      Show({
        when: Boolean(chapter.meta_text),
        ok() {
          return View({ class: "mt-0-5 text-content-muted text-caption" }, [
            chapter.meta_text,
          ]);
        },
      }),
    ]),
    Show({
      when: Boolean(chapter.url),
      ok() {
        return View({ class: "flex-[0_0_auto] flex text-content-primary" }, [
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
      return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel home-novel-detail dm-panel" }, [
        View({ class: "min-h-56px p-[var(--dm-space-2-5)_var(--dm-space-3)] flex items-center justify-between gap-2-5 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel-soft max-520:items-start" }, [
          View({ class: "min-w-0 flex items-center gap-2-5" }, [
            View({ class: "group-home-detail-card-icon w-34px h-34px flex-[0_0_auto] flex items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
              Timeless.Icon({ name: "file-stack", size: 18 }),
            ]),
            View({}, [
              View({ class: "max-w-full overflow-hidden text-content-text text-base font-680 leading-[1.25] text-ellipsis whitespace-nowrap" }, [novel.title]),
              View({ class: "max-w-full mt-1 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [novel.subtitle]),
            ]),
          ]),
          Tag({ name: "home-detail-badge", class: "flex-[0_0_auto] border-[1px_solid_color-mix(in_srgb,_var(--dm-content-primary)_28%,_var(--dm-content-border))] bg-content-primary-soft text-content-primary font-variant-numeric-tabular-nums max-520:max-w-[44%] max-520:overflow-hidden max-520:text-ellipsis max-520:whitespace-nowrap" }, [
            novel.progress_text,
          ]),
        ]),
        View({ class: "p-3-5 max-520:p-2-5" }, [
          View({ class: "grid grid-cols-[repeat(4,_minmax(0,_1fr))] gap-2 max-820:grid-cols-[repeat(2,_minmax(0,_1fr))]" }, [
            For({
              each: novel.metrics,
              render(metric_) {
                const metric = ScraperDetailValue(metric_);
                return View({ class: "min-w-0 p-[var(--dm-space-2-5)_var(--dm-space-2-5)] rounded-content bg-content-panel-soft" }, [
                  View({ class: "text-content-muted text-caption" }, [metric.label]),
                  View({ class: "mt-1 overflow-hidden text-content-text text-md font-680 leading-[1.35] text-ellipsis whitespace-nowrap" }, [metric.value]),
                ]);
              },
            }),
          ]),
          Show({
            when: novel.has_volumes,
            ok() {
              return View({ class: "mt-4" }, [
                View({ class: "mb-2 flex items-center justify-between gap-2-5" }, [
                  View({ class: "text-content-text text-md font-680" }, ["分卷"]),
                ]),
                View({ class: "grid grid-cols-[repeat(3,_minmax(0,_1fr))] gap-2 max-820:grid-cols-[minmax(0,_1fr)]" }, [
                  For({
                    key: "key",
                    each: novel.volumes,
                    render(volume_) {
                      const volume = ScraperDetailValue(volume_);
                      return View({ class: "min-w-0 p-[var(--dm-space-2-5)_var(--dm-space-2-5)] flex items-center gap-2 border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft" }, [
                        View({ class: "flex-[0_0_auto] text-content-primary text-caption font-680" }, [
                          volume.index_text,
                        ]),
                        View(
                          {
                            class: "min-w-0 overflow-hidden text-content-text text-sm text-ellipsis whitespace-nowrap",
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
            View({ class: "mb-2 flex items-center justify-between gap-2-5" }, [
              View({ class: "text-content-text text-md font-680" }, ["章节列表"]),
              View({ class: "text-content-muted text-xs font-variant-numeric-tabular-nums" }, [novel.progress_text]),
            ]),
            Show({
              when: novel.has_chapters,
              ok() {
                return View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-1-5 min-821:grid-cols-[repeat(4,_minmax(0,_1fr))] min-1280:grid-cols-[repeat(auto-fit,_minmax(240px,_1fr))] max-620:grid-cols-[minmax(0,_1fr)]" }, [
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
                return View({ class: "p-4-5 border-[1px_dashed_var(--dm-content-border)] rounded-content text-content-muted text-sm text-center" }, [
                  novel.empty_chapter_text,
                ]);
              },
            }),
            Show({
              when: novel.has_more_chapters,
              ok() {
                return View(
                  {
                    class: "w-full mt-2 [&_>_.dm-button]:w-full",
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
          ? "dm-button--choice-card motion-reduce:transition-none is-selected"
          : "dm-button--choice-card motion-reduce:transition-none",
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
      View({ class: "w-18px h-18px flex-[0_0_18px] flex items-center justify-center border-[1px_solid_var(--dm-content-border)] rounded-control bg-content-panel text-transparent transition-[color_var(--dm-duration-fast)_var(--dm-ease-standard),_border-color_var(--dm-duration-fast)_var(--dm-ease-standard),_background-color_var(--dm-duration-fast)_var(--dm-ease-standard),_box-shadow_var(--dm-duration-fast)_var(--dm-ease-standard)] [.dm-button--choice-card.is-selected_&]:border-color-color-selection-indicator [.dm-button--choice-card.is-selected_&]:bg-color-selection-indicator [.dm-button--choice-card.is-selected_&]:text-color-selection-indicator-text [.dm-button--choice-card.is-selected_&]:shadow-[0_0_0_3px_var(--dm-color-selection-halo)] motion-reduce:transition-none" }, [
        Show({
          when: selected_,
          ok() {
            return Timeless.Icon({ name: "check", size: 12 });
          },
        }),
      ]),
      View({ class: "min-w-0 flex-[1_1_auto]" }, [
        View(
          {
            class: "overflow-hidden text-content-text text-xs font-680 text-ellipsis whitespace-nowrap [.dm-button--choice-card.is-selected_&]:text-color-selection-text",
            attributes: { title: variant.variant_key },
          },
          [variant.title],
        ),
        View({ class: "mt-1 overflow-hidden text-content-muted text-caption text-ellipsis whitespace-nowrap" }, [
          variant.meta_text || variant.variant_key,
        ]),
        Show({
          when: Boolean(variant.url),
          ok() {
            return View(
              {
                class: "mt-1 overflow-hidden text-content-muted text-caption text-ellipsis whitespace-nowrap font-mono",
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
            ? "min-w-0 p-[var(--dm-space-2-5)_var(--dm-space-2-5)] flex items-start gap-2-5 border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft [&.is-selected]:border-color-color-selection-indicator [&.is-selected]:bg-color-selection-halo is-selected"
            : "min-w-0 p-[var(--dm-space-2-5)_var(--dm-space-2-5)] flex items-start gap-2-5 border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft [&.is-selected]:border-color-color-selection-indicator [&.is-selected]:bg-color-selection-halo",
        ),
      },
      [
        Checkbox({
          store: checkbox.store,
          class: "flex-[0_0_auto]",
          attributes: {
            n: "scraper-text-track-checkbox",
            "aria-label": `下载字幕 ${track.title || track.track_key}`,
          },
          onUnmounted: checkbox.dispose,
        }),
        View({ class: "min-w-0 flex-[1_1_auto]" }, [
          View(
            {
              class: "overflow-hidden text-content-text text-xs font-680 text-ellipsis whitespace-nowrap [.dm-button--choice-card.is-selected_&]:text-color-selection-text",
              attributes: { title: track.track_key },
            },
            [track.title],
          ),
          View({ class: "mt-1 overflow-hidden text-content-muted text-caption text-ellipsis whitespace-nowrap" }, [
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
        return View({ class: "min-w-0 flex flex-col gap-2 m-[var(--dm-space-1-5)_var(--dm-space-2)_0_var(--dm-space-5)]" }, [
          For({
            key: "key",
            each: track.sources,
            render(source_) {
              const source = ScraperDetailValue(source_);
              return View({ class: "min-w-0 p-[var(--dm-space-2)_var(--dm-space-2-5)] grid grid-cols-[54px_minmax(0,_1fr)] items-center gap-2 border-left-[2px_solid_var(--dm-content-primary)] bg-content-panel-soft max-520:grid-cols-[minmax(0,_1fr)]" }, [
                View({ class: "text-content-primary text-caption font-720" }, [
                  source.title,
                ]),
                View(
                  {
                    class: "overflow-hidden text-content-muted text-caption text-ellipsis whitespace-nowrap font-mono mt-0",
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
        return View({ class: "min-w-0 mt-3 pt-3 border-top-[1px_solid_var(--dm-content-border)]" }, [
          View({ class: "mb-2 flex items-center justify-between gap-2-5" }, [
            View({ class: "text-content-text text-xs font-700" }, [
              "ContentVideoVariant",
            ]),
            Tag(
              {
                name: "home-video-supplement-count",
                class: "min-w-22px bg-content-panel-soft text-content-muted font-variant-numeric-tabular-nums",
              },
              [String(detail.variants.length)],
            ),
          ]),
          View(
            {
              class: "min-w-0 flex-col gap-2 grid grid-cols-[repeat(3,_minmax(0,_1fr))] items-stretch min-1280:grid-cols-[repeat(4,_minmax(0,_1fr))] min-1580:grid-cols-[repeat(5,_minmax(0,_1fr))] max-820:grid-cols-[repeat(2,_minmax(0,_1fr))] max-620:grid-cols-[minmax(0,_1fr)]",
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
        return View({ class: "min-w-0 mt-3 pt-3 border-top-[1px_solid_var(--dm-content-border)]" }, [
          View({ class: "mb-2 flex items-center justify-between gap-2-5" }, [
            View({ class: "text-content-text text-xs font-700" }, [
              "ContentTextTrack",
            ]),
            Tag(
              {
                name: "home-video-supplement-count",
                class: "min-w-22px bg-content-panel-soft text-content-muted font-variant-numeric-tabular-nums",
              },
              [String(detail.text_tracks.length)],
            ),
          ]),
          View({ class: "min-w-0 flex flex-col gap-2" }, [
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
          class: "min-w-0 max-h-[min(70vh,_720px)] mt-2-5 p-3-5 overflow-auto !border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft text-content-text text-base leading-[1.75] overflow-wrap-anywhere [&.is-text]:whitespace-pre-wrap [&.is-markdown]:whitespace-pre-wrap [&.is-html_>_:first-child]:mt-0 [&.is-html_>_:last-child]:mb-0 [&.is-html_p]:m-[0_0_1em] [&.is-html_blockquote]:m-[0_0_1em] [&.is-html_figure]:m-[0_0_1em] [&.is-html_pre]:m-[0_0_1em] [&.is-html_table]:m-[0_0_1em] [&.is-html_ul]:m-[0_0_1em] [&.is-html_ol]:m-[0_0_1em] [&.is-html_h1]:m-[1.35em_0_0.65em] [&.is-html_h1]:text-content-text [&.is-html_h1]:leading-[1.35] [&.is-html_h2]:m-[1.35em_0_0.65em] [&.is-html_h2]:text-content-text [&.is-html_h2]:leading-[1.35] [&.is-html_h3]:m-[1.35em_0_0.65em] [&.is-html_h3]:text-content-text [&.is-html_h3]:leading-[1.35] [&.is-html_h4]:m-[1.35em_0_0.65em] [&.is-html_h4]:text-content-text [&.is-html_h4]:leading-[1.35] [&.is-html_h5]:m-[1.35em_0_0.65em] [&.is-html_h5]:text-content-text [&.is-html_h5]:leading-[1.35] [&.is-html_h6]:m-[1.35em_0_0.65em] [&.is-html_h6]:text-content-text [&.is-html_h6]:leading-[1.35] [&.is-html_img]:max-w-full [&.is-html_video]:max-w-full [&.is-html_audio]:max-w-full [&.is-html_iframe]:max-w-full [&.is-html_img]:h-auto [&.is-html_video]:h-auto [&.is-html_a]:text-content-primary [&.is-html_a]:text-decoration-underline [&.is-html_a]:text-underline-offset-2px [&.is-html_blockquote]:p-[var(--dm-space-2)_var(--dm-space-3)] [&.is-html_blockquote]:border-left-[3px_solid_var(--dm-content-primary)] [&.is-html_blockquote]:bg-content-panel [&.is-html_blockquote]:text-content-muted [&.is-html_pre]:p-[var(--dm-space-2-5)_var(--dm-space-3)] [&.is-html_pre]:overflow-x-auto [&.is-html_pre]:rounded-content [&.is-html_pre]:bg-content-panel [&.is-html_pre]:whitespace-pre [&.is-html_code]:font-mono [&.is-html_table]:w-full [&.is-html_table]:overflow-x-auto [&.is-html_table]:border-collapse-collapse [&.is-html_th]:p-[var(--dm-space-1-5)_var(--dm-space-2)] [&.is-html_th]:!border-[1px_solid_var(--dm-content-border)] [&.is-html_th]:text-left [&.is-html_td]:p-[var(--dm-space-1-5)_var(--dm-space-2)] [&.is-html_td]:!border-[1px_solid_var(--dm-content-border)] [&.is-html_td]:text-left is-html",
          content: article_body.content,
          attributes: {
            "data-content-format": "html",
          },
        });
      }
      return View(
        {
          class: `min-w-0 max-h-[min(70vh,_720px)] mt-2-5 p-3-5 overflow-auto !border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft text-content-text text-base leading-[1.75] overflow-wrap-anywhere [&.is-text]:whitespace-pre-wrap [&.is-markdown]:whitespace-pre-wrap [&.is-html_>_:first-child]:mt-0 [&.is-html_>_:last-child]:mb-0 [&.is-html_p]:m-[0_0_1em] [&.is-html_blockquote]:m-[0_0_1em] [&.is-html_figure]:m-[0_0_1em] [&.is-html_pre]:m-[0_0_1em] [&.is-html_table]:m-[0_0_1em] [&.is-html_ul]:m-[0_0_1em] [&.is-html_ol]:m-[0_0_1em] [&.is-html_h1]:m-[1.35em_0_0.65em] [&.is-html_h1]:text-content-text [&.is-html_h1]:leading-[1.35] [&.is-html_h2]:m-[1.35em_0_0.65em] [&.is-html_h2]:text-content-text [&.is-html_h2]:leading-[1.35] [&.is-html_h3]:m-[1.35em_0_0.65em] [&.is-html_h3]:text-content-text [&.is-html_h3]:leading-[1.35] [&.is-html_h4]:m-[1.35em_0_0.65em] [&.is-html_h4]:text-content-text [&.is-html_h4]:leading-[1.35] [&.is-html_h5]:m-[1.35em_0_0.65em] [&.is-html_h5]:text-content-text [&.is-html_h5]:leading-[1.35] [&.is-html_h6]:m-[1.35em_0_0.65em] [&.is-html_h6]:text-content-text [&.is-html_h6]:leading-[1.35] [&.is-html_img]:max-w-full [&.is-html_video]:max-w-full [&.is-html_audio]:max-w-full [&.is-html_iframe]:max-w-full [&.is-html_img]:h-auto [&.is-html_video]:h-auto [&.is-html_a]:text-content-primary [&.is-html_a]:text-decoration-underline [&.is-html_a]:text-underline-offset-2px [&.is-html_blockquote]:p-[var(--dm-space-2)_var(--dm-space-3)] [&.is-html_blockquote]:border-left-[3px_solid_var(--dm-content-primary)] [&.is-html_blockquote]:bg-content-panel [&.is-html_blockquote]:text-content-muted [&.is-html_pre]:p-[var(--dm-space-2-5)_var(--dm-space-3)] [&.is-html_pre]:overflow-x-auto [&.is-html_pre]:rounded-content [&.is-html_pre]:bg-content-panel [&.is-html_pre]:whitespace-pre [&.is-html_code]:font-mono [&.is-html_table]:w-full [&.is-html_table]:overflow-x-auto [&.is-html_table]:border-collapse-collapse [&.is-html_th]:p-[var(--dm-space-1-5)_var(--dm-space-2)] [&.is-html_th]:!border-[1px_solid_var(--dm-content-border)] [&.is-html_th]:text-left [&.is-html_td]:p-[var(--dm-space-1-5)_var(--dm-space-2)] [&.is-html_td]:!border-[1px_solid_var(--dm-content-border)] [&.is-html_td]:text-left is-${article_body.format || "text"}`,
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
      class: `min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel home-typed-detail ${home_typed_detail_classes[detail.kind] || ""} dm-panel`,
    },
    [
      View({ class: "min-h-56px p-[var(--dm-space-2-5)_var(--dm-space-3)] flex items-center justify-between gap-2-5 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel-soft max-520:items-start" }, [
        View({ class: "min-w-0 flex items-center gap-2-5" }, [
          View({ class: "group-home-detail-card-icon w-34px h-34px flex-[0_0_auto] flex items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
            Timeless.Icon({ name: detail.icon, size: 18 }),
          ]),
          View({}, [
            View({ class: "max-w-full overflow-hidden text-content-text text-base font-680 leading-[1.25] text-ellipsis whitespace-nowrap" }, [detail.title]),
            View({ class: "max-w-full mt-1 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [detail.model_name]),
          ]),
        ]),
        Tag({ name: "home-detail-badge", class: "flex-[0_0_auto] border-[1px_solid_color-mix(in_srgb,_var(--dm-content-primary)_28%,_var(--dm-content-border))] bg-content-primary-soft text-content-primary font-variant-numeric-tabular-nums max-520:max-w-[44%] max-520:overflow-hidden max-520:text-ellipsis max-520:whitespace-nowrap" }, [
          detail.type_name,
        ]),
      ]),
      View({ class: "p-3-5 max-520:p-2-5" }, [
        Show({
          when: detail.subject.present,
          ok() {
            return View({ class: "mb-3 p-[var(--dm-space-2-5)_var(--dm-space-2-5)] flex items-center justify-between gap-2-5 border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft" }, [
              View({ class: "min-w-0" }, [
                View({ class: "overflow-hidden text-ellipsis whitespace-nowrap text-content-text text-sm font-650" }, [
                  detail.subject.title,
                ]),
                View({ class: "overflow-hidden text-ellipsis whitespace-nowrap mt-1 text-content-muted font-mono text-caption" }, [
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
            return View({ class: "mt-2-5 grid grid-cols-[repeat(4,_minmax(0,_1fr))] gap-2" }, [
              For({
                key: "key",
                each: detail.images,
                render(image_) {
                  const image = ScraperDetailValue(image_);
                  return View({ class: "relative min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft" }, [
                    LazyImg({
                      class: "w-full aspect-ratio-1 block object-cover",
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
                            class: "absolute top-6px right-6px border-[1px_solid_var(--dm-color-media-border)] bg-color-media-surface text-color-media-text",
                            attributes: { title: "实况图" },
                          },
                          [
                            Timeless.Icon({ name: "play", size: 10 }),
                            image.live_photo_label,
                          ],
                        );
                      },
                    }),
                    View({ class: "p-1-5 overflow-hidden text-content-muted text-caption text-center text-ellipsis whitespace-nowrap" }, [image.meta]),
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
  return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow" }, [
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
        View({ class: "min-w-0 flex items-center gap-3" }, [
          View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-36px h-36px flex-[0_0_auto] rounded-content" }, [
            Timeless.Icon({ name: "braces", size: 17 }),
          ]),
          View({}, [
            View({ class: "text-content-text text-md font-650" }, [vm$.state.json_toggle_text]),
            View({ class: "mt-1 text-content-muted text-xs" }, [
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
            class: "max-h-[min(520px,_calc(100vh_-_180px))] m-0 p-4 overflow-auto border-top-[1px_solid_var(--dm-content-border)] bg-content-panel-soft text-content-text font-mono text-sm leading-[1.6] whitespace-pre-wrap word-break-break-word",
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
      return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow home-raw-fetch" }, [
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
            View({ class: "min-w-0 flex items-center gap-3" }, [
              View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-36px h-36px flex-[0_0_auto] rounded-content" }, [
                Timeless.Icon({ name: "braces", size: 17 }),
              ]),
              View({}, [
                View({ class: "text-content-text text-md font-650" }, ["抓取原始内容"]),
                View({ class: "mt-1 text-content-muted text-xs" }, [raw_result.meta_text]),
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
                class: "max-h-[min(520px,_calc(100vh_-_180px))] m-0 p-4 overflow-auto border-top-[1px_solid_var(--dm-content-border)] bg-content-panel-soft text-content-text font-mono text-sm leading-[1.6] whitespace-pre-wrap word-break-break-word",
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
        class: "grid grid-cols-[18px_20px_minmax(0,_1fr)_auto] items-center gap-1 p-[4px_var(--dm-space-1-5)] text-color-text-primary text-sm rounded-content [&.is-directory]:text-color-text-secondary [&.is-directory]:font-semibold [&.is-directory]:cursor-pointer [&.is-directory:hover]:bg-color-bg-hover is-directory",
        style: { "padding-left": indent_px },
        attributes: { n: "scraper-tree-dir", title: node._path || node.name },
        onClick() {
          onToggle(node._path);
        },
      },
      [
        View({ class: "w-18px shrink-0 text-center text-13px text-color-text-muted leading-1" }, [is_collapsed ? "›" : "⌄"]),
        View({ class: "w-18px shrink-0" }, [
          Timeless.Icon({ name: "folder", size: 16 }),
        ]),
        View({ class: "min-w-0 dm-truncate" }, [node.name || "根目录"]),
        View({ class: "text-color-text-muted text-11px font-normal whitespace-nowrap pl-2" }, [`${count_tree_children(node)} 项`]),
      ],
    );
  }

  const resource = node && node.resource;
  return View(
    {
      class: "grid items-center gap-1 p-[4px_var(--dm-space-1-5)] text-color-text-primary text-sm rounded-content [&.is-directory]:text-color-text-secondary [&.is-directory]:font-semibold [&.is-directory]:cursor-pointer [&.is-directory:hover]:bg-color-bg-hover grid-cols-[18px_20px_minmax(0,_1fr)_auto_auto] [&_button]:flex-[0_0_auto] [&_button]:ml-1",
      style: { "padding-left": indent_px },
      attributes: { n: "scraper-tree-file", title: node._path || node.name },
    },
    [
      View({ class: "w-18px shrink-0 text-center text-13px text-color-text-muted leading-1" }),
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
      View({ class: "text-color-text-muted text-11px font-normal whitespace-nowrap pl-2" }, [
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
  return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel" }, [
    View({ class: "min-h-42px p-[var(--dm-space-2)_var(--dm-space-3)] flex items-center justify-between gap-2-5 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel-soft" }, [
      View({ class: "text-content-text text-sm font-700" }, [props.title]),
      Tag(
        {
          name: "home-download-section-count",
          class: "min-w-22px bg-content-panel-soft text-content-muted font-variant-numeric-tabular-nums",
        },
        [props.count],
      ),
    ]),
    View({ class: "p-2-5" }, children),
  ]);
}

function ScraperDownloadInfo(props) {
  const vm$ = props.store;
  const download_info = vm$.state.download_info;
  const task = download_info.task;
  return Show({
    when: download_info.present,
    ok() {
      return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow home-download-info dm-panel" }, [
        View({ class: "group-home-card-heading min-h-58px p-[var(--dm-space-2-5)_var(--dm-space-3-5)] flex items-center justify-between gap-3 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel max-520:pr-3 max-520:pl-3 max-520:flex-wrap" }, [
          View({ class: "min-w-0 flex items-center gap-2-5" }, [
            View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-34px h-34px flex-[0_0_auto] rounded-content" }, [
              Timeless.Icon({ name: "download", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-content-muted text-caption font-750 tracking-0 leading-[1.2]" }, ["DOWNLOAD INFO"]),
              View({ class: "mt-0-5 text-ui font-680 leading-[1.2]" }, ["下载任务预览"]),
            ]),
          ]),
          View({ class: "ml-auto flex items-center justify-end flex-wrap gap-2 max-520:w-full" }, [
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
        View({ class: "p-3-5 flex flex-col gap-3 max-520:p-2-5" }, [

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
                          class: "dm-notice-row mb-2-5 text-color-text-secondary text-xs leading-base",
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
            View({ class: "min-w-0 flex flex-col gap-2" }, [
              View({ class: "min-w-0 p-2-5 grid grid-cols-[34px_minmax(0,_1fr)_auto] items-center gap-2-5 border-[1px_solid_var(--dm-content-border)] rounded-content bg-content-panel-soft max-520:grid-cols-[28px_minmax(0,_1fr)] max-520:[&_>_.group-home-download-status]:grid-column-2 max-520:[&_>_.group-home-download-status]:justify-self-start" }, [
                View({ class: "text-content-muted font-mono text-caption font-variant-numeric-tabular-nums text-center" }, [task.id_text]),
                View({ class: "min-w-0" }, [
                  View(
                    {
                      class: "overflow-hidden text-content-text text-sm font-680 text-ellipsis whitespace-nowrap",
                      attributes: { title: task.name },
                    },
                    [task.name],
                  ),
                  View({ class: "mt-1 overflow-hidden text-content-muted text-caption text-ellipsis whitespace-nowrap" }, [
                    task.meta_text || "任务将在确认后创建",
                  ]),
                ]),
                Show({
                  when: computed(task.status_text, (text) => Boolean(text)),
                  ok() {
                    return Tag(
                      {
                        name: "home-download-status",
                        class: "group-home-download-status bg-color-success-soft text-color-success",
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
      class: "sticky bottom-0 z-sticky w-full min-w-0 min-h-72px box-sizing-border-box p-[var(--dm-space-3)_var(--dm-space-4)] flex items-center justify-between gap-4 border-0 border-top-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-0 bg-[color-mix(_in_srgb,_var(--dm-content-panel)_72%,_transparent_)] shadow-none -webkit-backdrop-filter-[blur(18px)_saturate(140%)] backdrop-filter-[blur(18px)_saturate(140%)] max-520:flex-wrap max-520:p-2-5 max-520:items-stretch",
      attributes: {
        n: "result-actions",
        role: "toolbar",
        "aria-label": "抓取完成操作",
      },
    },
    [
      View(
        {
          class: "flex items-center min-w-0 flex-[1_1_auto] gap-2-5",
          attributes: { n: "result-actions-summary" },
        },
        [
          View(
            {
              class: "w-36px h-36px flex-[0_0_auto] grid place-items-center rounded-content bg-color-success-soft text-color-success",
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
                  class: "text-content-text text-sm font-700",
                  attributes: { n: "result-actions-title" },
                },
                ["抓取完成"],
              ),
              View(
                {
                  class: "mt-0-5 text-content-muted text-caption",
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
          class: "flex items-center flex-[0_0_auto] justify-end gap-2 max-520:w-full max-520:[&_>_.dm-button]:flex-[1_1_0] max-520:[&_>_.dm-button]:justify-center",
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
      return View({ class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow max-520:[&_.group-home-card-heading]:items-stretch max-520:[&_.group-home-card-heading]:flex-col dm-panel" }, [
        View({ class: "group-home-card-heading min-h-58px p-[var(--dm-space-2-5)_var(--dm-space-3-5)] flex items-center justify-between gap-3 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel max-520:pr-3 max-520:pl-3 max-520:flex-wrap" }, [
          View({ class: "min-w-0 flex items-center gap-2-5" }, [
            View({ class: "flex items-center justify-center bg-content-primary-soft text-content-primary w-34px h-34px flex-[0_0_auto] rounded-content" }, [
              Timeless.Icon({ name: "file-stack", size: 17 }),
            ]),
            View({}, [
              View({ class: "text-content-muted text-caption font-750 tracking-0 leading-[1.2]" }, ["CACHE"]),
              View({ class: "mt-0-5 text-ui font-680 leading-[1.2]" }, ["抓取缓存"]),
            ]),
          ]),
          View({ class: "[&_>_.dm-button]:min-w-120px flex items-center justify-end gap-2-5 max-520:items-stretch max-520:flex-col max-520:w-full max-520:[&_>_.dm-button]:flex-[1_1_auto]" }, [
            View({ class: "text-content-muted text-sm whitespace-nowrap max-520:whitespace-normal" }, [cache.summary_text]),
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
        View({ class: "max-h-360px p-[var(--dm-space-1-5)_var(--dm-space-3-5)] overflow-auto" }, [
          For({
            key: "key",
            each: cache.entries,
            render(entry_) {
              const entry = ScraperDetailValue(entry_);
              return Button(
                {
                  store: vm$.ui.btn_cache_entry$.bind(entry),
                  class: "dm-button--list-row max-520:grid-cols-[30px_minmax(0,_1fr)]",
                  attributes: {
                    type: "button",
                    title: `查看缓存内容：${entry.name}`,
                    "aria-label": `查看缓存内容：${entry.name}`,
                  },
                },
                [
                  View({ class: "w-30px h-30px flex items-center justify-center rounded-content bg-content-primary-soft text-content-primary" }, [
                    Timeless.Icon({ name: "file", size: 16 }),
                  ]),
                  View({ class: "min-w-0" }, [
                    View(
                      {
                        class: "overflow-hidden text-content-text text-sm font-620 text-ellipsis whitespace-nowrap",
                        attributes: { title: entry.name },
                      },
                      [entry.name],
                    ),
                    View(
                      {
                        class: "mt-1 overflow-hidden text-content-muted font-mono text-caption text-ellipsis whitespace-nowrap",
                        attributes: { title: entry.path },
                      },
                      [entry.path],
                    ),
                  ]),
                  View({ class: "inline-flex items-center gap-1-5 text-content-muted max-520:grid-column-2 max-520:justify-self-start" }, [
                    View({ class: "text-content-muted text-xs font-variant-numeric-tabular-nums whitespace-nowrap max-520:grid-column-2" }, [entry.size_text]),
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
      View({ class: "p-[var(--dm-space-4)_var(--dm-space-4-5)_var(--dm-space-3)] flex items-start gap-3-5" }, [
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "overflow-hidden text-lg font-680 text-ellipsis whitespace-nowrap" }, [cache_content.title]),
          View({ class: "mt-1 text-content-muted text-xs" }, [cache_content.meta_text]),
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
          class: "p-[0_var(--dm-space-4-5)_var(--dm-space-3)] overflow-hidden text-content-muted font-mono text-caption text-ellipsis whitespace-nowrap",
          attributes: { title: cache_content.path },
        },
        [cache_content.path],
      ),
      View({ class: "min-h-260px flex overflow-hidden border-top-[1px_solid_var(--dm-content-border)] bg-content-panel-soft" }, [
        Show({
          when: cache_content.loading,
          ok() {
            return View({ class: "min-h-260px w-full flex items-center justify-center gap-2-5 text-content-muted text-md" }, [
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
                    class: "w-full min-h-260px max-h-[min(68vh,_650px)] box-sizing-border-box m-0 p-[var(--dm-space-4)_var(--dm-space-4-5)] overflow-auto text-content-text font-mono text-sm leading-[1.6] tab-size-2 whitespace-pre-wrap word-break-break-word",
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
          class: "flex flex-col gap-3-5",
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
          class: "flex flex-col gap-3-5",
          attributes: { n: "platform-home-scraper-result" },
        },
        [
          View(
            {
              class: "min-w-0 overflow-hidden border-[1px_solid_var(--dm-content-border)] rounded-card bg-content-panel shadow-content-shadow home-platform-preview-card dm-panel",
              attributes: { n: "platform-home-preview-card" },
            },
            [
              View(
                {
                  class: "group-home-card-heading min-h-58px p-[var(--dm-space-2-5)_var(--dm-space-3-5)] flex items-center justify-between gap-3 border-bottom-[1px_solid_var(--dm-content-border)] bg-content-panel max-520:pr-3 max-520:pl-3 max-520:flex-wrap",
                  attributes: { n: "platform-home-preview-heading" },
                },
                [
                  View(
                    {
                      class: "mt-0-5 text-ui font-680 leading-[1.2]",
                      attributes: { n: "platform-home-preview-title" },
                    },
                    [home.title],
                  ),
                ],
              ),
              View(
                {
                  class: "h-[min(72vh,_820px)] bg-[#fff]",
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
                class: "box-sizing-border-box w-full min-w-0 grid grid-cols-[repeat(5,_minmax(0,_1fr))] gap-1",
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
                            class: "relative w-5 h-5 min-w-5 min-h-5 max-w-5 max-h-5 grid flex-[0_0_var(--dm-space-5)] place-items-center [&_>_.group-home-platform-status-dot]:absolute [&_>_.group-home-platform-status-dot]:right-[calc(var(--dm-space-0-5)_*_-1)] [&_>_.group-home-platform-status-dot]:bottom-[calc(var(--dm-space-0-5)_*_-1)] [&_>_.group-home-platform-status-dot]:outline-[var(--dm-space-0-5)_solid_var(--dm-color-bg-surface)]",
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
                              class: "group-home-platform-status-dot w-8px h-8px flex-[0_0_auto] rounded-round bg-content-muted [.dm-button--status.is-available_&]:bg-brand-success [.dm-button--status.is-available_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-brand-success)_14%,_transparent)] [.dm-button--status.is-unavailable_&]:bg-content-danger [.dm-button--status.is-unavailable_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-content-danger)_13%,_transparent)] [.dm-button--status.is-checking_&]:bg-content-primary [.dm-button--status.is-checking_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-content-primary)_13%,_transparent)]",
                              attributes: { n: "platform-status-dot" },
                            }),
                          ],
                        ),
                        View(
                          {
                            class: "w-full min-w-0 flex flex-col items-center gap-0",
                            attributes: { n: "platform-status-main" },
                          },
                          [
                            View(
                              {
                                class: "w-full min-w-0 flex flex-col items-center gap-0",
                                attributes: { n: "platform-status-head" },
                              },
                              [
                                View(
                                  {
                                    class: "w-full min-w-0 min-h-[2.4em] -webkit-box overflow-hidden text-caption font-semibold leading-tight text-center overflow-wrap-anywhere -webkit-box-orient-vertical -webkit-line-clamp-2",
                                    attributes: { n: "platform-status-name" },
                                  },
                                  [item.platform_name],
                                ),
                                Show({
                                  when: item.has_status_text,
                                  ok() {
                                    return View(
                                      {
                                        class: "group-home-platform-status-value flex-[0_0_auto] text-content-muted text-caption whitespace-nowrap",
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
            return View({ class: "p-[var(--dm-space-5)_var(--dm-space-3-5)] text-content-muted text-sm text-center" }, [
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
          View({ class: "group-home-platform-status-dot w-8px h-8px flex-[0_0_auto] rounded-round bg-content-muted [.dm-button--status.is-available_&]:bg-brand-success [.dm-button--status.is-available_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-brand-success)_14%,_transparent)] [.dm-button--status.is-unavailable_&]:bg-content-danger [.dm-button--status.is-unavailable_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-content-danger)_13%,_transparent)] [.dm-button--status.is-checking_&]:bg-content-primary [.dm-button--status.is-checking_&]:shadow-[0_0_0_3px_color-mix(in_srgb,_var(--dm-content-primary)_13%,_transparent)]" }),
          View({ class: "home-platform-status-trigger-label" }, ["平台状态"]),
          View({ class: "text-content-muted text-sm max-520:hidden" }, [
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
          class: "flex flex-col gap-2 p-[var(--dm-space-3)_var(--dm-space-3)] border-[1px_solid_transparent] rounded-content bg-content-panel-soft",
          attributes: {
            role: "progressbar",
            "aria-label": "抓取进度",
            "aria-valuemin": "0",
            "aria-valuemax": "100",
            "aria-valuenow": vm$.state.progress_percent,
          },
        },
        [
          View({ class: "flex items-center justify-between gap-3 min-w-0" }, [
            View({ class: "min-w-0 text-content-text text-md font-600 overflow-hidden text-ellipsis whitespace-nowrap" }, [
              vm$.state.progress_stage_text,
            ]),
            View({ class: "flex-[0_0_auto] flex items-center gap-2-5 text-content-muted text-sm" }, [
              Show({
                when: vm$.state.progress_updated_text,
                ok() {
                  return View({ class: "text-content-muted text-sm" }, [
                    vm$.state.progress_updated_text,
                  ]);
                },
              }),
              Show({
                when: vm$.state.progress_has_percent,
                ok() {
                  return View({ class: "text-content-primary font-variant-numeric-tabular-nums" }, [
                    vm$.state.progress_percent_text,
                  ]);
                },
              }),
            ]),
          ]),
          View({ class: "text-content-muted text-sm leading-[1.45] overflow-wrap-anywhere" }, [
            vm$.state.progress_message,
          ]),
          View({ class: "h-6px overflow-hidden rounded-round bg-[color-mix(in_srgb,_var(--dm-content-primary)_13%,_transparent)]" }, [
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
              return View({ class: "text-content-muted text-sm" }, [
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

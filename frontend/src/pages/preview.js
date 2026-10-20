import { ErrorMessage } from "@/dmui.js";
import { PreviewViewModel } from "./preview.model.js";

function PreviewStateView(props) {
  return View(
    {
      class: [
        "flex w-parent-inset max-w-130 min-h-360px flex-none flex-col items-center justify-center m-auto py-16 px-5 gap-2 text-center border-default border-solid rounded-card dm-empty-state border-color-default bg-color-bg-surface text-color-text-secondary shadow-lg state-panel-root state-panel",
        props.role === "alert" ? "is-error" : "",
      ]
        .filter(Boolean)
        .join(" "),
      role: props.role || "status",
    },
    [
      props.loading
        ? BrandLoading({
            size: 88,
            name: "preview-loading-symbol",
            decorative: true,
          })
        : null,
      !props.loading
        ? props.role === "alert"
          ? BrandError({ size: 124, name: "preview-error-symbol" })
          : BrandEmpty({ size: 124, name: "preview-empty-symbol" })
        : null,
      props.title
        ? View({ as: "h3", class: "m-0 text-lg font-semibold text-color-text-primary" }, [props.title])
        : null,
      props.message
        ? View({ as: "p", class: "m-0 text-md leading-relaxed overflow-wrap-anywhere text-color-text-secondary" }, [props.message])
        : null,
      props.action || null,
    ].filter(Boolean),
  );
}

function PreviewHeaderView(props) {
  const task = props.task;
  const account = task.account;
  return View(
    { class: "flex-none py-3 px-4 border-b border-b-solid border-b-color-default bg-color-bg-surface text-color-text-primary shadow-divider backdrop-blur-14px " },
    [
      account
        ? View({ class: "flex items-center mb-1.5 gap-2" }, [
            account.avatar_url
              ? LazyImg({
                  class: "w-24px h-24px object-cover rounded-avatar bg-color-bg-subtle",
                  src: account.avatar_url,
                  alt: "",
                  attributes: { referrerpolicy: "no-referrer" },
                })
              : null,
            View({ class: "overflow-hidden text-md font-500 whitespace-nowrap text-ellipsis text-color-text-primary" }, [
              account.nickname || account.external_id || "",
            ]),
          ])
        : null,
      View({ class: "-webkit-box mb-1 overflow-hidden text-ui font-600 leading-1.4 dm-font-display dm-font-bold -webkit-box-orient-vertical -webkit-line-clamp-2" }, [
        task.title,
      ]),
      View(
        { class: "flex items-center gap-2 text-sm text-color-text-secondary" },
        [
          task.platform_id
            ? PlatformTag({
                name: "preview-platform",
                favicon: task.platform_favicon,
                label: task.platform_name,
              })
            : null,
          task.content_type
            ? Tag({ variant: "info", name: "preview-content-type" }, [
                task.content_type,
              ])
            : null,
          Number.isFinite(props.fileCount)
            ? Tag({ variant: "info", name: "preview-file-count" }, [
                `文件 (${props.fileCount})`,
              ])
            : null,
        ].filter(Boolean),
      ),
    ].filter(Boolean),
  );
}

function PreviewSingleFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  const url = vm$.methods.fileURL(file);
  if (vm$.methods.isHTMLFile(file)) {
    return PreviewHTMLFileView({ store: vm$, file });
  }
  if (vm$.methods.isTextFile(file)) {
    return PreviewTextFileView({ store: vm$, file });
  }
  if (file.file_type === "video") {
    return View({ class: "flex min-h-0 flex-1 flex-col items-center justify-center bg-color-media-canvas" }, [
      PreviewVideoPlayerView({
        store: vm$,
        file,
        videoClass: "w-auto max-w-full h-auto max-h-captioned-media outline-none",
        autoplay: true,
      }),
      View({ class: "max-w-full py-1 px-3 overflow-hidden text-sm text-center whitespace-nowrap text-ellipsis text-color-text-disabled" }, [file.name]),
    ]);
  }
  return View({ class: "flex min-h-0 flex-1 flex-col items-center justify-center p-5 overflow-auto" }, [
    LazyImg({
      class: "max-w-full max-h-captioned-media object-contain rounded-content",
      src: url,
      alt: file.name,
    }),
    View({ class: "max-w-full py-1 px-3 overflow-hidden text-sm text-center whitespace-nowrap text-ellipsis text-color-text-disabled" }, [file.name]),
  ]);
}

function PreviewHTMLFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  return View(
    {
      class: ["flex relative min-w-0 min-h-0 flex-1 overflow-hidden bg-color-document-canvas text-document color-scheme-light document-stage ", props.class].filter(Boolean).join(" "),
      attributes: {
        n: props.nodeName || "preview-html-reader",
        role: "region",
        "aria-label": `HTML 预览：${file.name}`,
      },
      onMounted() {
        vm$.methods.mountHTMLReader(file);
      },
      onUnmounted() {
        vm$.methods.unmountHTMLReader();
      },
    },
    [
      Timeless.RichText({
        class: "block w-full min-w-0 min-h-0 p-5 overflow-auto leading-relaxed overflow-wrap-anywhere bg-color-document-canvas text-document rich-document ",
        content: vm$.state.html_content,
        attributes: {
          n: "preview-html-content",
          "data-content-format": "html",
          title: `HTML 预览：${file.name}`,
        },
      }),
      Show({
        when: vm$.state.html_loading,
        ok() {
          return PreviewStateView({
            loading: true,
            message: "正在加载 HTML…",
          });
        },
      }),
      Show({
        when: computed(vm$.state.html_error, (error) => Boolean(error)),
        ok() {
          return PreviewStateView({
            role: "alert",

            action: View(
              {
                as: "button",
                class:
                  "py-1.5 px-4 font-inherit text-md border border-solid border-color-media rounded-control dm-button--primary bg-none text-color-media-text cursor-pointer state-retry-action hover:bg-color-border dm-button dm-focus-ring",
                attributes: { type: "button" },
                onClick() {
                  vm$.methods.retryHTML();
                },
              },
              ["重试"],
            ),
          });
        },
      }),
    ],
  );
}

function PreviewTextFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  const render_text_line = (line_) => {
    const line = line_ && line_.value !== undefined ? line_.value : line_;
    return View(
      {
        class: "grid min-h-1.6em grid-cols-fit-content gap-4",
        attributes: {
          n: "preview-text-line",
          "data-line": String(line.number),
        },
      },
      [
        View(
          {
            class: "min-w-4ch text-right text-color-text-disabled user-select-none",
            attributes: {
              n: "preview-text-line-number",
              "aria-hidden": "true",
            },
          },
          [String(line.number)],
        ),
        View(
          {
            class: "min-w-0 whitespace-pre-wrap overflow-wrap-anywhere",
            attributes: { n: "preview-text-line-content" },
          },
          [line.text],
        ),
      ],
    );
  };
  return View(
    {
      class: ["flex min-w-0 min-h-0 flex-1 flex-col overflow-hidden bg-color-bg-surface", props.class].filter(Boolean).join(" "),
      attributes: {
        n: props.nodeName || "preview-text-reader",
        role: "region",
        "aria-label": `文本预览：${file.name}`,
      },
    },
    [
      View(
        {
          class: "flex flex-none items-center justify-between py-2.5 px-4 gap-3 border-b border-b-solid border-b-color-default",
          attributes: { n: "preview-text-toolbar" },
        },
        [
          View(
            {
              class: "min-w-0 overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis",
              attributes: { n: "preview-text-title", title: file.name },
            },
            [file.name],
          ),
          View(
            {
              class: "flex-none text-xs text-color-text-secondary",
              attributes: { n: "preview-text-count", "aria-live": "polite" },
            },
            [computed(vm$.state.text_lines, (lines) => `${lines.length} 行`)],
          ),
        ],
      ),
      Timeless.ui.ScrollViewPrimitive.Root(
        {
          store: vm$.methods.textScrollView(),
          class: "min-h-0 flex-1 py-4 px-5 overflow-auto font-mono text-sm leading-1.6 outline-none bg-color-canvas text-color-text-primary",
          attributes: {
            n: "preview-text-lines",
            tabindex: "0",
            "aria-label": "文本内容",
          },
          onMounted(event) {
            vm$.methods.mountTextReader(event, file);
          },
          onUnmounted() {
            vm$.methods.unmountTextReader();
          },
        },
        [
          VirtualListView({
            class: "preview-text-list",
            attributes: { n: "preview-text-list" },
            style: { "min-height": "100%" },
            each: vm$.state.text_lines,
            key: "number",
            size: 32,
            buffer: 8,
            itemHeight: 24,
            externalScroll: true,
            scrollTop: vm$.state.text_scroll_top,
            viewportHeight: vm$.state.text_viewport_height,
            render: render_text_line,
          }),
        ],
      ),
      Show({
        when: vm$.state.text_loading,
        ok() {
          return View(
            {
              class: "flex flex-none items-center justify-center py-2 px-4 gap-2 text-xs border-t border-t-solid border-t-color-default text-color-text-secondary",
              attributes: { n: "preview-text-loading", role: "status" },
            },
            ["正在加载更多内容…"],
          );
        },
      }),
      Show({
        when: computed(vm$.state.text_error, (error) => Boolean(error)),
        ok() {
          return View(
            {
              class: "flex flex-none items-center justify-center py-2 px-4 gap-2 text-xs border-t border-t-solid border-t-color-default text-color-danger",
              attributes: { n: "preview-text-error", role: "alert" },
            },
            [
              ErrorMessage({ tip: vm$.methods.tip, error: vm$.state.text_error }),
              View(
                {
                  as: "button",
                  class: "min-h-control-height-sm py-0 px-3 font-inherit border border-solid border-current rounded-control bg-transparent text-inherit cursor-pointer hover:bg-color-bg-subtle dm-button dm-focus-ring",
                  attributes: {
                    n: "preview-text-retry",
                    type: "button",
                  },
                  onClick() {
                    vm$.methods.retryText();
                  },
                },
                ["重试"],
              ),
            ],
          );
        },
      }),
      Show({
        when: computed(
          vm$.state.text_has_more,
          (has_more) =>
            !has_more &&
            vm$.state.text_lines.value.length > 0 &&
            !vm$.state.text_error.value,
        ),
        ok() {
          return View(
            {
              class: "flex flex-none items-center justify-center py-2 px-4 gap-2 text-xs border-t border-t-solid border-t-color-default text-color-text-secondary preview-text-end",
              attributes: { n: "preview-text-end", role: "status" },
            },
            ["已加载全部内容"],
          );
        },
      }),
    ],
  );
}

function PreviewVideoPlayerView(props) {
  const vm$ = props.store;
  const file = props.file;
  const is_live_playback = vm$.methods.isLivePlayback(file);
  const player$ = vm$.methods.createVideoPlayer(file, {
    autoplay: Boolean(props.autoplay),
  });
  let unsubscribe_tracks = null;
  return View(
    {
      class: [
        "flex relative w-full min-w-0 min-h-0 flex-1 items-center justify-center overflow-hidden video-stage-root video-stage",
        is_live_playback ? "is-live-playback" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: { n: "preview-video-player" },
    },
    [
      Timeless.Video({
        class: props.videoClass,
        src: vm$.methods.videoSource(file),
        controls: true,
        autoplay: Boolean(props.autoplay),
        playsInline: true,
        preload: is_live_playback ? "none" : "metadata",
        attributes: { n: "preview-video-media" },
        onMounted(event) {
          unsubscribe_tracks?.();
          const video = event.target.get$elm();
          // Timeless 0.33 View always renders a div, so render native tracks here.
          unsubscribe_tracks = player$.state.tracks.subscribe({
            onChange(tracks) {
              video
                .querySelectorAll('[data-n="preview-video-subtitle-track"]')
                .forEach((track) => track.remove());
              for (const track of tracks) {
                const element = video.ownerDocument.createElement("track");
                element.setAttribute("data-n", "preview-video-subtitle-track");
                element.kind = "captions";
                element.src = track.src;
                element.label = track.label;
                element.default = track.is_default;
                video.appendChild(element);
                player$.mountTrack(element, track);
              }
            },
          });
          player$.mount(event);
        },
        onUnmounted() {
          player$.unmount();
          unsubscribe_tracks?.();
          unsubscribe_tracks = null;
        },
      }),

      is_live_playback
        ? Tag(
            {
              class: computed(
                vm$.state.live_playback_status,
                (status) => `absolute top-3 left-3 z-1 max-w-parent-gutter overflow-hidden text-ellipsis border border-solid border-color-media bg-color-media-surface text-color-media-text shadow-sm pointer-events-none status-badge is-${status || "waiting"}`,
              ),
              attributes: {
                n: "live-playback-status",
                role: "status",
              },
            },
            [combine({ status: vm$.state.live_playback_status, message: vm$.state.live_playback_message },
              (state) => state.status === "error" ? "播放失败" : state.message)],
          )
        : null,
    ].filter(Boolean),
  );
}

function PreviewFileThumbnail(props) {
  const vm$ = props.store;
  const file = props.file;
  if (file.file_type !== "image" || !file.exists) {
    return View(
      {
        class: "text-color-text-secondary opacity-50 file-icon-target",
        attributes: { n: "file-type-icon" },
      },
      [
        Timeless.Icon({
          name: vm$.methods.fileTypeIcon(file.file_type),
          size: 42,
        }),
      ],
    );
  }
  return View({ class: "flex absolute inset-0 items-center justify-center thumbnail-target" }, [
    View(
      {
        class: "text-color-text-secondary opacity-50 file-icon-target",
        attributes: { n: "file-thumbnail-fallback-icon" },
      },
      [
        Timeless.Icon({
          name: vm$.methods.fileTypeIcon(file.file_type),
          size: 42,
        }),
      ],
    ),
    LazyImg({
      class: "absolute inset-0 w-full h-full object-cover",
      src: vm$.methods.fileURL(file),
      alt: file.name,
      attributes: { loading: "lazy" },
    }),
  ]);
}

function PreviewFileCardView(props) {
  const vm$ = props.store;
  const file = props.file;
  const playable = vm$.methods.filePlayable(file);
  return View(
    {
      as: "button",
      class: [
        "min-w-0 p-0 overflow-hidden font-inherit text-left border border-solid rounded-card border-color-default focus-visible:outline-none dm-panel--soft bg-color-bg-surface text-inherit disabled:opacity-68 cursor-pointer disabled:cursor-default focus-visible:-translate-y-px transition-file-card file-card focus-visible:border-color-color-primary dm-focus-ring",
        playable ? "" : "is-missing",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: file.name,
        disabled: !playable,
      },
      onClick() {
        vm$.methods.openPreview(file);
      },
    },
    [
      View(
        { class: "flex relative w-full h-120px items-center justify-center overflow-hidden bg-color-bg-subtle" },
        [
          PreviewFileThumbnail({ store: vm$, file }),
          file.status
            ? Tag(
                { name: "preview-file-status", class: "absolute top-6px right-6px z-1 bg-color-media-surface text-color-media-muted" },
                [file.status],
              )
            : null,
        ].filter(Boolean),
      ),
      View(
        { class: "py-2.5 px-3" },
        [
          View(
            {
              class: "mb-1 overflow-hidden text-md font-500 whitespace-nowrap text-ellipsis",
              attributes: { title: file.name },
            },
            [file.name],
          ),
          View({ class: "flex justify-between text-xs text-color-text-disabled" }, [
            View({}, [vm$.methods.formatBytes(file.size)]),
            View({}, [playable ? "" : "missing"]),
          ]),
          file.status === "downloading" && file.progress > 0
            ? View({ class: "h-3px mt-1.5 overflow-hidden rounded-round bg-color-border" }, [
                View({
                  class: "h-full rounded-round bg-color-primary transition-width-slow",
                  style: { width: `${file.progress}%` },
                }),
              ])
            : null,
        ].filter(Boolean),
      ),
    ],
  );
}

function PreviewFileGridView(props) {
  const vm$ = props.store;
  const files = props.files;
  return View({ class: "min-h-0 flex-1 p-4 overflow-auto " }, [
    View({ as: "h2", class: "mt-0 mb-3 mx-0 text-ui font-600 text-color-text-primary" }, [
      `文件 (${files.length})`,
    ]),
    files.length > 0
      ? View({ class: "grid grid-cols-fill-45 max-520:grid-cols-fill-35 gap-3" }, [
          For({
            each: files,
            render(file_) {
              const file =
                file_ && file_.value !== undefined ? file_.value : file_;
              return PreviewFileCardView({ store: vm$, file });
            },
          }),
        ])
      : PreviewStateView({ message: "暂无文件" }),
  ]);
}

function PreviewGalleryPlaceholderView(props) {
  const vm$ = props.store;
  const file = props.file;
  return View({ class: "grid w-full place-items-center content-center p-8 gap-3 text-center text-color-text-secondary" }, [
    View(
      {
        class: "grid w-16 h-16 place-items-center border-default border-solid rounded-content border-color-default bg-color-bg-surface text-color-text-secondary",
        attributes: { n: "gallery-placeholder-file-type-icon" },
      },
      [
        Timeless.Icon({
          name: vm$.methods.fileTypeIcon(file.file_type),
          size: 30,
        }),
      ],
    ),
    View(
      {
        class: "max-w-readable-title overflow-hidden text-base font-semibold whitespace-nowrap text-ellipsis text-color-text-primary",
        attributes: { title: file.name },
      },
      [file.name],
    ),
    View({ class: "max-w-48ch text-sm leading-relaxed" }, [
      vm$.methods.filePlayable(file)
        ? "此文件类型无法在画廊中直接预览，请打开文件查看。"
        : "文件尚未下载、下载未完成，或本地文件已被删除。",
    ]),
  ]);
}

function PreviewGalleryMediaView(props) {
  const vm$ = props.store;
  const file = props.file;
  const url = vm$.methods.fileURL(file);
  if (!vm$.methods.filePlayable(file)) {
    return PreviewGalleryPlaceholderView({ store: vm$, file });
  }
  if (vm$.methods.isHTMLFile(file)) {
    return PreviewHTMLFileView({
      store: vm$,
      file,
      class: "media-html-reader",
      nodeName: "gallery-html-reader",
    });
  }
  if (vm$.methods.isTextFile(file)) {
    return PreviewTextFileView({
      store: vm$,
      file,
      class: "media-text-reader",
      nodeName: "gallery-text-reader",
    });
  }
  if (file.file_type === "image") {
    return Timeless.Img({
      class: "block w-full h-full object-contain",
      src: url,
      alt: file.name,
      attributes: { loading: "eager" },
    });
  }
  if (file.file_type === "video") {
    return PreviewVideoPlayerView({
      store: vm$,
      file,
      videoClass: "block w-full h-full object-contain outline-none",
      autoplay: vm$.methods.isLivePlayback(file),
    });
  }
  if (file.file_type === "audio") {
    return View({ class: "grid w-full place-items-center p-8 gap-6" }, [
      View(
        {
          class: "grid w-16 h-16 place-items-center border-default border-solid rounded-content border-color-default bg-color-bg-surface text-color-text-secondary",
          attributes: { n: "gallery-audio-file-type-icon" },
        },
        [
          Timeless.Icon({
            name: vm$.methods.fileTypeIcon(file.file_type),
            size: 30,
          }),
        ],
      ),
      Timeless.Audio({
        class: "w-full max-w-130",
        src: url,
        controls: true,
        preload: "metadata",
      }),
    ]);
  }
  if (file.file_type === "pdf") {
    return Timeless.Webview({
      class: "block w-full h-full border-0 bg-color-document-canvas",
      href: url,
      attributes: {
        title: file.name,
        loading: "eager",
      },
    });
  }
  return PreviewGalleryPlaceholderView({ store: vm$, file });
}

function PreviewGalleryStageView(props) {
  const vm$ = props.store;
  const file = props.file;
  const meta = [
    vm$.methods.fileTypeLabel(file.file_type),
    vm$.methods.formatBytes(file.size),
    vm$.methods.filePlayable(file) ? file.status : "文件不存在",
  ]
    .filter(Boolean)
    .join(" · ");
  return View(
    {
      class: `flex w-full min-w-0 min-h-0 h-full flex-1 flex-col overflow-hidden bg-color-bg-surface gallery-stage is-${file.file_type}`,
    },
    [
      View({ class: "flex h-full min-h-0 min-w-0 flex-1 items-center justify-center overflow-hidden bg-color-bg-subtle gallery-viewport media-viewport " }, [
        PreviewGalleryMediaView({ store: vm$, file }),
      ]),
      View(
        { class: "grid grid-cols-auto-content-auto max-520:grid-cols-label-content items-center py-3 px-4 gap-3 border-t-default border-t-solid border-t-color-default" },
        [
          View(
            {
              class: "grid w-control-height-sm h-control-height-sm place-items-center rounded-content bg-color-primary-soft text-color-primary",
              attributes: { n: "gallery-caption-file-type-icon" },
            },
            [
              Timeless.Icon({
                name: vm$.methods.fileTypeIcon(file.file_type),
                size: 18,
              }),
            ],
          ),
          View({ class: "min-w-0" }, [
            View(
              {
                class: "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis",
                attributes: { title: file.name },
              },
              [file.name],
            ),
            View({ class: "mt-0.5 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-color-text-secondary" }, [meta]),
          ]),
          file.exists
            ? View(
                {
                  as: "button",
                  class: "inline-flex min-h-control-height-sm max-520:col-span-full items-center max-520:justify-center py-0 px-3 gap-1.5 font-inherit text-sm border-default border-solid rounded-control border-color-default bg-transparent text-color-text-primary cursor-pointer hover:bg-color-bg-subtle dm-button dm-focus-ring",
                  attributes: {
                    n: "gallery-show-file-action",
                    type: "button",
                    title: `在文件夹中显示 ${file.name}`,
                    "aria-label": `在文件夹中显示 ${file.name}`,
                  },
                  onClick() {
                    vm$.methods.showFile(file);
                  },
                },
                [
                  Timeless.Icon({
                    name: "folder",
                    size: 15,
                    attributes: { n: "gallery-show-file-icon" },
                  }),
                  View({ attributes: { n: "gallery-show-file-label" } }, [
                    "文件",
                  ]),
                ],
              )
            : null,
        ].filter(Boolean),
      ),
    ],
  );
}

function PreviewGalleryFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  const playable = vm$.methods.filePlayable(file);
  return View(
    {
      as: "button",
      class: computed(vm$.state.gallery_file, (selected_file) =>
        [
          "grid min-w-0 grid-cols-16-main items-center p-2 gap-3 font-inherit text-left border-default border-solid rounded-card border-color-default bg-color-bg-surface text-color-text-primary disabled:opacity-62 cursor-pointer disabled:cursor-default transition-surface-fast gallery-list-item dm-focus-ring",
          selected_file === file ? "is-selected" : "",
          playable ? "" : "is-missing",
        ]
          .filter(Boolean)
          .join(" "),
      ),
      attributes: {
        type: "button",
        title: file.name,
        disabled: !playable,
        "aria-pressed": computed(vm$.state.gallery_file, (selected_file) =>
          selected_file === file ? "true" : "false",
        ),
      },
      onClick() {
        vm$.methods.selectGalleryFile(file);
      },
    },
    [
      View({ class: "flex relative w-64px h-48px items-center justify-center overflow-hidden rounded-content bg-color-bg-subtle thumbnail-frame " }, [
        PreviewFileThumbnail({ store: vm$, file }),
      ]),
      View({ class: "min-w-0" }, [
        View(
          {
            class: "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View({ class: "mt-0.5 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-color-text-secondary" }, [
          [
            vm$.methods.fileTypeLabel(file.file_type),
            vm$.methods.formatBytes(file.size),
            playable ? file.status : "文件不存在",
          ]
            .filter(Boolean)
            .join(" · "),
        ]),
      ]),
    ],
  );
}

function PreviewFileGalleryView(props) {
  const vm$ = props.store;
  const files = props.files;
  if (files.length === 0) {
    return View({ class: "flex min-w-0 min-h-0 flex-1 flex-col p-4 gap-4 overflow-auto gallery-layout " }, [
      PreviewStateView({ message: "暂无文件" }),
    ]);
  }
  return View(
    {
      class: [
        "flex min-w-0 min-h-0 flex-1 flex-col p-4 gap-4 overflow-auto gallery-layout ",
        files.length === 1 ? "is-single" : "",
      ]
        .filter(Boolean)
        .join(" "),
      role: "region",
      attributes: { "aria-label": "文件画廊" },
    },
    [
      View({ class: "flex w-full min-w-0 min-h-0 h-0 flex-1" }, [
        For({
          each: files,
          render(file_) {
            const file =
              file_ && file_.value !== undefined ? file_.value : file_;
            return Show({
              when: computed(
                vm$.state.gallery_file,
                (selected_file) => selected_file === file,
              ),
              ok() {
                return PreviewGalleryStageView({ store: vm$, file });
              },
            });
          },
        }),
      ]),
      files.length > 1
        ? View({ class: "flex-none pt-0 pb-3 px-3" }, [
            View({ class: "grid grid-auto-flow-column auto-cols-55-70 p-border-width gap-2 overflow-x-auto overscroll-behavior-inline-contain" }, [
              For({
                each: files,
                render(file_) {
                  const file =
                    file_ && file_.value !== undefined ? file_.value : file_;
                  return PreviewGalleryFileView({ store: vm$, file });
                },
              }),
            ]),
          ])
        : null,
    ].filter(Boolean),
  );
}

function PreviewTaskBodyView(props) {
  const vm$ = props.store;
  const task = props.task;
  if (props.fileView === "gallery") {
    return [
      PreviewHeaderView({ task, fileCount: task.files.length }),
      PreviewFileGalleryView({ store: vm$, files: task.files }),
    ];
  }
  const existing_files = task.files.filter(vm$.methods.filePlayable);
  const single_file = existing_files.length === 1 ? existing_files[0] : null;
  return [
    PreviewHeaderView({ task }),
    single_file &&
    (vm$.methods.isHTMLFile(single_file) ||
      vm$.methods.isTextFile(single_file) ||
      ["video", "image"].includes(single_file.file_type))
      ? PreviewSingleFileView({ store: vm$, file: single_file })
      : PreviewFileGridView({ store: vm$, files: task.files }),
  ];
}

function PreviewDownloadLinkView(props) {
  const vm$ = props.store;
  return View(
    {
      as: "a",
      class: "inline-block mt-3 text-decoration-none hover:text-decoration-underline text-color-info dm-button dm-focus-ring",
      attributes: {
        href: vm$.methods.fileURL(props.file),
        target: "_blank",
        rel: "noopener noreferrer",
      },
    },
    ["下载或打开"],
  );
}

function PreviewZipView(props) {
  const vm$ = props.store;
  const file = props.file;
  return View({ class: "flex min-h-0 flex-1 items-center justify-center p-5 max-520:p-3 overflow-auto media-overlay is-zip" }, [
    Show({
      when: vm$.state.zip_loading,
      ok() {
        return PreviewStateView({
          loading: true,
          message: "正在加载压缩包预览…",
        });
      },
      else() {
        return Show({
          when: computed(vm$.state.zip_error, (error) => Boolean(error)),
          ok() {
            return PreviewStateView({
              role: "alert",

              action: PreviewDownloadLinkView({ store: vm$, file }),
            });
          },
          else() {
            return Show({
              when: computed(
                vm$.state.zip_images,
                (images) => images.length > 0,
              ),
              ok() {
                return View({ class: "grid w-full grid-cols-fill-45 max-520:grid-cols-fill-35 gap-3" }, [
                  For({
                    each: vm$.state.zip_images,
                    render(image_) {
                      const image =
                        image_ && image_.value !== undefined
                          ? image_.value
                          : image_;
                      return View(
                        {
                          as: "figure",
                          class: "m-0 overflow-hidden border border-solid rounded-card border-color-default bg-color-bg-surface",
                          attributes: { title: image.name },
                        },
                        [
                          Timeless.Img({
                            class: "block w-full aspect-square object-contain bg-color-canvas",
                            src: image.url,
                            alt: image.name,
                            attributes: { loading: "lazy" },
                          }),
                          View(
                            {
                              as: "figcaption",
                              class: "py-2 px-2.5 overflow-hidden text-sm whitespace-nowrap text-ellipsis text-color-text-secondary",
                            },
                            [image.name],
                          ),
                        ],
                      );
                    },
                  }),
                ]);
              },
              else() {
                return PreviewStateView({
                  message: "压缩包中没有可预览的图片",
                  action: PreviewDownloadLinkView({ store: vm$, file }),
                });
              },
            });
          },
        });
      },
    }),
  ]);
}

function PreviewOverlayMediaView(props) {
  const vm$ = props.store;
  const file = props.file;
  const url = vm$.methods.fileURL(file);
  if (vm$.methods.isHTMLFile(file)) {
    return PreviewHTMLFileView({
      store: vm$,
      file,
      class: "overlay-html-reader",
      nodeName: "overlay-html-reader",
    });
  }
  if (vm$.methods.isTextFile(file)) {
    return PreviewTextFileView({
      store: vm$,
      file,
      class: "overlay-text-reader",
      nodeName: "overlay-text-reader",
    });
  }
  if (file.file_type === "image") {
    return Timeless.Img({
      class: "max-w-full max-h-full object-contain rounded-content outline-none",
      src: url,
      alt: file.name,
    });
  }
  if (file.file_type === "video") {
    return PreviewVideoPlayerView({
      store: vm$,
      file,
      videoClass: "max-w-full max-h-full object-contain rounded-content outline-none",
      autoplay: true,
    });
  }
  if (file.file_type === "audio") {
    return Timeless.Audio({
      class: "w-full max-w-100",
      src: url,
      controls: true,
      autoplay: true,
    });
  }
  if (file.file_type === "pdf") {
    return Timeless.Webview({
      class: "w-full h-full border-0 rounded-content bg-color-document-canvas",
      href: url,
      attributes: {
        title: file.name,
      },
    });
  }
  return PreviewStateView({
    message: `${file.name} · ${vm$.methods.formatBytes(file.size)}`,
    action: PreviewDownloadLinkView({ store: vm$, file }),
  });
}

function PreviewOverlayView(props) {
  const vm$ = props.store;
  const file = props.file;
  return View(
    {
      class: "flex fixed inset-0 z-1000 flex-col bg-color-media-canvas text-color-text-primary overlay-root",
      onClick(event) {
        if (event.target === event.currentTarget) {
          vm$.methods.closePreview();
        }
      },
    },
    [
      View({ class: "flex flex-none items-center justify-between py-3 px-5 gap-4 border-b border-b-solid border-b-color-default bg-color-media-surface" }, [
        View(
          {
            class: "min-w-0 overflow-hidden text-base font-500 whitespace-nowrap text-ellipsis",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View(
          {
            as: "button",
            class: "py-1.5 px-4 font-inherit text-md border border-solid border-color-media rounded-control bg-none text-color-media-text cursor-pointer hover:bg-color-border dm-button dm-focus-ring",
            attributes: { type: "button" },
            onClick() {
              vm$.methods.closePreview();
            },
          },
          ["关闭"],
        ),
      ]),
      file.file_type === "zip"
        ? PreviewZipView({ store: vm$, file })
        : View({ class: "flex min-h-0 flex-1 items-center justify-center p-5 max-520:p-3 overflow-auto media-overlay" }, [
            PreviewOverlayMediaView({ store: vm$, file }),
          ]),
    ],
  );
}

function PreviewPageView(props) {
  const vm$ = PreviewViewModel(props);
  let unsubscribe_task_id = null;
  let unsubscribe_escape = null;

  return View(
    {
      class: ["flex relative w-full min-w-0 h-full min-h-0 flex-col overflow-hidden font-sans bg-image-page-grid bg-size-grid bg-color-bg-canvas bg-color-canvas text-color-text-primary media-pane", props.embedded ? "is-embedded" : ""]
        .filter(Boolean)
        .join(" "),
      onMounted() {
        unsubscribe_escape = props.app.onEscapeKeyDown(
          vm$.methods.closePreview,
        );
        if (props.taskId && typeof props.taskId.subscribe === "function") {
          unsubscribe_task_id = props.taskId.subscribe({
            onChange(task_id) {
              vm$.methods.loadTask(task_id);
            },
          });
        }
        vm$.methods.ready();
      },
      onUnmounted() {
        if (typeof unsubscribe_escape === "function") {
          unsubscribe_escape();
          unsubscribe_escape = null;
        }
        if (typeof unsubscribe_task_id === "function") {
          unsubscribe_task_id();
          unsubscribe_task_id = null;
        }
        vm$.methods.destroy();
      },
    },
    [
      Show({
        when: vm$.state.loading,
        ok() {
          return PreviewStateView({ loading: true, message: "正在加载预览…" });
        },
        else() {
          return Show({
            when: computed(vm$.state.error, (error) => Boolean(error)),
            ok() {
              return PreviewStateView({
                role: "alert",
                title: "无法打开预览",

                action: View(
                  {
                    as: "button",
                    class:
                      "py-1.5 px-4 font-inherit text-md border border-solid border-color-media rounded-control dm-button--primary bg-none text-color-media-text cursor-pointer state-retry-action hover:bg-color-border dm-button dm-focus-ring",
                    attributes: { type: "button" },
                    onClick() {
                      vm$.methods.retry();
                    },
                  },
                  ["重试"],
                ),
              });
            },
            else() {
              return Show({
                when: computed(vm$.state.task, (task) => Boolean(task)),
                ok() {
                  return PreviewTaskBodyView({
                    store: vm$,
                    task: vm$.state.task.value,
                    fileView: props.fileView,
                  });
                },
              });
            },
          });
        },
      }),
      Show({
        when: computed(vm$.state.active_file, (file) => Boolean(file)),
        ok() {
          return PreviewOverlayView({
            store: vm$,
            file: vm$.state.active_file.value,
          });
        },
      }),
    ],
  );
}

export { PreviewGalleryMediaView };
export default PreviewPageView;

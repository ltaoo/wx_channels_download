import { ErrorMessage } from "@/dmui.js";
import { PreviewViewModel } from "./preview.model.js";

function PreviewStateView(props) {
  return View(
    {
      class: [
        "group-preview-state flex w-[min(520px,_calc(100%_-_var(--dm-space-8)))] min-h-360px flex-[0_0_auto] flex-col items-center justify-center gap-2 m-auto p-[var(--dm-space-16)_var(--dm-space-5)] text-preview-muted text-center bg-preview-panel border-[var(--dm-border-width)_solid_var(--dm-preview-border)] rounded-card shadow-lg [&_.group-preview-retry]:min-h-control-height-md [&_.group-preview-retry]:mt-3 [&_.group-preview-retry]:p-[0_var(--dm-space-4)] [&_.group-preview-retry]:text-color-on-primary [&_.group-preview-retry]:bg-color-primary-fill [&_.group-preview-retry]:border-color-color-primary-fill [&_.group-preview-retry]:rounded-control [&_.group-preview-retry:hover]:text-color-on-primary [&_.group-preview-retry:hover]:bg-color-primary-fill-hover [&_.group-preview-retry:hover]:border-color-color-primary-fill-hover dm-empty-state",
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
        ? View({ as: "h3", class: "m-0 text-preview-text text-lg font-semibold" }, [props.title])
        : null,
      props.message
        ? View({ as: "p", class: "m-0 text-preview-muted text-md leading-relaxed overflow-wrap-anywhere" }, [props.message])
        : null,
      props.action || null,
    ].filter(Boolean),
  );
}

function PreviewHeaderView(props) {
  const task = props.task;
  const account = task.account;
  return View(
    { class: "flex-[0_0_auto] p-[var(--dm-space-3)_var(--dm-space-4)] bg-preview-panel border-bottom-[1px_solid_var(--dm-preview-border)] text-color-text-primary shadow-divider -webkit-backdrop-filter-[blur(14px)] backdrop-filter-[blur(14px)] " },
    [
      account
        ? View({ class: "flex items-center gap-2 mb-1-5" }, [
            account.avatar_url
              ? LazyImg({
                  class: "w-24px h-24px object-cover bg-preview-panel-soft rounded-avatar",
                  src: account.avatar_url,
                  alt: "",
                  attributes: { referrerpolicy: "no-referrer" },
                })
              : null,
            View({ class: "overflow-hidden text-preview-text text-md font-500 text-ellipsis whitespace-nowrap" }, [
              account.nickname || account.external_id || "",
            ]),
          ])
        : null,
      View({ class: "-webkit-box mb-1 overflow-hidden text-ui font-600 leading-[1.4] -webkit-box-orient-vertical -webkit-line-clamp-2 dm-font-display dm-font-bold" }, [
        task.title,
      ]),
      View(
        { class: "flex items-center gap-2 text-preview-muted text-sm" },
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
        videoClass: "w-auto max-w-full h-auto max-h-[calc(100%_-_28px)] outline-none",
        autoplay: true,
      }),
      View({ class: "max-w-full p-[var(--dm-space-1)_var(--dm-space-3)] overflow-hidden text-preview-subtle text-sm text-center text-ellipsis whitespace-nowrap" }, [file.name]),
    ]);
  }
  return View({ class: "flex min-h-0 flex-1 flex-col items-center justify-center p-5 overflow-auto" }, [
    LazyImg({
      class: "max-w-full max-h-[calc(100%_-_28px)] object-contain rounded-content",
      src: url,
      alt: file.name,
    }),
    View({ class: "max-w-full p-[var(--dm-space-1)_var(--dm-space-3)] overflow-hidden text-preview-subtle text-sm text-center text-ellipsis whitespace-nowrap" }, [file.name]),
  ]);
}

function PreviewHTMLFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  return View(
    {
      class: ["relative flex min-w-0 min-h-0 flex-1 overflow-hidden text-[#29251d] color-scheme-light bg-color-document-canvas [&_>_.group-preview-state]:absolute [&_>_.group-preview-state]:inset-0 [&_>_.group-preview-state]:z-1", props.class].filter(Boolean).join(" "),
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
        class: "block w-full min-w-0 min-h-0 p-5 overflow-auto text-[#29251d] leading-relaxed overflow-wrap-anywhere bg-color-document-canvas [&_>_:first-child]:mt-0 [&_>_:last-child]:mb-0 [&_img]:max-w-full [&_video]:max-w-full [&_audio]:max-w-full [&_iframe]:max-w-full [&_img]:h-auto [&_video]:h-auto [&_pre]:max-w-full [&_pre]:overflow-x-auto [&_pre]:whitespace-pre [&_table]:max-w-full [&_table]:border-collapse-collapse",
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
                  "group-preview-retry p-[var(--dm-space-1-5)_var(--dm-space-4)] text-color-media-text font-inherit text-md bg-none border-[1px_solid_var(--dm-color-media-border)] rounded-control cursor-pointer hover:bg-preview-border dm-button dm-button--primary dm-focus-ring",
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
        class: "grid grid-cols-[max-content_minmax(0,_1fr)] gap-4 min-h-[1.6em]",
        attributes: {
          n: "preview-text-line",
          "data-line": String(line.number),
        },
      },
      [
        View(
          {
            class: "min-w-4ch text-preview-subtle text-right user-select-none",
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
      class: ["flex min-w-0 min-h-0 flex-1 flex-col overflow-hidden bg-preview-panel", props.class].filter(Boolean).join(" "),
      attributes: {
        n: props.nodeName || "preview-text-reader",
        role: "region",
        "aria-label": `文本预览：${file.name}`,
      },
    },
    [
      View(
        {
          class: "flex flex-[0_0_auto] items-center justify-between gap-3 p-[var(--dm-space-2-5)_var(--dm-space-4)] border-bottom-[1px_solid_var(--dm-preview-border)]",
          attributes: { n: "preview-text-toolbar" },
        },
        [
          View(
            {
              class: "min-w-0 overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap",
              attributes: { n: "preview-text-title", title: file.name },
            },
            [file.name],
          ),
          View(
            {
              class: "flex-[0_0_auto] text-preview-muted text-xs",
              attributes: { n: "preview-text-count", "aria-live": "polite" },
            },
            [computed(vm$.state.text_lines, (lines) => `${lines.length} 行`)],
          ),
        ],
      ),
      Timeless.ui.ScrollViewPrimitive.Root(
        {
          store: vm$.methods.textScrollView(),
          class: "min-h-0 flex-1 overflow-auto p-[var(--dm-space-4)_var(--dm-space-5)] text-preview-text font-[var(--dm-font-family-mono,_ui-monospace,_SFMono-Regular,_Menlo,_Monaco,_Consolas,_monospace)] text-sm leading-[1.6] [background:var(--dm-preview-bg)] outline-none",
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
              class: "flex-[0_0_auto] text-preview-muted text-xs flex items-center justify-center gap-2 p-[var(--dm-space-2)_var(--dm-space-4)] border-top-[1px_solid_var(--dm-preview-border)]",
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
              class: "flex-[0_0_auto] text-xs flex items-center justify-center gap-2 p-[var(--dm-space-2)_var(--dm-space-4)] border-top-[1px_solid_var(--dm-preview-border)] text-color-danger",
              attributes: { n: "preview-text-error", role: "alert" },
            },
            [
              ErrorMessage({ tip: vm$.methods.tip, error: vm$.state.text_error }),
              View(
                {
                  as: "button",
                  class: "min-h-control-height-sm p-[0_var(--dm-space-3)] text-inherit font-inherit bg-transparent border-[1px_solid_currentColor] rounded-control cursor-pointer hover:bg-preview-panel-soft dm-button dm-focus-ring",
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
              class: "flex-[0_0_auto] text-preview-muted text-xs flex items-center justify-center gap-2 p-[var(--dm-space-2)_var(--dm-space-4)] border-top-[1px_solid_var(--dm-preview-border)] preview-text-end",
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
        "group-preview-video-player relative flex w-full min-w-0 min-h-0 flex-1 items-center justify-center overflow-hidden [&.is-live-playback]:bg-color-media-canvas",
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
                (status) => `absolute top-3 left-3 z-1 max-w-[calc(100%_-_var(--dm-space-6))] overflow-hidden text-color-media-text text-ellipsis bg-color-media-surface border-[1px_solid_var(--dm-color-media-border)] shadow-sm pointer-events-none [&.is-playing]:text-color-success [&.is-ready]:text-color-success [&.is-error]:text-color-danger is-${status || "waiting"}`,
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
        class: "group-preview-file-icon text-preview-muted opacity-[0.5]",
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
  return View({ class: "group-preview-file-thumbnail-wrap absolute inset-0 flex items-center justify-center" }, [
    View(
      {
        class: "group-preview-file-icon text-preview-muted opacity-[0.5]",
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
        "min-w-0 p-0 overflow-hidden text-inherit font-inherit text-left bg-preview-panel border-[1px_solid_var(--dm-preview-border)] rounded-card cursor-pointer transition-[border-color_var(--dm-duration-base)_var(--dm-ease-standard),_transform_var(--dm-duration-fast)_var(--dm-ease-standard)] [&:hover:not(:disabled)]:border-color-preview-accent [&:hover:not(:disabled)]:outline-none [&:hover:not(:disabled)]:transform-[translateY(-1px)] focus-visible:border-color-preview-accent focus-visible:outline-none focus-visible:transform-[translateY(-1px)] disabled:cursor-default disabled:opacity-[0.68] [&.is-missing]:cursor-default [&.is-missing]:opacity-[0.68] dm-panel--soft dm-focus-ring",
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
        { class: "relative flex w-full h-120px items-center justify-center overflow-hidden bg-preview-panel-soft" },
        [
          PreviewFileThumbnail({ store: vm$, file }),
          file.status
            ? Tag(
                { name: "preview-file-status", class: "absolute top-6px right-6px z-1 text-color-media-muted bg-color-media-surface" },
                [file.status],
              )
            : null,
        ].filter(Boolean),
      ),
      View(
        { class: "p-[var(--dm-space-2-5)_var(--dm-space-3)]" },
        [
          View(
            {
              class: "mb-1 overflow-hidden text-md font-500 text-ellipsis whitespace-nowrap",
              attributes: { title: file.name },
            },
            [file.name],
          ),
          View({ class: "flex justify-between text-preview-subtle text-xs" }, [
            View({}, [vm$.methods.formatBytes(file.size)]),
            View({}, [playable ? "" : "missing"]),
          ]),
          file.status === "downloading" && file.progress > 0
            ? View({ class: "h-3px mt-1-5 overflow-hidden bg-preview-border rounded-round" }, [
                View({
                  class: "h-full bg-preview-accent rounded-round transition-[width_var(--dm-duration-slow)_var(--dm-ease-standard)]",
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
    View({ as: "h2", class: "m-[0_0_var(--dm-space-3)] text-preview-text text-ui font-600" }, [
      `文件 (${files.length})`,
    ]),
    files.length > 0
      ? View({ class: "grid grid-cols-[repeat(auto-fill,_minmax(180px,_1fr))] gap-3 max-520:grid-cols-[repeat(auto-fill,_minmax(140px,_1fr))]" }, [
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
  return View({ class: "w-full grid place-items-center content-center gap-3 p-8 text-preview-muted text-center" }, [
    View(
      {
        class: "w-16 h-16 grid place-items-center text-preview-muted bg-preview-panel border-[var(--dm-border-width)_solid_var(--dm-preview-border)] rounded-content",
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
        class: "max-w-[min(520px,_90%)] overflow-hidden text-preview-text text-base font-semibold text-ellipsis whitespace-nowrap",
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
      class: "group-preview-gallery-html-reader",
      nodeName: "gallery-html-reader",
    });
  }
  if (vm$.methods.isTextFile(file)) {
    return PreviewTextFileView({
      store: vm$,
      file,
      class: "group-preview-gallery-text-reader",
      nodeName: "gallery-text-reader",
    });
  }
  if (file.file_type === "image") {
    return Timeless.Img({
      class: "w-full h-full block object-contain",
      src: url,
      alt: file.name,
      attributes: { loading: "eager" },
    });
  }
  if (file.file_type === "video") {
    return PreviewVideoPlayerView({
      store: vm$,
      file,
      videoClass: "w-full h-full block object-contain outline-none",
      autoplay: vm$.methods.isLivePlayback(file),
    });
  }
  if (file.file_type === "audio") {
    return View({ class: "w-full grid place-items-center gap-6 p-8" }, [
      View(
        {
          class: "w-16 h-16 grid place-items-center text-preview-muted bg-preview-panel border-[var(--dm-border-width)_solid_var(--dm-preview-border)] rounded-content",
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
        class: "w-[min(520px,_100%)]",
        src: url,
        controls: true,
        preload: "metadata",
      }),
    ]);
  }
  if (file.file_type === "pdf") {
    return Timeless.Webview({
      class: "w-full h-full block bg-color-document-canvas border-0",
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
      class: `w-full min-w-0 flex min-h-0 h-full flex-1 flex-col overflow-hidden bg-preview-panel [&.is-video_.group-preview-gallery-viewport]:bg-color-media-canvas is-${file.file_type}`,
    },
    [
      View({ class: "group-preview-gallery-viewport h-full min-h-0 min-w-0 flex-1 flex items-center justify-center overflow-hidden bg-preview-panel-soft [&_>_.group-preview-gallery-text-reader]:align-self-stretch [&_>_.group-preview-gallery-text-reader]:w-full [&_>_.group-preview-gallery-text-reader]:h-full [&_>_.group-preview-gallery-text-reader]:min-w-0 [&_>_.group-preview-gallery-text-reader]:min-h-0 [&_>_.group-preview-gallery-html-reader]:align-self-stretch [&_>_.group-preview-gallery-html-reader]:w-full [&_>_.group-preview-gallery-html-reader]:h-full [&_>_.group-preview-gallery-html-reader]:min-w-0 [&_>_.group-preview-gallery-html-reader]:min-h-0 [&_>_.group-preview-video-player]:h-full [.group-content-detail-media-viewport&]:flex [.group-content-detail-media-viewport&]:h-[min(72vh,_760px)] [.group-content-detail-media-viewport&]:min-h-0 [.group-content-detail-media-viewport&]:flex-none" }, [
        PreviewGalleryMediaView({ store: vm$, file }),
      ]),
      View(
        { class: "grid grid-cols-[auto_minmax(0,_1fr)_auto] items-center gap-3 p-[var(--dm-space-3)_var(--dm-space-4)] border-top-[var(--dm-border-width)_solid_var(--dm-preview-border)] max-520:grid-cols-[auto_minmax(0,_1fr)]" },
        [
          View(
            {
              class: "w-control-height-sm h-control-height-sm grid place-items-center text-color-primary bg-color-primary-soft rounded-content",
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
                class: "overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap",
                attributes: { title: file.name },
              },
              [file.name],
            ),
            View({ class: "mt-0-5 overflow-hidden text-preview-muted text-xs text-ellipsis whitespace-nowrap" }, [meta]),
          ]),
          file.exists
            ? View(
                {
                  as: "button",
                  class: "inline-flex items-center gap-1-5 min-h-control-height-sm p-[0_var(--dm-space-3)] text-preview-text font-inherit text-sm bg-transparent border-[var(--dm-border-width)_solid_var(--dm-preview-border)] rounded-control cursor-pointer hover:bg-preview-panel-soft max-520:grid-column-[1_/_-1] max-520:justify-center dm-button dm-focus-ring",
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
          "min-w-0 grid grid-cols-[64px_minmax(0,_1fr)] items-center gap-3 p-2 text-preview-text font-inherit text-left bg-preview-panel border-[var(--dm-border-width)_solid_var(--dm-preview-border)] rounded-card cursor-pointer transition-[background_var(--dm-duration-fast)_var(--dm-ease-standard),_border-color_var(--dm-duration-fast)_var(--dm-ease-standard)] [&:hover:not(:disabled)]:bg-color-primary-soft [&:hover:not(:disabled)]:border-color-color-primary-border [&.is-selected]:bg-color-primary-soft [&.is-selected]:border-color-color-primary-border [&.is-selected]:shadow-[inset_3px_0_0_var(--dm-preview-accent)] disabled:cursor-default disabled:opacity-[0.62] [&.is-missing]:cursor-default [&.is-missing]:opacity-[0.62] dm-focus-ring",
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
      View({ class: "relative w-64px h-48px flex items-center justify-center overflow-hidden bg-preview-panel-soft rounded-content [&_.group-preview-file-thumbnail-wrap]:absolute [&_.group-preview-file-icon]:text-2xl" }, [
        PreviewFileThumbnail({ store: vm$, file }),
      ]),
      View({ class: "min-w-0" }, [
        View(
          {
            class: "overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View({ class: "mt-0-5 overflow-hidden text-preview-muted text-xs text-ellipsis whitespace-nowrap" }, [
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
    return View({ class: "flex min-w-0 min-h-0 flex-1 flex-col gap-4 p-4 overflow-auto [&.is-single]:overflow-hidden " }, [
      PreviewStateView({ message: "暂无文件" }),
    ]);
  }
  return View(
    {
      class: [
        "flex min-w-0 min-h-0 flex-1 flex-col gap-4 p-4 overflow-auto [&.is-single]:overflow-hidden ",
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
        ? View({ class: "flex-[0_0_auto] p-[0_12px_12px_12px]" }, [
            View({ class: "grid grid-auto-columns-[minmax(220px,_280px)] grid-auto-flow-column gap-2 p-border-width overflow-x-auto overscroll-behavior-inline-contain" }, [
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
      class: "inline-block mt-3 text-color-info text-decoration-none hover:text-decoration-underline dm-button dm-focus-ring",
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
  return View({ class: "[&_>_.group-preview-overlay-text-reader]:align-self-stretch [&_>_.group-preview-overlay-text-reader]:w-full [&_>_.group-preview-overlay-text-reader]:h-full [&_>_.group-preview-overlay-text-reader]:min-w-0 [&_>_.group-preview-overlay-text-reader]:min-h-0 [&_>_.group-preview-overlay-html-reader]:align-self-stretch [&_>_.group-preview-overlay-html-reader]:w-full [&_>_.group-preview-overlay-html-reader]:h-full [&_>_.group-preview-overlay-html-reader]:min-w-0 [&_>_.group-preview-overlay-html-reader]:min-h-0 flex min-h-0 flex-1 items-center justify-center p-5 overflow-auto [&.is-zip]:items-start [&.is-zip]:justify-start [&_>_.group-preview-video-player]:h-full max-520:p-3 is-zip" }, [
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
                return View({ class: "grid w-full grid-cols-[repeat(auto-fill,_minmax(180px,_1fr))] gap-3 max-520:grid-cols-[repeat(auto-fill,_minmax(140px,_1fr))]" }, [
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
                          class: "m-0 overflow-hidden bg-preview-panel border-[1px_solid_var(--dm-preview-border)] rounded-card",
                          attributes: { title: image.name },
                        },
                        [
                          Timeless.Img({
                            class: "block w-full aspect-ratio-[1_/_1] object-contain [background:var(--dm-preview-bg)]",
                            src: image.url,
                            alt: image.name,
                            attributes: { loading: "lazy" },
                          }),
                          View(
                            {
                              as: "figcaption",
                              class: "p-[var(--dm-space-2)_var(--dm-space-2-5)] overflow-hidden text-preview-muted text-sm text-ellipsis whitespace-nowrap",
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
      class: "group-preview-overlay-html-reader",
      nodeName: "overlay-html-reader",
    });
  }
  if (vm$.methods.isTextFile(file)) {
    return PreviewTextFileView({
      store: vm$,
      file,
      class: "group-preview-overlay-text-reader",
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
      class: "w-[min(400px,_100%)]",
      src: url,
      controls: true,
      autoplay: true,
    });
  }
  if (file.file_type === "pdf") {
    return Timeless.Webview({
      class: "w-full h-full bg-color-document-canvas border-0 rounded-content",
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
      class: "group-preview-overlay fixed inset-0 z-1000 flex flex-col text-preview-text bg-color-media-canvas",
      onClick(event) {
        if (event.target === event.currentTarget) {
          vm$.methods.closePreview();
        }
      },
    },
    [
      View({ class: "flex flex-[0_0_auto] items-center justify-between gap-4 p-[var(--dm-space-3)_var(--dm-space-5)] bg-color-media-surface border-bottom-[1px_solid_var(--dm-preview-border)]" }, [
        View(
          {
            class: "min-w-0 overflow-hidden text-base font-500 text-ellipsis whitespace-nowrap",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View(
          {
            as: "button",
            class: "p-[var(--dm-space-1-5)_var(--dm-space-4)] text-color-media-text font-inherit text-md bg-none border-[1px_solid_var(--dm-color-media-border)] rounded-control cursor-pointer hover:bg-preview-border dm-button dm-focus-ring",
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
        : View({ class: "[&_>_.group-preview-overlay-text-reader]:align-self-stretch [&_>_.group-preview-overlay-text-reader]:w-full [&_>_.group-preview-overlay-text-reader]:h-full [&_>_.group-preview-overlay-text-reader]:min-w-0 [&_>_.group-preview-overlay-text-reader]:min-h-0 [&_>_.group-preview-overlay-html-reader]:align-self-stretch [&_>_.group-preview-overlay-html-reader]:w-full [&_>_.group-preview-overlay-html-reader]:h-full [&_>_.group-preview-overlay-html-reader]:min-w-0 [&_>_.group-preview-overlay-html-reader]:min-h-0 flex min-h-0 flex-1 items-center justify-center p-5 overflow-auto [&.is-zip]:items-start [&.is-zip]:justify-start [&_>_.group-preview-video-player]:h-full max-520:p-3" }, [
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
      class: ["[--dm-preview-bg:var(--dm-color-bg-canvas)] [--dm-preview-panel:var(--dm-color-bg-surface)] [--dm-preview-panel-soft:var(--dm-color-bg-subtle)] [--dm-preview-border:var(--dm-color-border)] [--dm-preview-text:var(--dm-color-text-primary)] [--dm-preview-muted:var(--dm-color-text-secondary)] [--dm-preview-subtle:var(--dm-color-text-disabled)] [--dm-preview-accent:var(--dm-color-primary)] relative flex w-full min-w-0 h-full min-h-0 flex-col overflow-hidden text-preview-text font-sans bg-preview-bg bg-image-[radial-gradient(_circle,_var(--dm-color-border-translucent)_var(--dm-border-width),_transparent_var(--dm-border-width)_)] background-size-[var(--dm-space-6)_var(--dm-space-6)] [&.is-embedded]:h-auto [&.is-embedded]:flex-1 [&.is-embedded_.group-preview-overlay]:absolute min-w-0 font-sans bg-color-bg-canvas", props.embedded ? "is-embedded" : ""]
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
                      "group-preview-retry p-[var(--dm-space-1-5)_var(--dm-space-4)] text-color-media-text font-inherit text-md bg-none border-[1px_solid_var(--dm-color-media-border)] rounded-control cursor-pointer hover:bg-preview-border dm-button dm-button--primary dm-focus-ring",
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

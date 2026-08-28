import { PreviewViewModel } from "./preview.model.js";

function PreviewStateView(props) {
  return View(
    {
      class: "preview-state dm-empty-state",
      role: props.role || "status",
    },
    [
      props.loading ? View({ class: "preview-spinner" }) : null,
      !props.loading
        ? View({ class: "preview-state-icon" }, [
            Timeless.Icon({
              name: props.role === "alert" ? "circle-alert" : "file-search",
              size: 22,
            }),
          ])
        : null,
      props.title
        ? View({ as: "h3", class: "preview-state-title" }, [props.title])
        : null,
      props.message
        ? View({ as: "p", class: "preview-state-message" }, [
            props.message,
          ])
        : null,
      props.action || null,
    ].filter(Boolean),
  );
}

function PreviewHeaderView(props) {
  const task = props.task;
  const account = task.account;
  return View({ class: "preview-header page-header container" }, [
    account
      ? View({ class: "preview-account" }, [
          account.avatar_url
            ? Timeless.Img({
                class: "preview-account-avatar",
                src: account.avatar_url,
                alt: "",
                attributes: { referrerpolicy: "no-referrer" },
                onError(event) {
                  event.target.style.display = "none";
                },
              })
            : null,
          View({ class: "preview-account-name" }, [
            account.nickname || account.external_id || "",
          ]),
        ])
      : null,
    View(
      { class: "preview-title dm-font-display dm-font-bold" },
      [task.title],
    ),
    View({ class: "preview-subtitle" }, [
      task.platform_id
        ? View({ class: "preview-platform" }, [
            task.platform_favicon
              ? Timeless.Img({
                  class: "preview-platform-icon",
                  src: task.platform_favicon,
                  alt: "",
                  attributes: { referrerpolicy: "no-referrer" },
                  onError(event) {
                    event.target.style.display = "none";
                  },
                })
              : null,
            task.platform_name,
          ])
        : null,
      task.content_type
        ? View(
            { class: "preview-badge dm-badge dm-badge--info" },
            [task.content_type],
          )
        : null,
      Number.isFinite(props.fileCount)
        ? View(
            { class: "preview-badge dm-badge dm-badge--info" },
            [`文件 (${props.fileCount})`],
          )
        : null,
    ].filter(Boolean)),
  ].filter(Boolean));
}

function PreviewSingleFileView(props) {
  const vm$ = props.store;
  const file = props.file;
  const url = vm$.methods.fileURL(file);
  if (file.file_type === "video") {
    return View({ class: "preview-video-container" }, [
      PreviewVideoPlayerView({
        store: vm$,
        file,
        videoClass: "preview-video",
        autoplay: true,
      }),
      View({ class: "preview-filename" }, [file.name]),
    ]);
  }
  return View({ class: "preview-image-container" }, [
    Timeless.Img({
      class: "preview-image",
      src: url,
      alt: file.name,
    }),
    View({ class: "preview-filename" }, [file.name]),
  ]);
}

function PreviewVideoPlayerView(props) {
  const vm$ = props.store;
  const file = props.file;
  const is_live_playback = vm$.methods.isLivePlayback(file);
  return View(
    {
      class: [
        "preview-video-player",
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
          vm$.methods.mountVideo(event, file, {
            autoplay: Boolean(props.autoplay),
          });
        },
        onUnmounted() {
          vm$.methods.unmountVideo(file);
        },
      }),
      is_live_playback
        ? View(
            {
              class: computed(
                vm$.state.live_playback_status,
                (status) =>
                  `preview-live-status is-${status || "waiting"}`,
              ),
              attributes: {
                n: "live-playback-status",
                role: "status",
              },
            },
            [vm$.state.live_playback_message],
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
        class: "preview-file-icon",
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
  return View({ class: "preview-file-thumbnail-wrap" }, [
    View(
      {
        class: "preview-file-icon",
        attributes: { n: "file-thumbnail-fallback-icon" },
      },
      [
        Timeless.Icon({
          name: vm$.methods.fileTypeIcon(file.file_type),
          size: 42,
        }),
      ],
    ),
    Timeless.Img({
      class: "preview-file-thumbnail",
      src: vm$.methods.fileURL(file),
      alt: file.name,
      attributes: { loading: "lazy" },
      onError(event) {
        event.target.style.display = "none";
      },
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
        "preview-file-card dm-panel--soft dm-focus-ring",
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
      View({ class: "preview-file-thumb" }, [
        PreviewFileThumbnail({ store: vm$, file }),
        file.status
          ? View({ class: "preview-file-status" }, [file.status])
          : null,
      ].filter(Boolean)),
      View({ class: "preview-file-info" }, [
        View(
          {
            class: "preview-file-name",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View({ class: "preview-file-meta" }, [
          View({}, [vm$.methods.formatBytes(file.size)]),
          View({}, [playable ? "" : "missing"]),
        ]),
        file.status === "downloading" && file.progress > 0
          ? View({ class: "preview-progress" }, [
              View({
                class: "preview-progress-value",
                style: { width: `${file.progress}%` },
              }),
            ])
          : null,
      ].filter(Boolean)),
    ],
  );
}

function PreviewFileGridView(props) {
  const vm$ = props.store;
  const files = props.files;
  return View({ class: "preview-main container" }, [
    View({ as: "h2", class: "preview-files-title" }, [
      `文件 (${files.length})`,
    ]),
    files.length > 0
      ? View({ class: "preview-file-grid" }, [
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
  return View({ class: "preview-gallery-placeholder" }, [
    View(
      {
        class: "preview-gallery-placeholder-icon",
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
        class: "preview-gallery-placeholder-name",
        attributes: { title: file.name },
      },
      [file.name],
    ),
    View({ class: "preview-gallery-placeholder-message" }, [
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
  if (file.file_type === "image") {
    return Timeless.Img({
      class: "preview-gallery-image",
      src: url,
      alt: file.name,
      attributes: { loading: "eager" },
    });
  }
  if (file.file_type === "video") {
    return PreviewVideoPlayerView({
      store: vm$,
      file,
      videoClass: "preview-gallery-video",
      autoplay: vm$.methods.isLivePlayback(file),
    });
  }
  if (file.file_type === "audio") {
    return View({ class: "preview-gallery-audio-stage" }, [
      View(
        {
          class: "preview-gallery-audio-icon",
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
        class: "preview-gallery-audio",
        src: url,
        controls: true,
        preload: "metadata",
      }),
    ]);
  }
  if (["html", "pdf"].includes(file.file_type)) {
    return Timeless.Webview({
      class: "preview-gallery-document",
      href: url,
      attributes: {
        title: file.name,
        loading: "eager",
        ...(file.file_type === "html"
          ? { sandbox: "allow-same-origin" }
          : {}),
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
      class: `preview-gallery-stage is-${file.file_type}`,
    },
    [
      View({ class: "preview-gallery-viewport" }, [
        PreviewGalleryMediaView({ store: vm$, file }),
      ]),
      View({ class: "preview-gallery-caption" }, [
        View(
          {
            class: "preview-gallery-caption-icon",
            attributes: { n: "gallery-caption-file-type-icon" },
          },
          [
            Timeless.Icon({
              name: vm$.methods.fileTypeIcon(file.file_type),
              size: 18,
            }),
          ],
        ),
        View({ class: "preview-gallery-caption-main" }, [
          View(
            {
              class: "preview-gallery-name",
              attributes: { title: file.name },
            },
            [file.name],
          ),
          View({ class: "preview-gallery-meta" }, [meta]),
        ]),
        file.exists
          ? View(
              {
                as: "button",
                class:
                  "preview-gallery-open dm-button dm-focus-ring",
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
                View(
                  { attributes: { n: "gallery-show-file-label" } },
                  ["文件"],
                ),
              ],
            )
          : null,
      ].filter(Boolean)),
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
          "preview-gallery-file dm-focus-ring",
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
        "aria-pressed": computed(
          vm$.state.gallery_file,
          (selected_file) => (selected_file === file ? "true" : "false"),
        ),
      },
      onClick() {
        vm$.methods.selectGalleryFile(file);
      },
    },
    [
      View({ class: "preview-gallery-file-thumb" }, [
        PreviewFileThumbnail({ store: vm$, file }),
      ]),
      View({ class: "preview-gallery-file-main" }, [
        View(
          {
            class: "preview-gallery-file-name",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View({ class: "preview-gallery-file-meta" }, [
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
    return View({ class: "preview-gallery container" }, [
      PreviewStateView({ message: "暂无文件" }),
    ]);
  }
  return View(
    {
      class: [
        "preview-gallery container",
        files.length === 1 ? "is-single" : "",
      ]
        .filter(Boolean)
        .join(" "),
      role: "region",
      attributes: { "aria-label": "文件画廊" },
    },
    [
      View({ class: "preview-gallery-stage-list" }, [
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
        ? View({ class: "preview-gallery-file-list-wrap" }, [
            View({ class: "preview-gallery-file-list" }, [
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
    single_file && ["video", "image"].includes(single_file.file_type)
      ? PreviewSingleFileView({ store: vm$, file: single_file })
      : PreviewFileGridView({ store: vm$, files: task.files }),
  ];
}

function PreviewDownloadLinkView(props) {
  const vm$ = props.store;
  return View(
    {
      as: "a",
      class: "preview-download-link dm-button dm-focus-ring",
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
  return View({ class: "preview-overlay-body is-zip" }, [
    Show({
      when: vm$.state.zip_loading,
      ok() {
          return PreviewStateView({ loading: true, message: "正在加载压缩包预览…" });
      },
      else() {
        return Show({
          when: computed(vm$.state.zip_error, (error) => Boolean(error)),
          ok() {
            return PreviewStateView({
              role: "alert",
              message: vm$.state.zip_error,
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
                return View({ class: "preview-zip-gallery" }, [
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
                          class: "preview-zip-item",
                          attributes: { title: image.name },
                        },
                        [
                          Timeless.Img({
                            class: "preview-zip-image",
                            src: image.url,
                            alt: image.name,
                            attributes: { loading: "lazy" },
                          }),
                          View(
                            {
                              as: "figcaption",
                              class: "preview-zip-caption",
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
  if (file.file_type === "image") {
    return Timeless.Img({
      class: "preview-overlay-image",
      src: url,
      alt: file.name,
    });
  }
  if (file.file_type === "video") {
    return PreviewVideoPlayerView({
      store: vm$,
      file,
      videoClass: "preview-overlay-video",
      autoplay: true,
    });
  }
  if (file.file_type === "audio") {
    return Timeless.Audio({
      class: "preview-overlay-audio",
      src: url,
      controls: true,
      autoplay: true,
    });
  }
  if (["html", "pdf"].includes(file.file_type)) {
    return Timeless.Webview({
      class: "preview-overlay-frame",
      href: url,
      attributes: {
        title: file.name,
        ...(file.file_type === "html"
          ? { sandbox: "allow-same-origin" }
          : {}),
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
      class: "preview-overlay",
      onClick(event) {
        if (event.target === event.currentTarget) {
          vm$.methods.closePreview();
        }
      },
    },
    [
      View({ class: "preview-overlay-header" }, [
        View(
          {
            class: "preview-overlay-name",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View(
          {
            as: "button",
            class: "preview-close dm-button dm-focus-ring",
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
        : View({ class: "preview-overlay-body" }, [
            PreviewOverlayMediaView({ store: vm$, file }),
          ]),
    ],
  );
}

function PreviewPageView(props) {
  const vm$ = PreviewViewModel(props);
  let unsubscribe_task_id = null;

  function handle_keydown(event) {
    if (event.key === "Escape") {
      vm$.methods.closePreview();
    }
  }

  return View(
    {
      class: ["preview-page page", props.embedded ? "is-embedded" : ""]
        .filter(Boolean)
        .join(" "),
      onMounted() {
        window.document.addEventListener("keydown", handle_keydown);
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
        window.document.removeEventListener("keydown", handle_keydown);
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
                message: vm$.state.error,
                action: View(
                  {
                    as: "button",
                    class:
                      "preview-retry dm-button dm-button--primary dm-focus-ring",
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

export default PreviewPageView;

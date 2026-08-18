import { PreviewViewModel } from "./preview.model.js";

function PreviewStateView(props) {
  return View(
    {
      class: "wx-preview-state dm-empty-state",
      role: props.role || "status",
    },
    [
      props.loading ? View({ class: "wx-preview-spinner" }) : null,
      !props.loading
        ? View({ class: "wx-preview-state-icon" }, [
            Timeless.Icon({
              name: props.role === "alert" ? "circle-alert" : "file-search",
              size: 22,
            }),
          ])
        : null,
      props.title
        ? View({ as: "h3", class: "wx-preview-state-title" }, [props.title])
        : null,
      props.message
        ? View({ as: "p", class: "wx-preview-state-message" }, [
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
  return View({ class: "wx-preview-header dm-page-header dm-container" }, [
    account
      ? View({ class: "wx-preview-account" }, [
          account.avatar_url
            ? Timeless.Img({
                class: "wx-preview-account-avatar",
                src: account.avatar_url,
                alt: "",
                attributes: { referrerpolicy: "no-referrer" },
                onError(event) {
                  event.target.style.display = "none";
                },
              })
            : null,
          View({ class: "wx-preview-account-name" }, [
            account.nickname || account.external_id || "",
          ]),
        ])
      : null,
    View(
      { class: "wx-preview-title dm-font-display dm-font-bold" },
      [task.title],
    ),
    View({ class: "wx-preview-subtitle" }, [
      task.platform_id
        ? View({ class: "wx-preview-platform" }, [
            task.platform_favicon
              ? Timeless.Img({
                  class: "wx-preview-platform-icon",
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
            { class: "wx-preview-badge dm-badge dm-badge--info" },
            [task.content_type],
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
    return View({ class: "wx-preview-video-container" }, [
      Timeless.Video({
        class: "wx-preview-video",
        src: url,
        controls: true,
        autoplay: true,
        playsInline: true,
        preload: "metadata",
      }),
      View({ class: "wx-preview-filename" }, [file.name]),
    ]);
  }
  return View({ class: "wx-preview-image-container" }, [
    Timeless.Img({
      class: "wx-preview-image",
      src: url,
      alt: file.name,
    }),
    View({ class: "wx-preview-filename" }, [file.name]),
  ]);
}

function PreviewFileThumbnail(props) {
  const vm$ = props.store;
  const file = props.file;
  if (file.file_type !== "image" || !file.exists) {
    return View({ class: "wx-preview-file-icon" }, [
      vm$.methods.fileTypeIcon(file.file_type),
    ]);
  }
  return View({ class: "wx-preview-file-thumbnail-wrap" }, [
    View({ class: "wx-preview-file-icon" }, [
      vm$.methods.fileTypeIcon(file.file_type),
    ]),
    Timeless.Img({
      class: "wx-preview-file-thumbnail",
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
  return View(
    {
      as: "button",
      class: [
        "wx-preview-file-card dm-panel--soft dm-focus-ring",
        file.exists ? "" : "is-missing",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: file.name,
        disabled: !file.exists,
      },
      onClick() {
        vm$.methods.openPreview(file);
      },
    },
    [
      View({ class: "wx-preview-file-thumb" }, [
        PreviewFileThumbnail({ store: vm$, file }),
        file.status
          ? View({ class: "wx-preview-file-status" }, [file.status])
          : null,
      ].filter(Boolean)),
      View({ class: "wx-preview-file-info" }, [
        View(
          {
            class: "wx-preview-file-name",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View({ class: "wx-preview-file-meta" }, [
          View({}, [vm$.methods.formatBytes(file.size)]),
          View({}, [file.exists ? "" : "missing"]),
        ]),
        file.status === "downloading" && file.progress > 0
          ? View({ class: "wx-preview-progress" }, [
              View({
                class: "wx-preview-progress-value",
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
  return View({ class: "wx-preview-main dm-container" }, [
    View({ as: "h2", class: "wx-preview-files-title" }, [
      `文件 (${files.length})`,
    ]),
    files.length > 0
      ? View({ class: "wx-preview-file-grid" }, [
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

function PreviewTaskBodyView(props) {
  const vm$ = props.store;
  const task = props.task;
  const existing_files = task.files.filter((file) => file.exists);
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
      class: "wx-preview-download-link dm-button dm-focus-ring",
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
  return View({ class: "wx-preview-overlay-body is-zip" }, [
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
                return View({ class: "wx-preview-zip-gallery" }, [
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
                          class: "wx-preview-zip-item",
                          attributes: { title: image.name },
                        },
                        [
                          Timeless.Img({
                            class: "wx-preview-zip-image",
                            src: image.url,
                            alt: image.name,
                            attributes: { loading: "lazy" },
                          }),
                          View(
                            {
                              as: "figcaption",
                              class: "wx-preview-zip-caption",
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
      class: "wx-preview-overlay-image",
      src: url,
      alt: file.name,
    });
  }
  if (file.file_type === "video") {
    return Timeless.Video({
      class: "wx-preview-overlay-video",
      src: url,
      controls: true,
      autoplay: true,
      playsInline: true,
    });
  }
  if (file.file_type === "audio") {
    return Timeless.Audio({
      class: "wx-preview-overlay-audio",
      src: url,
      controls: true,
      autoplay: true,
    });
  }
  if (file.file_type === "html") {
    return Timeless.Webview({
      class: "wx-preview-overlay-frame",
      href: url,
      attributes: {
        sandbox: "allow-same-origin",
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
      class: "wx-preview-overlay",
      onClick(event) {
        if (event.target === event.currentTarget) {
          vm$.methods.closePreview();
        }
      },
    },
    [
      View({ class: "wx-preview-overlay-header" }, [
        View(
          {
            class: "wx-preview-overlay-name",
            attributes: { title: file.name },
          },
          [file.name],
        ),
        View(
          {
            as: "button",
            class: "wx-preview-close dm-button dm-focus-ring",
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
        : View({ class: "wx-preview-overlay-body" }, [
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
      class: "wx-preview-page dm-page",
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
        vm$.methods.closePreview();
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
                      "wx-preview-retry dm-button dm-button--primary dm-focus-ring",
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

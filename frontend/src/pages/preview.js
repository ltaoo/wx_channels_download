(function (global) {
  "use strict";

  const TYPE_ICONS = {
    image: "\u{1F5BC}",
    video: "\u{1F3AC}",
    audio: "\u{1F3B5}",
    html: "\u{1F310}",
    zip: "\u{1F4E6}",
    pdf: "\u{1F4C4}",
    other: "\u{1F4C1}",
  };

  const PLATFORM_FAVICONS = {
    wxchannels:
      "https://res.wx.qq.com/t/wx_fed/finder/helper/finder-helper-web/res/favicon-v2.ico",
    wxmp: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
    officialaccount: "https://res.wx.qq.com/a/wx_fed/assets/res/NTI4MWU5.ico",
  };

  const PLATFORM_NAMES = {
    wxchannels: "视频号",
    wxmp: "公众号",
    officialaccount: "公众号",
    douyin: "抖音",
    bilibili: "Bilibili",
    xiaohongshu: "小红书",
    xhs: "小红书",
    youtube: "YouTube",
    zhihu: "知乎",
    douban: "豆瓣",
    qidian: "起点中文网",
    fanqienovel: "番茄小说",
    "69shuba": "69书吧",
  };

  const preview_request = Timeless.kit.request_factory({
    headers: { "Content-Type": "application/json" },
    process(response) {
      if (response.error) {
        return Timeless.Result.Err(response.error);
      }
      const payload = response.data || {};
      if (payload.code !== 0) {
        return Timeless.Result.Err(
          payload.msg || "获取下载任务详情失败",
          payload.code,
          payload.data,
        );
      }
      return Timeless.Result.Ok(payload.data || {});
    },
  });

  function first_non_empty(...values) {
    for (const value of values) {
      if (value !== undefined && value !== null && value !== "") {
        return value;
      }
    }
    return "";
  }

  function number_or_default(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function error_message(error, fallback) {
    if (error && error.message) {
      return error.message;
    }
    return error ? String(error) : fallback;
  }

  function platform_favicon(platform_id) {
    return PLATFORM_FAVICONS[platform_id] || "";
  }

  function platform_name(platform_id) {
    return PLATFORM_NAMES[platform_id] || platform_id || "";
  }

  function format_bytes(bytes) {
    const value = number_or_default(bytes, 0);
    if (value <= 0) {
      return "0 B";
    }
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(
      sizes.length - 1,
      Math.floor(Math.log(value) / Math.log(1024)),
    );
    return `${parseFloat((value / Math.pow(1024, index)).toFixed(1))} ${sizes[index]}`;
  }

  function file_type_icon(file_type) {
    return TYPE_ICONS[file_type] || TYPE_ICONS.other;
  }

  function file_url(file) {
    if (!file) {
      return "";
    }
    if (file.file_url) {
      return file.file_url;
    }
    return `/api/file?path=${encodeURIComponent(file.local_path || "")}`;
  }

  function normalize_account(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    return {
      ...source,
      nickname: first_non_empty(source.nickname, source.Nickname),
      avatar_url: first_non_empty(
        source.avatar_url,
        source.avatarUrl,
        source.AvatarURL,
      ),
      external_id: first_non_empty(
        source.external_id,
        source.externalId,
        source.ExternalID,
      ),
    };
  }

  function normalize_file(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    return {
      ...source,
      name: first_non_empty(source.name, source.Name, "未命名文件"),
      local_path: first_non_empty(
        source.local_path,
        source.localPath,
        source.LocalPath,
      ),
      file_url: first_non_empty(
        source.file_url,
        source.fileUrl,
        source.FileURL,
      ),
      file_type: first_non_empty(
        source.file_type,
        source.fileType,
        source.FileType,
        "other",
      ),
      status: first_non_empty(source.status, source.Status),
      size: Math.max(0, number_or_default(source.size || source.Size, 0)),
      progress: Math.min(
        100,
        Math.max(
          0,
          number_or_default(source.progress || source.Progress, 0),
        ),
      ),
      exists: Boolean(
        typeof source.exists !== "undefined"
          ? source.exists
          : source.Exists,
      ),
    };
  }

  function normalize_task(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    const content =
      source.content && typeof source.content === "object"
        ? source.content
        : {};
    const accounts = Array.isArray(content.accounts)
      ? content.accounts.map(normalize_account)
      : [];
    const files = Array.isArray(source.files)
      ? source.files.map(normalize_file)
      : [];
    const platform_id = first_non_empty(
      content.platform_id,
      content.platformId,
      source.platform_id,
      source.platformId,
    );
    return {
      ...source,
      content: {
        ...content,
        accounts,
      },
      files,
      title: first_non_empty(
        content.title,
        content.Title,
        source.name,
        source.Name,
        "未命名内容",
      ),
      name: first_non_empty(source.name, source.Name, "Preview"),
      platform_id,
      platform_name: platform_name(platform_id),
      platform_favicon: platform_favicon(platform_id),
      content_type: first_non_empty(
        source.content_type,
        source.contentType,
        content.type,
        content.Type,
      ),
      account: accounts.length > 0 ? accounts[0] : null,
    };
  }

  function normalize_zip_images(data) {
    const images = data && Array.isArray(data.images) ? data.images : [];
    return images
      .filter((image) => image && image.url)
      .map((image) => ({
        name: first_non_empty(image.name, "未命名图片"),
        url: image.url,
      }));
  }

  function create_http_client() {
    return new Timeless.kit.HttpClientCore({
      headers: { "Content-Type": "application/json" },
    });
  }

  function PreviewViewModel(props = {}) {
    const task_ = ref(null);
    const loading_ = ref(false);
    const error_ = ref("");
    const active_file_ = ref(null);
    const zip_images_ = refarr([]);
    const zip_loading_ = ref(false);
    const zip_error_ = ref("");
    const http_client = props.client || create_http_client();
    let detail_request_sequence = 0;
    let zip_request_sequence = 0;

    const detail_request = new Timeless.kit.RequestCore(
      (params) =>
        preview_request.get("/api/v1/download_task/detail", params),
      { client: http_client },
    );
    const zip_request = new Timeless.kit.RequestCore(
      (params) => preview_request.get("/api/file", params),
      { client: http_client },
    );

    async function load() {
      const task_id = new URLSearchParams(global.location.search).get("id");
      if (!task_id) {
        error_.as("Missing task id");
        task_.as(null);
        return null;
      }

      const sequence = ++detail_request_sequence;
      loading_.as(true);
      error_.as("");
      const result = await detail_request.run({ id: task_id });
      if (sequence !== detail_request_sequence) {
        return result;
      }
      loading_.as(false);
      if (result.error) {
        error_.as(error_message(result.error, "获取下载任务详情失败"));
        task_.as(null);
        return result;
      }

      const task = normalize_task(result.data);
      task_.as(task);
      global.document.title = task.name || "Preview";
      return result;
    }

    async function load_zip_preview(file) {
      const sequence = ++zip_request_sequence;
      zip_loading_.as(true);
      zip_error_.as("");
      zip_images_.as([], { reset: true });
      const result = await zip_request.run({
        preview: 1,
        path: file.local_path,
      });
      if (sequence !== zip_request_sequence || active_file_.value !== file) {
        return result;
      }
      zip_loading_.as(false);
      if (result.error) {
        zip_error_.as(error_message(result.error, "读取压缩包失败"));
        return result;
      }
      zip_images_.as(normalize_zip_images(result.data), { reset: true });
      return result;
    }

    const methods = {
      ready() {
        return load();
      },
      retry() {
        return load();
      },
      openPreview(file) {
        if (!file || !file.exists) {
          return;
        }
        active_file_.as(file);
        zip_images_.as([], { reset: true });
        zip_error_.as("");
        global.document.body.style.overflow = "hidden";
        if (file.file_type === "zip") {
          load_zip_preview(file);
        }
      },
      closePreview() {
        zip_request_sequence += 1;
        active_file_.as(null);
        zip_loading_.as(false);
        zip_error_.as("");
        zip_images_.as([], { reset: true });
        global.document.body.style.overflow = "";
      },
      fileURL: file_url,
      fileTypeIcon: file_type_icon,
      formatBytes: format_bytes,
    };

    return {
      state: {
        task: task_,
        loading: loading_,
        error: error_,
        active_file: active_file_,
        zip_images: zip_images_,
        zip_loading: zip_loading_,
        zip_error: zip_error_,
      },
      methods,
      ready: methods.ready,
    };
  }

  function PreviewStateView(props) {
    return View(
      {
        class: "wx-preview-state",
        role: props.role || "status",
      },
      [
        props.loading ? View({ class: "wx-preview-spinner" }) : null,
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
    return View({ class: "wx-preview-header" }, [
      account
        ? View({ class: "wx-preview-account" }, [
            account.avatar_url
              ? Img({
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
      View({ class: "wx-preview-title" }, [task.title]),
      View({ class: "wx-preview-subtitle" }, [
        task.platform_id
          ? View({ class: "wx-preview-platform" }, [
              task.platform_favicon
                ? Img({
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
          ? View({ class: "wx-preview-badge" }, [task.content_type])
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
      Img({
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
      Img({
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
          "wx-preview-file-card",
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
    return View({ class: "wx-preview-main" }, [
      View({ as: "h2", class: "wx-preview-files-title" }, [
        `Files (${files.length})`,
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
        : PreviewStateView({ message: "No files" }),
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
        class: "wx-preview-download-link",
        attributes: {
          href: vm$.methods.fileURL(props.file),
          target: "_blank",
          rel: "noopener noreferrer",
        },
      },
      ["Download / Open"],
    );
  }

  function PreviewZipView(props) {
    const vm$ = props.store;
    const file = props.file;
    return View({ class: "wx-preview-overlay-body is-zip" }, [
      Show({
        when: vm$.state.zip_loading,
        ok() {
          return PreviewStateView({ loading: true, message: "Loading zip preview..." });
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
                            Img({
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
                    message: "No previewable images in zip",
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
      return Img({ class: "wx-preview-overlay-image", src: url, alt: file.name });
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
      return View({
        as: "iframe",
        class: "wx-preview-overlay-frame",
        attributes: {
          src: url,
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
              class: "wx-preview-close",
              attributes: { type: "button" },
              onClick() {
                vm$.methods.closePreview();
              },
            },
            ["Close"],
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

  function PreviewPageView(props = {}) {
    const model = props.store || PreviewViewModel(props);
    const view = View(
      {
        class: "wx-preview-page",
        onMounted() {
          function handle_keydown(event) {
            if (event.key === "Escape") {
              model.methods.closePreview();
            }
          }
          global.document.addEventListener("keydown", handle_keydown);
          model.ready();
          return function cleanup() {
            global.document.removeEventListener("keydown", handle_keydown);
            model.methods.closePreview();
          };
        },
      },
      [
        Show({
          when: model.state.loading,
          ok() {
            return PreviewStateView({ loading: true, message: "Loading..." });
          },
          else() {
            return Show({
              when: computed(model.state.error, (error) => Boolean(error)),
              ok() {
                return PreviewStateView({
                  role: "alert",
                  title: "Error",
                  message: model.state.error,
                  action: View(
                    {
                      as: "button",
                      class: "wx-preview-retry",
                      attributes: { type: "button" },
                      onClick() {
                        model.methods.retry();
                      },
                    },
                    ["Retry"],
                  ),
                });
              },
              else() {
                return Show({
                  when: computed(model.state.task, (task) => Boolean(task)),
                  ok() {
                    return PreviewTaskBodyView({
                      store: model,
                      task: model.state.task.value,
                    });
                  },
                });
              },
            });
          },
        }),
        Show({
          when: computed(model.state.active_file, (file) => Boolean(file)),
          ok() {
            return PreviewOverlayView({
              store: model,
              file: model.state.active_file.value,
            });
          },
        }),
      ],
    );
    return view;
  }

  if (typeof global.register === "function") {
    global.register("preview_page", PreviewPageView);
    return;
  }

  function mount() {
    const root = global.document.getElementById("app");
    if (!root) {
      throw new Error("预览页面无法启动：缺少根节点");
    }
    const model = PreviewViewModel();
    const view = PreviewPageView({ store: model });
    Timeless.DOM.render(view, root);
  }

  if (global.document.readyState === "loading") {
    global.document.addEventListener("DOMContentLoaded", mount, { once: true });
  } else {
    mount();
  }
})(window);

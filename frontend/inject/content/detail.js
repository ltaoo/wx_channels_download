/// <reference path="../utils.js" />
/// <reference path="model.js" />
/**
 * @file Content detail page entry.
 */
function ContentPageActionButton(props) {
  return View(
    {
      type: "button",
      class: [
        "wx-content-action",
        props.compact ? "wx-content-action-compact" : "",
        props.class,
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        type: "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
    },
    [
      props.icon
        ? Timeless.Icon({ name: props.icon, size: props.iconSize || 16 })
        : null,
      props.label
        ? View({ class: "wx-content-action-label" }, [props.label])
        : null,
    ].filter(Boolean),
  );
}

function ContentCover(props) {
  const content = props.content;
  const fallback = View({ class: "wx-content-cover-fallback" }, [
    Timeless.Icon({ name: "file", size: 32 }),
    View({ class: "wx-content-cover-type" }, [
      props.store.methods.typeLabel(content.content_type),
    ]),
  ]);
  if (!content.cover_url) {
    return fallback;
  }
  return View({ class: "wx-content-cover-wrap" }, [
    fallback,
    Img({
      class: "wx-content-cover",
      src: content.cover_url,
      alt: content.title,
      attributes: {
        loading: "lazy",
        referrerpolicy: "no-referrer",
      },
      onError(event) {
        event.target.style.display = "none";
      },
    }),
  ]);
}

function ContentAccountItem(props) {
  const account = props.account || {};
  const name =
    account.nickname || account.alias || account.external_id || "未知账号";
  const profileURL = String(account.profile_url || "").trim();
  const clickable = Boolean(profileURL);

  return View(
    {
      type: clickable ? "button" : "div",
      class: [
        "wx-content-account-item",
        clickable ? "wx-content-account-item-clickable" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: clickable
        ? {
            type: "button",
            title: `打开 ${name} 的主页`,
          }
        : {},
      onClick(event) {
        if (!clickable) {
          return;
        }
        event.stopPropagation();
        window.open(profileURL, "_blank", "noopener,noreferrer");
      },
    },
    [
      Show({
        when: account.avatar_url,
        ok() {
          return Img({
            class: "wx-content-avatar",
            src: account.avatar_url,
            attributes: {
              alt: name,
              loading: "lazy",
              referrerpolicy: "no-referrer",
            },
            onError(event) {
              event.target.style.display = "none";
            },
          });
        },
        else() {
          return View(
            { class: "wx-content-avatar wx-content-avatar-fallback" },
            [String(name).slice(0, 1)],
          );
        },
      }),
      View({ class: "wx-content-account-details" }, [
        View({ class: "wx-content-account-name-line" }, [
          View(
            {
              class: "wx-content-account-name",
              attributes: { title: name },
            },
            [name],
          ),
        ]),
      ]),
      Show({
        when: clickable,
        ok() {
          return Timeless.Icon({ name: "external-link", size: 13 });
        },
      }),
    ],
  );
}

function ContentAccountList(props) {
  const accounts = props.content.accounts || [];
  if (accounts.length === 0) {
    return View({ class: "wx-content-accounts-empty wx-content-muted" }, [
      Timeless.Icon({ name: "user", size: 14 }),
      "暂无关联账号",
    ]);
  }
  return View({ class: "wx-content-accounts" }, [
    View({ class: "wx-content-account-list" }, [
      For({
        each: accounts,
        render(account) {
          return ContentAccountItem({ account });
        },
      }),
    ]),
  ]);
}

function ContentDetailHeader(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-header" }, [
    View({ class: "wx-content-header-inner" }, [
      View({ class: "wx-content-brand" }, [
        ContentPageActionButton({
          icon: "arrow-left",
          title: "返回内容列表",
          compact: true,
          onClick() {
            vm$.methods.backToList();
          },
        }),
        View({ class: "wx-content-brand-icon" }, [
          Timeless.Icon({ name: "file-text", size: 28 }),
        ]),
        View({ class: "wx-content-detail-heading" }, [
          View({ class: "wx-content-title" }, [
            computed(vm$.state.detail, (content) =>
              content && content.title ? content.title : "内容详情",
            ),
          ]),
          View({ class: "wx-content-subtitle" }, [
            computed(vm$.state.detail, (content) => {
              if (!content) {
                return vm$.state.detail_id.value || "";
              }
              return [
                vm$.methods.platformName(content),
                vm$.methods.typeLabel(content.content_type),
              ]
                .filter(Boolean)
                .join(" · ");
            }),
          ]),
        ]),
      ]),
      View({ class: "wx-content-detail-header-actions" }, [
        Show({
          when: computed(vm$.state.detail, (content) =>
            Boolean(vm$.methods.sourceURL(content)),
          ),
          ok() {
            return ContentPageActionButton({
              icon: "external-link",
              label: "源页面",
              onClick() {
                vm$.methods.openSource(vm$.state.detail.value);
              },
            });
          },
        }),
        ContentPageActionButton({
          icon: "refresh-cw",
          label: "刷新",
          onClick() {
            vm$.methods.refreshDetail();
          },
        }),
      ]),
    ]),
  ]);
}

function ContentDetailMetric(props) {
  return View({ class: "wx-content-detail-metric" }, [
    View({ class: "wx-content-detail-metric-icon" }, [
      Timeless.Icon({ name: props.icon, size: 16 }),
    ]),
    View({ class: "wx-content-detail-metric-body" }, [
      View({ class: "wx-content-detail-metric-label" }, [props.label]),
      View(
        {
          class: [
            "wx-content-detail-metric-value",
            props.tone ? `wx-content-detail-tone-${props.tone}` : "",
          ]
            .filter(Boolean)
            .join(" "),
        },
        [props.value || "未知"],
      ),
    ]),
  ]);
}

function ContentDetailSection(props) {
  return View({ class: "wx-content-detail-section" }, [
    View({ class: "wx-content-detail-section-head" }, [
      View({ class: "wx-content-detail-section-title" }, [props.title]),
      props.count !== undefined
        ? View({ class: "wx-content-detail-section-count" }, [
            String(props.count),
          ])
        : null,
    ].filter(Boolean)),
    View({ class: "wx-content-detail-section-body" }, props.children || []),
  ]);
}

function content_detail_field_label(key) {
  const labels = {
    id: "ID",
    duration: "时长",
    width: "宽度",
    height: "高度",
    fps: "帧率",
    bitrate: "码率",
    size: "大小",
    codec: "编码",
    format: "格式",
    url: "地址",
    play_times: "播放次数",
    image_count: "图片数量",
    cover_width: "封面宽度",
    cover_height: "封面高度",
    description: "描述",
    word_count: "字数",
    reading_time: "阅读时间",
    text: "正文",
    html: "HTML",
    markdown: "Markdown",
    author_name: "作者",
    chapter_count: "章节数",
    volume_count: "卷数",
    series_name: "系列",
    is_finished: "已完结",
  };
  return labels[key] || key.replace(/_/g, " ");
}

function content_detail_value(key, value, vm$) {
  if (value === null || value === undefined || value === "") {
    return "无";
  }
  if (typeof value === "boolean") {
    return value ? "是" : "否";
  }
  if (typeof value === "number") {
    if (key === "size" || key === "file_size") {
      return vm$.methods.formatBytes(value) || String(value);
    }
    if (key === "duration" || key === "duration_ms") {
      if (value <= 0) {
        return "0";
      }
      const seconds = key === "duration_ms" ? Math.round(value / 1000) : value;
      const minutes = Math.floor(seconds / 60);
      const remain = seconds % 60;
      return minutes > 0 ? `${minutes}分${remain}秒` : `${remain}秒`;
    }
    return String(value);
  }
  if (Array.isArray(value)) {
    return `${value.length} 项`;
  }
  if (typeof value === "object") {
    const text = JSON.stringify(value);
    return text.length > 240 ? `${text.slice(0, 240)}...` : text;
  }
  const text = String(value).trim();
  return text.length > 240 ? `${text.slice(0, 240)}...` : text;
}

function ContentDetailExtensionImages(props) {
  const images = (props.detail && props.detail.images) || [];
  if (!images.length) {
    return null;
  }
  return View({ class: "wx-content-detail-image-grid" }, [
    For({
      each: images,
      render(image) {
        return View({ class: "wx-content-detail-image-item" }, [
          Img({
            class: "wx-content-detail-image",
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
          View({ class: "wx-content-detail-image-meta" }, [
            [image.width, image.height].filter(Boolean).join(" x ") ||
              image.ext ||
              "图片",
          ]),
        ]);
      },
    }),
  ]);
}

function ContentDetailExtension(props) {
  const vm$ = props.store;
  const content = props.content;
  const detail = content.detail;
  const detailType = content.detail_type || "";
  if (!detail) {
    return View({ class: "wx-content-detail-empty" }, ["暂无类型扩展详情"]);
  }

  const fields = Object.keys(detail).filter(
    (key) => key !== "images" && key !== "deleted_at",
  );
  return View({ class: "wx-content-detail-extension" }, [
    detailType
      ? View({ class: "wx-content-detail-extension-type" }, [detailType])
      : null,
    View({ class: "wx-content-detail-field-grid" }, [
      For({
        each: fields,
        render(key) {
          return View({ class: "wx-content-detail-field" }, [
            View({ class: "wx-content-detail-field-label" }, [
              content_detail_field_label(key),
            ]),
            View(
              {
                class: "wx-content-detail-field-value",
                attributes: {
                  title: content_detail_value(key, detail[key], vm$),
                },
              },
              [content_detail_value(key, detail[key], vm$)],
            ),
          ]);
        },
      }),
    ]),
    ContentDetailExtensionImages({ detail }),
  ].filter(Boolean));
}

function ContentDetailResourceItem(props) {
  const vm$ = props.store;
  const resource = props.resource || {};
  const status = vm$.methods.resourceStatus(resource);
  const name = resource.name || resource.local_path || "未命名文件";
  const size = vm$.methods.formatBytes(resource.size);
  const secondary = [resource.kind, resource.type, size]
    .filter(Boolean)
    .join(" · ");
  const fileURL = vm$.methods.resourceFileURL(resource);

  return View({ class: "wx-content-detail-resource" }, [
    View({ class: "wx-content-detail-resource-icon" }, [
      Timeless.Icon({ name: vm$.methods.fileTypeIcon(resource), size: 18 }),
    ]),
    View({ class: "wx-content-detail-resource-main" }, [
      View(
        {
          class: "wx-content-detail-resource-name",
          attributes: { title: name },
        },
        [name],
      ),
      secondary
        ? View({ class: "wx-content-detail-resource-meta" }, [secondary])
        : null,
      resource.local_path
        ? View(
            {
              class: "wx-content-detail-resource-path",
              attributes: { title: resource.local_path },
            },
            [resource.local_path],
          )
        : null,
    ].filter(Boolean)),
    View(
      {
        class: [
          "wx-content-detail-status",
          `wx-content-detail-status-${status.tone}`,
        ].join(" "),
      },
      [status.label],
    ),
    resource.exists && fileURL
      ? ContentPageActionButton({
          icon: "folder-open",
          title: "打开文件",
          compact: true,
          onClick(event) {
            event.stopPropagation();
            vm$.methods.openResource(resource);
          },
        })
      : null,
  ].filter(Boolean));
}

function ContentDetailResources(props) {
  const resources = props.resources || [];
  if (!resources.length) {
    return View({ class: "wx-content-detail-empty" }, ["暂无资源文件"]);
  }
  return View({ class: "wx-content-detail-resource-list" }, [
    For({
      each: resources,
      render(resource) {
        return ContentDetailResourceItem({
          store: props.store,
          resource,
        });
      },
    }),
  ]);
}

function ContentDetailTaskItem(props) {
  const vm$ = props.store;
  const task = props.task || {};
  const status = vm$.methods.taskStatus(task.status);
  const name = task.name || task.source_url || `任务 ${task.id || ""}`;
  return View({ class: "wx-content-detail-task" }, [
    View({ class: "wx-content-detail-task-id" }, [
      task.id ? `#${task.id}` : "-",
    ]),
    View({ class: "wx-content-detail-task-main" }, [
      View(
        {
          class: "wx-content-detail-task-name",
          attributes: { title: name },
        },
        [name],
      ),
      View({ class: "wx-content-detail-task-meta" }, [
        vm$.methods.formatTime(task.updated_at || task.created_at),
      ]),
    ]),
    View(
      {
        class: [
          "wx-content-detail-status",
          `wx-content-detail-status-${status.tone}`,
        ].join(" "),
      },
      [status.label],
    ),
  ]);
}

function ContentDetailTasks(props) {
  const tasks = props.tasks || [];
  if (!tasks.length) {
    return View({ class: "wx-content-detail-empty" }, ["暂无下载任务"]);
  }
  return View({ class: "wx-content-detail-task-list" }, [
    For({
      each: tasks,
      render(task) {
        return ContentDetailTaskItem({ store: props.store, task });
      },
    }),
  ]);
}

function ContentDetailMain(props) {
  const vm$ = props.store;
  const content = props.content;
  const status = vm$.methods.downloadStatus(content.download_tasks);
  const description = String(content.description || "").trim();
  const sourceURL = vm$.methods.sourceURL(content);
  return View({ class: "wx-content-detail-layout" }, [
    View({ class: "wx-content-detail-summary" }, [
      View({ class: "wx-content-detail-cover" }, [
        ContentCover({ store: vm$, content }),
      ]),
      View({ class: "wx-content-detail-info" }, [
        View({ class: "wx-content-card-tags" }, [
          View({ class: "wx-content-platform" }, [
            vm$.methods.platformName(content),
          ]),
          View({ class: "wx-content-type-badge" }, [
            vm$.methods.typeLabel(content.content_type),
          ]),
        ]),
        View(
          {
            class: "wx-content-detail-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        View({ class: "wx-content-detail-inline-accounts" }, [
          View({ class: "wx-content-detail-inline-title" }, ["关联帐号"]),
          ContentAccountList({ content }),
        ]),
        description
          ? View({ class: "wx-content-detail-description" }, [description])
          : null,
        View({ class: "wx-content-detail-metrics" }, [
          ContentDetailMetric({
            icon: "clock3",
            label: "发布时间",
            value: vm$.methods.formatTime(content.publish_time),
          }),
          ContentDetailMetric({
            icon: "download",
            label: "下载状态",
            value: status.label,
            tone: status.tone,
          }),
          ContentDetailMetric({
            icon: "hard-drive",
            label: "文件大小",
            value: vm$.methods.formatBytes(content.file_size) || "未知",
          }),
        ]),
        sourceURL
          ? View({ class: "wx-content-detail-actions" }, [
              ContentPageActionButton({
                icon: "external-link",
                label: "源页面",
                onClick() {
                  vm$.methods.openSource(content);
                },
              }),
            ])
          : null,
      ].filter(Boolean)),
    ]),
    ContentDetailSection({
      title: "类型详情",
      children: [ContentDetailExtension({ store: vm$, content })],
    }),
    ContentDetailSection({
      title: "资源文件",
      count: (content.resources || []).length,
      children: [
        ContentDetailResources({
          store: vm$,
          resources: content.resources || [],
        }),
      ],
    }),
    ContentDetailSection({
      title: "下载任务",
      count: (content.download_tasks || []).length,
      children: [
        ContentDetailTasks({
          store: vm$,
          tasks: content.download_tasks || [],
        }),
      ],
    }),
  ]);
}

function ContentDetailSkeleton() {
  return View({ class: "wx-content-detail-layout" }, [
    View({ class: "wx-content-detail-summary wx-content-skeleton-row" }, [
      View({ class: "wx-content-detail-cover wx-content-skeleton" }),
      View({ class: "wx-content-detail-info" }, [
        View({ class: "wx-content-skeleton wx-content-skeleton-tag" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-title" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-line" }),
        View({ class: "wx-content-skeleton wx-content-skeleton-line-short" }),
      ]),
    ]),
  ]);
}

function ContentDetailBody(props) {
  const vm$ = props.store;
  return View({ class: "wx-content-main wx-content-detail-main" }, [
    Show({
      when: vm$.state.detail_loading,
      ok() {
        return ContentDetailSkeleton();
      },
      else() {
        return Show({
          when: computed(vm$.state.detail_error, (error) => Boolean(error)),
          ok() {
            return View({ class: "wx-content-state" }, [
              Timeless.Icon({ name: "circle-alert", size: 32 }),
              View({ class: "wx-content-state-title" }, ["内容加载失败"]),
              View({ class: "wx-content-state-text" }, [
                vm$.state.detail_error,
              ]),
              ContentPageActionButton({
                icon: "refresh-cw",
                label: "重试",
                onClick() {
                  vm$.methods.refreshDetail();
                },
              }),
            ]);
          },
          else() {
            return Show({
              when: computed(vm$.state.detail, (content) => Boolean(content)),
              ok() {
                return ContentDetailMain({
                  store: vm$,
                  content: vm$.state.detail.value,
                });
              },
              else() {
                return View({ class: "wx-content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "wx-content-state-title" }, ["暂无内容详情"]),
                ]);
              },
            });
          },
        });
      },
    }),
  ]);
}

function ContentDetailPageView(props) {
  const vm$ = props.store;
  return View(
    {
      class: "wx-content-page wx-content-detail-page",
      onMounted() {
        vm$.methods.readyDetail();
      },
    },
    [
      ContentDetailHeader({ store: vm$ }),
      ContentDetailBody({ store: vm$ }),
    ],
  );
}

(() => {
  function mount() {
    let root = document.getElementById("app");
    if (!root) {
      root = document.createElement("div");
      root.id = "app";
      document.body.appendChild(root);
    }
    const vm$ = ContentLibraryModel();
    Timeless.DOM.render(ContentDetailPageView({ store: vm$ }), root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount, { once: true });
    return;
  }
  mount();
})();

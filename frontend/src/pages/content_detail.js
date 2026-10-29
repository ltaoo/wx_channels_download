import { ErrorMessage } from "@/dmui.js";
import { TagSelect, ContentTagBadge } from "@/components/index.js";

import {
  ContentDetailViewModel,
  ContentDetailDescriptionModel,
  ContentDetailExtensionModel,
} from "./content_detail.model.js";
import { PreviewGalleryMediaView } from "./preview.js";

const content_detail_media_stage_classes = {
  __proto__: null,
  "list": "min-w-0",
  "video": "media-pane-video",
};

const content_detail_status_classes = {
  __proto__: null,
  "running": "text-color-warning",
  "paused": "text-color-warning",
  "finished": "text-color-success",
  "failed": "text-content-danger",
  "deleted": "text-content-danger",
  "waiting": "text-content-muted",
};

function ContentDetailAction(props) {
  const semantic_name = props.name || "content-detail-action";
  return View(
    {
      type: "button",
      class: [
        "dm-button--surface dm-button dm-focus-ring",
        props.compact ? "dm-button--sm" : "dm-button--md",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: {
        n: semantic_name,
        type: "button",
        title: props.title || "",
        ...(props.attributes || {}),
      },
      onClick: props.onClick,
    },
    [
      props.icon
        ? Timeless.Icon({
            name: props.icon,
            size: props.iconSize || 16,
            attributes: { n: `${semantic_name}-icon` },
          })
        : null,
      props.label || null,
    ].filter(Boolean),
  );
}

function ContentDetailCover(props) {
  const content = props.content;
  const cover_url = props.coverURL || props.store.methods.coverURL(content);
  if (!cover_url) return null;
  return View({ class: "relative min-w-0 w-full aspect-4/3 overflow-hidden rounded-content bg-content-panel-soft" }, [
    LazyImg({
      class: "block absolute inset-0 w-full h-full object-cover cover-error-state ",
      src: cover_url,
      alt: content.title,
      loading: "eager",
      attributes: { referrerpolicy: "no-referrer" },
    }),
  ]);
}

function ContentDetailAccount(props) {
  const account = props.account || {};
  const name =
    account.nickname || account.alias || account.external_id || "未知账号";
  const account_id = String(account.id || "").trim();
  const clickable = Boolean(
    account_id && props.history && typeof props.history.push === "function",
  );
  return View(
    {
      type: clickable ? "button" : "div",
      class: [
        "flex max-w-240px min-h-control-height-sm items-center p-0 gap-2 font-inherit text-decoration-none border-0 rounded-control bg-transparent text-content-text appearance-none dm-focus-ring",
        clickable ? "hover:text-content-primary cursor-pointer" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: clickable
        ? { type: "button", title: `在帐号管理中查看 ${name}` }
        : {},
      onClick(event) {
        if (!clickable) return;
        event.stopPropagation();
        props.history.push("root.shell.account", { id: account_id });
      },
    },
    [
      Show({
        when: account.avatar_url,
        ok() {
          return LazyImg({
            class: "grid w-26px h-26px flex-none place-items-center object-cover rounded-avatar bg-content-primary-soft text-content-primary",
            src: account.avatar_url,
            alt: name,
            attributes: { loading: "lazy", referrerpolicy: "no-referrer" },
          });
        },
        else() {
          return View({ class: "grid w-26px h-26px flex-none place-items-center object-cover rounded-avatar bg-content-primary-soft text-content-primary content-avatar-fallback" }, [
            String(name).slice(0, 1),
          ]);
        },
      }),
      View({ class: "min-w-0 overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis", attributes: { title: name } }, [
        name,
      ]),
    ].filter(Boolean),
  );
}

function ContentDetailAccounts(props) {
  const accounts = props.content.accounts || [];
  if (accounts.length === 0) {
    return null;
  }
  return View({ class: "flex flex-wrap gap-3" }, [
    For({
      each: accounts,
      render(account) {
        return ContentDetailAccount({
          account,
          history: props.history,
        });
      },
    }),
  ]);
}

function ContentDetailTags(props) {
  const vm$ = props.store;
  return View(
    {
      class: "flex min-w-0 flex-wrap items-center gap-1.5",
      attributes: { n: "content-detail-tags", "aria-label": "内容标签" },
    },
    [
      TagSelect({
        contentId: props.content.id,
        tagsRef: vm$.state.tags,
        client: props.client,
        onChange(next_tags) {
          vm$.methods.setTags(next_tags);
        },
      }),
      For({
        each: vm$.state.tags,
        render(tag) {
          return ContentTagBadge({ tag });
        },
      }),
    ],
  );
}

function ContentDetailPlatform(props) {
  const vm$ = props.store;
  const content = props.content;
  const platform_id = String(content.platform_id || "").trim();
  const favicon = (window.PLATFORM_FAVICONS || {})[platform_id] || "";
  return PlatformTag({
    name: "content-detail-platform",
    favicon,
    label: vm$.methods.platformName(content),
  });
}

function ContentDetailSection(props) {
  return View({ class: "overflow-hidden border-default border-solid border-color-content rounded-card bg-content-panel shadow-sm" }, [
    View(
      { class: "flex min-h-control-height-lg items-center justify-between py-3 px-4 gap-3 border-b-default border-b-solid border-b-color-content bg-content-panel-soft" },
      [
        View({ class: "text-base font-semibold text-content-text" }, [props.title]),
        props.count !== undefined
          ? Tag(
              {
                name: "content-detail-section-count",
                class: "bg-content-primary-soft text-content-primary",
              },
              [String(props.count)],
            )
          : null,
      ].filter(Boolean),
    ),
    View({ class: "p-4" }, props.children || []),
  ]);
}

function detail_object_value(source, ...keys) {
  if (!source || typeof source !== "object") return undefined;
  for (const key of keys) {
    if (source[key] !== undefined && source[key] !== null) {
      return source[key];
    }
  }
  return undefined;
}

function content_detail_assets(content) {
  const assets_by_key = new Map();
  const append_asset = (raw_asset) => {
    if (!raw_asset || typeof raw_asset !== "object") return;
    const nested_asset = detail_object_value(raw_asset, "asset", "Asset");
    const asset =
      nested_asset && typeof nested_asset === "object"
        ? nested_asset
        : raw_asset;
    const id = detail_object_value(asset, "id", "ID");
    const asset_key = String(
      detail_object_value(asset, "asset_key", "AssetKey") || "",
    ).trim();
    if ((id === undefined || id === null || id === "") && !asset_key) return;

    const role = String(detail_object_value(asset, "role", "Role") || "");
    const kind = String(detail_object_value(asset, "kind", "Kind") || "");
    const identity =
      id !== undefined && id !== null && id !== ""
        ? `id:${id}`
        : `${kind}:${role}:${asset_key}`;
    const current = assets_by_key.get(identity) || {};
    const current_resources = detail_object_value(
      current,
      "download_resources",
      "DownloadResources",
    );
    const asset_resources = detail_object_value(
      asset,
      "download_resources",
      "DownloadResources",
    );
    const resources_by_key = new Map();
    for (const resource of [
      ...(Array.isArray(current_resources) ? current_resources : []),
      ...(Array.isArray(asset_resources) ? asset_resources : []),
    ]) {
      if (!resource || typeof resource !== "object") continue;
      const resource_id = detail_object_value(resource, "id", "ID");
      const resource_key =
        resource_id !== undefined && resource_id !== null
          ? `id:${resource_id}`
          : String(
              detail_object_value(
                resource,
                "unique_id",
                "UniqueID",
                "name",
                "Name",
              ) || resources_by_key.size,
            );
      resources_by_key.set(resource_key, resource);
    }
    assets_by_key.set(identity, {
      ...current,
      ...asset,
      download_resources: Array.from(resources_by_key.values()),
    });
  };

  const content_record = detail_object_value(content, "content", "Content");
  const managed_assets = detail_object_value(
    content_record,
    "assets",
    "Assets",
  );
  if (Array.isArray(managed_assets)) {
    managed_assets.forEach(append_asset);
  }

  const visited = new WeakSet();
  const visit_detail = (value) => {
    if (!value || typeof value !== "object" || visited.has(value)) return;
    visited.add(value);
    if (Array.isArray(value)) {
      value.forEach(visit_detail);
      return;
    }
    for (const [key, child] of Object.entries(value)) {
      const normalized_key = key.toLowerCase();
      if (normalized_key === "asset") {
        append_asset(child);
      } else if (normalized_key === "assets" && Array.isArray(child)) {
        child.forEach(append_asset);
      }
      visit_detail(child);
    }
  };
  visit_detail(content.detail);
  visit_detail(content.embedded_contents);

  return Array.from(assets_by_key.values());
}

function content_asset_resources(asset, content_resources) {
  const resources_by_id = new Map(
    (Array.isArray(content_resources) ? content_resources : []).map(
      (resource) => [
        String(detail_object_value(resource, "id", "ID") || ""),
        resource,
      ],
    ),
  );
  const linked_resources = detail_object_value(
    asset,
    "download_resources",
    "DownloadResources",
  );
  if (!Array.isArray(linked_resources)) return [];
  return linked_resources.map((resource) => {
    const resource_id = String(detail_object_value(resource, "id", "ID") || "");
    return resources_by_id.get(resource_id) || resource;
  });
}

function content_media_type(resource, asset) {
  const normalize = (value) =>
    String(value || "")
      .trim()
      .toLowerCase();
  const asset_kind = normalize(detail_object_value(asset, "kind", "Kind"));
  const resource_type = normalize(
    detail_object_value(resource, "file_type", "FileType"),
  );
  const values = [
    detail_object_value(asset, "mime_type", "MIMEType"),
    detail_object_value(resource, "mime_type", "MIMEType"),
    detail_object_value(resource, "kind", "Kind"),
    resource_type,
  ].map(normalize);
  const name = String(
    detail_object_value(resource, "name", "Name", "local_path", "LocalPath") ||
      "",
  ).toLowerCase();

  if (
    resource_type === "html" ||
    values.includes("html") ||
    values.some((value) =>
      ["text/html", "application/xhtml+xml"].includes(value),
    ) ||
    /\.(html?|xhtml)$/.test(name)
  ) {
    return "html";
  }
  if (
    resource_type === "pdf" ||
    values.includes("pdf") ||
    values.includes("application/pdf") ||
    /\.pdf$/.test(name)
  ) {
    return "pdf";
  }

  // ContentAsset.kind is authoritative. This keeps audio-only MP4 assets from
  // being misclassified as video solely because of their file extension.
  if (["video", "audio", "image"].includes(asset_kind)) return asset_kind;
  if (asset_kind === "archive") return "archive";

  for (const value of [resource_type, ...values]) {
    if (value === "video" || value.startsWith("video/")) return "video";
    if (value === "audio" || value.startsWith("audio/")) return "audio";
    if (value === "image" || value.startsWith("image/")) return "image";
    if (value === "archive") return "archive";
  }
  if (/\.(mp4|mkv|avi|mov|webm|m4v)$/.test(name)) return "video";
  if (/\.(mp3|aac|ogg|oga|opus|wav|flac|m4a|wma)$/.test(name)) return "audio";
  if (/\.(jpe?g|png|gif|webp|bmp|avif|svg)$/.test(name)) return "image";
  if (/\.(zip|rar|7z|tar|gz|tgz|bz2|xz)$/.test(name)) return "archive";
  if (
    asset_kind === "text" ||
    asset_kind === "manifest" ||
    values.some((value) => value.startsWith("text/")) ||
    /\.(txt|md|markdown|json|jsonl|xml|csv|log|srt|vtt|ass|ssa|lrc|ya?ml|toml|ini|css|js|mjs|cjs|ts|tsx|jsx|go|py|java|c|cc|cpp|h|hpp|rs|sh|ps1)$/.test(
      name,
    )
  ) {
    return "text";
  }
  if (asset_kind === "document" || resource_type === "document") {
    return "document";
  }
  return "other";
}

function content_media_type_label(type) {
  return (
    {
      video: "视频",
      audio: "音频",
      image: "图片",
      html: "HTML",
      pdf: "PDF",
      text: "文本",
      archive: "压缩包",
      document: "文档",
      other: "文件",
    }[type] || "文件"
  );
}

function content_media_type_icon(type) {
  return (
    {
      video: "play",
      audio: "file-volume",
      image: "image",
      html: "file-code",
      pdf: "file-text",
      text: "file-text",
      archive: "archive",
      document: "file-text",
      other: "file",
    }[type] || "file"
  );
}

function content_media_asset_label(asset) {
  const role = String(detail_object_value(asset, "role", "Role") || "")
    .trim()
    .toLowerCase();
  const role_labels = {
    primary: "正文",
    video_variant: "视频",
    audio_variant: "音频",
    live_photo: "实况",
    article_body: "正文",
    novel_chapter: "章节",
    comic_page: "页面",
    message_attachment: "附件",
    generated_image: "生成图片",
    cover: "封面",
    thumbnail: "缩略图",
  };
  return String(
    detail_object_value(asset, "label", "Label") ||
      role_labels[role] ||
      detail_object_value(asset, "asset_key", "AssetKey") ||
      "已保存内容",
  );
}

function content_asset_previews(content, vm$) {
  const entries = [];
  const seen_resources = new Set();
  const append_resource = (resource, asset = {}) => {
    resource = resource && typeof resource === "object" ? resource : {};
    const type = content_media_type(resource, asset);
    const available = vm$.methods.resourceFileAvailable(resource);
    const url = available ? vm$.methods.resourceFileURL(resource) : "";
    const status_text = vm$.methods.resourceFileStatus(resource);
    const resource_id = detail_object_value(resource, "id", "ID");
    const local_path = String(
      detail_object_value(resource, "local_path", "LocalPath") || "",
    );
    const asset_id = detail_object_value(asset, "id", "ID");
    const asset_key = detail_object_value(asset, "asset_key", "AssetKey");
    const key = String(
      resource_id ||
        local_path ||
        url ||
        (asset_id ? `asset:${asset_id}` : `asset:${asset_key}`),
    );
    if (!key || key === "asset:undefined") return;
    if (seen_resources.has(key)) return;
    seen_resources.add(key);
    const name = String(
      detail_object_value(resource, "name", "Name") ||
        content_media_asset_label(asset),
    );
    entries.push({
      key,
      type,
      url,
      name,
      asset,
      resource,
      asset_label: content_media_asset_label(asset),
      available,
      status_text,
    });
  };

  const content_assets = vm$.methods.contentMediaAssets(
    content_detail_assets(content),
  );
  for (const asset of content_assets) {
    const linked_resources = content_asset_resources(asset, content.resources);
    if (linked_resources.length) {
      linked_resources.forEach((resource) => append_resource(resource, asset));
    }
  }

  // Include unlinked and legacy DownloadResource rows too. Already linked
  // files are deduplicated while retaining their richer ContentAsset metadata.
  for (const resource of content.resources || []) {
    append_resource(resource);
  }
  return vm$.methods.sortMediaEntries(entries, content);
}

function ContentDetailMediaStage(props) {
  const vm$ = props.store;
  const media = props.media;
  const file = vm$.methods.previewFile(media);
  const player = PreviewGalleryMediaView({ store: vm$.ui.preview$, file });

  const size =
    detail_object_value(media.resource, "size", "Size") ||
    detail_object_value(media.asset, "size", "Size");
  const meta = [
    content_media_type_label(media.type),
    media.asset_label,
    vm$.methods.formatBytes(size),
    media.status_text,
  ]
    .filter(Boolean)
    .join(" · ");
  return View(
    {
      class: `flex relative w-full min-w-0 h-full min-h-0 flex-col overflow-hidden font-sans border-default border-solid border-color-content rounded-content bg-image-page-grid bg-size-grid bg-color-canvas text-color-text-primary shadow-sm media-pane ${content_detail_media_stage_classes[media.type] || ""}`,
      attributes: { n: "content-detail-media-stage" },
    },
    [
      View(
        {
          class: "grid relative min-h-360px max-900:min-h-320px max-680:min-h-240px h-full min-w-0 flex-1 place-items-center items-center justify-center overflow-hidden bg-content-panel-soft media-viewport-root gallery-viewport media-viewport ",
          attributes: { n: "content-detail-media-viewport" },
        },
        [player],
      ),
      View(
        { class: "grid grid-cols-auto-content-auto max-680:grid-cols-label-content max-480:grid-cols-1 items-center py-3 px-4 gap-3 border-t-default border-t-solid border-t-color-content bg-content-panel-soft text-content-text media-toolbar " },
        [
          View({ class: "grid max-480:hidden w-control-height-sm h-control-height-sm place-items-center rounded-content bg-content-primary/18 text-content-primary" }, [
            Timeless.Icon({
              name: content_media_type_icon(media.type),
              size: 16,
            }),
          ]),
          View({ class: "min-w-0" }, [
            View(
              {
                class: "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis",
                attributes: { title: media.name },
              },
              [media.name],
            ),
            View({ class: "mt-0.5 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [meta]),
          ]),
          media.available
            ? ContentDetailAction({
                name: "content-detail-open-media-action",
                icon: "external-link",
                label: "打开原文件",
                onClick() {
                  vm$.methods.openResource(media.resource);
                },
              })
            : null,
        ].filter(Boolean),
      ),
    ],
  );
}

function ContentDetailMediaPicker(props) {
  const vm$ = props.store;
  const media = props.media;
  return View(
    {
      type: "button",
      class: computed(props.selected, (selected) =>
        selected && selected.key === media.key
          ? "grid min-w-0 grid-cols-auto-content-auto items-center p-3 gap-3 text-left border-default border-solid border-color-content rounded-card hover:border-color-content-primary-mixed bg-content-panel-soft hover:bg-content-primary-soft text-content-text cursor-pointer appearance-none media-list-item dm-focus-ring is-selected"
          : "grid min-w-0 grid-cols-auto-content-auto items-center p-3 gap-3 text-left border-default border-solid border-color-content rounded-card hover:border-color-content-primary-mixed bg-content-panel-soft hover:bg-content-primary-soft text-content-text cursor-pointer appearance-none media-list-item dm-focus-ring",
      ),
      attributes: {
        type: "button",
        "aria-pressed": computed(props.selected, (selected) =>
          selected && selected.key === media.key ? "true" : "false",
        ),
        title: `查看 ${media.name}`,
      },
      onClick() {
        props.onSelect(media);
      },
    },
    [
      View({ class: "grid w-control-height-sm h-control-height-sm place-items-center rounded-content bg-content-panel text-content-primary" }, [
        Timeless.Icon({ name: content_media_type_icon(media.type), size: 16 }),
      ]),
      View({ class: "min-w-0" }, [
        View({ class: "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis" }, [media.name]),
        View({ class: "mt-0.5 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [
          [
            content_media_type_label(media.type),
            media.asset_label,
            vm$.methods.formatBytes(
              detail_object_value(media.resource, "size", "Size") ||
                detail_object_value(media.asset, "size", "Size"),
            ),
            media.status_text,
          ]
            .filter(Boolean)
            .join(" · "),
        ]),
      ]),
      Timeless.Icon({
        name: ["video", "audio"].includes(media.type) ? "play" : "file",
        size: 15,
      }),
    ],
  );
}

function ContentDetailExtension(props) {
  const vm$ = props.store;
  const content = props.content;
  const media = content_asset_previews(content, vm$);
  if (!media.length) {
    return View({ class: "grid min-h-260px place-items-center content-center p-6 gap-3 text-center border-default border-dashed border-color-content rounded-content bg-content-panel-soft text-content-muted" }, [
      View({ class: "grid w-12 h-12 place-items-center rounded-round bg-content-primary-soft text-content-primary" }, [
        Timeless.Icon({ name: "play", size: 24 }),
      ]),
      View({ class: "text-base font-semibold text-content-text" }, ["还没有内容资产"]),
      View({ class: "max-w-42ch text-sm leading-relaxed" }, [
        "视频、音频、图片、HTML、PDF 及其他下载文件会显示在这里。",
      ]),
    ]);
  }
  const extension_vm$ = ContentDetailExtensionModel(media);
  return View(
    {
      class: "relative min-w-0",
      attributes: { n: "content-detail-content-block" },
    },
    [
      View(
        {
          class: "grid gap-4",
          attributes: { n: "content-detail-extension" },
        },
        [
          View(
            {
              class: "min-w-0",
              attributes: { n: "content-detail-media-stage-list" },
            },
            [
              For({
                each: media,
                render(item) {
                  return Show({
                    when: computed(extension_vm$.state.selected, (selected) =>
                      Boolean(selected && selected.key === item.key),
                    ),
                    ok() {
                      return ContentDetailMediaStage({
                        store: vm$,
                        content,
                        media: item,
                      });
                    },
                  });
                },
              }),
            ],
          ),
          media.length > 1
            ? View(
                {
                  class: "grid grid-cols-fit-60 max-480:grid-cols-1 gap-2",
                  attributes: { n: "content-detail-media-choices" },
                },
                [
                  For({
                    each: media,
                    render(item) {
                      return ContentDetailMediaPicker({
                        store: vm$,
                        media: item,
                        selected: extension_vm$.state.selected,
                        onSelect: extension_vm$.methods.select,
                      });
                    },
                  }),
                ],
              )
            : null,
        ].filter(Boolean),
      ),
      media.length > 1
        ? ContentDetailFileNavigationButton({
            store: extension_vm$,
            direction: "left",
            title: "上一个文件",
          })
        : null,
      media.length > 1
        ? ContentDetailFileNavigationButton({
            store: extension_vm$,
            direction: "right",
            title: "下一个文件",
          })
        : null,
    ],
  );
}

function ContentDetailResource(props) {
  const vm$ = props.store;
  const resource = props.resource || {};
  const deleted = resource.local_file_deleted === true;
  const name = resource.name || resource.local_path || "未命名文件";
  const owner_content = resource.owner_content || null;
  const owner_label = owner_content
    ? `内嵌${vm$.methods.typeLabel(owner_content.content_type)}：${owner_content.title}`
    : "";
  const meta = [
    owner_label,
    resource.kind,
    resource.type,
    vm$.methods.formatBytes(resource.size),
  ]
    .filter(Boolean)
    .join(" · ");
  return View(
    { class: "grid min-w-0 grid-cols-auto-content-auto max-680:grid-cols-auto-content-auto items-center py-3 px-0 gap-3 list-row-actions-root list-row-actions " },
    [
      View({ class: "grid w-control-height-sm h-control-height-sm place-items-center rounded-content bg-content-primary-soft text-content-primary" }, [
        Timeless.Icon({ name: vm$.methods.fileTypeIcon(resource), size: 18 }),
      ]),
      View(
        { class: "min-w-0" },
        [
          View(
            {
              class: deleted
                ? "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis text-content-text text-deletable is-deleted"
                : "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis text-content-text text-deletable ",
              attributes: { title: name },
            },
            [name],
          ),
          meta ? View({ class: "mt-1 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [meta]) : null,
          resource.local_path
            ? View(
                {
                  class: "mt-1 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted",
                  attributes: { title: resource.local_path },
                },
                [resource.local_path],
              )
            : null,
        ].filter(Boolean),
      ),
      deleted
        ? Tag(
            {
              name: "content-detail-status",
              class: "border-default border-solid border-current bg-content-panel-soft text-content-danger",
            },
            ["已删除"],
          )
        : null,
      !deleted && vm$.methods.resourceFileAvailable(resource)
        ? ContentDetailAction({
            name: "content-detail-show-resource-action",
            icon: "folder-open",
            title: "在文件夹中显示文件",
            compact: true,
            onClick(event) {
              event.stopPropagation();
              vm$.methods.showResource(resource);
            },
          })
        : null,
    ].filter(Boolean),
  );
}

function ContentDetailResources(props) {
  if (!props.resources.length) {
    return View({ class: "p-6 text-sm text-center text-content-muted" }, ["暂无资源文件"]);
  }
  return View({ class: "grid" }, [
    For({
      each: props.resources,
      render(resource) {
        return ContentDetailResource({ store: props.store, resource });
      },
    }),
  ]);
}

function ContentDetailTask(props) {
  const vm$ = props.store;
  const task = props.task || {};
  const status = vm$.methods.taskStatus(task.status);
  const name = task.name || task.source_url || `任务 ${task.id || ""}`;
  return View({ class: "grid min-w-0 max-480:grid-cols-content-auto grid-cols-32px-main-auto items-center py-3 px-0 gap-3 list-row-divided-root list-row-divided" }, [
    View({ class: "max-480:hidden font-mono text-xs text-content-muted" }, [task.id ? `#${task.id}` : "-"]),
    View({ class: "min-w-0" }, [
      View(
        {
          class: "overflow-hidden text-sm font-semibold whitespace-nowrap text-ellipsis text-content-text",
          attributes: { title: name },
        },
        [name],
      ),
      View({ class: "mt-1 overflow-hidden text-xs whitespace-nowrap text-ellipsis text-content-muted" }, [
        vm$.methods.formatTime(task.updated_at || task.created_at),
      ]),
    ]),
    Tag(
      {
        name: "content-detail-status",
        class: `border-default border-solid border-current bg-content-panel-soft ${content_detail_status_classes[status.tone] || ""}`,
      },
      [status.label],
    ),
  ]);
}

function ContentDetailTasks(props) {
  if (!props.tasks.length) {
    return View({ class: "p-6 text-sm text-center text-content-muted" }, ["暂无下载任务"]);
  }
  return View({ class: "grid" }, [
    For({
      each: props.tasks,
      render(task) {
        return ContentDetailTask({ store: props.store, task });
      },
    }),
  ]);
}

function content_relation_label(relation) {
  const type = String(relation.relation_type || relation.type || "").trim();
  const direction = String(relation.direction || "").trim();
  const labels = {
    answer_of: direction === "incoming" ? "回答" : "所属问题",
    reply_to: direction === "incoming" ? "回复" : "回复对象",
    part_of: direction === "incoming" ? "包含内容" : "所属内容",
    contains: direction === "incoming" ? "所属集合" : "包含内容",
    episode_of: direction === "incoming" ? "单集" : "所属系列",
    quote_of: direction === "incoming" ? "引用" : "引用内容",
    repost_of: direction === "incoming" ? "转发" : "原始内容",
    derived_from: direction === "incoming" ? "派生内容" : "来源内容",
  };
  return labels[type] || type || "相关内容";
}

function content_related_items(content) {
  const relations =
    content && content.relations && Array.isArray(content.relations.list)
      ? content.relations.list
      : [];
  const items_by_id = new Map();
  for (const relation of relations) {
    const related = relation && relation.content;
    const id = String((related && related.id) || "").trim();
    if (!id) continue;
    const current = items_by_id.get(id) || {
      content: related,
      labels: [],
    };
    const label = content_relation_label(relation);
    if (!current.labels.includes(label)) current.labels.push(label);
    items_by_id.set(id, current);
  }
  return Array.from(items_by_id.values());
}

function ContentDetailRelation(props) {
  const related = props.item.content || {};
  const id = String(related.id || "").trim();
  const title = related.title || related.description || id || "未命名内容";
  const subtype = related.subtype || related.type || "内容";
  const clickable = Boolean(
    id &&
    (typeof props.onOpenDetail === "function" ||
      (props.history && typeof props.history.push === "function")),
  );
  return View(
    {
      type: clickable ? "button" : "div",
      class: [
        "grid w-full min-w-0 grid-cols-auto-content-auto items-center py-3 px-0 gap-3 text-left border-0 bg-transparent text-inherit list-row-interactive-root list-row-interactive dm-focus-ring",
        clickable ? "is-clickable" : "",
      ]
        .filter(Boolean)
        .join(" "),
      attributes: clickable ? { type: "button", title: `查看 ${title}` } : {},
      onClick() {
        if (!clickable) return;
        if (typeof props.onOpenDetail === "function") {
          props.onOpenDetail(id);
          return;
        }
        props.history.push("root.shell.content_detail", { id });
      },
    },
    [
      View({ class: "grid w-control-height-sm h-control-height-sm place-items-center rounded-content bg-content-primary-soft text-content-primary" }, [
        Timeless.Icon({ name: "git-branch", size: 17 }),
      ]),
      View({ class: "min-w-0" }, [
        View(
          {
            class: "overflow-hidden text-sm font-medium whitespace-nowrap text-ellipsis text-content-text",
            attributes: { title },
          },
          [title],
        ),
        View({ class: "mt-1 text-xs text-content-muted" }, [
          [...props.item.labels, subtype].filter(Boolean).join(" · "),
        ]),
      ]),
      clickable ? Timeless.Icon({ name: "chevron-right", size: 17 }) : null,
    ].filter(Boolean),
  );
}

function ContentDetailRelations(props) {
  const items = content_related_items(props.content);
  if (!items.length) {
    return View({ class: "p-6 text-sm text-center text-content-muted" }, ["暂无关联内容"]);
  }
  return View({ class: "grid" }, [
    For({
      each: items,
      render(item) {
        return ContentDetailRelation({
          item,
          history: props.history,
          onOpenDetail: props.onOpenDetail,
        });
      },
    }),
  ]);
}

function ContentDetailDescription(props) {
  const vm$ = ContentDetailDescriptionModel();
  return View({ attributes: { n: "content-detail-description-section" } }, [
    View(
      {
        class: computed(
          vm$.state.expanded,
          (expanded) =>
            `text-base leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere text-content-muted text-collapsible ${expanded ? "" : " is-collapsed"}`,
        ),
        attributes: { n: "content-detail-description" },
        onMounted(event) {
          vm$.methods.mount(event.target.get$elm());
        },
        onUnmounted() {
          vm$.methods.destroy();
        },
      },
      [props.description],
    ),
    Show({
      when: vm$.state.overflowing,
      ok() {
        return ContentDetailAction({
          name: "content-detail-description-toggle",
          compact: true,
          label: computed(vm$.state.expanded, (expanded) =>
            expanded ? "收起" : "展开",
          ),
          attributes: {
            "aria-expanded": computed(vm$.state.expanded, String),
          },
          onClick: vm$.methods.toggle,
        });
      },
    }),
  ]);
}

function ContentDetailFileNavigationButton(props) {
  const vm$ = props.store;
  const direction = props.direction === "left" ? "previous" : "next";
  const semantic_name = `content-detail-${direction}-file-action`;
  return Button(
    {
      store: vm$.ui[`btn_${direction}$`],
      class: `absolute top-0 z-2 w-72px h-full p-0 border-0 bg-transparent cursor-pointer media-nav-action is-${props.direction}`,
      attributes: {
        n: semantic_name,
        type: "button",
        title: props.title,
        "aria-label": props.title,
      },
      onClick() {
        vm$.methods[direction]();
      },
    },
    [
      Timeless.Icon({
        name: props.direction === "left" ? "chevron-left" : "chevron-right",
        size: 18,
        attributes: { n: `${semantic_name}-icon` },
      }),
    ],
  );
}

function ContentDetailMain(props) {
  const vm$ = props.store;
  const content = props.content;
  const description = String(content.description || "").trim();
  const cover_url = vm$.methods.coverURL(content);
  return View({ class: "grid gap-5" }, [
    View(
      {
        class: [
          "grid max-680:grid-cols-1 grid-cols-sidebar-main max-900:grid-cols-sidebar-main-sm p-5 max-680:p-4 gap-6 dm-panel summary-layout-root",
          cover_url ? "" : "summary-layout-single",
        ]
          .filter(Boolean)
          .join(" "),
      },
      [
        cover_url
          ? View({ class: "min-w-0 max-680:w-full max-680:max-w-360px max-680:my-0 max-680:mx-auto overflow-hidden rounded-content" }, [
              ContentDetailCover({ store: vm$, content, coverURL: cover_url }),
            ])
          : null,
        View(
          { class: "flex min-w-0 flex-col items-start gap-4" },
          [
            View({ class: "flex items-center gap-2" }, [
              ContentDetailPlatform({ store: vm$, content }),
              Tag({ name: "content-type-badge", class: "border-default border-solid border-color-content bg-content-panel-soft text-content-muted" }, [
                vm$.methods.typeLabel(content.content_type),
              ]),
            ]),
            View(
              {
                class: "font-display text-fluid-heading font-semibold leading-tight max-480:text-2xl overflow-wrap-anywhere text-content-text",
                attributes: { n: "content-detail-title", title: content.title },
              },
              [content.title],
            ),
            View(
              {
                class: "inline-flex items-center gap-1.5 text-sm text-content-muted",
                attributes: { n: "content-detail-publish-time" },
              },
              [
                Timeless.Icon({
                  name: "clock3",
                  size: 14,
                  attributes: { n: "content-detail-publish-time-icon" },
                }),
                `发布于 ${vm$.methods.formatTime(content.publish_time)}`,
              ],
            ),
            ContentDetailAccounts({ content, history: props.history }),
            ContentDetailTags({
              store: vm$,
              content,
              client: props.client,
            }),
            description ? ContentDetailDescription({ description }) : null,
          ].filter(Boolean),
        ),
      ].filter(Boolean),
    ),
    ContentDetailSection({
      title: "内容",
      children: [ContentDetailExtension({ store: vm$, content })],
    }),
    ...(content.relations && content.relations.has_content
      ? [
          ContentDetailSection({
            title: "关联内容",
            count: content.relations.total,
            children: [
              ContentDetailRelations({
                content,
                history: props.history,
                onOpenDetail: props.onOpenDetail,
              }),
            ],
          }),
        ]
      : []),
    ContentDetailSection({
      title: "文件",
      count: content.resources.length,
      children: [
        ContentDetailResources({ store: vm$, resources: content.resources }),
      ],
    }),
    ContentDetailSection({
      title: "下载记录",
      count: content.download_tasks.length,
      children: [
        ContentDetailTasks({ store: vm$, tasks: content.download_tasks }),
      ],
    }),
  ]);
}

function ContentDetailSkeleton() {
  return View({ class: "grid gap-5" }, [
    View({ class: "grid max-680:grid-cols-1 grid-cols-sidebar-main max-900:grid-cols-sidebar-main-sm p-5 max-680:p-4 gap-6 dm-panel summary-layout-root" }, [
      View({ class: "relative min-w-0 max-680:w-full max-680:max-w-360px max-680:my-0 max-680:mx-auto overflow-hidden rounded-content bg-skeleton-bg skeleton-shimmer " }),
      View({ class: "flex min-w-0 flex-col items-start gap-4" }, [
        View({ class: "relative h-3 w-16 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
        View({ class: "relative w-4/5 max-w-90 h-4 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
        View({ class: "relative h-3 w-4/5 max-w-45 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
        View({ class: "relative h-3 w-72pct max-w-25 mt-2 overflow-hidden rounded-sm bg-skeleton-bg skeleton-shimmer" }),
      ]),
    ]),
  ]);
}

function ContentDetailBody(props) {
  const vm$ = props.store;
  return View({ class: "min-w-0 flex-auto p-4 group-content-main content-detail-main " }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return ContentDetailSkeleton();
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "grid min-h-320px content-center justify-items-center p-8 gap-3 text-center border-style-solid border-0 bg-state-danger text-content-muted shadow-sm state-icon-primary" }, [
              BrandError({
                size: 132,
                name: "content-detail-error-symbol",
              }),
              View({ class: "text-lg font-semibold text-content-text state-title" }, ["内容加载失败"]),
              ErrorMessage({ tip: vm$.methods.tip, error: vm$.state.error }),
              ContentDetailAction({
                name: "content-detail-retry-action",
                icon: "refresh-cw",
                label: "重试",
                onClick() {
                  vm$.methods.refresh();
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
                  history: props.history,
                  onOpenDetail: props.onOpenDetail,
                  client: props.client,
                });
              },
              else() {
                return View({ class: "grid min-h-320px content-center justify-items-center p-8 gap-3 text-center border-style-solid border-0 bg-state-primary text-content-muted shadow-sm state-icon-primary" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "text-lg font-semibold text-content-text state-title" }, ["暂无内容详情"]),
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
  const vm$ = ContentDetailViewModel(props);
  return View(
    {
      class: [
        "flex min-h-full min-w-0 flex-col font-sans bg-content-bg text-content-text group-content-page embedded-scroll-pane",
        props.embedded ? "is-embedded" : "",
      ]
        .filter(Boolean)
        .join(" "),
      onMounted() {
        vm$.methods.ready();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      ContentDetailBody({
        store: vm$,
        history: props.history,
        onOpenDetail: props.embedded
          ? (content_id) => vm$.methods.openDetail(content_id)
          : null,
        client: props.client,
      }),
    ],
  );
}

export default ContentDetailPageView;

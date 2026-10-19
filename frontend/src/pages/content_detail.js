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
  "video": "[&_.group-content-detail-media-viewport]:bg-color-media-canvas",
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
        "dm-button dm-button--surface dm-focus-ring",
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
  return View({ class: "min-w-0 overflow-hidden rounded-content relative w-full aspect-ratio-[4_/_3] bg-content-panel-soft" }, [
    LazyImg({
      class: "absolute inset-0 w-full h-full block object-cover [&.dm-lazy-img-error]:grid [&.dm-lazy-img-error]:place-items-center [&.dm-lazy-img-error]:rounded-inherit",
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
        "flex items-center max-w-240px min-h-control-height-sm gap-2 p-0 text-content-text font-inherit text-decoration-none appearance-none bg-transparent border-0 rounded-control dm-focus-ring",
        clickable ? "cursor-pointer hover:text-content-primary" : "",
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
            class: "w-26px h-26px flex-[0_0_auto] grid place-items-center object-cover text-content-primary bg-content-primary-soft rounded-avatar",
            src: account.avatar_url,
            alt: name,
            attributes: { loading: "lazy", referrerpolicy: "no-referrer" },
          });
        },
        else() {
          return View({ class: "w-26px h-26px flex-[0_0_auto] grid place-items-center object-cover text-content-primary bg-content-primary-soft rounded-avatar content-avatar-fallback" }, [
            String(name).slice(0, 1),
          ]);
        },
      }),
      View({ class: "min-w-0 overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap", attributes: { title: name } }, [
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
      class: "min-w-0 flex flex-wrap items-center gap-1-5",
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
  return View({ class: "overflow-hidden bg-content-panel border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-card shadow-sm" }, [
    View(
      { class: "min-h-control-height-lg flex items-center justify-between gap-3 p-[var(--dm-space-3)_var(--dm-space-4)] bg-content-panel-soft border-bottom-[var(--dm-border-width)_solid_var(--dm-content-border)]" },
      [
        View({ class: "text-content-text text-base font-semibold" }, [props.title]),
        props.count !== undefined
          ? Tag(
              {
                name: "content-detail-section-count",
                class: "text-content-primary bg-content-primary-soft",
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
      class: `[--dm-preview-bg:var(--dm-color-bg-canvas)] [--dm-preview-panel:var(--dm-color-bg-surface)] [--dm-preview-panel-soft:var(--dm-color-bg-subtle)] [--dm-preview-border:var(--dm-color-border)] [--dm-preview-text:var(--dm-color-text-primary)] [--dm-preview-muted:var(--dm-color-text-secondary)] [--dm-preview-subtle:var(--dm-color-text-disabled)] [--dm-preview-accent:var(--dm-color-primary)] relative flex w-full min-w-0 h-full min-h-0 flex-col overflow-hidden text-preview-text font-sans bg-preview-bg bg-image-[radial-gradient(_circle,_var(--dm-color-border-translucent)_var(--dm-border-width),_transparent_var(--dm-border-width)_)] background-size-[var(--dm-space-6)_var(--dm-space-6)] [&.is-embedded]:h-auto [&.is-embedded]:flex-1 [&.is-embedded_.group-preview-overlay]:absolute overflow-hidden bg-content-panel border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-content shadow-sm ${content_detail_media_stage_classes[media.type] || ""}`,
      attributes: { n: "content-detail-media-stage" },
    },
    [
      View(
        {
          class: "group-content-detail-media-viewport relative min-h-360px grid place-items-center bg-content-panel-soft max-900:min-h-320px max-680:min-h-240px group-preview-gallery-viewport h-full min-w-0 flex-1 items-center justify-center overflow-hidden [&_>_.group-preview-gallery-text-reader]:align-self-stretch [&_>_.group-preview-gallery-text-reader]:w-full [&_>_.group-preview-gallery-text-reader]:h-full [&_>_.group-preview-gallery-text-reader]:min-w-0 [&_>_.group-preview-gallery-text-reader]:min-h-0 [&_>_.group-preview-gallery-html-reader]:align-self-stretch [&_>_.group-preview-gallery-html-reader]:w-full [&_>_.group-preview-gallery-html-reader]:h-full [&_>_.group-preview-gallery-html-reader]:min-w-0 [&_>_.group-preview-gallery-html-reader]:min-h-0 [&_>_.group-preview-video-player]:h-full [.group-content-detail-media-viewport&]:flex [.group-content-detail-media-viewport&]:h-[min(72vh,_760px)] [.group-content-detail-media-viewport&]:min-h-0 [.group-content-detail-media-viewport&]:flex-none",
          attributes: { n: "content-detail-media-viewport" },
        },
        [player],
      ),
      View(
        { class: "grid grid-cols-[auto_minmax(0,_1fr)_auto] items-center gap-3 p-[var(--dm-space-3)_var(--dm-space-4)] text-content-text bg-content-panel-soft border-top-[var(--dm-border-width)_solid_var(--dm-content-border)] max-680:grid-cols-[auto_minmax(0,_1fr)] max-680:[&_>_.dm-button]:grid-column-2 max-680:[&_>_.dm-button]:justify-self-start max-480:grid-cols-[minmax(0,_1fr)] max-480:[&_>_.dm-button]:grid-column-1" },
        [
          View({ class: "w-control-height-sm h-control-height-sm grid place-items-center text-content-primary bg-[color-mix(in_srgb,_var(--dm-content-primary)_18%,_transparent)] rounded-content max-480:hidden" }, [
            Timeless.Icon({
              name: content_media_type_icon(media.type),
              size: 16,
            }),
          ]),
          View({ class: "min-w-0" }, [
            View(
              {
                class: "overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap",
                attributes: { title: media.name },
              },
              [media.name],
            ),
            View({ class: "mt-0-5 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [meta]),
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
          ? "min-w-0 grid grid-cols-[auto_minmax(0,_1fr)_auto] items-center gap-3 p-3 text-content-text text-left cursor-pointer appearance-none bg-content-panel-soft border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-card hover:bg-content-primary-soft hover:border-color-[color-mix(_in_srgb,_var(--dm-content-primary)_48%,_var(--dm-content-border)_)] [&.is-selected]:bg-content-primary-soft [&.is-selected]:border-color-[color-mix(_in_srgb,_var(--dm-content-primary)_48%,_var(--dm-content-border)_)] [&.is-selected]:shadow-[inset_3px_0_0_var(--dm-content-primary)] dm-focus-ring is-selected"
          : "min-w-0 grid grid-cols-[auto_minmax(0,_1fr)_auto] items-center gap-3 p-3 text-content-text text-left cursor-pointer appearance-none bg-content-panel-soft border-[var(--dm-border-width)_solid_var(--dm-content-border)] rounded-card hover:bg-content-primary-soft hover:border-color-[color-mix(_in_srgb,_var(--dm-content-primary)_48%,_var(--dm-content-border)_)] [&.is-selected]:bg-content-primary-soft [&.is-selected]:border-color-[color-mix(_in_srgb,_var(--dm-content-primary)_48%,_var(--dm-content-border)_)] [&.is-selected]:shadow-[inset_3px_0_0_var(--dm-content-primary)] dm-focus-ring",
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
      View({ class: "w-control-height-sm h-control-height-sm grid place-items-center text-content-primary bg-content-panel rounded-content" }, [
        Timeless.Icon({ name: content_media_type_icon(media.type), size: 16 }),
      ]),
      View({ class: "min-w-0" }, [
        View({ class: "overflow-hidden text-sm font-semibold text-ellipsis whitespace-nowrap" }, [media.name]),
        View({ class: "mt-0-5 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [
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
    return View({ class: "min-h-260px grid place-items-center content-center gap-3 p-6 text-content-muted text-center bg-content-panel-soft border-[var(--dm-border-width)_dashed_var(--dm-content-border)] rounded-content" }, [
      View({ class: "w-12 h-12 grid place-items-center text-content-primary bg-content-primary-soft rounded-round" }, [
        Timeless.Icon({ name: "play", size: 24 }),
      ]),
      View({ class: "text-content-text text-base font-semibold" }, ["还没有内容资产"]),
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
                  class: "grid grid-cols-[repeat(auto-fit,_minmax(240px,_1fr))] gap-2 max-480:grid-cols-[minmax(0,_1fr)]",
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
    { class: "group-content-detail-resource min-w-0 grid items-center gap-3 p-[var(--dm-space-3)_0] grid-cols-[auto_minmax(0,_1fr)_auto] [&_+_.group-content-detail-resource]:border-top-[var(--dm-border-width)_solid_var(--dm-content-border)] max-680:grid-cols-[auto_minmax(0,_1fr)_auto] max-680:[&_>_.dm-button]:grid-column-[2_/_-1] max-680:[&_>_.dm-button]:justify-self-start" },
    [
      View({ class: "w-control-height-sm h-control-height-sm grid place-items-center text-content-primary bg-content-primary-soft rounded-content" }, [
        Timeless.Icon({ name: vm$.methods.fileTypeIcon(resource), size: 18 }),
      ]),
      View(
        { class: "min-w-0" },
        [
          View(
            {
              class: deleted
                ? "overflow-hidden text-content-text text-sm font-semibold text-ellipsis whitespace-nowrap [&.is-deleted]:text-content-muted [&.is-deleted]:text-decoration-line-through [&.is-deleted]:text-decoration-thickness-border-width is-deleted"
                : "overflow-hidden text-content-text text-sm font-semibold text-ellipsis whitespace-nowrap [&.is-deleted]:text-content-muted [&.is-deleted]:text-decoration-line-through [&.is-deleted]:text-decoration-thickness-border-width",
              attributes: { title: name },
            },
            [name],
          ),
          meta ? View({ class: "mt-1 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [meta]) : null,
          resource.local_path
            ? View(
                {
                  class: "mt-1 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap",
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
              class: "bg-content-panel-soft border-[var(--dm-border-width)_solid_currentColor] text-content-danger",
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
    return View({ class: "p-6 text-content-muted text-sm text-center" }, ["暂无资源文件"]);
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
  return View({ class: "group-content-detail-task min-w-0 grid items-center gap-3 p-[var(--dm-space-3)_0] grid-cols-[32px_minmax(0,_1fr)_auto] [&_+_.group-content-detail-task]:border-top-[var(--dm-border-width)_solid_var(--dm-content-border)] max-480:grid-cols-[minmax(0,_1fr)_auto]" }, [
    View({ class: "text-content-muted font-mono text-xs max-480:hidden" }, [task.id ? `#${task.id}` : "-"]),
    View({ class: "min-w-0" }, [
      View(
        {
          class: "overflow-hidden text-content-text text-sm font-semibold text-ellipsis whitespace-nowrap",
          attributes: { title: name },
        },
        [name],
      ),
      View({ class: "mt-1 overflow-hidden text-content-muted text-xs text-ellipsis whitespace-nowrap" }, [
        vm$.methods.formatTime(task.updated_at || task.created_at),
      ]),
    ]),
    Tag(
      {
        name: "content-detail-status",
        class: `bg-content-panel-soft border-[var(--dm-border-width)_solid_currentColor] ${content_detail_status_classes[status.tone] || ""}`,
      },
      [status.label],
    ),
  ]);
}

function ContentDetailTasks(props) {
  if (!props.tasks.length) {
    return View({ class: "p-6 text-content-muted text-sm text-center" }, ["暂无下载任务"]);
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
        "group-content-detail-relation w-full min-w-0 grid grid-cols-[auto_minmax(0,_1fr)_auto] gap-3 items-center p-[var(--dm-space-3)_0] text-inherit text-left bg-transparent border-0 [&.is-clickable]:cursor-pointer [&_+_.group-content-detail-relation]:border-top-[var(--dm-border-width)_solid_var(--dm-content-border)] dm-focus-ring",
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
      View({ class: "w-control-height-sm h-control-height-sm grid place-items-center text-content-primary bg-content-primary-soft rounded-content" }, [
        Timeless.Icon({ name: "git-branch", size: 17 }),
      ]),
      View({ class: "min-w-0" }, [
        View(
          {
            class: "overflow-hidden text-content-text text-sm font-medium text-ellipsis whitespace-nowrap",
            attributes: { title },
          },
          [title],
        ),
        View({ class: "mt-1 text-content-muted text-xs" }, [
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
    return View({ class: "p-6 text-content-muted text-sm text-center" }, ["暂无关联内容"]);
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
            `text-content-muted text-base leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere [&.is-collapsed]:-webkit-box [&.is-collapsed]:-webkit-box-orient-vertical [&.is-collapsed]:-webkit-line-clamp-6 [&.is-collapsed]:overflow-hidden${expanded ? "" : " is-collapsed"}`,
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
      class: `absolute top-0 z-2 w-72px h-full p-0 border-0 bg-transparent cursor-pointer [&.is-left]:left-0 [&.is-left]:bg-[linear-gradient(_to_right,_rgb(0_0_0_/_22%),_transparent_)] [&.is-left:hover:not(:disabled)]:bg-[linear-gradient(_to_right,_rgb(0_0_0_/_28%),_transparent_)] [&.is-left:focus-visible:not(:disabled)]:bg-[linear-gradient(_to_right,_rgb(0_0_0_/_28%),_transparent_)] [&.is-right]:right-0 [&.is-right]:bg-[linear-gradient(_to_left,_rgb(0_0_0_/_22%),_transparent_)] [&.is-right:hover:not(:disabled)]:bg-[linear-gradient(_to_left,_rgb(0_0_0_/_28%),_transparent_)] [&.is-right:focus-visible:not(:disabled)]:bg-[linear-gradient(_to_left,_rgb(0_0_0_/_28%),_transparent_)] [&_>_svg]:box-sizing-content-box [&_>_svg]:w-18px [&_>_svg]:h-18px [&_>_svg]:p-13px [&_>_svg]:grid [&_>_svg]:place-items-center [&_>_svg]:text-color-text-primary [&_>_svg]:bg-transparent [&_>_svg]:border-[var(--dm-border-width)_solid_var(--dm-color-border)] [&_>_svg]:rounded-[50%] [&_>_svg]:shadow-sm [&_>_svg]:transition-[color_0.15s_ease,_background-color_0.15s_ease,_border-color_0.15s_ease] [&:hover:not(:disabled)_>_svg]:text-color-primary [&:hover:not(:disabled)_>_svg]:bg-color-bg-surface [&:hover:not(:disabled)_>_svg]:border-color-color-primary-border [&:focus-visible:not(:disabled)_>_svg]:text-color-primary [&:focus-visible:not(:disabled)_>_svg]:bg-color-bg-surface [&:focus-visible:not(:disabled)_>_svg]:border-color-color-primary-border is-${props.direction}`,
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
          "group-content-detail-summary grid grid-cols-[minmax(220px,_320px)_minmax(0,_1fr)] gap-6 p-5 max-900:grid-cols-[minmax(180px,_240px)_minmax(0,_1fr)] max-680:grid-cols-[minmax(0,_1fr)] max-680:p-4 dm-panel",
          cover_url ? "" : "[.group-content-detail-summary&]:grid-cols-[minmax(0,_1fr)]",
        ]
          .filter(Boolean)
          .join(" "),
      },
      [
        cover_url
          ? View({ class: "min-w-0 overflow-hidden rounded-content max-680:w-full max-680:max-w-360px max-680:m-[0_auto]" }, [
              ContentDetailCover({ store: vm$, content, coverURL: cover_url }),
            ])
          : null,
        View(
          { class: "min-w-0 flex items-start flex-col gap-4" },
          [
            View({ class: "flex items-center gap-2" }, [
              ContentDetailPlatform({ store: vm$, content }),
              Tag({ name: "content-type-badge", class: "text-content-muted bg-content-panel-soft border-[var(--dm-border-width)_solid_var(--dm-content-border)]" }, [
                vm$.methods.typeLabel(content.content_type),
              ]),
            ]),
            View(
              {
                class: "text-content-text font-display text-[clamp(var(--dm-font-size-2xl),_3vw,_var(--dm-font-size-6xl))] font-semibold leading-tight overflow-wrap-anywhere max-480:text-2xl",
                attributes: { n: "content-detail-title", title: content.title },
              },
              [content.title],
            ),
            View(
              {
                class: "inline-flex items-center gap-1-5 text-content-muted text-sm",
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
    View({ class: "group-content-detail-summary grid grid-cols-[minmax(220px,_320px)_minmax(0,_1fr)] gap-6 p-5 max-900:grid-cols-[minmax(180px,_240px)_minmax(0,_1fr)] max-680:grid-cols-[minmax(0,_1fr)] max-680:p-4 dm-panel" }, [
      View({ class: "min-w-0 overflow-hidden rounded-content max-680:w-full max-680:max-w-360px max-680:m-[0_auto] relative overflow-hidden bg-skeleton-bg [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite]" }),
      View({ class: "min-w-0 flex items-start flex-col gap-4" }, [
        View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-16" }),
        View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] w-[min(360px,_80%)] h-4" }),
        View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(180px,_80%)]" }),
        View({ class: "relative overflow-hidden bg-skeleton-bg rounded-sm [&::after]:content-[\"\"] [&::after]:absolute [&::after]:inset-0 [&::after]:transform-[translateX(-100%)] [&::after]:bg-[linear-gradient(_90deg,_transparent,_var(--dm-skeleton-highlight),_transparent_)] [&::after]:animation-[dm-skeleton-shimmer_1.2s_ease-in-out_infinite] h-3 mt-2 w-[min(100px,_72%)]" }),
      ]),
    ]),
  ]);
}

function ContentDetailBody(props) {
  const vm$ = props.store;
  return View({ class: "group-content-main flex-[1_1_auto] min-w-0 p-4 content-detail-main " }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return ContentDetailSkeleton();
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "border-style-solid shadow-sm min-h-320px grid content-center justify-items-center gap-3 p-8 text-content-muted text-center border-0 [&_>_svg]:text-color-primary bg-[radial-gradient(_circle_at_50%_0,_var(--dm-color-danger-soft),_transparent_42%_),_var(--dm-color-bg-surface)]" }, [
              BrandError({
                size: 132,
                name: "content-detail-error-symbol",
              }),
              View({ class: "group-content-state-title text-content-text text-lg font-semibold" }, ["内容加载失败"]),
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
                return View({ class: "bg-[radial-gradient(_circle_at_50%_0,_var(--dm-color-primary-soft),_transparent_42%_),_var(--dm-color-bg-surface)] border-style-solid shadow-sm min-h-320px grid content-center justify-items-center gap-3 p-8 text-content-muted text-center border-0 [&_>_svg]:text-color-primary" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "group-content-state-title text-content-text text-lg font-semibold" }, ["暂无内容详情"]),
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
        "group-content-page min-h-full flex flex-col bg-content-bg text-content-text min-h-full [&.is-embedded]:min-h-0 [&.is-embedded]:flex-1 [&.is-embedded]:overflow-auto min-w-0 min-h-full font-sans",
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

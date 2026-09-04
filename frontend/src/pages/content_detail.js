import { ContentDetailViewModel } from "./content_detail.model.js";
import { PlatformIcon } from "../components.js";

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
  return View({ class: "content-cover-wrap" }, [
    LazyImg({
      class: "content-cover",
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
        "content-account-item dm-focus-ring",
        clickable ? "content-account-item-clickable" : "",
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
            class: "content-avatar",
            src: account.avatar_url,
            alt: name,
            attributes: { loading: "lazy", referrerpolicy: "no-referrer" },
          });
        },
        else() {
          return View(
            { class: "content-avatar content-avatar-fallback" },
            [String(name).slice(0, 1)],
          );
        },
      }),
      View({ class: "content-account-name", attributes: { title: name } }, [
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
  return View({ class: "content-account-list" }, [
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

function ContentDetailPlatform(props) {
  const vm$ = props.store;
  const content = props.content;
  const platform_id = String(content.platform_id || "").trim();
  const favicon = (window.PLATFORM_FAVICONS || {})[platform_id] || "";
  return View({ class: "content-platform" }, [
    Show({
      when: favicon,
      ok() {
        return PlatformIcon({
          class: "content-platform-icon",
          favicon,
          name: "content-platform-icon",
        });
      },
      else() {
        return Timeless.Icon({ name: "globe", size: 14 });
      },
    }),
    vm$.methods.platformName(content),
  ]);
}

function ContentDetailSection(props) {
  return View({ class: "content-detail-section" }, [
    View({ class: "content-detail-section-head" }, [
      View({ class: "content-detail-section-title" }, [props.title]),
      props.count !== undefined
        ? View({ class: "content-detail-section-count" }, [
            String(props.count),
          ])
        : null,
    ].filter(Boolean)),
    View({ class: "content-detail-section-body" }, props.children || []),
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
    const resource_id = String(
      detail_object_value(resource, "id", "ID") || "",
    );
    return resources_by_id.get(resource_id) || resource;
  });
}

function content_media_type(resource, asset) {
  const normalize = (value) => String(value || "").trim().toLowerCase();
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
    const linked_resources = content_asset_resources(
      asset,
      content.resources,
    );
    if (linked_resources.length) {
      linked_resources.forEach((resource) => append_resource(resource, asset));
    }
  }

  // Include unlinked and legacy DownloadResource rows too. Already linked
  // files are deduplicated while retaining their richer ContentAsset metadata.
  for (const resource of content.resources || []) {
    append_resource(resource);
  }
  return entries;
}

function mounted_dom_element(event) {
  let target = event && event.target ? event.target : event;
  for (let depth = 0; depth < 4; depth += 1) {
    if (
      target &&
      target.nodeType === 1 &&
      typeof target.setAttribute === "function"
    ) {
      return target;
    }
    if (target && typeof target.get$elm === "function") {
      target = target.get$elm();
      continue;
    }
    if (target && target.$elm) {
      target = target.$elm;
      continue;
    }
    break;
  }
  return null;
}

function normalized_content_theme(value) {
  const theme = String(value || "")
    .trim()
    .toLowerCase();
  return theme === "light" || theme === "dark" ? theme : "";
}

function resolved_content_theme() {
  const root = document.documentElement;
  const body = document.body;
  const explicit_theme =
    normalized_content_theme(root && root.dataset.theme) ||
    normalized_content_theme(body && body.dataset.weuiTheme) ||
    normalized_content_theme(root && root.style.colorScheme);
  if (explicit_theme) {
    return explicit_theme;
  }
  if (root && root.classList.contains("dark")) {
    return "dark";
  }
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  } catch {
    return "light";
  }
}

function apply_html_document_theme(frame) {
  const theme = resolved_content_theme();
  frame.style.colorScheme = theme;
  frame.dataset.theme = theme;
  try {
    const document_ = frame.contentDocument;
    if (!document_ || !document_.documentElement) {
      return;
    }
    const root = document_.documentElement;
    root.dataset.theme = theme;
    root.style.colorScheme = theme;
    root.classList.toggle("dark", theme === "dark");
    if (document_.body) {
      document_.body.dataset.weuiTheme = theme;
    }
  } catch (error) {
    // External documents can deny DOM access; the iframe color-scheme still
    // supplies the correct prefers-color-scheme value to embedded content.
    console.debug("unable to sync html preview theme", error);
  }
}

function ContentDetailHTMLDocument(props) {
  const initial_theme = resolved_content_theme();
  return Timeless.Webview({
    class: "content-detail-media-document content-detail-media-document-html",
    href: props.media.url,
    style: { "color-scheme": initial_theme },
    attributes: {
      title: props.media.name,
      loading: "eager",
      sandbox: "allow-same-origin",
      "data-theme": initial_theme,
    },
    onMounted(event) {
      const frame = mounted_dom_element(event);
      if (!frame) {
        return undefined;
      }
      const apply_theme = () => apply_html_document_theme(frame);
      const observer = new MutationObserver(apply_theme);
      const observer_options = {
        attributes: true,
        attributeFilter: [
          "class",
          "data-theme",
          "data-weui-theme",
          "style",
        ],
      };
      observer.observe(document.documentElement, observer_options);
      if (document.body) {
        observer.observe(document.body, observer_options);
      }
      const media_query = window.matchMedia("(prefers-color-scheme: dark)");
      if (typeof media_query.addEventListener === "function") {
        media_query.addEventListener("change", apply_theme);
      } else if (typeof media_query.addListener === "function") {
        media_query.addListener(apply_theme);
      }
      frame.addEventListener("load", apply_theme);
      apply_theme();
      return () => {
        observer.disconnect();
        frame.removeEventListener("load", apply_theme);
        if (typeof media_query.removeEventListener === "function") {
          media_query.removeEventListener("change", apply_theme);
        } else if (typeof media_query.removeListener === "function") {
          media_query.removeListener(apply_theme);
        }
      };
    },
  });
}

function ContentDetailMediaStage(props) {
  const vm$ = props.store;
  const media = props.media;
  const cover_url = vm$.methods.coverURL(props.content);
  let player = null;
  if (!media.available) {
    player = View({ class: "content-detail-media-file-stage" }, [
      View({ class: "content-detail-media-file-icon" }, [
        Timeless.Icon({ name: content_media_type_icon(media.type), size: 48 }),
      ]),
      View({ class: "content-detail-media-file-title" }, [media.name]),
      View({ class: "content-detail-media-file-text" }, [
        "文件尚未下载、下载未完成，或本地文件已被删除。",
      ]),
    ]);
  } else if (media.type === "video") {
    player = Timeless.Video({
      class: "content-detail-media-video",
      src: media.url,
      poster: cover_url,
      controls: true,
      playsInline: true,
      preload: "metadata",
    });
  } else if (media.type === "audio") {
    player = View({
      class: [
        "content-detail-media-audio-stage",
        cover_url ? "" : "content-detail-media-audio-stage-no-artwork",
      ]
        .filter(Boolean)
        .join(" "),
    }, [
      cover_url
        ? View({ class: "content-detail-media-artwork" }, [
            LazyImg({
              class: "content-detail-media-artwork-image",
              src: cover_url,
              alt: "",
              attributes: { referrerpolicy: "no-referrer" },
            }),
            View({ class: "content-detail-media-artwork-fallback" }, [
              Timeless.Icon({ name: "file-volume", size: 42 }),
            ]),
          ])
        : null,
      Timeless.Audio({
        class: "content-detail-media-audio",
        src: media.url,
        controls: true,
        preload: "metadata",
      }),
    ].filter(Boolean));
  } else if (media.type === "image") {
    player = Img({
      class: "content-detail-media-image",
      src: media.url,
      alt: media.name,
      attributes: { loading: "eager" },
    });
  } else if (media.type === "html") {
    player = ContentDetailHTMLDocument({ media });
  } else if (["pdf", "text"].includes(media.type)) {
    player = Timeless.Webview({
      class: `content-detail-media-document content-detail-media-document-${media.type}`,
      href: media.url,
      attributes: {
        title: media.name,
        loading: "eager",
      },
    });
  } else {
    player = View({ class: "content-detail-media-file-stage" }, [
      View({ class: "content-detail-media-file-icon" }, [
        Timeless.Icon({ name: content_media_type_icon(media.type), size: 48 }),
      ]),
      View({ class: "content-detail-media-file-title" }, [media.name]),
      View({ class: "content-detail-media-file-text" }, [
        "此文件类型无法在浏览器中直接预览，请打开原文件查看。",
      ]),
    ]);
  }

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
      class: `content-detail-media-stage content-detail-media-stage-${media.type}`,
    },
    [
      View({ class: "content-detail-media-viewport" }, [player]),
      View({ class: "content-detail-media-caption" }, [
        View({ class: "content-detail-media-caption-icon" }, [
          Timeless.Icon({ name: content_media_type_icon(media.type), size: 16 }),
        ]),
        View({ class: "content-detail-media-caption-main" }, [
          View(
            {
              class: "content-detail-media-name",
              attributes: { title: media.name },
            },
            [media.name],
          ),
          View({ class: "content-detail-media-meta" }, [meta]),
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
      ].filter(Boolean)),
    ],
  );
}

function ContentDetailMediaPicker(props) {
  const vm$ = props.store;
  const media = props.media;
  const selected_ = props.selected;
  return View(
    {
      type: "button",
      class: computed(selected_, (selected) =>
        selected && selected.key === media.key
          ? "content-detail-media-choice dm-focus-ring is-selected"
          : "content-detail-media-choice dm-focus-ring",
      ),
      attributes: {
        type: "button",
        "aria-pressed": computed(selected_, (selected) =>
          selected && selected.key === media.key ? "true" : "false",
        ),
        title: `查看 ${media.name}`,
      },
      onClick() {
        selected_.as(media);
      },
    },
    [
      View({ class: "content-detail-media-choice-icon" }, [
        Timeless.Icon({ name: content_media_type_icon(media.type), size: 16 }),
      ]),
      View({ class: "content-detail-media-choice-main" }, [
        View({ class: "content-detail-media-choice-name" }, [media.name]),
        View({ class: "content-detail-media-choice-meta" }, [
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
    return View({ class: "content-detail-media-empty" }, [
      View({ class: "content-detail-media-empty-icon" }, [
        Timeless.Icon({ name: "play", size: 24 }),
      ]),
      View({ class: "content-detail-media-empty-title" }, [
        "还没有内容资产",
      ]),
      View({ class: "content-detail-media-empty-text" }, [
        "视频、音频、图片、HTML、PDF 及其他下载文件会显示在这里。",
      ]),
    ]);
  }
  const selected_ = ref(media.find((item) => item.available) || media[0]);
  return View({ class: "content-detail-extension" }, [
    View({ class: "content-detail-media-stage-list" }, [
      For({
        each: media,
        render(item) {
          return Show({
            when: computed(selected_, (selected) =>
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
    ]),
    media.length > 1
      ? View({ class: "content-detail-media-choices" }, [
          For({
            each: media,
            render(item) {
              return ContentDetailMediaPicker({
                store: vm$,
                media: item,
                selected: selected_,
              });
            },
          }),
        ])
      : null,
  ].filter(Boolean));
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
    { class: "content-detail-resource" },
    [
      View({ class: "content-detail-resource-icon" }, [
        Timeless.Icon({ name: vm$.methods.fileTypeIcon(resource), size: 18 }),
      ]),
      View({ class: "content-detail-resource-main" }, [
        View(
          {
            class: deleted
              ? "content-detail-resource-name is-deleted"
              : "content-detail-resource-name",
            attributes: { title: name },
          },
          [name],
        ),
        meta
          ? View({ class: "content-detail-resource-meta" }, [meta])
          : null,
        resource.local_path
          ? View(
              {
                class: "content-detail-resource-path",
                attributes: { title: resource.local_path },
              },
              [resource.local_path],
            )
          : null,
      ].filter(Boolean)),
      deleted
        ? View(
            {
              class:
                "content-detail-status content-detail-status-deleted",
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
    return View({ class: "content-detail-empty" }, ["暂无资源文件"]);
  }
  return View({ class: "content-detail-resource-list" }, [
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
  return View({ class: "content-detail-task" }, [
    View({ class: "content-detail-task-id" }, [
      task.id ? `#${task.id}` : "-",
    ]),
    View({ class: "content-detail-task-main" }, [
      View(
        {
          class: "content-detail-task-name",
          attributes: { title: name },
        },
        [name],
      ),
      View({ class: "content-detail-task-meta" }, [
        vm$.methods.formatTime(task.updated_at || task.created_at),
      ]),
    ]),
    View(
      {
        class: `content-detail-status content-detail-status-${status.tone}`,
      },
      [status.label],
    ),
  ]);
}

function ContentDetailTasks(props) {
  if (!props.tasks.length) {
    return View({ class: "content-detail-empty" }, ["暂无下载任务"]);
  }
  return View({ class: "content-detail-task-list" }, [
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
        "content-detail-relation dm-focus-ring",
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
      View({ class: "content-detail-relation-icon" }, [
        Timeless.Icon({ name: "git-branch", size: 17 }),
      ]),
      View({ class: "content-detail-relation-main" }, [
        View(
          {
            class: "content-detail-relation-name",
            attributes: { title },
          },
          [title],
        ),
        View({ class: "content-detail-relation-meta" }, [
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
    return View({ class: "content-detail-empty" }, ["暂无关联内容"]);
  }
  return View({ class: "content-detail-relation-list" }, [
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

function ContentDetailMain(props) {
  const vm$ = props.store;
  const content = props.content;
  const description = String(content.description || "").trim();
  const cover_url = vm$.methods.coverURL(content);
  return View({ class: "content-detail-layout" }, [
    View({
      class: [
        "content-detail-summary dm-panel",
        cover_url ? "" : "content-detail-summary-no-cover",
      ]
        .filter(Boolean)
        .join(" "),
    }, [
      cover_url
        ? View({ class: "content-detail-cover" }, [
            ContentDetailCover({ store: vm$, content, coverURL: cover_url }),
          ])
        : null,
      View({ class: "content-detail-info" }, [
        View({ class: "content-card-tags" }, [
          ContentDetailPlatform({ store: vm$, content }),
          View({ class: "content-type-badge" }, [
            vm$.methods.typeLabel(content.content_type),
          ]),
        ]),
        View(
          {
            class: "content-detail-title",
            attributes: { title: content.title },
          },
          [content.title],
        ),
        ContentDetailAccounts({ content, history: props.history }),
        description
          ? View({ class: "content-detail-description" }, [description])
          : null,
        View({ class: "content-detail-publish-time" }, [
          Timeless.Icon({ name: "clock3", size: 14 }),
          `发布于 ${vm$.methods.formatTime(content.publish_time)}`,
        ]),
      ].filter(Boolean)),
    ].filter(Boolean)),
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
  return View({ class: "content-detail-layout" }, [
    View({ class: "content-detail-summary dm-panel" }, [
      View({ class: "content-detail-cover content-skeleton" }),
      View({ class: "content-detail-info" }, [
        View({ class: "content-skeleton content-skeleton-tag" }),
        View({ class: "content-skeleton content-skeleton-title" }),
        View({ class: "content-skeleton content-skeleton-line" }),
        View({ class: "content-skeleton content-skeleton-line-short" }),
      ]),
    ]),
  ]);
}

function ContentDetailBody(props) {
  const vm$ = props.store;
  return View({ class: "content-main content-detail-main container" }, [
    Show({
      when: vm$.state.loading,
      ok() {
        return ContentDetailSkeleton();
      },
      else() {
        return Show({
          when: computed(vm$.state.error, (error) => Boolean(error)),
          ok() {
            return View({ class: "content-state" }, [
              Timeless.Icon({ name: "circle-alert", size: 32 }),
              View({ class: "content-state-title" }, ["内容加载失败"]),
              View({ class: "content-state-text" }, [vm$.state.error]),
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
                });
              },
              else() {
                return View({ class: "content-state" }, [
                  Timeless.Icon({ name: "inbox", size: 36 }),
                  View({ class: "content-state-title" }, ["暂无内容详情"]),
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
        "content-page content-detail-page page",
        props.embedded ? "is-embedded" : "",
      ]
        .filter(Boolean)
        .join(" "),
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      ContentDetailBody({
        store: vm$,
        history: props.history,
        onOpenDetail: props.embedded
          ? (content_id) => vm$.methods.openDetail(content_id)
          : null,
      }),
    ],
  );
}

export default ContentDetailPageView;

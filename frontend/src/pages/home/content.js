import { ContentPageModel } from "./content.model.js";
import { ProxyImg } from "@/components/proxy-img.js";

function contentIconName(content) {
  const type = String(
    content.output_format || content.content_type || content.type || "",
  ).toLowerCase();
  switch (type) {
    case "article":
    case "html":
    case "blog":
      return "file-code";
    case "json":
      return "braces";
    case "txt":
    case "md":
      return "file-text";
    case "image":
    case "image_set":
    case "jpg":
    case "jpeg":
    case "png":
    case "webp":
      return "image";
    case "audio":
    case "mp3":
    case "m4a":
      return "music";
    case "video":
    case "short_video":
    case "mp4":
    case "webm":
      return "film";
    case "pdf":
      return "file-type";
    case "zip":
    case "archive":
      return "archive";
    default:
      return "file";
  }
}

function normalizeTypeLabel(value) {
  return String(value || "").trim().toLowerCase();
}

function sourceTypeLabel(content) {
  return normalizeTypeLabel(
    content.source_content_type ||
      content.media_type ||
      content.content_type ||
      content.type ||
      "file",
  );
}

function fileTypeLabel(content) {
  const output = normalizeTypeLabel(content.output_format);
  if (output) return output;
  const display = normalizeTypeLabel(content.display_type || content.type_label);
  if (display.includes(" ")) {
    const parts = display.split(/\s+/);
    return parts[parts.length - 1];
  }
  return display || sourceTypeLabel(content);
}

function fullTypeLabel(content) {
  return normalizeTypeLabel(
    content.display_type ||
      content.type_label ||
      [sourceTypeLabel(content), fileTypeLabel(content)]
        .filter(Boolean)
        .filter((value, index, values) => values.indexOf(value) === index)
        .join(" "),
  );
}

function shouldShowContentCover(content) {
  return String(content.cover_url || "").trim() !== "";
}

function TypeBadge(label, tone = "content") {
  const toneClass =
    tone === "file"
      ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300"
      : "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-300";
  return View(
    {
      dataset: { t: "home-content-page-content-card-type-badge" },
      class:
        "inline-flex max-w-full items-center rounded border px-1.5 py-0.5 text-[11px] font-semibold leading-4 " +
        toneClass,
      title: label,
    },
    [label],
  );
}

function PlatformTag(content) {
  const platform = content.platform || {};
  const name =
    platform.name ||
    content.platform_name ||
    content.platform_id ||
    "未知平台";
  const faviconURL =
    platform.favicon_url ||
    content.platform_favicon_url ||
    platform.logo_url ||
    "";
  return View(
    {
      dataset: { t: "home-content-page-content-card-platform-tag" },
      class:
        "inline-flex h-6 max-w-full items-center gap-1.5 rounded border border-zinc-200 bg-zinc-50 px-1.5 text-xs font-medium text-zinc-700 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-300",
      title: name,
    },
    [
      faviconURL
        ? ProxyImg({
            class: "h-4 w-4 shrink-0 rounded-sm",
            src: faviconURL,
            alt: name,
          })
        : View(
            {
              dataset: { t: "home-content-page-content-card-platform-tag-fallback" },
              class:
                "flex h-4 w-4 shrink-0 items-center justify-center rounded-sm bg-zinc-200 text-[10px] text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
            },
            [String(name || "?").slice(0, 1)],
          ),
      View(
        {
          dataset: { t: "home-content-page-content-card-platform-tag-name" },
          class: "min-w-0 truncate",
        },
        [name],
      ),
    ],
  );
}

function ContentCard(content, vm$) {
  const accountName = content.account?.nickname || content.account?.username || "";
  const description = String(content.description || "").trim();
  const title = String(content.title || "").trim();
  const visibleDescription = description && description !== title ? description : "";
  const sourceType = sourceTypeLabel(content);
  const fileType = fileTypeLabel(content);
  const displayType = fullTypeLabel(content);
  const showCover = shouldShowContentCover(content);
  const sourceURL =
    content.source_url ||
    content.SourceURL ||
    content.url ||
    content.URL ||
    content.content_url ||
    content.ContentURL ||
    "";

  return View(
    {
      dataset: { t: "home-content-page-content-card-card-image-or-icon-content-icon-name-content-title-row-text-stack" },
      class:
        "group h-full cursor-pointer overflow-hidden rounded-md border border-zinc-200 bg-white shadow-sm transition hover:border-zinc-300 hover:shadow-md dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
      onClick() {
        vm$.methods.open(content);
      },
    },
    [
      View({ dataset: { t: "home-content-page-content-card-cover-media-image-or-icon-content-icon-name-content-title-row-text" }, class: "relative aspect-[16/10] bg-zinc-100 dark:bg-zinc-900" }, [
        Show({
          when: showCover,
          ok() {
            return View({ dataset: { t: "home-content-page-content-card-cover-image-wrap" }, class: "h-full w-full" }, [
              ProxyImg({
                class:
                  "h-full w-full object-cover transition group-hover:scale-105",
                src: content.cover_url,
                alt: content.title,
                platformId: content.platform_id || content.account?.platform_id,
                contentType: content.content_type || content.type,
              }),
              View({ dataset: { t: "home-content-page-content-card-cover-type-stack" }, class: "absolute left-2 top-2 flex max-w-[calc(100%-16px)] flex-wrap gap-1" }, [
                TypeBadge(fileType.toUpperCase(), "file"),
              ]),
            ]);
          },
          else() {
            return View({ dataset: { t: "home-content-page-content-card-row-icon-content-icon-name" }, class: "flex h-full w-full items-center justify-center text-zinc-400" }, [
              View({ dataset: { t: "home-content-page-content-card-file-visual" }, class: "flex flex-col items-center gap-2" }, [
                Icon({ name: contentIconName(content), size: 34 }),
                View({ dataset: { t: "home-content-page-content-card-file-visual-type" }, class: "rounded bg-white px-2 py-1 text-xs font-semibold uppercase text-zinc-700 shadow-sm dark:bg-zinc-950 dark:text-zinc-200" }, [
                  displayType || fileType || sourceType,
                ]),
              ]),
            ]);
          },
        }),
        View({ dataset: { t: "home-content-page-content-card-content-title-row-text" }, class: "absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent p-2 text-white" }, [
          View({ dataset: { t: "home-content-page-content-card-row-text" }, class: "mt-1 flex items-center justify-between text-xs text-white/80" }, [
            vm$.methods.formatBytes(content.file_size),
            content.publish_time ? vm$.methods.formatDate(Number(content.publish_time) * 1000) : "",
          ]),
        ]),
      ]),
      View({ dataset: { t: "home-content-page-content-card-stack" }, class: "space-y-2 p-2.5" }, [
        View({ dataset: { t: "home-content-page-content-card-platform-row" }, class: "flex min-w-0 items-center" }, [
          PlatformTag(content),
        ]),
        View({ dataset: { t: "home-content-page-content-card-content-title-text" }, class: "line-clamp-2 min-h-10 text-sm font-medium leading-5 text-zinc-900 dark:text-zinc-50" }, [content.title]),
        View({ dataset: { t: "home-content-page-content-card-type-row" }, class: "flex min-w-0 items-center justify-between gap-2" }, [
          View({ dataset: { t: "home-content-page-content-card-type-badges" }, class: "flex min-w-0 flex-wrap gap-1" }, [
            TypeBadge(`内容 ${sourceType}`, "content"),
            TypeBadge(`文件 ${fileType}`, "file"),
          ]),
          Show({
            when: !!sourceURL,
            ok() {
              return Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "ghost",
                    size: "sm",
                    onClick(event) {
                      event?.stopPropagation?.();
                      vm$.methods.openSource(content);
                    },
                  }),
                  title: "打开源地址",
                },
                [Icon({ name: "external-link", size: 14 }), "源"],
              );
            },
          }),
        ]),
        Show({
          when: !!content.account,
          ok() {
            return View({ dataset: { t: "home-content-page-content-card-row-image-or-row-text-account-name-value-未知帐号" }, class: "flex items-center gap-2" }, [
              View(
                {
                  dataset: { t: "home-content-page-content-card-avatar-or-badge-image-or-row-text" },
                  class:
                    "h-7 w-7 shrink-0 overflow-hidden rounded-full bg-zinc-100 ring-1 ring-zinc-200 dark:bg-zinc-900 dark:ring-zinc-800",
                },
                [
                  content.account.avatar_url
                    ? ProxyImg({
                        class: "h-full w-full object-cover",
                        src: content.account.avatar_url,
                        alt: accountName,
                        platformId:
                          content.account.platform_id || content.platform_id,
                        contentType: content.content_type || content.type,
                      })
                    : View(
                        {
                          dataset: { t: "home-content-page-content-card-row-text-2" },
                          class:
                            "flex h-full w-full items-center justify-center text-xs font-medium text-zinc-500 dark:text-zinc-400",
                        },
                        [String(accountName || "?").slice(0, 1)],
                      ),
                ],
              ),
              View({ dataset: { t: "home-content-page-content-card-account-name-value-未知帐号-text" }, class: "min-w-0 truncate text-sm font-medium text-zinc-700 dark:text-zinc-300" }, [
                accountName || "未知帐号",
              ]),
            ]);
          },
        }),
        Show({
          when: !!visibleDescription,
          ok() {
            return View({ dataset: { t: "home-content-page-content-card-visible-description-value-text" }, class: "line-clamp-2 text-sm text-zinc-500 dark:text-zinc-400" }, [visibleDescription]);
          },
        }),
      ]),
    ],
  );
}

export default function ContentPageView(props) {
  const vm$ = ContentPageModel(props);

  return View(
    {
      dataset: { t: "home-page-content-library-page-root-row-内容列表-查看已归档的内容-input-button-scroll-view" },
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
    },
    [
      View({ dataset: { t: "home-page-content-library-header-内容列表-查看已归档的内容-input-button" }, class: "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950" }, [
        View({ dataset: { t: "home-page-content-library-row-内容列表-查看已归档的内容-input-button" }, class: "flex flex-wrap items-center justify-between gap-3" }, [
          View({ dataset: { t: "home-page-content-library-内容列表-查看已归档的内容" } }, [
            View({ dataset: { t: "home-page-content-library-内容列表-heading" }, class: "text-2xl font-semibold text-zinc-950 dark:text-zinc-50" }, ["内容列表"]),
            View({ dataset: { t: "home-page-content-library-查看已归档的内容-text" }, class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, ["查看已归档的内容"]),
          ]),
          View({ dataset: { t: "home-page-content-library-row-input-button" }, class: "flex min-w-[280px] gap-2" }, [
            View({ dataset: { t: "home-page-content-library-row-input" }, class: "flex-1" }, [Input({ store: vm$.ui.keyword })]),
            Button(
              {
                store: new Timeless.ui.ButtonCore({
                  variant: "outline",
                  onClick() {
                    vm$.methods.search();
                  },
                }),
              },
              ["搜索"],
            ),
          ]),
        ]),
      ]),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ dataset: { t: "home-content-page-scroll-content-content-grid-region" }, class: "p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View({ dataset: { t: "home-page-content-library-error-card-vm-state-error-text" }, class: "mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300" }, [
                vm$.state.error,
              ]);
            },
          }),
          Show({
            when: computed(vm$.state.content, (list) => list.length === 0),
            ok() {
              return View({ dataset: { t: "home-page-content-library-row-icon-file-加载中-or-暂无内容" }, class: "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500" }, [
                Icon({ name: "file", size: 36 }),
                computed(vm$.state.loading, (loading) => (loading ? "加载中..." : "暂无内容")),
              ]);
            },
            else() {
              return View({ dataset: { t: "home-page-content-library-stack-vm-state-content-list" }, class: "space-y-5" }, [
                View({ dataset: { t: "home-page-content-library-grid-vm-state-content-list" }, class: "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6" }, [
                  For({
                    each: vm$.state.content,
                    render(content) {
                      return ContentCard(content, vm$);
                    },
                  }),
                ]),
                Show({
                  when: computed(vm$.state.noMore, (v) => !v),
                  ok() {
                    return View({ dataset: { t: "home-page-content-library-row-button" }, class: "flex justify-center py-4" }, [
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            disabled: vm$.state.loading,
                            onClick() {
                              vm$.methods.loadMore();
                            },
                          }),
                        },
                        [computed(vm$.state.loading, (v) => (v ? "加载中..." : "加载更多"))],
                      ),
                    ]);
                  },
                }),
              ]);
            },
          }),
        ]),
      ]),
    ],
  );
}

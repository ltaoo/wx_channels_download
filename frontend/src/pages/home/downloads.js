/* global classNames, Select */
import { ProxyImg } from "@/components/proxy-img.js";
import {
  DownloadTaskStatus,
  DownloadsPageModel,
  formatBytes,
} from "./downloads.model.js";

function mapStatusClassName(status) {
  if (status === DownloadTaskStatus.Running) {
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  }
  if (status === DownloadTaskStatus.Done) {
    return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  }
  if (status === DownloadTaskStatus.Error) {
    return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
  }
  if (status === DownloadTaskStatus.Paused) {
    return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
  }
  if (
    status === DownloadTaskStatus.Wait ||
    status === DownloadTaskStatus.Ready
  ) {
    return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
  }
  return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
}

function mapProgressClassName(status) {
  if (status === DownloadTaskStatus.Running) {
    return "bg-blue-500 dark:bg-blue-400";
  }
  if (status === DownloadTaskStatus.Done) {
    return "bg-emerald-500 dark:bg-emerald-400";
  }
  if (status === DownloadTaskStatus.Error) {
    return "bg-red-500 dark:bg-red-400";
  }
  if (status === DownloadTaskStatus.Paused) {
    return "bg-zinc-500 dark:bg-zinc-400";
  }
  return "bg-amber-500 dark:bg-amber-400";
}

function HeaderStat(props) {
  const { label, value, icon, class: cls = "" } = props;
  return View(
    {
      class: [
        "rounded-lg border border-zinc-200 bg-white p-4 ",
        "dark:border-zinc-800 dark:bg-zinc-950",
        cls,
      ].join(" "),
    },
    [
      View({ class: "flex items-center justify-between gap-3" }, [
        View({ class: "truncate text-sm text-zinc-500 dark:text-zinc-400" }, [
          label,
        ]),
        Icon({ name: icon, size: 18 }),
      ]),
      View(
        {
          class:
            "mt-2 truncate text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
        },
        [value],
      ),
    ],
  );
}

function countForTab(stats, tab) {
  if (!stats || !tab) return 0;
  return Number(stats[tab.countKey] || 0);
}

function isStartableStatus(status) {
  return (
    status === DownloadTaskStatus.Ready ||
    status === DownloadTaskStatus.Wait ||
    status === DownloadTaskStatus.Paused ||
    status === DownloadTaskStatus.Error
  );
}

function isPlayableStatus(status) {
  return status === DownloadTaskStatus.Done;
}

function parseTaskJSON(value) {
  if (!value) return {};
  if (typeof value === "object") return value;
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

function isHTMLTask(task) {
  const metadata2 = parseTaskJSON(task.metadata2 || task.Metadata2);
  const labels = parseTaskJSON(
    task.labels || task.Labels || task.extra || task.Extra,
  );
  const contentType = String(
    task.content_type ||
      task.contentType ||
      task.mime_type ||
      task.mimeType ||
      metadata2.content_type ||
      metadata2.contentType ||
      metadata2.mime_type ||
      metadata2.mimeType ||
      labels.content_type ||
      labels.contentType ||
      labels.mime_type ||
      labels.mimeType ||
      "",
  )
    .trim()
    .toLowerCase();
  if (contentType === "html" || contentType === "text/html") return true;

  const path = String(task.filepath || task.path || task.name || task.url || "")
    .split("?")[0]
    .split("#")[0]
    .toLowerCase();
  return path.endsWith(".html") || path.endsWith(".htm");
}

function taskPlayLabel(task) {
  return isHTMLTask(task) ? "在浏览器打开" : "播放";
}

function DownloadInfoItem(props) {
  const { label, value, class: cls = "" } = props;
  return View(
    {
      class: classNames([
        "flex min-w-0 items-center gap-1 text-xs text-zinc-500 dark:text-zinc-400",
        cls,
      ]),
    },
    [
      View({ class: "shrink-0 text-zinc-400 dark:text-zinc-500" }, [label]),
      View(
        {
          class:
            "min-w-0 truncate font-medium text-zinc-700 dark:text-zinc-200",
        },
        [value],
      ),
    ],
  );
}

function DownloadInfoDivider() {
  return View(
    { class: "hidden h-3 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800 sm:block" },
    [],
  );
}

function DownloadInfoBar(task) {
  return View(
    {
      class:
        "min-w-0 rounded-md border border-zinc-100 bg-zinc-50 px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900/60",
    },
    [
      View({ class: "flex items-center gap-3" }, [
        View(
          {
            class:
              "w-11 shrink-0 text-right text-sm font-semibold tabular-nums text-zinc-950 dark:text-zinc-50",
          },
          [
            computed(task, (t) => {
              return `${Math.floor(t.percent ?? t.progress_info.percent)}%`;
            }),
          ],
        ),
        View(
          {
            class:
              "h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800",
          },
          [
            View({
              class: classNames([
                "h-full rounded-full transition-all",
                computed(task, (t) => {
                  return mapProgressClassName(t.status);
                }),
              ]),
              style: {
                width: computed(task, (t) => {
                  return `${t.percent ?? t.progress_info.percent}%`;
                }),
              },
            }),
          ],
        ),
      ]),
      View(
        {
          class:
            "mt-2 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 pl-0 sm:pl-14",
        },
        [
          DownloadInfoItem({
            label: "大小",
            value: computed(task, (t) => t.size_text),
          }),
          DownloadInfoDivider(),
          DownloadInfoItem({
            label: "已下",
            value: computed(task, (t) => {
              return t.progress_info.total
                ? formatBytes(t.progress_info.downloaded)
                : "-";
            }),
          }),
          DownloadInfoDivider(),
          DownloadInfoItem({
            label: "速度",
            value: computed(task, (t) =>
              t.status === DownloadTaskStatus.Running ? t.speed_text : "-",
            ),
            icon: "tabular-nums",
          }),
          DownloadInfoDivider(),
          DownloadInfoItem({
            label: "更新",
            value: computed(task, (t) => t.updated_at_text),
          }),
          Show({
            when: computed(
              task,
              (t) => t.status === DownloadTaskStatus.Error && !!t.error,
            ),
            ok() {
              return [
                DownloadInfoDivider(),
                DownloadInfoItem({
                  label: "原因",
                  value: computed(task, (t) => t.error),
                  class: "max-w-full text-red-600 dark:text-red-300",
                }),
              ];
            },
          }),
        ],
      ),
    ],
  );
}

function TaskCard(task, vm$) {
  const deleteFileCheckbox$ = new Timeless.ui.CheckboxCore({});
  const deleteFileCheckboxId = `delete-file-${task.id || task.task_id}`;

  return View(
    {
      class:
        "group rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View({ class: "flex flex-col gap-4 lg:flex-row lg:items-start" }, [
        View(
          {
            class:
              "grid min-w-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,360px)_auto]",
          },
          [
            View({ class: "min-w-0" }, [
              View({ class: "flex items-start gap-3" }, [
                Show({
                  when: computed(
                    task,
                    (t) => t.display_cover_url || t.cover_url,
                  ),
                  ok() {
                    return View(
                      {
                        class:
                          "h-20 w-20 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                      },
                      [
                        ProxyImg({
                          class: "h-full w-full object-cover",
                          src: computed(
                            task,
                            (t) => t.display_cover_url || t.cover_url,
                          ),
                          alt: computed(task, (t) => t.title || "cover"),
                          platformId: computed(
                            task,
                            (t) => t.platform_id || t.platform,
                          ),
                          contentType: computed(
                            task,
                            (t) => t.content_type || t.type,
                          ),
                        }),
                      ],
                    );
                  },
                }),
                View({ class: "min-w-0 flex-1" }, [
                  View(
                    {
                      class:
                        "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                      // title: task.title || task.task_id,
                    },
                    [task.title || task.name || task.task_id || "未命名任务"],
                  ),
                  View(
                    {
                      class:
                        "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [task.filepath || task.url || "-"],
                  ),
                  View({ class: "mt-2" }, [
                    Show({
                      when: computed(task, (t) => isPlayableStatus(t.status)),
                      ok() {
                        return Button(
                          {
                            store: new Timeless.ui.ButtonCore({
                              variant: "outline",
                              size: "sm",
                              onClick() {
                                isHTMLTask(task)
                                  ? vm$.methods.openInBrowser(task)
                                  : vm$.methods.play(task);
                              },
                            }),
                          },
                          [computed(task, taskPlayLabel)],
                        );
                      },
                    }),
                    Show({
                      when: computed(
                        task,
                        (t) => t.status === DownloadTaskStatus.Done,
                      ),
                      ok() {
                        return Button(
                          {
                            store: new Timeless.ui.ButtonCore({
                              variant: "outline",
                              size: "sm",
                              onClick() {
                                vm$.methods.openFile(task);
                              },
                            }),
                          },
                          ["打开所在目录"],
                        );
                      },
                    }),
                  ]),
                ]),
                View(
                  {
                    class: classNames([
                      "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                      computed(task, (t) => mapStatusClassName(t.status)),
                    ]),
                  },
                  [computed(task, (t) => t.status_text)],
                ),
              ]),
            ]),
            DownloadInfoBar(task),
            View(
              {
                class:
                  "flex shrink-0 flex-wrap items-center gap-2 xl:w-28 xl:flex-col xl:items-stretch",
              },
              [
                Show({
                  when: computed(task, (t) => {
                    return t.status === DownloadTaskStatus.Error;
                  }),
                  ok() {
                    return Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          onClick() {
                            vm$.methods.retry(task);
                          },
                        }),
                      },
                      ["重试"],
                    );
                  },
                }),
                Show({
                  when: computed(task, (t) => isStartableStatus(t.status)),
                  ok() {
                    return Button(
                      {
                        store: new Timeless.ui.ButtonCore({
                          variant: "outline",
                          size: "sm",
                          onClick() {
                            vm$.methods.start(task);
                          },
                        }),
                      },
                      ["开始"],
                    );
                  },
                }),
                Button(
                  {
                    store: new Timeless.ui.ButtonCore({
                      variant: "ghost",
                      size: "sm",
                      onClick() {
                        vm$.methods.remove(task, deleteFileCheckbox$.value);
                      },
                    }),
                  },
                  ["删除"],
                ),
                View({ class: "flex items-center gap-1.5 xl:justify-start" }, [
                  Checkbox({
                    id: deleteFileCheckboxId,
                    store: deleteFileCheckbox$,
                  }),
                  Label(
                    {
                      for: deleteFileCheckboxId,
                      class:
                        "cursor-pointer whitespace-nowrap text-xs text-zinc-600 dark:text-zinc-300",
                    },
                    ["同时删除文件"],
                  ),
                ]),
              ],
            ),
          ],
        ),
      ]),
    ],
  );
}

function RemoteTaskCard(task) {
  return View(
    {
      class:
        "rounded-lg border border-sky-200 bg-white p-4 shadow-sm dark:border-sky-900 dark:bg-zinc-950",
    },
    [
      View({ class: "flex gap-4" }, [
        View(
          {
            class:
              "flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-300",
          },
          [Icon({ name: "server", size: 24 })],
        ),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex items-start justify-between gap-3" }, [
            View({ class: "min-w-0" }, [
              View(
                {
                  class:
                    "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                  // title: task.title || task.task_id,
                },
                [task.title || task.task_id || "未命名任务"],
              ),
              View(
                {
                  class:
                    "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                },
                [task.filepath || task.url || "-"],
              ),
            ]),
            View(
              {
                class: classNames([
                  "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                  computed(task, (t) => mapStatusClassName(t.status)),
                ]),
              },
              [computed(task, (t) => t.status_text)],
            ),
          ]),
          View({ class: "mt-3 space-y-2" }, [
            View(
              {
                class:
                  "h-2 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
              },
              [
                View({
                  class: "h-full rounded-full bg-sky-600 dark:bg-sky-300",
                  style: {
                    width: computed(task, (t) => `${t.progress_info.percent}%`),
                  },
                }),
              ],
            ),
            View(
              {
                class:
                  "flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400",
              },
              [
                computed(task, (t) => `${t.progress_info.percent}%`),
                computed(task, (t) => t.size_text),
                computed(task, (t) =>
                  t.status === DownloadTaskStatus.Running ? t.speed_text : "",
                ),
                "更新",
                computed(task, (t) => t.updated_at_text),
              ],
            ),
          ]),
        ]),
      ]),
    ],
  );
}

function RemoteServerPanel(vm$) {
  return Show({
    when: vm$.state.remoteEnabled,
    ok() {
      return View(
        {
          class:
            "space-y-3 rounded-lg border border-sky-200 bg-sky-50/50 p-4 dark:border-sky-900 dark:bg-sky-950/20",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({}, [
              View(
                {
                  class:
                    "flex items-center gap-2 text-base font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [Icon({ name: "server", size: 18 }), "RemoteServer"],
              ),
              View({ class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
                vm$.state.remoteLabel,
              ]),
            ]),
            View(
              {
                class:
                  "grid w-full gap-3 sm:grid-cols-4 xl:w-auto xl:min-w-[520px] xl:grid-cols-6",
              },
              [
                HeaderStat({
                  label: "远端任务",
                  value: computed(vm$.state.remoteTotal, (v) => String(v)),
                  icon: "list",
                }),
                HeaderStat({
                  label: "远端下载中",
                  value: computed(vm$.state.remoteRunningCount, (v) =>
                    String(v),
                  ),
                  icon: "activity",
                }),
                HeaderStat({
                  label: "远端速度",
                  value: vm$.state.remoteTotalSpeed,
                  icon: "gauge",
                  class: "sm:col-span-2 xl:col-span-4",
                }),
              ],
            ),
          ]),
          Show({
            when: vm$.state.remoteError,
            ok() {
              return View(
                {
                  class:
                    "rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.remoteError],
              );
            },
          }),
          Show({
            when: computed(vm$.state.remoteTasks, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-32 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "inbox", size: 28 }),
                  computed(vm$.state.remoteLoading, (loading) =>
                    loading ? "远端加载中..." : "暂无远端下载任务",
                  ),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.remoteTasks,
                  render(task) {
                    return RemoteTaskCard(task);
                  },
                }),
                Show({
                  when: computed(vm$.state.remoteNoMore, (v) => !v),
                  ok() {
                    return View({ class: "flex justify-center py-2" }, [
                      Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            disabled: vm$.state.remoteLoading,
                            onClick() {
                              vm$.methods.loadMoreRemote();
                            },
                          }),
                        },
                        [
                          computed(vm$.state.remoteLoading, (v) =>
                            v ? "加载中..." : "加载更多远端任务",
                          ),
                        ],
                      ),
                    ]);
                  },
                }),
              ]);
            },
          }),
        ],
      );
    },
  });
}

function platformLabel(platform) {
  const map = {
    wx_channels: "视频号",
    wx_official_account: "公众号",
    douyin: "抖音",
    zhihu: "知乎",
    officialaccount: "公众号",
    xiaohongshu: "小红书",
    bilibili: "B站",
    weibo: "微博",
    youtube: "YouTube",
  };
  return map[platform] || platform || "-";
}

function existingTaskText(list) {
  const total = Array.isArray(list) ? list.length : 0;
  if (!total) return "";
  const latest = list[0] || {};
  const statusMap = {
    0: "待下载",
    1: "下载中",
    2: "已暂停",
    3: "排队中",
    4: "已完成",
    5: "失败",
  };
  return `已存在 ${total} 个下载任务，最新状态：${statusMap[latest.status] || "未知"}`;
}

function formatJSON(value) {
  try {
    return JSON.stringify(value || {}, null, 2);
  } catch {
    return "{}";
  }
}

function firstNonEmpty(...values) {
  for (const value of values) {
    if (value === undefined || value === null) continue;
    const text = String(value).trim();
    if (text) return value;
  }
  return "";
}

function asArray(value) {
  if (Array.isArray(value)) return value;
  if (value === undefined || value === null || value === "") return [];
  return [value];
}

function formatCompactNumber(value) {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n <= 0) return "";
  if (n >= 10000) return `${(n / 10000).toFixed(n >= 100000 ? 0 : 1)}万`;
  return String(n);
}

function formatDurationSeconds(value) {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n <= 0) return "";
  const seconds = Math.floor(n);
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${m}:${String(s).padStart(2, "0")}`;
}

function formatTimestamp(value) {
  if (!value) return "";
  if (typeof value === "string" && Number.isNaN(Number(value))) return value;
  const n = Number(value);
  if (!Number.isFinite(n) || n <= 0) return "";
  return new Date(n < 10000000000 ? n * 1000 : n).toLocaleString();
}

function stripHTML(value) {
  return String(value || "")
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<[^>]*>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function shortText(value, max = 160) {
  const text = stripHTML(value);
  if (text.length <= max) return text;
  return `${text.slice(0, max)}...`;
}

function normalizeRawContentEnvelope(raw) {
  return raw?.output?.content || raw?.Output?.content || null;
}

function normalizeProbePreviewContent(content, probe, raw) {
  const envelope = normalizeRawContentEnvelope(raw);
  const summary = envelope?.summary || envelope?.Summary || {};
  const metadata = envelope?.metadata || envelope?.Metadata || {};
  const output = envelope?.output || envelope?.Output || raw?.output || {};
  const probeContent = probe?.content || probe?.Content || {};
  const base = content || {};
  const merged = {
    ...metadata,
    ...output,
    ...summary,
    ...probeContent,
    ...base,
  };
  merged.platform = firstNonEmpty(
    merged.platform,
    probe?.platform,
    raw?.platform,
  );
  merged.content_type = String(
    firstNonEmpty(
      merged.content_type,
      merged.type,
      output.content_type,
      summary.type,
      summary.Type,
      inferContentType(merged.platform, merged),
    ) || "",
  ).trim();
  merged.content_id = firstNonEmpty(
    merged.content_id,
    merged.external_id,
    merged.id,
    probe?.content_id,
  );
  merged.canonical_url = firstNonEmpty(
    merged.canonical_url,
    probe?.canonical_url,
    merged.url,
  );
  merged.source_url = firstNonEmpty(
    merged.source_url,
    probe?.source_url,
    raw?.url,
  );
  return merged;
}

function inferContentType(platform, content) {
  if (platform === "officialaccount" || platform === "wx_official_account") {
    return "article";
  }
  if (platform === "xiaohongshu") return "note";
  if (platform === "weibo") return "post";
  if (platform === "douyin" || platform === "wx_channels") {
    return content?.images || content?.image_count ? "image_album" : "video";
  }
  return "";
}

function contentTypeLabel(type) {
  const map = {
    video: "视频",
    image_album: "图集",
    image: "图片",
    note: "笔记",
    post: "动态",
    article: "文章",
    question: "问题",
    answer: "回答",
    live: "直播",
    collection: "合集",
    account: "帐号",
    topic: "话题",
  };
  return map[type] || type || "内容";
}

function IconMappedType(props) {
  return {
    video() {
      return Icon({
        name: "film",
        size: 28,
      });
    },
    image_album() {
      return Icon({
        name: "images",
        size: 28,
      });
    },
    image() {
      return Icon({
        name: "image",
        size: 28,
      });
    },
    note() {
      return Icon({
        name: "notebook-text",
        size: 28,
      });
    },
    post() {
      return Icon({
        name: "message-square-text",
        size: 28,
      });
    },
    article() {
      return Icon({
        name: "file-text",
        size: 28,
      });
    },
    question() {
      return Icon({
        name: "circle-help",
        size: 28,
      });
    },
    answer() {
      return Icon({
        name: "message-circle",
        size: 28,
      });
    },
    live() {
      return Icon({
        name: "radio",
        size: 28,
      });
    },
    collection() {
      return Icon({
        name: "list-video",
        size: 28,
      });
    },
    account() {
      return Icon({
        name: "user",
        size: 28,
      });
    },
    topic() {
      return Icon({
        name: "hash",
        size: 28,
      });
    },
  };
}

function contentAuthorName(content) {
  const author = content?.author;
  if (author && typeof author === "object") {
    return firstNonEmpty(
      author.nickname,
      author.name,
      author.username,
      author.id,
    );
  }
  return firstNonEmpty(
    author,
    content?.author_nickname,
    content?.account_nickname,
    content?.author_username,
    content?.author_id,
  );
}

function contentAuthorAvatar(content) {
  const author = content?.author;
  if (author && typeof author === "object") {
    return firstNonEmpty(author.avatar_url, author.avatarUrl);
  }
  return firstNonEmpty(content?.author_avatar_url, content?.account_avatar_url);
}

function imageURLOf(image) {
  if (!image) return "";
  if (typeof image === "string") return image;
  return firstNonEmpty(
    image.url,
    image.URL,
    image.src,
    image.thumbnail_url,
    image.cover_url,
  );
}

function contentImages(content) {
  const images = [
    ...asArray(content?.images),
    ...asArray(content?.preview_images),
    ...asArray(content?.files),
  ]
    .map(imageURLOf)
    .filter(Boolean);
  if (!images.length && content?.cover_url) return [content.cover_url];
  return [...new Set(images)];
}

function contentCoverURL(content) {
  return firstNonEmpty(content?.cover_url, contentImages(content)[0]);
}

function countImages(content) {
  return (
    Number(content?.image_count || 0) ||
    contentImages(content).filter((url) => url !== content?.cover_url).length
  );
}

function formatDimension(content) {
  const width = Number(content?.width || 0);
  const height = Number(content?.height || 0);
  if (!width || !height) return "";
  return `${width} x ${height}`;
}

function formatBoolean(value) {
  if (value === undefined || value === null || value === "") return "";
  return value ? "是" : "否";
}

function previewBadges(content) {
  const tags = [];
  if (content?.platform) tags.push(platformLabel(content.platform));
  tags.push(contentTypeLabel(content?.content_type));
  if (content?.visibility && content.visibility !== "public") {
    tags.push(String(content.visibility));
  }
  return tags.filter(Boolean);
}

function previewStats(content) {
  const stats = content?.stats || {};
  const pairs = [
    ["播放", firstNonEmpty(content?.play_count, stats.play_count)],
    ["浏览", firstNonEmpty(content?.view_count, stats.view_count)],
    ["点赞", firstNonEmpty(content?.like_count, stats.like_count)],
    ["评论", firstNonEmpty(content?.comment_count, stats.comment_count)],
    ["转发", firstNonEmpty(content?.repost_count, stats.repost_count)],
    ["分享", firstNonEmpty(content?.share_count, stats.share_count)],
    ["收藏", firstNonEmpty(content?.collect_count, stats.collect_count)],
    ["弹幕", firstNonEmpty(content?.danmaku_count, stats.danmaku_count)],
  ];
  return pairs
    .map(([label, value]) => [label, formatCompactNumber(value)])
    .filter(([, value]) => value);
}

function previewTags(content) {
  return [
    ...asArray(content?.tags),
    ...asArray(content?.topics).map((topic) =>
      typeof topic === "object" ? topic.name : topic,
    ),
  ]
    .map((tag) => String(tag || "").trim())
    .filter(Boolean)
    .slice(0, 8);
}

function previewTitle(content) {
  return firstNonEmpty(
    content?.title,
    content?.question_title,
    content?.name,
    content?.text,
    content?.content_id,
    "未命名内容",
  );
}

function previewDescription(content) {
  return firstNonEmpty(
    content?.description,
    content?.digest,
    content?.excerpt,
    content?.detail,
    content?.body_text,
    content?.body_html,
    content?.text,
  );
}

function PreviewBadge(label) {
  return View(
    {
      class:
        "inline-flex h-6 items-center rounded-md bg-zinc-100 px-2 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200",
    },
    [label],
  );
}

function PreviewInfoItem(label, data_, selector) {
  return Show({
    when: computed(
      data_,
      (content) => !!String(selector(content) || "").trim(),
    ),
    ok() {
      return View({ class: "min-w-0" }, [
        View({ class: "text-[11px] text-zinc-400 dark:text-zinc-500" }, [
          label,
        ]),
        View(
          {
            class:
              "mt-0.5 truncate text-xs font-medium text-zinc-700 dark:text-zinc-200",
          },
          [computed(data_, (content) => String(selector(content) || ""))],
        ),
      ]);
    },
  });
}

function PreviewInfoGrid(children) {
  return View(
    { class: "mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3" },
    children,
  );
}

function TypeSpecificPreview(data_) {
  return View({}, [
    Show({
      when: computed(data_, (content) => content.content_type === "video"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("时长", data_, (content) =>
            formatDurationSeconds(content.duration),
          ),
          PreviewInfoItem("分辨率", data_, formatDimension),
          PreviewInfoItem("帧率", data_, (content) =>
            content.fps ? `${content.fps} fps` : "",
          ),
          PreviewInfoItem("码率", data_, (content) =>
            content.bitrate ? `${formatCompactNumber(content.bitrate)}bps` : "",
          ),
          PreviewInfoItem("格式", data_, (content) => content.format),
          PreviewInfoItem("原创", data_, (content) =>
            formatBoolean(content.is_original),
          ),
        ]);
      },
    }),
    Show({
      when: computed(
        data_,
        (content) => content.content_type === "image_album",
      ),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("图片数", data_, (content) => countImages(content)),
          PreviewInfoItem("尺寸", data_, formatDimension),
          PreviewInfoItem("格式", data_, (content) => content.format),
          PreviewInfoItem("GIF", data_, (content) =>
            formatBoolean(content.is_gif),
          ),
          PreviewInfoItem("OCR", data_, (content) =>
            shortText(content.ocr_text, 48),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "note"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("笔记类型", data_, (content) => content.note_type),
          PreviewInfoItem("图片数", data_, (content) => countImages(content)),
          PreviewInfoItem("视频时长", data_, (content) =>
            formatDurationSeconds(content.video?.duration || content.duration),
          ),
          PreviewInfoItem(
            "商品卡片",
            data_,
            (content) => asArray(content.product_cards).length || "",
          ),
          PreviewInfoItem("收藏", data_, (content) =>
            formatCompactNumber(content.collect_count),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "post"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("图片数", data_, (content) => countImages(content)),
          PreviewInfoItem("视频时长", data_, (content) =>
            formatDurationSeconds(content.video?.duration || content.duration),
          ),
          PreviewInfoItem(
            "链接卡片",
            data_,
            (content) => asArray(content.link_cards).length || "",
          ),
          PreviewInfoItem("转发", data_, (content) =>
            content.repost_of || content.quote_of ? "是" : "",
          ),
          PreviewInfoItem("评论", data_, (content) =>
            formatCompactNumber(content.comment_count),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "article"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("摘要", data_, (content) =>
            shortText(firstNonEmpty(content.digest, content.description), 64),
          ),
          PreviewInfoItem("字数", data_, (content) =>
            formatCompactNumber(content.word_count),
          ),
          PreviewInfoItem("阅读时间", data_, (content) =>
            content.reading_time ? `${content.reading_time} 分钟` : "",
          ),
          PreviewInfoItem("图片数", data_, (content) => countImages(content)),
          PreviewInfoItem(
            "内嵌媒体",
            data_,
            (content) => asArray(content.embedded_media).length || "",
          ),
          PreviewInfoItem("正文", data_, (content) =>
            content.body_html || content.body_text ? "已解析" : "",
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "question"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("回答数", data_, (content) =>
            formatCompactNumber(content.answer_count),
          ),
          PreviewInfoItem("关注数", data_, (content) =>
            formatCompactNumber(content.follower_count),
          ),
          PreviewInfoItem("创建", data_, (content) =>
            formatTimestamp(content.created_time),
          ),
          PreviewInfoItem("更新", data_, (content) =>
            formatTimestamp(content.updated_time),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "answer"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("问题", data_, (content) => content.question_title),
          PreviewInfoItem("赞同", data_, (content) =>
            formatCompactNumber(content.vote_count),
          ),
          PreviewInfoItem("评论", data_, (content) =>
            formatCompactNumber(content.comment_count),
          ),
          PreviewInfoItem("创建", data_, (content) =>
            formatTimestamp(content.created_time),
          ),
          PreviewInfoItem("更新", data_, (content) =>
            formatTimestamp(content.updated_time),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "live"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("状态", data_, (content) => content.status),
          PreviewInfoItem("开始", data_, (content) =>
            formatTimestamp(content.start_time),
          ),
          PreviewInfoItem("结束", data_, (content) =>
            formatTimestamp(content.end_time),
          ),
          PreviewInfoItem("观众", data_, (content) =>
            formatCompactNumber(content.viewer_count),
          ),
          PreviewInfoItem("预约", data_, (content) =>
            formatCompactNumber(content.reservation_count),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "collection"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("内容数", data_, (content) =>
            formatCompactNumber(content.item_count),
          ),
          PreviewInfoItem("更新", data_, (content) =>
            formatTimestamp(content.updated_time),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "account"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("用户名", data_, (content) => content.username),
          PreviewInfoItem("粉丝", data_, (content) =>
            formatCompactNumber(content.follower_count),
          ),
          PreviewInfoItem("关注", data_, (content) =>
            formatCompactNumber(content.following_count),
          ),
          PreviewInfoItem("内容数", data_, (content) =>
            formatCompactNumber(content.content_count),
          ),
          PreviewInfoItem("认证", data_, (content) =>
            formatBoolean(content.verified),
          ),
        ]);
      },
    }),
    Show({
      when: computed(data_, (content) => content.content_type === "topic"),
      ok() {
        return PreviewInfoGrid([
          PreviewInfoItem("关注", data_, (content) =>
            formatCompactNumber(content.follower_count),
          ),
          PreviewInfoItem("浏览", data_, (content) =>
            formatCompactNumber(content.view_count),
          ),
          PreviewInfoItem("内容数", data_, (content) =>
            formatCompactNumber(content.post_count),
          ),
          PreviewInfoItem("相关话题", data_, (content) =>
            asArray(content.related_topics)
              .map((topic) => topic.name)
              .filter(Boolean)
              .slice(0, 3)
              .join("、"),
          ),
        ]);
      },
    }),
  ]);
}

function ContentPreviewMediaStrip(data_) {
  return Show({
    when: computed(data_, (content) => contentImages(content).length > 1),
    ok() {
      return View({ class: "mt-3 flex gap-2 overflow-hidden" }, [
        For({
          each: computed(data_, (content) =>
            contentImages(content).slice(0, 5),
          ),
          render(url) {
            return View(
              {
                class:
                  "h-12 w-12 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
              },
              [
                ProxyImg({
                  class: "h-full w-full object-cover",
                  src: url,
                  alt: "image",
                }),
              ],
            );
          },
        }),
      ]);
    },
  });
}

function ContentPreviewStats(data_) {
  return Show({
    when: computed(data_, (content) => previewStats(content).length > 0),
    ok() {
      return View({ class: "mt-3 flex flex-wrap gap-x-4 gap-y-1" }, [
        For({
          each: computed(data_, previewStats),
          render(pair) {
            return View({ class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
              View({ as: "span", class: "text-zinc-400 dark:text-zinc-500" }, [
                `${pair[0]} `,
              ]),
              View(
                {
                  as: "span",
                  class: "font-medium text-zinc-700 dark:text-zinc-200",
                },
                [pair[1]],
              ),
            ]);
          },
        }),
      ]);
    },
  });
}

function ContentPreviewTags(data_) {
  return Show({
    when: computed(data_, (content) => previewTags(content).length > 0),
    ok() {
      return View({ class: "mt-3 flex flex-wrap gap-1.5" }, [
        For({
          each: computed(data_, previewTags),
          render(tag) {
            return View(
              {
                class:
                  "inline-flex h-6 items-center rounded-md bg-zinc-50 px-2 text-xs text-zinc-500 ring-1 ring-inset ring-zinc-200 dark:bg-zinc-900 dark:text-zinc-300 dark:ring-zinc-800",
              },
              [tag],
            );
          },
        }),
      ]);
    },
  });
}

function ContentPreview(props) {
  const data_ = combine(
    {
      content: props.content,
      probe: props.probe,
      raw: props.raw,
    },
    ({ content, probe, raw }) =>
      normalizeProbePreviewContent(content, probe, raw),
  );

  return View(
    { class: "min-w-0 rounded-md bg-zinc-50 p-3 dark:bg-zinc-900/60" },
    [
      View({ class: "flex items-start gap-3" }, [
        View(
          {
            class:
              "h-20 w-20 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
          },
          [
            Show({
              when: computed(data_, contentCoverURL),
              ok() {
                return ProxyImg({
                  class: "h-full w-full object-cover",
                  src: computed(data_, contentCoverURL),
                  alt: computed(data_, previewTitle),
                });
              },
              else() {
                return View(
                  {
                    class:
                      "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                  },
                  [
                    // IconMappedType({
                    //   type:
                    // })
                    Match({
                      when: computed(data_, (content) => {
                        return content.content_type;
                      }),
                      cases: IconMappedType(),
                    }),
                  ],
                );
              },
            }),
          ],
        ),
        View({ class: "min-w-0 flex-1" }, [
          View({ class: "flex flex-wrap items-center gap-1.5" }, [
            For({
              each: computed(data_, previewBadges),
              render(label) {
                return PreviewBadge(label);
              },
            }),
          ]),
          View(
            {
              class:
                "mt-2 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
            },
            [computed(data_, previewTitle)],
          ),
          View({ class: "mt-1 flex min-w-0 items-center gap-1.5" }, [
            Show({
              when: computed(data_, contentAuthorAvatar),
              ok() {
                return View(
                  {
                    class:
                      "h-5 w-5 shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
                  },
                  [
                    ProxyImg({
                      class: "h-full w-full object-cover",
                      src: computed(data_, contentAuthorAvatar),
                      alt: computed(data_, (content) =>
                        contentAuthorName(content),
                      ),
                    }),
                  ],
                );
              },
            }),
            View(
              {
                class:
                  "min-w-0 truncate text-xs font-medium text-zinc-600 dark:text-zinc-300",
              },
              [computed(data_, (content) => contentAuthorName(content) || "-")],
            ),
          ]),
        ]),
      ]),
      Show({
        when: computed(data_, (content) => !!previewDescription(content)),
        ok() {
          return View(
            {
              class:
                "mt-3 line-clamp-3 text-xs leading-relaxed text-zinc-600 dark:text-zinc-300",
            },
            [
              computed(data_, (content) =>
                shortText(previewDescription(content), 220),
              ),
            ],
          );
        },
      }),
      ContentPreviewMediaStrip(data_),
      TypeSpecificPreview(data_),
      ContentPreviewStats(data_),
      ContentPreviewTags(data_),
      PreviewInfoGrid([
        PreviewInfoItem("内容 ID", data_, (content) => content.content_id),
        PreviewInfoItem("发布", data_, (content) =>
          formatTimestamp(content.publish_time),
        ),
        PreviewInfoItem("更新", data_, (content) =>
          formatTimestamp(content.update_time),
        ),
        PreviewInfoItem("链接", data_, (content) =>
          firstNonEmpty(content.canonical_url, content.source_url, content.url),
        ),
      ]),
    ],
  );
}

function PlatformCreatePanel(vm$) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
        View({ class: "flex items-center gap-2" }, [
          Icon({ name: "download", size: 18 }),
          View(
            {
              class: "text-base font-semibold text-zinc-950 dark:text-zinc-50",
            },
            ["新建下载"],
          ),
        ]),
        Show({
          when: vm$.state.createLoading,
          ok() {
            return View({ class: "text-xs text-zinc-500 dark:text-zinc-400" }, [
              "解析中...",
            ]);
          },
        }),
      ]),
      View({ class: "mt-3 flex flex-col gap-3 lg:flex-row" }, [
        View({ class: "min-w-0 flex-1" }, [
          Input({ store: vm$.ui.createUrlInput }),
        ]),
        Button(
          {
            store: vm$.ui.btnCreatePlatformTask,
          },
          [
            Icon({ name: "play", size: 16 }),
            computed(vm$.state.createCreating, (v) =>
              v ? "创建中..." : "开始下载",
            ),
          ],
        ),
      ]),
      Show({
        when: vm$.state.createError,
        ok() {
          return View(
            {
              class:
                "mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
            },
            [vm$.state.createError],
          );
        },
      }),
      Show({
        when: vm$.state.createProbe,
        ok() {
          return View(
            {
              class:
                "mt-4 grid gap-4 border-t border-zinc-100 pt-4 dark:border-zinc-800 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]",
            },
            [
              View({ class: "min-w-0" }, [
                Show({
                  when: computed(
                    vm$.state.createExisting,
                    (list) => Array.isArray(list) && list.length > 0,
                  ),
                  ok() {
                    return View(
                      {
                        class:
                          "mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
                      },
                      [
                        computed(vm$.state.createExisting, (list) =>
                          existingTaskText(list),
                        ),
                      ],
                    );
                  },
                }),
                ContentPreview({
                  content: vm$.state.createContent,
                  probe: vm$.state.createProbe,
                  raw: vm$.state.createProbeRaw,
                }),
              ]),
              View({ class: "grid gap-3 sm:grid-cols-2 lg:grid-cols-1" }, [
                View({}, [
                  Label(
                    {
                      class:
                        "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                    },
                    ["下载内容"],
                  ),
                  Select({ store: vm$.ui.variantSelect }),
                ]),
                View({}, [
                  Label(
                    {
                      class:
                        "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                    },
                    ["文件名"],
                  ),
                  Input({ store: vm$.ui.filenameInput }),
                ]),
              ]),
              View(
                {
                  class:
                    "lg:col-span-2 overflow-auto rounded-md border border-zinc-200 bg-zinc-50 p-3 text-xs text-zinc-700 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-200",
                },
                [
                  View(
                    {
                      class:
                        "mb-2 font-medium text-zinc-500 dark:text-zinc-400",
                    },
                    ["预请求 JSON"],
                  ),
                  View(
                    {
                      as: "pre",
                      class:
                        "max-h-80 whitespace-pre-wrap break-words font-mono leading-relaxed",
                    },
                    [
                      computed(vm$.state.createProbeRaw, (value) =>
                        formatJSON(value),
                      ),
                    ],
                  ),
                ],
              ),
            ],
          );
        },
      }),
    ],
  );
}

/**
 * @param {ViewComponentProps} props
 */
export default function DownloadsPageView(props) {
  const vm$ = DownloadsPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
      onUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View(
        {
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({}, [
              View(
                {
                  class:
                    "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["下载列表"],
              ),
              View({ class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" }, [
                "管理视频号下载任务和本地文件",
              ]),
            ]),
            Button(
              {
                store: vm$.ui.btn_refresh$,
              },
              [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
            ),
          ]),
          View({ class: "mt-5 grid gap-3 md:grid-cols-4 xl:grid-cols-6" }, [
            HeaderStat({
              label: "任务总数",
              value: computed(vm$.state.statusStats, (v) => {
                return String(v.total || 0);
              }),
              icon: "hard-drive",
            }),
            HeaderStat({
              label: "下载中",
              value: computed(vm$.state.statusStats, (v) => {
                return String(v.running || 0);
              }),
              icon: "activity",
            }),
            HeaderStat({
              label: "总速度",
              value: vm$.state.totalSpeed,
              icon: "gauge",
              class: "md:col-span-2 xl:col-span-4",
            }),
          ]),
          View({ class: "mt-4 flex flex-wrap gap-2" }, [
            For({
              each: vm$.state.tabs,
              render(tab) {
                return View(
                  {
                    class: computed(vm$.state.activeTab, (v) => {
                      const active = v === tab.value;
                      return active
                        ? "flex cursor-pointer items-center gap-2 rounded-md bg-zinc-900 px-3 py-1.5 text-sm text-white dark:bg-zinc-100 dark:text-zinc-900"
                        : "flex cursor-pointer items-center gap-2 rounded-md border border-zinc-200 px-3 py-1.5 text-sm text-zinc-600 hover:bg-zinc-100 dark:border-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-800";
                    }),
                    onClick() {
                      vm$.methods.filter(tab.value);
                    },
                  },
                  [
                    tab.label,
                    View(
                      {
                        class: computed(vm$.state.activeTab, (activeTab) => {
                          const active = activeTab === tab.value;
                          return active
                            ? "min-w-5 rounded-full bg-white/20 px-1.5 text-center text-xs font-semibold"
                            : "min-w-5 rounded-full bg-zinc-100 px-1.5 text-center text-xs font-semibold text-zinc-500 dark:bg-zinc-800 dark:text-zinc-300";
                        }),
                      },
                      [
                        computed(vm$.state.statusStats, (stats) => {
                          return String(countForTab(stats, tab));
                        }),
                      ],
                    ),
                  ],
                );
              },
            }),
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "space-y-3 p-6" }, [
          PlatformCreatePanel(vm$),
          RemoteServerPanel(vm$),
          Show({
            when: computed(vm$.state.error, (t) => !!t),
            ok() {
              return View(
                {
                  class:
                    "rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.error],
              );
            },
          }),
          Show({
            when: computed(vm$.state.tasks, (list) => list.length === 0),
            ok() {
              return View(
                {
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [
                  Icon({ name: "inbox", size: 36 }),
                  computed(vm$.state.loading, (loading) => {
                    return loading ? "加载中..." : "暂无下载任务";
                  }),
                ],
              );
            },
            else() {
              return View({ class: "space-y-3" }, [
                For({
                  each: vm$.state.tasks,
                  render(task) {
                    return TaskCard(task, vm$);
                  },
                }),
                Show({
                  when: computed(vm$.state.noMore, (v) => !v),
                  ok() {
                    return View({ class: "flex justify-center py-4" }, [
                      Button(
                        {
                          store: vm$.ui.btn_load_more$,
                        },
                        [
                          computed(vm$.state.loading, (v) => {
                            return v ? "加载中..." : "加载更多";
                          }),
                        ],
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

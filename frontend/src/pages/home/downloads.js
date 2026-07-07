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
      dataset: {
        t: "home-downloads-page-header-stat-label-value-icon-icon-value-value-value",
      },
      class: [
        "rounded-lg border border-zinc-200 bg-white p-4 ",
        "dark:border-zinc-800 dark:bg-zinc-950",
        cls,
      ].join(" "),
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-header-stat-row-label-value-icon-icon-value",
          },
          class: "flex items-center justify-between gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-header-stat-label-value-text",
              },
              class: "truncate text-sm text-zinc-500 dark:text-zinc-400",
            },
            [label],
          ),
          Icon({ name: icon, size: 18 }),
        ],
      ),
      View(
        {
          dataset: { t: "home-downloads-page-header-stat-value-value-heading" },
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
    status === DownloadTaskStatus.Error
  );
}

function isPausableStatus(status) {
  return (
    status === DownloadTaskStatus.Running ||
    status === DownloadTaskStatus.Wait ||
    status === DownloadTaskStatus.Ready
  );
}

function hasRetryableSubtasks(task) {
  const problemCount = Number(
    task.file_problem_count || task.file_error_count || 0,
  );
  if (problemCount > 0) return true;
  return (
    Number(task.file_count || 0) > 1 &&
    (task.status === DownloadTaskStatus.Error ||
      task.status === DownloadTaskStatus.Paused)
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

function filesContainHTML(files) {
  if (!Array.isArray(files)) return false;
  return files.some((file) => {
    if (filesContainHTML(file.children || file.Children)) return true;
    const name = String(file.path || file.Path || file.name || file.Name || "");
    return /\.(html|htm)$/i.test(name);
  });
}

function isHTMLTask(task) {
  const metadata = parseTaskJSON(task.metadata || task.Metadata);
  const metadata2 = parseTaskJSON(task.metadata2 || task.Metadata2);
  const taskMetadata = Object.keys(metadata).length ? metadata : metadata2;
  const labels = parseTaskJSON(
    task.labels || task.Labels || task.extra || task.Extra,
  );
  const contentType = String(
    task.content_type ||
      task.contentType ||
      task.mime_type ||
      task.mimeType ||
      taskMetadata.content_type ||
      taskMetadata.contentType ||
      taskMetadata.mime_type ||
      taskMetadata.mimeType ||
      taskMetadata.summary?.content_type ||
      taskMetadata.summary?.contentType ||
      labels.content_type ||
      labels.contentType ||
      labels.mime_type ||
      labels.mimeType ||
      "",
  )
    .trim()
    .toLowerCase();
  if (contentType === "html" || contentType === "text/html") return true;
  if (
    filesContainHTML(taskMetadata.files || taskMetadata.Files || task.files)
  ) {
    return true;
  }

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
      dataset: {
        t: "home-downloads-page-download-info-item-label-value-value-value",
      },
      class: classNames([
        "flex min-w-0 items-center gap-1 text-xs text-zinc-500 dark:text-zinc-400",
        cls,
      ]),
    },
    [
      View(
        {
          dataset: { t: "home-downloads-page-download-info-item-label-value" },
          class: "shrink-0 text-zinc-400 dark:text-zinc-500",
        },
        [label],
      ),
      View(
        {
          dataset: { t: "home-downloads-page-download-info-item-value-value" },
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
    {
      dataset: {
        t: "home-downloads-page-download-info-divider-vertical-separator",
      },
      class: "hidden h-3 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800 sm:block",
    },
    [],
  );
}

function DownloadInfoBar(task) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-download-info-bar-panel-computed-value-avatar-or-badge-row-download-info-item-download-info-divider-download-info-item-download-info-divider-download-info-item-download-info-divider",
      },
      class:
        "min-w-0 rounded-md border border-zinc-100 bg-zinc-50 px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900/60",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-download-info-bar-percent-and-progress-row",
          },
          class: "flex items-center gap-3",
        },
        [
          View(
            {
              dataset: { t: "home-downloads-page-download-info-bar-heading" },
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
              dataset: {
                t: "home-downloads-page-download-info-bar-progress-track",
              },
              class:
                "h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800",
            },
            [
              View({
                dataset: {
                  t: "home-downloads-page-download-info-bar-progress-fill",
                },
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
        ],
      ),
      View(
        {
          dataset: {
            t: "home-downloads-page-download-info-bar-row-download-info-item-download-info-divider-download-info-item-download-info-divider-download-info-item-download-info-divider",
          },
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

function collectTaskFilePreview(files, out = []) {
  if (!Array.isArray(files) || out.length >= 4) return out;
  for (const file of files) {
    if (out.length >= 4) break;
    const children = Array.isArray(file.children) ? file.children : [];
    if (children.length) {
      collectTaskFilePreview(children, out);
      continue;
    }
    out.push(file.path || file.name);
  }
  return out;
}

function taskFileSummary(task) {
  const count = Number(task.file_count || 0);
  if (count <= 1) return "";
  const preview = collectTaskFilePreview(task.files || [])
    .filter(Boolean)
    .slice(0, 3);
  const size = Number(task.size || 0) ? `，${formatBytes(task.size)}` : "";
  const names = preview.length ? `：${preview.join("、")}` : "";
  return `${count} 个文件${size}${names}`;
}

function normalizeTreePath(path) {
  return String(path || "")
    .replace(/\\/g, "/")
    .replace(/\/+/g, "/")
    .trim();
}

function pathDirname(path) {
  const value = normalizeTreePath(path);
  const idx = value.lastIndexOf("/");
  if (idx <= 0) return "";
  return value.slice(0, idx);
}

function commonPathRoot(paths) {
  const dirs = paths.map(pathDirname).filter(Boolean);
  if (!dirs.length) return "";
  const first = dirs[0].split("/").filter(Boolean);
  let end = first.length;
  for (const dir of dirs.slice(1)) {
    const parts = dir.split("/").filter(Boolean);
    end = Math.min(end, parts.length);
    for (let i = 0; i < end; i += 1) {
      if (first[i] !== parts[i]) {
        end = i;
        break;
      }
    }
  }
  if (end <= 0) return "";
  const prefix = first.slice(0, end).join("/");
  return dirs[0].startsWith("/") ? `/${prefix}` : prefix;
}

function relativeTreePath(path, root) {
  const value = normalizeTreePath(path);
  const prefix = normalizeTreePath(root);
  if (!prefix || value === prefix)
    return value.split("/").filter(Boolean).pop() || value;
  if (value.startsWith(`${prefix}/`)) return value.slice(prefix.length + 1);
  return value;
}

function pathBasename(path) {
  const value = normalizeTreePath(path);
  if (!value || value === "/") return value || "输出目录";
  const parts = value.split("/").filter(Boolean);
  return parts[parts.length - 1] || value;
}

function fileNodeTreePath(file, parentPath = "") {
  const prefix = normalizeTreePath(parentPath);
  const raw = normalizeTreePath(
    file.path || file.name || file.output_path || "",
  );
  if (!prefix) return raw;
  if (!raw) return prefix;
  if (raw.startsWith("/") || raw === prefix || raw.startsWith(`${prefix}/`)) {
    return raw;
  }
  return `${prefix}/${raw}`;
}

function collectFileTreeLeaves(files, out = [], parentPath = "") {
  if (!Array.isArray(files)) return out;
  for (const file of files) {
    const children = Array.isArray(file.children) ? file.children : [];
    const treePath = fileNodeTreePath(file, parentPath);
    if (children.length) {
      collectFileTreeLeaves(children, out, treePath);
      continue;
    }
    out.push({
      kind: "file",
      title: file.name || file.path || file.output_path || "文件",
      tree_path: treePath || file.name || "文件",
      output_path: file.output_path || "",
      status: file.status,
      status_text: file.status === "error" ? "失败" : file.status || "",
      error: file.error || "",
      size: Number(file.size || 0),
    });
  }
  return out;
}

function taskTreeLeaves(task) {
  const leaves = [];
  const subtasks = Array.isArray(task.subtasks) ? task.subtasks : [];
  for (const subtask of subtasks) {
    const filepath = subtask.filepath || subtask.Filepath || "";
    const outputPath =
      subtask.output_path ||
      subtask.outputPath ||
      subtask.OutputPath ||
      filepath ||
      subtask.title ||
      "";
    leaves.push({
      kind: "task",
      task: subtask,
      id: subtask.id,
      title: subtask.title || subtask.name || subtask.task_id || "子任务",
      tree_path: filepath || subtask.title || "",
      output_path: outputPath,
      status: subtask.status,
      status_text: subtask.status_text || "",
      error: subtask.error || "",
      size: Number(subtask.size || 0),
    });
  }
  if (!leaves.length) collectFileTreeLeaves(task.files || [], leaves);
  return leaves;
}

function insertTreeLeaf(root, parts, leaf) {
  if (!parts.length) return;
  const [name, ...rest] = parts;
  if (!rest.length) {
    root.children.push({ type: "leaf", name, leaf, children: [] });
    return;
  }
  let dir = root.children.find(
    (item) => item.type === "dir" && item.name === name,
  );
  if (!dir) {
    dir = { type: "dir", name, children: [] };
    root.children.push(dir);
  }
  insertTreeLeaf(dir, rest, leaf);
}

function buildSubtaskTree(task) {
  const leaves = taskTreeLeaves(task);
  const hasFileTree = leaves.some((leaf) => leaf.kind === "file");
  const paths = leaves.map((leaf) => leaf.tree_path).filter(Boolean);
  const rootPath = hasFileTree ? "" : commonPathRoot(paths);
  const root = {
    type: "dir",
    name: hasFileTree
      ? "输出目录"
      : pathBasename(
          rootPath || task.output_path || task.filepath || "输出目录",
        ),
    path: hasFileTree ? "" : rootPath,
    children: [],
  };
  for (const leaf of leaves) {
    const displayPath = relativeTreePath(
      leaf.tree_path || leaf.title || "子任务",
      rootPath,
    );
    const parts = displayPath.split("/").filter(Boolean);
    insertTreeLeaf(root, parts.length ? parts : [leaf.title || "子任务"], leaf);
  }
  sortTreeNodes(root.children);
  if (!root.children.length) return [];
  return [root];
}

function sortTreeNodes(nodes) {
  nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === "dir" ? -1 : 1;
    return String(a.name || "").localeCompare(
      String(b.name || ""),
      "zh-Hans-CN",
    );
  });
  for (const node of nodes) {
    if (Array.isArray(node.children)) sortTreeNodes(node.children);
  }
}

function treeStatusClassName(status) {
  if (status === DownloadTaskStatus.Error || status === "error") {
    return "text-red-600 dark:text-red-300";
  }
  if (status === DownloadTaskStatus.Done || status === "done") {
    return "text-emerald-600 dark:text-emerald-300";
  }
  if (
    status === DownloadTaskStatus.Paused ||
    status === "paused" ||
    status === "pause"
  ) {
    return "text-zinc-600 dark:text-zinc-300";
  }
  return "text-zinc-500 dark:text-zinc-400";
}

function taskViewKey(task) {
  const id = Number(task?.id || 0);
  if (Number.isFinite(id) && id > 0) return String(id);
  return String(task?.task_id || task?.task_uid || "");
}

function taskSubtaskCount(task) {
  const subtasks = Array.isArray(task.subtasks) ? task.subtasks : [];
  return Number(
    task.subtask_count || task.subtaskCount || subtasks.length || 0,
  );
}

function shouldShowSubtaskToggle(task) {
  return taskSubtaskCount(task) > 0 || Number(task.file_count || 0) > 1;
}

function SubtaskTreeNode(node, vm$, level = 0) {
  const isDir = node.type === "dir";
  const indent = `${Math.min(level * 18, 108)}px`;
  const branchClass =
    level > 0
      ? "relative before:absolute before:left-0 before:top-0 before:h-full before:border-l before:border-zinc-200 dark:before:border-zinc-800"
      : "";
  if (isDir) {
    return View(
      {
        dataset: {
          t: "home-downloads-page-subtask-tree-node-details-node-icon-folder-closed-node-name",
        },
        as: "details",
        open: true,
        class: `space-y-1 ${branchClass}`,
      },
      [
        View(
          {
            dataset: {
              t: "home-downloads-page-subtask-tree-node-details-summary-row-icon-folder-closed-node-name-text",
            },
            as: "summary",
            class:
              "flex min-w-0 cursor-pointer select-none items-center gap-1.5 rounded-md py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50 dark:text-zinc-200 dark:hover:bg-zinc-900",
            style: { "padding-left": indent },
          },
          [
            Icon({ name: "folder-closed", size: 14 }),
            View(
              {
                dataset: {
                  t: "home-downloads-page-subtask-tree-node-row-node-name",
                },
                class: "min-w-0 flex-1 truncate",
              },
              [node.name],
            ),
          ],
        ),
        ...node.children.map((child) => SubtaskTreeNode(child, vm$, level + 1)),
      ],
    );
  }
  const leaf = node.leaf || {};
  const leafTask = leaf.task;
  const failed =
    leaf.status === DownloadTaskStatus.Error || leaf.status === "error";
  return View(
    {
      dataset: { t: "home-downloads-page-subtask-tree-file-node-row" },
      class: `flex min-w-0 items-center gap-2 rounded-md py-1 pr-2 text-xs hover:bg-zinc-50 dark:hover:bg-zinc-900 ${branchClass}`,
      style: { "padding-left": indent },
    },
    [
      Icon({ name: "file", size: 14 }),
      View(
        {
          dataset: {
            t: "home-downloads-page-subtask-tree-node-row-node-name-leaf-title-leaf-error",
          },
          class: "min-w-0 flex-1",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-subtask-tree-node-node-name-leaf-title",
              },
              class: "truncate text-zinc-700 dark:text-zinc-200",
            },
            [node.name || leaf.title],
          ),
          leaf.error
            ? View(
                {
                  dataset: {
                    t: "home-downloads-page-subtask-tree-node-error-leaf-error",
                  },
                  class: "truncate text-[11px] text-red-500 dark:text-red-300",
                },
                [leaf.error],
              )
            : null,
        ],
      ),
      View(
        {
          dataset: {
            t: "home-downloads-page-subtask-tree-node-leaf-status_text",
          },
          class: `shrink-0 whitespace-nowrap ${treeStatusClassName(leaf.status)}`,
        },
        [leaf.status_text || ""],
      ),
      leaf.size
        ? View(
            {
              dataset: {
                t: "home-downloads-page-subtask-tree-node-format-bytes",
              },
              class: "shrink-0 text-zinc-500 dark:text-zinc-400",
            },
            [formatBytes(leaf.size)],
          )
        : null,
      failed && leafTask
        ? Button(
            {
              store: new Timeless.ui.ButtonCore({
                variant: "outline",
                size: "sm",
                onClick() {
                  vm$.methods.retry(leafTask);
                },
              }),
            },
            ["重试"],
          )
        : null,
      leafTask
        ? Button(
            {
              store: new Timeless.ui.ButtonCore({
                variant: "ghost",
                size: "sm",
                onClick() {
                  vm$.methods.remove(leafTask, false);
                },
              }),
            },
            ["删除"],
          )
        : null,
    ].filter(Boolean),
  );
}

function SubtaskTree(task, vm$) {
  const key = taskViewKey(task);
  return View(
    {
      dataset: {
        t: "home-downloads-page-subtask-tree-panel-icon-folder-tree-子任务-task-subtask-count-button",
      },
      class:
        "mt-3 rounded-md border border-zinc-200 bg-zinc-50/60 p-2 dark:border-zinc-800 dark:bg-zinc-950/60",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-subtask-tree-row-icon-folder-tree-子任务-task-subtask-count-button-text",
          },
          class:
            "mb-1 flex items-center justify-between gap-2 text-xs font-medium text-zinc-600 dark:text-zinc-300",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-subtask-tree-row-icon-folder-tree-子任务-task-subtask-count",
              },
              class: "flex min-w-0 items-center gap-1.5",
            },
            [
              Icon({ name: "folder-tree", size: 14 }),
              computed(task, (t) => `子任务 ${taskSubtaskCount(t) || ""}`),
            ],
          ),
          Button(
            {
              store: new Timeless.ui.ButtonCore({
                variant: "ghost",
                size: "sm",
                onClick() {
                  vm$.methods.toggleSubtasks(task);
                },
              }),
            },
            [
              Icon({
                name: computed(vm$.state.expandedTaskIDs, (ids) =>
                  ids?.[key] ? "chevron-up" : "chevron-down",
                ),
                size: 14,
              }),
            ],
          ),
        ],
      ),
      Show({
        when: computed(vm$.state.expandedTaskIDs, (ids) => !!ids?.[key]),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-subtask-tree-stack-build-subtask-tree-list",
              },
              class: "space-y-1",
            },
            [
              Show({
                when: computed(
                  vm$.state.loadingSubtaskIDs,
                  (ids) => !!ids?.[key],
                ),
                ok() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-subtask-tree-row-icon-loader-2-加载子任务-text",
                      },
                      class:
                        "flex items-center gap-1.5 px-1 py-2 text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [Icon({ name: "loader-2", size: 14 }), "加载子任务..."],
                  );
                },
              }),
              Show({
                when: computed(vm$.state.subtaskErrors, (errors) =>
                  Boolean(errors?.[key]),
                ),
                ok() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-subtask-tree-error-row-icon-circle-alert-errors-key-text",
                      },
                      class:
                        "flex items-center gap-1.5 px-1 py-2 text-xs text-red-600 dark:text-red-300",
                    },
                    [
                      Icon({ name: "circle-alert", size: 14 }),
                      computed(
                        vm$.state.subtaskErrors,
                        (errors) => errors?.[key],
                      ),
                    ],
                  );
                },
              }),
              For({
                each: computed(task, buildSubtaskTree),
                render(node) {
                  return SubtaskTreeNode(node, vm$);
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function SubtaskToggle(task, vm$) {
  const key = taskViewKey(task);
  return View(
    {
      dataset: { t: "home-downloads-page-subtask-toggle-row-button" },
      class: "mt-2 flex min-w-0 items-center gap-2",
    },
    [
      Button(
        {
          store: new Timeless.ui.ButtonCore({
            variant: "outline",
            size: "sm",
            onClick() {
              vm$.methods.toggleSubtasks(task);
            },
          }),
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-subtask-toggle-row-icon-chevron-up-or-folder-tree-收起子任务-or-查看子任务-computed-value",
              },
              class: "flex items-center gap-1.5",
            },
            [
              Icon({
                name: computed(vm$.state.expandedTaskIDs, (ids) =>
                  ids?.[key] ? "chevron-up" : "folder-tree",
                ),
                size: 14,
              }),
              computed(vm$.state.expandedTaskIDs, (ids) =>
                ids?.[key] ? "收起子任务" : "查看子任务",
              ),
              computed(task, (t) => {
                const count = taskSubtaskCount(t);
                return count ? `(${count})` : "";
              }),
            ],
          ),
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
      dataset: {
        t: "home-downloads-page-task-card-card-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-download-info-bar-button-checkbox-label",
      },
      class:
        "group rounded-lg border border-zinc-200 bg-white p-4 shadow-sm transition hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-950 dark:hover:border-zinc-700",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-task-card-row-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-download-info-bar-button-checkbox-label",
          },
          class: "flex flex-col gap-4",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-task-card-grid-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-download-info-bar-button-checkbox-label",
              },
              class: "grid min-w-0 gap-4",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-task-card-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text",
                  },
                  class: "min-w-0",
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-task-card-row-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text",
                      },
                      class: "flex items-start gap-3",
                    },
                    [
                      Show({
                        when: computed(
                          task,
                          (t) => t.display_cover_url || t.cover_url,
                        ),
                        ok() {
                          return View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-cover-media-image",
                              },
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
                                sourceURL: computed(
                                  task,
                                  (t) =>
                                    t.source_uri ||
                                    t.source_url ||
                                    t.canonical_url ||
                                    t.url,
                                ),
                              }),
                            ],
                          );
                        },
                      }),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-task-card-row-task-title-task-name-task-task_id-未命名任务-task-output_path-task-filepath-task-url",
                          },
                          class: "min-w-0 flex-1",
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-task-title-task-name-task-task_id-未命名任务-heading",
                              },
                              class:
                                "truncate text-base font-semibold text-zinc-950 dark:text-zinc-50",
                              // title: task.title || task.task_id,
                            },
                            [
                              task.title ||
                                task.name ||
                                task.task_id ||
                                "未命名任务",
                            ],
                          ),
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-task-output_path-task-filepath-task-url-text",
                              },
                              class:
                                "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                            },
                            [
                              task.output_path ||
                                task.filepath ||
                                task.url ||
                                "-",
                            ],
                          ),
                          Show({
                            when: computed(
                              task,
                              (t) => Number(t.file_count || 0) > 1,
                            ),
                            ok() {
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-task-card-row-icon-folder-tree-task-file-summary-text",
                                  },
                                  class:
                                    "mt-2 flex min-w-0 items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400",
                                },
                                [
                                  Icon({ name: "folder-tree", size: 14 }),
                                  View(
                                    {
                                      dataset: {
                                        t: "home-downloads-page-task-card-task-file-summary",
                                      },
                                      class: "min-w-0 truncate",
                                    },
                                    [computed(task, taskFileSummary)],
                                  ),
                                ],
                              );
                            },
                          }),
                          Show({
                            when: computed(
                              task,
                              (t) => Number(t.file_error_count || 0) > 0,
                            ),
                            ok() {
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-task-card-error-row-icon-circle-alert-computed-value-text",
                                  },
                                  class:
                                    "mt-2 flex min-w-0 items-center gap-1.5 text-xs text-red-600 dark:text-red-300",
                                },
                                [
                                  Icon({ name: "circle-alert", size: 14 }),
                                  View(
                                    {
                                      dataset: {
                                        t: "home-downloads-page-task-card-file-error-summary-text",
                                      },
                                      class: "min-w-0 truncate",
                                    },
                                    [
                                      computed(task, (t) => {
                                        const first = t.file_error || {};
                                        const count = Number(
                                          t.file_error_count || 0,
                                        );
                                        const path = first.path || "文件";
                                        const error = first.error || "下载失败";
                                        return `${count} 个文件失败：${path}，${error}`;
                                      }),
                                    ],
                                  ),
                                ],
                              );
                            },
                          }),
                          Show({
                            when: computed(task, shouldShowSubtaskToggle),
                            ok() {
                              return SubtaskToggle(task, vm$);
                            },
                          }),
                          Show({
                            when: computed(
                              vm$.state.expandedTaskIDs,
                              (ids) => !!ids?.[taskViewKey(task)],
                            ),
                            ok() {
                              return SubtaskTree(task, vm$);
                            },
                          }),
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-container-2",
                              },
                              class: "mt-2",
                            },
                            [
                              Show({
                                when: computed(task, (t) =>
                                  isPlayableStatus(t.status),
                                ),
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
                            ],
                          ),
                        ],
                      ),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-task-card-t-status_text",
                          },
                          class: classNames([
                            "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                            computed(task, (t) => mapStatusClassName(t.status)),
                          ]),
                        },
                        [computed(task, (t) => t.status_text)],
                      ),
                    ],
                  ),
                ],
              ),
              DownloadInfoBar(task),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-task-card-row-button-checkbox-label",
                  },
                  class: "flex shrink-0 flex-wrap items-center gap-2",
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
                    when: computed(task, hasRetryableSubtasks),
                    ok() {
                      return Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            size: "sm",
                            onClick() {
                              vm$.methods.retryChildren(task);
                            },
                          }),
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-row-icon-refresh-cw-重试子任务",
                              },
                              class: "flex items-center justify-center gap-1.5",
                            },
                            [
                              Icon({ name: "refresh-cw", size: 14 }),
                              "重试子任务",
                            ],
                          ),
                        ],
                      );
                    },
                  }),
                  Show({
                    when: computed(task, (t) => isPausableStatus(t.status)),
                    ok() {
                      return Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            size: "sm",
                            onClick() {
                              vm$.methods.pause(task);
                            },
                          }),
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-row-icon-pause-暂停",
                              },
                              class: "flex items-center justify-center gap-1.5",
                            },
                            [Icon({ name: "pause", size: 14 }), "暂停"],
                          ),
                        ],
                      );
                    },
                  }),
                  Show({
                    when: computed(
                      task,
                      (t) => t.status === DownloadTaskStatus.Paused,
                    ),
                    ok() {
                      return Button(
                        {
                          store: new Timeless.ui.ButtonCore({
                            variant: "outline",
                            size: "sm",
                            onClick() {
                              vm$.methods.resume(task);
                            },
                          }),
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-task-card-row-icon-play-继续",
                              },
                              class: "flex items-center justify-center gap-1.5",
                            },
                            [Icon({ name: "play", size: 14 }), "继续"],
                          ),
                        ],
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
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-task-card-row-checkbox-label",
                      },
                      class: "flex items-center gap-1.5",
                    },
                    [
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
                    ],
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function RemoteTaskCard(task) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-remote-task-card-card-icon-server-task-title-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-avatar-or-badge-t-progress_info-percent-t-size_text-t-speed_text-更新-t-updated_at_text",
      },
      class:
        "rounded-lg border border-sky-200 bg-white p-4 shadow-sm dark:border-sky-900 dark:bg-zinc-950",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-remote-task-card-row-icon-server-task-title-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-avatar-or-badge-t-progress_info-percent-t-size_text-t-speed_text-更新-t-updated_at_text",
          },
          class: "flex gap-4",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-remote-task-card-cover-media-row-icon-server",
              },
              class:
                "flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-300",
            },
            [Icon({ name: "server", size: 24 })],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-remote-task-card-row-task-title-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text-avatar-or-badge-t-progress_info-percent-t-size_text-t-speed_text-更新-t-updated_at_text",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-task-card-row-task-title-task-task_id-未命名任务-task-output_path-task-filepath-task-url-t-status_text",
                  },
                  class: "flex items-start justify-between gap-3",
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-task-card-task-title-task-task_id-未命名任务-task-output_path-task-filepath-task-url",
                      },
                      class: "min-w-0",
                    },
                    [
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-remote-task-card-task-title-task-task_id-未命名任务-heading",
                          },
                          class:
                            "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                          // title: task.title || task.task_id,
                        },
                        [task.title || task.task_id || "未命名任务"],
                      ),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-remote-task-card-task-output_path-task-filepath-task-url-text",
                          },
                          class:
                            "mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400",
                        },
                        [task.output_path || task.filepath || task.url || "-"],
                      ),
                    ],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-task-card-t-status_text",
                      },
                      class: classNames([
                        "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                        computed(task, (t) => mapStatusClassName(t.status)),
                      ]),
                    },
                    [computed(task, (t) => t.status_text)],
                  ),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-task-card-stack-avatar-or-badge-t-progress_info-percent-t-size_text-t-speed_text-更新-t-updated_at_text",
                  },
                  class: "mt-3 space-y-2",
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-task-card-avatar-or-badge",
                      },
                      class:
                        "h-2 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
                    },
                    [
                      View({
                        dataset: {
                          t: "home-downloads-page-remote-task-card-avatar-or-badge-2",
                        },
                        class: "h-full rounded-full bg-sky-600 dark:bg-sky-300",
                        style: {
                          width: computed(
                            task,
                            (t) => `${t.progress_info.percent}%`,
                          ),
                        },
                      }),
                    ],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-task-card-row-t-progress_info-percent-t-size_text-t-speed_text-更新-t-updated_at_text-text",
                      },
                      class:
                        "flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [
                      computed(task, (t) => `${t.progress_info.percent}%`),
                      computed(task, (t) => t.size_text),
                      computed(task, (t) =>
                        t.status === DownloadTaskStatus.Running
                          ? t.speed_text
                          : "",
                      ),
                      "更新",
                      computed(task, (t) => t.updated_at_text),
                    ],
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function RemoteServerPanel(vm$) {
  return Show({
    when: vm$.state.remoteEnabled,
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-remote-server-panel-card-stack-icon-server-RemoteServer-vm-state-remote-label-header-stat-header-stat-header-stat",
          },
          class:
            "space-y-3 rounded-lg border border-sky-200 bg-sky-50/50 p-4 dark:border-sky-900 dark:bg-sky-950/20",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-remote-server-panel-row-icon-server-RemoteServer-vm-state-remote-label-header-stat-header-stat-header-stat",
              },
              class: "flex flex-wrap items-center justify-between gap-3",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-server-panel-icon-server-RemoteServer-vm-state-remote-label",
                  },
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-server-panel-row-icon-server-RemoteServer-heading",
                      },
                      class:
                        "flex items-center gap-2 text-base font-semibold text-zinc-950 dark:text-zinc-50",
                    },
                    [Icon({ name: "server", size: 18 }), "RemoteServer"],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-remote-server-panel-vm-state-remote-label-text",
                      },
                      class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400",
                    },
                    [vm$.state.remoteLabel],
                  ),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-server-panel-grid-header-stat-header-stat-header-stat",
                  },
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
            ],
          ),
          Show({
            when: vm$.state.remoteError,
            ok() {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-server-panel-error-card-vm-state-remote-error-text",
                  },
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
                  dataset: {
                    t: "home-downloads-page-remote-server-panel-row-icon-inbox-远端加载中-or-暂无远端下载任务",
                  },
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
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-remote-server-panel-stack-vm-state-remote-tasks-list",
                  },
                  class: "space-y-3",
                },
                [
                  For({
                    each: vm$.state.remoteTasks,
                    render(task) {
                      return RemoteTaskCard(task);
                    },
                  }),
                  Show({
                    when: computed(vm$.state.remoteNoMore, (v) => !v),
                    ok() {
                      return View(
                        {
                          dataset: {
                            t: "home-downloads-page-remote-server-panel-row-button",
                          },
                          class: "flex justify-center py-2",
                        },
                        [
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
                        ],
                      );
                    },
                  }),
                ],
              );
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
  merged.raw_data = firstNonEmpty(merged.raw_data, envelope?.data);
  merged.warnings = [
    ...asArray(merged.warnings),
    ...asArray(probe?.warnings),
    ...asArray(raw?.warnings),
  ].filter(Boolean);
  return merged;
}

function normalizeProbePipeline(raw) {
  const workflowNodes = raw?.workflow?.nodes || raw?.workflow?.Nodes;
  const probeNodes =
    raw?.probe_pipeline || raw?.probePipeline || raw?.ProbePipeline;
  const fallbackNodes = raw?.pipeline || [];
  const nodes = [
    ...(Array.isArray(workflowNodes) ? workflowNodes : []),
    ...(Array.isArray(probeNodes) ? probeNodes : []),
  ];
  if (nodes.length > 0) return nodes;
  if (Array.isArray(fallbackNodes) && fallbackNodes.length > 0) {
    return fallbackNodes;
  }
  const workflow = raw?.workflow || raw?.Workflow;
  if (!workflow) return [];
  const currentNode = firstNonEmpty(
    workflow.current_node,
    workflow.currentNode,
    workflow.CurrentNode,
  );
  return [
    {
      id:
        currentNode && currentNode !== "start" ? currentNode : "match_platform",
      type:
        currentNode && currentNode !== "start" ? currentNode : "match_platform",
      status: workflow.status || workflow.Status || "running",
    },
  ];
}

function pipelineNodeOutput(node) {
  return node?.output || node?.Output || {};
}

function pipelineNodeID(node) {
  return String(
    firstNonEmpty(node?.id, node?.ID, node?.name, node?.Name),
  ).trim();
}

function pipelineNodeType(node) {
  return String(
    firstNonEmpty(node?.type, node?.Type, pipelineNodeID(node)),
  ).trim();
}

function pipelineNodeStatus(node) {
  return String(node?.status || node?.Status || "completed")
    .trim()
    .toLowerCase();
}

function pipelineNodeDefinition(node) {
  const id = pipelineNodeID(node);
  const type = pipelineNodeType(node);
  const key = type || id;
  const map = {
    match_platform: {
      label: "识别链接平台",
      running: "正在匹配支持的平台",
      completed: "已识别链接所属平台",
    },
    probe: {
      label: "解析内容信息",
      running: "正在抓取页面并读取标题、作者、封面等信息",
      completed: "内容信息解析完成",
    },
    check_existing: {
      label: "检查已有任务",
      running: "正在检查是否已经创建过下载任务",
      completed: "重复任务检查完成",
    },
    user_confirmation: {
      label: "确认下载内容",
      running: "正在准备下载选项",
      waiting: "请确认下载内容、格式和文件名",
      completed: "下载配置已确认",
    },
    pause_after_probe: {
      label: "确认下载内容",
      running: "正在准备下载选项",
      waiting: "请确认下载内容、格式和文件名",
      completed: "下载配置已确认",
    },
    resume: {
      label: "继续创建任务",
      running: "正在提交已确认的下载配置",
      completed: "下载配置已提交",
    },
    resume_after_probe: {
      label: "继续创建任务",
      running: "正在提交已确认的下载配置",
      completed: "下载配置已提交",
    },
    resolve: {
      label: "生成下载方案",
      running: "正在根据选择生成下载地址和任务参数",
      completed: "下载方案已生成",
    },
    create_task: {
      label: "创建下载任务",
      running: "正在写入任务并交给下载引擎",
      completed: "下载任务已创建",
    },
    create_account: {
      label: "保存作者信息",
      running: "正在保存作者或账号信息",
      completed: "作者信息已保存",
    },
    create_content: {
      label: "保存内容记录",
      running: "正在保存内容元数据",
      completed: "内容记录已保存",
    },
    download_asset: {
      label: "下载文件",
      running: "正在下载目标文件",
      completed: "文件下载完成",
    },
    download_69shuba_archive: {
      label: "下载小说章节",
      running: "正在下载章节和目录文件",
      completed: "小说章节已下载",
    },
    fetch_html: {
      label: "获取页面 HTML",
      running: "正在请求原始页面",
      completed: "页面 HTML 已获取",
    },
    fetch_full_catalog: {
      label: "获取完整目录",
      running: "正在读取完整章节目录",
      completed: "完整目录已获取",
    },
    local_directory: {
      label: "读取本地文件夹",
      running: "正在读取本地 HTML 文件夹",
      completed: "本地文件夹已读取",
    },
    clean_69shuba_html: {
      label: "整理小说 HTML",
      running: "正在清理页面脚本和广告内容",
      completed: "小说 HTML 已整理",
    },
    sanitize_html: {
      label: "清理 HTML",
      running: "正在清理页面脚本和无关内容",
      completed: "HTML 已清理",
    },
    render_html_template: {
      label: "生成阅读页面",
      running: "正在生成离线阅读页面",
      completed: "阅读页面已生成",
    },
    rewrite_html_assets: {
      label: "整理页面资源",
      running: "正在改写图片和静态资源引用",
      completed: "页面资源已整理",
    },
    render_pdf: {
      label: "生成 PDF",
      running: "正在渲染 PDF 文件",
      completed: "PDF 已生成",
    },
    postprocess: {
      label: "整理下载结果",
      running: "正在整理下载后的文件",
      completed: "下载结果已整理",
    },
    persist_artifacts: {
      label: "保存文件记录",
      running: "正在保存文件清单和任务结果",
      completed: "文件记录已保存",
    },
  };
  return map[key] || map[id] || null;
}

function pipelineNodeLabel(node, index) {
  const n = Number(index);
  const def = pipelineNodeDefinition(node);
  return firstNonEmpty(
    node?.label,
    node?.Label,
    node?.title,
    node?.Title,
    def?.label,
    Number.isFinite(n) ? `步骤 ${n + 1}` : "解析步骤",
  );
}

function pipelineNodeStatusLabel(node) {
  const status = pipelineNodeStatus(node);
  const map = {
    running: "加载中",
    completed: "成功",
    success: "成功",
    done: "成功",
    waiting_user: "待确认",
    paused: "待确认",
    pending: "等待中",
    failed: "失败",
    error: "失败",
  };
  return map[status] || status || "等待中";
}

function pipelineNodeStatusClass(node) {
  const status = pipelineNodeStatus(node);
  if (status === "running") {
    return "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300";
  }
  if (status === "completed" || status === "success" || status === "done") {
    return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300";
  }
  if (status === "failed" || status === "error") {
    return "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300";
  }
  if (status === "waiting_user" || status === "paused") {
    return "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";
  }
  return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
}

function pipelineNodeIconWrapClass(node) {
  const status = pipelineNodeStatus(node);
  if (status === "running") {
    return "bg-blue-50 text-blue-600 ring-blue-100 dark:bg-blue-950 dark:text-blue-300 dark:ring-blue-900";
  }
  if (status === "completed" || status === "success" || status === "done") {
    return "bg-emerald-50 text-emerald-600 ring-emerald-100 dark:bg-emerald-950 dark:text-emerald-300 dark:ring-emerald-900";
  }
  if (status === "failed" || status === "error") {
    return "bg-red-50 text-red-600 ring-red-100 dark:bg-red-950 dark:text-red-300 dark:ring-red-900";
  }
  if (status === "waiting_user" || status === "paused") {
    return "bg-amber-50 text-amber-600 ring-amber-100 dark:bg-amber-950 dark:text-amber-300 dark:ring-amber-900";
  }
  return "bg-zinc-50 text-zinc-500 ring-zinc-200 dark:bg-zinc-900 dark:text-zinc-400 dark:ring-zinc-800";
}

function pipelineNodeIconName(node) {
  const status = pipelineNodeStatus(node);
  if (status === "running") return "loader";
  if (status === "failed" || status === "error") return "circle-alert";
  if (status === "waiting_user" || status === "paused") return "clock";
  if (status === "completed" || status === "success" || status === "done") {
    return "check";
  }
  return "clock";
}

function pipelineNodeIconClass(node) {
  const status = pipelineNodeStatus(node);
  if (status === "running") return "animate-spin";
  return "";
}

function pipelineNodeDescription(node, raw) {
  const status = pipelineNodeStatus(node);
  const def = pipelineNodeDefinition(node);
  const error = firstNonEmpty(node?.error, node?.Error);
  if (status === "failed" || status === "error") {
    return error ? `失败：${error}` : "处理失败，请检查链接或稍后重试";
  }
  if (status === "running") return def?.running || "正在处理当前步骤";
  if (status === "waiting_user" || status === "paused") {
    return def?.waiting || "请确认后继续";
  }
  if (status === "completed" || status === "success" || status === "done") {
    if (pipelineNodeType(node) === "match_platform") {
      const platform = firstNonEmpty(raw?.platform, raw?.workflow?.platform);
      if (platform) return `已识别为 ${platformLabel(platform)}`;
    }
    if (pipelineNodeType(node) === "check_existing") {
      const existing = asArray(raw?.existing || raw?.Existing);
      if (existing.length > 0) return `发现 ${existing.length} 个已有任务`;
      return "没有发现重复任务";
    }
    return def?.completed || "当前步骤已完成";
  }
  return "等待前置步骤完成";
}

function pipelineOutputDetails(out) {
  const parts = [];
  const title = firstNonEmpty(out.title, out.Title);
  const url = firstNonEmpty(out.url, out.URL);
  const chapterCount = Number(out.chapter_count || out.chapterCount || 0);
  const htmlSize = Number(out.html_size || out.htmlSize || 0);
  const fileCount = Number(out.file_count || out.fileCount || 0);
  const size = Number(out.size || out.Size || 0);
  if (title) parts.push(`标题：${title}`);
  if (chapterCount > 0) parts.push(`章节：${chapterCount}`);
  if (fileCount > 0) parts.push(`文件：${fileCount}`);
  if (htmlSize > 0) parts.push(`HTML：${formatBytes(htmlSize)}`);
  if (size > 0) parts.push(`大小：${formatBytes(size)}`);
  if (url) parts.push(`来源：${url}`);
  return parts;
}

function pipelineSelectionDetails(out) {
  const parts = [];
  const variant = firstNonEmpty(out.variant_id, out.variantID, out.VariantID);
  const filename = firstNonEmpty(out.filename, out.Filename);
  const suffix = firstNonEmpty(out.suffix, out.Suffix);
  if (variant) parts.push(`内容：${variant}`);
  if (filename) parts.push(`文件名：${filename}${suffix || ""}`);
  return parts;
}

function pipelineNodeDetails(node) {
  const out = pipelineNodeOutput(node);
  const outputDetails = pipelineOutputDetails(out);
  if (outputDetails.length > 0) return outputDetails;
  return pipelineSelectionDetails(out);
}

function pipelineWorkflowText(raw) {
  const workflow = raw?.workflow || raw?.Workflow || {};
  const status = String(workflow.status || workflow.Status || "").toLowerCase();
  const hasProbe = !!(raw?.probe || raw?.Probe || raw?.content || raw?.Content);
  if (status === "failed" || status === "error") return "解析失败";
  if (hasProbe && (status === "paused" || status === "waiting_user")) {
    return "解析完成，请确认下载内容";
  }
  if (hasProbe) return "解析完成";
  if (status === "completed" || status === "done") return "流程已完成";
  return "正在解析链接";
}

function inferContentType(platform, content) {
  if (platform === "officialaccount" || platform === "wx_official_account") {
    return "article";
  }
  if (platform === "xiaohongshu") return "note";
  if (platform === "weibo") return "post";
  if (platform === "qidian" || platform === "mqidian") return "novel";
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
    novel: "小说",
  };
  return map[type] || type || "内容";
}

function IconMappedType() {
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
    novel() {
      return Icon({
        name: "book-open",
        size: 28,
      });
    },
  };
}

function contentAuthorName(content) {
  const author = content?.author;
  const authorName =
    author && typeof author === "object"
      ? firstNonEmpty(author.nickname, author.name, author.username, author.id)
      : "";
  return firstNonEmpty(
    authorName,
    typeof author === "string" ? author : "",
    content?.author_nickname,
    content?.account_nickname,
    content?.author_username,
    content?.author_id,
  );
}

function contentAuthorAvatar(content) {
  const author = content?.author;
  const raw = rawPreviewData(content);
  const rawAuthor = raw.author || raw.Author || {};
  const authorAvatar =
    author && typeof author === "object"
      ? firstNonEmpty(
          author.avatar,
          author.Avatar,
          author.avatar_url,
          author.avatarUrl,
          author.AvatarURL,
          author.AvatarUrl,
        )
      : "";
  return firstNonEmpty(
    authorAvatar,
    content?.author_avatar,
    content?.author_avatar_url,
    content?.authorAvatar,
    content?.authorAvatarURL,
    content?.authorAvatarUrl,
    content?.AuthorAvatarURL,
    rawAuthor.avatar,
    rawAuthor.Avatar,
    rawAuthor.avatar_url,
    rawAuthor.avatarUrl,
    rawAuthor.AvatarURL,
    rawAuthor.AvatarUrl,
    raw.author_avatar,
    raw.author_avatar_url,
    raw.authorAvatar,
    raw.authorAvatarURL,
    raw.authorAvatarUrl,
    raw.AuthorAvatarURL,
    content?.account_avatar_url,
  );
}

function contentAuthorHomepage(content) {
  const author = content?.author;
  const raw = rawPreviewData(content);
  const rawAuthor = raw.author || raw.Author || {};
  const authorHomepage =
    author && typeof author === "object"
      ? firstNonEmpty(
          author.homepage_url,
          author.homepageUrl,
          author.profile_url,
          author.profileUrl,
          author.channel_url,
          author.channelUrl,
          author.url,
          author.URL,
        )
      : "";
  return firstNonEmpty(
    authorHomepage,
    content?.author_homepage_url,
    content?.authorHomepageUrl,
    content?.author_url,
    content?.authorUrl,
    content?.profile_url,
    content?.profileUrl,
    content?.channel_url,
    content?.channelUrl,
    content?.uploader_url,
    content?.uploaderUrl,
    content?.account_homepage_url,
    content?.accountHomepageUrl,
    content?.account_url,
    content?.accountUrl,
    raw.author_homepage_url,
    raw.authorHomepageUrl,
    raw.author_url,
    raw.authorUrl,
    raw.profile_url,
    raw.profileUrl,
    raw.channel_url,
    raw.channelUrl,
    raw.uploader_url,
    raw.uploaderUrl,
    raw.account_homepage_url,
    raw.accountHomepageUrl,
    raw.account_url,
    raw.accountUrl,
    rawAuthor.homepage_url,
    rawAuthor.homepageUrl,
    rawAuthor.profile_url,
    rawAuthor.profileUrl,
    rawAuthor.channel_url,
    rawAuthor.channelUrl,
    rawAuthor.url,
    rawAuthor.URL,
  );
}

function imageURLOf(image) {
  if (!image) return "";
  if (typeof image === "string") return image;
  return firstNonEmpty(
    image.url,
    image.URL,
    image.src,
    image.Src,
    image.thumbnail_url,
    image.thumbnailUrl,
    image.cover_url,
    image.coverUrl,
    image.CoverURL,
    image.CoverUrl,
  );
}

function contentImages(content) {
  const raw = rawPreviewData(content);
  const images = [
    ...asArray(content?.images),
    ...asArray(content?.Images),
    ...asArray(content?.preview_images),
    ...asArray(content?.previewImages),
    ...asArray(content?.files),
    ...asArray(content?.Files),
    ...asArray(raw.images),
    ...asArray(raw.Images),
    ...asArray(raw.preview_images),
    ...asArray(raw.previewImages),
    ...asArray(raw.files),
    ...asArray(raw.Files),
  ]
    .map(imageURLOf)
    .filter(Boolean);
  if (!images.length && content?.cover_url) return [content.cover_url];
  return [...new Set(images)];
}

function contentCoverURL(content) {
  const raw = rawPreviewData(content);
  return firstNonEmpty(
    content?.cover_url,
    content?.coverUrl,
    content?.CoverURL,
    raw.cover_url,
    raw.coverUrl,
    raw.CoverURL,
    raw.CoverUrl,
    raw.image_url,
    raw.imageUrl,
    raw.ImageURL,
    contentImages(content)[0],
  );
}

function countImages(content) {
  return (
    Number(content?.image_count || 0) ||
    contentImages(content).filter((url) => url !== content?.cover_url).length
  );
}

function firstArray(...values) {
  for (const value of values) {
    if (Array.isArray(value) && value.length > 0) return value;
  }
  return [];
}

function qidianVolumes(content) {
  const raw = rawPreviewData(content);
  return firstArray(
    content?.volumes,
    content?.Volumes,
    raw.volumes,
    raw.Volumes,
  )
    .map((volume, index) => {
      const chapters = firstArray(volume?.chapters, volume?.Chapters).map(
        (chapter, chapterIndex) => ({
          idx: Number(chapter?.idx || chapter?.Idx || chapterIndex + 1),
          title: firstNonEmpty(chapter?.title, chapter?.Title),
          url: firstNonEmpty(chapter?.url, chapter?.URL),
          locked: !!firstNonEmpty(chapter?.locked, chapter?.Locked),
        }),
      );
      return {
        idx: Number(volume?.idx || volume?.Idx || index + 1),
        title: firstNonEmpty(
          volume?.title,
          volume?.Title,
          `第 ${index + 1} 卷`,
        ),
        chapters,
      };
    })
    .filter((volume) => volume.title || volume.chapters.length > 0);
}

function qidianChapterCount(content) {
  const raw = rawPreviewData(content);
  const explicit = Number(
    content?.chapter_count ||
      content?.chapterCount ||
      raw.chapter_count ||
      raw.chapterCount ||
      0,
  );
  if (explicit > 0) return explicit;
  return qidianVolumes(content).reduce(
    (total, volume) => total + volume.chapters.length,
    0,
  );
}

function qidianLatestChapterTitle(content) {
  const raw = rawPreviewData(content);
  const latest =
    content?.latest_chapter ||
    content?.latestChapter ||
    raw.latest_chapter ||
    raw.latestChapter ||
    {};
  if (typeof latest === "string") return latest;
  return firstNonEmpty(latest.title, latest.Title);
}

function qidianWordCount(content) {
  const raw = rawPreviewData(content);
  return firstNonEmpty(
    content?.display_word_count,
    content?.displayWordCount,
    raw.display_word_count,
    raw.displayWordCount,
    formatCompactNumber(content?.word_count || raw.word_count),
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

function normalizePlatformID(platform) {
  const value = String(platform || "")
    .trim()
    .toLowerCase();
  const aliases = {
    officialaccount: "officialaccount",
    wx_official_account: "officialaccount",
    xhs: "xiaohongshu",
    rednote: "xiaohongshu",
  };
  return aliases[value] || value;
}

function isKnownPreviewPlatform(platform) {
  return [
    "wx_channels",
    "douyin",
    "zhihu",
    "officialaccount",
    "youtube",
    "xiaohongshu",
    "weibo",
    "bilibili",
    "qidian",
    "mqidian",
  ].includes(normalizePlatformID(platform));
}

function rawPreviewData(content) {
  return content?.raw_data && typeof content.raw_data === "object"
    ? content.raw_data
    : {};
}

function valueAtPath(source, path) {
  if (!source || !path) return undefined;
  return String(path)
    .split(".")
    .reduce((current, key) => {
      if (current === undefined || current === null) return undefined;
      return current[key];
    }, source);
}

function hasPreviewValue(value) {
  if (value === undefined || value === null) return false;
  if (typeof value === "string") return value.trim() !== "";
  if (Array.isArray(value)) return value.length > 0;
  return true;
}

function previewField(content, ...paths) {
  const sources = [content || {}, rawPreviewData(content)];
  for (const path of paths) {
    for (const source of sources) {
      const value = valueAtPath(source, path);
      if (hasPreviewValue(value)) return value;
    }
  }
  return "";
}

function firstPreviewText(content, ...paths) {
  const value = previewField(content, ...paths);
  if (value === undefined || value === null) return "";
  if (typeof value === "object") return "";
  return String(value).trim();
}

function previewVideoDuration(content) {
  return formatDurationSeconds(
    previewField(
      content,
      "video.duration",
      "video.Duration",
      "duration",
      "Duration",
    ),
  );
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
  const raw = rawPreviewData(content);
  const rawStats = raw.stats || raw.Stats || {};
  const pairs = [
    [
      "播放",
      firstNonEmpty(content?.play_count, stats.play_count, rawStats.play_count),
    ],
    [
      "浏览",
      firstNonEmpty(content?.view_count, stats.view_count, rawStats.view_count),
    ],
    [
      "点赞",
      firstNonEmpty(content?.like_count, stats.like_count, rawStats.like_count),
    ],
    [
      "评论",
      firstNonEmpty(
        content?.comment_count,
        stats.comment_count,
        rawStats.comment_count,
        raw.commentCount,
      ),
    ],
    [
      "转发",
      firstNonEmpty(
        content?.repost_count,
        stats.repost_count,
        rawStats.repost_count,
      ),
    ],
    [
      "分享",
      firstNonEmpty(
        content?.share_count,
        stats.share_count,
        rawStats.share_count,
      ),
    ],
    [
      "收藏",
      firstNonEmpty(
        content?.collect_count,
        stats.collect_count,
        rawStats.collect_count,
      ),
    ],
    [
      "弹幕",
      firstNonEmpty(
        content?.danmaku_count,
        stats.danmaku_count,
        rawStats.danmaku_count,
      ),
    ],
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
      dataset: { t: "home-downloads-page-preview-badge-row-label-value-text" },
      class:
        "inline-flex h-6 items-center rounded-md bg-zinc-100 px-2 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200",
    },
    [label],
  );
}

function ContentAuthorIdentity(data_, options = {}) {
  const avatarSelector = options.avatarSelector || contentAuthorAvatar;
  const nameSelector = options.nameSelector || contentAuthorName;
  const homepageSelector = options.homepageSelector || contentAuthorHomepage;
  const fallbackName = options.fallbackName || "-";
  const rootClass = options.class || "mt-1 flex min-w-0 items-center gap-1.5";
  const avatarClass = options.avatarClass || "h-5 w-5";
  const nameClass =
    options.nameClass ||
    "min-w-0 truncate text-xs font-medium text-zinc-600 dark:text-zinc-300";
  const meta = options.meta || null;
  const href_ = computed(data_, (content) =>
    String(homepageSelector(content) || "").trim(),
  );
  const hasHref_ = computed(
    data_,
    (content) => !!String(homepageSelector(content) || "").trim(),
  );
  const platform_ = computed(
    data_,
    (content) =>
      options.platformId ||
      normalizePlatformID(content?.platform || content?.platform_id),
  );
  const children = () => [
    Show({
      when: computed(data_, (content) => !!avatarSelector(content)),
      ok() {
        return View(
          {
            dataset: {
              t: "home-downloads-page-content-author-identity-children-image",
            },
            class: classNames([
              avatarClass,
              "shrink-0 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-900",
            ]),
          },
          [
            ProxyImg({
              class: "h-full w-full object-cover",
              src: computed(data_, avatarSelector),
              alt: computed(data_, nameSelector),
              platformId: platform_,
            }),
          ],
        );
      },
    }),
    View(
      {
        dataset: {
          t: "home-downloads-page-content-author-identity-children-name-selector-fallback-name-value-meta-prefix-computed-value",
        },
        class: "min-w-0",
      },
      [
        View(
          {
            dataset: {
              t: "home-downloads-page-content-author-identity-children-name-selector-fallback-name-value",
            },
            class: classNames([nameClass, "group-hover/author:underline"]),
          },
          [computed(data_, (content) => nameSelector(content) || fallbackName)],
        ),
        meta
          ? View(
              {
                dataset: {
                  t: "home-downloads-page-content-author-identity-children-meta-prefix-computed-value",
                },
                class: meta.class || "text-xs text-zinc-400 dark:text-zinc-500",
              },
              [
                meta.prefix || "",
                computed(data_, (content) => {
                  if (typeof meta.selector !== "function") return "";
                  return meta.selector(content) || "";
                }),
              ],
            )
          : null,
      ],
    ),
  ];
  return Show({
    when: hasHref_,
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-content-author-identity-a-node-children",
          },
          as: "a",
          class: classNames([rootClass, "group/author no-underline"]),
          href: href_,
          target: "_blank",
          rel: "noopener noreferrer",
          onClick(event) {
            event.stopPropagation?.();
          },
        },
        children(),
      );
    },
    else() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-content-author-identity-children",
          },
          class: rootClass,
        },
        children(),
      );
    },
  });
}

function PreviewInfoItem(label, data_, selector) {
  return Show({
    when: computed(
      data_,
      (content) => !!String(selector(content) || "").trim(),
    ),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-preview-info-item-label-value-string",
          },
          class: "min-w-0",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-preview-info-item-label-value",
              },
              class: "text-[11px] text-zinc-400 dark:text-zinc-500",
            },
            [label],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-preview-info-item-string-text",
              },
              class:
                "mt-0.5 truncate text-xs font-medium text-zinc-700 dark:text-zinc-200",
            },
            [computed(data_, (content) => String(selector(content) || ""))],
          ),
        ],
      );
    },
  });
}

function PreviewInfoGrid(children) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-preview-info-grid-grid-children-value",
      },
      class: "mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3",
    },
    children,
  );
}

function TypeSpecificPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-type-specific-preview-dispatcher-root",
      },
    },
    [
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
              content.bitrate
                ? `${formatCompactNumber(content.bitrate)}bps`
                : "",
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
              formatDurationSeconds(
                content.video?.duration || content.duration,
              ),
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
              formatDurationSeconds(
                content.video?.duration || content.duration,
              ),
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
        when: computed(
          data_,
          (content) => content.content_type === "collection",
        ),
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
    ],
  );
}

function ContentPreviewMediaStrip(data_) {
  return Show({
    when: computed(data_, (content) => contentImages(content).length > 1),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-content-preview-media-strip-row-computed-value-list",
          },
          class: "mt-3 flex gap-2 overflow-hidden",
        },
        [
          For({
            each: computed(data_, (content) =>
              contentImages(content).slice(0, 5),
            ),
            render(url) {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-content-preview-media-strip-image",
                  },
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
        ],
      );
    },
  });
}

function ContentPreviewStats(data_) {
  return Show({
    when: computed(data_, (content) => previewStats(content).length > 0),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-content-preview-stats-row-preview-stats-list",
          },
          class: "mt-3 flex flex-wrap gap-x-4 gap-y-1",
        },
        [
          For({
            each: computed(data_, previewStats),
            render(pair) {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-content-preview-stats-pair-pair-1-text",
                  },
                  class: "text-xs text-zinc-500 dark:text-zinc-400",
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-content-preview-stats-span-node-pair",
                      },
                      as: "span",
                      class: "text-zinc-400 dark:text-zinc-500",
                    },
                    [`${pair[0]} `],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-content-preview-stats-span-node-pair-1",
                      },
                      as: "span",
                      class: "font-medium text-zinc-700 dark:text-zinc-200",
                    },
                    [pair[1]],
                  ),
                ],
              );
            },
          }),
        ],
      );
    },
  });
}

function ContentPreviewTags(data_) {
  return Show({
    when: computed(data_, (content) => previewTags(content).length > 0),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-content-preview-tags-row-preview-tags-list",
          },
          class: "mt-3 flex flex-wrap gap-1.5",
        },
        [
          For({
            each: computed(data_, previewTags),
            render(tag) {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-content-preview-tags-row-tag-value-text",
                  },
                  class:
                    "inline-flex h-6 items-center rounded-md bg-zinc-50 px-2 text-xs text-zinc-500 ring-1 ring-inset ring-zinc-200 dark:bg-zinc-900 dark:text-zinc-300 dark:ring-zinc-800",
                },
                [tag],
              );
            },
          }),
        ],
      );
    },
  });
}

function QidianCatalogPreview(data_) {
  return Show({
    when: computed(data_, (content) => qidianVolumes(content).length > 0),
    ok() {
      return View(
        {
          dataset: { t: "home-downloads-page-qidian-catalog-preview" },
          class: "mt-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-qidian-catalog-preview-title",
              },
              class: "text-xs font-medium text-zinc-500 dark:text-zinc-400",
            },
            ["目录"],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-qidian-catalog-preview-volumes",
              },
              class: "mt-2 space-y-2",
            },
            [
              For({
                each: computed(data_, (content) =>
                  qidianVolumes(content).slice(0, 4),
                ),
                render(volume) {
                  const shownChapters = volume.chapters.slice(0, 6);
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-qidian-catalog-preview-volume",
                      },
                      class:
                        "rounded-md bg-white px-3 py-2 ring-1 ring-inset ring-zinc-100 dark:bg-zinc-950/40 dark:ring-zinc-800",
                    },
                    [
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-qidian-catalog-preview-volume-title",
                          },
                          class:
                            "flex items-center justify-between gap-2 text-xs",
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-qidian-catalog-preview-volume-name",
                              },
                              class:
                                "min-w-0 truncate font-medium text-zinc-800 dark:text-zinc-100",
                            },
                            [volume.title],
                          ),
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-qidian-catalog-preview-volume-count",
                              },
                              class:
                                "shrink-0 text-zinc-400 dark:text-zinc-500",
                            },
                            [`${volume.chapters.length} 章`],
                          ),
                        ],
                      ),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-qidian-catalog-preview-chapters",
                          },
                          class: "mt-1.5 space-y-1",
                        },
                        [
                          For({
                            each: shownChapters,
                            render(chapter) {
                              const children = [
                                View(
                                  {
                                    dataset: {
                                      t: "home-downloads-page-qidian-catalog-preview-chapter-index",
                                    },
                                    as: "span",
                                    class:
                                      "shrink-0 text-zinc-400 dark:text-zinc-500",
                                  },
                                  [`${chapter.idx}.`],
                                ),
                                View(
                                  {
                                    dataset: {
                                      t: "home-downloads-page-qidian-catalog-preview-chapter-title",
                                    },
                                    as: "span",
                                    class: "min-w-0 truncate",
                                  },
                                  [chapter.title],
                                ),
                                chapter.locked
                                  ? View(
                                      {
                                        dataset: {
                                          t: "home-downloads-page-qidian-catalog-preview-chapter-locked",
                                        },
                                        as: "span",
                                        class:
                                          "shrink-0 text-zinc-400 dark:text-zinc-500",
                                      },
                                      ["VIP"],
                                    )
                                  : null,
                              ];
                              if (chapter.url) {
                                return View(
                                  {
                                    dataset: {
                                      t: "home-downloads-page-qidian-catalog-preview-chapter-link",
                                    },
                                    as: "a",
                                    class:
                                      "flex min-w-0 gap-1.5 text-xs text-zinc-600 no-underline hover:text-blue-600 dark:text-zinc-300 dark:hover:text-blue-300",
                                    href: chapter.url,
                                    target: "_blank",
                                    rel: "noopener noreferrer",
                                    onClick(event) {
                                      event.stopPropagation?.();
                                    },
                                  },
                                  children,
                                );
                              }
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-qidian-catalog-preview-chapter-row",
                                  },
                                  class:
                                    "flex min-w-0 gap-1.5 text-xs text-zinc-600 dark:text-zinc-300",
                                },
                                children,
                              );
                            },
                          }),
                          Show({
                            when: volume.chapters.length > shownChapters.length,
                            ok() {
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-qidian-catalog-preview-more",
                                  },
                                  class:
                                    "text-xs text-zinc-400 dark:text-zinc-500",
                                },
                                [
                                  `还有 ${volume.chapters.length - shownChapters.length} 章`,
                                ],
                              );
                            },
                          }),
                        ],
                      ),
                    ],
                  );
                },
              }),
              Show({
                when: computed(
                  data_,
                  (content) => qidianVolumes(content).length > 4,
                ),
                ok() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-qidian-catalog-preview-more-volumes",
                      },
                      class: "text-xs text-zinc-400 dark:text-zinc-500",
                    },
                    [
                      computed(
                        data_,
                        (content) =>
                          `还有 ${qidianVolumes(content).length - 4} 卷`,
                      ),
                    ],
                  );
                },
              }),
            ],
          ),
        ],
      );
    },
  });
}

function QidianContentPreview(data_) {
  return View(
    { dataset: { t: "home-downloads-page-qidian-content-preview" } },
    [
      View(
        {
          dataset: { t: "home-downloads-page-qidian-content-preview-row" },
          class: "flex items-start gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-qidian-content-preview-cover",
              },
              class:
                "h-24 w-16 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "qidian",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-qidian-content-preview-cover-empty",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "book-open", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: { t: "home-downloads-page-qidian-content-preview-main" },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-qidian-content-preview-badges",
                  },
                  class: "flex flex-wrap items-center gap-1.5",
                },
                [
                  PreviewBadge("起点小说"),
                  PreviewBadge(
                    computed(data_, (content) =>
                      firstNonEmpty(content.status, content.category),
                    ),
                  ),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-qidian-content-preview-title",
                  },
                  class:
                    "mt-2 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "qidian",
                meta: {
                  prefix: "章节 ",
                  selector(content) {
                    return qidianChapterCount(content) || "";
                  },
                },
              }),
            ],
          ),
        ],
      ),
      Show({
        when: computed(data_, (content) => !!previewDescription(content)),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-qidian-content-preview-description",
              },
              class:
                "mt-3 whitespace-pre-line text-xs leading-relaxed text-zinc-600 dark:text-zinc-300",
            },
            [
              computed(data_, (content) =>
                shortText(previewDescription(content), 260),
              ),
            ],
          );
        },
      }),
      PreviewInfoGrid([
        PreviewInfoItem("总字数", data_, qidianWordCount),
        PreviewInfoItem("章节", data_, (content) =>
          qidianChapterCount(content),
        ),
        PreviewInfoItem(
          "卷数",
          data_,
          (content) => qidianVolumes(content).length,
        ),
        PreviewInfoItem("最新章节", data_, (content) =>
          shortText(qidianLatestChapterTitle(content), 42),
        ),
        PreviewInfoItem("分类", data_, (content) =>
          firstNonEmpty(content.category, content.sub_category),
        ),
        PreviewInfoItem("内容 ID", data_, (content) =>
          firstPreviewText(content, "content_id", "id", "ID"),
        ),
      ]),
      QidianCatalogPreview(data_),
      ContentPreviewTags(data_),
    ],
  );
}

function GenericContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-generic-content-preview-cover-media-preview-badges-list-preview-title-content-author-identity-content-preview-media-strip-type-specific-preview-content-preview-stats-content-preview-tags-preview-info-grid",
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-generic-content-preview-row-cover-media-preview-badges-list-preview-title-content-author-identity",
          },
          class: "flex items-start gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-generic-content-preview-cover-media",
              },
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
                      dataset: {
                        t: "home-downloads-page-generic-content-preview-row-match",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [
                      Match({
                        when: computed(
                          data_,
                          (content) => content.content_type,
                        ),
                        cases: IconMappedType(),
                      }),
                    ],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-generic-content-preview-row-preview-badges-list-preview-title-content-author-identity",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-generic-content-preview-row-preview-badges-list",
                  },
                  class: "flex flex-wrap items-center gap-1.5",
                },
                [
                  For({
                    each: computed(data_, previewBadges),
                    render(label) {
                      return PreviewBadge(label);
                    },
                  }),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-generic-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-2 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_),
            ],
          ),
        ],
      ),
      Show({
        when: computed(data_, (content) => !!previewDescription(content)),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-generic-content-preview-short-text-text",
              },
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
        PreviewInfoItem("内容 ID", data_, (content) =>
          firstPreviewText(content, "content_id", "id", "ID"),
        ),
        PreviewInfoItem("发布", data_, (content) =>
          formatTimestamp(
            previewField(
              content,
              "publish_time",
              "created_time",
              "create_time",
              "createtime",
              "CreatedTime",
              "createdAt",
            ),
          ),
        ),
        PreviewInfoItem("更新", data_, (content) =>
          formatTimestamp(
            previewField(content, "update_time", "updated_time", "UpdatedTime"),
          ),
        ),
        PreviewInfoItem("链接", data_, (content) =>
          firstPreviewText(
            content,
            "canonical_url",
            "source_url",
            "share_url",
            "url",
            "URL",
          ),
        ),
      ]),
      Show({
        when: computed(
          data_,
          (content) => asArray(content.warnings).length > 0,
        ),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-generic-content-preview-warning-panel-as-array-list-text",
              },
              class:
                "mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
            },
            [
              For({
                each: computed(data_, (content) => asArray(content.warnings)),
                render(warning) {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-generic-content-preview-warning-value",
                      },
                      class: "truncate",
                    },
                    [warning],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function WxChannelsContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-wx-channels-content-preview-视频号-preview-title-content-author-identity-first-non-empty",
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-wx-channels-content-preview-row-视频号-preview-title-content-author-identity-first-non-empty",
          },
          class: "flex gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-wx-channels-content-preview-existing-task-warning",
              },
              class:
                "h-24 w-24 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "wx_channels",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-wx-channels-content-preview-row-icon-film",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "film", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-wx-channels-content-preview-row-视频号-preview-title-content-author-identity-first-non-empty-2",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-wx-channels-content-preview-success-视频号-text",
                  },
                  class:
                    "text-xs font-medium text-emerald-600 dark:text-emerald-300",
                },
                ["视频号"],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-wx-channels-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "wx_channels",
                nameClass:
                  "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
              }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-wx-channels-content-preview-first-non-empty-text",
                  },
                  class: "mt-2 text-xs text-zinc-500 dark:text-zinc-400",
                },
                [
                  computed(data_, (content) =>
                    firstNonEmpty(
                      previewVideoDuration(content),
                      countImages(content)
                        ? `${countImages(content)} 张图片`
                        : "",
                      content.content_id,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
      Show({
        when: computed(data_, (content) => {
          const description = previewDescription(content);
          return description && description !== previewTitle(content);
        }),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-wx-channels-content-preview-short-text-text",
              },
              class:
                "mt-3 line-clamp-3 text-xs leading-relaxed text-zinc-600 dark:text-zinc-300",
            },
            [
              computed(data_, (content) =>
                shortText(previewDescription(content), 180),
              ),
            ],
          );
        },
      }),
      Show({
        when: computed(data_, (content) => contentImages(content).length > 1),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-wx-channels-content-preview-grid-computed-value-list",
              },
              class: "mt-3 grid grid-cols-4 gap-2",
            },
            [
              For({
                each: computed(data_, (content) =>
                  contentImages(content).slice(0, 4),
                ),
                render(url) {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-wx-channels-content-preview-cover-media-image",
                      },
                      class:
                        "aspect-square overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                    },
                    [
                      ProxyImg({
                        class: "h-full w-full object-cover",
                        src: url,
                        alt: "channels image",
                        platformId: "wx_channels",
                      }),
                    ],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function DouyinContentPreview(data_) {
  return Show({
    when: computed(data_, (content) => content.content_type === "video"),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-douyin-content-preview-row-抖音视频-content-author-identity-short-text",
          },
          class: "flex gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-douyin-content-preview-existing-task-warning",
              },
              class:
                "h-32 w-24 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "douyin",
                    contentType: "video",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-douyin-content-preview-row-icon-film",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "film", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-douyin-content-preview-row-抖音视频-content-author-identity-short-text-2",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-douyin-content-preview-抖音视频-text",
                  },
                  class: "text-xs font-medium text-pink-600 dark:text-pink-300",
                },
                ["抖音视频"],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "douyin",
                class: "mt-2 flex min-w-0 items-center gap-1.5",
                nameClass:
                  "min-w-0 truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
              }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-douyin-content-preview-short-text-text",
                  },
                  class:
                    "mt-2 line-clamp-3 text-xs leading-relaxed text-zinc-600 dark:text-zinc-300",
                },
                [
                  computed(data_, (content) =>
                    shortText(
                      firstNonEmpty(
                        previewDescription(content),
                        content.title,
                        content.content_id,
                      ),
                      180,
                    ),
                  ),
                ],
              ),
              Show({
                when: computed(
                  data_,
                  (content) =>
                    !!formatTimestamp(
                      previewField(content, "publish_time", "created_time"),
                    ),
                ),
                ok() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-douyin-content-preview-发布于-format-timestamp-text",
                      },
                      class: "mt-2 text-xs text-zinc-400 dark:text-zinc-500",
                    },
                    [
                      "发布于 ",
                      computed(data_, (content) =>
                        formatTimestamp(
                          previewField(content, "publish_time", "created_time"),
                        ),
                      ),
                    ],
                  );
                },
              }),
            ],
          ),
        ],
      );
    },
    else() {
      return GenericContentPreview(data_);
    },
  });
}

function ZhihuContentPreview(data_) {
  return Show({
    when: computed(data_, (content) => content.content_type === "answer"),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-zhihu-content-preview-stack-知乎问题-computed-value-提问者-computed-value-computed-value-content-author-identity-computed-value",
          },
          class: "space-y-4",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-zhihu-content-preview-知乎问题-computed-value-提问者-computed-value-computed-value",
              },
              class: "border-b border-zinc-200 pb-4 dark:border-zinc-800",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-zhihu-content-preview-知乎问题-text",
                  },
                  class: "text-xs font-medium text-blue-600 dark:text-blue-300",
                },
                ["知乎问题"],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-zhihu-content-preview-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-base font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [
                  computed(data_, (content) => {
                    const raw = rawPreviewData(content);
                    const question = raw.question || raw.Question || {};
                    return firstNonEmpty(
                      content.question_title,
                      question.title,
                      question.Title,
                      content.title,
                      "未命名问题",
                    );
                  }),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-zhihu-content-preview-提问者-computed-value-text",
                  },
                  class: "mt-2 text-xs text-zinc-500 dark:text-zinc-400",
                },
                [
                  "提问者 ",
                  computed(data_, (content) => {
                    const raw = rawPreviewData(content);
                    const question = raw.question || raw.Question || {};
                    const author = question.author || question.Author || {};
                    return firstNonEmpty(
                      author.name,
                      author.Name,
                      author.nickname,
                      author.username,
                      author.id,
                      "未知",
                    );
                  }),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-zhihu-content-preview-text",
                  },
                  class:
                    "mt-2 line-clamp-3 text-xs leading-relaxed text-zinc-600 dark:text-zinc-300",
                },
                [
                  computed(data_, (content) => {
                    const raw = rawPreviewData(content);
                    const question = raw.question || raw.Question || {};
                    return shortText(
                      firstNonEmpty(
                        content.question_html,
                        question.detail,
                        question.Detail,
                        question.excerpt,
                        question.Excerpt,
                      ),
                      220,
                    );
                  }),
                ],
              ),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-zhihu-content-preview-content-author-identity-computed-value",
              },
            },
            [
              ContentAuthorIdentity(data_, {
                platformId: "zhihu",
                class: "flex min-w-0 items-center gap-2",
                avatarClass: "h-7 w-7",
                nameClass:
                  "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                avatarSelector(content) {
                  const raw = rawPreviewData(content);
                  const answer = raw.answer || raw.Answer || {};
                  const author = answer.author || answer.Author || {};
                  return firstNonEmpty(
                    contentAuthorAvatar(content),
                    author.avatarUrl,
                    author.avatar_url,
                    author.AvatarURL,
                  );
                },
                nameSelector(content) {
                  const raw = rawPreviewData(content);
                  const answer = raw.answer || raw.Answer || {};
                  const author = answer.author || answer.Author || {};
                  return firstNonEmpty(
                    contentAuthorName(content),
                    author.name,
                    author.Name,
                    author.id,
                    "未知回答人",
                  );
                },
                meta: {
                  prefix: "回答于 ",
                  selector(content) {
                    const raw = rawPreviewData(content);
                    const answer = raw.answer || raw.Answer || {};
                    return (
                      formatTimestamp(
                        firstNonEmpty(
                          content.created_time,
                          answer.createdTime,
                          answer.CreatedTime,
                        ),
                      ) || "-"
                    );
                  },
                },
              }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-zhihu-content-preview-text-2",
                  },
                  class:
                    "mt-3 line-clamp-6 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
                },
                [
                  computed(data_, (content) => {
                    const raw = rawPreviewData(content);
                    const answer = raw.answer || raw.Answer || {};
                    return shortText(
                      firstNonEmpty(
                        content.body_html,
                        content.body_text,
                        answer.content,
                        answer.Content,
                        content.excerpt,
                        content.description,
                      ),
                      420,
                    );
                  }),
                ],
              ),
            ],
          ),
        ],
      );
    },
    else() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-zhihu-content-preview-stack-知乎问题-or-知乎文章-preview-title-content-author-identity-short-text",
          },
          class: "space-y-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-zhihu-content-preview-知乎问题-or-知乎文章-text",
              },
              class: "text-xs font-medium text-blue-600 dark:text-blue-300",
            },
            [
              computed(data_, (content) =>
                content.content_type === "question" ? "知乎问题" : "知乎文章",
              ),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-zhihu-content-preview-preview-title-heading",
              },
              class:
                "line-clamp-2 text-base font-semibold text-zinc-950 dark:text-zinc-50",
            },
            [computed(data_, previewTitle)],
          ),
          ContentAuthorIdentity(data_, {
            platformId: "zhihu",
            class: "flex min-w-0 items-center gap-1.5",
            nameClass:
              "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
          }),
          View(
            {
              dataset: {
                t: "home-downloads-page-zhihu-content-preview-short-text-text",
              },
              class:
                "line-clamp-5 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
            },
            [
              computed(data_, (content) =>
                shortText(
                  firstNonEmpty(
                    content.body_html,
                    content.body_text,
                    content.detail,
                    content.description,
                  ),
                  360,
                ),
              ),
            ],
          ),
        ],
      );
    },
  });
}

function OfficialAccountContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-official-account-content-preview-stack-公众号文章-preview-title-content-author-identity-short-text",
      },
      class: "space-y-3",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-official-account-content-preview-row-公众号文章-preview-title-content-author-identity",
          },
          class: "flex gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-official-account-content-preview-existing-task-warning",
              },
              class:
                "h-20 w-28 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "officialaccount",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-official-account-content-preview-row-icon-file-text",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "file-text", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-official-account-content-preview-row-公众号文章-preview-title-content-author-identity-2",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-official-account-content-preview-success-公众号文章-text",
                  },
                  class:
                    "text-xs font-medium text-green-600 dark:text-green-300",
                },
                ["公众号文章"],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-official-account-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "officialaccount",
                nameClass:
                  "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
              }),
            ],
          ),
        ],
      ),
      View(
        {
          dataset: {
            t: "home-downloads-page-official-account-content-preview-short-text-text",
          },
          class:
            "line-clamp-4 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
        },
        [
          computed(data_, (content) =>
            shortText(
              firstNonEmpty(
                content.digest,
                content.description,
                content.body_text,
                content.body_html,
              ),
              320,
            ),
          ),
        ],
      ),
    ],
  );
}

function YoutubeContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-youtube-content-preview-YouTube-preview-title-content-author-identity-first-non-empty",
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-youtube-content-preview-row-YouTube-preview-title-content-author-identity-first-non-empty",
          },
          class: "flex gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-youtube-content-preview-existing-task-warning",
              },
              class:
                "h-20 w-32 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "youtube",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-youtube-content-preview-row-icon-youtube",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "youtube", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-youtube-content-preview-row-YouTube-preview-title-content-author-identity-first-non-empty-2",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-youtube-content-preview-error-YouTube-text",
                  },
                  class: "text-xs font-medium text-red-600 dark:text-red-300",
                },
                ["YouTube"],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-youtube-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "youtube",
                nameClass:
                  "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
              }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-youtube-content-preview-first-non-empty-text",
                  },
                  class: "mt-2 text-xs text-zinc-500 dark:text-zinc-400",
                },
                [
                  computed(data_, (content) =>
                    firstNonEmpty(
                      previewVideoDuration(content),
                      firstPreviewText(content, "video_id", "content_id"),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function XiaohongshuContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-xiaohongshu-content-preview-stack-小红书视频笔记-or-小红书图文笔记-preview-title-content-author-identity-short-text",
      },
      class: "space-y-3",
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-xiaohongshu-content-preview-row-小红书视频笔记-or-小红书图文笔记-preview-title-content-author-identity",
          },
          class: "flex min-w-0 items-start justify-between gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-xiaohongshu-content-preview-小红书视频笔记-or-小红书图文笔记-preview-title-content-author-identity",
              },
              class: "min-w-0",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-xiaohongshu-content-preview-error-小红书视频笔记-or-小红书图文笔记-text",
                  },
                  class: "text-xs font-medium text-red-500 dark:text-red-300",
                },
                [
                  computed(data_, (content) =>
                    content.note_type === "video"
                      ? "小红书视频笔记"
                      : "小红书图文笔记",
                  ),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-xiaohongshu-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "xiaohongshu",
                nameClass:
                  "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
              }),
            ],
          ),
          Show({
            when: computed(data_, contentCoverURL),
            ok() {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-xiaohongshu-content-preview-image",
                  },
                  class:
                    "h-20 w-16 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                },
                [
                  ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "xiaohongshu",
                  }),
                ],
              );
            },
          }),
        ],
      ),
      View(
        {
          dataset: {
            t: "home-downloads-page-xiaohongshu-content-preview-short-text-text",
          },
          class:
            "line-clamp-4 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
        },
        [
          computed(data_, (content) =>
            shortText(firstNonEmpty(content.text, content.description), 260),
          ),
        ],
      ),
      Show({
        when: computed(data_, (content) => contentImages(content).length > 0),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-xiaohongshu-content-preview-grid-computed-value-list",
              },
              class: "grid grid-cols-4 gap-2",
            },
            [
              For({
                each: computed(data_, (content) =>
                  contentImages(content).slice(0, 4),
                ),
                render(url) {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-xiaohongshu-content-preview-cover-media-image",
                      },
                      class:
                        "aspect-square overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                    },
                    [
                      ProxyImg({
                        class: "h-full w-full object-cover",
                        src: url,
                        alt: "xhs image",
                        platformId: "xiaohongshu",
                      }),
                    ],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function weiboPostImageURL(image) {
  if (!image) return "";
  if (typeof image === "string") return image;
  return firstNonEmpty(
    image.url,
    image.URL,
    image.original?.url,
    image.original?.URL,
    image.largest?.url,
    image.largest?.URL,
    image.large?.url,
    image.large?.URL,
    image.bmiddle?.url,
    image.bmiddle?.URL,
    image.thumbnail?.url,
    image.thumbnail?.URL,
    image.cover_url,
    image.coverUrl,
  );
}

function weiboPostImages(post) {
  const direct = [
    ...asArray(post?.pic_urls),
    ...asArray(post?.picUrls),
    ...asArray(post?.images),
    ...asArray(post?.Images),
  ]
    .map(weiboPostImageURL)
    .filter(Boolean);
  const picInfos = post?.pic_infos || post?.picInfos || {};
  const fromInfos =
    picInfos && typeof picInfos === "object"
      ? Object.values(picInfos).map(weiboPostImageURL).filter(Boolean)
      : [];
  return [...new Set([...direct, ...fromInfos])];
}

function weiboPosts(content) {
  const raw = rawPreviewData(content);
  const responseList =
    raw?.response?.data?.list ||
    raw?.Response?.Data?.List ||
    raw?.raw_response?.data?.list ||
    [];
  return [
    ...asArray(content?.posts),
    ...asArray(content?.list),
    ...asArray(raw?.posts),
    ...asArray(raw?.list),
    ...asArray(responseList),
  ].filter((post) => post && typeof post === "object");
}

function weiboPostText(post) {
  return firstNonEmpty(
    post?.text,
    post?.text_raw,
    post?.textRaw,
    post?.Text,
    post?.TextRaw,
    post?.description,
  );
}

function weiboPostTime(post) {
  return formatTimestamp(
    firstNonEmpty(
      post?.created_time,
      post?.createdTime,
      post?.created_at,
      post?.createdAt,
      post?.CreatedAt,
    ),
  );
}

function WeiboContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-weibo-content-preview-stack-content-author-identity-short-text",
      },
      class: "space-y-3",
    },
    [
      ContentAuthorIdentity(data_, {
        platformId: "weibo",
        class: "flex min-w-0 items-center gap-2",
        avatarClass: "h-7 w-7",
        nameClass:
          "truncate text-sm font-semibold text-zinc-950 dark:text-zinc-50",
        meta: {
          class: "text-xs text-orange-500 dark:text-orange-300",
          selector(content) {
            return weiboPosts(content).length > 0 ? "微博列表" : "微博动态";
          },
        },
      }),
      View(
        {
          dataset: {
            t: "home-downloads-page-weibo-content-preview-short-text-text",
          },
          class:
            "line-clamp-5 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
        },
        [
          computed(data_, (content) =>
            shortText(
              firstNonEmpty(
                content.rich_text,
                content.text,
                content.description,
                weiboPostText(weiboPosts(content)[0]),
              ),
              360,
            ),
          ),
        ],
      ),
      Show({
        when: computed(data_, (content) => weiboPosts(content).length > 0),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-weibo-content-preview-post-list-stack",
              },
              class: "space-y-2",
            },
            [
              For({
                each: computed(data_, (content) =>
                  weiboPosts(content).slice(0, 4),
                ),
                render(post) {
                  const images = weiboPostImages(post).slice(0, 3);
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-weibo-content-preview-post-row",
                      },
                      class:
                        "rounded-md border border-zinc-100 bg-zinc-50 px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900/60",
                    },
                    [
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-weibo-content-preview-post-text",
                          },
                          class:
                            "line-clamp-2 text-xs leading-relaxed text-zinc-700 dark:text-zinc-200",
                        },
                        [shortText(weiboPostText(post), 120) || "无正文"],
                      ),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-weibo-content-preview-post-meta",
                          },
                          class:
                            "mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-zinc-400 dark:text-zinc-500",
                        },
                        [
                          weiboPostTime(post),
                          post.like_count || post.attitudes_count
                            ? `赞 ${formatCompactNumber(post.like_count || post.attitudes_count)}`
                            : "",
                          post.comment_count || post.comments_count
                            ? `评 ${formatCompactNumber(post.comment_count || post.comments_count)}`
                            : "",
                          post.repost_count || post.reposts_count
                            ? `转 ${formatCompactNumber(post.repost_count || post.reposts_count)}`
                            : "",
                        ].filter(Boolean),
                      ),
                      Show({
                        when: images.length > 0,
                        ok() {
                          return View(
                            {
                              dataset: {
                                t: "home-downloads-page-weibo-content-preview-post-images",
                              },
                              class: "mt-2 grid grid-cols-3 gap-1.5",
                            },
                            [
                              For({
                                each: images,
                                render(url) {
                                  return View(
                                    {
                                      dataset: {
                                        t: "home-downloads-page-weibo-content-preview-post-image",
                                      },
                                      class:
                                        "aspect-square overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                                    },
                                    [
                                      ProxyImg({
                                        class: "h-full w-full object-cover",
                                        src: url,
                                        alt: "weibo image",
                                        platformId: "weibo",
                                      }),
                                    ],
                                  );
                                },
                              }),
                            ],
                          );
                        },
                      }),
                    ],
                  );
                },
              }),
            ],
          );
        },
      }),
      Show({
        when: computed(data_, (content) => contentImages(content).length > 0),
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-weibo-content-preview-grid-computed-value-list",
              },
              class: "grid grid-cols-3 gap-2",
            },
            [
              For({
                each: computed(data_, (content) =>
                  contentImages(content).slice(0, 6),
                ),
                render(url) {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-weibo-content-preview-cover-media-image",
                      },
                      class:
                        "aspect-square overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
                    },
                    [
                      ProxyImg({
                        class: "h-full w-full object-cover",
                        src: url,
                        alt: "weibo image",
                        platformId: "weibo",
                      }),
                    ],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function BilibiliContentPreview(data_) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-bilibili-content-preview-B站-content-type-label-preview-title-content-author-identity-preview-stats-list",
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-bilibili-content-preview-row-B站-content-type-label-preview-title-content-author-identity-preview-stats-list",
          },
          class: "flex gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-bilibili-content-preview-existing-task-warning",
              },
              class:
                "h-20 w-32 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900",
            },
            [
              Show({
                when: computed(data_, contentCoverURL),
                ok() {
                  return ProxyImg({
                    class: "h-full w-full object-cover",
                    src: computed(data_, contentCoverURL),
                    alt: computed(data_, previewTitle),
                    platformId: "bilibili",
                  });
                },
                else() {
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-bilibili-content-preview-row-icon-film",
                      },
                      class:
                        "flex h-full w-full items-center justify-center text-zinc-400 dark:text-zinc-500",
                    },
                    [Icon({ name: "film", size: 28 })],
                  );
                },
              }),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-bilibili-content-preview-row-B站-content-type-label-preview-title-content-author-identity-preview-stats-list-2",
              },
              class: "min-w-0 flex-1",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-bilibili-content-preview-B站-content-type-label-text",
                  },
                  class: "text-xs font-medium text-sky-600 dark:text-sky-300",
                },
                [
                  computed(
                    data_,
                    (content) => `B站${contentTypeLabel(content.content_type)}`,
                  ),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-bilibili-content-preview-preview-title-heading",
                  },
                  class:
                    "mt-1 line-clamp-2 text-sm font-semibold text-zinc-950 dark:text-zinc-50",
                },
                [computed(data_, previewTitle)],
              ),
              ContentAuthorIdentity(data_, {
                platformId: "bilibili",
                nameClass:
                  "min-w-0 truncate text-xs text-zinc-500 dark:text-zinc-400",
              }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-bilibili-content-preview-row-preview-stats-list",
                  },
                  class: "mt-2 flex flex-wrap gap-x-3 gap-y-1",
                },
                [
                  For({
                    each: computed(data_, previewStats),
                    render(pair) {
                      return View(
                        {
                          dataset: {
                            t: "home-downloads-page-bilibili-content-preview-pair-pair-1-text",
                          },
                          class: "text-xs text-zinc-500 dark:text-zinc-400",
                        },
                        [`${pair[0]} ${pair[1]}`],
                      );
                    },
                  }),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

function ContentPreview(props) {
  const data_ = combine(
    {
      content: props.content,
      probe: props.probe,
      raw: props.raw,
    },
    ({ content, probe, raw }) => {
      return normalizeProbePreviewContent(content, probe, raw);
    },
  );

  return View(
    {
      dataset: { t: "home-downloads-page-content-preview-panel-body" },
      class: "min-w-0 rounded-md bg-zinc-50 p-3 dark:bg-zinc-900/60",
    },
    [
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "wx_channels",
        ),
        ok() {
          return WxChannelsContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "douyin",
        ),
        ok() {
          return DouyinContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "zhihu",
        ),
        ok() {
          return ZhihuContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) =>
            normalizePlatformID(content.platform) === "officialaccount",
        ),
        ok() {
          return OfficialAccountContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "youtube",
        ),
        ok() {
          return YoutubeContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "xiaohongshu",
        ),
        ok() {
          return XiaohongshuContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "weibo",
        ),
        ok() {
          return WeiboContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => normalizePlatformID(content.platform) === "bilibili",
        ),
        ok() {
          return BilibiliContentPreview(data_);
        },
      }),
      Show({
        when: computed(data_, (content) =>
          ["qidian", "mqidian"].includes(normalizePlatformID(content.platform)),
        ),
        ok() {
          return QidianContentPreview(data_);
        },
      }),
      Show({
        when: computed(
          data_,
          (content) => !isKnownPreviewPlatform(content.platform),
        ),
        ok() {
          return GenericContentPreview(data_);
        },
      }),
    ],
  );
}

function ProbePipelinePreview(props) {
  const nodes_ = computed(props.raw, normalizeProbePipeline);
  return Show({
    when: computed(nodes_, (nodes) => nodes.length > 0),
    ok() {
      return View(
        {
          dataset: {
            t: "home-downloads-page-probe-pipeline-preview-panel-解析流程-pipeline-workflow-text-nodes_-list",
          },
          class:
            "mt-3 rounded-md border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-probe-pipeline-preview-header-row-解析流程-pipeline-workflow-text",
              },
              class:
                "flex items-center justify-between gap-3 border-b border-zinc-100 px-3 py-2 dark:border-zinc-800",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-probe-pipeline-preview-解析流程-text",
                  },
                  class: "text-xs font-medium text-zinc-500 dark:text-zinc-400",
                },
                ["解析流程"],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-probe-pipeline-preview-pipeline-workflow-text-text",
                  },
                  class: "shrink-0 text-xs text-zinc-400 dark:text-zinc-500",
                },
                [computed(props.raw, pipelineWorkflowText)],
              ),
            ],
          ),
          View(
            {
              dataset: {
                t: "home-downloads-page-probe-pipeline-preview-nodes_-list",
              },
              class: "divide-y divide-zinc-100 dark:divide-zinc-800",
            },
            [
              For({
                each: nodes_,
                render(node, index) {
                  const output_ = computed(node, pipelineNodeOutput);
                  const details_ = computed(node, pipelineNodeDetails);
                  const description_ = combine(
                    {
                      node,
                      raw: props.raw,
                    },
                    ({ node, raw }) => pipelineNodeDescription(node, raw),
                  );
                  return View(
                    {
                      dataset: {
                        t: "home-downloads-page-probe-pipeline-preview-row-icon-pipeline-node-icon-name-pipeline-node-label-pipeline-node-status-label-description_-value",
                      },
                      class: "flex min-w-0 gap-3 px-3 py-3",
                    },
                    [
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-probe-pipeline-preview-icon-pipeline-node-icon-name",
                          },
                          class: classNames([
                            "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full ring-1",
                            computed(node, pipelineNodeIconWrapClass),
                          ]),
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-probe-pipeline-preview-icon-pipeline-node-icon-name-2",
                              },
                              class: computed(node, pipelineNodeIconClass),
                            },
                            [
                              Icon({
                                name: computed(node, pipelineNodeIconName),
                                size: 14,
                              }),
                            ],
                          ),
                        ],
                      ),
                      View(
                        {
                          dataset: {
                            t: "home-downloads-page-probe-pipeline-preview-row-pipeline-node-label-pipeline-node-status-label-description_-value",
                          },
                          class: "min-w-0 flex-1",
                        },
                        [
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-probe-pipeline-preview-row-pipeline-node-label-pipeline-node-status-label",
                              },
                              class:
                                "flex min-w-0 flex-wrap items-center justify-between gap-2",
                            },
                            [
                              View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-probe-pipeline-preview-pipeline-node-label-text",
                                  },
                                  class:
                                    "min-w-0 truncate text-sm font-medium text-zinc-900 dark:text-zinc-100",
                                },
                                [
                                  computed(node, (n) =>
                                    pipelineNodeLabel(n, index),
                                  ),
                                ],
                              ),
                              View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-probe-pipeline-preview-pipeline-node-status-label",
                                  },
                                  class: classNames([
                                    "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
                                    computed(node, pipelineNodeStatusClass),
                                  ]),
                                },
                                [computed(node, pipelineNodeStatusLabel)],
                              ),
                            ],
                          ),
                          View(
                            {
                              dataset: {
                                t: "home-downloads-page-probe-pipeline-preview-description_-value-text",
                              },
                              class:
                                "mt-1 text-xs leading-5 text-zinc-500 dark:text-zinc-400",
                            },
                            [description_],
                          ),
                          Show({
                            when: computed(details_, (list) => list.length > 0),
                            ok() {
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-probe-pipeline-preview-grid-details_-list-text",
                                  },
                                  class:
                                    "mt-2 grid gap-1 text-xs text-zinc-500 dark:text-zinc-400",
                                },
                                [
                                  For({
                                    each: details_,
                                    render(detail) {
                                      return View(
                                        {
                                          dataset: {
                                            t: "home-downloads-page-probe-pipeline-preview-detail-value",
                                          },
                                          class:
                                            "min-w-0 truncate rounded bg-zinc-100 px-2 py-1 dark:bg-zinc-800/70",
                                        },
                                        [detail],
                                      );
                                    },
                                  }),
                                ],
                              );
                            },
                          }),
                          Show({
                            when: computed(
                              output_,
                              (out) =>
                                !!firstNonEmpty(out.body_html, out.bodyHTML),
                            ),
                            ok() {
                              return View(
                                {
                                  dataset: {
                                    t: "home-downloads-page-probe-pipeline-preview-panel-iframe-node",
                                  },
                                  class:
                                    "mt-3 overflow-hidden rounded-md border border-zinc-200 bg-white dark:border-zinc-800",
                                },
                                [
                                  View({
                                    dataset: {
                                      t: "home-downloads-page-probe-pipeline-preview-iframe-node",
                                    },
                                    tag: "iframe",
                                    class: "h-80 w-full border-0",
                                    srcdoc: computed(output_, (out) =>
                                      firstNonEmpty(
                                        out.body_html,
                                        out.bodyHTML,
                                      ),
                                    ),
                                  }),
                                ],
                              );
                            },
                          }),
                        ],
                      ),
                    ],
                  );
                },
              }),
            ],
          ),
        ],
      );
    },
  });
}

function PlatformCreateErrorPanel(error_) {
  const expanded_ = ref(false);
  const overflowing_ = ref(false);

  let content_el = null;
  let resize_observer = null;
  let frame = 0;

  function measure() {
    if (!content_el) return;
    const overflowing = content_el.scrollHeight > 121;
    if (overflowing_.value !== overflowing) {
      overflowing_.as(overflowing);
    }
    if (!overflowing && expanded_.value) {
      expanded_.as(false);
    }
  }

  function scheduleMeasure() {
    if (!content_el || typeof window === "undefined") return;
    if (frame) window.cancelAnimationFrame(frame);
    frame = window.requestAnimationFrame(() => {
      frame = 0;
      measure();
    });
  }

  function stopMeasure() {
    if (frame && typeof window !== "undefined") {
      window.cancelAnimationFrame(frame);
      frame = 0;
    }
    if (resize_observer) {
      resize_observer.disconnect();
      resize_observer = null;
    }
    content_el = null;
  }

  function mountedElement(event) {
    const target = event?.target || event;
    if (!target) return null;
    if (typeof HTMLElement !== "undefined" && target instanceof HTMLElement) {
      return target;
    }
    if (typeof target.get$elm === "function") {
      return target.get$elm();
    }
    if (
      typeof HTMLElement !== "undefined" &&
      target.$elm instanceof HTMLElement
    ) {
      return target.$elm;
    }
    return null;
  }

  return View(
    {
      dataset: {
        t: "home-downloads-page-platform-create-panel-error-panel-vm-state-create-error-text",
      },
      class:
        "mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
      onUnmounted() {
        stopMeasure();
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-platform-create-panel-error-panel-content-text",
          },
          class: classNames([
            "whitespace-pre-wrap break-words",
            computed(expanded_, (expanded) => {
              return expanded
                ? "overflow-visible"
                : "max-h-[120px] overflow-hidden";
            }),
          ]),
          onMounted(event) {
            content_el = mountedElement(event);
            if (!content_el) return;
            if (typeof ResizeObserver !== "undefined") {
              resize_observer = new ResizeObserver(scheduleMeasure);
              resize_observer.observe(content_el);
            }
            scheduleMeasure();
          },
        },
        [
          computed(error_, (value) => {
            scheduleMeasure();
            return value;
          }),
        ],
      ),
      Show({
        when: overflowing_,
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-platform-create-panel-error-panel-toggle-row",
              },
              class: "mt-2 flex justify-end",
            },
            [
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "ghost",
                    size: "sm",
                    onClick() {
                      expanded_.toggle();
                    },
                  }),
                },
                [
                  computed(expanded_, (expanded) =>
                    expanded ? "收起" : "展开",
                  ),
                  Icon({
                    name: computed(expanded_, (expanded) =>
                      expanded ? "chevron-up" : "chevron-down",
                    ),
                    size: 14,
                  }),
                ],
              ),
            ],
          );
        },
      }),
    ],
  );
}

function PlatformCreatePanel(vm$) {
  return View(
    {
      dataset: {
        t: "home-downloads-page-platform-create-panel-icon-download-新建下载-input-button",
      },
    },
    [
      View(
        {
          dataset: {
            t: "home-downloads-page-platform-create-panel-row-icon-download-新建下载",
          },
          class: "flex flex-wrap items-center justify-between gap-3",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-platform-create-panel-row-icon-download-新建下载-2",
              },
              class: "flex items-center gap-2",
            },
            [
              Icon({ name: "download", size: 18 }),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-platform-create-panel-新建下载-heading",
                  },
                  class:
                    "text-base font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["新建下载"],
              ),
            ],
          ),
          Show({
            when: vm$.state.createLoading,
            ok() {
              return View(
                {
                  dataset: {
                    t: "home-downloads-page-platform-create-panel-解析中-text",
                  },
                  class: "text-xs text-zinc-500 dark:text-zinc-400",
                },
                ["解析中..."],
              );
            },
          }),
        ],
      ),
      View(
        {
          dataset: {
            t: "home-downloads-page-platform-create-panel-row-input-button",
          },
          class: "mt-3 flex flex-col gap-3 lg:flex-row",
        },
        [
          View(
            {
              dataset: {
                t: "home-downloads-page-platform-create-panel-row-input",
              },
              class: "min-w-0 flex-1",
            },
            [Input({ store: vm$.ui.createUrlInput })],
          ),
          Button(
            {
              store: vm$.ui.btnCreatePlatformTask,
            },
            [
              Icon({ name: "play", size: 16 }),
              computed(vm$.state.createCreating, (v) => {
                return v ? "创建中..." : "开始下载";
              }),
            ],
          ),
        ],
      ),
      Show({
        when: vm$.state.createError,
        ok() {
          return PlatformCreateErrorPanel(vm$.state.createError);
        },
      }),
      Show({
        when: computed(
          vm$.state.createProbeRaw,
          (raw) => raw?.workflow && !vm$.state.createProbe.value,
        ),
        ok() {
          return ProbePipelinePreview({
            raw: vm$.state.createProbeRaw,
          });
        },
      }),
      Show({
        when: vm$.state.createProbe,
        ok() {
          return View(
            {
              dataset: {
                t: "home-downloads-page-platform-create-panel-grid-content-preview-probe-pipeline-preview-label-select-label-input",
              },
              class:
                "mt-4 grid gap-4 border-t border-zinc-100 pt-4 dark:border-zinc-800 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-downloads-page-platform-create-panel-scroll-area-content-preview-probe-pipeline-preview",
                  },
                  class: "overflow-y-auto min-w-0 max-h-54",
                },
                [
                  Show({
                    when: computed(
                      vm$.state.createExisting,
                      (list) => Array.isArray(list) && list.length > 0,
                    ),
                    ok() {
                      return View(
                        {
                          dataset: {
                            t: "home-downloads-page-platform-create-panel-warning-panel-existing-task-text-text",
                          },
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
                  ProbePipelinePreview({
                    raw: vm$.state.createProbeRaw,
                  }),
                ],
              ),
              View(
                {
                  dataset: {
                    t: "home-downloads-page-platform-create-panel-stack-label-select-label-input",
                  },
                  class: "space-y-4",
                },
                [
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-platform-create-panel-label-select",
                      },
                    },
                    [
                      Label(
                        {
                          class:
                            "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                        },
                        ["下载内容"],
                      ),
                      Select({ store: vm$.ui.variantSelect }),
                    ],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-downloads-page-platform-create-panel-label-input",
                      },
                    },
                    [
                      Label(
                        {
                          class:
                            "mb-1 block text-xs font-medium text-zinc-500 dark:text-zinc-400",
                        },
                        ["文件名"],
                      ),
                      Input({ store: vm$.ui.filenameInput }),
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
      dataset: {
        t: "home-page-downloads-page-root-row-下载列表-管理视频号下载任务和本地文件-button-platform-create-panel-vm-state-tabs-list-scroll-view",
      },
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
          dataset: {
            t: "home-page-downloads-header-下载列表-管理视频号下载任务和本地文件-button-platform-create-panel-vm-state-tabs-list",
          },
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View(
            {
              dataset: {
                t: "home-page-downloads-row-下载列表-管理视频号下载任务和本地文件-button",
              },
              class: "flex flex-wrap items-center justify-between gap-3",
            },
            [
              View(
                {
                  dataset: {
                    t: "home-page-downloads-下载列表-管理视频号下载任务和本地文件",
                  },
                },
                [
                  View(
                    {
                      dataset: { t: "home-page-downloads-下载列表-heading" },
                      class:
                        "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                    },
                    ["下载列表"],
                  ),
                  View(
                    {
                      dataset: {
                        t: "home-page-downloads-管理视频号下载任务和本地文件-text",
                      },
                      class: "mt-1 text-sm text-zinc-500 dark:text-zinc-400",
                    },
                    ["管理视频号下载任务和本地文件"],
                  ),
                ],
              ),
              Button(
                {
                  store: vm$.ui.btn_refresh$,
                },
                [Icon({ name: "refresh-cw", size: 16 }), "刷新"],
              ),
            ],
          ),
          View(
            {
              dataset: { t: "home-page-downloads-platform-create-panel" },
              class: "mt-5",
            },
            [PlatformCreatePanel(vm$)],
          ),
          View(
            {
              dataset: { t: "home-page-downloads-row-vm-state-tabs-list" },
              class: "mt-4 flex flex-wrap gap-2",
            },
            [
              For({
                each: vm$.state.tabs,
                render(tab) {
                  return View(
                    {
                      dataset: {
                        t: "home-page-downloads-tab-label-computed-value",
                      },
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
                          dataset: {
                            t: "home-downloads-page-tab-status-count-badge",
                          },
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
            ],
          ),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View(
          {
            dataset: { t: "home-page-downloads-stack-remote-server-panel" },
            class: "space-y-3 p-6",
          },
          [
            RemoteServerPanel(vm$),
            Show({
              when: computed(vm$.state.error, (t) => !!t),
              ok() {
                return View(
                  {
                    dataset: {
                      t: "home-page-downloads-error-card-vm-state-error-text",
                    },
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
                    dataset: {
                      t: "home-page-downloads-row-icon-inbox-computed-value",
                    },
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
                return View(
                  {
                    dataset: {
                      t: "home-page-downloads-stack-vm-state-tasks-list",
                    },
                    class: "space-y-3",
                  },
                  [
                    For({
                      each: vm$.state.tasks,
                      render(task) {
                        return TaskCard(task, vm$);
                      },
                    }),
                    Show({
                      when: computed(vm$.state.noMore, (v) => !v),
                      ok() {
                        return View(
                          {
                            dataset: { t: "home-page-downloads-row-button" },
                            class: "flex justify-center py-4",
                          },
                          [
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
                          ],
                        );
                      },
                    }),
                  ],
                );
              },
            }),
          ],
        ),
      ]),
    ],
  );
}

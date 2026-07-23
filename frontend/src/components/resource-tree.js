export function formatBytes(value) {
  const n = Number(value || 0);
  if (!n) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = n;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

/**
 * 根据文件名后缀推断资源类型标签。
 */
export function resourceLabelByExt(name) {
  const ext = String(name || "").split(".").pop()?.toLowerCase() || "";
  if (/^jpe?g|png|gif|webp|svg|bmp|ico$/i.test(ext)) return "图片";
  if (/^mp4|avi|mkv|mov|webm|flv|wmv|m4v$/i.test(ext)) return "视频";
  if (/^mp3|wav|aac|flac|ogg|wma|m4a$/i.test(ext)) return "音频";
  if (/^srt|ass|vtt|sub|ssa$/i.test(ext)) return "字幕";
  if (/^html?$/i.test(ext)) return "HTML";
  if (/^css$/i.test(ext)) return "CSS";
  if (/^js$/i.test(ext)) return "JS";
  if (/^json$/i.test(ext)) return "JSON";
  if (/^pdf$/i.test(ext)) return "PDF";
  if (/^zip|rar|7z|tar|gz$/i.test(ext)) return "压缩包";
  return ext || "文件";
}

/**
 * 根据文件名后缀推断图标名称。
 */
export function resourceIconByExt(name) {
  const ext = String(name || "").split(".").pop()?.toLowerCase() || "";
  if (/^jpe?g|png|gif|webp|svg|bmp|ico$/i.test(ext)) return "image";
  if (/^mp4|avi|mkv|mov|webm|flv|wmv|m4v$/i.test(ext)) return "video";
  if (/^mp3|wav|aac|flac|ogg|wma|m4a$/i.test(ext)) return "audio";
  if (/^srt|ass|vtt|sub|ssa$/i.test(ext)) return "file-text";
  if (/^html?|css|js|json|xml$/i.test(ext)) return "code";
  return "file";
}

function sortTreeNodes(nodes) {
  nodes.sort((a, b) => {
    if (a.type !== b.type) return a.type === "directory" ? -1 : 1;
    return String(a.name || "").localeCompare(
      String(b.name || ""),
      "zh-Hans-CN",
    );
  });
  for (const node of nodes) {
    if (Array.isArray(node.children)) sortTreeNodes(node.children);
  }
}

/**
 * 从扁平的 files 数组中构建树结构。
 * files[i].name 如 "chapters/0001.html" 会被拆分为 chapters 目录 + 0001.html 文件。
 */
export function buildFileTree(files) {
  const root = { name: "", children: [] };
  for (const file of files || []) {
    const hasChildren =
      Array.isArray(file.children) && file.children.length > 0;
    if (hasChildren) {
      root.children.push({
        type: "directory",
        name: file.name || "目录",
        children: buildFileTree(file.children),
        file,
      });
    } else {
      const name = file.name || "";
      const parts = name.split("/").filter(Boolean);
      if (parts.length === 0) {
        root.children.push({ type: "file", name, file });
        continue;
      }
      let node = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        if (i === parts.length - 1) {
          node.children.push({ type: "file", name: part, file });
        } else {
          let dir = node.children.find(
            (c) => c.type === "directory" && c.name === part,
          );
          if (!dir) {
            dir = { type: "directory", name: part, children: [] };
            node.children.push(dir);
          }
          node = dir;
        }
      }
    }
  }
  sortTreeNodes(root.children);
  return root.children;
}

/**
 * 文件树节点渲染，递归。
 * @param {{ type: string, name: string, file?: any, children?: any[] }} node
 * @param {number} level
 */
export function ResourceTreeNode(node, level = 0) {
  const indent = `${Math.min(level * 20, 108)}px`;
  const branchClass =
    level > 0
      ? "relative before:absolute before:left-0 before:top-0 before:h-full before:border-l before:border-zinc-200 dark:before:border-zinc-800"
      : "";

  if (node.type === "directory") {
    return View(
      {
        dataset: { t: "resource-tree-dir-node" },
        as: "details",
        open: true,
        class: `space-y-0.5 ${branchClass}`,
      },
      [
        View(
          {
            as: "summary",
            class:
              "flex min-w-0 cursor-pointer select-none items-center gap-1.5 rounded-md py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-900",
            style: { "padding-left": indent },
          },
          [
            Icon({ name: "folder-closed", size: 14 }),
            View({ class: "min-w-0 flex-1 truncate" }, [node.name]),
          ],
        ),
        ...(node.children || []).map((child) =>
          ResourceTreeNode(child, level + 1),
        ),
      ],
    );
  }

  const file = node.file || {};
  const name = node.name || file.name || "文件";
  const size = Number(file.size || node.size || 0);
  const status = String(file.status || node.status || "").toLowerCase();
  const statusClass =
    status === "error" || status === "failed"
      ? "text-red-500 dark:text-red-300"
      : "";
  const label = resourceLabelByExt(name);

  return View(
    {
      dataset: { t: "resource-tree-file-node" },
      class: `flex min-w-0 items-center gap-2 rounded-md py-1 pr-2 text-xs hover:bg-zinc-50 dark:hover:bg-zinc-900 ${branchClass}`,
      style: { "padding-left": indent },
    },
    [
      Icon({
        name: resourceIconByExt(name),
        size: 14,
      }),
      View({ class: `min-w-0 flex-1 truncate ${statusClass}` }, [
        name,
      ]),
      View(
        {
          class:
            "shrink-0 rounded bg-zinc-100 px-1.5 py-0.5 text-[10px] text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400",
        },
        [label],
      ),
      size
        ? View(
            {
              class: "shrink-0 text-zinc-400 dark:text-zinc-500",
            },
            [formatBytes(size)],
          )
        : null,
    ].filter(Boolean),
  );
}

/**
 * 资源文件树组件。
 * 优先使用 tree 字段（后端聚合好的树），否则从 files 扁平数组构建。
 *
 * @param {{ tree?: any, files?: any[], title?: string, class?: string }} props
 */
export function ResourceTree(props) {
  const { tree, files, title = "资源文件", class: cls = "" } = props || {};
  const resolvedTree = tree && tree.children ? tree.children : buildFileTree(files);
  const count = tree
    ? ((tree.node_count != null ? tree.node_count : undefined) ??
      (files ? files.length : undefined))
    : (files || []).length;

  return View(
    {
      class: `rounded-md border border-zinc-200 bg-zinc-50/60 p-2 dark:border-zinc-800 dark:bg-zinc-950/60 ${cls}`,
    },
    [
      View(
        {
          class:
            "mb-1 flex items-center justify-between gap-2 text-xs font-medium text-zinc-500 dark:text-zinc-400",
        },
        [
          View({ class: "flex min-w-0 items-center gap-1.5" }, [
            Icon({ name: "folder-tree", size: 14 }),
            title,
          ]),
          count != null
            ? View(
                { class: "shrink-0 text-zinc-400 dark:text-zinc-500" },
                [`${count} 个资源`],
              )
            : null,
        ],
      ),
      Show({
        when: resolvedTree.length > 0,
        ok() {
          return View(
            { class: "space-y-0.5" },
            resolvedTree.map((node) => ResourceTreeNode(node)),
          );
        },
      }),
    ],
  );
}

/**
 * Shared tree utilities, FileTreeView, and FileTreeRow.
 *
 * Used by both downloadv2.components.js (preview dialog) and scraper.js
 * (download-info resource list).
 */

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

function safe_string(value, fallback) {
  if (value === undefined || value === null || value === "") {
    return fallback || "";
  }
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

export function resource_file_icon(name) {
  const extension = String(name || "")
    .split(".")
    .pop()
    .toLowerCase();
  if (/^(jpe?g|png|gif|webp|svg|bmp|ico)$/.test(extension)) {
    return "file-image";
  }
  if (/^(mp4|avi|mkv|mov|webm|flv|wmv|m4v)$/.test(extension)) {
    return "file-play";
  }
  if (/^(mp3|wav|aac|flac|ogg|wma|m4a)$/.test(extension)) {
    return "file-volume";
  }
  if (/^(html?|css|js|json|xml)$/.test(extension)) return "file-code";
  return "file";
}

export function format_file_size(bytes) {
  if (!bytes || bytes <= 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes;
  for (const unit of units) {
    value /= 1024;
    if (value < 1024 || unit === "TB") return `${value.toFixed(1)} ${unit}`;
  }
  return `${bytes} B`;
}

export function tree_compare(a, b) {
  if (a.type !== b.type) return a.type === "directory" ? -1 : 1;
  return String(a.name || "").localeCompare(String(b.name || ""), undefined, {
    numeric: true,
    sensitivity: "base",
  });
}

export function count_tree_files(node) {
  if (!node || node.type !== "directory") return node ? 1 : 0;
  return (node.children || []).reduce((count, child) => {
    return count + count_tree_files(child);
  }, 0);
}

export function count_tree_children(node) {
  if (!node || node.type !== "directory") return 0;
  return (node.children || []).length;
}

/** Assign stable _path to every node for collapse tracking. */
export function assign_tree_paths(nodes, prefix) {
  for (const node of nodes) {
    node._path = prefix ? `${prefix}/${node.name}` : node.name;
    if (node.type === "directory" && node.children) {
      assign_tree_paths(node.children, node._path);
    }
  }
}

/**
 * Shallow-clone tree nodes so assign_tree_paths can mutate _path
 * without touching the original tree.  Preserves `resource` references.
 */
export function clone_tree_nodes(nodes) {
  return nodes.map((node) => {
    if (node.type === "directory" && node.children) {
      return {
        type: node.type,
        name: node.name,
        children: clone_tree_nodes(node.children),
      };
    }
    // file node — shallow copy, keep `resource` as-is
    return Object.assign({}, node);
  });
}

/**
 * Build a tree from a flat array of resources.
 *
 * Accepts either:
 *  - a raw array of resources (each having a `name` field with `/` separators)
 *  - a preview object `{ tree, resources }` (legacy compat for downloadv2)
 *
 * Options:
 *  - `name_key`: field(s) to read the name from (default uses common keys)
 *  - `fallback_label`: template for unnamed resources (receives index+1)
 */
export function build_resource_tree(input, opts) {
  // Legacy preview-object path
  if (input && !Array.isArray(input)) {
    if (input.tree && typeof input.tree === "object") {
      return input.tree;
    }
    if (Array.isArray(input.resources)) {
      return build_resource_tree(input.resources, opts);
    }
    return { type: "directory", name: "", children: [] };
  }

  const resources = Array.isArray(input) ? input : [];
  const root = { type: "directory", name: "", children: [] };

  resources.forEach((resource, index) => {
    const name = safe_string(
      resource &&
        (resource.display_name ||
          resource.name ||
          resource.filename ||
          resource.file_name ||
          resource.title),
      `资源 ${index + 1}`,
    );
    const parts = name.split("/").filter(Boolean);
    const file_name = parts.pop() || name;
    let parent = root;

    parts.forEach((part) => {
      let directory = parent.children.find((node) => {
        return node.type === "directory" && node.name === part;
      });
      if (!directory) {
        directory = { type: "directory", name: part, children: [] };
        parent.children.push(directory);
      }
      parent = directory;
    });
    parent.children.push({
      type: "file",
      name: file_name,
      kind: resource && resource.kind,
      size: resource && resource.size,
      endpoints: resource && resource.endpoints,
      resource,
    });
  });
  return root;
}

/** Flatten tree into a visible-rows array, respecting collapsed state. */
export function flatten_tree(nodes, collapsed, depth, output) {
  for (const node of nodes) {
    const path = node._path || node.name || "";
    output.push({ node, depth, path });
    if (node.type === "directory" && !collapsed.has(path)) {
      const children = (node.children || []).slice();
      children.sort(tree_compare);
      flatten_tree(children, collapsed, depth + 1, output);
    }
  }
  return output;
}

// ---------------------------------------------------------------------------
// FileTreeRow — default row renderer
// ---------------------------------------------------------------------------

export function FileTreeRow(props) {
  const { row, collapsed_, onToggle } = props;
  const { node, depth } = row.value || row;

  const is_directory = node && node.type === "directory";
  const is_collapsed = is_directory && collapsed_.value.has(node._path);
  const indent_px = `${Math.min(depth * 18, 180)}px`;

  if (is_directory) {
    return View(
      {
        class: "file-tree-row is-directory",
        style: { "padding-left": indent_px },
        attributes: { n: "file-tree-dir", title: node._path || node.name },
        onClick() {
          onToggle(node._path);
        },
      },
      [
        View({ class: "file-tree-caret" }, [is_collapsed ? "›" : "⌄"]),
        View({ class: "file-tree-icon" }, [
          Timeless.Icon({ name: "folder", size: 16 }),
        ]),
        View({ class: "file-tree-name dm-truncate" }, [node.name || "根目录"]),
        View({ class: "file-tree-meta" }, [`${count_tree_children(node)} 项`]),
      ],
    );
  }

  return View(
    {
      class: "file-tree-row",
      style: { "padding-left": indent_px },
      attributes: { n: "file-tree-file", title: node._path || node.name },
    },
    [
      View({ class: "file-tree-caret" }),
      View({ class: "file-tree-icon" }, [
        Timeless.Icon({
          name: resource_file_icon(node && node.name),
          size: 16,
        }),
      ]),
      View({ class: "file-tree-name dm-truncate" }, [
        (node && node.name) || "文件",
      ]),
      View({ class: "file-tree-meta" }, [format_file_size(node && node.size)]),
    ],
  );
}

/**
 * Resolve the DOM element from a Timeless onMounted event.
 * event.target may be a Timeless component — walk via get$elm()/$elm.
 */
export function mounted_element(event) {
  let target = event && event.target ? event.target : event;
  for (let depth = 0; depth < 4; depth += 1) {
    if (target && target.nodeType === 1) return target;
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

// ---------------------------------------------------------------------------
// FileTreeView — reusable virtual-scrolling tree component
// ---------------------------------------------------------------------------

/**
 * @param {Object} props
 * @param {Signal} props.resources_ - reactive signal of flat resource array
 * @param {Function} [props.renderRow] - custom row renderer (row, collapsed_, onToggle) => View
 * @param {number} [props.itemHeight] - row height in px (default 32)
 * @param {number} [props.maxHeight] - scroll container max height (default 360)
 */
export function FileTreeView(props) {
  const { resources_, renderRow, itemHeight = 32, maxHeight = 360 } = props;

  const collapsed_ = ref(new Set());
  const flat_rows_ = refarr([]);
  const scroll_top_ = ref(0);
  const viewport_height_ = ref(maxHeight);
  const scroll_view$ = new Timeless.vm.ScrollViewCore({
    onScroll(value) {
      const scrollTop = Number(value?.scrollTop ?? value?.target?.scrollTop) || 0;
      const clientHeight = Number(value?.clientHeight ?? value?.target?.clientHeight) || 0;
      scroll_top_.as(scrollTop);
      if (clientHeight > 0) {
        viewport_height_.as(clientHeight);
      }
    },
  });

  const tree_ = computed(resources_, (resources) => {
    return build_resource_tree(resources);
  });
  const tree_nodes_ = computed(tree_, (tree) => {
    return tree && Array.isArray(tree.children) ? tree.children : [];
  });
  const assigned_nodes_ = computed(tree_nodes_, (nodes) => {
    const safe = Array.isArray(nodes) ? nodes : [];
    const copy = clone_tree_nodes(safe);
    assign_tree_paths(copy, "");
    return copy;
  });

  combine({ nodes: assigned_nodes_, collapsed: collapsed_ }, (t) => {
    const { nodes, collapsed } = t;
    const safe = Array.isArray(nodes) ? nodes : [];
    const rows = flatten_tree(
      safe.slice().sort(tree_compare),
      collapsed,
      0,
      [],
    );
    flat_rows_.as(rows, { reset: true });
  });

  function onToggle(path) {
    const next = new Set(collapsed_.value);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    collapsed_.value = next;
  }

  return View({}, [
    Show({
      when: computed(tree_nodes_, (nodes) => nodes.length > 0),
      ok() {
        return Timeless.ui.ScrollViewPrimitive.Root(
          {
            store: scroll_view$,
            class: "file-tree-scroll",
            style: { "max-height": `${maxHeight}px`, overflow: "auto" },
            onUnmounted() {
              scroll_view$.destroy();
            },
          },
          [
            VirtualListView({
              class: "file-tree-virtual",
              attributes: { n: "file-tree-virtual" },
              style: { "min-height": "100%", overflow: "visible" },
              each: flat_rows_,
              key: "path",
              size: 20,
              buffer: 6,
              itemHeight,
              externalScroll: true,
              scrollTop: scroll_top_,
              viewportHeight: viewport_height_,
              onScrollTopAdjust(delta) {
                const next = Math.max(0, scroll_top_.value + delta);
                scroll_top_.as(next);
                scroll_view$.setScrollTop(next);
              },
              render(row) {
                const render_row = renderRow || FileTreeRow;
                return render_row({ row, collapsed_, onToggle });
              },
            }),
          ],
        );
      },
      else() {
        return View({}, ["No files"]);
      },
    }),
  ]);
}

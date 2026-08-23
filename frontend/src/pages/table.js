function table_class_names(values) {
  return values.filter(Boolean).join(" ");
}

function table_source(value) {
  return Boolean(
    value &&
      typeof value === "object" &&
      typeof value.subscribe === "function",
  );
}

function table_has_rows(rows) {
  const has_rows = (items) => Array.isArray(items) && items.length > 0;
  return table_source(rows) ? computed(rows, has_rows) : has_rows(rows);
}

function table_item_value(item_) {
  return item_ && item_.value !== undefined ? item_.value : item_;
}

function table_resolve(value, ...args) {
  return typeof value === "function" ? value(...args) : value;
}

function table_value(value) {
  return table_source(value) ? value.value : value;
}

function table_loading_visible(status, loading) {
  const visible = (current_status, current_loading) => {
    return current_status !== "initial" && Boolean(current_loading);
  };
  if (table_source(status) && table_source(loading)) {
    return combine({ status, loading }, (state) =>
      visible(state.status, state.loading),
    );
  }
  if (table_source(status)) {
    return computed(status, (current_status) =>
      visible(current_status, table_value(loading)),
    );
  }
  if (table_source(loading)) {
    return computed(loading, (current_loading) =>
      visible(table_value(status), current_loading),
    );
  }
  return visible(status, loading);
}

function table_children(value) {
  if (value === undefined || value === null) return [];
  return Array.isArray(value) ? value : [value];
}

function table_state_icon(value, fallback_name, fallback_size) {
  const icon = table_resolve(value);
  if (icon === false || icon === null) return null;
  if (icon === undefined) {
    return Timeless.Icon({ name: fallback_name, size: fallback_size });
  }
  return typeof icon === "string"
    ? Timeless.Icon({ name: icon, size: fallback_size })
    : icon;
}

function TableState(props) {
  const icon = table_resolve(props.icon);
  const title = table_resolve(props.title);
  const description = table_resolve(props.description);
  const action = table_resolve(props.action);
  const children = [];

  if (icon) children.push(icon);
  if (title !== undefined && title !== null) {
    children.push(
      View(
        {
          class: "wx-table-state-title wx-content-state-title",
          attributes: { n: `${props.name}-${props.state}-title` },
        },
        table_children(title),
      ),
    );
  }
  if (description !== undefined && description !== null) {
    children.push(
      View(
        {
          class: "wx-table-state-text wx-content-state-text",
          attributes: {
            n: `${props.name}-${props.state}-${props.descriptionSuffix}`,
          },
        },
        table_children(description),
      ),
    );
  }
  children.push(...table_children(action));

  return View(
    {
      class: table_class_names([
        "wx-table-state",
        "wx-content-state",
        props.class,
      ]),
      attributes: {
        n: `${props.name}-${props.state}`,
        ...(props.attributes || {}),
      },
    },
    children,
  );
}

function TableRetryAction(props) {
  const retry = props.retry;
  if (!retry || !retry.store) return null;

  return Button(
    {
      store: retry.store,
      class: table_class_names(["wx-table-state-action", retry.class]),
      attributes: {
        n: `${props.name}-retry`,
        type: "button",
        ...(retry.attributes || {}),
      },
      prefix:
        retry.icon === false
          ? null
          : Timeless.Icon({
              name: retry.icon || "refresh-cw",
              size: retry.iconSize || 16,
            }),
    },
    [
      View(
        {
          class:
            "wx-table-state-action-label wx-content-action-label",
        },
        table_children(retry.label ?? "重试"),
      ),
    ],
  );
}

function TableEmpty(props) {
  const action =
    typeof props.renderEmptyAction === "function"
      ? props.renderEmptyAction()
      : props.emptyAction;
  return TableState({
    name: props.name,
    state: "empty",
    class: props.emptyClass,
    attributes: props.emptyAttributes,
    icon: table_state_icon(props.emptyIcon, "inbox", 36),
    title: props.emptyTitle ?? "暂无数据",
    description: props.emptyDescription,
    descriptionSuffix: "description",
    action,
  });
}

function TableError(props) {
  const action =
    typeof props.renderErrorAction === "function"
      ? props.renderErrorAction(props.error)
      : props.errorAction === undefined
        ? TableRetryAction(props)
        : props.errorAction;
  return TableState({
    name: props.name,
    state: "error",
    class: props.errorClass,
    attributes: {
      role: "alert",
      ...(props.errorAttributes || {}),
    },
    icon: table_state_icon(props.errorIcon, "circle-alert", 32),
    title: props.errorTitle ?? "数据加载失败",
    description: props.errorMessage ?? props.error,
    descriptionSuffix: "message",
    action,
  });
}

function TableSelectionCheckbox(props) {
  const state_ = props.state;

  function toggle(event) {
    if (event && typeof event.stopPropagation === "function") {
      event.stopPropagation();
    }
    if (typeof props.onToggle === "function") {
      props.onToggle(event);
    }
  }

  return View(
    {
      role: "checkbox",
      tabIndex: "0",
      class: props.class || "",
      style: {
        width: `${props.size + 4}px`,
        height: `${props.size + 4}px`,
        display: "inline-flex",
        "align-items": "center",
        "justify-content": "center",
        cursor: "pointer",
        "user-select": "none",
        flex: "0 0 auto",
        ...(props.style || {}),
      },
      attributes: {
        n: props.name,
        "aria-label": props.ariaLabel,
        "aria-checked": computed(state_, (state) => {
          if (state.indeterminate) return "mixed";
          return state.checked ? "true" : "false";
        }),
      },
      onClick: toggle,
      onKeyDown(event) {
        if (event.key === " " || event.key === "Enter") {
          event.preventDefault();
          toggle(event);
        }
      },
    },
    [
      View(
        {
          attributes: { n: `${props.name}-indicator` },
          style: computed(state_, (state) => {
            const active = state.checked || state.indeterminate;
            return {
              width: `${props.size}px`,
              height: `${props.size}px`,
              "box-sizing": "border-box",
              "border-radius": "4px",
              border: `1px solid ${active ? "var(--dm-color-primary-fill)" : "var(--dm-color-border)"}`,
              background: active
                ? "var(--dm-color-primary-fill)"
                : "transparent",
              color: "var(--dm-color-on-primary)",
              display: "inline-flex",
              "align-items": "center",
              "justify-content": "center",
            };
          }),
        },
        [
          Show({
            when: computed(state_, (state) => state.indeterminate),
            ok() {
              return View({
                attributes: { n: `${props.name}-mixed-icon` },
                style: {
                  width: `${Math.max(8, props.size - 8)}px`,
                  height: "2px",
                  "border-radius": "1px",
                  background: "currentColor",
                },
              });
            },
            else() {
              return Show({
                when: computed(state_, (state) => state.checked),
                ok() {
                  return Timeless.Icon({
                    name: "check",
                    size: Math.max(12, props.size - 4),
                  });
                },
              });
            },
          }),
        ],
      ),
    ],
  );
}

function TableSelectionHeaderCell(props) {
  const row_selection = props.rowSelection;
  return View(
    {
      class: table_class_names([
        "wx-table-selection-cell",
        row_selection.headerClass,
      ]),
      style: {
        display: "flex",
        "align-items": "center",
        "justify-content": "center",
        "min-width": "0",
        ...(row_selection.headerStyle || {}),
      },
      attributes: {
        n: `${props.name}-selection-header`,
        role: "columnheader",
      },
    },
    [
      TableSelectionCheckbox({
        state: row_selection.headerState,
        name: `${props.name}-select-all-checkbox`,
        ariaLabel: row_selection.allAriaLabel || "全选表格数据",
        size: row_selection.size || 18,
        style: row_selection.checkboxStyle,
        onToggle: row_selection.onSelectAll,
      }),
    ],
  );
}

function TableHeaderCell(props) {
  const column = props.column;
  return View(
    {
      class: table_class_names([
        "wx-table-header-cell",
        props.cellClass,
        column.headerClass,
      ]),
      style: column.headerStyle || {},
      attributes: {
        n: `${props.name}-${column.name}-header`,
        role: "columnheader",
        ...(column.headerAttributes || {}),
      },
    },
    [
      View(
        {
          class: "wx-table-head-label",
          style: {
            overflow: "hidden",
            "text-overflow": "ellipsis",
            "white-space": "nowrap",
          },
          attributes: { n: `${props.name}-${column.name}-header-label` },
        },
        [column.title ?? column.label],
      ),
    ],
  );
}

function TableHeader(props) {
  const cells = props.columns.map((column) =>
    TableHeaderCell({
      name: props.name,
      cellClass: props.headerCellClass,
      column,
    }),
  );
  if (props.rowSelection) {
    cells.unshift(
      TableSelectionHeaderCell({
        name: props.name,
        rowSelection: props.rowSelection,
      }),
    );
  }
  return View(
    {
      class: table_class_names([
        "wx-table-row",
        "wx-table-header",
        props.headerClass,
      ]),
      attributes: { n: `${props.name}-header`, role: "row" },
    },
    cells,
  );
}

function TableSelectionCell(props) {
  const row_selection = props.rowSelection;
  const enabled =
    typeof row_selection.enabled !== "function" ||
    row_selection.enabled(props.item, props.itemSource);
  return View(
    {
      class: table_class_names([
        "wx-table-selection-cell",
        row_selection.cellClass,
      ]),
      style: {
        display: "flex",
        "align-items": "center",
        "justify-content": "center",
        "min-width": "0",
        ...(row_selection.cellStyle || {}),
      },
      attributes: {
        n: `${props.name}-selection-cell`,
        role: "cell",
      },
    },
    enabled
      ? [
          TableSelectionCheckbox({
            state: row_selection.itemState(props.item, props.itemSource),
            name: `${props.name}-row-checkbox`,
            ariaLabel:
              typeof row_selection.itemAriaLabel === "function"
                ? row_selection.itemAriaLabel(props.item)
                : row_selection.itemAriaLabel || "选择表格数据",
            size: row_selection.size || 18,
            style: row_selection.checkboxStyle,
            onToggle(event) {
              row_selection.onSelect(props.item, event, props.itemSource);
            },
          }),
        ]
      : [],
  );
}

function TableDataCell(props) {
  const column = props.column;
  const context = {
    itemSource: props.itemSource,
    name: props.name,
  };
  return View(
    {
      class: table_class_names([
        "wx-table-cell",
        table_resolve(column.cellClass, props.item, context),
      ]),
      style: table_resolve(column.cellStyle, props.item, context) || {},
      attributes: {
        n: `${props.name}-${column.name}-cell`,
        role: "cell",
        ...(table_resolve(column.cellAttributes, props.item, context) || {}),
      },
    },
    table_children(
      typeof column.render === "function"
        ? column.render(props.item, context)
        : props.item && props.item[column.dataIndex || column.name],
    ),
  );
}

function TableDataRow(props) {
  const item = table_item_value(props.itemSource);
  if (
    typeof props.isPlaceholder === "function" &&
    props.isPlaceholder(item)
  ) {
    if (typeof props.onPlaceholder === "function") {
      props.onPlaceholder(item, props.itemSource);
    }
    return typeof props.renderPlaceholderRow === "function"
      ? props.renderPlaceholderRow(item, props.itemSource)
      : null;
  }

  const row_props =
    typeof props.onRow === "function"
      ? props.onRow(item, props.itemSource) || {}
      : {};
  const {
    attributes: row_attributes,
    class: row_class,
    style: row_style,
    ...row_events
  } = row_props;
  const cells = props.columns.map((column) =>
    TableDataCell({
      name: props.name,
      column,
      item,
      itemSource: props.itemSource,
    }),
  );
  if (props.rowSelection) {
    cells.unshift(
      TableSelectionCell({
        name: props.name,
        item,
        itemSource: props.itemSource,
        rowSelection: props.rowSelection,
      }),
    );
  }

  return View(
    {
      ...row_events,
      class: table_class_names([
        "wx-table-row",
        table_resolve(props.rowClass, item, props.itemSource),
        row_class,
      ]),
      style: row_style || {},
      attributes: {
        n: `${props.name}-row`,
        role: "row",
        ...(row_attributes || {}),
      },
    },
    cells,
  );
}

function TableList(props) {
  return View(
    {
      class: table_class_names(["wx-table-list", props.listClass]),
      style: {
        overflow: "auto",
        ...(props.listStyle || {}),
      },
      attributes: { n: `${props.name}-list` },
    },
    [
      Show({
        when: table_has_rows(props.rows),
        ok() {
          return For({
            key: props.rowKey,
            each: props.rows,
            render(item_) {
              return TableDataRow({ ...props, itemSource: item_ });
            },
          });
        },
        else() {
          return typeof props.renderEmpty === "function"
            ? props.renderEmpty()
            : null;
        },
      }),
    ],
  );
}

function TableVirtualList(props) {
  return View(
    {
      class: table_class_names(["wx-table-list", props.listClass]),
      style: props.listStyle,
      attributes: { n: `${props.name}-list` },
    },
    [
      Show({
        when: table_has_rows(props.rows),
        ok() {
          return Show({
            when: props.renderEnabled,
            ok() {
              return VirtualListView({
                attributes: { n: `${props.name}-virtual-list` },
                style: {
                  height: "100%",
                  "max-height": "100%",
                  overflow: "auto",
                  position: "relative",
                  "box-sizing": "border-box",
                  "background-color": "transparent",
                  ...(props.virtualListStyle || {}),
                },
                key: props.rowKey,
                size: props.size,
                buffer: props.buffer,
                gutter: props.gutter,
                itemHeight: props.itemHeight,
                paddingBottom: props.paddingBottom,
                each: props.rows,
                onMounted: props.onListMounted,
                onScroll: props.onListScroll,
                render(item_) {
                  return TableDataRow({ ...props, itemSource: item_ });
                },
              });
            },
          });
        },
        else() {
          return typeof props.renderEmpty === "function"
            ? props.renderEmpty()
            : null;
        },
      }),
    ],
  );
}

function TableLoading(props) {
  const render_skeleton_row = props.renderSkeletonRow;
  return View(
    {
      class: table_class_names([
        "wx-table-list",
        props.listClass,
        props.loadingClass,
      ]),
      style: {
        overflow: "auto",
        ...(props.listStyle || {}),
      },
      attributes: { n: `${props.name}-loading`, role: "status" },
    },
    typeof render_skeleton_row === "function"
      ? Array.from({ length: props.skeletonCount }, () =>
          render_skeleton_row(),
        )
      : [],
  );
}

function TableLoadingOverlay(props) {
  return View(
    {
      class: "wx-table-loading-overlay",
      attributes: {
        n: `${props.name}-loading-overlay`,
        role: "status",
        "aria-label": "列表加载中",
      },
    },
    [
      View(
        {
          class: "wx-table-loading-indicator",
          attributes: { n: `${props.name}-loading-indicator` },
        },
        [
          View({
            class: "dm-ui-spinner",
            attributes: {
              n: `${props.name}-loading-spinner`,
              "aria-hidden": "true",
            },
          }),
          "加载中…",
        ],
      ),
    ],
  );
}

function TablePanel(props, render_list) {
  return View(
    {
      class: table_class_names(["wx-table-panel", props.panelClass]),
      attributes: {
        n: `${props.name}-panel`,
        role: "table",
        ...(props.panelAttributes || {}),
      },
    },
    [TableHeader(props), render_list(props)],
  );
}

function table_render(props, render_list) {
  const name = props.name || "table";
  const status = props.status || "normal";
  const show_header_when_empty = props.showHeaderWhenEmpty !== false;
  const table_props = {
    ...props,
    name,
    columns: props.columns || [],
    panelClass:
      props.panelClass ||
      "wx-content-rows wx-content-history-rows dm-panel",
    headerClass:
      props.headerClass || "wx-content-row wx-content-row-head",
    headerCellClass:
      props.headerCellClass || "wx-content-row-head-cell",
    listClass: props.listClass || "wx-content-history-list",
    rowClass: props.rowClass || "wx-content-row",
  };

  function render_panel() {
    return TablePanel(
      { ...table_props, renderEmpty: render_empty },
      render_list,
    );
  }

  function render_empty() {
    return typeof props.renderEmpty === "function"
      ? props.renderEmpty()
      : TableEmpty(table_props);
  }

  function render_initial() {
    return TablePanel(table_props, () =>
      TableLoading({
        ...table_props,
        loadingClass: props.loadingClass,
        skeletonCount: props.skeletonCount || 8,
      }),
    );
  }

  function render_error_state() {
    return typeof props.renderError === "function"
      ? props.renderError(props.error)
      : TableError(table_props);
  }

  function render_error() {
    return TablePanel(
      { ...table_props, rows: [], renderEmpty: render_error_state },
      render_list,
    );
  }

  return View(
    {
      class: table_class_names([
        "wx-table-container",
        props.containerClass || "wx-content-main dm-container",
      ]),
      attributes: {
        n: `${name}-container`,
        ...(props.containerAttributes || {}),
      },
    },
    [
      Match({
        when: status,
        cases: {
          initial: render_initial,
          empty() {
            return show_header_when_empty ? render_panel() : render_empty();
          },
          error: render_error,
          normal: render_panel,
        },
      }),
      Show({
        when: table_loading_visible(status, props.loading),
        ok() {
          return TableLoadingOverlay({ name });
        },
      }),
    ],
  );
}

export function Table(props) {
  return table_render(props, TableList);
}

export function TableWithVirtualList(props) {
  return table_render(
    {
      ...props,
      renderEnabled:
        typeof props.renderEnabled === "undefined" ? true : props.renderEnabled,
      rowKey: props.rowKey || "id",
      size: props.size || 10,
      buffer: props.buffer ?? 6,
      gutter: props.gutter ?? 0,
      itemHeight: props.itemHeight || 72,
      paddingBottom: props.paddingBottom ?? 0,
    },
    TableVirtualList,
  );
}

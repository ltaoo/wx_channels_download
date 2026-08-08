/**
 * macOS-style column file picker.
 *
 * FilePickerModel owns all navigation and async state. FilePicker and
 * FilePickerDialog only render that state and forward user intent to it.
 *
 * A loader receives `(folder, context)` and may return an array, or an object
 * containing `files`, `items`, `list`, or `dataSource`.
 */
var APIOrigin = WXEnv.get("apiOrigin");

function file_picker_normalize_file(file, parent) {
  const value = file || {};
  const name = String(value.name || value.file_name || value.title || "");
  const parentPath = parent && parent.path ? String(parent.path) : "";
  const path = String(
    typeof value.path === "string"
      ? value.path
      : value.file_path ||
          (parentPath === "/"
            ? `/${name}`
            : `${parentPath.replace(/\/$/, "")}/${name}`),
  );
  const rawType = value.type || value.kind || "";
  const type = String(rawType).toLowerCase();
  const isDirectory = Boolean(
    value.isDirectory ||
    value.is_directory ||
    value.is_dir ||
    value.isDir ||
    value.directory ||
    rawType === 2 ||
    type === "folder" ||
    type === "directory" ||
    type === "dir",
  );

  return {
    ...value,
    id: String(value.id || value.file_id || path || name),
    name,
    path,
    isDirectory,
    type: isDirectory ? "folder" : "file",
  };
}

function file_picker_response_files(response) {
  if (Array.isArray(response)) return response;
  const data = response && response.data ? response.data : response;
  if (Array.isArray(data)) return data;
  if (!data) return [];
  return (
    data.files ||
    data.items ||
    data.list ||
    data.dataSource ||
    data.children ||
    []
  );
}

function file_picker_index_value(index) {
  const value = index && index.__is_ref ? index.value : index;
  const number = Number(value);
  return Number.isInteger(number) ? number : -1;
}

function file_picker_accepts(file, accept) {
  if (!file || file.isDirectory || !accept) return true;
  if (typeof accept === "function") return accept(file) !== false;

  const patterns = (Array.isArray(accept) ? accept : String(accept).split(","))
    .flatMap((pattern) => String(pattern).split(","))
    .map((pattern) => pattern.trim().toLowerCase())
    .filter(Boolean);
  if (!patterns.length) return true;

  const name = String(file.name || "").toLowerCase();
  const path = String(file.path || name).toLowerCase();
  const mime = String(
    file.mimeType || file.mime_type || file.mime || file.contentType || "",
  ).toLowerCase();

  return patterns.some((pattern) => {
    if (pattern === "*" || pattern === "*/*") return true;
    if (pattern.startsWith(".")) return path.endsWith(pattern);
    if (pattern.endsWith("/*")) {
      return mime.startsWith(pattern.slice(0, -1));
    }
    if (pattern.includes("/")) return mime === pattern;
    if (pattern.includes("*")) {
      const expression = pattern
        .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
        .replace(/\*/g, ".*");
      return new RegExp(`^${expression}$`, "i").test(name);
    }
    return name === pattern || path.endsWith(`.${pattern.replace(/^\./, "")}`);
  });
}

function FilePickerModel(props = {}) {
  const loadFiles =
    props.loadFiles || props.loadChildren || props.listFiles || props.service;
  if (typeof loadFiles !== "function") {
    throw new TypeError("FilePickerModel requires a loadFiles function");
  }

  const root = file_picker_normalize_file(
    props.root || {
      id: "root",
      name: props.rootName || "文件",
      path: "/",
      isDirectory: true,
    },
    null,
  );
  const columns_ = refarr([]);
  const paths_ = refarr([root]);
  const selected_ = ref(null);
  const selected_position_ = ref(null);
  const initialized_ = ref(false);
  const loading_ = ref(false);
  const error_ = ref(null);
  const listeners = [];
  let destroyed = false;
  let requestSequence = 0;

  function emitStateChange() {
    const snapshot = model.getState();
    listeners.slice().forEach((listener) => listener(snapshot));
    if (typeof props.onStateChange === "function")
      props.onStateChange(snapshot);
  }

  function updateLoading() {
    loading_.as(columns_.some((column) => column.loading.value));
  }

  function makeColumn(folder) {
    return {
      id: `file-picker-column-${requestSequence + 1}`,
      folder,
      items: refarr([]),
      selectedId: ref(null),
      loading: ref(true),
      error: ref(null),
      requestId: 0,
    };
  }

  async function loadColumn(folder, columnIndex) {
    const column = columns_.value[columnIndex];
    if (!column) return null;
    const requestId = ++requestSequence;
    column.requestId = requestId;
    column.loading.as(true);
    column.error.as(null);
    error_.as(null);
    updateLoading();
    emitStateChange();

    try {
      const response = await loadFiles(folder, {
        columnIndex,
        path: folder.path,
        model,
      });
      if (
        destroyed ||
        column.requestId !== requestId ||
        columns_.value[columnIndex] !== column
      ) {
        return null;
      }
      const files = file_picker_response_files(response).map((file) =>
        file_picker_normalize_file(file, folder),
      );
      column.items.as(files);
      initialized_.as(true);
      return files;
    } catch (cause) {
      if (
        destroyed ||
        column.requestId !== requestId ||
        columns_.value[columnIndex] !== column
      ) {
        return null;
      }
      const nextError =
        cause instanceof Error ? cause : new Error(String(cause));
      column.error.as(nextError);
      error_.as(nextError);
      if (typeof props.onError === "function") props.onError(nextError);
      return null;
    } finally {
      if (
        !destroyed &&
        column.requestId === requestId &&
        columns_.value[columnIndex] === column
      ) {
        column.loading.as(false);
        updateLoading();
        emitStateChange();
      }
    }
  }

  function appendColumn(folder) {
    const column = makeColumn(folder);
    columns_.push(column);
    syncPathsFromColumns();
    column.ready = loadColumn(folder, columns_.value.length - 1);
    return column;
  }

  function syncPathsFromColumns() {
    console.log("[]syncPathsFromColumns", columns_.length);
    paths_.as(columns_.value.map((column) => column.folder));
  }

  function truncateAfter(columnIndex) {
    const removed = columns_.value.slice(columnIndex + 1);
    removed.forEach((column) => {
      column.requestId = ++requestSequence;
    });
    if (removed.length) {
      columns_.splice(columnIndex + 1, removed.length);
    }
    syncPathsFromColumns();
  }

  const methods = {
    init() {
      if (columns_.value.length) {
        return (
          columns_.value[0].ready ||
          Promise.resolve(columns_.value[0].items.value)
        );
      }
      const column = appendColumn(root);
      return column.ready;
    },
    reset(nextRoot) {
      columns_.value.forEach((column) => {
        column.requestId = ++requestSequence;
      });
      const normalizedRoot = nextRoot
        ? file_picker_normalize_file(nextRoot, null)
        : root;
      columns_.as([]);
      selected_.as(null);
      selected_position_.as(null);
      initialized_.as(false);
      error_.as(null);
      const column = appendColumn(normalizedRoot);
      emitStateChange();
      return column.ready;
    },
    select(file, position) {
      const positionColumn =
        position && !Array.isArray(position) ? position.column : null;
      const columnIndex = positionColumn
        ? columns_.value.indexOf(positionColumn)
        : file_picker_index_value(
            Array.isArray(position) ? position[0] : position,
          );
      const column = columns_.value[columnIndex];
      if (!column) {
        return;
      }
      const requestedRowIndex = file_picker_index_value(
        Array.isArray(position)
          ? position[1]
          : position && typeof position === "object"
            ? position.row
            : -1,
      );
      const rowIndex =
        requestedRowIndex >= 0
          ? requestedRowIndex
          : column.items.value.indexOf(file);
      const item =
        typeof rowIndex === "number" && rowIndex >= 0
          ? column.items.value[rowIndex] || file
          : file;
      if (!item) return;

      column.selectedId.as(item.id);
      selected_.as(item);
      selected_position_.as([columnIndex, rowIndex]);
      truncateAfter(columnIndex);

      if (item.isDirectory) {
        const nextColumn = appendColumn(item);
        emitStateChange();
        if (typeof props.onSelect === "function") {
          props.onSelect(item, [columnIndex, rowIndex]);
        }
        return nextColumn.ready;
      }
      emitStateChange();
      if (typeof props.onSelect === "function") {
        props.onSelect(item, [columnIndex, rowIndex]);
      }
    },
    retry(columnOrIndex) {
      const columnIndex =
        columnOrIndex && typeof columnOrIndex === "object"
          ? columns_.value.indexOf(columnOrIndex)
          : file_picker_index_value(columnOrIndex);
      const column = columns_.value[columnIndex];
      if (!column) return Promise.resolve(null);
      return loadColumn(column.folder, columnIndex);
    },
    clearSelection() {
      columns_.value.forEach((column) => column.selectedId.as(null));
      selected_.as(null);
      selected_position_.as(null);
      emitStateChange();
    },
    destroy() {
      destroyed = true;
      columns_.value.forEach((column) => {
        column.requestId = ++requestSequence;
      });
      listeners.length = 0;
    },
  };

  const model = {
    state: {
      columns: columns_,
      paths: paths_,
      selected: selected_,
      selectedPosition: selected_position_,
      initialized: initialized_,
      loading: loading_,
      error: error_,
    },
    methods,
    getState() {
      return {
        columns: columns_.value,
        paths: paths_.value,
        selected: selected_.value,
        selectedPosition: selected_position_.value,
        initialized: initialized_.value,
        loading: loading_.value,
        error: error_.value,
      };
    },
    onStateChange(listener) {
      listeners.push(listener);
      return () => {
        const index = listeners.indexOf(listener);
        if (index >= 0) listeners.splice(index, 1);
      };
    },
    destroy: methods.destroy,
  };

  if (props.autoInit !== false) methods.init();
  return model;
}

function file_picker_icon(file) {
  return Timeless.Icon({
    name: file.isDirectory ? "folder" : "file",
    size: 17,
  });
}

function FilePicker(props = {}) {
  const store = props.store || props.model;
  if (!store || !store.state || !store.methods) {
    throw new TypeError("FilePicker requires a FilePickerModel as store");
  }

  return View(
    {
      class: cn(["wx-file-picker", props.class]),
      style: props.style,
      attributes: {
        role: "group",
        "aria-label": props.ariaLabel || "文件选择器",
      },
    },
    [
      View({ class: "wx-file-picker__columns" }, [
        For({
          each: store.state.columns,
          key: "id",
          render(column) {
            return View({ class: "wx-file-picker__column" }, [
              Show({
                when: column.loading,
                ok() {
                  return View({ class: "wx-file-picker__message" }, [
                    View({ class: "weui-loading" }),
                    "正在加载…",
                  ]);
                },
              }),
              Show({
                when: computed(column.error, (error) => Boolean(error)),
                ok() {
                  return View(
                    { class: "wx-file-picker__message wx-file-picker__error" },
                    [
                      View({}, [
                        computed(column.error, (error) =>
                          error ? error.message : "加载失败",
                        ),
                      ]),
                      Button(
                        {
                          class: "weui-btn weui-btn_default weui-btn_mini",
                          onClick() {
                            store.methods.retry(column);
                          },
                        },
                        ["重试"],
                      ),
                    ],
                  );
                },
              }),
              Show({
                when: combine(
                  [column.loading, column.error, column.items],
                  (loading, error, items) =>
                    !loading && !error && items.length === 0,
                ),
                ok() {
                  return View({ class: "wx-file-picker__message" }, [
                    "空文件夹",
                  ]);
                },
              }),
              For({
                each: column.items,
                key: "id",
                render(file, rowIndex) {
                  const disabled = !file_picker_accepts(file, props.accept);
                  const selectedClass = computed(column.selectedId, (id) =>
                    id === file.id ? "is-selected" : "",
                  );
                  return View(
                    {
                      class: cn([
                        "wx-file-picker__item",
                        selectedClass,
                        disabled ? "is-disabled" : "",
                      ]),
                      attributes: {
                        role: "button",
                        tabindex: disabled ? "-1" : "0",
                        "aria-disabled": disabled ? "true" : "false",
                        title: file.path || file.name,
                      },
                      onClick() {
                        if (disabled) return;
                        store.methods.select(file, { column, row: rowIndex });
                      },
                      onKeyDown(event) {
                        if (disabled) return;
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          store.methods.select(file, { column, row: rowIndex });
                        }
                      },
                    },
                    [
                      View({ class: "wx-file-picker__icon" }, [
                        file_picker_icon(file),
                      ]),
                      View({ class: "wx-file-picker__name" }, [file.name]),
                      file.isDirectory
                        ? View({ class: "wx-file-picker__arrow" }, [
                            Timeless.Icon({ name: "chevron-right", size: 15 }),
                          ])
                        : null,
                    ].filter(Boolean),
                  );
                },
              }),
            ]);
          },
        }),
      ]),
    ],
  );
}

function FilePickerDialog(props = {}) {
  const picker =
    props.picker ||
    props.model ||
    props.filePicker ||
    FilePickerModel({
      root: {
        id: "documents",
        name: props.documentsName || "文稿",
        path: "",
        isDirectory: true,
      },
      async loadFiles(folder) {
        const [error, data] = await WXU.request({
          url: APIOrigin + "/api/v1/fs/list",
          method: "POST",
          body: { dir: folder.path },
        });
        if (error) throw error;
        return data;
      },
      onError: props.onError,
    });
  const dialogStore = props.dialogStore || props.store;
  if (!dialogStore)
    throw new TypeError("FilePickerDialog requires a dialogStore");
  if (typeof props.onReady === "function") props.onReady(picker);

  function close() {
    if (typeof props.onCancel === "function") props.onCancel();
    if (typeof dialogStore.hide === "function") dialogStore.hide();
  }

  function confirm() {
    const selected = picker.state.selected.value;
    if (!selected) return;
    if (selected.isDirectory && props.selectDirectories !== true) return;
    if (!file_picker_accepts(selected, props.accept)) return;
    if (typeof props.onConfirm === "function")
      props.onConfirm(selected, picker.getState());
    if (
      props.closeOnConfirm !== false &&
      typeof dialogStore.hide === "function"
    ) {
      dialogStore.hide();
    }
  }

  const canConfirm = computed(picker.state.selected, (selected) =>
    Boolean(
      selected &&
      (props.selectDirectories === true || !selected.isDirectory) &&
      file_picker_accepts(selected, props.accept),
    ),
  );

  return Dialog(
    {
      store: dialogStore,
      class: cn(["wx-file-picker-dialog", props.class]),
    },
    [
      View({ class: "wx-file-picker-dialog__header" }, [
        View({ class: "wx-file-picker-dialog__title" }, [
          props.title || "选择文件",
        ]),
        Button(
          {
            class: "wx-file-picker-dialog__close",
            attributes: { "aria-label": "关闭" },
            onClick: close,
          },
          ["×"],
        ),
      ]),
      View({ class: "wx-file-picker-dialog__path" }, [
        For({
          each: picker.state.paths,
          render(path, index) {
            const pathIndex = file_picker_index_value(index);
            return View({ class: "flex" }, [
              Show({
                when: pathIndex > 0,
                ok() {
                  return View(
                    { class: "wx-file-picker-dialog__path-separator" },
                    ["/"],
                  );
                },
              }),
              View({ class: "wx-file-picker-dialog__path-part" }, [path.name]),
            ]);
          },
        }),
      ]),
      FilePicker({ store: picker, accept: props.accept }),
      View({ class: "wx-file-picker-dialog__footer" }, [
        View({ class: "wx-file-picker-dialog__button-group" }, [
          Button(
            {
              class: "weui-btn weui-btn_default weui-btn_mini",
              onClick: close,
            },
            [props.cancelText || "取消"],
          ),
          Button(
            {
              class: "weui-btn weui-btn_primary weui-btn_mini",
              disabled: computed(canConfirm, (enabled) => !enabled),
              onClick: confirm,
            },
            [props.confirmText || "选择"],
          ),
        ]),
      ]),
    ],
  );
}

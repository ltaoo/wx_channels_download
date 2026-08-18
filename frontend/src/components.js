const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Fragment, For, Show, View, combine, computed, ref, refobj } = Runtime;
const { ui, vm } = Runtime;

function class_names(values) {
  return Runtime.classNames(values.filter(Boolean));
}

function static_classes(values) {
  return values.filter(Boolean).join(" ");
}

function require_store(component, store, StoreType) {
  // if (!store || (StoreType && !(store instanceof StoreType))) {
  //   throw new TypeError(`${component} 需要对应的 Timeless store`);
  // }
  if (!store) {
    throw new TypeError(`${component} 需要对应的 Timeless store`);
  }
  return store;
}

function is_source(value) {
  return Boolean(
    value &&
      typeof value === "object" &&
      "value" in value &&
      typeof value.subscribe === "function",
  );
}

function source_value(value, fallback) {
  if (is_source(value)) {
    return value.value;
  }
  return value === undefined ? fallback : value;
}

function subscribe_source(source, handler) {
  if (!is_source(source)) {
    return null;
  }
  return source.subscribe({ onChange: handler });
}

function own_store_bindings(store, unlistens) {
  store.__dm_owned_bindings = true;
  store.__dm_dispose_bindings = function () {
    while (unlistens.length > 0) {
      const unlisten = unlistens.pop();
      if (typeof unlisten === "function") {
        unlisten();
      }
    }
  };
  return store;
}

function dispose_owned_store(store) {
  if (
    store &&
    store.__dm_owned_bindings &&
    typeof store.__dm_dispose_bindings === "function"
  ) {
    store.__dm_dispose_bindings();
  }
}

export function createButtonStore(props = {}) {
  const disabled = props.disabled;
  const loading = props.loading;
  const store = new vm.ButtonCore({
    disabled: Boolean(source_value(disabled, false)),
    loading: Boolean(source_value(loading, false)),
    variant: props.variant || "default",
    size: props.size || "default",
    onClick: props.onClick,
  });
  const unlistens = [];
  const disabled_unlisten = subscribe_source(disabled, (value) => {
    if (value) {
      store.disable();
    } else {
      store.enable();
    }
  });
  const loading_unlisten = subscribe_source(loading, (value) => {
    store.setLoading(Boolean(value));
  });
  if (disabled_unlisten) unlistens.push(disabled_unlisten);
  if (loading_unlisten) unlistens.push(loading_unlisten);
  return own_store_bindings(store, unlistens);
}

export function createInputStore(props = {}) {
  const value_source = props.value;
  const default_value = source_value(
    value_source,
    props.defaultValue === undefined ? "" : props.defaultValue,
  );
  const store = new vm.InputCore({
    defaultValue: default_value,
    placeholder: props.placeholder || "",
    type: props.type || "text",
    disabled: Boolean(source_value(props.disabled, false)),
    autoFocus: Boolean(props.autoFocus),
    autoComplete: Boolean(props.autoComplete),
    allowClear: props.allowClear !== false,
    maxLength: props.maxLength,
    minLength: props.minLength,
    ignoreEnterEvent: Boolean(props.ignoreEnterEvent),
    onChange(value) {
      if (value_source && typeof value_source.as === "function") {
        value_source.as(value);
      }
      if (typeof props.onChange === "function") {
        props.onChange(value);
      }
    },
    onEnter: props.onEnter,
    onKeyDown: props.onKeyDown,
    onBlur: props.onBlur,
    onClear: props.onClear,
  });
  const unlistens = [];
  const value_unlisten = subscribe_source(value_source, (value) => {
    if (store.value !== value) {
      store.setValue(value, { silence: true });
    }
  });
  const disabled_unlisten = subscribe_source(props.disabled, (value) => {
    store.disabled = Boolean(value);
    store.emitStateChange?.();
    store.setValue(store.value, { silence: true });
  });
  if (value_unlisten) unlistens.push(value_unlisten);
  if (disabled_unlisten) unlistens.push(disabled_unlisten);
  return own_store_bindings(store, unlistens);
}

const BUTTON_VARIANTS = {
  default: "",
  primary: "dm-button--primary",
  secondary: "dm-ui-button--secondary",
  outline: "dm-ui-button--outline",
  ghost: "dm-ui-button--ghost",
  destructive: "dm-button--danger",
  danger: "dm-button--danger",
  warn: "dm-button--danger",
  link: "dm-ui-button--link",
};

const BUTTON_SIZES = {
  xs: "dm-ui-button--xs",
  sm: "dm-ui-button--sm",
  default: "dm-ui-button--md",
  md: "dm-ui-button--md",
  lg: "dm-ui-button--lg",
  icon: "dm-ui-button--icon",
  "icon-sm": "dm-ui-button--icon-sm",
};

function button_class(state) {
  return static_classes([
    "dm-button dm-ui-button dm-focus-ring",
    BUTTON_VARIANTS[state.variant] || BUTTON_VARIANTS.default,
    BUTTON_SIZES[state.size] || BUTTON_SIZES.default,
    state.loading ? "is-loading" : "",
    state.disabled ? "is-disabled" : "",
  ]);
}

export function Button(props, children = []) {
  const {
    store: provided_store,
    class: extra_class,
    prefix,
    onUnmounted,
    ...rest
  } = props || {};
  const store = require_store("Button", provided_store, vm.ButtonCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));

  return ui.ButtonPrimitive.Root(
    {
      ...rest,
      store,
      class: class_names([
        computed(state_, (state) => button_class(state)),
        extra_class,
      ]),
      attributes: {
        ...(rest.attributes || {}),
        type: (rest.attributes && rest.attributes.type) || "button",
        disabled: computed(state_, (state) =>
          state.disabled || state.loading ? true : undefined,
        ),
        "aria-busy": computed(state_, (state) =>
          state.loading ? "true" : undefined,
        ),
      },
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        dispose_owned_store(store);
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      ui.ButtonPrimitive.Loading({ store }, [
        View({ class: "dm-ui-spinner", attributes: { "aria-hidden": "true" } }),
      ]),
      prefix
        ? ui.ButtonPrimitive.Prefix(
            { class: "dm-ui-button__prefix" },
            Array.isArray(prefix) ? prefix : [prefix],
          )
        : null,
      ui.ButtonPrimitive.Content({}, children),
    ].filter(Boolean),
  );
}

export function IconButton(props, children = []) {
  const store = require_store("IconButton", props && props.store, vm.ButtonCore);
  if (!String(store.state.size || "").startsWith("icon")) {
    store.setSize("icon");
  }
  return Button(
    {
      ...props,
      store,
      class: class_names(["dm-ui-icon-button", props.class]),
    },
    children,
  );
}

export function Input(props) {
  const {
    store: provided_store,
    class: extra_class,
    rootClass,
    onUnmounted,
    onChange,
    onKeyDown,
    ...rest
  } = props || {};
  const store = require_store("Input", provided_store, vm.InputCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));
  const show_clear_ = combine(
    { state: state_ },
    ({ state }) =>
      Boolean(
        state.allowClear &&
          state.value !== "" &&
          state.value !== null &&
          state.value !== undefined &&
          !state.loading &&
          !state.disabled,
      ),
  );

  return ui.InputPrimitive.Root(
    {
      store,
      class: class_names(["dm-ui-input-root", rootClass]),
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        dispose_owned_store(store);
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      ui.InputPrimitive.Input({
        ...rest,
        store,
        class: class_names(["dm-field dm-ui-input", extra_class]),
        onChange(event) {
          store.handleChange(event);
          if (typeof onChange === "function") onChange(event);
        },
        onKeyDown(event) {
          store.handleKeyDown(event);
          if (typeof onKeyDown === "function") onKeyDown(event);
        },
      }),
      Show({
        when: show_clear_,
        ok() {
          return ui.InputPrimitive.Clear(
            {
              store,
              class: "dm-ui-input-action dm-focus-ring",
              attributes: { "aria-label": "清空输入" },
            },
            [Runtime.Icon({ name: "circle-x", size: 14 })],
          );
        },
      }),
      Show({
        when: computed(state_, (state) => state.loading),
        ok() {
          return View({
            class: "dm-ui-input-action dm-ui-input-loading dm-ui-spinner",
            attributes: { "aria-hidden": "true" },
          });
        },
      }),
    ],
  );
}

export function Textarea(props) {
  const {
    store: provided_store,
    class: extra_class,
    rootClass,
    showCount = false,
    onUnmounted,
    onKeyDown,
    ...rest
  } = props || {};
  const store = require_store("Textarea", provided_store, vm.InputCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));

  return ui.TextareaPrimitive.Root(
    {
      store,
      class: class_names(["dm-ui-textarea-root", rootClass]),
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        dispose_owned_store(store);
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      ui.TextareaPrimitive.Textarea({
        ...rest,
        store,
        class: class_names(["dm-field dm-ui-textarea", extra_class]),
        onKeyDown(event) {
          store.handleKeyDown(event);
          if (typeof onKeyDown === "function") onKeyDown(event);
        },
      }),
      showCount
        ? ui.TextareaPrimitive.Count(
            { store, class: "dm-ui-textarea-count" },
            [],
          )
        : null,
    ].filter(Boolean),
  );
}

function select_entry(select_store, entry) {
  if (is_instance(entry, vm.SelectGroupCore)) {
    return Fragment({}, [
      entry.label
        ? View({ class: "dm-ui-select-group-label" }, [entry.label])
        : null,
      For({
        each: entry.options || [],
        render(child) {
          return select_entry(select_store, child);
        },
      }),
    ].filter(Boolean));
  }

  const item_ = refobj(entry.state);
  const unlisten = entry.onStateChange((state) => item_.as(state));
  return ui.SelectPrimitive.Item(
    {
      select$: select_store,
      item$: entry,
      class: computed(item_, (state) =>
        static_classes([
          "dm-ui-select-item",
          state.focused ? "is-focused" : "",
          state.selected ? "is-selected" : "",
          state.disabled ? "is-disabled" : "",
        ]),
      ),
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
      },
    },
    [
      ui.SelectPrimitive.ItemText(
        { class: "dm-ui-select-item-text" },
        [entry.label],
      ),
      ui.SelectPrimitive.ItemIndicator(
        { store: entry, class: "dm-ui-select-item-indicator" },
        [Runtime.Icon({ name: "check", size: 12 })],
      ),
    ],
  );
}

export function Select(props) {
  const {
    store: provided_store,
    class: extra_class,
    onUnmounted,
    ...rest
  } = props || {};
  const store = require_store("Select", provided_store, vm.SelectCore);
  const state_ = refobj(store.state);
  const hovering_ = ref(false);
  const unlisten = store.onStateChange((state) => state_.as(state));
  const show_clear_ = combine(
    { state: state_, hovering: hovering_ },
    ({ state, hovering }) =>
      Boolean(
        hovering &&
          state.allowClear &&
          state.value !== null &&
          !state.loading &&
          !state.disabled,
      ),
  );

  return ui.SelectPrimitive.Root(
    {
      store,
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      ui.SelectPrimitive.Trigger(
        {
          ...rest,
          store,
          class: class_names([
            "dm-field dm-ui-select",
            computed(state_, (state) =>
              static_classes([
                state.open ? "is-open" : "",
                state.disabled ? "is-disabled" : "",
              ]),
            ),
            extra_class,
          ]),
          onMouseEnter() {
            hovering_.as(true);
          },
          onMouseLeave() {
            hovering_.as(false);
          },
        },
        [
          Show({
            when: computed(state_, (state) => state.search),
            ok() {
              return ui.SelectPrimitive.Search({
                store,
                class: "dm-ui-select-search",
              });
            },
            else() {
              return ui.SelectPrimitive.Value({
                store,
                class: "dm-ui-select-value",
              });
            },
          }),
          Show({
            when: show_clear_,
            ok() {
              return ui.SelectPrimitive.Clear(
                {
                  store,
                  class: "dm-ui-select-action dm-focus-ring",
                  attributes: { "aria-label": "清除选择" },
                },
                [Runtime.Icon({ name: "circle-x", size: 14 })],
              );
            },
            else() {
              return ui.SelectPrimitive.Icon(
                { store, class: "dm-ui-select-action" },
                [
                  Runtime.Icon({
                    name: "chevron-down",
                    size: 14,
                    class: class_names([
                      computed(state_, (state) =>
                        state.open ? "is-open" : "",
                      ),
                    ]),
                  }),
                ],
              );
            },
          }),
        ],
      ),
      ui.SelectPrimitive.Content(
        {
          store,
          class: "dm-ui-select-content",
          attributes: { role: "listbox" },
          animation: {
            in: "is-entering",
            out: "is-exiting",
          },
        },
        [
          ui.SelectPrimitive.Viewport(
            { store, class: "dm-ui-select-viewport" },
            [
              Show({
                when: computed(state_, (state) => state.loading),
                ok() {
                  return View({ class: "dm-ui-select-state" }, ["加载中…"]);
                },
                else() {
                  return Show({
                    when: computed(
                      state_,
                      () => (store.raw_options || store.options || []).length > 0,
                    ),
                    ok() {
                      return For({
                        each: computed(
                          state_,
                          () => store.raw_options || store.options || [],
                        ),
                        render(entry) {
                          return select_entry(store, entry);
                        },
                      });
                    },
                    else() {
                      return View({ class: "dm-ui-select-state" }, [
                        "暂无选项",
                      ]);
                    },
                  });
                },
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

export function Checkbox(props) {
  const { store: provided_store, class: extra_class, id, ...rest } = props || {};
  const store = require_store("Checkbox", provided_store, vm.CheckboxCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));

  return ui.CheckboxPrimitive.Root({ store }, [
    ui.CheckboxPrimitive.Input({ store, id }),
    ui.CheckboxPrimitive.Box(
      {
        ...rest,
        store,
        class: computed(state_, (state) =>
          static_classes([
            "dm-ui-checkbox dm-focus-ring",
            state.checked ? "is-checked" : "",
            state.disabled ? "is-disabled" : "",
            extra_class,
          ]),
        ),
        onUnmounted() {
          if (typeof unlisten === "function") unlisten();
        },
      },
      [
        View({ class: "dm-ui-checkbox-box" }, [
          Runtime.Icon({ name: "check", size: 14 }),
        ]),
      ],
    ),
  ]);
}

export function Dialog(props, children = []) {
  const {
    store: provided_store,
    class: extra_class,
    style,
    zIndex: manual_z_index,
    showClose = true,
    closeLabel = "关闭",
    cancelText = "取消",
    okText = "确认",
    onUnmounted,
    attributes,
    ...rest
  } = props || {};
  const store = require_store("Dialog", provided_store, vm.DialogCore);
  const state_ = refobj(store.state);
  const presence_state_ = refobj(store.presence.state);
  const was_exiting_ = ref(false);
  const layer_manager =
    typeof vm.getGlobalLayerManager === "function"
      ? vm.getGlobalLayerManager()
      : null;
  const z_index =
    manual_z_index ?? 200 + (layer_manager ? layer_manager.size * 50 : 0);
  const unlistens = [
    store.onStateChange((state) => state_.as(state)),
    store.presence.onStateChange((state) => {
      presence_state_.as(state);
      if (state.exit) was_exiting_.as(true);
      if (state.mounted) was_exiting_.as(false);
    }),
  ];

  return ui.DialogPrimitive.Root(
    {
      store,
      onUnmounted() {
        unlistens.forEach((unlisten) => {
          if (typeof unlisten === "function") unlisten();
        });
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    () => [
      ui.DialogPrimitive.Overlay({
        store,
        zIndex: z_index,
        class: computed(presence_state_, (state) =>
          static_classes([
            "dm-ui-dialog-overlay",
            state.enter ? "is-entering" : "",
            state.exit || (!state.mounted && was_exiting_.value)
              ? "is-exiting"
              : "",
          ]),
        ),
      }),
      View(
        {
          class: "dm-ui-dialog-positioner",
          style: { "z-index": z_index + 1 },
        },
        [
          View(
            {
              style: computed(state_, (state) => {
                const rect = state.viewportRect;
                if (!rect) return {};
                return {
                  position: "fixed",
                  left: `${rect.left + rect.width / 2}px`,
                  top: `${rect.top + rect.height / 2}px`,
                  transform: "translate(-50%, -50%)",
                };
              }),
            },
            [
              ui.DialogPrimitive.Content(
                {
                  ...rest,
                  store,
                  zIndex: z_index + 1,
                  style,
                  attributes: {
                    role: "dialog",
                    "aria-modal": "true",
                    ...attributes,
                  },
                  class: computed(presence_state_, (state) =>
                    static_classes([
                      "dm-ui-dialog-content",
                      state.enter ? "is-entering" : "",
                      state.exit || (!state.mounted && was_exiting_.value)
                        ? "is-exiting"
                        : "",
                      extra_class,
                    ]),
                  ),
                },
                [
                  Show({
                    when: computed(state_, (state) => Boolean(state.title)),
                    ok() {
                      return ui.DialogPrimitive.Header(
                        { store, class: "dm-ui-dialog-header" },
                        [
                          ui.DialogPrimitive.Title(
                            { store, class: "dm-ui-dialog-title" },
                            [computed(state_, (state) => state.title || "")],
                          ),
                        ],
                      );
                    },
                  }),
                  Fragment(
                    {},
                    typeof children === "function" ? children() : children,
                  ),
                  showClose
                    ? ui.DialogPrimitive.Close(
                        {
                          store,
                          class: "dm-ui-dialog-close dm-focus-ring",
                          attributes: { "aria-label": closeLabel },
                        },
                        [Runtime.Icon({ name: "x", size: 16 })],
                      )
                    : null,
                  Show({
                    when: computed(state_, (state) => Boolean(state.footer)),
                    ok() {
                      return ui.DialogPrimitive.Footer(
                        { store, class: "dm-ui-dialog-footer" },
                        [
                          Button({ store: store.cancelBtn }, [cancelText]),
                          Button({ store: store.okBtn }, [okText]),
                        ],
                      );
                    },
                  }),
                ].filter(Boolean),
              ),
            ],
          ),
        ],
      ),
    ],
  );
}

export function DialogHeader(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-ui-dialog-header", props.class]) },
    children,
  );
}

export function DialogTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      type: props.type || "h2",
      class: class_names(["dm-ui-dialog-title", props.class]),
    },
    children,
  );
}

export function DialogDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      type: props.type || "p",
      class: class_names(["dm-ui-dialog-description", props.class]),
    },
    children,
  );
}

export function DialogBody(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-ui-dialog-body", props.class]) },
    children,
  );
}

export function DialogFooter(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-ui-dialog-footer", props.class]) },
    children,
  );
}

export function Popover(props, children = []) {
  const {
    store: provided_store,
    content,
    title,
    class: extra_class,
    triggerClass,
    onTriggerMouseEnter,
    onTriggerMouseLeave,
    onContentMouseEnter,
    onContentMouseLeave,
    onUnmounted,
    ...content_props
  } = props || {};
  const store = require_store("Popover", provided_store, vm.PopoverCore);
  const presence_state_ = refobj(store.presence.state);
  const was_exiting_ = ref(false);
  const unlisten = store.presence.onStateChange((state) => {
    presence_state_.as(state);
    if (state.exit) was_exiting_.as(true);
    if (state.mounted) was_exiting_.as(false);
  });
  const uses_hover_trigger =
    typeof onTriggerMouseEnter === "function" ||
    typeof onTriggerMouseLeave === "function";
  const trigger = uses_hover_trigger
    ? View(
        {
          class: class_names(["dm-ui-popover-trigger", triggerClass]),
          onMounted(event) {
            const root = event.target;
            const trigger_children =
              typeof root.getChildren === "function" ? root.getChildren() : [];
            const target =
              trigger_children.find(
                (child) => child && child.getType && child.getType() === "view",
              ) ||
              trigger_children[0] ||
              root;
            store.popper.setReference(
              {
                $el: target,
                getRect: () => target.getBoundingClientRect(),
              },
              { force: true },
            );
          },
          onMouseEnter: onTriggerMouseEnter,
          onMouseLeave: onTriggerMouseLeave,
        },
        children,
      )
    : ui.PopoverPrimitive.Trigger(
        { store, class: class_names(["dm-ui-popover-trigger", triggerClass]) },
        children,
      );

  return ui.PopoverPrimitive.Root(
    {
      store,
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      trigger,
      ui.PopoverPrimitive.Portal({ store }, [
        ui.PopoverPrimitive.Content(
          {
            ...content_props,
            store,
            zIndex: 1000,
            onMouseEnter: onContentMouseEnter,
            onMouseLeave: onContentMouseLeave,
            class: computed(presence_state_, (state) =>
              static_classes([
                "dm-ui-popover-content",
                state.enter ? "is-entering" : "",
                state.exit || (!state.mounted && was_exiting_.value)
                  ? "is-exiting"
                  : "",
                extra_class,
              ]),
            ),
          },
          [
            title
              ? View({ class: "dm-ui-popover-title" }, [title])
              : null,
            Fragment({}, content || []),
          ].filter(Boolean),
        ),
      ]),
    ],
  );
}

function is_instance(value, Type) {
  return Boolean(typeof Type === "function" && value instanceof Type);
}

function dropdown_separator() {
  return ui.DropdownMenuPrimitive.Separator({
    class: "dm-ui-dropdown-separator",
  });
}

function dropdown_group(group) {
  const state_ = refobj(group.state);
  const unlisten = group.onStateChange((state) => state_.as(state));
  return ui.DropdownMenuPrimitive.Group(
    {
      store: group,
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
      },
    },
    [
      Show({
        when: computed(state_, (state) => Boolean(state.label)),
        ok() {
          return ui.DropdownMenuPrimitive.Label(
            { class: "dm-ui-dropdown-label" },
            [computed(state_, (state) => state.label || "")],
          );
        },
      }),
      For({
        each: computed(state_, (state) => state.items || []),
        render: dropdown_entry,
      }),
    ],
  );
}

function dropdown_item(store) {
  const state_ = refobj(store.state);
  const menu_state_ = refobj(store.menu ? store.menu.state : {});
  const unlistens = [
    store.onStateChange((state) => state_.as(state)),
    store.menu
      ? store.menu.onStateChange((state) => menu_state_.as(state))
      : null,
  ];
  const is_checkable_ = computed(state_, (state) =>
    Object.prototype.hasOwnProperty.call(state, "checked"),
  );
  const is_checked_ = computed(state_, (state) =>
    Boolean(state.checked === true || state.checked === "indeterminate"),
  );

  return View(
    {
      onUnmounted() {
        unlistens.forEach((unlisten) => {
          if (typeof unlisten === "function") unlisten();
        });
      },
    },
    [
      ui.DropdownMenuPrimitive.Item(
        {
          store,
          class: computed(state_, (state) =>
            static_classes([
              "dm-ui-dropdown-item",
              state.focused ? "is-focused" : "",
              state.disabled ? "is-disabled" : "",
              state.variant === "destructive" ? "is-destructive" : "",
            ]),
          ),
        },
        [
          Show({
            when: is_checkable_,
            ok() {
              return View({ class: "dm-ui-dropdown-check" }, [
                Show({
                  when: is_checked_,
                  ok() {
                    return Runtime.Icon({ name: "check", size: 14 });
                  },
                }),
              ]);
            },
          }),
          store.icon
            ? View({ class: "dm-ui-dropdown-icon" }, [store.icon])
            : null,
          View({ class: "dm-ui-dropdown-item-label" }, [store.label]),
          store.shortcut
            ? View({ class: "dm-ui-dropdown-shortcut" }, [store.shortcut])
            : null,
          store.menu
            ? Runtime.Icon({ name: "chevron-right", size: 14 })
            : null,
        ].filter(Boolean),
      ),
      store.menu
        ? Show({
            when: computed(menu_state_, (state) => Boolean(state.open)),
            ok() {
              return ui.DropdownMenuPrimitive.SubMenuContent(
                { store: store.menu },
                [
                  View({ class: "dm-ui-dropdown-content" }, [
                    For({
                      each: computed(menu_state_, (state) => state.items || []),
                      render: dropdown_entry,
                    }),
                  ]),
                ],
              );
            },
          })
        : null,
    ].filter(Boolean),
  );
}

function dropdown_entry(entry) {
  if (is_instance(entry, vm.MenuSeparatorCore)) {
    return dropdown_separator();
  }
  if (is_instance(entry, vm.MenuGroupCore)) {
    return dropdown_group(entry);
  }
  return dropdown_item(entry);
}

export function DropdownMenu(props, children = []) {
  const { store: provided_store, class: extra_class, ...rest } = props || {};
  const store = require_store(
    "DropdownMenu",
    provided_store,
    vm.DropdownMenuCore,
  );
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));

  return Fragment(
    {
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
      },
    },
    [
      children.length
        ? ui.DropdownMenuPrimitive.Trigger({ store }, children)
        : null,
      ui.DropdownMenuPrimitive.Content(
        { ...rest, store },
        () => [
          View(
            {
              class: class_names(["dm-ui-dropdown-content", extra_class]),
            },
            [
              For({
                each: computed(state_, (state) => state.items || []),
                render: dropdown_entry,
              }),
            ],
          ),
        ],
      ),
    ].filter(Boolean),
  );
}

export const Dropdown = DropdownMenu;

function pagination_source(value, fallback) {
  return is_source(value)
    ? value
    : ref(value === undefined ? fallback : value);
}

export function Pagination(props = {}) {
  const page_ = pagination_source(props.page, 1);
  const page_count_ = pagination_source(props.pageCount, 1);
  const loading_ = pagination_source(props.loading, false);
  const previous_disabled_ = combine(
    { page: page_, loading: loading_ },
    (state) =>
      state.loading ||
      typeof props.onPrevious !== "function" ||
      Number(state.page) <= 1,
  );
  const next_disabled_ = combine(
    { page: page_, pageCount: page_count_, loading: loading_ },
    (state) =>
      state.loading ||
      typeof props.onNext !== "function" ||
      Number(state.page) >= Math.max(1, Number(state.pageCount)),
  );
  const page_text_ = combine(
    { page: page_, pageCount: page_count_ },
    (state) =>
      `${Math.max(1, Number(state.page) || 1)} / ${Math.max(
        1,
        Number(state.pageCount) || 1,
      )}`,
  );
  return View(
    {
      as: "nav",
      class: class_names(["dm-pagination", props.class]),
      attributes: {
        "aria-label": props.ariaLabel || "分页",
        ...(props.attributes || {}),
      },
    },
    [
      View(
        {
          class: "dm-pagination__summary",
          attributes: { "aria-live": "polite" },
        },
        [props.summary || ""],
      ),
      View({ class: "dm-pagination__controls" }, [
        Button(
          {
            store: createButtonStore({
              variant: "outline",
              size: "icon-sm",
              disabled: previous_disabled_,
              onClick() {
                if (typeof props.onPrevious === "function") {
                  props.onPrevious();
                }
              },
            }),
            class: "dm-pagination__button",
            attributes: {
              type: "button",
              title: props.previousLabel || "上一页",
              "aria-label": props.previousLabel || "上一页",
            },
          },
          [Runtime.Icon({ name: "chevron-left", size: 15 })],
        ),
        View(
          {
            class: "dm-pagination__page",
            attributes: { "aria-live": "polite", "aria-atomic": "true" },
          },
          [page_text_],
        ),
        Button(
          {
            store: createButtonStore({
              variant: "outline",
              size: "icon-sm",
              disabled: next_disabled_,
              onClick() {
                if (typeof props.onNext === "function") {
                  props.onNext();
                }
              },
            }),
            class: "dm-pagination__button",
            attributes: {
              type: "button",
              title: props.nextLabel || "下一页",
              "aria-label": props.nextLabel || "下一页",
            },
          },
          [Runtime.Icon({ name: "chevron-right", size: 15 })],
        ),
      ].filter(Boolean)),
    ],
  );
}

function element(type, base_class, props = {}, children = []) {
  const { class: extra_class, ...rest } = props;
  return View(
    { ...rest, type, class: class_names([base_class, extra_class]) },
    children,
  );
}

export function Label(props = {}, children = []) {
  return element("label", "dm-ui-label", props, children);
}

export function Badge(props = {}, children = []) {
  const { variant = "default", ...rest } = props;
  const variants = {
    default: "",
    success: "dm-badge--success",
    warning: "dm-badge--warning",
    destructive: "dm-badge--danger",
    danger: "dm-badge--danger",
    info: "dm-badge--info",
  };
  return element(
    "span",
    class_names(["dm-badge dm-ui-badge", variants[variant] || ""]),
    rest,
    children,
  );
}

export function Card(props = {}, children = []) {
  return element("div", "dm-panel dm-ui-card", props, children);
}

export function CardHeader(props = {}, children = []) {
  return element("div", "dm-ui-card-header", props, children);
}

export function CardTitle(props = {}, children = []) {
  return element("h3", "dm-ui-card-title", props, children);
}

export function CardDescription(props = {}, children = []) {
  return element("p", "dm-ui-card-description", props, children);
}

export function CardContent(props = {}, children = []) {
  return element("div", "dm-ui-card-content", props, children);
}

export function CardFooter(props = {}, children = []) {
  return element("div", "dm-ui-card-footer", props, children);
}

export function Table(props = {}, children = []) {
  return element("table", "dm-ui-table", props, children);
}

export function TableHeader(props = {}, children = []) {
  return element("thead", "dm-ui-table-header", props, children);
}

export function TableBody(props = {}, children = []) {
  return element("tbody", "dm-ui-table-body", props, children);
}

export function TableRow(props = {}, children = []) {
  return element("tr", "dm-ui-table-row", props, children);
}

export function TableHead(props = {}, children = []) {
  return element("th", "dm-ui-table-head", props, children);
}

export function TableCell(props = {}, children = []) {
  return element("td", "dm-ui-table-cell", props, children);
}

export function Skeleton(props = {}) {
  return element("div", "dm-ui-skeleton", props, []);
}

export function Progress(props = {}) {
  const { store: provided_store, class: extra_class, ...rest } = props;
  const store = require_store("Progress", provided_store, vm.ProgressCore);
  return ui.ProgressPrimitive.Root(
    {
      ...rest,
      store,
      class: class_names(["dm-ui-progress", extra_class]),
    },
    [
      ui.ProgressPrimitive.Indicator({
        store,
        class: "dm-ui-progress-indicator",
      }),
    ],
  );
}

export function Separator(props = {}) {
  const { orientation = "horizontal", ...rest } = props;
  return element(
    "div",
    class_names([
      "dm-ui-separator",
      orientation === "vertical" ? "is-vertical" : "is-horizontal",
    ]),
    {
      ...rest,
      attributes: { role: "separator", "aria-orientation": orientation },
    },
    [],
  );
}

export function Alert(props = {}, children = []) {
  const { variant = "default", ...rest } = props;
  return element(
    "div",
    class_names([
      "dm-ui-alert",
      variant === "destructive" || variant === "danger"
        ? "is-destructive"
        : "",
    ]),
    { ...rest, attributes: { role: "alert", ...(rest.attributes || {}) } },
    children,
  );
}

export function AlertTitle(props = {}, children = []) {
  return element("div", "dm-ui-alert-title", props, children);
}

export function AlertDescription(props = {}, children = []) {
  return element("div", "dm-ui-alert-description", props, children);
}

export default Popover;

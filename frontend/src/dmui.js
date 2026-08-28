const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Fragment, For, Match, Show, View, combine, computed, ref, refobj } = Runtime;
const { ui, vm } = Runtime;

function class_names(values) {
  return Runtime.classNames(values.filter(Boolean));
}

function static_classes(values) {
  return values.filter(Boolean).join(" ");
}

function require_store(component, store) {
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

function lazy_img_dom_element(event) {
  let target = event && event.target ? event.target : event;
  for (let depth = 0; depth < 4; depth += 1) {
    if (
      target &&
      target.nodeType === 1 &&
      typeof target.addEventListener === "function"
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

function lazy_img_source(value) {
  const resolved = source_value(value, "");
  if (resolved === null || resolved === undefined) {
    return "";
  }
  return String(resolved).trim();
}

function create_lazy_img_model(props) {
  let image = null;
  let settled = false;
  let current_src = lazy_img_source(props.src);
  let current_srcset = lazy_img_source(props.srcset);
  const failed_ = ref(false);
  const unlistens = [];

  function handle_load(event) {
    settled = true;
    image?.setAttribute("data-lazy-img-state", "loaded");
    failed_.as(false);
    if (typeof props.onLoad === "function") {
      props.onLoad(event);
    }
  }

  function handle_error(event) {
    settled = true;
    image?.setAttribute("data-lazy-img-state", "error");
    failed_.as(true);
    if (typeof props.onError === "function") {
      props.onError(event);
    }
  }

  function reset_failure() {
    settled = false;
    failed_.as(false);
  }

  function detach_image() {
    if (!image) return;
    image.removeEventListener("load", handle_load);
    image.removeEventListener("error", handle_error);
    image = null;
    settled = false;
  }

  const src_unlisten = subscribe_source(props.src, (value) => {
    current_src = lazy_img_source(value);
    reset_failure();
  });
  const srcset_unlisten = subscribe_source(props.srcset, (value) => {
    current_srcset = lazy_img_source(value);
    reset_failure();
  });
  if (src_unlisten) unlistens.push(src_unlisten);
  if (srcset_unlisten) unlistens.push(srcset_unlisten);

  return {
    state: {
      failed: failed_,
    },
    methods: {
      mount(event) {
        image = lazy_img_dom_element(event);
        if (image) {
          image.setAttribute("data-lazy-img-state", "loading");
          image.addEventListener("load", handle_load);
          image.addEventListener("error", handle_error);
          if (image.complete && (current_src || current_srcset)) {
            const mounted_image = image;
            queueMicrotask(() => {
              if (image !== mounted_image || settled) return;
              if (mounted_image.naturalWidth > 0) {
                handle_load({ target: mounted_image });
              } else {
                handle_error({ target: mounted_image });
              }
            });
          }
        }
        if (typeof props.onMounted === "function") {
          props.onMounted(event);
        }
      },
      load: handle_load,
      error: handle_error,
      unmount_image: detach_image,
      destroy() {
        detach_image();
        while (unlistens.length > 0) {
          const unlisten = unlistens.pop();
          if (typeof unlisten === "function") unlisten();
        }
        failed_.destroy?.();
        if (typeof props.onUnmounted === "function") {
          props.onUnmounted();
        }
      },
    },
  };
}

function lazy_img_attributes(props) {
  const attributes = { ...(props.attributes || {}) };
  delete attributes.src;
  delete attributes.srcset;
  const image_attributes = {
    alt:
      props.alt === undefined
        ? attributes.alt === undefined
          ? ""
          : attributes.alt
        : props.alt,
    width: props.width,
    height: props.height,
    loading: props.loading || "lazy",
    decoding: props.decoding,
    crossorigin: props.crossOrigin,
    sizes: props.sizes,
    referrerpolicy: props.referrerPolicy,
    fetchpriority: props.fetchPriority,
    usemap: props.useMap,
    ismap: props.isMap,
  };
  Object.entries(image_attributes).forEach(([name, value]) => {
    if (value !== undefined) attributes[name] = value;
  });
  return attributes;
}

function lazy_img_failure_attributes(props) {
  const attributes = { ...(props.attributes || {}) };
  const alt = lazy_img_source(
    props.alt === undefined ? attributes.alt : props.alt,
  );
  [
    "src",
    "srcset",
    "alt",
    "loading",
    "decoding",
    "crossorigin",
    "sizes",
    "referrerpolicy",
    "fetchpriority",
    "usemap",
    "ismap",
  ].forEach((name) => delete attributes[name]);
  attributes["data-lazy-img-state"] = "error";
  attributes.role = attributes.role || "img";
  attributes["aria-label"] =
    attributes["aria-label"] ||
    (alt ? `${alt}（图片加载失败）` : "图片加载失败");
  attributes.title = attributes.title || "图片加载失败";
  return attributes;
}

export function LazyImg(props = {}) {
  const {
    src,
    srcset,
    alt,
    width,
    height,
    loading,
    decoding,
    crossOrigin,
    sizes,
    referrerPolicy,
    fetchPriority,
    useMap,
    isMap,
    onLoad,
    onError,
    onMounted,
    onUnmounted,
    attributes,
    ...rest
  } = props;
  const image_src = src === undefined ? attributes && attributes.src : src;
  const image_srcset =
    srcset === undefined ? attributes && attributes.srcset : srcset;
  const model = create_lazy_img_model({
    src: image_src,
    srcset: image_srcset,
    onLoad,
    onError,
    onMounted,
    onUnmounted,
  });

  return Fragment(
    {
      onUnmounted() {
        model.methods.destroy();
      },
    },
    [
      Show({
        when: model.state.failed,
        ok() {
          return View(
            {
              ...rest,
              class: static_classes([rest.class, "dm-lazy-img-error"]),
              attributes: lazy_img_failure_attributes({ attributes, alt }),
            },
            [Runtime.Icon({ name: "file", size: 18 })],
          );
        },
        else() {
          return Runtime.Img({
            ...rest,
            src: image_src,
            srcset: image_srcset,
            attributes: lazy_img_attributes({
              attributes,
              alt,
              width,
              height,
              loading,
              decoding,
              crossOrigin,
              sizes,
              referrerPolicy,
              fetchPriority,
              useMap,
              isMap,
            }),
            onMounted(event) {
              model.methods.mount(event);
            },
            onUnmounted() {
              model.methods.unmount_image();
            },
          });
        },
      }),
    ],
  );
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

export function createCheckboxStore(props = {}) {
  const checked_source = props.checked;
  const store = new vm.CheckboxCore({
    checked: Boolean(source_value(checked_source, false)),
    disabled: Boolean(source_value(props.disabled, false)),
    onChange(value) {
      if (typeof props.onChange === "function") {
        props.onChange(value);
      } else if (checked_source && typeof checked_source.as === "function") {
        checked_source.as(value);
      }
      const controlled_value = Boolean(source_value(checked_source, value));
      if (store.value !== controlled_value) {
        store.setValue(controlled_value, { silence: true });
      }
    },
  });
  const unlistens = [];
  const checked_unlisten = subscribe_source(checked_source, (value) => {
    const checked = Boolean(value);
    if (store.value !== checked) {
      checked ? store.check() : store.uncheck();
    }
  });
  if (checked_unlisten) unlistens.push(checked_unlisten);
  return own_store_bindings(store, unlistens);
}

const BUTTON_VARIANTS = {
  default: "",
  primary: "dm-button--primary",
  secondary: "dm-button--secondary",
  outline: "dm-button--outline",
  ghost: "dm-button--ghost",
  destructive: "dm-button--danger",
  danger: "dm-button--danger",
  warn: "dm-button--danger",
  link: "dm-button--link",
};

const BUTTON_SIZES = {
  xs: "dm-button--xs",
  sm: "dm-button--sm",
  default: "dm-button--md",
  md: "dm-button--md",
  lg: "dm-button--lg",
  icon: "dm-button--icon",
  "icon-sm": "dm-button--icon-sm",
};

function button_class(state) {
  return static_classes([
    "dm-button dm-focus-ring",
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
        View({ class: "dm-spinner", attributes: { "aria-hidden": "true" } }),
      ]),
      prefix
        ? ui.ButtonPrimitive.Prefix(
            { class: "dm-button__prefix" },
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
      class: class_names(["dm-icon-button", props.class]),
    },
    children,
  );
}

export function Input(props) {
  const {
    store: provided_store,
    class: extra_class,
    rootClass,
    rootAttributes,
    prefix,
    suffix,
    attributes,
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
  const show_suffix_ = combine(
    { state: state_, clear: show_clear_ },
    ({ state, clear }) => Boolean(suffix || clear || state.loading),
  );

  return ui.InputPrimitive.Root(
    {
      store,
      class: class_names([
        "dm-input-affix-wrapper",
        computed(state_, (state) =>
          static_classes([
            state.focus ? "dm-input-affix-wrapper-focused" : "",
            state.hovering && !state.disabled
              ? "dm-input-affix-wrapper-hovered"
              : "",
            state.disabled ? "dm-input-affix-wrapper-disabled" : "",
            state.loading ? "dm-input-affix-wrapper-loading" : "",
          ]),
        ),
        rootClass,
      ]),
      attributes: { n: "input-wrapper", ...(rootAttributes || {}) },
      onClick() {
        if (!state_.value.disabled) store.focus();
      },
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        dispose_owned_store(store);
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [
      prefix
        ? View(
            {
              class: "dm-input-prefix",
              attributes: { n: "input-prefix" },
            },
            Array.isArray(prefix) ? prefix : [prefix],
          )
        : null,
      ui.InputPrimitive.Input({
        ...rest,
        store,
        class: class_names(["dm-input", extra_class]),
        attributes: { n: "input", ...(attributes || {}) },
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
        when: show_suffix_,
        ok() {
          return View(
            {
              class: "dm-input-suffix",
              attributes: { n: "input-suffix" },
            },
            [
              ...(suffix ? (Array.isArray(suffix) ? suffix : [suffix]) : []),
              Show({
                when: show_clear_,
                ok() {
                  return ui.InputPrimitive.Clear(
                    {
                      as: "button",
                      store,
                      class: "dm-input-clear-icon dm-focus-ring",
                      attributes: {
                        n: "input-clear",
                        type: "button",
                        "aria-label": "清空输入",
                      },
                    },
                    [
                      Runtime.Icon({
                        name: "circle-x",
                        size: 14,
                        attributes: { n: "input-clear-icon" },
                      }),
                    ],
                  );
                },
              }),
              Show({
                when: computed(state_, (state) => state.loading),
                ok() {
                  return View({
                    class: "dm-input-loading dm-spinner",
                    attributes: {
                      n: "input-loading",
                      "aria-hidden": "true",
                    },
                  });
                },
              }),
            ],
          );
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
      class: class_names(["dm-textarea-root", rootClass]),
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
        class: class_names(["dm-field dm-textarea", extra_class]),
        onKeyDown(event) {
          store.handleKeyDown(event);
          if (typeof onKeyDown === "function") onKeyDown(event);
        },
      }),
      showCount
        ? ui.TextareaPrimitive.Count(
            { store, class: "dm-textarea-count" },
            [],
          )
        : null,
    ].filter(Boolean),
  );
}

function file_picker_state_class(value, class_name) {
  return is_source(value)
    ? computed(value, (active) => (active ? class_name : ""))
    : value
      ? class_name
      : "";
}

export function FilePicker(props = {}) {
  const store = require_store("FilePicker", props.store, vm.FilePickerCore);
  const name = props.name || "file-picker";
  const error_visible = is_source(props.error)
    ? computed(props.error, (error) => Boolean(error))
    : Boolean(props.error);

  return ui.FilePickerPrimitive.Root(
    {
      store,
      class: class_names(["dm-file-picker", props.class]),
      attributes: { n: name, ...(props.attributes || {}) },
    },
    [
      ui.FilePickerPrimitive.Input({
        store,
        accept: props.accept ?? store.accept,
        multiple: Boolean(props.multiple),
        class: "dm-sr-only",
        attributes: {
          n: `${name}-input`,
          "aria-label": props.inputLabel || "选择文件",
          ...(props.inputAttributes || {}),
        },
        onChange: props.onChange,
      }),
      ui.FilePickerPrimitive.DropZone(
        {
          store,
          class: class_names([
            "dm-file-picker__drop-zone",
            file_picker_state_class(props.dragging, "is-dragging"),
            file_picker_state_class(props.invalid, "is-invalid"),
            file_picker_state_class(props.loading, "is-loading"),
          ]),
          attributes: {
            n: `${name}-drop-zone`,
            role: "button",
            tabindex: "0",
            "aria-label": props.dropZoneLabel || "点击选择或拖拽文件",
            ...(props.dropZoneAttributes || {}),
          },
          onKeyDown: props.onKeyDown,
        },
        [
          View(
            {
              class: "dm-file-picker__icon",
              attributes: { n: `${name}-icon`, "aria-hidden": "true" },
            },
            [
              Runtime.Icon({
                name: props.icon || "upload",
                size: props.iconSize || 24,
                attributes: { n: `${name}-upload-icon` },
              }),
            ],
          ),
          View(
            {
              class: "dm-file-picker__title",
              attributes: { n: `${name}-title` },
            },
            [props.title || "拖拽文件到此处"],
          ),
          props.hint
            ? View(
                {
                  class: "dm-file-picker__hint",
                  attributes: { n: `${name}-hint` },
                },
                [props.hint],
              )
            : null,
          props.formats
            ? View(
                {
                  class: "dm-file-picker__formats",
                  attributes: { n: `${name}-formats` },
                },
                [props.formats],
              )
            : null,
          Show({
            when: error_visible,
            ok() {
              return View(
                {
                  class: "dm-file-picker__error",
                  attributes: { n: `${name}-error`, role: "alert" },
                },
                [
                  Runtime.Icon({
                    name: "circle-alert",
                    size: 14,
                    attributes: { n: `${name}-error-icon` },
                  }),
                  View(
                    { attributes: { n: `${name}-error-message` } },
                    [props.error],
                  ),
                ],
              );
            },
          }),
        ].filter(Boolean),
      ),
    ],
  );
}

function select_entry(select_store, entry) {
  if (is_instance(entry, vm.SelectGroupCore)) {
    return Fragment({}, [
      entry.label
        ? View(
            {
              class: "dm-select-group-label",
              attributes: { n: "select-group-label" },
            },
            [entry.label],
          )
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
  const item = ui.SelectPrimitive.Item(
    {
      select$: select_store,
      item$: entry,
      attributes: {
        n: "select-option",
        role: "option",
        "aria-selected": computed(item_, (state) => String(state.selected)),
      },
      class: computed(item_, (state) =>
        static_classes([
          "dm-select-item",
          state.focused ? "is-focused" : "",
          state.selected ? "is-selected" : "",
          state.disabled ? "is-disabled" : "",
        ]),
      ),
    },
    [
      ui.SelectPrimitive.ItemText(
        {
          class: "dm-select-item-text",
          attributes: { n: "select-option-label" },
        },
        [entry.label],
      ),
      ui.SelectPrimitive.ItemIndicator(
        {
          store: entry,
          class: "dm-select-item-indicator",
          attributes: { n: "select-option-indicator" },
        },
        [
          Runtime.Icon({
            name: "check",
            size: 12,
            attributes: { n: "select-option-check-icon" },
          }),
        ],
      ),
    ],
  );

  return View(
    {
      class: "dm-select-item-root",
      attributes: { n: "select-option-root" },
      onMouseEnter() {
        select_store.handleMouseEnterItem(entry);
      },
      onMouseLeave() {
        select_store.handleMouseLeaveItem(entry);
      },
      onUnmounted() {
        if (typeof unlisten === "function") unlisten();
        item_.destroy?.();
      },
    },
    [item],
  );
}

function clear_select(store) {
  store.selected_item$?.setSelected(false);
  store.clear();
  store.hide();
}

export function Select(props = {}) {
  const {
    contentClass: content_class,
    rootClass: root_class,
    store: provided_store,
    class: extra_class,
    onUnmounted,
    ...rest
  } = props;
  const {
    onClick: provided_on_click,
    onPointerDown: provided_on_pointer_down,
    attributes: trigger_attributes,
    ...trigger_props
  } = rest;
  const store = require_store("Select", provided_store, vm.SelectCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));
  let suppress_next_click = false;
  let click_suppression_timer = null;

  function suppress_click_once() {
    suppress_next_click = true;
    globalThis.clearTimeout(click_suppression_timer);
    click_suppression_timer = globalThis.setTimeout(() => {
      suppress_next_click = false;
      click_suppression_timer = null;
    }, 1000);
  }

  function consume_click_suppression() {
    const suppressed = suppress_next_click;
    suppress_next_click = false;
    globalThis.clearTimeout(click_suppression_timer);
    click_suppression_timer = null;
    return suppressed;
  }

  function ensure_trigger_reference(event) {
    const event_target = event.currentTarget || event.target;
    const trigger_element =
      event_target?.get$elm?.() ||
      event_target?.closest?.(".dm-select") ||
      event_target;
    if (!trigger_element?.getBoundingClientRect) {
      return;
    }
    store.setTrigger?.(trigger_element);
    store.popper$?.setReference(
      {
        $el: trigger_element,
        getRect() {
          return trigger_element.getBoundingClientRect();
        },
      },
      { force: true },
    );
  }

  const primitive_select = ui.SelectPrimitive.Root(
    {
      store,
    },
    [
      ui.SelectPrimitive.Trigger(
        {
          ...trigger_props,
          store,
          attributes: {
            n: "select-trigger",
            ...trigger_attributes,
          },
          class: class_names([
            "dm-field dm-select",
            computed(state_, (state) =>
              static_classes([
                state.open ? "is-open" : "",
                state.disabled ? "is-disabled" : "",
              ]),
            ),
            extra_class,
          ]),
          onPointerDown(event) {
            const target = event.target;
            if (
              target?.tagName === "INPUT" ||
              target?.tagName === "TEXTAREA" ||
              target?.isContentEditable
            ) {
              provided_on_pointer_down?.(event);
              return;
            }
            event.preventDefault();
            event.stopPropagation();
            event.stopImmediatePropagation?.();
            ensure_trigger_reference(event);
            suppress_click_once();
            store.handleClickTrigger();
            provided_on_pointer_down?.(event);
          },
          onClick(event) {
            if (!consume_click_suppression()) {
              event.preventDefault();
              event.stopPropagation();
              ensure_trigger_reference(event);
              store.handleClickTrigger();
            }
            provided_on_click?.(event);
          },
        },
        [
          Show({
            when: computed(state_, (state) => state.search),
            ok() {
              return ui.SelectPrimitive.Search({
                store,
                class: "dm-select-search",
                attributes: { n: "select-search-input" },
              });
            },
            else() {
              return View(
                {
                  class: class_names([
                    "dm-select-value",
                    computed(state_, (state) =>
                      state.selectedOption ? "has-value" : "is-placeholder",
                    ),
                  ]),
                  attributes: { n: "select-value" },
                },
                [
                  computed(state_, (state) =>
                    state.selectedOption?.label ??
                    state.selectedOption?.value ??
                    state.placeholder ??
                    "请选择",
                  ),
                ],
              );
            },
          }),
          View(
            {
              as: "button",
              class: "dm-select-action dm-select-clear dm-focus-ring",
              attributes: {
                n: "select-clear-button",
                type: "button",
                "aria-label": "清除选择",
              },
              onPointerDown(event) {
                event.preventDefault();
                event.stopPropagation();
              },
              onClick(event) {
                event.preventDefault();
                event.stopPropagation();
                clear_select(store);
              },
            },
            [
              Runtime.Icon({
                name: "circle-x",
                size: 14,
                attributes: { n: "select-clear-icon" },
              }),
            ],
          ),
          ui.SelectPrimitive.Icon(
            {
              store,
              class: "dm-select-action dm-select-chevron",
              attributes: { n: "select-chevron" },
            },
            [
              Runtime.Icon({
                name: "chevron-down",
                size: 14,
                attributes: { n: "select-chevron-icon" },
                class: class_names([
                  computed(state_, (state) =>
                    state.open ? "is-open" : "",
                  ),
                ]),
              }),
            ],
          ),
        ],
      ),
      Show({
        when: computed(state_, (state) => state.open),
        ok() {
          return ui.SelectPrimitive.Content(
            {
              store,
              class: class_names(["dm-select-content", content_class]),
              attributes: { n: "select-popup", role: "listbox" },
              animation: {
                in: "is-entering",
                out: "is-exiting",
              },
            },
            () => [
              ui.SelectPrimitive.Viewport(
                {
                  store,
                  class: "dm-select-viewport",
                  attributes: { n: "select-options" },
                },
                [
                  Show({
                    when: computed(state_, (state) => state.loading),
                    ok() {
                      return View(
                        {
                          class: "dm-select-state",
                          attributes: { n: "select-loading-state" },
                        },
                        ["加载中…"],
                      );
                    },
                    else() {
                      return Show({
                        when: computed(
                          state_,
                          (state) =>
                            (state.options || store.raw_options || []).length >
                            0,
                        ),
                        ok() {
                          return For({
                            each: computed(
                              state_,
                              (state) =>
                                state.options || store.raw_options || [],
                            ),
                            render(entry) {
                              return select_entry(store, entry);
                            },
                          });
                        },
                        else() {
                          return View(
                            {
                              class: "dm-select-state",
                              attributes: { n: "select-empty-state" },
                            },
                            ["暂无选项"],
                          );
                        },
                      });
                    },
                  }),
                ],
              ),
            ],
          );
        },
      }),
    ],
  );

  return View(
    {
      class: class_names([
        computed(state_, (state) =>
          static_classes([
            "dm-select-root",
            state.allowClear &&
            state.value !== null &&
            !state.loading &&
            !state.disabled
              ? "can-clear"
              : "",
            state.open ? "is-open" : "",
            state.disabled ? "is-disabled" : "",
          ]),
        ),
        root_class,
      ]),
      attributes: { n: "select-root" },
      onUnmounted() {
        consume_click_suppression();
        if (typeof unlisten === "function") unlisten();
        state_.destroy?.();
        if (typeof onUnmounted === "function") onUnmounted();
      },
    },
    [primitive_select],
  );
}

export function Checkbox(props) {
  const {
    store: provided_store,
    class: extra_class,
    id,
    indeterminate = false,
    text,
    textAttributes: text_attributes,
    onUnmounted,
    attributes,
    ...rest
  } = props || {};
  const store = require_store("Checkbox", provided_store, vm.CheckboxCore);
  const state_ = refobj(store.state);
  const unlisten = store.onStateChange((state) => state_.as(state));
  const indeterminate_class = is_source(indeterminate)
    ? computed(indeterminate, (value) => (value ? "is-indeterminate" : ""))
    : indeterminate
      ? "is-indeterminate"
      : "";
  const aria_checked_ = is_source(indeterminate)
    ? combine(
        { state: state_, indeterminate },
        (state) =>
          state.indeterminate ? "mixed" : String(Boolean(state.state.checked)),
      )
    : computed(state_, (state) =>
        indeterminate ? "mixed" : String(Boolean(state.checked)),
      );

  return ui.CheckboxPrimitive.Root({ store }, [
    ui.CheckboxPrimitive.Input({
      store,
      id,
      attributes: { n: "checkbox-input" },
    }),
    ui.CheckboxPrimitive.Box(
      {
        ...rest,
        store,
        attributes: {
          n: "checkbox",
          type: "button",
          role: "checkbox",
          "aria-checked": aria_checked_,
          ...(attributes || {}),
        },
        class: class_names([
          computed(state_, (state) =>
            static_classes([
              "dm-checkbox",
              state.checked ? "is-checked" : "",
              state.disabled ? "is-disabled" : "",
            ]),
          ),
          indeterminate_class,
          extra_class,
        ]),
        onUnmounted() {
          if (typeof unlisten === "function") unlisten();
          dispose_owned_store(store);
          if (typeof onUnmounted === "function") onUnmounted();
        },
      },
      [
        View(
          {
            class: "dm-checkbox-box",
            attributes: { n: "checkbox-box" },
          },
          [
            Show({
              when: indeterminate,
              ok() {
                return View({
                  class: "dm-checkbox-indeterminate",
                  attributes: { n: "checkbox-indeterminate-icon" },
                });
              },
              else() {
                return Runtime.Icon({
                  name: "check",
                  size: 14,
                  attributes: { n: "checkbox-check-icon" },
                });
              },
            }),
          ],
        ),
        ...(typeof text === "undefined"
          ? []
          : [
              View(
                {
                  class: "dm-checkbox-text",
                  attributes: {
                    n: "checkbox-text",
                    ...(text_attributes || {}),
                  },
                },
                [text],
              ),
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
            "dm-dialog-overlay",
            state.enter ? "is-entering" : "",
            state.exit || (!state.mounted && was_exiting_.value)
              ? "is-exiting"
              : "",
          ]),
        ),
      }),
      View(
        {
          class: "dm-dialog-positioner",
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
                      "dm-dialog-content",
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
                        { store, class: "dm-dialog-header" },
                        [
                          ui.DialogPrimitive.Title(
                            { store, class: "dm-dialog-title" },
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
                          class: "dm-dialog-close dm-focus-ring",
                          attributes: { "aria-label": closeLabel },
                        },
                        [Runtime.Icon({ name: "x", size: 16 })],
                      )
                    : null,
                  Show({
                    when: computed(state_, (state) => Boolean(state.footer)),
                    ok() {
                      return ui.DialogPrimitive.Footer(
                        { store, class: "dm-dialog-footer" },
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
    { ...props, class: class_names(["dm-dialog-header", props.class]) },
    children,
  );
}

export function DialogTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-dialog-title", props.class]),
    },
    children,
  );
}

export function DialogDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-dialog-description", props.class]),
    },
    children,
  );
}

export function DialogBody(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-dialog-body", props.class]) },
    children,
  );
}

export function DialogFooter(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-dialog-footer", props.class]) },
    children,
  );
}

export function Confirm(props = {}, children = []) {
  const {
    name = "confirm",
    title = "确认操作",
    description = "",
    attributes,
    ...dialog_props
  } = props;
  const content = Array.isArray(children) ? children : [children];

  return Dialog(
    {
      ...dialog_props,
      showClose: dialog_props.showClose ?? false,
      attributes: {
        ...attributes,
        n: `${name}-dialog`,
        role: "alertdialog",
      },
    },
    [
      DialogHeader(
        { attributes: { n: `${name}-header` } },
        [
          DialogTitle(
            { attributes: { n: `${name}-title` } },
            [title],
          ),
          description
            ? DialogDescription(
                { attributes: { n: `${name}-description` } },
                [description],
              )
            : null,
        ].filter(Boolean),
      ),
      ...content,
    ].filter(Boolean),
  );
}

export function Drawer(props, children = []) {
  const {
    store: provided_store,
    class: extra_class,
    style,
    placement = "right",
    zIndex: manual_z_index,
    showClose = true,
    closeLabel = "关闭",
    cancelText = "取消",
    okText = "确认",
    onUnmounted,
    attributes,
    ...rest
  } = props || {};
  const store = require_store("Drawer", provided_store, vm.DialogCore);
  const state_ = refobj(store.state);
  const presence_state_ = refobj(store.presence.state);
  const was_exiting_ = ref(false);
  const resolved_placement = placement === "left" ? "left" : "right";
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
        onClick(event) {
          if (event.target === event.currentTarget && state_.value.closeable) {
            store.hide();
          }
        },
        class: computed(presence_state_, (state) =>
          static_classes([
            "dm-drawer-overlay",
            state.enter ? "is-entering" : "",
            state.exit || (!state.mounted && was_exiting_.value)
              ? "is-exiting"
              : "",
          ]),
        ),
      }),
      View(
        {
          class: static_classes([
            "dm-drawer-positioner",
            `is-${resolved_placement}`,
          ]),
          style: { "z-index": z_index + 1 },
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
                  "dm-drawer-content",
                  `is-${resolved_placement}`,
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
                    { store, class: "dm-drawer-header" },
                    [
                      ui.DialogPrimitive.Title(
                        { store, class: "dm-drawer-title" },
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
                      class: "dm-drawer-close dm-focus-ring",
                      attributes: { "aria-label": closeLabel },
                    },
                    [Runtime.Icon({ name: "x", size: 18 })],
                  )
                : null,
              Show({
                when: computed(state_, (state) => Boolean(state.footer)),
                ok() {
                  return ui.DialogPrimitive.Footer(
                    { store, class: "dm-drawer-footer" },
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
  );
}

export function DrawerHeader(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-drawer-header", props.class]) },
    children,
  );
}

export function DrawerTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-drawer-title", props.class]),
    },
    children,
  );
}

export function DrawerDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-drawer-description", props.class]),
    },
    children,
  );
}

export function DrawerBody(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-drawer-body", props.class]) },
    children,
  );
}

export function DrawerFooter(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-drawer-footer", props.class]) },
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
          class: class_names(["dm-popover-trigger", triggerClass]),
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
        { store, class: class_names(["dm-popover-trigger", triggerClass]) },
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
                "dm-popover-content",
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
              ? View({ class: "dm-popover-title" }, [title])
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
    class: "dm-dropdown-separator",
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
            { class: "dm-dropdown-label" },
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
              "dm-dropdown-item",
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
              return View({ class: "dm-dropdown-check" }, [
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
            ? View({ class: "dm-dropdown-icon" }, [store.icon])
            : null,
          View({ class: "dm-dropdown-item-label" }, [store.label]),
          store.shortcut
            ? View({ class: "dm-dropdown-shortcut" }, [store.shortcut])
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
                  View({ class: "dm-dropdown-content" }, [
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
              class: class_names(["dm-dropdown-content", extra_class]),
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

export function pagination_items(page, page_count) {
  const total = Math.max(1, Number(page_count) || 1);
  const current = Math.min(total, Math.max(1, Number(page) || 1));
  const item = (value) => ({ key: `page-${value}`, page: value });
  const ellipsis = (key) => ({ key, page: null });

  if (total <= 7) {
    return Array.from({ length: total }, (_, index) => item(index + 1));
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5].map(item).concat(ellipsis("next"), item(total));
  }
  if (current >= total - 3) {
    return [
      item(1),
      ellipsis("previous"),
      ...[total - 4, total - 3, total - 2, total - 1, total].map(item),
    ];
  }
  return [
    item(1),
    ellipsis("previous"),
    item(current - 1),
    item(current),
    item(current + 1),
    ellipsis("next"),
    item(total),
  ];
}

function create_pagination_model(props) {
  const page_ = pagination_source(props.page, 1);
  const page_count_ = pagination_source(props.pageCount, 1);
  const loading_ = pagination_source(props.loading, false);
  const has_change_handler = typeof props.onChange === "function";
  const previous_disabled_ = combine(
    { page: page_, loading: loading_ },
    (state) =>
      state.loading ||
      (!has_change_handler && typeof props.onPrevious !== "function") ||
      Number(state.page) <= 1,
  );
  const next_disabled_ = combine(
    { page: page_, pageCount: page_count_, loading: loading_ },
    (state) =>
      state.loading ||
      (!has_change_handler && typeof props.onNext !== "function") ||
      Number(state.page) >= Math.max(1, Number(state.pageCount)),
  );
  const page_items_ = combine(
    { page: page_, pageCount: page_count_ },
    (state) => pagination_items(state.page, state.pageCount),
  );

  function change_page(target_page) {
    const current = Math.max(1, Number(page_.value) || 1);
    const page_count = Math.max(1, Number(page_count_.value) || 1);
    const target = Math.min(page_count, Math.max(1, Number(target_page) || 1));
    if (loading_.value || target === current) return null;
    if (has_change_handler) {
      return props.onChange(target, source_value(props.pageSize));
    }
    if (target === current - 1 && typeof props.onPrevious === "function") {
      return props.onPrevious();
    }
    if (target === current + 1 && typeof props.onNext === "function") {
      return props.onNext();
    }
    return null;
  }

  const ui = {
    next_button$: new vm.ButtonCore({
      variant: "outline",
      size: "icon-sm",
      disabled: Boolean(next_disabled_.value),
      onClick() {
        return change_page(Number(page_.value) + 1);
      },
    }),
    previous_button$: new vm.ButtonCore({
      variant: "outline",
      size: "icon-sm",
      disabled: Boolean(previous_disabled_.value),
      onClick() {
        return change_page(Number(page_.value) - 1);
      },
    }),
  };
  const unlistens = [
    next_disabled_.subscribe({
      onChange(value) {
        if (value) {
          ui.next_button$.disable();
        } else {
          ui.next_button$.enable();
        }
      },
    }),
    previous_disabled_.subscribe({
      onChange(value) {
        if (value) {
          ui.previous_button$.disable();
        } else {
          ui.previous_button$.enable();
        }
      },
    }),
  ];

  return {
    state: {
      loading: loading_,
      page: page_,
      page_items: page_items_,
    },
    ui,
    methods: {
      changePage: change_page,
      destroy() {
        unlistens.forEach((unlisten) => unlisten?.());
        Object.values(ui).forEach((store) => store.destroy?.());
        previous_disabled_.destroy?.();
        next_disabled_.destroy?.();
        page_items_.destroy?.();
      },
    },
  };
}

export function Pagination(props = {}) {
  const model = create_pagination_model(props);
  return View(
    {
      class: class_names(["dm-pagination", props.class]),
      attributes: {
        n: "pagination",
        role: "navigation",
        "aria-label": props.ariaLabel || "分页",
        ...(props.attributes || {}),
      },
      onUnmounted() {
        model.methods.destroy();
        if (typeof props.onUnmounted === "function") {
          props.onUnmounted();
        }
      },
    },
    [
      View(
        {
          class: computed(model.state.loading, (loading) =>
            static_classes([
              "dm-pagination__summary",
              loading ? "is-loading" : "",
            ]),
          ),
          attributes: {
            n: "pagination-summary",
            "aria-live": "polite",
            "aria-busy": model.state.loading,
          },
        },
        [
          props.summary || "",
          Show({
            when: model.state.loading,
            ok() {
              return Skeleton({
                class: "dm-pagination__summary-skeleton",
                attributes: {
                  n: "pagination-summary-skeleton",
                  "aria-hidden": "true",
                },
              });
            },
          }),
        ],
      ),
      View(
        {
          class: computed(model.state.loading, (loading) =>
            static_classes([
              "dm-pagination__controls",
              loading ? "is-loading" : "",
            ]),
          ),
          attributes: {
            n: "pagination-controls",
            "aria-busy": model.state.loading,
          },
        },
        [
          Button(
            {
              store: model.ui.previous_button$,
              class: "dm-pagination__button",
              attributes: {
                n: "pagination-previous",
                type: "button",
                title: props.previousLabel || "上一页",
                "aria-label": props.previousLabel || "上一页",
              },
            },
            [
              Runtime.Icon({
                name: "chevron-left",
                size: 15,
                attributes: { n: "pagination-previous-icon" },
              }),
            ],
          ),
          View(
            {
              class: "dm-pagination__pages",
              attributes: { n: "pagination-pages", "aria-live": "polite" },
            },
            [
              For({
                key: "key",
                each: model.state.page_items,
                render(item) {
                  if (item.page === null) {
                    return View(
                      {
                        class: "dm-pagination__ellipsis",
                        attributes: {
                          n: `pagination-${item.key}-ellipsis`,
                          "aria-hidden": "true",
                        },
                      },
                      ["…"],
                    );
                  }
                  return View(
                    {
                      as: "button",
                      class: computed(model.state.page, (page) =>
                        static_classes([
                          "dm-pagination__page dm-focus-ring",
                          Number(page) === item.page ? "is-active" : "",
                        ]),
                      ),
                      attributes: {
                        n: `pagination-page-${item.page}`,
                        type: "button",
                        "aria-label": `第 ${item.page} 页`,
                        "aria-current": computed(model.state.page, (page) =>
                          Number(page) === item.page ? "page" : undefined,
                        ),
                        disabled: computed(
                          model.state.loading,
                          (loading) =>
                            loading || typeof props.onChange !== "function"
                              ? true
                              : undefined,
                        ),
                      },
                      onClick() {
                        return model.methods.changePage(item.page);
                      },
                    },
                    [String(item.page)],
                  );
                },
              }),
            ],
          ),
          Button(
            {
              store: model.ui.next_button$,
              class: "dm-pagination__button",
              attributes: {
                n: "pagination-next",
                type: "button",
                title: props.nextLabel || "下一页",
                "aria-label": props.nextLabel || "下一页",
              },
            },
            [
              Runtime.Icon({
                name: "chevron-right",
                size: 15,
                attributes: { n: "pagination-next-icon" },
              }),
            ],
          ),
          Show({
            when: model.state.loading,
            ok() {
              return Skeleton({
                class: "dm-pagination__controls-skeleton",
                attributes: {
                  n: "pagination-controls-skeleton",
                  "aria-hidden": "true",
                },
              });
            },
          }),
        ].filter(Boolean),
      ),
    ],
  );
}

export function Label(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-label", props.class]),
      attributes: { n: "label", ...(props.attributes || {}) },
    },
    children,
  );
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
  return View(
    {
      ...rest,
      class: class_names(["dm-badge", variants[variant] || "", rest.class]),
      attributes: { n: "badge", ...(rest.attributes || {}) },
    },
    children,
  );
}

export function Card(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-panel dm-card", props.class]),
      attributes: { n: "card", ...(props.attributes || {}) },
    },
    children,
  );
}

export function CardHeader(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-card-header", props.class]),
      attributes: { n: "card-header", ...(props.attributes || {}) },
    },
    children,
  );
}

export function CardTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-card-title", props.class]),
      attributes: { n: "card-title", ...(props.attributes || {}) },
    },
    children,
  );
}

export function CardDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-card-description", props.class]),
      attributes: { n: "card-description", ...(props.attributes || {}) },
    },
    children,
  );
}

export function CardContent(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-card-content", props.class]),
      attributes: { n: "card-content", ...(props.attributes || {}) },
    },
    children,
  );
}

export function CardFooter(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-card-footer", props.class]),
      attributes: { n: "card-footer", ...(props.attributes || {}) },
    },
    children,
  );
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

function table_keyed_rows(rows, row_key) {
  if (typeof row_key !== "function") return rows;
  const wrap_rows = (items) =>
    (Array.isArray(items) ? items : []).map((item, index) => ({
      __table_row_key: row_key(item, index),
      item,
    }));
  return table_source(rows) ? computed(rows, wrap_rows) : wrap_rows(rows);
}

function table_keyed_item(entry, row_key) {
  return typeof row_key === "function" ? entry.item : entry;
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

function table_is_last_row(items, index) {
  return Array.isArray(items) && items.length > 0 && index === items.length - 1;
}

function table_last_row(rows, index_) {
  if (table_source(rows) && table_source(index_)) {
    return combine({ rows, index: index_ }, (state) =>
      table_is_last_row(state.rows, state.index),
    );
  }
  if (table_source(rows)) {
    return computed(rows, (items) =>
      table_is_last_row(items, table_value(index_)),
    );
  }
  if (table_source(index_)) {
    return computed(index_, (index) => table_is_last_row(rows, index));
  }
  return table_is_last_row(rows, index_);
}

function table_loading_state(status, loading, initial_loading) {
  const resolve = (current_status, current_loading) => {
    return current_status === "initial"
      ? initial_loading
      : Boolean(current_loading);
  };
  if (table_source(status) && table_source(loading)) {
    return combine({ status, loading }, (state) =>
      resolve(state.status, state.loading),
    );
  }
  if (table_source(status)) {
    return computed(status, (current_status) =>
      resolve(current_status, table_value(loading)),
    );
  }
  if (table_source(loading)) {
    return computed(loading, (current_loading) =>
      resolve(table_value(status), current_loading),
    );
  }
  return resolve(status, loading);
}

function table_loading_visible(status, loading) {
  return table_loading_state(status, loading, false);
}

function table_children(value) {
  if (value === undefined || value === null) return [];
  return Array.isArray(value) ? value : [value];
}

function table_column_width(column) {
  const width = column && column.width;
  if (width === undefined || width === null || width === "") {
    return "minmax(0, 1fr)";
  }
  return typeof width === "number" ? `${width}px` : String(width);
}

function table_grid_columns(props) {
  const columns = props.columns.map(table_column_width);
  if (props.rowSelection) {
    columns.unshift(
      table_column_width({ width: props.rowSelection.width ?? 32 }),
    );
  }
  return columns.join(" ");
}

function table_cell_alignment(value) {
  const align = value === "center" || value === "right" ? value : "left";
  return {
    "text-align": align,
    "justify-content":
      align === "right"
        ? "flex-end"
        : align === "center"
          ? "center"
          : "flex-start",
  };
}

function table_scrollbar_width(element) {
  if (!element || element.scrollHeight <= element.clientHeight) return 0;
  return Math.max(0, element.offsetWidth - element.clientWidth);
}

function create_table_scrollbar_model() {
  const width_ = ref(0);
  let element = null;
  let resize_observer = null;
  let mutation_observer = null;
  let frame = 0;

  function measure() {
    frame = 0;
    const width = table_scrollbar_width(element);
    if (width_.value !== width) width_.as(width);
  }

  function schedule_measure() {
    if (!element || frame) return;
    frame = window.requestAnimationFrame(measure);
  }

  function unmount() {
    if (frame) window.cancelAnimationFrame(frame);
    frame = 0;
    resize_observer?.disconnect();
    mutation_observer?.disconnect();
    resize_observer = null;
    mutation_observer = null;
    element = null;
    if (width_.value !== 0) width_.as(0);
  }

  return {
    state: { width: width_ },
    methods: {
      mount(event) {
        unmount();
        element = table_scroll_element(event);
        if (!element) return;
        resize_observer = new ResizeObserver(schedule_measure);
        resize_observer.observe(element);
        mutation_observer = new MutationObserver(schedule_measure);
        mutation_observer.observe(element, {
          attributes: true,
          childList: true,
          characterData: true,
          subtree: true,
        });
        measure();
      },
      measure: schedule_measure,
      unmount,
      destroy() {
        unmount();
        width_.destroy?.();
      },
    },
  };
}

function table_state_icon(value, fallback_name, fallback_size) {
  const icon = table_resolve(value);
  if (icon === false || icon === null) return null;
  if (icon === undefined) {
    return Runtime.Icon({ name: fallback_name, size: fallback_size });
  }
  return typeof icon === "string"
    ? Runtime.Icon({ name: icon, size: fallback_size })
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
          class: "dm-table-state-title",
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
          class: "dm-table-state-text",
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
      class: static_classes(["dm-table-state", props.class]),
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
      class: "dm-button--toolbar",
      attributes: {
        n: `${props.name}-retry`,
        type: "button",
        ...(retry.attributes || {}),
      },
      prefix:
        retry.icon === false
          ? null
          : Runtime.Icon({
              name: retry.icon || "refresh-cw",
              size: retry.iconSize || 16,
            }),
    },
    table_children(retry.label ?? "重试"),
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
  const checked_ = table_source(state_)
    ? computed(state_, (state) => Boolean(state.checked))
    : Boolean(state_ && state_.checked);
  const indeterminate_ = table_source(state_)
    ? computed(state_, (state) => Boolean(state.indeterminate))
    : Boolean(state_ && state_.indeterminate);
  let toggle_event = null;
  const checkbox_store = createCheckboxStore({
    checked: checked_,
    onChange() {
      const event = toggle_event;
      toggle_event = null;
      if (typeof props.onToggle === "function") props.onToggle(event);
    },
  });

  return Checkbox({
    store: checkbox_store,
    indeterminate: indeterminate_,
    attributes: {
      n: props.name,
      "aria-label": props.ariaLabel,
    },
    onClick(event) {
      event?.stopPropagation?.();
      toggle_event = event;
    },
    onUnmounted() {
      if (table_source(checked_)) checked_.destroy?.();
      if (table_source(indeterminate_)) indeterminate_.destroy?.();
    },
  });
}

function TableSelectionHeaderCell(props) {
  const row_selection = props.rowSelection;
  return View(
    {
      class: static_classes([
        "dm-table-selection-cell dm-flex dm-items-center dm-justify-center dm-min-w-0",
        row_selection.headerClass,
      ]),
      style: row_selection.headerStyle || {},
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
        onToggle: row_selection.onSelectAll,
      }),
    ],
  );
}

function TableHeaderCell(props) {
  const column = props.column;
  return View(
    {
      class: static_classes([
        "dm-table-header-cell",
        props.cellClass,
        column.headerClass,
      ]),
      style: {
        ...table_cell_alignment(column.align),
        ...(column.headerStyle || {}),
      },
      attributes: {
        n: `${props.name}-${column.name}-header`,
        role: "columnheader",
        ...(column.headerAttributes || {}),
      },
    },
    [
      View(
        {
          class: "dm-table-head-label",
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

function TableScrollbarHeaderCell(props) {
  return View({
    class: "dm-table-scrollbar-gutter",
    style: computed(props.width, (width) => ({ width: `${width}px` })),
    attributes: {
      n: `${props.name}-scrollbar-gutter`,
      role: "columnheader",
      "aria-hidden": "true",
    },
  });
}

function DataTableHeader(props) {
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
  cells.push(
    Show({
      when: props.scrollbar.state.width,
      ok() {
        return TableScrollbarHeaderCell({
          name: props.name,
          width: props.scrollbar.state.width,
        });
      },
    }),
  );
  return View(
    {
      class: static_classes([
        "dm-table-row dm-grid dm-items-center",
        "dm-table-header",
        props.headerClass,
      ]),
      style: computed(props.scrollbar.state.width, (width) =>
        width > 0 ? { "padding-right": `${width}px` } : {},
      ),
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
      class: static_classes([
        "dm-table-selection-cell dm-flex dm-items-center dm-justify-center dm-min-w-0",
        row_selection.cellClass,
      ]),
      style: row_selection.cellStyle || {},
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
      class: static_classes([
        "dm-table-cell",
        table_resolve(column.cellClass, props.item, context),
      ]),
      style: {
        ...table_cell_alignment(column.align),
        ...(table_resolve(column.cellStyle, props.item, context) || {}),
      },
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
  const last_row_class = table_source(props.lastRow)
    ? computed(props.lastRow, (last_row) =>
        last_row ? "dm-table-row-last" : "",
      )
    : props.lastRow
      ? "dm-table-row-last"
      : "";
  const striped_row_class = table_source(props.rowIndex)
    ? computed(props.rowIndex, (index) =>
        Number(index) % 2 === 1 ? "dm-table-row-striped" : "",
      )
    : Number(props.rowIndex) % 2 === 1
      ? "dm-table-row-striped"
      : "";
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
      class: class_names([
        "dm-table-row dm-grid dm-items-center",
        table_resolve(props.rowClass, item, props.itemSource),
        row_class,
        last_row_class,
        striped_row_class,
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
  const keyed_rows = table_keyed_rows(props.rows, props.rowKey);
  const row_key =
    typeof props.rowKey === "function" ? "__table_row_key" : props.rowKey;
  const scroll_view$ = new Runtime.vm.ScrollViewCore({
    onScroll(position) {
      if (typeof props.onListScroll === "function") {
        props.onListScroll(table_scroll_position(position));
      }
    },
  });
  return TableScrollView(
    props,
    scroll_view$,
    [
      Show({
        when: table_has_rows(props.rows),
        ok() {
          return For({
            key: row_key,
            each: keyed_rows,
            render(entry, index_) {
              return TableDataRow({
                ...props,
                itemSource: table_keyed_item(entry, props.rowKey),
                lastRow: table_last_row(props.rows, index_),
                rowIndex: index_,
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

function TableVirtualList(props) {
  const scroll_top_ = ref(0);
  const viewport_height_ = ref(
    Math.max(1, Number(props.itemHeight) || 1) *
      Math.max(1, Number(props.size) || 1),
  );
  const scroll_view$ = new Runtime.vm.ScrollViewCore({
    onScroll(value) {
      const position = table_scroll_position(value);
      scroll_top_.as(position.scrollTop);
      if (position.clientHeight > 0) {
        viewport_height_.as(position.clientHeight);
      }
      if (typeof props.onListScroll === "function") {
        props.onListScroll(position);
      }
    },
  });
  const content = Show({
    when: table_has_rows(props.rows),
    ok() {
      return Show({
        when: props.renderEnabled,
        ok() {
          return VirtualListView({
            attributes: { n: `${props.name}-virtual-list` },
            style: {
              height: "auto",
              "min-height": "100%",
              overflow: "visible",
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
            externalScroll: true,
            scrollTop: scroll_top_,
            viewportHeight: viewport_height_,
            onMounted: props.onListMounted,
            onScrollTopAdjust(delta) {
              const scroll_top = Math.max(0, scroll_top_.value + delta);
              scroll_top_.as(scroll_top);
              scroll_view$.setScrollTop(scroll_top);
            },
            render(item_, index_) {
              return TableDataRow({
                ...props,
                itemSource: item_,
                lastRow: table_last_row(props.rows, index_),
                rowIndex: index_,
              });
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
  });
  return TableScrollView(
    props,
    scroll_view$,
    [content],
    () => {
      scroll_top_.destroy?.();
      viewport_height_.destroy?.();
    },
  );
}

function TableScrollView(props, scroll_view$, children, on_unmounted) {
  props.onScrollView?.(scroll_view$);
  return Runtime.ui.ScrollViewPrimitive.Root(
    {
      store: scroll_view$,
      class: static_classes(["dm-table-list", props.listClass]),
      style: {
        overflow: "auto",
        ...(props.listStyle || {}),
      },
      attributes: { n: `${props.name}-list` },
      onMounted(event) {
        props.scrollbar.methods.mount(event);
      },
      onUnmounted() {
        props.scrollbar.methods.unmount();
        scroll_view$.destroy();
        on_unmounted?.();
      },
    },
    children,
  );
}

function TableLoading(props) {
  const render_skeleton_row = props.renderSkeletonRow;
  return View(
    {
      class: static_classes([
        "dm-table-list",
        props.listClass,
        props.loadingClass,
      ]),
      style: {
        overflow: "auto",
        ...(props.listStyle || {}),
      },
      attributes: { n: `${props.name}-loading`, role: "status" },
      onMounted(event) {
        props.scrollbar.methods.mount(event);
      },
      onUnmounted() {
        props.scrollbar.methods.unmount();
      },
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
      class: "dm-table-loading-overlay",
      attributes: {
        n: `${props.name}-loading-overlay`,
        role: "status",
        "aria-label": "列表加载中",
      },
    },
    [
      View({
        class: "dm-loading-logo",
        attributes: {
          n: `${props.name}-loading-watermark`,
          "aria-hidden": "true",
        },
      }),
    ],
  );
}

function TablePanel(props, render_list) {
  return View(
    {
      class: static_classes([
        "dm-table-panel dm-panel dm-flex dm-flex-col dm-min-w-0 dm-overflow-hidden",
        props.border !== false && "dm-table--bordered",
        props.panelClass,
      ]),
      style: {
        "--dm-table-columns": table_grid_columns(props),
        ...(props.panelStyle || {}),
      },
      attributes: {
        n: `${props.name}-panel`,
        role: "table",
        ...(props.panelAttributes || {}),
      },
    },
    [
      Show({
        when: props.headerVisible,
        ok() {
          return DataTableHeader(props);
        },
      }),
      render_list(props),
    ],
  );
}

function table_scroll_position(event) {
  const element = table_scroll_element(event);
  return {
    target: element,
    scrollTop: Number(element?.scrollTop) || 0,
    clientHeight: Number(element?.clientHeight) || 0,
    scrollHeight: Number(element?.scrollHeight) || 0,
  };
}

function table_scroll_element(event) {
  const target = event && (event.currentTarget || event.target || event);
  return target && typeof target.get$elm === "function"
    ? target.get$elm()
    : target;
}

function table_scroll_to_top(scroll_view) {
  scroll_view?.setScrollTop(0);
}

function TablePagination(props, scroll_to_top) {
  const pagination = props.pagination;
  if (!pagination) return null;
  return Show({
    when:
      typeof pagination.visible === "undefined"
        ? true
        : pagination.visible,
    ok() {
      return Pagination({
        ...pagination,
        loading: table_loading_state(
          props.status || "normal",
          pagination.loading,
          true,
        ),
        onChange:
          typeof pagination.onChange === "function"
            ? (page, page_size) => {
                scroll_to_top();
                return pagination.onChange(page, page_size);
              }
            : undefined,
      });
    },
  });
}

function table_render(props, render_list) {
  const name = props.name || "table";
  const scrollbar_model = create_table_scrollbar_model();
  let scroll_view = null;
  const status = props.status || "normal";
  const show_header_when_empty = props.showHeaderWhenEmpty !== false;
  const header_visible = table_source(status)
    ? computed(
        status,
        (current_status) =>
          current_status !== "empty" || show_header_when_empty,
      )
    : status !== "empty" || show_header_when_empty;
  const table_props = {
    ...props,
    name,
    scrollbar: scrollbar_model,
    columns: props.columns || [],
    panelClass: props.panelClass,
    headerClass: props.headerClass,
    headerCellClass: props.headerCellClass,
    listClass: props.listClass,
    rowClass: props.rowClass,
    onScrollView(value) {
      scroll_view = value;
      props.onScrollView?.(value);
    },
  };

  function render_empty() {
    return typeof props.renderEmpty === "function"
      ? props.renderEmpty()
      : TableEmpty(table_props);
  }

  function render_error_state() {
    return typeof props.renderError === "function"
      ? props.renderError(props.error)
      : TableError(table_props);
  }

  const table = View(
    {
      class: static_classes([
        "dm-table-container",
        props.containerClass,
      ]),
      attributes: {
        n: `${name}-container`,
        ...(props.containerAttributes || {}),
      },
      onUnmounted() {
        scrollbar_model.methods.destroy();
        props.onUnmounted?.();
      },
    },
    [
      TablePanel(
        { ...table_props, headerVisible: header_visible },
        () =>
          Match({
            when: status,
            cases: {
              initial() {
                return TableLoading({
                  ...table_props,
                  loadingClass: props.loadingClass,
                  skeletonCount: props.skeletonCount || 8,
                });
              },
              empty() {
                return show_header_when_empty
                  ? render_list({ ...table_props, renderEmpty: render_empty })
                  : render_empty();
              },
              error() {
                return render_list({
                  ...table_props,
                  rows: [],
                  renderEmpty: render_error_state,
                });
              },
              normal() {
                return render_list({
                  ...table_props,
                  renderEmpty: render_empty,
                });
              },
            },
          }),
      ),
      Show({
        when: table_loading_visible(status, props.loading),
        ok() {
          return TableLoadingOverlay({ name });
        },
      }),
    ],
  );
  if (!props.pagination) return table;
  return Fragment({}, [
    table,
    TablePagination(props, () => table_scroll_to_top(scroll_view)),
  ]);
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

export function Skeleton(props = {}) {
  return View({
    ...props,
    class: class_names(["dm-skeleton", props.class]),
    attributes: { n: "skeleton", ...(props.attributes || {}) },
  });
}

export function Progress(props = {}) {
  const { store: provided_store, class: extra_class, ...rest } = props;
  const store = require_store("Progress", provided_store, vm.ProgressCore);
  return ui.ProgressPrimitive.Root(
    {
      ...rest,
      store,
      class: class_names(["dm-progress", extra_class]),
    },
    [
      ui.ProgressPrimitive.Indicator({
        store,
        class: "dm-progress-indicator",
      }),
    ],
  );
}

export function Separator(props = {}) {
  const { orientation = "horizontal", ...rest } = props;
  return View(
    {
      ...rest,
      class: class_names([
        "dm-separator",
        orientation === "vertical" ? "is-vertical" : "is-horizontal",
        rest.class,
      ]),
      attributes: {
        n: "separator",
        role: "separator",
        "aria-orientation": orientation,
        ...(rest.attributes || {}),
      },
    },
    [],
  );
}

export function Alert(props = {}, children = []) {
  const { variant = "default", ...rest } = props;
  return View(
    {
      ...rest,
      class: class_names([
        "dm-alert",
        variant === "destructive" || variant === "danger"
          ? "is-destructive"
          : "",
        rest.class,
      ]),
      attributes: {
        n: "alert",
        role: "alert",
        ...(rest.attributes || {}),
      },
    },
    children,
  );
}

export function AlertTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-alert-title", props.class]),
      attributes: { n: "alert-title", ...(props.attributes || {}) },
    },
    children,
  );
}

export function AlertDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      class: class_names(["dm-alert-description", props.class]),
      attributes: { n: "alert-description", ...(props.attributes || {}) },
    },
    children,
  );
}

export default Popover;

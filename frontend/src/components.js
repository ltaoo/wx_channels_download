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

export function LoadingView() {
  return View(
    {
      class:
        "route-loading dm-page dm-grid dm-place-center dm-text-muted dm-p-8",
      role: "status",
    },
    ["页面加载中…"],
  );
}

/**
 * @param {Error} error
 * @param {string} view_name
 */
export function ErrorFallbackView(error, view_name) {
  return View(
    {
      class: "route-error dm-page dm-grid dm-place-center dm-p-8",
      attributes: { role: "alert" },
    },
    [
      View({ class: "route-error-card" }, [
        View(
          {
            class: "route-error-card__icon",
            attributes: { "aria-hidden": "true" },
          },
          [Runtime.Icon({ name: "circle-alert", size: 24 })],
        ),
        View({ class: "route-error-card__content" }, [
          View({ as: "strong", class: "route-error-card__title" }, [
            "页面加载失败",
          ]),
          View({ as: "span", class: "route-error-card__context" }, [
            view_name || "未知页面",
          ]),
        ]),
        View({ as: "pre", class: "route-error-card__detail" }, [
          error.message,
        ]),
      ]),
    ],
  );
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
              as: "span",
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
              as: "span",
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
                    as: "span",
                    class: "dm-input-loading dm-ui-spinner",
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
        ? View(
            {
              class: "dm-ui-select-group-label",
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
          "dm-ui-select-item",
          state.focused ? "is-focused" : "",
          state.selected ? "is-selected" : "",
          state.disabled ? "is-disabled" : "",
        ]),
      ),
    },
    [
      ui.SelectPrimitive.ItemText(
        {
          class: "dm-ui-select-item-text",
          attributes: { n: "select-option-label" },
        },
        [entry.label],
      ),
      ui.SelectPrimitive.ItemIndicator(
        {
          store: entry,
          class: "dm-ui-select-item-indicator",
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
      class: "dm-ui-select-item-root",
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
      event_target?.closest?.(".dm-ui-select") ||
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
            "dm-field dm-ui-select",
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
                class: "dm-ui-select-search",
                attributes: { n: "select-search-input" },
              });
            },
            else() {
              return View(
                {
                  class: class_names([
                    "dm-ui-select-value",
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
              class: "dm-ui-select-action dm-ui-select-clear dm-focus-ring",
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
              class: "dm-ui-select-action dm-ui-select-chevron",
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
              class: class_names(["dm-ui-select-content", content_class]),
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
                  class: "dm-ui-select-viewport",
                  attributes: { n: "select-options" },
                },
                [
                  Show({
                    when: computed(state_, (state) => state.loading),
                    ok() {
                      return View(
                        {
                          class: "dm-ui-select-state",
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
                              class: "dm-ui-select-state",
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
      as: "span",
      class: class_names([
        computed(state_, (state) =>
          static_classes([
            "dm-ui-select-root",
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
              "dm-ui-checkbox",
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
            class: "dm-ui-checkbox-box",
            attributes: { n: "checkbox-box" },
          },
          [
            Show({
              when: indeterminate,
              ok() {
                return View({
                  class: "dm-ui-checkbox-indeterminate",
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
            "dm-ui-drawer-overlay",
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
            "dm-ui-drawer-positioner",
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
                  "dm-ui-drawer-content",
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
                    { store, class: "dm-ui-drawer-header" },
                    [
                      ui.DialogPrimitive.Title(
                        { store, class: "dm-ui-drawer-title" },
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
                      class: "dm-ui-drawer-close dm-focus-ring",
                      attributes: { "aria-label": closeLabel },
                    },
                    [Runtime.Icon({ name: "x", size: 18 })],
                  )
                : null,
              Show({
                when: computed(state_, (state) => Boolean(state.footer)),
                ok() {
                  return ui.DialogPrimitive.Footer(
                    { store, class: "dm-ui-drawer-footer" },
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
    { ...props, class: class_names(["dm-ui-drawer-header", props.class]) },
    children,
  );
}

export function DrawerTitle(props = {}, children = []) {
  return View(
    {
      ...props,
      type: props.type || "h2",
      class: class_names(["dm-ui-drawer-title", props.class]),
    },
    children,
  );
}

export function DrawerDescription(props = {}, children = []) {
  return View(
    {
      ...props,
      type: props.type || "p",
      class: class_names(["dm-ui-drawer-description", props.class]),
    },
    children,
  );
}

export function DrawerBody(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-ui-drawer-body", props.class]) },
    children,
  );
}

export function DrawerFooter(props = {}, children = []) {
  return View(
    { ...props, class: class_names(["dm-ui-drawer-footer", props.class]) },
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

function create_pagination_model(props) {
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
  const ui = {
    next_button$: new vm.ButtonCore({
      variant: "outline",
      size: "icon-sm",
      disabled: Boolean(next_disabled_.value),
      onClick() {
        if (typeof props.onNext === "function") {
          return props.onNext();
        }
        return null;
      },
    }),
    previous_button$: new vm.ButtonCore({
      variant: "outline",
      size: "icon-sm",
      disabled: Boolean(previous_disabled_.value),
      onClick() {
        if (typeof props.onPrevious === "function") {
          return props.onPrevious();
        }
        return null;
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
    state: { page_text: page_text_ },
    ui,
    methods: {
      destroy() {
        unlistens.forEach((unlisten) => unlisten?.());
        Object.values(ui).forEach((store) => store.destroy?.());
        previous_disabled_.destroy?.();
        next_disabled_.destroy?.();
        page_text_.destroy?.();
      },
    },
  };
}

export function Pagination(props = {}) {
  const model = create_pagination_model(props);
  return View(
    {
      as: "nav",
      class: class_names(["dm-pagination", props.class]),
      attributes: {
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
          class: "dm-pagination__summary",
          attributes: { "aria-live": "polite" },
        },
        [props.summary || ""],
      ),
      View({ class: "dm-pagination__controls" }, [
        Button(
          {
            store: model.ui.previous_button$,
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
          [model.state.page_text],
        ),
        Button(
          {
            store: model.ui.next_button$,
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

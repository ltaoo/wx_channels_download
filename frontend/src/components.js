import { BrandError, BrandLoading, Input, PlatformIcon, Popover } from "./dmui.js";

const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Show, View } = Runtime;

const legacy_select_primitive = Runtime.ui && Runtime.ui.SelectPrimitive;
const select_primitive = {
  SelectRoot:
    Runtime.primitive?.SelectRoot || legacy_select_primitive?.Root,
  SelectTrigger:
    Runtime.primitive?.SelectTrigger || legacy_select_primitive?.Trigger,
  SelectIcon:
    Runtime.primitive?.SelectIcon || legacy_select_primitive?.Icon,
  SelectContent:
    Runtime.primitive?.SelectContent || legacy_select_primitive?.Content,
  SelectViewport:
    Runtime.primitive?.SelectViewport || legacy_select_primitive?.Viewport,
  SelectItem:
    Runtime.primitive?.SelectItem || legacy_select_primitive?.Item,
  SelectItemText:
    Runtime.primitive?.SelectItemText || legacy_select_primitive?.ItemText,
  SelectItemIndicator:
    Runtime.primitive?.SelectItemIndicator ||
    legacy_select_primitive?.ItemIndicator,
};

function select_class_names(values) {
  return Runtime.classNames(values.filter(Boolean));
}

function select_static_classes(values) {
  return values.filter(Boolean).join(" ");
}

function is_select_source(value) {
  return Boolean(
    value &&
      typeof value === "object" &&
      "value" in value &&
      typeof value.subscribe === "function",
  );
}

function filter_select_options(options, keyword) {
  const normalized_keyword = String(keyword || "")
    .trim()
    .toLocaleLowerCase();
  if (!normalized_keyword) return options;
  return options.filter((entry) =>
    String(entry.search_text || `${entry.label || ""} ${entry.value || ""}`)
      .toLocaleLowerCase()
      .includes(normalized_keyword),
  );
}

function require_select_store(component, store) {
  if (!store) {
    throw new TypeError(`${component} 需要对应的 Timeless select store`);
  }
  return store;
}

function require_select_primitives() {
  const missing = Object.entries(select_primitive)
    .filter(([, primitive]) => typeof primitive !== "function")
    .map(([name]) => name);
  if (missing.length > 0) {
    throw new Error(`Timeless 缺少选择器组件：${missing.join(", ")}`);
  }
  return select_primitive;
}

function ensure_select_trigger_reference(store, event) {
  const event_target = event.currentTarget || event.target;
  const trigger_element =
    event_target?.get$elm?.() ||
    event_target?.closest?.(".dm-select") ||
    event_target;
  if (!trigger_element?.getBoundingClientRect) return;

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

function select_search_input(props) {
  return Input({
    store: props.store.search_input$,
    rootAttributes: { n: `${props.name}-search` },
    prefix: Runtime.Icon({
      name: "search",
      size: 14,
      attributes: { n: `${props.name}-search-icon` },
    }),
    attributes: {
      n: `${props.name}-search-input`,
      type: "search",
      autocomplete: "off",
      "aria-label": props.label,
    },
    onPointerDown(event) {
      props.store.enableSearch?.();
      event.stopPropagation();
    },
    onClick(event) {
      event.stopPropagation();
    },
    onKeyDown(event) {
      event.stopPropagation();
      props.onKeyDown?.(event);
    },
  });
}

function select_entry(select_store, entry, render_label, semantic_name) {
  const primitives = require_select_primitives();
  const item_ = Runtime.refobj(entry.state);
  const unlisten = entry.onStateChange((state) => item_.as(state));
  const item = primitives.SelectItem(
    {
      select$: select_store,
      item$: entry,
      attributes: {
        n: `${semantic_name}-option`,
        role: "option",
        "aria-selected": Runtime.computed(item_, (state) =>
          String(state.selected),
        ),
      },
      class: Runtime.computed(item_, (state) =>
        select_static_classes([
          "dm-select-item",
          state.focused ? "is-focused" : "",
          state.selected ? "is-selected" : "",
          state.disabled ? "is-disabled" : "",
        ]),
      ),
    },
    [
      primitives.SelectItemText(
        {
          class: "dm-select-item-text",
          attributes: { n: `${semantic_name}-option-label` },
        },
        [render_label(entry)],
      ),
      primitives.SelectItemIndicator(
        {
          store: entry,
          class: "dm-select-item-indicator",
          attributes: { n: `${semantic_name}-option-indicator` },
        },
        [
          Runtime.Icon({
            name: "check",
            size: 12,
            attributes: { n: `${semantic_name}-option-check-icon` },
          }),
        ],
      ),
    ],
  );

  return View(
    {
      class: "dm-select-item-root",
      attributes: { n: `${semantic_name}-option-root` },
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

function EntitySelect(props, render_label, render_value) {
  const primitives = require_select_primitives();
  const semantic_name = props.name || "entity-select";
  const store = require_select_store(semantic_name, props.store);
  const state_ = Runtime.refobj(store.state);
  const search_keyword_ = Runtime.ref("");
  const search_placeholder = props.searchPlaceholder || "搜索选项";
  let was_open = Boolean(store.state.open);

  function reset_search() {
    search_keyword_.as("");
    if (store.search_input$?.state?.placeholder !== search_placeholder) {
      store.search_input$?.setPlaceholder?.(search_placeholder);
    }
    store.clearSearch?.();
    store.enableSearch?.();
  }

  const unlisten = store.onStateChange((state) => {
    const opening = Boolean(state.open) && !was_open;
    was_open = Boolean(state.open);
    if (opening) reset_search();
    state_.as(state);
  });
  const search_unlisten = store.onSearchChange?.((value) =>
    search_keyword_.as(String(value || "")),
  );
  const options_ = Runtime.combine(
    {
      state: state_,
      filter: props.filterSource,
      keyword: search_keyword_,
    },
    (source) => {
      let options = source.state.options || store.raw_options || [];
      if (props.filterOptions) {
        options = props.filterOptions(options, source.filter);
      }
      return filter_select_options(options, source.keyword);
    },
  );
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

  const primitive_select = primitives.SelectRoot(
    { store },
    [
      primitives.SelectTrigger(
        {
          store,
          class: select_class_names([
            "dm-field dm-select",
            Runtime.computed(state_, (state) =>
              select_static_classes([
                state.open ? "is-open" : "",
                state.disabled ? "is-disabled" : "",
              ]),
            ),
            props.class,
          ]),
          attributes: {
            n: `${semantic_name}-trigger`,
            type: "button",
            ...(props.attributes || {}),
          },
          onPointerDown(event) {
            event.preventDefault();
            event.stopPropagation();
            event.stopImmediatePropagation?.();
            ensure_select_trigger_reference(store, event);
            suppress_click_once();
            store.handleClickTrigger();
          },
          onClick(event) {
            if (consume_click_suppression()) return;
            event.preventDefault();
            event.stopPropagation();
            ensure_select_trigger_reference(store, event);
            store.handleClickTrigger();
          },
        },
        [
          View(
            {
              class: select_class_names([
                "dm-select-value",
                Runtime.computed(state_, (state) =>
                  state.selectedOption ? "has-value" : "is-placeholder",
                ),
              ]),
              attributes: { n: `${semantic_name}-value` },
            },
            render_value
              ? [render_value(state_, semantic_name)]
              : [
                  Runtime.computed(
                    state_,
                    (state) =>
                      state.selectedOption?.label ??
                      state.selectedOption?.value ??
                      state.placeholder ??
                      "请选择",
                  ),
                ],
          ),
          primitives.SelectIcon(
            {
              store,
              class: "dm-select-action dm-select-chevron",
              attributes: { n: `${semantic_name}-chevron` },
            },
            [
              Runtime.Icon({
                name: "chevron-down",
                size: 14,
                attributes: { n: `${semantic_name}-chevron-icon` },
                class: select_class_names([
                  Runtime.computed(state_, (state) =>
                    state.open ? "is-open" : "",
                  ),
                ]),
              }),
            ],
          ),
        ],
      ),
      Show({
        when: Runtime.computed(state_, (state) => state.open),
        ok() {
          return primitives.SelectContent(
            {
              store,
              class: select_class_names([
                "dm-select-content dm-entity-select-content",
                props.contentClass,
              ]),
              attributes: {
                n: `${semantic_name}-popup`,
                role: "listbox",
              },
              animation: { in: "is-entering", out: "is-exiting" },
            },
            () => [
              Show({
                when: Runtime.computed(state_, (state) => state.search),
                ok() {
                  return View(
                    {
                      class: "dm-entity-select-search-wrap",
                      attributes: { n: `${semantic_name}-search-wrap` },
                    },
                    [
                      select_search_input({
                        store,
                        name: semantic_name,
                        label: search_placeholder,
                        onKeyDown(event) {
                          switch (event.key) {
                            case "ArrowDown":
                              event.preventDefault();
                              store.focusNextOption();
                              break;
                            case "ArrowUp":
                              event.preventDefault();
                              store.focusPrevOption();
                              break;
                            case "Enter":
                              event.preventDefault();
                              store.selectFocusedOption();
                              break;
                            case "Escape":
                              event.preventDefault();
                              store.hide();
                              break;
                            default:
                              break;
                          }
                        },
                      }),
                    ],
                  );
                },
              }),
              primitives.SelectViewport(
                {
                  store,
                  class: "dm-select-viewport",
                  attributes: { n: `${semantic_name}-options` },
                },
                [
                  Show({
                    when: Runtime.computed(state_, (state) => state.loading),
                    ok() {
                      return View(
                        {
                          class: "dm-select-state",
                          attributes: { n: `${semantic_name}-loading-state` },
                        },
                        ["加载中…"],
                      );
                    },
                    else() {
                      return Show({
                        when: Runtime.computed(
                          options_,
                          (options) => options.length > 0,
                        ),
                        ok() {
                          return Runtime.For({
                            each: options_,
                            render(entry) {
                              return select_entry(
                                store,
                                entry,
                                render_label,
                                semantic_name,
                              );
                            },
                          });
                        },
                        else() {
                          return View(
                            {
                              class: "dm-select-state",
                              attributes: { n: `${semantic_name}-empty-state` },
                            },
                            [props.emptyText || "暂无选项"],
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
      class: select_class_names([
        Runtime.computed(state_, (state) =>
          select_static_classes([
            "dm-select-root dm-entity-select-root",
            state.open ? "is-open" : "",
            state.disabled ? "is-disabled" : "",
          ]),
        ),
        props.rootClass,
      ]),
      style: props.style,
      attributes: { n: `${semantic_name}-root` },
      onUnmounted() {
        consume_click_suppression();
        if (typeof unlisten === "function") unlisten();
        if (typeof search_unlisten === "function") search_unlisten();
        options_.destroy?.();
        search_keyword_.destroy?.();
        state_.destroy?.();
        props.onUnmounted?.();
      },
    },
    [primitive_select],
  );
}

function platform_selected_value(state_, semantic_name) {
  const favicon_ = Runtime.computed(state_, (state) => {
    const platform_id = String(state.selectedOption?.value || "");
    return (window.PLATFORM_FAVICONS || {})[platform_id] || "";
  });
  const has_favicon_ = Runtime.computed(favicon_, (favicon) =>
    String(favicon).includes("#"),
  );
  const label_ = Runtime.computed(
    state_,
    (state) =>
      state.selectedOption?.label ??
      state.selectedOption?.value ??
      state.placeholder ??
      "请选择",
  );
  return View(
    { class: "dm-entity-select-value" },
    [
      Show({
        when: has_favicon_,
        ok() {
          return Runtime.SVG.SVG(
            {
              class: "dm-entity-select-value__icon",
              attributes: {
                n: `${semantic_name}-value-icon`,
                viewBox: "0 0 32 32",
                "aria-hidden": "true",
                focusable: "false",
              },
            },
            [
              Runtime.SVG.Use({
                attributes: { href: favicon_ },
              }),
            ],
          );
        },
      }),
      View({ class: "dm-entity-select-value__label" }, [label_]),
    ],
  );
}

function PlatformPopoverItem(props) {
  const entry = props.entry;
  const selected_ = Runtime.computed(
    props.state,
    (state) => String(state.value ?? "") === String(entry.value ?? ""),
  );
  const favicon =
    (window.PLATFORM_FAVICONS || {})[String(entry.value || "")] || "";
  return View(
    {
      type: "button",
      class: select_class_names([
        "dm-platform-select-grid-item dm-focus-ring",
        Runtime.computed(selected_, (selected) =>
          selected ? "is-selected" : "",
        ),
      ]),
      attributes: {
        n: `${props.name}-option`,
        type: "button",
        role: "option",
        "aria-selected": Runtime.computed(selected_, String),
        title: entry.label,
        disabled: entry.disabled ? true : undefined,
      },
      onClick(event) {
        event.preventDefault();
        if (entry.disabled) return;
        props.store.setValue(entry.value);
        props.store.clearSearch?.();
        props.store.enableSearch?.();
        props.popoverStore.hide();
      },
    },
    [
      View(
        {
          class: "dm-platform-select-grid-item__icon",
          attributes: { "aria-hidden": "true" },
        },
        [
          favicon
            ? PlatformIcon({
                class: "dm-platform-select-grid-item__image",
                favicon,
                name: `${props.name}-option-icon`,
              })
            : Runtime.Icon({ name: "globe", size: 20 }),
        ],
      ),
      View({ class: "dm-platform-select-grid-item__label" }, [entry.label]),
    ],
  );
}

function PlatformPopoverSelect(props) {
  const semantic_name = props.name || "platform-select";
  const store = require_select_store(semantic_name, props.store);
  const state_ = Runtime.refobj(store.state);
  const popover_store =
    props.popoverStore || new Runtime.vm.PopoverCore();
  const owns_popover_store = !props.popoverStore;
  const popover_state_ = Runtime.refobj(popover_store.state);
  const trigger_active_ = Runtime.ref(false);
  const search_keyword_ = Runtime.ref("");
  const search_placeholder = props.searchPlaceholder || "搜索平台";

  function reset_search() {
    search_keyword_.as("");
    if (store.search_input$?.state?.placeholder !== search_placeholder) {
      store.search_input$?.setPlaceholder?.(search_placeholder);
    }
    store.clearSearch?.();
    store.enableSearch?.();
  }

  const platform_options_ = Runtime.combine(
    { state: state_, keyword: search_keyword_ },
    (source) =>
      filter_select_options(
        source.state.options || store.raw_options || [],
        source.keyword,
      ),
  );
  const unlistens = [
    store.onStateChange((state) => state_.as(state)),
    popover_store.onStateChange((state) => popover_state_.as(state)),
    store.onSearchChange?.((value) =>
      search_keyword_.as(String(value || "")),
    ),
    popover_store.onHide(reset_search),
  ];

  return Popover(
    {
      store: popover_store,
      side: "bottom",
      align: "start",
      triggerClass: select_class_names([
        "dm-platform-select-popover-trigger",
        props.class,
        props.rootClass,
      ]),
      class: select_class_names([
        "dm-platform-select-popover",
        props.contentClass,
      ]),
      content: [
        View(
          {
            class: "dm-platform-select-search-wrap",
            attributes: { n: `${semantic_name}-search-wrap` },
          },
          [
            select_search_input({
              store,
              name: semantic_name,
              label: search_placeholder,
              onKeyDown(event) {
                if (event.key === "Escape") {
                  event.preventDefault();
                  popover_store.hide();
                }
              },
            }),
          ],
        ),
        Show({
          when: Runtime.computed(
            platform_options_,
            (options) => options.length > 0,
          ),
          ok() {
            return View(
              {
                class: "dm-platform-select-grid",
                attributes: {
                  n: `${semantic_name}-options`,
                  role: "listbox",
                  "aria-label": "平台列表",
                },
              },
              [
                Runtime.For({
                  each: platform_options_,
                  render(entry) {
                    return PlatformPopoverItem({
                      entry,
                      name: semantic_name,
                      popoverStore: popover_store,
                      state: state_,
                      store,
                    });
                  },
                }),
              ],
            );
          },
          else() {
            return View(
              {
                class: "dm-select-state",
                attributes: { n: `${semantic_name}-empty-state` },
              },
              [props.emptyText || "暂无平台"],
            );
          },
        }),
      ],
      onUnmounted() {
        unlistens.forEach((unlisten) => {
          if (typeof unlisten === "function") unlisten();
        });
        state_.destroy?.();
        popover_state_.destroy?.();
        trigger_active_.destroy?.();
        search_keyword_.destroy?.();
        platform_options_.destroy?.();
        if (owns_popover_store) popover_store.destroy?.();
        props.onUnmounted?.();
      },
    },
    [
      View(
        {
          type: "button",
          class: select_class_names([
            "dm-field dm-select dm-platform-select-trigger",
            Runtime.computed(popover_state_, (state) =>
              state.visible ? "is-open" : "",
            ),
            Runtime.computed(trigger_active_, (active) =>
              active ? "is-active" : "",
            ),
            Runtime.computed(state_, (state) =>
              state.disabled ? "is-disabled" : "",
            ),
          ]),
          attributes: {
            n: `${semantic_name}-trigger`,
            type: "button",
            ...(props.attributes || {}),
            disabled: Runtime.computed(state_, (state) =>
              state.disabled ? true : undefined,
            ),
          },
          onMouseEnter() {
            if (!state_.value.disabled) trigger_active_.as(true);
          },
          onMouseLeave() {
            trigger_active_.as(false);
          },
        },
        [
          View(
            {
              class: select_class_names([
                "dm-select-value",
                Runtime.computed(state_, (state) =>
                  state.selectedOption ? "has-value" : "is-placeholder",
                ),
              ]),
              attributes: { n: `${semantic_name}-value` },
            },
            [platform_selected_value(state_, semantic_name)],
          ),
          View(
            {
              class: "dm-select-action dm-select-chevron",
              attributes: {
                n: `${semantic_name}-chevron`,
                "aria-hidden": "true",
              },
            },
            [
              Runtime.Icon({
                name: "chevron-down",
                size: 14,
                class: select_class_names([
                  Runtime.computed(popover_state_, (state) =>
                    state.visible ? "is-open" : "",
                  ),
                ]),
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

function account_option_label(entry) {
  const account = entry.account || {};
  const platform_id = String(account.platform_id || "");
  const platform_name =
    (window.PLATFORM_NAMES || {})[platform_id] || platform_id;
  const description = platform_name || (!entry.value ? "所有平台" : "");
  return View(
    { class: "dm-entity-select-option" },
    [
      View(
        {
          class: "dm-entity-select-option__avatar",
          attributes: { "aria-hidden": "true" },
        },
        [
          Show({
            when: account.avatar_url,
            ok() {
              return Runtime.Img({
                class: "dm-entity-select-option__image",
                src: account.avatar_url,
                attributes: {
                  alt: "",
                  loading: "lazy",
                  referrerpolicy: "no-referrer",
                },
              });
            },
            else() {
              return Runtime.Icon({
                name: "user",
                size: 15,
                class: "dm-entity-select-option__avatar-fallback",
              });
            },
          }),
        ],
      ),
      View({ class: "dm-entity-select-option__body" }, [
        View({ class: "dm-entity-select-option__label" }, [entry.label]),
        Show({
          when: description,
          ok() {
            return View(
              { class: "dm-entity-select-option__description" },
              [description],
            );
          },
        }),
      ]),
    ],
  );
}

function account_selected_value(state_, semantic_name) {
  const avatar_ = Runtime.computed(
    state_,
    (state) => state.selectedOption?.account?.avatar_url || "",
  );
  const label_ = Runtime.computed(
    state_,
    (state) =>
      state.selectedOption?.label ??
      state.selectedOption?.value ??
      state.placeholder ??
      "请选择",
  );

  return View(
    { class: "dm-entity-select-value" },
    [
      Show({
        when: Runtime.computed(avatar_, Boolean),
        ok() {
          return Runtime.Img({
            class: "dm-entity-select-value__avatar",
            src: avatar_,
            attributes: {
              n: `${semantic_name}-value-avatar`,
              alt: "",
              loading: "lazy",
              referrerpolicy: "no-referrer",
            },
          });
        },
        else() {
          return Runtime.Icon({
            name: "user",
            size: 20,
            class:
              "dm-entity-select-value__avatar dm-entity-select-value__avatar--fallback",
            attributes: {
              n: `${semantic_name}-value-avatar-fallback`,
              "aria-hidden": "true",
            },
          });
        },
      }),
      View({ class: "dm-entity-select-value__label" }, [label_]),
    ],
  );
}

export function PlatformSelect(props = {}) {
  return PlatformPopoverSelect({
    name: "platform-select",
    emptyText: "暂无平台",
    searchPlaceholder: "搜索平台",
    ...props,
  });
}

export function AccountSelect(props = {}) {
  const store = require_select_store("account-select", props.store);
  const provided_on_unmounted = props.onUnmounted;
  const platform_source = props.platform;

  function account_platform(entry) {
    return String(
      entry.account?.platform_id || entry.platform_id || "",
    ).trim();
  }

  function filter_accounts(options, platform) {
    const platform_id = String(platform || "").trim();
    if (!platform_id) return options;
    return options.filter(
      (entry) => !entry.value || account_platform(entry) === platform_id,
    );
  }

  function clear_mismatched_account(platform) {
    const selected = store.selected_item$;
    const platform_id = String(platform || "").trim();
    if (
      selected?.value &&
      platform_id &&
      account_platform(selected) !== platform_id
    ) {
      store.setValue("");
    }
  }

  clear_mismatched_account(
    is_select_source(platform_source)
      ? platform_source.value
      : platform_source,
  );
  const platform_unlisten = is_select_source(platform_source)
    ? platform_source.subscribe({ onChange: clear_mismatched_account })
    : null;

  return EntitySelect(
    {
      name: "account-select",
      emptyText: "暂无账号",
      searchPlaceholder: "搜索账号",
      ...props,
      contentClass: select_class_names([
        "dm-account-select-content",
        props.contentClass,
      ]),
      filterOptions: filter_accounts,
      filterSource: platform_source,
      onUnmounted() {
        if (typeof platform_unlisten === "function") platform_unlisten();
        provided_on_unmounted?.();
      },
    },
    account_option_label,
    account_selected_value,
  );
}

export function LoadingView() {
  return View(
    {
      class:
        "route-loading page dm-grid dm-place-center dm-text-muted dm-p-8",
      attributes: {
        n: "route-loading-placeholder",
        role: "status",
        "aria-label": "页面加载中",
      },
    },
    [
      BrandLoading({
        size: 112,
        name: "route-loading-logo",
        label: "正在载入页面",
        labelVisible: true,
        decorative: true,
      }),
    ],
  );
}

/**
 * @param {Error} error
 * @param {string} view_name
 */
export function ErrorFallbackView(error, view_name) {
  return View(
    {
      class: "route-error page dm-grid dm-place-center dm-p-8",
      attributes: { role: "alert" },
    },
    [
      View({ class: "route-error-card" }, [
        View(
          {
            class: "route-error-card__icon",
            attributes: { "aria-hidden": "true" },
          },
          [BrandError({ size: 76, name: "route-error-symbol" })],
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

export { PlatformIcon, PlatformTag as TablePlatformBadge } from "./dmui.js";

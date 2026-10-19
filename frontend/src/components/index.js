import { request } from "@/biz/request.js";

const Runtime = window.Timeless;

if (!Runtime) {
  throw new Error("组件库无法启动：Timeless 运行时未加载");
}

const { Show, View } = Runtime;

const legacy_select_primitive = Runtime.ui && Runtime.ui.SelectPrimitive;
const select_primitive = {
  SelectRoot: Runtime.primitive?.SelectRoot || legacy_select_primitive?.Root,
  SelectTrigger:
    Runtime.primitive?.SelectTrigger || legacy_select_primitive?.Trigger,
  SelectIcon: Runtime.primitive?.SelectIcon || legacy_select_primitive?.Icon,
  SelectContent:
    Runtime.primitive?.SelectContent || legacy_select_primitive?.Content,
  SelectViewport:
    Runtime.primitive?.SelectViewport || legacy_select_primitive?.Viewport,
  SelectItem: Runtime.primitive?.SelectItem || legacy_select_primitive?.Item,
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

  const primitive_select = primitives.SelectRoot({ store }, [
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
              "dm-select-content flex min-w-180px flex-col [&_>_.dm-select-viewport]:min-h-0 [&_>_.dm-select-viewport]:flex-[1_1_auto] [&_>_.dm-select-viewport]:max-h-none",
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
                    class: "[.dm-select-content_>_&]:min-w-0 [.dm-select-content_>_&]:flex-[0_0_auto] [.dm-select-content_>_&]:m-[var(--dm-space-1)_var(--dm-space-1)_0]",
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
  ]);

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
  return View({ class: "flex min-w-0 items-center gap-2" }, [
    Show({
      when: has_favicon_,
      ok() {
        return Runtime.SVG.SVG(
          {
            class: "w-5 h-5 flex-[0_0_auto]",
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
    View({ class: "min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap" }, [label_]),
  ]);
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
        "flex min-w-0 min-h-52px flex-col items-center justify-center gap-1 p-1 text-color-text-secondary bg-transparent border-[var(--dm-border-width)_solid_transparent] rounded-control cursor-pointer hover:text-color-primary hover:bg-color-primary-soft hover:border-color-[color-mix(in_srgb,_var(--dm-color-primary)_35%,_transparent)] [&.is-selected]:text-color-primary [&.is-selected]:bg-color-primary-soft [&.is-selected]:border-color-[color-mix(in_srgb,_var(--dm-color-primary)_35%,_transparent)] dm-focus-ring",
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
          class: "w-5 h-5 grid flex-[0_0_auto] place-items-center",
          attributes: { "aria-hidden": "true" },
        },
        [
          favicon
            ? PlatformIcon({
                class: "w-5 h-5",
                favicon,
                name: `${props.name}-option-icon`,
              })
            : Runtime.Icon({ name: "globe", size: 20 }),
        ],
      ),
      View({ class: "w-full overflow-hidden leading-tight text-center text-ellipsis whitespace-nowrap" }, [entry.label]),
    ],
  );
}

function PlatformPopoverSelect(props) {
  const semantic_name = props.name || "platform-select";
  const store = require_select_store(semantic_name, props.store);
  const state_ = Runtime.refobj(store.state);
  const popover_store = props.popoverStore || new Runtime.vm.PopoverCore();
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
    store.onSearchChange?.((value) => search_keyword_.as(String(value || ""))),
    popover_store.onHide(reset_search),
  ];

  return Popover(
    {
      store: popover_store,
      side: "bottom",
      align: "start",
      triggerClass: select_class_names([
        "inline-flex",
        props.class,
        props.rootClass,
      ]),
      class: select_class_names([
        "box-sizing-border-box w-[min(420px,_calc(100vw_-_var(--dm-space-6)))] max-w-[calc(100vw_-_var(--dm-space-6))] p-2 text-base",
        props.contentClass,
      ]),
      content: [
        View(
          {
            class: "min-w-0 mb-1-5",
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
                class: "grid grid-cols-[repeat(5,_minmax(0,_1fr))] gap-1",
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
            "dm-field dm-select group-dm-platform-select-trigger w-180px [&.is-active]:bg-color-bg-surface [&.is-active]:border-color-color-primary [&.is-active]:outline-0 [&.is-active]:shadow-focus-ring [&.is-open]:bg-color-bg-surface [&.is-open]:border-color-color-primary [&.is-open]:outline-0 [&.is-open]:shadow-focus-ring",
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
  return View({ class: "flex w-full min-w-0 items-center gap-2-5 text-left" }, [
    View(
      {
        class: "grid overflow-hidden w-7 h-7 flex-[0_0_auto] place-items-center text-color-text-secondary bg-color-bg-interactive border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-full",
        attributes: { "aria-hidden": "true" },
      },
      [
        Show({
          when: account.avatar_url,
          ok() {
            return Runtime.Img({
              class: "w-full h-full object-cover",
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
              class: "opacity-[0.8]",
            });
          },
        }),
      ],
    ),
    View({ class: "min-w-0 overflow-hidden text-ellipsis whitespace-nowrap flex flex-1 flex-col items-start justify-center gap-1px text-left" }, [
      View({ class: "group-dm-entity-select-option__label min-w-0 overflow-hidden text-ellipsis whitespace-nowrap w-full leading-tight text-left" }, [entry.label]),
      Show({
        when: description,
        ok() {
          return View({ class: "w-full overflow-hidden text-color-text-secondary leading-tight text-left text-ellipsis whitespace-nowrap" }, [
            description,
          ]);
        },
      }),
    ]),
  ]);
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

  return View({ class: "flex min-w-0 items-center gap-2" }, [
    Show({
      when: Runtime.computed(avatar_, Boolean),
      ok() {
        return Runtime.Img({
          class: "w-5 h-5 flex-[0_0_auto] object-cover rounded-full",
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
            "w-5 h-5 flex-[0_0_auto] object-cover rounded-full p-2px text-color-text-secondary bg-color-bg-interactive",
          attributes: {
            n: `${semantic_name}-value-avatar-fallback`,
            "aria-hidden": "true",
          },
        });
      },
    }),
    View({ class: "min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap" }, [label_]),
  ]);
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
    return String(entry.account?.platform_id || entry.platform_id || "").trim();
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
    is_select_source(platform_source) ? platform_source.value : platform_source,
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
        "min-w-240px [&_.dm-select-item]:min-h-48px [&_.dm-select-item]:py-1-5 [&_.dm-select-item-text]:min-w-0 [&_.dm-select-item-text]:flex-1 [&_.dm-select-item.is-selected_.group-dm-entity-select-option__label]:text-color-primary [&_.dm-select-item.is-selected_.group-dm-entity-select-option__label]:font-semibold",
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
      class: "flex min-h-full items-center justify-center p-8 text-color-text-secondary min-w-0 min-h-full font-sans bg-color-bg-canvas dm-grid dm-place-center dm-text-muted dm-p-8",
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
      class: "flex min-h-full items-center justify-center p-8 bg-[radial-gradient(_circle_at_50%_38%,_var(--dm-color-danger-soft),_transparent_min(360px,_46vw)_),_var(--dm-color-bg-canvas)] min-w-0 min-h-full text-color-text-primary font-sans dm-grid dm-place-center dm-p-8",
      attributes: { role: "alert" },
    },
    [
      View({ class: "relative w-[min(100%,_520px)] grid grid-cols-[auto_minmax(0,_1fr)] gap-4 p-6 overflow-hidden text-color-text-primary bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-danger-border)] rounded-xl shadow-lg [&::before]:content-[\"\"] [&::before]:absolute [&::before]:inset-[0_auto_0_0] [&::before]:w-1 [&::before]:bg-color-danger" }, [
        View(
          {
            class: "w-76px h-57px grid place-items-center",
            attributes: { "aria-hidden": "true" },
          },
          [BrandError({ size: 76, name: "route-error-symbol" })],
        ),
        View({ class: "min-w-0 flex flex-col justify-center gap-1" }, [
          View({ as: "strong", class: "font-display text-xl leading-tight" }, [
            "页面加载失败",
          ]),
          View({ as: "span", class: "overflow-hidden text-color-text-secondary text-sm leading-base text-ellipsis whitespace-nowrap" }, [
            view_name || "未知页面",
          ]),
        ]),
        View({ as: "pre", class: "grid-column-[1_/_-1] max-w-full m-[var(--dm-space-1)_0_0] p-[var(--dm-space-3)_var(--dm-space-4)] overflow-auto text-color-danger font-mono text-sm leading-relaxed whitespace-pre-wrap overflow-wrap-anywhere bg-color-danger-soft border-[var(--dm-border-width)_solid_var(--dm-color-danger-border)] rounded-md" }, [error.message]),
      ]),
    ],
  );
}

// --- Tag system -------------------------------------------------------------

// ContentTagBadge renders one user tag as a removable chip in a content row.
function ContentTagBadge(props) {
  const tag = props.tag || {};
  return Tag(
    {
      class: "flex-[0_0_auto]",
      attributes: {
        n: `content-tag-${tag.id}`,
        title: String(tag.name || ""),
      },
    },
    [
      String(tag.name || ""),
      Show({
        when: typeof props.onRemove === "function",
        ok() {
          return View(
            {
              type: "button",
              class: "inline-grid place-items-center p-0 w-14px h-14px text-inherit bg-transparent border-0 rounded-[50%] cursor-pointer opacity-[0.7] transition-[opacity_0.15s_ease,_background_0.15s_ease,_color_0.15s_ease] hover:opacity-1 hover:bg-color-danger-soft hover:text-color-danger",
              attributes: {
                n: `content-tag-remove-${tag.id}`,
                type: "button",
                "aria-label": `移除标签 ${tag.name}`,
              },
              onClick(event) {
                event.preventDefault();
                event.stopPropagation();
                props.onRemove(tag);
              },
            },
            [Runtime.Icon({ name: "x", size: 10 })],
          );
        },
      }),
    ],
  );
}

// TagSelect is an antd Select mode="tags" style control: a "+" trigger opens a
// popover with a searchable tag list, supports creating a new tag on Enter/Space
// when there is no exact match, and persists the selection immediately.
export function TagSelect(props = {}) {
  const SEARCH_DEBOUNCE_MS = 300;
  const content_id = props.contentId || props.content_id || "";
  const popover_store = new Runtime.vm.PopoverCore();
  const selected_ =
    props.tagsRef && typeof props.tagsRef.as === "function"
      ? props.tagsRef
      : Runtime.ref((props.tags || []).slice());
  const all_tags_ = Runtime.ref([]);
  const keyword_ = Runtime.ref("");
  let keyword_timer = null;
  let tag_select_destroyed = false;

  function set_search_keyword(value, immediate = false) {
    if (keyword_timer) {
      window.clearTimeout(keyword_timer);
      keyword_timer = null;
    }
    if (immediate) {
      keyword_.as(String(value || ""));
      return;
    }
    keyword_timer = window.setTimeout(() => {
      keyword_timer = null;
      if (tag_select_destroyed) return;
      keyword_.as(String(value || ""));
    }, SEARCH_DEBOUNCE_MS);
  }

  const search_input = new Runtime.vm.InputCore({
    defaultValue: "",
    placeholder: "搜索或创建标签",
    allowClear: true,
    autocomplete: false,
    onEnter() {
      const value = String(search_input.value || "").trim();
      if (!value) return;
      set_search_keyword(value, true);
      const match = (all_tags_.value || []).find((t) => t.name === value);
      if (match) {
        toggle(match);
        return;
      }
      create_and_toggle(value);
    },
  });

  const tag_list_request = new Runtime.kit.RequestCore(
    (params) => {
      return request.get("/api/tag/list", params);
    },
    { client: props.client },
  );
  const tag_set_request = new Runtime.kit.RequestCore(
    (params) => {
      return request.post("/api/tag/content/set", params);
    },
    { client: props.client },
  );
  const tag_create_request = new Runtime.kit.RequestCore(
    (params) => {
      request.post("/api/tag/create", params);
    },
    { client: props.client },
  );

  async function load_all() {
    const result = await tag_list_request.run({});
    if (result.error) return;
    const data =
      result.data && result.data.data
        ? result.data.data
        : Array.isArray(result.data)
          ? result.data
          : [];
    all_tags_.as(Array.isArray(data) ? data : []);
  }

  async function persist(next) {
    selected_.as(next || []);
    const result = await tag_set_request.run({
      content_id,
      tag_ids: (next || []).map((t) => t.id),
    });
    if (!result.error && typeof props.onChange === "function") {
      props.onChange(next || []);
    }
  }

  function toggle(tag) {
    const current = selected_.value || [];
    const next = current.some((t) => t.id === tag.id)
      ? current.filter((t) => t.id !== tag.id)
      : current.concat([tag]);
    persist(next);
  }

  async function create_and_toggle(name) {
    const clean = String(name || "").trim();
    if (!clean) return;
    const result = await tag_create_request.run({ name: clean });
    if (result.error) {
      return;
    }
    const resp = result.data || {};
    const tag = resp.data ? resp.data : resp;
    if (tag && tag.id) {
      const current = selected_.value || [];
      if (!current.some((t) => t.id === tag.id)) {
        persist(current.concat([tag]));
      }
    }
    load_all();
  }

  let popover_was_visible = false;
  const popover_unlisten = popover_store.onStateChange((state) => {
    const visible = Boolean(state && state.visible);
    if (visible && !popover_was_visible) {
      set_search_keyword("", true);
      load_all();
      setTimeout(() => search_input.focus(), 0);
    }
    popover_was_visible = visible;
  });
  const popover_hide_unlisten = popover_store.onHide(() => {
    set_search_keyword("", true);
  });

  const filtered_ = Runtime.combine({ all: all_tags_, kw: keyword_ }, (s) =>
    filter_select_options(
      (s.all || []).map((t) => ({ label: t.name, value: t.id, name: t.name })),
      s.kw,
    ),
  );

  const selected_ids_ = Runtime.computed(selected_, (list) =>
    (list || []).map((t) => t.id),
  );

  const trigger = View(
    {
      type: "button",
      class: "dm-tag flex-[0_0_auto] w-5 min-h-5 p-0 cursor-pointer transition-[color_0.15s_ease,_background_0.15s_ease,_border-color_0.15s_ease] hover:text-color-primary hover:bg-color-primary-soft hover:border-color-color-primary-border",
      attributes: {
        n: "tag-select-trigger",
        type: "button",
        "aria-label": "管理标签",
        title: "管理标签",
      },
      onPointerDown(event) {
        event.preventDefault();
        event.stopPropagation();
      },
      onClick(event) {
        event.preventDefault();
        event.stopPropagation();
        popover_store.toggle();
      },
    },
    [Runtime.Icon({ name: "plus", size: 12 })],
  );

  const popover = Popover(
    {
      store: popover_store,
      side: "bottom",
      align: "start",
      triggerClass: "dm-tag-select-popover-trigger",
      onTriggerMouseEnter() {},
      onTriggerMouseLeave() {},
      class: "w-260px p-2 text-base",
      content: [
        View({ class: "mb-2" }, [
          Input({
            store: search_input,
            rootAttributes: { n: "tag-select-search" },
            attributes: {
              n: "tag-select-search-input",
              type: "search",
              autocomplete: "off",
              "aria-label": "搜索或创建标签",
            },
            onChange(event) {
              const value = String(
                (event && event.target && event.target.value) || "",
              );
              set_search_keyword(value);
            },
            onKeyDown(event) {
              if (event.key === "Escape") popover_store.hide();
            },
          }),
        ]),
        Show({
          when: Runtime.computed(filtered_, (list) => (list || []).length > 0),
          ok() {
            return View(
              {
                class: "flex flex-row flex-wrap items-center content-flex-start gap-[var(--dm-space-1,_4px)] max-h-220px overflow-y-auto",
                attributes: { n: "tag-select-list", role: "listbox" },
              },
              [
                Runtime.For({
                  each: filtered_,
                  render(entry) {
                    const tag = { id: entry.value, name: entry.name };
                    return View(
                      {
                        type: "button",
                        class: select_class_names([
                          "inline-flex items-center justify-center flex-[0_0_auto] w-auto max-w-full min-h-5 p-[0_var(--dm-space-2)] font-semibold leading-[1.2] text-color-text-secondary whitespace-nowrap text-center bg-color-bg-interactive border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-badge cursor-pointer transition-[color_0.15s_ease,_background_0.15s_ease,_border-color_0.15s_ease] hover:text-color-primary hover:bg-color-primary-soft hover:border-color-color-primary-border [&.is-selected]:text-color-primary [&.is-selected]:bg-color-primary-soft [&.is-selected]:border-color-color-primary-border",
                          Runtime.computed(selected_ids_, (ids) =>
                            (ids || []).indexOf(entry.value) >= 0
                              ? "is-selected"
                              : "",
                          ),
                        ]),
                        attributes: {
                          n: `tag-select-option-${entry.value}`,
                          type: "button",
                          role: "option",
                        },
                        onClick(event) {
                          event.preventDefault();
                          toggle(tag);
                        },
                      },
                      [String(entry.name || "")],
                    );
                  },
                }),
              ],
            );
          },
          else() {
            return View({ class: "p-[var(--dm-space-3)_var(--dm-space-2)] text-content-muted text-center" }, [
              "没有匹配的标签，回车可创建",
            ]);
          },
        }),
      ],
      onUnmounted() {
        tag_select_destroyed = true;
        if (keyword_timer) {
          window.clearTimeout(keyword_timer);
          keyword_timer = null;
        }
        if (typeof popover_unlisten === "function") popover_unlisten();
        if (typeof popover_hide_unlisten === "function")
          popover_hide_unlisten();
        popover_store.destroy?.();
      },
    },
    [trigger],
  );

  return View(
    {
      onClick(event) {
        event.stopPropagation();
      },
    },
    [popover],
  );
}

export { ContentTagBadge };

# Primitive 模式

源文件：`packages/headless/src/` 下各组件文件

## 什么是 Primitive

Primitive 是一组**可组合的工厂函数命名空间**（如 `SelectPrimitive.Root`, `SelectPrimitive.Trigger`），每个函数返回 `TimelessElement`。

**职责**：
- 接收 `store`（Core 实例），订阅 `store.onStateChange`
- 将 DOM 事件委托给 Core 方法（click → store.click()）
- 通过 `onMounted` 注册 DOM 元素到 Core
- **不持有任何样式** — class 由 shadcn 层通过 props 注入

## Primitive 列表

| Primitive | 文件 | 子元素 |
|-----------|------|--------|
| `ButtonPrimitive` | `button.ts` | Root, Loading, Prefix, Content |
| `SelectPrimitive` | `select.ts` | Root, Trigger, Value, Icon, Content, Viewport, Item, ItemIndicator, ItemText, Search |
| `CascaderPrimitive` | `cascader.ts` | Root, Trigger, Value, Icon, Content, Panel, Item, Search |
| `DialogPrimitive` | `dialog.ts` | Root, Overlay, Content, Header, Title, Description, Body, Footer, Close, Cancel, OK |
| `PopoverPrimitive` | `popover.ts` | Root, Trigger, Portal, Content, Arrow, Close |
| `DropdownMenuPrimitive` | `dropdown-menu.ts` | Trigger, Content, Item, SubMenuContent |
| `ContextMenuPrimitive` | `context-menu.ts` | Trigger, Content, Item, SubMenuContent |
| `MenuPrimitive` | `menu.ts` | Root, Trigger, Content, Item, Label, Separator, SubMenuContent |
| `TabsPrimitive` | `tabs.ts` | Root, List, Tab, Content, Indicator |
| `ToastPrimitive` | `toast.ts` | Root, Mask, Viewport, Item, Icon, Text |
| `PopperPrimitive` | `popper.ts` | Root, Content |
| `FieldPrimitive` | `field.ts` | Field.Label/Control/Error/Help, ObjectField.Fields, ArrayField.Items/Append/Remove |

其他：`input.ts`, `number-input.ts`, `textarea.ts`, `checkbox.ts`, `radio.ts`, `slider.ts`, `date-picker.ts`, `date-range-picker.ts`, `time-picker.ts`, `toggle.ts`, `accordion.ts`, `sheet.ts`, `tooltip.ts`, `popconfirm.ts`, `steps.ts`, `progress.ts`, `resizable-panels.ts`, `tag-select.ts`

## 典型组合模式

```js
// shadcn 层如何使用 Primitive
SelectPrimitive.Root({ store }, [
  SelectPrimitive.Trigger({ store, class: "styled-trigger" }, [
    SelectPrimitive.Value({ store, class: "..." }),
    SelectPrimitive.Icon({ class: "..." }, [ChevronDownIcon]),
  ]),
  SelectPrimitive.Content({
    store,
    class: "styled-dropdown",
    animation: { in: "animate-in fade-in", out: "animate-out fade-out" },
  }, [
    SelectPrimitive.Viewport({ store, class: "p-1" }, [
      For({ each: options, render(opt) {
        return SelectPrimitive.Item({ store, value: opt.value, class: "..." }, [
          SelectPrimitive.ItemIndicator({ store, value: opt.value }),
          SelectPrimitive.ItemText({}, [opt.label]),
        ]);
      }})
    ])
  ])
]);
```

## animation prop

浮层类 Primitive 的 Content 通常接受 `animation` prop：

```ts
animation: {
  in: "animate-in fade-in-0 zoom-in-95",    // 进入时添加的 CSS class
  out: "animate-out fade-out-0 zoom-out-95", // 退出时添加的 CSS class
}
```

需要查看具体 Primitive 的子元素和 props → 直接读取 `packages/headless/src/<name>.ts`。

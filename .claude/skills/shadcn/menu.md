# DropdownMenu / ContextMenu / Menu

源文件：`packages/shadcn/src/dropdown-menu.ts`, `context-menu.ts`, `menu.ts`, `menu-shared.ts`
Core：`packages/ui/src/dropdown-menu/index.ts`, `context-menu/index.ts`, `menu/index.ts`, `menu/item.ts`

## DropdownMenu

```js
const dm$ = new Timeless.ui.DropdownMenuCore({
  items: [
    new Timeless.ui.MenuItemCore({ label: "Edit", onClick() {} }),
    new Timeless.ui.MenuItemCore({ label: "Delete", onClick() {} }),
    new Timeless.ui.MenuItemCore({ label: "Disabled", disabled: true }),
  ],
  side: "bottom",
  align: "start",
});

DropdownMenu({ store: dm$ }, [
  Button({ store: new Timeless.ui.ButtonCore({}) }, ["Actions"]),  // trigger
]);
```

### DropdownMenuCore API

```ts
new DropdownMenuCore({
  items?: MenuItemCore[], side?, align?,
  trigger?: "click" | "hover" | "manual",
  offsetX?, offsetY?, submenuOffsetX?, submenuOffsetY?,
})

dm$.state → { items, open, disabled, enter, visible, exit }
dm$.show() / dm$.hide() / dm$.toggle()
dm$.setItems(items)
dm$.onStateChange(fn)
```

## ContextMenu

```js
const cm$ = new Timeless.ui.ContextMenuCore({
  items: [
    new Timeless.ui.MenuItemCore({ label: "Copy", onClick() {} }),
    new Timeless.ui.MenuItemCore({ label: "Paste", onClick() {} }),
  ],
});

ContextMenu({ store: cm$ }, [
  View({ class: "w-full h-40 border" }, [Txt("右键点击此区域")]),  // trigger 区域
]);
```

### ContextMenuCore API

```ts
new ContextMenuCore({
  items: MenuItemCore[],
  trigger?: "contextmenu" | "hover" | "manual",
  side?, align?, submenuOffsetX?, submenuOffsetY?,
})

cm$.state → { items }
cm$.show(position?) / cm$.hide()
cm$.setItems(items)
cm$.onStateChange(fn) / cm$.onShow(fn) / cm$.onHide(fn)
```

## MenuItemCore

```ts
new Timeless.ui.MenuItemCore({
  label: "Edit",
  icon?: unknown,
  shortcut?: string,
  disabled?: boolean,
  onClick?: () => void,
  menu?: new Timeless.ui.MenuCore({ items: [...] }),  // 子菜单
})

item$.state → { label, icon, shortcut, open, disabled, focused }
item$.click() / item$.disable() / item$.enable()
item$.hide() / item$.show()
```

## 子菜单

```js
new Timeless.ui.MenuItemCore({
  label: "More",
  menu: new Timeless.ui.MenuCore({
    items: [
      new Timeless.ui.MenuItemCore({ label: "Sub Item 1", onClick() {} }),
      new Timeless.ui.MenuItemCore({ label: "Sub Item 2", onClick() {} }),
    ],
  }),
});
```

子菜单自动从右侧弹出，内置 hover 延迟关闭（300ms）。

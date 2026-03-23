# Popover / Popconfirm / Tooltip

源文件：`packages/shadcn/src/popover.ts`, `popconfirm.ts`, `tooltip.ts`
Core：`packages/ui/src/popover/index.ts`（Popover/Popconfirm 共享）

## Popover

```js
const pop$ = new Timeless.ui.PopoverCore({
  side: "bottom",     // "top" | "bottom" | "left" | "right"
  align: "center",    // "start" | "center" | "end"
  strategy: "absolute",
});

Popover(
  { store: pop$, title: [Txt("标题")], content: [Txt("内容")] },
  [Button({ store: new Timeless.ui.ButtonCore({}) }, ["Open"])],  // trigger（children）
);
```

### PopoverCore API

```ts
new PopoverCore({ side?, align?, strategy?: "fixed" | "absolute", closeable? })

pop$.state → { isPlaced, closeable, x, y, visible, enter, exit }

pop$.show() / pop$.hide() / pop$.toggle()
pop$.onShow(fn) / pop$.onHide(fn) / pop$.onStateChange(fn)
```

## Popconfirm

与 Popover 类似，但内置确认/取消按钮：

```js
Popconfirm({
  store: new Timeless.ui.PopoverCore({}),
  title: "确认删除？",
  onOk() { /* delete */ },
  onCancel() {},
}, [Button({ store: triggerBtn$ }, ["Delete"])]);
```

## Tooltip

无 Core，纯展示组件。**必须包裹在 TooltipProvider 内**：

```js
TooltipProvider({}, [
  Tooltip({ content: ["提示文字"], side: "top" }, [
    Button({ store: btn$ }, ["Hover me"]),  // trigger
  ]),
]);
```

### Props

```ts
Tooltip({ content: ViewChildren, side?: "top" | "bottom" | "left" | "right" }, trigger)
```

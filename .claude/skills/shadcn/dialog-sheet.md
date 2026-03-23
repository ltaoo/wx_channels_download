# Dialog / Sheet

源文件：`packages/shadcn/src/dialog.ts`, `sheet.ts` | Core：`packages/ui/src/dialog/index.ts`

两者共享 `DialogCore`，Sheet 通过 `side` prop 控制滑出方向。

## Dialog

```js
const dialog$ = new Timeless.ui.DialogCore({
  title: "确认操作",
  footer: true,        // 显示 取消/确认 按钮
  closeable: true,     // 显示右上角关闭按钮
  mask: true,          // 遮罩层
});

Dialog({ store: dialog$ }, [
  View({ class: "space-y-2" }, [
    Txt("确认删除此项吗？"),
  ]),
]);

// 控制
dialog$.show();
dialog$.hide();

// 监听按钮
dialog$.onOk(() => { /* 确认 */ });
dialog$.onCancel(() => { /* 取消 */ });
```

## Sheet

```js
const sheet$ = new Timeless.ui.DialogCore({ title: "Settings" });

Sheet({ store: sheet$, side: "right" }, [   // "left" | "right" | "top" | "bottom"
  View({}, [Txt("Sheet content")]),
]);

sheet$.show();
```

## Core API

```ts
new DialogCore({
  _name?, title?: string, footer?: boolean,
  closeable?: boolean, mask?: boolean, open?: boolean,
  onCancel?, onOk?, onUnmounted?,
})

dialog$.state → { open, title, footer, closeable, mask, enter, visible, exit }

dialog$.show() / dialog$.hide() / dialog$.toggle()
dialog$.ok() / dialog$.cancel()
dialog$.setTitle(title)

dialog$.onShow(fn) / dialog$.onHidden(fn)
dialog$.onOk(fn) / dialog$.onCancel(fn)
dialog$.onStateChange(fn)
```

## 注意

- 动画：enter/exit 状态驱动 `animate-in/animate-out` CSS
- Dialog 渲染到 Portal（body），自带遮罩 + 居中定位
- Sheet 渲染到 Portal，从指定边缘滑入

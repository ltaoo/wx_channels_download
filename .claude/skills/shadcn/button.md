# Button

源文件：`packages/shadcn/src/button.ts` | Core：`packages/ui/src/button/index.ts`

## 用法

```js
const btn$ = new Timeless.ui.ButtonCore({
  variant: "default",   // "default" | "outline" | "secondary" | "destructive" | "ghost" | "link"
  size: "default",      // "default" | "sm" | "lg" | "icon"
  disabled: false,
  loading: false,
  onClick() { console.log("clicked"); },
});

Button({ store: btn$ }, ["Submit"]);
Button({ store: btn$, variant: "outline", size: "sm" }, ["Cancel"]);
```

## Core API

```ts
new ButtonCore({ _name?, disabled?, loading?, variant?, size?, onClick? })

// 状态
btn$.state → { text, loading, disabled, variant, size }

// 方法
btn$.click()
btn$.disable() / btn$.enable()
btn$.setLoading(true)
btn$.setVariant("outline")
btn$.setSize("sm")
btn$.bind(record)          // 绑定数据，onClick 回调可接收

// 事件
btn$.onClick(handler)
btn$.onStateChange(handler)
```

## 注意

- `variant` / `size` 可在 View 的 props 中直接传（覆盖 Core 的值）
- `Button` 内部有 `Loading` 子组件，loading 时自动显示旋转图标

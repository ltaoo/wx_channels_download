# Portal / Presence / Popper / Transition

源文件：`packages/headless/src/portal.ts`, `presence.ts`, `popper.ts`, `transition.ts`

## Portal

将子节点挂载到 `document.body`，而非父元素位置：

```js
Portal({}, [View({ class: "fixed inset-0" }, [overlay])]);
```

- `render()` 返回 `null`（不在原位插入 DOM）
- 子节点通过 `document.body.appendChild` 挂载
- `cleanup()` 从 body 移除并执行子节点生命周期

## Presence

动画感知的条件渲染，包装 `Show` + `PresenceCore`：

```js
Presence({ store: presenceCore }, [
  View({ class: "animated-content" }, [children]),
]);
```

- 内部 `when` 为 `computed(state, t => t.mounted || t.visible || t.exit)`
- 在 exit 动画阶段仍保持子节点挂载，直到动画结束才卸载
- 由 `PresenceCore` 的 `show()`/`hide()` 驱动

## PopperPrimitive

浮动定位桥接，连接 `PopperCore`（@floating-ui/dom）到 DOM：

```js
PopperPrimitive.Root({ store: popperCore }, [
  // reference 元素（trigger）通过 onMounted 注册
  PopperPrimitive.Content({ store: popperCore, class: "floating" }, [
    // 浮动内容，自动计算 transform: translate3d(x, y, 0)
  ]),
]);
```

`Content` 的职责：
- 注册 floating 元素到 PopperCore
- 通过 computed style 设置 `position` + `transform`
- 管理滚动监听和视口检查
- 注册到 LayerManager 实现点击外部关闭

## Transition

CSS 过渡包装，配合 `PresenceCore` 使用：

```js
const p$ = new Timeless.ui.PresenceCore({});

Transition({
  store: p$,
  animation: {
    in: "animate-in fade-in",
    out: "animate-out fade-out",
  },
}, [View({}, [content])]);

p$.show();  // 触发进入动画
p$.hide();  // 触发退出动画
```

## NativeInput

原生 `<input>` 元素包装，与 `InputCore` 配合：

```js
NativeInput({ store: inputCore, class: "..." });
```

源文件：`packages/headless/src/native-input.ts`

## 环境抽象

`packages/headless/src/env.ts` 提供跨端 DOM 创建：
- `safeCreateElement(tag)` — 浏览器用真实 DOM，服务端用 stub 对象
- `isBrowser` — 环境检测

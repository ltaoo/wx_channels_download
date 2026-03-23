# PresenceCore / PopperCore / DismissableLayerCore / LayerManager

源文件：`packages/ui/src/presence/index.ts`, `popper/index.ts`, `dismissable-layer/index.ts`, `layer/index.ts`

## PresenceCore — 动画生命周期

管理 enter/visible/exit 动画状态机：

```
show() → mounted=true, enter=true, visible=true → 150ms → enter=false
hide() → exit=true → 150ms → visible=false, mounted=false, exit=false
```

```ts
new PresenceCore({ mounted?, visible? })

state → { mounted, enter, visible, exit, text }

.show() / .hide(opts?) / .toggle()
.handleAnimationEnd()   // CSS 动画结束时调用（替代 150ms 定时器）
.unmount() / .reset()

.onShow(fn) / .onHidden(fn) / .onStateChange(fn) / .onPresentChange(fn)
```

被 Dialog, Popover, Select, Menu, Toast 等浮层组件内部组合使用。

## PopperCore — 浮动定位

基于 `@floating-ui/dom`，计算浮动元素位置：

```ts
new PopperCore({ side?, align?, strategy?: "fixed" | "absolute", offset? })

.setReference(ref)    // 注册参考元素（{ $el, getRect() }）
.setFloating(el)      // 注册浮动元素
.setArrow(el)         // 注册箭头元素

state → { x, y, strategy, placement, placedSide, placedAlign, isPlaced, arrow, middlewareData }

.onStateChange(fn)
```

中间件：`offset` → `flip` → `shift` → `arrow`。支持虚拟元素（通过 getRect 回调）。

## DismissableLayerCore — 点击外部关闭

```ts
new DismissableLayerCore()

.register()     // 注册到 LayerManager 栈
.unregister()   // 从栈移除

.onDismiss(fn) / .onPointerDownOutside(fn)
```

## LayerManager — 浮层栈管理

全局单例，维护浮层层级栈：

```ts
getGlobalLayerManager()

.push(layer)     // 入栈
.remove(id)      // 出栈
.handlePointerDown(x, y)  // 从栈顶遍历，关闭不包含点击点的层
.dismissAll() / .dismissTop()
.getTopLayer()
```

`initGlobalPointerListener()` 注册全局 pointerdown 监听。

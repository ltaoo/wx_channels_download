# VirtualListView 实现原理

`VirtualListView` 是一个用 vanilla JavaScript 实现的动态高度虚拟列表组件。它保留 Timeless 组件的调用方式，外层仍返回 `View(...)`，但列表内部的滚动容器、内容高度、可见行挂载、测量和卸载都由原生 DOM 管理。

## 目标

- 替换 `Timeless.ListView({ ... })`，减少对 Timeless 内置 ListView 的依赖。
- 支持 item 高度不固定的列表，只渲染当前滚动窗口附近的列表项。
- 保持原有列表回调和 DOM 结构约定，例如 `onMounted`、`onScroll`、`data-list-view-root`、`data-list-view-content`、`data-list-view-viewport`、`data-list-view-item`。
- 兼容下载任务列表的占位项和分页预加载逻辑。

## DOM 结构

组件挂载后会在根节点内创建两层容器：

```html
<div data-list-view-root>
  <div data-list-view-content>
    <div data-list-view-viewport>
      <div data-list-view-item>...</div>
    </div>
  </div>
</div>
```

根节点来自 Timeless 的 `View`，负责接收外部传入的 `style`、`class`、`attributes` 和生命周期。内部节点由原生 DOM 创建：

- `data-list-view-root`：真实滚动容器。
- `data-list-view-content`：撑开总滚动高度。
- `data-list-view-viewport`：绝对定位的可见项承载层。
- `data-list-view-item`：单个可见列表项。

这些属性沿用原 ListView 的结构，现有 CSS 和下载列表的高度同步逻辑可以继续命中。

## 动态高度核心思路

动态高度虚拟列表不能直接用 `index * itemHeight` 定位。当前实现采用社区主流方案：

- 先用 `itemHeight` 作为未测量 item 的预估高度。
- 可见 item 挂载后，用 `ResizeObserver` 和 `getBoundingClientRect()` 实测真实高度。
- 通过 `Map<key, height>` 缓存每个 item 的实测高度。
- 用前缀和数组 `offsets` 表示每个 index 的起始位置。
- 滚动时用二分查找在 `offsets` 中定位当前可见起点。
- 高度变化时重建 offsets，并重新定位当前已挂载行。

这和 `react-window` 的 `VariableSizeList`、`TanStack Virtual` 等常见实现方向一致：估算未测量项，测量已挂载项，用累计尺寸查找可见范围。

## 前缀和 offsets

`offsets[index]` 表示第 `index` 个 item 的顶部位置：

```js
offsets[0] = 0
offsets[index + 1] = offsets[index] + measuredOrEstimatedHeight(index) + gutter
```

其中 `measuredOrEstimatedHeight(index)` 的规则是：

- 如果当前 item 的 key 已有实测高度，使用实测高度。
- 否则使用 `itemHeight` 预估高度。

总滚动高度默认是 `offsets[items.length]`。如果配置了 `paddingBottom`，最终高度会变为：

```js
contentHeight = offsets[items.length] + paddingBottom
```

`data-list-view-content` 的 `height/min-height` 会设置为这个值，用来撑开真实滚动条。`paddingBottom` 只影响滚动空间，不参与 item offset 计算，因此最后一项仍按真实累计高度定位，底部额外留白由 content 高度提供。

## 可见区间计算

滚动时不会按固定高度除法计算 index，而是用二分查找：

- 在 `offsets` 中找到 `scrollTop` 对应的第一个可见 index。
- 在 `offsets` 中找到 `scrollTop + clientHeight` 对应的最后一个可见 index。
- 在两端额外扩展 `buffer` 个 item，减少快速滚动时的空白。

`start` 和 `end` 会被限制在 `[0, items.length]` 范围内。组件只保留这个区间内的 DOM 行，区间外的行会被卸载。

## 总高度撑开

虚拟列表不能把所有行都放进 DOM，但滚动条必须表现得像完整列表存在一样。因此组件会给 `data-list-view-content` 设置总高度为当前 offsets 的末尾值。

下载列表旧的 `syncListViewSlotHeights` 会按业务估算高度写 spacer。动态高度版本会在根节点标记 `data-virtual-list-view="dynamic"`，业务侧检测到后不再写 spacer，避免覆盖组件根据真实测量得到的总高度。

## 行渲染和定位

每个可见行都是绝对定位，并用 transform 放到对应的累计 offset 上：

```js
row.style.position = "absolute";
row.style.transform = `translateY(${offsets[index]}px)`;
```

行内容来自外部传入的 `render(item, indexRef)`。如果返回 Timeless element，组件用 `Timeless.DOM.buildAndRender` 构建真实 DOM 并插入当前行；如果返回 ref，则创建文本节点并订阅 ref 变化；如果返回普通值，则渲染为文本。

## 高度测量和滚动锚定

item 挂载后会注册单行 `ResizeObserver`。行高度变化时：

1. 读取 `row.getBoundingClientRect().height`。
2. 写入 `measuredHeights` 缓存。
3. 标记 offsets 失效。
4. 更新 content 总高度。
5. 重新定位当前已挂载行。
6. 重新计算可见区间。

如果变化的 item 在当前可见区之前，组件会把 `root.scrollTop` 按高度差同步调整。这样上方 item 变高或变矮时，用户正在看的内容不会明显跳动，这就是常见的 scroll anchoring 处理。

## 响应式数据更新

`each` 支持 Timeless ref/refarr。组件会订阅 `each.subscribe`：

- `onPatch`：重新计算可见区间并强制重建当前可见行。
- `onChange`：同样强制重建当前可见行。

强制重建会先卸载现有可见行，再按最新 `items` 重新渲染，保证任务状态、占位项、排序或删除后的 DOM 与数据一致。高度缓存按 `key` 保存，数据刷新后相同 key 的 item 可以复用之前测量值。

## 生命周期处理

组件本身通过 Timeless `View` 接入生命周期：

- `onMounted`：创建原生 DOM 结构、绑定 scroll、订阅数据、执行首次渲染，并调用外部 `onMounted`。
- `beforeUnmounted`：转发外部回调。
- `onUnmounted`：销毁虚拟列表实例、移除事件监听、取消订阅、卸载当前行，并调用外部 `onUnmounted`。

单行被移出可见区时，会调用其 Timeless element 的 `beforeUnmounted` 和 `onUnmounted`，并清理 ref 订阅、测量任务和行级 `ResizeObserver`。

## 滚动回调

根节点监听原生 `scroll`，触发两类行为：

- 调用外部 `onScroll({ target, scrollTop, clientHeight, scrollHeight })`。
- 重新计算可见区间，增删可见行。

下载列表的 `handleListViewScroll` 依赖这个回调维护 `_scrollTop` 并触发分页预加载。

## 分页占位项兼容

下载列表会为未加载页生成 placeholder task。`VirtualListView` 只渲染可见区间，因此滚动到某个占位项时，业务侧 `render(task)` 仍会执行：

```js
if (vm$.methods.isPlaceholderTask(task)) {
  vm$.methods.ensureTaskPageForIndex(task.__index);
  return DownloadTaskSkeletonCard(...);
}
```

这保证用户滚动到未加载区域时，分页请求仍会被触发。

## Resize 处理

组件优先使用 `ResizeObserver`：

- 根容器 resize：重新计算可见区间。
- 行元素 resize：更新该 item 的实测高度。

如果运行环境不支持 `ResizeObserver`，根容器 resize 退化为监听 `window.resize`；行高度会在挂载后通过下一帧测量一次。

## 与原调用接口的对应关系

当前下载列表使用的参数都被保留：

- `style`、`class`、`attributes`：传给外层 `View`。
- `key`：写入 `data-list-view-key`，便于调试。
- `size`：最少可见行数。
- `buffer`：上下缓冲行数。
- `gutter`：行间距。
- `itemHeight`：未测量 item 的预估高度，不要求真实 item 固定高。
- `paddingBottom`：列表底部额外滚动留白，支持数字或 `px` 字符串。
- `each`：列表数据源。
- `onMounted`：根滚动容器挂载后回调。
- `onScroll`：滚动回调。
- `onItemResize`：可选，单个 item 实测高度变化时回调。
- `render`：单项渲染函数。

## 局限

- 未测量的远端 item 仍依赖 `itemHeight` 估算，因此首次跳到很远位置时，滚动条会随着实测逐步校正。
- 数据更新时采用当前可见区强制重建，逻辑简单可靠，但没有做 keyed DOM 复用优化。
- 组件依赖 `Timeless.DOM.buildAndRender` 来挂载 Timeless element，因此它是 vanilla DOM 管理加 Timeless element 渲染的混合实现。

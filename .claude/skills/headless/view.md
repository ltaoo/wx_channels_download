# View / TimelessElement / 生命周期

源文件：`packages/headless/src/view.ts`

## TimelessElement — 通用返回接口

所有组件（View, Show, For, Txt, Fragment, Portal...）均返回此接口：

```ts
interface TimelessElement {
  t: string;        // 类型标签："view" | "show" | "for" | "portal" | "text" | "fragment" | "match"
  $elm: HTMLElement | SVGElement | Text | DocumentFragment;
  render(): HTMLElement | ... | null;   // 创建并返回 DOM 元素
  cleanup?: () => void;
  onMounted?(el): void;                // 挂载后回调
  beforeUnmounted?(): void;
  onUnmounted?(): void;                // 卸载时回调，必须清理订阅
}
```

**不使用虚拟 DOM**，直接创建和操作真实 DOM 节点。

## View(props, children)

万物基石。创建一个 DOM 元素：

```ts
View({
  as: "div",            // 或 type: "div"（HTML 标签名）
  id: "my-id",          // 支持 string | Ref<string>
  class: "cls",         // 支持 string | Ref<string> | ClassNameRef
  style: "color:red",   // 支持 string | Ref<string> | StyleRef
  dataset: { key: "v" }, // data-* 属性

  // 事件
  onClick(event) {},
  onDblclick(event) {},
  onLongpress(event) {},
  onFocus(event) {},
  onBlur(event) {},
  onKeyDown(event) {},
  onContextmenu(event) {},
  onMouseEnter(event) {},
  onMouseLeave(event) {},
  onDragStart/onDragOver/onDrop/onDragEnd(event) {},

  // 生命周期
  onMounted($elm) {},       // 返回值作为 cleanup 函数
  beforeUnmounted() {},
  onUnmounted() {},         // 必须在此清理事件订阅
}, children)
```

## ViewChildren 类型

```ts
type ViewChildren = (
  | TimelessElement
  | (() => TimelessElement)     // 惰性元素（由 h() 创建）
  | string | number
  | Ref<string | number>       // 自动包装为 Txt()
  | null
)[];
```

## 辅助函数

| 函数 | 文件 | 用途 |
|------|------|------|
| `Txt(stringOrRef)` | `text.ts` | 响应式文本节点 |
| `Fragment(props, children)` | `fragment.ts` | 无包裹 DOM 分组（DocumentFragment） |
| `h(Component, props, children)` | `h.ts` | 惰性组件包装，延迟到 render 时创建 |
| `render(elm, $root)` | `render.ts` | 入口：挂载到 DOM 根元素 |

## 响应式属性绑定

View 的 `class`, `style`, `id` 等属性若传入 `Ref`，会自动订阅并响应式更新 DOM：

```js
const cls = computed(state, (s) => s.active ? "bg-blue-500" : "bg-gray-100");
View({ class: cls }, [Txt("auto update class")]);
```

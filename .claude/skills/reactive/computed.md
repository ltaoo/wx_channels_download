# computed / derive — 派生值

源文件：`packages/reactive/src/computed.ts`, `derive.ts`

## computed — 单依赖派生

```js
const count = ref(0);
const double = computed(count, (v) => v * 2);
double.value   // 0
count.as(5);
double.value   // 10
```

### 接受多种依赖类型

```js
// Ref
computed(myRef, (v) => ...)

// 普通对象（自动包装为 refobj）
const state = { name: "A", age: 20 };
computed(state, (s) => s.name + " is " + s.age);

// refobj
const state_ = refobj(store.state);
computed(state_, (s) => s.open ? "visible" : "hidden");
```

### 用于 View 的 class/style

```js
View({
  class: computed(state_, (s) => s.active ? "bg-blue-500 text-white" : "bg-gray-100"),
  style: computed(state_, (s) => `opacity: ${s.visible ? 1 : 0}`),
}, children);
```

### computed 返回 Ref（只读）

```ts
computed(dep, fn) → Ref<R>   // 有 .value, ._subscribe, 无 .as
```

### 销毁

```js
release(computedRef);   // 或 uncomputed(computedRef)，从全局 registry 移除
```

## derive — 多依赖派生

```js
const a = ref(1);
const b = ref(2);

// 数组形式（参数展开）
const sum = derive([a, b], (x, y) => x + y);

// 对象形式（命名参数）
const info = derive({ a, b }, ({ a, b }) => `${a} + ${b}`);
```

### 支持混合 Ref 和非 Ref

```js
derive([myRef, 100], (v, constant) => v + constant);
```

### derive 返回 Ref（只读）

```ts
derive(deps, fn) → Ref<R>
```

## 别名

| 正式名 | 别名 |
|-------|------|
| `derive` | `combine` |
| `release` | `uncomputed` |

## 全局 Registry

`computed` 内部使用全局 `Map` 避免对同一源对象重复包装。不再需要时调用 `release()` 清理。

```js
import { registryGet, registrySet, release } from "@timeless/reactive";
```

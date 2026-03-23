# ref — 基本响应式值

源文件：`packages/reactive/src/ref.ts`

## 创建

```js
const count = ref(0);
const name = ref("hello");
const flag = ref(true);
const obj = ref({ a: 1 });   // 注意：对象推荐用 refobj()
```

## 读取

```js
count.value  // 0
```

## 更新（触发订阅者通知）

```js
count.as(5);                    // 直接赋值
count.as(cur => cur + 1);       // 函数式更新
```

## 比较

```js
count.eq(5)              // ===
count.isSame(5)          // Object.is
count.isStrictEqual(5)   // ===
```

## 订阅

```js
count._subscribe({
  onChange(newValue) { console.log(newValue); },
  onPatch(action) { /* { type: "insert"|"update"|"delete"|"refresh", ... } */ },
});
```

## 销毁

```js
count._destroy();   // 清除所有订阅者
```

## Ref<T> 类型

```ts
type Ref<T> = {
  __is_ref: true;
  value: T;
  _subscribe(ctx: Subscriber): void;
  _destroy(): void;
  isSame(v: T): boolean;
  isStrictEqual(v: T): boolean;
};

type Subscriber = {
  onChange: (v: any) => void;
  onPatch?: (c: any) => void;
};
```

## 类型守卫

```js
import { isRef } from "@timeless/reactive";
isRef(count)  // true
```

## 注意

- `ref` 用于基本值（number, string, boolean）和简单引用
- 对象推荐 `refobj()`，数组推荐 `refarr()`——它们提供更丰富的操作方法
- View 的 class/style/id 属性若传入 Ref，会自动响应式更新 DOM
- children 中的 `Ref<string>` 自动包装为 `Txt()` 响应式文本

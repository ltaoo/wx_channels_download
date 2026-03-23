# refobj / refObject — 响应式对象

源文件：`packages/reactive/src/reactive-object.ts`

## 创建

```js
const state = refobj({ name: "Alice", age: 20 });
const nullable = refobj(null);   // RefObjectNullable<T>
```

## 读取

```js
state.value          // { name: "Alice", age: 20 }
state.value.name     // "Alice"
```

## 更新

```js
// 替换整个对象
state.as({ name: "Bob", age: 25 });

// 函数式更新（Object.assign 到原对象）
state.as(cur => ({ ...cur, age: cur.age + 1 }));

// 设置单个属性
state.set("name", "Charlie");
state.set("age", cur => cur + 1);   // 支持函数式

// 合并部分属性
state.assign({ age: 30 });

// 删除属性
state.delete("age");

// 手动触发通知（值已被外部修改时）
state.refresh();
```

## 订阅

```js
state._subscribe({
  onChange(newObj) { console.log(newObj); },
});
```

## 典型用法：包装 Core state

```js
const state_ = refobj(store.state);
store.onStateChange((v) => {
  state_.as(v);    // 触发所有依赖更新
});

// 用 computed 派生 UI 属性
const cls = computed(state_, (s) => s.active ? "bg-blue-500" : "bg-gray-100");
```

## RefObject<T> 接口

```ts
interface RefObject<T> extends Ref<T> {
  set(key: keyof T, item: any): void;
  get(key: keyof T): any;        // 对象类型的值自动包装为嵌套 refobj
  delete(key: keyof T): void;
  as(nextObj: T | ((cur: T) => T)): void;
  assign(updated: Partial<T>): void;
  refresh(): void;
}
```

# refarr / refArray — 响应式数组

源文件：`packages/reactive/src/reactive-array.ts`

## 创建

```js
const list = refarr([1, 2, 3]);
const items = refarr([], { key: "id" });   // 指定 diff key
```

## 读取

```js
list.value     // [1, 2, 3]
list.length    // 3
list.get(0)    // 1（对象元素自动包装为 refobj）
```

## 增删改（触发增量 patch 通知）

```js
list.push(4);                    // 尾部添加 → onPatch({ type: "insert" })
list.unshift(0);                 // 头部添加
list.insert(1, "a", "b");       // 指定位置插入
list.pop();                      // 尾部删除 → onPatch({ type: "delete" })
list.shift();                    // 头部删除
list.delete(2);                  // 按索引删除
list.remove(item);               // 按引用删除
list.set(0, "new");              // 按索引替换 → onPatch({ type: "update" })
list.splice(1, 2, "a", "b");    // 通用操作 → onChange（全量刷新）
```

## 整体替换（触发全量 onChange）

```js
list.as([4, 5, 6]);                     // 替换
list.as(cur => cur.filter(x => x > 2)); // 函数式
list.refresh();                          // 手动通知
list.sort((a, b) => a - b);             // 排序
list.reverse();                          // 反转
```

## 只读查询（不触发通知）

```js
list.filter(x => x > 2)
list.map(x => x * 2)
list.find(x => x === 3)
list.findIndex(x => x === 3)
list.includes(3)
list.indexOf(3)
list.every(x => x > 0)
list.some(x => x > 5)
list.reduce((acc, x) => acc + x, 0)
list.slice(1, 3)
list.concat([7, 8])
list.join(", ")
list.flat()
list.flatMap(x => [x, x])
for (const item of list) { ... }  // 支持 for...of
```

## 订阅

```js
list._subscribe({
  onChange(newArray) { /* 全量更新 */ },
  onPatch(action) {
    // action.type: "insert" | "update" | "delete"
    // action.index, action.items, action.deleteCount
  },
});
```

## 配合 For 控制流

```js
For({
  each: list,   // 直接传 RefArray
  render(item, index) { return View({}, [Txt(item.name)]); },
});
```

`For` 订阅 `onPatch`，实现增量 DOM 更新（insert/delete/update），不必全量重渲染。

## RefArray<T> 接口

完整接口实现了 Array 的所有标准方法。关键额外方法：

```ts
interface RefArray<T> extends Ref<T[]> {
  key: any;                           // 用于 For diff
  insert(idx, ...items): number;      // 指定位置插入
  delete(idx): void;                  // 按索引删除
  remove(item): void;                 // 按引用删除
  as(items | fn): void;               // 整体替换
  refresh(): void;                    // 手动通知
}
```

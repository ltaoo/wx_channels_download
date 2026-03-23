# MultipleSelectionCore — 多选管理

源文件：`packages/kit/src/multiple/index.ts`

## 用法

```js
const selection$ = new MultipleSelectionCore({
  defaultValue: [],
  options: [
    { label: "Apple", value: "apple" },
    { label: "Banana", value: "banana" },
  ],
  onChange(values) { console.log(values); },
});

selection$.toggle("apple");   // 切换选中
selection$.select("banana");  // 选中
selection$.remove("apple");   // 取消选中
selection$.isEmpty();         // false
selection$.clear();           // 清空

selection$.state → { value: ["banana"], options: [...] }
selection$.onChange(fn) / selection$.onStateChange(fn)
```

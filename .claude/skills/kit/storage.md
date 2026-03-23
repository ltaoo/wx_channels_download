# StorageCore — 键值存储

源文件：`packages/kit/src/storage/index.ts`

## 用法

```js
const storage$ = new StorageCore({
  key: "app",
  values: {},
  defaultValues: { theme: "light", lang: "zh" },
  client: { setItem: localStorage.setItem.bind(localStorage), getItem: localStorage.getItem.bind(localStorage) },
});

storage$.get("theme");                    // "light"
storage$.get("unknown", "fallback");     // "fallback"
storage$.set("theme", "dark");           // 去抖 100ms 后写入
storage$.merge("settings", { fontSize: 14 }); // 合并对象/数组
storage$.clear("theme");                 // 清除单个 key
storage$.remove("theme");               // 移除 key

storage$.onStateChange(fn);
```

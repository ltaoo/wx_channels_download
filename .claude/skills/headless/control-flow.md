# Show / For / Switch+Match

源文件：`packages/headless/src/show.ts`, `for.ts`, `switch.ts`

## Show — 条件渲染

```js
Show({ when: boolRef, fallback: [Txt("Loading...")] }, [
  Txt("Loaded!"),
]);
```

- `when`：`Ref<boolean> | boolean`
- `fallback`：可选，when 为 false 时显示
- 内部用 text node 锚点，切换时正确执行 onMounted/onUnmounted 生命周期
- 返回 `TimelessElement { t: "show" }`

## For — 列表渲染

```js
For({
  each: reactiveArray,    // T[] | Ref<T[]>
  render(item, index) {
    return View({ class: "item" }, [Txt(item.name)]);
  },
  key: "id",              // 可选，指定 diff 用的唯一键
});
```

- 支持 Ref 数组的增量 patch（insert/delete/update），不必全量重渲染
- 内部做 keyed reconciliation：O(1) lookup + 最小 DOM 操作
- 返回 `TimelessElement { t: "for" }`

## Switch + Match — 多分支

```js
Switch({ when: computedRef }, [
  Match("value1", [Txt("Branch 1")]),
  Match("value2", [Txt("Branch 2")]),
]);
```

- `when`：`Ref<any>`，值变化时切换到匹配的 Match 分支
- `Match(value, children)` 创建 `{ t: "match", value }` 特殊元素
- 切换时正确执行 unmount/mount 生命周期

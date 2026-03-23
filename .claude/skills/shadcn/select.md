# Select

源文件：`packages/shadcn/src/select.ts` | Core：`packages/ui/src/select/index.ts`
示例：`packages/shadcn/src/select.example.ts`

## 用法

```js
const select$ = new Timeless.ui.SelectCore({
  defaultValue: null,
  placeholder: "请选择",
  options: [
    { value: "apple", label: "Apple" },
    { value: "banana", label: "Banana" },
  ],
  onChange(v) { console.log("Selected:", v); },
});

Select({ store: select$ });
Select({ store: select$, id: "fruit-select" });  // 配合 Label
```

## Core API

```ts
new SelectCore({
  defaultValue: T | null,
  placeholder?: string,
  options?: { value: T, label: string }[],
  onChange?: (v: T | null) => void,
  search?: boolean,              // 启用搜索过滤
  searchPlaceholder?: string,
})

// 状态
select$.state → { options, value, value2, open, placeholder, disabled, required, search, searchKeyword, enter, visible, exit }

// 方法
select$.select(value)
select$.show() / select$.hide()
select$.setOptions(options)
select$.setValue(v)
select$.clear()
select$.focus() / select$.blur()
select$.setSearchKeyword(keyword)

// 事件
select$.onChange(fn)
select$.onStateChange(fn)
select$.onFocus(fn) / select$.onBlur(fn)
```

## 注意

- `options` 中每项的 state 会带 `selected` 和 `focused` 布尔值
- 浮层宽度自动匹配 trigger 宽度（通过 `store.reference.width`）
- 内置键盘导航：ArrowUp/Down 聚焦，Enter 选中，Escape 关闭

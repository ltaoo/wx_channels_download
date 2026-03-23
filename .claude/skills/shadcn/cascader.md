# Cascader

源文件：`packages/shadcn/src/cascader.ts` | Core：`packages/ui/src/cascader/index.ts`

## 用法

```js
const cascader$ = new Timeless.ui.CascaderCore({
  placeholder: "请选择",
  options: [
    { value: "zj", label: "浙江", children: [
      { value: "hz", label: "杭州", children: [
        { value: "xh", label: "西湖" },
      ]},
    ]},
    { value: "js", label: "江苏", children: [...] },
  ],
  onChange(valuePath, selectedOptions) { console.log(valuePath); },
  showFullPath: true,        // 显示完整路径 "浙江 / 杭州 / 西湖"
  expandTrigger: "click",    // "click" | "hover"
});

Cascader({ store: cascader$ });
```

## Core API

```ts
new CascaderCore({
  defaultValue?: T[],
  placeholder?, options?, onChange?,
  search?: boolean, searchPlaceholder?: string,
  expandTrigger?: "click" | "hover",
  showFullPath?: boolean, pathSeparator?: string,
})

cascader$.state → { options, value, panels, open, placeholder, disabled, displayText, search, searchKeyword, searchResults }

cascader$.expand(panelIndex, value)
cascader$.select(valuePath)           // e.g. ["zj", "hz", "xh"]
cascader$.clickOption(panelIndex, option)
cascader$.setOptions(options)
cascader$.setValue(value)
cascader$.clear()
cascader$.show() / cascader$.hide()
```

## 注意

- `panels` 是动态多列面板，每展开一级增加一列
- 搜索模式会扁平化所有路径进行模糊匹配

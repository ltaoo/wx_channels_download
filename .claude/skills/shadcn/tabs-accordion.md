# Tabs / Accordion / Steps

源文件：`packages/shadcn/src/tabs.ts`, `accordion.ts`, `steps.ts`
Core：`packages/ui/src/tab-header/index.ts`, `accordion/index.ts`

## Tabs

```js
const tabs$ = new Timeless.ui.TabHeaderCore({
  selected: "tab1",
  options: [
    { id: "tab1", text: "General" },
    { id: "tab2", text: "Advanced" },
  ],
  onChange(v) { console.log("switched to", v); },
});

// 方式1：items 包含 content
Tabs({
  store: tabs$,
  items: [
    { value: "tab1", label: "General", content: [Txt("General settings")] },
    { value: "tab2", label: "Advanced", content: [Txt("Advanced settings")] },
  ],
});

// 方式2：只用 tab header，children 自定义内容
Tabs({ store: tabs$ }, [
  Show({ when: computed(tabs$.state, t => t.curId === "tab1") }, [Txt("Custom content")]),
]);
```

### TabHeaderCore API

```ts
new TabHeaderCore({ selected?, options: { id, text, hidden? }[], onChange? })

tabs$.state → { tabs, current, curId, left }
tabs$.selectById(id)
tabs$.select(index)
tabs$.setTabs(options)
tabs$.onStateChange(fn) / tabs$.onChange(fn)
```

## Accordion

```js
const acc$ = new Timeless.ui.AccordionCore({
  type: "single",     // "single" | "multiple"
  defaultOpenItems: [0],
});

Accordion({
  store: acc$,
  items: [
    { title: "What is it?", content: "A component library." },
    { title: "How to use?", content: "Install and import." },
  ],
});
```

### AccordionCore API（工厂函数，非 class）

```ts
AccordionCore({ type?: "single" | "multiple", defaultOpenItems?: number[] })

acc$.state → { openItems, type }
acc$.toggle(index)
acc$.open(index) / acc$.close(index)
acc$.isOpen(index) → boolean
acc$.onStateChange(fn)
```

## Steps

无 Core，纯展示组件：

```js
Steps({
  current: ref(1),    // 当前步骤（支持 ref）
  items: [
    { title: "Step 1", description: "First step" },
    { title: "Step 2", description: "Second step" },
  ],
});
```

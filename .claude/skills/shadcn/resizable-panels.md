# ResizablePanels

源文件：`packages/shadcn/src/resizable-panels.ts`
详细文档：`packages/shadcn/src/resizable-panels.README.md`（中文）

## 用法

```js
ResizablePanels({ direction: "horizontal", class: "min-h-[200px] rounded-lg border" }, [
  ResizablePanel({ defaultSize: 50 }, [
    View({ class: "flex h-full items-center justify-center p-6" }, [Txt("Panel A")]),
  ]),
  ResizableHandle({ withHandle: true }),
  ResizablePanel({ defaultSize: 50 }, [
    View({ class: "flex h-full items-center justify-center p-6" }, [Txt("Panel B")]),
  ]),
]);
```

## Props

```ts
ResizablePanels({ direction: "horizontal" | "vertical" }, children)
ResizablePanel({ defaultSize: number, minSize?, maxSize?, collapsible?, collapsedSize?, onResize? }, children)
ResizableHandle({ withHandle?: boolean })  // withHandle 显示拖动把手图标
```

## 高级用法

```js
// 三栏布局
ResizablePanels({ direction: "horizontal" }, [
  ResizablePanel({ defaultSize: 25, minSize: 15 }, [sidebar]),
  ResizableHandle({}),
  ResizablePanel({ defaultSize: 50 }, [main]),
  ResizableHandle({}),
  ResizablePanel({ defaultSize: 25, collapsible: true, collapsedSize: 0 }, [aside]),
]);

// 垂直嵌套
ResizablePanels({ direction: "horizontal" }, [
  ResizablePanel({ defaultSize: 50 }, [left]),
  ResizableHandle({}),
  ResizablePanel({ defaultSize: 50 }, [
    ResizablePanels({ direction: "vertical" }, [
      ResizablePanel({ defaultSize: 50 }, [top]),
      ResizableHandle({}),
      ResizablePanel({ defaultSize: 50 }, [bottom]),
    ]),
  ]),
]);
```

更多细节请读取 `packages/shadcn/src/resizable-panels.README.md`。

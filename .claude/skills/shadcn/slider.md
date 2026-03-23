# Slider

源文件：`packages/shadcn/src/slider.ts`

无独立 Core，直接通过 props 控制。

## 用法

```js
const value = ref(50);

Slider({
  value,
  min: 0,
  max: 100,
  step: 1,
  onChange(v) { value.as(v); },
});
```

## Props

```ts
Slider({
  value: Ref<number> | number,
  min?: number,
  max?: number,
  step?: number,
  disabled?: boolean,
  onChange?: (value: number) => void,
})
```

详细实现见源文件 `packages/shadcn/src/slider.ts`。

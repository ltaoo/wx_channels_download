# Input / Textarea / NumberInput

源文件：`packages/shadcn/src/input.ts`, `textarea.ts`, `number-input.ts`
Core：`packages/ui/src/form/input/index.ts`, `form/number-input/index.ts`

## Input

```js
const input$ = new Timeless.ui.InputCore({
  defaultValue: "",
  placeholder: "请输入...",
  disabled: false,
  type: "text",          // "text" | "password" | "number" | ...
  onChange(v) { console.log(v); },
  onEnter(v) { console.log("enter:", v); },
});

Input({ store: input$ });
Input({ store: input$, id: "my-input" });  // 配合 Label htmlFor
```

### InputCore API

```ts
new InputCore({ defaultValue, placeholder?, disabled?, type?, autoFocus?, onChange?, onEnter?, onBlur?, onClear? })

input$.state → { value, placeholder, disabled, loading, focus, type, allowClear, autoFocus }

input$.setValue(v)
input$.clear()
input$.focus()
input$.setPlaceholder(v)
input$.setLoading(bool)
input$.showText() / input$.hideText()  // password 切换

input$.onChange(fn) / input$.onEnter(fn) / input$.onFocus(fn) / input$.onBlur(fn)
input$.onStateChange(fn)
```

## Textarea

复用 `InputCore`，只是渲染为 `<textarea>`：

```js
Textarea({ store: new Timeless.ui.InputCore({ defaultValue: "", placeholder: "..." }) });
```

## NumberInput

```js
const num$ = new Timeless.ui.NumberInputCore({
  defaultValue: 0,
  min: 0, max: 100,
  step: 5,
  precision: 2,
  onChange(v) { console.log(v); },
});

NumberInput({ store: num$, showControls: true });
```

### NumberInputCore API

```ts
new NumberInputCore({ defaultValue?, min?, max?, step?, precision?, formatter?, parser?, onChange? })

num$.state → { value, displayValue, placeholder, disabled, step, precision, min, max, canIncrease, canDecrease }

num$.increase() / num$.decrease()
num$.setValue(v)
num$.setMin(n) / num$.setMax(n) / num$.setStep(n)
num$.clear() / num$.reset()
```

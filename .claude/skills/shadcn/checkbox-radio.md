# Checkbox / CheckboxGroup / RadioGroup / Toggle

源文件：`packages/shadcn/src/checkbox.ts`, `checkbox-group.ts`, `radio.ts`, `toggle.ts`
Core：`packages/ui/src/checkbox/index.ts`, `checkbox/group.ts`, `radio/index.ts`, `toggle/index.ts`

## Checkbox

```js
const cb$ = new Timeless.ui.CheckboxCore({
  checked: false,
  disabled: false,
  onChange(checked) { console.log(checked); },
});

Checkbox({ store: cb$ });
```

### API

```ts
cb$.state → { label, checked, value, disabled }
cb$.toggle() / cb$.check() / cb$.uncheck() / cb$.reset()
cb$.onChange(fn) / cb$.onStateChange(fn)
```

## CheckboxGroup

```js
const cbg$ = new Timeless.ui.CheckboxGroupCore({
  options: [
    { value: "a", label: "Option A" },
    { value: "b", label: "Option B", checked: true },
  ],
  onChange(values) { console.log(values); },  // ["b"]
});

CheckboxGroup({ store: cbg$, direction: "horizontal" });  // "horizontal" | "vertical"
```

### API

```ts
cbg$.state → { values, options: { label, value, core: CheckboxCore }[], disabled, indeterminate }
cbg$.checkOption(value) / cbg$.uncheckOption(value) / cbg$.reset()
cbg$.setOptions(options)
```

## RadioGroup

```js
const rg$ = new Timeless.ui.RadioGroupCore({
  value: "a",
  options: [
    { value: "a", label: "Option A" },
    { value: "b", label: "Option B" },
  ],
  onChange(v) { console.log(v); },
});

RadioGroup({ store: rg$, direction: "horizontal" });
```

### API

```ts
rg$.state → { value, options: { label, value, core: RadioCore }[], disabled }
rg$.select(value) / rg$.setValue(value) / rg$.reset()
rg$.setOptions(options)
```

## Toggle

```js
const toggle$ = new Timeless.ui.ToggleCore({ defaultValue: false });
Toggle({ store: toggle$ });
```

### API

```ts
toggle$.state → { checked: boolean }
toggle$.toggle()
toggle$.onStateChange(fn)
```

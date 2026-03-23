# Field / Form（表单验证）

源文件：`packages/shadcn/src/form.ts` | Core：`packages/ui/src/formv2/field.ts`
类型：`packages/ui/src/formv2/types.ts`
测试：`packages/ui/src/formv2/__tests__/index.test.ts`

## 单字段

```js
const email$ = new Timeless.ui.SingleFieldCore({
  label: "Email",
  name: "email",
  input: new Timeless.ui.InputCore({ defaultValue: "", placeholder: "email@example.com" }),
  rules: [
    { required: true },
    { mode: "email" },
  ],
});

Field({ store: email$ }, [Input({ store: email$.input })]);
```

## 表单组合

```js
const name$ = new Timeless.ui.SingleFieldCore({
  label: "Name", name: "name",
  input: new Timeless.ui.InputCore({ defaultValue: "" }),
  rules: [{ required: true }, { minLength: 2, maxLength: 20 }],
});

const form$ = new Timeless.ui.ObjectFieldCore({
  fields: { email: email$, name: name$ },
});

// 验证
const r = await form$.validate();
if (r.error) { /* 验证失败 */ }
const values = r.data;  // { email: "...", name: "..." }

// 设值 / 清空
form$.setValue({ email: "a@b.com", name: "Test" });
form$.clear();
```

## 验证规则

```ts
type FieldRuleCore = Partial<{
  required: boolean;
  min: number;               // 数值最小值
  max: number;               // 数值最大值
  minLength: number;         // 字符串最小长度
  maxLength: number;         // 字符串最大长度
  mode: "email" | "number";  // 内置格式校验
  custom(value): Result<null>;  // 自定义校验，返回 Result.Ok(null) 或 Result.Err("msg")
}>;
```

## SingleFieldCore API

```ts
new SingleFieldCore({ label?, name?, input: InputCore | SelectCore | ..., rules?: FieldRuleCore[], hidden? })

field$.state → { label, name, hidden, focus, error, status, input: { shape, value, type } }
field$.validate() → Result
field$.setValue(v)
field$.clear()
field$.show() / field$.hide()
field$.onChange(fn) / field$.onError(fn) / field$.onStateChange(fn)
```

## ObjectFieldCore API

```ts
new ObjectFieldCore({ label?, name?, fields: Record<string, SingleFieldCore | ObjectFieldCore | ArrayFieldCore>, hidden? })

form$.validate() → Result
form$.setValue(values)
form$.clear()
form$.toJSON()
form$.showField(name) / form$.hideField(name)
form$.setFieldValue(key, value)
form$.onChange(fn) / form$.onError(fn) / form$.onStateChange(fn)
```

## ArrayFieldCore API

```ts
new ArrayFieldCore({ label?, name?, fields: FieldCore[], hidden? })

arr$.append() / arr$.remove(id)
arr$.insertBefore(id) / arr$.insertAfter(id)
arr$.validate() → Result
arr$.setValue(values)
```

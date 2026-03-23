# 表单系统（Form v2）

源文件：`packages/ui/src/formv2/field.ts`
类型：`packages/ui/src/formv2/types.ts`
测试：`packages/ui/src/formv2/__tests__/index.test.ts`

## SingleFieldCore

```ts
new SingleFieldCore({
  label?: string, name?: string,
  input: InputCore | SelectCore | NumberInputCore | ...,  // 实现 FormInputInterface
  rules?: FieldRuleCore[],
  hidden?: boolean,
})

state → { label, name, hidden, focus, error, status, input: { shape, value, type } }

.validate() → Result
.setValue(value)
.clear()
.show() / .hide()
.onChange(fn) / .onError(fn) / .onStateChange(fn)
```

## ObjectFieldCore

```ts
new ObjectFieldCore({
  label?, name?,
  fields: Record<string, SingleFieldCore | ObjectFieldCore | ArrayFieldCore>,
  hidden?,
})

.validate() → Result        // 递归验证所有子字段
.setValue(values)            // { fieldName: value, ... }
.clear()
.toJSON()
.showField(name) / .hideField(name)
.setFieldValue(key, value)
.onChange(fn) / .onError(fn) / .onStateChange(fn)
```

## ArrayFieldCore

```ts
new ArrayFieldCore({ label?, name?, fields: FieldCore[], hidden? })

.append() / .remove(id)
.insertBefore(id) / .insertAfter(id)
.upIdx(id) / .downIdx(id)       // 上移/下移
.validate() → Result
.setValue(values)
```

## 验证规则

```ts
type FieldRuleCore = Partial<{
  required: boolean;
  min: number;                // 数值最小值
  max: number;                // 数值最大值
  minLength: number;          // 字符串最小长度
  maxLength: number;          // 字符串最大长度
  mode: "email" | "number";   // 内置格式校验
  custom(value): Result<null>; // 自定义，返回 Result.Ok(null) 或 Result.Err("msg")
}>;
```

## FormInputInterface

Core 输入组件需实现此接口才能用于 SingleFieldCore：

```ts
interface FormInputInterface<T> {
  shape: string;           // "string" | "number" | "boolean" | "select" | ...
  value: T;
  defaultValue: T;
  setValue(v: T): void;
  onChange(fn): () => void;
  destroy?(): void;
}
```

`InputCore`, `NumberInputCore`, `SelectCore`, `CheckboxCore` 等均已实现。

## 注意

- Form v2 的 Core 类**不继承 BaseDomain**，使用内部 `base()` 事件总线
- Form v1（`packages/ui/src/form/`）是旧版，无内置规则验证，不推荐新代码使用

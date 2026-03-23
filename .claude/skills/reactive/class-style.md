# classNames / styleNames — 响应式 CSS

源文件：`packages/reactive/src/class-names.ts`, `style-names.ts`

## classNames（别名 cn）

将多个静态/动态 class 源合并为一个响应式 `ClassNameRef`：

```js
const cls = classNames([
  "base-class",                          // 静态字符串
  computed(state_, s => s.active ? "bg-blue-500" : ""),   // Ref<string>
  otherClassNameRef,                     // 嵌套 ClassNameRef
]);

View({ class: cls }, children);
```

### 命令式操作

```js
cls.add("new-class");      // 添加（去重）
cls.del("old-class");      // 删除
cls.append("a b c");       // 空格分隔批量添加
cls.toString();             // "base-class bg-blue-500 new-class"
```

### ClassNameRef 接口

```ts
type ClassNameRef = {
  __cn_ref: true;
  _subscribe(ctx: Subscriber): void;
  add(v: string): void;
  del(v: string): void;
  append(c: string): void;
  toString(): string;
};
```

- 自动去重 class 名
- 任何响应式源变化时自动重算并通知

## styleNames（别名 sn）

将多个 CSS style 源合并为一个响应式 `StyleRef`：

```js
const style = styleNames([
  "color: red; font-size: 14px",                        // 静态
  computed(state_, s => `opacity: ${s.visible ? 1 : 0}`), // 动态
]);

View({ style }, children);
```

### StyleRef 接口

```ts
type StyleRef = {
  __style_ref: true;
  _subscribe(ctx: Subscriber): void;
  toString(): string;
};
```

- 解析 CSS 声明并用 Map 去重（后者覆盖前者同名属性）
- `.toString()` 输出合并后的 inline style 字符串

## 类型守卫

```js
isClassName(v)   // v is ClassNameRef
isStyleRef(v)    // v is StyleRef
isRef(v)         // v is Ref<any>
```

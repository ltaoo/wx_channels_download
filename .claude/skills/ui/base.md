# BaseDomain / base() / Result / BizError

源文件：`packages/base/src/`

## BaseDomain — 类式 Core 基类

所有 class 风格的 Core 继承此类：

```ts
class XxxCore extends BaseDomain<TheTypesOfEvents> {
  _open = false;

  get state() { return { open: this._open }; }

  show() {
    this._open = true;
    this.emit(Events.StateChange, { ...this.state });
  }

  onStateChange(handler) { return this.on(Events.StateChange, handler); }
}
```

**核心方法**：
- `on(event, handler)` → 返回 `unlisten` 函数
- `emit(event, value?)` → 触发事件
- `destroy()` → 移除所有监听，发射 Destroy 事件
- `offEvent(key)` → 移除指定事件的所有监听
- `uid()` → 全局自增 ID
- `log(...args)` → 条件调试日志（`this.debug = true` 时生效）

底层使用 `mitt` 事件发射器。

## base() — 函数式替代

返回与 BaseDomain 同接口的普通对象，用于工厂模式 Core：

```ts
function XxxCore(props) {
  const bus = base<{ StateChange: State }>();
  let _open = false;
  return {
    get state() { return { open: _open }; },
    show() { _open = true; bus.emit("StateChange", ...); },
    onStateChange(fn) { return bus.on("StateChange", fn); },
  };
}
```

AccordionCore, DatePickerCore, DateRangePickerCore, TimePickerCore 使用此模式。

## Result

```ts
Result.Ok(value)   → { data: value, error: null }
Result.Err("msg")  → { data: null, error: BizError }
```

用于验证和异步操作的统一返回类型。

## BizError

```ts
class BizError extends Error {
  messages: string[];
  code?: string;
  data?: any;
}
```

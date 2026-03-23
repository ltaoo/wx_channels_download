# RequestCore — API 请求

源文件：`packages/kit/src/request/index.ts`
工具：`packages/kit/src/request/utils.ts`（request_factory, RequestPayload）
轮询：`packages/kit/src/request/loop.ts`（RequestLoop）

## 用法

```js
// 1. 定义 service 函数（返回 RequestPayload）
function fetchUser(id) {
  return request.get("/api/user", { id });
}

// 2. 创建 RequestCore
const req$ = new RequestCore(fetchUser, {
  client: httpClient,     // HttpClientCore 实例
  onSuccess(data) { console.log(data); },
  onFailed(err) { console.error(err); },
});

// 3. 执行
await req$.run(userId);
req$.reload();            // 使用上次参数重新执行
req$.cancel();
```

## Core API

```ts
new RequestCore(service, {
  client: HttpClientCore,
  delay?: number,
  defaultResponse?: T,
  process?: (v: any) => T,
  onSuccess?, onFailed?, onCompleted?, onCanceled?,
})

req$.loading / req$.initial / req$.pending / req$.response / req$.error
req$.run(...args)        // 执行请求
req$.reload()            // 重新执行
req$.cancel()            // 取消
req$.clear()             // 清除响应
req$.modifyResponse(fn)

// 事件
req$.onSuccess(fn) / req$.onFailed(fn) / req$.onCompleted(fn)
req$.onLoadingChange(fn) / req$.onStateChange(fn)
```

## request_factory

```js
const api = request_factory({
  hostnames: { dev: "http://localhost:3000", prod: "https://api.example.com" },
  headers: { "Content-Type": "application/json" },
});

api.setEnv("prod");
const payload = api.get("/user", { id: 1 });  // → RequestPayload
const payload2 = api.post("/user", { name: "Alice" });
```

## RequestLoop

```js
const loop = RequestLoop({ request: req$, interval: 3000 });
loop.start(userId);    // 持续轮询
loop.setFinish();      // 停止
```

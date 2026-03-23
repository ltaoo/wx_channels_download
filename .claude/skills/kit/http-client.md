# HttpClientCore — HTTP 客户端抽象

源文件：`packages/kit/src/http_client/index.ts`

## 用法

```js
const http$ = new HttpClientCore({
  hostname: "https://api.example.com",
  headers: { "Authorization": "Bearer token" },
});

const result = await http$.get("/users", { page: 1 });
const result2 = await http$.post("/users", { name: "Alice" });
http$.cancel(requestId);
```

## Core API

```ts
new HttpClientCore({ hostname?, headers?, debug? })

http$.get<T>(endpoint, query?, extra?)  → Promise<{ data: T }>
http$.post<T>(endpoint, body?, extra?)  → Promise<{ data: T }>
http$.fetch<T>(options)                 // 底层方法（需平台实现覆盖）
http$.cancel(id)

http$.setHeaders(headers)
http$.appendHeaders(headers)
http$.setDebug(debug)
http$.onStateChange(fn)
```

## 注意

- `fetch` 方法是 stub，需要在平台层（web/native）实现具体的网络调用
- 与 `RequestCore` 配合使用：`new RequestCore(service, { client: http$ })`

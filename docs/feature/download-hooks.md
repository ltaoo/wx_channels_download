---
title: 下载任务 Hook
---

# 下载任务 Hook

在工作目录的 `hooks.js` 中可以声明下载任务生命周期 Hook：

```js
function onTaskSuccess(ctx) {
  // 仅在任务成功时调用。
}

function onTaskFailed(ctx) {
  // 仅在任务失败时调用，可通过 ctx.error 获取失败原因。
}

function onTaskFinish(ctx) {
  // 任务成功或失败时都会调用。
}
```

同一任务的调用顺序如下：

- 成功：`onTaskSuccess(ctx)`、`onTaskFinish(ctx)`。
- 失败：`onTaskFailed(ctx)`、`onTaskFinish(ctx)`。

这些 Hook 异步执行，不阻塞任务进入终态。某个 Hook 执行失败时会记录警告日志，但不会改变任务状态；`onTaskSuccess` 或 `onTaskFailed` 执行失败也不会阻止 `onTaskFinish` 执行。

支持使用 `async/await` 和全局 `fetch` 将完整任务上下文提交到 HTTP 接口：

```js
async function onTaskSuccess(ctx) {
  const response = await fetch("http://localhost:8080/webhook/download_task/success", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(ctx),
  });
  if (!response.ok) {
    throw new Error(`Webhook 提交失败：HTTP ${response.status} ${await response.text()}`);
  }
}
```

Fetch API 使用社区的 [whatwg-fetch 3.6.20](https://github.com/JakeChampion/fetch/tree/v3.6.20)，事件循环使用 [goja_nodejs](https://github.com/dop251/goja_nodejs)，兼容 Go 1.20，无需安装 Node.js 或 npm 依赖。

- `fetch(url, options)` 返回 Promise；请求参数支持 `method`、`headers` 和 `body`，JSON 请求体需要自行 `JSON.stringify`。
- 提供 `Headers`、`Request`、`Response`，支持 `response.ok/status/statusText/url/headers`、`await response.text()` 和 `await response.json()`。
- 网络错误会拒绝 Promise；HTTP 4xx/5xx 正常返回 Response，需要自行检查 `response.ok`。
- Hook 返回 Promise 时，会等待其结束再执行后续 Hook；异步异常也会记录失败日志。请 `await` 或返回异步操作的 Promise；单次 Hook 的异步等待上限为 10 秒，超时会取消未完成请求。

当前 HTTP 适配面向文本和 JSON：请求体支持字符串、ArrayBuffer，响应以文本完整读取。重定向遵循 Go HTTP Client 的默认行为；未提供可配置的 `redirect/cache`、Blob、FormData、流式响应及 AbortController，并非完整 Node.js 运行时。

`ctx` 的结构为：

```ts
type FinishContext = {
  task: {
    id: number;
    name: string;
    download_dir?: string;
    config: Record<string, unknown>;
  };
  config: Record<string, unknown>;
  metadata: Record<string, unknown>;
  resources: Array<{
    id: number;
    name: string;
    kind: string;
    size: number;
    unique_id: string;
    extra: Record<string, string>;
    endpoints: Array<{
      protocol: string;
      url: string;
    }>;
  }>;
  filePaths: string[];
  downloadDir: string;
  status: "success" | "failed";
  error?: string;
};
```

失败任务的 `filePaths` 只包含失败前已完成的资源；没有已完成资源时为空数组。

文件命名 Hook `onFilename` 的用法参见[指定下载的文件名](./filename.md)。

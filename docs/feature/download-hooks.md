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

---
title: 下载任务 Hook
---

# 下载任务 Hook

在工作目录的 `hooks.js` 中可以声明下载任务生命周期 Hook。生命周期 Hook 只有显式命名导出的函数才会注册，未导出的同名函数不会执行；文件命名 Hook `onFilename` 兼容旧的未导出写法：

```js
export function onTaskSuccess(task) {
  // 仅在任务成功时调用。
}

export function onTaskFailed(task) {
  // 仅在任务失败时调用，可通过 task.error 获取失败原因。
}

export function onTaskFinish(task) {
  // 任务成功或失败时都会调用。
}
```

也支持 `export const onTaskSuccess = async (task) => { ... }` 和 `export { handler as onTaskSuccess }`。生命周期 Hook 不读取默认导出，导出的非函数值也会被忽略。`onTaskCreate` 同样必须显式导出；已有生命周期 Hook 需要补上 `export`。`onFilename` 优先使用同名命名导出，没有该导出时读取旧的 `function onFilename(...)`，无需修改旧的文件命名脚本。

同一任务的调用顺序如下：

- 成功：`onTaskSuccess(task)`、`onTaskFinish(task)`。
- 失败：`onTaskFailed(task)`、`onTaskFinish(task)`。

这些 Hook 异步执行，不阻塞任务进入终态。某个 Hook 执行失败时会记录警告日志，但不会改变任务状态；`onTaskSuccess` 或 `onTaskFailed` 执行失败也不会阻止 `onTaskFinish` 执行。

## 脚本日志

脚本内置 `console.log/info/warn/error/debug`，也可 `import console from "node:console"`。日志接入下载器的应用日志，可在界面的「日志」页面搜索脚本中自行设置的消息或前缀，也会写入启动时显示的 `app.log`。

```js
console.info("[上传] 开始上传", { task_id: task.id, file_path: file.file_path, size: file.size });
console.log("任务 %d，共 %d 个文件", task.id, task.files.length);
console.warn("跳过无文件任务");
console.error("上传失败", error);
```

`log/info` 对应 info 级别，其余方法对应同名级别。运行时只传递消息和级别，不自动添加任务 ID、Hook 名、脚本路径、来源或方法名；需要这些内容时，在 `console` 参数中显式传入。对象可直接打印，循环引用显示 `[Circular]`；显式打印 Error 时会输出其堆栈，异步 Hook 抛出的异常也保留脚本行号。支持多参数及 `%s/%d/%i/%f/%j/%o/%O/%%` 格式化占位符。

日志遵循现有的 `debug.enabled` 开关；关闭时应用日志不会写入。开启后重启生效，现有启动策略会清空旧的 `app.log`，需要保留时先备份。`ahooks.js` 已记录配置读取、文件打开、上传开始、HTTP 响应、上传结果、清单保存和 Webhook 调用；不会打印配置全文或上传凭证。

支持使用 `async/await` 和全局 `fetch` 将完整任务上下文提交到 HTTP 接口：

```js
export async function onTaskSuccess(task) {
  const response = await fetch("http://localhost:8080/webhook/download_task/success", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(task),
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
- Hook 返回 Promise 时，会等待其结束再执行后续 Hook；异步异常也会记录失败日志。推荐 `await` 或返回异步操作的 Promise，以便在脚本中处理错误；`fs.readFile` 回调及其触发的请求也会等待完成。单次 Hook 的异步等待上限为 10 分钟，超时会取消未完成请求。同一个脚本的 Hook 串行执行，较长的上传会让后续 Hook 等待。

请求体支持字符串、Buffer、Uint8Array、ArrayBuffer、Blob、FormData 和 `fs.createReadStream()`。重定向遵循 Go HTTP Client 的默认行为，307/308 可以重新发送 Blob 文件和表单；ReadStream 不能重放，遇到需要重放的重定向会拒绝请求。响应完整读取后提供给脚本。未提供可配置的 `redirect/cache`、流式响应及 AbortController，并非完整 Node.js 运行时。

## 读取和上传文件

脚本支持 `import fs from "fs"`，也支持命名导入、命名空间导入和 `node:fs` 别名。已实现的读取接口按 [Node.js fs](https://nodejs.org/docs/latest-v20.x/api/fs.html) 的调用方式提供：

- `fs.readFileSync(path[, options])`：返回 Buffer；指定 `"utf8"` 或 `{ encoding: "utf8" }` 时返回字符串，读取错误直接抛出。
- `fs.readFile(path[, options], callback)`：异步调用 `callback(err, data)`，成功时 `err` 为 `null`。方法本身返回 `undefined`。
- `await fs.promises.readFile(path[, options])`：返回 Promise，读取失败时拒绝；也可从 `fs/promises` 或 `node:fs/promises` 导入 `readFile`。
- `await fs.openAsBlob(path, { type })`：返回由文件支持的 Blob，上传时流式读取，不会将整个视频载入内存。上传前会检查文件是否已被修改。
- `fs.existsSync(path)`：检查路径是否存在。
- `fs.createReadStream(path[, options])`：创建二进制文件上传流，可直接作为 Fetch 请求体，使用方法见下文。

`options` 支持编码字符串，或包含 `encoding`、`flag` 的对象。默认编码为 `null`，返回 Buffer；编码支持 `utf8`、`utf-8`、`hex`、`base64`、`base64url`、`ascii`、`latin1`/`binary`、`utf16le`/`ucs2` 及其连字符别名，大小写不敏感。`flag` 默认是 `"r"`，也支持 Node 的字符串打开标志和操作系统数字标志。

路径支持字符串、Buffer 和 `file:` URL。相对路径按下载器进程的当前工作目录解析，绝对路径直接使用，符号链接按操作系统规则跟随；不依赖任务下载目录，脚本加载阶段也可以读取文件。错误提供 `code`，文件系统错误还提供 `path` 和 `syscall`，例如文件不存在时的 `"ENOENT"`。

同时提供全局 `Buffer`、`URL` 和 `URLSearchParams`，也可从 `buffer`/`node:buffer`、`url`/`node:url` 导入。`Blob` 支持 `size`、`type`、`text()` 和 `arrayBuffer()`；`FormData` 支持 `append(name, value, filename?)`。当前实现上述文件读取与上传能力；尚未实现数字文件描述符、AbortSignal、完整文件系统，以及第三方 npm 模块。

需要在脚本中签署七牛上传凭证时，可从 `crypto` 或 `node:crypto` 导入 `createHmac`。支持 `sha1`、`sha256`、`sha512`，调用方式为 `createHmac(algorithm, key).update(data).digest(encoding)`，不指定输出编码时返回 Buffer。

### 原始文件流上传

服务接受 PUT/POST 二进制文件时，调用方式与 Node.js Fetch 相同：

```js
import fs from "fs";

export async function onTaskSuccess(task) {
  for (const file of task.files) {
    const response = await fetch("https://your-service.example/upload", {
      method: "PUT",
      headers: { "Content-Type": file.kind || "application/octet-stream" },
      body: fs.createReadStream(file.file_path),
      duplex: "half",
    });
    if (!response.ok) throw new Error(`上传失败：HTTP ${response.status}`);
  }
}
```

`duplex: "half"` 是 Node.js Fetch 对流式请求体的要求。文件按接收速度读取，不会预先载入整个文件；超时、请求失败或调用 `stream.destroy()` 时会关闭文件。每个流只能上传一次，重试时需要重新创建。

支持 `start`、`end`（包含结束位置）、`highWaterMark`（默认 64 KiB）和 `flags`（默认 `"r"`）选项，以及 `bytesRead`、`destroyed`、`readableEnded`、`path`、`readableHighWaterMark` 和 `destroy()`。当前 ReadStream 用于普通文件的二进制上传，尚未提供 `pipe`、事件订阅、异步迭代、文本编码、`fd`、自定义 `fs` 和 `autoClose: false`。

### 流式表单上传

在成功回调中上传最终产物，并附带完整任务详情：

```js
import fs from "fs";

export async function onTaskSuccess(task) {
  for (const file of task.files) {
    const form = new FormData();
    form.append("file", await fs.openAsBlob(file.file_path, {
      type: file.kind || "application/octet-stream",
    }), file.name);
    form.append("task", JSON.stringify(task));

    const response = await fetch("https://your-service.example/upload", {
      method: "POST",
      body: form,
    });
    if (!response.ok) {
      throw new Error(`上传失败：HTTP ${response.status} ${await response.text()}`);
    }
  }
}
```

将地址和表单字段名替换成接收服务要求的值。FormData 会自动生成带 boundary 的 `Content-Type`，不要手动设置这个请求头；鉴权头可以照常添加。

服务要求 PUT 二进制文件时，可以直接将 `await fs.openAsBlob(file.file_path, { type: file.kind })` 作为 `fetch` 的 `body`，无需 FormData。小文件也可以用 `fs.readFileSync(file.file_path)` 作为请求体，它会将整个文件读入内存。

表单里的文件同样流式读取，只有字段和 multipart 头保存在内存中。与 Node.js 原生 FormData 一样，文件参数应传 Blob，不能直接传 `createReadStream()`；`fs.openAsBlob` 是 Node.js 提供的文件 Blob API。

### 七牛云测试

七牛原生上传接口使用 `token`、`key`、`file` 三个表单字段，`file` 使用上面的 `fs.openAsBlob`。华南 `z2` 的测试入口为 `https://upload-z2.qiniup.com`；其他区域应按七牛的[区域与域名表](https://developer.qiniu.com/kodo/1671/region-endpoint-fq)选择，不要直接拼接不同协议的 region 名称。[上传凭证](https://developer.qiniu.com/kodo/1208/upload-token)应按七牛要求生成。

提供真实服务测试，默认跳过；手动运行时读取仓库外的私有 JSON 配置，上传一个唯一命名的 PNG，核对七牛 ETag 并从配置域名下载比对内容：

```json
{
  "access_key": "YOUR_ACCESS_KEY",
  "secret_key": "YOUR_SECRET_KEY",
  "bucket": "YOUR_BUCKET",
  "domain": "https://YOUR_DOMAIN"
}
```

```sh
QINIU_HOOK_TEST_CONFIG=/absolute/path/private-qiniu-test.json \
  go test -run '^TestHookQiniuStreamUpload$' -count=1 -v ./pkg/hermes
```

测试会保留上传的对象，并在配置文件旁写入不含密钥的 `result.json`，记录对象 key、URL 和验证结果。

## 任务详情结构

三个终态 Hook 的参数都是完整任务详情，与 `GET /api/v1/download_task/detail` 的 `data`、前端 SDK `task.onSuccess(detail)`、Go SDK 的 `DownloadTaskDetail` 和 `WaitDownloadTask` 返回值使用同一结构。HTTP 接口仍保留 `{ code, msg, data }` 包装，回调参数和 Go SDK 返回值不含此包装。

成功只在后处理、最终重命名、输出信息及完成状态持久化成功后触发。`files` 是最终产物列表，合并前的分片不会保留；文件路径、MIME 类型和大小也使用最终结果。

```ts
type DownloadTaskDetail = {
  id: number;
  name: string;
  platform_id: string;
  status: 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7;
  error: string;
  content_id: string | null;
  content_type: string;
  content: ContentDetail | null; // 内容及 accounts，沿用详情接口结构
  parent_task_id: number | null;
  root_task_id: number;
  relation_type: string;
  child_count: number;
  source_url: string;
  url: string;
  cover_url: string;
  cover_width: string;
  cover_height: string;
  config_json: string;
  metadata_json: string;
  size: number;
  downloaded: number;
  speed: number;
  progress: number;
  files: DownloadTaskFile[];
  file_count: number;
  created_at: number;
  updated_at: number;
};

type DownloadTaskFile = {
  id: number;
  name: string;
  download_dir: string;
  file_path: string;
  local_path: string; // 与 file_path 相同，保留兼容现有调用方
  kind: string;
  type: "FILE" | "STREAM" | "COLLECTION";
  status: "waiting" | "downloading" | "paused" | "finished" | "error" | "cancelled";
  error: string;
  size: number;
  downloaded: number;
  speed: number;
  progress: number;
  url: string;
  file_type: string;
  file_url: string;
  playback_url?: string;
  playback_available: boolean;
  exists: boolean;
};
```

`status` 为 `5` 时成功、`6` 时失败；`error` 无错误时为空字符串。失败任务也返回相同结构，各文件的状态和 `exists` 反映实际结果。无文件时 `files` 为 `[]`，没有关联内容时 `content` 为 `null`。

旧版终态 Hook 的 `ctx.task.id`、`ctx.resources`、`ctx.filePaths` 和字符串状态不再提供，分别改用 `task.id`、`task.files`、`task.files.map(file => file.file_path)` 和数字状态。`config_json`、`metadata_json` 保持 JSON 字符串，需要时用 `JSON.parse` 读取。

Go SDK 的 `DownloadTaskDetail` 和 `WaitDownloadTask` 均返回 `*dm.DownloadTask`，其中 `Files` 为 `[]dm.DownloadTaskFile`，不再需要对详情或文件列表单独反序列化；错误信息统一读取 `Error`。

文件命名 Hook `onFilename` 的用法参见[指定下载的文件名](./filename.md)。

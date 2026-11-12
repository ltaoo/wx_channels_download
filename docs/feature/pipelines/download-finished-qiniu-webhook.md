# 下载完成后上传七牛云并通知 Webhook

该流程复现原 `workdir/ahooks.js` 的任务完成处理顺序：收到 `downloadtask.finished` → 上传全部最终文件 → 发送 Webhook。任一文件上传失败时，不发送 Webhook；没有文件时跳过上传，仍发送任务详情。

## 安装与切换

1. 将仓库 `examples/nodes/qiniu_upload.js` 复制到应用工作目录的 `nodes/qiniu_upload.js`。当前开发工作目录已放置该文件。
2. 准备原 Hook 使用的七牛配置值。新版节点不再读取 `qiniu-upload.json`，改由自动化的“配置”表单收集。
3. 从原 Hook 文件移除或取消导出 `onTaskSuccess`，其他 Hook 可以继续保留。避免同时启用旧处理和新流程，否则每次完成任务会上传、通知两次。
4. 重启应用，确认创建节点时可以选择“上传到七牛云”。
5. 在「自动化 → 自动化流程 → 导入 JSON」粘贴下方流程图。JSON 已声明 `downloadtask.finished` 默认事件触发器，导入只创建共享流程；新增配置实例后自动继承事件规则。
6. 打开实例的“配置”，填写 `access_key`、`secret_key`、`bucket`、`domain`；`upload_url` 默认是 `https://upload-z2.qiniup.com`，可以覆盖。配置未齐时执行会被校验拒绝。Webhook 的 URL、method、headers、token 和 body 模板也在此配置，默认仍以 POST 将原任务详情发送到 `http://localhost:8080/webhook/download_task/success`。
7. 实例“配置”中的触发器字段默认继承流程配置，事件响应和 600 秒超时由流程提供。和节点配置一样，取消某个字段的“继承流程配置”后可单独覆盖该项，其他字段继续同步默认值；例如只暂停当前实例的事件，不影响超时的继承。已有独立规则会保留，可点击“恢复全部默认值”并保存后使用流程规则。

切换前完成的下载不会被自动补发。示例文件也保存在仓库 `examples/pipelines/download_finished_qiniu_webhook.json` 与当前开发工作目录的同名文件中。

## 配置和结果

七牛节点是通过 JS 注册的 `QiniuUploadNode`，并非 Go 内置节点。节点 `arguments.task` 使用 `input.event.data.task`，来自与下载 Hook 相同的 `BuildTaskDetail` 投影，包含最终文件路径和内容详情；旧事件字段 `TaskID` 仍然保留。

节点通过注册的 `config_schema` 声明所需配置，在 `execute(arguments, config)` 的第二个参数读取。节点依次上传文件，使用与原脚本相同的上传凭证、唯一 key 和响应校验，每个成功文件都会更新 `qiniu-upload-<任务ID>.json` 清单。示例将事件的 `download_dir` 传入 `arguments.manifest_dir`，保证清单与原 Hook 一样保存在下载任务的根目录。

上传结果供下游读取 `output.upload.uploaded`，每项包含 `file_path`、`key`、`hash`、`url`。Webhook 默认正文仍是原任务详情，不附加上传结果，与原 Hook 保持一致；可用 body 模板调整。非 2xx 的上传或 Webhook 响应使运行失败，可在自动化运行记录中查看。

Webhook 节点支持以下实例配置，也可在流程中设置默认值供实例继承：

| 字段 | 默认值 | 用法 |
| --- | --- | --- |
| `url` | `http://localhost:8080/webhook/download_task/success` | 完整 HTTP(S) 地址 |
| `method` | `POST` | GET、POST、PUT、PATCH、DELETE、HEAD、OPTIONS；GET/HEAD 不发送正文 |
| `headers` | `{"Content-Type":"application/json"}` | JSON 对象，值支持作用域模板，例如 `{"X-Task-ID":"{{input.event.data.task.id}}"}` |
| `token` | 空 | 密钥输入，非空时设置 `Authorization: Bearer <token>` 并覆盖 headers 中的同名字段；留空时保留 headers 中的认证设置 |
| `body_template` | `"{{input.event.data.task}}"` | 多行 JSON 模板；先解析 JSON，再替换字符串内的作用域占位符。整个字符串为占位符时保留原值类型 |

默认 body 模板包含两端的双引号。若要同时通知任务与上传结果，可填写：

```json
{
  "task": "{{input.event.data.task}}",
  "uploaded": "{{output.upload.uploaded}}",
  "message": "下载完成：{{input.event.data.task.name}}"
}
```

`task` 会替换为对象，`uploaded` 会替换为数组，正文中的引号、换行等由 JSON 编码处理。无效 JSON 或不存在的作用域引用会使通知失败。已导入的旧流程需更新 Webhook 节点声明或重新导入此示例，才会出现这些配置项。

流程默认事件规则的整体超时为 600 秒，上传和通知共享这十分钟，与原任务完成 Hook 的运行上限一致。

示例没有启用自动重试。部分成功的上传不会被回滚；手动从头重试会生成新 key 并再次上传。应用内的下载完成事件会像原 Hook 一样串行处理，前一个流程执行期间到达的任务会等待，不因流程忙而直接丢弃。等待中的事件尚未持久化，进程退出期间不保证补发；已启动运行仍按自动化原有恢复规则处理。

## 可导入流程

<<< ./download_finished_qiniu_webhook.json

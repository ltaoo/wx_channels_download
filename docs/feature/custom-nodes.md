# 从 JS 文件注册自动化节点

在工作目录的 `nodes/` 下创建 `.js` 文件，重启下载器后即可在自动化编辑器的节点列表选择它。文件只在启动时加载；修改文件后需要重启。

脚本与下载 Hook 一样属于可信的本机代码，可以读取本机文件和访问网络。只安装自己信任的文件；节点的密钥通过自动化的“配置”表单填写，独立保存，不写入可导出的流程 JSON。

## 创建节点

例如在 `workdir/nodes/example.js` 中注册一个节点：

```js
registerNode({
  type: "ExampleNode",
  name: "生成消息",
  description: "根据参数生成一条消息",
  config_keys: [
    { key: "arguments", type: "object", required: true, description: "name: 消息中的名字" },
    { key: "output_key", type: "string", description: "结果名称" }
  ],
  config_schema: [
    { key: "prefix", label: "问候语", type: "string", required: true, default: "你好" }
  ],
  default_config: {
    arguments: { name: "下载器" },
    output_key: "message"
  },
  async execute(arguments_value, config) {
    if (typeof arguments_value.name !== "string") throw new Error("name 必须是字符串");
    return { text: `${config.prefix}，${arguments_value.name}` };
  }
});
```

一个文件可以多次调用 `registerNode()`，注册不同类型。注册应在文件顶层同步完成；不要在顶层上传文件、发送请求或创建定时器，顶层代码在加载和每次执行时都会运行。

| 字段 | 用途 |
| --- | --- |
| `type` | 唯一类型，使用以字母开头、由字母/数字/下划线组成且以 `Node` 结尾的名称；不能覆盖内置节点 |
| `name` | 编辑器中显示的节点名称，必填 |
| `description` | 节点说明 |
| `config_keys` | 编辑器显示的配置提示；节点应在 `execute` 中校验自己的业务参数 |
| `default_config` | 创建节点时预填的 JSON 配置 |
| `config_schema` | 节点所需配置声明，支持 required、secret、default；在自动化“配置”中填写 |
| `execute(arguments_value, config)` | 执行函数，支持 async；返回 JSON 可序列化的结果，抛出错误时进入流程的失败、重试或错误分支 |

非法脚本、重复类型和覆盖内置类型会使启动报错，并提示对应文件。删除或重命名已被流程使用的节点类型后，需要更新这些流程。

## 声明所需配置

`config_schema` 与流程节点已有的所需配置格式相同，`key`、`type` 指定名称和类型，`required: true` 表示必填，`secret: true` 遮蔽密钥输入，`default` 指定默认值。默认值同时用于表单预填和运行，省略或清空该项时使用默认值；`false` 和 `0` 也是有效默认值。

七牛节点已声明 `access_key`、`secret_key`、`bucket`、`domain` 必填，两个 key 使用密钥输入；`upload_url` 可不填，默认 `https://upload-z2.qiniup.com`。无需再读取 `qiniu-upload.json`。

注册声明会随节点创建、导入和保存自动带入流程，重启时也会更新已有流程的声明并保留已填值。注册声明为准，编辑流程不能删除注册的必填项。缺少必填配置时，配置保存和流程运行会报错。业务入参与配置分开传给 `execute(arguments_value, config)`，无需把密钥放进 `arguments`。

配置沿用现有的本地数据库存储与运行快照机制，不是加密凭据库；密钥不会出现在可复用的流程定义中。

## 传参和读取结果

自定义节点统一使用 `config.arguments` 传参。它支持与服务节点相同的作用域模板，保存和导入时校验引用：

```json
{
  "arguments": { "name": "{{input.name}}" },
  "output_key": "message",
  "timeout_seconds": 60
}
```

流程需要在 `context_schema` 声明 `name`。假设节点 ID 是 `greet`，下游读取 `output.greet.message.text`。未指定 `output_key` 时使用 `result`；`arguments` 默认为空对象。`default_config` 只用于编辑器预填，手写流程 JSON 时需要自己填写配置。

每次执行创建独立的 JS 运行环境，不通过模块变量共享业务状态。运行中使用启动时编译的脚本，不会临时读取被修改的文件。重启后运行及重试使用重新加载的脚本版本。

## 可用能力

复用[下载 Hook](./download-hooks.md)的运行环境，支持 `fetch`、`FormData`、`Buffer`、定时器，以及 `fs`、`fs/promises`、`crypto`、`buffer`、`url`、`console` 的有限实现；可使用这些模块的 `node:` 别名。不支持任意 npm 包、本地模块 import 或 `child_process`，也不是完整的 Node.js。

全局 `workdir` 为应用工作目录的绝对路径。`writeTextFile` 等 Hook 辅助函数默认以工作目录为根，可信脚本可设置 `__basePath` 指定清单等产物的保存目录。`fs.openAsBlob` 与 multipart 上传复用现有流式实现，无需一次读取整个视频。

脚本受运行取消、节点 `timeout_seconds`、流程剩余时间和脚本十分钟上限约束。`console` 输出进入应用日志；自动化节点日志记录参数、结果和错误。取消不能撤回已经完成的外部上传，重试可能再次产生外部副作用。

## 迁移下载完成 Hook

参见[下载完成后上传七牛云并通知 Webhook](./pipelines/download-finished-qiniu-webhook.md)。该示例将下载完成事件、七牛上传和 HTTP 通知拆为流程节点。

---
title: MCP Server
---

# MCP Server

项目提供 Streamable HTTP 与 stdio 两种 MCP 接入方式，能力与首页链接解析流程一致：

- `get_platform_status`：获取各平台当前可用状态。
- `fetch_content`：传入平台内容链接，等待解析完成并返回规范化内容。
- `download_content`：复用 `fetch_content` 的 `job_id`，或直接传入链接，创建并启动下载任务。
- `decrypt_wxchannels_video`：原地解密由 aria2 等第三方下载器保存的微信视频号视频。

## Streamable HTTP（推荐）

启动下载器时，内置 MCP 服务会默认开启。Agent 使用前端「设置 → MCP」页面显示的连接地址；默认配置下为：

```text
http://127.0.0.1:2022/mcp
```

设置页修改会立即生效，但不会写入配置文件。应用每次启动时 MCP 均会默认开启；手动关闭后 `/mcp` 会立即停止接受 Agent 请求，下次启动应用时会再次开启。MCP 与下载器 API 使用相同的监听地址；如果把 `api.hostname` 配置为局域网地址，应只向可信网络开放。

该开关只控制主服务内置的 `/mcp` HTTP 入口；下面的 stdio 命令由 MCP 客户端独立启动，不受此开关影响。

## stdio

stdio MCP server 同样复用下载器主服务的 API。先启动下载器：

```sh
wx_video_download server
```

再在 MCP 客户端中配置：

```json
{
  "mcpServers": {
    "dm": {
      "command": "/absolute/path/to/wx_video_download",
      "args": ["mcp"]
    }
  }
}
```

默认 API 地址来自项目配置中的 `api.protocol`、`api.hostname` 和 `api.port`。也可显式指定：

```json
{
  "mcpServers": {
    "dm": {
      "command": "/absolute/path/to/wx_video_download",
      "args": ["mcp", "--api-base-url", "http://127.0.0.1:2022"]
    }
  }
}
```

## 下载行为

`fetch_content` 成功后会返回 `job_id`，将它原样传给 `download_content`：

```json
{
  "job_id": "fetch-...",
  "wait_for_completion": true
}
```

旧参数名 `fetch_id` 仍然兼容，但新调用应统一使用 `job_id`。`download_content` 默认在任务创建并启动后返回；需要等待文件实际下载完成时，传入 `wait_for_completion: true`。重复任务默认报错，可通过 `existing_action` 选择 `skip`、`overwrite` 或 `duplicate`。其中 `overwrite` 会覆盖已有任务和文件。

## 微信视频号外部下载与解密

微信视频号的 `fetch_content` 结果会额外返回 `download_resources`。每项包含适合交给 aria2、curl 等第三方下载器使用的信息：

```json
{
  "resource": {
    "name": "视频名称",
    "kind": "video/mp4",
    "extra": "{\"decode_key\":\"123456789\"}"
  },
  "endpoints": [
    {
      "protocol": "https",
      "url": "https://example/video.mp4",
      "enabled": 1
    }
  ],
  "download_url": "https://example/video.mp4",
  "decode_key": "123456789",
  "requires_decryption": true
}
```

将 `download_url` 交给外部下载器，例如：

```sh
aria2c --out video.mp4 "<download_url>"
```

外部下载必须已经完成，并且文件必须位于运行下载器服务的同一台机器上。如果 `requires_decryption` 为 `true`，把绝对路径与返回的字符串形式 `decode_key` 传给：

```json
{
  "name": "decrypt_wxchannels_video",
  "arguments": {
    "file_path": "/absolute/path/to/video.mp4",
    "key": "123456789"
  }
}
```

解密会原地覆盖该文件，不能对同一文件重复调用。如果 `requires_decryption` 为 `false`，下载结果已经可以直接使用，不要调用解密工具。

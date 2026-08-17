---
title: MCP Server
---

# MCP Server

项目提供 Streamable HTTP 与 stdio 两种 MCP 接入方式，能力与首页链接解析流程一致：

- `get_platform_status`：获取各平台当前可用状态。
- `fetch_content`：传入平台内容链接，等待解析完成并返回规范化内容。
- `download_content`：复用 `fetch_content` 的 `job_id`，或直接传入链接，创建并启动下载任务。

## Streamable HTTP（推荐）

启动下载器后，在前端的「设置 → MCP」中开启服务。Agent 使用设置页显示的连接地址；默认配置下为：

```text
http://127.0.0.1:2022/mcp
```

设置页修改会立即生效，但不会写入配置文件。应用每次启动时 MCP 均为关闭状态，需要从前端重新开启。关闭后 `/mcp` 会立即停止接受 Agent 请求。MCP 与下载器 API 使用相同的监听地址；如果把 `api.hostname` 配置为局域网地址，应只向可信网络开放。

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

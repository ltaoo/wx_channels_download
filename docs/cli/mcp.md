---
title: MCP Server
---

# MCP Server

项目提供 Streamable HTTP 与 stdio 两种 MCP 接入方式，能力与首页链接解析流程一致：

- `get_platform_status`：获取各平台当前可用状态。
- `fetch_content`：传入平台内容链接，等待解析完成并返回规范化内容。
- `download_content`：复用 `fetch_content` 的 `job_id`，或直接传入链接，创建并启动下载任务。
- `decrypt_wxchannels_video`：原地解密由 aria2 等第三方下载器保存的微信视频号视频。
- `get_wxchannels_status`：检查视频号页面是否已连接。
- `search_wxchannels_accounts`：搜索视频号账号。
- `get_wxchannels_account_videos`：获取账号发布的视频列表。
- `get_wxchannels_live_replays`：获取账号的直播回放。
- `get_wxchannels_interacted_videos`：获取当前用户赞过或收藏的视频。
- `get_wxchannels_followed_accounts`：获取当前用户关注的视频号账号。
- `get_wxchannels_play_history`：获取当前用户的视频号播放记录。
- `get_wxchannels_video_profile`：通过链接、`oid`/`nid` 或 `eid` 获取视频详情。
- `get_wxchannels_video_comments`：获取视频评论或根评论的回复。
- `get_wxchannels_video_share_url`：通过 `oid` 获取视频分享链接。
- `get_download_tasks`：分页获取下载任务和状态统计。
- `get_download_task_detail`：获取下载任务、文件和关联内容详情。
- `get_accounts`：分页获取数据库中的平台账号。
- `get_browse_history`：分页获取浏览记录。
- `get_logs`：分页读取并筛选应用日志。
- `get_certificate_status`：获取代理根证书的安装及信任状态。

## Streamable HTTP（推荐）

启动下载器时，内置 MCP 服务会默认开启。Agent 使用前端「设置 → MCP」页面显示的连接地址；默认配置下为：

```text
http://127.0.0.1:2022/mcp
```

设置页修改会立即生效，但不会写入配置文件。应用每次启动时 MCP 均会默认开启；手动关闭后 `/mcp` 会立即停止接受 Agent 请求，下次启动应用时会再次开启。MCP 与下载器 API 使用相同的监听地址；如果把 `api.hostname` 配置为局域网地址，应只向可信网络开放。

该开关只控制主服务内置的 `/mcp` HTTP 入口；下面的 stdio 命令由 MCP 客户端独立启动，不受此开关影响。

## 微信视频号 API

视频号 API 工具通过已打开的视频号页面访问微信接口。使用前先确认下载器代理工作正常，并已有页面连接到 `/ws/channels`；可调用 `get_wxchannels_status` 检查，返回的 `available` 应为 `true`。

列表工具返回微信原始分页字段。继续翻页时，把上一页的 `data.lastBuffer`（账号搜索使用 `data.lastBuff`）原样传给下一次调用的 `next_marker`，不要自行解码或改写。

## 本地数据查询

内置 Streamable HTTP MCP 直接使用下载器当前进程中的数据库和运行时 service，查询下载任务、账号、浏览记录、日志和证书状态。独立 stdio MCP 不会再次打开数据库，而是通过已经启动的下载器 API 返回同样的数据。

所有数据查询工具均为只读。`get_download_tasks` 支持按状态、父任务和根任务筛选；`get_accounts` 支持账号 ID 和关键词；`get_browse_history` 支持平台、关联账号和关键词；`get_logs` 支持日志级别、来源和关键词。列表工具默认分页，并限制单页最大返回量。

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

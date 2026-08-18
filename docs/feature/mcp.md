---
title: MCP
---

# MCP

MCP（Model Context Protocol）让支持 MCP 的 AI 客户端以工具调用的方式访问下载器。当前实现提供应用配置管理、内容解析与下载、微信视频号查询，以及下载任务、账号、浏览记录、日志和证书状态等本地数据查询能力。

下载器支持两种接入方式：

- **Streamable HTTP（推荐）**：使用主服务的 `/mcp` 端点。内置 MCP 直接访问当前进程中的数据库和运行时服务。
- **stdio**：由 AI 客户端启动 `wx_video_download mcp` 子进程，再通过已经运行的下载器 API 完成操作。

两种接入方式提供相同的 MCP 工具。需要更完整的 stdio 配置与视频号外部下载说明，可参阅 [MCP Server 命令行文档](/cli/mcp)。

## 启用

### Streamable HTTP

正常启动下载器后，内置 MCP 默认启用。进入下载器的「设置 → MCP」可以查看连接地址并启用或停用服务。默认地址为：

```text
http://127.0.0.1:2022/mcp
```

在 AI 客户端中新增 Streamable HTTP 类型的 MCP Server，并填入这个地址即可。设置页中的开关只在当前进程内生效，不写入配置文件；下载器下次启动时 MCP 会重新默认启用。

也可以通过 API 管理开关：

```sh
# 查看状态、连接地址和可用工具
curl http://127.0.0.1:2022/api/mcp/status

# 启用
curl -X POST http://127.0.0.1:2022/api/mcp/enable

# 停用
curl -X POST http://127.0.0.1:2022/api/mcp/disable
```

MCP 与下载器 API 共用监听地址。如果将 `api.hostname` 配置为局域网地址，请只向可信网络开放，因为 MCP 可以读取和修改应用配置、读取本地任务、账号、浏览记录和日志，也可以创建下载任务、写入或覆盖文件。

### stdio

先启动下载器主服务：

```sh
wx_video_download server
```

然后在 AI 客户端中配置 stdio MCP Server：

```json
{
  "mcpServers": {
    "wx_channels_download": {
      "command": "/absolute/path/to/wx_video_download",
      "args": ["mcp"]
    }
  }
}
```

`mcp` 命令默认读取 `api.protocol`、`api.hostname` 和 `api.port`。连接其他下载器实例时可显式指定 API 地址：

```json
{
  "mcpServers": {
    "wx_channels_download": {
      "command": "/absolute/path/to/wx_video_download",
      "args": ["mcp", "--api-base-url", "http://127.0.0.1:2022"]
    }
  }
}
```

stdio 进程仍依赖已启动的下载器主服务。设置页中的 MCP 开关只控制 `/mcp` HTTP 端点，不控制由 AI 客户端独立启动的 stdio 进程。

## 命令

MCP 中的“命令”以工具（tool）的形式提供。调用时由 AI 客户端传入 JSON 参数。

### 应用配置

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_config` | 无 | 获取可配置字段的 schema、当前值和解析后的生效值；密码、Cookie、Token 等敏感字段不返回明文。 |
| `update_config` | `values` | 批量校验并保存非只读配置；实际值发生变化后自动安排应用优雅重启。 |
| `get_restart_status` | `restart_token` | 在连接恢复后确认是否已切换到新进程，以及新进程加载的配置是否与保存结果一致。 |

`get_config` 将 `internal/config/config.go` 注册的应用配置放在 `application_fields`，将 adapter 提供的配置放在 `plugin_fields`，并在兼容字段 `fields` 中返回两者合集。每个字段还带有 `source`；插件字段额外带有 `namespace`。

`update_config` 的键必须来自 `get_config`。值必须符合字段类型和 `options` 枚举约束。配置文件使用原子替换方式保存；成功响应中的 `restart_scheduled: true` 只表示重启已经安排，不能据此宣称重启完成。调用方必须保存 `restart_token`，在连接恢复后调用 `get_restart_status`。只有响应同时满足 `status: "completed"`、`restart_completed: true` 和 `config_applied: true`，才能确认新进程已经运行且保存后的配置已经加载。`pending` 需要继续重试，`failed` 表示重启请求失败，`config_mismatch` 表示进程已更换但配置摘要不一致。

重启期间 HTTP、WebSocket 和 MCP 连接可能短暂断开，客户端应重新连接。若修改了 `api.hostname` 或 `api.port`，应改用新地址连接；stdio 客户端也需要更新 API 地址或重新启动。提交的值与当前值完全相同时不会写盘或重启。

### 内容解析与下载

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_platform_status` | 无 | 获取支持的平台及其当前可用状态。 |
| `fetch_content` | `url`；可选 `force_refresh`、`timeout_seconds` | 解析内容链接，返回规范化内容、下载预览和可复用的 `job_id`。 |
| `download_content` | `job_id`、兼容参数 `fetch_id` 或 `url` 三选一 | 创建并启动下载任务；支持目录、文件名、视频规格、重复任务策略和等待完成等参数。 |
| `decrypt_wxchannels_video` | `file_path`、`key` | 原地解密由第三方下载器保存的视频号视频。 |

::: danger 写文件操作
`download_content` 会写入下载目录，调用前应先确认下载目标和覆盖策略。`existing_action: "overwrite"` 会覆盖已有任务及文件。`decrypt_wxchannels_video` 会原地覆盖指定文件，只能在 `fetch_content` 返回 `requires_decryption: true` 时使用。
:::

### 微信视频号

这些工具通过已打开的视频号页面访问微信接口。先调用 `get_wxchannels_status`，确认返回的 `available` 为 `true`。

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_wxchannels_status` | 无 | 检查视频号页面是否已连接。 |
| `search_wxchannels_accounts` | `keyword`，可选 `next_marker` | 搜索视频号账号。 |
| `get_wxchannels_account_videos` | `username`，可选 `next_marker` | 获取账号发布的视频列表。 |
| `get_wxchannels_live_replays` | `username`，可选 `next_marker` | 获取账号的直播回放。 |
| `get_wxchannels_interacted_videos` | 可选 `flag`、`next_marker` | 获取当前用户赞过或收藏的视频，`flag` 默认值为 `7`。 |
| `get_wxchannels_followed_accounts` | 可选 `next_marker` | 获取当前用户关注的视频号账号。 |
| `get_wxchannels_play_history` | 可选 `next_marker` | 获取当前用户最近的视频号播放记录。 |
| `get_wxchannels_video_profile` | `url`，或 `oid` + `nid`，或 `eid` | 获取单个视频详情。 |
| `get_wxchannels_video_comments` | `oid` + `nid`，或 `oid` + `comment_id`；可选 `next_marker` | 获取一级评论或指定根评论的回复。 |
| `get_wxchannels_video_share_url` | `oid` | 获取视频的 H5 分享链接。 |

列表工具会保留微信接口的分页字段。继续翻页时，将上一页 `data.lastBuffer` 原样传给 `next_marker`；只有账号搜索使用 `data.lastBuff`。游标不需要解码或修改。

### 本地数据

本地数据工具均为只读，并支持用筛选参数减少返回量。Streamable HTTP 直接使用当前进程中的数据库与运行时服务；stdio 则通过下载器 API 查询相同数据。

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_download_tasks` | `page`、`page_size`、`statuses`、`parent_task_id`、`root_task_id` | 分页获取下载任务和状态统计。状态 `0` 至 `7` 分别表示等待、准备、下载中、暂停、合并、完成、失败、取消。 |
| `get_download_task_detail` | `id` | 获取任务、文件、进度、关联内容和账号详情。 |
| `get_accounts` | `page`、`page_size`、`keyword`、`account_id` | 分页查询数据库中的平台账号。 |
| `get_browse_history` | `page`、`page_size`、`keyword`、`username`、`platform_ids` | 分页查询浏览记录；`username` 是数据库账号 ID，例如 `wxchannels:xxx`。 |
| `get_logs` | `page`、`page_size`、`max_bytes`、`keyword`、`source`、`levels`、`format_json` | 分页读取并筛选应用日志。 |
| `get_certificate_status` | 无 | 获取代理根证书的来源、安装、信任状态和风险提示。 |

## 常用场景

### 修改下载目录

先调用 `get_config` 确认 `download.dir` 的类型和当前值，再调用：

```json
{
  "name": "update_config",
  "arguments": {
    "values": {
      "download.dir": "/data/downloads"
    }
  }
}
```

工具返回成功后只代表重启已安排。连接恢复后，将返回的 `restart_token` 传给 `get_restart_status`；确认其返回 `completed` 且 `config_applied: true` 后，才能告知用户下载器已重启、新目录已经生效。新建下载任务将使用新的默认目录。

### 解析链接并下载

推荐的调用顺序是：

1. 调用 `get_platform_status`，确认目标平台可用。
2. 使用 `fetch_content` 解析链接并检查标题、资源和下载预览。
3. 向用户确认下载目录、文件名和覆盖策略。
4. 将返回的 `job_id` 传给 `download_content`，避免重复解析。

```json
{
  "name": "download_content",
  "arguments": {
    "job_id": "fetch-...",
    "wait_for_completion": true,
    "existing_action": "error"
  }
}
```

### 查询账号内容和浏览记录

查找某个视频号账号时，先用 `search_wxchannels_accounts` 获取 `username`，再调用 `get_wxchannels_account_videos` 或 `get_wxchannels_live_replays`。查询当前微信用户的关注账号、赞或收藏内容、播放记录时，可直接使用对应的三个列表工具。

如果要查询已经保存到下载器数据库中的历史数据，则使用 `get_accounts` 和 `get_browse_history`。这类查询不依赖视频号页面保持连接。

### 排查下载失败

先用 `get_download_tasks` 筛选失败任务，再读取任务详情和相关日志：

```json
{
  "name": "get_download_tasks",
  "arguments": {
    "statuses": [6],
    "page": 1,
    "page_size": 20
  }
}
```

取得任务 ID 后调用 `get_download_task_detail`，再根据错误文本调用 `get_logs`，通过 `keyword`、`source` 或 `levels: ["error"]` 缩小范围。遇到代理或 HTTPS 问题时，可追加调用 `get_certificate_status` 检查根证书安装和信任状态。

### 使用第三方下载器保存视频号视频

`fetch_content` 的视频号结果会包含 `download_resources`。可将其中的 `download_url` 交给 aria2、curl 等第三方下载器；下载完成后，仅当 `requires_decryption` 为 `true` 时，使用同一项中的字符串形式 `decode_key` 调用 `decrypt_wxchannels_video`。文件必须位于运行下载器服务的同一台机器上，并使用绝对路径。

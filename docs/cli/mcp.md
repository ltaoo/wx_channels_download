---
title: MCP Server
---

# MCP Server

项目提供 Streamable HTTP 与 stdio 两种 MCP 接入方式，包含以下语义化 tools：

- `get_config`：获取应用配置 schema、当前值和解析后的生效值；敏感值不会返回明文。
- `update_config`：批量修改非只读配置，保存成功后安排应用优雅重启。
- `get_restart_status`：通过 `update_config` 返回的确认令牌验证新进程和新配置是否已经生效。
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
- `delete_download_tasks`：批量删除下载任务，可通过 `delete_files` 选择是否同时删除关联的本地文件。
- `get_accounts`：分页获取数据库中的平台账号。
- `get_browse_history`：分页获取浏览记录。
- `get_logs`：分页读取并筛选应用日志。
- `get_certificate_status`：获取代理根证书的安装及信任状态。

## Streamable HTTP（推荐）

启动下载器后，使用前端「设置 → MCP」页面显示的连接地址；默认地址为：

```text
http://127.0.0.1:2022/mcp
```

设置页可立即启用或关闭 Streamable HTTP MCP。将 `api.hostname` 配置为局域网地址时，只应向可信网络开放。

## 微信视频号工具

视频号工具通过已打开的视频号页面获取数据。使用前先确认下载器代理工作正常，并已有页面连接到 `/ws/channels`；可调用 `get_wxchannels_status` 检查，返回的 `available` 应为 `true`。

列表工具返回微信原始分页字段。继续翻页时，把上一页的 `data.lastBuffer`（账号搜索使用 `data.lastBuff`）原样传给下一次调用的 `next_marker`，不要自行解码或改写。

## 数据查询

所有数据查询工具均为只读。`get_download_tasks` 支持按状态、父任务和根任务筛选；`get_accounts` 支持账号 ID 和关键词；`get_browse_history` 支持平台、关联账号和关键词；`get_logs` 支持日志级别、来源和关键词。列表工具默认分页，并限制单页最大返回量。

## stdio

使用 stdio MCP 前先启动下载器：

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

## 配置管理

修改配置前先调用 `get_config` 获取合法字段、字段类型和枚举选项。敏感字段只返回是否已配置，不返回明文。`update_config` 接收 `values` 对象，例如：

```json
{
  "values": {
    "download.dir": "/data/downloads",
    "download.playDoneAudio": false
  }
}
```

未知字段、只读字段、类型不匹配或不在 `options` 中的值会被拒绝。值发生变化后应用会优雅重启，MCP 连接可能短暂中断。修改 `api.hostname` 或 `api.port` 后，应使用新地址连接；stdio 客户端需要同步更新 API 地址或重新启动。相同值不会触发重启。

`update_config` 返回 `restart_scheduled: true` 时只表示已安排重启。调用方必须保留 `restart_token`，连接恢复后调用 `get_restart_status`。只有返回 `status: "completed"`、`restart_completed: true`、`config_applied: true` 时，才表示新进程已经启动并加载了保存后的配置；在此之前不应向用户声称重启完成。

## 下载行为

`fetch_content` 成功后会返回 `job_id`，将它原样传给 `download_content`：

```json
{
  "job_id": "fetch-...",
  "wait_for_completion": true
}
```

`download_content` 默认在任务创建并启动后返回；需要等待文件实际下载完成时，传入 `wait_for_completion: true`。重复任务默认报错，可通过 `existing_action` 选择 `skip`、`overwrite` 或 `duplicate`。其中 `overwrite` 会覆盖已有任务和文件。

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

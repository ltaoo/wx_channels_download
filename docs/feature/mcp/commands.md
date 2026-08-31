---
title: MCP 命令
---

# MCP 命令

MCP 中的“命令”以工具（tool）的形式提供。调用时由 AI 客户端传入 JSON 参数。

## 应用配置

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_config` | 无 | 获取可配置字段的 schema、当前值和解析后的生效值；密码、Cookie、Token 等敏感字段不返回明文。 |
| `update_config` | `values` | 批量校验并保存非只读配置；实际值发生变化后自动安排应用优雅重启。 |
| `get_restart_status` | `restart_token` | 在连接恢复后确认是否已切换到新进程，以及新进程加载的配置是否与保存结果一致。 |

`get_config` 将 `internal/config/config.go` 注册的应用配置放在 `application_fields`，将 adapter 提供的配置放在 `plugin_fields`，并在兼容字段 `fields` 中返回两者合集。每个字段还带有 `source`；插件字段额外带有 `namespace`。

`update_config` 的键必须来自 `get_config`。值必须符合字段类型和 `options` 枚举约束。配置文件使用原子替换方式保存；成功响应中的 `restart_scheduled: true` 只表示重启已经安排，不能据此宣称重启完成。调用方必须保存 `restart_token`，在连接恢复后调用 `get_restart_status`。只有响应同时满足 `status: "completed"`、`restart_completed: true` 和 `config_applied: true`，才能确认新进程已经运行且保存后的配置已经加载。`pending` 需要继续重试，`failed` 表示重启请求失败，`config_mismatch` 表示进程已更换但配置摘要不一致。

重启期间 HTTP、WebSocket 和 MCP 连接可能短暂断开，客户端应重新连接。若修改了 `api.hostname` 或 `api.port`，应改用新地址连接；stdio 客户端也需要更新 API 地址或重新启动。提交的值与当前值完全相同时不会写盘或重启。

## Worker 部署

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `deploy_sph_worker` | 无 | 从应用配置读取 Cloudflare 凭证，部署或覆盖视频号查询 Worker，并返回 workers.dev 地址。 |

`deploy_sph_worker` 读取 `cloudflare.accountId`、`cloudflare.apiToken`、`cloudflare.sphWorkerName`、`cloudflare.sphCookie` 和 `cloudflare.sphCredential`，不会通过 MCP 参数传输这些敏感值。调用会覆盖同名远端 Worker，因此必须先获得用户确认。`get_config` 可用时，可先检查这些字段的 `configured` 状态。

## 内容解析与下载

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_platform_status` | 无 | 获取支持的平台及其当前可用状态。 |
| `fetch_content` | `url`；可选 `force_refresh`、`timeout_seconds` | 解析内容链接，返回规范化内容、下载预览和可复用的 `job_id`。 |
| `download_content` | `job_id`、兼容参数 `fetch_id` 或 `url` 三选一 | 创建并启动下载任务；支持目录、文件名、视频规格、重复任务策略和等待完成等参数。 |
| `decrypt_wxchannels_video` | `file_path`、`key` | 原地解密由第三方下载器保存的视频号视频。 |

::: danger 写文件操作
`download_content` 会写入下载目录，调用前应先确认下载目标和覆盖策略。`existing_action: "overwrite"` 会覆盖已有任务及文件。`decrypt_wxchannels_video` 会原地覆盖指定文件，只能在 `fetch_content` 返回 `requires_decryption: true` 时使用。
:::

## 微信视频号

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

## 本地数据

本地数据工具均为只读，并支持用筛选参数减少返回量。Streamable HTTP 直接使用当前进程中的数据库与运行时服务；stdio 则通过下载器 API 查询相同数据。

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_download_tasks` | `page`、`page_size`、`statuses`、`parent_task_id`、`root_task_id` | 分页获取下载任务和状态统计。状态 `0` 至 `7` 分别表示等待、准备、下载中、暂停、合并、完成、失败、取消。 |
| `get_download_task_detail` | `id` | 获取任务、文件、进度、关联内容和账号详情。 |
| `get_accounts` | `page`、`page_size`、`keyword`、`account_id` | 分页查询数据库中的平台账号。 |
| `get_browse_history` | `page`、`page_size`、`keyword`、`username`、`platform_ids` | 分页查询浏览记录；`username` 是数据库账号 ID，例如 `wxchannels:xxx`。 |
| `get_logs` | `page`、`page_size`、`max_bytes`、`keyword`、`source`、`levels`、`format_json` | 分页读取并筛选应用日志。 |
| `get_certificate_status` | 无 | 获取代理根证书的来源、安装、信任状态和风险提示。 |

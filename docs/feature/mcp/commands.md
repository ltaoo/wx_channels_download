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

重启期间 HTTP、WebSocket 和 MCP 连接可能短暂断开，客户端应重新连接。若修改了 `api.hostname` 或 `api.port`，应改用新地址连接；stdio 客户端只在启动时读取一次配置，需要重新启动才能生效。提交的值与当前值完全相同时不会写盘或重启。

## 管理自动化流程实例

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `list_automation_instances` | 无 | 返回 `list`，用于定位实例 ID、标题和所属流程。 |
| `get_automation_instance` | `id` | 获取实例最新配置、默认配置和共享流程定义。 |
| `create_automation_instance` | `flow_id`；可选 `name` | 为已有流程定义创建配置实例。 |
| `update_automation_instance` | `id` 及需要修改的实例字段 | 修改实例标题、说明、节点配置和触发规则。 |
| `delete_automation_instance` | `id` | 删除指定实例及其触发规则。 |

`create_automation_instance` 接收已有流程定义的 `flow_id`，以及可选的实例名称 `name`。`flow_id` 是流程定义 ID，不是实例 ID；可从流程编辑页 URL 的 `id` 参数取得。

以已存在的流程定义 `flow-example` 为例，MCP `tools/call` 参数为：

```json
{
  "name": "create_automation_instance",
  "arguments": {
    "flow_id": "flow-example",
    "name": "创作者视频同步实例"
  }
}
```

成功时返回创建后的实例记录，`id` 是新实例 ID，`flow_id` 是所属流程定义 ID。省略 `name` 或传入空字符串时，默认使用流程名称加“（实例）”；仅含空白字符的名称会被拒绝。

新实例共享流程图并继承默认配置和触发器，已启用的默认定时或事件规则会生效；创建操作不会立即手动运行流程。每次成功调用都会新增实例，若调用超时或连接中断，先在页面核对实例列表，再决定是否重试。

编辑前先调用 `get_automation_instance`；其中 `node_configs`、`default_node_configs`、`default_triggers` 和 `trigger_overrides` 可能是 JSON 字符串，需先解析。自定义触发器的当前规则可通过 `list_automation_schedules` 返回的 `instance_id` 定位。以下示例假定已查询到实例 `instance-example`：

```json
{
  "name": "update_automation_instance",
  "arguments": {
    "id": "instance-example",
    "name": "创作者视频每日同步",
    "description": "视频同步配置实例"
  }
}
```

未提交的字段保持原值。`description: ""` 清空说明，`name` 不能为空。MCP 编辑工具不接受共享流程图和流程默认配置字段。

- `node_configs` 按节点 ID 到配置对象组织，提交的是**完整的实例覆盖集合**。修改单项前合并已有覆盖，避免丢失其他配置；`{}` 清空全部覆盖并使用流程默认配置。
- `triggers` 是完整自定义触发器列表，每项沿用现有格式：`id`（已有规则保留、新规则省略）、`type`（`Cron`、`Event`、`Webhook`、`Manual`）、`name`、`cron_expr`、`event_key`、`enabled`、`active`、`initial_data`、`timeout_sec`。`[]` 清空规则并取消继承；修改后立即生效。
- `inherit_triggers: true` 恢复默认触发器并清空触发器覆盖；不能同时提交自定义 `triggers`。`false` 仅取消继承，不停用现有规则。
- `trigger_overrides` 按默认触发器 ID 组织，是完整覆盖集合；提交后启用默认触发器继承，`{}` 恢复默认。不能与 `triggers` 或 `inherit_triggers` 同时提交。

删除时，先查询并核对用户指定的实例 ID 和标题，再调用 `delete_automation_instance`。删除会移除该实例及其触发规则，保留共享流程、默认配置和历史执行记录；不会中止已经启动的执行。

## Pipeline 实时编辑

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_pipeline_editor` | 可选 `editor_id`、`flow_id` | 从打开的编辑页面读取当前草稿、版本号、选中节点和节点目录，包括尚未保存的改动。 |
| `update_pipeline_editor` | `editor_id`、`revision`、`draft` | 校验草稿并通过 WebSocket 更新指定页面，等待页面应用后返回最新快照。 |

先在下载器中打开 `/automation/edit?id=<Pipeline ID>`，并在设置中启用 MCP。HTTP MCP 和连接同一下载器 API 的 stdio MCP 都支持这两个工具，无需额外配置模型或内置 Agent。

先调用 `get_pipeline_editor`。仅有一个编辑页面时直接返回 `editor_id`、`snapshot` 和 `catalog`；多个页面时返回 `selection_required: true` 和 `editors`，选择明确的 `editor_id` 后再次读取。同一个 Pipeline 的多个标签页也需要明确选择。

修改时使用最新 `snapshot.revision` 和 `snapshot.draft`，保留无关节点及其 ID、配置、位置和输入定义。`nodes` 为完整节点列表，`next_ids` 表示连线；条件分支需同步 `config.rules[].target_id`。`context_schema` 保留原输入定义。`triggers` 省略或为 `null` 时保留触发器表单，`[]` 清空额外触发器。`snapshot.node_config_text` 返回正在输入的节点配置；尚未应用时，先在页面应用或还原该文本，再提交 MCP 修改。

用户继续编辑后旧 `revision` 会被拒绝，必须重新读取并合并。成功返回 `applied: true`、`saved: false`：画布已更新并标记未保存，点击页面的「保存 Pipeline」后持久化。超时、断线或页面关闭时无法确认结果，应先重新读取核对，避免直接重放修改。

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
| `get_wxchannels_live_profile` | `username` + `oid` + `nid` + `id` | 获取直播详情和直播流信息，`id` 对应 `liveId`。 |
| `get_wxchannels_interacted_videos` | 可选 `flag`、`next_marker` | 获取当前用户赞过或收藏的视频，`flag` 默认值为 `7`。 |
| `get_wxchannels_followed_accounts` | 可选 `next_marker` | 获取当前用户关注的视频号账号。 |
| `get_wxchannels_play_history` | 可选 `next_marker` | 获取当前用户最近的视频号播放记录。 |
| `get_wxchannels_video_profile` | `url`，或 `oid` + `nid`，或 `eid` | 获取单个视频详情。 |
| `get_wxchannels_video_comments` | `oid` + `nid`，或 `oid` + `comment_id`；可选 `next_marker` | 获取一级评论或指定根评论的回复。 |
| `get_wxchannels_video_share_url` | `oid` | 获取视频的 H5 分享链接。 |

列表工具会保留微信接口的分页字段。继续翻页时，将上一页 `data.lastBuffer` 原样传给 `next_marker`；只有账号搜索使用 `data.lastBuff`。游标不需要解码或修改。

## 微信公众号

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_wxmp_biz_msg_list` | `username`，可选 `offset` | 获取指定微信公众号的历史消息列表；继续翻页时把上一页响应中的分页游标传给 `offset`。 |

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

## 按账号整理 Kodi 媒体库

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `organize_account_files` | `account_id`、`output_dir`；可选 `dry_run` | 移动并重命名账号已完成的本地视频，更新数据库路径，每个视频独立成电影目录并生成同名 NFO。HTTP MCP 与 stdio MCP 均支持。 |

先通过 `get_accounts` 获取完整账号 ID，再预览：

```json
{"account_id":"wxchannels:xxx","output_dir":"/media/Kodi","dry_run":true}
```

省略 `dry_run` 也只预览，不创建目录或文件，不更新数据库。确认移动目标后用相同参数设置 `dry_run:false` 执行。`output_dir` 是**下载器所在主机**的绝对路径，远程 MCP 客户端也在下载器主机上执行整理。

输出示例：

```text
/media/Kodi/
  账号名称 [账号ID摘要]/
    视频标题 [12]/
      视频标题 [12].mp4
      视频标题 [12].nfo
```

每个已完成的视频资源作为一部独立电影，按账号分目录归档，无季、集编号。目录和文件名中的 `[12]` 是资源 ID，用于区分同名视频和保持重复执行的路径稳定；多个清晰度或分段资源会各自输出为独立电影。支持常见本地视频格式；图文、音频、未完成下载、流资源和已删除记录不参与整理。缺失或空文件返回逐项错误。

NFO 根节点使用 `movie`，包含标题、简介、账号（`studio`）、发布日期（`premiered`）、年份和自定义唯一 ID。发布日期与年份取自内容的发布时间，未知时省略。封面以海报 URL 写入，显示图片仍需要可访问的 URL。文件名会清理路径分隔符和非法字符，XML 自动转义。NFO 与视频同目录、同名，遵循 [Kodi 电影 NFO](https://kodi.wiki/view/NFO_files/Movies) 与[电影文件名规范](https://kodi.wiki/view/Naming_video_files/Movies)。

执行会移动并重命名原视频，同时更新数据库 `download_resource.download_dir`、`name` 和 `updated_at`；下载任务和内容关联保持不变，后续播放、详情和文件管理使用新路径。NFO 始终按电影格式生成。

同一文件系统内优先建立新文件路径，数据库提交成功后移除旧路径，不复制视频数据；跨文件系统或不支持硬链接时先复制并同步写入，再提交数据库并移除原文件，因此跨盘移动需要临时空间。数据库提交前失败会保留原视频并清理本次新建文件，不移除事先存在的相同目标文件。若数据库已经提交但旧文件移除失败，会返回错误，新位置和数据库路径仍有效，旧文件会保留。文件系统和数据库无法组成一个跨系统事务，进程强制退出或断电可能留下两份文件，需要核对返回路径与数据库。

内容相同的目标文件允许重试，内容不同的已有视频或 NFO 一律不覆盖。多个资源记录共用同一源路径或源文件为符号链接时拒绝移动，避免其他记录失效。已完成移动的成功项不会因后续项失败而撤销；单项失败记录在 `files[].status="error"` 与 `error`，必须检查每项结果。`source` 是调用时的原路径，`video` 是目标路径；`planned` 表示仅规划且源文件可用，`ready` 表示视频移动、电影 NFO 和数据库路径更新均已完成。预览不校验写权限、磁盘余量或已有目标内容。元数据改变后若出现冲突，应先人工核对已有文件。

在 Kodi 中将 `output_dir` 添加为**电影**来源，信息提供者选择 **Local information only（仅本地信息）**，并启用递归扫描以扫描账号目录下的视频目录；也可以将单个账号目录添加为电影来源。其他播放器需要支持 Kodi 格式的本地 NFO，具体兼容性取决于应用。

## 数据同步

同步复用创建下载任务的保存流程，在目标端重建下载记录及其关联的内容、账号，并上传已有文件，不解析平台、不启动下载、不包含浏览记录，不要求两端数据库 ID 相同。完整流程见[同步数据到另一台下载器](./scenarios/sync-data.md)。

| 工具 | 主要参数 | 用途 |
| --- | --- | --- |
| `get_sync_skill` | 无 | 读取内置的同步 Skill；也可通过 MCP 资源 `dm://skills/sync-downloads` 读取。 |
| `get_sync_servers` | 无 | 查看已配置目标，不返回令牌。 |
| `configure_sync_server` | `name`、`url` | 按名称添加或更新目标，立即生效，无需令牌。 |
| `preview_sync` | 可选 `task_ids` | 只读统计选定任务的记录、文件及字节数。 |
| `sync_to_server` | `server_id`，`task_ids` 或 `all=true` | 用户授权后启动指定范围的后台同步，返回 `job_id`；全量须显式指定。 |
| `get_sync_status` | `job_id` | 查询任务，只有 `completed` 表示同步成功。 |

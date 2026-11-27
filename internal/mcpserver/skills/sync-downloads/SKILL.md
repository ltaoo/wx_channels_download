---
name: sync-downloads
description: 将本机已有下载任务及文件同步到另一台下载器或 NAS，通过 MCP 重建关联的内容、账号和资源，不重新下载。适用于单条、指定任务和用户明确要求的全量同步，不处理浏览记录。
---

# 同步已有下载

使用源下载器的 MCP 工具完成操作。用户明确要求同步即已授权，不需要再次确认同一范围。

1. 确定目标和任务范围。`get_sync_servers` 列出目标；目标不存在时，使用 `configure_sync_server` 传入名称和完整 HTTP(S) 地址。API 认证为可选配置；仅当目标服务同时配置非空的 `api.username`、`api.password` 时，才需要在目标 URL 中使用标准 Basic Auth 用户信息携带凭据，特殊字符需 URL 编码。`get_sync_servers` 不返回凭据。
2. 用户说“第一条”时，以 `get_download_tasks` 的列表顺序为准（最新任务在前），用 `page=1,page_size=1` 获取 ID；用 `get_download_task_detail` 核对标题和已有文件。确定 ID 后保持该范围，不因列表变化重新选择。指定 ID 时直接使用该 ID。
3. `preview_sync` 传入明确的 `task_ids`，核对任务、资源、文件数量和字节数。空的任务选择不能代替用户授权的单条范围；找不到任务时停止，不回退为全量同步。
4. 调用 `sync_to_server`，传 `server_id` 和相同的 `task_ids`。只有用户明确要求所有下载时才使用 `all=true`；不要同时传 `all` 和 `task_ids`。
5. 保存 `job_id`，通过 `get_sync_status` 查询到 `completed` 或 `failed`。运行中可报告 `completed_tasks / total_tasks`、正在传输的 `current_task_id` 和 `result` 中已成功的任务、文件及字节数。每条任务上传并提交后立即出现在目标下载列表。启动成功不等于全部完成。`completed` 时报告实际任务、文件、字节数和 `target_task_ids`；中途失败保留此前成功的任务，先核对 `result.target_task_ids` 和目标数据，避免盲目重复写入。

目标端复用创建下载任务的保存流程，自行生成 ID，并重建任务、资源、内容和账号的关联；不要求源、目标 ID 一致。已有文件同时上传并关联，不调用平台解析、下载器或创建下载任务的在线解析接口。

只同步选中的任务及其内容、账号、文件。未选中的父任务不会自动加入，所选子任务在目标作为独立任务创建。浏览记录、无关账号和内容不参与。缺失文件、`.part` 和未完成资源不会被报告为完整下载；目标保留暂停状态。

文件复制必须与目标 `DownloadResource` 记录保存处于同一同步流程；使用上述工具，不通过单独 SCP 或文件移动绕过记录关联。不要修改或删除源文件。

逐条同步要求两端支持协议 v3。接收端仍兼容 v1/v2 旧批次；旧批次保持整批提交，不能中途切换为逐条提交。

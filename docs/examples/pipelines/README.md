# 自动化 Pipeline 示例

此目录统一保存可导入的 Pipeline JSON，文档页面直接引用这些文件。可在「自动化 → 导入 Pipeline」中粘贴文件内容导入；已经导入的流程不会随示例文件自动更新。

| 文件 | 功能 | 建议触发方式 |
| --- | --- | --- |
| `01_creator_video_sync.json` | 通过 HTTP 接口逐页检查视频，仅下载缺失视频，遇到整页已存在时停止 | 每天 12:00、18:00（已内置） |
| `01_creator_video_sync_service.json` | 使用内置工具获取列表，其余增量检查与下载流程和 HTTP 版相同 | 每天 12:00、18:00（已内置） |
| `01_douyin_creator_video_sync.json` | 抖音：通过内置列表接口逐页检查作品，仅下载缺失视频，遇到整页已存在时停止 | 每天 12:00、18:00（已内置） |
| `02_creator_live_record.json` | 查询创作者当前直播，开播时创建录制任务 | Cron `*/1 * * * *` |
| `03_creator_new_video_notify.json` | 按本地下载任务判断新视频，支持企业微信和通用 Webhook | Cron `*/5 * * * *` |
| `download_finished_qiniu_webhook.json` | 通过 JS 自定义节点上传七牛云，再发送任务详情到 Webhook | 事件 `downloadtask.finished`（已内置） |
| `node_status_demo.json` | 通过子流程循环演示节点状态刷新 | 手动触发 |

导入步骤、配置、通知行为及运行边界统一见[流程示例说明](../../feature/pipelines/index.md)。新视频通知第一次运行就会通知扫描范围内没有下载任务的视频；尚未创建下载任务的视频可能在后续运行中再次通知。

七牛上传流程需先安装 `docs/examples/nodes/qiniu_upload.js`，详见[安装与 Hook 迁移](../../feature/pipelines/download-finished-qiniu-webhook.md)。

## 内置工具版视频同步

`01_creator_video_sync_service.json` 以 HTTP 版为基准，仅把「获取创作者视频列表」替换为 `get_wxchannels_account_videos` 内置工具，并移除 HTTP 专用的 `url`、`method`、`token` 配置。导入后同样在流程「配置」中填写 `username`；运行时需要视频号查询能力可用。HTTP 版只解析原始下划线字段；内置工具版只解析 `errCode/data` 包装及驼峰字段，两版不兼容对方的响应格式。定时触发、分页、批量检查、整页已存在时停止、并行提交下载及下载结果统计完全相同，下载均通过 `create_download_task` 复用列表媒体，不再查询详情。

内置工具版在顶层 `subflows` 中声明分页、列表查询和单个视频下载子流程。`WorkflowNode` 和 `LoopNode` 使用 `config.flow_id` 引用声明的键，循环和遍历会重复执行同一份定义；子流程内也可引用顶层的其他子流程。保存的模板保留引用，创建执行记录时冻结依赖，保证重试使用当时的定义。详情页直接展开子流程容器，显示内部节点的实时状态和日志。

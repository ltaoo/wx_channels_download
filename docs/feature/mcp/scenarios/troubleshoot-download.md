---
title: 排查下载失败
---

# 排查下载失败

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

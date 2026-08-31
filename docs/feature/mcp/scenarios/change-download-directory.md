---
title: 修改下载目录
---

# 修改下载目录

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

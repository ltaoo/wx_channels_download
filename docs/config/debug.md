---
title: 调试
---

# 调试

用于开发和调试的配置项。

## 错误捕获

```yaml
debug:
  error: true
  echolog: false
```

是否全局捕获前端错误，出现错误时弹窗展示错误信息。默认 `true`。

`echolog` 控制 Echo 代理日志，默认 `false`。设置为 `true` 时启用 Echo 日志。

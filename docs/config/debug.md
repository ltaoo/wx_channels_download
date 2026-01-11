---
title: 调试与 PageSpy
---

# 调试与 PageSpy

用于开发和调试的配置项。

## 错误捕获

```yaml
debug:
  error: true
```

是否全局捕获前端错误，出现错误时弹窗展示错误信息。默认 `true`。

## PageSpy 配置

PageSpy 是一个用于调试移动端网页的工具。

```yaml
pagespy:
  enabled: false
  protocol: "https"
  api: "debug.weixin.qq.com"
```

- `enabled`: 是否开启 PageSpy，默认 `false`。
- `protocol`: PageSpy 调试协议，可选 `http` 或 `https`，默认 `https`。
- `api`: PageSpy 调试 API 地址，默认 `debug.weixin.qq.com`。

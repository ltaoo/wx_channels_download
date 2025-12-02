---
title: 视频号
---

# 视频号

对视频号本身进行一些配置

## 禁止从详情页重定向到首页

默认情况下，从「文件助手」打开视频号时，会自动重定向到首页

```yaml
channel:
  disableLocationToHome: false
```

如果想禁止这个行为，可以在配置文件中将 `disableLocationToHome` 设置为 `true`：

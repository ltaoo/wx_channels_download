---
title: 视频号
---

# 视频号

对视频号本身进行一些配置

## 从详情页重定向到首页

从「文件助手」打开视频号时，应该跳转至「视频详情页」，但某些版本会自动重定向到首页。该配置可以用于解决这个问题，不要自动调整到首页

```yaml
channel:
  disableLocationToHome: false
```

是否禁止从详情页自动跳转至首页，默认 `false`，会自动跳转。将 `disableLocationToHome` 设置为 `true` 后，会禁止自动跳转至首页。

---
title: 视频号
---

# 视频号

通过 `config.yaml` 控制视频号及其下载行为。

## 启用视频号功能

```yaml
channels:
  enabled: true
```

是否启用视频号功能，默认 `true`。

## 从详情页重定向到首页

从「文件助手」打开视频号时，应该跳转至「视频详情页」，但某些版本会自动重定向到首页。该配置可以用于阻止自动跳转。

```yaml
channels:
  disableLocationToHome: false
```

是否禁止从详情页自动跳转至首页，默认 `false`，会自动跳转。将 `disableLocationToHome` 设置为 `true` 后，会禁止自动跳转至首页。

## 默认下载原始视频

```yaml
channels:
  download:
    defaultHighest: false
```

点击下载按钮时是否默认下载原始视频，默认 `false`。设置为 `true` 后会下载原始画质的视频。

## 前端下载

```yaml
channels:
  download:
    frontend: false
```

是否在前端完成下载和解密，默认 `false`，即创建后台下载任务。设置为 `true` 后，下载会在视频号页面中完成，无需调用后台下载能力。

前端下载长视频时可能失败，并且「下载目录」配置会失效，文件名也不支持使用 `/` 创建子目录。

## 同时下载封面

```yaml
channels:
  download:
    cover: false
```

下载视频或图集时是否同时下载封面，默认 `false`。该配置仅在后台下载时生效。

## 下载时暂停播放

```yaml
channels:
  download:
    pauseWhenDownload: false
```

前端下载视频时是否暂停当前视频的播放，默认 `false`。设置为 `true` 后，下载完成时会继续播放。

## 批量下载时检查所有视频

```yaml
channels:
  download:
    forceCheckAllFeeds: false
```

批量下载作者所有视频时是否强制检查全部视频，默认 `false`。

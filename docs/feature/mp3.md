---
title: 下载mp3
---

# 下载mp3

在「视频详情页」的「更多」菜单中，有「下载为mp3」菜单，点击即可下载 `mp3` 文件。目前有两种实现方式

## 前端下载

适用于短视频转换为 `mp3` 后下载，默认支持，无需额外配置

## 通过中转服务下载

适用于长视频转换为 `mp3` 后下载

该功能需要已经安装 `ffmpeg`，并配置到 `PATH` 环境变量中，在终端输入 `ffmpeg -version` 可以验证是否安装成功

确认 `ffmpeg` 已安装后，需要在 `config.yaml` 中开启 `download.localServer.enabled` 设置为 `true`

参考 [本地下载中转服务](../config/download.md#本地下载中转服务)


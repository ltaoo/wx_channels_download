---
title: 下载mp3
---

# 下载mp3

通过在本地开启下载中转服务，实现了在视频号页面直接下载 `mp3` 文件的功能，不再需要下载后手动转换

该功能需要已经安装 `ffmpeg`，并配置到 `PATH` 环境变量中，在终端输入 `ffmpeg -version` 可以验证是否安装成功

确认 `ffmpeg` 已安装后，需要在 `config.yaml` 中开启 `download.localServer.enabled` 设置为 `true`

最后，在「视频详情页」的「更多」菜单中，有「下载为mp3」菜单，点击即可下载 `mp3` 文件


---
title: 下载
---

# 下载

<br />

<ClientOnly>
<div class="mt-8">
<EnvInfo />
</div>
</ClientOnly>

下载对应平台的构建包：<https://github.com/ltaoo/wx_channels_download/releases>

<!-- <ClientOnly>
<DownloadButton />
</ClientOnly> -->

## 如何选择构建包

根据上述环境信息，选择对应的构建包

1. macOS arm64
选择 darwin_arm64 后缀

2. macOS x86_64
选择 darwin_x86_64 后缀

3. Windows x86_64
选择 windows_x86_64 后缀

> windows 平台有带 `safe` 标记的文件，表示「没有使用UPX压缩来减小体积」，在某些电脑上，可以避免被识别为病毒

## 运行下载器

在 `Windows` 平台，解压后双击直接运行 `wx_video_download.exe` 即可，首次使用会自动安装证书并设置系统代理。

`macOS` 平台请参考 [macOS 运行](./macos.md)

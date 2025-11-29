---
title: 常见问题 (FAQ)
---

# 常见问题 (FAQ)

以下问题根据发布记录与社区反馈整理，更多问题请前往：<https://github.com/ltaoo/wx_channels_download/issues>

## 下载按钮不显示或点击无效

- 确认已启动下载器，且页面已开启注入（默认会设置系统代理）。
- 若使用 Clash，建议关闭系统代理：在 `config.yaml` 将 `proxy.system` 设为 `false`，并将流量转发到下载器的代理服务（参考发布页中的 Global Extend Script 示例）。
- 升级至最新版本并刷新页面后重试。

## 下载后无法播放

- 请升级到最新版本。
- 如视频加密，需要在下载时或后续使用 `decrypt` 命令进行解密。
- 终端打印的下载命令是 `ffmpeg`，请确保本机已安装 `ffmpeg` 并保持版本较新。

## 终端提示需要 ffmpeg

- macOS 可通过 Homebrew 安装：`brew install ffmpeg`
- Windows 可通过 Scoop 或自带安装包安装：`scoop install ffmpeg` 或下载官方包并配置到 PATH。

## 科学上网失效/网络受影响

- 默认启动会设置系统代理，这可能影响其他代理软件。
- 可在 `config.yaml` 将 `proxy.system` 设为 `false`，保持系统代理不变，通过 Clash 将流量转发到下载器代理端口（默认 `2023`）。

## 为什么会申请管理员权限

- 当需要设置系统代理且以双击方式运行时，程序会按需申请管理员权限；在终端运行或不需要设置系统代理时不会申请。

## 如何默认下载原始视频（最高画质）

- 在 `config.yaml` 设置：

```yaml
download:
  defaultHighest: true
```

并重新启动下载器。

## 如何卸载安装的根证书

- 在终端执行：

```sh
wx_video_download uninstall
```

将尝试取消代理并删除初始化安装的根证书。

## 使用命令行下载/解密的示例

```sh
# 下载（示例参数）
wx_video_download download --url "视频URL" --filename "文件名.mp4" --key 123456

# 解密
wx_video_download decrypt --filepath "/绝对路径/文件名.mp4" --key 123456
```

下载器会先将临时文件保存到 `Downloads` 目录，再按需解密输出目标文件，并删除临时文件。

## Windows 下载按钮注入说明

- Windows 平台通过设置系统代理注入下载按钮，可能导致科学上网失效；可改用 Clash 转发方案以避免影响。

## 获取最新版本

- 访问发布页：<https://github.com/ltaoo/wx_channels_download/releases>
- 文档站首页提供“下载最新版本”按钮，可自动为当前系统匹配合适构建。

## 热门问题（来自 Issues）

<ClientOnly>
<LatestIssues />
</ClientOnly>

---
title: Releases
---

# Releases

最新发布页：<https://github.com/ltaoo/wx_channels_download/releases>

## v251122

- 支持通过 `config.yaml` 配置默认是否下载原始视频
- 新增代理配置项，可指定是否设置系统代理
- 支持自动检测并按需申请管理员权限

示例配置：

```yaml
download:
  defaultHighest: false

proxy:
  system: true
  port: 2023
```

如需默认下载原始视频，将压缩包内 `config.yaml` 的 `defaultHighest` 改为 `true` 后重启下载器。

如需配合 Clash 使用，可将 `system` 改为 `false`，并在 Clash 配置中添加 `Global Extend Script`，转发到下载器的代理服务。

## v251027

- 内置 `echo` 以替代 SunnyNet，二进制体积缩减到约 8M
- `download` 命令默认启用多线程
- 在直播详情页增加下载按钮，并在终端打印下载命令（`ffmpeg`）
- Windows 平台通过设置系统代理注入下载按钮，科学上网可能失效

> 提示：使用终端下载方式需要安装 `ffmpeg`。

## v250913

- 视频号首页视频操作栏增加「下载」按钮
- 增加 `uninstall` 命令，可卸载根证书
- 增加 `decrypt` 命令，支持使用指定 key 对已下载视频解密

典型命令：

```sh
wx_video_download_xx uninstall
wx_video_download_xx decrypt --filepath <绝对路径> --key <解密key>
```

## v250621

- 新增 `download` 命令，将视频下载到 `Downloads` 并解密
- 视频号「更多」菜单新增「打印下载命令」按钮
- Windows 平台双击以管理员身份运行，二进制文件增加图标

示例：

```sh
./wx_video_download_xx download --url "视频地址" --key 解密key --filename "文件名"
```

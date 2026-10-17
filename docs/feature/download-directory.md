---
title: 设置下载目录
---

# 设置下载目录

通过 `config.yaml` 中的 `download.dir` 设置下载文件存放的目录，默认为用户下载目录：

```yaml
download:
  dir: "%UserDownloads%"
```

可以将其修改为需要的绝对路径目录，保存配置后重启下载器。

## Windows 示例

```yaml
download:
  dir: 'D:\Downloads\wx_channels'
```

Windows 路径建议使用单引号，避免反斜杠被作为转义字符处理。

## macOS / Linux 示例

```yaml
download:
  dir: "/data/downloads/wx_channels"
```

请替换为自己电脑上可写入的绝对路径。如果已有 `download:` 配置，请在同一个配置块中修改或添加 `dir`。

## 生效范围

此配置仅在后台下载时生效。使用前端下载时，文件保存位置由浏览器的下载设置决定。

其他下载配置见[配置 → 下载](../config/download.md#下载目录)。如需按作者等信息创建子目录，请参考[指定文件名](./filename.md)。

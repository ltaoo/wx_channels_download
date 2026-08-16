---
title: 自定义根证书
---

# 自定义根证书

用于配置自定义证书与私钥。当证书和私钥文件均已配置且可读时，程序会优先使用自定义证书。

## 配置键

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `cert.file` | `""` | 自定义证书文件的绝对路径，支持 `.pem`、`.cer`、`.crt`、`.key` |
| `cert.key` | `""` | 自定义私钥文件的绝对路径，支持 `.pem`、`.key` |
| `cert.name` | `"Echo"` | 证书名称，用于在系统证书库中识别、安装和卸载证书 |

## 示例（config.yaml）

```yaml
cert:
  file: "C:/path/to/mycert.pem"
  key: "C:/path/to/mykey.pem"
  name: "MyProxyCA"
```

## 生效规则

- `cert.file` 和 `cert.key` 必须同时配置，并且两个文件都可读，才会使用自定义证书。
- 未配置 `cert.name` 时使用默认值 `Echo`。
- 自定义证书不可用时，程序会尝试使用本机已有的 mitmproxy 证书；仍不可用时使用内置 SunnyNet 证书。
- `cert.file` 和 `cert.key` 应填写绝对路径。

## 注意事项

- 证书与私钥必须属于同一密钥对。
- `cert.name` 应与证书的通用名称（CN）一致，否则可能无法正确检测或卸载已安装的证书。
- Windows 安装或卸载证书需要管理员权限，具体操作参考[手动安装根证书](../guide/certificate.md)。

---
title: 自定义证书
---

# 自定义证书

用于配置自定义证书与私钥。当同时提供证书与私钥文件且可读时，程序将优先使用用户证书；否则回退到内置证书（SunnyNet）。

## 配置键

- cert.file：用户证书文件路径（建议 PEM，亦可为常见 .cer/.crt）
- cert.key：用户私钥文件路径（建议 PEM）
- cert.name：证书名称（CN），用于系统证书库识别与安装；留空则使用 "Echo"

## 示例（config.yaml）

```yaml
cert:
  file: "C:/path/to/mycert.pem"
  keyFile: "C:/path/to/mykey.pem"
  name: "MyProxyCA"
```

## 生效规则

- 同时配置且文件存在时，使用用户证书与私钥；否则使用内置证书
- 未设置 cert.name 时，沿用默认名称（SunnyNet）
- 路径建议使用绝对路径；相对路径请确保与程序工作目录一致

## 注意事项

- 证书与私钥必须匹配（同一对密钥/证书）
- 证书名称用于检测是否已安装并在系统证书库中识别、卸载
- Windows 安装/卸载证书涉及管理员权限；详见“根证书指南”


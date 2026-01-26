# 安装 SunnyRoot 根证书指南

本文介绍如何在常见操作系统中安装根证书 SunnyRoot（证书文件位置：`c:\Users\litao\Documents\wx_channels_download\pkg\certificate\certs\SunnyRoot.cer`）。安装根证书会影响系统的全局信任链，请仅在充分信任的前提下进行。

## Windows

- 图形界面（MMC）
  - Win+R 输入 `mmc` 回车
  - 文件 → “添加/删除管理单元” → 选择“证书” → 点击“添加”
  - 选择“计算机帐户” → “本地计算机” → 完成 → 确定
  - 展开“证书（本地计算机）” → “受信任的根证书颁发机构” → “证书”
  - 右键空白处选择“所有任务” → “导入…”
  - 选择 `SunnyRoot.cer`，一路下一步完成（需要管理员权限）

- PowerShell（管理员）

```powershell
$cerPath = 'c:\Users\litao\Documents\wx_channels_download\pkg\certificate\certs\SunnyRoot.cer'
Import-Certificate -FilePath $cerPath -CertStoreLocation Cert:\LocalMachine\Root
```

- PowerShell（仅当前用户）

```powershell
$cerPath = 'c:\Users\litao\Documents\wx_channels_download\pkg\certificate\certs\SunnyRoot.cer'
Import-Certificate -FilePath $cerPath -CertStoreLocation Cert:\CurrentUser\Root
```

- 命令行（certutil）

```powershell
certutil -addstore -f root "c:\Users\litao\Documents\wx_channels_download\pkg\certificate\certs\SunnyRoot.cer"
```

### 验证（Windows）

```powershell
Get-ChildItem Cert:\LocalMachine\Root | Where-Object { $_.Subject -like '*SunnyNet*' } | Format-Table Subject, Thumbprint, NotAfter
```

或：

```powershell
certutil -store root SunnyRoot
```

## macOS

- 图形界面
  - 双击 `SunnyRoot.cer` 打开“钥匙串访问”
  - 在左上角选择“系统”钥匙串，将证书添加到系统钥匙串
  - 双击证书，展开“信任”，将“使用此证书时”设为“始终信任”，关闭窗口后输入管理员密码保存

- 终端

```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /path/to/SunnyRoot.cer
```

将 `/path/to/SunnyRoot.cer` 替换为实际文件路径。

### 验证（macOS）

```bash
security find-certificate -a -c "SunnyNet" /Library/Keychains/System.keychain
```

## Linux

- Debian/Ubuntu

```bash
sudo cp /path/to/SunnyRoot.cer /usr/local/share/ca-certificates/SunnyRoot.crt
sudo update-ca-certificates
```

- CentOS/RHEL/Fedora

```bash
sudo cp /path/to/SunnyRoot.cer /etc/pki/ca-trust/source/anchors/SunnyRoot.crt
sudo update-ca-trust
```

说明：上述路径要求扩展名为 `.crt`（PEM 编码内容不变，重命名即可）。

### 验证（Linux）

```bash
openssl x509 -in /usr/local/share/ca-certificates/SunnyRoot.crt -text -noout | grep Subject
```

或根据实际发行版存放路径调整。

## 卸载/移除

- Windows（管理员）

```powershell
$cert = Get-ChildItem Cert:\LocalMachine\Root | Where-Object { $_.Subject -like '*SunnyNet*' }
if ($cert) { Remove-Item -Path $cert.PSPath }
```

- macOS
  - 在“钥匙串访问”中找到证书，右键“删除”；或使用 `security delete-certificate` 搭配证书指纹

- Linux
  - 删除已复制的 `.crt` 文件，并执行 `update-ca-certificates` 或 `update-ca-trust`

## 注意事项

- 需要管理员权限的步骤请在具备管理员权限的终端或界面执行
- 安装根证书会影响系统及浏览器、开发工具（Git、Node、Curl 等）的信任链
- 仅安装来自可信来源的根证书；证书主题包含“SunnyNet”可用于识别


---
title: Windows 服务无法访问
---

# Windows 服务无法访问

## 问题描述

Windows 电脑可以启动程序，但通过浏览器访问 `http://127.0.0.1:2023` 或 `http://127.0.0.1:2022` 时超时无响应。

## 快速诊断

如果你看到 `netstat` 输出中有 `SYN_SENT` 状态的连接：

```
TCP 127.0.0.1:8115 127.0.0.1:2023 SYN_SENT 15616
```

**这个输出的含义：**
- `127.0.0.1:8115` - 客户端地址和临时端口（源）
- `127.0.0.1:2023` - 服务端地址和端口（目标）
- `SYN_SENT` - TCP 连接状态（正在尝试建立连接）
- `15616` - 客户端进程 PID

其中 8115 是操作系统自动分配的临时端口，用于标识这个连接。你可以通过以下命令查看是哪个程序在尝试连接：

```powershell
tasklist | findstr "15616"
```

**问题诊断：**

`SYN_SENT` 状态说明**服务正常运行，但连接被阻止了**。客户端发送了 SYN 包，但没有收到服务端的 SYN-ACK 响应。

最可能的原因是：
1. **Windows 防火墙阻止**（最常见）
2. **安全软件拦截**（360、腾讯管家等）
3. **网络驱动异常**

**立即尝试：** 临时关闭 Windows 防火墙和安全软件测试。

## 排查步骤

### 1. 确认服务已启动

启动程序后，查看控制台输出是否显示：
- `API服务启动成功, 地址: 127.0.0.1:2022`
- `代理服务启动成功`

### 2. 检查端口占用

打开 PowerShell 或命令提示符，执行：

```powershell
netstat -ano | findstr "2023"
netstat -ano | findstr "2022"
```

应该能看到类似输出：
```
TCP    127.0.0.1:2023         0.0.0.0:0              LISTENING       12345
TCP    127.0.0.1:2022         0.0.0.0:0              LISTENING       12345
```

如果没有输出，说明服务未正常监听。

### 3. 测试本地连接

使用 `curl` 或 PowerShell 测试：

```powershell
curl http://127.0.0.1:2022/health
```

或

```powershell
Invoke-WebRequest -Uri http://127.0.0.1:2022/health
```

如果返回错误，继续下一步排查。

### 4. 检查 Windows 防火墙（重点）

**这是最常见的原因！** 即使是本地回环地址 `127.0.0.1`，Windows 防火墙也可能阻止连接。

如果 `netstat` 显示连接状态为 `SYN_SENT`，说明防火墙正在阻止连接。

**解决方案 A：临时关闭防火墙测试**

1. 打开"Windows 安全中心"
2. 点击"防火墙和网络保护"
3. 临时关闭"专用网络"和"公用网络"防火墙
4. 重新测试访问

**如果关闭防火墙后可以访问，说明确实是防火墙问题。**

**解决方案 B：添加防火墙规则（推荐）**

以管理员身份运行 PowerShell 或命令提示符，执行：

```powershell
netsh advfirewall firewall add rule name="wx_video_download_api" dir=in action=allow protocol=TCP localport=2022
netsh advfirewall firewall add rule name="wx_video_download_proxy" dir=in action=allow protocol=TCP localport=2023
```

**解决方案 C：允许程序通过防火墙**

1. 打开"Windows 安全中心" → "防火墙和网络保护"
2. 点击"允许应用通过防火墙"
3. 点击"更改设置" → "允许其他应用"
4. 浏览并选择 `wx_video_download.exe`
5. 确保"专用"和"公用"都勾选
6. 点击"添加"

**解决方案 D：使用 Windows Defender 防火墙高级设置**

1. 按 `Win + R`，输入 `wf.msc`，回车
2. 点击左侧"入站规则"
3. 点击右侧"新建规则"
4. 选择"程序" → 下一步
5. 浏览选择 `wx_video_download.exe` → 下一步
6. 选择"允许连接" → 下一步
7. 全部勾选（域、专用、公用）→ 下一步
8. 输入名称"wx_video_download" → 完成

### 5. 检查 IPv4/IPv6 配置

某些系统可能存在 IPv4/IPv6 配置问题。尝试使用不同的地址访问：

- `http://127.0.0.1:2022` (IPv4)
- `http://[::1]:2022` (IPv6)
- `http://localhost:2022` (系统解析)

### 6. 检查代理设置

如果系统设置了其他代理，可能导致访问本地服务时出现问题。

**临时禁用系统代理：**
1. 打开"设置" → "网络和 Internet" → "代理"
2. 关闭"使用代理服务器"
3. 重新测试

### 7. 检查安全软件（重点）

**360、腾讯电脑管家、火绒等安全软件可能拦截本地网络连接。**

如果你安装了以下软件，它们可能是问题根源：
- 360 安全卫士 / 360 杀毒
- 腾讯电脑管家
- 火绒安全软件
- 金山毒霸
- 瑞星杀毒
- McAfee、Norton 等国外杀毒软件

**解决方案 A：添加到信任列表**

以 360 为例：
1. 打开 360 安全卫士
2. 点击"木马防火墙"或"防护中心"
3. 找到"信任与阻止"或"白名单"
4. 添加 `wx_video_download.exe` 到信任列表

以腾讯电脑管家为例：
1. 打开腾讯电脑管家
2. 点击"病毒查杀" → "信任区"
3. 添加 `wx_video_download.exe`

**解决方案 B：临时关闭测试**

临时退出或关闭安全软件，重新测试访问。如果可以访问，说明确实是安全软件问题。

**解决方案 C：关闭网络防护功能**

某些安全软件有"网络防护"或"流量监控"功能，可能拦截本地连接：
- 360：关闭"网络防护"
- 腾讯管家：关闭"网络防护"
- 火绒：关闭"网络防护"

### 8. 使用诊断工具

使用 `telnet` 测试端口连通性：

```powershell
# 如果没有 telnet，先启用
dism /online /Enable-Feature /FeatureName:TelnetClient

# 测试连接
telnet 127.0.0.1 2022
telnet 127.0.0.1 2023
```

如果连接成功，会显示空白屏幕（按 Ctrl+] 然后输入 quit 退出）。

## 常见解决方案

### 方案 1：以管理员身份运行（推荐）

右键点击 `wx_video_download.exe`，选择"以管理员身份运行"。

管理员权限可以避免某些防火墙和安全软件的拦截。

### 方案 2：添加防火墙规则（推荐）

如果确认是防火墙问题，添加规则后就可以正常使用：

```powershell
# 以管理员身份运行
netsh advfirewall firewall add rule name="wx_video_download_api" dir=in action=allow protocol=TCP localport=2022
netsh advfirewall firewall add rule name="wx_video_download_proxy" dir=in action=allow protocol=TCP localport=2023
```

### 方案 3：将程序添加到安全软件白名单

如果使用了 360、腾讯管家等安全软件，将 `wx_video_download.exe` 添加到信任列表。

### 方案 4：修改监听地址（不推荐，仅测试用）

编辑 `config.yaml`，尝试修改监听地址：

```yaml
api:
  hostname: "0.0.0.0"  # 监听所有网络接口
  port: 2022

proxy:
  hostname: "0.0.0.0"
  port: 2023
```

**注意：** 这会让服务在所有网络接口上监听，局域网内其他设备也能访问，存在安全风险。仅用于测试排查。

### 方案 5：更换端口

如果特定端口被限制，尝试更换端口：

```yaml
api:
  hostname: "127.0.0.1"
  port: 8022  # 更换为其他端口

proxy:
  hostname: "127.0.0.1"
  port: 8023
```

### 方案 6：检查 hosts 文件

确认 `C:\Windows\System32\drivers\etc\hosts` 文件中 `127.0.0.1` 的配置正确：

```
127.0.0.1       localhost
```

### 方案 7：重置网络栈（高级）

如果网络栈出现异常，可以尝试重置。以管理员身份运行：

```powershell
netsh winsock reset
netsh int ip reset
ipconfig /flushdns
```

执行后需要重启电脑。

## 仍然无法解决？

如果以上方法都无效，请提供以下信息到 GitHub Issues：

1. Windows 版本（运行 `winver` 查看）
2. 程序启动时的完整控制台输出
3. `netstat -ano | findstr "2022"` 和 `netstat -ano | findstr "2023"` 的输出
4. 是否安装了安全软件或防火墙
5. `config.yaml` 的配置内容

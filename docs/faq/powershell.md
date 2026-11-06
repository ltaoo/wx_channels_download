---
title: PowerShell
---

# PowerShell

Windows 启动失败时，先根据终端中的具体报错选择对应的处理方式。

| 报错或现象 | 常见原因 | 处理方式 |
| --- | --- | --- |
| `powershell.exe: executable file not found in %PATH%` | 找不到 Windows PowerShell，或 `Path` 缺少程序目录 | 检查程序是否存在并补充环境变量 |
| `Start-Process` / `ParameterAlreadyBound` / `ArgumentList` | 旧版设置系统代理时，提权命令重复传入参数 | 升级下载器；必要时手动配置代理 |
| Win7 安装证书失败或启动后闪退 | 旧版命令与系统自带 PowerShell 不兼容，也可能是程序本身不支持该系统 | 查看 PowerShell 版本、下载器版本及完整日志 |

## 找不到 powershell.exe

典型报错：

```text
ERROR 获取证书时发生错误，exec: "powershell.exe": executable file not found in %PATH%
```

这表示程序无法启动 PowerShell，还未执行到证书查询步骤；应先检查程序路径。

1. 在开始菜单搜索并打开 **Windows PowerShell**，确认可以正常启动。
2. 在文件资源管理器地址栏输入 `%SYSTEMROOT%\System32\WindowsPowerShell\v1.0\`，确认其中有 `powershell.exe`。
3. 打开“编辑系统环境变量 → 环境变量”，检查 `Path` 中是否包含下面的目录；如果没有，新增一条，保留其他已有条目：

   ```text
   %SYSTEMROOT%\System32\WindowsPowerShell\v1.0\
   ```

4. 关闭原有终端和下载器，重新打开终端，执行：

   ```powershell
   where.exe powershell.exe
   ```

   应输出 `powershell.exe` 的完整路径，再重新启动下载器。如果从未退出的启动器中运行，先退出并重新打开启动器，让它读取更新后的环境变量。

如果目录中没有 `powershell.exe`，应先恢复系统自带的 Windows PowerShell；仅添加不存在的目录无效。PowerShell 7 的 `pwsh.exe` 与下载器调用的 `powershell.exe` 名称不同，仅安装 PowerShell 7 不一定能解决这个报错，不建议通过改名或复制到系统目录来替代。

参考：[Issue #42](https://github.com/ltaoo/wx_channels_download/issues/42)、[Issue #232](https://github.com/ltaoo/wx_channels_download/issues/232)。#42 的提问者确认补充 `Path` 后可以使用。

## 设置系统代理失败，提示 ArgumentList 重复

典型报错：

```text
failed to configure proxy: 设置系统代理时发生错误
提权执行失败: Start-Process : Cannot bind parameter because parameter 'ArgumentList' is specified more than once.
FullyQualifiedErrorId : ParameterAlreadyBound,Microsoft.PowerShell.Commands.StartProcessCommand
```

旧版下载器在直接修改系统代理失败后，会通过 PowerShell 请求提权；命令重复传入 `-ArgumentList`，导致启动失败。这与找不到 `powershell.exe` 是不同的问题，修改 `Path` 无法解决。

先[升级下载器](../guide/start.md)。当前源码已将参数合并为一次 `-ArgumentList`，但不能仅根据 Issue 是否关闭判断所用构建包是否包含修复。

如果暂时无法升级，或仍无法自动设置系统代理，可以按作者在 #554 中提供的方式临时处理：

1. 退出下载器，在启动信息显示的 `Config File` 对应的 `config.yaml` 中，将已有 `proxy` 下的 `system` 改为 `false`，不要重复添加 `proxy` 节点：

   ```yaml
   proxy:
     system: false
   ```

2. 保存并重新启动下载器，确认代理服务已启动。
3. 在 Windows“设置 → 网络和 Internet → 代理 → 手动设置代理”中，启用代理服务器，地址填 `127.0.0.1`，端口填 `2023`。如果修改过 `proxy.hostname` / `proxy.port`，请使用实际配置的地址和端口。
4. 重新打开微信视频号页面，确认能正常加载并出现下载按钮。

手动代理会影响使用系统代理的其他应用。下载器退出后，需要手动关闭该代理或恢复原有配置，否则可能无法访问网络。使用 Clash 等代理软件时，参考[与 Clash 协同](../config/proxy.md#与-clash-协同)，避免相互覆盖。

`proxy.system: false` 只是不让下载器修改系统代理，不会跳过证书检查。若仍报证书错误，按[手动安装根证书](../guide/certificate.md)继续排查。

参考：[Issue #554](https://github.com/ltaoo/wx_channels_download/issues/554)，作者确认是权限相关 Bug 并给出了上述临时方案；[Issue #493](https://github.com/ltaoo/wx_channels_download/issues/493) 的评论中也有相同报错。

## Win7 的 PowerShell 兼容问题

如果在 Win7 上安装证书失败或程序闪退，先从 Windows PowerShell 中查看版本：

```powershell
$PSVersionTable.PSVersion
```

Issue #283 报告了旧版命令不兼容低版本 PowerShell 的问题，作者建议尝试 v260208；[v260208 发布日志](../releases/260208.md)也记录了“修复了在 win7 无法安装证书的问题”。

后续 Issue #288 还涉及 Go 工具链版本兼容；作者在 v260310 中将 Go 降至 1.20，提问者最终确认 Win7 64 位可以运行和下载。这些记录对应当时的版本，不能据此判断后续所有版本都能在 Win7 运行。若没有 PowerShell 报错而是直接闪退，应在终端中运行下载器保留输出，并检查所用构建包是否支持该系统。

参考：[Issue #283](https://github.com/ltaoo/wx_channels_download/issues/283)、[Issue #288](https://github.com/ltaoo/wx_channels_download/issues/288)。

## 仍然无法解决

提交反馈时，请附上 Windows 版本、下载器版本、Windows PowerShell 版本、`where.exe powershell.exe` 的输出，以及从启动到失败的完整终端日志。若修改过代理配置，也请说明 `proxy.system`、代理地址与端口、是否同时运行其他代理软件；分享配置和日志前请遮盖 Cookie、Token 等敏感信息。

# 关闭视频号 LiteApp 接管开关

将各微信账号、各数字版本的引擎配置中 `featureSwitch.handleFinder` 更新为 `false`。修改时在配置文件旁生成 `engineConfig.json.bak.*` 备份；已为 `false` 则跳过。运行后需完全退出并重新打开微信。

## macOS

在项目根目录执行：

```sh
sh scripts/disable-wechat-liteapp-finder.sh
```

默认搜索当前用户微信容器内的 `app_data/radium/users`；使用系统自带的 `plutil`。

## Windows

在项目根目录的 PowerShell 或 CMD 中执行：

```bat
.\scripts\disable-wechat-liteapp-finder.bat
```

BAT 内嵌 PowerShell 代码，可单独复制使用，不需要额外的 `.ps1` 文件。使用 Windows PowerShell 5.1 的命令与 API，默认搜索 `%APPDATA%`、`%LOCALAPPDATA%` 下的 `Tencent\xwechat\radium\users` 和 `Tencent\WeChat\radium\users`。

非默认位置可显式指定 `radium\users` 目录：

```bat
.\scripts\disable-wechat-liteapp-finder.bat "D:\WeChat Data\radium\users"
```

脚本只处理目标引擎包 `ilinkres_uzhblidb` 下的配置。非法 JSON、缺失字段、错误值类型或无法唯一定位的 `handleFinder=true` 会报错并返回非零退出码；已处理的其他文件保留修改及备份。Windows 版保留其他 JSON 文本，使用 [.NET File.Replace](https://learn.microsoft.com/en-us/dotnet/api/system.io.file.replace?view=netframework-4.8) 同时替换文件与保留备份。

## 自检

自检只操作临时配置：

```sh
sh scripts/test-disable-wechat-liteapp-finder.sh
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\test-disable-wechat-liteapp-finder.ps1
```

Windows 版内嵌逻辑已在 macOS PowerShell 7.7.0 上验证自动查找、多账号、多版本、含中文和特殊字符的路径、备份、其他 JSON 文本保留、重复运行及非法输入。尚未在 Windows PowerShell 5.1、CMD 或真实 Windows 微信环境运行验证。

微信更新可能覆盖配置；配置修改成功不等于已经验证视频卡片会回退到 H5。

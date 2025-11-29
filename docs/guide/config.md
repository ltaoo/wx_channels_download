---
title: 配置说明
---

# 配置说明

应用支持通过 `config.yaml` 调整行为。若没有此文件将按默认值运行。

## 下载相关

```yaml
download:
  defaultHighest: false
  filenameTemplate: "{{filename}}-{{spec}}"
```

- `defaultHighest` 是否默认下载最高画质
- `filenameTemplate` 下载文件名模板

## 代理相关

```yaml
proxy:
  system: true
  hostname: 127.0.0.1
  port: 2023
```

- `system` 是否设置系统代理
- `hostname` 代理主机名
- `port` 代理端口

如需与 Clash 协同，可关闭系统代理并在 Clash 中将全局或特定规则转发到上述端口。

## 调试相关

```yaml
debug:
  protocol: https
  api: debug.weixin.qq.com
```

- `protocol` 调试服务协议
- `api` 调试服务地址

## 其他设置

```yaml
globalUserScript: ""
inject:
  extraScript:
    afterJSMain: ""
```

- `globalUserScript` 全局用户脚本内容（也可通过项目根目录的 `global.js` 自动注入）
- `inject.extraScript.afterJSMain` 额外注入脚本路径或内容

若配置为文件路径，将在启动时读取并以内容形式注入。


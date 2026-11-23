---
title: API 服务
---

# API 服务

现在启动后默认会开启 API 服务，下载器默认会通过 API 服务提交下载任务，由 API 服务在后台进行下载。通过 `config.yaml` 可以配置服务地址。

```yaml
api:
  protocol: http
  hostname: 127.0.0.1
  port: 2022
  username: ""
  password: ""
```

- `protocol` API 服务协议，默认 `http`
- `hostname` API 服务监听地址。设为 `0.0.0.0` 时监听所有 IPv4 接口；本机前端和其他本机客户端仍通过 `127.0.0.1` 连接
- `port` API 服务端口
- `username` 可选的 HTTP Basic Auth 用户名，默认不配置
- `password` 可选的 HTTP Basic Auth 密码，默认不配置；用户名和密码均非空时启用认证，重启后生效

## 远程访问与认证

`api.username` 和 `api.password` 均为可选配置。两项均非空时才启用 HTTP Basic Auth；未配置或仅配置一项时，本机和远程访问都无需提供用户名和密码。启用认证后，浏览器通过 HTTP Basic Auth 登录，API 客户端发送相同认证信息。Basic Auth 本身不加密密码，可通过 HTTPS 反向代理或可信隧道连接。

本机默认通过 `http://127.0.0.1:2022` 使用，未配置认证时无需登录。API 会拒绝不可信网页的跨站请求；视频号、公众号的注入页面仍可连接。本应用的文件浏览功能可以读取当前进程有权限访问的文件，因此只应向可信用户开放。

直接监听远程连接时设置 `api.hostname: "0.0.0.0"`。启用认证的同机 HTTPS 反向代理可保留默认回环监听；代理应保留公开请求 Host，并设置 `X-Forwarded-Proto: https`。未启用认证且通过 HTTPS 反向代理访问时，设置 `api.hostname: "0.0.0.0"` 和 `api.protocol: "https"`，并保留公开请求 Host，确保来源校验使用公开地址。

## 客户端与同步认证

微信客户端转发到远程下载服务时，仅当远程服务启用认证，才需要在本机的 `download.remoteServer.username`、`download.remoteServer.password` 中填写账号密码。转发配置见[下载配置](./download.md#远程服务)。

同步目标无需认证时，直接填写 HTTP(S) 地址即可。目标启用认证时，URL 支持标准 Basic Auth 用户信息，例如 `https://download-user:password@downloads.example.com`；包含特殊字符的用户名和密码需要 URL 编码。

内置 MCP 服务以及 `mcp`、`download` 命令默认读取相同的 API 认证配置；显式指定 API 地址时，可在 URL 中携带认证信息。

## 图片代理与本地文件

图片代理只提供安全的位图格式，拒绝 HTML 和 SVG。本地文件以沙箱文档方式打开，HTML 文件中的脚本不会获得应用 API 的同源权限。

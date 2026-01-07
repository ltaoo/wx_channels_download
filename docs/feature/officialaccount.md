---
title: 公众号
---

# 公众号

和视频号相同的注入方案，可以获取指定公众号的推送消息列表，并提供 `API` 调用

- 获取指定公众号的推送消息列表
- RSS

需要打开某个公众号文章页面，并且不能关闭，否则调用接口时会提示 `{"code":400,"msg":"请先初始化客户端 socket 连接"}`。当出现该提示时，打开任意公众号文章页面即可，如果已经打开，刷新一下即可。

## 获取指定公众号消息列表

```bash
curl http://localhost:2022/api/official_account/msg/list?biz=MzI2NDk5NzA0Mw==
```

通过 `offset` 指定偏移量，可以获取到更多的消息记录

```bash
curl http://localhost:2022/api/official_account/msg/list?biz=MzI2NDk5NzA0Mw==&offset=10
```

## 获取可请求的公众号列表

```bash
curl http://localhost:2022/api/official_account/list
```

返回公众号昵称、头像和「授权凭证」。授权凭证用来请求「推送消息列表」，凭证有效期大概是半小时，需要定时刷新凭证

> 不一定可请求，凭证如果过期，不会从列表移除

## 公众号 RSS

```bash
curl http://localhost:2022/rss/mp?biz=MzI2NDk5NzA0Mw==
```

同样可以传入 `offset` 获取到更多消息记录。

还提供 `proxy` 和 `content` 参数，分别用于代理公众号内容和默认获取公众号文章全文

### proxy

如果阅读器提供获取全文能力，但是无法正确获取到公众号文章正文，可以指定 `proxy=1`，那么返回文章列表中，文章链接都会添加 `{ServiceAddr}/official_account/proxy` 前缀，当打开文章时，将使用代理代为请求微信公众号返回正文内容

### content

如果希望直接获取到正文，可以指定 `content=1`，那么请求 `RSS` 接口时，就会同时获取正文。但是缺点就是列表接口会比较慢（因为要依次请求到正文）


## 远端服务模式

本下载器依赖微信应用本体，导致无法在 `linux` 服务器上部署。所以额外增加「远端服务」模式，同时配合 本地+远端，可以在 `linux` 服务器上部署 `API` 和 `RSS` 服务

**仍需要 macOS 或 Windows 机器，用于同步「公众号授权凭证」到远端服务**


### 部署说明

下载构建包到 `linux` 服务器，修改配置文件

```yaml
# config.yaml
api:
  protocol: "http"
  hostname: "127.0.0.1"
  port: 2022
mp:
  remoteServer:
    protocol: "https"
    hostname: "rss.example.com"
    port: 80
  refreshToken: "123"
  tokenFilepath: ""
```

其中 `refreshToken` 用于「本地」向「远端服务」提交「公众号授权凭证」时的校验，可以避免被无授权的人修改公众号授权凭证，导致服务不可用

`tokenFilepath` 指定调用 `API` 或 `RSS` 时的授权凭证文件，可以留空。或者指定一个 `./token.txt`，那么会读取该文件，每行作为一个 `token`，调用接口时必须传入 `&token=`，否则会拒绝访问。

使用 `chmod +x ./wx_video_download` 给二进制文件添加权限，然后运行命令 `./wx_video_download mp -d` 启动，该命令仅运行公众号相关的功能，且作为守护进程运行

运行成功之后，调用「获取可请求的公众号列表」会返回空列表。调用「获取指定公众号消息列表」会提示 `Please adding Credentials first`

表示「远端服务」部署成功，接下来运行「本地」服务

在任意 `macOS` 或 `Windows` 机器上，下载对应平台构建包，修改配置文件

```yaml
# config.yaml
mp:
  remoteServer:
    protocol: "https"
    hostname: "rss.example.com"
    port: 80
  refreshToken: "123"
  tokenFilepath: ""
```

内容和「远端服务」的配置文件保持相同。然后，通过命令 `./wx_video_download` 启动，**打开任意公众号文章页面**，会自动向「远端服务」提交该公众号的授权凭证

同时，在打开的公众号文章页面，账号名称旁会出现 RSS 图标，点击会复制 `RSS` 订阅链接

![RSS按钮](../assets/official_account_rss.png)

内容类似 `https://rss.example.com/rss/mp?biz=MzI2NDk5NzA0Mw==`，可以在浏览器打开该链接，验证服务是否正常

调用「获取可请求的公众号列表」，也可以看到刚刚打开的公众号信息

到此，公众号RSS 服务就可以正常使用了。本地 `macOS` 或 `Windows` 上的服务不要关闭，会定时获取在列表中的公众号，刷新凭证，保持「远端服务」的 `RSS` 服务一直可用



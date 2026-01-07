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

通过 `offset` 获取到指定页的消息列表

```bash
curl http://localhost:2022/api/official_account/msg/list?biz=MzI2NDk5NzA0Mw==&offset=10
```

## 获取可请求的公众号列表

```bash
curl http://localhost:2022/api/official_account/list
```

> 不一定可请求，凭证可能过期，但不会从列表移除

## 生成指定公众号的 RSS 链接

```bash
curl http://localhost:2022/rss/mp?biz=MzI2NDk5NzA0Mw==
```

同样可以传入 `offset` 获取到更多消息记录。

还提供 `proxy` 和 `content` 参数，分别用于代理公众号内容和默认获取公众号文章全文

### proxy

如果阅读器提供获取全文能力，但是无法正确获取到内容，可以指定 `proxy=1`，那么返回的 `XML` 内容中的所有文章列表，都会添加 `/official_account/proxy` 前缀，当打开文章时，其实是打开我们自己服务，再由我们自己服务请求微信公众号，返回正文内容


### content

如果希望直接获取到正文，可以指定 `proxy=1`，那么请求订阅接口时，就会同时获取正文。但是缺点就是列表接口会比较慢（因为要依次请求到正文）


## 远端服务模式

由于大部分服务器都是 `Linux`，本下载器依赖微信应用本体，导致无法使用。所以增加远端服务模式，可以在 `Linux` 服务器上部署该服务，提供 API 和 RSS 服务


**但是仍需要 macOS 或 Windows 机器同步「公众号授权凭证」到远端服务**


### 部署说明

下载构建包到 `Linux` 服务，修改配置文件

```yaml
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

其中 `refreshToken` 指定外部向运行在 `Linux` 上的服务提交「公众号授权凭证」时的校验信息（避免被外部恶意修改内容）

`tokenFilepath` 指定调用 `API` 或 `RSS` 时的授权凭证文件，可以留空。或者指定一个 `./token.txt`，那么会读取该文件，每行作为一个 `token`，调用接口时必须传入 `&token=aaa`，否则会拒绝访问。

添加权限后，使用命令 `./wx_video_download mp` 启动，该命令仅运行公众号相关的功能

运行好之后，调用「获取可请求的公众号列表」会返回空列表。调用「获取指定公众号消息列表」会提示 `Please adding Credentials first`


此时还不可以正常使用。在任意 `macOS` 或 `Windows` 机器上，同样下载构建包，修改配置文件

```yaml
mp:
  remoteServer:
    protocol: "https"
    hostname: "rss.example.com"
    port: 80
  refreshToken: "123"
  tokenFilepath: ""
```

内容和上面的配置保持相同。然后，通过命令 `./wx_video_download` 启动，**打开任意公众号文章页面**，会自动向 Linux 服务提交该公众号的授权凭证

然后，再调用「获取可请求的公众号列表」，可以看到刚刚打开的公众号信息。以及在账号名称旁会出现 RSS 图标，点击会复制 `RSS` 订阅链接

![RSS按钮](../assets/official_account_rss.png)

内容类似 `https://rss.example.com/rss/mp?biz=MzI2NDk5NzA0Mw==`，可以在浏览器打开该链接，验证服务是否正常

到此，公众号RSS 服务就可以正常使用了。保持在 `macOS` 或 `Windows` 上的服务不要关闭，会定时获取在列表中的公众号，刷新凭证，保持 `Linux` 服务器上的 `RSS` 服务一直可用



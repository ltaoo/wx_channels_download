---
title: 公众号服务
---

# 公众号服务

对公众号的 RSS、刷新凭证、请求凭证进行配置

## remoteServer

```yaml
mp:
  remoteServer:
    protocol: "http"
    hostname: "127.0.0.1"
    port: 2022
```

刷新凭证时，是向该服务提交凭证信息。同时，如果 `RSS` 链接指定了 `proxy=1`，添加的代理前缀也是该服务的地址

如果在本地运行，需要和 `api` 的服务配置相同

### 远端服务模式

即部署在 `linux` 服务器上，仅提供 `RSS` 和 `API` 服务。该模式下，`remoteServer` 需要设置为服务器的地址，或者实际的域名，比如

```yaml
mp:
  remoteServer:
    protocol: "https"
    hostname: "rss.example.com"
    port: 80
```

同时配套的「本地服务」的配置，和上面保持一致

## 刷新凭证

```yaml
mp:
  refreshToken: "123"
```

同样本地和远端都要配置，本地向远端提交公众号凭证时，通过该 `token` 校验请求是否授权

## 请求凭证

```yaml
mp:
  tokenFilepath: "./token.txt"
```

是一个文件路径，可以指定一个绝对路径。启动后，如果该文件存在，则会读取内容，将**每一行作为一个token**，内容不为空（存在`token`时），每次 `API` 请求或 `RSS` 请求，都需要在 `query` 加上 `token=` 参数，否则会拒绝访问

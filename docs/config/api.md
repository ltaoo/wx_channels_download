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
```

- `protocol` API 服务协议，默认 `http`
- `hostname` API 服务主机名
- `port` API 服务端口

---
title: 代理配置
---

# 代理配置

通过 `config.yaml` 控制是否设置系统代理与代理服务端口。

```yaml
proxy:
  system: true
  hostname: 127.0.0.1
  port: 2023
```

- `system` 是否设置系统代理
- `hostname` 代理主机名
- `port` 代理端口

## 与 Clash 协同

当不希望修改系统代理时，可将 `system` 设为 `false`，并在 Clash 中加入以下 `Global Extend Script`，将流量转发到下载器代理服务（端口默认 `2023`）：

```js
function main(config, profileName) {
  config["proxy-groups"].unshift({
    name: 'ChannelsDownload',
    type: 'fallback',
    proxies: ["channels_download", "DIRECT"],
    interval: 5
  });
  config["proxies"].unshift({
    name: "channels_download",
    type: 'http',
    server: '127.0.0.1',
    port: 2023,
  });
  config.rules.unshift(...[
    "PROCESS-NAME,wx_video_download.exe,DIRECT",
    "PROCESS-NAME,wx_video_download,DIRECT",
    "DOMAIN-SUFFIX,qq.com,ChannelsDownload"
  ]);
  return config;
}
```

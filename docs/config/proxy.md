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
  tun: false
```

- `system` 是否设置系统代理
- `hostname` 代理主机名
- `port` 代理端口
- `tun` 是否启用 TUN 模式

## TUN 模式

TUN 模式在网络层截获流量，无需修改系统代理即可将目标域名的请求转发到代理服务。适用于不希望改动系统代理，或需要代理非 HTTP 流量的场景。

开启后会创建一个虚拟网卡，将匹配的流量路由到下载器代理进行处理。此时 `system` 设置将被忽略，不会修改系统代理。

> **注意**：TUN 模式会修改系统路由表，与 VPN 类软件（Clash、Surge 等）同时使用时会产生路由冲突，导致部分网站访问缓慢。建议不要同时开启 TUN 模式和 VPN，或使用 Clash 的 Script 方式（见下方）替代 TUN 模式。

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

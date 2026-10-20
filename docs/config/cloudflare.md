---
title: Cloudflare 配置
---

# Cloudflare 配置

用于配置公众号 RSS 服务和视频号分享链接解析服务使用的 Cloudflare Worker。

## 公众号 RSS 服务

```yaml
cloudflare:
  accountId: ""
  apiToken: ""
  refreshToken: ""
  adminToken: ""
  workerName: "official-account-api"
  d1Name: ""
```

- `accountId`: Cloudflare 帐号 ID。
- `apiToken`: Cloudflare Worker 认证 Token。
- `refreshToken`: 调用 mp-rss 凭证刷新接口所需的 token。
- `adminToken`: 调用 mp-rss 管理员接口所需的凭证。
- `workerName`: Cloudflare mp-rss Worker 名称，默认 `official-account-api`。
- `d1Name`: Cloudflare mp-rss D1 数据库名称。

## 视频号分享链接解析

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
  sphWorkerName: "sph"
  sphCookie: "hy_token=替换为你的实际值"
  sphCredential: "替换为你自己设置的访问密码"
```

- `sphWorkerName`: 视频号查询 Worker 名称。
- `sphCookie`: 元宝网页版 Cookie，关键字段为 `hy_token`，也可以填写完整 Cookie。本机解析也可使用此配置，无需部署 Worker。
- `sphCredential`: 自己设置的 Worker API 访问凭证，部署时不能为空。

Cookie 获取、部署和使用步骤见[分享链接解析](../feature/wxchannels/share-link.md)，部署命令说明见 [deploy sph](../cli/deploy.md#deploy-sph)。

同时使用公众号和视频号服务时，请把所需字段合并到同一个 `cloudflare:` 配置块中。

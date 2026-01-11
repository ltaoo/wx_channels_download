---
title: Cloudflare 配置
---

# Cloudflare 配置

用于配置 Cloudflare Worker 相关的参数，主要用于公众号 RSS 服务。

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

# 公众号 Cloudflare Worker

此目录包含 `deploy mp` 使用的 Worker 源码、D1 迁移和完整部署编排。

- `index.js`：公众号 RSS/API Worker 入口；
- `migrations/`：随 Go 二进制嵌入并由 `mp.Deploy` 执行的 D1 迁移；
- `deploy.go`：查找或创建 D1、验证连接、执行迁移、上传 Worker 并解析 workers.dev 地址；
- `wrangler.toml`、`package.json`：可选的 Wrangler 本地开发配置。

CLI 入口只负责读取 `cloudflare.*` 与 `mp.remoteServer.hostname` 配置并调用 `mp.Deploy`：

```bash
go run . deploy mp
```

API Token 需要 Workers Scripts:Edit 和 D1:Edit 权限。配置了 `cloudflare.d1Name` 时，部署会按名称查找数据库，不存在则自动创建；否则直接使用 `cloudflare.d1Id`。

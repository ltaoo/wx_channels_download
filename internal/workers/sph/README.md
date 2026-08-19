# 视频号查询 Worker

此目录包含 `deploy sph` 使用的 Cloudflare Worker 源码、查询页面和部署编排。

- `worker.js`：视频号查询 Worker 入口；
- `index.html`：Worker 根路径返回的查询页面；
- `deploy.go`：读取共享图标、上传 Worker、配置绑定并解析 workers.dev 地址。

CLI 入口只负责读取 `cloudflare.sph*` 配置并调用 `sph.Deploy`：

```bash
go run . deploy sph
```

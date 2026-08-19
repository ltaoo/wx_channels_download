# Cloudflare Durable Objects 部署能力

这个包只提供通用的 Durable Objects 编译和部署能力，不包含任何具体 Worker 源码、类名、Binding、Secret 或产品配置。

调用方负责提供：

- 已可直接上传的 JavaScript 模块；
- Worker 名称、兼容日期和入口模块名；
- Durable Object 的 Binding、导出类和存储类型；
- 需要写入的 Secrets；
- 是否启用 `workers.dev` 地址。

`Deploy` 将调用方提供的 JavaScript、Durable Object 声明和 Secrets 通过 Cloudflare REST API 上传，不负责编译具体应用源码。

当前 Task Hub 的原生 JavaScript 源码位于 `frontend/hub/index.js`，其部署参数和 `deploy hub` 子命令由 `cmd/deploy.go` 维护。

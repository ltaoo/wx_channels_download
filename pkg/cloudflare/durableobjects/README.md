# Cloudflare Durable Objects 部署能力

这个包只提供通用的 Durable Objects 编译和部署能力，不包含任何具体 Worker 源码、类名、Binding、Secret 或产品配置。

调用方负责提供：

- 已可直接上传的 JavaScript 模块；
- Worker 名称、兼容日期和入口模块名；
- Durable Object 的 Binding、导出类和存储类型；
- 需要写入的 Secrets；
- 是否启用 `workers.dev` 地址。

`Deploy` 将调用方提供的 JavaScript、Durable Object 声明和 Secrets 通过 Cloudflare REST API 上传，不负责编译具体应用源码。

当前 Task Hub 的原生 JavaScript 源码和 Worker + Pages 部署编排分别位于 `internal/workers/hub/index.js`、`internal/workers/hub/deploy.go`；`cmd/deploy.go` 只负责读取 CLI 配置、调用 `hub.Deploy` 和展示结果。

# Cloudflare Pages 部署能力

这个包通过 Cloudflare REST API 支持 Direct Upload Pages 项目的完整部署流程：

- 查询、创建或更新 Pages 项目；
- 配置 production/preview Secret 和 Service Binding；
- 校验、哈希、分桶并并发上传静态资源；
- 上传高级模式 `_worker.js` 和 `_routes.json`；
- 创建生产部署并返回项目、部署 ID 和访问地址。

调用方必须显式传入 Account ID 和 API Token。包内不保存认证信息，API Token 需要 Pages:Edit 权限。

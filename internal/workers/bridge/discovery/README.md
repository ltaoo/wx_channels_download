# Bridge Discovery Pages

一个独立 Cloudflare Pages 项目，提供类似 Folo Discover 的 RSS/Atom 订阅广场：关键词搜索、分类浏览、订阅源内容预览和 RSS 地址复制。

## 请求结构

```text
Browser
  ├── /，/app.js，/style.css ── Pages static assets
  ├── /api/search?q=关键词 ── Feedly public feed search
  └── /api/preview?url=RSS地址 ── 服务端代理并限制 512KB
```

`_worker.js` 使用 Pages advanced mode。搜索使用 Feedly 的公开 feed search API；预览在服务端校验公网 HTTP(S) 地址、跟随跳转后复查 URL、限制返回 512KB，并要求目标为 RSS/RDF/Atom 文档。静态响应带 CSP、`Referrer-Policy`、`X-Content-Type-Options` 和 `X-Frame-Options`。

前端 `app.js` 保持 Model/View 分离：Model 负责搜索状态、请求、RSS XML 解析、分类和复制状态；View 仅按状态渲染，并为每个节点声明 `data-n`。

## 部署

跟随 Bridge 一键部署：

```bash
go run . deploy bridge
```

也可以单独发布：

```bash
cd internal/workers/bridge/discovery
npx wrangler pages deploy public --project-name dm-bridge-discovery
```

Pages 项目名由 `bridge.deploy.discoveryProjectName` 指定；留空时使用 `<bridge.deploy.workerName>-discovery`。自定义 Worker 名称时不需要修改本项目，它没有 Bridge Service Binding，也不需要 secret。

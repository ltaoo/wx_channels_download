# Hub Admin Pages

这个目录是 Hub 管理页面的独立 Cloudflare Pages 项目。页面与 Durable Objects Worker 分开运行，浏览器只访问 Pages 域名。

`public` 只保存管理页自身的 HTML、CSS 和 JavaScript 源码。`build.sh` 在部署前创建被 Git 忽略的 `dist`，并从 `frontend/public/timeless` 复制页面需要的 Timeless 运行时，因此仓库中不保存第二份第三方资源。

## 请求结构

```text
Browser
  ├── /index.html, /app.js, /style.css ── Pages static assets
  └── /admin/api/* ── Pages Worker ── HUB Service Binding ── Hub Worker
```

`worker.js` 使用 Pages 高级模式和 HTTP Basic Auth 保护整个项目。用户名固定为 `admin`，密码来自 Pages secret `HUB_ADMIN_TOKEN`。API 代理保留 Authorization header，Worker 会再次验证同一个管理员 Token。

管理页通过右侧“调用 Token”抽屉调用 `/admin/api/access-tokens`，创建、立即过期和移除外部调用凭证。Token 可以留空自动生成，也可以手动指定；用途或使用人可选填。Token 明文只在创建响应中出现一次，Durable Object 只保存 SHA-256 摘要、可选说明、到期时间和最近使用时间。设备使用的 `HUB_TOKEN` 不会显示在管理页，也不应分发给外部调用者。

## 部署

在项目根目录执行以下命令，会依次部署 Worker、创建或更新 Pages 项目、配置 Secret 和 Service Binding，并发布管理页面：

```bash
go run . deploy hub
```

部署直接复用 `cloudflare.accountId` 和 `cloudflare.apiToken`，Token 需要 Workers Scripts:Edit 和 Pages:Edit 权限，不需要 Wrangler 登录。`HUB_ADMIN_TOKEN` 来自 `hub.deploy.adminToken`，不会写入 `wrangler.jsonc`、JavaScript 或任何静态文件。

Pages 项目名由 `hub.deploy.pagesProjectName` 指定；留空时自动使用 `<hub.deploy.workerName>-admin`。部署命令会把 `HUB` Service Binding 自动指向本次部署的 Worker。

## 本地检查

若要同时启动本地 Worker 和 Pages，推荐在项目根目录运行：

```bash
./internal/workers/hub/dev.sh
```

以下方式仅用于单独启动管理页面，并通过 Service Binding 联调另一个已经运行的 Worker。

仅检查静态页面可直接启动任意静态文件服务器。若要联调 Pages Worker 和远端 Worker，可使用 Wrangler，并提供本地 secret：

```bash
cd internal/workers/hub/admin
printf 'HUB_ADMIN_TOKEN="your-admin-token"\n' > .dev.vars
./build.sh
npx wrangler@latest pages dev
```

`.dev.vars*` 已加入仓库的 `.gitignore`；仍应避免把任何真实 secret 复制到其他受版本控制的文件中。

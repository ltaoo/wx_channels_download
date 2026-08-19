# Hub Admin Pages

这个目录是 Hub 管理页面的独立 Cloudflare Pages 项目。页面与 Durable Objects Worker 分开部署，浏览器只访问 Pages 域名。

`public` 只保存管理页自身的 HTML、CSS 和 JavaScript 源码。`build.sh` 在部署前创建被 Git 忽略的 `dist`，并从 `frontend/public/timeless` 复制页面需要的 Timeless 运行时，因此仓库中不保存第二份第三方资源。

## 请求结构

```text
Browser
  ├── /index.html, /app.js, /style.css ── Pages static assets
  └── /admin/api/* ── Pages Function ── HUB Service Binding ── Hub Worker
```

`functions/_middleware.js` 使用 HTTP Basic Auth 保护整个 Pages 项目。用户名固定为 `admin`，密码来自 Pages secret `HUB_ADMIN_TOKEN`。API 代理保留 Authorization header，Worker 会再次验证同一个管理员 Token。

## 首次部署

先部署 Worker：

```bash
go run . deploy hub
```

Wrangler 最新版要求 Node.js 22 或更高版本。先运行 `npx wrangler@latest login` 完成 Cloudflare 登录，再创建 Pages 项目、设置 secret 并部署：

```bash
cd frontend/hub/admin
npx wrangler@latest pages project create wx-channels-hub-admin --production-branch main
npx wrangler@latest pages secret put HUB_ADMIN_TOKEN --project-name wx-channels-hub-admin
./build.sh
npx wrangler@latest pages deploy
```

`HUB_ADMIN_TOKEN` 必须与 `config.yaml` 的 `hub.deploy.adminToken` 一致。不要把该值写入 `wrangler.jsonc`、JavaScript 或任何静态文件。

以后只更新管理页面时执行：

```bash
cd frontend
npm run deploy:hub-admin
```

默认 Pages 项目名是 `wx-channels-hub-admin`，Service Binding 指向 `wx-channels-hub`。如果 Worker 使用其他 `hub.deploy.workerName`，同步修改 `wrangler.jsonc` 中 `services[0].service`。

## 本地检查

仅检查静态页面可直接启动任意静态文件服务器。若要联调 Pages Function 和远端 Worker，可使用 Wrangler，并提供本地 secret：

```bash
cd frontend/hub/admin
printf 'HUB_ADMIN_TOKEN="your-admin-token"\n' > .dev.vars
./build.sh
npx wrangler@latest pages dev
```

`.dev.vars*` 已加入仓库的 `.gitignore`；仍应避免把任何真实 secret 复制到其他受版本控制的文件中。

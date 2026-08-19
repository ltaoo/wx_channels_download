# Cloudflare Durable Objects Task Hub

这个 Hub 让无法直接通信的内网 `wx_channels_download` 实例通过出站 WebSocket 共享能力和转交下载任务。Hub 只保存任务 JSON、解析结果和下载元数据，不代理或保存视频文件。

## 架构

```text
实例 A（无视频号）──WSS──┐
                          ├── Cloudflare Worker ── Durable Object（每个 hub.id 一个）
实例 B（有视频号）──WSS──┘              │
                                        └── SQLite：任务、租约、结果

管理员浏览器 ──HTTPS── Pages 静态页面 / Function ──Service Binding──┘
```

任务采用至少一次投递：Hub 分配任务时创建 120 秒租约，执行实例每约 40 秒续租。连接中断或实例退出后，租约过期的任务会重新排队；提交方错过 WebSocket 完成通知时，仍可通过任务查询 API 获取持久结果。

## 部署 Worker

Worker 部署器位于同一个 Go 可执行程序中。Hub 使用带 JSDoc 类型声明的原生 JavaScript，源码通过 `go:embed` 嵌入后由 Go 直接调用 Cloudflare REST API 上传；部署 Worker 不需要 esbuild、Node.js、npm 或 Wrangler。管理页面已拆分为独立 Cloudflare Pages 项目，使用 Wrangler 单独部署。

先在 `config.yaml` 中配置：

```yaml
cloudflare:
  accountId: "<ACCOUNT_ID>"
  apiToken: "<API_TOKEN>" # 需要 Workers Scripts:Edit

hub:
  deploy:
    workerName: "wx-channels-hub"
    token: "<随机高强度共享密钥>"
    adminToken: "<与客户端Token不同的管理员密码>"
```

然后在项目根目录执行：

```bash
go run . deploy hub
```

部署命令会直接上传 `frontend/hub/index.js` ES module Worker、声明 `HUBS` Durable Object namespace 和 SQLite `HubDurableObject` 导出、写入 `HUB_TOKEN`、`HUB_ADMIN_TOKEN` secrets，并启用 `workers.dev` 地址。重复执行会更新同一个 Worker，并保留 Durable Object 数据。

两个 Token 都应使用随机高强度凭证且不能相同。`HUB_TOKEN` 提供给连接 Hub 的电脑，`HUB_ADMIN_TOKEN` 只提供给管理员。部署后记录命令输出的 Worker URL，例如 `https://wx-channels-hub.<account>.workers.dev`。

Worker 的 `/health` 无需认证；任务接口要求 `Authorization: Bearer <HUB_TOKEN>`；`/admin/api/*` 管理 API 要求独立管理员认证。Worker 不再返回管理页面静态资源。

## 管理页面

管理页源码位于 `frontend/hub/admin`：

- `public/index.html`、`style.css`、`app.js` 是管理页自身的静态源码；
- `build.sh` 生成被 Git 忽略的 `dist`，并从共享的 `frontend/public/timeless` 复制运行时；
- `functions/_middleware.js` 对静态资源和 API 执行 HTTP Basic Auth；
- `functions/admin/api/[[path]].js` 通过 `HUB` Service Binding 将同源 `/admin/api/*` 请求转发给 Worker；
- `wrangler.jsonc` 声明 Pages 项目、静态目录、secret 和 Service Binding。

先完成上面的 Worker 部署，再单独部署 Pages。Wrangler 最新版要求 Node.js 22 或更高版本；首次部署前先运行 `npx wrangler@latest login` 完成 Cloudflare 登录，然后执行：

```bash
cd frontend/hub/admin
npx wrangler@latest pages project create wx-channels-hub-admin --production-branch main
npx wrangler@latest pages secret put HUB_ADMIN_TOKEN --project-name wx-channels-hub-admin
./build.sh
npx wrangler@latest pages deploy
```

设置 `HUB_ADMIN_TOKEN` 时输入与 `hub.deploy.adminToken` 完全相同的值。后续页面更新可在 `frontend` 目录执行 `npm run deploy:hub-admin`，该命令会先重新生成 Pages `dist` 再部署。

默认配置把 Pages 项目 `wx-channels-hub-admin` 绑定到 Worker `wx-channels-hub`。如果修改了 `hub.deploy.workerName` 或 Pages 项目名，需要同步修改 `frontend/hub/admin/wrangler.jsonc` 中的 `name` 和 `services[0].service`。

部署后浏览器访问：

```text
https://wx-channels-hub-admin.pages.dev
```

浏览器会显示 HTTP Basic Auth 登录框：

- 用户名：`admin`
- 密码：`hub.deploy.adminToken`

未通过认证时，Pages Function 返回 `401`，不会返回管理页面或管理数据。页面每 5 秒刷新，展示所有已自动登记的 Hub、电脑的在线/执行任务/离线状态、能力、连接与最近活跃时间，以及任务状态计数。同一电脑连接多个 Hub 时会分别显示在对应 Hub 中。

管理 API 也可以使用 Bearer 管理员 Token：

```bash
curl -H 'Authorization: Bearer <HUB_ADMIN_TOKEN>' \
  'https://wx-channels-hub.<account>.workers.dev/admin/api/overview'
```

Hub 会在客户端访问时自动登记，不需要在部署配置中维护 Hub ID 列表。电脑离线后仍保留在管理页面中，并显示最近活跃时间和离线时间；重新连接后状态会恢复为在线。

### 管理页能力测试

每个 Hub 卡片都提供 `wxchannels.fetch` 测试区域：

1. 从当前在线且声明了 `wxchannels.fetch` 能力的电脑中选择目标；
2. 填写一个视频号 URL；
3. 点击“创建测试任务”。

管理页会创建真实 Hub 任务，并轮询展示“等待领取、已推送、正在获取、成功回传或失败”等状态。任务成功时页面会显示目标电脑提交的 result/content。

获取成功后，可以继续从当前在线且声明了 `download.create` 能力的电脑中选择下载目标。管理端直接引用前一任务持久化的 content，创建 `download.create` 任务；可选填目标电脑上的下载目录和文件名，留空时使用目标电脑的本地默认配置。目标电脑创建任务后会自动开始下载，管理页继续展示该任务的接收、执行和回传结果。

测试和下载接口都受 `HUB_ADMIN_TOKEN` 保护。离线或没有对应能力的电脑不能作为目标。

管理页面中的更新时间、登记时间、连接时间、最近活跃时间、离线时间和测试任务时间统一使用 `YYYY-MM-DD HH:mm:ss` 格式。

## 配置多个 Hub

`hub.deploy` 只用于部署一个 Worker；`hub.instances` 定义当前电脑实际连接的 Hub。每项的 `name` 是本地 API 选择 Hub 时使用的名称，`id` 是 Cloudflare Durable Object 的隔离标识。

一台有视频号能力的电脑可以同时向家庭和办公两个 Hub 提供 `wxchannels.fetch`：

```yaml
hub:
  defaultInstance: "home"
  instances:
    - name: "home"
      enabled: true
      url: "https://wx-channels-hub.<account>.workers.dev"
      id: "home"
      clientId: "computer-c"
      token: "<HUB_TOKEN>"
      httpTimeoutSeconds: 30
      capabilities:
        wxchannels: true
        download: false
    - name: "office"
      enabled: true
      url: "https://wx-channels-hub.<account>.workers.dev"
      id: "office"
      clientId: "computer-c"
      token: "<HUB_TOKEN>"
      httpTimeoutSeconds: 30
      capabilities:
        wxchannels: true
        download: false
```

两个实例也可以使用不同的 Worker URL 和 token。`clientId` 只需在同一个 Hub ID 内唯一；同一台电脑连接不同 Hub 时可以复用同一个 `clientId`。

没有视频号、需要使用家庭 Hub 的电脑：

```yaml
hub:
  defaultInstance: "home"
  instances:
    - name: "home"
      enabled: true
      url: "https://wx-channels-hub.<account>.workers.dev"
      id: "home"
      clientId: "computer-a"
      token: "<HUB_TOKEN>"
      httpTimeoutSeconds: 30
      capabilities:
        wxchannels: false
        download: true
```

通过 `GET http://127.0.0.1:2022/api/hub/status` 检查全部连接。响应顶层是默认 Hub 状态，`hubs` 数组包含每个具名实例的状态。也可以使用 `?hub=office` 选择顶层展示的实例。

## 借用视频号能力并在 A 下载

向实例 A 的本地 API 提交 URL。省略 `target_client_id` 时，Hub 会选择任意在线且声明了 `wxchannels.fetch` 能力的实例；也可以指定 `service-b`。

```http
POST /api/hub/tasks/wxchannels
Content-Type: application/json

{
  "hub": "home",
  "url": "https://channels.weixin.qq.com/web/pages/feed?...",
  "target_client_id": "service-b",
  "idempotency_key": "wx-feed-123",
  "download": {
    "download_dir": "/path/on/service-a",
    "auto_start": true,
    "config": {}
  }
}
```

执行流程：

1. A 将 `wxchannels.fetch` 任务提交到 Hub。
2. Hub 推送给 B；B 直接调用已注册的 `wxchannels` adapter `Fetch(url)`。
3. B 把原始 content JSON 回传 Hub 并标记任务完成。
4. Hub 向 A 推送 `task.completed`。
5. A 使用返回的 content 和 `BuildFromFetch=true` 创建并自动启动本地下载任务。

如果没有提供 `download`，A 只接收和保存解析结果，不会自动下载。可使用返回的任务 ID 调用 `GET /api/hub/tasks/<task-id>?hub=home` 查询 content。省略 `hub` 时使用 `hub.defaultInstance`。

## 将下载任务从 A 交给 B

直接 URL 下载：

```http
POST /api/hub/tasks/download
Content-Type: application/json

{
  "hub": "home",
  "target_client_id": "service-b",
  "idempotency_key": "download-file-123",
  "url_request": {
    "url": "https://example.com/video.mp4",
    "download_dir": "/path/on/service-b",
    "filename": "video.mp4",
    "auto_start": true,
    "config": {}
  }
}
```

平台 content 下载使用 `request`，其结构与 `/api/v1/download_task/create` 中的单个 object 相同：

```json
{
  "hub": "home",
  "target_client_id": "service-b",
  "request": {
    "platform": "wxchannels",
    "content": {},
    "build_from_fetch": true,
    "download_dir": "/path/on/service-b",
    "auto_start": true,
    "config": {}
  }
}
```

## 本地 API

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/hub/status?hub=` | 全部 Hub 连接状态；可选择顶层实例 |
| `POST` | `/api/hub/tasks/wxchannels` | 提交视频号解析任务；body 使用 `hub` 选择实例 |
| `POST` | `/api/hub/tasks/download` | 向指定客户端提交下载任务；body 使用 `hub` 选择实例 |
| `GET` | `/api/hub/tasks/:id?hub=` | 查询指定 Hub 的任务、content/result 和错误 |
| `GET` | `/api/hub/tasks?hub=&status=&limit=` | 查询指定 Hub 上当前客户端发布的任务 |

## 可靠性和限制

- 相同发布方和 `idempotency_key` 只创建一个 Hub 任务。
- 单个 Hub 内的客户端当前一次领取一个任务；多个 Hub 可以各自分配任务。同一电脑上的视频号解析会跨 Hub 串行执行，避免同时操作微信视频号。
- 单个任务 payload 或 result 上限为 1 MiB。
- 完成和失败任务保留 7 天。
- 任务可能因租约过期而再次执行，执行端必须允许重复调用。下载服务已有内容冲突检查；调用方仍应提供稳定的 `idempotency_key`。
- `HUB_TOKEN` 是同一 Worker 内客户端共享的信任边界。互不信任的环境应使用不同 Worker/token；实例 `name` 只是本地别名，`id` 只隔离状态，不隔离拥有相同 token 的调用方。

# Personal CGI Hub

一个 Hub 由一个人管理，用来连接这个人的多台操作系统设备，并把统一的 `method + args` 调用调度给合适的在线设备。Hub 只保存调用参数、结果和下载元数据，不代理或保存视频文件。

## 概念

- 一个 Worker 部署就是一个 Hub，不再在 Worker 内继续划分多个 `hub.id`。
- 一台设备指一个操作系统实例：macOS 宿主机是一台；Docker 中的 Linux、虚拟机中的 Windows/Linux 分别是新的设备。
- 同一操作系统中的多个进程仍属于同一设备，使用同一个稳定的 `deviceId`；新连接会替换该设备的旧连接。
- 每台设备注册自己可处理的 `methods`。外部调用方只面对一个 CGI 风格的 Hub 接口，由 `method + args` 表达调用，由 Hub 选择实际执行设备。
- 多 Hub 的发现、授权、检索和路由属于更高层的 Hub 市场或 Hub 集合，不属于当前 Worker。

```text
外部调用方 ──HTTPS──┐
                    │
管理员浏览器 ─HTTPS─┼── Personal Hub Worker ── Durable Object + SQLite
                    │             │
macOS 设备 ─────WSS─┤             ├── method: wxchannels.fetch
Linux 容器 ─────WSS─┘             └── method: download.create
```

任务采用至少一次投递：Hub 分配任务时创建 120 秒租约，执行设备定期续租。连接中断后，租约过期的任务会重新排队；发布方错过 WebSocket 完成通知时，仍可通过任务查询 API 获取持久结果。

## 部署 Hub

在 `config.yaml` 中配置 Cloudflare 部署信息：

```yaml
cloudflare:
  accountId: "<ACCOUNT_ID>"
  apiToken: "<API_TOKEN>" # Workers Scripts:Edit 和 Pages:Edit

hub:
  deploy:
    workerName: "wx-channels-hub"
    pagesProjectName: "" # 留空时使用 wx-channels-hub-admin
    token: "<只供设备使用的随机高强度 Secret>"
    adminToken: "<与设备 Token 不同的管理员密码>"
```

执行：

```bash
go run . deploy hub
```

命令会部署单个 Durable Object Hub Worker 和独立的 Cloudflare Pages 管理项目。重复部署会更新代码并保留该 Hub 的设备登记和任务数据。

- `HUB_TOKEN` 只用于设备连接和设备自身发布调用，不应分发给外部调用者。
- `HUB_ADMIN_TOKEN` 只用于管理页面、管理 API 和调用 Token 管理。
- 外部调用使用管理员在 Hub 中动态创建的独立调用 Token。
- `/health` 不需要认证。
- `/v1/connect` 需要设备 `HUB_TOKEN` 和设备身份 header。
- 其他 `/v1/*` 接口接受动态调用 Token；设备程序也可使用自己的设备 Secret。
- `/admin/api/*` 需要管理员认证。

## 注册操作系统设备

每个操作系统使用一份单 Hub 配置：

```yaml
hub:
  enabled: true
  url: "https://wx-channels-hub.<account>.workers.dev"
  deviceId: "mayfair-macbook"
  deviceName: "Mayfair MacBook"
  token: "<HUB_TOKEN>"
  httpTimeoutSeconds: 30
  methods: "auto"
```

`deviceId` 在当前 Hub 内必须唯一且保持稳定。留空时程序使用系统主机名；生产环境建议显式设置。`deviceName` 是管理页显示名称，留空时同样使用主机名。操作系统类型与当前程序实际注册的方法由程序自动上报。

`methods` 是通用方法白名单，不再为每项能力增加布尔配置：`auto` 发布当前程序全部已注册方法，`none` 只允许该设备发布调用而不执行远程调用，也可填写 `wxchannels.fetch,wxchannels.contact.feed.list,download.create` 这样的逗号分隔列表。

视频号 adapter 当前会注册以下 Hub 方法：

| method | args | 对应 scraper 方法 |
| --- | --- | --- |
| `wxchannels.contact.search` | `keyword`, `next_marker` | `SearchChannelsContact` |
| `wxchannels.contact.feed.list` | `username`, `next_marker` | `FetchChannelsFeedListOfContact` |
| `wxchannels.live.replay.list` | `username`, `next_marker` | `FetchChannelsLiveReplayList` |
| `wxchannels.feed.profile` | `oid`, `nid`, `url`, `eid` | `FetchChannelsFeedProfile` |
| `wxchannels.feed.comment.list` | `oid`, `nid`, `comment_id`, `next_marker` | `FetchChannelsFeedCommentList` |
| `wxchannels.feed.share_url` | `oid` | `FetchChannelsFeedShareUrl` |

这些方法依赖设备上的视频号页面 WebSocket 连接；设备连接 Hub 但视频号页面未连接时，调用会失败或超时。

Docker 中的 Linux 使用独立身份：

```yaml
hub:
  enabled: true
  url: "https://wx-channels-hub.<account>.workers.dev"
  deviceId: "downloader-linux"
  deviceName: "Downloader Linux"
  token: "<HUB_TOKEN>"
  httpTimeoutSeconds: 30
  methods: "download.create"
```

本地状态接口：

```http
GET /api/hub/status
```

返回当前操作系统设备到个人 Hub 的唯一连接状态，不再接受 `?hub=` 选择参数。

## 管理页面

管理页源码位于 `internal/workers/hub/admin`：

- `public/index.html`、`style.css`、`app.js` 是静态源码；
- `build.sh` 生成被 Git 忽略的 `dist`，并从 `frontend/public/timeless` 复制共享运行时；
- `worker.js` 负责 HTTP Basic Auth 和 `/admin/api/*` Service Binding 转发。

浏览器登录：

- 用户名：`admin`
- 密码：`hub.deploy.adminToken`

管理页每 5 秒刷新，展示操作系统设备，并通过右侧抽屉添加和管理调用 Token。管理员可以创建独立 Token、选择 1/7/30/90 天或永久有效、让 Token 立即过期以及移除 Token。Token 可留空由 Hub 自动生成，也可手动指定；用途或使用人可选填。Token 明文只在创建成功时显示一次，Hub 仅保存 SHA-256 摘要。

每个调用 Token 都是独立发布方：它只能列出和查询自己创建的调用，不能读取其他 Token 或设备发布的任务。Token 过期或移除后，新请求立即返回 `401`；已经分配给设备的调用仍会继续执行。

管理 API：

```bash
curl -H 'Authorization: Bearer <HUB_ADMIN_TOKEN>' \
  'https://wx-channels-hub.<account>.workers.dev/admin/api/overview'
```

overview 返回：

```json
{
  "generated_at": 0,
  "devices": [],
  "methods": [],
  "task_counts": [],
  "access_tokens": []
}
```

管理 API 也支持直接维护调用 Token：

```http
GET /admin/api/access-tokens
POST /admin/api/access-tokens
DELETE /admin/api/access-tokens/:id
POST /admin/api/access-tokens/:id/expire
Authorization: Bearer <HUB_ADMIN_TOKEN>
```

创建请求：

```json
{
  "name": "合作方 A",
  "token": "custom-token-at-least-16-characters",
  "expires_in_seconds": 604800
}
```

`token` 留空或省略时由 Hub 自动生成；自定义值必须为 16–256 位，只能包含字母、数字和 `._~+/=-`。`name` 可留空，`expires_in_seconds` 为 `null` 时永不过期。创建响应中的 `token` 是唯一一次返回的明文。

每张设备卡片在设备注册 `wxchannels.fetch` 时提供定向方法调用测试。获取成功后，可以把结果作为 `args` 提交给任意在线且注册 `download.create` 的设备。两个操作都调用同一个 `/admin/api/call` 接口。

## 本地开发

```bash
./internal/workers/hub/dev.sh
```

Worker 默认监听 `http://127.0.0.1:8787`，Pages 默认监听 `http://127.0.0.1:8788`。管理页用户名为 `admin`，默认本地密码为 `local-hub-admin-token`。

可以覆盖端口和 Token：

```bash
HUB_WORKER_PORT=8797 \
HUB_PAGES_PORT=8798 \
HUB_TOKEN=my-local-token \
HUB_ADMIN_TOKEN=my-local-admin-token \
WRANGLER_VERSION=latest \
./internal/workers/hub/dev.sh
```

## 设备之间调用方法

本地 API 提供统一入口：

```http
POST /api/hub/call
Content-Type: application/json

{
  "method": "wxchannels.fetch",
  "target_device_id": "mayfair-macbook",
  "idempotency_key": "wx-feed-123",
  "args": {
    "url": "https://channels.weixin.qq.com/web/pages/feed?..."
  }
}
```

`target_device_id` 可省略，此时 Hub 从在线、空闲且注册了该 `method` 的设备中选择一个执行。增加新方法时，只需要设备端注册处理函数，不需要修改 Hub 的任务类型定义。

现有业务接口是统一调用的便捷适配层。例如，设备 A 可以把视频号解析交给设备 B：

设备 A 可以把视频号解析交给设备 B：

```http
POST /api/hub/tasks/wxchannels
Content-Type: application/json

{
  "url": "https://channels.weixin.qq.com/web/pages/feed?...",
  "target_device_id": "mayfair-macbook",
  "idempotency_key": "wx-feed-123",
  "download": {
    "download_dir": "/path/on/publisher",
    "auto_start": true,
    "config": {}
  }
}
```

省略 `target_device_id` 时，Hub 自动选择在线、空闲且注册了 `wxchannels.fetch` 的设备。

向指定设备创建下载任务：

```http
POST /api/hub/tasks/download
Content-Type: application/json

{
  "target_device_id": "downloader-linux",
  "idempotency_key": "download-file-123",
  "url_request": {
    "url": "https://example.com/video.mp4",
    "download_dir": "/downloads",
    "filename": "video.mp4",
    "auto_start": true,
    "config": {}
  }
}
```

## Hub 对外 CGI 接口

面向使用者的完整接入流程、任务状态说明和多语言代码示例见 [`docs/feature/hub.md`](../../../docs/feature/hub.md)。

外部系统可以把整个 Hub 当作一个 CGI 节点。查询在线设备和方法：

```http
GET /v1
Authorization: Bearer <CALL_TOKEN>
```

同步调用并直接获得方法结果：

```http
POST /v1/invoke
Authorization: Bearer <CALL_TOKEN>
Content-Type: application/json

{
  "method": "wxchannels.contact.feed.list",
  "target_device_id": "mayfair-macbook",
  "args": {
    "username": "example@finder",
    "next_marker": ""
  }
}
```

`/v1/invoke` 不需要 `idempotency_key`，不返回任务信息，最多等待 10 秒。成功时响应体就是设备方法返回的 JSON；执行失败返回 `502`，超时返回 `504`。超时不会取消内部任务，因此有副作用或可能超过 10 秒的调用应使用异步接口。

创建异步任务：

```http
POST /v1/call
Authorization: Bearer <CALL_TOKEN>
Content-Type: application/json

{
  "method": "wxchannels.fetch",
  "idempotency_key": "external-123",
  "args": {
    "url": "https://channels.weixin.qq.com/web/pages/feed?..."
  }
}
```

异步获取某个视频号账号的视频列表：

```http
POST /v1/call
Authorization: Bearer <CALL_TOKEN>
Content-Type: application/json

{
  "method": "wxchannels.contact.feed.list",
  "target_device_id": "mayfair-macbook",
  "args": {
    "username": "example@finder",
    "next_marker": ""
  }
}
```

不指定 `target_device_id` 时，外部调用方只依赖 Hub 提供的方法，不需要了解内部设备。指定时，Hub 会确认目标设备在线并注册了该方法后再调度。

提交响应中的任务 ID 可以继续使用同一个调用 Token 查询：

```http
GET /v1/tasks/<task-id>
Authorization: Bearer <CALL_TOKEN>
```

`GET /v1/tasks` 也只返回当前调用 Token 发布的任务。调用方不能通过 query 或 header 改写自己的发布方身份。

## 本地 API

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/hub/status` | 当前设备到个人 Hub 的连接状态 |
| `POST` | `/api/hub/call` | 使用 `method + args` 提交任意方法调用 |
| `POST` | `/api/hub/tasks/wxchannels` | 提交视频号解析任务 |
| `POST` | `/api/hub/tasks/download` | 向指定设备提交下载任务 |
| `GET` | `/api/hub/tasks/:id` | 查询任务、content/result 和错误 |
| `GET` | `/api/hub/tasks?status=&limit=` | 查询当前设备发布的任务 |

## 旧配置迁移

升级后，旧外部调用方不应继续使用共享 `HUB_TOKEN`。请先用 `HUB_ADMIN_TOKEN` 登录管理页，为每个调用方创建独立调用 Token，再替换其 `Authorization`。设备配置中的 `hub.token` 保持不变。

旧版只有一个 `hub.instances` 项时仍可临时连接，新版会使用其中的 `url`、`clientId` 和 token，并保留旧路径与旧协议字段兼容。旧 `capabilities` 布尔值会在迁移期映射为对应 methods；新配置请使用通用的 `hub.methods`。请迁移为新的 `hub.url`、`hub.deviceId`、`hub.deviceName` 和 `hub.token`。

旧配置包含多个实例时会直接报错，因为单个操作系统设备现在只属于一个个人 Hub；多 Hub 管理由更高层应用负责。

从旧版多 `hub.id` Worker 第一次升级时会启用新的单例 Durable Object。设备会在重连后重新登记，但旧 Durable Object 中的历史任务不会自动合并到新 Hub。

## 可靠性和限制

- 相同发布方和 `idempotency_key` 只创建一个任务。
- 每台设备当前一次领取一个任务；同一操作系统内的视频号解析串行执行。
- 单次调用的 args 或 result 上限为 1 MiB。
- 完成和失败任务保留 7 天。
- 任务可能因租约过期再次执行，执行端必须允许重复调用。
- `HUB_TOKEN` 是所有设备共享的高权限 Secret，只应写入设备配置；面向人员和外部系统必须分发独立调用 Token。
- 动态调用 Token 当前拥有全部在线 `methods` 的调用权限；更细粒度的方法授权、限流和计费仍属于后续的 Hub 市场或上层网关。

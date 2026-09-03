# Personal CGI Bridge

一个 Bridge 由一个人管理，是连接外部调用方与多台操作系统设备的桥接/转发服务。它把统一的 `method + args` 调用转发给合适的在线设备，只保存调用参数、结果和下载元数据，不代理或保存视频文件。

## 概念

- 一个 Worker 部署就是一个 Bridge，不再在 Worker 内继续划分多个 `bridge.id`。
- 一台设备指一个操作系统实例：macOS 宿主机是一台；Docker 中的 Linux、虚拟机中的 Windows/Linux 分别是新的设备。
- 同一操作系统中的多个进程仍属于同一设备，使用同一个稳定的 `deviceId`；新连接会替换该设备的旧连接。
- 每台设备注册自己可处理的 `methods`。外部调用方只面对一个 CGI 风格的 Bridge 接口，由 `method + args` 表达调用，由 Bridge 选择实际执行设备。
- 多 Bridge 的发现、授权、检索和路由属于更高层的 Bridge 市场或 Bridge 集合，不属于当前 Worker。

```text
外部调用方 ──HTTPS──┐
                    │
管理员浏览器 ─HTTPS─┼── Personal Bridge Worker ── Durable Object + SQLite
                    │             │
macOS 设备 ─────WSS─┤             ├── method: wxchannels.fetch
Linux 容器 ─────WSS─┘             └── method: download.create
```

任务采用至少一次投递：Bridge 分配任务时创建 120 秒租约，执行设备定期续租。连接中断后，租约过期的任务会重新排队；发布方错过 WebSocket 完成通知时，仍可通过任务查询 API 获取持久结果。

## 部署 Bridge

在 `config.yaml` 中配置 Cloudflare 部署信息：

```yaml
cloudflare:
  accountId: "<ACCOUNT_ID>"
  apiToken: "<API_TOKEN>" # Workers Scripts:Edit 和 Pages:Edit

bridge:
  deploy:
    workerName: "dm-bridge"
    pagesProjectName: "" # 留空时使用 dm-bridge-admin
    token: "<只供设备使用的随机高强度 Secret>"
    adminToken: "<与设备 Token 不同的管理员密码>"
```

执行：

```bash
go run . deploy bridge
```

命令会部署单个 Durable Object Bridge Worker 和独立的 Cloudflare Pages 管理项目。重复部署会更新代码并保留该 Bridge 的设备登记和任务数据。

- `BRIDGE_TOKEN` 只用于设备连接和设备自身发布调用，不应分发给外部调用者。
- `BRIDGE_ADMIN_TOKEN` 只用于管理页面、管理 API 和调用 Token 管理。
- 外部调用使用管理员在 Bridge 中动态创建的独立调用 Token。
- `/health` 不需要认证。
- `/v1/connect` 需要设备 `BRIDGE_TOKEN` 和设备身份 header。
- 其他 `/v1/*` 接口接受动态调用 Token；设备程序也可使用自己的设备 Secret。
- `/admin/api/*` 需要管理员认证。

## 注册操作系统设备

每个操作系统使用一份单 Bridge 配置：

```yaml
bridge:
  enabled: true
  url: "https://dm-bridge.<account>.workers.dev"
  deviceId: "mayfair-macbook"
  deviceName: "Mayfair MacBook"
  token: "<BRIDGE_TOKEN>"
  httpTimeoutSeconds: 30
  methods: "auto"
```

`deviceId` 在当前 Bridge 内必须唯一且保持稳定。留空时程序使用系统主机名；生产环境建议显式设置。`deviceName` 是管理页显示名称，留空时同样使用主机名。操作系统类型与当前程序实际注册的方法由程序自动上报。

`methods` 是通用方法白名单，不再为每项能力增加布尔配置：`auto` 发布当前程序全部已注册方法，`none` 只允许该设备发布调用而不执行远程调用，也可填写 `wxchannels.fetch,wxchannels.contact.feed.list,download.create` 这样的逗号分隔列表。

视频号 adapter 当前会注册以下 Bridge 方法：

| method | args | 对应 scraper 方法 |
| --- | --- | --- |
| `wxchannels.contact.search` | `keyword`, `next_marker` | `SearchChannelsContact` |
| `wxchannels.contact.feed.list` | `username`, `next_marker` | `FetchChannelsFeedListOfContact` |
| `wxchannels.live.replay.list` | `username`, `next_marker` | `FetchChannelsLiveReplayList` |
| `wxchannels.feed.profile` | `oid`, `nid`, `url`, `eid` | `FetchChannelsFeedProfile` |
| `wxchannels.feed.comment.list` | `oid`, `nid`, `comment_id`, `next_marker` | `FetchChannelsFeedCommentList` |
| `wxchannels.feed.share_url` | `oid` | `FetchChannelsFeedShareUrl` |

这些方法依赖设备上的视频号页面 WebSocket 连接；设备连接 Bridge 但视频号页面未连接时，调用会失败或超时。

公众号 adapter 当前会注册以下 Bridge 方法：

| method | args | 对应 scraper 方法 |
| --- | --- | --- |
| `wxmp.biz.msg.list` | `username`, `offset` | `FetchBizMsgList` |

该方法依赖设备上的公众号页面 WebSocket 连接。`username` 必填，`offset` 可省略或传上一页返回的偏移量。

Docker 中的 Linux 使用独立身份：

```yaml
bridge:
  enabled: true
  url: "https://dm-bridge.<account>.workers.dev"
  deviceId: "downloader-linux"
  deviceName: "Downloader Linux"
  token: "<BRIDGE_TOKEN>"
  httpTimeoutSeconds: 30
  methods: "download.create"
```

本地状态接口：

```http
GET /api/bridge/status
```

返回当前操作系统设备到个人 Bridge 的唯一连接状态，不再接受 `?bridge=` 选择参数。

## 管理页面

管理页源码位于 `internal/workers/bridge/admin`：

- `public/index.html`、`style.css`、`app.js` 是静态源码；
- `build.sh` 生成被 Git 忽略的 `dist`，并从 `frontend/public/timeless` 复制共享运行时；
- `worker.js` 负责 HTTP Basic Auth 和 `/admin/api/*` Service Binding 转发。

浏览器登录：

- 用户名：`admin`
- 密码：`bridge.deploy.adminToken`

管理页每 5 秒刷新，展示操作系统设备。每张设备卡片都有“日志”按钮，可在右侧抽屉查看该设备最近 7 天的连接、断开、连接心跳、调用下发、设备接收、任务心跳、成功响应、失败重试、租约超时和管理员重置等事件。日志支持分类筛选、自动刷新、向前分页，并可展开查看调用参数或响应数据；Token、Cookie、Authorization、Password、Secret 等敏感字段会在落库前自动脱敏，单条超大详情会截断。

管理页也通过右侧抽屉添加和管理调用 Token。管理员可以创建独立 Token、设置初始积分、充值积分、选择 1/7/30/90 天或永久有效、让 Token 立即过期以及移除 Token。Token 可留空由 Bridge 自动生成，也可手动指定；用途或使用人可选填。Token 明文只在创建成功时显示一次，Bridge 仅保存 SHA-256 摘要。

每个调用 Token 都是独立发布方：它只能列出和查询自己创建的调用，不能读取其他 Token 或设备发布的任务。Token 过期或移除后，新请求立即返回 `401`；已经分配给设备的调用仍会继续执行。

管理 API：

```bash
curl -H 'Authorization: Bearer <BRIDGE_ADMIN_TOKEN>' \
  'https://dm-bridge.<account>.workers.dev/admin/api/overview'
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
POST /admin/api/access-tokens/:id/credits
GET /admin/api/credit-transactions?access_token_id=&limit=
GET /admin/api/devices/:device_id/logs?category=&before_id=&limit=
Authorization: Bearer <BRIDGE_ADMIN_TOKEN>
```

设备日志 `category` 可选 `connection`、`heartbeat`、`call`、`response`、`system`；`limit` 范围为 1–500，使用响应中的 `next_before_id` 继续读取更早日志。

创建请求：

```json
{
  "name": "合作方 A",
  "token": "custom-token-at-least-16-characters",
  "expires_in_seconds": 604800,
  "credits": 1000
}
```

`token` 留空或省略时由 Bridge 自动生成；自定义值必须为 16–256 位，只能包含字母、数字和 `._~+/=-`。`name` 可留空，`expires_in_seconds` 为 `null` 时永不过期，`credits` 是非负整数且默认为 `0`。创建响应中的 `token` 是唯一一次返回的明文。

充值请求中的 `amount` 是积分增量。管理 API 允许负数调整以修正账目，但调整后余额不能小于 `0`；管理页只提供正数充值：

```json
{
  "amount": 500,
  "reason": "购买 500 积分"
}
```

每次变动都会写入永久积分流水，包含 Token、关联任务、变动值、变动后余额、method、原因和时间。移除 Token 使用软撤销，因此不会破坏历史流水。

每张设备卡片在设备注册 `wxchannels.fetch` 时提供定向方法调用测试。获取成功后，可以把结果作为 `args` 提交给任意在线且注册 `download.create` 的设备。两个操作都调用同一个 `/admin/api/call` 接口。

## 本地开发

```bash
./internal/workers/bridge/dev.sh
```

Worker 默认监听 `http://127.0.0.1:8787`，Pages 默认监听 `http://127.0.0.1:8788`。管理页用户名为 `admin`，默认本地密码为 `local-bridge-admin-token`。

可以覆盖端口和 Token：

```bash
BRIDGE_WORKER_PORT=8797 \
BRIDGE_PAGES_PORT=8798 \
BRIDGE_TOKEN=my-local-token \
BRIDGE_ADMIN_TOKEN=my-local-admin-token \
WRANGLER_VERSION=latest \
./internal/workers/bridge/dev.sh
```

## 设备之间调用方法

本地 API 提供统一入口：

```http
POST /api/bridge/call
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

`target_device_id` 可省略，此时 Bridge 从在线、空闲且注册了该 `method` 的设备中选择一个执行。增加新方法时，只需要设备端注册处理函数，不需要修改 Bridge 的任务类型定义。

现有业务接口是统一调用的便捷适配层。例如，设备 A 可以把视频号解析交给设备 B：

设备 A 可以把视频号解析交给设备 B：

```http
POST /api/bridge/tasks/wxchannels
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

省略 `target_device_id` 时，Bridge 自动选择在线、空闲且注册了 `wxchannels.fetch` 的设备。

向指定设备创建下载任务：

```http
POST /api/bridge/tasks/download
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

## Bridge 对外 CGI 接口

面向使用者的完整接入流程、任务状态说明和多语言代码示例见 [`docs/feature/bridge.md`](../../../docs/feature/bridge.md)。

外部系统可以把整个 Bridge 当作一个 CGI 节点。查询在线设备和方法：

```http
GET /v1
Authorization: Bearer <CALL_TOKEN>
```

查询当前 Token 的积分：

```http
GET /v1/credits
Authorization: Bearer <CALL_TOKEN>
```

外部 Token 每创建一个同步或异步调用消耗 `1` 积分。扣费与任务创建位于同一个 SQLite 事务；余额不足返回 `402 Payment Required`，无效请求不扣费，相同 `idempotency_key` 的异步重放不重复扣费。任务一旦创建，设备执行失败、内部重试或 `/v1/invoke` 超时均不退分。设备 Secret 与管理员控制台调用不计费。

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

不指定 `target_device_id` 时，外部调用方只依赖 Bridge 提供的方法，不需要了解内部设备。指定时，Bridge 会确认目标设备在线并注册了该方法后再调度。

提交响应中的任务 ID 可以继续使用同一个调用 Token 查询：

```http
GET /v1/tasks/<task-id>
Authorization: Bearer <CALL_TOKEN>
```

`GET /v1/tasks` 也只返回当前调用 Token 发布的任务。调用方不能通过 query 或 header 改写自己的发布方身份。

## 本地 API

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/bridge/status` | 当前设备到个人 Bridge 的连接状态 |
| `POST` | `/api/bridge/call` | 使用 `method + args` 提交任意方法调用 |
| `POST` | `/api/bridge/tasks/wxchannels` | 提交视频号解析任务 |
| `POST` | `/api/bridge/tasks/download` | 向指定设备提交下载任务 |
| `GET` | `/api/bridge/tasks/:id` | 查询任务、content/result 和错误 |
| `GET` | `/api/bridge/tasks?status=&limit=` | 查询当前设备发布的任务 |

## 旧配置迁移

升级后，旧外部调用方不应继续使用共享 `BRIDGE_TOKEN`。请先用 `BRIDGE_ADMIN_TOKEN` 登录管理页，为每个调用方创建独立调用 Token，再替换其 `Authorization`。设备配置中的 `bridge.token` 保持不变。

从不含积分字段的版本升级时，已有动态调用 Token 会保留，但初始积分余额为 `0`。升级部署后应先在管理页为这些 Token 充值，再恢复外部调用；任务和 Token 的既有身份不会改变。

旧版只有一个 `bridge.instances` 项时仍可临时连接，新版会使用其中的 `url`、`clientId` 和 token，并保留旧路径与旧协议字段兼容。旧 `capabilities` 布尔值会在迁移期映射为对应 methods；新配置请使用通用的 `bridge.methods`。请迁移为新的 `bridge.url`、`bridge.deviceId`、`bridge.deviceName` 和 `bridge.token`。

旧配置包含多个实例时会直接报错，因为单个操作系统设备现在只属于一个个人 Bridge；多 Bridge 管理由更高层应用负责。

从旧版多 `bridge.id` Worker 第一次升级时会启用新的单例 Durable Object。设备会在重连后重新登记，但旧 Durable Object 中的历史任务不会自动合并到新 Bridge。

## 可靠性和限制

- 相同发布方和 `idempotency_key` 只创建一个任务。
- 每个外部调用任务固定消耗 1 积分；余额不足返回 `402`，查询和轮询不消耗积分。
- 每台设备当前一次领取一个任务；同一操作系统内的视频号解析串行执行。
- 单次调用的 args 或 result 上限为 1 MiB。
- 完成和失败任务保留 7 天。
- 任务可能因租约过期再次执行，执行端必须允许重复调用。
- `BRIDGE_TOKEN` 是所有设备共享的高权限 Secret，只应写入设备配置；面向人员和外部系统必须分发独立调用 Token。
- 动态调用 Token 当前拥有全部在线 `methods` 的调用权限；更细粒度的方法授权和限流仍属于后续的 Bridge 市场或上层网关。

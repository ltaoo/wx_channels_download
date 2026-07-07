# AGENTS.md

本文件给后续 agent 快速了解项目。修改代码前先读本文件，再按需要打开具体目录内的 README、测试和相邻实现。

## 项目概览

这是一个以微信视频号下载为起点、现在扩展到多平台内容下载和管理的 Go 项目。核心能力包括：

- 运行本地代理，安装/使用根证书，拦截目标站点页面和 API 响应。
- 向微信视频号、公众号、知乎、小红书、B 站、YouTube、抖音、小说站等页面注入或配合前端脚本，记录浏览内容并创建下载任务。
- 提供本地 API、WebSocket、管理 GUI、任务列表、文件预览、RSS、文件传输助手、平台下载工作流等功能。
- 使用 Gopeed 下载引擎执行传统 HTTP 下载；新版平台下载使用 `pkg/contentplatform/download` 的 Probe/Resolve/Plan 工作流。
- 使用 SQLite/GORM 持久化账号、内容、浏览历史、下载任务和平台工作流。

入口是 `main.go`，模块名是 `wx_channel`，Go 版本目标为 `go 1.20`。

## 运行模式和主入口

- `main.go` 创建 `internal/config.Config`，然后调用 `cmd.Execute`。
- `cmd/root.go` 是默认本地模式：同时启动 `admin`、`api`、`interceptor` 三个服务。
- `cmd/server.go` 是服务器模式：只启动 API 相关能力，不启动本地代理注入。
- 其他 CLI 子命令在 `cmd/*.go`，例如 `download`、`decrypt`、`sph`、`update`、`uninstall`。
- 服务生命周期统一由 `internal/manager` 管理，服务 key 主要是 `admin`、`api`、`interceptor`。

默认本地模式的大致启动链路：

1. `main.go`
2. `cmd.Execute`
3. `cmd/root.go:root_command`
4. 初始化配置、日志、Velo app、SQLite/GORM 迁移
5. 创建 `admin.NewAdminServer`
6. 创建 `interceptor.NewInterceptorServer`
7. 创建 `api.NewAPIServer`
8. 注册各站点代理插件
9. 启动 admin、api、interceptor

## 目录定位

- `cmd/`: Cobra CLI 命令和本地/服务器模式启动逻辑。
- `internal/config/`: 配置 schema、默认值、配置文件加载/保存。新增配置要在 `LoadConfig` 中注册，并同步模板文件。
- `internal/manager/`: HTTP 服务抽象和服务状态控制。
- `internal/admin/`: 管理 GUI 服务和管理 API，前端由 `frontend` 目录或 embed FS 提供。
- `internal/api/`: Gin API、WebSocket、下载任务、平台工作流、兼容接口、文件预览、RSS 等。
- `internal/api/services/`: 数据库相关服务层，负责账号、内容、浏览历史、下载任务等业务持久化。
- `internal/database/`: 数据库接入、embed migration、GORM model。
- `internal/interceptor/`: 本地代理服务、系统代理/TUN 设置、证书使用、代理插件容器。
- `internal/platformbrowser/`: 通过代理注入脚本记录“用户打开过的内容”，主要写入账号和浏览历史；不是新版下载平台适配层。
- `internal/pipeline/`: 通用 pipeline 执行器，供平台下载工作流复用。
- `pkg/scraper/`: 各平台底层抓取/解析客户端和平台注册表，按平台子目录组织，例如视频号、公众号、优酷、知乎等。
- `pkg/contentplatform/`: 新版跨平台下载适配层。每个平台一个包，公共 contract 在 `pkg/contentplatform/download`。
- `pkg/gopeed/`: 本仓库内替换的 Gopeed 源码，`go.mod` 通过 replace 指向这里。
- `frontend/`: 管理 GUI 和注入脚本资源，使用原生 JS + Timeless/WEUI 等静态库；由 `frontend/assets.go` embed。
- `browser-extension/`: 浏览器扩展相关代码，不属于 Go 服务主链路。
- `docker/`: Webtop/Docker 运行环境脚本和镜像文件。
- `docs/`: 用户文档、架构文档、OpenAPI 片段、发布记录。
- `_example/`: 本地实验或端到端示例，不作为主产品入口。

## API 和下载任务

API 路由集中在 `internal/api/routes.go`。新增或修改接口时先确认是否属于以下哪条链路。

### 新版平台下载工作流

入口在 `internal/api/handler_platform_task.go`：

- `POST /api/task/pipeline/start`: 根据 URL 匹配平台 handler，执行 Probe，生成用户确认表单。
- `POST /api/task/pipeline/resume`: 接收用户选择，执行 Resolve，创建任务并按平台 Plan 下载/处理。
- `GET /api/task/pipeline/workflow`: 查询工作流状态。

平台 handler 注册在 `APIClient.platformDownloadRouter()`。新增平台时：

1. 在 `pkg/contentplatform/<name>/` 新增 handler。
2. 实现 `download.Handler`: `Platform`、`Match`、`Probe`、`Resolve`、`Plan`。
3. 如需自定义协议，新增 `download.SourceExecutor` 并在创建 `contentdownload.NewDownloader` 时注册。
4. 在 `APIClient.platformDownloadRouter()` 注册 handler。
5. 增加聚焦测试，至少覆盖 `Match`、`Probe`/解析、`Resolve`。

详细约束见 `pkg/contentplatform/README.md`。关键点：Probe 阶段抓取的大对象放在 `Probe.Internal`，不要序列化给前端；跨平台展示字段放 `Content.Summary`；平台特有 ID 放 `Content.Metadata` 或 `ResolvedRequest.Labels`。

### 传统/兼容下载任务

入口主要在 `internal/api/handler_tasks.go` 和 `internal/api/handler_download_task.go`：

- `/api/task/create` 仍兼容视频号原始 `ChannelsObject`、普通 URL 分发、部分旧平台入口。
- `/api/task/create_batch`、`/api/task/create_channels` 是视频号/旧任务路径。
- `/api/download_task/*` 是兼容下载任务 API。
- `internal/api/services/DownloadService` 包装 Gopeed 下载器。

不要把新版平台下载逻辑塞回旧 `build_task_body` 风格的前端处理。新平台优先走 Probe/Resolve/Plan。

## 代理注入和浏览记录

本地代理在 `internal/interceptor`，底层通过 `internal/interceptor/proxy` 和 `github.com/ltaoo/echo` 插件能力工作。

- 视频号注入插件在 `pkg/scraper/wxchannels/interceptor_plugin.go`，会改写 HTML/JS、注入 `frontend` 资源、接收 `__wx_channels_api/*` 回调。
- 公众号注入在 `pkg/scraper/officialaccount`。
- 知乎/小红书/B 站/YouTube/微博的“打开内容记录”插件在 `internal/platformbrowser/<platform>`。
- `internal/platformbrowser` 只负责浏览记录、账号 upsert、cookie 捕获等页面侧行为；真正下载适配应放在 `pkg/contentplatform/<platform>`。

## 数据库和迁移

- Model 在 `internal/database/model`，通用时间戳使用 `model.Timestamps`。
- 迁移 SQL 在 `internal/database/migrations`，通过 `internal/database/embed.go` 嵌入并由 Velo/GORM 初始化调用。
- 内容域核心表包括 `platform`、`account`、`content`、`content_account`、`browse_history`、`download_task`、`workflow` 等。
- 数据库写入尽量放在 `internal/api/services` 或已有 service/helper 中，避免 handler 里散落重复 upsert 逻辑。
- 新增字段需要同步 model、migration、兼容查询/序列化和测试。

## 配置

- 配置项注册在 `internal/config/config.go` 的 `LoadConfig`，schema 定义在 `internal/config/schema.go`。
- `internal/api/config.go` 把 Viper 配置转换为 API 运行时配置，并处理路径展开。
- 新配置项通常还要同步 `internal/config/config.template.yaml` 和相关文档。
- `go.mod` 中 `github.com/ltaoo/velo => ../velo` 指向工作区外目录；不要无意改动 replace 或运行会重排大量依赖的命令。

## 前端

- `frontend` 不是 React/Vue/Vite 项目。它主要是静态 HTML、原生 ES module、全局注入脚本和 Timeless/WEUI 等 UMD 库。
- 管理 GUI 入口在 `frontend/src/index.js`，页面在 `frontend/src/pages/**`，状态在 `frontend/src/store`。
- 注入脚本入口包括 `frontend/src/utils.js`、`downloaderv2.js`、`home.js`、`feed.js`、`live.js`、`profile.js`、`officialaccount.js`。
- embed 清单在 `frontend/assets.go`。新增被 Go 注入使用的静态资源时，要确认 `go:embed` 和 `Assets` 都已补齐。
- 前端 lint 配置在 `frontend/eslint.config.mjs`，命令是 `cd frontend && npm run lint`。
- 不要为了小改动引入新的前端构建系统、包管理器或框架。

## 代码风格

- Go 代码必须 `gofmt`。导入分组按标准库、第三方、本模块分组。
- 保持已有命名风格。本仓库历史上有 snake_case 局部变量和中文错误文案，局部改动优先贴合相邻代码。
- Handler 负责参数解析、错误响应和流程编排；可复用业务逻辑放 service/helper；平台解析放对应 `pkg/contentplatform/<platform>` 或 `pkg/<platform>`。
- 不要把 OS/系统代理逻辑放进 `pkg/contentplatform`。OS 辅助逻辑放 `pkg/platform`、`pkg/system` 或 `internal/interceptor`。
- 不要把内容平台适配放进 `pkg/platform`。下载平台适配统一放 `pkg/contentplatform`。
- 不要在 API 响应里泄露 cookie、token、完整请求头或 Probe/Internal 中的大对象。
- 避免在 tests 中依赖真实外网。需要外部页面时优先用 fixture、httptest 或已有 sandbox/CDP 抽象。
- 修改下载任务、工作流、数据库迁移、代理注入时，优先补对应单元测试或 handler 测试。
- 工作区可能有用户未提交修改。修改前看 `git status --short`，只改本任务需要的文件，不回退不相关变更。

## 常用验证命令

- 全量 Go 测试：`go test ./...`
- 聚焦 API 测试：`go test ./internal/api`
- 聚焦平台下载：`go test ./pkg/contentplatform/...`
- 聚焦视频号：`go test ./pkg/scraper/wxchannels ./internal/api`
- 前端 lint：`cd frontend && npm run lint`
- 格式化 Go 文件：`gofmt -w <files>`

如果测试需要联网、Docker、系统代理、证书安装、GUI 或外部目录写入，先明确风险并获得用户确认。

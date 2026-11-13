# Migrate Gopeed

仅保留在 `feat/migration` 分支，用于一次性迁移历史 Gopeed 数据；主版本不包含此工具。打包后的单个二进制已内嵌页面和新版数据库创建逻辑，运行时不需要源码。

读取 Gopeed `gopeed.db`，结合缓存 JSON 或详情接口，在当前版本重新创建下载记录、内容详情、账号和内容资产关联。

迁移复用现有同步工具的 `services.RecreateArchivedTask`，不会启动下载。文件继续使用原路径；迁移工具和主服务必须能访问该路径。操作前备份目标数据库；有详情时，同一内容或账号会按主应用规则更新。

## Run

从仓库根目录进入独立模块运行（需要 Go 1.24 或以上）：

```bash
cd migrate-gopeed
go run . --data-dir ../gopeed.db --port 8026 --target-url http://127.0.0.1:2022 --target-db ../data.db
```

本机 Go 版本较低时，可将 `go run` 替换为 `mise exec go@1.25 -- go run`。

请按目录运行整个包，不要使用 `go run migrate-gopeed/main.go`；单文件模式不会包含同包的其他 Go 文件。

打开 `http://127.0.0.1:8026/migration`。

普通构建默认 `--mode debug`，等同于 `dev`，前端资源由 `github.com/ltaoo/velo/frontendserver` 直接从本地 `web` 目录读取，修改 `web/index.html`、`web/src` 或 `web/assets` 后刷新浏览器即可看到效果。
如果从其他目录启动，可以通过 `--frontend-root /path/to/migrate-gopeed/web` 指定前端目录。使用 `embed_frontend_inject`、`embed_inject`、`release` 或 `prod` build tag 打包时会默认进入 `release`，直接使用编译进二进制的 embedded frontend；也可通过 `-ldflags "-X main.Mode=release"` 固定。

```bash
go build -tags embed_frontend_inject,sqlite_only -o migrate-gopeed .
```

`--data-dir` 指向要读取的 Gopeed `.db` 数据库文件。页面和迁移接口只接受显式的 `.db` 文件路径，不再从目录中隐式查找 `gopeed.db`。
`--target-url` 指向当前 wx_channels_download 服务地址，用于读取视频号详情。
`--target-db` 指向 wx_channels_download 的 SQLite 数据库；未指定时会优先尝试当前目录或父目录下的 `data.db`。

## Migration

执行迁移时会读取 Gopeed task labels 中的 `oid` 和 `uid`，并兼容旧字段 `id` 和 `nonce_id`。获取视频号详情需要 `oid`；`uid` 缺失时仍会调用目标 profile API，由目标服务处理。
视频号详情会按 `labels.id` 缓存在 Gopeed 数据目录的 `.gopeed_migration_profile_cache.json`；相同 `labels.id` 再次查看详情或迁移时会直接命中缓存，不再重复请求 `/api/channels/feed/profile`。

单个任务迁移流程：

1. 优先读取 `.gopeed_migration_profile_cache.json` 中的详情 JSON；命中时不请求接口。
2. 未命中时调用目标服务 `GET /api/channels/feed/profile?oid=<oid>&nid=<uid>&uid=<uid>` 并缓存结果；公众号任务同样支持详情缓存和在线获取。
3. 使用当前平台适配器生成内容、详情和资源资产信息，再用 Gopeed 记录恢复文件路径、创建时间与已下载状态。
4. 使用当前主应用的 schema 打开目标 SQLite 数据库，在事务中调用 `services.RecreateArchivedTask`，走与普通创建和同步相同的记录保存流程。
5. 只关联 Gopeed 已有的文件，不额外下载详情中的封面或其他资源。缺失文件或未完成的任务以暂停状态保存。
6. 任务元数据保留 `gopeedid`，重复迁移会跳过已迁移记录；创建失败时回滚该任务的全部数据库变更。

缺少详情时沿用原工具行为：创建没有内容关联的独立下载记录，并标记 `detail_missing`。已迁移记录后续不会自动补齐详情。

`--target-db` 是迁移工具所在机器可访问的 SQLite 路径；`--target-url` 仅用于获取详情。需要把文件和记录传到另一台服务器时，先完成本地迁移，再使用主应用现有的同步功能。


## 远端服务迁移

部署在 linux 服务器上没有视频号无法获取详情

先在服务器启动一个迁移服务

```bash
./migrate-gopeed -profile-agent-token abc --host 0.0.0.0 -port 8026
```

然后在有视频号的电脑上启动迁移服务

```bash
./migrate-gopeed -profile-agent -profile-agent-url ws://192.168.1.118:8026/ws/channels/profile-agent -profile-agent-token abc -profile-agent-local-url http://127.0.0.1:2022
```

`profile-agent-url` 就是部署在服务器上的迁移服务地址
`profile-agent-token` 验证用的
`profile-agent-local-url` 是运行在有微信电脑上的下载器服务地址，由代理调用它获取详情。


## API

- `GET /migration`
- `POST /api/v1/migration/load`
- `POST /api/v1/migration/table`
- `POST /api/v1/migration/execute`
- `POST /api/v1/migration/file/list`
- `POST /api/v1/fs/list`
- `GET /api/v1/migration/common_dirs`
- `GET /api/channels/feed/profile`

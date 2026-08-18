# Migrate Gopeed

独立的 Gopeed `gopeed.db` 任务查看应用，用于迁移前预览旧下载任务。

## Run

从仓库根目录运行：

```bash
go run ./migrate-gopeed --data-dir ./gopeed.db --port 8026 --target-url http://127.0.0.1:2022 --target-db ./data.db
```

也可以进入迁移工具模块运行：

```bash
cd migrate-gopeed
go run . --data-dir ../gopeed.db --port 8026 --target-url http://127.0.0.1:2022 --target-db ../data.db
```

请按目录运行整个包，不要使用 `go run migrate-gopeed/main.go`；单文件模式不会包含同包的其他 Go 文件。

打开 `http://127.0.0.1:8026/migration`。

普通构建默认 `--mode debug`，等同于 `dev`，前端资源由 `github.com/ltaoo/velo/frontendserver` 直接从本地 `web` 目录读取，修改 `web/index.html`、`web/src` 或 `web/assets` 后刷新浏览器即可看到效果。
如果从其他目录启动，可以通过 `--frontend-root /path/to/migrate-gopeed/web` 指定前端目录。使用 `embed_frontend_inject`、`embed_inject`、`release` 或 `prod` build tag 打包时会默认进入 `release`，直接使用编译进二进制的 embedded frontend；也可通过 `-ldflags "-X main.Mode=release"` 固定。

```bash
go build -tags embed_frontend_inject -o migrate-gopeed .
```

`--data-dir` 指向要读取的 Gopeed `.db` 数据库文件。页面和迁移接口只接受显式的 `.db` 文件路径，不再从目录中隐式查找 `gopeed.db`。
`--target-url` 指向当前 wx_channels_download 服务地址，用于读取视频号详情。
`--target-db` 指向 wx_channels_download 的 SQLite 数据库；未指定时会优先尝试当前目录或父目录下的 `data.db`。

## Migration

执行迁移时会读取 Gopeed task labels 中的 `oid` 和 `uid`，并兼容旧字段 `id` 和 `nonce_id`。`oid` 必需；`uid` 缺失时仍会调用目标 profile API，由目标服务处理。
视频号详情会按 `labels.id` 缓存在 Gopeed 数据目录的 `.gopeed_migration_profile_cache.json`；相同 `labels.id` 再次查看详情或迁移时会直接命中缓存，不再重复请求 `/api/channels/feed/profile`。

单个任务迁移流程：

1. 调用目标服务 `GET /api/channels/feed/profile?oid=<oid>&nid=<uid>&uid=<uid>` 获取视频号详情。
2. 将返回的 `data.data.object` 转成 `Content`、`Account` 和内容扩展记录。
3. 使用 velo 按主应用迁移方式打开目标 SQLite 数据库。
4. 直接写入 `download_task`、`download_resource`、`download_endpoint`、`content`、`account`、`content_account` 等记录，不启动下载流程。
5. `download_task.config_json` 只记录 `download_dir`、`.mp4` 文件的 `suffix`、以及 labels 中已有的 `spec`。
6. `download_task.metadata_json` 只记录 `gopeedid`；重新迁移时按该字段跳过已迁移的 Gopeed 记录。


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
`profile-agent-local-url` 这个有必要吗?


## API

- `GET /migration`
- `POST /api/v1/migration/load`
- `POST /api/v1/migration/table`
- `POST /api/v1/migration/execute`
- `POST /api/v1/migration/file/list`
- `POST /api/v1/fs/list`
- `GET /api/v1/migration/common_dirs`
- `GET /api/channels/feed/profile`

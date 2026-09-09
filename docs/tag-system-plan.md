# 标签系统改造：扁平标签 + 多对多关联

## Context

目前 `content` 表的 `tags` 字段以 JSON 数组字符串存储标签，仅在适配器层写入，从未被查询或展示。需要改造为独立的关系型标签体系，支持 Content 和 Account 两种实体打标签，并能按标签筛选。前端需要在内容列表的每行中加入 TagSelect 组件，支持多选、搜索、空格创建新标签。

---

## 一、后端

### 1. Migration（追加到 000003）

**`internal/database/migrations/000003_flow_schedule.up.sql`** — 末尾追加：

```sql
-- 标签表
CREATE TABLE IF NOT EXISTS `tag` (
    `id`         INTEGER PRIMARY KEY AUTOINCREMENT,
    `name`       TEXT NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    `updated_at` INTEGER NOT NULL DEFAULT 0,
    `deleted_at` INTEGER,
    UNIQUE(`name`)
);
CREATE INDEX IF NOT EXISTS idx_tag_deleted_at ON `tag` (`deleted_at`);

-- 内容-标签关联表
CREATE TABLE IF NOT EXISTS `content_tag` (
    `content_id` TEXT    NOT NULL,
    `tag_id`     INTEGER NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`content_id`, `tag_id`)
);
CREATE INDEX IF NOT EXISTS idx_content_tag_tag_id ON `content_tag` (`tag_id`);

-- 账号-标签关联表
CREATE TABLE IF NOT EXISTS `account_tag` (
    `account_id` TEXT    NOT NULL,
    `tag_id`     INTEGER NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`account_id`, `tag_id`)
);
CREATE INDEX IF NOT EXISTS idx_account_tag_tag_id ON `account_tag` (`tag_id`);

-- 迁移已有 JSON tags 数据
INSERT OR IGNORE INTO `tag` (`name`, `created_at`, `updated_at`)
SELECT DISTINCT j.value, strftime('%s','now')*1000, strftime('%s','now')*1000
FROM `content`, json_each(content.tags) AS j
WHERE content.tags IS NOT NULL AND content.tags != '' AND content.tags != '[]';

INSERT OR IGNORE INTO `content_tag` (`content_id`, `tag_id`, `created_at`)
SELECT content.id, tag.id, strftime('%s','now')*1000
FROM `content`, json_each(content.tags) AS j
JOIN `tag` ON tag.name = j.value
WHERE content.tags IS NOT NULL AND content.tags != '' AND content.tags != '[]';
```

**`internal/database/migrations/000003_flow_schedule.down.sql`** — 开头追加：
```sql
DROP TABLE IF EXISTS `account_tag`;
DROP TABLE IF EXISTS `content_tag`;
DROP TABLE IF EXISTS `tag`;
```

### 2. Model — 新建 `internal/database/model/tag.go`

```go
type Tag struct {
    Id   int    `gorm:"primaryKey;autoIncrement" json:"id"`
    Name string `gorm:"not null;uniqueIndex:idx_tag_name" json:"name"`
    Timestamps
}

type ContentTag struct {
    ContentId string `gorm:"primaryKey" json:"content_id"`
    TagId     int    `gorm:"primaryKey" json:"tag_id"`
    CreatedAt int64  `json:"created_at"`
}

type AccountTag struct {
    AccountId string `gorm:"primaryKey" json:"account_id"`
    TagId     int    `gorm:"primaryKey" json:"tag_id"`
    CreatedAt int64  `json:"created_at"`
}
```

遵循现有惯例：每个 struct 有 `TableName()` 方法。

### 3. Service — 新建 `internal/services/tag.go`

`TagService` struct 持有 `db *gorm.DB`，构造函数 `NewTagService(db)`。

方法列表：
- `ListTags(keyword string)` → `[]Tag` — 模糊搜索，`deleted_at IS NULL`
- `CreateTag(name string)` → `*Tag` — 去重，返回已有或新建
- `DeleteTag(tagId int)` — 软删除标签 + 硬删除 content_tag/account_tag 关联
- `RenameTag(tagId int, newName string)` — 重命名
- `SetContentTags(contentId string, tagIds []int)` — 事务内：删旧关联 → 插新关联
- `SetAccountTags(accountId string, tagIds []int)` — 同上
- `GetContentTags(contentId string)` → `[]Tag`
- `GetAccountTags(accountId string)` → `[]Tag`
- `BatchGetContentTags(contentIds []string)` → `map[string][]Tag` — 列表页批量查询

### 4. API Handler — 新建 `internal/api/handler_tag.go`

| Method | Path | Handler | 说明 |
|--------|------|---------|------|
| GET | `/api/tag/list` | `handle_tag_list` | 查询所有标签，支持 `keyword` |
| POST | `/api/tag/create` | `handle_tag_create` | `{ "name": "..." }` |
| POST | `/api/tag/delete` | `handle_tag_delete` | `{ "id": 1 }` |
| POST | `/api/tag/rename` | `handle_tag_rename` | `{ "id": 1, "name": "..." }` |
| POST | `/api/tag/content/set` | `handle_tag_content_set` | `{ "content_id": "...", "tag_ids": [1,2] }` |
| POST | `/api/tag/account/set` | `handle_tag_account_set` | `{ "account_id": "...", "tag_ids": [1,2] }` |

### 5. 注册路由和注入 Service

- **`internal/api/routes.go`** — 在 `SetupRoutes` 中注册上述路由
- **`internal/api/client.go`** — `APIClient` 增加 `tag_service *services.TagService`
- **`internal/api/server.go`** — `NewAPIServer` 参数增加 `tag_service`
- **`internal/application/start.go`** — 创建 `TagService` 并传入

### 6. 修改现有接口

- **`internal/services/content.go`**：
  - `ContentListOptions` 增加 `TagIDs []int`
  - `ListContents` 的 `build_query` 增加：当 `TagIDs` 非空时 `JOIN content_tag` 过滤
  - `ContentListItem` 增加 `Tags []TagRecord` 字段（id + name）
  - `ListContents` 返回结果时批量查询 content tags 并填充
- **`internal/api/handler_content.go`**：
  - `handle_content_list` 增加 `tag_ids` query 参数（逗号分隔整数）
  - `handle_content_detail` 的返回 gin.H 中增加 `"tags"` 字段

---

## 二、前端

### 7. 新增 TagSelect 组件 — `frontend/src/components.js`

在现有 `components.js` 中新增 `TagSelect` 组件，交互行为：
- 类似 antd 的 `Select mode="tags"`：Popover 内展示可选标签列表
- 点击 + 图标弹出 Popover
- Popover 内有搜索框，输入关键词实时过滤标签列表
- 输入空格（或回车）时，如果没有完全匹配的标签，自动创建新标签（调 `POST /api/tag/create`）
- 已选标签显示为 Tag 徽章，点击 x 可移除
- 选择/取消选择后立即调 `POST /api/tag/content/set` 保存

需要的 API 请求：
- `GET /api/tag/list?keyword=xxx` — 搜索标签
- `POST /api/tag/create` — 创建新标签
- `POST /api/tag/content/set` — 保存关联

组件 props：`{ contentId, tags (初始标签数组), onUpdate (回调) }`

### 8. 修改 content.js — 内容列表行增加标签

- **`ContentRowMain`** 函数中，在 `content-row-badges` 容器内，紧跟 `PlatformTag` 和 `Tag(type)` 之后，渲染已有标签 + TagSelect 的 + 按钮
- 内容列表数据 (`normalize_content_item` in `content.model.js`) 增加 `tags` 字段解析
- 点击 + 按钮时打开 TagSelect Popover

### 9. 修改 content.model.js

- `normalize_content_item` 增加 tags 字段提取
- 不需要额外的列表级标签加载请求（后端 `ListContents` 直接返回 tags）

### 10. 样式 — `frontend/src/pages/content.css`

- 标签徽章样式（复用 `.dm-tag` 基础样式，可加特定颜色区分用户标签）
- TagSelect Popover 样式（搜索框 + 标签列表 + 创建入口）

---

## 文件变更汇总

| 文件 | 操作 |
|------|------|
| `internal/database/migrations/000003_flow_schedule.up.sql` | 追加建表 + 数据迁移 SQL |
| `internal/database/migrations/000003_flow_schedule.down.sql` | 追加 DROP 表 |
| `internal/database/model/tag.go` | **新建** — Tag, ContentTag, AccountTag |
| `internal/services/tag.go` | **新建** — TagService |
| `internal/api/handler_tag.go` | **新建** — 标签 API handlers |
| `internal/api/routes.go` | 修改 — 注册标签路由 |
| `internal/api/client.go` | 修改 — 注入 tag_service |
| `internal/api/server.go` | 修改 — 构造参数增加 tag_service |
| `internal/application/start.go` | 修改 — 创建并传入 TagService |
| `internal/services/content.go` | 修改 — ListContents 增加 tag_ids 过滤，返回 tags |
| `internal/api/handler_content.go` | 修改 — content list/detail 增加 tags |
| `frontend/src/components.js` | 修改 — 新增 TagSelect 组件 |
| `frontend/src/pages/content.js` | 修改 — ContentRowMain 增加标签展示和 TagSelect |
| `frontend/src/pages/content.model.js` | 修改 — normalize 增加 tags 字段 |
| `frontend/src/pages/content.css` | 修改 — 标签相关样式 |

## 验证方式

1. 启动项目，确认 migration 自动执行、三张新表创建成功
2. 检查已有 JSON tags 数据已迁移到 tag + content_tag 表
3. API 测试：`GET /api/tag/list`、`POST /api/tag/create`、`POST /api/tag/content/set`
4. 前端：内容列表中每行显示已有标签，点击 + 弹出 TagSelect，搜索/创建/选择标签后保存
5. 内容列表的标签筛选功能（toolbar 中可按标签过滤）

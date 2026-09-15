# Phase 3 — 工具元数据下沉到 Go：吸收 `servicetools`

> 状态：待执行（plan，未落地）
> 分支：`feat/tag`
> 前置：Phase 2 已完成 `internal/mcpserver/registry.go` 的声明式分发

---

## 1. 目标

Phase 2 让**分发**变成声明式，但工具的**元数据**仍独立存在于
`internal/services/tools/catalog.json`（44 条，2214 行）。

于是「新增一个工具」仍然要改两个互不相关的文件：
- `registry.go` 里的 `tool_declarations` 行；
- `catalog.json` 里的 JSON 条目。

而且「已宣传但没有路由」这种不一致只有在运行时由 `validate_tool_registry()` 才会被发现。

**本阶段目标**：每条工具收敛成**一处 Go 声明**，同时携带
`name + title + description + schema + annotations + supports + handle`，
按领域切片放在各自 handler 旁边。删除 `catalog.json` 与整个
`internal/services/tools` 包。新增工具变成**单文件编辑**。

### 收益的诚实边界

- **彻底消除**「元数据 ↔ handler 跨文件漂移」这一类错误：工具只存在于其 handler
  被具名的地方（method expression，因此 handler 改名/删除会**编译报错**），
  不存在第二个宣传它的地方。
- **不是**全量编译期保证：漏写一行、或新增一个 `registry.go` 没有拼接的切片，
  依然能编译通过。这类问题由 `NewToolSet` 里的 `validate_tool_registry()`
  和完整性测试（见 §7）兜底。

### 已核实、使合并安全的事实

- `catalog.json` **仅**被 `//go:embed` 读取（`internal/services/tools/service.go:17`）。
  没有 `os.ReadFile`，没有 CI/docs/前端引用。没有任何地方把它当文件读。
- 只有 7 个文件 import `internal/services/tools`，全部在 §6 列出。

---

## 2. 目标形态

```go
// registry.go
type tool struct {
    name         string
    title        string
    description  string
    input_schema json.RawMessage // 精确 JSON 对象，包初始化时一次性物化
    annotations  json.RawMessage // nil ⇒ 由 Definition.Annotations 的 omitempty 省略
    supports     func(*ToolSet) bool
    handle       tool_handler
}

// 按 catalog 顺序拼接——这个列表就是注册点
var tool_declarations = concat_tools(
    core_tools, scraper_tools, wxchannels_tools, wxchannels_download_tools,
    sph_tools, zhihu_tools, automation_tools, data_tools,
)
```

```go
// 例如 data_tools.go —— 紧挨着它具名的 handler
var data_tools = []tool{
    {"get_download_tasks", "获取下载任务", "…", json.RawMessage(`{…}`), json.RawMessage(`{…}`), supports_data, (*ToolSet).get_download_tasks},
    {"get_accounts", "获取账号列表", "…", json.RawMessage(`{…}`), json.RawMessage(`{…}`), supports_data, (*ToolSet).get_accounts},
}
```

### 领域切片与顺序（必须精确——这就是 `tools/list` 的顺序）

| 文件 | 变量 | 工具 |
|---|---|---|
| `tools.go` | `core_tools` | get_config, update_config, get_restart_status, get_platform_status, fetch_content, download_content, decrypt_wxchannels_video |
| `scraper_tools.go` | `scraper_tools` | create_scraper_job, get_scraper_job |
| `wxchannels_tools.go` | `wxchannels_tools` | get_wxchannels_status … get_wxchannels_video_share_url（11 个） |
| `wxchannels_download_tools.go` | `wxchannels_download_tools` | download_wxchannels_live, download_wxchannels_video |
| `sph_tools.go` | `sph_tools` | deploy_sph_worker |
| `zhihu_tools.go` | `zhihu_tools` | 7 个 zhihu 工具（复用已有 `*_tool_name` 常量） |
| `automation_tools.go` | `automation_tools` | 6 个 automation 工具（复用已有 `*_tool_name` 常量） |
| `data_tools.go` | `data_tools` | get_download_tasks, get_download_task_detail, delete_download_tasks, create_download_task, get_accounts, get_browse_history, get_logs, get_certificate_status |

### 44 条完整顺序（有序断言用）

```
get_config, update_config, get_restart_status, get_platform_status,
fetch_content, download_content, decrypt_wxchannels_video,
create_scraper_job, get_scraper_job,
get_wxchannels_status, search_wxchannels_accounts, get_wxchannels_account_videos,
get_wxchannels_live_replays, get_wxchannels_live_profile, get_wxchannels_interacted_videos,
get_wxchannels_followed_accounts, get_wxchannels_play_history, get_wxchannels_video_profile,
get_wxchannels_video_comments, get_wxchannels_video_share_url,
download_wxchannels_live, download_wxchannels_video,
deploy_sph_worker,
get_zhihu_credential_status, get_my_zhihu_collections, get_zhihu_collection_contents,
get_my_zhihu_answers, get_my_zhihu_posts, get_my_zhihu_zvideos, get_my_zhihu_columns,
list_automation_schedules, get_automation_schedule, create_automation_schedule,
toggle_automation_schedule, trigger_automation_schedule, list_automation_runs,
get_download_tasks, get_download_task_detail, delete_download_tasks, create_download_task,
get_accounts, get_browse_history, get_logs, get_certificate_status
```

### 新建 `internal/mcpserver/definition.go`

从 `internal/services/tools/service.go` **逐字搬移**：

| 符号 | 原位置（service.go） |
|---|---|
| `ErrUnknownTool` | 15 |
| `Definition` | 21-28 |
| `FormField` | 31-44 |
| `FormOption` | 47-50 |
| `form_schema` | 208-278 |
| `form_options` | 280-295 |
| `json_object` | 297-313 |

外加新函数 `decode_json_object(json.RawMessage) map[string]any`（空 ⇒ nil）。

> `Definition` 的 JSON tag 与字段顺序必须**原样保留**——它是 `tool list`
> 和 `/api/v1/automation/list_flow_nodes` 的线上契约。

---

## 3. 执行前：抓取基线（HEAD，未改动）

在动任何代码前执行，产物留在 `/tmp`：

```bash
cd /Users/mayfair/Documents/other/wx_channels_download

# V1/V2/V3 用的二进制基线
go build -o /tmp/dm_before .

# V1：37 个 enabled 工具（过滤 LD_DYSYMTAB stderr 行）
/tmp/dm_before tool list 2>/dev/null > /tmp/a.json

# V1b：全部 44 个（用 write_tool_json 的设置：SetIndent("","  ") + SetEscapeHTML(false)）
#   —— 用下面的临时程序把 mcpserver.ToolCatalog() 落盘为 /tmp/a_all.json
```

V1b 临时程序（放在 `declgen/`，与生成器同目录，最后一起删）：

```go
// declgen/main.go（先只做 dump，后加生成逻辑；或拆两个文件）
package main

import (
    "encoding/json"
    "os"

    "wx_channel/internal/mcpserver"
)

func main() {
    f, _ := os.Create("/tmp/a_all.json")
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    enc.SetEscapeHTML(false)
    _ = enc.Encode(mcpserver.ToolCatalog())
}
```

---

## 4. 执行步骤

### Step 1 — 基线（§3）

### Step 2 — 新增 `definition.go`

搬移 §2 列出的符号。`Definition` / `FormField` / `FormOption` 保持字段顺序与 tag 不变。
`ErrUnknownTool = errors.New("未知工具")`。

### Step 3 — 用一次性生成器产出 8 个 `*_tools` 块

编写 **一次性、不提交** 的 `declgen/main.go`（stdlib `encoding/json` + `go/ast`），
在 Step 8 删除：

- 解析 `internal/services/tools/catalog.json` → 44 ×
  `{Name, Title, Description, InputSchema json.RawMessage, Annotations json.RawMessage}`。
- 用 `go/ast` 解析 `internal/mcpserver/registry.go`，逐行提取 `supports` 与 `handle`
  **表达式原文**（保留 5 个 context-only 工具的 `without_arguments(...)` 包裹）。
- 断言两边长度都是 44，且逐位对齐：handle 方法名 == catalog name，或 == catalog name + `_tool`
  （今天全部 44 条都满足）。任何错位都大声失败，而不是悄悄错配元数据。
- 判定每条工具的归属文件：在 `internal/mcpserver/*.go` 里找谁声明了
  `func (s *ToolSet) <method>(`；按文件分组，输出 `// === FILE: x.go ===` 分隔的块。
- 输出 `json.RawMessage(`…`)` 转换（裸 raw string **不能**赋给 `json.RawMessage`）；
  title/description 用反引号 raw string，若内容含反引号则回退 `strconv.Quote`。

把生成的 8 个块分别粘贴进对应文件。

### Step 4 — 重写 `registry.go`

- `tool` 类型（§2）
- `concat_tools(groups ...[]tool) []tool`
- `build_tool_registry(declarations []tool) map[string]tool`
- `build_tool_definitions(declarations []tool) []Definition` —— 急切实例化，
  经 `tool.definition()` 生成 `[]Definition`（含 `FormSchema`）
- `validate_tool_registry()`：删掉 `BuiltinCatalog()` 漂移循环，改为
  `len(input_schema) > 0` 的逐行检查 + `len(tool_definitions)==len(tool_declarations)`
- `supports_tool` / `execute_tool`：逻辑不变，`servicetools.ErrUnknownTool` → 本地 `ErrUnknownTool`
- 新增 `ToolSet.call`（见 §5，**行为关键**）

保留 `without_arguments` 适配器与 `supports_*` 谓词（逐字不动）。

### Step 5 — 改 `tools.go` facade 与 `toolset.go`

见 §6。
- `toolset.go`：`NewToolSet` 去掉 `NewBuiltin` 块；`Register` 用 `ToolCatalog()` + `call`。
- `tools.go`：`ToolDefinition = Definition`；删 `ToolFormField`/`ToolFormOption`（死代码）；
  重写 `ToolCatalog`/`ToolNames`/`ExecuteTool`；删 `ToolService()`。
  保留 `tool_execution_error = mcp.ToolError`（**真别名**）。

### Step 6 — 更新消费者

`services/mcp.go`、`services/user_flow.go`、`cmd/tool.go`（§6）。

### Step 7 — 删除 `internal/services/tools/`

### Step 8 — `gofmt`、更新/新增测试、跑 §7 验证、删除 `declgen/`

---

## 5. 关键方法语义（务必照抄）

### `ToolSet.call` —— 行为关键

旧的 `Service.New` 从**已按 supports 过滤**的 definitions 构建 `by_name`
（`service.go:67,74-80`），`Call` 查 `by_name`（`service.go:141-143`）。
因此**已知但不支持**的工具会返回 `未知工具: X`。

必须靠 `supports_tool` 门控来精确复现，**而不是**查 `tool_registry` 原始成员：

```go
func (s *ToolSet) call(ctx context.Context, name string, raw_arguments json.RawMessage) (map[string]any, error) {
    if s == nil {
        return nil, errors.New("工具服务未初始化")
    }
    name = strings.TrimSpace(name)
    if name == "" {
        return nil, errors.New("工具名称不能为空")
    }
    if !s.supports_tool(name) {
        return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
    }
    if len(raw_arguments) == 0 {
        raw_arguments = json.RawMessage("{}")
    }
    return s.execute_tool(ctx, name, raw_arguments)
}
```

> 若图省事查 `tool_registry`，会让全部 44 个工具无视后端配置，经
> flowengine/CLI 都能调用。

### 保留签名

- `ToolSet.ToolCatalog()`：`tool_declarations` 按 `supports` 过滤，再按索引取
  `tool_definitions`。
- `ToolSet.ToolNames()`
- `ToolSet.ExecuteTool(ctx, string, map[string]any) (any, error)`：
  marshal → `call` → 解包 `structuredContent`，语义逐字同 `service.go:152-168`。
  仍服务于 `flowengine.RegisterServiceNode`：
  - `internal/application/mcp_stdio.go:197`
  - `internal/application/mcp.go:46`
- `mcpserver.ToolCatalog()`（全部 44）、`mcpserver.ToolNames()`

---

## 6. 消费者修改明细

| 文件:行 | 修改 |
|---|---|
| `mcpserver/toolset.go:12,49,80-84,118-120,121,130,132` | 去 `servicetools`；删 `tool_service` 字段与 `NewBuiltin` 块；`Register` 守卫 → `t == nil`；`Definitions()`→`ToolCatalog()`；`Call`→`call`；`ErrUnknownTool` → 本地 |
| `mcpserver/tools.go:14,18,19-20,94-96,100-105,108-113,117-122,126-131` | 去 `servicetools`；`ToolDefinition = Definition`；**删除** `ToolFormField`/`ToolFormOption`（死代码）；重写 `ToolCatalog`/`ToolNames`/`ExecuteTool`；**删除** `ToolService()` |
| `internal/services/mcp.go:12,154-167` | 去 `servicetools`；**删除** `MCPService.ToolCatalog()`（零调用者，已核实）。`Status()` 不动 |
| `internal/services/user_flow.go:11,29,94,555` | import → `wx_channel/internal/mcpserver`；`[]servicetools.Definition` → `[]mcpserver.Definition`；`BuiltinCatalog()` → `mcpserver.ToolCatalog()`（×2） |
| `cmd/tool.go:11,29,48,74,83` | import → `mcpserver`；`.Definitions()`→`toolset.ToolCatalog()`；`.Execute()`→`toolset.ExecuteTool()`；返回类型 `*mcpserver.ToolSet`；`ToolService()`→`toolset` |

无 import 环：改完后 `internal/mcpserver` 不 import 任何 `internal/services/*`，
而 `services` 本就 import `mcpserver`（`mcp.go:11`）。

### 参考：`cmd/tool.go` 目标片段

```go
func new_cli_tool_service(cmd *cobra.Command) (*mcpserver.ToolSet, error) {
    api_base_url := strings.TrimSpace(mcp_api_base_url)
    if api_base_url == "" {
        api_base_url = configured_api_base_url()
    }
    _, toolset, err := new_remote_tool_server(api_base_url, strings.NewReader(""), io.Discard, cmd.ErrOrStderr())
    if err != nil {
        return nil, err
    }
    return toolset, nil
}
```

`tool_list_cmd` → `write_tool_json(cmd.OutOrStdout(), toolset.ToolCatalog())`；
`tool_call_cmd` → `toolset.ExecuteTool(cmd.Context(), args[0], arguments)`。

---

## 7. 验证

| # | 检查 |
|---|---|
| V1 | HEAD 下 `/tmp/dm_before tool list > /tmp/a.json`；改动后 `/tmp/dm_after tool list > /tmp/b.json`；`diff /tmp/a.json /tmp/b.json` 为空（37 个 enabled 工具；过滤 `LD_DYSYMTAB` stderr 行） |
| V1b | 同上，但用 `write_tool_json` 的设置（`SetIndent("","  ")`、`SetEscapeHTML(false)`）从临时程序 marshal `mcpserver.ToolCatalog()`，在 HEAD 与改动后各抓一份，diff 全部 44 个 |
| V2 | 把 stdio 的 `initialize` + `tools/list` + `tools/call`(bogus) 喂给 `dm_before`/`dm_after mcp --api-base-url http://127.0.0.1:1`；响应来自每请求 goroutine，需逐行解析后**按 JSON-RPC `id` 排序**再 diff |
| V3 | 上面 id=3 必须精确为 `{"code":-32602,"message":"未知工具: bogus"}`，两侧一致 |
| V4 | `SortedUserFlowNodeCatalog()` JSON：ServiceNode 的 `tools[]` 字段顺序 `name,title,description,input_schema,annotations,form_schema`，且零属性工具是 `"form_schema":[]`（非 `null`） |
| V5 | `go build ./... && go vet ./...`；`grep -rn "internal/services/tools" --include='*.go' .` 为空；`grep -rn "wx_channel/internal" pkg/` 为空 |
| V6 | `go test ./internal/mcpserver/ -v` |

---

## 8. 测试（`internal/mcpserver/registry_test.go`；去掉 `servicetools` import）

- 用 `TestToolDeclarationsAreComplete` 替换 `TestToolDeclarationsCoverCatalog`：
  - 断言 `len(tool_declarations) == len(tool_registry) == len(tool_definitions) == 44`；
  - 逐行：name/title/description 非空、`len(input_schema) > 0`、`supports`/`handle` 非 nil；
  - 逐个派生 `Definition`：`InputSchema != nil && FormSchema != nil`；
  - 外加一份**有序**的期望名字切片（44 个短字符串）——便宜，且它是顺序与
    「某个切片从未被拼接」的守卫。
- 保留 `TestToolDeclarationNamesAreUnique`（调用 `validate_tool_registry()`）、
  `TestToolSupportsMatrix`（11 组配置，不动）、`TestExecuteToolUnknownName`
  （本地 `ErrUnknownTool`，文本 `未知工具: bogus`）。
- 新增 `TestExecuteToolRejectsUnsupported`：
  `(&ToolSet{}).ExecuteTool(ctx,"get_config",nil)` → `errors.Is(err, ErrUnknownTool)`。
  这是上面 `call` 语义的回归守卫。
- 新增 `TestToolCatalogSupportedCounts`：
  `(&ToolSet{}).ToolCatalog()` 长度 0；`(&ToolSet{api_client:&api_client{}}).ToolCatalog()` 长度 29。

> **不要**提交全 44 条的 JSON golden——那会把正在删除的 2214 行产物又造出来。
> 字节一致性由 V1/V1b 一次性确立；有序名字测试是永久的、便宜的守卫。

---

## 9. 注意事项（坑）

1. **`call` 里的 `supports` 门控**（不是查 registry 成员）——见 §5。
2. **`InputSchema` 在 `Definition` 与 `mcp.Tool` 里保持 `map[string]any`**。
   `encoding/json` 会排序 map key；`json.RawMessage` 会按 catalog 字节序输出，
   改变 `tool list`/`tools/list`/flow-node 的字节。原始 JSON **只**存在于内部的
   `tool` 声明里。
3. **`form_schema` 的 nil vs `[]`**：无 `properties` 时 `form_schema` 返回非 nil 的
   `[]FormField{}`，而 `Definition.FormSchema` 没有 `omitempty`。
   逐字复制那处 early return，否则零属性工具会从 `[]` 翻成 `null`。
4. **helper 命名 `concat_tools`** —— `concat` 已在 `registry_test.go:177` 声明。
5. **init 时急切实例化**：`build_tool_definitions` 在包级 var 初始化里跑，每个
   `Definition`（含 `FormSchema`）算一次并共享。当前无人修改它们（只编码）；
   把返回的切片标注为只读。
6. **不要加 `init()`**。var 初始化按依赖顺序跨文件进行，
   `*_tools` → `tool_declarations` → `tool_registry`/`tool_definitions` 是安全的。
7. **registry list 就是注册点**：新增一个 `registry.go` 没有拼接的 `*_tools` 变量
   会被静默忽略（完整性测试看不到它）。`tool_declarations` 的 doc 注释必须写明这点。
8. `validate_tool_registry()` 的 `BuiltinCatalog()` 漂移循环删除（没有 catalog 可漂移），
   换成 `len(input_schema) > 0` 的行检查 + `len(tool_definitions)==len(tool_declarations)`。
9. `"初始化工具服务失败"`（`toolset.go:82`）随 `NewBuiltin` 一起消失；
   没有消费者/测试断言它。

---

## 10. 关键文件清单

| 文件 | 变更 |
|---|---|
| `internal/mcpserver/definition.go` | **新增** — 搬移 `Definition`/`FormField`/`FormOption`/`form_schema`/`form_options`/`json_object`/`ErrUnknownTool` + `decode_json_object` |
| `internal/mcpserver/registry.go` | `tool` 类型、`concat_tools`、`build_tool_definitions`、`call`、校验 |
| `internal/mcpserver/{tools,scraper_tools,wxchannels_tools,wxchannels_download_tools,sph_tools,zhihu_tools,automation_tools,data_tools}.go` | 各 + 一个 `*_tools` 切片；facade 在 `tools.go` 重写 |
| `internal/mcpserver/toolset.go` | 去 `NewBuiltin`/`tool_service`；`Register` 走 `ToolCatalog()`+`call` |
| `internal/mcpserver/registry_test.go` | completeness + unsupported-rejection + counts |
| `internal/services/mcp.go`、`internal/services/user_flow.go`、`cmd/tool.go` | 重新指向 `mcpserver`；删死 API |
| `internal/services/tools/` | **删除**（`service.go` + `catalog.json`） |
| `declgen/` | **新增、一次性** — Step 8 删除，永不提交 |

---

## 11. 回滚

改动全部是源码级、无数据迁移。回滚：
```bash
git checkout -- internal/mcpserver internal/services cmd
git clean -fd internal/services/tools declgen
```
（`catalog.json` 与 `service.go` 均受 git 跟踪，`git checkout` 即可还原。）

---

## 附：基线快照命令（一键）

```bash
cd /Users/mayfair/Documents/other/wx_channels_download
go build -o /tmp/dm_before . && /tmp/dm_before tool list 2>/dev/null > /tmp/a.json
wc -l /tmp/a.json   # 期望 37 个工具
```

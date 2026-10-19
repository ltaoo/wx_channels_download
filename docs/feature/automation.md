---
title: Automation 实现逻辑与平台对比
description: 自动化流程的节点配置、触发、取消、恢复与重试，以及与 n8n、扣子和 Dify 的功能差异。
---

# Automation 实现逻辑与平台对比

实现与官方资料核对日期：**2026-10-18**。本项目以本机下载器的查询、采集、下载服务为基础，提供可视化 Pipeline、定时/事件/Webhook 触发与执行管理。本次参照 n8n、Dify 补齐可靠运行和通用节点能力，未引入 AI 模型/知识库节点。

## 1. 当前能力

- 手动运行返回已落库的真实运行 ID；运行队列最多容纳 128 条排队/执行中的任务，共享引擎最多同时执行 8 个顶层流程。
- 整体超时贯穿 HTTP、服务调用、JS、延时与子流程；取消后不再执行后继节点，也不会被成功状态覆盖。
- 人工等待保存完整上下文，支持重启后继续同一次运行；恢复前检查回复 schema 和原运行截止时间。
- 每次运行冻结用户流程及引用的子流程定义，支持用原定义或当前已保存定义从头重试。
- 节点重试、退避、错误分支、输入/输出 schema 可经过导入、画布、MCP 草稿和保存链路保留。
- HTTP 支持请求头、查询参数、JSON/form/raw 请求体、Basic/Bearer 认证、动态模板及可配置超时。
- 专用数组迭代、子流程、延时、主动报错节点，以及隔离变量的并行分支与一次汇聚。
- 应用事件桥接和带 Bearer 令牌的 Webhook；节点日志落库，页面刷新后仍可查看。

这些能力以单实例为边界。进程在节点执行途中退出时，不自动重放可能已经发生的外部操作。

## 2. 实现结构

```text
画布 / HTTP API / MCP 草稿工具
          │
          ├── UserFlow.definition：用户流程定义
          └── FlowSchedule：Cron / Event / Webhook 规则
                        │
      手动 / 到期扫描 / 事件总线 / Webhook
                        │
             prepare_run：校验、分配 ID、冻结定义、落库
                        │
             execute_prepared_run → FlowEngine
                        │
        节点执行 / 重试 / 路由 / 并行 / 子流程
                        │
       检查点 + 运行结果 + 节点日志 → 数据库
                        │
                  事件总线 → WebSocket
```

| 层次 | 主要代码 | 职责 |
| --- | --- | --- |
| 页面 | `frontend/src/pages/automation.model.js`、`flow.components.js`、`flow_edit.js`、`flow_detail.js` | 编辑、触发器、执行策略、运行记录、取消/确认/重试 |
| API | `internal/api/handler_automation*.go`、`handler_workflow.go`、`routes.go` | 流程、规则、运行、事件与 Webhook |
| 服务 | `internal/services/automation.go`、`automation_runtime.go`、`automation_events.go` | 调度、持久化、恢复、重试与事件匹配 |
| 定义校验 | `internal/services/user_flow.go`、`automation_nodes.go`、`user_flow_triggers.go` | 节点白名单、配置校验、子流程快照、触发器事务保存 |
| 内核 | `pkg/flowengine/engine/engine.go`、`runtime.go`、`ctx.go` | 驱动队列、取消、检查点、步骤预算与并行隔离 |
| 节点 | `pkg/flowengine/nodes/` | HTTP、服务工具、JS、表达式、分支、变量、人工等待、迭代、子流程、延时、主动失败 |
| 启动 | `internal/application/start.go`、`mcp.go` | 注册引擎与服务工具、加载用户流程、桥接事件、恢复中断状态并启动调度器 |

应用内工具通过 `ServiceNode` 直接调用进程内 MCP 工具执行器。关闭对外 MCP 监听不影响这条内部执行路径。

### 2.1 持久化对象

| 表 | 内容 |
| --- | --- |
| `user_flow` | 名称、说明、流程定义 JSON；节点、边、坐标和 schema 都在定义内 |
| `flow_schedule` | 流程 ID、Cron 表达式、初始参数、超时、下次时间和最近运行结果 |
| `flow_run_record` | 真实运行 ID、触发来源、输入快照、定义快照、截止时间、恢复检查点、节点尝试次数/输出、日志及最终状态 |

新增字段通过 `000005_automation_runtime` 数据库迁移接入。`platform_workflow_run` 仍属于平台下载工作流，与 automation 运行记录分开。

触发器类型及扩展参数保存在规则的 `initial_data.__automation` 中：

```json
{
  "creator_username": "example@finder",
  "__automation": {
    "type": "Event",
    "start_node": "start",
    "event_key": "downloadtask.finished",
    "active": true
  }
}
```

执行前移除 `__automation`。Cron 的启停使用规则 `enabled`；Event/Webhook 使用元数据 `active`。非 Cron 规则的 `cron_expr` 为存储占位值，不参与定时扫描。Webhook 令牌由服务端随机生成，保存在元数据中。

## 3. 编辑与保存

创建 Pipeline 时生成 `flow-u-...` ID 和开始节点。节点配置及执行策略先应用到画布草稿，再保存 Pipeline。一次保存同时更新流程图与触发器，数据库事务失败时全部回滚。

后端校验节点 ID、类型、边、分支目标、错误分支、并行汇聚目标、配置和作用域。单个流程最多 500 个节点，子流程最多嵌套 16 层。`retry`、`error_next_node`、`input_schema`、`output_schema` 均可往返保存；旧客户端省略执行策略时保留原值。关闭重试可设置 `max_attempts: 1`，清除错误分支可传空字符串。

执行策略示例：

```json
{
  "retry": { "max_attempts": 3, "delay_ms": 1000, "backoff_multiplier": 2 },
  "error_next_node": "notify_failure",
  "input_schema": [],
  "output_schema": []
}
```

`max_attempts` 是总尝试次数，范围 1～20；首次失败后等待 `delay_ms`，后续按倍率退避，单次等待最多一小时且可取消。循环重访节点时重新计算本轮重试预算，累计尝试次数继续记录。错误分支可读取失败节点的 `output.<节点ID>.error.message`。

触发器集合的更新语义：省略/null 保留，`[]` 删除全部附加规则，非空数组替换集合。同一 Pipeline 可以配置多个触发器。规则 ID 必须属于当前 Pipeline，保留规则沿用历史和未改变的参数。

### 3.1 导入与 MCP

导入使用项目自己的 `FlowDefinition`：顶层 `start_node`、`context_schema`、`nodes` 对象。保存 API 使用 `start_node_id`、节点数组和 `next_ids`。这些格式不是 n8n JSON 或 Dify DSL，不能直接互导。

导入生成新流程 ID，**不自动导入触发规则**。创作者视频同步、直播录制和新增视频通知示例见 `examples/pipelines/README.md`。

`get_pipeline_editor` / `update_pipeline_editor` 读取或修改已连接页面的实时草稿，包含执行策略；修改必须携带 `revision`，旧版本会被拒绝。它只更新草稿，不自动保存或运行。更新请求体上限 1 MiB。待应用的节点配置或执行策略文本必须先处理。

## 4. 节点与变量

| 节点 | 关键配置 | 行为 |
| --- | --- | --- |
| `StartNode` | 流程 `context_schema` | 声明并校验运行输入 |
| `ServiceNode` | `tool_name`、`arguments`、`output_key`、`timeout_seconds` | 调用目录中的应用工具，默认输出 `service_result` |
| `APICallNode` | `url`、`method`、`headers`、`query`、`keys`、`body`、`body_type`、`authentication`、`timeout_seconds`、`fail_on_status`、`output_key` | HTTP 请求，默认输出 `api_response`、`api_response_status`、`api_response_headers` |
| `JSCodeNode` | `code`、`output_key` | Goja 执行 JS；响应流程取消和截止时间，不是 Node.js 环境 |
| `ExprNode` | `expression`、`output_key` | 表达式求值，默认输出 `calc_out` |
| `GatewayNode` | `gateway_type`、`rules`、`condition_language`、`join_node_id` | 条件路由或并行分支 |
| `SetVariableNode` | `variables` | 设置当前运行的显式全局变量 |
| `ManualNode` | `output_key`；节点 `input_schema` | 暂停等待人工输入，默认将回复写入 `response` |
| `WorkflowNode` | `flow_id` 或 `workflow`、`arguments`、`output_key` | 调用隔离作用域的子流程，默认输出 `workflow_result` |
| `LoopNode` | `items`、`flow_id` 或 `workflow`、`arguments`、`max_iterations`、`concurrency`、`continue_on_error`、`output_key` | 按数组执行子流程，默认输出 `loop_result` |
| `WaitNode` | `seconds` | 可取消的进程内延时，0～86400 秒 |
| `FailNode` | `message` | 业务条件不满足时主动失败，支持模板、重试和错误分支 |
| `EndNode` | — | 结束当前路径 |

`FuncNode` 仍只供内置 Go 流程使用，不向用户导入器开放。

### 4.1 显式作用域

| 作用域 | 来源 | 生命周期 |
| --- | --- | --- |
| `input.<键>` | `context_schema` 声明的运行输入 | 当前运行，从底层数据实时读取 |
| `output.<节点ID>.<键>` | 对应节点产生的输出 | 当前运行，按节点隔离 |
| `global.<键>` | `SetVariableNode` 写入 | 当前运行；不是跨运行存储 |

<span v-pre>模板例子：`{{input.creator_username}}`、`{{output.fetch.service_result}}`、`{{global.page_cursor}}`。完整模板保留原值类型，嵌入文本时转为字符串；参数支持对象和数组中的嵌套模板。不接受 `{{username}}` 或 `{{ctx.username}}`。</span>

HTTP、JS、服务、表达式等节点显式记录输出，同值写入也可供下游引用；旧 Go 节点仍有上下文差异索引兼容路径。运行检查点保存三个作用域，恢复后继续可用。运行摘要的 `node_outputs` 是扁平上下文，逐节点详情查看节点日志。

### 4.2 HTTP 配置

```json
{
  "url": "https://example.com/items",
  "method": "POST",
  "headers": { "X-Source": "wx-channels-download" },
  "query": {},
  "body_type": "json",
  "body": { "username": "{{input.creator_username}}" },
  "authentication": { "type": "bearer", "token": "替换为自己的令牌" },
  "timeout_seconds": 30,
  "fail_on_status": true,
  "output_key": "response"
}
```

支持 GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS。`keys` 保留旧参数映射方式，GET/HEAD 放 query，其他方法在未配置 `body` 时生成 JSON body。`body_type` 可选 json/form/raw，Basic 认证使用 `type: basic` 与 username/password。

默认节点超时 15 秒，受整体剩余时间约束。默认非 2xx 触发失败/重试，设置 `fail_on_status: false` 可自行按状态码分支。响应体上限 8 MiB，JSON 对象/数组可解码，否则返回文本。尚无 OAuth 凭据库、multipart 文件上传和自动分页。

### 4.3 数组迭代、子流程与并行

`WorkflowNode.arguments` 是子流程输入，子流程使用自己的 input/output/global。结果包含 `data`、`output`、`global` 三部分；子流程和主流程共享取消、整体时间及默认 10000 次节点尝试预算。引用的用户子流程在运行开始时一并冻结，修改子流程不会影响已开始运行的定义。

`LoopNode.items` 可为数组或指向数组的完整模板。每一项的子流程输入附加 `item` 和 `index`，需在子流程 schema 中声明。默认最多 1000 项，可设置 1～10000；超过上限直接失败，不把截断结果当作成功。默认串行，并发可设 1～32，输出按原数组顺序排列。`continue_on_error: true` 时失败项保留 error/index；取消或整体超时仍然终止运行。

Parallel 网关在独立上下文中同时执行分支，最多 32 条。设置共同的 `join_node_id` 后，所有分支成功到达该节点才执行一次汇聚。汇聚前不能共享执行节点；分支输出按节点 ID 合并，同名全局变量写入不同值时明确失败。条件网关 Exclusive 按顺序选择第一个命中规则，无命中时失败；expr 要求 bool，JS 使用真值判断且每条条件另有 1 秒限制。

**子流程、数组迭代和并行分支内暂不支持人工等待**，请把人工确认放在主流程、迭代外或汇聚后。`WaitNode` 是进程内延时，不是跨重启持久计时器。

## 5. 触发与调度

| 入口 | 行为 |
| --- | --- |
| `trigger_flow` | 手动调试，接受 `initial_data`，立即返回已落库的真实运行 ID |
| `trigger_schedule` | 使用规则参数立即执行；Event 类型还校验 key 与 active |
| Cron | 启动时扫描一次，之后每 30 秒查找到期规则，单轮最多 64 条 |
| 应用事件 | 事件总线按 Event 规则的 key 匹配，转为后台执行 |
| `dispatch_event` | 显式发出自定义事件，输入为 event_key/data |
| Webhook | POST 到规则地址，校验随机 Bearer 令牌后返回 202 与运行 ID |

Cron 支持五段表达式、`@daily`、`@every 1m` 等，默认进程时区，也可使用 `CRON_TZ=Asia/Shanghai 0 9 * * *`。有效扫描精度为 30 秒，积压会额外延迟；不支持秒级扫描保证。Cron 完成或进入人工等待后从当前时间计算下次时间，不补跑所有错过时刻。手动执行/重试不改变 Cron 计划。

同一规则处于排队、执行或人工等待时，拒绝重复启动；同一流程可以通过不同规则同时运行，变量互相隔离。当前使用单进程锁和数据库状态检查，没有分布式抢占机制。

### 5.1 事件载荷与 Webhook

使用事件载荷的流程需在 `context_schema` 中声明 `event`（object），然后读取 `input.event.type`、`input.event.data`。例如订阅 `downloadtask.finished` 后处理下载完成事件。`automation.*` 状态/日志事件和应用控制命令不自动触发用户流程，避免内部状态递归。

Webhook 在开始节点添加、保存后取得地址与令牌：

```text
POST /api/v1/automation/webhook/<规则ID>
Authorization: Bearer <规则元数据中的 webhook_token>
Content-Type: application/json
```

请求体必须为单个 JSON 对象且不超过 1 MiB。事件数据映射到 `input.event.data`。令牌无效返回 401，触发器未启用或正在执行时拒绝启动。不要把真实令牌写入可公开分享的流程示例。

事件总线目前不是持久消息队列：规则忙、进程退出或队列满时不保证事件补发，内部事件触发失败写日志；Webhook 调用方可根据错误重试。

## 6. 可靠运行、恢复与重试

### 6.1 生命周期

```text
QUEUED → RUNNING → COMPLETED
             ├──→ FAILED
             ├──→ CANCELLED
             └──→ WAITING → 人工确认 → RUNNING
```

规则整体超时默认 3600 秒，最大 86400 秒。截止时间在分配运行时确定，包含排队和人工等待；等待过期会被调度扫描标为失败，恢复接口也会立即检查截止时间。HTTP 与 JS 可主动中断，服务工具/内置 Go 回调是否及时退出还取决于其是否响应 context，无法强杀任意 Go 函数。

节点执行前后保存检查点，包含原定义、上下文、节点输出、全局变量、状态、尝试次数、待执行节点和步骤计数。写入运行记录失败时不启动流程，检查点写入失败时停止后续执行。每次节点尝试记录输入、行为、输出差异、耗时及错误，数据库保留每条运行最近 1000 条节点日志。

### 6.2 人工确认

1. 在主流程添加 `ManualNode`，可用节点 `input_schema` 声明人工回复字段。
2. 运行进入 WAITING 后，页面“最近运行”显示人工输入 JSON。
3. 提交后端校验 schema，原子地从 WAITING 切换到 RUNNING，重复确认被拒绝。
4. 回复保存在 `output.<人工节点ID>.response`（或其 output_key），继续原定义的后继节点，并保留其他待执行路径。

例如 schema 为 `[{"key":"approved","type":"boolean","required":true}]`，回复为 `{"approved":true}`。后续条件节点可根据人工回复选择发布或终止路径。

### 6.3 进程重启

重启后，WAITING 运行保留，确认时从数据库检查点恢复，不依赖旧引擎内存。排队或执行中的旧运行标记 FAILED，并提示最后一个节点的外部副作用可能已发生；核对后显式重试。这样避免把已经发出的通知、下载提交等操作自动再执行一次。

### 6.4 重试与取消

失败/取消的运行可以用“原流程”或“当前已保存流程”创建新运行，复用原输入，记录 `retry_of`。**这是从头重试，可能重复外部副作用**，不是恰好一次执行保证。原定义重试同时使用冻结的子流程；当前定义重试重新读取已保存定义。

取消把运行标记为终态并触发执行 context 取消；后续完成不能覆盖 CANCELLED。取消 Pipeline 不等于撤销此前已经提交给下载器的独立下载任务，文件最终结果仍在下载任务页面查看。

### 6.5 日志与凭据边界

`GET /ws/v1/automation` 推送运行和节点状态。页面定期查询数据库运行详情，刷新后也可查看最近节点日志；WebSocket 本身没有持久消息补发。

日志对 password/token/cookie 等敏感键脱敏，但 URL/脚本字符串中的密钥不保证可识别。定义、输入和恢复快照需要保存原始值才能重试，不是加密凭据库；应按本地业务数据管理数据库和 API 访问。

## 7. API

除 WebSocket 外，管理接口均为 POST，前缀 `/api/v1/automation/`：

| 分组 | 端点 |
| --- | --- |
| 流程 | list_flows、create_flow、get_flow、update_flow、delete_flow、import_flow、list_flow_nodes、get_flow_graph |
| 草稿 | get_editor、update_editor |
| 规则 | list_schedules、create_schedule、get_schedule、update_schedule、delete_schedule、toggle_schedule、trigger_schedule |
| 运行 | trigger_flow、list_runs、get_run、cancel_run、resume_run、retry_run |
| 事件 | dispatch_event、webhook/:id |

`resume_run` 请求为 `{"id":"run-...","node_id":"confirm","input":{"approved":true}}`；`retry_run` 为 `{"id":"run-...","use_current_definition":false}`；`dispatch_event` 为 `{"event_key":"custom.created","data":{"value":"example"}}`。运行列表只返回摘要，详情和持久日志通过 get_run 取得。

## 8. 与 n8n、Dify、豆包生态对比

豆包客户端、豆包模型与扣子编排平台是不同层次。下面的“扣子”指字节系可视化工作流，不把其全部能力归到豆包客户端。官方明确部分工作流能力受豆包/飞书等渠道限制。[渠道说明][c-agent]

| 维度 | 本项目 automation | n8n | Dify | 扣子（豆包生态） |
| --- | --- | --- | --- | --- |
| 定位 | 下载器业务工具与本地流程 | 通用服务集成 | AI 应用编排 | AI 工作流与对话流。[介绍][c-overview] |
| 数据与节点 | 显式作用域；服务、HTTP、JS、表达式等 13 类用户节点 | item 数据模型及丰富集成。[数据][n-data] | 原生 LLM、RAG、Agent、工具等节点。[节点导航][d-iteration] | 模型、插件、数据库等节点。[介绍][c-overview] |
| HTTP | 模板、headers/query/body、Basic/Bearer、超时和错误状态；缺 OAuth 凭据库、multipart、自动分页 | 完整认证、请求体和更多选项。[HTTP][n-http] | 专用 HTTP 节点与认证、重试。[HTTP][d-http] | 专用 HTTP 节点与异常配置。[HTTP][c-http] |
| 迭代/并行 | 数组子流程、并发上限、顺序汇总、隔离分支和一次汇聚；嵌套人工等待暂不支持 | 循环、合并、子流程与明确执行顺序。[逻辑][n-logic] | Iteration/Loop、串并行及失败项策略。[迭代][d-iteration] | 循环串行，批处理可并行。[循环][c-loop]、[批处理][c-batch] |
| 人工参与 | JSON 回复、schema 校验、原子确认、重启后恢复原运行 | Wait 暂停恢复。[逻辑][n-logic] | 表单、动作、投递和超时策略。[人工输入][d-human] | 问答等交互节点，受渠道限制。[介绍][c-overview] |
| 触发 | 手动、Cron、事件、令牌 Webhook；单实例且事件不保证补发 | Schedule、Webhook 等。[定时][n-schedule]、[Webhook][n-webhook] | Workflow 支持定时/集成事件/Webhook，多触发器受套餐限制。[触发器][d-trigger] | 应用定时触发；智能体能力另受渠道与升级限制。[应用触发器][c-schedule]、[智能体触发器][c-agent-trigger] |
| 重试与历史 | 节点退避/错误分支；按原/当前定义从头重试；持久日志最近 1000 条 | 错误工作流、执行历史及重试。[错误][n-errors]、[历史][n-runs] | HTTP 重试、迭代失败策略和版本管理。[HTTP][d-http]、[版本][d-version] | 插件重试、降级与试运行详情。[插件][c-plugin]、[调试][c-debug] |
| 发布与扩容 | 每次运行冻结定义；无独立发布版本、流程级 RBAC、分布式 worker | 发布流程与 Redis/worker 队列部署。[定时][n-schedule]、[队列][n-queue] | 区分草稿/发布版本，部分版本操作受套餐限制。[版本][d-version] | 发布 API 工作流与空间资源权限。[API][c-run]、[权限][c-overview] |
| 模板迁移 | 自有 JSON 导入，触发规则另配 | 原生 JSON 导入导出。[导入导出][n-import] | 应用版本与 DSL，不与本项目直接兼容。[版本][d-version] | 原生资源导入/导出。[介绍][c-overview] |

本项目优势是复用本机下载器服务和数据。仍未覆盖的方向包括：多实例持久队列/分布式调度、事件可靠投递、独立发布版本与权限治理、加密凭据/OAuth、持久延时、嵌套人工等待及 AI 专用节点。这些不在当前实现中宣称已完成。

## 9. 非浏览器验证

```sh
go test -race ./pkg/flowengine/... ./internal/services ./internal/api ./internal/mcpserver ./examples/pipelines
npm --prefix frontend test
npm --prefix docs run build
```

新增测试覆盖 JS/HTTP 取消、整体截止、并行重叠与一次汇聚、步骤上限、子流程迭代、重试/错误分支、策略往返、人工等待跨重启恢复、原定义重试、事件匹配和 Webhook 令牌。浏览器行为不自动验证，请手动检查画布与运行面板。

## 10. 官方参考资料

- n8n：[流程逻辑][n-logic]、[HTTP][n-http]、[错误处理][n-errors]、[执行历史][n-runs]、[队列部署][n-queue]。
- Dify：[Workflow/Chatflow][d-overview]、[触发器][d-trigger]、[编排][d-orchestration]、[迭代][d-iteration]、[人工输入][d-human]、[HTTP][d-http]、[版本][d-version]。
- 扣子：[工作流介绍][c-overview]、[渠道限制][c-agent]、[执行 API][c-run]、[触发器][c-schedule]、[批处理][c-batch]。

[n-logic]: https://docs.n8n.io/build/flow-logic.md
[n-data]: https://docs.n8n.io/build/work-with-data/understand-n8ns-data-structure.md
[n-http]: https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.httprequest
[n-schedule]: https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.scheduletrigger
[n-webhook]: https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook
[n-errors]: https://docs.n8n.io/build/flow-logic/handle-errors-gracefully.md
[n-runs]: https://docs.n8n.io/workflows/executions/all-executions/
[n-queue]: https://docs.n8n.io/deploy/host-n8n/configure-n8n/scaling/enable-queue-mode.md
[n-import]: https://docs.n8n.io/build/manage-workflows/export-and-import.md
[c-overview]: https://docs.coze.cn/guides_workflow
[c-agent]: https://docs.coze.cn/guides_agent_workflow
[c-run]: https://docs.coze.cn/developer_guides_workflow_run
[c-loop]: https://docs.coze.cn/guides_loop_node
[c-batch]: https://docs.coze.cn/guides_batch_node
[c-schedule]: https://docs.coze.cn/guides_set_trigger_for_app
[c-agent-trigger]: https://docs.coze.cn/guides_task
[c-plugin]: https://docs.coze.cn/guides_plugin_node
[c-http]: https://docs.coze.cn/guides_http_node
[c-debug]: https://docs.coze.cn/guides_ai_powered_workflow_development
[d-overview]: https://docs.dify.ai/en/cloud/use-dify/build/workflow-chatflow
[d-trigger]: https://docs.dify.ai/en/cloud/use-dify/nodes/trigger/overview
[d-orchestration]: https://docs.dify.ai/en/cloud/use-dify/build/orchestrate-node
[d-iteration]: https://docs.dify.ai/en/cloud/use-dify/nodes/iteration
[d-human]: https://docs.dify.ai/en/cloud/use-dify/nodes/human-input
[d-http]: https://docs.dify.ai/en/cloud/use-dify/nodes/http-request
[d-version]: https://docs.dify.ai/en/cloud/use-dify/build/version-control

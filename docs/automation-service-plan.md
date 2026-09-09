# Plan: Add `services/automation` for Reusable Scheduled Workflow Execution

## Context

The project has a mature `pkg/flowengine` that can define and execute JSON workflows with pluggable nodes, run records, and basic cron-like scheduling. However the current `ScheduleCron()` only supports simple duration intervals (`@every 5m`), everything is in-memory (lost on restart), and there's no service layer — so API, MCP, and CLI can't share automation capabilities. This plan adds `AutomationService` as a proper service following existing patterns, with SQLite persistence, real crontab expressions, and a reusable interface for all consumers.

## Files to Create

### 1. `internal/database/model/automation.go` — GORM Models

```go
type FlowSchedule struct {
    ID            string  `gorm:"primaryKey;size:64" json:"id"`
    Name          string  `gorm:"not null" json:"name"`
    Description   string  `gorm:"type:text" json:"description"`
    CronExpr      string  `gorm:"not null" json:"cron_expr"`
    FlowID        string  `gorm:"not null;index" json:"flow_id"`
    InitialData   string  `gorm:"type:text" json:"initial_data"`    // JSON
    Enabled       bool    `gorm:"not null;default:true" json:"enabled"`
    TimeoutSec    int     `gorm:"not null;default:3600" json:"timeout_sec"`
    NextRunAt     *int64  `gorm:"index" json:"next_run_at"`
    LastRunID     string  `json:"last_run_id"`
    LastRunStatus string  `json:"last_run_status"`
    Timestamps
}

type FlowRunRecord struct {
    ID           string  `gorm:"primaryKey;size:96" json:"id"`
    ScheduleID   string  `gorm:"index" json:"schedule_id"`         // nullable for manual/API triggers
    FlowID       string  `gorm:"not null;index" json:"flow_id"`
    TriggerType  string  `json:"trigger_type"`                     // "Cron" | "API" | "Manual"
    TriggerKey   string  `json:"trigger_key"`
    Status       string  `gorm:"not null;index" json:"status"`     // QUEUED/RUNNING/COMPLETED/FAILED/CANCELLED
    CurrentNode  string  `json:"current_node"`
    NodeAttempts string  `gorm:"type:text" json:"node_attempts"`   // JSON
    NodeOutputs  string  `gorm:"type:text" json:"node_outputs"`    // JSON
    Error        string  `gorm:"type:text" json:"error"`
    StartedAt    *int64  `json:"started_at"`
    CompletedAt  *int64  `json:"completed_at"`
    Timestamps
}
```

Follow existing patterns: embed `model.Timestamps`, explicit `TableName()`, int64 epoch millis.

### 2. `internal/database/migrations/000003_flow_schedule.up.sql` + `.down.sql`

Create `flow_schedule` and `flow_run_record` tables with indexes. The `//go:embed all:migrations` in `embed.go` picks these up automatically.

### 3. `internal/services/automation.go` — Core AutomationService

```go
type AutomationService struct {
    db          *gorm.DB
    logger      *zerolog.Logger
    flow_engine *flowengine.FlowEngine
    event_bus   events.Publisher
    cron_parser cron.Parser           // robfig/cron/v3, parse-only
    stop        chan struct{}
    running     sync.Map              // schedule_id → struct{}, prevent concurrent exec
}

func NewAutomationService(
    db *gorm.DB,
    logger *zerolog.Logger,
    flow_engine *flowengine.FlowEngine,
    event_bus events.Publisher,
) *AutomationService
```

**Methods (all transport-agnostic, reusable by API/MCP/CLI):**

| Method | Description |
|---|---|
| `Start()` | Start the 30s ticker scheduler loop |
| `Stop()` | Stop scheduler, wait for in-flight runs |
| `CreateSchedule(input CreateScheduleInput) (*FlowSchedule, error)` | Validate cron expr, compute next_run_at, persist |
| `UpdateSchedule(id string, input UpdateScheduleInput) (*FlowSchedule, error)` | Update fields, recompute next_run_at |
| `DeleteSchedule(id string) error` | Soft-delete |
| `GetSchedule(id string) (*FlowSchedule, error)` | Single schedule |
| `ListSchedules(input ListSchedulesInput) ([]FlowSchedule, int64, error)` | Paginated list |
| `ToggleSchedule(id string) (*FlowSchedule, error)` | Flip enabled, recompute next_run_at |
| `TriggerSchedule(id string) (*FlowRunRecord, error)` | Manual trigger (trigger_type="Manual") |
| `ListRuns(input ListRunsInput) ([]FlowRunRecord, int64, error)` | Paginated, filterable by schedule_id |
| `GetRun(id string) (*FlowRunRecord, error)` | Single run with details |
| `CancelRun(id string) error` | Cancel a running execution |

**Scheduler loop (internal):**
```
tick() every 30s:
  → query: SELECT * FROM flow_schedule WHERE enabled=1 AND next_run_at <= NOW() AND deleted_at IS NULL
  → for each: skip if running (sync.Map check), else go executeSchedule(sched)

executeSchedule(sched):
  → mark running in sync.Map
  → create FlowRunRecord (status=RUNNING) in DB
  → call flow_engine.StartFlowWithOptions(flow_id, initial_data, {Trigger: {Type: Cron, Key: cron_expr, Source: schedule_id}})
  → get RunRecord from engine via GetRunSnapshot(instance_id)
  → update FlowRunRecord in DB with final status
  → compute next_run_at via cron_parser, update schedule
  → clear sync.Map entry
  → publish event
```

**Cron parsing:** Use `cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)` for standard 5-field crontab. Call `schedule.Next(time.Now())` to compute `next_run_at`. We do NOT use `cron.New()` — the DB-driven tick is the scheduler.

### 4. `internal/events/types.go` — Add Automation Events

```go
const (
    TypeAutomationRunStarted   = "automation.run_started"
    TypeAutomationRunCompleted = "automation.run_completed"
    TypeAutomationRunFailed    = "automation.run_failed"
)

type AutomationRunStarted struct {
    ScheduleID string
    RunID      string
    FlowID     string
}
type AutomationRunCompleted struct { ... }
type AutomationRunFailed struct { ... }
```

### 5. `internal/api/automation.go` — Gin HTTP Handlers

Thin layer calling `AutomationService` methods. Pattern matches existing handlers.

```go
// Routes (added to routes.go):
c.engine.GET("/api/v1/automation/schedules", c.handle_list_schedules)
c.engine.POST("/api/v1/automation/schedules", c.handle_create_schedule)
c.engine.GET("/api/v1/automation/schedules/:id", c.handle_get_schedule)
c.engine.PUT("/api/v1/automation/schedules/:id", c.handle_update_schedule)
c.engine.DELETE("/api/v1/automation/schedules/:id", c.handle_delete_schedule)
c.engine.POST("/api/v1/automation/schedules/:id/toggle", c.handle_toggle_schedule)
c.engine.POST("/api/v1/automation/schedules/:id/trigger", c.handle_trigger_schedule)
c.engine.GET("/api/v1/automation/runs", c.handle_list_runs)
c.engine.GET("/api/v1/automation/runs/:id", c.handle_get_run)
```

### 6. `internal/api/client.go` + `server.go` — Wire AutomationService

Add `automation_service *services.AutomationService` field to `APIClient`, add parameter to `NewAPIClient` and `NewAPIServer`.

### 7. `internal/mcpserver/automation_tools.go` — MCP Tools

```go
// Backend interface
type AutomationBackend interface {
    ListSchedules(ctx context.Context) ([]AutomationScheduleSummary, error)
    GetSchedule(ctx context.Context, id string) (*AutomationScheduleDetail, error)
    CreateSchedule(ctx context.Context, input AutomationCreateScheduleInput) (*AutomationScheduleDetail, error)
    ToggleSchedule(ctx context.Context, id string) (*AutomationScheduleDetail, error)
    TriggerSchedule(ctx context.Context, id string) (*AutomationRunSummary, error)
    ListRuns(ctx context.Context, scheduleID string, limit int) ([]AutomationRunSummary, error)
}

// Tools: list_automation_schedules, create_automation_schedule,
//        toggle_automation_schedule, trigger_automation_schedule,
//        list_automation_runs
```

### 8. `internal/mcpserver/protocol.go` + `tools.go` — Wire Automation Backend

Add `automation AutomationBackend` field to `Server` and `Config`. Add entries to `tool_definitions()`, `supports_tool()`, `call_tool()`.

### 9. `internal/application/mcp_backends.go` — Adapter

```go
type mcp_automation_backend struct {
    automation_service *services.AutomationService
}
// implements AutomationBackend by calling automation_service methods
```

### 10. `internal/application/start.go` — Wiring

After creating `download_task_service`, before creating API server:

```go
flow_engine := flowengine.NewWorkflowEngine()
automation_service := services.NewAutomationService(b.DB, logger, flow_engine, bus)
automation_service.Start()
// add to cleanup: defer automation_service.Stop()
```

Pass `automation_service` to `NewAPIServer(...)` and to `new_mcp_service(...)`.

### 11. `internal/application/mcp_stdio.go` — Standalone MCP Wiring

Same pattern: create `flow_engine`, `automation_service`, pass to MCP server config.

### 12. `go.mod` — Add Dependency

```
github.com/robfig/cron/v3
```

## Files to Modify

| File | Change |
|---|---|
| `internal/api/client.go` | Add `automation_service` field + constructor param |
| `internal/api/server.go` | Add `automation_service` param, pass to `NewAPIClient` |
| `internal/api/routes.go` | Add 9 automation routes |
| `internal/events/types.go` | Add 3 automation event types + structs |
| `internal/mcpserver/protocol.go` | Add `AutomationBackend` to Config/Server |
| `internal/mcpserver/tools.go` | Wire automation tools into dispatch |
| `internal/application/start.go` | Create flow_engine + automation_service, pass around |
| `internal/application/mcp_stdio.go` | Same wiring for standalone mode |
| `internal/application/mcp_backends.go` | Add automation adapter |
| `go.mod` / `go.sum` | Add robfig/cron/v3 |

## Key Design Decisions

1. **AutomationService owns persistence + scheduling; FlowEngine unchanged.** AutomationService calls `FlowEngine.StartFlowWithOptions()` as an external caller. No changes to `pkg/flowengine/engine/`.

2. **No separate store interface.** Follow the existing service pattern — services use `*gorm.DB` directly (like `AccountService`, `ContentService`). No ScheduleStore abstraction.

3. **robfig/cron/v3 for parsing only.** We use `cron.Parser.Parse()` + `schedule.Next()` to compute `next_run_at`. The actual scheduler is our own 30s ticker reading from SQLite. This is simpler and naturally persistent.

4. **Shared FlowEngine instance.** `start.go` creates one `flowengine.NewWorkflowEngine()` that's injected into AutomationService. Flow definitions can be registered on it by adapters or via future CRUD. Initially starts empty — callers register flows as needed.

5. **Same-schedule concurrency guard via `sync.Map`.** If a schedule's previous run is still in-flight, the tick skips it. Simple and correct.

## Implementation Order

1. `go get github.com/robfig/cron/v3`
2. `internal/database/model/automation.go`
3. `internal/database/migrations/000003_flow_schedule.up.sql` + `.down.sql`
4. `internal/events/types.go` (add automation events)
5. `internal/services/automation.go` (core service)
6. `internal/api/automation.go` (handlers)
7. `internal/api/client.go` + `server.go` + `routes.go` (wire service)
8. `internal/mcpserver/automation_tools.go` (backend interface + tools)
9. `internal/mcpserver/protocol.go` + `tools.go` (wire)
10. `internal/application/mcp_backends.go` (adapter)
11. `internal/application/start.go` + `mcp_stdio.go` (composition root)

## Verification

1. `go build ./...` — compile check
2. Create a schedule via API: `POST /api/v1/automation/schedules` with a `@every 1m` cron expr and a registered flow_id
3. Check `GET /api/v1/automation/schedules` shows the schedule with computed `next_run_at`
4. Wait for tick or manually trigger: `POST /api/v1/automation/schedules/:id/trigger`
5. Check run records: `GET /api/v1/automation/runs`
6. Toggle disable: `POST /api/v1/automation/schedules/:id/toggle` — verify skipped on next tick
7. Restart the app — verify schedules and run history survive

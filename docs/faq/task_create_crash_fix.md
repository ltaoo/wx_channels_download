# 修复：调用 /api/task/create 接口导致程序崩溃

## 问题描述

用户反馈在多次调用 `/api/task/create` 接口后，程序会无提示崩溃。

## 根本原因

经过代码分析，发现以下几个可能导致崩溃的问题：

### 1. 空指针访问 (主要原因)

在 `pkg/gopeed/pkg/download/downloader.go` 的 `watch` 函数中：

```go
// Line 793
err := task.fetcher.Wait()
```

在并发场景下，`task.fetcher` 可能为 `nil`，直接调用 `Wait()` 会导致 panic，程序崩溃。

类似的问题还出现在：
- Line 806: `task.fetcher.Progress().TotalDownloaded()`

### 2. Goroutine Panic 未捕获

多个 goroutine 中没有 `recover` 机制，任何 panic 都会导致整个程序崩溃：
- `watch` 函数的主 goroutine
- `watch` 函数中的 upload goroutine
- `doStart` 函数中的 handler goroutine
- `doPause` 函数中的 handler goroutine

### 3. 并发竞态条件

在 `doCreate` 函数中，可能会对同一个 task 调用两次 `watch`，虽然有 `LoadOrStore` 保护，但仍存在潜在风险。

## 修复方案

### 1. 添加空指针检查

在 `watch` 函数中，调用 `task.fetcher.Wait()` 之前检查是否为 `nil`：

```go
// Check if fetcher is nil before calling Wait
if task.fetcher == nil {
    d.Logger.Error().Msgf("task fetcher is nil, task id: %s", task.ID)
    return
}

err := task.fetcher.Wait()
```

在访问 `task.fetcher.Progress()` 之前也添加检查：

```go
if task.Meta.Res.Size == 0 {
    if task.fetcher != nil {
        task.Meta.Res.Size = task.fetcher.Progress().TotalDownloaded()
    }
}
```

### 2. 添加 Panic 恢复机制

在所有 goroutine 中添加 `defer recover()`，捕获 panic 并记录日志。

## 修复效果

1. **防止空指针崩溃**：在访问 `task.fetcher` 之前进行检查，避免 nil pointer dereference
2. **捕获所有 panic**：所有 goroutine 都有 recover 机制，即使发生 panic 也不会导致程序崩溃
3. **详细的错误日志**：panic 发生时会记录完整的堆栈信息，便于排查问题

## 测试建议

1. 快速连续调用多次 `/api/task/create` 接口
2. 在下载过程中删除任务
3. 在任务启动过程中暂停任务
4. 检查日志文件中是否有 panic 记录

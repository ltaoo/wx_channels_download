1. **改用 cookie 认证跑 live 测试**：修改 `bigmodel_overview_test.go` 和 `save_overview_test.go`，去掉 BIGMODEL_TOKEN 强制依赖：用 `cookies.NewReader(workdir/cookies.json)` + `browser.SetCookieProvider(...)`（机制已存在于 minib.go:179）；Authorization 头仅在 env 提供 token 时附加（org/project 头保留）。

2. **现场复现**：运行 `TestSaveBigmodelOverview`，保存渲染 HTML 到 workdir/overview_rendered.html，收集 ScriptFailures / Resources 错误日志，确认 progress bar 仍卡 0% 以及是哪条 JS 失败（若有）。

3. **定位根因并修复 pkg/minib/page.go**，按现场证据排查以下候选（按可能性排序）：
   - 补间回调里 `displayPercents` 更新走 Vue 调度器（Promise 微任务），但 timer 回调后没有 flush 微任务队列 → 更新丢失；
   - 实际线上 chunk 的 tween 用 `Date.now()`（而非 fixture 里的 `performance.now()`），而页面运行时的 `Date.now` 未挂在虚拟时钟上；
   - 最后一次 `pump_event_loop` 在 quota 响应触发 tween 前退出（网络任务等待逻辑覆盖不到），需要响应落地后再补一次 pump。

4. **加回归测试**：在 `animation_frame_test.go`（或新文件）加入复现真实根因的最小 fixture（例如 tween 回调内经 Promise.then 更新 DOM），保证 `progress-bar-value` width 与 `percentage-value` 一致且非 0。

5. **验证**：
   - `go test ./pkg/minib/ -run 'AnimationFrame|Bigmodel'`（含 live overview 测试，允许数值与真实页不同，只断言 width==percentage 且非 0）；
   - 跑完整个 `pkg/minib` 测试套件确认无回归。
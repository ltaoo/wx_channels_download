# wxchannels 动态插件架构评估与实施建议

> 状态：设计草案  
> 评估日期：2026-08-03  
> 目标模块：`internal/adapter/wxchannels`

## 1. 背景与目标

计划将 `wxchannels` adapter 从主程序中拆出，作为可独立发布和更新的插件。期望能力包括：

- 插件在主程序启动时动态加载；
- 插件文件可以独立更新、替换和回滚；
- 支持 macOS、Linux 和 Windows；
- 最终视需求支持不重启主程序的运行时更新。

本文记录当前代码对该目标的支持程度、主要阻塞点以及推荐的实施路线。

## 2. 结论

当前实现已经具备一部分“接口插件化”基础，但尚不支持把 `wxchannels` 直接编译为 `.so` 或 `.dll` 后动态加载，也不支持运行中安全替换插件。

当前大致成熟度：

| 能力 | 当前状态 |
| --- | --- |
| Adapter 接口解耦 | 已有基础，约 60% |
| 启动时从外部文件动态加载 | 尚未实现，约 20% |
| 替换插件后重启生效 | 尚未实现，约 20% |
| 不重启主程序热替换 | 尚未实现，接近 0% |
| Windows `.dll` 形式的 Go 插件 | 不支持 |

若只要求“更新插件文件后重启生效”，可以在现有接口基础上继续改造。若要求跨平台且支持可靠热更新，推荐使用独立插件进程，而不是 Go 原生 `.so/.dll` 插件。

## 3. 当前已有基础

### 3.1 统一的 Adapter 协议

`internal/download/registry/registry.go` 已定义：

- `PlatformHandler`：平台内容解析和下载任务构建；
- `RuntimeAdapter`：安装路由、资源和拦截器等运行时能力；
- `RuntimeHandle`：提供 `Stop()` 生命周期；
- `RuntimeDeps`：由宿主向 adapter 注入数据库、配置、事件总线、路由和拦截器等能力。

应用启动时遍历 registry，通过这些接口初始化 adapter。因此，业务调用层已经不需要直接依赖 `wxchannels` 的具体实现。

### 3.2 宿主能力注入

`internal/adapter/wxchannels/register.go` 中的 `RegisterRuntime` 接收 `registry.RuntimeDeps`，说明 adapter 已开始通过能力接口访问宿主，而不是直接控制整个应用。

### 3.3 基础生命周期

`wxchannels.Handle` 实现了 `Stop()`，应用退出时会倒序停止 adapter。这个生命周期可以作为未来卸载、停止接收新请求和释放资源的起点。

## 4. 当前主要阻塞点

### 4.1 wxchannels 仍被静态编译进主程序

`internal/application/start.go` 空导入 `internal/adapter/builtin`，而 `internal/adapter/builtin/builtin.go` 又空导入 `internal/adapter/wxchannels`。`wxchannels` 通过 `init()` 自动注册到全局 registry。

因此当前行为是编译期链接，不存在以下能力：

- 插件目录扫描；
- 外部文件加载；
- 插件元数据和协议版本校验；
- 签名或校验和验证；
- 加载失败隔离和版本回滚。

### 4.2 当前包不能直接构建成 Go plugin

Go 的 `-buildmode=plugin` 需要入口为 `package main`，当前目录是普通的 `package wxchannels`。即使选择 Go 原生 plugin，也需要增加单独入口，例如：

```text
cmd/plugins/wxchannels/
└── main.go        # package main，导出 NewPlugin 或 Plugin
```

同时需要从主程序的 builtin 列表移除 `wxchannels`，否则内置版本和动态版本会重复注册。

### 4.3 Registry 只能注册，不能替换或注销

当前 `registry.Register` 遇到相同 `PlatformID` 会 panic，并且没有：

```go
Replace(...)
Unregister(...)
Acquire(...)
Release(...)
```

这意味着加载新版本时无法原子切换 handler，也无法等待使用旧 handler 的请求完成后再释放旧版本。

### 4.4 路由、拦截器、静态资源和配置均为只增不减

目前各类注册能力存在相同问题：

- Gin 路由只有 `RegisterGET/RegisterPOST`，没有撤销和替换；
- interceptor 只有 `AddPostPlugin`，没有按插件 ID 删除或替换；
- static assets 遇到相同路径会返回重复挂载错误，没有 `Unmount/Replace`；
- config plugin 在 `init()` 中全局注册，没有对应的注销生命周期；
- adapter 注册的事件回调没有统一收集和取消机制。

当前 `Handle.Stop()` 只停止 `WebsocketRoutes` 内部 client，无法撤销上述注册。旧插件函数仍可能被路由或回调引用，因此不能视为真正卸载。

### 4.5 Go 原生 plugin 的平台和 ABI 限制

Go 原生 plugin 更适合“在受控 Unix 环境中启动时加载”，不适合作为本项目的跨平台热更新方案：

- Windows 没有与 Go `plugin.Open` 对等的 `.dll` 插件机制；
- 宿主与插件需要高度一致的 Go 版本、依赖版本和构建参数；
- 已加载 Go plugin 不能可靠卸载；
- 对相同路径再次加载不能保证获得更新后的实例；
- 插件中的 Go 类型、interface 和宿主内部类型形成较强 ABI 耦合。

此外，`wxchannels` 当前大量依赖 `wx_channel/internal/...`。若以后作为独立 module 构建，会受到 Go `internal` 包导入规则限制，也难以形成稳定、独立发布的插件 SDK。

### 4.6 配置 HotReload 不等于二进制热更新

配置项中的 `HotReload: true` 只表示配置值可热加载，不代表 adapter 二进制可以卸载或替换。

## 5. 推荐技术路线

### 5.1 推荐：独立插件进程

跨平台、可更新场景建议采用进程隔离：

```text
主程序
├── Plugin Manager
├── 稳定的 HTTP 路由和资源入口
├── 下载任务调度与数据库访问
└── wxchannels 插件进程
    └── JSON-RPC / gRPC / 本地 socket
```

发布形态：

- macOS/Linux：独立 executable；
- Windows：独立 `.exe`；
- 插件包可包含 manifest、可执行文件和前端静态资源。

优点：

- 不受 Go plugin ABI 严格一致性的限制；
- 插件崩溃不会直接带崩宿主；
- 更新时可以停止旧进程、原子替换文件、启动新进程；
- 容易实现健康检查、调用超时、权限限制和回滚；
- 宿主与插件可以独立升级，协议通过版本协商保持兼容。

建议不要让插件直接接收 `*gorm.DB`、Gin handler 或 Go interface。跨进程协议只传稳定的数据结构，优先使用 JSON、protobuf 或字节数据。

### 5.2 次选：Go 原生 `.so`，仅支持重启生效

如果部署环境可以限定为 Go plugin 支持的平台，且接受更新后重启，可以使用 `-buildmode=plugin`：

1. 新建 `package main` 插件入口；
2. 导出固定符号，例如 `Manifest` 和 `NewPlugin`；
3. 将公共协议移动至稳定的公共包；
4. 主程序启动时扫描并调用 `plugin.Open`；
5. 校验宿主 API、Go 工具链和依赖构建标识；
6. 下载新版到临时文件，校验后原子替换；
7. 重启主程序加载新版。

该方案不应承诺 Windows 支持或运行中卸载。

### 5.3 不推荐：Go `c-shared` `.so/.dll`

若产品强制要求 `.so/.dll` 文件，可以用 `-buildmode=c-shared` 暴露 C ABI，但需要重新设计边界：

- 只导出 C 兼容函数；
- 使用 JSON/字节数组传递数据；
- 使用显式句柄管理实例；
- 不跨边界传递 Go 指针、interface、Gin handler 或 GORM 对象；
- 明确定义内存分配和释放责任；
- 仍不建议尝试卸载包含 Go runtime 的动态库。

实现和维护成本通常高于独立进程方案。

## 6. 建议的目标协议

无论最终采用进程还是动态库，建议先抽出稳定 SDK，例如：

```text
pkg/pluginapi/
├── manifest.go
├── adapter.go
├── content.go
├── download.go
└── protocol_version.go
```

插件 manifest 至少包含：

```go
type Manifest struct {
    ID                 string
    Name               string
    Version            string
    ProtocolVersion    int
    MinHostVersion     string
    SupportedPlatforms []string
    Capabilities       []string
}
```

协议设计原则：

- 公共协议包不能放在 `internal` 下；
- 避免向插件暴露宿主的数据库模型；
- 下载内容、配置和结果采用版本化 DTO；
- 未知字段应向前兼容；
- 每次调用携带 context、deadline 和 cancellation；
- 插件能力通过 manifest 声明，宿主不能依赖具体插件类型。

## 7. 支持热替换所需的宿主改造

如果以后确实需要不重启更新，需要引入 Plugin Manager，并完成以下能力。

### 7.1 Handler 原子切换

- registry 按插件 ID 保存可替换的 handler slot；
- 新请求通过 slot 获取当前版本；
- 更新时先加载并健康检查新版本，再原子切换；
- 旧版本停止接收请求，等待 in-flight 调用结束；
- 超时后取消剩余调用并停止旧实例。

### 7.2 稳定路由入口

不要让每个插件版本反复向 Gin 注册相同路径。应由宿主只注册一次稳定入口，再由 dispatcher 转发给当前插件：

```text
/api/plugins/:pluginID/*path
          ↓
Plugin Dispatcher
          ↓
当前 wxchannels 实例
```

已有兼容路由如 `/api/channels/...` 也应注册为宿主代理，而不是直接持有某一版本插件的 handler。

### 7.3 可撤销的资源注册

所有注册动作都应返回 disposable handle：

```go
type Registration interface {
    Close() error
}
```

需要覆盖：

- interceptor plugins；
- static asset mounts；
- event subscriptions；
- config schema；
- 定时任务和后台 goroutine。

`RuntimeHandle.Stop()` 应停止接收新任务、释放全部 registration，并等待内部 goroutine 退出。

### 7.4 安全更新与回滚

建议更新流程：

1. 下载到版本化临时目录；
2. 校验 SHA-256 和发布签名；
3. 读取 manifest 并校验协议兼容性；
4. 启动候选版本并执行健康检查；
5. 原子切换流量；
6. drain 旧版本；
7. 记录新版本为 active；
8. 健康检查失败时回滚到上一版本。

不要直接覆盖正在运行或已加载的文件。插件文件应使用版本化路径，例如：

```text
plugins/wxchannels/1.4.0/plugin
plugins/wxchannels/1.5.0/plugin
plugins/wxchannels/current.json
```

## 8. 分阶段实施建议

### 阶段一：整理边界

- 创建 `pkg/pluginapi` 和版本化 DTO；
- 消除插件协议对 `internal/database/model`、Gin、GORM 等具体类型的依赖；
- 将 `init()` 注册改为显式构造；
- 为所有资源注册增加可撤销 handle；
- 补充 adapter 生命周期和泄漏测试。

### 阶段二：支持外部插件和重启更新

- 实现 Plugin Manager、manifest 和插件目录；
- 首先采用独立进程协议；
- 移除 builtin 中的 `wxchannels` 静态导入；
- 实现安装、校验、启用、禁用、版本选择和回滚；
- 更新完成后安全重启插件进程，必要时重启主程序。

### 阶段三：运行时无感更新

- 实现稳定路由 dispatcher；
- 实现 handler 原子切换和请求 drain；
- 加入健康检查、故障熔断和自动回滚；
- 验证 websocket、下载任务和 interceptor 请求在更新期间的行为；
- 明确长连接更新策略，例如等待结束、主动断开或连接继续绑定旧版本。

## 9. 验收标准

第一阶段可用版本建议满足：

- 主程序二进制不再包含 `wxchannels` 实现；
- 缺少插件时主程序仍可启动，并明确报告能力不可用；
- 插件协议不引用 `wx_channel/internal/...`；
- 插件版本与协议不兼容时拒绝加载并给出可诊断错误；
- 替换插件版本后不需要重新编译主程序；
- 更新失败可以回滚到上一版本；
- Windows、macOS、Linux 使用相同的协议和更新流程；
- 插件停止后不存在遗留 goroutine、路由回调、事件订阅或 interceptor；
- 已开始的下载任务在插件更新时有明确且经过测试的处理策略。

## 10. 当前推荐决策

短期目标定为“外部插件独立发布，更新后重启插件或主程序生效”；实现时优先选择独立进程插件。等协议边界、生命周期和回滚机制稳定后，再评估是否确实需要主程序无感热替换。

不建议把 Go 原生 `.so/.dll` 作为跨平台插件系统的核心方案。

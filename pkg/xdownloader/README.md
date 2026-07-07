# xdownloader 设计方案

`pkg/xdownloader` 是新的底层下载核心，目标是把“下载协议实现”从 `pkg/contentplatform/download` 的业务编排里拆出来。现有 `contentplatform/download` 继续负责平台解析、任务状态和文件命名；`xdownloader` 只负责把一个已经解析好的 `Request` 下载到本地。

## 参考 yt-dlp 的拆分

`yt_dlp/downloader` 的可复用点主要有四层：

1. `common.FileDownloader`：统一参数、临时文件、断点续传、进度、重试、速率限制。
2. `__init__.py`：根据 `protocol` 和参数选择下载器，例如 http、m3u8 native、dash、ffmpeg。
3. `http.py`：单文件 HTTP 下载，支持 Range、`.part` 临时文件、续传、限速、文件大小校验。
4. `fragment.py` + `hls.py` + `dash.py`：分片媒体的公共状态文件、分片并发下载、片段合并、失败片段策略。

Go 侧不照搬 Python 类继承，而是改为：

- `Downloader` 接口：一个协议下载器实现。
- `Registry`：按 `Request.Protocol` 和 URL 选择下载器。
- `Request`/`Progress`/`Result`：稳定的数据契约。
- 分片下载器共享 `FragmentPlan`，后续 HLS/DASH 只负责解析 manifest 并填充片段计划。

## 包边界

`pkg/contentplatform/download`

- 继续保留平台 `Handler`、`Router`、`ResolvedRequest`、任务生命周期。
- 后续 `HTTPExecutor`、`ZipExecutor` 可逐步改成调用 `xdownloader.Engine`。

`pkg/xdownloader`

- 不做平台解析。
- 不持久化业务任务。
- 不依赖前端或数据库。
- 只暴露协议下载能力和进度事件。

## 第一阶段实现

已经放入基础骨架：

- `types.go`：请求、进度、结果、下载器接口。
- `registry.go`：协议选择器，等价于 yt-dlp 的 `get_suitable_downloader` 的 Go 简化版。
- `engine.go`：统一入口，负责选择下载器并执行。
- `http.go`：HTTP/HTTPS 基础下载器，支持 `.part`、断点续传、Range、限速、进度回调、大小校验。

第一阶段可以替换现有 `pkg/contentplatform/download/executor_http.go` 的简单下载逻辑，但建议先保持并行，等测试覆盖齐后再切换。

## 第二阶段：分片核心

新增 `FragmentDownloader`，复用 `FragmentPlan`：

- 使用 `filename.part` 作为合并目标。
- 使用 `filename.ytdl.json` 或同等状态文件记录已完成 fragment index。
- 每个 fragment 下载到 `filename.part-FragN`，成功后追加到目标文件。
- 支持 `Concurrent` 并发下载，但合并按 index 顺序进行。
- 支持 `SkipUnavailableFragments`、`KeepFragments`、`FragmentRetries`。

这层对应 `yt-dlp/downloader/fragment.py`。

## 第三阶段：HLS/DASH

HLS 下载器：

- 解析 m3u8 manifest，生成 `FragmentPlan`。
- 先支持 VOD、普通 TS/fMP4 fragment。
- AES-128 可作为单独能力接入；DRM 直接返回不可下载错误。
- live HLS 默认委托外部下载器，或明确标记 unsupported。

DASH 下载器：

- 接收上游已经解析出的 fragment 列表，或解析 MPD 后生成 `FragmentPlan`。
- live DASH 第一版返回 unsupported。
- 多格式合并留给外部工具或后处理层，不放进底层下载器。

这层对应 `yt-dlp/downloader/hls.py` 和 `dash.py`。

## 第四阶段：外部下载器

引入 `ExternalDownloader`：

- 适配 ffmpeg、aria2c、curl。
- 只在能力匹配时接管，例如 live m3u8、复杂 HLS、需要 mux 的多格式下载。
- 统一透传 headers、cookies、proxy、ratelimit、retries。
- 进度解析作为可选能力；无法解析时至少返回 started/finished/error。

这层对应 `yt-dlp/downloader/external.py`。

## 推荐迁移路径

1. 给 `pkg/xdownloader` 补 HTTP 单元测试和断点续传测试。
2. 在 `pkg/contentplatform/download` 新增一个 `XHTTPExecutor` 或替换 `HTTPExecutor` 内部实现。
3. 将 zip 内部的单文件拉取改用 `xdownloader.HTTPDownloader`，保留 zip 打包逻辑。
4. 实现 `FragmentDownloader`，再接 HLS/DASH。
5. 最后接外部 ffmpeg/aria2c，用能力选择覆盖 native 不适合处理的协议。

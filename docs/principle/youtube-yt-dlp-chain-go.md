# YouTube yt-dlp 下载链路与 Go 版实现参考

本文基于本机源码 `/Users/litao/Documents/temp/yt-dlp` 梳理，目标是作为后续实现 Go 版本 YouTube 下载器时的工程参考。

- 源码版本: `yt-dlp 2026.06.09`
- Git 分支: `master`
- Git 提交: `ad6b5f4`
- 示例 URL: `https://www.youtube.com/watch?v=3ryh7PNhz3E`
- 示例视频 ID: `3ryh7PNhz3E`

## 总览

`yt-dlp` 下载 YouTube 视频不是简单地请求 watch 页面后拿一个 MP4 链接。完整链路分为两段：

1. Extractor 阶段: 输入 URL，提取视频 ID，下载 YouTube 页面和 Innertube API JSON，解析出一组可下载 `formats`。
2. Download 阶段: 选择最佳格式，按单文件或多流下载，必要时用 ffmpeg 合并视频流和音频流。

对 Go 实现来说，建议也按这个边界拆：

- URL 解析器: `URL -> videoID -> canonical watch URL`
- YouTube extractor: `watch page + Innertube player API -> VideoInfo + []Format`
- Player JS challenge solver: 解 `signatureCipher.s` 和 URL 里的 `n`
- Format selector: 实现 `bestvideo*+bestaudio/best`
- Downloader: HTTP 下载、Range/断点续传、分片下载
- Merger: 多流下载后用 ffmpeg `-c copy` 合并

## 1. 命令入口

CLI 入口在 `yt_dlp.__init__._real_main()`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/__init__.py:964`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/__init__.py:1071`

流程：

1. `parse_options(argv)` 解析命令行参数。
2. 创建 `YoutubeDL(ydl_opts)`。
3. 如果没有 `--load-info-json`，调用 `ydl.download(all_urls)`。

对应 Go 版：

```go
func DownloadURL(ctx context.Context, rawURL string) error {
    info, err := extractor.Extract(ctx, rawURL)
    if err != nil {
        return err
    }
    selection := selector.SelectDefault(info.Formats)
    return downloader.Download(ctx, info, selection)
}
```

## 2. 选择 YouTube Extractor

`YoutubeDL.download()` 对每个 URL 调用 `extract_info()`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:3694`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:1671`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/__init__.py:25`

`extract_info()` 会遍历所有 extractor，调用 `ie.suitable(url)` 判断是否支持。YouTube 单视频由 `YoutubeIE` 处理。

`YoutubeIE` 的 URL 正则支持：

- `youtube.com/watch?v=...`
- `youtu.be/...`
- `youtube.com/shorts/...`
- `youtube.com/embed/...`
- `youtube.com/live/...`
- 裸 11 位视频 ID

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:84`

对示例 URL，提取到：

```text
video_id = 3ryh7PNhz3E
webpage_url = https://www.youtube.com/watch?v=3ryh7PNhz3E
```

当前 Go 代码已有类似逻辑：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/handler.go:82`

## 3. 下载 watch 页面

`YoutubeIE._real_extract()` 是 YouTube 单视频的核心入口。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3913`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3896`

流程：

1. `unsmuggle_url()` 处理内部附加参数。
2. `_match_id(url)` 提取视频 ID。
3. 构造 canonical watch URL。
4. 调用 `_initial_extract()`。

`_initial_extract()` 会：

1. 下载 watch 页面。
2. 从页面解析 `ytcfg`。
3. 尝试解析 `ytInitialData`。
4. 调 `_extract_player_responses()` 获取 player responses。

Go 版要解析的页面字段：

- `ytcfg.set({...})`
- `ytInitialPlayerResponse = {...}`
- `PLAYER_JS_URL`
- `WEB_PLAYER_CONTEXT_CONFIGS.*.jsUrl`
- `INNERTUBE_API_KEY`
- `INNERTUBE_CONTEXT`
- `VISITOR_DATA`
- `DATASYNC_ID`
- `STS`

当前 Go 代码已覆盖：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:333`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:660`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:692`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1207`

## 4. 获取 Innertube player responses

`yt-dlp` 不是只依赖 watch 页面内嵌的 `ytInitialPlayerResponse`。它会请求多个 Innertube client，合并返回的 `streamingData`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3033`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:2919`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_base.py:795`

默认 client：

```python
_DEFAULT_CLIENTS = ('android_vr', 'web_safari')
_DEFAULT_JSLESS_CLIENTS = ('android_vr',)
_DEFAULT_AUTHED_CLIENTS = ('tv_downgraded', 'web_safari')
_DEFAULT_PREMIUM_CLIENTS = ('tv_downgraded', 'web_creator')
```

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:143`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_base.py:97`

普通未登录、JS runtime 可用时，默认请求：

- `android_vr`
- `web_safari`

Innertube API 形态：

```http
POST https://www.youtube.com/youtubei/v1/player?key=<INNERTUBE_API_KEY>&prettyPrint=false
Content-Type: application/json
X-YouTube-Client-Name: <client id>
X-YouTube-Client-Version: <client version>
X-Goog-Visitor-Id: <visitorData>
Origin: https://www.youtube.com
User-Agent: <client userAgent>
```

body 结构：

```json
{
  "context": {
    "client": {
      "clientName": "WEB",
      "clientVersion": "2.20260114.08.00",
      "hl": "en",
      "timeZone": "UTC",
      "utcOffsetMinutes": 0
    }
  },
  "videoId": "3ryh7PNhz3E",
  "contentCheckOk": true,
  "racyCheckOk": true,
  "playbackContext": {
    "contentPlaybackContext": {
      "html5Preference": "HTML5_PREF_WANTS",
      "signatureTimestamp": 12345
    }
  }
}
```

`signatureTimestamp` 来自 player JS 或 ytcfg：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:2238`

当前 Go 代码已有两个 client：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:63`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:487`

## 5. 合并 player responses

每个 client 返回的 `playerResponse` 可能包含不同格式。`yt-dlp` 会保留多个 responses，并在 format 提取阶段遍历全部 `streamingData`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3131`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3368`

每个 `streamingData` 会附加内部字段：

- client name
- Innertube context
- PO token fetcher
- 是否已经提供 player PO token
- 是否 Premium
- available timestamp

Go 版最小实现可以不保留这些闭包字段，但至少要保留：

```go
type RawPlayerResponse struct {
    ClientName    string
    StreamingData StreamingData
    VideoDetails  VideoDetails
    Microformat   Microformat
    Playability   PlayabilityStatus
}
```

当前 Go 代码已有去重合并：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:861`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:872`

## 6. 从 streamingData 提取 formats

核心函数是 `_extract_formats_and_subtitles()`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3232`

它处理三类来源：

1. Direct HTTPS formats: `streamingData.formats` 和 `streamingData.adaptiveFormats`
2. HLS manifest: `streamingData.hlsManifestUrl`
3. DASH manifest: `streamingData.dashManifestUrl`

### 6.1 Direct HTTPS

字段来源：

- `itag`
- `url`
- `signatureCipher` 或 `cipher`
- `mimeType`
- `quality`
- `qualityLabel`
- `audioQuality`
- `bitrate`
- `averageBitrate`
- `contentLength`
- `width`
- `height`
- `fps`
- `audioTrack`
- `drmFamilies`

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3368`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3403`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3493`

Go 版 format 建议结构：

```go
type Format struct {
    ID            string
    Itag          int
    URL           string
    Ext           string
    MimeType      string
    AudioCodec    string
    VideoCodec    string
    HasAudio      bool
    HasVideo      bool
    Width         int
    Height        int
    FPS           int
    Bitrate       int
    Filesize      int64
    QualityLabel  string
    Protocol      string // https, hls, dash, http_dash_segments
    SourceClient  string
    HasDRM        bool
}
```

当前 Go 代码已覆盖 direct HTTPS：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:998`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1078`

### 6.2 HLS manifest

如果有 `hlsManifestUrl`，`yt-dlp` 会先处理 manifest URL 上的 `n challenge` 和 PO token，然后调用 m3u8 解析器展开格式。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3675`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3705`

Go 版需要实现：

- 下载 m3u8
- 解析 master playlist
- 解析 stream variants
- 保留 bandwidth、resolution、codecs
- 处理字幕 tracks
- 处理 live 和 VOD 差异

当前 Go 代码只记录警告，未展开 HLS：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1072`

### 6.3 DASH manifest

如果有 `dashManifestUrl`，`yt-dlp` 会处理 URL 上的 `n challenge` 和 PO token，然后调用 MPD 解析器展开格式。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3717`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3743`

Go 版需要实现：

- 下载 MPD XML
- 解析 Period/AdaptationSet/Representation
- 获取 baseURL、segment template、fragment list
- 标记 protocol 为 `dash` 或 `http_dash_segments`

当前 Go 代码只记录警告，未展开 DASH：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1072`

## 7. signatureCipher 解签

YouTube 有些 format 没有直接 `url`，只有：

```text
signatureCipher=url=...&s=...&sp=sig
```

`yt-dlp` 流程：

1. `parse_qs(signatureCipher)`。
2. 取 `url`。
3. 取加密签名 `s`。
4. 取签名参数名 `sp`，默认 `signature`。
5. 调 JS challenge solver 得到真实签名。
6. 追加 `&sp=<decrypted sig>` 到 format URL。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3532`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3556`

当前 Go 代码已有基本实现：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1117`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1264`

注意：`yt-dlp` 新版已经把挑战求解抽象为 `jsc` provider，不再依赖固定 Python 正则。Go 版若想稳定，也不建议只硬编码某个 player.js 函数形态。

## 8. n challenge 解算

即使 format URL 有直接 `url`，其中可能含有 `n=<challenge>`。如果不解，下载可能被限速或失败。

`yt-dlp` 流程：

1. 扫描所有 direct URL 和 manifest URL。
2. 收集所有 `n` challenge。
3. 调 JS challenge solver 批量解算。
4. 用解算结果替换 URL 查询参数 `n`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3381`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3569`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3683`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3721`

当前 Go 代码已有 direct URL 的 `n` 处理：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1154`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1283`

仍需补齐：

- manifest path 中 `/n/<challenge>/` 的替换
- 批量解算和缓存
- solver 抽象层

## 9. JS challenge solver 机制

`yt-dlp` 新版的关键变化：JS challenge solver 是独立 provider framework。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/provider.py:36`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_director.py:40`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/ejs.py:75`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/node.py:20`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/deno.py:32`

`yt-dlp` 的 EJS provider 大致流程：

1. 下载或读取 player JS。
2. 加载 `yt.solver.lib.js` 和 `yt.solver.core.js`。
3. 构造 stdin，把 player JS 和 challenges 传给 solver。
4. 使用 Node/Deno/Bun/QuickJS 等 JS runtime 执行。
5. 返回：
   - `NChallengeOutput`
   - `SigChallengeOutput`
6. 缓存结果。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/ejs.py:148`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/ejs.py:185`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/_builtin/node.py:26`

Go 版建议接口：

```go
type ChallengeType string

const (
    ChallengeN   ChallengeType = "n"
    ChallengeSig ChallengeType = "sig"
)

type ChallengeRequest struct {
    Type       ChallengeType
    PlayerURL  string
    PlayerCode string
    Inputs     []string
    VideoID    string
}

type ChallengeSolver interface {
    Solve(ctx context.Context, req ChallengeRequest) (map[string]string, error)
}
```

实现优先级建议：

1. `GojaSolver`: 直接用 goja 执行 player JS。当前仓库已经有基础。
2. `ExternalNodeSolver`: 类似 `yt-dlp` NodeJCP，把 solver JS 喂给 node。
3. `CachedSolver`: 包一层按 player ID、challenge 缓存。

当前 Go 代码用 goja 直接跑 player JS：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1376`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1383`

风险：

- YouTube player JS 可能引用浏览器 API，纯 goja stub 不一定够。
- 函数名定位靠正则，容易随 player.js 改动失效。
- `yt-dlp` 当前已经转向 EJS solver，说明固定正则路线维护成本高。

## 10. PO Token

新版 YouTube 对某些 client/protocol 会要求 GVS PO Token。`yt-dlp` 会按 client 和 protocol 判断是否需要，如果有 token 会追加到 URL。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3272`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3513`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3581`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3692`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:3730`

当前 Go 代码支持手动配置 PO token 并追加 `pot=`：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:611`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1174`

需要注意：

- `yt-dlp` 支持 `client.gvs+TOKEN` 格式。
- 不同 client/protocol 的 token 要求不同。
- 没有 PO token 时，某些格式应降级或跳过，不应盲目暴露为默认格式。

## 11. 生成 VideoInfo

`YoutubeIE._real_extract()` 最后返回一个 info dict，包含：

- `id`
- `title`
- `description`
- `formats`
- `thumbnails`
- `duration`
- `channel_id`
- `channel_url`
- `uploader`
- `view_count`
- `age_limit`
- `live_status`
- `subtitles`
- `automatic_captions`
- `_format_sort_fields`

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py:4159`

当前 Go 代码已有对应的 `VideoInfo`：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:102`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:935`

## 12. 格式排序与默认选择

`yt-dlp` 在 `process_video_result()` 里做格式清洗、排序、过滤、默认格式选择。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:2832`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:2990`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:3061`

默认 format spec：

```text
bestvideo*+bestaudio/best
```

前提是 ffmpeg 可用。否则倾向单文件 `best`。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:2313`

含义：

- `bestvideo*`: 最佳视频格式，可以包含音频，也可以 video-only。
- `+bestaudio`: 再选最佳音频。
- `/best`: 如果前面的组合不可用，则退回最佳预合并格式。

合并选择时会生成 `requested_formats`：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:2450`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:2490`

Go 版建议：

```go
type Selection struct {
    Format          Format
    RequestedFormats []Format // len > 1 时需要合并
    OutputExt       string
}
```

选择策略：

1. 如果 ffmpeg 可用：
   - 优先最高分 video format
   - 配最佳 audio-only format
   - 输出容器根据 codec 决定，常见为 `mp4`、`webm`、`mkv`
2. 如果 ffmpeg 不可用：
   - 优先带音频和视频的 progressive format，例如 itag 18/22
3. 过滤：
   - DRM
   - 无 URL
   - 需要但无法解签
   - 缺少强制 PO token 的格式

当前 Go 代码现在只是按 `formatScore` 排序并暴露单个 format：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1049`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:227`

## 13. 下载器选择

`YoutubeDL.dl()` 根据 format protocol 选择下载器。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:3283`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/__init__.py:4`

protocol 映射：

- `http`/`https` -> `HttpFD`
- `m3u8_native` -> `HlsFD`
- `m3u8` -> `FFmpegFD`
- `http_dash_segments` -> `DashSegmentsFD`
- 其他特殊协议按 map 分发

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/__init__.py:41`

Go 版建议 SourceExecutor：

- `HTTPExecutor`
- `HLSExecutor`
- `DASHExecutor`
- `MultiFormatExecutor`
- `FFmpegMergeExecutor`

当前仓库下载 executor 是单 URL HTTP copy：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/download/executor_http.go:37`

## 14. HTTP 下载细节

`HttpFD.real_download()` 处理：

- `Accept-Encoding: identity`
- `.part` 临时文件
- 断点续传
- Range 请求
- content length 校验
- block size 自适应
- 重试
- 下载完成后 rename

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/http.py:23`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/http.py:76`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/http.py:248`
- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/http.py:343`

Go 版最小实现：

```go
func DownloadHTTP(ctx context.Context, url string, dest string, headers http.Header) error {
    tmp := dest + ".part"
    resumeFrom := existingSize(tmp)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    req.Header.Set("Accept-Encoding", "identity")
    if resumeFrom > 0 {
        req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
    }
    // copy body to tmp, then os.Rename(tmp, dest)
    return nil
}
```

## 15. 多流下载与 ffmpeg 合并

当 format selection 产生 `requested_formats` 时，`yt-dlp` 进入多格式下载分支。

关键源码：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py:3482`

流程：

1. 为每个 requested format 生成独立临时文件名。
2. 分别下载视频-only、音频-only。
3. 下载成功后加入 `__files_to_merge`。
4. 添加 `FFmpegMergerPP` postprocessor。
5. 调 ffmpeg 合并。

合并器：

- `/Users/litao/Documents/temp/yt-dlp/yt_dlp/postprocessor/ffmpeg.py:822`

ffmpeg 参数核心：

```text
ffmpeg -i video -i audio -c copy -map 0:v:0 -map 1:a:0 output
```

源码中 `FFmpegMergerPP.run()` 会：

1. 遍历 `requested_formats`。
2. 对有 audio 的输入加 `-map i:a:0`。
3. 对有 video 的输入加 `-map i:v:0`。
4. 对 m3u8 AAC 做 `aac_adtstoasc` 修复。
5. 输出临时文件，再 rename。

Go 版建议：

```go
type MultiFormatDownload struct {
    Formats []Format
    Output  string
}

func DownloadAndMerge(ctx context.Context, item MultiFormatDownload) error {
    files := make([]string, 0, len(item.Formats))
    for _, f := range item.Formats {
        path := tempPathForFormat(item.Output, f.ID, f.Ext)
        if err := DownloadHTTP(ctx, f.URL, path, f.Headers); err != nil {
            return err
        }
        files = append(files, path)
    }
    return MergeWithFFmpeg(ctx, files, item.Formats, item.Output)
}
```

当前仓库缺口：

- `ResolvedRequest.Download` 当前只表达单 URL。
- `HTTPExecutor` 只下载单个 source。
- Pipeline 有 post 节点，但 YouTube handler 当前没有声明“下载多个源并合并”的节点。

相关源码：

- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/download/types.go:178`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/download/executor_http.go:37`
- `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/handler.go:65`

## 16. 针对当前 Go 仓库的实现路线

### 已有能力

当前仓库 `pkg/contentplatform/youtube` 已经覆盖：

- YouTube URL 匹配和视频 ID 提取  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/handler.go:82`

- watch 页面下载  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:425`

- `ytInitialPlayerResponse` 提取  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:660`

- `ytcfg.set(...)` 提取  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:680`

- `android_vr` 和 `web_safari` client  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:63`

- `/youtubei/v1/player` 请求  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:487`

- direct HTTPS formats 解析  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:998`

- `signatureCipher` 解签  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1117`

- `n` challenge 基础解算  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1154`

- PO token 追加  
  `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go:1174`

### 优先补齐项

按收益排序：

1. 多流选择和合并
   - 增加 `requested_formats` 概念。
   - 默认选择 `bestvideo*+bestaudio/best`。
   - 下载 video-only 和 audio-only。
   - ffmpeg `-c copy` 合并。

2. 下载管线扩展
   - `DownloadSpec` 支持多个源，或新增 `Protocol: "multi"`。
   - 新增 `MultiSourceExecutor`。
   - 新增 `ffmpeg_merge` pipeline 节点。

3. HLS/DASH manifest 展开
   - m3u8 parser。
   - MPD parser。
   - manifest URL 上的 `n` 和 `pot` 处理。

4. JS challenge solver 抽象
   - 把当前 `playerResolver` 拆成 `ChallengeSolver` interface。
   - 保留 goja solver。
   - 增加外部 Node/Deno solver，以对齐 `yt-dlp` 的 EJS 思路。
   - 按 player ID 缓存 `sig` spec 和 `n` 结果。

5. Innertube client 策略
   - 按登录状态、Premium、JS runtime 可用性选择 client。
   - 支持跳过不支持 cookie 的 client。
   - 支持 client 级别 PO token policy。

## 17. Go 版模块设计建议

建议目录：

```text
pkg/contentplatform/youtube/
  handler.go
  client.go
  innertube.go
  player.go
  challenge.go
  formats.go
  selector.go
  manifest_hls.go
  manifest_dash.go
  merge.go
```

建议核心接口：

```go
type Extractor interface {
    Extract(ctx context.Context, rawURL string) (*VideoInfo, error)
}

type PlayerResolver interface {
    PlayerURL() string
    SignatureTimestamp(ctx context.Context) (string, error)
    SolveSignature(ctx context.Context, input string) (string, error)
    SolveN(ctx context.Context, input string) (string, error)
}

type FormatSelector interface {
    Select(info *VideoInfo, opts SelectOptions) (*Selection, error)
}

type MediaDownloader interface {
    Download(ctx context.Context, info *VideoInfo, selection *Selection, dest string) error
}
```

## 18. 最小可用链路

如果先做一个能稳定下载大多数公开视频的 Go 版本，可以先实现：

1. URL 提取 video ID。
2. GET watch 页面。
3. 解析 `ytcfg` 和 `ytInitialPlayerResponse`。
4. 用 `android_vr` 请求 `/youtubei/v1/player`。
5. 用 `web_safari` 请求 `/youtubei/v1/player`。
6. 合并 direct HTTPS formats。
7. 解 `signatureCipher.s`。
8. 解 URL 查询参数 `n`。
9. 加用户配置的 PO token。
10. 选择：
    - 如果 ffmpeg 可用，选最佳 video-only + 最佳 audio-only。
    - 否则选最佳 progressive。
11. HTTP 下载。
12. 多流时 ffmpeg 合并。

## 19. 对齐 yt-dlp 的关键行为清单

实现后可以用这个清单验收：

- 输入 `https://www.youtube.com/watch?v=3ryh7PNhz3E` 能提取 `3ryh7PNhz3E`。
- watch 请求带合理 UA、Accept-Language、Cookie。
- 能解析 `ytcfg.set(...)`。
- 能解析 `ytInitialPlayerResponse`。
- 能请求 `/youtubei/v1/player`。
- 请求 player API 时带 `X-YouTube-Client-Name` 和 `X-YouTube-Client-Version`。
- 能获取 player JS URL。
- 能提取 `signatureTimestamp`。
- 能处理 direct `url` format。
- 能处理 `signatureCipher` format。
- 能处理 `n` challenge。
- 能跳过 DRM format。
- 能识别 audio-only、video-only、progressive。
- 默认优先 video-only + audio-only。
- ffmpeg 不可用时回退 progressive。
- 下载时 `Accept-Encoding: identity`。
- 支持 `.part` 临时文件和断点续传。
- 多流下载后用 ffmpeg 合并。
- HLS/DASH manifest 至少不误选为 direct URL。
- 缺 PO token 的格式能降级或提示。

## 20. 参考源码索引

yt-dlp 关键文件：

- CLI 入口: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/__init__.py`
- 下载调度: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/YoutubeDL.py`
- YouTube extractor: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_video.py`
- YouTube base/client 配置: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/_base.py`
- JS challenge provider: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/extractor/youtube/jsc/`
- HTTP downloader: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/http.py`
- downloader 分发: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/downloader/__init__.py`
- ffmpeg 合并: `/Users/litao/Documents/temp/yt-dlp/yt_dlp/postprocessor/ffmpeg.py`

当前 Go 仓库关键文件：

- YouTube handler: `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/handler.go`
- YouTube client/extractor: `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/youtube/client.go`
- 下载协议类型: `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/download/types.go`
- HTTP executor: `/Users/litao/Documents/workspace/wx_channels_download/pkg/contentplatform/download/executor_http.go`
- 旧 YouTube 包: `/Users/litao/Documents/workspace/wx_channels_download/pkg/scraper/youtube/youtube.go`

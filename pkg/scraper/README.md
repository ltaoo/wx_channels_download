# 平台提取、模型适配与媒体执行

代码按照 Bilibili 的边界组织：平台抓取包负责从源站提取内容，平台适配器负责解析抓取结果并转换为下载器的数据模型。

| 平台 | 提取包 | 适配器 | 平台 ID |
| --- | --- | --- | --- |
| 爱奇艺 | `pkg/scraper/iqiyi` | `internal/adapter/iqiyi` | `iqiyi` |
| 腾讯视频 | `pkg/scraper/vqq` | `internal/adapter/vqq` | `tencent`，兼容已有记录 |
| 芒果 TV | `pkg/scraper/mgtv` | `internal/adapter/mgtv` | `mgtv` |
| 优酷 | `pkg/scraper/youku` | `internal/adapter/youku` | `youku` |
| 红果 | `pkg/scraper/hongguo` | `internal/adapter/hongguo` | `hongguo` |

## 职责边界

`pkg/scraper/<平台>` 提供各自的 `FetchResult`、`Fetch`、`FetchContext`、`ResolveContext` 和 `ParseURL`，负责源站请求、身份核对、签名、分集枚举、播放信息和页面元数据提取。提取客户端不提供 `Download`，不依赖下载任务、ORM 或内容数据库模型。`pkg/scraper/internal/extract` 仅复用提取流程。

`internal/adapter/<平台>` 解码和校验对应平台的提取结果，适配成 `Content`、`Account`、系列/分集详情、视频表示、下载任务、资源端点和资产关联。其 `convert.go` 接受平台原生结果对象及其 JSON 表示；从抓取结果构建任务预览时无需再次抓取。通用运行时、模型转换及后处理复用 `internal/adapter/internal/vod`，该包不选择源平台。`Postprocess` 负责分片合并、独立音轨合并、解密、MP4 封装与媒体校验，随后回填媒体信息；与 YouTube 适配器一样，在 Hermes 最终命名和标记完成之前执行。

`internal/adapter/internal/vod/prepare.go` 在任务开始时刷新播放信息，将每集展开为独立的 MP4、HLS 分片、初始化片段和密钥资源，事务性保存到已有的资源、端点表。Hermes 随后通过 HTTP/HTTPS 驱动直接下载这些文件，各资源拥有自己的进度、分段状态和重试记录。请求头与重定向策略由适配器提供，账号 Cookie 仅用于元数据接口。腾讯特殊 CDN 的小范围读取通过流式 HTTP transport 适配，不写临时媒体文件。

`pkg/hls` 提供无平台依赖的有限媒体清单与属性列表解析，返回原始 URI、确定性的本地引用及总时长；它不发起网络请求，也不判断平台域名。`pkg/scraper/internal/source/hls_policy.go` 通过平台显式提供的策略逐项校验分片、密钥与初始化对象的地址，再按源站声明检查完整时长。主清单选择、相对地址解析与网络请求仍在源请求层处理。

`pkg/scraper/types.go` 定义共享提取类型 `Profile`、`Episode`、`PlayInfo` 与 `Video`。请求、页面元数据及 HLS 取流助手位于 `pkg/scraper/internal/source`，只允许提取层使用；各平台在自己的 `client.go` 提供域名与 HTTPS 策略，不再进行全局平台注册。播放源携带地址校验回调，供适配器检查下载端点和重定向。

`pkg/media`、`internal/download/media`、TAR 资源包和内层下载任务已移除。后处理通过硬链接读取 Hermes 下载的原始文件，只生成本地清单与最终 MP4。

Hermes 提供通用的 `Preprocessor` 与 `Postprocessor` 接口，不依赖平台或分集模型。新任务的 `media` 标记表示适配器的延迟解析输入，启动时即展开为 HTTP/HTTPS 端点，不注册为 Hermes 下载协议；旧的 `drama` 输入可在同一步识别。旧中转格式若已有下载进度，需要重新创建任务。腾讯继续使用 `tencent` 平台 ID。

```go
// 提取阶段：仅获得源平台内容与播放信息。
client := youku.NewClient(cookie)
raw_result, err := client.FetchContext(ctx, page_url)

// 解析适配阶段：对应平台适配器构造下载器的数据模型。
handler := adapter.Get("youku")
models, err := handler.(adapter.FetchDownloadTaskBuilder).
    BuildDownloadTaskFromFetch(raw_result, json.RawMessage(`{}`))
```

## 输入与范围

| 平台 | 支持的官网 URL | 范围 |
| --- | --- | --- |
| 爱奇艺 | `https://www.iqiyi.com/v_编号.html`、`/a_编号.html` | 当前分集或专辑中的完整正片列表 |
| 优酷 | `/v_show/id_X….html`、移动页 `?vid=X…`；剧集页 `/v_nextstage/id_剧集ID.html` | 视频入口下载当前集并补充所属剧集资料；剧集入口获取整剧详情及正片列表 |
| 腾讯视频 | `/x/cover/CID.html`、`/detail/首字母/CID.html`、`/x/cover/CID/VID.html`、`/x/page/VID.html` | 完整可核对的正片列表，或指定视频；列表分页、缺集或版本冲突会报错，需改用分集 URL |
| 芒果 TV | `https://www.mgtv.com/b/专辑ID/视频ID.html` | 当前视频及所属系列资料 |
| 红果 | `https://hongguoduanju.com/detail?series_id=ID`、`/player/系列ID`、`/player/系列ID/分集ID` | 整剧或指定分集 |

输入 URL 后自动识别平台。`iqiyi.cookie`、`youku.cookie`、`tencent.cookie`、`mgtv.cookie` 使用具有对应播放权限的账号 Cookie；未显式配置时尝试已有 Cookie 存储。红果使用匿名设备注册和签名 App 播放接口。Cookie 只发给官方元数据接口，不发给媒体 CDN。

爱奇艺适配器通过 `FetchDetails` / `FetchDetailsContext` 采集官方初始化与目录接口中的内容资料，不要求当场解析片源。剧名、简介、封面、年份、总集数、已列正片数和更新状态可以独立保存；视频链接仍只创建当前分集的资源，所属剧集保留完整目录数量。站点的 `publish_date` 不作为首播日期。开始下载时调用 `ResolveContext` 获取片源；直接调用 `Fetch` / `FetchContext` 仍要求首集可播放。播放响应同时检查 `code`、分集编号、`data.st`、授权与试看标记：`A00000` 搭配 `st=502`、`program:[]` 表示当前网络地区受版权限制，不能报成 JSON 解析失败或分集不匹配。版权、登录、DRM、网络错误和空片源分别报告，完整播放授权与清单时长检查仍然生效。

腾讯适配器同样通过 `FetchDetails` / `FetchDetailsContext` 独立采集带 CID 的剧集链接资料，开始下载时再解析片源；不含 CID 的 `/x/page/VID.html` 链接保留原有播放信息提取流程。资料读取 `pc_introduction.children_list` 的实际内容卡片，保留剧名、类型、简介、海报、年份、首播日期、总集数、已列正片数及更新提示。指定先导片或预告片链接时保留该视频身份、时长及标记，不将其改成第 1 集；整剧入口过滤这些视频。目录的 `is_all_data` 只表示当前标签页完整，存在其他未选分页时提示使用分集链接，避免将部分目录当作整剧。播放接口默认请求 H.264，并把接口实际返回的编码传给适配器；如果返回 HEVC，后处理为 MP4 添加 `hvc1` 标记。账号仅获试看时仍拒绝将试看作为完整正片保存。

优酷旧 UPS 返回 `-6004` 时，自动进入纯 HTTP + Goja 的新版主动提取：从官网播放页（必要时从公开首页）发现播放器与 AWSC 脚本地址；两个页面均未发布脚本地址时，使用已验证的官方 SDK 地址。读取下载的 JS 中的播放器版本、客户端编号和 UAB 模块地址，在隔离的 Goja 运行环境中执行官网下载的 UAB 脚本生成 `ckey`，随后计算 MTOP 签名并请求完整片源。脚本请求只允许指定的 `g.alicdn.com` SDK 路径，不发送 Cookie；仅执行签名模块，不执行网页脚本，也不提供文件、进程或网络绑定。Goja 执行限时 10 秒，MTOP 令牌错误最多请求 3 次，账号或播放业务错误不反复尝试。使用独立内存 CookieJar 处理令牌刷新，不修改调用方 CookieJar，不保存固定 `ckey`、请求签名或账号 Cookie。整个流程不调用浏览器，不依赖代理拦截或用户手动播放；官网返回账号、地区、权限或不兼容片源错误时仍会报错。

优酷 `FetchSeriesDetails` / `FetchSeriesDetailsContext` 接受视频或剧集链接，通过官方 Columbus MTOP 接口提取剧名、简介、海报、年份、国家、类型、更新状态、导演、编剧、演员及角色，以及各集编号、集序、标题和封面。剧集入口的 `Fetch` 只获取详情与目录，创建预览时不解析视频片源，开始下载各集时才运行 Goja 播放提取。视频入口仍只返回一个下载资源，完整目录保存在剧集资料中；目录服务暂不可用时保留已成功提取的视频。正片按官方 `isZp` 和 `videoType` 过滤，排除预告、看点与推荐组件；分页沿用官方组件会话，拒绝缺集、集序冲突、身份变化及无进展分页。`episode_count` 为平台声明总集数，`available_episode_count` 为已列出的正片数；未完结作品不补造未发布分集。只提供年份时保存年份，不捏造首播日期。

适配器后处理需要服务端 PATH 中存在 `ffmpeg` 和 `ffprobe`。提取器选择可用清晰度，下载层保存 HLS 分片及独立音轨，适配器将其合并为 MP4；红果支持参考项目的 Spade URL 和媒体密钥解析。只有完整时长、尺寸和视频解码检查通过的文件才进入完成状态；试看、截断清单、身份不匹配及不支持的 DRM 会报错。

播放源在任务开始、恢复或重试时重新提取，避免使用过期链接。同一片源刷新签名 URL 时保留资源 ID 和已下载偏移；清单结构、媒体参数或密钥发生变化时拒绝复用原有分片，提示重新创建任务。任务创建预览仍不额外抓取分集。

后处理在原始文件所在文件系统中创建硬链接与本地清单，合并、解密、完整时长和视频解码检查全部成功后才原子替换主资源；失败保留已下载文件供重试。媒体信息归档成功后清理辅助文件及资源行，最终只保留 MP4 和用户选择的封面。

## 数据表复用

业务表结构沿用既有设计。`000007_drama_platforms` 补充四个平台，`000008_iqiyi_platform` 补充爱奇艺。

| 信息 | 存储位置 |
| --- | --- |
| 系列标题、简介、封面、首播日期、集数 | `content(type=collection, subtype=series)` + `content_series` |
| 剧集国家、类型、更新状态、目录数量 | `content_series.origin_country_json`、`genres_json`、`status`、`source_count` |
| 分集标题、集序、时长 | `content(type=video, subtype=episode)` + `content_episode` + `content_video` |
| 分集所属系列 | `content_relation(type=episode_of)`，由分集指向系列 |
| 电影标题、简介、封面、技术参数 | `content(type=video, subtype=movie)` + `content_video` |
| 演员、导演、编剧与角色 | `influencer` + `content_influencer`；角色保存在关系元数据中 |
| 年份、上映日期及平台资料 | `content.metadata` 中的 Profile；没有独立 `content_movie` 表 |
| 视频表示、下载文件及关联 | `content_video_variant`、`content_asset`、`download_resource_asset` |

影片类型参考官方接口、JSON-LD 和 Open Graph。缺少明确电影类型时保留普通视频类型；未知集序、上映日期和演职员不补造。优酷视频入口会补充完整剧集目录，但下载资源仍为当前集；芒果单集入口不枚举整剧。单集任务的根内容仍为当前视频，剧集首页和单集编号保持独立稳定。

## 验证与来源

平台包测试覆盖源站请求、签名、分集及权限校验；各平台适配器测试覆盖提取对象、JSON 和前端通用对象到下载器模型的转换。`pkg/scraper/integration` 覆盖跨平台内容提取与按需解析；Hermes 逐资源下载、适配器后处理和服务层测试覆盖本地合成媒体、完整时长、独立音轨、解密、范围下载、取消清理及归档落库。没有启动或自动化浏览器；真实平台账号权限、地区和接口可用性由用户使用实际 URL 验收。

协议与媒体实现参考本机 `dramadl`。原始实现及上游署名保留在 [NOTICE](NOTICE)，相应改编源码的 GPL-3.0-only 许可证保留在 [LICENSE](LICENSE)。

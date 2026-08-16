# 网络内容归档数据模型

## 1. 设计目标

本项目是一个通用内容归档工具，需要覆盖视频、音乐、小说、漫画、短内容、长文章、图集、AI 对话等常见网络内容，并支持：

- 同一种内容存在多种可下载表示，例如视频分辨率、音质、字幕格式。
- 内容包含章节、图片、漫画页、课时等嵌套实体。
- 同一个逻辑资产可以被多次下载、重新下载或从不同端点下载。
- 从视频规格、字幕、小说章节、图集图片等对象直接找到实际下载记录。
- 表达合集、系列、回复、引用、转发、翻译等内容间关系。
- 完整保留 AI 对话的消息树、重新生成分支、多模态内容和生成产物。
- 在增加新内容类型时复用现有资产和下载关联结构。

核心原则是将“内容本身”“逻辑归档资产”“实际下载记录”分开存储：

```text
Content（一个网络内容）
├── 类型扩展：ContentVideo / ContentNovel / ContentAlbum / ContentConversation / ...
├── 嵌套实体：章节 / 图片 / 漫画页 / 课程课时 / 对话消息
├── ContentAsset（可归档的逻辑资产）
│   └── DownloadResourceAsset
│       └── DownloadResource（一次实际下载）
│           └── DownloadEndpoint（下载地址或镜像）
└── ContentRelation（系列、回复、引用、转发等关系）
```

## 2. 常见内容类型与场景

| `Content.Type` | 常见 `Subtype` 或场景 | 典型归档资产 |
| --- | --- | --- |
| `video` | 短视频、长视频、电影、剧集、片段、直播回放 | 多分辨率视频、独立音轨、字幕、封面 |
| `audio` | 音乐、有声书、语音、广播 | 多音质音频、歌词、封面 |
| `podcast` | 播客节目、播客单集 | 音频、Transcript、章节信息、封面 |
| `live` | 视频直播、语音房 | Manifest、录制视频、分段文件、封面 |
| `image` | 单张图片、插画、海报 | 原图、缩略图、不同尺寸 |
| `album` | 图集、相册、抖音或小红书图文 | 多张图片、Live Photo、背景音乐 |
| `article` | 长文章、博客、新闻、Newsletter、问答、Wiki | HTML、Markdown、纯文本、正文图片、附件 |
| `post` | 微博、Twitter/X、动态、评论、Thread | 短文本、图片、视频、引用附件 |
| `novel` | 网络小说、电子小说 | 卷、章节、TXT、HTML、EPUB、PDF |
| `comic` | 漫画、条漫、连载 | 章节、漫画页、CBZ、PDF |
| `document` | PDF、电子书、幻灯片、电子表格 | 原始文件、预览图、提取文本 |
| `course` | 在线课程、教程系列 | 课时视频、字幕、讲义、附件 |
| `collection` | 播放列表、系列、合集、Feed | 内容之间的包含和排序关系 |
| `webpage` | 网页快照、书签、存档页面 | HTML、截图、网页资源、源快照 |
| `conversation` | ChatGPT、豆包等 AI 对话、人类聊天、邮件线程 | 消息树、分支、内容块、附件、生成文件、导出快照 |
| `other` | 数据集、软件包、源码、压缩包等 | Data、Archive、Binary |

### 2.1 Type 和 Subtype 的职责

`Content.Type` 保持稳定和宽泛，平台差异、内容形态差异放入 `Content.Subtype`：

```text
article  / answer
article  / newsletter
video    / short_video
audio    / audiobook
document / pdf
```

旧数据中的 `answer`、`music`、`image_set` 等类型会在迁移时归一到稳定大类，同时把原类型保留到 `subtype`。

### 2.2 项目当前可直接回归的平台

以下平台在当前代码中已经有实际的内容或下载适配器，适合优先用于端到端测试。测试时应选择自己发布、公开授权或公共领域的内容。

| 平台 | 适合测试的内容 | 建议至少准备的样本 | 重点验证 |
| --- | --- | --- | --- |
| [哔哩哔哩](https://www.bilibili.com/) | 普通视频、分 P 视频、番剧或课程视频 | 普通单视频、DASH 音视频分离视频、分 P 视频各一条 | `video_variant`、`audio_variant` 是否分别建资产；多个 Resource 是否指向正确资产 |
| [抖音](https://www.douyin.com/) | 短视频、图文或图集 | 无明确分辨率短视频、单图图文、多图图文、带背景音乐图集 | 短视频是否生成 `default` 规格；图片 URI 是否稳定；图片和 BGM 是否分开关联 |
| [微信视频号](https://channels.weixin.qq.com/) | 短视频、长视频、图集、背景音乐、直播 | 普通视频、多规格视频、多图动态、带 BGM 图集、自己可控制的直播 | `spec` 对应的 Variant、动态下载 URL 更新、图片 decode key、直播 Stream Resource |
| [微信公众号](https://mp.weixin.qq.com/) | HTML 长文章、正文图片、图片消息、Live Photo | 普通文章、多图文章、纯图片消息、包含 Live Photo 的自有素材 | 正文快照、正文图片、图集图片、Live Photo 视频是否成为不同资产 |
| [知乎](https://www.zhihu.com/) | 问题、回答、专栏文章 | 一个问题、一个回答、一篇专栏文章、一个含多图回答 | 是否分别归一为 `article/question`、`article/answer`、`article`；HTML 和附件是否正确归档 |
| [番茄小说](https://fanqienovel.com/) | 小说、卷、章节 | 有多个卷的公开小说、章节标题重复的小说、章节 URL 可更新的小说 | `volume_key`、`chapter_key`、章节顺序以及章节到 DownloadResource 的直接关联 |
| [69书吧](https://www.69shuba.com/) | 小说目录、章节 HTML、简介页快照 | 短篇小说、长目录小说、存在相对章节链接的小说 | 简介页 `source_snapshot`、章节 HTML 资产、相对 URL 解析和章节关联 |

当前代码中的 YouTube、微博和小红书目录主要提供配置或浏览侧入口，不应暂时当作“下载适配器已经完整支持”的验收项。它们仍然很适合作为下一阶段模型覆盖和适配器开发的测试目标。

### 2.3 用于扩展测试的常见平台

下表用于验证数据模型是否能覆盖主流网络内容。标记为扩展测试的平台通常还需要新增或完善适配器。

| 内容类型 | 国内常见平台 | 国外常见平台 | 重点场景 |
| --- | --- | --- | --- |
| 视频 | [哔哩哔哩](https://www.bilibili.com/)、[抖音](https://www.douyin.com/)、微信视频号、[快手](https://www.kuaishou.com/)、[西瓜视频](https://www.ixigua.com/) | [YouTube](https://www.youtube.com/)、[TikTok](https://www.tiktok.com/)、[Vimeo](https://vimeo.com/)、[Dailymotion](https://www.dailymotion.com/) | 多分辨率、DASH 音视频分离、HDR、不同编码、多音轨、付费或登录内容 |
| 音乐和音频 | [网易云音乐](https://music.163.com/)、[QQ 音乐](https://y.qq.com/)、[喜马拉雅](https://www.ximalaya.com/) | [Spotify](https://open.spotify.com/)、[SoundCloud](https://soundcloud.com/)、[Bandcamp](https://bandcamp.com/)、[Apple Podcasts](https://podcasts.apple.com/) | 多音质、专辑、歌词、封面、单曲与节目关系、付费试听 |
| 播客 | [小宇宙](https://www.xiaoyuzhoufm.com/)、喜马拉雅、网易云播客 | Spotify、Apple Podcasts、[Pocket Casts](https://pocketcasts.com/) | 节目和单集关系、Transcript、章节时间点、季和集序号 |
| 直播 | [哔哩哔哩直播](https://live.bilibili.com/)、[虎牙](https://www.huya.com/)、[斗鱼](https://www.douyu.com/)、抖音直播、视频号直播 | [Twitch](https://www.twitch.tv/)、YouTube Live | HLS/DASH Manifest、分段、断线重连、滚动录制、直播与回放关系 |
| 图片和图集 | [小红书](https://www.xiaohongshu.com/)、[微博](https://weibo.com/)、[LOFTER](https://www.lofter.com/) | [Instagram](https://www.instagram.com/)、[Flickr](https://www.flickr.com/)、[Imgur](https://imgur.com/)、[Pinterest](https://www.pinterest.com/) | 原图和缩略图、动态 CDN URL、多图顺序、动图、Live Photo、图片说明 |
| 长文章 | 微信公众号、知乎、[简书](https://www.jianshu.com/) | [Medium](https://medium.com/)、[Substack](https://substack.com/)、[WordPress](https://wordpress.com/)、[DEV Community](https://dev.to/) | HTML、Markdown、正文图片、脚注、代码块、附件、更新版本 |
| 短内容和讨论 | 微博、知乎想法、[豆瓣](https://www.douban.com/) | [X](https://x.com/)、[Threads](https://www.threads.net/)、[Mastodon](https://joinmastodon.org/)、[Bluesky](https://bsky.app/)、[Reddit](https://www.reddit.com/) | 回复、引用、转发、Thread、投票、外链卡片、删除或设为私密 |
| 小说 | 番茄小说、[起点中文网](https://www.qidian.com/)、[晋江文学城](https://www.jjwxc.net/)、69书吧 | [Project Gutenberg](https://www.gutenberg.org/)、[AO3](https://archiveofourown.org/)、[Royal Road](https://www.royalroad.com/) | 卷、章节、锁定章节、重复标题、章节重排、多格式电子书 |
| 漫画 | [哔哩哔哩漫画](https://manga.bilibili.com/)、[腾讯动漫](https://ac.qq.com/)、[快看漫画](https://www.kuaikanmanhua.com/) | [WEBTOON](https://www.webtoons.com/)、[MANGA Plus](https://mangaplus.shueisha.co.jp/)、[ComicWalker](https://comic-walker.com/) | 章节和页面、长条图、双页、阅读方向、CBZ/PDF、付费章节 |
| 文档 | 百度文库、豆丁文档、公开飞书文档 | [arXiv](https://arxiv.org/)、[Internet Archive](https://archive.org/)、[SlideShare](https://www.slideshare.net/)、[Issuu](https://issuu.com/) | PDF、EPUB、PPT、预览图、OCR 文本、附件版本 |
| 课程 | 哔哩哔哩课程、[中国大学 MOOC](https://www.icourse163.org/)、[学堂在线](https://www.xuetangx.com/) | [Coursera](https://www.coursera.org/)、[edX](https://www.edx.org/)、[Khan Academy](https://www.khanacademy.org/)、[Udemy](https://www.udemy.com/) | 课程、章节、课时、字幕、讲义、测验附件、学习权限 |
| 合集和播放列表 | 哔哩哔哩合集、网易云歌单、微信公众号合集 | YouTube Playlist、Spotify Playlist、Podcast Show | `contains`、`part_of`、排序、分页、内容增删 |
| 网页快照 | [维基百科](https://zh.wikipedia.org/)、GitHub Pages、新闻网站 | [Wikipedia](https://www.wikipedia.org/)、[Internet Archive Wayback Machine](https://web.archive.org/) | HTML、CSS、图片、页面截图、Canonical URL、更新时间 |
| 会话和邮件 | 微信导出、QQ 导出、飞书或钉钉导出 | Telegram Export、Discord 数据包、Slack Export、MBOX 邮件 | 仅测试自己的导出数据；消息顺序、回复链、附件、编辑和撤回 |
| AI 对话 | [豆包](https://www.doubao.com/)、腾讯元宝、Kimi、通义、文心一言、DeepSeek | [ChatGPT](https://chatgpt.com/)、Claude、Gemini、Microsoft Copilot、Perplexity | 消息父子树、编辑和重新生成、模型切换、多模态输入、引用、工具调用、Canvas/Artifact、生成文件 |
| 数据和软件包 | Gitee Release、模型或数据附件 | [GitHub Releases](https://github.com/)、[Hugging Face](https://huggingface.co/)、[Kaggle](https://www.kaggle.com/)、[Zenodo](https://zenodo.org/)、[PyPI](https://pypi.org/) | 版本、校验和、压缩包、分片、大文件、许可证和依赖关系 |

### 2.4 推荐的最小验收样本集

为了尽快验证当前数据模型，可以先准备以下 14 组样本：

1. 一条没有字幕、没有明确分辨率的抖音短视频。
2. 一条音视频分离的哔哩哔哩视频。
3. 一条具有两个以上规格的视频号视频。
4. 一条具有两种语言、每种语言两种格式字幕的测试视频；当前可先使用测试 Fixture，后续可接入 YouTube 公开样本。
5. 一条包含三张以上图片和背景音乐的抖音或视频号图集。
6. 一条包含普通图片和 Live Photo 的自有公众号图片消息。
7. 一篇包含正文、图片、代码块和外部附件的公众号或知乎文章。
8. 一部至少包含两个卷、十个章节的番茄小说。
9. 一部章节页面使用相对 URL 的 69书吧小说。
10. 同一个视频规格连续下载两次，用于验证一个 ContentAsset 对应多个 DownloadResource。
11. 同一资源提供两个 DownloadEndpoint，用于验证端点优先级和失效切换。
12. 手工构造一个播放列表、回复或转发关系，用于验证 `ContentRelation` 的方向和排序。
13. 导入一段包含“编辑问题”和“重新生成回答”的 ChatGPT 或豆包对话，验证消息树和多个分支没有被展平成重复文本。
14. 导入一段包含图片上传、网页引用、工具调用和生成文件的 AI 对话，验证消息内容块与 DownloadResource 的关联。

### 2.5 每个平台的通用验收检查

每个样本下载完成后至少检查：

- `Content.Id` 是否稳定，重复抓取时没有产生第二条内容。
- `Content.Type` 是否为稳定大类，平台内容形态是否进入 `Subtype`。
- 每个文件是否拥有唯一且稳定的 `(content_id, role, asset_key)`。
- 下载 URL 的 token 更新后是否复用原 ContentAsset，只新增或更新下载实例及端点。
- 同一个 ContentAsset 是否允许关联多个 DownloadResource。
- 一个 DownloadResource 是否可以通过 `DownloadResourceAsset` 关联一个或多个资产。
- 章节、图片等嵌套对象是否能通过 `ContentAssetLink` 找到 DownloadResource。
- 删除、私密、锁定、登录过期、下载失败时是否保留内容元数据和失败状态。
- 标题包含中文、Emoji、斜杠、换行或超长字符时，逻辑 ID 是否不依赖文件名。
- 重复下载、断点续传、端点切换和派生文件是否不会破坏逻辑资产身份。
- AI 对话重新生成回答后是否保留原分支，当前分支是否可从根消息正确还原。
- AI 对话中的附件、生成图片、Canvas/Artifact 和引用是否关联到正确的消息内容块。

### 2.6 测试数据使用约束

- 优先使用自己发布的内容、平台官方示例、公共领域内容或明确授权内容。
- 不绕过 DRM、付费墙、访问控制或平台验证码。
- 登录态、Cookie、PO Token 等凭据只放在本地配置中，不写入 Fixture、日志或数据库 Metadata。
- 直播、私密内容和会话数据优先使用自己控制的测试账号。
- 外部平台页面和接口会变化，回归测试应保存脱敏后的原始响应 Fixture，线上冒烟测试与离线解析测试分开执行。

## 3. Content：内容主体

`Content` 表示一个独立、可识别的网络内容，常用字段包括：

```text
Content
- id                 通常为 platform_id:external_id
- platform_id
- type
- subtype
- external_id
- title
- description
- source_url
- cover_url
- publish_time
- 互动数据
- metadata
```

内容类型特有字段存入一对一扩展表：

```text
Content 1 ── 0..1 ContentVideo
Content 1 ── 0..1 ContentAudio
Content 1 ── 0..1 ContentArticle
Content 1 ── 0..1 ContentNovel
Content 1 ── 0..1 ContentAlbum
Content 1 ── 0..1 ContentLive
Content 1 ── 0..1 ContentPodcast
Content 1 ── 0..1 ContentDocument
Content 1 ── 0..1 ContentCourse
Content 1 ── 0..1 ContentComic
Content 1 ── 0..1 ContentPost
Content 1 ── 0..1 ContentConversation
```

内容通用字段不会因为增加一种媒体形态而膨胀，类型扩展表也不负责记录具体的下载过程。

## 4. ContentAsset：逻辑归档资产

`ContentAsset` 表示一个稳定的、可下载或可生成的逻辑资产：

```text
ContentAsset
- id
- content_id
- kind
- role
- asset_key
- label
- language_code
- mime_type
- size
- sort_order
- metadata
```

逻辑唯一约束为：

```text
(content_id, role, asset_key)
```

### 4.1 Kind：资产的物理形式

```text
video
audio
image
text
document
archive
manifest
data
binary
```

### 4.2 Role：资产在内容中的语义

```text
primary
video_variant
audio_variant
subtitle
transcript
lyrics
cover
thumbnail
live_photo
article_body
novel_chapter
novel_book
comic_page
message_attachment
generated_image
generated_file
canvas
artifact
code_output
conversation_export
attachment
source_snapshot
```

`Kind` 和 `Role` 分离后，同一种物理文件可以承担不同语义。例如：

- `kind=image, role=cover` 表示封面。
- `kind=image, role=comic_page` 表示漫画页。
- `kind=text, role=subtitle` 表示字幕文件。
- `kind=text, role=novel_chapter` 表示小说章节文本。
- `kind=archive, role=conversation_export` 表示 AI 平台的原始导出包。
- `kind=image, role=generated_image` 表示 AI 助手生成的图片。

## 5. 视频规格与多语言字幕

### 5.1 多种视频规格

一个 `Content` 只对应一个 `ContentVideo`，不同分辨率或编码保存为多个 `ContentVideoVariant`：

```text
Content
└── ContentVideo
    ├── ContentVideoVariant 1080p
    ├── ContentVideoVariant 720p
    └── ContentVideoVariant 480p
```

每个 `ContentVideoVariant.AssetId` 对应一个 `ContentAsset`：

```text
kind      = video
role      = video_variant
asset_key = 平台格式 ID、itag、spec 或 default
```

抖音、视频号等没有明确分辨率信息的视频创建一个默认规格：

```text
asset_key = default
width     = NULL
height    = NULL
```

未知数据保持为空，不虚构分辨率。

### 5.2 多语言字幕

字幕分为逻辑轨道和具体文件：

```text
Content
└── ContentTextTrack
    ├── en / official
    │   ├── VTT source
    │   └── SRT source
    ├── zh-Hans / official
    └── ja / auto-generated
```

`ContentTextTrack` 保存：

- 语言代码和显示名称。
- 字幕、说明字幕或强制字幕类型。
- 是否默认。
- 是否自动生成。
- 是否为听障字幕。

`ContentTextTrackSource` 保存具体格式、URL、编码和过期时间。每个 Source 对应一个 `ContentAsset`，未知语言使用 `und`。

## 6. 小说章节和图集图片

### 6.1 小说章节

```text
ContentNovel
├── ContentNovelVolume
└── ContentNovelChapter
    └── ContentAssetLink
        └── ContentAsset
            └── DownloadResource
```

章节通过稳定的 `chapter_key` 标识，生成优先级为：

1. 平台章节 ID。
2. 章节 URL。
3. 章节序号，仅作为兼容回退。

同一章节可以有多种表示：

```text
chapter-1:txt
chapter-1:html
chapter-1:epub-fragment
```

因此可以通过以下链路从章节直接获得实际下载记录：

```text
ContentNovelChapter
  → Assets
  → Asset.DownloadResources
```

### 6.2 图集图片

```text
ContentAlbum
└── ContentImage
    └── ContentAssetLink
        ├── 原图 ContentAsset
        └── Live Photo 视频 ContentAsset
```

图片身份优先使用平台稳定标识：

- 抖音使用图片 URI。
- 视频号优先使用 decode key。
- 公众号图集使用内容内顺序。
- URL 仅作为兼容回退。

这样可以避免 CDN URL 的 token 更新后重复创建图片。

## 7. DownloadResource：实际下载记录

`DownloadResource` 表示一次具体下载过程或结果：

- 下载文件名和目录。
- 下载状态和已下载字节数。
- 下载速度和实际大小。
- Stream 录制信息。
- 所属下载任务。
- 可用下载端点。

它不再承担逻辑资产身份。逻辑资产与下载实例通过 `DownloadResourceAsset` 建立多对多关系：

```text
ContentAsset M ── N DownloadResource
```

```text
DownloadResourceAsset
- resource_id
- asset_id
- relation
```

支持的关系包括：

```text
source        Resource 下载得到该资产
contains      Resource 包含该资产
derived_from  Resource 由该资产派生
```

该结构支持：

- 同一个资产被多次下载或重新下载。
- 下载重试产生不同的 Resource。
- 一个 ZIP、合并文件或容器包含多个资产。
- 转码文件与原始资产之间建立关联。
- 不再依赖 `content_id + spec` 猜测业务关联。

`DownloadResource.UniqueID` 可以继续用于任务或文件去重，但不再作为资产外键。

## 8. ContentAssetLink：嵌套实体关联

`ContentAssetLink` 将资产绑定到章节、图片、漫画页、课时等内容内部实体：

```text
ContentAssetLink
- content_id
- subject_type
- subject_key
- asset_id
- relation
```

目前已经实际接入：

```text
novel_chapter
album_image
conversation_message
conversation_message_part
```

已经预留：

```text
novel_volume
live_photo
comic_chapter
comic_page
course_lesson
podcast_episode
```

其中 AI 对话可以把附件关联到整条消息，也可以精确关联到消息中的某个有序内容块。增加漫画页或课程课时时只需要：

1. 给子实体分配稳定的 `subject_key`。
2. 创建对应的 `ContentAsset`。
3. 创建 `ContentAssetLink`。
4. 将下载实例写入 `DownloadResourceAsset`。

不需要为每种内容重新设计下载关联表。

## 9. ContentRelation：内容之间的关系

`ContentRelation` 表达顶层内容之间的有向关系：

```text
SourceContent -- relation --> TargetContent
```

支持：

```text
contains
part_of
episode_of
reply_to
quote_of
repost_of
translation_of
derived_from
related
```

适用场景包括：

- 播放列表包含多个视频。
- 播客节目包含多期单集。
- 课程包含多个课时。
- Twitter/X Thread 的上下级帖子。
- 微博转发和引用。
- 文章的不同语言版本。
- 直播与直播回放。
- 原视频与剪辑版本。

## 10. AI 对话平台归档

ChatGPT、豆包、Claude、Gemini 等 AI 对话不是简单的线性文本。一个完整对话通常可能包含：

- 用户、助手、System、Developer 和 Tool 等不同角色。
- 编辑问题后产生的分支。
- 同一个问题重新生成的多个回答。
- 对话过程中切换模型。
- 图片、音频、视频和文件输入。
- 网页搜索引用、工具调用及工具结果。
- 生成图片、代码执行结果和可下载文件。
- Canvas、Artifact 或其他可继续编辑的独立产物。

因此不能只使用一张包含 `question` 和 `answer` 的线性表。

当前实现范围需要区分为两层：

- 已实现统一数据库模型、幂等保存、详情装配、分支还原，以及消息附件到 `DownloadResource` 的关联。
- 尚未实现 ChatGPT、豆包等平台原始 JSON、HTML 或 ZIP 的专用解析适配器；接入平台时只需把原始数据转换成下述统一模型。

### 10.1 与应用自身聊天表的边界

现有 `chat_session`、`chat_member`、`chat_message` 用于应用自身聊天或通用聊天记录导入。外部 AI 对话属于内容归档体系，使用独立表：

```text
Content
├── ContentAsset(role=conversation_export)  原始 JSON、HTML 或 ZIP
└── ContentConversation
    ├── ContentConversationBranch
    └── ContentConversationMessage
        ├── ContentAssetLink                 整条消息的附件
        └── ContentConversationMessagePart
            └── ContentAssetLink             具体内容块或生成产物
                └── ContentAsset
                    └── DownloadResourceAsset
                        └── DownloadResource
```

这种分离可以避免两类数据在以下方面发生冲突：

- 应用聊天使用自增 Session ID，归档内容使用 `platform_id:external_id`。
- 应用聊天通常是线性的，AI 对话可能是一棵消息树。
- AI 对话具有模型、工具、引用、Artifact 等特有信息。
- 归档内容需要统一接入 ContentAsset 和 DownloadResource。

### 10.2 ContentConversation

一个外部平台对话对应一个 `Content`：

```text
Content.Type    = conversation
Content.Subtype = ai_chat
Content.Id      = chatgpt:<conversation_id>
                  doubao:<conversation_id>
```

`ContentConversation` 保存会话级字段：

```text
- id
- source_type
- source_format
- format_version
- default_model_provider
- default_model_name
- current_branch_key
- message_count
- branch_count
- is_shared
- metadata
```

实际数据库表及稳定唯一键如下：

| 表 | 用途 | 稳定身份或唯一约束 |
| --- | --- | --- |
| `content` | 平台对话的内容主体 | `id = platform_id:external_id` |
| `content_conversation` | 对话级元数据 | `id`，同时外键关联 `content.id` |
| `content_conversation_branch` | 可选择的消息路径 | `(conversation_id, branch_key)` |
| `content_conversation_message` | 消息树节点 | `(conversation_id, message_key)` |
| `content_conversation_message_part` | 消息中的有序内容块 | `(conversation_id, message_key, part_key)` |
| `content_asset` | 附件、生成文件和原始导出 | `(content_id, role, asset_key)` |

重复导入同一对话时，保存逻辑按照这些稳定键更新已有 Branch、Message 和 Part，不依赖会变化的标题、文件名或临时 URL。

`source_type` 用来区分获取方式：

```text
official_export  平台官方数据导出
browser_capture  浏览器或客户端捕获
api              平台 API
share_page       公开分享页面
manual_import    手工导入
```

不同来源的数据完整度并不相同。例如分享页面可能只包含当前分支，官方导出可能包含整个消息树，浏览器捕获则可能包含更完整的附件地址但 URL 有时效性。

原始 JSON、HTML 或 ZIP 不应该只解析后丢弃，应同时作为：

```text
ContentAsset.Role = conversation_export
```

保存。解析后的表是当前查询投影，原始导出资产用于审计、重放和兼容未来格式变化。

### 10.3 消息树和对话分支

`ContentConversationMessage` 是对话树中的稳定节点：

```text
- conversation_id
- message_key
- parent_message_key
- role
- author_name
- model_provider
- model_name
- status
- content_text
- content_hash
- sequence
- sent_at
- edited_at
- metadata
```

消息角色包括：

```text
unknown
system
developer
user
assistant
tool
```

`message_key` 优先使用平台消息 ID，没有平台 ID 时才回退到导入顺序。`parent_message_key` 保留消息父子关系。

平台消息 ID 在统一模型中使用 `external:<id>` 形式；没有 ID 时才使用 `idx:<sequence>`。Branch 和 Part 使用相同原则生成稳定 Key。适配器必须保证同一份平台数据重复导入时生成相同 Key。

例如，重新生成两次回答后的结构可能是：

```text
user-1
└── assistant-1
    └── user-2
        ├── assistant-2a
        └── assistant-2b  ← 当前分支
```

不能将 `assistant-2a` 覆盖或删除。两条回答是不同消息节点，分别拥有自己的模型信息、内容块和资产。

`ContentConversationBranch` 记录一个可选择的消息路径：

```text
- branch_key
- root_message_key
- leaf_message_key
- is_current
- sort_order
```

共享祖先无需复制。通过 `leaf_message_key` 沿 `parent_message_key` 向上遍历即可还原一个完整分支。

查询空 `branch_key` 时按以下顺序选择分支：

1. `ContentConversation.CurrentBranchKey` 指向的有效分支。
2. `Branch.IsCurrent = 1` 的分支。
3. 第一条已保存分支。

还原时会检测父子循环、缺失叶子消息以及无法到达声明根消息的损坏数据。

### 10.4 消息内容块

一条消息可能同时包含文本、图片和工具结果，因此使用有序的 `ContentConversationMessagePart`：

```text
- conversation_id
- message_id
- message_key
- part_key
- subject_key
- sort_order
- type
- text
- url
- mime_type
- language_code
- tool_call_id
- tool_name
- metadata
```

支持的内容块类型包括：

```text
text
markdown
image
audio
video
file
code
tool_call
tool_result
citation
reasoning_summary
structured
link_preview
canvas
artifact
refusal
```

只保存平台实际展示或导出的思考摘要，不推断或构造模型未提供的隐藏推理过程。

`ContentText` 是便于搜索和列表展示的文本投影；准确还原消息时应按照 `Parts.SortOrder` 渲染各内容块。

`subject_key` 是 Part 的稳定资产关联键，由 `message_key` 和 `part_key` 组合生成。由于平台 ID 可能包含分隔符，实现使用带消息 Key 长度前缀的编码，不能由适配器自行拼接普通冒号字符串。

### 10.5 对话附件和生成产物

消息或内容块本身不存储下载状态。所有上传附件、生成图片和 Artifact 继续使用 ContentAsset：

```text
message_attachment
generated_image
generated_file
canvas
artifact
code_output
conversation_export
```

关联链路为：

```text
ContentConversationMessagePart
  → ContentAssetLink(subject_type=conversation_message_part)
  → ContentAsset
  → DownloadResourceAsset
  → DownloadResource
```

如果一个资产属于整条消息而不是某一个 Part，可以使用：

```text
subject_type = conversation_message
subject_key  = message_key
```

如果资产属于具体内容块，则使用：

```text
subject_type = conversation_message_part
subject_key  = part.subject_key
```

这可以处理：

- 用户上传一张图片，平台生成缩略图和解析结果。
- 助手生成一张图片，并存在原图和预览图。
- Code Interpreter 生成 CSV、PDF 或 ZIP。
- Canvas/Artifact 同时存在源码、渲染结果和导出文件。
- 同一个附件由于重试或 URL 刷新对应多个 DownloadResource。

一个逻辑资产允许对应多个下载记录，一个下载容器也允许包含多个逻辑资产。因此对话附件同样不应通过 `content_id + 文件名` 或临时 URL 反向猜测归属。

### 10.6 平台字段映射

平台适配器应先保留平台原始字段，再映射到统一模型：

| 平台概念 | 统一字段 |
| --- | --- |
| 平台 Conversation ID | `Content.ExternalId` |
| 归一化内容 ID | `Content.Id` 和 `ContentConversation.Id`，格式为 `platform_id:external_id` |
| 对话标题 | `Content.Title` |
| 分享或原始页面 | `Content.SourceURL` |
| 当前节点或当前会话路径 | `CurrentBranchKey`、`Branch.LeafMessageKey` |
| Message ID | `Message.MessageKey` |
| Parent Message ID | `Message.ParentMessageKey` |
| User、Assistant、Tool | `Message.Role` |
| 使用的模型 | `Message.ModelProvider`、`Message.ModelName` |
| 文本和多模态内容数组 | `Message.Parts` |
| 附件、生成图片和文件 | `ContentAsset` 和 `ContentAssetLink` |
| Citation 或搜索结果 | `Part.Type=citation`，URL 和标题存入 Part 或 Metadata |
| Tool Call 和 Result | `tool_call_id`、`tool_name`、`tool_call/tool_result` Part |
| 平台无法归一的字段 | 对应层级的 `Metadata` |

适配器不得将 Cookie、Authorization、临时访问 Token 或包含凭据的完整请求头写入 Metadata。`Metadata` 应优先保存无法归一但重建内容确实需要的业务字段；完整原始响应应保存为受保护的 `conversation_export` 资产。

### 10.7 查询 AI 对话

查询内容详情会返回所有分支、消息和内容块：

```text
ContentConversation.Branches
ContentConversation.Messages
ContentConversation.Messages[].Parts
ContentConversation.Messages[].Assets
ContentConversation.Messages[].Parts[].Assets
```

查询当前分支：

```go
messages, err := content_service.GetContentConversationBranch(content_id, "")
```

查询指定分支：

```go
messages, err := content_service.GetContentConversationBranch(content_id, branch_key)
```

返回结果按照根消息到叶子消息排序，并且每个消息内容块已经带有对应的 ContentAsset 和 DownloadResource。

直接查询某个消息内容块的下载资源：

```sql
SELECT resource.*
FROM content_conversation_message_part AS part
JOIN content_asset_link AS asset_link
  ON asset_link.content_id = part.conversation_id
 AND asset_link.subject_type = 'conversation_message_part'
 AND asset_link.subject_key = part.subject_key
JOIN download_resource_asset AS resource_asset
  ON resource_asset.asset_id = asset_link.asset_id
JOIN download_resource AS resource
  ON resource.id = resource_asset.resource_id
WHERE part.conversation_id = ?
  AND part.message_key = ?
  AND part.part_key = ?
  AND resource.deleted_at IS NULL;
```

### 10.8 AI 对话测试建议

ChatGPT 和豆包至少分别准备以下测试样本：

1. 纯文本单分支对话。
2. 编辑用户问题后产生两个分支的对话。
3. 同一个问题重新生成两次回答的对话。
4. 对话中途切换模型的对话。
5. 上传图片并继续追问的对话。
6. 上传 PDF、表格或压缩包的对话。
7. 包含联网搜索结果和引用来源的对话。
8. 包含工具调用及工具结果的对话。
9. 包含生成图片或生成文件的对话。
10. 包含 Canvas、Artifact 或类似可编辑产物的对话。
11. 仅能通过分享页面访问的对话。
12. 同一对话重复导入，且附件临时 URL 已经变化的对话。

验收时重点检查：

- 重复导入不会重复创建 Content、Message、Part 和 ContentAsset。
- 消息编辑和回答重新生成不会破坏旧分支。
- 当前分支可以从叶子消息完整还原。
- 模型名称记录在具体助手消息上，而不是只记录会话默认模型。
- 多模态内容按照原始顺序还原。
- 附件临时 URL 更新后仍然复用同一个逻辑资产。
- 原始导出文件和解析后的结构化记录能够同时保留。

AI 对话通常包含隐私和账号数据，平台适配器或导入入口应默认将 `Content.IsPrivate` 设为 `1`，并避免把对话正文、上传文件内容和凭据写入普通运行日志。

### 10.9 平台适配器接入流程

ChatGPT、豆包或其他 AI 平台适配器建议按下面的顺序输出数据：

1. 解析平台 Conversation ID，创建 `Content(Type=conversation, Subtype=ai_chat)`，默认标记为私有。
2. 保存获取方式、平台格式版本和默认模型等会话级信息。
3. 遍历原始消息图，为每个节点生成稳定 `message_key` 和 `parent_message_key`。
4. 按原始顺序把文本、多模态输入、工具调用、引用和产物转换成 Message Part。
5. 根据当前叶子节点和其他可选叶子构造 Branch，不丢弃重新生成的旧回答。
6. 将上传附件、生成文件和原始导出声明为 `ContentAssetReference`，并指定消息或 Part 的 `subject_type/subject_key`。
7. 交给现有下载任务保存流程：先保存 Content，再保存 Conversation 扩展，最后创建 DownloadResource 和资产关联。

适配器生成稳定键时应调用统一帮助方法：

```go
message_key := model.BuildContentConversationMessageKey(platform_message_id, sequence)
part_key := model.BuildContentConversationMessagePartKey(platform_part_id, sort_order)
subject_key := model.BuildContentConversationMessagePartSubjectKey(message_key, part_key)
asset_key := model.BuildContentConversationAssetKey(subject_key, representation_key)
```

推荐保留两类数据：

```text
原始层：平台导出的 JSON / HTML / ZIP / 附件
投影层：ContentConversation / Branch / Message / Part
```

原始层保证将来可以用新版解析器重新建模，投影层用于日常列表、全文搜索、分支展示和附件查询。平台格式发生变化时，优先新增或升级平台解析器，不修改统一表来迁就某个平台的一次性字段。

## 11. 查询方式

### 11.1 查询内容及其根资产

内容详情查询会预加载：

```text
Content.Assets
└── ContentAsset.DownloadResources
```

```go
var content model.Content

err := db.
	Preload("Assets").
	Preload("Assets.DownloadResources").
	Where("id = ?", content_id).
	First(&content).Error
```

### 11.2 根据视频规格查询下载记录

```go
var variant model.ContentVideoVariant

err := db.
	Preload("Asset.DownloadResources").
	Where("video_id = ? AND variant_key = ?", content_id, variant_key).
	First(&variant).Error
```

查询结果：

```go
variant.Asset.DownloadResources
```

### 11.3 根据字幕源查询下载记录

```go
var source model.ContentTextTrackSource

err := db.
	Preload("Asset.DownloadResources").
	Where("track_id = ? AND source_key = ?", track_id, "vtt").
	First(&source).Error
```

查询结果：

```go
source.Asset.DownloadResources
```

### 11.4 查询小说章节或图集图片的下载记录

通用 SQL：

```sql
SELECT resource.*
FROM content_asset_link AS asset_link
JOIN content_asset AS asset
  ON asset.id = asset_link.asset_id
JOIN download_resource_asset AS resource_asset
  ON resource_asset.asset_id = asset.id
JOIN download_resource AS resource
  ON resource.id = resource_asset.resource_id
WHERE asset_link.content_id = ?
  AND asset_link.subject_type = ?
  AND asset_link.subject_key = ?
  AND resource.deleted_at IS NULL;
```

查询小说章节时：

```text
subject_type = novel_chapter
subject_key  = chapter.chapter_key
```

查询图集图片时：

```text
subject_type = album_image
subject_key  = image.image_key
```

内容详情服务已经组装以下查询结果：

```go
chapter.Assets[n].Asset.DownloadResources
image.Assets[n].Asset.DownloadResources
```

### 11.5 查询内容关系

查询一个合集或系列包含的内容：

```sql
SELECT target.*
FROM content_relation AS relation
JOIN content AS target
  ON target.id = relation.target_content_id
WHERE relation.source_content_id = ?
  AND relation.type = 'contains'
ORDER BY relation.sort_order, target.id;
```

查询一条短内容引用、回复或转发的原内容：

```sql
SELECT target.*, relation.type
FROM content_relation AS relation
JOIN content AS target
  ON target.id = relation.target_content_id
WHERE relation.source_content_id = ?
  AND relation.type IN ('reply_to', 'quote_of', 'repost_of');
```

## 12. 当前实现边界

已经完整打通：

- 视频多分辨率。
- 没有分辨率信息的短视频。
- 多语言字幕及多种字幕格式。
- 小说卷和章节。
- 小说章节到下载资源的直接关联。
- 图集图片和 Live Photo。
- 图集图片到下载资源的直接关联。
- 内容根资产。
- 旧数据迁移、关联回填和回滚。
- AI 对话、消息父子树和分支。
- AI 消息内容块、附件和生成文件到下载资源的直接关联。
- 当前或指定对话分支的根到叶查询。

AI 对话当前能力边界：

| 能力 | 状态 |
| --- | --- |
| 统一 Conversation、Branch、Message、Part 表结构 | 已实现 |
| 重复导入时按稳定键更新 | 已实现 |
| 当前分支和指定分支还原 | 已实现，服务层方法为 `GetContentConversationBranch` |
| 消息或 Part 到 DownloadResource 的直接关联 | 已实现 |
| 原始导出包作为 `conversation_export` 保存 | 模型和资产角色已支持，由平台适配器提供资源 |
| ChatGPT 原始导出解析 | 待实现，需要脱敏 Fixture |
| 豆包原始数据解析 | 待实现，需要脱敏 Fixture |
| 面向前端的专用 AI 对话导入和分支 API | 待具体产品入口接入 |

AI 对话表已合并在 `000002_content_media_asset` 迁移中。ChatGPT、豆包等平台的原始格式解析仍需要各自的导入适配器；在没有脱敏样本时，不应猜测不稳定的私有接口字段。

漫画页、课程课时、播客单集的 `subject_type` 和通用关联机制已经预留，但对应的章节、页面、课时子表还需要在具体平台接入时继续完善。后续扩展可以复用现有 `ContentAsset`、`ContentAssetLink` 和 `DownloadResourceAsset`，不需要重新设计下载关联结构。

## 13. 相关代码

- `internal/database/model/content.go`：内容主体、类型扩展、视频、字幕、小说和图集模型。
- `internal/database/model/content_media.go`：内容类型、通用资产、嵌套实体关联和内容关系。
- `internal/database/model/content_conversation.go`：AI 对话、分支、消息树和有序内容块。
- `internal/database/model/download_task.go`：下载资源及资产关联。
- `internal/services/download_task.go`：内容扩展保存、资产创建和下载资源关联。
- `internal/services/content.go`：内容详情、章节/图片/AI 消息资产装配，以及 AI 对话分支查询。
- `internal/database/migrations/000002_content_media_asset.up.sql`：通用内容资产结构、旧数据回填、小说章节扩展及外部 AI 对话归档表迁移。

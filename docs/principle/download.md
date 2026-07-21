# 微信频道下载器原理说明

## 概述

微信频道下载器的核心机制是通过**本地 HTTPS 代理**拦截微信频道网页的请求，在响应中注入自定义 JavaScript 代码，从而在官方网页中嵌入下载功能 UI，并将下载请求转发到本地后端处理。

## 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户浏览器                              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              微信频道网页 (channels.weixin.qq.com)        │   │
│  │  ┌────────────────┐  ┌───────────────────────────────┐  │   │
│  │  │   原始网页代码  │  │   注入的自定义 JS (downloader)  │  │   │
│  │  └────────────────┘  └───────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ HTTPS 代理
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   wx_channels_download 服务                     │
│  ┌─────────────────┐    ┌─────────────────┐    ┌────────────┐  │
│  │  Interceptor    │───▶│   API Server    │───▶│  Gopeed   │  │
│  │  (代理拦截层)    │    │  (任务处理)      │    │ (下载引擎) │  │
│  └─────────────────┘    └─────────────────┘    └────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. 代理拦截层 (Interceptor)

**文件位置**: `internal/interceptor/`

#### 1.1 代理服务器启动

```go
// interceptor.go:46-115
func (c *Interceptor) Start() error {
    // 1. 创建本地 HTTPS 代理
    client, err := proxy.NewProxy(c.Cert.Cert, c.Cert.PrivateKey)
    
    // 2. 配置路由规则
    //   - localapi.weixin.qq.com -> 本地 API 服务器
    //   - remoteapi.weixin.qq.com -> 远程服务器 (可选)
    //   - debug.weixin.qq.com -> 调试服务器 (可选)
    
    // 3. 添加频道拦截插件
    plugins := CreateChannelInterceptorPlugins(c, Assets)
    for _, plugin := range plugins {
        client.AddPlugin(plugin)
    }
    
    // 4. 安装根证书 (用于 HTTPS 解密)
    if !c.Settings.ProxySkipInstallRootCert {
        certificate.InstallCertificate(c.Cert.Name, c.Cert.Cert)
    }
    
    // 5. 设置系统代理 (可选)
    if c.Settings.ProxySetSystem {
        system.EnableProxy(...)
    }
    
    // 6. 启动代理服务器
    return client.Start(c.Settings.ProxyServerPort)
}
```

#### 1.2 拦截插件机制

代理使用插件模式来处理不同的请求：

```go
// plugin.go:52-257
func CreateChannelInterceptorPlugins(interceptor *Interceptor, files *ChannelInjectedFiles) []*proxy.Plugin {
    // 插件1: 处理 channels.weixin.qq.com
    plugin1 := &proxy.Plugin{
        Match: "channels.weixin.qq.com",
        OnRequest: func(ctx proxy.Context) {
            // 处理特定 API 调用，如 __wx_channels_api/profile
        },
        OnResponse: func(ctx proxy.Context) {
            // 在 HTML 响应中注入自定义 JS
            // 1. 注入工具库 (Mitt, Timeless, EventBus 等)
            // 2. 注入下载器 UI 代码
            // 3. 注入页面特定代码 (home, feed, live, profile)
        },
    }
    
    // 插件2: 处理静态资源 res.wx.qq.com
    plugin2 := &proxy.Plugin{
        Match: "res.wx.qq.com",
        OnResponse: func(ctx proxy.Context) {
            // 修改 JS 模块，注入事件触发代码
            // 用于捕获页面数据加载事件
        },
    }
}
```

### 2. 前端注入 (Frontend Injection)

**文件位置**: `internal/interceptor/inject/src/`

#### 2.1 注入机制

拦截器在微信频道网页的 `<head>` 标签中注入以下脚本：

```javascript
// 1. 工具库
<script src="mitt.js"></script>          // 事件总线
<script src="eventbus.js"></script>       // 事件处理
<script src="utils.js"></script>          // 工具函数

// 2. UI 组件
<script src="components.js"></script>    // UI 组件

// 3. 下载器核心
<script src="downloader.js"></script>      // 下载面板 v1 (已废弃)
<script src="downloaderv2.js"></script>    // 下载面板 v2 (当前使用)
```

#### 2.2 下载面板 (downloaderv2.js)

```javascript
// downloaderv2.js:75-459
function DownloaderPanelViewModel() {
    // 1. 创建请求核心
    const taskListReq = new Timeless.kit.RequestCore(
        (params) => request.get("/api/task/list", params),
        ...
    );
    
    // 2. 定义操作方法
    const methods = {
        startTask(task)    // 开始下载
        pauseTask(task)    // 暂停下载
        resumeTask(task)   // 恢复下载
        deleteTask(task)   // 删除任务
        openTask(task)     // 打开文件
    };
    
    // 3. WebSocket 连接实时更新
    connect() {
        const ws = new WebSocket("ws://" + FakeAPIServerAddr + "/ws/downloader");
        ws.onmessage = (ev) => {
            // 接收任务状态更新
            // batch_tasks - 批量任务
            // event - 单任务更新
        };
    }
}
```

#### 2.3 页面事件捕获

通过修改 `res.wx.qq.com` 的 JavaScript 代码，注入事件触发：

```javascript
// plugin.go:290-445
// 修改 finderInit 函数，在初始化完成后触发事件
js_init := `async finderInit() {
    var result = await (async () => { $1; })();
    var data = result.data;
    WXU.emit(WXU.Events.Init, data);  // 触发初始化事件
    return result;
}`

// 修改 finderGetCommentDetail 函数，在查看视频详情时触发
js_feed_profile := `async finderGetCommentDetail($1) {
    var result = await (async () => { $2; })();
    var feed = result.data.object;
    WXU.emit(WXU.Events.FeedProfileLoaded, feed);  // 触发视频详情加载事件
    return result;
}`
```

### 3. 后端 API (API Server)

**文件位置**: `internal/api/`

#### 3.1 下载任务创建

```go
// handler_download_task.go
func (c *APIClient) handleCompatDownloadTaskCreate(ctx *gin.Context) {
    // 1. 解析请求体，获取视频信息
    var body channels.ChannelsObject
    ctx.ShouldBindJSON(&body)

    // 2. 直接使用 wxchannels 包辅助函数
    downloadURL := wxchannels.ObjectURL(&body)
    spec := wxchannels.PickSpec(&body)
    title := wxchannels.ObjectTitle(&body)

    // 3. 处理视频格式 (图片/视频)
    if body.Type == "picture" {
        // 图片转为 zip 下载
        downloadURL = "zip://weixin.qq.com?files=" + filesJSON
    }

    // 4. 添加质量参数
    downloadURL = downloadURL + "&X-snsvideoflag=" + spec

    // 5. 使用 Gopeed 创建下载任务
    taskId, err := c.downloader.CreateDirect(
        &base.Request{
            URL: downloadURL,
            Labels: map[string]string{
                "id":         feed.ObjectId,
                "title":      feed.Title,
                "key":        strconv.Itoa(key),
                "spec":       spec,
                "source_url": sourceURL,
            },
        },
        &base.Options{
            Name: filename + suffix,
            Path: filepath.Join(c.cfg.DownloadDir, dir),
        },
    )
}
```

#### 3.2 API 路由

```go
// routes.go - 路由定义
POST   /api/task/create           // 创建下载任务
POST   /api/task/batch_create     // 批量创建
GET    /api/task/list             // 获取任务列表
POST   /api/task/start            // 开始任务
POST   /api/task/pause            // 暂停任务
POST   /api/task/resume           // 恢复任务
POST   /api/task/delete           // 删除任务
POST   /api/task/retry            // 重试失败任务

WS     /ws/downloader             // WebSocket 实时更新
```

### 4. 下载引擎 (Gopeed)

使用 [Gopeed](https://github.com/GopeedLab/gopeed) 作为下载引擎：

- 支持 HTTP/HTTPS 下载
- 支持多线程下载
- 支持断点续传
- 支持速度限制
- 提供 WebSocket 状态推送

## 数据流

### 下载流程

```
用户点击下载按钮
       │
       ▼
┌──────────────────┐
│ downloaderv2.js │
│  收集视频信息     │
└────────┬─────────┘
         │ HTTP POST /api/task/create
         ▼
┌──────────────────┐
│ handler_download │
│ _task.go         │
│ 解析视频参数      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Gopeed           │
│ 创建下载任务      │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 下载文件到本地    │
│ (配置目录)       │
└────────┬─────────┘
         │ WebSocket 推送状态
         ▼
┌──────────────────┐
│ downloaderv2.js │
│ 更新 UI 显示     │
└──────────────────┘
```

### 拦截流程

```
用户浏览器
    │
    │ HTTPS 请求 channels.weixin.qq.com
    ▼
┌─────────────────────────────────────┐
│ Interceptor 代理服务器               │
│  1. 解密 HTTPS                      │
│  2. 匹配域名 channels.weixin.qq.com │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│ Plugin1: OnResponse                 │
│  1. 获取原始 HTML                   │
│  2. 注入自定义 JS 脚本              │
│  3. 替换 <head> 内容                │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│ 返回修改后的 HTML                   │
│ (用户浏览器渲染)                    │
└─────────────────────────────────────┘
```

## 关键技术点

### 1. HTTPS 中间人攻击

- 生成自签名 CA 证书
- 安装到系统信任存储
- 动态解密 HTTPS 流量

### 2. JavaScript 注入

- 正则替换 HTML 中的 `<head>` 标签
- 注入自定义 JS 脚本
- 修改原有 JS 模块，注入事件触发

### 3. API 转发

- 将 `localapi.weixin.qq.com` 转发到本地 API 服务器
- 实现前后端通信

### 4. 事件驱动

- 使用 Mitt 作为事件总线
- 监听页面数据加载事件
- 触发下载 UI 更新

## 数据结构与概念

### 核心数据类型

微信频道下载器涉及两种类型定义：
- **TypeScript 类型** (前端): `internal/interceptor/inject/src/utils.d.ts`
- **Go 类型** (后端): `internal/channels/type.go`

两者一一对应，前端用于浏览器中的数据处理，后端用于 API 请求/响应处理。

### 1. 内容类型 (Content Types)

| 类型 | 值 | 说明 | 示例 |
|------|-----|------|------|
| media | `"media"` | 视频内容 | 普通视频 |
| picture | `"picture"` | 图片内容 | 相册/图文 |
| live | `"live"` | 直播内容 | 直播中/直播回放 |

对应字段：`ChannelsObject.Type` / `FeedProfile.type`

### 2. 媒体类型 (Media Types)

| 值 | 说明 |
|-----|------|
| 4 | 视频 |
| 2 | 图片/图文 |
| 9 | 直播回放 |

对应字段：`ChannelsObjectDesc.MediaType`

### 3. 数据结构层次

```
┌─────────────────────────────────────────────────────────────────┐
│                    ChannelsObject (原始 API 响应)                │
├─────────────────────────────────────────────────────────────────┤
│ id                 string   │ 视频唯一标识                      │
│ type               string   │ 类型: media/picture/live          │
│ objectNonceId      string   │ 随机 ID                          │
│ sourceURL          string   │ 视频来源页面 URL                  │
│ createtime         int      │ 创建时间戳                        │
│ contact            ChannelsContact │ 发布者信息                  │
│ objectDesc         ChannelsObjectDesc │ 内容描述                 │
│   ├── description  string   │ 视频描述/标题                    │
│   ├── mediaType    int      │ 媒体类型 (4/2/9)                 │
│   └── media        ChannelsMedia[] │ 媒体列表                   │
│ files              ChannelsMedia[] │ 图片列表 (picture 类型)     │
│ liveInfo           ChannelsLiveInfo │ 直播信息 (live 类型)       │
│ anchorContact      ChannelsContact │ 主播信息 (live 类型)        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼ 转换 (wxchannels 包)
┌─────────────────────────────────────────────────────────────────┐
│                 model.Content (内容记录)                          │
├─────────────────────────────────────────────────────────────────┤
│ Id            string              │ 主键                         │
│ PlatformId    string              │ 平台: "wx_channels"          │
│ ExternalId    string              │ 外部 ID (视频 ID)            │
│ ExternalId2   string              │ NonceId                      │
│ ExternalId3   string              │ DecodeKey                    │
│ ContentType   string              │ video / picture / live       │
│ Title         string              │ 标题                         │
│ ContentURL    string              │ 下载 URL                     │
│ CoverURL      string              │ 封面 URL                     │
│ Duration      int64               │ 时长                         │
│ Size          int64               │ 文件大小                     │
│ PublishTime   *int64              │ 发布时间                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                     ┌────────┴────────┐
                     ▼                 ▼
            ┌───────────────┐  ┌───────────────┐
            │ model.Account │  │ DownloadTask  │
            │ (发布者)       │  │ (下载任务)    │
            └───────────────┘  └───────────────┘
                              │
                              ▼ 前端处理
┌─────────────────────────────────────────────────────────────────┐
│                    FeedProfile (前端展示用)                      │
├─────────────────────────────────────────────────────────────────┤
│ type        "media" | "picture" | "live" │ 类型                 │
│ id          string │ 视频 ID                                  │
│ nonce_id    string │ 随机 ID                                   │
│ title       string │ 标题                                      │
│ url         string │ 下载地址                                  │
│ key         number │ 解密密钥                                  │
│ cover_url   string │ 封面地址                                  │
│ createtime  number │ 发布时间                                 │
│ size        number │ 文件大小                                  │
│ duration    number │ 时长                                      │
│ files       { url: string }[] │ 图片列表 (picture 类型)        │
│ spec        ChannelsMediaSpec[] │ 规格列表 (media 类型)        │
│ contact     { id, avatar_url, nickname } │ 发布者               │
└─────────────────────────────────────────────────────────────────┘
```

### 4. 数据结构详细说明

#### 4.1 微信原始数据

**ChannelsObject** (Go) / **ChannelsFeed** (TS)
```go
// Go - internal/channels/type.go:166-178
type ChannelsObject struct {
    ID            string              `json:"id"`
    Contact       ChannelsContact     `json:"contact"`
    ObjectDesc    ChannelsObjectDesc  `json:"objectDesc"`
    ObjectNonceId string              `json:"objectNonceId"`
    SourceURL     string              `json:"source_url"`
    CreateTime    int                 `json:"createtime"`
    Type          string              `json:"type"`
    Spec          []ChannelsMediaSpec `json:"spec"`
    LiveInfo      *ChannelsLiveInfo   `json:"liveInfo,omitempty"`
    Files         []ChannelsMediaItem `json:"files"`
    AnchorContact *ChannelsContact    `json:"anchorContact,omitempty"`
}
```

```typescript
// TS - internal/interceptor/inject/src/utils.d.ts:40-80
type ChannelsFeed = {
  id: string;
  description?: string;
  objectDesc: {
    mediaType: number;  // 4视频 9直播
    description: string;
    media: ChannelsMedia[];
  };
  objectNonceId: string;
  // ...
};
```

**ChannelsMediaItem** (Go) / **ChannelsMedia** (TS)
```go
// Go - internal/channels/type.go:68-79
type ChannelsMediaItem struct {
    URL          string              `json:"url"`
    MediaType    int                 `json:"mediaType"`
    VideoPlayLen int                 `json:"videoPlayLen"`
    Width        int                 `json:"width"`
    Height       int                 `json:"height"`
    FileSize     int                 `json:"fileSize"`
    Spec         []ChannelsMediaSpec `json:"spec"`
    CoverUrl     string              `json:"coverUrl"`
    DecodeKey    string              `json:"decodeKey"`
    URLToken     string              `json:"urlToken"`
}
```

```typescript
// TS - internal/interceptor/inject/src/utils.d.ts:82-93
type ChannelsMedia = {
  url: string;
  urlToken: string;
  coverUrl: string;
  fileSize: number;
  decodeKey: number;
  videoPlayLen: number;
  width: number;
  height: number;
  spec: ChannelsMediaSpec[];
};
```

**ChannelsMediaSpec** - 视频规格
```go
// Go - internal/channels/type.go:32-48
type ChannelsMediaSpec struct {
    FileFormat string `json:"fileFormat"`  // original, hd, sd, ld 等
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    DurationMs int    `json:"durationMs"`
}
```

```typescript
// TS - internal/interceptor/inject/src/utils.d.ts:94-97
type ChannelsMediaSpec = {
  fileFormat: string;  // 规格值
};
```

#### 4.2 转换辅助函数

ChannelsObject 通过 `wxchannels` 包直接转换为 model 结构，无需中间类型：

```go
// internal/adapter/wxchannels/model.go

// 转换为 model.Content (内容记录)
func ToContent(obj *ChannelsObject) (*model.Content, error)

// 转换为 model.Account (发布者记录)
func ToAccount(obj *ChannelsObject) (*model.Account, error)

// 辅助取值函数
func ObjectTitle(obj *ChannelsObject) string  // 标题
func ObjectURL(obj *ChannelsObject) string    // 下载 URL
func PickSpec(obj *ChannelsObject) string      // 编码规格
func DecryptKeyInt(obj *ChannelsObject) int     // 解密密钥
func BuildJumpURLFromParts(objectId, nonceId, sourceURL, username string) string  // 页面跳转 URL

// 浏览记录构造
func BuildBrowseRecord(profile *MediaProfile) *model.BrowseHistory
```

**FeedProfile** (TS) - 前端展示用
```typescript
// internal/interceptor/inject/src/utils.d.ts:102-131
type FeedProfile = {
  type: "media" | "picture" | "live";
  id: string;
  nonce_id: string;
  title: string;
  url: string;
  key: number;
  cover_url: string;
  createtime: number;
  size?: number;
  duration?: number;
  files?: { url: string }[];
  spec?: ChannelsMediaSpec[];
  contact: {
    id: string;
    avatar_url: string;
    nickname: string;
  };
};
```

#### 4.3 配置类型

**ChannelsConfig** - 前端配置
```typescript
// internal/interceptor/inject/src/utils.d.ts:16-33
type ChannelsConfig = {
  defaultHighest: boolean;           // 下载按钮默认下载原始视频
  downloadFilenameTemplate: string;   // 下载文件名模板
  downloadPauseWhenDownload: boolean;// 下载时暂停播放
  downloadInFrontend: boolean;       // 在前端下载
  apiServerAddr: string;             // API 服务地址
  remoteServerEnabled: string;
  remoteServerProtocol: string;
  remoteServerHostname: string;
  remoteServerPort: number;
  MaxRunning: number;                // 最大并发下载数
  downloadForceCheckAllFeeds: boolean;
};
```

### 5. API 响应类型

**MediaProfileResp** - 视频详情响应
```go
// internal/channels/type.go:232-257
type MediaProfileResp struct {
    BaseResponse   BaseResponse    `json:"BaseResponse"`
    Object         ChannelsObject  `json:"object"`
    CommentCount   int             `json:"commentCount"`
    // ...
}
```

**ChannelsFeedListOfAccountResp** - 账号视频列表响应
```go
// internal/channels/type.go:191-230
type ChannelsFeedListOfAccountResp struct {
    BaseResponse   BaseResponse     `json:"BaseResponse"`
    Object         []ChannelsObject `json:"object"`
    Contact        ChannelsContact  `json:"contact"`
    FeedsCount     int              `json:"feedsCount"`
    ContinueFlag   int              `json:"continueFlag"`
    LastBuffer     string           `json:"lastBuffer"`
    // ...
}
```

## 文件结构

```
internal/
├── interceptor/           # 代理拦截层
│   ├── interceptor.go     # 主入口
│   ├── server.go          # 服务器管理
│   ├── plugin.go          # 拦截插件
│   ├── types.go           # 类型定义
│   ├── config.go          # 配置
│   ├── assets.go          # 注入的资源
│   ├── inject/src/        # 前端注入代码
│   │   ├── utils.d.ts     # TypeScript 类型定义
│   │   ├── downloader.js
│   │   ├── downloaderv2.js
│   │   ├── eventbus.js
│   │   └── ...
│   └── proxy/             # 代理实现
│       ├── echo.go        # Echo 代理
│       └── ...
├── channels/              # 频道数据处理
│   ├── type.go            # Go 类型定义
│   ├── client.go          # API 客户端
│   └── ...
├── api/                   # API 服务器
│   ├── handler_download_task.go  # 下载任务处理
│   ├── routes.go          # 路由定义
│   └── client.go          # API 客户端
└── downloader/            # 下载客户端
    ├── client.go          # WebSocket 客户端
    └── handler.go         # (已注释)
```

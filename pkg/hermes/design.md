多协议下载器设计文档（V1）

1. 设计目标

构建一个统一下载框架，支持：

* 多协议下载
* 多资源下载
* 文件夹下载
* 直播流录制
* 断点续传
* 多线程下载
* 插件化协议扩展

设计原则：

* 协议无关（Protocol Agnostic）
* 资源统一（Resource Oriented）
* 任务统一（Task Driven）
* 插件扩展（Plugin Architecture）

⸻

2. 核心模型

整个系统只抽象三种资源。

Resource Type	描述	是否有限
File	单个文件	✅
Collection	文件集合（目录、多文件）	✅
Stream	数据流（直播）	❌

例如：

HTTP 下载
        ↓
      File
FTP 文件夹
        ↓
   Collection
BT
        ↓
   Collection
HLS Live
        ↓
     Stream
RTMP
        ↓
     Stream

协议不是资源类型。

⸻

3. 下载任务

Task 表示用户的一次下载行为。

例如：

下载 Ubuntu ISO
Task

例如：

录制 Twitch
Task

Task 不关心协议。

Task 只关心：

* 下载什么
* 保存哪里
* 当前状态

⸻

4. 资源(Resource)

一个 Task 可以拥有多个 Resource。

例如：

Task
    │
    ├── Video
    ├── Audio
    ├── Subtitle
    └── Cover

对于普通下载：

Task
    │
    └── File

对于 BT：

Task
    │
    ├── file1.iso
    ├── file2.pdf
    └── file3.zip

⸻

5. Endpoint（下载源）

每个 Resource 可以拥有多个下载源。

例如：

Video
HTTP
FTP
S3

或者：

Ubuntu.iso
HTTP Mirror
FTP Mirror
BT
S3

Endpoint 用于：

* 镜像
* 容灾
* 自动测速
* 自动切换

而不是多个协议同时下载。

⸻

6. 协议（Protocol）

协议负责：

* 如何连接
* 如何认证
* 如何读取数据
* 如何获取资源信息

例如：

HTTP
HTTPS
FTP
SFTP
WebDAV
S3
BitTorrent
Magnet
HLS
DASH
RTMP
RTSP
SRT
WebRTC

协议全部实现统一接口。

Prepare()
Start()
Pause()
Resume()
Cancel()
Close()
Progress()

因此新增协议无需修改业务代码。

⸻

7. 下载流程

Task
↓
Scheduler
↓
Resource
↓
Endpoint
↓
Protocol Driver
↓
Reader
↓
Writer
↓
Disk

协议负责读取。

Writer 负责写文件。

Scheduler 负责调度。

⸻

8. 数据库设计

download_task

下载任务。

id
name
resource_type
status
save_path
create_time
start_time
finish_time
config_json

resource_type

FILE
COLLECTION
STREAM

⸻

download_resource

任务中的资源。

id
task_id
name
kind
size
status
merge_order

kind

file
video
audio
subtitle
cover

⸻

download_endpoint

资源下载源。

id
resource_id
protocol
url
priority
enabled
headers
cookies
status

例如：

Ubuntu.iso
HTTP
FTP
S3
BT

⸻

download_segment

统一分片。

id
resource_id
index
url
offset_start
offset_end
size
downloaded
status
retry

统一表示：

HTTP Range

HLS TS

DASH Chunk

BT Piece

全部统一。

⸻

download_connection

连接状态。

id
endpoint_id
worker_id
host
ip
speed
bytes
status
last_active

用于：

* 多线程
* 多连接
* CDN

⸻

download_live

直播信息。

id
task_id
stream_url
record_start
record_end
duration
rotate_minutes
rotate_size
is_live

支持：

* 自动切片
* 自动续录
* 自动重连

⸻

download_log

日志。

id
task_id
level
message
create_time

⸻

9. 状态机

WAITING
↓
PREPARING
↓
DOWNLOADING
↓
PAUSED
↓
MERGING
↓
FINISHED

异常：

FAILED
CANCELLED

直播没有 MERGING 也可以直接 FINISHED。

⸻

10. 插件架构

所有协议实现统一接口。

ProtocolDriver
Prepare()
Start()
Pause()
Resume()
Cancel()
Close()
Progress()

例如：

HttpDriver
FtpDriver
S3Driver
TorrentDriver
HlsDriver
DashDriver
RtmpDriver
RtspDriver

调度器完全不知道协议细节。

⸻

11. 整体架构

                  Download Task
                        │
        ┌───────────────┴───────────────┐
        │                               │
     Resource 1                     Resource N
        │                               │
   ┌────┴────┐                    ┌─────┴─────┐
   │         │                    │           │
Endpoint1 Endpoint2          Endpoint1 Endpoint2
   │         │                    │           │
 HTTP      FTP                 HLS         RTMP
   │         │                    │           │
   └─────────┴────────────┬───────┴───────────┘
                           │
                    Scheduler（调度器）
                           │
                     Reader / Writer
                           │
                         Storage

12. 设计特点

* 资源与协议解耦：协议仅负责获取数据，任务围绕资源组织，新增协议无需修改任务模型。
* 统一资源抽象：无论是单文件、文件集合还是直播流，都通过统一的 Task → Resource → Endpoint 模型管理。
* 插件化扩展：新增协议只需实现统一驱动接口，调度器和数据库无需调整。
* 支持镜像与容灾：同一 Resource 可配置多个 Endpoint，实现自动切换、优先级和镜像源管理。
* 统一分片模型：HTTP Range、HLS TS、DASH Chunk、BT Piece 等统一抽象为 Segment，便于断点续传、重试和并发下载。
* 易于扩展：后续可增加限速、任务队列、计划下载、云存储上传、媒体转码等能力，而无需重构核心模型。

这套设计适合作为一个长期演进的下载引擎，能够覆盖浏览器下载、桌面下载器以及服务端下载服务等多种场景。

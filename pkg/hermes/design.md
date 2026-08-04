Multi-Protocol Downloader Design (V1)

1. Design Goals

Build a unified download framework that supports:

* Multi-protocol downloads
* Multi-resource downloads
* Directory downloads
* Live-stream recording
* Resumable downloads
* Multi-threaded downloads
* Pluggable protocol extensions

Design principles:

* Protocol agnostic
* Resource oriented
* Task driven
* Plugin architecture

---

2. Core Model

The system distinguishes two types at the Resource level.

| Resource Type | Description | Finite |
| --- | --- | --- |
| File | A single file | Yes |
| Stream | A data stream (live) | No |

Examples:

```text
HTTP download
      ↓
    File

HLS Live
      ↓
   Stream

RTMP
      ↓
   Stream
```

A Task is a pure container and may contain both File and Stream resources.
A protocol is not a resource type.

---

3. Download Tasks

A Task represents one download operation requested by the user.

Examples:

```text
Download Ubuntu ISO
Task

Record Twitch
Task
```

A Task is independent of both protocols and resource types.

A Task is concerned only with:

* Which resources it contains
* The root output directory
* Its current status

---

4. Resources

A Task may own multiple Resources.

Example:

```text
Task
    │
    ├── Video
    ├── Audio
    ├── Subtitle
    └── Cover
```

For a regular download:

```text
Task
    │
    └── File
```

For BitTorrent:

```text
Task
    │
    ├── file1.iso
    ├── file2.pdf
    └── file3.zip
```

---

5. Endpoints (Download Sources)

Each Resource may have multiple download sources.

Examples:

```text
Video
HTTP
FTP
S3
```

Or:

```text
Ubuntu.iso
HTTP Mirror
FTP Mirror
BT
S3
```

Endpoints provide:

* Mirrors
* Failover
* Automatic speed testing
* Automatic switching

They do not download through multiple protocols simultaneously.

---

6. Protocols

A protocol is responsible for:

* Establishing connections
* Authentication
* Reading data
* Obtaining resource information

Examples:

```text
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
```

Every protocol implements the same interface:

```text
Prepare()
Start()
Pause()
Resume()
Cancel()
Close()
Progress()
```

Adding a protocol therefore requires no changes to business logic.

---

7. Download Flow

```text
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
```

The protocol reads data.

The Writer writes files.

The Scheduler coordinates execution.

---

8. Database Design

### `download_task`

A download task (pure container).

```text
id
name
status
save_path
create_time
start_time
finish_time
config_json
```

Note: `resource_type` and the live-stream fields (`stream_url`, `record_start`, `record_end`, `duration`,
`rotate_minutes`, and `rotate_size`) have moved to `download_resource`.
A Task's `save_path` is always the output root directory, never a complete file path.

### `download_resource`

A resource within a task.

```text
id
task_id
name
kind
resource_type
size
downloaded
speed
status
merge_order
stream_url
record_start
record_end
duration
rotate_minutes
rotate_size
start_time
finish_time
```

`resource_type` values:

```text
file
stream
```

`kind` values:

```text
file
video
audio
subtitle
cover
```

### `download_endpoint`

A resource's download source.

```text
id
resource_id
protocol
url
priority
enabled
headers
cookies
status
```

Example:

```text
Ubuntu.iso
HTTP
FTP
S3
BT
```

### `download_segment`

A unified segment.

```text
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
```

It provides one representation for:

```text
HTTP Range
HLS TS
DASH Chunk
BT Piece
```

All segment types use the same model.

### `download_connection`

Connection state.

```text
id
endpoint_id
worker_id
host
ip
speed
bytes
status
last_active
```

Used for:

* Multiple threads
* Multiple connections
* CDN access

### `download_log`

Logs.

```text
id
task_id
level
message
create_time
```

---

9. State Machine

```text
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
```

Exceptional terminal states:

```text
FAILED
CANCELLED
```

A live stream may proceed directly to `FINISHED` without entering `MERGING`.

---

10. Plugin Architecture

Every protocol implements the same interface.

```text
ProtocolDriver
Prepare()
Start()
Pause()
Resume()
Cancel()
Close()
Progress()
```

Examples:

```text
HttpDriver
FtpDriver
S3Driver
TorrentDriver
HlsDriver
DashDriver
RtmpDriver
RtspDriver
```

The Scheduler has no knowledge of protocol-specific details.

---

11. Overall Architecture

```text
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
                     Scheduler
                         │
                   Reader / Writer
                         │
                       Storage
```

12. Design Characteristics

* Resource/protocol separation: protocols only retrieve data, while tasks are organized around resources. Adding a protocol does not require changing the task model.
* Unified resource abstraction: single files, file collections, and live streams all use the same Task → Resource → Endpoint model.
* Pluggable extensions: adding a protocol requires only an implementation of the common driver interface; the Scheduler and database remain unchanged.
* Mirror and failover support: one Resource may configure multiple Endpoints for automatic switching, priorities, and mirror management.
* Unified segment model: HTTP Range, HLS TS, DASH Chunk, and BT Piece data are all represented as Segments, simplifying resume, retry, and concurrent download behavior.
* Extensibility: rate limiting, task queues, scheduled downloads, cloud-storage uploads, and media transcoding can be added later without restructuring the core model.

This design is suitable for a download engine that evolves over the long term and covers browser downloads, desktop downloaders, and server-side download services.

# Hermes 下载引擎

`hermes` 是协议可插拔的下载任务引擎。名字取自希腊神话中跨越边界传递信息与物品的信使 Hermes，对应本包从不同协议端点获取并搬运资源的职责。

包边界：

- `Engine` 负责任务并发、状态流转、端点容灾、分片重试与断点续传。
- `ProtocolDriver` 负责协议连接、认证、资源探测和数据读取。
- `Store` 负责把任务、资源、分片和进度持久化到外部存储。
- HTTP/HTTPS 是默认驱动；其他协议通过 `Engine.RegisterProtocol` 注册。
- 本包不依赖 API、GORM、前端或具体内容平台。

当前执行器面向有限 `FILE` 资源。`COLLECTION` 和 `STREAM` 已保留资源类型，但需要后续增加相应规划与录制调度器。

## 当前可直接验证的协议

`Engine.New` 会默认注册 HTTP 驱动，因此当前无需额外注册即可下载 `http://` 和
`https://` 资源。其他协议是通过 `Engine.RegisterProtocol` 扩展的接口能力，仓库中
尚未提供 FTP、SFTP、BT 等协议驱动；请不要把它们当成当前可用功能。

## 手工下载验收数据

可直接使用 [testdata/manual-downloads.json](testdata/manual-downloads.json) 中的请求体，
在“下载管理”页面创建普通下载任务，或调用
`POST /api/v1/download_task/create_by_url`。所有地址都是公开的无鉴权静态文件；已于
2026-07-21 验证可访问，且支持 HTTP Range 请求。

| 样本 | 请求体中的 `url` | 预期结果 |
| --- | --- | --- |
| HTTP、10 MiB ZIP | `http://download.thinkbroadband.com/10MB.zip` | 成功下载；验证 HTTP 入口与文件名覆盖。 |
| HTTPS、1 MiB DAT | `https://proof.ovh.net/files/1Mb.dat` | 成功下载；资源大小恰好为 1,048,576 字节，应只创建一个分片。 |
| HTTPS、10 MiB DAT | `https://proof.ovh.net/files/10Mb.dat` | 成功下载；应创建 10 个 1 MiB 分片，用于验证 Range 并发下载与合并。 |
| HTTPS、100 MiB DAT | `https://proof.ovh.net/files/100Mb.dat` | 成功下载；分片数会被上限限制为 10，适合在下载过程中验证暂停、恢复与进度持久化。 |

### HLS 与 BitTorrent 样本

下列样本同样已记录在 `manual-downloads.json`。它们用于明确区分“下载协议入口”与
“把协议内容真正解析并下载为媒体/文件”的能力：

| 样本 | 地址 | 当前 Hermes 预期 |
| --- | --- | --- |
| HLS 主播放列表 | `https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8` | 会作为一个 752 字节的普通 HTTP 文件保存；**不会**选择码率、下载 TS/fMP4 分片或合并媒体。 |
| HLS 媒体播放列表 | `https://test-streams.mux.dev/x36xhzz/url_0/193039199_mp4_h264_aac_hd_7.m3u8` | 会保存 3,606 字节的含 TS 分片清单；**不会**下载其中的 TS 分片。 |
| MPEG-DASH MPD | `https://storage.googleapis.com/shaka-demo-assets/angel-one/dash.mpd` | 会保存 11,431 字节的 XML 清单；**不会**解析 Representation 或下载音视频分片。 |
| Sintel `.torrent` 元数据 | `https://webtorrent.io/torrents/sintel.torrent` | 会作为一个约 20 KiB 的普通 HTTP 文件保存；**不会**连接 tracker/DHT 或下载种子中的约 123 MiB 载荷。 |
| Sintel magnet URI | 见 `manual-downloads.json` | 当前 `/create_by_url` 会在参数校验阶段拒绝：`magnet:` URI 没有 HTTP 主机，且没有 BitTorrent 驱动。应确认没有创建任务。 |

因此，m3u8 和 `.torrent` 任务“已完成”只表示清单/元数据文件已保存，不能作为 HLS 或
BitTorrent 下载已实现的验收结论。待新增对应 `ProtocolDriver` 后，可复用同一批样本进行
端到端验收；磁链样本使用 WebTorrent 官方文档提供的自由电影 Sintel。

例如，可直接提交 10 MiB 分片样本（将 `API_BASE` 换成实际 API 地址）：

```sh
curl -X POST "$API_BASE/api/v1/download_task/create_by_url" \
  -H 'Content-Type: application/json' \
  --data '{
    "url": "https://proof.ovh.net/files/10Mb.dat",
    "filename": "hermes-https-10MiB.dat"
  }'
```

验收时确认任务最终为“已完成”、文件大小与表中一致；10 MiB/100 MiB 样本还应确认
下载记录中有对应的 Range 分片。外部测试站点可能在未来调整内容或限流，因此这些地址
仅用于手工验收，单元测试仍应使用 `httptest`，不能依赖它们。

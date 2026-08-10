# Hermes Download Engine

`hermes` is a download task engine with pluggable protocols. Its name comes from Hermes, the messenger in Greek mythology who carried information and goods across boundaries, reflecting this package's role in retrieving and moving resources from endpoints that use different protocols.

Package boundaries:

- `Engine` manages task concurrency, state transitions, endpoint failover, segment retries, and resumable downloads.
- `ProtocolDriver` manages protocol connections, authentication, resource probing, and data reads.
- `Store` persists tasks, resources, segments, and progress in external storage.
- `New(HermesNewConfig{})` supplies an in-memory store and a standard HTTP/HTTPS driver for direct Go use.
- Applications can still provide a persistent `Store` and replace protocol drivers through `Engine.RegisterProtocol`.
- This package does not depend on the API, GORM, the frontend, or any specific content platform.

## High-level Go API

The zero-config API creates and starts a single-resource HTTP/HTTPS task. The
returned handle can wait for completion and report the output path. `OnEvent`
replays replayable events that occurred before the callback was registered, so
it is safe to register immediately after `CreateTask`. With no base path set,
downloads are written to the process working directory.

```go
downloader := hermes.New(hermes.HermesNewConfig{})
task := downloader.CreateTask(raw_url)

downloader.OnEvent(func(event hermes.EventType, data hermes.EventData) {
    switch event {
    case hermes.EventProgress:
        progress_data := data.(hermes.TaskProgressEventData)
        fmt.Printf("task %d: %d bytes downloaded\n", progress_data.TaskID, progress_data.Progress.Downloaded)
    case hermes.EventFinished:
        finished_data := data.(hermes.TaskFinishedEventData)
        fmt.Printf("task %d finished: %v\n", finished_data.TaskID, finished_data.FilePaths)
    }
})

if err := task.Wait(); err != nil {
    return err
}
fmt.Println(task.FilePath())
```

Optional task settings do not require a custom `Store` or protocol driver:

```go
task := downloader.CreateTask(
    rawURL,
    hermes.WithFilename("release.zip"),
    hermes.WithDownloadDir("downloads"),
    hermes.WithProxyServer(hermes.ProxyServer{
        Address:  "socks5://127.0.0.1:1080",
        Username: "download-user",
        Password: "secret",
    }),
)
```

The lower-level `StartTask`, `Store`, and `RegisterProtocol` APIs remain
available for database-backed, multi-resource, inline-content, and live-stream
workflows.

## Task-level proxy

Set `TaskJob.ProxyServer` to route every network endpoint in that task through
the same proxy. `ProxyServer` keeps `Address`, `Username`, and `Password`
separate; `Address` accepts a complete proxy URL or an HTTP proxy `host:port`.
Stores that use the generic task configuration can instead provide the same
fields under `Config["proxy_server"]`. An explicit `TaskJob.ProxyServer` takes
precedence. The proxy is copied to `Endpoint.ProxyServer` before resource probing
and transfer, so a shared engine can run direct tasks and tasks using different
proxies at the same time.

```go
task.ProxyServer = hermes.ProxyServer{
    Address:  "socks5://127.0.0.1:1080",
    Username: "download-user",
    Password: "secret",
}
```

Proxy configuration is redacted from Hermes structured logs.

The bundled HTTP driver supports `http`, `https`, and `socks5` proxy URLs for
both full and byte-range requests. The live-stream driver passes the proxy to
FFmpeg's `http_proxy` input option for HTTP/HTTPS stream URLs.

The executor supports finite `FILE` resources and live `STREAM` resources. `COLLECTION` remains a reserved resource type that requires a dedicated planner.

## Protocols That Can Currently Be Verified

`New` registers a standard HTTP/HTTPS driver by default. An application may replace it by registering another driver for the same protocol names. Inline-content and `livestream` drivers remain explicit registrations. The live driver requires `ffmpeg` and records HTTP-FLV, HLS, RTMP, RTSP, and other inputs supported by the installed FFmpeg build. FTP, SFTP, BitTorrent, and similar drivers are not provided.

## Live-stream Recording

`STREAM` resources are routed through the optional `StreamRecorder` driver capability rather than the finite byte-range download loop. The bundled `livestream` driver:

- forwards endpoint HTTP headers and cookies to FFmpeg;
- enables HTTP network reconnect options;
- records without re-encoding (`-c copy`);
- writes ten-minute MKV chunks by default, or uses `download_resource.rotate_minutes`;
- retains closed chunks across pause and retry;
- reports aggregate byte/time progress and persists chunk state in `download_segment`;
- concatenates playable chunks into one MKV file and atomically commits it when the stream ends or reaches its configured stop time.

`record_start`, `record_end`, and `duration` are honored by the recording scheduler. `rotate_size` is carried through the recorder contract but the bundled FFmpeg recorder does not yet implement size-based rotation. HTTP 401, 403, and 410 responses are treated as fatal for that endpoint and persisted as concrete task errors because they usually mean that the signed URL or its authorization is no longer valid. Hermes does not refresh signed live URLs; other transport failures still use bounded retries.

Submitting an `.m3u8` through the generic HTTP URL endpoint still downloads the manifest as a finite file. To record it as a live stream, create a `STREAM` resource whose endpoint protocol is `livestream`.

## Manual Download Acceptance Data

The request bodies in [testdata/manual-downloads.json](testdata/manual-downloads.json) can be used directly to create a regular download task from the Download Management page or by calling `POST /api/v1/download_task/create_by_url`. All URLs point to public static files that require no authentication. They were verified as accessible and supporting HTTP Range requests on 2026-07-21.

| Sample | `url` in request body | Expected result |
| --- | --- | --- |
| HTTP, 10 MiB ZIP | `http://download.thinkbroadband.com/10MB.zip` | Downloads successfully; verifies the HTTP entry point and filename override. |
| HTTPS, 1 MiB DAT | `https://proof.ovh.net/files/1Mb.dat` | Downloads successfully; the resource is exactly 1,048,576 bytes and should create only one segment. |
| HTTPS, 10 MiB DAT | `https://proof.ovh.net/files/10Mb.dat` | Downloads successfully; should create ten 1 MiB segments to verify concurrent Range downloads and merging. |
| HTTPS, 100 MiB DAT | `https://proof.ovh.net/files/100Mb.dat` | Downloads successfully; the segment count is capped at ten, making this useful for verifying pause, resume, and progress persistence during a download. |

### HLS and BitTorrent Samples

The following samples are also recorded in `manual-downloads.json`. They clarify the distinction between accepting a download protocol URL and actually parsing and downloading its contents as media or files:

| Sample | URL | Current Hermes behavior |
| --- | --- | --- |
| HLS master playlist | `https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8` | Saves a regular 752-byte HTTP file; it does **not** select a bitrate, download TS/fMP4 segments, or merge media. |
| HLS media playlist | `https://test-streams.mux.dev/x36xhzz/url_0/193039199_mp4_h264_aac_hd_7.m3u8` | Saves a 3,606-byte manifest containing TS segments; it does **not** download those TS segments. |
| MPEG-DASH MPD | `https://storage.googleapis.com/shaka-demo-assets/angel-one/dash.mpd` | Saves an 11,431-byte XML manifest; it does **not** parse Representations or download audio/video segments. |
| Sintel `.torrent` metadata | `https://webtorrent.io/torrents/sintel.torrent` | Saves a regular HTTP file of about 20 KiB; it does **not** connect to a tracker/DHT or download the torrent's roughly 123 MiB payload. |
| Sintel magnet URI | See `manual-downloads.json` | The current `/create_by_url` endpoint rejects it during parameter validation: a `magnet:` URI has no HTTP host, and no BitTorrent driver is registered. Verify that no task is created. |

Consequently, a completed m3u8 or `.torrent` task means only that the manifest or metadata file was saved; it does not demonstrate that HLS or BitTorrent downloading is implemented. Once the corresponding `ProtocolDriver` is added, the same samples can be reused for end-to-end acceptance testing. The magnet sample uses the freely available Sintel movie referenced in WebTorrent's official documentation.

For example, submit the 10 MiB segmented sample directly after replacing `API_BASE` with the actual API address:

```sh
curl -X POST "$API_BASE/api/v1/download_task/create_by_url" \
  -H 'Content-Type: application/json' \
  --data '{
    "url": "https://proof.ovh.net/files/10Mb.dat",
    "filename": "hermes-https-10MiB.dat"
  }'
```

During acceptance testing, verify that the final task state is "completed" and that the file size matches the table. For the 10 MiB and 100 MiB samples, also verify that the download records contain the corresponding Range segments. External test sites may change their content or apply rate limits in the future, so these URLs are for manual acceptance only; unit tests must continue to use `httptest` and must not depend on them.

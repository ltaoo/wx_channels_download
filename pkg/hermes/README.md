# Hermes Download Engine

`hermes` is a download task engine with pluggable protocols. Its name comes from Hermes, the messenger in Greek mythology who carried information and goods across boundaries, reflecting this package's role in retrieving and moving resources from endpoints that use different protocols.

Package boundaries:

- `Engine` manages task concurrency, state transitions, endpoint failover, segment retries, and resumable downloads.
- `ProtocolDriver` manages protocol connections, authentication, resource probing, and data reads.
- `Store` persists tasks, resources, segments, and progress in external storage.
- HTTP/HTTPS is the default driver; other protocols are registered through `Engine.RegisterProtocol`.
- This package does not depend on the API, GORM, the frontend, or any specific content platform.

The current executor targets finite `FILE` resources. `COLLECTION` and `STREAM` are reserved resource types, but they still require dedicated planning and recording schedulers.

## Protocols That Can Currently Be Verified

`Engine.New` registers the HTTP driver by default, so `http://` and `https://` resources can be downloaded without additional registration. Other protocols can be added through the `Engine.RegisterProtocol` interface, but the repository does not yet provide FTP, SFTP, BT, or similar drivers; do not treat them as currently available features.

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

# Content Platform Download Pipeline

This package contains content-source platform integrations, not OS/platform
helpers. Keep OS/platform code in `pkg/platform`.

## Core Concepts

- **Content platform**: A source site or app that can produce downloadable
  content, for example WeChat Channels, Douyin, Zhihu, Official Account, or
  YouTube.
- **Handler**: The platform adapter. Every platform implements
  `download.Handler` with `Match`, `Probe`, `Resolve`, and `Plan`.
- **Probe**: The pre-request node. It receives a URL, identifies and fetches
  enough platform data to build a pass-through generic `Content[T]`, available
  variants, and internal reuse state.
- **Interaction**: A user confirmation node. It receives the probe output and a
  form definition, then pauses the workflow until the frontend resumes it with
  user-selected options.
- **ResolvedRequest**: The concrete download request concept. It combines the
  selected variant/options with the platform source data and produces a
  `DownloadSpec`.
- **PipelinePlan**: A platform-declared list of follow-up nodes such as
  download, decrypt, parse HTML, download assets, rewrite HTML, transcode, and
  persist.

## Start/Resume Flow

The API entrypoint is in `internal/api/handler_platform_task.go`.

1. `POST /api/task/pipeline/start` creates an in-memory workflow run.
2. The router matches the URL to a content platform handler.
3. The platform handler runs `Probe`.
4. The API checks whether this content already has download records.
5. The workflow enters `pause_after_probe`, a `user_confirmation` node with
   `status=waiting_user`.
6. The frontend displays `interaction.form`, `content`, `existing`, and
   `output`.
7. `POST /api/task/pipeline/resume` submits the user choices.
8. The confirmation node output becomes the user selection, for example
   `variant_id`, `filename`, `suffix`, and `spec`.
9. The platform handler runs `Resolve`, creates a task, then executes the
   platform `PipelinePlan`.

The start response intentionally exposes large body data only once:

- `data.output` is the probe node output.
- `data.output.content` is the generic `Probe.Content`.
- `data.output.body_html` is text content HTML when available.
- `data.interaction.form` is the confirmation node input form.
- `data.workflow` is a slim state snapshot and should not contain large body
  payloads.
- `data.probe` is a public view and must not expose `Probe.Internal`.

`Probe.Internal` may contain heavy internal objects such as parsed Zhihu pages
or Official Account articles. Those objects are kept in memory so `Resolve` and
executors can avoid re-fetching, but they should not be serialized to the
frontend.

## Handler Responsibilities

`Match(rawURL string) bool`

Return true if the handler owns the URL. Keep this cheap and deterministic.

`Probe(ctx, input) (*download.Probe, error)`

Fetch/parse platform data needed before user confirmation. Populate:

- `Platform`, `SourceURL`, `CanonicalURL`
- `ContentID`
- `Content` as `download.Content[T]`, where `T` is owned by the platform
  package. For example, Zhihu answer content can contain both `question` and
  `answer`; WeChat Channels content can contain the feed object JSON.
- `Variants`, `Defaults`
- `Content.Summary` for the small cross-platform projection used by filenames
  and confirmation UI
- `Content.Metadata` for content-owned auxiliary IDs and attributes
- `Content.Output` for content-derived probe output such as `body_html`
- `Internal` for execution cache only

Keep platform-specific auxiliary identifiers, such as nonce IDs, question IDs,
or feed URL parts, in `Content.Metadata` or explicitly named
`ResolvedRequest.Labels`
instead of adding generic ID slots to the shared probe contract.

For text platforms, `Content.Output` should include:

```json
{
  "content": "... added by API wrapper ...",
  "format": "html",
  "content_type": "answer|question|article",
  "title": "...",
  "source_url": "...",
  "body_html": "..."
}
```

The API wrapper adds `output.content` from `Probe.Content`. Platform handlers
should provide content-specific output values such as `body_html` and
`question_html`.

`Resolve(ctx, input) (*download.ResolvedRequest, error)`

Use `input.Probe` when present. This is important because start already fetched
the platform data. Avoid doing the same network fetch twice for Zhihu and
Official Account. Convert the selected user options into a `DownloadSpec`.

`Plan(ctx, resolved) (*download.PipelinePlan, error)`

Declare the platform-specific processing graph. Examples:

- video: download -> optional decrypt/transcode -> persist
- image album: zip/archive download -> persist
- HTML/text: render/sanitize -> parse assets -> download assets -> rewrite HTML
  -> persist

## Text Content Pipeline

Zhihu and Official Account are text-first platforms. Their probe step already
fetches the page/article body, so the expected flow is:

1. probe node outputs structured content and `body_html`
2. user confirmation node outputs selected options
3. parse HTML node extracts image URLs from `body_html`
4. asset download nodes fetch images in parallel
5. rewrite node replaces `img src` with local artifact paths
6. build artifact node writes the final HTML/package destination
7. persist node records the completed download task

This keeps the expensive platform fetch in one place while allowing later nodes
to work from pipeline data.

## Package Layout

- `download/`: shared contracts, router, downloader, and source executors.
- `channels/`: WeChat Channels handler, including video and picture album
  variants.
- `douyin/`: Douyin video handler.
- `zhihu/`: Zhihu answer/question/article handler and HTML executor.
- `officialaccount/`: WeChat Official Account article handler and HTML
  executor.
- `youtube/`: YouTube video handler. It extracts watch/player metadata,
  exposes direct progressive video, MP3-from-audio, and cover variants.

## Adding a Platform

1. Create `pkg/contentplatform/<name>/handler.go`.
2. Implement `download.Handler`.
3. Add focused tests for `Match`, `Probe`, and `Resolve`.
4. Register the handler in `APIClient.platformDownloadRouter`.
5. If the platform needs a custom protocol, implement a `download.SourceExecutor`
   and register it where the downloader is constructed.
6. Keep large internal parsed data in `Probe.Internal`, not in API response
   views.

## Naming Notes

Use `contentplatform` for source-site adapters. Do not add new content platform
code under `pkg/platform`; that package is reserved for OS/platform helpers.

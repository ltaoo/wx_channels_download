# asset-sync

`asset-sync` is a small companion CLI for repository assets that are useful for tests or scraping development but too large or noisy for Git.

Git stores:

- `.asset-sync.yaml`: sync configuration.
- `.asset-sync.lock.json`: asset index. It can be file-level or root-level depending on `track_files`.

Git does not store:

- `scraper_examples/`: large local fixtures and captured pages.
- The object storage credentials or remote configuration.

## Configure Storage

The committed config should not contain private hostnames, local usernames, or absolute home paths. Keep those values in environment variables or in `.asset-sync.local.yaml`, which is ignored by Git.

The repository default reads the storage root from `ASSET_SYNC_STORAGE_ROOT`. `asset-sync` loads the repository `.env` file before reading `.asset-sync.yaml`, so local values can live there without being committed:

```yaml
roots:
  - path: scraper_examples
    include:
      - "**/*"
    exclude:
      - "**/.DS_Store"
      - "**/*.tmp"

storage:
  type: local
  local_path: ${ASSET_SYNC_STORAGE_ROOT}
  prefix: wx_channels_download
```

Set the value per machine in `.env`:

```bash
ASSET_SYNC_STORAGE_ROOT="$HOME/SynologyDrive/asset-sync-store"
```

Then run:

```bash
asset-sync sync
```

An environment variable already exported by the shell takes precedence over `.env`.

The effective object path is:

```text
$ASSET_SYNC_STORAGE_ROOT/wx_channels_download/<root>/<file>
```

Synology Drive/WebDAV handles machine-to-machine transfer. `asset-sync` only copies between the repository and that synced directory, then verifies hashes.

If a root sets `track_files: false`, Git records only that `scraper_examples` is an external synced root. It does not record file names, hashes, or sizes in `.asset-sync.lock.json`.

With `track_files: false`, the generated lock looks like:

```json
{
  "version": 1,
  "roots": ["scraper_examples"],
  "opaque_roots": [
    {
      "path": "scraper_examples",
      "storage_key": "scraper_examples"
    }
  ],
  "files": []
}
```

If you need reproducible fixtures bound to a commit, omit `track_files` or set it to `true`.

If you prefer not to export environment variables, create a private `.asset-sync.local.yaml`:

```yaml
version: 1
manifest: .asset-sync.lock.json
roots:
  - path: scraper_examples
storage:
  type: local
  local_path: /path/to/private/asset-sync-store
  prefix: wx_channels_download
```

Then pass it explicitly:

```bash
asset-sync --config .asset-sync.local.yaml sync
```

For hooks, install with the same config path:

```bash
asset-sync install-hooks --command 'asset-sync --config .asset-sync.local.yaml'
```

`asset-sync hostname` is available for local debugging, but avoid committing real hostnames as `devices` keys unless the repository is private and that exposure is intentional.

Device mappings are still supported for private configs. Resolution order is: `--device`, `ASSET_SYNC_DEVICE`, `device` in config, then hostname matching when `device: auto`.

For S3, Cloudflare R2, OSS, COS, MinIO, WebDAV, SFTP, and similar backends, switch the storage block to `rclone`:

```yaml
storage:
  type: rclone
  rclone_binary: rclone
  prefix: wx_channels_download

devices:
  ci:
    storage_root: r2:wx-channels-download
```

The rclone object path is:

```text
r2:wx-channels-download/wx_channels_download/<root>/<file>
```

For local testing without Synology Drive, use a private config:

```yaml
version: 1
manifest: .asset-sync.lock.json
roots:
  - path: scraper_examples
storage:
  type: local
  local_path: ../wx_channels_download_asset_store
  prefix: wx_channels_download
```

Then run commands with:

```bash
go run ./cmd/asset-sync --config .asset-sync.local.yaml status
```

## Workflow

For day-to-day use, run one command:

```bash
go run ./cmd/asset-sync sync
git add .asset-sync.lock.json
git commit -m "update scraper examples"
git push
```

`sync` uploads local added or modified assets, downloads missing assets, and updates `.asset-sync.lock.json` when the tracked asset set changed.

For `track_files: false` roots, `sync` copies the whole configured root in both directions without deleting either side. `include` and `exclude` still apply, but file names are not written to Git.

On another machine:

```bash
git pull
go run ./cmd/asset-sync sync
```

Lower-level commands are still available when you want one direction only:

```bash
go run ./cmd/asset-sync push
go run ./cmd/asset-sync pull
```

Verify local assets:

```bash
go run ./cmd/asset-sync verify
```

`verify --strict` also fails on local files that are not listed in the lock. Opaque roots with `track_files: false` cannot be hash-verified because the file list is intentionally outside Git.

## Git Hooks

Install hooks after building or installing the CLI:

```bash
go run ./cmd/asset-sync install-hooks --command asset-sync
```

When using the development entrypoint directly:

```bash
go run ./cmd/asset-sync install-hooks --command 'go run ./cmd/asset-sync'
```

Installed hooks:

- `post-merge`: pulls assets when `.asset-sync.lock.json` changed after merge.
- `post-rewrite`: covers `git pull --rebase`.
- `post-checkout`: covers branch switches.
- `pre-push`: blocks push when local assets are added or modified but the lock was not updated.

Hooks never delete local or remote assets. Remote cleanup should be a separate explicit command in a later version.

## Commands

```bash
asset-sync init
asset-sync sync
asset-sync status
asset-sync push
asset-sync pull
asset-sync verify
asset-sync hostname
asset-sync install-hooks
```

`push --all` re-uploads every locked file. `push --allow-empty` is required before writing an empty lock when the previous lock still had files.

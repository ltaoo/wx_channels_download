# Docker Webtop image

Build an arm64 Webtop image with WeChat, the SunnyRoot certificate, and the
Linux `wx_video_download` binary:

```bash
bash scripts/build-webtop-image.sh
```

The build script expects the WeChat package at
`/Users/litao/Downloads/WeChatLinux_arm64.deb`. Override it when needed:

```bash
WECHAT_DEB=/path/to/WeChatLinux_arm64.deb IMAGE=wx-download:webtop bash scripts/build-webtop-image.sh
```

Run the container:

```bash
bash scripts/run-webtop-container.sh
```

The defaults mirror the manual Webtop command: port `3000`, `NET_ADMIN`,
`seccomp=unconfined`, `/dev/net/tun`, `PUID=1000`, `PGID=1000`, timezone, and
`/config:/config` persistence. If a container named `wx_download` already
exists, run with another name:

```bash
NAME=wx_download_test WEB_PORT=3100 bash scripts/run-webtop-container.sh
```

Open `http://127.0.0.1:3000` for the desktop. WeChat starts automatically, and
`wx_video_download` starts in a visible terminal window so its runtime logs stay
on screen.

Persistent files:

- `/config/wx_video_download/config.yaml`
- `/config/wx_video_download/app.log`
- `/config/logs/wx_video_download.out.log`
- `/config/logs/wx_video_download_terminal.log`
- `/config/logs/wechat.log`
- `/config/Downloads`

Useful runtime overrides:

- `WECHAT_AUTOSTART=false`
- `WX_VIDEO_AUTOSTART=false`
- `CONFIG_DIR=/host/path`
- `WEB_PORT=3100`

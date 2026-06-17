# Docker image

Docker 镜像包含 WeChat、SunnyRoot 系统根证书、Linux `wx_video_download` 二进制和
自动启动脚本。

默认发布镜像：

```text
ghcr.io/ltaoo/wx_video_download:v260607
```

使用已发布镜像时，不需要自己准备 `WeChatLinux_arm64.deb`。deb 只用于本地构建或调试
Docker 镜像。

可以先检查镜像是否可拉取：

```bash
docker pull ghcr.io/ltaoo/wx_video_download:v260607
```

如果提示 `denied`，说明镜像还没有发布、GHCR 包是私有的，或当前 Docker 没有登录
GHCR。此时需要先发布/公开镜像，或者执行 `docker login ghcr.io` 后再拉取；否则只能走
“本地构建镜像”路径。

## 直接启动

```bash
bash scripts/run-webtop-container.sh
```

默认参数等价于下面的手动 Webtop 命令：`3000` 端口、`NET_ADMIN`、
`seccomp=unconfined`、`/dev/net/tun`、`PUID=1000`、`PGID=1000`、
`Asia/Shanghai`、`/config:/config` 持久化，并使用固定 hostname
`wx-linux`。

如果已经存在同名容器，换一个容器名和端口：

```bash
NAME=wx_download_test WEB_PORT=3100 bash scripts/run-webtop-container.sh
```

打开 `http://127.0.0.1:3000` 进入桌面。WeChat 会自动启动，
`wx_video_download` 会在可见终端窗口中前台运行，方便直接观察日志。

## 手动 docker run 示例

```bash
docker run -d \
  --name=wx_download \
  --restart=unless-stopped \
  --hostname=wx-linux \
  --security-opt seccomp=unconfined \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -e RESOLUTION=1920x1080x24 \
  -p 3000:3000 \
  -v /config:/config \
  ghcr.io/ltaoo/wx_video_download:v260607
```

## 多账号多开

明确规则：

- 不同 WeChat 账号必须使用不同的宿主机 `CONFIG_DIR`，例如
  `/config/wxaccounts/wxaccount1`、`/config/wxaccounts/wxaccount2`。
- 不同账号同时运行时，也必须使用不同的 `NAME` 和 `WEB_PORT`。
- 建议不同账号使用不同但固定的 `CONTAINER_HOSTNAME`，让每个账号对应一个稳定的
  Linux 环境。
- 同一个 WeChat 账号重建或升级容器时，继续使用同一个 `CONFIG_DIR` 和
  `CONTAINER_HOSTNAME`，不要并发启动两份相同账号的数据目录。
- `/config` 目录里保存 WeChat 用户数据、`wx_video_download` 配置、系统
  `machine-id` 和 hostname。删除或更换这些数据会让 WeChat 看到一个新的 Linux
  环境。
- 不要在账号已经登录后，把一个账号的 `/config` 复制给另一个账号使用。如果需要模板，
  只在未登录 WeChat 之前复制空白初始化目录。

推荐目录结构：

```text
/config/wxaccounts/
  wxaccount1/
  wxaccount2/
  wxaccount3/
```

启动账号 1：

```bash
NAME=wxaccount1 \
WEB_PORT=3001 \
CONFIG_DIR=/config/wxaccounts/wxaccount1 \
CONTAINER_HOSTNAME=wx-linux-account1 \
bash scripts/run-webtop-container.sh
```

启动账号 2：

```bash
NAME=wxaccount2 \
WEB_PORT=3002 \
CONFIG_DIR=/config/wxaccounts/wxaccount2 \
CONTAINER_HOSTNAME=wx-linux-account2 \
bash scripts/run-webtop-container.sh
```

启动账号 3 时继续递增即可：

```bash
NAME=wxaccount3 \
WEB_PORT=3003 \
CONFIG_DIR=/config/wxaccounts/wxaccount3 \
CONTAINER_HOSTNAME=wx-linux-account3 \
bash scripts/run-webtop-container.sh
```

访问地址：

```text
账号 1: http://127.0.0.1:3001
账号 2: http://127.0.0.1:3002
账号 3: http://127.0.0.1:3003
```

停止、删除、重建某个账号容器时，只删除容器，不删除 `CONFIG_DIR`：

```bash
docker stop wxaccount1
docker rm wxaccount1

NAME=wxaccount1 \
WEB_PORT=3001 \
CONFIG_DIR=/config/wxaccounts/wxaccount1 \
CONTAINER_HOSTNAME=wx-linux-account1 \
bash scripts/run-webtop-container.sh
```

## 本地构建镜像

普通使用不需要本地构建。只有修改 Dockerfile、调试镜像内容，或还没有可用发布镜像时，
才需要自己准备 `WeChatLinux_arm64.deb`。

构建 arm64 Webtop 镜像：

```bash
bash scripts/build-webtop-image.sh
```

默认读取 `/Users/litao/Downloads/WeChatLinux_arm64.deb`。需要指定路径或镜像名时：

```bash
WECHAT_DEB=/path/to/WeChatLinux_arm64.deb \
IMAGE=wx_video_download:v260607 \
bash scripts/build-webtop-image.sh
```

默认配置来自 `internal/config/config.template.yaml`，`global.js` 不存在时会使用空脚本占位。需要打包自定义默认配置或用户脚本时：

```bash
CONFIG_FILE=/path/to/config.yaml \
GLOBAL_SCRIPT=/path/to/global.js \
bash scripts/build-webtop-image.sh
```

使用本地构建镜像启动：

```bash
IMAGE=wx_video_download:v260607 bash scripts/run-webtop-container.sh
```

手动启动本地镜像时，把镜像名改成 `wx_video_download:v260607` 即可：

```bash
docker run -d \
  --name=wx_download_local \
  --restart=unless-stopped \
  --hostname=wx-linux \
  --security-opt seccomp=unconfined \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -e RESOLUTION=1920x1080x24 \
  -p 3000:3000 \
  -v /config:/config \
  wx_video_download:v260607
```

## 持久化文件

- `/config/wx_video_download/config.yaml`
- `/config/wx_video_download/app.log`
- `/config/.wx_identity/machine-id`
- `/config/.wx_identity/hostname`
- `/config/logs/wx_video_download.out.log`
- `/config/logs/wx_video_download_terminal.log`
- `/config/logs/wechat.log`
- `/config/Downloads`

## 常用运行参数

- `IMAGE=ghcr.io/ltaoo/wx_video_download:v260607`
- `WECHAT_AUTOSTART=false`
- `WX_VIDEO_AUTOSTART=false`
- `WX_CHANNELS_DOWNLOAD_CONFIG_FILEPATH=/config/wx_video_download/config.yaml`
- `CONFIG_DIR=/host/path`
- `CONTAINER_HOSTNAME=wx-linux`
- `WEB_PORT=3100`

## 检查运行状态

```bash
docker exec -it wxaccount1 wx-status
docker exec -it wxaccount1 tail -f /config/logs/wx_video_download.out.log
docker exec -it wxaccount1 tail -f /config/logs/wechat.log
```

备份某个账号时，先停止容器，再备份对应目录：

```bash
docker stop wxaccount1
tar -C /config/wxaccounts -czf wxaccount1-backup.tgz wxaccount1
```

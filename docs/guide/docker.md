---
title: 使用 Docker 运行
---

# 使用 Docker 运行

本文适合只有 Docker、没有源码目录的用户。镜像已经包含 WeChat、系统根证书和
`wx_video_download`。

镜像地址：

```text
ghcr.io/ltaoo/wx_video_download:v260607
```

## 启动

先创建一个持久化数据卷，用来保存 WeChat 登录状态、下载器配置、日志和下载文件：

```bash
docker volume create wx_download_config
```

启动容器：

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
  -v wx_download_config:/config \
  ghcr.io/ltaoo/wx_video_download:v260607
```

打开浏览器访问：

```text
http://127.0.0.1:3000
```

进入桌面后，WeChat 和 `wx_video_download` 会自动启动。登录 WeChat 后，在容器桌面里打开
视频号页面即可使用。

## 多账号

不同 WeChat 账号要使用不同的数据卷、容器名、端口和 hostname。

账号 1：

```bash
docker volume create wxaccount1_config

docker run -d \
  --name=wxaccount1 \
  --restart=unless-stopped \
  --hostname=wx-linux-account1 \
  --security-opt seccomp=unconfined \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -e RESOLUTION=1920x1080x24 \
  -p 3001:3000 \
  -v wxaccount1_config:/config \
  ghcr.io/ltaoo/wx_video_download:v260607
```

账号 2：

```bash
docker volume create wxaccount2_config

docker run -d \
  --name=wxaccount2 \
  --restart=unless-stopped \
  --hostname=wx-linux-account2 \
  --security-opt seccomp=unconfined \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -e RESOLUTION=1920x1080x24 \
  -p 3002:3000 \
  -v wxaccount2_config:/config \
  ghcr.io/ltaoo/wx_video_download:v260607
```

访问地址：

```text
账号 1: http://127.0.0.1:3001
账号 2: http://127.0.0.1:3002
```

不要让两个容器同时使用同一个数据卷。重建同一个账号的容器时，继续使用原来的数据卷。

## 常用命令

查看容器：

```bash
docker ps
```

查看运行状态：

```bash
docker exec -it wx_download wx-status
```

查看下载器日志：

```bash
docker exec -it wx_download tail -f /config/logs/wx_video_download.out.log
```

停止容器：

```bash
docker stop wx_download
```

重新启动容器：

```bash
docker start wx_download
```

删除容器：

```bash
docker rm wx_download
```

删除容器不会删除 `wx_download_config` 数据卷。只要继续挂载同一个数据卷，WeChat 数据就会保留。

---
title: 使用 Docker 运行
---

# 使用 Docker 运行

> 在 Docker 里的 linux 运行微信并登录，风险自己承担，建议用小号

镜像地址：

```text
ghcr.io/ltaoo/wx_video_download:v260614
```

## 启动

先创建一个目录集中存放微信数据目录，登录状态不丢失。同时视频也是下载到该目录中

```bash
mkdir wxchannelsdata
```

为第一个容器创建数据目录

```bash
cd wxchannelsdata
mkdir wx_account1
# 确保当前在 wxchannelsdata 目录中
pwd
~/xxx/wxchannelsdata
ls
wx_account1
```

然后启动容器

> 端口3000 如果有冲突，则修改为 `-p 8001:3000` ，即左边的端口修改为不会冲突的端口
> 端口2022、2023 如果有冲突，也按同样方式修改左边的宿主机端口

```bash
docker run -d \
  --name=wx_account1 \
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
  -p 2022:2022 \
  -p 2023:2023 \
  -v ./wx_account1:/config \
  ghcr.io/ltaoo/wx_video_download:v260614
```

打开浏览器访问：

```text
http://127.0.0.1:3000
```

进入桌面后，WeChat 和 `wx_video_download` 会自动启动。登录 WeChat 后，在容器桌面里打开视频号页面即可使用。下载好的视频默认在下面目录

```bash
~/xxx/wxchannelsdata/wx_account1/Downloads
```

不要启动多个容器运行多个微信帐号，没有经过测试可能封号风险很大

<!-- 
## 多账号

使用第二个微信帐号登录

```bash
pwd
# ~/xxx/wxchannelsdata
mkdir wx_account2
```

然后启动容器

```bash
docker run -d \
  --name=wx_account2 \
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
  -p 2024:2022 \
  -p 2025:2023 \
  -v ./wx_account2:/config \
  ghcr.io/ltaoo/wx_video_download:v260614
```

第二个帐号使用 3001 端口访问

```text
账号 2: http://127.0.0.1:3001
```

不要让两个容器同时使用同一个数据卷。重建同一个账号的容器时，继续使用原来的数据卷。 -->

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

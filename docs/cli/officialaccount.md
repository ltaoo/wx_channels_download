---
title: 公众号远端服务
---

# 公众号远端服务

仅开启公众号接口、RSS能力，使其可以在 `linux` 服务器上运行

## 用法

```sh
wx_video_download mp -d
```

将以守护进程模式运行，端口默认为 `2022`，可以在 `config.yaml` 中修改。

### 非守护进程模式运行

```bash
wx_video_download mp start
```

将启动服务，但默认在前台运行，服务器退出后服务也会关闭

### 查看服务状态

```bash
wx_video_download mp status
```

查看端口号、是否运行中


### 停止服务

```bash
wx_video_download mp stop
```

将停止服务

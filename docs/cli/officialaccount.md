---
title: 公众号远端服务
---

# 公众号远端服务

仅开启公众号接口、RSS能力，使其可以在 `linux` 服务器上运行

## 用法

```sh
wx_video_download server
```

将以前台方式启动服务，端口默认为 `2022`，可以在 `config.yaml` 中修改。终端退出后服务也会关闭。

### 查看服务状态

```bash
wx_video_download server status
```

查看端口号、是否运行中


### 停止服务

```bash
wx_video_download server stop
```

也可以在运行服务的终端按 `Ctrl+C`。

### 重启服务

```bash
wx_video_download server restart
```

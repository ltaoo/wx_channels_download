---
title: 服务器模式
---

# 服务器模式

用于在 `linux` 服务器上运行，提供 `API` 服务

## 用法

```sh
wx_video_download server
```

服务以前台方式运行，终端退出后服务也会关闭。

### 查看服务状态

```sh
wx_video_download server status
```

### 停止服务

```sh
wx_video_download server stop
```

也可以在运行服务的终端按 `Ctrl+C`。

### 重启服务

```sh
wx_video_download server restart
```

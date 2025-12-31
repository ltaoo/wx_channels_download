---
title: API
---

# API

可以通过 `HTTP` 请求来调用下载器的 API，现在提供了以下几个接口

- 获取下载任务列表
- 开始下载任务
- 暂停下载任务
- 删除下载任务
- 搜索视频号账号
- 获取指定视频号账号发布的视频
- 获取视频号视频的详情

视频号相关接口，都需要保持视频号页面打开的状态，否则调用接口时会提示 `{"code":400,"msg":"请先初始化客户端 socket 连接"}`。当出现该提示时，打开视频号页面即可，如果已经打开，刷新一下即可。

## 搜索视频号账号

```bash
curl http://localhost:2022/api/channels/contact/search?keyword=龙虾
```

## 获取视频号账号的视频列表

```bash
curl http://localhost:2022/api/channels/contact/feed/list?username=v2_060000231003b20faec8c4e48e1dc3ddcd03ec3cb077bb11b3c6c9a42ee3cea8073a64e6e2bd@finder
```

## 获取视频号指定视频详情

```bash
curl http://localhost:2022/api/channels/feed/profile?oid=14545102784038246591&nid=17453311911488030792_0_140_2_32_5217671461141767_bf99bd48-e5eb-11f0-9859-81948764da50
```

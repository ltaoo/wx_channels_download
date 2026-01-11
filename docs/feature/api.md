---
title: API
---

# API

可以通过 `HTTP` 请求来调用下载器的 API，现在提供了以下几类接口

- 视频号相关
- 公众号相关
- 下载器相关

## 视频号接口

视频号相关接口，都需要保持视频号页面打开的状态，否则调用接口时会提示 `{"code":400,"msg":"请先初始化客户端 socket 连接"}`。当出现该提示时，打开视频号页面即可，如果已经打开，刷新一下即可。

### 搜索视频号账号

```bash
curl http://localhost:2022/api/channels/contact/search?keyword=龙虾
```

### 获取视频号指定账号的视频列表

```bash
curl http://localhost:2022/api/channels/contact/feed/list?username=v2_060000231003b20faec8c4e48e1dc3ddcd03ec3cb077bb11b3c6c9a42ee3cea8073a64e6e2bd@finder
```

### 获取视频号指定视频详情

```bash
curl http://localhost:2022/api/channels/feed/profile?oid=14545102784038246591&nid=17453311911488030792_0_140_2_32_5217671461141767_bf99bd48-e5eb-11f0-9859-81948764da50
```

或者直接传入视频号页面地址，如 `https://channels.weixin.qq.com/web/pages/feed?oid=zagCB5LjCrE&nid=d3pMFaDgxy4`

传入的地址需要进行编码

```js
encodeURIComponent(
  "https://channels.weixin.qq.com/web/pages/feed?oid=zagCB5LjCrE&nid=d3pMFaDgxy4"
);
```

```bash
curl http://localhost:2022/api/channels/feed/profile?url=https%3A%2F%2Fchannels.weixin.qq.com%2Fweb%2Fpages%2Ffeed%3Foid%3DzagCB5LjCrE%26nid%3Dd3pMFaDgxy4
```

### 视频代理播放

```bash
curl "http://localhost:2022/play?url=http://example.com/video.mp4&key=12345"
```

url 是视频地址，非「视频号页面地址」，同样需要对地址进行编码

### RSS 订阅

获取指定视频号的 RSS Feed。

```bash
curl "http://localhost:2022/rss/channels?username=v2_xxx@finder"
```

## 公众号接口

### 获取公众号列表

```bash
curl "http://localhost:2022/api/mp/list?token=YOUR_TOKEN&page=1&page_size=10"
```

### 获取公众号消息列表

```bash
curl "http://localhost:2022/api/mp/msg/list?token=YOUR_TOKEN&biz=BIZ_ID&offset=0"
```

### 删除公众号

```bash
curl -X POST "http://localhost:2022/api/mp/delete?token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"biz": "BIZ_ID"}'
```

### 刷新凭证

该接口需要打开公众号文章页面（任意一个即可）

```bash
curl -X POST "http://localhost:2022/api/mp/refresh?token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"biz": "BIZ_ID", "key": ""}'
```

### RSS 订阅

```bash
curl "http://localhost:2022/rss/mp?token=YOUR_TOKEN&biz=BIZ_ID"
```

## 下载器接口

### 创建视频号下载任务

```bash
curl -X POST "http://127.0.0.1:2022/api/task/create_channels" \
  -H "Content-Type: application/json" \
  -d '{"cover": true, "url": "https://channels.weixin.qq.com/web/pages/feed?oid=zagCB5LjCrE&nid=d3pMFaDgxy4&context_id=33-9-141-18a2bc728e23eacd62e8fc98e3bbff391768023553823&entrance_id=1002&req_time=1767887337&exportkey=n_ChQIAhIQeQFDxKsyNw296ySx31udbxKMAgIE97dBBAEAAAAAAJtcOgVgeQAAAAAOpnltbLcz9gKNyK89dVj0Z5WHOTv3WqpHc4LJgnpXV6g383YJo8%2BUIOjT2Y9k%2FNCj%2BnAXGPEP5rSwX6eTMFijG9xRV5wJM7F4%2F%2BKked55Q2Ao8WRg7LI05FClrpb0iNlfi%2B4HttbXt0E5o4U3vpzAAb%2F3WXhHUBrbc3DgmXHpxOSHPx3BdgQaE7IotUe9IS5cv%2Bf3BJCBEI1pZHs3e5%2FMs1ZRjV3Crwg0%2FShUoUG%2FqKstXMRHn2KJ0uM4H93DWxIBxtTMnDbk3%2F9CFjCo6n4J73vvGRoIex8nLMUZ%2FC6mW3GYqn%2Fp9hp70GlLg5ScHexjw3HklyQ%3D&pass_ticket=VgBMoEBGN9Dup64gcPQ%2BHeruABRSIVerbzmQp9w1bCZsXDsoZwddjH0M%2Bzaey5yuJsXz02LqYJZrqzgl57DvKw%3D%3D&wx_header=0"}'
```

其中 `body` 和详情接口一样，支持传 `oid`+`nid`，或者 `url`。

传 `mp3`，表示下载为 `mp3` 文件

传 `cover`，表示下载视频封面

### 获取所有下载任务

```bash
curl "http://localhost:2022/api/task/list?status=all&page=1&page_size=20"
```

参数说明：

- `status`: 任务状态过滤，可选值：`all` (默认), `ready`, `running`, `pause`, `error`, `done`
- `page`: 页码，默认为 1
- `page_size`: 每页数量，默认为 20

### 开始任务

```bash
curl -X POST "http://localhost:2022/api/task/start" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

### 暂停任务

```bash
curl -X POST "http://localhost:2022/api/task/pause" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

### 恢复任务

```bash
curl -X POST "http://localhost:2022/api/task/resume" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

### 删除任务

```bash
curl -X POST "http://localhost:2022/api/task/delete" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

### 清空所有任务

```bash
curl -X POST "http://localhost:2022/api/task/clear"
```

## 系统操作

### 打开下载目录

```bash
curl -X POST "http://localhost:2022/api/open_download_dir"
```

### 在文件夹中显示文件

```bash
curl -X POST "http://localhost:2022/api/show_file" \
  -H "Content-Type: application/json" \
  -d '{"path": "/absolute/path/to/download/dir", "name": "filename.mp4"}'
```

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

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `keyword` | 是 | 搜索关键词 |

### 获取视频号指定账号的视频列表

```bash
curl http://localhost:2022/api/channels/contact/feed/list?username=v2_060000231003b20faec8c4e48e1dc3ddcd03ec3cb077bb11b3c6c9a42ee3cea8073a64e6e2bd@finder
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `username` | 是 | 视频号 ID，例如 `v2_xxx@finder` |
| `next_marker` | 否 | 分页标记，用于获取下一页数据 |

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

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `oid` | 否 | 视频唯一 ID (配合 nid 使用) |
| `nid` | 否 | 视频唯一 ID (配合 oid 使用) |
| `url` | 否 | 视频号页面 URL (如果提供，将自动解析出 oid 和 nid) |

> 注意：必须提供 `oid`+`nid` 或 `url` 其中一种组合。

### RSS 订阅

获取指定视频号的 RSS Feed。

```bash
curl "http://localhost:2022/rss/channels?username=v2_xxx@finder"
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `username` | 是 | 视频号 ID |
| `next_marker` | 否 | 分页标记 |

### 视频代理播放

```bash
curl "http://localhost:2022/play?url=http://example.com/video.mp4&key=12345"
```

url 是视频地址，非「视频号页面地址」，同样需要对地址进行编码

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `url` | 是 | 目标视频地址 |
| `key` | 否 | 解密密钥 |

## 公众号接口

### 获取公众号列表

```bash
curl "http://localhost:2022/api/mp/list?token=YOUR_TOKEN&page=1&page_size=10"
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |
| `page` | 否 | 页码，默认 1 |
| `page_size` | 否 | 每页数量，默认 10 |
| `keyword` | 否 | 搜索关键词 (匹配 biz 或 nickname) |
| `is_effective` | 否 | 过滤是否有效 (1/true 为有效) |

### 获取公众号消息列表

```bash
curl "http://localhost:2022/api/mp/msg/list?token=YOUR_TOKEN&biz=BIZ_ID&offset=0"
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |
| `biz` | 是 | 公众号唯一 ID |
| `offset` | 否 | 偏移量，默认 0 |

### 删除公众号

```bash
curl -X POST "http://localhost:2022/api/mp/delete?token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"biz": "BIZ_ID"}'
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `biz` | 是 | 公众号唯一 ID |

### 使用前端刷新凭证

借助已连接的前端（公众号文章页面）批量刷新公众号凭证。该接口会同步等待任务完成（或超时），但只返回操作结果状态，不返回详细的刷新数据。

```bash
curl -X POST "http://localhost:2022/api/mp/refresh_with_frontend" \
  -H "Content-Type: application/json" \
  -d '{"biz_list": ["BIZ_ID_1", "BIZ_ID_2"]}'
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| - | - | 无 |

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `biz_list` | 否 | 需要刷新的公众号 `biz` ID 列表。如果为空或不传，则刷新所有包含 `refresh_uri` 的公众号。 |

### 刷新凭证
该接口需要打开公众号文章页面（任意一个即可）

```bash
curl -X POST "http://localhost:2022/api/mp/refresh?token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"biz": "BIZ_ID", "key": ""}'
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `biz` | 是 | 公众号唯一 ID |
| `key` | 是 | 授权 Key |
| `pass_ticket` | 是 | 票据 |
| `appmsg_token` | 是 | 消息 Token |
| `uin` | 是 | 用户唯一 ID |
| `nickname` | 否 | 昵称 |

### RSS 订阅

```bash
curl "http://localhost:2022/rss/mp?token=YOUR_TOKEN&biz=BIZ_ID"
```

同样可以传入 `offset` 指定偏移量，获取到更多消息记录。

还提供 `proxy` 和 `content` 参数，分别用于代理公众号内容和默认获取公众号文章全文

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |
| `biz` | 是 | 公众号唯一 ID |
| `offset` | 否 | 偏移量 |
| `content` | 否 | 是否获取正文 (1 为开启) |
| `proxy` | 否 | 是否使用代理 (1 为开启) |
| `proxy_cover` | 否 | 是否只代理封面 (1 为开启) |

#### proxy

> 仅在 `linux` 部署的服务上支持

如果阅读器提供获取全文能力，但是无法正确获取到公众号文章正文，可以指定 `proxy=1`，那么返回文章列表中，文章链接都会添加 `{{APIServerAddr}}/mp/proxy` 前缀，当打开文章时，将使用代理代为请求微信公众号返回正文内容

#### proxy_cover

> 仅在 `linux` 部署的服务上支持

相比 `proxy`，`proxy_cover` 仅代理封面图片，可以用在仅需查看列表，点击跳转到原文的场景

#### content

> 仅在 `linux` 部署的服务上支持

如果希望直接获取到正文，可以指定 `content=1`，那么请求 `RSS` 接口时，就会同时获取正文。但是缺点就是列表接口会比较慢（因为要依次请求到正文）

### 公众号内容代理

用于代理访问公众号文章或图片，解决跨域或防盗链问题。

```bash
curl "http://localhost:2022/mp/proxy?url=ENCODED_URL&token=YOUR_TOKEN"
```

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `url` | 是 | 目标 URL (需 URL 编码) |
| `token` | 否 | 鉴权 Token (开启鉴权时必填) |

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

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `oid` | 否 | 视频唯一 ID (配合 nid 使用) |
| `nid` | 否 | 视频唯一 ID (配合 oid 使用) |
| `url` | 否 | 视频号页面 URL |
| `mp3` | 否 | 是否下载为 MP3，默认为 false |
| `cover` | 否 | 是否下载为封面，默认为 false |

### 获取所有下载任务

```bash
curl "http://localhost:2022/api/task/list?status=all&page=1&page_size=20"
```

参数说明：

- `status`: 任务状态过滤，可选值：`all` (默认), `ready`, `running`, `pause`, `error`, `done`
- `page`: 页码，默认为 1
- `page_size`: 每页数量，默认为 20

**Query 参数：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `status` | 否 | 任务状态过滤，可选值：`all` (默认), `ready`, `running`, `pause`, `error`, `done` |
| `page` | 否 | 页码，默认 1 |
| `page_size` | 否 | 每页数量，默认 20 |

### 开始任务

```bash
curl -X POST "http://localhost:2022/api/task/start" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `id` | 是 | 任务 ID |

### 暂停任务

```bash
curl -X POST "http://localhost:2022/api/task/pause" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `id` | 是 | 任务 ID |

### 恢复任务

```bash
curl -X POST "http://localhost:2022/api/task/resume" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `id` | 是 | 任务 ID |

### 删除任务

```bash
curl -X POST "http://localhost:2022/api/task/delete" \
  -H "Content-Type: application/json" \
  -d '{"id": "task_id"}'
```

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `id` | 是 | 任务 ID |

### 清空所有任务

```bash
curl -X POST "http://localhost:2022/api/task/clear"
```

### 系统操作

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

**Body 参数 (JSON)：**

| 参数名 | 必填 | 说明 |
| :--- | :--- | :--- |
| `path` | 否 | 文件所在目录绝对路径 |
| `name` | 否 | 文件名 |

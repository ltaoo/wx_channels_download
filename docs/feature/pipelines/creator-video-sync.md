---
title: 创作者视频同步
description: 复制 Pipeline JSON，定时检查创作者视频并下载缺失内容。
---

# 创作者视频同步

逐页检查创作者视频的本地下载任务，只提交缺失视频的下载任务，遇到整页已存在时停止。

点击下方代码块右上角的复制按钮，将完整 JSON 粘贴到「自动化 → 导入 Pipeline」。导入后点击流程的「配置」，填写 `username` 和可选备注。示例已内置每天 12:00、18:00 的定时触发规则。

分页列表默认通过 GET 请求 `http://127.0.0.1:2022/api/channels/contact/feed/list`，自动携带 `username` 和 `next_marker` 查询参数。在同一配置中可填写 `url` 覆盖完整接口地址，并用 `method` 选择 GET、POST、PUT、PATCH、DELETE、HEAD 或 OPTIONS（留空使用 GET）。POST 等带请求体的方法还会发送 `{"username":"...","next_marker":"..."}` JSON，GET/HEAD 不发送请求体。可选 `token` 使用 secret 输入遮罩，只需填写令牌本身，填写后发送 `Authorization: Bearer <token>`，留空不发送认证头。

列表响应直接返回包含 `BaseResponse`、`object` 的原始列表，分页字段为 `continue_flag/last_buffer`，视频字段为 `object_desc/object_nonce_id/media_type`。账号和分页游标都必须按接口约定使用字符串。

原始列表中 `object_desc.media[].spec` 的 `file_format`、`duration_ms`、`video_bitrate`、`level_order` 也会转换为下载器使用的画质字段，保留清晰度选项与默认画质选择。

下载直接复用列表返回的视频对象，通过 `create_download_task` 创建任务，不会再调用 `download_wxchannels_video` 查询详情，因此自定义 HTTP 列表接口不依赖本机视频号页面连接。接口需返回可下载的 `object_desc.media`，包括媒体 URL、URL token 以及加密视频所需的解密 key；字段名使用 `url`、`url_token`、`decode_key`。列表只返回 ID 时无法直接下载，需要由该 HTTP 接口补齐媒体数据。

已经导入的流程不会随模板文件自动更新。可在原流程编辑页导入此 JSON，核对接口地址、认证和创作者配置后保存；新导入的流程需重新填写配置。

更多说明见[导入和配置](./index.md#导入和配置)与[运行结果与边界](./index.md#运行结果与边界)。

<<< ./01_creator_video_sync.json

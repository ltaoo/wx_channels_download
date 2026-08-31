---
title: 使用第三方下载器保存视频号视频
---

# 使用第三方下载器保存视频号视频

`fetch_content` 的视频号结果会包含 `download_resources`。可将其中的 `download_url` 交给 aria2、curl 等第三方下载器；下载完成后，仅当 `requires_decryption` 为 `true` 时，使用同一项中的字符串形式 `decode_key` 调用 `decrypt_wxchannels_video`。文件必须位于运行下载器服务的同一台机器上，并使用绝对路径。

---
title: 查询账号内容和浏览记录
---

# 查询账号内容和浏览记录

查找某个视频号账号时，先用 `search_wxchannels_accounts` 获取 `username`，再调用 `get_wxchannels_account_videos` 或 `get_wxchannels_live_replays`。查询当前微信用户的关注账号、赞或收藏内容、播放记录时，可直接使用对应的三个列表工具。

如果要查询已经保存到下载器数据库中的历史数据，则使用 `get_accounts` 和 `get_browse_history`。这类查询不依赖视频号页面保持连接。

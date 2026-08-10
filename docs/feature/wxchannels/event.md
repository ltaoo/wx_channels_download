---
title: 监听事件
---

# 监听事件

可以在视频号发生事件时，触发自定义脚本。本下载器的「插入下载按钮」、「在终端打印访问的视频」，都是基于该能力实现的

目前支持的事件有

- `onFeed`：加载了 feed（首页、详情、直播都会触发）
- `onPCFlowLoaded`：首页推荐获取到视频列表
- `onRecommendFeedsLoaded`：获取到推荐列表
- `onUserFeedsLoaded`：获取到指定用户的部分视频列表
- `onGotoNextFeed`：首页推荐切换到下一个视频
- `onGotoPrevFeed`：首页推荐切换到上一个视频
- `onFetchFeedProfile`：获取到视频详情
- `onFetchLiveProfile`：获取到直播详情
- `beforeDownloadMedia`：下载视频之前
- `beforeDownloadCover`：下载封面之前
- `onMediaDownloaded`：视频下载完成
- `onMP3Downloaded`：MP3 下载完成
- `onDOMContentLoaded`：DOM 完全加载和解析
- `onDOMContentBeforeUnLoaded`：DOM 加载完成前（页面即将离开，DOM 仍存在）
- `onWindowLoaded`：所有资源加载完成
- `onWindowUnLoaded`：页面卸载完成（DOM 即将被销毁）

参数类型可以参考 [`utils.d.ts`](https://github.com/ltaoo/wx_channels_download/blob/main/internal/interceptor/inject/src/utils.d.ts)

<a href="https://github.com/ltaoo/wx_channels_download/blob/main/internal/interceptor/inject/src/utils.d.ts?raw=1" download>点击下载 utils.d.ts</a>

基于上面的事件，可以实现任意的功能，包括但不限于

- 记录所有访问过的视频
- 记录下载过的视频
- 自动下载所有视频

下面给出一个「打印访问过的视频」功能代码示例

## 打印访问过的视频

在和 `wx_video_download.exe` 同目录下增加一个 `global.js` 文件，内容如下

```js
// global.js
WXU.onFeed(async (feed) => {
  const [err, res] = await WXU.request({
    method: "POST",
    url: "http://127.0.0.1:1234/api/feed",
    body: feed,
  });
  if (err) {
    WXU.error({ msg: err.message });
    return;
  }
  WXU.log({ msg: JSON.stringify(res) });
});
```

该代码实现了在访问视频时，将视频的完整信息，提交到 `http://127.0.0.1:1234/api/feed` 这个服务

```go
// main.go
package main

import (
	"io"
	"log"
	"net/http"
)

func setCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func feedHandler(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("read body error: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	log.Printf("feed: %s", string(body))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/feed", feedHandler)
	addr := "127.0.0.1:1234"
	log.Printf("HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

上面代码中

```go
log.Printf("feed: %s", string(body))
```

就是打印访问的视频完整信息。有了完整信息，可以自己实现保存到数据库、下载视频等任意功能。

如果要实现下载访问过的视频，只需要在上面 `log.Printf` 处，解析传过来的 `body`，拿到 `url` 和 `key`，下载并解密即可。

下载和解密功能，可以参考本项目 `cmd` 目录下代码

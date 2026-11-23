---
title: 监听事件
---

# 监听事件

全局用户脚本可以通过 `WXU` 监听视频号页面事件。事件来自页面中的 API 调用、视频切换和下载入口，具体触发情况取决于当前页面及对应代码是否已注入。

## 事件与参数

### 视频列表与详情

| 监听方法 | 回调参数 | 触发时机 |
| --- | --- | --- |
| `onFeed` | 原始 `ChannelsFeed` | 首页或视频详情页设置当前视频，包括首次加载、前后切换及本地列表切换；不是列表中每条视频都会触发 |
| `onPCFlowLoaded` | `result.data.object`，视频数组 | `finderPcFlow` 返回后 |
| `onRecommendFeedsLoaded` | 视频数组 | `finderGetRecommend` 或 `getRecommendTabsFromService` 返回后；后者从 `result.resp` 或 `result` 中取 `data.object` |
| `onInteractionedFeedsLoaded` | `result.data.object`，视频数组 | `finderGetInteractionedFeedList` 返回后 |
| `onUserFeedsLoaded` | `result.data.object`，视频数组 | `finderUserPage` 返回后 |
| `onLiveUserFeedsLoaded` | `result.data.object`，回放数组 | `finderLiveUserPage` 返回后 |
| `onGotoNextFeed` | 切换后的原始 `ChannelsFeed` | 切换到下一个视频；LiteApp 中仅在当前视频 ID 改变时触发 |
| `onGotoPrevFeed` | 切换后的原始 `ChannelsFeed` | 切换到上一个视频；LiteApp 中仅在当前视频 ID 改变时触发 |
| `onHomeFeedChanged` | 切换后的原始 `ChannelsFeed` | 本地播放列表切换；LiteApp 中仅在当前视频 ID 改变时触发 |
| `onFetchFeedProfile` | `result.data.object`，原始视频对象 | `finderGetCommentDetail` 返回后 |
| `onFetchFeedCommentList` | `result.data`，评论响应对象 | `finderGetCommentList` 返回后 |
| `onFetchLiveProfile` | `result.data`，直播响应对象 | `finderGetLiveInfo` 返回后；参数不是统一的 `ChannelsFeed` |
| `onJoinLive` | `result.data`，加入直播的响应对象 | `joinLive` 返回后，直播页使用此事件读取直播信息 |
| `onGetFeedH5Url` | `result.data.object` | `finderGetFeedH5Url` 返回后；若上游响应没有 `object` 字段，参数为 `undefined` |

当前直播页使用 `onFetchFeedProfile` 和 `onJoinLive` 组合初始化，没有派发 `onFeed`。监听直播数据时应分别使用这两个事件。`onFeed` 也没有自动去重，记录访问历史时可按业务需要处理重复视频。

### 初始化与页面生命周期

| 监听方法 | 回调参数 | 触发时机 |
| --- | --- | --- |
| `onAPILoaded` | 页面 API 模块的导出对象 | 页面 API 已暴露；LiteApp 中在安装 API 和导航监听后触发 |
| `onUtilsLoaded` | 工具方法对象 | 被拦截的页面工具模块加载时，暴露 `decodeBase64ToUint64String`、`createAdapterFromGlobalMapper`、`finderJoinLiveMapper`；依赖对应模块的注入匹配 |
| `onInit` | `result.data` | `finderInit` 返回后 |
| `onDOMContentLoaded` | `{ href: string }` | `document` 的 `DOMContentLoaded`；LiteApp 若在 DOM 加载后才完成注入，会补发一次 |
| `onDOMContentBeforeUnLoaded` | `{ href: string }` | `window` 的 `beforeunload`，页面即将离开、DOM 仍存在；名称中的 DOMContent 不是指加载前 |
| `onWindowLoaded` | `{ href: string }` | `window` 的 `load` |
| `onWindowUnLoaded` | `{ href: string }` | `window` 的 `unload` |

### 下载事件

| 监听方法 | 回调参数 | 触发时机与限制 |
| --- | --- | --- |
| `beforeDownloadMedia` | 当前原始视频对象的浅拷贝，增加 `source_url` | 页面视频下载入口调用具体下载方法之前；后续前端下载可能继续在此对象上追加规范化字段 |
| `beforeDownloadCover` | 无当前内置触发参数 | 只有监听方法和事件常量，当前封面下载实现没有派发该事件 |
| `onMediaDownloaded` | 合并了规范化字段的原始视频对象 | 仅在前端视频下载分支中，取回媒体并尝试解密后、调用 `WXU.save` 之前触发；解密报错后仍可能继续触发 |
| `onMP3Downloaded` | 合并了规范化字段的原始视频对象 | 仅在前端下载分支中，MP3 转换成功后、调用 `WXU.save` 之前触发 |

后端下载任务不会通过上述两个完成事件通知页面，前端图文打包下载也没有派发这两个事件。它们不代表后端任务成功或文件已经保存到磁盘；监听后端任务成功、失败和结束请使用[下载任务 Hook](../download-hooks.md)。

## 监听与取消

所有上述监听方法均返回取消监听函数：

```js
const stop = WXU.onFeed((feed) => {
  console.log("当前视频", feed.id);
});

// 不再需要监听时调用。
stop();
```

也可以使用 `WXU.on(event_name, handler)` 和 `WXU.off(event_name, handler)`。视频号专用事件名带有 `channels:` 前缀，例如 `WXU.Events.Feed` 的值是 `channels:Feed`。这组通用方法直接使用事件总线接口，`WXU.on` 不返回上面这样的取消函数。

监听器不会自动重放已经发生的事件，事件总线也不会等待异步回调完成。应在全局脚本加载时注册监听，并在异步回调中处理请求错误。

视频和规范化下载数据的结构见 [channels.d.ts](https://github.com/ltaoo/wx_channels_download/blob/main/pkg/scraper/wxchannels/inject/channels.d.ts)，通用日志参数见 [utils.d.ts](https://github.com/ltaoo/wx_channels_download/blob/main/frontend/inject/utils.d.ts)。这些文件没有完整声明所有事件方法，触发参数以 [channels.events.js](https://github.com/ltaoo/wx_channels_download/blob/main/pkg/scraper/wxchannels/inject/channels.events.js)、[LiteApp 运行时](https://github.com/ltaoo/wx_channels_download/blob/main/pkg/scraper/wxchannels/inject/runtime.js)和[页面事件总线](https://github.com/ltaoo/wx_channels_download/blob/main/frontend/inject/eventbus.js)的实现为准。

## 打印访问过的视频

在[工作目录](../../config/workdir.md)的 `global.js` 中注册监听。默认路径是 `workdir/global.js`，也可以通过 `inject.globalScript` 配置其他路径，相对路径按工作目录解析。创建脚本后重启下载器，并重新打开或刷新视频号页面。

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

该示例将首页或视频详情页当前视频的原始信息提交给本地服务，再由服务打印。`WXU.request` 返回 `[err, data]`，JSON 响应需要使用 `code=0` 表示成功，第二项为响应体的 `data`。只返回 `{"ok":true}` 会被当前实现判为业务错误。

以下服务处理 CORS 预检，读取并打印原始视频信息，然后返回匹配的响应结构。将代码保存为 `main.go`，执行 `go run main.go` 后再访问视频号。

```go
// main.go
package main

import (
	"io"
	"log"
	"net/http"
)

func set_cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func feed_handler(w http.ResponseWriter, r *http.Request) {
	set_cors(w)
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
	_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"received":true}}`))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/feed", feed_handler)
	addr := "127.0.0.1:1234"
	log.Printf("HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

收到的 `feed` 是原始视频对象，不能假设其顶层已经包含 `url` 和 `key`。需要规范化下载信息时，可以在页面回调中使用 `WXU.format_feed(feed)`；返回 `null` 表示不支持该数据形态。普通视频的规范化结果包含 `url` 和 `key`，图文则通过 `files` 提供图片列表。

实际下载与解密的前端实现位于 [channels.utils.js](https://github.com/ltaoo/wx_channels_download/blob/main/pkg/scraper/wxchannels/inject/channels.utils.js)，后端下载流程位于 [internal/services/download_task.go](https://github.com/ltaoo/wx_channels_download/blob/main/internal/services/download_task.go)。

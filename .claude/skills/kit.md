---
description: "Look up Timeless Kit API: RequestCore, ListCore, HistoryCore, RouteViewCore, NavigatorCore, ApplicationModel, StorageCore, HttpClientCore, and utility functions. Trigger when user asks about HTTP requests, paginated lists, routing, navigation, app lifecycle, storage, or @timeless/kit."
---

# @timeless/kit 应用工具包查阅

用户询问 kit 模块时，**读取对应子文件**后再回答。

## 子文件索引

| 用户提到 | 读取文件 |
|---------|---------|
| RequestCore, 请求, HTTP, request_factory | `.claude/skills/kit/request.md` |
| ListCore, 分页列表, loadMore, search | `.claude/skills/kit/list.md` |
| HistoryCore, 路由导航, push, back, forward | `.claude/skills/kit/history.md` |
| RouteViewCore, 路由视图, buildRoutes, RouteMenusModel | `.claude/skills/kit/route-view.md` |
| NavigatorCore, URL 解析, pathname | `.claude/skills/kit/navigator.md` |
| ApplicationModel, 应用生命周期, theme, 设备尺寸 | `.claude/skills/kit/app.md` |
| StorageCore, 存储, get/set | `.claude/skills/kit/storage.md` |
| HttpClientCore, HTTP 客户端 | `.claude/skills/kit/http-client.md` |
| MultipleSelectionCore, 多选 | `.claude/skills/kit/multiple.md` |
| @timeless/utils, debounce, throttle, diff, qs, sleep | `.claude/skills/kit/utils.md` |

## 查阅流程

1. 查上表 → 读取子文件
2. 如需完整 API → 子文件标注了源文件路径

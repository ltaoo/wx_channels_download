# HistoryCore — 应用导航栈

源文件：`packages/kit/src/history/index.ts`

## 用法

```js
const history$ = new HistoryCore({
  view: rootView,            // RouteViewCore 根视图
  router: navigator$,       // NavigatorCore 实例
  routes: routeConfig,      // Record<key, RouteConfig>
  views: viewsMap,          // Record<key, RouteViewCore>
});

// 导航
history$.push("root.home_layout.index.form");
history$.push("root.detail", { id: "123" });
history$.replace("root.login");
history$.back();
history$.forward();
history$.reload();
history$.destroyAllAndPush("root.login");   // 清空栈并跳转

// URL 构建
history$.buildURL("root.detail", { id: "123" });
history$.buildURLWithPrefix("root.detail", { id: "123" });
```

## Core API

```ts
new HistoryCore({ view, router, routes, views, virtual? })

history$.stacks          // RouteViewCore[] 导航栈
history$.cursor          // 当前位置索引
history$.$router         // NavigatorCore
history$.$view           // RouteViewCore 根视图

history$.push(name, query?, options?)
history$.replace(name, query?)
history$.back(opt?)
history$.forward()
history$.reload()
history$.destroyAllAndPush(name, query?, options?)
history$.isRoot(name) / history$.isLayout(name)

// 事件
history$.onRouteChange(fn)
history$.onTopViewChange(fn)
history$.onBack(fn) / history$.onForward(fn)
history$.onClickLink(fn)
history$.onStateChange(fn)
```

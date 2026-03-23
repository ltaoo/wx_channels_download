# NavigatorCore — URL/pathname 管理

源文件：`packages/kit/src/navigator/index.ts`

## 用法

```js
const nav$ = new NavigatorCore();

nav$.prepare({ pathname: "/home", href: "http://localhost/home" });
nav$.start();

// 编程式导航
nav$.pushState("/home/detail?id=1");
nav$.replaceState("/login");
```

## Core API

```ts
NavigatorCore.prefix     // 静态：URL 前缀
NavigatorCore.parse(url) // 静态：解析 URL

nav$.pathname / nav$.query / nav$.href / nav$.origin / nav$.host
nav$.histories / nav$.prevPathname

nav$.prepare(location)
nav$.start()
nav$.pushState(url)
nav$.replaceState(url)
nav$.handlePopState({ type, pathname, href })

// 事件
nav$.onStart(fn)
nav$.onPushState(fn) / nav$.onReplaceState(fn) / nav$.onPopState(fn)
nav$.onPathnameChange(fn)
nav$.onBack(fn) / nav$.onForward(fn) / nav$.onReload(fn)
nav$.onHistoriesChange(fn)
```

## RouteAction 类型

```ts
type RouteAction = "initialize" | "push" | "replace" | "back" | "forward";
```

# ApplicationModel — 应用生命周期

源文件：`packages/kit/src/app/index.ts`
类型：`packages/kit/src/app/types.ts`
工具：`packages/kit/src/app/utils.ts`

## 用法

```js
const app$ = new ApplicationModel({
  storage: storageCore,
  clipboard: clipboardModel,
  async beforeReady() { /* 初始化 */ },
  onReady() { /* 就绪后 */ },
});

app$.start({ width: window.innerWidth });
app$.setTheme("dark");      // "dark" | "light" | "system"
app$.tip({ text: ["操作成功"] });
app$.loading({ text: "加载中..." });
app$.hideLoading();
app$.copy("复制的文字");
```

## Core API

```ts
new ApplicationModel({ storage, clipboard, beforeReady?, onReady? })

app$.screen        // { width, height, statusBarHeight, menuButton }
app$.env           // { wechat, ios, android, pc, weapp, prod }
app$.curDeviceSize // "sm" | "md" | "lg" | "xl" | "2xl"
app$.theme         // "dark" | "light" | "system"

app$.start(size)
app$.setTheme(theme)
app$.setSize(size)
app$.setTitle(title)
app$.openWindow(url)
app$.setEnv(env)
app$.tip(arg) / app$.loading(arg) / app$.hideLoading()
app$.copy(text)
app$.vibrate()
app$.disablePointer() / app$.enablePointer()
app$.keydown(event) / app$.escape()

// 事件
app$.onReady(fn) / app$.onDeviceSizeChange(fn)
app$.onTip(fn) / app$.onLoading(fn) / app$.onHideLoading(fn)
app$.onError(fn) / app$.onStateChange(fn)
app$.onKeydown(fn) / app$.onKeyup(fn) / app$.onEscapeKeyDown(fn)
app$.onResize(fn) / app$.onOrientationChange(fn)
app$.onShow(fn) / app$.onHidden(fn) / app$.onBlur(fn)
```

## 设备尺寸断点

```ts
sm: 0, md: 768, lg: 992, xl: 1200, 2xl: 1536
getCurrentDeviceSize(width) → "sm" | "md" | "lg" | "xl" | "2xl"
```

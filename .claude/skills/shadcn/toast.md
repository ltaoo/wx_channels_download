# Toast

源文件：`packages/shadcn/src/toast.ts` | Core：`packages/ui/src/toast/index.ts`

## 用法

```js
const toast$ = new Timeless.ui.ToastCore({ delay: 2000 });

// 挂载（一般在页面顶层挂载一次）
Toast({ store: toast$ });

// 显示
toast$.show({ texts: ["操作成功！"] });
toast$.show({ texts: ["加载中..."], icon: "loading", mask: true });
toast$.show({ texts: ["第一行", "第二行"] });

// 隐藏
toast$.hide();
```

## Core API

```ts
new ToastCore({ _name?, delay?: number })  // delay: 自动隐藏毫秒数，默认 2000

toast$.state → { mask, icon, texts, enter, visible, exit }

toast$.show({ mask?: boolean, icon?: unknown, texts: string[] })
toast$.hide()
toast$.clearTimer()

toast$.onShow(fn) / toast$.onHide(fn) / toast$.onStateChange(fn)
```

## 注意

- Toast 居中显示（fixed + translate），带遮罩时阻止背景交互
- icon 为 `"loading"` 时显示旋转动画
- delay 为自动关闭时间，调用 show 时重置计时器

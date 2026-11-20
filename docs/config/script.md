---
title: 用户脚本
---

# 用户脚本

目前有两种方式在视频号页面注入额外的 `js` 脚本

## 全局脚本

在[工作目录](./workdir.md) `workdir` 下，如果存在 `global.js`，则会将其插入视频号页面。目前可以通过其指定下载时的文件名称。

也可以通过配置指定全局脚本路径；相对路径会按 `workdir` 解析：

```yaml
inject:
  globalScript: "global.js"
```

### 脚本示例

```js
// global.js
function beforeFilename(filename, params) {
  return filename;
}
```

## LiteApp 页面注入（实验）

部分新版视频号从本地 LiteApp 包加载，打开视频时不会请求 H5 播放页。这种情况下，常规 HTML 注入不会触发。

代理在视频号加载远程 libpag SDK 时保留原 SDK，并追加只在目标 LiteApp 地址运行的业务加载器。加载器取得页面现有的 Webpack API 模块，将源码交给项目拦截器，复用 H5 的业务方法重写规则；页面再加载改写后的真实 JavaScript 文件，把改写方法安装到原 API 类的原型上。已有 API 实例、Vue 和播放器实例保留，不重跑完整 feed bundle，也不需要关闭 LiteApp。

改写文件使用 `/__assets/wxchannels/liteapp/modules/<sha256>.js?v=<sha256>`。哈希包含改写后的内容，业务源码或改写规则变化后 URL 变化；SDK、业务模块及项目注入资源均返回 `Cache-Control: no-store`，并清除条件缓存。加载器同时复用现有项目事件总线、工具及详情页脚本；配置了 `inject.globalScript` 时也加载用户脚本。每个页面安装一次，修改后需要重新打开页面。

当前适配的是导出 API 类的原生详情模块，含初始化、详情、评论等能匹配共享规则的方法。其他原生模块中的首页切换、直播等行为尚未全部适配。加载后会补做一次初始化；能从路由或现有 feed store 取得详情参数时，再经改写后的详情方法补取数据，避免 SDK 晚于首屏请求导致项目漏收事件。

入口匹配 `/t/wx_fed/cdn_libs/res/libpag/<版本>/libpag.min.js`，也支持非压缩文件名。如果该 SDK 已有长期浏览器缓存，首次验证需要让它重新请求；`no-store` 无法使此前已缓存的 SDK 主动发出请求。此方案依赖微信继续加载该远程 SDK，以及当前 Webpack/API 类结构。

诊断地址为 `https://res.wx.qq.com/__assets/wxchannels/liteapp/status`，字段包含阶段、内容版本、替换方法数、实际初始化/详情事件计数及 Vue 实例是否保留；不包含视频详情或认证参数。`installed` 表示方法已替换，实际事件计数才说明业务方法执行；黄色标签只说明用户脚本执行。

可用 `scripts/wxchannels-liteapp-extra.js` 作为独立标记示例，示例回执依赖本机专用转发规则。此前已在 macOS 正式微信验证该远程入口可执行标记且视频正常播放；新增业务重写已通过 Go、Node 与真实已下载模块的离线检查，正式微信内的业务事件与播放结果仍需重新验证。

本机实验依赖 EAPI 8899 与转发服务 18998，新增 `/__assets/*` 转发以服务改写模块和项目脚本。现有已启动的旧版下载器不会自动获得源码改动。Windows 注入与未来微信版本尚未验证。

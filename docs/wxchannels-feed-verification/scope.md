# 视频号 feed.js 验证范围

- 授权来源：本次会话中用户要求本机 `feed.js` 修改生效，并手动打开、关闭视频号进行验证。
- 目标：本机视频号 LiteApp `wxalite28e098851253e34f4777f7ca024236fd`，资源版本 226；早期阶段使用 WechatNode 管理的副本，最终 MITM 验证使用正式 `/Applications/WeChat.app`。
- 验证内容：独立新增 JavaScript 执行、页面标记、视频播放、EAPI 视频 CDN 路由，以及正式版原生 LiteApp 的远程 SDK 追加脚本。
- 网络范围：本机 PageSpy `127.0.0.1:6752`、EAPI `127.0.0.1:8899`，视频 CDN `finder.video.qq.com`。
- 用户约束：不修改微信原始应用二进制；用户此前要求考虑 MITM 与 JavaScript 修改。当前 PageSpy 运行时注入单独记录，不视为纯 MITM 替换成功。
- 排除：任意修改原有 bundle 内部逻辑、替换签名信任链、内存 Hook、跨版本适配及 Windows 注入实测。

后续阶段：用户再次明确限定 MITM 与 JavaScript 文件修改，不接受内存 Hook。正式版 LiteApp 的远程 SDK 响应追加脚本、SDK 单项缓存备份移动及恢复 `handleFinder=true` 为当前验证范围；正式版页面执行和正常播放已由独立 H 回执与用户确认验证，见同目录 `native-mitm-success.json`，不沿用副本的 G 结果。

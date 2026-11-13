---
title: 下载配置
---

# 下载配置

通过 `config.yaml` 控制下载行为。

视频号专属的下载行为，请参考[视频号配置](./channels/index.md)。

## 下载时的文件名称

```yaml
download:
  filenameTemplate: "{{filename}}_{{spec}}"
```

下载时的文件名称，默认「文件名\_视频质量」。`.mp4` 后缀由下载器添加，不支持手动设置后缀

目前支持如下变量

```js
type params = {
  /** 默认文件名，优先取 title，没有则取视频 id，仍没有则使用 当前时间秒数 */
  filename: string,
  /** 视频 id */
  id: string,
  /** 视频标题 */
  title: string,
  /** 视频质量 original | 'xWT111' */
  spec: string,
  /** 视频发布时间（单位秒） */
  created_at: number,
  /** 视频下载时间（单位秒） */
  download_at: number,
  /** up主名称 */
  author: string,
};
```

如果存在 `/` 符号，例如 <code v-pre>{{author}}/{{filename}}\_{{spec}}</code>，这样下载的文件会放在以作者名为目录的目录中，该能力仅在后台下载时生效

## 下载目录

```yaml
download:
  dir: "%UserDownloads%"
```

下载文件存放的目录，默认为用户下载目录。可以修改为任意「绝对路径目录」。仅在后台下载时生效

分平台配置示例和生效范围说明见[设置下载目录](../feature/download-directory.md)。

## 完成音效

```yaml
download:
  playDoneAudio: true
```

后台下载模式下，下载完成后是否播放完成音效，默认 `true`。

## 最大并发任务数

```yaml
download:
  maxRunning: 3
```

同时运行的下载任务数量上限，默认 `3`。

## Default action when a task exists

```yaml
download:
  defaultActionWhenExisting: ""
```

When creating a download task, an existing task normally returns a conflict so the caller can choose an action. Set this to `skip`, `duplicate`, or `overwrite` to skip the incoming task, create another copy, or overwrite the existing task.

## 远程服务

```yaml
download:
  remoteServer:
    enabled: false
    protocol: "http"
    hostname: "192.168.1.118"
    port: 2022
```

是否启用远程服务下载，默认 `false`。开启后，视频号注入的下载面板通过 `https://weixin110.qq.com` 和 `wss://weixin110.qq.com` 请求服务，由本地拦截器转发到此处配置的远端服务；预览和打开下载管理页也使用此转发入口。浏览器中的该域名是代理入口，实际下载服务仍是 `remoteServer` 指定的地址。

此转发方式用于避免视频号 HTTPS 页面直连远端 HTTP/WS 时的混合内容限制，需要微信流量经过本地代理，且浏览器信任代理证书。

启动日志会显示远端 API/MCP 地址。本地 API 和代理仍会启动；代理日志中的监听地址用于本机流量接入，`remote server` 表示使用的远端服务。

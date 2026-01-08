---
title: 下载配置
---

# 下载配置

通过 `config.yaml` 控制下载行为。

## 默认下载原始视频

```yaml
download:
  defaultHighest: false
```

点击下载图标按钮时，是否下载原始视频，默认 `false`，下载的是最低画质，也是视频号默认播放的视频画质，兼顾了体积和画面

## 下载时的文件名称

```yaml
download:
  filenameTemplate: "{{filename}}_{{spec}}"
```

下载时的文件名称，默认「文件名_视频质量」。`.mp4` 后缀由下载器添加，不支持手动设置后缀

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

如果存在 `/` 符号，例如 <code v-pre>{{author}}/{{filename}}_{{spec}}</code>，这样下载的文件会放在以作者名为目录的目录中，该能力仅在后台下载时生效

## 下载目录

```yaml
download:
  dir: "%UserDownloads%"
```

指定下载目录，默认用户下载目录。仅在后台下载时生效

## 前端下载

```yaml
download:
  frontend: false
```

是否使用前端下载，默认 `false`

设置为 `true` 后，会在前端进行下载、解密，下载长视频大概率会有问题。并且「下载目录」配置会失效、下载文件名不支持 `/` 字符来创建目录。

## 下载视频时暂停视频播放

```yaml
download:
  pauseVideoWhenDownload: false
```

是否在前端下载时暂停视频播放，默认 `false`。下载完成后会继续播放。


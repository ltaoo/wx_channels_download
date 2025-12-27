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

`false` 表示「否」

## 下载时的文件名称

```yaml
download:
  filenameTemplate: "{{filename}}_{{spec}}"
```

`filenameTemplate` 通过模板语法指定下载时的文件名称，默认「文件名+视频质量」

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

如果存在 `/` 符号，例如 `{{author}}/{{filename}}_{{spec}}`，这样下载的文件会放在以作者名为目录的目录中

## 下载目录

```yaml
download:
  dir: ./downloads
```

指定下载目录，默认当前目录 `./downloads`

## 前端下载

```yaml
download:
  frontend: false
```

开启后将恢复旧版的下载行为

## 是否在下载视频时暂停视频播放

```yaml
download:
  pauseVideoWhenDownload: false
```

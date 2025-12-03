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
  filename: string;
  /** 视频 id */
  id: string;
  /** 视频标题 */
  title: string;
  /** 视频质量 original | 'xWT111' */
  spec: string;
  /** 视频发布时间（单位秒） */
  created_at: number;
  /** 视频下载时间（单位秒） */
  download_at: number;
  /** up主名称 */
  author: string;
};
```

## 本地下载中转服务

```yaml
download:
  localServer:
    enabled: false
    addr: "127.0.0.1:8080"
```

开启 `localServer` 后，将通过本地服务进行下载，从而实现

1、在页面下载长视频不阻塞操作
<br />
2、将视频转换成 `mp3` 并下载

## 是否在下载视频时暂停视频播放

```yaml
download:
  pauseVideoWhenDownload: false
```


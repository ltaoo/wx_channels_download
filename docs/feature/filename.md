---
title: 指定下载的文件名
---

# 指定下载的文件名

现在有两种方式可以配置下载文件时的文件名

## 通过配置文件

- [下载配置](../config/download.md#下载时的文件名称)


## 通过全局脚本

- [全局脚本](../config/script.md#全局脚本)

### 在下载文件名称中增加视频发布时间

```js
function secondsToYMD(seconds, startTimestamp = 0) {
  const date = new Date((startTimestamp + seconds) * 1000);

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");

  return `${year}${month}${day}`;
}

function beforeFilename(filename, params) {
  const t = secondsToYMD(params.created_at);
  return [params.author, params.title, t, params.spec].filter(Boolean).join("_");
}
```

下载的文件名，就是 `作者名称_标题_20260710_规格.mp4`

其中 `params` 类型为

```ts
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


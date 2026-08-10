---
title: 指定下载的文件名
---

# 指定下载的文件名

建议通过 `hooks.js` 配置下载文件的文件名。

## 通过配置文件（已废弃）

此方式已废弃，请改用 `hooks.js`。旧版配置方式可参考[下载配置](../config/download.md#下载时的文件名称)。

## 通过 hooks.js

不建议通过全局脚本修改文件名，请在工作目录 `workdir` 下创建 `hooks.js`，并定义 `onFilename`：

```js
function onFilename(meta) {
  return {
    directories: [meta.author],
    name: [meta.title, meta.spec, meta.idx].filter(Boolean).join("_"),
  };
}
```

`directories` 是相对于下载目录的子目录列表，`name` 是不带扩展名的文件名；文件扩展名由下载器自动添加。以上示例会按作者创建子目录，并生成 `标题_规格_序号.mp4`。

`meta.idx` 建议保留吧


### 在下载文件名称中增加视频发布时间

```js
function secondsToYMD(seconds, startTimestamp = 0) {
  const date = new Date((startTimestamp + seconds) * 1000);

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");

  return `${year}${month}${day}`;
}

function onFilename(meta) {
  const t = secondsToYMD(meta.created_at);
  return {
    directories: [],
    name: [meta.author, meta.title, t, meta.spec, meta.idx].filter(Boolean).join("_"),
  };
}
```

下载的文件名，就是 `作者名称_标题_20260710_规格.mp4`

其中 `meta` 类型为

```ts
type Meta = {
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

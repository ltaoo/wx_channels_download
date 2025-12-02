---
title: 用户脚本
---

# 用户脚本

目前有两种方式在视频号页面注入额外的 `js` 脚本

## 全局脚本

在和 `wx_video_download.exe` 同级目录，如果存在 `global.js`，则会将其插入视频号页面。目前可以通过其指定下载时的文件名称

```js
// global.js
function beforeFilename(filename, params) {
  return filename;
}
```


## 主脚本之后插入的脚本

```yaml
inject:
  extraScript:
    afterJSMain: "./extra.js"
```

可以用来自定义额外功能

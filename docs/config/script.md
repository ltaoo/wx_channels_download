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

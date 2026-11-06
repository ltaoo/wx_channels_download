---
title: 默认下载原始视频
---

# 默认下载原始视频

```yaml
channels:
  download:
    defaultHighest: false
```

点击下载按钮时是否默认选择最高画质，默认 `false`。设置为 `true` 后，若视频规格提供了有效的 `levelOrder`，将选择等级最高的规格（较小的正数代表较高等级；相同等级时再比较码率）；若没有等级信息，则仍请求原始视频。

---
description: "Look up Timeless UI Core class API, state machines, event system, and shared infrastructure. Trigger when user asks about a Core class (ButtonCore, SelectCore...), BaseDomain, PopperCore, PresenceCore, DismissableLayerCore, form validation, or @timeless/ui internals."
---

# @timeless/ui Core 逻辑层查阅

用户询问 Core 类或底层架构时，**读取对应子文件**后再回答。

## 子文件索引

| 用户提到 | 读取文件 |
|---------|---------|
| BaseDomain, base(), 事件系统, Result, BizError | `.claude/skills/ui/base.md` |
| PresenceCore, PopperCore, DismissableLayerCore, LayerManager | `.claude/skills/ui/infra.md` |
| SingleFieldCore, ObjectFieldCore, ArrayFieldCore, 表单验证, FieldRule | `.claude/skills/ui/form.md` |
| 某个具体 Core 类的完整 API | `.claude/skills/ui/core-index.md`（查源文件路径后直接读源文件） |

## 查阅流程

1. 查上表 → 读取子文件
2. 如需某个 Core 的完整 API → 读取 `ui/core-index.md` 找到源文件路径 → 读取源文件
3. 如涉及 shadcn 层用法 → 配合 `shadcn-components` skill 查阅

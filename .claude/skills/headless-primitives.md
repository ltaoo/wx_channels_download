---
description: "Look up Timeless Headless Primitive API, View system, control flow, and rendering architecture. Trigger when user asks about Primitives (SelectPrimitive, DialogPrimitive...), View/TimelessElement, Show/For/Switch, Portal/Presence, or headless rendering."
---

# @timeless/headless 无头渲染层查阅

用户询问 Primitive 或渲染机制时，**读取对应子文件**后再回答。

## 子文件索引

| 用户提到 | 读取文件 |
|---------|---------|
| View, ViewProps, ViewChildren, TimelessElement, 生命周期 | `.claude/skills/headless/view.md` |
| Show, For, Switch, Match, 控制流 | `.claude/skills/headless/control-flow.md` |
| Primitive 模式, XxxPrimitive 的用法和结构 | `.claude/skills/headless/primitives.md` |
| Portal, Presence, Popper, Transition, 底层原语 | `.claude/skills/headless/infra.md` |

## 查阅流程

1. 查上表 → 读取子文件
2. 如需具体某个 Primitive 的完整子元素列表 → 读取 `packages/headless/src/<component>.ts`
3. 如涉及响应式系统 → 参考 `@timeless/reactive`（headless 直接重导出所有 reactive 原语）

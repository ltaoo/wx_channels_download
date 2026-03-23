---
description: "Look up Timeless ShadcnUI component usage, API, and best practices. Trigger when user asks about a specific component (Button, Select, Dialog, ContextMenu, etc.) or needs to use/modify a shadcn component."
---

# @timeless/shadcn 组件查阅

用户询问组件时，**读取对应子文件**后再回答。不要凭记忆回答。

## 子文件索引

| 用户提到 | 读取文件 |
|---------|---------|
| Button | `.claude/skills/shadcn/button.md` |
| Input, Textarea, NumberInput | `.claude/skills/shadcn/input.md` |
| Select | `.claude/skills/shadcn/select.md` |
| Cascader | `.claude/skills/shadcn/cascader.md` |
| Checkbox, CheckboxGroup, RadioGroup, Toggle | `.claude/skills/shadcn/checkbox-radio.md` |
| Dialog, Sheet | `.claude/skills/shadcn/dialog-sheet.md` |
| Popover, Popconfirm, Tooltip | `.claude/skills/shadcn/popover-tooltip.md` |
| DropdownMenu, ContextMenu, Menu | `.claude/skills/shadcn/menu.md` |
| Tabs, Accordion, Steps | `.claude/skills/shadcn/tabs-accordion.md` |
| Toast | `.claude/skills/shadcn/toast.md` |
| DatePicker, DateRangePicker, TimePicker | `.claude/skills/shadcn/date-time.md` |
| Field, Form, 表单验证 | `.claude/skills/shadcn/form.md` |
| Slider | `.claude/skills/shadcn/slider.md` |
| Card, Table, Badge, Alert, Separator, Avatar, Skeleton, Progress, ScrollArea, AspectRatio | `.claude/skills/shadcn/display.md` |
| ResizablePanels | `.claude/skills/shadcn/resizable-panels.md` |

## 查阅流程

1. 查上表 → 读取子文件
2. 如子文件不够详细 → 读取子文件中标注的源文件路径
3. 需要实际页面用法 → 读取 `apps/web-vanilla/src/pages/` 下的页面文件

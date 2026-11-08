---
title: 创作者直播录制
description: 复制 Pipeline JSON，定时检查创作者直播并创建录制任务。
---

# 创作者直播录制

查询创作者是否正在直播，开播时创建录制任务，未开播时正常结束。

点击下方代码块右上角的复制按钮，将完整 JSON 粘贴到「自动化 → 导入 Pipeline」。导入后在「配置」节点填写 `creator_username`，并在开始节点添加定时触发器，建议 Cron 为 `*/1 * * * *`。

更多说明见[导入和配置](./index.md#导入和配置)与[运行结果与边界](./index.md#运行结果与边界)。

<<< ./02_creator_live_record.json

---
title: MCP
---

# MCP

MCP（Model Context Protocol）让支持 MCP 的 AI 客户端以工具调用的方式访问下载器。当前实现提供应用配置管理、内容解析与下载、微信视频号查询，以及下载任务、账号、浏览记录、日志和证书状态等本地数据查询能力。

下载器支持两种接入方式：

- **Streamable HTTP（推荐）**：使用主服务的 `/mcp` 端点。内置 MCP 直接访问当前进程中的数据库和运行时服务。
- **stdio**：由 AI 客户端启动 `wx_video_download mcp` 子进程，再通过已经运行的下载器 API 完成操作。

两种接入方式提供相同的 MCP 工具。需要更完整的 stdio 配置与视频号外部下载说明，可参阅 [MCP Server 命令行文档](/cli/mcp)。

## 文档

- [启用](./mcp/enable.md)
- [命令](./mcp/commands.md)

### 常用场景

- [修改下载目录](./mcp/scenarios/change-download-directory.md)
- [解析链接并下载](./mcp/scenarios/fetch-and-download.md)
- [查询账号内容和浏览记录](./mcp/scenarios/query-account-content.md)
- [排查下载失败](./mcp/scenarios/troubleshoot-download.md)
- [使用第三方下载器保存视频号视频](./mcp/scenarios/external-downloader.md)

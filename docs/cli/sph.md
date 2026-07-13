---
title: 视频号查询部署
---

# sph_deploy

将视频号视频信息查询页面部署到 Cloudflare Worker，提供 Web 界面查询视频号视频下载地址。

## 用法

```sh
wx_video_download sph_deploy
```

## 前置条件

- 拥有 Cloudflare 账号，并获取 Account ID 和 API Token
- API Token 需要有 Workers 相关的读写权限

## 配置

在配置文件中添加以下字段：

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
  sphWorkerName: "worker-name"
  sphCookie: "元宝 web 端 cookie"
```

| 配置项 | 说明 |
|--------|------|
| `cloudflare.accountId` | Cloudflare 账户 ID，可在 Workers 页面找到 |
| `cloudflare.apiToken` | Cloudflare API Token，需 Workers 读写权限 |
| `cloudflare.sphWorkerName` | Worker 名称，部署后标识该 Worker |
| `cloudflare.sphCookie` | 视频号接口所需的元宝 Web 端 Cookie |

元宝Web端 指 https://yuanbao.tencent.com/ 网站，登录后，获取 `cookie` 作为配置即可

## 说明

- 部署内容为 `internal/api/sph/` 目录下的 `worker.js` 和 `index.html`
- Worker 运行在 Cloudflare 边缘节点，无需自有服务器
- Cookie 以环境变量的方式注入 Worker，用于调用视频号接口时的身份认证

## 可用 API

| Method | Path | 说明 |
|--------|------|------|
| GET | `/` | 视频号视频信息查询页面 |
| POST | `/api/fetch_video_profile` | 通过分享链接获取视频号视频信息 |

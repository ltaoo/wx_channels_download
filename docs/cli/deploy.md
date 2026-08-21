---
title: 部署命令
---

# deploy

部署 Cloudflare Worker。该命令是父命令，需要指定二级命令。

## 用法

```sh
wx_video_download deploy <command>
```

## 前置条件

- 拥有 Cloudflare 账号，并获取 Account ID 和 API Token。
- API Token 需要有 Workers 相关的读写权限。
- 部署公众号 Worker 时，还需要 D1 相关权限。

## 二级命令

| 命令 | 说明 |
|------|------|
| `wx_video_download deploy mp` | 部署公众号 RSS/API Worker |
| `wx_video_download deploy sph` | 部署视频号查询 Worker |
| `wx_video_download deploy bridge` | 部署 Bridge 桥接/转发 Worker 和 Pages 管理页面 |

## 通用配置

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
```

| 配置项 | 说明 |
|--------|------|
| `cloudflare.accountId` | Cloudflare 账户 ID，可在 Workers 页面找到 |
| `cloudflare.apiToken` | Cloudflare API Token，需 Workers 读写权限 |

## deploy bridge

一次部署用于桥接/转发远程调用的 Durable Objects Bridge Worker 和受密码保护的 Pages 管理页面。

部署完成后的设备注册、调用 Token 创建、API 协议和多语言示例请参阅 [Bridge 使用](/feature/bridge)。

```sh
wx_video_download deploy bridge
```

### 配置

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"

bridge:
  deploy:
    workerName: "dm-bridge"
    pagesProjectName: "" # 留空时使用 dm-bridge-admin
    token: "bridge-client-token"
    adminToken: "bridge-admin-token"
```

API Token 需要 Workers Scripts:Edit 和 Pages:Edit 权限。`token` 与 `adminToken` 必须使用不同的高强度随机值。

部署命令会依次执行：

1. 上传 Worker，并配置 Durable Objects、`BRIDGE_TOKEN` 和 `BRIDGE_ADMIN_TOKEN`；
2. 创建或更新 Pages 项目；
3. 为 Pages 配置 `BRIDGE_ADMIN_TOKEN` Secret 和指向 Worker 的 `BRIDGE` Service Binding；
4. 上传管理页面静态资源和高级模式 Worker。

任一步骤失败都会停止后续部署；如果 Pages 阶段失败，命令会明确提示 Worker 已经部署成功，不会回滚 Worker。

## deploy mp

部署公众号 RSS/API 相关的 Cloudflare Worker。

```sh
wx_video_download deploy mp
```

### 配置

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
  workerName: "mp-rss-api"
  d1Name: "mp-rss-db"
  refreshToken: "refresh-token"
  adminToken: "admin-token"

mp:
  remoteServer:
    hostname: "your-remote-server-hostname"
```

| 配置项 | 说明 |
|--------|------|
| `cloudflare.workerName` | 公众号 Worker 名称 |
| `cloudflare.d1Name` | D1 数据库名称，命令会按名称查找，不存在时会创建 |
| `cloudflare.d1Id` | D1 数据库 ID；未配置 `d1Name` 时可直接指定 |
| `cloudflare.refreshToken` | 刷新公众号授权凭证接口所需 Token |
| `cloudflare.adminToken` | 管理员接口所需 Token |
| `mp.remoteServer.hostname` | 注入到 Worker 的远端服务地址 |

### 部署内容

- 部署嵌入 `internal/workers/mp/index.js` 的 Worker。
- 执行嵌入 `internal/workers/mp/migrations/` 的 D1 迁移。
- 注入 D1 绑定 `DB`，以及 `ADMIN_TOKEN`、`REFRESH_TOKEN`、`REMOTE_SERVER` 环境变量。

### 可用 API

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/mp/list` | 获取公众号列表 |
| GET | `/api/mp/msg/list` | 获取公众号消息列表 |
| POST | `/api/mp/refresh` | 刷新/同步公众号信息 |
| POST | `/admin/token/add` | 添加访问 Token |
| POST | `/admin/token/delete` | 删除访问 Token |
| GET | `/rss/mp` | RSS 订阅地址 |

## deploy sph

部署视频号视频信息查询页面到 Cloudflare Worker，提供 Web 界面查询视频号视频下载地址。

!!不要提供给外部使用，仅自己使用即可，有被限制使用 yuanbao 的风险!!

```sh
wx_video_download deploy sph
```

### 配置

在配置文件中添加或修改以下字段：

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
  sphWorkerName: "worker-name"
  sphCookie: "元宝 web 端 cookie"
  sphCredential: "页面及 API 的访问凭证"
```

| 配置项 | 说明 |
|--------|------|
| `cloudflare.accountId` | Cloudflare 账户 ID，可在 Workers 页面找到 |
| `cloudflare.apiToken` | Cloudflare API Token，需 Workers 读写权限 |
| `cloudflare.sphWorkerName` | 视频号查询 Worker 名称，部署后标识该 Worker |
| `cloudflare.sphCookie` | 视频号接口所需的元宝 Web 端 Cookie |
| `cloudflare.sphCredential` | 访问页面和 API 所需的凭证，部署时注入 `ACCESS_CREDENTIAL` 环境变量 |

元宝 Web 端指 https://yuanbao.tencent.com/ 网站，登录后获取 `cookie` 作为配置即可。
元宝 Web 端 Cookie 有效期约 1 个月，失效后重新登录获取新的 Cookie，并到 Cloudflare Worker 更新 `COOKIE` 环境变量即可。

!!不要提供给外部使用，仅自己使用即可，有被限制使用 yuanbao 的风险!!

### 部署内容

- 部署嵌入 `internal/workers/sph/worker.js` 的 Worker。
- 同时上传嵌入的 `internal/workers/sph/index.html` 和 `build/icon.png`。
- Cookie 以 `COOKIE` 环境变量注入 Worker，用于调用视频号接口时的身份认证。
- 访问凭证以 `ACCESS_CREDENTIAL` 环境变量注入 Worker；未配置时部署命令会拒绝部署。

### 访问认证

浏览器访问 Worker 时会显示 HTTP Basic Auth 登录框。用户名固定为 `wxchannels`，密码填写 `cloudflare.sphCredential`。登录后，页面对同源 API 的请求会自动携带认证信息。

直接调用 API 时可使用 Bearer Token：

```sh
curl -H 'Authorization: Bearer <sphCredential>' \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://weixin.qq.com/sph/example"}' \
  'https://<worker-name>.<subdomain>.workers.dev/api/fetch_video_profile'
```

也可使用 Basic Auth：`curl -u 'wxchannels:<sphCredential>' ...`。

### 可用 API

| Method | Path | 说明 |
|--------|------|------|
| GET | `/` | 视频号视频信息查询页面 |
| GET | `/favicon.ico` | 页面图标 |
| GET | `/icon.png` | 页面图标 |
| POST | `/api/fetch_video_profile` | 通过分享链接获取视频号视频信息 |
| POST | `/api/download_feed_zip` | 将图集及 BGM 打包为 ZIP |

除 CORS 预检请求外，以上页面、静态资源和 API 均需认证。认证失败返回 `401`；Worker 未配置 `ACCESS_CREDENTIAL` 时返回 `503`。

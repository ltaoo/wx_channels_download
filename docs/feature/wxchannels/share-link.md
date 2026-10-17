---
title: 分享链接解析
---

# 分享链接解析

支持通过视频号分享链接获取视频信息并下载，例如 `https://weixin.qq.com/sph/xxx`。可以在本机下载器中使用，也可以部署独立的 Cloudflare Worker 查询页面。

本文补充了 [issue #583](https://github.com/ltaoo/wx_channels_download/issues/583) 中提到的元宝 Cookie 获取、配置和部署步骤。

## 在本机下载器中使用

1. 按照[下载并运行](../../guide/start.md)启动下载器。
2. 打开下载器的「内容抓取」页面，默认地址为 <http://127.0.0.1:2022/scraper>。如果修改过 API 地址或端口，请使用自己的配置。
3. 粘贴完整的视频号分享链接，提交解析，确认结果后下载。

未配置元宝 Cookie 时，下载器会尝试通过已连接的视频号页面解析；请保持下载器运行，并在微信中打开视频号页面。该方式无需部署 Cloudflare Worker。

如果希望使用元宝接口解析，可以按下一节获取 Cookie，写入本机使用的 `config.yaml`，保存后重启下载器：

```yaml
cloudflare:
  sphCookie: "hy_token=替换为你的实际值"
```

仅在本机使用元宝解析时，无需填写 Cloudflare 账号、API Token 或 Worker 访问凭证。下载器会优先读取 `cookies.json` 中匹配 `.tencent.com` 域的 Cookie，未找到时使用 `cloudflare.sphCookie`。

## 获取元宝 Cookie {#cookie}

这里的 Cookie 来自[腾讯元宝网页版](https://yuanbao.tencent.com/)，需要先登录元宝账号。

根据 [issue #583 的说明](https://github.com/ltaoo/wx_channels_download/issues/583#issuecomment-6207966324)，实际有用的字段是 **`hy_token`**。可以复制元宝的完整 Cookie，不需要逐个筛选 `_ga` 等字段。

### 方法一：使用插件复制

1. 按照 [dm-extension](https://github.com/ltaoo/dm-extension) 项目说明安装插件。
2. 打开 <https://yuanbao.tencent.com/> 并完成登录，停留在元宝页面。
3. 打开插件面板，选中需要复制的元宝 Cookie，点击「复制已选」。
4. 将复制的内容粘贴到 `config.yaml` 的 `cloudflare.sphCookie` 中。

完整 Cookie 的配置格式如下，其中所有值都需要替换为你自己的值：

```yaml
cloudflare:
  sphCookie: "_ga=xxx; _ga_6P1G7NCG3R=xxx; hy_token=xxx"
```

### 方法二：使用开发者工具复制

1. 登录元宝网页版后，打开浏览器开发者工具。
2. 切换到「应用 / Application」→「存储 / Storage」→「Cookie / Cookies」，选择元宝站点。
3. 找到名称为 `hy_token` 的 Cookie，复制它的值，按 `hy_token=实际值` 的形式填写 `sphCookie`。

也可以在「网络 / Network」中找到发送到 `yuanbao.tencent.com` 且带有 Cookie 的请求，复制「请求标头 / Request Headers」里的完整 `Cookie` 值。

填写时注意：

- 内容应为 `名称=值; 名称=值`，不要带上 `Cookie:` 前缀，也不要粘贴 JSON、整个请求或 `Set-Cookie` 响应头。
- 如果只复制 `hy_token` 的值，记得在前面加上 `hy_token=`。
- 已有 `cloudflare:` 配置时，在同一个配置块内补充字段，不要重复添加顶层 `cloudflare:`。
- Cookie 属于登录凭证，请勿提交到仓库或公开粘贴到 issue 中。

## 部署独立查询页面

部署后可以直接访问 Worker 页面，输入分享链接查询并下载，无需保持本机下载器或微信运行。部署命令、API 和认证方式的完整说明见 [deploy sph](../../cli/deploy.md#deploy-sph)。

::: warning 仅供自己使用
元宝接口可能限制账号使用，请勿向大量用户共享服务或 Cookie。
:::

### 1. 准备 Cloudflare 配置

准备 Cloudflare 账户 ID 和具有 `Workers Scripts:Edit` 权限的 API Token。视频号 Worker 不需要公众号 RSS 服务使用的 D1 数据库配置。

在下载器使用的 `config.yaml` 中填写：

```yaml
cloudflare:
  accountId: "your-cloudflare-account-id"
  apiToken: "your-cloudflare-api-token"
  sphWorkerName: "sph"
  sphCookie: "hy_token=替换为你的实际值"
  sphCredential: "替换为你自己设置的访问密码"
```

| 配置项 | 如何填写 |
| --- | --- |
| `accountId` | Cloudflare 账户 ID，可在 Workers 页面找到 |
| `apiToken` | 用于上传 Worker 的 Cloudflare API Token |
| `sphWorkerName` | 自定义 Worker 名称，例如 `sph`；再次部署会更新同名 Worker |
| `sphCookie` | 上一节获取的元宝 Cookie，也可以填写完整 Cookie |
| `sphCredential` | 自己设置的非空访问密码，用于保护查询 API；与元宝 Cookie、Cloudflare API Token 无关 |

### 2. 执行部署命令

在下载器所在目录打开终端，指定刚才修改的配置文件。

Windows PowerShell：

```powershell
.\wx_video_download.exe --config .\config.yaml deploy sph
```

macOS / Linux：

```sh
./wx_video_download --config ./config.yaml deploy sph
```

部署成功后，终端会输出 Worker 名称和访问 URL，例如 `https://sph.<subdomain>.workers.dev`。

### 3. 查询和下载

1. 打开部署命令输出的 URL。
2. 在「访问凭证（调用接口需要）」输入框填写 `sphCredential`，点击「保存凭证」。凭证会保存在当前浏览器中。
3. 粘贴完整的视频号分享链接，点击「查询」。
4. 查询成功后查看视频信息，点击「下载视频」保存；图集内容可点击「下载图集 ZIP」。

页面本身可以直接打开，查询和下载 API 需要凭证。如果浏览器中保存了旧凭证，点击「重新填写」后保存新的 `sphCredential`。

## 更新失效的 Cookie

Cookie 可能过期，请以实际解析结果为准。失效后重新登录元宝，按[获取元宝 Cookie](#cookie)的步骤复制新值。

- **本机使用**：更新 `config.yaml` 的 `cloudflare.sphCookie`，保存后重启下载器。如果使用了 `cookies.json`，也需要更新其中的元宝 Cookie，避免继续优先读取旧值。
- **Worker 使用**：更新 `config.yaml` 的 `cloudflare.sphCookie` 后重新执行 `deploy sph`；也可以直接在 Cloudflare 对应 Worker 的变量设置中更新 `COOKIE` 并部署生效。仅修改本地文件不会自动更新已部署的 Worker。

Worker 中的 `COOKIE` 对应 `sphCookie`，`ACCESS_CREDENTIAL` 对应 `sphCredential`。如果直接修改了 Worker 变量，也应同步修改本地配置，避免下次部署覆盖新值。

## 常见问题

### Cookie 很多，需要复制哪些？

关键字段是 `hy_token`，可以直接复制元宝完整 Cookie。请确认包含 `hy_token=...`，而不是只有 token 的值。

### 页面能打开，查询却提示 401 / unauthorized？

检查页面保存的访问凭证是否与部署时的 `sphCredential` 一致。这是 Worker 的访问认证失败，需要填写自己的访问密码。

### API 返回 503 / access credential is not configured？

Worker 缺少 `ACCESS_CREDENTIAL`。填写非空的 `cloudflare.sphCredential` 后重新部署，或在 Worker 中设置对应变量并部署生效。

### 提示 parseShareUrl 错误或 missing wx_export_id？

确认粘贴的是完整的视频号分享链接，并检查元宝是否仍处于登录状态、Cookie 是否包含有效的 `hy_token`。Worker 使用的是部署时上传的 `COOKIE`，更新本地配置后需要重新部署。

### 本机未配置 Cookie 时解析失败？

确认微信中的视频号页面已打开并连接到下载器；也可以配置有效的元宝 Cookie 后重启下载器，再重新解析。

### 提示内容不可用或 getFeedInfo 错误？

先检查原视频是否仍能在微信中正常访问。如果内容已删除、被限制访问或上游接口限制了请求，更新 Cookie 也不一定能恢复解析，请按页面返回的具体错误处理。

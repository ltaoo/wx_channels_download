---
title: Hub 使用
---

# Hub 使用

Hub 把一个人管理的多台操作系统设备组织成一个可远程调用的能力节点。外部程序只需要向 Hub 提交 `method + args`，Hub 会选择能够执行该方法的在线设备；也可以通过 `target_device_id` 指定设备。

```text
外部程序 ── HTTPS ── Hub Worker ── WebSocket ── macOS / Windows / Linux 设备
             │
             └── 调用 Token 鉴权、任务持久化、设备选择与结果查询
```

Hub 提供两种调用方式：`/v1/invoke` 会等待最多 10 秒并直接返回方法结果，适合快速调用；`/v1/call` 会立即返回持久任务，适合耗时操作、可靠重试和后台处理。

## 使用前准备

### 1. 部署 Hub

按照 [deploy hub](/cli/deploy#deploy-hub) 部署 Cloudflare Worker 和 Pages 管理页面。本文使用下面的 Worker 地址作为示例：

```text
https://wx-channels-hub.litao.workers.dev
```

### 2. 注册执行设备

需要提供能力的每个操作系统分别配置并运行 `wx_channels_download`：

```yaml
hub:
  enabled: true
  url: "https://wx-channels-hub.example.workers.dev"
  deviceId: "my-macbook"
  deviceName: "My MacBook"
  token: "<HUB_TOKEN>"
  httpTimeoutSeconds: 30
  methods: "auto"
```

- `deviceId` 是当前 Hub 内稳定且唯一的设备标识。
- `token` 是部署时设置的设备 Secret，只用于设备连接，不应提供给外部调用者。
- `methods: "auto"` 注册当前设备支持的全部方法；也可以使用逗号分隔的方法白名单。
- 视频号方法需要设备上的视频号页面已经连接；仅显示设备在线不代表视频号页面一定可用。

### 3. 创建调用 Token

打开 Pages 管理页面，点击右上角的“调用 Token”进入抽屉，然后创建调用 Token：

- Token 留空时由 Hub 自动生成，也可以手动指定 16–256 位 Token。
- 用途或使用人可选填。
- 可以设置 1、7、30、90 天或永不过期。
- Token 明文只显示一次，请立即保存。

本文示例通过环境变量读取地址和 Token：

```sh
export HUB_URL="https://wx-channels-hub.litao.workers.dev"
export HUB_CALL_TOKEN="hub_call_xxx"
```

::: warning 三类凭证不要混用

- 调用 Token：供外部程序访问 `/v1/*`，本文所有示例都使用它。
- 设备 Secret：配置项 `hub.token`，只供设备连接 Hub。
- 管理员 Token：配置项 `hub.deploy.adminToken`，只供管理页面和管理 API 使用。

:::

## API 概览

除健康检查外，请求都使用 Bearer 鉴权：

```http
Authorization: Bearer <CALL_TOKEN>
```

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 健康检查，不需要认证 |
| `GET` | `/v1` | 查询设备、在线方法和任务状态统计 |
| `POST` | `/v1/invoke` | 同步调用，最长等待 10 秒并直接返回结果 |
| `POST` | `/v1/call` | 创建异步方法调用 |
| `GET` | `/v1/tasks/:id` | 查询一个任务 |
| `GET` | `/v1/tasks?status=&limit=` | 查询当前 Token 创建的任务 |

每个调用 Token 都对应独立的发布方身份。一个 Token 不能读取另一个 Token 或设备创建的任务；尝试查询其他发布方的任务时返回 `404`。

## 查询 Hub 状态

```sh
curl -sS \
  -H "Authorization: Bearer $HUB_CALL_TOKEN" \
  "$HUB_URL/v1"
```

返回示例：

```json
{
  "devices": [
    {
      "device_id": "my-macbook",
      "device_name": "My MacBook",
      "device_os": "darwin",
      "methods": [
        "wxchannels.contact.feed.list",
        "wxchannels.feed.profile"
      ],
      "status": "online"
    }
  ],
  "methods": [
    "wxchannels.contact.feed.list",
    "wxchannels.feed.profile"
  ],
  "task_counts": []
}
```

`methods` 是当前至少有一台在线设备能够执行的方法合集。设备状态可能为 `online`、`busy` 或 `offline`。

## 同步调用

`POST /v1/invoke` 是日常调用推荐使用的便捷接口。请求只需要提供 `method`、`args`，以及可选的 `target_device_id`；不需要传 `idempotency_key`，也不需要自行查询任务状态。

下面的请求调用设备上的 `FetchChannelsFeedListOfContact`：

```sh
curl -sS \
  -X POST \
  -H "Authorization: Bearer $HUB_CALL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "wxchannels.contact.feed.list",
    "args": {
      "username": "example@finder",
      "next_marker": ""
    }
  }' \
  "$HUB_URL/v1/invoke"
```

请求字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `method` | 是 | 设备注册的方法名 |
| `args` | 否 | 传给方法的 JSON 对象，默认 `{}` |
| `target_device_id` | 否 | 指定执行设备；省略时由 Hub 自动选择 |

任务在 10 秒内完成时返回 `200`，响应体就是设备方法返回的 JSON 结果，不包含 `task`、任务 ID 或其他任务信息。设备执行失败时返回 `502`；10 秒内没有完成时返回 `504`：

```json
{
  "error": "invoke timed out after 10 seconds"
}
```

`/v1/invoke` 仍会在 Hub 内创建持久任务。HTTP 超时不会取消已经创建或正在执行的任务，该任务可能在响应超时后继续完成。由于响应不提供任务 ID，而且该接口不使用幂等键，不要直接重试可能产生副作用的调用；这类调用应使用 `/v1/call`。

## 创建异步调用

下面的请求调用设备上的 `FetchChannelsFeedListOfContact`：

```sh
curl -sS \
  -X POST \
  -H "Authorization: Bearer $HUB_CALL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "wxchannels.contact.feed.list",
    "args": {
      "username": "example@finder",
      "next_marker": ""
    },
    "idempotency_key": "feed-list-example-001"
  }' \
  "$HUB_URL/v1/call"
```

请求字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `method` | 是 | 设备注册的方法名 |
| `args` | 否 | 传给方法的 JSON 对象，默认 `{}` |
| `target_device_id` | 否 | 指定执行设备；省略时由 Hub 自动选择 |
| `idempotency_key` | 否 | 当前调用 Token 下的全局幂等键，最长 128 个字符 |

新任务通常返回 `201`：

```json
{
  "task": {
    "id": "7e793f36-87e0-4ae2-b899-3d57decd95de",
    "method": "wxchannels.contact.feed.list",
    "publisher_device_id": "caller:4ef4d8a0-...",
    "target_device_id": null,
    "idempotency_key": "feed-list-example-001",
    "args": {
      "username": "example@finder",
      "next_marker": ""
    },
    "status": "queued",
    "assigned_device_id": null,
    "attempt_count": 0,
    "result": null,
    "error": null,
    "created_at": 1787200000000,
    "updated_at": 1787200000000,
    "completed_at": null
  }
}
```

在同一个调用 Token 下重复提交相同的非空 `idempotency_key`，Hub 不会创建新任务，而是返回原任务和 `"idempotent_replay": true`。幂等键不按 method 分组，因此同一 Token 的所有调用都应使用不同的业务键。

### 指定执行设备

默认建议省略 `target_device_id`，让调用方只依赖 Hub 方法，不依赖内部设备。如果业务必须固定到某台设备，可以指定：

```json
{
  "method": "wxchannels.contact.feed.list",
  "target_device_id": "my-macbook",
  "args": {
    "username": "example@finder",
    "next_marker": ""
  }
}
```

目标设备未登记时返回 `404`；设备没有注册该方法时返回 `409`。未指定设备时，如果暂时没有合适的在线设备，任务会保持 `queued`，等待设备上线或空闲。

## 查询任务和结果

```sh
TASK_ID="7e793f36-87e0-4ae2-b899-3d57decd95de"

curl -sS \
  -H "Authorization: Bearer $HUB_CALL_TOKEN" \
  "$HUB_URL/v1/tasks/$TASK_ID"
```

任务状态：

| 状态 | 含义 |
| --- | --- |
| `queued` | 等待符合条件的设备 |
| `assigned` | 已分配并推送给设备 |
| `running` | 设备已确认并正在执行 |
| `completed` | 执行成功，结果位于 `task.result` |
| `failed` | 执行失败，原因位于 `task.error` |

推荐每 1–5 秒查询一次，遇到 `completed` 或 `failed` 后停止。网络错误、`429` 和 `5xx` 可以使用指数退避重试；不要高频无间隔轮询。

查询当前 Token 创建的最近任务：

```sh
curl -sS \
  -H "Authorization: Bearer $HUB_CALL_TOKEN" \
  "$HUB_URL/v1/tasks?status=completed&limit=20"
```

`limit` 的有效范围为 1–200。省略 `status` 时返回所有状态。

## 可调用方法

实际可用方法以 `GET /v1` 返回的 `methods` 为准。

| method | args | 说明 |
| --- | --- | --- |
| `wxchannels.fetch` | `url` | 根据视频号 URL 获取规范化内容 |
| `wxchannels.contact.search` | `keyword`，可选 `next_marker` | 调用 `SearchChannelsContact` 搜索账号 |
| `wxchannels.contact.feed.list` | `username`，可选 `next_marker` | 调用 `FetchChannelsFeedListOfContact` 获取账号视频列表 |
| `wxchannels.live.replay.list` | `username`，可选 `next_marker` | 调用 `FetchChannelsLiveReplayList` 获取直播回放 |
| `wxchannels.feed.profile` | `oid`、`nid`、`url`、`eid` | 调用 `FetchChannelsFeedProfile`；`oid`、`url`、`eid` 至少提供一个 |
| `wxchannels.feed.comment.list` | `oid`，`nid` 或 `comment_id`，可选 `next_marker` | 调用 `FetchChannelsFeedCommentList` 获取评论 |
| `wxchannels.feed.share_url` | `oid` | 调用 `FetchChannelsFeedShareUrl` 获取分享链接 |
| `download.create` | `request` 或 `url_request` | 在执行设备上创建并启动下载任务 |

账号搜索继续翻页时，把上一页 `data.lastBuff` 作为 `next_marker`；其他视频号列表把上一页 `data.lastBuffer` 作为 `next_marker`。游标应原样传递，不要自行解码。

### 搜索视频号账号

```json
{
  "method": "wxchannels.contact.search",
  "args": {
    "keyword": "视频号名称",
    "next_marker": ""
  }
}
```

### 获取单个视频详情

按视频号 URL 获取：

```json
{
  "method": "wxchannels.feed.profile",
  "args": {
    "url": "https://channels.weixin.qq.com/web/pages/feed?..."
  }
}
```

也可以传 `oid` + `nid`，或只传 `eid`。

### 在设备上创建下载任务

```json
{
  "method": "download.create",
  "target_device_id": "downloader-linux",
  "args": {
    "url_request": {
      "url": "https://example.com/video.mp4",
      "download_dir": "/downloads",
      "filename": "video.mp4",
      "auto_start": true,
      "config": {}
    }
  }
}
```

`download.create` 会在目标设备写入文件系统。外部调用方应明确下载目录和覆盖策略，并优先指定受控的下载设备。

## 主流语言调用示例

下面的示例都通过 `/v1/invoke` 调用 `wxchannels.contact.feed.list`，并直接输出方法结果。客户端 HTTP 超时应略大于 Hub 的 10 秒等待时间。

### JavaScript / TypeScript（Node.js 18+）

```js
const hub_url = process.env.HUB_URL?.replace(/\/+$/, "");
const call_token = process.env.HUB_CALL_TOKEN;

if (!hub_url || !call_token) {
  throw new Error("请设置 HUB_URL 和 HUB_CALL_TOKEN");
}

const response = await fetch(`${hub_url}/v1/invoke`, {
  method: "POST",
  headers: {
    Authorization: `Bearer ${call_token}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    method: "wxchannels.contact.feed.list",
    args: {
      username: "example@finder",
      next_marker: "",
    },
  }),
  signal: AbortSignal.timeout(15_000),
});

const response_body = await response.text();
if (!response.ok) {
  throw new Error(`Hub 返回 ${response.status}: ${response_body}`);
}

const result = JSON.parse(response_body);
console.log(result);
```

### Python 3

先安装 Requests：

```sh
python3 -m pip install requests
```

```python
import os
import requests

hub_url = os.environ["HUB_URL"].rstrip("/")
call_token = os.environ["HUB_CALL_TOKEN"]

response = requests.post(
    f"{hub_url}/v1/invoke",
    headers={"Authorization": f"Bearer {call_token}"},
    json={
        "method": "wxchannels.contact.feed.list",
        "args": {
            "username": "example@finder",
            "next_marker": "",
        },
    },
    timeout=15,
)
response.raise_for_status()

result = response.json()
print(result)
```

### Go

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type CallRequest struct {
	Method string         `json:"method"`
	Args   map[string]any `json:"args"`
}

func main() {
	hub_url := strings.TrimRight(os.Getenv("HUB_URL"), "/")
	call_token := os.Getenv("HUB_CALL_TOKEN")
	if hub_url == "" || call_token == "" {
		log.Fatal("请设置 HUB_URL 和 HUB_CALL_TOKEN")
	}

	request_body, err := json.Marshal(CallRequest{
		Method: "wxchannels.contact.feed.list",
		Args: map[string]any{
			"username":    "example@finder",
			"next_marker": "",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	http_client := &http.Client{Timeout: 15 * time.Second}
	http_request, err := http.NewRequest(
		http.MethodPost,
		hub_url+"/v1/invoke",
		bytes.NewReader(request_body),
	)
	if err != nil {
		log.Fatal(err)
	}
	http_request.Header.Set("Authorization", "Bearer "+call_token)
	http_request.Header.Set("Content-Type", "application/json")

	http_response, err := http_client.Do(http_request)
	if err != nil {
		log.Fatal(err)
	}
	defer http_response.Body.Close()

	response_body, err := io.ReadAll(http_response.Body)
	if err != nil {
		log.Fatal(err)
	}
	if http_response.StatusCode < 200 || http_response.StatusCode >= 300 {
		log.Fatalf("Hub 返回 %s: %s", http_response.Status, response_body)
	}

	fmt.Println(string(response_body))
}
```

### Java 11+

下面的示例只使用 JDK 标准库，并直接输出方法返回的 JSON；实际项目可使用 Jackson、Gson 等库解析结果。

```java
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public class HubExample {
    public static void main(String[] args) throws Exception {
        String hubUrlValue = System.getenv("HUB_URL");
        String callToken = System.getenv("HUB_CALL_TOKEN");
        if (hubUrlValue == null || hubUrlValue.isBlank()
                || callToken == null || callToken.isBlank()) {
            throw new IllegalStateException("请设置 HUB_URL 和 HUB_CALL_TOKEN");
        }
        String hubUrl = hubUrlValue.replaceAll("/+$", "");
        String requestBody = "{"
            + "\"method\":\"wxchannels.contact.feed.list\","
            + "\"args\":{"
            + "\"username\":\"example@finder\","
            + "\"next_marker\":\"\"}"
            + "}";

        HttpRequest request = HttpRequest.newBuilder()
            .uri(URI.create(hubUrl + "/v1/invoke"))
            .timeout(Duration.ofSeconds(15))
            .header("Authorization", "Bearer " + callToken)
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(requestBody))
            .build();

        HttpResponse<String> response = HttpClient.newHttpClient().send(
            request,
            HttpResponse.BodyHandlers.ofString()
        );
        if (response.statusCode() < 200 || response.statusCode() >= 300) {
            throw new IllegalStateException(
                "Hub 返回 " + response.statusCode() + ": " + response.body()
            );
        }
        System.out.println(response.body());
    }
}
```

### PHP 8+

```php
<?php

$hub_url = rtrim((string) getenv('HUB_URL'), '/');
$call_token = (string) getenv('HUB_CALL_TOKEN');
if ($hub_url === '' || $call_token === '') {
    throw new RuntimeException('请设置 HUB_URL 和 HUB_CALL_TOKEN');
}
$request_body = json_encode([
    'method' => 'wxchannels.contact.feed.list',
    'args' => [
        'username' => 'example@finder',
        'next_marker' => '',
    ],
], JSON_THROW_ON_ERROR);

$curl_handle = curl_init($hub_url . '/v1/invoke');
curl_setopt_array($curl_handle, [
    CURLOPT_POST => true,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 15,
    CURLOPT_HTTPHEADER => [
        'Authorization: Bearer ' . $call_token,
        'Content-Type: application/json',
    ],
    CURLOPT_POSTFIELDS => $request_body,
]);

$response_body = curl_exec($curl_handle);
if ($response_body === false) {
    throw new RuntimeException(curl_error($curl_handle));
}
$status_code = curl_getinfo($curl_handle, CURLINFO_RESPONSE_CODE);
curl_close($curl_handle);

if ($status_code < 200 || $status_code >= 300) {
    throw new RuntimeException("Hub 返回 {$status_code}: {$response_body}");
}

json_decode($response_body, true, flags: JSON_THROW_ON_ERROR);
echo $response_body . PHP_EOL;
```

### C#（.NET 6+）

```csharp
using System.Net.Http.Headers;
using System.Net.Http.Json;

var hub_url = (Environment.GetEnvironmentVariable("HUB_URL") ?? "").TrimEnd('/');
var call_token = Environment.GetEnvironmentVariable("HUB_CALL_TOKEN") ?? "";
if (string.IsNullOrWhiteSpace(hub_url) || string.IsNullOrWhiteSpace(call_token))
{
    throw new InvalidOperationException("请设置 HUB_URL 和 HUB_CALL_TOKEN");
}

using var http_client = new HttpClient { Timeout = TimeSpan.FromSeconds(15) };
http_client.DefaultRequestHeaders.Authorization =
    new AuthenticationHeaderValue("Bearer", call_token);

var request_body = new
{
    method = "wxchannels.contact.feed.list",
    args = new
    {
        username = "example@finder",
        next_marker = "",
    },
};

using var http_response = await http_client.PostAsJsonAsync(
    hub_url + "/v1/invoke",
    request_body
);
var response_body = await http_response.Content.ReadAsStringAsync();
if (!http_response.IsSuccessStatusCode)
{
    throw new HttpRequestException(
        $"Hub 返回 {(int)http_response.StatusCode}: {response_body}"
    );
}

Console.WriteLine(response_body);
```

### Rust

依赖：

```toml
[dependencies]
reqwest = { version = "0.12", features = ["json"] }
serde_json = "1"
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

```rust
use reqwest::Client;
use serde_json::{json, Value};
use std::env;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let hub_url = env::var("HUB_URL")?.trim_end_matches('/').to_owned();
    let call_token = env::var("HUB_CALL_TOKEN")?;
    let http_client = Client::builder()
        .timeout(std::time::Duration::from_secs(15))
        .build()?;

    let result: Value = http_client
        .post(format!("{hub_url}/v1/invoke"))
        .bearer_auth(call_token)
        .json(&json!({
            "method": "wxchannels.contact.feed.list",
            "args": {
                "username": "example@finder",
                "next_marker": ""
            }
        }))
        .send()
        .await?
        .error_for_status()?
        .json()
        .await?;

    println!("{result}");
    Ok(())
}
```

## 完整轮询示例

下面的 JavaScript 示例创建调用并等待最终结果。`idempotency_key` 应使用业务对象 ID 或请求唯一 ID，而不是每次重试都生成新值。

```js
const hub_url = process.env.HUB_URL?.replace(/\/+$/, "");
const call_token = process.env.HUB_CALL_TOKEN;
if (!hub_url || !call_token) {
  throw new Error("请设置 HUB_URL 和 HUB_CALL_TOKEN");
}
const headers = {
  Authorization: `Bearer ${call_token}`,
  "Content-Type": "application/json",
};

async function hub_request(path, options = {}) {
  const response = await fetch(hub_url + path, {
    ...options,
    headers: { ...headers, ...(options.headers || {}) },
  });
  const response_body = await response.text();
  if (!response.ok) {
    throw new Error(`Hub 返回 ${response.status}: ${response_body}`);
  }
  return JSON.parse(response_body);
}

async function wait_for_task(task_id) {
  let delay_milliseconds = 1000;
  while (true) {
    const { task } = await hub_request(
      `/v1/tasks/${encodeURIComponent(task_id)}`,
    );
    if (task.status === "completed") return task.result;
    if (task.status === "failed") throw new Error(task.error || "Hub 调用失败");
    await new Promise((resolve) => setTimeout(resolve, delay_milliseconds));
    delay_milliseconds = Math.min(delay_milliseconds * 1.5, 5000);
  }
}

const { task } = await hub_request("/v1/call", {
  method: "POST",
  body: JSON.stringify({
    method: "wxchannels.contact.feed.list",
    args: { username: "example@finder", next_marker: "" },
    idempotency_key: "complete-example-001",
  }),
});

const result = await wait_for_task(task.id);
console.log(result);
```

## 错误处理

Hub 错误响应统一为：

```json
{
  "error": "错误说明"
}
```

常见 HTTP 状态：

| 状态 | 说明 |
| --- | --- |
| `400` | JSON、method、args、设备 ID 或幂等键无效 |
| `401` | Token 缺失、错误或已经过期 |
| `404` | 目标设备未登记，或任务不存在/不属于当前 Token |
| `409` | 指定设备没有注册所需方法 |
| `413` | 请求体或 args 超过 1 MiB |
| `429` | Cloudflare 或上层网关限流，应退避重试 |
| `502` | `/v1/invoke` 的设备方法执行失败，错误位于 `error` |
| `504` | `/v1/invoke` 等待超过 10 秒，内部任务可能仍会继续执行 |
| 其他 `5xx` | Hub 或执行环境暂时异常；异步调用应使用相同幂等键重试 |

异步任务进入 `failed` 时，HTTP 查询本身仍返回 `200`，业务错误位于 `task.error`。同步调用则直接返回非 `2xx` 状态和 `{ "error": "..." }`。

## 可靠性与安全建议

- Hub 使用至少一次投递。设备断线或租约过期时，任务可能重新执行；执行方法应能容忍重复调用。
- 对可能产生副作用的调用始终设置稳定的 `idempotency_key`。
- `/v1/invoke` 不使用幂等键。只应自动重试无副作用的方法；其他调用使用 `/v1/call` 并设置幂等键。
- 不要把调用 Token 放在 URL、浏览器前端代码、日志或 Git 仓库中。
- 为不同人员和系统创建不同 Token，分别设置有效期；不再使用时立即过期或移除。
- 调用 Token 当前可以访问 Hub 中全部在线方法。需要方法级权限、限流或计费时，应在 Hub 前增加上层网关。
- 调用 args、结果和任务元数据会在 Hub 中持久化；不要通过 Hub 传递不必要的敏感信息。
- 完成和失败任务保留 7 天，调用方应及时保存需要长期使用的结果。

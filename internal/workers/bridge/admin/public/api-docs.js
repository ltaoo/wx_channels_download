(function expose_bridge_api_docs(root, factory) {
  const api_docs = factory();
  if (typeof module === "object" && module.exports) {
    module.exports = api_docs;
  }
  if (root) {
    root.BridgeApiDocs = api_docs;
  }
})(typeof globalThis === "undefined" ? this : globalThis, function bridge_api_docs_factory() {
  "use strict";

  const METHOD_DESCRIPTORS = {
    "wxchannels.fetch": {
      title: "获取视频号内容",
      description: "根据视频号 URL 获取规范化内容。",
      recommended_mode: "invoke",
      fields: [
        { name: "url", required: true, description: "视频号分享或详情页 URL" },
        { name: "download", required: false, description: "获取后在执行设备创建下载的可选配置" },
      ],
      args: {
        url: "https://channels.weixin.qq.com/web/pages/feed?...",
      },
    },
    "wxchannels.contact.search": {
      title: "搜索视频号账号",
      description: "调用 SearchChannelsContact 搜索视频号账号。",
      recommended_mode: "invoke",
      fields: [
        { name: "keyword", required: true, description: "视频号名称或搜索关键词" },
        { name: "next_marker", required: false, description: "上一页 data.lastBuff，首次请求传空字符串" },
      ],
      args: {
        keyword: "视频号名称",
        next_marker: "",
      },
    },
    "wxchannels.contact.feed.list": {
      title: "获取账号视频列表",
      description: "调用 FetchChannelsFeedListOfContact 获取账号的视频列表。",
      recommended_mode: "invoke",
      fields: [
        { name: "username", required: true, description: "视频号账号 username" },
        { name: "next_marker", required: false, description: "上一页 data.lastBuffer，首次请求传空字符串" },
      ],
      args: {
        username: "example@finder",
        next_marker: "",
      },
    },
    "wxchannels.live.replay.list": {
      title: "获取直播回放",
      description: "调用 FetchChannelsLiveReplayList 获取账号的直播回放列表。",
      recommended_mode: "invoke",
      fields: [
        { name: "username", required: true, description: "视频号账号 username" },
        { name: "next_marker", required: false, description: "上一页 data.lastBuffer，首次请求传空字符串" },
      ],
      args: {
        username: "example@finder",
        next_marker: "",
      },
    },
    "wxchannels.feed.profile": {
      title: "获取视频详情",
      description: "调用 FetchChannelsFeedProfile；oid、url、eid 至少提供一个。",
      recommended_mode: "invoke",
      fields: [
        { name: "url", required: false, description: "视频号详情页 URL，可单独使用" },
        { name: "oid", required: false, description: "视频对象 ID，可与 nid 一起使用" },
        { name: "nid", required: false, description: "视频的 nonce ID" },
        { name: "eid", required: false, description: "加密视频 ID，可单独使用" },
      ],
      args: {
        url: "https://channels.weixin.qq.com/web/pages/feed?...",
      },
    },
    "wxchannels.feed.comment.list": {
      title: "获取视频评论",
      description: "调用 FetchChannelsFeedCommentList；必须提供 oid，nid 与 comment_id 至少提供一个。",
      recommended_mode: "invoke",
      fields: [
        { name: "oid", required: true, description: "视频对象 ID" },
        { name: "nid", required: false, description: "视频的 nonce ID" },
        { name: "comment_id", required: false, description: "继续读取回复时使用的评论 ID" },
        { name: "next_marker", required: false, description: "上一页 data.lastBuffer" },
      ],
      args: {
        oid: "video-object-id",
        nid: "video-nonce-id",
        next_marker: "",
      },
    },
    "wxchannels.feed.share_url": {
      title: "获取视频分享链接",
      description: "调用 FetchChannelsFeedShareUrl 获取视频分享链接。",
      recommended_mode: "invoke",
      fields: [
        { name: "oid", required: true, description: "视频对象 ID" },
      ],
      args: {
        oid: "video-object-id",
      },
    },
    "wxmp.biz.msg.list": {
      title: "获取公众号消息列表",
      description: "调用 FetchBizMsgList 获取公众号消息列表；设备端可能等待 20 秒，推荐异步调用。",
      recommended_mode: "call",
      fields: [
        { name: "username", required: true, description: "公众号 username" },
        { name: "offset", required: false, description: "分页偏移量，首次请求传 0" },
      ],
      args: {
        username: "公众号用户名",
        offset: "0",
      },
    },
    "download.create": {
      title: "在设备上创建下载",
      description: "在执行设备上创建并启动本地下载任务；request 与 url_request 必须且只能提供一个。",
      recommended_mode: "call",
      fields: [
        { name: "url_request", required: false, description: "根据一个直接 URL 创建下载任务" },
        { name: "request", required: false, description: "根据规范化内容创建下载任务" },
      ],
      args: {
        url_request: {
          url: "https://example.com/video.mp4",
          download_dir: "/downloads",
          filename: "video.mp4",
          auto_start: true,
          config: {},
        },
      },
    },
  };

  const LANGUAGE_OPTIONS = [
    { value: "curl", label: "cURL" },
    { value: "node-fetch", label: "Node.js Fetch" },
    { value: "axios", label: "Axios" },
    { value: "go", label: "Go Client" },
    { value: "python", label: "Python Requests" },
  ];

  function unique_strings(values) {
    return [
      ...new Set(
        (Array.isArray(values) ? values : [])
          .map((value) => String(value || "").trim())
          .filter(Boolean),
      ),
    ];
  }

  function method_names(online_methods) {
    return unique_strings([
      ...Object.keys(METHOD_DESCRIPTORS),
      ...unique_strings(online_methods),
    ]);
  }

  function method_descriptor(method) {
    const method_name = String(method || "").trim();
    const descriptor = METHOD_DESCRIPTORS[method_name];
    if (descriptor) {
      return Object.assign({ method: method_name, custom: false }, descriptor, {
        fields: descriptor.fields.map((field) => Object.assign({}, field)),
        args: JSON.parse(JSON.stringify(descriptor.args)),
      });
    }
    return {
      method: method_name,
      title: "自定义 Bridge 方法",
      description: "此方法由在线设备动态注册；请按对应执行设备约定填写 args。",
      recommended_mode: "invoke",
      fields: [],
      args: {},
      custom: true,
    };
  }

  function request_args(method, value) {
    if (typeof value === "undefined") {
      return method_descriptor(method).args;
    }
    let args = value;
    if (typeof args === "string") {
      if (args.trim() === "") return {};
      try {
        args = JSON.parse(args);
      } catch (error) {
        throw new Error(
          "args 必须是有效的 JSON：" +
            (error instanceof Error ? error.message : String(error)),
        );
      }
    }
    if (args === null || Array.isArray(args) || typeof args !== "object") {
      throw new Error("args 必须是 JSON 对象");
    }
    return JSON.parse(JSON.stringify(args));
  }

  function request_payload(method, mode, args) {
    const descriptor = method_descriptor(method);
    const payload = {
      method: descriptor.method,
      args: request_args(method, args),
    };
    if (mode === "call") {
      payload.idempotency_key = descriptor.method.replace(/[^A-Za-z0-9]+/g, "-") + "-example-001";
    }
    return payload;
  }

  function endpoint_path(mode) {
    return mode === "call" ? "/v1/call" : "/v1/invoke";
  }

  function normalize_hostname(value) {
    let hostname = String(value || "").trim();
    if (hostname === "") return "";
    if (!/^https?:\/\//i.test(hostname)) {
      hostname = "https://" + hostname;
    }
    try {
      const url = new URL(hostname);
      if (url.protocol !== "http:" && url.protocol !== "https:") return "";
      url.username = "";
      url.password = "";
      url.search = "";
      url.hash = "";
      url.pathname = url.pathname.replace(/\/+$/, "");
      return url.toString().replace(/\/+$/, "");
    } catch {
      return "";
    }
  }

  function shell_single_quote(value) {
    return "'" + String(value).replace(/'/g, "'\\''") + "'";
  }

  function curl_code(payload, path, base_url, token) {
    const json_body = JSON.stringify(payload, null, 2)
      .split("\n")
      .map((line, index) => (index === 0 ? line : "  " + line))
      .join("\n");
    const authorization = token
      ? shell_single_quote("Authorization: Bearer " + token)
      : '"Authorization: Bearer $BRIDGE_CALL_TOKEN"';
    const request_url = base_url
      ? shell_single_quote(base_url + path)
      : '"$BRIDGE_URL' + path + '"';
    return [
      "curl -sS \\",
      "  -X POST \\",
      "  -H " + authorization + " \\",
      '  -H "Content-Type: application/json" \\',
      "  -d " + shell_single_quote(json_body) + " \\",
      "  " + request_url,
    ].join("\n");
  }

  function node_fetch_code(payload, path, base_url, token) {
    const payload_json = JSON.stringify(payload, null, 2);
    const bridge_url_value = base_url
      ? JSON.stringify(base_url)
      : 'process.env.BRIDGE_URL?.replace(/\\/+$/, "")';
    const call_token_value = token
      ? JSON.stringify(token)
      : "process.env.BRIDGE_CALL_TOKEN";
    const missing_environment = [
      base_url ? "" : "BRIDGE_URL",
      token ? "" : "BRIDGE_CALL_TOKEN",
    ].filter(Boolean);
    const environment_check = missing_environment.length === 0
      ? ""
      : `
if (!bridge_url || !call_token) {
  throw new Error("请设置 ${missing_environment.join(" 和 ")}");
}
`;
    const main_body = `const bridge_url = ${bridge_url_value};
const call_token = ${call_token_value};
${environment_check}
const response = await fetch(\`${"${bridge_url}"}${path}\`, {
  method: "POST",
  headers: {
    Authorization: \`Bearer ${"${call_token}"}\`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(${indent_block(payload_json, 2)}),
  signal: AbortSignal.timeout(15_000),
});

const response_body = await response.text();
if (!response.ok) {
  throw new Error(\`Bridge 返回 ${"${response.status}"}: ${"${response_body}"}\`);
}

console.log(JSON.parse(response_body));`;
    return `// Node.js 18+
async function main() {
  ${indent_block(main_body, 2)}
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});`;
  }

  function axios_code(payload, path, base_url, token) {
    const payload_json = JSON.stringify(payload, null, 2);
    const bridge_url_value = base_url
      ? JSON.stringify(base_url)
      : 'process.env.BRIDGE_URL?.replace(/\\/+$/, "")';
    const call_token_value = token
      ? JSON.stringify(token)
      : "process.env.BRIDGE_CALL_TOKEN";
    const missing_environment = [
      base_url ? "" : "BRIDGE_URL",
      token ? "" : "BRIDGE_CALL_TOKEN",
    ].filter(Boolean);
    const environment_check = missing_environment.length === 0
      ? ""
      : `
if (!bridge_url || !call_token) {
  throw new Error("请设置 ${missing_environment.join(" 和 ")}");
}
`;
    const main_body = `const bridge_url = ${bridge_url_value};
const call_token = ${call_token_value};
${environment_check}
const response = await axios.post(
  \`${"${bridge_url}"}${path}\`,
  ${indent_block(payload_json, 2)},
  {
    headers: { Authorization: \`Bearer ${"${call_token}"}\` },
    timeout: 15_000,
  },
);

console.log(response.data);`;
    return `// npm install axios
const axios = require("axios");

async function main() {
  ${indent_block(main_body, 2)}
}

main().catch((error) => {
  console.error(error.response?.data || error);
  process.exitCode = 1;
});`;
  }

  function go_code(payload, path, base_url, token) {
    const payload_json = JSON.stringify(payload, null, 2);
    const bridge_url_value = base_url
      ? JSON.stringify(base_url)
      : 'strings.TrimRight(os.Getenv("BRIDGE_URL"), "/")';
    const call_token_value = token
      ? JSON.stringify(token)
      : 'os.Getenv("BRIDGE_CALL_TOKEN")';
    const environment_imports =
      base_url && token ? "" : '\t"os"\n' + (base_url ? "" : '\t"strings"\n');
    const missing_environment = [
      base_url ? "" : "BRIDGE_URL",
      token ? "" : "BRIDGE_CALL_TOKEN",
    ].filter(Boolean);
    const environment_check = missing_environment.length === 0
      ? ""
      : `	if bridge_url == "" || call_token == "" {
		log.Fatal("请设置 ${missing_environment.join(" 和 ")}")
	}
`;
    return `package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
${environment_imports}	"time"
)

func main() {
	bridge_url := ${bridge_url_value}
	call_token := ${call_token_value}
${environment_check}

	request_body := []byte(${JSON.stringify(payload_json)})
	http_request, err := http.NewRequest(
		http.MethodPost,
		bridge_url+"${path}",
		bytes.NewReader(request_body),
	)
	if err != nil {
		log.Fatal(err)
	}
	http_request.Header.Set("Authorization", "Bearer "+call_token)
	http_request.Header.Set("Content-Type", "application/json")

	http_client := &http.Client{Timeout: 15 * time.Second}
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
		log.Fatalf("Bridge 返回 %s: %s", http_response.Status, response_body)
	}

	fmt.Println(string(response_body))
}
`;
  }

  function python_code(payload, path, base_url, token) {
    const payload_json = JSON.stringify(payload, null, 2);
    const bridge_url_value = base_url
      ? JSON.stringify(base_url)
      : 'os.environ["BRIDGE_URL"].rstrip("/")';
    const call_token_value = token
      ? JSON.stringify(token)
      : 'os.environ["BRIDGE_CALL_TOKEN"]';
    return `# python3 -m pip install requests
import json
import os
import requests

bridge_url = ${bridge_url_value}
call_token = ${call_token_value}

response = requests.post(
    f"{bridge_url}${path}",
    headers={"Authorization": f"Bearer {call_token}"},
    json=json.loads(${JSON.stringify(payload_json)}),
    timeout=15,
)
response.raise_for_status()

print(response.json())`;
  }

  function indent_block(value, spaces) {
    const indentation = " ".repeat(spaces);
    return String(value)
      .split("\n")
      .map((line, index) => (index === 0 || line === "" ? line : indentation + line))
      .join("\n");
  }

  function generate_code(options) {
    const method = String(options?.method || "wxchannels.fetch");
    const mode = options?.mode === "call" ? "call" : "invoke";
    const language = String(options?.language || "curl");
    const base_url = normalize_hostname(options?.hostname || options?.base_url);
    const token = String(options?.token || "").trim();
    const payload = request_payload(method, mode, options?.args);
    const path = endpoint_path(mode);
    switch (language) {
      case "node-fetch":
        return node_fetch_code(payload, path, base_url, token);
      case "axios":
        return axios_code(payload, path, base_url, token);
      case "go":
        return go_code(payload, path, base_url, token);
      case "python":
        return python_code(payload, path, base_url, token);
      default:
        return curl_code(payload, path, base_url, token);
    }
  }

  function openapi_document(online_methods, hostname) {
    const methods = method_names(online_methods);
    const method_extensions = methods.map((method) => {
      const descriptor = method_descriptor(method);
      return {
        method,
        title: descriptor.title,
        description: descriptor.description,
        recommended_endpoint: endpoint_path(descriptor.recommended_mode),
        args_example: descriptor.args,
        online: unique_strings(online_methods).includes(method),
      };
    });
    const method_schema = {
      type: "string",
      description: "设备注册的 Bridge 方法名",
    };
    if (methods.length > 0) {
      method_schema.enum = methods;
    }
    return {
      openapi: "3.1.0",
      info: {
        title: "WX Channels Bridge API",
        version: "1.0.0",
        description: "通过 method + args 同步调用设备能力或创建异步任务。",
      },
      servers: [
        {
          url: normalize_hostname(hostname) || "https://your-bridge.workers.dev",
          description: normalize_hostname(hostname)
            ? "当前接口文档填写的 Bridge URL"
            : "请替换为部署输出的 Bridge Worker URL",
        },
      ],
      security: [{ bearerAuth: [] }],
      paths: {
        "/v1": {
          get: {
            summary: "查询 Bridge 状态与在线方法",
            responses: { "200": { description: "Bridge 状态" } },
          },
        },
        "/v1/credits": {
          get: {
            summary: "查询当前调用 Token 的积分",
            responses: { "200": { description: "积分余额" } },
          },
        },
        "/v1/invoke": call_operation("同步调用并直接返回方法结果", "InvokeRequest", "200"),
        "/v1/call": call_operation("创建异步方法调用", "AsyncCallRequest", "201"),
        "/v1/tasks": {
          get: {
            summary: "查询当前调用 Token 创建的任务",
            parameters: [
              { name: "status", in: "query", schema: { type: "string", enum: ["queued", "assigned", "running", "completed", "failed"] } },
              { name: "limit", in: "query", schema: { type: "integer", minimum: 1, maximum: 200, default: 20 } },
            ],
            responses: { "200": { description: "任务列表" } },
          },
        },
        "/v1/tasks/{task_id}": {
          get: {
            summary: "查询一个任务及其结果",
            parameters: [
              { name: "task_id", in: "path", required: true, schema: { type: "string" } },
            ],
            responses: { "200": { description: "任务详情" }, "404": { description: "任务不存在" } },
          },
        },
      },
      components: {
        securitySchemes: {
          bearerAuth: { type: "http", scheme: "bearer", bearerFormat: "Bridge call token" },
        },
        schemas: {
          InvokeRequest: request_schema(method_schema, false),
          AsyncCallRequest: request_schema(method_schema, true),
        },
      },
      "x-bridge-methods": method_extensions,
    };
  }

  function request_schema(method_schema, asynchronous) {
    const properties = {
      method: method_schema,
      args: { type: "object", additionalProperties: true, default: {} },
      target_device_id: { type: "string", description: "可选的目标执行设备 ID" },
    };
    if (asynchronous) {
      properties.idempotency_key = {
        type: "string",
        maxLength: 128,
        description: "当前调用 Token 下的可选全局幂等键",
      };
    }
    return {
      type: "object",
      required: ["method"],
      properties,
      additionalProperties: false,
    };
  }

  function call_operation(summary, schema_name, success_status) {
    return {
      post: {
        summary,
        requestBody: {
          required: true,
          content: {
            "application/json": {
              schema: { $ref: "#/components/schemas/" + schema_name },
            },
          },
        },
        responses: {
          [success_status]: { description: "调用成功" },
          "400": { description: "请求参数无效" },
          "401": { description: "调用 Token 无效" },
          "402": { description: "积分不足" },
        },
      },
    };
  }

  return Object.freeze({
    LANGUAGE_OPTIONS,
    endpointPath: endpoint_path,
    generateCode: generate_code,
    methodDescriptor: method_descriptor,
    methodNames: method_names,
    normalizeHostname: normalize_hostname,
    openapiDocument: openapi_document,
    requestPayload: request_payload,
  });
});

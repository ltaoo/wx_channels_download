import fs from "fs";
import { createHmac } from "crypto";

function base64_url(value) {
  return value.replace(/\+/g, "-").replace(/\//g, "_");
}

function create_upload_token(config, key, size) {
  const policy = {
    scope: `${config.bucket}:${key}`,
    deadline: Math.floor(Date.now() / 1000) + 3600,
    insertOnly: 1,
    fsizeLimit: size,
  };
  const encoded_policy = base64_url(
    Buffer.from(JSON.stringify(policy)).toString("base64"),
  );
  const signature = base64_url(
    createHmac("sha1", config.secret_key)
      .update(encoded_policy)
      .digest("base64"),
  );
  return `${config.access_key}:${signature}:${encoded_policy}`;
}

async function upload_task_files({ task, manifest_dir }, settings) {
  if (!task || !Array.isArray(task.files)) throw new Error("缺少下载完成任务详情 task.files");
  if (!task.files.length) {
    console.warn("任务没有最终文件，跳过七牛上传", { task_id: task.id });
    return [];
  }
  const task_config = JSON.parse(task.config_json || "{}");
  __basePath = manifest_dir || task_config.download_dir || task.files[0].download_dir || workdir;
  const config = { upload_url: "https://upload-z2.qiniup.com", ...settings };
  for (const field of [
    "access_key",
    "secret_key",
    "bucket",
    "domain",
    "upload_url",
  ]) {
    if (!config[field]) throw new Error(`七牛配置缺少 ${field}`);
  }
  console.info("七牛配置读取完成", {
    bucket: config.bucket,
    upload_url: config.upload_url,
    domain: config.domain,
  });

  const uploaded = [];
  const batch_id = Date.now();
  for (const [index, file] of task.files.entries()) {
    console.info("打开待上传文件", {
      index: index + 1,
      total: task.files.length,
      file_path: file.file_path,
    });
    if (!file.file_path) throw new Error(`文件缺少最终路径：${file.name}`);
    const filename = file.file_path.split(/[\\/]/).pop();
    const key = `${task.id}-${batch_id}-${index}-${filename}`;
    const blob = await fs.openAsBlob(file.file_path, {
      type: file.kind || "application/octet-stream",
    });
    const form = new FormData();
    form.append("token", create_upload_token(config, key, blob.size));
    form.append("key", key);
    form.append("file", blob, filename);

    const started_at = Date.now();
    console.info("开始七牛上传", {
      file_path: file.file_path,
      key,
      size: blob.size,
    });
    const response = await fetch(config.upload_url, {
      method: "POST",
      body: form,
    });
    console.info("收到七牛响应", {
      key,
      status: response.status,
      request_id: response.headers.get("x-reqid"),
      elapsed_ms: Date.now() - started_at,
    });
    if (!response.ok) {
      throw new Error(
        `七牛上传失败（${filename}）：HTTP ${response.status} ${await response.text()}`,
      );
    }
    const result = await response.json();
    if (result.key !== key || !result.hash)
      throw new Error(`七牛返回的文件信息不正确：${filename}`);
    const url = `${config.domain.replace(/\/+$/, "")}/${key.split("/").map(encodeURIComponent).join("/")}`;
    uploaded.push({ file_path: file.file_path, key, hash: result.hash, url });
    console.info("七牛上传成功", {
      file_path: file.file_path,
      key,
      hash: result.hash,
      url,
      elapsed_ms: Date.now() - started_at,
    });
    const manifest = `qiniu-upload-${task.id}.json`;
    console.info("保存上传清单", { manifest, uploaded_count: uploaded.length });
    const error = writeTextFile(manifest, JSON.stringify(uploaded, null, 2));
    if (error) throw new Error(`上传清单保存失败：${error}`);
    console.info("上传清单已保存", { manifest });
  }
  return uploaded;
}

registerNode({
  type: "QiniuUploadNode",
  name: "上传到七牛云",
  description: "上传下载任务的所有最终文件，逐个保存上传清单；全部成功后继续后续节点",
  config_keys: [
    { key: "arguments", type: "object", required: true, description: "task: 下载完成任务详情；manifest_dir: 可选的清单目录" },
    { key: "output_key", type: "string", description: "上传结果键，默认 result" },
    { key: "timeout_seconds", type: "number", description: "上传超时秒数；同时受流程剩余时间与脚本 10 分钟上限约束" }
  ],
  config_schema: [
    { key: "access_key", label: "Access Key", type: "string", required: true, secret: true },
    { key: "secret_key", label: "Secret Key", type: "string", required: true, secret: true },
    { key: "bucket", label: "存储空间", type: "string", required: true },
    { key: "domain", label: "访问域名", type: "string", required: true },
    { key: "upload_url", label: "上传地址", type: "string", default: "https://upload-z2.qiniup.com" }
  ],
  default_config: {
    arguments: { task: "{{input.event.data.task}}", manifest_dir: "{{input.event.data.download_dir}}" },
    output_key: "uploaded",
    timeout_seconds: 600
  },
  execute: upload_task_files
});

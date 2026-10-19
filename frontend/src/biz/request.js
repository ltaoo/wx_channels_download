export const request = Timeless.kit.request_factory({
  headers: { "Content-Type": "application/json" },
  process(r) {
    if (r.error) {
      return Timeless.Result.Err(r.error);
    }
    const data = r.data || {};
    if (typeof data.code === "undefined") {
      return Timeless.Result.Ok(data);
    }
    if (data.code !== 0) {
      return Timeless.Result.Err(data.msg || "请求失败", data.code, data.data);
    }
    return Timeless.Result.Ok(data.data || {});
  },
});

/** 获取账号列表。 */
export function fetchAccountList(params) {
  return request.get("/api/account/list", params);
}

/** 按 scope 获取账号主页内容，并对路径中的 scope 进行编码。 */
export function fetchAccountContents(body) {
  return request.get(
    `/api/account/${encodeURIComponent(body.scope)}/content/list`,
    body.params,
  );
}

/** 获取自动化流程列表。 */
export function fetchPipelineList(params) {
  return request.post("/api/v1/automation/list_flows", params);
}

/** 创建自动化流程。 */
export function createPipeline(body) {
  return request.post("/api/v1/automation/create_flow", body);
}

/** 更新自动化流程及触发器配置。 */
export function updatePipeline(body) {
  return request.post("/api/v1/automation/update_flow", body);
}

/** 删除自动化流程。 */
export function deletePipeline(body) {
  return request.post("/api/v1/automation/delete_flow", body);
}

/** 获取自动化流程详情。 */
export function fetchPipelineDetail(body) {
  return request.post("/api/v1/automation/get_flow", body);
}

/** 获取自动化流程节点目录。 */
export function fetchPipelineNodeCatalog(params) {
  return request.post("/api/v1/automation/list_flow_nodes", params);
}

/** 手动触发自动化流程。 */
export function triggerPipeline(body) {
  return request.post("/api/v1/automation/trigger_flow", body);
}

/** 导入自动化流程。 */
export function importPipeline(body) {
  return request.post("/api/v1/automation/import_flow", body);
}

/** 获取自动化调度列表。 */
export function fetchScheduleList(params) {
  return request.post("/api/v1/automation/list_schedules", params);
}

/** 手动触发自动化调度。 */
export function triggerSchedule(body) {
  return request.post("/api/v1/automation/trigger_schedule", body);
}

/** 获取自动化流程执行记录列表。 */
export function fetchPipelineRunList(body) {
  return request.post("/api/v1/automation/list_runs", body);
}

/** 获取自动化流程执行记录详情。 */
export function fetchPipelineRunDetail(body) {
  return request.post("/api/v1/automation/get_run", body);
}

/** 取消自动化流程执行。 */
export function cancelPipelineRun(body) {
  return request.post("/api/v1/automation/cancel_run", body);
}

/** 恢复自动化流程执行。 */
export function resumePipelineRun(body) {
  return request.post("/api/v1/automation/resume_run", body);
}

/** 重试自动化流程执行。 */
export function retryPipelineRun(body) {
  return request.post("/api/v1/automation/retry_run", body);
}

/** 获取浏览历史列表。 */
export function fetchBrowseHistoryList(params) {
  return request.post("/api/browse_history/list", params);
}

/** 获取内容列表。 */
export function fetchContentList(params) {
  return request.get("/api/content/list", params);
}

/** 获取内容详情。 */
export function fetchContentDetail(params) {
  return request.get("/api/content/detail", params);
}

/** 设置内容关联的标签。 */
export function setContentTags(body) {
  return request.post("/api/tag/content/set", body);
}

/** 检查下载任务资源文件是否存在，files 为待检查文件列表。 */
export function checkDownloadTaskFiles(files) {
  return request.post("/api/v1/download_task/check_files", { files });
}

/** 获取文件传输助手登录状态。 */
export function fetchFileHelperStatus() {
  return request.get("/api/filehelper/status");
}

/** 获取文件传输助手登录二维码。 */
export function fetchFileHelperQRCode() {
  return request.get("/api/filehelper/qrcode");
}

/** 等待文件传输助手扫码登录结果。 */
export function waitForFileHelperLogin() {
  return request.get("/api/filehelper/login/wait");
}

/** 获取视频号连接状态和服务信息。 */
export function fetchChannelsStatus() {
  return request.get("/api/status");
}

/** 检查文件传输助手是否有新消息。 */
export function checkFileHelperSync() {
  return request.get("/api/filehelper/synccheck");
}

/** 同步文件传输助手消息。 */
export function syncFileHelperMessages() {
  return request.get("/api/filehelper/sync");
}

/** 通过文件传输助手发送消息。 */
export function sendFileHelperMessage(params) {
  return request.post("/api/filehelper/send", params);
}

/** 退出文件传输助手登录。 */
export function logoutFileHelper() {
  return request.post("/api/filehelper/logout");
}

/** 获取日志列表。 */
export function fetchLogs(params) {
  return request.get("/api/logs", params);
}

/** 清空日志。 */
export function clearLogs() {
  return request.post("/api/logs/clear");
}

/** 获取下载任务详情。 */
export function fetchDownloadTaskDetail(params) {
  return request.get("/api/v1/download_task/detail", params);
}

/** 读取文件内容或压缩包目录。 */
export function fetchFile(params) {
  return request.get("/api/file", params);
}

/** 提交内容解析任务。 */
export function fetchScraperContent(body) {
  return request.post("/api/scraper/fetch", body);
}

/** 获取抖音账号主页内容。 */
export function fetchDouyinHome(params) {
  return request.get("/api/douyin/contact/home", params);
}

/** 获取内容解析任务状态。 */
export function fetchScraperJob(params) {
  return request.get("/api/scraper/job", params);
}

/** 中断内容解析任务。 */
export function interruptScraperFetch(body) {
  return request.post("/api/scraper/fetch/interrupt", body);
}

/** 清除指定链接的解析缓存。 */
export function clearScraperCache(body) {
  return request.post("/api/scraper/cache/clear", body);
}

/** 获取缓存中的解析内容。 */
export function fetchScraperCacheContent(params) {
  return request.get("/api/scraper/cache/content", params);
}

/** 获取 MCP 服务状态。 */
export function fetchMcpStatus() {
  return request.get("/api/mcp/status");
}

/** 根据 enabled 开启或关闭 MCP 服务。 */
export function setMcpEnabled(enabled) {
  return request.post(enabled ? "/api/mcp/enable" : "/api/mcp/disable");
}

/** 获取代理证书安装状态。 */
export function fetchProxyCertificateStatus() {
  return request.get("/api/proxy/certificate/status");
}

/** 卸载代理证书。 */
export function uninstallProxyCertificate() {
  return request.post("/api/proxy/certificate/uninstall");
}

/** 获取可同步的下载服务列表。 */
export function fetchSyncServers() {
  return request.get("/api/v1/sync/servers");
}

/** 将本地数据同步到指定下载服务。 */
export function syncToServer(server_id) {
  return request.post(`/api/v1/sync/servers/${server_id}`);
}

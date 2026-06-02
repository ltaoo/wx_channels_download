export const request = Timeless.request_factory({
  headers: { "Content-Type": "application/json" },
  process(r) {
    if (r.error) {
      return r;
    }
    const { code, msg, data } = r.data;
    if (code !== 0) {
      return Timeless.Result.Err(msg);
    }
    return Timeless.Result.Ok(data);
  },
});

export function fetchBooks() {
  return request.get("/api/v1/books");
}

export function fetchBookProfile(id) {
  return request.get("/api/v1/books/profile", { id });
}

export function fetchBookDetail(id) {
  return request.get("/api/v1/books/detail", { id });
}

export function fetchBookChapters(bookId) {
  return request.get("/api/v1/books/chapters", { book_id: bookId });
}

export function fetchReadingProgress(bookId) {
  return request.get("/api/v1/reading/progress", { book_id: bookId });
}

export function saveReadingProgress(body) {
  return request.post("/api/v1/reading/progress/save", body);
}

export function fetchChapterContent(chapterId) {
  return request.get("/api/v1/books/chapters/content", { chapter_id: chapterId });
}

export function importBookByPath(filePath) {
  return request.post("/api/v1/books/import", { file_path: filePath });
}

export async function importBookFile(file) {
  const formData = new FormData();
  formData.append("file", file);
  const resp = await fetch("/api/v1/books/import/upload", {
    method: "POST",
    body: formData,
  });
  const body = await resp.json();
  if (body.code !== 0) {
    return Timeless.Result.Err(body.msg || "upload failed");
  }
  return Timeless.Result.Ok(body.data);
}

/** @param {string} token */
export async function memberLogin(token) {
  const resp = await fetch("/api/v1/member/login", {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: token },
  });
  const body = await resp.json();
  if (body.code !== 0) {
    return { error: { msg: body.msg || "invalid token" } };
  }
  return { data: body.data };
}

export function searchFruits(body) {
  return request.get("/api/fruit", body);
}

/** @param {Record<string, any>} params */
export function fetchDownloadList(params) {
  return request.post("/api/download_task/list", params);
}

/** @param {Record<string, any>} params */
export function fetchRemoteDownloadList(params) {
  return proxyRemoteRequest({
    method: "GET",
    path: "/api/task/list",
    query: params,
  });
}

/** @param {{ method?: string; path: string; query?: Record<string, any>; body?: any; headers?: Record<string, string> }} body */
export function proxyRemoteRequest(body) {
  return request.post("/api/remote/proxy", body);
}

/** @param {Record<string, any>} params */
export function fetchAccountList(params = {}) {
  return request.post("/api/account/list", params);
}

/** @param {{ account_id?: number; username?: string }} body */
export function synchronizeAccount(body) {
  return request.post("/api/account/synchronize", body);
}

/** @param {Record<string, any>} params */
export function fetchVideoList(params = {}) {
  const { pageSize, ...rest } = params || {};
  return request.post("/api/video/list", {
    ...rest,
    page_size: pageSize || params.page_size,
  });
}

/** @param {{ username?: string }} params */
export function fetchBrowseHistoryList(params = {}) {
  return request.post("/api/browse_history/list", params);
}

export function fetchAppStatus() {
  return request.get("/api/status");
}

/** @param {{ name: string }} body */
export function startService(body) {
  return request.post("/api/service/start", body);
}

/** @param {{ name: string }} body */
export function stopService(body) {
  return request.post("/api/service/stop", body);
}

/** @param {{ values: Record<string, any> }} body */
export function updateServiceConfig(body) {
  return request.post("/api/service/config", body);
}

export function fetchRootCertificateStatus() {
  return request.get("/api/certificate/root/status");
}

export function installRootCertificate() {
  return request.post("/api/certificate/root/install", {});
}

export function uninstallRootCertificate() {
  return request.post("/api/certificate/root/uninstall", {});
}

export function fetchAppConfig() {
  return request.get("/api/admin/config");
}

export function fetchDownloadAppConfig() {
  return request.get("/api/admin/config");
}

/** @param {{ values: Record<string, any> }} body */
export function updateAppConfig(body) {
  return request.post("/api/admin/config", body);
}

/** @param {{ URL?: string; url?: string; Filename?: string; filename?: string; Dir?: string; dir?: string; Extra?: Record<string, string>; extra?: Record<string, string> }} body */
export function createDownloadTask(body) {
  const url = body.url || body.URL || "";
  const filename = body.filename || body.Filename || "";
  const dir = body.dir || body.Dir || "";
  const extra = body.extra || body.Extra || {};
  return request.post("/api/task/create2", {
    URL: url,
    Filename: filename,
    Dir: dir,
    Extra: extra,
  });
}

/** @param {{ url: string; cover?: boolean }} body */
export function createTask(body) {
  return request.post("/api/task/create", {
    url: body.url,
    cover: !!body.cover,
  });
}

/** @param {{ download_task_id: number }} body */
export function startDownloadTask(body) {
  return request.post("/api/download_task/start", body);
}

/** @param {{ id: number }} body */
export function retryDownloadTask(body) {
  return request.post("/api/download_task/retry", body);
}

/** @param {{ download_task_id: number }} body */
export function deleteDownloadTask(body) {
  return request.post("/api/download_task/delete", body);
}

/** @param {{ file_path: string }} body */
export function highlightDownloadFile(body) {
  return request.post("/api/download_task/highlight_file", body);
}

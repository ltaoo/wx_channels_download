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
  return request.get("/api/mock/downloads", params);
}

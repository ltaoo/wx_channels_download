const feed_search_endpoint = "https://cloud.feedly.com/v3/search/feeds";
const search_result_limit = 36;
const preview_byte_limit = 512 * 1024;
const upstream_timeout_ms = 10000;

/**
 * @param {string} value
 * @returns {boolean}
 */
export function is_public_feed_url(value) {
  if (typeof value !== "string" || value.length === 0 || value.length > 2048) {
    return false;
  }
  let url;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return false;
  }
  if (url.username !== "" || url.password !== "") {
    return false;
  }
  const hostname = url.hostname.toLowerCase();
  if (hostname === "localhost" || hostname.endsWith(".localhost") || hostname.endsWith(".local")) {
    return false;
  }
  if (hostname.includes(":")) {
    return false;
  }
  const ipv4 = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(hostname);
  if (ipv4) {
    const [first, second] = ipv4.slice(1, 3).map(Number);
    if (first === 0 || first === 10 || first === 127) return false;
    if (first === 169 && second === 254) return false;
    if (first === 172) return second < 16 || second > 31;
    if (first === 192) return second !== 168;
    return true;
  }
  return true;
}

/**
 * @param {unknown} payload
 * @param {number} status
 * @returns {Response}
 */
function json_response(payload, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Cache-Control": status === 200 ? "public, max-age=300" : "no-store",
      "Content-Type": "application/json; charset=utf-8",
    },
  });
}

/** @param {string} message */
function api_error(message) {
  return json_response({ error: message }, 400);
}

/** @param {Response} response */
function secure_response(response) {
  const headers = new Headers(response.headers);
  headers.set("Content-Security-Policy", "default-src 'none'; img-src https: data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'");
  headers.set("Referrer-Policy", "no-referrer");
  headers.set("X-Content-Type-Options", "nosniff");
  headers.set("X-Frame-Options", "DENY");
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

/** @param {string} query */
function search_url(query) {
  const url = new URL(feed_search_endpoint);
  url.searchParams.set("query", query);
  url.searchParams.set("count", String(search_result_limit));
  return url;
}

/**
 * @param {unknown} payload
 * @returns {Array<Record<string, unknown>>}
 */
function normalize_search_results(payload) {
  const results = Array.isArray(payload?.results) ? payload.results : [];
  return results.slice(0, search_result_limit).map((item) => {
    const feed_url = typeof item?.feedId === "string" && item.feedId.startsWith("feed/")
      ? item.feedId.slice(5)
      : "";
    return {
      title: String(item?.title || feed_url || "未命名订阅源"),
      description: String(item?.description || "").slice(0, 500),
      website: String(item?.website || ""),
      feed_url,
      language: String(item?.language || ""),
      topics: Array.isArray(item?.topics) ? item.topics.map(String).slice(0, 6) : [],
      subscribers: Number(item?.subscribers || 0),
      velocity: Number(item?.velocity || 0),
      last_updated: Number(item?.lastUpdated || 0),
    };
  }).filter((item) => is_public_feed_url(String(item.feed_url)));
}

/** @param {Request} request */
async function search_feeds(request) {
  const query = new URL(request.url).searchParams.get("q")?.trim().slice(0, 100) || "";
  if (query.length < 1) {
    return api_error("请输入搜索关键词");
  }
  const response = await fetch(search_url(query), {
    headers: {
      Accept: "application/json",
      "User-Agent": "wx-channels-bridge-discovery/1.0",
    },
    signal: AbortSignal.timeout(upstream_timeout_ms),
  });
  if (!response.ok) {
    return json_response({ error: "订阅源搜索服务暂时不可用" }, 502);
  }
  const payload = await response.json();
  return json_response({ query, results: normalize_search_results(payload) });
}

/** @param {Request} request */
async function preview_feed(request) {
  const feed_url = new URL(request.url).searchParams.get("url") || "";
  if (!is_public_feed_url(feed_url)) {
    return api_error("只支持公网 HTTP(S) RSS/Atom 地址");
  }
  const response = await fetch(feed_url, {
    headers: {
      Accept: "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.1",
      "User-Agent": "wx-channels-bridge-discovery/1.0",
    },
    redirect: "follow",
    signal: AbortSignal.timeout(upstream_timeout_ms),
  });
  if (!response.ok || !is_public_feed_url(response.url)) {
    return json_response({ error: "订阅源读取失败" }, 502);
  }
  const body = await response.arrayBuffer();
  if (body.byteLength > preview_byte_limit) {
    return json_response({ error: "订阅源超过 512KB 预览限制" }, 413);
  }
  const xml = new TextDecoder().decode(body);
  const document_start = xml.slice(0, 2048).toLowerCase();
  if (!document_start.includes("<rss") && !document_start.includes("<feed") && !document_start.includes("<rdf:rdf")) {
    return json_response({ error: "目标地址不是 RSS/Atom 文档" }, 415);
  }
  return new Response(body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": "application/xml; charset=utf-8",
      "X-Content-Type-Options": "nosniff",
    },
  });
}

/** @type {ExportedHandler<{ ASSETS: Fetcher }>} */
export default {
  async fetch(request, env) {
    const pathname = new URL(request.url).pathname;
    if (request.method !== "GET") {
      return json_response({ error: "Method Not Allowed" }, 405);
    }
    if (pathname === "/api/search") {
      return search_feeds(request);
    }
    if (pathname === "/api/preview") {
      return preview_feed(request);
    }
    return secure_response(await env.ASSETS.fetch(request));
  },
};

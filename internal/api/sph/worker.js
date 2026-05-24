// Cloudflare Worker — fetch_video_profile_with_share_url
// 对应 fetch_video_profile.go

import indexHtml from "./index.html";
import iconBase64 from "./icon.js";

function base64ToBytes(base64) {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // CORS preflight
    if (request.method === "OPTIONS") {
      return new Response(null, {
        headers: corsHeaders(),
      });
    }

    // GET /favicon.ico or /icon.png → serve icon
    if ((url.pathname === "/favicon.ico" || url.pathname === "/icon.png") && request.method === "GET") {
      return new Response(base64ToBytes(iconBase64), {
        headers: { "Content-Type": "image/png" },
      });
    }

    // GET / → serve index.html
    if (url.pathname === "/" && request.method === "GET") {
      return new Response(indexHtml, {
        headers: { "Content-Type": "text/html; charset=utf-8" },
      });
    }

    // POST /api/fetch_video_profile
    if (url.pathname === "/api/fetch_video_profile" && request.method === "POST") {
      return handleFetchVideoProfile(request, env);
    }

    // 其他请求返回 404
    return new Response("not found", { status: 404 });
  },
};

function corsHeaders() {
  return {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  };
}

function log(...args) {
  console.log(`[${new Date().toISOString()}]`, ...args);
}

// ---- Step 1: parse share URL ----

const PARSE_URL = "https://yuanbao.tencent.com/api/weixin/get_parse_result";

const PARSE_HEADERS = {
  "accept": "application/json, text/plain, */*",
  "accept-language": "zh-CN,zh;q=0.9,en;q=0.8",
  "content-type": "application/json",
  "origin": "https://yuanbao.tencent.com",
  "referer": "https://yuanbao.tencent.com/chat/naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1",
  "user-agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
  "sec-ch-ua": `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`,
  "sec-ch-ua-mobile": "?0",
  "sec-ch-ua-platform": `"macOS"`,
  "sec-fetch-dest": "empty",
  "sec-fetch-mode": "cors",
  "sec-fetch-site": "same-origin",
  "t-userid": "b9575f6b0a8c4a55a08096904a5ef20a",
  "x-agentid": "naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1",
  "x-commit-tag": "72282a0d",
  "x-device-id": "1921b001708100d7fa31002b9646bd0cc15a3e2e1f",
  "x-hy106": "",
  "x-hy92": "e963067ffa31002b9646bd0c03000008b1951a",
  "x-hy93": "1921b001708100d7fa31002b9646bd0cc15a3e2e1f",
  "x-id": "b9575f6b0a8c4a55a08096904a5ef20a",
  "x-instance-id": "5",
  "x-language": "zh-CN",
  "x-os_version": "Mac OS(10.15.7)-Blink",
  "x-platform": "mac",
  "x-requested-with": "XMLHttpRequest",
  "x-source": "web",
  "x-web-third-source": "main",
  "x-webdriver": "0",
  "x-webversion": "2.69.0",
  "x-ybuitest": "0",
};

async function parseShareUrl(shareUrl, cookie) {
  log("[parseShareUrl] start, url:", shareUrl);
  const payload = JSON.stringify({
    type: "video_channel_url",
    url: shareUrl,
    scene: 1,
  });
  const resp = await fetch(PARSE_URL, {
    method: "POST",
    headers: { ...PARSE_HEADERS, cookie },
    body: payload,
  });
  if (!resp.ok) {
    log("[parseShareUrl] http request failed, status:", resp.status);
    throw new Error(`parseShareUrl: http ${resp.status}`);
  }
  const result = await resp.json();
  if (!result.data || !result.data.wx_export_id) {
    log("[parseShareUrl] missing wx_export_id in response");
    throw new Error("parseShareUrl: missing wx_export_id");
  }
  log("[parseShareUrl] success, exportId:", result.data.wx_export_id);
  return result.data;
}

// ---- Step 2: get feed info ----

const FEED_INFO_URL =
  "https://channels.weixin.qq.com/finder-preview/api/feed/get_feed_info";

// Yg = zg() + "-" + Gg()
function generateRid() {
  const timestampHex = Math.floor(Date.now() / 1000).toString(16);
  let randomHex = "";
  const chars = "0123456789abcdef";
  for (let i = 0; i < 8; i++) {
    randomHex += chars[Math.floor(Math.random() * 16)];
  }
  return `${timestampHex}-${randomHex}`;
}

const FEED_INFO_HEADERS = {
  "Accept": "application/json, text/plain, */*",
  "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
  "Connection": "keep-alive",
  "Content-Type": "application/json",
  "Origin": "https://channels.weixin.qq.com",
  "Sec-Fetch-Dest": "empty",
  "Sec-Fetch-Mode": "cors",
  "Sec-Fetch-Site": "same-origin",
  "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
  "sec-ch-ua": `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`,
  "sec-ch-ua-mobile": "?0",
  "sec-ch-ua-platform": `"macOS"`,
};

async function getFeedInfo(exportId, generalToken) {
  log("[getFeedInfo] start, exportId:", exportId, "generalToken:", generalToken);
  const rid = generateRid();
  const payload = JSON.stringify({
    baseReq: { generalToken },
    exportId,
  });
  const apiUrl = `${FEED_INFO_URL}?_rid=${rid}&_pageUrl=https:%2F%2Fchannels.weixin.qq.com%2Ffinder-preview%2Fpages%2Ffeed`;

  const referer =
    `https://channels.weixin.qq.com/finder-preview/pages/feed` +
    `?entry_card_type=48&comment_scene=39&appid=0` +
    `&token=${encodeURIComponent(generalToken)}` +
    `&entry_scene=0&eid=${encodeURIComponent(exportId)}`;

  const resp = await fetch(apiUrl, {
    method: "POST",
    headers: { ...FEED_INFO_HEADERS, Referer: referer },
    body: payload,
  });
  if (!resp.ok) {
    log("[getFeedInfo] http request failed, status:", resp.status);
    throw new Error(`getFeedInfo: http ${resp.status}`);
  }
  const result = await resp.json();
  log("[getFeedInfo] success, errCode:", result.errCode);
  return result;
}

// ---- combined ----

async function fetchVideoProfile(shareUrl, cookie) {
  log("[fetch] start, shareUrl:", shareUrl);

  // Step 1: parse share URL → get parse data
  log("[fetch] step 1/2: parseShareUrl...");
  let parseData;
  try {
    parseData = await parseShareUrl(shareUrl, cookie);
  } catch (err) {
    log("[fetch] step 1/2 failed:", err.message);
    throw new Error(`parse share url: ${err.message}`);
  }
  log("[fetch] step 1/2 done, exportId:", parseData.wx_export_id);

  // extract generalToken and exportId from playable_url query params
  let generalToken = "";
  let exportId = "";
  try {
    const playableUrl = new URL(parseData.playable_url);
    generalToken = playableUrl.searchParams.get("token") || "";
    exportId = playableUrl.searchParams.get("eid") || "";
  } catch (_) {
    // ignore parse error
  }
  if (!generalToken) {
    log("[fetch] warn: generalToken is empty in playable_url");
  }
  if (!exportId) {
    log("[fetch] warn: exportId (eid) is empty in playable_url");
  }
  log("[fetch] generalToken:", generalToken, "exportId:", exportId);

  // Step 2: get feed info by export ID
  log("[fetch] step 2/2: getFeedInfo...");
  let feedResult;
  try {
    feedResult = await getFeedInfo(exportId, generalToken);
  } catch (err) {
    log("[fetch] step 2/2 failed:", err.message);
    throw new Error(`get feed info: ${err.message}`);
  }
  log("[fetch] step 2/2 done");
  log("[fetch] all done");
  return feedResult;
}

// ---- request handler ----

async function handleFetchVideoProfile(request, env) {
  try {
    const body = await request.json();
    const shareUrl = body.url;
    if (!shareUrl) {
      return new Response(
        JSON.stringify({ error: "missing url" }),
        {
          status: 400,
          headers: { ...corsHeaders(), "Content-Type": "application/json" },
        }
      );
    }
    const result = await fetchVideoProfile(shareUrl, env.COOKIE);
    return new Response(JSON.stringify(result), {
      status: 200,
      headers: { ...corsHeaders(), "Content-Type": "application/json" },
    });
  } catch (err) {
    log("[handleFetchVideoProfile] error:", err.message);
    return new Response(
      JSON.stringify({ error: err.message }),
      {
        status: 500,
        headers: { ...corsHeaders(), "Content-Type": "application/json" },
      }
    );
  }
}

// Cloudflare Worker — fetch_video_profile_with_share_url
// Corresponds to fetch_video_profile.go

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

    // POST /api/download_feed_zip
    if (url.pathname === "/api/download_feed_zip" && request.method === "POST") {
      return handleDownloadFeedZip(request);
    }

    // Return 404 for all other requests
    return new Response("not found", { status: 404 });
  },
};

function corsHeaders() {
  return {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
    "Access-Control-Expose-Headers": "Content-Disposition",
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

async function handleDownloadFeedZip(request) {
  try {
    const body = await request.json();
    const feed = body.feed || body;
    const files = extractFeedZipFiles(feed);
    if (files.length === 0) {
      return new Response(
        JSON.stringify({ error: "no downloadable picture or bgm found" }),
        {
          status: 400,
          headers: { ...corsHeaders(), "Content-Type": "application/json" },
        }
      );
    }

    const entries = [];
    entries.push({
      name: "info.json",
      data: new TextEncoder().encode(JSON.stringify(feed, null, 2)),
    });

    for (const file of files) {
      const resp = await fetch(file.url, {
        headers: {
          "Accept": "*/*",
        },
      });
      if (!resp.ok) {
        throw new Error(`${file.name}: http ${resp.status}`);
      }
      const contentType = resp.headers.get("Content-Type") || "";
      const data = new Uint8Array(await resp.arrayBuffer());
      entries.push({
        name: ensureFileExtension(file.name, file.url, contentType),
        data,
      });
    }

    const zip = buildZip(entries);
    const filename = zipFilename(feed);
    return new Response(zip, {
      status: 200,
      headers: {
        ...corsHeaders(),
        "Content-Type": "application/zip",
        "Content-Disposition": `attachment; filename="channels_feed.zip"; filename*=UTF-8''${encodeURIComponent(filename)}`,
      },
    });
  } catch (err) {
    log("[handleDownloadFeedZip] error:", err.message);
    return new Response(
      JSON.stringify({ error: err.message }),
      {
        status: 500,
        headers: { ...corsHeaders(), "Content-Type": "application/json" },
      }
    );
  }
}

function extractFeedZipFiles(feed) {
  const data = sharedFeedData(feed);
  const feedInfo = data && data.feedInfo ? data.feedInfo : {};
  const files = [];
  const picInfo = Array.isArray(feedInfo.picInfo) ? feedInfo.picInfo : [];
  picInfo.forEach((pic, index) => {
    const url = pic && typeof pic.url === "string" ? pic.url.trim() : "";
    if (!url) return;
    files.push({
      url,
      name: `${String(index + 1).padStart(2, "0")}.jpg`,
    });
  });
  const bgmInfo = feedInfo.bgmInfo || {};
  const bgmUrl = String(bgmInfo.bgmUrl || bgmInfo.mediaStreamingUrl || "").trim();
  if (bgmUrl) {
    const bgmName = sanitizeZipEntryName(bgmInfo.name || "bgm").replace(/\.[a-z0-9]{2,5}$/i, "");
    files.push({
      url: bgmUrl,
      name: (bgmName || "bgm") + ".mp3",
    });
  }
  return files;
}

function zipFilename(feed) {
  const data = sharedFeedData(feed);
  const feedInfo = data && data.feedInfo ? data.feedInfo : {};
  const base = baseFilename(feedInfo.description, feedInfo.createtime) || "channels_feed";
  return base + ".zip";
}

function sharedFeedData(feed) {
  if (feed && feed.data && feed.data.feedInfo) return feed.data;
  if (feed && feed.data && feed.data.data && feed.data.data.feedInfo) return feed.data.data;
  if (feed && feed.feedInfo) return feed;
  return {};
}

function baseFilename(desc, createtime) {
  if (desc) return sanitizeZipEntryName(desc).slice(0, 160);
  if (createtime) {
    const d = new Date(Number(createtime) * 1000);
    const pad = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}_${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
  }
  return "";
}

function ensureFileExtension(name, url, contentType) {
  const cleanName = sanitizeZipEntryName(name) || "file";
  if (/\.[a-z0-9]{2,5}$/i.test(cleanName)) {
    return cleanName;
  }
  const ext = extensionFromContentType(contentType) || extensionFromURL(url) || ".bin";
  return cleanName + ext;
}

function extensionFromContentType(contentType) {
  const value = String(contentType || "").split(";")[0].trim().toLowerCase();
  const map = {
    "image/jpeg": ".jpg",
    "image/png": ".png",
    "image/webp": ".webp",
    "image/gif": ".gif",
    "audio/mpeg": ".mp3",
    "audio/mp4": ".m4a",
    "audio/aac": ".aac",
    "application/json": ".json",
  };
  return map[value] || "";
}

function extensionFromURL(rawURL) {
  try {
    const ext = new URL(rawURL).pathname.match(/\.[a-z0-9]{2,5}$/i);
    return ext ? ext[0].toLowerCase() : "";
  } catch (_) {
    return "";
  }
}

function sanitizeZipEntryName(name) {
  return String(name || "")
    .replace(/[\\/:*?"<>|]/g, "_")
    .replace(/\s+/g, " ")
    .trim();
}

const crcTable = (() => {
  const table = new Uint32Array(256);
  for (let i = 0; i < 256; i += 1) {
    let c = i;
    for (let k = 0; k < 8; k += 1) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[i] = c >>> 0;
  }
  return table;
})();

function crc32(data) {
  let crc = 0xffffffff;
  for (let i = 0; i < data.length; i += 1) {
    crc = crcTable[(crc ^ data[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function buildZip(entries) {
  const encoder = new TextEncoder();
  const chunks = [];
  const central = [];
  let offset = 0;
  const now = new Date();
  const dosTime =
    (now.getHours() << 11) | (now.getMinutes() << 5) | Math.floor(now.getSeconds() / 2);
  const dosDate =
    ((now.getFullYear() - 1980) << 9) | ((now.getMonth() + 1) << 5) | now.getDate();

  for (const entry of entries) {
    const nameBytes = encoder.encode(entry.name);
    const data = entry.data instanceof Uint8Array ? entry.data : new Uint8Array(entry.data);
    const crc = crc32(data);
    const local = new Uint8Array(30 + nameBytes.length);
    const view = new DataView(local.buffer);
    writeZipHeader(view, 0x04034b50, 20, 0, 0, dosTime, dosDate, crc, data.length, data.length, nameBytes.length, 0);
    local.set(nameBytes, 30);
    chunks.push(local, data);

    const centralHeader = new Uint8Array(46 + nameBytes.length);
    const centralView = new DataView(centralHeader.buffer);
    centralView.setUint32(0, 0x02014b50, true);
    centralView.setUint16(4, 20, true);
    centralView.setUint16(6, 20, true);
    centralView.setUint16(8, 0, true);
    centralView.setUint16(10, 0, true);
    centralView.setUint16(12, dosTime, true);
    centralView.setUint16(14, dosDate, true);
    centralView.setUint32(16, crc, true);
    centralView.setUint32(20, data.length, true);
    centralView.setUint32(24, data.length, true);
    centralView.setUint16(28, nameBytes.length, true);
    centralView.setUint16(30, 0, true);
    centralView.setUint16(32, 0, true);
    centralView.setUint16(34, 0, true);
    centralView.setUint16(36, 0, true);
    centralView.setUint32(38, 0, true);
    centralView.setUint32(42, offset, true);
    centralHeader.set(nameBytes, 46);
    central.push(centralHeader);
    offset += local.length + data.length;
  }

  const centralOffset = offset;
  const centralSize = central.reduce((sum, item) => sum + item.length, 0);
  const end = new Uint8Array(22);
  const endView = new DataView(end.buffer);
  endView.setUint32(0, 0x06054b50, true);
  endView.setUint16(8, entries.length, true);
  endView.setUint16(10, entries.length, true);
  endView.setUint32(12, centralSize, true);
  endView.setUint32(16, centralOffset, true);
  return concatUint8Arrays([...chunks, ...central, end]);
}

function writeZipHeader(view, signature, version, flags, method, time, date, crc, compressedSize, size, nameLength, extraLength, offset = 0) {
  if (signature) view.setUint32(offset, signature, true);
  view.setUint16(offset + 4, version, true);
  view.setUint16(offset + 6, flags, true);
  view.setUint16(offset + 8, method, true);
  view.setUint16(offset + 10, time, true);
  view.setUint16(offset + 12, date, true);
  view.setUint32(offset + 14, crc, true);
  view.setUint32(offset + 18, compressedSize, true);
  view.setUint32(offset + 22, size, true);
  view.setUint16(offset + 26, nameLength, true);
  view.setUint16(offset + 28, extraLength, true);
}

function concatUint8Arrays(parts) {
  const size = parts.reduce((sum, part) => sum + part.length, 0);
  const out = new Uint8Array(size);
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

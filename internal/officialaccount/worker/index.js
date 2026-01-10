// Cloudflare Worker for Official Account API
// Simplified version of the Go implementation

// Response utilities
function apiResponse(code, msg, data, status = 200) {
  return new Response(
    JSON.stringify({
      code,
      msg,
      data,
    }),
    {
      status,
      headers: {
        "Content-Type": "application/json",
        "Access-Control-Allow-Origin": "*",
        "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
        "Access-Control-Allow-Headers": "Content-Type",
      },
    }
  );
}


const Result = {
  Ok(data = {}, msg = "") {
    return apiResponse(0, msg, data, 200);
  },
  Err(code, message, status = 400) {
    return apiResponse(code, message, {}, status);
  },
};
// Global cache for tokens
const TOKEN_CACHE = new Set();

// Load tokens from DB to cache
async function loadTokens(env) {
  if (TOKEN_CACHE !== null) return;

  try {
    // Create table if not exists (in case migration didn't run)
    // In production, migrations should be run via wrangler, but for safety:
    // await env.DB.prepare(`CREATE TABLE IF NOT EXISTS tokens (token TEXT PRIMARY KEY, description TEXT, created_at INTEGER)`).run();

    const { results } = await env.DB.prepare("SELECT token FROM tokens").all();
    if (results && results.length > 0) {
      results.forEach((row) => TOKEN_CACHE.add(row.token));
    }
    console.log(`Loaded ${TOKEN_CACHE.size} tokens from DB`);
  } catch (error) {
    console.error("Error loading tokens:", error);
    // Fallback to empty set if DB fails, so we don't crash, but auth might fail for DB tokens
  }
}

// Token validation
function validateToken(token) {
  if (!token) return false;
  // Check environment variables first (Superuser/Admin tokens)
  // Initialize cache if needed
  // Check cache
  return TOKEN_CACHE.has(token);
}

// Fetch messages from WeChat API
async function getMsgList(account, offset = 0) {
  const params = new URLSearchParams({
    action: "getmsg",
    __biz: account.biz,
    uin: account.uin,
    key: account.key,
    pass_ticket: account.pass_ticket,
    wxtoken: "",
    x5: "0",
    count: "10",
    offset: offset.toString(),
    f: "json",
  });

  const url = `https://mp.weixin.qq.com/mp/profile_ext?${params.toString()}`;

  // Construct Referer
  const refererParams = new URLSearchParams({
    action: "home",
    __biz: account.biz,
    scene: "124",
    uin: account.uin,
    key: account.key,
    devicetype: "UnifiedPCWindows",
    version: "f2541022",
    lang: "zh_CN",
    a8scene: "1",
    acctmode: "0",
    pass_ticket: account.pass_ticket,
  });
  const referer = `https://mp.weixin.qq.com/mp/profile_ext?${refererParams.toString()}`;

  const response = await fetch(url, {
    method: "GET",
    headers: {
      "content-type": "application/json",
      "accept-language": "en-US,en;q=0.9",
      priority: "u=1, i",
      "sec-fetch-dest": "empty",
      "sec-fetch-mode": "cors",
      "sec-fetch-site": "same-origin",
      "user-agent":
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue",
      referer: referer,
    },
  });

  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }

  return await response.json();
}

// Generate RSS feed from messages
function generateRSS(officialAccount, messages) {
  const now = new Date().toUTCString();
  let rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>${officialAccount.nickname || officialAccount.biz}</title>
    <link>https://mp.weixin.qq.com/mp/profile_ext?action=home&amp;__biz=${
      officialAccount.biz
    }</link>
    <description>公众号文章 RSS</description>
    <language>zh-CN</language>
    <lastBuildDate>${now}</lastBuildDate>
    <generator>WeChat Official Account RSS</generator>`;

  if (messages && messages.length > 0) {
    messages.forEach((msg) => {
      const pubDate = new Date(msg.create_time * 1000).toUTCString();
      rss += `
    <item>
      <title><![CDATA[${msg.title || "无标题"}]]></title>
      <link>${msg.content_url || "#"}</link>
      <description><![CDATA[${msg.digest || ""}]]></description>
      <pubDate>${pubDate}</pubDate>
      <guid>${msg.content_url || msg.id}</guid>
    </item>`;
    });
  }

  rss += `</channel>
</rss>`;
  return rss;
}

// API Handlers
async function handleFetchOfficialAccountList(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token");
  const page = Math.max(
    1,
    parseInt(url.searchParams.get("page") || "1", 10) || 1
  );
  let pageSize = parseInt(url.searchParams.get("page_size") || "10", 10) || 10;
  if (pageSize <= 0) pageSize = 10;
  if (pageSize > 200) pageSize = 200;
  const keyword = (url.searchParams.get("keyword") || "").trim();

  if (!validateToken(token)) {
    return Result.Err(1001, "Invalid token");
  }

  try {
    const where = keyword
      ? "WHERE (LOWER(nickname) LIKE ? OR LOWER(biz) LIKE ?)"
      : "";
    const kw = keyword ? `%${keyword.toLowerCase()}%` : "";
    const countStmt = env.DB.prepare(
      `SELECT COUNT(*) as total FROM accounts ${where}`
    );
    const countRow = keyword
      ? await countStmt.bind(kw, kw).first()
      : await countStmt.first();
    const total = countRow ? countRow.total || 0 : 0;

    const offset = (page - 1) * pageSize;
    const listStmt = env.DB.prepare(
      `SELECT * FROM accounts ${where} ORDER BY update_time DESC LIMIT ? OFFSET ?`
    );
    const { results } = keyword
      ? await listStmt.bind(kw, kw, pageSize, offset).all()
      : await listStmt.bind(pageSize, offset).all();

    // Filter effective accounts and format response
    const now = Math.floor(Date.now() / 1000);
    const list = results.map((account) => {
      const isEffective =
        account.is_effective === 1 && now - account.update_time < 30 * 60;
      return {
        nickname: account.nickname,
        avatar_url: account.avatar_url,
        biz: account.biz,
        refresh_uri: account.refresh_uri,
        is_effective: isEffective,
        created_at: account.created_at,
        update_time: account.update_time,
      };
    });

    return Result.Ok({ list, total, page, page_size: pageSize, keyword });
  } catch (error) {
    console.error("Error fetching account list:", error);
    return Result.Err(500, "Internal server error");
  }
}

async function handleFetchOfficialAccountMsgList(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token");
  const biz = url.searchParams.get("biz");
  const offset = parseInt(url.searchParams.get("offset") || "0");

  if (!validateToken(token)) {
    return Result.Err(1001, "Invalid token");
  }

  if (!biz) {
    return Result.Err(1002, "Missing biz parameter");
  }

  try {
    // Get account info
    const account = await env.DB.prepare(
      "SELECT nickname FROM accounts WHERE biz = ?"
    )
      .bind(biz)
      .first();

    if (!account) {
      return Result.Err(1003, "Account not found");
    }

    // Get messages count
    const countResult = await env.DB.prepare(
      "SELECT COUNT(*) as total FROM messages WHERE biz = ?"
    )
      .bind(biz)
      .first();
    const total = countResult ? countResult.total : 0;

    // Get messages for this account
    const pageSize = 10;
    const { results } = await env.DB.prepare(
      "SELECT raw_json FROM messages WHERE biz = ? ORDER BY create_time DESC LIMIT ? OFFSET ?"
    )
      .bind(biz, pageSize, offset)
      .all();

    const paginatedMessages = results.map((row) => {
      try {
        return JSON.parse(row.raw_json);
      } catch (e) {
        return {};
      }
    });

    return Result.Ok({
      msg_count: total,
      title: account.nickname,
      list: paginatedMessages,
    });
  } catch (error) {
    console.error("Error fetching message list:", error);
    return Result.Err(500, "Internal server error");
  }
}

async function handleRefreshOfficialAccountEvent(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token");

  if (token !== env.REFRESH_TOKEN) {
    return Result.Err(1001, "Invalid refresh token");
  }

  try {
    const body = await request.json();

    if (
      !body.biz ||
      !body.key ||
      !body.nickname ||
      !body.uin ||
      !body.pass_ticket ||
      !body.refresh_uri
    ) {
      return Result.Err(
        1002,
        "Missing required parameters: biz, key, nickname, uin, pass_ticket, refresh_uri"
      );
    }

    // Update or create account
    const {
      nickname = "",
      avatar_url = "",
      uin = "",
      key,
      pass_ticket = "",
      appmsg_token = "",
      refresh_uri = "",
      created_at = 0,
    } = body;

    const update_time = Math.floor(Date.now() / 1000);
    const create_time = created_at || update_time;

    await env.DB.prepare(
      `INSERT INTO accounts (biz, nickname, avatar_url, uin, key, pass_ticket, appmsg_token, refresh_uri, is_effective, created_at, update_time, error)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, '')
       ON CONFLICT(biz) DO UPDATE SET
       nickname=excluded.nickname,
       avatar_url=excluded.avatar_url,
       uin=excluded.uin,
       key=excluded.key,
       pass_ticket=excluded.pass_ticket,
       appmsg_token=excluded.appmsg_token,
       refresh_uri=excluded.refresh_uri,
       is_effective=1,
       update_time=excluded.update_time,
       error=''`
    )
      .bind(
        body.biz,
        nickname,
        avatar_url,
        uin,
        key,
        pass_ticket,
        appmsg_token,
        refresh_uri,
        create_time,
        update_time
      )
      .run();

    return Result.Ok();
  } catch (error) {
    console.error("Error refreshing account:", error);
    return Result.Err(500, "Internal server error");
  }
}

async function handleFetchMsgListOfOfficialAccountRSS(request, env) {
  const url = new URL(request.url);
  const biz = url.searchParams.get("biz");

  if (!biz) {
    return new Response("Missing biz parameter", { status: 400 });
  }

  try {
    // Get account info
    const account = await env.DB.prepare("SELECT * FROM accounts WHERE biz = ?")
      .bind(biz)
      .first();

    if (!account) {
      return new Response("Account not found", { status: 404 });
    }

    let messages = [];
    let fetchError = null;

    // Try to fetch fresh messages
    try {
      const data = await getMsgList(account, 0);

      if (data.ret === 0) {
        const listData = JSON.parse(data.general_msg_list);
        if (listData.list && listData.list.length > 0) {
          listData.list.forEach((item) => {
            const msg = item.app_msg_ext_info;
            const common = item.comm_msg_info;

            // Add main message
            messages.push({
              title: msg.title,
              digest: msg.digest,
              content_url: msg.content_url,
              cover: msg.cover,
              create_time: common.datetime,
              id: common.id,
            });

            // Add sub-messages (multi-app messages)
            if (
              msg.is_multi === 1 &&
              msg.multi_app_msg_item_list &&
              msg.multi_app_msg_item_list.length > 0
            ) {
              msg.multi_app_msg_item_list.forEach((art) => {
                messages.push({
                  title: art.title,
                  digest: art.digest,
                  content_url: art.content_url,
                  cover: art.cover,
                  create_time: common.datetime,
                  id: art.fileid || common.id,
                });
              });
            }
          });
        }
      } else if (data.ret === -3) {
        // Session expired, mark as ineffective
        await env.DB.prepare(
          "UPDATE accounts SET is_effective = 0 WHERE biz = ?"
        )
          .bind(biz)
          .run();
        console.log(`Account ${biz} expired`);
      } else {
        console.error(`WeChat API error: ${data.errmsg} (ret: ${data.ret})`);
      }
    } catch (e) {
      console.error("Error fetching from WeChat:", e);
      fetchError = e;
    }

    // Fallback to DB if no messages fetched or error occurred
    if (messages.length === 0) {
      const { results } = await env.DB.prepare(
        "SELECT raw_json FROM messages WHERE biz = ? ORDER BY create_time DESC LIMIT 20"
      )
        .bind(biz)
        .all();

      const dbMessages = results.map((row) => {
        try {
          return JSON.parse(row.raw_json);
        } catch (e) {
          return {};
        }
      });

      if (dbMessages.length > 0) {
        messages = dbMessages;
      }
    }

    // Generate RSS feed
    const rss = generateRSS(account, messages);

    return new Response(rss, {
      headers: {
        "Content-Type": "application/rss+xml; charset=utf-8",
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch (error) {
    console.error("Error generating RSS:", error);
    return new Response("Internal server error", { status: 500 });
  }
}

async function handleOfficialAccountProxy(request, env) {
  const url = new URL(request.url);
  const targetURL = url.searchParams.get("url");
  const token = url.searchParams.get("token");

  if (!validateToken(token)) {
    return Result.Err(1001, "Invalid token");
  }

  if (!targetURL) {
    return Result.Err(1002, "Missing url parameter");
  }

  try {
    // Decode HTML entities in URL
    const decodedURL = targetURL.replace(/&amp;/g, "&");

    // Create proxy request
    const proxyRequest = new Request(decodedURL, {
      method: "GET",
      headers: {
        accept:
          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
        "accept-language": "zh-CN,zh;q=0.9",
        priority: "u=0, i",
        "sec-ch-ua":
          '"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"',
        "sec-ch-ua-mobile": "?0",
        "sec-ch-ua-platform": '"macOS"',
        "sec-fetch-dest": "document",
        "sec-fetch-mode": "navigate",
        "sec-fetch-site": "none",
        "sec-fetch-user": "?1",
        "upgrade-insecure-requests": "1",
        "user-agent":
          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36",
      },
    });

    const response = await fetch(proxyRequest);

    // Return the response with appropriate headers
    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: {
        "Content-Type": response.headers.get("Content-Type") || "text/html",
        "Access-Control-Allow-Origin": "*",
      },
    });
  } catch (error) {
    console.error("Proxy error:", error);
    return Result.Err(500, "Proxy request failed");
  }
}

async function handleAddToken(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token");

  // Only Admin/Auth Token can add new tokens
  if (!token || token !== env.ADMIN_TOKEN) {
    return Result.Err(1001, "Unauthorized: Only admin can add tokens");
  }

  try {
    const body = await request.json();
    const { token, description = "" } = body;

    if (!token) {
      return Result.Err(1002, "Missing token parameter");
    }

    const now = Math.floor(Date.now() / 1000);

    // Insert into DB
    await env.DB.prepare(
      `INSERT INTO tokens (token, description, created_at) VALUES (?, ?, ?)`
    )
      .bind(token, description, now)
      .run();

    // Update cache
    if (TOKEN_CACHE === null) {
      await loadTokens(env);
    } else {
      TOKEN_CACHE.add(token);
    }

    return Result.Ok({ token }, "Token added successfully");
  } catch (error) {
    if (error.message && error.message.includes("UNIQUE constraint failed")) {
      return Result.Err(1003, "Token already exists");
    }
    console.error("Error adding token:", error);
    return Result.Err(500, "Internal server error");
  }
}

async function handleDeleteToken(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token");

  // Only Admin/Auth Token can delete tokens
  if (!token || token !== env.ADMIN_TOKEN) {
    return Result.Err(1001, "Unauthorized: Only admin can delete tokens");
  }

  try {
    const body = await request.json();
    const { token } = body;

    if (!token) {
      return Result.Err(1002, "Missing token parameter");
    }

    // Delete from DB
    await env.DB.prepare(`DELETE FROM tokens WHERE token = ?`)
      .bind(token)
      .run();

    // Update cache
    if (TOKEN_CACHE !== null) {
      TOKEN_CACHE.delete(token);
    }

    return Result.Ok({ token }, "Token deleted successfully");
  } catch (error) {
    console.error("Error deleting token:", error);
    return Result.Err(500, "Internal server error");
  }
}


// Main request handler
export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;
    const method = request.method;

    // Handle CORS preflight
    if (method === "OPTIONS") {
      return new Response(null, {
        status: 200,
        headers: {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
          "Access-Control-Allow-Headers": "Content-Type",
        },
      });
    }

    // Route handling
    try {
      if (path === "/api/mp/list" && method === "GET") {
        return await handleFetchOfficialAccountList(request, env);
      }

      if (path === "/api/mp/msg/list" && method === "GET") {
        return await handleFetchOfficialAccountMsgList(request, env);
      }

      if (path === "/api/mp/refresh" && method === "POST") {
        return await handleRefreshOfficialAccountEvent(request, env);
      }

      if (path === "/admin/token/add" && method === "POST") {
        return await handleAddToken(request, env);
      }

      if (path === "/admin/token/delete" && method === "POST") {
        return await handleDeleteToken(request, env);
      }

      if (path === "/rss/mp" && method === "GET") {
        return await handleFetchMsgListOfOfficialAccountRSS(request, env);
      }

      if (path === "/mp/proxy" && method === "GET") {
        return await handleOfficialAccountProxy(request, env);
      }

      // 404 for unmatched routes
      return Result.Err(404, "Not Found", 404);
    } catch (error) {
      console.error("Worker error:", error);
      return Result.Err(500, "Internal server error");
    }
  },
};

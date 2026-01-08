// Cloudflare Worker for Official Account API
// Simplified version of the Go implementation

// Response utilities
function apiResponse(code, msg, data, status = 200) {
  return new Response(JSON.stringify({
    code,
    msg,
    data,
  }), {
    status,
    headers: {
      'Content-Type': 'application/json',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
    },
  });
}

function successResponse(data = {}, msg = '') {
  return apiResponse(0, msg, data, 200);
}

function errorResponse(code, message, status = 400) {
  return apiResponse(code, message, {}, status);
}

function htmlResponse(html, status = 200) {
  return new Response(html, {
    status,
    headers: {
      'Content-Type': 'text/html; charset=utf-8',
      'Access-Control-Allow-Origin': '*',
    },
  });
}

// Token validation
function validateToken(token, env) {
  if (!token) return false;
  // Simple token validation - in production, use proper JWT or similar
  return token === env.AUTH_TOKEN || token === env.REFRESH_TOKEN;
}

// Generate RSS feed from messages
function generateRSS(officialAccount, messages) {
  const now = new Date().toUTCString();
  let rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>${officialAccount.nickname || officialAccount.biz}</title>
    <link>https://mp.weixin.qq.com/mp/profile_ext?action=home&amp;__biz=${officialAccount.biz}</link>
    <description>公众号文章 RSS</description>
    <language>zh-CN</language>
    <lastBuildDate>${now}</lastBuildDate>
    <generator>WeChat Official Account RSS</generator>`;

  if (messages && messages.length > 0) {
    messages.forEach(msg => {
      const pubDate = new Date(msg.create_time * 1000).toUTCString();
      rss += `
    <item>
      <title><![CDATA[${msg.title || '无标题'}]]></title>
      <link>${msg.content_url || '#'}</link>
      <description><![CDATA[${msg.digest || ''}]]></description>
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
  const token = url.searchParams.get('token');
  
  if (!validateToken(token, env)) {
    return errorResponse(1001, 'Invalid token');
  }

  try {
    const { results } = await env.DB.prepare(
      "SELECT * FROM accounts"
    ).all();
    
    // Filter effective accounts and format response
    const now = Math.floor(Date.now() / 1000);
    const list = results.map(account => {
      const isEffective = account.is_effective === 1 && (now - account.update_time) < (30 * 60);
      return {
        nickname: account.nickname,
        avatar_url: account.avatar_url,
        biz: account.biz,
        is_effective: isEffective,
        update_time: account.update_time,
      };
    });

    return successResponse({ list });
  } catch (error) {
    console.error('Error fetching account list:', error);
    return errorResponse(500, 'Internal server error');
  }
}

async function handleFetchOfficialAccountMsgList(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get('token');
  const biz = url.searchParams.get('biz');
  const offset = parseInt(url.searchParams.get('offset') || '0');
  
  if (!validateToken(token, env)) {
    return errorResponse(1001, 'Invalid token');
  }

  if (!biz) {
    return errorResponse(1002, 'Missing biz parameter');
  }

  try {
    // Get account info
    const account = await env.DB.prepare(
      "SELECT nickname FROM accounts WHERE biz = ?"
    ).bind(biz).first();
    
    if (!account) {
      return errorResponse(1003, 'Account not found');
    }

    // Get messages count
    const countResult = await env.DB.prepare(
      "SELECT COUNT(*) as total FROM messages WHERE biz = ?"
    ).bind(biz).first();
    const total = countResult ? countResult.total : 0;
    
    // Get messages for this account
    const pageSize = 10;
    const { results } = await env.DB.prepare(
      "SELECT raw_json FROM messages WHERE biz = ? ORDER BY create_time DESC LIMIT ? OFFSET ?"
    ).bind(biz, pageSize, offset).all();

    const paginatedMessages = results.map(row => {
      try {
        return JSON.parse(row.raw_json);
      } catch (e) {
        return {};
      }
    });

    return successResponse({
      msg_count: total,
      title: account.nickname,
      list: paginatedMessages,
    });
  } catch (error) {
    console.error('Error fetching message list:', error);
    return errorResponse(500, 'Internal server error');
  }
}

async function handleRefreshOfficialAccountEvent(request, env) {
  const url = new URL(request.url);
  const token = url.searchParams.get('token');
  
  if (token !== env.REFRESH_TOKEN) {
    return errorResponse(1001, 'Invalid refresh token');
  }

  try {
    const body = await request.json();
    
    if (!body.biz || !body.key) {
      return errorResponse(1002, 'Missing biz or key parameter');
    }

    // Update or create account
    const {
        nickname = '',
        avatar_url = '',
        uin = '',
        key,
        pass_ticket = '',
        appmsg_token = ''
    } = body;

    const updateTime = Math.floor(Date.now() / 1000);

    await env.DB.prepare(
      `INSERT INTO accounts (biz, nickname, avatar_url, uin, key, pass_ticket, appmsg_token, is_effective, update_time, error)
       VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, '')
       ON CONFLICT(biz) DO UPDATE SET
       nickname=excluded.nickname,
       avatar_url=excluded.avatar_url,
       uin=excluded.uin,
       key=excluded.key,
       pass_ticket=excluded.pass_ticket,
       appmsg_token=excluded.appmsg_token,
       is_effective=1,
       update_time=excluded.update_time,
       error=''`
    ).bind(
        body.biz, nickname, avatar_url, uin, key, pass_ticket, appmsg_token, updateTime
    ).run();

    return successResponse();
  } catch (error) {
    console.error('Error refreshing account:', error);
    return errorResponse(500, 'Internal server error');
  }
}

// Fetch messages from WeChat API
async function getMsgList(account, offset = 0) {
  const params = new URLSearchParams({
    action: 'getmsg',
    __biz: account.biz,
    uin: account.uin,
    key: account.key,
    pass_ticket: account.pass_ticket,
    wxtoken: '',
    x5: '0',
    count: '10',
    offset: offset.toString(),
    f: 'json',
  });

  const url = `https://mp.weixin.qq.com/mp/profile_ext?${params.toString()}`;
  
  // Construct Referer
  const refererParams = new URLSearchParams({
    action: 'home',
    __biz: account.biz,
    scene: '124',
    uin: account.uin,
    key: account.key,
    devicetype: 'UnifiedPCWindows',
    version: 'f2541022',
    lang: 'zh_CN',
    a8scene: '1',
    acctmode: '0',
    pass_ticket: account.pass_ticket
  });
  const referer = `https://mp.weixin.qq.com/mp/profile_ext?${refererParams.toString()}`;

  const response = await fetch(url, {
     method: 'GET',
     headers: {
       'content-type': 'application/json',
       'accept-language': 'en-US,en;q=0.9',
       'priority': 'u=1, i',
       'sec-fetch-dest': 'empty',
       'sec-fetch-mode': 'cors',
       'sec-fetch-site': 'same-origin',
       'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue',
       'referer': referer
     },
   });

  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }

  return await response.json();
}

async function handleFetchMsgListOfOfficialAccountRSS(request, env) {
  const url = new URL(request.url);
  const biz = url.searchParams.get('biz');
  
  if (!biz) {
    return new Response('Missing biz parameter', { status: 400 });
  }

  try {
    // Get account info
    const account = await env.DB.prepare(
      "SELECT * FROM accounts WHERE biz = ?"
    ).bind(biz).first();
    
    if (!account) {
      return new Response('Account not found', { status: 404 });
    }

    let messages = [];
    let fetchError = null;

    // Try to fetch fresh messages
    try {
      const data = await getMsgList(account, 0);
      
      if (data.ret === 0) {
        const listData = JSON.parse(data.general_msg_list);
        if (listData.list && listData.list.length > 0) {
           listData.list.forEach(item => {
             const msg = item.app_msg_ext_info;
             const common = item.comm_msg_info;
             
             // Add main message
             messages.push({
               title: msg.title,
               digest: msg.digest,
               content_url: msg.content_url,
               cover: msg.cover,
               create_time: common.datetime,
               id: common.id
             });

             // Add sub-messages (multi-app messages)
             if (msg.is_multi === 1 && msg.multi_app_msg_item_list && msg.multi_app_msg_item_list.length > 0) {
               msg.multi_app_msg_item_list.forEach(art => {
                 messages.push({
                   title: art.title,
                   digest: art.digest,
                   content_url: art.content_url,
                   cover: art.cover,
                   create_time: common.datetime,
                   id: art.fileid || common.id
                 });
               });
             }
           });
         }
      } else if (data.ret === -3) {
        // Session expired, mark as ineffective
        await env.DB.prepare(
          "UPDATE accounts SET is_effective = 0 WHERE biz = ?"
        ).bind(biz).run();
        console.log(`Account ${biz} expired`);
      } else {
        console.error(`WeChat API error: ${data.errmsg} (ret: ${data.ret})`);
      }
    } catch (e) {
      console.error('Error fetching from WeChat:', e);
      fetchError = e;
    }

    // Fallback to DB if no messages fetched or error occurred
    if (messages.length === 0) {
      const { results } = await env.DB.prepare(
        "SELECT raw_json FROM messages WHERE biz = ? ORDER BY create_time DESC LIMIT 20"
      ).bind(biz).all();

      const dbMessages = results.map(row => {
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
        'Content-Type': 'application/rss+xml; charset=utf-8',
        'Cache-Control': 'public, max-age=3600',
      },
    });
  } catch (error) {
    console.error('Error generating RSS:', error);
    return new Response('Internal server error', { status: 500 });
  }
}

async function handleOfficialAccountProxy(request, env) {
  const url = new URL(request.url);
  const targetURL = url.searchParams.get('url');
  const token = url.searchParams.get('token');
  
  if (!validateToken(token, env)) {
    return errorResponse(1001, 'Invalid token');
  }

  if (!targetURL) {
    return errorResponse(1002, 'Missing url parameter');
  }

  try {
    // Decode HTML entities in URL
    const decodedURL = targetURL.replace(/&amp;/g, '&');
    
    // Create proxy request
    const proxyRequest = new Request(decodedURL, {
      method: 'GET',
      headers: {
        'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7',
        'accept-language': 'zh-CN,zh;q=0.9',
        'priority': 'u=0, i',
        'sec-ch-ua': '"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"',
        'sec-ch-ua-mobile': '?0',
        'sec-ch-ua-platform': '"macOS"',
        'sec-fetch-dest': 'document',
        'sec-fetch-mode': 'navigate',
        'sec-fetch-site': 'none',
        'sec-fetch-user': '?1',
        'upgrade-insecure-requests': '1',
        'user-agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36',
      },
    });

    const response = await fetch(proxyRequest);
    
    // Return the response with appropriate headers
    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: {
        'Content-Type': response.headers.get('Content-Type') || 'text/html',
        'Access-Control-Allow-Origin': '*',
      },
    });
  } catch (error) {
    console.error('Proxy error:', error);
    return errorResponse(500, 'Proxy request failed');
  }
}

async function handleOfficialAccountManagerHome(request, env) {
  const html = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>公众号管理</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            margin-bottom: 20px;
        }
        .info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .info-card {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #007bff;
        }
        .info-card h3 {
            margin-top: 0;
            color: #495057;
        }
        .info-card p {
            margin: 5px 0;
            color: #6c757d;
        }
        .status {
            display: inline-block;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: bold;
        }
        .status.active {
            background: #d4edda;
            color: #155724;
        }
        .status.inactive {
            background: #f8d7da;
            color: #721c24;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>公众号管理后台</h1>
        <div class="info-grid">
            <div class="info-card">
                <h3>服务状态</h3>
                <p>运行模式: Cloudflare Worker</p>
                <p>远程服务器: ${env.REMOTE_SERVER || '未配置'}</p>
                <p>认证状态: <span class="status active">已启用</span></p>
            </div>
            <div class="info-card">
                <h3>API 端点</h3>
                <p>📋 公众号列表: <code>GET /api/mp/list</code></p>
                <p>📰 文章列表: <code>GET /api/mp/msg/list</code></p>
                <p>🔄 刷新凭证: <code>POST /api/mp/refresh</code></p>
                <p>📡 RSS 订阅: <code>GET /rss/mp</code></p>
                <p>🌐 代理服务: <code>GET /mp/proxy</code></p>
            </div>
        </div>
        <div class="info-card">
            <h3>使用说明</h3>
            <p>这是一个简化版的公众号管理 API，部署在 Cloudflare Worker 上。</p>
            <p>支持基本的公众号管理和文章获取功能。</p>
        </div>
    </div>
</body>
</html>`;

  return htmlResponse(html);
}

// Main request handler
export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;
    const method = request.method;

    // Handle CORS preflight
    if (method === 'OPTIONS') {
      return new Response(null, {
        status: 200,
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
          'Access-Control-Allow-Headers': 'Content-Type',
        },
      });
    }

    // Route handling
    try {
      if (path === '/api/mp/list' && method === 'GET') {
        return await handleFetchOfficialAccountList(request, env);
      }
      
      if (path === '/api/mp/msg/list' && method === 'GET') {
        return await handleFetchOfficialAccountMsgList(request, env);
      }
      
      if (path === '/api/mp/refresh' && method === 'POST') {
        return await handleRefreshOfficialAccountEvent(request, env);
      }
      
      if (path === '/rss/mp' && method === 'GET') {
        return await handleFetchMsgListOfOfficialAccountRSS(request, env);
      }
      
      if (path === '/mp/proxy' && method === 'GET') {
        return await handleOfficialAccountProxy(request, env);
      }
      
      if (path === '/mp/home' && method === 'GET') {
        return await handleOfficialAccountManagerHome(request, env);
      }

      // 404 for unmatched routes
      return new Response(
        `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>404 Not Found</title><style>body{margin:0;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#0b0c0f;color:#e6e6e6;display:flex;align-items:center;justify-content:center;height:100vh}.box{max-width:560px;padding:24px 28px;border-radius:12px;background:#14171f;box-shadow:0 8px 24px rgba(0,0,0,.3)}h1{margin:0 0 8px;font-size:24px}p{margin:0;color:#b0b0b0}a{color:#8ab4f8;text-decoration:none}a:hover{text-decoration:underline}</style></head><body><div class="box"><h1>404 未找到页面</h1><p>请求的路径不存在。</p></div></body></html>`,
        {
          status: 404,
          headers: { 'Content-Type': 'text/html; charset=utf-8' },
        }
      );
    } catch (error) {
      console.error('Worker error:', error);
      return errorResponse(500, 'Internal server error');
    }
  },
};
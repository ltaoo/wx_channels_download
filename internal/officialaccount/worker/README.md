# Official Account API - Cloudflare Worker

A simplified Node.js version of the Official Account API, designed to run on Cloudflare Workers.

## Features

This implementation provides the following API endpoints from the original Go version:

- `GET /api/mp/list` - Fetch official account list
- `GET /api/mp/msg/list` - Fetch official account message list
- `POST /api/mp/refresh` - Refresh official account credentials
- `GET /rss/mp` - Generate RSS feed for official account messages
- `GET /mp/proxy` - Proxy requests to external URLs
- `GET /mp/home` - Management dashboard

## Deployment

### Prerequisites

1. Install [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/install-and-update/):
```bash
npm install -g wrangler
```

2. Authenticate with Cloudflare:
```bash
wrangler login
```

### Setup

1. Create KV namespaces in Cloudflare:
```bash
wrangler kv:namespace create "OFFICIAL_ACCOUNTS"
wrangler kv:namespace create "OFFICIAL_MESSAGES"
```

2. Update `wrangler.toml` with your KV namespace IDs

3. Set environment variables:
```bash
wrangler secret put AUTH_TOKEN
wrangler secret put REFRESH_TOKEN
```

### Deploy

```bash
npm run deploy
```

### Development

```bash
npm run dev
```

## API Usage

### Authentication

Most endpoints require a token parameter. Set `AUTH_TOKEN` and `REFRESH_TOKEN` in your environment variables.

### Endpoints

#### Get Account List
```
GET /api/mp/list?token=YOUR_TOKEN
```

#### Get Message List
```
GET /api/mp/msg/list?token=YOUR_TOKEN&biz=ACCOUNT_BIZ&offset=0
```

#### Refresh Account Credentials
```
POST /api/mp/refresh?token=REFRESH_TOKEN
Content-Type: application/json

{
  "biz": "account_biz",
  "key": "account_key",
  "nickname": "Account Name",
  "avatar_url": "https://example.com/avatar.jpg",
  "uin": "uin_value",
  "pass_ticket": "pass_ticket_value",
  "appmsg_token": "appmsg_token_value"
}
```

#### Get RSS Feed
```
GET /rss/mp?biz=ACCOUNT_BIZ
```

#### Proxy Request
```
GET /mp/proxy?token=YOUR_TOKEN&url=TARGET_URL
```

#### Management Dashboard
```
GET /mp/home
```

## Data Storage

The worker uses Cloudflare KV for data persistence:

- `OFFICIAL_ACCOUNTS` namespace stores account information
- `OFFICIAL_MESSAGES` namespace stores message data

## Error Codes

- `1001` - Invalid token
- `1002` - Missing required parameters
- `1003` - Account not found
- `500` - Internal server error

## Security Notes

- Always use HTTPS in production
- Keep your tokens secure
- Consider implementing rate limiting for production use
- Use proper JWT tokens instead of simple string tokens for better security

## Limitations

This is a simplified version compared to the original Go implementation:
- No WebSocket support
- No real-time features
- Simplified authentication
- Basic error handling
- Limited message storage (KV has size limits)

For full functionality, consider using the original Go implementation with proper infrastructure.
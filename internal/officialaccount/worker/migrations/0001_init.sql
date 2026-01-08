-- Migration number: 0001 	 2024-01-08T00:00:00.000Z
-- Init schema

CREATE TABLE IF NOT EXISTS accounts (
  biz TEXT PRIMARY KEY,
  nickname TEXT,
  avatar_url TEXT,
  uin TEXT,
  key TEXT,
  pass_ticket TEXT,
  appmsg_token TEXT,
  is_effective INTEGER DEFAULT 1, -- 0 or 1 for boolean
  update_time INTEGER,
  error TEXT
);

CREATE TABLE IF NOT EXISTS messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  biz TEXT NOT NULL,
  msg_id INTEGER, -- The 'id' field from the message object
  title TEXT,
  digest TEXT,
  content_url TEXT,
  create_time INTEGER,
  raw_json TEXT -- Store the full JSON object for flexibility
);


-- Reuse content + content_video (movie subtype), content_series,
-- content_episode, content_relation and the existing resource/asset catalog.
INSERT OR IGNORE INTO platform (id, code, name, homepage, entry_url) VALUES
 ('youku', 'youku', '优酷', 'https://www.youku.com', 'https://www.youku.com'),
 ('tencent', 'tencent', '腾讯视频', 'https://v.qq.com', 'https://v.qq.com'),
 ('mgtv', 'mgtv', '芒果TV', 'https://www.mgtv.com', 'https://www.mgtv.com'),
 ('hongguo', 'hongguo', '红果短剧', 'https://hongguoduanju.com', 'https://hongguoduanju.com');

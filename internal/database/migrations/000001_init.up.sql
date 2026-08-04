-- 平台表 (Platform)
CREATE TABLE IF NOT EXISTS `platform` (
  `id` TEXT PRIMARY KEY,
  `code` TEXT NOT NULL UNIQUE, -- 平台唯一标识, 如 'wx_channels', 'youtube'
  `name` TEXT NOT NULL, -- 平台名称
  `homepage` TEXT, -- 官网地址
  `logo_url` TEXT, -- 平台logo链接
  `entry_url` TEXT, -- 入口网站/登录页
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 默认平台数据
INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'wx_channels',
  'wx_channels',
  '微信视频号',
  'https://channels.weixin.qq.com',
  '/assets/platform/wx_channels/logo.png',
  'https://channels.weixin.qq.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'tiktok',
  'tiktok',
  'TikTok',
  'https://www.tiktok.com',
  '/assets/platform/tiktok/logo.png',
  'https://www.tiktok.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'douyin',
  'douyin',
  '抖音',
  'https://www.douyin.com',
  '/assets/platform/douyin/logo.png',
  'https://www.douyin.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'bilibili',
  'bilibili',
  'Bilibili',
  'https://www.bilibili.com',
  '/assets/platform/bilibili/logo.png',
  'https://www.bilibili.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'x',
  'x',
  'X',
  'https://x.com',
  '/assets/platform/x/logo.png',
  'https://x.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'youtube',
  'youtube',
  'YouTube',
  'https://www.youtube.com',
  '/assets/platform/youtube/logo.png',
  'https://www.youtube.com'
);

INSERT OR IGNORE INTO `platform` (
  `id`, `code`, `name`, `homepage`, `logo_url`, `entry_url`
) VALUES (
  'zhihu',
  'zhihu',
  '知乎',
  'https://www.zhihu.com',
  '/assets/platform/zhihu/logo.png',
  'https://www.zhihu.com'
);

CREATE TABLE IF NOT EXISTS `auth_credential` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `platform_id` TEXT NOT NULL,
  `name` TEXT NOT NULL,
  `kind` TEXT NOT NULL,
  `secret` TEXT,
  `payload` TEXT,
  `expires_at` INTEGER,
  `status` INTEGER NOT NULL DEFAULT 1,
  `is_default` INTEGER NOT NULL DEFAULT 0,
  `last_used_at` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_auth_credential_platform_status_default
ON `auth_credential` (`platform_id`, `status`, `is_default`);

-- 网红/博主表 (Influencer) - 归纳的自然人或IP实体
CREATE TABLE IF NOT EXISTS `influencer` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `name` TEXT NOT NULL, -- 网红名称
  `avatar_url` TEXT, -- 头像链接
  `sex` INTEGER DEFAULT 0, -- 性别 (0:未知, 1:男, 2:女)
  `description` TEXT, -- 备注/描述
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 平台帐号表 (Account) - 具体在某个平台上的注册帐号
CREATE TABLE IF NOT EXISTS `account` (
  `id` TEXT PRIMARY KEY,
  `platform_id` TEXT NOT NULL, -- 所属平台
  `influencer_id` INTEGER, -- 归属的网红 (可为空，表示尚未归纳或独立帐号)
  `external_id` TEXT NOT NULL, -- 平台侧的用户ID (如 openid, uid)
  `alias` TEXT, -- 别名 (如 张三)
  `nickname` TEXT, -- 昵称
  `signature` TEXT, -- 签名
  `avatar_url` TEXT, -- 头像
  `profile_url` TEXT, -- 主页链接
  `is_listen` INTEGER DEFAULT 0, -- 是否监控帐号内容
  `follower_count` INTEGER, -- 粉丝数
  `past_names` TEXT, -- 曾用名 (JSON array)
  `past_avatars` TEXT, -- 使用过的头像 (JSON array)
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_account_platform_external ON `account` (`platform_id`, `external_id`);

-- 分片表：按固定 piece 大小切分，记录范围与状态
CREATE TABLE IF NOT EXISTS `download_task_piece` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `task_id` INTEGER NOT NULL,
  `piece_index` INTEGER NOT NULL,
  `start_offset` INTEGER NOT NULL,
  `end_offset` INTEGER NOT NULL,
  `size` INTEGER NOT NULL,
  `status` INTEGER DEFAULT 0,
  `retry_count` INTEGER DEFAULT 0,
  `checksum` TEXT,
  `temp_path` TEXT,
  `locked_by` TEXT,
  `lease_expires_at` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_download_task_piece_unique
ON `download_task_piece` (`task_id`, `piece_index`);

CREATE INDEX IF NOT EXISTS idx_download_task_piece_status ON `download_task_piece` (`task_id`, `status`);

-- 事件表：记录暂停/恢复/失败/重试等事件
CREATE TABLE IF NOT EXISTS `download_task_event` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `task_id` INTEGER NOT NULL,
  `type` TEXT NOT NULL,
  `message` TEXT,
  `data` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_download_task_event_task ON `download_task_event` (`task_id`, `created_at`);

CREATE TABLE IF NOT EXISTS `chat_session` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `name` TEXT NOT NULL,
  `platform` TEXT NOT NULL,
  `type` TEXT NOT NULL,
  `group_id` TEXT,
  `group_avatar` TEXT,
  `format_version` TEXT,
  `exported_at` INTEGER,
  `generator` TEXT,
  `description` TEXT,
  `extra_data` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_chat_session_platform_type
ON `chat_session` (`platform`, `type`);

CREATE INDEX IF NOT EXISTS idx_chat_session_platform_name
ON `chat_session` (`platform`, `name`);

CREATE TABLE IF NOT EXISTS `chat_member` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `session_id` INTEGER NOT NULL,
  `platform_id` TEXT NOT NULL,
  `account_name` TEXT NOT NULL,
  `group_nickname` TEXT,
  `aliases` TEXT,
  `avatar` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_member_session_platform_id
ON `chat_member` (`session_id`, `platform_id`);

CREATE INDEX IF NOT EXISTS idx_chat_member_session
ON `chat_member` (`session_id`);

CREATE TABLE IF NOT EXISTS `chat_message` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `session_id` INTEGER NOT NULL,
  `source_message_id` TEXT,
  `sender` TEXT NOT NULL,
  `account_name` TEXT NOT NULL,
  `group_nickname` TEXT,
  `timestamp` INTEGER NOT NULL,
  `type` INTEGER NOT NULL,
  `content` TEXT,
  `payload` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_message_session_source_message_id
ON `chat_message` (`session_id`, `source_message_id`);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_time
ON `chat_message` (`session_id`, `timestamp`);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_sender_time
ON `chat_message` (`session_id`, `sender`, `timestamp`);

CREATE TABLE IF NOT EXISTS `wx_video_access` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `account_id` TEXT NOT NULL,
  `url` TEXT NOT NULL,
  `description` TEXT,
  `cover_url` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wx_video_access_account_url ON `wx_video_access` (`account_id`, `url`);

-- 直播下载任务表 (LiveDownloadTask)
CREATE TABLE IF NOT EXISTS `live_download_task` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `task_id` TEXT NOT NULL UNIQUE, -- 任务唯一标识
  `platform_id` TEXT NOT NULL, -- 平台ID (可选)
  `account_id` TEXT, -- 帐号ID (可选)
  
  -- 直播信息
  `live_url` TEXT NOT NULL, -- 直播流地址
  `title` TEXT, -- 直播标题
  `streamer_name` TEXT, -- 主播名称
  `cover_url` TEXT, -- 封面图
  
  -- 下载配置
  `save_path` TEXT NOT NULL, -- 保存路径
  `filename` TEXT NOT NULL, -- 文件名
  `quality` TEXT, -- 质量选项
  
  -- 下载状态
  `status` INTEGER DEFAULT 0, -- 0:Pending, 1:Downloading, 2:Paused, 3:Completed, 4:Failed, 5:Cancelled
  `progress` REAL DEFAULT 0, -- 下载进度 (0-100)
  `downloaded_size` INTEGER DEFAULT 0, -- 已下载大小 (bytes)
  `download_speed` REAL DEFAULT 0, -- 下载速度 (bytes/s)
  `estimated_time` INTEGER DEFAULT 0, -- 预计剩余时间 (秒)
  
  -- 时间信息
  `start_time` INTEGER, -- 开始时间
  `end_time` INTEGER, -- 结束时间
  `pause_time` INTEGER, -- 暂停时间
  
  -- 错误信息
  `error_msg` TEXT, -- 错误信息
  `retry_count` INTEGER DEFAULT 0, -- 重试次数
  
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_live_download_task_status ON `live_download_task` (`status`);
CREATE INDEX IF NOT EXISTS idx_live_download_task_created ON `live_download_task` (`created_at`);

-- ========================================
-- 通用内容管理系统 (Content Management)
-- ========================================

-- 内容主表 (Content) - 存储所有类型内容的通用字段
CREATE TABLE IF NOT EXISTS `content` (
  `id` TEXT PRIMARY KEY,
  `platform_id` TEXT NOT NULL, -- 发布平台
  `type` TEXT NOT NULL, -- 内容类型: 'video', 'short_video', 'image', 'image_set', 'novel', 'blog', 'podcast', 'music', 'article'
  
  -- 基础信息
  `external_id` TEXT NOT NULL, -- 平台侧内容ID
  `external_id2` TEXT,
  `external_id3` TEXT,
  `title` TEXT, -- 标题
  `description` TEXT, -- 描述/简介
  `content_url` TEXT, -- 内容原始链接
  `url` TEXT,
  `source_url` TEXT,
  `cover_url` TEXT, -- 封面图/缩略图
  `cover_width` TEXT,
  `cover_height` TEXT,
  `metadata` TEXT,
  
  -- 发布信息
  `publish_time` INTEGER, -- 发布时间
  `update_time` INTEGER, -- 更新时间 (针对可编辑内容)
  `is_original` INTEGER DEFAULT 1, -- 是否原创 (0:否, 1:是)
  `is_private` INTEGER DEFAULT 0, -- 是否私密 (0:公开, 1:私密)
  
  -- 互动数据
  `view_count` INTEGER DEFAULT 0, -- 浏览量
  `play_times` INTEGER DEFAULT 0,
  `like_count` INTEGER DEFAULT 0, -- 点赞数
  `comment_count` INTEGER DEFAULT 0, -- 评论数
  `share_count` INTEGER DEFAULT 0, -- 分享数
  `collect_count` INTEGER DEFAULT 0, -- 收藏数
  
  -- 下载状态
  `download_status` INTEGER DEFAULT 0, -- 0:未下载, 1:下载中, 2:已完成, 3:失败
  `download_path` TEXT, -- 本地存储路径
  `file_size` INTEGER, -- 文件大小 (bytes)
  `size` INTEGER,
  `duration` INTEGER,
  `download_time` INTEGER, -- 下载完成时间
  `error_msg` TEXT, -- 错误信息
  `unread` INTEGER DEFAULT 0,
  `source_deleted` INTEGER DEFAULT 0,
  `validated` INTEGER DEFAULT 0,
  
  -- 标签与分类
  `tags` TEXT, -- 标签 (JSON array)
  `category` TEXT, -- 分类
  
  -- 扩展字段
  `extra_data` TEXT, -- 扩展数据 (JSON, 存储平台特定字段)
  `html` TEXT, -- 文章 HTML 正文

  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 视频内容扩展表 (ContentVideo) - 视频/短视频特定字段
CREATE TABLE IF NOT EXISTS `content_video` (
  `id` TEXT PRIMARY KEY,
  `duration` INTEGER, -- 时长 (秒)
  `width` INTEGER, -- 视频宽度
  `height` INTEGER, -- 视频高度
  `cover_width` TEXT,
  `cover_height` TEXT,
  `fps` INTEGER, -- 帧率
  `bitrate` INTEGER, -- 码率 (kbps)
  `size` INTEGER,
  `codec` TEXT, -- 编码格式 (h264, h265, vp9, etc.)
  `format` TEXT, -- 文件格式 (mp4, mov, flv, etc.)
  `has_subtitle` INTEGER DEFAULT 0, -- 是否有字幕
  `subtitle_url` TEXT, -- 字幕文件链接
  `audio_track_count` INTEGER DEFAULT 1, -- 音轨数量
  `url` TEXT,
  `metadata` TEXT,
  `play_times` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

-- 图片内容扩展表 (ContentImage) - 统一图片存储
CREATE TABLE IF NOT EXISTS `content_image` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `album_id` TEXT NOT NULL,
  `sort_order` INTEGER DEFAULT 0,
  `url` TEXT,
  `width` INTEGER,
  `height` INTEGER,
  `size` INTEGER DEFAULT 0,
  `ext` TEXT,
  `image_type` TEXT DEFAULT 'still',
  `deleted_at` INTEGER
);
CREATE INDEX IF NOT EXISTS idx_content_image_album ON `content_image` (`album_id`);

-- 音频内容扩展表 (ContentAudio) - 播客/音乐特定字段
CREATE TABLE IF NOT EXISTS `content_audio` (
  `id` TEXT PRIMARY KEY,
  `duration` INTEGER, -- 时长 (秒)
  `bitrate` INTEGER, -- 码率 (kbps)
  `format` TEXT, -- 音频格式 (mp3, aac, flac, etc.)
  `sample_rate` INTEGER, -- 采样率 (Hz)
  
  -- 音乐特定字段
  `artist` TEXT, -- 艺术家/歌手
  `album` TEXT, -- 专辑
  `genre` TEXT, -- 流派
  `lyrics_url` TEXT, -- 歌词链接
  
  -- 播客特定字段
  `episode_number` INTEGER, -- 集数
  `season_number` INTEGER, -- 季数
  `series_name` TEXT -- 系列名称
);

-- 文章内容扩展表 (ContentArticle) - 文章/小说特定字段
CREATE TABLE IF NOT EXISTS `content_article` (
  `id` TEXT PRIMARY KEY,
  `type` TEXT NOT NULL DEFAULT '',
  `word_count` INTEGER NOT NULL DEFAULT 0,
  `reading_time` INTEGER NOT NULL DEFAULT 0,
  `text` TEXT,
  `html` TEXT,
  `markdown` TEXT,
  `chapter_number` INTEGER NOT NULL DEFAULT 0,
  `volume_number` INTEGER NOT NULL DEFAULT 0,
  `series_name` TEXT,
  `is_finished` INTEGER NOT NULL DEFAULT 0,
  `publish_platform` TEXT,
  `is_original` INTEGER NOT NULL DEFAULT 0
);

-- 直播内容扩展表 (ContentLive)
CREATE TABLE IF NOT EXISTS `content_live` (
  `id` TEXT PRIMARY KEY,
  `duration` INTEGER,
  `stream_url` TEXT,
  `cover_width` INTEGER,
  `cover_height` INTEGER,
  `viewer_count` INTEGER,
  `format` TEXT,
  `bitrate` INTEGER,
  `size` INTEGER,
  `metadata` TEXT
);

-- 小说内容扩展表 (ContentNovel)
CREATE TABLE IF NOT EXISTS `content_novel` (
  `id` TEXT PRIMARY KEY,
  `author_name` TEXT,
  `word_count` INTEGER,
  `chapter_count` INTEGER,
  `volume_count` INTEGER,
  `series_name` TEXT,
  `is_finished` INTEGER,
  `text` TEXT,
  `html` TEXT
);

-- 小说卷表 (ContentNovelVolume)
CREATE TABLE IF NOT EXISTS `content_novel_volume` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `novel_id` TEXT NOT NULL,
  `idx` INTEGER NOT NULL DEFAULT 0,
  `title` TEXT
);
CREATE INDEX IF NOT EXISTS idx_novel_volume_novel ON `content_novel_volume` (`novel_id`);

-- 小说章节表 (ContentNovelChapter)
CREATE TABLE IF NOT EXISTS `content_novel_chapter` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `novel_id` TEXT NOT NULL,
  `volume_id` INTEGER,
  `idx` INTEGER NOT NULL DEFAULT 0,
  `title` TEXT,
  `url` TEXT,
  `locked` INTEGER DEFAULT 0,
  `word_count` INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_novel_chapter_novel ON `content_novel_chapter` (`novel_id`);

-- 图集/相册内容扩展表 (ContentAlbum)
CREATE TABLE IF NOT EXISTS `content_album` (
  `id` TEXT PRIMARY KEY,
  `image_count` INTEGER,
  `cover_width` INTEGER,
  `cover_height` INTEGER,
  `format` TEXT,
  `description` TEXT
);

-- 播客内容扩展表 (ContentPodcast)
CREATE TABLE IF NOT EXISTS `content_podcast` (
  `id` TEXT PRIMARY KEY,
  `duration` INTEGER,
  `bitrate` INTEGER,
  `format` TEXT,
  `sample_rate` INTEGER,
  `artist` TEXT,
  `series_name` TEXT,
  `episode_number` INTEGER,
  `season_number` INTEGER,
  `size` INTEGER
);

-- 文档内容扩展表 (ContentDocument)
CREATE TABLE IF NOT EXISTS `content_document` (
  `id` TEXT PRIMARY KEY,
  `page_count` INTEGER,
  `file_format` TEXT,
  `file_size` INTEGER,
  `author_name` TEXT,
  `word_count` INTEGER,
  `isbn` TEXT,
  `publisher` TEXT,
  `language` TEXT
);

-- 课程内容扩展表 (ContentCourse)
CREATE TABLE IF NOT EXISTS `content_course` (
  `id` TEXT PRIMARY KEY,
  `duration` INTEGER,
  `instructor` TEXT,
  `series_name` TEXT,
  `episode_number` INTEGER,
  `total_episodes` INTEGER,
  `platform_name` TEXT,
  `chapter_count` INTEGER
);

-- 漫画内容扩展表 (ContentComic)
CREATE TABLE IF NOT EXISTS `content_comic` (
  `id` TEXT PRIMARY KEY,
  `chapter_number` INTEGER,
  `volume_number` INTEGER,
  `series_name` TEXT,
  `page_count` INTEGER,
  `author_name` TEXT,
  `artist_name` TEXT,
  `format` TEXT,
  `is_finished` INTEGER,
  `reading_direction` TEXT
);

-- 帖子/动态内容扩展表 (ContentPost)
CREATE TABLE IF NOT EXISTS `content_post` (
  `id` TEXT PRIMARY KEY,
  `text` TEXT,
  `reply_count` INTEGER,
  `repost_count` INTEGER,
  `quote_count` INTEGER,
  `language` TEXT,
  `mentions_json` TEXT,
  `hashtags_json` TEXT
);

-- 内容与帐号关联表 (多对多) - 记录内容归属的帐号
CREATE TABLE IF NOT EXISTS `content_account` (
  `content_id` TEXT NOT NULL,
  `account_id` TEXT NOT NULL,
  `role` TEXT DEFAULT 'author', -- 角色: 'author'(作者), 'co_author'(联合作者), 'featured'(出镜), 'mentioned'(提及)
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`content_id`, `account_id`)
);

-- 内容与网红关联表 (多对多) - 记录内容关联的网红
CREATE TABLE IF NOT EXISTS `content_influencer` (
  `content_id` TEXT NOT NULL,
  `influencer_id` INTEGER NOT NULL,
  `role` TEXT DEFAULT 'creator', -- 角色: 'creator'(创作者), 'featured'(出镜), 'mentioned'(提及)
  `created_at` INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (`content_id`, `influencer_id`)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_content_platform_type ON `content` (`platform_id`, `type`);
CREATE INDEX IF NOT EXISTS idx_content_external_id ON `content` (`platform_id`, `external_id`);
CREATE INDEX IF NOT EXISTS idx_content_publish_time ON `content` (`publish_time` DESC);
CREATE INDEX IF NOT EXISTS idx_content_download_status ON `content` (`download_status`);
CREATE INDEX IF NOT EXISTS idx_content_type ON `content` (`type`);
CREATE INDEX IF NOT EXISTS idx_content_account_account ON `content_account` (`account_id`);
CREATE INDEX IF NOT EXISTS idx_content_influencer_influencer ON `content_influencer` (`influencer_id`);

CREATE TABLE IF NOT EXISTS `browse_history` (
  `id` TEXT PRIMARY KEY,
  `platform_id` TEXT NOT NULL,
  `visited_times` INTEGER NOT NULL DEFAULT 1, --访问次数
  `account_id` TEXT,
  `influencer_id` INTEGER,
  `account_external_id` TEXT,
  `account_nickname` TEXT,
  `account_avatar_url` TEXT,
  `type` TEXT,
  `external_id` TEXT,
  `title` TEXT,
  `url` TEXT,
  `source_url` TEXT,
  `cover_url` TEXT,
  `cover_width` TEXT,
  `cover_height` TEXT,
  `publish_time` INTEGER,
  `extra_data` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_browse_history_time ON `browse_history` (`visited_times`);
CREATE INDEX IF NOT EXISTS idx_browse_history_external
ON `browse_history` (`platform_id`, `external_id`);
CREATE INDEX IF NOT EXISTS idx_browse_history_platform_account_updated
ON `browse_history` (`platform_id`, `account_id`, `updated_at`);
CREATE INDEX IF NOT EXISTS idx_browse_history_platform_influencer_updated
ON `browse_history` (`platform_id`, `influencer_id`, `updated_at`);

CREATE TABLE IF NOT EXISTS `platform_workflow_run` (
  `id` TEXT PRIMARY KEY,
  `url` TEXT NOT NULL,
  `platform` TEXT,
  `status` TEXT,
  `current_node` TEXT,
  `task_id` TEXT,
  `download_task_id` INTEGER DEFAULT 0,
  `extra` TEXT,
  `output` TEXT,
  `selection` TEXT,
  `nodes` TEXT,
  `error` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_platform_workflow_run_status
ON `platform_workflow_run` (`status`);

CREATE INDEX IF NOT EXISTS idx_platform_workflow_run_platform
ON `platform_workflow_run` (`platform`);

CREATE INDEX IF NOT EXISTS idx_platform_workflow_run_task_id
ON `platform_workflow_run` (`task_id`);

CREATE INDEX IF NOT EXISTS idx_platform_workflow_run_download_task_id
ON `platform_workflow_run` (`download_task_id`);

-- ========================================
-- 多协议下载器表
-- ========================================

CREATE TABLE IF NOT EXISTS `download_task` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `content_id` TEXT,
  `parent_task_id` INTEGER,
  `root_task_id` INTEGER NOT NULL DEFAULT 0,
  `relation_type` TEXT NOT NULL DEFAULT '',
  `name` TEXT NOT NULL,
  `platform_id` TEXT NOT NULL DEFAULT '',
  `unique_id` TEXT,
  `status` INTEGER NOT NULL DEFAULT 0,
  `source_url` TEXT,
  `cover_url` TEXT,
  `cover_width` TEXT,
  `cover_height` TEXT,
  `config_json` TEXT,
  `metadata_json` TEXT,
  `error_message` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_download_task_status ON `download_task` (`status`);
CREATE INDEX IF NOT EXISTS idx_task_unique_id ON `download_task` (`unique_id`);
CREATE INDEX IF NOT EXISTS idx_download_task_content ON `download_task` (`content_id`);
CREATE INDEX IF NOT EXISTS idx_download_task_parent ON `download_task` (`parent_task_id`);
CREATE INDEX IF NOT EXISTS idx_download_task_root ON `download_task` (`root_task_id`);

CREATE TABLE IF NOT EXISTS `download_resource` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `task_id` INTEGER NOT NULL,
  `content_id` TEXT,
  `name` TEXT,
  `kind` TEXT NOT NULL DEFAULT 'file',
  `unique_id` TEXT,
  `type` TEXT NOT NULL DEFAULT 'file',
  `size` INTEGER,
  `status` INTEGER DEFAULT 0,
  `downloaded` INTEGER DEFAULT 0,
  `speed` INTEGER DEFAULT 0,
  `merge_order` INTEGER DEFAULT 0,
  `extra` TEXT,
  `stream_url` TEXT,
  `record_start` INTEGER,
  `record_end` INTEGER,
  `duration` INTEGER DEFAULT 0,
  `rotate_minutes` INTEGER DEFAULT 0,
  `rotate_size` INTEGER DEFAULT 0,
  `start_time` INTEGER,
  `finish_time` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_resource_task ON `download_resource` (`task_id`);
CREATE INDEX IF NOT EXISTS idx_resource_unique_id ON `download_resource` (`unique_id`);

CREATE TABLE IF NOT EXISTS `download_endpoint` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `resource_id` INTEGER NOT NULL,
  `protocol` TEXT NOT NULL,
  `url` TEXT NOT NULL,
  `priority` INTEGER DEFAULT 0,
  `enabled` INTEGER DEFAULT 1,
  `headers` TEXT,
  `cookies` TEXT,
  `status` INTEGER DEFAULT 0,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_endpoint_resource ON `download_endpoint` (`resource_id`);

CREATE TABLE IF NOT EXISTS `download_segment` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `resource_id` INTEGER NOT NULL,
  `index` INTEGER NOT NULL,
  `url` TEXT,
  `offset_start` INTEGER,
  `offset_end` INTEGER,
  `size` INTEGER,
  `downloaded` INTEGER DEFAULT 0,
  `status` INTEGER DEFAULT 0,
  `retry` INTEGER DEFAULT 0,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_segment_resource ON `download_segment` (`resource_id`);

CREATE TABLE IF NOT EXISTS `download_connection` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `endpoint_id` INTEGER NOT NULL,
  `worker_id` TEXT,
  `host` TEXT,
  `ip` TEXT,
  `speed` INTEGER DEFAULT 0,
  `bytes` INTEGER DEFAULT 0,
  `status` INTEGER DEFAULT 0,
  `last_active` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS idx_conn_endpoint ON `download_connection` (`endpoint_id`);

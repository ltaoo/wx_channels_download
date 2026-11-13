-- Reusable scheduled workflow execution: persisted crontab rules and the run
-- history produced by them.
CREATE TABLE IF NOT EXISTS `flow_schedule` (
  `id` TEXT PRIMARY KEY,
  `name` TEXT NOT NULL,
  `description` TEXT,
  `cron_expr` TEXT NOT NULL,
  `flow_id` TEXT NOT NULL,
  `instance_id` TEXT NOT NULL REFERENCES user_flow_instance(id),
  `initial_data` TEXT,
  `enabled` INTEGER NOT NULL DEFAULT 1,
  `timeout_sec` INTEGER NOT NULL DEFAULT 3600,
  `next_run_at` INTEGER,
  `last_run_id` TEXT,
  `last_run_status` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS `idx_flow_schedule_flow_id`
ON `flow_schedule` (`flow_id`);
CREATE INDEX IF NOT EXISTS `idx_flow_schedule_next_run_at`
ON `flow_schedule` (`next_run_at`);
CREATE INDEX IF NOT EXISTS `idx_flow_schedule_deleted_at`
ON `flow_schedule` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `flow_run_record` (
  `id` TEXT PRIMARY KEY,
  `schedule_id` TEXT,
  `flow_id` TEXT NOT NULL,
  `instance_id` TEXT NOT NULL REFERENCES user_flow_instance(id),
  `trigger_type` TEXT,
  `trigger_key` TEXT,
  `status` TEXT NOT NULL,
  `current_node` TEXT,
  `node_attempts` TEXT,
  `node_outputs` TEXT,
  `node_states` TEXT NOT NULL DEFAULT '{}',
  `error` TEXT,
  `started_at` INTEGER,
  `completed_at` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER,
  `definition` TEXT,
  `initial_data` TEXT,
  `checkpoint` TEXT,
  `timeout_sec` INTEGER NOT NULL DEFAULT 3600,
  `deadline_at` INTEGER NOT NULL DEFAULT 0,
  `retry_of` TEXT
);

CREATE INDEX IF NOT EXISTS `idx_flow_run_record_schedule_id`
ON `flow_run_record` (`schedule_id`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_flow_id`
ON `flow_run_record` (`flow_id`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_status`
ON `flow_run_record` (`status`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_deleted_at`
ON `flow_run_record` (`deleted_at`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_deadline_at`
ON `flow_run_record` (`deadline_at`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_retry_of`
ON `flow_run_record` (`retry_of`);

-- 标签系统：扁平标签 + 内容/账号多对多关联
CREATE TABLE IF NOT EXISTS `tag` (
    `id`         INTEGER PRIMARY KEY AUTOINCREMENT,
    `name`       TEXT NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    `updated_at` INTEGER NOT NULL DEFAULT 0,
    `deleted_at` INTEGER,
    UNIQUE(`name`)
);
CREATE INDEX IF NOT EXISTS `idx_tag_deleted_at` ON `tag` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `content_tag` (
    `content_id` TEXT    NOT NULL,
    `tag_id`     INTEGER NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`content_id`, `tag_id`)
);
CREATE INDEX IF NOT EXISTS `idx_content_tag_tag_id` ON `content_tag` (`tag_id`);

CREATE TABLE IF NOT EXISTS `account_tag` (
    `account_id` TEXT    NOT NULL,
    `tag_id`     INTEGER NOT NULL,
    `created_at` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`account_id`, `tag_id`)
);
CREATE INDEX IF NOT EXISTS `idx_account_tag_tag_id` ON `account_tag` (`tag_id`);

-- 迁移已有 JSON tags 数据
INSERT OR IGNORE INTO `tag` (`name`, `created_at`, `updated_at`)
SELECT DISTINCT j.value, strftime('%s','now')*1000, strftime('%s','now')*1000
FROM `content`, json_each(content.tags) AS j
WHERE content.tags IS NOT NULL AND content.tags != '' AND content.tags != '[]'
    AND json_valid(content.tags) = 1;

INSERT OR IGNORE INTO `content_tag` (`content_id`, `tag_id`, `created_at`)
SELECT content.id, tag.id, strftime('%s','now')*1000
FROM `content`, json_each(content.tags) AS j
JOIN `tag` ON tag.name = j.value
WHERE content.tags IS NOT NULL AND content.tags != '' AND content.tags != '[]'
    AND json_valid(content.tags) = 1;

-- A flow can exist without any instances. Only instances are executable.
CREATE TABLE `user_flow` (
 `id` TEXT PRIMARY KEY, `name` TEXT NOT NULL, `description` TEXT,
 `definition` TEXT NOT NULL, `default_node_configs` TEXT NOT NULL DEFAULT '{}',
 `default_triggers` TEXT NOT NULL DEFAULT '[]',
 `created_at` INTEGER NOT NULL DEFAULT 0, `updated_at` INTEGER NOT NULL DEFAULT 0, `deleted_at` INTEGER
);
CREATE INDEX `idx_user_flow_deleted_at` ON `user_flow` (`deleted_at`);
CREATE TABLE `user_flow_instance` (
 `id` TEXT PRIMARY KEY, `flow_id` TEXT NOT NULL REFERENCES user_flow(id),
 `name` TEXT NOT NULL, `description` TEXT,
 `node_configs` TEXT NOT NULL DEFAULT '{}',
 `inherit_triggers` BOOLEAN NOT NULL DEFAULT 0,
 `trigger_overrides` TEXT NOT NULL DEFAULT '{}',
 `trigger_type` TEXT NOT NULL DEFAULT 'Manual', `event_key` TEXT,
 `created_at` INTEGER NOT NULL DEFAULT 0, `updated_at` INTEGER NOT NULL DEFAULT 0, `deleted_at` INTEGER
);
CREATE INDEX `idx_user_flow_instance_flow_id` ON `user_flow_instance` (`flow_id`, `deleted_at`);
CREATE INDEX `idx_flow_schedule_instance_id` ON `flow_schedule` (`instance_id`, `deleted_at`);
CREATE INDEX `idx_flow_run_record_instance_history` ON `flow_run_record` (`instance_id`, `deleted_at`, `created_at` DESC);
CREATE INDEX `idx_flow_run_record_instance_active` ON `flow_run_record` (`instance_id`, `deleted_at`, `status`);
CREATE TABLE `flow_run_node_log` (
 `id` INTEGER PRIMARY KEY AUTOINCREMENT,
 `run_id` TEXT NOT NULL REFERENCES flow_run_record(id),
 `flow_id` TEXT NOT NULL, `node_id` TEXT NOT NULL, `entry` TEXT NOT NULL
);
CREATE INDEX `idx_flow_run_node_log_node` ON `flow_run_node_log` (`run_id`, `flow_id`, `node_id`, `id`);

-- Reuse content + content_video (movie subtype), content_series,
-- content_episode, content_relation and the existing resource/asset catalog.
INSERT OR IGNORE INTO platform (id, code, name, homepage, entry_url) VALUES
 ('youku', 'youku', '优酷', 'https://www.youku.com', 'https://www.youku.com'),
 ('tencent', 'tencent', '腾讯视频', 'https://v.qq.com', 'https://v.qq.com'),
 ('mgtv', 'mgtv', '芒果TV', 'https://www.mgtv.com', 'https://www.mgtv.com'),
 ('hongguo', 'hongguo', '红果短剧', 'https://hongguoduanju.com', 'https://hongguoduanju.com');

INSERT OR IGNORE INTO platform (id, code, name, homepage, entry_url)
VALUES ('iqiyi', 'iqiyi', '爱奇艺', 'https://www.iqiyi.com', 'https://www.iqiyi.com');

-- Composite index for expiring waiting automation runs.
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_status_deleted_deadline`
ON `flow_run_record` (`status`, `deleted_at`, `deadline_at`);

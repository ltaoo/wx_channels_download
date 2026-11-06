-- Reusable scheduled workflow execution: persisted crontab rules and the run
-- history produced by them.
CREATE TABLE IF NOT EXISTS `flow_schedule` (
  `id` TEXT PRIMARY KEY,
  `name` TEXT NOT NULL,
  `description` TEXT,
  `cron_expr` TEXT NOT NULL,
  `flow_id` TEXT NOT NULL,
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
  `trigger_type` TEXT,
  `trigger_key` TEXT,
  `status` TEXT NOT NULL,
  `current_node` TEXT,
  `node_attempts` TEXT,
  `node_outputs` TEXT,
  `error` TEXT,
  `started_at` INTEGER,
  `completed_at` INTEGER,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS `idx_flow_run_record_schedule_id`
ON `flow_run_record` (`schedule_id`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_flow_id`
ON `flow_run_record` (`flow_id`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_status`
ON `flow_run_record` (`status`);
CREATE INDEX IF NOT EXISTS `idx_flow_run_record_deleted_at`
ON `flow_run_record` (`deleted_at`);

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

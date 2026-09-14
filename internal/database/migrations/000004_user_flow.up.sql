-- User-defined pipelines: JSON flow definitions that are registered into the
-- shared flow engine so schedules can trigger them like built-in flows.
CREATE TABLE IF NOT EXISTS `user_flow` (
  `id` TEXT PRIMARY KEY,
  `name` TEXT NOT NULL,
  `description` TEXT,
  `trigger_type` TEXT NOT NULL DEFAULT 'Cron',
  `event_key` TEXT,
  `definition` TEXT,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` INTEGER
);

CREATE INDEX IF NOT EXISTS `idx_user_flow_deleted_at`
ON `user_flow` (`deleted_at`);

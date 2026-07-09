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

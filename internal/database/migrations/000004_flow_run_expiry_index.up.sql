CREATE INDEX IF NOT EXISTS `idx_flow_run_record_status_deleted_deadline`
ON `flow_run_record` (`status`, `deleted_at`, `deadline_at`);

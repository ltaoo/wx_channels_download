-- Drop the tag tables first because content_tag/account_tag reference tag.
DROP TABLE IF EXISTS `account_tag`;
DROP TABLE IF EXISTS `content_tag`;
DROP TABLE IF EXISTS `tag`;

-- Drop the scheduled workflow tables. Run history is not preserved because it
-- only ever referenced the schedule and flow identifiers it was created with.
DROP INDEX IF EXISTS `idx_flow_run_record_deleted_at`;
DROP INDEX IF EXISTS `idx_flow_run_record_status`;
DROP INDEX IF EXISTS `idx_flow_run_record_flow_id`;
DROP INDEX IF EXISTS `idx_flow_run_record_schedule_id`;
DROP TABLE IF EXISTS `flow_run_record`;

DROP INDEX IF EXISTS `idx_flow_schedule_deleted_at`;
DROP INDEX IF EXISTS `idx_flow_schedule_next_run_at`;
DROP INDEX IF EXISTS `idx_flow_schedule_flow_id`;
DROP TABLE IF EXISTS `flow_schedule`;

DROP INDEX IF EXISTS idx_flow_run_record_retry_of;
DROP INDEX IF EXISTS idx_flow_run_record_deadline_at;
ALTER TABLE flow_run_record DROP COLUMN retry_of;
ALTER TABLE flow_run_record DROP COLUMN deadline_at;
ALTER TABLE flow_run_record DROP COLUMN timeout_sec;
ALTER TABLE flow_run_record DROP COLUMN node_logs;
ALTER TABLE flow_run_record DROP COLUMN checkpoint;
ALTER TABLE flow_run_record DROP COLUMN initial_data;
ALTER TABLE flow_run_record DROP COLUMN definition;

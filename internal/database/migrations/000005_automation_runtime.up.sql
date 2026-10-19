ALTER TABLE flow_run_record ADD COLUMN definition TEXT;
ALTER TABLE flow_run_record ADD COLUMN initial_data TEXT;
ALTER TABLE flow_run_record ADD COLUMN checkpoint TEXT;
ALTER TABLE flow_run_record ADD COLUMN node_logs TEXT;
ALTER TABLE flow_run_record ADD COLUMN timeout_sec INTEGER NOT NULL DEFAULT 3600;
ALTER TABLE flow_run_record ADD COLUMN deadline_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE flow_run_record ADD COLUMN retry_of TEXT;
CREATE INDEX IF NOT EXISTS idx_flow_run_record_deadline_at ON flow_run_record(deadline_at);
CREATE INDEX IF NOT EXISTS idx_flow_run_record_retry_of ON flow_run_record(retry_of);

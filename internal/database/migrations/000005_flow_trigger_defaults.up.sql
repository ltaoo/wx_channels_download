ALTER TABLE `user_flow` ADD COLUMN `default_triggers` TEXT NOT NULL DEFAULT '[]';
-- Existing instances keep their current rules until inheritance is explicitly enabled.
ALTER TABLE `user_flow_instance` ADD COLUMN `inherit_triggers` BOOLEAN NOT NULL DEFAULT 0;

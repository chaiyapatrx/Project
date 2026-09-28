ALTER TABLE `usage_logs`
    ADD COLUMN `session_ends_at` DATETIME NULL AFTER `end_time`,
    ADD INDEX `idx_usage_active_sessions` (`computer_id`, `end_time`, `session_ends_at`);

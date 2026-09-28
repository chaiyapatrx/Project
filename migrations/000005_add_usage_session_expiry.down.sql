ALTER TABLE `usage_logs`
    DROP INDEX `idx_usage_active_sessions`,
    DROP COLUMN `session_ends_at`;

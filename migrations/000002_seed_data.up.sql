-- =============================================================================
-- Migration: 000002_seed_data.up.sql
-- Description: Idempotent, NON-destructive seed for production.
--
-- IMPORTANT:
--   * Safe to run on a database that already contains real data. It never
--     deletes users, computers, bookings, or logs.
--   * The super admin account is NOT seeded here. It is provisioned by the Go
--     backend from SADMIN_USERNAME / SADMIN_PASSWORD env vars
--     (see internal/database/mysql.go), so no password hash lives in source.
--   * Real computers auto-register via the agent or are created by an admin.
-- =============================================================================

-- Seed default system settings only. Idempotent: existing admin-customized
-- values are preserved (we do not overwrite on key conflict).
INSERT INTO `system_settings` (`setting_key`, `setting_value`, `description`) VALUES
('session_duration', '120', 'Maximum usage duration per session (minutes)'),
('maintenance_mode', 'false', 'Temporarily put the system into maintenance mode')
ON DUPLICATE KEY UPDATE `setting_key` = `setting_key`;

-- =============================================================================
-- Migration: 000001_init_schema.down.sql
-- =============================================================================

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS `audit_logs`;
DROP TABLE IF EXISTS `system_settings`;
DROP TABLE IF EXISTS `usage_logs`;
DROP TABLE IF EXISTS `bookings`;
DROP TABLE IF EXISTS `computers`;
DROP TABLE IF EXISTS `users`;

-- Drop obsolete tables if they exist
DROP TABLE IF EXISTS `computer_commands`;

SET FOREIGN_KEY_CHECKS = 1;

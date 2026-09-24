-- =============================================================================
-- Combined Migration Script: init_database.sql
-- Run this directly in MySQL Workbench / phpMyAdmin / CLI
-- =============================================================================

-- NOTE: The target database is selected by the connection (DB_NAME), so this
-- script deliberately does not hardcode a `USE <db>` statement.
-- Use apply_migrations.py for upgrades: CREATE TABLE IF NOT EXISTS does not
-- add new columns to tables that already exist.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 1. Table: users
CREATE TABLE IF NOT EXISTS `users` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `username` VARCHAR(50) NOT NULL,
    `password_hash` VARCHAR(255) NOT NULL,
    `full_name` VARCHAR(100) NOT NULL,
    `role` ENUM('admin', 'staff', 'executive', 'student') NOT NULL DEFAULT 'student',
    `department` VARCHAR(100) NULL,
    `user_type` VARCHAR(50) NULL,
    `is_active` BOOLEAN NOT NULL DEFAULT TRUE,
    `token_version` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `idx_users_username` (`username`),
    INDEX `idx_users_role` (`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Table: computers
CREATE TABLE IF NOT EXISTS `computers` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(50) NOT NULL,
    `hwid` VARCHAR(100) NULL,
    `agent_secret_hash` CHAR(64) NULL,
    `ip_address` VARCHAR(45) NULL,
    `mac_address` VARCHAR(50) NULL,
    `status` ENUM('available', 'in_use', 'maintenance', 'disabled') NOT NULL DEFAULT 'available',
    `is_active` BOOLEAN NOT NULL DEFAULT TRUE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `idx_computers_name` (`name`),
    UNIQUE KEY `idx_computers_hwid` (`hwid`),
    INDEX `idx_computers_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Table: bookings
CREATE TABLE IF NOT EXISTS `bookings` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `user_id` INT NOT NULL,
    `computer_id` INT NOT NULL,
    `start_time` DATETIME NOT NULL,
    `end_time` DATETIME NOT NULL,
    `access_code` VARCHAR(10) NULL,
    `status` ENUM('pending', 'active', 'completed', 'cancelled', 'expired') NOT NULL DEFAULT 'pending',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY (`computer_id`) REFERENCES `computers`(`id`) ON DELETE CASCADE,
    INDEX `idx_bookings_user` (`user_id`),
    INDEX `idx_bookings_computer` (`computer_id`),
    INDEX `idx_bookings_status` (`status`),
    INDEX `idx_bookings_time_range` (`start_time`, `end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. Table: usage_logs
CREATE TABLE IF NOT EXISTS `usage_logs` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `user_id` INT NOT NULL,
    `computer_id` INT NOT NULL,
    `booking_id` INT NULL,
    `start_time` DATETIME NOT NULL,
    `end_time` DATETIME NULL,
    `duration_minutes` INT NOT NULL DEFAULT 0,
    `termination_reason` VARCHAR(50) NOT NULL DEFAULT 'normal',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY (`computer_id`) REFERENCES `computers`(`id`) ON DELETE CASCADE,
    INDEX `idx_usage_user` (`user_id`),
    INDEX `idx_usage_computer` (`computer_id`),
    INDEX `idx_usage_start_time` (`start_time`),
    INDEX `idx_usage_duration` (`duration_minutes`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. Table: system_settings
CREATE TABLE IF NOT EXISTS `system_settings` (
    `setting_key` VARCHAR(50) PRIMARY KEY,
    `setting_value` TEXT NOT NULL,
    `description` VARCHAR(255) NULL,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 6. Table: audit_logs
CREATE TABLE IF NOT EXISTS `audit_logs` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `user_id` INT NULL,
    `action` VARCHAR(50) NOT NULL,
    `target_type` VARCHAR(50) NULL,
    `target_id` VARCHAR(50) NULL,
    `ip_address` VARCHAR(45) NULL,
    `details` TEXT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_audit_action` (`action`),
    INDEX `idx_audit_user` (`user_id`),
    INDEX `idx_audit_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;

-- =============================================================================
-- SEED DATA (idempotent, NON-destructive)
--
-- This script is safe to run on an existing production database:
--   * Tables use CREATE TABLE IF NOT EXISTS.
--   * No DELETE/TRUNCATE of real data (users, computers, bookings, logs).
--   * The super admin is provisioned by the Go backend from
--     SADMIN_USERNAME / SADMIN_PASSWORD env vars (no hash in source).
--   * Real computers auto-register via the agent or are created by an admin.
-- =============================================================================

-- System Settings (preserve any admin-customized values on conflict)
INSERT INTO `system_settings` (`setting_key`, `setting_value`, `description`) VALUES
('session_duration', '120', 'ระยะเวลาการใช้งานสูงสุดต่อรอบ (นาที)'),
('maintenance_mode', 'false', 'โหมดปิดปรับปรุงระบบชั่วคราว')
ON DUPLICATE KEY UPDATE `setting_key` = `setting_key`;

-- Real stations auto-register via the agent or are created by an admin.
-- (No mock/seed computers, and no destructive DELETE here.)

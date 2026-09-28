ALTER TABLE `computers` ADD COLUMN `agent_version` VARCHAR(32) NULL AFTER `agent_secret_hash`;
ALTER TABLE `computers` ADD COLUMN `agent_update_error` VARCHAR(255) NULL AFTER `agent_version`;

CREATE TABLE IF NOT EXISTS `agent_releases` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `version` VARCHAR(32) NOT NULL,
    `sha256` CHAR(64) NOT NULL,
    `file_size` BIGINT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY `idx_agent_releases_version` (`version`),
    UNIQUE KEY `idx_agent_releases_sha256` (`sha256`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

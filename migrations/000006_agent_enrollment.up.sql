ALTER TABLE `computers`
    ADD COLUMN `pending_approval` BOOLEAN NOT NULL DEFAULT FALSE AFTER `is_active`,
    ADD INDEX `idx_computers_pending_approval` (`pending_approval`);

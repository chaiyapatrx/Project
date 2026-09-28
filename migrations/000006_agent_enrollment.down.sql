ALTER TABLE `computers`
    DROP INDEX `idx_computers_pending_approval`,
    DROP COLUMN `pending_approval`;

DROP TABLE IF EXISTS `agent_releases`;
ALTER TABLE `computers` DROP COLUMN `agent_version`;
ALTER TABLE `computers` DROP COLUMN `agent_update_error`;

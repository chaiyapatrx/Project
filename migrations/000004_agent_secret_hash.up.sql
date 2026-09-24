-- Store only a hash of each machine's high-entropy agent credential.
ALTER TABLE `computers` ADD COLUMN `agent_secret_hash` CHAR(64) NULL AFTER `hwid`;

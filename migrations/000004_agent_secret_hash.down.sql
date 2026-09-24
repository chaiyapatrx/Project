-- Removing credentials does not affect booking history, but requires an outage
-- while every agent is provisioned again after rollback.
ALTER TABLE `computers` DROP COLUMN `agent_secret_hash`;

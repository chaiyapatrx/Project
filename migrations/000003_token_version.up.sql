-- Add server-side token revocation for existing databases.
ALTER TABLE `users` ADD COLUMN `token_version` INT NOT NULL DEFAULT 0;

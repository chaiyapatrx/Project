-- Keep token_version when rolling back the application so old tokens remain revoked.
-- Older application versions ignore this extra column.
SELECT 1;

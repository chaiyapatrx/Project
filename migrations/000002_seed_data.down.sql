-- =============================================================================
-- Migration: 000002_seed_data.down.sql
-- =============================================================================

-- No safe automatic rollback exists: the up migration preserves values when
-- keys already exist, so a down migration cannot distinguish seeded rows from
-- settings created or customized by an administrator.
SELECT 1;

-- =========================================
-- Down migration for workers
-- =========================================

-- Drop indexes (если они ещё существуют)
DROP INDEX IF EXISTS idx_workers_status;
DROP INDEX IF EXISTS idx_workers_last_seen;

-- Drop table
DROP TABLE IF EXISTS workers CASCADE;

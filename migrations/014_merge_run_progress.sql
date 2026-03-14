-- 014_merge_run_progress.sql
-- SC-002 (phase 1): add progress columns to run and backfill from run_progress.
--
-- NOTE: Non-destructive pass. run_progress is kept for compatibility until all
-- query paths are migrated.

ALTER TABLE run ADD COLUMN badge_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE run ADD COLUMN current_location_id INTEGER REFERENCES location(id);
ALTER TABLE run ADD COLUMN progress_updated_at TEXT;

UPDATE run
SET badge_count = COALESCE((
        SELECT rp.badge_count
        FROM run_progress rp
        WHERE rp.run_id = run.id
    ), badge_count),
    current_location_id = COALESCE((
        SELECT rp.current_location_id
        FROM run_progress rp
        WHERE rp.run_id = run.id
    ), current_location_id),
    progress_updated_at = COALESCE((
        SELECT rp.updated_at
        FROM run_progress rp
        WHERE rp.run_id = run.id
    ), progress_updated_at)
WHERE EXISTS (SELECT 1 FROM run_progress rp WHERE rp.run_id = run.id);

-- Migration 004: soft-archive support for runs.
-- archived_at is NULL for active runs; set to a timestamp when the user archives.

ALTER TABLE run ADD COLUMN archived_at TEXT DEFAULT NULL;

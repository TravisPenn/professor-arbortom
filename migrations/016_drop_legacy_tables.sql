-- 016_drop_legacy_tables.sql
-- SC-001/SC-002/SC-003 (phase 2): drop the legacy tables that were superseded by
-- migrations 013-015. All data has already been migrated into the consolidated
-- tables (pokemon, run.badge_count/current_location_id, run_setting).
--
-- PRAGMA foreign_keys is disabled for this migration because:
--   1. game_starter.form_id references pokemon_form, which we are dropping.
--   2. Several tables (encounter, learnset_entry, etc.) reference pokemon_form
--      in their schema; SQLite does not enforce FK on DROP TABLE but some
--      driver versions do perform an implicit check.
-- We rebuild game_starter to reference the new unified pokemon table before
-- the legacy tables are dropped.

PRAGMA foreign_keys = OFF;

-- SC-001a: rebuild game_starter to point at pokemon(id) instead of pokemon_form(id).
-- All existing form_id values (1,4,7,252,255,258) are already present in
-- pokemon (migrated by 013), so the copy is safe.
CREATE TABLE IF NOT EXISTS game_starter_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id INTEGER NOT NULL REFERENCES game_version(id),
    form_id    INTEGER NOT NULL REFERENCES pokemon(id),
    priority   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(version_id, form_id)
);
INSERT OR IGNORE INTO game_starter_new (id, version_id, form_id, priority)
    SELECT id, version_id, form_id, priority FROM game_starter;
DROP TABLE game_starter;
ALTER TABLE game_starter_new RENAME TO game_starter;

-- SC-001b: drop the five normalised Pokemon reference tables.
-- Data is now in the pokemon table (migration 013).
DROP TABLE IF EXISTS pokemon_ability;
DROP TABLE IF EXISTS pokemon_stats;
DROP TABLE IF EXISTS pokemon_type;
DROP TABLE IF EXISTS pokemon_form;
DROP TABLE IF EXISTS pokemon_species;

-- SC-002: drop the separate run progress table.
-- badge_count, current_location_id, and progress_updated_at are now columns on run
-- (migration 014).
DROP TABLE IF EXISTS run_progress;

-- SC-003: drop the three rule/flag tables.
-- All data is now in run_setting (migration 015).
DROP TABLE IF EXISTS run_rule;
DROP TABLE IF EXISTS rule_def;
DROP TABLE IF EXISTS run_flag;

PRAGMA foreign_keys = ON;

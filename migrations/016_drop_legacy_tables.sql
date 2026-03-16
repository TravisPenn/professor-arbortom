-- 016_drop_legacy_tables.sql
-- SC-001/SC-002/SC-003 (phase 2): drop the legacy tables that were superseded by
-- migrations 013–015. All data has already been migrated into the consolidated
-- tables (pokemon, run.badge_count/current_location_id, run_setting).
--
-- This brings the total table count from 25 down to 18.

-- SC-001: drop the five normalised Pokémon reference tables.
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

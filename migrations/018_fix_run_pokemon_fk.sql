-- 018_fix_run_pokemon_fk.sql
-- SC-001 (phase 2): rebuild tables whose form_id FK still references the
-- now-dropped pokemon_form table.  Affected: run_pokemon, encounter,
-- learnset_entry, evolution_condition.

PRAGMA foreign_keys = OFF;

CREATE TABLE run_pokemon_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id           INTEGER NOT NULL REFERENCES run(id),
    form_id          INTEGER NOT NULL REFERENCES pokemon(id),
    level            INTEGER NOT NULL CHECK(level BETWEEN 1 AND 100),
    met_location_id  INTEGER REFERENCES location(id),
    is_alive         INTEGER NOT NULL DEFAULT 1,
    in_party         INTEGER NOT NULL DEFAULT 0,
    party_slot       INTEGER CHECK(party_slot BETWEEN 1 AND 6),
    moves_json       TEXT NOT NULL DEFAULT '[]',
    held_item_id     INTEGER REFERENCES item(id),
    acquisition_type TEXT NOT NULL DEFAULT 'manual',
    caught_level     INTEGER
);

INSERT INTO run_pokemon_new
    SELECT id, run_id, form_id, level, met_location_id, is_alive, in_party,
           party_slot, moves_json, held_item_id, acquisition_type, caught_level
    FROM run_pokemon;

DROP TABLE run_pokemon;
ALTER TABLE run_pokemon_new RENAME TO run_pokemon;

CREATE UNIQUE INDEX IF NOT EXISTS ux_run_pokemon_party_slot
    ON run_pokemon(run_id, party_slot) WHERE in_party = 1;

-- ── encounter ────────────────────────────────────────────────────────────────

CREATE TABLE encounter_new (
    id              INTEGER PRIMARY KEY,
    location_id     INTEGER NOT NULL REFERENCES location(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon(id),
    min_level       INTEGER NOT NULL,
    max_level       INTEGER NOT NULL,
    method          TEXT NOT NULL,
    conditions_json TEXT NOT NULL DEFAULT '[]'
);

INSERT INTO encounter_new SELECT * FROM encounter;
DROP TABLE encounter;
ALTER TABLE encounter_new RENAME TO encounter;

-- ── learnset_entry ───────────────────────────────────────────────────────────

CREATE TABLE learnset_entry_new (
    id               INTEGER PRIMARY KEY,
    form_id          INTEGER NOT NULL REFERENCES pokemon(id),
    version_group_id INTEGER NOT NULL,
    move_id          INTEGER NOT NULL REFERENCES move(id),
    learn_method     TEXT NOT NULL,
    level_learned    INTEGER NOT NULL DEFAULT 0,
    UNIQUE(form_id, version_group_id, move_id, learn_method)
);

INSERT INTO learnset_entry_new SELECT * FROM learnset_entry;
DROP TABLE learnset_entry;
ALTER TABLE learnset_entry_new RENAME TO learnset_entry;

-- ── evolution_condition ──────────────────────────────────────────────────────

CREATE TABLE evolution_condition_new (
    id              INTEGER PRIMARY KEY,
    from_form_id    INTEGER NOT NULL REFERENCES pokemon(id),
    to_form_id      INTEGER NOT NULL REFERENCES pokemon(id),
    trigger         TEXT NOT NULL,
    conditions_json TEXT NOT NULL DEFAULT '{}'
);

INSERT INTO evolution_condition_new SELECT * FROM evolution_condition;
DROP TABLE evolution_condition;
ALTER TABLE evolution_condition_new RENAME TO evolution_condition;

PRAGMA foreign_keys = ON;

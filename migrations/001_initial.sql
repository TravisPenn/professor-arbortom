-- PokemonProfessor initial schema
-- Applied by internal/db/migrate.go when PRAGMA user_version = 0
-- All CREATE TABLE use IF NOT EXISTS for idempotency.
-- All seed INSERTs use INSERT OR IGNORE.

-- ── Reference / Game Data Tables ────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS game_version (
    id               INTEGER PRIMARY KEY,
    name             TEXT NOT NULL UNIQUE,
    version_group_id INTEGER NOT NULL,
    generation_id    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pokemon_species (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS pokemon_form (
    id         INTEGER PRIMARY KEY,
    species_id INTEGER NOT NULL REFERENCES pokemon_species(id),
    form_name  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS move (
    id       INTEGER PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    type_name TEXT NOT NULL,
    power    INTEGER,
    accuracy INTEGER,
    pp       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS item (
    id       INTEGER PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    category TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS location (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    version_id INTEGER NOT NULL REFERENCES game_version(id),
    region     TEXT NOT NULL,
    UNIQUE(name, version_id)
);

CREATE TABLE IF NOT EXISTS encounter (
    id              INTEGER PRIMARY KEY,
    location_id     INTEGER NOT NULL REFERENCES location(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    min_level       INTEGER NOT NULL,
    max_level       INTEGER NOT NULL,
    method          TEXT NOT NULL,
    conditions_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS learnset_entry (
    id               INTEGER PRIMARY KEY,
    form_id          INTEGER NOT NULL REFERENCES pokemon_form(id),
    version_group_id INTEGER NOT NULL,
    move_id          INTEGER NOT NULL REFERENCES move(id),
    learn_method     TEXT NOT NULL,
    level_learned    INTEGER NOT NULL DEFAULT 0,
    UNIQUE(form_id, version_group_id, move_id, learn_method)
);

CREATE TABLE IF NOT EXISTS item_availability (
    id         INTEGER PRIMARY KEY,
    item_id    INTEGER NOT NULL REFERENCES item(id),
    location_id INTEGER NOT NULL REFERENCES location(id),
    version_id INTEGER NOT NULL REFERENCES game_version(id),
    method     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS evolution_condition (
    id           INTEGER PRIMARY KEY,
    from_form_id INTEGER NOT NULL REFERENCES pokemon_form(id),
    to_form_id   INTEGER NOT NULL REFERENCES pokemon_form(id),
    trigger      TEXT NOT NULL,
    conditions_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS api_cache_log (
    resource    TEXT NOT NULL,
    resource_id INTEGER NOT NULL,
    fetched_at  TEXT NOT NULL,
    PRIMARY KEY (resource, resource_id)
);

-- ── Run Tracking Tables ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS user (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES user(id),
    version_id INTEGER NOT NULL REFERENCES game_version(id),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_progress (
    run_id              INTEGER PRIMARY KEY REFERENCES run(id),
    current_location_id INTEGER REFERENCES location(id),
    badge_count         INTEGER NOT NULL DEFAULT 0,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_flag (
    run_id INTEGER NOT NULL REFERENCES run(id),
    key    TEXT NOT NULL,
    value  TEXT NOT NULL DEFAULT 'true',
    PRIMARY KEY (run_id, key)
);

CREATE TABLE IF NOT EXISTS run_party (
    run_id      INTEGER NOT NULL REFERENCES run(id),
    slot        INTEGER NOT NULL CHECK(slot BETWEEN 1 AND 6),
    form_id     INTEGER NOT NULL REFERENCES pokemon_form(id),
    level       INTEGER NOT NULL CHECK(level BETWEEN 1 AND 100),
    moves_json  TEXT NOT NULL DEFAULT '[]',
    held_item_id INTEGER REFERENCES item(id),
    PRIMARY KEY (run_id, slot)
);

CREATE TABLE IF NOT EXISTS run_box (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          INTEGER NOT NULL REFERENCES run(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    level           INTEGER NOT NULL,
    met_location_id INTEGER REFERENCES location(id),
    is_alive        INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS run_item (
    run_id  INTEGER NOT NULL REFERENCES run(id),
    item_id INTEGER NOT NULL REFERENCES item(id),
    qty     INTEGER NOT NULL DEFAULT 1 CHECK(qty >= 0),
    PRIMARY KEY (run_id, item_id)
);

CREATE TABLE IF NOT EXISTS rule_def (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS run_rule (
    run_id      INTEGER NOT NULL REFERENCES run(id),
    rule_def_id INTEGER NOT NULL REFERENCES rule_def(id),
    enabled     INTEGER NOT NULL DEFAULT 0,
    params_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (run_id, rule_def_id)
);

CREATE TABLE IF NOT EXISTS gen3_badge_cap (
    badge_count INTEGER PRIMARY KEY,
    level_cap   INTEGER NOT NULL
);

-- ── Indexes ──────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_encounter_location  ON encounter(location_id);
CREATE INDEX IF NOT EXISTS idx_encounter_form      ON encounter(form_id);
CREATE INDEX IF NOT EXISTS idx_learnset_form_vg    ON learnset_entry(form_id, version_group_id);
CREATE INDEX IF NOT EXISTS idx_evo_from            ON evolution_condition(from_form_id);
CREATE INDEX IF NOT EXISTS idx_run_box_run         ON run_box(run_id);
CREATE INDEX IF NOT EXISTS idx_run_item_run        ON run_item(run_id);
CREATE INDEX IF NOT EXISTS idx_location_version    ON location(version_id);

-- ── Seed Data ────────────────────────────────────────────────────────────────

-- Gen 3 game versions (IDs match PokeAPI — do not change)
INSERT OR IGNORE INTO game_version (id, name, version_group_id, generation_id) VALUES
  (6,  'ruby',       5, 3),
  (7,  'sapphire',   5, 3),
  (8,  'emerald',    6, 3),
  (10, 'firered',    7, 3),
  (11, 'leafgreen',  7, 3);

-- Built-in rule definitions
INSERT OR IGNORE INTO rule_def (key, description) VALUES
  ('nuzlocke',            'One catch per route; fainted Pokémon are permanently dead'),
  ('level_cap',           'Pokémon above the current badge level cap cannot be used'),
  ('no_trade_evolutions', 'Trade-based evolutions are banned'),
  ('theme_run',           'Only Pokémon matching a theme (type, colour, etc.) are allowed');

-- Gen 3 badge-based level caps
INSERT OR IGNORE INTO gen3_badge_cap (badge_count, level_cap) VALUES
  (0, 15),
  (1, 20),
  (2, 25),
  (3, 30),
  (4, 38),
  (5, 42),
  (6, 46),
  (7, 52),
  (8, 55);
-- badge_count = 9 (post-Champion): no cap — handled in Go as NULL return

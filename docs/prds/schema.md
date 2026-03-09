# PokemonProfessor — Database Schema

**File**: `migrations/001_initial.sql`
**Applied by**: `internal/db/migrate.go` on first start if `PRAGMA user_version = 0`

Cross-reference: [architecture.md](architecture.md) for module structure,
[api.md](api.md) for query consumers.

---

## Migration Runner Contract

`internal/db/migrate.go` must:

1. Open the DB at `POKEMON_DB_PATH` (create if not exists)
2. Execute `PRAGMA foreign_keys = ON` and `PRAGMA journal_mode = WAL` and
   `PRAGMA busy_timeout = 5000` on every connection open (not just migration)
3. Read `PRAGMA user_version`
4. If `user_version = 0`: execute `migrations/001_initial.sql`, then `PRAGMA user_version = 1`
5. If `user_version = 1`: no-op — already current
6. Future migrations follow the same pattern, incrementing the version number

The migration SQL file is embedded via `//go:embed migrations/001_initial.sql` in `migrate.go`.

---

## Schema

### Reference / Game Data Tables

These tables are populated by the PokeAPI layer and are never written to by the application logic.
They are the legality engine's source of truth.

```sql
-- Games and versions
CREATE TABLE IF NOT EXISTS game_version (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,   -- e.g. 'firered', 'ruby'
    version_group_id INTEGER NOT NULL,      -- FK to version group (5,6,7 for Gen 3)
    generation_id   INTEGER NOT NULL        -- always 3 for initial scope
);

-- Pokémon species (national dex entry)
CREATE TABLE IF NOT EXISTS pokemon_species (
    id              INTEGER PRIMARY KEY,    -- national dex number
    name            TEXT NOT NULL UNIQUE    -- e.g. 'bulbasaur'
);

-- Pokémon forms (species + form; form_id is used in party/box/encounters)
CREATE TABLE IF NOT EXISTS pokemon_form (
    id              INTEGER PRIMARY KEY,    -- PokeAPI pokemon.id (not species.id)
    species_id      INTEGER NOT NULL REFERENCES pokemon_species(id),
    form_name       TEXT NOT NULL           -- 'default', 'alolan', etc.
);

-- Moves
CREATE TABLE IF NOT EXISTS move (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    type_name       TEXT NOT NULL,
    power           INTEGER,                -- NULL for status moves
    accuracy        INTEGER,                -- NULL for moves that never miss
    pp              INTEGER NOT NULL
);

-- Items
CREATE TABLE IF NOT EXISTS item (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    category        TEXT NOT NULL           -- 'held-items', 'medicine', 'tm', etc.
);

-- Locations within a game version
CREATE TABLE IF NOT EXISTS location (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    version_id      INTEGER NOT NULL REFERENCES game_version(id),
    region          TEXT NOT NULL,          -- 'kanto', 'hoenn'
    UNIQUE(name, version_id)
);

-- Wild encounters at a location
CREATE TABLE IF NOT EXISTS encounter (
    id              INTEGER PRIMARY KEY,
    location_id     INTEGER NOT NULL REFERENCES location(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    min_level       INTEGER NOT NULL,
    max_level       INTEGER NOT NULL,
    method          TEXT NOT NULL,          -- 'walk', 'surf', 'fish-old', 'fish-good', 'fish-super'
    conditions_json TEXT NOT NULL DEFAULT '[]'  -- JSON array of condition strings e.g. ["time-morning"]
);

-- Moves learnable by a form in a version group
CREATE TABLE IF NOT EXISTS learnset_entry (
    id              INTEGER PRIMARY KEY,
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    version_group_id INTEGER NOT NULL,
    move_id         INTEGER NOT NULL REFERENCES move(id),
    learn_method    TEXT NOT NULL,          -- 'level-up', 'machine', 'tutor', 'egg'
    level_learned   INTEGER NOT NULL DEFAULT 0,  -- 0 for non-level-up methods
    UNIQUE(form_id, version_group_id, move_id, learn_method)
);

-- Where items can be obtained
CREATE TABLE IF NOT EXISTS item_availability (
    id              INTEGER PRIMARY KEY,
    item_id         INTEGER NOT NULL REFERENCES item(id),
    location_id     INTEGER NOT NULL REFERENCES location(id),
    version_id      INTEGER NOT NULL REFERENCES game_version(id),
    method          TEXT NOT NULL           -- 'buy', 'find', 'reward', 'tutor-cost'
);

-- Evolution conditions
CREATE TABLE IF NOT EXISTS evolution_condition (
    id              INTEGER PRIMARY KEY,
    from_form_id    INTEGER NOT NULL REFERENCES pokemon_form(id),
    to_form_id      INTEGER NOT NULL REFERENCES pokemon_form(id),
    trigger         TEXT NOT NULL,          -- 'level-up', 'use-item', 'trade', 'other'
    conditions_json TEXT NOT NULL DEFAULT '{}'
    -- conditions_json examples:
    --   level-up: {"min_level": 16}
    --   use-item: {"item_id": 83}        (Thunder Stone = 83)
    --   trade:    {"held_item_id": null}
    --   other:    {"friendship": true}
);

-- PokeAPI fetch cache — prevents duplicate requests
CREATE TABLE IF NOT EXISTS api_cache_log (
    resource        TEXT NOT NULL,          -- 'pokemon', 'location-area', 'item', 'evolution-chain'
    resource_id     INTEGER NOT NULL,
    fetched_at      TEXT NOT NULL,          -- ISO8601 timestamp
    PRIMARY KEY (resource, resource_id)
);
```

---

### Run Tracking Tables

These tables are written to by the application. They represent a user's active playthrough state.

```sql
-- Users
CREATE TABLE IF NOT EXISTS user (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Playthroughs
CREATE TABLE IF NOT EXISTS run (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES user(id),
    version_id      INTEGER NOT NULL REFERENCES game_version(id),
    name            TEXT NOT NULL,          -- e.g. 'FireRed Nuzlocke', 'Emerald Casual'
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Current progress within a run
CREATE TABLE IF NOT EXISTS run_progress (
    run_id          INTEGER PRIMARY KEY REFERENCES run(id),
    current_location_id INTEGER REFERENCES location(id),  -- NULL until first location set
    badge_count     INTEGER NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Story flags (version-specific events that gate content)
CREATE TABLE IF NOT EXISTS run_flag (
    run_id          INTEGER NOT NULL REFERENCES run(id),
    key             TEXT NOT NULL,
    value           TEXT NOT NULL DEFAULT 'true',
    PRIMARY KEY (run_id, key)
    -- example keys: 'story.gym1_defeated', 'hm.cut_obtained', 'hm.surf_obtained'
    -- 'hm.surf_obtained' gates surf encounters; 'hm.cut_obtained' gates cut-locked areas
);

-- Active party (up to 6 slots)
CREATE TABLE IF NOT EXISTS run_party (
    run_id          INTEGER NOT NULL REFERENCES run(id),
    slot            INTEGER NOT NULL CHECK(slot BETWEEN 1 AND 6),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    level           INTEGER NOT NULL CHECK(level BETWEEN 1 AND 100),
    moves_json      TEXT NOT NULL DEFAULT '[]',   -- JSON array of move IDs (max 4)
    held_item_id    INTEGER REFERENCES item(id),
    PRIMARY KEY (run_id, slot)
);

-- Box (all caught Pokémon including fainted for Nuzlocke)
CREATE TABLE IF NOT EXISTS run_box (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          INTEGER NOT NULL REFERENCES run(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    level           INTEGER NOT NULL,
    met_location_id INTEGER REFERENCES location(id),  -- NULL if starter/gift
    is_alive        INTEGER NOT NULL DEFAULT 1         -- 0 = fainted (Nuzlocke dead)
);

-- Inventory
CREATE TABLE IF NOT EXISTS run_item (
    run_id          INTEGER NOT NULL REFERENCES run(id),
    item_id         INTEGER NOT NULL REFERENCES item(id),
    qty             INTEGER NOT NULL DEFAULT 1 CHECK(qty >= 0),
    PRIMARY KEY (run_id, item_id)
);

-- Global rule catalogue (seeded)
CREATE TABLE IF NOT EXISTS rule_def (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    key             TEXT NOT NULL UNIQUE,
    description     TEXT NOT NULL
);

-- Per-run rule activation
CREATE TABLE IF NOT EXISTS run_rule (
    run_id          INTEGER NOT NULL REFERENCES run(id),
    rule_def_id     INTEGER NOT NULL REFERENCES rule_def(id),
    enabled         INTEGER NOT NULL DEFAULT 0,    -- 0/1 boolean
    params_json     TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (run_id, rule_def_id)
);
```

---

## Seed Data

All seed data is inserted in `001_initial.sql` using `INSERT OR IGNORE` (idempotent on re-run).

### `game_version` — Gen 3 rows

```sql
INSERT OR IGNORE INTO game_version (id, name, version_group_id, generation_id) VALUES
  (6,  'ruby',             5, 3),
  (7,  'sapphire',         5, 3),
  (8,  'emerald',          6, 3),
  (10, 'firered',          7, 3),
  (11, 'leafgreen',        7, 3);
```

*(IDs match PokeAPI `game/version` endpoint IDs — do not change.)*

### `rule_def` — Built-in rules

```sql
INSERT OR IGNORE INTO rule_def (key, description) VALUES
  ('nuzlocke',            'One catch per route; fainted Pokémon are permanently dead'),
  ('level_cap',           'Pokémon above the current badge level cap cannot be used'),
  ('no_trade_evolutions', 'Trade-based evolutions are banned'),
  ('theme_run',           'Only Pokémon matching a theme (type, colour, etc.) are allowed');
```

### `level_cap` — Gen 3 badge thresholds

This data drives the `level_cap` rule in `LegalAcquisitions`. It lives in the DB so it can be
inspected and extended without recompiling.

```sql
CREATE TABLE IF NOT EXISTS gen3_badge_cap (
    badge_count     INTEGER PRIMARY KEY,  -- 0 = no badges
    level_cap       INTEGER NOT NULL      -- max legal level under the cap rule
);

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
-- badge_count = 9 means "post-Champion": no cap. handled in Go as NULL return.
```

---

## Key Relationships

```
user ──< run >── game_version
run ──< run_progress
run ──< run_flag
run ──< run_party >── pokemon_form >── pokemon_species
run ──< run_box   >── pokemon_form
run ──< run_item  >── item
run ──< run_rule  >── rule_def

pokemon_form ──< encounter        >── location
pokemon_form ──< learnset_entry   >── move
pokemon_form ──< evolution_condition (from + to)

item ──< item_availability >── location

location >── game_version
```

---

## Index Recommendations

```sql
CREATE INDEX IF NOT EXISTS idx_encounter_location  ON encounter(location_id);
CREATE INDEX IF NOT EXISTS idx_encounter_form      ON encounter(form_id);
CREATE INDEX IF NOT EXISTS idx_learnset_form_vg    ON learnset_entry(form_id, version_group_id);
CREATE INDEX IF NOT EXISTS idx_evo_from            ON evolution_condition(from_form_id);
CREATE INDEX IF NOT EXISTS idx_run_box_run         ON run_box(run_id);
CREATE INDEX IF NOT EXISTS idx_run_item_run        ON run_item(run_id);
CREATE INDEX IF NOT EXISTS idx_location_version    ON location(version_id);
```

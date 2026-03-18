# PokemonProfessor — run_pokemon Data Continuity PRD

**Status**: Phase 1 + 1b + 2 Implemented
**Priority**: High
**Date**: 2026-03-14
**Last updated**: 2026-03-16

Cross-reference: [architecture.md](architecture.md), [schema.md](schema.md), [api.md](api.md)

---

## Executive Summary

This PRD covers two sequential phases of work.

**Phase 1 — Data Continuity (DC-001 – DC-007)** fixes three bugs caused by isolated
write paths into the `run_pokemon` table: level values recorded on Routes are overwritten
by Team edits; the same Pokémon can accumulate duplicate rows with conflicting data; and
there is no way to distinguish a wild catch from a starter from a gift from a manually
placed Pokémon. The concrete symptom: **Moltres logged on Routes at level 50 appears at
level 1 with no catch location in Box and Team** — indistinguishable from a gift Pokémon.

**Phase 2 — Schema Consolidation (SC-001 – SC-003)** reduces the table count from 25 to
18 by merging three groups of tables that model the same entity or are always accessed
together. This eliminates unnecessary JOIN depth in every handler query and removes
table-per-concern fragmentation that was added migration-by-migration without a
higher-level design review. Phase 2 depends on Phase 1 being complete and all handlers
already updated.

---

## Problem Statement

### Bug 1 — Level Overwrite (Routes log → Team edit)

1. Player logs Moltres caught at level 50 on Routes.
   `LogEncounter` inserts:
   ```
   run_pokemon (run_id=1, form_id=146, level=50, met_location_id=488, in_party=0)
   ```
2. Player opens Team, assigns Moltres to slot 1, enters level 1 (or the default).
   `UpdateTeam` finds the existing row (`pkmnID != 0`) and executes:
   ```sql
   UPDATE run_pokemon SET in_party=1, party_slot=1, level=1, ... WHERE id=<row>
   ```
3. **The Routes log now shows Moltres at level 1.** The catch level is permanently lost
   because `level` is the only level column and is shared by both write paths.

### Bug 2 — Duplicate Rows (Team first, Routes second)

1. Player opens Team, picks Moltres from the legal acquisitions list before logging the catch.
   `UpdateTeam` finds no existing `run_pokemon` row and inserts:
   ```
   run_pokemon (run_id=1, form_id=146, level=1, met_location_id=NULL, in_party=1, party_slot=1)
   ```
2. Player logs the catch retrospectively on Routes.
   `LogEncounter` always does an unconditional `INSERT`:
   ```
   run_pokemon (run_id=1, form_id=146, level=50, met_location_id=488, in_party=0)
   ```
3. **Box displays two Moltres rows.** Routes log shows two entries (ordered newest-first,
   so the legitimate catch row appears second). Team shows the level-1 row because it is
   the one flagged `in_party=1`.

### Bug 3 — Indistinguishable Acquisition Types

`run_pokemon` carries no record of *how* a Pokémon was obtained:

| Acquisition path | `met_location_id` | `level` | `in_party` | Observable symptom |
|---|---|---|---|---|
| Starter (run creation) | NULL | 5 | 1 | Looks like an untracked manual add |
| Wild catch (Routes) | set | caught level | 0 | Correct until Team edits it |
| Manual Team add (no Routes log) | NULL | user-entered | 1 | Identical to starter |
| Gift / event (future) | NULL or set | fixed | 0 | Indistinguishable from wild |

The Box and Team pages cannot display acquisition context. The Nuzlocke duplicate-catch
guard uses `met_location_id IS NOT NULL` as a proxy for "was this a wild catch," which
silently ignores its own starter and manual-add rows.

---

## Proposed Changes

### DC-001 · Schema — Migration 012 ✅

Add two columns to `run_pokemon`:

```sql
-- 012_pokemon_acquisition.sql

ALTER TABLE run_pokemon ADD COLUMN acquisition_type TEXT NOT NULL DEFAULT 'manual'
    CHECK(acquisition_type IN ('starter','wild','gift','trade','manual'));

ALTER TABLE run_pokemon ADD COLUMN caught_level INTEGER;
-- NULL = level was not recorded at time of acquisition (legacy rows, manual adds).
-- Set ONCE on INSERT. Never overwritten by Team edits.
```

**Backfill** (run once in migration 012 to classify existing rows):

```sql
-- Rows with a met_location_id were logged via Routes → wild catches.
UPDATE run_pokemon
SET acquisition_type = 'wild',
    caught_level = level
WHERE met_location_id IS NOT NULL
  AND acquisition_type = 'manual';

-- Rows at exactly level 5 with in_party=1, no location → starters.
-- Safe heuristic: no wild Gen 3 starter appears at level 5 in the field.
UPDATE run_pokemon
SET acquisition_type = 'starter',
    caught_level = level
WHERE met_location_id IS NULL
  AND in_party = 1
  AND level = 5
  AND acquisition_type = 'manual';
```

### DC-002 · Handler — `internal/handlers/runs.go` (starter insertion) ✅

Set `acquisition_type` and `caught_level` on the existing `INSERT`:

```go
// Before
db.Exec(`INSERT INTO run_pokemon
    (run_id, form_id, level, is_alive, in_party, party_slot, moves_json)
    VALUES (?, ?, 5, 1, 1, 1, '[]')`, runID, starterFormID)

// After
db.Exec(`INSERT INTO run_pokemon
    (run_id, form_id, level, caught_level, acquisition_type,
     is_alive, in_party, party_slot, moves_json)
    VALUES (?, ?, 5, 5, 'starter', 1, 1, 1, '[]')`, runID, starterFormID)
```

### DC-003 · Handler — `internal/handlers/routes.go` (`LogEncounter`) ✅

Replace the unconditional `INSERT` with an **upsert-or-merge** strategy:

1. Look for an existing `run_pokemon` row for the same `(run_id, form_id)` with
   `acquisition_type IN ('manual', 'wild')` (alive rows only). This covers the "Team
   first, Routes second" case.
2. **If found**: attach catch data to that row — set `met_location_id`, `caught_level`,
   `acquisition_type = 'wild'`. Leave `level` untouched (current in-game level may
   differ from caught level after grinding).
3. **If not found**: insert a new row with `acquisition_type = 'wild'` and
   `caught_level = level`.

```go
// Step 1: check for a promotable existing row.
var existingID int
db.QueryRow(`
    SELECT id FROM run_pokemon
    WHERE run_id = ? AND form_id = ? AND is_alive = 1
      AND acquisition_type IN ('manual', 'wild')
    ORDER BY id LIMIT 1
`, run.ID, formID).Scan(&existingID)

if existingID > 0 {
    // Step 2: merge catch data into the existing row.
    db.Exec(`
        UPDATE run_pokemon
        SET met_location_id  = ?,
            caught_level     = ?,
            acquisition_type = 'wild'
        WHERE id = ?
    `, metLocPtr, level, existingID)
} else {
    // Step 3: entirely new catch — insert.
    db.Exec(`
        INSERT INTO run_pokemon
            (run_id, form_id, level, caught_level, met_location_id,
             acquisition_type, is_alive)
        VALUES (?, ?, ?, ?, ?, 'wild', 1)
    `, run.ID, formID, level, level, metLocPtr)
}
```

> **Nuzlocke note**: the existing duplicate-location guard (`prevSpecies` check) runs
> *before* this block and remains unchanged. `gift` and `starter` rows are deliberately
> excluded from the merge match so they are never silently reclassified as `wild`.

### DC-004 · Handler — `internal/handlers/team.go` (`UpdateTeam`) ✅

Remove `level` from the `UPDATE` statement so a Team edit never overwrites the
Routes-logged level:

```go
// Before
db.Exec(`UPDATE run_pokemon
    SET in_party=1, party_slot=?, level=?, moves_json=?, held_item_id=?
    WHERE id=?`, slot, level, movesJSON, heldPtr, pkmnID)

// After
db.Exec(`UPDATE run_pokemon
    SET in_party=1, party_slot=?, moves_json=?, held_item_id=?
    WHERE id=?`, slot, movesJSON, heldPtr, pkmnID)
```

The `level` form input is preserved for use only on the **new-row INSERT path**
(when `pkmnID == 0` and no existing row exists):

```go
if pkmnID == 0 {
    res, err2 = db.Exec(`
        INSERT INTO run_pokemon
            (run_id, form_id, level, caught_level, acquisition_type,
             is_alive, in_party, party_slot, moves_json, held_item_id)
        VALUES (?, ?, ?, ?, 'manual', 1, 1, ?, ?, ?)`,
        run.ID, formID, level, level, slot, string(movesJSON), heldPtr)
}
```

### DC-005 · Handler — `internal/handlers/loaders.go` (`loadRouteLog`) ✅

Use `caught_level` (not `level`) in the Routes log query, falling back to `level` for
legacy rows where `caught_level` is NULL:

```sql
SELECT
    COALESCE(l.name, 'unknown')              AS loc_name,
    ps.name                                  AS species_name,
    rp.acquisition_type                      AS outcome,
    COALESCE(rp.caught_level, rp.level)      AS level,
    rp.met_location_id
FROM run_pokemon rp
JOIN pokemon_form pf ON pf.id = rp.form_id
JOIN pokemon_species ps ON ps.id = pf.species_id
LEFT JOIN location l ON l.id = rp.met_location_id
WHERE rp.run_id = ?
ORDER BY rp.id DESC
```

Update `RouteEntry.Outcome` to render `acquisition_type` as a human label —
`'wild'` → "Caught", `'starter'` → "Starter", `'gift'` → "Gift",
`'trade'` → "Trade", `'manual'` → "Added".

### DC-006 · Data Model — `internal/handlers/pages.go` ✅

Add `CaughtLevel` and `AcquisitionType` to `BoxEntry` so templates can render them:

```go
// Before
type BoxEntry struct {
    ID          int
    FormID      int
    SpeciesName string
    FormName    string
    Level       int
    MetLocation string
    IsAlive     bool
    Evolutions  []legality.Evolution
}

// After
type BoxEntry struct {
    ID              int
    FormID          int
    SpeciesName     string
    FormName        string
    Level           int
    CaughtLevel     *int   // nil for legacy/manual rows
    MetLocation     string
    AcquisitionType string // 'starter','wild','gift','trade','manual'
    IsAlive         bool
    Evolutions      []legality.Evolution
}
```

Update the `ShowBox` query in `team.go` to scan `rp.caught_level` and
`rp.acquisition_type` into the new fields.

### DC-007 · Templates ✅

**`templates/box.html`**

- Show `CaughtLevel` alongside the current level when they differ
  (e.g., "Lv 68 (caught Lv 50)").
- Render a small `AcquisitionType` badge next to the met-location field:
  "Starter", "Gift", "Trade" are highlighted; "Wild" and "Added" are plain text.

**`templates/routes.html`**

- Replace the hard-coded "caught" string in the Outcome column with the
  `acquisition_type` label from `RouteEntry.Outcome`.

---

## Phase 1b — Coach Display Improvements

The Coach page displays three data panels that need accuracy and usability fixes.
These changes modify the legality engine (`CoachMoves`), the handler (`buildCoachPage`),
the page structs, and the template — but require no schema changes.

### CD-001 · Available Pokémon — Deduplicate by Species, Show Level Range ✅

**Current behaviour**: the Acquisitions table lists one row per encounter row.
A location with five Oddish encounter methods (walk, surf, rod…) at different level
ranges produces five identical-looking rows. The table is noisy and unhelpful.

**Required behaviour**: group by `(species_name, form_name, location_name)` and show a
single row with the combined level range (min of all `min_level` → max of all `max_level`).
The method column is dropped or shows a comma-separated summary.

**Implementation** — `internal/handlers/coach.go` (`buildCoachPage`):

After the `LegalAcquisitions` call, aggregate rows by species+location:

```go
type acqKey struct{ species, form, location string }
grouped := map[acqKey]*legality.Acquisition{}
for _, a := range acqs {
    k := acqKey{a.SpeciesName, a.FormName, a.LocationName}
    if g, ok := grouped[k]; ok {
        if a.MinLevel < g.MinLevel { g.MinLevel = a.MinLevel }
        if a.MaxLevel > g.MaxLevel { g.MaxLevel = a.MaxLevel }
    } else {
        copy := a
        grouped[k] = &copy
    }
}
// Flatten back to slice — order by species name.
```

**Template** — `templates/coach.html`:

Remove the "Method" column from the Available Pokémon table. Change column header
from "Levels" to "Level Range".

### CD-002 · Party Moves — Filter Past Level-Up Moves ✅ (no change needed)

**Current behaviour**: `CoachMoves` already filters `level_learned <= currentLevel` for
level-up moves. However, the filter uses **strict less-than-or-equal**, which means a
move at the Pokémon's exact current level is hidden. This is correct — the player already
had the chance to learn it when they reached that level.

**No code change needed** — this filter is already working correctly. Confirmed by reading
`CoachMoves` line: `if mv.LearnMethod == "level-up" && currentLevel > 0 && mv.LevelLearned <= currentLevel`.

### CD-003 · Party Moves — Show All Evo Levels in Evo Column ✅

**Current behaviour**: `annotateEvoNotes` shows an evo note only when the evolution learns
the **same move** at a **different level**. It produces a note like "Ivysaur Lv22 ↑" but
does not show Venusaur's level for that same move, and does not show notes when the level
is the same.

**Required behaviour**: for every level-up move in the table, the Evo column should show
**all** evolution stages' levels for that move, regardless of whether the level differs.

Example: Bulbasaur Lv 5, move "Solar Beam" (learns at Lv 46):
- Evo column: "Ivysaur Lv46 · Venusaur Lv46"

Example: Bulbasaur Lv 5, move "Razor Leaf" (learns at Lv 19):
- Evo column: "Ivysaur Lv20 · Venusaur Lv20"

**Implementation** — `internal/legality/moves.go` (`annotateEvoNotes`):

Remove the `if evoLvl < moves[i].LevelLearned` / `else if evoLvl > ...` conditional.
Always append the evolution's level for any move it learns via level-up, including when
the level is identical:

```go
for _, evo := range evos {
    ls := evoLearnsets[evo.formID]
    evoLvl, ok := ls[moves[i].Name]
    if !ok {
        continue
    }
    label := capitalizeFirst(evo.name)
    notes = append(notes, fmt.Sprintf("%s Lv%d", label, evoLvl))
}
```

The ↑/↓ arrows are removed — the raw level numbers are more useful than a relative
indicator when the player is comparing across three stages.

### CD-004 · Party Moves — Include Future Evolution-Exclusive Moves ✅

**Current behaviour**: the "Your Party Moves" table only shows moves learnable by the
**current form**. If Ivysaur learns "Petal Dance" at Lv 44 but Bulbasaur does not learn
it at all, it is invisible — the player has no way to plan for it.

**Required behaviour**: after listing the current form's moves, append moves that are
**exclusive** to one or more evolutions (i.e., not in the current form's learnset at all).
These appear with a synthetic `LearnMethod` of `"evo-exclusive"` and the Evo column shows
which evolution learns it and at what level.

**Implementation** — `internal/legality/moves.go` (new function `appendEvoExclusiveMoves`):

Called at the end of `CoachMoves`, after `annotateEvoNotes`:

```go
func appendEvoExclusiveMoves(db *sql.DB, moves []Move, formID, versionGroupID int) ([]Move, error) {
    // 1. Load all direct evolution form IDs (same query as annotateEvoNotes).
    // 2. For each evolution, load its full learnset (all methods, not just level-up).
    // 3. Build a set of move names already in `moves`.
    // 4. For each evo move NOT in the current form's set:
    //    a. Create a Move with LearnMethod = "evo-exclusive"
    //    b. Set EvoNote = "<EvoName> Lv<X>" (level-up) or "<EvoName> (machine)" etc.
    //    c. Append to moves.
    // 5. Also walk second-stage evolutions (e.g., Venusaur from Ivysaur).
    return moves, nil
}
```

The function must also check the **full evolution chain** — not just direct evolutions.
For Bulbasaur: check both Ivysaur and Venusaur. This means querying
`evolution_condition` recursively (max depth 2 for Gen 3).

**Template** — `templates/coach.html`:

Rows with `LearnMethod == "evo-exclusive"` should be styled differently (muted background,
italic species name) to visually separate them from the current form's moves. The "Learn 
Method" cell shows the evolution name and level (e.g., "Venusaur Lv 65").

**Data model** — `internal/handlers/pages.go`:

No change to `MoveOption` needed — `LearnMethod` and `EvoNote` already exist. The
`LearnMethod` value `"evo-exclusive"` is new but the field is already a free-form string.

---

## Phase 2 — Schema Consolidation ✅

These changes reduce the table count from 25 to 18. They have no effect on application
behaviour — only on query complexity and schema legibility. All handlers must be updated
to reference the new table and column names.

### SC-001 · Merge Pokémon Reference Tables (5 → 1)

**Tables removed**: `pokemon_species`, `pokemon_form`, `pokemon_type`, `pokemon_stats`,
`pokemon_ability`

**New table**: `pokemon`

```sql
-- 013_merge_pokemon_tables.sql

CREATE TABLE IF NOT EXISTS pokemon (
    id           INTEGER PRIMARY KEY,   -- PokeAPI pokemon.id (was pokemon_form.id)
    species_name TEXT NOT NULL,         -- was pokemon_species.name
    form_name    TEXT NOT NULL,         -- was pokemon_form.form_name
    type1        TEXT NOT NULL,         -- primary type
    type2        TEXT,                  -- secondary type; NULL if mono-type
    hp           INTEGER NOT NULL DEFAULT 0,
    attack       INTEGER NOT NULL DEFAULT 0,
    defense      INTEGER NOT NULL DEFAULT 0,
    sp_attack    INTEGER NOT NULL DEFAULT 0,
    sp_defense   INTEGER NOT NULL DEFAULT 0,
    speed        INTEGER NOT NULL DEFAULT 0,
    ability1     TEXT,
    ability2     TEXT
);
```

**Rationale**: Gen 3 has essentially no alternate forms. `pokemon_type` stored two rows
per Pokémon (slot 1 / slot 2); `pokemon_stats` was always 1:1 with `pokemon_form`;
`pokemon_ability` stored at most two rows. Denormalising these into columns eliminates
five tables and three JOINs on every legality query. The `species_id` level of
normalisation (originally splitting species from form for PokeAPI alignment) is not
needed by any application query — species name is only displayed, never joined upon.

**Handler impact**: every query referencing `pokemon_form`, `pokemon_species`,
`pokemon_type`, `pokemon_stats`, or `pokemon_ability` is rewritten to reference `pokemon`.
The PokeAPI seeding layer (`internal/pokeapi/`) is updated to `INSERT INTO pokemon` instead
of the five separate tables.

**Data migration** (inside `013_merge_pokemon_tables.sql`):
```sql
INSERT INTO pokemon (id, species_name, form_name, type1, type2,
    hp, attack, defense, sp_attack, sp_defense, speed, ability1, ability2)
SELECT
    pf.id,
    ps.name,
    pf.form_name,
    COALESCE((SELECT type_name FROM pokemon_type WHERE form_id = pf.id AND slot = 1), 'normal'),
    (SELECT type_name FROM pokemon_type WHERE form_id = pf.id AND slot = 2),
    COALESCE(st.hp,        0),
    COALESCE(st.attack,    0),
    COALESCE(st.defense,   0),
    COALESCE(st.sp_attack, 0),
    COALESCE(st.sp_defense,0),
    COALESCE(st.speed,     0),
    (SELECT ability_name FROM pokemon_ability WHERE form_id = pf.id AND slot = 1),
    (SELECT ability_name FROM pokemon_ability WHERE form_id = pf.id AND slot = 2)
FROM pokemon_form pf
JOIN pokemon_species ps ON ps.id = pf.species_id
LEFT JOIN pokemon_stats st ON st.form_id = pf.id;

-- Retarget FKs (SQLite requires recreating tables with FKs; cascade via PRAGMA)
-- encounter, learnset_entry, evolution_condition, game_starter already reference
-- form_id; those column names remain valid since pokemon.id == old pokemon_form.id.
-- run_pokemon.form_id → rename to pokemon_id in the same migration.
```

### SC-002 · Merge `run` + `run_progress` (2 → 1)

**Tables removed**: `run_progress`

**Columns added to `run`**: `badge_count INTEGER NOT NULL DEFAULT 0`,
`current_location_id INTEGER REFERENCES location(id)`, `progress_updated_at TEXT`

```sql
-- 014_merge_run_progress.sql

ALTER TABLE run ADD COLUMN badge_count         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE run ADD COLUMN current_location_id INTEGER REFERENCES location(id);
ALTER TABLE run ADD COLUMN progress_updated_at TEXT;

UPDATE run
SET badge_count         = (SELECT badge_count         FROM run_progress WHERE run_id = run.id),
    current_location_id = (SELECT current_location_id FROM run_progress WHERE run_id = run.id),
    progress_updated_at = (SELECT updated_at           FROM run_progress WHERE run_id = run.id)
WHERE EXISTS (SELECT 1 FROM run_progress WHERE run_id = run.id);
```

**Rationale**: `run_progress` is always a strict 1:1 record with `run`. Every page that
needs badge count or current location must JOIN or do a second query anyway. Merging
eliminates that JOIN from `RunContextMiddleware`, `ShowProgress`, `UpdateProgress`, and
the run summary loader — the most frequently executed queries in the app.

**Handler impact**: `internal/handlers/progress.go` `UpdateProgress`, `ShowProgress`, and
`internal/handlers/middleware.go` `RunContextMiddleware` — replace
`SELECT ... FROM run_progress WHERE run_id = ?` with columns on `run`.

### SC-003 · Merge `run_flag` + `run_rule` + `rule_def` (3 → 1)

**Tables removed**: `run_flag`, `run_rule`, `rule_def`

**New table**: `run_setting`

```sql
-- 015_merge_run_settings.sql

CREATE TABLE IF NOT EXISTS run_setting (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES run(id),
    type   TEXT NOT NULL CHECK(type IN ('flag','rule')),
    key    TEXT NOT NULL,
    value  TEXT NOT NULL DEFAULT 'true',
    UNIQUE(run_id, type, key)
);

-- Migrate flags
INSERT INTO run_setting (run_id, type, key, value)
SELECT run_id, 'flag', key, value FROM run_flag;

-- Migrate rules (enabled=1 only; disabled rules are the default state)
INSERT INTO run_setting (run_id, type, key, value)
SELECT rr.run_id, 'rule', rd.key, rr.params_json
FROM run_rule rr
JOIN rule_def rd ON rd.id = rr.rule_def_id
WHERE rr.enabled = 1;
```

**Rationale**: `run_flag` stores boolean feature flags; `run_rule` stores enabled game
rules with a JSON params blob; `rule_def` is a lookup table referenced only to get the
string key. All three are per-run key/value config. A `type` discriminator column is
sufficient to distinguish them. The `rule_def` catalogue is replaced by the application
already knowing the set of valid rule keys — they are already hard-coded in
`internal/handlers/rules.go`.

**Handler impact**: `internal/handlers/rules.go`, `internal/handlers/loaders.go`
(`loadFlags`), and `RunContextMiddleware` — replace the three-table JOINs with:
```sql
SELECT key, value FROM run_setting WHERE run_id = ? AND type = 'rule'
SELECT key, value FROM run_setting WHERE run_id = ? AND type = 'flag'
```

---

## Migration Plan

#### Phase 1 — Data Continuity

Steps 1–2 are a hard dependency for all others. Steps 3–11 are independent after step 2.

| # | File | Action |
|---|---|---|
| 1 | `migrations/012_pokemon_acquisition.sql` | New file — `ALTER TABLE` adds `acquisition_type`, `caught_level`; backfill UPDATEs |
| 2 | `internal/db/migrate.go` | Register migration 012; bump max version to 12 |
| 3 | `docs/prds/schema.md` | Document the two new `run_pokemon` columns |
| 4 | `internal/handlers/runs.go` | Starter INSERT — add `acquisition_type='starter'`, `caught_level=5` |
| 5 | `internal/handlers/routes.go` | Replace unconditional INSERT with upsert-or-merge (DC-003) |
| 6 | `internal/handlers/team.go` | Remove `level` from UPDATE; new-row INSERT sets `acquisition_type='manual'` |
| 7 | `internal/handlers/loaders.go` | `loadRouteLog` — use `COALESCE(caught_level, level)`; expose `acquisition_type` |
| 8 | `internal/handlers/pages.go` | Add `CaughtLevel *int`, `AcquisitionType string` to `BoxEntry` |
| 9 | `templates/box.html` | Render acquisition badge; show caught level when it differs from current level |
| 10 | `templates/routes.html` | Display `acquisition_type` label in Outcome column |
| 11 | Tests | Update `bootstrapProgressDB` schema; add upsert-or-merge test for `LogEncounter` |

#### Phase 1b — Coach Display Improvements

No schema dependency. Can run in parallel with Phase 1 steps 3–11.

| # | File | Action |
|---|---|---|
| 1b-1 | `internal/handlers/coach.go` | Deduplicate acquisitions by species+location; aggregate level range (CD-001) |
| 1b-2 | `templates/coach.html` | Remove Method column from Available Pokémon table; rename Levels → Level Range |
| 1b-3 | `internal/legality/moves.go` | `annotateEvoNotes` — always show evo levels, remove ↑/↓ arrows (CD-003) |
| 1b-4 | `internal/legality/moves.go` | New `appendEvoExclusiveMoves` — add evo-exclusive moves to CoachMoves output (CD-004) |
| 1b-5 | `templates/coach.html` | Style `evo-exclusive` rows distinctly in Party Moves table |
| 1b-6 | Tests | Add test for evo-exclusive move inclusion; test deduplication of acquisitions |

#### Phase 2 — Schema Consolidation ✅

Depends on Phase 1 complete. SC-001, SC-002, SC-003 are independent of each other.

| # | File | Action |
|---|---|---|
| 12 | `migrations/013_merge_pokemon_tables.sql` | ✅ Create `pokemon`; migrate data from 5 tables |
| 13 | `migrations/014_merge_run_progress.sql` | ✅ Add progress columns to `run`; migrate data |
| 14 | `migrations/015_merge_run_settings.sql` | ✅ Create `run_setting`; migrate flags + rules |
| 15 | `migrations/016_drop_legacy_tables.sql` | ✅ Drop all 9 legacy tables (new file) |
| 16 | `internal/db/migrate.go` | ✅ Register migrations 013–016; bump max version to 16 |
| 17 | `internal/pokeapi/` | ✅ Update all seeders to write to `pokemon` only |
| 18 | `internal/handlers/` | ✅ Update all queries referencing removed tables |
| 19 | `internal/legality/` | ✅ Update all queries referencing `pokemon_form`, `pokemon_species`, `pokemon_type`, `pokemon_stats` |
| 20 | `docs/prds/schema.md` | ✅ Rewrite schema section to reflect 18-table target |
| 21 | Tests | ✅ Updated all test bootstrap schemas; 103/103 tests passing |

---

## Out of Scope

- **Editing acquisition type retroactively** — a future "edit catch details" UI is not
  part of this PRD. The migration backfill handles existing data adequately.
- **Multiple wild catches of the same species** — the upsert-or-merge merges into the
  *oldest* matching row. The Nuzlocke duplicate-location guard already prevents logging
  the same species twice at the same location; this case only arises in non-Nuzlocke runs
  where the player legitimately catches two of the same species.
- **Auto-inserting gift Pokémon from `in_game_trade`** — that table provides gift data to
  the Coach but is not yet wired to create `run_pokemon` rows automatically. That belongs
  in a separate Gift Acquisition PRD.
- **Level editing on existing Team Pokémon** — DC-004 removes the accidental level
  overwrite but does not add an explicit "update level" flow. If a player genuinely wants
  to record a level-up, the Routes page (or a future level-up log) is the correct place.
- **Further normalisation of `encounter` or `learnset_entry`** — those tables are already
  well-structured and are query targets, not consolidation candidates.
- **`in_game_trade` and `shop_item`** — these are small reference tables with no
  duplication; they remain as-is.

---

## Acceptance Criteria

### Phase 1

1. Logging Moltres caught at level 50 on Routes, then assigning it to the Team at level 1,
   results in:
   - Routes log shows level **50** (from `caught_level`).
   - Box shows "Lv 1 (caught Lv 50)" with location "Mt. Ember" and acquisition badge "Wild".
   - Team slot shows Moltres with its current in-party data unchanged by the Routes log.
2. Assigning Moltres to Team first (level 1, no Routes log), then logging the catch on Routes
   (level 50, Mt. Ember), results in **one row** in Box — not two — with `met_location_id`
   and `caught_level` populated and `acquisition_type = 'wild'`.
3. Starters created at run initialisation display a "Starter" badge in Box with no met-location.
4. All existing `run_pokemon` rows are backfilled with a non-NULL `acquisition_type`
   (verified by `SELECT acquisition_type, COUNT(*) FROM run_pokemon GROUP BY 1`).
5. All existing handler tests pass; new test for the upsert-or-merge path in `LogEncounter`
   (Team-first, Routes-second) passes.

### Phase 1b

6. The Available Pokémon table on Coach shows one row per distinct species+location
   regardless of how many encounter methods exist at that location. Level column shows
   the full range (e.g., "Lv 25–30").
7. The Party Moves table for Bulbasaur Lv 5 does not show moves with `level_learned <= 5`.
8. The Evo column for a move like "Razor Leaf" on Bulbasaur shows both "Ivysaur Lv20 ·
   Venusaur Lv20" — not just one evolution, and no ↑/↓ arrows.
9. The Party Moves table for Bulbasaur includes moves exclusive to Ivysaur and Venusaur
   (e.g., "Petal Dance") with a `LearnMethod` of `"evo-exclusive"` and the Evo column
   showing which evolution learns it and at what level.
10. Evo-exclusive moves are visually distinct from the current form's moves in the template.

### Phase 2

6. `SELECT COUNT(*) FROM sqlite_master WHERE type='table'` returns 18 (down from 25) after
   all three consolidation migrations are applied.
7. All legality queries (`LegalAcquisitions`, `LegalMoves`, `LegalItems`, `EvolutionOptions`)
   return identical results before and after the migration on a copy of the production DB.
8. The PokeAPI seeder successfully hydrates a fresh DB (no pre-existing data) using the new
   `pokemon` table — all species, types, stats, and abilities are present after seeding.
9. `run_progress` table is absent; `badge_count` and `current_location_id` are columns on
   `run` and `RunContextMiddleware` populates them without a second query.
10. `run_flag`, `run_rule`, and `rule_def` tables are absent; `run_setting` contains all
    migrated rows; Nuzlocke and all other rules behave identically in the UI.

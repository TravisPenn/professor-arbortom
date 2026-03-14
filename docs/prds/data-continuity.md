# PokemonProfessor — run_pokemon Data Continuity PRD

**Status**: Draft
**Priority**: High
**Date**: 2026-03-14

Cross-reference: [architecture.md](architecture.md), [schema.md](schema.md), [api.md](api.md)

---

## Executive Summary

The `run_pokemon` table is the single canonical record for every Pokémon a player owns.
Three independent code paths write to it — run creation (starters), Routes / `LogEncounter`,
and Team / `UpdateTeam` — but they were designed in isolation. The result is silent data
corruption: level values recorded in the Routes log are overwritten by Team edits; the same
Pokémon can have duplicate rows with conflicting data; and there is no way to distinguish a
wild catch from a starter from a gift from a manually placed Pokémon.

The concrete symptom that exposed this: **Moltres logged on Routes at level 50 appears at
level 1 with no catch location in Box and Team** — indistinguishable from a gift Pokémon.

This PRD specifies the schema additions and handler corrections needed to establish full data
continuity across all three write paths.

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

### DC-001 · Schema — Migration 012

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

### DC-002 · Handler — `internal/handlers/runs.go` (starter insertion)

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

### DC-003 · Handler — `internal/handlers/routes.go` (`LogEncounter`)

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

### DC-004 · Handler — `internal/handlers/team.go` (`UpdateTeam`)

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

### DC-005 · Handler — `internal/handlers/loaders.go` (`loadRouteLog`)

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

### DC-006 · Data Model — `internal/handlers/pages.go`

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

### DC-007 · Templates

**`templates/box.html`**

- Show `CaughtLevel` alongside the current level when they differ
  (e.g., "Lv 68 (caught Lv 50)").
- Render a small `AcquisitionType` badge next to the met-location field:
  "Starter", "Gift", "Trade" are highlighted; "Wild" and "Added" are plain text.

**`templates/routes.html`**

- Replace the hard-coded "caught" string in the Outcome column with the
  `acquisition_type` label from `RouteEntry.Outcome`.

---

## Migration Plan

Steps 1–2 are a hard dependency for all others. Steps 3–10 are independent after step 2.

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
| 11 | Tests | Update `bootstrapProgressDB` schema in `progress_routes_test.go`; add test for upsert-or-merge in `LogEncounter` |

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

---

## Acceptance Criteria

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

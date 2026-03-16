---
name: db-schema-migrations
description: "SQLite database layer, schema migrations, and PokeAPI seeding in internal/db/ and migrations/. Use when: writing new migrations, modifying schema, debugging migration runner, understanding table relationships, working with seeds/PokeAPI data flow, adding reference vs app-owned tables, troubleshooting WAL mode or connection pooling, or reviewing PRAGMA configuration."
---

# Database Schema & Migrations

The database layer (`internal/db/`, `migrations/`) manages a SQLite database with WAL mode, sequential migrations, and PokeAPI-sourced reference data seeding.

## Connection Setup (db.go)

### Pool & Pragmas

```
MaxOpenConns = 10  (concurrent readers in WAL mode)
MaxIdleConns = 2
```

Per-connection pragmas applied via custom `pragmaConnector` (implements `driver.Connector`):
- `journal_mode = WAL` — concurrent reads during writes
- `foreign_keys = ON` — enforce referential integrity
- `busy_timeout = 5000` — 5-second wait before BUSY error
- `synchronous = NORMAL` — reduced write fsync frequency

### Security (SEC-013)

`Open()` validates `dbPath`:
- `filepath.Clean()` normalization
- Rejects paths containing `".."` (directory traversal prevention)
- Creates parent directory if missing (`os.MkdirAll` with `0o755`)

## Migration System (migrate.go)

### Version Tracking

Uses `PRAGMA user_version` as single source of truth. Current version: **15**.

Each migration is applied sequentially via `if version == N` blocks. No rollback support — forward-only by design.

### File Naming Convention

```
NNN_description.sql
```

- `NNN` = zero-padded 3-digit number (001, 002, ..., 015)
- `description` = lowercase snake_case summary
- Files embedded via `//go:embed *.sql` in `migrations/embed.go`
- Read at runtime: `migrations.FS.ReadFile("NNN_description.sql")`

### Adding a New Migration

1. Create `migrations/NNN_description.sql` (next sequential number)
2. In `migrate.go`, add a new `if version == N` block after the last one
3. Call `setUserVersion(db, N+1)` at the end of the block
4. Update the `currentVersion` constant to `N+1`

### Security (SEC-004)

`setUserVersion()` bounds-checks values (0–1000) because PRAGMA statements cannot use parameterized queries — uses `fmt.Sprintf()` with `%d` only.

## Table Architecture

### Reference Tables (PokeAPI-sourced, seeded once)

| Table | Purpose |
|-------|---------|
| `game_version` | Gen 3 versions (FireRed=10, LeafGreen=11, etc.) with version_group_id |
| `pokemon_species` | Species names (e.g., "Bulbasaur") |
| `pokemon_form` | Form variants (abilities, stats, movesets) |
| `move` | Move definitions (name, type, power, accuracy, PP) |
| `item` | Item definitions (name, category) |
| `location` | Named places per version + region |
| `encounter` | Wild Pokémon at locations (level ranges, methods, conditions) |
| `learnset_entry` | Move learnsets per form + version_group (level-up/TM/HM/tutor/egg) |
| `item_availability` | Items findable at locations |
| `evolution_condition` | Evolution triggers + JSON conditions |
| `api_cache_log` | Tracks fetched PokeAPI resources |

Additional reference tables added by later migrations:
| `gen3_badge_cap` | Badge count → level cap mapping |
| `tm_move` / `hm_move` | TM/HM number → move name |
| `tutor_move` | Move tutor locations per version_group |
| `shop_item` | Shop inventory at locations (price, currency) |
| `in_game_trade` | NPC trades and Game Corner entries |
| `pokemon_type` | Form types with slot ordering |
| `pokemon_stats` | Base stats per form (HP, Atk, Def, SpA, SpD, Spe) |
| `pokemon_ability` | Abilities per form with slot |

### App-Owned Tables (user-created runs and state)

| Table | Purpose |
|-------|---------|
| `user` | Players |
| `run` | Playthroughs (FK to user + game_version) |
| `run_progress` | Current state: badge_count, current_location_id |
| `run_pokemon` | Caught Pokémon (party/box, level, nickname) |
| `run_pokemon_move` | Moves assigned to caught Pokémon (slot 1–4) |
| `run_flag` | Key-value story flags (legacy) |
| `run_rule` | Active challenge rules with params_json (legacy) |
| `run_setting` | Consolidated flag/rule storage (migration 015, replaces run_flag + run_rule) |

### Key Relationships

- `run.version_id` → `game_version.id` (determines available content)
- `run_pokemon.form_id` → `pokemon_form.id` (links to reference data)
- `encounter.version_id` filters by specific version; `learnset_entry.version_group_id` filters by version group
- Reference tables use `INSERT OR IGNORE` for idempotent seeding
- App tables use `AUTOINCREMENT` primary keys

## Seeding System (seeds.go)

### Seed Source Priority

`ApplySeedsIfEmpty()` checks if `location` table is empty, then:
1. **Filesystem** `seeds.sql` adjacent to DB file (only if `POKEMON_TRUST_FS_SEEDS=1` — opt-in for security, SEC-014)
2. **Embedded** bundled seeds compiled into binary (default, safe)
3. **Fallback:** skip; PokeAPI hydrates lazily on first queries

### Seed Format

- SQL INSERT statements, one per line ending in `";\n"`
- `INSERT OR IGNORE` for idempotency
- Wrapped in `PRAGMA foreign_keys = OFF` / `ON` for FK-safe bulk loading
- Comment lines (`--` prefix) and blank lines skipped

### Seed Order (FK-safe parent-before-children)

```
game_version → pokemon_species → pokemon_form → move → item → location →
encounter → learnset_entry → item_availability → evolution_condition → api_cache_log
```

### ExportSeeds()

Exports all reference table rows as SQL INSERT statements to `seeds.sql` file. Uses `validTableName` regex (`^[a-z_]+$`) for table name validation (SEC-004 — identifiers can't use SQL placeholders).

String escaping: single quotes doubled (`'` → `''`). Handles NULL, integers, floats, strings, bytes via `writeSQLLiteral()`.

## Models (models.go)

Key structs:
- `Run` — ID, Name, UserID, VersionID, CreatedAt
- `RunProgress` — RunID, CurrentLocationID (*int, optional), BadgeCount, UpdatedAt
- `ActiveRule` — Key, Enabled, ParamsJSON
- `GameVersion` — ID, Name, VersionGroupID, GenerationID
- `Location` — ID, Name, VersionID, Region
- `RuleDef` — ID, Key, Description

## Migration History

| # | Name | Purpose |
|---|------|---------|
| 001 | initial | Core schema: versions, species, forms, moves, items, locations, encounters, users, runs |
| 002 | starters | Starter Pokémon tracking |
| 003 | merge_pokemon | Consolidate Pokémon tables |
| 004 | archive_run | Run archival support |
| 005 | static_locations | Pre-populated location data |
| 006 | coach_improvements | Coach data enrichment |
| 007 | tm_moves | TM move mapping table |
| 008 | hm_tutor_moves | HM + tutor move tables |
| 009 | pokemon_types_stats | Type slots + base stats tables |
| 010 | current_moves | run_pokemon_move tracking |
| 011 | static_encounters | Pre-populated encounter data |
| 012 | pokemon_acquisition | Acquisition tracking |
| 013 | merge_pokemon_tables | Consolidate Pokémon storage |
| 014 | merge_run_progress | Merge progress tracking |
| 015 | merge_run_settings | Consolidated run_setting table (flag + rule in one) |

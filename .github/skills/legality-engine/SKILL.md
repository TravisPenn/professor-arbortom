---
name: legality-engine
description: "Gen 3 Pokémon legality engine in internal/legality/. Use when: fixing legality bugs, adding rules, debugging acquisition/move/evolution/item availability, working with type coverage analysis, modifying pathfinding, editing typechart, changing how badge-gated level caps work, or understanding version-specific filtering (FireRed vs Emerald)."
---

# Legality Engine

The legality engine (`internal/legality/`) determines what Pokémon, moves, items, and evolutions are legally obtainable at any point in a Gen 3 run. It is the core business logic of the application.

## Architecture

### Central Pattern: RunState

Every legality function starts by calling `LoadRunState(db, runID)` which loads a lightweight snapshot:
- **VersionID** / **VersionGroupID** — gates encounters (version) and learnsets (version group)
- **BadgeCount** — derives level cap via `gen3_badge_cap` table
- **LocationID** — filters location-bound acquisitions, items, trades
- **ActiveRules** — map of enabled rules with JSON params (nuzlocke, level_cap, etc.)
- **Flags** — story progression keys (`hm.cut_obtained`, `story.high_friendship`, etc.)

### Annotation Over Removal

`ApplyRules()` **never discards** blocked entries. It sets `BlockedByRule` (a `*string` pointer) with the rule name. The Coach layer reads this field to present unavailable options visually. Always check `BlockedByRule != nil` rather than filtering results.

## Module Map

| File | Entry Points | Purpose |
|------|-------------|---------|
| `skills.go` | `LoadRunState()` | Loads run context (version, badges, rules, flags) |
| `rules.go` | `LevelCap()`, `ApplyRules()` | Badge-derived caps; annotates acquisitions |
| `acquisitions.go` | `LegalAcquisitions()`, `LegalTrades()` | Catchable Pokémon + NPC trades at current location |
| `moves.go` | `LegalMoves()`, `CoachMoves()` | Learnable moves; Coach variant filters past moves and adds evo notes |
| `items.go` | `LegalItems()`, `ShopItems()` | Owned + obtainable + shop inventory at location |
| `evolutions.go` | `EvolutionOptions()` | Single-hop evolution options with legality annotations |
| `pathfind.go` | `LoadEvolutionGraph()`, `FindEvolutionPaths()`, `MoveDelayAnalysis()` | BFS multi-hop evolution chains; pre-evo vs post-evo move timing |
| `coverage.go` | `TeamDefensiveProfile()`, `TeamOffensiveCoverage()`, `CounterChain()` | Team type analysis (weaknesses, resistances, gaps) |
| `typechart.go` | `TypeEffectiveness()`, `DualTypeEffectiveness()` | 18-type effectiveness matrix (Gen 3+Fairy) |
| `types.go` | All structs | `Acquisition`, `Move`, `Item`, `Trade`, `Evolution`, `EvolutionPath`, `BaseStats`, `Warning`, etc. |

## Key Data Flow

### Moves for a Pokémon (typical Coach path)

```
CoachMoves(db, runID, formID, currentLevel)
├── LoadRunState(db, runID)
├── LevelCap(db, rs) → cap
├── Query learnset_entry WHERE form_id = ? AND version_group_id = ?
├── For each move:
│   ├── Skip egg moves (not actionable)
│   ├── Skip level-up moves where level_learned <= currentLevel (already passed)
│   ├── Annotate: level_learned > cap → BlockedByRule = "level_cap"
│   └── Annotate: HM without flag → BlockedByRule = "hm_flag_missing"
├── annotateEvoNotes() → adds EvoNote for evo-shared moves
└── appendEvoExclusiveMoves() → appends evo-only moves (depth 2)
```

### Version-Specific Filtering

- **VersionID** filters: encounters, locations, item availability, shop items, trades
- **VersionGroupID** filters: learnsets, evolution conditions, tutor moves
- Gen 3 version groups: FireRed/LeafGreen = 7, Ruby/Sapphire/Emerald = 8

### HM Gating

Eight HMs tracked as run flags: `hm.{move}_obtained` (cut, fly, surf, strength, flash, rock_smash, waterfall, dive). `hmBlockedRule()` checks flags in `LegalMoves()` and `CoachMoves()`.

## Rules System

Currently enforced rules via `ApplyRules()`:
- **level_cap** — min_level or level_learned exceeds badge-derived cap
- **nuzlocke** — location duplication check (partial; full check in handlers)
- **no_trade_evolutions** — blocks trade-triggered evolutions
- **hm_flag_missing** — HM move not yet obtained in story

Rule parameters stored as JSON in `run_rule.params_json`. Example: level cap override value.

## Evolution Pathfinding

- `LoadEvolutionGraph(db)` builds in-memory adjacency list (~200 edges)
- `FindEvolutionPaths(graph, fromFormID, rs, levelCap)` does BFS (max depth 5, cycle-safe via visited set)
- `evalStepFeasibility()` checks each edge: level cap, friendship flag, trade rule, item requirements
- `MoveDelayAnalysis()` compares pre-evo vs post-evo move learn levels (only analyzes first hop)

## Coverage Analysis

- `TeamDefensiveProfile(team)` — weaknesses, resistances, immunities, uncovered types
- `TeamOffensiveCoverage(team)` — which types can be hit super-effectively, which are gaps
- `CounterChain(threatType, team, maxDepth)` — multi-depth (1-3) type counter chains
- Type chart: 18 types including Fairy; effectiveness via `map[attack]map[defend]float64` (omitted = 1.0 neutral)
- `DualTypeEffectiveness()` = product of single-type lookups

## Trade System

`LegalTrades()` queries `in_game_trade` table:
- `give_species IS NULL` → Game Corner entry (method = "game-corner")
- `give_species IS NOT NULL` → NPC trade
- Filtered by current location

## Testing Patterns

Tests use `setupRunDB(t, cfg)` helper that creates minimal in-memory SQLite with schema + test data. Key assertion patterns:
- Check `BlockedByRule` pointer: `nil` (legal) vs `non-nil` (blocked with reason)
- Verify slice length and ordering (queries often `ORDER BY name`)
- Validate `Warning` codes for advisory conditions (e.g., `"no_location"`)

## Common Pitfalls

1. **Location is optional** — many queries return empty (not error) when `rs.LocationID == nil`
2. **Don't filter blocked entries** — Coach and UI rely on `BlockedByRule` annotations being present
3. **Egg moves excluded from CoachMoves** — only actionable moves included
4. **Evolution conditions are JSON** — `evolution_condition.conditions_json` holds arbitrary data (min_level, friendship, held_item_id)
5. **No caching** — `LoadEvolutionGraph()` rebuilds on every call; fine for small Gen 3 dataset
6. **MoveDelayAnalysis only checks first hop** — `path.Steps[0]`, not full chain

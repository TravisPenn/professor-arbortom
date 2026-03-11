# PokemonProfessor — Coach Intelligence Enrichment PRD

**Status**: Draft
**Priority**: High
**Date**: 2026-03-11

Cross-reference: [architecture.md](architecture.md) for module structure,
[schema.md](schema.md) for data model, [api.md](api.md) for route inventory.

---

## Executive Summary

The AI Coach (ZeroClaw integration) currently receives a limited payload: catchable Pokémon,
learnable moves, and available items — all scoped to the player's current location. It has no
knowledge of type effectiveness, no view of the team's current movesets or stats, and no ability
to reason about evolution paths or cross-generation strategy.

This PRD defines five enrichment areas that enable the Coach to answer sophisticated questions:

1. **Type effectiveness reasoning** — multi-hop coverage / weakness analysis
2. **Cross-generation Pokédex knowledge** — breeding chains, habitat overlaps, competitive context
3. **Evolution path-finding** — optimal paths given current items, level, and rules
4. **Team composition analysis** — base stats, abilities, current movesets, synergy
5. **Enriched Coach payload** — delivering all computed analysis to ZeroClaw

### Architecture Decision

All graph-like computation happens in Go. No graph database is introduced.

| Need | Solution | Rationale |
|------|----------|-----------|
| Type chart (18×18) | Go constant map | Static data, deterministic, ~50 lines |
| Evolution path-finding | In-memory adjacency list + BFS | Max ~1000 nodes, tree-structured, constraint logic already in Go |
| Cross-gen knowledge | LLM's built-in training data | Hybrid model: run-specific data is DB-grounded; general Pokémon facts use LLM knowledge |
| Stats / abilities | New DB columns on `pokemon_form` | Seeded from PokeAPI alongside existing form data |

SQLite remains the sole persistent store. The single-binary, `FROM scratch` deployment is preserved.

---

## COACH-001 · Type Effectiveness Engine (HIGH)

**Location**: New file `internal/legality/typechart.go`

### Problem

The Coach receives move type names (`"fire"`, `"water"`) but has no context about type relationships.
It cannot answer:
- "What are my team's weaknesses?"
- "Do I have coverage against Steel types?"
- "What resists the counter to my Water-type starter?"

Type effectiveness is a fixed 18×18 matrix (unchanged since Gen 6). The Coach currently relies
entirely on the LLM's training data for type reasoning, which is unreliable for multi-hop chains
(e.g., "what resists my counter's counter?").

### Required Actions

1. Define the 18 Pokémon types as string constants in `internal/legality/typechart.go`.
2. Encode the full 18×18 effectiveness matrix as a `map[string]map[string]float64` where values
   are `2.0` (super effective), `0.5` (not very effective), `0.0` (immune), or `1.0` (neutral).
3. Implement `TypeEffectiveness(attackType, defendType string) float64`.
4. Implement `DualTypeEffectiveness(attackType, defendType1, defendType2 string) float64` — returns
   the product of effectiveness against each defending type.

```go
// typechart.go

var effectiveness = map[string]map[string]float64{
    "normal":   {"rock": 0.5, "ghost": 0.0, "steel": 0.5},
    "fire":     {"fire": 0.5, "water": 0.5, "grass": 2.0, "ice": 2.0, "bug": 2.0, "rock": 0.5, "dragon": 0.5, "steel": 2.0},
    // ... all 18 types
}

func TypeEffectiveness(atk, def string) float64 { ... }
func DualTypeEffectiveness(atk, def1, def2 string) float64 { ... }
```

### Acceptance Criteria

- [ ] All 324 type matchups (18×18) are encoded. Values verified against Bulbapedia Gen 3+ chart.
- [ ] `TypeEffectiveness("fire", "grass")` returns `2.0`.
- [ ] `TypeEffectiveness("normal", "ghost")` returns `0.0`.
- [ ] `DualTypeEffectiveness("ground", "fire", "flying")` returns `1.0` (2.0 × 0.5, cancels).
- [ ] Unit tests cover super effective, not very effective, immune, and dual-type combinations.

---

## COACH-002 · Team Coverage Analysis (HIGH)

**Location**: New file `internal/legality/coverage.go`
**Depends on**: COACH-001 (type chart), COACH-004 (Pokémon types in DB)

### Problem

The Coach cannot assess whether a team is balanced. It doesn't know the team's collective offensive
coverage, shared defensive weaknesses, or which opponent types the team cannot hit effectively.

### Required Actions

1. Implement `TeamDefensiveProfile(team []TeamMember) DefensiveProfile` — for each of the 18 types,
   compute the best and worst effectiveness multiplier across the team.

```go
type TeamMember struct {
    SpeciesName string
    Type1       string
    Type2       string   // empty if mono-type
    MoveTypes   []string // types of known moves
}

type DefensiveProfile struct {
    // Weaknesses lists types that hit at least one team member for ≥2× damage
    // with no team member resisting (≤0.5×).
    Weaknesses []TypeThreat
    // Resistances lists types that no team member is weak to and at least one resists.
    Resistances []string
    // Immunities lists types that at least one team member is immune to.
    Immunities []string
    // Uncovered lists types where no team member has a super-effective move.
    Uncovered []string
}

type TypeThreat struct {
    Type          string
    WeakMembers   []string // species names hit for ≥2×
    BestResistance float64 // best multiplier any team member has (e.g. 0.25)
}
```

2. Implement `TeamOffensiveCoverage(team []TeamMember) OffensiveCoverage` — for each of the 18
   defending types, determine whether any team member's moves hit it super effectively.

```go
type OffensiveCoverage struct {
    // Covered lists defending types that at least one team member can hit for ≥2×.
    Covered []CoverageEntry
    // Gaps lists defending types where no team member has a super-effective move.
    Gaps []string
}

type CoverageEntry struct {
    DefendingType string
    CoveredBy     []string // "Charizard (Fire Blast)" format
}
```

3. Implement `CounterChain(threatType string, team []TeamMember, depth int) []CounterHop` —
   multi-hop analysis: given a type threat, find what counters it, then what counters the counter,
   up to `depth` hops (default 2, max 3).

```go
type CounterHop struct {
    Depth        int      // 1 = direct counter, 2 = counter's counter
    CounterTypes []string // types that are super effective at this depth
    TeamCovers   bool     // true if the team already has coverage at this depth
}
```

### Acceptance Criteria

- [ ] A team of 3 Water types reports Ground and Electric as `Weaknesses`.
- [ ] A team with no Fighting-type moves reports `Gaps` includes "normal", "dark", "steel", etc.
- [ ] `CounterChain("electric", waterTeam, 2)` returns Ground at depth 1, Water/Grass/Ice at depth 2.
- [ ] Analysis runs in <1ms for a 6-member team (in-memory computation, no DB).
- [ ] Unit tests with known team compositions and expected profiles.

---

## COACH-003 · Evolution Path-Finding (HIGH)

**Location**: New file `internal/legality/pathfind.go`
**Depends on**: Existing `evolution_condition` table

### Problem

The Coach can show one-hop evolution options (what can this Pokémon evolve into next?) but cannot
answer multi-hop questions like:
- "What's the shortest path from Eevee to Espeon given my current items and level?"
- "Can I reach Alakazam without trading?" (when `no_trade_evolutions` rule is active)
- "Should I delay evolving Pikachu to learn Thunderbolt first?"

### Required Actions

1. Implement `EvolutionGraph` — load all `evolution_condition` rows from the DB into an in-memory
   adjacency list at Coach query time (or cache per version group).

```go
type EvolutionGraph struct {
    edges map[int][]EvolutionEdge // from_form_id → []edges
}

type EvolutionEdge struct {
    ToFormID      int
    ToSpeciesName string
    Trigger       string
    Conditions    map[string]interface{}
}
```

2. Implement `FindEvolutionPaths(graph *EvolutionGraph, fromFormID int, state *RunState) []EvolutionPath`
   — BFS/DFS from the given form, returning all reachable evolution targets with annotated feasibility.

```go
type EvolutionPath struct {
    Steps       []EvolutionStep
    FullyLegal  bool   // all steps are currently possible
    BlockReason string // first blocking constraint, if any
}

type EvolutionStep struct {
    FromFormID    int
    ToFormID      int
    ToSpeciesName string
    Trigger       string
    Conditions    map[string]interface{}
    Possible      bool    // given current level, items, rules
    BlockedBy     *string // "no_trade_evolutions", "level_cap", "missing_item:moon-stone"
}
```

3. Constraint evaluation for each step:
   - `trigger = "level-up"` + `min_level` → check against player's Pokémon level and badge level cap
   - `trigger = "use-item"` + `item_id` → check `run_item` for item ownership
   - `trigger = "trade"` → check `no_trade_evolutions` rule flag
   - `trigger = "level-up"` + `friendship` → check `story.high_friendship` run flag
   - `trigger = "level-up"` + `held_item_id` → check `run_item` and pending future discovery

4. Implement `MoveDelayAnalysis(db *sql.DB, formID int, evoPath EvolutionPath, vgID int) []MoveDelayNote`
   — for a given evolution path, identify moves that the pre-evolution learns at a lower level than
   the evolution (suggesting the player should delay evolution).

```go
type MoveDelayNote struct {
    MoveName        string
    PreEvoLevel     int    // level the current form learns it
    PostEvoLevel    int    // level the evolution learns it (0 if never)
    Recommendation  string // "delay", "evolve_now", "neutral"
}
```

### Acceptance Criteria

- [ ] `FindEvolutionPaths` for Eevee (form 133) returns paths to all 3 Gen 3 Eeveelutions.
- [ ] Umbreon path is marked `BlockedBy: "missing: high friendship"` if flag is unset.
- [ ] With `no_trade_evolutions` active, Alakazam path shows `BlockedBy: "no_trade_evolutions"`.
- [ ] `MoveDelayAnalysis` for Pikachu → Raichu flags Thunderbolt (Pikachu Lv26, Raichu never).
- [ ] BFS terminates cleanly on circular or missing edges (no infinite loops).
- [ ] Unit tests for linear chains (Charmander→Charmeleon→Charizard), branching (Eevee), and blocked paths.

---

## COACH-004 · Pokémon Stats & Types in Database (MEDIUM)

**Location**: `migrations/009_pokemon_types_stats.sql`, `internal/pokeapi/pokemon.go`

### Problem

The `pokemon_form` table stores only `id`, `species_id`, and `form_name`. The Coach has no access
to base stats, types, or abilities — it cannot evaluate team composition, compute type coverage
(COACH-002), or assess stat-based matchups.

### Required Actions

1. Add migration `009_pokemon_types_stats.sql`:

```sql
-- Pokémon type assignments (one or two rows per form)
CREATE TABLE IF NOT EXISTS pokemon_type (
    form_id    INTEGER NOT NULL REFERENCES pokemon_form(id),
    slot       INTEGER NOT NULL,  -- 1 = primary, 2 = secondary
    type_name  TEXT NOT NULL,
    PRIMARY KEY (form_id, slot)
);

-- Base stats per form
CREATE TABLE IF NOT EXISTS pokemon_stats (
    form_id    INTEGER PRIMARY KEY REFERENCES pokemon_form(id),
    hp         INTEGER NOT NULL,
    attack     INTEGER NOT NULL,
    defense    INTEGER NOT NULL,
    sp_attack  INTEGER NOT NULL,
    sp_defense INTEGER NOT NULL,
    speed      INTEGER NOT NULL
);

-- Abilities per form
CREATE TABLE IF NOT EXISTS pokemon_ability (
    form_id      INTEGER NOT NULL REFERENCES pokemon_form(id),
    slot         INTEGER NOT NULL,  -- 1,2 = regular; 3 = hidden (not available in Gen 3)
    ability_name TEXT NOT NULL,
    PRIMARY KEY (form_id, slot)
);
```

2. Extend `pokeapi.EnsurePokemon()` to fetch and store types, base stats, and abilities from
   the PokeAPI `/pokemon/{id}` response (already fetched — these fields are in the same payload).

3. Populate new tables during the existing lazy-load flow — no additional HTTP requests needed.

### Acceptance Criteria

- [ ] After seeding Charizard (form 6): `pokemon_type` has `(6,1,"fire")` and `(6,2,"flying")`.
- [ ] After seeding Charizard: `pokemon_stats` has `(6, 78, 84, 78, 109, 85, 100)`.
- [ ] After seeding Charizard: `pokemon_ability` has `(6,1,"blaze")`.
- [ ] Existing PokeAPI cache prevents re-fetching — only processes forms not yet populated.
- [ ] Migration is safe to run against existing databases (empty tables, no data loss).

---

## COACH-005 · Current Moveset Tracking (MEDIUM)

**Location**: `migrations/010_current_moves.sql`, `internal/handlers/team.go`

### Problem

The Coach receives *learnable* moves (what a Pokémon could learn) but not *equipped* moves (what
it currently knows). The `run_pokemon` table tracks party membership and level but not the 1–4
moves each Pokémon actually has. Without this, the Coach cannot:
- Assess offensive coverage of the current team (only hypothetical coverage)
- Suggest which move to replace when learning a new one
- Identify teams with redundant coverage

### Required Actions

1. Add migration `010_current_moves.sql`:

```sql
CREATE TABLE IF NOT EXISTS run_pokemon_move (
    run_pokemon_id INTEGER NOT NULL REFERENCES run_pokemon(id) ON DELETE CASCADE,
    slot           INTEGER NOT NULL,  -- 1-4
    move_id        INTEGER NOT NULL REFERENCES move(id),
    PRIMARY KEY (run_pokemon_id, slot)
);
```

2. Add UI for managing moves on the team page — when a Pokémon learns a new move, the player picks
   which slot to assign it to (or which move to replace).

3. Expose current movesets in the Coach payload under `party[].current_moves`.

4. If no moves are assigned (`run_pokemon_move` empty for a Pokémon), the Coach payload should omit
   `current_moves` for that slot rather than sending an empty array, so the LLM knows data is
   missing vs. the Pokémon truly has no moves.

### Acceptance Criteria

- [ ] A party Pokémon can have 1–4 moves assigned via the team page.
- [ ] Assigning a 5th move prompts a "replace which move?" selection.
- [ ] Coach payload includes `current_moves` with `move_id`, `name`, and `type_name` for each slot.
- [ ] Existing runs with no move data assigned continue to work (empty table, no errors).

---

## COACH-006 · Enriched Coach Payload (HIGH)

**Location**: `internal/handlers/coach.go`, `internal/services/zeroclaw.go`
**Depends on**: COACH-001, COACH-002, COACH-003, COACH-004, COACH-005

### Problem

The current `CoachPayload` sends three flat candidate lists (`acquisitions`, `items`, `party_moves`)
and a question string. The Coach has no structured context about team composition, type analysis,
evolution strategy, or move power/accuracy. The LLM must infer all strategic context from
unstructured candidate lists.

### Required Actions

1. Extend `CoachPayload.Candidates` with new sections:

```go
type CoachPayload struct {
    Candidates CoachCandidates `json:"candidates"`
    Question   string          `json:"question"`
}

type CoachCandidates struct {
    // Existing
    Acquisitions []legality.Acquisition       `json:"acquisitions"`
    Items        []legality.Item              `json:"items"`
    PartyMoves   []PartyMoveSummary           `json:"party_moves"`

    // New: COACH-002
    TeamAnalysis *TeamAnalysisPayload         `json:"team_analysis,omitempty"`

    // New: COACH-003
    EvolutionPaths map[int][]legality.EvolutionPath `json:"evolution_paths,omitempty"` // keyed by form_id

    // New: COACH-004 + COACH-005
    PartyDetails []PartyDetailPayload         `json:"party_details,omitempty"`
}

type TeamAnalysisPayload struct {
    Weaknesses  []legality.TypeThreat    `json:"weaknesses"`
    Resistances []string                 `json:"resistances"`
    Immunities  []string                 `json:"immunities"`
    Uncovered   []string                 `json:"uncovered_types"`
    CounterChains map[string][]legality.CounterHop `json:"counter_chains,omitempty"` // keyed by threat type
}

type PartyDetailPayload struct {
    Slot        int      `json:"slot"`
    SpeciesName string   `json:"species_name"`
    Level       int      `json:"level"`
    Types       []string `json:"types"`
    BaseStats   *legality.BaseStats `json:"base_stats,omitempty"`
    Ability     string   `json:"ability,omitempty"`
    CurrentMoves []MoveDetail `json:"current_moves,omitempty"` // nil = not yet tracked
}

type MoveDetail struct {
    Name     string  `json:"name"`
    TypeName string  `json:"type_name"`
    Power    *int    `json:"power"`    // nil for status moves
    Accuracy *int    `json:"accuracy"` // nil for never-miss moves
    PP       int     `json:"pp"`
}
```

2. In `buildCoachPage()` (or a new `buildCoachPayload()` helper):
   - Load party details with types and stats (COACH-004 tables).
   - Load current movesets (COACH-005 table).
   - Compute team defensive profile and offensive coverage (COACH-002).
   - For each party member, compute evolution paths (COACH-003).
   - For top-2 weaknesses, compute counter chains to depth 2 (COACH-002).
   - Include move `power`, `accuracy`, `pp` from the `move` table (already stored but not sent).

3. Gate new payload sections on data availability — if `pokemon_type` is empty (not yet seeded),
   omit `team_analysis` and `party_details.types` rather than sending empty/zero values.

### Acceptance Criteria

- [ ] Coach payload size stays under 32 KB for a 6-member team (measure with `json.Marshal`).
- [ ] `team_analysis` is populated when at least one party member has type data seeded.
- [ ] `evolution_paths` is populated for each party member with at least one evolution edge.
- [ ] `party_details[].current_moves` is populated only for Pokémon with assigned moves.
- [ ] Existing Coach queries continue to work if no COACH-004/005 data exists (graceful omission).
- [ ] Move `power`, `accuracy`, `pp` are included in `party_moves` entries.

---

## COACH-007 · Cross-Generation Knowledge Strategy (LOW)

**Location**: ZeroClaw agent configuration (external repo), `internal/handlers/coach.go`

### Problem

Players may ask the Coach questions spanning all 9 generations:
- "How do I breed Dragon Dance onto Larvitar?"
- "Is this Pokémon competitively viable in modern OU?"
- "What's this Pokémon's hidden ability in Gen 8?"

These questions require knowledge of 1000+ species across 9 generations. Storing all this in the
local database would be impractical and redundant with the LLM's training data.

### Design Decision: Hybrid Approach

**Run-specific answers** (what can I catch, what's legal, what are my team's weaknesses) are
grounded in the database via COACH-001 through COACH-006. These are deterministic and must be
correct.

**General Pokémon knowledge** (breeding chains, competitive tiers, cross-gen trivia, habitat lore)
is delegated to the LLM's built-in training data. The LLM already has comprehensive Pokémon
knowledge from Bulbapedia, Serebii, Smogon, and similar sources.

### Required Actions

1. Update the ZeroClaw agent system prompt (external repo) to establish a clear boundary:
   - "When answering questions about the player's *current run*, use ONLY the structured data
     provided in the `candidates` payload. Do not hallucinate encounters, moves, or type matchups."
   - "When answering *general Pokémon questions* (breeding, competitive tiers, cross-generation
     comparisons, lore), you may use your training knowledge. Clearly indicate when an answer is
     based on general knowledge vs. the player's run data."

2. Add a `context_note` field to `CoachPayload` that the handler populates with a brief summary
   of what data is grounded vs. what should use LLM knowledge:

```go
type CoachPayload struct {
    Candidates  CoachCandidates `json:"candidates"`
    Question    string          `json:"question"`
    ContextNote string          `json:"context_note"` // e.g. "Run data is Gen 3 (FireRed). Team analysis is computed from verified DB data."
}
```

3. If the question does not reference the current run (no "my team", "my Pokémon", "this run"),
   the handler may skip computing run-specific analysis to reduce latency.

### Acceptance Criteria

- [ ] ZeroClaw system prompt distinguishes grounded vs. general-knowledge answers.
- [ ] `context_note` is populated on every Coach request.
- [ ] Coach correctly answers "How do I breed Dragon Dance onto Larvitar?" using LLM knowledge.
- [ ] Coach correctly answers "What are my team's weaknesses?" using computed type analysis.

---

## Implementation Order

```
Phase 1 — Foundation (no UI changes)
├── COACH-001  Type effectiveness engine         (new file, unit tests)
├── COACH-004  Pokémon types/stats in DB         (migration + pokeapi extension)
│
Phase 2 — Analysis (Coach payload enriched)
├── COACH-002  Team coverage analysis            (depends on COACH-001, COACH-004)
├── COACH-003  Evolution path-finding            (depends on existing evo table)
├── COACH-006  Enriched Coach payload            (depends on COACH-001–004)
│
Phase 3 — Full tracking
├── COACH-005  Current moveset tracking          (migration + team page UI)
├── COACH-006  Update payload with movesets      (depends on COACH-005)
│
Phase 4 — LLM integration
└── COACH-007  Cross-gen knowledge strategy      (ZeroClaw config, prompt tuning)
```

### Phase Dependencies

- Phase 1 items are independent and can be developed in parallel.
- Phase 2 requires Phase 1 tables to be seeded (types/stats available).
- Phase 3 requires UI work on the team page (move slot management).
- Phase 4 is ZeroClaw-side configuration, independent of Go code.

---

## Out of Scope

These items were considered but excluded from this PRD:

| Item | Reason |
|------|--------|
| Graph database (Neo4j, Cayley) | Dataset is too small (~400 species, depth-3 trees). In-memory Go computation is faster and simpler. |
| Full multi-gen DB storage | Hybrid model delegates cross-gen knowledge to LLM. Only active run's gen is DB-grounded. |
| EV/IV optimization | Out of scope for PokemonProfessor (stated in architecture.md). |
| Battle simulation | Out of scope per architecture.md. Type coverage analysis is advisory, not predictive. |
| Opponent team data (gym leaders, rivals) | Useful but requires manual data entry or scraping — separate PRD. |
| Encounter rate probabilities | PokeAPI provides rates but they add complexity without strong coaching value. |

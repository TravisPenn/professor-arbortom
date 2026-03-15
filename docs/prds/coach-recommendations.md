# PokémonProfessor — AI Coach: Actionable Recommendations

**Status**: Not started
**Priority**: High
**Date**: 2026-03-15

Cross-reference: [architecture.md](architecture.md), [schema.md](schema.md),
[coach-enrichment.md](coach-enrichment.md), [coach-ollama-migration.md](coach-ollama-migration.md).

---

## Executive Summary

The AI Coach has strong structured data — type coverage, evolution paths, current movesets, and base
stats — but is missing four inputs that are essential for producing actionable recommendations:

1. **No system prompt by default** — Without a system prompt, the LLM receives raw JSON with no
   persona, no role instructions, and no guidance on response format. This single gap degrades output
   quality more than any missing data field.
2. **No move power/accuracy in learnable moves** — `MoveOption` (the learnable-moves struct) carries
   type and learn method but not power/accuracy/PP, so neither the LLM nor the player can compare
   move strength when deciding what to teach.
3. **No opponent data** — There is no gym leader or Elite Four table. The Coach cannot say "prep for
   Misty with an Electric type" because it has no record of who Misty is, where she appears, or what
   her team looks like.
4. **No route progression awareness** — `badge_count` is tracked but there is no mapping of badge
   number to next opponent. The Coach cannot tell the player how many badges remain before a specific
   boss fight.

This PRD specifies four requirements (COACH-012 through COACH-015) that fill these gaps and enable
the following recommendation categories:

| Category | Example | Requires |
|---|---|---|
| Move + Evolution trade-off | "Evolve Bulbasaur at Lv 16 unless you want Sleep Powder at Lv 15 — Ivysaur learns it one level later." | COACH-012, COACH-013 |
| Catch advice | "Route 3 is the first area where you can catch Nidoran♂ for coverage against Lt. Surge." | COACH-012, COACH-015 |
| Item reminders | "After seeing 10 Pokémon species, talk to Oak's aide on Route 2 to receive the Exp. Share." | COACH-012 |
| Team theme observation | "Your team leans heavily on Water. Voltorb is available at the Power Plant and would cover your Electric gap before Sabrina." | COACH-012, COACH-015 |

### Architecture Decision

| Gap | Solution | Rationale |
|---|---|---|
| Missing system prompt | Hardcode a default `Professor Arbortom` persona; env var overrides | Biggest quality-of-life improvement for zero schema cost |
| No move power in learnable moves | JOIN `move` table in existing `CoachMoves()` query | Data already seeded; no migration needed |
| No opponent data | New migration — `gym_leader` + `gym_leader_pokemon` tables with static seed data | PokeAPI has no trainer data; static seed matches existing pattern from `011_static_encounters.sql` |
| No route progression | Query `gym_leader` WHERE `badge_order > badge_count` | Direct badge-gated lookup; no graph traversal needed |

Scope: **Gym Leaders + Elite Four / Champion only** — rivals are excluded (team composition varies
by starter choice and encounter number; a dedicated `rival_encounter` table is deferred).

---

## COACH-012 · Default System Prompt with Professor Persona (HIGH)

**Location**: `internal/services/coach.go`, `cmd/professor-arbortom/main.go`

### Problem

`COACH_SYSTEM_PROMPT` defaults to empty. When empty, `QueryCoach` omits the system message
entirely. The LLM receives structured JSON game data and a bare user question with no persona, no
role instructions, and no awareness of Nuzlocke rules. It produces generic, unfocused answers that
don't reflect the recommendation categories the player needs.

This is the highest-leverage fix in this PRD — it costs no DB changes and immediately improves
every Coach response.

### Required Actions

1. Add a package-level constant in `internal/services/coach.go`:

```go
// defaultSystemPrompt is used when COACH_SYSTEM_PROMPT is not set.
// It establishes the Professor Arbortom persona with Nuzlocke-aware coaching
// instructions and the four recommendation categories the player expects.
// COACH_SYSTEM_PROMPT env var fully replaces this when set.
const defaultSystemPrompt = `You are Professor Arbortom, a Nuzlocke coach for
Generation 3 Pokémon games (FireRed, LeafGreen, Ruby, Sapphire, Emerald).

Nuzlocke rules always in effect:
- Only the first Pokémon encountered in each new area may be caught.
- Any Pokémon that faints is permanently lost — treat it as unavailable.
- Additional rules may be active; they appear in the game data if set.

You will receive structured game data (party members, learnable moves, available
items, encounter options, upcoming opponents). Use it as ground truth. Fill gaps
from your general Pokémon knowledge, but note when you do so.

When answering, cover one or more of these categories where relevant:

  MOVE + EVOLUTION: Compare what the current form and its evolution(s) learn,
  and at what levels. Advise whether to evolve now or wait to learn a move first.
  Example: "Evolve Bulbasaur at Lv 16 for Ivysaur's better stats, unless you
  want Sleep Powder — Bulbasaur learns it at Lv 15, Ivysaur at Lv 16."

  CATCHES: Identify the first (or best) area to find a Pokémon that fills a
  type coverage gap relevant to the next gym. Mention the encounter level range.
  Example: "Route 3 is the first area for Nidoran♂ (Lv 3-5), useful against
  Lt. Surge's Electric team."

  ITEMS: Reference available shop items, NPC gifts, and held-item strategy.
  Note prerequisite conditions for NPC gifts.
  Example: "After registering 10 Pokémon species, Oak's aide on Route 2
  gives you the Exp. Share."

  TEAM THEME: Notice dominant types and suggest a Pokémon that would improve
  coverage or counter the next opponent.
  Example: "Three Water-types leave you weak to Electric. Magnemite is
  available just before Lt. Surge and resists his whole team."

Be concise. Use 3-5 sentences for simple questions; a short bullet list for
multi-part comparisons. Never suggest a fainted Pokémon.`
```

2. Update `NewCoachClient` to fall back to `defaultSystemPrompt` when the argument is empty:

```go
func NewCoachClient(host, model, systemPrompt string) *CoachClient {
    if systemPrompt == "" {
        systemPrompt = defaultSystemPrompt
    }
    return &CoachClient{
        host:         host,
        model:        model,
        systemPrompt: systemPrompt,
        http:         &http.Client{Timeout: 120 * time.Second},
    }
}
```

`COACH_SYSTEM_PROMPT` in `main.go` is unchanged — when set, it is passed to `NewCoachClient` and
fully replaces the default. No change to `QueryCoach`; the existing path already includes
`systemPrompt` in the messages array when non-empty.

### Acceptance Criteria

- [ ] `NewCoachClient("http://host", "qwen2.5:3b", "")` sets `systemPrompt` to `defaultSystemPrompt`.
- [ ] `NewCoachClient("http://host", "qwen2.5:3b", "custom prompt")` sets `systemPrompt` to `"custom prompt"`.
- [ ] `COACH_SYSTEM_PROMPT=""` in env → `NewCoachClient` receives `""` → default prompt is used.
- [ ] `COACH_SYSTEM_PROMPT="x"` in env → `NewCoachClient` receives `"x"` → default is not used.
- [ ] The Ollama request body includes a `system` role message in both cases.
- [ ] Unit test: `NewCoachClient("", "", "")` → `c.systemPrompt == defaultSystemPrompt`.
- [ ] `go test ./internal/services/...` passes.

---

## COACH-013 · Learnable Move Stats — Power, Accuracy, PP (MEDIUM)

**Location**: `internal/legality/types.go`, `internal/legality/moves.go`,
`internal/handlers/pages.go`, `internal/handlers/converters.go`

### Problem

`MoveOption` (the struct for learnable moves in the Coach Party Moves table) carries `Name`,
`TypeName`, `LearnMethod`, `Level`, and `EvoNote` but **not** `Power`, `Accuracy`, or `PP`. The
`move` table already contains all three columns, seeded from PokeAPI. The LLM payload's
`party_moves` field therefore cannot let the model compare, for example, whether Mega Punch
(Power 80, Accuracy 85) is worth teaching over Headbutt (Power 70, Accuracy 100).

`MoveDetail` (current assigned moves, COACH-005/006) already has `Power *int`, `Accuracy *int`,
`PP int`. Learnable moves should match.

No migration is required — the data is already in the `move` table.

### Required Actions

1. **`internal/legality/types.go` — `Move` struct**: Add three fields.

```go
type Move struct {
    MoveID        int
    Name          string
    TypeName      string
    LearnMethod   string
    LevelLearned  int
    TMNumber      int
    HMNumber      int
    TutorLocation string
    BlockedByRule string
    EvoNote       string
    // COACH-013: move stats for comparison in Coach payload and Party Moves table.
    Power    *int // nil for status moves (no base power)
    Accuracy *int // nil for never-miss moves (e.g. Swift, Aerial Ace)
    PP       int
}
```

2. **`internal/legality/moves.go`** — Update all SELECT queries that build `Move` rows to JOIN
   the `move` table and scan `power`, `accuracy`, `pp`. The primary paths are `CoachMoves()`,
   `LegalMoves()`, and `LegalTMMoves()`.

   In each query, replace the existing column list with:

   ```sql
   SELECT le.move_id, m.name, m.type_name, le.learn_method, le.level_learned,
          m.power, m.accuracy, m.pp
   FROM learnset_entry le
   JOIN move m ON m.id = le.move_id
   WHERE ...
   ```

   Scan `power` and `accuracy` as `sql.NullInt64`, converting to `*int` (nil when not valid).
   `pp` is always a non-null integer; scan directly.

3. **`internal/handlers/pages.go` — `MoveOption`**: Add the three fields.

```go
type MoveOption struct {
    ID            int
    Name          string
    TypeName      string
    LearnMethod   string
    Level         int
    EvoNote       string
    TMNumber      int
    HMNumber      int
    TutorLocation string
    Power    *int // nil for status moves
    Accuracy *int // nil for never-miss moves
    PP       int
}
```

4. **`internal/handlers/converters.go` — `moveToOption()`**: Copy the three new fields from
   `legality.Move` to `MoveOption`.

5. **`templates/coach.html` — Party Moves table (optional UI enhancement)**: Add Power and
   Accuracy columns to the learnable moves table. Display `—` when nil. This is a display
   improvement; the Coach payload benefits from the data regardless of template changes.

### Acceptance Criteria

- [ ] `CoachMoves()` for a Pokémon with level-up moves returns `Move.Power != nil` for all damaging moves.
- [ ] `CoachMoves()` returns `Move.Power == nil` for status moves (e.g. Growl, Swords Dance).
- [ ] `MoveOption.Power` is populated in `buildCoachPage` via `moveToOption()`.
- [ ] The `party_moves` block in the Coach JSON payload includes `"power"` for damaging moves.
- [ ] `go test ./internal/legality/...` passes — no existing move tests broken.
- [ ] No new migration required.

---

## COACH-014 · Gym Leader & Elite Four Schema and Seed Data (HIGH)

**Location**: New migration `016_opponent_teams.sql`

### Problem

There is no opponent data in the database. `badge_count` is tracked in `run_progress` but there is
no mapping from badge number to gym leader name, type specialty, location, or party. The Coach
cannot answer "what's the next gym?" or "should I have Ground coverage soon?"

### Schema

```sql
-- 016_opponent_teams.sql
PRAGMA user_version = 16;

-- gym_leader covers 8 gym leaders + 5 Elite Four/Champion entries per version.
-- badge_order: 1–8 for gym leaders (matching badge_count after defeating them);
--              9 = first E4 member, 10 = second, 11 = third, 12 = fourth, 13 = Champion.
CREATE TABLE IF NOT EXISTS gym_leader (
    id             INTEGER PRIMARY KEY,
    version_id     INTEGER NOT NULL REFERENCES game_version(id),
    badge_order    INTEGER NOT NULL,  -- 1–13
    name           TEXT    NOT NULL,
    type_specialty TEXT    NOT NULL,  -- primary type label (display only)
    location_name  TEXT    NOT NULL,  -- city or dungeon display name
    UNIQUE(version_id, badge_order)
);

-- gym_leader_pokemon is the opponent's party in slot order (1–6).
-- Moves are nullable text (display names); not all historical sources document
-- every trainer's exact moveset, but gym leaders are well-documented.
CREATE TABLE IF NOT EXISTS gym_leader_pokemon (
    id            INTEGER PRIMARY KEY,
    gym_leader_id INTEGER NOT NULL REFERENCES gym_leader(id),
    slot          INTEGER NOT NULL,  -- 1–6
    form_id       INTEGER NOT NULL REFERENCES pokemon_form(id),
    level         INTEGER NOT NULL,
    held_item     TEXT,              -- display name, nullable
    move_1        TEXT,
    move_2        TEXT,
    move_3        TEXT,
    move_4        TEXT
);
```

### Seed Data Scope

Seed all **5 Gen 3 versions** — FireRed (version_id=10), LeafGreen (version_id=11), Ruby
(version_id=6), Sapphire (version_id=7), Emerald (version_id=8).

FireRed and LeafGreen share identical gym leader teams; both are seeded separately by `version_id`.
Ruby and Sapphire share identical gym leader teams; both seeded separately.
Emerald gym leaders use their Emerald-specific (upgraded) teams — seeded independently.

| Version(s) | Gym Leaders | E4 + Champion | Approx. Pokémon rows |
|---|---|---|---|
| FireRed | 8 | 5 | ~44 |
| LeafGreen | 8 | 5 | ~44 |
| Ruby | 8 | 5 | ~44 |
| Sapphire | 8 | 5 | ~44 |
| Emerald | 8 | 5 | ~50 |
| **Total** | **40** | **25** | **~226** |

**Kanto gym leaders (FireRed/LeafGreen)**:

| badge_order | Name | Type | Location | Team (level) |
|---|---|---|---|---|
| 1 | Brock | Rock | Pewter City | Geodude L12, Onix L14 |
| 2 | Misty | Water | Cerulean City | Staryu L18, Starmie L21 |
| 3 | Lt. Surge | Electric | Vermilion City | Voltorb L21, Pikachu L24, Raichu L24 |
| 4 | Erika | Grass | Celadon City | Victreebel L29, Tangela L24, Vileplume L29 |
| 5 | Koga | Poison | Fuchsia City | Koffing L37, Muk L39, Koffing L37, Weezing L43 |
| 6 | Sabrina | Psychic | Saffron City | Kadabra L38, Mr. Mime L37, Venomoth L38, Alakazam L43 |
| 7 | Blaine | Fire | Cinnabar Island | Growlithe L42, Ponyta L40, Rapidash L42, Arcanine L47 |
| 8 | Giovanni | Ground | Viridian City | Rhyhorn L45, Dugtrio L42, Nidoqueen L44, Nidoking L45, Rhydon L50 |
| 9 | Lorelei | Ice | Indigo Plateau | Dewgong L52, Cloyster L51, Slowbro L52, Jynx L54, Lapras L54 |
| 10 | Bruno | Fighting | Indigo Plateau | Onix L53, Hitmonchan L55, Hitmonlee L55, Onix L54, Machamp L58 |
| 11 | Agatha | Ghost | Indigo Plateau | Gengar L54, Haunter L53, Gengar L58, Arbok L54, Gengar L58 |
| 12 | Lance | Dragon | Indigo Plateau | Gyarados L58, Dragonair L56, Dragonair L56, Aerodactyl L60, Dragonite L62 |
| 13 | Blue | Mixed | Indigo Plateau | Pidgeot L59, Alakazam L59, Rhydon L61, Arcanine L61, Exeggutor L61, *(starter-evo L65)* |

> **Blue's sixth slot** — Blue's ace varies by the player's starter choice: Charizard (if player chose Squirtle), Blastoise (if Bulbasaur), or Venusaur (if Charmander). The `gym_leader_pokemon` schema has no discriminator for this. **Resolution**: seed three rows for Blue's slot 6 using a new nullable `starter_counter_form_id` column, or simplify by seeding only Charizard as a canonical placeholder with a comment. The migration SQL must note this approximation. A `starter_variant` column (`TEXT`, nullable, values `'fire'`/`'water'`/`'grass'`) should be added to `gym_leader_pokemon` to model all three variants correctly — `nextOpponents()` would then filter by the run's starter type when known.

**Hoenn gym leaders (Ruby/Sapphire)**:

| badge_order | Name | Type | Location | Ace (level) |
|---|---|---|---|---|
| 1 | Roxanne | Rock | Rustboro City | Nosepass L15 |
| 2 | Brawly | Fighting | Dewford Town | Hariyama L17 |
| 3 | Wattson | Electric | Mauville City | Manectric L22 |
| 4 | Flannery | Fire | Lavaridge Town | Torkoal L26 |
| 5 | Norman | Normal | Petalburg City | Slaking L31 |
| 6 | Winona | Flying | Fortree City | Altaria L33 |
| 7 | Tate & Liza | Psychic | Mossdeep City | Solrock L42, Lunatone L42 |
| 8 | Wallace | Water | Sootopolis City | Milotic L46 |
| 9–13 | Sidney / Phoebe / Glacia / Drake / Steven | Mixed | Ever Grande | — |

*Emerald replaces Wallace with Juan at badge_order=8, and changes the Champion to Wallace.*

**`form_id` reference**: PokeAPI assigns canonical form IDs matching species number for single-form
Pokémon (e.g. Geodude = 74, Onix = 95, Staryu = 120, Starmie = 121). The migration SQL should
include an inline comment mapping each `form_id` to species name for maintainability.

### Acceptance Criteria

- [ ] `SELECT COUNT(*) FROM gym_leader` returns 65 (13 leaders × 5 versions).
- [ ] `SELECT COUNT(*) FROM gym_leader WHERE badge_order BETWEEN 1 AND 8` returns 40.
- [ ] `SELECT name FROM gym_leader WHERE version_id=10 AND badge_order=1` returns `'Brock'`.
- [ ] `SELECT name FROM gym_leader WHERE version_id=8 AND badge_order=1` returns `'Roxanne'` (Emerald).
- [ ] `SELECT name FROM gym_leader WHERE version_id=8 AND badge_order=13` returns `'Wallace'` (Emerald Champion).
- [ ] All `form_id` values in `gym_leader_pokemon` reference existing rows in `pokemon_form`.
- [ ] `PRAGMA user_version` returns `16` after migration.
- [ ] Migration applies cleanly on a fresh DB (idempotent `CREATE TABLE IF NOT EXISTS`).

---

## COACH-015 · Next Opponents Integration — Payload, Page, and Template Section (HIGH)

**Location**: `internal/handlers/coach.go`, `internal/handlers/pages.go`,
`internal/services/coach.go`, `templates/coach.html`

**Depends on**: COACH-014

### Problem

Even after COACH-014 seeds gym leader data, nothing queries it. The coach page and AI payload have
no opponent information. This requirement wires the data into the existing Coach pipeline.

### Required Actions

#### 1. Types in `internal/handlers/pages.go`

```go
// OpponentSummary is one gym leader or Elite Four member with their party.
// Used in CoachPage for the "Next Battle" UI section and the Coach AI payload.
type OpponentSummary struct {
    Name          string            `json:"name"`
    TypeSpecialty string            `json:"type_specialty"`
    LocationName  string            `json:"location_name"`
    BadgeOrder    int               `json:"badge_order"`
    Team          []OpponentPokemon `json:"team"`
}

// OpponentPokemon is one party member in an opponent's team.
type OpponentPokemon struct {
    SpeciesName string   `json:"species_name"`
    Level       int      `json:"level"`
    Types       []string `json:"types,omitempty"` // from pokemon_type if seeded
    HeldItem    string   `json:"held_item,omitempty"`
    Moves       []string `json:"moves,omitempty"` // up to 4 move names
}
```

Add `NextOpponents []OpponentSummary` to `CoachPage`.

#### 2. Query helper in `internal/handlers/coach.go`

```go
// nextOpponents returns the next 2 gym leaders / E4 members after the run's
// current badge count. Returns nil, nil if the gym_leader table is not present
// (pre-migration or pre-seed state) so callers degrade gracefully.
func nextOpponents(db *sql.DB, runID int) ([]OpponentSummary, error)
```

Implementation steps:
1. Look up `version_id` and `badge_count` for the run via:
   ```sql
   SELECT r.version_id, COALESCE(rp.badge_count, 0)
   FROM run r LEFT JOIN run_progress rp ON rp.run_id = r.id
   WHERE r.id = ?
   ```
2. Call `tableExists(db, "gym_leader")` — return `nil, nil` if false.
3. Query the next 2 opponents:
   ```sql
   SELECT id, name, type_specialty, location_name, badge_order
   FROM gym_leader
   WHERE version_id = ? AND badge_order > ?
   ORDER BY badge_order
   LIMIT 2
   ```
4. For each leader, fetch the team:
   ```sql
   SELECT ps.name, glp.level, glp.held_item,
          glp.move_1, glp.move_2, glp.move_3, glp.move_4
   FROM gym_leader_pokemon glp
   JOIN pokemon_form pf ON pf.id = glp.form_id
   JOIN pokemon_species ps ON ps.id = pf.species_id
   WHERE glp.gym_leader_id = ?
   ORDER BY glp.slot
   ```
5. Optionally join `pokemon_type` to populate `OpponentPokemon.Types` (LEFT JOIN; omit if empty).
6. Return populated `[]OpponentSummary`.

#### 3. `buildCoachPage`

Call `nextOpponents(db, runID)` and assign to `page.NextOpponents`. Log (non-fatal) if it errors.

#### 4. `CoachCandidates` in `internal/services/coach.go`

```go
type CoachCandidates struct {
    Acquisitions   interface{} `json:"acquisitions"`
    Items          interface{} `json:"items"`
    PartyMoves     interface{} `json:"party_moves"`
    TeamAnalysis   interface{} `json:"team_analysis,omitempty"`
    EvolutionPaths interface{} `json:"evolution_paths,omitempty"`
    PartyDetails   interface{} `json:"party_details,omitempty"`
    NextOpponents  interface{} `json:"next_opponents,omitempty"` // COACH-015
}
```

#### 5. `buildCoachPayload`

Include `page.NextOpponents` in `Candidates.NextOpponents` (nil when empty — omitted from JSON).

#### 6. `templates/coach.html` — "Next Battle" section

Render before Team Insights. Hidden entirely when `NextOpponents` is empty.

```html
{{if .NextOpponents}}
<section class="coach-section next-battle">
  <h2>Next Battle</h2>
  {{range .NextOpponents}}
  <div class="opponent-card">
    <h3>{{.Name}} <span class="type-badge type-{{.TypeSpecialty | lower}}">{{.TypeSpecialty}}</span></h3>
    <p class="opponent-location">📍 {{.LocationName}}</p>
    <table class="opponent-team">
      <thead>
        <tr><th>Pokémon</th><th>Lv</th><th>Moves</th></tr>
      </thead>
      <tbody>
        {{range .Team}}
        <tr>
          <td>{{.SpeciesName | title}}</td>
          <td>{{.Level}}</td>
          <td class="opponent-moves">{{join .Moves " · "}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
  {{end}}
</section>
{{end}}
```

Add `.opponent-card`, `.opponent-location`, `.opponent-team`, `.next-battle` CSS rules to
`static/style.css` consistent with existing `.coach-section` styling.

> **Template FuncMap requirement** — Go's `html/template` does not include `lower`, `title`, or
> `join` built-ins. The `FuncMap` in `cmd/professor-arbortom/main.go` currently registers `add`,
> `deref`, `mkrange`, `toJSON`, and `evoMethod`. Three new entries must be added before the
> template parses:
> ```go
> "lower": strings.ToLower,
> "title": strings.Title, // or cases.Title(language.Und) for Unicode correctness
> "join":  strings.Join,
> ```
> The template will fail to parse at startup (not at render time) if these are missing, so the
> acceptance criteria must include a clean `go build ./...` and a `/runs/:id/coach` page load.

### Acceptance Criteria

- [ ] At `badge_count=0`, `nextOpponents()` returns the first gym leader with their full team.
- [ ] At `badge_count=8`, `nextOpponents()` returns the first Elite Four member.
- [ ] At `badge_count=13` (post-Champion), `nextOpponents()` returns empty slice.
- [ ] When `gym_leader` table does not exist, `nextOpponents()` returns `nil, nil`.
- [ ] Coach payload JSON includes `"next_opponents": [...]` when data is available.
- [ ] `"next_opponents"` is absent from payload JSON when the slice is empty (omitempty).
- [ ] Coach page renders "Next Battle" section when `NextOpponents` is non-empty.
- [ ] Section is absent when `NextOpponents` is empty.
- [ ] The Coach AI can reference the next opponent by name in a response about team prep.
- [ ] `go test ./internal/handlers/...` passes.

---

## Implementation Order

```
COACH-012  (no schema changes — do first, ships improvement immediately)
COACH-013  (no schema changes — parallel with 012)
COACH-014  (migration — required before 015)
COACH-015  (wires 014 into Coach pipeline)
```

COACH-012 and COACH-013 are independent of each other and of COACH-014/015. They can ship
in a single commit before the gym leader data work begins.

---

## Further Considerations

### Rival Teams
Rivals in Gen 3 (Gary/Blue in FRLG, Brendan/May in RSE) change party composition based on
starter choice and have multiple encounter points. A separate `rival_encounter` table keyed on
`(version_id, starter_form_id, encounter_number)` would be needed. Defer to a follow-up PRD.

### Prose-Formatted Payload (COACH-016 candidate)
`qwen2.5:3b` at `keep_alive: 0` is a small, cold-started model. The current `formatPayload()`
serialises candidates as a raw JSON blob (e.g. `{"acquisitions":[...],"party_moves":[...]}`),
which small models handle less reliably than structured prose. A COACH-016 task would rewrite
`formatPayload()` to emit a compact human-readable brief:
```
## Party
- Bulbasaur Lv 14 (Grass/Poison) | HP 45 Atk 49 | Ability: Overgrow
  Current moves: Tackle (Normal, 40pw), Vine Whip (Grass, 45pw), Growl (status)
  Learnable: Sleep Powder (Grass, status, Lv 15), Razor Leaf (Grass, 55pw, Lv 20)

## Next opponent
- Misty (Water) — Cerulean City
  Staryu Lv 18 | Starmie Lv 21 (Water/Psychic)
```
This requires no schema changes — only changes to `formatPayload()` in `internal/services/coach.go`.
Worth specifying after COACH-015 ships and real LLM output quality can be evaluated.

### Move-Learning Delay Advisor (COACH-017 candidate)
A Go function that pre-computes "delay evolving X because the evolution learns move Y N levels
later" would produce more reliable advice than asking the LLM to infer this from raw learnset JSON.
Requires comparing `learnset_entry` rows for `from_form_id` vs. `to_form_id` on each evolution
edge in `evolution_condition`. Worth specifying as COACH-017 after COACH-015 and COACH-016 ship.

### Form IDs in Seed Data
`form_id` values in `gym_leader_pokemon` must match `pokemon_form.id` values inserted by PokeAPI
seeding. For Gen 1 single-form Pokémon, the canonical PokeAPI form ID equals the species number
(Geodude = 74, Onix = 95, Staryu = 120). For Hoenn Pokémon, IDs fall in the 270–389 range. The
migration SQL must include inline species-name comments on each row.

### Trainer Move Data Sourcing
PokeAPI does not provide trainer team data. The `move_1`–`move_4` columns must be sourced from
Bulbapedia or equivalent references. Canonical move lists for Gen 3 gym leaders are well-documented
and should be included in full for all 226 seed rows. Do not leave move columns NULL for major
opponents.

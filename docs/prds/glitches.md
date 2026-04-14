# PokemonProfessor — Glitch Recommendations PRD

**Status**: Planned
**Priority**: Low
**Date**: 2026-04-13

Cross-reference: [architecture.md](architecture.md) for module structure,
[api.md](api.md) for route inventory, [schema.md](schema.md) for data model.

---

## Executive Summary

Players who choose to exploit in-game glitches (money duplication, item farming, sequence breaks)
currently receive no coach guidance for them. This feature adds an opt-in **Glitch
Recommendations** run rule that surfaces location-aware and progression-aware glitch tips through
the existing coach delivery pipeline — both as LLM context (WALKTHROUGH section) and as numbered
VERIFIED RECOMMENDATIONS when the player's current location matches a known glitch site.

Glitch data is embedded in the existing walkthrough markdown files as a `### Glitches` subsection,
meaning no new package, no schema migration, and no changes to the `walkthroughs` package API are
required. The `SubSection` and `FilterTableByLocation` helpers already support this subsection
format without modification.

---

## Goals

- Players can opt in to glitch tips per-run via the Rules page toggle
- Tips are scoped to the player's current badge phase and fuzzy-matched to their current location
- Content authors can add glitches for any game/badge phase by editing a single markdown file
- No glitch content reaches opt-out runs (strict gate on `glitch_recommendations` rule key)
- Feature composes with existing rules (e.g. Nuzlocke + Glitch Recommendations can coexist)

---

## Non-Goals

- No server-side verification that a glitch is "real" or still works in a given revision/patch
- No glitch categories, difficulty ratings, or risk warnings in V1
- No glitch-specific UI beyond what the existing coach panel already shows
- No ROM hacks, emulator-specific glitches, or anything requiring third-party tools
- No Pokémon GO, Stadium, or side-game glitches

---

## Data Model

Glitch data lives entirely inside the embedded walkthrough markdown files. No new tables, no new
migrations, no changes to `run_setting` schema (beyond the new rule key described in GLITCH-002).

### Walkthrough Subsection Format

A `### Glitches` subsection is added to the relevant `## Badge N` section inside each game's
walkthrough file. The table has four columns:

```markdown
### Glitches
| Location | Name | How To | Effect |
|---|---|---|---|
| Route 24 (Nugget Bridge) | Nugget Bridge Glitch | After defeating all 5 bridge trainers and the Rocket grunt, leave the area and re-enter; talk to the grunt again | Receive another Nugget ($5,000) per visit — repeat for fast money |
```

**Column rules:**
- **Location** — must use the same human-readable location naming style as the `### Items & TMs`
  table in the same file; `NormalizeLocation` slugifies it for fuzzy matching, so
  `Route 24 (Nugget Bridge)` and `cerulean-city` both work.
- **Name** — display name of the glitch (used in the VERIFIED RECOMMENDATION string).
- **How To** — concise step-by-step trigger description passed to the LLM.
- **Effect** — plaintext description of what the glitch does; used directly in the numbered rec.

Multiple glitch rows may appear in a single badge section. The parser handles them the same way
as any other markdown table row — each row becomes one independent recommendation candidate.

---

## GLITCH-001 · Walkthrough Data — Initial Content

**Files**: `data/walkthroughs/firered.md`, `data/walkthroughs/leafgreen.md`

### Problem

No glitch content exists in the walkthrough files. The `### Glitches` subsection must be seeded
before the coach can surface any tips.

### Required Actions

Add a `### Glitches` subsection inside `## Badge 1: Brock (Boulder Badge)` in both
`firered.md` and `leafgreen.md`:

```markdown
### Glitches
| Location | Name | How To | Effect |
|---|---|---|---|
| Route 24 (Nugget Bridge) | Nugget Bridge Glitch | After defeating all 5 bridge trainers and the Rocket grunt, leave the area and re-enter; talk to the grunt again | Receive another Nugget ($5,000) per visit — repeat as often as needed for fast money before the Cerulean Gym |
```

**Placement**: immediately after `### Side Quests` within the Badge 1 block (before the `---`
horizontal rule that opens the next badge section).

**LeafGreen**: identical content — the glitch works the same way in both versions.

### Future Content Expansion

New glitches for any game or badge phase follow the same pattern: add a `### Glitches` subsection
(or append a row to an existing one) in the relevant `## Badge N` section of the target game's
`.md` file. No code changes are required.

Known candidates for future addition:

| Game | Badge Phase | Glitch | Effect |
|---|---|---|---|
| Emerald | Badge 0 | Pokémon Box Clone | Clone Pokémon and held items using Pokémon Box link |
| FireRed / LeafGreen | Badge 3 | Missingno (not applicable — FR/LG patched) | N/A |
| Ruby / Sapphire | Badge 2 | Glitch City (not applicable — Gen 3 fixed) | N/A |
| Emerald | Any | Battle Tower clone glitch | Duplicate held items via link cable action |

> Authors: verify a glitch exists and is reproducible on cartridge before adding it. Emulator-only
> or patched glitches should not be added.

---

## GLITCH-002 · Rule Toggle

**File**: `internal/handlers/rules.go`

### Problem

There is no opt-in mechanism for glitch content. Without a rule gate, glitch tips would appear for
all players regardless of intent.

### Required Actions

Add one entry to `ruleCatalog` (no params, boolean flag only):

```go
{"glitch_recommendations", "Glitch Recommendations", "Surface known in-game glitches in coach tips based on your current location and badge progress"},
```

**Storage**: uses the existing `run_setting` table with `type = 'rule'` and
`key = 'glitch_recommendations'`, `value = '{}'`. No migration needed.

**UI**: `rules.html` iterates `.Rules` dynamically — no template changes needed. The toggle
renders as a standard checkbox alongside Nuzlocke, Level Cap, etc.

**Interaction with other rules**: no conflicts. `glitch_recommendations` is independent of all
existing rules. It does not suppress or require any other rule.

---

## GLITCH-003 · Coach Integration — WALKTHROUGH Context

**File**: `internal/handlers/coach.go`, function `buildWalkthroughContext`

### Problem

The LLM receives a WALKTHROUGH section that includes gym info, items, unlocks, gates, and side
quests. It has no glitch data to draw on even when the player has opted in.

### Required Actions

After the existing `### Side Quests` block (step 5), add step 6 inside `buildWalkthroughContext`:

```go
// 6. Glitches — surfaced as walkthrough context when the rule is active.
//    Filtered to the player's current location when possible.
if _, glitchOn := activeRules["glitch_recommendations"]; glitchOn {
    if gl := walkthroughs.SubSection(section, "Glitches"); gl != "" {
        filtered := walkthroughs.FilterTableByLocation(gl, currentLocation)
        sb.WriteString("\n")
        sb.WriteString(filtered)
        sb.WriteByte('\n')
    }
}
```

**Behavior**: `FilterTableByLocation` returns the full table when no rows match the current
location hint — consistent with how Items & TMs behaves. This means the LLM may receive all
glitches for the badge phase even when the player is not at a glitch site, giving it broader
context for answering free-form questions like "are there any tricks I can use right now?".

---

## GLITCH-004 · Coach Integration — VERIFIED RECOMMENDATIONS

**File**: `internal/handlers/coach.go`, function `buildPreComputedRecommendations`

### Problem

The WALKTHROUGH context passes glitch data to the LLM but does not guarantee the player sees a
numbered, bolded recommendation in the coach panel. A player at Nugget Bridge with badge 1 should
see a concrete tip without having to prompt the coach.

### Required Actions

After the existing item-pickup loop (currently item 5 in the `recs` slice), add a glitch-scanning
pass that checks both `badgeCount` and `badgeCount+1` badge sections — matching the pattern used
by the walkthrough item-pickup loop:

```go
// 6. Glitches at the player's current location — a numbered rec when the
//    glitch_recommendations rule is active and location matches.
_, glitchOn := activeRules["glitch_recommendations"]
if glitchOn && currentLocation != "" {
    hint := walkthroughs.NormalizeLocation(currentLocation)
    for _, bc := range []int{badgeCount, badgeCount + 1} {
        section := walkthroughs.Lookup(versionName, bc)
        if section == "" {
            continue
        }
        glitchSection := walkthroughs.SubSection(section, "Glitches")
        if glitchSection == "" {
            continue
        }
        headerSkipped := 0
        for _, line := range strings.Split(glitchSection, "\n") {
            trimmed := strings.TrimSpace(line)
            if !strings.Contains(trimmed, "|") {
                continue
            }
            if headerSkipped < 2 {
                headerSkipped++
                continue
            }
            cols := strings.Split(trimmed, "|")
            if len(cols) < 5 {
                continue
            }
            loc := strings.TrimSpace(cols[1])
            name := strings.TrimSpace(cols[2])
            effect := strings.TrimSpace(cols[4])
            if loc == "" || name == "" || effect == "" {
                continue
            }
            normLoc := walkthroughs.NormalizeLocation(loc)
            if !strings.Contains(normLoc, hint) && !strings.Contains(hint, normLoc) {
                continue
            }
            recs = append(recs, fmt.Sprintf("Glitch available: %s at %s — %s", name, loc, effect))
        }
    }
}
```

**Output format**: `"Glitch available: Nugget Bridge Glitch at Route 24 (Nugget Bridge) — Receive another Nugget ($5,000) per visit — repeat as often as needed for fast money before the Cerulean Gym."`

**Location guard**: only emits a numbered rec when `NormalizeLocation(row location)` fuzzy-matches
the player's current location. Unlike GLITCH-003, this path does **not** fall back to showing all
glitches when no match — a numbered rec without a location match would be misleading.

---

## Testing

### Manual Verification Checklist

1. **Toggle persistence**: enable Glitch Recommendations on a FireRed run → save →
   confirm `run_setting` row exists with `key = 'glitch_recommendations'`, `value = '{}'`

2. **Location match — both delivery paths**: set current location to Route 24 (or Cerulean City),
   badge count = 1, toggle on → open Coach →
   - WALKTHROUGH section contains the `### Glitches` table
   - VERIFIED RECOMMENDATIONS contains a numbered `"Glitch available: Nugget Bridge Glitch…"` rec

3. **Toggle off**: disable toggle → re-open Coach → confirm neither delivery path emits glitch
   content

4. **Location mismatch**: set location to Pallet Town, badge = 1, toggle on →
   - WALKTHROUGH: may show full glitches table (FilterTableByLocation fallback — expected)
   - VERIFIED RECOMMENDATIONS: no glitch numbered rec (location guard active — expected)

5. **Wrong game**: Ruby/Sapphire/Emerald run with toggle on → no glitch output (no `### Glitches`
   subsection exists in those files)

6. **Badge boundary**: badge count = 0 (pre-Brock) → no glitch rec (Nugget Bridge not accessible
   until after Brock — data is in Badge 1 section only)

7. **Rule coexistence**: Nuzlocke + Glitch Recommendations both active → both sets of coach output
   appear without interference

### Automated Tests

No new test files are required for V1. Existing `walkthroughs_test.go` covers `SubSection` and
`FilterTableByLocation`. If glitch-specific parsing logic is extracted into a helper in the future,
add unit tests at that point.

---

## Rollout

No feature flag beyond the per-run toggle. No migration rollout steps. Deploy as a normal binary
update — existing runs are unaffected until a player explicitly enables the toggle.

---

## Open Questions

| # | Question | Status |
|---|----------|--------|
| 1 | Should glitch descriptions link to an external source (e.g. Bulbapedia)? V1 omits links since the coach panel renders plain text. | Open |
| 2 | Should a `### Glitches` subsection be added to Emerald for the Battle Tower clone glitch? | Open — future content PR |
| 3 | Should the How To column be surfaced to players directly, or only to the LLM? | Open — currently LLM context only |
| 4 | Should glitch recs be suppressed on Nuzlocke runs (cloning would violate the spirit)? | Open — V1 allows coexistence |

---
name: coach-llm-integration
description: "AI Coach / Ollama LLM integration in internal/services/coach.go and internal/handlers/coach.go. Use when: adding coach features, modifying LLM payloads, implementing COACH-012 through COACH-015 PRD items, debugging Ollama connectivity, changing system prompts, working on graceful degradation, fixing SSRF validation (SEC-005), updating coach templates, or modifying keep_alive/VRAM behavior."
---

# Coach / LLM Integration

The Coach subsystem (`internal/services/coach.go`, `internal/handlers/coach.go`) provides optional AI-powered team recommendations via an Ollama LLM. It degrades gracefully when unavailable.

## Service Layer (services/coach.go)

### CoachClient

```go
type CoachClient struct {
    host         string        // Base URL (e.g., "http://ollama-lxc:11434"); empty = disabled
    model        string        // Ollama model (default: "qwen2.5:3b")
    systemPrompt string        // Persona instructions; empty = omitted from request
    http         *http.Client  // 120-second timeout
}
```

### Core Methods

| Method | Purpose |
|--------|---------|
| `NewCoachClient(host, model, systemPrompt)` | Constructor; empty host = disabled state |
| `ValidateConfig()` | Startup URL validation (SEC-005); rejects non-http/https schemes, empty hosts |
| `IsAvailable()` | Returns false if host empty or `GET {host}/` != 200 |
| `QueryCoach(runID, payload)` | Sends prompt to Ollama, returns `CoachResponse` |

### Graceful Degradation Pattern

**Every error path returns `CoachResponse{Available: false}` — never panics or returns errors to callers.**

- Empty host → `Available: false`
- Network error → `Available: false`
- HTTP non-200 → `Available: false`
- Malformed JSON → `Available: false`
- Handlers check `IsAvailable()` and also handle `Available: false` in response
- Frontend disables Coach UI section when unavailable

### Ollama Request Shape

```json
{
  "model": "qwen2.5:3b",
  "stream": false,
  "keep_alive": 0,
  "messages": [
    { "role": "system", "content": "<systemPrompt>" },
    { "role": "user", "content": "<formatPayload output>" }
  ]
}
```

**Critical:** `keep_alive: 0` is hardcoded (not configurable). GPU (GTX 970) is shared with Immich — model must evict from VRAM immediately after response.

System message is **omitted entirely** when `systemPrompt` is empty (message array has 1 element instead of 2).

### Payload Construction

```go
type CoachPayload struct {
    Candidates  CoachCandidates
    Question    string
    ContextNote string           // Game version metadata
}

type CoachCandidates struct {
    Acquisitions   interface{}  // Catchable Pokémon at location
    Items          interface{}  // Owned + shop items
    PartyMoves     interface{}  // Current party + learnable moves
    TeamAnalysis   interface{}  // Type coverage (omitempty)
    EvolutionPaths interface{}  // Evolution routes (omitempty)
    PartyDetails   interface{}  // Base stats, abilities, types (omitempty)
}
```

`formatPayload()` serializes the payload into a human-readable prompt string: context note → JSON game data → question.

### Response Contract

```go
type CoachResponse struct {
    Available bool   // true only if done=true AND non-empty content
    Answer    string // LLM text (empty if !Available)
    Model     string // Echo of model used
    Truncated bool   // true if done_reason == "length"
}
```

## Security

### SEC-005: URL Validation (ValidateConfig)

Called at startup. Prevents SSRF by:
- Rejecting non-http/https schemes (`ftp://`, `file://`, etc.)
- Rejecting URLs with empty host field
- Empty host is valid (disabled state, not an error)

### SEC-009: Input Length Validation

Question field capped at **2000 characters** in handler (prevents token abuse on LLM gateway).

## Handler Layer (handlers/coach.go)

### Routes

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| GET | `/runs/:run_id/coach` | `ShowCoach()` | Renders coach.html with game data |
| POST | `/runs/:run_id/coach` | `QueryCoach()` | Processes question, calls LLM, re-renders |

### buildCoachPage() Flow

1. **Acquisitions** — `legality.LegalAcquisitions()` deduplicated by (species, form, location) with merged level ranges
2. **Trades** — `legality.LegalTrades()` converted to `TradeOption` structs
3. **Items** — `legality.LegalItems()` (owned) + `legality.ShopItems()` (purchasable)
4. **Party Moves** — Query party (`in_party = 1`), fetch `legality.CoachMoves()` per member; triggers background `pokeClient.GoEnsurePokemon()` for evolution targets missing learnset data
5. **Team Insights** — `buildTeamInsights()`:
   - Per-member: types, base stats, abilities, current moves (from `run_pokemon_move`)
   - Team-wide: defensive profile (weaknesses/resistances/immunities), offensive coverage gaps
   - Evolution paths via `legality.FindEvolutionPaths()`

### CoachPage Struct

```go
type CoachPage struct {
    BasePage
    CoachAvailable bool
    Acquisitions   []legality.Acquisition
    Trades         []TradeOption
    PartyMoves     []PartyMoveSummary
    LegalItems     []ItemOption
    TeamInsights   *TeamInsights
    PlayerQuestion string
    CoachAnswer    *CoachAnswer  // nil if no response
}
```

## Testing Patterns (services/coach_test.go)

Uses `httptest.NewServer` for mock Ollama endpoints with 2-second timeout.

Key test areas:
- **IsAvailable**: empty host → false; 200 → true; 500 → false
- **QueryCoach**: success → Available=true; server error → Available=false; malformed JSON → Available=false
- **keep_alive**: request body verified to contain `"keep_alive": 0`
- **Truncation**: `done_reason: "length"` → `Truncated: true`
- **System prompt omission**: empty prompt → message count = 1
- **ValidateConfig**: empty=ok, http/https=ok, ftp=error, empty host=error

## Pending PRD Items

From [docs/prds/coach-recommendations.md](./docs/prds/coach-recommendations.md):

### COACH-012: Default System Prompt (High priority, Low effort)

Add `defaultSystemPrompt` constant with Professor Arbortom persona. `NewCoachClient()` uses default when systemPrompt arg is empty. `COACH_SYSTEM_PROMPT` env var replaces default when set. Persona defines 4 recommendation categories: Move+Evolution, Catches, Items, Team Theme.

### COACH-013: Learnable Move Stats (Medium priority, Medium effort)

JOIN move table in `CoachMoves()` / `LegalMoves()` to add Power (`*int`, nil for status), Accuracy (`*int`, nil for never-miss), PP (`int`) to `Move` struct. No migration needed — data already in `move` table. Propagate to `MoveOption` in handlers via `moveToOption()`.

### COACH-014: Gym Leader & Elite Four Schema (High priority, Medium effort)

New migration `016_opponent_teams.sql` with `gym_leader` (version_id, badge_order 1–13, name, type_specialty, location_name) and `gym_leader_pokemon` (slot 1–6, form_id, level, held_item, moves 1–4) tables. Seed all 5 Gen 3 versions × 13 opponents = 65 leaders, ~226 Pokémon rows.

### COACH-015: Next Opponents Integration (High priority, Low effort, depends on COACH-014)

`nextOpponents(db, runID)` queries next 2 gym leaders by `badge_order > current_badge_count`. Adds `NextOpponents []OpponentSummary` to `CoachPage` and `CoachCandidates.NextOpponents` to payload. Gracefully returns nil if `gym_leader` table doesn't exist (pre-migration). Template shows "Next Battle" section.

## Environment Variables

| Var | Default | Purpose |
|-----|---------|---------|
| `COACH_HOST` | empty (disabled) | Ollama base URL |
| `COACH_MODEL` | `qwen2.5:3b` | LLM model name |
| `COACH_SYSTEM_PROMPT` | empty (uses default after COACH-012) | Custom persona override |

# PokemonProfessor — HTTP API Specification

**Server**: Gin (`github.com/gin-gonic/gin`)
**Port**: `PORT` env var (default `8000`)
**Base URL**: `http://192.168.1.131:8000` (LAN)

Cross-reference: [architecture.md](architecture.md) for module structure,
[schema.md](schema.md) for data types, [gui.md](gui.md) for template consumers.

---

## Conventions

### HTML Routes (browser-facing)

- All return `text/html` via `html/template` rendering
- Form submissions use `POST` with `application/x-www-form-urlencoded`
- Validation errors: **re-render the same form** with inline error messages — no redirect on error
- Successful mutations: **redirect** to the relevant `GET` route (Post-Redirect-Get pattern)
- Flash messages stored in a signed cookie (`gin-contrib/sessions` or equivalent); single key `flash`

### JSON Routes (`/api/*`)

- All return `application/json`
- Success: HTTP 200 with payload struct
- Error: HTTP 4xx/5xx with `{"error": "...", "field": "..."}` (`field` omitted if not field-specific)
- No authentication — LAN access only; UFW restricts port 8000 to LAN

### Error Handling

- DB unavailable at startup: binary exits with non-zero code (systemd will restart)
- DB error during request: HTML routes render a minimal error page; JSON routes return 500
- PokeAPI failure during legality query: returns candidate set with `warnings` populated — never 500
- ZeroClaw failure during Coach POST: returns `CoachResponse{Available: false}` — never 500

---

## Middleware

### `RunContextMiddleware`

Applied to all routes under `/runs/:run_id/*`. Runs before the handler.

Loads from DB and stores in Gin context (`c.Set`):

| Key | Type | Content |
|-----|------|---------|
| `"run"` | `models.Run` | `id`, `name`, `version_id`, `user_id` |
| `"progress"` | `models.RunProgress` | `badge_count`, `current_location_id` |
| `"active_rules"` | `[]models.ActiveRule` | `key`, `enabled`, `params_json` |
| `"version"` | `models.GameVersion` | `id`, `name`, `version_group_id` |

If `run_id` not found: renders 404 page (HTML) or returns `{"error": "run not found"}` (JSON).

All templates receive a `RunContext` struct populated from these keys — handlers must not re-query
this data.

---

## Route Table

### Root

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/` | `handlers.RedirectToRuns` | 302 → `/runs` |
| `GET` | `/health` | `handlers.Health` | JSON |

### Runs

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs` | `handlers.ListRuns` | HTML: `runs.html` |
| `POST` | `/runs` | `handlers.CreateRun` | 302 → `/runs/:id/progress` on success; re-render on error |
| `GET` | `/runs/:run_id` | `handlers.ShowRun` | 302 → `/runs/:run_id/progress` |

### Progress

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs/:run_id/progress` | `handlers.ShowProgress` | HTML: `progress.html` |
| `POST` | `/runs/:run_id/progress` | `handlers.UpdateProgress` | 302 → same GET on success |

### Team & Box

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs/:run_id/team` | `handlers.ShowTeam` | HTML: `team.html` |
| `POST` | `/runs/:run_id/team` | `handlers.UpdateTeam` | 302 → GET on success; re-render on legality violation |
| `GET` | `/runs/:run_id/box` | `handlers.ShowBox` | HTML: `box.html` |
| `POST` | `/runs/:run_id/box/:entry_id/faint` | `handlers.MarkFainted` | 302 → `/runs/:run_id/box` |
| `POST` | `/runs/:run_id/box/:entry_id/revive` | `handlers.MarkRevived` | 302 → `/runs/:run_id/box` (only if Nuzlocke rule disabled) |

### Routes Log

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs/:run_id/routes` | `handlers.ShowRoutes` | HTML: `routes.html` |
| `POST` | `/runs/:run_id/routes` | `handlers.LogEncounter` | 302 → GET on success; re-render with warning on Nuzlocke duplicate |

### Rules

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs/:run_id/rules` | `handlers.ShowRules` | HTML: `rules.html` |
| `POST` | `/runs/:run_id/rules` | `handlers.UpdateRules` | 302 → GET on success |

### Coach

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/runs/:run_id/coach` | `handlers.ShowCoach` | HTML: `coach.html` |
| `POST` | `/runs/:run_id/coach` | `handlers.QueryCoach` | HTML: `coach.html` with response inline (no redirect) |

### API (JSON — Coach agent skill path)

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/api/legal/acquisitions/:run_id` | `handlers.APILegalAcquisitions` | JSON |
| `GET` | `/api/legal/moves/:run_id/:form_id` | `handlers.APILegalMoves` | JSON |
| `GET` | `/api/legal/items/:run_id` | `handlers.APILegalItems` | JSON |
| `GET` | `/api/legal/evolutions/:run_id/:form_id` | `handlers.APILegalEvolutions` | JSON |

---

## Request / Response Shapes

### `POST /runs` — Create Run

Form fields:

| Field | Type | Validation |
|-------|------|-----------|
| `user_name` | string | required, 1–50 chars; creates user if not exists |
| `run_name` | string | required, 1–100 chars |
| `version_id` | integer | required; must exist in `game_version` |

On success: inserts `user` (or finds existing), inserts `run`, inserts `run_progress` with defaults,
inserts one `run_rule` row per `rule_def` (all `enabled=0`). Redirects to progress page.

---

### `POST /runs/:run_id/progress` — Update Progress

Form fields:

| Field | Type | Notes |
|-------|------|-------|
| `badge_count` | integer | 0–8 |
| `current_location_id` | integer | must belong to run's `version_id` |
| `flags` | repeated string | flag keys to set `value='true'`; absent keys set `value='false'` |

Triggers `EnsureLocationEncounters` for the new location before responding (background goroutine,
non-blocking — response does not wait for PokeAPI fetch).

---

### `POST /runs/:run_id/team` — Update Team Slot

Form fields:

| Field | Type | Notes |
|-------|------|-------|
| `slot` | integer | 1–6 |
| `form_id` | integer | validated against `legal_acquisitions` result |
| `level` | integer | 1–100 |
| `move_1` through `move_4` | integer | move IDs; validated against `legal_moves` result |
| `held_item_id` | integer | optional; validated against `legal_items` result |

**Legality check**: handler calls `LegalAcquisitions`, `LegalMoves(form_id)`, `LegalItems` and
verifies each submitted value is in the returned sets. On failure: re-renders `team.html` with
`LegalityErrors` map keyed by field name (e.g. `{"move_2": "Surf is not yet obtainable"}`).

---

### `POST /runs/:run_id/routes` — Log Encounter

Form fields:

| Field | Type | Notes |
|-------|------|-------|
| `location_id` | integer | must belong to run's version |
| `form_id` | integer | the Pokémon encountered |
| `outcome` | string | `caught`, `fainted`, `fled`, `skipped` |
| `level` | integer | required if `outcome = caught` |

**Nuzlocke duplicate check**: if `nuzlocke` rule enabled and `outcome = caught` and
`location_id` already appears in `run_box.met_location_id` for this run: re-render with yellow
warning badge `"Nuzlocke: you already caught a Pokémon on this route"`. Does **not** block the
submission — player may have a rule variant that allows it.

---

### `POST /runs/:run_id/rules` — Update Rules

Form encodes all rule states. For each `rule_def`:

| Field | Type | Notes |
|-------|------|-------|
| `rule_{key}` | checkbox | present = enabled |
| `rule_{key}_params` | string (JSON) | only for parameterized rules (`level_cap`) |

For `level_cap`: `rule_level_cap_params` should be `{"cap": <integer>}`. Validated: cap must be
1–100 or absent (uses `gen3_badge_cap` table instead when absent).

---

### `POST /runs/:run_id/coach` — Query Coach

Form fields:

| Field | Type | Notes |
|-------|------|-------|
| `question` | string | player's free-text question |

Handler flow:
1. Calls `LegalAcquisitions`, `LegalMoves` for each party slot, `LegalItems`
2. Serializes candidate sets to JSON
3. Calls `services.zeroclaw.QueryCoach(runID, CoachPayload{Candidates: ..., Question: ...})`
4. If ZeroClaw unavailable: renders `coach.html` with `ZeroClawAvailable: false`
5. If ZeroClaw returns response: renders `coach.html` with `CoachAnswer` populated

Does **not** redirect — response is rendered inline in the coaching panel.

---

## JSON Response Structs

### `GET /health`

```json
{
  "status": "ok",
  "db": "ok",
  "zeroclaw": "unavailable",
  "version": "abc1234"
}
```

`zeroclaw` values: `"available"` | `"unavailable"`. **HTTP 200 in both cases** — ZeroClaw
unavailable is an expected operating mode, not an error.

`version` is the git commit SHA, injected at build time via `-ldflags "-X main.Version=$(git rev-parse --short HEAD)"`.

---

### `GET /api/legal/acquisitions/:run_id`

```json
{
  "run_id": 1,
  "badge_count": 3,
  "acquisitions": [
    {
      "form_id": 16,
      "species_name": "pidgey",
      "form_name": "default",
      "location_name": "route-1",
      "method": "walk",
      "min_level": 2,
      "max_level": 5,
      "blocked_by_rule": null
    }
  ],
  "warnings": []
}
```

`blocked_by_rule`: `null` | `"nuzlocke"` | `"level_cap"` — present in the set but annotated, not
removed, so the Coach agent can explain why something is blocked.

---

### `GET /api/legal/moves/:run_id/:form_id`

```json
{
  "run_id": 1,
  "form_id": 16,
  "moves": [
    {
      "move_id": 33,
      "name": "tackle",
      "type_name": "normal",
      "learn_method": "level-up",
      "level_learned": 1,
      "blocked_by_rule": null
    }
  ],
  "warnings": []
}
```

---

### `GET /api/legal/items/:run_id`

```json
{
  "run_id": 1,
  "items": [
    {
      "item_id": 1,
      "name": "master-ball",
      "category": "standard-balls",
      "source": "owned",
      "qty": 1
    }
  ]
}
```

`source`: `"owned"` (in `run_item`) | `"obtainable"` (in `item_availability` for accessible location).

---

### `GET /api/legal/evolutions/:run_id/:form_id`

```json
{
  "run_id": 1,
  "form_id": 16,
  "evolutions": [
    {
      "to_form_id": 17,
      "to_species_name": "pidgeotto",
      "trigger": "level-up",
      "conditions": {"min_level": 18},
      "currently_possible": true,
      "blocked_by_rule": null
    }
  ]
}
```

`currently_possible`: `true` if all conditions are satisfied given current run state (level,
held items, story flags). `false` with reason annotation if not yet satisfiable.

---

## Gin Router Wiring (`cmd/pokemonprofessor/main.go`)

```go
r := gin.Default()
r.SetHTMLTemplate(templates)  // from go:embed
r.Static("/static", ...)      // from go:embed

r.GET("/", handlers.RedirectToRuns)
r.GET("/health", handlers.Health(db, zc))

runs := r.Group("/runs")
{
    runs.GET("", handlers.ListRuns(db))
    runs.POST("", handlers.CreateRun(db))

    run := runs.Group("/:run_id", middleware.RunContext(db))
    {
        run.GET("", handlers.ShowRun)
        run.GET("/progress", handlers.ShowProgress(db))
        run.POST("/progress", handlers.UpdateProgress(db, pokeapiClient))
        run.GET("/team", handlers.ShowTeam(db))
        run.POST("/team", handlers.UpdateTeam(db))
        run.GET("/box", handlers.ShowBox(db))
        run.POST("/box/:entry_id/faint", handlers.MarkFainted(db))
        run.POST("/box/:entry_id/revive", handlers.MarkRevived(db))
        run.GET("/routes", handlers.ShowRoutes(db))
        run.POST("/routes", handlers.LogEncounter(db))
        run.GET("/rules", handlers.ShowRules(db))
        run.POST("/rules", handlers.UpdateRules(db))
        run.GET("/coach", handlers.ShowCoach(db))
        run.POST("/coach", handlers.QueryCoach(db, zc))
    }
}

api := r.Group("/api")
{
    legal := api.Group("/legal")
    {
        legal.GET("/acquisitions/:run_id", handlers.APILegalAcquisitions(db))
        legal.GET("/moves/:run_id/:form_id", handlers.APILegalMoves(db))
        legal.GET("/items/:run_id", handlers.APILegalItems(db))
        legal.GET("/evolutions/:run_id/:form_id", handlers.APILegalEvolutions(db))
    }
}
```

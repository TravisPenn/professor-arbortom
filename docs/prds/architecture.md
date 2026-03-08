# PokemonProfessor — Architecture

**Repo**: `github.com/<owner>/pokemonprofessor`
**Status**: Deployed (Gen 3 complete)
**Updated**: March 11, 2026

> This document is the authoritative architecture reference for the `pokemonprofessor` application.
> Infrastructure deployment (LXC provisioning, Ansible playbooks, systemd unit) is managed in the
> `proxmox` repo — see `docs/prds/agent-pokeprofessor.md` there.

---

## 1. What This System Does

PokemonProfessor is a legality-first Pokémon run tracker. It tells players what they can legally
obtain, use, and evolve at their current point in a playthrough, respecting version differences and
player-imposed restrictions (Nuzlocke, level caps, theme runs).

It is **not** a battle simulator, EV/IV optimizer, or cloud service. It runs fully offline once
game data is seeded from PokeAPI.

---

## 2. Component Diagram

```
LAN (<your-subnet>)
│
│   Browser
│     │  HTTP :8000
│     ▼
└── pokemonprofessor-host (<your-host-ip>)
      │
      │  /usr/local/bin/pokemonprofessor   ← single static Go binary
      │  /data/pokemonprofessor/db/pokemon.sqlite
      │  /data/pokemonprofessor/config/.env
      │
      │  ZEROCLAW_GATEWAY (optional, blank = disabled)
      │     │
      │     ▼ HTTP :42617
      │  zeroclaw-host (<your-zeroclaw-ip>)   ← separate PRD, deploy independently
      │     └── Ollama :11434 (localhost)
      │          └── llama3:8b
      │
      │  PokeAPI (https://pokeapi.co)
      │     └── lazy-fetched at runtime, cached in SQLite
      │          never re-fetched once cached (offline after seed)
```

**LXC 131 is self-contained.** ZeroClaw unavailable = Coach AI panel shows placeholder banner.
All other screens function fully without network access after initial PokeAPI seed.

---

## 3. Go Module Structure

**Module path**: `github.com/<owner>/pokemonprofessor`

```
cmd/pokemonprofessor/
  main.go               ← entrypoint: env load, DB init, Gin router, HTTP listen

internal/
  db/
    db.go               ← open connection, set PRAGMAs (WAL, foreign_keys=ON, busy_timeout)
    migrate.go          ← check PRAGMA user_version, apply missing migrations, bump version

  pokeapi/
    client.go           ← stdlib net/http wrapper; respects api_cache_log; Gen 3 version
                           group guard (rejects vg IDs outside {5,6,7})
    pokemon.go          ← EnsurePokemon(db, formID, versionGroupID)
    location.go         ← EnsureLocationEncounters(db, locationID, versionID)
    item.go             ← EnsureItem(db, itemID, versionID)
    evolution.go        ← EnsureEvolutionChain(db, formID)
    region.go           ← EnsureRegionLocations(db, regionID); EnsureAllEncounters(db, versionID)
                           RegionIDForVersionID(versionID) int

  legality/
    acquisitions.go     ← LegalAcquisitions(db, runID) ([]Acquisition, []Warning, error)
    moves.go            ← LegalMoves(db, runID, formID) ([]Move, []Warning, error)
    items.go            ← LegalItems(db, runID) ([]Item, error)
    evolutions.go       ← EvolutionOptions(db, runID, formID) ([]Evolution, error)
    rules.go            ← ApplyRules(db, runID, candidates) — decoration layer
    skills.go           ← FetchEncounters, FetchLearnset, FetchItemLocation, FetchEvolution
                           (thin wrappers; reserved for Coach agent JSON path)
    types.go            ← Acquisition, Move, Item, Evolution, Warning structs

  handlers/
    runs.go             ← GET /runs, POST /runs, GET|POST /runs/:id/archive|unarchive
    progress.go         ← GET|POST /runs/:id/progress; GET /runs/:id/progress/hydration
    team.go             ← GET|POST /runs/:id/team; GET /runs/:id/team/:slot
    pages.go            ← all page/struct definitions (BasePage, RunContext, etc.)
    routes.go           ← GET|POST /runs/:id/routes
    rules.go            ← GET|POST /runs/:id/rules
    coach.go            ← GET|POST /runs/:id/coach
    api.go              ← /api/legal/*, /health  (JSON)
    middleware.go       ← RunContextMiddleware
    loaders.go          ← shared DB query helpers (loadLocations, loadFlags, etc.)
    util.go             ← shared response helpers, formInt, itoa, etc.
    converters.go       ← PokeAPI → model conversions

  services/
    zeroclaw.go         ← IsAvailable(), QueryCoach() — only file making outbound calls to LXC 130

migrations/
  001_initial.sql       ← core schema (game data + run tracking tables)
  002_starters.sql      ← game_starter table; Gen 3 starter species pre-seeded
  003_merge_pokemon.sql ← run_party + run_box → unified run_pokemon table
  004_archive_run.sql   ← run.archived_at column (soft-archive)
  005_static_locations.sql ← negative-ID static town/city locations for Gen 3
  006_coach_improvements.sql ← in_game_trade + shop_item tables; NPC trades + Game Corner data
  007_tm_moves.sql      ← tm_move table; Gen 3 TM→move mapping
  008_hm_tutor_moves.sql ← hm_move + tutor_move tables; HM list + tutor locations

templates/              ← embedded via //go:embed
  base.html
  runs.html, overview.html, progress.html
  team.html, team_slot.html, box.html
  routes.html, rules.html, coach.html

static/                 ← embedded via //go:embed
  style.css

data/
  data.go               ← embeds seeds.sql; exposes SeedsSQL []byte
  seeds.sql             ← pre-built reference data applied on first startup

docs/
  architecture.md       ← this file
  schema.md
  api.md
  gui.md

Dockerfile              ← multi-stage; final stage FROM scratch
go.mod
.gitignore
README.md
```

---

## 4. Deployment Contract

The binary is built outside LXC 131 (multi-stage Docker build on Windows) and deployed by
Ansible via `pct push`. It receives its runtime environment through:

| Source | How | Content |
|--------|-----|---------|
| `.env` file | `EnvironmentFile=` in systemd unit | `POKEMON_DB_PATH`, `ZEROCLAW_GATEWAY`, `ZEROCLAW_AGENT`, `PORT` |
| Bind mount | bind mount in host config | `/data/pokemonprofessor` available at the same path inside the container |
| systemd | `ExecStart=/usr/local/bin/pokemonprofessor` | Binary invocation |

The binary **must not** assume any filesystem path exists except those declared in `.env`. On
first start it creates the SQLite DB file at `POKEMON_DB_PATH`, runs all pending migrations
(currently 8), and applies `seeds.sql` reference data if the DB is freshly empty.

**Binary build command** (run on Windows, output copied to Proxmox host):
```
docker build -t pokemonprofessor:latest .
docker create --name extract pokemonprofessor:latest
docker cp extract:/app/pokemonprofessor ./pokemonprofessor-linux-amd64
docker rm extract
```

The Proxmox Ansible playbook then does:
```
scp pokemonprofessor-linux-amd64 root@<your-host-ip>:/tmp/
ansible-playbook playbooks/deploy-pokemonprofessor.yml
```

---

## 5. ZeroClaw Optional Integration

ZeroClaw (LXC 130) is a separate service. PokemonProfessor connects to it only for the Coach AI
panel. The integration is fully optional and degrades gracefully:

| `ZEROCLAW_GATEWAY` env var | Behaviour |
|---------------------------|-----------|
| Blank or unset | `IsAvailable()` returns `false`; Coach panel shows placeholder banner; all other screens fully functional |
| Set to valid URL | `IsAvailable()` returns `true`; `/runs/:id/coach` POST calls ZeroClaw; if call fails mid-request, returns `CoachResponse{Available: false}` — no 500 |

**Agent profile**: `[agents.pokemonprofessor]` must be present in LXC 130's
`/data/zeroclaw/config/config.toml`. It is appended by the deploy playbook,
not by this application.

**What the Coach agent receives**: The `/runs/:id/coach` POST handler assembles the full legality
candidate set (legal acquisitions + party move options) and sends it as context to ZeroClaw along
with the player's question. The agent never receives raw SQLite data — only the pre-filtered
candidate structs.

---

## 6. PokeAPI Data Layer

PokeAPI (`https://pokeapi.co`) is used once per resource — when the legality engine needs data
that isn't in the cache. After initial population the system is fully offline.

**Fetch lifecycle**:
1. Legality query calls `Ensure*(db, id, ...)` before running its SQL
2. `Ensure*` checks `api_cache_log` for `(resource, resource_id)` — if found, returns immediately
3. If not cached: fetches from PokeAPI, writes to cache tables, inserts into `api_cache_log`
4. On PokeAPI error: logs warning, returns empty slice + `Warning` — never crashes the request

**Gen 3 scope enforcement**: `client.go` rejects requests for version group IDs outside `{5, 6, 7}`.
This is a compile-time constant, not a runtime config. Expanding to Gen 4+ requires adding IDs to
this set; the schema is already generation-agnostic (all tables are version-group-keyed) so no
migration is needed to add new generations.

**Gen 3 version group IDs** (sourced from PokeAPI, immutable):

| ID | Name | Games |
|----|------|-------|
| 5 | ruby-sapphire | Ruby, Sapphire |
| 6 | emerald | Emerald |
| 7 | firered-leafgreen | FireRed, LeafGreen |

---

## 7. Technology Choices

| Choice | Rationale |
|--------|-----------|
| **Go** | Single static binary; ~10MB vs ~150MB Python runtime; 2GB RAM LXC constraint |
| **Gin** | Middleware for `RunContextMiddleware` (run context injection) and flash messages; one dep justifies ergonomic routing |
| **`modernc.org/sqlite`** | Pure Go SQLite driver; no CGO; no gcc in LXC 131 |
| **`html/template` stdlib** | No JS framework; templates embedded at compile time |
| **`//go:embed`** | Templates and static files compiled into binary; fully self-contained artifact |
| **`net/http` stdlib** | PokeAPI calls; no external HTTP client dependency |
| **`github.com/joho/godotenv`** | `.env` loading; one small dep |
| **Raw SQL, no ORM** | Schema is explicit in `schema.md`; legality queries are complex JOINs that belong in SQL not ORM wrappers |
| **`data/seeds.sql` embed** | Pre-built reference data applied on first startup so new installs work offline immediately |
| **Unified `run_pokemon` table** | Replaces separate `run_party` + `run_box` (migration 003); single source of truth for every owned Pokémon with `in_party`/`party_slot` columns |

**Total external runtime dependencies**: `gin-gonic/gin`, `modernc.org/sqlite`, `joho/godotenv`. PokeAPI calls use stdlib `net/http` — no extra HTTP client library needed.

---

## 8. Security Notes

- LXC 131 is **unprivileged** — no Docker, no GPU passthrough needed
- UFW: allow 22 (SSH), allow 8000 (app), deny all else
- `.env` is gitignored in the application repo; deployed by Ansible from operator's machine
- No secrets in the binary or templates
- ZeroClaw connection is one-way outbound from LXC 131 — no inbound port opened for LXC 130
- PokeAPI calls are read-only with no API key required

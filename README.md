# Professor Arbortom

A legality-first Pokémon run tracker. Tells you what you can legally obtain, use, and
evolve at your current point in a playthrough — respecting version differences and
player-imposed restrictions (Nuzlocke, level caps, theme runs).

Currently implemented for **Gen 3** (FireRed, LeafGreen, Ruby, Sapphire, Emerald), with the
architecture designed to support all generations going forward.

Runs fully offline after initial PokeAPI seed. Served as a single static Go binary on a LAN.

## Architecture

See [`docs/prds/architecture.md`](docs/prds/architecture.md).

## Building

```sh
# Build Linux binary from Windows via Docker
docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t professor-arbortom:latest .
docker create --name extract professor-arbortom:latest
docker cp extract:/app/professor-arbortom ./professor-arbortom-linux-amd64
docker rm extract
```

## Running locally

```sh
cp .env.example .env
# edit .env: set POKEMON_DB_PATH, PORT
go run ./cmd/professor-arbortom
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POKEMON_DB_PATH` | yes | — | Path to SQLite DB file (created on first run) |
| `PORT` | no | `8000` | HTTP listen port |
| `ZEROCLAW_GATEWAY` | no | `""` | ZeroClaw LXC base URL; blank = AI coach disabled |
| `ZEROCLAW_AGENT` | no | `""` | ZeroClaw agent profile name |

## Project Structure

```
cmd/professor-arbortom/   entrypoint
internal/
  db/                   DB connection + migrations
  pokeapi/              PokeAPI fetch + cache layer
  legality/             Legality engine
  handlers/             Gin HTTP handlers
  services/             External service clients (ZeroClaw)
migrations/             SQL migration files
templates/              HTML templates (go:embed)
static/                 CSS + assets (go:embed)
docs/prds/              Product requirements documents
```

## Dependencies

### Runtime

| Package | Version | Purpose |
|---------|---------|---------|
| [github.com/gin-gonic/gin](https://github.com/gin-gonic/gin) | v1.10.0 | HTTP web framework (routing, middleware, rendering) |
| [github.com/joho/godotenv](https://github.com/joho/godotenv) | v1.5.1 | `.env` file loading for local development |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | v1.29.9 | Pure-Go CGO-free SQLite driver (no C toolchain needed) |

### External Data Source

| Source | Usage |
|--------|-------|
| [PokéAPI](https://pokeapi.co) | Seeded once at startup to populate Pokémon, moves, items, evolutions, and locations. No runtime network dependency after the initial seed. |

### Build Toolchain

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.22 | `CGO_ENABLED=0` — pure-Go build |
| Docker | — | Multi-stage build; final image is `FROM scratch` (no OS layer) |

> Indirect dependencies (JSON codecs, validators, SQLite internals, etc.) are managed automatically via `go.mod` / `go.sum` and do not require manual installation.

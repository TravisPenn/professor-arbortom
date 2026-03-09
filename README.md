# PokemonProfessor

A legality-first Pokémon Gen 3 run tracker. Tells you what you can legally obtain, use, and
evolve at your current point in a playthrough — respecting version differences and
player-imposed restrictions (Nuzlocke, level caps, theme runs).

Runs fully offline after initial PokeAPI seed. Served as a single static Go binary on a LAN.

## Architecture

See [`docs/prds/architecture.md`](docs/prds/architecture.md).

## Building

```sh
# Build Linux binary from Windows via Docker
docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t pokemonprofessor:latest .
docker create --name extract pokemonprofessor:latest
docker cp extract:/app/pokemonprofessor ./pokemonprofessor-linux-amd64
docker rm extract
```

## Running locally

```sh
cp .env.example .env
# edit .env: set POKEMON_DB_PATH, PORT
go run ./cmd/pokemonprofessor
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
cmd/pokemonprofessor/   entrypoint
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

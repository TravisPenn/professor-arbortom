# syntax=docker/dockerfile:1
# Multi-stage build — final image is FROM scratch (no OS, no shell)

# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Run tests before building — fail fast on any regression
RUN CGO_ENABLED=0 go test ./...

# Inject git commit SHA; CGO disabled for pure-Go sqlite driver
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/pokemonprofessor \
    ./cmd/pokemonprofessor

# SEC-017: Create a non-root user for the final image.
RUN adduser -D -u 10001 appuser

# ── Stage 2: final ──────────────────────────────────────────────────────────
FROM scratch

# CA certs needed for HTTPS calls to pokeapi.co
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# SEC-017: Copy passwd so scratch image can run as non-root.
COPY --from=builder /etc/passwd /etc/passwd

# Copy the statically linked binary
COPY --from=builder /app/pokemonprofessor /app/pokemonprofessor

# SEC-017: Run as non-root user.
USER appuser

# The binary must not embed DB — DB lives on the host bind-mount.
# Templates and static files ARE embedded at compile time via go:embed.

# SEC-007: Run Gin in release mode in production.
ENV GIN_MODE=release

EXPOSE 8000

ENTRYPOINT ["/app/pokemonprofessor"]

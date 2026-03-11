# PokemonProfessor — Security Remediation PRD

**Status**: Draft
**Priority**: Critical
**Date**: 2026-03-11

Cross-reference: [api.md](api.md) for route inventory, [architecture.md](architecture.md) for module layout, [schema.md](schema.md) for data model.

---

## Executive Summary

A security review of the PokemonProfessor Go module identified **18 findings** spanning 7 of the OWASP Top 10 categories. The application currently operates on a trusted LAN with no authentication, no CSRF protection, no security headers, and several code-level anti-patterns that would become exploitable if the deployment model changes. This PRD defines the remediation items, ordered by severity.

---

## SEC-001 · No Authentication or Authorization (CRITICAL)

**OWASP**: A01 Broken Access Control
**Location**: All route handlers; `cmd/pokemonprofessor/main.go` (routing setup)

### Problem

The application has zero authentication. Every endpoint is publicly accessible to anyone who can reach the port. All run data is accessible and mutable by any client via direct URL manipulation (IDOR — Insecure Direct Object Reference). A user can read, modify, archive, or delete any other user's run by changing the `run_id` path parameter.

The current security model ("UFW restricts port 8000 to LAN") is documented in [api.md](api.md) but is a network-level control, not an application-level one. It provides no isolation between users on the same LAN.

### Required Actions

1. Implement session-based authentication using `gin-contrib/sessions` with a cookie store.
2. Add a login/register flow (can be simple username-based for LAN use, or username+password).
3. Store sessions in a signed, `HttpOnly`, `SameSite=Strict` cookie.
4. Add authorization middleware to `/runs/:run_id/*` routes that verifies `session.user_id == run.user_id`.
5. Return HTTP 403 (not 404) when a user attempts to access another user's run.

### Acceptance Criteria

- [ ] Unauthenticated requests to any route except `/health` and `/` redirect to a login page.
- [ ] A user cannot read or mutate another user's runs.
- [ ] Session cookie is signed, `HttpOnly`, and `SameSite=Strict`.

---

## SEC-002 · No CSRF Protection (CRITICAL)

**OWASP**: A01 Broken Access Control
**Location**: All POST handlers — `runs.go`, `progress.go`, `team.go`, `routes.go`, `rules.go`, `coach.go`

### Problem

No CSRF tokens are generated or validated on any form submission. An attacker on the same LAN (or via a malicious page loaded in a browser on the LAN) can forge POST requests to archive runs, modify teams, update progress, or trigger PokeAPI hydration.

### Required Actions

1. Integrate a CSRF middleware (e.g. `gin-contrib/csrf` or a custom double-submit cookie approach).
2. Inject a CSRF token into every HTML form via a template function (e.g. `{{csrfField}}`).
3. Validate the token on every POST handler before processing.
4. For the JSON API routes under `/api/*`, either require a custom header (`X-Requested-With`) or exempt them and rely on CORS restrictions.

### Acceptance Criteria

- [ ] Every HTML `<form>` includes a hidden CSRF token field.
- [ ] POST requests without a valid CSRF token receive HTTP 403.
- [ ] CSRF token rotates per session.

---

## SEC-003 · Missing Security Headers (HIGH)

**OWASP**: A05 Security Misconfiguration
**Location**: `cmd/pokemonprofessor/main.go` — no middleware registered via `r.Use()`

### Problem

The Gin router uses `gin.Default()` (logger + recovery only). No security headers are set on responses:

| Header | Status | Risk |
|--------|--------|------|
| `Content-Security-Policy` | Missing | Enables XSS via injected inline scripts |
| `X-Frame-Options` | Missing | Enables clickjacking |
| `X-Content-Type-Options` | Missing | Enables MIME-sniffing attacks |
| `Referrer-Policy` | Missing | Leaks URL paths in referrer headers |
| `Permissions-Policy` | Missing | Allows browser API abuse |

### Required Actions

1. Add a middleware function that sets security headers on all responses:

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        c.Header("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'")
        c.Next()
    }
}
```

2. Register it with `r.Use(SecurityHeaders())` before any route definitions.

### Acceptance Criteria

- [ ] All responses include the five security headers listed above.
- [ ] No inline scripts or styles break under the CSP policy (audit templates).

---

## SEC-004 · SQL Injection via String Formatting (HIGH)

**OWASP**: A03 Injection
**Location**: `internal/db/migrate.go` — `setUserVersion()` at line ~112; `internal/db/seeds.go` — `appendTableInserts()` at line ~100

### Problem

**migrate.go**: `setUserVersion` uses `fmt.Sprintf("PRAGMA user_version = %d", v)`. While `%d` only formats integers (currently safe), this pattern normalizes `Sprintf`-based SQL construction. If the function signature ever changes to accept a string, it becomes injectable.

**seeds.go**: `appendTableInserts` constructs `"SELECT * FROM " + table` using a hardcoded table name list. The `table` variable comes from the internal `seedsTables` slice (currently safe), but the pattern is flagged by static analysis tools and sets a bad precedent.

### Required Actions

1. In `setUserVersion`: SQLite PRAGMAs cannot use parameterized queries. Add a bounds check (`v >= 0 && v <= 1000`) and a code comment explaining why `Sprintf` is acceptable here.
2. In `appendTableInserts`: Add a validation function that asserts the table name matches `^[a-z_]+$` before interpolation, and add a `//nolint:gosec` comment with justification.

### Acceptance Criteria

- [ ] `setUserVersion` rejects values outside a sane range.
- [ ] `appendTableInserts` validates table names against an allowlist pattern.
- [ ] Both locations have comments explaining why parameterization is not possible.

---

## SEC-005 · SSRF Risk in ZeroClaw Client (HIGH)

**OWASP**: A10 Server-Side Request Forgery
**Location**: `internal/services/zeroclaw.go` — `IsAvailable()` line ~54, `QueryCoach()` line ~68

### Problem

The `ZEROCLAW_GATEWAY` environment variable is used directly as a base URL for HTTP requests without any validation. If an attacker can influence the environment (e.g. in a shared hosting scenario or via a `.env` file write), they can redirect the server to make requests to arbitrary internal endpoints.

Additionally, the `z.agent` value is interpolated into the URL path (`fmt.Sprintf("%s/agent/%s/query", z.gateway, z.agent)`) without sanitization, allowing path traversal if the agent name contains `/` or `..` characters.

### Required Actions

1. Validate `ZEROCLAW_GATEWAY` at startup: parse as URL, enforce scheme is `http` or `https`, reject `localhost`/`127.0.0.1`/`0.0.0.0` unless explicitly allowed, reject non-LAN ranges if desired.
2. Validate `ZEROCLAW_AGENT` at startup: reject values containing `/`, `..`, or non-alphanumeric characters (allow `a-z`, `A-Z`, `0-9`, `-`, `_`).
3. Fail fast at startup (log.Fatal) if either value is malformed.

### Acceptance Criteria

- [ ] Application refuses to start with a malformed gateway URL.
- [ ] Application refuses to start with an agent name containing path traversal characters.
- [ ] Unit tests cover rejected inputs.

---

## SEC-006 · Error Messages Expose Internal Details (MEDIUM)

**OWASP**: A04 Insecure Design
**Location**: `internal/handlers/util.go` — `respondError()` at line ~37; `api.go` — multiple handlers

### Problem

`respondError()` passes `err.Error()` directly to both JSON and HTML responses. Go error messages frequently include file paths, SQL queries, table names, and driver-specific details that aid an attacker in reconnaissance.

Similarly, the JSON API handlers (`APILegalAcquisitions`, `APILegalMoves`, etc.) return `err.Error()` in 500 responses.

### Required Actions

1. In production mode, replace `respondError()` to return a generic message ("An internal error occurred") while logging the full error server-side.
2. Use `gin.Mode() == gin.ReleaseMode` to distinguish production from development.
3. Set `GIN_MODE=release` in the Dockerfile or systemd unit.

### Acceptance Criteria

- [ ] In release mode, no 500 response body contains SQL, file paths, or driver errors.
- [ ] Full error details are logged to server stderr/stdout.
- [ ] Dockerfile sets `GIN_MODE=release`.

---

## SEC-007 · Gin Debug Mode in Production (MEDIUM)

**OWASP**: A05 Security Misconfiguration
**Location**: `cmd/pokemonprofessor/main.go` — `gin.Default()` without mode setting; `Dockerfile`

### Problem

`gin.SetMode()` is never called outside test files. In production the default debug mode is used, which:
- Logs verbose route registration info
- Prints a warning banner on every startup
- Enables debug-level logging that may leak sensitive data

### Required Actions

1. Set `gin.SetMode(gin.ReleaseMode)` before creating the router, or read from a `GIN_MODE` env var.
2. Add `ENV GIN_MODE=release` to the Dockerfile.

### Acceptance Criteria

- [ ] Production deployments run in release mode.
- [ ] No debug banner appears in production logs.

---

## SEC-008 · Unchecked Errors on State-Mutating Operations (MEDIUM)

**OWASP**: A04 Insecure Design
**Location**: Multiple handlers with `//nolint:errcheck` on `db.Exec` and `db.QueryRow`

### Affected Files

| File | Line(s) | Operation |
|------|---------|-----------|
| `routes.go` | ~112 | `INSERT INTO run_pokemon` (caught encounter) |
| `team.go` | ~76, ~188 | `UPDATE run_pokemon` (evict from slot) |
| `progress.go` | ~136 | `INSERT OR REPLACE INTO run_flag` |
| `runs.go` | ~175 | `db.QueryRow` for `version_group_id` |
| `loaders.go` | ~52 | `ruleRows.Scan` |

### Problem

Write operations silently swallow errors. A failed `INSERT INTO run_pokemon` means a caught Pokémon vanishes without any user-visible feedback. A failed flag write means progress state is silently lost.

### Required Actions

1. Audit every `//nolint:errcheck` annotation. For each state-mutating `db.Exec`:
   - If the operation is critical (encounter log, team update), check the error and call `respondError()`.
   - If truly non-fatal, add a `log.Printf` and a comment explaining why.
2. For read-only `db.QueryRow` scans where ErrNoRows is acceptable, use named error handling (`if err != nil && err != sql.ErrNoRows`).

### Acceptance Criteria

- [ ] No `//nolint:errcheck` on INSERT or UPDATE operations that affect user-visible state.
- [ ] Remaining `//nolint:errcheck` annotations have justifying comments.

---

## SEC-009 · No Input Length Limits on Free-Text Fields (MEDIUM)

**OWASP**: A03 Injection / A04 Insecure Design
**Location**: `runs.go` — `CreateRun()`, `routes.go` — `LogEncounter()`, `coach.go` — `QueryCoach()`

### Problem

- `user_name` and `run_name` are length-validated (1–50 / 1–100 chars) in `CreateRun`, but:
- `speciesName` (form POST `form_id` text) in `LogEncounter` has no length limit.
- `question` in `QueryCoach` has no length limit — forwarded to the external ZeroClaw service, enabling:
  - Excessive memory allocation on the server
  - Prompt injection / token abuse on the LLM gateway
  - Denial-of-service via oversized payloads

### Required Actions

1. Add `Gin.MaxMultipartMemory` or use `http.MaxBytesReader` to cap total request body size (e.g. 64 KB).
2. Validate `question` length in `QueryCoach` (e.g. max 2000 characters).
3. Validate `speciesName` length in `LogEncounter` (e.g. max 100 characters).

### Acceptance Criteria

- [ ] No text field exceeds its documented max length.
- [ ] Oversized inputs return HTTP 400 with a user-friendly message.

---

## SEC-010 · Trusted Proxies Not Configured (MEDIUM)

**OWASP**: A05 Security Misconfiguration
**Location**: `cmd/pokemonprofessor/main.go`

### Problem

Gin's default trusts all proxies for `X-Forwarded-For` / `X-Real-IP` header parsing. If the application is behind a reverse proxy (nginx, Caddy), an attacker can spoof their client IP by injecting these headers. Gin prints a warning about this on startup in debug mode.

### Required Actions

1. Call `r.SetTrustedProxies([]string{"127.0.0.1"})` (or the specific reverse proxy IP).
2. If no reverse proxy is used, call `r.SetTrustedProxies(nil)` to disable header trust entirely.

### Acceptance Criteria

- [ ] `SetTrustedProxies` is called before route registration.
- [ ] `ClientIP()` returns the correct address in the deployment topology.

---

## SEC-011 · Race Conditions in Background Goroutines (MEDIUM)

**OWASP**: A04 Insecure Design
**Location**: `progress.go` — lines ~47, ~70; `runs.go` — line ~185

### Problem

Multiple handlers launch background goroutines (`go func(...)`) that write to the database concurrently:

- `ShowProgress`: `go pokeClient.EnsureRegionLocations(db, rid)`
- `ShowProgress`: `go pokeClient.EnsureAllEncounters(db, run.VersionID)`
- `UpdateProgress`: `go pokeClient.EnsureLocationEncounters(db, locID, versionID)`
- `CreateRun`: `go pokeClient.EnsurePokemon(db, starterFormID, versionGroupID)`

The `pokeapi.Client` has a `writeMu sync.Mutex` for serialization, but:
1. The goroutines capture the `db` handle from the handler closure. If the server shuts down while goroutines are active, database writes occur after the `db.Close()` in `main()`.
2. Multiple page refreshes can spawn duplicate goroutines seeding the same data concurrently.

### Required Actions

1. Use a `sync.WaitGroup` or `context.Context` with cancellation to track background goroutines and wait for them on shutdown.
2. Add a deduplication mechanism (e.g. `sync.Once` per version/region, or a `sync.Map` of in-flight operations) to prevent duplicate seeding goroutines.
3. Pass a `context.Context` from the request to the goroutine so background work can be cancelled on shutdown.

### Acceptance Criteria

- [ ] Graceful shutdown waits for in-flight background goroutines.
- [ ] Duplicate goroutines for the same seeding operation are prevented.

---

## SEC-012 · No Rate Limiting (LOW)

**OWASP**: A04 Insecure Design
**Location**: All endpoints

### Problem

No rate limiting exists on any endpoint. While currently LAN-only, this enables:
- Abuse of the `/api/legal/*` endpoints to trigger excessive PokeAPI fetches.
- Abuse of `POST /runs/:run_id/coach` to flood the ZeroClaw LLM gateway.
- Trivial DoS by spamming `POST /runs` to create thousands of runs.

### Required Actions

1. Add rate limiting middleware (e.g. `gin-contrib/limiter` or a simple token bucket) scoped per client IP.
2. Apply stricter limits to expensive endpoints: `/coach` (LLM calls), `/progress` POST (triggers PokeAPI seeding).

### Acceptance Criteria

- [ ] Default rate limit of 60 requests/minute per IP across all endpoints.
- [ ] Coach endpoint limited to 5 requests/minute per IP.

---

## SEC-013 · Database Path Not Validated (LOW)

**OWASP**: A01 Broken Access Control
**Location**: `internal/db/db.go` — `Open()` at line ~63; `cmd/pokemonprofessor/main.go` at line ~28

### Problem

`POKEMON_DB_PATH` is read from the environment and used directly in `os.MkdirAll` and `sql.Open` without validation. If an attacker controls the environment, they can write the SQLite database to an arbitrary path (e.g. overwriting system files, writing to `/tmp` for exfiltration).

### Required Actions

1. Validate `POKEMON_DB_PATH` at startup: ensure it resolves to an expected directory (e.g. under `/data/` or the working directory).
2. Use `filepath.Clean()` and reject paths containing `..`.

### Acceptance Criteria

- [ ] Paths with `..` or absolute paths outside an allowed prefix are rejected at startup.

---

## SEC-014 · Seeds File Read from Uncontrolled Path (LOW)

**OWASP**: A08 Software and Data Integrity Failures
**Location**: `internal/db/seeds.go` — `ApplySeedsIfEmpty()` at line ~61

### Problem

`ApplySeedsIfEmpty` reads a `seeds.sql` file from a path derived from the database path. The SQL content is executed directly via `db.Exec()`. If an attacker can place a malicious `seeds.sql` file adjacent to the database (e.g. in a shared directory), they can execute arbitrary SQL on startup.

### Required Actions

1. Prefer the embedded `bundled` seeds data over the filesystem file by default.
2. If a filesystem seeds file is supported, validate its integrity (e.g. checksum) or require an explicit opt-in flag.
3. Log a warning when using a filesystem seeds file.

### Acceptance Criteria

- [ ] Default behavior uses the embedded seeds.
- [ ] Filesystem seeds file requires explicit configuration to be trusted.

---

## SEC-015 · No Graceful Shutdown (LOW)

**OWASP**: A05 Security Misconfiguration
**Location**: `cmd/pokemonprofessor/main.go`

### Problem

The server does not handle `SIGTERM`/`SIGINT` for graceful shutdown. On container stop (Docker) or systemd `stop`, in-flight requests are terminated abruptly, potentially leaving the SQLite database in a dirty state (possible WAL corruption on forced termination).

### Required Actions

1. Replace `r.Run()` with a manual `http.Server` and `server.Shutdown(ctx)` pattern.
2. Listen for `os.Signal` (`syscall.SIGTERM`, `syscall.SIGINT`) and initiate graceful shutdown.
3. Wait for background goroutines (SEC-011) before closing the database.

### Acceptance Criteria

- [ ] `SIGTERM` triggers a graceful drain with a configurable timeout (default 10s).
- [ ] Database is closed only after all in-flight requests and background goroutines complete.

---

## SEC-016 · Stale Dependencies (LOW)

**OWASP**: A06 Vulnerable and Outdated Components
**Location**: `go.mod`

### Problem

The module uses Go 1.22. While not EOL at time of writing, dependency versions should be audited regularly. `modernc.org/sqlite v1.29.9` and `gin v1.10.0` should be checked against known CVE databases.

### Required Actions

1. Run `govulncheck ./...` in CI to check for known vulnerabilities.
2. Run `go list -m -u all` to identify available updates.
3. Add `govulncheck` to the CI pipeline (or pre-push hook).
4. Pin Go version in `go.mod` and Dockerfile to a supported release.

### Acceptance Criteria

- [ ] `govulncheck` runs in CI and blocks merges on findings.
- [ ] Dependencies are updated to the latest patch versions.

---

## SEC-017 · Container Runs as Implicit Root (LOW)

**OWASP**: A05 Security Misconfiguration
**Location**: `Dockerfile`

### Problem

The final Docker stage is `FROM scratch` with no `USER` directive. The process runs as UID 0 (root) inside the container. While `scratch` has no shell or OS tools (limiting blast radius), running as root is a defense-in-depth violation.

### Required Actions

1. Create a non-root user in the builder stage and copy the `/etc/passwd` entry to the scratch image.
2. Add a `USER` directive before `ENTRYPOINT`.

```dockerfile
# In builder stage:
RUN adduser -D -u 10001 appuser

# In scratch stage:
COPY --from=builder /etc/passwd /etc/passwd
USER appuser
```

### Acceptance Criteria

- [ ] Container process runs as a non-root UID.
- [ ] Database volume mount is writable by the container user.

---

## SEC-018 · Coach Answer Rendered Without Sanitization Review (LOW)

**OWASP**: A03 Injection (XSS)
**Location**: `templates/coach.html` (renders `CoachAnswer.Text`), `internal/handlers/coach.go`

### Problem

The `CoachAnswer.Text` field contains free-text output from the ZeroClaw LLM. Go's `html/template` auto-escapes by default, so this is **currently safe** as long as templates use `{{.CoachAnswer.Text}}` and not `{{.CoachAnswer.Text | safeHTML}}` or similar unescaping functions. However, this is a trust boundary that should be documented and monitored — any future use of `template.HTML` to render rich coach output would introduce stored XSS.

### Required Actions

1. Audit all templates to confirm no use of `template.HTML`, `safeHTML`, or similar unescaping for user-controllable or LLM-generated content.
2. Add a code comment at the `CoachAnswer` struct and in `coach.html` marking this as a security-sensitive boundary.
3. If rich rendering (Markdown) is ever needed, use a sanitizing Markdown renderer (e.g. `bluemonday`).

### Acceptance Criteria

- [ ] No template uses `template.HTML` or equivalent for LLM-generated content.
- [ ] Security boundary is documented in code comments.

---

## Summary Matrix

| ID | Severity | OWASP | Title | Effort |
|----|----------|-------|-------|--------|
| SEC-001 | Critical | A01 | No Authentication or Authorization | Large |
| SEC-002 | Critical | A01 | No CSRF Protection | Medium |
| SEC-003 | High | A05 | Missing Security Headers | Small |
| SEC-004 | High | A03 | SQL Injection via String Formatting | Small |
| SEC-005 | High | A10 | SSRF Risk in ZeroClaw Client | Small |
| SEC-006 | Medium | A04 | Error Messages Expose Internal Details | Small |
| SEC-007 | Medium | A05 | Gin Debug Mode in Production | Small |
| SEC-008 | Medium | A04 | Unchecked Errors on State-Mutating Ops | Medium |
| SEC-009 | Medium | A03 | No Input Length Limits on Free-Text Fields | Small |
| SEC-010 | Medium | A05 | Trusted Proxies Not Configured | Small |
| SEC-011 | Medium | A04 | Race Conditions in Background Goroutines | Medium |
| SEC-012 | Low | A04 | No Rate Limiting | Medium |
| SEC-013 | Low | A01 | Database Path Not Validated | Small |
| SEC-014 | Low | A08 | Seeds File Read from Uncontrolled Path | Small |
| SEC-015 | Low | A05 | No Graceful Shutdown | Medium |
| SEC-016 | Low | A06 | Stale Dependencies | Small |
| SEC-017 | Low | A05 | Container Runs as Implicit Root | Small |
| SEC-018 | Low | A03 | Coach Answer Rendered Without Sanitization Review | Small |

### Recommended Implementation Order

1. **SEC-003** — Security headers (quick win, no breaking changes)
2. **SEC-007** — Gin release mode (one-line fix)
3. **SEC-010** — Trusted proxies (one-line fix)
4. **SEC-004** — SQL formatting guards (small, targeted)
5. **SEC-005** — ZeroClaw URL validation (small, targeted)
6. **SEC-006** — Error message sanitization (small)
7. **SEC-009** — Input length limits (small)
8. **SEC-017** — Dockerfile non-root user (small)
9. **SEC-002** — CSRF protection (medium, requires template changes)
10. **SEC-001** — Authentication & authorization (large, foundational)
11. **SEC-008** — Error check audit (medium, file-by-file)
12. **SEC-011** — Goroutine lifecycle management (medium)
13. **SEC-015** — Graceful shutdown (medium)
14. **SEC-012** — Rate limiting (medium)
15. **SEC-013** — DB path validation (small)
16. **SEC-014** — Seeds file trust model (small)
17. **SEC-016** — Dependency audit pipeline (small)
18. **SEC-018** — Template sanitization audit (small)

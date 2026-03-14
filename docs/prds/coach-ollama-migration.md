# PokemonProfessor — AI Coach: Migrate to Direct Ollama Client

**Status**: Ready for implementation
**Priority**: High
**Date**: 2026-03-14

Cross-reference: [architecture.md](architecture.md) for module structure,
[coach-enrichment.md](coach-enrichment.md) for payload schema,
[api.md](api.md) for route inventory.

---

## Executive Summary

The current AI Coach integration targets a custom ZeroClaw gateway API
(`POST /agent/:agent/query`) that does not exist in the real ZeroClaw project. The
service in `internal/services/zeroclaw.go` has never been able to reach a live LLM.

This PRD replaces the non-functional ZeroClaw transport with a direct Ollama HTTP
client (`CoachClient`). The enriched coach payload, graceful-degradation pattern,
and all business logic are unchanged. Only the transport layer, env var names, and a
handful of type/field references are updated.

Ollama LXC provisioning is out of scope for this repo and is handled by the proxmox
repo (`playbooks/`).

### Architecture Decision

| Concern | Decision | Rationale |
|---|---|---|
| Client type name | `CoachClient` | Provider-agnostic; easy to swap backend |
| Env vars | `COACH_HOST`, `COACH_MODEL`, `COACH_SYSTEM_PROMPT` | Provider-agnostic; set by Ansible in proxmox repo |
| System prompt | `COACH_SYSTEM_PROMPT` env var | Decouples persona config from binary; managed at deploy time |
| Default model | `qwen2.5:3b` | Best reasoning/VRAM trade-off for GTX 970 shared with Immich |
| VRAM eviction | `keep_alive: 0` hardcoded in every request | GPU is shared; model must be evicted after each query |
| ZeroClaw agent fields | Removed | No longer applicable; removes `validAgent` regex and `ZEROCLAW_AGENT` |
| `buildCoachPayload` | Unchanged | All enrichment logic already in Go; no migration needed |

---

## COACH-007 · Direct Ollama Transport (HIGH)

**Location**: `internal/services/coach.go` (replaces `zeroclaw.go`)

### Problem

`internal/services/zeroclaw.go` calls `POST /agent/:agent/query` — an endpoint that does
not exist in ZeroClaw v0.1.x. The `ZEROCLAW_AGENT` env var introduces a path-segment
that requires sanitisation but provides no functional value. The ZeroClaw daemon would
add a second process to maintain on LXC 130 with no benefit over talking directly to
Ollama.

### Required Actions

1. Delete `internal/services/zeroclaw.go` and `internal/services/zeroclaw_test.go`.

2. Create `internal/services/coach.go` with the following public surface:

```go
// CoachClient is a client for the Ollama /api/chat endpoint.
// All methods degrade gracefully — callers should check IsAvailable() before
// calling QueryCoach, but QueryCoach will also return a safe response on failure.
// Empty host = disabled.
type CoachClient struct {
    host         string
    model        string
    systemPrompt string
    http         *http.Client
}

func NewCoachClient(host, model, systemPrompt string) *CoachClient

// ValidateConfig returns an error if host is set but malformed (bad scheme, no host).
// Empty host is valid (disabled).
func (c *CoachClient) ValidateConfig() error

// IsAvailable returns true if GET {host}/ returns 200.
func (c *CoachClient) IsAvailable() bool

// QueryCoach posts to Ollama /api/chat with keep_alive=0 and returns the response.
// On any failure, returns CoachResponse{Available: false} — never errors.
func (c *CoachClient) QueryCoach(runID int, payload CoachPayload) CoachResponse
```

3. `CoachPayload`, `CoachCandidates`, and `CoachResponse` struct definitions move
   from `zeroclaw.go` into `coach.go` unchanged.

4. `QueryCoach` must send `keep_alive: 0` in every request body. This is not
   configurable — it is the only correct behaviour when the GPU is shared with
   Immich (GTX 970, ~3.5 GB usable VRAM).

5. The Ollama request body shape:

```json
{
  "model": "<COACH_MODEL>",
  "stream": false,
  "keep_alive": 0,
  "messages": [
    { "role": "system", "content": "<COACH_SYSTEM_PROMPT>" },
    { "role": "user",   "content": "<formatPayload output>" }
  ]
}
```

The system message is omitted when `COACH_SYSTEM_PROMPT` is empty.

6. Implement `formatPayload(p CoachPayload) string` — a private helper that
   serialises the enriched `CoachPayload` fields into a compact, human-readable
   prompt string that is sent as the user message.

7. Map Ollama's `done_reason == "length"` to `CoachResponse.Truncated = true`.

8. The Ollama response shape consumed:

```json
{
  "model": "qwen2.5:3b",
  "message": { "role": "assistant", "content": "..." },
  "done": true,
  "done_reason": "stop"
}
```

### Acceptance Criteria

- [ ] `go build ./...` succeeds with no reference to `ZeroClaw` or `zeroclaw`.
- [ ] `IsAvailable()` returns `false` when `host` is empty.
- [ ] `IsAvailable()` returns `false` when `GET {host}/` does not return 200.
- [ ] `QueryCoach` request body always contains `"keep_alive": 0`.
- [ ] `QueryCoach` omits the system message when `systemPrompt` is `""`.
- [ ] `QueryCoach` returns `CoachResponse{Available: false}` on any network or parse error.
- [ ] `CoachResponse.Truncated` is `true` when `done_reason == "length"`.
- [ ] `ValidateConfig` rejects non-http/https schemes and empty host when host is set.
- [ ] `ValidateConfig` accepts empty host (disabled state).

---

## COACH-008 · Handler and Page Type Updates (HIGH)

**Location**: `internal/handlers/`

### Problem

Four handler files reference `*services.ZeroClaw` and/or the `ZeroClawAvailable` page
field. These are mechanical renames — no logic changes.

### Required Actions

**`internal/handlers/coach.go`**

| Location | Before | After |
|---|---|---|
| `ShowCoach` signature | `zc *services.ZeroClaw` | `zc *services.CoachClient` |
| `QueryCoach` signature | `zc *services.ZeroClaw` | `zc *services.CoachClient` |
| page assignment (coach unavailable) | `page.ZeroClawAvailable = false` | `page.CoachAvailable = false` |
| page literal in `buildCoachPage` | `ZeroClawAvailable: available` | `CoachAvailable: available` |
| comment on `buildCoachPayload` | `"for ZeroClaw (COACH-006)"` | `"for AI Coach (COACH-006)"` |

**`internal/handlers/api.go`**

| Location | Before | After |
|---|---|---|
| `Health` signature | `zc *services.ZeroClaw` | `zc *services.CoachClient` |
| health JSON key | `"zeroclaw"` | `"coach"` |

**`internal/handlers/pages.go`**

| Location | Before | After |
|---|---|---|
| `OverviewPage.ZeroClawAvailable` | `ZeroClawAvailable bool` | `CoachAvailable bool` |
| `CoachPage.ZeroClawAvailable` | `ZeroClawAvailable bool` | `CoachAvailable bool` |
| SEC-018 comment | `"ZeroClaw LLM gateway"` | `"AI Coach LLM host"` |

**`internal/handlers/runs.go`**

| Location | Before | After |
|---|---|---|
| `ShowOverview` signature | `zc *services.ZeroClaw` | `zc *services.CoachClient` |
| page literal field | `ZeroClawAvailable: zc.IsAvailable()` | `CoachAvailable: zc.IsAvailable()` |

### Acceptance Criteria

- [ ] No reference to `ZeroClawAvailable` remains in any `.go` file.
- [ ] `go vet ./internal/handlers/...` is clean.
- [ ] Existing handler tests pass unchanged.

---

## COACH-009 · Entry Point Env Vars (HIGH)

**Location**: `cmd/professor-arbortom/main.go`

### Problem

`main.go` reads `ZEROCLAW_GATEWAY` and `ZEROCLAW_AGENT` and passes them to
`services.NewZeroClaw`. These must be replaced with the provider-agnostic vars read by
the Ansible playbook in the proxmox repo.

### Required Actions

1. Replace env var reads:

```go
// Before
zcGateway := os.Getenv("ZEROCLAW_GATEWAY")
zcAgent   := os.Getenv("ZEROCLAW_AGENT")
zc := services.NewZeroClaw(zcGateway, zcAgent)

// After
coachHost   := os.Getenv("COACH_HOST")
coachModel  := os.Getenv("COACH_MODEL")
if coachHost != "" && coachModel == "" {
    coachModel = "qwen2.5:3b"
}
coachPrompt := os.Getenv("COACH_SYSTEM_PROMPT")
zc := services.NewCoachClient(coachHost, coachModel, coachPrompt)
```

2. Update the `ValidateConfig` call comment from `"SEC-005: Validate ZeroClaw config"` to
   `"SEC-005: Validate AI Coach config"`.

### Acceptance Criteria

- [ ] `ZEROCLAW_GATEWAY` and `ZEROCLAW_AGENT` are not read anywhere in the binary.
- [ ] Binary starts with no env vars set (`COACH_HOST` empty) — logs no fatal errors.
- [ ] `COACH_MODEL` defaults to `"qwen2.5:3b"` when `COACH_HOST` is set and `COACH_MODEL` is empty.

---

## COACH-010 · Template Field Updates (MEDIUM)

**Location**: `templates/`

### Problem

Two templates reference the renamed `ZeroClawAvailable` page field, and the coach
placeholder banner tells the user to set `ZEROCLAW_GATEWAY`.

### Required Actions

**`templates/coach.html`**

1. `{{if .ZeroClawAvailable}}` → `{{if .CoachAvailable}}`
2. Placeholder banner text: replace `ZEROCLAW_GATEWAY` with `COACH_HOST`.

**`templates/overview.html`**

1. `{{if .ZeroClawAvailable}}` → `{{if .CoachAvailable}}`

### Acceptance Criteria

- [ ] No reference to `ZeroClawAvailable` or `ZEROCLAW_GATEWAY` remains in any `.html` file.
- [ ] Coach panel renders the placeholder banner when `COACH_HOST` is unset.
- [ ] Coach panel renders the query form when `COACH_HOST` is set and Ollama is reachable.

---

## COACH-011 · Service Tests (HIGH)

**Location**: `internal/services/coach_test.go` (replaces `zeroclaw_test.go`)

### Required Actions

1. Rewrite the test file against the new `CoachClient` and Ollama API shapes.
2. Mock server handles `POST /api/chat` with Ollama response shape.
3. Tests to carry over (updated for new types):
   - `TestIsAvailable_NoHost`
   - `TestIsAvailable_ServiceUp`
   - `TestIsAvailable_ServiceReturns500`
   - `TestQueryCoach_NoHost`
   - `TestQueryCoach_Success`
   - `TestQueryCoach_ServerError`
   - `TestQueryCoach_MalformedResponse`
   - `TestValidateConfig_EmptyHost`
   - `TestValidateConfig_ValidHTTP`
   - `TestValidateConfig_ValidHTTPS`
   - `TestValidateConfig_BadScheme`
   - `TestValidateConfig_NoHost`
4. Tests to add:
   - `TestQueryCoach_KeepAliveIsZero` — assert request body contains `"keep_alive":0`
   - `TestQueryCoach_Truncated` — `done_reason: "length"` → `Truncated: true`
   - `TestQueryCoach_NoSystemPromptWhenEmpty` — omits system message when `systemPrompt == ""`
5. Tests to remove:
   - `TestValidateConfig_AgentPathTraversal` — no agent field exists on `CoachClient`

### Acceptance Criteria

- [ ] `go test ./internal/services/... -v` — all tests pass.
- [ ] `go test -race ./internal/services/...` — no races.

---

## Out of Scope

- Ollama LXC provisioning — handled in proxmox repo `playbooks/`
- Model selection or quantisation — `qwen2.5:3b Q4_K_M` is the recommended default; the
  proxmox Ansible playbook sets `COACH_MODEL` and runs `ollama pull`
- ZeroClaw installation or configuration — not used by this repo
- Any changes to `buildCoachPayload`, `buildTeamInsights`, the legality engine, or any
  other business logic

---

## File Change Summary

| File | Action |
|---|---|
| `internal/services/zeroclaw.go` | Delete |
| `internal/services/zeroclaw_test.go` | Delete |
| `internal/services/coach.go` | Create (COACH-007) |
| `internal/services/coach_test.go` | Create (COACH-011) |
| `internal/handlers/coach.go` | Edit — type refs + field names (COACH-008) |
| `internal/handlers/api.go` | Edit — type ref + JSON key (COACH-008) |
| `internal/handlers/pages.go` | Edit — field rename (COACH-008) |
| `internal/handlers/runs.go` | Edit — type ref + field name (COACH-008) |
| `cmd/professor-arbortom/main.go` | Edit — env vars + constructor (COACH-009) |
| `templates/coach.html` | Edit — field + env var text (COACH-010) |
| `templates/overview.html` | Edit — field name (COACH-010) |

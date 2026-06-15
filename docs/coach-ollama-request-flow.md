# Coach → Ollama Request Flow

End-to-end sequence showing how a browser request becomes an Ollama API call, including all pre-processing steps.

```mermaid
sequenceDiagram
    participant Browser
    participant Handler as handlers/coach.go
    participant DB as SQLite DB
    participant Legality as legality engine
    participant Service as services/coach.go
    participant Ollama

    Browser->>Handler: GET/POST /runs/:id/coach[/recommendation]

    Note over Handler: SEC-009: Reject if len(question) > 2000

    Handler->>Handler: IsAvailable() — GET {host}/ == 200?

    Handler->>DB: buildCoachPage()
    activate Handler
        DB-->>Handler: acquisitions (LegalAcquisitions)
        Handler->>Handler: CD-001 dedup by (species,form,location,method)<br/>merge level ranges
        DB-->>Handler: trades (LegalTrades)
        DB-->>Handler: owned + shop items (LegalItems + ShopItems)
        DB-->>Handler: party slots (form_id, level, party_slot)
        DB-->>Handler: version name, badge count, current location
        Handler->>Legality: team insights, coverage, evo paths
        Legality-->>Handler: TeamInsights (weaknesses/resistances/immunities/uncoveredTypes)
        DB-->>Handler: next opponents (gym leaders / rivals)
        DB-->>Handler: active run rules (LoadRunState)
        Handler->>DB: trigger background evo-target seeding if learnset missing
    deactivate Handler

    Handler->>Handler: buildCoachPayload()
    activate Handler
        Note over Handler: contextNote = badge/level access hint
        Handler->>Handler: buildGameSummary()<br/>(party, items, type analysis, evo paths, opponents)
        Handler->>Handler: buildWalkthroughContext()<br/>(walkthrough badge section for version)
        Handler->>Handler: buildPreComputedRecommendations()<br/>(best TM upgrade, evo opportunities,<br/>catches vs next opponent, HM coverage)
        Handler->>Handler: wrapUserQuestion(q) OR defaultRecommendationPrompt
    deactivate Handler

    Handler->>Service: QueryCoach(runID, CoachPayload)
    activate Service
        Service->>Service: formatContext(payload)<br/>contextNote + GameSummary (or JSON candidates)
        Service->>Service: sha256(runID + context + question) → cacheKey
        alt cache hit (TTL 3 min)
            Service-->>Handler: cached CoachResponse
        else cache miss
            Service->>Service: Build messages[]<br/>[system: systemPrompt]<br/>[user: game state context]<br/>[assistant: acknowledgement]<br/>[user: question]
            Service->>Service: JSON marshal {model, stream:false, think:false,<br/>keep_alive:0, messages}
            Service->>Ollama: POST /api/chat
            Ollama-->>Service: JSON response (max 8 MB)
            Service->>Service: stripThinking() — remove <think>...</think>
            Service->>Service: Store in cache (expire in 3 min)
            Service-->>Handler: CoachResponse{Available, Answer, Model, Truncated}
        end
    deactivate Service

    Handler-->>Browser: coach.html / coach-recommendation.html fragment
```

## Pre-processing summary

### Stage 1 — `buildCoachPage` (DB queries)
- Loads legal acquisitions → deduplicates by `(species, form, location, method)` merging level ranges (CD-001)
- Loads trades, owned items + shop items (sorted by source/category)
- Loads party slots with form/level, version name, badge count, current location
- Runs legality engine for type coverage, weaknesses/resistances, evolution paths
- Loads next opponents and active run rules
- Triggers background seeding for any evo-target missing learnset data

### Stage 2 — `buildCoachPayload` (prompt assembly)
- `buildGameSummary` — structured text of party, items, type analysis, evo paths, opponents
- `buildWalkthroughContext` — pulls the relevant walkthrough section for the player's version + badge progress
- `buildPreComputedRecommendations` — server-side verified facts (best TM upgrade, evo opportunities, optimal catches vs next opponent) so the LLM presents data, not hallucinations
- Wraps user question via `wrapUserQuestion()` or uses `defaultRecommendationPrompt`

### Stage 3 — `QueryCoach` (cache + send)
- `formatContext` serializes context note + GameSummary (or JSON fallback)
- SHA-256 cache check (3-min TTL, max 256 entries)
- Builds 4-turn message array: `system → user (game state) → assistant (ack) → user (question)`
- Sends with `keep_alive:0`, `stream:false`, `think:false`
- Response stripped of `<think>...</think>` blocks, capped at 8 MB

# PokemonProfessor — GUI Specification

**Rendering**: Go `html/template` stdlib, embedded via `//go:embed templates/*`
**Styling**: Single `static/style.css`, embedded via `//go:embed static/style.css`
**No JavaScript framework.** No build step. Forms use standard HTML POST.

Cross-reference: [api.md](api.md) for handler data flow, [schema.md](schema.md) for data types.

---

## Template Inheritance Pattern

Go's `html/template` does not support Jinja2-style `extends`. Use the following composition pattern:

All templates are parsed together as one set via `template.ParseGlob("templates/*.html")`.
`base.html` defines named blocks. Each screen template invokes `{{template "base" .}}` and
fills blocks via `{{define "content"}}`.

### `base.html` structure

```html
{{define "base"}}
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>PokemonProfessor — {{.PageTitle}}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  {{template "nav" .}}
  {{template "run-banner" .}}
  {{template "flash" .}}
  <main>
    {{template "content" .}}
  </main>
</body>
</html>
{{end}}
```

### `nav` partial (defined in `base.html`)

Top navigation bar. Links render as active if `{{.ActiveNav}}` matches:

```
Runs | [Run Name] Progress · Team · Box · Routes · Rules · Coach
```

- **Left side**: `Runs` link always visible → `/runs`
- **Right side**: only shown when a run is in context (`.RunContext` is non-nil)
  - Each link: `/runs/{{.RunContext.ID}}/progress` etc.
- Active link gets CSS class `nav-active`

### `run-banner` partial (defined in `base.html`)

Shown only when `.RunContext` is non-nil. Displays on every screen within a run:

```
[Run Name]  Version: FireRed  Badges: ████░░░░ (3/8)  Rules: NUZLOCKE LEVEL-CAP
```

- Badge count rendered as pip icons (filled/empty) — pure CSS, no JS
- Active rules shown as pill badges; inactive rules are not shown

### `flash` partial (defined in `base.html`)

Shows `.Flash.Message` if non-empty, with class `flash-success` or `flash-error` per `.Flash.Type`.
Dismisses on next page load (cookie cleared after render).

---

## Template Data Structs

Every handler constructs a page-specific struct and passes it to `c.HTML`. All structs embed
`BasePage`:

```go
type BasePage struct {
    PageTitle  string
    ActiveNav  string      // "runs", "progress", "team", "box", "routes", "rules", "coach"
    RunContext *RunContext  // nil on /runs list page
    Flash      *Flash
}

type RunContext struct {
    ID          int
    Name        string
    VersionName string
    BadgeCount  int
    ActiveRules []string   // rule keys where enabled=true, e.g. ["nuzlocke", "level_cap"]
    BadgePips   []bool     // true=filled pip; always 8 elements; pre-computed in handler
}

type Flash struct {
    Type    string  // "success" | "error" | "warning"
    Message string
}
```

---

## Screen Specifications

---

### 1. Run Dashboard — `runs.html`

**URL**: `GET /runs`
**Handler data struct**:

```go
type RunsPage struct {
    BasePage
    Runs              []RunSummary
    ArchivedRuns      []RunSummary
    Versions          []VersionOption
    StartersByVersion map[int][]StarterOption
}

type VersionOption struct {
    ID   int
    Name string // display name, e.g. "FireRed"
}

type StarterOption struct {
    FormID      int
    SpeciesName string // capitalized, e.g. "Bulbasaur"
}

type RunSummary struct {
    ID          int
    Name        string
    UserName    string
    VersionName string
    BadgeCount  int
    ActiveRules []string
    UpdatedAt   string
    Archived    bool
}
```

**Layout**:
- Page heading: "Your Runs"
- Table with columns: Run Name, User, Game, Badges, Rules, Last Updated, Actions
- Actions column: "Continue" → `/runs/{id}/progress`
- Below table: "New Run" form (inline, collapsible or always visible):
  - `user_name` text input (label: "Your name")
  - `run_name` text input (label: "Run name")
  - `version_id` select (options from `game_version` table, display name capitalized: "FireRed", "LeafGreen", etc.)
  - Submit button: "Start Run"
- Empty state: "No runs yet. Start one above."

---

### 2. Progress Tracker — `progress.html`

**URL**: `GET /runs/:run_id/progress`
**Handler data struct**:

```go
type ProgressPage struct {
    BasePage
    Locations        []Location      // filtered to run's version_id
    CurrentLocID     *int
    BadgeCount       int
    AllFlags         []FlagDef       // all known flags for this version
    ActiveFlags      map[string]bool // run_flag rows: key → true
    LocationsSeeding bool            // true when PokeAPI region location seed is running
    HydrationTotal   int             // total location areas for this version
    HydrationSeeded  int             // how many have encounter data in api_cache_log
}

type FlagDef struct {
    Key         string
    Label       string   // human-readable: "Got HM Cut", "Defeated Gym 1"
    Description string
}
```

**Layout**:
- Badge count stepper: row of 8 badge icons; click to set count (POST form with `badge_count`)
- Location selector: `<select>` of all locations for this version, grouped by region (optgroup)
- Story flags: grouped checkbox list (HM flags, story milestones, DLC gates)
  - Each checkbox: `name="flags"` `value="{key}"` — checked if in `ActiveFlags`
- "Save Progress" submit button

**Validation errors**: inline below each field that failed.

---

### 3. Team Builder — `team.html`

**URL**: `GET /runs/:run_id/team`
**Handler data struct**:

```go
type TeamPage struct {
    BasePage
    Slots          [6]PartySlot
    LegalForms     []FormOption     // from LegalAcquisitions
    LegalItems     []ItemOption     // from LegalItems
    LegalityErrors map[string]string // field → reason, populated on POST failure
}

// TeamSlotPage is used by the per-slot edit form (GET /runs/:id/team/:slot).
type TeamSlotPage struct {
    BasePage
    SlotNum        int
    Slot           PartySlot
    LegalForms     []FormOption
    LegalItems     []ItemOption
    LegalityErrors map[string]string
}

type PartySlot struct {
    Slot        int
    FormID      *int
    FormName    string
    SpeciesName string
    Level       *int
    MoveIDs     [4]*int
    MoveNames   [4]string
    HeldItemID  *int
    HeldItemName string
    LegalMoves  []MoveOption       // from LegalMoves(form_id) — populated if FormID set
}

type FormOption struct {
    ID          int
    SpeciesName string
    FormName    string
    LocationName string
    BlockedByRule string  // "" if available, "nuzlocke"/"level_cap" if blocked
}

type MoveOption struct {
    ID            int
    Name          string
    TypeName      string
    LearnMethod   string
    Level         int
    EvoNote       string // e.g. "Learns on evolution"
    TMNumber      int    // >0 if learned via TM
    HMNumber      int    // >0 if learned via HM
    TutorLocation string // non-empty if taught by a move tutor
}

type ItemOption struct {
    ID       int
    Name     string
    Category string
    Source   string  // "owned" | "obtainable"
}
```

**Layout**:
- 6-slot grid (2 columns × 3 rows on wide screens; 1 column on narrow)
- Each slot card:
  - Species select: `legal_acquisitions` result; blocked options shown but visually dimmed with
    rule badge tooltip; empty option = "Empty slot"
  - Level input: number 1–100
  - 4 move selects: `legal_moves` for the selected form; only shown if species selected
  - Held item select: `legal_items` result
- Legality violation display: red inline badge below the offending field
  - Example: `⚠ Surf is not yet obtainable — need HM Surf flag`
- "Save Team" submit button at bottom

**Note**: Species select changes do not trigger AJAX — full form POST re-renders with correct move
options for the new form selection. The submitted `form_id` determines which `LegalMoves` are
loaded for that slot on re-render.

---

### 4. Box Manager — `box.html`

**URL**: `GET /runs/:run_id/box`
**Handler data struct**:

```go
type BoxPage struct {
    BasePage
    Entries     []BoxEntry
    ShowFainted bool        // query param ?fainted=true
    NuzlockeOn  bool
}

type BoxEntry struct {
    ID           int
    FormID       int
    SpeciesName  string
    FormName     string
    Level        int
    MetLocation  string   // "" if starter/gift
    IsAlive      bool
    Evolutions   []Evolution // available evolutions shown inline; used for "Evolve" button
}
```

**Layout**:
- Filter toggle: "Show alive" / "Show all (including fainted)" — links with `?fainted=true` param
- Table: Pokémon, Form, Level, Met At, Status, Actions
- Status: green "Alive" pill | red "Fainted" pill
- Actions (Nuzlocke mode active):
  - Alive entry: "Mark Fainted" button (POST `/runs/:id/box/:entry_id/faint`)
  - Fainted entry: "Revive" button only shown if Nuzlocke rule is **disabled**
- Empty state: "No Pokémon in box yet."

---

### 5. Route Log — `routes.html`

**URL**: `GET /runs/:run_id/routes`
**Handler data struct**:

```go
type RoutesPage struct {
    BasePage
    Log            []RouteEntry
    Locations      []Location       // for the log form dropdown
    NuzlockeOn     bool
    DuplicateWarning *DuplicateWarning  // set on POST re-render if Nuzlocke duplicate
}

type RouteEntry struct {
    LocationName string
    SpeciesName  string
    Outcome      string   // "caught", "fainted", "fled", "skipped"
    Level        int
    IsDuplicate  bool     // Nuzlocke: another catch on same route
}

type DuplicateWarning struct {
    LocationName string
    PreviousCatch string
}
```

**Layout**:
- Log form (at top):
  - Location select (version-filtered)
  - Pokémon name text input (free text — not forced to legal set, player records what they saw)
  - Outcome select: `caught` | `fainted` | `fled` | `skipped`
  - Level number (shown when outcome = caught)
  - Submit: "Log Encounter"
- Nuzlocke duplicate warning banner (yellow, non-blocking):
  `"⚠ Nuzlocke: you already caught a Pokémon on [Route Name] ([Previous Pokémon]). Log anyway?"`
  — the form re-renders with the data pre-filled and a "Log Anyway" confirm button
- Log table below form: Location, Pokémon, Outcome, Level; Nuzlocke mode adds a "Duplicate" column
- Duplicate rows highlighted in amber

---

### 6. Rules Manager — `rules.html`

**URL**: `GET /runs/:run_id/rules`
**Handler data struct**:

```go
type RulesPage struct {
    BasePage
    Rules []RuleCard
}

type RuleCard struct {
    Key         string
    Label       string
    Description string
    Enabled     bool
    Params      map[string]interface{}  // parsed from params_json
}
```

**Layout**:
- One card per `rule_def` row
- Card content:
  - Rule label (bold) + description
  - Toggle checkbox: `name="rule_{key}"` checked if enabled
  - Params field (only for parameterized rules):
    - `level_cap`: number input labeled "Level cap", `name="rule_level_cap_params"`,
      placeholder "Leave blank to use badge-based caps"
    - `theme_run`: text input labeled "Theme description" (freeform, stored in params_json)
- "Save Rules" submit button
- Changes take effect immediately on all legality queries — no restart needed

---

### 7. Coaching Panel — `coach.html`

**URL**: `GET /runs/:run_id/coach`
**Handler data struct**:

```go
type CoachPage struct {
    BasePage
    ZeroClawAvailable  bool
    Acquisitions       []FormOption       // from LegalAcquisitions
    Trades             []TradeOption      // NPC trades + Game Corner at current location
    PartyMoves         []PartyMoveSummary // legal moves per party slot
    LegalItems         []ItemOption
    CoachAnswer        *CoachAnswer       // nil until POST submitted
    PlayerQuestion     string             // echoed back after POST
}

// TradeOption is an NPC trade or Game Corner entry at the run's current location.
type TradeOption struct {
    Method         string // "trade" | "game-corner"
    GiveSpecies    string // empty for game-corner entries
    ReceiveSpecies string
    ReceiveNick    string
    PriceCoins     int
    Notes          string
}

type PartyMoveSummary struct {
    Slot        int
    Level       int
    SpeciesName string
    Moves       []MoveOption
}

type CoachAnswer struct {
    Text      string
    Model     string   // e.g. "llama3:8b"
    Truncated bool     // if ZeroClaw hit max_iterations
}
```

**Layout — two states**:

#### State A: ZeroClaw available (`ZeroClawAvailable = true`)

```
[ Candidate Set ]                          [ Ask the Professor ]
──────────────────────────────────          ─────────────────────────────────────
Available Pokémon (N)                       Your question:
  [table: species, location, method]        [<textarea rows=4 name="question">]

Your Party Moves                            [Submit: "Ask Professor"]
  Slot 1 · Charizard
    [table: move, type, learn method]       [ Professor's Answer ]
  Slot 2 · Blastoise                        ─────────────────────────────────────
    ...                                     [{{.CoachAnswer.Text}}]

Legal Items (N)                             Model: {{.CoachAnswer.Model}}
  [table: item, category, source]
```

#### State B: ZeroClaw unavailable (`ZeroClawAvailable = false`)

Same candidate tables on the left, but the right panel is replaced by:

```
┌─────────────────────────────────────────────────────┐
│  AI coaching unavailable                            │
│                                                     │
│  Configure ZEROCLAW_GATEWAY in .env and restart     │
│  the service to enable the Professor AI coach.      │
│                                                     │
│  The legality data above is fully functional        │
│  without the AI coach.                              │
└─────────────────────────────────────────────────────┘
```

The gray placeholder box must not suggest the system is broken — it is an expected operating mode.

---

## CSS Conventions (`static/style.css`)

No CSS framework. Purpose-built, minimal.

### Type Color Classes

Each Pokémon type gets a CSS class `type-{typename}` with a background color:

```css
.type-normal   { background: #A8A878; color: #fff; }
.type-fire     { background: #F08030; color: #fff; }
.type-water    { background: #6890F0; color: #fff; }
.type-grass    { background: #78C850; color: #fff; }
.type-electric { background: #F8D030; color: #333; }
.type-ice      { background: #98D8D8; color: #333; }
.type-fighting { background: #C03028; color: #fff; }
.type-poison   { background: #A040A0; color: #fff; }
.type-ground   { background: #E0C068; color: #333; }
.type-flying   { background: #A890F0; color: #fff; }
.type-psychic  { background: #F85888; color: #fff; }
.type-bug      { background: #A8B820; color: #fff; }
.type-rock     { background: #B8A038; color: #fff; }
.type-ghost    { background: #705898; color: #fff; }
.type-dragon   { background: #7038F8; color: #fff; }
.type-dark     { background: #705848; color: #fff; }
.type-steel    { background: #B8B8D0; color: #333; }
```

### Legality Badge Classes

```css
.badge-blocked   { background: #e74c3c; color: #fff; border-radius: 4px; padding: 2px 6px; font-size: 0.75em; }
.badge-warning   { background: #f39c12; color: #fff; ... }
.badge-rule      { background: #8e44ad; color: #fff; ... }  /* active rule pills in banner */
.badge-available { background: #27ae60; color: #fff; ... }
```

### Nav Active State

```css
.nav-active { font-weight: bold; border-bottom: 2px solid currentColor; }
```

### Badge Pip Icons (progress tracker)

```css
.badge-pip        { display: inline-block; width: 20px; height: 20px; border-radius: 50%; ... }
.badge-pip-filled { background: #f1c40f; }
.badge-pip-empty  { background: #ddd; border: 1px solid #bbb; }
```

---

## Flash Message Pattern

Flash messages are set by handlers before redirecting and cleared after rendering on the next page.

```go
// In handler (before redirect):
session.Set("flash_type", "success")
session.Set("flash_message", "Progress saved.")
session.Save()

// In base.html:
{{if .Flash}}
<div class="flash flash-{{.Flash.Type}}">{{.Flash.Message}}</div>
{{end}}
```

Common flash messages:
- Progress saved → `"Progress updated."`
- Rule change → `"Run rules updated."`
- Team slot updated → `"Team saved."`
- Legality violation → error flash with specific reason (shown inline in form, not as flash)

// Package handlers contains Gin HTTP handlers and page data structs for
// PokemonProfessor. All template data structs embed BasePage.
package handlers

import "github.com/TravisPenn/professor-arbortom/internal/legality"

// ─── Base page types ──────────────────────────────────────────────────────────

// BasePage is embedded in every page struct.
type BasePage struct {
	PageTitle  string
	ActiveNav  string
	RunContext *RunContext
	Flash      *Flash
}

// RunContext carries the active run fields available on every run-scoped page.
type RunContext struct {
	ID              int
	Name            string
	VersionName     string
	BadgeCount      int
	CurrentLocation string   // human-readable location name, or ""
	ActiveRules     []string // only enabled rule keys
	BadgePips       []bool   // true=filled, length always 8
}

// Flash carries a single flash message to display after redirect.
type Flash struct {
	Type    string // "success" | "error" | "warning"
	Message string
}

// ─── Run Dashboard ────────────────────────────────────────────────────────────

type RunsPage struct {
	BasePage
	Runs              []RunSummary
	ArchivedRuns      []RunSummary
	Versions          []VersionOption
	StartersByVersion map[int][]StarterOption
	SelectedVersionID int // re-populates version select after a validation error
}

// StarterOption represents a choosable starter Pokémon for a game version.
type StarterOption struct {
	FormID      int    `json:"id"`
	SpeciesName string `json:"name"` // capitalized, e.g. "Bulbasaur"
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

type VersionOption struct {
	ID   int
	Name string // display name, e.g. "FireRed"
}

// ─── Progress Tracker ─────────────────────────────────────────────────────────

type ProgressPage struct {
	BasePage
	Locations        []LocationOption
	CurrentLocID     *int
	BadgeCount       int
	AllFlags         []FlagDef
	ActiveFlags      map[string]bool
	LocationsSeeding bool // true when location data is being fetched from PokeAPI
	HydrationTotal   int  // total location areas for this version
	HydrationSeeded  int  // how many have encounter data in api_cache_log
}

type LocationOption struct {
	ID     int
	Name   string
	Region string
}

type FlagDef struct {
	Key         string
	Label       string
	Description string
}

// ─── Team Builder ─────────────────────────────────────────────────────────────

type TeamPage struct {
	BasePage
	Slots          [6]PartySlot
	LegalForms     []FormOption
	LegalItems     []ItemOption
	LegalityErrors map[string]string
}

// MoveChip carries the data needed to render a move pill with a hover tooltip
// on the team overview page.
type MoveChip struct {
	Name        string
	DamageClass string // "physical", "special", "status", or ""
	Power       *int
	Accuracy    *int
	PP          int
	Effect      string
}

type PartySlot struct {
	Slot         int
	RunPokemonID int // run_pokemon.id of current occupant (0 = empty)
	FormID       *int
	FormName     string
	SpeciesName  string
	Level        *int
	MoveIDs      [4]*int
	MoveNames    [4]string
	Moves        [4]MoveChip // populated for overview tooltips
	HeldItemID   *int
	HeldItemName string
	LegalMoves   []MoveOption
}

type FormOption struct {
	ID            int
	RunPokemonID  int // non-zero for owned run_pokemon instances
	Level         int // current level; populated for owned instances
	SpeciesName   string
	FormName      string
	LocationName  string
	BlockedByRule string
}

type MoveOption struct {
	ID            int
	Name          string
	TypeName      string
	LearnMethod   string
	Level         int
	EvoNote       string
	TMNumber      int
	HMNumber      int
	TutorLocation string
	Power         *int // nil for status moves
	Accuracy      *int // nil for never-miss moves
	PP            int
	DamageClass   string // "physical", "special", "status", or "" if not seeded
	Effect        string // short description, or "" if not seeded
}

type ItemOption struct {
	ID          int
	Name        string
	DisplayName string // formatted for display; same as Name except TMs → "TM24"
	Category    string
	Source      string
	Price       int
}

// TeamSlotPage is used by the per-slot team edit page (/runs/:id/team/:slot).
type TeamSlotPage struct {
	BasePage
	SlotNum        int
	Slot           PartySlot
	OwnedPokemon   []FormOption // owned run_pokemon instances (select by rp-id)
	LegalForms     []FormOption // encounter-legal, not yet owned (select by new-id)
	LegalItems     []ItemOption
	LegalityErrors map[string]string
}

// ─── Box Manager ──────────────────────────────────────────────────────────────

type BoxPage struct {
	BasePage
	Entries     []BoxEntry
	ShowFainted bool
	NuzlockeOn  bool
}

type BoxEntry struct {
	ID              int
	FormID          int
	SpeciesName     string
	FormName        string
	Level           int
	CaughtLevel     *int
	MetLocation     string
	AcquisitionType string
	IsAlive         bool
	Evolutions      []legality.Evolution
	Moves           []string // current move names
}

// ─── Routes Log ───────────────────────────────────────────────────────────────

// EncounterOption holds a catchable species and its level range at a location.
type EncounterOption struct {
	Name     string `json:"name"`
	MinLevel int    `json:"min_level"`
	MaxLevel int    `json:"max_level"`
}

type RoutesPage struct {
	BasePage
	Log                  []RouteEntry
	Locations            []LocationOption
	EncountersByLocation map[int][]EncounterOption // keyed by location ID
	NuzlockeOn           bool
	DuplicateWarning     *DuplicateWarning
	ValidationError      string
	// Pre-filled form values on re-render
	FormLocationID int
	FormSpecies    string
	FormOutcome    string
	FormLevel      int
}

type RouteEntry struct {
	LocationName string
	SpeciesName  string
	Outcome      string
	Level        int
	IsDuplicate  bool
}

type DuplicateWarning struct {
	LocationName  string
	PreviousCatch string
}

// ─── Rules Manager ────────────────────────────────────────────────────────────

type RulesPage struct {
	BasePage
	Rules []RuleCard
}

type RuleCard struct {
	Key         string
	Label       string
	Description string
	Enabled     bool
	Params      map[string]interface{}
}

// ─── Run Overview (dashboard) ─────────────────────────────────────────────────

type OverviewPage struct {
	BasePage
	// Progress summary
	CurrentLocationName string
	ActiveFlags         []string // labels of set flags
	// Team summary
	TeamSlots []OverviewSlot
	// Box stats
	BoxAlive   int
	BoxFainted int
	// Route log (last 5)
	RecentRoutes []RouteEntry
	// Rules
	ActiveRules []string
	// Coach
	CoachAvailable bool

	// Inline progress editing (merged from ProgressPage)
	Locations        []LocationOption
	CurrentLocID     *int
	BadgeCount       int
	AllFlags         []FlagDef
	ActiveFlagMap    map[string]bool // keyed by flag key
	LocationsSeeding bool
	HydrationTotal   int
	HydrationSeeded  int
}

type OverviewSlot struct {
	Slot        int
	SpeciesName string // empty = empty slot
	Level       int
}

// ─── Pokémon page (merged Team + Box + Route logging) ─────────────────────────

type PokemonPage struct {
	BasePage
	// Team section
	Slots [6]PartySlot
	// Box section
	Entries     []BoxEntry
	ShowFainted bool
	NuzlockeOn  bool
	// Route logging section
	Log                  []RouteEntry
	Locations            []LocationOption
	EncountersByLocation map[int][]EncounterOption
	DuplicateWarning     *DuplicateWarning
	ValidationError      string
	FormLocationID       int
	FormSpecies          string
	FormOutcome          string
	FormLevel            int
}

// ─── Coaching Panel ───────────────────────────────────────────────────────────

type CoachPage struct {
	BasePage
	CoachAvailable bool
	Acquisitions   []legality.Acquisition
	Trades         []TradeOption
	PartyMoves     []PartyMoveSummary
	LegalItems     []ItemOption
	CoachAnswer    *CoachAnswer
	CoachError     string // non-empty when LLM was available but query failed
	PlayerQuestion string
	TeamInsights   *TeamInsights
	NextOpponents  []OpponentSummary // COACH-015
}

// TeamInsights holds pre-computed analysis rendered in the coach panel regardless of AI coach availability.
type TeamInsights struct {
	Members        []PartyDetailPayload
	Weaknesses     []legality.TypeThreat
	Resistances    []string
	Immunities     []string
	UncoveredTypes []string
	EvoSummaries   []EvoSummary
	// EvoPaths is keyed by form_id; used by buildCoachPayload to avoid re-querying.
	EvoPaths map[int][]legality.EvolutionPath
}

// EvoSummary is one party member's evolution options, structured for template rendering.
type EvoSummary struct {
	Slot        int
	FormID      int
	SpeciesName string
	Paths       []legality.EvolutionPath
}

// TradeOption is an NPC trade or Game Corner entry at the current location.
type TradeOption struct {
	Method         string // "trade" | "game-corner"
	GiveSpecies    string // empty for game-corner
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

// CoachAnswer holds the LLM-generated coach response.
// SECURITY BOUNDARY (SEC-018): Text comes from the AI Coach LLM host and
// must NEVER be rendered with template.HTML, safeHTML, or any unescaping
// function. Go's html/template auto-escapes it safely. If rich rendering
// (Markdown) is ever needed, use a sanitizing renderer (e.g. bluemonday).
type CoachAnswer struct {
	Text      string
	Model     string
	Truncated bool
	Question  string // non-empty when responding to a user question (vs. auto-recommendation)
}

// ─── Coach payload enrichment types (COACH-006) ───────────────────────────────

// MoveDetail carries the full details of an assigned move for the Coach payload.
type MoveDetail struct {
	Name     string `json:"name"`
	TypeName string `json:"type_name"`
	Power    *int   `json:"power"`    // nil for status moves
	Accuracy *int   `json:"accuracy"` // nil for never-miss moves
	PP       int    `json:"pp"`
}

// PartyDetailPayload is the per-member detail block sent to the Coach.
type PartyDetailPayload struct {
	Slot         int          `json:"slot"`
	RunPokemonID int          `json:"run_pokemon_id"`
	SpeciesName  string       `json:"species_name"`
	Level        int          `json:"level"`
	Types        []string     `json:"types,omitempty"`
	BaseStats    interface{}  `json:"base_stats,omitempty"` // *legality.BaseStats or nil
	Ability      string       `json:"ability,omitempty"`
	CurrentMoves []MoveDetail `json:"current_moves,omitempty"` // nil = not tracked
}

// TeamAnalysisPayload is the structured team analysis block for the Coach.
type TeamAnalysisPayload struct {
	Weaknesses     interface{} `json:"weaknesses"`
	Resistances    []string    `json:"resistances"`
	Immunities     []string    `json:"immunities"`
	UncoveredTypes []string    `json:"uncovered_types"`
}

// ─── Coach opponent types (COACH-015) ────────────────────────────────────────

// OpponentSummary is one gym leader or Elite Four member.
type OpponentSummary struct {
	Name          string            `json:"name"`
	TypeSpecialty string            `json:"type_specialty"`
	LocationName  string            `json:"location_name"`
	BadgeOrder    int               `json:"badge_order"`
	Team          []OpponentPokemon `json:"team"`
}

// OpponentPokemon is one party member in an opponent's team.
type OpponentPokemon struct {
	SpeciesName string   `json:"species_name"`
	Level       int      `json:"level"`
	Types       []string `json:"types,omitempty"`
	HeldItem    string   `json:"held_item,omitempty"`
	Moves       []string `json:"moves,omitempty"`
}

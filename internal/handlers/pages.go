// Package handlers contains Gin HTTP handlers and page data structs for
// PokemonProfessor. All template data structs embed BasePage.
package handlers

import "github.com/pennt/pokemonprofessor/internal/legality"

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
	ID          int
	Name        string
	VersionName string
	BadgeCount  int
	ActiveRules []string // only enabled rule keys
	BadgePips   []bool   // true=filled, length always 8
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
}

// StarterOption represents a choosable starter Pokémon for a game version.
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

type PartySlot struct {
	Slot         int
	FormID       *int
	FormName     string
	SpeciesName  string
	Level        *int
	MoveIDs      [4]*int
	MoveNames    [4]string
	HeldItemID   *int
	HeldItemName string
	LegalMoves   []MoveOption
}

type FormOption struct {
	ID            int
	SpeciesName   string
	FormName      string
	LocationName  string
	BlockedByRule string
}

type MoveOption struct {
	ID          int
	Name        string
	TypeName    string
	LearnMethod string
	Level       int
}

type ItemOption struct {
	ID       int
	Name     string
	Category string
	Source   string
}

// TeamSlotPage is used by the per-slot team edit page (/runs/:id/team/:slot).
type TeamSlotPage struct {
	BasePage
	SlotNum        int
	Slot           PartySlot
	LegalForms     []FormOption
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
	ID          int
	FormID      int
	SpeciesName string
	FormName    string
	Level       int
	MetLocation string
	IsAlive     bool
	Evolutions  []legality.Evolution
}

// ─── Routes Log ───────────────────────────────────────────────────────────────

type RoutesPage struct {
	BasePage
	Log              []RouteEntry
	Locations        []LocationOption
	NuzlockeOn       bool
	DuplicateWarning *DuplicateWarning
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

// ─── Coaching Panel ───────────────────────────────────────────────────────────

type CoachPage struct {
	BasePage
	ZeroClawAvailable bool
	Acquisitions      []legality.Acquisition
	PartyMoves        []PartyMoveSummary
	LegalItems        []ItemOption
	CoachAnswer       *CoachAnswer
	PlayerQuestion    string
}

type PartyMoveSummary struct {
	Slot        int
	SpeciesName string
	Moves       []MoveOption
}

type CoachAnswer struct {
	Text      string
	Model     string
	Truncated bool
}

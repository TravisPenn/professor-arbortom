// Package legality contains the core legality engine for PokemonProfessor.
// It determines what Pokémon, moves, items, and evolutions are currently
// legal given a run's progress and active rules.
package legality

// Acquisition represents a Pokémon that can legally be obtained at the
// player's current point in the run.
type Acquisition struct {
	FormID        int     `json:"form_id"`
	SpeciesName   string  `json:"species_name"`
	FormName      string  `json:"form_name"`
	LocationName  string  `json:"location_name"`
	Method        string  `json:"method"`
	MinLevel      int     `json:"min_level"`
	MaxLevel      int     `json:"max_level"`
	BlockedByRule *string `json:"blocked_by_rule"` // nil = available; "nuzlocke"/"level_cap" = annotated block
}

// Move represents a move legally learnable by a form at the player's
// current point in the run.
type Move struct {
	MoveID        int     `json:"move_id"`
	Name          string  `json:"name"`
	TypeName      string  `json:"type_name"`
	LearnMethod   string  `json:"learn_method"`
	LevelLearned  int     `json:"level_learned"`
	BlockedByRule *string `json:"blocked_by_rule"`
}

// Item represents an item that is owned or legally obtainable.
type Item struct {
	ItemID   int    `json:"item_id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Source   string `json:"source"` // "owned" | "obtainable"
	Qty      int    `json:"qty"`    // 0 if source == "obtainable"
}

// Evolution represents a possible evolution for a Pokémon form.
type Evolution struct {
	ToFormID          int                    `json:"to_form_id"`
	ToSpeciesName     string                 `json:"to_species_name"`
	Trigger           string                 `json:"trigger"`
	Conditions        map[string]interface{} `json:"conditions"`
	CurrentlyPossible bool                   `json:"currently_possible"`
	BlockedByRule     *string                `json:"blocked_by_rule"`
}

// Warning is a non-fatal advisory returned alongside legality results.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// blocked returns a pointer to the rule key string, for use in BlockedByRule fields.
func blocked(rule string) *string {
	s := rule
	return &s
}

// RunState is a lightweight projection of the run's current status used
// internally by legality functions to avoid repeated DB queries.
type RunState struct {
	RunID          int
	VersionID      int
	VersionGroupID int
	BadgeCount     int
	LocationID     *int // nil if not yet set
	ActiveRules    map[string]bool
	RuleParams     map[string]map[string]interface{}
	// Flags set to "true" in run_flag
	Flags map[string]bool
}

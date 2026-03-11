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
	// TMNumber is non-zero for machine moves and holds the TM number (1-50)
	// that teaches this move in the current generation.
	TMNumber int `json:"tm_number,omitempty"`
	// HMNumber is non-zero for HM machine moves and holds the HM number (1-8).
	HMNumber int `json:"hm_number,omitempty"`
	// TutorLocation is non-empty for tutor moves and names the in-game
	// location where the tutor can be found, e.g. "Viridian City".
	TutorLocation string `json:"tutor_location,omitempty"`
	// EvoNote is non-empty when a direct evolution learns this move at a
	// different level, e.g. "Ivysaur Lv15 ↑" (sooner) or "Ivysaur Lv31 ↓" (later).
	// Only populated by CoachMoves.
	EvoNote string `json:"evo_note,omitempty"`
}

// Item represents an item that is owned, obtainable, or purchasable at a shop.
type Item struct {
	ItemID   int    `json:"item_id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Source   string `json:"source"` // "owned" | "obtainable" | "shop"
	Qty      int    `json:"qty"`    // 0 if source != "owned"
	Price    int    `json:"price,omitempty"`
	Currency string `json:"currency,omitempty"` // "pokedollar" | "coins"
	// MoveName is non-empty for TM items and holds the PokeAPI move slug
	// the TM teaches (e.g. "thunderbolt"). Only populated by ShopItems.
	MoveName string `json:"move_name,omitempty"`
}

// Trade represents a Pokémon obtainable via NPC trade or Game Corner.
type Trade struct {
	LocationName   string `json:"location_name"`
	Method         string `json:"method"`       // "trade" | "game-corner"
	GiveSpecies    string `json:"give_species"` // empty string for game-corner entries
	ReceiveSpecies string `json:"receive_species"`
	ReceiveNick    string `json:"receive_nick,omitempty"`
	PriceCoins     int    `json:"price_coins,omitempty"`
	Notes          string `json:"notes,omitempty"`
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

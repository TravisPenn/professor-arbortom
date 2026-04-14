package legality

import (
	"database/sql"
	"fmt"
)

// LevelCap returns the current level cap for the run given its badge count
// and level_cap rule parameters. Returns 0 if no cap applies
// (post-champion or rule disabled).
func LevelCap(db *sql.DB, rs *RunState) (int, error) {
	if !rs.ActiveRules["level_cap"] {
		return 0, nil // rule disabled — no cap
	}

	// Check for explicit override in params
	if params, ok := rs.RuleParams["level_cap"]; ok {
		if cap, ok := params["cap"]; ok {
			switch v := cap.(type) {
			case float64:
				return int(v), nil
			case int:
				return v, nil
			}
		}
	}

	// Derive from badge count table
	var cap int
	err := db.QueryRow(
		`SELECT level_cap FROM gen3_badge_cap WHERE badge_count = ?`, rs.BadgeCount,
	).Scan(&cap)
	if err == sql.ErrNoRows {
		return 0, nil // post-champion
	}
	if err != nil {
		return 0, fmt.Errorf("legality: level cap lookup: %w", err)
	}
	return cap, nil
}

// hmFlagMap maps HM move names to the run flag that gates their availability.
// Kept alongside methodFlagMap so both flag-driven gating maps are co-located.
var hmFlagMap = map[string]string{
	"cut":        "hm.cut_obtained",
	"fly":        "hm.fly_obtained",
	"surf":       "hm.surf_obtained",
	"strength":   "hm.strength_obtained",
	"flash":      "hm.flash_obtained",
	"rock-smash": "hm.rock_smash_obtained",
	"waterfall":  "hm.waterfall_obtained",
	"dive":       "hm.dive_obtained",
}

// methodFlagMap maps encounter methods to the run flag that must be set
// for those encounters to be available. If the flag is not set (or false),
// the acquisition is annotated as blocked.
var methodFlagMap = map[string]string{
	"old-rod":    "item.old_rod",
	"good-rod":   "item.good_rod",
	"super-rod":  "item.super_rod",
	"surf":       "hm.surf_obtained",
	"rock-smash": "hm.rock_smash_obtained",
}

// ApplyRules decorates acquisitions with BlockedByRule annotations.
// It does NOT remove blocked entries — the Coach agent needs them.
func ApplyRules(db *sql.DB, rs *RunState, acqs []Acquisition) ([]Acquisition, error) {
	// Nuzlocke: mark duplicates (form already caught at same location)
	if rs.ActiveRules["nuzlocke"] {
		// Build set of location IDs that already have a catch
		caughtLocations := make(map[int]bool)
		rows, err := db.Query(
			`SELECT met_location_id FROM run_pokemon WHERE run_id = ? AND met_location_id IS NOT NULL`, rs.RunID,
		)
		if err != nil {
			return acqs, fmt.Errorf("legality: nuzlocke query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var locID int
			if err := rows.Scan(&locID); err == nil {
				caughtLocations[locID] = true
			}
		}

		// Duplicate check is handled in handlers/routes.go which has location IDs.
		_ = caughtLocations
	}

	// Level cap: annotate acquisitions whose max_level exceeds cap
	cap, err := LevelCap(db, rs)
	if err != nil {
		return acqs, err
	}
	if cap > 0 {
		for i := range acqs {
			if acqs[i].MinLevel > cap {
				acqs[i].BlockedByRule = blocked("level_cap")
			}
		}
	}

	// Encounter method gating: annotate encounters that require an item or
	// HM the player hasn't obtained yet (rods, Surf, Rock Smash).
	for i := range acqs {
		if acqs[i].BlockedByRule != nil {
			continue // already blocked by a higher-priority rule
		}
		if flag, ok := methodFlagMap[acqs[i].Method]; ok {
			if !rs.Flags[flag] {
				acqs[i].BlockedByRule = blocked("missing_item")
			}
		}
	}

	return acqs, nil
}

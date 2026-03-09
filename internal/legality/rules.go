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
		rule := "level_cap"
		for i := range acqs {
			if acqs[i].MinLevel > cap {
				acqs[i].BlockedByRule = &rule
			}
		}
	}

	return acqs, nil
}

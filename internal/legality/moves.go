package legality

import (
	"database/sql"
	"fmt"
)

// LegalMoves returns all moves legally learnable by a form at the current
// point in the run (version group + badge count level cap).
func LegalMoves(db *sql.DB, runID, formID int) ([]Move, []Warning, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(`
		SELECT
			m.id,
			m.name,
			m.type_name,
			le.learn_method,
			le.level_learned
		FROM learnset_entry le
		JOIN move m ON m.id = le.move_id
		WHERE le.form_id = ?
		  AND le.version_group_id = ?
		ORDER BY le.learn_method, le.level_learned, m.name
	`, formID, rs.VersionGroupID)
	if err != nil {
		return nil, nil, fmt.Errorf("legality: moves query: %w", err)
	}
	defer rows.Close()

	cap, _ := LevelCap(db, rs)

	var moves []Move
	var warns []Warning

	for rows.Next() {
		var mv Move
		if err := rows.Scan(&mv.MoveID, &mv.Name, &mv.TypeName, &mv.LearnMethod, &mv.LevelLearned); err != nil {
			return nil, nil, err
		}

		// Annotate level-up moves blocked by cap
		if cap > 0 && mv.LearnMethod == "level-up" && mv.LevelLearned > cap {
			rule := "level_cap"
			mv.BlockedByRule = &rule
		}

		// HM moves — check flag availability
		if blocked := hmBlockedRule(rs, mv.Name); blocked != "" {
			rule := blocked
			mv.BlockedByRule = &rule
			warns = append(warns, Warning{
				Code:    "hm_flag",
				Message: fmt.Sprintf("%s requires HM flag not yet set", mv.Name),
			})
		}

		moves = append(moves, mv)
	}

	return moves, warns, nil
}

// hmBlockedRule returns a non-empty rule key if this move is an HM move that requires
// a flag the run hasn't set yet.
func hmBlockedRule(rs *RunState, moveName string) string {
	hmFlags := map[string]string{
		"cut":        "hm.cut_obtained",
		"fly":        "hm.fly_obtained",
		"surf":       "hm.surf_obtained",
		"strength":   "hm.strength_obtained",
		"flash":      "hm.flash_obtained",
		"rock-smash": "hm.rock_smash_obtained",
		"waterfall":  "hm.waterfall_obtained",
		"dive":       "hm.dive_obtained",
	}

	flagKey, isHM := hmFlags[moveName]
	if !isHM {
		return ""
	}
	if rs.Flags[flagKey] {
		return "" // flag set → move available
	}
	return "hm_flag_missing"
}

package legality

import (
	"database/sql"
	"fmt"
	"strings"
)

// LegalMoves returns all moves legally learnable by a form at the current
// point in the run (version group + badge count level cap).
func LegalMoves(db *sql.DB, runID, formID int) ([]Move, []Warning, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, nil, err
	}

	cap, err := LevelCap(db, rs)
	if err != nil {
		return nil, nil, fmt.Errorf("legality: level cap for moves: %w", err)
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

	var moves []Move
	var warns []Warning

	for rows.Next() {
		var mv Move
		if err := rows.Scan(&mv.MoveID, &mv.Name, &mv.TypeName, &mv.LearnMethod, &mv.LevelLearned); err != nil {
			return nil, nil, err
		}

		// Annotate level-up moves blocked by cap
		if cap > 0 && mv.LearnMethod == "level-up" && mv.LevelLearned > cap {
			mv.BlockedByRule = blocked("level_cap")
		}

		// HM moves — check flag availability
		if b := hmBlockedRule(rs, mv.Name); b != "" {
			mv.BlockedByRule = blocked(b)
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

// CoachMoves returns moves for the Coach panel for a single party Pokémon.
// Differences from LegalMoves:
//   - egg-method moves are omitted (not actionable in a live run)
//   - level-up moves with level_learned <= currentLevel are omitted (already accessible)
//   - remaining level-up moves gain an EvoNote when a direct evolution would
//     learn the same move at a notably different level
//
// currentLevel == 0 disables the "already accessible" filtering.
func CoachMoves(db *sql.DB, runID, formID, currentLevel int) ([]Move, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, err
	}

	cap, err := LevelCap(db, rs)
	if err != nil {
		return nil, fmt.Errorf("legality: level cap for coach moves: %w", err)
	}

	rows, err := db.Query(`
		SELECT m.id, m.name, m.type_name, le.learn_method, le.level_learned,
		       COALESCE(tm.tm_number, 0)       AS tm_number,
		       COALESCE(hm.hm_number, 0)       AS hm_number,
		       COALESCE(tut.location_name, '') AS tutor_location
		FROM learnset_entry le
		JOIN move m ON m.id = le.move_id
		LEFT JOIN tm_move tm ON (le.learn_method = 'machine' AND m.name = tm.move_name)
		LEFT JOIN hm_move hm ON (le.learn_method = 'machine' AND m.name = hm.move_name)
		LEFT JOIN tutor_move tut ON (
		    le.learn_method = 'tutor'
		    AND m.name = tut.move_name
		    AND tut.version_group_id = le.version_group_id
		)
		WHERE le.form_id = ?
		  AND le.version_group_id = ?
		  AND le.learn_method != 'egg'
		ORDER BY le.learn_method, le.level_learned, m.name
	`, formID, rs.VersionGroupID)
	if err != nil {
		return nil, fmt.Errorf("legality: coach moves query: %w", err)
	}
	defer rows.Close()

	var moves []Move
	for rows.Next() {
		var mv Move
		if err := rows.Scan(&mv.MoveID, &mv.Name, &mv.TypeName, &mv.LearnMethod, &mv.LevelLearned,
			&mv.TMNumber, &mv.HMNumber, &mv.TutorLocation); err != nil {
			return nil, err
		}
		// Skip level-up moves the Pokémon has already had the chance to learn.
		if mv.LearnMethod == "level-up" && currentLevel > 0 && mv.LevelLearned <= currentLevel {
			continue
		}
		// Cap annotation — same as LegalMoves.
		if cap > 0 && mv.LearnMethod == "level-up" && mv.LevelLearned > cap {
			mv.BlockedByRule = blocked("level_cap")
		}
		// HM annotation.
		if b := hmBlockedRule(rs, mv.Name); b != "" {
			mv.BlockedByRule = blocked(b)
		}
		moves = append(moves, mv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Annotate evo learnset differences (non-fatal: skip on any error).
	_ = annotateEvoNotes(db, moves, formID, rs.VersionGroupID)

	// Append moves exclusive to evolutions (non-fatal: skip on any error).
	moves, _ = appendEvoExclusiveMoves(db, moves, formID, rs.VersionGroupID)

	return moves, nil
}

// annotateEvoNotes adds EvoNote to level-up moves where a direct evolution
// learns the same move at a different level in the same version group.
func annotateEvoNotes(db *sql.DB, moves []Move, formID, versionGroupID int) error {
	if len(moves) == 0 {
		return nil
	}

	evoRows, err := db.Query(`
		SELECT ec.to_form_id, ps.name
		FROM evolution_condition ec
		JOIN pokemon_form pf ON pf.id = ec.to_form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		WHERE ec.from_form_id = ?
		GROUP BY ec.to_form_id
	`, formID)
	if err != nil {
		return err
	}
	defer evoRows.Close()

	type evoInfo struct {
		formID int
		name   string
	}
	var evos []evoInfo
	for evoRows.Next() {
		var e evoInfo
		if err := evoRows.Scan(&e.formID, &e.name); err != nil {
			continue
		}
		evos = append(evos, e)
	}
	if len(evos) == 0 {
		return nil
	}

	// Build level-up learnset per evolution: moveName → earliest level.
	evoLearnsets := make(map[int]map[string]int, len(evos))
	for _, evo := range evos {
		ls := map[string]int{}
		lsRows, err := db.Query(`
			SELECT m.name, le.level_learned
			FROM learnset_entry le
			JOIN move m ON m.id = le.move_id
			WHERE le.form_id = ? AND le.version_group_id = ? AND le.learn_method = 'level-up'
			ORDER BY le.level_learned
		`, evo.formID, versionGroupID)
		if err != nil {
			continue
		}
		for lsRows.Next() {
			var name string
			var lvl int
			if err := lsRows.Scan(&name, &lvl); err == nil {
				if _, exists := ls[name]; !exists {
					ls[name] = lvl // keep earliest
				}
			}
		}
		lsRows.Close()
		evoLearnsets[evo.formID] = ls
	}

	// Annotate each level-up move with all evo levels.
	for i := range moves {
		if moves[i].LearnMethod != "level-up" {
			continue
		}
		var notes []string
		for _, evo := range evos {
			ls := evoLearnsets[evo.formID]
			evoLvl, ok := ls[moves[i].Name]
			if !ok {
				continue // evolution doesn't learn this move at all via level-up
			}
			label := capitalizeFirst(evo.name)
			notes = append(notes, fmt.Sprintf("%s Lv%d", label, evoLvl))
		}
		if len(notes) > 0 {
			moves[i].EvoNote = strings.Join(notes, " · ")
		}
	}
	return nil
}

// capitalizeFirst returns s with its first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// appendEvoExclusiveMoves appends moves that are only learnable by one or more
// evolutions of formID (not by formID itself). These are tagged with
// LearnMethod "evo-exclusive" and EvoNote naming the evolution and level.
func appendEvoExclusiveMoves(db *sql.DB, moves []Move, formID, versionGroupID int) ([]Move, error) {
	// Collect all evolution form IDs (recursive up to depth 2).
	type evoInfo struct {
		formID int
		name   string
	}
	var allEvos []evoInfo
	frontier := []int{formID}
	for depth := 0; depth < 2 && len(frontier) > 0; depth++ {
		var nextFrontier []int
		for _, fid := range frontier {
			rows, err := db.Query(`
				SELECT ec.to_form_id, ps.name
				FROM evolution_condition ec
				JOIN pokemon_form pf ON pf.id = ec.to_form_id
				JOIN pokemon_species ps ON ps.id = pf.species_id
				WHERE ec.from_form_id = ?
				GROUP BY ec.to_form_id
			`, fid)
			if err != nil {
				continue
			}
			for rows.Next() {
				var e evoInfo
				if rows.Scan(&e.formID, &e.name) == nil {
					allEvos = append(allEvos, e)
					nextFrontier = append(nextFrontier, e.formID)
				}
			}
			rows.Close()
		}
		frontier = nextFrontier
	}
	if len(allEvos) == 0 {
		return moves, nil
	}

	// Build set of move names already present in the current moves list.
	currentMoveNames := make(map[string]bool, len(moves))
	for _, mv := range moves {
		currentMoveNames[mv.Name] = true
	}

	// Also build the base form's full learnset for exclusion.
	baseLearnset := make(map[string]bool)
	baseRows, err := db.Query(`
		SELECT m.name FROM learnset_entry le
		JOIN move m ON m.id = le.move_id
		WHERE le.form_id = ? AND le.version_group_id = ?
	`, formID, versionGroupID)
	if err == nil {
		for baseRows.Next() {
			var name string
			if baseRows.Scan(&name) == nil {
				baseLearnset[name] = true
			}
		}
		baseRows.Close()
	}

	// For each evo, find moves exclusive to evolutions.
	seen := make(map[string]bool)
	for _, evo := range allEvos {
		rows, err := db.Query(`
			SELECT m.id, m.name, m.type_name, le.learn_method, le.level_learned
			FROM learnset_entry le
			JOIN move m ON m.id = le.move_id
			WHERE le.form_id = ? AND le.version_group_id = ?
			  AND le.learn_method != 'egg'
			ORDER BY le.learn_method, le.level_learned, m.name
		`, evo.formID, versionGroupID)
		if err != nil {
			continue
		}
		for rows.Next() {
			var mv Move
			if rows.Scan(&mv.MoveID, &mv.Name, &mv.TypeName, &mv.LearnMethod, &mv.LevelLearned) != nil {
				continue
			}
			// Skip if base form can learn it or we already added it.
			if baseLearnset[mv.Name] || currentMoveNames[mv.Name] || seen[mv.Name] {
				continue
			}
			seen[mv.Name] = true

			label := capitalizeFirst(evo.name)
			if mv.LearnMethod == "level-up" {
				mv.EvoNote = fmt.Sprintf("%s Lv%d", label, mv.LevelLearned)
			} else {
				mv.EvoNote = fmt.Sprintf("%s (%s)", label, mv.LearnMethod)
			}
			mv.LearnMethod = "evo-exclusive"
			moves = append(moves, mv)
		}
		rows.Close()
	}

	return moves, nil
}

package legality

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// EvolutionOptions returns possible evolutions for the given form in the context
// of the current run state.
func EvolutionOptions(db *sql.DB, runID, formID int) ([]Evolution, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, err
	}

	cap, err := LevelCap(db, rs)
	if err != nil {
		return nil, fmt.Errorf("legality: level cap for evolutions: %w", err)
	}

	rows, err := db.Query(`
		SELECT
			ec.to_form_id,
			ps.name AS to_species_name,
			ec.trigger,
			ec.conditions_json
		FROM evolution_condition ec
		JOIN pokemon_form pf ON pf.id = ec.to_form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		WHERE ec.from_form_id = ?
		ORDER BY ec.trigger
	`, formID)
	if err != nil {
		return nil, fmt.Errorf("legality: evolutions query: %w", err)
	}
	defer rows.Close()

	var evos []Evolution
	for rows.Next() {
		var evo Evolution
		var condJSON string
		if err := rows.Scan(&evo.ToFormID, &evo.ToSpeciesName, &evo.Trigger, &condJSON); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(condJSON), &evo.Conditions); err != nil {
			evo.Conditions = map[string]interface{}{}
		}

		evo.CurrentlyPossible = isEvoCurrentlyPossible(rs, evo, cap)

		// no_trade_evolutions rule
		if rs.ActiveRules["no_trade_evolutions"] && evo.Trigger == "trade" {
			evo.BlockedByRule = blocked("no_trade_evolutions")
		}

		evos = append(evos, evo)
	}

	return evos, nil
}

func isEvoCurrentlyPossible(rs *RunState, evo Evolution, levelCap int) bool {
	cond := evo.Conditions
	switch evo.Trigger {
	case "level-up":
		if minLevel, ok := cond["min_level"]; ok {
			minLvl := int(toFloat64(minLevel))
			if levelCap > 0 && minLvl > levelCap {
				return false
			}
		}
		if _, ok := cond["friendship"]; ok {
			return rs.Flags["story.high_friendship"]
		}
		return true
	case "use-item":
		return true // item availability handled separately
	case "trade":
		return !rs.ActiveRules["no_trade_evolutions"]
	default:
		return true
	}
}

func toFloat64(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return 0
}

package legality

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// LoadRunState loads the RunState for a given run ID from the database.
func LoadRunState(db *sql.DB, runID int) (*RunState, error) {
	var rs RunState
	rs.RunID = runID

	// Core run + progress (SC-002: badge_count and current_location_id are now on run)
	err := db.QueryRow(`
		SELECT r.version_id, gv.version_group_id, COALESCE(r.badge_count, 0), r.current_location_id
		FROM run r
		JOIN game_version gv ON gv.id = r.version_id
		WHERE r.id = ?
	`, runID).Scan(&rs.VersionID, &rs.VersionGroupID, &rs.BadgeCount, &rs.LocationID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("legality: run %d not found", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("legality: load run state: %w", err)
	}

	// Active rules + params (SC-003: run_setting replaces run_rule + rule_def)
	rs.ActiveRules = make(map[string]bool)
	rs.RuleParams = make(map[string]map[string]interface{})

	rows, err := db.Query(`
		SELECT key, value
		FROM run_setting
		WHERE run_id = ? AND type = 'rule'
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("legality: load rules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, paramsJSON string
		if err := rows.Scan(&key, &paramsJSON); err != nil {
			return nil, err
		}
		rs.ActiveRules[key] = true

		if paramsJSON != "" && paramsJSON != "{}" {
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(paramsJSON), &params); err == nil {
				rs.RuleParams[key] = params
			}
		}
	}

	// Run flags (SC-003: run_setting replaces run_flag)
	rs.Flags = make(map[string]bool)
	flagRows, err := db.Query(
		`SELECT key, value FROM run_setting WHERE run_id = ? AND type = 'flag'`, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("legality: load flags: %w", err)
	}
	defer flagRows.Close()

	for flagRows.Next() {
		var key, val string
		if err := flagRows.Scan(&key, &val); err != nil {
			return nil, err
		}
		rs.Flags[key] = val == "true"
	}

	return &rs, nil
}

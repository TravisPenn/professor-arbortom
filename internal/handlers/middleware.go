package handlers

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/gin-gonic/gin"
)

// RunContext sets the "run", "progress", "active_rules", and "version" keys
// in the Gin context for all routes under /runs/:run_id/*.
func RunContextMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runIDStr := c.Param("run_id")
		runID, err := strconv.Atoi(runIDStr)
		if err != nil {
			respondNotFound(c)
			return
		}

		// Load run
		var run models.Run
		err = db.QueryRow(
			`SELECT id, name, user_id, version_id, created_at FROM run WHERE id = ?`, runID,
		).Scan(&run.ID, &run.Name, &run.UserID, &run.VersionID, &run.CreatedAt)
		if err == sql.ErrNoRows {
			respondNotFound(c)
			return
		}
		if err != nil {
			respondError(c, err)
			return
		}

		// Load progress from run columns (SC-002)
		var progress models.RunProgress
		progress.RunID = runID
		err = db.QueryRow(
			`SELECT COALESCE(badge_count, 0), current_location_id, COALESCE(progress_updated_at, '') FROM run WHERE id = ?`,
			runID,
		).Scan(&progress.BadgeCount, &progress.CurrentLocationID, &progress.UpdatedAt)
		if err != nil {
			respondError(c, err)
			return
		}

		// Load active rules
		activeRules, err := loadActiveRules(db, runID)
		if err != nil {
			respondError(c, err)
			return
		}

		// Load version
		var version models.GameVersion
		err = db.QueryRow(
			`SELECT id, name, version_group_id, generation_id FROM game_version WHERE id = ?`, run.VersionID,
		).Scan(&version.ID, &version.Name, &version.VersionGroupID, &version.GenerationID)
		if err != nil {
			respondError(c, err)
			return
		}

		c.Set("run", run)
		c.Set("progress", progress)
		c.Set("active_rules", activeRules)
		c.Set("version", version)

		c.Next()
	}
}

func loadActiveRules(db *sql.DB, runID int) ([]models.ActiveRule, error) {
	// SC-003: use run_setting exclusively
	rows, err := db.Query(`
		SELECT key, value
		FROM run_setting
		WHERE run_id = ? AND type = 'rule'
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.ActiveRule
	for rows.Next() {
		var r models.ActiveRule
		if err := rows.Scan(&r.Key, &r.ParamsJSON); err != nil {
			return nil, err
		}
		r.Enabled = true
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// buildRunContext constructs the RunContext template struct from Gin context values.
func buildRunContext(c *gin.Context) *RunContext {
	runVal, exists := c.Get("run")
	if !exists {
		return nil
	}
	run := runVal.(models.Run)

	progress, _ := c.Get("progress")
	prog := progress.(models.RunProgress)

	activeRules, _ := c.Get("active_rules")
	rules := activeRules.([]models.ActiveRule)
	version, _ := c.Get("version")
	ver := version.(models.GameVersion)

	var enabledKeys []string
	for _, r := range rules {
		if r.Enabled {
			enabledKeys = append(enabledKeys, r.Key)
		}
	}

	pips := make([]bool, 8)
	for i := 0; i < 8; i++ {
		pips[i] = i < prog.BadgeCount
	}

	return &RunContext{
		ID:          run.ID,
		Name:        run.Name,
		VersionName: ver.Name,
		BadgeCount:  prog.BadgeCount,
		ActiveRules: enabledKeys,
		BadgePips:   pips,
	}
}

// ruleParams parses a rules params_json string into a map.
func ruleParams(paramsJSON string) map[string]interface{} {
	if paramsJSON == "" || paramsJSON == "{}" {
		return nil
	}
	var m map[string]interface{}
	json.Unmarshal([]byte(paramsJSON), &m) //nolint:errcheck
	return m
}

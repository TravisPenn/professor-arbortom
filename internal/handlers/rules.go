package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/gin-gonic/gin"
)

// ruleCatalog is the authoritative list of supported rules.
// Replaces the rule_def table (SC-003).
var ruleCatalog = []struct {
	Key         string
	Label       string
	Description string
}{
	{"nuzlocke", "Nuzlocke", "Only one catch per route; fainted Pokémon are considered dead"},
	{"level_cap", "Level Cap", "Pokémon cannot exceed the level of the next gym leader's ace"},
	{"no_trade_evolutions", "No Trade Evolutions", "Trade-evolution Pokémon cannot be evolved"},
	{"no_item_locations", "No Item/TM Locations", "Hide item and TM pickup recommendations from the coach"},
	{"theme_run", "Theme Run", "Restrict your team to a specific theme or type"},
}

// ShowRules renders GET /runs/:run_id/rules
func ShowRules(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		page, err := buildRulesPage(c, db, run.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.HTML(http.StatusOK, "rules.html", page)
	}
}

// UpdateRules handles POST /runs/:run_id/rules
func UpdateRules(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)

		for _, def := range ruleCatalog {
			enabled := c.PostForm("rule_"+def.Key) != ""

			paramsJSON := "{}"
			if def.Key == "level_cap" {
				rawParams := c.PostForm("rule_level_cap_params")
				if rawParams != "" {
					capVal, err := strconv.Atoi(rawParams)
					if err == nil && capVal >= 1 && capVal <= 100 {
						b, _ := json.Marshal(map[string]int{"cap": capVal})
						paramsJSON = string(b)
					}
				}
			} else if def.Key == "theme_run" {
				desc := c.PostForm("rule_theme_run_params")
				if desc != "" {
					b, _ := json.Marshal(map[string]string{"description": desc})
					paramsJSON = string(b)
				}
			}

			// SC-003: write directly to run_setting.
			if enabled {
				db.Exec( //nolint:errcheck
					`INSERT OR REPLACE INTO run_setting (run_id, type, key, value) VALUES (?, 'rule', ?, ?)`,
					run.ID, def.Key, paramsJSON,
				)
			} else {
				db.Exec( //nolint:errcheck
					`DELETE FROM run_setting WHERE run_id = ? AND type = 'rule' AND key = ?`,
					run.ID, def.Key,
				)
			}
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/rules")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildRulesPage(c *gin.Context, db *sql.DB, runID int) (RulesPage, error) {
	// Load enabled rules from run_setting (SC-003).
	enabledRows, err := db.Query(
		`SELECT key, value FROM run_setting WHERE run_id = ? AND type = 'rule'`, runID,
	)
	if err != nil {
		return RulesPage{}, err
	}
	defer enabledRows.Close()

	enabledMap := make(map[string]string)
	for enabledRows.Next() {
		var key, val string
		if err := enabledRows.Scan(&key, &val); err == nil {
			enabledMap[key] = val
		}
	}

	var cards []RuleCard
	for _, def := range ruleCatalog {
		rc := RuleCard{
			Key:         def.Key,
			Label:       def.Label,
			Description: def.Description,
		}
		if paramsJSON, ok := enabledMap[def.Key]; ok {
			rc.Enabled = true
			rc.Params = ruleParams(paramsJSON)
		}
		cards = append(cards, rc)
	}

	return RulesPage{
		BasePage: BasePage{
			PageTitle:  "Rules",
			ActiveNav:  "rules",
			RunContext: buildRunContext(c),
		},
		Rules: cards,
	}, nil
}

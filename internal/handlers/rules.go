package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/gin-gonic/gin"
)

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

		// Load rule defs
		rows, err := db.Query(
			`SELECT rd.id, rd.key FROM rule_def rd`,
		)
		if err != nil {
			respondError(c, err)
			return
		}
		defer rows.Close()

		type ruleDef struct {
			id  int
			key string
		}
		var defs []ruleDef
		for rows.Next() {
			var d ruleDef
			rows.Scan(&d.id, &d.key) //nolint:errcheck
			defs = append(defs, d)
		}

		for _, d := range defs {
			enabled := 0
			if c.PostForm("rule_"+d.key) != "" {
				enabled = 1
			}

			paramsJSON := "{}"
			if d.key == "level_cap" {
				rawParams := c.PostForm("rule_level_cap_params")
				if rawParams != "" {
					capVal, err := strconv.Atoi(rawParams)
					if err == nil && capVal >= 1 && capVal <= 100 {
						b, _ := json.Marshal(map[string]int{"cap": capVal})
						paramsJSON = string(b)
					}
				}
			} else if d.key == "theme_run" {
				desc := c.PostForm("rule_theme_run_params")
				if desc != "" {
					b, _ := json.Marshal(map[string]string{"description": desc})
					paramsJSON = string(b)
				}
			}

			db.Exec(`
				INSERT INTO run_rule (run_id, rule_def_id, enabled, params_json)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(run_id, rule_def_id) DO UPDATE SET
					enabled = excluded.enabled,
					params_json = excluded.params_json
			`, run.ID, d.id, enabled, paramsJSON) //nolint:errcheck

			// SC-003 compatibility: mirror enabled rules into run_setting.
			if tableExists(db, "run_setting") {
				if enabled == 1 {
					db.Exec(
						`INSERT OR REPLACE INTO run_setting (run_id, type, key, value) VALUES (?, 'rule', ?, ?)`,
						run.ID, d.key, paramsJSON,
					) //nolint:errcheck
				} else {
					db.Exec(
						`DELETE FROM run_setting WHERE run_id = ? AND type = 'rule' AND key = ?`,
						run.ID, d.key,
					) //nolint:errcheck
				}
			}
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/rules")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

var ruleLabels = map[string]string{
	"nuzlocke":            "Nuzlocke",
	"level_cap":           "Level Cap",
	"no_trade_evolutions": "No Trade Evolutions",
	"theme_run":           "Theme Run",
}

func buildRulesPage(c *gin.Context, db *sql.DB, runID int) (RulesPage, error) {
	rows, err := db.Query(`
		SELECT rd.key, rd.description, rr.enabled, rr.params_json
		FROM rule_def rd
		LEFT JOIN run_rule rr ON rr.rule_def_id = rd.id AND rr.run_id = ?
		ORDER BY rd.id
	`, runID)
	if err != nil {
		return RulesPage{}, err
	}
	defer rows.Close()

	var cards []RuleCard
	for rows.Next() {
		var rc RuleCard
		var enabled int
		var paramsJSON string
		if err := rows.Scan(&rc.Key, &rc.Description, &enabled, &paramsJSON); err != nil {
			continue
		}
		rc.Label = ruleLabels[rc.Key]
		if rc.Label == "" {
			rc.Label = rc.Key
		}
		rc.Enabled = enabled == 1
		rc.Params = ruleParams(paramsJSON)
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

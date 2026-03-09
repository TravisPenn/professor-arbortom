package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pennt/pokemonprofessor/internal/models"
	"github.com/pennt/pokemonprofessor/internal/pokeapi"
)

// RedirectToRuns is the root handler: redirects / → /runs
func RedirectToRuns(c *gin.Context) {
	c.Redirect(http.StatusFound, "/runs")
}

// ListRuns renders the runs dashboard.
func ListRuns(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runs, err := loadRunSummaries(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		versions, err := loadVersionOptions(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		starters, err := loadStartersByVersion(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		page := RunsPage{
			BasePage: BasePage{
				PageTitle: "Your Runs",
				ActiveNav: "runs",
			},
			Runs:              runs,
			Versions:          versions,
			StartersByVersion: starters,
		}

		c.HTML(http.StatusOK, "runs.html", page)
	}
}

// CreateRun handles POST /runs.
func CreateRun(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userName := strings.TrimSpace(c.PostForm("user_name"))
		runName := strings.TrimSpace(c.PostForm("run_name"))
		versionIDStr := c.PostForm("version_id")

		// Validate
		errs := map[string]string{}
		if userName == "" || len(userName) > 50 {
			errs["user_name"] = "Name must be 1–50 characters"
		}
		if runName == "" || len(runName) > 100 {
			errs["run_name"] = "Run name must be 1–100 characters"
		}

		var versionID int
		if n, err := scanInt(versionIDStr); err != nil || n <= 0 {
			errs["version_id"] = "Please select a game version"
		} else {
			versionID = n
			var exists int
			db.QueryRow(`SELECT COUNT(*) FROM game_version WHERE id = ?`, versionID).Scan(&exists) //nolint:errcheck
			if exists == 0 {
				errs["version_id"] = "Unknown version"
			}
		}

		var starterFormID int
		starterFormIDStr := c.PostForm("starter_form_id")
		if n, err := scanInt(starterFormIDStr); err != nil || n <= 0 {
			errs["starter_form_id"] = "Please select a starter Pokémon"
		} else {
			var valid int
			db.QueryRow(`SELECT COUNT(*) FROM game_starter WHERE version_id = ? AND form_id = ?`, versionID, n).Scan(&valid) //nolint:errcheck
			if valid == 0 {
				errs["starter_form_id"] = "Invalid starter for this game version"
			} else {
				starterFormID = n
			}
		}

		if len(errs) > 0 {
			versions, _ := loadVersionOptions(db)
			starters, _ := loadStartersByVersion(db)
			page := RunsPage{
				BasePage: BasePage{
					PageTitle: "Your Runs",
					ActiveNav: "runs",
					Flash:     &Flash{Type: "error", Message: "Please fix the errors below."},
				},
				Versions:          versions,
				StartersByVersion: starters,
			}
			c.HTML(http.StatusUnprocessableEntity, "runs.html", page)
			return
		}

		// Upsert user
		if _, err := db.Exec(`INSERT OR IGNORE INTO user (name) VALUES (?)`, userName); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		var userID int
		if err := db.QueryRow(`SELECT id FROM user WHERE name = ?`, userName).Scan(&userID); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		// Insert run
		res, err := db.Exec(
			`INSERT INTO run (user_id, version_id, name) VALUES (?, ?, ?)`,
			userID, versionID, runName,
		)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}
		runID64, _ := res.LastInsertId()
		runID := int(runID64)

		// Insert run_progress
		if _, err := db.Exec(`INSERT INTO run_progress (run_id) VALUES (?)`, runID); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		// Insert one run_rule per rule_def (all disabled)
		if _, err := db.Exec(`
			INSERT INTO run_rule (run_id, rule_def_id, enabled)
			SELECT ?, id, 0 FROM rule_def
		`, runID); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		// Insert starter into run_pokemon: slot 1, level 5, on the party.
		if _, err := db.Exec(
			`INSERT INTO run_pokemon (run_id, form_id, level, is_alive, in_party, party_slot, moves_json)
			 VALUES (?, ?, 5, 1, 1, 1, '[]')`,
			runID, starterFormID,
		); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		// Seed the starter's learnset in the background so move dropdowns are
		// populated by the time the user opens the team page.
		var versionGroupID int
		db.QueryRow(`SELECT version_group_id FROM game_version WHERE id = ?`, versionID).Scan(&versionGroupID) //nolint:errcheck
		if versionGroupID > 0 {
			go pokeClient.EnsurePokemon(db, starterFormID, versionGroupID) //nolint:errcheck
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(runID)+"/progress")
	}
}

// ShowRun redirects /runs/:run_id → /runs/:run_id/progress
func ShowRun(c *gin.Context) {
	run := c.MustGet("run").(models.Run)
	c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/progress")
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func loadRunSummaries(db *sql.DB) ([]RunSummary, error) {
	rows, err := db.Query(`
		SELECT
			r.id, r.name, u.name AS user_name, gv.name AS version_name,
			COALESCE(rp.badge_count, 0) AS badge_count,
			COALESCE(rp.updated_at, r.created_at) AS updated_at
		FROM run r
		JOIN user u ON u.id = r.user_id
		JOIN game_version gv ON gv.id = r.version_id
		LEFT JOIN run_progress rp ON rp.run_id = r.id
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []RunSummary
	for rows.Next() {
		var rs RunSummary
		if err := rows.Scan(&rs.ID, &rs.Name, &rs.UserName, &rs.VersionName, &rs.BadgeCount, &rs.UpdatedAt); err != nil {
			return nil, err
		}

		// Load active rules for this run
		ruleRows, err := db.Query(`
			SELECT rd.key FROM run_rule rr
			JOIN rule_def rd ON rd.id = rr.rule_def_id
			WHERE rr.run_id = ? AND rr.enabled = 1
		`, rs.ID)
		if err == nil {
			defer ruleRows.Close()
			for ruleRows.Next() {
				var key string
				ruleRows.Scan(&key) //nolint:errcheck
				rs.ActiveRules = append(rs.ActiveRules, key)
			}
		}

		runs = append(runs, rs)
	}
	return runs, rows.Err()
}

func loadVersionOptions(db *sql.DB) ([]VersionOption, error) {
	rows, err := db.Query(`SELECT id, name FROM game_version ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opts []VersionOption
	for rows.Next() {
		var o VersionOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		o.Name = capitalizeVersion(o.Name)
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

func loadStartersByVersion(db *sql.DB) (map[int][]StarterOption, error) {
	rows, err := db.Query(`
		SELECT gs.version_id, gs.form_id, ps.name
		FROM game_starter gs
		JOIN pokemon_form pf ON pf.id = gs.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		ORDER BY gs.version_id, gs.priority
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]StarterOption)
	for rows.Next() {
		var versionID, formID int
		var name string
		if err := rows.Scan(&versionID, &formID, &name); err != nil {
			return nil, err
		}
		result[versionID] = append(result[versionID], StarterOption{
			FormID:      formID,
			SpeciesName: capitalizeVersion(name),
		})
	}
	return result, rows.Err()
}

func capitalizeVersion(name string) string {
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

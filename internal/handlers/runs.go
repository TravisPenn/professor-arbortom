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
		allRuns, err := loadRunSummaries(db)
		if err != nil {
			respondError(c, err)
			return
		}

		versions, err := loadVersionOptions(db)
		if err != nil {
			respondError(c, err)
			return
		}

		starters, err := loadStartersByVersion(db)
		if err != nil {
			respondError(c, err)
			return
		}

		var active, archived []RunSummary
		for _, r := range allRuns {
			if r.Archived {
				archived = append(archived, r)
			} else {
				active = append(active, r)
			}
		}

		page := RunsPage{
			BasePage: BasePage{
				PageTitle: "Your Runs",
				ActiveNav: "runs",
			},
			Runs:              active,
			ArchivedRuns:      archived,
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
			respondError(c, err)
			return
		}

		var userID int
		if err := db.QueryRow(`SELECT id FROM user WHERE name = ?`, userName).Scan(&userID); err != nil {
			respondError(c, err)
			return
		}

		// Insert run
		res, err := db.Exec(
			`INSERT INTO run (user_id, version_id, name) VALUES (?, ?, ?)`,
			userID, versionID, runName,
		)
		if err != nil {
			respondError(c, err)
			return
		}
		runID64, _ := res.LastInsertId()
		runID := int(runID64)

		// Insert run_progress
		if _, err := db.Exec(`INSERT INTO run_progress (run_id) VALUES (?)`, runID); err != nil {
			respondError(c, err)
			return
		}

		// Insert one run_rule per rule_def (all disabled)
		if _, err := db.Exec(`
			INSERT INTO run_rule (run_id, rule_def_id, enabled)
			SELECT ?, id, 0 FROM rule_def
		`, runID); err != nil {
			respondError(c, err)
			return
		}

		// Insert starter into run_pokemon: slot 1, level 5, on the party.
		if _, err := db.Exec(
			`INSERT INTO run_pokemon (run_id, form_id, level, is_alive, in_party, party_slot, moves_json)
			 VALUES (?, ?, 5, 1, 1, 1, '[]')`,
			runID, starterFormID,
		); err != nil {
			respondError(c, err)
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

// ArchiveRun handles POST /runs/:run_id/archive — soft-archives the run.
func ArchiveRun(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		if _, err := db.Exec(
			`UPDATE run SET archived_at = datetime('now') WHERE id = ?`, run.ID,
		); err != nil {
			respondError(c, err)
			return
		}
		c.Redirect(http.StatusFound, "/runs")
	}
}

// UnarchiveRun handles POST /runs/:run_id/unarchive — restores an archived run.
func UnarchiveRun(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		if _, err := db.Exec(
			`UPDATE run SET archived_at = NULL WHERE id = ?`, run.ID,
		); err != nil {
			respondError(c, err)
			return
		}
		c.Redirect(http.StatusFound, "/runs")
	}
}

package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/TravisPenn/professor-arbortom/internal/services"
	"github.com/gin-gonic/gin"
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
				SelectedVersionID: versionID,
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

		// Insert starter into run_pokemon: slot 1, level 5, on the party.
		if _, err := db.Exec(
			`INSERT INTO run_pokemon (run_id, form_id, level, caught_level, acquisition_type,
			 is_alive, in_party, party_slot, moves_json)
			 VALUES (?, ?, 5, 5, 'starter', 1, 1, 1, '[]')`,
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
			// SEC-011: deduplicated, tracked goroutine
			pokeClient.GoEnsurePokemon(db, starterFormID, versionGroupID)
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(runID)+"/overview")
	}
}

// ShowRun redirects /runs/:run_id → /runs/:run_id/overview
func ShowRun(c *gin.Context) {
	run := c.MustGet("run").(models.Run)
	c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/overview")
}

// ShowOverview renders GET /runs/:run_id/overview — a single-page summary dashboard.
func ShowOverview(db *sql.DB, pokeClient *pokeapi.Client, zc *services.CoachClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		progress := c.MustGet("progress").(models.RunProgress)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		// ── Progress: current location name + HM flags ─────────────────────────
		var currentLocName string
		if progress.CurrentLocationID != nil {
			db.QueryRow(`SELECT name FROM location WHERE id = ?`, *progress.CurrentLocationID).
				Scan(&currentLocName) //nolint:errcheck
			currentLocName = humanizeLocationName(currentLocName)
		}

		allFlags, activeFlags, _ := loadFlags(db, run.ID, run.VersionID)
		var flagLabels []string
		for _, fd := range gen3FlagDefs {
			if activeFlags[fd.Key] {
				flagLabels = append(flagLabels, fd.Label)
			}
		}

		// ── Locations (for inline progress editing) ───────────────────────────
		locations, _ := loadLocations(db, run.VersionID)
		hasPokeAPILocations := false
		for _, l := range locations {
			if l.ID > 0 {
				hasPokeAPILocations = true
				break
			}
		}
		seeding := false
		if !hasPokeAPILocations && pokeClient != nil {
			regionID := pokeapi.RegionIDForVersionID(run.VersionID)
			if regionID != 0 {
				seeding = true
				pokeClient.GoEnsureRegionLocations(db, regionID)
			}
		}
		hydTotal, hydSeeded, _ := getHydrationStats(db, run.VersionID)
		if !seeding && hydTotal > 0 && hydSeeded < hydTotal && pokeClient != nil {
			pokeClient.GoEnsureAllEncounters(db, run.VersionID)
		}

		// ── Team: slots 1-6 ───────────────────────────────────────────────────
		slots := [6]OverviewSlot{}
		for i := range slots {
			slots[i].Slot = i + 1
		}
		teamRows, err := db.Query(`
			SELECT rp.party_slot, p.species_name, rp.level
			FROM run_pokemon rp
			JOIN pokemon p ON p.id = rp.form_id
			WHERE rp.run_id = ? AND rp.in_party = 1 AND rp.is_alive = 1
			ORDER BY rp.party_slot
		`, run.ID)
		if err == nil {
			defer teamRows.Close()
			for teamRows.Next() {
				var s, lvl int
				var name string
				if teamRows.Scan(&s, &name, &lvl) == nil && s >= 1 && s <= 6 {
					slots[s-1] = OverviewSlot{Slot: s, SpeciesName: capitalizeVersion(name), Level: lvl}
				}
			}
		}

		// ── Box stats ─────────────────────────────────────────────────────────
		var alive, fainted int
		db.QueryRow(`SELECT COUNT(*) FROM run_pokemon WHERE run_id = ? AND in_party = 0 AND is_alive = 1`, run.ID).Scan(&alive) //nolint:errcheck
		db.QueryRow(`SELECT COUNT(*) FROM run_pokemon WHERE run_id = ? AND is_alive = 0`, run.ID).Scan(&fainted)                //nolint:errcheck

		// ── Recent route log (last 5) ─────────────────────────────────────────
		recentRoutes, _ := loadRouteLog(db, run.ID, false)
		if len(recentRoutes) > 5 {
			recentRoutes = recentRoutes[:5]
		}

		// ── Active rule keys ──────────────────────────────────────────────────
		var ruleKeys []string
		for _, r := range activeRules {
			if r.Enabled {
				ruleKeys = append(ruleKeys, r.Key)
			}
		}

		// ── Team slot list (slice for template) ──────────────────────────────
		slotList := make([]OverviewSlot, 6)
		copy(slotList, slots[:])

		page := OverviewPage{
			BasePage: BasePage{
				PageTitle:  "Overview",
				ActiveNav:  "overview",
				RunContext: buildRunContext(c),
			},
			CurrentLocationName: currentLocName,
			ActiveFlags:         flagLabels,
			TeamSlots:           slotList,
			BoxAlive:            alive,
			BoxFainted:          fainted,
			RecentRoutes:        recentRoutes,
			ActiveRules:         ruleKeys,
			CoachAvailable:      zc.IsAvailable(),
			// Inline progress editing
			Locations:        locations,
			CurrentLocID:     progress.CurrentLocationID,
			BadgeCount:       progress.BadgeCount,
			AllFlags:         allFlags,
			ActiveFlagMap:    activeFlags,
			LocationsSeeding: seeding,
			HydrationTotal:   hydTotal,
			HydrationSeeded:  hydSeeded,
		}
		c.HTML(http.StatusOK, "overview.html", page)
	}
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

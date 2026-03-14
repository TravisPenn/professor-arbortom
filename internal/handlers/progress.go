package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/gin-gonic/gin"
)

// ShowProgress renders GET /runs/:run_id/progress
func ShowProgress(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		progress := c.MustGet("progress").(models.RunProgress)

		locations, err := loadLocations(db, run.VersionID)
		if err != nil {
			respondError(c, err)
			return
		}

		// If no PokeAPI locations exist yet (positive IDs), trigger a background
		// seed from PokeAPI and render immediately — user refreshes once done (~10s).
		// Static town/city locations (negative IDs) do not count as seeded.
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
				// SEC-011: deduplicated, tracked goroutine
				pokeClient.GoEnsureRegionLocations(db, regionID)
			}
		}

		flags, activeFlags, err := loadFlags(db, run.ID, run.VersionID)
		if err != nil {
			respondError(c, err)
			return
		}

		// Hydration stats — how many location areas have encounter data cached.
		hydTotal, hydSeeded, _ := getHydrationStats(db, run.VersionID)

		// If locations exist but encounter seeding isn't complete, kick off a
		// background batch seeder so the progress bar actually advances.
		if !seeding && hydTotal > 0 && hydSeeded < hydTotal && pokeClient != nil {
			// SEC-011: deduplicated, tracked goroutine
			pokeClient.GoEnsureAllEncounters(db, run.VersionID)
		}

		page := ProgressPage{
			BasePage: BasePage{
				PageTitle:  "Progress",
				ActiveNav:  "progress",
				RunContext: buildRunContext(c),
			},
			Locations:        locations,
			CurrentLocID:     progress.CurrentLocationID,
			BadgeCount:       progress.BadgeCount,
			AllFlags:         flags,
			ActiveFlags:      activeFlags,
			LocationsSeeding: seeding,
			HydrationTotal:   hydTotal,
			HydrationSeeded:  hydSeeded,
		}

		c.HTML(http.StatusOK, "progress.html", page)
	}
}

// UpdateProgress handles POST /runs/:run_id/progress
func UpdateProgress(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)

		locationIDStr := c.PostForm("current_location_id")
		flags := c.PostFormArray("flags")

		badgeCount := formInt(c, "badge_count", 0)
		if badgeCount < 0 {
			badgeCount = 0
		}
		if badgeCount > 8 {
			badgeCount = 8
		}

		var locationID *int
		if locationIDStr != "" {
			if lid, err := strconv.Atoi(locationIDStr); err == nil && lid != 0 {
				locationID = &lid
			}
		}

		// Update progress
		if _, err := db.Exec(`
			INSERT INTO run_progress (run_id, badge_count, current_location_id, updated_at)
			VALUES (?, ?, ?, datetime('now'))
			ON CONFLICT(run_id) DO UPDATE SET
				badge_count = excluded.badge_count,
				current_location_id = excluded.current_location_id,
				updated_at = excluded.updated_at
		`, run.ID, badgeCount, locationID); err != nil {
			respondError(c, err)
			return
		}

		// SC-002 compatibility: also persist progress onto run when columns exist.
		if columnExists(db, "run", "badge_count") && columnExists(db, "run", "current_location_id") {
			if _, err := db.Exec(`
				UPDATE run
				SET badge_count = ?,
				    current_location_id = ?,
				    progress_updated_at = datetime('now')
				WHERE id = ?
			`, badgeCount, locationID, run.ID); err != nil {
				log.Printf("WARN: update run progress columns for run %d: %v", run.ID, err)
			}
		}

		// Rebuild flags: present in POST array = true, absent = false
		allFlags, _, _ := loadFlags(db, run.ID, run.VersionID)
		for _, fd := range allFlags {
			val := "false"
			for _, f := range flags {
				if f == fd.Key {
					val = "true"
					break
				}
			}
			// SEC-008: Flag writes are non-fatal — flags re-sync on next page load.
			// Keys come from the controlled rule_def table.
			if _, err := db.Exec(
				`INSERT OR REPLACE INTO run_flag (run_id, key, value) VALUES (?, ?, ?)`,
				run.ID, fd.Key, val,
			); err != nil {
				log.Printf("WARN: write flag %s for run %d: %v", fd.Key, run.ID, err)
			}

			// SC-003 compatibility: mirror flags into run_setting when available.
			if tableExists(db, "run_setting") {
				if _, err := db.Exec(
					`INSERT OR REPLACE INTO run_setting (run_id, type, key, value) VALUES (?, 'flag', ?, ?)`,
					run.ID, fd.Key, val,
				); err != nil {
					log.Printf("WARN: write run_setting flag %s for run %d: %v", fd.Key, run.ID, err)
				}
			}
		}

		// Background: seed location encounters from PokeAPI (skip static towns — negative IDs)
		if locationID != nil && *locationID > 0 && pokeClient != nil {
			// SEC-011: deduplicated, tracked goroutine
			pokeClient.GoEnsureLocationEncounters(db, *locationID, run.VersionID)
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/progress")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// HydrationStatus handles GET /runs/:run_id/progress/hydration
// Returns JSON {"total":N,"seeded":N} for lightweight polling.
func HydrationStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		total, seeded, err := getHydrationStats(db, run.VersionID)
		if err != nil {
			log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": apiErrorMsg(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"total": total, "seeded": seeded})
	}
}

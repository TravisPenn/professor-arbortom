package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pennt/pokemonprofessor/internal/models"
	"github.com/pennt/pokemonprofessor/internal/pokeapi"
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

		// If no locations yet, trigger a background seed from PokeAPI and
		// render immediately — user refreshes once seeding is done (~10s).
		seeding := false
		if len(locations) == 0 && pokeClient != nil {
			regionID := pokeapi.RegionIDForVersionID(run.VersionID)
			if regionID != 0 {
				seeding = true
				go func(rid, vid int) {
					_ = pokeClient.EnsureRegionLocations(db, rid)
				}(regionID, run.VersionID)
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
			go pokeClient.EnsureAllEncounters(db, run.VersionID)
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
		version := c.MustGet("version").(models.GameVersion)

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
			if lid, err := strconv.Atoi(locationIDStr); err == nil && lid > 0 {
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
			db.Exec( //nolint:errcheck
				`INSERT OR REPLACE INTO run_flag (run_id, key, value) VALUES (?, ?, ?)`,
				run.ID, fd.Key, val,
			)
		}

		// Background: seed location encounters from PokeAPI
		if locationID != nil && pokeClient != nil {
			go func(locID, versionID, vgID int) {
				_ = pokeClient.EnsureLocationEncounters(db, locID, versionID)
			}(*locationID, run.VersionID, version.VersionGroupID)
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"total": total, "seeded": seeded})
	}
}

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
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
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
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
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

		badgeCountStr := c.PostForm("badge_count")
		locationIDStr := c.PostForm("current_location_id")
		flags := c.PostFormArray("flags")

		badgeCount, _ := strconv.Atoi(badgeCountStr)
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
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
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

func loadLocations(db *sql.DB, versionID int) ([]LocationOption, error) {
	rows, err := db.Query(
		`SELECT id, name, region FROM location WHERE version_id = ? ORDER BY region, name`,
		versionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locs []LocationOption
	for rows.Next() {
		var l LocationOption
		if err := rows.Scan(&l.ID, &l.Name, &l.Region); err != nil {
			return nil, err
		}
		locs = append(locs, l)
	}
	return locs, rows.Err()
}

// gen3FlagDefs defines known story flags for all Gen 3 games.
// These are displayed as checkboxes on the progress screen.
var gen3FlagDefs = []FlagDef{
	{Key: "hm.cut_obtained", Label: "Got HM Cut", Description: "Enables Cut on the field and unlocks cut-locked areas"},
	{Key: "hm.fly_obtained", Label: "Got HM Fly", Description: "Enables Fly between towns"},
	{Key: "hm.surf_obtained", Label: "Got HM Surf", Description: "Enables Surf encounters and water routes"},
	{Key: "hm.strength_obtained", Label: "Got HM Strength", Description: "Enables moving boulders"},
	{Key: "hm.flash_obtained", Label: "Got HM Flash", Description: "Enables Flash in dark areas"},
	{Key: "hm.rock_smash_obtained", Label: "Got HM Rock Smash", Description: "Enables smashing rocks"},
	{Key: "hm.waterfall_obtained", Label: "Got HM Waterfall", Description: "Enables climbing waterfalls"},
	{Key: "hm.dive_obtained", Label: "Got HM Dive", Description: "Enables diving (Emerald / R/S only)"},
}

func loadFlags(db *sql.DB, runID, versionID int) ([]FlagDef, map[string]bool, error) {
	rows, err := db.Query(
		`SELECT key, value FROM run_flag WHERE run_id = ?`, runID,
	)
	if err != nil {
		return gen3FlagDefs, map[string]bool{}, err
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err == nil {
			active[key] = val == "true"
		}
	}

	return gen3FlagDefs, active, nil
}

// getHydrationStats returns the total number of location areas for a game version
// and how many of those areas have encounter data cached in api_cache_log.
func getHydrationStats(db *sql.DB, versionID int) (total, seeded int, err error) {
	if err = db.QueryRow(
		`SELECT COUNT(*) FROM location WHERE version_id = ?`, versionID,
	).Scan(&total); err != nil {
		return
	}
	err = db.QueryRow(`
		SELECT COUNT(DISTINCT l.id)
		FROM location l
		JOIN api_cache_log a ON a.resource = 'location-area' AND a.resource_id = l.id
		WHERE l.version_id = ?
	`, versionID).Scan(&seeded)
	return
}

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

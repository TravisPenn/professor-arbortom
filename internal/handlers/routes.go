package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pennt/pokemonprofessor/internal/models"
)

// ShowRoutes renders GET /runs/:run_id/routes
func ShowRoutes(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		nuzlockeOn := false
		for _, r := range activeRules {
			if r.Key == "nuzlocke" && r.Enabled {
				nuzlockeOn = true
			}
		}

		log, err := loadRouteLog(db, run.ID, nuzlockeOn)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		locations, err := loadLocations(db, run.VersionID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}

		page := RoutesPage{
			BasePage:   BasePage{PageTitle: "Routes", ActiveNav: "routes", RunContext: buildRunContext(c)},
			Log:        log,
			Locations:  locations,
			NuzlockeOn: nuzlockeOn,
		}
		c.HTML(http.StatusOK, "routes.html", page)
	}
}

// LogEncounter handles POST /runs/:run_id/routes
func LogEncounter(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		nuzlockeOn := false
		for _, r := range activeRules {
			if r.Key == "nuzlocke" && r.Enabled {
				nuzlockeOn = true
			}
		}

		locationIDStr := c.PostForm("location_id")
		speciesName := c.PostForm("form_id") // free text from form
		outcome := c.PostForm("outcome")
		levelStr := c.PostForm("level")

		locationID, _ := strconv.Atoi(locationIDStr)
		level, _ := strconv.Atoi(levelStr)

		// Resolve form ID from species name (best-effort)
		var formID int
		db.QueryRow(`
			SELECT pf.id FROM pokemon_form pf
			JOIN pokemon_species ps ON ps.id = pf.species_id
			WHERE ps.name = ? LIMIT 1
		`, speciesName).Scan(&formID) //nolint:errcheck

		// Nuzlocke duplicate check
		if nuzlockeOn && outcome == "caught" && locationID > 0 {
			var prevSpecies string
			err := db.QueryRow(`
				SELECT ps.name FROM run_pokemon rp
				JOIN pokemon_form pf ON pf.id = rp.form_id
				JOIN pokemon_species ps ON ps.id = pf.species_id
				WHERE rp.run_id = ? AND rp.met_location_id = ?
				LIMIT 1
			`, run.ID, locationID).Scan(&prevSpecies)

			if err == nil && prevSpecies != "" {
				// Duplicate — re-render with warning
				var locName string
				db.QueryRow(`SELECT name FROM location WHERE id = ?`, locationID).Scan(&locName) //nolint:errcheck

				log, _ := loadRouteLog(db, run.ID, nuzlockeOn)
				locations, _ := loadLocations(db, run.VersionID)

				page := RoutesPage{
					BasePage: BasePage{
						PageTitle:  "Routes",
						ActiveNav:  "routes",
						RunContext: buildRunContext(c),
					},
					Log:        log,
					Locations:  locations,
					NuzlockeOn: nuzlockeOn,
					DuplicateWarning: &DuplicateWarning{
						LocationName:  locName,
						PreviousCatch: prevSpecies,
					},
					FormLocationID: locationID,
					FormSpecies:    speciesName,
					FormOutcome:    outcome,
					FormLevel:      level,
				}
				c.HTML(http.StatusOK, "routes.html", page)
				return
			}
		}

		// Insert box entry if caught
		var metLocPtr interface{} = nil
		if locationID > 0 {
			metLocPtr = locationID
		}

		if outcome == "caught" && formID > 0 {
			db.Exec(`
				INSERT INTO run_pokemon (run_id, form_id, level, met_location_id, is_alive)
				VALUES (?, ?, ?, ?, 1)
			`, run.ID, formID, level, metLocPtr) //nolint:errcheck
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/routes")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func loadRouteLog(db *sql.DB, runID int, nuzlockeOn bool) ([]RouteEntry, error) {
	// Build a log from run_pokemon entries
	rows, err := db.Query(`
		SELECT
			COALESCE(l.name, 'unknown') AS loc_name,
			ps.name AS species_name,
			'caught' AS outcome,
			rp.level,
			rp.met_location_id
		FROM run_pokemon rp
		JOIN pokemon_form pf ON pf.id = rp.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		LEFT JOIN location l ON l.id = rp.met_location_id
		WHERE rp.run_id = ?
		ORDER BY rp.id DESC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build duplicate set if Nuzlocke
	duplicateLocations := make(map[int]int) // locationID → count
	if nuzlockeOn {
		dupRows, err := db.Query(
			`SELECT met_location_id FROM run_pokemon WHERE run_id = ? AND met_location_id IS NOT NULL`,
			runID,
		)
		if err == nil {
			defer dupRows.Close()
			for dupRows.Next() {
				var lid int
				if dupRows.Scan(&lid) == nil {
					duplicateLocations[lid]++
				}
			}
		}
	}

	var log []RouteEntry
	for rows.Next() {
		var e RouteEntry
		var locID *int
		if err := rows.Scan(&e.LocationName, &e.SpeciesName, &e.Outcome, &e.Level, &locID); err != nil {
			continue
		}
		if nuzlockeOn && locID != nil && duplicateLocations[*locID] > 1 {
			e.IsDuplicate = true
		}
		log = append(log, e)
	}
	return log, rows.Err()
}

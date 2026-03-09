package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pennt/pokemonprofessor/internal/models"
)

// ShowRoutes renders GET /runs/:run_id/routes
func ShowRoutes(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		nuzlockeOn := isRuleEnabled(activeRules, "nuzlocke")

		log, err := loadRouteLog(db, run.ID, nuzlockeOn)
		if err != nil {
			respondError(c, err)
			return
		}

		locations, err := loadLocations(db, run.VersionID)
		if err != nil {
			respondError(c, err)
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

		nuzlockeOn := isRuleEnabled(activeRules, "nuzlocke")

		locationID := formInt(c, "location_id", 0)
		speciesName := c.PostForm("form_id") // free text from form
		outcome := c.PostForm("outcome")
		level := formInt(c, "level", 0)

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

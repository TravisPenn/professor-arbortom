package handlers

import (
	"database/sql"
	"net/http"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/gin-gonic/gin"
)

// ShowRoutes renders GET /runs/:run_id/routes
func ShowRoutes(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
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

		encounters, _ := loadEncountersByLocation(db, run.VersionID)

		// Trigger background seeding if encounter data is not yet cached.
		if pokeClient != nil {
			pokeClient.GoEnsureAllEncounters(db, run.VersionID)
		}

		page := RoutesPage{
			BasePage:             BasePage{PageTitle: "Routes", ActiveNav: "routes", RunContext: buildRunContext(c)},
			Log:                  log,
			Locations:            locations,
			EncountersByLocation: encounters,
			NuzlockeOn:           nuzlockeOn,
		}
		c.HTML(http.StatusOK, "routes.html", page)
	}
}

// LogEncounter handles POST /runs/:run_id/routes
func LogEncounter(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		nuzlockeOn := isRuleEnabled(activeRules, "nuzlocke")

		locationID := formInt(c, "location_id", 0)
		speciesName := c.PostForm("form_id") // free text from form
		outcome := c.PostForm("outcome")
		level := formInt(c, "level", 0)

		// SEC-009: Limit species name length.
		if len(speciesName) > 100 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Species name is too long"})
			return
		}

		// Resolve form ID from species name (best-effort, case-insensitive).
		var formID int
		db.QueryRow(`
			SELECT pf.id FROM pokemon_form pf
			JOIN pokemon_species ps ON ps.id = pf.species_id
			WHERE LOWER(ps.name) = LOWER(?) LIMIT 1
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
				encounters, _ := loadEncountersByLocation(db, run.VersionID)

				page := RoutesPage{
					BasePage: BasePage{
						PageTitle:  "Routes",
						ActiveNav:  "routes",
						RunContext: buildRunContext(c),
					},
					Log:                  log,
					Locations:            locations,
					EncountersByLocation: encounters,
					NuzlockeOn:           nuzlockeOn,
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

		// Validate level before attempting the DB insert.
		if outcome == "caught" && (level < 1 || level > 100) {
			log, _ := loadRouteLog(db, run.ID, nuzlockeOn)
			locations, _ := loadLocations(db, run.VersionID)
			encounters, _ := loadEncountersByLocation(db, run.VersionID)

			page := RoutesPage{
				BasePage:             BasePage{PageTitle: "Routes", ActiveNav: "routes", RunContext: buildRunContext(c)},
				Log:                  log,
				Locations:            locations,
				EncountersByLocation: encounters,
				NuzlockeOn:           nuzlockeOn,
				ValidationError:      "Level is required (1–100) when catching a Pokémon.",
				FormLocationID:       locationID,
				FormSpecies:          speciesName,
				FormOutcome:          outcome,
			}
			c.HTML(http.StatusUnprocessableEntity, "routes.html", page)
			return
		}

		// Insert box entry if caught — upsert-or-merge to avoid duplicate rows.
		var metLocPtr interface{} = nil
		if locationID != 0 {
			metLocPtr = locationID
		}

		if outcome == "caught" && formID > 0 {
			// Check for a promotable existing row (Team-first, Routes-second case).
			var existingID int
			row := db.QueryRow(`
				SELECT id FROM run_pokemon
				WHERE run_id = ? AND form_id = ? AND is_alive = 1
				  AND acquisition_type IN ('manual', 'wild')
				ORDER BY id LIMIT 1
			`, run.ID, formID)
			if err := row.Scan(&existingID); err != nil && err != sql.ErrNoRows {
				respondError(c, err)
				return
			}

			if existingID > 0 {
				// Merge catch data into the existing row.
				if _, err := db.Exec(`
					UPDATE run_pokemon
					SET met_location_id  = ?,
					    caught_level     = ?,
					    acquisition_type = 'wild'
					WHERE id = ?
				`, metLocPtr, level, existingID); err != nil {
					respondError(c, err)
					return
				}
			} else {
				// Entirely new catch — insert.
				if _, err := db.Exec(`
					INSERT INTO run_pokemon (run_id, form_id, level, caught_level,
					    met_location_id, acquisition_type, is_alive)
					VALUES (?, ?, ?, ?, ?, 'wild', 1)
				`, run.ID, formID, level, level, metLocPtr); err != nil {
					respondError(c, err)
					return
				}
			}
		}

		// Trigger background encounter seeding for this specific location.
		if locationID > 0 && pokeClient != nil {
			pokeClient.GoEnsureLocationEncounters(db, locationID, run.VersionID)
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/routes")
	}
}

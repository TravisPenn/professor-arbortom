package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pennt/pokemonprofessor/internal/legality"
	"github.com/pennt/pokemonprofessor/internal/models"
	"github.com/pennt/pokemonprofessor/internal/services"
)

// ShowCoach renders GET /runs/:run_id/coach
func ShowCoach(db *sql.DB, zc *services.ZeroClaw) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		page, err := buildCoachPage(c, db, run.ID, zc.IsAvailable())
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}
		c.HTML(http.StatusOK, "coach.html", page)
	}
}

// QueryCoach handles POST /runs/:run_id/coach
func QueryCoach(db *sql.DB, zc *services.ZeroClaw) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		question := c.PostForm("question")

		available := zc.IsAvailable()
		page, err := buildCoachPage(c, db, run.ID, available)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": err.Error()})
			return
		}
		page.PlayerQuestion = question

		if !available || question == "" {
			c.HTML(http.StatusOK, "coach.html", page)
			return
		}

		// Build candidate payload
		acqs, _, _ := legality.LegalAcquisitions(db, run.ID)
		items, _ := legality.LegalItems(db, run.ID)

		payload := services.CoachPayload{
			Candidates: map[string]interface{}{
				"acquisitions": acqs,
				"items":        items,
				"party_moves":  page.PartyMoves,
			},
			Question: question,
		}

		response := zc.QueryCoach(run.ID, payload)

		if response.Available {
			page.CoachAnswer = &CoachAnswer{
				Text:      response.Answer,
				Model:     response.Model,
				Truncated: response.Truncated,
			}
		} else {
			page.ZeroClawAvailable = false
		}

		c.HTML(http.StatusOK, "coach.html", page)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildCoachPage(c *gin.Context, db *sql.DB, runID int, available bool) (CoachPage, error) {
	acqs, _, err := legality.LegalAcquisitions(db, runID)
	if err != nil {
		return CoachPage{}, err
	}

	items, err := legality.LegalItems(db, runID)
	if err != nil {
		return CoachPage{}, err
	}
	var itemOptions []ItemOption
	for _, it := range items {
		itemOptions = append(itemOptions, itemToOption(it))
	}

	// Summarize party moves
	rows, err := db.Query(
		`SELECT party_slot, form_id FROM run_pokemon WHERE run_id = ? AND in_party = 1 ORDER BY party_slot`,
		runID,
	)
	if err != nil {
		return CoachPage{}, err
	}
	defer rows.Close()

	var party []PartyMoveSummary
	for rows.Next() {
		var slot, formID int
		if err := rows.Scan(&slot, &formID); err != nil {
			continue
		}

		var speciesName string
		db.QueryRow(`
			SELECT ps.name FROM pokemon_form pf
			JOIN pokemon_species ps ON ps.id = pf.species_id
			WHERE pf.id = ?
		`, formID).Scan(&speciesName) //nolint:errcheck

		mvs, _, _ := legality.LegalMoves(db, runID, formID)
		var moveOpts []MoveOption
		for _, mv := range mvs {
			moveOpts = append(moveOpts, moveToOption(mv))
		}

		party = append(party, PartyMoveSummary{
			Slot:        slot,
			SpeciesName: speciesName,
			Moves:       moveOpts,
		})
	}

	return CoachPage{
		BasePage: BasePage{
			PageTitle:  "Coach",
			ActiveNav:  "coach",
			RunContext: buildRunContext(c),
		},
		ZeroClawAvailable: available,
		Acquisitions:      acqs,
		PartyMoves:        party,
		LegalItems:        itemOptions,
	}, nil
}

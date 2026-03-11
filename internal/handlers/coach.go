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
			respondError(c, err)
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
			respondError(c, err)
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

	trades, err := legality.LegalTrades(db, runID)
	if err != nil {
		return CoachPage{}, err
	}
	var tradeOptions []TradeOption
	for _, t := range trades {
		tradeOptions = append(tradeOptions, TradeOption{
			Method:         t.Method,
			GiveSpecies:    t.GiveSpecies,
			ReceiveSpecies: t.ReceiveSpecies,
			ReceiveNick:    t.ReceiveNick,
			PriceCoins:     t.PriceCoins,
			Notes:          t.Notes,
		})
	}

	ownedItems, err := legality.LegalItems(db, runID)
	if err != nil {
		return CoachPage{}, err
	}
	shopItems, err := legality.ShopItems(db, runID)
	if err != nil {
		return CoachPage{}, err
	}
	var itemOptions []ItemOption
	for _, it := range append(ownedItems, shopItems...) {
		itemOptions = append(itemOptions, itemToOption(it))
	}

	// Summarize party moves — select level as well.
	rows, err := db.Query(
		`SELECT party_slot, form_id, level FROM run_pokemon WHERE run_id = ? AND in_party = 1 ORDER BY party_slot`,
		runID,
	)
	if err != nil {
		return CoachPage{}, err
	}
	defer rows.Close()

	var party []PartyMoveSummary
	for rows.Next() {
		var slot, formID, level int
		if err := rows.Scan(&slot, &formID, &level); err != nil {
			continue
		}

		var speciesName string
		db.QueryRow(`
			SELECT ps.name FROM pokemon_form pf
			JOIN pokemon_species ps ON ps.id = pf.species_id
			WHERE pf.id = ?
		`, formID).Scan(&speciesName) //nolint:errcheck

		mvs, _ := legality.CoachMoves(db, runID, formID, level)
		var moveOpts []MoveOption
		for _, mv := range mvs {
			moveOpts = append(moveOpts, moveToOption(mv))
		}

		party = append(party, PartyMoveSummary{
			Slot:        slot,
			Level:       level,
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
		Trades:            tradeOptions,
		PartyMoves:        party,
		LegalItems:        itemOptions,
	}, nil
}

package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/TravisPenn/professor-arbortom/internal/legality"
	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/TravisPenn/professor-arbortom/internal/services"
	"github.com/gin-gonic/gin"
)

// ShowCoach renders GET /runs/:run_id/coach
func ShowCoach(db *sql.DB, pokeClient *pokeapi.Client, zc *services.CoachClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		page, err := buildCoachPage(c, db, pokeClient, run.ID, zc.IsAvailable())
		if err != nil {
			respondError(c, err)
			return
		}
		c.HTML(http.StatusOK, "coach.html", page)
	}
}

// QueryCoach handles POST /runs/:run_id/coach
func QueryCoach(db *sql.DB, pokeClient *pokeapi.Client, zc *services.CoachClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		question := c.PostForm("question")

		// SEC-009: Limit question length to prevent token abuse on LLM gateway.
		if len(question) > 2000 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Question must be 2000 characters or fewer"})
			return
		}

		available := zc.IsAvailable()
		page, err := buildCoachPage(c, db, pokeClient, run.ID, available)
		if err != nil {
			respondError(c, err)
			return
		}
		page.PlayerQuestion = question

		if !available || question == "" {
			c.HTML(http.StatusOK, "coach.html", page)
			return
		}

		payload, err := buildCoachPayload(db, run.ID, page, question)
		if err != nil {
			respondError(c, err)
			return
		}

		response := zc.QueryCoach(run.ID, payload)

		if response.Available {
			page.CoachAnswer = &CoachAnswer{
				Text:      response.Answer,
				Model:     response.Model,
				Truncated: response.Truncated,
			}
		} else {
			page.CoachAvailable = false
		}

		c.HTML(http.StatusOK, "coach.html", page)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func buildCoachPage(c *gin.Context, db *sql.DB, pokeClient *pokeapi.Client, runID int, available bool) (CoachPage, error) {
	acqs, _, err := legality.LegalAcquisitions(db, runID)
	if err != nil {
		return CoachPage{}, err
	}

	// CD-001: Deduplicate acquisitions by (species, form, location), merging level ranges.
	type acqKey struct{ species, form, location string }
	grouped := map[acqKey]*legality.Acquisition{}
	var groupOrder []acqKey
	for _, a := range acqs {
		k := acqKey{a.SpeciesName, a.FormName, a.LocationName}
		if g, ok := grouped[k]; ok {
			if a.MinLevel < g.MinLevel {
				g.MinLevel = a.MinLevel
			}
			if a.MaxLevel > g.MaxLevel {
				g.MaxLevel = a.MaxLevel
			}
		} else {
			copy := a
			grouped[k] = &copy
			groupOrder = append(groupOrder, k)
		}
	}
	acqs = nil
	for _, k := range groupOrder {
		acqs = append(acqs, *grouped[k])
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
		db.QueryRow(`SELECT species_name FROM pokemon WHERE id = ?`, formID).Scan(&speciesName) //nolint:errcheck

		// Trigger background seeding for direct evolution targets that lack
		// learnset data — ensures evo-note and evo-exclusive columns are populated.
		if pokeClient != nil {
			var vgID int
			db.QueryRow(`SELECT gv.version_group_id FROM run r JOIN game_version gv ON gv.id = r.version_id WHERE r.id = ?`, runID).Scan(&vgID) //nolint:errcheck
			if vgID > 0 {
				evoTargets, _ := db.Query(`SELECT DISTINCT to_form_id FROM evolution_condition WHERE from_form_id = ?`, formID)
				if evoTargets != nil {
					for evoTargets.Next() {
						var eid int
						if evoTargets.Scan(&eid) == nil {
							var cnt int
							db.QueryRow(`SELECT COUNT(*) FROM learnset_entry WHERE form_id = ?`, eid).Scan(&cnt) //nolint:errcheck
							if cnt == 0 {
								pokeClient.GoEnsurePokemon(db, eid, vgID)
							}
						}
					}
					evoTargets.Close()
				}
			}
		}

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

	insights, _ := buildTeamInsights(db, runID)
	opponents, _ := nextOpponents(db, runID)

	return CoachPage{
		BasePage: BasePage{
			PageTitle:  "Coach",
			ActiveNav:  "coach",
			RunContext: buildRunContext(c),
		},
		CoachAvailable: available,
		Acquisitions:   acqs,
		Trades:         tradeOptions,
		PartyMoves:     party,
		LegalItems:     itemOptions,
		TeamInsights:   insights,
		NextOpponents:  opponents,
	}, nil
}

// buildTeamInsights computes per-member detail and team-wide coverage analysis.
// Returns nil if the party is empty or the query fails.
func buildTeamInsights(db *sql.DB, runID int) (*TeamInsights, error) {
	type partyRow struct {
		pkmnID  int
		slot    int
		formID  int
		level   int
		species string
	}

	rows, err := db.Query(`
		SELECT rp.id, rp.party_slot, rp.form_id, rp.level, p.species_name
		FROM run_pokemon rp
		JOIN pokemon p ON p.id = rp.form_id
		WHERE rp.run_id = ? AND rp.in_party = 1
		ORDER BY rp.party_slot
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pRows []partyRow
	for rows.Next() {
		var r partyRow
		if err := rows.Scan(&r.pkmnID, &r.slot, &r.formID, &r.level, &r.species); err != nil {
			continue
		}
		pRows = append(pRows, r)
	}
	if len(pRows) == 0 {
		return nil, nil
	}

	var members []PartyDetailPayload
	var teamMembers []legality.TeamMember

	for _, pr := range pRows {
		detail := PartyDetailPayload{
			Slot:         pr.slot,
			RunPokemonID: pr.pkmnID,
			SpeciesName:  capitalizeVersion(pr.species),
			Level:        pr.level,
		}

		// Types from pokemon.type1/type2 (SC-001)
		var pType1, pType2 sql.NullString
		db.QueryRow(`SELECT type1, type2 FROM pokemon WHERE id = ?`, pr.formID).Scan(&pType1, &pType2) //nolint:errcheck
		var types []string
		if pType1.Valid && pType1.String != "" {
			types = append(types, pType1.String)
		}
		if pType2.Valid && pType2.String != "" {
			types = append(types, pType2.String)
		}
		detail.Types = types

		// Base stats from pokemon table (SC-001)
		var stats legality.BaseStats
		if err := db.QueryRow(`SELECT hp, attack, defense, sp_attack, sp_defense, speed FROM pokemon WHERE id = ?`, pr.formID).
			Scan(&stats.HP, &stats.Attack, &stats.Defense, &stats.SpAttack, &stats.SpDefense, &stats.Speed); err == nil {
			detail.BaseStats = &stats
		}

		// Primary ability from pokemon.ability1 (SC-001)
		var abilityName sql.NullString
		db.QueryRow(`SELECT ability1 FROM pokemon WHERE id = ?`, pr.formID).Scan(&abilityName) //nolint:errcheck
		detail.Ability = abilityName.String

		// Current moves from run_pokemon_move (COACH-005)
		moveRows, _ := db.Query(`
			SELECT m.name, m.type_name, m.power, m.accuracy, m.pp
			FROM run_pokemon_move rpm
			JOIN move m ON m.id = rpm.move_id
			WHERE rpm.run_pokemon_id = ?
			ORDER BY rpm.slot
		`, pr.pkmnID)
		var currentMoves []MoveDetail
		if moveRows != nil {
			for moveRows.Next() {
				var md MoveDetail
				var power, accuracy sql.NullInt64
				moveRows.Scan(&md.Name, &md.TypeName, &power, &accuracy, &md.PP) //nolint:errcheck
				if power.Valid {
					v := int(power.Int64)
					md.Power = &v
				}
				if accuracy.Valid {
					v := int(accuracy.Int64)
					md.Accuracy = &v
				}
				currentMoves = append(currentMoves, md)
			}
			moveRows.Close()
		}
		detail.CurrentMoves = currentMoves
		members = append(members, detail)

		// Build legality.TeamMember for coverage analysis.
		tm := legality.TeamMember{SpeciesName: pr.species}
		if len(types) > 0 {
			tm.Type1 = types[0]
		}
		if len(types) > 1 {
			tm.Type2 = types[1]
		}
		for _, mv := range currentMoves {
			tm.MoveTypes = append(tm.MoveTypes, mv.TypeName)
		}
		teamMembers = append(teamMembers, tm)
	}

	// Defensive / offensive coverage (COACH-002).
	var weaknesses []legality.TypeThreat
	var resistances, immunities, uncovered []string
	for _, m := range teamMembers {
		if m.Type1 != "" {
			profile := legality.TeamDefensiveProfile(teamMembers)
			weaknesses = profile.Weaknesses
			resistances = profile.Resistances
			immunities = profile.Immunities
			uncovered = profile.Uncovered
			break
		}
	}

	// Evolution paths (COACH-003).
	evoPaths := make(map[int][]legality.EvolutionPath)
	var evoSummaries []EvoSummary
	evoGraph, _ := legality.LoadEvolutionGraph(db)
	rs, _ := legality.LoadRunState(db, runID)
	if evoGraph != nil && rs != nil {
		levelCap, _ := legality.LevelCap(db, rs)
		for _, pr := range pRows {
			paths := legality.FindEvolutionPaths(evoGraph, pr.formID, rs, levelCap)
			if len(paths) > 0 {
				evoPaths[pr.formID] = paths
				evoSummaries = append(evoSummaries, EvoSummary{
					Slot:        pr.slot,
					FormID:      pr.formID,
					SpeciesName: capitalizeVersion(pr.species),
					Paths:       paths,
				})
			}
		}
	}
	if len(evoPaths) == 0 {
		evoPaths = nil
	}

	return &TeamInsights{
		Members:        members,
		Weaknesses:     weaknesses,
		Resistances:    resistances,
		Immunities:     immunities,
		UncoveredTypes: uncovered,
		EvoSummaries:   evoSummaries,
		EvoPaths:       evoPaths,
	}, nil
}

// buildCoachPayload assembles the enriched CoachPayload for AI Coach (COACH-006).
// It reuses the TeamInsights already computed for the page to avoid extra DB queries.
func buildCoachPayload(db *sql.DB, runID int, page CoachPage, question string) (services.CoachPayload, error) {
	var versionName string
	db.QueryRow(`SELECT gv.name FROM run r JOIN game_version gv ON gv.id = r.version_id WHERE r.id = ?`, runID).Scan(&versionName) //nolint:errcheck

	contextNote := fmt.Sprintf(
		"Run data is %s. Team/type analysis is computed from verified DB data. "+
			"For general Pokemon knowledge (breeding, cross-gen), use your training knowledge.",
		capitalizeVersion(versionName),
	)

	var partyDetails []PartyDetailPayload
	var teamAnalysis *TeamAnalysisPayload
	var evolutionPaths map[int][]legality.EvolutionPath

	if page.TeamInsights != nil {
		partyDetails = page.TeamInsights.Members
		if len(page.TeamInsights.Weaknesses) > 0 || len(page.TeamInsights.Resistances) > 0 || len(page.TeamInsights.Immunities) > 0 {
			teamAnalysis = &TeamAnalysisPayload{
				Weaknesses:     page.TeamInsights.Weaknesses,
				Resistances:    page.TeamInsights.Resistances,
				Immunities:     page.TeamInsights.Immunities,
				UncoveredTypes: page.TeamInsights.UncoveredTypes,
			}
		}
		evolutionPaths = page.TeamInsights.EvoPaths
	}

	return services.CoachPayload{
		Candidates: services.CoachCandidates{
			Acquisitions:   page.Acquisitions,
			Items:          page.LegalItems,
			PartyMoves:     page.PartyMoves,
			TeamAnalysis:   teamAnalysis,
			EvolutionPaths: evolutionPaths,
			PartyDetails:   partyDetails,
			NextOpponents:  page.NextOpponents,
		},
		Question:    question,
		ContextNote: contextNote,
	}, nil
}

// nextOpponents returns up to 2 upcoming gym leaders / E4 members for the run's
// current version, ordered by badge_order.
// Returns nil, nil when the gym_leader table does not yet exist or no leaders remain.
func nextOpponents(db *sql.DB, runID int) ([]OpponentSummary, error) {
	if !tableExists(db, "gym_leader") {
		return nil, nil
	}

	var versionID, badgeCount int
	err := db.QueryRow(
		`SELECT version_id, COALESCE(badge_count, 0) FROM run WHERE id = ?`,
		runID,
	).Scan(&versionID, &badgeCount)
	if err != nil {
		return nil, nil
	}

	leaders, err := db.Query(`
		SELECT id, name, type_specialty, location_name, badge_order
		FROM gym_leader
		WHERE version_id = ? AND badge_order > ?
		ORDER BY badge_order
		LIMIT 2`,
		versionID, badgeCount,
	)
	if err != nil {
		return nil, nil
	}
	defer leaders.Close()

	var summaries []OpponentSummary
	for leaders.Next() {
		var leaderID, badgeOrder int
		var name, typeSpecialty, locationName string
		if err := leaders.Scan(&leaderID, &name, &typeSpecialty, &locationName, &badgeOrder); err != nil {
			continue
		}

		teamRows, err := db.Query(`
			SELECT p.species_name, glp.level, COALESCE(glp.held_item,''),
			       COALESCE(glp.move_1,''), COALESCE(glp.move_2,''),
			       COALESCE(glp.move_3,''), COALESCE(glp.move_4,''),
			       COALESCE(p.type1,''), COALESCE(p.type2,'')
			FROM gym_leader_pokemon glp
			JOIN pokemon p ON p.id = glp.form_id
			WHERE glp.gym_leader_id = ? AND glp.starter_variant IS NULL
			ORDER BY glp.slot`,
			leaderID,
		)
		if err != nil {
			continue
		}

		var team []OpponentPokemon
		for teamRows.Next() {
			var species, heldItem, m1, m2, m3, m4, t1, t2 string
			var level int
			if err := teamRows.Scan(&species, &level, &heldItem, &m1, &m2, &m3, &m4, &t1, &t2); err != nil {
				continue
			}
			op := OpponentPokemon{
				SpeciesName: species,
				Level:       level,
				HeldItem:    heldItem,
			}
			if t1 != "" {
				op.Types = append(op.Types, t1)
			}
			if t2 != "" {
				op.Types = append(op.Types, t2)
			}
			for _, mv := range []string{m1, m2, m3, m4} {
				if mv != "" {
					op.Moves = append(op.Moves, mv)
				}
			}
			team = append(team, op)
		}
		teamRows.Close()

		summaries = append(summaries, OpponentSummary{
			Name:          name,
			TypeSpecialty: typeSpecialty,
			LocationName:  locationName,
			BadgeOrder:    badgeOrder,
			Team:          team,
		})
	}

	return summaries, nil
}

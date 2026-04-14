package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/TravisPenn/professor-arbortom/data/walkthroughs"
	"github.com/TravisPenn/professor-arbortom/internal/legality"
	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/TravisPenn/professor-arbortom/internal/services"
	"github.com/gin-gonic/gin"
)

// defaultRecommendationPrompt is sent to the LLM on every page load when no
// user question is present, producing an automatic recommendation.
const defaultRecommendationPrompt = "Present ONLY the VERIFIED RECOMMENDATIONS above. " +
	"Output exactly one numbered tip per verified recommendation — no more, no fewer. " +
	"Keep the original meaning and tense: if it says 'Teach X to Y', say the player SHOULD teach it (future action), " +
	"NOT that the Pokémon already knows it. If it says 'evolves to', it is a FUTURE evolution, not one that already happened. " +
	"Do NOT add any recommendations, catches, evolutions, moves, or tips that are not in the VERIFIED RECOMMENDATIONS list. " +
	"Do NOT invent or change any names, types, levels, locations, or TM numbers. " +
	"Use **bold** for Pokémon, move, and item names. Number each tip."

// wrapUserQuestion wraps an open-ended player question with grounding instructions
// so the model answers strictly from the provided game state rather than hallucinating.
func wrapUserQuestion(q string) string {
	return "Answer this question using ONLY the game data provided above. " +
		"If the answer is not in the data, say you don't have that information. " +
		"Do NOT use outside knowledge about TM numbers, move names, locations, or types — " +
		"only reference what appears in the game state context.\n\nPlayer question: " + q
}

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

// GetRecommendation handles GET /runs/:run_id/coach/recommendation
// It is called asynchronously by the page after initial load.
// Optional ?q= param allows submitting a user question via the same endpoint.
func GetRecommendation(db *sql.DB, pokeClient *pokeapi.Client, zc *services.CoachClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		if !zc.IsAvailable() {
			c.String(http.StatusServiceUnavailable, "")
			return
		}

		question := c.Query("q")
		// SEC-009: Limit question length to prevent token abuse on LLM gateway.
		if len(question) > 2000 {
			c.String(http.StatusBadRequest, "Question must be 2000 characters or fewer")
			return
		}

		page, err := buildCoachPage(c, db, pokeClient, run.ID, true)
		if err != nil {
			c.String(http.StatusInternalServerError, "")
			return
		}

		prompt := defaultRecommendationPrompt
		if question != "" {
			prompt = wrapUserQuestion(question)
		}

		payload, err := buildCoachPayload(db, run.ID, page, prompt)
		if err != nil {
			c.String(http.StatusInternalServerError, "")
			return
		}
		response := zc.QueryCoach(run.ID, payload)
		if !response.Available {
			c.String(http.StatusServiceUnavailable, "")
			return
		}
		c.HTML(http.StatusOK, "coach-recommendation.html", CoachAnswer{
			Text:      response.Answer,
			Model:     response.Model,
			Truncated: response.Truncated,
			Question:  question,
		})
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

		payload, err := buildCoachPayload(db, run.ID, page, wrapUserQuestion(question))
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
			page.CoachError = "The Professor could not be reached right now. Please try again."
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

	// CD-001: Deduplicate acquisitions by (species, form, location, method),
	// merging level ranges. Method is part of the key so rod-locked fishing
	// encounters stay separate from NPC gifts at the same location.
	type acqKey struct{ species, form, location, method string }
	grouped := map[acqKey]*legality.Acquisition{}
	var groupOrder []acqKey
	for _, a := range acqs {
		k := acqKey{a.SpeciesName, a.FormName, a.LocationName, a.Method}
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
	sort.Slice(itemOptions, func(i, j int) bool {
		si, sj := sourceOrder(itemOptions[i].Source), sourceOrder(itemOptions[j].Source)
		if si != sj {
			return si < sj
		}
		if itemOptions[i].Category != itemOptions[j].Category {
			return itemOptions[i].Category < itemOptions[j].Category
		}
		return itemOptions[i].DisplayName < itemOptions[j].DisplayName
	})

	// Summarize party moves — select level as well.
	// Pre-compute TM locations from walkthrough for annotation.
	var versionName string
	db.QueryRow(`SELECT gv.name FROM run r JOIN game_version gv ON gv.id = r.version_id WHERE r.id = ?`, runID).Scan(&versionName) //nolint:errcheck
	tmLocs := walkthroughs.TMLocations(versionName)

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
			opt := moveToOption(mv)
			if opt.TMNumber > 0 && opt.TutorLocation == "" && tmLocs != nil {
				opt.TutorLocation = tmLocs[opt.TMNumber]
			}
			moveOpts = append(moveOpts, opt)
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

	// Humanize location names from legality engine (DB slugs → display names).
	for i := range acqs {
		acqs[i].LocationName = humanizeLocationName(acqs[i].LocationName)
	}

	return CoachPage{
		BasePage: BasePage{
			PageTitle:  "Professor Arbortom",
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

	var badgeCount int
	db.QueryRow(`SELECT COALESCE(badge_count, 0) FROM run WHERE id = ?`, runID).Scan(&badgeCount) //nolint:errcheck

	var currentLocationName string
	db.QueryRow(
		`SELECT COALESCE(l.name, '') FROM run r LEFT JOIN location l ON l.id = r.current_location_id WHERE r.id = ?`,
		runID,
	).Scan(&currentLocationName) //nolint:errcheck
	currentLocationName = humanizeLocationName(currentLocationName)

	var maxPartyLevel int
	for _, pm := range page.PartyMoves {
		if pm.Level > maxPartyLevel {
			maxPartyLevel = pm.Level
		}
	}

	contextNote := "Only suggest content accessible at the player's current badge count and party level. " +
		"Team/type analysis is computed from verified DB data. " +
		"For general Pokemon knowledge (breeding, cross-gen), use your training knowledge."

	// Load active rules so the LLM knows which constraints apply.
	var activeRules map[string]interface{}
	if rs, rsErr := legality.LoadRunState(db, runID); rsErr == nil && len(rs.ActiveRules) > 0 {
		activeRules = make(map[string]interface{}, len(rs.ActiveRules))
		for k := range rs.ActiveRules {
			params := rs.RuleParams[k] // nil if no extra params
			activeRules[k] = params
		}
	}

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
			ActiveRules:    activeRules,
		},
		Question:    question,
		ContextNote: contextNote,
		GameSummary: buildGameSummary(page, activeRules, versionName, badgeCount, maxPartyLevel, currentLocationName) +
			buildWalkthroughContext(versionName, badgeCount, currentLocationName, activeRules) +
			buildPreComputedRecommendations(db, runID, page, versionName, badgeCount, currentLocationName, activeRules),
	}, nil
}

// buildPreComputedRecommendations generates server-side verified recommendations
// from the structured game data. These are facts the LLM merely needs to present
// — it cannot hallucinate names, types, or levels because they're computed here.
func buildPreComputedRecommendations(db *sql.DB, runID int, page CoachPage, versionName string, badgeCount int, currentLocation string, activeRules map[string]interface{}) string {
	var recs []string

	// Determine next opponent info for type-relevance scoring
	var oppName, oppType string
	if len(page.NextOpponents) > 0 {
		opp := page.NextOpponents[0]
		oppName = opp.Name
		oppType = opp.TypeSpecialty
	}

	// 1. Best TM upgrade: find the single best TM a party member can learn
	//    that's super-effective or at least strong against the next opponent.
	//    Only consider TMs the player actually has access to right now (owned,
	//    in a shop at the current location, or a nearby pickup). This prevents
	//    recommending TMs that require backtracking past progression gates.
	legalItemTMs := make(map[int]string) // TMNumber -> source ("owned"/"shop"/"obtainable")
	for _, item := range page.LegalItems {
		lower := strings.ToLower(item.Name)
		if !strings.HasPrefix(lower, "tm") {
			continue
		}
		var tmn int
		if _, err := fmt.Sscanf(lower, "tm%d", &tmn); err == nil && tmn > 0 {
			legalItemTMs[tmn] = item.Source
		}
	}
	availableTMs := walkthroughs.AvailableTMs(versionName, badgeCount)
	type tmCandidate struct {
		species  string
		tmNum    int
		moveName string
		moveType string
		power    int
		location string // walkthrough location string, e.g. "Pewter Gym (Brock reward)"
	}
	var bestTM *tmCandidate
	for _, pm := range page.PartyMoves {
		currentBestByType := map[string]int{}
		if page.TeamInsights != nil {
			for _, m := range page.TeamInsights.Members {
				if capitalizeVersion(pm.SpeciesName) == m.SpeciesName {
					for _, mv := range m.CurrentMoves {
						if mv.Power != nil && *mv.Power > currentBestByType[mv.TypeName] {
							currentBestByType[mv.TypeName] = *mv.Power
						}
					}
					break
				}
			}
		}
		for _, mv := range pm.Moves {
			if mv.LearnMethod != "machine" || mv.TMNumber == 0 || mv.Power == nil || *mv.Power < 50 {
				continue
			}
			// Only recommend TMs the player can actually get right now.
			// legalItemTMs is built from page.LegalItems (owned bag, current shop,
			// or nearby pickup). This excludes TMs behind progression gates or
			// requiring a backtrack the player can't easily make.
			if _, accessible := legalItemTMs[mv.TMNumber]; !accessible {
				continue
			}
			// availableTMs provides a secondary badge-count check for versions
			// that have a TM reference table (FRLG). nil = RSE, allow through.
			if availableTMs != nil && !availableTMs[mv.TMNumber] {
				continue
			}
			if currentBest, ok := currentBestByType[mv.TypeName]; ok && currentBest >= *mv.Power {
				continue
			}
			if bestTM == nil || *mv.Power > bestTM.power {
				bestTM = &tmCandidate{
					species:  capitalizeVersion(pm.SpeciesName),
					tmNum:    mv.TMNumber,
					moveName: mv.Name,
					moveType: mv.TypeName,
					power:    *mv.Power,
					location: mv.TutorLocation, // populated from walkthrough TM table
				}
			}
		}
	}
	if bestTM != nil {
		rec := fmt.Sprintf("Teach TM%02d %s (%s-type, %d power) to %s.",
			bestTM.tmNum, bestTM.moveName, bestTM.moveType, bestTM.power, bestTM.species)
		// Annotate acquisition method using the source already known from legalItemTMs.
		switch legalItemTMs[bestTM.tmNum] {
		case "owned":
			rec += " (already in your bag)"
		case "shop":
			rec += " (buy at the Poké Mart)"
		case "obtainable":
			if bestTM.location != "" {
				rec += fmt.Sprintf(" (get from %s)", bestTM.location)
			} else {
				rec += " (pick up nearby)"
			}
		}
		if oppName != "" {
			rec += fmt.Sprintf(" Useful against %s's %s-type team.", oppName, oppType)
		}
		recs = append(recs, rec)
	}

	// 2. Best catch: find a catch that covers a team weakness
	weakTypes := map[string]bool{}
	if page.TeamInsights != nil {
		for _, w := range page.TeamInsights.Weaknesses {
			weakTypes[w.Type] = true
		}
	}
	type catchCandidate struct {
		species  string
		location string
		method   string
		minLv    int
		maxLv    int
	}
	var bestCatch *catchCandidate
	for _, a := range page.Acquisitions {
		if a.BlockedByRule != nil {
			continue
		}
		bestCatch = &catchCandidate{
			species:  capitalizeVersion(a.SpeciesName),
			location: a.LocationName,
			method:   humanizeMethod(a.Method),
			minLv:    a.MinLevel,
			maxLv:    a.MaxLevel,
		}
		break // take first available (they're already sorted by relevance)
	}
	if bestCatch != nil {
		lvl := fmt.Sprintf("Lv%d", bestCatch.minLv)
		if bestCatch.maxLv != bestCatch.minLv {
			lvl = fmt.Sprintf("Lv%d-%d", bestCatch.minLv, bestCatch.maxLv)
		}
		rec := fmt.Sprintf("Catch %s at %s (%s, %s).",
			bestCatch.species, bestCatch.location, lvl, bestCatch.method)
		recs = append(recs, rec)
	}

	// 3. Best trade option
	if len(page.Trades) > 0 {
		t := page.Trades[0]
		if t.GiveSpecies != "" {
			recs = append(recs, fmt.Sprintf("Trade %s to receive %s.",
				capitalizeVersion(t.GiveSpecies), capitalizeVersion(t.ReceiveSpecies)))
		} else if t.PriceCoins > 0 {
			recs = append(recs, fmt.Sprintf("Buy %s at the Game Corner for %d coins.",
				capitalizeVersion(t.ReceiveSpecies), t.PriceCoins))
		} else {
			recs = append(recs, fmt.Sprintf("Obtain %s via %s.",
				capitalizeVersion(t.ReceiveSpecies), t.Method))
		}
	}

	// 4. Evolution delay advice: only when delaying gets a good move sooner.
	//    General evolution info is in the Team Insights panel.
	if page.TeamInsights != nil {
		var vgID int
		db.QueryRow(`SELECT gv.version_group_id FROM run r JOIN game_version gv ON gv.id = r.version_id WHERE r.id = ?`, runID).Scan(&vgID) //nolint:errcheck
		for _, es := range page.TeamInsights.EvoSummaries {
			for _, path := range es.Paths {
				if len(path.Steps) == 0 || !path.FullyLegal {
					continue
				}
				step := path.Steps[0]
				minLvl, ok := step.Conditions["min_level"]
				if !ok {
					continue // only level-based evos have delay advice
				}
				evoLvl := 0
				switch v := minLvl.(type) {
				case float64:
					evoLvl = int(v)
				case int:
					evoLvl = v
				}
				if evoLvl == 0 {
					continue
				}
				// Find current level
				var curLevel int
				for _, pm := range page.PartyMoves {
					if capitalizeVersion(pm.SpeciesName) == es.SpeciesName {
						curLevel = pm.Level
						break
					}
				}
				if curLevel == 0 || curLevel+5 < evoLvl {
					continue // too far from evolving to be actionable
				}
				delays, _ := legality.MoveDelayAnalysis(db, es.FormID, path, vgID)
				for _, d := range delays {
					if d.Recommendation != "delay" {
						continue
					}
					if d.PreEvoLevel <= curLevel || d.PreEvoLevel > evoLvl {
						continue // already learned or post-evo level
					}
					rec := fmt.Sprintf("Delay evolving %s — it learns %s at Lv%d, but %s doesn't learn it until Lv%d.",
						es.SpeciesName, d.MoveName, d.PreEvoLevel, capitalizeVersion(step.ToSpeciesName), d.PostEvoLevel)
					if d.PostEvoLevel == 0 {
						rec = fmt.Sprintf("Delay evolving %s — it learns %s at Lv%d, which %s can't learn via level-up at all.",
							es.SpeciesName, d.MoveName, d.PreEvoLevel, capitalizeVersion(step.ToSpeciesName))
					}
					recs = append(recs, rec)
					break // one delay note per pokemon is enough
				}
				break // first path only
			}
		}
	}

	// 5. Walkthrough items at current location — surface TMs/items the player
	//    can pick up right now, extracted from the embedded walkthrough guide.
	//    For TMs, annotate which party member can learn it.
	//    Check both current and next badge sections since players are between gyms.
	//    Skipped when the no_item_locations rule is active.
	_, noItems := activeRules["no_item_locations"]
	if currentLocation != "" && !noItems {
		// Build map of TM move name → species that can learn it.
		tmLearners := map[string]string{} // lowercase move name → species
		for _, pm := range page.PartyMoves {
			for _, mv := range pm.Moves {
				if mv.TMNumber > 0 {
					tmLearners[strings.ToLower(mv.Name)] = pm.SpeciesName
				}
			}
		}

		hint := walkthroughs.NormalizeLocation(currentLocation)
		for _, bc := range []int{badgeCount, badgeCount + 1} {
			section := walkthroughs.Lookup(versionName, bc)
			if section == "" {
				continue
			}
			items := walkthroughs.SubSection(section, "Items & TMs")
			if items == "" {
				continue
			}
			headerSkipped := 0
			for _, line := range strings.Split(items, "\n") {
				trimmed := strings.TrimSpace(line)
				if !strings.Contains(trimmed, "|") {
					continue
				}
				if headerSkipped < 2 {
					headerSkipped++
					continue
				}
				cols := strings.Split(trimmed, "|")
				if len(cols) < 3 {
					continue
				}
				loc := strings.TrimSpace(cols[1])
				itemDesc := strings.TrimSpace(cols[2])
				if loc == "" || itemDesc == "" || itemDesc == "—" {
					continue
				}
				normLoc := walkthroughs.NormalizeLocation(loc)
				if !strings.Contains(normLoc, hint) && !strings.Contains(hint, normLoc) {
					continue
				}
				rec := fmt.Sprintf("Pick up %s at %s.", itemDesc, loc)
				// For TMs, annotate which party member can learn it.
				if strings.HasPrefix(itemDesc, "TM") {
					tmParts := strings.SplitN(itemDesc, " ", 3) // ["TM05", "Roar", ...]
					if len(tmParts) >= 2 {
						moveName := strings.ToLower(tmParts[1])
						if species, ok := tmLearners[moveName]; ok {
							rec = fmt.Sprintf("Pick up %s at %s — %s can learn it.", itemDesc, loc, capitalizeVersion(species))
						}
					}
				}
				recs = append(recs, rec)
			}
		}
	}

	if len(recs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nVERIFIED RECOMMENDATIONS (these are facts computed from the game database — present them as-is, do not modify names/types/levels):\n")
	for i, r := range recs {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, r)
	}
	return sb.String()
}

// buildGameSummary produces a concise human-readable summary of the game state
// for the LLM. Raw JSON overwhelms small models (qwen2.5:3b); this cuts the
// context from ~3000 tokens to ~300 tokens of natural language.
func buildGameSummary(page CoachPage, activeRules map[string]interface{}, versionName string, badgeCount, maxPartyLevel int, currentLocation string) string {
	var sb strings.Builder

	// Foundational data — game, progression, party size
	fmt.Fprintf(&sb, "GAME: Pokemon %s\n", capitalizeVersion(versionName))
	fmt.Fprintf(&sb, "BADGES: %d\n", badgeCount)
	fmt.Fprintf(&sb, "HIGHEST PARTY LEVEL: %d\n", maxPartyLevel)
	fmt.Fprintf(&sb, "PARTY SIZE: %d/6\n", len(page.PartyMoves))
	if currentLocation != "" {
		fmt.Fprintf(&sb, "CURRENT LOCATION: %s\n", capitalizeVersion(currentLocation))
	} else {
		sb.WriteString("CURRENT LOCATION: Not set\n")
	}

	// Active rules
	if len(activeRules) > 0 {
		sb.WriteString("ACTIVE RULES: ")
		first := true
		keys := make([]string, 0, len(activeRules))
		for k := range activeRules {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			first = false
		}
		sb.WriteString("\n\n")
	}

	// Party with current moves
	sb.WriteString("PARTY:\n")
	if page.TeamInsights != nil {
		for _, m := range page.TeamInsights.Members {
			fmt.Fprintf(&sb, "- %s Lv%d (%s)", m.SpeciesName, m.Level, strings.Join(m.Types, "/"))
			if m.Ability != "" {
				fmt.Fprintf(&sb, " [%s]", m.Ability)
			}
			sb.WriteString("\n")
			if len(m.CurrentMoves) > 0 {
				sb.WriteString("  Knows: ")
				for i, mv := range m.CurrentMoves {
					if i > 0 {
						sb.WriteString(", ")
					}
					if mv.Power != nil && *mv.Power > 0 {
						fmt.Fprintf(&sb, "%s (%s %dpwr)", mv.Name, mv.TypeName, *mv.Power)
					} else {
						fmt.Fprintf(&sb, "%s (%s)", mv.Name, mv.TypeName)
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	// Upcoming level-up moves (within next 20 levels)
	for _, pm := range page.PartyMoves {
		var upcoming []string
		for _, mv := range pm.Moves {
			if mv.LearnMethod == "level-up" && mv.Level > pm.Level && mv.Level <= pm.Level+20 {
				entry := fmt.Sprintf("Lv%d %s (%s", mv.Level, mv.Name, mv.TypeName)
				if mv.Power != nil && *mv.Power > 0 {
					entry += fmt.Sprintf(" %dpwr", *mv.Power)
				}
				entry += ")"
				upcoming = append(upcoming, entry)
			}
		}
		if len(upcoming) > 0 {
			fmt.Fprintf(&sb, "\n%s upcoming moves:\n", capitalizeVersion(pm.SpeciesName))
			for _, u := range upcoming {
				fmt.Fprintf(&sb, "  %s\n", u)
			}
		}

		// Strong learnable TMs/HMs (power >= 50)
		var tms []string
		for _, mv := range pm.Moves {
			if mv.LearnMethod != "machine" || mv.Power == nil || *mv.Power < 50 {
				continue
			}
			label := mv.Name
			if mv.TMNumber > 0 {
				label = fmt.Sprintf("TM%02d %s", mv.TMNumber, mv.Name)
			} else if mv.HMNumber > 0 {
				label = fmt.Sprintf("HM%02d %s", mv.HMNumber, mv.Name)
			}
			tms = append(tms, fmt.Sprintf("%s (%s %dpwr)", label, mv.TypeName, *mv.Power))
		}
		if len(tms) > 0 {
			fmt.Fprintf(&sb, "  Usable TMs: %s\n", strings.Join(tms, ", "))
		}
	}

	// Evolution paths
	if page.TeamInsights != nil && len(page.TeamInsights.EvoSummaries) > 0 {
		sb.WriteString("\nEVOLUTION:\n")
		for _, es := range page.TeamInsights.EvoSummaries {
			for _, path := range es.Paths {
				chain := es.SpeciesName
				for _, step := range path.Steps {
					cond := formatEvoCondition(step)
					chain += fmt.Sprintf(" -> %s %s", capitalizeVersion(step.ToSpeciesName), cond)
				}
				fmt.Fprintf(&sb, "- %s\n", chain)
			}
		}
	}

	// Team weaknesses/resistances
	if page.TeamInsights != nil {
		if len(page.TeamInsights.Weaknesses) > 0 {
			types := make([]string, 0, len(page.TeamInsights.Weaknesses))
			for _, w := range page.TeamInsights.Weaknesses {
				types = append(types, w.Type)
			}
			fmt.Fprintf(&sb, "\nTEAM WEAK TO: %s\n", strings.Join(types, ", "))
		}
		if len(page.TeamInsights.Resistances) > 0 {
			fmt.Fprintf(&sb, "TEAM RESISTS: %s\n", strings.Join(page.TeamInsights.Resistances, ", "))
		}
	}

	// Next opponents
	if len(page.NextOpponents) > 0 {
		sb.WriteString("\nNEXT OPPONENTS:\n")
		for _, opp := range page.NextOpponents {
			fmt.Fprintf(&sb, "- %s (%s-type, %s, Badge #%d)\n",
				opp.Name, capitalizeVersion(opp.TypeSpecialty), opp.LocationName, opp.BadgeOrder)
			for _, p := range opp.Team {
				fmt.Fprintf(&sb, "  %s Lv%d (%s)", capitalizeVersion(p.SpeciesName), p.Level, strings.Join(p.Types, "/"))
				if len(p.Moves) > 0 {
					fmt.Fprintf(&sb, " — %s", strings.Join(p.Moves, ", "))
				}
				sb.WriteString("\n")
			}
		}
	}

	// Catches
	if len(page.Acquisitions) > 0 {
		sb.WriteString("\nAVAILABLE CATCHES:\n")
		for _, a := range page.Acquisitions {
			method := humanizeMethod(a.Method)
			entry := fmt.Sprintf("- %s at %s", capitalizeVersion(a.SpeciesName), a.LocationName)
			if a.MinLevel == a.MaxLevel {
				entry += fmt.Sprintf(" (Lv%d, %s)", a.MinLevel, method)
			} else {
				entry += fmt.Sprintf(" (Lv%d-%d, %s)", a.MinLevel, a.MaxLevel, method)
			}
			if a.BlockedByRule != nil {
				entry += fmt.Sprintf(" [BLOCKED: %s]", *a.BlockedByRule)
			}
			sb.WriteString(entry + "\n")
		}
	} else {
		sb.WriteString("\nAVAILABLE CATCHES: None at current progress.\n")
	}

	// Trades
	if len(page.Trades) > 0 {
		sb.WriteString("\nAVAILABLE TRADES:\n")
		for _, t := range page.Trades {
			fmt.Fprintf(&sb, "- Give %s -> Receive %s", capitalizeVersion(t.GiveSpecies), capitalizeVersion(t.ReceiveSpecies))
			if t.Notes != "" {
				fmt.Fprintf(&sb, " (%s)", t.Notes)
			}
			sb.WriteString("\n")
		}
	}

	// Targeted TM recommendations: only show shop TMs that a party member can
	// learn AND that are stronger than their current move of the same type.
	shopTMSet := make(map[int]string) // TMNumber -> display name
	for _, item := range page.LegalItems {
		if strings.HasPrefix(item.Name, "tm") {
			// Parse TM number from display name like "TM24 — Thunderbolt"
			var tmn int
			if _, err := fmt.Sscanf(item.Name, "tm%d", &tmn); err == nil && tmn > 0 {
				shopTMSet[tmn] = item.DisplayName
			}
		}
	}
	if len(shopTMSet) > 0 {
		var tmLines []string
		for _, pm := range page.PartyMoves {
			// Build set of current move powers by type for this party member
			currentBestByType := map[string]int{}
			if page.TeamInsights != nil {
				for _, m := range page.TeamInsights.Members {
					if capitalizeVersion(pm.SpeciesName) == m.SpeciesName {
						for _, mv := range m.CurrentMoves {
							if mv.Power != nil && *mv.Power > currentBestByType[mv.TypeName] {
								currentBestByType[mv.TypeName] = *mv.Power
							}
						}
						break
					}
				}
			}
			for _, mv := range pm.Moves {
				if mv.LearnMethod != "machine" || mv.TMNumber == 0 {
					continue
				}
				if _, inShop := shopTMSet[mv.TMNumber]; !inShop {
					continue
				}
				if mv.Power == nil || *mv.Power < 50 {
					continue
				}
				// Only recommend if it's better than what they already have in that type
				if currentBest, ok := currentBestByType[mv.TypeName]; ok && currentBest >= *mv.Power {
					continue
				}
				tmLines = append(tmLines, fmt.Sprintf("- %s can learn TM%02d %s (%s %dpwr) — buy at shop",
					capitalizeVersion(pm.SpeciesName), mv.TMNumber, mv.Name, mv.TypeName, *mv.Power))
			}
		}
		if len(tmLines) > 0 {
			sb.WriteString("\nTM UPGRADES (available in shop):\n")
			for _, line := range tmLines {
				sb.WriteString(line + "\n")
			}
		}
	}

	return sb.String()
}

// buildWalkthroughContext returns targeted walkthrough context for the LLM.
// Instead of dumping the full badge section, it extracts:
//  1. Section header (gym info, threats, strategy) — always included.
//  2. Items & TMs filtered to the player's current location.
//  3. Unlocks — what opens after this badge.
//  4. Side Quests — when present.
//
// This keeps the payload focused on the player's actual progress.
func buildWalkthroughContext(versionName string, badgeCount int, currentLocation string, activeRules map[string]interface{}) string {
	section := walkthroughs.Lookup(versionName, badgeCount)
	if section == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nWALKTHROUGH (current progression):\n")

	// 1. Gym/strategy header (everything before the first ### sub-section).
	if hdr := walkthroughs.SectionHeader(section); hdr != "" {
		sb.WriteString(hdr)
		sb.WriteByte('\n')
	}

	// 2. Items & TMs — filtered to the player's current location when possible.
	//    Skipped when the no_item_locations rule is active.
	if _, noItemsWT := activeRules["no_item_locations"]; !noItemsWT {
		if items := walkthroughs.SubSection(section, "Items & TMs"); items != "" {
			filtered := walkthroughs.FilterTableByLocation(items, currentLocation)
			sb.WriteString("\n")
			sb.WriteString(filtered)
			sb.WriteByte('\n')
		}
	}

	// 3. Unlocks — key progression gates.
	if unlocks := walkthroughs.SubSection(section, "Unlocks"); unlocks != "" {
		sb.WriteString("\n")
		sb.WriteString(unlocks)
		sb.WriteByte('\n')
	}

	// 4. Gates — map access restrictions and one-way points of no return.
	if gates := walkthroughs.SubSection(section, "Gates"); gates != "" {
		sb.WriteString("\n")
		sb.WriteString(gates)
		sb.WriteByte('\n')
	}

	// 5. Side Quests — Oak's Aides, etc.
	if sq := walkthroughs.SubSection(section, "Side Quests"); sq != "" {
		sb.WriteString("\n")
		sb.WriteString(sq)
		sb.WriteByte('\n')
	}

	return sb.String()
}

// sourceOrder returns a sort key for item sources: owned < obtainable < shop.
func sourceOrder(source string) int {
	switch source {
	case "owned":
		return 0
	case "obtainable":
		return 1
	case "shop":
		return 2
	default:
		return 3
	}
}

// humanizeMethod converts raw encounter method slugs into readable text.
func humanizeMethod(method string) string {
	switch method {
	case "walk":
		return "tall grass"
	case "old-rod":
		return "Old Rod fishing"
	case "good-rod":
		return "Good Rod fishing"
	case "super-rod":
		return "Super Rod fishing"
	case "surf":
		return "surfing"
	case "rock-smash":
		return "Rock Smash"
	case "headbutt":
		return "Headbutt trees"
	case "gift", "gift-egg":
		return "NPC gift"
	case "only-one":
		return "static encounter"
	default:
		return method
	}
}

// formatEvoCondition renders an evolution step's trigger as readable text.
func formatEvoCondition(step legality.EvolutionStep) string {
	if minLvl, ok := step.Conditions["min_level"]; ok {
		return fmt.Sprintf("(Lv%v)", minLvl)
	}
	if item, ok := step.Conditions["item"]; ok {
		return fmt.Sprintf("(use %v)", item)
	}
	if step.Trigger == "trade" {
		return "(trade)"
	}
	// Fallback: show trigger name
	return fmt.Sprintf("(%s)", step.Trigger)
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
			LocationName:  humanizeLocationName(locationName),
			BadgeOrder:    badgeOrder,
			Team:          team,
		})
	}

	return summaries, nil
}

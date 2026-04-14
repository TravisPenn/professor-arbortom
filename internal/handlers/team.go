package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/TravisPenn/professor-arbortom/internal/legality"
	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/gin-gonic/gin"
)

// ShowTeam renders GET /runs/:run_id/team — compact overview, no heavy selects.
func ShowTeam(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		page, err := buildTeamPage(c, db, run.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.HTML(http.StatusOK, "team.html", page)
	}
}

// ShowTeamSlot renders GET /runs/:run_id/team/:slot — single-slot edit form.
func ShowTeamSlot(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		slot, ok := mustParamInt(c, "slot")
		if !ok {
			return
		}
		if slot < 1 || slot > 6 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Slot must be 1–6"})
			return
		}
		page, err := buildTeamSlotPage(c, db, run.ID, slot, nil)
		if err != nil {
			respondError(c, err)
			return
		}
		c.HTML(http.StatusOK, "team_slot.html", page)
	}
}

// UpdateTeam handles POST /runs/:run_id/team
func UpdateTeam(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)

		slot := formInt(c, "slot", 0)
		if slot < 1 || slot > 6 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Invalid slot"})
			return
		}

		// pokemon_select: "" = clear, "rp-{id}" = owned pokemon, "new-{formID}" = new catch
		pokemonSel := c.PostForm("pokemon_select")
		levelStr := c.PostForm("level")

		if pokemonSel == "" {
			// Clear slot — mark the occupant as no longer in the party.
			if _, err := db.Exec(`UPDATE run_pokemon SET in_party = 0, party_slot = NULL WHERE run_id = ? AND party_slot = ? AND in_party = 1`, run.ID, slot); err != nil {
				respondError(c, err)
				return
			}
			c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
			return
		}

		// Resolve runPokemonID (owned) or formID (new catch).
		var runPokemonID, formID int
		switch {
		case strings.HasPrefix(pokemonSel, "rp-"):
			id, err := strconv.Atoi(pokemonSel[3:])
			if err != nil || id <= 0 {
				c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Invalid pokemon selection"})
				return
			}
			runPokemonID = id
			// SEC-013: validate ownership scoped to this run.
			if err := db.QueryRow(`SELECT form_id FROM run_pokemon WHERE id = ? AND run_id = ? AND is_alive = 1`, id, run.ID).Scan(&formID); err != nil {
				c.HTML(http.StatusNotFound, "error.html", gin.H{"Message": "Pokémon not found in this run"})
				return
			}
		case strings.HasPrefix(pokemonSel, "new-"):
			id, err := strconv.Atoi(pokemonSel[4:])
			if err != nil || id <= 0 {
				c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Invalid species selection"})
				return
			}
			formID = id
		default:
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Invalid selection"})
			return
		}

		level, _ := strconv.Atoi(levelStr)
		if level < 1 || level > 100 {
			level = 1
		}

		// Collect move IDs from form.
		var moveIDs []int
		for i := 1; i <= 4; i++ {
			if mid, err := strconv.Atoi(c.PostForm("move_" + strconv.Itoa(i))); err == nil && mid > 0 {
				moveIDs = append(moveIDs, mid)
			}
		}

		heldItemID := 0
		if hid, err := strconv.Atoi(c.PostForm("held_item_id")); err == nil {
			heldItemID = hid
		}

		// Legality: owned pokemon skip encounter check (already in run).
		// New catches must be encounter-legal at current location.
		legalErrors := map[string]string{}
		if runPokemonID == 0 {
			acqs, _, _ := legality.LegalAcquisitions(db, run.ID)
			legalForms := make(map[int]bool)
			for _, a := range acqs {
				legalForms[a.FormID] = true
			}
			if !legalForms[formID] {
				legalErrors["pokemon_select"] = "This Pokémon is not currently obtainable"
			}
		}

		// Move legality applies to both paths when the user explicitly submitted moves.
		if len(moveIDs) > 0 {
			legalMoves, _, _ := legality.LegalMoves(db, run.ID, formID)
			legalMoveSet := make(map[int]bool)
			for _, mv := range legalMoves {
				if mv.BlockedByRule == nil {
					legalMoveSet[mv.MoveID] = true
				}
			}
			for i, mid := range moveIDs {
				if !legalMoveSet[mid] {
					legalErrors["move_"+strconv.Itoa(i+1)] = "Move is not legally learnable at this point"
				}
			}
		}

		if heldItemID > 0 {
			legalItems, _ := legality.LegalItems(db, run.ID)
			legalItemSet := make(map[int]bool)
			for _, it := range legalItems {
				legalItemSet[it.ItemID] = true
			}
			if !legalItemSet[heldItemID] {
				legalErrors["held_item_id"] = "Item is not currently available"
			}
		}

		if len(legalErrors) > 0 {
			page, _ := buildTeamSlotPage(c, db, run.ID, slot, legalErrors)
			page.BasePage.Flash = &Flash{Type: "error", Message: "Legality check failed. See errors below."}
			c.HTML(http.StatusUnprocessableEntity, "team_slot.html", page)
			return
		}

		// For owned pokemon: preserve existing level/moves if the form fields were hidden.
		if runPokemonID > 0 {
			if level == 1 {
				db.QueryRow(`SELECT level FROM run_pokemon WHERE id = ?`, runPokemonID).Scan(&level) //nolint:errcheck
			}
			if len(moveIDs) == 0 {
				var existingJSON string
				if db.QueryRow(`SELECT moves_json FROM run_pokemon WHERE id = ?`, runPokemonID).Scan(&existingJSON) == nil {
					json.Unmarshal([]byte(existingJSON), &moveIDs) //nolint:errcheck
				}
			}
		}

		movesJSON, _ := json.Marshal(moveIDs)

		var heldPtr interface{} = nil
		if heldItemID > 0 {
			heldPtr = heldItemID
		}

		// Evict whoever currently occupies this slot (may be a different pokemon).
		if _, err := db.Exec(`UPDATE run_pokemon SET in_party = 0, party_slot = NULL WHERE run_id = ? AND party_slot = ? AND in_party = 1`, run.ID, slot); err != nil {
			respondError(c, err)
			return
		}

		// Place the pokemon in the slot.
		var pkmnID int
		if runPokemonID > 0 {
			// Owned pokemon: target the specific row directly.
			pkmnID = runPokemonID
		} else {
			// New catch: insert a fresh run_pokemon row.
			res, err2 := db.Exec(`INSERT INTO run_pokemon (run_id, form_id, level, caught_level, acquisition_type, is_alive, in_party, party_slot, moves_json, held_item_id) VALUES (?, ?, ?, ?, 'manual', 1, 1, ?, ?, ?)`,
				run.ID, formID, level, level, slot, string(movesJSON), heldPtr)
			if err2 != nil {
				respondError(c, err2)
				return
			}
			newID, _ := res.LastInsertId()
			pkmnID = int(newID)
		}
		if _, err := db.Exec(`UPDATE run_pokemon SET in_party = 1, party_slot = ?, level = ?, moves_json = ?, held_item_id = ? WHERE id = ?`,
			slot, level, string(movesJSON), heldPtr, pkmnID); err != nil {
			respondError(c, err)
			return
		}

		// Sync run_pokemon_move for COACH-005 moveset tracking.
		if _, err := db.Exec(`DELETE FROM run_pokemon_move WHERE run_pokemon_id = ?`, pkmnID); err != nil {
			log.Printf("WARN: delete run_pokemon_move for %d: %v", pkmnID, err) // SEC-008: non-fatal, log
		}
		for i, mid := range moveIDs {
			if _, err := db.Exec(`INSERT OR REPLACE INTO run_pokemon_move (run_pokemon_id, slot, move_id) VALUES (?, ?, ?)`,
				pkmnID, i+1, mid); err != nil {
				log.Printf("WARN: insert run_pokemon_move for %d slot %d: %v", pkmnID, i+1, err) // SEC-008: non-fatal, log
			}
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
	}
}

// ShowBox renders GET /runs/:run_id/box
func ShowBox(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		showFainted := c.Query("fainted") == "true"
		nuzlockeOn := isRuleEnabled(activeRules, "nuzlocke")

		query := `
			SELECT rp.id, rp.form_id, p.species_name, p.form_name, rp.level,
				rp.caught_level, rp.acquisition_type,
				COALESCE(l.name, '') AS met_location, rp.is_alive, rp.moves_json
			FROM run_pokemon rp
			JOIN pokemon p ON p.id = rp.form_id
			LEFT JOIN location l ON l.id = rp.met_location_id
			WHERE rp.run_id = ? AND rp.in_party = 0`
		if !showFainted {
			query += ` AND rp.is_alive = 1`
		}
		query += ` ORDER BY p.species_name, rp.id`

		rows, err := db.Query(query, run.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		defer rows.Close()

		var entries []BoxEntry
		var rawMoveIDs [][]int
		for rows.Next() {
			var e BoxEntry
			var alive int
			var movesJSON string
			if err := rows.Scan(&e.ID, &e.FormID, &e.SpeciesName, &e.FormName, &e.Level, &e.CaughtLevel, &e.AcquisitionType, &e.MetLocation, &alive, &movesJSON); err != nil {
				continue
			}
			e.MetLocation = humanizeLocationName(e.MetLocation)
			e.IsAlive = alive == 1
			var mids []int
			json.Unmarshal([]byte(movesJSON), &mids) //nolint:errcheck
			entries = append(entries, e)
			rawMoveIDs = append(rawMoveIDs, mids)
		}
		enrichBoxMoves(db, entries, rawMoveIDs)

		// Load available evolutions for each alive entry.
		for i, e := range entries {
			if !e.IsAlive {
				continue
			}
			evos, _ := legality.EvolutionOptions(db, run.ID, e.FormID)
			for _, evo := range evos {
				if evo.CurrentlyPossible && evo.BlockedByRule == nil {
					entries[i].Evolutions = append(entries[i].Evolutions, evo)
				}
			}
		}

		page := BoxPage{
			BasePage:    BasePage{PageTitle: "Box", ActiveNav: "box", RunContext: buildRunContext(c)},
			Entries:     entries,
			ShowFainted: showFainted,
			NuzlockeOn:  nuzlockeOn,
		}
		c.HTML(http.StatusOK, "box.html", page)
	}
}

// MarkFainted handles POST /runs/:run_id/box/:entry_id/faint
func MarkFainted(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		entryID, ok := mustParamInt(c, "entry_id")
		if !ok {
			return
		}
		db.Exec(`UPDATE run_pokemon SET is_alive = 0 WHERE id = ? AND run_id = ?`, entryID, run.ID) // SEC-008: Faint status is user-visible but non-fatal if missed; nuzlocke re-checks on load.
		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
	}
}

// MarkRevived handles POST /runs/:run_id/box/:entry_id/revive
func MarkRevived(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		// Revive is only allowed if Nuzlocke is disabled
		if isRuleEnabled(activeRules, "nuzlocke") {
			c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
			return
		}

		entryID, ok := mustParamInt(c, "entry_id")
		if !ok {
			return
		}
		db.Exec(`UPDATE run_pokemon SET is_alive = 1 WHERE id = ? AND run_id = ?`, entryID, run.ID) // SEC-008: Revive status is user-visible but non-fatal; page refreshes re-check state.
		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
	}
}

// EvolveBox handles POST /runs/:run_id/box/:entry_id/evolve
func EvolveBox(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)

		entryID, ok := mustParamInt(c, "entry_id")
		if !ok {
			return
		}
		toFormID := formInt(c, "to_form_id", 0)
		if toFormID <= 0 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "Invalid to_form_id"})
			return
		}

		// Fetch the current form_id for this box entry, scoped to the run for safety.
		var currentFormID int
		if err := db.QueryRow(`SELECT form_id FROM run_pokemon WHERE id = ? AND run_id = ?`, entryID, run.ID).Scan(&currentFormID); err != nil {
			c.HTML(http.StatusNotFound, "error.html", gin.H{"Message": "Box entry not found"})
			return
		}

		// Verify the requested evolution is currently legal.
		evos, err := legality.EvolutionOptions(db, run.ID, currentFormID)
		if err != nil {
			respondError(c, err)
			return
		}
		legalEvo := false
		for _, evo := range evos {
			if evo.ToFormID == toFormID && evo.CurrentlyPossible && evo.BlockedByRule == nil {
				legalEvo = true
				break
			}
		}
		if !legalEvo {
			c.HTML(http.StatusUnprocessableEntity, "error.html", gin.H{"Message": "This evolution is not currently legal"})
			return
		}

		// Apply evolution: update the single run_pokemon row in-place.
		if _, err := db.Exec(`UPDATE run_pokemon SET form_id = ? WHERE id = ? AND run_id = ?`, toFormID, entryID, run.ID); err != nil {
			respondError(c, err)
			return
		}

		// Seed the evolved form's learnset in the background.
		var versionGroupID int
		db.QueryRow(`SELECT gv.version_group_id FROM run r JOIN game_version gv ON gv.id = r.version_id WHERE r.id = ?`, run.ID).Scan(&versionGroupID) //nolint:errcheck
		if versionGroupID > 0 {
			// SEC-011: deduplicated, tracked goroutine
			pokeClient.GoEnsurePokemon(db, toFormID, versionGroupID)
		}

		c.Redirect(http.StatusFound, "/runs/"+itoa(run.ID)+"/pokemon")
	}
}

// ShowPokemon renders GET /runs/:run_id/pokemon — merged Team + Box + Route logging.
func ShowPokemon(db *sql.DB, pokeClient *pokeapi.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		run := c.MustGet("run").(models.Run)
		activeRules := c.MustGet("active_rules").([]models.ActiveRule)

		nuzlockeOn := isRuleEnabled(activeRules, "nuzlocke")

		// ── Team slots ─────────────────────────────────────────
		var slots [6]PartySlot
		for i := 0; i < 6; i++ {
			slots[i] = PartySlot{Slot: i + 1}
		}
		teamRows, err := db.Query(
			`SELECT party_slot, form_id, level, moves_json, COALESCE(held_item_id, 0)
			FROM run_pokemon WHERE run_id = ? AND in_party = 1`, run.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		defer teamRows.Close()
		for teamRows.Next() {
			var s PartySlot
			var movesJSON string
			var heldItemID int
			if err := teamRows.Scan(&s.Slot, &s.FormID, &s.Level, &movesJSON, &heldItemID); err != nil {
				continue
			}
			if heldItemID > 0 {
				s.HeldItemID = &heldItemID
			}
			var moveIDs []int
			json.Unmarshal([]byte(movesJSON), &moveIDs) //nolint:errcheck
			for i, mid := range moveIDs {
				if i < 4 {
					id := mid
					s.MoveIDs[i] = &id
				}
			}
			if s.FormID != nil {
				db.QueryRow(`SELECT p.species_name, p.form_name FROM pokemon p WHERE p.id = ?`, *s.FormID).Scan(&s.SpeciesName, &s.FormName) //nolint:errcheck
				for i, mid := range s.MoveIDs {
					if mid != nil {
						var chip MoveChip
						var power, accuracy sql.NullInt64
						db.QueryRow(
							`SELECT name, COALESCE(damage_class,''), power, accuracy, pp, COALESCE(effect_entry,'')
							FROM move WHERE id = ?`, *mid,
						).Scan(&chip.Name, &chip.DamageClass, &power, &accuracy, &chip.PP, &chip.Effect) //nolint:errcheck
						if power.Valid {
							v := int(power.Int64)
							chip.Power = &v
						}
						if accuracy.Valid {
							v := int(accuracy.Int64)
							chip.Accuracy = &v
						}
						s.MoveNames[i] = chip.Name
						s.Moves[i] = chip
					}
				}
			}
			slots[s.Slot-1] = s
		}

		// ── Box entries ────────────────────────────────────────
		showFainted := c.Query("fainted") == "true"
		boxQuery := `
			SELECT rp.id, rp.form_id, p.species_name, p.form_name, rp.level,
				rp.caught_level, rp.acquisition_type,
				COALESCE(l.name, '') AS met_location, rp.is_alive, rp.moves_json
			FROM run_pokemon rp
			JOIN pokemon p ON p.id = rp.form_id
			LEFT JOIN location l ON l.id = rp.met_location_id
			WHERE rp.run_id = ? AND rp.in_party = 0`
		if !showFainted {
			boxQuery += ` AND rp.is_alive = 1`
		}
		boxQuery += ` ORDER BY p.species_name, rp.id`
		boxRows, err := db.Query(boxQuery, run.ID)
		if err != nil {
			respondError(c, err)
			return
		}
		defer boxRows.Close()
		var entries []BoxEntry
		var rawMoveIDs [][]int
		for boxRows.Next() {
			var e BoxEntry
			var alive int
			var movesJSON string
			if err := boxRows.Scan(&e.ID, &e.FormID, &e.SpeciesName, &e.FormName, &e.Level, &e.CaughtLevel, &e.AcquisitionType, &e.MetLocation, &alive, &movesJSON); err != nil {
				continue
			}
			e.MetLocation = humanizeLocationName(e.MetLocation)
			e.IsAlive = alive == 1
			var mids []int
			json.Unmarshal([]byte(movesJSON), &mids) //nolint:errcheck
			entries = append(entries, e)
			rawMoveIDs = append(rawMoveIDs, mids)
		}
		enrichBoxMoves(db, entries, rawMoveIDs)
		for i, e := range entries {
			if !e.IsAlive {
				continue
			}
			evos, _ := legality.EvolutionOptions(db, run.ID, e.FormID)
			for _, evo := range evos {
				if evo.CurrentlyPossible && evo.BlockedByRule == nil {
					entries[i].Evolutions = append(entries[i].Evolutions, evo)
				}
			}
		}

		// ── Route log + locations ────────────────────────────
		routeLog, _ := loadRouteLog(db, run.ID, nuzlockeOn)
		locations, _ := loadLocations(db, run.VersionID)
		encounters, _ := loadEncountersByLocation(db, run.VersionID)
		if pokeClient != nil {
			pokeClient.GoEnsureAllEncounters(db, run.VersionID)
		}

		// Default the location dropdown to the player's current location.
		progress := c.MustGet("progress").(models.RunProgress)
		var defaultLocID int
		if progress.CurrentLocationID != nil {
			defaultLocID = *progress.CurrentLocationID
		}

		page := PokemonPage{
			BasePage: BasePage{
				PageTitle:  "Pokémon",
				ActiveNav:  "pokemon",
				RunContext: buildRunContext(c),
			},
			Slots:                slots,
			Entries:              entries,
			ShowFainted:          showFainted,
			NuzlockeOn:           nuzlockeOn,
			Log:                  routeLog,
			Locations:            locations,
			EncountersByLocation: encounters,
			FormLocationID:       defaultLocID,
		}
		c.HTML(http.StatusOK, "pokemon.html", page)
	}
}

// ─── team page helpers ────────────────────────────────────────────────────────

// buildTeamPage builds the compact team overview (no legality queries).
func buildTeamPage(c *gin.Context, db *sql.DB, runID int) (TeamPage, error) {
	var slots [6]PartySlot
	for i := 0; i < 6; i++ {
		slots[i] = PartySlot{Slot: i + 1}
	}

	rows, err := db.Query(
		`SELECT party_slot, form_id, level, moves_json, COALESCE(held_item_id, 0)
		FROM run_pokemon WHERE run_id = ? AND in_party = 1`, runID)
	if err != nil {
		return TeamPage{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var s PartySlot
		var movesJSON string
		var heldItemID int
		if err := rows.Scan(&s.Slot, &s.FormID, &s.Level, &movesJSON, &heldItemID); err != nil {
			continue
		}
		if heldItemID > 0 {
			s.HeldItemID = &heldItemID
		}
		var moveIDs []int
		json.Unmarshal([]byte(movesJSON), &moveIDs) //nolint:errcheck
		for i, mid := range moveIDs {
			if i < 4 {
				id := mid
				s.MoveIDs[i] = &id
			}
		}
		if s.FormID != nil {
			db.QueryRow(`
				SELECT p.species_name, p.form_name
				FROM pokemon p
				WHERE p.id = ?`, *s.FormID).Scan(&s.SpeciesName, &s.FormName) //nolint:errcheck
			// Resolve move details for overview display and tooltips
			for i, mid := range s.MoveIDs {
				if mid != nil {
					var chip MoveChip
					var power, accuracy sql.NullInt64
					db.QueryRow( //nolint:errcheck
						`SELECT name, COALESCE(damage_class,''), power, accuracy, pp, COALESCE(effect_entry,'')
						FROM move WHERE id = ?`, *mid,
					).Scan(&chip.Name, &chip.DamageClass, &power, &accuracy, &chip.PP, &chip.Effect)
					if power.Valid {
						v := int(power.Int64)
						chip.Power = &v
					}
					if accuracy.Valid {
						v := int(accuracy.Int64)
						chip.Accuracy = &v
					}
					s.MoveNames[i] = chip.Name
					s.Moves[i] = chip
				}
			}
		}
		slots[s.Slot-1] = s
	}

	return TeamPage{
		BasePage: BasePage{
			PageTitle:  "Team",
			ActiveNav:  "team",
			RunContext: buildRunContext(c),
		},
		Slots: slots,
	}, nil
}

// buildTeamSlotPage loads data for a single slot edit (one set of legality queries).
func buildTeamSlotPage(c *gin.Context, db *sql.DB, runID, slotNum int, legalErrors map[string]string) (TeamSlotPage, error) {
	if legalErrors == nil {
		legalErrors = map[string]string{}
	}

	// OwnedPokemon: each individual run_pokemon row (alive), shown in "Your Pokémon" optgroup.
	ownedRows, _ := db.Query(`
		SELECT rp.id, p.id, p.species_name, p.form_name, rp.level, COALESCE(l.name, '') AS met_loc
		FROM run_pokemon rp
		JOIN pokemon p ON p.id = rp.form_id
		LEFT JOIN location l ON l.id = rp.met_location_id
		WHERE rp.run_id = ? AND rp.is_alive = 1
		ORDER BY p.species_name, rp.id
	`, runID)
	var ownedPokemon []FormOption
	seenForm := make(map[int]bool)
	if ownedRows != nil {
		defer ownedRows.Close()
		for ownedRows.Next() {
			var rpID, fid, lvl int
			var sname, fname, metLoc string
			if ownedRows.Scan(&rpID, &fid, &sname, &fname, &lvl, &metLoc) == nil {
				seenForm[fid] = true
				ownedPokemon = append(ownedPokemon, FormOption{
					ID:           fid,
					RunPokemonID: rpID,
					Level:        lvl,
					SpeciesName:  capitalizeVersion(sname),
					FormName:     fname,
					LocationName: humanizeLocationName(metLoc),
				})
			}
		}
	}

	// LegalForms: encounter-legal species NOT already owned (available to catch).
	acqs, _, _ := legality.LegalAcquisitions(db, runID)
	legalForms := make([]FormOption, 0, len(acqs))
	for _, a := range acqs {
		if !seenForm[a.FormID] {
			legalForms = append(legalForms, acquisitionToFormOption(a))
		}
	}

	items, _ := legality.LegalItems(db, runID)
	legalItems := make([]ItemOption, 0, len(items))
	for _, it := range items {
		legalItems = append(legalItems, itemToOption(it))
	}

	// Load current slot occupant (includes run_pokemon.id for precise re-selection).
	slot := PartySlot{Slot: slotNum}
	var movesJSON string
	var heldItemID int
	err := db.QueryRow(
		`SELECT rp.id, rp.form_id, rp.level, rp.moves_json, COALESCE(rp.held_item_id, 0)
		FROM run_pokemon rp WHERE rp.run_id = ? AND rp.party_slot = ? AND rp.in_party = 1`, runID, slotNum,
	).Scan(&slot.RunPokemonID, &slot.FormID, &slot.Level, &movesJSON, &heldItemID)
	if err != nil && err != sql.ErrNoRows {
		return TeamSlotPage{}, err
	}
	if err == nil {
		if heldItemID > 0 {
			slot.HeldItemID = &heldItemID
		}
		var moveIDs []int
		json.Unmarshal([]byte(movesJSON), &moveIDs) //nolint:errcheck
		for i, mid := range moveIDs {
			if i < 4 {
				id := mid
				slot.MoveIDs[i] = &id
			}
		}
	}
	if slot.FormID != nil {
		mvs, _, _ := legality.LegalMoves(db, runID, *slot.FormID)
		for _, mv := range mvs {
			slot.LegalMoves = append(slot.LegalMoves, moveToOption(mv))
		}
		db.QueryRow(`
			SELECT p.species_name, p.form_name
			FROM pokemon p
			WHERE p.id = ?`, *slot.FormID).Scan(&slot.SpeciesName, &slot.FormName) //nolint:errcheck
	}

	return TeamSlotPage{
		BasePage: BasePage{
			PageTitle:  fmt.Sprintf("Edit Slot %d", slotNum),
			ActiveNav:  "team",
			RunContext: buildRunContext(c),
		},
		SlotNum:        slotNum,
		Slot:           slot,
		OwnedPokemon:   ownedPokemon,
		LegalForms:     legalForms,
		LegalItems:     legalItems,
		LegalityErrors: legalErrors,
	}, nil
}

// enrichBoxMoves resolves move IDs from run_pokemon.moves_json into names
// and sets BoxEntry.Moves for each entry. rawMoveIDs must parallel entries.
func enrichBoxMoves(db *sql.DB, entries []BoxEntry, rawMoveIDs [][]int) {
	idSet := make(map[int]bool)
	for _, ids := range rawMoveIDs {
		for _, id := range ids {
			if id > 0 {
				idSet[id] = true
			}
		}
	}
	if len(idSet) == 0 {
		return
	}
	placeholders := make([]string, 0, len(idSet))
	args := make([]interface{}, 0, len(idSet))
	for id := range idSet {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	nameByID := make(map[int]string, len(idSet))
	rows, err := db.Query(`SELECT id, name FROM move WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if rows.Scan(&id, &name) == nil {
				nameByID[id] = name
			}
		}
	}
	for i, ids := range rawMoveIDs {
		for _, id := range ids {
			if name := nameByID[id]; name != "" {
				entries[i].Moves = append(entries[i].Moves, name)
			}
		}
	}
}

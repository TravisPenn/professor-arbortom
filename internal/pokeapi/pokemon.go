package pokeapi

import (
	"database/sql"
	"fmt"
)

// pokemonResponse is a partial representation of the PokeAPI /pokemon/{id} response.
type pokemonResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Species struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"species"`
	Types []struct {
		Slot int `json:"slot"`
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
	Stats []struct {
		BaseStat int `json:"base_stat"`
		Stat     struct {
			Name string `json:"name"`
		} `json:"stat"`
	} `json:"stats"`
	Abilities []struct {
		Slot     int  `json:"slot"`
		IsHidden bool `json:"is_hidden"`
		Ability  struct {
			Name string `json:"name"`
		} `json:"ability"`
	} `json:"abilities"`
	Moves []struct {
		Move struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"move"`
		VersionGroupDetails []struct {
			LevelLearnedAt  int `json:"level_learned_at"`
			MoveLearnMethod struct {
				Name string `json:"name"`
			} `json:"move_learn_method"`
			VersionGroup struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"version_group"`
		} `json:"version_group_details"`
	} `json:"moves"`
}

type moveResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Accuracy *int   `json:"accuracy"`
	Power    *int   `json:"power"`
	PP       int    `json:"pp"`
	Type     struct {
		Name string `json:"name"`
	} `json:"type"`
	DamageClass struct {
		Name string `json:"name"`
	} `json:"damage_class"`
	EffectEntries []struct {
		ShortEffect string `json:"short_effect"`
		Language    struct {
			Name string `json:"name"`
		} `json:"language"`
	} `json:"effect_entries"`
}

// EnsurePokemon fetches and caches species, form, and learnset data for the given
// PokeAPI pokemon ID (form ID) within the given version group.
// No-op if already cached.
//
// HTTP fetches are done before acquiring writeMu so multiple goroutines can
// fetch data concurrently; DB writes are then serialised to avoid
// SQLITE_BUSY_SNAPSHOT in WAL mode.
// After seeding, the evolution chain is also ensured so Evolve buttons appear.
func (c *Client) EnsurePokemon(db *sql.DB, formID, versionGroupID int) error {
	if err := assertGen3(versionGroupID); err != nil {
		return err
	}

	cached, err := c.isCached("pokemon", formID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	// ── Phase 1: HTTP fetch (concurrent) ─────────────────────────────────────
	var poke pokemonResponse
	if err := c.get(fmt.Sprintf("%s/pokemon/%d", baseURL, formID), &poke); err != nil {
		return err
	}

	// Extract species ID directly from the URL — avoids a redundant API call.
	speciesID := extractIDFromURL(poke.Species.URL)
	if speciesID == 0 {
		return fmt.Errorf("pokeapi: could not extract species ID from %s", poke.Species.URL)
	}

	// Fetch the evolution chain ID for this species so we can seed it after writing.
	chainID, chainErr := c.EvoChainIDForSpecies(speciesID)

	vgURL := fmt.Sprintf("%s/version-group/%d/", baseURL, versionGroupID)

	// Collect learnset entries for the requested version group.
	type learnEntry struct {
		moveName string
		method   string
		level    int
	}
	var toLearn []learnEntry
	needFetch := map[string]string{} // move name → URL for moves not yet in DB

	for _, m := range poke.Moves {
		for _, vgd := range m.VersionGroupDetails {
			if vgd.VersionGroup.URL != vgURL {
				continue
			}
			toLearn = append(toLearn, learnEntry{m.Move.Name, vgd.MoveLearnMethod.Name, vgd.LevelLearnedAt})
			// Check DB without a transaction — we just need a quick existence test.
			var id int
			if db.QueryRow(`SELECT id FROM move WHERE name = ?`, m.Move.Name).Scan(&id) == sql.ErrNoRows {
				needFetch[m.Move.Name] = m.Move.URL
			}
		}
	}

	// Fetch missing moves from the API before acquiring the write lock.
	fetched := make(map[string]moveResponse, len(needFetch))
	for name, url := range needFetch {
		var mv moveResponse
		if err := c.get(url, &mv); err != nil {
			logWarn("fetch move %s: %v", name, err)
			continue
		}
		fetched[name] = mv
	}

	// ── Phase 2: DB write (serialised) ───────────────────────────────────────
	c.writeMu.Lock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Collect types
	var type1, type2 string
	for _, t := range poke.Types {
		if t.Slot == 1 {
			type1 = t.Type.Name
		} else if t.Slot == 2 {
			type2 = t.Type.Name
		}
	}
	if type1 == "" {
		type1 = "normal"
	}

	// Collect base stats
	statMap := make(map[string]int, 6)
	for _, s := range poke.Stats {
		statMap[s.Stat.Name] = s.BaseStat
	}

	// Collect abilities (non-hidden only)
	var ability1, ability2 string
	for _, a := range poke.Abilities {
		if a.IsHidden {
			continue
		}
		if a.Slot == 1 {
			ability1 = a.Ability.Name
		} else if a.Slot == 2 {
			ability2 = a.Ability.Name
		}
	}

	// Upsert into consolidated pokemon table (SC-001)
	if _, err := tx.Exec(`
		INSERT OR REPLACE INTO pokemon
			(id, species_name, form_name, type1, type2,
			 hp, attack, defense, sp_attack, sp_defense, speed, ability1, ability2)
		VALUES (?, ?, 'default', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		poke.ID, poke.Species.Name, type1, type2,
		statMap["hp"], statMap["attack"], statMap["defense"],
		statMap["special-attack"], statMap["special-defense"], statMap["speed"],
		ability1, ability2,
	); err != nil {
		return fmt.Errorf("pokeapi: insert pokemon %d: %w", poke.ID, err)
	}

	// Upsert moves and learnset entries
	for _, entry := range toLearn {
		var moveID int
		err := tx.QueryRow(`SELECT id FROM move WHERE name = ?`, entry.moveName).Scan(&moveID)
		if err == sql.ErrNoRows {
			mv, ok := fetched[entry.moveName]
			if !ok {
				continue // could not fetch earlier; skip
			}
			power, accuracy := 0, 0
			if mv.Power != nil {
				power = *mv.Power
			}
			if mv.Accuracy != nil {
				accuracy = *mv.Accuracy
			}
			effectEntry := ""
			for _, e := range mv.EffectEntries {
				if e.Language.Name == "en" {
					effectEntry = e.ShortEffect
					break
				}
			}
			if _, insertErr := tx.Exec(
				`INSERT OR IGNORE INTO move (id, name, type_name, power, accuracy, pp, damage_class, effect_entry) VALUES (?,?,?,?,?,?,?,?)`,
				mv.ID, mv.Name, mv.Type.Name, power, accuracy, mv.PP, mv.DamageClass.Name, effectEntry,
			); insertErr != nil {
				logWarn("insert move %s: %v", entry.moveName, insertErr)
				continue
			}
			moveID = mv.ID
		} else if err != nil {
			return fmt.Errorf("pokeapi: query move %s: %w", entry.moveName, err)
		}

		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO learnset_entry (form_id, version_group_id, move_id, learn_method, level_learned)
			 VALUES (?, ?, ?, ?, ?)`,
			poke.ID, versionGroupID, moveID, entry.method, entry.level,
		); err != nil {
			logWarn("insert learnset %d/%d/%d: %v", poke.ID, versionGroupID, moveID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		c.writeMu.Unlock()
		return err
	}

	if err := c.markCached("pokemon", formID); err != nil {
		c.writeMu.Unlock()
		return err
	}
	c.writeMu.Unlock()

	// ── Phase 3: seed evolution chain (acquires writeMu independently) ───────
	if chainErr == nil && chainID > 0 {
		if err := c.EnsureEvolutionChain(db, chainID); err != nil {
			logWarn("EnsureEvolutionChain %d: %v", chainID, err)
		}
	}

	// ── Phase 4: seed direct evolution targets' full data (background) ───────
	// evolution_condition is now populated; fire background seeding for each
	// direct evolution so learnset data is available for evo-note annotations.
	evoRows, evoErr := db.Query(
		`SELECT DISTINCT to_form_id FROM evolution_condition WHERE from_form_id = ?`,
		formID,
	)
	if evoErr == nil {
		var evoFormIDs []int
		for evoRows.Next() {
			var eid int
			if evoRows.Scan(&eid) == nil {
				evoFormIDs = append(evoFormIDs, eid)
			}
		}
		evoRows.Close()
		for _, eid := range evoFormIDs {
			c.GoEnsurePokemon(db, eid, versionGroupID)
		}
	}

	return nil
}

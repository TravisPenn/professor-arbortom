package pokeapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// evolutionChainResponse is the PokeAPI /evolution-chain/{id} response.
type evolutionChainResponse struct {
	ID    int       `json:"id"`
	Chain chainLink `json:"chain"`
}

type chainLink struct {
	Species struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"species"`
	EvolvesTo        []chainLink `json:"evolves_to"`
	EvolutionDetails []evoDetail `json:"evolution_details"`
}

type evoDetail struct {
	Trigger struct {
		Name string `json:"name"`
	} `json:"trigger"`
	MinLevel *int `json:"min_level"`
	Item     *struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"item"`
	HeldItem *struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"held_item"`
	MinHappiness *int `json:"min_happiness"` // API field is min_happiness, not min_friendship
	TradeSpecies *struct {
		Name string `json:"name"`
	} `json:"trade_species"`
}

// EnsureEvolutionChain fetches and caches evolution chain data for the species
// containing the given form ID.
func (c *Client) EnsureEvolutionChain(db *sql.DB, chainID int) error {
	cached, err := c.isCached("evolution-chain", chainID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	var chain evolutionChainResponse
	if err := c.get(fmt.Sprintf("%s/evolution-chain/%d", baseURL, chainID), &chain); err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := walkChain(tx, chain.Chain); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return c.markCached("evolution-chain", chainID)
}

func walkChain(tx *sql.Tx, link chainLink) error {
	fromSpeciesID := extractIDFromURL(link.Species.URL)

	// Resolve form ID for this species (default form has id == species id in Gen 3)
	var fromFormID int
	err := tx.QueryRow(
		`SELECT id FROM pokemon WHERE id = ? LIMIT 1`, fromSpeciesID,
	).Scan(&fromFormID)
	if err == sql.ErrNoRows {
		// Form not yet seeded — skip silently
		return nil
	} else if err != nil {
		return err
	}

	for _, nextLink := range link.EvolvesTo {
		toSpeciesID := extractIDFromURL(nextLink.Species.URL)

		var toFormID int
		err := tx.QueryRow(
			`SELECT id FROM pokemon WHERE id = ? LIMIT 1`, toSpeciesID,
		).Scan(&toFormID)
		if err == sql.ErrNoRows {
			// Target not seeded yet — insert a minimal stub so the evolution
			// condition row can be created. In Gen 3 the default form ID equals
			// the species ID, so this is safe.
			tx.Exec(`
				INSERT OR IGNORE INTO pokemon
					(id, species_name, form_name, type1, hp, attack, defense, sp_attack, sp_defense, speed)
				VALUES (?, ?, 'default', 'normal', 0, 0, 0, 0, 0, 0)
			`, toSpeciesID, nextLink.Species.Name) //nolint:errcheck
			toFormID = toSpeciesID
		} else if err != nil {
			return err
		}

		for _, detail := range nextLink.EvolutionDetails {
			conditions, _ := buildEvoConditions(detail)
			condJSON, _ := json.Marshal(conditions)

			trigger := detail.Trigger.Name
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO evolution_condition
				 (from_form_id, to_form_id, trigger, conditions_json)
				 VALUES (?, ?, ?, ?)`,
				fromFormID, toFormID, trigger, string(condJSON),
			); err != nil {
				logWarn("insert evolution_condition %d->%d: %v", fromFormID, toFormID, err)
			}
		}

		if err := walkChain(tx, nextLink); err != nil {
			return err
		}
	}
	return nil
}

func buildEvoConditions(d evoDetail) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	if d.MinLevel != nil {
		m["min_level"] = *d.MinLevel
	}
	if d.Item != nil {
		itemID := extractIDFromURL(d.Item.URL)
		m["item_id"] = itemID
	}
	if d.HeldItem != nil {
		heldItemID := extractIDFromURL(d.HeldItem.URL)
		m["held_item_id"] = heldItemID
	}
	if d.MinHappiness != nil {
		m["min_happiness"] = *d.MinHappiness
	}
	if d.TradeSpecies != nil {
		m["trade_species"] = d.TradeSpecies.Name
	}
	if d.Trigger.Name == "trade" && len(m) == 0 {
		m["trade"] = true
	}
	return m, nil
}

// EvoChainIDForSpecies resolves the evolution chain ID for a Pokemon species
// by fetching the species endpoint.
func (c *Client) EvoChainIDForSpecies(speciesID int) (int, error) {
	type speciesResp struct {
		EvolutionChain struct {
			URL string `json:"url"`
		} `json:"evolution_chain"`
	}
	var sr speciesResp
	if err := c.get(fmt.Sprintf("%s/pokemon-species/%d", baseURL, speciesID), &sr); err != nil {
		return 0, err
	}
	chainID := extractIDFromURL(sr.EvolutionChain.URL)
	if chainID == 0 {
		return 0, fmt.Errorf("pokeapi: could not resolve chain ID for species %d", speciesID)
	}
	return chainID, nil
}

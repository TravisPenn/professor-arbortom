package pokeapi

import (
	"database/sql"
	"fmt"
)

// itemResponse is a partial PokeAPI /item/{id} response.
type itemResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category struct {
		Name string `json:"name"`
	} `json:"category"`
	HeldByPokemon []interface{} `json:"held_by_pokemon"`
	GameIndices   []struct {
		GameIndex  int `json:"game_index"`
		Generation struct {
			Name string `json:"name"`
		} `json:"generation"`
	} `json:"game_indices"`
}

// EnsureItem fetches and caches item data.
// No-op if already cached.
func (c *Client) EnsureItem(db *sql.DB, itemID, versionID int) error {
	cached, err := c.isCached("item", itemID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	var item itemResponse
	if err := c.get(fmt.Sprintf("%s/item/%d", baseURL, itemID), &item); err != nil {
		return err
	}

	if _, err := db.Exec(
		`INSERT OR IGNORE INTO item (id, name, category) VALUES (?, ?, ?)`,
		item.ID, item.Name, item.Category.Name,
	); err != nil {
		return fmt.Errorf("pokeapi: insert item %s: %w", item.Name, err)
	}

	return c.markCached("item", itemID)
}

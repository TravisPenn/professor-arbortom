package legality

import (
	"database/sql"
	"fmt"
)

// LegalItems returns all items the player owns plus items legally obtainable
// at currently accessible locations.
func LegalItems(db *sql.DB, runID int) ([]Item, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, err
	}

	var items []Item

	// Items already owned
	ownedRows, err := db.Query(`
		SELECT i.id, i.name, i.category, ri.qty
		FROM run_item ri
		JOIN item i ON i.id = ri.item_id
		WHERE ri.run_id = ? AND ri.qty > 0
		ORDER BY i.category, i.name
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("legality: owned items query: %w", err)
	}
	defer ownedRows.Close()

	ownedSet := make(map[int]bool)
	for ownedRows.Next() {
		var it Item
		if err := ownedRows.Scan(&it.ItemID, &it.Name, &it.Category, &it.Qty); err != nil {
			return nil, err
		}
		it.Source = "owned"
		items = append(items, it)
		ownedSet[it.ItemID] = true
	}

	// Items obtainable at the current location
	if rs.LocationID != nil {
		obtRows, err := db.Query(`
			SELECT DISTINCT i.id, i.name, i.category
			FROM item_availability ia
			JOIN item i ON i.id = ia.item_id
			WHERE ia.location_id = ?
			  AND ia.version_id = ?
			ORDER BY i.category, i.name
		`, *rs.LocationID, rs.VersionID)
		if err != nil {
			return nil, fmt.Errorf("legality: obtainable items query: %w", err)
		}
		defer obtRows.Close()

		for obtRows.Next() {
			var it Item
			if err := obtRows.Scan(&it.ItemID, &it.Name, &it.Category); err != nil {
				return nil, err
			}
			if ownedSet[it.ItemID] {
				continue // already listed as owned
			}
			it.Source = "obtainable"
			items = append(items, it)
		}
	}

	return items, nil
}

// ShopItems returns items for sale at the run's current location.
// Uses the shop_item table which stores item names as TEXT, independent of
// PokeAPI hydration. Returns an empty slice when no location is set or when
// no shop data exists for the location.
func ShopItems(db *sql.DB, runID int) ([]Item, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, err
	}
	if rs.LocationID == nil {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT s.item_name, s.price, s.currency,
		       COALESCE(i.id, 0)         AS item_id,
		       COALESCE(i.category, '')  AS category,
		       COALESCE(tm.move_name, '') AS move_name
		FROM shop_item s
		LEFT JOIN item i ON i.name = s.item_name
		LEFT JOIN tm_move tm ON (
		    s.item_name LIKE 'tm%'
		    AND CAST(SUBSTR(s.item_name, 3) AS INTEGER) = tm.tm_number
		)
		WHERE s.location_id = ?
		ORDER BY s.price, s.item_name
	`, *rs.LocationID)
	if err != nil {
		return nil, fmt.Errorf("legality: shop items query: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.Name, &it.Price, &it.Currency, &it.ItemID, &it.Category, &it.MoveName); err != nil {
			return nil, err
		}
		it.Source = "shop"
		items = append(items, it)
	}
	return items, rows.Err()
}

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

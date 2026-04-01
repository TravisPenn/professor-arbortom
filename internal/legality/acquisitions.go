package legality

import (
	"database/sql"
	"fmt"
)

// LegalAcquisitions returns all Pokémon legally obtainable at the current
// run location. Blocked entries are included but annotated with BlockedByRule.
func LegalAcquisitions(db *sql.DB, runID int) ([]Acquisition, []Warning, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, nil, err
	}

	if rs.LocationID == nil {
		// No location set — return empty with advisory
		return nil, []Warning{{
			Code:    "no_location",
			Message: "No current location set — update progress first",
		}}, nil
	}

	rows, err := db.Query(`
		SELECT
			p.id             AS form_id,
			p.species_name,
			p.form_name,
			l.name           AS location_name,
			e.method,
			e.min_level,
			e.max_level
		FROM encounter e
		JOIN pokemon p ON p.id = e.form_id
		JOIN location l ON l.id = e.location_id
		WHERE e.location_id = ?
		  AND l.version_id = ?
		ORDER BY e.method, p.species_name
	`, *rs.LocationID, rs.VersionID)
	if err != nil {
		return nil, nil, fmt.Errorf("legality: acquisitions query: %w", err)
	}
	defer rows.Close()

	var acqs []Acquisition
	var warns []Warning

	for rows.Next() {
		var a Acquisition
		if err := rows.Scan(
			&a.FormID, &a.SpeciesName, &a.FormName,
			&a.LocationName, &a.Method, &a.MinLevel, &a.MaxLevel,
		); err != nil {
			return nil, nil, err
		}
		acqs = append(acqs, a)
	}
	if rows.Err() != nil {
		warns = append(warns, Warning{Code: "db_partial", Message: "partial results due to DB error"})
	}

	acqs, err = ApplyRules(db, rs, acqs)
	if err != nil {
		warns = append(warns, Warning{Code: "rules_error", Message: err.Error()})
	}

	return acqs, warns, nil
}

// LegalTrades returns in-game trades and Game Corner Pokémon available at the
// run's current location. Returns an empty slice (not an error) when no
// location is set or when the current location has no trade data.
func LegalTrades(db *sql.DB, runID int) ([]Trade, error) {
	rs, err := LoadRunState(db, runID)
	if err != nil {
		return nil, err
	}
	if rs.LocationID == nil {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT t.give_species, t.receive_species,
		       COALESCE(t.receive_nick, ''),
		       COALESCE(t.price_coins,  0),
		       COALESCE(t.notes, ''),
		       l.name
		FROM in_game_trade t
		JOIN location l ON l.id = t.location_id
		WHERE t.location_id = ?
		ORDER BY
		    CASE WHEN t.give_species IS NULL THEN 1 ELSE 0 END,
		    t.receive_species
	`, *rs.LocationID)
	if err != nil {
		return nil, fmt.Errorf("legality: trades query: %w", err)
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var giveSpec sql.NullString
		var t Trade
		if err := rows.Scan(
			&giveSpec, &t.ReceiveSpecies, &t.ReceiveNick,
			&t.PriceCoins, &t.Notes, &t.LocationName,
		); err != nil {
			return nil, err
		}
		if giveSpec.Valid {
			t.GiveSpecies = giveSpec.String
			t.Method = "trade"
		} else {
			t.Method = "game-corner"
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

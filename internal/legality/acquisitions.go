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
			pf.id            AS form_id,
			ps.name          AS species_name,
			pf.form_name,
			l.name           AS location_name,
			e.method,
			e.min_level,
			e.max_level
		FROM encounter e
		JOIN pokemon_form pf ON pf.id = e.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		JOIN location l ON l.id = e.location_id
		WHERE e.location_id = ?
		  AND l.version_id = ?
		ORDER BY ps.name, e.method
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

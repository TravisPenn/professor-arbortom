package legality

import (
	"database/sql"
	"fmt"
	"strings"
)

// FindPokemonLocations returns every encounter slot for a species in a game
// version. species is matched case-insensitively against pokemon.species_name.
// Returns an empty slice (not an error) when the species has no encounters in
// that version.
func FindPokemonLocations(db *sql.DB, versionID int, species string) ([]PokemonLocation, error) {
	rows, err := db.Query(`
		SELECT l.name, e.method, MIN(e.min_level), MAX(e.max_level)
		FROM encounter e
		JOIN pokemon p ON p.id = e.form_id
		JOIN location l ON l.id = e.location_id
		WHERE lower(p.species_name) = lower(?)
		  AND l.version_id = ?
		GROUP BY l.name, e.method
		ORDER BY l.name, e.method
	`, species, versionID)
	if err != nil {
		return nil, fmt.Errorf("legality: pokemon locations: %w", err)
	}
	defer rows.Close()

	var locs []PokemonLocation
	for rows.Next() {
		var loc PokemonLocation
		if err := rows.Scan(&loc.LocationName, &loc.Method, &loc.MinLevel, &loc.MaxLevel); err != nil {
			return nil, err
		}
		locs = append(locs, loc)
	}
	return locs, rows.Err()
}

// PreEvoLocations groups the catchable encounter slots for one pre-evolution
// species. Returned by FindPreEvoLocations when the target species itself has
// no wild encounters.
type PreEvoLocations struct {
	Species string
	Locs    []PokemonLocation
}

// FindPreEvoLocations walks the evolution chain backwards (up to 3 hops) and
// returns encounter data for any pre-evolution forms that are catchable in the
// given game version. Used to answer "where do I get X?" when X is only
// obtainable by evolving a catchable form (e.g. Poliwhirl → catch Poliwag).
// Returns an empty slice when no catchable pre-evolution is found.
func FindPreEvoLocations(db *sql.DB, versionID int, species string) ([]PreEvoLocations, error) {
	rows, err := db.Query(`
		WITH RECURSIVE prevo(form_id, depth) AS (
			SELECT ec.from_form_id, 1
			FROM evolution_condition ec
			JOIN pokemon p ON p.id = ec.to_form_id
			WHERE lower(p.species_name) = lower(?)
			UNION ALL
			SELECT ec.from_form_id, prevo.depth + 1
			FROM evolution_condition ec
			JOIN prevo ON prevo.form_id = ec.to_form_id
			WHERE prevo.depth < 3
		)
		SELECT p.species_name, l.name, e.method, MIN(e.min_level), MAX(e.max_level)
		FROM (SELECT DISTINCT form_id FROM prevo) pf
		JOIN encounter e ON e.form_id = pf.form_id
		JOIN pokemon p ON p.id = pf.form_id
		JOIN location l ON l.id = e.location_id
		WHERE l.version_id = ?
		GROUP BY p.species_name, l.name, e.method
		ORDER BY p.species_name, l.name, e.method
	`, species, versionID)
	if err != nil {
		return nil, fmt.Errorf("legality: pre-evo locations: %w", err)
	}
	defer rows.Close()

	grouped := map[string]*PreEvoLocations{}
	var order []string
	for rows.Next() {
		var preEvoSpecies string
		var loc PokemonLocation
		if err := rows.Scan(&preEvoSpecies, &loc.LocationName, &loc.Method, &loc.MinLevel, &loc.MaxLevel); err != nil {
			return nil, err
		}
		if _, ok := grouped[preEvoSpecies]; !ok {
			grouped[preEvoSpecies] = &PreEvoLocations{Species: preEvoSpecies}
			order = append(order, preEvoSpecies)
		}
		grouped[preEvoSpecies].Locs = append(grouped[preEvoSpecies].Locs, loc)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	result := make([]PreEvoLocations, 0, len(order))
	for _, s := range order {
		result = append(result, *grouped[s])
	}
	return result, nil
}

// GetPokemonBasicInfo returns type and ability data for a species by name.
// Returns nil (not an error) when the species is not in the database.
func GetPokemonBasicInfo(db *sql.DB, species string) (*PokemonBasicInfo, error) {
	var info PokemonBasicInfo
	var type2, ability sql.NullString
	err := db.QueryRow(`
		SELECT p.species_name, p.type1, p.type2, p.ability1
		FROM pokemon p
		WHERE lower(p.species_name) = lower(?)
		  AND p.form_name = 'default'
		LIMIT 1
	`, species).Scan(&info.SpeciesName, &info.Type1, &type2, &ability)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("legality: pokemon basic info: %w", err)
	}
	if type2.Valid {
		info.Type2 = type2.String
	}
	if ability.Valid {
		info.Ability = ability.String
	}
	return &info, nil
}

// FindItemLocations returns the location names where an item is obtainable in
// a game version. itemName is matched case-insensitively against item.name.
// Returns an empty slice when the item has no recorded availability.
func FindItemLocations(db *sql.DB, versionID int, itemName string) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT l.name
		FROM item_availability ia
		JOIN item i ON i.id = ia.item_id
		JOIN location l ON l.id = ia.location_id
		WHERE lower(i.name) = lower(?)
		  AND ia.version_id = ?
		ORDER BY l.name
	`, itemName, versionID)
	if err != nil {
		return nil, fmt.Errorf("legality: item locations: %w", err)
	}
	defer rows.Close()

	var locs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		locs = append(locs, name)
	}
	return locs, rows.Err()
}

// VersionSpeciesNames returns all species names that have at least one
// encounter entry in the given game version. Used for entity extraction from
// player questions.
func VersionSpeciesNames(db *sql.DB, versionID int) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT p.species_name
		FROM pokemon p
		JOIN encounter e ON e.form_id = p.id
		JOIN location l ON l.id = e.location_id
		WHERE l.version_id = ?
		ORDER BY p.species_name
	`, versionID)
	if err != nil {
		return nil, fmt.Errorf("legality: version species names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// AllPokemonSpeciesNames returns every distinct species name present in the
// pokemon table. Used for broader on-demand entity extraction — catches
// species that aren't wild-catchable in the current version (e.g. Poliwhirl,
// which is only obtainable by evolving Poliwag).
func AllPokemonSpeciesNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT species_name FROM pokemon ORDER BY species_name`)
	if err != nil {
		return nil, fmt.Errorf("legality: all pokemon species names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// FindPreEvolutionNames returns the species names of forms that directly evolve
// into the given species, without requiring location data. Used as a last-resort
// fallback when encounter data hasn't been seeded for the pre-evo forms yet.
func FindPreEvolutionNames(db *sql.DB, species string) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT p_pre.species_name
		FROM evolution_condition ec
		JOIN pokemon p_target ON p_target.id = ec.to_form_id
		  AND lower(p_target.species_name) = lower(?)
		JOIN pokemon p_pre ON p_pre.id = ec.from_form_id
		ORDER BY p_pre.species_name
	`, species)
	if err != nil {
		return nil, fmt.Errorf("legality: pre-evolution names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// SpeciesInText reports whether a species name (PokeAPI slug, e.g. "mr-mime")
// appears as a complete word in text. Case-insensitive; hyphens in the species
// name are treated as spaces so "mr-mime" matches "mr mime" and "Mr. Mime".
func SpeciesInText(text, species string) bool {
	normText := strings.ToLower(strings.ReplaceAll(text, "-", " "))
	normSpecies := strings.ToLower(strings.ReplaceAll(species, "-", " "))
	idx := strings.Index(normText, normSpecies)
	if idx == -1 {
		return false
	}
	// Preceding character must not be alphanumeric (word boundary).
	if idx > 0 {
		prev := normText[idx-1]
		if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
			return false
		}
	}
	// Trailing character must not be alphanumeric.
	end := idx + len(normSpecies)
	if end < len(normText) {
		next := normText[end]
		if (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
			return false
		}
	}
	return true
}

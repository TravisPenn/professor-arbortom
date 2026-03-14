package handlers

import (
	"database/sql"
	"strings"
)

// ─── Run loaders ──────────────────────────────────────────────────────────────

// loadRunSummaries returns all runs (active and archived) sorted by updated_at DESC.
// The caller splits them by rs.Archived.
func loadRunSummaries(db *sql.DB) ([]RunSummary, error) {
	rows, err := db.Query(`
		SELECT
			r.id, r.name, u.name AS user_name, gv.name AS version_name,
			COALESCE(rp.badge_count, 0) AS badge_count,
			COALESCE(rp.updated_at, r.created_at) AS updated_at,
			COALESCE(r.archived_at, '') AS archived_at
		FROM run r
		JOIN user u ON u.id = r.user_id
		JOIN game_version gv ON gv.id = r.version_id
		LEFT JOIN run_progress rp ON rp.run_id = r.id
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []RunSummary
	for rows.Next() {
		var rs RunSummary
		var archivedAt string
		if err := rows.Scan(&rs.ID, &rs.Name, &rs.UserName, &rs.VersionName, &rs.BadgeCount, &rs.UpdatedAt, &archivedAt); err != nil {
			return nil, err
		}
		rs.Archived = archivedAt != ""

		// Load active rules for this run
		ruleRows, err := db.Query(`
			SELECT rd.key FROM run_rule rr
			JOIN rule_def rd ON rd.id = rr.rule_def_id
			WHERE rr.run_id = ? AND rr.enabled = 1
		`, rs.ID)
		if err == nil {
			defer ruleRows.Close()
			for ruleRows.Next() {
				var key string
				ruleRows.Scan(&key) //nolint:errcheck
				rs.ActiveRules = append(rs.ActiveRules, key)
			}
		}

		runs = append(runs, rs)
	}
	return runs, rows.Err()
}

func loadVersionOptions(db *sql.DB) ([]VersionOption, error) {
	rows, err := db.Query(`SELECT id, name FROM game_version ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opts []VersionOption
	for rows.Next() {
		var o VersionOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		o.Name = capitalizeVersion(o.Name)
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

func loadStartersByVersion(db *sql.DB) (map[int][]StarterOption, error) {
	rows, err := db.Query(`
		SELECT gs.version_id, gs.form_id, ps.name
		FROM game_starter gs
		JOIN pokemon_form pf ON pf.id = gs.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		ORDER BY gs.version_id, gs.priority
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]StarterOption)
	for rows.Next() {
		var versionID, formID int
		var name string
		if err := rows.Scan(&versionID, &formID, &name); err != nil {
			return nil, err
		}
		result[versionID] = append(result[versionID], StarterOption{
			FormID:      formID,
			SpeciesName: capitalizeVersion(name),
		})
	}
	return result, rows.Err()
}

// capitalizeVersion converts a hyphenated name like "fire-red" to "Fire Red".
func capitalizeVersion(name string) string {
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// ─── Location & flag loaders ──────────────────────────────────────────────────

func loadLocations(db *sql.DB, versionID int) ([]LocationOption, error) {
	rows, err := db.Query(
		`SELECT id, name, region FROM location WHERE version_id = ? ORDER BY region, name`,
		versionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locs []LocationOption
	for rows.Next() {
		var l LocationOption
		if err := rows.Scan(&l.ID, &l.Name, &l.Region); err != nil {
			return nil, err
		}
		locs = append(locs, l)
	}
	return locs, rows.Err()
}

// loadEncountersByLocation returns a map from location ID to a deduplicated,
// sorted list of catchable Pokémon (with aggregate level range) for versionID.
// min_level == max_level indicates a fixed-level encounter (e.g. static legendary).
func loadEncountersByLocation(db *sql.DB, versionID int) (map[int][]EncounterOption, error) {
	rows, err := db.Query(`
		SELECT e.location_id, ps.name, MIN(e.min_level), MAX(e.max_level)
		FROM encounter e
		JOIN pokemon_form pf ON pf.id = e.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		JOIN location l ON l.id = e.location_id
		WHERE l.version_id = ?
		GROUP BY e.location_id, ps.id
		ORDER BY e.location_id, ps.name
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]EncounterOption)
	for rows.Next() {
		var locID, minLvl, maxLvl int
		var name string
		if err := rows.Scan(&locID, &name, &minLvl, &maxLvl); err != nil {
			return nil, err
		}
		result[locID] = append(result[locID], EncounterOption{
			Name:     capitalizeVersion(name),
			MinLevel: minLvl,
			MaxLevel: maxLvl,
		})
	}
	return result, rows.Err()
}

// gen3FlagDefs defines known story flags for all Gen 3 games.
// These are displayed as checkboxes on the progress screen.
var gen3FlagDefs = []FlagDef{
	{Key: "hm.cut_obtained", Label: "Got HM Cut", Description: "Enables Cut on the field and unlocks cut-locked areas"},
	{Key: "hm.fly_obtained", Label: "Got HM Fly", Description: "Enables Fly between towns"},
	{Key: "hm.surf_obtained", Label: "Got HM Surf", Description: "Enables Surf encounters and water routes"},
	{Key: "hm.strength_obtained", Label: "Got HM Strength", Description: "Enables moving boulders"},
	{Key: "hm.flash_obtained", Label: "Got HM Flash", Description: "Enables Flash in dark areas"},
	{Key: "hm.rock_smash_obtained", Label: "Got HM Rock Smash", Description: "Enables smashing rocks"},
	{Key: "hm.waterfall_obtained", Label: "Got HM Waterfall", Description: "Enables climbing waterfalls"},
	{Key: "hm.dive_obtained", Label: "Got HM Dive", Description: "Enables diving (Emerald / R/S only)"},
}

func loadFlags(db *sql.DB, runID, versionID int) ([]FlagDef, map[string]bool, error) {
	rows, err := db.Query(
		`SELECT key, value FROM run_flag WHERE run_id = ?`, runID,
	)
	if err != nil {
		return gen3FlagDefs, map[string]bool{}, err
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err == nil {
			active[key] = val == "true"
		}
	}
	return gen3FlagDefs, active, nil
}

// getHydrationStats returns the total number of location areas for a game version
// and how many of those areas have encounter data cached in api_cache_log.
func getHydrationStats(db *sql.DB, versionID int) (total, seeded int, err error) {
	if err = db.QueryRow(
		`SELECT COUNT(*) FROM location WHERE version_id = ? AND id > 0`, versionID,
	).Scan(&total); err != nil {
		return
	}
	err = db.QueryRow(`
		SELECT COUNT(DISTINCT l.id)
		FROM location l
		JOIN api_cache_log a ON a.resource = 'location-area' AND a.resource_id = l.id
		WHERE l.version_id = ? AND l.id > 0
	`, versionID).Scan(&seeded)
	return
}

// ─── Route log loader ─────────────────────────────────────────────────────────

func loadRouteLog(db *sql.DB, runID int, nuzlockeOn bool) ([]RouteEntry, error) {
	rows, err := db.Query(`
		SELECT
			COALESCE(l.name, 'unknown') AS loc_name,
			ps.name AS species_name,
			'caught' AS outcome,
			rp.level,
			rp.met_location_id
		FROM run_pokemon rp
		JOIN pokemon_form pf ON pf.id = rp.form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
		LEFT JOIN location l ON l.id = rp.met_location_id
		WHERE rp.run_id = ?
		ORDER BY rp.id DESC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build duplicate set if Nuzlocke
	duplicateLocations := make(map[int]int) // locationID → count
	if nuzlockeOn {
		dupRows, err := db.Query(
			`SELECT met_location_id FROM run_pokemon WHERE run_id = ? AND met_location_id IS NOT NULL`,
			runID,
		)
		if err == nil {
			defer dupRows.Close()
			for dupRows.Next() {
				var lid int
				if dupRows.Scan(&lid) == nil {
					duplicateLocations[lid]++
				}
			}
		}
	}

	var log []RouteEntry
	for rows.Next() {
		var e RouteEntry
		var locID *int
		if err := rows.Scan(&e.LocationName, &e.SpeciesName, &e.Outcome, &e.Level, &locID); err != nil {
			continue
		}
		if nuzlockeOn && locID != nil && duplicateLocations[*locID] > 1 {
			e.IsDuplicate = true
		}
		log = append(log, e)
	}
	return log, rows.Err()
}

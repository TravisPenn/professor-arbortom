package pokeapi

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// locationAreaResponse is a partial PokeAPI /location-area/{id} response.
type locationAreaResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Location struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"location"`
	PokemonEncounters []struct {
		Pokemon struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"pokemon"`
		VersionDetails []struct {
			Version struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"version"`
			MaxChance        int `json:"max_chance"`
			EncounterDetails []struct {
				MinLevel int `json:"min_level"`
				MaxLevel int `json:"max_level"`
				Method   struct {
					Name string `json:"name"`
				} `json:"method"`
				ConditionValues []struct {
					Name string `json:"name"`
				} `json:"condition_values"`
			} `json:"encounter_details"`
		} `json:"version_details"`
	} `json:"pokemon_encounters"`
}

// gen3VersionIDs maps PokeAPI version names to IDs (Gen 3 only).
var gen3VersionIDs = map[string]int{
	"ruby":      6,
	"sapphire":  7,
	"emerald":   8,
	"firered":   10,
	"leafgreen": 11,
}

// gen3VersionRegion maps version names to region strings.
var gen3VersionRegion = map[string]string{
	"ruby": "hoenn", "sapphire": "hoenn", "emerald": "hoenn",
	"firered": "kanto", "leafgreen": "kanto",
}

// EnsureLocationEncounters fetches and caches encounter data for a location area ID
// for the given PokeAPI version ID.
//
// Phase 1: fetch area JSON from API (concurrent HTTP, no DB lock).
// Phase 2: ensure all referenced Pokémon are seeded via EnsurePokemon, which
//
//	handles its own write serialisation internally.
//
// Phase 3: write location + encounter rows under writeMu so no concurrent
//
//	write transaction can produce SQLITE_BUSY_SNAPSHOT in WAL mode.
func (c *Client) EnsureLocationEncounters(db *sql.DB, locationAreaID, versionID int) error {
	cached, err := c.isCached("location-area", locationAreaID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	// ── Phase 1: fetch area from PokeAPI ─────────────────────────────────────
	var area locationAreaResponse
	if err := c.get(fmt.Sprintf("%s/location-area/%d", baseURL, locationAreaID), &area); err != nil {
		return err
	}

	// ── Phase 2: ensure all referenced Pokémon exist (each call is
	//             independently serialised by writeMu inside EnsurePokemon) ───
	seenForms := map[int]bool{}
	for _, pe := range area.PokemonEncounters {
		formID := extractIDFromURL(pe.Pokemon.URL)
		if formID == 0 || seenForms[formID] {
			continue
		}
		seenForms[formID] = true

		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM pokemon_form WHERE id = ?`, formID).Scan(&exists) //nolint:errcheck
		if exists > 0 {
			continue
		}

		for _, vd := range pe.VersionDetails {
			vid, ok := gen3VersionIDs[vd.Version.Name]
			if !ok {
				continue
			}
			if fetchErr := c.EnsurePokemon(db, formID, versionGroupForVersion(vid)); fetchErr != nil {
				logWarn("EnsurePokemon %d: %v", formID, fetchErr)
			}
			break
		}
	}

	// ── Phase 3: write encounter data (serialised by writeMu) ────────────────
	return func() error {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback() //nolint:errcheck

		for _, pe := range area.PokemonEncounters {
			for _, vd := range pe.VersionDetails {
				vid, ok := gen3VersionIDs[vd.Version.Name]
				if !ok {
					continue // not Gen 3
				}

				region := gen3VersionRegion[vd.Version.Name]

				// Upsert location
				var locID int
				err := tx.QueryRow(
					`SELECT id FROM location WHERE name = ? AND version_id = ?`,
					area.Location.Name, vid,
				).Scan(&locID)
				if err == sql.ErrNoRows {
					res, insertErr := tx.Exec(
						`INSERT OR IGNORE INTO location (name, version_id, region) VALUES (?, ?, ?)`,
						area.Location.Name, vid, region,
					)
					if insertErr != nil {
						logWarn("insert location %s: %v", area.Location.Name, insertErr)
						continue
					}
					id, _ := res.LastInsertId()
					locID = int(id)
				} else if err != nil {
					return err
				}

				// Resolve form ID from URL: .../pokemon/16/
				formID := extractIDFromURL(pe.Pokemon.URL)
				if formID == 0 {
					continue
				}

				// Skip if EnsurePokemon failed for this form earlier.
				var exists int
				tx.QueryRow(`SELECT COUNT(*) FROM pokemon_form WHERE id = ?`, formID).Scan(&exists) //nolint:errcheck
				if exists == 0 {
					continue
				}

				for _, ed := range vd.EncounterDetails {
					conditions := buildConditionsJSON(ed.ConditionValues)
					if _, insertErr := tx.Exec(
						`INSERT OR IGNORE INTO encounter
					 (location_id, form_id, min_level, max_level, method, conditions_json)
					 VALUES (?, ?, ?, ?, ?, ?)`,
						locID, formID, ed.MinLevel, ed.MaxLevel, ed.Method.Name, conditions,
					); insertErr != nil {
						logWarn("insert encounter form %d @ loc %d: %v", formID, locID, insertErr)
					}
				}
				_ = versionID // caller may pass 0 to mean "all versions in area"
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		// markCached must run inside writeMu: if called after unlock, another
		// goroutine could BEGIN a transaction, then our INSERT into api_cache_log
		// would post-date their snapshot → SQLITE_BUSY_SNAPSHOT.
		return c.markCached("location-area", locationAreaID)
	}()
}

func extractIDFromURL(url string) int {
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.Atoi(parts[len(parts)-1])
	return id
}

func versionGroupForVersion(versionID int) int {
	switch versionID {
	case 6, 7:
		return 5 // ruby-sapphire
	case 8:
		return 6 // emerald
	case 10, 11:
		return 7 // firered-leafgreen
	}
	return 0
}

func buildConditionsJSON(conditions []struct {
	Name string `json:"name"`
}) string {
	if len(conditions) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, c := range conditions {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('"')
		sb.WriteString(c.Name)
		sb.WriteByte('"')
	}
	sb.WriteByte(']')
	return sb.String()
}

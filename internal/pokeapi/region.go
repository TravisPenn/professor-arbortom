package pokeapi

import (
	"database/sql"
	"fmt"
	"sync"

	pkgdb "github.com/TravisPenn/professor-arbortom/internal/db"
)

// regionResponse is a partial PokeAPI /region/{id} response.
type regionResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Locations []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"locations"`
}

// locationDetailResponse is a partial PokeAPI /location/{id} response.
type locationDetailResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Areas []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"areas"`
	Region struct {
		Name string `json:"name"`
	} `json:"region"`
}

// gen3RegionVersionIDs maps PokeAPI region ID → list of Gen3 game_version.id values.
var gen3RegionVersionIDs = map[int][]int{
	1: {10, 11},  // Kanto  → FireRed, LeafGreen
	3: {6, 7, 8}, // Hoenn  → Ruby, Sapphire, Emerald
}

// gen3RegionName maps PokeAPI region ID → region name string stored in location.region.
var gen3RegionName = map[int]string{
	1: "kanto",
	3: "hoenn",
}

// RegionIDForVersionID returns the PokeAPI region ID for a Gen3 game_version.id.
// Returns 0 if unknown.
func RegionIDForVersionID(versionID int) int {
	switch versionID {
	case 10, 11:
		return 1 // kanto
	case 6, 7, 8:
		return 3 // hoenn
	}
	return 0
}

// EnsureRegionLocations seeds the location table with every location-area for the
// given PokeAPI region ID (1=Kanto, 3=Hoenn). Only inserts name+id — encounter data
// is fetched lazily by EnsureLocationEncounters when the user navigates to that area.
// Idempotent: skips silently if already cached.
func (c *Client) EnsureRegionLocations(db *sql.DB, regionID int) error {
	versionIDs, ok := gen3RegionVersionIDs[regionID]
	if !ok {
		return nil // unsupported region
	}

	cached, err := c.isCached("region", regionID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	var reg regionResponse
	if err := c.get(fmt.Sprintf("%s/region/%d", baseURL, regionID), &reg); err != nil {
		return fmt.Errorf("fetch region %d: %w", regionID, err)
	}

	regionName := gen3RegionName[regionID]

	// Fetch each location in parallel (max 10 concurrent).
	type areaRow struct {
		id   int
		name string
	}

	locURLs := make([]string, len(reg.Locations))
	for i, l := range reg.Locations {
		locURLs[i] = l.URL
	}

	results := make([][]areaRow, len(locURLs))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for i, url := range locURLs {
		wg.Add(1)
		go func(idx int, locURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var loc locationDetailResponse
			if err := c.get(locURL, &loc); err != nil {
				logWarn("fetch location %s: %v", locURL, err)
				return
			}
			var rows []areaRow
			for _, area := range loc.Areas {
				areaID := extractIDFromURL(area.URL)
				if areaID == 0 {
					continue
				}
				rows = append(rows, areaRow{id: areaID, name: loc.Name})
			}
			results[idx] = rows
		}(i, url)
	}
	wg.Wait()

	// Bulk insert into location table (serialised so encounter seeder goroutines
	// cannot observe a stale WAL snapshot).
	return func() error {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback() //nolint:errcheck

		for _, rowSet := range results {
			for _, row := range rowSet {
				for _, vid := range versionIDs {
					if _, err := tx.Exec(
						`INSERT OR IGNORE INTO location (id, name, version_id, region) VALUES (?, ?, ?, ?)`,
						row.id, row.name, vid, regionName,
					); err != nil {
						logWarn("insert location area %d v%d: %v", row.id, vid, err)
					}
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return c.markCached("region", regionID)
	}()
}

// EnsureAllEncounters seeds encounter data for every unseeded location area
// that exists in the location table for the given game version ID.
// Intended to be called in a goroutine — errors are logged, not returned.
// Already-cached areas are skipped by EnsureLocationEncounters automatically.
func (c *Client) EnsureAllEncounters(db *sql.DB, versionID int) {
	rows, err := db.Query(`
		SELECT l.id FROM location l
		WHERE l.version_id = ?
		  AND l.id > 0
		  AND NOT EXISTS (
		    SELECT 1 FROM api_cache_log a
		    WHERE a.resource = 'location-area' AND a.resource_id = l.id
		  )
		ORDER BY l.id
	`, versionID)
	if err != nil {
		logWarn("EnsureAllEncounters query v%d: %v", versionID, err)
		return
	}

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	if len(ids) == 0 {
		return
	}

	sem := make(chan struct{}, 5) // max 5 concurrent PokeAPI calls
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(locID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := c.EnsureLocationEncounters(db, locID, versionID); err != nil {
				logWarn("EnsureAllEncounters area %d: %v", locID, err)
			}
		}(id)
	}
	wg.Wait()

	// Export all reference data to seeds.sql so future fresh installs don't
	// need to re-fetch from PokeAPI.
	if c.dbPath != "" {
		if err := pkgdb.ExportSeeds(db, c.dbPath); err != nil {
			logWarn("ExportSeeds: %v", err)
		}
	}
}

// Package pokeapi provides a caching HTTP client for https://pokeapi.co.
// Only Gen 3 version groups (5, 6, 7) are supported.
// Resources are fetched at most once; subsequent calls are served from the
// SQLite cache without hitting the network.
package pokeapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// gen3VersionGroups is the complete set of version group IDs supported at this scope.
var gen3VersionGroups = map[int]struct{}{
	5: {}, // ruby-sapphire
	6: {}, // emerald
	7: {}, // firered-leafgreen
}

const baseURL = "https://pokeapi.co/api/v2"

// Client wraps the PokeAPI HTTP client and an SQLite connection for caching.
type Client struct {
	http    *http.Client
	db      *sql.DB
	dbPath  string     // path to the on-disk DB file; used for seeds export
	writeMu sync.Mutex // serialises SQLite write transactions during concurrent seeding
}

// New creates a Client with a 30-second timeout.
// dbPath is the filesystem path to the SQLite file (used for seeds.sql export).
func New(db *sql.DB, dbPath string) *Client {
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		db:     db,
		dbPath: dbPath,
	}
}

// isCached returns true if (resource, resourceID) exists in api_cache_log.
func (c *Client) isCached(resource string, resourceID int) (bool, error) {
	var count int
	err := c.db.QueryRow(
		`SELECT COUNT(*) FROM api_cache_log WHERE resource = ? AND resource_id = ?`,
		resource, resourceID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// markCached inserts a row into api_cache_log.
func (c *Client) markCached(resource string, resourceID int) error {
	_, err := c.db.Exec(
		`INSERT OR IGNORE INTO api_cache_log (resource, resource_id, fetched_at) VALUES (?, ?, ?)`,
		resource, resourceID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// get fetches a PokeAPI endpoint and unmarshals the JSON into dest.
// Every request is logged with its HTTP status and round-trip duration.
func (c *Client) get(url string, dest interface{}) error {
	start := time.Now()
	log.Printf("[pokeapi] GET %s", url)

	resp, err := c.http.Get(url)
	if err != nil {
		log.Printf("[pokeapi] ERR %s (%s): %v", url, time.Since(start).Round(time.Millisecond), err)
		return fmt.Errorf("pokeapi: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	log.Printf("[pokeapi] %d %s (%s)", resp.StatusCode, url, time.Since(start).Round(time.Millisecond))

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("pokeapi: %s not found (404)", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pokeapi: %s returned %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pokeapi: read body %s: %w", url, err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("pokeapi: unmarshal %s: %w", url, err)
	}
	return nil
}

// assertGen3 returns an error if versionGroupID is not in the supported set.
func assertGen3(versionGroupID int) error {
	if _, ok := gen3VersionGroups[versionGroupID]; !ok {
		return fmt.Errorf("pokeapi: version group %d is outside Gen 3 scope {5,6,7}", versionGroupID)
	}
	return nil
}

// logWarn logs a warning without returning an error — callers continue with partial data.
func logWarn(format string, args ...interface{}) {
	log.Printf("[pokeapi warn] "+format, args...)
}

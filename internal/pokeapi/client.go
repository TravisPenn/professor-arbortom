// Package pokeapi provides a caching HTTP client for https://pokeapi.co.
// Only Gen 3 version groups (5, 6, 7) are supported.
// Resources are fetched at most once; subsequent calls are served from the
// SQLite cache without hitting the network.
package pokeapi

import (
	"database/sql"
	"encoding/json"
	"errors"
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

// ErrNotFound is returned by get() when the PokeAPI responds with 404.
// Callers can check errors.Is(err, ErrNotFound) to distinguish a permanently
// missing resource from a transient network error.
var ErrNotFound = errors.New("pokeapi: not found (404)")

// Client wraps the PokeAPI HTTP client and an SQLite connection for caching.
type Client struct {
	http    *http.Client
	db      *sql.DB
	dbPath  string     // path to the on-disk DB file; used for seeds export
	writeMu sync.Mutex // serialises SQLite write transactions during concurrent seeding

	// SEC-011: Track in-flight background goroutines for graceful shutdown
	// and prevent duplicate seeding operations.
	wg       sync.WaitGroup
	inflight sync.Map // key: string (operation key) → struct{}
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

// Wait blocks until all in-flight background operations complete.
// SEC-011: Called during graceful shutdown to avoid DB writes after close.
func (c *Client) Wait() {
	c.wg.Wait()
}

// tryStart marks an operation as in-flight and increments the WaitGroup.
// Returns false if the operation is already in progress (deduplication).
func (c *Client) tryStart(key string) bool {
	if _, loaded := c.inflight.LoadOrStore(key, struct{}{}); loaded {
		return false // already running
	}
	c.wg.Add(1)
	return true
}

// done marks an operation as finished.
func (c *Client) done(key string) {
	c.inflight.Delete(key)
	c.wg.Done()
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
		return fmt.Errorf("pokeapi: %s not found (404): %w", url, ErrNotFound)
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

// ─── SEC-011: Deduplicated background goroutine launchers ─────────────────────

// GoEnsureRegionLocations runs EnsureRegionLocations in a tracked, deduplicated
// background goroutine.
func (c *Client) GoEnsureRegionLocations(db *sql.DB, regionID int) {
	key := fmt.Sprintf("region:%d", regionID)
	if !c.tryStart(key) {
		return
	}
	go func() {
		defer c.done(key)
		_ = c.EnsureRegionLocations(db, regionID)
	}()
}

// GoEnsureAllEncounters runs EnsureAllEncounters in a tracked, deduplicated
// background goroutine.
func (c *Client) GoEnsureAllEncounters(db *sql.DB, versionID int) {
	key := fmt.Sprintf("encounters:%d", versionID)
	if !c.tryStart(key) {
		return
	}
	go func() {
		defer c.done(key)
		c.EnsureAllEncounters(db, versionID)
	}()
}

// GoEnsureLocationEncounters runs EnsureLocationEncounters in a tracked,
// deduplicated background goroutine.
func (c *Client) GoEnsureLocationEncounters(db *sql.DB, locationAreaID, versionID int) {
	key := fmt.Sprintf("loc-enc:%d:%d", locationAreaID, versionID)
	if !c.tryStart(key) {
		return
	}
	go func() {
		defer c.done(key)
		_ = c.EnsureLocationEncounters(db, locationAreaID, versionID)
	}()
}

// GoEnsurePokemon runs EnsurePokemon in a tracked, deduplicated background
// goroutine.
func (c *Client) GoEnsurePokemon(db *sql.DB, formID, versionGroupID int) {
	key := fmt.Sprintf("pokemon:%d:%d", formID, versionGroupID)
	if !c.tryStart(key) {
		return
	}
	go func() {
		defer c.done(key)
		_ = c.EnsurePokemon(db, formID, versionGroupID)
	}()
}

package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TravisPenn/professor-arbortom/internal/models"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

// ─── shared test helpers ──────────────────────────────────────────────────────

func newHandlerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func hMustExec(t *testing.T, db *sql.DB, q string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// bootstrapProgressDB creates the minimal schema for UpdateProgress tests.
//
//   - game_version, user, run (with badge_count/current_location_id), run_setting, location, api_cache_log
//   - inserts version 10 (firered) and run id=1 with no progress yet
func bootstrapProgressDB(t *testing.T) *sql.DB {
	t.Helper()
	db := newHandlerDB(t)
	hMustExec(t, db, `CREATE TABLE game_version (id INTEGER PRIMARY KEY, name TEXT, version_group_id INTEGER, generation_id INTEGER)`)
	hMustExec(t, db, `CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT UNIQUE)`)
	hMustExec(t, db, `CREATE TABLE run (id INTEGER PRIMARY KEY, user_id INTEGER, version_id INTEGER, name TEXT, archived_at TEXT, badge_count INTEGER, current_location_id INTEGER, progress_updated_at TEXT)`)
	hMustExec(t, db, `CREATE TABLE run_setting (run_id INTEGER, type TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL DEFAULT '', PRIMARY KEY (run_id, type, key))`)
	hMustExec(t, db, `CREATE TABLE location (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, region TEXT)`)
	hMustExec(t, db, `CREATE TABLE api_cache_log (id INTEGER PRIMARY KEY, resource TEXT, resource_id INTEGER)`)

	hMustExec(t, db, `INSERT INTO game_version VALUES (10, 'firered', 7, 3)`)
	hMustExec(t, db, `INSERT INTO user VALUES (1, 'tester')`)
	hMustExec(t, db, `INSERT INTO run VALUES (1, 1, 10, 'test run', NULL, NULL, NULL, NULL)`)

	// Add a static town (negative ID) so that loadLocations returns results
	// even before PokeAPI seeding — this is what migration 005 provides.
	hMustExec(t, db, `INSERT INTO location VALUES (-1, 'Pallet Town', 10, 'kanto')`)
	return db
}

// postForm fires a POST through the given gin router and returns the recorder.
func postForm(t *testing.T, router *gin.Engine, path string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// injectRunContext sets the "run", "progress", "active_rules", and "version"
// context keys that RunContextMiddleware would normally populate — so tests
// can call handler functions directly without a full DB middleware chain.
func injectRunContext(run models.Run, progress models.RunProgress) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("run", run)
		c.Set("progress", progress)
		c.Set("active_rules", []models.ActiveRule{})
		c.Set("version", models.GameVersion{ID: run.VersionID, Name: "firered", VersionGroupID: 7})
		c.Next()
	}
}

// ─── UpdateProgress tests ─────────────────────────────────────────────────────

// newProgressRouter builds a minimal gin router that wires UpdateProgress
// under POST /runs/:run_id/progress with a pre-injected run context.
func newProgressRouter(db *sql.DB, run models.Run) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/runs/:run_id/progress",
		injectRunContext(run, models.RunProgress{RunID: run.ID}),
		UpdateProgress(db, nil), // nil pokeClient — no background seeding in tests
	)
	return r
}

func savedLocationID(t *testing.T, db *sql.DB, runID int) *int {
	t.Helper()
	var id *int
	db.QueryRow(`SELECT current_location_id FROM run WHERE id = ?`, runID).Scan(&id) //nolint:errcheck
	return id
}

// TestUpdateProgress_StaticLocationSaved verifies that submitting a negative
// location ID (a static town such as Pallet Town with id=-1) is persisted.
// Regression: the original guard `lid > 0` silently discarded negative IDs.
func TestUpdateProgress_StaticLocationSaved(t *testing.T) {
	db := bootstrapProgressDB(t)
	run := models.Run{ID: 1, VersionID: 10}
	router := newProgressRouter(db, run)

	w := postForm(t, router, "/runs/1/progress", map[string]string{
		"current_location_id": "-1", // Pallet Town (static, negative ID)
		"badge_count":         "0",
	})

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", w.Code, w.Body.String())
	}

	got := savedLocationID(t, db, 1)
	if got == nil || *got != -1 {
		t.Fatalf("expected current_location_id=-1, got %v", got)
	}
}

// TestUpdateProgress_PositiveLocationSaved checks the normal (PokeAPI) case
// still works after the fix.
func TestUpdateProgress_PositiveLocationSaved(t *testing.T) {
	db := bootstrapProgressDB(t)
	hMustExec(t, db, `INSERT INTO location VALUES (42, 'Route 1', 10, 'kanto')`)
	run := models.Run{ID: 1, VersionID: 10}
	router := newProgressRouter(db, run)

	w := postForm(t, router, "/runs/1/progress", map[string]string{
		"current_location_id": "42",
		"badge_count":         "1",
	})

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}

	got := savedLocationID(t, db, 1)
	if got == nil || *got != 42 {
		t.Fatalf("expected current_location_id=42, got %v", got)
	}
}

// TestUpdateProgress_ZeroLocationIgnored ensures that omitting a location
// (empty string → formInt → 0) leaves current_location_id as NULL.
func TestUpdateProgress_ZeroLocationIgnored(t *testing.T) {
	db := bootstrapProgressDB(t)
	run := models.Run{ID: 1, VersionID: 10}
	router := newProgressRouter(db, run)

	w := postForm(t, router, "/runs/1/progress", map[string]string{
		"current_location_id": "",
		"badge_count":         "0",
	})

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}

	got := savedLocationID(t, db, 1)
	if got != nil {
		t.Fatalf("expected current_location_id=NULL, got %v", *got)
	}
}

// ─── LogEncounter tests ───────────────────────────────────────────────────────

// bootstrapRoutesDB extends bootstrapProgressDB with the tables needed by
// LogEncounter: pokemon, run_pokemon.
func bootstrapRoutesDB(t *testing.T) *sql.DB {
	t.Helper()
	db := bootstrapProgressDB(t)
	hMustExec(t, db, `CREATE TABLE pokemon (
		id INTEGER PRIMARY KEY, species_name TEXT NOT NULL, form_name TEXT NOT NULL DEFAULT 'default',
		type1 TEXT, type2 TEXT,
		hp INTEGER, attack INTEGER, defense INTEGER,
		sp_attack INTEGER, sp_defense INTEGER, speed INTEGER,
		ability1 TEXT, ability2 TEXT)`)
	hMustExec(t, db, `CREATE TABLE run_pokemon (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER, form_id INTEGER, level INTEGER,
		caught_level INTEGER,
		acquisition_type TEXT NOT NULL DEFAULT 'manual'
			CHECK(acquisition_type IN ('starter','wild','gift','trade','manual')),
		met_location_id INTEGER, is_alive INTEGER DEFAULT 1,
		in_party INTEGER DEFAULT 0, party_slot INTEGER,
		moves_json TEXT DEFAULT '[]', held_item_id INTEGER
	)`)
	hMustExec(t, db, `INSERT INTO pokemon (id, species_name) VALUES (19, 'rattata')`)
	return db
}

func newRoutesRouter(db *sql.DB, run models.Run) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/runs/:run_id/routes",
		injectRunContext(run, models.RunProgress{RunID: run.ID}),
		LogEncounter(db, nil),
	)
	return r
}

// metLocationForLatestCatch returns the met_location_id of the most recently
// inserted run_pokemon row for the given run.
func metLocationForLatestCatch(t *testing.T, db *sql.DB, runID int) *int {
	t.Helper()
	var id *int
	db.QueryRow(`SELECT met_location_id FROM run_pokemon WHERE run_id = ? ORDER BY id DESC LIMIT 1`, runID).Scan(&id) //nolint:errcheck
	return id
}

// TestLogEncounter_StaticLocationSaved verifies a catch at a static town
// (negative location ID) records the correct met_location_id.
// Regression: `metLocPtr` was only set when locationID > 0.
func TestLogEncounter_StaticLocationSaved(t *testing.T) {
	db := bootstrapRoutesDB(t)
	run := models.Run{ID: 1, VersionID: 10}
	router := newRoutesRouter(db, run)

	w := postForm(t, router, "/runs/1/routes", map[string]string{
		"location_id": "-1", // Pallet Town
		"form_id":     "rattata",
		"outcome":     "caught",
		"level":       "5",
	})

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", w.Code, w.Body.String())
	}

	got := metLocationForLatestCatch(t, db, 1)
	if got == nil || *got != -1 {
		t.Fatalf("expected met_location_id=-1, got %v", got)
	}
}

// TestLogEncounter_PositiveLocationSaved checks the normal path still works.
func TestLogEncounter_PositiveLocationSaved(t *testing.T) {
	db := bootstrapRoutesDB(t)
	hMustExec(t, db, `INSERT INTO location VALUES (10, 'Route 1', 10, 'kanto')`)
	run := models.Run{ID: 1, VersionID: 10}
	router := newRoutesRouter(db, run)

	w := postForm(t, router, "/runs/1/routes", map[string]string{
		"location_id": "10",
		"form_id":     "rattata",
		"outcome":     "caught",
		"level":       "3",
	})

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}

	got := metLocationForLatestCatch(t, db, 1)
	if got == nil || *got != 10 {
		t.Fatalf("expected met_location_id=10, got %v", got)
	}
}

// TestLogEncounter_FledDoesNotRecord ensures a "fled" outcome adds no
// run_pokemon row regardless of location type.
func TestLogEncounter_FledDoesNotRecord(t *testing.T) {
	db := bootstrapRoutesDB(t)
	run := models.Run{ID: 1, VersionID: 10}
	router := newRoutesRouter(db, run)

	postForm(t, router, "/runs/1/routes", map[string]string{
		"location_id": "-1",
		"form_id":     "rattata",
		"outcome":     "fled",
		"level":       "5",
	})

	got := metLocationForLatestCatch(t, db, 1)
	if got != nil {
		t.Fatalf("expected no run_pokemon row for fled outcome, got met_location_id=%d", *got)
	}
}

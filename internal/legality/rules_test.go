package legality

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestDB opens an in-memory SQLite database and registers a cleanup hook.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ── LevelCap ────────────────────────────────────────────────────────────────

func TestLevelCap_RuleDisabled(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": false},
		RuleParams:  map[string]map[string]interface{}{},
	}
	cap, err := LevelCap(nil, rs) // DB must not be touched
	if err != nil || cap != 0 {
		t.Fatalf("expected (0, nil), got (%d, %v)", cap, err)
	}
}

func TestLevelCap_ParamsFloat64Override(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams: map[string]map[string]interface{}{
			"level_cap": {"cap": float64(45)},
		},
	}
	cap, err := LevelCap(nil, rs) // DB must not be touched
	if err != nil || cap != 45 {
		t.Fatalf("expected (45, nil), got (%d, %v)", cap, err)
	}
}

func TestLevelCap_ParamsIntOverride(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams: map[string]map[string]interface{}{
			"level_cap": {"cap": 50},
		},
	}
	cap, err := LevelCap(nil, rs) // DB must not be touched
	if err != nil || cap != 50 {
		t.Fatalf("expected (50, nil), got (%d, %v)", cap, err)
	}
}

func TestLevelCap_DBLookup(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO gen3_badge_cap VALUES (3, 35)`); err != nil {
		t.Fatal(err)
	}

	rs := &RunState{
		BadgeCount:  3,
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams:  map[string]map[string]interface{}{},
	}
	cap, err := LevelCap(db, rs)
	if err != nil || cap != 35 {
		t.Fatalf("expected (35, nil), got (%d, %v)", cap, err)
	}
}

func TestLevelCap_PostChampion(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(`CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	// No row for badge_count 9 — post-champion situation

	rs := &RunState{
		BadgeCount:  9,
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams:  map[string]map[string]interface{}{},
	}
	cap, err := LevelCap(db, rs)
	if err != nil || cap != 0 {
		t.Fatalf("expected (0, nil) for post-champion, got (%d, %v)", cap, err)
	}
}

// ── ApplyRules ───────────────────────────────────────────────────────────────

func TestApplyRules_NoRules(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{},
		RuleParams:  map[string]map[string]interface{}{},
	}
	acqs := []Acquisition{
		{FormID: 1, MinLevel: 5, MaxLevel: 10},
		{FormID: 2, MinLevel: 20, MaxLevel: 25},
	}
	got, err := ApplyRules(nil, rs, acqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range got {
		if a.BlockedByRule != nil {
			t.Errorf("form %d should not be blocked, got %q", a.FormID, *a.BlockedByRule)
		}
	}
}

func TestApplyRules_LevelCapAnnotates(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams: map[string]map[string]interface{}{
			"level_cap": {"cap": float64(25)},
		},
	}
	acqs := []Acquisition{
		{FormID: 1, MinLevel: 10, MaxLevel: 20}, // within cap — must stay clear
		{FormID: 2, MinLevel: 30, MaxLevel: 40}, // min_level > cap — must be annotated
	}
	got, err := ApplyRules(nil, rs, acqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].BlockedByRule != nil {
		t.Errorf("form 1 should not be blocked")
	}
	if got[1].BlockedByRule == nil || *got[1].BlockedByRule != "level_cap" {
		t.Errorf("form 2 should be blocked by level_cap, got %v", got[1].BlockedByRule)
	}
}

func TestApplyRules_LevelCapExactBoundary(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams: map[string]map[string]interface{}{
			"level_cap": {"cap": float64(30)},
		},
	}
	acqs := []Acquisition{
		{FormID: 1, MinLevel: 30, MaxLevel: 35}, // min_level == cap — NOT blocked (rule is MinLevel > cap)
		{FormID: 2, MinLevel: 31, MaxLevel: 40}, // min_level just over cap — blocked
	}
	got, err := ApplyRules(nil, rs, acqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].BlockedByRule != nil {
		t.Errorf("form 1 at exact cap should not be blocked")
	}
	if got[1].BlockedByRule == nil {
		t.Errorf("form 2 above cap should be blocked")
	}
}

func TestApplyRules_EmptyAcquisitions(t *testing.T) {
	rs := &RunState{
		ActiveRules: map[string]bool{"level_cap": true},
		RuleParams: map[string]map[string]interface{}{
			"level_cap": {"cap": float64(20)},
		},
	}
	got, err := ApplyRules(nil, rs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

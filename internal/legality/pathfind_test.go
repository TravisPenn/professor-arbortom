package legality

import (
	"database/sql"
	"encoding/json"
	"testing"
)

// setupPathfindDB creates a minimal in-memory DB with evolution data for testing.
func setupPathfindDB(t *testing.T) *sql.DB {
	t.Helper()
	db := newTestDB(t)
	db.SetMaxOpenConns(1)

	mustExecPathfind(t, db, `
		CREATE TABLE pokemon (
			id INTEGER PRIMARY KEY,
			species_name TEXT NOT NULL,
			form_name TEXT NOT NULL DEFAULT 'default',
			type1 TEXT NOT NULL DEFAULT 'normal',
			type2 TEXT,
			hp INTEGER NOT NULL DEFAULT 0,
			attack INTEGER NOT NULL DEFAULT 0,
			defense INTEGER NOT NULL DEFAULT 0,
			sp_attack INTEGER NOT NULL DEFAULT 0,
			sp_defense INTEGER NOT NULL DEFAULT 0,
			speed INTEGER NOT NULL DEFAULT 0,
			ability1 TEXT,
			ability2 TEXT
		);
		CREATE TABLE evolution_condition (
			id INTEGER PRIMARY KEY,
			from_form_id INTEGER,
			to_form_id INTEGER,
			trigger TEXT,
			conditions_json TEXT
		);
		CREATE TABLE learnset_entry (
			id INTEGER PRIMARY KEY,
			form_id INTEGER, version_group_id INTEGER, move_id INTEGER,
			learn_method TEXT, level_learned INTEGER
		);
		CREATE TABLE move (id INTEGER PRIMARY KEY, name TEXT UNIQUE, type_name TEXT, power INTEGER, accuracy INTEGER, pp INTEGER NOT NULL DEFAULT 0, damage_class TEXT, effect_entry TEXT);
	`)

	// Species: Charmander(4) -> Charmeleon(5) -> Charizard(6)
	mustExecPathfind(t, db, `INSERT INTO pokemon (id, species_name, form_name, type1) VALUES (4,'charmander','default','fire'),(5,'charmeleon','default','fire'),(6,'charizard','default','fire')`)
	condLevel16, _ := json.Marshal(map[string]interface{}{"min_level": 16})
	condLevel36, _ := json.Marshal(map[string]interface{}{"min_level": 36})
	mustExecPathfind(t, db, `INSERT INTO evolution_condition (from_form_id, to_form_id, trigger, conditions_json) VALUES
		(4, 5, 'level-up', ?), (5, 6, 'level-up', ?)`, string(condLevel16), string(condLevel36))

	// Eevee (133) branching: Vaporeon(134) via water-stone, Jolteon(135) via thunder-stone, Espeon(196) via friendship
	mustExecPathfind(t, db, `INSERT INTO pokemon (id, species_name, form_name, type1) VALUES (133,'eevee','default','normal'),(134,'vaporeon','default','water'),(135,'jolteon','default','electric'),(196,'espeon','default','psychic')`)
	condItem, _ := json.Marshal(map[string]interface{}{"item": "water-stone"})
	condItem2, _ := json.Marshal(map[string]interface{}{"item": "thunder-stone"})
	condFriend, _ := json.Marshal(map[string]interface{}{"friendship": true, "time_of_day": "day"})
	mustExecPathfind(t, db, `INSERT INTO evolution_condition (from_form_id, to_form_id, trigger, conditions_json) VALUES
		(133, 134, 'use-item', ?), (133, 135, 'use-item', ?), (133, 196, 'level-up', ?)`,
		string(condItem), string(condItem2), string(condFriend))

	// Kadabra(64) -> Alakazam(65) via trade
	mustExecPathfind(t, db, `INSERT INTO pokemon (id, species_name, form_name, type1) VALUES (64,'kadabra','default','psychic'),(65,'alakazam','default','psychic')`)
	mustExecPathfind(t, db, `INSERT INTO evolution_condition (from_form_id, to_form_id, trigger, conditions_json) VALUES
		(64, 65, 'trade', '{}')`)

	// Pikachu(25) -> Raichu(26) via thunder-stone; Pikachu learns Thunderbolt at level 26, Raichu never
	mustExecPathfind(t, db, `INSERT INTO pokemon (id, species_name, form_name, type1) VALUES (25,'pikachu','default','electric'),(26,'raichu','default','electric')`)
	condStone, _ := json.Marshal(map[string]interface{}{"item": "thunder-stone"})
	mustExecPathfind(t, db, `INSERT INTO evolution_condition (from_form_id, to_form_id, trigger, conditions_json) VALUES
		(25, 26, 'use-item', ?)`, string(condStone))
	mustExecPathfind(t, db, `INSERT INTO move VALUES (396,'thunderbolt','electric',90,100,15,NULL,NULL)`)
	mustExecPathfind(t, db, `INSERT INTO learnset_entry (form_id,version_group_id,move_id,learn_method,level_learned) VALUES
		(25, 7, 396, 'level-up', 26)`) // Pikachu learns Thunderbolt at Lv26
	// Raichu does NOT learn Thunderbolt via level-up

	return db
}

func mustExecPathfind(t *testing.T, db *sql.DB, q string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("mustExec %q: %v", q, err)
	}
}

func baseRunState() *RunState {
	return &RunState{
		ActiveRules: map[string]bool{},
		RuleParams:  map[string]map[string]interface{}{},
		Flags:       map[string]bool{},
	}
}

// ── LoadEvolutionGraph ────────────────────────────────────────────────────────

func TestLoadEvolutionGraph_LinearChain(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()

	g, err := LoadEvolutionGraph(db)
	if err != nil {
		t.Fatalf("LoadEvolutionGraph: %v", err)
	}

	// Charmander(4) should have one edge to Charmeleon(5)
	edges := g.edges[4]
	if len(edges) != 1 || edges[0].ToFormID != 5 {
		t.Errorf("expected Charmander->Charmeleon edge, got %v", edges)
	}
}

// ── FindEvolutionPaths ────────────────────────────────────────────────────────

func TestFindEvolutionPaths_LinearChain(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()

	paths := FindEvolutionPaths(g, 4, rs, 0) // Charmander, no cap
	// Expect two paths: Charmander->Charmeleon, Charmander->Charmeleon->Charizard
	if len(paths) < 2 {
		t.Fatalf("expected >= 2 paths from Charmander, got %d", len(paths))
	}
}

func TestFindEvolutionPaths_Branching_Eevee(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()

	paths := FindEvolutionPaths(g, 133, rs, 0) // Eevee
	// Should have path to Vaporeon, Jolteon, Espeon
	targets := make(map[int]bool)
	for _, p := range paths {
		if len(p.Steps) > 0 {
			last := p.Steps[len(p.Steps)-1]
			targets[last.ToFormID] = true
		}
	}
	for _, id := range []int{134, 135, 196} {
		if !targets[id] {
			t.Errorf("expected path to form %d in Eevee paths", id)
		}
	}
}

func TestFindEvolutionPaths_EspeonBlockedByFriendship(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState() // friendship flag NOT set

	paths := FindEvolutionPaths(g, 133, rs, 0)
	for _, p := range paths {
		if len(p.Steps) == 0 {
			continue
		}
		last := p.Steps[len(p.Steps)-1]
		if last.ToFormID == 196 { // Espeon
			if p.FullyLegal {
				t.Error("Espeon path should not be FullyLegal without friendship flag")
			}
			if p.BlockReason == "" {
				t.Error("Espeon path should have a BlockReason")
			}
		}
	}
}

func TestFindEvolutionPaths_EspeonUnblockedWithFriendship(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()
	rs.Flags["story.high_friendship"] = true

	paths := FindEvolutionPaths(g, 133, rs, 0)
	for _, p := range paths {
		if len(p.Steps) == 0 {
			continue
		}
		last := p.Steps[len(p.Steps)-1]
		if last.ToFormID == 196 {
			if !p.FullyLegal {
				t.Errorf("Espeon path should be FullyLegal with friendship set; BlockReason=%q", p.BlockReason)
			}
		}
	}
}

func TestFindEvolutionPaths_TradeBlockedByRule(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()
	rs.ActiveRules["no_trade_evolutions"] = true

	paths := FindEvolutionPaths(g, 64, rs, 0) // Kadabra
	if len(paths) == 0 {
		t.Fatal("expected some path from Kadabra")
	}
	for _, p := range paths {
		if !p.FullyLegal && p.BlockReason == "no_trade_evolutions" {
			return // found the expected blocked path
		}
	}
	t.Error("expected Alakazam path to be blocked by no_trade_evolutions")
}

func TestFindEvolutionPaths_TradeAllowedWithoutRule(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState() // rule not active

	paths := FindEvolutionPaths(g, 64, rs, 0)
	for _, p := range paths {
		if len(p.Steps) > 0 && p.Steps[len(p.Steps)-1].ToFormID == 65 {
			if !p.FullyLegal {
				t.Errorf("Alakazam path should be fully legal without rule; got BlockReason=%q", p.BlockReason)
			}
			return
		}
	}
	t.Error("expected to find Alakazam path")
}

func TestFindEvolutionPaths_NoInfiniteLoop(t *testing.T) {
	// Create a graph with no evolutions from self — just ensures BFS terminates.
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()

	paths := FindEvolutionPaths(g, 9999, rs, 0) // non-existent form
	if len(paths) != 0 {
		t.Error("expected empty paths for non-existent form")
	}
}

// ── MoveDelayAnalysis ─────────────────────────────────────────────────────────

func TestMoveDelayAnalysis_PikachuThunderbolt(t *testing.T) {
	db := setupPathfindDB(t)
	defer db.Close()
	g, _ := LoadEvolutionGraph(db)
	rs := baseRunState()

	paths := FindEvolutionPaths(g, 25, rs, 0) // Pikachu -> Raichu path
	if len(paths) == 0 {
		t.Fatal("expected Pikachu->Raichu path")
	}

	notes, err := MoveDelayAnalysis(db, 25, paths[0], 7)
	if err != nil {
		t.Fatalf("MoveDelayAnalysis: %v", err)
	}

	for _, note := range notes {
		if note.MoveName == "thunderbolt" {
			if note.PreEvoLevel != 26 {
				t.Errorf("Pikachu Thunderbolt PreEvoLevel = %d, want 26", note.PreEvoLevel)
			}
			if note.PostEvoLevel != 0 {
				t.Errorf("Raichu Thunderbolt PostEvoLevel = %d, want 0", note.PostEvoLevel)
			}
			if note.Recommendation != "delay" {
				t.Errorf("Thunderbolt recommendation = %q, want delay", note.Recommendation)
			}
			return
		}
	}
	t.Error("expected MoveDelayNote for thunderbolt")
}

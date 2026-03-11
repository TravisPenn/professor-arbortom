package legality

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// runCfg is the configuration passed to setupRunDB.
type runCfg struct {
	versionID      int
	versionGroupID int
	badgeCount     int
	locationID     *int // nil = no location set
}

// setupRunDB opens an in-memory DB, creates the tables needed by LoadRunState,
// inserts a run with the given config, and returns (db, runID=1).
func setupRunDB(t *testing.T, cfg runCfg) (*sql.DB, int) {
	t.Helper()
	db := newTestDB(t)
	db.SetMaxOpenConns(1) // in-memory SQLite: all ops must share one connection

	mustExec(t, db,
		`CREATE TABLE game_version (id INTEGER PRIMARY KEY, name TEXT, version_group_id INTEGER, generation_id INTEGER)`)
	mustExec(t, db,
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT UNIQUE)`)
	mustExec(t, db,
		`CREATE TABLE run (id INTEGER PRIMARY KEY, user_id INTEGER, version_id INTEGER, name TEXT)`)
	mustExec(t, db,
		`CREATE TABLE run_progress (run_id INTEGER PRIMARY KEY, badge_count INTEGER DEFAULT 0, current_location_id INTEGER)`)
	mustExec(t, db,
		`CREATE TABLE rule_def (id INTEGER PRIMARY KEY, key TEXT)`)
	mustExec(t, db,
		`CREATE TABLE run_rule (id INTEGER PRIMARY KEY, run_id INTEGER, rule_def_id INTEGER, enabled INTEGER DEFAULT 0, params_json TEXT DEFAULT '{}')`)
	mustExec(t, db,
		`CREATE TABLE run_flag (run_id INTEGER, key TEXT, value TEXT)`)

	mustExec(t, db, `INSERT INTO game_version VALUES (?, ?, ?, 3)`,
		cfg.versionID, "firered", cfg.versionGroupID)
	mustExec(t, db, `INSERT INTO user VALUES (1, 'tester')`)
	mustExec(t, db, `INSERT INTO run VALUES (1, 1, ?, 'test run')`, cfg.versionID)
	mustExec(t, db,
		`INSERT INTO run_progress (run_id, badge_count, current_location_id) VALUES (1, ?, ?)`,
		cfg.badgeCount, cfg.locationID)
	return db, 1
}

// mustExec runs a SQL statement and fails the test on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("mustExec %q: %v", query, err)
	}
}

// ── LegalAcquisitions ─────────────────────────────────────────────────────────

func TestLegalAcquisitions_WithLevelCap(t *testing.T) {
	locID := 10
	db, runID := setupRunDB(t, runCfg{
		versionID: 10, versionGroupID: 7, badgeCount: 2, locationID: &locID,
	})

	mustExec(t, db, `CREATE TABLE pokemon_species (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(t, db, `CREATE TABLE pokemon_form (id INTEGER PRIMARY KEY, species_id INTEGER, form_name TEXT)`)
	mustExec(t, db, `CREATE TABLE location (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, region TEXT)`)
	mustExec(t, db, `CREATE TABLE encounter (id INTEGER PRIMARY KEY, location_id INTEGER, form_id INTEGER,
		version_id INTEGER, min_level INTEGER, max_level INTEGER, method TEXT, conditions_json TEXT)`)
	mustExec(t, db, `CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER)`)

	mustExec(t, db, `INSERT INTO pokemon_species VALUES (1,'ekans'),(2,'rattata')`)
	mustExec(t, db, `INSERT INTO pokemon_form VALUES (101,1,'default'),(102,2,'default')`)
	mustExec(t, db, `INSERT INTO location VALUES (10,'Route 1',10,'kanto')`)
	// ekans: min_level 35 — above cap; rattata: min_level 2 — within cap
	mustExec(t, db, `INSERT INTO encounter VALUES (1,10,101,10,35,40,'walk','[]')`)
	mustExec(t, db, `INSERT INTO encounter VALUES (2,10,102,10,2,4,'walk','[]')`)
	mustExec(t, db, `INSERT INTO rule_def VALUES (1,'level_cap')`)
	mustExec(t, db, `INSERT INTO run_rule VALUES (1,1,1,1,'{}')`) // level_cap enabled
	mustExec(t, db, `INSERT INTO gen3_badge_cap VALUES (2,25)`)   // badge 2 → cap 25

	acqs, _, err := LegalAcquisitions(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(acqs) != 2 {
		t.Fatalf("expected 2 acquisitions, got %d", len(acqs))
	}

	// Query orders by ps.name ASC: ekans (e) before rattata (r)
	if acqs[0].SpeciesName != "ekans" || acqs[0].BlockedByRule == nil || *acqs[0].BlockedByRule != "level_cap" {
		t.Errorf("ekans (min_level 35) should be blocked by level_cap; got blocked=%v", acqs[0].BlockedByRule)
	}
	if acqs[1].SpeciesName != "rattata" || acqs[1].BlockedByRule != nil {
		t.Errorf("rattata (min_level 2) should not be blocked; got blocked=%v", acqs[1].BlockedByRule)
	}
}

func TestLegalAcquisitions_NoLocation(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7}) // no locationID

	mustExec(t, db, `CREATE TABLE pokemon_species (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(t, db, `CREATE TABLE pokemon_form (id INTEGER PRIMARY KEY, species_id INTEGER, form_name TEXT)`)
	mustExec(t, db, `CREATE TABLE location (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, region TEXT)`)
	mustExec(t, db, `CREATE TABLE encounter (id INTEGER PRIMARY KEY, location_id INTEGER, form_id INTEGER,
		version_id INTEGER, min_level INTEGER, max_level INTEGER, method TEXT, conditions_json TEXT)`)

	acqs, warns, err := LegalAcquisitions(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(acqs) != 0 {
		t.Errorf("expected empty acquisitions with no location, got %d", len(acqs))
	}
	if len(warns) == 0 || warns[0].Code != "no_location" {
		t.Errorf("expected no_location warning, got %v", warns)
	}
}

// ── LegalMoves ────────────────────────────────────────────────────────────────

func TestLegalMoves_HMBlocked(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7})

	mustExec(t, db, `CREATE TABLE move (id INTEGER PRIMARY KEY, name TEXT, type_name TEXT)`)
	mustExec(t, db, `CREATE TABLE learnset_entry (
		form_id INTEGER, move_id INTEGER, version_group_id INTEGER, learn_method TEXT, level_learned INTEGER)`)

	mustExec(t, db, `INSERT INTO move VALUES (1,'tackle','normal'),(2,'surf','water')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'level-up',1)`) // tackle
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,2,7,'machine',0)`)  // surf

	// hm.surf_obtained flag NOT set → surf should be blocked
	moves, warns, err := LegalMoves(db, runID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]*string)
	for _, mv := range moves {
		byName[mv.Name] = mv.BlockedByRule
	}
	if r := byName["tackle"]; r != nil {
		t.Errorf("tackle should not be blocked, got %q", *r)
	}
	if r, ok := byName["surf"]; !ok {
		t.Fatal("surf missing from results")
	} else if r == nil {
		t.Error("surf should be blocked when hm.surf_obtained is not set")
	} else if *r != "hm_flag_missing" {
		t.Errorf("surf BlockedByRule = %q, want hm_flag_missing", *r)
	}

	hasHMWarn := false
	for _, w := range warns {
		if w.Code == "hm_flag" {
			hasHMWarn = true
		}
	}
	if !hasHMWarn {
		t.Error("expected hm_flag warning for surf")
	}
}

func TestLegalMoves_LevelCapBlocksHighLevel(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, badgeCount: 1})

	mustExec(t, db, `CREATE TABLE move (id INTEGER PRIMARY KEY, name TEXT, type_name TEXT)`)
	mustExec(t, db, `CREATE TABLE learnset_entry (
		form_id INTEGER, move_id INTEGER, version_group_id INTEGER, learn_method TEXT, level_learned INTEGER)`)
	mustExec(t, db, `CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER)`)
	mustExec(t, db, `INSERT INTO rule_def VALUES (1,'level_cap')`)
	mustExec(t, db, `INSERT INTO run_rule VALUES (1,1,1,1,'{}')`)
	mustExec(t, db, `INSERT INTO gen3_badge_cap VALUES (1,20)`)

	mustExec(t, db, `INSERT INTO move VALUES (1,'tackle','normal'),(2,'hyper-beam','normal')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'level-up',1)`)  // level 1 - within cap
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,2,7,'level-up',40)`) // level 40 - above cap 20

	moves, _, err := LegalMoves(db, runID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]*string)
	for _, mv := range moves {
		byName[mv.Name] = mv.BlockedByRule
	}
	if r := byName["tackle"]; r != nil {
		t.Errorf("tackle should not be blocked")
	}
	if r, ok := byName["hyper-beam"]; !ok {
		t.Fatal("hyper-beam missing")
	} else if r == nil || *r != "level_cap" {
		t.Errorf("hyper-beam should be blocked by level_cap, got %v", r)
	}
}

// ── LegalItems ────────────────────────────────────────────────────────────────

func TestLegalItems_OwnedAndObtainable(t *testing.T) {
	locID := 20
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, locationID: &locID})

	mustExec(t, db, `CREATE TABLE item (id INTEGER PRIMARY KEY, name TEXT, category TEXT)`)
	mustExec(t, db, `CREATE TABLE run_item (run_id INTEGER, item_id INTEGER, qty INTEGER)`)
	mustExec(t, db, `CREATE TABLE item_availability (location_id INTEGER, item_id INTEGER, version_id INTEGER)`)

	mustExec(t, db, `INSERT INTO item VALUES (1,'poke-ball','pokeball'),(2,'potion','healing')`)
	mustExec(t, db, `INSERT INTO run_item VALUES (?,1,5)`, runID)     // owns 5 poke-balls
	mustExec(t, db, `INSERT INTO item_availability VALUES (20,2,10)`) // potion buyable here

	items, err := LegalItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (1 owned + 1 obtainable), got %d", len(items))
	}

	sources := make(map[string]string)
	for _, it := range items {
		sources[it.Name] = it.Source
	}
	if sources["poke-ball"] != "owned" {
		t.Errorf("poke-ball should be source=owned, got %q", sources["poke-ball"])
	}
	if sources["potion"] != "obtainable" {
		t.Errorf("potion should be source=obtainable, got %q", sources["potion"])
	}
}

func TestLegalItems_OwnedNotDuplicatedAsObtainable(t *testing.T) {
	locID := 20
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, locationID: &locID})

	mustExec(t, db, `CREATE TABLE item (id INTEGER PRIMARY KEY, name TEXT, category TEXT)`)
	mustExec(t, db, `CREATE TABLE run_item (run_id INTEGER, item_id INTEGER, qty INTEGER)`)
	mustExec(t, db, `CREATE TABLE item_availability (location_id INTEGER, item_id INTEGER, version_id INTEGER)`)

	mustExec(t, db, `INSERT INTO item VALUES (1,'poke-ball','pokeball')`)
	mustExec(t, db, `INSERT INTO run_item VALUES (?,1,3)`, runID)     // already owns it
	mustExec(t, db, `INSERT INTO item_availability VALUES (20,1,10)`) // also buyable

	items, err := LegalItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("owned item should not appear twice; got %d items", len(items))
	}
	if items[0].Source != "owned" {
		t.Errorf("expected source=owned, got %q", items[0].Source)
	}
}

// ── EvolutionOptions ──────────────────────────────────────────────────────────

func TestEvolutionOptions_TradeBlocked(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7})

	mustExec(t, db, `CREATE TABLE pokemon_species (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(t, db, `CREATE TABLE pokemon_form (id INTEGER PRIMARY KEY, species_id INTEGER, form_name TEXT)`)
	mustExec(t, db, `CREATE TABLE evolution_condition (
		id INTEGER PRIMARY KEY, from_form_id INTEGER, to_form_id INTEGER, trigger TEXT, conditions_json TEXT)`)
	mustExec(t, db, `CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER)`)

	mustExec(t, db, `INSERT INTO pokemon_species VALUES (1,'haunter'),(2,'gengar')`)
	mustExec(t, db, `INSERT INTO pokemon_form VALUES (10,1,'default'),(11,2,'default')`)
	mustExec(t, db, `INSERT INTO evolution_condition VALUES (1,10,11,'trade','{}')`)
	mustExec(t, db, `INSERT INTO rule_def VALUES (1,'no_trade_evolutions')`)
	mustExec(t, db, `INSERT INTO run_rule VALUES (1,1,1,1,'{}')`) // enabled

	evos, err := EvolutionOptions(db, runID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evos) != 1 {
		t.Fatalf("expected 1 evolution, got %d", len(evos))
	}
	if evos[0].BlockedByRule == nil || *evos[0].BlockedByRule != "no_trade_evolutions" {
		t.Errorf("haunter→gengar should be blocked by no_trade_evolutions, got %v", evos[0].BlockedByRule)
	}
	if evos[0].CurrentlyPossible {
		t.Error("CurrentlyPossible should be false when rule is active")
	}
}

func TestEvolutionOptions_LevelUpPossible(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, badgeCount: 5})

	mustExec(t, db, `CREATE TABLE pokemon_species (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(t, db, `CREATE TABLE pokemon_form (id INTEGER PRIMARY KEY, species_id INTEGER, form_name TEXT)`)
	mustExec(t, db, `CREATE TABLE evolution_condition (
		id INTEGER PRIMARY KEY, from_form_id INTEGER, to_form_id INTEGER, trigger TEXT, conditions_json TEXT)`)
	mustExec(t, db, `CREATE TABLE gen3_badge_cap (badge_count INTEGER PRIMARY KEY, level_cap INTEGER)`)
	mustExec(t, db, `INSERT INTO gen3_badge_cap VALUES (5,45)`)
	mustExec(t, db, `INSERT INTO rule_def VALUES (1,'level_cap')`)
	mustExec(t, db, `INSERT INTO run_rule VALUES (1,1,1,1,'{}')`) // level_cap enabled

	mustExec(t, db, `INSERT INTO pokemon_species VALUES (1,'magikarp'),(2,'gyarados')`)
	mustExec(t, db, `INSERT INTO pokemon_form VALUES (10,1,'default'),(11,2,'default')`)
	// Evolves at level 20, which is within cap 45
	mustExec(t, db, `INSERT INTO evolution_condition VALUES (1,10,11,'level-up','{"min_level":20}')`)

	evos, err := EvolutionOptions(db, runID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evos) != 1 {
		t.Fatalf("expected 1 evolution, got %d", len(evos))
	}
	if !evos[0].CurrentlyPossible {
		t.Error("magikarp→gyarados should be currently possible (level 20 <= cap 45)")
	}
	if evos[0].BlockedByRule != nil {
		t.Errorf("should not be blocked, got %q", *evos[0].BlockedByRule)
	}
}

// ── LegalTrades ───────────────────────────────────────────────────────────────

func setupTradesDB(t *testing.T, locID int) (*sql.DB, int) {
	t.Helper()
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, locationID: &locID})
	mustExec(t, db, `CREATE TABLE location (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, region TEXT)`)
	mustExec(t, db, `CREATE TABLE in_game_trade (
		id INTEGER PRIMARY KEY, location_id INTEGER,
		give_species TEXT, receive_species TEXT, receive_nick TEXT,
		price_coins INTEGER, notes TEXT)`)
	mustExec(t, db, `INSERT INTO location VALUES (?, 'Cerulean City', 10, 'kanto')`, locID)
	return db, runID
}

func TestLegalTrades_NpcTrade(t *testing.T) {
	db, runID := setupTradesDB(t, 20)
	mustExec(t, db, `INSERT INTO in_game_trade VALUES (1,20,'poliwhirl','jynx','Lola',NULL,NULL)`)

	trades, err := LegalTrades(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	tr := trades[0]
	if tr.Method != "trade" {
		t.Errorf("Method = %q, want trade", tr.Method)
	}
	if tr.GiveSpecies != "poliwhirl" {
		t.Errorf("GiveSpecies = %q, want poliwhirl", tr.GiveSpecies)
	}
	if tr.ReceiveSpecies != "jynx" {
		t.Errorf("ReceiveSpecies = %q, want jynx", tr.ReceiveSpecies)
	}
	if tr.ReceiveNick != "Lola" {
		t.Errorf("ReceiveNick = %q, want Lola", tr.ReceiveNick)
	}
}

func TestLegalTrades_GameCorner(t *testing.T) {
	db, runID := setupTradesDB(t, 20)
	// give_species IS NULL → game-corner entry
	mustExec(t, db, `INSERT INTO in_game_trade VALUES (1,20,NULL,'dratini',NULL,2800,NULL)`)

	trades, err := LegalTrades(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	tr := trades[0]
	if tr.Method != "game-corner" {
		t.Errorf("Method = %q, want game-corner", tr.Method)
	}
	if tr.GiveSpecies != "" {
		t.Errorf("GiveSpecies should be empty for game-corner, got %q", tr.GiveSpecies)
	}
	if tr.PriceCoins != 2800 {
		t.Errorf("PriceCoins = %d, want 2800", tr.PriceCoins)
	}
}

func TestLegalTrades_NoLocation(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7}) // nil locationID
	mustExec(t, db, `CREATE TABLE location (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, region TEXT)`)
	mustExec(t, db, `CREATE TABLE in_game_trade (
		id INTEGER PRIMARY KEY, location_id INTEGER,
		give_species TEXT, receive_species TEXT, receive_nick TEXT,
		price_coins INTEGER, notes TEXT)`)

	trades, err := LegalTrades(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("expected no trades with no location set, got %d", len(trades))
	}
}

func TestLegalTrades_WrongLocation(t *testing.T) {
	db, runID := setupTradesDB(t, 20) // current location = 20
	// Trade exists at a different location (99) — should not appear
	mustExec(t, db, `INSERT INTO location VALUES (99, 'Lavender Town', 10, 'kanto')`)
	mustExec(t, db, `INSERT INTO in_game_trade VALUES (1,99,'cubone','haunter',NULL,NULL,NULL)`)

	trades, err := LegalTrades(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("expected no trades at location 20, got %d", len(trades))
	}
}

// ── ShopItems ─────────────────────────────────────────────────────────────────

func setupShopDB(t *testing.T, locID int) (*sql.DB, int) {
	t.Helper()
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, locationID: &locID})
	mustExec(t, db, `CREATE TABLE item (id INTEGER PRIMARY KEY, name TEXT, category TEXT)`)
	mustExec(t, db, `CREATE TABLE shop_item (
		id INTEGER PRIMARY KEY, location_id INTEGER, version_id INTEGER,
		item_name TEXT, price INTEGER, currency TEXT DEFAULT 'pokedollar',
		UNIQUE(location_id, version_id, item_name))`)
	mustExec(t, db, `CREATE TABLE tm_move (tm_number INTEGER PRIMARY KEY, move_name TEXT NOT NULL)`)
	return db, runID
}

func TestShopItems_ReturnsShopSource(t *testing.T) {
	db, runID := setupShopDB(t, 20)
	mustExec(t, db, `INSERT INTO shop_item VALUES (1,20,10,'potion',300,'pokedollar')`)

	items, err := ShopItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 shop item, got %d", len(items))
	}
	it := items[0]
	if it.Source != "shop" {
		t.Errorf("Source = %q, want shop", it.Source)
	}
	if it.Name != "potion" {
		t.Errorf("Name = %q, want potion", it.Name)
	}
	if it.Price != 300 {
		t.Errorf("Price = %d, want 300", it.Price)
	}
	if it.Currency != "pokedollar" {
		t.Errorf("Currency = %q, want pokedollar", it.Currency)
	}
}

func TestShopItems_JoinsItemTable(t *testing.T) {
	db, runID := setupShopDB(t, 20)
	mustExec(t, db, `INSERT INTO item VALUES (7,'great-ball','pokeball')`)
	mustExec(t, db, `INSERT INTO shop_item VALUES (1,20,10,'great-ball',600,'pokedollar')`)

	items, err := ShopItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ItemID != 7 {
		t.Errorf("ItemID = %d, want 7 (from item table JOIN)", items[0].ItemID)
	}
	if items[0].Category != "pokeball" {
		t.Errorf("Category = %q, want pokeball", items[0].Category)
	}
}

func TestShopItems_NoLocation(t *testing.T) {
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7}) // nil locationID
	mustExec(t, db, `CREATE TABLE item (id INTEGER PRIMARY KEY, name TEXT, category TEXT)`)
	mustExec(t, db, `CREATE TABLE shop_item (
		id INTEGER PRIMARY KEY, location_id INTEGER, version_id INTEGER,
		item_name TEXT, price INTEGER, currency TEXT DEFAULT 'pokedollar',
		UNIQUE(location_id, version_id, item_name))`)
	mustExec(t, db, `CREATE TABLE tm_move (tm_number INTEGER PRIMARY KEY, move_name TEXT NOT NULL)`)

	items, err := ShopItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no shop items with no location, got %d", len(items))
	}
}

func TestShopItems_WrongLocation(t *testing.T) {
	db, runID := setupShopDB(t, 20)                                                     // current = 20
	mustExec(t, db, `INSERT INTO shop_item VALUES (1,99,10,'potion',300,'pokedollar')`) // location 99

	items, err := ShopItems(db, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no items at location 20, got %d", len(items))
	}
}

// ── CoachMoves ────────────────────────────────────────────────────────────────

func setupCoachMovesDB(t *testing.T) (*sql.DB, int) {
	t.Helper()
	db, runID := setupRunDB(t, runCfg{versionID: 10, versionGroupID: 7, badgeCount: 0})
	mustExec(t, db, `CREATE TABLE move (id INTEGER PRIMARY KEY, name TEXT, type_name TEXT)`)
	mustExec(t, db, `CREATE TABLE learnset_entry (
		form_id INTEGER, move_id INTEGER, version_group_id INTEGER, learn_method TEXT, level_learned INTEGER)`)
	mustExec(t, db, `CREATE TABLE pokemon_species (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(t, db, `CREATE TABLE pokemon_form (id INTEGER PRIMARY KEY, species_id INTEGER, form_name TEXT)`)
	mustExec(t, db, `CREATE TABLE evolution_condition (
		id INTEGER PRIMARY KEY, from_form_id INTEGER, to_form_id INTEGER, trigger TEXT, conditions_json TEXT)`)
	mustExec(t, db, `CREATE TABLE tm_move (tm_number INTEGER PRIMARY KEY, move_name TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE hm_move (hm_number INTEGER PRIMARY KEY, move_name TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE tutor_move (
		id INTEGER PRIMARY KEY, version_group_id INTEGER NOT NULL,
		move_name TEXT NOT NULL, location_name TEXT NOT NULL,
		UNIQUE(version_group_id, move_name))`)
	return db, runID
}

func TestCoachMoves_ExcludesEggMoves(t *testing.T) {
	db, runID := setupCoachMovesDB(t)
	mustExec(t, db, `INSERT INTO move VALUES (1,'tackle','normal'),(2,'dragon-rage','dragon')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'level-up',5)`) // kept
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,2,7,'egg',0)`)      // filtered

	moves, err := CoachMoves(db, runID, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, mv := range moves {
		if mv.LearnMethod == "egg" {
			t.Errorf("CoachMoves should not include egg moves, found %q", mv.Name)
		}
	}
	if len(moves) != 1 || moves[0].Name != "tackle" {
		t.Errorf("expected only tackle, got %v", moves)
	}
}

func TestCoachMoves_FiltersAlreadyLearnableLevelUp(t *testing.T) {
	db, runID := setupCoachMovesDB(t)
	mustExec(t, db, `INSERT INTO move VALUES (1,'tackle','normal'),(2,'growl','normal'),(3,'vine-whip','grass')`)
	// currentLevel = 15; Lv1 and Lv10 should be filtered, Lv16 should appear
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'level-up',1)`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,2,7,'level-up',10)`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,3,7,'level-up',16)`)

	moves, err := CoachMoves(db, runID, 5, 15)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("expected 1 move (vine-whip at lv16), got %d: %v", len(moves), moves)
	}
	if moves[0].Name != "vine-whip" {
		t.Errorf("expected vine-whip, got %q", moves[0].Name)
	}
}

func TestCoachMoves_ZeroCurrentLevelSkipsFilter(t *testing.T) {
	db, runID := setupCoachMovesDB(t)
	mustExec(t, db, `INSERT INTO move VALUES (1,'tackle','normal'),(2,'growl','normal')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'level-up',1)`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,2,7,'level-up',3)`)

	// currentLevel=0 means "don't filter" — both moves should appear
	moves, err := CoachMoves(db, runID, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moves) != 2 {
		t.Errorf("expected 2 moves with currentLevel=0, got %d", len(moves))
	}
}

func TestCoachMoves_EvoNoteAnnotated(t *testing.T) {
	db, runID := setupCoachMovesDB(t)

	// bulbasaur (form 1) learns razor-leaf at lv20
	// its evolution ivysaur (form 2) learns razor-leaf at lv15 (sooner)
	mustExec(t, db, `INSERT INTO pokemon_species VALUES (1,'bulbasaur'),(2,'ivysaur')`)
	mustExec(t, db, `INSERT INTO pokemon_form VALUES (1,1,'default'),(2,2,'default')`)
	mustExec(t, db, `INSERT INTO evolution_condition VALUES (1,1,2,'level-up','{"min_level":16}')`)
	mustExec(t, db, `INSERT INTO move VALUES (1,'razor-leaf','grass')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (1,1,7,'level-up',20)`) // bulbasaur lv20
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (2,1,7,'level-up',15)`) // ivysaur lv15 (sooner ↑)

	// currentLevel = 5 so lv20 is not yet filtered
	moves, err := CoachMoves(db, runID, 1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].EvoNote == "" {
		t.Error("expected EvoNote to be set (ivysaur learns razor-leaf sooner)")
	}
}

func TestCoachMoves_HMNumberAnnotated(t *testing.T) {
	db, runID := setupCoachMovesDB(t)
	mustExec(t, db, `INSERT INTO move VALUES (1,'surf','water')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'machine',0)`)
	mustExec(t, db, `INSERT INTO hm_move VALUES (3,'surf')`)

	moves, err := CoachMoves(db, runID, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].HMNumber != 3 {
		t.Errorf("HMNumber = %d, want 3 (HM03 Surf)", moves[0].HMNumber)
	}
	if moves[0].TMNumber != 0 {
		t.Errorf("TMNumber should be 0 for an HM move, got %d", moves[0].TMNumber)
	}
}

func TestCoachMoves_TutorLocationAnnotated(t *testing.T) {
	db, runID := setupCoachMovesDB(t)
	mustExec(t, db, `INSERT INTO move VALUES (1,'dream-eater','psychic')`)
	mustExec(t, db, `INSERT INTO learnset_entry VALUES (5,1,7,'tutor',0)`)
	mustExec(t, db, `INSERT INTO tutor_move VALUES (1,7,'dream-eater','Viridian City')`)

	moves, err := CoachMoves(db, runID, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].TutorLocation != "Viridian City" {
		t.Errorf("TutorLocation = %q, want \"Viridian City\"", moves[0].TutorLocation)
	}
}

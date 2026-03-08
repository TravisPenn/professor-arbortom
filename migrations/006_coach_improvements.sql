-- Migration 006: Coach improvements — in-game trades, Game Corner Pokémon, and shop items.
-- Tables use TEXT species/item names (no FK) for resilience to partial PokeAPI hydration.

PRAGMA foreign_keys = OFF;

-- ── in_game_trade ──────────────────────────────────────────────────────────
-- Pokémon obtainable via NPC trade or Game Corner at a static location.
CREATE TABLE IF NOT EXISTS in_game_trade (
    id              INTEGER PRIMARY KEY,
    location_id     INTEGER NOT NULL,
    give_species    TEXT,           -- NULL = Game Corner entry (coins, not a traded Pokémon)
    receive_species TEXT NOT NULL,
    receive_nick    TEXT,
    price_coins     INTEGER,        -- coins cost for Game Corner; NULL for standard trades
    notes           TEXT
);

-- ── shop_item ──────────────────────────────────────────────────────────────
-- Items sold in Pokémon Mart or Department Store at a specific static location.
-- item_name is TEXT (no FK to item) so data survives before PokeAPI hydration;
-- the handler joins to the item table by name when available.
CREATE TABLE IF NOT EXISTS shop_item (
    id          INTEGER PRIMARY KEY,
    location_id INTEGER NOT NULL,
    version_id  INTEGER NOT NULL,
    item_name   TEXT NOT NULL,
    price       INTEGER NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'pokedollar',
    UNIQUE(location_id, version_id, item_name)
);

-- ── FireRed / LeafGreen: NPC Trades ───────────────────────────────────────

-- Cerulean City: trade Poliwhirl → Jynx
--   FireRed: location -4,  LeafGreen: location -21
INSERT OR IGNORE INTO in_game_trade (location_id, give_species, receive_species)
VALUES (-4,  'poliwhirl', 'jynx'),
       (-21, 'poliwhirl', 'jynx');

-- Lavender Town: trade Cubone → Haunter (holds Everstone)
--   FireRed: location -6,  LeafGreen: location -23
INSERT OR IGNORE INTO in_game_trade (location_id, give_species, receive_species, notes)
VALUES (-6,  'cubone', 'haunter', 'Holds Everstone — evolve via trade from here for Gengar'),
       (-23, 'cubone', 'haunter', 'Holds Everstone — evolve via trade from here for Gengar');

-- ── FireRed: Celadon City Game Corner ─────────────────────────────────────
--   location -7 = Celadon City (FireRed v10)
INSERT OR IGNORE INTO in_game_trade (location_id, give_species, receive_species, price_coins)
VALUES (-7, NULL, 'abra',     1000),
       (-7, NULL, 'clefairy',  500),
       (-7, NULL, 'scyther',  5500),
       (-7, NULL, 'dratini',  2800),
       (-7, NULL, 'porygon',  9999);

-- ── LeafGreen: Celadon City Game Corner ───────────────────────────────────
--   location -24 = Celadon City (LeafGreen v11)
INSERT OR IGNORE INTO in_game_trade (location_id, give_species, receive_species, price_coins)
VALUES (-24, NULL, 'abra',     1000),
       (-24, NULL, 'clefairy',  500),
       (-24, NULL, 'pinsir',   2500),
       (-24, NULL, 'dratini',  2800),
       (-24, NULL, 'porygon',  9999);

-- ── FireRed: Viridian City Poké Mart ──────────────────────────────────────
--   location -2 = Viridian City (FireRed v10)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-2, 10, 'poke-ball',    200),
    (-2, 10, 'potion',       300),
    (-2, 10, 'antidote',     100),
    (-2, 10, 'parlyz-heal',  200),
    (-2, 10, 'escape-rope',  550),
    (-2, 10, 'repel',        350);

-- ── LeafGreen: Viridian City Poké Mart ────────────────────────────────────
--   location -19 = Viridian City (LeafGreen v11)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-19, 11, 'poke-ball',    200),
    (-19, 11, 'potion',       300),
    (-19, 11, 'antidote',     100),
    (-19, 11, 'parlyz-heal',  200),
    (-19, 11, 'escape-rope',  550),
    (-19, 11, 'repel',        350);

-- ── FireRed: Cerulean City Poké Mart ──────────────────────────────────────
--   location -4 = Cerulean City (FireRed v10)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-4, 10, 'great-ball',    600),
    (-4, 10, 'super-potion',  700),
    (-4, 10, 'ice-heal',      250),
    (-4, 10, 'awakening',     250),
    (-4, 10, 'antidote',      100),
    (-4, 10, 'parlyz-heal',   200),
    (-4, 10, 'repel',         350);

-- ── LeafGreen: Cerulean City Poké Mart ────────────────────────────────────
--   location -21 = Cerulean City (LeafGreen v11)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-21, 11, 'great-ball',    600),
    (-21, 11, 'super-potion',  700),
    (-21, 11, 'ice-heal',      250),
    (-21, 11, 'awakening',     250),
    (-21, 11, 'antidote',      100),
    (-21, 11, 'parlyz-heal',   200),
    (-21, 11, 'repel',         350);

-- ── FireRed: Celadon City Dept Store (Medicine + Balls) ───────────────────
--   location -7 = Celadon City (FireRed v10)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-7, 10, 'ultra-ball',    1200),
    (-7, 10, 'great-ball',     600),
    (-7, 10, 'poke-ball',      200),
    (-7, 10, 'full-restore',  3000),
    (-7, 10, 'max-potion',    2500),
    (-7, 10, 'hyper-potion',  1200),
    (-7, 10, 'super-potion',   700),
    (-7, 10, 'potion',         300),
    (-7, 10, 'revive',        1500),
    (-7, 10, 'full-heal',      600),
    (-7, 10, 'ice-heal',       250),
    (-7, 10, 'burn-heal',      250),
    (-7, 10, 'awakening',      250),
    (-7, 10, 'antidote',       100),
    (-7, 10, 'parlyz-heal',    200),
    (-7, 10, 'max-repel',      700),
    (-7, 10, 'super-repel',    500),
    (-7, 10, 'repel',          350),
    (-7, 10, 'escape-rope',    550);

-- ── LeafGreen: Celadon City Dept Store ────────────────────────────────────
--   location -24 = Celadon City (LeafGreen v11)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-24, 11, 'ultra-ball',    1200),
    (-24, 11, 'great-ball',     600),
    (-24, 11, 'poke-ball',      200),
    (-24, 11, 'full-restore',  3000),
    (-24, 11, 'max-potion',    2500),
    (-24, 11, 'hyper-potion',  1200),
    (-24, 11, 'super-potion',   700),
    (-24, 11, 'potion',         300),
    (-24, 11, 'revive',        1500),
    (-24, 11, 'full-heal',      600),
    (-24, 11, 'ice-heal',       250),
    (-24, 11, 'burn-heal',      250),
    (-24, 11, 'awakening',      250),
    (-24, 11, 'antidote',       100),
    (-24, 11, 'parlyz-heal',    200),
    (-24, 11, 'max-repel',      700),
    (-24, 11, 'super-repel',    500),
    (-24, 11, 'repel',          350),
    (-24, 11, 'escape-rope',    550);

-- ── Ruby: Rustboro City Poké Mart ─────────────────────────────────────────
--   location -38 = Rustboro City (Ruby v6)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-38, 6, 'great-ball',    600),
    (-38, 6, 'super-potion',  700),
    (-38, 6, 'ice-heal',      250),
    (-38, 6, 'awakening',     250),
    (-38, 6, 'antidote',      100),
    (-38, 6, 'parlyz-heal',   200),
    (-38, 6, 'escape-rope',   550),
    (-38, 6, 'repel',         350);

-- ── Sapphire: Rustboro City Poké Mart ─────────────────────────────────────
--   location -54 = Rustboro City (Sapphire v7)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-54, 7, 'great-ball',    600),
    (-54, 7, 'super-potion',  700),
    (-54, 7, 'ice-heal',      250),
    (-54, 7, 'awakening',     250),
    (-54, 7, 'antidote',      100),
    (-54, 7, 'parlyz-heal',   200),
    (-54, 7, 'escape-rope',   550),
    (-54, 7, 'repel',         350);

-- ── Emerald: Rustboro City Poké Mart ──────────────────────────────────────
--   location -70 = Rustboro City (Emerald v8)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-70, 8, 'great-ball',    600),
    (-70, 8, 'super-potion',  700),
    (-70, 8, 'ice-heal',      250),
    (-70, 8, 'awakening',     250),
    (-70, 8, 'antidote',      100),
    (-70, 8, 'parlyz-heal',   200),
    (-70, 8, 'escape-rope',   550),
    (-70, 8, 'repel',         350);

-- ── Ruby: Mauville City Poké Mart ─────────────────────────────────────────
--   location -41 = Mauville City (Ruby v6)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-41, 6, 'ultra-ball',    1200),
    (-41, 6, 'great-ball',     600),
    (-41, 6, 'hyper-potion',  1200),
    (-41, 6, 'super-potion',   700),
    (-41, 6, 'revive',        1500),
    (-41, 6, 'ice-heal',       250),
    (-41, 6, 'burn-heal',      250),
    (-41, 6, 'awakening',      250),
    (-41, 6, 'antidote',       100),
    (-41, 6, 'parlyz-heal',    200),
    (-41, 6, 'max-repel',      700),
    (-41, 6, 'super-repel',    500),
    (-41, 6, 'repel',          350),
    (-41, 6, 'escape-rope',    550);

-- ── Sapphire: Mauville City Poké Mart ─────────────────────────────────────
--   location -57 = Mauville City (Sapphire v7)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-57, 7, 'ultra-ball',    1200),
    (-57, 7, 'great-ball',     600),
    (-57, 7, 'hyper-potion',  1200),
    (-57, 7, 'super-potion',   700),
    (-57, 7, 'revive',        1500),
    (-57, 7, 'ice-heal',       250),
    (-57, 7, 'burn-heal',      250),
    (-57, 7, 'awakening',      250),
    (-57, 7, 'antidote',       100),
    (-57, 7, 'parlyz-heal',    200),
    (-57, 7, 'max-repel',      700),
    (-57, 7, 'super-repel',    500),
    (-57, 7, 'repel',          350),
    (-57, 7, 'escape-rope',    550);

-- ── Emerald: Mauville City Poké Mart ──────────────────────────────────────
--   location -73 = Mauville City (Emerald v8)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-73, 8, 'ultra-ball',    1200),
    (-73, 8, 'great-ball',     600),
    (-73, 8, 'hyper-potion',  1200),
    (-73, 8, 'super-potion',   700),
    (-73, 8, 'revive',        1500),
    (-73, 8, 'ice-heal',       250),
    (-73, 8, 'burn-heal',      250),
    (-73, 8, 'awakening',      250),
    (-73, 8, 'antidote',       100),
    (-73, 8, 'parlyz-heal',    200),
    (-73, 8, 'max-repel',      700),
    (-73, 8, 'super-repel',    500),
    (-73, 8, 'repel',          350),
    (-73, 8, 'escape-rope',    550);

-- ── FireRed: Celadon City Dept Store — TMs (5F) ───────────────────────────
--   "The Big 3" elemental TMs + high-power specials + Psychic/Shadow Ball
--   location -7 = Celadon City (FireRed v10)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-7, 10, 'tm24',  3500),   -- Thunderbolt
    (-7, 10, 'tm13',  3500),   -- Ice Beam
    (-7, 10, 'tm35',  3500),   -- Flamethrower
    (-7, 10, 'tm29',  3500),   -- Psychic
    (-7, 10, 'tm30',  3000),   -- Shadow Ball
    (-7, 10, 'tm15',  7500),   -- Hyper Beam
    (-7, 10, 'tm25',  5500),   -- Thunder
    (-7, 10, 'tm14',  5500),   -- Blizzard
    (-7, 10, 'tm38',  5500);   -- Fire Blast

-- ── LeafGreen: Celadon City Dept Store — TMs (5F) ─────────────────────────
--   location -24 = Celadon City (LeafGreen v11)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-24, 11, 'tm24',  3500),
    (-24, 11, 'tm13',  3500),
    (-24, 11, 'tm35',  3500),
    (-24, 11, 'tm29',  3500),
    (-24, 11, 'tm30',  3000),
    (-24, 11, 'tm15',  7500),
    (-24, 11, 'tm25',  5500),
    (-24, 11, 'tm14',  5500),
    (-24, 11, 'tm38',  5500);

-- ── Ruby: Lilycove City Dept Store — items + TMs ──────────────────────────
--   location -46 = Lilycove City (Ruby v6)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    -- 2F medicine
    (-46, 6, 'ultra-ball',    1200),
    (-46, 6, 'great-ball',     600),
    (-46, 6, 'full-restore',  3000),
    (-46, 6, 'max-potion',    2500),
    (-46, 6, 'hyper-potion',  1200),
    (-46, 6, 'super-potion',   700),
    (-46, 6, 'revive',        1500),
    (-46, 6, 'full-heal',      600),
    (-46, 6, 'max-repel',      700),
    (-46, 6, 'super-repel',    500),
    (-46, 6, 'repel',          350),
    (-46, 6, 'escape-rope',    550),
    -- 3F TMs
    (-46, 6, 'tm24',  3500),   -- Thunderbolt
    (-46, 6, 'tm13',  3500),   -- Ice Beam
    (-46, 6, 'tm35',  3500),   -- Flamethrower
    (-46, 6, 'tm29',  3500),   -- Psychic
    (-46, 6, 'tm22',  3000),   -- Solar Beam
    (-46, 6, 'tm15',  7500),   -- Hyper Beam
    (-46, 6, 'tm25',  5500),   -- Thunder
    (-46, 6, 'tm14',  5500),   -- Blizzard
    (-46, 6, 'tm38',  5500);   -- Fire Blast

-- ── Sapphire: Lilycove City Dept Store — items + TMs ──────────────────────
--   location -62 = Lilycove City (Sapphire v7)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-62, 7, 'ultra-ball',    1200),
    (-62, 7, 'great-ball',     600),
    (-62, 7, 'full-restore',  3000),
    (-62, 7, 'max-potion',    2500),
    (-62, 7, 'hyper-potion',  1200),
    (-62, 7, 'super-potion',   700),
    (-62, 7, 'revive',        1500),
    (-62, 7, 'full-heal',      600),
    (-62, 7, 'max-repel',      700),
    (-62, 7, 'super-repel',    500),
    (-62, 7, 'repel',          350),
    (-62, 7, 'escape-rope',    550),
    (-62, 7, 'tm24',  3500),
    (-62, 7, 'tm13',  3500),
    (-62, 7, 'tm35',  3500),
    (-62, 7, 'tm29',  3500),
    (-62, 7, 'tm22',  3000),
    (-62, 7, 'tm15',  7500),
    (-62, 7, 'tm25',  5500),
    (-62, 7, 'tm14',  5500),
    (-62, 7, 'tm38',  5500);

-- ── Emerald: Lilycove City Dept Store — items + TMs ───────────────────────
--   location -78 = Lilycove City (Emerald v8)
INSERT OR IGNORE INTO shop_item (location_id, version_id, item_name, price) VALUES
    (-78, 8, 'ultra-ball',    1200),
    (-78, 8, 'great-ball',     600),
    (-78, 8, 'full-restore',  3000),
    (-78, 8, 'max-potion',    2500),
    (-78, 8, 'hyper-potion',  1200),
    (-78, 8, 'super-potion',   700),
    (-78, 8, 'revive',        1500),
    (-78, 8, 'full-heal',      600),
    (-78, 8, 'max-repel',      700),
    (-78, 8, 'super-repel',    500),
    (-78, 8, 'repel',          350),
    (-78, 8, 'escape-rope',    550),
    (-78, 8, 'tm24',  3500),
    (-78, 8, 'tm13',  3500),
    (-78, 8, 'tm35',  3500),
    (-78, 8, 'tm29',  3500),
    (-78, 8, 'tm22',  3000),
    (-78, 8, 'tm15',  7500),
    (-78, 8, 'tm25',  5500),
    (-78, 8, 'tm14',  5500),
    (-78, 8, 'tm38',  5500);

PRAGMA foreign_keys = ON;

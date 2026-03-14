-- Migration 011: static/scripted encounters for Gen 3 games.
-- PokeAPI /location-area only covers wild slots; static encounters
-- (legendary Pokémon, event Pokémon, item-gated encounters) must be
-- seeded manually because they never appear in PokeAPI encounter data.
--
-- Negative location IDs continue from migration 005 (min was -82).
-- Encounter IDs start at 1001 to stay well above seeds.sql (max 328)
-- and PokeAPI auto-increment values.

PRAGMA foreign_keys = OFF;

-- ── Species + forms not yet in seeds ──────────────────────────────────────────
INSERT OR IGNORE INTO pokemon_species VALUES (143, 'snorlax');
INSERT OR IGNORE INTO pokemon_species VALUES (144, 'articuno');
INSERT OR IGNORE INTO pokemon_species VALUES (145, 'zapdos');
INSERT OR IGNORE INTO pokemon_species VALUES (146, 'moltres');
INSERT OR IGNORE INTO pokemon_species VALUES (150, 'mewtwo');
INSERT OR IGNORE INTO pokemon_species VALUES (249, 'lugia');
INSERT OR IGNORE INTO pokemon_species VALUES (250, 'ho-oh');
INSERT OR IGNORE INTO pokemon_species VALUES (377, 'regirock');
INSERT OR IGNORE INTO pokemon_species VALUES (378, 'regice');
INSERT OR IGNORE INTO pokemon_species VALUES (379, 'registeel');
INSERT OR IGNORE INTO pokemon_species VALUES (380, 'latias');
INSERT OR IGNORE INTO pokemon_species VALUES (381, 'latios');
INSERT OR IGNORE INTO pokemon_species VALUES (382, 'kyogre');
INSERT OR IGNORE INTO pokemon_species VALUES (383, 'groudon');
INSERT OR IGNORE INTO pokemon_species VALUES (384, 'rayquaza');
INSERT OR IGNORE INTO pokemon_species VALUES (385, 'jirachi');
INSERT OR IGNORE INTO pokemon_species VALUES (386, 'deoxys');

INSERT OR IGNORE INTO pokemon_form VALUES (143, 143, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (144, 144, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (145, 145, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (146, 146, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (150, 150, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (249, 249, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (250, 250, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (377, 377, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (378, 378, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (379, 379, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (380, 380, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (381, 381, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (382, 382, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (383, 383, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (384, 384, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (385, 385, 'default');
INSERT OR IGNORE INTO pokemon_form VALUES (386, 386, 'default');

-- ── Missing Kanto locations (LeafGreen routes not yet seeded from PokeAPI) ────
-- power-plant for LeafGreen (version 11) — only FR was seeded
INSERT OR IGNORE INTO location VALUES (-83, 'power-plant',     11, 'kanto');
-- Snorlax blocking routes for LeafGreen
INSERT OR IGNORE INTO location VALUES (-84, 'kanto-route-12',  11, 'kanto');
INSERT OR IGNORE INTO location VALUES (-85, 'kanto-route-16',  11, 'kanto');
-- Event islands — LeafGreen counterparts (FR rows already seeded by PokeAPI)
INSERT OR IGNORE INTO location VALUES (-86, 'navel-rock',      11, 'kanto');
INSERT OR IGNORE INTO location VALUES (-87, 'birth-island',    11, 'kanto');

-- ── Hoenn legendary locations (no PokeAPI wild-slot data for these areas) ─────
-- Sky Pillar (Rayquaza)
INSERT OR IGNORE INTO location VALUES (-88, 'sky-pillar',         6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-89, 'sky-pillar',         7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-90, 'sky-pillar',         8, 'hoenn');
-- Cave of Origin (Kyogre in Sapphire+Emerald; Groudon in Ruby+Emerald)
INSERT OR IGNORE INTO location VALUES (-91, 'cave-of-origin',     6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-92, 'cave-of-origin',     7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-93, 'cave-of-origin',     8, 'hoenn');
-- Regi trio chambers
INSERT OR IGNORE INTO location VALUES (-94, 'desert-ruins',       6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-95, 'desert-ruins',       7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-96, 'desert-ruins',       8, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-97, 'island-cave',        6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-98, 'island-cave',        7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-99, 'island-cave',        8, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-100, 'ancient-tomb',      6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-101, 'ancient-tomb',      7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-102, 'ancient-tomb',      8, 'hoenn');
-- Southern Island (Latias gift in Ruby; Latios gift in Sapphire; both via eon ticket in Emerald)
INSERT OR IGNORE INTO location VALUES (-103, 'southern-island',   6, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-104, 'southern-island',   7, 'hoenn');
INSERT OR IGNORE INTO location VALUES (-105, 'southern-island',   8, 'hoenn');
-- Space Center Mossdeep (Jirachi via bonus disc; Deoxys via event in Emerald)
INSERT OR IGNORE INTO location VALUES (-106, 'mossdeep-space-center', 8, 'hoenn');

-- ── Kanto static encounters (FireRed = version 10, LeafGreen = version 11) ────

-- Articuno — Seafoam Islands (FR: loc 258, LG: loc 259), level 50
INSERT OR IGNORE INTO encounter VALUES (1001, 258, 144, 50, 50, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1002, 259, 144, 50, 50, 'static', '[]');

-- Zapdos — Power Plant (FR: loc 330, LG: loc -83), level 50
INSERT OR IGNORE INTO encounter VALUES (1003, 330,  145, 50, 50, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1004, -83,  145, 50, 50, 'static', '[]');

-- Moltres — Mt. Ember (FR: loc 488, LG: loc 489), level 50
INSERT OR IGNORE INTO encounter VALUES (1005, 488, 146, 50, 50, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1006, 489, 146, 50, 50, 'static', '[]');

-- Mewtwo — Cerulean Cave (FR: loc 323, LG: loc 324), level 70
INSERT OR IGNORE INTO encounter VALUES (1007, 323, 150, 70, 70, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1008, 324, 150, 70, 70, 'static', '[]');

-- Snorlax — Route 12 (FR: loc 276, LG: loc -84) and Route 16 (FR: loc 309, LG: loc -85), level 30
INSERT OR IGNORE INTO encounter VALUES (1009, 276,  143, 30, 30, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1010, -84,  143, 30, 30, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1011, 309,  143, 30, 30, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1012, -85,  143, 30, 30, 'static', '[]');

-- Lugia + Ho-Oh — Navel Rock (FR: loc 807, LG: loc -86), level 70
-- (event item; both Pokémon are available at Navel Rock in both versions)
INSERT OR IGNORE INTO encounter VALUES (1013, 807,  249, 70, 70, 'static', '["item-mysticticket"]');
INSERT OR IGNORE INTO encounter VALUES (1014, -86,  249, 70, 70, 'static', '["item-mysticticket"]');
INSERT OR IGNORE INTO encounter VALUES (1015, 807,  250, 70, 70, 'static', '["item-mysticticket"]');
INSERT OR IGNORE INTO encounter VALUES (1016, -86,  250, 70, 70, 'static', '["item-mysticticket"]');

-- Deoxys — Birth Island (FR: loc 806, LG: loc -87), level 30
-- (event item: AuroraTicket)
INSERT OR IGNORE INTO encounter VALUES (1017, 806,  386, 30, 30, 'static', '["item-auroraticket"]');
INSERT OR IGNORE INTO encounter VALUES (1018, -87,  386, 30, 30, 'static', '["item-auroraticket"]');

-- ── Hoenn static encounters (Ruby = 6, Sapphire = 7, Emerald = 8) ─────────────

-- Rayquaza — Sky Pillar, level 70 (required story encounter in Emerald; optional in R/S)
INSERT OR IGNORE INTO encounter VALUES (1019, -88, 384, 70, 70, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1020, -89, 384, 70, 70, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1021, -90, 384, 70, 70, 'static', '[]');

-- Kyogre — Cave of Origin (Sapphire: level 45; Emerald: level 50)
INSERT OR IGNORE INTO encounter VALUES (1022, -92, 382, 45, 45, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1023, -93, 382, 50, 50, 'static', '[]');

-- Groudon — Cave of Origin (Ruby: level 45; Emerald: level 50)
INSERT OR IGNORE INTO encounter VALUES (1024, -91, 383, 45, 45, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1025, -93, 383, 50, 50, 'static', '[]');

-- Regirock — Desert Ruins, level 40
INSERT OR IGNORE INTO encounter VALUES (1026, -94, 377, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1027, -95, 377, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1028, -96, 377, 40, 40, 'static', '[]');

-- Regice — Island Cave, level 40
INSERT OR IGNORE INTO encounter VALUES (1029, -97,  378, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1030, -98,  378, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1031, -99,  378, 40, 40, 'static', '[]');

-- Registeel — Ancient Tomb, level 40
INSERT OR IGNORE INTO encounter VALUES (1032, -100, 379, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1033, -101, 379, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1034, -102, 379, 40, 40, 'static', '[]');

-- Latias — roaming in Ruby (Southern Island in Emerald via Eon Ticket)
INSERT OR IGNORE INTO encounter VALUES (1035, -103, 380, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1036, -105, 380, 40, 40, 'static', '["item-eonticket"]');

-- Latios — roaming in Sapphire (Southern Island in Emerald via Eon Ticket)
INSERT OR IGNORE INTO encounter VALUES (1037, -104, 381, 40, 40, 'static', '[]');
INSERT OR IGNORE INTO encounter VALUES (1038, -105, 381, 40, 40, 'static', '["item-eonticket"]');

-- Jirachi — Space Center (event: Pokémon Colosseum bonus disc)
INSERT OR IGNORE INTO encounter VALUES (1039, -106, 385,  5,  5, 'static', '["item-event"]');

-- Deoxys — Space Center Emerald (event item)
INSERT OR IGNORE INTO encounter VALUES (1040, -106, 386, 30, 30, 'static', '["item-event"]');

PRAGMA foreign_keys = ON;

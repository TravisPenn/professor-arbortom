-- 017_opponent_teams.sql
-- Gym Leader and Elite Four / Champion data for all Gen 3 versions.
--
-- badge_order: 1-8 = gym leaders, 9-12 = Elite Four members, 13 = Champion.
-- version_ids: Ruby=6, Sapphire=7, Emerald=8, FireRed=10, LeafGreen=11.
--
-- form_id values reference pokemon.id (= PokeAPI pokemon form ID, which equals
-- the National Dex number for single-form Pokémon in Gen 3).
--
-- PRAGMA foreign_keys is disabled for the INSERT block because the pokemon
-- table is only sparsely populated at migration time (only the 6 starters from
-- migration 002 are present). PokeAPI hydration fills the rest later. The FK
-- constraints remain defined in the schema for future enforcement.

CREATE TABLE IF NOT EXISTS gym_leader (
    id             INTEGER PRIMARY KEY,
    version_id     INTEGER NOT NULL REFERENCES game_version(id),
    badge_order    INTEGER NOT NULL,  -- 1–13
    name           TEXT    NOT NULL,
    type_specialty TEXT    NOT NULL,  -- primary type label (display only)
    location_name  TEXT    NOT NULL,
    UNIQUE(version_id, badge_order)
);

CREATE TABLE IF NOT EXISTS gym_leader_pokemon (
    id              INTEGER PRIMARY KEY,
    gym_leader_id   INTEGER NOT NULL REFERENCES gym_leader(id),
    slot            INTEGER NOT NULL,  -- 1–6
    form_id         INTEGER NOT NULL REFERENCES pokemon(id),
    level           INTEGER NOT NULL,
    held_item       TEXT,
    move_1          TEXT,
    move_2          TEXT,
    move_3          TEXT,
    move_4          TEXT,
    -- starter_variant: non-NULL only for Blue/Champion slot-6 variants.
    -- Value is the player's starter type that causes this mon to appear.
    -- 'grass' = player chose Bulbasaur → Blue uses Charizard
    -- 'fire'  = player chose Charmander → Blue uses Blastoise
    -- 'water' = player chose Squirtle → Blue uses Venusaur
    starter_variant TEXT
);

-- ═══════════════════════════════════════════════════════════
-- GYM LEADERS
-- ═══════════════════════════════════════════════════════════

PRAGMA foreign_keys = OFF;

-- FireRed (version_id=10) ──────────────────────────────────
INSERT INTO gym_leader (id, version_id, badge_order, name, type_specialty, location_name) VALUES
( 1, 10,  1, 'Brock',    'rock',      'Pewter City'),
( 2, 10,  2, 'Misty',    'water',     'Cerulean City'),
( 3, 10,  3, 'Lt. Surge','electric',  'Vermilion City'),
( 4, 10,  4, 'Erika',    'grass',     'Celadon City'),
( 5, 10,  5, 'Koga',     'poison',    'Fuchsia City'),
( 6, 10,  6, 'Sabrina',  'psychic',   'Saffron City'),
( 7, 10,  7, 'Blaine',   'fire',      'Cinnabar Island'),
( 8, 10,  8, 'Giovanni', 'ground',    'Viridian City'),
( 9, 10,  9, 'Lorelei',  'ice',       'Indigo Plateau'),
(10, 10, 10, 'Bruno',    'fighting',  'Indigo Plateau'),
(11, 10, 11, 'Agatha',   'ghost',     'Indigo Plateau'),
(12, 10, 12, 'Lance',    'dragon',    'Indigo Plateau'),
(13, 10, 13, 'Blue',     'mixed',     'Indigo Plateau');

-- LeafGreen (version_id=11) — identical teams to FireRed ───
INSERT INTO gym_leader (id, version_id, badge_order, name, type_specialty, location_name) VALUES
(14, 11,  1, 'Brock',    'rock',      'Pewter City'),
(15, 11,  2, 'Misty',    'water',     'Cerulean City'),
(16, 11,  3, 'Lt. Surge','electric',  'Vermilion City'),
(17, 11,  4, 'Erika',    'grass',     'Celadon City'),
(18, 11,  5, 'Koga',     'poison',    'Fuchsia City'),
(19, 11,  6, 'Sabrina',  'psychic',   'Saffron City'),
(20, 11,  7, 'Blaine',   'fire',      'Cinnabar Island'),
(21, 11,  8, 'Giovanni', 'ground',    'Viridian City'),
(22, 11,  9, 'Lorelei',  'ice',       'Indigo Plateau'),
(23, 11, 10, 'Bruno',    'fighting',  'Indigo Plateau'),
(24, 11, 11, 'Agatha',   'ghost',     'Indigo Plateau'),
(25, 11, 12, 'Lance',    'dragon',    'Indigo Plateau'),
(26, 11, 13, 'Blue',     'mixed',     'Indigo Plateau');

-- Ruby (version_id=6) ──────────────────────────────────────
INSERT INTO gym_leader (id, version_id, badge_order, name, type_specialty, location_name) VALUES
(27,  6,  1, 'Roxanne',   'rock',      'Rustboro City'),
(28,  6,  2, 'Brawly',    'fighting',  'Dewford Town'),
(29,  6,  3, 'Wattson',   'electric',  'Mauville City'),
(30,  6,  4, 'Flannery',  'fire',      'Lavaridge Town'),
(31,  6,  5, 'Norman',    'normal',    'Petalburg City'),
(32,  6,  6, 'Winona',    'flying',    'Fortree City'),
(33,  6,  7, 'Tate & Liza','psychic',  'Mossdeep City'),
(34,  6,  8, 'Wallace',   'water',     'Sootopolis City'),
(35,  6,  9, 'Sidney',    'dark',      'Ever Grande City'),
(36,  6, 10, 'Phoebe',    'ghost',     'Ever Grande City'),
(37,  6, 11, 'Glacia',    'ice',       'Ever Grande City'),
(38,  6, 12, 'Drake',     'dragon',    'Ever Grande City'),
(39,  6, 13, 'Steven',    'steel',     'Ever Grande City');

-- Sapphire (version_id=7) — identical teams to Ruby ────────
INSERT INTO gym_leader (id, version_id, badge_order, name, type_specialty, location_name) VALUES
(40,  7,  1, 'Roxanne',   'rock',      'Rustboro City'),
(41,  7,  2, 'Brawly',    'fighting',  'Dewford Town'),
(42,  7,  3, 'Wattson',   'electric',  'Mauville City'),
(43,  7,  4, 'Flannery',  'fire',      'Lavaridge Town'),
(44,  7,  5, 'Norman',    'normal',    'Petalburg City'),
(45,  7,  6, 'Winona',    'flying',    'Fortree City'),
(46,  7,  7, 'Tate & Liza','psychic',  'Mossdeep City'),
(47,  7,  8, 'Wallace',   'water',     'Sootopolis City'),
(48,  7,  9, 'Sidney',    'dark',      'Ever Grande City'),
(49,  7, 10, 'Phoebe',    'ghost',     'Ever Grande City'),
(50,  7, 11, 'Glacia',    'ice',       'Ever Grande City'),
(51,  7, 12, 'Drake',     'dragon',    'Ever Grande City'),
(52,  7, 13, 'Steven',    'steel',     'Ever Grande City');

-- Emerald (version_id=8) — upgraded teams; Juan at badge 8; Wallace is Champion ─
INSERT INTO gym_leader (id, version_id, badge_order, name, type_specialty, location_name) VALUES
(53,  8,  1, 'Roxanne',   'rock',      'Rustboro City'),
(54,  8,  2, 'Brawly',    'fighting',  'Dewford Town'),
(55,  8,  3, 'Wattson',   'electric',  'Mauville City'),
(56,  8,  4, 'Flannery',  'fire',      'Lavaridge Town'),
(57,  8,  5, 'Norman',    'normal',    'Petalburg City'),
(58,  8,  6, 'Winona',    'flying',    'Fortree City'),
(59,  8,  7, 'Tate & Liza','psychic',  'Mossdeep City'),
(60,  8,  8, 'Juan',      'water',     'Sootopolis City'),
(61,  8,  9, 'Sidney',    'dark',      'Ever Grande City'),
(62,  8, 10, 'Phoebe',    'ghost',     'Ever Grande City'),
(63,  8, 11, 'Glacia',    'ice',       'Ever Grande City'),
(64,  8, 12, 'Drake',     'dragon',    'Ever Grande City'),
(65,  8, 13, 'Wallace',   'water',     'Ever Grande City');

-- ═══════════════════════════════════════════════════════════
-- GYM LEADER POKÉMON
-- form_id = PokeAPI pokemon form id (= National Dex # for single-form Pokémon)
-- ═══════════════════════════════════════════════════════════

-- ── FireRed & LeafGreen shared team data ──────────────────
-- We insert for FR (leader IDs 1-13) then LG (14-26) referencing the same data.

-- Brock — FR (id=1) / LG (id=14)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(1, 1,  74, 12, 'tackle',      'defense-curl', 'rock-throw',  'magnitude'),   -- Geodude
(1, 2,  95, 14, 'rock-throw',  'screech',      'bide',        'rock-tomb'),   -- Onix
(14, 1, 74, 12, 'tackle',      'defense-curl', 'rock-throw',  'magnitude'),
(14, 2, 95, 14, 'rock-throw',  'screech',      'bide',        'rock-tomb');

-- Misty — FR (id=2) / LG (id=15)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(2, 1, 120, 18, 'water-gun',   'rapid-spin',   'recover',     'bubblebeam'),  -- Staryu
(2, 2, 121, 21, 'bubblebeam',  'water-pulse',  'swift',       'recover'),     -- Starmie
(15, 1, 120, 18, 'water-gun',  'rapid-spin',   'recover',     'bubblebeam'),
(15, 2, 121, 21, 'bubblebeam', 'water-pulse',  'swift',       'recover');

-- Lt. Surge — FR (id=3) / LG (id=16)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(3, 1, 100, 21, 'sonic-boom',  'thundershock', 'screech',     'charge'),      -- Voltorb
(3, 2,  25, 24, 'thunderbolt', 'quick-attack', 'thunder-wave','double-team'), -- Pikachu
(3, 3,  26, 24, 'thunderbolt', 'mega-punch',   'thunder-wave','mega-kick'),   -- Raichu
(16, 1, 100, 21, 'sonic-boom', 'thundershock', 'screech',     'charge'),
(16, 2,  25, 24, 'thunderbolt','quick-attack', 'thunder-wave','double-team'),
(16, 3,  26, 24, 'thunderbolt','mega-punch',   'thunder-wave','mega-kick');

-- Erika — FR (id=4) / LG (id=17)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(4, 1,  71, 29, 'acid',        'wrap',         'razor-leaf',  'sweet-scent'), -- Victreebel
(4, 2, 114, 24, 'stun-spore',  'vine-whip',    'constrict',   'ingrain'),     -- Tangela
(4, 3,  45, 29, 'petal-dance', 'sleep-powder', 'stun-spore',  'acid'),        -- Vileplume
(17, 1,  71, 29, 'acid',       'wrap',         'razor-leaf',  'sweet-scent'),
(17, 2, 114, 24, 'stun-spore', 'vine-whip',    'constrict',   'ingrain'),
(17, 3,  45, 29, 'petal-dance','sleep-powder', 'stun-spore',  'acid');

-- Koga — FR (id=5) / LG (id=18)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(5, 1, 109, 37, 'smog',        'smokescreen',  'sludge',      'selfdestruct'),-- Koffing
(5, 2,  89, 39, 'minimize',    'smokescreen',  'poison-gas',  'sludge'),      -- Muk
(5, 3, 109, 37, 'smog',        'smokescreen',  'sludge',      'selfdestruct'),-- Koffing
(5, 4, 110, 43, 'smokescreen', 'haze',         'toxic',       'selfdestruct'),-- Weezing
(18, 1, 109, 37, 'smog',       'smokescreen',  'sludge',      'selfdestruct'),
(18, 2,  89, 39, 'minimize',   'smokescreen',  'poison-gas',  'sludge'),
(18, 3, 109, 37, 'smog',       'smokescreen',  'sludge',      'selfdestruct'),
(18, 4, 110, 43, 'smokescreen','haze',         'toxic',       'selfdestruct');

-- Sabrina — FR (id=6) / LG (id=19)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(6, 1,  64, 38, 'psybeam',     'kinesis',      'recover',     'psychic'),     -- Kadabra
(6, 2, 122, 37, 'psych-up',    'encore',       'substitute',  'psychic'),     -- Mr. Mime
(6, 3,  49, 38, 'psychic',     'sleep-powder', 'stun-spore',  'leech-life'),  -- Venomoth
(6, 4,  65, 43, 'psychic',     'recover',      'calm-mind',   'shadow-ball'), -- Alakazam
(19, 1,  64, 38, 'psybeam',    'kinesis',      'recover',     'psychic'),
(19, 2, 122, 37, 'psych-up',   'encore',       'substitute',  'psychic'),
(19, 3,  49, 38, 'psychic',    'sleep-powder', 'stun-spore',  'leech-life'),
(19, 4,  65, 43, 'psychic',    'recover',      'calm-mind',   'shadow-ball');

-- Blaine — FR (id=7) / LG (id=20)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(7, 1,  58, 42, 'ember',       'bite',         'roar',        'flamethrower'),-- Growlithe
(7, 2,  77, 40, 'ember',       'tail-whip',    'agility',     'flame-wheel'), -- Ponyta
(7, 3,  78, 42, 'ember',       'tail-whip',    'fire-spin',   'flamethrower'),-- Rapidash
(7, 4,  59, 47, 'fire-blast',  'flamethrower', 'crunch',      'extreme-speed'),-- Arcanine
(20, 1,  58, 42, 'ember',      'bite',         'roar',        'flamethrower'),
(20, 2,  77, 40, 'ember',      'tail-whip',    'agility',     'flame-wheel'),
(20, 3,  78, 42, 'ember',      'tail-whip',    'fire-spin',   'flamethrower'),
(20, 4,  59, 47, 'fire-blast', 'flamethrower', 'crunch',      'extreme-speed');

-- Giovanni — FR (id=8) / LG (id=21)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(8, 1, 111, 45, 'stomp',       'fury-attack',  'earthquake',  'rock-blast'),  -- Rhyhorn
(8, 2,  51, 42, 'earthquake',  'slash',        'sand-attack', 'scratch'),     -- Dugtrio
(8, 3,  31, 44, 'earthquake',  'body-slam',    'rock-slide',  'poison-sting'),-- Nidoqueen
(8, 4,  34, 45, 'earthquake',  'mega-punch',   'mega-kick',   'thrash'),      -- Nidoking
(8, 5, 112, 50, 'earthquake',  'stomp',        'horn-drill',  'rock-blast'),  -- Rhydon
(21, 1, 111, 45, 'stomp',      'fury-attack',  'earthquake',  'rock-blast'),
(21, 2,  51, 42, 'earthquake', 'slash',        'sand-attack', 'scratch'),
(21, 3,  31, 44, 'earthquake', 'body-slam',    'rock-slide',  'poison-sting'),
(21, 4,  34, 45, 'earthquake', 'mega-punch',   'mega-kick',   'thrash'),
(21, 5, 112, 50, 'earthquake', 'stomp',        'horn-drill',  'rock-blast');

-- Lorelei — FR (id=9) / LG (id=22)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(9, 1,  87, 52, 'surf',        'ice-beam',     'blizzard',    'protect'),     -- Dewgong
(9, 2,  91, 51, 'ice-beam',    'blizzard',     'spikes',      'protect'),     -- Cloyster
(9, 3,  80, 52, 'surf',        'psychic',      'amnesia',     'withdraw'),    -- Slowbro
(9, 4, 124, 54, 'ice-punch',   'lovely-kiss',  'psychic',     'blizzard'),    -- Jynx
(9, 5, 131, 54, 'ice-beam',    'thunderbolt',  'psychic',     'confuse-ray'), -- Lapras
(22, 1,  87, 52, 'surf',       'ice-beam',     'blizzard',    'protect'),
(22, 2,  91, 51, 'ice-beam',   'blizzard',     'spikes',      'protect'),
(22, 3,  80, 52, 'surf',       'psychic',      'amnesia',     'withdraw'),
(22, 4, 124, 54, 'ice-punch',  'lovely-kiss',  'psychic',     'blizzard'),
(22, 5, 131, 54, 'ice-beam',   'thunderbolt',  'psychic',     'confuse-ray');

-- Bruno — FR (id=10) / LG (id=23)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(10, 1,  95, 53, 'rock-tomb',  'iron-tail',    'bind',        'sandstorm'),   -- Onix
(10, 2, 107, 55, 'counter',    'ice-punch',    'fire-punch',  'thunder-punch'),-- Hitmonchan
(10, 3, 106, 55, 'hi-jump-kick','rock-slide',  'mind-reader', 'mega-kick'),   -- Hitmonlee
(10, 4,  95, 54, 'rock-tomb',  'iron-tail',    'bind',        'sandstorm'),   -- Onix
(10, 5,  68, 58, 'cross-chop', 'earthquake',   'rock-slide',  'bulk-up'),     -- Machamp
(23, 1,  95, 53, 'rock-tomb',  'iron-tail',    'bind',        'sandstorm'),
(23, 2, 107, 55, 'counter',    'ice-punch',    'fire-punch',  'thunder-punch'),
(23, 3, 106, 55, 'hi-jump-kick','rock-slide',  'mind-reader', 'mega-kick'),
(23, 4,  95, 54, 'rock-tomb',  'iron-tail',    'bind',        'sandstorm'),
(23, 5,  68, 58, 'cross-chop', 'earthquake',   'rock-slide',  'bulk-up');

-- Agatha — FR (id=11) / LG (id=24)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(11, 1,  94, 54, 'shadow-ball', 'confuse-ray',  'mean-look',   'spite'),        -- Gengar
(11, 2,  93, 53, 'mean-look',   'hypnosis',     'dream-eater', 'night-shade'),  -- Haunter
(11, 3,  94, 58, 'psychic',     'shadow-ball',  'destiny-bond','hypnosis'),     -- Gengar
(11, 4,  24, 54, 'crunch',      'glare',        'acid',        'body-slam'),    -- Arbok
(11, 5,  94, 58, 'nightmare',   'shadow-ball',  'destiny-bond','confuse-ray'),  -- Gengar
(24, 1,  94, 54, 'shadow-ball', 'confuse-ray',  'mean-look',   'spite'),
(24, 2,  93, 53, 'mean-look',   'hypnosis',     'dream-eater', 'night-shade'),
(24, 3,  94, 58, 'psychic',     'shadow-ball',  'destiny-bond','hypnosis'),
(24, 4,  24, 54, 'crunch',      'glare',        'acid',        'body-slam'),
(24, 5,  94, 58, 'nightmare',   'shadow-ball',  'destiny-bond','confuse-ray');

-- Lance — FR (id=12) / LG (id=25)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(12, 1, 130, 58, 'hyper-beam',  'dragon-rage',  'bite',        'leer'),         -- Gyarados
(12, 2, 148, 56, 'surf',        'thunder-wave', 'iron-tail',   'slam'),         -- Dragonair
(12, 3, 148, 56, 'safeguard',   'thunder-wave', 'outrage',     'slam'),         -- Dragonair
(12, 4, 142, 60, 'hyper-beam',  'rock-slide',   'dragon-claw', 'iron-tail'),    -- Aerodactyl
(12, 5, 149, 62, 'outrage',     'hyper-beam',   'safeguard',   'dragon-dance'), -- Dragonite
(25, 1, 130, 58, 'hyper-beam',  'dragon-rage',  'bite',        'leer'),
(25, 2, 148, 56, 'surf',        'thunder-wave', 'iron-tail',   'slam'),
(25, 3, 148, 56, 'safeguard',   'thunder-wave', 'outrage',     'slam'),
(25, 4, 142, 60, 'hyper-beam',  'rock-slide',   'dragon-claw', 'iron-tail'),
(25, 5, 149, 62, 'outrage',     'hyper-beam',   'safeguard',   'dragon-dance');

-- Blue (Champion) — FR (id=13) / LG (id=26)
-- Slot 6 has three variants based on the player's starter choice (starter_variant).
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4, starter_variant) VALUES
(13, 1,  18, 59, 'sky-attack',  'feather-dance','mirror-move', 'agility',   NULL), -- Pidgeot
(13, 2,  65, 59, 'psychic',     'shadow-ball',  'calm-mind',   'reflect',   NULL), -- Alakazam
(13, 3, 112, 61, 'earthquake',  'megahorn',     'rock-blast',  'horn-drill',NULL), -- Rhydon
(13, 4,  59, 61, 'fire-blast',  'extreme-speed','crunch',      'roar',      NULL), -- Arcanine
(13, 5, 103, 61, 'psychic',     'sleep-powder', 'egg-bomb',    'leech-seed',NULL), -- Exeggutor
(13, 6,   6, 65, 'fire-blast',  'fly',          'dragon-claw', 'earthquake','grass'), -- Charizard (player chose Bulbasaur)
(13, 6,   9, 65, 'surf',        'blizzard',     'ice-beam',    'earthquake','fire'),  -- Blastoise (player chose Charmander)
(13, 6,   3, 65, 'solar-beam',  'synthesis',    'earthquake',  'razor-leaf','water'), -- Venusaur (player chose Squirtle)
(26, 1,  18, 59, 'sky-attack',  'feather-dance','mirror-move', 'agility',   NULL),
(26, 2,  65, 59, 'psychic',     'shadow-ball',  'calm-mind',   'reflect',   NULL),
(26, 3, 112, 61, 'earthquake',  'megahorn',     'rock-blast',  'horn-drill',NULL),
(26, 4,  59, 61, 'fire-blast',  'extreme-speed','crunch',      'roar',      NULL),
(26, 5, 103, 61, 'psychic',     'sleep-powder', 'egg-bomb',    'leech-seed',NULL),
(26, 6,   6, 65, 'fire-blast',  'fly',          'dragon-claw', 'earthquake','grass'),
(26, 6,   9, 65, 'surf',        'blizzard',     'ice-beam',    'earthquake','fire'),
(26, 6,   3, 65, 'solar-beam',  'synthesis',    'earthquake',  'razor-leaf','water');

-- ── Ruby & Sapphire shared team data ────────────────────────
-- Ruby leader IDs: 27-39, Sapphire: 40-52

-- Roxanne — Ruby (id=27) / Sapphire (id=40)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(27, 1,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),          -- Geodude
(27, 2,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),          -- Geodude
(27, 3, 299, 15, 'tackle',      'harden',       'rock-throw',  'rock-tomb'),    -- Nosepass
(40, 1,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),
(40, 2,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),
(40, 3, 299, 15, 'tackle',      'harden',       'rock-throw',  'rock-tomb');

-- Brawly — Ruby (id=28) / Sapphire (id=41)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(28, 1,  66, 17, 'karate-chop', 'low-kick',    'focus-energy', 'seismic-toss'),-- Machop
(28, 2, 307, 17, 'detect',      'focus-punch', 'meditate',     'confusion'),   -- Meditite
(28, 3, 297, 19, 'arm-thrust',  'vital-throw', 'fake-out',     'whirlwind'),   -- Hariyama
(41, 1,  66, 17, 'karate-chop', 'low-kick',    'focus-energy', 'seismic-toss'),
(41, 2, 307, 17, 'detect',      'focus-punch', 'meditate',     'confusion'),
(41, 3, 297, 19, 'arm-thrust',  'vital-throw', 'fake-out',     'whirlwind');

-- Wattson — Ruby (id=29) / Sapphire (id=42)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(29, 1,  81, 20, 'thundershock','sonic-boom',  'thunder-wave', 'spark'),       -- Magnemite
(29, 2, 100, 20, 'tackle',      'sonic-boom',  'screech',      'spark'),       -- Voltorb
(29, 3,  82, 22, 'thundershock','sonic-boom',  'thunder-wave', 'spark'),       -- Magneton
(42, 1,  81, 20, 'thundershock','sonic-boom',  'thunder-wave', 'spark'),
(42, 2, 100, 20, 'tackle',      'sonic-boom',  'screech',      'spark'),
(42, 3,  82, 22, 'thundershock','sonic-boom',  'thunder-wave', 'spark');

-- Flannery — Ruby (id=30) / Sapphire (id=43)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(30, 1, 218, 24, 'ember',       'smog',        'amnesia',      'flamethrower'),-- Slugma
(30, 2, 218, 24, 'ember',       'smog',        'amnesia',      'flamethrower'),-- Slugma
(30, 3, 324, 28, 'ember',       'smog',        'smokescreen',  'flamethrower'),-- Torkoal
(43, 1, 218, 24, 'ember',       'smog',        'amnesia',      'flamethrower'),
(43, 2, 218, 24, 'ember',       'smog',        'amnesia',      'flamethrower'),
(43, 3, 324, 28, 'ember',       'smog',        'smokescreen',  'flamethrower');

-- Norman — Ruby (id=31) / Sapphire (id=44)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(31, 1, 327, 27, 'psybeam',     'teeter-dance','encore',       'facade'),      -- Spinda
(31, 2, 293, 27, 'facade',      'slash',       'endure',       'aerial-ace'),  -- Vigoroth
(31, 3, 289, 31, 'facade',      'yawn',        'hyper-beam',   'slack-off'),   -- Slaking
(44, 1, 327, 27, 'psybeam',     'teeter-dance','encore',       'facade'),
(44, 2, 293, 27, 'facade',      'slash',       'endure',       'aerial-ace'),
(44, 3, 289, 31, 'facade',      'yawn',        'hyper-beam',   'slack-off');

-- Winona — Ruby (id=32) / Sapphire (id=45)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(32, 1, 277, 31, 'facade',      'quick-attack','aerial-ace',   'endeavor'),    -- Swellow
(32, 2, 279, 30, 'water-pulse', 'protect',     'surf',         'fly'),         -- Pelipper
(32, 3, 227, 32, 'steel-wing',  'aerial-ace',  'toxic',        'spikes'),      -- Skarmory
(32, 4, 334, 33, 'dragonbreath','dragon-dance','safeguard',    'aerial-ace'),  -- Altaria
(45, 1, 277, 31, 'facade',      'quick-attack','aerial-ace',   'endeavor'),
(45, 2, 279, 30, 'water-pulse', 'protect',     'surf',         'fly'),
(45, 3, 227, 32, 'steel-wing',  'aerial-ace',  'toxic',        'spikes'),
(45, 4, 334, 33, 'dragonbreath','dragon-dance','safeguard',    'aerial-ace');

-- Tate & Liza — Ruby (id=33) / Sapphire (id=46)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(33, 1, 338, 42, 'confusion',   'flamethrower','shadow-ball',  'light-screen'),-- Solrock
(33, 2, 337, 42, 'confusion',   'ice-beam',    'shadow-ball',  'reflect'),     -- Lunatone
(46, 1, 338, 42, 'confusion',   'flamethrower','shadow-ball',  'light-screen'),
(46, 2, 337, 42, 'confusion',   'ice-beam',    'shadow-ball',  'reflect');

-- Wallace — Ruby (id=34) / Sapphire (id=47)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(34, 1, 370, 40, 'sweet-kiss',  'attract',     'water-pulse',  'safeguard'),   -- Luvdisc
(34, 2, 340, 42, 'amnesia',     'surf',        'ancient-power','earthquake'),  -- Whiscash
(34, 3, 364, 40, 'hail',        'surf',        'blizzard',     'body-slam'),   -- Sealeo
(34, 4, 364, 42, 'hail',        'surf',        'blizzard',     'body-slam'),   -- Sealeo
(34, 5, 350, 46, 'surf',        'recover',     'ice-beam',     'attract'),     -- Milotic
(47, 1, 370, 40, 'sweet-kiss',  'attract',     'water-pulse',  'safeguard'),
(47, 2, 340, 42, 'amnesia',     'surf',        'ancient-power','earthquake'),
(47, 3, 364, 40, 'hail',        'surf',        'blizzard',     'body-slam'),
(47, 4, 364, 42, 'hail',        'surf',        'blizzard',     'body-slam'),
(47, 5, 350, 46, 'surf',        'recover',     'ice-beam',     'attract');

-- Sidney (E4 Dark) — Ruby (id=35) / Sapphire (id=48)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(35, 1, 262, 46, 'roar',        'swagger',     'shadow-ball',  'take-down'),   -- Mightyena
(35, 2, 275, 48, 'fake-out',    'swagger',     'extrasensory', 'faint-attack'),-- Shiftry
(35, 3, 332, 46, 'destiny-bond','focus-punch', 'needle-arm',   'faint-attack'),-- Cacturne
(35, 4, 359, 49, 'slash',       'bite',        'double-team',  'swords-dance'),-- Absol
(35, 5, 319, 48, 'crunch',      'scary-face',  'slash',        'surf'),        -- Sharpedo
(48, 1, 262, 46, 'roar',        'swagger',     'shadow-ball',  'take-down'),
(48, 2, 275, 48, 'fake-out',    'swagger',     'extrasensory', 'faint-attack'),
(48, 3, 332, 46, 'destiny-bond','focus-punch', 'needle-arm',   'faint-attack'),
(48, 4, 359, 49, 'slash',       'bite',        'double-team',  'swords-dance'),
(48, 5, 319, 48, 'crunch',      'scary-face',  'slash',        'surf');

-- Phoebe (E4 Ghost) — Ruby (id=36) / Sapphire (id=49)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(36, 1, 356, 48, 'will-o-wisp', 'curse',       'shadow-ball',  'mean-look'),   -- Dusclops
(36, 2, 354, 49, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),  -- Banette
(36, 3, 353, 48, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),  -- Shuppet
(36, 4, 354, 50, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),  -- Banette
(36, 5, 356, 51, 'will-o-wisp', 'curse',       'shadow-ball',  'mean-look'),   -- Dusclops
(49, 1, 356, 48, 'will-o-wisp', 'curse',       'shadow-ball',  'mean-look'),
(49, 2, 354, 49, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),
(49, 3, 353, 48, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),
(49, 4, 354, 50, 'shadow-ball', 'will-o-wisp', 'faint-attack', 'skill-swap'),
(49, 5, 356, 51, 'will-o-wisp', 'curse',       'shadow-ball',  'mean-look');

-- Glacia (E4 Ice) — Ruby (id=37) / Sapphire (id=50)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(37, 1, 364, 50, 'hail',        'blizzard',    'surf',         'body-slam'),   -- Sealeo
(37, 2, 362, 50, 'hail',        'blizzard',    'crunch',       'ice-beam'),    -- Glalie
(37, 3, 364, 52, 'hail',        'blizzard',    'surf',         'body-slam'),   -- Sealeo
(37, 4, 362, 52, 'hail',        'blizzard',    'crunch',       'ice-beam'),    -- Glalie
(37, 5, 365, 53, 'sheer-cold',  'blizzard',    'surf',         'body-slam'),   -- Walrein
(50, 1, 364, 50, 'hail',        'blizzard',    'surf',         'body-slam'),
(50, 2, 362, 50, 'hail',        'blizzard',    'crunch',       'ice-beam'),
(50, 3, 364, 52, 'hail',        'blizzard',    'surf',         'body-slam'),
(50, 4, 362, 52, 'hail',        'blizzard',    'crunch',       'ice-beam'),
(50, 5, 365, 53, 'sheer-cold',  'blizzard',    'surf',         'body-slam');

-- Drake (E4 Dragon) — Ruby (id=38) / Sapphire (id=51)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(38, 1, 372, 52, 'dragon-dance','protect',     'ember',        'headbutt'),    -- Shelgon
(38, 2, 334, 54, 'dragon-dance','dragonbreath','aerial-ace',   'safeguard'),   -- Altaria
(38, 3, 330, 53, 'crunch',      'hyper-beam',  'earthquake',   'dragonbreath'),-- Flygon
(38, 4, 330, 53, 'crunch',      'hyper-beam',  'flamethrower', 'dragonbreath'),-- Flygon
(38, 5, 373, 55, 'dragon-dance','fly',         'crunch',       'flamethrower'),-- Salamence
(51, 1, 372, 52, 'dragon-dance','protect',     'ember',        'headbutt'),
(51, 2, 334, 54, 'dragon-dance','dragonbreath','aerial-ace',   'safeguard'),
(51, 3, 330, 53, 'crunch',      'hyper-beam',  'earthquake',   'dragonbreath'),
(51, 4, 330, 53, 'crunch',      'hyper-beam',  'flamethrower', 'dragonbreath'),
(51, 5, 373, 55, 'dragon-dance','fly',         'crunch',       'flamethrower');

-- Steven (Champion Steel) — Ruby (id=39) / Sapphire (id=52)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(39, 1, 227, 57, 'aerial-ace',  'steel-wing',  'toxic',        'spikes'),      -- Skarmory
(39, 2, 375, 57, 'metal-claw',  'confusion',   'light-screen', 'take-down'),   -- Metang
(39, 3, 344, 55, 'earthquake',  'reflect',     'ancient-power','hyper-beam'),  -- Claydol
(39, 4, 306, 56, 'iron-tail',   'earthquake',  'shadow-ball',  'rock-tomb'),   -- Aggron
(39, 5, 346, 56, 'ingrain',     'confuse-ray', 'ancient-power','giga-drain'),  -- Cradily
(39, 6, 348, 56, 'metal-claw',  'ancient-power','water-pulse', 'slash'),       -- Armaldo
(39, 7, 376, 58, 'meteor-mash', 'earthquake',  'shadow-ball',  'hyper-beam'),  -- Metagross
(52, 1, 227, 57, 'aerial-ace',  'steel-wing',  'toxic',        'spikes'),
(52, 2, 375, 57, 'metal-claw',  'confusion',   'light-screen', 'take-down'),
(52, 3, 344, 55, 'earthquake',  'reflect',     'ancient-power','hyper-beam'),
(52, 4, 306, 56, 'iron-tail',   'earthquake',  'shadow-ball',  'rock-tomb'),
(52, 5, 346, 56, 'ingrain',     'confuse-ray', 'ancient-power','giga-drain'),
(52, 6, 348, 56, 'metal-claw',  'ancient-power','water-pulse', 'slash'),
(52, 7, 376, 58, 'meteor-mash', 'earthquake',  'shadow-ball',  'hyper-beam');

-- ── Emerald-specific teams ──────────────────────────────────────────────────
-- Roxanne — Emerald (id=53) — same as RS
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(53, 1,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),
(53, 2,  74, 14, 'tackle',      'defense-curl', 'rock-throw',   NULL),
(53, 3, 299, 15, 'tackle',      'harden',       'rock-throw',  'rock-tomb');

-- Brawly — Emerald (id=54) — no Machop; upgraded Meditite
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(54, 1, 307, 16, 'detect',      'focus-punch',  'meditate',    'confusion'),   -- Meditite
(54, 2, 297, 19, 'arm-thrust',  'vital-throw',  'bulk-up',     'fake-out');    -- Hariyama

-- Wattson — Emerald (id=55) — upgraded with Electrode and Manectric
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(55, 1, 100, 20, 'sonic-boom',  'thundershock', 'screech',     'charge'),      -- Voltorb
(55, 2,  82, 22, 'thundershock','sonic-boom',   'thunder-wave','spark'),       -- Magneton
(55, 3, 101, 24, 'sonic-boom',  'rollout',      'screech',     'spark'),       -- Electrode
(55, 4, 310, 24, 'bite',        'spark',        'thunder-wave','howl');        -- Manectric

-- Flannery — Emerald (id=56) — upgraded with Camerupt
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(56, 1, 322, 24, 'ember',       'magnitude',    'amnesia',     'tackle'),      -- Numel
(56, 2, 322, 24, 'ember',       'magnitude',    'focus-energy',NULL),          -- Numel
(56, 3, 324, 26, 'ember',       'smog',         'smokescreen', 'body-slam'),   -- Torkoal
(56, 4, 323, 26, 'ember',       'magnitude',    'amnesia',     'flamethrower');-- Camerupt

-- Norman — Emerald (id=57) — two Vigoroth + two Slaking
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(57, 1, 327, 27, 'psybeam',     'teeter-dance', 'encore',      'facade'),      -- Spinda
(57, 2, 293, 27, 'facade',      'slash',        'endure',      'aerial-ace'),  -- Vigoroth
(57, 3, 264, 29, 'facade',      'headbutt',     'belly-drum',  'tail-whip'),   -- Linoone
(57, 4, 289, 31, 'facade',      'yawn',         'hyper-beam',  'slack-off');   -- Slaking

-- Winona — Emerald (id=58) — extra Swablu + Tropius
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(58, 1, 333, 29, 'sing',        'astonish',     'peck',        'safeguard'),   -- Swablu (id=333)
(58, 2, 357, 33, 'sunny-day',   'magical-leaf', 'razor-leaf',  'fly'),         -- Tropius
(58, 3, 279, 30, 'water-pulse', 'protect',      'surf',        'fly'),         -- Pelipper
(58, 4, 227, 32, 'steel-wing',  'aerial-ace',   'toxic',       'spikes'),      -- Skarmory
(58, 5, 334, 33, 'dragonbreath','dragon-dance', 'safeguard',   'aerial-ace');  -- Altaria

-- Tate & Liza — Emerald (id=59) — four Pokémon
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(59, 1, 344, 41, 'earthquake',  'cosmic-power', 'ancient-power','hyper-beam'), -- Claydol
(59, 2, 337, 42, 'confusion',   'ice-beam',     'shadow-ball', 'reflect'),     -- Lunatone
(59, 3, 178, 41, 'psychic',     'aerial-ace',   'shadow-ball', 'wish'),        -- Xatu (id=178)
(59, 4, 338, 42, 'confusion',   'flamethrower', 'shadow-ball', 'light-screen');-- Solrock

-- Juan (Emerald badge 8 Water) — Emerald (id=60)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(60, 1, 370, 41, 'sweet-kiss',  'attract',      'water-pulse', 'safeguard'),   -- Luvdisc
(60, 2, 340, 41, 'amnesia',     'surf',         'ancient-power','earthquake'), -- Whiscash
(60, 3, 364, 43, 'hail',        'surf',         'blizzard',    'body-slam'),   -- Sealeo
(60, 4, 342, 43, 'surf',        'crabhammer',   'protect',     'rain-dance'),  -- Crawdaunt
(60, 5, 230, 46, 'surf',        'dragon-dance', 'dragonbreath','smokescreen'); -- Kingdra (id=230)

-- Sidney — Emerald (id=61) — slightly different from RS
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(61, 1, 262, 46, 'roar',        'swagger',       'shadow-ball', 'take-down'),
(61, 2, 275, 48, 'fake-out',    'swagger',       'extrasensory','faint-attack'),
(61, 3, 359, 49, 'slash',       'bite',          'swords-dance','double-team'),
(61, 4, 332, 46, 'destiny-bond','focus-punch',   'needle-arm',  'faint-attack'),
(61, 5, 319, 48, 'crunch',      'scary-face',    'slash',       'surf');

-- Phoebe — Emerald (id=62) — same as RS
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(62, 1, 356, 48, 'will-o-wisp', 'curse',         'shadow-ball', 'mean-look'),
(62, 2, 354, 49, 'shadow-ball', 'will-o-wisp',   'faint-attack','skill-swap'),
(62, 3, 353, 48, 'shadow-ball', 'will-o-wisp',   'faint-attack','skill-swap'),
(62, 4, 354, 50, 'shadow-ball', 'will-o-wisp',   'faint-attack','skill-swap'),
(62, 5, 356, 51, 'will-o-wisp', 'curse',         'shadow-ball', 'mean-look');

-- Glacia — Emerald (id=63) — same as RS
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(63, 1, 364, 50, 'hail',        'blizzard',      'surf',        'body-slam'),
(63, 2, 364, 50, 'hail',        'blizzard',      'surf',        'body-slam'),
(63, 3, 362, 52, 'hail',        'blizzard',      'crunch',      'ice-beam'),
(63, 4, 362, 52, 'hail',        'blizzard',      'crunch',      'ice-beam'),
(63, 5, 365, 53, 'sheer-cold',  'blizzard',      'surf',        'body-slam');

-- Drake — Emerald (id=64) — same as RS
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(64, 1, 372, 52, 'dragon-dance','protect',       'ember',       'headbutt'),
(64, 2, 334, 54, 'dragon-dance','dragonbreath',  'aerial-ace',  'safeguard'),
(64, 3, 330, 53, 'crunch',      'hyper-beam',    'earthquake',  'dragonbreath'),
(64, 4, 330, 53, 'crunch',      'hyper-beam',    'flamethrower','dragonbreath'),
(64, 5, 373, 55, 'dragon-dance','fly',           'crunch',      'flamethrower');

-- Wallace (Emerald Champion Water) — Emerald (id=65)
INSERT INTO gym_leader_pokemon (gym_leader_id, slot, form_id, level, move_1, move_2, move_3, move_4) VALUES
(65, 1, 130, 57, 'hyper-beam',  'dragon-dance',  'bite',        'earthquake'),  -- Gyarados
(65, 2, 340, 58, 'amnesia',     'surf',          'ancient-power','earthquake'), -- Whiscash
(65, 3, 272, 56, 'surf',        'rain-dance',    'giga-drain',  'ice-beam'),    -- Ludicolo
(65, 4,  73, 55, 'surf',        'sludge-bomb',   'ice-beam',    'protect'),     -- Tentacruel
(65, 5, 321, 57, 'surf',        'hyper-beam',    'blizzard',    'body-slam'),   -- Wailord
(65, 6, 350, 58, 'surf',        'recover',       'ice-beam',    'dragon-dance');-- Milotic

PRAGMA foreign_keys = ON;

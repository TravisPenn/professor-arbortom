-- 008_hm_tutor_moves.sql
-- Maps HM numbers and move-tutor locations to move names.

CREATE TABLE IF NOT EXISTS hm_move (
    hm_number INTEGER PRIMARY KEY,
    move_name TEXT NOT NULL
);

-- Gen 3 HMs are identical across Ruby/Sapphire/Emerald and FireRed/LeafGreen.
-- Dive (HM08) only exists in RSE but is listed here for completeness.
INSERT OR IGNORE INTO hm_move VALUES
    (1, 'cut'),
    (2, 'fly'),
    (3, 'surf'),
    (4, 'strength'),
    (5, 'flash'),
    (6, 'rock-smash'),
    (7, 'waterfall'),
    (8, 'dive');

-- tutor_move stores where each move tutor is located per version group.
-- version_group_id 5 = Ruby/Sapphire, 6 = Emerald, 7 = FireRed/LeafGreen.
CREATE TABLE IF NOT EXISTS tutor_move (
    id INTEGER PRIMARY KEY,
    version_group_id INTEGER NOT NULL,
    move_name TEXT NOT NULL,       -- PokeAPI move slug
    location_name TEXT NOT NULL,   -- human-readable display string
    UNIQUE(version_group_id, move_name)
);

-- ── FireRed / LeafGreen (vg 7) ───────────────────────────────────────────────
INSERT OR IGNORE INTO tutor_move (version_group_id, move_name, location_name) VALUES
    -- Standalone tutors
    (7, 'mega-punch',   'Mt. Moon'),
    (7, 'dream-eater',  'Viridian City'),
    (7, 'softboiled',   'Celadon City'),
    -- Cape Brink, Two Island (all repeatable)
    (7, 'swords-dance', 'Two Island'),
    (7, 'mega-kick',    'Two Island'),
    (7, 'body-slam',    'Two Island'),
    (7, 'double-edge',  'Two Island'),
    (7, 'counter',      'Two Island'),
    (7, 'seismic-toss', 'Two Island'),
    (7, 'mimic',        'Two Island');

-- ── Ruby / Sapphire (vg 5) ───────────────────────────────────────────────────
INSERT OR IGNORE INTO tutor_move (version_group_id, move_name, location_name) VALUES
    (5, 'fury-cutter',  'Dewford Town'),
    (5, 'rollout',      'Mauville City'),
    (5, 'swagger',      'Slateport City'),
    (5, 'dynamic-punch','Verdanturf Town'),
    (5, 'thunder-wave', 'Mauville City');

-- ── Emerald (vg 6) ───────────────────────────────────────────────────────────
INSERT OR IGNORE INTO tutor_move (version_group_id, move_name, location_name) VALUES
    (6, 'fury-cutter',  'Dewford Town'),
    (6, 'rollout',      'Mauville City'),
    (6, 'swagger',      'Slateport City'),
    (6, 'dynamic-punch','Verdanturf Town'),
    (6, 'thunder-wave', 'Mauville City'),
    (6, 'body-slam',    'Fortree City'),
    (6, 'double-edge',  'Fortree City'),
    (6, 'mimic',        'Lavaridge Town'),
    (6, 'dream-eater',  'Viridian City'),  -- post-National Dex (FR/LG visit)
    (6, 'softboiled',   'Viridian City');  -- post-National Dex (FR/LG visit)
